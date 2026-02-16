package swordpure

import (
	"testing"
)

func TestRefString(t *testing.T) {
	tests := []struct {
		name string
		ref  Ref
		want string
	}{
		{
			name: "simple reference",
			ref:  Ref{Book: "Gen", Chapter: 1, Verse: 1},
			want: "Gen.1.1",
		},
		{
			name: "verse range",
			ref:  Ref{Book: "Matt", Chapter: 5, Verse: 3, VerseEnd: 12},
			want: "Matt.5.3-12",
		},
		{
			name: "chapter range",
			ref:  Ref{Book: "John", Chapter: 3, Verse: 16, ChapterEnd: 4, VerseEnd: 5},
			want: "John.3.16-4.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.String(); got != tt.want {
				t.Errorf("Ref.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRef(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Ref
		wantErr bool
	}{
		// OSIS format
		{
			name:  "OSIS simple",
			input: "Gen.1.1",
			want:  &Ref{Book: "Gen", Chapter: 1, Verse: 1},
		},
		{
			name:  "OSIS verse range",
			input: "Matt.5.3-12",
			want:  &Ref{Book: "Matt", Chapter: 5, Verse: 3, VerseEnd: 12},
		},
		{
			name:  "OSIS chapter range",
			input: "John.3.16-4.5",
			want:  &Ref{Book: "John", Chapter: 3, Verse: 16, ChapterEnd: 4, VerseEnd: 5},
		},
		// Human-readable format
		{
			name:  "human simple",
			input: "Gen 1:1",
			want:  &Ref{Book: "Gen", Chapter: 1, Verse: 1},
		},
		{
			name:  "human full name",
			input: "Genesis 1:1",
			want:  &Ref{Book: "Gen", Chapter: 1, Verse: 1},
		},
		{
			name:  "human verse range",
			input: "Matt 5:3-12",
			want:  &Ref{Book: "Matt", Chapter: 5, Verse: 3, VerseEnd: 12},
		},
		{
			name:  "numbered book",
			input: "1 John 3:16",
			want:  &Ref{Book: "1John", Chapter: 3, Verse: 16},
		},
		{
			name:  "period separator",
			input: "Matt.5.3",
			want:  &Ref{Book: "Matt", Chapter: 5, Verse: 3},
		},
		// Error cases
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "unknown book",
			input:   "FakeBook 1:1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRef(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Book != tt.want.Book {
				t.Errorf("Book = %q, want %q", got.Book, tt.want.Book)
			}
			if got.Chapter != tt.want.Chapter {
				t.Errorf("Chapter = %d, want %d", got.Chapter, tt.want.Chapter)
			}
			if got.Verse != tt.want.Verse {
				t.Errorf("Verse = %d, want %d", got.Verse, tt.want.Verse)
			}
			if got.VerseEnd != tt.want.VerseEnd {
				t.Errorf("VerseEnd = %d, want %d", got.VerseEnd, tt.want.VerseEnd)
			}
			if got.ChapterEnd != tt.want.ChapterEnd {
				t.Errorf("ChapterEnd = %d, want %d", got.ChapterEnd, tt.want.ChapterEnd)
			}
		})
	}
}

func TestBookIndex(t *testing.T) {
	tests := []struct {
		book string
		want int
	}{
		{"Gen", 0},
		{"Exod", 1},
		{"Mal", 38},
		{"Matt", 39},
		{"Rev", 65},
		{"Unknown", -1},
		{"", -1},
	}

	for _, tt := range tests {
		t.Run(tt.book, func(t *testing.T) {
			if got := BookIndex(tt.book); got != tt.want {
				t.Errorf("BookIndex(%q) = %d, want %d", tt.book, got, tt.want)
			}
		})
	}
}

func TestBookAbbreviations(t *testing.T) {
	// Test that various abbreviations map to correct OSIS IDs
	tests := []struct {
		input    string
		wantOSIS string
	}{
		// Old Testament
		{"gen", "Gen"},
		{"genesis", "Gen"},
		{"exod", "Exod"},
		{"exodus", "Exod"},
		{"ex", "Exod"},
		{"ps", "Ps"},
		{"psalm", "Ps"},
		{"psalms", "Ps"},
		{"pss", "Ps"},
		{"song", "Song"},
		{"sos", "Song"},
		{"canticles", "Song"},
		// New Testament
		{"matt", "Matt"},
		{"matthew", "Matt"},
		{"mt", "Matt"},
		{"mark", "Mark"},
		{"mk", "Mark"},
		{"john", "John"},
		{"jn", "John"},
		{"acts", "Acts"},
		{"rom", "Rom"},
		{"romans", "Rom"},
		{"1cor", "1Cor"},
		{"1 cor", "1Cor"},
		{"1 corinthians", "1Cor"},
		{"rev", "Rev"},
		{"revelation", "Rev"},
		{"apocalypse", "Rev"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := BookAbbreviations[tt.input]; got != tt.wantOSIS {
				t.Errorf("BookAbbreviations[%q] = %q, want %q", tt.input, got, tt.wantOSIS)
			}
		})
	}
}

func TestOSISBookOrder(t *testing.T) {
	// Verify book order
	if len(OSISBookOrder) != 66 {
		t.Errorf("OSISBookOrder has %d books, want 66", len(OSISBookOrder))
	}

	// First book should be Genesis
	if OSISBookOrder[0] != "Gen" {
		t.Errorf("First book = %q, want Gen", OSISBookOrder[0])
	}

	// Last book should be Revelation
	if OSISBookOrder[65] != "Rev" {
		t.Errorf("Last book = %q, want Rev", OSISBookOrder[65])
	}

	// Matthew should be at index 39 (first NT book)
	if OSISBookOrder[39] != "Matt" {
		t.Errorf("Book at index 39 = %q, want Matt", OSISBookOrder[39])
	}
}

func TestParseRefEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"whitespace", "  Gen 1:1  "},
		{"mixed case", "GENESIS 1:1"},
		{"lowercase", "genesis 1:1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseRef(tt.input)
			if err != nil {
				t.Errorf("ParseRef(%q) failed: %v", tt.input, err)
				return
			}
			if ref.Book != "Gen" || ref.Chapter != 1 || ref.Verse != 1 {
				t.Errorf("ParseRef(%q) = %+v, want Gen.1.1", tt.input, ref)
			}
		})
	}
}

func TestParseRefHumanChapterRange(t *testing.T) {
	// Test human-readable format with chapter ranges
	ref, err := ParseRef("Matt 5:3-6:5")
	if err != nil {
		t.Fatalf("ParseRef failed: %v", err)
	}

	if ref.Book != "Matt" {
		t.Errorf("Book = %q, want %q", ref.Book, "Matt")
	}
	if ref.Chapter != 5 {
		t.Errorf("Chapter = %d, want %d", ref.Chapter, 5)
	}
	if ref.Verse != 3 {
		t.Errorf("Verse = %d, want %d", ref.Verse, 3)
	}
	if ref.ChapterEnd != 6 {
		t.Errorf("ChapterEnd = %d, want %d", ref.ChapterEnd, 6)
	}
	if ref.VerseEnd != 5 {
		t.Errorf("VerseEnd = %d, want %d", ref.VerseEnd, 5)
	}
}

func TestParseRefMoreBookAbbreviations(t *testing.T) {
	// Test additional abbreviations to increase coverage
	tests := []struct {
		input    string
		wantBook string
	}{
		{"Dt 1:1", "Deut"},
		{"1 Samuel 1:1", "1Sam"},
		{"2 Kings 1:1", "2Kgs"},
		{"1 Chronicles 1:1", "1Chr"},
		{"2 Chronicles 1:1", "2Chr"},
		{"Lk 1:1", "Luke"},
		{"Jn 1:1", "John"},
		{"2 Thessalonians 1:1", "2Thess"},
		{"1 Timothy 1:1", "1Tim"},
		{"2 Timothy 1:1", "2Tim"},
		{"Philemon 1:1", "Phlm"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ref, err := ParseRef(tt.input)
			if err != nil {
				t.Errorf("ParseRef(%q) failed: %v", tt.input, err)
				return
			}
			if ref.Book != tt.wantBook {
				t.Errorf("ParseRef(%q).Book = %q, want %q", tt.input, ref.Book, tt.wantBook)
			}
		})
	}
}

func TestNormalizeBookName(t *testing.T) {
	// Test that OSIS book names are returned unchanged
	for _, osisID := range OSISBookOrder {
		result := normalizeBookName(osisID)
		if result != osisID {
			t.Errorf("normalizeBookName(%q) = %q, want %q", osisID, result, osisID)
		}
	}
}
