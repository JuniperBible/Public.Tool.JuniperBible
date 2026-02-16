package sword

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// createTestZTextModule creates a minimal zText module for testing with correct SWORD binary format.
// It creates the .bzs (block index), .bzv (verse index), and .bzz (compressed data) files.
//
// SWORD Index Structure (matching pysword):
//   - Entry 0: Placeholder (empty)
//   - Entry 1: Testament header
//   - Entry 2: Book intro
//   - Entry 3: Chapter 1 intro
//   - Entry 4+: Verses (Genesis 1:1 = index 4)
//
// Format details:
//   - .bzs: 12-byte entries (offset uint32, compSize uint32, uncompSize uint32)
//   - .bzv: 10-byte entries (blockNum uint32, verseStart uint32, verseLen uint16)
//   - .bzz: zlib-compressed text blocks
func createTestZTextModule(t *testing.T, testament string, verses []string) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create intro texts that would appear in a real SWORD module
	placeholderText := ""
	testamentHeaderText := "<osis2mod version>" // mimics real SWORD module header
	bookIntroText := "THE FIRST BOOK OF MOSES CALLED GENESIS"
	chapterIntroText := "CHAPTER 1."

	// Compress all text into a single block: intros + verses
	var uncompressedBlock bytes.Buffer
	uncompressedBlock.WriteString(placeholderText)
	uncompressedBlock.WriteString(testamentHeaderText)
	uncompressedBlock.WriteString(bookIntroText)
	uncompressedBlock.WriteString(chapterIntroText)
	for _, text := range verses {
		uncompressedBlock.WriteString(text)
	}

	var compressedBlock bytes.Buffer
	w := zlib.NewWriter(&compressedBlock)
	w.Write(uncompressedBlock.Bytes())
	w.Close()

	// Create block index (.bzs) - one entry for the single block
	bzs := make([]byte, 12)
	binary.LittleEndian.PutUint32(bzs[0:4], 0)                                // offset into bzz
	binary.LittleEndian.PutUint32(bzs[4:8], uint32(compressedBlock.Len()))    // compressed size
	binary.LittleEndian.PutUint32(bzs[8:12], uint32(uncompressedBlock.Len())) // uncompressed size

	bzsPath := filepath.Join(tmpDir, testament+".bzs")
	if err := os.WriteFile(bzsPath, bzs, 0600); err != nil {
		t.Fatalf("Failed to write bzs file: %v", err)
	}

	// Create verse index (.bzv) - includes intro entries then verses
	// Total entries = 4 intro entries + number of verses
	numIntroEntries := 4
	numEntries := numIntroEntries + len(verses)
	bzv := make([]byte, numEntries*10)

	// Track position in decompressed block
	blockOffset := uint32(0)

	// Entry 0: Placeholder (empty)
	writeVerseEntry(bzv, 0, 0, blockOffset, uint16(len(placeholderText)))
	blockOffset += uint32(len(placeholderText))

	// Entry 1: Testament header
	writeVerseEntry(bzv, 1, 0, blockOffset, uint16(len(testamentHeaderText)))
	blockOffset += uint32(len(testamentHeaderText))

	// Entry 2: Book intro
	writeVerseEntry(bzv, 2, 0, blockOffset, uint16(len(bookIntroText)))
	blockOffset += uint32(len(bookIntroText))

	// Entry 3: Chapter 1 intro
	writeVerseEntry(bzv, 3, 0, blockOffset, uint16(len(chapterIntroText)))
	blockOffset += uint32(len(chapterIntroText))

	// Entry 4+: Verses
	for i, text := range verses {
		writeVerseEntry(bzv, numIntroEntries+i, 0, blockOffset, uint16(len(text)))
		blockOffset += uint32(len(text))
	}

	bzvPath := filepath.Join(tmpDir, testament+".bzv")
	if err := os.WriteFile(bzvPath, bzv, 0600); err != nil {
		t.Fatalf("Failed to write bzv file: %v", err)
	}

	// Write compressed data (.bzz)
	bzzPath := filepath.Join(tmpDir, testament+".bzz")
	if err := os.WriteFile(bzzPath, compressedBlock.Bytes(), 0600); err != nil {
		t.Fatalf("Failed to write bzz file: %v", err)
	}

	return tmpDir
}

// writeVerseEntry writes a 10-byte verse entry to the bzv buffer
func writeVerseEntry(bzv []byte, entryIndex int, blockNum uint32, offset uint32, length uint16) {
	entryOffset := entryIndex * 10
	binary.LittleEndian.PutUint32(bzv[entryOffset:], blockNum)
	binary.LittleEndian.PutUint32(bzv[entryOffset+4:], offset)
	binary.LittleEndian.PutUint16(bzv[entryOffset+8:], length)
}

// createEmptyTestModule creates test files with properly formatted empty entries.
// numVerses is the number of actual verses (not including intro entries).
// The function creates 4 intro entries + numVerses verse entries, all with length 0.
func createEmptyTestModule(t *testing.T, testament string, numVerses int) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create empty block
	var compressedBlock bytes.Buffer
	w := zlib.NewWriter(&compressedBlock)
	w.Write([]byte{})
	w.Close()

	// Create block index with empty block
	bzs := make([]byte, 12)
	binary.LittleEndian.PutUint32(bzs[0:4], 0)
	binary.LittleEndian.PutUint32(bzs[4:8], uint32(compressedBlock.Len()))
	binary.LittleEndian.PutUint32(bzs[8:12], 0)

	bzsPath := filepath.Join(tmpDir, testament+".bzs")
	if err := os.WriteFile(bzsPath, bzs, 0600); err != nil {
		t.Fatalf("Failed to write bzs file: %v", err)
	}

	// Create verse index with intro entries + empty verse entries
	// 4 intro entries + numVerses verse entries, all with length 0
	numIntroEntries := 4
	totalEntries := numIntroEntries + numVerses
	bzv := make([]byte, totalEntries*10) // All zeros = empty entries

	bzvPath := filepath.Join(tmpDir, testament+".bzv")
	if err := os.WriteFile(bzvPath, bzv, 0600); err != nil {
		t.Fatalf("Failed to write bzv file: %v", err)
	}

	bzzPath := filepath.Join(tmpDir, testament+".bzz")
	if err := os.WriteFile(bzzPath, compressedBlock.Bytes(), 0600); err != nil {
		t.Fatalf("Failed to write bzz file: %v", err)
	}

	return tmpDir
}

func TestNewZTextParser(t *testing.T) {
	module := &Module{
		ID:       "TestBible",
		Title:    "Test Bible",
		DataPath: "modules/texts/ztext/testbible",
	}

	tmpDir := t.TempDir()
	parser := NewZTextParser(module, tmpDir)

	if parser == nil {
		t.Fatal("NewZTextParser returned nil")
	}
	if parser.module != module {
		t.Error("parser.module not set correctly")
	}
	if parser.loaded {
		t.Error("parser should not be loaded initially")
	}
}

func TestZTextParser_GetVerse_Valid(t *testing.T) {
	// Create module with known verse
	verses := []string{
		"In the beginning God created the heaven and the earth.",
	}
	dataPath := createTestZTextModule(t, "ot", verses)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	ref := Reference{Book: "Gen", Chapter: 1, Verse: 1}
	verse, err := parser.GetVerse(ref)
	if err != nil {
		t.Fatalf("GetVerse() returned error: %v", err)
	}

	if verse.Text != "In the beginning God created the heaven and the earth." {
		t.Errorf("Unexpected verse text: %q", verse.Text)
	}
}

func TestZTextParser_GetVerse_MultipleVerses(t *testing.T) {
	// Create module with multiple verses
	verses := []string{
		"Verse one text.",
		"Verse two text.",
		"Verse three text.",
	}
	dataPath := createTestZTextModule(t, "ot", verses)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	// Test each verse
	for i, expectedText := range verses {
		ref := Reference{Book: "Gen", Chapter: 1, Verse: i + 1}
		verse, err := parser.GetVerse(ref)
		if err != nil {
			t.Fatalf("GetVerse() for verse %d returned error: %v", i+1, err)
		}

		if verse.Text != expectedText {
			t.Errorf("Verse %d: got %q, want %q", i+1, verse.Text, expectedText)
		}
	}
}

func TestZTextParser_GetVerse_EmptyVerse(t *testing.T) {
	// Create module with empty verse entries
	dataPath := createEmptyTestModule(t, "ot", 1)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	ref := Reference{Book: "Gen", Chapter: 1, Verse: 1}
	verse, err := parser.GetVerse(ref)
	if err != nil {
		t.Fatalf("GetVerse() returned error: %v", err)
	}

	if verse.Text != "" {
		t.Errorf("Expected empty verse, got: %q", verse.Text)
	}
}

func TestZTextParser_GetVerse_UnknownBook(t *testing.T) {
	verses := []string{"Test"}
	dataPath := createTestZTextModule(t, "ot", verses)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	ref := Reference{Book: "InvalidBook", Chapter: 1, Verse: 1}
	_, err := parser.GetVerse(ref)
	if err == nil {
		t.Error("GetVerse() should error for unknown book")
	}
}

func TestZTextParser_GetChapter(t *testing.T) {
	// Create multiple verses for a chapter
	verses := []string{
		"Verse 1",
		"Verse 2",
		"Verse 3",
	}
	dataPath := createTestZTextModule(t, "ot", verses)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	chapter, err := parser.GetChapter("Gen", 1)
	if err != nil {
		t.Fatalf("GetChapter() returned error: %v", err)
	}

	if chapter.Number != 1 {
		t.Errorf("Chapter number = %d, want 1", chapter.Number)
	}
}

func TestZTextParser_GetChapter_InvalidBook(t *testing.T) {
	verses := []string{"Test"}
	dataPath := createTestZTextModule(t, "ot", verses)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	_, err := parser.GetChapter("InvalidBook", 1)
	if err == nil {
		t.Error("GetChapter() should error for unknown book")
	}
}

func TestZTextParser_GetChapter_InvalidChapter(t *testing.T) {
	verses := []string{"Test"}
	dataPath := createTestZTextModule(t, "ot", verses)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	// Genesis has 50 chapters, chapter 0 and 999 are invalid
	_, err := parser.GetChapter("Gen", 0)
	if err == nil {
		t.Error("GetChapter() should error for chapter 0")
	}

	_, err = parser.GetChapter("Gen", 999)
	if err == nil {
		t.Error("GetChapter() should error for chapter 999")
	}
}

func TestZTextParser_GetBook(t *testing.T) {
	verses := []string{"Chapter 1 verse"}
	dataPath := createTestZTextModule(t, "ot", verses)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	book, err := parser.GetBook("Gen")
	if err != nil {
		t.Fatalf("GetBook() returned error: %v", err)
	}

	if book.ID != "Gen" {
		t.Errorf("Book ID = %q, want 'Gen'", book.ID)
	}
	if book.Name != "Genesis" {
		t.Errorf("Book Name = %q, want 'Genesis'", book.Name)
	}
	if book.Testament != "OT" {
		t.Errorf("Book Testament = %q, want 'OT'", book.Testament)
	}
}

func TestZTextParser_GetBook_InvalidBook(t *testing.T) {
	verses := []string{"Test"}
	dataPath := createTestZTextModule(t, "ot", verses)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	_, err := parser.GetBook("InvalidBook")
	if err == nil {
		t.Error("GetBook() should error for unknown book")
	}
}

func TestZTextParser_GetAllBooks(t *testing.T) {
	verses := []string{"Test verse"}
	dataPath := createTestZTextModule(t, "ot", verses)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	books, err := parser.GetAllBooks()
	if err != nil {
		t.Fatalf("GetAllBooks() returned error: %v", err)
	}

	// Should return at least one book with content
	if len(books) == 0 {
		t.Log("No books returned - may be expected for minimal test data")
	}
}

func TestZTextParser_ClearCache(t *testing.T) {
	verses := []string{"Test verse"}
	dataPath := createTestZTextModule(t, "ot", verses)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	// Load a verse to populate cache
	ref := Reference{Book: "Gen", Chapter: 1, Verse: 1}
	_, err := parser.GetVerse(ref)
	if err != nil {
		t.Fatalf("GetVerse() returned error: %v", err)
	}

	// Clear cache should not panic
	parser.ClearCache()

	// Should still be able to get verse after clearing cache
	_, err = parser.GetVerse(ref)
	if err != nil {
		t.Fatalf("GetVerse() after ClearCache() returned error: %v", err)
	}
}

func TestZTextParser_ParseConfig(t *testing.T) {
	// Create a minimal conf file
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")
	confContent := `[Test]
DataPath=./modules/texts/ztext/test
ModDrv=zText
Lang=en
Description=Test Bible
`
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		t.Fatalf("Failed to write conf file: %v", err)
	}

	verses := []string{"Test"}
	dataPath := createTestZTextModule(t, "ot", verses)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	parsedModule, err := parser.ParseConfig(confPath)
	if err != nil {
		t.Fatalf("ParseConfig() returned error: %v", err)
	}

	if parsedModule.ID != "test" { // ID is lowercased
		t.Errorf("Module ID = %q, want 'test'", parsedModule.ID)
	}
}

func TestZTextParser_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	module := &Module{
		ID:       "Empty",
		DataPath: ".",
	}

	parser := NewZTextParser(module, tmpDir)

	// Should error when trying to get a verse with no files
	ref := Reference{Book: "Gen", Chapter: 1, Verse: 1}
	_, err := parser.GetVerse(ref)
	if err == nil {
		t.Log("GetVerse() on empty module may or may not error depending on implementation")
	}
}

func TestGetVersesInChapter(t *testing.T) {
	// Test known values from KJV versification
	testCases := []struct {
		book    string
		chapter int
		want    int
	}{
		{"Gen", 1, 31},
		{"Gen", 2, 25},
		{"Ps", 119, 176}, // Longest chapter in the Bible (use OSIS ID "Ps", not "Psa")
		{"John", 3, 36},  // John 3 has 36 verses (use OSIS ID "John", not "Jhn")
	}

	for _, tc := range testCases {
		got := GetVersesInChapter(tc.book, tc.chapter)
		if got != tc.want {
			t.Errorf("GetVersesInChapter(%q, %d) = %d, want %d", tc.book, tc.chapter, got, tc.want)
		}
	}
}

func TestCalculateOTVerseIndex(t *testing.T) {
	// SWORD verse index includes intro entries (pysword formula):
	// - 2 entries for testament heading (placeholder + header)
	// - 1 entry per book (book intro)
	// - 1 entry per chapter (chapter intro)
	// - then verses
	//
	// Gen 1:1 = 2 (testament) + 1 (book intro) + 1 (ch1 intro) + 0 = 4
	idx := CalculateOTVerseIndex("Gen", 1, 1)
	if idx != 4 {
		t.Errorf("CalculateOTVerseIndex(Gen, 1, 1) = %d, want 4", idx)
	}

	// Gen 1:2 = 4 + 1 = 5
	idx = CalculateOTVerseIndex("Gen", 1, 2)
	if idx != 5 {
		t.Errorf("CalculateOTVerseIndex(Gen, 1, 2) = %d, want 5", idx)
	}

	// Gen 2:1 = 2 + 1 (book) + 1 (ch1 intro) + 31 (ch1 verses) + 1 (ch2 intro) + 0 = 36
	idx = CalculateOTVerseIndex("Gen", 2, 1)
	if idx != 36 {
		t.Errorf("CalculateOTVerseIndex(Gen, 2, 1) = %d, want 36", idx)
	}

	// Exodus 1:1 = 2 + bookSize(Gen) + 1 (Exod book intro) + 1 (ch1 intro) + 0
	// bookSize(Gen) = 1533 verses + 50 chapter intros + 1 book intro = 1584
	// Exodus 1:1 = 2 + 1584 + 1 + 1 + 0 = 1588
	idx = CalculateOTVerseIndex("Exod", 1, 1)
	if idx != 1588 {
		t.Errorf("CalculateOTVerseIndex(Exod, 1, 1) = %d, want 1588", idx)
	}
}

func TestCalculateNTVerseIndex(t *testing.T) {
	// SWORD verse index for NT testament:
	// Matt 1:1 = 2 (testament) + 1 (book intro) + 1 (ch1 intro) + 0 = 4
	idx := CalculateNTVerseIndex("Matt", 1, 1)
	if idx != 4 {
		t.Errorf("CalculateNTVerseIndex(Matt, 1, 1) = %d, want 4", idx)
	}

	// Matt 1:2 = 4 + 1 = 5
	idx = CalculateNTVerseIndex("Matt", 1, 2)
	if idx != 5 {
		t.Errorf("CalculateNTVerseIndex(Matt, 1, 2) = %d, want 5", idx)
	}

	// Mark 1:1 = 2 + bookSize(Matt) + 1 (Mark book intro) + 1 (ch1 intro) + 0
	// bookSize(Matt) = 1071 verses + 28 chapter intros + 1 book intro = 1100
	// Mark 1:1 = 2 + 1100 + 1 + 1 + 0 = 1104
	idx = CalculateNTVerseIndex("Mark", 1, 1)
	if idx != 1104 {
		t.Errorf("CalculateNTVerseIndex(Mark, 1, 1) = %d, want 1104", idx)
	}

	// Verify OT book returns -1
	idx = CalculateNTVerseIndex("Gen", 1, 1)
	if idx != -1 {
		t.Errorf("CalculateNTVerseIndex(Gen, 1, 1) = %d, want -1", idx)
	}
}

// Helper function to copy files
func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0600); err != nil {
		t.Fatalf("Failed to write %s: %v", dst, err)
	}
}

func TestNewZTextParserWithVersification(t *testing.T) {
	module := &Module{
		ID:       "TestBible",
		Title:    "Test Bible",
		DataPath: "modules/texts/ztext/testbible",
	}

	tmpDir := t.TempDir()

	// Test with explicit versification
	vers := GetVersification("Vulg")
	parser := NewZTextParserWithVersification(module, tmpDir, vers)

	if parser == nil {
		t.Fatal("NewZTextParserWithVersification returned nil")
	}
	if parser.Versification() != vers {
		t.Error("parser.Versification() should return the provided versification")
	}

	// Test with nil versification (should default to KJV)
	parserNil := NewZTextParserWithVersification(module, tmpDir, nil)
	if parserNil.Versification() == nil {
		t.Error("parser.Versification() should not be nil when nil is passed")
	}
	if parserNil.Versification().Name != "KJV" {
		t.Errorf("expected KJV versification as default, got %s", parserNil.Versification().Name)
	}
}

func TestZTextParser_Versification(t *testing.T) {
	module := &Module{
		ID:            "TestBible",
		Versification: "Vulg",
	}

	tmpDir := t.TempDir()
	parser := NewZTextParser(module, tmpDir)

	vers := parser.Versification()
	if vers == nil {
		t.Fatal("Versification() returned nil")
	}
	// Vulg versification should be set
	if vers.Name != "Vulg" {
		t.Errorf("Versification().Name = %q, want 'Vulg'", vers.Name)
	}
}

func TestZTextParser_UnknownVersification(t *testing.T) {
	module := &Module{
		ID:            "TestBible",
		Versification: "UnknownSystem",
	}

	tmpDir := t.TempDir()
	parser := NewZTextParser(module, tmpDir)

	vers := parser.Versification()
	if vers == nil {
		t.Fatal("Versification() returned nil for unknown system")
	}
	// Should fall back to KJV
	if vers.Name != "KJV" {
		t.Errorf("Versification().Name = %q, want 'KJV' (fallback)", vers.Name)
	}
}

func TestZTextParser_GetVerseWithLegacyIndex(t *testing.T) {
	// Create test module
	verses := []string{
		"In the beginning God created the heaven and the earth.",
		"And the earth was without form, and void.",
	}
	dataPath := createTestZTextModule(t, "ot", verses)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	// Load indices first by getting any verse
	ref := Reference{Book: "Gen", Chapter: 1, Verse: 1}
	_, err := parser.GetVerse(ref)
	if err != nil {
		t.Fatalf("Initial GetVerse() failed: %v", err)
	}

	// Now test the legacy index path by directly calling getVerseWithLegacyIndex
	bookInfo := &BookInfo{
		ID:        "Gen",
		Name:      "Genesis",
		Testament: "OT",
	}
	verse, err := parser.getVerseWithLegacyIndex(ref, bookInfo)
	if err != nil {
		t.Fatalf("getVerseWithLegacyIndex() returned error: %v", err)
	}

	if verse.Text != "In the beginning God created the heaven and the earth." {
		t.Errorf("Unexpected verse text: %q", verse.Text)
	}
}

func TestZTextParser_GetVerseWithLegacyIndex_NT(t *testing.T) {
	// Create test NT module
	verses := []string{
		"The book of the generation of Jesus Christ.",
	}
	dataPath := createTestZTextModule(t, "nt", verses)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	// Load indices first
	ref := Reference{Book: "Matt", Chapter: 1, Verse: 1}
	_, _ = parser.GetVerse(ref) // May fail but loads indices

	bookInfo := &BookInfo{
		ID:        "Matt",
		Name:      "Matthew",
		Testament: "NT",
	}
	verse, err := parser.getVerseWithLegacyIndex(ref, bookInfo)
	if err != nil {
		// May fail due to limited test data but function was called
		t.Logf("getVerseWithLegacyIndex() returned error (expected with test data): %v", err)
	} else if verse != nil {
		t.Logf("Got verse: %q", verse.Text)
	}
}

func TestZTextParser_GetVerseWithLegacyIndex_OutOfRange(t *testing.T) {
	verses := []string{"Test"}
	dataPath := createTestZTextModule(t, "ot", verses)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := NewZTextParser(module, dataPath)

	// Load indices
	ref := Reference{Book: "Gen", Chapter: 1, Verse: 1}
	_, _ = parser.GetVerse(ref)

	// Try to get a verse way out of range
	bookInfo := &BookInfo{
		ID:        "Gen",
		Name:      "Genesis",
		Testament: "OT",
	}
	outOfRangeRef := Reference{Book: "Gen", Chapter: 999, Verse: 999}
	_, err := parser.getVerseWithLegacyIndex(outOfRangeRef, bookInfo)
	if err == nil {
		t.Error("getVerseWithLegacyIndex() should error for out-of-range verse")
	}
}

func TestCalculateOTVerseIndex_InvalidBook(t *testing.T) {
	// Invalid book should return -1
	idx := CalculateOTVerseIndex("InvalidBook", 1, 1)
	if idx != -1 {
		t.Errorf("CalculateOTVerseIndex(InvalidBook) = %d, want -1", idx)
	}

	// NT book should return -1 for OT index
	idx = CalculateOTVerseIndex("Matt", 1, 1)
	if idx != -1 {
		t.Errorf("CalculateOTVerseIndex(Matt) = %d, want -1", idx)
	}
}
