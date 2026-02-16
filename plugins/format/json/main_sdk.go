//go:build sdk

// Plugin format-json handles JSON Bible format using the SDK pattern.
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

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// Parallel Corpus Types
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
	if err := format.Run(&format.Config{
		Name:       "JSON",
		Extensions: []string{".json"},
		Detect:     detectJSON,
		Parse:      parseJSON,
		Emit:       emitJSON,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// detectJSON checks if the file is a Capsule JSON Bible format
func detectJSON(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "path is a directory",
		}, nil
	}

	// Check extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".json" {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not a .json file",
		}, nil
	}

	// Try to parse as JSONBible
	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		}, nil
	}

	var jsonBible JSONBible
	if err := json.Unmarshal(data, &jsonBible); err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not valid JSON",
		}, nil
	}

	// Check for our format markers
	if jsonBible.Meta.ID == "" && len(jsonBible.Books) == 0 && len(jsonBible.Verses) == 0 {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not a Capsule JSON Bible format",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: true,
		Format:   "JSON",
		Reason:   "Capsule JSON Bible format detected",
	}, nil
}

// parseJSON converts JSON Bible format to IR
func parseJSON(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)

	var jsonBible JSONBible
	if err := json.Unmarshal(data, &jsonBible); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Convert JSONBible to IR Corpus
	corpus := &ir.Corpus{
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
		doc := &ir.Document{
			ID:    book.ID,
			Title: book.Name,
			Order: book.Order,
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
									Type:          "VERSE",
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

	// Also handle flat verses if present
	if len(jsonBible.Books) == 0 && len(jsonBible.Verses) > 0 {
		bookDocs := make(map[string]*ir.Document)
		for _, verse := range jsonBible.Verses {
			doc, ok := bookDocs[verse.Book]
			if !ok {
				doc = &ir.Document{
					ID:    verse.Book,
					Title: verse.Book,
					Order: len(bookDocs) + 1,
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
								Type:          "VERSE",
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

	return corpus, nil
}

// emitJSON converts IR back to JSON Bible format
func emitJSON(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".json")

	// Check for raw JSON for L0 round-trip
	if raw, ok := corpus.Attributes["_json_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			return "", fmt.Errorf("failed to write JSON: %w", err)
		}
		return outputPath, nil
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
		return "", fmt.Errorf("failed to serialize JSON: %w", err)
	}

	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return "", fmt.Errorf("failed to write JSON: %w", err)
	}

	return outputPath, nil
}
