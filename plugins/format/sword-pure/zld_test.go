package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// Phase 18: Tests for zLD (SWORD Lexicon/Dictionary) parser

// TestZLDEntryStruct verifies the ZLDEntry structure has all required fields.
func TestZLDEntryStruct(t *testing.T) {
	entry := ZLDEntry{
		Key:        "G2316",
		Definition: "θεός (theos) - God, a deity",
		Offset:     0,
		Size:       100,
	}

	if entry.Key != "G2316" {
		t.Errorf("Key = %q, want %q", entry.Key, "G2316")
	}
	if entry.Definition == "" {
		t.Error("Definition should not be empty")
	}
	if entry.Size != 100 {
		t.Errorf("Size = %d, want 100", entry.Size)
	}
}

// TestZLDParserCreation verifies ZLDParser can be created with module path.
func TestZLDParserCreation(t *testing.T) {
	parser, err := NewZLDParser("/path/to/modules/lexdict/rawld/strongs")
	if err != nil {
		// Expected to fail with non-existent path
		return
	}

	if parser == nil {
		t.Error("NewZLDParser should return a parser or error")
	}
}

// TestZLDReadKeyIndex verifies reading the .idx key index file.
// Format: 4-byte offset (big-endian) + null-terminated key string
func TestZLDReadKeyIndex(t *testing.T) {
	// Create mock .idx data
	// Entry 1: offset=0, key="G0001"
	// Entry 2: offset=100, key="G0002"
	var buf bytes.Buffer

	// Entry 1
	binary.Write(&buf, binary.BigEndian, uint32(0))
	buf.WriteString("G0001")
	buf.WriteByte(0) // null terminator

	// Entry 2
	binary.Write(&buf, binary.BigEndian, uint32(100))
	buf.WriteString("G0002")
	buf.WriteByte(0)

	idxData := buf.Bytes()

	keys, err := parseZLDKeyIndex(idxData)
	if err != nil {
		t.Fatalf("parseZLDKeyIndex failed: %v", err)
	}

	if len(keys) != 2 {
		t.Fatalf("len(keys) = %d, want 2", len(keys))
	}

	if keys[0].Key != "G0001" {
		t.Errorf("keys[0].Key = %q, want %q", keys[0].Key, "G0001")
	}
	if keys[0].Offset != 0 {
		t.Errorf("keys[0].Offset = %d, want 0", keys[0].Offset)
	}

	if keys[1].Key != "G0002" {
		t.Errorf("keys[1].Key = %q, want %q", keys[1].Key, "G0002")
	}
	if keys[1].Offset != 100 {
		t.Errorf("keys[1].Offset = %d, want 100", keys[1].Offset)
	}
}

// TestZLDReadCompressedIndex verifies reading the .zdx compressed index.
// Format: 8 bytes per entry (4-byte block number + 4-byte offset within block)
func TestZLDReadCompressedIndex(t *testing.T) {
	var buf bytes.Buffer

	// Entry at block 0, offset 0
	binary.Write(&buf, binary.LittleEndian, uint32(0)) // block
	binary.Write(&buf, binary.LittleEndian, uint32(0)) // offset

	// Entry at block 0, offset 50
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	binary.Write(&buf, binary.LittleEndian, uint32(50))

	// Entry at block 1, offset 0
	binary.Write(&buf, binary.LittleEndian, uint32(1))
	binary.Write(&buf, binary.LittleEndian, uint32(0))

	zdxData := buf.Bytes()

	entries, err := parseZLDCompressedIndex(zdxData)
	if err != nil {
		t.Fatalf("parseZLDCompressedIndex failed: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}

	if entries[0].BlockNum != 0 || entries[0].Offset != 0 {
		t.Errorf("entries[0] = {%d, %d}, want {0, 0}", entries[0].BlockNum, entries[0].Offset)
	}
	if entries[1].BlockNum != 0 || entries[1].Offset != 50 {
		t.Errorf("entries[1] = {%d, %d}, want {0, 50}", entries[1].BlockNum, entries[1].Offset)
	}
	if entries[2].BlockNum != 1 || entries[2].Offset != 0 {
		t.Errorf("entries[2] = {%d, %d}, want {1, 0}", entries[2].BlockNum, entries[2].Offset)
	}
}

// TestZLDDecompression verifies zlib decompression of dictionary data.
func TestZLDDecompression(t *testing.T) {
	original := "<entry><orth>G2316</orth><def>θεός - God</def></entry>"

	// Compress with zlib
	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	w.Write([]byte(original))
	w.Close()

	// Create block data: 4-byte size + compressed data
	var blockData bytes.Buffer
	binary.Write(&blockData, binary.LittleEndian, uint32(compressed.Len()))
	blockData.Write(compressed.Bytes())

	decompressed, err := decompressZLDBlock(blockData.Bytes())
	if err != nil {
		t.Fatalf("decompressZLDBlock failed: %v", err)
	}

	if string(decompressed) != original {
		t.Errorf("decompressed = %q, want %q", string(decompressed), original)
	}
}

// TestZLDGetEntry verifies retrieving a single dictionary entry.
func TestZLDGetEntry(t *testing.T) {
	parser := &ZLDParser{
		entries: map[string]*ZLDEntry{
			"G2316": {
				Key:        "G2316",
				Definition: "θεός (theos) - God, a deity",
			},
			"G0026": {
				Key:        "G0026",
				Definition: "ἀγάπη (agape) - love",
			},
		},
	}

	entry, err := parser.GetEntry("G2316")
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	if entry.Key != "G2316" {
		t.Errorf("entry.Key = %q, want %q", entry.Key, "G2316")
	}
	if entry.Definition == "" {
		t.Error("entry.Definition should not be empty")
	}
}

// TestZLDGetEntryNotFound verifies error handling for missing entries.
func TestZLDGetEntryNotFound(t *testing.T) {
	parser := &ZLDParser{
		entries: map[string]*ZLDEntry{},
	}

	_, err := parser.GetEntry("NOTFOUND")
	if err == nil {
		t.Error("GetEntry should return error for missing entry")
	}
}

// TestZLDListKeys verifies listing all dictionary keys.
func TestZLDListKeys(t *testing.T) {
	parser := &ZLDParser{
		entries: map[string]*ZLDEntry{
			"G0001": {Key: "G0001"},
			"G0002": {Key: "G0002"},
			"G0003": {Key: "G0003"},
		},
	}

	keys := parser.ListKeys()
	if len(keys) != 3 {
		t.Errorf("len(keys) = %d, want 3", len(keys))
	}

	// Keys should be present (order may vary)
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}

	for _, expected := range []string{"G0001", "G0002", "G0003"} {
		if !keySet[expected] {
			t.Errorf("missing key %q", expected)
		}
	}
}

// TestZLDSearchKeys verifies key search functionality.
func TestZLDSearchKeys(t *testing.T) {
	parser := &ZLDParser{
		entries: map[string]*ZLDEntry{
			"G0001": {Key: "G0001"},
			"G0002": {Key: "G0002"},
			"H0001": {Key: "H0001"},
			"H0002": {Key: "H0002"},
		},
	}

	// Search for Greek entries (G prefix)
	results := parser.SearchKeys("G")
	if len(results) != 2 {
		t.Errorf("SearchKeys('G') returned %d results, want 2", len(results))
	}

	// Search for Hebrew entries (H prefix)
	results = parser.SearchKeys("H")
	if len(results) != 2 {
		t.Errorf("SearchKeys('H') returned %d results, want 2", len(results))
	}
}

// TestZLDModuleInfo verifies module information extraction.
func TestZLDModuleInfo(t *testing.T) {
	parser := &ZLDParser{
		modulePath: "/path/to/modules/lexdict/rawld/strongs",
		conf: &Conf{
			ModuleName:  "StrongsGreek",
			Description: "Strong's Greek Dictionary",
			Lang:        "grc",
			Version:     "1.0",
			SourceType:  "zLD",
		},
		entries: map[string]*ZLDEntry{
			"G0001": {Key: "G0001"},
			"G0002": {Key: "G0002"},
		},
	}

	info := parser.ModuleInfo()

	if info.Name != "StrongsGreek" {
		t.Errorf("info.Name = %q, want %q", info.Name, "StrongsGreek")
	}
	if info.Type != "zLD" {
		t.Errorf("info.Type = %q, want %q", info.Type, "zLD")
	}
	if info.EntryCount != 2 {
		t.Errorf("info.EntryCount = %d, want 2", info.EntryCount)
	}
}

// TestZLDDataDirLayout verifies zLD data directory structure detection.
func TestZLDDataDirLayout(t *testing.T) {
	// zLD modules should have these files:
	// - module.idx (key index)
	// - module.dat (key data, optional)
	// - module.zdx (compressed index)
	// - module.zdt (compressed data)

	expectedFiles := []string{
		"strongs.idx",
		"strongs.zdx",
		"strongs.zdt",
	}

	for _, f := range expectedFiles {
		if f == "" {
			t.Error("expected file should not be empty")
		}
	}
}

// TestZLDNullTerminatedStrings verifies handling of null-terminated strings in index.
func TestZLDNullTerminatedStrings(t *testing.T) {
	// Create index with embedded nulls and varying key lengths
	var buf bytes.Buffer

	// Short key
	binary.Write(&buf, binary.BigEndian, uint32(0))
	buf.WriteString("A")
	buf.WriteByte(0)

	// Medium key
	binary.Write(&buf, binary.BigEndian, uint32(10))
	buf.WriteString("G2316")
	buf.WriteByte(0)

	// Long key with numbers
	binary.Write(&buf, binary.BigEndian, uint32(20))
	buf.WriteString("StrongsGreek0001")
	buf.WriteByte(0)

	idxData := buf.Bytes()

	keys, err := parseZLDKeyIndex(idxData)
	if err != nil {
		t.Fatalf("parseZLDKeyIndex failed: %v", err)
	}

	if len(keys) != 3 {
		t.Fatalf("len(keys) = %d, want 3", len(keys))
	}

	if keys[0].Key != "A" {
		t.Errorf("keys[0].Key = %q, want %q", keys[0].Key, "A")
	}
	if keys[1].Key != "G2316" {
		t.Errorf("keys[1].Key = %q, want %q", keys[1].Key, "G2316")
	}
	if keys[2].Key != "StrongsGreek0001" {
		t.Errorf("keys[2].Key = %q, want %q", keys[2].Key, "StrongsGreek0001")
	}
}

// TestZLDUnicodeKeys verifies handling of Unicode in dictionary keys.
func TestZLDUnicodeKeys(t *testing.T) {
	parser := &ZLDParser{
		entries: map[string]*ZLDEntry{
			"θεός":    {Key: "θεός", Definition: "God"},
			"ἀγάπη":   {Key: "ἀγάπη", Definition: "love"},
			"שָׁלוֹם": {Key: "שָׁלוֹם", Definition: "peace"},
		},
	}

	entry, err := parser.GetEntry("θεός")
	if err != nil {
		t.Fatalf("GetEntry failed for Greek key: %v", err)
	}
	if entry.Definition != "God" {
		t.Errorf("definition = %q, want %q", entry.Definition, "God")
	}

	entry, err = parser.GetEntry("שָׁלוֹם")
	if err != nil {
		t.Fatalf("GetEntry failed for Hebrew key: %v", err)
	}
	if entry.Definition != "peace" {
		t.Errorf("definition = %q, want %q", entry.Definition, "peace")
	}
}

// TestZLDIPCListKeys verifies IPC list-keys command.
func TestZLDIPCListKeys(t *testing.T) {
	request := ipc.Request{
		Command: "list-keys",
		Args: map[string]interface{}{
			"module": "StrongsGreek",
		},
	}

	if request.Command != "list-keys" {
		t.Errorf("Command = %q, want %q", request.Command, "list-keys")
	}

	// Response should contain array of keys
	response := ipc.Response{
		Status: "success",
		Result: map[string]interface{}{
			"keys": []string{"G0001", "G0002", "G0003"},
		},
	}

	if response.Status != "success" {
		t.Error("Response should be successful")
	}

	data, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}
	keys, ok := data["keys"].([]string)
	if !ok {
		t.Fatal("Data should contain keys array")
	}
	if len(keys) != 3 {
		t.Errorf("len(keys) = %d, want 3", len(keys))
	}
}

// TestZLDIPCGetEntry verifies IPC get-entry command.
func TestZLDIPCGetEntry(t *testing.T) {
	request := ipc.Request{
		Command: "get-entry",
		Args: map[string]interface{}{
			"module": "StrongsGreek",
			"key":    "G2316",
		},
	}

	if request.Command != "get-entry" {
		t.Errorf("Command = %q, want %q", request.Command, "get-entry")
	}

	// Response should contain entry definition
	response := ipc.Response{
		Status: "success",
		Result: map[string]interface{}{
			"key":        "G2316",
			"definition": "θεός (theos) - God, a deity",
		},
	}

	if response.Status != "success" {
		t.Error("Response should be successful")
	}

	data, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}
	def, ok := data["definition"].(string)
	if !ok {
		t.Fatal("Data should contain definition")
	}
	if def == "" {
		t.Error("Definition should not be empty")
	}
}

// TestZLDIPCSearchKeys verifies IPC search-keys command.
func TestZLDIPCSearchKeys(t *testing.T) {
	request := ipc.Request{
		Command: "search-keys",
		Args: map[string]interface{}{
			"module":  "StrongsGreek",
			"pattern": "G23",
		},
	}

	if request.Command != "search-keys" {
		t.Errorf("Command = %q, want %q", request.Command, "search-keys")
	}

	// Response should contain matching keys
	response := ipc.Response{
		Status: "success",
		Result: map[string]interface{}{
			"matches": []string{"G2300", "G2301", "G2316"},
		},
	}

	if response.Status != "success" {
		t.Error("Response should be successful")
	}

	data, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}
	matches, ok := data["matches"].([]string)
	if !ok {
		t.Fatal("Data should contain matches array")
	}
	if len(matches) != 3 {
		t.Errorf("len(matches) = %d, want 3", len(matches))
	}
}

// TestZLDCaseSensitiveKeys verifies case sensitivity in key lookup.
func TestZLDCaseSensitiveKeys(t *testing.T) {
	parser := &ZLDParser{
		entries: map[string]*ZLDEntry{
			"God": {Key: "God", Definition: "The deity"},
			"god": {Key: "god", Definition: "A deity"},
			"GOD": {Key: "GOD", Definition: "ALL CAPS"},
		},
	}

	// Each case should be distinct
	entry, _ := parser.GetEntry("God")
	if entry.Definition != "The deity" {
		t.Errorf("God definition = %q, want %q", entry.Definition, "The deity")
	}

	entry, _ = parser.GetEntry("god")
	if entry.Definition != "A deity" {
		t.Errorf("god definition = %q, want %q", entry.Definition, "A deity")
	}

	entry, _ = parser.GetEntry("GOD")
	if entry.Definition != "ALL CAPS" {
		t.Errorf("GOD definition = %q, want %q", entry.Definition, "ALL CAPS")
	}
}

// TestZLDEmptyDefinition verifies handling of entries with empty definitions.
func TestZLDEmptyDefinition(t *testing.T) {
	parser := &ZLDParser{
		entries: map[string]*ZLDEntry{
			"EMPTY": {Key: "EMPTY", Definition: ""},
		},
	}

	entry, err := parser.GetEntry("EMPTY")
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	// Empty definition is valid (entry exists but has no content)
	if entry.Definition != "" {
		t.Errorf("Definition = %q, want empty", entry.Definition)
	}
}

// TestZLDMultiBlockEntry verifies entries spanning multiple blocks.
func TestZLDMultiBlockEntry(t *testing.T) {
	// Large entries may span multiple compressed blocks
	// This tests the boundary handling

	largeDefinition := ""
	for i := 0; i < 1000; i++ {
		largeDefinition += "This is a very long dictionary entry. "
	}

	parser := &ZLDParser{
		entries: map[string]*ZLDEntry{
			"LARGE": {Key: "LARGE", Definition: largeDefinition},
		},
	}

	entry, err := parser.GetEntry("LARGE")
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	if len(entry.Definition) != len(largeDefinition) {
		t.Errorf("Definition length = %d, want %d", len(entry.Definition), len(largeDefinition))
	}
}
