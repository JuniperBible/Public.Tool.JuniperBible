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
	moduleType, ok := getMySwordModuleType(base)
	if !ok {
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
	hasBooksTable, hasInfoTable := checkMySwordTables(db)

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

// getMySwordModuleType extracts the module type from a MySword file extension.
// Returns the module type ("bible", "commentary", "dictionary") and whether the file is valid.
func getMySwordModuleType(base string) (moduleType string, ok bool) {
	if !strings.HasSuffix(base, ".mybible") {
		return "", false
	}

	if strings.HasSuffix(base, ".commentaries.mybible") {
		return "commentary", true
	}

	if strings.HasSuffix(base, ".dictionary.mybible") {
		return "dictionary", true
	}

	return "bible", true
}

// checkMySwordTables checks for the presence of MySword-specific tables in the database.
// Returns whether Books/Bible table exists and whether info/Details table exists.
func checkMySwordTables(db *sql.DB) (hasBooksTable, hasInfoTable bool) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		return false, false
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}

		if strings.EqualFold(name, "Books") || strings.EqualFold(name, "Bible") {
			hasBooksTable = true
		}

		if strings.EqualFold(name, "info") || strings.EqualFold(name, "Details") {
			hasInfoTable = true
		}
	}

	return hasBooksTable, hasInfoTable
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
	path, outputDir, ok := extractIRArgs(args)
	if !ok {
		return
	}

	artifactID := extractArtifactID(path)
	sourceHash, err := hashSourceFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read source: %v", err)
		return
	}

	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		ipc.RespondErrorf("failed to open database: %v", err)
		return
	}
	defer db.Close()

	corpus := createCorpus(artifactID, sourceHash, path)
	var lostElements []ipc.LostElement
	extractContentByType(db, corpus, &lostElements, path)
	extractMetadata(db, corpus)

	irPath, err := writeIRFile(outputDir, corpus)
	if err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}

	respondExtractIR(irPath, lostElements)
}

// extractIRArgs validates and extracts path and output_dir from args
func extractIRArgs(args map[string]interface{}) (path, outputDir string, ok bool) {
	path, ok = args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return "", "", false
	}

	outputDir, ok = args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return "", "", false
	}

	return path, outputDir, true
}

// extractArtifactID derives artifact ID from file path by removing extensions
func extractArtifactID(path string) string {
	artifactID := filepath.Base(path)
	for strings.Contains(artifactID, ".") {
		artifactID = strings.TrimSuffix(artifactID, filepath.Ext(artifactID))
	}
	return artifactID
}

// hashSourceFile reads and computes SHA256 hash of source file
func hashSourceFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// createCorpus initializes a corpus with basic metadata
func createCorpus(artifactID, sourceHash, path string) *ipc.Corpus {
	return &ipc.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "MySword",
		LossClass:    "L1",
		SourceHash:   sourceHash,
		Attributes:   make(map[string]string),
	}
}

// extractContentByType routes to appropriate extraction function based on file type
func extractContentByType(db *sql.DB, corpus *ipc.Corpus, lostElements *[]ipc.LostElement, path string) {
	base := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(base, ".commentaries.mybible") {
		corpus.ModuleType = "COMMENTARY"
		extractCommentaryIR(db, corpus, lostElements)
	} else if strings.HasSuffix(base, ".dictionary.mybible") {
		corpus.ModuleType = "DICTIONARY"
		extractDictionaryIR(db, corpus, lostElements)
	} else {
		corpus.ModuleType = "BIBLE"
		extractBibleIR(db, corpus, lostElements)
	}
}

// extractMetadata retrieves module metadata from database tables
func extractMetadata(db *sql.DB, corpus *ipc.Corpus) {
	// Try MySword info table first
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
		return
	}

	// Fall back to e-Sword Details table
	var title, abbreviation, info string
	row = db.QueryRow("SELECT Title, Abbreviation, Information FROM Details LIMIT 1")
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

// writeIRFile serializes corpus to JSON and writes to output directory
func writeIRFile(outputDir string, corpus *ipc.Corpus) (string, error) {
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize IR: %w", err)
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		return "", err
	}

	return irPath, nil
}

// respondExtractIR sends the extract IR response
func respondExtractIR(irPath string, lostElements []ipc.LostElement) {
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
	tableName := determineBibleTableName(db)
	rows, err := db.Query(fmt.Sprintf("SELECT Book, Chapter, Verse, Scripture FROM %s ORDER BY Book, Chapter, Verse", tableName))
	if err != nil {
		return
	}
	defer rows.Close()

	bookDocs := make(map[int]*ipc.Document)
	sequence := 0

	for rows.Next() {
		sequence++
		processVerseRow(rows, bookDocs, &sequence, lostElements)
	}

	addBookDocumentsToCorpus(corpus, bookDocs)
}

// determineBibleTableName returns "Books" if it exists, otherwise "Bible"
func determineBibleTableName(db *sql.DB) string {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM Books").Scan(&count); err != nil {
		return "Bible"
	}
	return "Books"
}

// getOrCreateBookDocument retrieves or creates a document for the given book number
func getOrCreateBookDocument(bookDocs map[int]*ipc.Document, bookNum int) *ipc.Document {
	if doc, ok := bookDocs[bookNum]; ok {
		return doc
	}

	osisID := getOSISIDForBook(bookNum)
	doc := &ipc.Document{
		ID:         osisID,
		Title:      osisID,
		Order:      bookNum,
		Attributes: map[string]string{"book_num": fmt.Sprintf("%d", bookNum)},
	}
	bookDocs[bookNum] = doc
	return doc
}

// getOSISIDForBook returns the OSIS ID for a book number, or a fallback
func getOSISIDForBook(bookNum int) string {
	if osisID := bookNumToOSIS[bookNum]; osisID != "" {
		return osisID
	}
	return fmt.Sprintf("Book%d", bookNum)
}

// createVerseContentBlock creates a content block for a verse
func createVerseContentBlock(sequence int, bookNum, chapter, verse int, text string) *ipc.ContentBlock {
	osisID := getOSISIDForBook(bookNum)
	refID := fmt.Sprintf("%s.%d.%d", osisID, chapter, verse)
	hash := sha256.Sum256([]byte(text))

	return &ipc.ContentBlock{
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
}

// trackHTMLLoss records when HTML formatting is lost during extraction
func trackHTMLLoss(scripture, text string, refID string, lostElements *[]ipc.LostElement) {
	if scripture != text && (strings.Contains(scripture, "<") || strings.Contains(scripture, "&")) {
		*lostElements = append(*lostElements, ipc.LostElement{
			Path:        refID,
			ElementType: "html-formatting",
			Reason:      "HTML formatting stripped during extraction",
		})
	}
}

// processVerseRow processes a single verse row from the database
func processVerseRow(rows *sql.Rows, bookDocs map[int]*ipc.Document, sequence *int, lostElements *[]ipc.LostElement) {
	var bookNum, chapter, verse int
	var scripture string
	if err := rows.Scan(&bookNum, &chapter, &verse, &scripture); err != nil {
		return
	}

	doc := getOrCreateBookDocument(bookDocs, bookNum)
	text := stripHTML(scripture)
	cb := createVerseContentBlock(*sequence, bookNum, chapter, verse, text)

	osisID := getOSISIDForBook(bookNum)
	refID := fmt.Sprintf("%s.%d.%d", osisID, chapter, verse)
	trackHTMLLoss(scripture, text, refID, lostElements)

	doc.ContentBlocks = append(doc.ContentBlocks, cb)
}

// addBookDocumentsToCorpus adds book documents to the corpus in canonical order
func addBookDocumentsToCorpus(corpus *ipc.Corpus, bookDocs map[int]*ipc.Document) {
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
