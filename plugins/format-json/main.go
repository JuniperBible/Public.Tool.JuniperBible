//go:build !sdk

// Plugin format-json handles JSON Bible format.
// Provides clean JSON structure for programmatic access.
//
// IR Support:
// - extract-ir: Reads JSON Bible format back to IR (L0)
// - emit-native: Converts IR to clean JSON format (L0)
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// EnumerateEntry represents a file entry.
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

// JSONBible is the clean output format
type JSONBible struct {
	Meta   JSONMeta    `json:"meta"`
	Books  []JSONBook  `json:"books"`
	Verses []JSONVerse `json:"verses,omitempty"` // Flat verse list for easy access
}

type JSONMeta struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Language    string `json:"language,omitempty"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

type JSONBook struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Order    int           `json:"order"`
	Chapters []JSONChapter `json:"chapters"`
}

type JSONChapter struct {
	Number int         `json:"number"`
	Verses []JSONVerse `json:"verses"`
}

type JSONVerse struct {
	Book    string `json:"book"`
	Chapter int    `json:"chapter"`
	Verse   int    `json:"verse"`
	Text    string `json:"text"`
	ID      string `json:"id"` // OSIS ID like "Gen.1.1"
}

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
			Reason:   "path is a directory",
		})
		return
	}

	// Check extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".json" {
		respond(&DetectResult{
			Detected: false,
			Reason:   "not a .json file",
		})
		return
	}

	// Try to parse as JSONBible
	data, err := os.ReadFile(path)
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		})
		return
	}

	var jsonBible JSONBible
	if err := json.Unmarshal(data, &jsonBible); err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   "not valid JSON",
		})
		return
	}

	// Check for our format markers
	if jsonBible.Meta.ID == "" && len(jsonBible.Books) == 0 && len(jsonBible.Verses) == 0 {
		respond(&DetectResult{
			Detected: false,
			Reason:   "not a Capsule JSON Bible format",
		})
		return
	}

	respond(&DetectResult{
		Detected: true,
		Format:   "JSON",
		Reason:   "Capsule JSON Bible format detected",
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

	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

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

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	respond(&IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "JSON",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to stat: %v", err))
		return
	}

	entries := []EnumerateEntry{
		{
			Path:      filepath.Base(path),
			SizeBytes: info.Size(),
			IsDir:     false,
			Metadata: map[string]string{
				"format": "JSON",
			},
		},
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

	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	sourceHash := sha256.Sum256(data)

	var jsonBible JSONBible
	if err := json.Unmarshal(data, &jsonBible); err != nil {
		respondError(fmt.Sprintf("failed to parse JSON: %v", err))
		return
	}

	// Convert JSONBible to IR Corpus
	corpus := &Corpus{
		ID:           jsonBible.Meta.ID,
		Version:      jsonBible.Meta.Version,
		ModuleType:   "BIBLE",
		Title:        jsonBible.Meta.Title,
		Language:     jsonBible.Meta.Language,
		Description:  jsonBible.Meta.Description,
		SourceFormat: "JSON",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L0",
		Attributes:   make(map[string]string),
	}

	// Store raw for L0 round-trip
	corpus.Attributes["_json_raw"] = string(data)

	sequence := 0
	for _, book := range jsonBible.Books {
		doc := &Document{
			ID:    book.ID,
			Title: book.Name,
			Order: book.Order,
		}

		for _, chapter := range book.Chapters {
			for _, verse := range chapter.Verses {
				sequence++
				hash := sha256.Sum256([]byte(verse.Text))
				osisID := fmt.Sprintf("%s.%d.%d", book.ID, chapter.Number, verse.Verse)

				cb := &ContentBlock{
					ID:       fmt.Sprintf("cb-%d", sequence),
					Sequence: sequence,
					Text:     verse.Text,
					Hash:     hex.EncodeToString(hash[:]),
					Anchors: []*Anchor{
						{
							ID:       fmt.Sprintf("a-%d-0", sequence),
							Position: 0,
							Spans: []*Span{
								{
									ID:            fmt.Sprintf("s-%s", osisID),
									Type:          "VERSE",
									StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
									Ref: &Ref{
										Book:    book.ID,
										Chapter: chapter.Number,
										Verse:   verse.Verse,
										OSISID:  osisID,
									},
								},
							},
						},
					},
				}
				doc.ContentBlocks = append(doc.ContentBlocks, cb)
			}
		}

		corpus.Documents = append(corpus.Documents, doc)
	}

	// Also handle flat verses if present
	if len(jsonBible.Books) == 0 && len(jsonBible.Verses) > 0 {
		bookDocs := make(map[string]*Document)
		for _, verse := range jsonBible.Verses {
			doc, ok := bookDocs[verse.Book]
			if !ok {
				doc = &Document{
					ID:    verse.Book,
					Title: verse.Book,
					Order: len(bookDocs) + 1,
				}
				bookDocs[verse.Book] = doc
				corpus.Documents = append(corpus.Documents, doc)
			}

			sequence++
			hash := sha256.Sum256([]byte(verse.Text))

			cb := &ContentBlock{
				ID:       fmt.Sprintf("cb-%d", sequence),
				Sequence: sequence,
				Text:     verse.Text,
				Hash:     hex.EncodeToString(hash[:]),
				Anchors: []*Anchor{
					{
						ID:       fmt.Sprintf("a-%d-0", sequence),
						Position: 0,
						Spans: []*Span{
							{
								ID:            fmt.Sprintf("s-%s", verse.ID),
								Type:          "VERSE",
								StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
								Ref: &Ref{
									Book:    verse.Book,
									Chapter: verse.Chapter,
									Verse:   verse.Verse,
									OSISID:  verse.ID,
								},
							},
						},
					},
				},
			}
			doc.ContentBlocks = append(doc.ContentBlocks, cb)
		}
	}

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
		LossClass: "L0",
		LossReport: &LossReport{
			SourceFormat: "JSON",
			TargetFormat: "IR",
			LossClass:    "L0",
		},
	})
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

	outputPath := filepath.Join(outputDir, corpus.ID+".json")

	// Check for raw JSON for L0 round-trip
	if raw, ok := corpus.Attributes["_json_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			respondError(fmt.Sprintf("failed to write JSON: %v", err))
			return
		}

		respond(&EmitNativeResult{
			OutputPath: outputPath,
			Format:     "JSON",
			LossClass:  "L0",
			LossReport: &LossReport{
				SourceFormat: "IR",
				TargetFormat: "JSON",
				LossClass:    "L0",
			},
		})
		return
	}

	// Generate JSON from IR
	jsonBible := JSONBible{
		Meta: JSONMeta{
			ID:          corpus.ID,
			Title:       corpus.Title,
			Language:    corpus.Language,
			Description: corpus.Description,
			Version:     corpus.Version,
		},
	}

	for _, doc := range corpus.Documents {
		book := JSONBook{
			ID:    doc.ID,
			Name:  doc.Title,
			Order: doc.Order,
		}

		chapterMap := make(map[int]*JSONChapter)
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						chapter, ok := chapterMap[span.Ref.Chapter]
						if !ok {
							chapter = &JSONChapter{Number: span.Ref.Chapter}
							chapterMap[span.Ref.Chapter] = chapter
						}

						verse := JSONVerse{
							Book:    doc.ID,
							Chapter: span.Ref.Chapter,
							Verse:   span.Ref.Verse,
							Text:    cb.Text,
							ID:      span.Ref.OSISID,
						}
						chapter.Verses = append(chapter.Verses, verse)
						jsonBible.Verses = append(jsonBible.Verses, verse)
					}
				}
			}
		}

		// Sort chapters and add to book
		for i := 1; i <= 200; i++ {
			if ch, ok := chapterMap[i]; ok {
				book.Chapters = append(book.Chapters, *ch)
			}
		}

		jsonBible.Books = append(jsonBible.Books, book)
	}

	jsonData, err := json.MarshalIndent(jsonBible, "", "  ")
	if err != nil {
		respondError(fmt.Sprintf("failed to serialize JSON: %v", err))
		return
	}

	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write JSON: %v", err))
		return
	}

	respond(&EmitNativeResult{
		OutputPath: outputPath,
		Format:     "JSON",
		LossClass:  "L1",
		LossReport: &LossReport{
			SourceFormat: "IR",
			TargetFormat: "JSON",
			LossClass:    "L1",
		},
	})
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
