//go:build !sdk

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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
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
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorf("failed to decode request: %v", err)
		return
	}

	switch req.Command {
	case "detect":
		handleDetect(req.Args)
	case "ingest":
		handleIngest(req.Args)
	case "enumerate":
		handleEnumerate(req.Args)
	case "extract-ir":
		handleExtractIR(req.Args)
	case "emit-native":
		handleEmitNative(req.Args)
	default:
		ipc.RespondErrorf("unknown command: %s", req.Command)
	}
}

func handleDetect(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if info.IsDir() {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "path is a directory, not a file",
		})
		return
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
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not a .mybible file",
		})
		return
	}

	// Verify it's a valid SQLite database
	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open as SQLite: %v", err),
		})
		return
	}
	defer db.Close()

	// Try a simple query to verify it's a valid database
	if err := db.Ping(); err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("not a valid SQLite database: %v", err),
		})
		return
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
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "no Books/Bible table found",
		})
		return
	}

	// hasInfoTable indicates higher confidence (used for future enhancements)
	_ = hasInfoTable

	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   "MySword",
		Reason:   fmt.Sprintf("MySword %s database detected", moduleType),
	})
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	// Read file and compute hash
	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	// Write blob
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		ipc.RespondErrorf("failed to create blob dir: %v", err)
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		ipc.RespondErrorf("failed to write blob: %v", err)
		return
	}

	// Get artifact ID from filename
	artifactID := filepath.Base(path)
	// Remove multiple extensions
	for strings.Contains(artifactID, ".") {
		artifactID = strings.TrimSuffix(artifactID, filepath.Ext(artifactID))
	}

	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format":    "MySword",
			"extension": ".mybible",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		ipc.RespondErrorf("failed to open database: %v", err)
		return
	}
	defer db.Close()

	// List all tables
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		ipc.RespondErrorf("failed to query tables: %v", err)
		return
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

		// Get row count for each table
		var count int64
		countRow := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %q", tableName))
		if err := countRow.Scan(&count); err != nil {
			count = 0
		}

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

	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: entries,
	})
}

func handleExtractIR(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	base := strings.ToLower(filepath.Base(path))
	artifactID := filepath.Base(path)
	for strings.Contains(artifactID, ".") {
		artifactID = strings.TrimSuffix(artifactID, filepath.Ext(artifactID))
	}

	// Read source for hashing
	sourceData, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read source: %v", err)
		return
	}
	sourceHash := sha256.Sum256(sourceData)

	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		ipc.RespondErrorf("failed to open database: %v", err)
		return
	}
	defer db.Close()

	corpus := &ipc.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "MySword",
		LossClass:    "L1",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		Attributes:   make(map[string]string),
	}

	var lostElements []ipc.LostElement

	// Determine module type from filename
	if strings.HasSuffix(base, ".commentaries.mybible") {
		corpus.ModuleType = "COMMENTARY"
		extractCommentaryIR(db, corpus, &lostElements)
	} else if strings.HasSuffix(base, ".dictionary.mybible") {
		corpus.ModuleType = "DICTIONARY"
		extractDictionaryIR(db, corpus, &lostElements)
	} else {
		corpus.ModuleType = "BIBLE"
		extractBibleIR(db, corpus, &lostElements)
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

	// Serialize IR to JSON
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to serialize IR: %v", err)
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}

	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "MySword",
			TargetFormat: "IR",
			LossClass:    "L1",
			LostElements: lostElements,
			Warnings: []string{
				"HTML formatting in Scripture field simplified to plain text",
			},
		},
	})
}

func extractBibleIR(db *sql.DB, corpus *ipc.Corpus, lostElements *[]ipc.LostElement) {
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
	bookDocs := make(map[int]*ipc.Document)
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
			doc = &ipc.Document{
				ID:         osisID,
				Title:      osisID,
				Order:      bookNum,
				Attributes: map[string]string{"book_num": fmt.Sprintf("%d", bookNum)},
			}
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
		cb := &ipc.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     text,
			Hash:     hex.EncodeToString(hash[:]),
			Anchors: []*ipc.Anchor{
				{
					ID:       fmt.Sprintf("a-%d-0", sequence),
					Position: 0,
					Spans: []*ipc.Span{
						{
							ID:            fmt.Sprintf("s-%s", refID),
							Type:          "VERSE",
							StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
							Ref: &ipc.Ref{
								Book:    osisID,
								Chapter: chapter,
								Verse:   verse,
								OSISID:  refID,
							},
						},
					},
				},
			},
		}

		// Track HTML loss if present
		if scripture != text && (strings.Contains(scripture, "<") || strings.Contains(scripture, "&")) {
			*lostElements = append(*lostElements, ipc.LostElement{
				Path:        refID,
				ElementType: "html-formatting",
				Reason:      "HTML formatting stripped during extraction",
			})
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

func extractCommentaryIR(db *sql.DB, corpus *ipc.Corpus, lostElements *[]ipc.LostElement) {
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
	doc := &ipc.Document{
		ID:    "commentary",
		Title: "Commentary",
		Order: 1,
	}

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
		cb := &ipc.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     text,
			Hash:     hex.EncodeToString(hash[:]),
			Attributes: map[string]interface{}{
				"type": "commentary",
			},
			Anchors: []*ipc.Anchor{
				{
					ID:       fmt.Sprintf("a-%d-0", sequence),
					Position: 0,
					Spans: []*ipc.Span{
						{
							ID:            fmt.Sprintf("s-%s", refID),
							Type:          "COMMENT",
							StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
							Ref: &ipc.Ref{
								Book:     osisID,
								Chapter:  chapterBegin,
								Verse:    verseBegin,
								VerseEnd: verseEnd,
								OSISID:   refID,
							},
						},
					},
				},
			},
		}

		doc.ContentBlocks = append(doc.ContentBlocks, cb)
	}

	corpus.Documents = []*ipc.Document{doc}
}

func extractDictionaryIR(db *sql.DB, corpus *ipc.Corpus, lostElements *[]ipc.LostElement) {
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
	doc := &ipc.Document{
		ID:    "dictionary",
		Title: "Dictionary",
		Order: 1,
	}

	for rows.Next() {
		var topic, definition string
		if err := rows.Scan(&topic, &definition); err != nil {
			continue
		}

		text := stripHTML(definition)
		sequence++

		hash := sha256.Sum256([]byte(text))
		cb := &ipc.ContentBlock{
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

	corpus.Documents = []*ipc.Document{doc}
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

func handleEmitNative(args map[string]interface{}) {
	irPath, ok := args["ir_path"].(string)
	if !ok {
		ipc.RespondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	// Read IR file
	data, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR file: %v", err)
		return
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		ipc.RespondErrorf("failed to parse IR: %v", err)
		return
	}

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
	db, err := sqlite.Open(outputPath)
	if err != nil {
		ipc.RespondErrorf("failed to create database: %v", err)
		return
	}
	defer db.Close()

	var emitErr error
	switch corpus.ModuleType {
	case "BIBLE":
		emitErr = emitBibleNative(db, &corpus)
	case "COMMENTARY":
		emitErr = emitCommentaryNative(db, &corpus)
	case "DICTIONARY":
		emitErr = emitDictionaryNative(db, &corpus)
	default:
		emitErr = emitBibleNative(db, &corpus)
	}

	if emitErr != nil {
		ipc.RespondErrorf("failed to emit content: %v", emitErr)
		return
	}

	// Create info table with metadata (MySword style)
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS info (name TEXT, value TEXT)"); err != nil {
		ipc.RespondErrorf("failed to create info table: %v", err)
		return
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

	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "MySword",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "MySword",
			LossClass:    "L1",
			Warnings: []string{
				"HTML formatting not recreated from plain text",
			},
		},
	})
}

func emitBibleNative(db *sql.DB, corpus *ipc.Corpus) error {
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

func emitCommentaryNative(db *sql.DB, corpus *ipc.Corpus) error {
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

func emitDictionaryNative(db *sql.DB, corpus *ipc.Corpus) error {
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

// Compile check
var _ = io.Copy
