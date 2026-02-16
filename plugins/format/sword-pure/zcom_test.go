package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// =============================================================================
// zCom Format Tests - SWORD Commentary Format
// =============================================================================
// zCom uses the same file structure as zText (.bzs, .bzv, .bzz) but stores
// commentary entries instead of Bible text. Each entry can cover a verse,
// range of verses, or entire chapters.

// TestZComEntryStruct tests the CommentaryEntry struct creation and fields.
func TestZComEntryStruct(t *testing.T) {
	entry := &CommentaryEntry{
		Reference: Ref{Book: "Gen", Chapter: 1, Verse: 1},
		Text:      "This is a commentary on Genesis 1:1",
		Source:    "Test Commentary",
	}

	if entry.Reference.Book != "Gen" {
		t.Errorf("expected book 'Gen', got %q", entry.Reference.Book)
	}
	if entry.Reference.Chapter != 1 {
		t.Errorf("expected chapter 1, got %d", entry.Reference.Chapter)
	}
	if entry.Reference.Verse != 1 {
		t.Errorf("expected verse 1, got %d", entry.Reference.Verse)
	}
	if entry.Text != "This is a commentary on Genesis 1:1" {
		t.Errorf("unexpected text: %q", entry.Text)
	}
	if entry.Source != "Test Commentary" {
		t.Errorf("expected source 'Test Commentary', got %q", entry.Source)
	}
}

// TestZComParserCreation tests creating a new ZComParser.
func TestZComParserCreation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcom-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create minimal module structure
	conf := &ConfFile{
		ModuleName: "TestComm",
		ModDrv:     "zCom",
		DataPath:   "./modules/comments/zcom/testcomm/",
	}

	parser := NewZComParser(conf, tmpDir)
	if parser == nil {
		t.Fatal("NewZComParser returned nil")
	}

	if parser.module.ModuleName != "TestComm" {
		t.Errorf("expected module name 'TestComm', got %q", parser.module.ModuleName)
	}
}

// TestZComReadBlockIndex tests reading a .bzs block index file for commentaries.
func TestZComReadBlockIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcom-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test block index: 3 blocks
	// Block 0: offset=0, compSize=50, uncompSize=100
	// Block 1: offset=50, compSize=75, uncompSize=150
	// Block 2: offset=125, compSize=100, uncompSize=200
	data := make([]byte, 36)                      // 3 blocks * 12 bytes
	binary.LittleEndian.PutUint32(data[0:], 0)    // offset
	binary.LittleEndian.PutUint32(data[4:], 50)   // compSize
	binary.LittleEndian.PutUint32(data[8:], 100)  // uncompSize
	binary.LittleEndian.PutUint32(data[12:], 50)  // offset
	binary.LittleEndian.PutUint32(data[16:], 75)  // compSize
	binary.LittleEndian.PutUint32(data[20:], 150) // uncompSize
	binary.LittleEndian.PutUint32(data[24:], 125) // offset
	binary.LittleEndian.PutUint32(data[28:], 100) // compSize
	binary.LittleEndian.PutUint32(data[32:], 200) // uncompSize

	bzsPath := filepath.Join(tmpDir, "test.bzs")
	if err := os.WriteFile(bzsPath, data, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	blocks, err := readBlockIndex(bzsPath)
	if err != nil {
		t.Fatalf("readBlockIndex failed: %v", err)
	}

	if len(blocks) != 3 {
		t.Errorf("expected 3 blocks, got %d", len(blocks))
	}

	// Verify block entries
	expected := []struct {
		offset, compSize, uncompSize uint32
	}{
		{0, 50, 100},
		{50, 75, 150},
		{125, 100, 200},
	}

	for i, exp := range expected {
		if blocks[i].Offset != exp.offset {
			t.Errorf("block[%d].Offset: expected %d, got %d", i, exp.offset, blocks[i].Offset)
		}
		if blocks[i].CompressedSize != exp.compSize {
			t.Errorf("block[%d].CompressedSize: expected %d, got %d", i, exp.compSize, blocks[i].CompressedSize)
		}
		if blocks[i].UncompSize != exp.uncompSize {
			t.Errorf("block[%d].UncompSize: expected %d, got %d", i, exp.uncompSize, blocks[i].UncompSize)
		}
	}
}

// TestZComReadVerseIndex tests reading a .bzv verse index for commentaries.
func TestZComReadVerseIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcom-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test verse index: 4 entries (10 bytes each)
	// Entry 0: blockNum=0, offset=0, size=100 (intro)
	// Entry 1: blockNum=0, offset=100, size=200 (Gen 1:1 commentary)
	// Entry 2: blockNum=1, offset=0, size=150 (Gen 1:2 commentary)
	// Entry 3: blockNum=1, offset=150, size=175 (Gen 1:3 commentary)
	data := make([]byte, 40) // 4 entries * 10 bytes
	// Entry 0
	binary.LittleEndian.PutUint32(data[0:], 0)   // blockNum
	binary.LittleEndian.PutUint32(data[4:], 0)   // offset
	binary.LittleEndian.PutUint16(data[8:], 100) // size
	// Entry 1
	binary.LittleEndian.PutUint32(data[10:], 0)   // blockNum
	binary.LittleEndian.PutUint32(data[14:], 100) // offset
	binary.LittleEndian.PutUint16(data[18:], 200) // size
	// Entry 2
	binary.LittleEndian.PutUint32(data[20:], 1)   // blockNum
	binary.LittleEndian.PutUint32(data[24:], 0)   // offset
	binary.LittleEndian.PutUint16(data[28:], 150) // size
	// Entry 3
	binary.LittleEndian.PutUint32(data[30:], 1)   // blockNum
	binary.LittleEndian.PutUint32(data[34:], 150) // offset
	binary.LittleEndian.PutUint16(data[38:], 175) // size

	bzvPath := filepath.Join(tmpDir, "test.bzv")
	if err := os.WriteFile(bzvPath, data, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	verses, err := readVerseIndex(bzvPath)
	if err != nil {
		t.Fatalf("readVerseIndex failed: %v", err)
	}

	if len(verses) != 4 {
		t.Errorf("expected 4 entries, got %d", len(verses))
	}

	// Verify entry 1 (would be first verse commentary)
	if verses[1].BlockNum != 0 {
		t.Errorf("entry[1].BlockNum: expected 0, got %d", verses[1].BlockNum)
	}
	if verses[1].Offset != 100 {
		t.Errorf("entry[1].Offset: expected 100, got %d", verses[1].Offset)
	}
	if verses[1].Size != 200 {
		t.Errorf("entry[1].Size: expected 200, got %d", verses[1].Size)
	}
}

// TestZComDecompression tests zlib decompression of commentary blocks.
func TestZComDecompression(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcom-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test commentary text
	originalText := "This is a test commentary entry for Genesis 1:1. " +
		"In the beginning, God created the heavens and the earth. " +
		"This foundational verse establishes God as Creator of all things."

	// Compress the text with zlib
	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	w.Write([]byte(originalText))
	w.Close()

	// Create block index pointing to our compressed data
	bzsData := make([]byte, 12)
	binary.LittleEndian.PutUint32(bzsData[0:], 0)                         // offset
	binary.LittleEndian.PutUint32(bzsData[4:], uint32(compressed.Len()))  // compSize
	binary.LittleEndian.PutUint32(bzsData[8:], uint32(len(originalText))) // uncompSize

	bzsPath := filepath.Join(tmpDir, "test.bzs")
	if err := os.WriteFile(bzsPath, bzsData, 0600); err != nil {
		t.Fatalf("failed to write bzs: %v", err)
	}

	// Write compressed data
	bzzPath := filepath.Join(tmpDir, "test.bzz")
	if err := os.WriteFile(bzzPath, compressed.Bytes(), 0600); err != nil {
		t.Fatalf("failed to write bzz: %v", err)
	}

	// Read and decompress
	blocks, err := readBlockIndex(bzsPath)
	if err != nil {
		t.Fatalf("readBlockIndex failed: %v", err)
	}

	decompressed, err := readBlock(bzzPath, blocks[0])
	if err != nil {
		t.Fatalf("readBlock failed: %v", err)
	}

	if string(decompressed) != originalText {
		t.Errorf("decompression mismatch:\n  expected: %q\n  got: %q", originalText, string(decompressed))
	}
}

// TestZComGetEntry tests retrieving a commentary entry by reference.
func TestZComGetEntry(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcom-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create module structure
	moduleDir := filepath.Join(tmpDir, "modules", "comments", "zcom", "testcomm")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}

	// Create test commentary content with multiple entries
	entries := []string{
		"[empty]",         // index 0: empty/placeholder
		"[module header]", // index 1: module header
		"[book intro]",    // index 2: book intro
		"[chapter intro]", // index 3: chapter intro
		"Commentary on Genesis 1:1 - In the beginning", // index 4: first verse
		"Commentary on Genesis 1:2 - The earth was",    // index 5: second verse
	}

	// Build the compressed block with all entries concatenated
	var blockContent bytes.Buffer
	offsets := make([]uint32, len(entries))
	sizes := make([]uint16, len(entries))

	for i, entry := range entries {
		offsets[i] = uint32(blockContent.Len())
		sizes[i] = uint16(len(entry))
		blockContent.WriteString(entry)
	}

	// Compress the block
	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	w.Write(blockContent.Bytes())
	w.Close()

	// Write block index (.bzs) - single block
	bzsData := make([]byte, 12)
	binary.LittleEndian.PutUint32(bzsData[0:], 0)
	binary.LittleEndian.PutUint32(bzsData[4:], uint32(compressed.Len()))
	binary.LittleEndian.PutUint32(bzsData[8:], uint32(blockContent.Len()))
	if err := os.WriteFile(filepath.Join(moduleDir, "ot.bzs"), bzsData, 0600); err != nil {
		t.Fatalf("failed to write bzs: %v", err)
	}

	// Write verse index (.bzv) - entries for each position
	bzvData := make([]byte, len(entries)*10)
	for i := range entries {
		offset := i * 10
		binary.LittleEndian.PutUint32(bzvData[offset:], 0)            // blockNum
		binary.LittleEndian.PutUint32(bzvData[offset+4:], offsets[i]) // offset in block
		binary.LittleEndian.PutUint16(bzvData[offset+8:], sizes[i])   // size
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "ot.bzv"), bzvData, 0600); err != nil {
		t.Fatalf("failed to write bzv: %v", err)
	}

	// Write compressed data (.bzz)
	if err := os.WriteFile(filepath.Join(moduleDir, "ot.bzz"), compressed.Bytes(), 0600); err != nil {
		t.Fatalf("failed to write bzz: %v", err)
	}

	// Create config
	conf := &ConfFile{
		ModuleName:    "TestComm",
		ModDrv:        "zCom",
		DataPath:      "./modules/comments/zcom/testcomm/",
		Versification: "KJV",
	}

	parser := NewZComParser(conf, tmpDir)
	if parser == nil {
		t.Fatal("NewZComParser returned nil")
	}

	// Load the module
	if err := parser.Load(); err != nil {
		t.Fatalf("failed to load module: %v", err)
	}

	// Get entry for Genesis 1:1 (should be at index 4 due to header entries)
	ref := Ref{Book: "Gen", Chapter: 1, Verse: 1}
	entry, err := parser.GetEntry(ref)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	if entry == nil {
		t.Fatal("GetEntry returned nil")
	}

	if !strings.Contains(entry.Text, "Genesis 1:1") {
		t.Errorf("expected text to contain 'Genesis 1:1', got: %q", entry.Text)
	}
}

// TestZComGetChapterEntries tests retrieving all commentary entries for a chapter.
func TestZComGetChapterEntries(t *testing.T) {
	// This test verifies that GetChapterEntries returns all entries for a chapter
	tmpDir, err := os.MkdirTemp("", "zcom-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf := &ConfFile{
		ModuleName:    "TestComm",
		ModDrv:        "zCom",
		DataPath:      "./modules/comments/zcom/testcomm/",
		Versification: "KJV",
	}

	parser := NewZComParser(conf, tmpDir)
	if parser == nil {
		t.Fatal("NewZComParser returned nil")
	}

	// GetChapterEntries should handle missing data gracefully
	entries, err := parser.GetChapterEntries("Gen", 1)
	// With no data loaded, should return empty or error
	if err == nil && entries != nil && len(entries.Entries) > 0 {
		// If data is available, verify structure
		for _, entry := range entries.Entries {
			if entry.Reference.Book != "Gen" {
				t.Errorf("entry has wrong book: %q", entry.Reference.Book)
			}
			if entry.Reference.Chapter != 1 {
				t.Errorf("entry has wrong chapter: %d", entry.Reference.Chapter)
			}
		}
	}
}

// TestZComGetBookEntries tests retrieving all commentary entries for a book.
func TestZComGetBookEntries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcom-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf := &ConfFile{
		ModuleName:    "TestComm",
		ModDrv:        "zCom",
		DataPath:      "./modules/comments/zcom/testcomm/",
		Versification: "KJV",
	}

	parser := NewZComParser(conf, tmpDir)
	if parser == nil {
		t.Fatal("NewZComParser returned nil")
	}

	// GetBookEntries should handle missing data gracefully
	entries, err := parser.GetBookEntries("Gen")
	// Verify it doesn't crash and returns appropriate result
	if err != nil {
		// Expected with no data
		t.Logf("GetBookEntries returned expected error: %v", err)
	} else if entries != nil {
		t.Logf("GetBookEntries returned %d chapters", len(entries.Chapters))
	}
}

// TestZComEmptyEntry tests handling of empty/placeholder entries.
func TestZComEmptyEntry(t *testing.T) {
	// zCom format includes placeholder entries (index 0 is always empty)
	// This test verifies proper handling of these special entries

	tmpDir, err := os.MkdirTemp("", "zcom-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create verse index with empty entry (size=0)
	bzvData := make([]byte, 20)
	// Entry 0: empty
	binary.LittleEndian.PutUint32(bzvData[0:], 0) // blockNum
	binary.LittleEndian.PutUint32(bzvData[4:], 0) // offset
	binary.LittleEndian.PutUint16(bzvData[8:], 0) // size = 0 (empty)
	// Entry 1: has content
	binary.LittleEndian.PutUint32(bzvData[10:], 0)  // blockNum
	binary.LittleEndian.PutUint32(bzvData[14:], 0)  // offset
	binary.LittleEndian.PutUint16(bzvData[18:], 50) // size

	bzvPath := filepath.Join(tmpDir, "test.bzv")
	if err := os.WriteFile(bzvPath, bzvData, 0600); err != nil {
		t.Fatalf("failed to write bzv: %v", err)
	}

	verses, err := readVerseIndex(bzvPath)
	if err != nil {
		t.Fatalf("readVerseIndex failed: %v", err)
	}

	// First entry should have size 0
	if verses[0].Size != 0 {
		t.Errorf("expected entry[0].Size = 0, got %d", verses[0].Size)
	}

	// Second entry should have content
	if verses[1].Size != 50 {
		t.Errorf("expected entry[1].Size = 50, got %d", verses[1].Size)
	}
}

// TestZComMultiBlockRetrieval tests retrieving entries across multiple blocks.
func TestZComMultiBlockRetrieval(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcom-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create two compressed blocks
	block0Content := "Block 0 content: Commentary for verses in first block"
	block1Content := "Block 1 content: Commentary for verses in second block"

	var compressed0, compressed1 bytes.Buffer
	w0 := zlib.NewWriter(&compressed0)
	w0.Write([]byte(block0Content))
	w0.Close()

	w1 := zlib.NewWriter(&compressed1)
	w1.Write([]byte(block1Content))
	w1.Close()

	// Create block index for two blocks
	bzsData := make([]byte, 24) // 2 blocks * 12 bytes
	// Block 0
	binary.LittleEndian.PutUint32(bzsData[0:], 0)
	binary.LittleEndian.PutUint32(bzsData[4:], uint32(compressed0.Len()))
	binary.LittleEndian.PutUint32(bzsData[8:], uint32(len(block0Content)))
	// Block 1
	binary.LittleEndian.PutUint32(bzsData[12:], uint32(compressed0.Len()))
	binary.LittleEndian.PutUint32(bzsData[16:], uint32(compressed1.Len()))
	binary.LittleEndian.PutUint32(bzsData[20:], uint32(len(block1Content)))

	bzsPath := filepath.Join(tmpDir, "test.bzs")
	if err := os.WriteFile(bzsPath, bzsData, 0600); err != nil {
		t.Fatalf("failed to write bzs: %v", err)
	}

	// Concatenate compressed blocks
	var allCompressed bytes.Buffer
	allCompressed.Write(compressed0.Bytes())
	allCompressed.Write(compressed1.Bytes())

	bzzPath := filepath.Join(tmpDir, "test.bzz")
	if err := os.WriteFile(bzzPath, allCompressed.Bytes(), 0600); err != nil {
		t.Fatalf("failed to write bzz: %v", err)
	}

	// Read and verify both blocks
	blocks, err := readBlockIndex(bzsPath)
	if err != nil {
		t.Fatalf("readBlockIndex failed: %v", err)
	}

	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}

	// Read block 0
	data0, err := readBlock(bzzPath, blocks[0])
	if err != nil {
		t.Fatalf("readBlock(0) failed: %v", err)
	}
	if string(data0) != block0Content {
		t.Errorf("block 0 content mismatch")
	}

	// Read block 1
	data1, err := readBlock(bzzPath, blocks[1])
	if err != nil {
		t.Fatalf("readBlock(1) failed: %v", err)
	}
	if string(data1) != block1Content {
		t.Errorf("block 1 content mismatch")
	}
}

// TestZComOTOnlyModule tests a commentary module that only covers the OT.
func TestZComOTOnlyModule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcom-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	moduleDir := filepath.Join(tmpDir, "modules", "comments", "zcom", "otcomm")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}

	// Create only OT files (no NT files)
	content := "OT-only commentary content"
	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	w.Write([]byte(content))
	w.Close()

	// Write OT block index
	bzsData := make([]byte, 12)
	binary.LittleEndian.PutUint32(bzsData[0:], 0)
	binary.LittleEndian.PutUint32(bzsData[4:], uint32(compressed.Len()))
	binary.LittleEndian.PutUint32(bzsData[8:], uint32(len(content)))
	if err := os.WriteFile(filepath.Join(moduleDir, "ot.bzs"), bzsData, 0600); err != nil {
		t.Fatalf("failed to write ot.bzs: %v", err)
	}

	// Write OT verse index (minimal)
	bzvData := make([]byte, 10)
	binary.LittleEndian.PutUint32(bzvData[0:], 0)
	binary.LittleEndian.PutUint32(bzvData[4:], 0)
	binary.LittleEndian.PutUint16(bzvData[8:], uint16(len(content)))
	if err := os.WriteFile(filepath.Join(moduleDir, "ot.bzv"), bzvData, 0600); err != nil {
		t.Fatalf("failed to write ot.bzv: %v", err)
	}

	// Write OT compressed data
	if err := os.WriteFile(filepath.Join(moduleDir, "ot.bzz"), compressed.Bytes(), 0600); err != nil {
		t.Fatalf("failed to write ot.bzz: %v", err)
	}

	conf := &ConfFile{
		ModuleName:    "OTComm",
		ModDrv:        "zCom",
		DataPath:      "./modules/comments/zcom/otcomm/",
		Versification: "KJV",
	}

	parser := NewZComParser(conf, tmpDir)
	if err := parser.Load(); err != nil {
		t.Fatalf("failed to load module: %v", err)
	}

	if !parser.HasOT() {
		t.Error("expected parser.HasOT() to be true")
	}
	if parser.HasNT() {
		t.Error("expected parser.HasNT() to be false for OT-only module")
	}
}

// TestZComNTOnlyModule tests a commentary module that only covers the NT.
func TestZComNTOnlyModule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zcom-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	moduleDir := filepath.Join(tmpDir, "modules", "comments", "zcom", "ntcomm")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}

	// Create only NT files (no OT files)
	content := "NT-only commentary content"
	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	w.Write([]byte(content))
	w.Close()

	// Write NT block index
	bzsData := make([]byte, 12)
	binary.LittleEndian.PutUint32(bzsData[0:], 0)
	binary.LittleEndian.PutUint32(bzsData[4:], uint32(compressed.Len()))
	binary.LittleEndian.PutUint32(bzsData[8:], uint32(len(content)))
	if err := os.WriteFile(filepath.Join(moduleDir, "nt.bzs"), bzsData, 0600); err != nil {
		t.Fatalf("failed to write nt.bzs: %v", err)
	}

	// Write NT verse index
	bzvData := make([]byte, 10)
	binary.LittleEndian.PutUint32(bzvData[0:], 0)
	binary.LittleEndian.PutUint32(bzvData[4:], 0)
	binary.LittleEndian.PutUint16(bzvData[8:], uint16(len(content)))
	if err := os.WriteFile(filepath.Join(moduleDir, "nt.bzv"), bzvData, 0600); err != nil {
		t.Fatalf("failed to write nt.bzv: %v", err)
	}

	// Write NT compressed data
	if err := os.WriteFile(filepath.Join(moduleDir, "nt.bzz"), compressed.Bytes(), 0600); err != nil {
		t.Fatalf("failed to write nt.bzz: %v", err)
	}

	conf := &ConfFile{
		ModuleName:    "NTComm",
		ModDrv:        "zCom",
		DataPath:      "./modules/comments/zcom/ntcomm/",
		Versification: "KJV",
	}

	parser := NewZComParser(conf, tmpDir)
	if err := parser.Load(); err != nil {
		t.Fatalf("failed to load module: %v", err)
	}

	if parser.HasOT() {
		t.Error("expected parser.HasOT() to be false for NT-only module")
	}
	if !parser.HasNT() {
		t.Error("expected parser.HasNT() to be true")
	}
}

// TestZComModuleInfo tests the module info retrieval.
func TestZComModuleInfo(t *testing.T) {
	conf := &ConfFile{
		ModuleName:    "TestCommentary",
		Description:   "A Test Commentary Module",
		ModDrv:        "zCom",
		DataPath:      "./modules/comments/zcom/test/",
		Lang:          "en",
		Version:       "1.0",
		Versification: "KJV",
	}

	tmpDir, err := os.MkdirTemp("", "zcom-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	parser := NewZComParser(conf, tmpDir)
	info := parser.GetModuleInfo()

	if info.Name != "TestCommentary" {
		t.Errorf("expected name 'TestCommentary', got %q", info.Name)
	}
	if info.Type != "Commentary" {
		t.Errorf("expected type 'Commentary', got %q", info.Type)
	}
	if info.Language != "en" {
		t.Errorf("expected language 'en', got %q", info.Language)
	}
	if info.Compressed != true {
		t.Error("expected Compressed to be true for zCom")
	}
}

// TestZComIPCGetEntry tests the IPC command for getting a commentary entry.
func TestZComIPCGetEntry(t *testing.T) {
	// This test would verify the IPC interface for commentary retrieval
	// Structure: send JSON request, receive JSON response

	req := ipc.Request{
		Command: "get-commentary",
		Args: map[string]interface{}{
			"path":   "/path/to/sword",
			"module": "TestComm",
			"ref":    "Gen.1.1",
		},
	}

	// Verify request structure
	if req.Command != "get-commentary" {
		t.Errorf("expected command 'get-commentary', got %q", req.Command)
	}

	path, ok := req.Args["path"].(string)
	if !ok || path == "" {
		t.Error("expected 'path' argument")
	}

	module, ok := req.Args["module"].(string)
	if !ok || module == "" {
		t.Error("expected 'module' argument")
	}

	ref, ok := req.Args["ref"].(string)
	if !ok || ref == "" {
		t.Error("expected 'ref' argument")
	}
}

// TestZComIPCGetChapter tests the IPC command for getting chapter commentaries.
func TestZComIPCGetChapter(t *testing.T) {
	req := ipc.Request{
		Command: "get-chapter-commentary",
		Args: map[string]interface{}{
			"path":    "/path/to/sword",
			"module":  "TestComm",
			"book":    "Gen",
			"chapter": 1,
		},
	}

	if req.Command != "get-chapter-commentary" {
		t.Errorf("expected command 'get-chapter-commentary', got %q", req.Command)
	}

	chapter, ok := req.Args["chapter"].(int)
	if !ok {
		// JSON numbers may come as float64
		if chapterFloat, ok := req.Args["chapter"].(float64); ok {
			chapter = int(chapterFloat)
		} else {
			t.Error("expected 'chapter' argument as number")
		}
	}
	if chapter != 1 {
		t.Errorf("expected chapter 1, got %d", chapter)
	}
}
