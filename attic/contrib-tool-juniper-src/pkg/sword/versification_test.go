package sword

import (
	"testing"
)

func TestKJVBooks_Count(t *testing.T) {
	if len(KJVBooks) != 66 {
		t.Errorf("len(KJVBooks) = %d, want 66", len(KJVBooks))
	}
}

func TestKJVBooks_OTCount(t *testing.T) {
	count := 0
	for _, book := range KJVBooks {
		if book.Testament == "OT" {
			count++
		}
	}
	if count != 39 {
		t.Errorf("OT book count = %d, want 39", count)
	}
}

func TestKJVBooks_NTCount(t *testing.T) {
	count := 0
	for _, book := range KJVBooks {
		if book.Testament == "NT" {
			count++
		}
	}
	if count != 27 {
		t.Errorf("NT book count = %d, want 27", count)
	}
}

func TestKJVBooks_AllBooksHaveValidData(t *testing.T) {
	for i, book := range KJVBooks {
		t.Run(book.ID, func(t *testing.T) {
			if book.ID == "" {
				t.Errorf("Book %d has empty ID", i)
			}
			if book.Name == "" {
				t.Errorf("Book %s has empty Name", book.ID)
			}
			if book.Abbrev == "" {
				t.Errorf("Book %s has empty Abbrev", book.ID)
			}
			if book.Testament != "OT" && book.Testament != "NT" {
				t.Errorf("Book %s has invalid Testament: %q", book.ID, book.Testament)
			}
			if book.Chapters < 1 {
				t.Errorf("Book %s has invalid Chapters: %d", book.ID, book.Chapters)
			}
		})
	}
}

func TestKJVBooks_FirstBook(t *testing.T) {
	first := KJVBooks[0]
	if first.ID != "Gen" {
		t.Errorf("First book ID = %q, want Gen", first.ID)
	}
	if first.Name != "Genesis" {
		t.Errorf("First book Name = %q, want Genesis", first.Name)
	}
	if first.Testament != "OT" {
		t.Errorf("First book Testament = %q, want OT", first.Testament)
	}
	if first.Chapters != 50 {
		t.Errorf("First book Chapters = %d, want 50", first.Chapters)
	}
}

func TestKJVBooks_LastBook(t *testing.T) {
	last := KJVBooks[65]
	if last.ID != "Rev" {
		t.Errorf("Last book ID = %q, want Rev", last.ID)
	}
	if last.Name != "Revelation" {
		t.Errorf("Last book Name = %q, want Revelation", last.Name)
	}
	if last.Testament != "NT" {
		t.Errorf("Last book Testament = %q, want NT", last.Testament)
	}
	if last.Chapters != 22 {
		t.Errorf("Last book Chapters = %d, want 22", last.Chapters)
	}
}

func TestKJVBooks_FirstNTBook(t *testing.T) {
	// Matthew should be the first NT book (index 39)
	matt := KJVBooks[39]
	if matt.ID != "Matt" {
		t.Errorf("First NT book ID = %q, want Matt", matt.ID)
	}
	if matt.Testament != "NT" {
		t.Errorf("First NT book Testament = %q, want NT", matt.Testament)
	}

	// Previous book should be OT
	mal := KJVBooks[38]
	if mal.ID != "Mal" {
		t.Errorf("Last OT book ID = %q, want Mal", mal.ID)
	}
	if mal.Testament != "OT" {
		t.Errorf("Last OT book Testament = %q, want OT", mal.Testament)
	}
}

func TestKJVBooks_SpecificChapterCounts(t *testing.T) {
	tests := []struct {
		id       string
		chapters int
	}{
		{"Gen", 50},
		{"Ps", 150},  // Psalms has most chapters
		{"Obad", 1},  // Shortest OT book
		{"Phlm", 1},  // Philemon - one chapter
		{"2John", 1}, // 2 John - one chapter
		{"3John", 1}, // 3 John - one chapter
		{"Jude", 1},  // Jude - one chapter
		{"Isa", 66},  // Isaiah
		{"Matt", 28}, // Matthew
		{"Rev", 22},  // Revelation
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			book, ok := GetBookInfo(tt.id)
			if !ok {
				t.Fatalf("GetBookInfo(%q) returned false", tt.id)
			}
			if book.Chapters != tt.chapters {
				t.Errorf("Book %s Chapters = %d, want %d", tt.id, book.Chapters, tt.chapters)
			}
		})
	}
}

func TestGetBookInfo_ByOSISID(t *testing.T) {
	tests := []struct {
		id       string
		wantName string
		wantOK   bool
	}{
		{"Gen", "Genesis", true},
		{"Exod", "Exodus", true},
		{"Matt", "Matthew", true},
		{"Rev", "Revelation", true},
		{"1Sam", "1 Samuel", true},
		{"1Cor", "1 Corinthians", true},
		{"Ps", "Psalms", true},
		{"Song", "Song of Solomon", true},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			book, ok := GetBookInfo(tt.id)
			if ok != tt.wantOK {
				t.Errorf("GetBookInfo(%q) ok = %v, want %v", tt.id, ok, tt.wantOK)
				return
			}
			if ok && book.Name != tt.wantName {
				t.Errorf("GetBookInfo(%q).Name = %q, want %q", tt.id, book.Name, tt.wantName)
			}
		})
	}
}

func TestGetBookInfo_ByAlias(t *testing.T) {
	tests := []struct {
		alias  string
		wantID string
		wantOK bool
	}{
		{"genesis", "Gen", true},
		{"Genesis", "Gen", true}, // Gets lowercased and matches alias
		{"psalms", "Ps", true},
		{"psalm", "Ps", true},
		{"matthew", "Matt", true},
		{"revelation", "Rev", true},
		{"revelations", "Rev", true}, // Common misspelling alias
		{"song of songs", "Song", true},
		{"1 samuel", "1Sam", true},
		{"1 corinthians", "1Cor", true},
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			book, ok := GetBookInfo(tt.alias)
			if ok != tt.wantOK {
				t.Errorf("GetBookInfo(%q) ok = %v, want %v", tt.alias, ok, tt.wantOK)
				return
			}
			if ok && book.ID != tt.wantID {
				t.Errorf("GetBookInfo(%q).ID = %q, want %q", tt.alias, book.ID, tt.wantID)
			}
		})
	}
}

func TestGetBookInfo_InvalidBook(t *testing.T) {
	invalid := []string{
		"Invalid",
		"NotABook",
		"",
		"gen", // lowercase OSIS ID - should fail (not an alias)
	}

	for _, id := range invalid {
		t.Run(id, func(t *testing.T) {
			book, ok := GetBookInfo(id)
			if ok {
				t.Errorf("GetBookInfo(%q) = (%v, true), want (nil, false)", id, book)
			}
		})
	}
}

func TestGetBookIndex(t *testing.T) {
	tests := []struct {
		id        string
		wantIndex int
		wantOK    bool
	}{
		{"Gen", 0, true},
		{"Exod", 1, true},
		{"Mal", 38, true},
		{"Matt", 39, true},
		{"Rev", 65, true},
		{"Invalid", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			idx, ok := GetBookIndex(tt.id)
			if ok != tt.wantOK {
				t.Errorf("GetBookIndex(%q) ok = %v, want %v", tt.id, ok, tt.wantOK)
				return
			}
			if ok && idx != tt.wantIndex {
				t.Errorf("GetBookIndex(%q) = %d, want %d", tt.id, idx, tt.wantIndex)
			}
		})
	}
}

func TestIsOldTestament(t *testing.T) {
	otBooks := []string{"Gen", "Exod", "Ps", "Isa", "Mal"}
	for _, id := range otBooks {
		t.Run(id, func(t *testing.T) {
			if !IsOldTestament(id) {
				t.Errorf("IsOldTestament(%q) = false, want true", id)
			}
		})
	}

	ntBooks := []string{"Matt", "John", "Rom", "Rev"}
	for _, id := range ntBooks {
		t.Run(id, func(t *testing.T) {
			if IsOldTestament(id) {
				t.Errorf("IsOldTestament(%q) = true for NT book, want false", id)
			}
		})
	}

	t.Run("invalid", func(t *testing.T) {
		if IsOldTestament("Invalid") {
			t.Error("IsOldTestament(Invalid) = true, want false")
		}
	})
}

func TestIsNewTestament(t *testing.T) {
	ntBooks := []string{"Matt", "John", "Acts", "Rom", "Rev"}
	for _, id := range ntBooks {
		t.Run(id, func(t *testing.T) {
			if !IsNewTestament(id) {
				t.Errorf("IsNewTestament(%q) = false, want true", id)
			}
		})
	}

	otBooks := []string{"Gen", "Ps", "Isa"}
	for _, id := range otBooks {
		t.Run(id, func(t *testing.T) {
			if IsNewTestament(id) {
				t.Errorf("IsNewTestament(%q) = true for OT book, want false", id)
			}
		})
	}

	t.Run("invalid", func(t *testing.T) {
		if IsNewTestament("Invalid") {
			t.Error("IsNewTestament(Invalid) = true, want false")
		}
	})
}

func TestNormalizeBookID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Already normalized
		{"Gen", "Gen"},
		{"Matt", "Matt"},
		{"Rev", "Rev"},

		// Aliases get normalized
		{"genesis", "Gen"},
		{"matthew", "Matt"},
		{"revelation", "Rev"},
		{"psalms", "Ps"},
		{"psalm", "Ps"},
		{"1 corinthians", "1Cor"},
		{"revelations", "Rev"},

		// Unknown returns unchanged
		{"Invalid", "Invalid"},
		{"unknown", "unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeBookID(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeBookID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBookAliases_AllResolveToValidBooks(t *testing.T) {
	for alias, osisID := range bookAliases {
		t.Run(alias, func(t *testing.T) {
			book, ok := GetBookInfo(osisID)
			if !ok {
				t.Errorf("Alias %q points to invalid OSIS ID %q", alias, osisID)
			}
			if book.ID != osisID {
				t.Errorf("Alias %q: GetBookInfo(%q).ID = %q, want %q", alias, osisID, book.ID, osisID)
			}
		})
	}
}

func TestBookIndex_Consistency(t *testing.T) {
	// Verify bookIndex map is consistent with KJVBooks slice
	for i, book := range KJVBooks {
		idx, ok := GetBookIndex(book.ID)
		if !ok {
			t.Errorf("Book %s not found in bookIndex", book.ID)
			continue
		}
		if idx != i {
			t.Errorf("bookIndex[%s] = %d, but book is at position %d in KJVBooks", book.ID, idx, i)
		}
	}
}

func TestGetBookInfo_ReturnsPointerToKJVBooks(t *testing.T) {
	// Verify we get a pointer to the actual KJVBooks entry, not a copy
	book1, ok1 := GetBookInfo("Gen")
	book2, ok2 := GetBookInfo("Gen")

	if !ok1 || !ok2 {
		t.Fatal("GetBookInfo(Gen) failed")
	}

	if book1 != book2 {
		t.Error("GetBookInfo returns different pointers for same book")
	}
}

func TestSingleChapterBooks(t *testing.T) {
	singleChapterBooks := []string{
		"Obad",  // Obadiah
		"Phlm",  // Philemon
		"2John", // 2 John
		"3John", // 3 John
		"Jude",  // Jude
	}

	for _, id := range singleChapterBooks {
		t.Run(id, func(t *testing.T) {
			book, ok := GetBookInfo(id)
			if !ok {
				t.Fatalf("GetBookInfo(%q) returned false", id)
			}
			if book.Chapters != 1 {
				t.Errorf("Book %s should have 1 chapter, got %d", id, book.Chapters)
			}
		})
	}
}

func TestLongestBooks(t *testing.T) {
	tests := []struct {
		id       string
		chapters int
	}{
		{"Ps", 150},  // Psalms - longest by chapter count
		{"Isa", 66},  // Isaiah
		{"Jer", 52},  // Jeremiah
		{"Gen", 50},  // Genesis
		{"Ezek", 48}, // Ezekiel
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			book, ok := GetBookInfo(tt.id)
			if !ok {
				t.Fatalf("GetBookInfo(%q) returned false", tt.id)
			}
			if book.Chapters != tt.chapters {
				t.Errorf("Book %s has %d chapters, want %d", tt.id, book.Chapters, tt.chapters)
			}
		})
	}
}
