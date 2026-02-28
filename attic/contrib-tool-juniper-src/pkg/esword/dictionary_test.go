package esword

import (
	"strings"
	"testing"
)

func TestNewDictionaryParser_Valid(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "Test", Definition: "A test entry"},
	}, &DictionaryMetadata{
		Title:        "Test Dictionary",
		Abbreviation: "TD",
		Version:      "1.0",
	})

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	if parser.db == nil {
		t.Error("parser.db is nil")
	}
	if parser.metadata == nil {
		t.Error("parser.metadata is nil")
	}
}

func TestNewDictionaryParser_InvalidPath(t *testing.T) {
	// SQLite may create file, so test with empty database instead
	dbPath := CreateEmptyDB(t)

	parser, err := NewDictionaryParser(dbPath)
	if err == nil {
		defer parser.Close()
		// Parser opened but GetEntry should fail
		_, entryErr := parser.GetEntry("test")
		if entryErr == nil {
			t.Error("GetEntry should fail on empty database")
		}
	}
}

func TestDictionaryParser_Close(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "Test", Definition: "Definition"},
	}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}

	err = parser.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestDictionaryParser_GetMetadata(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "Test", Definition: "Definition"},
	}, &DictionaryMetadata{
		Title:        "Strong's Greek Dictionary",
		Abbreviation: "SG",
		Information:  "Greek lexicon",
		Version:      "3.0",
	})

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	metadata := parser.GetMetadata()
	if metadata.Title != "Strong's Greek Dictionary" {
		t.Errorf("Title = %q, want %q", metadata.Title, "Strong's Greek Dictionary")
	}
	if metadata.Abbreviation != "SG" {
		t.Errorf("Abbreviation = %q, want %q", metadata.Abbreviation, "SG")
	}
}

func TestDictionaryParser_GetEntry_Exact(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "logos", Definition: "word, speech, reason"},
	}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	entry, err := parser.GetEntry("logos")
	if err != nil {
		t.Fatalf("GetEntry() returned error: %v", err)
	}

	if entry.Topic != "logos" {
		t.Errorf("Topic = %q, want %q", entry.Topic, "logos")
	}
	if entry.Definition != "word, speech, reason" {
		t.Errorf("Definition = %q, want expected text", entry.Definition)
	}
}

func TestDictionaryParser_GetEntry_CaseInsensitive(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "Logos", Definition: "word"},
	}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	// Should find regardless of case
	entry, err := parser.GetEntry("logos")
	if err != nil {
		t.Fatalf("GetEntry(lowercase) returned error: %v", err)
	}
	if entry.Topic != "Logos" {
		t.Errorf("Topic = %q, want %q", entry.Topic, "Logos")
	}

	entry, err = parser.GetEntry("LOGOS")
	if err != nil {
		t.Fatalf("GetEntry(uppercase) returned error: %v", err)
	}
	if entry.Topic != "Logos" {
		t.Errorf("Topic = %q, want %q", entry.Topic, "Logos")
	}
}

func TestDictionaryParser_GetEntry_NotFound(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "exists", Definition: "yes"},
	}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	_, err = parser.GetEntry("nonexistent")
	if err == nil {
		t.Error("GetEntry() should return error for non-existent topic")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should mention 'not found', got: %v", err)
	}
}

func TestDictionaryParser_GetAllTopics(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "alpha", Definition: "first"},
		{Topic: "beta", Definition: "second"},
		{Topic: "gamma", Definition: "third"},
	}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	topics, err := parser.GetAllTopics()
	if err != nil {
		t.Fatalf("GetAllTopics() returned error: %v", err)
	}

	if len(topics) != 3 {
		t.Errorf("len(topics) = %d, want 3", len(topics))
	}

	// Should be ordered
	if topics[0] != "alpha" {
		t.Errorf("First topic = %q, want %q", topics[0], "alpha")
	}
}

func TestDictionaryParser_GetAllEntries(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "A", Definition: "def A"},
		{Topic: "B", Definition: "def B"},
	}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	entries, err := parser.GetAllEntries()
	if err != nil {
		t.Fatalf("GetAllEntries() returned error: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("len(entries) = %d, want 2", len(entries))
	}
}

func TestDictionaryParser_SearchTopics(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "abraham", Definition: "patriarch"},
		{Topic: "abimelech", Definition: "king"},
		{Topic: "moses", Definition: "prophet"},
	}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	topics, err := parser.SearchTopics("ab")
	if err != nil {
		t.Fatalf("SearchTopics() returned error: %v", err)
	}

	if len(topics) != 2 {
		t.Errorf("len(topics) = %d, want 2 (abraham, abimelech)", len(topics))
	}
}

func TestDictionaryParser_GetTopicsByLetter(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "Alpha", Definition: "a"},
		{Topic: "Ark", Definition: "b"},
		{Topic: "Beta", Definition: "c"},
	}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	topics, err := parser.GetTopicsByLetter("A")
	if err != nil {
		t.Fatalf("GetTopicsByLetter() returned error: %v", err)
	}

	if len(topics) != 2 {
		t.Errorf("len(topics) = %d, want 2", len(topics))
	}
}

func TestDictionaryParser_GetLetterIndex(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "Alpha", Definition: "a"},
		{Topic: "Beta", Definition: "b"},
		{Topic: "Delta", Definition: "d"},
	}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	letters, err := parser.GetLetterIndex()
	if err != nil {
		t.Fatalf("GetLetterIndex() returned error: %v", err)
	}

	if len(letters) != 3 {
		t.Errorf("len(letters) = %d, want 3", len(letters))
	}

	// Should be sorted
	expected := []string{"A", "B", "D"}
	for i, letter := range letters {
		if letter != expected[i] {
			t.Errorf("letters[%d] = %q, want %q", i, letter, expected[i])
		}
	}
}

func TestDictionaryParser_GetEntryCount(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "A", Definition: "a"},
		{Topic: "B", Definition: "b"},
		{Topic: "C", Definition: "c"},
		{Topic: "D", Definition: "d"},
	}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	count, err := parser.GetEntryCount()
	if err != nil {
		t.Fatalf("GetEntryCount() returned error: %v", err)
	}

	if count != 4 {
		t.Errorf("GetEntryCount() = %d, want 4", count)
	}
}

func TestDictionaryParser_IsStrongsLexicon(t *testing.T) {
	tests := []struct {
		title    string
		expected bool
	}{
		{"Strong's Greek Dictionary", true},
		{"Strong's Hebrew Dictionary", true},
		{"Strongs Greek", true},
		{"strongs hebrew", true},
		{"Regular Dictionary", false},
		{"Nave's Topical Bible", false},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
				{Topic: "Test", Definition: "test"},
			}, &DictionaryMetadata{Title: tt.title})

			parser, err := NewDictionaryParser(dbPath)
			if err != nil {
				t.Fatalf("NewDictionaryParser() returned error: %v", err)
			}
			defer parser.Close()

			result := parser.IsStrongsLexicon()
			if result != tt.expected {
				t.Errorf("IsStrongsLexicon() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDictionaryParser_GetStrongsEntry_Exact(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "G2316", Definition: "theos - God"},
		{Topic: "H430", Definition: "elohim - God"},
	}, &DictionaryMetadata{Title: "Strong's Dictionary"})

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	entry, err := parser.GetStrongsEntry("G2316")
	if err != nil {
		t.Fatalf("GetStrongsEntry() returned error: %v", err)
	}

	if entry.Topic != "G2316" {
		t.Errorf("Topic = %q, want %q", entry.Topic, "G2316")
	}
}

func TestDictionaryParser_GetStrongsEntry_WithoutPrefix(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "2316", Definition: "theos - God"},
	}, &DictionaryMetadata{Title: "Strong's Dictionary"})

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	// Request with prefix, should find without prefix
	entry, err := parser.GetStrongsEntry("G2316")
	if err != nil {
		t.Fatalf("GetStrongsEntry() returned error: %v", err)
	}

	if entry.Topic != "2316" {
		t.Errorf("Topic = %q, want %q", entry.Topic, "2316")
	}
}

func TestDictionaryParser_GetStrongsEntry_LeadingZeros(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "H00430", Definition: "elohim - God"},
	}, &DictionaryMetadata{Title: "Strong's Dictionary"})

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	// Request without zeros, should find with zeros
	entry, err := parser.GetStrongsEntry("H430")
	if err != nil {
		t.Fatalf("GetStrongsEntry() returned error: %v", err)
	}

	if entry.Topic != "H00430" {
		t.Errorf("Topic = %q, want %q", entry.Topic, "H00430")
	}
}

func TestDictionaryParser_GetStrongsEntry_NotFound(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "G2316", Definition: "theos"},
	}, &DictionaryMetadata{Title: "Strong's Dictionary"})

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	_, err = parser.GetStrongsEntry("G9999")
	if err == nil {
		t.Error("GetStrongsEntry() should return error for non-existent entry")
	}
}

func TestDictionaryParser_GetStrongsEntry_Normalization(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "G2316", Definition: "theos"},
	}, &DictionaryMetadata{Title: "Strong's Dictionary"})

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	// Test various input formats
	inputs := []string{"g2316", "G2316", "  G2316  "}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			entry, err := parser.GetStrongsEntry(input)
			if err != nil {
				t.Fatalf("GetStrongsEntry(%q) returned error: %v", input, err)
			}
			if entry.Topic != "G2316" {
				t.Errorf("Topic = %q, want G2316", entry.Topic)
			}
		})
	}
}

func TestDictionaryParser_TextCleaning(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "test", Definition: "\\bBold\\b0 and \\iitalic\\i0"},
	}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	entry, err := parser.GetEntry("test")
	if err != nil {
		t.Fatalf("GetEntry() returned error: %v", err)
	}

	// RTF codes should be cleaned
	if strings.Contains(entry.Definition, "\\b") {
		t.Errorf("Definition still contains RTF codes: %q", entry.Definition)
	}
}

func TestDictionaryParser_Close_NilDB(t *testing.T) {
	// Test Close when db is nil
	parser := &DictionaryParser{db: nil}
	err := parser.Close()
	if err != nil {
		t.Errorf("Close() on nil db should not error, got: %v", err)
	}
}

func TestDictionaryParser_GetAllTopics_Empty(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	topics, err := parser.GetAllTopics()
	if err != nil {
		t.Fatalf("GetAllTopics() returned error: %v", err)
	}

	if len(topics) != 0 {
		t.Errorf("Expected 0 topics for empty database, got %d", len(topics))
	}
}

func TestDictionaryParser_GetAllEntries_Empty(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	entries, err := parser.GetAllEntries()
	if err != nil {
		t.Fatalf("GetAllEntries() returned error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries for empty database, got %d", len(entries))
	}
}

func TestDictionaryParser_SearchTopics_NoMatch(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "alpha", Definition: "first"},
	}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	topics, err := parser.SearchTopics("xyz")
	if err != nil {
		t.Fatalf("SearchTopics() returned error: %v", err)
	}

	if len(topics) != 0 {
		t.Errorf("Expected 0 topics for no match, got %d", len(topics))
	}
}

func TestDictionaryParser_GetTopicsByLetter_NoMatch(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "Alpha", Definition: "a"},
	}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	topics, err := parser.GetTopicsByLetter("Z")
	if err != nil {
		t.Fatalf("GetTopicsByLetter() returned error: %v", err)
	}

	if len(topics) != 0 {
		t.Errorf("Expected 0 topics for letter Z, got %d", len(topics))
	}
}

func TestDictionaryParser_GetLetterIndex_Empty(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{}, nil)

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	letters, err := parser.GetLetterIndex()
	if err != nil {
		t.Fatalf("GetLetterIndex() returned error: %v", err)
	}

	if len(letters) != 0 {
		t.Errorf("Expected 0 letters for empty database, got %d", len(letters))
	}
}

func TestDictionaryParser_GetStrongsEntry_NumericOnly(t *testing.T) {
	dbPath := CreateTestDictionaryDB(t, []TestDictionaryEntry{
		{Topic: "00430", Definition: "elohim - God"},
	}, &DictionaryMetadata{Title: "Strong's Dictionary"})

	parser, err := NewDictionaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewDictionaryParser() returned error: %v", err)
	}
	defer parser.Close()

	// Request with prefix, should find numeric version with leading zeros
	entry, err := parser.GetStrongsEntry("430")
	if err != nil {
		t.Fatalf("GetStrongsEntry() returned error: %v", err)
	}

	if entry.Topic != "00430" {
		t.Errorf("Topic = %q, want %q", entry.Topic, "00430")
	}
}
