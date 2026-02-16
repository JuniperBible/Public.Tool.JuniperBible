//go:build sdk

// Plugin format-mybible handles MyBible.zone Bible module ingestion.
// MyBible is an Android Bible app that uses SQLite databases with extension:
// - .SQLite3: Bible text (MyBible.zone format)
//
// MyBible.zone schema uses lowercase table/column names:
// - verses table: book_number, chapter, verse, text
// - books table: book_number, book_name, book_color
// - info table: name, value pairs for metadata
//
// IR Support:
// - extract-ir: Extracts IR from MyBible database (L1 - text preserved)
// - emit-native: Converts IR back to MyBible format (L1)
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

// MyBible book number to OSIS ID mapping (same as e-Sword/MySword)
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
		Name:       "MyBible",
		Extensions: []string{".sqlite3", ".SQLite3"},
		Detect:     detectMyBible,
		Parse:      parseMyBible,
		Emit:       emitMyBible,
		Enumerate:  enumerateMyBible,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectMyBible(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{Detected: false, Reason: "path is a directory, not a file"}, nil
	}

	// Check file extension - MyBible.zone uses .SQLite3 or .sqlite3
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".sqlite3" {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("extension %s is not .SQLite3", ext)}, nil
	}

	// Verify it's a valid SQLite database
	db, err := sql.Open(sqliteDriver, path+"?mode=ro")
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot open as SQLite: %v", err)}, nil
	}
	defer db.Close()

	// Try a simple query to verify it's a valid database
	if err := db.Ping(); err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("not a valid SQLite database: %v", err)}, nil
	}

	// Check for MyBible-specific tables (lowercase)
	hasVersesTable := false

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err == nil {
				if name == "verses" {
					hasVersesTable = true
				}
			}
		}
	}

	// MyBible.zone requires verses table
	if !hasVersesTable {
		return &ipc.DetectResult{Detected: false, Reason: "no verses table found (MyBible.zone format requires verses table)"}, nil
	}

	// Verify verses table has expected columns
	var hasBookNumber, hasChapter, hasVerse, hasText bool
	colRows, err := db.Query("PRAGMA table_info(verses)")
	if err == nil {
		defer colRows.Close()
		for colRows.Next() {
			var cid int
			var name string
			var ctype string
			var notnull int
			var dfltValue interface{}
			var pk int
			if err := colRows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err == nil {
				switch name {
				case "book_number":
					hasBookNumber = true
				case "chapter":
					hasChapter = true
				case "verse":
					hasVerse = true
				case "text":
					hasText = true
				}
			}
		}
	}

	if !hasBookNumber || !hasChapter || !hasVerse || !hasText {
		return &ipc.DetectResult{Detected: false, Reason: "verses table missing required columns (book_number, chapter, verse, text)"}, nil
	}

	return &ipc.DetectResult{Detected: true, Format: "MyBible", Reason: "MyBible.zone Bible database detected"}, nil
}

func enumerateMyBible(path string) (*ipc.EnumerateResult, error) {
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

func parseMyBible(path string) (*ir.Corpus, error) {
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

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
	corpus.SourceFormat = "MyBible"
	corpus.LossClass = "L1"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])

	// Extract Bible content from verses table
	extractBibleIR(db, corpus)

	// Try to get metadata from info table (MyBible.zone style: name-value pairs)
	infoMap := make(map[string]string)
	rows, err := db.Query("SELECT name, value FROM info")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name, value string
			if err := rows.Scan(&name, &value); err == nil {
				infoMap[name] = value
			}
		}
	}

	// Map common metadata fields
	if desc, ok := infoMap["description"]; ok && desc != "" {
		corpus.Title = desc
	}
	if detailedInfo, ok := infoMap["detailed_info"]; ok && detailedInfo != "" {
		corpus.Description = detailedInfo
	}
	if lang, ok := infoMap["language"]; ok && lang != "" {
		corpus.Language = lang
	}
	if version, ok := infoMap["version"]; ok && version != "" {
		corpus.Attributes["version"] = version
	}

	// Store all info fields as attributes
	for k, v := range infoMap {
		if k != "description" && k != "detailed_info" && k != "language" && k != "version" {
			corpus.Attributes[k] = v
		}
	}

	return corpus, nil
}

func extractBibleIR(db *sql.DB, corpus *ir.Corpus) {
	// Query verses table: book_number, chapter, verse, text
	rows, err := db.Query("SELECT book_number, chapter, verse, text FROM verses ORDER BY book_number, chapter, verse")
	if err != nil {
		return
	}
	defer rows.Close()

	// Group by book
	bookDocs := make(map[int]*ir.Document)
	sequence := 0

	for rows.Next() {
		var bookNum, chapter, verse int
		var text string
		if err := rows.Scan(&bookNum, &chapter, &verse, &text); err != nil {
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

		// Clean HTML from text (MyBible uses HTML)
		cleanText := stripHTML(text)

		sequence++
		osisID := bookNumToOSIS[bookNum]
		if osisID == "" {
			osisID = fmt.Sprintf("Book%d", bookNum)
		}
		refID := fmt.Sprintf("%s.%d.%d", osisID, chapter, verse)

		// Create content block for verse
		hash := sha256.Sum256([]byte(cleanText))
		cb := &ir.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     cleanText,
			Hash:     hex.EncodeToString(hash[:]),
			Anchors: []*ir.Anchor{{
				ID:       fmt.Sprintf("a-%d-0", sequence),
				Position: 0,
				Spans: []*ir.Span{{
					ID:            fmt.Sprintf("s-%s", refID),
					Type:          "VERSE",
					StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
					Ref:           &ir.Ref{Book: osisID, Chapter: chapter, Verse: verse, OSISID: refID},
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

// stripHTML removes HTML tags and decodes entities from text
func stripHTML(text string) string {
	// Remove HTML tags first
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

	// Then decode common HTML entities
	cleaned := result.String()
	cleaned = strings.ReplaceAll(cleaned, "&amp;", "&")
	cleaned = strings.ReplaceAll(cleaned, "&lt;", "<")
	cleaned = strings.ReplaceAll(cleaned, "&gt;", ">")
	cleaned = strings.ReplaceAll(cleaned, "&quot;", "\"")
	cleaned = strings.ReplaceAll(cleaned, "&apos;", "'")
	cleaned = strings.ReplaceAll(cleaned, "&nbsp;", " ")
	cleaned = strings.ReplaceAll(cleaned, "&#39;", "'")

	return strings.TrimSpace(cleaned)
}

func emitMyBible(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".SQLite3")

	// Create new SQLite database
	db, err := sql.Open(sqliteDriver, outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	// Create verses table (MyBible.zone schema)
	if _, err := db.Exec(`CREATE TABLE verses (
		book_number INTEGER NOT NULL,
		chapter INTEGER NOT NULL,
		verse INTEGER NOT NULL,
		text TEXT NOT NULL
	)`); err != nil {
		return "", fmt.Errorf("failed to create verses table: %w", err)
	}

	// Create indexes for performance
	db.Exec("CREATE INDEX book_number_index ON verses (book_number)")
	db.Exec("CREATE INDEX chapter_index ON verses (chapter)")
	db.Exec("CREATE INDEX verse_index ON verses (verse)")

	// Emit Bible content
	if err := emitBibleNative(db, corpus); err != nil {
		return "", fmt.Errorf("failed to emit content: %w", err)
	}

	// Create info table with metadata (MyBible.zone style: name-value pairs)
	if _, err := db.Exec("CREATE TABLE info (name TEXT NOT NULL, value TEXT NOT NULL)"); err != nil {
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
	if corpus.Language != "" {
		db.Exec("INSERT INTO info (name, value) VALUES ('language', ?)", corpus.Language)
	}
	if v, ok := corpus.Attributes["version"]; ok {
		db.Exec("INSERT INTO info (name, value) VALUES ('version', ?)", v)
	}

	// Insert other attributes
	for k, v := range corpus.Attributes {
		if k != "version" {
			db.Exec("INSERT INTO info (name, value) VALUES (?, ?)", k, v)
		}
	}

	return outputPath, nil
}

func emitBibleNative(db *sql.DB, corpus *ir.Corpus) error {
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
						if _, err := db.Exec("INSERT INTO verses (book_number, chapter, verse, text) VALUES (?, ?, ?, ?)",
							bookNum, span.Ref.Chapter, span.Ref.Verse, cb.Text); err != nil {
							return fmt.Errorf("insert verse %s.%d.%d: %w", doc.ID, span.Ref.Chapter, span.Ref.Verse, err)
						}
					}
				}
			}
		}
	}
	return nil
}
