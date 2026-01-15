package tei

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.tei" {
		t.Errorf("Expected PluginID 'format.tei', got %s", m.PluginID)
	}
	if m.Kind != "format" {
		t.Errorf("Expected Kind 'format', got %s", m.Kind)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", m.Version)
	}
}

func TestRegister(t *testing.T) {
	// Register should not panic
	Register()
}

func TestDetect_TEIFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	file := filepath.Join(tmpDir, "test.tei")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(file)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "tei" {
		t.Errorf("Expected format 'tei', got %s", result.Format)
	}
}

func TestDetect_NonTEIFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	file := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(file)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-TEI file to not be detected")
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

func TestDetect_NonExistentFile(t *testing.T) {
	h := &Handler{}

	result, err := h.Detect("/nonexistent/file.tei")
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-existent file to not be detected")
	}
	if !strings.Contains(result.Reason, "cannot stat") {
		t.Errorf("Expected reason to mention stat error, got: %s", result.Reason)
	}
}

func TestIngest_TEIFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	file := filepath.Join(tmpDir, "test.tei")
	content := []byte("test TEI content")
	if err := os.WriteFile(file, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	result, err := h.Ingest(file, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "test" {
		t.Errorf("Expected artifact ID 'test', got %s", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected blob hash to be set")
	}
	if result.Metadata["format"] != "tei" {
		t.Errorf("Expected format 'tei', got %s", result.Metadata["format"])
	}
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

func TestIngest_NonExistentFile(t *testing.T) {
	h := &Handler{}
	outputDir := t.TempDir()

	_, err := h.Ingest("/nonexistent/file.tei", outputDir)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected 'failed to read file' error, got: %v", err)
	}
}

func TestEnumerate_TEIFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	file := filepath.Join(tmpDir, "test.tei")
	content := []byte("test content")
	if err := os.WriteFile(file, content, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Enumerate(file)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.tei" {
		t.Errorf("Expected path 'test.tei', got %s", entry.Path)
	}
	if entry.IsDir {
		t.Error("Expected entry to not be a directory")
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
}

func TestEnumerate_NonExistentFile(t *testing.T) {
	h := &Handler{}

	_, err := h.Enumerate("/nonexistent/file.tei")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to stat") {
		t.Errorf("Expected 'failed to stat' error, got: %v", err)
	}
}

func TestExtractIR(t *testing.T) {
	h := &Handler{}

	_, err := h.ExtractIR("test.tei", "/output")
	if err == nil {
		t.Error("Expected error for unsupported ExtractIR")
	}
	if !strings.Contains(err.Error(), "does not support IR extraction") {
		t.Errorf("Expected IR extraction error, got: %v", err)
	}
}

func TestEmitNative(t *testing.T) {
	h := &Handler{}

	_, err := h.EmitNative("ir.json", "/output")
	if err == nil {
		t.Error("Expected error for unsupported EmitNative")
	}
	if !strings.Contains(err.Error(), "does not support native emission") {
		t.Errorf("Expected native emission error, got: %v", err)
	}
}
