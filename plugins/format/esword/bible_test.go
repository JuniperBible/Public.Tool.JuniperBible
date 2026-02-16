package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
)

// TestVerse holds test data for creating Bible databases
type TestVerse struct {
	Book      int
	Chapter   int
	Verse     int
	Scripture string
}

// TestBibleMetadata holds metadata for test databases
type TestBibleMetadata struct {
	Title        string
	Abbreviation string
	Information  string
	Version      string
	Font         string
	RightToLeft  bool
}

// createTestBibleDB creates a temporary SQLite database with Bible data for testing.
func createTestBibleDB(t *testing.T, verses []TestVerse, metadata *TestBibleMetadata) string {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.bblx")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create Bible table
	_, err = db.Exec(`
		CREATE TABLE Bible (
			Book INTEGER,
			Chapter INTEGER,
			Verse INTEGER,
			Scripture TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create Bible table: %v", err)
	}

	// Insert verses
	for _, v := range verses {
		_, err = db.Exec(
			"INSERT INTO Bible (Book, Chapter, Verse, Scripture) VALUES (?, ?, ?, ?)",
			v.Book, v.Chapter, v.Verse, v.Scripture,
		)
		if err != nil {
			t.Fatalf("Failed to insert verse: %v", err)
		}
	}

	// Create Details table if metadata provided
	if metadata != nil {
		_, err = db.Exec(`
			CREATE TABLE Details (
				Description TEXT,
				Abbreviation TEXT,
				Information TEXT,
				Version TEXT,
				Font TEXT,
				RightToLeft INTEGER
			)
		`)
		if err != nil {
			t.Fatalf("Failed to create Details table: %v", err)
		}

		rtl := 0
		if metadata.RightToLeft {
			rtl = 1
		}
		_, err = db.Exec(
			"INSERT INTO Details (Description, Abbreviation, Information, Version, Font, RightToLeft) VALUES (?, ?, ?, ?, ?, ?)",
			metadata.Title, metadata.Abbreviation, metadata.Information, metadata.Version, metadata.Font, rtl,
		)
		if err != nil {
			t.Fatalf("Failed to insert metadata: %v", err)
		}
	}

	return dbPath
}

// createEmptyDB creates an empty SQLite database without any tables.
func createEmptyDB(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "empty.bblx")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to create empty database: %v", err)
	}
	defer db.Close()

	// Just create the file, no tables
	_, err = db.Exec("SELECT 1")
	if err != nil {
		t.Fatalf("Failed to initialize empty database: %v", err)
	}

	return dbPath
}

// createInvalidDB creates a file that is not a valid SQLite database.
func createInvalidDB(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "invalid.bblx")

	if err := os.WriteFile(dbPath, []byte("not a database"), 0600); err != nil {
		t.Fatalf("Failed to create invalid file: %v", err)
	}

	return dbPath
}

// TestNewBibleParser tests creating a new Bible parser.
func TestNewBibleParser(t *testing.T) {
	dbPath := createTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "In the beginning God created the heaven and the earth."},
	}, &TestBibleMetadata{
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

// TestBibleParserClose tests closing the parser.
func TestBibleParserClose(t *testing.T) {
	dbPath := createTestBibleDB(t, []TestVerse{
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

// TestBibleParserMetadata tests loading metadata from Details table.
func TestBibleParserMetadata(t *testing.T) {
	dbPath := createTestBibleDB(t, []TestVerse{
		{Book: 1, Chapter: 1, Verse: 1, Scripture: "Test"},
	}, &TestBibleMetadata{
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

// TestBibleParserGetVerse tests getting a single verse.
func TestBibleParserGetVerse(t *testing.T) {
	dbPath := createTestBibleDB(t, []TestVerse{
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

// TestBibleParserGetVerseNotFound tests getting a non-existent verse.
func TestBibleParserGetVerseNotFound(t *testing.T) {
	dbPath := createTestBibleDB(t, []TestVerse{
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

// TestBibleParserGetChapter tests getting all verses in a chapter.
func TestBibleParserGetChapter(t *testing.T) {
	dbPath := createTestBibleDB(t, []TestVerse{
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

// TestBibleParserGetBook tests getting all verses in a book.
func TestBibleParserGetBook(t *testing.T) {
	dbPath := createTestBibleDB(t, []TestVerse{
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

// TestBibleParserGetAllVerses tests getting all verses.
func TestBibleParserGetAllVerses(t *testing.T) {
	dbPath := createTestBibleDB(t, []TestVerse{
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

// TestCleanESwordText tests the RTF cleaning function.
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
