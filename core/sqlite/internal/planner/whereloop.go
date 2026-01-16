package planner

import (
	"fmt"
)

// WhereLoopBuilder is responsible for generating WhereLoop objects
// for all possible access paths for a given table.
type WhereLoopBuilder struct {
	// Table being analyzed
	Table *TableInfo

	// WHERE clause terms that apply to this table
	Terms []*WhereTerm

	// Cost model for estimation
	CostModel *CostModel

	// Generated loops
	Loops []*WhereLoop

	// Cursor is the cursor number for this table
	Cursor int

	// NotReady is bitmask of tables not yet processed
	NotReady Bitmask
}

// NewWhereLoopBuilder creates a new builder for a table.
func NewWhereLoopBuilder(table *TableInfo, cursor int, terms []*WhereTerm, costModel *CostModel) *WhereLoopBuilder {
	return &WhereLoopBuilder{
		Table:     table,
		Cursor:    cursor,
		Terms:     terms,
		CostModel: costModel,
		Loops:     make([]*WhereLoop, 0),
	}
}

// Build generates all possible WhereLoop objects for this table.
func (b *WhereLoopBuilder) Build() []*WhereLoop {
	// Always generate a full table scan option
	b.addFullScan()

	// Generate index scan options for each index
	for _, index := range b.Table.Indexes {
		b.addIndexScans(index)
	}

	// Generate primary key lookup if applicable
	if b.Table.PrimaryKey != nil {
		b.addPrimaryKeyLookup()
	}

	return b.Loops
}

// addFullScan adds a full table scan WhereLoop.
func (b *WhereLoopBuilder) addFullScan() {
	cost, nOut := b.CostModel.EstimateFullScan(b.Table)

	loop := &WhereLoop{
		TabIndex: b.Cursor,
		Setup:    0,
		Run:      cost,
		NOut:     nOut,
		Flags:    0, // No special flags for full scan
		Index:    nil,
		Terms:    make([]*WhereTerm, 0),
	}

	// Set bitmask for this table
	loop.MaskSelf.Set(b.Cursor)

	// Apply all WHERE terms that reference only this table
	b.applyTermsToLoop(loop)

	b.Loops = append(b.Loops, loop)
}

// addIndexScans adds WhereLoop objects for all possible ways to use an index.
func (b *WhereLoopBuilder) addIndexScans(index *IndexInfo) {
	// Try using increasing numbers of index columns
	for nCol := 1; nCol <= len(index.Columns); nCol++ {
		b.addIndexScanWithColumns(index, nCol)
	}
}

// addIndexScanWithColumns adds loops for using nCol columns of an index.
func (b *WhereLoopBuilder) addIndexScanWithColumns(index *IndexInfo, nCol int) {
	// Find terms that can use the first nCol index columns
	usableTerms := b.findUsableTerms(index, nCol)
	if len(usableTerms) == 0 {
		return // No terms can use this index prefix
	}

	// Count equality constraints
	nEq := 0
	hasRange := false
	for _, term := range usableTerms {
		if term.Operator == WO_EQ {
			nEq++
		} else if term.Operator&(WO_LT|WO_LE|WO_GT|WO_GE) != 0 {
			hasRange = true
		}
	}

	// Check if index is covering (for SELECT *)
	// In a real implementation, we'd need to know which columns are needed
	covering := false // Simplified: assume not covering

	// Estimate cost
	cost, nOut := b.CostModel.EstimateIndexScan(b.Table, index, usableTerms, nEq, hasRange, covering)

	// Determine flags
	flags := WHERE_INDEXED
	if nEq > 0 {
		flags |= WHERE_COLUMN_EQ
	}
	if hasRange {
		flags |= WHERE_COLUMN_RANGE
		if hasLowerBound(usableTerms) {
			flags |= WHERE_BTM_LIMIT
		}
		if hasUpperBound(usableTerms) {
			flags |= WHERE_TOP_LIMIT
		}
	}
	if covering {
		flags |= WHERE_IDX_ONLY
	}

	// Check if this gives exactly one row (unique index with all columns = const)
	if index.Unique && nEq >= len(index.Columns) {
		flags |= WHERE_ONEROW
	}

	loop := &WhereLoop{
		TabIndex: b.Cursor,
		Setup:    0,
		Run:      cost,
		NOut:     nOut,
		Flags:    flags,
		Index:    index,
		Terms:    usableTerms,
	}

	// Set bitmask for this table
	loop.MaskSelf.Set(b.Cursor)

	// Set prerequisites (tables that must be evaluated first)
	b.setPrerequisites(loop)

	b.Loops = append(b.Loops, loop)

	// Also try with IN operator if applicable
	b.tryInOperator(index, nCol, usableTerms)
}

// addPrimaryKeyLookup adds a WhereLoop for direct rowid/primary key lookup.
func (b *WhereLoopBuilder) addPrimaryKeyLookup() {
	// Find term that constrains the primary key
	var pkTerm *WhereTerm
	for _, term := range b.Terms {
		if term.LeftCursor == b.Cursor && term.LeftColumn == -1 { // -1 = rowid
			if term.Operator == WO_EQ {
				pkTerm = term
				break
			}
		}
	}

	if pkTerm == nil {
		return // No primary key constraint
	}

	cost, nOut := b.CostModel.EstimateRowidLookup()

	loop := &WhereLoop{
		TabIndex: b.Cursor,
		Setup:    0,
		Run:      cost,
		NOut:     nOut,
		Flags:    WHERE_IPK | WHERE_COLUMN_EQ | WHERE_ONEROW,
		Index:    b.Table.PrimaryKey,
		Terms:    []*WhereTerm{pkTerm},
	}

	loop.MaskSelf.Set(b.Cursor)
	b.setPrerequisites(loop)

	b.Loops = append(b.Loops, loop)
}

// findUsableTerms finds WHERE terms that can use the first nCol columns of an index.
func (b *WhereLoopBuilder) findUsableTerms(index *IndexInfo, nCol int) []*WhereTerm {
	usable := make([]*WhereTerm, 0)

	// Check each index column
	for i := 0; i < nCol && i < len(index.Columns); i++ {
		colIdx := index.Columns[i].Index

		// Find term for this column
		var termForCol *WhereTerm
		for _, term := range b.Terms {
			if term.LeftCursor == b.Cursor && term.LeftColumn == colIdx {
				// This term references the column
				if isUsableOperator(term.Operator) {
					// Once we find a non-equality, we can't use further columns
					termForCol = term
					break
				}
			}
		}

		if termForCol == nil {
			// No term for this column - can't use remaining columns
			break
		}

		usable = append(usable, termForCol)

		// If this is not an equality, we can't use further columns
		if termForCol.Operator != WO_EQ {
			break
		}
	}

	return usable
}

// isUsableOperator checks if an operator can be used with an index.
func isUsableOperator(op WhereOperator) bool {
	return op&(WO_EQ|WO_LT|WO_LE|WO_GT|WO_GE|WO_IN|WO_ISNULL) != 0
}

// hasLowerBound checks if terms include a lower bound (> or >=).
func hasLowerBound(terms []*WhereTerm) bool {
	for _, term := range terms {
		if term.Operator&(WO_GT|WO_GE) != 0 {
			return true
		}
	}
	return false
}

// hasUpperBound checks if terms include an upper bound (< or <=).
func hasUpperBound(terms []*WhereTerm) bool {
	for _, term := range terms {
		if term.Operator&(WO_LT|WO_LE) != 0 {
			return true
		}
	}
	return false
}

// tryInOperator attempts to optimize using IN operator with an index.
func (b *WhereLoopBuilder) tryInOperator(index *IndexInfo, nCol int, baseTerms []*WhereTerm) {
	// Look for an IN operator on the next column
	if nCol >= len(index.Columns) {
		return
	}

	nextColIdx := index.Columns[nCol].Index

	var inTerm *WhereTerm
	for _, term := range b.Terms {
		if term.LeftCursor == b.Cursor && term.LeftColumn == nextColIdx {
			if term.Operator == WO_IN {
				inTerm = term
				break
			}
		}
	}

	if inTerm == nil {
		return
	}

	// Estimate IN list size (simplified)
	inListSize := 5 // Default assumption

	// Count equalities in base terms
	nEq := 0
	for _, term := range baseTerms {
		if term.Operator == WO_EQ {
			nEq++
		}
	}

	// Check covering
	covering := false // Simplified

	cost, nOut := b.CostModel.EstimateInOperator(b.Table, index, nEq, inListSize, covering)

	// Create terms list including IN term
	terms := make([]*WhereTerm, len(baseTerms)+1)
	copy(terms, baseTerms)
	terms[len(baseTerms)] = inTerm

	flags := WHERE_INDEXED | WHERE_COLUMN_IN | WHERE_IN_ABLE
	if nEq > 0 {
		flags |= WHERE_COLUMN_EQ
	}
	if covering {
		flags |= WHERE_IDX_ONLY
	}

	loop := &WhereLoop{
		TabIndex: b.Cursor,
		Setup:    0,
		Run:      cost,
		NOut:     nOut,
		Flags:    flags,
		Index:    index,
		Terms:    terms,
	}

	loop.MaskSelf.Set(b.Cursor)
	b.setPrerequisites(loop)

	b.Loops = append(b.Loops, loop)
}

// applyTermsToLoop applies WHERE terms to refine cost of a full table scan.
func (b *WhereLoopBuilder) applyTermsToLoop(loop *WhereLoop) {
	selectivity := LogEst(0)

	for _, term := range b.Terms {
		// Check if term applies to this table only
		if term.LeftCursor == b.Cursor && term.PrereqRight == 0 {
			// Apply truth probability to reduce estimated rows
			prob := b.CostModel.EstimateTruthProbability(term)
			selectivity += prob

			// Add term to loop
			loop.Terms = append(loop.Terms, term)
		}
	}

	// Adjust output rows based on selectivity
	loop.NOut += selectivity
	if loop.NOut < 0 {
		loop.NOut = 0 // At least 1 row
	}
}

// setPrerequisites determines which tables must be evaluated before this loop.
func (b *WhereLoopBuilder) setPrerequisites(loop *WhereLoop) {
	prereq := Bitmask(0)

	for _, term := range loop.Terms {
		// If term's right side references other tables, they're prerequisites
		prereq |= term.PrereqRight
	}

	// Remove this table itself from prerequisites
	prereq &= ^loop.MaskSelf

	loop.Prereq = prereq
}

// OptimizeForSkipScan checks if skip-scan optimization is possible.
// Skip-scan allows using an index even when the first column is not constrained.
func (b *WhereLoopBuilder) OptimizeForSkipScan(index *IndexInfo) *WhereLoop {
	// Skip-scan is beneficial when:
	// 1. First column has low cardinality
	// 2. Later column has constraint
	// 3. Cost of scanning all first-column values is less than full table scan

	if len(index.Columns) < 2 {
		return nil
	}

	// Check if first column is unconstrained but later columns are constrained
	firstColIdx := index.Columns[0].Index
	hasFirstCol := false
	for _, term := range b.Terms {
		if term.LeftCursor == b.Cursor && term.LeftColumn == firstColIdx {
			hasFirstCol = true
			break
		}
	}

	if hasFirstCol {
		return nil // First column is already constrained, normal index scan is better
	}

	// Find terms for later columns
	laterTerms := make([]*WhereTerm, 0)
	for i := 1; i < len(index.Columns); i++ {
		colIdx := index.Columns[i].Index
		for _, term := range b.Terms {
			if term.LeftCursor == b.Cursor && term.LeftColumn == colIdx {
				laterTerms = append(laterTerms, term)
				break
			}
		}
	}

	if len(laterTerms) == 0 {
		return nil // No constraints on later columns
	}

	// Estimate distinct values in first column (simplified)
	distinctFirst := LogEst(40) // Assume ~10 distinct values (2^4 = 16, rounded up)

	// Estimate cost: seek for each distinct value in first column
	// For each value, scan matching rows based on later column constraints
	nEq := 0
	for _, term := range laterTerms {
		if term.Operator == WO_EQ {
			nEq++
		}
	}

	baseCost, baseNOut := b.CostModel.EstimateIndexScan(b.Table, index, laterTerms, nEq, false, false)

	// Total cost = cost per first-column value * number of distinct values
	cost := distinctFirst.Add(baseCost)
	nOut := distinctFirst.Add(baseNOut)

	// Only use skip-scan if cheaper than full table scan
	fullScanCost, _ := b.CostModel.EstimateFullScan(b.Table)
	if cost >= fullScanCost {
		return nil
	}

	loop := &WhereLoop{
		TabIndex: b.Cursor,
		Setup:    0,
		Run:      cost,
		NOut:     nOut,
		Flags:    WHERE_INDEXED | WHERE_SKIPSCAN,
		Index:    index,
		Terms:    laterTerms,
	}

	loop.MaskSelf.Set(b.Cursor)
	b.setPrerequisites(loop)

	return loop
}

// String returns a string representation of a WhereLoop for debugging.
func (loop *WhereLoop) String() string {
	s := fmt.Sprintf("WhereLoop[tab=%d", loop.TabIndex)

	if loop.Index != nil {
		s += fmt.Sprintf(" index=%s", loop.Index.Name)
	} else {
		s += " scan=FULL"
	}

	s += fmt.Sprintf(" cost=%d nOut=%d", loop.Run, loop.NOut)

	if loop.Flags&WHERE_ONEROW != 0 {
		s += " ONEROW"
	}
	if loop.Flags&WHERE_COLUMN_EQ != 0 {
		s += " EQ"
	}
	if loop.Flags&WHERE_COLUMN_RANGE != 0 {
		s += " RANGE"
	}
	if loop.Flags&WHERE_COLUMN_IN != 0 {
		s += " IN"
	}
	if loop.Flags&WHERE_IDX_ONLY != 0 {
		s += " COVERING"
	}
	if loop.Flags&WHERE_SKIPSCAN != 0 {
		s += " SKIPSCAN"
	}

	s += "]"
	return s
}

// Clone creates a deep copy of a WhereLoop.
func (loop *WhereLoop) Clone() *WhereLoop {
	clone := &WhereLoop{
		Prereq:   loop.Prereq,
		MaskSelf: loop.MaskSelf,
		TabIndex: loop.TabIndex,
		Setup:    loop.Setup,
		Run:      loop.Run,
		NOut:     loop.NOut,
		Flags:    loop.Flags,
		Index:    loop.Index,
		NextLoop: nil, // Don't copy the linked list pointer
	}

	// Copy terms
	clone.Terms = make([]*WhereTerm, len(loop.Terms))
	copy(clone.Terms, loop.Terms)

	return clone
}
