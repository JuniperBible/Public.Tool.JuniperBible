package swordpure

import (
	"fmt"
	"testing"
)

func TestNewVersification(t *testing.T) {
	tests := []struct {
		id   VersificationID
		want VersificationID
	}{
		{VersKJV, VersKJV},
		{"", VersKJV},         // Empty defaults to KJV
		{VersNRSV, VersNRSV},
		{VersVulgate, VersVulgate},
		{"Unknown", VersKJV}, // Unknown defaults to KJV
	}

	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			v, err := NewVersification(tt.id)
			if err != nil {
				t.Fatalf("NewVersification failed: %v", err)
			}
			if v.ID != tt.want {
				t.Errorf("ID = %q, want %q", v.ID, tt.want)
			}
		})
	}
}

func TestVersificationFromConf(t *testing.T) {
	tests := []struct {
		name          string
		versification string
		wantID        VersificationID
	}{
		{"KJV explicit", "KJV", VersKJV},
		{"NRSV", "NRSV", VersNRSV},
		{"empty defaults to KJV", "", VersKJV},
		{"whitespace defaults to KJV", "  ", VersKJV},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := &ConfFile{Versification: tt.versification}
			v, err := VersificationFromConf(conf)
			if err != nil {
				t.Fatalf("VersificationFromConf failed: %v", err)
			}
			if v.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", v.ID, tt.wantID)
			}
		})
	}
}

func TestVersificationGetBookIndex(t *testing.T) {
	v, _ := NewVersification(VersKJV)

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
			if got := v.GetBookIndex(tt.book); got != tt.want {
				t.Errorf("GetBookIndex(%q) = %d, want %d", tt.book, got, tt.want)
			}
		})
	}
}

func TestVersificationGetChapterCount(t *testing.T) {
	v, _ := NewVersification(VersKJV)

	tests := []struct {
		book string
		want int
	}{
		{"Gen", 50},
		{"Ps", 150},
		{"Matt", 28},
		{"Rev", 22},
		{"Obad", 1},     // Single chapter book
		{"Unknown", 0},  // Invalid book
	}

	for _, tt := range tests {
		t.Run(tt.book, func(t *testing.T) {
			if got := v.GetChapterCount(tt.book); got != tt.want {
				t.Errorf("GetChapterCount(%q) = %d, want %d", tt.book, got, tt.want)
			}
		})
	}
}

func TestVersificationGetVerseCount(t *testing.T) {
	v, _ := NewVersification(VersKJV)

	tests := []struct {
		book    string
		chapter int
		want    int
	}{
		{"Gen", 1, 31},
		{"Gen", 50, 26},
		{"Ps", 119, 176},  // Longest chapter
		{"Ps", 117, 2},    // Shortest chapter
		{"Matt", 1, 25},
		{"Unknown", 1, 0}, // Invalid book
		{"Gen", 0, 0},     // Invalid chapter (too low)
		{"Gen", 100, 0},   // Invalid chapter (too high)
	}

	for _, tt := range tests {
		t.Run(tt.book, func(t *testing.T) {
			if got := v.GetVerseCount(tt.book, tt.chapter); got != tt.want {
				t.Errorf("GetVerseCount(%q, %d) = %d, want %d", tt.book, tt.chapter, got, tt.want)
			}
		})
	}
}

func TestVersificationGetTotalVerses(t *testing.T) {
	v, _ := NewVersification(VersKJV)

	tests := []struct {
		book string
		want int
	}{
		{"Gen", 1533},
		{"Ps", 2461},
		{"Obad", 21},      // Single chapter
		{"Unknown", 0},    // Invalid book
	}

	for _, tt := range tests {
		t.Run(tt.book, func(t *testing.T) {
			if got := v.GetTotalVerses(tt.book); got != tt.want {
				t.Errorf("GetTotalVerses(%q) = %d, want %d", tt.book, got, tt.want)
			}
		})
	}
}

func TestVersificationCalculateIndex(t *testing.T) {
	v, _ := NewVersification(VersKJV)

	// Test OT references
	tests := []struct {
		name    string
		ref     *Ref
		isNT    bool
		wantErr bool
	}{
		{
			name: "Gen 1:1 OT",
			ref:  &Ref{Book: "Gen", Chapter: 1, Verse: 1},
			isNT: false,
		},
		{
			name: "Gen 1:31 OT",
			ref:  &Ref{Book: "Gen", Chapter: 1, Verse: 31},
			isNT: false,
		},
		{
			name: "Matt 1:1 NT",
			ref:  &Ref{Book: "Matt", Chapter: 1, Verse: 1},
			isNT: true,
		},
		{
			name:    "Unknown book",
			ref:     &Ref{Book: "Unknown", Chapter: 1, Verse: 1},
			isNT:    false,
			wantErr: true,
		},
		{
			name:    "OT book in NT",
			ref:     &Ref{Book: "Gen", Chapter: 1, Verse: 1},
			isNT:    true,
			wantErr: true,
		},
		{
			name:    "Invalid chapter",
			ref:     &Ref{Book: "Gen", Chapter: 100, Verse: 1},
			isNT:    false,
			wantErr: true,
		},
		{
			name:    "Invalid verse",
			ref:     &Ref{Book: "Gen", Chapter: 1, Verse: 100},
			isNT:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, err := v.CalculateIndex(tt.ref, tt.isNT)
			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateIndex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && idx < 0 {
				t.Errorf("CalculateIndex() returned negative index: %d", idx)
			}
		})
	}
}

func TestVersificationIndexToRef(t *testing.T) {
	v, _ := NewVersification(VersKJV)

	// Test roundtrip: ref -> index -> ref
	tests := []struct {
		name string
		ref  *Ref
		isNT bool
	}{
		{"Gen 1:1", &Ref{Book: "Gen", Chapter: 1, Verse: 1}, false},
		{"Gen 1:31", &Ref{Book: "Gen", Chapter: 1, Verse: 31}, false},
		{"Gen 2:1", &Ref{Book: "Gen", Chapter: 2, Verse: 1}, false},
		{"Matt 1:1", &Ref{Book: "Matt", Chapter: 1, Verse: 1}, true},
		{"Rev 22:21", &Ref{Book: "Rev", Chapter: 22, Verse: 21}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate index
			idx, err := v.CalculateIndex(tt.ref, tt.isNT)
			if err != nil {
				t.Fatalf("CalculateIndex failed: %v", err)
			}

			// Convert back to ref
			gotRef, err := v.IndexToRef(idx, tt.isNT)
			if err != nil {
				t.Fatalf("IndexToRef failed: %v", err)
			}

			// Verify roundtrip
			if gotRef.Book != tt.ref.Book {
				t.Errorf("Book = %q, want %q", gotRef.Book, tt.ref.Book)
			}
			if gotRef.Chapter != tt.ref.Chapter {
				t.Errorf("Chapter = %d, want %d", gotRef.Chapter, tt.ref.Chapter)
			}
			if gotRef.Verse != tt.ref.Verse {
				t.Errorf("Verse = %d, want %d", gotRef.Verse, tt.ref.Verse)
			}
		})
	}
}

func TestVersificationIndexToRefErrors(t *testing.T) {
	v, _ := NewVersification(VersKJV)

	tests := []struct {
		name    string
		index   int
		isNT    bool
		wantErr bool
	}{
		{"index 0", 0, false, true},
		{"index 1", 1, false, true},
		{"index 2", 2, false, true},
		{"index 3", 3, false, true},
		{"index 4 valid", 4, false, false},
		{"very large index", 100000, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := v.IndexToRef(tt.index, tt.isNT)
			if (err != nil) != tt.wantErr {
				t.Errorf("IndexToRef(%d) error = %v, wantErr %v", tt.index, err, tt.wantErr)
			}
		})
	}
}

func TestKJVVersificationData(t *testing.T) {
	v := newKJVVersification()

	// Verify we have 66 books
	if len(v.Books) != 66 {
		t.Errorf("KJV has %d books, want 66", len(v.Books))
	}

	// Verify first and last books
	if v.Books[0].OSIS != "Gen" {
		t.Errorf("First book = %q, want Gen", v.Books[0].OSIS)
	}
	if v.Books[65].OSIS != "Rev" {
		t.Errorf("Last book = %q, want Rev", v.Books[65].OSIS)
	}

	// Verify some specific verse counts
	// Genesis 1:31
	if v.Books[0].Chapters[0] != 31 {
		t.Errorf("Gen 1 has %d verses, want 31", v.Books[0].Chapters[0])
	}

	// Psalm 119:176
	psIdx := v.GetBookIndex("Ps")
	if v.Books[psIdx].Chapters[118] != 176 {
		t.Errorf("Ps 119 has %d verses, want 176", v.Books[psIdx].Chapters[118])
	}
}

func TestNRSVVersification(t *testing.T) {
	v := newNRSVVersification()
	if v.ID != VersNRSV {
		t.Errorf("ID = %q, want NRSV", v.ID)
	}
	// Should have same book count as KJV
	if len(v.Books) != 66 {
		t.Errorf("NRSV has %d books, want 66", len(v.Books))
	}
}

func TestVulgateVersification(t *testing.T) {
	v := newVulgateVersification()
	if v.ID != VersVulgate {
		t.Errorf("ID = %q, want Vulgate", v.ID)
	}
	// Vulgate includes deuterocanonical books (73 total)
	if len(v.Books) != 73 {
		t.Errorf("Vulgate has %d books, want 73", len(v.Books))
	}
}

// TestVulgatePsalmVerseCounts verifies that Vulgate Psalm verse counts
// match the SWORD canon_vulg.h specification. This is critical for correct
// verse extraction from Vulgate-versified modules like DRC.
func TestVulgatePsalmVerseCounts(t *testing.T) {
	v := newVulgateVersification()

	// Find Psalms book
	var psalms *BookData
	for i := range v.Books {
		if v.Books[i].OSIS == "Ps" {
			psalms = &v.Books[i]
			break
		}
	}
	if psalms == nil {
		t.Fatal("Psalms not found in Vulgate versification")
	}

	// Verify key Psalm verse counts that differ from KJV
	// These are from SWORD canon_vulg.h
	// Note: Vulgate Psalm numbering is offset from Hebrew/KJV:
	// - Vulg Ps 9 = Hebrew Ps 9-10 combined
	// - Vulg Ps 118 = Hebrew Ps 119 (the longest psalm, 176 verses)
	testCases := []struct {
		psalm int // 1-indexed
		want  int
	}{
		{9, 39},   // Vulg Ps 9 = Hebrew Ps 9-10 combined (39 verses)
		{50, 21},  // Vulg Ps 50
		{65, 20},  // Vulg Ps 65 = Hebrew Ps 66 (20 verses, NOT 13 like KJV Ps 65)
		{66, 8},   // Vulg Ps 66 = Hebrew Ps 67 (8 verses)
		{113, 26}, // Vulg Ps 113 = Hebrew Ps 114-115 combined
		{118, 176}, // Vulg Ps 118 = Hebrew Ps 119 (176 verses)
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Psalm_%d", tc.psalm), func(t *testing.T) {
			if tc.psalm > len(psalms.Chapters) {
				t.Fatalf("Psalm %d out of range (max %d)", tc.psalm, len(psalms.Chapters))
			}
			got := psalms.Chapters[tc.psalm-1]
			if got != tc.want {
				t.Errorf("Vulgate Psalm %d: got %d verses, want %d", tc.psalm, got, tc.want)
			}
		})
	}
}

func TestVersificationConstants(t *testing.T) {
	// Verify constant values
	if VersKJV != "KJV" {
		t.Errorf("VersKJV = %q, want KJV", VersKJV)
	}
	if VersNRSV != "NRSV" {
		t.Errorf("VersNRSV = %q, want NRSV", VersNRSV)
	}
	if VersVulgate != "Vulgate" {
		t.Errorf("VersVulgate = %q, want Vulgate", VersVulgate)
	}
}
