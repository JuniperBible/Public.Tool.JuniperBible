// Package main provides a CLI tool for extracting Bible text using diatheke.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Versification represents the structure of a versification YAML file.
type Versification struct {
	Name            string `yaml:"name"`
	Extends         string `yaml:"extends"`
	Books           []Book `yaml:"books"`
	AdditionalBooks []Book `yaml:"additional_books"`
}

// Book represents a book in a versification.
type Book struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Chapters    int    `yaml:"chapters"`
	Testament   string `yaml:"testament"`
	InsertAfter string `yaml:"insert_after"`
	MergeWith   string `yaml:"merge_with"`
}

// BibleMeta contains metadata for a Bible translation.
type BibleMeta struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Abbrev        string   `json:"abbrev"`
	Language      string   `json:"language"`
	Versification string   `json:"versification"`
	Features      []string `json:"features"`
	Tags          []string `json:"tags"`
	Weight        int      `json:"weight"`
}

// BibleMetadata is the structure for bibles.json.
type BibleMetadata struct {
	Bibles []BibleMeta `json:"bibles"`
	Meta   struct {
		Granularity string `json:"granularity"`
		Generated   string `json:"generated"`
		Version     string `json:"version"`
	} `json:"meta"`
}

// BibleAuxiliary is kept for backwards compatibility but no longer used.
// Individual Bible files are now written to bibles_auxiliary/ directory.
type BibleAuxiliary struct {
	Bibles map[string]BibleContent `json:"bibles"`
}

// BibleContent contains the full content of a Bible.
type BibleContent struct {
	Content  string        `json:"content"`
	Books    []BookContent `json:"books"`
	Sections []interface{} `json:"sections"`
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
	Number int    `json:"number"`
	Text   string `json:"text"`
}

// Scripture module registry - all 27 SWORD modules supported
var scriptures = map[string]BibleMeta{
	// Historic English Translations
	"KJV": {
		ID:            "kjv",
		Title:         "King James Version (1769)",
		Description:   "The Authorized Version of the Bible, the most influential English translation in history.",
		Abbrev:        "KJV",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "Historic"},
	},
	"Tyndale": {
		ID:            "tyndale",
		Title:         "Tyndale Bible (1525/1530)",
		Description:   "First English Bible translated directly from Greek and Hebrew.",
		Abbrev:        "TYN",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "Historic"},
	},
	"Geneva1599": {
		ID:            "geneva1599",
		Title:         "Geneva Bible (1599)",
		Description:   "The primary English Protestant Bible of the 16th century.",
		Abbrev:        "GNV",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "Historic"},
	},
	"Wycliffe": {
		ID:            "wycliffe",
		Title:         "Wycliffe Bible (c.1395)",
		Description:   "First complete English translation of the Bible, translated from the Latin Vulgate.",
		Abbrev:        "WYC",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Historic", "Medieval"},
	},
	// Catholic Translations
	"DRC": {
		ID:            "drc",
		Title:         "Douay-Rheims Bible",
		Description:   "English translation of the Latin Vulgate by Catholic scholars (1582-1610).",
		Abbrev:        "DRB",
		Language:      "en",
		Versification: "catholic",
		Tags:          []string{"English", "Catholic", "Historic"},
	},
	"CPDV": {
		ID:            "cpdv",
		Title:         "Catholic Public Domain Version",
		Description:   "Modern Catholic translation based on the Latin Vulgate with Deuterocanonical books.",
		Abbrev:        "CPDV",
		Language:      "en",
		Versification: "catholic",
		Tags:          []string{"English", "Catholic", "Modern"},
	},
	// Latin
	"Vulgate": {
		ID:            "vulgate",
		Title:         "Latin Vulgate",
		Description:   "Jerome's 4th century Latin translation, the authoritative Bible of the Catholic Church.",
		Abbrev:        "VUL",
		Language:      "la",
		Versification: "catholic",
		Tags:          []string{"Latin", "Catholic", "Historic"},
	},
	// American Translations
	"ASV": {
		ID:            "asv",
		Title:         "American Standard Version (1901)",
		Description:   "An American revision of the KJV, known for its literal accuracy and use of 'Jehovah'.",
		Abbrev:        "ASV",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "American"},
	},
	"Darby": {
		ID:            "darby",
		Title:         "Darby Bible (1890)",
		Description:   "John Nelson Darby's literal translation emphasizing consistency in rendering Greek/Hebrew words.",
		Abbrev:        "DBY",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "Literal"},
	},
	"YLT": {
		ID:            "ylt",
		Title:         "Young's Literal Translation (1898)",
		Description:   "Robert Young's extremely literal translation preserving Hebrew/Greek verb tenses.",
		Abbrev:        "YLT",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "Literal"},
	},
	// Modern Translations
	"WEB": {
		ID:            "web",
		Title:         "World English Bible",
		Description:   "A public domain modern English translation based on the ASV with updated language.",
		Abbrev:        "WEB",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "Modern", "Public Domain"},
	},
	"BBE": {
		ID:            "bbe",
		Title:         "Bible in Basic English (1965)",
		Description:   "Translation using a vocabulary of only 1000 common English words.",
		Abbrev:        "BBE",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "Simple"},
	},
	// Greek Text
	"LXX": {
		ID:            "lxx",
		Title:         "Septuagint (Rahlfs)",
		Description:   "The ancient Greek translation of the Hebrew Bible, the Old Testament of the early Church.",
		Abbrev:        "LXX",
		Language:      "grc",
		Versification: "catholic",
		Tags:          []string{"Greek", "Historic", "Septuagint"},
	},
	"SBLGNT": {
		ID:            "sblgnt",
		Title:         "SBL Greek New Testament",
		Description:   "The Society of Biblical Literature's critical edition of the Greek New Testament.",
		Abbrev:        "SBLGNT",
		Language:      "grc",
		Versification: "protestant",
		Tags:          []string{"Greek", "Critical Text", "Academic"},
	},
	// Hebrew Text
	"OSMHB": {
		ID:            "osmhb",
		Title:         "Open Scriptures Hebrew Bible",
		Description:   "Open source morphological Hebrew Bible based on the Westminster Leningrad Codex.",
		Abbrev:        "OSHB",
		Language:      "he",
		Versification: "protestant",
		Tags:          []string{"Hebrew", "Masoretic", "Open Source"},
	},
	// Additional Historic Translations
	"Webster": {
		ID:            "webster",
		Title:         "Webster Bible (1833)",
		Description:   "Noah Webster's revision of the KJV with updated language and Americanized spelling.",
		Abbrev:        "WBS",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "American", "Historic"},
	},
	"Rotherham": {
		ID:            "rotherham",
		Title:         "Rotherham Emphasized Bible (1902)",
		Description:   "Joseph Rotherham's translation with emphasis marks showing Greek/Hebrew emphasis.",
		Abbrev:        "EBR",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "Literal"},
	},
	"AKJV": {
		ID:            "akjv",
		Title:         "American King James Version",
		Description:   "The KJV with archaic words replaced with modern equivalents.",
		Abbrev:        "AKJV",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "Updated KJV"},
	},
	// Jewish Translations
	"JPS": {
		ID:            "jps",
		Title:         "JPS Tanakh (1917)",
		Description:   "Jewish Publication Society translation of the Hebrew Bible.",
		Abbrev:        "JPS",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Jewish", "Historic"},
	},
	// Additional Modern
	"GodsWord": {
		ID:            "godsword",
		Title:         "GOD'S WORD Translation",
		Description:   "A thought-for-thought translation emphasizing natural English.",
		Abbrev:        "GW",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "Modern"},
	},
	"LEB": {
		ID:            "leb",
		Title:         "Lexham English Bible",
		Description:   "A transparent English translation designed for study.",
		Abbrev:        "LEB",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "Modern", "Study"},
	},
	// Greek NT Editions
	"TR": {
		ID:            "tr",
		Title:         "Textus Receptus (1550/1894)",
		Description:   "The 'Received Text' Greek New Testament underlying the KJV translation.",
		Abbrev:        "TR",
		Language:      "grc",
		Versification: "protestant",
		Tags:          []string{"Greek", "Textus Receptus", "Historic"},
	},
	"Byz": {
		ID:            "byz",
		Title:         "Byzantine Textform (2013)",
		Description:   "Robinson-Pierpont Byzantine Greek New Testament.",
		Abbrev:        "BYZ",
		Language:      "grc",
		Versification: "protestant",
		Tags:          []string{"Greek", "Byzantine", "Academic"},
	},
	// Additional Literal Translations
	"RLT": {
		ID:            "rlt",
		Title:         "Revised Literal Translation (2018)",
		Description:   "A thoroughly revised literal translation of the KJV.",
		Abbrev:        "RLT",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "Literal"},
	},
	// Messianic
	"HNV": {
		ID:            "hnv",
		Title:         "Hebrew Names Version",
		Description:   "World English Bible with Hebrew names for God and biblical figures.",
		Abbrev:        "HNV",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Messianic", "Modern"},
	},
	// Early English
	"Weymouth": {
		ID:            "weymouth",
		Title:         "Weymouth New Testament (1912)",
		Description:   "Richard Weymouth's modern speech translation of the New Testament.",
		Abbrev:        "WNT",
		Language:      "en",
		Versification: "protestant",
		Tags:          []string{"English", "Protestant", "Historic"},
	},
	// Apocryphal
	"KJVA": {
		ID:            "kjva",
		Title:         "King James with Apocrypha",
		Description:   "The King James Version including the Deuterocanonical/Apocryphal books.",
		Abbrev:        "KJVA",
		Language:      "en",
		Versification: "catholic",
		Tags:          []string{"English", "Protestant", "Apocrypha"},
	},
}

var (
	outputDir string
	modules   []string
	verbose   bool
)

// placeholderPattern matches verse references that appear as placeholder text
var placeholderPattern = regexp.MustCompile(`^(?:[1-4]\s+|I{1,3}V?\s+)?[A-Za-z]+(?:\s+(?:of\s+)?[A-Za-z]+)*\s+\d+:\d+:?$`)

func main() {
	rootCmd := &cobra.Command{
		Use:   "extract",
		Short: "Extract Bible text using diatheke",
		Long: `Extract Bible text from SWORD modules using diatheke and generate
Hugo-compatible JSON files for the religion section.`,
		RunE: runExtract,
	}

	rootCmd.Flags().StringVarP(&outputDir, "output", "o", "data", "Output directory for JSON files")
	rootCmd.Flags().StringSliceVarP(&modules, "modules", "m", nil, "Specific modules to extract (default: all available)")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runExtract(cmd *cobra.Command, args []string) error {
	// Check if diatheke is available
	if _, err := exec.LookPath("diatheke"); err != nil {
		return fmt.Errorf("diatheke not found in PATH. Install SWORD tools")
	}

	// Load versifications
	versifications := make(map[string]*Versification)
	// Try multiple paths to find versifications
	versDir := ""
	for _, tryDir := range []string{
		"versifications",
		"tools/juniper/versifications",
		filepath.Join(filepath.Dir(os.Args[0]), "versifications"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "versifications"),
	} {
		if _, err := os.Stat(tryDir); err == nil {
			versDir = tryDir
			break
		}
	}
	if versDir == "" {
		return fmt.Errorf("versifications directory not found")
	}

	for _, name := range []string{"protestant", "catholic"} {
		v, err := loadVersification(filepath.Join(versDir, name+".yaml"))
		if err != nil {
			return fmt.Errorf("loading versification %s: %w", name, err)
		}
		versifications[name] = v
	}

	metadata := BibleMetadata{
		Bibles: make([]BibleMeta, 0),
	}
	metadata.Meta.Granularity = "chapter"
	metadata.Meta.Generated = time.Now().UTC().Format(time.RFC3339)
	metadata.Meta.Version = "2.0.0"

	// Store Bible content for individual file output
	bibleContents := make(map[string]*BibleContent)

	// Determine which modules to extract
	modulesToExtract := modules
	if len(modulesToExtract) == 0 {
		for m := range scriptures {
			modulesToExtract = append(modulesToExtract, m)
		}
	}

	weight := 1
	for _, module := range modulesToExtract {
		meta, ok := scriptures[module]
		if !ok {
			fmt.Fprintf(os.Stderr, "Warning: Unknown module %s, skipping\n", module)
			continue
		}

		// Check if module is available
		if !checkModuleAvailable(module) {
			fmt.Fprintf(os.Stderr, "Warning: Module %s not available, skipping\n", module)
			continue
		}

		vers := versifications[meta.Versification]
		if vers == nil {
			vers = versifications["protestant"]
		}

		fmt.Fprintf(os.Stderr, "Extracting %s using %s versification...\n", module, vers.Name)

		content, err := extractBible(module, meta, vers)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error extracting %s: %v\n", module, err)
			continue
		}

		meta.Weight = weight
		weight++
		meta.Features = []string{}
		metadata.Bibles = append(metadata.Bibles, meta)
		bibleContents[meta.ID] = content
	}

	// Write output files
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	metaPath := filepath.Join(outputDir, "bibles.json")
	if err := writeJSON(metaPath, metadata); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Wrote %s\n", metaPath)

	// Write individual Bible files to bibles_auxiliary/ directory
	auxDir := filepath.Join(outputDir, "bibles_auxiliary")
	if err := os.MkdirAll(auxDir, 0700); err != nil {
		return fmt.Errorf("creating bibles_auxiliary directory: %w", err)
	}

	for id, content := range bibleContents {
		auxPath := filepath.Join(auxDir, id+".json")
		if err := writeJSON(auxPath, content); err != nil {
			return fmt.Errorf("writing %s: %w", auxPath, err)
		}
		fmt.Fprintf(os.Stderr, "Wrote %s\n", auxPath)
	}

	// Summary
	totalBooks := 0
	totalVerses := 0
	for _, bible := range bibleContents {
		totalBooks += len(bible.Books)
		for _, book := range bible.Books {
			for _, ch := range book.Chapters {
				totalVerses += len(ch.Verses)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "\nExtracted %d Bibles with %d books and %d total verses\n",
		len(bibleContents), totalBooks, totalVerses)

	return nil
}

func loadVersification(path string) (*Versification, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is validated
	if err != nil {
		return nil, err
	}

	var v Versification
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, err
	}

	// Handle inheritance
	if v.Extends != "" {
		parentPath := filepath.Join(filepath.Dir(path), v.Extends+".yaml")
		parent, err := loadVersification(parentPath)
		if err != nil {
			return nil, fmt.Errorf("loading parent versification %s: %w", v.Extends, err)
		}

		// Start with parent's books
		books := make([]Book, len(parent.Books))
		copy(books, parent.Books)

		// Add additional books at appropriate positions
		for _, addBook := range v.AdditionalBooks {
			if addBook.MergeWith != "" {
				continue // Skip merged books
			}
			if addBook.Chapters == 0 {
				continue // Skip structural entries
			}

			if addBook.InsertAfter != "" {
				// Find insertion point
				inserted := false
				for i, b := range books {
					if b.ID == addBook.InsertAfter {
						// Insert after this book
						newBooks := make([]Book, 0, len(books)+1)
						newBooks = append(newBooks, books[:i+1]...)
						newBooks = append(newBooks, addBook)
						newBooks = append(newBooks, books[i+1:]...)
						books = newBooks
						inserted = true
						break
					}
				}
				if !inserted {
					books = append(books, addBook)
				}
			} else {
				books = append(books, addBook)
			}
		}

		v.Books = books
	}

	return &v, nil
}

func checkModuleAvailable(module string) bool {
	cmd := exec.Command("diatheke", "-b", module, "-k", "Gen 1:1")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}

func extractBible(module string, meta BibleMeta, vers *Versification) (*BibleContent, error) {
	content := &BibleContent{
		Content:  meta.Description,
		Books:    make([]BookContent, 0),
		Sections: []interface{}{},
	}

	for _, book := range vers.Books {
		if verbose {
			fmt.Fprintf(os.Stderr, "  %s...", book.Name)
		}

		// Check if book exists in this module
		if !checkBookExists(module, book.Name) {
			if verbose {
				fmt.Fprintf(os.Stderr, " (not in module)\n")
			}
			continue
		}

		bookContent := extractBook(module, book)
		if len(bookContent.Chapters) > 0 {
			content.Books = append(content.Books, bookContent)
			if verbose {
				fmt.Fprintf(os.Stderr, " %d chapters\n", len(bookContent.Chapters))
			}
		} else {
			if verbose {
				fmt.Fprintf(os.Stderr, " (no content)\n")
			}
		}
	}

	return content, nil
}

func checkBookExists(module, bookName string) bool {
	cmd := exec.Command("diatheke", "-b", module, "-f", "plain", "-k", bookName+" 1:1")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	text := strings.TrimSpace(string(output))
	return len(text) > 0 && !strings.HasPrefix(text, "("+module+")")
}

func extractBook(module string, book Book) BookContent {
	bc := BookContent{
		ID:        book.ID,
		Name:      book.Name,
		Testament: book.Testament,
		Chapters:  make([]ChapterContent, 0),
	}

	for ch := 1; ch <= book.Chapters; ch++ {
		chapter := extractChapter(module, book.Name, ch)
		if len(chapter.Verses) > 0 {
			bc.Chapters = append(bc.Chapters, chapter)
		}
	}

	return bc
}

func extractChapter(module, bookName string, chapterNum int) ChapterContent {
	cc := ChapterContent{
		Number: chapterNum,
		Verses: make([]VerseContent, 0),
	}

	ref := fmt.Sprintf("%s %d", bookName, chapterNum)
	cmd := exec.Command("diatheke", "-b", module, "-f", "plain", "-k", ref)
	output, err := cmd.Output()
	if err != nil {
		return cc
	}

	verses := parseDiathekeOutput(string(output))
	cc.Verses = verses

	return cc
}

func parseDiathekeOutput(output string) []VerseContent {
	verses := make([]VerseContent, 0)

	// Remove module attribution line at the end
	output = regexp.MustCompile(`\n\([^)]+\)\s*$`).ReplaceAllString(strings.TrimSpace(output), "")

	// Split by verse references
	parts := regexp.MustCompile(`(?:^|\n)([A-Za-z0-9 ]+\s+\d+:\d+):\s*`).Split(output, -1)
	refs := regexp.MustCompile(`(?:^|\n)([A-Za-z0-9 ]+\s+\d+:\d+):\s*`).FindAllStringSubmatch(output, -1)

	for i, ref := range refs {
		if i+1 < len(parts) {
			text := strings.TrimSpace(parts[i+1])
			if text == "" {
				continue
			}

			// Skip placeholder text
			if isPlaceholderText(text) {
				continue
			}

			// Extract verse number from reference
			verseMatch := regexp.MustCompile(`:(\d+)$`).FindStringSubmatch(ref[1])
			if verseMatch == nil {
				continue
			}

			verseNum, _ := strconv.Atoi(verseMatch[1])
			text = strings.Join(strings.Fields(text), " ")

			verses = append(verses, VerseContent{
				Number: verseNum,
				Text:   text,
			})
		}
	}

	return verses
}

func isPlaceholderText(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) < 5 {
		return true
	}
	return placeholderPattern.MatchString(text)
}

func writeJSON(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(data)
}
