package sql

import (
	"errors"
	"fmt"
)

// InsertStmt represents a compiled INSERT statement
type InsertStmt struct {
	Table       string
	Columns     []string
	Values      [][]Value
	IsOrReplace bool
	IsOrIgnore  bool
	IsOrAbort   bool
	IsOrFail    bool
	IsOrRollback bool
}

// OpCode represents a VDBE opcode
type OpCode int

const (
	OpInit OpCode = iota
	OpHalt
	OpOpenWrite
	OpOpenRead
	OpClose
	OpNewRowid
	OpInsert
	OpDelete
	OpRowData
	OpColumn
	OpRowid
	OpMakeRecord
	OpInteger
	OpString
	OpReal
	OpBlob
	OpNull
	OpCopy
	OpMove
	OpGoto
	OpIf
	OpIfNot
	OpEq
	OpNe
	OpLt
	OpLe
	OpGt
	OpGe
	OpAdd
	OpSubtract
	OpMultiply
	OpDivide
	OpNotFound
	OpNotExists
	OpSeek
	OpRewind
	OpNext
	OpPrev
	OpIdxInsert
	OpIdxDelete
	OpIdxRowid
	OpIdxLT
	OpIdxGE
	OpIdxGT
	OpResultRow
	OpAddImm
	OpMustBeInt
	OpAffinity
	OpTypeCheck
	OpFinishSeek
	OpFkCheck
)

// Instruction represents a single VDBE instruction
type Instruction struct {
	OpCode OpCode
	P1     int
	P2     int
	P3     int
	P4     interface{} // Can be string, int, or other data
	P5     int
	Comment string
}

// Program represents a compiled VDBE program
type Program struct {
	Instructions []Instruction
	NumRegisters int
	NumCursors   int
}

// String returns a string representation of an opcode
func (op OpCode) String() string {
	names := map[OpCode]string{
		OpInit:        "Init",
		OpHalt:        "Halt",
		OpOpenWrite:   "OpenWrite",
		OpOpenRead:    "OpenRead",
		OpClose:       "Close",
		OpNewRowid:    "NewRowid",
		OpInsert:      "Insert",
		OpDelete:      "Delete",
		OpRowData:     "RowData",
		OpColumn:      "Column",
		OpRowid:       "Rowid",
		OpMakeRecord:  "MakeRecord",
		OpInteger:     "Integer",
		OpString:      "String",
		OpReal:        "Real",
		OpBlob:        "Blob",
		OpNull:        "Null",
		OpCopy:        "Copy",
		OpMove:        "Move",
		OpGoto:        "Goto",
		OpIf:          "If",
		OpIfNot:       "IfNot",
		OpEq:          "Eq",
		OpNe:          "Ne",
		OpLt:          "Lt",
		OpLe:          "Le",
		OpGt:          "Gt",
		OpGe:          "Ge",
		OpAdd:         "Add",
		OpSubtract:    "Subtract",
		OpMultiply:    "Multiply",
		OpDivide:      "Divide",
		OpNotFound:    "NotFound",
		OpNotExists:   "NotExists",
		OpSeek:        "Seek",
		OpRewind:      "Rewind",
		OpNext:        "Next",
		OpPrev:        "Prev",
		OpIdxInsert:   "IdxInsert",
		OpIdxDelete:   "IdxDelete",
		OpIdxRowid:    "IdxRowid",
		OpIdxLT:       "IdxLT",
		OpIdxGE:       "IdxGE",
		OpIdxGT:       "IdxGT",
		OpResultRow:   "ResultRow",
		OpAddImm:      "AddImm",
		OpMustBeInt:   "MustBeInt",
		OpAffinity:    "Affinity",
		OpTypeCheck:   "TypeCheck",
		OpFinishSeek:  "FinishSeek",
		OpFkCheck:     "FkCheck",
	}
	if name, ok := names[op]; ok {
		return name
	}
	return fmt.Sprintf("Unknown(%d)", op)
}

// CompileInsert compiles an INSERT statement into VDBE bytecode
//
// Generated code structure:
//   OP_Init         0, end
//   OP_OpenWrite    0, table_root
//   OP_NewRowid     0, reg_rowid
//   OP_Integer      reg_col1, value1
//   OP_String       reg_col2, value2
//   ...
//   OP_MakeRecord   reg_col1, num_cols, reg_record
//   OP_Insert       0, reg_record, reg_rowid
//   OP_Close        0
// end:
//   OP_Halt
func CompileInsert(stmt *InsertStmt, tableRoot int) (*Program, error) {
	if stmt == nil {
		return nil, errors.New("nil insert statement")
	}

	if len(stmt.Values) == 0 {
		return nil, errors.New("no values to insert")
	}

	numCols := len(stmt.Columns)
	if numCols == 0 && len(stmt.Values) > 0 {
		numCols = len(stmt.Values[0])
	}

	prog := &Program{
		Instructions: make([]Instruction, 0),
		NumRegisters: 0,
		NumCursors:   1,
	}

	// Calculate end label (will be the last instruction)
	endLabel := -1 // Placeholder

	// OP_Init: Initialize, jump to end on error
	prog.add(OpInit, 0, 0, 0, nil, 0, "Initialize program")

	cursorNum := 0 // Cursor 0 for the table

	// OP_OpenWrite: Open table for writing
	prog.add(OpOpenWrite, cursorNum, tableRoot, 0, nil, 0,
		fmt.Sprintf("Open table %s for writing", stmt.Table))

	// Process each row of values
	for rowIdx, row := range stmt.Values {
		if len(row) != numCols {
			return nil, fmt.Errorf("row %d has %d values, expected %d",
				rowIdx, len(row), numCols)
		}

		// Allocate registers
		regRowid := prog.allocReg()      // Register for rowid
		regCols := prog.allocRegs(numCols) // Registers for column values
		regRecord := prog.allocReg()     // Register for the record

		// OP_NewRowid: Generate new rowid
		prog.add(OpNewRowid, cursorNum, regRowid, 0, nil, 0,
			fmt.Sprintf("Generate new rowid for row %d", rowIdx))

		// Load column values into registers
		for i, val := range row {
			reg := regCols + i
			if err := prog.addValueLoad(val, reg); err != nil {
				return nil, fmt.Errorf("row %d, column %d: %v", rowIdx, i, err)
			}
		}

		// OP_MakeRecord: Create record from column values
		prog.add(OpMakeRecord, regCols, numCols, regRecord, nil, 0,
			fmt.Sprintf("Make record from %d columns", numCols))

		// OP_Insert: Insert record into table
		prog.add(OpInsert, cursorNum, regRecord, regRowid, nil, 0,
			fmt.Sprintf("Insert row %d", rowIdx))
	}

	// OP_Close: Close table cursor
	prog.add(OpClose, cursorNum, 0, 0, nil, 0,
		fmt.Sprintf("Close table %s", stmt.Table))

	// Set end label to current position
	endLabel = len(prog.Instructions)

	// Update Init instruction to jump to end
	prog.Instructions[0].P2 = endLabel

	// OP_Halt: End of program
	prog.add(OpHalt, 0, 0, 0, nil, 0, "End program")

	return prog, nil
}

// add appends an instruction to the program
func (p *Program) add(op OpCode, p1, p2, p3 int, p4 interface{}, p5 int, comment string) {
	p.Instructions = append(p.Instructions, Instruction{
		OpCode:  op,
		P1:      p1,
		P2:      p2,
		P3:      p3,
		P4:      p4,
		P5:      p5,
		Comment: comment,
	})
}

// allocReg allocates a new register
func (p *Program) allocReg() int {
	reg := p.NumRegisters
	p.NumRegisters++
	return reg
}

// allocRegs allocates n consecutive registers
func (p *Program) allocRegs(n int) int {
	reg := p.NumRegisters
	p.NumRegisters += n
	return reg
}

// addValueLoad adds instructions to load a value into a register
func (p *Program) addValueLoad(val Value, reg int) error {
	switch val.Type {
	case TypeNull:
		p.add(OpNull, 0, reg, 0, nil, 0, "Load NULL")
		return nil

	case TypeInteger:
		p.add(OpInteger, int(val.Int), reg, 0, nil, 0,
			fmt.Sprintf("Load integer %d", val.Int))
		return nil

	case TypeFloat:
		p.add(OpReal, 0, reg, 0, val.Float, 0,
			fmt.Sprintf("Load float %f", val.Float))
		return nil

	case TypeText:
		p.add(OpString, 0, reg, 0, val.Text, 0,
			fmt.Sprintf("Load string '%s'", val.Text))
		return nil

	case TypeBlob:
		p.add(OpBlob, len(val.Blob), reg, 0, val.Blob, 0,
			fmt.Sprintf("Load blob (%d bytes)", len(val.Blob)))
		return nil

	default:
		return fmt.Errorf("unsupported value type: %v", val.Type)
	}
}

// Disassemble returns a human-readable representation of the program
func (p *Program) Disassemble() string {
	result := fmt.Sprintf("Program: %d instructions, %d registers, %d cursors\n",
		len(p.Instructions), p.NumRegisters, p.NumCursors)
	result += fmt.Sprintf("%-4s %-12s %-4s %-4s %-4s %-8s %-4s %s\n",
		"Addr", "Opcode", "P1", "P2", "P3", "P4", "P5", "Comment")
	result += "--------------------------------------------------------------------------------\n"

	for i, inst := range p.Instructions {
		p4str := ""
		if inst.P4 != nil {
			switch v := inst.P4.(type) {
			case string:
				if len(v) > 20 {
					p4str = fmt.Sprintf("'%.17s...'", v)
				} else {
					p4str = fmt.Sprintf("'%s'", v)
				}
			case float64:
				p4str = fmt.Sprintf("%.6f", v)
			case []byte:
				p4str = fmt.Sprintf("<blob:%d>", len(v))
			default:
				p4str = fmt.Sprintf("%v", v)
			}
		}

		result += fmt.Sprintf("%-4d %-12s %-4d %-4d %-4d %-8s %-4d %s\n",
			i, inst.OpCode.String(), inst.P1, inst.P2, inst.P3,
			p4str, inst.P5, inst.Comment)
	}

	return result
}

// CompileInsertWithAutoInc compiles an INSERT with auto-increment support
func CompileInsertWithAutoInc(stmt *InsertStmt, tableRoot int, hasAutoInc bool) (*Program, error) {
	prog, err := CompileInsert(stmt, tableRoot)
	if err != nil {
		return nil, err
	}

	if hasAutoInc {
		// Add auto-increment handling
		// This would involve reading/updating sqlite_sequence table
		// For now, we'll just use the basic NewRowid which handles this
		// in the actual VDBE implementation
	}

	return prog, nil
}

// ValidateInsert performs validation on an INSERT statement
func ValidateInsert(stmt *InsertStmt) error {
	if stmt == nil {
		return errors.New("nil insert statement")
	}

	if stmt.Table == "" {
		return errors.New("table name is required")
	}

	if len(stmt.Values) == 0 {
		return errors.New("no values to insert")
	}

	// Check that all rows have the same number of columns
	numCols := len(stmt.Columns)
	if numCols == 0 && len(stmt.Values) > 0 {
		numCols = len(stmt.Values[0])
	}

	for i, row := range stmt.Values {
		if len(row) != numCols {
			return fmt.Errorf("row %d has %d values, expected %d",
				i, len(row), numCols)
		}
	}

	return nil
}

// NewInsertStmt creates a new INSERT statement
func NewInsertStmt(table string, columns []string, values [][]Value) *InsertStmt {
	return &InsertStmt{
		Table:   table,
		Columns: columns,
		Values:  values,
	}
}
