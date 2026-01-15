// Package mybible provides the embedded handler for MyBible.zone Bible format plugin.
package mybible

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// Handler implements the EmbeddedFormatHandler interface for MyBible.zone Bible.
type Handler struct{}

// MyBible book number to OSIS ID mapping
var bookNumToOSISMap = map[int]string{
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

var osisToBookNumMap = func() map[string]int {
	m := make(map[string]int)
	for k, v := range bookNumToOSISMap {
		m[v] = k
	}
	return m
}()

// bookNumToOSIS converts a MyBible book number to OSIS ID.
func bookNumToOSIS(bookNum int) string {
	if osis, ok := bookNumToOSISMap[bookNum]; ok {
		return osis
	}
	return fmt.Sprintf("Book%d", bookNum)
}

// osisToBookNum converts an OSIS ID to MyBible book number.
func osisToBookNum(osisID string) int {
	if num, ok := osisToBookNumMap[osisID]; ok {
		return num
	}
	return 0
}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.mybible",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-mybible",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"file"},
			Outputs: []string{"artifact.kind:mybible"},
		},
	}
}

// Register registers this plugin with the embedded registry.
func Register() {
	plugins.RegisterEmbeddedPlugin(&plugins.EmbeddedPlugin{
		Manifest: Manifest(),
		Format:   &Handler{},
	})
}

func init() {
	Register()
}

// Detect implements EmbeddedFormatHandler.Detect.
func (h *Handler) Detect(path string) (*plugins.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &plugins.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}, nil
	}

	if info.IsDir() {
		return &plugins.DetectResult{Detected: false, Reason: "path is a directory"}, nil
	}

	// MyBible.zone uses .SQLite3 extension
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))
	if ext != ".sqlite3" && !strings.HasSuffix(base, ".sqlite3") {
		return &plugins.DetectResult{Detected: false, Reason: "not a .SQLite3 file"}, nil
	}

	return &plugins.DetectResult{
		Detected: true,
		Format:   "mybible",
		Reason:   "MyBible.zone database file detected",
	}, nil
}

// Ingest implements EmbeddedFormatHandler.Ingest.
func (h *Handler) Ingest(path, outputDir string) (*plugins.IngestResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create blob dir: %w", err)
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write blob: %w", err)
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return &plugins.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata:   map[string]string{"format": "mybible"},
	}, nil
}

// Enumerate implements EmbeddedFormatHandler.Enumerate.
func (h *Handler) Enumerate(path string) (*plugins.EnumerateResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	return &plugins.EnumerateResult{
		Entries: []plugins.EnumerateEntry{
			{Path: filepath.Base(path), SizeBytes: info.Size(), IsDir: false},
		},
	}, nil
}

// ExtractIR implements EmbeddedFormatHandler.ExtractIR.
func (h *Handler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	// Create parser
	parser, err := NewParser(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}
	defer parser.Close()

	// Get artifact ID from filename
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	// Read source for hashing
	sourceData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read source: %w", err)
	}
	sourceHash := sha256.Sum256(sourceData)

	// Create corpus
	corpus := &ipc.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "mybible",
		LossClass:    "L1",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		Attributes:   make(map[string]string),
	}

	// Extract metadata
	if title := parser.GetMetadata("description"); title != "" {
		corpus.Title = title
	}
	if desc := parser.GetMetadata("detailed_info"); desc != "" {
		corpus.Description = desc
	}
	if lang := parser.GetMetadata("language"); lang != "" {
		corpus.Language = lang
	}
	if version := parser.GetMetadata("version"); version != "" {
		corpus.Attributes["version"] = version
	}

	// Get all verses
	verses, err := parser.GetAllVerses()
	if err != nil {
		return nil, fmt.Errorf("failed to get verses: %w", err)
	}

	// Convert verses to IR format
	var lostElements []plugins.LostElementIPC
	bookDocs := make(map[int]*ipc.Document)
	sequence := 0

	for _, verse := range verses {
		// Get or create document for this book
		doc, ok := bookDocs[verse.BookNumber]
		if !ok {
			osisID := bookNumToOSIS(verse.BookNumber)
			doc = &ipc.Document{
				ID:         osisID,
				Title:      osisID,
				Order:      verse.BookNumber,
				Attributes: map[string]string{"book_num": fmt.Sprintf("%d", verse.BookNumber)},
			}
			bookDocs[verse.BookNumber] = doc
		}

		sequence++
		osisID := bookNumToOSIS(verse.BookNumber)
		refID := fmt.Sprintf("%s.%d.%d", osisID, verse.Chapter, verse.Verse)

		// Create content block for verse
		hash := sha256.Sum256([]byte(verse.Text))
		cb := &ipc.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     verse.Text,
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
								Chapter: verse.Chapter,
								Verse:   verse.Verse,
								OSISID:  refID,
							},
						},
					},
				},
			},
		}

		doc.ContentBlocks = append(doc.ContentBlocks, cb)
	}

	// Add documents to corpus in order (1-66 for standard Bible books)
	for i := 1; i <= 66; i++ {
		if doc, ok := bookDocs[i]; ok {
			corpus.Documents = append(corpus.Documents, doc)
		}
	}

	// Serialize IR to JSON
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize IR: %w", err)
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write IR: %w", err)
	}

	return &plugins.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "mybible",
			TargetFormat: "IR",
			LossClass:    "L1",
			LostElements: lostElements,
			Warnings: []string{
				"HTML formatting in text field simplified to plain text",
			},
		},
	}, nil
}

// EmitNative implements EmbeddedFormatHandler.EmitNative.
func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	// Read IR file
	data, err := os.ReadFile(irPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read IR file: %w", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return nil, fmt.Errorf("failed to parse IR: %w", err)
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".SQLite3")

	// Create new SQLite database
	db, err := sqlite.Open(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	// Create verses table (MyBible.zone schema)
	if _, err := db.Exec(`CREATE TABLE verses (
		book_number INTEGER NOT NULL,
		chapter INTEGER NOT NULL,
		verse INTEGER NOT NULL,
		text TEXT NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("failed to create verses table: %w", err)
	}

	// Create indexes for performance
	db.Exec("CREATE INDEX book_number_index ON verses (book_number)")
	db.Exec("CREATE INDEX chapter_index ON verses (chapter)")
	db.Exec("CREATE INDEX verse_index ON verses (verse)")

	// Emit Bible content
	if err := emitBibleNative(db, &corpus); err != nil {
		return nil, fmt.Errorf("failed to emit content: %w", err)
	}

	// Create info table with metadata (MyBible.zone style: name-value pairs)
	if _, err := db.Exec("CREATE TABLE info (name TEXT NOT NULL, value TEXT NOT NULL)"); err != nil {
		return nil, fmt.Errorf("failed to create info table: %w", err)
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

	return &plugins.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "mybible",
		LossClass:  "L1",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "IR",
			TargetFormat: "mybible",
			LossClass:    "L1",
			Warnings: []string{
				"HTML formatting not recreated from plain text",
			},
		},
	}, nil
}

// emitBibleNative converts IR documents to MyBible verses table.
func emitBibleNative(db *sql.DB, corpus *ipc.Corpus) error {
	for _, doc := range corpus.Documents {
		bookNum := 0
		if num, ok := doc.Attributes["book_num"]; ok {
			fmt.Sscanf(num, "%d", &bookNum)
		} else if num := osisToBookNum(doc.ID); num > 0 {
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
