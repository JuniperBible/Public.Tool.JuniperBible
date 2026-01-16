package schema

import (
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/expr"
)

// Affinity represents SQLite type affinity.
// We re-export the type from expr package for convenience.
type Affinity = expr.Affinity

// Affinity constants for schema use.
const (
	AffinityNone    = expr.AFF_NONE
	AffinityText    = expr.AFF_TEXT
	AffinityNumeric = expr.AFF_NUMERIC
	AffinityInteger = expr.AFF_INTEGER
	AffinityReal    = expr.AFF_REAL
	AffinityBlob    = expr.AFF_BLOB
)

// DetermineAffinity determines the type affinity from a column type name.
//
// SQLite type affinity rules (from https://sqlite.org/datatype3.html):
// 1. If the type contains "INT" -> INTEGER affinity
// 2. If the type contains "CHAR", "CLOB", or "TEXT" -> TEXT affinity
// 3. If the type contains "BLOB" or no type specified -> BLOB affinity
// 4. If the type contains "REAL", "FLOA", or "DOUB" -> REAL affinity
// 5. Otherwise -> NUMERIC affinity
//
// Examples:
//   - "INTEGER", "BIGINT", "INT2" -> INTEGER
//   - "VARCHAR(100)", "CHARACTER(20)", "TEXT" -> TEXT
//   - "BLOB", "" -> BLOB
//   - "REAL", "DOUBLE", "FLOAT" -> REAL
//   - "NUMERIC", "DECIMAL(10,2)", "BOOLEAN", "DATE" -> NUMERIC
func DetermineAffinity(typeName string) Affinity {
	// Empty type name gets BLOB affinity (per SQLite rules)
	if typeName == "" {
		return AffinityBlob
	}

	// Convert to uppercase for case-insensitive matching
	upper := strings.ToUpper(typeName)

	// Rule 1: INTEGER affinity
	if strings.Contains(upper, "INT") {
		return AffinityInteger
	}

	// Rule 2: TEXT affinity
	if strings.Contains(upper, "CHAR") ||
		strings.Contains(upper, "CLOB") ||
		strings.Contains(upper, "TEXT") {
		return AffinityText
	}

	// Rule 3: BLOB affinity
	if strings.Contains(upper, "BLOB") {
		return AffinityBlob
	}

	// Rule 4: REAL affinity
	if strings.Contains(upper, "REAL") ||
		strings.Contains(upper, "FLOA") ||
		strings.Contains(upper, "DOUB") {
		return AffinityReal
	}

	// Rule 5: Default to NUMERIC affinity
	return AffinityNumeric
}

// IsNumericAffinity returns true if the affinity is numeric (NUMERIC, INTEGER, or REAL).
func IsNumericAffinity(aff Affinity) bool {
	return aff == AffinityNumeric || aff == AffinityInteger || aff == AffinityReal
}

// AffinityName returns the canonical name for an affinity.
func AffinityName(aff Affinity) string {
	switch aff {
	case AffinityNone:
		return "NONE"
	case AffinityText:
		return "TEXT"
	case AffinityNumeric:
		return "NUMERIC"
	case AffinityInteger:
		return "INTEGER"
	case AffinityReal:
		return "REAL"
	case AffinityBlob:
		return "BLOB"
	default:
		return "UNKNOWN"
	}
}
