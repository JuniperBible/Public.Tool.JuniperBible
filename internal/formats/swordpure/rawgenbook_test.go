package swordpure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewRawGenBookParser(t *testing.T) {
	parser, err := NewRawGenBookParser("/tmp/test")
	if err != nil {
		t.Fatalf("NewRawGenBookParser failed: %v", err)
	}
	if parser == nil {
		t.Fatal("parser is nil")
	}
	if parser.modulePath != "/tmp/test" {
		t.Errorf("expected modulePath /tmp/test, got %s", parser.modulePath)
	}
}

func TestParseRawGenBookTreeIndex(t *testing.T) {
	// Create test data: 2 entries
	// Entry 0: parent=-1, child=1, sibling=-1, name="Root"
	// Entry 1: parent=0, child=-1, sibling=-1, name="Child"
	data := make([]byte, 0)

	// Entry 0: parent=-1 (0xFFFFFFFF)
	data = append(data, 0xFF, 0xFF, 0xFF, 0xFF)
	// firstChild=1
	data = append(data, 0x01, 0x00, 0x00, 0x00)
	// nextSibling=-1 (0xFFFFFFFF)
	data = append(data, 0xFF, 0xFF, 0xFF, 0xFF)
	// name="Root\0"
	data = append(data, 'R', 'o', 'o', 't', 0x00)

	// Entry 1: parent=0
	data = append(data, 0x00, 0x00, 0x00, 0x00)
	// firstChild=-1 (0xFFFFFFFF)
	data = append(data, 0xFF, 0xFF, 0xFF, 0xFF)
	// nextSibling=-1 (0xFFFFFFFF)
	data = append(data, 0xFF, 0xFF, 0xFF, 0xFF)
	// name="Child\0"
	data = append(data, 'C', 'h', 'i', 'l', 'd', 0x00)

	keys, err := parseRawGenBookTreeIndex(data)
	if err != nil {
		t.Fatalf("parseRawGenBookTreeIndex failed: %v", err)
	}

	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}

	if keys[0].Name != "Root" {
		t.Errorf("expected name Root, got %s", keys[0].Name)
	}
	if keys[0].Parent != -1 {
		t.Errorf("expected parent -1, got %d", keys[0].Parent)
	}
	if keys[0].FirstChild != 1 {
		t.Errorf("expected firstChild 1, got %d", keys[0].FirstChild)
	}

	if keys[1].Name != "Child" {
		t.Errorf("expected name Child, got %s", keys[1].Name)
	}
	if keys[1].Parent != 0 {
		t.Errorf("expected parent 0, got %d", keys[1].Parent)
	}
}

func TestParseRawGenBookTreeIndexEmpty(t *testing.T) {
	keys, err := parseRawGenBookTreeIndex([]byte{})
	if err != nil {
		t.Fatalf("parseRawGenBookTreeIndex failed: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

func TestParseRawGenBookTreeIndexTruncated(t *testing.T) {
	// Only 10 bytes (needs at least 12 for one entry)
	data := make([]byte, 10)
	keys, err := parseRawGenBookTreeIndex(data)
	if err != nil {
		t.Fatalf("parseRawGenBookTreeIndex failed: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys for truncated data, got %d", len(keys))
	}
}

func TestParseRawGenBookDataIndex(t *testing.T) {
	// Create test data: 2 entries (8 bytes each)
	data := make([]byte, 16)

	// Entry 0: offset=0, size=100
	data[0], data[1], data[2], data[3] = 0x00, 0x00, 0x00, 0x00
	data[4], data[5], data[6], data[7] = 0x64, 0x00, 0x00, 0x00

	// Entry 1: offset=100, size=50
	data[8], data[9], data[10], data[11] = 0x64, 0x00, 0x00, 0x00
	data[12], data[13], data[14], data[15] = 0x32, 0x00, 0x00, 0x00

	entries, err := parseRawGenBookDataIndex(data)
	if err != nil {
		t.Fatalf("parseRawGenBookDataIndex failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Offset != 0 || entries[0].Size != 100 {
		t.Errorf("entry 0: expected offset=0, size=100, got offset=%d, size=%d", entries[0].Offset, entries[0].Size)
	}

	if entries[1].Offset != 100 || entries[1].Size != 50 {
		t.Errorf("entry 1: expected offset=100, size=50, got offset=%d, size=%d", entries[1].Offset, entries[1].Size)
	}
}

func TestParseRawGenBookDataIndexInvalidSize(t *testing.T) {
	// Invalid size (not multiple of 8)
	data := make([]byte, 10)
	_, err := parseRawGenBookDataIndex(data)
	if err == nil {
		t.Error("expected error for invalid size")
	}
}

func TestRawGenBookParserGetEntry(t *testing.T) {
	parser, _ := NewRawGenBookParser("/tmp/test")

	// Add a test entry
	parser.entries["/Test/Entry"] = &RawGenBookEntry{
		Key:     "/Test/Entry",
		Content: "Test content",
	}

	entry, err := parser.GetEntry("/Test/Entry")
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if entry.Content != "Test content" {
		t.Errorf("expected content 'Test content', got '%s'", entry.Content)
	}
}

func TestRawGenBookParserGetEntryNotFound(t *testing.T) {
	parser, _ := NewRawGenBookParser("/tmp/test")

	_, err := parser.GetEntry("/NonExistent")
	if err == nil {
		t.Error("expected error for non-existent entry")
	}
}

func TestRawGenBookParserListKeys(t *testing.T) {
	parser, _ := NewRawGenBookParser("/tmp/test")

	parser.entries["/A"] = &RawGenBookEntry{Key: "/A"}
	parser.entries["/B"] = &RawGenBookEntry{Key: "/B"}

	keys := parser.ListKeys()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestRawGenBookParserGetRoot(t *testing.T) {
	parser, _ := NewRawGenBookParser("/tmp/test")

	// No tree keys
	if parser.GetRoot() != nil {
		t.Error("expected nil for empty tree")
	}

	// Add tree keys
	parser.treeKeys = []TreeKey{
		{Name: "Root", Parent: -1},
		{Name: "Child", Parent: 0},
	}

	root := parser.GetRoot()
	if root == nil {
		t.Fatal("GetRoot returned nil")
	}
	if root.Name != "Root" {
		t.Errorf("expected name Root, got %s", root.Name)
	}
}

func TestRawGenBookParserGetChildren(t *testing.T) {
	parser, _ := NewRawGenBookParser("/tmp/test")

	parser.treeKeys = []TreeKey{
		{Name: "Root", Parent: -1, FirstChild: 1, NextSibling: -1},
		{Name: "Child1", Parent: 0, FirstChild: -1, NextSibling: 2},
		{Name: "Child2", Parent: 0, FirstChild: -1, NextSibling: -1},
	}

	children := parser.GetChildren(0)
	if len(children) != 2 {
		t.Errorf("expected 2 children, got %d", len(children))
	}

	// Invalid index
	children = parser.GetChildren(-1)
	if children != nil {
		t.Error("expected nil for invalid index")
	}

	children = parser.GetChildren(100)
	if children != nil {
		t.Error("expected nil for out of range index")
	}
}

func TestRawGenBookParserBuildKeyPath(t *testing.T) {
	parser, _ := NewRawGenBookParser("/tmp/test")

	parser.treeKeys = []TreeKey{
		{Name: "Root", Parent: -1},
		{Name: "Chapter 1", Parent: 0},
		{Name: "Article 1", Parent: 1},
	}

	path := parser.BuildKeyPath(2)
	if path != "/Root/Chapter 1/Article 1" {
		t.Errorf("expected '/Root/Chapter 1/Article 1', got '%s'", path)
	}

	// Root node
	path = parser.BuildKeyPath(0)
	if path != "/Root" {
		t.Errorf("expected '/Root', got '%s'", path)
	}

	// Invalid index
	path = parser.BuildKeyPath(-1)
	if path != "" {
		t.Errorf("expected empty string for invalid index, got '%s'", path)
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct {
		parts []string
		want  string
	}{
		{[]string{}, ""},
		{[]string{"A"}, "A"},
		{[]string{"A", "B"}, "A/B"},
		{[]string{"A", "B", "C"}, "A/B/C"},
	}

	for _, tt := range tests {
		got := joinPath(tt.parts)
		if got != tt.want {
			t.Errorf("joinPath(%v) = %q, want %q", tt.parts, got, tt.want)
		}
	}
}

func TestRawGenBookParserModuleInfo(t *testing.T) {
	parser, _ := NewRawGenBookParser("/tmp/test")
	parser.entries["/Test"] = &RawGenBookEntry{}

	info := parser.ModuleInfo()
	if info.EntryCount != 1 {
		t.Errorf("expected EntryCount 1, got %d", info.EntryCount)
	}
}

// Writer tests

func TestNewRawGenBookWriter(t *testing.T) {
	writer := NewRawGenBookWriter("/tmp/test")
	if writer == nil {
		t.Fatal("NewRawGenBookWriter returned nil")
	}
	if writer.dataPath != "/tmp/test" {
		t.Errorf("expected dataPath /tmp/test, got %s", writer.dataPath)
	}
}

func TestRawGenBookWriterAddEntry(t *testing.T) {
	writer := NewRawGenBookWriter(t.TempDir())

	writer.AddEntry("/WCF/Chapter 1", "First chapter content")
	writer.AddEntry("/WCF/Chapter 2", "Second chapter content")

	if len(writer.nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(writer.nodes))
	}
}

func TestRawGenBookWriterWriteModule(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "genbook")

	writer := NewRawGenBookWriter(dataPath)
	writer.AddEntry("/WCF", "Westminster Confession of Faith")
	writer.AddEntry("/WCF/Chapter 1", "Of Holy Scripture")
	writer.AddEntry("/WCF/Chapter 1/Article 1", "The light of nature...")

	count, err := writer.WriteModule()
	if err != nil {
		t.Fatalf("WriteModule failed: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 entries, got %d", count)
	}

	// Check files exist
	files := []string{"book.bdt", "book.idx", "book.dat"}
	for _, f := range files {
		path := filepath.Join(dataPath, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}
}

func TestRawGenBookWriterWriteModuleEmpty(t *testing.T) {
	writer := NewRawGenBookWriter(t.TempDir())

	count, err := writer.WriteModule()
	if err != nil {
		t.Fatalf("WriteModule failed: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 entries for empty writer, got %d", count)
	}
}

func TestGetParentPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/WCF/Chapter 1", "/WCF"},
		{"/WCF", ""},
		{"/A/B/C", "/A/B"},
		{"/Single", ""},
	}

	for _, tt := range tests {
		got := getParentPath(tt.path)
		if got != tt.want {
			t.Errorf("getParentPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestEmitRawGenBook(t *testing.T) {
	tmpDir := t.TempDir()

	corpus := &IRCorpus{
		ID:       "TestGenBook",
		Title:    "Test General Book",
		Language: "en",
		Documents: []*IRDocument{
			{
				ID: "Entries",
				ContentBlocks: []*IRContentBlock{
					{ID: "WCF", Text: "Westminster Confession"},
					{ID: "WCF/Chapter1", Text: "Of Holy Scripture"},
				},
			},
		},
	}

	result, err := EmitRawGenBook(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitRawGenBook failed: %v", err)
	}

	if result.ModuleID != "TestGenBook" {
		t.Errorf("expected ModuleID TestGenBook, got %s", result.ModuleID)
	}

	if result.VersesWritten != 2 {
		t.Errorf("expected 2 entries written, got %d", result.VersesWritten)
	}

	// Check conf file exists
	confPath := filepath.Join(tmpDir, "mods.d", "testgenbook.conf")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Error("conf file not created")
	}
}

func TestEmitRawGenBookUsesRawMarkup(t *testing.T) {
	tmpDir := t.TempDir()

	corpus := &IRCorpus{
		ID:       "Test",
		Title:    "Test",
		Language: "en",
		Documents: []*IRDocument{
			{
				ID: "Entries",
				ContentBlocks: []*IRContentBlock{
					{ID: "test", Text: "plain", RawMarkup: "<markup>rich</markup>"},
				},
			},
		},
	}

	result, err := EmitRawGenBook(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitRawGenBook failed: %v", err)
	}

	if result.VersesWritten != 1 {
		t.Errorf("expected 1 entry, got %d", result.VersesWritten)
	}
}

func TestGenerateGenBookConf(t *testing.T) {
	corpus := &IRCorpus{
		ID:       "WCF",
		Title:    "Westminster Confession of Faith",
		Language: "en",
	}

	conf := generateGenBookConf(corpus)

	expected := []string{
		"[WCF]",
		"Description=Westminster Confession of Faith",
		"Lang=en",
		"ModDrv=RawGenBook",
		"Encoding=UTF-8",
		"DataPath=./modules/genbook/rawgenbook/wcf/book",
	}

	for _, e := range expected {
		if !strContains(conf, e) {
			t.Errorf("conf missing: %s", e)
		}
	}
}
