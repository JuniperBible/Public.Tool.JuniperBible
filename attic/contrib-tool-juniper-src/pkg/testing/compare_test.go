package testing

import (
	"testing"
)

func TestCompare_ExactMatch(t *testing.T) {
	expected := "In the beginning God created the heaven and the earth."
	actual := "In the beginning God created the heaven and the earth."

	result := Compare(expected, actual, DefaultCompareOptions())

	if !result.Match {
		t.Errorf("Expected match for identical strings")
	}
	if result.Similarity != 1.0 {
		t.Errorf("Expected similarity 1.0, got %f", result.Similarity)
	}
}

func TestCompare_WhitespaceNormalization(t *testing.T) {
	expected := "In the   beginning"
	actual := "In the beginning"

	opts := DefaultCompareOptions()
	opts.IgnoreWhitespace = true

	result := Compare(expected, actual, opts)

	if !result.Match {
		t.Errorf("Expected match with whitespace normalization")
	}
}

func TestCompare_CaseInsensitive(t *testing.T) {
	expected := "GOD CREATED"
	actual := "god created"

	opts := DefaultCompareOptions()
	opts.IgnoreCase = true

	result := Compare(expected, actual, opts)

	if !result.Match {
		t.Errorf("Expected match with case insensitivity")
	}
}

func TestCompare_PunctuationIgnored(t *testing.T) {
	expected := "In the beginning, God created..."
	actual := "In the beginning God created"

	opts := DefaultCompareOptions()
	opts.IgnorePunctuation = true

	result := Compare(expected, actual, opts)

	if !result.Match {
		t.Errorf("Expected match with punctuation ignored")
	}
}

func TestCompare_SimilarityThreshold(t *testing.T) {
	expected := "In the beginning God created the heaven and the earth."
	actual := "In the beginning God created the heaven and the sky."

	// Low threshold should pass
	opts := DefaultCompareOptions()
	opts.SimilarityThreshold = 0.8

	result := Compare(expected, actual, opts)

	if !result.Match {
		t.Errorf("Expected match with 80%% threshold, got similarity %f", result.Similarity)
	}

	// High threshold should fail
	opts.SimilarityThreshold = 0.99
	result = Compare(expected, actual, opts)

	if result.Match {
		t.Errorf("Expected no match with 99%% threshold")
	}
}

func TestCompare_EmptyStrings(t *testing.T) {
	result := Compare("", "", DefaultCompareOptions())

	if !result.Match {
		t.Errorf("Expected match for empty strings")
	}
	if result.Similarity != 1.0 {
		t.Errorf("Expected similarity 1.0 for empty strings, got %f", result.Similarity)
	}
}

func TestCompare_DiffGeneration(t *testing.T) {
	expected := "Line 1\nLine 2\nLine 3"
	actual := "Line 1\nModified\nLine 3"

	opts := StrictCompareOptions()
	result := Compare(expected, actual, opts)

	if result.Match {
		t.Errorf("Expected no match for different strings")
	}
	if result.Diff == "" {
		t.Errorf("Expected diff to be generated")
	}
	if len(result.Details) == 0 {
		t.Errorf("Expected diff details to be populated")
	}
}

func TestCompare_DiffDetails(t *testing.T) {
	expected := "Line 1\nLine 2"
	actual := "Line 1\nChanged"

	opts := StrictCompareOptions()
	result := Compare(expected, actual, opts)

	if len(result.Details) != 1 {
		t.Errorf("Expected 1 diff detail, got %d", len(result.Details))
	}

	detail := result.Details[0]
	if detail.Line != 2 {
		t.Errorf("Expected diff on line 2, got %d", detail.Line)
	}
	if detail.Type != "change" {
		t.Errorf("Expected diff type 'change', got %s", detail.Type)
	}
	if detail.Expected != "Line 2" {
		t.Errorf("Expected 'Line 2', got %s", detail.Expected)
	}
	if detail.Actual != "Changed" {
		t.Errorf("Expected 'Changed', got %s", detail.Actual)
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
	}

	for _, tt := range tests {
		result := levenshteinDistance(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("levenshteinDistance(%q, %q) = %d, expected %d",
				tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"no change", "no change"},
		{"multiple   spaces", "multiple spaces"},
		{"tabs\t\tand spaces", "tabs and spaces"},
		{"\n\nnewlines\n\n", "newlines"},
		{"  leading and trailing  ", "leading and trailing"},
	}

	for _, tt := range tests {
		result := normalizeWhitespace(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeWhitespace(%q) = %q, expected %q",
				tt.input, result, tt.expected)
		}
	}
}

func TestRemovePunctuation(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"no punctuation", "no punctuation"},
		{"Hello, world!", "Hello world"},
		{"one. two; three:", "one two three"},
		{"quotes \"and\" 'marks'", "quotes and marks"},
	}

	for _, tt := range tests {
		result := removePunctuation(tt.input)
		if result != tt.expected {
			t.Errorf("removePunctuation(%q) = %q, expected %q",
				tt.input, result, tt.expected)
		}
	}
}

func TestCalculateSimilarity(t *testing.T) {
	tests := []struct {
		a      string
		b      string
		minSim float64
		maxSim float64
	}{
		{"", "", 1.0, 1.0},
		{"abc", "abc", 1.0, 1.0},
		{"abc", "", 0.0, 0.0},
		{"", "abc", 0.0, 0.0},
		{"hello world", "hello world", 1.0, 1.0},
		{"hello world", "hello there", 0.4, 0.6},    // 1 of 2 words match
		{"one two three", "one two four", 0.6, 0.7}, // 2 of 3 words match
	}

	for _, tt := range tests {
		result := calculateSimilarity(tt.a, tt.b)
		if result < tt.minSim || result > tt.maxSim {
			t.Errorf("calculateSimilarity(%q, %q) = %f, expected between %f and %f",
				tt.a, tt.b, result, tt.minSim, tt.maxSim)
		}
	}
}

func TestGenerateDiff(t *testing.T) {
	expected := "Line 1\nLine 2\nLine 3"
	actual := "Line 1\nModified\nLine 3"

	diff := generateDiff(expected, actual, 50)

	if diff == "" {
		t.Errorf("Expected non-empty diff")
	}
	if !contains(diff, "--- expected") {
		t.Errorf("Expected diff header with '--- expected'")
	}
	if !contains(diff, "+++ actual") {
		t.Errorf("Expected diff header with '+++ actual'")
	}
	if !contains(diff, "-Line 2") {
		t.Errorf("Expected removed line indicator")
	}
	if !contains(diff, "+Modified") {
		t.Errorf("Expected added line indicator")
	}
}

func TestGenerateDiff_Truncation(t *testing.T) {
	// Create content with many differences
	var expected, actual string
	for i := 0; i < 100; i++ {
		expected += "Original line\n"
		actual += "Modified line\n"
	}

	diff := generateDiff(expected, actual, 10)

	if !contains(diff, "truncated") {
		t.Errorf("Expected truncation notice for large diff")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCompare_Unicode(t *testing.T) {
	// Hebrew text
	expected := "בְּרֵאשִׁית בָּרָא אֱלֹהִים"
	actual := "בְּרֵאשִׁית בָּרָא אֱלֹהִים"

	result := Compare(expected, actual, DefaultCompareOptions())

	if !result.Match {
		t.Errorf("Expected match for identical Hebrew text")
	}

	// Greek text
	expected = "Ἐν ἀρχῇ ἦν ὁ λόγος"
	actual = "Ἐν ἀρχῇ ἦν ὁ λόγος"

	result = Compare(expected, actual, DefaultCompareOptions())

	if !result.Match {
		t.Errorf("Expected match for identical Greek text")
	}
}

func BenchmarkCompare(b *testing.B) {
	expected := "In the beginning God created the heaven and the earth. And the earth was without form, and void; and darkness was upon the face of the deep."
	actual := "In the beginning God created the heaven and the earth. And the earth was without form, and void; and darkness was upon the face of the waters."
	opts := DefaultCompareOptions()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Compare(expected, actual, opts)
	}
}

func BenchmarkLevenshteinDistance(b *testing.B) {
	a := "In the beginning God created the heaven and the earth."
	c := "In the beginning God created the heaven and the sky."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		levenshteinDistance(a, c)
	}
}
