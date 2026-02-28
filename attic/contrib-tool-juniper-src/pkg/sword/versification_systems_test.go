package sword

import (
	"testing"
)

func TestVersificationSystemsRegistered(t *testing.T) {
	// Verify all expected versification systems are registered
	expected := []string{"KJV", "KJVA", "Vulg", "LXX"}

	for _, name := range expected {
		t.Run(name, func(t *testing.T) {
			system := GetVersification(name)
			if system == nil {
				t.Errorf("Versification system %q not registered", name)
				return
			}
			if system.Name != name {
				t.Errorf("System name = %q, want %q", system.Name, name)
			}
		})
	}
}

func TestListVersifications(t *testing.T) {
	names := ListVersifications()
	if len(names) < 4 {
		t.Errorf("ListVersifications() returned %d systems, want at least 4", len(names))
	}

	// Check that KJV, KJVA, Vulg, and LXX are present
	found := make(map[string]bool)
	for _, name := range names {
		found[name] = true
	}

	required := []string{"KJV", "KJVA", "Vulg", "LXX"}
	for _, name := range required {
		if !found[name] {
			t.Errorf("ListVersifications() missing %q", name)
		}
	}
}

func TestNormalizeVersificationName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// KJV aliases
		{"KJV", "KJV"},
		{"kjv", "KJV"},
		{"King James", "KJV"},
		{"Protestant", "KJV"},

		// Vulgate aliases
		{"Vulg", "Vulg"},
		{"vulg", "Vulg"},
		{"Vulgate", "Vulg"},
		{"vulgate", "Vulg"},
		{"Latin Vulgate", "Vulg"},

		// LXX aliases
		{"LXX", "LXX"},
		{"lxx", "LXX"},
		{"Septuagint", "LXX"},

		// Unknown returns unchanged
		{"Unknown", "Unknown"},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeVersificationName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeVersificationName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestKJVSystem_BookCount(t *testing.T) {
	system := GetVersification("KJV")
	if system == nil {
		t.Fatal("KJV system not found")
	}

	if len(system.Books) != 66 {
		t.Errorf("KJV has %d books, want 66", len(system.Books))
	}

	otCount := 0
	ntCount := 0
	for _, book := range system.Books {
		switch book.Testament {
		case "OT":
			otCount++
		case "NT":
			ntCount++
		}
	}

	if otCount != 39 {
		t.Errorf("KJV OT books = %d, want 39", otCount)
	}
	if ntCount != 27 {
		t.Errorf("KJV NT books = %d, want 27", ntCount)
	}
}

func TestVulgSystem_BookCount(t *testing.T) {
	system := GetVersification("Vulg")
	if system == nil {
		t.Fatal("Vulg system not found")
	}

	// Vulgate has 73 books (Catholic canon) plus additional texts
	if len(system.Books) < 73 {
		t.Errorf("Vulg has %d books, want at least 73", len(system.Books))
	}

	// Check for deuterocanonical books
	deuterocanonical := []string{"Tob", "Jdt", "Wis", "Sir", "Bar", "1Macc", "2Macc"}
	for _, bookID := range deuterocanonical {
		book, ok := system.GetBook(bookID)
		if !ok {
			t.Errorf("Vulg missing deuterocanonical book %q", bookID)
		} else if book.Testament != "AP" {
			t.Errorf("Vulg book %q Testament = %q, want AP", bookID, book.Testament)
		}
	}
}

func TestLXXSystem_BookCount(t *testing.T) {
	system := GetVersification("LXX")
	if system == nil {
		t.Fatal("LXX system not found")
	}

	// LXX includes additional books beyond the 66-book Protestant canon
	if len(system.Books) < 50 {
		t.Errorf("LXX has %d books, want at least 50", len(system.Books))
	}

	// Check for LXX-specific books
	lxxSpecific := []string{"1Macc", "2Macc", "3Macc", "4Macc", "PssSol", "Odes"}
	for _, bookID := range lxxSpecific {
		_, ok := system.GetBook(bookID)
		if !ok {
			t.Errorf("LXX missing book %q", bookID)
		}
	}
}

func TestVersificationSystem_GetBook(t *testing.T) {
	system := GetVersification("KJV")
	if system == nil {
		t.Fatal("KJV system not found")
	}

	tests := []struct {
		bookID    string
		wantName  string
		wantFound bool
	}{
		{"Gen", "Genesis", true},
		{"Ps", "Psalms", true},
		{"Matt", "Matthew", true},
		{"Rev", "Revelation", true},
		{"Invalid", "", false},
		{"Tob", "", false}, // Not in KJV
	}

	for _, tt := range tests {
		t.Run(tt.bookID, func(t *testing.T) {
			book, ok := system.GetBook(tt.bookID)
			if ok != tt.wantFound {
				t.Errorf("GetBook(%q) found = %v, want %v", tt.bookID, ok, tt.wantFound)
				return
			}
			if ok && book.Name != tt.wantName {
				t.Errorf("GetBook(%q).Name = %q, want %q", tt.bookID, book.Name, tt.wantName)
			}
		})
	}
}

func TestVersificationBook_Verses(t *testing.T) {
	system := GetVersification("KJV")
	if system == nil {
		t.Fatal("KJV system not found")
	}

	tests := []struct {
		bookID  string
		chapter int
		want    int
	}{
		{"Gen", 1, 31},
		{"Gen", 50, 26},
		{"Ps", 119, 176},
		{"Ps", 117, 2}, // Shortest psalm
		{"Matt", 1, 25},
		{"John", 3, 36},
		{"Rev", 22, 21},
		// Out of bounds
		{"Gen", 0, 0},
		{"Gen", 51, 0},
	}

	for _, tt := range tests {
		t.Run(tt.bookID, func(t *testing.T) {
			book, ok := system.GetBook(tt.bookID)
			if !ok {
				t.Fatalf("Book %q not found", tt.bookID)
			}
			got := book.Verses(tt.chapter)
			if got != tt.want {
				t.Errorf("book %s chapter %d: Verses() = %d, want %d", tt.bookID, tt.chapter, got, tt.want)
			}
		})
	}
}

func TestVersificationBook_TotalVerses(t *testing.T) {
	system := GetVersification("KJV")
	if system == nil {
		t.Fatal("KJV system not found")
	}

	// Genesis has 1533 verses
	gen, ok := system.GetBook("Gen")
	if !ok {
		t.Fatal("Genesis not found")
	}
	total := gen.TotalVerses()
	if total < 1500 || total > 1550 {
		t.Errorf("Genesis TotalVerses() = %d, expected ~1533", total)
	}

	// Psalms has 2461 verses
	ps, ok := system.GetBook("Ps")
	if !ok {
		t.Fatal("Psalms not found")
	}
	total = ps.TotalVerses()
	if total < 2400 || total > 2500 {
		t.Errorf("Psalms TotalVerses() = %d, expected ~2461", total)
	}
}

func TestVersificationSystem_TestamentBooks(t *testing.T) {
	system := GetVersification("KJV")
	if system == nil {
		t.Fatal("KJV system not found")
	}

	otBooks := system.OTBooks()
	ntBooks := system.NTBooks()
	apBooks := system.APBooks()

	if len(otBooks) != 39 {
		t.Errorf("OTBooks() = %d, want 39", len(otBooks))
	}
	if len(ntBooks) != 27 {
		t.Errorf("NTBooks() = %d, want 27", len(ntBooks))
	}
	if len(apBooks) != 0 {
		t.Errorf("APBooks() = %d for KJV, want 0", len(apBooks))
	}

	// First OT book should be Genesis
	if len(otBooks) > 0 && otBooks[0].ID != "Gen" {
		t.Errorf("First OT book = %q, want Gen", otBooks[0].ID)
	}

	// First NT book should be Matthew
	if len(ntBooks) > 0 && ntBooks[0].ID != "Matt" {
		t.Errorf("First NT book = %q, want Matt", ntBooks[0].ID)
	}
}

func TestVulgSystem_HasApocrypha(t *testing.T) {
	system := GetVersification("Vulg")
	if system == nil {
		t.Fatal("Vulg system not found")
	}

	apBooks := system.APBooks()
	if len(apBooks) < 7 {
		t.Errorf("Vulg APBooks() = %d, want at least 7", len(apBooks))
	}
}

func TestKJVASystem_BookCount(t *testing.T) {
	system := GetVersification("KJVA")
	if system == nil {
		t.Fatal("KJVA system not found")
	}

	// KJVA has 66 canonical books + 15 apocryphal books = 81 total
	if len(system.Books) < 80 {
		t.Errorf("KJVA has %d books, want at least 80", len(system.Books))
	}

	otCount := 0
	ntCount := 0
	apCount := 0
	for _, book := range system.Books {
		switch book.Testament {
		case "OT":
			otCount++
		case "NT":
			ntCount++
		case "AP":
			apCount++
		}
	}

	if otCount != 39 {
		t.Errorf("KJVA OT books = %d, want 39", otCount)
	}
	if ntCount != 27 {
		t.Errorf("KJVA NT books = %d, want 27", ntCount)
	}
	if apCount < 14 {
		t.Errorf("KJVA AP books = %d, want at least 14", apCount)
	}
}

func TestKJVASystem_ApocryphaBooks(t *testing.T) {
	system := GetVersification("KJVA")
	if system == nil {
		t.Fatal("KJVA system not found")
	}

	// Check for key Apocrypha books from 1611 KJV
	expectedBooks := []string{"1Esd", "2Esd", "Tob", "Jdt", "Wis", "Sir", "Bar", "1Macc", "2Macc", "PrMan"}
	for _, bookID := range expectedBooks {
		book, ok := system.GetBook(bookID)
		if !ok {
			t.Errorf("KJVA missing Apocrypha book %q", bookID)
		} else if book.Testament != "AP" {
			t.Errorf("KJVA book %q Testament = %q, want AP", bookID, book.Testament)
		}
	}
}

// =============================================================================
// GetBookByIndex Tests
// =============================================================================

func TestVersificationSystem_GetBookByIndex(t *testing.T) {
	system := GetVersification("KJV")
	if system == nil {
		t.Fatal("KJV system not found")
	}

	tests := []struct {
		index  int
		wantID string
		wantOK bool
	}{
		{0, "Gen", true},
		{1, "Exod", true},
		{38, "Mal", true},  // Last OT book
		{39, "Matt", true}, // First NT book
		{65, "Rev", true},  // Last book
		{-1, "", false},    // Invalid index
		{66, "", false},    // Out of bounds
		{100, "", false},   // Way out of bounds
	}

	for _, tt := range tests {
		t.Run(tt.wantID, func(t *testing.T) {
			book, ok := system.GetBookByIndex(tt.index)
			if ok != tt.wantOK {
				t.Errorf("GetBookByIndex(%d) ok = %v, want %v", tt.index, ok, tt.wantOK)
				return
			}
			if ok && book.ID != tt.wantID {
				t.Errorf("GetBookByIndex(%d).ID = %q, want %q", tt.index, book.ID, tt.wantID)
			}
		})
	}
}

// =============================================================================
// TotalBooks Tests
// =============================================================================

func TestVersificationSystem_TotalBooks(t *testing.T) {
	tests := []struct {
		name     string
		minBooks int
	}{
		{"KJV", 66},
		{"KJVA", 80},
		{"Vulg", 73},
		{"LXX", 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system := GetVersification(tt.name)
			if system == nil {
				t.Fatalf("System %q not found", tt.name)
			}
			total := system.TotalBooks()
			if total < tt.minBooks {
				t.Errorf("TotalBooks() = %d, want at least %d", total, tt.minBooks)
			}
		})
	}
}

// =============================================================================
// GetKJVVerseCount Tests
// =============================================================================

func TestGetKJVVerseCount(t *testing.T) {
	tests := []struct {
		book    string
		chapter int
		want    int
	}{
		{"Gen", 1, 31},
		{"Gen", 50, 26},
		{"Exod", 1, 22},
		{"Ps", 1, 6},
		{"Ps", 119, 176},
		{"Ps", 117, 2},
		{"Ps", 150, 6},
		{"Matt", 1, 25},
		{"Matt", 28, 20},
		{"John", 3, 36},
		{"Rev", 22, 21},
		// Invalid cases
		{"Gen", 0, 0},
		{"Gen", 51, 0},
		{"Invalid", 1, 0},
		{"", 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.book, func(t *testing.T) {
			got := GetKJVVerseCount(tt.book, tt.chapter)
			if got != tt.want {
				t.Errorf("GetKJVVerseCount(%q, %d) = %d, want %d", tt.book, tt.chapter, got, tt.want)
			}
		})
	}
}

// =============================================================================
// NormalizeVersificationName More Tests
// =============================================================================

func TestNormalizeVersificationName_AllCases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// KJV variants (exact matches only)
		{"KJV", "KJV"},
		{"kjv", "KJV"},
		{"King James", "KJV"},
		{"Protestant", "KJV"},

		// KJVA variants
		{"KJVA", "KJVA"},
		{"kjva", "KJVA"},
		{"KJV with Apocrypha", "KJVA"},

		// Vulgate variants
		{"Vulg", "Vulg"},
		{"vulg", "Vulg"},
		{"Vulgate", "Vulg"},
		{"vulgate", "Vulg"},
		{"Latin Vulgate", "Vulg"},

		// LXX variants
		{"LXX", "LXX"},
		{"lxx", "LXX"},
		{"Septuagint", "LXX"},

		// Catholic
		{"Catholic", "Catholic"},
		{"catholic", "Catholic"},

		// NRSV
		{"NRSV", "NRSV"},
		{"nrsv", "NRSV"},

		// MT
		{"MT", "MT"},
		{"mt", "MT"},
		{"Masoretic", "MT"},
		{"Hebrew", "MT"},

		// Synodal
		{"Synodal", "Synodal"},
		{"synodal", "Synodal"},
		{"Russian", "Synodal"},

		// Unknown - returns as-is
		{"Unknown", "Unknown"},
		{"custom", "custom"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeVersificationName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeVersificationName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// CalculateVerseIndexForSystem Tests
// =============================================================================

func TestVersificationSystem_CalculateVerseIndexForSystem(t *testing.T) {
	system := GetVersification("KJV")
	if system == nil {
		t.Fatal("KJV system not found")
	}

	tests := []struct {
		book    string
		chapter int
		verse   int
		wantPos bool // expect positive/valid index
	}{
		// Valid KJV references
		{"Gen", 1, 1, true},
		{"Gen", 1, 31, true},
		{"Exod", 1, 1, true},
		{"Ps", 1, 1, true},
		{"Matt", 1, 1, true},
		{"Rev", 22, 21, true},

		// Invalid references (will return -1)
		{"Invalid", 1, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.book, func(t *testing.T) {
			idx := system.CalculateVerseIndexForSystem(tt.book, tt.chapter, tt.verse)
			if tt.wantPos && idx < 0 {
				t.Errorf("CalculateVerseIndexForSystem(%q, %d, %d) = %d, want positive index",
					tt.book, tt.chapter, tt.verse, idx)
			}
			if !tt.wantPos && idx >= 0 {
				t.Errorf("CalculateVerseIndexForSystem(%q, %d, %d) = %d, want negative index",
					tt.book, tt.chapter, tt.verse, idx)
			}
		})
	}
}

func TestVersificationSystem_CalculateVerseIndexForSystem_Ordering(t *testing.T) {
	system := GetVersification("KJV")
	if system == nil {
		t.Fatal("KJV system not found")
	}

	// Verify that verse indices are in order
	// Gen 1:1 < Gen 1:2 < Gen 2:1 < Exod 1:1 < ... < Rev 22:21

	tests := []struct {
		ref1         [3]int
		ref2         [3]int
		book1, book2 string
	}{
		{[3]int{1, 1, 0}, [3]int{1, 2, 0}, "Gen", "Gen"},
		{[3]int{1, 31, 0}, [3]int{2, 1, 0}, "Gen", "Gen"},
	}

	for i, tt := range tests {
		t.Run("order", func(t *testing.T) {
			idx1 := system.CalculateVerseIndexForSystem(tt.book1, tt.ref1[0], tt.ref1[1])
			idx2 := system.CalculateVerseIndexForSystem(tt.book2, tt.ref2[0], tt.ref2[1])
			if idx1 < 0 || idx2 < 0 {
				t.Skipf("Could not calculate indices for test %d", i)
			}
			if idx1 >= idx2 {
				t.Errorf("Index for %s %d:%d (%d) should be less than index for %s %d:%d (%d)",
					tt.book1, tt.ref1[0], tt.ref1[1], idx1,
					tt.book2, tt.ref2[0], tt.ref2[1], idx2)
			}
		})
	}
}
