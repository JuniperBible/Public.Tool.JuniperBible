package sword

import (
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// ScriptureRange represents a parsed scripture reference that may span ranges.
// For single verse references, use ToReference() to convert to simple Reference type.
type ScriptureRange struct {
	Book         string `@Book`
	ChapterStart *int   `( @Number`
	VerseStart   *int   `( ":" @Number )?`
	ChapterEnd   *int   `( "-" ( @Number`
	VerseEnd     *int   `    ( ":" @Number )? )? )? )?`
}

// referenceLexer tokenizes scripture references.
var referenceLexer = lexer.MustSimple([]lexer.SimpleRule{
	// Book names: letters, numbers, optional trailing period
	// Examples: Genesis, Gen, Gen., 1John, 1 John, Song of Solomon
	{Name: "Book", Pattern: `(?:\d\s*)?[A-Za-z]+(?:\s+(?:of\s+)?[A-Za-z]+)*\.?`},
	// Numbers (chapter/verse)
	{Name: "Number", Pattern: `\d+`},
	// Separators
	{Name: "Colon", Pattern: `:`},
	{Name: "Dash", Pattern: `-`},
	// Whitespace
	{Name: "Whitespace", Pattern: `\s+`},
})

// referenceParser parses scripture references.
var referenceParser = participle.MustBuild[ScriptureRange](
	participle.Lexer(referenceLexer),
	participle.Elide("Whitespace"),
)

// ParseScriptureRange parses a scripture reference string into a range.
// Supported formats:
//   - "Genesis 1:1" (book chapter:verse)
//   - "Gen 1:1" (abbreviated book)
//   - "Gen.1.1" or "Gen 1.1" (dot separator)
//   - "Genesis 1:1-5" (verse range within chapter)
//   - "Genesis 1:1-2:5" (range across chapters)
//   - "Genesis 1-2" (chapter range)
//   - "Genesis 1" (full chapter)
//   - "Genesis" (full book)
func ParseScriptureRange(input string) (*ScriptureRange, error) {
	// Normalize input: convert dots to colons for "Gen.1.1" style
	normalized := normalizeSeparators(input)

	ref, err := referenceParser.ParseString("", normalized)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference %q: %w", input, err)
	}

	// Normalize book name
	ref.Book = NormalizeBookName(ref.Book)

	// Post-processing: fix verse range detection
	// If we have ChapterStart:VerseStart-ChapterEnd (no VerseEnd),
	// and VerseStart is set, then the number after dash is actually VerseEnd
	// Example: "Genesis 1:1-5" should be chapter 1, verse 1-5 (not chapter 1-5)
	if ref.VerseStart != nil && ref.ChapterEnd != nil && ref.VerseEnd == nil {
		// The number after dash is verse end, not chapter end
		ref.VerseEnd = ref.ChapterEnd
		ref.ChapterEnd = nil
	}

	return ref, nil
}

// ParseReference is an alias for ParseScriptureRange for backward compatibility.
func ParseReference(input string) (*ScriptureRange, error) {
	return ParseScriptureRange(input)
}

// normalizeSeparators converts dot separators to standard colon format.
// "Gen.1.1" -> "Gen 1:1"
// "Gen 1.1" -> "Gen 1:1"
func normalizeSeparators(input string) string {
	// Replace dots between numbers with colons
	result := input

	// Handle "Book.Chapter.Verse" format
	parts := strings.Split(result, ".")
	if len(parts) >= 2 {
		// Check if parts after first look like numbers
		book := parts[0]
		rest := parts[1:]

		// If rest contains numbers, rejoin with proper separators
		allNumbers := true
		for _, p := range rest {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			for _, c := range p {
				if c < '0' || c > '9' {
					allNumbers = false
					break
				}
			}
		}

		if allNumbers && len(rest) > 0 {
			// Reconstruct as "Book Chapter:Verse"
			if len(rest) == 1 {
				result = book + " " + rest[0]
			} else if len(rest) >= 2 {
				result = book + " " + rest[0] + ":" + strings.Join(rest[1:], ":")
			}
		}
	}

	return result
}

// String returns the canonical string representation of the reference.
func (r *ScriptureRange) String() string {
	if r.ChapterStart == nil {
		return r.Book
	}

	var sb strings.Builder
	sb.WriteString(r.Book)
	sb.WriteString(" ")
	sb.WriteString(fmt.Sprintf("%d", *r.ChapterStart))

	if r.VerseStart != nil {
		sb.WriteString(fmt.Sprintf(":%d", *r.VerseStart))
	}

	if r.ChapterEnd != nil {
		sb.WriteString("-")
		sb.WriteString(fmt.Sprintf("%d", *r.ChapterEnd))
		if r.VerseEnd != nil {
			sb.WriteString(fmt.Sprintf(":%d", *r.VerseEnd))
		}
	} else if r.VerseEnd != nil {
		sb.WriteString(fmt.Sprintf("-%d", *r.VerseEnd))
	}

	return sb.String()
}

// IsRange returns true if this reference spans multiple verses or chapters.
func (r *ScriptureRange) IsRange() bool {
	return r.ChapterEnd != nil || r.VerseEnd != nil
}

// IsChapterOnly returns true if this reference is for full chapter(s).
func (r *ScriptureRange) IsChapterOnly() bool {
	return r.ChapterStart != nil && r.VerseStart == nil
}

// IsBookOnly returns true if this reference is for the entire book.
func (r *ScriptureRange) IsBookOnly() bool {
	return r.ChapterStart == nil
}

// ToReference converts a ScriptureRange to a simple Reference (start point only).
// Useful for single verse lookups.
func (r *ScriptureRange) ToReference() Reference {
	ref := Reference{
		Book: r.Book,
	}
	if r.ChapterStart != nil {
		ref.Chapter = *r.ChapterStart
	}
	if r.VerseStart != nil {
		ref.Verse = *r.VerseStart
	}
	return ref
}

// NormalizeBookName converts various book name formats to a canonical form.
func NormalizeBookName(book string) string {
	// Remove trailing period
	book = strings.TrimSuffix(book, ".")
	book = strings.TrimSpace(book)

	// Normalize common abbreviations
	normalized := strings.ToLower(book)

	// Book name mappings (lowercase -> canonical)
	bookMap := map[string]string{
		// Genesis
		"gen": "Genesis", "genesis": "Genesis",
		// Exodus
		"exod": "Exodus", "exo": "Exodus", "exodus": "Exodus", "ex": "Exodus",
		// Leviticus
		"lev": "Leviticus", "leviticus": "Leviticus",
		// Numbers
		"num": "Numbers", "numbers": "Numbers",
		// Deuteronomy
		"deut": "Deuteronomy", "deu": "Deuteronomy", "deuteronomy": "Deuteronomy",
		// Joshua
		"josh": "Joshua", "jos": "Joshua", "joshua": "Joshua",
		// Judges
		"judg": "Judges", "jdg": "Judges", "judges": "Judges",
		// Ruth
		"ruth": "Ruth",
		// 1 Samuel
		"1sam": "1Samuel", "1 sam": "1Samuel", "1 samuel": "1Samuel", "1samuel": "1Samuel",
		// 2 Samuel
		"2sam": "2Samuel", "2 sam": "2Samuel", "2 samuel": "2Samuel", "2samuel": "2Samuel",
		// 1 Kings
		"1kgs": "1Kings", "1 kgs": "1Kings", "1 kings": "1Kings", "1kings": "1Kings",
		// 2 Kings
		"2kgs": "2Kings", "2 kgs": "2Kings", "2 kings": "2Kings", "2kings": "2Kings",
		// 1 Chronicles
		"1chr": "1Chronicles", "1 chr": "1Chronicles", "1 chronicles": "1Chronicles", "1chronicles": "1Chronicles",
		// 2 Chronicles
		"2chr": "2Chronicles", "2 chr": "2Chronicles", "2 chronicles": "2Chronicles", "2chronicles": "2Chronicles",
		// Ezra
		"ezra": "Ezra", "ezr": "Ezra",
		// Nehemiah
		"neh": "Nehemiah", "nehemiah": "Nehemiah",
		// Esther
		"esth": "Esther", "est": "Esther", "esther": "Esther",
		// Job
		"job": "Job",
		// Psalms
		"ps": "Psalms", "psa": "Psalms", "psalm": "Psalms", "psalms": "Psalms",
		// Proverbs
		"prov": "Proverbs", "pro": "Proverbs", "proverbs": "Proverbs",
		// Ecclesiastes
		"eccl": "Ecclesiastes", "ecc": "Ecclesiastes", "ecclesiastes": "Ecclesiastes",
		// Song of Solomon
		"song": "Song of Solomon", "song of solomon": "Song of Solomon",
		"song of songs": "Song of Solomon", "sos": "Song of Solomon", "canticles": "Song of Solomon",
		// Isaiah
		"isa": "Isaiah", "isaiah": "Isaiah",
		// Jeremiah
		"jer": "Jeremiah", "jeremiah": "Jeremiah",
		// Lamentations
		"lam": "Lamentations", "lamentations": "Lamentations",
		// Ezekiel
		"ezek": "Ezekiel", "eze": "Ezekiel", "ezekiel": "Ezekiel",
		// Daniel
		"dan": "Daniel", "daniel": "Daniel",
		// Hosea
		"hos": "Hosea", "hosea": "Hosea",
		// Joel
		"joel": "Joel",
		// Amos
		"amos": "Amos",
		// Obadiah
		"obad": "Obadiah", "oba": "Obadiah", "obadiah": "Obadiah",
		// Jonah
		"jonah": "Jonah", "jon": "Jonah",
		// Micah
		"mic": "Micah", "micah": "Micah",
		// Nahum
		"nah": "Nahum", "nahum": "Nahum",
		// Habakkuk
		"hab": "Habakkuk", "habakkuk": "Habakkuk",
		// Zephaniah
		"zeph": "Zephaniah", "zep": "Zephaniah", "zephaniah": "Zephaniah",
		// Haggai
		"hag": "Haggai", "haggai": "Haggai",
		// Zechariah
		"zech": "Zechariah", "zec": "Zechariah", "zechariah": "Zechariah",
		// Malachi
		"mal": "Malachi", "malachi": "Malachi",
		// Matthew
		"matt": "Matthew", "mat": "Matthew", "matthew": "Matthew", "mt": "Matthew",
		// Mark
		"mark": "Mark", "mrk": "Mark", "mk": "Mark",
		// Luke
		"luke": "Luke", "luk": "Luke", "lk": "Luke",
		// John
		"john": "John", "joh": "John", "jn": "John",
		// Acts
		"acts": "Acts", "act": "Acts",
		// Romans
		"rom": "Romans", "romans": "Romans",
		// 1 Corinthians
		"1cor": "1Corinthians", "1 cor": "1Corinthians", "1 corinthians": "1Corinthians", "1corinthians": "1Corinthians",
		// 2 Corinthians
		"2cor": "2Corinthians", "2 cor": "2Corinthians", "2 corinthians": "2Corinthians", "2corinthians": "2Corinthians",
		// Galatians
		"gal": "Galatians", "galatians": "Galatians",
		// Ephesians
		"eph": "Ephesians", "ephesians": "Ephesians",
		// Philippians
		"phil": "Philippians", "philippians": "Philippians",
		// Colossians
		"col": "Colossians", "colossians": "Colossians",
		// 1 Thessalonians
		"1thess": "1Thessalonians", "1 thess": "1Thessalonians", "1 thessalonians": "1Thessalonians", "1thessalonians": "1Thessalonians",
		// 2 Thessalonians
		"2thess": "2Thessalonians", "2 thess": "2Thessalonians", "2 thessalonians": "2Thessalonians", "2thessalonians": "2Thessalonians",
		// 1 Timothy
		"1tim": "1Timothy", "1 tim": "1Timothy", "1 timothy": "1Timothy", "1timothy": "1Timothy",
		// 2 Timothy
		"2tim": "2Timothy", "2 tim": "2Timothy", "2 timothy": "2Timothy", "2timothy": "2Timothy",
		// Titus
		"titus": "Titus", "tit": "Titus",
		// Philemon
		"phlm": "Philemon", "philemon": "Philemon", "phm": "Philemon",
		// Hebrews
		"heb": "Hebrews", "hebrews": "Hebrews",
		// James
		"jas": "James", "james": "James",
		// 1 Peter
		"1pet": "1Peter", "1 pet": "1Peter", "1 peter": "1Peter", "1peter": "1Peter",
		// 2 Peter
		"2pet": "2Peter", "2 pet": "2Peter", "2 peter": "2Peter", "2peter": "2Peter",
		// 1 John
		"1john": "1John", "1 john": "1John", "1jn": "1John", "1 jn": "1John",
		// 2 John
		"2john": "2John", "2 john": "2John", "2jn": "2John", "2 jn": "2John",
		// 3 John
		"3john": "3John", "3 john": "3John", "3jn": "3John", "3 jn": "3John",
		// Jude
		"jude": "Jude",
		// Revelation
		"rev": "Revelation", "revelation": "Revelation",
	}

	if canonical, ok := bookMap[normalized]; ok {
		return canonical
	}

	// Return original with proper capitalization
	return strings.Title(book)
}
