// reference.go implements Bible reference parsing.
// Supports formats like "Gen 1:1", "Genesis 1:1-5", "Matt.5.3-12"
package swordpure

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Ref represents a Bible reference.
type Ref struct {
	Book       string `json:"book"`
	Chapter    int    `json:"chapter"`
	Verse      int    `json:"verse"`
	VerseEnd   int    `json:"verse_end,omitempty"`   // For ranges like 1:1-5
	ChapterEnd int    `json:"chapter_end,omitempty"` // For ranges like 1:1-2:5
}

// String returns the OSIS-style reference string.
func (r *Ref) String() string {
	if r.VerseEnd > 0 {
		if r.ChapterEnd > 0 {
			return fmt.Sprintf("%s.%d.%d-%d.%d", r.Book, r.Chapter, r.Verse, r.ChapterEnd, r.VerseEnd)
		}
		return fmt.Sprintf("%s.%d.%d-%d", r.Book, r.Chapter, r.Verse, r.VerseEnd)
	}
	return fmt.Sprintf("%s.%d.%d", r.Book, r.Chapter, r.Verse)
}

// BookAbbreviations maps common book names/abbreviations to OSIS book IDs.
var BookAbbreviations = map[string]string{
	// Old Testament
	"gen": "Gen", "genesis": "Gen",
	"exod": "Exod", "exodus": "Exod", "ex": "Exod",
	"lev": "Lev", "leviticus": "Lev",
	"num": "Num", "numbers": "Num",
	"deut": "Deut", "deuteronomy": "Deut", "dt": "Deut",
	"josh": "Josh", "joshua": "Josh",
	"judg": "Judg", "judges": "Judg",
	"ruth": "Ruth",
	"1sam": "1Sam", "1 sam": "1Sam", "1 samuel": "1Sam", "1samuel": "1Sam",
	"2sam": "2Sam", "2 sam": "2Sam", "2 samuel": "2Sam", "2samuel": "2Sam",
	"1kgs": "1Kgs", "1 kgs": "1Kgs", "1 kings": "1Kgs", "1kings": "1Kgs",
	"2kgs": "2Kgs", "2 kgs": "2Kgs", "2 kings": "2Kgs", "2kings": "2Kgs",
	"1chr": "1Chr", "1 chr": "1Chr", "1 chronicles": "1Chr", "1chronicles": "1Chr",
	"2chr": "2Chr", "2 chr": "2Chr", "2 chronicles": "2Chr", "2chronicles": "2Chr",
	"ezra": "Ezra",
	"neh":  "Neh", "nehemiah": "Neh",
	"esth": "Esth", "esther": "Esth",
	"job": "Job",
	"ps":  "Ps", "pss": "Ps", "psalm": "Ps", "psalms": "Ps",
	"prov": "Prov", "proverbs": "Prov",
	"eccl": "Eccl", "ecclesiastes": "Eccl", "qoh": "Eccl",
	"song": "Song", "sos": "Song", "song of solomon": "Song", "canticles": "Song",
	"isa": "Isa", "isaiah": "Isa",
	"jer": "Jer", "jeremiah": "Jer",
	"lam": "Lam", "lamentations": "Lam",
	"ezek": "Ezek", "ezekiel": "Ezek",
	"dan": "Dan", "daniel": "Dan",
	"hos": "Hos", "hosea": "Hos",
	"joel": "Joel",
	"amos": "Amos",
	"obad": "Obad", "obadiah": "Obad",
	"jonah": "Jonah",
	"mic":   "Mic", "micah": "Mic",
	"nah": "Nah", "nahum": "Nah",
	"hab": "Hab", "habakkuk": "Hab",
	"zeph": "Zeph", "zephaniah": "Zeph",
	"hag": "Hag", "haggai": "Hag",
	"zech": "Zech", "zechariah": "Zech",
	"mal": "Mal", "malachi": "Mal",

	// New Testament
	"matt": "Matt", "matthew": "Matt", "mt": "Matt",
	"mark": "Mark", "mk": "Mark",
	"luke": "Luke", "lk": "Luke",
	"john": "John", "jn": "John",
	"acts": "Acts",
	"rom":  "Rom", "romans": "Rom",
	"1cor": "1Cor", "1 cor": "1Cor", "1 corinthians": "1Cor", "1corinthians": "1Cor",
	"2cor": "2Cor", "2 cor": "2Cor", "2 corinthians": "2Cor", "2corinthians": "2Cor",
	"gal": "Gal", "galatians": "Gal",
	"eph": "Eph", "ephesians": "Eph",
	"phil": "Phil", "philippians": "Phil",
	"col": "Col", "colossians": "Col",
	"1thess": "1Thess", "1 thess": "1Thess", "1 thessalonians": "1Thess",
	"2thess": "2Thess", "2 thess": "2Thess", "2 thessalonians": "2Thess",
	"1tim": "1Tim", "1 tim": "1Tim", "1 timothy": "1Tim",
	"2tim": "2Tim", "2 tim": "2Tim", "2 timothy": "2Tim",
	"titus": "Titus",
	"phlm":  "Phlm", "philemon": "Phlm",
	"heb": "Heb", "hebrews": "Heb",
	"jas": "Jas", "james": "Jas",
	"1pet": "1Pet", "1 pet": "1Pet", "1 peter": "1Pet",
	"2pet": "2Pet", "2 pet": "2Pet", "2 peter": "2Pet",
	"1john": "1John", "1 john": "1John", "1jn": "1John",
	"2john": "2John", "2 john": "2John", "2jn": "2John",
	"3john": "3John", "3 john": "3John", "3jn": "3John",
	"jude": "Jude",
	"rev":  "Rev", "revelation": "Rev", "apocalypse": "Rev",
}

// Regular expressions for parsing references
var (
	// Matches: "Gen 1:1", "Genesis 1:1-5", "Matt.5.3-12", "1 John 3:16"
	refPattern = regexp.MustCompile(`(?i)^(\d?\s*[A-Za-z]+)\s*[.\s]+(\d+)[.:]\s*(\d+)(?:\s*[-–]\s*(?:(\d+)[.:]\s*)?(\d+))?$`)

	// Matches OSIS-style: "Gen.1.1", "Matt.5.3-12"
	osisPattern = regexp.MustCompile(`^([A-Za-z0-9]+)\.(\d+)\.(\d+)(?:-(?:(\d+)\.)?(\d+))?$`)
)

// ParseRef parses a Bible reference string into a Ref.
func ParseRef(s string) (*Ref, error) {
	s = strings.TrimSpace(s)

	// Try OSIS pattern first
	if matches := osisPattern.FindStringSubmatch(s); matches != nil {
		return parseOSISMatches(matches)
	}

	// Try human-readable pattern
	if matches := refPattern.FindStringSubmatch(s); matches != nil {
		return parseHumanMatches(matches)
	}

	return nil, fmt.Errorf("unable to parse reference: %s", s)
}

func parseOSISMatches(matches []string) (*Ref, error) {
	ref := &Ref{
		Book: matches[1],
	}

	var err error
	ref.Chapter, err = strconv.Atoi(matches[2])
	if err != nil {
		return nil, err
	}

	ref.Verse, err = strconv.Atoi(matches[3])
	if err != nil {
		return nil, err
	}

	if matches[4] != "" {
		ref.ChapterEnd, err = strconv.Atoi(matches[4])
		if err != nil {
			return nil, err
		}
	}

	if matches[5] != "" {
		ref.VerseEnd, err = strconv.Atoi(matches[5])
		if err != nil {
			return nil, err
		}
	}

	return ref, nil
}

func parseHumanMatches(matches []string) (*Ref, error) {
	bookStr := strings.TrimSpace(matches[1])
	book := normalizeBookName(bookStr)
	if book == "" {
		return nil, fmt.Errorf("unknown book: %s", bookStr)
	}

	ref := &Ref{
		Book: book,
	}

	var err error
	ref.Chapter, err = strconv.Atoi(matches[2])
	if err != nil {
		return nil, err
	}

	ref.Verse, err = strconv.Atoi(matches[3])
	if err != nil {
		return nil, err
	}

	if matches[4] != "" {
		ref.ChapterEnd, err = strconv.Atoi(matches[4])
		if err != nil {
			return nil, err
		}
	}

	if matches[5] != "" {
		ref.VerseEnd, err = strconv.Atoi(matches[5])
		if err != nil {
			return nil, err
		}
	}

	return ref, nil
}

// normalizeBookName converts a book name or abbreviation to OSIS book ID.
func normalizeBookName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))

	// Direct lookup
	if osisID, ok := BookAbbreviations[lower]; ok {
		return osisID
	}

	// Check if already an OSIS ID
	for _, osisID := range BookAbbreviations {
		if strings.EqualFold(name, osisID) {
			return osisID
		}
	}

	return ""
}

// OSISBookOrder returns the canonical order of OSIS books.
var OSISBookOrder = []string{
	// Old Testament
	"Gen", "Exod", "Lev", "Num", "Deut",
	"Josh", "Judg", "Ruth", "1Sam", "2Sam",
	"1Kgs", "2Kgs", "1Chr", "2Chr", "Ezra",
	"Neh", "Esth", "Job", "Ps", "Prov",
	"Eccl", "Song", "Isa", "Jer", "Lam",
	"Ezek", "Dan", "Hos", "Joel", "Amos",
	"Obad", "Jonah", "Mic", "Nah", "Hab",
	"Zeph", "Hag", "Zech", "Mal",
	// New Testament
	"Matt", "Mark", "Luke", "John", "Acts",
	"Rom", "1Cor", "2Cor", "Gal", "Eph",
	"Phil", "Col", "1Thess", "2Thess", "1Tim",
	"2Tim", "Titus", "Phlm", "Heb", "Jas",
	"1Pet", "2Pet", "1John", "2John", "3John",
	"Jude", "Rev",
}

// BookIndex returns the index of a book in canonical order (-1 if not found).
func BookIndex(book string) int {
	for i, b := range OSISBookOrder {
		if b == book {
			return i
		}
	}
	return -1
}
