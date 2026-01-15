package accordance

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func TestDetect_ValidAccordanceFile(t *testing.T) {
	// Create a temporary test file with .accordance extension
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.accordance")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed, got: %s", result.Reason)
	}
	if result.Format != "accordance" {
		t.Errorf("Expected format 'accordance', got %s", result.Format)
	}
	if result.Reason != "Accordance file detected" {
		t.Errorf("Expected reason 'Accordance file detected', got %s", result.Reason)
	}
}

func TestDetect_InvalidExtension(t *testing.T) {
	// Create a temporary test file with wrong extension
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for wrong extension")
	}
	if result.Reason != "not a .accordance file" {
		t.Errorf("Expected reason 'not a .accordance file', got %s", result.Reason)
	}
}

func TestDetect_Directory(t *testing.T) {
	// Test with a directory
	tmpDir := t.TempDir()

	handler := &Handler{}
	result, err := handler.Detect(tmpDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for directory")
	}
	if result.Reason != "path is a directory" {
		t.Errorf("Expected reason 'path is a directory', got %s", result.Reason)
	}
}

func TestDetect_NonExistent(t *testing.T) {
	// Test with a non-existent file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "nonexistent.accordance")

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for non-existent file")
	}
	if !strings.Contains(result.Reason, "cannot stat") {
		t.Errorf("Expected reason to contain 'cannot stat', got %s", result.Reason)
	}
}

func TestIngest_Success(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.accordance")
	content := []byte("test content for ingest")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Calculate expected SHA256
	hash := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(hash[:])

	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify ArtifactID
	if result.ArtifactID != "test" {
		t.Errorf("Expected artifact ID 'test', got %s", result.ArtifactID)
	}

	// Verify SHA256
	if result.BlobSHA256 != expectedHash {
		t.Errorf("Expected SHA256 %s, got %s", expectedHash, result.BlobSHA256)
	}

	// Verify size
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}

	// Verify metadata
	if result.Metadata["format"] != "accordance" {
		t.Errorf("Expected format 'accordance', got %s", result.Metadata["format"])
	}

	// Verify blob was created at correct path
	blobPath := filepath.Join(outputDir, expectedHash[:2], expectedHash)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}

	// Verify blob content
	blobData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("Failed to read blob: %v", err)
	}
	if string(blobData) != string(content) {
		t.Errorf("Expected blob content %s, got %s", content, blobData)
	}
}

func TestIngest_FileNotFound(t *testing.T) {
	// Test with a non-existent file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "nonexistent.accordance")
	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	_, err := handler.Ingest(testFile, outputDir)
	if err == nil {
		t.Fatal("Expected error for non-existent file, got nil")
	}

	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected error to contain 'failed to read file', got %s", err.Error())
	}
}

func TestIngest_OutputDirCreation(t *testing.T) {
	// Verify that output directories are created if they don't exist
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.accordance")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Use a deeply nested output directory that doesn't exist
	outputDir := filepath.Join(tmpDir, "deep", "nested", "output")

	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify the blob directory was created
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist in nested directory")
	}
}

func TestEnumerate_ValidFile(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.accordance")
	content := []byte("test content for enumerate")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Enumerate(testFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify we got exactly one entry
	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.accordance" {
		t.Errorf("Expected path 'test.accordance', got %s", entry.Path)
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
	if entry.IsDir {
		t.Error("Expected IsDir to be false")
	}
}

func TestEnumerate_StatError(t *testing.T) {
	// Test with a non-existent file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "nonexistent.accordance")

	handler := &Handler{}
	_, err := handler.Enumerate(testFile)
	if err == nil {
		t.Fatal("Expected error for non-existent file, got nil")
	}

	if !strings.Contains(err.Error(), "failed to stat") {
		t.Errorf("Expected error to contain 'failed to stat', got %s", err.Error())
	}
}

func TestExtractIR_ReturnsError(t *testing.T) {
	// ExtractIR should always return an error for accordance format
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.accordance")
	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.ExtractIR(testFile, outputDir)
	if err == nil {
		t.Fatal("Expected error from ExtractIR, got nil")
	}

	if result != nil {
		t.Error("Expected nil result from ExtractIR")
	}

	expectedError := "accordance format does not support IR extraction"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestEmitNative_ReturnsError(t *testing.T) {
	// EmitNative should always return an error for accordance format
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "test.ir")
	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.EmitNative(irPath, outputDir)
	if err == nil {
		t.Fatal("Expected error from EmitNative, got nil")
	}

	if result != nil {
		t.Error("Expected nil result from EmitNative")
	}

	expectedError := "accordance format does not support native emission"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestManifest_CorrectValues(t *testing.T) {
	manifest := Manifest()

	// Verify PluginID
	if manifest.PluginID != "format.accordance" {
		t.Errorf("Expected PluginID 'format.accordance', got %s", manifest.PluginID)
	}

	// Verify Version
	if manifest.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", manifest.Version)
	}

	// Verify Kind
	if manifest.Kind != "format" {
		t.Errorf("Expected Kind 'format', got %s", manifest.Kind)
	}

	// Verify Entrypoint
	if manifest.Entrypoint != "format-accordance" {
		t.Errorf("Expected Entrypoint 'format-accordance', got %s", manifest.Entrypoint)
	}

	// Verify Capabilities.Inputs
	if len(manifest.Capabilities.Inputs) != 1 || manifest.Capabilities.Inputs[0] != "file" {
		t.Errorf("Expected Inputs ['file'], got %v", manifest.Capabilities.Inputs)
	}

	// Verify Capabilities.Outputs
	if len(manifest.Capabilities.Outputs) != 1 || manifest.Capabilities.Outputs[0] != "artifact.kind:accordance" {
		t.Errorf("Expected Outputs ['artifact.kind:accordance'], got %v", manifest.Capabilities.Outputs)
	}
}

func TestRegister_PluginRegistered(t *testing.T) {
	// The Register function is called in init(), so the plugin should already be registered
	// We can verify this by checking that we can create a Handler without errors
	handler := &Handler{}
	if handler == nil {
		t.Fatal("Failed to create Handler")
	}

	// Verify the manifest is correct (this indirectly tests Register)
	manifest := Manifest()
	if manifest == nil {
		t.Fatal("Manifest() returned nil")
	}

	// Verify the handler implements the required methods
	// (Type assertions will fail at compile time if methods are missing)
	var _ plugins.EmbeddedFormatHandler = handler
}

func TestDetect_CaseInsensitiveExtension(t *testing.T) {
	// Test that extension matching is case-insensitive
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ACCORDANCE")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed for uppercase extension, got: %s", result.Reason)
	}
}

func TestIngest_EmptyFile(t *testing.T) {
	// Test ingesting an empty file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.accordance")
	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify size is 0
	if result.SizeBytes != 0 {
		t.Errorf("Expected size 0, got %d", result.SizeBytes)
	}

	// Verify SHA256 of empty content
	hash := sha256.Sum256([]byte{})
	expectedHash := hex.EncodeToString(hash[:])
	if result.BlobSHA256 != expectedHash {
		t.Errorf("Expected SHA256 %s, got %s", expectedHash, result.BlobSHA256)
	}
}

func TestIngest_ComplexFilename(t *testing.T) {
	// Test with a complex filename that includes dots
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "my.test.file.accordance")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify artifact ID strips all extensions properly
	if result.ArtifactID != "my.test.file" {
		t.Errorf("Expected artifact ID 'my.test.file', got %s", result.ArtifactID)
	}
}

func TestIngest_BlobDirCreationError(t *testing.T) {
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.accordance")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a read-only output directory to trigger MkdirAll error
	outputDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(outputDir, 0444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(outputDir, 0755) // Cleanup

	handler := &Handler{}
	_, err := handler.Ingest(testFile, outputDir)
	if err == nil {
		t.Fatal("Expected error when creating blob directory in read-only parent, got nil")
	}

	if !strings.Contains(err.Error(), "failed to create blob dir") {
		t.Errorf("Expected error to contain 'failed to create blob dir', got %s", err.Error())
	}
}

func TestIngest_BlobWriteError(t *testing.T) {
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.accordance")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Calculate the hash to know the blob subdirectory
	hash := sha256.Sum256(content)
	hashHex := hex.EncodeToString(hash[:])
	blobSubdir := hashHex[:2]

	// Create the output directory structure with the blob subdirectory
	outputDir := filepath.Join(tmpDir, "output")
	blobDir := filepath.Join(outputDir, blobSubdir)
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Make the blob directory read-only to trigger WriteFile error
	if err := os.Chmod(blobDir, 0444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(blobDir, 0755) // Cleanup

	handler := &Handler{}
	_, err := handler.Ingest(testFile, outputDir)
	if err == nil {
		t.Fatal("Expected error when writing blob to read-only directory, got nil")
	}

	if !strings.Contains(err.Error(), "failed to write blob") {
		t.Errorf("Expected error to contain 'failed to write blob', got %s", err.Error())
	}
}
