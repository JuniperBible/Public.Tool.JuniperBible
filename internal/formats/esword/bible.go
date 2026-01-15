// bible.go implements e-Sword Bible (.bblx) parsing.
// e-Sword uses SQLite databases with specific table schemas.
package esword

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
)

// BibleParser parses e-Sword Bible files (.bblx).
type BibleParser struct {
	db       *sql.DB
	filePath string
	metadata *BibleMetadata
}

// BibleMetadata contains information about an e-Sword Bible.
type BibleMetadata struct {
	Title        string `json:"title"`
	Abbreviation string `json:"abbreviation"`
	Information  string `json:"information"`
	Version      string `json:"version"`
	Font         string `json:"font"`
	RightToLeft  bool   `json:"right_to_left"`
}

// BibleVerse represents a single verse from an e-Sword Bible.
type BibleVerse struct {
	Book      int    `json:"book"`
	Chapter   int    `json:"chapter"`
	Verse     int    `json:"verse"`
	Scripture string `json:"scripture"`
}

// NewBibleParser creates a new e-Sword Bible parser.
func NewBibleParser(filePath string) (*BibleParser, error) {
	db, err := sqlite.OpenReadOnly(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	parser := &BibleParser{
		db:       db,
		filePath: filePath,
	}

	// Load metadata
	if err := parser.loadMetadata(); err != nil {
		db.Close()
		return nil, fmt.Errorf("loading metadata: %w", err)
	}

	return parser, nil
}

// Close closes the database connection.
func (p *BibleParser) Close() error {
	if p.db != nil {
		err := p.db.Close()
		p.db = nil
		return err
	}
	return nil
}

// loadMetadata loads Bible metadata from the Details table.
func (p *BibleParser) loadMetadata() error {
	p.metadata = &BibleMetadata{}

	// Check if Details table exists
	var count int
	err := p.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='Details'").Scan(&count)
	if err != nil || count == 0 {
		// No Details table, use defaults
		return nil
	}

	rows, err := p.db.Query("SELECT Title, Abbreviation, Information, Version, Font, RightToLeft FROM Details LIMIT 1")
	if err != nil {
		// Table might have different columns
		return nil
	}
	defer rows.Close()

	if rows.Next() {
		var title, abbrev, info, version, font sql.NullString
		var rtl sql.NullInt64
		if err := rows.Scan(&title, &abbrev, &info, &version, &font, &rtl); err != nil {
			return nil // Ignore scan errors, use defaults
		}
		p.metadata.Title = title.String
		p.metadata.Abbreviation = abbrev.String
		p.metadata.Information = info.String
		p.metadata.Version = version.String
		p.metadata.Font = font.String
		p.metadata.RightToLeft = rtl.Int64 != 0
	}

	return nil
}

// GetMetadata returns the Bible metadata.
func (p *BibleParser) GetMetadata() *BibleMetadata {
	return p.metadata
}

// GetVerse retrieves a single verse.
func (p *BibleParser) GetVerse(book, chapter, verse int) (*BibleVerse, error) {
	row := p.db.QueryRow(
		"SELECT Book, Chapter, Verse, Scripture FROM Bible WHERE Book = ? AND Chapter = ? AND Verse = ?",
		book, chapter, verse,
	)

	v := &BibleVerse{}
	var scripture sql.NullString
	if err := row.Scan(&v.Book, &v.Chapter, &v.Verse, &scripture); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("verse not found: %d:%d:%d", book, chapter, verse)
		}
		return nil, err
	}

	v.Scripture = cleanESwordText(scripture.String)
	return v, nil
}

// GetChapter retrieves all verses in a chapter.
func (p *BibleParser) GetChapter(book, chapter int) ([]*BibleVerse, error) {
	rows, err := p.db.Query(
		"SELECT Book, Chapter, Verse, Scripture FROM Bible WHERE Book = ? AND Chapter = ? ORDER BY Verse",
		book, chapter,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var verses []*BibleVerse
	for rows.Next() {
		v := &BibleVerse{}
		var scripture sql.NullString
		if err := rows.Scan(&v.Book, &v.Chapter, &v.Verse, &scripture); err != nil {
			return nil, err
		}
		v.Scripture = cleanESwordText(scripture.String)
		verses = append(verses, v)
	}

	return verses, rows.Err()
}

// GetBook retrieves all verses in a book.
func (p *BibleParser) GetBook(book int) ([]*BibleVerse, error) {
	rows, err := p.db.Query(
		"SELECT Book, Chapter, Verse, Scripture FROM Bible WHERE Book = ? ORDER BY Chapter, Verse",
		book,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var verses []*BibleVerse
	for rows.Next() {
		v := &BibleVerse{}
		var scripture sql.NullString
		if err := rows.Scan(&v.Book, &v.Chapter, &v.Verse, &scripture); err != nil {
			return nil, err
		}
		v.Scripture = cleanESwordText(scripture.String)
		verses = append(verses, v)
	}

	return verses, rows.Err()
}

// GetAllVerses retrieves all verses in the Bible.
func (p *BibleParser) GetAllVerses() ([]*BibleVerse, error) {
	rows, err := p.db.Query(
		"SELECT Book, Chapter, Verse, Scripture FROM Bible ORDER BY Book, Chapter, Verse",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var verses []*BibleVerse
	for rows.Next() {
		v := &BibleVerse{}
		var scripture sql.NullString
		if err := rows.Scan(&v.Book, &v.Chapter, &v.Verse, &scripture); err != nil {
			return nil, err
		}
		v.Scripture = cleanESwordText(scripture.String)
		verses = append(verses, v)
	}

	return verses, rows.Err()
}

// GetChapterCount returns the number of chapters in a book.
func (p *BibleParser) GetChapterCount(book int) (int, error) {
	var count int
	err := p.db.QueryRow(
		"SELECT MAX(Chapter) FROM Bible WHERE Book = ?",
		book,
	).Scan(&count)
	return count, err
}

// GetVerseCount returns the number of verses in a chapter.
func (p *BibleParser) GetVerseCount(book, chapter int) (int, error) {
	var count int
	err := p.db.QueryRow(
		"SELECT MAX(Verse) FROM Bible WHERE Book = ? AND Chapter = ?",
		book, chapter,
	).Scan(&count)
	return count, err
}

// cleanESwordText removes e-Sword RTF-like formatting codes from text.
func cleanESwordText(text string) string {
	// e-Sword uses RTF-like formatting
	text = strings.ReplaceAll(text, "\\par", "\n")
	text = strings.ReplaceAll(text, "\\line", "\n")

	// Remove font specifications
	for strings.Contains(text, "\\f") {
		start := strings.Index(text, "\\f")
		end := start + 2
		for end < len(text) && (text[end] >= '0' && text[end] <= '9') {
			end++
		}
		text = text[:start] + text[end:]
	}

	// Remove color specifications
	for strings.Contains(text, "\\cf") {
		start := strings.Index(text, "\\cf")
		end := start + 3
		for end < len(text) && (text[end] >= '0' && text[end] <= '9') {
			end++
		}
		text = text[:start] + text[end:]
	}

	// Remove other common RTF codes
	rtfCodes := []string{
		"\\b0", "\\b", "\\i0", "\\i", "\\ul0", "\\ul",
		"\\super", "\\nosupersub", "\\sub",
		"\\fs20", "\\fs22", "\\fs24", "\\fs26", "\\fs28",
	}
	for _, code := range rtfCodes {
		text = strings.ReplaceAll(text, code, "")
	}

	// Clean up extra whitespace
	text = strings.TrimSpace(text)

	return text
}
