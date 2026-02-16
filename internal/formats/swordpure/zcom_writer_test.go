package swordpure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewZComWriter(t *testing.T) {
	vers, _ := NewVersification(VersKJV)
	writer := NewZComWriter("/tmp/test", vers)

	if writer == nil {
		t.Fatal("NewZComWriter returned nil")
	}
	if writer.dataPath != "/tmp/test" {
		t.Errorf("expected dataPath /tmp/test, got %s", writer.dataPath)
	}
	if writer.vers != vers {
		t.Error("versification not set correctly")
	}
}

func TestZComWriterWriteModule(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "modules", "comments", "zcom", "testcom")

	vers, err := NewVersification(VersKJV)
	if err != nil {
		t.Fatalf("failed to get versification: %v", err)
	}

	writer := NewZComWriter(dataPath, vers)

	// Create a test corpus with commentary entries
	corpus := &IRCorpus{
		ID:            "TestCom",
		Title:         "Test Commentary",
		Language:      "en",
		Versification: "KJV",
		Documents: []*IRDocument{
			{
				ID: "Genesis",
				ContentBlocks: []*IRContentBlock{
					{ID: "Gen.1.1", Text: "Commentary on Genesis 1:1"},
					{ID: "Gen.1.2", RawMarkup: "<note>Commentary on Genesis 1:2</note>"},
				},
			},
			{
				ID: "Matthew",
				ContentBlocks: []*IRContentBlock{
					{ID: "Matt.1.1", Text: "Commentary on Matthew 1:1"},
				},
			},
		},
	}

	count, err := writer.WriteModule(corpus)
	if err != nil {
		t.Fatalf("WriteModule failed: %v", err)
	}

	if count < 3 {
		t.Errorf("expected at least 3 entries written, got %d", count)
	}

	// Check that files were created
	files := []string{"ot.bzz", "ot.bzs", "ot.bzv", "nt.bzz", "nt.bzs", "nt.bzv"}
	for _, f := range files {
		path := filepath.Join(dataPath, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}
}

func TestZComWriterWriteModuleEmptyCorpus(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "data")

	vers, _ := NewVersification(VersKJV)
	writer := NewZComWriter(dataPath, vers)

	corpus := &IRCorpus{
		ID:       "Empty",
		Language: "en",
	}

	count, err := writer.WriteModule(corpus)
	if err != nil {
		t.Fatalf("WriteModule failed: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 entries for empty corpus, got %d", count)
	}
}

func TestEmitZCom(t *testing.T) {
	tmpDir := t.TempDir()

	corpus := &IRCorpus{
		ID:            "TestCommentary",
		Title:         "Test Commentary Module",
		Language:      "en",
		Versification: "KJV",
		Documents: []*IRDocument{
			{
				ID: "Genesis",
				ContentBlocks: []*IRContentBlock{
					{ID: "Gen.1.1", Text: "In the beginning comment"},
					{ID: "Gen.1.2", Text: "And the earth comment"},
				},
			},
		},
	}

	result, err := EmitZCom(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitZCom failed: %v", err)
	}

	if result.ModuleID != "TestCommentary" {
		t.Errorf("expected ModuleID TestCommentary, got %s", result.ModuleID)
	}

	// Check conf file exists
	confPath := filepath.Join(tmpDir, "mods.d", "testcommentary.conf")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Error("conf file not created")
	}

	// Check data directory exists
	dataPath := filepath.Join(tmpDir, "modules", "comments", "zcom", "testcommentary")
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		t.Error("data directory not created")
	}
}

func TestEmitZComDefaultVersification(t *testing.T) {
	tmpDir := t.TempDir()

	corpus := &IRCorpus{
		ID:       "TestCom",
		Title:    "Test",
		Language: "en",
		// No versification specified - should default to KJV
	}

	result, err := EmitZCom(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitZCom failed: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestEmitZComWithSpecificVersification(t *testing.T) {
	tmpDir := t.TempDir()

	corpus := &IRCorpus{
		ID:            "TestCom",
		Title:         "Test",
		Language:      "en",
		Versification: "NRSV", // Use a valid versification
	}

	result, err := EmitZCom(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitZCom failed: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestGenerateCommentaryConf(t *testing.T) {
	corpus := &IRCorpus{
		ID:            "MHC",
		Title:         "Matthew Henry Commentary",
		Language:      "en",
		Versification: "KJV",
	}

	conf := generateCommentaryConf(corpus)

	if conf == "" {
		t.Fatal("generateCommentaryConf returned empty string")
	}

	// Check required fields
	expected := []string{
		"[MHC]",
		"Description=Matthew Henry Commentary",
		"Lang=en",
		"ModDrv=zCom",
		"Encoding=UTF-8",
		"DataPath=./modules/comments/zcom/mhc/",
		"Versification=KJV",
	}

	for _, e := range expected {
		if !strContains(conf, e) {
			t.Errorf("conf missing: %s", e)
		}
	}
}

func TestGenerateCommentaryConfNoVersification(t *testing.T) {
	corpus := &IRCorpus{
		ID:       "TestCom",
		Title:    "Test Commentary",
		Language: "en",
		// No versification
	}

	conf := generateCommentaryConf(corpus)

	if strContains(conf, "Versification=") {
		t.Error("conf should not contain Versification when not specified")
	}
}

func TestZComWriterFlushBlock(t *testing.T) {
	vers, _ := NewVersification(VersKJV)
	writer := NewZComWriter(t.TempDir(), vers)

	// Write enough data to trigger block flush
	writer.currentBlock.WriteString("test data for compression")

	err := writer.flushBlock()
	if err != nil {
		t.Fatalf("flushBlock failed: %v", err)
	}

	if len(writer.blockEntries) != 1 {
		t.Errorf("expected 1 block entry, got %d", len(writer.blockEntries))
	}

	if writer.compressedBuf.Len() == 0 {
		t.Error("compressed buffer should not be empty")
	}
}

func TestZComWriterFlushEmptyBlock(t *testing.T) {
	vers, _ := NewVersification(VersKJV)
	writer := NewZComWriter(t.TempDir(), vers)

	// Flush empty block should be no-op
	err := writer.flushBlock()
	if err != nil {
		t.Fatalf("flushBlock failed: %v", err)
	}

	if len(writer.blockEntries) != 0 {
		t.Errorf("expected 0 block entries, got %d", len(writer.blockEntries))
	}
}

func TestZComWriterAddEntryEntry(t *testing.T) {
	vers, _ := NewVersification(VersKJV)
	writer := NewZComWriter(t.TempDir(), vers)

	writer.addEntryEntry(1, 100, 50)

	if len(writer.entryEntries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(writer.entryEntries))
	}

	entry := writer.entryEntries[0]
	if entry.BlockNum != 1 {
		t.Errorf("expected BlockNum 1, got %d", entry.BlockNum)
	}
	if entry.Offset != 100 {
		t.Errorf("expected Offset 100, got %d", entry.Offset)
	}
	if entry.Size != 50 {
		t.Errorf("expected Size 50, got %d", entry.Size)
	}
}

func TestZComWriterLargeCorpus(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "data")

	vers, _ := NewVersification(VersKJV)
	writer := NewZComWriter(dataPath, vers)

	// Create corpus with many entries to trigger block flushing
	var blocks []*IRContentBlock
	for i := 1; i <= 100; i++ {
		blocks = append(blocks, &IRContentBlock{
			ID:   "Gen.1." + string(rune('0'+i%10)),
			Text: "This is a longer commentary text that will help fill up blocks " + string(rune('A'+i%26)),
		})
	}

	corpus := &IRCorpus{
		ID:            "LargeCom",
		Language:      "en",
		Versification: "KJV",
		Documents: []*IRDocument{
			{ID: "Genesis", ContentBlocks: blocks},
		},
	}

	count, err := writer.WriteModule(corpus)
	if err != nil {
		t.Fatalf("WriteModule failed: %v", err)
	}

	if count == 0 {
		t.Error("expected some entries written")
	}
}

// strContains checks if s contains substr (local helper to avoid redeclaration)
func strContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
