package rtf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.rtf" {
		t.Errorf("Expected PluginID 'format.rtf', got %s", m.PluginID)
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

func TestDetect_RTFFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	rtfFile := filepath.Join(tmpDir, "document.rtf")
	if err := os.WriteFile(rtfFile, []byte("{\\rtf1\\ansi content}"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(rtfFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected RTF file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "rtf" {
		t.Errorf("Expected format 'rtf', got %s", result.Format)
	}
}

func TestDetect_NonRTFFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	txtFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(txtFile, []byte("not RTF"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(txtFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-RTF file to not be detected")
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

	result, err := h.Detect("/nonexistent/file.rtf")
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

	rtfFile := filepath.Join(tmpDir, "document.rtf")
	content := []byte("{\\rtf1\\ansi content}")
	if err := os.WriteFile(rtfFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	result, err := h.Ingest(rtfFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "document" {
		t.Errorf("Expected artifact ID 'document', got %s", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected blob hash to be set")
	}
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}
	if result.Metadata["format"] != "rtf" {
		t.Errorf("Expected format 'rtf', got %s", result.Metadata["format"])
	}

	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

func TestIngest_NonExistent(t *testing.T) {
	h := &Handler{}

	_, err := h.Ingest("/nonexistent/file.rtf", t.TempDir())
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestEnumerate(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	rtfFile := filepath.Join(tmpDir, "document.rtf")
	content := []byte("{\\rtf1\\ansi content}")
	if err := os.WriteFile(rtfFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Enumerate(rtfFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "document.rtf" {
		t.Errorf("Expected path 'document.rtf', got %s", entry.Path)
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
}

func TestEnumerate_NonExistent(t *testing.T) {
	h := &Handler{}

	_, err := h.Enumerate("/nonexistent/file.rtf")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestExtractIR(t *testing.T) {
	h := &Handler{}

	_, err := h.ExtractIR("document.rtf", "/output")
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
