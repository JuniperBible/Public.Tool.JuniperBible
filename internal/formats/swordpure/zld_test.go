package swordpure

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"
)

func TestNewZLDParser(t *testing.T) {
	parser, err := NewZLDParser("/test/path")
	if err != nil {
		t.Fatalf("NewZLDParser failed: %v", err)
	}

	if parser == nil {
		t.Fatal("NewZLDParser returned nil")
	}

	if parser.modulePath != "/test/path" {
		t.Errorf("modulePath = %q, want %q", parser.modulePath, "/test/path")
	}

	if parser.entries == nil {
		t.Error("entries map should be initialized")
	}
}

func TestParseZLDKeyIndex(t *testing.T) {
	// Create test index data
	var buf bytes.Buffer

	// Entry 1: offset=100, key="G2316"
	binary.Write(&buf, binary.BigEndian, uint32(100))
	buf.WriteString("G2316")
	buf.WriteByte(0) // null terminator

	// Entry 2: offset=200, key="G2424"
	binary.Write(&buf, binary.BigEndian, uint32(200))
	buf.WriteString("G2424")
	buf.WriteByte(0)

	entries, err := parseZLDKeyIndex(buf.Bytes())
	if err != nil {
		t.Fatalf("parseZLDKeyIndex failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("parseZLDKeyIndex returned %d entries, want 2", len(entries))
	}

	// Check entry 1
	if entries[0].Key != "G2316" {
		t.Errorf("entries[0].Key = %q, want %q", entries[0].Key, "G2316")
	}
	if entries[0].Offset != 100 {
		t.Errorf("entries[0].Offset = %d, want 100", entries[0].Offset)
	}

	// Check entry 2
	if entries[1].Key != "G2424" {
		t.Errorf("entries[1].Key = %q, want %q", entries[1].Key, "G2424")
	}
	if entries[1].Offset != 200 {
		t.Errorf("entries[1].Offset = %d, want 200", entries[1].Offset)
	}
}

func TestParseZLDKeyIndexEmpty(t *testing.T) {
	entries, err := parseZLDKeyIndex([]byte{})
	if err != nil {
		t.Fatalf("parseZLDKeyIndex failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("parseZLDKeyIndex returned %d entries, want 0", len(entries))
	}
}

func TestParseZLDKeyIndexTruncated(t *testing.T) {
	// Only 3 bytes (incomplete offset)
	data := []byte{0x00, 0x00, 0x00}

	entries, err := parseZLDKeyIndex(data)
	if err != nil {
		t.Fatalf("parseZLDKeyIndex failed: %v", err)
	}

	// Should return empty list for truncated data
	if len(entries) != 0 {
		t.Errorf("parseZLDKeyIndex should return 0 entries for truncated data, got %d", len(entries))
	}
}

func TestParseZLDCompressedIndex(t *testing.T) {
	// Create test compressed index data (2 entries * 8 bytes)
	data := make([]byte, 16)

	// Entry 0: blockNum=0, offset=0
	binary.LittleEndian.PutUint32(data[0:4], 0)
	binary.LittleEndian.PutUint32(data[4:8], 0)

	// Entry 1: blockNum=1, offset=100
	binary.LittleEndian.PutUint32(data[8:12], 1)
	binary.LittleEndian.PutUint32(data[12:16], 100)

	entries, err := parseZLDCompressedIndex(data)
	if err != nil {
		t.Fatalf("parseZLDCompressedIndex failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("parseZLDCompressedIndex returned %d entries, want 2", len(entries))
	}

	// Check entry 0
	if entries[0].BlockNum != 0 || entries[0].Offset != 0 {
		t.Errorf("entries[0] = %+v, want {BlockNum:0, Offset:0}", entries[0])
	}

	// Check entry 1
	if entries[1].BlockNum != 1 || entries[1].Offset != 100 {
		t.Errorf("entries[1] = %+v, want {BlockNum:1, Offset:100}", entries[1])
	}
}

func TestParseZLDCompressedIndexInvalidSize(t *testing.T) {
	// Invalid size (not multiple of 8)
	data := make([]byte, 9)

	_, err := parseZLDCompressedIndex(data)
	if err == nil {
		t.Error("parseZLDCompressedIndex should fail for invalid size")
	}
}

func TestDecompressZLDBlock(t *testing.T) {
	// Create test data
	testData := []byte("This is test lexicon entry data")

	// Compress with zlib
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	zw.Write(testData)
	zw.Close()

	// Create block data: 4-byte size + compressed data
	var blockData bytes.Buffer
	binary.Write(&blockData, binary.LittleEndian, uint32(compressed.Len()))
	blockData.Write(compressed.Bytes())

	// Decompress
	decompressed, err := decompressZLDBlock(blockData.Bytes())
	if err != nil {
		t.Fatalf("decompressZLDBlock failed: %v", err)
	}

	if string(decompressed) != string(testData) {
		t.Errorf("decompressed = %q, want %q", string(decompressed), string(testData))
	}
}

func TestDecompressZLDBlockTooShort(t *testing.T) {
	// Block data too short (less than 4 bytes)
	data := []byte{0x00, 0x00}

	_, err := decompressZLDBlock(data)
	if err == nil {
		t.Error("decompressZLDBlock should fail for too short data")
	}
}

func TestDecompressZLDBlockTruncated(t *testing.T) {
	// Create block with size larger than actual data
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[0:4], 100) // size=100, but only 4 bytes follow

	_, err := decompressZLDBlock(data)
	if err == nil {
		t.Error("decompressZLDBlock should fail for truncated data")
	}
}

func TestZLDParserGetEntry(t *testing.T) {
	parser, err := NewZLDParser("/test/path")
	if err != nil {
		t.Fatalf("NewZLDParser failed: %v", err)
	}

	// Add test entry
	parser.entries["G2316"] = &ZLDEntry{
		Key:        "G2316",
		Definition: "God, deity",
		Offset:     100,
		Size:       50,
	}

	entry, err := parser.GetEntry("G2316")
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	if entry.Key != "G2316" {
		t.Errorf("entry.Key = %q, want %q", entry.Key, "G2316")
	}

	if entry.Definition != "God, deity" {
		t.Errorf("entry.Definition = %q, want %q", entry.Definition, "God, deity")
	}
}

func TestZLDParserGetEntryNotFound(t *testing.T) {
	parser, err := NewZLDParser("/test/path")
	if err != nil {
		t.Fatalf("NewZLDParser failed: %v", err)
	}

	_, err = parser.GetEntry("NonExistent")
	if err == nil {
		t.Error("GetEntry should fail for non-existent key")
	}
}

func TestZLDParserListKeys(t *testing.T) {
	parser, err := NewZLDParser("/test/path")
	if err != nil {
		t.Fatalf("NewZLDParser failed: %v", err)
	}

	// Add test entries
	parser.entries["G2316"] = &ZLDEntry{Key: "G2316"}
	parser.entries["G2424"] = &ZLDEntry{Key: "G2424"}
	parser.entries["H430"] = &ZLDEntry{Key: "H430"}

	keys := parser.ListKeys()

	if len(keys) != 3 {
		t.Fatalf("ListKeys returned %d keys, want 3", len(keys))
	}

	// Check that all keys are present (order doesn't matter)
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}

	if !keyMap["G2316"] || !keyMap["G2424"] || !keyMap["H430"] {
		t.Error("ListKeys did not return all keys")
	}
}

func TestZLDParserListKeysEmpty(t *testing.T) {
	parser, err := NewZLDParser("/test/path")
	if err != nil {
		t.Fatalf("NewZLDParser failed: %v", err)
	}

	keys := parser.ListKeys()

	if len(keys) != 0 {
		t.Errorf("ListKeys returned %d keys, want 0", len(keys))
	}
}

func TestZLDParserSearchKeys(t *testing.T) {
	parser, err := NewZLDParser("/test/path")
	if err != nil {
		t.Fatalf("NewZLDParser failed: %v", err)
	}

	// Add test entries
	parser.entries["G2316"] = &ZLDEntry{Key: "G2316"}
	parser.entries["G2424"] = &ZLDEntry{Key: "G2424"}
	parser.entries["H430"] = &ZLDEntry{Key: "H430"}

	// Search for keys starting with "G"
	matches := parser.SearchKeys("G")

	if len(matches) != 2 {
		t.Fatalf("SearchKeys returned %d matches, want 2", len(matches))
	}

	// Check that both G entries are present
	keyMap := make(map[string]bool)
	for _, k := range matches {
		keyMap[k] = true
	}

	if !keyMap["G2316"] || !keyMap["G2424"] {
		t.Error("SearchKeys did not return correct matches")
	}

	if keyMap["H430"] {
		t.Error("SearchKeys should not return H430")
	}
}

func TestZLDParserSearchKeysNoMatches(t *testing.T) {
	parser, err := NewZLDParser("/test/path")
	if err != nil {
		t.Fatalf("NewZLDParser failed: %v", err)
	}

	parser.entries["G2316"] = &ZLDEntry{Key: "G2316"}

	matches := parser.SearchKeys("H")

	if len(matches) != 0 {
		t.Errorf("SearchKeys returned %d matches, want 0", len(matches))
	}
}

func TestZLDParserModuleInfo(t *testing.T) {
	parser, err := NewZLDParser("/test/path")
	if err != nil {
		t.Fatalf("NewZLDParser failed: %v", err)
	}

	// Set conf
	parser.conf = &Conf{
		ModuleName:  "TestLex",
		Description: "Test Lexicon",
		SourceType:  "Lexicon",
	}

	// Add entries
	parser.entries["G2316"] = &ZLDEntry{Key: "G2316"}
	parser.entries["G2424"] = &ZLDEntry{Key: "G2424"}

	info := parser.ModuleInfo()

	if info.Name != "TestLex" {
		t.Errorf("Name = %q, want %q", info.Name, "TestLex")
	}

	if info.Type != "Lexicon" {
		t.Errorf("Type = %q, want %q", info.Type, "Lexicon")
	}

	if info.EntryCount != 2 {
		t.Errorf("EntryCount = %d, want 2", info.EntryCount)
	}
}

func TestZLDParserModuleInfoNoConf(t *testing.T) {
	parser, err := NewZLDParser("/test/path")
	if err != nil {
		t.Fatalf("NewZLDParser failed: %v", err)
	}

	info := parser.ModuleInfo()

	if info.Name != "" {
		t.Errorf("Name should be empty when conf is nil, got %q", info.Name)
	}

	if info.EntryCount != 0 {
		t.Errorf("EntryCount = %d, want 0", info.EntryCount)
	}
}
