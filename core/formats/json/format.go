// Package json handles JSON Bible format.
// Provides clean JSON structure for programmatic access.
//
// IR Support:
// - extract-ir: Reads JSON Bible format back to IR (L0)
// - emit-native: Converts IR to clean JSON format (L0)
package json

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

// Config defines the JSON format plugin.
var Config = &format.Config{
	PluginID:   "format.json",
	Name:       "JSON",
	Extensions: []string{".json"},
	Detect:     detectJSON,
	Parse:      parseJSON,
	Emit:       emitJSON,
}

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
	corpus := ir.NewCorpus(jsonBible.Meta.ID, "BIBLE", jsonBible.Meta.Language)
	corpus.Version = jsonBible.Meta.Version
	corpus.Title = jsonBible.Meta.Title
	corpus.Description = jsonBible.Meta.Description
	corpus.SourceFormat = "JSON"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L0"
	corpus.Attributes = make(map[string]string)

	// Store raw for L0 round-trip
	corpus.Attributes["_json_raw"] = string(data)

	sequence := 0
	for _, book := range jsonBible.Books {
		doc := ir.NewDocument(book.ID, book.Name, book.Order)

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
				doc = ir.NewDocument(verse.Book, verse.Book, len(bookDocs)+1)
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

func emitJSON(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".json")

	if written, err := tryWriteRawJSON(corpus, outputPath); written {
		return outputPath, err
	}

	jsonBible := buildJSONBible(corpus)

	jsonData, err := json.MarshalIndent(jsonBible, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize JSON: %w", err)
	}

	if err := os.WriteFile(outputPath, jsonData, 0600); err != nil {
		return "", fmt.Errorf("failed to write JSON: %w", err)
	}

	return outputPath, nil
}

// tryWriteRawJSON writes the stored raw JSON for an L0 round-trip when available.
// It returns (true, err) if a raw payload was found (whether the write succeeded or not),
// and (false, nil) when no raw payload exists and normal generation should proceed.
func tryWriteRawJSON(corpus *ir.Corpus, outputPath string) (bool, error) {
	raw, ok := corpus.Attributes["_json_raw"]
	if !ok || raw == "" {
		return false, nil
	}
	if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
		return true, fmt.Errorf("failed to write JSON: %w", err)
	}
	return true, nil
}

// buildJSONBible constructs a JSONBible value from the IR corpus.
func buildJSONBible(corpus *ir.Corpus) JSONBible {
	bible := JSONBible{
		Meta: JSONMeta{
			ID:          corpus.ID,
			Title:       corpus.Title,
			Language:    corpus.Language,
			Description: corpus.Description,
			Version:     corpus.Version,
		},
	}

	for _, doc := range corpus.Documents {
		book, verses := buildJSONBook(doc)
		bible.Books = append(bible.Books, book)
		bible.Verses = append(bible.Verses, verses...)
	}

	return bible
}

// buildJSONBook converts a single IR document into a JSONBook and its flat verse list.
func buildJSONBook(doc *ir.Document) (JSONBook, []JSONVerse) {
	book := JSONBook{
		ID:    doc.ID,
		Name:  doc.Title,
		Order: doc.Order,
	}

	chapterMap := make(map[int]*JSONChapter)
	var flatVerses []JSONVerse

	for _, cb := range doc.ContentBlocks {
		for _, anchor := range cb.Anchors {
			for _, span := range anchor.Spans {
				verse, ok := verseFromSpan(span, doc.ID, cb.Text)
				if !ok {
					continue
				}
				chapter := getOrCreateChapter(chapterMap, span.Ref.Chapter)
				chapter.Verses = append(chapter.Verses, verse)
				flatVerses = append(flatVerses, verse)
			}
		}
	}

	book.Chapters = sortedChapters(chapterMap)
	return book, flatVerses
}

// verseFromSpan extracts a JSONVerse from a span when it is a VERSE span with a valid Ref.
// Returns the verse and true on success, or a zero value and false otherwise.
func verseFromSpan(span *ir.Span, bookID, text string) (JSONVerse, bool) {
	if span.Ref == nil || span.Type != "VERSE" {
		return JSONVerse{}, false
	}
	return JSONVerse{
		Book:    bookID,
		Chapter: span.Ref.Chapter,
		Verse:   span.Ref.Verse,
		Text:    text,
		ID:      span.Ref.OSISID,
	}, true
}

// getOrCreateChapter returns the JSONChapter for the given number from the map,
// creating and storing it if it does not yet exist.
func getOrCreateChapter(chapterMap map[int]*JSONChapter, number int) *JSONChapter {
	if ch, ok := chapterMap[number]; ok {
		return ch
	}
	ch := &JSONChapter{Number: number}
	chapterMap[number] = ch
	return ch
}

// sortedChapters returns chapters from the map in ascending numeric order.
func sortedChapters(chapterMap map[int]*JSONChapter) []JSONChapter {
	var chapters []JSONChapter
	for i := 1; i <= 200; i++ {
		if ch, ok := chapterMap[i]; ok {
			chapters = append(chapters, *ch)
		}
	}
	return chapters
}
