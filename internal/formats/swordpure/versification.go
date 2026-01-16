// versification.go implements Bible versification systems for SWORD modules.
// Different Bible traditions use different verse numbering schemes.
package swordpure

import (
	"fmt"
	"strings"
)

// VersificationID identifies a versification system.
type VersificationID string

// Standard versification systems.
const (
	VersKJV       VersificationID = "KJV"
	VersNRSV      VersificationID = "NRSV"
	VersNRSVA     VersificationID = "NRSVA"
	VersVulgate   VersificationID = "Vulgate"
	VersCatholic  VersificationID = "Catholic"
	VersLXX       VersificationID = "LXX"
	VersMT        VersificationID = "MT"
	VersSynodal   VersificationID = "Synodal"
	VersGerman    VersificationID = "German"
	VersLuther    VersificationID = "Luther"
	VersEthiopian VersificationID = "Ethiopian"
	VersLDS       VersificationID = "LDS" // LDS uses KJV Bible versification
)

// BookData contains verse counts for each chapter of a book.
type BookData struct {
	Name     string
	OSIS     string
	Chapters []int // Verse counts per chapter
}

// Versification contains the complete versification data for a system.
type Versification struct {
	ID    VersificationID
	Books []BookData
}

// ntBookSet contains all New Testament book OSIS IDs for quick lookup.
var ntBookSet = map[string]bool{
	"Matt": true, "Mark": true, "Luke": true, "John": true,
	"Acts": true, "Rom": true, "1Cor": true, "2Cor": true,
	"Gal": true, "Eph": true, "Phil": true, "Col": true,
	"1Thess": true, "2Thess": true, "1Tim": true, "2Tim": true,
	"Titus": true, "Phlm": true, "Heb": true, "Jas": true,
	"1Pet": true, "2Pet": true, "1John": true, "2John": true,
	"3John": true, "Jude": true, "Rev": true,
}

// NewVersification creates a versification instance for the given system.
func NewVersification(id VersificationID) (*Versification, error) {
	switch id {
	case VersKJV, VersLDS, "":
		// LDS uses KJV Bible versification (additional LDS scriptures are separate)
		return newKJVVersification(), nil
	case VersNRSV:
		return newNRSVVersification(), nil
	case VersVulgate, VersCatholic, "Vulg":
		return newVulgateVersification(), nil
	case VersEthiopian, "Orthodox", "Tewahedo":
		return newEthiopianVersification(), nil
	default:
		// Default to KJV for unknown systems
		return newKJVVersification(), nil
	}
}

// VersificationFromConf returns the versification for a conf file.
func VersificationFromConf(conf *ConfFile) (*Versification, error) {
	versStr := strings.TrimSpace(conf.Versification)
	if versStr == "" {
		versStr = "KJV"
	}
	return NewVersification(VersificationID(versStr))
}

// GetBookIndex returns the index of the book (0-65 for standard Bible).
func (v *Versification) GetBookIndex(book string) int {
	for i, b := range v.Books {
		if b.OSIS == book {
			return i
		}
	}
	return -1
}

// GetChapterCount returns the number of chapters in a book.
func (v *Versification) GetChapterCount(book string) int {
	idx := v.GetBookIndex(book)
	if idx < 0 || idx >= len(v.Books) {
		return 0
	}
	return len(v.Books[idx].Chapters)
}

// GetVerseCount returns the number of verses in a specific chapter.
func (v *Versification) GetVerseCount(book string, chapter int) int {
	idx := v.GetBookIndex(book)
	if idx < 0 || idx >= len(v.Books) {
		return 0
	}
	if chapter < 1 || chapter > len(v.Books[idx].Chapters) {
		return 0
	}
	return v.Books[idx].Chapters[chapter-1]
}

// GetTotalVerses returns the total verse count for a book.
func (v *Versification) GetTotalVerses(book string) int {
	idx := v.GetBookIndex(book)
	if idx < 0 || idx >= len(v.Books) {
		return 0
	}
	total := 0
	for _, count := range v.Books[idx].Chapters {
		total += count
	}
	return total
}

// GetOTBookCount returns the number of OT books in this versification.
// This is determined by finding the first NT book in the book list.
func (v *Versification) GetOTBookCount() int {
	for i, book := range v.Books {
		if ntBookSet[book.OSIS] {
			return i
		}
	}
	return len(v.Books) // All books are OT
}

// CalculateIndex calculates the absolute verse index for a reference.
// If isNT is true, indices are relative to the NT start (Matt = 0).
// SWORD modules use a complex indexing scheme with headers for:
// - Module (1 entry at start)
// - Each book (1 entry per book)
// - Each chapter (1 entry per chapter)
// Plus the actual verse entries.
func (v *Versification) CalculateIndex(ref *Ref, isNT bool) (int, error) {
	bookIdx := v.GetBookIndex(ref.Book)
	if bookIdx < 0 {
		return -1, fmt.Errorf("unknown book: %s", ref.Book)
	}

	// Adjust for testament - use dynamic OT book count for this versification
	otBookCount := v.GetOTBookCount()
	startBook := 0
	if isNT {
		startBook = otBookCount
		if bookIdx < otBookCount {
			return -1, fmt.Errorf("book %s is not in NT", ref.Book)
		}
		bookIdx -= otBookCount
	}

	book := v.Books[startBook+bookIdx]

	// Validate chapter and verse
	if ref.Chapter < 1 || ref.Chapter > len(book.Chapters) {
		return -1, fmt.Errorf("invalid chapter %d for %s", ref.Chapter, ref.Book)
	}
	if ref.Verse < 1 || ref.Verse > book.Chapters[ref.Chapter-1] {
		return -1, fmt.Errorf("invalid verse %d for %s %d", ref.Verse, ref.Book, ref.Chapter)
	}

	// Calculate index with SWORD headers:
	// [0] = empty slot
	// [1] = module header
	// [2] = book intro
	// [3] = chapter heading
	// [4+] = verses

	index := 2 // Empty slot + Module header

	// Add book intros + chapter headings + verses from previous books
	for i := 0; i < bookIdx; i++ {
		index++ // Book intro
		for _, count := range v.Books[startBook+i].Chapters {
			index++        // Chapter heading
			index += count // Verses in chapter
		}
	}

	// Add current book intro
	index++

	// Add chapter headings + verses from previous chapters in current book
	for c := 0; c < ref.Chapter-1; c++ {
		index++                   // Chapter heading
		index += book.Chapters[c] // Verses in chapter
	}

	// Add current chapter heading
	index++

	// Add current verse (1-based within chapter)
	index += ref.Verse - 1

	return index, nil
}

// IndexToRef converts an absolute verse index to a reference.
// Accounts for the SWORD header scheme: module(1) + book(1) + chapter(1) + verses.
func (v *Versification) IndexToRef(index int, isNT bool) (*Ref, error) {
	if index < 4 {
		return nil, fmt.Errorf("invalid index: %d (must be >= 4 for first verse)", index)
	}

	otBookCount := v.GetOTBookCount()
	startBook := 0
	endBook := otBookCount
	if isNT {
		startBook = otBookCount
		endBook = len(v.Books)
	}

	// Start after empty slot + module header (2)
	remaining := index - 2

	for bookIdx := startBook; bookIdx < endBook; bookIdx++ {
		book := v.Books[bookIdx]

		// Subtract book intro (1)
		remaining--
		if remaining < 0 {
			return nil, fmt.Errorf("index %d is a book intro", index)
		}

		for chapterIdx, verseCount := range book.Chapters {
			// Subtract chapter heading (1)
			remaining--
			if remaining < 0 {
				return nil, fmt.Errorf("index %d is a chapter heading", index)
			}

			// Check if the verse is in this chapter
			if remaining < verseCount {
				return &Ref{
					Book:    book.OSIS,
					Chapter: chapterIdx + 1,
					Verse:   remaining + 1,
				}, nil
			}

			remaining -= verseCount
		}
	}

	return nil, fmt.Errorf("index %d out of range", index)
}

// newKJVVersification creates the KJV versification system.
func newKJVVersification() *Versification {
	return &Versification{
		ID: VersKJV,
		Books: []BookData{
			// Old Testament
			{Name: "Genesis", OSIS: "Gen", Chapters: []int{31, 25, 24, 26, 32, 22, 24, 22, 29, 32, 32, 20, 18, 24, 21, 16, 27, 33, 38, 18, 34, 24, 20, 67, 34, 35, 46, 22, 35, 43, 55, 32, 20, 31, 29, 43, 36, 30, 23, 23, 57, 38, 34, 34, 28, 34, 31, 22, 33, 26}},
			{Name: "Exodus", OSIS: "Exod", Chapters: []int{22, 25, 22, 31, 23, 30, 25, 32, 35, 29, 10, 51, 22, 31, 27, 36, 16, 27, 25, 26, 36, 31, 33, 18, 40, 37, 21, 43, 46, 38, 18, 35, 23, 35, 35, 38, 29, 31, 43, 38}},
			{Name: "Leviticus", OSIS: "Lev", Chapters: []int{17, 16, 17, 35, 19, 30, 38, 36, 24, 20, 47, 8, 59, 57, 33, 34, 16, 30, 37, 27, 24, 33, 44, 23, 55, 46, 34}},
			{Name: "Numbers", OSIS: "Num", Chapters: []int{54, 34, 51, 49, 31, 27, 89, 26, 23, 36, 35, 16, 33, 45, 41, 50, 13, 32, 22, 29, 35, 41, 30, 25, 18, 65, 23, 31, 40, 16, 54, 42, 56, 29, 34, 13}},
			{Name: "Deuteronomy", OSIS: "Deut", Chapters: []int{46, 37, 29, 49, 33, 25, 26, 20, 29, 22, 32, 32, 18, 29, 23, 22, 20, 22, 21, 20, 23, 30, 25, 22, 19, 19, 26, 68, 29, 20, 30, 52, 29, 12}},
			{Name: "Joshua", OSIS: "Josh", Chapters: []int{18, 24, 17, 24, 15, 27, 26, 35, 27, 43, 23, 24, 33, 15, 63, 10, 18, 28, 51, 9, 45, 34, 16, 33}},
			{Name: "Judges", OSIS: "Judg", Chapters: []int{36, 23, 31, 24, 31, 40, 25, 35, 57, 18, 40, 15, 25, 20, 20, 31, 13, 31, 30, 48, 25}},
			{Name: "Ruth", OSIS: "Ruth", Chapters: []int{22, 23, 18, 22}},
			{Name: "1 Samuel", OSIS: "1Sam", Chapters: []int{28, 36, 21, 22, 12, 21, 17, 22, 27, 27, 15, 25, 23, 52, 35, 23, 58, 30, 24, 42, 15, 23, 29, 22, 44, 25, 12, 25, 11, 31, 13}},
			{Name: "2 Samuel", OSIS: "2Sam", Chapters: []int{27, 32, 39, 12, 25, 23, 29, 18, 13, 19, 27, 31, 39, 33, 37, 23, 29, 33, 43, 26, 22, 51, 39, 25}},
			{Name: "1 Kings", OSIS: "1Kgs", Chapters: []int{53, 46, 28, 34, 18, 38, 51, 66, 28, 29, 43, 33, 34, 31, 34, 34, 24, 46, 21, 43, 29, 53}},
			{Name: "2 Kings", OSIS: "2Kgs", Chapters: []int{18, 25, 27, 44, 27, 33, 20, 29, 37, 36, 21, 21, 25, 29, 38, 20, 41, 37, 37, 21, 26, 20, 37, 20, 30}},
			{Name: "1 Chronicles", OSIS: "1Chr", Chapters: []int{54, 55, 24, 43, 26, 81, 40, 40, 44, 14, 47, 40, 14, 17, 29, 43, 27, 17, 19, 8, 30, 19, 32, 31, 31, 32, 34, 21, 30}},
			{Name: "2 Chronicles", OSIS: "2Chr", Chapters: []int{17, 18, 17, 22, 14, 42, 22, 18, 31, 19, 23, 16, 22, 15, 19, 14, 19, 34, 11, 37, 20, 12, 21, 27, 28, 23, 9, 27, 36, 27, 21, 33, 25, 33, 27, 23}},
			{Name: "Ezra", OSIS: "Ezra", Chapters: []int{11, 70, 13, 24, 17, 22, 28, 36, 15, 44}},
			{Name: "Nehemiah", OSIS: "Neh", Chapters: []int{11, 20, 32, 23, 19, 19, 73, 18, 38, 39, 36, 47, 31}},
			{Name: "Esther", OSIS: "Esth", Chapters: []int{22, 23, 15, 17, 14, 14, 10, 17, 32, 3}},
			{Name: "Job", OSIS: "Job", Chapters: []int{22, 13, 26, 21, 27, 30, 21, 22, 35, 22, 20, 25, 28, 22, 35, 22, 16, 21, 29, 29, 34, 30, 17, 25, 6, 14, 23, 28, 25, 31, 40, 22, 33, 37, 16, 33, 24, 41, 30, 24, 34, 17}},
			{Name: "Psalms", OSIS: "Ps", Chapters: []int{6, 12, 8, 8, 12, 10, 17, 9, 20, 18, 7, 8, 6, 7, 5, 11, 15, 50, 14, 9, 13, 31, 6, 10, 22, 12, 14, 9, 11, 12, 24, 11, 22, 22, 28, 12, 40, 22, 13, 17, 13, 11, 5, 26, 17, 11, 9, 14, 20, 23, 19, 9, 6, 7, 23, 13, 11, 11, 17, 12, 8, 12, 11, 10, 13, 20, 7, 35, 36, 5, 24, 20, 28, 23, 10, 12, 20, 72, 13, 19, 16, 8, 18, 12, 13, 17, 7, 18, 52, 17, 16, 15, 5, 23, 11, 13, 12, 9, 9, 5, 8, 28, 22, 35, 45, 48, 43, 13, 31, 7, 10, 10, 9, 8, 18, 19, 2, 29, 176, 7, 8, 9, 4, 8, 5, 6, 5, 6, 8, 8, 3, 18, 3, 3, 21, 26, 9, 8, 24, 13, 10, 7, 12, 15, 21, 10, 20, 14, 9, 6}},
			{Name: "Proverbs", OSIS: "Prov", Chapters: []int{33, 22, 35, 27, 23, 35, 27, 36, 18, 32, 31, 28, 25, 35, 33, 33, 28, 24, 29, 30, 31, 29, 35, 34, 28, 28, 27, 28, 27, 33, 31}},
			{Name: "Ecclesiastes", OSIS: "Eccl", Chapters: []int{18, 26, 22, 16, 20, 12, 29, 17, 18, 20, 10, 14}},
			{Name: "Song of Solomon", OSIS: "Song", Chapters: []int{17, 17, 11, 16, 16, 13, 13, 14}},
			{Name: "Isaiah", OSIS: "Isa", Chapters: []int{31, 22, 26, 6, 30, 13, 25, 22, 21, 34, 16, 6, 22, 32, 9, 14, 14, 7, 25, 6, 17, 25, 18, 23, 12, 21, 13, 29, 24, 33, 9, 20, 24, 17, 10, 22, 38, 22, 8, 31, 29, 25, 28, 28, 25, 13, 15, 22, 26, 11, 23, 15, 12, 17, 13, 12, 21, 14, 21, 22, 11, 12, 19, 12, 25, 24}},
			{Name: "Jeremiah", OSIS: "Jer", Chapters: []int{19, 37, 25, 31, 31, 30, 34, 22, 26, 25, 23, 17, 27, 22, 21, 21, 27, 23, 15, 18, 14, 30, 40, 10, 38, 24, 22, 17, 32, 24, 40, 44, 26, 22, 19, 32, 21, 28, 18, 16, 18, 22, 13, 30, 5, 28, 7, 47, 39, 46, 64, 34}},
			{Name: "Lamentations", OSIS: "Lam", Chapters: []int{22, 22, 66, 22, 22}},
			{Name: "Ezekiel", OSIS: "Ezek", Chapters: []int{28, 10, 27, 17, 17, 14, 27, 18, 11, 22, 25, 28, 23, 23, 8, 63, 24, 32, 14, 49, 32, 31, 49, 27, 17, 21, 36, 26, 21, 26, 18, 32, 33, 31, 15, 38, 28, 23, 29, 49, 26, 20, 27, 31, 25, 24, 23, 35}},
			{Name: "Daniel", OSIS: "Dan", Chapters: []int{21, 49, 30, 37, 31, 28, 28, 27, 27, 21, 45, 13}},
			{Name: "Hosea", OSIS: "Hos", Chapters: []int{11, 23, 5, 19, 15, 11, 16, 14, 17, 15, 12, 14, 16, 9}},
			{Name: "Joel", OSIS: "Joel", Chapters: []int{20, 32, 21}},
			{Name: "Amos", OSIS: "Amos", Chapters: []int{15, 16, 15, 13, 27, 14, 17, 14, 15}},
			{Name: "Obadiah", OSIS: "Obad", Chapters: []int{21}},
			{Name: "Jonah", OSIS: "Jonah", Chapters: []int{17, 10, 10, 11}},
			{Name: "Micah", OSIS: "Mic", Chapters: []int{16, 13, 12, 13, 15, 16, 20}},
			{Name: "Nahum", OSIS: "Nah", Chapters: []int{15, 13, 19}},
			{Name: "Habakkuk", OSIS: "Hab", Chapters: []int{17, 20, 19}},
			{Name: "Zephaniah", OSIS: "Zeph", Chapters: []int{18, 15, 20}},
			{Name: "Haggai", OSIS: "Hag", Chapters: []int{15, 23}},
			{Name: "Zechariah", OSIS: "Zech", Chapters: []int{21, 13, 10, 14, 11, 15, 14, 23, 17, 12, 17, 14, 9, 21}},
			{Name: "Malachi", OSIS: "Mal", Chapters: []int{14, 17, 18, 6}},
			// New Testament
			{Name: "Matthew", OSIS: "Matt", Chapters: []int{25, 23, 17, 25, 48, 34, 29, 34, 38, 42, 30, 50, 58, 36, 39, 28, 27, 35, 30, 34, 46, 46, 39, 51, 46, 75, 66, 20}},
			{Name: "Mark", OSIS: "Mark", Chapters: []int{45, 28, 35, 41, 43, 56, 37, 38, 50, 52, 33, 44, 37, 72, 47, 20}},
			{Name: "Luke", OSIS: "Luke", Chapters: []int{80, 52, 38, 44, 39, 49, 50, 56, 62, 42, 54, 59, 35, 35, 32, 31, 37, 43, 48, 47, 38, 71, 56, 53}},
			{Name: "John", OSIS: "John", Chapters: []int{51, 25, 36, 54, 47, 71, 53, 59, 41, 42, 57, 50, 38, 31, 27, 33, 26, 40, 42, 31, 25}},
			{Name: "Acts", OSIS: "Acts", Chapters: []int{26, 47, 26, 37, 42, 15, 60, 40, 43, 48, 30, 25, 52, 28, 41, 40, 34, 28, 41, 38, 40, 30, 35, 27, 27, 32, 44, 31}},
			{Name: "Romans", OSIS: "Rom", Chapters: []int{32, 29, 31, 25, 21, 23, 25, 39, 33, 21, 36, 21, 14, 23, 33, 27}},
			{Name: "1 Corinthians", OSIS: "1Cor", Chapters: []int{31, 16, 23, 21, 13, 20, 40, 13, 27, 33, 34, 31, 13, 40, 58, 24}},
			{Name: "2 Corinthians", OSIS: "2Cor", Chapters: []int{24, 17, 18, 18, 21, 18, 16, 24, 15, 18, 33, 21, 14}},
			{Name: "Galatians", OSIS: "Gal", Chapters: []int{24, 21, 29, 31, 26, 18}},
			{Name: "Ephesians", OSIS: "Eph", Chapters: []int{23, 22, 21, 32, 33, 24}},
			{Name: "Philippians", OSIS: "Phil", Chapters: []int{30, 30, 21, 23}},
			{Name: "Colossians", OSIS: "Col", Chapters: []int{29, 23, 25, 18}},
			{Name: "1 Thessalonians", OSIS: "1Thess", Chapters: []int{10, 20, 13, 18, 28}},
			{Name: "2 Thessalonians", OSIS: "2Thess", Chapters: []int{12, 17, 18}},
			{Name: "1 Timothy", OSIS: "1Tim", Chapters: []int{20, 15, 16, 16, 25, 21}},
			{Name: "2 Timothy", OSIS: "2Tim", Chapters: []int{18, 26, 17, 22}},
			{Name: "Titus", OSIS: "Titus", Chapters: []int{16, 15, 15}},
			{Name: "Philemon", OSIS: "Phlm", Chapters: []int{25}},
			{Name: "Hebrews", OSIS: "Heb", Chapters: []int{14, 18, 19, 16, 14, 20, 28, 13, 28, 39, 40, 29, 25}},
			{Name: "James", OSIS: "Jas", Chapters: []int{27, 26, 18, 17, 20}},
			{Name: "1 Peter", OSIS: "1Pet", Chapters: []int{25, 25, 22, 19, 14}},
			{Name: "2 Peter", OSIS: "2Pet", Chapters: []int{21, 22, 18}},
			{Name: "1 John", OSIS: "1John", Chapters: []int{10, 29, 24, 21, 21}},
			{Name: "2 John", OSIS: "2John", Chapters: []int{13}},
			{Name: "3 John", OSIS: "3John", Chapters: []int{14}},
			{Name: "Jude", OSIS: "Jude", Chapters: []int{25}},
			{Name: "Revelation", OSIS: "Rev", Chapters: []int{20, 29, 22, 11, 14, 17, 17, 13, 21, 11, 19, 17, 18, 20, 8, 21, 18, 24, 21, 15, 27, 21}},
		},
	}
}

// newNRSVVersification creates the NRSV versification system.
// For now, returns same as KJV - most differences are minor.
func newNRSVVersification() *Versification {
	v := newKJVVersification()
	v.ID = VersNRSV
	return v
}

// newVulgateVersification creates the Vulgate versification system.
// Includes 73 books: 46 OT (with deuterocanonical) + 27 NT.
func newVulgateVersification() *Versification {
	return &Versification{
		ID: VersVulgate,
		Books: []BookData{
			// Old Testament (46 books including deuterocanonical)
			{Name: "Genesis", OSIS: "Gen", Chapters: []int{31, 25, 24, 26, 32, 22, 24, 22, 29, 32, 32, 20, 18, 24, 21, 16, 27, 33, 38, 18, 34, 24, 20, 67, 34, 35, 46, 22, 35, 43, 55, 32, 20, 31, 29, 43, 36, 30, 23, 23, 57, 38, 34, 34, 28, 34, 31, 22, 33, 26}},
			{Name: "Exodus", OSIS: "Exod", Chapters: []int{22, 25, 22, 31, 23, 30, 25, 32, 35, 29, 10, 51, 22, 31, 27, 36, 16, 27, 25, 26, 36, 31, 33, 18, 40, 37, 21, 43, 46, 38, 18, 35, 23, 35, 35, 38, 29, 31, 43, 38}},
			{Name: "Leviticus", OSIS: "Lev", Chapters: []int{17, 16, 17, 35, 19, 30, 38, 36, 24, 20, 47, 8, 59, 57, 33, 34, 16, 30, 37, 27, 24, 33, 44, 23, 55, 46, 34}},
			{Name: "Numbers", OSIS: "Num", Chapters: []int{54, 34, 51, 49, 31, 27, 89, 26, 23, 36, 35, 16, 33, 45, 41, 50, 13, 32, 22, 29, 35, 41, 30, 25, 18, 65, 23, 31, 40, 16, 54, 42, 56, 29, 34, 13}},
			{Name: "Deuteronomy", OSIS: "Deut", Chapters: []int{46, 37, 29, 49, 33, 25, 26, 20, 29, 22, 32, 32, 18, 29, 23, 22, 20, 22, 21, 20, 23, 30, 25, 22, 19, 19, 26, 68, 29, 20, 30, 52, 29, 12}},
			{Name: "Joshua", OSIS: "Josh", Chapters: []int{18, 24, 17, 24, 15, 27, 26, 35, 27, 43, 23, 24, 33, 15, 63, 10, 18, 28, 51, 9, 45, 34, 16, 33}},
			{Name: "Judges", OSIS: "Judg", Chapters: []int{36, 23, 31, 24, 31, 40, 25, 35, 57, 18, 40, 15, 25, 20, 20, 31, 13, 31, 30, 48, 25}},
			{Name: "Ruth", OSIS: "Ruth", Chapters: []int{22, 23, 18, 22}},
			{Name: "1 Samuel", OSIS: "1Sam", Chapters: []int{28, 36, 21, 22, 12, 21, 17, 22, 27, 27, 15, 25, 23, 52, 35, 23, 58, 30, 24, 42, 15, 23, 29, 22, 44, 25, 12, 25, 11, 31, 13}},
			{Name: "2 Samuel", OSIS: "2Sam", Chapters: []int{27, 32, 39, 12, 25, 23, 29, 18, 13, 19, 27, 31, 39, 33, 37, 23, 29, 33, 43, 26, 22, 51, 39, 25}},
			{Name: "1 Kings", OSIS: "1Kgs", Chapters: []int{53, 46, 28, 34, 18, 38, 51, 66, 28, 29, 43, 33, 34, 31, 34, 34, 24, 46, 21, 43, 29, 53}},
			{Name: "2 Kings", OSIS: "2Kgs", Chapters: []int{18, 25, 27, 44, 27, 33, 20, 29, 37, 36, 21, 21, 25, 29, 38, 20, 41, 37, 37, 21, 26, 20, 37, 20, 30}},
			{Name: "1 Chronicles", OSIS: "1Chr", Chapters: []int{54, 55, 24, 43, 26, 81, 40, 40, 44, 14, 47, 40, 14, 17, 29, 43, 27, 17, 19, 8, 30, 19, 32, 31, 31, 32, 34, 21, 30}},
			{Name: "2 Chronicles", OSIS: "2Chr", Chapters: []int{17, 18, 17, 22, 14, 42, 22, 18, 31, 19, 23, 16, 22, 15, 19, 14, 19, 34, 11, 37, 20, 12, 21, 27, 28, 23, 9, 27, 36, 27, 21, 33, 25, 33, 27, 23}},
			{Name: "Ezra", OSIS: "Ezra", Chapters: []int{11, 70, 13, 24, 17, 22, 28, 36, 15, 44}},
			{Name: "Nehemiah", OSIS: "Neh", Chapters: []int{11, 20, 32, 23, 19, 19, 73, 18, 38, 39, 36, 47, 31}},
			// Tobit (deuterocanonical)
			{Name: "Tobit", OSIS: "Tob", Chapters: []int{22, 14, 17, 21, 22, 18, 16, 21, 6, 13, 18, 22, 18, 15}},
			// Judith (deuterocanonical)
			{Name: "Judith", OSIS: "Jdt", Chapters: []int{16, 28, 10, 15, 24, 21, 32, 36, 14, 23, 23, 20, 20, 19, 14, 25}},
			{Name: "Esther", OSIS: "Esth", Chapters: []int{22, 23, 15, 17, 14, 14, 10, 17, 32, 3}},
			{Name: "Job", OSIS: "Job", Chapters: []int{22, 13, 26, 21, 27, 30, 21, 22, 35, 22, 20, 25, 28, 22, 35, 22, 16, 21, 29, 29, 34, 30, 17, 25, 6, 14, 23, 28, 25, 31, 40, 22, 33, 37, 16, 33, 24, 41, 30, 24, 34, 17}},
			{Name: "Psalms", OSIS: "Ps", Chapters: []int{6, 12, 8, 8, 12, 10, 17, 9, 20, 18, 7, 8, 6, 7, 5, 11, 15, 50, 14, 9, 13, 31, 6, 10, 22, 12, 14, 9, 11, 12, 24, 11, 22, 22, 28, 12, 40, 22, 13, 17, 13, 11, 5, 26, 17, 11, 9, 14, 20, 23, 19, 9, 6, 7, 23, 13, 11, 11, 17, 12, 8, 12, 11, 10, 13, 20, 7, 35, 36, 5, 24, 20, 28, 23, 10, 12, 20, 72, 13, 19, 16, 8, 18, 12, 13, 17, 7, 18, 52, 17, 16, 15, 5, 23, 11, 13, 12, 9, 9, 5, 8, 28, 22, 35, 45, 48, 43, 13, 31, 7, 10, 10, 9, 8, 18, 19, 2, 29, 176, 7, 8, 9, 4, 8, 5, 6, 5, 6, 8, 8, 3, 18, 3, 3, 21, 26, 9, 8, 24, 13, 10, 7, 12, 15, 21, 10, 20, 14, 9, 6}},
			{Name: "Proverbs", OSIS: "Prov", Chapters: []int{33, 22, 35, 27, 23, 35, 27, 36, 18, 32, 31, 28, 25, 35, 33, 33, 28, 24, 29, 30, 31, 29, 35, 34, 28, 28, 27, 28, 27, 33, 31}},
			{Name: "Ecclesiastes", OSIS: "Eccl", Chapters: []int{18, 26, 22, 16, 20, 12, 29, 17, 18, 20, 10, 14}},
			{Name: "Song of Solomon", OSIS: "Song", Chapters: []int{17, 17, 11, 16, 16, 13, 13, 14}},
			// Wisdom (deuterocanonical)
			{Name: "Wisdom", OSIS: "Wis", Chapters: []int{16, 24, 19, 20, 23, 25, 30, 21, 18, 21, 26, 27, 19, 31, 19, 29, 21, 25, 22}},
			// Sirach/Ecclesiasticus (deuterocanonical)
			{Name: "Sirach", OSIS: "Sir", Chapters: []int{30, 18, 31, 31, 15, 37, 36, 19, 18, 31, 34, 18, 26, 27, 20, 30, 32, 33, 30, 31, 28, 27, 27, 34, 26, 29, 30, 26, 28, 25, 31, 24, 33, 31, 26, 31, 31, 34, 35, 30, 22, 25, 33, 23, 26, 20, 25, 25, 16, 29, 30}},
			{Name: "Isaiah", OSIS: "Isa", Chapters: []int{31, 22, 26, 6, 30, 13, 25, 22, 21, 34, 16, 6, 22, 32, 9, 14, 14, 7, 25, 6, 17, 25, 18, 23, 12, 21, 13, 29, 24, 33, 9, 20, 24, 17, 10, 22, 38, 22, 8, 31, 29, 25, 28, 28, 25, 13, 15, 22, 26, 11, 23, 15, 12, 17, 13, 12, 21, 14, 21, 22, 11, 12, 19, 12, 25, 24}},
			{Name: "Jeremiah", OSIS: "Jer", Chapters: []int{19, 37, 25, 31, 31, 30, 34, 22, 26, 25, 23, 17, 27, 22, 21, 21, 27, 23, 15, 18, 14, 30, 40, 10, 38, 24, 22, 17, 32, 24, 40, 44, 26, 22, 19, 32, 21, 28, 18, 16, 18, 22, 13, 30, 5, 28, 7, 47, 39, 46, 64, 34}},
			{Name: "Lamentations", OSIS: "Lam", Chapters: []int{22, 22, 66, 22, 22}},
			// Baruch (deuterocanonical)
			{Name: "Baruch", OSIS: "Bar", Chapters: []int{22, 35, 38, 37, 9, 72}},
			{Name: "Ezekiel", OSIS: "Ezek", Chapters: []int{28, 10, 27, 17, 17, 14, 27, 18, 11, 22, 25, 28, 23, 23, 8, 63, 24, 32, 14, 49, 32, 31, 49, 27, 17, 21, 36, 26, 21, 26, 18, 32, 33, 31, 15, 38, 28, 23, 29, 49, 26, 20, 27, 31, 25, 24, 23, 35}},
			{Name: "Daniel", OSIS: "Dan", Chapters: []int{21, 49, 30, 37, 31, 28, 28, 27, 27, 21, 45, 13}},
			{Name: "Hosea", OSIS: "Hos", Chapters: []int{11, 23, 5, 19, 15, 11, 16, 14, 17, 15, 12, 14, 16, 9}},
			{Name: "Joel", OSIS: "Joel", Chapters: []int{20, 32, 21}},
			{Name: "Amos", OSIS: "Amos", Chapters: []int{15, 16, 15, 13, 27, 14, 17, 14, 15}},
			{Name: "Obadiah", OSIS: "Obad", Chapters: []int{21}},
			{Name: "Jonah", OSIS: "Jonah", Chapters: []int{17, 10, 10, 11}},
			{Name: "Micah", OSIS: "Mic", Chapters: []int{16, 13, 12, 13, 15, 16, 20}},
			{Name: "Nahum", OSIS: "Nah", Chapters: []int{15, 13, 19}},
			{Name: "Habakkuk", OSIS: "Hab", Chapters: []int{17, 20, 19}},
			{Name: "Zephaniah", OSIS: "Zeph", Chapters: []int{18, 15, 20}},
			{Name: "Haggai", OSIS: "Hag", Chapters: []int{15, 23}},
			{Name: "Zechariah", OSIS: "Zech", Chapters: []int{21, 13, 10, 14, 11, 15, 14, 23, 17, 12, 17, 14, 9, 21}},
			{Name: "Malachi", OSIS: "Mal", Chapters: []int{14, 17, 18, 6}},
			// 1 Maccabees (deuterocanonical)
			{Name: "1 Maccabees", OSIS: "1Macc", Chapters: []int{64, 70, 60, 61, 68, 63, 50, 32, 73, 89, 74, 53, 53, 49, 41, 24}},
			// 2 Maccabees (deuterocanonical)
			{Name: "2 Maccabees", OSIS: "2Macc", Chapters: []int{36, 32, 40, 50, 27, 31, 42, 36, 29, 38, 38, 45, 26, 46, 39}},
			// New Testament (27 books - Vulgate)
			{Name: "Matthew", OSIS: "Matt", Chapters: []int{25, 23, 17, 25, 48, 34, 29, 34, 38, 42, 30, 50, 58, 36, 39, 28, 27, 35, 30, 34, 46, 46, 39, 51, 46, 75, 66, 20}},
			{Name: "Mark", OSIS: "Mark", Chapters: []int{45, 28, 35, 41, 43, 56, 37, 38, 50, 52, 33, 44, 37, 72, 47, 20}},
			{Name: "Luke", OSIS: "Luke", Chapters: []int{80, 52, 38, 44, 39, 49, 50, 56, 62, 42, 54, 59, 35, 35, 32, 31, 37, 43, 48, 47, 38, 71, 56, 53}},
			{Name: "John", OSIS: "John", Chapters: []int{51, 25, 36, 54, 47, 71, 53, 59, 41, 42, 57, 50, 38, 31, 27, 33, 26, 40, 42, 31, 25}},
			{Name: "Acts", OSIS: "Acts", Chapters: []int{26, 47, 26, 37, 42, 15, 60, 40, 43, 48, 30, 25, 52, 28, 41, 40, 34, 28, 41, 38, 40, 30, 35, 27, 27, 32, 44, 31}},
			{Name: "Romans", OSIS: "Rom", Chapters: []int{32, 29, 31, 25, 21, 23, 25, 39, 33, 21, 36, 21, 14, 23, 33, 27}},
			{Name: "1 Corinthians", OSIS: "1Cor", Chapters: []int{31, 16, 23, 21, 13, 20, 40, 13, 27, 33, 34, 31, 13, 40, 58, 24}},
			{Name: "2 Corinthians", OSIS: "2Cor", Chapters: []int{24, 17, 18, 18, 21, 18, 16, 24, 15, 18, 33, 21, 14}},
			{Name: "Galatians", OSIS: "Gal", Chapters: []int{24, 21, 29, 31, 26, 18}},
			{Name: "Ephesians", OSIS: "Eph", Chapters: []int{23, 22, 21, 32, 33, 24}},
			{Name: "Philippians", OSIS: "Phil", Chapters: []int{30, 30, 21, 23}},
			{Name: "Colossians", OSIS: "Col", Chapters: []int{29, 23, 25, 18}},
			{Name: "1 Thessalonians", OSIS: "1Thess", Chapters: []int{10, 20, 13, 18, 28}},
			{Name: "2 Thessalonians", OSIS: "2Thess", Chapters: []int{12, 17, 18}},
			{Name: "1 Timothy", OSIS: "1Tim", Chapters: []int{20, 15, 16, 16, 25, 21}},
			{Name: "2 Timothy", OSIS: "2Tim", Chapters: []int{18, 26, 17, 22}},
			{Name: "Titus", OSIS: "Titus", Chapters: []int{16, 15, 15}},
			{Name: "Philemon", OSIS: "Phlm", Chapters: []int{25}},
			{Name: "Hebrews", OSIS: "Heb", Chapters: []int{14, 18, 19, 16, 14, 20, 28, 13, 28, 39, 40, 29, 25}},
			{Name: "James", OSIS: "Jas", Chapters: []int{27, 26, 18, 17, 20}},
			{Name: "1 Peter", OSIS: "1Pet", Chapters: []int{25, 25, 22, 19, 14}},
			{Name: "2 Peter", OSIS: "2Pet", Chapters: []int{21, 22, 18}},
			{Name: "1 John", OSIS: "1John", Chapters: []int{10, 29, 24, 21, 21}},
			{Name: "2 John", OSIS: "2John", Chapters: []int{13}},
			{Name: "3 John", OSIS: "3John", Chapters: []int{14}},
			{Name: "Jude", OSIS: "Jude", Chapters: []int{25}},
			{Name: "Revelation", OSIS: "Rev", Chapters: []int{20, 29, 22, 11, 14, 17, 17, 13, 21, 11, 19, 17, 18, 20, 8, 21, 18, 24, 21, 15, 27, 21}},
		},
	}
}

// newEthiopianVersification creates the Ethiopian Orthodox Tewahedo versification system.
// The Ethiopian Orthodox canon has 81 books (46 OT + 35 NT), the largest biblical canon.
// Unique books include: 1 Enoch, Jubilees, 4 Baruch, 3 Maccabees, Psalm 151.
// Some additional NT books (Sinodos, Didascalia, etc.) are rarely in digital form.
func newEthiopianVersification() *Versification {
	return &Versification{
		ID: VersEthiopian,
		Books: []BookData{
			// Old Testament (Ethiopian - includes deuterocanonical and unique Ethiopian books)
			{Name: "Genesis", OSIS: "Gen", Chapters: []int{31, 25, 24, 26, 32, 22, 24, 22, 29, 32, 32, 20, 18, 24, 21, 16, 27, 33, 38, 18, 34, 24, 20, 67, 34, 35, 46, 22, 35, 43, 55, 32, 20, 31, 29, 43, 36, 30, 23, 23, 57, 38, 34, 34, 28, 34, 31, 22, 33, 26}},
			{Name: "Exodus", OSIS: "Exod", Chapters: []int{22, 25, 22, 31, 23, 30, 25, 32, 35, 29, 10, 51, 22, 31, 27, 36, 16, 27, 25, 26, 36, 31, 33, 18, 40, 37, 21, 43, 46, 38, 18, 35, 23, 35, 35, 38, 29, 31, 43, 38}},
			{Name: "Leviticus", OSIS: "Lev", Chapters: []int{17, 16, 17, 35, 19, 30, 38, 36, 24, 20, 47, 8, 59, 57, 33, 34, 16, 30, 37, 27, 24, 33, 44, 23, 55, 46, 34}},
			{Name: "Numbers", OSIS: "Num", Chapters: []int{54, 34, 51, 49, 31, 27, 89, 26, 23, 36, 35, 16, 33, 45, 41, 50, 13, 32, 22, 29, 35, 41, 30, 25, 18, 65, 23, 31, 40, 16, 54, 42, 56, 29, 34, 13}},
			{Name: "Deuteronomy", OSIS: "Deut", Chapters: []int{46, 37, 29, 49, 33, 25, 26, 20, 29, 22, 32, 32, 18, 29, 23, 22, 20, 22, 21, 20, 23, 30, 25, 22, 19, 19, 26, 68, 29, 20, 30, 52, 29, 12}},
			{Name: "Joshua", OSIS: "Josh", Chapters: []int{18, 24, 17, 24, 15, 27, 26, 35, 27, 43, 23, 24, 33, 15, 63, 10, 18, 28, 51, 9, 45, 34, 16, 33}},
			{Name: "Judges", OSIS: "Judg", Chapters: []int{36, 23, 31, 24, 31, 40, 25, 35, 57, 18, 40, 15, 25, 20, 20, 31, 13, 31, 30, 48, 25}},
			{Name: "Ruth", OSIS: "Ruth", Chapters: []int{22, 23, 18, 22}},
			{Name: "1 Samuel", OSIS: "1Sam", Chapters: []int{28, 36, 21, 22, 12, 21, 17, 22, 27, 27, 15, 25, 23, 52, 35, 23, 58, 30, 24, 42, 15, 23, 29, 22, 44, 25, 12, 25, 11, 31, 13}},
			{Name: "2 Samuel", OSIS: "2Sam", Chapters: []int{27, 32, 39, 12, 25, 23, 29, 18, 13, 19, 27, 31, 39, 33, 37, 23, 29, 33, 43, 26, 22, 51, 39, 25}},
			{Name: "1 Kings", OSIS: "1Kgs", Chapters: []int{53, 46, 28, 34, 18, 38, 51, 66, 28, 29, 43, 33, 34, 31, 34, 34, 24, 46, 21, 43, 29, 53}},
			{Name: "2 Kings", OSIS: "2Kgs", Chapters: []int{18, 25, 27, 44, 27, 33, 20, 29, 37, 36, 21, 21, 25, 29, 38, 20, 41, 37, 37, 21, 26, 20, 37, 20, 30}},
			{Name: "1 Chronicles", OSIS: "1Chr", Chapters: []int{54, 55, 24, 43, 26, 81, 40, 40, 44, 14, 47, 40, 14, 17, 29, 43, 27, 17, 19, 8, 30, 19, 32, 31, 31, 32, 34, 21, 30}},
			{Name: "2 Chronicles", OSIS: "2Chr", Chapters: []int{17, 18, 17, 22, 14, 42, 22, 18, 31, 19, 23, 16, 22, 15, 19, 14, 19, 34, 11, 37, 20, 12, 21, 27, 28, 23, 9, 27, 36, 27, 21, 33, 25, 33, 27, 23}},
			// Jubilees - unique to Ethiopian canon (50 chapters)
			{Name: "Jubilees", OSIS: "Jub", Chapters: []int{29, 35, 35, 33, 19, 39, 40, 30, 15, 35, 32, 29, 29, 24, 34, 31, 17, 19, 29, 13, 25, 30, 32, 33, 23, 35, 27, 31, 20, 23, 32, 34, 23, 22, 27, 18, 25, 14, 12, 13, 11, 25, 16, 34, 16, 10, 12, 18, 11, 13}},
			// 1 Enoch (Ethiopic Enoch) - unique to Ethiopian canon (108 chapters)
			{Name: "1 Enoch", OSIS: "1En", Chapters: []int{9, 3, 15, 6, 10, 8, 6, 4, 11, 22, 2, 6, 10, 25, 12, 4, 8, 16, 3, 8, 10, 14, 4, 6, 7, 6, 5, 3, 2, 3, 3, 6, 4, 3, 6, 4, 6, 6, 14, 10, 9, 3, 4, 1, 6, 8, 4, 10, 4, 5, 5, 9, 7, 10, 4, 8, 3, 6, 3, 25, 13, 16, 12, 2, 12, 3, 13, 5, 30, 4, 17, 37, 8, 17, 9, 14, 9, 17, 6, 8, 10, 20, 11, 6, 10, 6, 4, 3, 77, 42, 19, 17, 14, 11, 7, 7, 10, 16, 16, 9, 9, 11, 15, 13, 2, 19, 3, 15}},
			{Name: "Ezra", OSIS: "Ezra", Chapters: []int{11, 70, 13, 24, 17, 22, 28, 36, 15, 44}},
			{Name: "Nehemiah", OSIS: "Neh", Chapters: []int{11, 20, 32, 23, 19, 19, 73, 18, 38, 39, 36, 47, 31}},
			// Tobit (deuterocanonical)
			{Name: "Tobit", OSIS: "Tob", Chapters: []int{22, 14, 17, 21, 22, 18, 16, 21, 6, 13, 18, 22, 18, 15}},
			// Judith (deuterocanonical)
			{Name: "Judith", OSIS: "Jdt", Chapters: []int{16, 28, 10, 15, 24, 21, 32, 36, 14, 23, 23, 20, 20, 19, 14, 25}},
			{Name: "Esther", OSIS: "Esth", Chapters: []int{22, 23, 15, 17, 14, 14, 10, 17, 32, 3}},
			{Name: "Job", OSIS: "Job", Chapters: []int{22, 13, 26, 21, 27, 30, 21, 22, 35, 22, 20, 25, 28, 22, 35, 22, 16, 21, 29, 29, 34, 30, 17, 25, 6, 14, 23, 28, 25, 31, 40, 22, 33, 37, 16, 33, 24, 41, 30, 24, 34, 17}},
			// Psalms with Psalm 151 (Ethiopian has 151 Psalms)
			{Name: "Psalms", OSIS: "Ps", Chapters: []int{6, 12, 8, 8, 12, 10, 17, 9, 20, 18, 7, 8, 6, 7, 5, 11, 15, 50, 14, 9, 13, 31, 6, 10, 22, 12, 14, 9, 11, 12, 24, 11, 22, 22, 28, 12, 40, 22, 13, 17, 13, 11, 5, 26, 17, 11, 9, 14, 20, 23, 19, 9, 6, 7, 23, 13, 11, 11, 17, 12, 8, 12, 11, 10, 13, 20, 7, 35, 36, 5, 24, 20, 28, 23, 10, 12, 20, 72, 13, 19, 16, 8, 18, 12, 13, 17, 7, 18, 52, 17, 16, 15, 5, 23, 11, 13, 12, 9, 9, 5, 8, 28, 22, 35, 45, 48, 43, 13, 31, 7, 10, 10, 9, 8, 18, 19, 2, 29, 176, 7, 8, 9, 4, 8, 5, 6, 5, 6, 8, 8, 3, 18, 3, 3, 21, 26, 9, 8, 24, 13, 10, 7, 12, 15, 21, 10, 20, 14, 9, 6, 7}},
			{Name: "Proverbs", OSIS: "Prov", Chapters: []int{33, 22, 35, 27, 23, 35, 27, 36, 18, 32, 31, 28, 25, 35, 33, 33, 28, 24, 29, 30, 31, 29, 35, 34, 28, 28, 27, 28, 27, 33, 31}},
			{Name: "Ecclesiastes", OSIS: "Eccl", Chapters: []int{18, 26, 22, 16, 20, 12, 29, 17, 18, 20, 10, 14}},
			{Name: "Song of Solomon", OSIS: "Song", Chapters: []int{17, 17, 11, 16, 16, 13, 13, 14}},
			// Wisdom (deuterocanonical)
			{Name: "Wisdom", OSIS: "Wis", Chapters: []int{16, 24, 19, 20, 23, 25, 30, 21, 18, 21, 26, 27, 19, 31, 19, 29, 21, 25, 22}},
			// Sirach/Ecclesiasticus (deuterocanonical)
			{Name: "Sirach", OSIS: "Sir", Chapters: []int{30, 18, 31, 31, 15, 37, 36, 19, 18, 31, 34, 18, 26, 27, 20, 30, 32, 33, 30, 31, 28, 27, 27, 34, 26, 29, 30, 26, 28, 25, 31, 24, 33, 31, 26, 31, 31, 34, 35, 30, 22, 25, 33, 23, 26, 20, 25, 25, 16, 29, 30}},
			{Name: "Isaiah", OSIS: "Isa", Chapters: []int{31, 22, 26, 6, 30, 13, 25, 22, 21, 34, 16, 6, 22, 32, 9, 14, 14, 7, 25, 6, 17, 25, 18, 23, 12, 21, 13, 29, 24, 33, 9, 20, 24, 17, 10, 22, 38, 22, 8, 31, 29, 25, 28, 28, 25, 13, 15, 22, 26, 11, 23, 15, 12, 17, 13, 12, 21, 14, 21, 22, 11, 12, 19, 12, 25, 24}},
			{Name: "Jeremiah", OSIS: "Jer", Chapters: []int{19, 37, 25, 31, 31, 30, 34, 22, 26, 25, 23, 17, 27, 22, 21, 21, 27, 23, 15, 18, 14, 30, 40, 10, 38, 24, 22, 17, 32, 24, 40, 44, 26, 22, 19, 32, 21, 28, 18, 16, 18, 22, 13, 30, 5, 28, 7, 47, 39, 46, 64, 34}},
			{Name: "Lamentations", OSIS: "Lam", Chapters: []int{22, 22, 66, 22, 22}},
			// Baruch (deuterocanonical) - includes Letter of Jeremiah as chapter 6
			{Name: "Baruch", OSIS: "Bar", Chapters: []int{22, 35, 38, 37, 9, 72}},
			// 4 Baruch (Paralipomena of Jeremiah) - Ethiopian addition
			{Name: "4 Baruch", OSIS: "4Bar", Chapters: []int{12, 10, 22, 11, 35, 25, 37, 14, 32}},
			{Name: "Ezekiel", OSIS: "Ezek", Chapters: []int{28, 10, 27, 17, 17, 14, 27, 18, 11, 22, 25, 28, 23, 23, 8, 63, 24, 32, 14, 49, 32, 31, 49, 27, 17, 21, 36, 26, 21, 26, 18, 32, 33, 31, 15, 38, 28, 23, 29, 49, 26, 20, 27, 31, 25, 24, 23, 35}},
			{Name: "Daniel", OSIS: "Dan", Chapters: []int{21, 49, 30, 37, 31, 28, 28, 27, 27, 21, 45, 13}},
			{Name: "Hosea", OSIS: "Hos", Chapters: []int{11, 23, 5, 19, 15, 11, 16, 14, 17, 15, 12, 14, 16, 9}},
			{Name: "Joel", OSIS: "Joel", Chapters: []int{20, 32, 21}},
			{Name: "Amos", OSIS: "Amos", Chapters: []int{15, 16, 15, 13, 27, 14, 17, 14, 15}},
			{Name: "Obadiah", OSIS: "Obad", Chapters: []int{21}},
			{Name: "Jonah", OSIS: "Jonah", Chapters: []int{17, 10, 10, 11}},
			{Name: "Micah", OSIS: "Mic", Chapters: []int{16, 13, 12, 13, 15, 16, 20}},
			{Name: "Nahum", OSIS: "Nah", Chapters: []int{15, 13, 19}},
			{Name: "Habakkuk", OSIS: "Hab", Chapters: []int{17, 20, 19}},
			{Name: "Zephaniah", OSIS: "Zeph", Chapters: []int{18, 15, 20}},
			{Name: "Haggai", OSIS: "Hag", Chapters: []int{15, 23}},
			{Name: "Zechariah", OSIS: "Zech", Chapters: []int{21, 13, 10, 14, 11, 15, 14, 23, 17, 12, 17, 14, 9, 21}},
			{Name: "Malachi", OSIS: "Mal", Chapters: []int{14, 17, 18, 6}},
			// 1 Maccabees (deuterocanonical)
			{Name: "1 Maccabees", OSIS: "1Macc", Chapters: []int{64, 70, 60, 61, 68, 63, 50, 32, 73, 89, 74, 53, 53, 49, 41, 24}},
			// 2 Maccabees (deuterocanonical)
			{Name: "2 Maccabees", OSIS: "2Macc", Chapters: []int{36, 32, 40, 50, 27, 31, 42, 36, 29, 38, 38, 45, 26, 46, 39}},
			// 3 Maccabees - Ethiopian addition (also in some Eastern Orthodox canons)
			{Name: "3 Maccabees", OSIS: "3Macc", Chapters: []int{29, 33, 30, 21, 51, 41, 23}},
			// New Testament (27 standard books - Ethiopian)
			{Name: "Matthew", OSIS: "Matt", Chapters: []int{25, 23, 17, 25, 48, 34, 29, 34, 38, 42, 30, 50, 58, 36, 39, 28, 27, 35, 30, 34, 46, 46, 39, 51, 46, 75, 66, 20}},
			{Name: "Mark", OSIS: "Mark", Chapters: []int{45, 28, 35, 41, 43, 56, 37, 38, 50, 52, 33, 44, 37, 72, 47, 20}},
			{Name: "Luke", OSIS: "Luke", Chapters: []int{80, 52, 38, 44, 39, 49, 50, 56, 62, 42, 54, 59, 35, 35, 32, 31, 37, 43, 48, 47, 38, 71, 56, 53}},
			{Name: "John", OSIS: "John", Chapters: []int{51, 25, 36, 54, 47, 71, 53, 59, 41, 42, 57, 50, 38, 31, 27, 33, 26, 40, 42, 31, 25}},
			{Name: "Acts", OSIS: "Acts", Chapters: []int{26, 47, 26, 37, 42, 15, 60, 40, 43, 48, 30, 25, 52, 28, 41, 40, 34, 28, 41, 38, 40, 30, 35, 27, 27, 32, 44, 31}},
			{Name: "Romans", OSIS: "Rom", Chapters: []int{32, 29, 31, 25, 21, 23, 25, 39, 33, 21, 36, 21, 14, 23, 33, 27}},
			{Name: "1 Corinthians", OSIS: "1Cor", Chapters: []int{31, 16, 23, 21, 13, 20, 40, 13, 27, 33, 34, 31, 13, 40, 58, 24}},
			{Name: "2 Corinthians", OSIS: "2Cor", Chapters: []int{24, 17, 18, 18, 21, 18, 16, 24, 15, 18, 33, 21, 14}},
			{Name: "Galatians", OSIS: "Gal", Chapters: []int{24, 21, 29, 31, 26, 18}},
			{Name: "Ephesians", OSIS: "Eph", Chapters: []int{23, 22, 21, 32, 33, 24}},
			{Name: "Philippians", OSIS: "Phil", Chapters: []int{30, 30, 21, 23}},
			{Name: "Colossians", OSIS: "Col", Chapters: []int{29, 23, 25, 18}},
			{Name: "1 Thessalonians", OSIS: "1Thess", Chapters: []int{10, 20, 13, 18, 28}},
			{Name: "2 Thessalonians", OSIS: "2Thess", Chapters: []int{12, 17, 18}},
			{Name: "1 Timothy", OSIS: "1Tim", Chapters: []int{20, 15, 16, 16, 25, 21}},
			{Name: "2 Timothy", OSIS: "2Tim", Chapters: []int{18, 26, 17, 22}},
			{Name: "Titus", OSIS: "Titus", Chapters: []int{16, 15, 15}},
			{Name: "Philemon", OSIS: "Phlm", Chapters: []int{25}},
			{Name: "Hebrews", OSIS: "Heb", Chapters: []int{14, 18, 19, 16, 14, 20, 28, 13, 28, 39, 40, 29, 25}},
			{Name: "James", OSIS: "Jas", Chapters: []int{27, 26, 18, 17, 20}},
			{Name: "1 Peter", OSIS: "1Pet", Chapters: []int{25, 25, 22, 19, 14}},
			{Name: "2 Peter", OSIS: "2Pet", Chapters: []int{21, 22, 18}},
			{Name: "1 John", OSIS: "1John", Chapters: []int{10, 29, 24, 21, 21}},
			{Name: "2 John", OSIS: "2John", Chapters: []int{13}},
			{Name: "3 John", OSIS: "3John", Chapters: []int{14}},
			{Name: "Jude", OSIS: "Jude", Chapters: []int{25}},
			{Name: "Revelation", OSIS: "Rev", Chapters: []int{20, 29, 22, 11, 14, 17, 17, 13, 21, 11, 19, 17, 18, 20, 8, 21, 18, 24, 21, 15, 27, 21}},
			// Note: Ethiopian broader canon NT additions (Sinodos, Didascalia, Clement, etc.)
			// are not included here as they rarely exist in digital form with verse structures
		},
	}
}
