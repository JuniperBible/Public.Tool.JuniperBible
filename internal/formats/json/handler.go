// Package json provides the embedded handler for the JSON Bible format plugin.
// It implements the EmbeddedFormatHandler interface from core/plugins.
package json

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// Handler implements the EmbeddedFormatHandler interface for JSON format.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.json",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-json",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"file"},
			Outputs: []string{"artifact.kind:json-bible"},
		},
		IRSupport: &plugins.IRCapabilities{
			CanExtract: true,
			CanEmit:    true,
			LossClass:  "L0",
			Formats:    []string{"JSON"},
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

// init automatically registers this plugin when the package is imported.
func init() {
	Register()
}

// IR types are now imported from core/ir package

// JSONBible is the clean output format
type JSONBible struct {
	Meta   JSONMeta    `json:"meta"`
	Books  []JSONBook  `json:"books"`
	Verses []JSONVerse `json:"verses,omitempty"`
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
	ID      string `json:"id"`
}

// Detect implements EmbeddedFormatHandler.Detect.
func (h *Handler) Detect(path string) (*plugins.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		}, nil
	}

	if info.IsDir() {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   "path is a directory",
		}, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".json" {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   "not a .json file",
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		}, nil
	}

	var jsonBible JSONBible
	if err := json.Unmarshal(data, &jsonBible); err != nil {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   "not valid JSON",
		}, nil
	}

	if jsonBible.Meta.ID == "" && len(jsonBible.Books) == 0 && len(jsonBible.Verses) == 0 {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   "not a Capsule JSON Bible format",
		}, nil
	}

	return &plugins.DetectResult{
		Detected: true,
		Format:   "JSON",
		Reason:   "Capsule JSON Bible format detected",
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
		Metadata: map[string]string{
			"format": "JSON",
		},
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
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
				Metadata: map[string]string{
					"format": "JSON",
				},
			},
		},
	}, nil
}

// ExtractIR implements EmbeddedFormatHandler.ExtractIR.
func (h *Handler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)

	var jsonBible JSONBible
	if err := json.Unmarshal(data, &jsonBible); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	corpus := &ir.Corpus{
		ID:           jsonBible.Meta.ID,
		Version:      jsonBible.Meta.Version,
		ModuleType:   ir.ModuleBible,
		Title:        jsonBible.Meta.Title,
		Language:     jsonBible.Meta.Language,
		Description:  jsonBible.Meta.Description,
		SourceFormat: "JSON",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    ir.LossL0,
		Attributes:   make(map[string]string),
	}

	// Store raw for L0 round-trip
	corpus.Attributes["_json_raw"] = string(data)

	sequence := 0
	for _, book := range jsonBible.Books {
		doc := &ir.Document{
			ID:         book.ID,
			Title:      book.Name,
			Order:      book.Order,
			Attributes: make(map[string]string),
		}

		for _, chapter := range book.Chapters {
			for _, verse := range chapter.Verses {
				sequence++
				hash := sha256.Sum256([]byte(verse.Text))
				osisID := fmt.Sprintf("%s.%d.%d", book.ID, chapter.Number, verse.Verse)

				cb := &ir.ContentBlock{
					ID:       fmt.Sprintf("cb-%d", sequence),
					Sequence: sequence,
					Text:     verse.Text,
					Hash:     hex.EncodeToString(hash[:]),
					Anchors: []*ir.Anchor{
						{
							ID:       fmt.Sprintf("a-%d-0", sequence),
							Position: 0,
							Spans: []*ir.Span{
								{
									ID:            fmt.Sprintf("s-%s", osisID),
									Type:          ir.SpanVerse,
									StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
									Ref: &ir.Ref{
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

	// Handle flat verses
	if len(jsonBible.Books) == 0 && len(jsonBible.Verses) > 0 {
		bookDocs := make(map[string]*ir.Document)
		for _, verse := range jsonBible.Verses {
			doc, ok := bookDocs[verse.Book]
			if !ok {
				doc = &ir.Document{
					ID:         verse.Book,
					Title:      verse.Book,
					Order:      len(bookDocs) + 1,
					Attributes: make(map[string]string),
				}
				bookDocs[verse.Book] = doc
				corpus.Documents = append(corpus.Documents, doc)
			}

			sequence++
			hash := sha256.Sum256([]byte(verse.Text))

			cb := &ir.ContentBlock{
				ID:       fmt.Sprintf("cb-%d", sequence),
				Sequence: sequence,
				Text:     verse.Text,
				Hash:     hex.EncodeToString(hash[:]),
				Anchors: []*ir.Anchor{
					{
						ID:       fmt.Sprintf("a-%d-0", sequence),
						Position: 0,
						Spans: []*ir.Span{
							{
								ID:            fmt.Sprintf("s-%s", verse.ID),
								Type:          ir.SpanVerse,
								StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
								Ref: &ir.Ref{
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
		return nil, fmt.Errorf("failed to serialize IR: %w", err)
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write IR: %w", err)
	}

	return &plugins.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L0",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "JSON",
			TargetFormat: "IR",
			LossClass:    "L0",
		},
	}, nil
}

// EmitNative implements EmbeddedFormatHandler.EmitNative.
func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	data, err := os.ReadFile(irPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read IR file: %w", err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return nil, fmt.Errorf("failed to parse IR: %w", err)
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".json")

	// Check for raw JSON for L0 round-trip
	if raw, ok := corpus.Attributes["_json_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			return nil, fmt.Errorf("failed to write JSON: %w", err)
		}
		return &plugins.EmitNativeResult{
			OutputPath: outputPath,
			Format:     "JSON",
			LossClass:  "L0",
			LossReport: &plugins.LossReportIPC{
				SourceFormat: "IR",
				TargetFormat: "JSON",
				LossClass:    "L0",
			},
		}, nil
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
					if span.Ref != nil && span.Type == ir.SpanVerse {
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

		for i := 1; i <= 200; i++ {
			if ch, ok := chapterMap[i]; ok {
				book.Chapters = append(book.Chapters, *ch)
			}
		}

		jsonBible.Books = append(jsonBible.Books, book)
	}

	jsonData, err := json.MarshalIndent(jsonBible, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize JSON: %w", err)
	}

	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write JSON: %w", err)
	}

	return &plugins.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "JSON",
		LossClass:  "L1",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "IR",
			TargetFormat: "JSON",
			LossClass:    "L1",
		},
	}, nil
}
