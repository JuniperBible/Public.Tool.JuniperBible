// Package testing provides text comparison utilities for parser validation.
package testing

import (
	"fmt"
	"strings"
	"unicode"
)

// CompareOptions configures text comparison behavior.
type CompareOptions struct {
	// IgnoreWhitespace treats all whitespace as equivalent
	IgnoreWhitespace bool

	// IgnoreCase performs case-insensitive comparison
	IgnoreCase bool

	// IgnorePunctuation ignores punctuation differences
	IgnorePunctuation bool

	// NormalizeUnicode normalizes Unicode to NFC form
	NormalizeUnicode bool

	// SimilarityThreshold is the minimum similarity (0.0-1.0) to pass
	SimilarityThreshold float64

	// MaxDiffLines limits diff output length
	MaxDiffLines int
}

// DefaultCompareOptions returns sensible defaults.
func DefaultCompareOptions() CompareOptions {
	return CompareOptions{
		IgnoreWhitespace:    true,
		IgnoreCase:          false,
		IgnorePunctuation:   false,
		NormalizeUnicode:    true,
		SimilarityThreshold: 0.99, // 99% match required
		MaxDiffLines:        50,
	}
}

// StrictCompareOptions returns options for exact matching.
func StrictCompareOptions() CompareOptions {
	return CompareOptions{
		IgnoreWhitespace:    false,
		IgnoreCase:          false,
		IgnorePunctuation:   false,
		NormalizeUnicode:    false,
		SimilarityThreshold: 1.0, // 100% match required
		MaxDiffLines:        100,
	}
}

// CompareResult contains the result of a text comparison.
type CompareResult struct {
	Match      bool
	Similarity float64
	Diff       string
	Expected   string
	Actual     string
	Details    []DiffDetail
}

// DiffDetail describes a specific difference.
type DiffDetail struct {
	Line     int
	Type     string // "add", "remove", "change"
	Expected string
	Actual   string
}

// Compare compares two texts with the given options.
func Compare(expected, actual string, opts CompareOptions) CompareResult {
	result := CompareResult{
		Expected: expected,
		Actual:   actual,
		Details:  make([]DiffDetail, 0),
	}

	// Normalize texts based on options
	normExpected := normalize(expected, opts)
	normActual := normalize(actual, opts)

	// Check for exact match first
	if normExpected == normActual {
		result.Match = true
		result.Similarity = 1.0
		return result
	}

	// Calculate similarity
	result.Similarity = calculateSimilarity(normExpected, normActual)
	result.Match = result.Similarity >= opts.SimilarityThreshold

	// Generate diff if not matching
	if !result.Match || result.Similarity < 1.0 {
		result.Diff = generateDiff(expected, actual, opts.MaxDiffLines)
		result.Details = findDifferences(expected, actual)
	}

	return result
}

// normalize applies normalization options to text.
func normalize(text string, opts CompareOptions) string {
	if opts.IgnoreWhitespace {
		text = normalizeWhitespace(text)
	}

	if opts.IgnoreCase {
		text = strings.ToLower(text)
	}

	if opts.IgnorePunctuation {
		text = removePunctuation(text)
	}

	return text
}

// normalizeWhitespace collapses all whitespace to single spaces.
func normalizeWhitespace(text string) string {
	var result strings.Builder
	inWhitespace := false

	for _, r := range text {
		if unicode.IsSpace(r) {
			if !inWhitespace {
				result.WriteRune(' ')
				inWhitespace = true
			}
		} else {
			result.WriteRune(r)
			inWhitespace = false
		}
	}

	return strings.TrimSpace(result.String())
}

// removePunctuation removes punctuation characters.
func removePunctuation(text string) string {
	var result strings.Builder

	for _, r := range text {
		if !unicode.IsPunct(r) {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// calculateSimilarity computes Levenshtein-based similarity ratio.
func calculateSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}

	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	// Use word-based comparison for better results with text
	wordsA := strings.Fields(a)
	wordsB := strings.Fields(b)

	if len(wordsA) == 0 || len(wordsB) == 0 {
		// Fall back to character-based
		distance := levenshteinDistance(a, b)
		maxLen := max(len(a), len(b))
		return 1.0 - float64(distance)/float64(maxLen)
	}

	// Count matching words
	matches := 0
	usedB := make([]bool, len(wordsB))

	for _, wordA := range wordsA {
		for j, wordB := range wordsB {
			if !usedB[j] && wordA == wordB {
				matches++
				usedB[j] = true
				break
			}
		}
	}

	// Calculate similarity based on word matches
	totalWords := max(len(wordsA), len(wordsB))
	return float64(matches) / float64(totalWords)
}

// levenshteinDistance computes edit distance between strings.
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Use runes for proper Unicode handling
	runesA := []rune(a)
	runesB := []rune(b)

	// Create distance matrix
	m := len(runesA)
	n := len(runesB)
	d := make([][]int, m+1)
	for i := range d {
		d[i] = make([]int, n+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			cost := 1
			if runesA[i-1] == runesB[j-1] {
				cost = 0
			}
			d[i][j] = min(
				d[i-1][j]+1,      // deletion
				d[i][j-1]+1,      // insertion
				d[i-1][j-1]+cost, // substitution
			)
		}
	}

	return d[m][n]
}

// generateDiff creates a unified diff between two texts.
func generateDiff(expected, actual string, maxLines int) string {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var diff strings.Builder
	diff.WriteString("--- expected\n")
	diff.WriteString("+++ actual\n")

	lineCount := 0
	maxIdx := max(len(expectedLines), len(actualLines))

	for i := 0; i < maxIdx && lineCount < maxLines; i++ {
		var expLine, actLine string
		if i < len(expectedLines) {
			expLine = expectedLines[i]
		}
		if i < len(actualLines) {
			actLine = actualLines[i]
		}

		if expLine != actLine {
			if expLine != "" {
				diff.WriteString(fmt.Sprintf("-%s\n", expLine))
				lineCount++
			}
			if actLine != "" {
				diff.WriteString(fmt.Sprintf("+%s\n", actLine))
				lineCount++
			}
		}
	}

	if lineCount >= maxLines {
		diff.WriteString("... (truncated)\n")
	}

	return diff.String()
}

// findDifferences identifies specific differences between texts.
func findDifferences(expected, actual string) []DiffDetail {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var details []DiffDetail
	maxIdx := max(len(expectedLines), len(actualLines))

	for i := 0; i < maxIdx; i++ {
		var expLine, actLine string
		if i < len(expectedLines) {
			expLine = expectedLines[i]
		}
		if i < len(actualLines) {
			actLine = actualLines[i]
		}

		if expLine != actLine {
			var diffType string
			if expLine == "" {
				diffType = "add"
			} else if actLine == "" {
				diffType = "remove"
			} else {
				diffType = "change"
			}

			details = append(details, DiffDetail{
				Line:     i + 1,
				Type:     diffType,
				Expected: expLine,
				Actual:   actLine,
			})
		}
	}

	return details
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
