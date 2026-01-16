package driver

import (
	"context"
	"database/sql/driver"
	"fmt"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/parser"
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

// compileSelect compiles a SELECT statement.
func (s *Stmt) compileSelect(vm *vdbe.VDBE, stmt *parser.SelectStmt, args []driver.NamedValue) (*vdbe.VDBE, error) {
	// This is a simplified implementation
	// A real implementation would use the planner to generate an optimal plan

	// Mark as read-only
	vm.SetReadOnly(true)

	// Get the table name from the FROM clause
	if stmt.From == nil || len(stmt.From.Tables) == 0 {
		return nil, fmt.Errorf("SELECT requires FROM clause")
	}

	tableName := stmt.From.Tables[0].TableName

	// Look up table in schema
	table, ok := s.conn.schema.GetTable(tableName)
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	// Allocate registers: we need space for result columns
	numCols := len(stmt.Columns)
	vm.AllocMemory(numCols + 10) // Extra registers for temporaries
	vm.AllocCursors(1)

	// Set result column names
	vm.ResultCols = make([]string, numCols)
	for i, col := range stmt.Columns {
		if col.Alias != "" {
			vm.ResultCols[i] = col.Alias
		} else if ident, ok := col.Expr.(*parser.IdentExpr); ok {
			vm.ResultCols[i] = ident.Name
		} else {
			vm.ResultCols[i] = fmt.Sprintf("column%d", i+1)
		}
	}

	// Generate bytecode for simple table scan
	// addr 0: Init
	vm.AddOp(vdbe.OpInit, 0, 0, 0)

	// addr 1: OpenRead - open cursor 0 on table root page
	vm.AddOp(vdbe.OpOpenRead, 0, int(table.RootPage), len(table.Columns))

	// addr 2: Rewind - move to first row, jump to addr 7 if empty
	rewindAddr := vm.AddOp(vdbe.OpRewind, 0, 0, 0)

	// addr 3-N: Column ops - read each column into registers
	// Calculate which columns are INTEGER PRIMARY KEY (rowid aliases)
	// For such columns, use OpRowid instead of OpColumn
	for i, col := range stmt.Columns {
		// For now, assume simple column references
		if ident, ok := col.Expr.(*parser.IdentExpr); ok {
			colIdx := table.GetColumnIndex(ident.Name)
			if colIdx == -1 {
				return nil, fmt.Errorf("column not found: %s", ident.Name)
			}

			// Check if this column is INTEGER PRIMARY KEY (rowid alias)
			columnDef := table.Columns[colIdx]
			isRowidAlias := columnDef.PrimaryKey &&
				(columnDef.Type == "INTEGER" || columnDef.Type == "INT")

			if isRowidAlias {
				// Use OpRowid to get the rowid value
				vm.AddOp(vdbe.OpRowid, 0, i, 0)
			} else {
				// Calculate record index: subtract preceding INTEGER PRIMARY KEY columns
				recordIdx := 0
				for j := 0; j < colIdx; j++ {
					prevCol := table.Columns[j]
					isPrevRowid := prevCol.PrimaryKey &&
						(prevCol.Type == "INTEGER" || prevCol.Type == "INT")
					if !isPrevRowid {
						recordIdx++
					}
				}
				// OpColumn: read column recordIdx from cursor 0 into register i
				vm.AddOp(vdbe.OpColumn, 0, recordIdx, i)
			}
		} else {
			// For expressions, just put NULL for now
			vm.AddOp(vdbe.OpNull, 0, 0, i)
		}
	}

	// addr N+1: ResultRow - output the row
	vm.AddOp(vdbe.OpResultRow, 0, numCols, 0)

	// addr N+2: Next - move to next row, loop back to addr 3 if more rows
	vm.AddOp(vdbe.OpNext, 0, rewindAddr+1, 0)

	// addr N+3: Close - close cursor
	vm.AddOp(vdbe.OpClose, 0, 0, 0)

	// addr N+4: Halt - end program
	haltAddr := vm.AddOp(vdbe.OpHalt, 0, 0, 0)

	// Update the Rewind jump target to halt address
	vm.Program[rewindAddr].P2 = haltAddr

	return vm, nil
}

// compileInsert compiles an INSERT statement.
func (s *Stmt) compileInsert(vm *vdbe.VDBE, stmt *parser.InsertStmt, args []driver.NamedValue) (*vdbe.VDBE, error) {
	// Mark as read-write
	vm.SetReadOnly(false)

	// Look up table in schema
	table, ok := s.conn.schema.GetTable(stmt.Table)
	if !ok {
		return nil, fmt.Errorf("table not found: %s", stmt.Table)
	}

	// Determine how many values we're inserting
	var numValues int
	if stmt.Values != nil && len(stmt.Values) > 0 {
		numValues = len(stmt.Values[0])
	} else {
		return nil, fmt.Errorf("INSERT requires VALUES clause")
	}

	// Map INSERT columns to table columns
	// If no columns specified in INSERT, use table column order
	insertColNames := stmt.Columns
	if len(insertColNames) == 0 {
		// Use all table columns in order
		for _, col := range table.Columns {
			insertColNames = append(insertColNames, col.Name)
		}
	}

	// Find INTEGER PRIMARY KEY column (rowid alias) if any
	rowidColIdx := -1 // index in INSERT column list
	rowidTableIdx := -1
	for i, name := range insertColNames {
		tableColIdx := table.GetColumnIndex(name)
		if tableColIdx >= 0 {
			col := table.Columns[tableColIdx]
			if col.PrimaryKey && (col.Type == "INTEGER" || col.Type == "INT") {
				rowidColIdx = i
				rowidTableIdx = tableColIdx
				break
			}
		}
	}

	// Count non-rowid columns (these go into the record)
	numRecordCols := numValues
	if rowidColIdx >= 0 {
		numRecordCols-- // One column is the rowid, not stored in record
	}

	// Allocate registers
	// Register 1: rowid (use 1 not 0 because P3=0 has special meaning in OpInsert)
	// Registers 2-(N+1): record column values (non-rowid columns only)
	// Register N+2: record
	rowidReg := 1
	recordStartReg := 2 // First register for record values
	vm.AllocMemory(numRecordCols + 10)
	vm.AllocCursors(1)

	// Generate bytecode
	// addr 0: Init
	vm.AddOp(vdbe.OpInit, 0, 0, 0)

	// addr 1: OpenWrite - open cursor 0 for writing
	vm.AddOp(vdbe.OpOpenWrite, 0, int(table.RootPage), len(table.Columns))

	// Track parameter index for binding
	paramIdx := 0

	// If rowid column is specified, load it into rowidReg
	// Otherwise, generate a new rowid
	if rowidColIdx >= 0 {
		// Load the rowid value from the VALUES clause
		val := stmt.Values[0][rowidColIdx]
		s.compileValue(vm, val, rowidReg, args, &paramIdx)
	} else {
		// Generate new rowid into rowidReg
		// OpNewRowid: P1=cursor, P3=destination register
		vm.AddOp(vdbe.OpNewRowid, 0, 0, rowidReg)
	}

	// Load non-rowid columns into consecutive registers starting at recordStartReg
	regIdx := recordStartReg
	for i, val := range stmt.Values[0] {
		if i == rowidColIdx {
			// Skip rowid column - it's already handled
			continue
		}
		s.compileValue(vm, val, regIdx, args, &paramIdx)
		regIdx++
	}

	// Suppress unused variable warning
	_ = rowidTableIdx

	// MakeRecord - create record from registers recordStartReg to recordStartReg+numRecordCols-1
	resultReg := recordStartReg + numRecordCols
	vm.AddOp(vdbe.OpMakeRecord, recordStartReg, numRecordCols, resultReg)

	// Insert - insert record into cursor 0 with rowid from rowidReg
	vm.AddOp(vdbe.OpInsert, 0, resultReg, rowidReg)

	// Close cursor
	vm.AddOp(vdbe.OpClose, 0, 0, 0)

	// Halt
	vm.AddOp(vdbe.OpHalt, 0, 0, 0)

	return vm, nil
}

// compileValue compiles a value expression into bytecode that stores the result in reg.
func (s *Stmt) compileValue(vm *vdbe.VDBE, val parser.Expression, reg int, args []driver.NamedValue, paramIdx *int) {
	switch expr := val.(type) {
	case *parser.LiteralExpr:
		switch expr.Type {
		case parser.LiteralInteger:
			var intVal int64
			fmt.Sscanf(expr.Value, "%d", &intVal)
			vm.AddOp(vdbe.OpInteger, int(intVal), reg, 0)
		case parser.LiteralFloat:
			vm.AddOpWithP4Str(vdbe.OpString8, 0, reg, 0, expr.Value)
		case parser.LiteralString:
			vm.AddOpWithP4Str(vdbe.OpString8, 0, reg, 0, expr.Value)
		case parser.LiteralNull:
			vm.AddOp(vdbe.OpNull, 0, 0, reg)
		case parser.LiteralBlob:
			vm.AddOpWithP4Str(vdbe.OpString8, 0, reg, 0, expr.Value)
		default:
			vm.AddOp(vdbe.OpNull, 0, 0, reg)
		}
	case *parser.VariableExpr:
		if *paramIdx < len(args) {
			arg := args[*paramIdx]
			*paramIdx++
			switch v := arg.Value.(type) {
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
		} else {
			vm.AddOp(vdbe.OpNull, 0, 0, reg)
		}
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
