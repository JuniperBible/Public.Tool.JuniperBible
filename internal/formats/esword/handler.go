// Package esword provides the embedded handler for e-Sword Bible format plugin.
package esword

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
	"github.com/FocuswithJustin/JuniperBible/core/plugins"
	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
)

// Handler implements the EmbeddedFormatHandler interface for e-Sword Bible.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.esword",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-esword",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"file"},
			Outputs: []string{"artifact.kind:esword"},
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
	validExts := map[string]string{
		".bblx": "e-Sword Bible file",
		".cmtx": "e-Sword Commentary file",
		".dctx": "e-Sword Dictionary file",
	}

	reason, ok := validExts[ext]
	if !ok {
		return &plugins.DetectResult{Detected: false, Reason: fmt.Sprintf("not an e-Sword file (expected .bblx, .cmtx, or .dctx)")}, nil
	}

	return &plugins.DetectResult{
		Detected: true,
		Format:   "esword",
		Reason:   reason + " detected",
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
		Metadata:   map[string]string{"format": "esword"},
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
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".bblx":
		return h.extractBibleIR(path, outputDir)
	case ".cmtx":
		return h.extractCommentaryIR(path, outputDir)
	case ".dctx":
		return h.extractDictionaryIR(path, outputDir)
	default:
		return nil, fmt.Errorf("unsupported e-Sword file type: %s", ext)
	}
}

// EmitNative implements EmbeddedFormatHandler.EmitNative.
func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	// Read IR file
	data, err := os.ReadFile(irPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read IR file: %w", err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return nil, fmt.Errorf("failed to parse IR: %w", err)
	}

	// Determine output file extension
	ext := ".bblx"
	switch corpus.ModuleType {
	case ir.ModuleCommentary:
		ext = ".cmtx"
	case ir.ModuleDictionary:
		ext = ".dctx"
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, corpus.ID+ext)

	// Create new SQLite database
	db, err := sqlite.Open(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	var emitErr error
	switch corpus.ModuleType {
	case ir.ModuleBible:
		emitErr = h.emitBibleNative(db, &corpus)
	case ir.ModuleCommentary:
		emitErr = h.emitCommentaryNative(db, &corpus)
	case ir.ModuleDictionary:
		emitErr = h.emitDictionaryNative(db, &corpus)
	default:
		emitErr = h.emitBibleNative(db, &corpus)
	}
	if emitErr != nil {
		return nil, fmt.Errorf("failed to emit content: %w", emitErr)
	}

	// Create Details table with metadata
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version TEXT, Font TEXT, RightToLeft INTEGER)"); err != nil {
		return nil, fmt.Errorf("failed to create Details table: %w", err)
	}
	title := corpus.Title
	if title == "" {
		title = corpus.ID
	}
	abbreviation := corpus.Attributes["abbreviation"]
	if _, err := db.Exec("INSERT INTO Details (Title, Abbreviation, Information, Version, RightToLeft) VALUES (?, ?, ?, ?, ?)",
		title, abbreviation, corpus.Description, "1.0", 0); err != nil {
		return nil, fmt.Errorf("failed to insert Details: %w", err)
	}

	return &plugins.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "e-Sword",
		LossClass:  "L1",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "IR",
			TargetFormat: "e-Sword",
			LossClass:    "L1",
			Warnings: []string{
				"RTF formatting not recreated from plain text",
			},
		},
	}, nil
}

// extractBibleIR extracts IR from a .bblx file.
func (h *Handler) extractBibleIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	parser, err := NewBibleParser(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create Bible parser: %w", err)
	}
	defer parser.Close()

	// Compute source hash
	sourceData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file: %w", err)
	}
	sourceHash := sha256.Sum256(sourceData)

	// Create corpus
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	corpus := &ir.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   ir.ModuleBible,
		SourceFormat: "e-Sword",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    ir.LossL1,
		Attributes:   make(map[string]string),
	}

	// Get metadata
	metadata := parser.GetMetadata()
	if metadata != nil {
		if metadata.Title != "" {
			corpus.Title = metadata.Title
		}
		if metadata.Abbreviation != "" {
			corpus.Attributes["abbreviation"] = metadata.Abbreviation
		}
		if metadata.Information != "" {
			corpus.Description = metadata.Information
		}
		if metadata.Version != "" {
			corpus.Attributes["version"] = metadata.Version
		}
		if metadata.Font != "" {
			corpus.Attributes["font"] = metadata.Font
		}
		if metadata.RightToLeft {
			corpus.Attributes["right_to_left"] = "true"
		}
	}

	// Get all verses
	verses, err := parser.GetAllVerses()
	if err != nil {
		return nil, fmt.Errorf("failed to get verses: %w", err)
	}

	// Group verses by book
	bookDocs := make(map[int]*ir.Document)
	sequence := 0

	for _, verse := range verses {
		// Get or create document for this book
		doc, ok := bookDocs[verse.Book]
		if !ok {
			osisID := bookNumToOSIS(verse.Book)
			doc = &ir.Document{
				ID:         osisID,
				Title:      osisID,
				Order:      verse.Book,
				Attributes: map[string]string{"book_num": fmt.Sprintf("%d", verse.Book)},
			}
			bookDocs[verse.Book] = doc
		}

		sequence++
		osisID := bookNumToOSIS(verse.Book)
		refID := fmt.Sprintf("%s.%d.%d", osisID, verse.Chapter, verse.Verse)

		// Create content block for verse
		hash := sha256.Sum256([]byte(verse.Scripture))
		cb := &ir.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     verse.Scripture,
			Hash:     hex.EncodeToString(hash[:]),
			Anchors: []*ir.Anchor{
				{
					ID:       fmt.Sprintf("a-%d-0", sequence),
					Position: 0,
					Spans: []*ir.Span{
						{
							ID:            fmt.Sprintf("s-%s", refID),
							Type:          ir.SpanVerse,
							StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
							Ref: &ir.Ref{
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

	// Add documents to corpus in book order
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

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write IR: %w", err)
	}

	return &plugins.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "e-Sword",
			TargetFormat: "IR",
			LossClass:    "L1",
			Warnings: []string{
				"RTF formatting in Scripture field is simplified to plain text",
			},
		},
	}, nil
}

// extractCommentaryIR extracts IR from a .cmtx file.
func (h *Handler) extractCommentaryIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	parser, err := NewCommentaryParser(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create Commentary parser: %w", err)
	}
	defer parser.Close()

	// Compute source hash
	sourceData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file: %w", err)
	}
	sourceHash := sha256.Sum256(sourceData)

	// Create corpus
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	corpus := &ir.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   ir.ModuleCommentary,
		SourceFormat: "e-Sword",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    ir.LossL1,
		Attributes:   make(map[string]string),
	}

	// Get module info
	info := parser.ModuleInfo()
	if info.Title != "" {
		corpus.Title = info.Title
	}

	// Create a single document for all commentary entries
	doc := &ir.Document{
		ID:    "commentary",
		Title: "Commentary",
		Order: 1,
	}

	sequence := 0
	books := parser.ListBooks()
	for _, bookNum := range books {
		entries := parser.GetBook(bookNum)
		for _, entry := range entries {
			sequence++
			osisID := bookNumToOSIS(entry.Book)

			// Build reference ID
			refID := fmt.Sprintf("%s.%d.%d", osisID, entry.ChapterStart, entry.VerseStart)
			if entry.ChapterEnd != entry.ChapterStart || entry.VerseEnd != entry.VerseStart {
				refID = fmt.Sprintf("%s.%d.%d-%s.%d.%d", osisID, entry.ChapterStart, entry.VerseStart, osisID, entry.ChapterEnd, entry.VerseEnd)
			}

			// Create content block
			hash := sha256.Sum256([]byte(entry.Comments))
			cb := &ir.ContentBlock{
				ID:       fmt.Sprintf("cb-%d", sequence),
				Sequence: sequence,
				Text:     entry.Comments,
				Hash:     hex.EncodeToString(hash[:]),
				Attributes: map[string]interface{}{
					"type": "commentary",
				},
				Anchors: []*ir.Anchor{
					{
						ID:       fmt.Sprintf("a-%d-0", sequence),
						Position: 0,
						Spans: []*ir.Span{
							{
								ID:            fmt.Sprintf("s-%s", refID),
								Type:          "COMMENT",
								StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
								Ref: &ir.Ref{
									Book:     osisID,
									Chapter:  entry.ChapterStart,
									Verse:    entry.VerseStart,
									VerseEnd: entry.VerseEnd,
									OSISID:   refID,
								},
							},
						},
					},
				},
			}

			doc.ContentBlocks = append(doc.ContentBlocks, cb)
		}
	}

	corpus.Documents = []*ir.Document{doc}

	// Serialize IR to JSON
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize IR: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write IR: %w", err)
	}

	return &plugins.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "e-Sword",
			TargetFormat: "IR",
			LossClass:    "L1",
			Warnings: []string{
				"RTF formatting in Comments field is simplified to plain text",
			},
		},
	}, nil
}

// extractDictionaryIR extracts IR from a .dctx file.
func (h *Handler) extractDictionaryIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	parser, err := NewDictionaryParser(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create Dictionary parser: %w", err)
	}
	defer parser.Close()

	// Compute source hash
	sourceData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file: %w", err)
	}
	sourceHash := sha256.Sum256(sourceData)

	// Create corpus
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	corpus := &ir.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   ir.ModuleDictionary,
		SourceFormat: "e-Sword",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    ir.LossL1,
		Attributes:   make(map[string]string),
	}

	// Get module info
	info := parser.ModuleInfo()
	if info.Title != "" {
		corpus.Title = info.Title
	}

	// Create a single document for all dictionary entries
	doc := &ir.Document{
		ID:    "dictionary",
		Title: "Dictionary",
		Order: 1,
	}

	sequence := 0
	topics := parser.ListTopicsSorted()
	for _, topic := range topics {
		entry, err := parser.GetEntry(topic)
		if err != nil {
			continue
		}

		sequence++

		// Create content block
		hash := sha256.Sum256([]byte(entry.Definition))
		cb := &ir.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     entry.Definition,
			Hash:     hex.EncodeToString(hash[:]),
			Attributes: map[string]interface{}{
				"topic": entry.Topic,
				"type":  "dictionary",
			},
		}

		doc.ContentBlocks = append(doc.ContentBlocks, cb)
	}

	corpus.Documents = []*ir.Document{doc}

	// Serialize IR to JSON
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize IR: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write IR: %w", err)
	}

	return &plugins.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "e-Sword",
			TargetFormat: "IR",
			LossClass:    "L1",
			Warnings: []string{
				"RTF formatting in Definition field is simplified to plain text",
			},
		},
	}, nil
}

// bookNumToOSIS converts an e-Sword book number to OSIS ID.
func bookNumToOSIS(bookNum int) string {
	bookMap := map[int]string{
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

	if osis, ok := bookMap[bookNum]; ok {
		return osis
	}
	return fmt.Sprintf("Book%d", bookNum)
}

// osisToBookNum converts an OSIS ID to e-Sword book number.
func osisToBookNum(osisID string) int {
	osisMap := map[string]int{
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
		"Phil": 50, "Col": 51, "1Thess": 52, "2Thess": 53,
		"1Tim": 54, "2Tim": 55, "Titus": 56, "Phlm": 57, "Heb": 58,
		"Jas": 59, "1Pet": 60, "2Pet": 61, "1John": 62, "2John": 63,
		"3John": 64, "Jude": 65, "Rev": 66,
	}

	if num, ok := osisMap[osisID]; ok {
		return num
	}
	return 0
}

// emitBibleNative creates a Bible table from IR.
func (h *Handler) emitBibleNative(db *sql.DB, corpus *ir.Corpus) error {
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS Bible (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)"); err != nil {
		return fmt.Errorf("create Bible table: %w", err)
	}

	for _, doc := range corpus.Documents {
		bookNum := 0
		if num, ok := doc.Attributes["book_num"]; ok {
			fmt.Sscanf(num, "%d", &bookNum)
		} else {
			bookNum = osisToBookNum(doc.ID)
		}

		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && (span.Type == ir.SpanVerse || span.Type == "VERSE") {
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

// emitCommentaryNative creates a Commentary table from IR.
func (h *Handler) emitCommentaryNative(db *sql.DB, corpus *ir.Corpus) error {
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS Commentary (Book INTEGER, ChapterBegin INTEGER, ChapterEnd INTEGER, VerseBegin INTEGER, VerseEnd INTEGER, Comments TEXT)"); err != nil {
		return fmt.Errorf("create Commentary table: %w", err)
	}

	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil {
						bookNum := osisToBookNum(span.Ref.Book)
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

// emitDictionaryNative creates a Dictionary table from IR.
func (h *Handler) emitDictionaryNative(db *sql.DB, corpus *ir.Corpus) error {
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
