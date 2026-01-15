package swordpure

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestNewZTextWriter(t *testing.T) {
	vers, err := NewVersification(VersKJV)
	if err != nil {
		t.Fatalf("NewVersification failed: %v", err)
	}

	writer := NewZTextWriter("/tmp/test", vers)
	if writer == nil {
		t.Fatal("NewZTextWriter returned nil")
	}

	if writer.dataPath != "/tmp/test" {
		t.Errorf("dataPath = %q, want %q", writer.dataPath, "/tmp/test")
	}

	if writer.vers != vers {
		t.Error("versification not set correctly")
	}
}

func TestZTextWriterWriteModule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vers, err := NewVersification(VersKJV)
	if err != nil {
		t.Fatalf("NewVersification failed: %v", err)
	}

	// Create a minimal IR corpus
	corpus := &IRCorpus{
		ID:             "TEST",
		Versification:  "KJV",
		Documents: []*IRDocument{
			{
				ID: "Gen",
				ContentBlocks: []*IRContentBlock{
					{
						ID:   "Gen.1.1",
						Text: "In the beginning God created the heaven and the earth.",
					},
					{
						ID:   "Gen.1.2",
						Text: "And the earth was without form, and void.",
					},
				},
			},
		},
	}

	writer := NewZTextWriter(tmpDir, vers)
	count, err := writer.WriteModule(corpus)
	if err != nil {
		t.Fatalf("WriteModule failed: %v", err)
	}

	if count != 2 {
		t.Errorf("WriteModule wrote %d verses, want 2", count)
	}

	// Verify files were created
	otBzz := filepath.Join(tmpDir, "ot.bzz")
	otBzs := filepath.Join(tmpDir, "ot.bzs")
	otBzv := filepath.Join(tmpDir, "ot.bzv")

	for _, path := range []string{otBzz, otBzs, otBzv} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("file not created: %s", path)
		}
	}
}

func TestZTextWriterEmptyCorpus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vers, err := NewVersification(VersKJV)
	if err != nil {
		t.Fatalf("NewVersification failed: %v", err)
	}

	corpus := &IRCorpus{
		ID:            "EMPTY",
		Versification: "KJV",
		Documents:     []*IRDocument{},
	}

	writer := NewZTextWriter(tmpDir, vers)
	count, err := writer.WriteModule(corpus)
	if err != nil {
		t.Fatalf("WriteModule failed: %v", err)
	}

	if count != 0 {
		t.Errorf("WriteModule wrote %d verses, want 0", count)
	}
}

func TestEmitZText(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := &IRCorpus{
		ID:             "TESTBIBLE",
		Versification:  "KJV",
		Title:          "Test Bible",
		Language:       "en",
		Documents: []*IRDocument{
			{
				ID: "Gen",
				ContentBlocks: []*IRContentBlock{
					{
						ID:   "Gen.1.1",
						Text: "In the beginning.",
					},
				},
			},
		},
	}

	result, err := EmitZText(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitZText failed: %v", err)
	}

	if result.ModuleID != "TESTBIBLE" {
		t.Errorf("ModuleID = %q, want %q", result.ModuleID, "TESTBIBLE")
	}

	if result.VersesWritten != 1 {
		t.Errorf("VersesWritten = %d, want 1", result.VersesWritten)
	}

	// Verify directory structure
	modsDir := filepath.Join(tmpDir, "mods.d")
	if _, err := os.Stat(modsDir); os.IsNotExist(err) {
		t.Error("mods.d directory not created")
	}

	confPath := filepath.Join(modsDir, "testbible.conf")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Error("conf file not created")
	}

	dataPath := filepath.Join(tmpDir, "modules", "texts", "ztext", "testbible")
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		t.Error("data directory not created")
	}
}

func TestUpdateConfDataPath(t *testing.T) {
	confContent := `[TestModule]
DataPath=./old/path/
ModDrv=zText
Description=Test
`

	updated := updateConfDataPath(confContent, "NEWMOD")

	if updated == confContent {
		t.Error("updateConfDataPath did not modify content")
	}

	// Should contain new path
	expected := "DataPath=./modules/texts/ztext/newmod/"
	if !contains(updated, expected) {
		t.Errorf("updateConfDataPath did not update path correctly, got:\n%s", updated)
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"line1", []string{"line1"}},
		{"line1\nline2", []string{"line1", "line2"}},
		{"line1\nline2\nline3", []string{"line1", "line2", "line3"}},
	}

	for _, tt := range tests {
		got := splitLines(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitLines(%q) returned %d lines, want %d", tt.input, len(got), len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitLines(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestJoinLines(t *testing.T) {
	tests := []struct {
		input []string
		want  string
	}{
		{[]string{}, ""},
		{[]string{"line1"}, "line1"},
		{[]string{"line1", "line2"}, "line1\nline2"},
		{[]string{"line1", "line2", "line3"}, "line1\nline2\nline3"},
	}

	for _, tt := range tests {
		got := joinLines(tt.input)
		if got != tt.want {
			t.Errorf("joinLines(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStringToLower(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"abc", "abc"},
		{"ABC", "abc"},
		{"AbC123", "abc123"},
		{"TESTBIBLE", "testbible"},
	}

	for _, tt := range tests {
		got := stringToLower(tt.input)
		if got != tt.want {
			t.Errorf("stringToLower(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestZTextWriterFlushBlock(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vers, err := NewVersification(VersKJV)
	if err != nil {
		t.Fatalf("NewVersification failed: %v", err)
	}

	writer := NewZTextWriter(tmpDir, vers)

	// Add some data to current block
	testData := []byte("Test verse data")
	writer.currentBlock.Write(testData)
	writer.currentBlockSize = uint32(len(testData))

	// Flush the block
	if err := writer.flushBlock(); err != nil {
		t.Fatalf("flushBlock failed: %v", err)
	}

	// Check that block entry was added
	if len(writer.blockEntries) != 1 {
		t.Errorf("flushBlock did not add block entry, got %d entries", len(writer.blockEntries))
	}

	// Check that compressed data was added
	if writer.compressedBuf.Len() == 0 {
		t.Error("flushBlock did not add compressed data")
	}

	// Check that current block was reset
	if writer.currentBlock.Len() != 0 {
		t.Error("flushBlock did not reset current block")
	}

	if writer.currentBlockNum != 1 {
		t.Errorf("currentBlockNum = %d, want 1", writer.currentBlockNum)
	}
}

func TestZTextWriterWriteFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	vers, err := NewVersification(VersKJV)
	if err != nil {
		t.Fatalf("NewVersification failed: %v", err)
	}

	writer := NewZTextWriter(tmpDir, vers)

	// Add test data
	testData := []byte("Test data")
	writer.currentBlock.Write(testData)
	writer.currentBlockSize = uint32(len(testData))
	writer.flushBlock()

	// Add a verse entry
	writer.addVerseEntry(0, 0, uint16(len(testData)))

	// Write files
	if err := writer.writeFiles("test"); err != nil {
		t.Fatalf("writeFiles failed: %v", err)
	}

	// Verify files exist
	bzzPath := filepath.Join(tmpDir, "test.bzz")
	bzsPath := filepath.Join(tmpDir, "test.bzs")
	bzvPath := filepath.Join(tmpDir, "test.bzv")

	for _, path := range []string{bzzPath, bzsPath, bzvPath} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("file not created: %s", path)
		}
	}

	// Verify bzs content
	bzsData, err := os.ReadFile(bzsPath)
	if err != nil {
		t.Fatalf("failed to read bzs: %v", err)
	}

	if len(bzsData) != 12 {
		t.Errorf("bzs size = %d, want 12", len(bzsData))
	}

	// Verify bzv content
	bzvData, err := os.ReadFile(bzvPath)
	if err != nil {
		t.Fatalf("failed to read bzv: %v", err)
	}

	if len(bzvData) != 10 {
		t.Errorf("bzv size = %d, want 10", len(bzvData))
	}

	// Verify verse entry
	blockNum := binary.LittleEndian.Uint32(bzvData[0:4])
	offset := binary.LittleEndian.Uint32(bzvData[4:8])
	size := binary.LittleEndian.Uint16(bzvData[8:10])

	if blockNum != 0 || offset != 0 || size != uint16(len(testData)) {
		t.Errorf("verse entry = {%d, %d, %d}, want {0, 0, %d}", blockNum, offset, size, len(testData))
	}
}

