package zip

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.zip" {
		t.Errorf("Expected PluginID 'format.zip', got %s", m.PluginID)
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

func createZipFile(t *testing.T, dir, filename string) string {
	t.Helper()
	zipPath := filepath.Join(dir, filename)
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// Add a test file
	fw, err := w.Create("test.txt")
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("test content")
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}

	return zipPath
}

func TestDetect_ZipFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	zipPath := createZipFile(t, tmpDir, "test.zip")
	result, err := h.Detect(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected zip file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "zip" {
		t.Errorf("Expected format 'zip', got %s", result.Format)
	}
}

func TestDetect_NonZipFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	txtFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtFile, []byte("not a zip"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(txtFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-zip file to not be detected")
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

	result, err := h.Detect("/nonexistent/file.zip")
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

func TestDetect_InvalidZipContent(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	// Create a file with .zip extension but invalid content
	invalidZip := filepath.Join(tmpDir, "invalid.zip")
	if err := os.WriteFile(invalidZip, []byte("not a valid zip"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(invalidZip)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected invalid zip content to not be detected")
	}
	if !strings.Contains(result.Reason, "wrong magic bytes") {
		t.Errorf("Expected reason to mention wrong magic bytes, got: %s", result.Reason)
	}
}

func TestIngest_ZipFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	zipPath := createZipFile(t, tmpDir, "test.zip")
	outputDir := filepath.Join(tmpDir, "output")

	result, err := h.Ingest(zipPath, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "test" {
		t.Errorf("Expected artifact ID 'test', got %s", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected blob hash to be set")
	}
	if result.Metadata["format"] != "zip" {
		t.Errorf("Expected format 'zip', got %s", result.Metadata["format"])
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

	_, err := h.Ingest("/nonexistent/file.zip", outputDir)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected 'failed to read file' error, got: %v", err)
	}
}

func TestEnumerate_ZipFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	zipPath := createZipFile(t, tmpDir, "test.zip")

	result, err := h.Enumerate(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.txt" {
		t.Errorf("Expected path 'test.txt', got %s", entry.Path)
	}
	if entry.IsDir {
		t.Error("Expected entry to not be a directory")
	}
}

func TestEnumerate_NonExistentFile(t *testing.T) {
	h := &Handler{}

	_, err := h.Enumerate("/nonexistent/file.zip")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to open ZIP") {
		t.Errorf("Expected 'failed to open ZIP' error, got: %v", err)
	}
}

func TestEnumerate_InvalidZip(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	invalidZip := filepath.Join(tmpDir, "invalid.zip")
	if err := os.WriteFile(invalidZip, []byte("not a zip"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := h.Enumerate(invalidZip)
	if err == nil {
		t.Error("Expected error for invalid zip")
	}
	if !strings.Contains(err.Error(), "failed to open ZIP") {
		t.Errorf("Expected ZIP open error, got: %v", err)
	}
}

func TestExtractIR(t *testing.T) {
	h := &Handler{}

	_, err := h.ExtractIR("test.zip", "/output")
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
