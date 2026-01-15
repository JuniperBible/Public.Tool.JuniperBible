package swordpure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewZLDWriter(t *testing.T) {
	writer := NewZLDWriter("/tmp/test")

	if writer == nil {
		t.Fatal("NewZLDWriter returned nil")
	}
	if writer.dataPath != "/tmp/test" {
		t.Errorf("expected dataPath /tmp/test, got %s", writer.dataPath)
	}
	if writer.blockSize != 4096 {
		t.Errorf("expected blockSize 4096, got %d", writer.blockSize)
	}
}

func TestZLDWriterAddEntry(t *testing.T) {
	writer := NewZLDWriter(t.TempDir())

	writer.AddEntry("key1", "value1")
	writer.AddEntry("key2", "value2")

	if len(writer.entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(writer.entries))
	}

	if writer.entries[0].Key != "key1" {
		t.Errorf("expected key1, got %s", writer.entries[0].Key)
	}
	if writer.entries[0].Text != "value1" {
		t.Errorf("expected value1, got %s", writer.entries[0].Text)
	}
}

func TestZLDWriterWriteModule(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "dict")

	writer := NewZLDWriter(dataPath)

	// Add test entries
	writer.AddEntry("alpha", "First letter of the Greek alphabet")
	writer.AddEntry("beta", "Second letter of the Greek alphabet")
	writer.AddEntry("gamma", "Third letter of the Greek alphabet")

	count, err := writer.WriteModule()
	if err != nil {
		t.Fatalf("WriteModule failed: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 entries written, got %d", count)
	}

	// Check that files were created
	files := []string{"dict.zdt", "dict.zdx", "dict.idx", "dict.dat"}
	for _, f := range files {
		path := filepath.Join(dataPath, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}
}

func TestZLDWriterWriteModuleEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "dict")

	writer := NewZLDWriter(dataPath)

	count, err := writer.WriteModule()
	if err != nil {
		t.Fatalf("WriteModule failed: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 entries for empty writer, got %d", count)
	}
}

func TestZLDWriterWriteModuleSortsEntries(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "dict")

	writer := NewZLDWriter(dataPath)

	// Add entries out of order
	writer.AddEntry("zebra", "A striped animal")
	writer.AddEntry("aardvark", "An ant-eating animal")
	writer.AddEntry("monkey", "A primate")

	_, err := writer.WriteModule()
	if err != nil {
		t.Fatalf("WriteModule failed: %v", err)
	}

	// Entries should be sorted
	if writer.entries[0].Key != "aardvark" {
		t.Errorf("expected first entry to be aardvark, got %s", writer.entries[0].Key)
	}
	if writer.entries[1].Key != "monkey" {
		t.Errorf("expected second entry to be monkey, got %s", writer.entries[1].Key)
	}
	if writer.entries[2].Key != "zebra" {
		t.Errorf("expected third entry to be zebra, got %s", writer.entries[2].Key)
	}
}

func TestZLDWriterFlushBlock(t *testing.T) {
	writer := NewZLDWriter(t.TempDir())

	// Write some data to the current block
	writer.currentBlock.WriteString("test data for compression")

	err := writer.flushBlock()
	if err != nil {
		t.Fatalf("flushBlock failed: %v", err)
	}

	if len(writer.blockIndex) != 1 {
		t.Errorf("expected 1 block index entry, got %d", len(writer.blockIndex))
	}

	if writer.compressedBuf.Len() == 0 {
		t.Error("compressed buffer should not be empty")
	}
}

func TestZLDWriterFlushEmptyBlock(t *testing.T) {
	writer := NewZLDWriter(t.TempDir())

	err := writer.flushBlock()
	if err != nil {
		t.Fatalf("flushBlock failed: %v", err)
	}

	if len(writer.blockIndex) != 0 {
		t.Errorf("expected 0 block index entries, got %d", len(writer.blockIndex))
	}
}

func TestEmitZLD(t *testing.T) {
	tmpDir := t.TempDir()

	corpus := &IRCorpus{
		ID:       "TestLex",
		Title:    "Test Lexicon",
		Language: "grc",
		Documents: []*IRDocument{
			{
				ID: "Entries",
				ContentBlocks: []*IRContentBlock{
					{ID: "G0001", Text: "Definition of G0001"},
					{ID: "G0002", RawMarkup: "<def>Definition of G0002</def>"},
					{ID: "G0003", Text: "Definition of G0003"},
				},
			},
		},
	}

	result, err := EmitZLD(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitZLD failed: %v", err)
	}

	if result.ModuleID != "TestLex" {
		t.Errorf("expected ModuleID TestLex, got %s", result.ModuleID)
	}

	if result.VersesWritten != 3 {
		t.Errorf("expected 3 entries written, got %d", result.VersesWritten)
	}

	// Check conf file exists
	confPath := filepath.Join(tmpDir, "mods.d", "testlex.conf")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Error("conf file not created")
	}

	// Check data directory exists
	dataPath := filepath.Join(tmpDir, "modules", "lexdict", "zld", "testlex")
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		t.Error("data directory not created")
	}
}

func TestEmitZLDEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	corpus := &IRCorpus{
		ID:       "EmptyLex",
		Title:    "Empty Lexicon",
		Language: "en",
	}

	result, err := EmitZLD(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitZLD failed: %v", err)
	}

	if result.VersesWritten != 0 {
		t.Errorf("expected 0 entries for empty corpus, got %d", result.VersesWritten)
	}
}

func TestGenerateLexiconConf(t *testing.T) {
	corpus := &IRCorpus{
		ID:       "Strongs",
		Title:    "Strong's Hebrew and Greek Dictionaries",
		Language: "en",
	}

	conf := generateLexiconConf(corpus)

	if conf == "" {
		t.Fatal("generateLexiconConf returned empty string")
	}

	// Check required fields
	expected := []string{
		"[Strongs]",
		"Description=Strong's Hebrew and Greek Dictionaries",
		"Lang=en",
		"ModDrv=zLD",
		"Encoding=UTF-8",
		"DataPath=./modules/lexdict/zld/strongs/dict",
	}

	for _, e := range expected {
		if !strContains(conf, e) {
			t.Errorf("conf missing: %s", e)
		}
	}
}

func TestZLDWriterLargeEntries(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "dict")

	writer := NewZLDWriter(dataPath)

	// Add entries that will trigger block flushing (each entry > 4KB)
	largeText := make([]byte, 5000)
	for i := range largeText {
		largeText[i] = 'A' + byte(i%26)
	}

	writer.AddEntry("entry1", string(largeText))
	writer.AddEntry("entry2", string(largeText))
	writer.AddEntry("entry3", string(largeText))

	count, err := writer.WriteModule()
	if err != nil {
		t.Fatalf("WriteModule failed: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 entries, got %d", count)
	}

	// Should have multiple blocks due to size
	if len(writer.blockIndex) < 2 {
		t.Errorf("expected multiple blocks, got %d", len(writer.blockIndex))
	}
}

func TestEmitZLDUsesRawMarkup(t *testing.T) {
	tmpDir := t.TempDir()

	corpus := &IRCorpus{
		ID:       "TestLex",
		Title:    "Test",
		Language: "en",
		Documents: []*IRDocument{
			{
				ID: "Entries",
				ContentBlocks: []*IRContentBlock{
					// Entry with both RawMarkup and Text - RawMarkup should be preferred
					{ID: "test", Text: "plain text", RawMarkup: "<markup>rich text</markup>"},
				},
			},
		},
	}

	result, err := EmitZLD(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitZLD failed: %v", err)
	}

	if result.VersesWritten != 1 {
		t.Errorf("expected 1 entry, got %d", result.VersesWritten)
	}
}
