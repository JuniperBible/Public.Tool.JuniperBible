package onlinebible

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.onlinebible" {
		t.Errorf("Expected PluginID 'format.onlinebible', got %s", m.PluginID)
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

func TestDetect_OLBFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	olbFile := filepath.Join(tmpDir, "bible.olb")
	if err := os.WriteFile(olbFile, []byte("Online Bible content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(olbFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected OLB file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "onlinebible" {
		t.Errorf("Expected format 'onlinebible', got %s", result.Format)
	}
}

func TestDetect_NonOLBFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	txtFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(txtFile, []byte("not OLB"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(txtFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-OLB file to not be detected")
	}
	if !strings.Contains(result.Reason, ".olb") {
		t.Errorf("Expected reason to mention .olb, got: %s", result.Reason)
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

	result, err := h.Detect("/nonexistent/file.olb")
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

func TestDetect_UppercaseExtension(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	olbFile := filepath.Join(tmpDir, "bible.OLB")
	if err := os.WriteFile(olbFile, []byte("Online Bible content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(olbFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected uppercase .OLB file to be detected, reason: %s", result.Reason)
	}
}

func TestIngest_OLBFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	olbFile := filepath.Join(tmpDir, "kjv.olb")
	content := []byte("Online Bible KJV data")
	if err := os.WriteFile(olbFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	result, err := h.Ingest(olbFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "kjv" {
		t.Errorf("Expected artifact ID 'kjv', got %s", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected blob hash to be set")
	}
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}
	if result.Metadata["format"] != "onlinebible" {
		t.Errorf("Expected format 'onlinebible', got %s", result.Metadata["format"])
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

	_, err := h.Ingest("/nonexistent/file.olb", outputDir)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected 'failed to read file' error, got: %v", err)
	}
}

func TestIngest_BlobDirError(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	olbFile := filepath.Join(tmpDir, "test.olb")
	if err := os.WriteFile(olbFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file where we need a directory
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.WriteFile(outputDir, []byte("block"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := h.Ingest(olbFile, outputDir)
	if err == nil {
		t.Error("Expected error when blob dir cannot be created")
	}
	if !strings.Contains(err.Error(), "failed to create blob dir") {
		t.Errorf("Expected 'failed to create blob dir' error, got: %v", err)
	}
}

func TestEnumerate_OLBFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	olbFile := filepath.Join(tmpDir, "bible.olb")
	content := []byte("Online Bible content")
	if err := os.WriteFile(olbFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Enumerate(olbFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "bible.olb" {
		t.Errorf("Expected path 'bible.olb', got %s", entry.Path)
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
	if entry.IsDir {
		t.Error("Expected entry to not be a directory")
	}
}

func TestEnumerate_NonExistentFile(t *testing.T) {
	h := &Handler{}

	_, err := h.Enumerate("/nonexistent/file.olb")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to stat") {
		t.Errorf("Expected 'failed to stat' error, got: %v", err)
	}
}

func TestExtractIR(t *testing.T) {
	h := &Handler{}

	_, err := h.ExtractIR("bible.olb", "/output")
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
