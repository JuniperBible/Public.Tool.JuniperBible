package osis

import (
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

func TestParseOSISToIR_SimpleGenesis(t *testing.T) {
	osisXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="KJV" xml:lang="en">
    <header>
      <work osisWork="KJV">
        <title>King James Version</title>
        <language>en</language>
        <refSystem>Bible.KJV</refSystem>
      </work>
    </header>
    <div type="book" osisID="Gen">
      <title>Genesis</title>
      <p>In the beginning God created the heaven and the earth.</p>
    </div>
  </osisText>
</osis>`)

	corpus, err := parseOSISToIR(osisXML)
	if err != nil {
		t.Fatalf("parseOSISToIR failed: %v", err)
	}

	if corpus.ID != "KJV" {
		t.Errorf("Corpus ID = %q, want KJV", corpus.ID)
	}
	if corpus.Title != "King James Version" {
		t.Errorf("Corpus Title = %q, want 'King James Version'", corpus.Title)
	}
	if corpus.Language != "en" {
		t.Errorf("Corpus Language = %q, want en", corpus.Language)
	}
	if corpus.Versification != "Bible.KJV" {
		t.Errorf("Corpus Versification = %q, want Bible.KJV", corpus.Versification)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}

	doc := corpus.Documents[0]
	if doc.ID != "Gen" {
		t.Errorf("Document ID = %q, want Gen", doc.ID)
	}
	if doc.Title != "Genesis" {
		t.Errorf("Document Title = %q, want Genesis", doc.Title)
	}
}

func TestParseOSISToIR_MultipleBooks(t *testing.T) {
	osisXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="Test" lang="en">
    <div type="book" osisID="Matt">
      <p>First book content</p>
    </div>
    <div type="book" osisID="Mark">
      <p>Second book content</p>
    </div>
  </osisText>
</osis>`)

	corpus, err := parseOSISToIR(osisXML)
	if err != nil {
		t.Fatalf("parseOSISToIR failed: %v", err)
	}

	if len(corpus.Documents) != 2 {
		t.Fatalf("Expected 2 documents, got %d", len(corpus.Documents))
	}

	if corpus.Documents[0].ID != "Matt" {
		t.Errorf("First document ID = %q, want Matt", corpus.Documents[0].ID)
	}
	if corpus.Documents[1].ID != "Mark" {
		t.Errorf("Second document ID = %q, want Mark", corpus.Documents[1].ID)
	}
}

func TestParseOSISToIR_PoetryLines(t *testing.T) {
	osisXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="Test">
    <div type="book" osisID="Ps">
      <lg>
        <l>The LORD is my shepherd;</l>
        <l>I shall not want.</l>
      </lg>
    </div>
  </osisText>
</osis>`)

	corpus, err := parseOSISToIR(osisXML)
	if err != nil {
		t.Fatalf("parseOSISToIR failed: %v", err)
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}

	doc := corpus.Documents[0]
	if len(doc.ContentBlocks) != 2 {
		t.Errorf("Expected 2 content blocks for poetry lines, got %d", len(doc.ContentBlocks))
	}
}

func TestParseOSISToIR_VersesInParagraph(t *testing.T) {
	osisXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="Test">
    <div type="book" osisID="Gen">
      <p><verse osisID="Gen.1.1"/>In the beginning God created the heaven and the earth.</p>
    </div>
  </osisText>
</osis>`)

	corpus, err := parseOSISToIR(osisXML)
	if err != nil {
		t.Fatalf("parseOSISToIR failed: %v", err)
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}

	doc := corpus.Documents[0]
	if len(doc.ContentBlocks) < 1 {
		t.Fatalf("Expected at least 1 content block, got %d", len(doc.ContentBlocks))
	}
}

func TestParseOSISToIR_VerseSID(t *testing.T) {
	osisXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="Test">
    <div type="book" osisID="Matt">
      <p><verse sID="Matt.1.1"/>In the beginning</p>
    </div>
  </osisText>
</osis>`)

	corpus, err := parseOSISToIR(osisXML)
	if err != nil {
		t.Fatalf("parseOSISToIR failed: %v", err)
	}

	if len(corpus.Documents) < 1 {
		t.Fatal("Expected at least 1 document")
	}
}

func TestParseOSISToIR_NestedDivs(t *testing.T) {
	osisXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="Test">
    <div type="testament">
      <div type="book" osisID="Gen">
        <p>Genesis content</p>
      </div>
      <div type="book" osisID="Exod">
        <p>Exodus content</p>
      </div>
    </div>
  </osisText>
</osis>`)

	corpus, err := parseOSISToIR(osisXML)
	if err != nil {
		t.Fatalf("parseOSISToIR failed: %v", err)
	}

	// Should have 2 documents (nested books)
	if len(corpus.Documents) != 2 {
		t.Errorf("Expected 2 documents from nested divs, got %d", len(corpus.Documents))
	}
}

func TestParseOSISToIR_BookIDWithoutType(t *testing.T) {
	osisXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="Test">
    <div osisID="Gen">
      <p>Content without explicit type=book</p>
    </div>
  </osisText>
</osis>`)

	corpus, err := parseOSISToIR(osisXML)
	if err != nil {
		t.Fatalf("parseOSISToIR failed: %v", err)
	}

	// Should recognize Gen as a book by ID
	if len(corpus.Documents) != 1 {
		t.Errorf("Expected 1 document from book ID, got %d", len(corpus.Documents))
	}
}

func TestParseOSISToIR_EmptyInput(t *testing.T) {
	osisXML := []byte("")
	_, err := parseOSISToIR(osisXML)
	if err == nil {
		t.Error("Expected error for empty input")
	}
}

func TestParseOSISToIR_InvalidXML(t *testing.T) {
	osisXML := []byte("not valid xml")
	_, err := parseOSISToIR(osisXML)
	if err == nil {
		t.Error("Expected error for invalid XML")
	}
}

func TestParseOSISToIR_SourceHash(t *testing.T) {
	osisXML := []byte(`<?xml version="1.0"?><osis><osisText osisIDWork="Test"></osisText></osis>`)
	corpus, err := parseOSISToIR(osisXML)
	if err != nil {
		t.Fatalf("parseOSISToIR failed: %v", err)
	}

	if corpus.SourceHash == "" {
		t.Error("SourceHash should not be empty")
	}
	// SHA256 hex string should be 64 characters
	if len(corpus.SourceHash) != 64 {
		t.Errorf("SourceHash length = %d, want 64", len(corpus.SourceHash))
	}
}

func TestParseOSISRef(t *testing.T) {
	tests := []struct {
		name       string
		osisID     string
		wantBook   string
		wantChap   int
		wantVerse  int
		wantEnd    int
	}{
		{"simple", "Gen.1.1", "Gen", 1, 1, 0},
		{"verse range", "Matt.5.3-12", "Matt", 5, 3, 12},
		{"book only", "Rev", "Rev", 0, 0, 0},
		{"book and chapter", "John.3", "John", 3, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := parseOSISRef(tt.osisID)
			if ref.Book != tt.wantBook {
				t.Errorf("Book = %q, want %q", ref.Book, tt.wantBook)
			}
			if ref.Chapter != tt.wantChap {
				t.Errorf("Chapter = %d, want %d", ref.Chapter, tt.wantChap)
			}
			if ref.Verse != tt.wantVerse {
				t.Errorf("Verse = %d, want %d", ref.Verse, tt.wantVerse)
			}
			if ref.VerseEnd != tt.wantEnd {
				t.Errorf("VerseEnd = %d, want %d", ref.VerseEnd, tt.wantEnd)
			}
		})
	}
}

func TestIsBookID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"Gen", true},
		{"Exod", true},
		{"Matt", true},
		{"Rev", true},
		{"1Cor", true},
		{"2Tim", true},
		{"3John", true},
		{"Ps", true},
		{"Song", true},
		{"Phlm", true},
		{"Unknown", false},
		{"", false},
		{"chapter1", false},
		{"Gen.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := isBookID(tt.id); got != tt.want {
				t.Errorf("isBookID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestEmitOSISFromIR_Simple(t *testing.T) {
	corpus := &ir.Corpus{
		ID:       "KJV",
		Title:    "King James Version",
		Language: "en",
		Documents: []*ir.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:   "cb-1",
						Text: "In the beginning God created the heaven and the earth.",
					},
				},
			},
		},
	}

	data, err := emitOSISFromIR(corpus)
	if err != nil {
		t.Fatalf("emitOSISFromIR failed: %v", err)
	}

	output := string(data)
	if !strings.Contains(output, "<?xml") {
		t.Error("Output should contain XML declaration")
	}
	if !strings.Contains(output, "<osis") {
		t.Error("Output should contain osis element")
	}
	if !strings.Contains(output, `osisIDWork="KJV"`) {
		t.Error("Output should contain osisIDWork")
	}
	if !strings.Contains(output, "<title>King James Version</title>") {
		t.Error("Output should contain title")
	}
	if !strings.Contains(output, `osisID="Gen"`) {
		t.Error("Output should contain book osisID")
	}
}

func TestEmitOSISFromIR_WithVersification(t *testing.T) {
	corpus := &ir.Corpus{
		ID:            "Test",
		Versification: "Bible.KJV",
	}

	data, err := emitOSISFromIR(corpus)
	if err != nil {
		t.Fatalf("emitOSISFromIR failed: %v", err)
	}

	output := string(data)
	if !strings.Contains(output, "<refSystem>Bible.KJV</refSystem>") {
		t.Error("Output should contain refSystem")
	}
}

func TestEmitOSISFromIR_Empty(t *testing.T) {
	corpus := &ir.Corpus{
		ID: "Empty",
	}

	data, err := emitOSISFromIR(corpus)
	if err != nil {
		t.Fatalf("emitOSISFromIR failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Output should not be empty")
	}
}

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"plain text", "plain text"},
		{"<tag>", "&lt;tag&gt;"},
		{"&ampersand", "&amp;ampersand"},
		{`"quotes"`, "&quot;quotes&quot;"},
		{"'apostrophe'", "&apos;apostrophe&apos;"},
		{"<&>\"'", "&lt;&amp;&gt;&quot;&apos;"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := escapeXML(tt.input); got != tt.want {
				t.Errorf("escapeXML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseOSISDiv_TitleFromOsisID(t *testing.T) {
	div := &OSISDiv{
		Type:   "book",
		OsisID: "Gen",
		Title:  "",
		Ps: []OSISP{
			{Content: "Test content"},
		},
	}

	order := 0
	docs := parseOSISDiv(div, &order)

	if len(docs) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(docs))
	}

	// Title should fall back to OsisID
	if docs[0].Title != "Gen" {
		t.Errorf("Title = %q, want Gen", docs[0].Title)
	}
}

func TestExtractContentBlocks_DirectContent(t *testing.T) {
	div := &OSISDiv{
		Content: "  Direct text content  ",
	}

	seq := 0
	blocks := extractContentBlocks(div, &seq)

	if len(blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(blocks))
	}

	if blocks[0].Text != "Direct text content" {
		t.Errorf("Text = %q, want 'Direct text content'", blocks[0].Text)
	}
}

func TestExtractContentBlocks_EmptyParagraph(t *testing.T) {
	div := &OSISDiv{
		Ps: []OSISP{
			{Content: ""},
			{Content: "   "},
			{Content: "Valid content"},
		},
	}

	seq := 0
	blocks := extractContentBlocks(div, &seq)

	// Should only have 1 block (empty ones skipped)
	if len(blocks) != 1 {
		t.Errorf("Expected 1 block (empty skipped), got %d", len(blocks))
	}
}

func TestExtractContentBlocks_EmptyPoetry(t *testing.T) {
	div := &OSISDiv{
		Lgs: []OSISLg{
			{Ls: []OSISL{{Content: ""}, {Content: "   "}, {Content: "Valid line"}}},
		},
	}

	seq := 0
	blocks := extractContentBlocks(div, &seq)

	// Should only have 1 block (empty ones skipped)
	if len(blocks) != 1 {
		t.Errorf("Expected 1 block (empty skipped), got %d", len(blocks))
	}
}

func TestExtractContentBlocks_NestedDivs(t *testing.T) {
	div := &OSISDiv{
		Divs: []OSISDiv{
			{
				Ps: []OSISP{{Content: "Nested content 1"}},
			},
			{
				Ps: []OSISP{{Content: "Nested content 2"}},
			},
		},
	}

	seq := 0
	blocks := extractContentBlocks(div, &seq)

	if len(blocks) != 2 {
		t.Errorf("Expected 2 blocks from nested divs, got %d", len(blocks))
	}
}

func TestOSISTypes(t *testing.T) {
	// Test that OSIS types can be marshaled/unmarshaled properly
	doc := OSISDoc{
		Namespace: "http://www.bibletechnologies.net/2003/OSIS/namespace",
		OsisText: OSISText{
			OsisIDWork: "Test",
			Lang:       "en",
			Header: &OSISHeader{
				Work: []OSISWork{
					{
						OsisWork:    "Test",
						Title:       "Test Title",
						Type:        "Bible",
						Identifier:  "test-id",
						RefSystem:   "Bible.KJV",
						Language:    "en",
						Publisher:   "Test Publisher",
						Rights:      "Public Domain",
						Description: "Test description",
					},
				},
			},
			Divs: []OSISDiv{
				{
					Type:   "book",
					OsisID: "Gen",
					Title:  "Genesis",
					Chapters: []OSISChapter{
						{OsisID: "Gen.1", SID: "Gen.1", EID: "Gen.1"},
					},
					Verses: []OSISVerse{
						{OsisID: "Gen.1.1", SID: "Gen.1.1", EID: "Gen.1.1"},
					},
				},
			},
		},
	}

	// Just verify structure is valid
	if doc.OsisText.OsisIDWork != "Test" {
		t.Error("OsisIDWork not set correctly")
	}
	if doc.OsisText.Header.Work[0].Title != "Test Title" {
		t.Error("Work title not set correctly")
	}
}

func createTestOSISCorpus(id, title string, numDocs, numBlocks int) *ir.Corpus {
	corpus := &ir.Corpus{
		ID:    id,
		Title: title,
	}
	for i := 0; i < numDocs; i++ {
		doc := &ir.Document{
			ID:    id,
			Title: title,
		}
		for j := 0; j < numBlocks; j++ {
			doc.ContentBlocks = append(doc.ContentBlocks, &ir.ContentBlock{
				ID:   "cb-" + string(rune(i)) + string(rune(j)),
				Text: "Test content",
			})
		}
		corpus.Documents = append(corpus.Documents, doc)
	}
	return corpus
}
