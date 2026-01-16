package engine

import (
	"fmt"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/parser"
	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/vdbe"
)

// Compiler compiles SQL AST to VDBE bytecode.
type Compiler struct {
	engine *Engine
}

// NewCompiler creates a new SQL to VDBE compiler.
func NewCompiler(engine *Engine) *Compiler {
	return &Compiler{
		engine: engine,
	}
}

// Compile compiles a SQL statement to VDBE bytecode.
func (c *Compiler) Compile(stmt parser.Statement) (*vdbe.VDBE, error) {
	switch s := stmt.(type) {
	case *parser.SelectStmt:
		return c.CompileSelect(s)
	case *parser.InsertStmt:
		return c.CompileInsert(s)
	case *parser.UpdateStmt:
		return c.CompileUpdate(s)
	case *parser.DeleteStmt:
		return c.CompileDelete(s)
	case *parser.CreateTableStmt:
		return c.CompileCreateTable(s)
	case *parser.CreateIndexStmt:
		return c.CompileCreateIndex(s)
	case *parser.DropTableStmt:
		return c.CompileDropTable(s)
	case *parser.DropIndexStmt:
		return c.CompileDropIndex(s)
	case *parser.BeginStmt:
		return c.CompileBegin(s)
	case *parser.CommitStmt:
		return c.CompileCommit(s)
	case *parser.RollbackStmt:
		return c.CompileRollback(s)
	default:
		return nil, fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

// CompileSelect compiles a SELECT statement.
func (c *Compiler) CompileSelect(stmt *parser.SelectStmt) (*vdbe.VDBE, error) {
	vm := vdbe.New()
	vm.SetReadOnly(true)

	// Allocate registers
	// We need registers for:
	// - Result columns
	// - WHERE clause evaluation
	// - Temporary values
	numCols := len(stmt.Columns)
	vm.AllocMemory(numCols + 10) // Extra registers for temps

	// Open cursor for the table
	if stmt.From == nil || len(stmt.From.Tables) == 0 {
		// SELECT without FROM (e.g., SELECT 1+1)
		// Just evaluate expressions and return
		for i, col := range stmt.Columns {
			if col.Expr != nil {
				// Evaluate expression into register
				// For now, simplified
				vm.AddOp(vdbe.OpNull, 0, i, 0)
			}
		}
		vm.AddOp(vdbe.OpResultRow, 0, numCols, 0)
		vm.AddOp(vdbe.OpHalt, 0, 0, 0)

		// Set result column names
		colNames := make([]string, numCols)
		for i, col := range stmt.Columns {
			if col.Alias != "" {
				colNames[i] = col.Alias
			} else {
				colNames[i] = fmt.Sprintf("column_%d", i)
			}
		}
		vm.ResultCols = colNames
		return vm, nil
	}

	// Get table name from FROM clause
	tableName := stmt.From.Tables[0].TableName
	table, ok := c.engine.schema.GetTable(tableName)
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	// Allocate cursor
	cursorIdx := 0
	vm.AllocCursors(1)

	// Open cursor for table scan
	vm.AddOp(vdbe.OpOpenRead, cursorIdx, int(table.RootPage), 0)
	vm.SetComment(vm.NumOps()-1, fmt.Sprintf("Open cursor for %s", tableName))

	// Move to first row
	vm.AddOp(vdbe.OpRewind, cursorIdx, vm.NumOps()+100, 0) // Jump to end if empty
	loopStart := vm.NumOps()

	// Evaluate WHERE clause if present
	if stmt.Where != nil {
		// Evaluate WHERE expression
		// If false, skip to Next
		_ = numCols // whereReg would be numCols
		// TODO: Compile WHERE expression
		// For now, no filtering
	}

	// Extract columns into result registers
	for i, col := range stmt.Columns {
		if col.Star {
			// SELECT * - need all columns
			for j := range table.Columns {
				vm.AddOp(vdbe.OpColumn, cursorIdx, j, i+j)
			}
		} else {
			// SELECT specific column
			// Find column index
			colIdx := 0
			if ident, ok := col.Expr.(*parser.IdentExpr); ok {
				colIdx = table.GetColumnIndex(ident.Name)
				if colIdx < 0 {
					return nil, fmt.Errorf("column not found: %s", ident.Name)
				}
			}
			vm.AddOp(vdbe.OpColumn, cursorIdx, colIdx, i)
		}
	}

	// Return result row
	vm.AddOp(vdbe.OpResultRow, 0, numCols, 0)

	// Move to next row
	vm.AddOp(vdbe.OpNext, cursorIdx, loopStart, 0)

	// Close cursor and halt
	vm.AddOp(vdbe.OpClose, cursorIdx, 0, 0)
	vm.AddOp(vdbe.OpHalt, 0, 0, 0)

	// Set result column names
	colNames := make([]string, numCols)
	for i, col := range stmt.Columns {
		if col.Alias != "" {
			colNames[i] = col.Alias
		} else if col.Star {
			// For SELECT *, use actual column names
			if i < len(table.Columns) {
				colNames[i] = table.Columns[i].Name
			}
		} else if ident, ok := col.Expr.(*parser.IdentExpr); ok {
			colNames[i] = ident.Name
		} else {
			colNames[i] = fmt.Sprintf("column_%d", i)
		}
	}
	vm.ResultCols = colNames

	return vm, nil
}

// CompileInsert compiles an INSERT statement.
func (c *Compiler) CompileInsert(stmt *parser.InsertStmt) (*vdbe.VDBE, error) {
	vm := vdbe.New()
	vm.SetReadOnly(false)

	// Get table
	table, ok := c.engine.schema.GetTable(stmt.Table)
	if !ok {
		return nil, fmt.Errorf("table not found: %s", stmt.Table)
	}

	// Allocate registers for values
	numCols := len(table.Columns)
	vm.AllocMemory(numCols + 10)

	// Open cursor for table
	cursorIdx := 0
	vm.AllocCursors(1)
	vm.AddOp(vdbe.OpOpenWrite, cursorIdx, int(table.RootPage), 0)

	// For each row to insert
	for _, values := range stmt.Values {
		// Generate new rowid
		rowidReg := numCols
		vm.AddOp(vdbe.OpNewRowid, cursorIdx, rowidReg, 0)

		// Load values into registers
		for i, val := range values {
			reg := i
			if lit, ok := val.(*parser.LiteralExpr); ok {
				switch lit.Type {
				case parser.LiteralInteger:
					// Parse the integer from string
					intVal := int64(0)
					fmt.Sscanf(lit.Value, "%d", &intVal)
					vm.AddOp(vdbe.OpInteger, int(intVal), reg, 0)
				case parser.LiteralString:
					vm.AddOpWithP4Str(vdbe.OpString8, 0, reg, 0, lit.Value)
				case parser.LiteralNull:
					vm.AddOp(vdbe.OpNull, 0, reg, 0)
				}
			}
		}

		// Create record from registers
		recordReg := numCols + 1
		vm.AddOp(vdbe.OpMakeRecord, 0, len(values), recordReg)

		// Insert the record
		vm.AddOp(vdbe.OpInsert, cursorIdx, recordReg, rowidReg)
	}

	// Close cursor and halt
	vm.AddOp(vdbe.OpClose, cursorIdx, 0, 0)
	vm.AddOp(vdbe.OpHalt, 0, 0, 0)

	return vm, nil
}

// CompileUpdate compiles an UPDATE statement.
func (c *Compiler) CompileUpdate(stmt *parser.UpdateStmt) (*vdbe.VDBE, error) {
	vm := vdbe.New()
	vm.SetReadOnly(false)

	// Get table
	_, ok := c.engine.schema.GetTable(stmt.Table)
	if !ok {
		return nil, fmt.Errorf("table not found: %s", stmt.Table)
	}

	// TODO: Implement UPDATE compilation
	// This requires:
	// 1. Open cursor and scan table
	// 2. Evaluate WHERE clause for each row
	// 3. For matching rows, evaluate SET expressions
	// 4. Update the row

	vm.AddOp(vdbe.OpHalt, 0, 0, 0)
	return vm, nil
}

// CompileDelete compiles a DELETE statement.
func (c *Compiler) CompileDelete(stmt *parser.DeleteStmt) (*vdbe.VDBE, error) {
	vm := vdbe.New()
	vm.SetReadOnly(false)

	// Get table
	_, ok := c.engine.schema.GetTable(stmt.Table)
	if !ok {
		return nil, fmt.Errorf("table not found: %s", stmt.Table)
	}

	// TODO: Implement DELETE compilation
	// This requires:
	// 1. Open cursor and scan table
	// 2. Evaluate WHERE clause for each row
	// 3. Delete matching rows

	vm.AddOp(vdbe.OpHalt, 0, 0, 0)
	return vm, nil
}

// CompileCreateTable compiles a CREATE TABLE statement.
func (c *Compiler) CompileCreateTable(stmt *parser.CreateTableStmt) (*vdbe.VDBE, error) {
	vm := vdbe.New()
	vm.SetReadOnly(false)

	// Create table in schema
	table, err := c.engine.schema.CreateTable(stmt)
	if err != nil {
		return nil, err
	}

	// Allocate a root page for the table
	rootPage, err := c.engine.btree.CreateTable()
	if err != nil {
		return nil, fmt.Errorf("failed to create table btree: %w", err)
	}

	table.RootPage = rootPage

	// In a real implementation, we would also:
	// 1. Insert a row into sqlite_master
	// 2. Persist the schema

	vm.AddOp(vdbe.OpHalt, 0, 0, 0)
	return vm, nil
}

// CompileCreateIndex compiles a CREATE INDEX statement.
func (c *Compiler) CompileCreateIndex(stmt *parser.CreateIndexStmt) (*vdbe.VDBE, error) {
	vm := vdbe.New()
	vm.SetReadOnly(false)

	// Create index in schema
	index, err := c.engine.schema.CreateIndex(stmt)
	if err != nil {
		return nil, err
	}

	// Allocate a root page for the index
	rootPage, err := c.engine.btree.CreateTable()
	if err != nil {
		return nil, fmt.Errorf("failed to create index btree: %w", err)
	}

	index.RootPage = rootPage

	vm.AddOp(vdbe.OpHalt, 0, 0, 0)
	return vm, nil
}

// CompileDropTable compiles a DROP TABLE statement.
func (c *Compiler) CompileDropTable(stmt *parser.DropTableStmt) (*vdbe.VDBE, error) {
	vm := vdbe.New()
	vm.SetReadOnly(false)

	// Get table
	table, ok := c.engine.schema.GetTable(stmt.Name)
	if !ok {
		if stmt.IfExists {
			// Not an error
			vm.AddOp(vdbe.OpHalt, 0, 0, 0)
			return vm, nil
		}
		return nil, fmt.Errorf("table not found: %s", stmt.Name)
	}

	// Drop the B-tree
	if err := c.engine.btree.DropTable(table.RootPage); err != nil {
		return nil, fmt.Errorf("failed to drop table btree: %w", err)
	}

	// Remove from schema
	if err := c.engine.schema.DropTable(stmt.Name); err != nil {
		return nil, err
	}

	vm.AddOp(vdbe.OpHalt, 0, 0, 0)
	return vm, nil
}

// CompileDropIndex compiles a DROP INDEX statement.
func (c *Compiler) CompileDropIndex(stmt *parser.DropIndexStmt) (*vdbe.VDBE, error) {
	vm := vdbe.New()
	vm.SetReadOnly(false)

	// Get index
	index, ok := c.engine.schema.GetIndex(stmt.Name)
	if !ok {
		if stmt.IfExists {
			// Not an error
			vm.AddOp(vdbe.OpHalt, 0, 0, 0)
			return vm, nil
		}
		return nil, fmt.Errorf("index not found: %s", stmt.Name)
	}

	// Drop the B-tree
	if err := c.engine.btree.DropTable(index.RootPage); err != nil {
		return nil, fmt.Errorf("failed to drop index btree: %w", err)
	}

	// Remove from schema
	if err := c.engine.schema.DropIndex(stmt.Name); err != nil {
		return nil, err
	}

	vm.AddOp(vdbe.OpHalt, 0, 0, 0)
	return vm, nil
}

// CompileBegin compiles a BEGIN statement.
func (c *Compiler) CompileBegin(stmt *parser.BeginStmt) (*vdbe.VDBE, error) {
	vm := vdbe.New()
	vm.SetReadOnly(false)

	// Transaction is started by the engine
	// VDBE just needs to track it
	vm.InTxn = true

	vm.AddOp(vdbe.OpTransaction, 1, 0, 0) // Read-write transaction
	vm.AddOp(vdbe.OpHalt, 0, 0, 0)
	return vm, nil
}

// CompileCommit compiles a COMMIT statement.
func (c *Compiler) CompileCommit(stmt *parser.CommitStmt) (*vdbe.VDBE, error) {
	vm := vdbe.New()
	vm.SetReadOnly(false)

	vm.AddOp(vdbe.OpCommit, 0, 0, 0)
	vm.AddOp(vdbe.OpHalt, 0, 0, 0)
	return vm, nil
}

// CompileRollback compiles a ROLLBACK statement.
func (c *Compiler) CompileRollback(stmt *parser.RollbackStmt) (*vdbe.VDBE, error) {
	vm := vdbe.New()
	vm.SetReadOnly(false)

	vm.AddOp(vdbe.OpRollback, 0, 0, 0)
	vm.AddOp(vdbe.OpHalt, 0, 0, 0)
	return vm, nil
}
