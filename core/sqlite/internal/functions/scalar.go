package functions

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// RegisterScalarFunctions registers all scalar functions.
func RegisterScalarFunctions(r *Registry) {
	// String functions
	r.Register(NewScalarFunc("length", 1, lengthFunc))
	r.Register(NewScalarFunc("substr", -1, substrFunc)) // 2 or 3 args
	r.Register(NewScalarFunc("upper", 1, upperFunc))
	r.Register(NewScalarFunc("lower", 1, lowerFunc))
	r.Register(NewScalarFunc("trim", -1, trimFunc))     // 1 or 2 args
	r.Register(NewScalarFunc("ltrim", -1, ltrimFunc))   // 1 or 2 args
	r.Register(NewScalarFunc("rtrim", -1, rtrimFunc))   // 1 or 2 args
	r.Register(NewScalarFunc("replace", 3, replaceFunc))
	r.Register(NewScalarFunc("instr", 2, instrFunc))
	r.Register(NewScalarFunc("hex", 1, hexFunc))
	r.Register(NewScalarFunc("unhex", -1, unhexFunc)) // 1 or 2 args
	r.Register(NewScalarFunc("quote", 1, quoteFunc))
	r.Register(NewScalarFunc("unicode", 1, unicodeFunc))
	r.Register(NewScalarFunc("char", -1, charFunc)) // variadic

	// Type functions
	r.Register(NewScalarFunc("typeof", 1, typeofFunc))
	r.Register(NewScalarFunc("coalesce", -1, coalesceFunc)) // variadic
	r.Register(NewScalarFunc("ifnull", 2, ifnullFunc))
	r.Register(NewScalarFunc("nullif", 2, nullifFunc))
	r.Register(NewScalarFunc("iif", 3, iifFunc))

	// Blob functions
	r.Register(NewScalarFunc("zeroblob", 1, zeroblobFunc))
}

// lengthFunc implements length(X)
// Returns the number of characters in X (UTF-8 aware for text)
func lengthFunc(args []Value) (Value, error) {
	if args[0].IsNull() {
		return NewNullValue(), nil
	}

	switch args[0].Type() {
	case TypeBlob, TypeInteger, TypeFloat:
		return NewIntValue(int64(args[0].Bytes())), nil
	case TypeText:
		s := args[0].AsString()
		return NewIntValue(int64(utf8.RuneCountInString(s))), nil
	default:
		return NewNullValue(), nil
	}
}

// substrFunc implements substr(X, Y [, Z])
// Returns a substring of X starting at Y with length Z (or to end if Z omitted)
// Y is 1-indexed; negative Y counts from end
func substrFunc(args []Value) (Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("substr() requires 2 or 3 arguments")
	}

	if args[0].IsNull() {
		return NewNullValue(), nil
	}

	isBlob := args[0].Type() == TypeBlob
	var data []byte
	var length int

	if isBlob {
		data = args[0].AsBlob()
		length = len(data)
	} else {
		s := args[0].AsString()
		data = []byte(s)
		length = utf8.RuneCountInString(s)
	}

	start := args[1].AsInt64()
	var subLen int64 = int64(length) // default to rest of string

	if len(args) == 3 {
		subLen = args[2].AsInt64()
		if subLen == 0 && args[2].IsNull() {
			return NewNullValue(), nil
		}
	}

	// Handle negative start (count from end)
	if start < 0 {
		start = int64(length) + start
		if start < 0 {
			if subLen < 0 {
				subLen = 0
			} else {
				subLen += start
			}
			start = 0
		}
	} else if start > 0 {
		start-- // Convert to 0-indexed
	} else if start == 0 {
		// SQLite compatibility: substr(X, 0, N) returns empty string in standard mode
		if args[1].IsNull() {
			return NewNullValue(), nil
		}
	}

	// Handle negative length (characters before start)
	if subLen < 0 {
		if subLen < -start {
			subLen = start
		} else {
			subLen = -subLen
		}
		start -= subLen
	}

	if start < 0 {
		start = 0
	}
	if subLen < 0 {
		subLen = 0
	}

	if isBlob {
		// Byte-based substring
		if start >= int64(length) {
			return NewBlobValue([]byte{}), nil
		}
		end := start + subLen
		if end > int64(length) {
			end = int64(length)
		}
		return NewBlobValue(data[start:end]), nil
	} else {
		// Character-based substring (UTF-8 aware)
		s := args[0].AsString()
		runes := []rune(s)
		if start >= int64(len(runes)) {
			return NewTextValue(""), nil
		}
		end := start + subLen
		if end > int64(len(runes)) {
			end = int64(len(runes))
		}
		return NewTextValue(string(runes[start:end])), nil
	}
}

// upperFunc implements upper(X)
func upperFunc(args []Value) (Value, error) {
	if args[0].IsNull() {
		return NewNullValue(), nil
	}
	return NewTextValue(strings.ToUpper(args[0].AsString())), nil
}

// lowerFunc implements lower(X)
func lowerFunc(args []Value) (Value, error) {
	if args[0].IsNull() {
		return NewNullValue(), nil
	}
	return NewTextValue(strings.ToLower(args[0].AsString())), nil
}

// trimFunc implements trim(X [, Y])
// Removes characters in Y from both ends of X
func trimFunc(args []Value) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("trim() requires 1 or 2 arguments")
	}

	if args[0].IsNull() {
		return NewNullValue(), nil
	}

	s := args[0].AsString()
	cutset := " " // default to space

	if len(args) == 2 && !args[1].IsNull() {
		cutset = args[1].AsString()
	}

	return NewTextValue(strings.Trim(s, cutset)), nil
}

// ltrimFunc implements ltrim(X [, Y])
func ltrimFunc(args []Value) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("ltrim() requires 1 or 2 arguments")
	}

	if args[0].IsNull() {
		return NewNullValue(), nil
	}

	s := args[0].AsString()
	cutset := " "

	if len(args) == 2 && !args[1].IsNull() {
		cutset = args[1].AsString()
	}

	return NewTextValue(strings.TrimLeft(s, cutset)), nil
}

// rtrimFunc implements rtrim(X [, Y])
func rtrimFunc(args []Value) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("rtrim() requires 1 or 2 arguments")
	}

	if args[0].IsNull() {
		return NewNullValue(), nil
	}

	s := args[0].AsString()
	cutset := " "

	if len(args) == 2 && !args[1].IsNull() {
		cutset = args[1].AsString()
	}

	return NewTextValue(strings.TrimRight(s, cutset)), nil
}

// replaceFunc implements replace(X, Y, Z)
// Replaces all occurrences of Y in X with Z
func replaceFunc(args []Value) (Value, error) {
	if args[0].IsNull() || args[1].IsNull() {
		return NewNullValue(), nil
	}

	x := args[0].AsString()
	y := args[1].AsString()
	z := ""
	if !args[2].IsNull() {
		z = args[2].AsString()
	}

	// Handle empty pattern
	if y == "" {
		return NewTextValue(x), nil
	}

	return NewTextValue(strings.ReplaceAll(x, y, z)), nil
}

// instrFunc implements instr(X, Y)
// Returns the 1-indexed position of the first occurrence of Y in X, or 0 if not found
func instrFunc(args []Value) (Value, error) {
	if args[0].IsNull() || args[1].IsNull() {
		return NewNullValue(), nil
	}

	haystack := args[0].AsString()
	needle := args[1].AsString()

	// Handle both as blobs
	if args[0].Type() == TypeBlob && args[1].Type() == TypeBlob {
		haystackBytes := args[0].AsBlob()
		needleBytes := args[1].AsBlob()
		idx := bytes.Index(haystackBytes, needleBytes)
		if idx < 0 {
			return NewIntValue(0), nil
		}
		return NewIntValue(int64(idx + 1)), nil
	}

	// Text-based search (UTF-8 aware)
	if needle == "" {
		return NewIntValue(1), nil
	}

	idx := strings.Index(haystack, needle)
	if idx < 0 {
		return NewIntValue(0), nil
	}

	// Convert byte index to character index
	charIdx := utf8.RuneCountInString(haystack[:idx])
	return NewIntValue(int64(charIdx + 1)), nil
}

// hexFunc implements hex(X)
// Returns hex representation of X
func hexFunc(args []Value) (Value, error) {
	if args[0].IsNull() {
		return NewNullValue(), nil
	}

	data := args[0].AsBlob()
	return NewTextValue(strings.ToUpper(hex.EncodeToString(data))), nil
}

// unhexFunc implements unhex(X [, Y])
// Decodes hex string X, optionally ignoring characters in Y
func unhexFunc(args []Value) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("unhex() requires 1 or 2 arguments")
	}

	if args[0].IsNull() {
		return NewNullValue(), nil
	}

	hexStr := args[0].AsString()
	ignore := ""

	if len(args) == 2 && !args[1].IsNull() {
		ignore = args[1].AsString()
	}

	// Remove ignored characters
	if ignore != "" {
		var filtered strings.Builder
		for _, r := range hexStr {
			if !strings.ContainsRune(ignore, r) {
				filtered.WriteRune(r)
			}
		}
		hexStr = filtered.String()
	}

	// Decode hex
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		return NewNullValue(), nil // Return NULL on error
	}

	return NewBlobValue(decoded), nil
}

// quoteFunc implements quote(X)
// Returns SQL literal representation of X
func quoteFunc(args []Value) (Value, error) {
	if args[0].IsNull() {
		return NewTextValue("NULL"), nil
	}

	switch args[0].Type() {
	case TypeInteger:
		return NewTextValue(fmt.Sprintf("%d", args[0].AsInt64())), nil
	case TypeFloat:
		f := args[0].AsFloat64()
		return NewTextValue(fmt.Sprintf("%g", f)), nil
	case TypeText:
		s := args[0].AsString()
		// Escape single quotes
		escaped := strings.ReplaceAll(s, "'", "''")
		return NewTextValue(fmt.Sprintf("'%s'", escaped)), nil
	case TypeBlob:
		data := args[0].AsBlob()
		hexStr := hex.EncodeToString(data)
		return NewTextValue(fmt.Sprintf("X'%s'", strings.ToUpper(hexStr))), nil
	default:
		return NewTextValue("NULL"), nil
	}
}

// unicodeFunc implements unicode(X)
// Returns the Unicode code point of the first character of X
func unicodeFunc(args []Value) (Value, error) {
	if args[0].IsNull() {
		return NewNullValue(), nil
	}

	s := args[0].AsString()
	if s == "" {
		return NewNullValue(), nil
	}

	r, _ := utf8.DecodeRuneInString(s)
	return NewIntValue(int64(r)), nil
}

// charFunc implements char(X1, X2, ..., XN)
// Returns a string composed of characters with Unicode code points
func charFunc(args []Value) (Value, error) {
	var result strings.Builder

	for _, arg := range args {
		if arg.IsNull() {
			continue
		}

		codePoint := arg.AsInt64()
		// Invalid code points become replacement character
		if codePoint < 0 || codePoint > 0x10FFFF {
			codePoint = 0xFFFD
		}

		result.WriteRune(rune(codePoint))
	}

	return NewTextValue(result.String()), nil
}

// typeofFunc implements typeof(X)
// Returns the type of X as a string
func typeofFunc(args []Value) (Value, error) {
	return NewTextValue(args[0].Type().String()), nil
}

// coalesceFunc implements coalesce(X, Y, ...)
// Returns the first non-NULL argument
func coalesceFunc(args []Value) (Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("coalesce() requires at least 1 argument")
	}

	for _, arg := range args {
		if !arg.IsNull() {
			return arg, nil
		}
	}

	return NewNullValue(), nil
}

// ifnullFunc implements ifnull(X, Y)
// Returns X if X is not NULL, otherwise Y
func ifnullFunc(args []Value) (Value, error) {
	if !args[0].IsNull() {
		return args[0], nil
	}
	return args[1], nil
}

// nullifFunc implements nullif(X, Y)
// Returns NULL if X == Y, otherwise X
func nullifFunc(args []Value) (Value, error) {
	// If both are NULL, they are equal
	if args[0].IsNull() && args[1].IsNull() {
		return NewNullValue(), nil
	}

	// If one is NULL, they are not equal
	if args[0].IsNull() || args[1].IsNull() {
		return args[0], nil
	}

	// Compare values
	if compareValues(args[0], args[1]) == 0 {
		return NewNullValue(), nil
	}

	return args[0], nil
}

// iifFunc implements iif(X, Y, Z)
// Returns Y if X is true, otherwise Z
func iifFunc(args []Value) (Value, error) {
	if args[0].IsNull() {
		return args[2], nil // NULL is false
	}

	// Determine truthiness
	isTrue := false
	switch args[0].Type() {
	case TypeInteger:
		isTrue = args[0].AsInt64() != 0
	case TypeFloat:
		isTrue = args[0].AsFloat64() != 0.0
	case TypeText:
		// Non-empty string is true if it can be parsed as non-zero number
		f := args[0].AsFloat64()
		isTrue = f != 0.0
	}

	if isTrue {
		return args[1], nil
	}
	return args[2], nil
}

// zeroblobFunc implements zeroblob(N)
// Returns a blob of N zero bytes
func zeroblobFunc(args []Value) (Value, error) {
	if args[0].IsNull() {
		return NewNullValue(), nil
	}

	n := args[0].AsInt64()
	if n < 0 {
		n = 0
	}

	blob := make([]byte, n)
	return NewBlobValue(blob), nil
}

// compareValues compares two values
// Returns -1 if a < b, 0 if a == b, 1 if a > b
func compareValues(a, b Value) int {
	// NULL handling
	if a.IsNull() && b.IsNull() {
		return 0
	}
	if a.IsNull() {
		return -1
	}
	if b.IsNull() {
		return 1
	}

	// Type affinity ordering: NULL < INTEGER < REAL < TEXT < BLOB
	if a.Type() != b.Type() {
		return int(a.Type()) - int(b.Type())
	}

	switch a.Type() {
	case TypeInteger:
		aVal, bVal := a.AsInt64(), b.AsInt64()
		if aVal < bVal {
			return -1
		} else if aVal > bVal {
			return 1
		}
		return 0

	case TypeFloat:
		aVal, bVal := a.AsFloat64(), b.AsFloat64()
		if aVal < bVal {
			return -1
		} else if aVal > bVal {
			return 1
		}
		return 0

	case TypeText:
		return strings.Compare(a.AsString(), b.AsString())

	case TypeBlob:
		return bytes.Compare(a.AsBlob(), b.AsBlob())

	default:
		return 0
	}
}

// isDigit checks if a rune is a digit
func isDigit(r rune) bool {
	return unicode.IsDigit(r)
}

// isSpace checks if a rune is whitespace
func isSpace(r rune) bool {
	return unicode.IsSpace(r)
}

// abs returns the absolute value of an integer
func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// fabs returns the absolute value of a float
func fabs(f float64) float64 {
	return math.Abs(f)
}
