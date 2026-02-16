// Plugin format-mysword handles MySword Bible module ingestion.
// MySword is an Android Bible app that uses SQLite databases with extensions:
// - .mybible: Bible text (may also contain commentaries/dictionaries)
// - .commentaries.mybible: Commentary
// - .dictionary.mybible: Dictionary
//
// MySword schema is similar to e-Sword but with some differences:
// - Books table: Book, Chapter, Verse, Scripture (same as e-Sword)
// - info table contains module metadata (description, detailed_info, etc.)
//
// IR Support:
// - extract-ir: Extracts IR from MySword database (L1 - text preserved)
// - emit-native: Converts IR back to MySword format (L1)
package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// MySword book number to OSIS ID mapping (same as e-Sword)
var bookNumToOSIS = map[int]string{
	1: "Gen", 2: "Exod", 3: "Lev", 4: "Num", 5: "Deut",
	6: "Josh", 7: "Judg", 8: "Ruth", 9: "1Sam", 10: "2Sam",
	11: "1Kgs", 12: "2Kgs", 13: "1Chr", 14: "2Chr", 15: "Ezra",
	16: "Neh", 17: "Esth", 18: "Job", 19: "Ps", 20: "Prov",
	21: "Eccl", 22: "Song", 23: "Isa", 24: "Jer", 25: "Lam",
	26: "Ezek", 27: "Dan", 28: "Hos", 29: "Joel", 30: "Amos",
	31: "Obad", 32: "Jonah", 33: "Mic", 34: "Nah", 35: "Hab",
	36: "Zeph", 37: "Hag", 38: "Zech", 39: "Mal",
	40: "Matt", 41: "Mark", 42: "Luke", 43: "John", 44: "Acts",
	45: "Rom", 46: "1Cor", 47: "2Cor", 48: "Gal", 49: "Eph",
	50: "Phil", 51: "Col", 52: "1Thess", 53: "2Thess",
	54: "1Tim", 55: "2Tim", 56: "Titus", 57: "Phlm", 58: "Heb",
	59: "Jas", 60: "1Pet", 61: "2Pet", 62: "1John", 63: "2John",
	64: "3John", 65: "Jude", 66: "Rev",
}

var osisToBookNum = func() map[string]int {
	m := make(map[string]int)
	for k, v := range bookNumToOSIS {
		m[v] = k
	}
	return m
}()

func main() {
	if err := format.Run(&format.Config{
		Name:       "MySword",
		Extensions: []string{".mybible", ".commentaries.mybible", ".dictionary.mybible"},
		Detect:     detectMySword,
		Parse:      parseMySword,
		Emit:       emitMySword,
		Enumerate:  enumerateMySword,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectMySword(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{Detected: false, Reason: "path is a directory, not a file"}, nil
	}

	// Check file extension - MySword uses .mybible
	base := strings.ToLower(filepath.Base(path))
	moduleType := ""

	if strings.HasSuffix(base, ".mybible") {
		if strings.HasSuffix(base, ".commentaries.mybible") {
			moduleType = "commentary"
		} else if strings.HasSuffix(base, ".dictionary.mybible") {
			moduleType = "dictionary"
		} else {
			moduleType = "bible"
		}
	} else {
		return &ipc.DetectResult{Detected: false, Reason: "not a .mybible file"}, nil
	}

	// Verify it's a valid SQLite database
	db, err := sql.Open(sqliteDriver, path+"?mode=ro")
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot open as SQLite: %v", err)}, nil
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("not a valid SQLite database: %v", err)}, nil
	}

	// Check for MySword-specific tables
	hasBooksTable := false
	hasInfoTable := false

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err == nil {
				if strings.EqualFold(name, "Books") || strings.EqualFold(name, "Bible") {
					hasBooksTable = true
				}
				if strings.EqualFold(name, "info") || strings.EqualFold(name, "Details") {
					hasInfoTable = true
				}
			}
		}
	}

	// For Bible modules, require a Books/Bible table
	if !hasBooksTable && moduleType == "bible" {
		return &ipc.DetectResult{Detected: false, Reason: "no Books/Bible table found"}, nil
	}

	// hasInfoTable indicates higher confidence (used for future enhancements)
	_ = hasInfoTable

	return &ipc.DetectResult{Detected: true, Format: "MySword", Reason: fmt.Sprintf("MySword %s database detected", moduleType)}, nil
}

func enumerateMySword(path string) (*ipc.EnumerateResult, error) {
	db, err := sql.Open(sqliteDriver, path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var entries []ipc.EnumerateEntry
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}

		// SEC-005 FIX: Validate table name before using in query
		if !isValidTableName(tableName) {
			// Skip invalid table names to prevent SQL injection
			continue
		}

		var count int64
		db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %q", tableName)).Scan(&count)

		entries = append(entries, ipc.EnumerateEntry{
			Path:      tableName,
			SizeBytes: count,
			IsDir:     false,
			Metadata: map[string]string{
				"type":      "table",
				"row_count": fmt.Sprintf("%d", count),
			},
		})
	}

	return &ipc.EnumerateResult{Entries: entries}, nil
}

func parseMySword(path string) (*ir.Corpus, error) {
	base := strings.ToLower(filepath.Base(path))
	artifactID := filepath.Base(path)
	for strings.Contains(artifactID, ".") {
		artifactID = strings.TrimSuffix(artifactID, filepath.Ext(artifactID))
	}

	// Read source for hashing
	sourceData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read source: %w", err)
	}
	sourceHash := sha256.Sum256(sourceData)

	db, err := sql.Open(sqliteDriver, path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "MySword"
	corpus.LossClass = "L1"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])

	// Determine module type from filename
	if strings.HasSuffix(base, ".commentaries.mybible") {
		corpus.ModuleType = "COMMENTARY"
		extractCommentaryIR(db, corpus)
	} else if strings.HasSuffix(base, ".dictionary.mybible") {
		corpus.ModuleType = "DICTIONARY"
		extractDictionaryIR(db, corpus)
	} else {
		corpus.ModuleType = "BIBLE"
		extractBibleIR(db, corpus)
	}

	// Try to get metadata from info table (MySword style)
	var desc, detailedInfo, version string
	row := db.QueryRow("SELECT description, detailed_info, version FROM info LIMIT 1")
	if err := row.Scan(&desc, &detailedInfo, &version); err == nil {
		if desc != "" {
			corpus.Title = desc
		}
		if detailedInfo != "" {
			corpus.Description = detailedInfo
		}
		if version != "" {
			corpus.Attributes["version"] = version
		}
	}

	// Fall back to Details table (e-Sword compatible)
	if corpus.Title == "" {
		var title, abbreviation, info string
		row := db.QueryRow("SELECT Title, Abbreviation, Information FROM Details LIMIT 1")
		if err := row.Scan(&title, &abbreviation, &info); err == nil {
			if title != "" {
				corpus.Title = title
			}
			if abbreviation != "" {
				corpus.Attributes["abbreviation"] = abbreviation
			}
			if info != "" {
				corpus.Description = info
			}
		}
	}

	return corpus, nil
}

func extractBibleIR(db *sql.DB, corpus *ir.Corpus) {
	// MySword can use either "Books" or "Bible" table
	tableName := "Books"
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM Books").Scan(&count); err != nil {
		tableName = "Bible"
	}

	// Query Bible table: Book, Chapter, Verse, Scripture
	rows, err := db.Query(fmt.Sprintf("SELECT Book, Chapter, Verse, Scripture FROM %s ORDER BY Book, Chapter, Verse", tableName))
	if err != nil {
		return
	}
	defer rows.Close()

	// Group by book
	bookDocs := make(map[int]*ir.Document)
	sequence := 0

	for rows.Next() {
		var bookNum, chapter, verse int
		var scripture string
		if err := rows.Scan(&bookNum, &chapter, &verse, &scripture); err != nil {
			continue
		}

		// Get or create document for this book
		doc, ok := bookDocs[bookNum]
		if !ok {
			osisID := bookNumToOSIS[bookNum]
			if osisID == "" {
				osisID = fmt.Sprintf("Book%d", bookNum)
			}
			doc = ir.NewDocument(osisID, osisID, bookNum)
			doc.Attributes = map[string]string{"book_num": fmt.Sprintf("%d", bookNum)}
			bookDocs[bookNum] = doc
		}

		// Clean HTML from scripture text (MySword uses HTML, not RTF)
		text := stripHTML(scripture)

		sequence++
		osisID := bookNumToOSIS[bookNum]
		if osisID == "" {
			osisID = fmt.Sprintf("Book%d", bookNum)
		}
		refID := fmt.Sprintf("%s.%d.%d", osisID, chapter, verse)

		// Create content block for verse
		hash := sha256.Sum256([]byte(text))
		cb := &ir.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     text,
			Hash:     hex.EncodeToString(hash[:]),
			Anchors: []*ir.Anchor{{
				ID:       fmt.Sprintf("a-%d-0", sequence),
				Position: 0,
				Spans: []*ir.Span{{
					ID:            fmt.Sprintf("s-%s", refID),
					Type:          "VERSE",
					StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
					Ref: &ir.Ref{
						Book:    osisID,
						Chapter: chapter,
						Verse:   verse,
						OSISID:  refID,
					},
				}},
			}},
		}

		doc.ContentBlocks = append(doc.ContentBlocks, cb)
	}

	// Add documents to corpus in order
	for i := 1; i <= 66; i++ {
		if doc, ok := bookDocs[i]; ok {
			corpus.Documents = append(corpus.Documents, doc)
		}
	}
}

func extractCommentaryIR(db *sql.DB, corpus *ir.Corpus) {
	// MySword commentary table structure
	rows, err := db.Query("SELECT book_number, chapter_number_from, chapter_number_to, verse_number_from, verse_number_to, text FROM commentaries ORDER BY book_number, chapter_number_from, verse_number_from")
	if err != nil {
		// Fall back to e-Sword style
		rows, err = db.Query("SELECT Book, ChapterBegin, ChapterEnd, VerseBegin, VerseEnd, Comments FROM Commentary ORDER BY Book, ChapterBegin, VerseBegin")
		if err != nil {
			return
		}
	}
	defer rows.Close()

	sequence := 0
	doc := ir.NewDocument("commentary", "Commentary", 1)

	for rows.Next() {
		var bookNum, chapterBegin, chapterEnd, verseBegin, verseEnd int
		var comments string
		if err := rows.Scan(&bookNum, &chapterBegin, &chapterEnd, &verseBegin, &verseEnd, &comments); err != nil {
			continue
		}

		text := stripHTML(comments)
		sequence++

		osisID := bookNumToOSIS[bookNum]
		if osisID == "" {
			osisID = fmt.Sprintf("Book%d", bookNum)
		}

		refID := fmt.Sprintf("%s.%d.%d", osisID, chapterBegin, verseBegin)
		if chapterEnd != chapterBegin || verseEnd != verseBegin {
			refID = fmt.Sprintf("%s.%d.%d-%s.%d.%d", osisID, chapterBegin, verseBegin, osisID, chapterEnd, verseEnd)
		}

		hash := sha256.Sum256([]byte(text))
		cb := &ir.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     text,
			Hash:     hex.EncodeToString(hash[:]),
			Attributes: map[string]interface{}{
				"type": "commentary",
			},
			Anchors: []*ir.Anchor{{
				ID:       fmt.Sprintf("a-%d-0", sequence),
				Position: 0,
				Spans: []*ir.Span{{
					ID:            fmt.Sprintf("s-%s", refID),
					Type:          "COMMENT",
					StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
					Ref: &ir.Ref{
						Book:     osisID,
						Chapter:  chapterBegin,
						Verse:    verseBegin,
						VerseEnd: verseEnd,
						OSISID:   refID,
					},
				}},
			}},
		}

		doc.ContentBlocks = append(doc.ContentBlocks, cb)
	}

	corpus.Documents = []*ir.Document{doc}
}

func extractDictionaryIR(db *sql.DB, corpus *ir.Corpus) {
	// MySword dictionary table structure
	rows, err := db.Query("SELECT topic, definition FROM dictionary ORDER BY topic")
	if err != nil {
		// Fall back to e-Sword style
		rows, err = db.Query("SELECT Topic, Definition FROM Dictionary ORDER BY Topic")
		if err != nil {
			return
		}
	}
	defer rows.Close()

	sequence := 0
	doc := ir.NewDocument("dictionary", "Dictionary", 1)

	for rows.Next() {
		var topic, definition string
		if err := rows.Scan(&topic, &definition); err != nil {
			continue
		}

		text := stripHTML(definition)
		sequence++

		hash := sha256.Sum256([]byte(text))
		cb := &ir.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     text,
			Hash:     hex.EncodeToString(hash[:]),
			Attributes: map[string]interface{}{
				"topic": topic,
				"type":  "dictionary",
			},
		}

		doc.ContentBlocks = append(doc.ContentBlocks, cb)
	}

	corpus.Documents = []*ir.Document{doc}
}

// stripHTML removes HTML tags and decodes entities from text
func stripHTML(text string) string {
	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&apos;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&#39;", "'")

	// Remove HTML tags
	var result strings.Builder
	inTag := false

	for _, c := range text {
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(c)
		}
	}

	return strings.TrimSpace(result.String())
}

func emitMySword(corpus *ir.Corpus, outputDir string) (string, error) {
	// Determine output file extension
	ext := ".mybible"
	switch corpus.ModuleType {
	case "COMMENTARY":
		ext = ".commentaries.mybible"
	case "DICTIONARY":
		ext = ".dictionary.mybible"
	}

	outputPath := filepath.Join(outputDir, corpus.ID+ext)

	// Create new SQLite database
	db, err := sql.Open(sqliteDriver, outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	var emitErr error
	switch corpus.ModuleType {
	case "BIBLE":
		emitErr = emitBibleNative(db, corpus)
	case "COMMENTARY":
		emitErr = emitCommentaryNative(db, corpus)
	case "DICTIONARY":
		emitErr = emitDictionaryNative(db, corpus)
	default:
		emitErr = emitBibleNative(db, corpus)
	}

	if emitErr != nil {
		return "", fmt.Errorf("failed to emit content: %w", emitErr)
	}

	// Create info table with metadata (MySword style)
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS info (name TEXT, value TEXT)"); err != nil {
		return "", fmt.Errorf("failed to create info table: %w", err)
	}

	title := corpus.Title
	if title == "" {
		title = corpus.ID
	}

	// Insert metadata
	db.Exec("INSERT INTO info (name, value) VALUES ('description', ?)", title)
	if corpus.Description != "" {
		db.Exec("INSERT INTO info (name, value) VALUES ('detailed_info', ?)", corpus.Description)
	}
	if v, ok := corpus.Attributes["version"]; ok {
		db.Exec("INSERT INTO info (name, value) VALUES ('version', ?)", v)
	}
	db.Exec("INSERT INTO info (name, value) VALUES ('language', ?)", corpus.Language)

	return outputPath, nil
}

func emitBibleNative(db *sql.DB, corpus *ir.Corpus) error {
	// MySword uses "Books" table (lowercase columns)
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS Books (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)"); err != nil {
		return fmt.Errorf("create Books table: %w", err)
	}

	for _, doc := range corpus.Documents {
		bookNum := 0
		if num, ok := doc.Attributes["book_num"]; ok {
			fmt.Sscanf(num, "%d", &bookNum)
		} else if num, ok := osisToBookNum[doc.ID]; ok {
			bookNum = num
		}

		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						if _, err := db.Exec("INSERT INTO Books (Book, Chapter, Verse, Scripture) VALUES (?, ?, ?, ?)",
							bookNum, span.Ref.Chapter, span.Ref.Verse, cb.Text); err != nil {
							return fmt.Errorf("insert Books verse %s.%d.%d: %w", doc.ID, span.Ref.Chapter, span.Ref.Verse, err)
						}
					}
				}
			}
		}
	}
	return nil
}

func emitCommentaryNative(db *sql.DB, corpus *ir.Corpus) error {
	// MySword commentary table
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS commentaries (book_number INTEGER, chapter_number_from INTEGER, chapter_number_to INTEGER, verse_number_from INTEGER, verse_number_to INTEGER, text TEXT)"); err != nil {
		return fmt.Errorf("create commentaries table: %w", err)
	}

	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil {
						bookNum := osisToBookNum[span.Ref.Book]
						verseEnd := span.Ref.Verse
						if span.Ref.VerseEnd > 0 {
							verseEnd = span.Ref.VerseEnd
						}
						if _, err := db.Exec("INSERT INTO commentaries (book_number, chapter_number_from, chapter_number_to, verse_number_from, verse_number_to, text) VALUES (?, ?, ?, ?, ?, ?)",
							bookNum, span.Ref.Chapter, span.Ref.Chapter, span.Ref.Verse, verseEnd, cb.Text); err != nil {
							return fmt.Errorf("insert commentaries entry: %w", err)
						}
					}
				}
			}
		}
	}
	return nil
}

func emitDictionaryNative(db *sql.DB, corpus *ir.Corpus) error {
	// MySword dictionary table
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS dictionary (topic TEXT, definition TEXT)"); err != nil {
		return fmt.Errorf("create dictionary table: %w", err)
	}

	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			topic := ""
			if t, ok := cb.Attributes["topic"].(string); ok {
				topic = t
			}
			if _, err := db.Exec("INSERT INTO dictionary (topic, definition) VALUES (?, ?)", topic, cb.Text); err != nil {
				return fmt.Errorf("insert dictionary entry: %w", err)
			}
		}
	}
	return nil
}

// isValidTableName validates that a table name contains only safe characters.
// This prevents SQL injection when using table names from sqlite_master in queries.
// SEC-005 FIX: Whitelist valid table names for MySword databases.
func isValidTableName(name string) bool {
	// Empty names are invalid
	if name == "" {
		return false
	}

	// Whitelist of known valid MySword table names
	validTables := map[string]bool{
		"Books":           true,
		"Bible":           true,
		"info":            true,
		"Details":         true,
		"commentaries":    true,
		"Commentary":      true,
		"dictionary":      true,
		"Dictionary":      true,
		"sqlite_sequence": true, // SQLite internal table
	}

	// If it's in the whitelist, it's valid
	if validTables[name] {
		return true
	}

	// For other tables, validate characters: allow alphanumeric, underscore, and hyphen
	// Table names should not contain quotes, semicolons, or other SQL metacharacters
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '_' || ch == '-') {
			return false
		}
	}

	return true
}
