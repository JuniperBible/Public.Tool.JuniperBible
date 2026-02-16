package sword

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// createTestRawGenBookModule creates a minimal test RawGenBook module.
func createTestRawGenBookModule(t *testing.T) (string, *Module) {
	t.Helper()

	// Create temp directory
	tmpDir := t.TempDir()
	modDir := filepath.Join(tmpDir, "testbook")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatalf("creating module dir: %v", err)
	}

	// Create .bdt file with sample content
	content1 := []byte("This is the first entry content.\nIt has multiple lines.")
	content2 := []byte("Second entry with different content.\n\nContains blank lines.")
	content3 := []byte("Third entry - short.")

	bdtContent := append(content1, content2...)
	bdtContent = append(bdtContent, content3...)

	bdtPath := filepath.Join(modDir, "testbook.bdt")
	if err := os.WriteFile(bdtPath, bdtContent, 0600); err != nil {
		t.Fatalf("writing .bdt file: %v", err)
	}

	// Create .idx file with offsets (4-byte little-endian each)
	idxData := make([]byte, 12)                                                     // 3 entries * 4 bytes
	binary.LittleEndian.PutUint32(idxData[0:], 0)                                   // First entry at offset 0
	binary.LittleEndian.PutUint32(idxData[4:], uint32(len(content1)))               // Second entry
	binary.LittleEndian.PutUint32(idxData[8:], uint32(len(content1)+len(content2))) // Third entry

	idxPath := filepath.Join(modDir, "testbook.idx")
	if err := os.WriteFile(idxPath, idxData, 0600); err != nil {
		t.Fatalf("writing .idx file: %v", err)
	}

	// Create .dat file with TreeKey structure
	// Format: 0xFFFFFFFF marker, metadata, null-terminated key
	var datContent []byte

	// Entry 1
	datContent = append(datContent, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF) // Marker
	datContent = append(datContent, 0x00, 0x00, 0x00, 0x00)                         // Offset
	datContent = append(datContent, 0x00, 0x00, 0x00, 0x00)                         // Size
	datContent = append(datContent, []byte("Entry One")...)
	datContent = append(datContent, 0x00) // Null terminator

	// Entry 2
	datContent = append(datContent, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)
	datContent = append(datContent, 0x00, 0x00, 0x00, 0x00)
	datContent = append(datContent, 0x00, 0x00, 0x00, 0x00)
	datContent = append(datContent, []byte("Entry Two")...)
	datContent = append(datContent, 0x00)

	// Entry 3
	datContent = append(datContent, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)
	datContent = append(datContent, 0x00, 0x00, 0x00, 0x00)
	datContent = append(datContent, 0x00, 0x00, 0x00, 0x00)
	datContent = append(datContent, []byte("Entry Three")...)
	datContent = append(datContent, 0x00)

	datPath := filepath.Join(modDir, "testbook.dat")
	if err := os.WriteFile(datPath, datContent, 0600); err != nil {
		t.Fatalf("writing .dat file: %v", err)
	}

	module := &Module{
		ID:         "testbook",
		Title:      "Test Book",
		ModuleType: ModuleTypeGenBook,
		Driver:     DriverRawGenBook,
		DataPath:   "modules/genbook/rawgenbook/testbook",
	}

	return tmpDir, module
}

func TestNewRawGenBookParser(t *testing.T) {
	tmpDir, module := createTestRawGenBookModule(t)

	parser := NewRawGenBookParser(module, tmpDir)

	if parser == nil {
		t.Fatal("parser should not be nil")
	}
	if parser.module != module {
		t.Error("parser should store module reference")
	}
	if parser.entries == nil {
		t.Error("entries map should be initialized")
	}
}

func TestRawGenBookParser_LoadIndices(t *testing.T) {
	tmpDir, module := createTestRawGenBookModule(t)

	// Adjust data path to point directly to module dir
	modDir := filepath.Join(tmpDir, "testbook")
	parser := &RawGenBookParser{
		module:   module,
		dataPath: modDir,
		entries:  make(map[string]GenBookEntry),
	}

	err := parser.loadIndices()
	if err != nil {
		t.Fatalf("loadIndices failed: %v", err)
	}

	if !parser.loaded {
		t.Error("loaded flag should be true after loadIndices")
	}

	// Should have found some entries
	if len(parser.entries) == 0 {
		t.Error("should have loaded some entries")
	}
}

func TestRawGenBookParser_GetAllKeys(t *testing.T) {
	tmpDir, module := createTestRawGenBookModule(t)
	modDir := filepath.Join(tmpDir, "testbook")

	parser := &RawGenBookParser{
		module:   module,
		dataPath: modDir,
		entries:  make(map[string]GenBookEntry),
	}

	keys, err := parser.GetAllKeys()
	if err != nil {
		t.Fatalf("GetAllKeys failed: %v", err)
	}

	if len(keys) == 0 {
		t.Error("should have found keys")
	}

	t.Logf("Found %d keys: %v", len(keys), keys)
}

func TestRawGenBookParser_GetEntry(t *testing.T) {
	tmpDir, module := createTestRawGenBookModule(t)
	modDir := filepath.Join(tmpDir, "testbook")

	parser := &RawGenBookParser{
		module:   module,
		dataPath: modDir,
		entries:  make(map[string]GenBookEntry),
	}

	// Load indices first to get actual keys
	if err := parser.loadIndices(); err != nil {
		t.Fatalf("loadIndices failed: %v", err)
	}

	if len(parser.keys) == 0 {
		t.Skip("no keys found in test module")
	}

	// Try getting the first entry
	firstKey := parser.keys[0]
	entry, err := parser.GetEntry(firstKey)
	if err != nil {
		t.Fatalf("GetEntry failed for %q: %v", firstKey, err)
	}

	if entry == nil {
		t.Fatal("entry should not be nil")
	}
	if entry.Key == "" {
		t.Error("entry key should not be empty")
	}
	t.Logf("Got entry: key=%q, content=%q", entry.Key, entry.Content[:min(50, len(entry.Content))])
}

func TestRawGenBookParser_GetEntry_NotFound(t *testing.T) {
	tmpDir, module := createTestRawGenBookModule(t)
	modDir := filepath.Join(tmpDir, "testbook")

	parser := &RawGenBookParser{
		module:   module,
		dataPath: modDir,
		entries:  make(map[string]GenBookEntry),
	}

	_, err := parser.GetEntry("nonexistent-key-xyz")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestRawGenBookParser_SearchKeys(t *testing.T) {
	tmpDir, module := createTestRawGenBookModule(t)
	modDir := filepath.Join(tmpDir, "testbook")

	parser := &RawGenBookParser{
		module:   module,
		dataPath: modDir,
		entries:  make(map[string]GenBookEntry),
	}

	// First ensure we have entries
	if err := parser.loadIndices(); err != nil {
		t.Fatalf("loadIndices failed: %v", err)
	}

	if len(parser.keys) == 0 {
		t.Skip("no keys to search")
	}

	// Search for "entry" which should match our test entries
	matches, err := parser.SearchKeys("entry")
	if err != nil {
		t.Fatalf("SearchKeys failed: %v", err)
	}

	t.Logf("Search 'entry' found %d matches: %v", len(matches), matches)
}

func TestRawGenBookParser_GetAllEntries(t *testing.T) {
	tmpDir, module := createTestRawGenBookModule(t)
	modDir := filepath.Join(tmpDir, "testbook")

	parser := &RawGenBookParser{
		module:   module,
		dataPath: modDir,
		entries:  make(map[string]GenBookEntry),
	}

	entries, err := parser.GetAllEntries()
	if err != nil {
		t.Fatalf("GetAllEntries failed: %v", err)
	}

	if len(entries) == 0 {
		t.Error("should have found entries")
	}

	for i, entry := range entries {
		t.Logf("Entry %d: key=%q, level=%d, content_len=%d", i, entry.Key, entry.Level, len(entry.Content))
	}
}

func TestRawGenBookParser_EntryCount(t *testing.T) {
	tmpDir, module := createTestRawGenBookModule(t)
	modDir := filepath.Join(tmpDir, "testbook")

	parser := &RawGenBookParser{
		module:   module,
		dataPath: modDir,
		entries:  make(map[string]GenBookEntry),
	}

	count, err := parser.EntryCount()
	if err != nil {
		t.Fatalf("EntryCount failed: %v", err)
	}

	if count == 0 {
		t.Error("entry count should be > 0")
	}
	t.Logf("Entry count: %d", count)
}

func TestRawGenBookParser_Module(t *testing.T) {
	tmpDir, module := createTestRawGenBookModule(t)
	modDir := filepath.Join(tmpDir, "testbook")

	parser := &RawGenBookParser{
		module:   module,
		dataPath: modDir,
		entries:  make(map[string]GenBookEntry),
	}

	if parser.Module() != module {
		t.Error("Module() should return the module")
	}
}

func TestRawGenBookParser_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	emptyDir := filepath.Join(tmpDir, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("creating empty dir: %v", err)
	}

	module := &Module{
		ID:         "empty",
		ModuleType: ModuleTypeGenBook,
		Driver:     DriverRawGenBook,
	}

	parser := &RawGenBookParser{
		module:   module,
		dataPath: emptyDir,
		entries:  make(map[string]GenBookEntry),
	}

	err := parser.loadIndices()
	if err == nil {
		t.Error("expected error for missing files")
	}
}

func TestIsLikelyText(t *testing.T) {
	parser := &RawGenBookParser{}

	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{"empty", []byte{}, false},
		{"single byte", []byte{0x41}, false},
		{"ascii text", []byte("Hello World"), true},
		{"utf8 text", []byte("Привет"), true},
		{"binary data", []byte{0x00, 0x01, 0x02, 0x03}, false},
		{"mixed text", []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f}, true}, // "Hello"
		{"mostly binary", []byte{0x00, 0x00, 0x00, 0x41}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.isLikelyText(tt.data)
			if result != tt.expected {
				t.Errorf("isLikelyText(%v) = %v, expected %v", tt.data, result, tt.expected)
			}
		})
	}
}

// Integration test with real SWORD module (if available)
func TestRawGenBookParser_RealModule(t *testing.T) {
	swordDir := os.Getenv("HOME") + "/.sword"

	// Check if a known RawGenBook module exists
	testModules := []string{"creed", "didache", "enoch", "josephus"}

	var foundModule string
	var modPath string

	for _, modName := range testModules {
		path := filepath.Join(swordDir, "modules/genbook/rawgenbook", modName)
		if _, err := os.Stat(path); err == nil {
			foundModule = modName
			modPath = path
			break
		}
	}

	if foundModule == "" {
		t.Skip("no RawGenBook test modules found in ~/.sword")
	}

	t.Logf("Testing with real module: %s", foundModule)

	module := &Module{
		ID:         foundModule,
		Title:      foundModule,
		ModuleType: ModuleTypeGenBook,
		Driver:     DriverRawGenBook,
		DataPath:   "modules/genbook/rawgenbook/" + foundModule,
	}

	parser := &RawGenBookParser{
		module:   module,
		dataPath: modPath,
		entries:  make(map[string]GenBookEntry),
	}

	// Test loading indices
	err := parser.loadIndices()
	if err != nil {
		t.Fatalf("loadIndices failed: %v", err)
	}

	t.Logf("Loaded %d entries from %s", len(parser.entries), foundModule)

	// Test getting keys
	keys, err := parser.GetAllKeys()
	if err != nil {
		t.Fatalf("GetAllKeys failed: %v", err)
	}

	if len(keys) == 0 {
		t.Error("expected some keys")
	}

	t.Logf("First 5 keys: %v", keys[:min(5, len(keys))])

	// Test getting first entry
	if len(keys) > 0 {
		entry, err := parser.GetEntry(keys[0])
		if err != nil {
			t.Fatalf("GetEntry failed: %v", err)
		}

		t.Logf("First entry: key=%q, content_preview=%q",
			entry.Key,
			entry.Content[:min(100, len(entry.Content))])
	}
}

// min is defined in integration_test.go

// TestRawGenBookParser_Enoch tests with the Book of Enoch module
func TestRawGenBookParser_Enoch(t *testing.T) {
	swordDir := os.Getenv("HOME") + "/.sword"
	modPath := filepath.Join(swordDir, "modules/genbook/rawgenbook/enoch")

	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		t.Skip("Enoch module not installed")
	}

	module := &Module{
		ID:         "enoch",
		Title:      "The Book of Enoch",
		ModuleType: ModuleTypeGenBook,
		Driver:     DriverRawGenBook,
		DataPath:   "modules/genbook/rawgenbook/enoch",
	}

	parser := &RawGenBookParser{
		module:   module,
		dataPath: modPath,
		entries:  make(map[string]GenBookEntry),
	}

	// Load indices
	err := parser.loadIndices()
	if err != nil {
		t.Fatalf("loadIndices failed: %v", err)
	}

	keys, err := parser.GetAllKeys()
	if err != nil {
		t.Fatalf("GetAllKeys failed: %v", err)
	}

	t.Logf("Enoch has %d entries", len(keys))
	if len(keys) == 0 {
		t.Fatal("expected entries in Enoch")
	}

	// Log first 10 keys
	t.Logf("First 10 keys: %v", keys[:min(10, len(keys))])

	// Get and verify first entry has content
	entry, err := parser.GetEntry(keys[0])
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	if len(entry.Content) < 10 {
		t.Error("expected substantial content in first entry")
	}
	t.Logf("First entry (%s): %d chars", entry.Key, len(entry.Content))
}

// TestRawGenBookParser_Jubilees tests with the Book of Jubilees module
func TestRawGenBookParser_Jubilees(t *testing.T) {
	swordDir := os.Getenv("HOME") + "/.sword"
	modPath := filepath.Join(swordDir, "modules/genbook/rawgenbook/jubilees")

	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		t.Skip("Jubilees module not installed")
	}

	module := &Module{
		ID:         "jubilees",
		Title:      "The Book of Jubilees",
		ModuleType: ModuleTypeGenBook,
		Driver:     DriverRawGenBook,
		DataPath:   "modules/genbook/rawgenbook/jubilees",
	}

	parser := &RawGenBookParser{
		module:   module,
		dataPath: modPath,
		entries:  make(map[string]GenBookEntry),
	}

	// Load indices
	err := parser.loadIndices()
	if err != nil {
		t.Fatalf("loadIndices failed: %v", err)
	}

	keys, err := parser.GetAllKeys()
	if err != nil {
		t.Fatalf("GetAllKeys failed: %v", err)
	}

	t.Logf("Jubilees has %d entries", len(keys))
	if len(keys) == 0 {
		t.Fatal("expected entries in Jubilees")
	}

	// Log first 10 keys
	t.Logf("First 10 keys: %v", keys[:min(10, len(keys))])

	// Get and verify first entry has content
	entry, err := parser.GetEntry(keys[0])
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}

	if len(entry.Content) < 10 {
		t.Error("expected substantial content in first entry")
	}
	t.Logf("First entry (%s): %d chars", entry.Key, len(entry.Content))
}

// TestRawGenBookParser_AllEntries_Enoch retrieves all Enoch entries
func TestRawGenBookParser_AllEntries_Enoch(t *testing.T) {
	swordDir := os.Getenv("HOME") + "/.sword"
	modPath := filepath.Join(swordDir, "modules/genbook/rawgenbook/enoch")

	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		t.Skip("Enoch module not installed")
	}

	module := &Module{
		ID:         "enoch",
		Title:      "The Book of Enoch",
		ModuleType: ModuleTypeGenBook,
		Driver:     DriverRawGenBook,
		DataPath:   "modules/genbook/rawgenbook/enoch",
	}

	parser := &RawGenBookParser{
		module:   module,
		dataPath: modPath,
		entries:  make(map[string]GenBookEntry),
	}

	entries, err := parser.GetAllEntries()
	if err != nil {
		t.Fatalf("GetAllEntries failed: %v", err)
	}

	t.Logf("Retrieved %d entries from Enoch", len(entries))

	// Calculate total content size
	totalSize := 0
	for _, entry := range entries {
		totalSize += len(entry.Content)
	}
	t.Logf("Total content: %d bytes (%.1f KB)", totalSize, float64(totalSize)/1024)

	// Verify we have substantial content (Enoch is ~500KB)
	if totalSize < 100000 {
		t.Errorf("expected at least 100KB of content, got %d bytes", totalSize)
	}
}

// TestRawGenBookParser_ParseSimpleKeyData tests the fallback simple key parser
func TestRawGenBookParser_ParseSimpleKeyData(t *testing.T) {
	parser := &RawGenBookParser{
		entries: make(map[string]GenBookEntry),
	}

	// Create test data with null-terminated strings
	datData := []byte("First Entry\x00Second Entry\x00Third Entry\x00")
	offsets := []uint32{0, 100, 200}
	sizes := []uint32{50, 50, 50}

	err := parser.parseSimpleKeyData(datData, offsets, sizes)
	if err != nil {
		t.Fatalf("parseSimpleKeyData failed: %v", err)
	}

	// Check that entries were created
	if len(parser.entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(parser.entries))
	}

	// Verify specific entries
	if entry, ok := parser.entries["First Entry"]; !ok {
		t.Error("'First Entry' not found")
	} else if entry.Offset != 0 {
		t.Errorf("First Entry offset = %d, want 0", entry.Offset)
	}

	if entry, ok := parser.entries["Second Entry"]; !ok {
		t.Error("'Second Entry' not found")
	} else if entry.Offset != 100 {
		t.Errorf("Second Entry offset = %d, want 100", entry.Offset)
	}
}

// TestRawGenBookParser_ParseSimpleKeyData_Binary tests with binary data
func TestRawGenBookParser_ParseSimpleKeyData_Binary(t *testing.T) {
	parser := &RawGenBookParser{
		entries: make(map[string]GenBookEntry),
	}

	// Create test data with mostly binary content (should be skipped)
	datData := []byte{0x00, 0x01, 0x02, 0x03, 0x00, 'T', 'e', 'x', 't', 0x00}
	offsets := []uint32{0, 100}
	sizes := []uint32{50, 50}

	err := parser.parseSimpleKeyData(datData, offsets, sizes)
	if err != nil {
		t.Fatalf("parseSimpleKeyData failed: %v", err)
	}

	// Only "Text" should be recognized as text
	if len(parser.entries) > 1 {
		t.Errorf("expected at most 1 entry, got %d", len(parser.entries))
	}
}

// TestRawGenBookParser_ParseSimpleKeyData_Empty tests with empty data
func TestRawGenBookParser_ParseSimpleKeyData_Empty(t *testing.T) {
	parser := &RawGenBookParser{
		entries: make(map[string]GenBookEntry),
	}

	// Empty data
	datData := []byte{}
	offsets := []uint32{}
	sizes := []uint32{}

	err := parser.parseSimpleKeyData(datData, offsets, sizes)
	if err != nil {
		t.Fatalf("parseSimpleKeyData failed: %v", err)
	}

	if len(parser.entries) != 0 {
		t.Errorf("expected 0 entries for empty data, got %d", len(parser.entries))
	}
}

// TestRawGenBookParser_Quran tests with the Quran module (Russian translation)
func TestRawGenBookParser_Quran(t *testing.T) {
	swordDir := os.Getenv("HOME") + "/.sword"
	modPath := filepath.Join(swordDir, "modules/genbook/rawgenbook/koran")

	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		t.Skip("Quran (KORAN) module not installed")
	}

	module := &Module{
		ID:         "koran",
		Title:      "Quran (Russian)",
		ModuleType: ModuleTypeGenBook,
		Driver:     DriverRawGenBook,
		DataPath:   "modules/genbook/rawgenbook/koran",
	}

	parser := &RawGenBookParser{
		module:   module,
		dataPath: modPath,
		entries:  make(map[string]GenBookEntry),
	}

	// Load indices
	err := parser.loadIndices()
	if err != nil {
		t.Fatalf("loadIndices failed: %v", err)
	}

	keys, err := parser.GetAllKeys()
	if err != nil {
		t.Fatalf("GetAllKeys failed: %v", err)
	}

	t.Logf("Quran has %d entries (surahs)", len(keys))
	if len(keys) == 0 {
		t.Fatal("expected entries in Quran")
	}

	// Log first 10 keys
	t.Logf("First 10 keys: %v", keys[:min(10, len(keys))])

	// Get all entries and calculate total size
	entries, err := parser.GetAllEntries()
	if err != nil {
		t.Fatalf("GetAllEntries failed: %v", err)
	}

	totalSize := 0
	for _, entry := range entries {
		totalSize += len(entry.Content)
	}
	t.Logf("Total Quran content: %d bytes (%.1f KB)", totalSize, float64(totalSize)/1024)

	// Quran should have substantial content
	if totalSize < 50000 {
		t.Errorf("expected at least 50KB of content, got %d bytes", totalSize)
	}
}
