// Package sql implements SQL statement compilation to VDBE bytecode.
package sql

import (
	"fmt"
)

// SelectCompiler compiles SELECT statements to VDBE bytecode.
type SelectCompiler struct {
	parse *Parse
}

// SelectDest describes where SELECT results should go.
type SelectDest struct {
	Dest     SelectDestType // Destination type
	SDParm   int            // First parameter (meaning depends on Dest)
	SDParm2  int            // Second parameter
	AffSdst  string         // Affinity string for SRT_Set destination
	Sdst     int            // Base register to hold results
	NSdst    int            // Number of result columns
}

// SelectDestType defines how to dispose of query results.
type SelectDestType int

const (
	SRT_Output     SelectDestType = iota + 1 // Output to callback/result row
	SRT_Mem                                   // Store result in a memory cell
	SRT_Set                                   // Store results as keys in an index
	SRT_Union                                 // Store results as keys of union
	SRT_Except                                // Remove result from union
	SRT_Exists                                // Store 1 if result is not empty
	SRT_Table                                 // Store results in a table
	SRT_EphemTab                              // Store results in ephemeral table
	SRT_Coroutine                             // Generate a single row of result
	SRT_Fifo                                  // Store results in FIFO queue
	SRT_DistFifo                              // Like SRT_Fifo but distinct
	SRT_Queue                                 // Store results in priority queue
	SRT_DistQueue                             // Like SRT_Queue but distinct
	SRT_Upfrom                                // Store results for UPDATE FROM
)

// DistinctCtx records information about how to process DISTINCT keyword.
type DistinctCtx struct {
	IsTnct    uint8 // 0: Not distinct. 1: DISTINCT  2: DISTINCT and ORDER BY
	ETnctType uint8 // One of WHERE_DISTINCT_* operators
	TabTnct   int   // Ephemeral table used for DISTINCT processing
	AddrTnct  int   // Address of OP_OpenEphemeral opcode for TabTnct
}

// WHERE_DISTINCT_* values for DISTINCT processing
const (
	WHERE_DISTINCT_NOOP      = 0 // No DISTINCT keyword
	WHERE_DISTINCT_UNIQUE    = 1 // DISTINCT can be optimized away
	WHERE_DISTINCT_ORDERED   = 2 // DISTINCT with ORDER BY optimization
	WHERE_DISTINCT_UNORDERED = 3 // DISTINCT requires ephemeral table
)

// SortCtx contains information about ORDER BY or GROUP BY clause.
type SortCtx struct {
	OrderBy        *ExprList // The ORDER BY (or GROUP BY clause)
	NOBSat         int       // Number of ORDER BY terms satisfied by indices
	ECursor        int       // Cursor number for the sorter
	RegReturn      int       // Register holding block-output return address
	LabelBkOut     int       // Start label for the block-output subroutine
	AddrSortIndex  int       // Address of the OP_SorterOpen or OP_OpenEphemeral
	LabelDone      int       // Jump here when done, ex: LIMIT reached
	LabelOBLopt    int       // Jump here when sorter is full
	SortFlags      uint8     // Zero or more SORTFLAG_* bits
	DeferredRowLd  *RowLoadInfo
}

// SORTFLAG_* bit values
const (
	SORTFLAG_UseSorter = 0x01 // Use SorterOpen instead of OpenEphemeral
)

// RowLoadInfo contains information for loading a result row.
type RowLoadInfo struct {
	RegResult      int       // Start of memory holding result
	EcelFlags      uint8     // ExprCodeExprList flags
}

// NewSelectCompiler creates a new SELECT compiler.
func NewSelectCompiler(parse *Parse) *SelectCompiler {
	return &SelectCompiler{
		parse: parse,
	}
}

// InitSelectDest initializes a SelectDest structure.
func InitSelectDest(dest *SelectDest, destType SelectDestType, parm int) {
	dest.Dest = destType
	dest.SDParm = parm
	dest.SDParm2 = 0
	dest.AffSdst = ""
	dest.Sdst = 0
	dest.NSdst = 0
}

// CompileSelect compiles a SELECT statement to VDBE bytecode.
// This is the main entry point for SELECT compilation.
func (c *SelectCompiler) CompileSelect(sel *Select, dest *SelectDest) error {
	if sel == nil {
		return fmt.Errorf("nil SELECT statement")
	}

	// Handle compound SELECT (UNION, INTERSECT, EXCEPT)
	if sel.Prior != nil {
		return c.compileCompoundSelect(sel, dest)
	}

	// Simple SELECT
	return c.compileSimpleSelect(sel, dest)
}

// compileSimpleSelect compiles a single (non-compound) SELECT statement.
func (c *SelectCompiler) compileSimpleSelect(sel *Select, dest *SelectDest) error {
	vdbe := c.parse.GetVdbe()
	if vdbe == nil {
		return fmt.Errorf("no VDBE available")
	}

	// Initialize output destination
	if dest.Sdst == 0 {
		dest.Sdst = c.parse.AllocRegs(sel.EList.Len())
		dest.NSdst = sel.EList.Len()
	}

	// Generate prologue
	addrEnd := vdbe.MakeLabel()

	// Process FROM clause - open table cursors
	if err := c.processFromClause(sel); err != nil {
		return err
	}

	// Set up DISTINCT handling if needed
	var distinct DistinctCtx
	if sel.SelFlags&SF_Distinct != 0 {
		c.setupDistinct(sel, &distinct)
	}

	// Set up ORDER BY handling if needed
	var sort SortCtx
	hasOrderBy := sel.OrderBy != nil && sel.OrderBy.Len() > 0
	if hasOrderBy {
		if err := c.setupOrderBy(sel, &sort); err != nil {
			return err
		}
	}

	// Generate loop labels
	addrBreak := vdbe.MakeLabel()
	addrContinue := vdbe.MakeLabel()

	// Generate WHERE clause code
	if sel.Where != nil {
		c.compileWhereClause(sel.Where, addrContinue)
	}

	// Process GROUP BY and aggregates if present
	if sel.GroupBy != nil && sel.GroupBy.Len() > 0 {
		return c.compileGroupBy(sel, dest)
	}

	// Generate inner loop code (extract result columns)
	c.selectInnerLoop(sel, -1, &sort, &distinct, dest, addrContinue, addrBreak)

	// Generate ORDER BY sorting code
	if hasOrderBy {
		c.generateSortTail(sel, &sort, sel.EList.Len(), dest)
	}

	// Apply LIMIT/OFFSET
	if sel.Limit != 0 {
		c.applyLimit(sel, dest, addrBreak)
	}

	// Generate epilogue
	vdbe.ResolveLabel(addrBreak)
	vdbe.ResolveLabel(addrEnd)

	return nil
}

// processFromClause processes the FROM clause and opens necessary table cursors.
func (c *SelectCompiler) processFromClause(sel *Select) error {
	if sel.Src == nil || sel.Src.Len() == 0 {
		return nil // No tables (e.g., SELECT 1+1)
	}

	vdbe := c.parse.GetVdbe()

	for i := 0; i < sel.Src.Len(); i++ {
		srcItem := sel.Src.Get(i)
		if srcItem.Table == nil {
			continue
		}

		// Get cursor number
		cursor := srcItem.Cursor
		if cursor < 0 {
			cursor = c.parse.AllocCursor()
			srcItem.Cursor = cursor
		}

		// Open table cursor
		// OP_OpenRead cursor_num, root_page
		rootPage := srcItem.Table.RootPage
		vdbe.AddOp2(OP_OpenRead, cursor, rootPage)

		// Rewind cursor to start
		// OP_Rewind cursor_num, done_label
		addrRewind := vdbe.AddOp2(OP_Rewind, cursor, 0)
		srcItem.AddrFillIndex = addrRewind // Store for patching later
	}

	return nil
}

// compileWhereClause generates code for the WHERE clause.
func (c *SelectCompiler) compileWhereClause(where *Expr, jumpIfFalse int) error {
	// Generate code to evaluate WHERE expression
	reg := c.parse.AllocReg()
	c.compileExpr(where, reg)

	// Jump if false
	vdbe := c.parse.GetVdbe()
	vdbe.AddOp3(OP_IfNot, reg, jumpIfFalse, 1)

	c.parse.ReleaseReg(reg)
	return nil
}

// selectInnerLoop generates the code for the inner loop of a SELECT.
// This extracts the result columns and processes each row.
func (c *SelectCompiler) selectInnerLoop(
	sel *Select,
	srcTab int,
	sort *SortCtx,
	distinct *DistinctCtx,
	dest *SelectDest,
	iContinue int,
	iBreak int,
) error {
	vdbe := c.parse.GetVdbe()
	nResultCol := sel.EList.Len()

	// Allocate registers for result if needed
	regResult := dest.Sdst
	if regResult == 0 {
		regResult = c.parse.AllocRegs(nResultCol)
		dest.Sdst = regResult
		dest.NSdst = nResultCol
	}

	// Extract result columns
	if srcTab >= 0 {
		// Pull data from intermediate table
		for i := 0; i < nResultCol; i++ {
			// OP_Column srcTab, column_idx, reg
			vdbe.AddOp3(OP_Column, srcTab, i, regResult+i)
		}
	} else {
		// Evaluate result expressions
		for i := 0; i < nResultCol; i++ {
			expr := sel.EList.Get(i).Expr
			c.compileExpr(expr, regResult+i)
		}
	}

	// Handle DISTINCT if needed
	if distinct != nil && distinct.IsTnct != 0 {
		c.codeDistinct(distinct, nResultCol, regResult, iContinue)
	}

	// Apply OFFSET if no ORDER BY
	if sort == nil && sel.Offset > 0 {
		c.codeOffset(sel.Offset, iContinue)
	}

	// Dispose of the result according to destination
	switch dest.Dest {
	case SRT_Output:
		// Return result row
		// OP_ResultRow reg, num_cols
		vdbe.AddOp2(OP_ResultRow, regResult, nResultCol)

	case SRT_Mem:
		// Store in memory cells
		if regResult != dest.SDParm {
			// OP_Copy src, dst, count
			vdbe.AddOp3(OP_Copy, regResult, dest.SDParm, nResultCol-1)
		}

	case SRT_Set:
		// Store as key in index
		r1 := c.parse.AllocReg()
		// OP_MakeRecord reg, count, dst
		vdbe.AddOp3(OP_MakeRecord, regResult, nResultCol, r1)
		// OP_IdxInsert cursor, record_reg, key_reg, key_count
		vdbe.AddOp4Int(OP_IdxInsert, dest.SDParm, r1, regResult, nResultCol)
		c.parse.ReleaseReg(r1)

	case SRT_Table, SRT_EphemTab:
		// Store in table
		r1 := c.parse.AllocReg()
		r2 := c.parse.AllocReg()
		// Make record
		vdbe.AddOp3(OP_MakeRecord, regResult, nResultCol, r1)
		// Get new rowid
		vdbe.AddOp2(OP_NewRowid, dest.SDParm, r2)
		// Insert row
		vdbe.AddOp3(OP_Insert, dest.SDParm, r1, r2)
		c.parse.ReleaseReg(r1)
		c.parse.ReleaseReg(r2)

	case SRT_Exists:
		// Just set flag to 1
		vdbe.AddOp2(OP_Integer, 1, dest.SDParm)

	case SRT_Coroutine:
		// Yield to coroutine
		vdbe.AddOp1(OP_Yield, dest.SDParm)

	default:
		return fmt.Errorf("unsupported destination type: %d", dest.Dest)
	}

	// Apply LIMIT if no ORDER BY
	if sort == nil && sel.Limit > 0 {
		c.applyLimitCheck(sel.Limit, iBreak)
	}

	return nil
}

// codeDistinct generates code to enforce DISTINCT.
func (c *SelectCompiler) codeDistinct(distinct *DistinctCtx, nCol int, regResult int, jumpIfDup int) {
	vdbe := c.parse.GetVdbe()

	if distinct.ETnctType == WHERE_DISTINCT_ORDERED {
		// Use comparison with previous row
		regPrev := c.parse.AllocRegs(nCol)
		addrJump := vdbe.AddOp4Int(OP_Compare, regPrev, regResult, nCol, 0)
		vdbe.AddOp3(OP_Jump, addrJump+2, jumpIfDup, addrJump+2)
		vdbe.AddOp3(OP_Copy, regResult, regPrev, nCol-1)
	} else {
		// Use ephemeral table
		r1 := c.parse.AllocReg()
		vdbe.AddOp3(OP_MakeRecord, regResult, nCol, r1)
		vdbe.AddOp4Int(OP_Found, distinct.TabTnct, jumpIfDup, r1, 0)
		vdbe.AddOp4Int(OP_IdxInsert, distinct.TabTnct, r1, regResult, nCol)
		c.parse.ReleaseReg(r1)
	}
}

// setupDistinct initializes DISTINCT processing.
func (c *SelectCompiler) setupDistinct(sel *Select, distinct *DistinctCtx) {
	distinct.IsTnct = 1
	distinct.TabTnct = c.parse.AllocCursor()

	// Determine if we can optimize DISTINCT
	if sel.OrderBy != nil && c.canUseOrderedDistinct(sel) {
		distinct.ETnctType = WHERE_DISTINCT_ORDERED
	} else {
		distinct.ETnctType = WHERE_DISTINCT_UNORDERED

		// Open ephemeral table for DISTINCT
		vdbe := c.parse.GetVdbe()
		nCol := sel.EList.Len()
		distinct.AddrTnct = vdbe.AddOp2(OP_OpenEphemeral, distinct.TabTnct, nCol)
	}
}

// canUseOrderedDistinct checks if DISTINCT can use ORDER BY optimization.
func (c *SelectCompiler) canUseOrderedDistinct(sel *Select) bool {
	// Simple heuristic: if ORDER BY covers all result columns, can use optimization
	if sel.OrderBy == nil || sel.EList.Len() != sel.OrderBy.Len() {
		return false
	}
	return true
}

// compileExpr generates code to evaluate an expression.
// This is a simplified version - full implementation would be in expr.go
func (c *SelectCompiler) compileExpr(expr *Expr, target int) {
	vdbe := c.parse.GetVdbe()

	switch expr.Op {
	case TK_COLUMN:
		// OP_Column cursor, column_idx, reg
		vdbe.AddOp3(OP_Column, expr.Table, expr.Column, target)

	case TK_INTEGER:
		// OP_Integer value, reg
		vdbe.AddOp2(OP_Integer, expr.IntValue, target)

	case TK_STRING:
		// OP_String8 0, reg, value
		vdbe.AddOp4(OP_String8, 0, target, 0, expr.StringValue)

	case TK_NULL:
		// OP_Null 0, reg
		vdbe.AddOp2(OP_Null, 0, target)

	case TK_ASTERISK:
		// SELECT * - handled elsewhere

	default:
		// For complex expressions, would call expression compiler
		vdbe.AddOp2(OP_Null, 0, target)
	}
}

// compileCompoundSelect handles UNION, INTERSECT, EXCEPT.
func (c *SelectCompiler) compileCompoundSelect(sel *Select, dest *SelectDest) error {
	// Determine compound type
	op := sel.Op

	switch op {
	case TK_UNION, TK_UNION_ALL:
		return c.compileUnion(sel, dest)
	case TK_INTERSECT:
		return c.compileIntersect(sel, dest)
	case TK_EXCEPT:
		return c.compileExcept(sel, dest)
	default:
		return fmt.Errorf("unsupported compound operator: %d", op)
	}
}

// compileUnion compiles UNION/UNION ALL.
func (c *SelectCompiler) compileUnion(sel *Select, dest *SelectDest) error {
	vdbe := c.parse.GetVdbe()

	// Create temporary table for union
	unionTab := c.parse.AllocCursor()
	nCol := sel.EList.Len()
	vdbe.AddOp2(OP_OpenEphemeral, unionTab, nCol)

	// Compile left side into temp table
	leftDest := &SelectDest{
		Dest:   SRT_Union,
		SDParm: unionTab,
	}
	if err := c.CompileSelect(sel.Prior, leftDest); err != nil {
		return err
	}

	// Compile right side into temp table
	if err := c.compileSimpleSelect(sel, leftDest); err != nil {
		return err
	}

	// Read from temp table to destination
	addrEnd := vdbe.MakeLabel()
	vdbe.AddOp2(OP_Rewind, unionTab, addrEnd)

	addrLoop := vdbe.CurrentAddr()
	regResult := c.parse.AllocRegs(nCol)
	for i := 0; i < nCol; i++ {
		vdbe.AddOp3(OP_Column, unionTab, i, regResult+i)
	}
	vdbe.AddOp2(OP_ResultRow, regResult, nCol)
	vdbe.AddOp2(OP_Next, unionTab, addrLoop)

	vdbe.ResolveLabel(addrEnd)
	vdbe.AddOp1(OP_Close, unionTab)

	return nil
}

// compileIntersect compiles INTERSECT.
func (c *SelectCompiler) compileIntersect(sel *Select, dest *SelectDest) error {
	// Similar to UNION but with different logic
	return fmt.Errorf("INTERSECT not yet implemented")
}

// compileExcept compiles EXCEPT.
func (c *SelectCompiler) compileExcept(sel *Select, dest *SelectDest) error {
	// Similar to UNION but with different logic
	return fmt.Errorf("EXCEPT not yet implemented")
}

// codeOffset generates code to skip OFFSET rows.
func (c *SelectCompiler) codeOffset(offset int, jumpTo int) {
	vdbe := c.parse.GetVdbe()

	// Check if we've skipped enough rows yet
	regOffset := c.parse.AllocReg()
	vdbe.AddOp2(OP_Integer, offset, regOffset)
	vdbe.AddOp3(OP_IfPos, regOffset, jumpTo, -1)
	c.parse.ReleaseReg(regOffset)
}

// applyLimitCheck generates code to check LIMIT.
func (c *SelectCompiler) applyLimitCheck(limit int, jumpTo int) {
	vdbe := c.parse.GetVdbe()

	regLimit := c.parse.AllocReg()
	vdbe.AddOp2(OP_Integer, limit, regLimit)
	vdbe.AddOp3(OP_IfNot, regLimit, jumpTo, -1)
	c.parse.ReleaseReg(regLimit)
}
