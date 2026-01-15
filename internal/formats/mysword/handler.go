// Package mysword provides the embedded handler for MySword Bible format plugin.
package mysword

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

// Handler implements the EmbeddedFormatHandler interface for MySword Bible.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.mysword",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-mysword",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"file"},
			Outputs: []string{"artifact.kind:mysword"},
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

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".mybible" {
		return &plugins.DetectResult{Detected: false, Reason: "not a .mybible file"}, nil
	}

	return &plugins.DetectResult{
		Detected: true,
		Format:   "mysword",
		Reason:   "MySword Bible file detected",
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
		Metadata:   map[string]string{"format": "mysword"},
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

	// Extract all verses
	verses, err := parser.GetAllVerses()
	if err != nil {
		return nil, fmt.Errorf("failed to extract verses: %w", err)
	}

	// Convert to IR corpus format
	corpus, lostElements := h.versesToIR(path, verses, parser)

	// Serialize to JSON
	irData, err := serializeCorpus(corpus)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize IR: %w", err)
	}

	// Write IR file
	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write IR file: %w", err)
	}

	return &plugins.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "MySword",
			TargetFormat: "IR",
			LossClass:    "L1",
			LostElements: lostElements,
			Warnings: []string{
				"HTML formatting in Scripture field simplified to plain text",
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
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	// Emit content based on module type
	var emitErr error
	switch corpus.ModuleType {
	case "BIBLE":
		emitErr = h.emitBibleNative(db, &corpus)
	case "COMMENTARY":
		emitErr = h.emitCommentaryNative(db, &corpus)
	case "DICTIONARY":
		emitErr = h.emitDictionaryNative(db, &corpus)
	default:
		emitErr = h.emitBibleNative(db, &corpus)
	}

	if emitErr != nil {
		return nil, fmt.Errorf("failed to emit content: %w", emitErr)
	}

	// Create info table with metadata
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS info (name TEXT, value TEXT)"); err != nil {
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
	if v, ok := corpus.Attributes["version"]; ok {
		db.Exec("INSERT INTO info (name, value) VALUES ('version', ?)", v)
	}
	if corpus.Language != "" {
		db.Exec("INSERT INTO info (name, value) VALUES ('language', ?)", corpus.Language)
	}

	return &plugins.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "MySword",
		LossClass:  "L1",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "IR",
			TargetFormat: "MySword",
			LossClass:    "L1",
			Warnings: []string{
				"HTML formatting not recreated from plain text",
			},
		},
	}, nil
}

// MySword book number to OSIS ID mapping
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

// versesToIR converts verses to IR corpus format
func (h *Handler) versesToIR(path string, verses []Verse, parser *Parser) (*ipc.Corpus, []plugins.LostElementIPC) {
	// Read source for hashing
	sourceData, _ := os.ReadFile(path)
	sourceHash := sha256.Sum256(sourceData)

	// Extract artifact ID from filename
	artifactID := filepath.Base(path)
	for strings.Contains(artifactID, ".") {
		artifactID = strings.TrimSuffix(artifactID, filepath.Ext(artifactID))
	}

	corpus := &ipc.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "MySword",
		LossClass:    "L1",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		Attributes:   make(map[string]string),
	}

	// Get metadata from parser
	if title := parser.GetMetadata("description"); title != "" {
		corpus.Title = title
	}
	if desc := parser.GetMetadata("detailed_info"); desc != "" {
		corpus.Description = desc
	}
	if version := parser.GetMetadata("version"); version != "" {
		corpus.Attributes["version"] = version
	}
	if lang := parser.GetMetadata("language"); lang != "" {
		corpus.Language = lang
	}

	var lostElements []plugins.LostElementIPC

	// Group verses by book
	bookDocs := make(map[int]*ipc.Document)
	sequence := 0

	for _, v := range verses {
		// Get or create document for this book
		doc, ok := bookDocs[v.Book]
		if !ok {
			osisID := bookNumToOSIS[v.Book]
			if osisID == "" {
				osisID = fmt.Sprintf("Book%d", v.Book)
			}
			doc = &ipc.Document{
				ID:         osisID,
				Title:      osisID,
				Order:      v.Book,
				Attributes: map[string]string{"book_num": fmt.Sprintf("%d", v.Book)},
			}
			bookDocs[v.Book] = doc
		}

		sequence++
		osisID := bookNumToOSIS[v.Book]
		if osisID == "" {
			osisID = fmt.Sprintf("Book%d", v.Book)
		}
		refID := fmt.Sprintf("%s.%d.%d", osisID, v.Chapter, v.Verse)

		// Create content block for verse
		hash := sha256.Sum256([]byte(v.Text))
		cb := &ipc.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     v.Text,
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
								Chapter: v.Chapter,
								Verse:   v.Verse,
								OSISID:  refID,
							},
						},
					},
				},
			},
		}

		doc.ContentBlocks = append(doc.ContentBlocks, cb)
	}

	// Add documents to corpus in order
	for i := 1; i <= 66; i++ {
		if doc, ok := bookDocs[i]; ok {
			corpus.Documents = append(corpus.Documents, doc)
		}
	}

	return corpus, lostElements
}

// serializeCorpus serializes a corpus to JSON
func serializeCorpus(corpus *ipc.Corpus) ([]byte, error) {
	return json.MarshalIndent(corpus, "", "  ")
}

// emitBibleNative emits a Bible corpus to MySword format
func (h *Handler) emitBibleNative(db *sql.DB, corpus *ipc.Corpus) error {
	// Create Books table
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

// emitCommentaryNative emits a commentary corpus to MySword format
func (h *Handler) emitCommentaryNative(db *sql.DB, corpus *ipc.Corpus) error {
	// Create commentaries table
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

// emitDictionaryNative emits a dictionary corpus to MySword format
func (h *Handler) emitDictionaryNative(db *sql.DB, corpus *ipc.Corpus) error {
	// Create dictionary table
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
