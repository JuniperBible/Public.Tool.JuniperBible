package oshb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.oshb" {
		t.Errorf("Expected PluginID 'format.oshb', got %s", m.PluginID)
	}
	if m.Kind != "format" {
		t.Errorf("Expected Kind 'format', got %s", m.Kind)
	}
}

func TestRegister(t *testing.T) {
	Register()
}

func TestDetect_OSHBFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	oshbFile := filepath.Join(tmpDir, "hebrew.oshb")
	if err := os.WriteFile(oshbFile, []byte("OSHB content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(oshbFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected OSHB file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "oshb" {
		t.Errorf("Expected format 'oshb', got %s", result.Format)
	}
}

func TestDetect_NonOSHBFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	txtFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(txtFile, []byte("not OSHB"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(txtFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-OSHB file to not be detected")
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

	result, err := h.Detect("/nonexistent/file.oshb")
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

	oshbFile := filepath.Join(tmpDir, "hebrew.oshb")
	content := []byte("OSHB content")
	if err := os.WriteFile(oshbFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	result, err := h.Ingest(oshbFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "hebrew" {
		t.Errorf("Expected artifact ID 'hebrew', got %s", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected blob hash to be set")
	}
	if result.Metadata["format"] != "oshb" {
		t.Errorf("Expected format 'oshb', got %s", result.Metadata["format"])
	}

	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

func TestIngest_NonExistent(t *testing.T) {
	h := &Handler{}

	_, err := h.Ingest("/nonexistent/file.oshb", t.TempDir())
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestEnumerate(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	oshbFile := filepath.Join(tmpDir, "hebrew.oshb")
	content := []byte("OSHB content")
	if err := os.WriteFile(oshbFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Enumerate(oshbFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}
	if result.Entries[0].Path != "hebrew.oshb" {
		t.Errorf("Expected path 'hebrew.oshb', got %s", result.Entries[0].Path)
	}
}

func TestEnumerate_NonExistent(t *testing.T) {
	h := &Handler{}

	_, err := h.Enumerate("/nonexistent/file.oshb")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestExtractIR(t *testing.T) {
	h := &Handler{}

	_, err := h.ExtractIR("hebrew.oshb", "/output")
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
