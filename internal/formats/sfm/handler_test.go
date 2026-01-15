package sfm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.sfm" {
		t.Errorf("Expected PluginID 'format.sfm', got %s", m.PluginID)
	}
	if m.Kind != "format" {
		t.Errorf("Expected Kind 'format', got %s", m.Kind)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", m.Version)
	}
}

func TestRegister(t *testing.T) {
	Register()
}

func TestDetect_SFMFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	sfmFile := filepath.Join(tmpDir, "genesis.sfm")
	if err := os.WriteFile(sfmFile, []byte("\\id GEN\n\\c 1\n\\v 1 In the beginning"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(sfmFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected SFM file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "sfm" {
		t.Errorf("Expected format 'sfm', got %s", result.Format)
	}
}

func TestDetect_NonSFMFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	txtFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(txtFile, []byte("not SFM"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(txtFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-SFM file to not be detected")
	}
}

func TestDetect_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	result, err := h.Detect(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected directory to not be detected")
	}
	if !strings.Contains(result.Reason, "directory") {
		t.Errorf("Expected reason to mention directory, got: %s", result.Reason)
	}
}

func TestDetect_NonExistent(t *testing.T) {
	h := &Handler{}

	result, err := h.Detect("/nonexistent/file.sfm")
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-existent file to not be detected")
	}
}

func TestIngest(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	sfmFile := filepath.Join(tmpDir, "genesis.sfm")
	content := []byte("\\id GEN\n\\c 1\n\\v 1 In the beginning")
	if err := os.WriteFile(sfmFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	result, err := h.Ingest(sfmFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "genesis" {
		t.Errorf("Expected artifact ID 'genesis', got %s", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected blob hash to be set")
	}
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}
	if result.Metadata["format"] != "sfm" {
		t.Errorf("Expected format 'sfm', got %s", result.Metadata["format"])
	}

	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

func TestIngest_NonExistent(t *testing.T) {
	h := &Handler{}

	_, err := h.Ingest("/nonexistent/file.sfm", t.TempDir())
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestEnumerate(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	sfmFile := filepath.Join(tmpDir, "genesis.sfm")
	content := []byte("\\id GEN\n\\c 1\n\\v 1 In the beginning")
	if err := os.WriteFile(sfmFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Enumerate(sfmFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "genesis.sfm" {
		t.Errorf("Expected path 'genesis.sfm', got %s", entry.Path)
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
}

func TestEnumerate_NonExistent(t *testing.T) {
	h := &Handler{}

	_, err := h.Enumerate("/nonexistent/file.sfm")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestExtractIR(t *testing.T) {
	h := &Handler{}

	_, err := h.ExtractIR("genesis.sfm", "/output")
	if err == nil {
		t.Error("Expected error for unsupported ExtractIR")
	}
}

func TestEmitNative(t *testing.T) {
	h := &Handler{}

	_, err := h.EmitNative("ir.json", "/output")
	if err == nil {
		t.Error("Expected error for unsupported EmitNative")
	}
}
