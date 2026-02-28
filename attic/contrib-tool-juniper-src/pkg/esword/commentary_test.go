package esword

import (
	"strings"
	"testing"
)

func TestNewCommentaryParser_Valid(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "Test comment"},
	}, &CommentaryMetadata{
		Title:        "Test Commentary",
		Abbreviation: "TC",
		Version:      "1.0",
	})

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	if parser.db == nil {
		t.Error("parser.db is nil")
	}
	if parser.metadata == nil {
		t.Error("parser.metadata is nil")
	}
}

func TestNewCommentaryParser_InvalidPath(t *testing.T) {
	// SQLite may create file, so test with empty database instead
	dbPath := CreateEmptyDB(t)

	parser, err := NewCommentaryParser(dbPath)
	if err == nil {
		defer parser.Close()
		// Parser opened but GetEntry should fail
		_, entryErr := parser.GetEntry(1, 1, 1)
		if entryErr == nil {
			t.Error("GetEntry should fail on empty database")
		}
	}
}

func TestCommentaryParser_Close(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "Test"},
	}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}

	err = parser.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestCommentaryParser_GetMetadata(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "Test"},
	}, &CommentaryMetadata{
		Title:        "Matthew Henry Commentary",
		Abbreviation: "MHC",
		Information:  "Public Domain",
		Version:      "2.0",
	})

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	metadata := parser.GetMetadata()
	if metadata.Title != "Matthew Henry Commentary" {
		t.Errorf("Title = %q, want %q", metadata.Title, "Matthew Henry Commentary")
	}
	if metadata.Abbreviation != "MHC" {
		t.Errorf("Abbreviation = %q, want %q", metadata.Abbreviation, "MHC")
	}
}

func TestCommentaryParser_GetEntry_Exact(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "Comment on Gen 1:1"},
	}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	entry, err := parser.GetEntry(1, 1, 1)
	if err != nil {
		t.Fatalf("GetEntry() returned error: %v", err)
	}

	if entry.Book != 1 {
		t.Errorf("Book = %d, want 1", entry.Book)
	}
	if entry.Comments != "Comment on Gen 1:1" {
		t.Errorf("Comments = %q, want expected text", entry.Comments)
	}
}

func TestCommentaryParser_GetEntry_Range(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 5, Comments: "Comment on Gen 1:1-5"},
	}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	// Request verse 3, should get the range that covers it
	entry, err := parser.GetEntry(1, 1, 3)
	if err != nil {
		t.Fatalf("GetEntry() returned error: %v", err)
	}

	if entry.VerseBegin != 1 || entry.VerseEnd != 5 {
		t.Errorf("Entry range = %d-%d, want 1-5", entry.VerseBegin, entry.VerseEnd)
	}
}

func TestCommentaryParser_GetEntry_NotFound(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "Test"},
	}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	_, err = parser.GetEntry(99, 99, 99)
	if err == nil {
		t.Error("GetEntry() should return error for non-existent entry")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should mention 'not found', got: %v", err)
	}
}

func TestCommentaryParser_GetChapterEntries(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "V1"},
		{Book: 1, ChapterBegin: 1, VerseBegin: 2, ChapterEnd: 1, VerseEnd: 3, Comments: "V2-3"},
		{Book: 1, ChapterBegin: 2, VerseBegin: 1, ChapterEnd: 2, VerseEnd: 1, Comments: "Ch2"},
	}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	entries, err := parser.GetChapterEntries(1, 1)
	if err != nil {
		t.Fatalf("GetChapterEntries() returned error: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("len(entries) = %d, want 2", len(entries))
	}
}

func TestCommentaryParser_GetBookEntries(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "Gen"},
		{Book: 1, ChapterBegin: 2, VerseBegin: 1, ChapterEnd: 2, VerseEnd: 1, Comments: "Gen 2"},
		{Book: 2, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "Exod"},
	}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	entries, err := parser.GetBookEntries(1)
	if err != nil {
		t.Fatalf("GetBookEntries() returned error: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("len(entries) = %d, want 2", len(entries))
	}

	for _, e := range entries {
		if e.Book != 1 {
			t.Errorf("Entry has Book=%d, want 1", e.Book)
		}
	}
}

func TestCommentaryParser_GetAllEntries(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "E1"},
		{Book: 1, ChapterBegin: 1, VerseBegin: 2, ChapterEnd: 1, VerseEnd: 2, Comments: "E2"},
		{Book: 2, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "E3"},
	}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	entries, err := parser.GetAllEntries()
	if err != nil {
		t.Fatalf("GetAllEntries() returned error: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("len(entries) = %d, want 3", len(entries))
	}
}

func TestCommentaryEntry_FormatReference_Single(t *testing.T) {
	entry := &CommentaryEntry{
		Book:         1,
		ChapterBegin: 1,
		VerseBegin:   1,
		ChapterEnd:   1,
		VerseEnd:     1,
	}

	ref := entry.FormatReference()
	if ref != "Genesis 1:1" {
		t.Errorf("FormatReference() = %q, want %q", ref, "Genesis 1:1")
	}
}

func TestCommentaryEntry_FormatReference_Range(t *testing.T) {
	entry := &CommentaryEntry{
		Book:         1,
		ChapterBegin: 1,
		VerseBegin:   1,
		ChapterEnd:   1,
		VerseEnd:     5,
	}

	ref := entry.FormatReference()
	if ref != "Genesis 1:1-5" {
		t.Errorf("FormatReference() = %q, want %q", ref, "Genesis 1:1-5")
	}
}

func TestCommentaryEntry_FormatReference_ChapterRange(t *testing.T) {
	entry := &CommentaryEntry{
		Book:         1,
		ChapterBegin: 1,
		VerseBegin:   1,
		ChapterEnd:   2,
		VerseEnd:     3,
	}

	ref := entry.FormatReference()
	if ref != "Genesis 1:1-2:3" {
		t.Errorf("FormatReference() = %q, want %q", ref, "Genesis 1:1-2:3")
	}
}

func TestCommentaryEntry_FormatReference_InvalidBook(t *testing.T) {
	entry := &CommentaryEntry{
		Book:         99,
		ChapterBegin: 1,
		VerseBegin:   1,
		ChapterEnd:   1,
		VerseEnd:     1,
	}

	ref := entry.FormatReference()
	if !strings.Contains(ref, "Book 99") {
		t.Errorf("FormatReference() should fallback to 'Book 99', got %q", ref)
	}
}

func TestCommentaryParser_TextCleaning(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1,
			Comments: "\\bBold\\b0\\par\\iItalic\\i0"},
	}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	entry, err := parser.GetEntry(1, 1, 1)
	if err != nil {
		t.Fatalf("GetEntry() returned error: %v", err)
	}

	// RTF codes should be cleaned
	if strings.Contains(entry.Comments, "\\b") {
		t.Errorf("Comments still contains RTF codes: %q", entry.Comments)
	}
}

func TestCommentaryParser_MultipleEntries(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "Entry 1"},
		{Book: 1, ChapterBegin: 1, VerseBegin: 2, ChapterEnd: 1, VerseEnd: 2, Comments: "Entry 2"},
		{Book: 1, ChapterBegin: 1, VerseBegin: 3, ChapterEnd: 1, VerseEnd: 3, Comments: "Entry 3"},
		{Book: 1, ChapterBegin: 2, VerseBegin: 1, ChapterEnd: 2, VerseEnd: 1, Comments: "Ch2"},
		{Book: 2, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 2, VerseEnd: 1, Comments: "Book2"},
	}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	entries, err := parser.GetAllEntries()
	if err != nil {
		t.Fatalf("GetAllEntries() returned error: %v", err)
	}

	if len(entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(entries))
	}
}

func TestCommentaryParser_GetAllEntries_Empty(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	entries, err := parser.GetAllEntries()
	if err != nil {
		t.Fatalf("GetAllEntries() returned error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestCommentaryParser_Close_NilDB(t *testing.T) {
	// Test Close when db is nil
	parser := &CommentaryParser{db: nil}
	err := parser.Close()
	if err != nil {
		t.Errorf("Close() on nil db should not error, got: %v", err)
	}
}

func TestCommentaryParser_GetChapterEntries_Empty(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "Test"},
	}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	// Get a chapter that doesn't exist
	entries, err := parser.GetChapterEntries(99, 99)
	if err != nil {
		t.Fatalf("GetChapterEntries() returned error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries for non-existent chapter, got %d", len(entries))
	}
}

func TestCommentaryParser_GetBookEntries_Empty(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "Test"},
	}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	// Get a book that doesn't exist
	entries, err := parser.GetBookEntries(99)
	if err != nil {
		t.Fatalf("GetBookEntries() returned error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries for non-existent book, got %d", len(entries))
	}
}

func TestCommentaryEntry_FormatReference_WholeChapter(t *testing.T) {
	entry := &CommentaryEntry{
		Book:         19, // Psalms
		ChapterBegin: 23,
		VerseBegin:   0, // Whole chapter
		ChapterEnd:   23,
		VerseEnd:     0,
	}

	ref := entry.FormatReference()
	// Should format as "Psalms 23:0-0" or similar
	if ref == "" {
		t.Error("FormatReference() should not return empty string")
	}
}

func TestCleanCommentaryText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple scripture tag",
			input:    "<scripture>John 3:16</scripture>",
			expected: "John 3:16",
		},
		{
			name:     "multiple scripture tags",
			input:    "See <scripture>Gen 1:1</scripture> and <scripture>Rev 22:21</scripture>",
			expected: "See Gen 1:1 and Rev 22:21",
		},
		{
			name:     "scripture with RTF",
			input:    "\\bBold\\b0 <scripture>Ref</scripture>",
			expected: "Bold Ref",
		},
		{
			name:     "nested text",
			input:    "Text <scripture>inner text</scripture> more",
			expected: "Text inner text more",
		},
		{
			name:     "no scripture tags",
			input:    "Just plain text",
			expected: "Just plain text",
		},
		{
			name:     "unclosed scripture tag",
			input:    "<scripture>Unclosed",
			expected: "<scripture>Unclosed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanCommentaryText(tt.input)
			if result != tt.expected {
				t.Errorf("cleanCommentaryText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCommentaryParser_GetChapterEntries_MultipleRanges(t *testing.T) {
	// Test multiple entries with various chapter ranges
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 5, Comments: "Entry spanning 1:1-5"},
		{Book: 1, ChapterBegin: 1, VerseBegin: 6, ChapterEnd: 1, VerseEnd: 10, Comments: "Entry spanning 1:6-10"},
		{Book: 1, ChapterBegin: 1, VerseBegin: 11, ChapterEnd: 2, VerseEnd: 3, Comments: "Cross-chapter entry"},
	}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	// Get chapter 1 entries - should include the cross-chapter entry
	entries, err := parser.GetChapterEntries(1, 1)
	if err != nil {
		t.Fatalf("GetChapterEntries() returned error: %v", err)
	}

	// All 3 entries cover chapter 1 in some way
	if len(entries) != 3 {
		t.Errorf("len(entries) = %d, want 3", len(entries))
	}
}

func TestCommentaryParser_GetBookEntries_MultipleChapters(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "Ch1"},
		{Book: 1, ChapterBegin: 2, VerseBegin: 1, ChapterEnd: 2, VerseEnd: 1, Comments: "Ch2"},
		{Book: 1, ChapterBegin: 50, VerseBegin: 1, ChapterEnd: 50, VerseEnd: 26, Comments: "Ch50"},
	}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	entries, err := parser.GetBookEntries(1)
	if err != nil {
		t.Fatalf("GetBookEntries() returned error: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("len(entries) = %d, want 3", len(entries))
	}
}

func TestCommentaryParser_GetAllEntries_MultipleBooks(t *testing.T) {
	dbPath := CreateTestCommentaryDB(t, []TestCommentaryEntry{
		{Book: 1, ChapterBegin: 1, VerseBegin: 1, ChapterEnd: 1, VerseEnd: 1, Comments: "Genesis"},
		{Book: 19, ChapterBegin: 23, VerseBegin: 1, ChapterEnd: 23, VerseEnd: 6, Comments: "Psalm 23"},
		{Book: 43, ChapterBegin: 3, VerseBegin: 16, ChapterEnd: 3, VerseEnd: 16, Comments: "John 3:16"},
		{Book: 66, ChapterBegin: 22, VerseBegin: 21, ChapterEnd: 22, VerseEnd: 21, Comments: "Revelation"},
	}, nil)

	parser, err := NewCommentaryParser(dbPath)
	if err != nil {
		t.Fatalf("NewCommentaryParser() returned error: %v", err)
	}
	defer parser.Close()

	entries, err := parser.GetAllEntries()
	if err != nil {
		t.Fatalf("GetAllEntries() returned error: %v", err)
	}

	if len(entries) != 4 {
		t.Errorf("len(entries) = %d, want 4", len(entries))
	}

	// Verify ordering by book
	if entries[0].Book != 1 {
		t.Errorf("First entry should be book 1, got %d", entries[0].Book)
	}
	if entries[3].Book != 66 {
		t.Errorf("Last entry should be book 66, got %d", entries[3].Book)
	}
}
