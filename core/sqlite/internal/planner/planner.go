package planner

import (
	"fmt"
	"sort"
)

// Planner is the main query planner that generates execution plans.
type Planner struct {
	CostModel *CostModel
}

// NewPlanner creates a new query planner.
func NewPlanner() *Planner {
	return &Planner{
		CostModel: NewCostModel(),
	}
}

// PlanQuery generates an execution plan for a query.
func (p *Planner) PlanQuery(tables []*TableInfo, whereClause *WhereClause) (*WhereInfo, error) {
	if len(tables) == 0 {
		return nil, fmt.Errorf("no tables in query")
	}

	info := &WhereInfo{
		Clause:   whereClause,
		Tables:   tables,
		AllLoops: make([]*WhereLoop, 0),
	}

	// Phase 1: Analyze WHERE clause and split into terms
	if whereClause != nil {
		// Terms are already split in WhereClause
	}

	// Phase 2: Generate all possible WhereLoop objects for each table
	for cursor, table := range tables {
		loops := p.generateLoops(table, cursor, whereClause)
		info.AllLoops = append(info.AllLoops, loops...)
	}

	// Phase 3: Find the optimal query plan
	bestPath, err := p.findBestPath(info)
	if err != nil {
		return nil, err
	}

	info.BestPath = bestPath
	info.NOut = bestPath.NRow

	return info, nil
}

// generateLoops generates all WhereLoop options for a single table.
func (p *Planner) generateLoops(table *TableInfo, cursor int, whereClause *WhereClause) []*WhereLoop {
	// Filter terms that apply to this table
	var terms []*WhereTerm
	if whereClause != nil {
		terms = make([]*WhereTerm, 0)
		for _, term := range whereClause.Terms {
			if term.LeftCursor == cursor {
				terms = append(terms, term)
			}
		}
	}

	// Build all possible access paths
	builder := NewWhereLoopBuilder(table, cursor, terms, p.CostModel)
	loops := builder.Build()

	// Also try skip-scan optimization for each index
	for _, index := range table.Indexes {
		if skipLoop := builder.OptimizeForSkipScan(index); skipLoop != nil {
			loops = append(loops, skipLoop)
		}
	}

	return loops
}

// findBestPath finds the optimal sequence of WhereLoops (one per table).
// This implements a dynamic programming algorithm similar to SQLite's solver.
func (p *Planner) findBestPath(info *WhereInfo) (*WherePath, error) {
	nTables := len(info.Tables)

	if nTables == 1 {
		// Single table: just pick the best loop
		return p.findBestSingleTable(info)
	}

	// Multi-table join: use dynamic programming
	return p.findBestMultiTable(info)
}

// findBestSingleTable finds the best plan for a single table.
func (p *Planner) findBestSingleTable(info *WhereInfo) (*WherePath, error) {
	// Find all loops for the single table
	loops := make([]*WhereLoop, 0)
	for _, loop := range info.AllLoops {
		if loop.TabIndex == 0 {
			loops = append(loops, loop)
		}
	}

	if len(loops) == 0 {
		return nil, fmt.Errorf("no access paths found")
	}

	// Select the best loop
	bestLoop := p.CostModel.SelectBestLoop(loops)

	path := &WherePath{
		MaskLoop: bestLoop.MaskSelf,
		NRow:     bestLoop.NOut,
		Cost:     p.CostModel.CalculateLoopCost(bestLoop),
		Loops:    []*WhereLoop{bestLoop},
	}

	return path, nil
}

// findBestMultiTable finds the best plan for multiple tables using dynamic programming.
func (p *Planner) findBestMultiTable(info *WhereInfo) (*WherePath, error) {
	nTables := len(info.Tables)

	// We'll use a simplified version of SQLite's algorithm
	// Track the N best partial paths of each length

	const N = 5 // Keep top 5 paths at each level

	// Start with empty path
	currentPaths := []*WherePath{{
		MaskLoop: 0,
		NRow:     0,
		Cost:     0,
		Loops:    make([]*WhereLoop, 0),
	}}

	// For each table position in the join
	for level := 0; level < nTables; level++ {
		nextPaths := make([]*WherePath, 0)

		// For each current partial path
		for _, path := range currentPaths {
			// Try extending with each possible table
			for cursor := 0; cursor < nTables; cursor++ {
				// Skip if table already in path
				mask := Bitmask(1 << uint(cursor))
				if path.MaskLoop.Overlaps(mask) {
					continue
				}

				// Find loops for this table
				candidateLoops := p.findLoopsForTable(info, cursor)

				// Try each loop
				for _, loop := range candidateLoops {
					// Check prerequisites are satisfied
					if !path.MaskLoop.HasAll(loop.Prereq) {
						continue
					}

					// Create extended path
					newPath := p.extendPath(path, loop)
					nextPaths = append(nextPaths, newPath)
				}
			}
		}

		if len(nextPaths) == 0 {
			return nil, fmt.Errorf("no valid join order found at level %d", level)
		}

		// Keep only the best N paths
		currentPaths = p.selectBestPaths(nextPaths, N)
	}

	// Return the best complete path
	if len(currentPaths) == 0 {
		return nil, fmt.Errorf("no complete path found")
	}

	return currentPaths[0], nil
}

// findLoopsForTable finds all loops for a specific table.
func (p *Planner) findLoopsForTable(info *WhereInfo, cursor int) []*WhereLoop {
	loops := make([]*WhereLoop, 0)
	for _, loop := range info.AllLoops {
		if loop.TabIndex == cursor {
			loops = append(loops, loop)
		}
	}
	return loops
}

// extendPath extends a partial path with a new loop.
func (p *Planner) extendPath(path *WherePath, loop *WhereLoop) *WherePath {
	newPath := &WherePath{
		MaskLoop: path.MaskLoop | loop.MaskSelf,
		Loops:    make([]*WhereLoop, len(path.Loops)+1),
	}

	copy(newPath.Loops, path.Loops)
	newPath.Loops[len(path.Loops)] = loop

	// Calculate combined cost and row count
	newPath.Cost, newPath.NRow = p.CostModel.CombineLoopCosts(newPath.Loops)

	return newPath
}

// selectBestPaths selects the top N paths by cost.
func (p *Planner) selectBestPaths(paths []*WherePath, n int) []*WherePath {
	// Sort by cost (ascending)
	sort.Slice(paths, func(i, j int) bool {
		// Primary: cost
		if paths[i].Cost != paths[j].Cost {
			return paths[i].Cost < paths[j].Cost
		}
		// Tie-breaker: output rows
		return paths[i].NRow < paths[j].NRow
	})

	// Keep top N
	if len(paths) > n {
		paths = paths[:n]
	}

	return paths
}

// OptimizeWhereClause analyzes and optimizes WHERE clause terms.
func (p *Planner) OptimizeWhereClause(expr Expr, tables []*TableInfo) (*WhereClause, error) {
	clause := &WhereClause{
		Terms: make([]*WhereTerm, 0),
	}

	// Split AND terms
	andTerms := p.splitAnd(expr)

	// Convert each term to WhereTerm
	for _, term := range andTerms {
		whereTerm, err := p.analyzeExpr(term, tables)
		if err != nil {
			return nil, err
		}
		if whereTerm != nil {
			clause.Terms = append(clause.Terms, whereTerm)
			if whereTerm.Operator == WO_OR {
				clause.HasOr = true
			}
		}
	}

	// Apply transitive closure (if a=b and b=c, add a=c)
	p.applyTransitiveClosure(clause)

	return clause, nil
}

// splitAnd recursively splits AND expressions.
func (p *Planner) splitAnd(expr Expr) []Expr {
	if andExpr, ok := expr.(*AndExpr); ok {
		result := make([]Expr, 0)
		for _, term := range andExpr.Terms {
			result = append(result, p.splitAnd(term)...)
		}
		return result
	}
	return []Expr{expr}
}

// analyzeExpr analyzes a single expression and creates a WhereTerm.
func (p *Planner) analyzeExpr(expr Expr, tables []*TableInfo) (*WhereTerm, error) {
	binExpr, ok := expr.(*BinaryExpr)
	if !ok {
		// Not a binary expression, might be OR or other complex expr
		if orExpr, ok := expr.(*OrExpr); ok {
			return p.analyzeOrExpr(orExpr, tables)
		}
		return nil, nil
	}

	// Extract column reference
	colExpr, ok := binExpr.Left.(*ColumnExpr)
	if !ok {
		return nil, nil
	}

	// Determine operator
	op := p.parseOperator(binExpr.Op)
	if op == 0 {
		return nil, nil // Unknown operator
	}

	// Find column index
	colIdx := -1
	for i, col := range tables[colExpr.Cursor].Columns {
		if col.Name == colExpr.Column {
			colIdx = i
			break
		}
	}

	// Extract right-side value
	var rightValue interface{}
	if valExpr, ok := binExpr.Right.(*ValueExpr); ok {
		rightValue = valExpr.Value
	}

	term := &WhereTerm{
		Expr:        expr,
		Operator:    op,
		LeftCursor:  colExpr.Cursor,
		LeftColumn:  colIdx,
		RightValue:  rightValue,
		PrereqRight: binExpr.Right.UsedTables(),
		PrereqAll:   expr.UsedTables(),
		TruthProb:   0, // Will be estimated later
		Flags:       0,
		Parent:      -1,
	}

	return term, nil
}

// analyzeOrExpr handles OR expressions specially.
func (p *Planner) analyzeOrExpr(expr *OrExpr, tables []*TableInfo) (*WhereTerm, error) {
	term := &WhereTerm{
		Expr:       expr,
		Operator:   WO_OR,
		LeftCursor: -1,
		LeftColumn: -1,
		PrereqAll:  expr.UsedTables(),
		TruthProb:  0,
		Flags:      0,
		Parent:     -1,
	}

	return term, nil
}

// parseOperator converts string operator to WhereOperator.
func (p *Planner) parseOperator(op string) WhereOperator {
	switch op {
	case "=":
		return WO_EQ
	case "<":
		return WO_LT
	case "<=":
		return WO_LE
	case ">":
		return WO_GT
	case ">=":
		return WO_GE
	case "IN":
		return WO_IN
	case "IS":
		return WO_IS
	case "IS NULL":
		return WO_ISNULL
	default:
		return 0
	}
}

// applyTransitiveClosure adds implied constraints from transitive relationships.
// For example, if we have a=b and b=5, we can infer a=5.
func (p *Planner) applyTransitiveClosure(clause *WhereClause) {
	// Build equivalence classes
	equiv := make(map[string][]string) // map column to equivalent columns

	// Find equality constraints between columns
	for _, term := range clause.Terms {
		if term.Operator != WO_EQ {
			continue
		}

		// Check if right side is also a column
		if colExpr, ok := term.Expr.(*BinaryExpr); ok {
			if rightCol, ok := colExpr.Right.(*ColumnExpr); ok {
				leftKey := fmt.Sprintf("%d.%d", term.LeftCursor, term.LeftColumn)
				rightKey := fmt.Sprintf("%d.%d", rightCol.Cursor, rightCol.UsedTables())

				// Add to equivalence class
				equiv[leftKey] = append(equiv[leftKey], rightKey)
				equiv[rightKey] = append(equiv[rightKey], leftKey)
			}
		}
	}

	// For each constant constraint, propagate to equivalent columns
	newTerms := make([]*WhereTerm, 0)
	for _, term := range clause.Terms {
		if term.Operator != WO_EQ {
			continue
		}

		// Check if this is column = constant
		if _, ok := term.Expr.(*BinaryExpr); ok {
			if term.RightValue != nil {
				// This is column = constant
				leftKey := fmt.Sprintf("%d.%d", term.LeftCursor, term.LeftColumn)

				// Add equivalent constraints
				for _, equivKey := range equiv[leftKey] {
					// Parse equivalent column
					var cursor, column int
					fmt.Sscanf(equivKey, "%d.%d", &cursor, &column)

					// Create new term
					newTerm := &WhereTerm{
						Expr:        term.Expr, // Simplified
						Operator:    WO_EQ,
						LeftCursor:  cursor,
						LeftColumn:  column,
						RightValue:  term.RightValue,
						PrereqRight: 0,
						PrereqAll:   term.PrereqAll,
						TruthProb:   term.TruthProb,
						Flags:       TERM_VIRTUAL, // Mark as generated
						Parent:      -1,
					}
					newTerms = append(newTerms, newTerm)
				}
			}
		}
	}

	// Add new terms to clause
	clause.Terms = append(clause.Terms, newTerms...)
}

// ExplainPlan returns a human-readable explanation of the query plan.
func (p *Planner) ExplainPlan(info *WhereInfo) string {
	if info.BestPath == nil {
		return "No plan available"
	}

	result := "QUERY PLAN:\n"
	result += fmt.Sprintf("Estimated output rows: %d\n", info.BestPath.NRow.ToInt())
	result += fmt.Sprintf("Estimated cost: %d\n\n", info.BestPath.Cost)

	for i, loop := range info.BestPath.Loops {
		table := info.Tables[loop.TabIndex]
		indent := ""
		for j := 0; j < i; j++ {
			indent += "  "
		}

		if loop.Index != nil {
			result += fmt.Sprintf("%s%d. SEARCH %s USING INDEX %s",
				indent, i+1, table.Name, loop.Index.Name)

			// Add constraint details
			constraints := make([]string, 0)
			for _, term := range loop.Terms {
				if term.LeftColumn >= 0 && term.LeftColumn < len(table.Columns) {
					col := table.Columns[term.LeftColumn].Name
					op := operatorString(term.Operator)
					constraints = append(constraints, fmt.Sprintf("%s%s?", col, op))
				}
			}

			if len(constraints) > 0 {
				result += " (" + joinStrings(constraints, " AND ") + ")"
			}

			result += "\n"
		} else {
			result += fmt.Sprintf("%s%d. SCAN %s\n", indent, i+1, table.Name)
		}

		result += fmt.Sprintf("%s   Cost: %d, Rows: %d\n",
			indent, loop.Run.ToInt(), loop.NOut.ToInt())
	}

	return result
}

// joinStrings joins strings with a separator (helper function).
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// ValidatePlan performs sanity checks on a query plan.
func (p *Planner) ValidatePlan(info *WhereInfo) error {
	if info.BestPath == nil {
		return fmt.Errorf("no plan generated")
	}

	// Check that we have a loop for each table
	if len(info.BestPath.Loops) != len(info.Tables) {
		return fmt.Errorf("plan has %d loops but %d tables",
			len(info.BestPath.Loops), len(info.Tables))
	}

	// Check that each table appears exactly once
	seen := make(map[int]bool)
	for _, loop := range info.BestPath.Loops {
		if seen[loop.TabIndex] {
			return fmt.Errorf("table %d appears multiple times in plan", loop.TabIndex)
		}
		seen[loop.TabIndex] = true
	}

	// Check that prerequisites are satisfied
	available := Bitmask(0)
	for _, loop := range info.BestPath.Loops {
		if !available.HasAll(loop.Prereq) {
			return fmt.Errorf("prerequisites not satisfied for table %d", loop.TabIndex)
		}
		available |= loop.MaskSelf
	}

	return nil
}
