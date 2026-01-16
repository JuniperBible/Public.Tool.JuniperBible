package sql

import (
	"fmt"
)

// OrderByCompiler handles ORDER BY clause compilation.
type OrderByCompiler struct {
	parse *Parse
}

// NewOrderByCompiler creates a new ORDER BY compiler.
func NewOrderByCompiler(parse *Parse) *OrderByCompiler {
	return &OrderByCompiler{parse: parse}
}

// Sort order constants
const (
	SQLITE_SO_ASC  = 0 // Ascending order
	SQLITE_SO_DESC = 1 // Descending order
)

// setupOrderBy initializes ORDER BY processing for a SELECT.
func (sc *SelectCompiler) setupOrderBy(sel *Select, sort *SortCtx) error {
	if sel.OrderBy == nil || sel.OrderBy.Len() == 0 {
		return nil
	}

	vdbe := sc.parse.GetVdbe()
	sort.OrderBy = sel.OrderBy
	sort.NOBSat = 0 // Number of ORDER BY terms satisfied by index

	// Allocate cursor for sorter
	sort.ECursor = sc.parse.AllocCursor()

	// Determine if we should use sorter or ephemeral table
	sort.SortFlags = SORTFLAG_UseSorter

	// Calculate number of columns in sorter record
	nOrderBy := sel.OrderBy.Len()
	nResultCol := sel.EList.Len()

	// Open sorter/ephemeral table
	// Record format: [ORDER BY cols...] [result cols...]
	nCol := nOrderBy + nResultCol

	if sort.SortFlags&SORTFLAG_UseSorter != 0 {
		// Use sorter
		sort.AddrSortIndex = vdbe.AddOp2(OP_SorterOpen, sort.ECursor, nCol)
	} else {
		// Use ephemeral B-tree table
		sort.AddrSortIndex = vdbe.AddOp2(OP_OpenEphemeral, sort.ECursor, nCol)
	}

	// Set up done label (will be resolved when sorting completes)
	sort.LabelDone = vdbe.MakeLabel()

	return nil
}

// generateSortTail generates code to extract sorted results.
// This is called after all data has been inserted into the sorter.
func (sc *SelectCompiler) generateSortTail(sel *Select, sort *SortCtx, nColumn int, dest *SelectDest) error {
	obc := NewOrderByCompiler(sc.parse)
	return obc.generateSortTail(sel, sort, nColumn, dest)
}

// generateSortTail implements the main sorting output loop.
func (obc *OrderByCompiler) generateSortTail(sel *Select, sort *SortCtx, nColumn int, dest *SelectDest) error {
	vdbe := obc.parse.GetVdbe()

	addrBreak := sort.LabelDone
	addrContinue := vdbe.MakeLabel()

	orderBy := sort.OrderBy
	nKey := orderBy.Len() - sort.NOBSat

	// Start loop to extract sorted records
	iTab := sort.ECursor

	// Allocate registers for result row
	var regRow, regRowid int
	eDest := dest.Dest

	if eDest == SRT_Output || eDest == SRT_Coroutine || eDest == SRT_Mem {
		regRow = dest.Sdst
		regRowid = 0
	} else {
		regRowid = obc.parse.AllocReg()
		if eDest == SRT_EphemTab || eDest == SRT_Table {
			regRow = obc.parse.AllocReg()
			nColumn = 0
		} else {
			regRow = obc.parse.AllocRegs(nColumn)
		}
	}

	// Generate sort loop based on sorter type
	var addr, iSortTab int
	var bSeq bool

	if sort.SortFlags&SORTFLAG_UseSorter != 0 {
		// Using sorter
		regSortOut := obc.parse.AllocReg()
		iSortTab = obc.parse.AllocCursor()

		// Open pseudo-cursor to read sorter output
		vdbe.AddOp3(OP_OpenPseudo, iSortTab, regSortOut, nKey+1+nColumn)

		// Sort the data
		addr = vdbe.AddOp2(OP_SorterSort, iTab, addrBreak)

		// Extract data from sorter
		vdbe.AddOp3(OP_SorterData, iTab, regSortOut, iSortTab)
		bSeq = false
	} else {
		// Using ephemeral table
		addr = vdbe.AddOp2(OP_Sort, iTab, addrBreak)

		// Handle OFFSET if present
		if sel.Offset > 0 {
			obc.codeOffset(sel.Offset, addrContinue)
		}

		iSortTab = iTab
		bSeq = true

		// Adjust LIMIT for OFFSET
		if sel.Offset > 0 && sel.Limit > 0 {
			vdbe.AddOp2(OP_AddImm, sel.Limit, -1)
		}
	}

	// Extract result columns from sorted record
	// Record format: [ORDER BY keys...] [seq?] [result columns...]
	iCol := nKey
	if bSeq {
		iCol++ // Skip sequence number
	}

	aOutEx := sel.EList

	// Extract each result column
	for i := nColumn - 1; i >= 0; i-- {
		item := aOutEx.Get(i)
		var iRead int

		// Check if this column is also an ORDER BY expression
		if item.OrderByCol > 0 {
			// Column value is in ORDER BY section
			iRead = item.OrderByCol - 1
		} else {
			// Column value is in result section
			iRead = iCol
			iCol++
		}

		// Extract column
		vdbe.AddOp3(OP_Column, iSortTab, iRead, regRow+i)
	}

	// Output the row based on destination
	switch eDest {
	case SRT_Table, SRT_EphemTab:
		// Insert into table
		vdbe.AddOp3(OP_Column, iSortTab, nKey+boolToInt(bSeq), regRow)
		vdbe.AddOp2(OP_NewRowid, dest.SDParm, regRowid)
		vdbe.AddOp3(OP_Insert, dest.SDParm, regRow, regRowid)

	case SRT_Set:
		// Insert into index
		r1 := obc.parse.AllocReg()
		vdbe.AddOp4(OP_MakeRecord, regRow, nColumn, r1, dest.AffSdst)
		vdbe.AddOp4Int(OP_IdxInsert, dest.SDParm, r1, regRow, nColumn)
		obc.parse.ReleaseReg(r1)

	case SRT_Mem:
		// Store in memory - LIMIT will break the loop

	case SRT_Output:
		// Return result row
		vdbe.AddOp2(OP_ResultRow, dest.Sdst, nColumn)

	case SRT_Coroutine:
		// Yield to coroutine
		vdbe.AddOp1(OP_Yield, dest.SDParm)

	default:
		return fmt.Errorf("unsupported destination type in ORDER BY: %d", eDest)
	}

	// Clean up registers
	if regRowid != 0 {
		if eDest == SRT_Set {
			obc.parse.ReleaseRegs(regRow, nColumn)
		} else {
			obc.parse.ReleaseReg(regRow)
		}
		obc.parse.ReleaseReg(regRowid)
	}

	// Loop to next sorted record
	vdbe.ResolveLabel(addrContinue)

	if sort.SortFlags&SORTFLAG_UseSorter != 0 {
		vdbe.AddOp2(OP_SorterNext, iTab, addr)
	} else {
		vdbe.AddOp2(OP_Next, iTab, addr)
	}

	vdbe.ResolveLabel(addrBreak)

	return nil
}

// pushOntoSorter generates code to insert a row into the sorter.
func (obc *OrderByCompiler) pushOntoSorter(
	sort *SortCtx,
	sel *Select,
	regData int,
	regOrigData int,
	nData int,
	nPrefixReg int,
) error {
	vdbe := obc.parse.GetVdbe()

	nExpr := sort.OrderBy.Len()

	// Allocate registers for ORDER BY keys
	regBase := obc.parse.AllocRegs(nExpr + 1)
	regRecord := regBase + nExpr

	// Evaluate ORDER BY expressions
	for i := 0; i < nExpr; i++ {
		item := sort.OrderBy.Get(i)
		obc.compileExpr(item.Expr, regBase+i)
	}

	// Create sorter key record
	if sort.SortFlags&SORTFLAG_UseSorter != 0 {
		// Sorter: key includes ORDER BY values plus data
		// Make key record
		vdbe.AddOp3(OP_MakeRecord, regBase, nExpr, regRecord)

		// Insert into sorter
		// OP_SorterInsert cursor, record_reg
		vdbe.AddOp2(OP_SorterInsert, sort.ECursor, regRecord)

		// Also need to store the actual data
		// This is typically done by concatenating data to the key
		if nData > 0 {
			regDataRecord := obc.parse.AllocReg()
			vdbe.AddOp3(OP_MakeRecord, regData, nData, regDataRecord)
			// In real SQLite, this would be concatenated with the key
			obc.parse.ReleaseReg(regDataRecord)
		}
	} else {
		// Ephemeral table: key is ORDER BY values, data is result
		// Make complete record: [ORDER BY keys...] [sequence] [data...]
		nRec := nExpr + 1 + nData

		// Add sequence number (for stable sort)
		regSeq := obc.parse.AllocReg()
		vdbe.AddOp2(OP_Sequence, sort.ECursor, regSeq)

		// Copy data columns
		if nData > 0 && regData != regBase+nExpr+1 {
			vdbe.AddOp3(OP_Copy, regData, regBase+nExpr+1, nData-1)
		}

		// Make complete record
		vdbe.AddOp3(OP_MakeRecord, regBase, nRec, regRecord)

		// Get rowid and insert
		regRowid := obc.parse.AllocReg()
		vdbe.AddOp2(OP_NewRowid, sort.ECursor, regRowid)
		vdbe.AddOp3(OP_Insert, sort.ECursor, regRecord, regRowid)
		obc.parse.ReleaseReg(regRowid)
		obc.parse.ReleaseReg(regSeq)
	}

	obc.parse.ReleaseRegs(regBase, nExpr+1)

	return nil
}

// CompileOrderBy compiles ORDER BY expressions and generates sorting code.
func (obc *OrderByCompiler) CompileOrderBy(sel *Select, orderBy *ExprList) error {
	if orderBy == nil || orderBy.Len() == 0 {
		return nil
	}

	// Validate and resolve ORDER BY expressions
	for i := 0; i < orderBy.Len(); i++ {
		item := orderBy.Get(i)

		// ORDER BY can reference:
		// 1. Column number (e.g., ORDER BY 1)
		// 2. Column alias
		// 3. Expression

		if item.Expr.Op == TK_INTEGER {
			// Column number - validate range
			colNum := item.Expr.IntValue
			if colNum < 1 || colNum > sel.EList.Len() {
				return fmt.Errorf("ORDER BY column number %d out of range (1..%d)",
					colNum, sel.EList.Len())
			}

			// Replace with reference to result column
			item.Expr = sel.EList.Get(colNum - 1).Expr
			item.OrderByCol = colNum

		} else if item.Expr.Op == TK_ID {
			// Column alias - try to find in result columns
			name := item.Expr.StringValue
			found := false

			for j := 0; j < sel.EList.Len(); j++ {
				resultItem := sel.EList.Get(j)
				if resultItem.Name == name {
					item.Expr = resultItem.Expr
					item.OrderByCol = j + 1
					found = true
					break
				}
			}

			if !found {
				// Not an alias - treat as expression
				// Resolve column references
				if err := obc.resolveOrderByExpr(sel, item.Expr); err != nil {
					return err
				}
			}

		} else {
			// Expression - resolve column references
			if err := obc.resolveOrderByExpr(sel, item.Expr); err != nil {
				return err
			}
		}
	}

	return nil
}

// resolveOrderByExpr resolves column references in ORDER BY expression.
func (obc *OrderByCompiler) resolveOrderByExpr(sel *Select, expr *Expr) error {
	if expr == nil {
		return nil
	}

	switch expr.Op {
	case TK_COLUMN:
		// Resolve column reference
		return obc.resolveColumnInOrderBy(sel, expr)

	case TK_DOT:
		// Qualified column reference
		return obc.resolveQualifiedColumnInOrderBy(sel, expr)

	default:
		// Recurse into child expressions
		if expr.Left != nil {
			if err := obc.resolveOrderByExpr(sel, expr.Left); err != nil {
				return err
			}
		}
		if expr.Right != nil {
			if err := obc.resolveOrderByExpr(sel, expr.Right); err != nil {
				return err
			}
		}
	}

	return nil
}

// resolveColumnInOrderBy resolves unqualified column in ORDER BY.
func (obc *OrderByCompiler) resolveColumnInOrderBy(sel *Select, expr *Expr) error {
	colName := expr.StringValue

	// Search in FROM clause tables
	if sel.Src != nil {
		for i := 0; i < sel.Src.Len(); i++ {
			srcItem := sel.Src.Get(i)
			if srcItem.Table == nil {
				continue
			}

			table := srcItem.Table
			for colIdx := 0; colIdx < table.NumColumns; colIdx++ {
				col := table.GetColumn(colIdx)
				if col.Name == colName {
					expr.Table = srcItem.Cursor
					expr.Column = colIdx
					expr.ColumnRef = col
					return nil
				}
			}
		}
	}

	return fmt.Errorf("no such column: %s", colName)
}

// resolveQualifiedColumnInOrderBy resolves qualified column (table.col) in ORDER BY.
func (obc *OrderByCompiler) resolveQualifiedColumnInOrderBy(sel *Select, expr *Expr) error {
	if expr.Left == nil || expr.Left.Op != TK_ID {
		return fmt.Errorf("invalid table reference in ORDER BY")
	}
	if expr.Right == nil || expr.Right.Op != TK_ID {
		return fmt.Errorf("invalid column reference in ORDER BY")
	}

	tableName := expr.Left.StringValue
	colName := expr.Right.StringValue

	// Find table in FROM clause
	if sel.Src != nil {
		for i := 0; i < sel.Src.Len(); i++ {
			srcItem := sel.Src.Get(i)
			if srcItem.Table == nil {
				continue
			}

			matchName := srcItem.Table.Name == tableName
			matchAlias := srcItem.Alias == tableName

			if matchName || matchAlias {
				table := srcItem.Table
				for colIdx := 0; colIdx < table.NumColumns; colIdx++ {
					col := table.GetColumn(colIdx)
					if col.Name == colName {
						expr.Op = TK_COLUMN
						expr.Table = srcItem.Cursor
						expr.Column = colIdx
						expr.ColumnRef = col
						expr.Left = nil
						expr.Right = nil
						return nil
					}
				}
				return fmt.Errorf("no such column: %s.%s", tableName, colName)
			}
		}
	}

	return fmt.Errorf("no such table: %s", tableName)
}

// codeOffset generates code to skip OFFSET rows during sorting.
func (obc *OrderByCompiler) codeOffset(offset int, jumpTo int) {
	vdbe := obc.parse.GetVdbe()

	regOffset := obc.parse.AllocReg()
	vdbe.AddOp2(OP_Integer, offset, regOffset)
	vdbe.AddOp3(OP_IfPos, regOffset, jumpTo, -1)
	obc.parse.ReleaseReg(regOffset)
}

// compileExpr is a helper to compile expressions.
func (obc *OrderByCompiler) compileExpr(expr *Expr, target int) {
	vdbe := obc.parse.GetVdbe()

	switch expr.Op {
	case TK_COLUMN:
		vdbe.AddOp3(OP_Column, expr.Table, expr.Column, target)
	case TK_INTEGER:
		vdbe.AddOp2(OP_Integer, expr.IntValue, target)
	case TK_STRING:
		vdbe.AddOp4(OP_String8, 0, target, 0, expr.StringValue)
	case TK_NULL:
		vdbe.AddOp2(OP_Null, 0, target)
	default:
		vdbe.AddOp2(OP_Null, 0, target)
	}
}

// boolToInt converts bool to int (for bSeq in sort tail).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
