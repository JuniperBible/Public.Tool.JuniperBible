package driver

import (
	"context"
	"database/sql/driver"
	"fmt"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/parser"
	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/schema"
	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/vdbe"
)

// Stmt implements database/sql/driver.Stmt for SQLite.
type Stmt struct {
	conn   *Conn
	query  string
	ast    parser.Statement
	vdbe   *vdbe.VDBE
	closed bool
}

// Close closes the statement.
func (s *Stmt) Close() error {
	if s.closed {
		return nil
	}

	s.closed = true

	// Finalize VDBE if it exists
	if s.vdbe != nil {
		s.vdbe.Finalize()
		s.vdbe = nil
	}

	// Remove from connection's statement map
	s.conn.removeStmt(s)

	return nil
}

// NumInput returns the number of placeholder parameters.
func (s *Stmt) NumInput() int {
	// Count the number of parameters in the AST
	// For now, return -1 to indicate unknown (the driver will check args at exec time)
	return -1
}

// Exec executes a statement that doesn't return rows.
func (s *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.ExecContext(context.Background(), valuesToNamedValues(args))
}

// ExecContext executes a statement that doesn't return rows.
func (s *Stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if s.closed {
		return nil, driver.ErrBadConn
	}

	// Compile the statement to VDBE bytecode
	vm, err := s.compile(args)
	if err != nil {
		return nil, fmt.Errorf("compile error: %w", err)
	}
	defer vm.Finalize()

	// Execute the statement
	if err := vm.Run(); err != nil {
		return nil, fmt.Errorf("execution error: %w", err)
	}

	// Return result
	result := &Result{
		lastInsertID: 0, // TODO: track last insert ID
		rowsAffected: vm.NumChanges,
	}

	return result, nil
}

// Query executes a query that returns rows.
func (s *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.QueryContext(context.Background(), valuesToNamedValues(args))
}

// QueryContext executes a query that returns rows.
func (s *Stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if s.closed {
		return nil, driver.ErrBadConn
	}

	// Compile the statement to VDBE bytecode
	vm, err := s.compile(args)
	if err != nil {
		return nil, fmt.Errorf("compile error: %w", err)
	}

	// Create rows iterator
	rows := &Rows{
		stmt:    s,
		vdbe:    vm,
		columns: vm.ResultCols,
		ctx:     ctx,
	}

	return rows, nil
}

// compile compiles the SQL statement into VDBE bytecode.
func (s *Stmt) compile(args []driver.NamedValue) (*vdbe.VDBE, error) {
	// Create a new VDBE
	vm := vdbe.New()

	// Set the execution context with btree access
	vm.Ctx = &vdbe.VDBEContext{
		Btree:  s.conn.btree,
		Schema: s.conn.schema,
	}

	// For now, this is a simplified compilation process
	// In a real implementation, this would:
	// 1. Use the planner to generate a query plan
	// 2. Use a code generator to emit VDBE opcodes
	// 3. Bind parameters

	switch stmt := s.ast.(type) {
	case *parser.SelectStmt:
		return s.compileSelect(vm, stmt, args)
	case *parser.InsertStmt:
		return s.compileInsert(vm, stmt, args)
	case *parser.UpdateStmt:
		return s.compileUpdate(vm, stmt, args)
	case *parser.DeleteStmt:
		return s.compileDelete(vm, stmt, args)
	case *parser.CreateTableStmt:
		return s.compileCreateTable(vm, stmt, args)
	case *parser.DropTableStmt:
		return s.compileDropTable(vm, stmt, args)
	case *parser.BeginStmt:
		return s.compileBegin(vm, stmt, args)
	case *parser.CommitStmt:
		return s.compileCommit(vm, stmt, args)
	case *parser.RollbackStmt:
		return s.compileRollback(vm, stmt, args)
	default:
		return nil, fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

// schemaColIsRowid reports whether a *schema.Column is an INTEGER PRIMARY KEY
// (a rowid alias). Such columns are not stored in the B-tree record itself.
func schemaColIsRowid(col *schema.Column) bool {
	return col.PrimaryKey && (col.Type == "INTEGER" || col.Type == "INT")
}

// selectFromTableName returns the first table name from a SELECT FROM clause.
// It returns an error when no FROM clause or no tables are present.
func selectFromTableName(stmt *parser.SelectStmt) (string, error) {
	if stmt.From == nil || len(stmt.From.Tables) == 0 {
		return "", fmt.Errorf("SELECT requires FROM clause")
	}
	return stmt.From.Tables[0].TableName, nil
}

// selectColName derives the output column name for a single SELECT column:
// alias > identifier name > positional fallback.
func selectColName(col parser.ResultColumn, pos int) string {
	if col.Alias != "" {
		return col.Alias
	}
	if ident, ok := col.Expr.(*parser.IdentExpr); ok {
		return ident.Name
	}
	return fmt.Sprintf("column%d", pos+1)
}

// schemaRecordIdx computes the B-tree record index for column colIdx in table.
// It equals the number of non-rowid columns that precede position colIdx.
func schemaRecordIdx(columns []*schema.Column, colIdx int) int {
	recordIdx := 0
	for j := 0; j < colIdx; j++ {
		if !schemaColIsRowid(columns[j]) {
			recordIdx++
		}
	}
	return recordIdx
}

// emitSelectColumnOp emits the VDBE opcode(s) required to read the i-th SELECT
// column into register i. It returns an error when the named column is not
// found in the table.
func emitSelectColumnOp(vm *vdbe.VDBE, table *schema.Table, col parser.ResultColumn, i int) error {
	ident, ok := col.Expr.(*parser.IdentExpr)
	if !ok {
		// Non-identifier expression: emit NULL placeholder.
		vm.AddOp(vdbe.OpNull, 0, 0, i)
		return nil
	}

	colIdx := table.GetColumnIndex(ident.Name)
	if colIdx == -1 {
		return fmt.Errorf("column not found: %s", ident.Name)
	}

	if schemaColIsRowid(table.Columns[colIdx]) {
		vm.AddOp(vdbe.OpRowid, 0, i, 0)
		return nil
	}

	vm.AddOp(vdbe.OpColumn, 0, schemaRecordIdx(table.Columns, colIdx), i)
	return nil
}

// compileSelect compiles a SELECT statement. CC=5
func (s *Stmt) compileSelect(vm *vdbe.VDBE, stmt *parser.SelectStmt, args []driver.NamedValue) (*vdbe.VDBE, error) {
	vm.SetReadOnly(true)

	tableName, err := selectFromTableName(stmt)
	if err != nil {
		return nil, err
	}

	table, ok := s.conn.schema.GetTable(tableName)
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	numCols := len(stmt.Columns)
	vm.AllocMemory(numCols + 10)
	vm.AllocCursors(1)

	// Build result column name list.
	vm.ResultCols = make([]string, numCols)
	for i, col := range stmt.Columns {
		vm.ResultCols[i] = selectColName(col, i)
	}

	// Emit scan preamble.
	vm.AddOp(vdbe.OpInit, 0, 0, 0)
	vm.AddOp(vdbe.OpOpenRead, 0, int(table.RootPage), len(table.Columns))
	rewindAddr := vm.AddOp(vdbe.OpRewind, 0, 0, 0)

	// Emit one column-read op per SELECT column.
	for i, col := range stmt.Columns {
		if err := emitSelectColumnOp(vm, table, col, i); err != nil {
			return nil, err
		}
	}

	// Emit scan tail.
	vm.AddOp(vdbe.OpResultRow, 0, numCols, 0)
	vm.AddOp(vdbe.OpNext, 0, rewindAddr+1, 0)
	vm.AddOp(vdbe.OpClose, 0, 0, 0)
	haltAddr := vm.AddOp(vdbe.OpHalt, 0, 0, 0)
	vm.Program[rewindAddr].P2 = haltAddr

	return vm, nil
}

// insertFirstRow validates that stmt has a VALUES clause and returns the first
// value row. It returns an error when no values are present.
func insertFirstRow(stmt *parser.InsertStmt) ([]parser.Expression, error) {
	if len(stmt.Values) == 0 {
		return nil, fmt.Errorf("INSERT requires VALUES clause")
	}
	return stmt.Values[0], nil
}

// resolveInsertColumns returns the column name list for an INSERT statement.
// When the statement omits columns, every table column is used in order.
func resolveInsertColumns(stmt *parser.InsertStmt, table *schema.Table) []string {
	if len(stmt.Columns) > 0 {
		return stmt.Columns
	}
	names := make([]string, len(table.Columns))
	for i, col := range table.Columns {
		names[i] = col.Name
	}
	return names
}

// findInsertRowidCol returns the index within names of the INTEGER PRIMARY KEY
// column, or -1 when none exists.
func findInsertRowidCol(names []string, table *schema.Table) int {
	for i, name := range names {
		idx := table.GetColumnIndex(name)
		if idx < 0 {
			continue
		}
		if schemaColIsRowid(table.Columns[idx]) {
			return i
		}
	}
	return -1
}

// emitInsertRowid emits the opcode that places the rowid into rowidReg.
// When the INSERT specifies an explicit rowid value it is loaded from the
// VALUES clause; otherwise OpNewRowid generates a fresh rowid.
func (s *Stmt) emitInsertRowid(vm *vdbe.VDBE, row []parser.Expression, rowidColIdx int, rowidReg int, args []driver.NamedValue, paramIdx *int) {
	if rowidColIdx >= 0 {
		s.compileValue(vm, row[rowidColIdx], rowidReg, args, paramIdx)
		return
	}
	// OpNewRowid: P1=cursor, P3=destination register
	vm.AddOp(vdbe.OpNewRowid, 0, 0, rowidReg)
}

// emitInsertRecordValues emits OpXxx opcodes that load each non-rowid value
// from row into consecutive registers beginning at startReg.
func (s *Stmt) emitInsertRecordValues(vm *vdbe.VDBE, row []parser.Expression, rowidColIdx int, startReg int, args []driver.NamedValue, paramIdx *int) {
	reg := startReg
	for i, val := range row {
		if i == rowidColIdx {
			continue // rowid already loaded separately
		}
		s.compileValue(vm, val, reg, args, paramIdx)
		reg++
	}
}

// compileInsert compiles an INSERT statement. CC=3
func (s *Stmt) compileInsert(vm *vdbe.VDBE, stmt *parser.InsertStmt, args []driver.NamedValue) (*vdbe.VDBE, error) {
	vm.SetReadOnly(false)

	table, ok := s.conn.schema.GetTable(stmt.Table)
	if !ok {
		return nil, fmt.Errorf("table not found: %s", stmt.Table)
	}

	row, err := insertFirstRow(stmt)
	if err != nil {
		return nil, err
	}

	colNames := resolveInsertColumns(stmt, table)
	rowidColIdx := findInsertRowidCol(colNames, table)

	numRecordCols := len(row)
	if rowidColIdx >= 0 {
		numRecordCols--
	}

	// Register layout:
	//   reg 1         - rowid  (P3=0 is special in OpInsert, so start at 1)
	//   reg 2..N+1    - record column values (non-rowid only)
	//   reg N+2       - assembled record
	const rowidReg = 1
	const recordStartReg = 2
	vm.AllocMemory(numRecordCols + 10)
	vm.AllocCursors(1)

	vm.AddOp(vdbe.OpInit, 0, 0, 0)
	vm.AddOp(vdbe.OpOpenWrite, 0, int(table.RootPage), len(table.Columns))

	paramIdx := 0
	s.emitInsertRowid(vm, row, rowidColIdx, rowidReg, args, &paramIdx)
	s.emitInsertRecordValues(vm, row, rowidColIdx, recordStartReg, args, &paramIdx)

	resultReg := recordStartReg + numRecordCols
	vm.AddOp(vdbe.OpMakeRecord, recordStartReg, numRecordCols, resultReg)
	vm.AddOp(vdbe.OpInsert, 0, resultReg, rowidReg)
	vm.AddOp(vdbe.OpClose, 0, 0, 0)
	vm.AddOp(vdbe.OpHalt, 0, 0, 0)

	return vm, nil
}

// compileLiteralExpr emits the VDBE opcode for a literal value into register reg.
// Float, String, and Blob literals all map to OpString8; Integer and Null have
// dedicated opcodes. CC=4
func compileLiteralExpr(vm *vdbe.VDBE, expr *parser.LiteralExpr, reg int) {
	switch expr.Type {
	case parser.LiteralInteger:
		var intVal int64
		fmt.Sscanf(expr.Value, "%d", &intVal)
		vm.AddOp(vdbe.OpInteger, int(intVal), reg, 0)
	case parser.LiteralNull:
		vm.AddOp(vdbe.OpNull, 0, 0, reg)
	case parser.LiteralFloat, parser.LiteralString, parser.LiteralBlob:
		vm.AddOpWithP4Str(vdbe.OpString8, 0, reg, 0, expr.Value)
	default:
		vm.AddOp(vdbe.OpNull, 0, 0, reg)
	}
}

// compileArgValue emits the VDBE opcode for a concrete bound-parameter value
// into register reg. CC=6
func compileArgValue(vm *vdbe.VDBE, val driver.Value, reg int) {
	switch v := val.(type) {
	case nil:
		vm.AddOp(vdbe.OpNull, 0, 0, reg)
	case int:
		vm.AddOp(vdbe.OpInteger, v, reg, 0)
	case int64:
		vm.AddOp(vdbe.OpInteger, int(v), reg, 0)
	case float64:
		vm.AddOpWithP4Real(vdbe.OpReal, 0, reg, 0, v)
	case string:
		vm.AddOpWithP4Str(vdbe.OpString8, 0, reg, 0, v)
	case []byte:
		vm.AddOpWithP4Blob(vdbe.OpBlob, len(v), reg, 0, v)
	default:
		vm.AddOpWithP4Str(vdbe.OpString8, 0, reg, 0, fmt.Sprintf("%v", v))
	}
}

// compileValue compiles a value expression into bytecode that stores the result in reg.
// CC=3
func (s *Stmt) compileValue(vm *vdbe.VDBE, val parser.Expression, reg int, args []driver.NamedValue, paramIdx *int) {
	switch val.(type) {
	case *parser.LiteralExpr:
		compileLiteralExpr(vm, val.(*parser.LiteralExpr), reg)
	case *parser.VariableExpr:
		if *paramIdx >= len(args) {
			vm.AddOp(vdbe.OpNull, 0, 0, reg)
			return
		}
		arg := args[*paramIdx]
		*paramIdx++
		compileArgValue(vm, arg.Value, reg)
	default:
		vm.AddOp(vdbe.OpNull, 0, 0, reg)
	}
}

// compileUpdate compiles an UPDATE statement.
func (s *Stmt) compileUpdate(vm *vdbe.VDBE, stmt *parser.UpdateStmt, args []driver.NamedValue) (*vdbe.VDBE, error) {
	vm.SetReadOnly(false)

	// Look up table in schema
	table, ok := s.conn.schema.GetTable(stmt.Table)
	if !ok {
		return nil, fmt.Errorf("table not found: %s", stmt.Table)
	}

	vm.AllocMemory(len(table.Columns) + 10)
	vm.AllocCursors(1)

	// Simplified UPDATE: update all rows (no WHERE clause support yet)
	vm.AddOp(vdbe.OpInit, 0, 0, 0)
	vm.AddOp(vdbe.OpOpenWrite, 0, int(table.RootPage), len(table.Columns))

	// TODO: Implement actual UPDATE logic with WHERE clause
	// For now, just close and halt
	vm.AddOp(vdbe.OpClose, 0, 0, 0)
	vm.AddOp(vdbe.OpHalt, 0, 0, 0)

	return vm, nil
}

// compileDelete compiles a DELETE statement.
func (s *Stmt) compileDelete(vm *vdbe.VDBE, stmt *parser.DeleteStmt, args []driver.NamedValue) (*vdbe.VDBE, error) {
	vm.SetReadOnly(false)

	// Look up table in schema
	table, ok := s.conn.schema.GetTable(stmt.Table)
	if !ok {
		return nil, fmt.Errorf("table not found: %s", stmt.Table)
	}

	vm.AllocMemory(10)
	vm.AllocCursors(1)

	// Simplified DELETE: delete all rows (no WHERE clause support yet)
	vm.AddOp(vdbe.OpInit, 0, 0, 0)
	vm.AddOp(vdbe.OpOpenWrite, 0, int(table.RootPage), len(table.Columns))

	// TODO: Implement actual DELETE logic with WHERE clause
	// For now, just close and halt
	vm.AddOp(vdbe.OpClose, 0, 0, 0)
	vm.AddOp(vdbe.OpHalt, 0, 0, 0)

	return vm, nil
}

// compileCreateTable compiles a CREATE TABLE statement.
func (s *Stmt) compileCreateTable(vm *vdbe.VDBE, stmt *parser.CreateTableStmt, args []driver.NamedValue) (*vdbe.VDBE, error) {
	vm.SetReadOnly(false)
	vm.AllocMemory(10)

	// Create the table in the schema
	// This simplified implementation registers the table in memory
	// A full implementation would also persist to sqlite_master
	table, err := s.conn.schema.CreateTable(stmt)
	if err != nil {
		return nil, err
	}

	// Allocate a root page for the table btree
	if s.conn.btree != nil {
		rootPage, err := s.conn.btree.CreateTable()
		if err != nil {
			return nil, fmt.Errorf("failed to allocate table root page: %w", err)
		}
		table.RootPage = rootPage
	} else {
		// For in-memory databases without btree, use a placeholder
		table.RootPage = 2
	}

	vm.AddOp(vdbe.OpInit, 0, 0, 0)
	vm.AddOp(vdbe.OpHalt, 0, 0, 0)

	return vm, nil
}

// compileDropTable compiles a DROP TABLE statement.
func (s *Stmt) compileDropTable(vm *vdbe.VDBE, stmt *parser.DropTableStmt, args []driver.NamedValue) (*vdbe.VDBE, error) {
	vm.SetReadOnly(false)
	vm.AllocMemory(10)

	// In a real implementation, this would:
	// 1. Remove entry from sqlite_master
	// 2. Free all pages used by the table
	// 3. Update the schema in memory

	vm.AddOp(vdbe.OpInit, 0, 0, 0)

	// TODO: Generate bytecode to:
	// - Delete from sqlite_master table
	// - Free table pages
	// - Update schema cookie

	vm.AddOp(vdbe.OpHalt, 0, 0, 0)

	return vm, nil
}

// compileBegin compiles a BEGIN statement.
func (s *Stmt) compileBegin(vm *vdbe.VDBE, stmt *parser.BeginStmt, args []driver.NamedValue) (*vdbe.VDBE, error) {
	vm.SetReadOnly(false)
	vm.InTxn = true

	vm.AddOp(vdbe.OpInit, 0, 3, 0)
	vm.AddOp(vdbe.OpHalt, 0, 0, 0)

	return vm, nil
}

// compileCommit compiles a COMMIT statement.
func (s *Stmt) compileCommit(vm *vdbe.VDBE, stmt *parser.CommitStmt, args []driver.NamedValue) (*vdbe.VDBE, error) {
	vm.SetReadOnly(false)

	vm.AddOp(vdbe.OpInit, 0, 3, 0)
	// TODO: Add commit opcode
	vm.AddOp(vdbe.OpHalt, 0, 0, 0)

	return vm, nil
}

// compileRollback compiles a ROLLBACK statement.
func (s *Stmt) compileRollback(vm *vdbe.VDBE, stmt *parser.RollbackStmt, args []driver.NamedValue) (*vdbe.VDBE, error) {
	vm.SetReadOnly(false)

	vm.AddOp(vdbe.OpInit, 0, 3, 0)
	// TODO: Add rollback opcode
	vm.AddOp(vdbe.OpHalt, 0, 0, 0)

	return vm, nil
}

// valuesToNamedValues converts []driver.Value to []driver.NamedValue
func valuesToNamedValues(args []driver.Value) []driver.NamedValue {
	nv := make([]driver.NamedValue, len(args))
	for i, v := range args {
		nv[i] = driver.NamedValue{
			Ordinal: i + 1,
			Value:   v,
		}
	}
	return nv
}
