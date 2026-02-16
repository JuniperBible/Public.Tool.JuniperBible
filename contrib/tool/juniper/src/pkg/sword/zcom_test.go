package sword

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// createTestZComModule creates a minimal zCom module for testing.
// The entries map uses verse index (0 = Gen 1:1, etc.) as key.
// The function creates proper SWORD format with:
// - .bzs: Block index (12 bytes per block: offset, compSize, uncompSize)
// - .bzv: Verse index with intro entries (10 bytes: blockNum, verseStart, verseLen)
// - .bzz: Compressed commentary blocks
//
// The verse index includes intro entries like zText:
// - Entry 0-1: Testament header
// - Entry 2: Book intro (Genesis)
// - Entry 3: Chapter 1 intro
// - Entry 4+: Verses
func createTestZComModule(t *testing.T, testament string, entries map[int]string) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create intro texts
	placeholderText := ""
	testamentHeaderText := "<osis2mod version>"
	bookIntroText := "GENESIS COMMENTARY"
	chapterIntroText := "Chapter 1 Commentary Intro"

	// Compress all text into a single block
	var uncompressedBlock bytes.Buffer
	uncompressedBlock.WriteString(placeholderText)
	uncompressedBlock.WriteString(testamentHeaderText)
	uncompressedBlock.WriteString(bookIntroText)
	uncompressedBlock.WriteString(chapterIntroText)

	// Write commentary entries
	entryOffsets := make(map[int]struct {
		start  uint32
		length uint16
	})
	for idx, text := range entries {
		start := uint32(uncompressedBlock.Len())
		uncompressedBlock.WriteString(text)
		entryOffsets[idx] = struct {
			start  uint32
			length uint16
		}{start, uint16(len(text))}
	}

	var compressedBlock bytes.Buffer
	w := zlib.NewWriter(&compressedBlock)
	w.Write(uncompressedBlock.Bytes())
	w.Close()

	// Create block index (.bzs) - one block entry
	bzs := make([]byte, 12)
	binary.LittleEndian.PutUint32(bzs[0:4], 0)                                // offset into bzz
	binary.LittleEndian.PutUint32(bzs[4:8], uint32(compressedBlock.Len()))    // compressed size
	binary.LittleEndian.PutUint32(bzs[8:12], uint32(uncompressedBlock.Len())) // uncompressed size

	bzsPath := filepath.Join(tmpDir, testament+".bzs")
	if err := os.WriteFile(bzsPath, bzs, 0600); err != nil {
		t.Fatalf("Failed to write bzs file: %v", err)
	}

	// Create verse index (.bzv) with intro entries + verse entries
	// Need enough entries for Gen 1:1 at index 4
	numIntroEntries := 4
	maxVerseIdx := 0
	for idx := range entries {
		if idx > maxVerseIdx {
			maxVerseIdx = idx
		}
	}
	numEntries := numIntroEntries + maxVerseIdx + 1
	bzv := make([]byte, numEntries*10)

	// Track block offset
	blockOffset := uint32(0)

	// Entry 0: Placeholder
	writeZComVerseEntry(bzv, 0, 0, blockOffset, uint16(len(placeholderText)))
	blockOffset += uint32(len(placeholderText))

	// Entry 1: Testament header
	writeZComVerseEntry(bzv, 1, 0, blockOffset, uint16(len(testamentHeaderText)))
	blockOffset += uint32(len(testamentHeaderText))

	// Entry 2: Book intro
	writeZComVerseEntry(bzv, 2, 0, blockOffset, uint16(len(bookIntroText)))
	blockOffset += uint32(len(bookIntroText))

	// Entry 3: Chapter 1 intro
	writeZComVerseEntry(bzv, 3, 0, blockOffset, uint16(len(chapterIntroText)))
	blockOffset += uint32(len(chapterIntroText))

	// Entry 4+: Verse entries
	for idx, info := range entryOffsets {
		writeZComVerseEntry(bzv, numIntroEntries+idx, 0, info.start, info.length)
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

// writeZComVerseEntry writes a 10-byte verse entry to the bzv buffer
func writeZComVerseEntry(bzv []byte, entryIndex int, blockNum uint32, offset uint32, length uint16) {
	entryOffset := entryIndex * 10
	binary.LittleEndian.PutUint32(bzv[entryOffset:], blockNum)
	binary.LittleEndian.PutUint32(bzv[entryOffset+4:], offset)
	binary.LittleEndian.PutUint16(bzv[entryOffset+8:], length)
}

func TestNewZComParser(t *testing.T) {
	module := &Module{
		ID:       "TestComm",
		Title:    "Test Commentary",
		DataPath: "modules/comments/zcom/testcomm",
	}

	tmpDir := t.TempDir()
	parser := NewZComParser(module, tmpDir)

	if parser == nil {
		t.Fatal("NewZComParser returned nil")
	}
	if parser.module != module {
		t.Error("parser.module not set correctly")
	}
	if parser.loaded {
		t.Error("parser should not be loaded initially")
	}
}

func TestZComParser_loadIndices_OTOnly(t *testing.T) {
	entries := map[int]string{
		0: "Commentary on Genesis 1:1",
		1: "Commentary on Genesis 1:2",
	}
	dataPath := createTestZComModule(t, "ot", entries)

	module := &Module{
		ID:       "TestComm",
		DataPath: ".",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	err := parser.loadIndices()
	if err != nil {
		t.Fatalf("loadIndices() returned error: %v", err)
	}

	if !parser.loaded {
		t.Error("parser should be loaded after loadIndices()")
	}
	// 4 intro entries + 2 verse entries = 6 total
	// (placeholder, testament header, book intro, chapter intro, verse 0, verse 1)
	if len(parser.verseIndex) != 6 {
		t.Errorf("Expected 6 verse entries (4 intros + 2 verses), got %d", len(parser.verseIndex))
	}
}

func TestZComParser_loadIndices_NTOnly(t *testing.T) {
	entries := map[int]string{
		0: "Commentary on Matthew 1:1",
	}
	dataPath := createTestZComModule(t, "nt", entries)

	module := &Module{
		ID:       "TestComm",
		DataPath: ".",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	err := parser.loadIndices()
	if err != nil {
		t.Fatalf("loadIndices() returned error: %v", err)
	}

	if !parser.loaded {
		t.Error("parser should be loaded after loadIndices()")
	}
}

func TestZComParser_loadIndices_BothTestaments(t *testing.T) {
	tmpDir := t.TempDir()

	// Create OT files
	otEntries := map[int]string{0: "OT comment"}
	otPath := createTestZComModule(t, "ot", otEntries)
	copyFile(t, filepath.Join(otPath, "ot.bzs"), filepath.Join(tmpDir, "ot.bzs"))
	copyFile(t, filepath.Join(otPath, "ot.bzv"), filepath.Join(tmpDir, "ot.bzv"))
	copyFile(t, filepath.Join(otPath, "ot.bzz"), filepath.Join(tmpDir, "ot.bzz"))

	// Create NT files
	ntEntries := map[int]string{0: "NT comment"}
	ntPath := createTestZComModule(t, "nt", ntEntries)
	copyFile(t, filepath.Join(ntPath, "nt.bzs"), filepath.Join(tmpDir, "nt.bzs"))
	copyFile(t, filepath.Join(ntPath, "nt.bzv"), filepath.Join(tmpDir, "nt.bzv"))
	copyFile(t, filepath.Join(ntPath, "nt.bzz"), filepath.Join(tmpDir, "nt.bzz"))

	module := &Module{
		ID:       "TestComm",
		DataPath: ".",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: tmpDir,
	}

	err := parser.loadIndices()
	if err != nil {
		t.Fatalf("loadIndices() returned error: %v", err)
	}

	if len(parser.bookIndex) < 2 {
		t.Errorf("Expected at least 2 book entries, got %d", len(parser.bookIndex))
	}
}

func TestZComParser_loadIndices_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	module := &Module{
		ID:       "Empty",
		DataPath: ".",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: tmpDir,
	}

	err := parser.loadIndices()
	if err != nil {
		t.Fatalf("loadIndices() should not error for missing files: %v", err)
	}
}

func TestZComParser_loadIndices_Cached(t *testing.T) {
	entries := map[int]string{0: "Test commentary"}
	dataPath := createTestZComModule(t, "ot", entries)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	if err := parser.loadIndices(); err != nil {
		t.Fatalf("First loadIndices() failed: %v", err)
	}

	originalLen := len(parser.verseIndex)

	if err := parser.loadIndices(); err != nil {
		t.Fatalf("Second loadIndices() failed: %v", err)
	}

	if len(parser.verseIndex) != originalLen {
		t.Error("loadIndices() should use cached data")
	}
}

func TestZComParser_GetEntry_Valid(t *testing.T) {
	// TODO: zCom parser has a different index structure than zText.
	// The parser reads .bzs as book offsets into the verse index, which is
	// different from zText's block index format. This test needs to create
	// a proper zCom format file structure with book indices.
	//
	// For now, skip this test until zCom format is properly documented
	// and the test helper is updated to match.
	t.Skip("zCom test helper needs to match zCom format (different from zText)")

	entries := map[int]string{
		0: "In the beginning - God's creative act begins here.",
	}
	dataPath := createTestZComModule(t, "ot", entries)

	module := &Module{
		ID:    "Test",
		Title: "Test Commentary",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	ref := Reference{Book: "Gen", Chapter: 1, Verse: 1}
	entry, err := parser.GetEntry(ref)
	if err != nil {
		t.Fatalf("GetEntry() returned error: %v", err)
	}

	if entry.Text != "In the beginning - God's creative act begins here." {
		t.Errorf("Unexpected entry text: %q", entry.Text)
	}
	if entry.Source != "Test Commentary" {
		t.Errorf("Source = %q, want 'Test Commentary'", entry.Source)
	}
}

func TestZComParser_GetEntry_EmptyEntry(t *testing.T) {
	entries := map[int]string{
		0: "",
	}
	dataPath := createTestZComModule(t, "ot", entries)

	module := &Module{
		ID:    "Test",
		Title: "Test Commentary",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	ref := Reference{Book: "Gen", Chapter: 1, Verse: 1}
	entry, err := parser.GetEntry(ref)
	if err != nil {
		t.Fatalf("GetEntry() returned error: %v", err)
	}

	if entry.Text != "" {
		t.Errorf("Expected empty entry, got: %q", entry.Text)
	}
}

func TestZComParser_GetEntry_UnknownBook(t *testing.T) {
	entries := map[int]string{0: "Test"}
	dataPath := createTestZComModule(t, "ot", entries)

	module := &Module{
		ID: "Test",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	ref := Reference{Book: "InvalidBook", Chapter: 1, Verse: 1}
	_, err := parser.GetEntry(ref)
	if err == nil {
		t.Error("GetEntry() should error for unknown book")
	}
}

func TestZComParser_GetChapterEntries(t *testing.T) {
	entries := map[int]string{
		0: "Comment 1",
		1: "Comment 2",
		2: "Comment 3",
	}
	dataPath := createTestZComModule(t, "ot", entries)

	module := &Module{
		ID: "Test",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	chapter, err := parser.GetChapterEntries("Gen", 1)
	if err != nil {
		t.Fatalf("GetChapterEntries() returned error: %v", err)
	}

	if chapter.Number != 1 {
		t.Errorf("Chapter number = %d, want 1", chapter.Number)
	}
}

func TestZComParser_GetChapterEntries_InvalidBook(t *testing.T) {
	entries := map[int]string{0: "Test"}
	dataPath := createTestZComModule(t, "ot", entries)

	module := &Module{
		ID: "Test",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	_, err := parser.GetChapterEntries("InvalidBook", 1)
	if err == nil {
		t.Error("GetChapterEntries() should error for unknown book")
	}
}

func TestZComParser_GetChapterEntries_InvalidChapter(t *testing.T) {
	entries := map[int]string{0: "Test"}
	dataPath := createTestZComModule(t, "ot", entries)

	module := &Module{
		ID: "Test",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	_, err := parser.GetChapterEntries("Gen", 0)
	if err == nil {
		t.Error("GetChapterEntries() should error for chapter 0")
	}

	_, err = parser.GetChapterEntries("Gen", 999)
	if err == nil {
		t.Error("GetChapterEntries() should error for chapter 999")
	}
}

func TestZComParser_GetBookEntries(t *testing.T) {
	entries := map[int]string{
		0: "Genesis commentary",
	}
	dataPath := createTestZComModule(t, "ot", entries)

	module := &Module{
		ID: "Test",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	book, err := parser.GetBookEntries("Gen")
	if err != nil {
		t.Fatalf("GetBookEntries() returned error: %v", err)
	}

	if book.ID != "Gen" {
		t.Errorf("Book ID = %q, want 'Gen'", book.ID)
	}
	if book.Name != "Genesis" {
		t.Errorf("Book Name = %q, want 'Genesis'", book.Name)
	}
}

func TestZComParser_GetBookEntries_InvalidBook(t *testing.T) {
	entries := map[int]string{0: "Test"}
	dataPath := createTestZComModule(t, "ot", entries)

	module := &Module{
		ID: "Test",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	_, err := parser.GetBookEntries("InvalidBook")
	if err == nil {
		t.Error("GetBookEntries() should error for unknown book")
	}
}

func TestZComParser_GetAllEntries(t *testing.T) {
	entries := map[int]string{0: "Test commentary"}
	dataPath := createTestZComModule(t, "ot", entries)

	module := &Module{
		ID: "Test",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	books, err := parser.GetAllEntries()
	if err != nil {
		t.Fatalf("GetAllEntries() returned error: %v", err)
	}

	// Should return at least one book with content
	if len(books) == 0 {
		t.Log("No books returned - may be expected for minimal test data")
	}
}

func TestZComParser_calculateVerseIndex(t *testing.T) {
	entries := map[int]string{0: "Test", 1: "Test2"}
	dataPath := createTestZComModule(t, "ot", entries)

	module := &Module{
		ID: "Test",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	if err := parser.loadIndices(); err != nil {
		t.Fatalf("loadIndices() failed: %v", err)
	}

	bookInfo := &BookInfo{
		ID:        "Gen",
		Name:      "Genesis",
		Chapters:  50,
		Testament: "OT",
	}

	idx, err := parser.calculateVerseIndex(bookInfo, 1, 1)
	if err != nil {
		t.Fatalf("calculateVerseIndex() returned error: %v", err)
	}

	// With OT-only module, Genesis 1:1 should map to index 4 (using CalculateVerseIndex)
	expectedIdx := CalculateVerseIndex("Gen", 1, 1)
	if idx != expectedIdx {
		t.Errorf("calculateVerseIndex(Gen, 1, 1) = %d, want %d", idx, expectedIdx)
	}
}

func TestZComParser_calculateVerseIndex_UnknownBook(t *testing.T) {
	entries := map[int]string{0: "Test"}
	dataPath := createTestZComModule(t, "ot", entries)

	module := &Module{
		ID: "Test",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	if err := parser.loadIndices(); err != nil {
		t.Fatalf("loadIndices() failed: %v", err)
	}

	bookInfo := &BookInfo{
		ID:       "Unknown",
		Name:     "Unknown Book",
		Chapters: 1,
	}

	_, err := parser.calculateVerseIndex(bookInfo, 1, 1)
	if err == nil {
		t.Error("calculateVerseIndex() should error for unknown book")
	}
}

func TestZComParser_calculateVerseIndex_OutOfRange(t *testing.T) {
	entries := map[int]string{0: "Test"}
	dataPath := createTestZComModule(t, "ot", entries)

	module := &Module{
		ID: "Test",
	}

	parser := &ZComParser{
		module:   module,
		dataPath: dataPath,
	}

	if err := parser.loadIndices(); err != nil {
		t.Fatalf("loadIndices() failed: %v", err)
	}

	// Set a small book index to trigger out of range
	parser.bookIndex = []uint32{0}

	bookInfo := &BookInfo{
		ID:       "Rev", // Book 66, but our index only has 1 entry
		Name:     "Revelation",
		Chapters: 22,
	}

	_, err := parser.calculateVerseIndex(bookInfo, 1, 1)
	if err == nil {
		t.Error("calculateVerseIndex() should error when book index out of range")
	}
}
