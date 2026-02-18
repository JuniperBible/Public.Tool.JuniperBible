package ir

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// Ref represents a canonical scripture reference.
type Ref struct {
	// Book is the OSIS book ID (e.g., "Gen", "Matt", "1John").
	Book string `json:"book"`

	// Chapter is the chapter number (1-indexed, 0 for whole-book references).
	Chapter int `json:"chapter,omitempty"`

	// Verse is the verse number (1-indexed, 0 for whole-chapter references).
	Verse int `json:"verse,omitempty"`

	// VerseEnd is the ending verse for ranges (optional).
	VerseEnd int `json:"verse_end,omitempty"`

	// SubVerse is the verse subdivision (e.g., "a", "b").
	SubVerse string `json:"sub_verse,omitempty"`

	// OSISID is the full OSIS ID string (e.g., "Gen.1.1", "Matt.5.3-12").
	OSISID string `json:"osis_id,omitempty"`
}

// refGrammar is the participle grammar for OSIS-style references.
// Examples: "Gen", "Gen.1", "Gen.1.1", "Gen.1.1a", "Gen.1.1-3", "1John.3.16"
//
//nolint:govet // participle grammar tags are not standard struct tags
type refGrammar struct {
	BookPrefix string       `@Int?`
	BookName   string       `@Ident`
	ChapterRef *chapterPart `( "." @@ )?`
}

//nolint:govet // participle grammar tags are not standard struct tags
type chapterPart struct {
	Chapter  int        `@Int`
	VerseRef *versePart `( "." @@ )?`
}

//nolint:govet // participle grammar tags are not standard struct tags
type versePart struct {
	Verse    int     `@Int`
	SubVerse *string `@SubVerse?`
	Range    *int    `( "-" @Int )?`
}

// refLexer defines the lexer for OSIS references.
// Note: Ident starts with uppercase to distinguish from SubVerse (single lowercase)
var refLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Int", Pattern: `[0-9]+`},
	{Name: "Ident", Pattern: `[A-Z][A-Za-z]*`}, // Book names start with uppercase
	{Name: "SubVerse", Pattern: `[a-z]`},       // Single lowercase letter for sub-verse
	{Name: "Punct", Pattern: `[.\-]`},
	{Name: "Whitespace", Pattern: `\s+`},
})

// refParser is the participle parser for OSIS references.
var refParser = participle.MustBuild[refGrammar](
	participle.Lexer(refLexer),
	participle.Elide("Whitespace"),
)

// ParseRef parses an OSIS-style reference string.
// Supported formats:
//   - "Gen" (book only)
//   - "Gen.1" (book and chapter)
//   - "Gen.1.1" (book, chapter, and verse)
//   - "Gen.1.1a" (with sub-verse)
//   - "Gen.1.1-3" (verse range)
//   - "Matt.5.3-12" (verse range)
func ParseRef(s string) (*Ref, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty reference string")
	}

	parsed, err := refParser.ParseString("", s)
	if err != nil {
		return nil, fmt.Errorf("invalid reference format: %q: %w", s, err)
	}

	// Build the Ref from parsed grammar
	ref := &Ref{
		Book:   parsed.BookPrefix + parsed.BookName,
		OSISID: s,
	}

	if parsed.ChapterRef != nil {
		ref.Chapter = parsed.ChapterRef.Chapter

		if parsed.ChapterRef.VerseRef != nil {
			ref.Verse = parsed.ChapterRef.VerseRef.Verse

			if parsed.ChapterRef.VerseRef.SubVerse != nil {
				ref.SubVerse = *parsed.ChapterRef.VerseRef.SubVerse
			}

			if parsed.ChapterRef.VerseRef.Range != nil {
				ref.VerseEnd = *parsed.ChapterRef.VerseRef.Range
			}
		}
	}

	return ref, nil
}

// String returns the OSIS ID representation of the reference.
func (r *Ref) String() string {
	if r.OSISID != "" {
		return r.OSISID
	}

	var sb strings.Builder
	sb.WriteString(r.Book)

	if r.Chapter > 0 {
		sb.WriteString(".")
		sb.WriteString(strconv.Itoa(r.Chapter))

		if r.Verse > 0 {
			sb.WriteString(".")
			sb.WriteString(strconv.Itoa(r.Verse))
			sb.WriteString(r.SubVerse)

			if r.VerseEnd > 0 {
				sb.WriteString("-")
				sb.WriteString(strconv.Itoa(r.VerseEnd))
			}
		}
	}

	return sb.String()
}

// IsRange returns true if this reference spans multiple verses.
func (r *Ref) IsRange() bool {
	return r.VerseEnd > 0 && r.VerseEnd > r.Verse
}

// Contains returns true if this reference contains the other reference.
func (r *Ref) Contains(other *Ref) bool {
	if r.Book != other.Book {
		return false
	}

	// Book-only reference contains all chapters
	if r.Chapter == 0 {
		return true
	}

	// Different chapters
	if r.Chapter != other.Chapter {
		return false
	}

	// Chapter-only reference contains all verses in that chapter
	if r.Verse == 0 {
		return true
	}

	// Check verse range
	otherVerse := other.Verse
	if r.IsRange() {
		return otherVerse >= r.Verse && otherVerse <= r.VerseEnd
	}

	return r.Verse == otherVerse
}

// RefSet represents a non-contiguous set of references (for cross-references).
type RefSet struct {
	// ID is the unique identifier for this reference set.
	ID string `json:"id"`

	// Refs contains all references in this set.
	Refs []*Ref `json:"refs"`

	// Label is an optional human-readable label.
	Label string `json:"label,omitempty"`
}

// Add adds a reference to the set.
func (rs *RefSet) Add(ref *Ref) {
	rs.Refs = append(rs.Refs, ref)
}

// RefRange represents a contiguous range of references.
type RefRange struct {
	// Start is the beginning of the range.
	Start *Ref `json:"start"`

	// End is the end of the range.
	End *Ref `json:"end"`
}

// Contains returns true if the reference is within this range.
func (rr *RefRange) Contains(ref *Ref) bool {
	if !rr.bookMatches(ref) {
		return false
	}
	if !rr.chapterInRange(ref) {
		return false
	}
	return rr.verseInRange(ref)
}

// bookMatches returns true if ref's book matches both start and end.
func (rr *RefRange) bookMatches(ref *Ref) bool {
	return rr.Start.Book == ref.Book && rr.End.Book == ref.Book
}

// chapterInRange returns true if ref's chapter is within range.
func (rr *RefRange) chapterInRange(ref *Ref) bool {
	return ref.Chapter >= rr.Start.Chapter && ref.Chapter <= rr.End.Chapter
}

// verseInRange returns true if ref's verse is within range for its chapter.
func (rr *RefRange) verseInRange(ref *Ref) bool {
	if ref.Chapter == rr.Start.Chapter && ref.Verse < rr.Start.Verse {
		return false
	}
	if ref.Chapter == rr.End.Chapter && ref.Verse > rr.End.Verse {
		return false
	}
	return true
}
