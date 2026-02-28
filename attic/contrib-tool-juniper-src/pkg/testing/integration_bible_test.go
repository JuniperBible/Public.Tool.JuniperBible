package testing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// stripXMLTags removes XML/OSIS markup from text for comparison
// This is used for Bibles that have Strong's numbers or morphology embedded
var xmlTagPattern = regexp.MustCompile(`<[^>]+>`)

func stripXMLTags(text string) string {
	return strings.TrimSpace(xmlTagPattern.ReplaceAllString(text, ""))
}

// normalizeText prepares text for comparison by stripping tags and normalizing whitespace
func normalizeText(text string) string {
	text = stripXMLTags(text)
	// Normalize whitespace
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// BibleMetadata mirrors the output structure for testing
type BibleMetadata struct {
	Bibles []BibleEntry `json:"bibles"`
	Meta   MetaInfo     `json:"meta"`
}

type BibleEntry struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Abbrev      string   `json:"abbrev"`
	Language    string   `json:"language"`
	Features    []string `json:"features"`
	Tags        []string `json:"tags"`
	Weight      int      `json:"weight"`
}

type MetaInfo struct {
	Granularity string `json:"granularity"`
	Generated   string `json:"generated"`
	Version     string `json:"version"`
}

type BibleAuxiliary struct {
	Bibles map[string]BibleContent `json:"bibles"`
}

type BibleContent struct {
	Content  string           `json:"content"`
	Books    []BookContent    `json:"books"`
	Sections []ContentSection `json:"sections,omitempty"`
}

type BookContent struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Testament string           `json:"testament"`
	Chapters  []ChapterContent `json:"chapters"`
}

type ChapterContent struct {
	Number int            `json:"number"`
	Verses []VerseContent `json:"verses"`
}

type VerseContent struct {
	Number     int      `json:"number"`
	Text       string   `json:"text"`
	Strongs    []string `json:"strongs,omitempty"`
	Morphology []string `json:"morphology,omitempty"`
}

type ContentSection struct {
	Heading string `json:"heading"`
	Content string `json:"content,omitempty"`
}

// HistoricBible represents expected data for a historic Bible
type HistoricBible struct {
	ID       string
	Title    string
	Language string
	Gen1_1   string // Genesis 1:1 text
	John1_1  string // John 1:1 text
	John3_16 string // John 3:16 text
}

// TestHistoricBibles contains the 5 available historic Bibles with expected content
// Note: Coverdale is not available in SWORD repository
var TestHistoricBibles = []HistoricBible{
	{
		ID:       "kjv",
		Title:    "King James Version (1769) with Strongs Numbers and Morphology  and CatchWords",
		Language: "en",
		Gen1_1:   "In the beginning God created the heaven and the earth.",
		John1_1:  "In the beginning was the Word, and the Word was with God, and the Word was God.",
		John3_16: "For God so loved the world, that he gave his only begotten Son, that whosoever believeth in him should not perish, but have everlasting life.",
	},
	{
		ID:       "drc",
		Title:    "Douay-Rheims Bible, Challoner Revision",
		Language: "en",
		Gen1_1:   "In the beginning God created heaven, and earth.",
		John1_1:  "In the beginning was the Word: and the Word was with God: and the Word was God.",
		John3_16: "For God so loved the world, as to give his only begotten Son: that whosoever believeth in him may not perish, but may have life everlasting.",
	},
	{
		ID:       "geneva1599",
		Title:    "Geneva Bible (1599)",
		Language: "en",
		Gen1_1:   "In the beginning God created the heauen and the earth.",
		John1_1:  "In the beginning was that Word, and that Word was with God, and that Word was God.",
		John3_16: "For God so loued the worlde, that hee hath giuen his onely begotten Sonne, that whosoeuer beleeueth in him, should not perish, but haue euerlasting life.",
	},
	{
		ID:       "vulgate",
		Title:    "Latin Vulgate",
		Language: "la",
		Gen1_1:   "in principio creavit Deus caelum et terram",
		John1_1:  "in principio erat Verbum et Verbum erat apud Deum et Deus erat Verbum",
		John3_16: "sic enim dilexit Deus mundum ut Filium suum unigenitum daret ut omnis qui credit in eum non pereat sed habeat vitam aeternam",
	},
	{
		ID:       "tyndale",
		Title:    "William Tyndale Bible (1525/1530)",
		Language: "en",
		Gen1_1:   "In the begynnynge God created heaven and erth.",
		John1_1:  "In the beginnynge was the worde and the worde was with God: and the worde was God.",
		John3_16: "For God so loveth the worlde yt he hath geven his only sonne that none that beleve in him shuld perisshe: but shuld have everlastinge lyfe.",
	},
}

// SourceTextBible represents expected data for a Greek/Hebrew source text
type SourceTextBible struct {
	ID         string
	Title      string
	Language   string
	HasStrongs bool
	HasMorph   bool
	Testament  string // "OT", "NT", or "both"
}

// TestSourceTextBibles contains the Greek/Hebrew source texts with expected metadata
// These have markup with Strong's numbers and morphology
var TestSourceTextBibles = []SourceTextBible{
	{
		ID:         "sblgnt",
		Title:      "The Greek New Testament: SBL Edition",
		Language:   "grc",
		HasStrongs: false, // SBLGNT doesn't have Strong's in the module
		HasMorph:   false,
		Testament:  "NT",
	},
	{
		ID:         "lxx",
		Title:      "Septuagint, Morphologically Tagged Rahlfs'",
		Language:   "grc",
		HasStrongs: true,
		HasMorph:   true,
		Testament:  "OT",
	},
	{
		ID:         "osmhb",
		Title:      "Open Scriptures Morphological Hebrew Bible (morphology forthcoming)",
		Language:   "he",
		HasStrongs: true,
		HasMorph:   false, // Morphology is listed as "forthcoming"
		Testament:  "OT",
	},
}

// getTestDataPath returns the path to the data directory
func getTestDataPath() string {
	// First try the project root data directory
	path := filepath.Join("..", "..", "..", "..", "data")
	if _, err := os.Stat(filepath.Join(path, "bibles.json")); err == nil {
		return path
	}

	// Try current directory
	path = "data"
	if _, err := os.Stat(filepath.Join(path, "bibles.json")); err == nil {
		return path
	}

	// Try going up from tools/juniper
	path = filepath.Join("..", "..", "data")
	if _, err := os.Stat(filepath.Join(path, "bibles.json")); err == nil {
		return path
	}

	return ""
}

// loadBibleMetadata loads the bibles.json file
func loadBibleMetadata(t *testing.T) *BibleMetadata {
	t.Helper()

	dataPath := getTestDataPath()
	if dataPath == "" {
		t.Skip("bibles.json not found - run juniper convert first")
	}

	path := filepath.Join(dataPath, "bibles.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("Cannot read bibles.json: %v", err)
	}

	var metadata BibleMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("Cannot parse bibles.json: %v", err)
	}

	return &metadata
}

// loadBibleAuxiliary loads individual Bible files from bibles_auxiliary/ directory
func loadBibleAuxiliary(t *testing.T) *BibleAuxiliary {
	t.Helper()

	dataPath := getTestDataPath()
	if dataPath == "" {
		t.Skip("bibles_auxiliary directory not found - run juniper convert first")
	}

	auxDir := filepath.Join(dataPath, "bibles_auxiliary")
	entries, err := os.ReadDir(auxDir)
	if err != nil {
		t.Skipf("Cannot read bibles_auxiliary directory: %v", err)
	}

	auxiliary := &BibleAuxiliary{
		Bibles: make(map[string]BibleContent),
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(auxDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Cannot read %s: %v", entry.Name(), err)
		}

		var content BibleContent
		if err := json.Unmarshal(data, &content); err != nil {
			t.Fatalf("Cannot parse %s: %v", entry.Name(), err)
		}

		// Extract Bible ID from filename (e.g., "kjv.json" -> "kjv")
		bibleID := strings.TrimSuffix(entry.Name(), ".json")
		auxiliary.Bibles[bibleID] = content
	}

	return auxiliary
}

// TestIntegration_BibleDataFiles_Exist checks that the Bible data files exist
func TestIntegration_BibleDataFiles_Exist(t *testing.T) {
	dataPath := getTestDataPath()
	if dataPath == "" {
		t.Skip("Data directory not found")
	}

	// Check bibles.json exists
	metaPath := filepath.Join(dataPath, "bibles.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("Expected bibles.json to exist")
	}

	// Check bibles_auxiliary directory exists and has files
	auxDir := filepath.Join(dataPath, "bibles_auxiliary")
	if _, err := os.Stat(auxDir); os.IsNotExist(err) {
		t.Error("Expected bibles_auxiliary directory to exist")
	}

	entries, err := os.ReadDir(auxDir)
	if err != nil {
		t.Fatalf("Cannot read bibles_auxiliary directory: %v", err)
	}
	if len(entries) == 0 {
		t.Error("Expected bibles_auxiliary directory to contain files")
	}
}

// TestIntegration_BibleMetadata_ValidJSON checks that bibles.json is valid JSON
func TestIntegration_BibleMetadata_ValidJSON(t *testing.T) {
	metadata := loadBibleMetadata(t)

	if len(metadata.Bibles) == 0 {
		t.Error("Expected at least one Bible in metadata")
	}

	if metadata.Meta.Granularity == "" {
		t.Error("Expected granularity to be set")
	}

	if metadata.Meta.Version == "" {
		t.Error("Expected version to be set")
	}
}

// TestIntegration_BibleAuxiliary_ValidJSON checks that bibles_auxiliary files are valid JSON
func TestIntegration_BibleAuxiliary_ValidJSON(t *testing.T) {
	auxiliary := loadBibleAuxiliary(t)

	if len(auxiliary.Bibles) == 0 {
		t.Error("Expected at least one Bible in auxiliary data")
	}
}

// TestIntegration_AllFiveHistoricBibles_Present checks that all 5 available historic Bibles are present
func TestIntegration_AllFiveHistoricBibles_Present(t *testing.T) {
	metadata := loadBibleMetadata(t)

	bibleIDs := make(map[string]bool)
	for _, bible := range metadata.Bibles {
		bibleIDs[bible.ID] = true
	}

	for _, historic := range TestHistoricBibles {
		if !bibleIDs[historic.ID] {
			t.Errorf("Historic Bible %s not found in bibles.json", historic.ID)
		}
	}
}

// TestIntegration_HistoricBibles_Metadata validates metadata for each historic Bible
func TestIntegration_HistoricBibles_Metadata(t *testing.T) {
	metadata := loadBibleMetadata(t)

	for _, expected := range TestHistoricBibles {
		t.Run(expected.ID, func(t *testing.T) {
			var found *BibleEntry
			for i := range metadata.Bibles {
				if metadata.Bibles[i].ID == expected.ID {
					found = &metadata.Bibles[i]
					break
				}
			}

			if found == nil {
				t.Fatalf("Bible %s not found", expected.ID)
			}

			if found.Title != expected.Title {
				t.Errorf("Title mismatch: got %q, want %q", found.Title, expected.Title)
			}

			if found.Language != expected.Language {
				t.Errorf("Language mismatch: got %q, want %q", found.Language, expected.Language)
			}
		})
	}
}

// TestIntegration_HistoricBibles_Content validates content exists for each historic Bible
func TestIntegration_HistoricBibles_Content(t *testing.T) {
	auxiliary := loadBibleAuxiliary(t)

	for _, expected := range TestHistoricBibles {
		t.Run(expected.ID, func(t *testing.T) {
			content, ok := auxiliary.Bibles[expected.ID]
			if !ok {
				t.Fatalf("Bible %s not found in auxiliary data", expected.ID)
			}

			if content.Content == "" {
				t.Error("Expected content description to be non-empty")
			}

			if len(content.Books) == 0 {
				t.Error("Expected at least one book")
			}
		})
	}
}

// TestIntegration_HistoricBibles_Genesis1_1 validates Genesis 1:1 for each historic Bible
// Note: Skips content validation for Bibles with non-KJV versification (e.g., Vulgate)
// which may have different verse numbering.
func TestIntegration_HistoricBibles_Genesis1_1(t *testing.T) {
	auxiliary := loadBibleAuxiliary(t)

	for _, expected := range TestHistoricBibles {
		t.Run(expected.ID+"_Gen1_1", func(t *testing.T) {
			content, ok := auxiliary.Bibles[expected.ID]
			if !ok {
				t.Fatalf("Bible %s not found", expected.ID)
			}

			// Find Genesis
			var genesis *BookContent
			for i := range content.Books {
				if content.Books[i].ID == "Gen" {
					genesis = &content.Books[i]
					break
				}
			}

			if genesis == nil {
				t.Fatal("Genesis not found")
			}

			if genesis.Testament != "OT" {
				t.Errorf("Expected Genesis to be OT, got %s", genesis.Testament)
			}

			// Find chapter 1
			var chapter1 *ChapterContent
			for i := range genesis.Chapters {
				if genesis.Chapters[i].Number == 1 {
					chapter1 = &genesis.Chapters[i]
					break
				}
			}

			if chapter1 == nil {
				t.Fatal("Genesis chapter 1 not found")
			}

			// Find verse 1
			var verse1 *VerseContent
			for i := range chapter1.Verses {
				if chapter1.Verses[i].Number == 1 {
					verse1 = &chapter1.Verses[i]
					break
				}
			}

			if verse1 == nil {
				t.Fatal("Genesis 1:1 not found")
			}

			// Compare normalized text (strips XML markup for modules with Strong's)
			got := normalizeText(verse1.Text)
			want := normalizeText(expected.Gen1_1)
			if got != want {
				t.Errorf("Genesis 1:1 text mismatch:\n\tgot:  %q\n\twant: %q", got, want)
			}
		})
	}
}

// TestIntegration_HistoricBibles_John1_1 validates John 1:1 for each historic Bible
// Note: Skips DRC and Vulgate which use Vulgate versification with different numbering
func TestIntegration_HistoricBibles_John1_1(t *testing.T) {
	auxiliary := loadBibleAuxiliary(t)

	// Bibles that use Vulgate versification and may have different numbering
	skipVersification := map[string]bool{"drc": true, "vulgate": true}

	for _, expected := range TestHistoricBibles {
		if skipVersification[expected.ID] {
			continue
		}
		t.Run(expected.ID+"_John1_1", func(t *testing.T) {
			content, ok := auxiliary.Bibles[expected.ID]
			if !ok {
				t.Fatalf("Bible %s not found", expected.ID)
			}

			// Find John
			var john *BookContent
			for i := range content.Books {
				if content.Books[i].ID == "John" {
					john = &content.Books[i]
					break
				}
			}

			if john == nil {
				t.Fatal("John not found")
			}

			if john.Testament != "NT" {
				t.Errorf("Expected John to be NT, got %s", john.Testament)
			}

			// Find chapter 1
			var chapter1 *ChapterContent
			for i := range john.Chapters {
				if john.Chapters[i].Number == 1 {
					chapter1 = &john.Chapters[i]
					break
				}
			}

			if chapter1 == nil {
				t.Fatal("John chapter 1 not found")
			}

			// Find verse 1
			var verse1 *VerseContent
			for i := range chapter1.Verses {
				if chapter1.Verses[i].Number == 1 {
					verse1 = &chapter1.Verses[i]
					break
				}
			}

			if verse1 == nil {
				t.Fatal("John 1:1 not found")
			}

			// Compare normalized text (strips XML markup for modules with Strong's)
			got := normalizeText(verse1.Text)
			want := normalizeText(expected.John1_1)
			if got != want {
				t.Errorf("John 1:1 text mismatch:\n\tgot:  %q\n\twant: %q", got, want)
			}
		})
	}
}

// TestIntegration_HistoricBibles_John3_16 validates John 3:16 for each historic Bible
// Note: Skips DRC and Vulgate which use Vulgate versification with different numbering
func TestIntegration_HistoricBibles_John3_16(t *testing.T) {
	auxiliary := loadBibleAuxiliary(t)

	// Bibles that use Vulgate versification and may have different numbering
	skipVersification := map[string]bool{"drc": true, "vulgate": true}

	for _, expected := range TestHistoricBibles {
		if skipVersification[expected.ID] {
			continue
		}
		t.Run(expected.ID+"_John3_16", func(t *testing.T) {
			content, ok := auxiliary.Bibles[expected.ID]
			if !ok {
				t.Fatalf("Bible %s not found", expected.ID)
			}

			// Find John
			var john *BookContent
			for i := range content.Books {
				if content.Books[i].ID == "John" {
					john = &content.Books[i]
					break
				}
			}

			if john == nil {
				t.Fatal("John not found")
			}

			// Find chapter 3
			var chapter3 *ChapterContent
			for i := range john.Chapters {
				if john.Chapters[i].Number == 3 {
					chapter3 = &john.Chapters[i]
					break
				}
			}

			if chapter3 == nil {
				t.Fatal("John chapter 3 not found")
			}

			// Find verse 16
			var verse16 *VerseContent
			for i := range chapter3.Verses {
				if chapter3.Verses[i].Number == 16 {
					verse16 = &chapter3.Verses[i]
					break
				}
			}

			if verse16 == nil {
				t.Fatal("John 3:16 not found")
			}

			// Compare normalized text (strips XML markup for modules with Strong's)
			got := normalizeText(verse16.Text)
			want := normalizeText(expected.John3_16)
			if got != want {
				t.Errorf("John 3:16 text mismatch:\n\tgot:  %q\n\twant: %q", got, want)
			}
		})
	}
}

// TestIntegration_BibleContent_NoEmptyVerses checks that there are no empty verses
func TestIntegration_BibleContent_NoEmptyVerses(t *testing.T) {
	auxiliary := loadBibleAuxiliary(t)

	for bibleID, content := range auxiliary.Bibles {
		t.Run(bibleID, func(t *testing.T) {
			for _, book := range content.Books {
				for _, chapter := range book.Chapters {
					for _, verse := range chapter.Verses {
						if strings.TrimSpace(verse.Text) == "" {
							t.Errorf("Empty verse found: %s %s %d:%d",
								bibleID, book.ID, chapter.Number, verse.Number)
						}
					}
				}
			}
		})
	}
}

// TestIntegration_BibleContent_AllBooksHaveChapters checks that all books have at least one chapter
func TestIntegration_BibleContent_AllBooksHaveChapters(t *testing.T) {
	auxiliary := loadBibleAuxiliary(t)

	for bibleID, content := range auxiliary.Bibles {
		t.Run(bibleID, func(t *testing.T) {
			for _, book := range content.Books {
				if len(book.Chapters) == 0 {
					t.Errorf("Book %s has no chapters in %s", book.ID, bibleID)
				}
			}
		})
	}
}

// TestIntegration_BibleContent_AllChaptersHaveVerses checks that all chapters have at least one verse
// Note: Skips source texts (LXX, OSMHB, SBLGNT) which may have empty chapters for books
// outside their testament/canon scope.
func TestIntegration_BibleContent_AllChaptersHaveVerses(t *testing.T) {
	auxiliary := loadBibleAuxiliary(t)

	// Skip source texts with partial coverage
	skipBibles := map[string]bool{
		"lxx":     true, // OT only
		"osmhb":   true, // OT (Hebrew) only
		"sblgnt":  true, // NT (Greek) only
		"tyndale": true, // Incomplete (NT + Pentateuch)
	}

	for bibleID, content := range auxiliary.Bibles {
		if skipBibles[bibleID] {
			continue
		}
		t.Run(bibleID, func(t *testing.T) {
			for _, book := range content.Books {
				for _, chapter := range book.Chapters {
					if len(chapter.Verses) == 0 {
						t.Errorf("Chapter %s %d has no verses in %s",
							book.ID, chapter.Number, bibleID)
					}
				}
			}
		})
	}
}

// TestIntegration_BibleMetadata_UniqueIDs checks that all Bible IDs are unique
func TestIntegration_BibleMetadata_UniqueIDs(t *testing.T) {
	metadata := loadBibleMetadata(t)

	seen := make(map[string]bool)
	for _, bible := range metadata.Bibles {
		if seen[bible.ID] {
			t.Errorf("Duplicate Bible ID: %s", bible.ID)
		}
		seen[bible.ID] = true
	}
}

// TestIntegration_BibleMetadata_WeightsArePositive checks that all weights are positive
func TestIntegration_BibleMetadata_WeightsArePositive(t *testing.T) {
	metadata := loadBibleMetadata(t)

	for _, bible := range metadata.Bibles {
		if bible.Weight <= 0 {
			t.Errorf("Bible %s has non-positive weight: %d", bible.ID, bible.Weight)
		}
	}
}

// TestIntegration_BibleMetadata_LanguageIsValid checks that all languages are valid ISO codes
func TestIntegration_BibleMetadata_LanguageIsValid(t *testing.T) {
	metadata := loadBibleMetadata(t)

	validLanguages := map[string]bool{
		"en":  true, // English
		"la":  true, // Latin
		"el":  true, // Modern Greek
		"grc": true, // Ancient Greek (Biblical)
		"he":  true, // Hebrew
		"de":  true, // German
		"fr":  true, // French
		"es":  true, // Spanish
	}

	for _, bible := range metadata.Bibles {
		if bible.Language == "" {
			t.Errorf("Bible %s has empty language", bible.ID)
		} else if !validLanguages[bible.Language] {
			t.Logf("Bible %s has uncommon language: %s (this may be valid)", bible.ID, bible.Language)
		}
	}
}

// TestIntegration_MetadataAndAuxiliary_Consistent checks that metadata and auxiliary data are consistent
func TestIntegration_MetadataAndAuxiliary_Consistent(t *testing.T) {
	metadata := loadBibleMetadata(t)
	auxiliary := loadBibleAuxiliary(t)

	// Check all metadata entries have corresponding auxiliary entries
	for _, bible := range metadata.Bibles {
		if _, ok := auxiliary.Bibles[bible.ID]; !ok {
			t.Errorf("Bible %s in metadata but not in auxiliary", bible.ID)
		}
	}

	// Check all auxiliary entries have corresponding metadata entries
	metadataIDs := make(map[string]bool)
	for _, bible := range metadata.Bibles {
		metadataIDs[bible.ID] = true
	}

	for id := range auxiliary.Bibles {
		if !metadataIDs[id] {
			t.Errorf("Bible %s in auxiliary but not in metadata", id)
		}
	}
}

// TestIntegration_SourceTextBibles_Present checks that Greek/Hebrew source texts are present
func TestIntegration_SourceTextBibles_Present(t *testing.T) {
	auxiliary := loadBibleAuxiliary(t)

	for _, expected := range TestSourceTextBibles {
		t.Run(expected.ID, func(t *testing.T) {
			content, ok := auxiliary.Bibles[expected.ID]
			if !ok {
				t.Skipf("Source text %s not found in auxiliary data (may not be converted yet)", expected.ID)
				return
			}

			if len(content.Books) == 0 {
				t.Error("Expected at least one book")
			}

			// Verify expected testament
			switch expected.Testament {
			case "OT":
				hasOT := false
				for _, book := range content.Books {
					if book.Testament == "OT" {
						hasOT = true
						break
					}
				}
				if !hasOT {
					t.Error("Expected OT books for Old Testament source text")
				}
			case "NT":
				hasNT := false
				for _, book := range content.Books {
					if book.Testament == "NT" {
						hasNT = true
						break
					}
				}
				if !hasNT {
					t.Error("Expected NT books for New Testament source text")
				}
			}
		})
	}
}

// TestIntegration_SourceTextBibles_HasMarkup checks that source texts contain markup tags
func TestIntegration_SourceTextBibles_HasMarkup(t *testing.T) {
	auxiliary := loadBibleAuxiliary(t)

	for _, expected := range TestSourceTextBibles {
		t.Run(expected.ID, func(t *testing.T) {
			content, ok := auxiliary.Bibles[expected.ID]
			if !ok {
				t.Skipf("Source text %s not found", expected.ID)
				return
			}

			// Check that verse text contains OSIS/XML markup
			hasMarkup := false
			for _, book := range content.Books {
				for _, chapter := range book.Chapters {
					for _, verse := range chapter.Verses {
						if strings.Contains(verse.Text, "<w") || strings.Contains(verse.Text, "<seg") {
							hasMarkup = true
							break
						}
					}
					if hasMarkup {
						break
					}
				}
				if hasMarkup {
					break
				}
			}

			if !hasMarkup {
				t.Error("Expected source text to contain XML/OSIS markup (e.g., <w> or <seg> tags)")
			}
		})
	}
}

// TestIntegration_SourceTextBibles_ValidLanguage checks that source texts have correct language
func TestIntegration_SourceTextBibles_ValidLanguage(t *testing.T) {
	metadata := loadBibleMetadata(t)

	// Build a map of Bible IDs to entries
	bibleMap := make(map[string]BibleEntry)
	for _, bible := range metadata.Bibles {
		bibleMap[bible.ID] = bible
	}

	for _, expected := range TestSourceTextBibles {
		t.Run(expected.ID, func(t *testing.T) {
			bible, ok := bibleMap[expected.ID]
			if !ok {
				t.Skipf("Source text %s not found in metadata", expected.ID)
				return
			}

			if bible.Language != expected.Language {
				t.Errorf("Language mismatch: got %q, want %q", bible.Language, expected.Language)
			}
		})
	}
}
