package usfm

import (
	"testing"
)

func TestParseUSFMToIR_SimpleGenesis(t *testing.T) {
	usfm := []byte(`\id GEN
\h Genesis
\mt Genesis
\c 1
\v 1 In the beginning God created the heaven and the earth.
\v 2 And the earth was without form, and void.
`)
	corpus, err := parseUSFMToIR(usfm)
	if err != nil {
		t.Fatalf("parseUSFMToIR failed: %v", err)
	}

	if corpus.ID != "GEN" {
		t.Errorf("Corpus ID = %q, want GEN", corpus.ID)
	}
	if corpus.Title != "Genesis" {
		t.Errorf("Corpus Title = %q, want Genesis", corpus.Title)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}

	doc := corpus.Documents[0]
	if doc.ID != "GEN" {
		t.Errorf("Document ID = %q, want GEN", doc.ID)
	}
	if doc.Title != "Genesis" {
		t.Errorf("Document Title = %q, want Genesis", doc.Title)
	}
	if len(doc.ContentBlocks) < 2 {
		t.Errorf("Expected at least 2 content blocks, got %d", len(doc.ContentBlocks))
	}
}

func TestParseUSFMToIR_Poetry(t *testing.T) {
	usfm := []byte(`\id PSA
\h Psalms
\c 23
\q1 The LORD is my shepherd;
\q2 I shall not want.
\qr Right aligned
\qc Center aligned
`)
	corpus, err := parseUSFMToIR(usfm)
	if err != nil {
		t.Fatalf("parseUSFMToIR failed: %v", err)
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}

	doc := corpus.Documents[0]
	// Poetry lines create content blocks
	if len(doc.ContentBlocks) < 4 {
		t.Errorf("Expected at least 4 content blocks for poetry, got %d", len(doc.ContentBlocks))
	}
}

func TestParseUSFMToIR_Paragraphs(t *testing.T) {
	usfm := []byte(`\id JHN
\c 3
\p Paragraph text here
\m More text
\pi Indented paragraph
\mi More indented text
\nb No break text
`)
	corpus, err := parseUSFMToIR(usfm)
	if err != nil {
		t.Fatalf("parseUSFMToIR failed: %v", err)
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}

	doc := corpus.Documents[0]
	if len(doc.ContentBlocks) < 5 {
		t.Errorf("Expected at least 5 content blocks for paragraphs, got %d", len(doc.ContentBlocks))
	}
}

func TestParseUSFMToIR_VerseRanges(t *testing.T) {
	usfm := []byte(`\id MAT
\c 1
\v 1-2 Verse spanning two numbers
\v 3 Single verse
`)
	corpus, err := parseUSFMToIR(usfm)
	if err != nil {
		t.Fatalf("parseUSFMToIR failed: %v", err)
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}

	doc := corpus.Documents[0]
	if len(doc.ContentBlocks) != 2 {
		t.Errorf("Expected 2 content blocks, got %d", len(doc.ContentBlocks))
	}
}

func TestParseUSFMToIR_TOCMarkers(t *testing.T) {
	usfm := []byte(`\id GEN
\toc1 Genesis
\toc2 Genesis
\toc3 Gen
\mt The Book of Genesis
\c 1
\v 1 Text
`)
	corpus, err := parseUSFMToIR(usfm)
	if err != nil {
		t.Fatalf("parseUSFMToIR failed: %v", err)
	}

	// TOC markers should be processed without error
	if corpus.Title != "The Book of Genesis" {
		t.Errorf("Title = %q, want 'The Book of Genesis'", corpus.Title)
	}
}

func TestParseUSFMToIR_EmptyInput(t *testing.T) {
	usfm := []byte("")
	corpus, err := parseUSFMToIR(usfm)
	if err != nil {
		t.Fatalf("parseUSFMToIR failed: %v", err)
	}

	// Should return empty corpus without error
	if len(corpus.Documents) != 0 {
		t.Errorf("Expected 0 documents for empty input, got %d", len(corpus.Documents))
	}
}

func TestParseUSFMToIR_BlankLines(t *testing.T) {
	usfm := []byte(`\id GEN

\c 1

\v 1 Text

`)
	corpus, err := parseUSFMToIR(usfm)
	if err != nil {
		t.Fatalf("parseUSFMToIR failed: %v", err)
	}

	// Should skip blank lines
	if len(corpus.Documents) != 1 {
		t.Errorf("Expected 1 document, got %d", len(corpus.Documents))
	}
}

func TestParseUSFMToIR_AllMainTitleVariants(t *testing.T) {
	variants := []string{"mt", "mt1", "mt2", "mt3"}
	for _, marker := range variants {
		t.Run(marker, func(t *testing.T) {
			usfm := []byte("\\id GEN\n\\" + marker + " Title Text\n")
			corpus, err := parseUSFMToIR(usfm)
			if err != nil {
				t.Fatalf("parseUSFMToIR failed: %v", err)
			}
			if corpus.Title != "Title Text" {
				t.Errorf("Title = %q, want 'Title Text'", corpus.Title)
			}
		})
	}
}

func TestParseUSFMToIR_UnknownBook(t *testing.T) {
	usfm := []byte(`\id XYZ
\c 1
\v 1 Unknown book verse
`)
	corpus, err := parseUSFMToIR(usfm)
	if err != nil {
		t.Fatalf("parseUSFMToIR failed: %v", err)
	}

	// Should still create document, just without title
	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}
	if corpus.Documents[0].ID != "XYZ" {
		t.Errorf("Document ID = %q, want XYZ", corpus.Documents[0].ID)
	}
}

func TestParseUSFMToIR_HeaderSetsTitle(t *testing.T) {
	usfm := []byte(`\id XYZ
\h Custom Header Title
\c 1
\v 1 Text
`)
	corpus, err := parseUSFMToIR(usfm)
	if err != nil {
		t.Fatalf("parseUSFMToIR failed: %v", err)
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}
	// For unknown books, header should set the title
	if corpus.Documents[0].Title != "Custom Header Title" {
		t.Errorf("Title = %q, want 'Custom Header Title'", corpus.Documents[0].Title)
	}
}

func TestParseUSFMToIR_AllPoetryMarkers(t *testing.T) {
	markers := []string{"q", "q1", "q2", "q3", "qr", "qc", "qm"}
	for _, marker := range markers {
		t.Run(marker, func(t *testing.T) {
			usfm := []byte("\\id PSA\n\\" + marker + " Poetry line\n")
			corpus, err := parseUSFMToIR(usfm)
			if err != nil {
				t.Fatalf("parseUSFMToIR failed: %v", err)
			}
			if len(corpus.Documents[0].ContentBlocks) != 1 {
				t.Errorf("Expected 1 content block for %s marker", marker)
			}
		})
	}
}

func TestEmitUSFMFromIR_Simple(t *testing.T) {
	corpus := createTestUSFMCorpus("GEN", "Genesis", 1, 2)
	data, err := emitUSFMFromIR(corpus)
	if err != nil {
		t.Fatalf("emitUSFMFromIR failed: %v", err)
	}

	output := string(data)
	if len(output) == 0 {
		t.Error("Output should not be empty")
	}
	if !contains(output, "\\id GEN") {
		t.Error("Output should contain book ID")
	}
	if !contains(output, "\\h Genesis") {
		t.Error("Output should contain header")
	}
}

func TestEmitUSFMFromIR_EmptyCorpus(t *testing.T) {
	corpus := createTestUSFMCorpus("", "", 0, 0)
	data, err := emitUSFMFromIR(corpus)
	if err != nil {
		t.Fatalf("emitUSFMFromIR failed: %v", err)
	}

	// Empty corpus should produce empty output
	if len(data) != 0 {
		t.Errorf("Expected empty output for empty corpus, got %d bytes", len(data))
	}
}

func TestEmitUSFMFromIR_NoTitle(t *testing.T) {
	corpus := createTestUSFMCorpus("MAT", "", 1, 1)
	data, err := emitUSFMFromIR(corpus)
	if err != nil {
		t.Fatalf("emitUSFMFromIR failed: %v", err)
	}

	output := string(data)
	// Should still have ID but no header
	if !contains(output, "\\id MAT") {
		t.Error("Output should contain book ID")
	}
}

func TestBookNames(t *testing.T) {
	// Verify some expected book mappings
	tests := []struct {
		code string
		name string
	}{
		{"GEN", "Genesis"},
		{"MAT", "Matthew"},
		{"PSA", "Psalms"},
		{"REV", "Revelation"},
		{"1CO", "1 Corinthians"},
		{"3JN", "3 John"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			name, ok := bookNames[tt.code]
			if !ok {
				t.Errorf("Book code %s not found", tt.code)
				return
			}
			if name != tt.name {
				t.Errorf("bookNames[%s] = %q, want %q", tt.code, name, tt.name)
			}
		})
	}
}

func TestBookNamesCount(t *testing.T) {
	// Should have 66 books
	if len(bookNames) != 66 {
		t.Errorf("bookNames has %d entries, want 66", len(bookNames))
	}
}

func TestMarkerRegex(t *testing.T) {
	tests := []struct {
		input   string
		matches bool
		marker  string
	}{
		{`\id GEN`, true, "id"},
		{`\v 1`, true, "v"},
		{`\c 1`, true, "c"},
		{`\mt Genesis`, true, "mt"},
		{`\add*`, true, "add"},
		{`plain text`, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			matches := markerRegex.FindStringSubmatch(tt.input)
			if tt.matches && len(matches) == 0 {
				t.Errorf("Expected match for %q", tt.input)
			}
			if !tt.matches && len(matches) > 0 {
				t.Errorf("Did not expect match for %q", tt.input)
			}
			if tt.matches && len(matches) > 1 && matches[1] != tt.marker {
				t.Errorf("Marker = %q, want %q", matches[1], tt.marker)
			}
		})
	}
}

func TestVerseNumRegex(t *testing.T) {
	tests := []struct {
		input      string
		matches    bool
		verseStart string
		verseEnd   string
	}{
		{"1", true, "1", ""},
		{"1-5", true, "1", "5"},
		{"123", true, "123", ""},
		{"invalid", false, "", ""},
		{"", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			matches := verseNumRegex.FindStringSubmatch(tt.input)
			if tt.matches && len(matches) == 0 {
				t.Errorf("Expected match for %q", tt.input)
				return
			}
			if !tt.matches && len(matches) > 0 {
				t.Errorf("Did not expect match for %q", tt.input)
				return
			}
			if tt.matches && len(matches) > 1 && matches[1] != tt.verseStart {
				t.Errorf("VerseStart = %q, want %q", matches[1], tt.verseStart)
			}
			if tt.matches && len(matches) > 2 && matches[2] != tt.verseEnd {
				t.Errorf("VerseEnd = %q, want %q", matches[2], tt.verseEnd)
			}
		})
	}
}

func TestChapterRegex(t *testing.T) {
	tests := []struct {
		input   string
		matches bool
		chapter string
	}{
		{"1", true, "1"},
		{"150", true, "150"},
		{"abc", false, ""},
		{"", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			matches := chapterRegex.FindStringSubmatch(tt.input)
			if tt.matches && len(matches) == 0 {
				t.Errorf("Expected match for %q", tt.input)
				return
			}
			if !tt.matches && len(matches) > 0 {
				t.Errorf("Did not expect match for %q", tt.input)
				return
			}
			if tt.matches && len(matches) > 1 && matches[1] != tt.chapter {
				t.Errorf("Chapter = %q, want %q", matches[1], tt.chapter)
			}
		})
	}
}

// Helper function
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
