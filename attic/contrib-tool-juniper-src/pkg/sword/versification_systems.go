// Package sword provides versification system definitions for Bible modules.
//
// Different Bible traditions use different versification systems that affect:
// - Book order and inclusion (Protestant 66, Catholic 73, Orthodox 76+)
// - Chapter and verse numbering (especially Psalms)
// - Verse boundaries (some verses split or merged)
//
// Reference: https://wiki.crosswire.org/Alternate_Versification
package sword

// VersificationSystem represents a complete versification definition.
type VersificationSystem struct {
	// Name is the canonical name (e.g., "KJV", "Vulg", "LXX")
	Name string

	// Books contains book definitions in canonical order
	Books []VersificationBook

	// BookIndex maps book ID to index for fast lookup
	BookIndex map[string]int
}

// VersificationBook defines a book within a versification system.
type VersificationBook struct {
	// ID is the OSIS book identifier (e.g., "Gen", "Ps", "Matt")
	ID string

	// Name is the full book name
	Name string

	// Abbrev is the common abbreviation
	Abbrev string

	// Testament is "OT", "NT", or "AP" (Apocrypha/Deuterocanon)
	Testament string

	// ChapterVerseCounts contains verse count per chapter (1-indexed, [0] = chapter 1)
	ChapterVerseCounts []int
}

// VersificationMapping defines how to map references between two systems.
type VersificationMapping struct {
	// From is the source versification system name
	From string

	// To is the target versification system name
	To string

	// Rules contains the mapping rules
	Rules []MappingRule
}

// MappingRule defines how a reference maps between systems.
type MappingRule struct {
	// SourceBook is the OSIS book ID in the source system
	SourceBook string

	// SourceChapter is the chapter number in the source system
	SourceChapter int

	// SourceVerse is the verse number in the source system (0 = entire chapter)
	SourceVerse int

	// TargetBook is the OSIS book ID in the target system
	TargetBook string

	// TargetChapter is the chapter number in the target system
	TargetChapter int

	// TargetVerse is the verse number in the target system (0 = entire chapter)
	TargetVerse int

	// Type indicates the mapping type
	Type MappingType
}

// MappingType indicates how verses map between systems.
type MappingType int

const (
	// MapDirect indicates a 1:1 verse mapping
	MapDirect MappingType = iota

	// MapRenumber indicates the verse has a different number (same content)
	MapRenumber

	// MapSplit indicates one verse becomes multiple verses
	MapSplit

	// MapMerge indicates multiple verses become one verse
	MapMerge

	// MapMissing indicates the verse doesn't exist in the target
	MapMissing

	// MapAdded indicates the verse only exists in the target
	MapAdded
)

// Chapters returns the number of chapters in the book.
func (b *VersificationBook) Chapters() int {
	return len(b.ChapterVerseCounts)
}

// Verses returns the number of verses in a specific chapter (1-indexed).
func (b *VersificationBook) Verses(chapter int) int {
	if chapter < 1 || chapter > len(b.ChapterVerseCounts) {
		return 0
	}
	return b.ChapterVerseCounts[chapter-1]
}

// TotalVerses returns the total verse count for the book.
func (b *VersificationBook) TotalVerses() int {
	total := 0
	for _, v := range b.ChapterVerseCounts {
		total += v
	}
	return total
}

// GetBook returns the book info by ID.
func (v *VersificationSystem) GetBook(bookID string) (*VersificationBook, bool) {
	if idx, ok := v.BookIndex[bookID]; ok {
		return &v.Books[idx], true
	}
	return nil, false
}

// GetBookByIndex returns the book at the given index.
func (v *VersificationSystem) GetBookByIndex(index int) (*VersificationBook, bool) {
	if index < 0 || index >= len(v.Books) {
		return nil, false
	}
	return &v.Books[index], true
}

// TotalBooks returns the number of books in this versification.
func (v *VersificationSystem) TotalBooks() int {
	return len(v.Books)
}

// OTBooks returns only Old Testament books.
func (v *VersificationSystem) OTBooks() []VersificationBook {
	var books []VersificationBook
	for _, b := range v.Books {
		if b.Testament == "OT" {
			books = append(books, b)
		}
	}
	return books
}

// NTBooks returns only New Testament books.
func (v *VersificationSystem) NTBooks() []VersificationBook {
	var books []VersificationBook
	for _, b := range v.Books {
		if b.Testament == "NT" {
			books = append(books, b)
		}
	}
	return books
}

// APBooks returns only Apocrypha/Deuterocanonical books.
func (v *VersificationSystem) APBooks() []VersificationBook {
	var books []VersificationBook
	for _, b := range v.Books {
		if b.Testament == "AP" {
			books = append(books, b)
		}
	}
	return books
}

// buildIndex populates the BookIndex map from Books slice.
func (v *VersificationSystem) buildIndex() {
	v.BookIndex = make(map[string]int)
	for i, book := range v.Books {
		v.BookIndex[book.ID] = i
	}
}

// CalculateVerseIndexForSystem computes the verse index for a reference in this system.
// This accounts for intro entries (testament header, book intros, chapter intros).
//
// SWORD index format (per testament file - ot.bzv or nt.bzv):
// - Entry 0-1: Testament heading (2 entries)
// - For each book: 1 book intro + (for each chapter: 1 chapter intro + verses)
//
// For Vulgate versification, OT includes deuterocanonical books interspersed,
// so we count all OT/AP books before the target book.
func (v *VersificationSystem) CalculateVerseIndexForSystem(bookID string, chapter, verse int) int {
	book, ok := v.GetBook(bookID)
	if !ok {
		return -1
	}

	bookIdx := v.BookIndex[bookID]

	// Determine which testament file this book is in
	// NT books are in nt.bzv, everything else (OT + AP) is in ot.bzv
	isNT := book.Testament == "NT"

	// Find the starting index for this testament's books
	startIdx := 0
	if isNT {
		// For NT, find the first NT book in the array
		for i, b := range v.Books {
			if b.Testament == "NT" {
				startIdx = i
				break
			}
		}
	}

	// Start after testament heading (2 entries: placeholder + header)
	index := 2

	// Add sizes of all previous books in this testament file
	// Book size = total verses + number of chapters (chapter intros) + 1 (book intro)
	for i := startIdx; i < bookIdx; i++ {
		prevBook := v.Books[i]
		// For OT file: include both OT and AP books (they're interspersed in Vulgate)
		// For NT file: only include NT books
		if isNT {
			if prevBook.Testament != "NT" {
				continue
			}
		} else {
			// OT file contains OT and AP books
			if prevBook.Testament == "NT" {
				continue
			}
		}
		totalVerses := prevBook.TotalVerses()
		numChapters := prevBook.Chapters()
		index += totalVerses + numChapters + 1
	}

	// Add book intro for current book
	index += 1

	// Add chapter intros and verses from previous chapters
	for ch := 1; ch < chapter; ch++ {
		if ch <= book.Chapters() {
			index += 1 // Chapter intro
			index += book.Verses(ch)
		}
	}

	// Add current chapter's intro
	index += 1

	// Add verse position (verse is 1-based, becomes 0-based offset)
	index += verse - 1

	return index
}

// versificationSystems is a registry of all supported versification systems.
var versificationSystems = make(map[string]*VersificationSystem)

// RegisterVersification adds a versification system to the registry.
func RegisterVersification(system *VersificationSystem) {
	system.buildIndex()
	versificationSystems[system.Name] = system
}

// GetVersification retrieves a versification system by name.
// Returns nil if not found.
func GetVersification(name string) *VersificationSystem {
	return versificationSystems[name]
}

// ListVersifications returns all registered versification system names.
func ListVersifications() []string {
	names := make([]string, 0, len(versificationSystems))
	for name := range versificationSystems {
		names = append(names, name)
	}
	return names
}

// NormalizeVersificationName converts common aliases to canonical names.
func NormalizeVersificationName(name string) string {
	switch name {
	case "KJV", "kjv", "King James", "Protestant":
		return "KJV"
	case "KJVA", "kjva", "KJV with Apocrypha":
		return "KJVA"
	case "Vulg", "vulg", "Vulgate", "vulgate", "Latin Vulgate":
		return "Vulg"
	case "LXX", "lxx", "Septuagint":
		return "LXX"
	case "Catholic", "catholic":
		return "Catholic"
	case "Catholic2", "catholic2":
		return "Catholic2"
	case "NRSV", "nrsv":
		return "NRSV"
	case "NRSVA", "nrsva":
		return "NRSVA"
	case "MT", "mt", "Masoretic", "Hebrew":
		return "MT"
	case "Leningrad", "leningrad":
		return "Leningrad"
	case "Synodal", "synodal", "Russian":
		return "Synodal"
	case "SynodalProt", "synodalProt":
		return "SynodalProt"
	case "Luther", "luther", "German":
		return "Luther"
	case "Orthodox", "orthodox":
		return "Orthodox"
	default:
		return name
	}
}
