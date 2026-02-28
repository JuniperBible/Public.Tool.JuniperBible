package sword

import "strings"

// BookInfo contains information about a Bible book.
type BookInfo struct {
	ID        string // OSIS ID (e.g., "Gen", "Matt")
	Name      string // Full name (e.g., "Genesis", "Matthew")
	Abbrev    string // Common abbreviation
	Testament string // "OT" or "NT"
	Chapters  int    // Number of chapters in KJV versification
}

// KJVBooks is the canonical list of books in KJV versification order.
var KJVBooks = []BookInfo{
	// Old Testament
	{"Gen", "Genesis", "Gen", "OT", 50},
	{"Exod", "Exodus", "Exod", "OT", 40},
	{"Lev", "Leviticus", "Lev", "OT", 27},
	{"Num", "Numbers", "Num", "OT", 36},
	{"Deut", "Deuteronomy", "Deut", "OT", 34},
	{"Josh", "Joshua", "Josh", "OT", 24},
	{"Judg", "Judges", "Judg", "OT", 21},
	{"Ruth", "Ruth", "Ruth", "OT", 4},
	{"1Sam", "1 Samuel", "1Sam", "OT", 31},
	{"2Sam", "2 Samuel", "2Sam", "OT", 24},
	{"1Kgs", "1 Kings", "1Kgs", "OT", 22},
	{"2Kgs", "2 Kings", "2Kgs", "OT", 25},
	{"1Chr", "1 Chronicles", "1Chr", "OT", 29},
	{"2Chr", "2 Chronicles", "2Chr", "OT", 36},
	{"Ezra", "Ezra", "Ezra", "OT", 10},
	{"Neh", "Nehemiah", "Neh", "OT", 13},
	{"Esth", "Esther", "Esth", "OT", 10},
	{"Job", "Job", "Job", "OT", 42},
	{"Ps", "Psalms", "Ps", "OT", 150},
	{"Prov", "Proverbs", "Prov", "OT", 31},
	{"Eccl", "Ecclesiastes", "Eccl", "OT", 12},
	{"Song", "Song of Solomon", "Song", "OT", 8},
	{"Isa", "Isaiah", "Isa", "OT", 66},
	{"Jer", "Jeremiah", "Jer", "OT", 52},
	{"Lam", "Lamentations", "Lam", "OT", 5},
	{"Ezek", "Ezekiel", "Ezek", "OT", 48},
	{"Dan", "Daniel", "Dan", "OT", 12},
	{"Hos", "Hosea", "Hos", "OT", 14},
	{"Joel", "Joel", "Joel", "OT", 3},
	{"Amos", "Amos", "Amos", "OT", 9},
	{"Obad", "Obadiah", "Obad", "OT", 1},
	{"Jonah", "Jonah", "Jonah", "OT", 4},
	{"Mic", "Micah", "Mic", "OT", 7},
	{"Nah", "Nahum", "Nah", "OT", 3},
	{"Hab", "Habakkuk", "Hab", "OT", 3},
	{"Zeph", "Zephaniah", "Zeph", "OT", 3},
	{"Hag", "Haggai", "Hag", "OT", 2},
	{"Zech", "Zechariah", "Zech", "OT", 14},
	{"Mal", "Malachi", "Mal", "OT", 4},

	// New Testament
	{"Matt", "Matthew", "Matt", "NT", 28},
	{"Mark", "Mark", "Mark", "NT", 16},
	{"Luke", "Luke", "Luke", "NT", 24},
	{"John", "John", "John", "NT", 21},
	{"Acts", "Acts", "Acts", "NT", 28},
	{"Rom", "Romans", "Rom", "NT", 16},
	{"1Cor", "1 Corinthians", "1Cor", "NT", 16},
	{"2Cor", "2 Corinthians", "2Cor", "NT", 13},
	{"Gal", "Galatians", "Gal", "NT", 6},
	{"Eph", "Ephesians", "Eph", "NT", 6},
	{"Phil", "Philippians", "Phil", "NT", 4},
	{"Col", "Colossians", "Col", "NT", 4},
	{"1Thess", "1 Thessalonians", "1Thess", "NT", 5},
	{"2Thess", "2 Thessalonians", "2Thess", "NT", 3},
	{"1Tim", "1 Timothy", "1Tim", "NT", 6},
	{"2Tim", "2 Timothy", "2Tim", "NT", 4},
	{"Titus", "Titus", "Titus", "NT", 3},
	{"Phlm", "Philemon", "Phlm", "NT", 1},
	{"Heb", "Hebrews", "Heb", "NT", 13},
	{"Jas", "James", "Jas", "NT", 5},
	{"1Pet", "1 Peter", "1Pet", "NT", 5},
	{"2Pet", "2 Peter", "2Pet", "NT", 3},
	{"1John", "1 John", "1John", "NT", 5},
	{"2John", "2 John", "2John", "NT", 1},
	{"3John", "3 John", "3John", "NT", 1},
	{"Jude", "Jude", "Jude", "NT", 1},
	{"Rev", "Revelation", "Rev", "NT", 22},
}

// bookIndex maps OSIS book IDs to their index in KJVBooks.
var bookIndex map[string]int

// bookAliases maps alternative book names to OSIS IDs.
var bookAliases map[string]string

func init() {
	bookIndex = make(map[string]int)
	for i, book := range KJVBooks {
		bookIndex[book.ID] = i
	}

	// Common aliases for book names
	bookAliases = map[string]string{
		// OT aliases
		"genesis":       "Gen",
		"exodus":        "Exod",
		"leviticus":     "Lev",
		"numbers":       "Num",
		"deuteronomy":   "Deut",
		"joshua":        "Josh",
		"judges":        "Judg",
		"1 samuel":      "1Sam",
		"2 samuel":      "2Sam",
		"1 kings":       "1Kgs",
		"2 kings":       "2Kgs",
		"1 chronicles":  "1Chr",
		"2 chronicles":  "2Chr",
		"nehemiah":      "Neh",
		"esther":        "Esth",
		"psalms":        "Ps",
		"psalm":         "Ps",
		"proverbs":      "Prov",
		"ecclesiastes":  "Eccl",
		"song of songs": "Song",
		"isaiah":        "Isa",
		"jeremiah":      "Jer",
		"lamentations":  "Lam",
		"ezekiel":       "Ezek",
		"daniel":        "Dan",
		"hosea":         "Hos",
		"obadiah":       "Obad",
		"micah":         "Mic",
		"nahum":         "Nah",
		"habakkuk":      "Hab",
		"zephaniah":     "Zeph",
		"haggai":        "Hag",
		"zechariah":     "Zech",
		"malachi":       "Mal",

		// NT aliases
		"matthew":         "Matt",
		"romans":          "Rom",
		"1 corinthians":   "1Cor",
		"2 corinthians":   "2Cor",
		"galatians":       "Gal",
		"ephesians":       "Eph",
		"philippians":     "Phil",
		"colossians":      "Col",
		"1 thessalonians": "1Thess",
		"2 thessalonians": "2Thess",
		"1 timothy":       "1Tim",
		"2 timothy":       "2Tim",
		"philemon":        "Phlm",
		"hebrews":         "Heb",
		"james":           "Jas",
		"1 peter":         "1Pet",
		"2 peter":         "2Pet",
		"1 john":          "1John",
		"2 john":          "2John",
		"3 john":          "3John",
		"revelation":      "Rev",
		"revelations":     "Rev",
	}
}

// GetBookInfo returns info for a book by ID or alias.
func GetBookInfo(bookRef string) (*BookInfo, bool) {
	// Try direct lookup first
	if idx, ok := bookIndex[bookRef]; ok {
		return &KJVBooks[idx], true
	}

	// Try lowercase alias lookup
	if osisID, ok := bookAliases[strings.ToLower(bookRef)]; ok {
		if idx, ok := bookIndex[osisID]; ok {
			return &KJVBooks[idx], true
		}
	}

	return nil, false
}

// GetBookIndex returns the index of a book in the canonical order.
func GetBookIndex(bookID string) (int, bool) {
	idx, ok := bookIndex[bookID]
	return idx, ok
}

// IsOldTestament returns true if the book is in the Old Testament.
func IsOldTestament(bookID string) bool {
	if book, ok := GetBookInfo(bookID); ok {
		return book.Testament == "OT"
	}
	return false
}

// IsNewTestament returns true if the book is in the New Testament.
func IsNewTestament(bookID string) bool {
	if book, ok := GetBookInfo(bookID); ok {
		return book.Testament == "NT"
	}
	return false
}

// NormalizeBookID converts various book references to the canonical OSIS ID.
func NormalizeBookID(bookRef string) string {
	if book, ok := GetBookInfo(bookRef); ok {
		return book.ID
	}
	return bookRef
}

// KJVVerseCounts contains the number of verses per chapter for each book in KJV versification.
// Data derived from SWORD Project via PySword (https://gitlab.com/tgc-dk/pysword)
var KJVVerseCounts = map[string][]int{
	"Gen":    {31, 25, 24, 26, 32, 22, 24, 22, 29, 32, 32, 20, 18, 24, 21, 16, 27, 33, 38, 18, 34, 24, 20, 67, 34, 35, 46, 22, 35, 43, 55, 32, 20, 31, 29, 43, 36, 30, 23, 23, 57, 38, 34, 34, 28, 34, 31, 22, 33, 26},
	"Exod":   {22, 25, 22, 31, 23, 30, 25, 32, 35, 29, 10, 51, 22, 31, 27, 36, 16, 27, 25, 26, 36, 31, 33, 18, 40, 37, 21, 43, 46, 38, 18, 35, 23, 35, 35, 38, 29, 31, 43, 38},
	"Lev":    {17, 16, 17, 35, 19, 30, 38, 36, 24, 20, 47, 8, 59, 57, 33, 34, 16, 30, 37, 27, 24, 33, 44, 23, 55, 46, 34},
	"Num":    {54, 34, 51, 49, 31, 27, 89, 26, 23, 36, 35, 16, 33, 45, 41, 50, 13, 32, 22, 29, 35, 41, 30, 25, 18, 65, 23, 31, 40, 16, 54, 42, 56, 29, 34, 13},
	"Deut":   {46, 37, 29, 49, 33, 25, 26, 20, 29, 22, 32, 32, 18, 29, 23, 22, 20, 22, 21, 20, 23, 30, 25, 22, 19, 19, 26, 68, 29, 20, 30, 52, 29, 12},
	"Josh":   {18, 24, 17, 24, 15, 27, 26, 35, 27, 43, 23, 24, 33, 15, 63, 10, 18, 28, 51, 9, 45, 34, 16, 33},
	"Judg":   {36, 23, 31, 24, 31, 40, 25, 35, 57, 18, 40, 15, 25, 20, 20, 31, 13, 31, 30, 48, 25},
	"Ruth":   {22, 23, 18, 22},
	"1Sam":   {28, 36, 21, 22, 12, 21, 17, 22, 27, 27, 15, 25, 23, 52, 35, 23, 58, 30, 24, 42, 15, 23, 29, 22, 44, 25, 12, 25, 11, 31, 13},
	"2Sam":   {27, 32, 39, 12, 25, 23, 29, 18, 13, 19, 27, 31, 39, 33, 37, 23, 29, 33, 43, 26, 22, 51, 39, 25},
	"1Kgs":   {53, 46, 28, 34, 18, 38, 51, 66, 28, 29, 43, 33, 34, 31, 34, 34, 24, 46, 21, 43, 29, 53},
	"2Kgs":   {18, 25, 27, 44, 27, 33, 20, 29, 37, 36, 21, 21, 25, 29, 38, 20, 41, 37, 37, 21, 26, 20, 37, 20, 30},
	"1Chr":   {54, 55, 24, 43, 26, 81, 40, 40, 44, 14, 47, 40, 14, 17, 29, 43, 27, 17, 19, 8, 30, 19, 32, 31, 31, 32, 34, 21, 30},
	"2Chr":   {17, 18, 17, 22, 14, 42, 22, 18, 31, 19, 23, 16, 22, 15, 19, 14, 19, 34, 11, 37, 20, 12, 21, 27, 28, 23, 9, 27, 36, 27, 21, 33, 25, 33, 27, 23},
	"Ezra":   {11, 70, 13, 24, 17, 22, 28, 36, 15, 44},
	"Neh":    {11, 20, 32, 23, 19, 19, 73, 18, 38, 39, 36, 47, 31},
	"Esth":   {22, 23, 15, 17, 14, 14, 10, 17, 32, 3},
	"Job":    {22, 13, 26, 21, 27, 30, 21, 22, 35, 22, 20, 25, 28, 22, 35, 22, 16, 21, 29, 29, 34, 30, 17, 25, 6, 14, 23, 28, 25, 31, 40, 22, 33, 37, 16, 33, 24, 41, 30, 24, 34, 17},
	"Ps":     {6, 12, 8, 8, 12, 10, 17, 9, 20, 18, 7, 8, 6, 7, 5, 11, 15, 50, 14, 9, 13, 31, 6, 10, 22, 12, 14, 9, 11, 12, 24, 11, 22, 22, 28, 12, 40, 22, 13, 17, 13, 11, 5, 26, 17, 11, 9, 14, 20, 23, 19, 9, 6, 7, 23, 13, 11, 11, 17, 12, 8, 12, 11, 10, 13, 20, 7, 35, 36, 5, 24, 20, 28, 23, 10, 12, 20, 72, 13, 19, 16, 8, 18, 12, 13, 17, 7, 18, 52, 17, 16, 15, 5, 23, 11, 13, 12, 9, 9, 5, 8, 28, 22, 35, 45, 48, 43, 13, 31, 7, 10, 10, 9, 8, 18, 19, 2, 29, 176, 7, 8, 9, 4, 8, 5, 6, 5, 6, 8, 8, 3, 18, 3, 3, 21, 26, 9, 8, 24, 13, 10, 7, 12, 15, 21, 10, 20, 14, 9, 6},
	"Prov":   {33, 22, 35, 27, 23, 35, 27, 36, 18, 32, 31, 28, 25, 35, 33, 33, 28, 24, 29, 30, 31, 29, 35, 34, 28, 28, 27, 28, 27, 33, 31},
	"Eccl":   {18, 26, 22, 16, 20, 12, 29, 17, 18, 20, 10, 14},
	"Song":   {17, 17, 11, 16, 16, 13, 13, 14},
	"Isa":    {31, 22, 26, 6, 30, 13, 25, 22, 21, 34, 16, 6, 22, 32, 9, 14, 14, 7, 25, 6, 17, 25, 18, 23, 12, 21, 13, 29, 24, 33, 9, 20, 24, 17, 10, 22, 38, 22, 8, 31, 29, 25, 28, 28, 25, 13, 15, 22, 26, 11, 23, 15, 12, 17, 13, 12, 21, 14, 21, 22, 11, 12, 19, 12, 25, 24},
	"Jer":    {19, 37, 25, 31, 31, 30, 34, 22, 26, 25, 23, 17, 27, 22, 21, 21, 27, 23, 15, 18, 14, 30, 40, 10, 38, 24, 22, 17, 32, 24, 40, 44, 26, 22, 19, 32, 21, 28, 18, 16, 18, 22, 13, 30, 5, 28, 7, 47, 39, 46, 64, 34},
	"Lam":    {22, 22, 66, 22, 22},
	"Ezek":   {28, 10, 27, 17, 17, 14, 27, 18, 11, 22, 25, 28, 23, 23, 8, 63, 24, 32, 14, 49, 32, 31, 49, 27, 17, 21, 36, 26, 21, 26, 18, 32, 33, 31, 15, 38, 28, 23, 29, 49, 26, 20, 27, 31, 25, 24, 23, 35},
	"Dan":    {21, 49, 30, 37, 31, 28, 28, 27, 27, 21, 45, 13},
	"Hos":    {11, 23, 5, 19, 15, 11, 16, 14, 17, 15, 12, 14, 16, 9},
	"Joel":   {20, 32, 21},
	"Amos":   {15, 16, 15, 13, 27, 14, 17, 14, 15},
	"Obad":   {21},
	"Jonah":  {17, 10, 10, 11},
	"Mic":    {16, 13, 12, 13, 15, 16, 20},
	"Nah":    {15, 13, 19},
	"Hab":    {17, 20, 19},
	"Zeph":   {18, 15, 20},
	"Hag":    {15, 23},
	"Zech":   {21, 13, 10, 14, 11, 15, 14, 23, 17, 12, 17, 14, 9, 21},
	"Mal":    {14, 17, 18, 6},
	"Matt":   {25, 23, 17, 25, 48, 34, 29, 34, 38, 42, 30, 50, 58, 36, 39, 28, 27, 35, 30, 34, 46, 46, 39, 51, 46, 75, 66, 20},
	"Mark":   {45, 28, 35, 41, 43, 56, 37, 38, 50, 52, 33, 44, 37, 72, 47, 20},
	"Luke":   {80, 52, 38, 44, 39, 49, 50, 56, 62, 42, 54, 59, 35, 35, 32, 31, 37, 43, 48, 47, 38, 71, 56, 53},
	"John":   {51, 25, 36, 54, 47, 71, 53, 59, 41, 42, 57, 50, 38, 31, 27, 33, 26, 40, 42, 31, 25},
	"Acts":   {26, 47, 26, 37, 42, 15, 60, 40, 43, 48, 30, 25, 52, 28, 41, 40, 34, 28, 41, 38, 40, 30, 35, 27, 27, 32, 44, 31},
	"Rom":    {32, 29, 31, 25, 21, 23, 25, 39, 33, 21, 36, 21, 14, 23, 33, 27},
	"1Cor":   {31, 16, 23, 21, 13, 20, 40, 13, 27, 33, 34, 31, 13, 40, 58, 24},
	"2Cor":   {24, 17, 18, 18, 21, 18, 16, 24, 15, 18, 33, 21, 14},
	"Gal":    {24, 21, 29, 31, 26, 18},
	"Eph":    {23, 22, 21, 32, 33, 24},
	"Phil":   {30, 30, 21, 23},
	"Col":    {29, 23, 25, 18},
	"1Thess": {10, 20, 13, 18, 28},
	"2Thess": {12, 17, 18},
	"1Tim":   {20, 15, 16, 16, 25, 21},
	"2Tim":   {18, 26, 17, 22},
	"Titus":  {16, 15, 15},
	"Phlm":   {25},
	"Heb":    {14, 18, 19, 16, 14, 20, 28, 13, 28, 39, 40, 29, 25},
	"Jas":    {27, 26, 18, 17, 20},
	"1Pet":   {25, 25, 22, 19, 14},
	"2Pet":   {21, 22, 18},
	"1John":  {10, 29, 24, 21, 21},
	"2John":  {13},
	"3John":  {14},
	"Jude":   {25},
	"Rev":    {20, 29, 22, 11, 14, 17, 17, 13, 21, 11, 19, 17, 18, 20, 8, 21, 18, 24, 21, 15, 27, 21},
}

// GetVersesInChapter returns the number of verses in a specific chapter.
func GetVersesInChapter(bookID string, chapter int) int {
	if verses, ok := KJVVerseCounts[bookID]; ok {
		if chapter >= 1 && chapter <= len(verses) {
			return verses[chapter-1]
		}
	}
	return 0
}

// CalculateVerseIndex computes the absolute verse index for a reference within a testament.
// This matches the SWORD/pysword index calculation which includes intro entries.
//
// SWORD Index Structure (per testament):
// - Entry 0-1: Testament heading (placeholder + header)
// - Per book: 1 book intro entry
// - Per chapter: 1 chapter intro entry
// - Then: actual verse entries
//
// Formula from pysword books.py:
//
//	testament_offset = 2 (skip testament heading)
//	book_offset = sum(book_sizes for previous books) where book_size = verses + chapters + 1
//	chapter_offset = sum(verse_counts for previous chapters) + chapter_number + 1 (for book title)
//	verse_index = testament_offset + book_offset + chapter_offset + verse - 1
func CalculateVerseIndex(bookID string, chapter, verse int) int {
	return calculateIndexWithIntros(bookID, chapter, verse, 0)
}

// calculateIndexWithIntros calculates the verse index accounting for intro entries.
// startBookIdx is the first book index to consider (0 for OT, 39 for NT).
func calculateIndexWithIntros(bookID string, chapter, verse, startBookIdx int) int {
	idx, ok := GetBookIndex(bookID)
	if !ok {
		return -1
	}

	// Start after testament heading (2 entries: placeholder + header)
	index := 2

	// Add sizes of all previous books in this testament
	// Book size = total verses + number of chapters (chapter intros) + 1 (book intro)
	for i := startBookIdx; i < idx; i++ {
		book := KJVBooks[i]
		if verses, ok := KJVVerseCounts[book.ID]; ok {
			totalVerses := 0
			for _, v := range verses {
				totalVerses += v
			}
			// Book size: all verses + chapter headings + book heading
			index += totalVerses + len(verses) + 1
		}
	}

	// Add book intro for current book
	index += 1

	// Add chapter intros and verses from previous chapters
	if verses, ok := KJVVerseCounts[bookID]; ok {
		for ch := 1; ch < chapter; ch++ {
			if ch <= len(verses) {
				index += 1 // Chapter intro
				index += verses[ch-1]
			}
		}
	}

	// Add current chapter's intro
	index += 1

	// Add verse position (verse is 1-based, becomes 0-based offset)
	index += verse - 1

	return index
}

// CalculateOTVerseIndex computes the verse index within just the Old Testament.
// Returns -1 if the book is not in the OT.
func CalculateOTVerseIndex(bookID string, chapter, verse int) int {
	if !IsOldTestament(bookID) {
		return -1
	}
	return CalculateVerseIndex(bookID, chapter, verse)
}

// CalculateNTVerseIndex computes the verse index within just the New Testament.
// Returns -1 if the book is not in the NT.
// NT books start at index 39 in KJVBooks.
func CalculateNTVerseIndex(bookID string, chapter, verse int) int {
	if !IsNewTestament(bookID) {
		return -1
	}
	return calculateIndexWithIntros(bookID, chapter, verse, 39)
}
