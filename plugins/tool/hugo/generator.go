// generator.go implements Hugo JSON generation from Bible data.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// placeholderPattern matches verse references that appear as placeholder text.
var placeholderPattern = regexp.MustCompile(`^(?:[1-4]\s+|I{1,3}V?\s+)?[A-Za-z]+(?:\s+(?:of\s+)?[A-Za-z]+)*\s+\d+:\d+:?$`)

// isPlaceholderText checks if text is a placeholder verse reference.
func isPlaceholderText(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) < 5 {
		return true
	}
	return placeholderPattern.MatchString(text)
}

// Generator creates JSON files for Hugo from Bible data.
type Generator struct {
	OutputDir   string
	Granularity string // "book", "chapter", or "verse"
}

// NewGenerator creates a new JSON generator.
func NewGenerator(outputDir, granularity string) *Generator {
	if granularity == "" {
		granularity = "chapter"
	}
	return &Generator{
		OutputDir:   outputDir,
		Granularity: granularity,
	}
}

// HugoBibleMetadata is the structure for bibles.json.
type HugoBibleMetadata struct {
	Bibles []HugoBibleEntry `json:"bibles"`
	Meta   HugoMetaInfo     `json:"meta"`
}

// HugoBibleEntry represents a single Bible in the metadata file.
type HugoBibleEntry struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Abbrev        string   `json:"abbrev"`
	Language      string   `json:"language"`
	License       string   `json:"license"`
	Versification string   `json:"versification"`
	Features      []string `json:"features"`
	Tags          []string `json:"tags"`
	Weight        int      `json:"weight"`
}

// HugoMetaInfo contains metadata about the generated files.
type HugoMetaInfo struct {
	Granularity string    `json:"granularity"`
	Generated   time.Time `json:"generated"`
	Version     string    `json:"version"`
}

// HugoBibleContent contains the full content of a Bible.
type HugoBibleContent struct {
	Content       string         `json:"content"`
	Books         []HugoBook     `json:"books"`
	ExcludedBooks []HugoExcluded `json:"excludedBooks,omitempty"`
}

// HugoBook represents a book's content.
type HugoBook struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Testament string        `json:"testament"`
	Chapters  []HugoChapter `json:"chapters"`
}

// HugoChapter represents a chapter's content.
type HugoChapter struct {
	Number int         `json:"number"`
	Verses []HugoVerse `json:"verses"`
}

// HugoVerse represents a single verse.
type HugoVerse struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

// HugoExcluded represents an excluded book.
type HugoExcluded struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Testament string `json:"testament"`
	Reason    string `json:"reason"`
}

// InputBible represents input Bible data for generation.
type InputBible struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Language    string      `json:"language"`
	License     string      `json:"license"`
	Books       []InputBook `json:"books"`
}

// InputBook represents input book data.
type InputBook struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Testament string         `json:"testament"`
	Chapters  []InputChapter `json:"chapters"`
}

// InputChapter represents input chapter data.
type InputChapter struct {
	Number int          `json:"number"`
	Verses []InputVerse `json:"verses"`
}

// InputVerse represents input verse data.
type InputVerse struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

// GenerateFromInput generates Hugo JSON from an input Bible structure.
func (g *Generator) GenerateFromInput(bibles []InputBible) (*GenerateResult, error) {
	result := &GenerateResult{
		OutputFiles: []string{},
	}

	metadata := HugoBibleMetadata{
		Bibles: make([]HugoBibleEntry, 0),
		Meta: HugoMetaInfo{
			Granularity: g.Granularity,
			Generated:   time.Now(),
			Version:     "1.0.0",
		},
	}

	// Create output directory
	if err := os.MkdirAll(g.OutputDir, 0700); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	// Create auxiliary directory
	auxDir := filepath.Join(g.OutputDir, "bibles_auxiliary")
	if err := os.MkdirAll(auxDir, 0700); err != nil {
		return nil, fmt.Errorf("creating auxiliary directory: %w", err)
	}

	for i, bible := range bibles {
		entry := HugoBibleEntry{
			ID:            bible.ID,
			Title:         bible.Title,
			Description:   bible.Description,
			Abbrev:        strings.ToUpper(bible.ID),
			Language:      bible.Language,
			License:       bible.License,
			Versification: "protestant",
			Features:      []string{},
			Tags:          []string{bible.Language},
			Weight:        i + 1,
		}
		metadata.Bibles = append(metadata.Bibles, entry)

		// Generate content
		content, chapters := g.convertToHugoContent(bible)
		result.ChaptersWritten += chapters

		// Write auxiliary file
		auxPath := filepath.Join(auxDir, bible.ID+".json")
		if err := g.writeJSON(auxPath, content); err != nil {
			return nil, fmt.Errorf("writing auxiliary for %s: %w", bible.ID, err)
		}
		result.OutputFiles = append(result.OutputFiles, auxPath)
		result.BiblesGenerated++
	}

	// Write metadata file
	metaPath := filepath.Join(g.OutputDir, "bibles.json")
	if err := g.writeJSON(metaPath, metadata); err != nil {
		return nil, fmt.Errorf("writing metadata: %w", err)
	}
	result.OutputFiles = append(result.OutputFiles, metaPath)

	return result, nil
}

// convertToHugoContent converts input Bible to Hugo format.
func (g *Generator) convertToHugoContent(bible InputBible) (*HugoBibleContent, int) {
	content := &HugoBibleContent{
		Content:       fmt.Sprintf("The %s translation.", bible.Title),
		Books:         make([]HugoBook, 0, len(bible.Books)),
		ExcludedBooks: make([]HugoExcluded, 0),
	}

	totalChapters := 0

	for _, book := range bible.Books {
		hugoBook := HugoBook{
			ID:        book.ID,
			Name:      book.Name,
			Testament: book.Testament,
			Chapters:  make([]HugoChapter, 0, len(book.Chapters)),
		}

		totalVerses := 0
		for _, chapter := range book.Chapters {
			hugoChapter := HugoChapter{
				Number: chapter.Number,
				Verses: make([]HugoVerse, 0, len(chapter.Verses)),
			}

			for _, verse := range chapter.Verses {
				// Skip placeholder verses
				if isPlaceholderText(verse.Text) {
					continue
				}
				hugoVerse := HugoVerse{
					Number: verse.Number,
					Text:   verse.Text,
				}
				hugoChapter.Verses = append(hugoChapter.Verses, hugoVerse)
			}

			totalVerses += len(hugoChapter.Verses)
			hugoBook.Chapters = append(hugoBook.Chapters, hugoChapter)
			totalChapters++
		}

		// Skip books with no verse content
		if totalVerses == 0 {
			excluded := HugoExcluded{
				ID:        book.ID,
				Name:      book.Name,
				Testament: book.Testament,
				Reason:    "no content in source",
			}
			content.ExcludedBooks = append(content.ExcludedBooks, excluded)
			continue
		}

		content.Books = append(content.Books, hugoBook)
	}

	return content, totalChapters
}

// GenerateFromFile generates Hugo JSON from a JSON input file.
func (g *Generator) GenerateFromFile(inputPath string) (*GenerateResult, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("reading input file: %w", err)
	}

	// Try to parse as array of bibles
	var bibles []InputBible
	if err := json.Unmarshal(data, &bibles); err != nil {
		// Try single bible
		var single InputBible
		if err := json.Unmarshal(data, &single); err != nil {
			return nil, fmt.Errorf("parsing input: %w", err)
		}
		bibles = []InputBible{single}
	}

	return g.GenerateFromInput(bibles)
}

// writeJSON writes data to a JSON file with proper formatting.
func (g *Generator) writeJSON(path string, data interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Generate is the main entry point for Hugo generation.
func Generate(opts *GenerateOptions) (*GenerateResult, error) {
	if opts.OutputPath == "" {
		return nil, fmt.Errorf("output path is required")
	}

	generator := NewGenerator(opts.OutputPath, "chapter")

	return generator.GenerateFromFile(opts.InputPath)
}
