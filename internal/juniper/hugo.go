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

	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/formats/swordpure"
	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/safefile"
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
	swordPath, modules, err := loadSwordModules(cfg.Path)
	if err != nil {
		return err
	}

	toExport, err := filterModulesToExport(modules, cfg)
	if err != nil {
		return err
	}

	auxDir, err := setupOutputDirectories(cfg.Output)
	if err != nil {
		return err
	}

	workers := determineWorkerCount(cfg.Workers)
	fmt.Printf("Exporting %d Bible(s) to Hugo JSON in %s/ using %d workers\n\n", len(toExport), cfg.Output, workers)

	metadata := initializeMetadata()
	exportModulesInParallel(toExport, swordPath, auxDir, workers, &metadata)

	return finalizeMetadataFile(cfg.Output, metadata)
}

// loadSwordModules loads and validates SWORD modules from the given path.
func loadSwordModules(path string) (string, []*Module, error) {
	swordPath, err := ResolveSwordPath(path)
	if err != nil {
		return "", nil, err
	}

	modules, err := ListModules(swordPath)
	if err != nil {
		return "", nil, err
	}

	if len(modules) == 0 {
		return "", nil, fmt.Errorf("no Bible modules found in %s", swordPath)
	}

	return swordPath, modules, nil
}

// filterModulesToExport filters modules based on configuration.
func filterModulesToExport(modules []*Module, cfg HugoConfig) ([]*Module, error) {
	var toExport []*Module

	if cfg.All {
		toExport = modules
	} else if len(cfg.Modules) > 0 {
		toExport = selectSpecificModules(modules, cfg.Modules)
	} else {
		return nil, fmt.Errorf("specify module names or use --all")
	}

	if len(toExport) == 0 {
		return nil, fmt.Errorf("no modules to export")
	}

	return toExport, nil
}

// selectSpecificModules selects modules by name from the available modules.
func selectSpecificModules(modules []*Module, names []string) []*Module {
	moduleMap := make(map[string]*Module)
	for _, m := range modules {
		moduleMap[strings.ToLower(m.Name)] = m
	}

	var selected []*Module
	for _, name := range names {
		if m, ok := moduleMap[strings.ToLower(name)]; ok {
			selected = append(selected, m)
		} else {
			fmt.Printf("Warning: module '%s' not found\n", name)
		}
	}

	return selected
}

// setupOutputDirectories creates the necessary output directories.
func setupOutputDirectories(output string) (string, error) {
	if err := os.MkdirAll(output, 0700); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	auxDir := filepath.Join(output, "bibles_auxiliary")
	if err := os.MkdirAll(auxDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create auxiliary directory: %w", err)
	}

	return auxDir, nil
}

// determineWorkerCount determines the number of workers to use.
func determineWorkerCount(configuredWorkers int) int {
	if configuredWorkers <= 0 {
		return runtime.NumCPU()
	}
	return configuredWorkers
}

// initializeMetadata creates the initial metadata structure.
func initializeMetadata() HugoBibleMetadata {
	return HugoBibleMetadata{
		Bibles: make([]HugoBibleEntry, 0),
		Meta: HugoMetaInfo{
			Granularity: "chapter",
			Generated:   time.Now(),
			Version:     "1.0.0",
		},
	}
}

// exportResult holds the result of exporting a single module.
type exportResult struct {
	entry   *HugoBibleEntry
	content *HugoBibleContent
	auxPath string
	err     error
}

// exportModulesInParallel exports modules in parallel and collects results.
func exportModulesInParallel(modules []*Module, swordPath, auxDir string, workers int, metadata *HugoBibleMetadata) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	results := make(chan exportResult, len(modules))

	// Encrypted modules are now supported via Sapphire II cipher decryption
	for i, m := range modules {
		wg.Add(1)
		go exportSingleModule(m, i+1, swordPath, auxDir, sem, results, &wg)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	collectExportResults(results, metadata)
}

// exportSingleModule exports a single module in a goroutine.
func exportSingleModule(m *Module, weight int, swordPath, auxDir string, sem chan struct{}, results chan<- exportResult, wg *sync.WaitGroup) {
	defer wg.Done()
	sem <- struct{}{}
	defer func() { <-sem }()

	fmt.Printf("Processing %s...\n", m.Name)

	content, entry, err := exportModuleToHugo(swordPath, m, weight)
	if err != nil {
		results <- exportResult{err: fmt.Errorf("%s: %w", m.Name, err)}
		return
	}

	auxPath := filepath.Join(auxDir, entry.ID+".json")
	if err := writeJSON(auxPath, content); err != nil {
		results <- exportResult{err: fmt.Errorf("%s: write error: %w", m.Name, err)}
		return
	}

	fmt.Printf("  %s: %d books exported\n", entry.Abbrev, len(content.Books))
	results <- exportResult{entry: entry, content: content, auxPath: auxPath}
}

// collectExportResults collects results from export goroutines.
func collectExportResults(results <-chan exportResult, metadata *HugoBibleMetadata) {
	for res := range results {
		if res.err != nil {
			fmt.Printf("  Error: %v\n", res.err)
			continue
		}
		metadata.Bibles = append(metadata.Bibles, *res.entry)
	}

	sort.Slice(metadata.Bibles, func(i, j int) bool {
		return metadata.Bibles[i].Weight < metadata.Bibles[j].Weight
	})
}

// finalizeMetadataFile writes the final metadata file.
func finalizeMetadataFile(output string, metadata HugoBibleMetadata) error {
	biblesPath := filepath.Join(output, "bibles.json")
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

// moduleResources bundles the parsed artifacts needed to export one module.
type moduleResources struct {
	conf *swordpure.ConfFile
	zt   *swordpure.ZTextModule
	vers *swordpure.Versification
}

// openModuleResources parses the conf, opens the ZText reader, and resolves
// the versification scheme for a module in one call.
func openModuleResources(swordPath string, module *Module) (moduleResources, error) {
	conf, err := swordpure.ParseConfFile(module.ConfPath)
	if err != nil {
		return moduleResources{}, fmt.Errorf("failed to parse conf: %w", err)
	}

	zt, err := swordpure.OpenZTextModule(conf, swordPath)
	if err != nil {
		return moduleResources{}, fmt.Errorf("failed to open module: %w", err)
	}

	vers, err := swordpure.VersificationFromConf(conf)
	if err != nil {
		return moduleResources{}, fmt.Errorf("failed to get versification: %w", err)
	}

	return moduleResources{conf: conf, zt: zt, vers: vers}, nil
}

// buildModuleEntry constructs the HugoBibleEntry metadata for a module.
func buildModuleEntry(module *Module, res moduleResources, swordPath string, weight int) *HugoBibleEntry {
	return &HugoBibleEntry{
		ID:            strings.ToLower(module.Name),
		Title:         module.Description,
		Description:   module.Description,
		Abbrev:        strings.ToUpper(module.Name),
		Language:      module.Lang,
		License:       getLicense(res.conf),
		LicenseText:   getLicenseText(res.conf, swordPath, module),
		Versification: res.conf.Versification,
		Features:      []string{},
		Tags:          generateBibleTags(module, res.conf),
		Weight:        weight,
	}
}

// processChapter reads all verses for one chapter and returns a populated
// HugoChapter, or nil when no usable verse text is found.
func processChapter(zt *swordpure.ZTextModule, book swordpure.BookData, ch int) *HugoChapter {
	hugoChapter := HugoChapter{Number: ch, Verses: make([]HugoVerse, 0)}

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
		hugoChapter.Verses = append(hugoChapter.Verses, HugoVerse{Number: v, Text: plainText})
	}

	if len(hugoChapter.Verses) == 0 {
		return nil
	}
	return &hugoChapter
}

// processBook builds a bookResult for one canonical book, reading its verses
// via processChapter. Books with no usable content become an excluded entry.
func processBook(zt *swordpure.ZTextModule, idx int, book swordpure.BookData) bookResult {
	testament := "OT"
	if isNTBook(book.OSIS) {
		testament = "NT"
	}

	hugoBook := HugoBook{
		ID: book.OSIS, Name: book.Name,
		Testament: testament,
		Chapters:  make([]HugoChapter, 0),
	}

	for ch := 1; ch <= len(book.Chapters); ch++ {
		if chapter := processChapter(zt, book, ch); chapter != nil {
			hugoBook.Chapters = append(hugoBook.Chapters, *chapter)
		}
	}

	if len(hugoBook.Chapters) > 0 {
		return bookResult{idx: idx, book: &hugoBook}
	}
	return bookResult{idx: idx, excluded: &HugoExcluded{
		ID: book.OSIS, Name: book.Name,
		Testament: testament, Reason: "no content in source",
	}}
}

// dispatchBooks fans out book-processing goroutines and closes results when done.
func dispatchBooks(res moduleResources, results chan<- bookResult) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	hasOT, hasNT := res.zt.HasOT(), res.zt.HasNT()

	for bookIdx, book := range res.vers.Books {
		isNT := isNTBook(book.OSIS)
		if (isNT && !hasNT) || (!isNT && !hasOT) {
			continue
		}
		wg.Add(1)
		go func(idx int, b swordpure.BookData) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results <- processBook(res.zt, idx, b)
		}(bookIdx, book)
	}

	wg.Wait()
	close(results)
}

// assembleContent collects bookResults, sorts them into canonical order, and
// populates the Books / ExcludedBooks slices on content.
func assembleContent(rawResults <-chan bookResult, content *HugoBibleContent) {
	var bookResults []bookResult
	for res := range rawResults {
		bookResults = append(bookResults, res)
	}
	sort.Slice(bookResults, func(i, j int) bool {
		return bookResults[i].idx < bookResults[j].idx
	})
	for _, res := range bookResults {
		if res.book != nil {
			content.Books = append(content.Books, *res.book)
		} else if res.excluded != nil {
			content.ExcludedBooks = append(content.ExcludedBooks, *res.excluded)
		}
	}
}

// exportModuleToHugo exports a single SWORD module to Hugo format.
func exportModuleToHugo(swordPath string, module *Module, weight int) (*HugoBibleContent, *HugoBibleEntry, error) {
	res, err := openModuleResources(swordPath, module)
	if err != nil {
		return nil, nil, err
	}

	entry := buildModuleEntry(module, res, swordPath, weight)

	content := &HugoBibleContent{
		Content:       fmt.Sprintf("The %s translation.", module.Description),
		Books:         make([]HugoBook, 0),
		ExcludedBooks: make([]HugoExcluded, 0),
	}

	results := make(chan bookResult, len(res.vers.Books))
	go dispatchBooks(res, results)
	assembleContent(results, content)

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

// findLicenseFile finds the license file in a directory.
func findLicenseFile(dir string) (string, error) {
	licenseFiles := []string{"LICENSE", "LICENSE.txt", "COPYING", "license.txt"}
	for _, lf := range licenseFiles {
		licensePath := filepath.Join(dir, lf)
		if _, err := os.Stat(licensePath); err == nil {
			return licensePath, nil
		}
	}
	return "", fmt.Errorf("no license file found")
}

// readLicenseFromPath reads license content from a path.
func readLicenseFromPath(path string) (string, error) {
	data, err := safefile.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// getLicenseText extracts the full license text from conf properties or LICENSE file.
func confPropertyText(conf *swordpure.ConfFile, key string) string {
	v := conf.Properties[key]
	return v
}

func buildConfLicenseParts(conf *swordpure.ConfFile) string {
	var parts []string
	if conf.Copyright != "" {
		parts = append(parts, conf.Copyright)
	}
	if ts := confPropertyText(conf, "TextSource"); ts != "" {
		parts = append(parts, "Source: "+ts)
	}
	if conf.About != "" {
		if len(parts) == 0 {
			return conf.About
		}
		parts = append(parts, conf.About)
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n\n")
	}
	return ""
}

func licenseFromFile(swordPath string, module *Module) string {
	if module.DataPath == "" {
		return ""
	}
	dataPath := strings.TrimPrefix(module.DataPath, "./")
	dir := filepath.Join(swordPath, dataPath)
	licensePath, err := findLicenseFile(dir)
	if err != nil {
		return ""
	}
	content, err := readLicenseFromPath(licensePath)
	if err != nil {
		return ""
	}
	return content
}

func getLicenseText(conf *swordpure.ConfFile, swordPath string, module *Module) string {
	if v := confPropertyText(conf, "DistributionLicenseNotes"); v != "" {
		return v
	}
	if v := confPropertyText(conf, "ShortCopyright"); v != "" {
		return v
	}
	if v := buildConfLicenseParts(conf); v != "" {
		return v
	}
	return licenseFromFile(swordPath, module)
}

// generateBibleTags creates comprehensive tags for a Bible module.
func generateBibleTags(module *Module, conf *swordpure.ConfFile) []string {
	moduleID := strings.ToLower(module.Name)
	var tags []string

	tags = appendLanguageTag(tags, module.Lang)
	tags = appendTestamentTags(tags, moduleID)
	tags = appendCanonTags(tags, conf.Versification, moduleID)
	tags = appendLicenseTag(tags, getLicense(conf))
	tags = appendEraTag(tags, moduleID)
	tags = appendFeatureTags(tags, module.Description)

	return tags
}

// appendLanguageTag adds language tag if recognized.
func appendLanguageTag(tags []string, langCode string) []string {
	langMap := map[string]string{"en": "English", "la": "Latin", "grc": "Greek", "he": "Hebrew"}
	if lang, ok := langMap[langCode]; ok {
		return append(tags, lang)
	}
	return tags
}

// appendTestamentTags adds testament tags based on module ID.
func appendTestamentTags(tags []string, moduleID string) []string {
	switch moduleID {
	case "sblgnt", "oeb":
		return append(tags, "New Testament")
	case "osmhb", "lxx":
		return append(tags, "Old Testament")
	default:
		return append(tags, "Old Testament", "New Testament")
	}
}

// appendCanonTags adds canon and text type tags.
func appendCanonTags(tags []string, versification, moduleID string) []string {
	switch versification {
	case "Vulg":
		tags = append(tags, "Catholic Canon")
	case "LXX":
		tags = append(tags, "Orthodox Canon", "Septuagint")
	case "Leningrad":
		tags = append(tags, "Masoretic Text")
	default:
		tags = append(tags, "Protestant Canon")
	}
	if moduleID == "sblgnt" {
		tags = append(tags, "Critical Text")
	}
	return tags
}

// appendLicenseTag adds public domain tag if applicable.
func appendLicenseTag(tags []string, license string) []string {
	if strings.Contains(strings.ToLower(license), "public domain") {
		return append(tags, "Public Domain")
	}
	return tags
}

// appendEraTag adds historical or modern translation tag.
func appendEraTag(tags []string, moduleID string) []string {
	historicalBibles := map[string]bool{
		"kjv": true, "asv": true, "tyndale": true, "geneva1599": true,
		"drc": true, "vulgate": true, "lxx": true,
	}
	if historicalBibles[moduleID] {
		return append(tags, "Historical Translation")
	}
	if moduleID == "web" || moduleID == "oeb" || moduleID == "sblgnt" {
		return append(tags, "Modern Translation")
	}
	return tags
}

// appendFeatureTags adds feature-based tags.
func appendFeatureTags(tags []string, description string) []string {
	if strings.Contains(strings.ToLower(description), "strong") {
		return append(tags, "Strong's Numbers")
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
