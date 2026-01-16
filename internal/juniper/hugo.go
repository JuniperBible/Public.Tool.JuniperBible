// Package juniper provides shared logic for Bible/SWORD module tools.
package juniper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FocuswithJustin/JuniperBible/internal/formats/swordpure"
)

// ntBooks contains all New Testament book OSIS IDs for quick lookup.
var ntBooks = map[string]bool{
	"Matt": true, "Mark": true, "Luke": true, "John": true,
	"Acts": true, "Rom": true, "1Cor": true, "2Cor": true,
	"Gal": true, "Eph": true, "Phil": true, "Col": true,
	"1Thess": true, "2Thess": true, "1Tim": true, "2Tim": true,
	"Titus": true, "Phlm": true, "Heb": true, "Jas": true,
	"1Pet": true, "2Pet": true, "1John": true, "2John": true,
	"3John": true, "Jude": true, "Rev": true,
}

// isNTBook returns true if the book OSIS ID is a New Testament book.
func isNTBook(osis string) bool {
	return ntBooks[osis]
}

// HugoConfig holds configuration for Hugo JSON generation.
type HugoConfig struct {
	Path    string   // SWORD installation path (default: ~/.sword)
	Output  string   // Output directory for Hugo data files
	Modules []string // Specific modules to export (empty = all)
	All     bool     // Export all Bible modules
	Workers int      // Number of parallel workers (0 = sequential)
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
	LicenseText   string   `json:"licenseText,omitempty"`
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

// Hugo generates Hugo-compatible JSON data files from SWORD modules.
func Hugo(cfg HugoConfig) error {
	swordPath, err := ResolveSwordPath(cfg.Path)
	if err != nil {
		return err
	}

	modules, err := ListModules(swordPath)
	if err != nil {
		return err
	}

	if len(modules) == 0 {
		return fmt.Errorf("no Bible modules found in %s", swordPath)
	}

	// Filter modules
	var toExport []*Module
	if cfg.All {
		toExport = modules
	} else if len(cfg.Modules) > 0 {
		moduleMap := make(map[string]*Module)
		for _, m := range modules {
			moduleMap[strings.ToLower(m.Name)] = m
		}
		for _, name := range cfg.Modules {
			if m, ok := moduleMap[strings.ToLower(name)]; ok {
				toExport = append(toExport, m)
			} else {
				fmt.Printf("Warning: module '%s' not found\n", name)
			}
		}
	} else {
		return fmt.Errorf("specify module names or use --all")
	}

	if len(toExport) == 0 {
		return fmt.Errorf("no modules to export")
	}

	// Create output directories
	if err := os.MkdirAll(cfg.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	auxDir := filepath.Join(cfg.Output, "bibles_auxiliary")
	if err := os.MkdirAll(auxDir, 0755); err != nil {
		return fmt.Errorf("failed to create auxiliary directory: %w", err)
	}

	// Determine worker count
	workers := cfg.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	fmt.Printf("Exporting %d Bible(s) to Hugo JSON in %s/ using %d workers\n\n", len(toExport), cfg.Output, workers)

	metadata := HugoBibleMetadata{
		Bibles: make([]HugoBibleEntry, 0),
		Meta: HugoMetaInfo{
			Granularity: "chapter",
			Generated:   time.Now(),
			Version:     "1.0.0",
		},
	}

	// Result collector
	type exportResult struct {
		entry   *HugoBibleEntry
		content *HugoBibleContent
		auxPath string
		err     error
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	results := make(chan exportResult, len(toExport))

	for i, m := range toExport {
		if m.Encrypted {
			fmt.Printf("Skipping %s: encrypted\n", m.Name)
			continue
		}

		wg.Add(1)
		go func(m *Module, weight int) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			fmt.Printf("Processing %s...\n", m.Name)

			content, entry, err := exportModuleToHugo(swordPath, m, weight)
			if err != nil {
				results <- exportResult{err: fmt.Errorf("%s: %w", m.Name, err)}
				return
			}

			// Write auxiliary file
			auxPath := filepath.Join(auxDir, entry.ID+".json")
			if err := writeJSON(auxPath, content); err != nil {
				results <- exportResult{err: fmt.Errorf("%s: write error: %w", m.Name, err)}
				return
			}

			fmt.Printf("  %s: %d books exported\n", entry.Abbrev, len(content.Books))
			results <- exportResult{entry: entry, content: content, auxPath: auxPath}
		}(m, i+1)
	}

	// Close results channel when all workers done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for res := range results {
		if res.err != nil {
			fmt.Printf("  Error: %v\n", res.err)
			continue
		}
		metadata.Bibles = append(metadata.Bibles, *res.entry)
	}

	// Sort bibles by weight for consistent output
	sort.Slice(metadata.Bibles, func(i, j int) bool {
		return metadata.Bibles[i].Weight < metadata.Bibles[j].Weight
	})

	// Write bibles.json metadata
	biblesPath := filepath.Join(cfg.Output, "bibles.json")
	if err := writeJSON(biblesPath, metadata); err != nil {
		return fmt.Errorf("failed to write bibles.json: %w", err)
	}

	fmt.Printf("\nCreated %s with %d Bible(s)\n", biblesPath, len(metadata.Bibles))
	fmt.Println("Done!")

	return nil
}

// bookResult holds the result of processing a single book.
type bookResult struct {
	idx      int
	book     *HugoBook
	excluded *HugoExcluded
}

// exportModuleToHugo exports a single SWORD module to Hugo format.
func exportModuleToHugo(swordPath string, module *Module, weight int) (*HugoBibleContent, *HugoBibleEntry, error) {
	// Load conf file using swordpure
	conf, err := swordpure.ParseConfFile(module.ConfPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse conf: %w", err)
	}

	// Open the module
	zt, err := swordpure.OpenZTextModule(conf, swordPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open module: %w", err)
	}

	// Get versification
	vers, err := swordpure.VersificationFromConf(conf)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get versification: %w", err)
	}

	// Create metadata entry
	entry := &HugoBibleEntry{
		ID:            strings.ToLower(module.Name),
		Title:         module.Description,
		Description:   module.Description,
		Abbrev:        strings.ToUpper(module.Name),
		Language:      module.Lang,
		License:       getLicense(conf),
		LicenseText:   getLicenseText(conf, swordPath, module),
		Versification: conf.Versification,
		Features:      []string{},
		Tags:          generateBibleTags(module, conf),
		Weight:        weight,
	}

	// Determine testament availability
	hasOT := zt.HasOT()
	hasNT := zt.HasNT()

	// Build content
	content := &HugoBibleContent{
		Content:       fmt.Sprintf("The %s translation.", module.Description),
		Books:         make([]HugoBook, 0),
		ExcludedBooks: make([]HugoExcluded, 0),
	}

	// Process books in parallel
	var wg sync.WaitGroup
	results := make(chan bookResult, len(vers.Books))
	sem := make(chan struct{}, runtime.NumCPU())

	for bookIdx, book := range vers.Books {
		isNT := isNTBook(book.OSIS)
		if isNT && !hasNT {
			continue
		}
		if !isNT && !hasOT {
			continue
		}

		wg.Add(1)
		go func(idx int, book swordpure.BookData, isNT bool) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			testament := "OT"
			if isNT {
				testament = "NT"
			}

			hugoBook := HugoBook{
				ID:        book.OSIS,
				Name:      book.Name,
				Testament: testament,
				Chapters:  make([]HugoChapter, 0),
			}

			totalVerses := 0
			for ch := 1; ch <= len(book.Chapters); ch++ {
				hugoChapter := HugoChapter{
					Number: ch,
					Verses: make([]HugoVerse, 0),
				}

				for v := 1; v <= book.Chapters[ch-1]; v++ {
					ref := &swordpure.Ref{Book: book.OSIS, Chapter: ch, Verse: v}
					rawText, err := zt.GetVerseText(ref)
					if err != nil || rawText == "" {
						continue
					}

					plainText := stripMarkup(rawText)
					if plainText == "" || isPlaceholder(plainText) {
						continue
					}

					hugoChapter.Verses = append(hugoChapter.Verses, HugoVerse{
						Number: v,
						Text:   plainText,
					})
					totalVerses++
				}

				if len(hugoChapter.Verses) > 0 {
					hugoBook.Chapters = append(hugoBook.Chapters, hugoChapter)
				}
			}

			if totalVerses > 0 {
				results <- bookResult{idx: idx, book: &hugoBook}
			} else {
				results <- bookResult{idx: idx, excluded: &HugoExcluded{
					ID:        book.OSIS,
					Name:      book.Name,
					Testament: testament,
					Reason:    "no content in source",
				}}
			}
		}(bookIdx, book, isNT)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var bookResults []bookResult
	for res := range results {
		bookResults = append(bookResults, res)
	}

	// Sort by original book order
	sort.Slice(bookResults, func(i, j int) bool {
		return bookResults[i].idx < bookResults[j].idx
	})

	// Build final content
	for _, res := range bookResults {
		if res.book != nil {
			content.Books = append(content.Books, *res.book)
		} else if res.excluded != nil {
			content.ExcludedBooks = append(content.ExcludedBooks, *res.excluded)
		}
	}

	return content, entry, nil
}

// stripMarkup removes OSIS/ThML markup, returning plain text.
func stripMarkup(text string) string {
	var result strings.Builder
	inTag := false

	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteByte(c)
		}
	}

	return strings.TrimSpace(result.String())
}

// isPlaceholder checks if text is just a verse reference placeholder.
var placeholderPattern = regexp.MustCompile(`^(?:[1-4]\s+|I{1,3}V?\s+)?[A-Za-z]+(?:\s+(?:of\s+)?[A-Za-z]+)*\s+\d+:\d+:?$`)

func isPlaceholder(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) < 5 {
		return true
	}
	return placeholderPattern.MatchString(text)
}

// getLicense extracts license information from conf.
func getLicense(conf *swordpure.ConfFile) string {
	if conf.License != "" {
		return conf.License
	}
	if conf.Copyright != "" {
		return conf.Copyright
	}
	return "Unknown"
}

// getLicenseText extracts the full license text from conf properties or LICENSE file.
func getLicenseText(conf *swordpure.ConfFile, swordPath string, module *Module) string {
	// Try DistributionLicenseNotes first (common in SWORD modules)
	if notes, ok := conf.Properties["DistributionLicenseNotes"]; ok && notes != "" {
		return notes
	}

	// Try ShortCopyright
	if shortCopy, ok := conf.Properties["ShortCopyright"]; ok && shortCopy != "" {
		return shortCopy
	}

	// Try TextSource + Copyright combination
	var parts []string
	if conf.Copyright != "" {
		parts = append(parts, conf.Copyright)
	}
	if textSource, ok := conf.Properties["TextSource"]; ok && textSource != "" {
		parts = append(parts, "Source: "+textSource)
	}

	// Try About field which often contains license details
	if conf.About != "" {
		if len(parts) > 0 {
			parts = append(parts, conf.About)
		} else {
			return conf.About
		}
	}

	if len(parts) > 0 {
		return strings.Join(parts, "\n\n")
	}

	// Try to read LICENSE file from module data path
	if module.DataPath != "" {
		dataPath := strings.TrimPrefix(module.DataPath, "./")
		licenseFiles := []string{"LICENSE", "LICENSE.txt", "COPYING", "license.txt"}
		for _, lf := range licenseFiles {
			licensePath := filepath.Join(swordPath, dataPath, lf)
			if data, err := os.ReadFile(licensePath); err == nil {
				return string(data)
			}
		}
	}

	return ""
}

// generateBibleTags creates comprehensive tags for a Bible module.
func generateBibleTags(module *Module, conf *swordpure.ConfFile) []string {
	tags := []string{}

	// Language tag
	langMap := map[string]string{
		"en":  "English",
		"la":  "Latin",
		"grc": "Greek",
		"he":  "Hebrew",
	}
	if lang, ok := langMap[module.Lang]; ok {
		tags = append(tags, lang)
	}

	// Testament tags based on versification and known modules
	moduleID := strings.ToLower(module.Name)
	switch moduleID {
	case "sblgnt":
		tags = append(tags, "New Testament")
	case "osmhb":
		tags = append(tags, "Old Testament")
	case "lxx":
		tags = append(tags, "Old Testament")
	case "oeb":
		// OEB has partial OT (Psalms) and full NT
		tags = append(tags, "New Testament")
	default:
		// Most Bibles have both testaments
		tags = append(tags, "Old Testament", "New Testament")
	}

	// Canon tags based on versification
	switch conf.Versification {
	case "Vulg":
		tags = append(tags, "Catholic Canon")
	case "LXX":
		tags = append(tags, "Orthodox Canon", "Septuagint")
	case "Leningrad":
		tags = append(tags, "Masoretic Text")
	default:
		tags = append(tags, "Protestant Canon")
	}

	// Text type for Greek NT
	if moduleID == "sblgnt" {
		tags = append(tags, "Critical Text")
	}

	// License tags
	license := strings.ToLower(getLicense(conf))
	if strings.Contains(license, "public domain") || license == "public domain" {
		tags = append(tags, "Public Domain")
	}

	// Era tags - historical vs modern
	historicalBibles := map[string]bool{
		"kjv": true, "asv": true, "tyndale": true, "geneva1599": true,
		"drc": true, "vulgate": true, "lxx": true,
	}
	if historicalBibles[moduleID] {
		tags = append(tags, "Historical Translation")
	} else if moduleID == "web" || moduleID == "oeb" || moduleID == "sblgnt" {
		tags = append(tags, "Modern Translation")
	}

	// Special features
	if strings.Contains(strings.ToLower(module.Description), "strong") {
		tags = append(tags, "Strong's Numbers")
	}

	return tags
}

// writeJSON writes data to a JSON file with proper formatting.
func writeJSON(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// HugoResult contains the result of Hugo generation.
type HugoResult struct {
	BiblesGenerated int
	OutputFiles     []string
}

// parseVerseRef parses a verse reference like "Genesis 1:1" into book, chapter, verse.
func parseVerseRef(ref string) (book string, chapter, verse int) {
	// Handle refs like "Genesis 1:1", "1 John 3:16", "Song of Solomon 1:1"
	parts := strings.Split(ref, " ")
	if len(parts) < 2 {
		return ref, 0, 0
	}

	// Find the chapter:verse part (last element)
	chapterVerse := parts[len(parts)-1]
	book = strings.Join(parts[:len(parts)-1], " ")

	// Parse chapter:verse
	cvParts := strings.Split(chapterVerse, ":")
	if len(cvParts) >= 1 {
		chapter, _ = strconv.Atoi(cvParts[0])
	}
	if len(cvParts) >= 2 {
		verse, _ = strconv.Atoi(cvParts[1])
	}

	return book, chapter, verse
}
