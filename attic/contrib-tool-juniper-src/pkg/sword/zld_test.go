package sword

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// createTestZLDModule creates a minimal zLD module for testing.
// This creates files in the proper SWORD format:
// - .idx: Binary file with 8-byte entries (4-byte offset + 4-byte size into .dat)
// - .dat: Contains actual key strings (null-terminated or with CRLF)
// - .zdx: Binary file with 8-byte entries (4-byte offset + 4-byte size into .zdt)
// - .zdt: Contains zlib-compressed definition text
func createTestZLDModule(t *testing.T, entries map[string]string, compressed bool) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Collect keys in consistent order
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}

	// Build .dat file (key strings) and .idx file (binary index)
	var datBuffer bytes.Buffer
	var idxBuffer bytes.Buffer

	for _, key := range keys {
		offset := uint32(datBuffer.Len())
		// Write key with null terminator to .dat
		datBuffer.WriteString(key)
		datBuffer.WriteByte(0)
		size := uint32(len(key) + 1) // Include null terminator

		// Write 8-byte entry to .idx: offset (4 bytes) + size (4 bytes)
		idxEntry := make([]byte, 8)
		binary.LittleEndian.PutUint32(idxEntry[0:4], offset)
		binary.LittleEndian.PutUint32(idxEntry[4:8], size)
		idxBuffer.Write(idxEntry)
	}

	// Write .idx and .dat files
	idxPath := filepath.Join(tmpDir, "dict.idx")
	if err := os.WriteFile(idxPath, idxBuffer.Bytes(), 0600); err != nil {
		t.Fatalf("Failed to write idx: %v", err)
	}

	datPath := filepath.Join(tmpDir, "dict.dat")
	if err := os.WriteFile(datPath, datBuffer.Bytes(), 0600); err != nil {
		t.Fatalf("Failed to write dat: %v", err)
	}

	if compressed {
		// Create compressed format (.zdx/.zdt)
		var zdxBuffer bytes.Buffer
		var zdtBuffer bytes.Buffer

		for _, key := range keys {
			text := entries[key]
			offset := uint32(zdtBuffer.Len())

			// Compress the definition
			var compressedBuf bytes.Buffer
			w := zlib.NewWriter(&compressedBuf)
			w.Write([]byte(text))
			w.Close()

			zdtBuffer.Write(compressedBuf.Bytes())

			// Write index entry (8 bytes)
			entry := make([]byte, 8)
			binary.LittleEndian.PutUint32(entry[0:4], offset)
			binary.LittleEndian.PutUint32(entry[4:8], uint32(compressedBuf.Len()))
			zdxBuffer.Write(entry)
		}

		zdxPath := filepath.Join(tmpDir, "dict.zdx")
		if err := os.WriteFile(zdxPath, zdxBuffer.Bytes(), 0600); err != nil {
			t.Fatalf("Failed to write zdx: %v", err)
		}

		zdtPath := filepath.Join(tmpDir, "dict.zdt")
		if err := os.WriteFile(zdtPath, zdtBuffer.Bytes(), 0600); err != nil {
			t.Fatalf("Failed to write zdt: %v", err)
		}
	}

	return tmpDir
}

func TestNewZLDParser(t *testing.T) {
	module := &Module{
		ID:       "TestDict",
		Title:    "Test Dictionary",
		DataPath: "modules/lexdict/zld/testdict",
	}

	tmpDir := t.TempDir()
	parser := NewZLDParser(module, tmpDir)

	if parser == nil {
		t.Fatal("NewZLDParser returned nil")
	}
	if parser.module != module {
		t.Error("parser.module not set correctly")
	}
	if parser.loaded {
		t.Error("parser should not be loaded initially")
	}
	if parser.entries == nil {
		t.Error("parser.entries should be initialized")
	}
}

func TestZLDParser_loadIndices_Compressed(t *testing.T) {
	entries := map[string]string{
		"logos": "word, speech, reason",
		"theos": "God, deity",
	}
	dataPath := createTestZLDModule(t, entries, true)

	module := &Module{
		ID:       "TestDict",
		DataPath: ".",
	}

	parser := &ZLDParser{
		module:   module,
		dataPath: dataPath,
		entries:  make(map[string]DictIndexEntry),
	}

	err := parser.loadIndices()
	if err != nil {
		t.Fatalf("loadIndices() returned error: %v", err)
	}

	if !parser.loaded {
		t.Error("parser should be loaded after loadIndices()")
	}
	if len(parser.keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(parser.keys))
	}
}

func TestZLDParser_loadIndices_Uncompressed(t *testing.T) {
	entries := map[string]string{
		"alpha": "first letter",
		"beta":  "second letter",
	}
	dataPath := createTestZLDModule(t, entries, false)

	module := &Module{
		ID:       "TestDict",
		DataPath: ".",
	}

	parser := &ZLDParser{
		module:   module,
		dataPath: dataPath,
		entries:  make(map[string]DictIndexEntry),
	}

	err := parser.loadIndices()
	if err != nil {
		t.Fatalf("loadIndices() returned error: %v", err)
	}

	if !parser.loaded {
		t.Error("parser should be loaded after loadIndices()")
	}
}

func TestZLDParser_loadIndices_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	module := &Module{
		ID:       "Empty",
		DataPath: ".",
	}

	parser := &ZLDParser{
		module:   module,
		dataPath: tmpDir,
		entries:  make(map[string]DictIndexEntry),
	}

	err := parser.loadIndices()
	if err == nil {
		t.Error("loadIndices() should error when no index files exist")
	}
}

func TestZLDParser_loadIndices_Cached(t *testing.T) {
	entries := map[string]string{"test": "value"}
	dataPath := createTestZLDModule(t, entries, true)

	module := &Module{
		ID:       "Test",
		DataPath: ".",
	}

	parser := &ZLDParser{
		module:   module,
		dataPath: dataPath,
		entries:  make(map[string]DictIndexEntry),
	}

	if err := parser.loadIndices(); err != nil {
		t.Fatalf("First loadIndices() failed: %v", err)
	}

	originalLen := len(parser.keys)

	if err := parser.loadIndices(); err != nil {
		t.Fatalf("Second loadIndices() failed: %v", err)
	}

	if len(parser.keys) != originalLen {
		t.Error("loadIndices() should use cached data")
	}
}

func TestZLDParser_GetEntry_Valid(t *testing.T) {
	entries := map[string]string{
		"logos": "word, speech, divine reason",
	}
	dataPath := createTestZLDModule(t, entries, true)

	module := &Module{
		ID:       "TestDict",
		DataPath: ".",
	}

	parser := &ZLDParser{
		module:   module,
		dataPath: dataPath,
		entries:  make(map[string]DictIndexEntry),
	}

	entry, err := parser.GetEntry("logos")
	if err != nil {
		t.Fatalf("GetEntry() returned error: %v", err)
	}

	if entry.Key != "logos" {
		t.Errorf("Entry Key = %q, want 'logos'", entry.Key)
	}
	if entry.Definition != "word, speech, divine reason" {
		t.Errorf("Entry Definition = %q, want expected text", entry.Definition)
	}
}

func TestZLDParser_GetEntry_CaseInsensitive(t *testing.T) {
	entries := map[string]string{
		"Logos": "word",
	}
	dataPath := createTestZLDModule(t, entries, true)

	module := &Module{
		ID:       "TestDict",
		DataPath: ".",
	}

	parser := &ZLDParser{
		module:   module,
		dataPath: dataPath,
		entries:  make(map[string]DictIndexEntry),
	}

	// Should find regardless of case
	entry, err := parser.GetEntry("LOGOS")
	if err != nil {
		t.Fatalf("GetEntry(uppercase) returned error: %v", err)
	}
	if entry.Key != "Logos" {
		t.Errorf("Entry Key = %q, want 'Logos'", entry.Key)
	}

	entry, err = parser.GetEntry("logos")
	if err != nil {
		t.Fatalf("GetEntry(lowercase) returned error: %v", err)
	}
	if entry.Key != "Logos" {
		t.Errorf("Entry Key = %q, want 'Logos'", entry.Key)
	}
}

func TestZLDParser_GetEntry_NotFound(t *testing.T) {
	entries := map[string]string{
		"exists": "yes",
	}
	dataPath := createTestZLDModule(t, entries, true)

	module := &Module{
		ID:       "TestDict",
		DataPath: ".",
	}

	parser := &ZLDParser{
		module:   module,
		dataPath: dataPath,
		entries:  make(map[string]DictIndexEntry),
	}

	_, err := parser.GetEntry("nonexistent")
	if err == nil {
		t.Error("GetEntry() should error for non-existent key")
	}
}

func TestZLDParser_GetEntry_WithStrongs(t *testing.T) {
	entries := map[string]string{
		"H430": "elohim - God, gods",
	}
	dataPath := createTestZLDModule(t, entries, true)

	module := &Module{
		ID:       "StrongHebrew",
		Title:    "Strong's Hebrew Dictionary",
		DataPath: ".",
	}

	parser := &ZLDParser{
		module:   module,
		dataPath: dataPath,
		entries:  make(map[string]DictIndexEntry),
	}

	entry, err := parser.GetEntry("H430")
	if err != nil {
		t.Fatalf("GetEntry() returned error: %v", err)
	}

	if entry.StrongsNum != "H430" {
		t.Errorf("StrongsNum = %q, want 'H430'", entry.StrongsNum)
	}
}

func TestZLDParser_GetAllKeys(t *testing.T) {
	entries := map[string]string{
		"alpha": "first",
		"beta":  "second",
		"gamma": "third",
	}
	dataPath := createTestZLDModule(t, entries, true)

	module := &Module{
		ID:       "TestDict",
		DataPath: ".",
	}

	parser := &ZLDParser{
		module:   module,
		dataPath: dataPath,
		entries:  make(map[string]DictIndexEntry),
	}

	keys, err := parser.GetAllKeys()
	if err != nil {
		t.Fatalf("GetAllKeys() returned error: %v", err)
	}

	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}
}

func TestZLDParser_GetAllEntries(t *testing.T) {
	entries := map[string]string{
		"A": "def A",
		"B": "def B",
	}
	dataPath := createTestZLDModule(t, entries, true)

	module := &Module{
		ID:       "TestDict",
		DataPath: ".",
	}

	parser := &ZLDParser{
		module:   module,
		dataPath: dataPath,
		entries:  make(map[string]DictIndexEntry),
	}

	allEntries, err := parser.GetAllEntries()
	if err != nil {
		t.Fatalf("GetAllEntries() returned error: %v", err)
	}

	if len(allEntries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(allEntries))
	}
}

func TestZLDParser_SearchKeys(t *testing.T) {
	entries := map[string]string{
		"abraham":   "patriarch",
		"abimelech": "king",
		"moses":     "prophet",
	}
	dataPath := createTestZLDModule(t, entries, true)

	module := &Module{
		ID:       "TestDict",
		DataPath: ".",
	}

	parser := &ZLDParser{
		module:   module,
		dataPath: dataPath,
		entries:  make(map[string]DictIndexEntry),
	}

	matches, err := parser.SearchKeys("ab")
	if err != nil {
		t.Fatalf("SearchKeys() returned error: %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(matches))
	}
}

func TestZLDParser_SearchKeys_NoMatch(t *testing.T) {
	entries := map[string]string{
		"alpha": "first",
	}
	dataPath := createTestZLDModule(t, entries, true)

	module := &Module{
		ID:       "TestDict",
		DataPath: ".",
	}

	parser := &ZLDParser{
		module:   module,
		dataPath: dataPath,
		entries:  make(map[string]DictIndexEntry),
	}

	matches, err := parser.SearchKeys("xyz")
	if err != nil {
		t.Fatalf("SearchKeys() returned error: %v", err)
	}

	if len(matches) != 0 {
		t.Errorf("Expected 0 matches, got %d", len(matches))
	}
}

func TestZLDParser_isStrongsLexicon(t *testing.T) {
	tests := []struct {
		moduleID string
		expected bool
	}{
		{"StrongHebrew", true},
		{"StrongGreek", true},
		{"strongshebrew", true},
		{"strongsgreek", true},
		{"RegularDict", false},
		{"NavesTopical", false},
	}

	for _, tt := range tests {
		t.Run(tt.moduleID, func(t *testing.T) {
			parser := &ZLDParser{
				module: &Module{ID: tt.moduleID},
			}
			result := parser.isStrongsLexicon()
			if result != tt.expected {
				t.Errorf("isStrongsLexicon() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestZLDParser_extractStrongsNumber(t *testing.T) {
	tests := []struct {
		name     string
		moduleID string
		key      string
		expected string
	}{
		{"already prefixed H", "Strong", "H430", "H430"},
		{"already prefixed G", "Strong", "G2316", "G2316"},
		{"hebrew module", "StrongHebrew", "430", "H430"},
		{"greek module", "StrongGreek", "2316", "G2316"},
		{"unknown module", "OtherDict", "430", "430"},
		{"with whitespace", "Strong", "  H430  ", "H430"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &ZLDParser{
				module: &Module{ID: tt.moduleID},
			}
			result := parser.extractStrongsNumber(tt.key)
			if result != tt.expected {
				t.Errorf("extractStrongsNumber(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestZLDParser_parseKeyIndex(t *testing.T) {
	parser := &ZLDParser{}

	tests := []struct {
		name     string
		data     []byte
		expected []string
	}{
		{
			name:     "null separated",
			data:     []byte("alpha\x00beta\x00gamma\x00"),
			expected: []string{"alpha", "beta", "gamma"},
		},
		{
			name:     "no trailing null",
			data:     []byte("alpha\x00beta\x00gamma"),
			expected: []string{"alpha", "beta", "gamma"},
		},
		{
			name:     "with whitespace",
			data:     []byte("  alpha  \x00  beta  \x00"),
			expected: []string{"alpha", "beta"},
		},
		{
			name:     "empty strings ignored",
			data:     []byte("\x00\x00alpha\x00\x00"),
			expected: []string{"alpha"},
		},
		{
			name:     "empty input",
			data:     []byte{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.parseKeyIndex(tt.data)
			if len(result) != len(tt.expected) {
				t.Errorf("parseKeyIndex() returned %d keys, want %d", len(result), len(tt.expected))
				return
			}
			for i, key := range result {
				if key != tt.expected[i] {
					t.Errorf("parseKeyIndex()[%d] = %q, want %q", i, key, tt.expected[i])
				}
			}
		})
	}
}

func TestZLDParser_loadCompressedIndices_EmptyIndex(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty zdx file
	zdxPath := filepath.Join(tmpDir, "dict.zdx")
	if err := os.WriteFile(zdxPath, []byte{}, 0600); err != nil {
		t.Fatalf("Failed to write empty zdx: %v", err)
	}

	// Create empty idx file (binary format - 8 bytes per entry)
	idxPath := filepath.Join(tmpDir, "dict.idx")
	if err := os.WriteFile(idxPath, []byte{}, 0600); err != nil {
		t.Fatalf("Failed to write empty idx: %v", err)
	}

	// Create empty dat file (for key strings)
	datPath := filepath.Join(tmpDir, "dict.dat")
	if err := os.WriteFile(datPath, []byte{}, 0600); err != nil {
		t.Fatalf("Failed to write empty dat: %v", err)
	}

	parser := &ZLDParser{
		module:   &Module{ID: "Test"},
		dataPath: tmpDir,
		entries:  make(map[string]DictIndexEntry),
	}

	err := parser.loadCompressedIndices()
	if err == nil {
		t.Error("loadCompressedIndices() should error for empty index")
	}
}

func TestZLDParser_readEntryText_FromCompressed(t *testing.T) {
	// Test reading entry text from compressed zLD format
	entries := map[string]string{
		"test": "test definition",
	}
	dataPath := createTestZLDModule(t, entries, true) // Use compressed format

	module := &Module{
		ID:       "TestDict",
		DataPath: ".",
	}

	parser := &ZLDParser{
		module:   module,
		dataPath: dataPath,
		entries:  make(map[string]DictIndexEntry),
	}

	if err := parser.loadIndices(); err != nil {
		t.Fatalf("loadIndices() failed: %v", err)
	}

	entry, err := parser.GetEntry("test")
	if err != nil {
		t.Fatalf("GetEntry() returned error: %v", err)
	}

	// The definition should be readable
	if entry.Definition == "" {
		t.Error("Definition should not be empty")
	}
	if entry.Definition != "test definition" {
		t.Errorf("Definition = %q, want 'test definition'", entry.Definition)
	}
}

func TestZLDParser_readUncompressedEntry_Direct(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple uncompressed data file
	ddtContent := "This is test content for reading"
	ddtPath := filepath.Join(tmpDir, "test.ddt")
	if err := os.WriteFile(ddtPath, []byte(ddtContent), 0600); err != nil {
		t.Fatalf("Failed to write ddt: %v", err)
	}

	parser := &ZLDParser{
		module:   &Module{ID: "Test"},
		dataPath: tmpDir,
		entries:  make(map[string]DictIndexEntry),
	}

	// Create an entry that points to the content
	entry := DictIndexEntry{
		Offset: 0,
		Size:   uint32(len(ddtContent)),
	}

	text, err := parser.readUncompressedEntry(ddtPath, entry)
	if err != nil {
		t.Fatalf("readUncompressedEntry() returned error: %v", err)
	}

	if text != ddtContent {
		t.Errorf("text = %q, want %q", text, ddtContent)
	}
}

func TestZLDParser_readUncompressedEntry_PartialRead(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a data file with multiple entries
	ddtContent := "First EntrySecond Entry"
	ddtPath := filepath.Join(tmpDir, "test.ddt")
	if err := os.WriteFile(ddtPath, []byte(ddtContent), 0600); err != nil {
		t.Fatalf("Failed to write ddt: %v", err)
	}

	parser := &ZLDParser{
		module:   &Module{ID: "Test"},
		dataPath: tmpDir,
		entries:  make(map[string]DictIndexEntry),
	}

	// Read only the first entry
	entry := DictIndexEntry{
		Offset: 0,
		Size:   11, // "First Entry"
	}

	text, err := parser.readUncompressedEntry(ddtPath, entry)
	if err != nil {
		t.Fatalf("readUncompressedEntry() returned error: %v", err)
	}

	if text != "First Entry" {
		t.Errorf("text = %q, want 'First Entry'", text)
	}

	// Read the second entry
	entry = DictIndexEntry{
		Offset: 11,
		Size:   12, // "Second Entry"
	}

	text, err = parser.readUncompressedEntry(ddtPath, entry)
	if err != nil {
		t.Fatalf("readUncompressedEntry() returned error: %v", err)
	}

	if text != "Second Entry" {
		t.Errorf("text = %q, want 'Second Entry'", text)
	}
}

func TestZLDParser_readUncompressedEntry_FileNotFound(t *testing.T) {
	parser := &ZLDParser{
		module:   &Module{ID: "Test"},
		dataPath: "/nonexistent",
		entries:  make(map[string]DictIndexEntry),
	}

	entry := DictIndexEntry{
		Offset: 0,
		Size:   10,
	}

	_, err := parser.readUncompressedEntry("/nonexistent/file.ddt", entry)
	if err == nil {
		t.Error("readUncompressedEntry() should error for nonexistent file")
	}
}
