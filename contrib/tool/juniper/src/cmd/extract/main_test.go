// Package main provides comprehensive tests for the extract command.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Diatheke Output Parsing Tests
// =============================================================================

func TestParseDiathekeOutput_SingleVerse(t *testing.T) {
	output := `Genesis 1:1: In the beginning God created the heaven and the earth.

(KJV)`
	verses := parseDiathekeOutput(output)

	if len(verses) != 1 {
		t.Fatalf("expected 1 verse, got %d", len(verses))
	}
	if verses[0].Number != 1 {
		t.Errorf("verse number = %d, want 1", verses[0].Number)
	}
	if verses[0].Text == "" {
		t.Error("verse text is empty")
	}
	if !strings.Contains(verses[0].Text, "In the beginning") {
		t.Errorf("verse text missing expected content: %q", verses[0].Text)
	}
}

func TestParseDiathekeOutput_MultipleVerses(t *testing.T) {
	output := `Genesis 1:1: In the beginning God created the heaven and the earth.
Genesis 1:2: And the earth was without form, and void; and darkness was upon the face of the deep.
Genesis 1:3: And God said, Let there be light: and there was light.

(KJV)`
	verses := parseDiathekeOutput(output)

	if len(verses) != 3 {
		t.Fatalf("expected 3 verses, got %d", len(verses))
	}
	if verses[0].Number != 1 {
		t.Errorf("verse[0].Number = %d, want 1", verses[0].Number)
	}
	if verses[1].Number != 2 {
		t.Errorf("verse[1].Number = %d, want 2", verses[1].Number)
	}
	if verses[2].Number != 3 {
		t.Errorf("verse[2].Number = %d, want 3", verses[2].Number)
	}
}

func TestParseDiathekeOutput_HighVerseNumbers(t *testing.T) {
	output := `Psalms 119:175: Let my soul live, and it shall praise thee.
Psalms 119:176: I have gone astray like a lost sheep.

(KJV)`
	verses := parseDiathekeOutput(output)

	if len(verses) != 2 {
		t.Fatalf("expected 2 verses, got %d", len(verses))
	}
	if verses[0].Number != 175 {
		t.Errorf("verse[0].Number = %d, want 175", verses[0].Number)
	}
	if verses[1].Number != 176 {
		t.Errorf("verse[1].Number = %d, want 176", verses[1].Number)
	}
}

func TestParseDiathekeOutput_NewTestamentBooks(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{
			name: "Matthew",
			output: `Matthew 1:1: The book of the generation of Jesus Christ.

(KJV)`,
			want: 1,
		},
		{
			name: "1 Corinthians",
			output: `1 Corinthians 13:1: Though I speak with the tongues of men and of angels.

(KJV)`,
			want: 1,
		},
		{
			name: "2 Timothy",
			output: `2 Timothy 3:16: All scripture is given by inspiration of God.

(KJV)`,
			want: 16,
		},
		{
			name: "Revelation",
			output: `Revelation 22:21: The grace of our Lord Jesus Christ be with you all.

(KJV)`,
			want: 21,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verses := parseDiathekeOutput(tt.output)
			if len(verses) != 1 {
				t.Fatalf("expected 1 verse, got %d", len(verses))
			}
			if verses[0].Number != tt.want {
				t.Errorf("verse number = %d, want %d", verses[0].Number, tt.want)
			}
		})
	}
}

func TestParseDiathekeOutput_RemovesModuleAttribution(t *testing.T) {
	modules := []string{"KJV", "DRC", "Vulgate", "WEB", "ASV"}
	for _, mod := range modules {
		t.Run(mod, func(t *testing.T) {
			output := `John 3:16: For God so loved the world.

(` + mod + `)`
			verses := parseDiathekeOutput(output)

			if len(verses) != 1 {
				t.Fatalf("expected 1 verse, got %d", len(verses))
			}
			if strings.Contains(verses[0].Text, "("+mod+")") {
				t.Error("verse text should not contain module attribution")
			}
		})
	}
}

func TestParseDiathekeOutput_EmptyOutput(t *testing.T) {
	verses := parseDiathekeOutput("")
	if len(verses) != 0 {
		t.Errorf("expected 0 verses for empty output, got %d", len(verses))
	}
}

func TestParseDiathekeOutput_OnlyAttribution(t *testing.T) {
	output := "(KJV)"
	verses := parseDiathekeOutput(output)
	if len(verses) != 0 {
		t.Errorf("expected 0 verses for attribution-only output, got %d", len(verses))
	}
}

func TestParseDiathekeOutput_WhitespaceOnly(t *testing.T) {
	outputs := []string{
		"   ",
		"\n\n\n",
		"\t\t",
		"  \n  \t  \n  ",
	}
	for _, output := range outputs {
		verses := parseDiathekeOutput(output)
		if len(verses) != 0 {
			t.Errorf("expected 0 verses for whitespace output %q, got %d", output, len(verses))
		}
	}
}

func TestParseDiathekeOutput_MultilineVerse(t *testing.T) {
	output := `Psalms 23:1: The LORD is my shepherd; I shall not want.
He maketh me to lie down in green pastures.

(KJV)`
	verses := parseDiathekeOutput(output)

	// May combine or parse as single verse depending on implementation
	if len(verses) < 1 {
		t.Error("expected at least 1 verse")
	}
}

func TestParseDiathekeOutput_SpecialCharacters(t *testing.T) {
	output := `John 1:1: In the beginning was the Word, and the Word was with God, and the Word was God.

(KJV)`
	verses := parseDiathekeOutput(output)

	if len(verses) != 1 {
		t.Fatalf("expected 1 verse, got %d", len(verses))
	}
	// Check that commas and special punctuation are preserved
	if !strings.Contains(verses[0].Text, ",") {
		t.Error("expected commas to be preserved in verse text")
	}
}

func TestParseDiathekeOutput_HebrewMarkup(t *testing.T) {
	// Test OSIS markup from Hebrew text
	output := `Genesis 1:1: <w savlm="strong:H07225">In the beginning</w> <w savlm="strong:H0430">God</w> created.

(KJV)`
	verses := parseDiathekeOutput(output)

	if len(verses) != 1 {
		t.Fatalf("expected 1 verse, got %d", len(verses))
	}
	// Strong's markup may be preserved or stripped depending on format
	if verses[0].Text == "" {
		t.Error("verse text should not be empty")
	}
}

// =============================================================================
// Placeholder Detection Tests
// =============================================================================

func TestIsPlaceholderText(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		// Short text - placeholder
		{"", true},
		{"a", true},
		{"abc", true},
		{"ab", true},
		{"abcd", true},

		// Verse reference patterns - placeholder
		{"Genesis 1:1:", true},
		{"II Chronicles 19:2:", true},
		{"1 John 3:16:", true},
		{"Song of Songs 1:1:", true},
		{"Psalms 119:176:", true},
		{"Matthew 28:20:", true},
		{"Revelation 22:21:", true},
		{"3 John 1:14:", true},

		// Actual verse content - not placeholder
		{"In the beginning God created the heaven and the earth.", false},
		{"For God so loved the world, that he gave his only begotten Son.", false},
		{"The LORD is my shepherd; I shall not want.", false},
		{"In principio creavit Deus caelum et terram.", false},
		{"Ἐν ἀρχῇ ἦν ὁ λόγος", false},
		{"בְּרֵאשִׁית בָּרָא אֱלֹהִים", false},
	}

	for _, tt := range tests {
		name := tt.text
		if len(name) > 30 {
			name = name[:30] + "..."
		}
		if name == "" {
			name = "(empty)"
		}
		t.Run(name, func(t *testing.T) {
			result := isPlaceholderText(tt.text)
			if result != tt.expected {
				t.Errorf("isPlaceholderText(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}

func TestIsPlaceholderText_EdgeCases(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		// Exactly 5 characters (boundary)
		{"abcde", false},
		{"abcd", true},

		// Short words that aren't placeholders
		{"Jesus", false},
		{"Amen.", false},
		{"Selah", false},

		// Numbers only
		{"12345", false},
		{"123", true},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := isPlaceholderText(tt.text)
			if result != tt.expected {
				t.Errorf("isPlaceholderText(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Scripture Registry Tests
// =============================================================================

func TestScripturesRegistry(t *testing.T) {
	// Verify we have all 27 expected modules
	if len(scriptures) != 27 {
		t.Errorf("scriptures has %d entries, want 27", len(scriptures))
	}

	// Verify key modules exist
	expectedModules := []string{
		"KJV", "Tyndale", "Geneva1599", "DRC", "Vulgate",
		"ASV", "WEB", "LXX", "SBLGNT", "OSMHB",
		"Darby", "YLT", "BBE", "Webster", "AKJV",
	}

	for _, module := range expectedModules {
		if _, ok := scriptures[module]; !ok {
			t.Errorf("scriptures missing expected module %q", module)
		}
	}
}

func TestScripturesMetadata(t *testing.T) {
	for module, meta := range scriptures {
		t.Run(module, func(t *testing.T) {
			if meta.ID == "" {
				t.Errorf("module %s has empty ID", module)
			}
			if meta.Title == "" {
				t.Errorf("module %s has empty Title", module)
			}
			if meta.Description == "" {
				t.Errorf("module %s has empty Description", module)
			}
			if meta.Abbrev == "" {
				t.Errorf("module %s has empty Abbrev", module)
			}
			if meta.Language == "" {
				t.Errorf("module %s has empty Language", module)
			}
			if meta.Versification == "" {
				t.Errorf("module %s has empty Versification", module)
			}
			if len(meta.Tags) == 0 {
				t.Errorf("module %s has no Tags", module)
			}
		})
	}
}

func TestScripturesIDUniqueness(t *testing.T) {
	ids := make(map[string]string)
	for module, meta := range scriptures {
		if existing, ok := ids[meta.ID]; ok {
			t.Errorf("duplicate ID %q: used by both %s and %s", meta.ID, existing, module)
		}
		ids[meta.ID] = module
	}
}

func TestScripturesAbbrevUniqueness(t *testing.T) {
	abbrevs := make(map[string]string)
	for module, meta := range scriptures {
		if existing, ok := abbrevs[meta.Abbrev]; ok {
			t.Errorf("duplicate Abbrev %q: used by both %s and %s", meta.Abbrev, existing, module)
		}
		abbrevs[meta.Abbrev] = module
	}
}

func TestVersificationValues(t *testing.T) {
	validVersifications := map[string]bool{
		"protestant": true,
		"catholic":   true,
	}

	for module, meta := range scriptures {
		if !validVersifications[meta.Versification] {
			t.Errorf("module %s has invalid versification %q", module, meta.Versification)
		}
	}
}

func TestLanguageCodes(t *testing.T) {
	validLanguages := map[string]bool{
		"en":  true, // English
		"la":  true, // Latin
		"grc": true, // Greek (Ancient)
		"he":  true, // Hebrew
	}

	for module, meta := range scriptures {
		if !validLanguages[meta.Language] {
			t.Errorf("module %s has unexpected language code %q", module, meta.Language)
		}
	}
}

func TestTagsNotEmpty(t *testing.T) {
	for module, meta := range scriptures {
		if len(meta.Tags) == 0 {
			t.Errorf("module %s has no tags", module)
		}
		for i, tag := range meta.Tags {
			if tag == "" {
				t.Errorf("module %s has empty tag at index %d", module, i)
			}
		}
	}
}

// =============================================================================
// Versification Loading Tests
// =============================================================================

func TestLoadVersification_Protestant(t *testing.T) {
	versDir := findVersificationsDir(t)
	v, err := loadVersification(filepath.Join(versDir, "protestant.yaml"))
	if err != nil {
		t.Fatalf("failed to load protestant versification: %v", err)
	}

	if v.Name != "Protestant" {
		t.Errorf("Name = %q, want 'Protestant'", v.Name)
	}
	if len(v.Books) != 66 {
		t.Errorf("Books count = %d, want 66", len(v.Books))
	}
}

func TestLoadVersification_Catholic(t *testing.T) {
	versDir := findVersificationsDir(t)
	v, err := loadVersification(filepath.Join(versDir, "catholic.yaml"))
	if err != nil {
		t.Fatalf("failed to load catholic versification: %v", err)
	}

	if v.Name != "Catholic" {
		t.Errorf("Name = %q, want 'Catholic'", v.Name)
	}
	// Catholic should have more than 66 books (includes deuterocanonical)
	if len(v.Books) <= 66 {
		t.Errorf("Books count = %d, want > 66", len(v.Books))
	}
}

func TestLoadVersification_ProtestantBookOrder(t *testing.T) {
	versDir := findVersificationsDir(t)
	v, err := loadVersification(filepath.Join(versDir, "protestant.yaml"))
	if err != nil {
		t.Fatalf("failed to load protestant versification: %v", err)
	}

	// First book should be Genesis
	if v.Books[0].ID != "Gen" {
		t.Errorf("first book ID = %q, want 'Gen'", v.Books[0].ID)
	}
	if v.Books[0].Name != "Genesis" {
		t.Errorf("first book Name = %q, want 'Genesis'", v.Books[0].Name)
	}
	if v.Books[0].Chapters != 50 {
		t.Errorf("Genesis chapters = %d, want 50", v.Books[0].Chapters)
	}

	// Last book should be Revelation
	lastIdx := len(v.Books) - 1
	if v.Books[lastIdx].ID != "Rev" {
		t.Errorf("last book ID = %q, want 'Rev'", v.Books[lastIdx].ID)
	}
	if v.Books[lastIdx].Name != "Revelation" {
		t.Errorf("last book Name = %q, want 'Revelation'", v.Books[lastIdx].Name)
	}
	if v.Books[lastIdx].Chapters != 22 {
		t.Errorf("Revelation chapters = %d, want 22", v.Books[lastIdx].Chapters)
	}
}

func TestLoadVersification_TestamentDistribution(t *testing.T) {
	versDir := findVersificationsDir(t)
	v, err := loadVersification(filepath.Join(versDir, "protestant.yaml"))
	if err != nil {
		t.Fatalf("failed to load protestant versification: %v", err)
	}

	otCount := 0
	ntCount := 0
	for _, book := range v.Books {
		switch book.Testament {
		case "OT":
			otCount++
		case "NT":
			ntCount++
		}
	}

	if otCount != 39 {
		t.Errorf("OT books = %d, want 39", otCount)
	}
	if ntCount != 27 {
		t.Errorf("NT books = %d, want 27", ntCount)
	}
}

func TestLoadVersification_CatholicDeuterocanonical(t *testing.T) {
	versDir := findVersificationsDir(t)
	v, err := loadVersification(filepath.Join(versDir, "catholic.yaml"))
	if err != nil {
		t.Fatalf("failed to load catholic versification: %v", err)
	}

	// Check for deuterocanonical books
	deuterocanonical := []string{"Tob", "Jdt", "Wis", "Sir", "Bar", "1Macc", "2Macc"}
	bookIDs := make(map[string]bool)
	for _, book := range v.Books {
		bookIDs[book.ID] = true
	}

	for _, dc := range deuterocanonical {
		if !bookIDs[dc] {
			t.Errorf("missing deuterocanonical book: %s", dc)
		}
	}
}

func TestLoadVersification_ChapterCounts(t *testing.T) {
	versDir := findVersificationsDir(t)
	v, err := loadVersification(filepath.Join(versDir, "protestant.yaml"))
	if err != nil {
		t.Fatalf("failed to load protestant versification: %v", err)
	}

	expectedChapters := map[string]int{
		"Gen":  50,
		"Exod": 40,
		"Ps":   150,
		"Isa":  66,
		"Matt": 28,
		"John": 21,
		"Acts": 28,
		"Rom":  16,
		"Rev":  22,
	}

	for _, book := range v.Books {
		if expected, ok := expectedChapters[book.ID]; ok {
			if book.Chapters != expected {
				t.Errorf("%s chapters = %d, want %d", book.ID, book.Chapters, expected)
			}
		}
	}
}

// =============================================================================
// Book and Chapter Content Types Tests
// =============================================================================

func TestBookContent_Empty(t *testing.T) {
	bc := BookContent{
		ID:        "Gen",
		Name:      "Genesis",
		Testament: "OT",
		Chapters:  []ChapterContent{},
	}

	if len(bc.Chapters) != 0 {
		t.Errorf("expected empty chapters, got %d", len(bc.Chapters))
	}
}

func TestChapterContent_WithVerses(t *testing.T) {
	cc := ChapterContent{
		Number: 1,
		Verses: []VerseContent{
			{Number: 1, Text: "In the beginning..."},
			{Number: 2, Text: "And the earth was..."},
			{Number: 3, Text: "And God said..."},
		},
	}

	if cc.Number != 1 {
		t.Errorf("chapter number = %d, want 1", cc.Number)
	}
	if len(cc.Verses) != 3 {
		t.Errorf("verses count = %d, want 3", len(cc.Verses))
	}
}

func TestVerseContent_Structure(t *testing.T) {
	vc := VerseContent{
		Number: 16,
		Text:   "For God so loved the world...",
	}

	if vc.Number != 16 {
		t.Errorf("verse number = %d, want 16", vc.Number)
	}
	if vc.Text == "" {
		t.Error("verse text is empty")
	}
}

// =============================================================================
// BibleMeta Tests
// =============================================================================

func TestBibleMeta_AllFieldsSet(t *testing.T) {
	meta := BibleMeta{
		ID:            "kjv",
		Title:         "King James Version",
		Description:   "The Authorized Version",
		Abbrev:        "KJV",
		Language:      "en",
		Versification: "protestant",
		Features:      []string{"strongs", "morph"},
		Tags:          []string{"English", "Protestant"},
		Weight:        1,
	}

	if meta.ID == "" {
		t.Error("ID is empty")
	}
	if meta.Title == "" {
		t.Error("Title is empty")
	}
	if meta.Description == "" {
		t.Error("Description is empty")
	}
	if meta.Abbrev == "" {
		t.Error("Abbrev is empty")
	}
	if meta.Language == "" {
		t.Error("Language is empty")
	}
	if meta.Versification == "" {
		t.Error("Versification is empty")
	}
	if len(meta.Features) == 0 {
		t.Error("Features is empty")
	}
	if len(meta.Tags) == 0 {
		t.Error("Tags is empty")
	}
	if meta.Weight == 0 {
		t.Error("Weight is zero")
	}
}

// =============================================================================
// BibleMetadata Tests
// =============================================================================

func TestBibleMetadata_Structure(t *testing.T) {
	metadata := BibleMetadata{
		Bibles: []BibleMeta{
			{ID: "kjv", Title: "KJV"},
			{ID: "drc", Title: "DRC"},
		},
	}
	metadata.Meta.Granularity = "chapter"
	metadata.Meta.Generated = "2026-01-01T00:00:00Z"
	metadata.Meta.Version = "2.0.0"

	if len(metadata.Bibles) != 2 {
		t.Errorf("bibles count = %d, want 2", len(metadata.Bibles))
	}
	if metadata.Meta.Granularity != "chapter" {
		t.Errorf("granularity = %q, want 'chapter'", metadata.Meta.Granularity)
	}
	if metadata.Meta.Version != "2.0.0" {
		t.Errorf("version = %q, want '2.0.0'", metadata.Meta.Version)
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func findVersificationsDir(t *testing.T) string {
	t.Helper()

	tryDirs := []string{
		"versifications",
		"../../versifications",
		"tools/juniper/versifications",
	}

	for _, dir := range tryDirs {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}

	// Try to find from GOPATH or module root
	cwd, err := os.Getwd()
	if err == nil {
		// Walk up to find versifications
		for i := 0; i < 5; i++ {
			versDir := filepath.Join(cwd, "versifications")
			if _, err := os.Stat(versDir); err == nil {
				return versDir
			}
			cwd = filepath.Dir(cwd)
		}
	}

	t.Skip("versifications directory not found")
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// =============================================================================
// WriteJSON Tests
// =============================================================================

func TestWriteJSON_ValidData(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	data := map[string]string{"key": "value"}
	if err := writeJSON(path, data); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if !strings.Contains(string(content), `"key": "value"`) {
		t.Errorf("written content missing expected data: %s", content)
	}
}

func TestWriteJSON_BibleMeta(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bible.json")

	meta := BibleMeta{
		ID:            "kjv",
		Title:         "King James Version",
		Description:   "The Authorized Version",
		Abbrev:        "KJV",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant"},
		Weight:        1,
	}

	if err := writeJSON(path, meta); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	// Verify key fields are present
	contentStr := string(content)
	if !strings.Contains(contentStr, `"id": "kjv"`) {
		t.Error("missing id field")
	}
	if !strings.Contains(contentStr, `"title": "King James Version"`) {
		t.Error("missing title field")
	}
}

func TestWriteJSON_BibleMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bibles.json")

	metadata := BibleMetadata{
		Bibles: []BibleMeta{
			{ID: "kjv", Title: "KJV"},
			{ID: "drc", Title: "DRC"},
		},
	}
	metadata.Meta.Granularity = "chapter"
	metadata.Meta.Generated = "2026-01-01T00:00:00Z"
	metadata.Meta.Version = "2.0.0"

	if err := writeJSON(path, metadata); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, `"granularity": "chapter"`) {
		t.Error("missing granularity field")
	}
	if !strings.Contains(contentStr, `"version": "2.0.0"`) {
		t.Error("missing version field")
	}
}

func TestWriteJSON_BibleContent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "kjv.json")

	content := BibleContent{
		Content: "Test description",
		Books: []BookContent{
			{
				ID:        "Gen",
				Name:      "Genesis",
				Testament: "OT",
				Chapters: []ChapterContent{
					{
						Number: 1,
						Verses: []VerseContent{
							{Number: 1, Text: "In the beginning..."},
							{Number: 2, Text: "And the earth was..."},
						},
					},
				},
			},
		},
		Sections: []interface{}{},
	}

	if err := writeJSON(path, content); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	dataStr := string(data)
	if !strings.Contains(dataStr, `"id": "Gen"`) {
		t.Error("missing book id")
	}
	if !strings.Contains(dataStr, `"In the beginning..."`) {
		t.Error("missing verse text")
	}
}

func TestWriteJSON_InvalidPath(t *testing.T) {
	err := writeJSON("/nonexistent/directory/file.json", map[string]string{})
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestWriteJSON_NoHTMLEscaping(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	data := map[string]string{"html": "<div>test & more</div>"}
	if err := writeJSON(path, data); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	// HTML should NOT be escaped
	if strings.Contains(string(content), `\u003c`) {
		t.Error("HTML characters should not be escaped")
	}
	if !strings.Contains(string(content), "<div>") {
		t.Error("HTML characters should be preserved as-is")
	}
}

// =============================================================================
// Placeholder Pattern Tests
// =============================================================================

func TestPlaceholderPattern_BookNames(t *testing.T) {
	tests := []struct {
		text    string
		isMatch bool
	}{
		{"Genesis 1:1:", true},
		{"Exodus 20:1:", true},
		{"Psalms 23:1:", true},
		{"Proverbs 1:1:", true},
		{"Song of Solomon 1:1:", true},
		{"Song of Songs 1:1:", true},
		{"1 Samuel 1:1:", true},
		{"2 Kings 1:1:", true},
		{"1 Corinthians 1:1:", true},
		{"2 Corinthians 1:1:", true},
		{"3 John 1:1:", true},
		{"I Samuel 1:1:", true},
		{"II Kings 1:1:", true},
		{"III John 1:1:", true},
		{"IV Esdras 1:1:", true},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			match := placeholderPattern.MatchString(tt.text)
			if match != tt.isMatch {
				t.Errorf("placeholderPattern.MatchString(%q) = %v, want %v", tt.text, match, tt.isMatch)
			}
		})
	}
}

func TestPlaceholderPattern_NonMatches(t *testing.T) {
	// These should NOT match the pattern
	tests := []string{
		"In the beginning God created",
		"For God so loved the world",
		"The LORD is my shepherd",
		"Blessed are the poor in spirit",
		"And he said unto them",
		"", // empty string
	}

	for _, text := range tests {
		t.Run(text, func(t *testing.T) {
			if placeholderPattern.MatchString(text) {
				t.Errorf("placeholderPattern should NOT match %q", text)
			}
		})
	}
}

// =============================================================================
// Versification Inheritance Tests
// =============================================================================

func TestLoadVersification_Inheritance(t *testing.T) {
	versDir := findVersificationsDir(t)

	// Catholic inherits from Protestant
	catholic, err := loadVersification(filepath.Join(versDir, "catholic.yaml"))
	if err != nil {
		t.Fatalf("failed to load catholic versification: %v", err)
	}

	// Should have more books than base Protestant (66)
	if len(catholic.Books) <= 66 {
		t.Errorf("catholic versification should have more than 66 books, got %d", len(catholic.Books))
	}

	// First book should still be Genesis (inherited)
	if catholic.Books[0].ID != "Gen" {
		t.Errorf("first book should be Gen, got %s", catholic.Books[0].ID)
	}
}

func TestLoadVersification_NonexistentFile(t *testing.T) {
	_, err := loadVersification("/nonexistent/path/versification.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadVersification_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.yaml")

	if err := os.WriteFile(invalidPath, []byte("{{invalid yaml"), 0600); err != nil {
		t.Fatalf("failed to create invalid yaml file: %v", err)
	}

	_, err := loadVersification(invalidPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

// =============================================================================
// BibleAuxiliary Tests (backwards compatibility)
// =============================================================================

func TestBibleAuxiliary_Structure(t *testing.T) {
	aux := BibleAuxiliary{
		Bibles: map[string]BibleContent{
			"kjv": {
				Content: "KJV content",
				Books: []BookContent{
					{ID: "Gen", Name: "Genesis", Testament: "OT"},
				},
				Sections: []interface{}{},
			},
		},
	}

	if len(aux.Bibles) != 1 {
		t.Errorf("expected 1 bible, got %d", len(aux.Bibles))
	}
	if _, ok := aux.Bibles["kjv"]; !ok {
		t.Error("missing kjv bible")
	}
}

// =============================================================================
// Data Structure Serialization Tests
// =============================================================================

func TestBookContent_JSONMarshaling(t *testing.T) {
	bc := BookContent{
		ID:        "Gen",
		Name:      "Genesis",
		Testament: "OT",
		Chapters: []ChapterContent{
			{
				Number: 1,
				Verses: []VerseContent{
					{Number: 1, Text: "In the beginning..."},
				},
			},
		},
	}

	data, err := json.Marshal(bc)
	if err != nil {
		t.Fatalf("failed to marshal BookContent: %v", err)
	}

	var unmarshaled BookContent
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal BookContent: %v", err)
	}

	if unmarshaled.ID != bc.ID {
		t.Errorf("ID mismatch: got %s, want %s", unmarshaled.ID, bc.ID)
	}
	if unmarshaled.Name != bc.Name {
		t.Errorf("Name mismatch: got %s, want %s", unmarshaled.Name, bc.Name)
	}
	if len(unmarshaled.Chapters) != len(bc.Chapters) {
		t.Errorf("Chapters count mismatch: got %d, want %d", len(unmarshaled.Chapters), len(bc.Chapters))
	}
}

func TestVersification_JSONMarshaling(t *testing.T) {
	v := Versification{
		Name: "Protestant",
		Books: []Book{
			{ID: "Gen", Name: "Genesis", Chapters: 50, Testament: "OT"},
			{ID: "Exod", Name: "Exodus", Chapters: 40, Testament: "OT"},
		},
	}

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal Versification: %v", err)
	}

	var unmarshaled Versification
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal Versification: %v", err)
	}

	if unmarshaled.Name != v.Name {
		t.Errorf("Name mismatch: got %s, want %s", unmarshaled.Name, v.Name)
	}
	if len(unmarshaled.Books) != len(v.Books) {
		t.Errorf("Books count mismatch: got %d, want %d", len(unmarshaled.Books), len(v.Books))
	}
}

// =============================================================================
// Additional Parser Edge Cases
// =============================================================================

func TestParseDiathekeOutput_ColonsInText(t *testing.T) {
	// Verse text contains colons (common in Bible)
	output := `John 1:1: In the beginning was the Word: and the Word was with God.

(KJV)`
	verses := parseDiathekeOutput(output)

	if len(verses) != 1 {
		t.Fatalf("expected 1 verse, got %d", len(verses))
	}
	if !strings.Contains(verses[0].Text, ":") {
		t.Error("verse text should preserve internal colons")
	}
}

func TestParseDiathekeOutput_NumericBookNames(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{
			name: "1 John",
			output: `1 John 1:1: That which was from the beginning.

(KJV)`,
			want: 1,
		},
		{
			name: "2 Peter",
			output: `2 Peter 3:18: But grow in grace.

(KJV)`,
			want: 18,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verses := parseDiathekeOutput(tt.output)
			if len(verses) != 1 {
				t.Fatalf("expected 1 verse, got %d", len(verses))
			}
			if verses[0].Number != tt.want {
				t.Errorf("verse number = %d, want %d", verses[0].Number, tt.want)
			}
		})
	}
}

func TestParseDiathekeOutput_Apocrypha(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{
			name: "Tobit",
			output: `Tobit 1:1: The book of the words of Tobit.

(DRC)`,
			want: 1,
		},
		{
			name: "Wisdom",
			output: `Wisdom 1:1: Love justice, you that are the judges of the earth.

(DRC)`,
			want: 1,
		},
		{
			name: "Sirach",
			output: `Sirach 1:1: All wisdom is from the Lord God.

(DRC)`,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verses := parseDiathekeOutput(tt.output)
			if len(verses) != 1 {
				t.Fatalf("expected 1 verse, got %d", len(verses))
			}
			if verses[0].Number != tt.want {
				t.Errorf("verse number = %d, want %d", verses[0].Number, tt.want)
			}
		})
	}
}

// =============================================================================
// Book Structure Tests
// =============================================================================

func TestBook_Structure(t *testing.T) {
	book := Book{
		ID:          "Gen",
		Name:        "Genesis",
		Chapters:    50,
		Testament:   "OT",
		InsertAfter: "",
		MergeWith:   "",
	}

	if book.ID == "" {
		t.Error("ID is empty")
	}
	if book.Chapters == 0 {
		t.Error("Chapters is zero")
	}
}

func TestBook_ApocryphaFields(t *testing.T) {
	// Deuterocanonical books may have special fields
	book := Book{
		ID:          "Tob",
		Name:        "Tobit",
		Chapters:    14,
		Testament:   "OT", // Or "DC" for deuterocanonical
		InsertAfter: "Neh",
	}

	if book.InsertAfter == "" {
		t.Error("InsertAfter should be set for deuterocanonical books")
	}
}

// =============================================================================
// Versification Detailed Tests
// =============================================================================

func TestVersification_AllBooksHaveChapters(t *testing.T) {
	versDir := findVersificationsDir(t)
	v, err := loadVersification(filepath.Join(versDir, "protestant.yaml"))
	if err != nil {
		t.Fatalf("failed to load protestant versification: %v", err)
	}

	for _, book := range v.Books {
		if book.Chapters == 0 {
			t.Errorf("book %s has 0 chapters", book.ID)
		}
	}
}

func TestVersification_AllBooksHaveTestament(t *testing.T) {
	versDir := findVersificationsDir(t)
	v, err := loadVersification(filepath.Join(versDir, "protestant.yaml"))
	if err != nil {
		t.Fatalf("failed to load protestant versification: %v", err)
	}

	for _, book := range v.Books {
		if book.Testament != "OT" && book.Testament != "NT" {
			t.Errorf("book %s has invalid testament: %s", book.ID, book.Testament)
		}
	}
}

func TestVersification_BookIDsAreUnique(t *testing.T) {
	versDir := findVersificationsDir(t)
	v, err := loadVersification(filepath.Join(versDir, "protestant.yaml"))
	if err != nil {
		t.Fatalf("failed to load protestant versification: %v", err)
	}

	ids := make(map[string]bool)
	for _, book := range v.Books {
		if ids[book.ID] {
			t.Errorf("duplicate book ID: %s", book.ID)
		}
		ids[book.ID] = true
	}
}

func TestVersification_BookNamesAreUnique(t *testing.T) {
	versDir := findVersificationsDir(t)
	v, err := loadVersification(filepath.Join(versDir, "protestant.yaml"))
	if err != nil {
		t.Fatalf("failed to load protestant versification: %v", err)
	}

	names := make(map[string]bool)
	for _, book := range v.Books {
		if names[book.Name] {
			t.Errorf("duplicate book name: %s", book.Name)
		}
		names[book.Name] = true
	}
}
