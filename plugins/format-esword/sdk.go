
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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
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

func runSDK() {
	if err := format.Run(&format.Config{
		Name:       "e-Sword",
		Extensions: []string{".bblx", ".cmtx", ".dctx", ".topx"},
		Detect:     detectESword,
		Parse:      parseESword,
		Emit:       emitESword,
		Enumerate:  enumerateESword,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectESword(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{Detected: false, Reason: "path is a directory"}, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	validExts := map[string]string{
		".bblx": "bible",
		".cmtx": "commentary",
		".dctx": "dictionary",
		".topx": "topics",
	}

	moduleType, validExt := validExts[ext]
	if !validExt {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("extension %s is not a known e-Sword format", ext)}, nil
	}

	db, err := sql.Open(sqliteDriver, path+"?mode=ro")
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot open as SQLite: %v", err)}, nil
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("not a valid SQLite database: %v", err)}, nil
	}

	return &ipc.DetectResult{Detected: true, Format: "e-Sword", Reason: fmt.Sprintf("e-Sword %s database detected", moduleType)}, nil
}

func enumerateESword(path string) (*ipc.EnumerateResult, error) {
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

func parseESword(path string) (*ir.Corpus, error) {
	ext := strings.ToLower(filepath.Ext(path))
	artifactID := strings.TrimSuffix(filepath.Base(path), ext)

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
	corpus.SourceFormat = "e-Sword"
	corpus.LossClass = "L1"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])

	switch ext {
	case ".bblx":
		corpus.ModuleType = "BIBLE"
		extractBibleIR(db, corpus)
	case ".cmtx":
		corpus.ModuleType = "COMMENTARY"
		extractCommentaryIR(db, corpus)
	case ".dctx":
		corpus.ModuleType = "DICTIONARY"
		extractDictionaryIR(db, corpus)
	default:
		corpus.ModuleType = "GENERAL"
	}

	// Get metadata from Details table
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

	return corpus, nil
}

func extractBibleIR(db *sql.DB, corpus *ir.Corpus) {
	rows, err := db.Query("SELECT Book, Chapter, Verse, Scripture FROM Bible ORDER BY Book, Chapter, Verse")
	if err != nil {
		return
	}
	defer rows.Close()

	bookDocs := make(map[int]*ir.Document)
	sequence := 0

	for rows.Next() {
		var bookNum, chapter, verse int
		var scripture string
		if err := rows.Scan(&bookNum, &chapter, &verse, &scripture); err != nil {
			continue
		}

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

		text := stripRTF(scripture)
		sequence++
		osisID := bookNumToOSIS[bookNum]
		if osisID == "" {
			osisID = fmt.Sprintf("Book%d", bookNum)
		}
		refID := fmt.Sprintf("%s.%d.%d", osisID, chapter, verse)

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
					Ref:           &ir.Ref{Book: osisID, Chapter: chapter, Verse: verse, OSISID: refID},
				}},
			}},
		}

		doc.ContentBlocks = append(doc.ContentBlocks, cb)
	}

	for i := 1; i <= 66; i++ {
		if doc, ok := bookDocs[i]; ok {
			corpus.Documents = append(corpus.Documents, doc)
		}
	}
}

func extractCommentaryIR(db *sql.DB, corpus *ir.Corpus) {
	rows, err := db.Query("SELECT Book, ChapterBegin, ChapterEnd, VerseBegin, VerseEnd, Comments FROM Commentary ORDER BY Book, ChapterBegin, VerseBegin")
	if err != nil {
		return
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

		text := stripRTF(comments)
		sequence++

		osisID := bookNumToOSIS[bookNum]
		if osisID == "" {
			osisID = fmt.Sprintf("Book%d", bookNum)
		}
		refID := fmt.Sprintf("%s.%d.%d", osisID, chapterBegin, verseBegin)

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
					Type:          "COMMENT",
					StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
					Ref:           &ir.Ref{Book: osisID, Chapter: chapterBegin, Verse: verseBegin, VerseEnd: verseEnd, OSISID: refID},
				}},
			}},
		}

		doc.ContentBlocks = append(doc.ContentBlocks, cb)
	}

	corpus.Documents = []*ir.Document{doc}
}

func extractDictionaryIR(db *sql.DB, corpus *ir.Corpus) {
	rows, err := db.Query("SELECT Topic, Definition FROM Dictionary ORDER BY Topic")
	if err != nil {
		return
	}
	defer rows.Close()

	sequence := 0
	doc := ir.NewDocument("dictionary", "Dictionary", 1)

	for rows.Next() {
		var topic, definition string
		if err := rows.Scan(&topic, &definition); err != nil {
			continue
		}

		text := stripRTF(definition)
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

func stripRTF(text string) string {
	if !strings.HasPrefix(text, "{\\rtf") {
		return text
	}

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
			for i+1 < len(text) && ((text[i+1] >= 'a' && text[i+1] <= 'z') || (text[i+1] >= 'A' && text[i+1] <= 'Z')) {
				i++
			}
			for i+1 < len(text) && ((text[i+1] >= '0' && text[i+1] <= '9') || text[i+1] == '-') {
				i++
			}
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

func emitESword(corpus *ir.Corpus, outputDir string) (string, error) {
	ext := ".bblx"
	switch corpus.ModuleType {
	case "COMMENTARY":
		ext = ".cmtx"
	case "DICTIONARY":
		ext = ".dctx"
	}

	outputPath := filepath.Join(outputDir, corpus.ID+ext)

	db, err := sql.Open(sqliteDriver, outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	switch corpus.ModuleType {
	case "BIBLE":
		emitBibleNative(db, corpus)
	case "COMMENTARY":
		emitCommentaryNative(db, corpus)
	case "DICTIONARY":
		emitDictionaryNative(db, corpus)
	default:
		emitBibleNative(db, corpus)
	}

	// Create Details table
	db.Exec("CREATE TABLE IF NOT EXISTS Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version TEXT, Font TEXT, RightToLeft INTEGER)")
	title := corpus.Title
	if title == "" {
		title = corpus.ID
	}
	abbreviation := corpus.Attributes["abbreviation"]
	db.Exec("INSERT INTO Details (Title, Abbreviation, Information, Version, RightToLeft) VALUES (?, ?, ?, ?, ?)",
		title, abbreviation, corpus.Description, "1.0", 0)

	return outputPath, nil
}

func emitBibleNative(db *sql.DB, corpus *ir.Corpus) {
	db.Exec("CREATE TABLE IF NOT EXISTS Bible (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")

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
						db.Exec("INSERT INTO Bible (Book, Chapter, Verse, Scripture) VALUES (?, ?, ?, ?)",
							bookNum, span.Ref.Chapter, span.Ref.Verse, cb.Text)
					}
				}
			}
		}
	}
}

func emitCommentaryNative(db *sql.DB, corpus *ir.Corpus) {
	db.Exec("CREATE TABLE IF NOT EXISTS Commentary (Book INTEGER, ChapterBegin INTEGER, ChapterEnd INTEGER, VerseBegin INTEGER, VerseEnd INTEGER, Comments TEXT)")

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
						db.Exec("INSERT INTO Commentary (Book, ChapterBegin, ChapterEnd, VerseBegin, VerseEnd, Comments) VALUES (?, ?, ?, ?, ?, ?)",
							bookNum, span.Ref.Chapter, span.Ref.Chapter, span.Ref.Verse, verseEnd, cb.Text)
					}
				}
			}
		}
	}
}

func emitDictionaryNative(db *sql.DB, corpus *ir.Corpus) {
	db.Exec("CREATE TABLE IF NOT EXISTS Dictionary (Topic TEXT, Definition TEXT)")

	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			topic := ""
			if t, ok := cb.Attributes["topic"].(string); ok {
				topic = t
			}
			db.Exec("INSERT INTO Dictionary (Topic, Definition) VALUES (?, ?)", topic, cb.Text)
		}
	}
}
