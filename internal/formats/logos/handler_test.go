package logos

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func TestDetect_ValidLogosFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.logos")
	if err := os.WriteFile(testFile, []byte("test logos content"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed, got: %s", result.Reason)
	}
	if result.Format != "logos" {
		t.Errorf("Expected format 'logos', got %s", result.Format)
	}
	if result.Reason != "Logos file detected" {
		t.Errorf("Expected reason 'Logos file detected', got %s", result.Reason)
	}
}

func TestDetect_InvalidExtension(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for wrong extension")
	}
	if result.Reason != "not a .logos file" {
		t.Errorf("Expected reason 'not a .logos file', got %s", result.Reason)
	}
}

func TestDetect_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	handler := &Handler{}
	result, err := handler.Detect(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for directory")
	}
	if result.Reason != "path is a directory" {
		t.Errorf("Expected reason 'path is a directory', got %s", result.Reason)
	}
}

func TestDetect_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.logos")

	handler := &Handler{}
	result, err := handler.Detect(nonExistentFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for non-existent file")
	}
	if !strings.Contains(result.Reason, "cannot stat") {
		t.Errorf("Expected reason to contain 'cannot stat', got %s", result.Reason)
	}
}

func TestDetect_CaseInsensitiveExtension(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.LOGOS")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed for uppercase extension, got: %s", result.Reason)
	}
}

func TestIngest_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.logos")
	content := []byte("test logos content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "test" {
		t.Errorf("Expected artifact ID 'test', got %s", result.ArtifactID)
	}
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}
	if result.Metadata["format"] != "logos" {
		t.Errorf("Expected format 'logos', got %s", result.Metadata["format"])
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

func TestIngest_HashVerification(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.logos")
	content := []byte("test logos content for hash verification")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Compute expected hash
	hash := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(hash[:])

	outputDir := filepath.Join(tmpDir, "output")
	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.BlobSHA256 != expectedHash {
		t.Errorf("Expected hash %s, got %s", expectedHash, result.BlobSHA256)
	}

	// Verify blob is stored in correct subdirectory
	expectedSubdir := expectedHash[:2]
	blobPath := filepath.Join(outputDir, expectedSubdir, expectedHash)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Errorf("Expected blob at %s, but it doesn't exist", blobPath)
	}
}

func TestIngest_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.logos")
	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	_, err := handler.Ingest(nonExistentFile, outputDir)
	if err == nil {
		t.Error("Expected error when ingesting non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected error to contain 'failed to read file', got: %v", err)
	}
}

func TestIngest_CreatesOutputDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.logos")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "nested", "output", "dir")
	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the nested directory structure was created
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob directory to be created")
	}
}

func TestIngest_FailToCreateBlobDir(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.logos")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file where the output directory should be, to cause MkdirAll to fail
	outputDir := filepath.Join(tmpDir, "blocked")
	if err := os.WriteFile(outputDir, []byte("blocking file"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	_, err := handler.Ingest(testFile, outputDir)
	if err == nil {
		t.Error("Expected error when blob directory cannot be created")
	}
	if !strings.Contains(err.Error(), "failed to create blob dir") {
		t.Errorf("Expected error to contain 'failed to create blob dir', got: %v", err)
	}
}

func TestIngest_FailToWriteBlob(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.logos")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Compute the hash to determine where the blob will be written
	hash := sha256.Sum256(content)
	hashHex := hex.EncodeToString(hash[:])

	outputDir := filepath.Join(tmpDir, "output")
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create the blob path as a directory to make WriteFile fail
	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.MkdirAll(blobPath, 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	_, err := handler.Ingest(testFile, outputDir)
	if err == nil {
		t.Error("Expected error when blob file cannot be written")
	}
	if !strings.Contains(err.Error(), "failed to write blob") {
		t.Errorf("Expected error to contain 'failed to write blob', got: %v", err)
	}
}

func TestEnumerate_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.logos")
	content := []byte("test logos content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Enumerate(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.logos" {
		t.Errorf("Expected path 'test.logos', got %s", entry.Path)
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
	if entry.IsDir {
		t.Error("Expected IsDir to be false")
	}
}

func TestEnumerate_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.logos")

	handler := &Handler{}
	_, err := handler.Enumerate(nonExistentFile)
	if err == nil {
		t.Error("Expected error when enumerating non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to stat") {
		t.Errorf("Expected error to contain 'failed to stat', got: %v", err)
	}
}

func TestExtractIR_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.logos")
	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.ExtractIR(testFile, outputDir)
	if err == nil {
		t.Error("Expected error from ExtractIR")
	}
	if result != nil {
		t.Error("Expected nil result from ExtractIR")
	}
	if err.Error() != "logos format does not support IR extraction" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestEmitNative_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "ir.json")
	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.EmitNative(irPath, outputDir)
	if err == nil {
		t.Error("Expected error from EmitNative")
	}
	if result != nil {
		t.Error("Expected nil result from EmitNative")
	}
	if err.Error() != "logos format does not support native emission" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest.PluginID != "format.logos" {
		t.Errorf("Expected PluginID 'format.logos', got %s", manifest.PluginID)
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", manifest.Version)
	}
	if manifest.Kind != "format" {
		t.Errorf("Expected Kind 'format', got %s", manifest.Kind)
	}
	if manifest.Entrypoint != "format-logos" {
		t.Errorf("Expected Entrypoint 'format-logos', got %s", manifest.Entrypoint)
	}

	// Check capabilities
	if len(manifest.Capabilities.Inputs) != 1 {
		t.Fatalf("Expected 1 input capability, got %d", len(manifest.Capabilities.Inputs))
	}
	if manifest.Capabilities.Inputs[0] != "file" {
		t.Errorf("Expected input capability 'file', got %s", manifest.Capabilities.Inputs[0])
	}

	if len(manifest.Capabilities.Outputs) != 1 {
		t.Fatalf("Expected 1 output capability, got %d", len(manifest.Capabilities.Outputs))
	}
	if manifest.Capabilities.Outputs[0] != "artifact.kind:logos" {
		t.Errorf("Expected output capability 'artifact.kind:logos', got %s", manifest.Capabilities.Outputs[0])
	}
}

func TestRegister(t *testing.T) {
	// Test that Register doesn't panic
	// This function is called in init(), but we can call it again
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Register() panicked: %v", r)
		}
	}()

	Register()
}

func TestHandler_ImplementsInterface(t *testing.T) {
	// Verify that Handler implements the EmbeddedFormatHandler interface
	var _ plugins.EmbeddedFormatHandler = (*Handler)(nil)
}
