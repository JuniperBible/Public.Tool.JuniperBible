package expr

import (
	"strconv"
	"strings"
)

// GetExprAffinity returns the affinity of an expression.
// This determines how values should be coerced for comparison.
//
// If the expression is a column, returns that column's affinity.
// For CAST expressions, returns the target type's affinity.
// For other expressions, returns NONE.
func GetExprAffinity(e *Expr) Affinity {
	if e == nil {
		return AFF_NONE
	}

	op := e.Op

	// Handle various expression types
	for {
		switch op {
		case OpColumn, OpAggColumn:
			// Column references get affinity from table definition
			// In a real implementation, this would look up the table schema
			// For now, we return the stored affinity
			return e.Affinity

		case OpSelect:
			// Scalar subquery: get affinity from first result column
			if e.Select != nil && e.Select.Columns != nil &&
				len(e.Select.Columns.Items) > 0 {
				return GetExprAffinity(e.Select.Columns.Items[0].Expr)
			}
			return AFF_NONE

		case OpCast:
			// CAST expression: parse the target type
			return AffinityFromType(e.Token)

		case OpSelectColumn:
			// Column from subquery result
			if e.Left != nil && e.Left.Op == OpSelect {
				if e.Left.Select != nil && e.Left.Select.Columns != nil {
					cols := e.Left.Select.Columns.Items
					if e.IColumn >= 0 && e.IColumn < len(cols) {
						return GetExprAffinity(cols[e.IColumn].Expr)
					}
				}
			}
			return AFF_NONE

		case OpVector:
			// Vector expression: get affinity from first element
			if e.List != nil && len(e.List.Items) > 0 {
				return GetExprAffinity(e.List.Items[0].Expr)
			}
			return AFF_NONE

		case OpFunction:
			// Functions with deferred affinity determination
			if e.Affinity == AFF_NONE && e.List != nil && len(e.List.Items) > 0 {
				return GetExprAffinity(e.List.Items[0].Expr)
			}
			return e.Affinity

		case OpCollate, OpUnaryPlus:
			// Skip over these and check the operand
			if e.Left != nil {
				e = e.Left
				op = e.Op
				continue
			}
			return AFF_NONE

		case OpRegister:
			// Register may have original op stored
			if e.IOp2 != 0 {
				op = e.IOp2
				continue
			}
			return e.Affinity

		default:
			// For all other expressions, use stored affinity
			return e.Affinity
		}
	}
}

// AffinityFromType determines affinity from a type name.
// This implements SQLite's type affinity rules:
//   - Contains "INT" -> INTEGER
//   - Contains "CHAR", "CLOB", or "TEXT" -> TEXT
//   - Contains "BLOB" or is empty -> BLOB
//   - Contains "REAL", "FLOA", or "DOUB" -> REAL
//   - Otherwise -> NUMERIC
func AffinityFromType(typeName string) Affinity {
	if typeName == "" {
		return AFF_BLOB
	}

	upper := strings.ToUpper(typeName)

	// INTEGER affinity
	if strings.Contains(upper, "INT") {
		return AFF_INTEGER
	}

	// TEXT affinity
	if strings.Contains(upper, "CHAR") ||
		strings.Contains(upper, "CLOB") ||
		strings.Contains(upper, "TEXT") {
		return AFF_TEXT
	}

	// BLOB affinity
	if strings.Contains(upper, "BLOB") {
		return AFF_BLOB
	}

	// REAL affinity
	if strings.Contains(upper, "REAL") ||
		strings.Contains(upper, "FLOA") ||
		strings.Contains(upper, "DOUB") {
		return AFF_REAL
	}

	// Default to NUMERIC
	return AFF_NUMERIC
}

// CompareAffinity determines the affinity to use when comparing two expressions.
// This implements SQLite's type affinity rules for comparisons.
func CompareAffinity(left, right *Expr) Affinity {
	aff1 := GetExprAffinity(left)
	aff2 := GetExprAffinity(right)

	if aff1 > AFF_NONE && aff2 > AFF_NONE {
		// Both sides are columns
		// If either has numeric affinity, use NUMERIC
		if IsNumericAffinity(aff1) || IsNumericAffinity(aff2) {
			return AFF_NUMERIC
		}
		// Otherwise use BLOB (no affinity conversion)
		return AFF_BLOB
	}

	// One side is a column, use that column's affinity
	if aff1 <= AFF_NONE {
		return aff2
	}
	return aff1
}

// GetComparisonAffinity returns the affinity for a comparison expression.
func GetComparisonAffinity(e *Expr) Affinity {
	if e == nil {
		return AFF_NONE
	}

	// Must be a comparison operator
	switch e.Op {
	case OpEq, OpNe, OpLt, OpLe, OpGt, OpGe, OpIs, OpIsNot:
		// OK
	default:
		return AFF_NONE
	}

	if e.Left == nil {
		return AFF_BLOB
	}

	aff := GetExprAffinity(e.Left)
	if e.Right != nil {
		aff = CompareAffinity(e.Right, e.Left)
	} else if e.HasProperty(EP_xIsSelect) && e.Select != nil {
		// Comparing with subquery result
		if e.Select.Columns != nil && len(e.Select.Columns.Items) > 0 {
			rightAff := GetExprAffinity(e.Select.Columns.Items[0].Expr)
			aff = CompareAffinity(&Expr{Affinity: rightAff}, e.Left)
		}
	}

	if aff == AFF_NONE {
		aff = AFF_BLOB
	}

	return aff
}

// ApplyAffinity converts a value according to the specified affinity.
// This is a simplified version - a real implementation would work with
// actual SQLite values.
func ApplyAffinity(value interface{}, aff Affinity) interface{} {
	if value == nil {
		return nil
	}

	switch aff {
	case AFF_INTEGER:
		// Try to convert to integer
		switch v := value.(type) {
		case int64:
			return v
		case float64:
			return int64(v)
		case string:
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				return i
			}
			// Try as float first
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return int64(f)
			}
			// Can't convert, keep as string
			return v
		default:
			return value
		}

	case AFF_REAL, AFF_NUMERIC:
		// Try to convert to numeric
		switch v := value.(type) {
		case float64:
			return v
		case int64:
			return float64(v)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
			// For NUMERIC, also try integer
			if aff == AFF_NUMERIC {
				if i, err := strconv.ParseInt(v, 10, 64); err == nil {
					return i
				}
			}
			// Can't convert, keep as string
			return v
		default:
			return value
		}

	case AFF_TEXT:
		// Convert to text
		switch v := value.(type) {
		case string:
			return v
		case int:
			return strconv.Itoa(v)
		case int64:
			return strconv.FormatInt(v, 10)
		case float64:
			return strconv.FormatFloat(v, 'g', -1, 64)
		default:
			return value
		}

	case AFF_BLOB, AFF_NONE:
		// No conversion
		return value

	default:
		return value
	}
}

// SetTableColumnAffinity sets the affinity for a column expression based
// on the table schema. This would be called during name resolution.
func SetTableColumnAffinity(e *Expr, colType string) {
	if e == nil || e.Op != OpColumn {
		return
	}
	e.Affinity = AffinityFromType(colType)
}

// PropagateAffinity propagates affinity information through an expression tree.
// This is called during semantic analysis.
func PropagateAffinity(e *Expr) {
	if e == nil {
		return
	}

	// Recursively process children
	if e.Left != nil {
		PropagateAffinity(e.Left)
	}
	if e.Right != nil {
		PropagateAffinity(e.Right)
	}
	if e.List != nil {
		for _, item := range e.List.Items {
			PropagateAffinity(item.Expr)
		}
	}

	// Set affinity based on operation
	switch e.Op {
	case OpPlus, OpMinus, OpMultiply, OpDivide, OpRemainder:
		// Arithmetic operators produce numeric results
		e.Affinity = AFF_NUMERIC

	case OpConcat:
		// String concatenation produces text
		e.Affinity = AFF_TEXT

	case OpBitAnd, OpBitOr, OpBitXor, OpLShift, OpRShift:
		// Bitwise operators produce integers
		e.Affinity = AFF_INTEGER

	case OpEq, OpNe, OpLt, OpLe, OpGt, OpGe,
		OpAnd, OpOr, OpNot, OpIs, OpIsNot,
		OpIsNull, OpNotNull, OpIn, OpNotIn,
		OpBetween, OpNotBetween, OpLike, OpGlob, OpExists:
		// Comparison and logical operators produce boolean (integer 0/1)
		e.Affinity = AFF_INTEGER

	case OpNegate:
		// Unary minus preserves numeric affinity
		if e.Left != nil {
			aff := GetExprAffinity(e.Left)
			if IsNumericAffinity(aff) {
				e.Affinity = aff
			} else {
				e.Affinity = AFF_NUMERIC
			}
		}

	case OpCase:
		// CASE expression: affinity from THEN/ELSE clauses
		// Look at all THEN clauses and ELSE clause
		if e.List != nil {
			var resultAff Affinity = AFF_NONE
			// CASE list is: WHEN1, THEN1, WHEN2, THEN2, ..., [ELSE]
			for i := 1; i < len(e.List.Items); i += 2 {
				thenAff := GetExprAffinity(e.List.Items[i].Expr)
				if resultAff == AFF_NONE {
					resultAff = thenAff
				} else if resultAff != thenAff {
					// Different affinities, use NONE
					resultAff = AFF_NONE
				}
			}
			// Check ELSE clause
			if len(e.List.Items)%2 == 1 {
				elseIdx := len(e.List.Items) - 1
				elseAff := GetExprAffinity(e.List.Items[elseIdx].Expr)
				if resultAff == AFF_NONE {
					resultAff = elseAff
				} else if resultAff != elseAff {
					resultAff = AFF_NONE
				}
			}
			e.Affinity = resultAff
		}

	default:
		// For other operations, affinity is already set or remains NONE
	}
}
