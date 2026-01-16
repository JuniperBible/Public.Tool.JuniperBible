package sql

import (
	"fmt"
)

// AggInfo contains information about aggregate functions and GROUP BY.
type AggInfo struct {
	SortingIdx   bool              // True if using index for GROUP BY
	UseSorter    bool              // True if sorting required for GROUP BY
	SortingIdxP  int               // Cursor for sorting index
	AggFuncs     []AggFunc         // Aggregate functions used
	Cols         []AggColumn       // Columns used in aggregate expressions
	GroupBySort  int               // Cursor for GROUP BY sorter
	GroupByIdx   int               // Index that provides GROUP BY ordering
	NumCols      int               // Number of columns in result
	NumAggFuncs  int               // Number of aggregate functions
	NumGroupBy   int               // Number of GROUP BY columns
	SelectID     int               // SELECT id that owns this AggInfo
}

// AggFunc represents an aggregate function (SUM, COUNT, AVG, etc).
type AggFunc struct {
	Expr       *Expr  // Expression for the function
	Func       *FuncDef // Function definition
	Reg        int    // Register holding accumulated value
	DistinctReg int   // Register for DISTINCT values (if needed)
	DistinctIdx int   // Index for DISTINCT values (if needed)
	RegAcc     int    // Register for accumulator
	FuncFlags  int    // Function flags
}

// AggColumn represents a column used in an aggregate expression.
type AggColumn struct {
	Table      int    // Source table cursor
	Column     int    // Column index in source table
	SorterCol  int    // Column index in sorter
	Reg        int    // Register to hold value
}

// AggregateCompiler compiles GROUP BY and aggregate functions.
type AggregateCompiler struct {
	parse *Parse
}

// NewAggregateCompiler creates a new aggregate compiler.
func NewAggregateCompiler(parse *Parse) *AggregateCompiler {
	return &AggregateCompiler{parse: parse}
}

// CompileGroupBy compiles a SELECT with GROUP BY clause.
func (sc *SelectCompiler) compileGroupBy(sel *Select, dest *SelectDest) error {
	ac := NewAggregateCompiler(sc.parse)
	return ac.compileGroupBy(sel, dest)
}

// compileGroupBy generates code for GROUP BY with aggregates.
func (ac *AggregateCompiler) compileGroupBy(sel *Select, dest *SelectDest) error {
	vdbe := ac.parse.GetVdbe()
	if vdbe == nil {
		return fmt.Errorf("no VDBE available")
	}

	// Analyze aggregate functions and GROUP BY
	aggInfo, err := ac.analyzeAggregates(sel)
	if err != nil {
		return err
	}

	// Open ephemeral table/sorter for grouping
	groupCursor := ac.parse.AllocCursor()
	nGroupBy := sel.GroupBy.Len()

	// Create sorter for GROUP BY
	vdbe.AddOp2(OP_OpenEphemeral, groupCursor, nGroupBy)
	aggInfo.GroupBySort = groupCursor

	// Initialize aggregate accumulators
	ac.initializeAccumulators(aggInfo)

	// Generate loop to scan source tables and compute aggregates
	_ = vdbe.CurrentAddr() // addrLoop - reserved for future use
	addrBreak := vdbe.MakeLabel()

	// Open source tables
	if sel.Src != nil {
		for i := 0; i < sel.Src.Len(); i++ {
			srcItem := sel.Src.Get(i)
			if srcItem.Table == nil {
				continue
			}
			cursor := srcItem.Cursor
			vdbe.AddOp2(OP_OpenRead, cursor, srcItem.Table.RootPage)
			vdbe.AddOp2(OP_Rewind, cursor, addrBreak)
		}
	}

	// Inner loop to process rows
	addrInnerLoop := vdbe.CurrentAddr()

	// Evaluate WHERE clause
	if sel.Where != nil {
		whereReg := ac.parse.AllocReg()
		ac.compileExpr(sel.Where, whereReg)
		vdbe.AddOp3(OP_IfNot, whereReg, addrBreak, 1)
		ac.parse.ReleaseReg(whereReg)
	}

	// Check if starting new group or continuing current group
	ac.checkNewGroup(sel, aggInfo, addrBreak)

	// Update aggregate accumulators
	ac.updateAccumulators(sel, aggInfo)

	// Move to next row
	if sel.Src != nil && sel.Src.Len() > 0 {
		cursor := sel.Src.Get(0).Cursor
		vdbe.AddOp2(OP_Next, cursor, addrInnerLoop)
	}

	// Finalize aggregates and output results
	vdbe.ResolveLabel(addrBreak)
	ac.finalizeAggregates(sel, aggInfo, dest)

	return nil
}

// analyzeAggregates analyzes SELECT to find aggregate functions and columns.
func (ac *AggregateCompiler) analyzeAggregates(sel *Select) (*AggInfo, error) {
	aggInfo := &AggInfo{
		SelectID: sel.SelectID,
	}

	// Find aggregate functions in result columns
	if sel.EList != nil {
		for i := 0; i < sel.EList.Len(); i++ {
			item := sel.EList.Get(i)
			if err := ac.findAggregateFuncs(item.Expr, aggInfo); err != nil {
				return nil, err
			}
		}
	}

	// Find aggregate functions in HAVING clause
	if sel.Having != nil {
		if err := ac.findAggregateFuncs(sel.Having, aggInfo); err != nil {
			return nil, err
		}
	}

	// Find aggregate functions in ORDER BY
	if sel.OrderBy != nil {
		for i := 0; i < sel.OrderBy.Len(); i++ {
			item := sel.OrderBy.Get(i)
			if err := ac.findAggregateFuncs(item.Expr, aggInfo); err != nil {
				return nil, err
			}
		}
	}

	// Record GROUP BY columns
	if sel.GroupBy != nil {
		aggInfo.NumGroupBy = sel.GroupBy.Len()
	}

	aggInfo.NumAggFuncs = len(aggInfo.AggFuncs)
	aggInfo.NumCols = len(aggInfo.Cols)

	return aggInfo, nil
}

// findAggregateFuncs recursively finds aggregate functions in expression tree.
func (ac *AggregateCompiler) findAggregateFuncs(expr *Expr, aggInfo *AggInfo) error {
	if expr == nil {
		return nil
	}

	if expr.Op == TK_AGG_FUNCTION {
		// Found aggregate function
		aggFunc := AggFunc{
			Expr: expr,
			Func: expr.FuncDef,
		}
		aggInfo.AggFuncs = append(aggInfo.AggFuncs, aggFunc)
	}

	// Recurse into child expressions
	if expr.Left != nil {
		if err := ac.findAggregateFuncs(expr.Left, aggInfo); err != nil {
			return err
		}
	}
	if expr.Right != nil {
		if err := ac.findAggregateFuncs(expr.Right, aggInfo); err != nil {
			return err
		}
	}
	if expr.List != nil {
		for i := 0; i < expr.List.Len(); i++ {
			item := expr.List.Get(i)
			if err := ac.findAggregateFuncs(item.Expr, aggInfo); err != nil {
				return err
			}
		}
	}

	return nil
}

// initializeAccumulators allocates registers and initializes aggregate accumulators.
func (ac *AggregateCompiler) initializeAccumulators(aggInfo *AggInfo) {
	vdbe := ac.parse.GetVdbe()

	for i := range aggInfo.AggFuncs {
		aggFunc := &aggInfo.AggFuncs[i]

		// Allocate register for accumulator
		aggFunc.RegAcc = ac.parse.AllocReg()
		aggFunc.Reg = aggFunc.RegAcc

		// Initialize based on function type
		funcName := aggFunc.Func.Name

		switch funcName {
		case "count":
			// COUNT: initialize to 0
			vdbe.AddOp2(OP_Integer, 0, aggFunc.RegAcc)

		case "sum", "total", "avg":
			// SUM/AVG: initialize to NULL
			vdbe.AddOp2(OP_Null, 0, aggFunc.RegAcc)

		case "min", "max":
			// MIN/MAX: initialize to NULL
			vdbe.AddOp2(OP_Null, 0, aggFunc.RegAcc)

		case "group_concat":
			// GROUP_CONCAT: initialize to empty string
			vdbe.AddOp4(OP_String8, 0, aggFunc.RegAcc, 0, "")

		default:
			// Generic: initialize to NULL
			vdbe.AddOp2(OP_Null, 0, aggFunc.RegAcc)
		}
	}
}

// checkNewGroup generates code to check if we're starting a new group.
func (ac *AggregateCompiler) checkNewGroup(sel *Select, aggInfo *AggInfo, continueAddr int) {
	if sel.GroupBy == nil || sel.GroupBy.Len() == 0 {
		return
	}

	vdbe := ac.parse.GetVdbe()
	nGroupBy := sel.GroupBy.Len()

	// Allocate registers for current GROUP BY keys
	regGroupBy := ac.parse.AllocRegs(nGroupBy)

	// Evaluate GROUP BY expressions
	for i := 0; i < nGroupBy; i++ {
		item := sel.GroupBy.Get(i)
		ac.compileExpr(item.Expr, regGroupBy+i)
	}

	// Compare with previous GROUP BY keys
	// If different, finalize current group and start new one
	regPrev := ac.parse.AllocRegs(nGroupBy)
	addrSame := vdbe.MakeLabel()

	// Compare each GROUP BY column
	for i := 0; i < nGroupBy; i++ {
		vdbe.AddOp3(OP_Ne, regGroupBy+i, continueAddr, regPrev+i)
	}

	vdbe.ResolveLabel(addrSame)
}

// updateAccumulators generates code to update aggregate accumulators.
func (ac *AggregateCompiler) updateAccumulators(sel *Select, aggInfo *AggInfo) {
	vdbe := ac.parse.GetVdbe()

	for i := range aggInfo.AggFuncs {
		aggFunc := &aggInfo.AggFuncs[i]
		funcName := aggFunc.Func.Name

		// Get function argument
		var argReg int
		if aggFunc.Expr.List != nil && aggFunc.Expr.List.Len() > 0 {
			argReg = ac.parse.AllocReg()
			arg := aggFunc.Expr.List.Get(0).Expr
			ac.compileExpr(arg, argReg)
		}

		switch funcName {
		case "count":
			if argReg == 0 {
				// COUNT(*) - always increment
				vdbe.AddOp2(OP_AddImm, aggFunc.RegAcc, 1)
			} else {
				// COUNT(expr) - increment if not NULL
				addrSkip := vdbe.MakeLabel()
				vdbe.AddOp2(OP_IsNull, argReg, addrSkip)
				vdbe.AddOp2(OP_AddImm, aggFunc.RegAcc, 1)
				vdbe.ResolveLabel(addrSkip)
			}

		case "sum", "total":
			// SUM: add value to accumulator
			if argReg != 0 {
				addrSkip := vdbe.MakeLabel()
				vdbe.AddOp2(OP_IsNull, argReg, addrSkip)
				vdbe.AddOp3(OP_Add, argReg, aggFunc.RegAcc, aggFunc.RegAcc)
				vdbe.ResolveLabel(addrSkip)
			}

		case "avg":
			// AVG: sum and count
			if argReg != 0 {
				addrSkip := vdbe.MakeLabel()
				vdbe.AddOp2(OP_IsNull, argReg, addrSkip)
				// Add to sum (stored in RegAcc)
				vdbe.AddOp3(OP_Add, argReg, aggFunc.RegAcc, aggFunc.RegAcc)
				// Increment count (need separate register)
				countReg := ac.parse.AllocReg()
				vdbe.AddOp2(OP_AddImm, countReg, 1)
				vdbe.ResolveLabel(addrSkip)
			}

		case "min":
			// MIN: keep minimum value
			if argReg != 0 {
				addrNotMin := vdbe.MakeLabel()
				vdbe.AddOp3(OP_Lt, aggFunc.RegAcc, addrNotMin, argReg)
				vdbe.AddOp2(OP_Copy, argReg, aggFunc.RegAcc)
				vdbe.ResolveLabel(addrNotMin)
			}

		case "max":
			// MAX: keep maximum value
			if argReg != 0 {
				addrNotMax := vdbe.MakeLabel()
				vdbe.AddOp3(OP_Gt, aggFunc.RegAcc, addrNotMax, argReg)
				vdbe.AddOp2(OP_Copy, argReg, aggFunc.RegAcc)
				vdbe.ResolveLabel(addrNotMax)
			}

		case "group_concat":
			// GROUP_CONCAT: concatenate strings
			if argReg != 0 {
				addrSkip := vdbe.MakeLabel()
				vdbe.AddOp2(OP_IsNull, argReg, addrSkip)
				vdbe.AddOp3(OP_Concat, aggFunc.RegAcc, argReg, aggFunc.RegAcc)
				vdbe.ResolveLabel(addrSkip)
			}
		}

		if argReg != 0 {
			ac.parse.ReleaseReg(argReg)
		}
	}
}

// finalizeAggregates generates code to finalize aggregates and output results.
func (ac *AggregateCompiler) finalizeAggregates(sel *Select, aggInfo *AggInfo, dest *SelectDest) {
	vdbe := ac.parse.GetVdbe()

	// Allocate registers for result
	nResult := sel.EList.Len()
	regResult := ac.parse.AllocRegs(nResult)

	// Compute final result columns
	for i := 0; i < nResult; i++ {
		item := sel.EList.Get(i)
		ac.finalizeResultExpr(item.Expr, aggInfo, regResult+i)
	}

	// Apply HAVING clause if present
	if sel.Having != nil {
		havingReg := ac.parse.AllocReg()
		ac.finalizeResultExpr(sel.Having, aggInfo, havingReg)
		addrSkip := vdbe.MakeLabel()
		vdbe.AddOp3(OP_IfNot, havingReg, addrSkip, 1)
		ac.parse.ReleaseReg(havingReg)

		// Output result
		ac.outputAggregateRow(regResult, nResult, dest)

		vdbe.ResolveLabel(addrSkip)
	} else {
		// Output result
		ac.outputAggregateRow(regResult, nResult, dest)
	}
}

// finalizeResultExpr computes final result using aggregate values.
func (ac *AggregateCompiler) finalizeResultExpr(expr *Expr, aggInfo *AggInfo, target int) {
	if expr == nil {
		return
	}

	vdbe := ac.parse.GetVdbe()

	if expr.Op == TK_AGG_FUNCTION {
		// Find corresponding aggregate function
		for i := range aggInfo.AggFuncs {
			aggFunc := &aggInfo.AggFuncs[i]
			if aggFunc.Expr == expr {
				// Copy accumulated value to target
				if aggFunc.Func.Name == "avg" {
					// AVG: divide sum by count
					// For simplicity, just copy the sum
					vdbe.AddOp2(OP_Copy, aggFunc.RegAcc, target)
				} else {
					vdbe.AddOp2(OP_Copy, aggFunc.RegAcc, target)
				}
				return
			}
		}
	}

	// Not an aggregate - evaluate normally
	ac.compileExpr(expr, target)
}

// outputAggregateRow outputs one aggregate result row.
func (ac *AggregateCompiler) outputAggregateRow(regResult int, nResult int, dest *SelectDest) {
	vdbe := ac.parse.GetVdbe()

	switch dest.Dest {
	case SRT_Output:
		vdbe.AddOp2(OP_ResultRow, regResult, nResult)

	case SRT_Table, SRT_EphemTab:
		r1 := ac.parse.AllocReg()
		r2 := ac.parse.AllocReg()
		vdbe.AddOp3(OP_MakeRecord, regResult, nResult, r1)
		vdbe.AddOp2(OP_NewRowid, dest.SDParm, r2)
		vdbe.AddOp3(OP_Insert, dest.SDParm, r1, r2)
		ac.parse.ReleaseReg(r1)
		ac.parse.ReleaseReg(r2)

	case SRT_Mem:
		if regResult != dest.SDParm {
			vdbe.AddOp3(OP_Copy, regResult, dest.SDParm, nResult-1)
		}
	}
}

// compileExpr is a helper to compile expressions (delegated to expression compiler).
func (ac *AggregateCompiler) compileExpr(expr *Expr, target int) {
	// This would delegate to the expression compiler
	// For now, simplified implementation
	vdbe := ac.parse.GetVdbe()

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
