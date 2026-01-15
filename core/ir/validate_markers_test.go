// Package ir provides tests for structural marker and empty text validation.
package ir

import (
	"testing"
)

func TestAnalyzeEmptyText(t *testing.T) {
	tests := []struct {
		name           string
		rawMarkup      string
		wantReason     string
		wantPurposeful bool
	}{
		{
			name:           "empty markup - data loss",
			rawMarkup:      "",
			wantReason:     "no raw markup present - possible data loss",
			wantPurposeful: false,
		},
		{
			name:           "chapter start marker only",
			rawMarkup:      `<chapter osisID="1Pet.5" sID="gen36546"/> `,
			wantReason:     "chapter boundary marker (versification difference)",
			wantPurposeful: true,
		},
		{
			name:           "chapter end marker only",
			rawMarkup:      `<chapter eID="gen36526" osisID="1Pet.4"/>`,
			wantReason:     "chapter boundary marker (versification difference)",
			wantPurposeful: true,
		},
		{
			name:           "book marker only",
			rawMarkup:      `<div type="book" osisID="Gen"/>`,
			wantReason:     "book boundary marker",
			wantPurposeful: true,
		},
		{
			name:           "section marker only",
			rawMarkup:      `<div type="section" osisID="Ps.1"/>`,
			wantReason:     "section boundary marker",
			wantPurposeful: true,
		},
		{
			name:           "milestone marker only",
			rawMarkup:      `<milestone type="x-extra-space"/>`,
			wantReason:     "milestone marker only",
			wantPurposeful: true,
		},
		{
			name:           "whitespace only",
			rawMarkup:      "   \n\t  ",
			wantReason:     "markup-only content (no actual text)",
			wantPurposeful: true,
		},
		{
			name:           "actual text present",
			rawMarkup:      "In the beginning God created",
			wantReason:     "raw markup contains text but stripped result is empty - possible parsing error",
			wantPurposeful: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, isPurposeful := analyzeEmptyText(tt.rawMarkup)
			if reason != tt.wantReason {
				t.Errorf("analyzeEmptyText() reason = %q, want %q", reason, tt.wantReason)
			}
			if isPurposeful != tt.wantPurposeful {
				t.Errorf("analyzeEmptyText() isPurposeful = %v, want %v", isPurposeful, tt.wantPurposeful)
			}
		})
	}
}

func TestContainsOSISMarker(t *testing.T) {
	tests := []struct {
		name    string
		markup  string
		element string
		want    bool
	}{
		{"has chapter start", `<chapter osisID="Gen.1"/>`, "chapter", true},
		{"has chapter end", `</chapter>`, "chapter", true},
		{"no chapter", `<verse osisID="Gen.1.1"/>`, "chapter", false},
		{"has div", `<div type="book"/>`, "div", true},
		{"has milestone", `<milestone type="x"/>`, "milestone", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsOSISMarker(tt.markup, tt.element); got != tt.want {
				t.Errorf("containsOSISMarker() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsActualText(t *testing.T) {
	tests := []struct {
		name   string
		markup string
		want   bool
	}{
		{"text only", "Hello world", true},
		{"text with tags", "<b>Hello</b> world", true},
		{"tags only", "<chapter osisID='Gen.1'/>", false},
		{"whitespace only", "   \n\t  ", false},
		{"empty", "", false},
		{"tags with whitespace", "<div>   </div>", false},
		{"nested tags no text", "<a><b><c/></b></a>", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsActualText(tt.markup); got != tt.want {
				t.Errorf("containsActualText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateEmptyTextFields(t *testing.T) {
	corpus := &Corpus{
		ID:      "test",
		Version: "1.0.0",
		Documents: []*Document{
			{
				ID: "Gen",
				ContentBlocks: []*ContentBlock{
					{ID: "Gen.1.1", Text: "In the beginning", Attributes: map[string]interface{}{"raw_markup": "In the beginning"}},
					{ID: "Gen.1.2", Text: "", Attributes: map[string]interface{}{"raw_markup": "<chapter sID='ch1'/>"}},           // Purposeful
					{ID: "Gen.1.3", Text: "", Attributes: nil},                                                                      // Data loss
					{ID: "Gen.1.4", Text: "", Attributes: map[string]interface{}{"raw_markup": "Has text but stripped is empty"}}, // Error
				},
			},
		},
	}

	results := ValidateEmptyTextFields(corpus)

	if len(results) != 3 {
		t.Errorf("ValidateEmptyTextFields() returned %d results, want 3", len(results))
	}

	// Check that purposeful empty is correctly identified
	foundPurposeful := false
	foundDataLoss := false
	for _, r := range results {
		if r.ContentBlockID == "Gen.1.2" && r.IsPurposeful {
			foundPurposeful = true
		}
		if r.ContentBlockID == "Gen.1.3" && !r.IsPurposeful {
			foundDataLoss = true
		}
	}

	if !foundPurposeful {
		t.Error("Expected Gen.1.2 to be marked as purposeful empty")
	}
	if !foundDataLoss {
		t.Error("Expected Gen.1.3 to be marked as data loss")
	}
}

func TestValidateNoUnexpectedEmptyText(t *testing.T) {
	corpus := &Corpus{
		ID:      "test",
		Version: "1.0.0",
		Documents: []*Document{
			{
				ID: "Gen",
				ContentBlocks: []*ContentBlock{
					{ID: "Gen.1.1", Text: "In the beginning", Attributes: map[string]interface{}{"raw_markup": "In the beginning"}},
					{ID: "Gen.1.2", Text: "", Attributes: map[string]interface{}{"raw_markup": "<chapter sID='ch1'/>"}}, // OK
					{ID: "Gen.1.3", Text: "", Attributes: nil}, // Error
				},
			},
		},
	}

	errs := ValidateNoUnexpectedEmptyText(corpus)

	if len(errs) != 1 {
		t.Errorf("ValidateNoUnexpectedEmptyText() returned %d errors, want 1", len(errs))
	}
}

// TestMarkupPatterns tests various OSIS/ThML markup patterns that should be recognized
func TestMarkupPatterns(t *testing.T) {
	// Common patterns from different Bible formats
	patterns := []struct {
		name           string
		markup         string
		expectText     bool
		expectChapter  bool
		expectMilestone bool
	}{
		{
			name:          "SWORD chapter start",
			markup:        `<chapter osisID="1Pet.5" sID="gen36546"/>`,
			expectChapter: true,
		},
		{
			name:          "SWORD chapter end",
			markup:        `<chapter eID="gen36526" osisID="1Pet.4"/>`,
			expectChapter: true,
		},
		{
			name:           "OSIS milestone",
			markup:         `<milestone type="x-p"/>`,
			expectMilestone: true,
		},
		{
			name:       "Regular verse text",
			markup:     "And God said, Let there be light: and there was light.",
			expectText: true,
		},
		{
			name:       "Verse with inline markup",
			markup:     `<w lemma="H430">God</w> created the heavens`,
			expectText: true,
		},
		{
			name:          "Combined chapter marker and text",
			markup:        `<chapter eID="gen1"/> In the beginning <chapter sID="gen2"/>`,
			expectChapter: true,
			expectText:    true,
		},
	}

	for _, p := range patterns {
		t.Run(p.name, func(t *testing.T) {
			hasText := containsActualText(p.markup)
			hasChapter := containsOSISMarker(p.markup, "chapter")
			hasMilestone := containsOSISMarker(p.markup, "milestone")

			if hasText != p.expectText {
				t.Errorf("containsActualText() = %v, want %v", hasText, p.expectText)
			}
			if hasChapter != p.expectChapter {
				t.Errorf("containsOSISMarker(chapter) = %v, want %v", hasChapter, p.expectChapter)
			}
			if hasMilestone != p.expectMilestone {
				t.Errorf("containsOSISMarker(milestone) = %v, want %v", hasMilestone, p.expectMilestone)
			}
		})
	}
}

// BenchmarkContainsActualText benchmarks the text detection function
func BenchmarkContainsActualText(b *testing.B) {
	markup := `<chapter osisID="Gen.1" sID="gen1"/><verse osisID="Gen.1.1">In the beginning God created</verse>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		containsActualText(markup)
	}
}

// BenchmarkAnalyzeEmptyText benchmarks the empty text analysis
func BenchmarkAnalyzeEmptyText(b *testing.B) {
	markup := `<chapter osisID="1Pet.5" sID="gen36546"/> `

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzeEmptyText(markup)
	}
}
