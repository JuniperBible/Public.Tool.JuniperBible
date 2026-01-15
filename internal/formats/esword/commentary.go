// commentary.go implements e-Sword Commentary (.cmtx) parser.
// Commentary files are SQLite databases with Commentary and Details tables.
//
// Table: Commentary
// - Book INTEGER (1-66)
// - ChapterBegin INTEGER
// - VerseBegin INTEGER
// - ChapterEnd INTEGER
// - VerseEnd INTEGER
// - Comments TEXT (may contain RTF formatting)
//
// Table: Details
// - Title TEXT
// - Abbreviation TEXT
// - Information TEXT
// - Version INTEGER
// - RightToLeft INTEGER
package esword

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
)

// CommentaryDetails contains metadata about a commentary module.
type CommentaryDetails struct {
	Title        string
	Abbreviation string
	Information  string
	Version      int
	RightToLeft  bool
}

// CommentaryModuleInfo contains summary information about a commentary.
type CommentaryModuleInfo struct {
	Title      string
	EntryCount int
}

// CommentaryEntry represents a commentary entry.
type CommentaryEntry struct {
	Book         int    `json:"book"`
	ChapterStart int    `json:"chapter_start"`
	VerseStart   int    `json:"verse_start"`
	ChapterEnd   int    `json:"chapter_end"`
	VerseEnd     int    `json:"verse_end"`
	Comments     string `json:"comments"`
}

// CommentaryParser handles parsing of e-Sword commentary files.
type CommentaryParser struct {
	db      *sql.DB
	dbPath  string
	details *CommentaryDetails
	entries map[string]*CommentaryEntry
}

// NewCommentaryParser creates a new parser for an e-Sword commentary file.
func NewCommentaryParser(path string) (*CommentaryParser, error) {
	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	parser := &CommentaryParser{
		db:      db,
		dbPath:  path,
		entries: make(map[string]*CommentaryEntry),
	}

	// Load details
	if err := parser.loadDetails(); err != nil {
		db.Close()
		return nil, err
	}

	// Load entries into cache
	if err := parser.loadEntries(); err != nil {
		db.Close()
		return nil, err
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

// loadDetails loads the Details table.
func (p *CommentaryParser) loadDetails() error {
	row := p.db.QueryRow(`SELECT Title, Abbreviation, Information, Version, RightToLeft FROM Details LIMIT 1`)

	var d CommentaryDetails
	var title, abbrev, info sql.NullString
	var version, rtl sql.NullInt64
	if err := row.Scan(&title, &abbrev, &info, &version, &rtl); err != nil {
		if err == sql.ErrNoRows {
			// No details table or empty
			p.details = &CommentaryDetails{}
			return nil
		}
		return fmt.Errorf("reading details: %w", err)
	}

	d.Title = title.String
	d.Abbreviation = abbrev.String
	d.Information = info.String
	d.Version = int(version.Int64)
	d.RightToLeft = rtl.Int64 != 0
	p.details = &d
	return nil
}

// loadEntries loads all commentary entries into the cache.
func (p *CommentaryParser) loadEntries() error {
	rows, err := p.db.Query(`SELECT Book, ChapterBegin, VerseBegin, ChapterEnd, VerseEnd, Comments FROM Commentary`)
	if err != nil {
		return fmt.Errorf("querying commentary: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry CommentaryEntry
		if err := rows.Scan(&entry.Book, &entry.ChapterStart, &entry.VerseStart,
			&entry.ChapterEnd, &entry.VerseEnd, &entry.Comments); err != nil {
			return fmt.Errorf("scanning row: %w", err)
		}

		key := fmt.Sprintf("%d:%d:%d", entry.Book, entry.ChapterStart, entry.VerseStart)
		p.entries[key] = &CommentaryEntry{
			Book:         entry.Book,
			ChapterStart: entry.ChapterStart,
			VerseStart:   entry.VerseStart,
			ChapterEnd:   entry.ChapterEnd,
			VerseEnd:     entry.VerseEnd,
			Comments:     entry.Comments,
		}
	}

	return rows.Err()
}

// GetEntry retrieves a commentary entry by book, chapter, and verse.
func (p *CommentaryParser) GetEntry(book, chapter, verse int) (*CommentaryEntry, error) {
	key := fmt.Sprintf("%d:%d:%d", book, chapter, verse)
	entry, ok := p.entries[key]
	if !ok {
		return nil, fmt.Errorf("entry not found: %s", key)
	}
	return entry, nil
}

// GetEntryByRef retrieves a commentary entry by OSIS reference.
func (p *CommentaryParser) GetEntryByRef(ref string) (*CommentaryEntry, error) {
	// Parse OSIS reference (e.g., "Matt.5.3")
	book, chapter, verse, err := parseOSISRef(ref)
	if err != nil {
		return nil, err
	}
	return p.GetEntry(book, chapter, verse)
}

// GetChapter retrieves all commentary entries for a chapter.
func (p *CommentaryParser) GetChapter(book, chapter int) []*CommentaryEntry {
	var entries []*CommentaryEntry
	for _, entry := range p.entries {
		if entry.Book == book && entry.ChapterStart == chapter {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetBook retrieves all commentary entries for a book.
func (p *CommentaryParser) GetBook(book int) []*CommentaryEntry {
	var entries []*CommentaryEntry
	for _, entry := range p.entries {
		if entry.Book == book {
			entries = append(entries, entry)
		}
	}
	return entries
}

// ListBooks returns a list of all books with commentary entries.
func (p *CommentaryParser) ListBooks() []int {
	bookSet := make(map[int]bool)
	for _, entry := range p.entries {
		bookSet[entry.Book] = true
	}

	var books []int
	for book := range bookSet {
		books = append(books, book)
	}
	return books
}

// ModuleInfo returns summary information about the commentary.
func (p *CommentaryParser) ModuleInfo() CommentaryModuleInfo {
	title := ""
	if p.details != nil {
		title = p.details.Title
	}
	return CommentaryModuleInfo{
		Title:      title,
		EntryCount: len(p.entries),
	}
}

// HasOT returns true if the commentary has Old Testament entries.
func (p *CommentaryParser) HasOT() bool {
	for _, entry := range p.entries {
		if entry.Book >= 1 && entry.Book <= 39 {
			return true
		}
	}
	return false
}

// HasNT returns true if the commentary has New Testament entries.
func (p *CommentaryParser) HasNT() bool {
	for _, entry := range p.entries {
		if entry.Book >= 40 && entry.Book <= 66 {
			return true
		}
	}
	return false
}

// IsRange returns true if the entry spans multiple verses.
func (e *CommentaryEntry) IsRange() bool {
	return e.VerseEnd > e.VerseStart || e.ChapterEnd > e.ChapterStart
}

// IsMultiChapter returns true if the entry spans multiple chapters.
func (e *CommentaryEntry) IsMultiChapter() bool {
	return e.ChapterEnd > e.ChapterStart
}

// IsCommentaryFile returns true if the filename is an e-Sword commentary file.
func IsCommentaryFile(filename string) bool {
	ext := strings.ToLower(filename)
	return strings.HasSuffix(ext, ".cmtx")
}

// cleanCommentaryText removes RTF formatting from commentary text.
func cleanCommentaryText(text string) string {
	// Remove RTF control words like \rtf1, \b, \par, etc. but keep content
	// First, remove control words with their optional numeric parameter and trailing space
	rtfControlWord := regexp.MustCompile(`\\[a-z]+\d*\s?`)
	cleaned := rtfControlWord.ReplaceAllString(text, " ")

	// Remove braces
	cleaned = strings.ReplaceAll(cleaned, "{", "")
	cleaned = strings.ReplaceAll(cleaned, "}", "")

	// Normalize multiple spaces to single space
	multiSpace := regexp.MustCompile(`\s+`)
	cleaned = multiSpace.ReplaceAllString(cleaned, " ")

	return strings.TrimSpace(cleaned)
}

// BookName returns the English name for a book number (1-66).
func BookName(book int) string {
	names := []string{
		"", // 0 is unused
		"Genesis", "Exodus", "Leviticus", "Numbers", "Deuteronomy",
		"Joshua", "Judges", "Ruth", "1 Samuel", "2 Samuel",
		"1 Kings", "2 Kings", "1 Chronicles", "2 Chronicles", "Ezra",
		"Nehemiah", "Esther", "Job", "Psalms", "Proverbs",
		"Ecclesiastes", "Song of Solomon", "Isaiah", "Jeremiah", "Lamentations",
		"Ezekiel", "Daniel", "Hosea", "Joel", "Amos",
		"Obadiah", "Jonah", "Micah", "Nahum", "Habakkuk",
		"Zephaniah", "Haggai", "Zechariah", "Malachi",
		"Matthew", "Mark", "Luke", "John", "Acts",
		"Romans", "1 Corinthians", "2 Corinthians", "Galatians", "Ephesians",
		"Philippians", "Colossians", "1 Thessalonians", "2 Thessalonians", "1 Timothy",
		"2 Timothy", "Titus", "Philemon", "Hebrews", "James",
		"1 Peter", "2 Peter", "1 John", "2 John", "3 John",
		"Jude", "Revelation",
	}

	if book >= 1 && book < len(names) {
		return names[book]
	}
	return fmt.Sprintf("Book %d", book)
}

// parseOSISRef parses an OSIS reference into book number, chapter, and verse.
func parseOSISRef(ref string) (book, chapter, verse int, err error) {
	// Map OSIS book IDs to numbers
	bookMap := map[string]int{
		"Gen": 1, "Exod": 2, "Lev": 3, "Num": 4, "Deut": 5,
		"Josh": 6, "Judg": 7, "Ruth": 8, "1Sam": 9, "2Sam": 10,
		"1Kgs": 11, "2Kgs": 12, "1Chr": 13, "2Chr": 14, "Ezra": 15,
		"Neh": 16, "Esth": 17, "Job": 18, "Ps": 19, "Prov": 20,
		"Eccl": 21, "Song": 22, "Isa": 23, "Jer": 24, "Lam": 25,
		"Ezek": 26, "Dan": 27, "Hos": 28, "Joel": 29, "Amos": 30,
		"Obad": 31, "Jonah": 32, "Mic": 33, "Nah": 34, "Hab": 35,
		"Zeph": 36, "Hag": 37, "Zech": 38, "Mal": 39,
		"Matt": 40, "Mark": 41, "Luke": 42, "John": 43, "Acts": 44,
		"Rom": 45, "1Cor": 46, "2Cor": 47, "Gal": 48, "Eph": 49,
		"Phil": 50, "Col": 51, "1Thess": 52, "2Thess": 53, "1Tim": 54,
		"2Tim": 55, "Titus": 56, "Phlm": 57, "Heb": 58, "Jas": 59,
		"1Pet": 60, "2Pet": 61, "1John": 62, "2John": 63, "3John": 64,
		"Jude": 65, "Rev": 66,
	}

	parts := strings.Split(ref, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid OSIS reference: %s", ref)
	}

	book, ok := bookMap[parts[0]]
	if !ok {
		return 0, 0, 0, fmt.Errorf("unknown book: %s", parts[0])
	}

	if _, err := fmt.Sscanf(parts[1], "%d", &chapter); err != nil {
		return 0, 0, 0, fmt.Errorf("invalid chapter: %s", parts[1])
	}

	if _, err := fmt.Sscanf(parts[2], "%d", &verse); err != nil {
		return 0, 0, 0, fmt.Errorf("invalid verse: %s", parts[2])
	}

	return book, chapter, verse, nil
}
