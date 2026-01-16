package sql

import (
	"errors"
	"fmt"
)

// UpdateStmt represents a compiled UPDATE statement
type UpdateStmt struct {
	Table        string
	Columns      []string // Columns to update
	Values       []Value  // New values for columns
	Where        *WhereClause
	OrderBy      []OrderByColumn
	Limit        *int
	IsOrReplace  bool
	IsOrIgnore   bool
	IsOrAbort    bool
	IsOrFail     bool
	IsOrRollback bool
}

// WhereClause represents a WHERE clause
type WhereClause struct {
	Expr *Expression
}

// Expression represents a SQL expression
type Expression struct {
	Type     ExprType
	Column   string
	Operator string
	Value    Value
	Left     *Expression
	Right    *Expression
}

// ExprType represents the type of expression
type ExprType int

const (
	ExprColumn ExprType = iota
	ExprLiteral
	ExprBinary
	ExprUnary
	ExprFunction
)

// OrderByColumn represents an ORDER BY column
type OrderByColumn struct {
	Column     string
	Descending bool
}

// CompileUpdate compiles an UPDATE statement into VDBE bytecode
//
// Generated code structure (simplified one-pass update):
//   OP_Init         0, end
//   OP_OpenWrite    0, table_root
//   OP_Rewind       0, end
// loop:
//   OP_Rowid        0, reg_rowid
//   [WHERE evaluation if present]
//   [OP_IfNot       reg_where, next]
//   OP_Column       0, col_idx, reg_old_val
//   [Compute new values]
//   OP_MakeRecord   reg_cols, num_cols, reg_record
//   OP_Insert       0, reg_record, reg_rowid
//   OP_Delete       0, reg_old_rowid
// next:
//   OP_Next         0, loop
// end:
//   OP_Close        0
//   OP_Halt
func CompileUpdate(stmt *UpdateStmt, tableRoot int, numColumns int) (*Program, error) {
	if stmt == nil {
		return nil, errors.New("nil update statement")
	}

	if len(stmt.Columns) == 0 {
		return nil, errors.New("no columns to update")
	}

	if len(stmt.Columns) != len(stmt.Values) {
		return nil, fmt.Errorf("column count (%d) does not match value count (%d)",
			len(stmt.Columns), len(stmt.Values))
	}

	prog := &Program{
		Instructions: make([]Instruction, 0),
		NumRegisters: 0,
		NumCursors:   1,
	}

	cursorNum := 0

	// Allocate registers
	regRowid := prog.allocReg()
	_ = prog.allocReg() // regOldRowid - reserved for future use
	regNewRowid := prog.allocReg()
	regOldCols := prog.allocRegs(numColumns)
	regNewCols := prog.allocRegs(numColumns)
	regRecord := prog.allocReg()
	regWhere := prog.allocReg()

	// Calculate labels
	endLabel := -1
	loopLabel := -1
	nextLabel := -1

	// OP_Init: Initialize program
	prog.add(OpInit, 0, 0, 0, nil, 0, "Initialize program")

	// OP_OpenWrite: Open table for read/write
	prog.add(OpOpenWrite, cursorNum, tableRoot, 0, nil, 0,
		fmt.Sprintf("Open table %s for update", stmt.Table))

	// OP_Rewind: Start at beginning of table
	prog.add(OpRewind, cursorNum, 0, 0, nil, 0, "Rewind to start")
	rewindInst := len(prog.Instructions) - 1

	// Loop starts here
	loopLabel = len(prog.Instructions)

	// OP_Rowid: Get current rowid
	prog.add(OpRowid, cursorNum, regRowid, 0, nil, 0, "Get current rowid")

	// WHERE clause evaluation
	whereJumpInst := -1
	if stmt.Where != nil {
		// Evaluate WHERE expression
		if err := prog.compileExpression(stmt.Where.Expr, cursorNum, regOldCols, regWhere); err != nil {
			return nil, fmt.Errorf("WHERE clause: %v", err)
		}

		// OP_IfNot: Skip this row if WHERE is false
		prog.add(OpIfNot, regWhere, 0, 0, nil, 0, "Skip if WHERE is false")
		whereJumpInst = len(prog.Instructions) - 1
	}

	// Load old column values
	for i := 0; i < numColumns; i++ {
		prog.add(OpColumn, cursorNum, i, regOldCols+i, nil, 0,
			fmt.Sprintf("Load old column %d", i))
	}

	// Build new row with updated values
	// Copy old values first
	prog.add(OpCopy, regOldCols, regNewCols, numColumns, nil, 0,
		"Copy old values to new")

	// Update specified columns with new values
	for i, colName := range stmt.Columns {
		// Find column index (simplified - in real implementation would use table metadata)
		colIdx := i // Placeholder
		reg := regNewCols + colIdx

		if err := prog.addValueLoad(stmt.Values[i], reg); err != nil {
			return nil, fmt.Errorf("column %s: %v", colName, err)
		}
	}

	// Copy rowid (unchanged in simple update)
	prog.add(OpCopy, regRowid, regNewRowid, 0, nil, 0, "Copy rowid")

	// OP_MakeRecord: Create new record
	prog.add(OpMakeRecord, regNewCols, numColumns, regRecord, nil, 0,
		"Make updated record")

	// Delete old record
	prog.add(OpDelete, cursorNum, 0, 0, nil, 0, "Delete old record")

	// Insert new record
	prog.add(OpInsert, cursorNum, regRecord, regNewRowid, nil, 0,
		"Insert updated record")

	// Next iteration
	nextLabel = len(prog.Instructions)

	// Update WHERE jump target if present
	if whereJumpInst >= 0 {
		prog.Instructions[whereJumpInst].P2 = nextLabel
	}

	// OP_Next: Move to next row
	prog.add(OpNext, cursorNum, loopLabel, 0, nil, 0, "Next row")

	// End of loop
	endLabel = len(prog.Instructions)

	// Update Init and Rewind instructions
	prog.Instructions[0].P2 = endLabel
	prog.Instructions[rewindInst].P2 = endLabel

	// OP_Close: Close table cursor
	prog.add(OpClose, cursorNum, 0, 0, nil, 0,
		fmt.Sprintf("Close table %s", stmt.Table))

	// OP_Halt: End program
	prog.add(OpHalt, 0, 0, 0, nil, 0, "End program")

	return prog, nil
}

// compileExpression compiles an expression to VDBE bytecode
func (p *Program) compileExpression(expr *Expression, cursorNum int, regBase int, regDest int) error {
	if expr == nil {
		return errors.New("nil expression")
	}

	switch expr.Type {
	case ExprLiteral:
		// Load literal value
		return p.addValueLoad(expr.Value, regDest)

	case ExprColumn:
		// Load column value
		// In real implementation, would look up column index from table metadata
		colIdx := 0 // Placeholder
		p.add(OpColumn, cursorNum, colIdx, regDest, nil, 0,
			fmt.Sprintf("Load column %s", expr.Column))
		return nil

	case ExprBinary:
		// Evaluate left and right operands
		regLeft := p.allocReg()
		regRight := p.allocReg()

		if err := p.compileExpression(expr.Left, cursorNum, regBase, regLeft); err != nil {
			return err
		}
		if err := p.compileExpression(expr.Right, cursorNum, regBase, regRight); err != nil {
			return err
		}

		// Apply operator
		switch expr.Operator {
		case "=":
			p.add(OpEq, regLeft, regRight, regDest, nil, 0, "Equal comparison")
		case "!=":
			p.add(OpNe, regLeft, regRight, regDest, nil, 0, "Not equal comparison")
		case "<":
			p.add(OpLt, regLeft, regRight, regDest, nil, 0, "Less than comparison")
		case "<=":
			p.add(OpLe, regLeft, regRight, regDest, nil, 0, "Less than or equal")
		case ">":
			p.add(OpGt, regLeft, regRight, regDest, nil, 0, "Greater than")
		case ">=":
			p.add(OpGe, regLeft, regRight, regDest, nil, 0, "Greater than or equal")
		case "+":
			p.add(OpAdd, regLeft, regRight, regDest, nil, 0, "Addition")
		case "-":
			p.add(OpSubtract, regLeft, regRight, regDest, nil, 0, "Subtraction")
		case "*":
			p.add(OpMultiply, regLeft, regRight, regDest, nil, 0, "Multiplication")
		case "/":
			p.add(OpDivide, regLeft, regRight, regDest, nil, 0, "Division")
		default:
			return fmt.Errorf("unsupported operator: %s", expr.Operator)
		}
		return nil

	default:
		return fmt.Errorf("unsupported expression type: %v", expr.Type)
	}
}

// CompileUpdateWithIndex compiles an UPDATE that affects indexes
func CompileUpdateWithIndex(stmt *UpdateStmt, tableRoot int, numColumns int, indexes []int) (*Program, error) {
	// Start with basic update
	prog, err := CompileUpdate(stmt, tableRoot, numColumns)
	if err != nil {
		return nil, err
	}

	// In a full implementation, we would:
	// 1. Delete old index entries
	// 2. Insert new index entries
	// This requires additional cursors and complexity

	// For now, return the basic program
	return prog, nil
}

// ValidateUpdate performs validation on an UPDATE statement
func ValidateUpdate(stmt *UpdateStmt) error {
	if stmt == nil {
		return errors.New("nil update statement")
	}

	if stmt.Table == "" {
		return errors.New("table name is required")
	}

	if len(stmt.Columns) == 0 {
		return errors.New("no columns to update")
	}

	if len(stmt.Columns) != len(stmt.Values) {
		return fmt.Errorf("column count (%d) does not match value count (%d)",
			len(stmt.Columns), len(stmt.Values))
	}

	return nil
}

// NewUpdateStmt creates a new UPDATE statement
func NewUpdateStmt(table string, columns []string, values []Value, where *WhereClause) *UpdateStmt {
	return &UpdateStmt{
		Table:   table,
		Columns: columns,
		Values:  values,
		Where:   where,
	}
}

// NewWhereClause creates a new WHERE clause
func NewWhereClause(expr *Expression) *WhereClause {
	return &WhereClause{Expr: expr}
}

// NewBinaryExpression creates a new binary expression
func NewBinaryExpression(left *Expression, operator string, right *Expression) *Expression {
	return &Expression{
		Type:     ExprBinary,
		Operator: operator,
		Left:     left,
		Right:    right,
	}
}

// NewColumnExpression creates a column reference expression
func NewColumnExpression(column string) *Expression {
	return &Expression{
		Type:   ExprColumn,
		Column: column,
	}
}

// NewLiteralExpression creates a literal value expression
func NewLiteralExpression(value Value) *Expression {
	return &Expression{
		Type:  ExprLiteral,
		Value: value,
	}
}
