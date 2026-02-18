// Package testutil provides test helpers for IPC protocol testing.
// These utilities ensure deterministic golden tests across platforms.
package testutil

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Normalization patterns for deterministic test output
var (
	// ISO 8601 timestamp pattern
	timestampPattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})`)

	// UUID pattern (v4)
	uuidPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}`)

	// Temp path patterns (platform-specific)
	tempPathPatterns = []*regexp.Regexp{
		regexp.MustCompile(`/tmp/[^"]+`),
		regexp.MustCompile(`/var/folders/[^"]+`),
		regexp.MustCompile(`C:\\Users\\[^"]+\\AppData\\Local\\Temp\\[^"]+`),
		regexp.MustCompile(`C:\\Temp\\[^"]+`),
	}
)

// NormalizeJSON normalizes JSON for deterministic comparison.
// It performs the following transformations:
// - Sorts object keys alphabetically
// - Replaces timestamps with "<TIMESTAMP>"
// - Replaces UUIDs with "<UUID>"
// - Replaces temp paths with "<TEMP_PATH>"
// - Normalizes OS path separators to "/"
func NormalizeJSON(data []byte) ([]byte, error) {
	// Parse JSON into generic structure
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}

	// Recursively normalize
	normalized := normalizeValue(v)

	// Re-encode with sorted keys (Go's json.Marshal sorts by default)
	return json.MarshalIndent(normalized, "", "  ")
}

// NormalizeJSONString normalizes a JSON string for deterministic comparison.
func NormalizeJSONString(s string) (string, error) {
	normalized, err := NormalizeJSON([]byte(s))
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

// normalizeValue recursively normalizes a JSON value.
func normalizeValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return normalizeObject(val)
	case []interface{}:
		return normalizeArray(val)
	case string:
		return normalizeString(val)
	default:
		return v
	}
}

// normalizeObject normalizes a JSON object.
func normalizeObject(obj map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(obj))

	// Get sorted keys
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Process each key in sorted order
	for _, k := range keys {
		result[k] = normalizeValue(obj[k])
	}

	return result
}

// normalizeArray normalizes a JSON array.
func normalizeArray(arr []interface{}) []interface{} {
	result := make([]interface{}, len(arr))
	for i, v := range arr {
		result[i] = normalizeValue(v)
	}
	return result
}

// normalizeString normalizes a string value.
func normalizeString(s string) string {
	// Replace timestamps
	s = timestampPattern.ReplaceAllString(s, "<TIMESTAMP>")

	// Replace UUIDs
	s = uuidPattern.ReplaceAllString(s, "<UUID>")

	// Replace temp paths
	for _, pattern := range tempPathPatterns {
		s = pattern.ReplaceAllString(s, "<TEMP_PATH>")
	}

	// Normalize path separators (Windows -> Unix)
	s = normalizePathSeparators(s)

	return s
}

// normalizePathSeparators converts Windows path separators to Unix.
func normalizePathSeparators(s string) string {
	// Only normalize if this looks like a path (contains backslash + forward context)
	if strings.Contains(s, "\\") {
		// Be careful not to break escape sequences in JSON
		// Only replace backslash when followed by alphanumeric or path chars
		result := strings.Builder{}
		runes := []rune(s)
		for i := 0; i < len(runes); i++ {
			if runes[i] == '\\' && i+1 < len(runes) {
				next := runes[i+1]
				// If next char is alphanumeric or another path char, it's likely a path
				if isPathChar(next) {
					result.WriteRune('/')
					continue
				}
			}
			result.WriteRune(runes[i])
		}
		return result.String()
	}
	return s
}

// pathCharSet contains characters typically found in file paths.
var pathCharSet = func() map[rune]bool {
	m := make(map[rune]bool)
	for r := 'a'; r <= 'z'; r++ {
		m[r] = true
	}
	for r := 'A'; r <= 'Z'; r++ {
		m[r] = true
	}
	for r := '0'; r <= '9'; r++ {
		m[r] = true
	}
	for _, r := range []rune{'_', '-', '.', '/'} {
		m[r] = true
	}
	return m
}()

// isPathChar returns true if the rune is typically found in file paths.
func isPathChar(r rune) bool {
	return pathCharSet[r]
}

// NormalizePath normalizes a file path for cross-platform comparison.
func NormalizePath(path string) string {
	// Convert to forward slashes
	path = filepath.ToSlash(path)

	// Replace temp directory prefix
	for _, pattern := range tempPathPatterns {
		path = pattern.ReplaceAllString(path, "<TEMP_PATH>")
	}

	return path
}

// NormalizeFilePerms normalizes file permission bits to octal string.
// This ensures consistency across platforms (Windows vs Unix).
func NormalizeFilePerms(mode uint32) string {
	// Mask to Unix permission bits only
	return strings.TrimPrefix(strings.TrimPrefix(
		strings.ToLower(strings.Replace(
			string(rune('0'+((mode>>6)&7)))+
				string(rune('0'+((mode>>3)&7)))+
				string(rune('0'+(mode&7))),
			"", "", 0)), "0"), "0")
}

// CompareNormalizedJSON compares two JSON values after normalization.
// Returns true if they are equivalent.
func CompareNormalizedJSON(a, b []byte) (bool, error) {
	normalizedA, err := NormalizeJSON(a)
	if err != nil {
		return false, err
	}

	normalizedB, err := NormalizeJSON(b)
	if err != nil {
		return false, err
	}

	return string(normalizedA) == string(normalizedB), nil
}
