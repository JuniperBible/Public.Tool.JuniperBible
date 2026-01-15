package swordpure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHandlerManifest(t *testing.T) {
	manifest := Manifest()

	if manifest == nil {
		t.Fatal("Manifest returned nil")
	}

	if manifest.PluginID != "format.sword-pure" {
		t.Errorf("PluginID = %q, want %q", manifest.PluginID, "format.sword-pure")
	}

	if manifest.Kind != "format" {
		t.Errorf("Kind = %q, want %q", manifest.Kind, "format")
	}

	if manifest.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", manifest.Version, "1.0.0")
	}
}

func TestHandlerDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, swordPath := createMockZTextModule(t, tmpDir)

	handler := &Handler{}
	result, err := handler.Detect(swordPath)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if !result.Detected {
		t.Error("Detect should return true for valid SWORD directory")
	}

	if result.Format != "sword-pure" {
		t.Errorf("Format = %q, want %q", result.Format, "sword-pure")
	}

	if result.Reason != "SWORD module directory detected (mods.d/*.conf found)" {
		t.Errorf("Reason = %q", result.Reason)
	}
}

func TestHandlerDetectNotDirectory(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	handler := &Handler{}
	result, err := handler.Detect(tmpFile.Name())
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if result.Detected {
		t.Error("Detect should return false for non-directory")
	}

	if result.Reason != "not a directory" {
		t.Errorf("Reason = %q, want %q", result.Reason, "not a directory")
	}
}

func TestHandlerDetectNoModsDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	handler := &Handler{}
	result, err := handler.Detect(tmpDir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if result.Detected {
		t.Error("Detect should return false for directory without mods.d")
	}

	if result.Reason != "no mods.d directory" {
		t.Errorf("Reason = %q, want %q", result.Reason, "no mods.d directory")
	}
}

func TestHandlerDetectNoConfFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create empty mods.d directory
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Detect(modsDir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if result.Detected {
		t.Error("Detect should return false for directory with no conf files")
	}
}

func TestHandlerIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, swordPath := createMockZTextModule(t, tmpDir)

	outputDir, err := os.MkdirTemp("", "swordpure-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	handler := &Handler{}
	result, err := handler.Ingest(swordPath, outputDir)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if result == nil {
		t.Fatal("Ingest returned nil result")
	}

	if result.ArtifactID == "" {
		t.Error("ArtifactID should not be empty")
	}

	if result.BlobSHA256 == "" {
		t.Error("BlobSHA256 should not be empty")
	}

	if result.SizeBytes == 0 {
		t.Error("SizeBytes should not be zero")
	}

	if result.Metadata["format"] != "sword-pure" {
		t.Errorf("Metadata[format] = %q, want %q", result.Metadata["format"], "sword-pure")
	}
}

func TestHandlerEnumerate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, swordPath := createMockZTextModule(t, tmpDir)

	handler := &Handler{}
	result, err := handler.Enumerate(swordPath)
	if err != nil {
		t.Fatalf("Enumerate failed: %v", err)
	}

	if result == nil {
		t.Fatal("Enumerate returned nil result")
	}

	if len(result.Entries) == 0 {
		t.Error("Enumerate should return at least one entry")
	}

	// Check that entries have paths
	for _, entry := range result.Entries {
		if entry.Path == "" {
			t.Error("entry should have non-empty path")
		}
	}
}

func TestHandlerExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, swordPath := createMockZTextModule(t, tmpDir)

	outputDir, err := os.MkdirTemp("", "swordpure-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	handler := &Handler{}
	result, err := handler.ExtractIR(swordPath, outputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	if result == nil {
		t.Fatal("ExtractIR returned nil result")
	}

	if result.IRPath != outputDir {
		t.Errorf("IRPath = %q, want %q", result.IRPath, outputDir)
	}

	if result.LossClass != "L1" {
		t.Errorf("LossClass = %q, want %q", result.LossClass, "L1")
	}

	// Check that IR file was created
	files, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("failed to read output dir: %v", err)
	}

	if len(files) == 0 {
		t.Error("ExtractIR should create at least one file")
	}
}

func TestHandlerEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock IR corpus file
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

	irPath := filepath.Join(tmpDir, "test.ir.json")
	data, err := json.Marshal(corpus)
	if err != nil {
		t.Fatalf("failed to marshal corpus: %v", err)
	}
	if err := os.WriteFile(irPath, data, 0644); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputDir, err := os.MkdirTemp("", "swordpure-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	handler := &Handler{}
	result, err := handler.EmitNative(irPath, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	if result == nil {
		t.Fatal("EmitNative returned nil result")
	}

	if result.OutputPath != outputDir {
		t.Errorf("OutputPath = %q, want %q", result.OutputPath, outputDir)
	}

	if result.Format != "sword-pure" {
		t.Errorf("Format = %q, want %q", result.Format, "sword-pure")
	}

	if result.LossClass != "L1" {
		t.Errorf("LossClass = %q, want %q", result.LossClass, "L1")
	}

	// Verify output structure was created
	modsDir := filepath.Join(outputDir, "mods.d")
	if _, err := os.Stat(modsDir); os.IsNotExist(err) {
		t.Error("mods.d directory not created")
	}
}

func TestHandlerEmitNativeInvalidIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create invalid IR file
	irPath := filepath.Join(tmpDir, "invalid.ir.json")
	if err := os.WriteFile(irPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputDir, err := os.MkdirTemp("", "swordpure-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	handler := &Handler{}
	_, err = handler.EmitNative(irPath, outputDir)
	if err == nil {
		t.Error("EmitNative should fail for invalid IR file")
	}
}

func TestHandlerEmitNativeNonExistentIR(t *testing.T) {
	outputDir, err := os.MkdirTemp("", "swordpure-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	handler := &Handler{}
	_, err = handler.EmitNative("/nonexistent/ir.json", outputDir)
	if err == nil {
		t.Error("EmitNative should fail for non-existent IR file")
	}
}

func TestHandlerExtractIRNonExistent(t *testing.T) {
	outputDir, err := os.MkdirTemp("", "swordpure-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	handler := &Handler{}
	_, err = handler.ExtractIR("/nonexistent/path", outputDir)
	if err == nil {
		t.Error("ExtractIR should fail for non-existent path")
	}
}

func TestHandlerIngestNonExistent(t *testing.T) {
	outputDir, err := os.MkdirTemp("", "swordpure-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	handler := &Handler{}
	_, err = handler.Ingest("/nonexistent/path", outputDir)
	if err == nil {
		t.Error("Ingest should fail for non-existent path")
	}
}

func TestHandlerEnumerateNonExistent(t *testing.T) {
	handler := &Handler{}
	_, err := handler.Enumerate("/nonexistent/path")
	if err == nil {
		t.Error("Enumerate should fail for non-existent path")
	}
}

func TestHandlerEmitNativeCommentary(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a commentary corpus
	corpus := &IRCorpus{
		ID:            "TESTCOM",
		ModuleType:    "COMMENTARY",
		Versification: "KJV",
		Title:         "Test Commentary",
		Language:      "en",
		Documents: []*IRDocument{
			{
				ID: "Gen",
				ContentBlocks: []*IRContentBlock{
					{
						ID:   "Gen.1.1",
						Text: "Commentary on Genesis 1:1.",
					},
				},
			},
		},
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	data, err := json.Marshal(corpus)
	if err != nil {
		t.Fatalf("failed to marshal corpus: %v", err)
	}
	if err := os.WriteFile(irPath, data, 0644); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputDir, err := os.MkdirTemp("", "swordpure-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	handler := &Handler{}
	result, err := handler.EmitNative(irPath, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	if result == nil {
		t.Fatal("EmitNative returned nil result")
	}
}

func TestHandlerEmitNativeLexicon(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a lexicon corpus
	corpus := &IRCorpus{
		ID:         "TESTLEX",
		ModuleType: "LEXICON",
		Title:      "Test Lexicon",
		Language:   "en",
		Documents: []*IRDocument{
			{
				ID: "Entries",
				ContentBlocks: []*IRContentBlock{
					{
						ID:   "entry1",
						Text: "Definition of entry1.",
					},
				},
			},
		},
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	data, err := json.Marshal(corpus)
	if err != nil {
		t.Fatalf("failed to marshal corpus: %v", err)
	}
	if err := os.WriteFile(irPath, data, 0644); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputDir, err := os.MkdirTemp("", "swordpure-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	handler := &Handler{}
	result, err := handler.EmitNative(irPath, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	if result == nil {
		t.Fatal("EmitNative returned nil result")
	}
}

func TestHandlerEmitNativeGenBook(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a general book corpus
	corpus := &IRCorpus{
		ID:         "TESTBOOK",
		ModuleType: "GENBOOK",
		Title:      "Test Book",
		Language:   "en",
		Documents: []*IRDocument{
			{
				ID: "Entries",
				ContentBlocks: []*IRContentBlock{
					{
						ID:   "chapter1",
						Text: "Chapter 1 content.",
					},
				},
			},
		},
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	data, err := json.Marshal(corpus)
	if err != nil {
		t.Fatalf("failed to marshal corpus: %v", err)
	}
	if err := os.WriteFile(irPath, data, 0644); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputDir, err := os.MkdirTemp("", "swordpure-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	handler := &Handler{}
	result, err := handler.EmitNative(irPath, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	if result == nil {
		t.Fatal("EmitNative returned nil result")
	}
}

func TestHandlerExtractIREncryptedModule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mock encrypted module
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	confContent := `[EncMod]
DataPath=./modules/texts/ztext/encmod/
ModDrv=zText
Description=Encrypted Module
Lang=en
CipherKey=12345
`
	if err := os.WriteFile(filepath.Join(modsDir, "encmod.conf"), []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	outputDir, err := os.MkdirTemp("", "swordpure-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	handler := &Handler{}
	result, err := handler.ExtractIR(tmpDir, outputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	// Should succeed but skip the encrypted module
	if result == nil {
		t.Fatal("ExtractIR returned nil result")
	}
}

func TestHandlerExtractIRNonBibleModule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mock commentary module (non-Bible)
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	confContent := `[TestCom]
DataPath=./modules/comments/zcom/testcom/
ModDrv=zCom
Description=Test Commentary
Lang=en
`
	if err := os.WriteFile(filepath.Join(modsDir, "testcom.conf"), []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	outputDir, err := os.MkdirTemp("", "swordpure-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	handler := &Handler{}
	result, err := handler.ExtractIR(tmpDir, outputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	// Should succeed but skip the non-Bible module
	if result == nil {
		t.Fatal("ExtractIR returned nil result")
	}
}

func TestHandlerExtractIRUncompressedModule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mock uncompressed Bible module
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	confContent := `[RawBible]
DataPath=./modules/texts/rawtext/rawbible/
ModDrv=RawText
Description=Uncompressed Bible
Lang=en
`
	if err := os.WriteFile(filepath.Join(modsDir, "rawbible.conf"), []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	outputDir, err := os.MkdirTemp("", "swordpure-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	handler := &Handler{}
	result, err := handler.ExtractIR(tmpDir, outputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	// Should succeed but skip the uncompressed module
	if result == nil {
		t.Fatal("ExtractIR returned nil result")
	}
}

func TestHandlerDetectNonExistent(t *testing.T) {
	handler := &Handler{}
	result, err := handler.Detect("/nonexistent/path")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if result.Detected {
		t.Error("Detect should return false for non-existent path")
	}
}
