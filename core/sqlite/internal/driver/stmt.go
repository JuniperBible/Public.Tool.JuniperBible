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
	for i, col := range stmt.Columns {
		// For now, assume simple column references
		if ident, ok := col.Expr.(*parser.IdentExpr); ok {
			colIdx := table.GetColumnIndex(ident.Name)
			if colIdx == -1 {
				return nil, fmt.Errorf("column not found: %s", ident.Name)
			}
			// OpColumn: read column colIdx from cursor 0 into register i
			vm.AddOp(vdbe.OpColumn, 0, colIdx, i)
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

	// Allocate registers
	// Register 0: new rowid
	// Registers 1-N: column values
	// Register N+1: record
	vm.AllocMemory(numValues + 10)
	vm.AllocCursors(1)

	// Generate bytecode
	// addr 0: Init
	vm.AddOp(vdbe.OpInit, 0, 0, 0)

	// addr 1: OpenWrite - open cursor 0 for writing
	vm.AddOp(vdbe.OpOpenWrite, 0, int(table.RootPage), len(table.Columns))

	// addr 2: NewRowid - generate new rowid in register 0
	vm.AddOp(vdbe.OpNewRowid, 0, 0, 0)

	// addr 3-N: Load values into registers 1-N
	// For now, support only literal values (not expressions)
	for i, val := range stmt.Values[0] {
		reg := i + 1
		// Simplified: assume all values are literals
		if lit, ok := val.(*parser.LiteralExpr); ok {
			switch lit.Type {
			case parser.LiteralInteger:
				// Parse the integer value
				var intVal int64
				fmt.Sscanf(lit.Value, "%d", &intVal)
				// Use OpInteger for values that fit in int, store result in P3
				vm.AddOp(vdbe.OpInteger, int(intVal), 0, reg)
			case parser.LiteralFloat:
				// For now, store floats as strings (simplified)
				vm.AddOpWithP4Str(vdbe.OpString8, 0, 0, reg, lit.Value)
			case parser.LiteralString:
				vm.AddOpWithP4Str(vdbe.OpString8, 0, 0, reg, lit.Value)
			case parser.LiteralNull:
				vm.AddOp(vdbe.OpNull, 0, 0, reg)
			case parser.LiteralBlob:
				// For now, store blobs as strings (simplified)
				vm.AddOpWithP4Str(vdbe.OpString8, 0, 0, reg, lit.Value)
			default:
				vm.AddOp(vdbe.OpNull, 0, 0, reg)
			}
		} else {
			// For non-literals, put NULL for now
			vm.AddOp(vdbe.OpNull, 0, 0, reg)
		}
	}

	// addr N+1: MakeRecord - create record from registers 1-N into register N+1
	recordReg := numValues + 1
	vm.AddOp(vdbe.OpMakeRecord, 1, numValues, recordReg)

	// addr N+2: Insert - insert record into cursor 0 with rowid from register 0
	vm.AddOp(vdbe.OpInsert, 0, recordReg, 0)

	// addr N+3: Close cursor
	vm.AddOp(vdbe.OpClose, 0, 0, 0)

	// addr N+4: Halt
	vm.AddOp(vdbe.OpHalt, 0, 0, 0)

	return vm, nil
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
