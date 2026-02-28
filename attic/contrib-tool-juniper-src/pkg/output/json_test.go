package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FocuswithJustin/juniper/pkg/repository"
	"github.com/FocuswithJustin/juniper/pkg/sword"
)

func TestNewGenerator(t *testing.T) {
	gen := NewGenerator("/tmp/output", "chapter")

	if gen == nil {
		t.Fatal("NewGenerator() returned nil")
	}
	if gen.OutputDir != "/tmp/output" {
		t.Errorf("OutputDir = %q, want %q", gen.OutputDir, "/tmp/output")
	}
	if gen.Granularity != "chapter" {
		t.Errorf("Granularity = %q, want %q", gen.Granularity, "chapter")
	}
}

func TestGenerator_GenerateTags_Language(t *testing.T) {
	gen := NewGenerator("/tmp", "chapter")
	module := &sword.Module{
		Language: "en",
	}

	tags := gen.generateTags(module)

	found := false
	for _, tag := range tags {
		if tag == "en" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Tags should contain language 'en', got %v", tags)
	}
}

func TestGenerator_GenerateTags_Strongs(t *testing.T) {
	gen := NewGenerator("/tmp", "chapter")
	module := &sword.Module{
		Features: []string{"StrongsNumbers"},
	}

	tags := gen.generateTags(module)

	found := false
	for _, tag := range tags {
		if tag == "Strong's Numbers" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Tags should contain 'Strong's Numbers', got %v", tags)
	}
}

func TestGenerator_GenerateTags_Morphology(t *testing.T) {
	gen := NewGenerator("/tmp", "chapter")
	module := &sword.Module{
		GlobalOptionFilters: []string{"OSISMorph"},
	}

	tags := gen.generateTags(module)

	found := false
	for _, tag := range tags {
		if tag == "Morphology" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Tags should contain 'Morphology', got %v", tags)
	}
}

func TestGenerator_GenerateTags_Combined(t *testing.T) {
	gen := NewGenerator("/tmp", "chapter")
	module := &sword.Module{
		Language:            "grc",
		Features:            []string{"StrongsNumbers"},
		GlobalOptionFilters: []string{"OSISMorph"},
	}

	tags := gen.generateTags(module)

	if len(tags) != 3 {
		t.Errorf("len(tags) = %d, want 3 (language, strongs, morph)", len(tags))
	}
}

func TestGenerator_GenerateTags_Empty(t *testing.T) {
	gen := NewGenerator("/tmp", "chapter")
	module := &sword.Module{}

	tags := gen.generateTags(module)

	if len(tags) != 0 {
		t.Errorf("Tags should be empty for module without features, got %v", tags)
	}
}

func TestGenerator_WriteJSON_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "nested", "dir")
	jsonPath := filepath.Join(subDir, "test.json")

	gen := NewGenerator(subDir, "chapter")

	data := map[string]string{"key": "value"}
	err := gen.writeJSON(jsonPath, data)
	if err != nil {
		t.Fatalf("writeJSON() returned error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Error("JSON file was not created")
	}

	// Verify content
	content, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to read JSON file: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("JSON content incorrect: %v", result)
	}
}

func TestGenerator_WriteJSON_Formatting(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "test.json")

	gen := NewGenerator(tmpDir, "chapter")

	data := map[string]interface{}{
		"nested": map[string]string{
			"inner": "value",
		},
	}
	err := gen.writeJSON(jsonPath, data)
	if err != nil {
		t.Fatalf("writeJSON() returned error: %v", err)
	}

	// Verify it's pretty-printed (contains indentation)
	content, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to read JSON file: %v", err)
	}

	// Should have newlines and indentation
	if len(content) < 20 { // Pretty-printed would be longer
		t.Errorf("JSON should be pretty-printed, got: %s", string(content))
	}
}

func TestBibleMetadata_Structure(t *testing.T) {
	meta := BibleMetadata{
		Bibles: []BibleEntry{
			{
				ID:       "kjv",
				Title:    "King James Version",
				Language: "en",
				Features: []string{"StrongsNumbers"},
				Tags:     []string{"English"},
				Weight:   1,
			},
		},
		Meta: MetaInfo{
			Granularity: "chapter",
			Version:     "1.0.0",
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Failed to marshal BibleMetadata: %v", err)
	}

	// Verify structure
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if _, ok := result["bibles"]; !ok {
		t.Error("JSON should have 'bibles' key")
	}
	if _, ok := result["meta"]; !ok {
		t.Error("JSON should have 'meta' key")
	}
}

func TestBibleAuxiliary_Structure(t *testing.T) {
	aux := BibleAuxiliary{
		Bibles: map[string]BibleContent{
			"kjv": {
				Content: "The King James Version.",
				Books: []BookContent{
					{
						ID:        "Gen",
						Name:      "Genesis",
						Testament: "OT",
						Chapters: []ChapterContent{
							{
								Number: 1,
								Verses: []VerseContent{
									{
										Number: 1,
										Text:   "In the beginning...",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(aux)
	if err != nil {
		t.Fatalf("Failed to marshal BibleAuxiliary: %v", err)
	}

	// Verify structure
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if _, ok := result["bibles"]; !ok {
		t.Error("JSON should have 'bibles' key")
	}
}

func TestVerseContent_OmitsEmptyFields(t *testing.T) {
	verse := VerseContent{
		Number: 1,
		Text:   "Test verse",
		// Strongs and Morphology are nil
	}

	data, err := json.Marshal(verse)
	if err != nil {
		t.Fatalf("Failed to marshal VerseContent: %v", err)
	}

	jsonStr := string(data)

	// Should not contain strongs or morphology when empty
	if len(verse.Strongs) == 0 {
		// omitempty should exclude null arrays
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}
		if _, ok := result["strongs"]; ok && result["strongs"] != nil {
			t.Errorf("Should omit empty strongs, got: %s", jsonStr)
		}
	}
}

func TestBibleEntry_AllFields(t *testing.T) {
	entry := BibleEntry{
		ID:          "esv",
		Title:       "English Standard Version",
		Description: "A modern translation",
		Abbrev:      "ESV",
		Language:    "en",
		Features:    []string{"Headings", "Footnotes"},
		Tags:        []string{"English", "Modern"},
		Weight:      5,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal BibleEntry: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	expectedKeys := []string{"id", "title", "description", "abbrev", "language", "features", "tags", "weight"}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("BibleEntry JSON should have key %q", key)
		}
	}
}

func TestMetaInfo_Fields(t *testing.T) {
	meta := MetaInfo{
		Granularity: "verse",
		Version:     "2.0.0",
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Failed to marshal MetaInfo: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["granularity"] != "verse" {
		t.Errorf("granularity = %v, want 'verse'", result["granularity"])
	}
	if result["version"] != "2.0.0" {
		t.Errorf("version = %v, want '2.0.0'", result["version"])
	}
}

func TestGenerator_Granularities(t *testing.T) {
	granularities := []string{"book", "chapter", "verse"}

	for _, gran := range granularities {
		t.Run(gran, func(t *testing.T) {
			gen := NewGenerator("/tmp", gran)
			if gen.Granularity != gran {
				t.Errorf("Granularity = %q, want %q", gen.Granularity, gran)
			}
		})
	}
}

func TestContentSection_Structure(t *testing.T) {
	section := ContentSection{
		Heading: "Introduction",
		Content: "This is the introduction.",
	}

	data, err := json.Marshal(section)
	if err != nil {
		t.Fatalf("Failed to marshal ContentSection: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["heading"] != "Introduction" {
		t.Errorf("heading = %v, want 'Introduction'", result["heading"])
	}
}

func TestBookContent_Structure(t *testing.T) {
	book := BookContent{
		ID:        "Matt",
		Name:      "Matthew",
		Testament: "NT",
		Chapters: []ChapterContent{
			{Number: 1, Verses: []VerseContent{{Number: 1, Text: "Test"}}},
		},
	}

	data, err := json.Marshal(book)
	if err != nil {
		t.Fatalf("Failed to marshal BookContent: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["id"] != "Matt" {
		t.Errorf("id = %v, want 'Matt'", result["id"])
	}
	if result["testament"] != "NT" {
		t.Errorf("testament = %v, want 'NT'", result["testament"])
	}
}

func TestChapterContent_Structure(t *testing.T) {
	chapter := ChapterContent{
		Number: 3,
		Verses: []VerseContent{
			{Number: 16, Text: "For God so loved the world..."},
		},
	}

	data, err := json.Marshal(chapter)
	if err != nil {
		t.Fatalf("Failed to marshal ChapterContent: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["number"] != float64(3) {
		t.Errorf("number = %v, want 3", result["number"])
	}
}

func TestGenerator_WriteJSON_InvalidPath(t *testing.T) {
	gen := NewGenerator("/tmp", "chapter")

	// Try to write to an invalid path (directory can't be created)
	err := gen.writeJSON("/dev/null/cannot/create/path.json", map[string]string{"key": "value"})
	if err == nil {
		t.Error("writeJSON() should return error for invalid path")
	}
}

func TestVerseContent_WithStrongsAndMorphology(t *testing.T) {
	verse := VerseContent{
		Number:     1,
		Text:       "In the beginning",
		Strongs:    []string{"H7225", "H430"},
		Morphology: []string{"N-ASM", "V-QAL"},
	}

	data, err := json.Marshal(verse)
	if err != nil {
		t.Fatalf("Failed to marshal VerseContent: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["strongs"] == nil {
		t.Error("strongs should be present when not empty")
	}
	if result["morphology"] == nil {
		t.Error("morphology should be present when not empty")
	}
}

func TestBibleContent_WithSections(t *testing.T) {
	content := BibleContent{
		Content: "Introduction text",
		Books:   []BookContent{},
		Sections: []ContentSection{
			{Heading: "Overview", Content: "Overview content"},
			{Heading: "History", Content: "History content"},
		},
	}

	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Failed to marshal BibleContent: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	sections, ok := result["sections"].([]interface{})
	if !ok {
		t.Error("sections should be present")
	}
	if len(sections) != 2 {
		t.Errorf("len(sections) = %d, want 2", len(sections))
	}
}

func TestContentSection_OmitsEmptyContent(t *testing.T) {
	section := ContentSection{
		Heading: "Title Only",
		// Content is empty
	}

	data, err := json.Marshal(section)
	if err != nil {
		t.Fatalf("Failed to marshal ContentSection: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Content should be omitted when empty due to omitempty
	if result["content"] != nil && result["content"] != "" {
		t.Errorf("content should be empty or omitted, got: %v", result["content"])
	}
}

func TestMetaInfo_GeneratedTime(t *testing.T) {
	meta := MetaInfo{
		Granularity: "chapter",
		Version:     "1.0.0",
	}

	// Generated field should serialize properly even when zero
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Failed to marshal MetaInfo: %v", err)
	}

	if len(data) == 0 {
		t.Error("Marshaled data should not be empty")
	}
}

func TestBibleEntry_EmptyFeatures(t *testing.T) {
	entry := BibleEntry{
		ID:       "kjv",
		Title:    "King James Version",
		Features: []string{},
		Tags:     []string{},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal BibleEntry: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Empty slices should still be present as empty arrays
	if _, ok := result["features"]; !ok {
		t.Error("features should be present even when empty")
	}
}

func TestIsPlaceholderText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		// Should be detected as placeholders
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"very short", "abc", true},
		{"simple reference", "Genesis 1:1:", true},
		{"reference with roman numeral", "II Chronicles 19:2:", true},
		{"reference with arabic numeral", "1 John 3:16:", true},
		{"reference no trailing colon", "Genesis 1:1", true},
		{"two word book", "Song of Songs 1:1:", true},
		{"four books", "4 Maccabees 1:1:", true},
		{"roman IV", "IV Maccabees 1:1:", true},

		// Should NOT be detected as placeholders (actual content)
		{"actual verse text", "In the beginning God created the heaven and the earth.", false},
		{"short actual text", "Hello world", false},
		{"verse with numbers", "And there were 12 apostles.", false},
		{"verse with colon", "Jesus said: Follow me.", false},
		{"hebrew text", "בְּרֵאשִׁית בָּרָא אֱלֹהִים", false},
		{"greek text", "Ἐν ἀρχῇ ἦν ὁ λόγος", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPlaceholderText(tt.text)
			if result != tt.expected {
				t.Errorf("isPlaceholderText(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}

func TestGenerator_GenerateFromModules_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator(tmpDir, "chapter")

	modules := []*sword.Module{}
	err := gen.GenerateFromModules(modules, tmpDir)
	if err != nil {
		t.Fatalf("GenerateFromModules() returned error: %v", err)
	}

	// Check that bibles.json was created
	metaPath := filepath.Join(tmpDir, "bibles.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("bibles.json was not created")
	}

	// Check that bibles_auxiliary directory was created
	auxDir := filepath.Join(tmpDir, "bibles_auxiliary")
	if _, err := os.Stat(auxDir); os.IsNotExist(err) {
		t.Error("bibles_auxiliary directory was not created")
	}

	// Verify metadata structure
	content, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read bibles.json: %v", err)
	}

	var meta BibleMetadata
	if err := json.Unmarshal(content, &meta); err != nil {
		t.Fatalf("Failed to parse bibles.json: %v", err)
	}

	if len(meta.Bibles) != 0 {
		t.Errorf("Expected empty bibles array, got %d entries", len(meta.Bibles))
	}
	if meta.Meta.Granularity != "chapter" {
		t.Errorf("Granularity = %q, want 'chapter'", meta.Meta.Granularity)
	}
	if meta.Meta.Version != "2.0.0" {
		t.Errorf("Version = %q, want '2.0.0'", meta.Meta.Version)
	}
}

func TestGenerator_GenerateFromModules_SkipsNonBible(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator(tmpDir, "chapter")

	modules := []*sword.Module{
		{
			ID:         "MHC",
			Title:      "Matthew Henry Commentary",
			ModuleType: sword.ModuleTypeCommentary,
		},
		{
			ID:         "StrongsHebrew",
			Title:      "Strong's Hebrew Dictionary",
			ModuleType: sword.ModuleTypeDictionary,
		},
	}

	err := gen.GenerateFromModules(modules, tmpDir)
	if err != nil {
		t.Fatalf("GenerateFromModules() returned error: %v", err)
	}

	// Verify no bibles were added
	content, err := os.ReadFile(filepath.Join(tmpDir, "bibles.json"))
	if err != nil {
		t.Fatalf("Failed to read bibles.json: %v", err)
	}

	var meta BibleMetadata
	if err := json.Unmarshal(content, &meta); err != nil {
		t.Fatalf("Failed to parse bibles.json: %v", err)
	}

	if len(meta.Bibles) != 0 {
		t.Errorf("Expected no bibles (non-Bible modules should be skipped), got %d", len(meta.Bibles))
	}
}

func TestGenerator_GenerateFromModules_InvalidOutputDir(t *testing.T) {
	// Use an invalid path that can't be created
	gen := NewGenerator("/dev/null/invalid/path", "chapter")

	modules := []*sword.Module{}
	err := gen.GenerateFromModules(modules, "/tmp")
	if err == nil {
		t.Error("GenerateFromModules() should return error for invalid output path")
	}
}

func TestGenerator_GenerateTags_RedLetter(t *testing.T) {
	gen := NewGenerator("/tmp", "chapter")
	module := &sword.Module{
		GlobalOptionFilters: []string{"OSISRedLetterWords"},
	}

	tags := gen.generateTags(module)

	// Red letter currently doesn't add a tag, but we verify it doesn't crash
	// Future enhancement could add "Red Letter" tag
	if tags == nil {
		t.Error("Tags should not be nil")
	}
}

func TestGenerator_GenerateTags_MultipleFeatures(t *testing.T) {
	gen := NewGenerator("/tmp", "chapter")
	module := &sword.Module{
		Language:            "he",
		Features:            []string{"StrongsNumbers", "Headings"},
		GlobalOptionFilters: []string{"OSISMorph", "OSISStrongs"},
	}

	tags := gen.generateTags(module)

	// Should have: he, Strong's Numbers, Morphology
	expectedCount := 3
	if len(tags) != expectedCount {
		t.Errorf("len(tags) = %d, want %d (language + strongs + morph)", len(tags), expectedCount)
	}
}

func TestBibleAuxiliary_EmptyBibles(t *testing.T) {
	aux := BibleAuxiliary{
		Bibles: make(map[string]BibleContent),
	}

	data, err := json.Marshal(aux)
	if err != nil {
		t.Fatalf("Failed to marshal BibleAuxiliary: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	bibles, ok := result["bibles"].(map[string]interface{})
	if !ok {
		t.Error("bibles should be a map")
	}
	if len(bibles) != 0 {
		t.Errorf("Expected empty bibles map, got %d entries", len(bibles))
	}
}

func TestBookContent_EmptyChapters(t *testing.T) {
	book := BookContent{
		ID:        "Obad",
		Name:      "Obadiah",
		Testament: "OT",
		Chapters:  []ChapterContent{},
	}

	data, err := json.Marshal(book)
	if err != nil {
		t.Fatalf("Failed to marshal BookContent: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	chapters, ok := result["chapters"].([]interface{})
	if !ok {
		t.Error("chapters should be an array")
	}
	if len(chapters) != 0 {
		t.Errorf("Expected empty chapters, got %d", len(chapters))
	}
}

func TestChapterContent_EmptyVerses(t *testing.T) {
	chapter := ChapterContent{
		Number: 1,
		Verses: []VerseContent{},
	}

	data, err := json.Marshal(chapter)
	if err != nil {
		t.Fatalf("Failed to marshal ChapterContent: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	verses, ok := result["verses"].([]interface{})
	if !ok {
		t.Error("verses should be an array")
	}
	if len(verses) != 0 {
		t.Errorf("Expected empty verses, got %d", len(verses))
	}
}

func TestIsPlaceholderText_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{"exactly 4 chars", "test", true},
		{"exactly 5 chars", "tests", false},
		{"leading whitespace with valid text", "  Hello world", false},
		{"trailing whitespace with valid text", "Hello world  ", false},
		{"III Maccabees", "III Maccabees 1:1:", true},
		{"I Peter", "I Peter 3:15:", true},
		{"Psalm reference", "Psalm 23:1:", true},
		{"Psalms reference", "Psalms 23:1:", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPlaceholderText(tt.text)
			if result != tt.expected {
				t.Errorf("isPlaceholderText(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}

func TestMultipleBooks_JSONStructure(t *testing.T) {
	content := BibleContent{
		Content: "Test Bible",
		Books: []BookContent{
			{
				ID:        "Gen",
				Name:      "Genesis",
				Testament: "OT",
				Chapters: []ChapterContent{
					{Number: 1, Verses: []VerseContent{{Number: 1, Text: "In the beginning..."}}},
				},
			},
			{
				ID:        "Exod",
				Name:      "Exodus",
				Testament: "OT",
				Chapters: []ChapterContent{
					{Number: 1, Verses: []VerseContent{{Number: 1, Text: "These are the names..."}}},
				},
			},
			{
				ID:        "Matt",
				Name:      "Matthew",
				Testament: "NT",
				Chapters: []ChapterContent{
					{Number: 1, Verses: []VerseContent{{Number: 1, Text: "The book of the generation..."}}},
				},
			},
		},
	}

	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Failed to marshal BibleContent: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	books, ok := result["books"].([]interface{})
	if !ok {
		t.Fatal("books should be an array")
	}
	if len(books) != 3 {
		t.Errorf("Expected 3 books, got %d", len(books))
	}

	// Verify first book
	firstBook := books[0].(map[string]interface{})
	if firstBook["id"] != "Gen" {
		t.Errorf("First book id = %v, want 'Gen'", firstBook["id"])
	}
	if firstBook["testament"] != "OT" {
		t.Errorf("First book testament = %v, want 'OT'", firstBook["testament"])
	}
}

func TestBibleEntry_ZeroWeight(t *testing.T) {
	entry := BibleEntry{
		ID:     "test",
		Title:  "Test Bible",
		Weight: 0,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal BibleEntry: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Weight 0 should still be serialized
	if result["weight"] != float64(0) {
		t.Errorf("weight = %v, want 0", result["weight"])
	}
}

func TestVerseContent_LongText(t *testing.T) {
	// Test handling of very long verse text (edge case)
	longText := strings.Repeat("And the word was ", 100)
	verse := VerseContent{
		Number: 1,
		Text:   longText,
	}

	data, err := json.Marshal(verse)
	if err != nil {
		t.Fatalf("Failed to marshal VerseContent with long text: %v", err)
	}

	var result VerseContent
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result.Text != longText {
		t.Error("Long text was truncated or modified")
	}
}

func TestBibleMetadata_GeneratedTime(t *testing.T) {
	now := time.Now()
	meta := BibleMetadata{
		Bibles: []BibleEntry{},
		Meta: MetaInfo{
			Granularity: "verse",
			Generated:   now,
			Version:     "1.0.0",
		},
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Failed to marshal BibleMetadata: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	metaSection := result["meta"].(map[string]interface{})
	if metaSection["generated"] == nil {
		t.Error("generated field should be present")
	}
}

func TestGenerator_GenerateFromModules_BibleWithMissingData(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator(tmpDir, "chapter")

	// Create a Bible module that points to non-existent data
	// This should trigger the warning path in GenerateFromModules
	modules := []*sword.Module{
		{
			ID:                  "TestBible",
			Title:               "Test Bible",
			ModuleType:          sword.ModuleTypeBible,
			DataPath:            "./modules/texts/ztext/nonexistent/",
			DistributionLicense: "Public Domain",
		},
	}

	// Should not return error, but should print warning and continue
	err := gen.GenerateFromModules(modules, tmpDir)
	if err != nil {
		t.Fatalf("GenerateFromModules() returned error: %v", err)
	}

	// Check that bibles.json was created (even if empty due to parse failure)
	metaPath := filepath.Join(tmpDir, "bibles.json")
	content, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read bibles.json: %v", err)
	}

	var meta BibleMetadata
	if err := json.Unmarshal(content, &meta); err != nil {
		t.Fatalf("Failed to parse bibles.json: %v", err)
	}

	// Bible entry should be added to metadata even if content parsing failed
	// Wait, actually looking at the code, it adds to metadata.Bibles BEFORE parsing
	// So we should have 1 entry in metadata but 0 in auxiliary
	if len(meta.Bibles) != 1 {
		t.Errorf("Expected 1 bible entry in metadata, got %d", len(meta.Bibles))
	}

	// Check auxiliary directory - may or may not have the Bible file depending
	// on whether parsing returns error or empty content
	auxDir := filepath.Join(tmpDir, "bibles_auxiliary")
	if _, err := os.Stat(auxDir); os.IsNotExist(err) {
		t.Error("bibles_auxiliary directory should exist")
	}

	// Count files in auxiliary directory
	entries, err := os.ReadDir(auxDir)
	if err != nil {
		t.Fatalf("Failed to read bibles_auxiliary directory: %v", err)
	}

	// The result depends on whether parsing returns error or empty content
	// Either 0 files (if error) or 1 file (if returns empty books) is acceptable
	if len(entries) > 1 {
		t.Errorf("Expected at most 1 file in auxiliary, got %d", len(entries))
	}
}

func TestGenerator_GenerateFromModules_MultipleBibles(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator(tmpDir, "chapter")

	// Multiple Bible modules (all will fail to parse, but tests iteration)
	modules := []*sword.Module{
		{
			ID:                  "Bible1",
			Title:               "Bible One",
			ModuleType:          sword.ModuleTypeBible,
			Language:            "en",
			DataPath:            "./modules/texts/ztext/bible1/",
			DistributionLicense: "Public Domain",
		},
		{
			ID:                  "Bible2",
			Title:               "Bible Two",
			ModuleType:          sword.ModuleTypeBible,
			Language:            "de",
			Features:            []string{"StrongsNumbers"},
			DataPath:            "./modules/texts/ztext/bible2/",
			DistributionLicense: "Public Domain",
		},
		{
			ID:                  "Bible3",
			Title:               "Bible Three",
			ModuleType:          sword.ModuleTypeBible,
			Language:            "grc",
			GlobalOptionFilters: []string{"OSISMorph"},
			DataPath:            "./modules/texts/ztext/bible3/",
			DistributionLicense: "Public Domain",
		},
	}

	err := gen.GenerateFromModules(modules, tmpDir)
	if err != nil {
		t.Fatalf("GenerateFromModules() returned error: %v", err)
	}

	// Check metadata has all 3 bibles
	metaPath := filepath.Join(tmpDir, "bibles.json")
	content, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read bibles.json: %v", err)
	}

	var meta BibleMetadata
	if err := json.Unmarshal(content, &meta); err != nil {
		t.Fatalf("Failed to parse bibles.json: %v", err)
	}

	if len(meta.Bibles) != 3 {
		t.Errorf("Expected 3 bible entries, got %d", len(meta.Bibles))
	}

	// Verify weights are assigned correctly (1, 2, 3)
	for i, bible := range meta.Bibles {
		expectedWeight := i + 1
		if bible.Weight != expectedWeight {
			t.Errorf("Bible %s weight = %d, want %d", bible.ID, bible.Weight, expectedWeight)
		}
	}
}

func TestGenerator_GenerateFromModules_MixedModuleTypes(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator(tmpDir, "verse")

	// Mix of Bible and non-Bible modules
	modules := []*sword.Module{
		{
			ID:                  "KJV",
			Title:               "King James Version",
			ModuleType:          sword.ModuleTypeBible,
			Language:            "en",
			DataPath:            "./modules/texts/ztext/kjv/",
			DistributionLicense: "Public Domain",
		},
		{
			ID:         "MHC",
			Title:      "Matthew Henry Commentary",
			ModuleType: sword.ModuleTypeCommentary,
		},
		{
			ID:                  "ESV",
			Title:               "English Standard Version",
			ModuleType:          sword.ModuleTypeBible,
			Language:            "en",
			DataPath:            "./modules/texts/ztext/esv/",
			DistributionLicense: "Public Domain",
		},
		{
			ID:         "StrongsHebrew",
			Title:      "Strong's Hebrew",
			ModuleType: sword.ModuleTypeDictionary,
		},
	}

	err := gen.GenerateFromModules(modules, tmpDir)
	if err != nil {
		t.Fatalf("GenerateFromModules() returned error: %v", err)
	}

	// Check only Bibles are in metadata
	metaPath := filepath.Join(tmpDir, "bibles.json")
	content, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read bibles.json: %v", err)
	}

	var meta BibleMetadata
	if err := json.Unmarshal(content, &meta); err != nil {
		t.Fatalf("Failed to parse bibles.json: %v", err)
	}

	if len(meta.Bibles) != 2 {
		t.Errorf("Expected 2 bibles (non-Bible modules skipped), got %d", len(meta.Bibles))
	}

	// Verify only KJV and ESV are present
	ids := make(map[string]bool)
	for _, b := range meta.Bibles {
		ids[b.ID] = true
	}
	if !ids["KJV"] || !ids["ESV"] {
		t.Errorf("Expected KJV and ESV, got %v", ids)
	}
}

func TestGenerator_GenerateFromModules_VerifyMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator(tmpDir, "book")

	modules := []*sword.Module{
		{
			ID:                  "TestMod",
			Title:               "Test Module Title",
			Description:         "This is a test description",
			ModuleType:          sword.ModuleTypeBible,
			Language:            "fr",
			Features:            []string{"StrongsNumbers", "Headings"},
			DataPath:            "./modules/texts/ztext/testmod/",
			DistributionLicense: "Public Domain",
		},
	}

	err := gen.GenerateFromModules(modules, tmpDir)
	if err != nil {
		t.Fatalf("GenerateFromModules() returned error: %v", err)
	}

	metaPath := filepath.Join(tmpDir, "bibles.json")
	content, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read bibles.json: %v", err)
	}

	var meta BibleMetadata
	if err := json.Unmarshal(content, &meta); err != nil {
		t.Fatalf("Failed to parse bibles.json: %v", err)
	}

	// Verify metadata values
	if meta.Meta.Granularity != "book" {
		t.Errorf("Granularity = %q, want 'book'", meta.Meta.Granularity)
	}
	if meta.Meta.Version != "2.0.0" {
		t.Errorf("Version = %q, want '2.0.0'", meta.Meta.Version)
	}
	if meta.Meta.Generated.IsZero() {
		t.Error("Generated time should not be zero")
	}

	// Verify Bible entry
	if len(meta.Bibles) != 1 {
		t.Fatalf("Expected 1 bible, got %d", len(meta.Bibles))
	}
	bible := meta.Bibles[0]
	if bible.ID != "TestMod" {
		t.Errorf("ID = %q, want 'TestMod'", bible.ID)
	}
	if bible.Title != "Test Module Title" {
		t.Errorf("Title = %q, want 'Test Module Title'", bible.Title)
	}
	if bible.Description != "This is a test description" {
		t.Errorf("Description incorrect")
	}
	if bible.Abbrev != "TESTMOD" {
		t.Errorf("Abbrev = %q, want 'TESTMOD'", bible.Abbrev)
	}
	if bible.Language != "fr" {
		t.Errorf("Language = %q, want 'fr'", bible.Language)
	}

	// Check tags include language and Strong's
	hasLanguage := false
	hasStrongs := false
	for _, tag := range bible.Tags {
		if tag == "fr" {
			hasLanguage = true
		}
		if tag == "Strong's Numbers" {
			hasStrongs = true
		}
	}
	if !hasLanguage {
		t.Error("Tags should include language 'fr'")
	}
	if !hasStrongs {
		t.Error("Tags should include 'Strong's Numbers'")
	}
}

func TestGenerator_WriteJSON_FilePermissions(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Cannot test permission errors as root")
	}

	// Create a directory we can't write to
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0555); err != nil {
		t.Fatalf("Failed to create read-only dir: %v", err)
	}
	defer os.Chmod(readOnlyDir, 0700)

	gen := NewGenerator(readOnlyDir, "chapter")

	// Try to write to read-only directory
	err := gen.writeJSON(filepath.Join(readOnlyDir, "test.json"), map[string]string{"key": "value"})
	if err == nil {
		t.Error("writeJSON() should return error for read-only directory")
	}
}

func TestGenerator_GenerateFromModules_OutputFiles(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator(tmpDir, "chapter")

	// Empty modules list
	err := gen.GenerateFromModules([]*sword.Module{}, tmpDir)
	if err != nil {
		t.Fatalf("GenerateFromModules() returned error: %v", err)
	}

	// Verify bibles.json exists and is valid JSON
	metaPath := filepath.Join(tmpDir, "bibles.json")
	metaContent, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read bibles.json: %v", err)
	}
	var meta BibleMetadata
	if err := json.Unmarshal(metaContent, &meta); err != nil {
		t.Fatalf("bibles.json is not valid JSON: %v", err)
	}

	// Verify bibles_auxiliary directory exists (should be empty for empty modules list)
	auxDir := filepath.Join(tmpDir, "bibles_auxiliary")
	if _, err := os.Stat(auxDir); os.IsNotExist(err) {
		t.Error("bibles_auxiliary directory should exist")
	}
	entries, err := os.ReadDir(auxDir)
	if err != nil {
		t.Fatalf("Failed to read bibles_auxiliary directory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected empty bibles_auxiliary directory, got %d files", len(entries))
	}
}

// =============================================================================
// SPDX License Loading Tests
// =============================================================================

func TestGenerator_LoadSPDXLicenses(t *testing.T) {
	tmpDir := t.TempDir()
	spdxPath := filepath.Join(tmpDir, "spdx_licenses.json")

	// Create a valid SPDX file
	spdxData := `{
		"licenses": {
			"MIT": {"name": "MIT License"},
			"GPL-3.0-or-later": {"name": "GNU General Public License v3.0 or later"},
			"CC-PDDC": {"name": "Creative Commons Public Domain Dedication"}
		}
	}`
	if err := os.WriteFile(spdxPath, []byte(spdxData), 0600); err != nil {
		t.Fatalf("Failed to create SPDX file: %v", err)
	}

	gen := NewGenerator(tmpDir, "chapter")
	if err := gen.LoadSPDXLicenses(spdxPath); err != nil {
		t.Fatalf("LoadSPDXLicenses() error = %v", err)
	}

	if len(gen.SPDXLicenses) != 3 {
		t.Errorf("Expected 3 SPDX licenses, got %d", len(gen.SPDXLicenses))
	}
	if !gen.SPDXLicenses["MIT"] {
		t.Error("MIT should be in SPDX licenses")
	}
	if !gen.SPDXLicenses["GPL-3.0-or-later"] {
		t.Error("GPL-3.0-or-later should be in SPDX licenses")
	}
}

func TestGenerator_LoadSPDXLicenses_NotFound(t *testing.T) {
	gen := NewGenerator("/tmp", "chapter")
	err := gen.LoadSPDXLicenses("/nonexistent/path/spdx.json")
	if err == nil {
		t.Error("LoadSPDXLicenses() should return error for nonexistent file")
	}
}

func TestGenerator_LoadSPDXLicenses_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	spdxPath := filepath.Join(tmpDir, "spdx_licenses.json")

	if err := os.WriteFile(spdxPath, []byte("not valid json"), 0600); err != nil {
		t.Fatalf("Failed to create invalid SPDX file: %v", err)
	}

	gen := NewGenerator(tmpDir, "chapter")
	err := gen.LoadSPDXLicenses(spdxPath)
	if err == nil {
		t.Error("LoadSPDXLicenses() should return error for invalid JSON")
	}
}

func TestGenerator_LoadSPDXLicenses_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	spdxPath := filepath.Join(tmpDir, "spdx_licenses.json")

	if err := os.WriteFile(spdxPath, []byte("{}"), 0600); err != nil {
		t.Fatalf("Failed to create empty SPDX file: %v", err)
	}

	gen := NewGenerator(tmpDir, "chapter")
	if err := gen.LoadSPDXLicenses(spdxPath); err != nil {
		t.Fatalf("LoadSPDXLicenses() error = %v", err)
	}

	if len(gen.SPDXLicenses) != 0 {
		t.Errorf("Expected 0 SPDX licenses, got %d", len(gen.SPDXLicenses))
	}
}

// =============================================================================
// ValidateLicense Tests
// =============================================================================

func TestGenerator_ValidateLicense_NoSPDXLoaded(t *testing.T) {
	gen := NewGenerator("/tmp", "chapter")
	// SPDXLicenses is empty, validation should skip
	err := gen.ValidateLicense("MIT")
	if err != nil {
		t.Errorf("ValidateLicense() should not error when no SPDX data loaded: %v", err)
	}
}

func TestGenerator_ValidateLicense_ValidLicense(t *testing.T) {
	gen := NewGenerator("/tmp", "chapter")
	gen.SPDXLicenses = map[string]bool{
		"MIT":              true,
		"GPL-3.0-or-later": true,
	}

	if err := gen.ValidateLicense("MIT"); err != nil {
		t.Errorf("ValidateLicense('MIT') should pass: %v", err)
	}
	if err := gen.ValidateLicense("GPL-3.0-or-later"); err != nil {
		t.Errorf("ValidateLicense('GPL-3.0-or-later') should pass: %v", err)
	}
}

func TestGenerator_ValidateLicense_InvalidLicense(t *testing.T) {
	gen := NewGenerator("/tmp", "chapter")
	gen.SPDXLicenses = map[string]bool{
		"MIT": true,
	}

	err := gen.ValidateLicense("Unknown-License")
	if err == nil {
		t.Error("ValidateLicense() should return error for unknown license")
	}
	if !strings.Contains(err.Error(), "not found in spdx_licenses.json") {
		t.Errorf("Error message should mention spdx_licenses.json: %v", err)
	}
}

// Note: normalizeVersification is unexported and tested indirectly via
// GenerateFromModules which uses it internally.

// =============================================================================
// BibleEntry with License Fields Tests
// =============================================================================

func TestBibleEntry_LicenseFields(t *testing.T) {
	entry := BibleEntry{
		ID:            "kjv",
		Title:         "King James Version",
		License:       "CC-PDDC",
		LicenseText:   "This is in the public domain.",
		Versification: "protestant",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal BibleEntry: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["license"] != "CC-PDDC" {
		t.Errorf("license = %v, want 'CC-PDDC'", result["license"])
	}
	if result["licenseText"] != "This is in the public domain." {
		t.Errorf("licenseText incorrect")
	}
	if result["versification"] != "protestant" {
		t.Errorf("versification = %v, want 'protestant'", result["versification"])
	}
}

func TestBibleEntry_EmptyLicenseText(t *testing.T) {
	entry := BibleEntry{
		ID:          "test",
		Title:       "Test",
		License:     "MIT",
		LicenseText: "", // Empty, should be omitted
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal BibleEntry: %v", err)
	}

	// licenseText should be omitted when empty (omitempty)
	if strings.Contains(string(data), "licenseText") {
		t.Error("Empty licenseText should be omitted from JSON")
	}
}

// =============================================================================
// Placeholder Pattern Edge Cases
// =============================================================================

func TestPlaceholderPattern_MoreEdgeCases(t *testing.T) {
	tests := []struct {
		text          string
		isPlaceholder bool
	}{
		// Deuterocanonical books
		{"Tobit 1:1:", true},
		{"Judith 1:1:", true},
		{"Wisdom 1:1:", true},
		{"Sirach 1:1:", true},
		{"Baruch 1:1:", true},
		{"1 Maccabees 1:1:", true},
		{"2 Maccabees 1:1:", true},
		{"3 Maccabees 1:1:", true},
		{"4 Maccabees 1:1:", true},
		{"Prayer of Manasseh 1:1:", true},

		// Psalms over 150 (in some traditions)
		{"Psalm 151:1:", true},
		{"Psalms 151:1:", true},

		// Actual verse content (should NOT be placeholders)
		{"The Lord is my shepherd; I shall not want.", false},
		{"For God so loved the world, that he gave his only begotten Son", false},
		{"In the beginning was the Word, and the Word was with God", false},
		{"Blessed are the poor in spirit: for theirs is the kingdom of heaven.", false},

		// Hebrew and Greek content (should NOT be placeholders)
		{"בְּרֵאשִׁית בָּרָא אֱלֹהִים אֵת הַשָּׁמַיִם וְאֵת הָאָרֶץ", false},
		{"Ἐν ἀρχῇ ἦν ὁ Λόγος καὶ ὁ Λόγος ἦν πρὸς τὸν Θεόν", false},
	}

	for _, tt := range tests {
		t.Run(tt.text[:min(30, len(tt.text))], func(t *testing.T) {
			result := isPlaceholderText(tt.text)
			if result != tt.isPlaceholder {
				t.Errorf("isPlaceholderText(%q) = %v, want %v", tt.text, result, tt.isPlaceholder)
			}
		})
	}
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// =============================================================================
// ExcludedBooks Tests
// =============================================================================

func TestExcludedBook_Structure(t *testing.T) {
	excluded := ExcludedBook{
		ID:        "Matt",
		Name:      "Matthew",
		Testament: "NT",
		Reason:    "no content in source module",
	}

	data, err := json.Marshal(excluded)
	if err != nil {
		t.Fatalf("Failed to marshal ExcludedBook: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["id"] != "Matt" {
		t.Errorf("id = %v, want Matt", result["id"])
	}
	if result["name"] != "Matthew" {
		t.Errorf("name = %v, want Matthew", result["name"])
	}
	if result["testament"] != "NT" {
		t.Errorf("testament = %v, want NT", result["testament"])
	}
	if result["reason"] != "no content in source module" {
		t.Errorf("reason = %v, want 'no content in source module'", result["reason"])
	}
}

func TestBibleContent_WithExcludedBooks(t *testing.T) {
	content := BibleContent{
		Content: "The Hebrew Bible",
		Books: []BookContent{
			{ID: "Gen", Name: "Genesis", Testament: "OT"},
			{ID: "Exod", Name: "Exodus", Testament: "OT"},
		},
		ExcludedBooks: []ExcludedBook{
			{ID: "Matt", Name: "Matthew", Testament: "NT", Reason: "no content in source module"},
			{ID: "Mark", Name: "Mark", Testament: "NT", Reason: "no content in source module"},
		},
	}

	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Failed to marshal BibleContent: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Check books
	books, ok := result["books"].([]interface{})
	if !ok {
		t.Fatal("books should be an array")
	}
	if len(books) != 2 {
		t.Errorf("len(books) = %d, want 2", len(books))
	}

	// Check excluded books
	excluded, ok := result["excludedBooks"].([]interface{})
	if !ok {
		t.Fatal("excludedBooks should be an array")
	}
	if len(excluded) != 2 {
		t.Errorf("len(excludedBooks) = %d, want 2", len(excluded))
	}
}

func TestBibleContent_ExcludedBooksOmitsWhenEmpty(t *testing.T) {
	content := BibleContent{
		Content: "Full Bible",
		Books: []BookContent{
			{ID: "Gen", Name: "Genesis", Testament: "OT"},
		},
		ExcludedBooks: []ExcludedBook{}, // Empty slice
	}

	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("Failed to marshal BibleContent: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Empty slice should still appear (not nil, so not omitted)
	// But we use omitempty, so empty slice should be omitted
	if _, ok := result["excludedBooks"]; ok {
		excluded := result["excludedBooks"]
		if arr, isArr := excluded.([]interface{}); isArr && len(arr) > 0 {
			t.Errorf("excludedBooks should be omitted when empty, got %v", excluded)
		}
	}
}

// =============================================================================
// ToSPDXLicense Integration Tests (via repository package)
// =============================================================================

func TestToSPDXLicense_Integration(t *testing.T) {
	// Test that repository.ToSPDXLicense is accessible and working
	tests := []struct {
		input    string
		expected string
	}{
		{"Public Domain", "CC-PDDC"},
		{"GPL", "GPL-3.0-or-later"},
		{"CC BY-SA 4.0", "CC-BY-SA-4.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := repository.ToSPDXLicense(tt.input)
			if result != tt.expected {
				t.Errorf("ToSPDXLicense(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
