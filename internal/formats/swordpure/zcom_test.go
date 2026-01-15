package swordpure

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// createMockZComModule creates a minimal zCom commentary module for testing
func createMockZComModule(t *testing.T, tmpDir string) (*ConfFile, string) {
	t.Helper()

	// Create module structure
	modsDir := filepath.Join(tmpDir, "mods.d")
	dataDir := filepath.Join(tmpDir, "modules", "comments", "zcom", "testcom")

	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	// Create conf file
	confContent := `[TestCom]
DataPath=./modules/comments/zcom/testcom/
ModDrv=zCom
Encoding=UTF-8
Lang=en
Version=1.0
Description=Test Commentary Module
CompressType=ZIP
Versification=KJV
`
	confPath := filepath.Join(modsDir, "testcom.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	// Parse conf
	conf, err := ParseConfFile(confPath)
	if err != nil {
		t.Fatalf("failed to parse conf: %v", err)
	}

	// Create minimal OT commentary data (Genesis 1:1)
	commentaryText := "This verse describes the creation of the universe."

	// Create compressed block
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	zw.Write([]byte(commentaryText))
	zw.Close()
	compressed := buf.Bytes()

	// Write .bzz (compressed data)
	bzzPath := filepath.Join(dataDir, "ot.bzz")
	if err := os.WriteFile(bzzPath, compressed, 0644); err != nil {
		t.Fatalf("failed to write bzz: %v", err)
	}

	// Write .bzs (block index)
	bzsData := make([]byte, 12)
	binary.LittleEndian.PutUint32(bzsData[0:4], 0)
	binary.LittleEndian.PutUint32(bzsData[4:8], uint32(len(compressed)))
	binary.LittleEndian.PutUint32(bzsData[8:12], uint32(len(commentaryText)))

	bzsPath := filepath.Join(dataDir, "ot.bzs")
	if err := os.WriteFile(bzsPath, bzsData, 0644); err != nil {
		t.Fatalf("failed to write bzs: %v", err)
	}

	// Write .bzv (verse index)
	numEntries := 5
	bzvData := make([]byte, numEntries*10)

	// [0-3] = empty entries
	for i := 0; i < 4; i++ {
		offset := i * 10
		binary.LittleEndian.PutUint32(bzvData[offset:offset+4], 0)
		binary.LittleEndian.PutUint32(bzvData[offset+4:offset+8], 0)
		binary.LittleEndian.PutUint16(bzvData[offset+8:offset+10], 0)
	}

	// [4] = actual commentary entry (Genesis 1:1)
	binary.LittleEndian.PutUint32(bzvData[40:44], 0)
	binary.LittleEndian.PutUint32(bzvData[44:48], 0)
	binary.LittleEndian.PutUint16(bzvData[48:50], uint16(len(commentaryText)))

	bzvPath := filepath.Join(dataDir, "ot.bzv")
	if err := os.WriteFile(bzvPath, bzvData, 0644); err != nil {
		t.Fatalf("failed to write bzv: %v", err)
	}

	return conf, tmpDir
}

func TestNewZComParser(t *testing.T) {
	conf := &ConfFile{
		ModuleName: "TestCom",
		DataPath:   "./test/path/",
	}

	parser := NewZComParser(conf, "/sword/path")
	if parser == nil {
		t.Fatal("NewZComParser returned nil")
	}

	if parser.module != conf {
		t.Error("module not set correctly")
	}

	if parser.basePath != "/sword/path" {
		t.Errorf("basePath = %q, want %q", parser.basePath, "/sword/path")
	}

	if parser.loaded {
		t.Error("parser should not be loaded initially")
	}
}

func TestZComParserLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZComModule(t, tmpDir)
	parser := NewZComParser(conf, swordPath)

	if err := parser.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !parser.loaded {
		t.Error("parser should be loaded")
	}

	if !parser.HasOT() {
		t.Error("parser should have OT data")
	}

	if parser.HasNT() {
		t.Error("parser should not have NT data")
	}
}

func TestZComParserGetEntry(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZComModule(t, tmpDir)
	parser := NewZComParser(conf, swordPath)

	if err := parser.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	ref := Ref{
		Book:    "Gen",
		Chapter: 1,
		Verse:   1,
	}

	entry, err := parser.GetEntry(ref)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	if entry == nil {
		t.Fatal("GetEntry returned nil")
	}

	expected := "This verse describes the creation of the universe."
	if entry.Text != expected {
		t.Errorf("entry.Text = %q, want %q", entry.Text, expected)
	}

	if entry.Source != "TestCom" {
		t.Errorf("entry.Source = %q, want %q", entry.Source, "TestCom")
	}
}

func TestZComParserGetEntryNotLoaded(t *testing.T) {
	conf := &ConfFile{
		ModuleName: "TestCom",
		DataPath:   "./test/path/",
	}
	parser := NewZComParser(conf, "/sword/path")

	ref := Ref{Book: "Gen", Chapter: 1, Verse: 1}
	_, err := parser.GetEntry(ref)
	if err == nil {
		t.Error("GetEntry should fail when parser not loaded")
	}
}

func TestZComParserGetEntryInvalidBook(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZComModule(t, tmpDir)
	parser := NewZComParser(conf, swordPath)

	if err := parser.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	ref := Ref{Book: "InvalidBook", Chapter: 1, Verse: 1}
	_, err = parser.GetEntry(ref)
	if err == nil {
		t.Error("GetEntry should fail for invalid book")
	}
}

func TestZComParserGetChapterEntries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZComModule(t, tmpDir)
	parser := NewZComParser(conf, swordPath)

	if err := parser.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	entries, err := parser.GetChapterEntries("Gen", 1)
	if err != nil {
		t.Fatalf("GetChapterEntries failed: %v", err)
	}

	if entries == nil {
		t.Fatal("GetChapterEntries returned nil")
	}

	if entries.Book != "Gen" {
		t.Errorf("entries.Book = %q, want %q", entries.Book, "Gen")
	}

	if entries.Chapter != 1 {
		t.Errorf("entries.Chapter = %d, want %d", entries.Chapter, 1)
	}

	if len(entries.Entries) < 1 {
		t.Error("GetChapterEntries should return at least one entry")
	}
}

func TestZComParserGetChapterEntriesNotLoaded(t *testing.T) {
	conf := &ConfFile{
		ModuleName: "TestCom",
		DataPath:   "./test/path/",
	}
	parser := NewZComParser(conf, "/sword/path")

	_, err := parser.GetChapterEntries("Gen", 1)
	if err == nil {
		t.Error("GetChapterEntries should fail when parser not loaded")
	}
}

func TestZComParserGetBookEntries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZComModule(t, tmpDir)
	parser := NewZComParser(conf, swordPath)

	if err := parser.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	bookEntries, err := parser.GetBookEntries("Gen")
	if err != nil {
		t.Fatalf("GetBookEntries failed: %v", err)
	}

	if bookEntries == nil {
		t.Fatal("GetBookEntries returned nil")
	}

	if bookEntries.Book != "Gen" {
		t.Errorf("bookEntries.Book = %q, want %q", bookEntries.Book, "Gen")
	}

	if len(bookEntries.Chapters) == 0 {
		t.Error("GetBookEntries should return at least one chapter")
	}
}

func TestZComParserGetModuleInfo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZComModule(t, tmpDir)
	parser := NewZComParser(conf, swordPath)

	if err := parser.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	info := parser.GetModuleInfo()

	if info.Name != "TestCom" {
		t.Errorf("Name = %q, want %q", info.Name, "TestCom")
	}

	if info.Type != "Commentary" {
		t.Errorf("Type = %q, want %q", info.Type, "Commentary")
	}

	if !info.Compressed {
		t.Error("Compressed should be true")
	}
}

func TestReadBlock(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test .bzz file with compressed data
	testData := []byte("Test block data for decompression")
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	zw.Write(testData)
	zw.Close()
	compressed := buf.Bytes()

	bzzPath := filepath.Join(tmpDir, "test.bzz")
	if err := os.WriteFile(bzzPath, compressed, 0644); err != nil {
		t.Fatalf("failed to write bzz: %v", err)
	}

	block := BlockEntry{
		Offset:         0,
		CompressedSize: uint32(len(compressed)),
		UncompSize:     uint32(len(testData)),
	}

	decompressed, err := readBlock(bzzPath, block)
	if err != nil {
		t.Fatalf("readBlock failed: %v", err)
	}

	if string(decompressed) != string(testData) {
		t.Errorf("readBlock = %q, want %q", string(decompressed), string(testData))
	}
}

func TestReadBlockNonExistent(t *testing.T) {
	block := BlockEntry{
		Offset:         0,
		CompressedSize: 100,
		UncompSize:     200,
	}

	_, err := readBlock("/nonexistent/test.bzz", block)
	if err == nil {
		t.Error("readBlock should fail for non-existent file")
	}
}

func TestZComParserGetEntryNoData(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create module without NT data
	conf, swordPath := createMockZComModule(t, tmpDir)
	parser := NewZComParser(conf, swordPath)

	if err := parser.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Try to get an NT reference when there's no NT data
	ref := Ref{Book: "Matt", Chapter: 1, Verse: 1}
	_, err = parser.GetEntry(ref)
	if err == nil {
		t.Error("GetEntry should fail when no data for testament")
	}
}

func TestZComParserGetBookEntriesNotLoaded(t *testing.T) {
	conf := &ConfFile{
		ModuleName: "TestCom",
		DataPath:   "./test/path/",
	}
	parser := NewZComParser(conf, "/sword/path")

	_, err := parser.GetBookEntries("Gen")
	if err == nil {
		t.Error("GetBookEntries should fail when parser not loaded")
	}
}

func TestZComParserGetBookEntriesInvalidBook(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZComModule(t, tmpDir)
	parser := NewZComParser(conf, swordPath)

	if err := parser.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, err = parser.GetBookEntries("InvalidBook")
	if err == nil {
		t.Error("GetBookEntries should fail for invalid book")
	}
}

func TestZComParserGetChapterEntriesInvalidBook(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZComModule(t, tmpDir)
	parser := NewZComParser(conf, swordPath)

	if err := parser.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, err = parser.GetChapterEntries("InvalidBook", 1)
	if err == nil {
		t.Error("GetChapterEntries should fail for invalid book")
	}
}

func TestZComParserGetChapterEntriesInvalidChapter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZComModule(t, tmpDir)
	parser := NewZComParser(conf, swordPath)

	if err := parser.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, err = parser.GetChapterEntries("Gen", 999)
	if err == nil {
		t.Error("GetChapterEntries should fail for invalid chapter")
	}
}

func TestZComParserLoadAbsolutePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create data directory with absolute path
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	// Create conf with absolute DataPath
	conf := &ConfFile{
		ModuleName: "TestAbs",
		DataPath:   dataDir,
	}

	parser := NewZComParser(conf, tmpDir)
	err = parser.Load()
	// Should succeed (no files = empty but loaded)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !parser.loaded {
		t.Error("parser should be loaded")
	}
}

func TestZComParserHasOTNT(t *testing.T) {
	conf := &ConfFile{
		ModuleName: "TestCom",
		DataPath:   "./test/",
	}
	parser := NewZComParser(conf, "/sword/path")

	// Before loading
	if parser.HasOT() {
		t.Error("HasOT should return false before loading")
	}
	if parser.HasNT() {
		t.Error("HasNT should return false before loading")
	}
}

// createMockZComModuleNT creates a minimal zCom commentary module with NT data
func createMockZComModuleNT(t *testing.T, tmpDir string) (*ConfFile, string) {
	t.Helper()

	modsDir := filepath.Join(tmpDir, "mods.d")
	dataDir := filepath.Join(tmpDir, "modules", "comments", "zcom", "ntcom")

	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	confContent := `[NTCom]
DataPath=./modules/comments/zcom/ntcom/
ModDrv=zCom
Lang=en
Description=NT Commentary Module
CompressType=ZIP
`
	confPath := filepath.Join(modsDir, "ntcom.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	conf, err := ParseConfFile(confPath)
	if err != nil {
		t.Fatalf("failed to parse conf: %v", err)
	}

	// Create NT commentary data (Matthew 1:1)
	commentaryText := "The genealogy of Jesus Christ."

	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	zw.Write([]byte(commentaryText))
	zw.Close()
	compressed := buf.Bytes()

	// Write NT .bzz
	bzzPath := filepath.Join(dataDir, "nt.bzz")
	if err := os.WriteFile(bzzPath, compressed, 0644); err != nil {
		t.Fatalf("failed to write bzz: %v", err)
	}

	// Write NT .bzs
	bzsData := make([]byte, 12)
	binary.LittleEndian.PutUint32(bzsData[0:4], 0)
	binary.LittleEndian.PutUint32(bzsData[4:8], uint32(len(compressed)))
	binary.LittleEndian.PutUint32(bzsData[8:12], uint32(len(commentaryText)))

	bzsPath := filepath.Join(dataDir, "nt.bzs")
	if err := os.WriteFile(bzsPath, bzsData, 0644); err != nil {
		t.Fatalf("failed to write bzs: %v", err)
	}

	// Write NT .bzv (5 entries)
	numEntries := 5
	bzvData := make([]byte, numEntries*10)

	for i := 0; i < 4; i++ {
		offset := i * 10
		binary.LittleEndian.PutUint32(bzvData[offset:offset+4], 0)
		binary.LittleEndian.PutUint32(bzvData[offset+4:offset+8], 0)
		binary.LittleEndian.PutUint16(bzvData[offset+8:offset+10], 0)
	}

	binary.LittleEndian.PutUint32(bzvData[40:44], 0)
	binary.LittleEndian.PutUint32(bzvData[44:48], 0)
	binary.LittleEndian.PutUint16(bzvData[48:50], uint16(len(commentaryText)))

	bzvPath := filepath.Join(dataDir, "nt.bzv")
	if err := os.WriteFile(bzvPath, bzvData, 0644); err != nil {
		t.Fatalf("failed to write bzv: %v", err)
	}

	return conf, tmpDir
}

func TestZComParserLoadNT(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZComModuleNT(t, tmpDir)
	parser := NewZComParser(conf, swordPath)

	if err := parser.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if parser.HasOT() {
		t.Error("parser should not have OT data")
	}

	if !parser.HasNT() {
		t.Error("parser should have NT data")
	}
}

func TestZComParserGetEntryNT(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZComModuleNT(t, tmpDir)
	parser := NewZComParser(conf, swordPath)

	if err := parser.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	ref := Ref{Book: "Matt", Chapter: 1, Verse: 1}
	entry, err := parser.GetEntry(ref)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	if entry == nil {
		t.Fatal("GetEntry returned nil")
	}

	expected := "The genealogy of Jesus Christ."
	if entry.Text != expected {
		t.Errorf("entry.Text = %q, want %q", entry.Text, expected)
	}
}

func TestZComParserLoadBadBzsPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "modules", "comments", "zcom", "badmod")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	// Create a valid .bzs but invalid .bzv
	bzsData := make([]byte, 12)
	binary.LittleEndian.PutUint32(bzsData[0:4], 0)
	binary.LittleEndian.PutUint32(bzsData[4:8], 100)
	binary.LittleEndian.PutUint32(bzsData[8:12], 200)

	if err := os.WriteFile(filepath.Join(dataDir, "ot.bzs"), bzsData, 0644); err != nil {
		t.Fatalf("failed to write bzs: %v", err)
	}

	// Create invalid .bzv (truncated)
	if err := os.WriteFile(filepath.Join(dataDir, "ot.bzv"), []byte{1, 2, 3}, 0644); err != nil {
		t.Fatalf("failed to write bzv: %v", err)
	}

	conf := &ConfFile{
		ModuleName: "BadMod",
		DataPath:   "./modules/comments/zcom/badmod/",
	}

	parser := NewZComParser(conf, tmpDir)
	// This will fail because bzv is too short
	_ = parser.Load()
}

func TestReadBlockSeekError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a small file
	bzzPath := filepath.Join(tmpDir, "test.bzz")
	if err := os.WriteFile(bzzPath, []byte("small"), 0644); err != nil {
		t.Fatalf("failed to write bzz: %v", err)
	}

	// Try to read with offset beyond file size
	block := BlockEntry{
		Offset:         1000000, // Way beyond file size
		CompressedSize: 100,
		UncompSize:     200,
	}

	_, err = readBlock(bzzPath, block)
	if err == nil {
		t.Error("readBlock should fail when seeking beyond file")
	}
}

func TestReadBlockInvalidZlib(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create file with invalid zlib data
	bzzPath := filepath.Join(tmpDir, "test.bzz")
	if err := os.WriteFile(bzzPath, []byte("not valid zlib data here!"), 0644); err != nil {
		t.Fatalf("failed to write bzz: %v", err)
	}

	block := BlockEntry{
		Offset:         0,
		CompressedSize: 25,
		UncompSize:     100,
	}

	_, err = readBlock(bzzPath, block)
	if err == nil {
		t.Error("readBlock should fail for invalid zlib data")
	}
}
