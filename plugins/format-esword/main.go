//go:build cgo

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

	_ "github.com/mattn/go-sqlite3"
)

// IPCRequest is the incoming JSON request.
type IPCRequest struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args,omitempty"`
}

// IPCResponse is the outgoing JSON response.
type IPCResponse struct {
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// DetectResult is the result of a detect command.
type DetectResult struct {
	Detected bool   `json:"detected"`
	Format   string `json:"format,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// IngestResult is the result of an ingest command.
type IngestResult struct {
	ArtifactID string            `json:"artifact_id"`
	BlobSHA256 string            `json:"blob_sha256"`
	SizeBytes  int64             `json:"size_bytes"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// EnumerateResult is the result of an enumerate command.
type EnumerateResult struct {
	Entries []EnumerateEntry `json:"entries"`
}

// EnumerateEntry represents a database table entry.
type EnumerateEntry struct {
	Path      string            `json:"path"`
	SizeBytes int64             `json:"size_bytes"`
	IsDir     bool              `json:"is_dir"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ExtractIRResult is the result of an extract-ir command.
type ExtractIRResult struct {
	IRPath     string      `json:"ir_path"`
	LossClass  string      `json:"loss_class"`
	LossReport *LossReport `json:"loss_report,omitempty"`
}

// EmitNativeResult is the result of an emit-native command.
type EmitNativeResult struct {
	OutputPath string      `json:"output_path"`
	Format     string      `json:"format"`
	LossClass  string      `json:"loss_class"`
	LossReport *LossReport `json:"loss_report,omitempty"`
}

// LossReport describes any data loss during conversion.
type LossReport struct {
	SourceFormat string        `json:"source_format"`
	TargetFormat string        `json:"target_format"`
	LossClass    string        `json:"loss_class"`
	LostElements []LostElement `json:"lost_elements,omitempty"`
	Warnings     []string      `json:"warnings,omitempty"`
}

// LostElement describes a specific element that was lost.
type LostElement struct {
	Path          string      `json:"path"`
	ElementType   string      `json:"element_type"`
	Reason        string      `json:"reason"`
	OriginalValue interface{} `json:"original_value,omitempty"`
}

// IR Types (matching core/ir package)
type Corpus struct {
	ID            string            `json:"id"`
	Version       string            `json:"version"`
	ModuleType    string            `json:"module_type"`
	Versification string            `json:"versification,omitempty"`
	Language      string            `json:"language,omitempty"`
	Title         string            `json:"title,omitempty"`
	Description   string            `json:"description,omitempty"`
	Publisher     string            `json:"publisher,omitempty"`
	Rights        string            `json:"rights,omitempty"`
	SourceFormat  string            `json:"source_format,omitempty"`
	Documents     []*Document       `json:"documents,omitempty"`
	SourceHash    string            `json:"source_hash,omitempty"`
	LossClass     string            `json:"loss_class,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type Document struct {
	ID            string            `json:"id"`
	Title         string            `json:"title,omitempty"`
	Order         int               `json:"order"`
	ContentBlocks []*ContentBlock   `json:"content_blocks,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type ContentBlock struct {
	ID         string                 `json:"id"`
	Sequence   int                    `json:"sequence"`
	Text       string                 `json:"text"`
	Tokens     []*Token               `json:"tokens,omitempty"`
	Anchors    []*Anchor              `json:"anchors,omitempty"`
	Hash       string                 `json:"hash,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type Token struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Text     string `json:"text"`
	StartPos int    `json:"start_pos"`
	EndPos   int    `json:"end_pos"`
}

type Anchor struct {
	ID       string  `json:"id"`
	Position int     `json:"position"`
	Spans    []*Span `json:"spans,omitempty"`
}

type Span struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	StartAnchorID string                 `json:"start_anchor_id"`
	EndAnchorID   string                 `json:"end_anchor_id,omitempty"`
	Ref           *Ref                   `json:"ref,omitempty"`
	Attributes    map[string]interface{} `json:"attributes,omitempty"`
}

type Ref struct {
	Book     string `json:"book"`
	Chapter  int    `json:"chapter,omitempty"`
	Verse    int    `json:"verse,omitempty"`
	VerseEnd int    `json:"verse_end,omitempty"`
	SubVerse string `json:"sub_verse,omitempty"`
	OSISID   string `json:"osis_id,omitempty"`
}

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
	var req IPCRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		respondError(fmt.Sprintf("failed to decode request: %v", err))
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
		respondError(fmt.Sprintf("unknown command: %s", req.Command))
	}
}

func handleDetect(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if info.IsDir() {
		respond(&DetectResult{
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
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("extension %s is not a known e-Sword format", ext),
		})
		return
	}

	// Verify it's a valid SQLite database
	db, err := sql.Open("sqlite3", path+"?mode=ro")
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open as SQLite: %v", err),
		})
		return
	}
	defer db.Close()

	// Try a simple query to verify it's a valid database
	if err := db.Ping(); err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("not a valid SQLite database: %v", err),
		})
		return
	}

	respond(&DetectResult{
		Detected: true,
		Format:   "e-Sword",
		Reason:   fmt.Sprintf("e-Sword %s database detected", moduleType),
	})
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	// Read file and compute hash
	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	// Write blob
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		respondError(fmt.Sprintf("failed to create blob dir: %v", err))
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write blob: %v", err))
		return
	}

	// Get artifact ID from filename
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	respond(&IngestResult{
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
		respondError("path argument required")
		return
	}

	db, err := sql.Open("sqlite3", path+"?mode=ro")
	if err != nil {
		respondError(fmt.Sprintf("failed to open database: %v", err))
		return
	}
	defer db.Close()

	// List all tables
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		respondError(fmt.Sprintf("failed to query tables: %v", err))
		return
	}
	defer rows.Close()

	var entries []EnumerateEntry
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}

		// Get row count for each table
		var count int64
		countRow := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %q", tableName))
		countRow.Scan(&count)

		entries = append(entries, EnumerateEntry{
			Path:      tableName,
			SizeBytes: count, // Using row count as "size"
			IsDir:     false,
			Metadata: map[string]string{
				"type":      "table",
				"row_count": fmt.Sprintf("%d", count),
			},
		})
	}

	respond(&EnumerateResult{
		Entries: entries,
	})
}

func handleExtractIR(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	artifactID := strings.TrimSuffix(filepath.Base(path), ext)

	// Read source for hashing
	sourceData, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read source: %v", err))
		return
	}
	sourceHash := sha256.Sum256(sourceData)

	db, err := sql.Open("sqlite3", path+"?mode=ro")
	if err != nil {
		respondError(fmt.Sprintf("failed to open database: %v", err))
		return
	}
	defer db.Close()

	corpus := &Corpus{
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

	var lostElements []LostElement

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
		respondError(fmt.Sprintf("failed to serialize IR: %v", err))
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write IR: %v", err))
		return
	}

	respond(&ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &LossReport{
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

func extractBibleIR(db *sql.DB, corpus *Corpus, lostElements *[]LostElement) {
	// Query Bible table: Book, Chapter, Verse, Scripture
	rows, err := db.Query("SELECT Book, Chapter, Verse, Scripture FROM Bible ORDER BY Book, Chapter, Verse")
	if err != nil {
		return
	}
	defer rows.Close()

	// Group by book
	bookDocs := make(map[int]*Document)
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
			doc = &Document{
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
		cb := &ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     text,
			Hash:     hex.EncodeToString(hash[:]),
			Anchors: []*Anchor{
				{
					ID:       fmt.Sprintf("a-%d-0", sequence),
					Position: 0,
					Spans: []*Span{
						{
							ID:            fmt.Sprintf("s-%s", refID),
							Type:          "VERSE",
							StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
							Ref: &Ref{
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
			*lostElements = append(*lostElements, LostElement{
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

func extractCommentaryIR(db *sql.DB, corpus *Corpus, lostElements *[]LostElement) {
	rows, err := db.Query("SELECT Book, ChapterBegin, ChapterEnd, VerseBegin, VerseEnd, Comments FROM Commentary ORDER BY Book, ChapterBegin, VerseBegin")
	if err != nil {
		return
	}
	defer rows.Close()

	sequence := 0
	doc := &Document{
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
		cb := &ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     text,
			Hash:     hex.EncodeToString(hash[:]),
			Attributes: map[string]interface{}{
				"type": "commentary",
			},
			Anchors: []*Anchor{
				{
					ID:       fmt.Sprintf("a-%d-0", sequence),
					Position: 0,
					Spans: []*Span{
						{
							ID:            fmt.Sprintf("s-%s", refID),
							Type:          "COMMENT",
							StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
							Ref: &Ref{
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

	corpus.Documents = []*Document{doc}
}

func extractDictionaryIR(db *sql.DB, corpus *Corpus, lostElements *[]LostElement) {
	rows, err := db.Query("SELECT Topic, Definition FROM Dictionary ORDER BY Topic")
	if err != nil {
		return
	}
	defer rows.Close()

	sequence := 0
	doc := &Document{
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
		cb := &ContentBlock{
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

	corpus.Documents = []*Document{doc}
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
		respondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	// Read IR file
	data, err := os.ReadFile(irPath)
	if err != nil {
		respondError(fmt.Sprintf("failed to read IR file: %v", err))
		return
	}

	var corpus Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		respondError(fmt.Sprintf("failed to parse IR: %v", err))
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
	db, err := sql.Open("sqlite3", outputPath)
	if err != nil {
		respondError(fmt.Sprintf("failed to create database: %v", err))
		return
	}
	defer db.Close()

	switch corpus.ModuleType {
	case "BIBLE":
		emitBibleNative(db, &corpus)
	case "COMMENTARY":
		emitCommentaryNative(db, &corpus)
	case "DICTIONARY":
		emitDictionaryNative(db, &corpus)
	default:
		emitBibleNative(db, &corpus)
	}

	// Create Details table with metadata
	db.Exec("CREATE TABLE IF NOT EXISTS Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version TEXT, Font TEXT, RightToLeft INTEGER)")
	title := corpus.Title
	if title == "" {
		title = corpus.ID
	}
	abbreviation := corpus.Attributes["abbreviation"]
	db.Exec("INSERT INTO Details (Title, Abbreviation, Information, Version, RightToLeft) VALUES (?, ?, ?, ?, ?)",
		title, abbreviation, corpus.Description, "1.0", 0)

	respond(&EmitNativeResult{
		OutputPath: outputPath,
		Format:     "e-Sword",
		LossClass:  "L1",
		LossReport: &LossReport{
			SourceFormat: "IR",
			TargetFormat: "e-Sword",
			LossClass:    "L1",
			Warnings: []string{
				"RTF formatting not recreated from plain text",
			},
		},
	})
}

func emitBibleNative(db *sql.DB, corpus *Corpus) {
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

func emitCommentaryNative(db *sql.DB, corpus *Corpus) {
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

func emitDictionaryNative(db *sql.DB, corpus *Corpus) {
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

func respond(result interface{}) {
	resp := IPCResponse{
		Status: "ok",
		Result: result,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
}

func respondError(msg string) {
	resp := IPCResponse{
		Status: "error",
		Error:  msg,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
	os.Exit(1)
}

// Compile check
var _ = io.Copy
