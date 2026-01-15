package pdb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.pdb" {
		t.Errorf("Expected PluginID 'format.pdb', got %s", m.PluginID)
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

func TestDetect_PDBFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	pdbFile := filepath.Join(tmpDir, "bible.pdb")
	if err := os.WriteFile(pdbFile, []byte("PDB content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(pdbFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected PDB file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "pdb" {
		t.Errorf("Expected format 'pdb', got %s", result.Format)
	}
}

func TestDetect_NonPDBFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	txtFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(txtFile, []byte("not PDB"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(txtFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-PDB file to not be detected")
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

	result, err := h.Detect("/nonexistent/file.pdb")
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

	pdbFile := filepath.Join(tmpDir, "bible.pdb")
	content := []byte("PDB content")
	if err := os.WriteFile(pdbFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	result, err := h.Ingest(pdbFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "bible" {
		t.Errorf("Expected artifact ID 'bible', got %s", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected blob hash to be set")
	}
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}
	if result.Metadata["format"] != "pdb" {
		t.Errorf("Expected format 'pdb', got %s", result.Metadata["format"])
	}

	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

func TestIngest_NonExistent(t *testing.T) {
	h := &Handler{}

	_, err := h.Ingest("/nonexistent/file.pdb", t.TempDir())
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestEnumerate(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	pdbFile := filepath.Join(tmpDir, "bible.pdb")
	content := []byte("PDB content")
	if err := os.WriteFile(pdbFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Enumerate(pdbFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "bible.pdb" {
		t.Errorf("Expected path 'bible.pdb', got %s", entry.Path)
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
}

func TestEnumerate_NonExistent(t *testing.T) {
	h := &Handler{}

	_, err := h.Enumerate("/nonexistent/file.pdb")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestExtractIR(t *testing.T) {
	h := &Handler{}

	_, err := h.ExtractIR("bible.pdb", "/output")
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
