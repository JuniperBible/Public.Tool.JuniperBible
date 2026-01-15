package base

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectFile_ExtensionOnly(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := DetectFile(testFile, DetectConfig{
		Extensions: []string{".txt"},
		FormatName: "TXT",
	})
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed, got: %s", result.Reason)
	}
	if result.Format != "TXT" {
		t.Errorf("Expected format TXT, got %s", result.Format)
	}
}

func TestDetectFile_WrongExtension(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xml")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := DetectFile(testFile, DetectConfig{
		Extensions: []string{".txt"},
		FormatName: "TXT",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for wrong extension")
	}
}

func TestDetectFile_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := DetectFile(tmpDir, DetectConfig{
		Extensions: []string{".txt"},
		FormatName: "TXT",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for directory")
	}
	if !strings.Contains(result.Reason, "directory") {
		t.Errorf("Expected reason to mention directory, got: %s", result.Reason)
	}
}

func TestDetectFile_ContentMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.usfm")
	content := "\\id GEN\n\\c 1\n\\v 1 In the beginning..."
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := DetectFile(testFile, DetectConfig{
		Extensions:     []string{".usfm"},
		FormatName:     "USFM",
		ContentMarkers: []string{"\\id ", "\\c ", "\\v "},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection with content markers, got: %s", result.Reason)
	}
}

func TestDetectFile_CustomValidator(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(testFile, []byte(`{"valid": true}`), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := DetectFile(testFile, DetectConfig{
		Extensions:   []string{".json"},
		FormatName:   "JSON",
		CheckContent: true,
		CustomValidator: func(path string, data []byte) (bool, string, error) {
			if strings.Contains(string(data), `"valid"`) {
				return true, "Valid JSON detected", nil
			}
			return false, "", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected custom validator to detect, got: %s", result.Reason)
	}
}

func TestIngestFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	result, err := IngestFile(testFile, outputDir, IngestConfig{
		FormatName: "TXT",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "test" {
		t.Errorf("Expected artifact ID 'test', got %s", result.ArtifactID)
	}
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}
	if result.Metadata["format"] != "TXT" {
		t.Errorf("Expected format TXT, got %s", result.Metadata["format"])
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

func TestIngestFile_CustomArtifactID(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("\\id CUSTOM")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	result, err := IngestFile(testFile, outputDir, IngestConfig{
		FormatName: "TXT",
		ArtifactIDExtractor: func(path string, data []byte) string {
			if idx := strings.Index(string(data), "\\id "); idx >= 0 {
				return strings.TrimSpace(string(data)[idx+4:])
			}
			return filepath.Base(path)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "CUSTOM" {
		t.Errorf("Expected artifact ID 'CUSTOM', got %s", result.ArtifactID)
	}
}

func TestEnumerateFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := EnumerateFile(testFile, map[string]string{
		"format": "TXT",
	})
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
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
	if entry.IsDir {
		t.Error("Expected IsDir to be false")
	}
	if entry.Metadata["format"] != "TXT" {
		t.Errorf("Expected format TXT, got %s", entry.Metadata["format"])
	}
}

func TestReadFileInfo(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	info, err := ReadFileInfo(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if info.Path != testFile {
		t.Errorf("Expected path %s, got %s", testFile, info.Path)
	}
	if string(info.Data) != string(content) {
		t.Errorf("Expected data %s, got %s", content, info.Data)
	}
	if info.Size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), info.Size)
	}
	if info.Extension != ".txt" {
		t.Errorf("Expected extension .txt, got %s", info.Extension)
	}
	if info.Hash == "" {
		t.Error("Expected hash to be computed")
	}
}

func TestWriteOutput(t *testing.T) {
	tmpDir := t.TempDir()
	content := []byte("output content")

	outputPath, err := WriteOutput(tmpDir, "output.json", content)
	if err != nil {
		t.Fatal(err)
	}

	expectedPath := filepath.Join(tmpDir, "output.json")
	if outputPath != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, outputPath)
	}

	// Verify file was written
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(content) {
		t.Errorf("Expected content %s, got %s", content, data)
	}
}

func TestUnsupportedOperationError(t *testing.T) {
	err := UnsupportedOperationError("IR extraction", "SWORD")
	expected := "SWORD format does not support IR extraction"
	if err.Error() != expected {
		t.Errorf("Expected error %s, got %s", expected, err.Error())
	}
}

// TestDetectFile_NonExistentFile tests detection of non-existent files
func TestDetectFile_NonExistentFile(t *testing.T) {
	result, err := DetectFile("/nonexistent/file.txt", DetectConfig{
		Extensions: []string{".txt"},
		FormatName: "TXT",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for non-existent file")
	}
	if !strings.Contains(result.Reason, "cannot stat") {
		t.Errorf("Expected reason to mention stat error, got: %s", result.Reason)
	}
}

// TestDetectFile_ContentMarkersPartialMatch tests when only some markers are found
func TestDetectFile_ContentMarkersPartialMatch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.usfm")
	content := "\\id GEN\n\\c 1\nSome text without verse marker"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := DetectFile(testFile, DetectConfig{
		Extensions:     []string{".usfm"},
		FormatName:     "USFM",
		ContentMarkers: []string{"\\id ", "\\c ", "\\v "},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should fall back to extension match
	if !result.Detected {
		t.Errorf("Expected detection via extension, got: %s", result.Reason)
	}
}

// TestDetectFile_UnreadableFile tests when file cannot be read for content check
func TestDetectFile_UnreadableFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Make file unreadable
	if err := os.Chmod(testFile, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(testFile, 0644)

	result, err := DetectFile(testFile, DetectConfig{
		Extensions:     []string{".txt"},
		FormatName:     "TXT",
		ContentMarkers: []string{"marker"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for unreadable file")
	}
	if !strings.Contains(result.Reason, "cannot read") {
		t.Errorf("Expected reason to mention read error, got: %s", result.Reason)
	}
}

// TestDetectFile_CustomValidatorError tests custom validator returning an error
func TestDetectFile_CustomValidatorError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(testFile, []byte(`{"test": true}`), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := DetectFile(testFile, DetectConfig{
		Extensions:   []string{".json"},
		FormatName:   "JSON",
		CheckContent: true,
		CustomValidator: func(path string, data []byte) (bool, string, error) {
			return false, "", fmt.Errorf("validation failed")
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail when validator returns error")
	}
	if !strings.Contains(result.Reason, "validation error") {
		t.Errorf("Expected reason to mention validation error, got: %s", result.Reason)
	}
}

// TestDetectFile_CustomValidatorRejects tests custom validator rejecting file
func TestDetectFile_CustomValidatorRejects(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(testFile, []byte(`{"invalid": true}`), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := DetectFile(testFile, DetectConfig{
		Extensions:   []string{".json"},
		FormatName:   "JSON",
		CheckContent: true,
		CustomValidator: func(path string, data []byte) (bool, string, error) {
			return false, "invalid content", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should fall back to extension match
	if !result.Detected {
		t.Errorf("Expected detection via extension, got: %s", result.Reason)
	}
}

// TestIngestFile_ReadError tests handling of file read errors
func TestIngestFile_ReadError(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	_, err := IngestFile("/nonexistent/file.txt", outputDir, IngestConfig{
		FormatName: "TXT",
	})
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected 'failed to read file' error, got: %v", err)
	}
}

// TestIngestFile_MkdirError tests handling of directory creation errors
func TestIngestFile_MkdirError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file where we need a directory
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.WriteFile(outputDir, []byte("block"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := IngestFile(testFile, outputDir, IngestConfig{
		FormatName: "TXT",
	})
	if err == nil {
		t.Error("Expected error when blob dir cannot be created")
	}
	if !strings.Contains(err.Error(), "failed to create blob dir") {
		t.Errorf("Expected 'failed to create blob dir' error, got: %v", err)
	}
}

// TestIngestFile_WriteBlobError tests handling of blob write errors
func TestIngestFile_WriteBlobError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	// Pre-create the blob directory structure but make it unwritable
	hash := sha256.Sum256(content)
	hashHex := hex.EncodeToString(hash[:])
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blobDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(blobDir, 0755)

	_, err := IngestFile(testFile, outputDir, IngestConfig{
		FormatName: "TXT",
	})
	if err == nil {
		t.Error("Expected error when blob cannot be written")
	}
	if !strings.Contains(err.Error(), "failed to write blob") {
		t.Errorf("Expected 'failed to write blob' error, got: %v", err)
	}
}

// TestIngestFile_AdditionalMetadata tests adding custom metadata
func TestIngestFile_AdditionalMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	result, err := IngestFile(testFile, outputDir, IngestConfig{
		FormatName: "TXT",
		AdditionalMetadata: map[string]string{
			"custom_key": "custom_value",
			"version":    "1.0",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Metadata["custom_key"] != "custom_value" {
		t.Errorf("Expected custom_key metadata, got: %v", result.Metadata)
	}
	if result.Metadata["version"] != "1.0" {
		t.Errorf("Expected version metadata, got: %v", result.Metadata)
	}
	if result.Metadata["format"] != "TXT" {
		t.Errorf("Expected format TXT, got %s", result.Metadata["format"])
	}
}

// TestEnumerateFile_Error tests handling of stat errors
func TestEnumerateFile_Error(t *testing.T) {
	_, err := EnumerateFile("/nonexistent/file.txt", map[string]string{})
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to stat") {
		t.Errorf("Expected 'failed to stat' error, got: %v", err)
	}
}

// TestReadFileInfo_Error tests handling of read errors
func TestReadFileInfo_Error(t *testing.T) {
	_, err := ReadFileInfo("/nonexistent/file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected 'failed to read file' error, got: %v", err)
	}
}

// TestWriteOutput_Error tests handling of write errors
func TestWriteOutput_Error(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a read-only directory
	outputDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(outputDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(outputDir, 0755)

	_, err := WriteOutput(outputDir, "test.txt", []byte("content"))
	if err == nil {
		t.Error("Expected error when writing to read-only directory")
	}
	if !strings.Contains(err.Error(), "failed to write file") {
		t.Errorf("Expected 'failed to write file' error, got: %v", err)
	}
}
