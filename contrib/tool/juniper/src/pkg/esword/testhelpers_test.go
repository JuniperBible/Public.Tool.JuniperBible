package esword

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestVerse holds test data for creating Bible databases
type TestVerse struct {
	Book      int
	Chapter   int
	Verse     int
	Scripture string
}

// TestCommentaryEntry holds test data for creating commentary databases
type TestCommentaryEntry struct {
	Book         int
	ChapterBegin int
	VerseBegin   int
	ChapterEnd   int
	VerseEnd     int
	Comments     string
}

// TestDictionaryEntry holds test data for creating dictionary databases
type TestDictionaryEntry struct {
	Topic      string
	Definition string
}

// CreateTestBibleDB creates a temporary SQLite database with Bible data for testing.
func CreateTestBibleDB(t *testing.T, verses []TestVerse, metadata *BibleMetadata) string {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.bblx")

	db, err := sql.Open("sqlite3", dbPath)
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

// CreateTestCommentaryDB creates a temporary SQLite database with commentary data for testing.
func CreateTestCommentaryDB(t *testing.T, entries []TestCommentaryEntry, metadata *CommentaryMetadata) string {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.cmtx")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create Commentary table
	_, err = db.Exec(`
		CREATE TABLE Commentary (
			Book INTEGER,
			ChapterBegin INTEGER,
			VerseBegin INTEGER,
			ChapterEnd INTEGER,
			VerseEnd INTEGER,
			Comments TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create Commentary table: %v", err)
	}

	// Insert entries
	for _, e := range entries {
		_, err = db.Exec(
			"INSERT INTO Commentary (Book, ChapterBegin, VerseBegin, ChapterEnd, VerseEnd, Comments) VALUES (?, ?, ?, ?, ?, ?)",
			e.Book, e.ChapterBegin, e.VerseBegin, e.ChapterEnd, e.VerseEnd, e.Comments,
		)
		if err != nil {
			t.Fatalf("Failed to insert commentary entry: %v", err)
		}
	}

	// Create Details table if metadata provided
	if metadata != nil {
		_, err = db.Exec(`
			CREATE TABLE Details (
				Description TEXT,
				Abbreviation TEXT,
				Information TEXT,
				Version TEXT
			)
		`)
		if err != nil {
			t.Fatalf("Failed to create Details table: %v", err)
		}

		_, err = db.Exec(
			"INSERT INTO Details (Description, Abbreviation, Information, Version) VALUES (?, ?, ?, ?)",
			metadata.Title, metadata.Abbreviation, metadata.Information, metadata.Version,
		)
		if err != nil {
			t.Fatalf("Failed to insert metadata: %v", err)
		}
	}

	return dbPath
}

// CreateTestDictionaryDB creates a temporary SQLite database with dictionary data for testing.
func CreateTestDictionaryDB(t *testing.T, entries []TestDictionaryEntry, metadata *DictionaryMetadata) string {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.dctx")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create Dictionary table
	_, err = db.Exec(`
		CREATE TABLE Dictionary (
			Topic TEXT,
			Definition TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create Dictionary table: %v", err)
	}

	// Insert entries
	for _, e := range entries {
		_, err = db.Exec(
			"INSERT INTO Dictionary (Topic, Definition) VALUES (?, ?)",
			e.Topic, e.Definition,
		)
		if err != nil {
			t.Fatalf("Failed to insert dictionary entry: %v", err)
		}
	}

	// Create Details table if metadata provided
	if metadata != nil {
		_, err = db.Exec(`
			CREATE TABLE Details (
				Description TEXT,
				Abbreviation TEXT,
				Information TEXT,
				Version TEXT
			)
		`)
		if err != nil {
			t.Fatalf("Failed to create Details table: %v", err)
		}

		_, err = db.Exec(
			"INSERT INTO Details (Description, Abbreviation, Information, Version) VALUES (?, ?, ?, ?)",
			metadata.Title, metadata.Abbreviation, metadata.Information, metadata.Version,
		)
		if err != nil {
			t.Fatalf("Failed to insert metadata: %v", err)
		}
	}

	return dbPath
}

// CreateInvalidDB creates an empty file that is not a valid SQLite database.
func CreateInvalidDB(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "invalid.bblx")

	if err := os.WriteFile(dbPath, []byte("not a database"), 0600); err != nil {
		t.Fatalf("Failed to create invalid file: %v", err)
	}

	return dbPath
}

// CreateEmptyDB creates an empty SQLite database without any tables.
func CreateEmptyDB(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "empty.bblx")

	db, err := sql.Open("sqlite3", dbPath)
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
