//go:build !sdk

// Plugin format-esword handles e-Sword Bible module ingestion.
// e-Sword uses SQLite databases with extensions:
// - .bblx: Bible text
// - .cmtx: Commentary
// - .dctx: Dictionary
// - .topx: Topics
//
// IR Support:
// - extract-ir: Extracts IR from e-Sword database (L1 - text preserved)
// - emit-native: Converts IR back to e-Sword format (L1)
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

// e-Sword book number to OSIS ID mapping
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

	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	validExts := map[string]string{
		".bblx": "bible",
		".cmtx": "commentary",
		".dctx": "dictionary",
		".topx": "topics",
	}

	moduleType, validExt := validExts[ext]
	if !validExt {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("extension %s is not a known e-Sword format", ext),
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
	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   "e-Sword",
		Reason:   fmt.Sprintf("e-Sword %s database detected", moduleType),
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
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format":    "e-Sword",
			"extension": filepath.Ext(path),
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

		// Get row count for each table
		var count int64
		countRow := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %q", tableName))
		if err := countRow.Scan(&count); err != nil {
			count = 0 // Default to 0 on error
		}

		entries = append(entries, ipc.EnumerateEntry{
			Path:      tableName,
			SizeBytes: count, // Using row count as "size"
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

	ext := strings.ToLower(filepath.Ext(path))
	artifactID := strings.TrimSuffix(filepath.Base(path), ext)

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
		SourceFormat: "e-Sword",
		LossClass:    "L1",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		Attributes:   make(map[string]string),
	}

	// Store the original database as base64 for L0 reconstruction
	// (For L1 we just extract the text content)

	var lostElements []ipc.LostElement

	switch ext {
	case ".bblx":
		corpus.ModuleType = "BIBLE"
		extractBibleIR(db, corpus, &lostElements)
	case ".cmtx":
		corpus.ModuleType = "COMMENTARY"
		extractCommentaryIR(db, corpus, &lostElements)
	case ".dctx":
		corpus.ModuleType = "DICTIONARY"
		extractDictionaryIR(db, corpus, &lostElements)
	default:
		corpus.ModuleType = "GENERAL"
	}

	// Try to get metadata from Details table
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
			SourceFormat: "e-Sword",
			TargetFormat: "IR",
			LossClass:    "L1",
			LostElements: lostElements,
			Warnings: []string{
				"RTF formatting in Scripture field is simplified to plain text",
			},
		},
	})
}

func extractBibleIR(db *sql.DB, corpus *ipc.Corpus, lostElements *[]ipc.LostElement) {
	// Query Bible table: Book, Chapter, Verse, Scripture
	rows, err := db.Query("SELECT Book, Chapter, Verse, Scripture FROM Bible ORDER BY Book, Chapter, Verse")
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

		// Clean RTF from scripture text
		text := stripRTF(scripture)

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

		// Track RTF loss if present
		if scripture != text && strings.Contains(scripture, "{\\rtf") {
			*lostElements = append(*lostElements, ipc.LostElement{
				Path:        refID,
				ElementType: "rtf-formatting",
				Reason:      "RTF formatting stripped during extraction",
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
	rows, err := db.Query("SELECT Book, ChapterBegin, ChapterEnd, VerseBegin, VerseEnd, Comments FROM Commentary ORDER BY Book, ChapterBegin, VerseBegin")
	if err != nil {
		return
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

		text := stripRTF(comments)
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
	rows, err := db.Query("SELECT Topic, Definition FROM Dictionary ORDER BY Topic")
	if err != nil {
		return
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

		text := stripRTF(definition)
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

// stripRTF removes RTF formatting from text
func stripRTF(text string) string {
	if !strings.HasPrefix(text, "{\\rtf") {
		return text
	}

	// Simple RTF stripping - remove control words and groups
	var result strings.Builder
	inControl := false
	braceDepth := 0

	for i := 0; i < len(text); i++ {
		c := text[i]

		if c == '{' {
			braceDepth++
			continue
		}
		if c == '}' {
			braceDepth--
			continue
		}
		if c == '\\' {
			inControl = true
			// Skip control word
			for i+1 < len(text) && ((text[i+1] >= 'a' && text[i+1] <= 'z') || (text[i+1] >= 'A' && text[i+1] <= 'Z')) {
				i++
			}
			// Skip optional number
			for i+1 < len(text) && ((text[i+1] >= '0' && text[i+1] <= '9') || text[i+1] == '-') {
				i++
			}
			// Skip optional space
			if i+1 < len(text) && text[i+1] == ' ' {
				i++
			}
			inControl = false
			continue
		}

		if !inControl && braceDepth <= 1 {
			result.WriteByte(c)
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
	ext := ".bblx"
	switch corpus.ModuleType {
	case "COMMENTARY":
		ext = ".cmtx"
	case "DICTIONARY":
		ext = ".dctx"
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

	// Create Details table with metadata
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version TEXT, Font TEXT, RightToLeft INTEGER)"); err != nil {
		ipc.RespondErrorf("failed to create Details table: %v", err)
		return
	}
	title := corpus.Title
	if title == "" {
		title = corpus.ID
	}
	abbreviation := corpus.Attributes["abbreviation"]
	if _, err := db.Exec("INSERT INTO Details (Title, Abbreviation, Information, Version, RightToLeft) VALUES (?, ?, ?, ?, ?)",
		title, abbreviation, corpus.Description, "1.0", 0); err != nil {
		ipc.RespondErrorf("failed to insert Details: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "e-Sword",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "e-Sword",
			LossClass:    "L1",
			Warnings: []string{
				"RTF formatting not recreated from plain text",
			},
		},
	})
}

func emitBibleNative(db *sql.DB, corpus *ipc.Corpus) error {
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS Bible (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)"); err != nil {
		return fmt.Errorf("create Bible table: %w", err)
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
						if _, err := db.Exec("INSERT INTO Bible (Book, Chapter, Verse, Scripture) VALUES (?, ?, ?, ?)",
							bookNum, span.Ref.Chapter, span.Ref.Verse, cb.Text); err != nil {
							return fmt.Errorf("insert Bible verse %s.%d.%d: %w", doc.ID, span.Ref.Chapter, span.Ref.Verse, err)
						}
					}
				}
			}
		}
	}
	return nil
}

func emitCommentaryNative(db *sql.DB, corpus *ipc.Corpus) error {
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS Commentary (Book INTEGER, ChapterBegin INTEGER, ChapterEnd INTEGER, VerseBegin INTEGER, VerseEnd INTEGER, Comments TEXT)"); err != nil {
		return fmt.Errorf("create Commentary table: %w", err)
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
						if _, err := db.Exec("INSERT INTO Commentary (Book, ChapterBegin, ChapterEnd, VerseBegin, VerseEnd, Comments) VALUES (?, ?, ?, ?, ?, ?)",
							bookNum, span.Ref.Chapter, span.Ref.Chapter, span.Ref.Verse, verseEnd, cb.Text); err != nil {
							return fmt.Errorf("insert Commentary entry: %w", err)
						}
					}
				}
			}
		}
	}
	return nil
}

func emitDictionaryNative(db *sql.DB, corpus *ipc.Corpus) error {
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS Dictionary (Topic TEXT, Definition TEXT)"); err != nil {
		return fmt.Errorf("create Dictionary table: %w", err)
	}

	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			topic := ""
			if t, ok := cb.Attributes["topic"].(string); ok {
				topic = t
			}
			if _, err := db.Exec("INSERT INTO Dictionary (Topic, Definition) VALUES (?, ?)", topic, cb.Text); err != nil {
				return fmt.Errorf("insert Dictionary entry: %w", err)
			}
		}
	}
	return nil
}

// Compile check
var _ = io.Copy
