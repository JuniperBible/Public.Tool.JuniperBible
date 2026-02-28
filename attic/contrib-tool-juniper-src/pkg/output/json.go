// Package output generates Hugo-compatible JSON from parsed SWORD modules.
package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/FocuswithJustin/juniper/pkg/repository"
	"github.com/FocuswithJustin/juniper/pkg/sword"
)

// placeholderPattern matches verse references that appear as placeholder text
// when the actual verse content is missing from a module.
// Examples: "Genesis 1:1:", "II Chronicles 19:2:", "1 John 3:16:", "Song of Songs 1:1:"
var placeholderPattern = regexp.MustCompile(`^(?:[1-4]\s+|I{1,3}V?\s+)?[A-Za-z]+(?:\s+(?:of\s+)?[A-Za-z]+)*\s+\d+:\d+:?$`)

// isPlaceholderText checks if text is a placeholder verse reference.
func isPlaceholderText(text string) bool {
	text = strings.TrimSpace(text)
	// Empty or very short text (< 5 chars is likely just punctuation or reference)
	if len(text) < 5 {
		return true
	}
	return placeholderPattern.MatchString(text)
}

// Generator creates JSON files for Hugo from parsed SWORD modules.
type Generator struct {
	OutputDir    string
	Granularity  string          // "book", "chapter", or "verse"
	SPDXLicenses map[string]bool // Valid SPDX license IDs loaded from spdx_licenses.json
}

// NewGenerator creates a new JSON generator.
func NewGenerator(outputDir string, granularity string) *Generator {
	return &Generator{
		OutputDir:    outputDir,
		Granularity:  granularity,
		SPDXLicenses: make(map[string]bool),
	}
}

// LoadSPDXLicenses loads valid SPDX license IDs from the spdx_licenses.json file.
func (g *Generator) LoadSPDXLicenses(spdxPath string) error {
	data, err := os.ReadFile(spdxPath)
	if err != nil {
		return fmt.Errorf("reading spdx_licenses.json: %w", err)
	}

	var spdxData struct {
		Licenses map[string]interface{} `json:"licenses"`
	}
	if err := json.Unmarshal(data, &spdxData); err != nil {
		return fmt.Errorf("parsing spdx_licenses.json: %w", err)
	}

	for id := range spdxData.Licenses {
		g.SPDXLicenses[id] = true
	}

	return nil
}

// ValidateLicense checks if a license ID exists in the SPDX data.
func (g *Generator) ValidateLicense(licenseID string) error {
	if len(g.SPDXLicenses) == 0 {
		// No SPDX data loaded, skip validation
		return nil
	}
	if !g.SPDXLicenses[licenseID] {
		return fmt.Errorf("license %q not found in spdx_licenses.json - please add it before building", licenseID)
	}
	return nil
}

// BibleMetadata is the structure for bibles.json.
type BibleMetadata struct {
	Bibles []BibleEntry `json:"bibles"`
	Meta   MetaInfo     `json:"meta"`
}

// BibleEntry represents a single Bible in the metadata file.
type BibleEntry struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Abbrev        string   `json:"abbrev"`
	Language      string   `json:"language"`
	License       string   `json:"license"`
	LicenseText   string   `json:"licenseText,omitempty"`
	Versification string   `json:"versification"`
	Features      []string `json:"features"`
	Tags          []string `json:"tags"`
	Weight        int      `json:"weight"`
}

// MetaInfo contains metadata about the generated files.
type MetaInfo struct {
	Granularity string    `json:"granularity"`
	Generated   time.Time `json:"generated"`
	Version     string    `json:"version"`
}

// BibleAuxiliary is the structure for bibles_auxiliary.json.
type BibleAuxiliary struct {
	Bibles map[string]BibleContent `json:"bibles"`
}

// BibleContent contains the full content of a Bible.
type BibleContent struct {
	Content       string           `json:"content"`
	Books         []BookContent    `json:"books"`
	ExcludedBooks []ExcludedBook   `json:"excludedBooks,omitempty"`
	Sections      []ContentSection `json:"sections,omitempty"`
}

// ExcludedBook represents a book that exists in the versification system
// but has no content in this particular module.
type ExcludedBook struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Testament string `json:"testament"`
	Reason    string `json:"reason"`
}

// BookContent represents a book's content.
type BookContent struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Testament string           `json:"testament"`
	Chapters  []ChapterContent `json:"chapters"`
}

// ChapterContent represents a chapter's content.
type ChapterContent struct {
	Number int            `json:"number"`
	Verses []VerseContent `json:"verses"`
}

// VerseContent represents a single verse.
type VerseContent struct {
	Number     int      `json:"number"`
	Text       string   `json:"text"`
	Strongs    []string `json:"strongs,omitempty"`
	Morphology []string `json:"morphology,omitempty"`
}

// ContentSection represents a section in the auxiliary content.
type ContentSection struct {
	Heading string `json:"heading"`
	Content string `json:"content,omitempty"`
}

// GenerateFromModules generates JSON files from a list of modules.
func (g *Generator) GenerateFromModules(modules []*sword.Module, swordDir string) error {
	metadata := BibleMetadata{
		Bibles: make([]BibleEntry, 0),
		Meta: MetaInfo{
			Granularity: g.Granularity,
			Generated:   time.Now(),
			Version:     "2.0.0",
		},
	}

	auxiliary := BibleAuxiliary{
		Bibles: make(map[string]BibleContent),
	}

	for i, module := range modules {
		if module.ModuleType != sword.ModuleTypeBible {
			continue
		}

		// Convert to SPDX license identifier
		spdxLicense := repository.ToSPDXLicense(module.DistributionLicense)
		if spdxLicense == "" {
			return fmt.Errorf("module %s has no distribution license specified", module.ID)
		}

		// Validate license exists in SPDX data
		if err := g.ValidateLicense(spdxLicense); err != nil {
			return fmt.Errorf("module %s: %w", module.ID, err)
		}

		entry := BibleEntry{
			ID:            module.ID,
			Title:         module.Title,
			Description:   module.Description,
			Abbrev:        strings.ToUpper(module.ID),
			Language:      module.Language,
			License:       spdxLicense,
			LicenseText:   module.About,
			Versification: g.normalizeVersification(module.Versification),
			Features:      module.Features,
			Tags:          g.generateTags(module),
			Weight:        i + 1,
		}
		metadata.Bibles = append(metadata.Bibles, entry)

		// Parse Bible content
		content, err := g.parseBibleContent(module, swordDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", module.ID, err)
			continue
		}
		auxiliary.Bibles[module.ID] = *content
	}

	// Write metadata file
	metaPath := filepath.Join(g.OutputDir, "bibles.json")
	if err := g.writeJSON(metaPath, metadata); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	// Write individual auxiliary files (one per Bible)
	auxDir := filepath.Join(g.OutputDir, "bibles_auxiliary")
	if err := os.MkdirAll(auxDir, 0700); err != nil {
		return fmt.Errorf("creating bibles_auxiliary directory: %w", err)
	}

	for bibleID, content := range auxiliary.Bibles {
		auxPath := filepath.Join(auxDir, bibleID+".json")
		if err := g.writeJSON(auxPath, content); err != nil {
			return fmt.Errorf("writing auxiliary for %s: %w", bibleID, err)
		}
	}

	return nil
}

// normalizeVersification converts SWORD versification names to our standard format.
func (g *Generator) normalizeVersification(vers string) string {
	lower := strings.ToLower(vers)
	switch {
	case lower == "" || lower == "kjv":
		return "protestant"
	case lower == "vulg" || lower == "vulgate":
		return "catholic"
	case lower == "lxx" || lower == "septuagint":
		return "orthodox"
	default:
		return lower
	}
}

// parseBibleContent parses a Bible module and returns its content.
func (g *Generator) parseBibleContent(module *sword.Module, swordDir string) (*BibleContent, error) {
	parser := sword.NewZTextParser(module, swordDir)

	books, err := parser.GetAllBooks()
	if err != nil {
		return nil, err
	}

	content := &BibleContent{
		Content:       fmt.Sprintf("The %s translation.", module.Title),
		Books:         make([]BookContent, 0, len(books)),
		ExcludedBooks: make([]ExcludedBook, 0),
	}

	for _, book := range books {
		bookContent := BookContent{
			ID:        book.ID,
			Name:      book.Name,
			Testament: book.Testament,
			Chapters:  make([]ChapterContent, 0, len(book.Chapters)),
		}

		totalVerses := 0
		for _, chapter := range book.Chapters {
			chapterContent := ChapterContent{
				Number: chapter.Number,
				Verses: make([]VerseContent, 0, len(chapter.Verses)),
			}

			for _, verse := range chapter.Verses {
				// Skip placeholder verses (missing content)
				if isPlaceholderText(verse.Text) {
					continue
				}
				verseContent := VerseContent{
					Number:     verse.Reference.Verse,
					Text:       verse.Text,
					Strongs:    verse.Strongs,
					Morphology: verse.Morphology,
				}
				chapterContent.Verses = append(chapterContent.Verses, verseContent)
			}

			totalVerses += len(chapterContent.Verses)
			bookContent.Chapters = append(bookContent.Chapters, chapterContent)
		}

		// Skip books with no verse content (e.g., NT books in Hebrew-only modules)
		if totalVerses == 0 {
			// Record as excluded with reason
			excluded := ExcludedBook{
				ID:        book.ID,
				Name:      book.Name,
				Testament: book.Testament,
				Reason:    "no content in source module",
			}
			content.ExcludedBooks = append(content.ExcludedBooks, excluded)
			continue
		}

		content.Books = append(content.Books, bookContent)
	}

	return content, nil
}

// generateTags creates tags from module metadata.
func (g *Generator) generateTags(module *sword.Module) []string {
	tags := make([]string, 0)

	// Add language tag
	if module.Language != "" {
		tags = append(tags, module.Language)
	}

	// Add feature tags
	if module.HasStrongsNumbers() {
		tags = append(tags, "Strong's Numbers")
	}
	if module.HasMorphology() {
		tags = append(tags, "Morphology")
	}

	return tags
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
