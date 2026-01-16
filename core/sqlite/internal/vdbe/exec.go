package vdbe

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/btree"
)

// Step executes one step of the VDBE program.
// Returns true if there are more steps to execute, false if halted.
func (v *VDBE) Step() (bool, error) {
	// Check state
	if v.State == StateHalt {
		return false, fmt.Errorf("VDBE is halted")
	}

	if v.State == StateInit {
		v.State = StateReady
	}

	if v.State == StateReady {
		v.PC = 0
		v.State = StateRun
	}

	// If a row is ready, clear it and continue execution
	if v.State == StateRowReady {
		v.ResultRow = nil
		v.State = StateRun
	}

	// Check if we're at the end of the program
	if v.PC >= len(v.Program) {
		v.State = StateHalt
		return false, nil
	}

	// Get the current instruction
	instr := v.Program[v.PC]
	v.PC++
	v.NumSteps++

	// Execute the instruction
	err := v.execInstruction(instr)
	if err != nil {
		v.SetError(err.Error())
		v.State = StateHalt
		return false, err
	}

	// Check if we halted
	if v.State == StateHalt {
		return false, nil
	}

	// Check if a row is ready - this pauses execution
	if v.State == StateRowReady {
		return true, nil
	}

	return true, nil
}

// Run executes the entire VDBE program until completion.
func (v *VDBE) Run() error {
	for {
		hasMore, err := v.Step()
		if err != nil {
			return err
		}
		if !hasMore {
			break
		}
	}
	return nil
}

// execInstruction executes a single instruction.
func (v *VDBE) execInstruction(instr *Instruction) error {
	switch instr.Opcode {

	// Control flow opcodes
	case OpInit:
		return v.execInit(instr)
	case OpGoto:
		return v.execGoto(instr)
	case OpGosub:
		return v.execGosub(instr)
	case OpReturn:
		return v.execReturn(instr)
	case OpHalt:
		return v.execHalt(instr)
	case OpIf:
		return v.execIf(instr)
	case OpIfNot:
		return v.execIfNot(instr)

	// Register operations
	case OpInteger:
		return v.execInteger(instr)
	case OpInt64:
		return v.execInt64(instr)
	case OpReal:
		return v.execReal(instr)
	case OpString, OpString8:
		return v.execString(instr)
	case OpBlob:
		return v.execBlob(instr)
	case OpNull:
		return v.execNull(instr)
	case OpCopy:
		return v.execCopy(instr)
	case OpMove:
		return v.execMove(instr)
	case OpSCopy:
		return v.execSCopy(instr)

	// Cursor operations
	case OpOpenRead:
		return v.execOpenRead(instr)
	case OpOpenWrite:
		return v.execOpenWrite(instr)
	case OpClose:
		return v.execClose(instr)
	case OpRewind:
		return v.execRewind(instr)
	case OpNext:
		return v.execNext(instr)
	case OpPrev:
		return v.execPrev(instr)
	case OpSeekGE:
		return v.execSeekGE(instr)
	case OpSeekLE:
		return v.execSeekLE(instr)
	case OpSeekRowid:
		return v.execSeekRowid(instr)

	// Data retrieval
	case OpColumn:
		return v.execColumn(instr)
	case OpRowid:
		return v.execRowid(instr)
	case OpResultRow:
		return v.execResultRow(instr)

	// Data modification
	case OpNewRowid:
		return v.execNewRowid(instr)
	case OpMakeRecord:
		return v.execMakeRecord(instr)
	case OpInsert:
		return v.execInsert(instr)
	case OpDelete:
		return v.execDelete(instr)

	// Comparison
	case OpEq:
		return v.execEq(instr)
	case OpNe:
		return v.execNe(instr)
	case OpLt:
		return v.execLt(instr)
	case OpLe:
		return v.execLe(instr)
	case OpGt:
		return v.execGt(instr)
	case OpGe:
		return v.execGe(instr)

	// Arithmetic
	case OpAdd:
		return v.execAdd(instr)
	case OpSubtract:
		return v.execSubtract(instr)
	case OpMultiply:
		return v.execMultiply(instr)
	case OpDivide:
		return v.execDivide(instr)
	case OpRemainder:
		return v.execRemainder(instr)

	// Aggregate functions
	case OpAggStep:
		return v.execAggStep(instr)
	case OpAggFinal:
		return v.execAggFinal(instr)

	// Function calls
	case OpFunction:
		return v.execFunction(instr)

	case OpNoop:
		// No operation
		return nil

	default:
		return fmt.Errorf("unimplemented opcode: %s", instr.Opcode.String())
	}
}

// Control flow instruction implementations

func (v *VDBE) execInit(instr *Instruction) error {
	// Initialize the program - typically jumps to P2
	if instr.P2 > 0 {
		v.PC = instr.P2
	}
	return nil
}

func (v *VDBE) execGoto(instr *Instruction) error {
	// Unconditional jump to P2
	if instr.P2 < 0 || instr.P2 >= len(v.Program) {
		return fmt.Errorf("invalid jump address: %d", instr.P2)
	}
	v.PC = instr.P2
	return nil
}

func (v *VDBE) execGosub(instr *Instruction) error {
	// Save return address in P1, jump to P2
	mem, err := v.GetMem(instr.P1)
	if err != nil {
		return err
	}
	mem.SetInt(int64(v.PC))
	v.PC = instr.P2
	return nil
}

func (v *VDBE) execReturn(instr *Instruction) error {
	// Jump to address stored in P1
	mem, err := v.GetMem(instr.P1)
	if err != nil {
		return err
	}
	if mem.IsInt() {
		v.PC = int(mem.IntValue())
	}
	return nil
}

func (v *VDBE) execHalt(instr *Instruction) error {
	// Halt execution
	v.State = StateHalt
	v.RC = instr.P1
	if instr.P4Type == P4Static || instr.P4Type == P4Dynamic {
		v.SetError(instr.P4.Z)
	}
	return nil
}

func (v *VDBE) execIf(instr *Instruction) error {
	// Jump to P2 if P1 is true
	mem, err := v.GetMem(instr.P1)
	if err != nil {
		return err
	}

	isTrue := false
	if !mem.IsNull() {
		if mem.IsInt() {
			isTrue = mem.IntValue() != 0
		} else {
			isTrue = mem.RealValue() != 0.0
		}
	}

	if isTrue {
		v.PC = instr.P2
	}
	return nil
}

func (v *VDBE) execIfNot(instr *Instruction) error {
	// Jump to P2 if P1 is false
	mem, err := v.GetMem(instr.P1)
	if err != nil {
		return err
	}

	isFalse := mem.IsNull()
	if !isFalse {
		if mem.IsInt() {
			isFalse = mem.IntValue() == 0
		} else {
			isFalse = mem.RealValue() == 0.0
		}
	}

	if isFalse {
		v.PC = instr.P2
	}
	return nil
}

// Register operation implementations

func (v *VDBE) execInteger(instr *Instruction) error {
	// Store integer P1 in register P2
	mem, err := v.GetMem(instr.P2)
	if err != nil {
		return err
	}
	mem.SetInt(int64(instr.P1))
	return nil
}

func (v *VDBE) execInt64(instr *Instruction) error {
	// Store 64-bit integer P4 in register P2
	mem, err := v.GetMem(instr.P2)
	if err != nil {
		return err
	}
	if instr.P4Type != P4Int64 {
		return fmt.Errorf("expected P4_INT64 for Int64 opcode")
	}
	mem.SetInt(instr.P4.I64)
	return nil
}

func (v *VDBE) execReal(instr *Instruction) error {
	// Store real P4 in register P2
	mem, err := v.GetMem(instr.P2)
	if err != nil {
		return err
	}
	if instr.P4Type != P4Real {
		return fmt.Errorf("expected P4_REAL for Real opcode")
	}
	mem.SetReal(instr.P4.R)
	return nil
}

func (v *VDBE) execString(instr *Instruction) error {
	// Store string P4 in register P2
	mem, err := v.GetMem(instr.P2)
	if err != nil {
		return err
	}
	if instr.P4Type != P4Static && instr.P4Type != P4Dynamic {
		return fmt.Errorf("expected P4_STATIC or P4_DYNAMIC for String opcode")
	}
	mem.SetStr(instr.P4.Z)
	return nil
}

func (v *VDBE) execBlob(instr *Instruction) error {
	// Store blob P4 in register P2
	mem, err := v.GetMem(instr.P2)
	if err != nil {
		return err
	}
	// P4 contains the blob data
	if instr.P4.P != nil {
		if blob, ok := instr.P4.P.([]byte); ok {
			mem.SetBlob(blob)
		} else {
			return fmt.Errorf("P4 is not a byte slice for Blob opcode")
		}
	}
	return nil
}

func (v *VDBE) execNull(instr *Instruction) error {
	// Store NULL in registers P2 through P2+P3
	for i := instr.P2; i <= instr.P2+instr.P3; i++ {
		mem, err := v.GetMem(i)
		if err != nil {
			return err
		}
		mem.SetNull()
	}
	return nil
}

func (v *VDBE) execCopy(instr *Instruction) error {
	// Copy register P1 to register P2
	src, err := v.GetMem(instr.P1)
	if err != nil {
		return err
	}
	dst, err := v.GetMem(instr.P2)
	if err != nil {
		return err
	}
	return dst.Copy(src)
}

func (v *VDBE) execMove(instr *Instruction) error {
	// Move P3 registers starting at P1 to starting at P2
	count := instr.P3
	if count <= 0 {
		count = 1
	}

	for i := 0; i < count; i++ {
		src, err := v.GetMem(instr.P1 + i)
		if err != nil {
			return err
		}
		dst, err := v.GetMem(instr.P2 + i)
		if err != nil {
			return err
		}
		dst.Move(src)
	}
	return nil
}

func (v *VDBE) execSCopy(instr *Instruction) error {
	// Shallow copy register P1 to register P2
	src, err := v.GetMem(instr.P1)
	if err != nil {
		return err
	}
	dst, err := v.GetMem(instr.P2)
	if err != nil {
		return err
	}
	dst.ShallowCopy(src)
	return nil
}

// Cursor operation implementations

func (v *VDBE) execOpenRead(instr *Instruction) error {
	// Open cursor P1 for reading on root page P2
	// P1 = cursor number, P2 = root page, P3 = num columns
	if v.Ctx == nil || v.Ctx.Btree == nil {
		return fmt.Errorf("no btree context available")
	}

	bt, ok := v.Ctx.Btree.(*btree.Btree)
	if !ok {
		return fmt.Errorf("invalid btree context type")
	}

	// Allocate cursors if needed
	if err := v.AllocCursors(instr.P1 + 1); err != nil {
		return err
	}

	// Create btree cursor
	btCursor := btree.NewCursor(bt, uint32(instr.P2))

	// Create VDBE cursor
	cursor := &Cursor{
		CurType:     CursorBTree,
		IsTable:     true,
		RootPage:    uint32(instr.P2),
		BtreeCursor: btCursor,
		CachedCols:  make([][]byte, 0),
		CacheStatus: 0,
	}

	v.Cursors[instr.P1] = cursor
	return nil
}

func (v *VDBE) execOpenWrite(instr *Instruction) error {
	// Open cursor P1 for writing on root page P2
	// P1 = cursor number, P2 = root page, P3 = num columns
	if v.Ctx == nil || v.Ctx.Btree == nil {
		return fmt.Errorf("no btree context available")
	}

	bt, ok := v.Ctx.Btree.(*btree.Btree)
	if !ok {
		return fmt.Errorf("invalid btree context type")
	}

	// Allocate cursors if needed
	if err := v.AllocCursors(instr.P1 + 1); err != nil {
		return err
	}

	// Create btree cursor (same as read cursor - btree cursors support both read and write)
	btCursor := btree.NewCursor(bt, uint32(instr.P2))

	// Create VDBE cursor with writable flag set
	cursor := &Cursor{
		CurType:     CursorBTree,
		IsTable:     true,
		Writable:    true, // Mark as writable for write operations
		RootPage:    uint32(instr.P2),
		BtreeCursor: btCursor,
		CachedCols:  make([][]byte, 0),
		CacheStatus: 0,
	}

	v.Cursors[instr.P1] = cursor
	return nil
}

func (v *VDBE) execClose(instr *Instruction) error {
	// Close cursor P1
	if instr.P1 < 0 || instr.P1 >= len(v.Cursors) {
		return fmt.Errorf("cursor index %d out of range", instr.P1)
	}

	cursor := v.Cursors[instr.P1]
	if cursor != nil {
		// Clear any btree cursor
		cursor.BtreeCursor = nil
		cursor.CurrentKey = nil
		cursor.CurrentVal = nil
		cursor.CachedCols = nil
	}

	v.Cursors[instr.P1] = nil
	return nil
}

func (v *VDBE) execRewind(instr *Instruction) error {
	// Rewind cursor P1 to the beginning, jump to P2 if empty
	cursor, err := v.GetCursor(instr.P1)
	if err != nil {
		return err
	}

	// Reset state
	cursor.EOF = false
	cursor.NullRow = false
	cursor.CurrentKey = nil
	cursor.CurrentVal = nil
	v.IncrCacheCtr() // Invalidate column cache

	// Get btree cursor
	btCursor, ok := cursor.BtreeCursor.(*btree.BtCursor)
	if !ok || btCursor == nil {
		// Empty table or invalid cursor - jump to P2
		if instr.P2 > 0 {
			v.PC = instr.P2
		}
		cursor.EOF = true
		return nil
	}

	// Move to first entry
	err = btCursor.MoveToFirst()
	if err != nil {
		// Empty table or error - jump to P2
		if instr.P2 > 0 {
			v.PC = instr.P2
		}
		cursor.EOF = true
		return nil
	}

	// Successfully positioned at first entry
	cursor.EOF = false
	return nil
}

func (v *VDBE) execNext(instr *Instruction) error {
	// Move cursor P1 to next entry, jump to P2 if more rows exist
	cursor, err := v.GetCursor(instr.P1)
	if err != nil {
		return err
	}

	// Get btree cursor
	btCursor, ok := cursor.BtreeCursor.(*btree.BtCursor)
	if !ok || btCursor == nil {
		cursor.EOF = true
		return nil
	}

	// Invalidate column cache
	v.IncrCacheCtr()

	// Move to next entry
	err = btCursor.Next()
	if err != nil {
		// Reached end of btree or error
		cursor.EOF = true
		return nil
	}

	// Successfully moved to next entry - jump to P2
	cursor.EOF = false
	v.PC = instr.P2

	return nil
}

func (v *VDBE) execPrev(instr *Instruction) error {
	// Move cursor P1 to previous entry, jump to P2 if successful
	cursor, err := v.GetCursor(instr.P1)
	if err != nil {
		return err
	}

	// Get btree cursor
	btCursor, ok := cursor.BtreeCursor.(*btree.BtCursor)
	if !ok || btCursor == nil {
		cursor.EOF = true
		return nil
	}

	// Invalidate column cache
	v.IncrCacheCtr()

	// Move to previous entry
	err = btCursor.Previous()
	if err != nil {
		// Reached beginning of btree or error
		cursor.EOF = true
		return nil
	}

	// Successfully moved to previous entry - jump to P2
	cursor.EOF = false
	v.PC = instr.P2

	return nil
}

func (v *VDBE) execSeekGE(instr *Instruction) error {
	// Seek cursor P1 to entry >= key in register P3
	cursor, err := v.GetCursor(instr.P1)
	if err != nil {
		return err
	}

	keyReg, err := v.GetMem(instr.P3)
	if err != nil {
		return err
	}

	// In a real implementation, this would perform a B-tree seek
	_ = keyReg
	cursor.EOF = false

	return nil
}

func (v *VDBE) execSeekLE(instr *Instruction) error {
	// Seek cursor P1 to entry <= key in register P3
	cursor, err := v.GetCursor(instr.P1)
	if err != nil {
		return err
	}

	keyReg, err := v.GetMem(instr.P3)
	if err != nil {
		return err
	}

	_ = keyReg
	cursor.EOF = false

	return nil
}

func (v *VDBE) execSeekRowid(instr *Instruction) error {
	// Seek cursor P1 to rowid in register P3, jump to P2 if not found
	cursor, err := v.GetCursor(instr.P1)
	if err != nil {
		return err
	}

	// Get the target rowid
	rowidReg, err := v.GetMem(instr.P3)
	if err != nil {
		return err
	}

	targetRowid := rowidReg.IntValue()

	// Get btree cursor
	btCursor, ok := cursor.BtreeCursor.(*btree.BtCursor)
	if !ok || btCursor == nil {
		// No cursor - not found, jump to P2
		if instr.P2 > 0 {
			v.PC = instr.P2
		}
		cursor.EOF = true
		return nil
	}

	// Move to first entry
	err = btCursor.MoveToFirst()
	if err != nil {
		// Empty tree - not found, jump to P2
		if instr.P2 > 0 {
			v.PC = instr.P2
		}
		cursor.EOF = true
		return nil
	}

	// Linear search through the btree to find matching rowid
	// In a real implementation, this would use a binary search or btree search
	for {
		currentRowid := btCursor.GetKey()
		if currentRowid == targetRowid {
			// Found it - don't jump
			cursor.EOF = false
			v.IncrCacheCtr()
			return nil
		}

		if currentRowid > targetRowid {
			// Passed it - not found
			if instr.P2 > 0 {
				v.PC = instr.P2
			}
			cursor.EOF = true
			return nil
		}

		// Move to next entry
		err = btCursor.Next()
		if err != nil {
			// Reached end - not found, jump to P2
			if instr.P2 > 0 {
				v.PC = instr.P2
			}
			cursor.EOF = true
			return nil
		}
	}
}

// Data retrieval implementations

func (v *VDBE) execColumn(instr *Instruction) error {
	// Read column P2 from cursor P1 into register P3
	cursor, err := v.GetCursor(instr.P1)
	if err != nil {
		return err
	}

	dst, err := v.GetMem(instr.P3)
	if err != nil {
		return err
	}

	// Check for null row or EOF
	if cursor.NullRow || cursor.EOF {
		dst.SetNull()
		return nil
	}

	// Get btree cursor
	btCursor, ok := cursor.BtreeCursor.(*btree.BtCursor)
	if !ok || btCursor == nil {
		dst.SetNull()
		return nil
	}

	// Get payload from current cell
	payload := btCursor.GetPayload()
	if payload == nil {
		dst.SetNull()
		return nil
	}

	// Parse the record to extract columns
	colIndex := instr.P2
	err = parseRecordColumn(payload, colIndex, dst)
	if err != nil {
		return fmt.Errorf("failed to parse record column: %w", err)
	}

	return nil
}

func (v *VDBE) execRowid(instr *Instruction) error {
	// Get rowid from cursor P1 into register P2
	cursor, err := v.GetCursor(instr.P1)
	if err != nil {
		return err
	}

	dst, err := v.GetMem(instr.P2)
	if err != nil {
		return err
	}

	// Check for null row or EOF
	if cursor.NullRow || cursor.EOF {
		dst.SetNull()
		return nil
	}

	// Get btree cursor
	btCursor, ok := cursor.BtreeCursor.(*btree.BtCursor)
	if !ok || btCursor == nil {
		dst.SetNull()
		return nil
	}

	// Get rowid (key) from current cell
	rowid := btCursor.GetKey()
	dst.SetInt(rowid)

	return nil
}

// parseRecordColumn parses a specific column from a SQLite record
// This is a simplified version to avoid import cycles with the sql package
func parseRecordColumn(data []byte, colIndex int, dst *Mem) error {
	if len(data) == 0 {
		dst.SetNull()
		return nil
	}

	// Read header size (varint)
	headerSize, n := getVarint(data, 0)
	if n == 0 {
		dst.SetNull()
		return fmt.Errorf("invalid header size")
	}

	offset := n

	// Read serial types from header
	serialTypes := make([]uint64, 0)
	for offset < int(headerSize) {
		st, n := getVarint(data, offset)
		if n == 0 {
			dst.SetNull()
			return fmt.Errorf("invalid serial type")
		}
		serialTypes = append(serialTypes, st)
		offset += n
	}

	// Check if column index is valid
	if colIndex < 0 || colIndex >= len(serialTypes) {
		dst.SetNull()
		return nil
	}

	// Skip to the target column in the body
	for i := 0; i < colIndex; i++ {
		offset += serialTypeLen(serialTypes[i])
	}

	// Parse the target column value
	st := serialTypes[colIndex]
	return parseSerialValue(data, offset, st, dst)
}

// getVarint reads a varint from buf starting at offset
func getVarint(buf []byte, offset int) (uint64, int) {
	if offset >= len(buf) {
		return 0, 0
	}

	b := buf[offset]
	if b <= 240 {
		return uint64(b), 1
	}
	if b <= 248 {
		if offset+1 >= len(buf) {
			return 0, 0
		}
		return 240 + 256*uint64(b-241) + uint64(buf[offset+1]), 2
	}
	if b == 249 {
		if offset+2 >= len(buf) {
			return 0, 0
		}
		return 2288 + 256*uint64(buf[offset+1]) + uint64(buf[offset+2]), 3
	}

	// Standard varint decoding
	var v uint64
	n := 0
	for i := offset; i < len(buf) && n < 9; i++ {
		b := buf[i]
		v = (v << 7) | uint64(b&0x7f)
		n++
		if b&0x80 == 0 {
			return v, n
		}
	}
	return v, n
}

// serialTypeLen returns the number of bytes required for a value with the given serial type
func serialTypeLen(serialType uint64) int {
	switch serialType {
	case 0, 8, 9: // NULL, 0, 1
		return 0
	case 1: // INT8
		return 1
	case 2: // INT16
		return 2
	case 3: // INT24
		return 3
	case 4: // INT32
		return 4
	case 5: // INT48
		return 6
	case 6, 7: // INT64, FLOAT64
		return 8
	default:
		if serialType >= 12 {
			return int(serialType-12) / 2
		}
		return 0
	}
}

// parseSerialValue parses a value from the record body
func parseSerialValue(data []byte, offset int, st uint64, mem *Mem) error {
	switch st {
	case 0: // NULL
		mem.SetNull()
		return nil

	case 8: // 0
		mem.SetInt(0)
		return nil

	case 9: // 1
		mem.SetInt(1)
		return nil

	case 1: // INT8
		if offset >= len(data) {
			mem.SetNull()
			return fmt.Errorf("truncated int8")
		}
		mem.SetInt(int64(int8(data[offset])))
		return nil

	case 2: // INT16
		if offset+2 > len(data) {
			mem.SetNull()
			return fmt.Errorf("truncated int16")
		}
		v := int64(int16(binary.BigEndian.Uint16(data[offset:])))
		mem.SetInt(v)
		return nil

	case 3: // INT24
		if offset+3 > len(data) {
			mem.SetNull()
			return fmt.Errorf("truncated int24")
		}
		v := int32(data[offset])<<16 | int32(data[offset+1])<<8 | int32(data[offset+2])
		if v&0x800000 != 0 {
			v |= ^0xffffff // Sign extend
		}
		mem.SetInt(int64(v))
		return nil

	case 4: // INT32
		if offset+4 > len(data) {
			mem.SetNull()
			return fmt.Errorf("truncated int32")
		}
		v := int64(int32(binary.BigEndian.Uint32(data[offset:])))
		mem.SetInt(v)
		return nil

	case 5: // INT48
		if offset+6 > len(data) {
			mem.SetNull()
			return fmt.Errorf("truncated int48")
		}
		v := int64(data[offset])<<40 | int64(data[offset+1])<<32 |
			int64(data[offset+2])<<24 | int64(data[offset+3])<<16 |
			int64(data[offset+4])<<8 | int64(data[offset+5])
		if v&0x800000000000 != 0 {
			v |= ^0xffffffffffff // Sign extend
		}
		mem.SetInt(v)
		return nil

	case 6: // INT64
		if offset+8 > len(data) {
			mem.SetNull()
			return fmt.Errorf("truncated int64")
		}
		v := int64(binary.BigEndian.Uint64(data[offset:]))
		mem.SetInt(v)
		return nil

	case 7: // FLOAT64
		if offset+8 > len(data) {
			mem.SetNull()
			return fmt.Errorf("truncated float64")
		}
		bits := binary.BigEndian.Uint64(data[offset:])
		v := math.Float64frombits(bits)
		mem.SetReal(v)
		return nil

	default:
		// Blob or Text
		length := serialTypeLen(st)
		if offset+length > len(data) {
			mem.SetNull()
			return fmt.Errorf("truncated blob/text")
		}

		b := make([]byte, length)
		copy(b, data[offset:offset+length])

		if st%2 == 0 {
			// Even: BLOB
			mem.SetBlob(b)
			return nil
		} else {
			// Odd: TEXT
			mem.SetStr(string(b))
			return nil
		}
	}
}

func (v *VDBE) execResultRow(instr *Instruction) error {
	// Copy values from registers P1..P1+P2-1 to ResultRow
	// This makes a row of data available to the driver
	v.ResultRow = make([]*Mem, instr.P2)
	for i := 0; i < instr.P2; i++ {
		mem, err := v.GetMem(instr.P1 + i)
		if err != nil {
			return err
		}
		// Make a copy for the result
		v.ResultRow[i] = NewMem()
		v.ResultRow[i].Copy(mem)
	}

	// Set state to StateRowReady to pause execution and allow the driver to read this row
	// The Run() loop will pause here, and the driver's Next() method can read ResultRow
	// When Step() is called again, it will clear ResultRow and continue execution
	v.State = StateRowReady
	return nil
}

// Data modification implementations

func (v *VDBE) execNewRowid(instr *Instruction) error {
	// Generate a new rowid for cursor P1, store in register P3
	// P1 = cursor number, P3 = destination register
	cursor, err := v.GetCursor(instr.P1)
	if err != nil {
		return err
	}

	// Get the btree to find max rowid
	bt, ok := v.Ctx.Btree.(*btree.Btree)
	if !ok {
		return fmt.Errorf("invalid btree context type")
	}

	// Generate new rowid by finding the max rowid in the table
	newRowid, err := bt.NewRowid(cursor.RootPage)
	if err != nil {
		// If table is empty, NewRowid returns an error, so start with 1
		newRowid = 1
	}

	// Store the new rowid
	cursor.LastRowid = newRowid

	// Store the new rowid in register P3
	mem, err := v.GetMem(instr.P3)
	if err != nil {
		return err
	}
	mem.SetInt(newRowid)

	return nil
}

func (v *VDBE) execMakeRecord(instr *Instruction) error {
	// Create a record from registers P1..P1+P2-1 and store in register P3
	// P1 = first register, P2 = number of registers, P3 = destination
	startReg := instr.P1
	numFields := instr.P2
	destReg := instr.P3

	// Collect values from registers
	values := make([]interface{}, numFields)
	for i := 0; i < numFields; i++ {
		mem, err := v.GetMem(startReg + i)
		if err != nil {
			values[i] = nil
			continue
		}
		values[i] = memToInterface(mem)
	}

	// Create a simple record representation
	// In a full implementation, this would encode according to SQLite record format
	mem, err := v.GetMem(destReg)
	if err != nil {
		return err
	}
	mem.SetBlob(encodeSimpleRecord(values))

	return nil
}

// memToInterface converts a Mem value to a Go interface{}
func memToInterface(m *Mem) interface{} {
	if m == nil || m.IsNull() {
		return nil
	}
	if m.IsInt() {
		return m.IntValue()
	}
	if m.IsReal() {
		return m.RealValue()
	}
	if m.IsString() {
		return m.StrValue()
	}
	if m.IsBlob() {
		return m.BlobValue()
	}
	return nil
}

// encodeSimpleRecord creates a SQLite record encoding
// SQLite record format:
// - Header size (varint)
// - Serial type for each column (varint)
// - Data for each column
func encodeSimpleRecord(values []interface{}) []byte {
	if len(values) == 0 {
		return []byte{0} // Empty record
	}

	// Build header and data separately
	var header []byte
	var data []byte

	// Reserve space for header size (will update later)
	header = append(header, 0)

	// Process each value
	for _, val := range values {
		var serialType uint64
		var valueData []byte

		switch v := val.(type) {
		case nil:
			// NULL - serial type 0
			serialType = 0

		case int64:
			// Integer - choose appropriate serial type based on value
			if v == 0 {
				serialType = 8 // 0 as special serial type
			} else if v == 1 {
				serialType = 9 // 1 as special serial type
			} else if v >= -128 && v <= 127 {
				serialType = 1 // INT8
				valueData = []byte{byte(v)}
			} else if v >= -32768 && v <= 32767 {
				serialType = 2 // INT16
				valueData = make([]byte, 2)
				binary.BigEndian.PutUint16(valueData, uint16(v))
			} else if v >= -8388608 && v <= 8388607 {
				serialType = 3 // INT24
				valueData = make([]byte, 3)
				valueData[0] = byte(v >> 16)
				valueData[1] = byte(v >> 8)
				valueData[2] = byte(v)
			} else if v >= -2147483648 && v <= 2147483647 {
				serialType = 4 // INT32
				valueData = make([]byte, 4)
				binary.BigEndian.PutUint32(valueData, uint32(v))
			} else {
				serialType = 6 // INT64
				valueData = make([]byte, 8)
				binary.BigEndian.PutUint64(valueData, uint64(v))
			}

		case float64:
			// REAL - serial type 7
			serialType = 7
			valueData = make([]byte, 8)
			bits := math.Float64bits(v)
			binary.BigEndian.PutUint64(valueData, bits)

		case string:
			// TEXT - serial type 2*len+13 (odd number)
			valueData = []byte(v)
			serialType = uint64(2*len(valueData) + 13)

		case []byte:
			// BLOB - serial type 2*len+12 (even number)
			valueData = v
			serialType = uint64(2*len(valueData) + 12)

		default:
			// Unsupported type - treat as NULL
			serialType = 0
		}

		// Append serial type to header
		header = appendVarint(header, serialType)

		// Append value data to data section
		data = append(data, valueData...)
	}

	// Calculate final header size
	headerSize := len(header)

	// Encode header size as varint
	headerSizeVarint := encodeVarint(uint64(headerSize))

	// Build final record: header size + serial types + data
	record := make([]byte, 0, len(headerSizeVarint)+len(header)-1+len(data))
	record = append(record, headerSizeVarint...)
	record = append(record, header[1:]...) // Skip the placeholder byte
	record = append(record, data...)

	return record
}

// encodeVarint encodes a uint64 as a varint and returns the bytes
func encodeVarint(v uint64) []byte {
	if v <= 240 {
		return []byte{byte(v)}
	}
	if v <= 2287 {
		v -= 240
		return []byte{byte(v/256 + 241), byte(v % 256)}
	}
	if v <= 67823 {
		v -= 2288
		return []byte{249, byte(v / 256), byte(v % 256)}
	}

	// Standard varint encoding for larger values
	var buf []byte
	for i := 0; i < 9; i++ {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf = append([]byte{b}, buf...) // Prepend
		if v == 0 {
			break
		}
	}
	return buf
}

// appendVarint appends a varint to a byte slice
func appendVarint(buf []byte, v uint64) []byte {
	return append(buf, encodeVarint(v)...)
}

func (v *VDBE) execInsert(instr *Instruction) error {
	// Insert record from register P2 into cursor P1
	// P1 = cursor number
	// P2 = register containing record data (blob)
	// P3 = register containing rowid (or 0 to use cursor.LastRowid)
	cursor, err := v.GetCursor(instr.P1)
	if err != nil {
		return err
	}

	// Verify cursor is writable
	if !cursor.Writable {
		return fmt.Errorf("cursor %d is not writable (opened with OpenRead instead of OpenWrite)", instr.P1)
	}

	// Get the btree cursor
	btCursor, ok := cursor.BtreeCursor.(*btree.BtCursor)
	if !ok || btCursor == nil {
		return fmt.Errorf("invalid btree cursor for insert")
	}

	// Get the record data from register P2
	data, err := v.GetMem(instr.P2)
	if err != nil {
		return err
	}

	if !data.IsBlob() {
		return fmt.Errorf("insert data must be a blob")
	}

	payload := data.BlobValue()

	// Get the rowid from register P3 (or use cursor.LastRowid if P3 is 0)
	var rowid int64
	if instr.P3 == 0 {
		rowid = cursor.LastRowid
	} else {
		rowidMem, err := v.GetMem(instr.P3)
		if err != nil {
			return err
		}
		rowid = rowidMem.IntValue()
	}

	// Insert into the btree
	err = btCursor.Insert(rowid, payload)
	if err != nil {
		return fmt.Errorf("btree insert failed: %w", err)
	}

	// Update cursor's LastRowid
	cursor.LastRowid = rowid

	v.NumChanges++
	return nil
}

func (v *VDBE) execDelete(instr *Instruction) error {
	// Delete current record from cursor P1
	cursor, err := v.GetCursor(instr.P1)
	if err != nil {
		return err
	}

	// Verify cursor is writable
	if !cursor.Writable {
		return fmt.Errorf("cursor %d is not writable (opened with OpenRead instead of OpenWrite)", instr.P1)
	}

	// Get the btree cursor
	btCursor, ok := cursor.BtreeCursor.(*btree.BtCursor)
	if !ok || btCursor == nil {
		return fmt.Errorf("invalid btree cursor for delete operation")
	}

	// Delete the current row from the btree
	err = btCursor.Delete()
	if err != nil {
		return fmt.Errorf("failed to delete from btree: %w", err)
	}

	// Invalidate column cache since cursor state changed
	v.IncrCacheCtr()

	// Update change counter
	v.NumChanges++
	return nil
}

// Comparison implementations

func (v *VDBE) execEq(instr *Instruction) error {
	return v.execCompare(instr, func(cmp int) bool { return cmp == 0 })
}

func (v *VDBE) execNe(instr *Instruction) error {
	return v.execCompare(instr, func(cmp int) bool { return cmp != 0 })
}

func (v *VDBE) execLt(instr *Instruction) error {
	return v.execCompare(instr, func(cmp int) bool { return cmp < 0 })
}

func (v *VDBE) execLe(instr *Instruction) error {
	return v.execCompare(instr, func(cmp int) bool { return cmp <= 0 })
}

func (v *VDBE) execGt(instr *Instruction) error {
	return v.execCompare(instr, func(cmp int) bool { return cmp > 0 })
}

func (v *VDBE) execGe(instr *Instruction) error {
	return v.execCompare(instr, func(cmp int) bool { return cmp >= 0 })
}

func (v *VDBE) execCompare(instr *Instruction, test func(int) bool) error {
	// Compare registers P1 and P3, jump to P2 if test passes
	left, err := v.GetMem(instr.P1)
	if err != nil {
		return err
	}

	right, err := v.GetMem(instr.P3)
	if err != nil {
		return err
	}

	cmp := left.Compare(right)
	if test(cmp) {
		v.PC = instr.P2
	}

	return nil
}

// Arithmetic implementations

func (v *VDBE) execAdd(instr *Instruction) error {
	// P3 = P1 + P2
	left, err := v.GetMem(instr.P1)
	if err != nil {
		return err
	}

	right, err := v.GetMem(instr.P2)
	if err != nil {
		return err
	}

	result, err := v.GetMem(instr.P3)
	if err != nil {
		return err
	}

	result.Copy(left)
	return result.Add(right)
}

func (v *VDBE) execSubtract(instr *Instruction) error {
	// P3 = P1 - P2
	left, err := v.GetMem(instr.P1)
	if err != nil {
		return err
	}

	right, err := v.GetMem(instr.P2)
	if err != nil {
		return err
	}

	result, err := v.GetMem(instr.P3)
	if err != nil {
		return err
	}

	result.Copy(left)
	return result.Subtract(right)
}

func (v *VDBE) execMultiply(instr *Instruction) error {
	// P3 = P1 * P2
	left, err := v.GetMem(instr.P1)
	if err != nil {
		return err
	}

	right, err := v.GetMem(instr.P2)
	if err != nil {
		return err
	}

	result, err := v.GetMem(instr.P3)
	if err != nil {
		return err
	}

	result.Copy(left)
	return result.Multiply(right)
}

func (v *VDBE) execDivide(instr *Instruction) error {
	// P3 = P1 / P2
	left, err := v.GetMem(instr.P1)
	if err != nil {
		return err
	}

	right, err := v.GetMem(instr.P2)
	if err != nil {
		return err
	}

	result, err := v.GetMem(instr.P3)
	if err != nil {
		return err
	}

	result.Copy(left)
	return result.Divide(right)
}

func (v *VDBE) execRemainder(instr *Instruction) error {
	// P3 = P1 % P2
	left, err := v.GetMem(instr.P1)
	if err != nil {
		return err
	}

	right, err := v.GetMem(instr.P2)
	if err != nil {
		return err
	}

	result, err := v.GetMem(instr.P3)
	if err != nil {
		return err
	}

	result.Copy(left)
	return result.Remainder(right)
}

// Aggregate function implementations

func (v *VDBE) execAggStep(instr *Instruction) error {
	// Execute aggregate step function
	// P1 = cursor (for grouping context)
	// P2 = first argument register
	// P3 = aggregate function index
	// P4 = function name (string)
	// P5 = number of arguments
	return v.opAggStep(instr.P1, instr.P2, instr.P3, instr.P1, int(instr.P5))
}

func (v *VDBE) execAggFinal(instr *Instruction) error {
	// Execute aggregate finalization function
	// P1 = cursor (for grouping context)
	// P2 = output register
	// P3 = aggregate function index
	return v.opAggFinal(instr.P1, instr.P2, instr.P3)
}

// Function call implementation

func (v *VDBE) execFunction(instr *Instruction) error {
	// Call function with arguments in registers P2...P2+P5-1, store result in P3
	// P1 = constant mask (bit flags for which args are constant)
	// P2 = first argument register
	// P3 = output register
	// P4 = function name (string)
	// P5 = number of arguments
	return v.opFunction(instr.P1, instr.P2, instr.P3, instr.P1, int(instr.P5))
}
