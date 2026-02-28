// Package esword provides parsers for e-Sword Bible module format.
package esword

import (
	"database/sql"
	"fmt"
	"strings"
)

// CommentaryParser parses e-Sword commentary files (.cmtx).
//
// The .cmtx format is a SQLite database with a Commentary table:
//   - Book: INTEGER (1-66)
//   - ChapterBegin: INTEGER
//   - VerseBegin: INTEGER
//   - ChapterEnd: INTEGER
//   - VerseEnd: INTEGER
//   - Comments: TEXT (commentary content)
type CommentaryParser struct {
	db       *sql.DB
	filePath string
	metadata *CommentaryMetadata
}

// CommentaryMetadata contains information about an e-Sword commentary.
type CommentaryMetadata struct {
	Title        string
	Abbreviation string
	Information  string
	Version      string
}

// CommentaryEntry represents a commentary entry from e-Sword.
type CommentaryEntry struct {
	Book         int
	ChapterBegin int
	VerseBegin   int
	ChapterEnd   int
	VerseEnd     int
	Comments     string
}

// NewCommentaryParser creates a new e-Sword commentary parser.
func NewCommentaryParser(filePath string) (*CommentaryParser, error) {
	db, err := sql.Open("sqlite3", filePath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	parser := &CommentaryParser{
		db:       db,
		filePath: filePath,
	}

	if err := parser.loadMetadata(); err != nil {
		db.Close()
		return nil, fmt.Errorf("loading metadata: %w", err)
	}

	return parser, nil
}

// Close closes the database connection.
func (p *CommentaryParser) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// loadMetadata loads commentary metadata from the Details table.
func (p *CommentaryParser) loadMetadata() error {
	p.metadata = &CommentaryMetadata{}

	var count int
	err := p.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='Details'").Scan(&count)
	if err != nil || count == 0 {
		return nil
	}

	rows, err := p.db.Query("SELECT Description, Abbreviation, Information, Version FROM Details LIMIT 1")
	if err != nil {
		return nil
	}
	defer rows.Close()

	if rows.Next() {
		var title, abbrev, info, version sql.NullString
		if err := rows.Scan(&title, &abbrev, &info, &version); err != nil {
			return nil
		}
		p.metadata.Title = title.String
		p.metadata.Abbreviation = abbrev.String
		p.metadata.Information = info.String
		p.metadata.Version = version.String
	}

	return nil
}

// GetMetadata returns the commentary metadata.
func (p *CommentaryParser) GetMetadata() *CommentaryMetadata {
	return p.metadata
}

// GetEntry retrieves a commentary entry for a specific verse.
func (p *CommentaryParser) GetEntry(book, chapter, verse int) (*CommentaryEntry, error) {
	row := p.db.QueryRow(`
		SELECT Book, ChapterBegin, VerseBegin, ChapterEnd, VerseEnd, Comments
		FROM Commentary
		WHERE Book = ? AND ChapterBegin <= ? AND ChapterEnd >= ?
		  AND VerseBegin <= ? AND VerseEnd >= ?
		LIMIT 1`,
		book, chapter, chapter, verse, verse,
	)

	entry := &CommentaryEntry{}
	var comments sql.NullString
	if err := row.Scan(&entry.Book, &entry.ChapterBegin, &entry.VerseBegin,
		&entry.ChapterEnd, &entry.VerseEnd, &comments); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("entry not found: %d:%d:%d", book, chapter, verse)
		}
		return nil, err
	}

	entry.Comments = cleanESwordText(comments.String)
	return entry, nil
}

// GetChapterEntries retrieves all commentary entries for a chapter.
func (p *CommentaryParser) GetChapterEntries(book, chapter int) ([]*CommentaryEntry, error) {
	rows, err := p.db.Query(`
		SELECT Book, ChapterBegin, VerseBegin, ChapterEnd, VerseEnd, Comments
		FROM Commentary
		WHERE Book = ? AND (
			(ChapterBegin <= ? AND ChapterEnd >= ?) OR
			(ChapterBegin = ? AND ChapterEnd = ?)
		)
		ORDER BY ChapterBegin, VerseBegin`,
		book, chapter, chapter, chapter, chapter,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*CommentaryEntry
	for rows.Next() {
		entry := &CommentaryEntry{}
		var comments sql.NullString
		if err := rows.Scan(&entry.Book, &entry.ChapterBegin, &entry.VerseBegin,
			&entry.ChapterEnd, &entry.VerseEnd, &comments); err != nil {
			return nil, err
		}
		entry.Comments = cleanESwordText(comments.String)
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// GetBookEntries retrieves all commentary entries for a book.
func (p *CommentaryParser) GetBookEntries(book int) ([]*CommentaryEntry, error) {
	rows, err := p.db.Query(`
		SELECT Book, ChapterBegin, VerseBegin, ChapterEnd, VerseEnd, Comments
		FROM Commentary
		WHERE Book = ?
		ORDER BY ChapterBegin, VerseBegin`,
		book,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*CommentaryEntry
	for rows.Next() {
		entry := &CommentaryEntry{}
		var comments sql.NullString
		if err := rows.Scan(&entry.Book, &entry.ChapterBegin, &entry.VerseBegin,
			&entry.ChapterEnd, &entry.VerseEnd, &comments); err != nil {
			return nil, err
		}
		entry.Comments = cleanESwordText(comments.String)
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// GetAllEntries retrieves all commentary entries.
func (p *CommentaryParser) GetAllEntries() ([]*CommentaryEntry, error) {
	rows, err := p.db.Query(`
		SELECT Book, ChapterBegin, VerseBegin, ChapterEnd, VerseEnd, Comments
		FROM Commentary
		ORDER BY Book, ChapterBegin, VerseBegin`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*CommentaryEntry
	for rows.Next() {
		entry := &CommentaryEntry{}
		var comments sql.NullString
		if err := rows.Scan(&entry.Book, &entry.ChapterBegin, &entry.VerseBegin,
			&entry.ChapterEnd, &entry.VerseEnd, &comments); err != nil {
			return nil, err
		}
		entry.Comments = cleanESwordText(comments.String)
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// FormatReference formats the entry reference as a string.
func (e *CommentaryEntry) FormatReference() string {
	bookInfo, ok := ToSwordBook(e.Book)
	if !ok {
		return fmt.Sprintf("Book %d", e.Book)
	}

	if e.ChapterBegin == e.ChapterEnd && e.VerseBegin == e.VerseEnd {
		return fmt.Sprintf("%s %d:%d", bookInfo.Name, e.ChapterBegin, e.VerseBegin)
	}

	if e.ChapterBegin == e.ChapterEnd {
		return fmt.Sprintf("%s %d:%d-%d", bookInfo.Name, e.ChapterBegin, e.VerseBegin, e.VerseEnd)
	}

	return fmt.Sprintf("%s %d:%d-%d:%d", bookInfo.Name, e.ChapterBegin, e.VerseBegin, e.ChapterEnd, e.VerseEnd)
}

// cleanCommentaryText performs additional cleaning for commentary text.
func cleanCommentaryText(text string) string {
	text = cleanESwordText(text)

	// Remove scripture reference tags
	for strings.Contains(text, "<scripture>") {
		start := strings.Index(text, "<scripture>")
		end := strings.Index(text, "</scripture>")
		if end > start {
			text = text[:start] + text[start+11:end] + text[end+12:]
		} else {
			break
		}
	}

	return text
}
