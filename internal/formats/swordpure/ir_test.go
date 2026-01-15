// Package swordpure provides tests for IR extraction and structural marker handling.
package swordpure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseChapterMarkers(t *testing.T) {
	tests := []struct {
		name       string
		rawMarkup  string
		wantCount  int
		wantOsisID string
		wantStart  bool
		wantEnd    bool
	}{
		{
			name:       "chapter start marker",
			rawMarkup:  `<chapter osisID="1Pet.5" sID="gen36546"/> `,
			wantCount:  1,
			wantOsisID: "1Pet.5",
			wantStart:  true,
			wantEnd:    false,
		},
		{
			name:       "chapter end marker",
			rawMarkup:  `<chapter eID="gen36526" osisID="1Pet.4"/>`,
			wantCount:  1,
			wantOsisID: "1Pet.4",
			wantStart:  false,
			wantEnd:    true,
		},
		{
			name:       "both start and end markers",
			rawMarkup:  `<chapter eID="gen36526" osisID="1Pet.4"/><chapter osisID="1Pet.5" sID="gen36546"/>`,
			wantCount:  2,
			wantOsisID: "1Pet.4", // First one
			wantStart:  false,
			wantEnd:    true,
		},
		{
			name:      "no chapter markers",
			rawMarkup: "In the beginning God created the heavens",
			wantCount: 0,
		},
		{
			name:      "verse markup without chapter",
			rawMarkup: `<verse osisID="Gen.1.1">In the beginning</verse>`,
			wantCount: 0,
		},
		{
			name:       "chapter marker with text",
			rawMarkup:  `Verse text here <chapter eID="gen1" osisID="Gen.1"/>`,
			wantCount:  1,
			wantOsisID: "Gen.1",
			wantEnd:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markers := ParseChapterMarkers(tt.rawMarkup)

			if len(markers) != tt.wantCount {
				t.Errorf("ParseChapterMarkers() got %d markers, want %d", len(markers), tt.wantCount)
				return
			}

			if tt.wantCount > 0 {
				m := markers[0]
				if m.OsisID != tt.wantOsisID {
					t.Errorf("OsisID = %q, want %q", m.OsisID, tt.wantOsisID)
				}
				if m.IsStart != tt.wantStart {
					t.Errorf("IsStart = %v, want %v", m.IsStart, tt.wantStart)
				}
				if m.IsEnd != tt.wantEnd {
					t.Errorf("IsEnd = %v, want %v", m.IsEnd, tt.wantEnd)
				}
			}
		})
	}
}

func TestIsChapterBoundaryOnly(t *testing.T) {
	tests := []struct {
		name      string
		rawMarkup string
		want      bool
	}{
		{
			name:      "chapter marker only",
			rawMarkup: `<chapter osisID="1Pet.5" sID="gen36546"/> `,
			want:      true,
		},
		{
			name:      "chapter marker with text",
			rawMarkup: `Some text <chapter osisID="1Pet.5" sID="gen36546"/>`,
			want:      false,
		},
		{
			name:      "no chapter marker",
			rawMarkup: "Just regular verse text",
			want:      false,
		},
		{
			name:      "empty markup",
			rawMarkup: "",
			want:      false,
		},
		{
			name:      "whitespace with chapter marker",
			rawMarkup: `  <chapter osisID="Gen.2" sID="x"/>  `,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsChapterBoundaryOnly(tt.rawMarkup); got != tt.want {
				t.Errorf("IsChapterBoundaryOnly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectVersificationDifference(t *testing.T) {
	tests := []struct {
		name      string
		rawMarkup string
		wantDesc  string
	}{
		{
			name:      "chapter start versification",
			rawMarkup: `<chapter osisID="1Pet.5" sID="gen36546"/> `,
			wantDesc:  "versification: chapter 1Pet.5 begins here in source",
		},
		{
			name:      "chapter end versification",
			rawMarkup: `<chapter eID="gen36526" osisID="1Pet.4"/>`,
			wantDesc:  "versification: chapter 1Pet.4 ends here in source",
		},
		{
			name:      "no versification difference",
			rawMarkup: "Regular verse text",
			wantDesc:  "",
		},
		{
			name:      "text with chapter marker",
			rawMarkup: `Verse text <chapter osisID="Gen.1" sID="x"/>`,
			wantDesc:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectVersificationDifference(tt.rawMarkup)
			if got != tt.wantDesc {
				t.Errorf("DetectVersificationDifference() = %q, want %q", got, tt.wantDesc)
			}
		})
	}
}

func TestStripMarkup(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
	}{
		{"plain text", "Hello world", "Hello world"},
		{"text with tags", "<b>Hello</b> world", "Hello world"},
		{"nested tags", "<a><b>Hello</b></a> world", "Hello world"},
		{"self-closing tag", "<br/>Hello", "Hello"},
		{"chapter marker only", `<chapter osisID="Gen.1"/>`, ""},
		{"whitespace preserved", "  Hello  world  ", "Hello  world"},
		{"empty", "", ""},
		{"tags only", "<a><b><c/></b></a>", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripMarkup(tt.input); got != tt.want {
				t.Errorf("stripMarkup() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestVersificationEdgeCases tests edge cases found in real Bible data
func TestVersificationEdgeCases(t *testing.T) {
	// Real examples from DRC Bible
	tests := []struct {
		name           string
		rawMarkup      string
		expectEmpty    bool
		expectChapter  bool
		description    string
	}{
		{
			name:          "DRC 1Pet.4.15 - chapter boundary",
			rawMarkup:     `<chapter osisID="1Pet.5" sID="gen36546"/> `,
			expectEmpty:   true,
			expectChapter: true,
			description:   "DRC chapter 4 ends at verse 14, chapter 5 starts",
		},
		{
			name:          "Normal verse with chapter end marker",
			rawMarkup:     `Wherefore let them also that suffer... <chapter eID="gen36526" osisID="1Pet.4"/>`,
			expectEmpty:   false,
			expectChapter: true,
			description:   "Last verse of chapter with end marker",
		},
		{
			name:          "Normal verse text",
			rawMarkup:     "The ancients therefore that are among you, I beseech...",
			expectEmpty:   false,
			expectChapter: false,
			description:   "Regular verse with no markers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plainText := stripMarkup(tt.rawMarkup)
			isEmpty := plainText == ""
			hasChapter := len(ParseChapterMarkers(tt.rawMarkup)) > 0

			if isEmpty != tt.expectEmpty {
				t.Errorf("Expected empty=%v, got %v (%s)", tt.expectEmpty, isEmpty, tt.description)
			}
			if hasChapter != tt.expectChapter {
				t.Errorf("Expected chapter=%v, got %v (%s)", tt.expectChapter, hasChapter, tt.description)
			}
		})
	}
}

// BenchmarkParseChapterMarkers benchmarks chapter marker parsing
func BenchmarkParseChapterMarkers(b *testing.B) {
	markup := `<chapter eID="gen36526" osisID="1Pet.4"/><chapter osisID="1Pet.5" sID="gen36546"/>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseChapterMarkers(markup)
	}
}

// BenchmarkIsChapterBoundaryOnly benchmarks boundary detection
func BenchmarkIsChapterBoundaryOnly(b *testing.B) {
	markup := `<chapter osisID="1Pet.5" sID="gen36546"/> `

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsChapterBoundaryOnly(markup)
	}
}

func TestComputeHash(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"simple text", "Hello world"},
		{"unicode text", "שָׁלוֹם"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := computeHash(tt.input)
			if hash == "" {
				t.Error("computeHash returned empty string")
			}
			if len(hash) != 64 { // SHA-256 is 64 hex characters
				t.Errorf("computeHash returned %d characters, want 64", len(hash))
			}
			// Hash should be deterministic
			hash2 := computeHash(tt.input)
			if hash != hash2 {
				t.Error("computeHash is not deterministic")
			}
		})
	}
}

func TestExtractAttr(t *testing.T) {
	tests := []struct {
		name  string
		tag   string
		attr  string
		want  string
	}{
		{"simple attribute", `<w lemma="strong:G2316">`, "lemma", "strong:G2316"},
		{"no attribute", `<w>test</w>`, "lemma", ""},
		{"multiple attributes", `<w lemma="G123" morph="N-NSM">`, "morph", "N-NSM"},
		{"empty tag", "", "lemma", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAttr(tt.tag, tt.attr)
			if got != tt.want {
				t.Errorf("extractAttr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseVerseContent(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		rawText     string
		wantText    string
		wantMarkup  bool
		wantHash    bool
	}{
		{
			name:       "simple text",
			id:         "Gen.1.1",
			rawText:    "In the beginning God created the heaven and the earth.",
			wantText:   "In the beginning God created the heaven and the earth.",
			wantMarkup: true,
			wantHash:   true,
		},
		{
			name:       "text with markup",
			rawText:    "<b>In the beginning</b> God created",
			wantText:   "In the beginning God created",
			wantMarkup: true,
			wantHash:   true,
		},
		{
			name:       "empty text",
			id:         "Test.1.1",
			rawText:    "",
			wantText:   "",
			wantMarkup: true,
			wantHash:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := parseVerseContent(tt.id, tt.rawText, 0)
			if block == nil {
				t.Fatal("parseVerseContent returned nil")
			}
			if block.ID != tt.id {
				t.Errorf("ID = %q, want %q", block.ID, tt.id)
			}
			if block.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", block.Text, tt.wantText)
			}
			if tt.wantMarkup && block.RawMarkup != tt.rawText {
				t.Errorf("RawMarkup = %q, want %q", block.RawMarkup, tt.rawText)
			}
			if tt.wantHash && block.Hash == "" {
				t.Error("Hash should not be empty")
			}
		})
	}
}

func TestExtractCorpus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZTextModule(t, tmpDir)
	mod, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		t.Fatalf("OpenZTextModule failed: %v", err)
	}

	corpus, stats, err := extractCorpus(mod, conf)
	if err != nil {
		t.Fatalf("extractCorpus failed: %v", err)
	}

	if corpus == nil {
		t.Fatal("extractCorpus returned nil corpus")
	}

	if corpus.ID != "TestMod" {
		t.Errorf("corpus.ID = %q, want %q", corpus.ID, "TestMod")
	}

	if corpus.ModuleType != "BIBLE" {
		t.Errorf("corpus.ModuleType = %q, want %q", corpus.ModuleType, "BIBLE")
	}

	if stats == nil {
		t.Fatal("extractCorpus returned nil stats")
	}

	// Should have extracted at least one document
	if stats.Documents == 0 {
		t.Error("extractCorpus should extract at least one document")
	}
}

func TestGenerateConfFromIR(t *testing.T) {
	corpus := &IRCorpus{
		ID:             "TESTBIBLE",
		Version:        "1.0.0",
		ModuleType:     "BIBLE",
		Versification:  "KJV",
		Language:       "en",
		Title:          "Test Bible",
		Attributes: map[string]string{
			"about":       "Test about",
			"copyright":   "Test copyright",
			"license":     "Public Domain",
			"source_type": "OSIS",
		},
	}

	confContent := generateConfFromIR(corpus)

	if confContent == "" {
		t.Error("generateConfFromIR returned empty string")
	}

	// Check for required fields
	requiredFields := []string{
		"[TESTBIBLE]",
		"Description=Test Bible",
		"Lang=en",
		"Versification=KJV",
		"About=Test about",
	}

	for _, field := range requiredFields {
		if !contains(confContent, field) {
			t.Errorf("generateConfFromIR missing field: %s", field)
		}
	}
}

func TestWriteCorpusJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := &IRCorpus{
		ID:            "TEST",
		Version:       "1.0.0",
		ModuleType:    "BIBLE",
		Versification: "KJV",
		Language:      "en",
		Title:         "Test",
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	if err := writeCorpusJSON(corpus, irPath); err != nil {
		t.Fatalf("writeCorpusJSON failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(irPath); os.IsNotExist(err) {
		t.Error("writeCorpusJSON did not create file")
	}

	// Verify file contents
	data, err := os.ReadFile(irPath)
	if err != nil {
		t.Fatalf("failed to read IR file: %v", err)
	}

	var loaded IRCorpus
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to parse IR JSON: %v", err)
	}

	if loaded.ID != corpus.ID {
		t.Errorf("loaded.ID = %q, want %q", loaded.ID, corpus.ID)
	}
}

func TestParseStrongs(t *testing.T) {
	tests := []struct {
		name  string
		lemma string
		want  []string
	}{
		{"simple Hebrew", "H1234", []string{"H1234"}},
		{"simple Greek", "G2532", []string{"G2532"}},
		{"with strong prefix", "strong:H1234", []string{"H1234"}},
		{"multiple numbers", "H1234 H5678", []string{"H1234", "H5678"}},
		{"Greek with strong prefix", "strong:G2316", []string{"G2316"}},
		{"multiple with prefix", "strong:H1234 strong:G5678", []string{"H1234", "G5678"}},
		{"empty string", "", nil},
		{"invalid - no digits", "Habc", nil},
		{"invalid - no H/G", "1234", nil},
		{"mixed valid and invalid", "H1234 abc G5678", []string{"H1234", "G5678"}},
		{"single letter", "H", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStrongs(tt.lemma)
			if len(got) != len(tt.want) {
				t.Errorf("parseStrongs(%q) returned %d items, want %d", tt.lemma, len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseStrongs(%q)[%d] = %q, want %q", tt.lemma, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTokenizePlainText(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantCount int
	}{
		{"simple words", "Hello world", 2},
		{"with punctuation", "Hello, world!", 2},
		{"with apostrophe", "don't", 1},
		{"unicode text", "שָׁלוֹם", 3}, // Hebrew with diacritics splits by combining chars
		{"numbers", "test123 456test", 2},
		{"empty string", "", 0},
		{"only spaces", "   ", 0},
		{"mixed content", "The LORD said: 'Go!'", 5}, // 'Go' has apostrophe, treated as Go'
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokenizePlainText(tt.text)
			if len(tokens) != tt.wantCount {
				t.Errorf("tokenizePlainText(%q) returned %d tokens, want %d", tt.text, len(tokens), tt.wantCount)
			}
		})
	}
}

func TestParseTokensFromMarkup(t *testing.T) {
	tests := []struct {
		name      string
		markup    string
		wantCount int
		checkFirst bool
		firstText  string
		firstLemma string
	}{
		{"plain text no tags", "In the beginning", 3, true, "In", ""},
		{"simple w tag", `<w lemma="strong:G2316">God</w>`, 1, true, "God", "strong:G2316"},
		{"w tag with morph", `<w lemma="H1234" morph="N-NSM">word</w>`, 1, true, "word", "H1234"},
		{"multiple w tags", `<w lemma="G1">the</w> <w lemma="G2">Lord</w>`, 2, true, "the", "G1"},
		{"mixed content", `text <w lemma="G123">word</w> more`, 1, true, "word", "G123"},
		{"unclosed w tag", `<w lemma="G1"`, 0, false, "", ""},
		{"empty w tag", `<w lemma="G1"></w>`, 0, false, "", ""},
		{"empty string", "", 0, false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := parseTokensFromMarkup(tt.markup)
			if len(tokens) != tt.wantCount {
				t.Errorf("parseTokensFromMarkup(%q) returned %d tokens, want %d", tt.markup, len(tokens), tt.wantCount)
				return
			}
			if tt.checkFirst && len(tokens) > 0 {
				if tokens[0].Text != tt.firstText {
					t.Errorf("first token text = %q, want %q", tokens[0].Text, tt.firstText)
				}
				if tokens[0].Lemma != tt.firstLemma {
					t.Errorf("first token lemma = %q, want %q", tokens[0].Lemma, tt.firstLemma)
				}
			}
		})
	}
}

func TestParseAnnotationsFromMarkup(t *testing.T) {
	tests := []struct {
		name      string
		markup    string
		wantCount int
	}{
		{"no annotations", "plain text", 0},
		{"footnote", `text<note>footnote text</note>more text`, 1},
		{"cross reference", `text<reference osisRef="Gen.1.1">Genesis 1:1</reference>more`, 1},
		{"multiple notes", `<note>first</note> text <note>second</note>`, 2},
		{"note and reference", `<note>fn</note><reference>ref</reference>`, 2},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annots := parseAnnotationsFromMarkup(tt.markup)
			if len(annots) != tt.wantCount {
				t.Errorf("parseAnnotationsFromMarkup(%q) returned %d annotations, want %d", tt.markup, len(annots), tt.wantCount)
			}
		})
	}
}

// Helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
