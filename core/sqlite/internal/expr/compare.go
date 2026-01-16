package expr

import (
	"bytes"
	"math"
	"strconv"
	"strings"
)

// CompareResult represents the result of a comparison.
type CompareResult int

const (
	CmpLess    CompareResult = -1
	CmpEqual   CompareResult = 0
	CmpGreater CompareResult = 1
	CmpNull    CompareResult = 2 // Either operand is NULL
)

// CollSeq represents a collation sequence for string comparison.
type CollSeq struct {
	Name    string
	Compare func(a, b string) int
}

// Standard collation sequences
var (
	CollSeqBinary = &CollSeq{
		Name:    "BINARY",
		Compare: compareBinary,
	}

	CollSeqNoCase = &CollSeq{
		Name:    "NOCASE",
		Compare: compareNoCase,
	}

	CollSeqRTrim = &CollSeq{
		Name:    "RTRIM",
		Compare: compareRTrim,
	}
)

// compareBinary performs binary (byte-by-byte) comparison.
func compareBinary(a, b string) int {
	return strings.Compare(a, b)
}

// compareNoCase performs case-insensitive comparison.
func compareNoCase(a, b string) int {
	return strings.Compare(strings.ToUpper(a), strings.ToUpper(b))
}

// compareRTrim performs comparison with trailing spaces ignored.
func compareRTrim(a, b string) int {
	a = strings.TrimRight(a, " ")
	b = strings.TrimRight(b, " ")
	return strings.Compare(a, b)
}

// GetCollSeq returns the collation sequence for an expression.
// Returns CollSeqBinary if no specific collation is set.
func GetCollSeq(e *Expr) *CollSeq {
	if e == nil {
		return CollSeqBinary
	}

	// Walk up the tree looking for COLLATE operator
	for e != nil {
		if e.Op == OpCollate {
			switch strings.ToUpper(e.CollSeq) {
			case "NOCASE":
				return CollSeqNoCase
			case "RTRIM":
				return CollSeqRTrim
			case "BINARY":
				return CollSeqBinary
			default:
				// Unknown collation, use binary
				return CollSeqBinary
			}
		}

		// For binary operators, check left operand for collation
		if e.HasProperty(EP_Collate) {
			if e.Left != nil && e.Left.HasProperty(EP_Collate) {
				e = e.Left
				continue
			}
		}

		// Check column collation
		if e.Op == OpColumn && e.CollSeq != "" {
			switch strings.ToUpper(e.CollSeq) {
			case "NOCASE":
				return CollSeqNoCase
			case "RTRIM":
				return CollSeqRTrim
			}
		}

		break
	}

	return CollSeqBinary
}

// GetBinaryCompareCollSeq returns the collation for a binary comparison.
// Left operand takes precedence over right operand.
func GetBinaryCompareCollSeq(left, right *Expr) *CollSeq {
	if left != nil && left.HasProperty(EP_Collate) {
		return GetCollSeq(left)
	}
	if right != nil && right.HasProperty(EP_Collate) {
		return GetCollSeq(right)
	}
	if left != nil {
		coll := GetCollSeq(left)
		if coll != CollSeqBinary {
			return coll
		}
	}
	if right != nil {
		return GetCollSeq(right)
	}
	return CollSeqBinary
}

// CompareValues compares two values according to SQLite semantics.
// Returns CmpLess, CmpEqual, CmpGreater, or CmpNull.
func CompareValues(left, right interface{}, aff Affinity, coll *CollSeq) CompareResult {
	// Handle NULL values
	if left == nil || right == nil {
		return CmpNull
	}

	// Apply affinity conversion
	left = ApplyAffinity(left, aff)
	right = ApplyAffinity(right, aff)

	// Compare based on types
	leftInt, leftIsInt := left.(int64)
	rightInt, rightIsInt := right.(int64)

	leftFloat, leftIsFloat := left.(float64)
	rightFloat, rightIsFloat := right.(float64)

	leftStr, leftIsStr := left.(string)
	rightStr, rightIsStr := right.(string)

	leftBlob, leftIsBlob := left.([]byte)
	rightBlob, rightIsBlob := right.([]byte)

	// Integer comparison
	if leftIsInt && rightIsInt {
		if leftInt < rightInt {
			return CmpLess
		}
		if leftInt > rightInt {
			return CmpGreater
		}
		return CmpEqual
	}

	// Float comparison
	if (leftIsInt || leftIsFloat) && (rightIsInt || rightIsFloat) {
		var lf, rf float64
		if leftIsInt {
			lf = float64(leftInt)
		} else {
			lf = leftFloat
		}
		if rightIsInt {
			rf = float64(rightInt)
		} else {
			rf = rightFloat
		}

		// Handle NaN
		if math.IsNaN(lf) || math.IsNaN(rf) {
			return CmpNull
		}

		if lf < rf {
			return CmpLess
		}
		if lf > rf {
			return CmpGreater
		}
		return CmpEqual
	}

	// String comparison
	if leftIsStr && rightIsStr {
		if coll == nil {
			coll = CollSeqBinary
		}
		cmp := coll.Compare(leftStr, rightStr)
		if cmp < 0 {
			return CmpLess
		}
		if cmp > 0 {
			return CmpGreater
		}
		return CmpEqual
	}

	// BLOB comparison
	if leftIsBlob && rightIsBlob {
		cmp := bytes.Compare(leftBlob, rightBlob)
		if cmp < 0 {
			return CmpLess
		}
		if cmp > 0 {
			return CmpGreater
		}
		return CmpEqual
	}

	// Mixed type comparison: use type precedence
	// NULL < INTEGER < REAL < TEXT < BLOB
	leftType := valueType(left)
	rightType := valueType(right)

	if leftType < rightType {
		return CmpLess
	}
	if leftType > rightType {
		return CmpGreater
	}

	// Same type but different Go types (shouldn't happen)
	return CmpEqual
}

// valueType returns a type order for mixed comparisons.
func valueType(v interface{}) int {
	switch v.(type) {
	case nil:
		return 0
	case int64:
		return 1
	case float64:
		return 2
	case string:
		return 3
	case []byte:
		return 4
	default:
		return 5
	}
}

// EvaluateComparison evaluates a comparison expression.
// Returns true, false, or nil (for NULL result).
func EvaluateComparison(op OpCode, left, right interface{}, aff Affinity, coll *CollSeq) interface{} {
	cmp := CompareValues(left, right, aff, coll)

	// Handle NULL propagation (except for IS and IS NOT)
	if cmp == CmpNull {
		switch op {
		case OpIs, OpIsNot:
			// IS and IS NOT don't propagate NULL
		default:
			return nil
		}
	}

	switch op {
	case OpEq:
		if cmp == CmpNull {
			return nil
		}
		return cmp == CmpEqual

	case OpNe:
		if cmp == CmpNull {
			return nil
		}
		return cmp != CmpEqual

	case OpLt:
		if cmp == CmpNull {
			return nil
		}
		return cmp == CmpLess

	case OpLe:
		if cmp == CmpNull {
			return nil
		}
		return cmp == CmpLess || cmp == CmpEqual

	case OpGt:
		if cmp == CmpNull {
			return nil
		}
		return cmp == CmpGreater

	case OpGe:
		if cmp == CmpNull {
			return nil
		}
		return cmp == CmpGreater || cmp == CmpEqual

	case OpIs:
		// IS operator: NULL IS NULL is true
		if left == nil && right == nil {
			return true
		}
		if left == nil || right == nil {
			return false
		}
		return cmp == CmpEqual

	case OpIsNot:
		// IS NOT operator: NULL IS NOT NULL is false
		if left == nil && right == nil {
			return false
		}
		if left == nil || right == nil {
			return true
		}
		return cmp != CmpEqual

	default:
		return nil
	}
}

// EvaluateLike evaluates the LIKE operator.
// pattern: the LIKE pattern (may contain % and _)
// str: the string to match
// escape: the escape character (0 for none)
func EvaluateLike(pattern, str string, escape rune) bool {
	return matchLike(pattern, str, escape, false)
}

// EvaluateGlob evaluates the GLOB operator.
// pattern: the GLOB pattern (may contain * and ?)
// str: the string to match
func EvaluateGlob(pattern, str string) bool {
	return matchLike(pattern, str, 0, true)
}

// matchLike implements LIKE and GLOB pattern matching.
// isGlob: true for GLOB (case-sensitive), false for LIKE (case-insensitive)
func matchLike(pattern, str string, escape rune, isGlob bool) bool {
	// Convert to runes for proper Unicode handling
	pRunes := []rune(pattern)
	sRunes := []rune(str)

	return matchLikeRunes(pRunes, sRunes, escape, isGlob, 0, 0)
}

// matchLikeRunes performs recursive pattern matching.
func matchLikeRunes(pattern, str []rune, escape rune, isGlob bool, pi, si int) bool {
	for pi < len(pattern) {
		pc := pattern[pi]

		// Handle escape character
		if escape != 0 && pc == escape {
			pi++
			if pi >= len(pattern) {
				return false
			}
			pc = pattern[pi]
			if si >= len(str) {
				return false
			}
			if !matchChar(pc, str[si], isGlob) {
				return false
			}
			pi++
			si++
			continue
		}

		// Wildcard matching
		if (isGlob && pc == '*') || (!isGlob && pc == '%') {
			// Match zero or more characters
			pi++
			if pi >= len(pattern) {
				// Trailing wildcard matches everything
				return true
			}

			// Try matching at each position
			for si <= len(str) {
				if matchLikeRunes(pattern, str, escape, isGlob, pi, si) {
					return true
				}
				si++
			}
			return false
		}

		if (isGlob && pc == '?') || (!isGlob && pc == '_') {
			// Match exactly one character
			if si >= len(str) {
				return false
			}
			pi++
			si++
			continue
		}

		// Literal character match
		if si >= len(str) {
			return false
		}
		if !matchChar(pc, str[si], isGlob) {
			return false
		}
		pi++
		si++
	}

	// Pattern exhausted: match if string is also exhausted
	return si >= len(str)
}

// matchChar checks if two characters match.
// For GLOB, comparison is case-sensitive.
// For LIKE, comparison is case-insensitive.
func matchChar(pattern, str rune, isGlob bool) bool {
	if isGlob {
		return pattern == str
	}
	// Case-insensitive for LIKE
	return strings.EqualFold(string(pattern), string(str))
}

// EvaluateBetween evaluates the BETWEEN operator.
// value BETWEEN low AND high
func EvaluateBetween(value, low, high interface{}, aff Affinity, coll *CollSeq) interface{} {
	// value >= low AND value <= high
	cmpLow := CompareValues(value, low, aff, coll)
	cmpHigh := CompareValues(value, high, aff, coll)

	if cmpLow == CmpNull || cmpHigh == CmpNull {
		return nil
	}

	return (cmpLow == CmpGreater || cmpLow == CmpEqual) &&
		(cmpHigh == CmpLess || cmpHigh == CmpEqual)
}

// EvaluateIn evaluates the IN operator.
// value IN (list...)
func EvaluateIn(value interface{}, list []interface{}, aff Affinity, coll *CollSeq) interface{} {
	if value == nil {
		return nil
	}

	hasNull := false
	for _, item := range list {
		if item == nil {
			hasNull = true
			continue
		}

		cmp := CompareValues(value, item, aff, coll)
		if cmp == CmpEqual {
			return true
		}
	}

	// If we found a NULL in the list and no match, result is NULL
	if hasNull {
		return nil
	}

	return false
}

// CoerceToNumeric attempts to convert a value to numeric.
// Returns the numeric value, or the original value if not convertible.
func CoerceToNumeric(v interface{}) interface{} {
	switch val := v.(type) {
	case int64, float64:
		return val
	case string:
		// Try integer first
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
		// Try float
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		// Not numeric
		return val
	default:
		return val
	}
}

// CoerceToInteger attempts to convert a value to integer.
func CoerceToInteger(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case int64:
		return val, true
	case float64:
		return int64(val), true
	case string:
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i, true
		}
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return int64(f), true
		}
		return 0, false
	case bool:
		if val {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

// CoerceToBoolean converts a value to boolean.
// SQLite treats 0 as false, non-zero as true.
func CoerceToBoolean(v interface{}) bool {
	if v == nil {
		return false
	}

	switch val := v.(type) {
	case bool:
		return val
	case int64:
		return val != 0
	case float64:
		return val != 0.0
	case string:
		// Try to parse as number
		if i, ok := CoerceToInteger(val); ok {
			return i != 0
		}
		// Non-numeric strings are considered false
		return false
	default:
		return false
	}
}
