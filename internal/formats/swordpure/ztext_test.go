package swordpure

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// createMockZTextModule creates a minimal zText module for testing
func createMockZTextModule(t *testing.T, tmpDir string) (*ConfFile, string) {
	t.Helper()

	// Create module structure
	modsDir := filepath.Join(tmpDir, "mods.d")
	dataDir := filepath.Join(tmpDir, "modules", "texts", "ztext", "testmod")

	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	// Create conf file
	confContent := `[TestMod]
DataPath=./modules/texts/ztext/testmod/
ModDrv=zText
Encoding=UTF-8
Lang=en
Version=1.0
Description=Test Bible Module
CompressType=ZIP
Versification=KJV
`
	confPath := filepath.Join(modsDir, "testmod.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	// Parse conf
	conf, err := ParseConfFile(confPath)
	if err != nil {
		t.Fatalf("failed to parse conf: %v", err)
	}

	// Create minimal NT data (just Genesis 1:1 in OT position for simplicity)
	// We'll create a single verse at index 2 (after empty and module header)
	verseText := "In the beginning God created the heaven and the earth."

	// Create compressed block
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	zw.Write([]byte(verseText))
	zw.Close()
	compressed := buf.Bytes()

	// Write .bzz (compressed data)
	bzzPath := filepath.Join(dataDir, "ot.bzz")
	if err := os.WriteFile(bzzPath, compressed, 0644); err != nil {
		t.Fatalf("failed to write bzz: %v", err)
	}

	// Write .bzs (block index)
	// Single block entry: offset=0, compressedSize=len(compressed), uncompSize=len(verseText)
	bzsData := make([]byte, 12)
	binary.LittleEndian.PutUint32(bzsData[0:4], 0) // offset
	binary.LittleEndian.PutUint32(bzsData[4:8], uint32(len(compressed))) // compressed size
	binary.LittleEndian.PutUint32(bzsData[8:12], uint32(len(verseText))) // uncompressed size

	bzsPath := filepath.Join(dataDir, "ot.bzs")
	if err := os.WriteFile(bzsPath, bzsData, 0644); err != nil {
		t.Fatalf("failed to write bzs: %v", err)
	}

	// Write .bzv (verse index)
	// We need entries for: [0]=empty, [1]=module header, [2]=book intro, [3]=chapter heading, [4]=verse
	// Create 5 entries total
	numEntries := 5
	bzvData := make([]byte, numEntries*10)

	// [0] = empty
	binary.LittleEndian.PutUint32(bzvData[0:4], 0)   // blockNum
	binary.LittleEndian.PutUint32(bzvData[4:8], 0)   // offset
	binary.LittleEndian.PutUint16(bzvData[8:10], 0)  // size

	// [1] = module header (empty)
	binary.LittleEndian.PutUint32(bzvData[10:14], 0)
	binary.LittleEndian.PutUint32(bzvData[14:18], 0)
	binary.LittleEndian.PutUint16(bzvData[18:20], 0)

	// [2] = book intro (empty)
	binary.LittleEndian.PutUint32(bzvData[20:24], 0)
	binary.LittleEndian.PutUint32(bzvData[24:28], 0)
	binary.LittleEndian.PutUint16(bzvData[28:30], 0)

	// [3] = chapter heading (empty)
	binary.LittleEndian.PutUint32(bzvData[30:34], 0)
	binary.LittleEndian.PutUint32(bzvData[34:38], 0)
	binary.LittleEndian.PutUint16(bzvData[38:40], 0)

	// [4] = actual verse (Genesis 1:1)
	binary.LittleEndian.PutUint32(bzvData[40:44], 0)                    // blockNum=0
	binary.LittleEndian.PutUint32(bzvData[44:48], 0)                    // offset=0
	binary.LittleEndian.PutUint16(bzvData[48:50], uint16(len(verseText))) // size

	bzvPath := filepath.Join(dataDir, "ot.bzv")
	if err := os.WriteFile(bzvPath, bzvData, 0644); err != nil {
		t.Fatalf("failed to write bzv: %v", err)
	}

	return conf, tmpDir
}

func TestOpenZTextModule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZTextModule(t, tmpDir)

	mod, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		t.Fatalf("OpenZTextModule failed: %v", err)
	}

	if mod == nil {
		t.Fatal("OpenZTextModule returned nil module")
	}

	if !mod.HasOT() {
		t.Error("module should have OT data")
	}

	if mod.HasNT() {
		t.Error("module should not have NT data")
	}
}

func TestOpenZTextModuleNonExistent(t *testing.T) {
	conf := &ConfFile{
		ModuleName: "NonExistent",
		DataPath:   "./nonexistent/path/",
		ModDrv:     "zText",
	}

	mod, err := OpenZTextModule(conf, "/tmp")
	// OpenZTextModule doesn't fail if files don't exist, it just returns empty module
	if err != nil {
		t.Errorf("OpenZTextModule unexpectedly failed: %v", err)
	}
	if mod == nil {
		t.Error("OpenZTextModule returned nil module")
	}
	// Module should have no data
	if mod.HasOT() || mod.HasNT() {
		t.Error("non-existent module should have no data")
	}
}

func TestGetVerseText(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZTextModule(t, tmpDir)
	mod, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		t.Fatalf("OpenZTextModule failed: %v", err)
	}

	// Try to get Genesis 1:1
	ref := &Ref{
		Book:    "Gen",
		Chapter: 1,
		Verse:   1,
	}

	text, err := mod.GetVerseText(ref)
	if err != nil {
		t.Fatalf("GetVerseText failed: %v", err)
	}

	expected := "In the beginning God created the heaven and the earth."
	if text != expected {
		t.Errorf("GetVerseText = %q, want %q", text, expected)
	}
}

func TestGetVerseTextInvalidBook(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZTextModule(t, tmpDir)
	mod, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		t.Fatalf("OpenZTextModule failed: %v", err)
	}

	ref := &Ref{
		Book:    "InvalidBook",
		Chapter: 1,
		Verse:   1,
	}

	_, err = mod.GetVerseText(ref)
	if err == nil {
		t.Error("GetVerseText should fail for invalid book")
	}
}

func TestReadBlockIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test .bzs file with 2 entries
	bzsPath := filepath.Join(tmpDir, "test.bzs")
	bzsData := make([]byte, 24) // 2 entries * 12 bytes

	// Entry 0
	binary.LittleEndian.PutUint32(bzsData[0:4], 0)
	binary.LittleEndian.PutUint32(bzsData[4:8], 100)
	binary.LittleEndian.PutUint32(bzsData[8:12], 200)

	// Entry 1
	binary.LittleEndian.PutUint32(bzsData[12:16], 100)
	binary.LittleEndian.PutUint32(bzsData[16:20], 150)
	binary.LittleEndian.PutUint32(bzsData[20:24], 250)

	if err := os.WriteFile(bzsPath, bzsData, 0644); err != nil {
		t.Fatalf("failed to write test bzs: %v", err)
	}

	entries, err := readBlockIndex(bzsPath)
	if err != nil {
		t.Fatalf("readBlockIndex failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("readBlockIndex returned %d entries, want 2", len(entries))
	}

	// Check entry 0
	if entries[0].Offset != 0 || entries[0].CompressedSize != 100 || entries[0].UncompSize != 200 {
		t.Errorf("entry[0] = %+v, want {Offset:0, CompressedSize:100, UncompSize:200}", entries[0])
	}

	// Check entry 1
	if entries[1].Offset != 100 || entries[1].CompressedSize != 150 || entries[1].UncompSize != 250 {
		t.Errorf("entry[1] = %+v, want {Offset:100, CompressedSize:150, UncompSize:250}", entries[1])
	}
}

func TestReadBlockIndexInvalidSize(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test .bzs file with invalid size (not multiple of 12)
	bzsPath := filepath.Join(tmpDir, "test.bzs")
	bzsData := make([]byte, 13) // Invalid size

	if err := os.WriteFile(bzsPath, bzsData, 0644); err != nil {
		t.Fatalf("failed to write test bzs: %v", err)
	}

	_, err = readBlockIndex(bzsPath)
	if err == nil {
		t.Error("readBlockIndex should fail for invalid size")
	}
}

func TestReadVerseIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test .bzv file with 2 entries
	bzvPath := filepath.Join(tmpDir, "test.bzv")
	bzvData := make([]byte, 20) // 2 entries * 10 bytes

	// Entry 0
	binary.LittleEndian.PutUint32(bzvData[0:4], 0)
	binary.LittleEndian.PutUint32(bzvData[4:8], 0)
	binary.LittleEndian.PutUint16(bzvData[8:10], 50)

	// Entry 1
	binary.LittleEndian.PutUint32(bzvData[10:14], 1)
	binary.LittleEndian.PutUint32(bzvData[14:18], 100)
	binary.LittleEndian.PutUint16(bzvData[18:20], 75)

	if err := os.WriteFile(bzvPath, bzvData, 0644); err != nil {
		t.Fatalf("failed to write test bzv: %v", err)
	}

	entries, err := readVerseIndex(bzvPath)
	if err != nil {
		t.Fatalf("readVerseIndex failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("readVerseIndex returned %d entries, want 2", len(entries))
	}

	// Check entry 0
	if entries[0].BlockNum != 0 || entries[0].Offset != 0 || entries[0].Size != 50 {
		t.Errorf("entry[0] = %+v, want {BlockNum:0, Offset:0, Size:50}", entries[0])
	}

	// Check entry 1
	if entries[1].BlockNum != 1 || entries[1].Offset != 100 || entries[1].Size != 75 {
		t.Errorf("entry[1] = %+v, want {BlockNum:1, Offset:100, Size:75}", entries[1])
	}
}

func TestReadVerseIndexInvalidSize(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test .bzv file with invalid size (not multiple of 10)
	bzvPath := filepath.Join(tmpDir, "test.bzv")
	bzvData := make([]byte, 11) // Invalid size

	if err := os.WriteFile(bzvPath, bzvData, 0644); err != nil {
		t.Fatalf("failed to write test bzv: %v", err)
	}

	_, err = readVerseIndex(bzvPath)
	if err == nil {
		t.Error("readVerseIndex should fail for invalid size")
	}
}

func TestGetModuleInfo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZTextModule(t, tmpDir)
	mod, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		t.Fatalf("OpenZTextModule failed: %v", err)
	}

	info := mod.GetModuleInfo()

	if info.Name != "TestMod" {
		t.Errorf("Name = %q, want %q", info.Name, "TestMod")
	}
	if info.Type != "Bible" {
		t.Errorf("Type = %q, want %q", info.Type, "Bible")
	}
	if !info.Compressed {
		t.Error("Compressed should be true")
	}
	if info.Encrypted {
		t.Error("Encrypted should be false")
	}
}

// createMockZTextModuleNT creates a minimal zText module with NT data
func createMockZTextModuleNT(t *testing.T, tmpDir string) (*ConfFile, string) {
	t.Helper()

	modsDir := filepath.Join(tmpDir, "mods.d")
	dataDir := filepath.Join(tmpDir, "modules", "texts", "ztext", "ntmod")

	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	confContent := `[NTMod]
DataPath=./modules/texts/ztext/ntmod/
ModDrv=zText
Lang=en
Description=NT Bible Module
CompressType=ZIP
`
	confPath := filepath.Join(modsDir, "ntmod.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	conf, err := ParseConfFile(confPath)
	if err != nil {
		t.Fatalf("failed to parse conf: %v", err)
	}

	// Create NT data (Matthew 1:1)
	verseText := "The book of the generation of Jesus Christ, the son of David, the son of Abraham."

	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	zw.Write([]byte(verseText))
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
	binary.LittleEndian.PutUint32(bzsData[8:12], uint32(len(verseText)))

	bzsPath := filepath.Join(dataDir, "nt.bzs")
	if err := os.WriteFile(bzsPath, bzsData, 0644); err != nil {
		t.Fatalf("failed to write bzs: %v", err)
	}

	// Write NT .bzv (5 entries for Matthew 1:1)
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
	binary.LittleEndian.PutUint16(bzvData[48:50], uint16(len(verseText)))

	bzvPath := filepath.Join(dataDir, "nt.bzv")
	if err := os.WriteFile(bzvPath, bzvData, 0644); err != nil {
		t.Fatalf("failed to write bzv: %v", err)
	}

	return conf, tmpDir
}

func TestOpenZTextModuleNT(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZTextModuleNT(t, tmpDir)
	mod, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		t.Fatalf("OpenZTextModule failed: %v", err)
	}

	if mod.HasOT() {
		t.Error("module should not have OT data")
	}

	if !mod.HasNT() {
		t.Error("module should have NT data")
	}
}

func TestGetVerseTextNT(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	conf, swordPath := createMockZTextModuleNT(t, tmpDir)
	mod, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		t.Fatalf("OpenZTextModule failed: %v", err)
	}

	ref := &Ref{Book: "Matt", Chapter: 1, Verse: 1}
	text, err := mod.GetVerseText(ref)
	if err != nil {
		t.Fatalf("GetVerseText failed: %v", err)
	}

	expected := "The book of the generation of Jesus Christ, the son of David, the son of Abraham."
	if text != expected {
		t.Errorf("GetVerseText = %q, want %q", text, expected)
	}
}

func TestGetVerseTextNoData(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create OT-only module
	conf, swordPath := createMockZTextModule(t, tmpDir)
	mod, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		t.Fatalf("OpenZTextModule failed: %v", err)
	}

	// Try to get NT verse when no NT data exists
	ref := &Ref{Book: "Matt", Chapter: 1, Verse: 1}
	_, err = mod.GetVerseText(ref)
	if err == nil {
		t.Error("GetVerseText should fail when no data for testament")
	}
}

func TestOpenZTextModuleWithAbsolutePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	// Create conf with absolute DataPath
	conf := &ConfFile{
		ModuleName: "AbsMod",
		DataPath:   dataDir, // Absolute path
		ModDrv:     "zText",
	}

	mod, err := OpenZTextModule(conf, tmpDir)
	if err != nil {
		t.Fatalf("OpenZTextModule failed: %v", err)
	}

	if mod == nil {
		t.Error("OpenZTextModule returned nil")
	}
}

func TestOpenZTextModuleBadBzsPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "modules", "texts", "ztext", "badmod")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	// Create valid .bzs but invalid .bzv (will fail during readVerseIndex)
	bzsData := make([]byte, 12)
	binary.LittleEndian.PutUint32(bzsData[0:4], 0)
	binary.LittleEndian.PutUint32(bzsData[4:8], 100)
	binary.LittleEndian.PutUint32(bzsData[8:12], 200)

	if err := os.WriteFile(filepath.Join(dataDir, "ot.bzs"), bzsData, 0644); err != nil {
		t.Fatalf("failed to write bzs: %v", err)
	}

	// Create truncated .bzv (will fail)
	if err := os.WriteFile(filepath.Join(dataDir, "ot.bzv"), []byte{1, 2, 3}, 0644); err != nil {
		t.Fatalf("failed to write bzv: %v", err)
	}

	conf := &ConfFile{
		ModuleName: "BadMod",
		DataPath:   "./modules/texts/ztext/badmod/",
		ModDrv:     "zText",
	}

	_, err = OpenZTextModule(conf, tmpDir)
	if err == nil {
		t.Error("OpenZTextModule should fail for invalid bzv file")
	}
}

func TestReadBlockIndexNonExistent(t *testing.T) {
	_, err := readBlockIndex("/nonexistent/path/test.bzs")
	if err == nil {
		t.Error("readBlockIndex should fail for non-existent file")
	}
}

func TestReadVerseIndexNonExistent(t *testing.T) {
	_, err := readVerseIndex("/nonexistent/path/test.bzv")
	if err == nil {
		t.Error("readVerseIndex should fail for non-existent file")
	}
}
