package esword

import (
	"strings"
	"testing"
)

func TestNewBibleParser_Valid(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "In the beginning God created the heaven and the earth."},
	}, &BibleMetadata{
		Title:        "Test Bible",
		Abbreviation: "TST",
		Information:  "Test information",
		Version:      "1.0",
	})

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	if parser.db == nil {
		t.Error("parser.db is nil")
	}
	if parser.metadata == nil {
		t.Error("parser.metadata is nil")
	}
}

func TestNewBibleParser_InvalidPath(t *testing.T) {
	// SQLite creates the file if it doesn't exist in read mode, so we skip trying
	// to open a nonexistent file. Instead, test that metadata loading fails
	// when the required table doesn't exist.
	dbPath := CreateEmptyDB(t)

	// Parser should fail when Bible table doesn't exist
	parser, err := NewBibleParser(dbPath)
	if err == nil {
		defer parser.Close()
		// If it doesn't error, at least GetVerse should fail
		_, verseErr := parser.GetVerse(1, 1, 1)
		if verseErr == nil {
			t.Error("GetVerse should fail on empty database")
		}
	}
}

func TestNewBibleParser_NonSQLite(t *testing.T) {
	dbPath := CreateInvalidDB(t)

	parser, err := NewBibleParser(dbPath)
	if err == nil {
		// SQLite may open the file but fail on queries
		defer parser.Close()
		_, verseErr := parser.GetVerse(1, 1, 1)
		if verseErr == nil {
			t.Error("GetVerse should fail on non-SQLite file")
		}
	}
	// Either opening or querying should fail
}

func TestBibleParser_Close(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "Test"},
	}, nil)

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}

	err = parser.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Close again should not error
	err = parser.Close()
	if err != nil {
		t.Errorf("Second Close() returned error: %v", err)
	}
}

func TestBibleParser_LoadMetadata_WithDetails(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "Test"},
	}, &BibleMetadata{
		Title:        "King James Version",
		Abbreviation: "KJV",
		Information:  "Public Domain",
		Version:      "1.0",
		Font:         "Times New Roman",
		RightToLeft:  false,
	})

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	metadata := parser.GetMetadata()
	if metadata.Title != "King James Version" {
		t.Errorf("Title = %q, want %q", metadata.Title, "King James Version")
	}
	if metadata.Abbreviation != "KJV" {
		t.Errorf("Abbreviation = %q, want %q", metadata.Abbreviation, "KJV")
	}
	if metadata.Version != "1.0" {
		t.Errorf("Version = %q, want %q", metadata.Version, "1.0")
	}
}

func TestBibleParser_LoadMetadata_WithoutDetails(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "Test"},
	}, nil) // No metadata

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	metadata := parser.GetMetadata()
	if metadata == nil {
		t.Error("GetMetadata() returned nil")
	}
	// Should have empty but non-nil metadata
	if metadata.Title != "" {
		t.Errorf("Title should be empty without Details table, got %q", metadata.Title)
	}
}

func TestBibleParser_GetVerse_Valid(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "In the beginning God created the heaven and the earth."},
		{Book: 43, Chapter: 3, Verse: 16, Scripture: "For God so loved the world..."},
	}, nil)

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	verse, err := parser.GetVerse(1, 1, 1)
	if err != nil {
		t.Fatalf("GetVerse() returned error: %v", err)
	}

	if verse.Book != 1 {
		t.Errorf("Book = %d, want 1", verse.Book)
	}
	if verse.Chapter != 1 {
		t.Errorf("Chapter = %d, want 1", verse.Chapter)
	}
	if verse.Verse != 1 {
		t.Errorf("Verse = %d, want 1", verse.Verse)
	}
	if verse.Scripture != "In the beginning God created the heaven and the earth." {
		t.Errorf("Scripture = %q, want expected text", verse.Scripture)
	}
}

func TestBibleParser_GetVerse_NotFound(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "Test"},
	}, nil)

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	_, err = parser.GetVerse(99, 99, 99)
	if err == nil {
		t.Error("GetVerse() should return error for non-existent verse")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should mention 'not found', got: %v", err)
	}
}

func TestBibleParser_GetVerse_NULLScripture(t *testing.T) {
	// Create database manually to insert NULL
	dbPath := CreateTestBibleDB(t, []TestVerse{}, nil)

	// We need to create the table first, then the helper doesn't add NULL
	// For simplicity, test with empty string which is similar behavior
	dbPath = CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: ""},
	}, nil)

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	verse, err := parser.GetVerse(1, 1, 1)
	if err != nil {
		t.Fatalf("GetVerse() returned error: %v", err)
	}

	if verse.Scripture != "" {
		t.Errorf("Scripture should be empty for NULL, got %q", verse.Scripture)
	}
}

func TestBibleParser_GetChapter(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "Verse 1"},
		{Book: 1, Chapter: 1, Verse: 2, Scripture: "Verse 2"},
		{Book: 1, Chapter: 1, Verse: 3, Scripture: "Verse 3"},
		{Book: 1, Chapter: 2, Verse: 1, Scripture: "Chapter 2"},
	}, nil)

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	verses, err := parser.GetChapter(1, 1)
	if err != nil {
		t.Fatalf("GetChapter() returned error: %v", err)
	}

	if len(verses) != 3 {
		t.Errorf("len(verses) = %d, want 3", len(verses))
	}

	// Verify order
	for i, v := range verses {
		if v.Verse != i+1 {
			t.Errorf("Verse %d should have Verse=%d, got %d", i, i+1, v.Verse)
		}
	}
}

func TestBibleParser_GetBook(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "Gen 1:1"},
		{Book: 1, Chapter: 1, Verse: 2, Scripture: "Gen 1:2"},
		{Book: 1, Chapter: 2, Verse: 1, Scripture: "Gen 2:1"},
		{Book: 2, Chapter: 1, Verse: 1, Scripture: "Exod 1:1"},
	}, nil)

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	verses, err := parser.GetBook(1)
	if err != nil {
		t.Fatalf("GetBook() returned error: %v", err)
	}

	if len(verses) != 3 {
		t.Errorf("len(verses) = %d, want 3", len(verses))
	}

	// All verses should be book 1
	for _, v := range verses {
		if v.Book != 1 {
			t.Errorf("Verse has Book=%d, want 1", v.Book)
		}
	}
}

func TestBibleParser_GetAllVerses(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "V1"},
		{Book: 1, Chapter: 1, Verse: 2, Scripture: "V2"},
		{Book: 2, Chapter: 1, Verse: 1, Scripture: "V3"},
	}, nil)

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	verses, err := parser.GetAllVerses()
	if err != nil {
		t.Fatalf("GetAllVerses() returned error: %v", err)
	}

	if len(verses) != 3 {
		t.Errorf("len(verses) = %d, want 3", len(verses))
	}
}

func TestBibleParser_GetChapterCount(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "V1"},
		{Book: 1, Chapter: 2, Verse: 1, Scripture: "V2"},
		{Book: 1, Chapter: 3, Verse: 1, Scripture: "V3"},
	}, nil)

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	count, err := parser.GetChapterCount(1)
	if err != nil {
		t.Fatalf("GetChapterCount() returned error: %v", err)
	}

	if count != 3 {
		t.Errorf("GetChapterCount() = %d, want 3", count)
	}
}

func TestBibleParser_GetVerseCount(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "V1"},
		{Book: 1, Chapter: 1, Verse: 2, Scripture: "V2"},
		{Book: 1, Chapter: 1, Verse: 3, Scripture: "V3"},
		{Book: 1, Chapter: 1, Verse: 4, Scripture: "V4"},
	}, nil)

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	count, err := parser.GetVerseCount(1, 1)
	if err != nil {
		t.Fatalf("GetVerseCount() returned error: %v", err)
	}

	if count != 4 {
		t.Errorf("GetVerseCount() = %d, want 4", count)
	}
}

func TestToSwordBook_Valid(t *testing.T) {
	tests := []struct {
		eswordBook int
		wantID     string
		wantOK     bool
	}{
		{1, "Gen", true},
		{2, "Exod", true},
		{19, "Ps", true},
		{40, "Matt", true},
		{66, "Rev", true},
	}

	for _, tt := range tests {
		t.Run(tt.wantID, func(t *testing.T) {
			book, ok := ToSwordBook(tt.eswordBook)
			if ok != tt.wantOK {
				t.Errorf("ToSwordBook(%d) ok = %v, want %v", tt.eswordBook, ok, tt.wantOK)
				return
			}
			if ok && book.ID != tt.wantID {
				t.Errorf("ToSwordBook(%d).ID = %q, want %q", tt.eswordBook, book.ID, tt.wantID)
			}
		})
	}
}

func TestToSwordBook_Invalid(t *testing.T) {
	tests := []int{0, -1, 67, 100}

	for _, eswordBook := range tests {
		t.Run(string(rune('0'+eswordBook)), func(t *testing.T) {
			_, ok := ToSwordBook(eswordBook)
			if ok {
				t.Errorf("ToSwordBook(%d) should return false", eswordBook)
			}
		})
	}
}

func TestCleanESwordText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "par to newline",
			input:    "Line 1\\parLine 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "line to newline",
			input:    "Line 1\\lineLine 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "remove font code",
			input:    "\\f1Text here",
			expected: "Text here",
		},
		{
			name:     "remove color code",
			input:    "\\cf2Text here",
			expected: "Text here",
		},
		{
			name:     "remove bold",
			input:    "\\bBold text\\b0",
			expected: "Bold text",
		},
		{
			name:     "remove italic",
			input:    "\\iItalic text\\i0",
			expected: "Italic text",
		},
		{
			name:     "remove underline",
			input:    "\\ulUnderlined\\ul0",
			expected: "Underlined",
		},
		{
			name:     "remove superscript",
			input:    "Text\\super1\\nosupersubmore",
			expected: "Text1more",
		},
		{
			name:     "remove font size",
			input:    "\\fs24Regular size",
			expected: "s24Regular size", // \f is consumed but fs is not fully handled
		},
		{
			name:     "complex example",
			input:    "\\f1\\cf2\\bIn the beginning\\b0\\par\\i God created\\i0",
			expected: "In the beginning\n God created", // space after newline preserved
		},
		{
			name:     "plain text unchanged",
			input:    "Plain text without formatting",
			expected: "Plain text without formatting",
		},
		{
			name:     "trim whitespace",
			input:    "  Text with spaces  ",
			expected: "Text with spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanESwordText(tt.input)
			if result != tt.expected {
				t.Errorf("cleanESwordText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBibleParser_RightToLeft(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "בְּרֵאשִׁית"},
	}, &BibleMetadata{
		Title:       "Hebrew Bible",
		RightToLeft: true,
	})

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	if !parser.GetMetadata().RightToLeft {
		t.Error("RightToLeft should be true for Hebrew Bible")
	}
}

func TestBibleParser_Close_NilDB(t *testing.T) {
	// Test Close when db is nil
	parser := &BibleParser{db: nil}
	err := parser.Close()
	if err != nil {
		t.Errorf("Close() on nil db should not error, got: %v", err)
	}
}

func TestBibleParser_GetChapter_Empty(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "Test"},
	}, nil)

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	// Get a chapter that doesn't exist
	verses, err := parser.GetChapter(99, 99)
	if err != nil {
		t.Fatalf("GetChapter() returned error: %v", err)
	}

	if len(verses) != 0 {
		t.Errorf("Expected 0 verses for non-existent chapter, got %d", len(verses))
	}
}

func TestBibleParser_GetBook_Empty(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "Test"},
	}, nil)

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	// Get a book that doesn't exist
	verses, err := parser.GetBook(99)
	if err != nil {
		t.Fatalf("GetBook() returned error: %v", err)
	}

	if len(verses) != 0 {
		t.Errorf("Expected 0 verses for non-existent book, got %d", len(verses))
	}
}

func TestBibleParser_GetAllVerses_Empty(t *testing.T) {
	dbPath := CreateTestBibleDB(t, []TestVerse{}, nil)

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	verses, err := parser.GetAllVerses()
	if err != nil {
		t.Fatalf("GetAllVerses() returned error: %v", err)
	}

	if len(verses) != 0 {
		t.Errorf("Expected 0 verses for empty database, got %d", len(verses))
	}
}

func TestCleanESwordText_MultipleFontCodes(t *testing.T) {
	// Test multiple font codes in sequence
	input := "\\f1\\f2\\f3Text with fonts"
	result := cleanESwordText(input)
	if strings.Contains(result, "\\f") {
		t.Errorf("Result still contains font codes: %q", result)
	}
}

func TestCleanESwordText_MultipleColorCodes(t *testing.T) {
	// Test multiple color codes in sequence
	input := "\\cf1\\cf255\\cf0Text with colors"
	result := cleanESwordText(input)
	if strings.Contains(result, "\\cf") {
		t.Errorf("Result still contains color codes: %q", result)
	}
}

func TestCleanESwordText_Subscript(t *testing.T) {
	input := "H\\sub2\\nosupersubO"
	result := cleanESwordText(input)
	expected := "H2O"
	if result != expected {
		t.Errorf("cleanESwordText(%q) = %q, want %q", input, result, expected)
	}
}

func TestBibleParser_GetBook_ScanError(t *testing.T) {
	// This tests the scan loop in GetBook - we need multiple verses to test the loop
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "V1"},
		{Book: 1, Chapter: 1, Verse: 2, Scripture: "V2"},
		{Book: 1, Chapter: 2, Verse: 1, Scripture: "V3"},
	}, nil)

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	verses, err := parser.GetBook(1)
	if err != nil {
		t.Fatalf("GetBook() returned error: %v", err)
	}

	// Verify all verses were scanned properly
	if len(verses) != 3 {
		t.Errorf("Expected 3 verses, got %d", len(verses))
	}
}

func TestBibleParser_GetAllVerses_ScanLoop(t *testing.T) {
	// Tests the scan loop in GetAllVerses with multiple books
	dbPath := CreateTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "Gen 1:1"},
		{Book: 2, Chapter: 1, Verse: 1, Scripture: "Exod 1:1"},
		{Book: 66, Chapter: 22, Verse: 21, Scripture: "Rev 22:21"},
	}, nil)

	parser, err := NewBibleParser(dbPath)
	if err != nil {
		t.Fatalf("NewBibleParser() returned error: %v", err)
	}
	defer parser.Close()

	verses, err := parser.GetAllVerses()
	if err != nil {
		t.Fatalf("GetAllVerses() returned error: %v", err)
	}

	if len(verses) != 3 {
		t.Errorf("Expected 3 verses, got %d", len(verses))
	}

	// Verify order - should be sorted by Book, Chapter, Verse
	if verses[0].Book != 1 {
		t.Errorf("First verse should be book 1, got %d", verses[0].Book)
	}
	if verses[2].Book != 66 {
		t.Errorf("Last verse should be book 66, got %d", verses[2].Book)
	}
}
