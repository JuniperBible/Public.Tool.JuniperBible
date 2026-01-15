package file

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// TestDetect_RegularFile tests detection of a regular file.
func TestDetect_RegularFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed, got: %s", result.Reason)
	}
	if result.Format != "file" {
		t.Errorf("Expected format 'file', got %s", result.Format)
	}
	if result.Reason != "generic file" {
		t.Errorf("Expected reason 'generic file', got %s", result.Reason)
	}
}

// TestDetect_EmptyFile tests detection of an empty file.
func TestDetect_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed for empty file, got: %s", result.Reason)
	}
	if result.Format != "file" {
		t.Errorf("Expected format 'file', got %s", result.Format)
	}
}

// TestDetect_Directory tests that directories are not detected as files.
func TestDetect_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	h := &Handler{}
	result, err := h.Detect(tmpDir)
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

// TestDetect_NonExistent tests detection of a non-existent file.
func TestDetect_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.txt")

	h := &Handler{}
	result, err := h.Detect(nonExistentFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for non-existent file")
	}
	if !strings.Contains(result.Reason, "cannot stat") {
		t.Errorf("Expected reason to mention 'cannot stat', got: %s", result.Reason)
	}
}

// TestIngest_TextFile tests ingesting a simple text file.
func TestIngest_TextFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify artifact ID
	if result.ArtifactID != "test" {
		t.Errorf("Expected artifact ID 'test', got %s", result.ArtifactID)
	}

	// Verify size
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}

	// Verify metadata
	if result.Metadata["format"] != "file" {
		t.Errorf("Expected format 'file', got %s", result.Metadata["format"])
	}

	// Verify hash
	hash := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(hash[:])
	if result.BlobSHA256 != expectedHash {
		t.Errorf("Expected hash %s, got %s", expectedHash, result.BlobSHA256)
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}

	// Verify blob content
	blobData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(blobData) != string(content) {
		t.Errorf("Expected blob content %s, got %s", content, blobData)
	}
}

// TestIngest_BinaryFile tests ingesting a binary file.
func TestIngest_BinaryFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "binary.dat")
	content := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify artifact ID
	if result.ArtifactID != "binary" {
		t.Errorf("Expected artifact ID 'binary', got %s", result.ArtifactID)
	}

	// Verify size
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	blobData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(blobData) != len(content) {
		t.Errorf("Expected blob size %d, got %d", len(content), len(blobData))
	}
	for i, b := range blobData {
		if b != content[i] {
			t.Errorf("Blob content mismatch at byte %d: expected %x, got %x", i, content[i], b)
		}
	}
}

// TestIngest_LargeFile tests ingesting a large file.
func TestIngest_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.dat")
	// Create a 1MB file
	content := make([]byte, 1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify size
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}

	// Verify blob was written with correct size
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	info, err := os.Stat(blobPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(len(content)) {
		t.Errorf("Expected blob file size %d, got %d", len(content), info.Size())
	}
}

// TestIngest_SpecialCharactersInName tests ingesting a file with special characters in the name.
func TestIngest_SpecialCharactersInName(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-file_with.special.chars.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify artifact ID (should strip extension)
	if result.ArtifactID != "test-file_with.special.chars" {
		t.Errorf("Expected artifact ID 'test-file_with.special.chars', got %s", result.ArtifactID)
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

// TestIngest_HashVerification tests that the hash is computed correctly.
func TestIngest_HashVerification(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("known content for hash verification")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	// Compute expected hash
	hash := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(hash[:])

	if result.BlobSHA256 != expectedHash {
		t.Errorf("Expected hash %s, got %s", expectedHash, result.BlobSHA256)
	}

	// Verify blob is stored in correct directory (first 2 chars of hash)
	expectedDir := filepath.Join(outputDir, expectedHash[:2])
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("Expected blob directory %s to exist", expectedDir)
	}

	// Verify blob file name is the full hash
	blobPath := filepath.Join(expectedDir, expectedHash)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Errorf("Expected blob file %s to exist", blobPath)
	}
}

// TestEnumerate_ValidFile tests enumerating a valid file.
func TestEnumerate_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Enumerate(testFile)
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
}

// TestEnumerate_StatError tests enumeration with a non-existent file.
func TestEnumerate_StatError(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.txt")

	h := &Handler{}
	_, err := h.Enumerate(nonExistentFile)
	if err == nil {
		t.Error("Expected error when enumerating non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to stat") {
		t.Errorf("Expected error to mention 'failed to stat', got: %v", err)
	}
}

// TestExtractIR_ReturnsError tests that ExtractIR returns an error.
func TestExtractIR_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	result, err := h.ExtractIR(testFile, outputDir)
	if err == nil {
		t.Error("Expected error from ExtractIR")
	}
	if result != nil {
		t.Error("Expected nil result from ExtractIR")
	}
	if !strings.Contains(err.Error(), "does not support IR extraction") {
		t.Errorf("Expected error to mention 'does not support IR extraction', got: %v", err)
	}
}

// TestEmitNative_ReturnsError tests that EmitNative returns an error.
func TestEmitNative_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "ir.json")
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	result, err := h.EmitNative(irPath, outputDir)
	if err == nil {
		t.Error("Expected error from EmitNative")
	}
	if result != nil {
		t.Error("Expected nil result from EmitNative")
	}
	if !strings.Contains(err.Error(), "does not support native emission") {
		t.Errorf("Expected error to mention 'does not support native emission', got: %v", err)
	}
}

// TestManifest tests the plugin manifest.
func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest.PluginID != "format.file" {
		t.Errorf("Expected plugin ID 'format.file', got %s", manifest.PluginID)
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", manifest.Version)
	}
	if manifest.Kind != "format" {
		t.Errorf("Expected kind 'format', got %s", manifest.Kind)
	}
	if manifest.Entrypoint != "format-file" {
		t.Errorf("Expected entrypoint 'format-file', got %s", manifest.Entrypoint)
	}

	// Verify capabilities
	if len(manifest.Capabilities.Inputs) != 1 || manifest.Capabilities.Inputs[0] != "file" {
		t.Errorf("Expected inputs ['file'], got %v", manifest.Capabilities.Inputs)
	}
	if len(manifest.Capabilities.Outputs) != 1 || manifest.Capabilities.Outputs[0] != "artifact.kind:file" {
		t.Errorf("Expected outputs ['artifact.kind:file'], got %v", manifest.Capabilities.Outputs)
	}
}

// TestRegister tests that the plugin registers correctly.
func TestRegister(t *testing.T) {
	// Call Register to ensure it doesn't panic
	Register()

	// Verify the plugin was registered
	manifest := Manifest()
	plugin := plugins.GetEmbeddedPlugin(manifest.PluginID)
	if plugin == nil {
		t.Fatal("Expected plugin to be registered")
	}

	if plugin.Manifest.PluginID != "format.file" {
		t.Errorf("Expected registered plugin ID 'format.file', got %s", plugin.Manifest.PluginID)
	}

	if plugin.Format == nil {
		t.Error("Expected Format handler to be set")
	}
}

// TestIngest_NonExistentFile tests ingesting a non-existent file.
func TestIngest_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.txt")
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	_, err := h.Ingest(nonExistentFile, outputDir)
	if err == nil {
		t.Error("Expected error when ingesting non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected error to mention 'failed to read file', got: %v", err)
	}
}

// TestIngest_FileWithNoExtension tests ingesting a file without an extension.
func TestIngest_FileWithNoExtension(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "testfile")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify artifact ID (should be the full filename since no extension)
	if result.ArtifactID != "testfile" {
		t.Errorf("Expected artifact ID 'testfile', got %s", result.ArtifactID)
	}
}

// TestIngest_BlobDirectoryCreation tests that blob directories are created correctly.
func TestIngest_BlobDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify blob directory was created
	blobDir := filepath.Join(outputDir, result.BlobSHA256[:2])
	info, err := os.Stat(blobDir)
	if err != nil {
		t.Fatalf("Expected blob directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected blob directory to be a directory")
	}

	// Verify directory permissions
	if info.Mode().Perm() != 0755 {
		t.Errorf("Expected blob directory permissions 0755, got %o", info.Mode().Perm())
	}
}

// TestIngest_BlobDirectoryCreationError tests error handling when blob directory creation fails.
func TestIngest_BlobDirectoryCreationError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file where the output directory should be, to force MkdirAll to fail
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.WriteFile(outputDir, []byte("block"), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	_, err := h.Ingest(testFile, outputDir)
	if err == nil {
		t.Error("Expected error when blob directory creation fails")
	}
	if !strings.Contains(err.Error(), "failed to create blob dir") {
		t.Errorf("Expected error to mention 'failed to create blob dir', got: %v", err)
	}
}

// TestIngest_BlobWriteError tests error handling when blob file write fails.
func TestIngest_BlobWriteError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	h := &Handler{}

	// First, compute the expected hash to know where the blob will be written
	hash := sha256.Sum256(content)
	hashHex := hex.EncodeToString(hash[:])

	// Create the blob directory structure
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a directory where the blob file should be, to force WriteFile to fail
	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.Mkdir(blobPath, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := h.Ingest(testFile, outputDir)
	if err == nil {
		t.Error("Expected error when blob file write fails")
	}
	if !strings.Contains(err.Error(), "failed to write blob") {
		t.Errorf("Expected error to mention 'failed to write blob', got: %v", err)
	}
}
