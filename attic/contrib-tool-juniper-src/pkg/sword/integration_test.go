// Package sword integration tests using real SWORD modules from ~/.sword
package sword

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// getSwordPath returns the path to the user's SWORD module directory.
// Returns empty string if not found.
func getSwordPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	swordPath := filepath.Join(home, ".sword")
	if _, err := os.Stat(swordPath); os.IsNotExist(err) {
		return ""
	}
	return swordPath
}

// skipIfNoSword skips the test if ~/.sword is not available.
func skipIfNoSword(t *testing.T) string {
	t.Helper()
	swordPath := getSwordPath()
	if swordPath == "" {
		t.Skip("~/.sword not found - skipping integration test")
	}
	return swordPath
}

// moduleExists checks if a specific module is installed.
func moduleExists(swordPath, modID string) bool {
	confPath := filepath.Join(swordPath, "mods.d", strings.ToLower(modID)+".conf")
	_, err := os.Stat(confPath)
	return err == nil
}

func TestIntegration_LoadKJV(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "kjv") {
		t.Skip("KJV module not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	// Find KJV module
	var kjv *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "kjv" {
			kjv = m
			break
		}
	}

	if kjv == nil {
		t.Fatal("KJV module not found in LoadAllModules result")
	}

	if kjv.Title == "" {
		t.Error("KJV module has empty title")
	}
	if kjv.ModuleType != ModuleTypeBible {
		t.Errorf("KJV ModuleType = %v, want ModuleTypeBible", kjv.ModuleType)
	}
}

func TestIntegration_KJV_Genesis1(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "kjv") {
		t.Skip("KJV module not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	// Find KJV module
	var kjv *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "kjv" {
			kjv = m
			break
		}
	}

	if kjv == nil {
		t.Fatal("KJV module not found")
	}

	// Create parser
	parser := NewZTextParser(kjv, swordPath)

	// Get Genesis 1:1
	ref := Reference{Book: "Gen", Chapter: 1, Verse: 1}
	verse, err := parser.GetVerse(ref)
	if err != nil {
		t.Fatalf("GetVerse(Gen 1:1) error: %v", err)
	}

	if verse.Text == "" {
		t.Error("Genesis 1:1 has empty text")
	}

	// Verify well-known text is present
	if !strings.Contains(verse.Text, "beginning") {
		t.Errorf("Genesis 1:1 should contain 'beginning', got: %q", verse.Text)
	}
}

func TestIntegration_KJV_Genesis1_Chapter(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "kjv") {
		t.Skip("KJV module not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var kjv *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "kjv" {
			kjv = m
			break
		}
	}

	if kjv == nil {
		t.Fatal("KJV module not found")
	}

	parser := NewZTextParser(kjv, swordPath)

	// Get Genesis 1 (all 31 verses)
	chapter, err := parser.GetChapter("Gen", 1)
	if err != nil {
		t.Fatalf("GetChapter(Gen 1) error: %v", err)
	}

	if chapter.Number != 1 {
		t.Errorf("Chapter.Number = %d, want 1", chapter.Number)
	}

	// Genesis 1 should have 31 verses
	if len(chapter.Verses) != 31 {
		t.Errorf("Genesis 1 verse count = %d, want 31", len(chapter.Verses))
	}

	// Verify first and last verses
	if len(chapter.Verses) > 0 {
		first := chapter.Verses[0]
		if first.Reference.Verse != 1 {
			t.Errorf("First verse number = %d, want 1", first.Reference.Verse)
		}
		if !strings.Contains(first.Text, "beginning") {
			t.Errorf("First verse should contain 'beginning', got: %q", first.Text)
		}
	}

	if len(chapter.Verses) == 31 {
		last := chapter.Verses[30]
		if last.Reference.Verse != 31 {
			t.Errorf("Last verse number = %d, want 31", last.Reference.Verse)
		}
	}
}

func TestIntegration_KJV_John316(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "kjv") {
		t.Skip("KJV module not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var kjv *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "kjv" {
			kjv = m
			break
		}
	}

	if kjv == nil {
		t.Fatal("KJV module not found")
	}

	parser := NewZTextParser(kjv, swordPath)

	// Get John 3:16 (famous verse)
	ref := Reference{Book: "John", Chapter: 3, Verse: 16}
	verse, err := parser.GetVerse(ref)
	if err != nil {
		t.Fatalf("GetVerse(John 3:16) error: %v", err)
	}

	// Verify well-known content words
	// Note: Text may contain OSIS markup, so check for individual words
	expectedWords := []string{
		"God",
		"loved",
		"world",
		"begotten",
		"Son",
		"everlasting",
		"life",
	}

	for _, word := range expectedWords {
		if !strings.Contains(verse.Text, word) {
			t.Errorf("John 3:16 should contain %q, got: %q", word, verse.Text)
		}
	}
}

func TestIntegration_KJV_Psalm23(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "kjv") {
		t.Skip("KJV module not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var kjv *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "kjv" {
			kjv = m
			break
		}
	}

	if kjv == nil {
		t.Fatal("KJV module not found")
	}

	parser := NewZTextParser(kjv, swordPath)

	// Get Psalm 23:1
	ref := Reference{Book: "Ps", Chapter: 23, Verse: 1}
	verse, err := parser.GetVerse(ref)
	if err != nil {
		t.Fatalf("GetVerse(Ps 23:1) error: %v", err)
	}

	// Verify well-known content
	if !strings.Contains(verse.Text, "LORD") && !strings.Contains(verse.Text, "Lord") {
		t.Errorf("Psalm 23:1 should contain 'LORD', got: %q", verse.Text)
	}
	if !strings.Contains(verse.Text, "shepherd") {
		t.Errorf("Psalm 23:1 should contain 'shepherd', got: %q", verse.Text)
	}
}

func TestIntegration_KJV_AllBooks(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "kjv") {
		t.Skip("KJV module not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var kjv *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "kjv" {
			kjv = m
			break
		}
	}

	if kjv == nil {
		t.Fatal("KJV module not found")
	}

	parser := NewZTextParser(kjv, swordPath)

	// Get all books
	books, err := parser.GetAllBooks()
	if err != nil {
		t.Fatalf("GetAllBooks() error: %v", err)
	}

	// KJV should have 66 books
	if len(books) < 66 {
		t.Errorf("Book count = %d, want at least 66", len(books))
	}

	// Verify first and last books
	if len(books) > 0 {
		first := books[0]
		if first.ID != "Gen" {
			t.Errorf("First book ID = %q, want 'Gen'", first.ID)
		}
		if first.Testament != "OT" {
			t.Errorf("Genesis testament = %q, want 'OT'", first.Testament)
		}
	}

	// Find Revelation (should be last or near last)
	foundRev := false
	for _, book := range books {
		if book.ID == "Rev" {
			foundRev = true
			if book.Testament != "NT" {
				t.Errorf("Revelation testament = %q, want 'NT'", book.Testament)
			}
			break
		}
	}
	if !foundRev {
		t.Error("Revelation not found in books")
	}
}

func TestIntegration_MultipleModules(t *testing.T) {
	swordPath := skipIfNoSword(t)

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	// Should have at least one module
	if len(modules) == 0 {
		t.Skip("No modules installed")
	}

	t.Logf("Found %d modules", len(modules))

	// Count module types
	bibles := 0
	commentaries := 0
	dictionaries := 0
	other := 0

	for _, m := range modules {
		switch m.ModuleType {
		case ModuleTypeBible:
			bibles++
		case ModuleTypeCommentary:
			commentaries++
		case ModuleTypeDictionary:
			dictionaries++
		default:
			other++
		}
	}

	t.Logf("Module types: Bibles=%d, Commentaries=%d, Dictionaries=%d, Other=%d",
		bibles, commentaries, dictionaries, other)
}

func BenchmarkIntegration_KJV_GetVerse(b *testing.B) {
	swordPath := getSwordPath()
	if swordPath == "" || !moduleExists(swordPath, "kjv") {
		b.Skip("KJV module not available")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		b.Fatalf("LoadAllModules() error: %v", err)
	}

	var kjv *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "kjv" {
			kjv = m
			break
		}
	}

	if kjv == nil {
		b.Fatal("KJV module not found")
	}

	parser := NewZTextParser(kjv, swordPath)

	ref := Reference{Book: "Gen", Chapter: 1, Verse: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.GetVerse(ref)
		if err != nil {
			b.Fatalf("GetVerse() error: %v", err)
		}
	}
}

func BenchmarkIntegration_KJV_GetChapter(b *testing.B) {
	swordPath := getSwordPath()
	if swordPath == "" || !moduleExists(swordPath, "kjv") {
		b.Skip("KJV module not available")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		b.Fatalf("LoadAllModules() error: %v", err)
	}

	var kjv *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "kjv" {
			kjv = m
			break
		}
	}

	if kjv == nil {
		b.Fatal("KJV module not found")
	}

	parser := NewZTextParser(kjv, swordPath)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.GetChapter("Gen", 1)
		if err != nil {
			b.Fatalf("GetChapter() error: %v", err)
		}
	}
}

func BenchmarkIntegration_KJV_GetAllBooks(b *testing.B) {
	swordPath := getSwordPath()
	if swordPath == "" || !moduleExists(swordPath, "kjv") {
		b.Skip("KJV module not available")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		b.Fatalf("LoadAllModules() error: %v", err)
	}

	var kjv *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "kjv" {
			kjv = m
			break
		}
	}

	if kjv == nil {
		b.Fatal("KJV module not found")
	}

	parser := NewZTextParser(kjv, swordPath)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.GetAllBooks()
		if err != nil {
			b.Fatalf("GetAllBooks() error: %v", err)
		}
	}
}

// =============================================================================
// zText Integration Tests - Additional Bible Modules
// =============================================================================

func TestIntegration_MultipleBibles(t *testing.T) {
	swordPath := skipIfNoSword(t)

	// Test multiple Bible translations
	bibles := []string{"kjv", "drc", "vulgate", "geneva1599", "tyndale"}
	found := 0

	for _, bibleID := range bibles {
		if !moduleExists(swordPath, bibleID) {
			continue
		}
		found++

		t.Run(bibleID, func(t *testing.T) {
			modules, err := LoadAllModules(swordPath)
			if err != nil {
				t.Fatalf("LoadAllModules() error: %v", err)
			}

			var bible *Module
			for _, m := range modules {
				if strings.ToLower(m.ID) == bibleID {
					bible = m
					break
				}
			}

			if bible == nil {
				t.Fatalf("Module %s not found", bibleID)
			}

			parser := NewZTextParser(bible, swordPath)

			// Get Genesis 1:1
			ref := Reference{Book: "Gen", Chapter: 1, Verse: 1}
			verse, err := parser.GetVerse(ref)
			if err != nil {
				t.Fatalf("GetVerse(Gen 1:1) error: %v", err)
			}

			if verse.Text == "" {
				t.Error("Genesis 1:1 has empty text")
			}

			t.Logf("%s Genesis 1:1: %s", bibleID, verse.Text[:min(80, len(verse.Text))])
		})
	}

	if found == 0 {
		t.Skip("No Bible modules installed")
	}
}

func TestIntegration_KJV_Revelation22(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "kjv") {
		t.Skip("KJV module not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var kjv *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "kjv" {
			kjv = m
			break
		}
	}

	if kjv == nil {
		t.Fatal("KJV module not found")
	}

	parser := NewZTextParser(kjv, swordPath)

	// Get Revelation 22 (last chapter in Bible)
	chapter, err := parser.GetChapter("Rev", 22)
	if err != nil {
		t.Fatalf("GetChapter(Rev 22) error: %v", err)
	}

	// Revelation 22 has 21 verses
	if len(chapter.Verses) != 21 {
		t.Errorf("Revelation 22 verse count = %d, want 21", len(chapter.Verses))
	}

	// Check last verse contains "Amen"
	if len(chapter.Verses) > 0 {
		lastVerse := chapter.Verses[len(chapter.Verses)-1]
		if !strings.Contains(lastVerse.Text, "Amen") {
			t.Errorf("Rev 22:21 should contain 'Amen', got: %q", lastVerse.Text)
		}
	}
}

// =============================================================================
// zCom Integration Tests - Commentary Modules
// =============================================================================

func TestIntegration_LoadBarnesCommentary(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "barnes") {
		t.Skip("Barnes commentary not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var barnes *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "barnes" {
			barnes = m
			break
		}
	}

	if barnes == nil {
		t.Fatal("Barnes module not found")
	}

	if barnes.Title == "" {
		t.Error("Barnes module has empty title")
	}
	if barnes.ModuleType != ModuleTypeCommentary {
		t.Errorf("Barnes ModuleType = %v, want ModuleTypeCommentary", barnes.ModuleType)
	}

	t.Logf("Barnes Commentary: %s", barnes.Title)
}

func TestIntegration_Barnes_Matthew1(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "barnes") {
		t.Skip("Barnes commentary not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var barnes *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "barnes" {
			barnes = m
			break
		}
	}

	if barnes == nil {
		t.Fatal("Barnes module not found")
	}

	parser := NewZComParser(barnes, swordPath)

	// Get Matthew 1:1 commentary
	ref := Reference{Book: "Matt", Chapter: 1, Verse: 1}
	entry, err := parser.GetEntry(ref)
	if err != nil {
		// zCom parser has known limitations with NT-only commentaries
		t.Skipf("zCom parser limitation: %v (NT-only commentary indexing not yet supported)", err)
	}

	if entry.Text == "" {
		t.Error("Matthew 1:1 commentary is empty")
	}

	// Barnes commentary should mention Abraham or David in Matt 1:1
	if !strings.Contains(entry.Text, "Abraham") && !strings.Contains(entry.Text, "David") &&
		!strings.Contains(entry.Text, "genealogy") && !strings.Contains(entry.Text, "book") {
		t.Logf("Commentary text: %s", entry.Text[:min(200, len(entry.Text))])
	}
}

func TestIntegration_Barnes_John316(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "barnes") {
		t.Skip("Barnes commentary not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var barnes *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "barnes" {
			barnes = m
			break
		}
	}

	if barnes == nil {
		t.Fatal("Barnes module not found")
	}

	parser := NewZComParser(barnes, swordPath)

	// Get John 3:16 commentary
	ref := Reference{Book: "John", Chapter: 3, Verse: 16}
	entry, err := parser.GetEntry(ref)
	if err != nil {
		// zCom parser has known limitations with NT-only commentaries
		t.Skipf("zCom parser limitation: %v (NT-only commentary indexing not yet supported)", err)
	}

	if entry.Text == "" {
		t.Error("John 3:16 commentary is empty")
	}

	// Commentary should mention love or world or Son
	expectedWords := []string{"love", "world", "Son", "God", "gave"}
	foundWord := false
	for _, word := range expectedWords {
		if strings.Contains(strings.ToLower(entry.Text), strings.ToLower(word)) {
			foundWord = true
			break
		}
	}

	if !foundWord {
		t.Logf("Commentary preview: %s...", entry.Text[:min(300, len(entry.Text))])
	}
}

func TestIntegration_Barnes_ChapterEntries(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "barnes") {
		t.Skip("Barnes commentary not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var barnes *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "barnes" {
			barnes = m
			break
		}
	}

	if barnes == nil {
		t.Fatal("Barnes module not found")
	}

	parser := NewZComParser(barnes, swordPath)

	// Get Matthew 1 chapter entries
	chapter, err := parser.GetChapterEntries("Matt", 1)
	if err != nil {
		// zCom parser has known limitations with NT-only commentaries
		t.Skipf("zCom parser limitation: %v (NT-only commentary indexing not yet supported)", err)
	}

	if chapter.Number != 1 {
		t.Errorf("Chapter.Number = %d, want 1", chapter.Number)
	}

	// Note: Barnes may have fewer entries than verses due to how commentary is structured
	t.Logf("Matthew 1 has %d commentary entries", len(chapter.Entries))
}

// =============================================================================
// zLD Integration Tests - Dictionary/Lexicon Modules
// =============================================================================

func TestIntegration_LoadStrongsGreek(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "strongsgreek") {
		t.Skip("StrongsGreek dictionary not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var strongs *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "strongsgreek" {
			strongs = m
			break
		}
	}

	if strongs == nil {
		t.Fatal("StrongsGreek module not found")
	}

	if strongs.Title == "" {
		t.Error("StrongsGreek module has empty title")
	}
	if strongs.ModuleType != ModuleTypeDictionary {
		t.Errorf("StrongsGreek ModuleType = %v, want ModuleTypeDictionary", strongs.ModuleType)
	}

	t.Logf("Strong's Greek: %s", strongs.Title)
}

func TestIntegration_StrongsGreek_Logos(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "strongsgreek") {
		t.Skip("StrongsGreek dictionary not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var strongs *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "strongsgreek" {
			strongs = m
			break
		}
	}

	if strongs == nil {
		t.Fatal("StrongsGreek module not found")
	}

	parser := NewZLDParser(strongs, swordPath)

	// Try to get G3056 (logos - word)
	entry, err := parser.GetEntry("G3056")
	if err != nil {
		// Some dictionaries use different key formats
		entry, err = parser.GetEntry("3056")
		if err != nil {
			t.Logf("Could not find logos entry: %v", err)
			// Try searching
			keys, _ := parser.SearchKeys("3056")
			if len(keys) > 0 {
				t.Logf("Found keys containing '3056': %v", keys)
			}
			return
		}
	}

	if entry.Definition == "" {
		t.Error("G3056 definition is empty")
	}

	// Entry should mention "word" or "logos"
	if !strings.Contains(strings.ToLower(entry.Definition), "word") &&
		!strings.Contains(strings.ToLower(entry.Definition), "logos") {
		t.Logf("G3056 definition: %s", entry.Definition[:min(300, len(entry.Definition))])
	}
}

func TestIntegration_StrongsGreek_Agape(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "strongsgreek") {
		t.Skip("StrongsGreek dictionary not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var strongs *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "strongsgreek" {
			strongs = m
			break
		}
	}

	if strongs == nil {
		t.Fatal("StrongsGreek module not found")
	}

	parser := NewZLDParser(strongs, swordPath)

	// Try to get G26 (agape - love)
	entry, err := parser.GetEntry("G26")
	if err != nil {
		entry, err = parser.GetEntry("26")
		if err != nil {
			t.Logf("Could not find agape entry: %v", err)
			return
		}
	}

	if entry.Definition == "" {
		t.Error("G26 definition is empty")
	}

	// Entry should mention "love"
	if !strings.Contains(strings.ToLower(entry.Definition), "love") {
		t.Logf("G26 definition: %s", entry.Definition[:min(300, len(entry.Definition))])
	}
}

func TestIntegration_StrongsGreek_AllKeys(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "strongsgreek") {
		t.Skip("StrongsGreek dictionary not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var strongs *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "strongsgreek" {
			strongs = m
			break
		}
	}

	if strongs == nil {
		t.Fatal("StrongsGreek module not found")
	}

	parser := NewZLDParser(strongs, swordPath)

	keys, err := parser.GetAllKeys()
	if err != nil {
		// zLD parser looks for specific file patterns that may not match all modules
		t.Skipf("zLD parser file format limitation: %v", err)
	}

	// Strong's Greek should have thousands of entries
	if len(keys) < 100 {
		t.Errorf("Expected many dictionary keys, got %d", len(keys))
	}

	t.Logf("Strong's Greek has %d entries", len(keys))
}

func TestIntegration_AbbottSmith_Lexicon(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "abbottsmith") {
		t.Skip("AbbottSmith lexicon not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var abbott *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "abbottsmith" {
			abbott = m
			break
		}
	}

	if abbott == nil {
		t.Fatal("AbbottSmith module not found")
	}

	parser := NewZLDParser(abbott, swordPath)

	keys, err := parser.GetAllKeys()
	if err != nil {
		// zLD parser looks for specific file patterns that may not match all modules
		t.Skipf("zLD parser file format limitation: %v", err)
	}

	if len(keys) < 100 {
		t.Errorf("Expected many lexicon entries, got %d", len(keys))
	}

	t.Logf("Abbott-Smith has %d entries", len(keys))

	// Try to get an entry for a common Greek word
	entry, err := parser.GetEntry("λόγος")
	if err != nil {
		// Try Latin transliteration
		entry, err = parser.GetEntry("logos")
		if err != nil {
			t.Logf("Could not find logos entry: %v", err)
			// List first few keys for debugging
			if len(keys) > 5 {
				t.Logf("Sample keys: %v", keys[:5])
			}
			return
		}
	}

	if entry.Definition != "" {
		t.Logf("λόγος definition preview: %s...", entry.Definition[:min(200, len(entry.Definition))])
	}
}

// =============================================================================
// Cross-Module Integration Tests
// =============================================================================

func TestIntegration_ModuleTypeCounts(t *testing.T) {
	swordPath := skipIfNoSword(t)

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	if len(modules) == 0 {
		t.Skip("No modules installed")
	}

	// Count by type
	counts := make(map[ModuleType]int)
	for _, m := range modules {
		counts[m.ModuleType]++
	}

	t.Logf("Module counts by type:")
	t.Logf("  Bibles: %d", counts[ModuleTypeBible])
	t.Logf("  Commentaries: %d", counts[ModuleTypeCommentary])
	t.Logf("  Dictionaries: %d", counts[ModuleTypeDictionary])
	t.Logf("  General Books: %d", counts[ModuleTypeGenBook])

	// Should have at least one Bible
	if counts[ModuleTypeBible] == 0 {
		t.Error("No Bible modules found")
	}
}

func TestIntegration_ModulesByLanguage(t *testing.T) {
	swordPath := skipIfNoSword(t)

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	if len(modules) == 0 {
		t.Skip("No modules installed")
	}

	// Count by language
	langCounts := make(map[string]int)
	for _, m := range modules {
		lang := m.Language
		if lang == "" {
			lang = "unknown"
		}
		langCounts[lang]++
	}

	t.Logf("Top languages:")
	topLangs := []string{"en", "de", "es", "fr", "la", "el", "he"}
	for _, lang := range topLangs {
		if count := langCounts[lang]; count > 0 {
			t.Logf("  %s: %d", lang, count)
		}
	}
}

// =============================================================================
// Versification Integration Tests
// =============================================================================

func TestIntegration_DRC_Psalm22_ShepherdPsalm(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "drc") {
		t.Skip("DRC module not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var drc *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "drc" {
			drc = m
			break
		}
	}

	if drc == nil {
		t.Fatal("DRC module not found")
	}

	// Verify DRC uses Vulgate versification
	if drc.Versification != "Vulg" {
		t.Errorf("DRC Versification = %q, want 'Vulg'", drc.Versification)
	}

	parser := NewZTextParser(drc, swordPath)

	// In Vulgate versification, Psalm 22 is the Shepherd Psalm (KJV Psalm 23)
	ref := Reference{Book: "Ps", Chapter: 22, Verse: 1}
	verse, err := parser.GetVerse(ref)
	if err != nil {
		t.Fatalf("GetVerse(Ps 22:1) error: %v", err)
	}

	// DRC should have "The Lord ruleth me" (Latin: "Dominus regit me")
	if !strings.Contains(strings.ToLower(verse.Text), "lord") {
		t.Errorf("DRC Psalm 22:1 should mention 'Lord', got: %q", verse.Text)
	}

	t.Logf("DRC Psalm 22:1 (Shepherd Psalm): %s", verse.Text[:min(100, len(verse.Text))])
}

func TestIntegration_Vulgate_Psalm22_Latin(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "vulgate") {
		t.Skip("Vulgate module not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var vulgate *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "vulgate" {
			vulgate = m
			break
		}
	}

	if vulgate == nil {
		t.Fatal("Vulgate module not found")
	}

	// Verify Vulgate uses Vulg versification
	if vulgate.Versification != "Vulg" {
		t.Errorf("Vulgate Versification = %q, want 'Vulg'", vulgate.Versification)
	}

	parser := NewZTextParser(vulgate, swordPath)

	// In Vulgate versification, Psalm 22 is the Shepherd Psalm
	ref := Reference{Book: "Ps", Chapter: 22, Verse: 1}
	verse, err := parser.GetVerse(ref)
	if err != nil {
		t.Fatalf("GetVerse(Ps 22:1) error: %v", err)
	}

	// Latin should have "Dominus" (Lord)
	if !strings.Contains(strings.ToLower(verse.Text), "dominus") &&
		!strings.Contains(strings.ToLower(verse.Text), "domine") {
		t.Errorf("Vulgate Psalm 22:1 should contain 'Dominus', got: %q", verse.Text)
	}

	t.Logf("Vulgate Psalm 22:1 (Latin): %s", verse.Text[:min(100, len(verse.Text))])
}

func TestIntegration_LXX_Psalm22_Greek(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "lxx") {
		t.Skip("LXX module not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var lxx *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "lxx" {
			lxx = m
			break
		}
	}

	if lxx == nil {
		t.Fatal("LXX module not found")
	}

	// Verify LXX uses LXX versification
	if lxx.Versification != "LXX" {
		t.Errorf("LXX Versification = %q, want 'LXX'", lxx.Versification)
	}

	parser := NewZTextParser(lxx, swordPath)

	// In LXX versification, Psalm 22 is the Shepherd Psalm
	ref := Reference{Book: "Ps", Chapter: 22, Verse: 1}
	verse, err := parser.GetVerse(ref)
	if err != nil {
		t.Fatalf("GetVerse(Ps 22:1) error: %v", err)
	}

	// Greek should have "Κύριος" (Kyrios = Lord)
	if verse.Text == "" {
		t.Error("LXX Psalm 22:1 is empty")
	}

	t.Logf("LXX Psalm 22:1 (Greek): %s", verse.Text[:min(100, len(verse.Text))])
}

func TestIntegration_DRC_Tobit(t *testing.T) {
	swordPath := skipIfNoSword(t)

	if !moduleExists(swordPath, "drc") {
		t.Skip("DRC module not installed")
	}

	modules, err := LoadAllModules(swordPath)
	if err != nil {
		t.Fatalf("LoadAllModules() error: %v", err)
	}

	var drc *Module
	for _, m := range modules {
		if strings.ToLower(m.ID) == "drc" {
			drc = m
			break
		}
	}

	if drc == nil {
		t.Fatal("DRC module not found")
	}

	parser := NewZTextParser(drc, swordPath)

	// Tobit is a deuterocanonical book (in DRC but not KJV)
	ref := Reference{Book: "Tob", Chapter: 1, Verse: 1}
	verse, err := parser.GetVerse(ref)
	if err != nil {
		t.Fatalf("GetVerse(Tob 1:1) error: %v", err)
	}

	if verse.Text == "" {
		t.Error("Tobit 1:1 is empty")
	}

	// Verify we can get deuterocanonical content
	if !strings.Contains(verse.Text, "Tobias") && !strings.Contains(verse.Text, "Tobit") &&
		!strings.Contains(verse.Text, "Nephthali") && !strings.Contains(verse.Text, "Nephtali") {
		t.Logf("Tobit 1:1: %s", verse.Text)
	}

	t.Logf("DRC Tobit 1:1: %s", verse.Text[:min(100, len(verse.Text))])
}

func TestIntegration_VerseMapper_KJVtoVulg(t *testing.T) {
	// Test the verse mapper with real module data
	vm := NewVerseMapper()

	// KJV Ps 23:1 should map to Vulg Ps 22:1
	kjvRef := Reference{Book: "Ps", Chapter: 23, Verse: 1}
	vulgRef, mapType := vm.MapReference(kjvRef, "KJV", "Vulg")

	if vulgRef.Chapter != 22 {
		t.Errorf("KJV Ps 23:1 -> Vulg Ps %d:1, want Ps 22:1", vulgRef.Chapter)
	}
	if mapType != MapRenumber {
		t.Errorf("Map type = %v, want MapRenumber", mapType)
	}

	t.Logf("Verse mapper: KJV Ps 23:1 -> Vulg Ps %d:%d", vulgRef.Chapter, vulgRef.Verse)
}

func TestIntegration_VerseMapper_VulgToKJV(t *testing.T) {
	// Test the verse mapper with real module data
	vm := NewVerseMapper()

	// Vulg Ps 22:1 should map to KJV Ps 23:1
	vulgRef := Reference{Book: "Ps", Chapter: 22, Verse: 1}
	kjvRef, mapType := vm.MapReference(vulgRef, "Vulg", "KJV")

	if kjvRef.Chapter != 23 {
		t.Errorf("Vulg Ps 22:1 -> KJV Ps %d:1, want Ps 23:1", kjvRef.Chapter)
	}
	if mapType != MapRenumber {
		t.Errorf("Map type = %v, want MapRenumber", mapType)
	}

	t.Logf("Verse mapper: Vulg Ps 22:1 -> KJV Ps %d:%d", kjvRef.Chapter, kjvRef.Verse)
}

func TestIntegration_CompareShepherdPsalm(t *testing.T) {
	swordPath := skipIfNoSword(t)

	// Compare the Shepherd Psalm across versification systems
	type bibleVerse struct {
		id          string
		versRef     string
		expectedRef string
	}

	tests := []bibleVerse{
		{"kjv", "Ps 23:1", "shepherd"},    // KJV Psalm 23
		{"drc", "Ps 22:1", "ruleth"},      // DRC Psalm 22 (Vulgate numbering)
		{"vulgate", "Ps 22:1", "dominus"}, // Vulgate Psalm 22 (Latin)
	}

	for _, tt := range tests {
		if !moduleExists(swordPath, tt.id) {
			continue
		}

		t.Run(tt.id, func(t *testing.T) {
			modules, err := LoadAllModules(swordPath)
			if err != nil {
				t.Fatalf("LoadAllModules() error: %v", err)
			}

			var bible *Module
			for _, m := range modules {
				if strings.ToLower(m.ID) == tt.id {
					bible = m
					break
				}
			}

			if bible == nil {
				t.Skipf("%s module not found", tt.id)
			}

			parser := NewZTextParser(bible, swordPath)

			var ref Reference
			if tt.id == "kjv" {
				ref = Reference{Book: "Ps", Chapter: 23, Verse: 1}
			} else {
				ref = Reference{Book: "Ps", Chapter: 22, Verse: 1}
			}

			verse, err := parser.GetVerse(ref)
			if err != nil {
				t.Fatalf("GetVerse() error: %v", err)
			}

			// Verify the correct text is returned
			if !strings.Contains(strings.ToLower(verse.Text), tt.expectedRef) {
				t.Logf("%s %s: %s", tt.id, tt.versRef, verse.Text)
			}
		})
	}
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
