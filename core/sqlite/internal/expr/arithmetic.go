package expr

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// EvaluateArithmetic evaluates an arithmetic expression.
// Returns the result or nil if the operation produces NULL.
func EvaluateArithmetic(op OpCode, left, right interface{}) interface{} {
	// Handle NULL propagation
	if left == nil || right == nil {
		return nil
	}

	// Convert operands to numeric values
	leftNum := CoerceToNumeric(left)
	rightNum := CoerceToNumeric(right)

	// Extract numeric values
	leftInt, leftIsInt := leftNum.(int64)
	rightInt, rightIsInt := rightNum.(int64)

	leftFloat, leftIsFloat := leftNum.(float64)
	rightFloat, rightIsFloat := rightNum.(float64)

	// If neither operand is numeric, return NULL
	if !leftIsInt && !leftIsFloat {
		return nil
	}
	if !rightIsInt && !rightIsFloat {
		return nil
	}

	// Perform the operation
	switch op {
	case OpPlus:
		return add(leftInt, leftIsInt, rightInt, rightIsInt, leftFloat, rightFloat)

	case OpMinus:
		return subtract(leftInt, leftIsInt, rightInt, rightIsInt, leftFloat, rightFloat)

	case OpMultiply:
		return multiply(leftInt, leftIsInt, rightInt, rightIsInt, leftFloat, rightFloat)

	case OpDivide:
		return divide(leftInt, leftIsInt, rightInt, rightIsInt, leftFloat, rightFloat)

	case OpRemainder:
		return remainder(leftInt, leftIsInt, rightInt, rightIsInt, leftFloat, rightFloat)

	default:
		return nil
	}
}

// add performs addition.
func add(li int64, lis bool, ri int64, ris bool, lf, rf float64) interface{} {
	// If both are integers, try integer addition
	if lis && ris {
		// Check for overflow
		result := li + ri
		// Overflow check: if signs are same and result has different sign
		if (li > 0 && ri > 0 && result < 0) || (li < 0 && ri < 0 && result > 0) {
			// Overflow, return float
			return float64(li) + float64(ri)
		}
		return result
	}

	// Float addition
	var leftVal, rightVal float64
	if lis {
		leftVal = float64(li)
	} else {
		leftVal = lf
	}
	if ris {
		rightVal = float64(ri)
	} else {
		rightVal = rf
	}
	return leftVal + rightVal
}

// subtract performs subtraction.
func subtract(li int64, lis bool, ri int64, ris bool, lf, rf float64) interface{} {
	// If both are integers, try integer subtraction
	if lis && ris {
		// Check for overflow
		result := li - ri
		// Overflow check
		if (li > 0 && ri < 0 && result < 0) || (li < 0 && ri > 0 && result > 0) {
			// Overflow, return float
			return float64(li) - float64(ri)
		}
		return result
	}

	// Float subtraction
	var leftVal, rightVal float64
	if lis {
		leftVal = float64(li)
	} else {
		leftVal = lf
	}
	if ris {
		rightVal = float64(ri)
	} else {
		rightVal = rf
	}
	return leftVal - rightVal
}

// multiply performs multiplication.
func multiply(li int64, lis bool, ri int64, ris bool, lf, rf float64) interface{} {
	// If both are integers, try integer multiplication
	if lis && ris {
		// Check for overflow using float conversion
		result := li * ri
		check := float64(li) * float64(ri)
		if float64(result) != check || math.IsInf(check, 0) {
			// Overflow, return float
			return check
		}
		return result
	}

	// Float multiplication
	var leftVal, rightVal float64
	if lis {
		leftVal = float64(li)
	} else {
		leftVal = lf
	}
	if ris {
		rightVal = float64(ri)
	} else {
		rightVal = rf
	}
	return leftVal * rightVal
}

// divide performs division.
func divide(li int64, lis bool, ri int64, ris bool, lf, rf float64) interface{} {
	// Check for division by zero
	if (ris && ri == 0) || (!ris && rf == 0.0) {
		return nil
	}

	// If both are integers, try integer division
	if lis && ris {
		// SQLite uses integer division for integers
		if ri == -1 && li == math.MinInt64 {
			// Special case: prevent overflow
			return float64(li) / float64(ri)
		}
		return li / ri
	}

	// Float division
	var leftVal, rightVal float64
	if lis {
		leftVal = float64(li)
	} else {
		leftVal = lf
	}
	if ris {
		rightVal = float64(ri)
	} else {
		rightVal = rf
	}
	result := leftVal / rightVal

	// Check for overflow to infinity
	if math.IsInf(result, 0) {
		return nil
	}

	return result
}

// remainder performs modulo operation.
func remainder(li int64, lis bool, ri int64, ris bool, lf, rf float64) interface{} {
	// Check for division by zero
	if (ris && ri == 0) || (!ris && rf == 0.0) {
		return nil
	}

	// If both are integers, use integer modulo
	if lis && ris {
		return li % ri
	}

	// For floats, use math.Mod
	var leftVal, rightVal float64
	if lis {
		leftVal = float64(li)
	} else {
		leftVal = lf
	}
	if ris {
		rightVal = float64(ri)
	} else {
		rightVal = rf
	}
	return math.Mod(leftVal, rightVal)
}

// EvaluateUnary evaluates a unary arithmetic expression.
func EvaluateUnary(op OpCode, operand interface{}) interface{} {
	if operand == nil {
		return nil
	}

	switch op {
	case OpNegate:
		return negate(operand)
	case OpUnaryPlus:
		// Unary plus: convert to numeric but don't change sign
		return CoerceToNumeric(operand)
	case OpBitNot:
		return bitNot(operand)
	default:
		return nil
	}
}

// negate performs unary negation.
func negate(v interface{}) interface{} {
	switch val := v.(type) {
	case int64:
		// Check for overflow (negating MinInt64)
		if val == math.MinInt64 {
			return -float64(val)
		}
		return -val
	case float64:
		return -val
	case string:
		// Try to parse as number
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			if i == math.MinInt64 {
				return -float64(i)
			}
			return -i
		}
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return -f
		}
		// Non-numeric string becomes 0
		return int64(0)
	default:
		return int64(0)
	}
}

// bitNot performs bitwise NOT.
func bitNot(v interface{}) interface{} {
	i, ok := CoerceToInteger(v)
	if !ok {
		return int64(0)
	}
	return ^i
}

// EvaluateBitwise evaluates a bitwise operation.
func EvaluateBitwise(op OpCode, left, right interface{}) interface{} {
	if left == nil || right == nil {
		return nil
	}

	// Convert to integers
	leftInt, leftOk := CoerceToInteger(left)
	rightInt, rightOk := CoerceToInteger(right)

	if !leftOk || !rightOk {
		return nil
	}

	switch op {
	case OpBitAnd:
		return leftInt & rightInt

	case OpBitOr:
		return leftInt | rightInt

	case OpBitXor:
		return leftInt ^ rightInt

	case OpLShift:
		// Limit shift amount to prevent undefined behavior
		if rightInt < 0 || rightInt >= 64 {
			return int64(0)
		}
		return leftInt << uint(rightInt)

	case OpRShift:
		// Limit shift amount to prevent undefined behavior
		if rightInt < 0 || rightInt >= 64 {
			if leftInt < 0 {
				return int64(-1)
			}
			return int64(0)
		}
		return leftInt >> uint(rightInt)

	default:
		return nil
	}
}

// EvaluateConcat evaluates string concatenation.
func EvaluateConcat(left, right interface{}) interface{} {
	// NULL propagation
	if left == nil || right == nil {
		return nil
	}

	// Convert both operands to strings
	leftStr := valueToString(left)
	rightStr := valueToString(right)

	return leftStr + rightStr
}

// valueToString converts a value to string for concatenation.
func valueToString(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		// Use SQLite's float formatting
		return formatFloat(val)
	case []byte:
		return string(val)
	case bool:
		if val {
			return "1"
		}
		return "0"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// formatFloat formats a float like SQLite does.
func formatFloat(f float64) string {
	// SQLite uses a specific format for floats
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "Inf"
	}
	if math.IsInf(f, -1) {
		return "-Inf"
	}

	// Use 'g' format but ensure enough precision
	s := strconv.FormatFloat(f, 'g', -1, 64)

	// If there's no decimal point and no exponent, add .0
	if !strings.Contains(s, ".") && !strings.Contains(s, "e") && !strings.Contains(s, "E") {
		s += ".0"
	}

	return s
}

// EvaluateLogical evaluates a logical operation.
func EvaluateLogical(op OpCode, left, right interface{}) interface{} {
	switch op {
	case OpAnd:
		return evaluateAnd(left, right)
	case OpOr:
		return evaluateOr(left, right)
	case OpNot:
		return evaluateNot(left)
	default:
		return nil
	}
}

// evaluateAnd evaluates logical AND with NULL handling.
// SQLite three-valued logic:
//   true AND true = true
//   true AND false = false
//   true AND NULL = NULL
//   false AND anything = false
//   NULL AND false = false
//   NULL AND true = NULL
//   NULL AND NULL = NULL
func evaluateAnd(left, right interface{}) interface{} {
	leftBool := CoerceToBoolean(left)
	rightBool := CoerceToBoolean(right)

	// If either is false, result is false
	if !leftBool || !rightBool {
		return false
	}

	// If both are true and neither is NULL, result is true
	if left != nil && right != nil {
		return true
	}

	// At least one is NULL and the other is not false
	return nil
}

// evaluateOr evaluates logical OR with NULL handling.
// SQLite three-valued logic:
//   false OR false = false
//   false OR true = true
//   false OR NULL = NULL
//   true OR anything = true
//   NULL OR true = true
//   NULL OR false = NULL
//   NULL OR NULL = NULL
func evaluateOr(left, right interface{}) interface{} {
	leftBool := CoerceToBoolean(left)
	rightBool := CoerceToBoolean(right)

	// If either is true, result is true
	if leftBool || rightBool {
		return true
	}

	// If both are false and neither is NULL, result is false
	if left != nil && right != nil {
		return false
	}

	// At least one is NULL and the other is not true
	return nil
}

// evaluateNot evaluates logical NOT.
func evaluateNot(operand interface{}) interface{} {
	if operand == nil {
		return nil
	}

	return !CoerceToBoolean(operand)
}

// EvaluateCast performs type casting.
func EvaluateCast(value interface{}, targetType string) interface{} {
	if value == nil {
		return nil
	}

	aff := AffinityFromType(targetType)

	switch aff {
	case AFF_INTEGER:
		i, ok := CoerceToInteger(value)
		if ok {
			return i
		}
		return int64(0)

	case AFF_REAL:
		switch v := value.(type) {
		case float64:
			return v
		case int64:
			return float64(v)
		case string:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return 0.0
			}
			return f
		default:
			return 0.0
		}

	case AFF_TEXT:
		return valueToString(value)

	case AFF_NUMERIC:
		// Try integer first, then float
		if i, ok := CoerceToInteger(value); ok {
			return i
		}
		if s, ok := value.(string); ok {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f
			}
		}
		return value

	case AFF_BLOB:
		// Convert to blob (byte array)
		switch v := value.(type) {
		case []byte:
			return v
		case string:
			return []byte(v)
		default:
			s := valueToString(value)
			return []byte(s)
		}

	default:
		return value
	}
}
