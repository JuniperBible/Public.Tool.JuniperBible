package flex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func TestDetect_ValidFlexText(t *testing.T) {
	// Create a temporary .flextext file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.flextext")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Detect(tmpFile)

	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if !result.Detected {
		t.Errorf("Expected Detected=true, got false. Reason: %s", result.Reason)
	}
	if result.Format != "flex" {
		t.Errorf("Expected Format='flex', got '%s'", result.Format)
	}
	if result.Reason != "FLEx file detected" {
		t.Errorf("Expected Reason='FLEx file detected', got '%s'", result.Reason)
	}
}

func TestDetect_InvalidExtension(t *testing.T) {
	// Create a temporary file with wrong extension
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Detect(tmpFile)

	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if result.Detected {
		t.Errorf("Expected Detected=false for non-.flextext file, got true")
	}
	if result.Reason != "not a .flextext file" {
		t.Errorf("Expected Reason='not a .flextext file', got '%s'", result.Reason)
	}
}

func TestDetect_Directory(t *testing.T) {
	// Use a temporary directory
	tmpDir := t.TempDir()

	handler := &Handler{}
	result, err := handler.Detect(tmpDir)

	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if result.Detected {
		t.Errorf("Expected Detected=false for directory, got true")
	}
	if result.Reason != "path is a directory" {
		t.Errorf("Expected Reason='path is a directory', got '%s'", result.Reason)
	}
}

func TestDetect_FileNotFound(t *testing.T) {
	// Use a non-existent path
	handler := &Handler{}
	result, err := handler.Detect("/nonexistent/path/file.flextext")

	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if result.Detected {
		t.Errorf("Expected Detected=false for non-existent file, got true")
	}
	if result.Reason == "" {
		t.Errorf("Expected non-empty Reason for stat error")
	}
}

func TestIngest_Success(t *testing.T) {
	// Create a temporary .flextext file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.flextext")
	content := []byte("test flex content")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Create output directory
	outputDir := t.TempDir()

	handler := &Handler{}
	result, err := handler.Ingest(tmpFile, outputDir)

	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Ingest returned nil result")
	}
	if result.ArtifactID != "test" {
		t.Errorf("Expected ArtifactID='test', got '%s'", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Errorf("Expected non-empty BlobSHA256")
	}
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected SizeBytes=%d, got %d", len(content), result.SizeBytes)
	}
	if result.Metadata["format"] != "flex" {
		t.Errorf("Expected Metadata[format]='flex', got '%s'", result.Metadata["format"])
	}

	// Verify blob file was created
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Errorf("Blob file not created at %s", blobPath)
	}

	// Verify blob content
	blobContent, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("Failed to read blob file: %v", err)
	}
	if string(blobContent) != string(content) {
		t.Errorf("Blob content mismatch. Expected '%s', got '%s'", content, blobContent)
	}
}

func TestIngest_FileNotFound(t *testing.T) {
	handler := &Handler{}
	outputDir := t.TempDir()

	result, err := handler.Ingest("/nonexistent/path/file.flextext", outputDir)

	if err == nil {
		t.Errorf("Expected error for non-existent file, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result on error, got %v", result)
	}
}

func TestIngest_BlobDirCreation(t *testing.T) {
	// Create a temporary .flextext file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.flextext")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Create output directory
	outputDir := t.TempDir()

	handler := &Handler{}
	result, err := handler.Ingest(tmpFile, outputDir)

	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	// Verify blob directory was created
	blobDir := filepath.Join(outputDir, result.BlobSHA256[:2])
	if info, err := os.Stat(blobDir); err != nil {
		t.Errorf("Blob directory not created: %v", err)
	} else if !info.IsDir() {
		t.Errorf("Blob path is not a directory")
	}
}

func TestIngest_FailedBlobDirCreation(t *testing.T) {
	// Create a temporary .flextext file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.flextext")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Use an invalid output directory (a file instead of directory)
	outputDir := t.TempDir()
	invalidOutputDir := filepath.Join(outputDir, "notadir")
	if err := os.WriteFile(invalidOutputDir, []byte("blocker"), 0444); err != nil {
		t.Fatalf("Failed to create blocker file: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Ingest(tmpFile, invalidOutputDir)

	if err == nil {
		t.Errorf("Expected error when blob dir creation fails, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result on error, got %v", result)
	}
}

func TestIngest_FailedBlobWrite(t *testing.T) {
	// Create a temporary .flextext file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.flextext")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Create output directory with no write permissions
	outputDir := t.TempDir()
	handler := &Handler{}

	// First, do a normal ingest to find out what the hash would be
	tempOut := t.TempDir()
	tempResult, err := handler.Ingest(tmpFile, tempOut)
	if err != nil {
		t.Fatalf("Failed to get hash: %v", err)
	}

	// Now create the blob directory but make it read-only
	blobDir := filepath.Join(outputDir, tempResult.BlobSHA256[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		t.Fatalf("Failed to create blob dir: %v", err)
	}
	if err := os.Chmod(blobDir, 0444); err != nil {
		t.Fatalf("Failed to chmod blob dir: %v", err)
	}
	defer os.Chmod(blobDir, 0755) // cleanup

	result, err := handler.Ingest(tmpFile, outputDir)

	if err == nil {
		t.Errorf("Expected error when blob write fails, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result on error, got %v", result)
	}
}

func TestEnumerate_ValidFile(t *testing.T) {
	// Create a temporary .flextext file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.flextext")
	content := []byte("test content for enumeration")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Enumerate(tmpFile)

	if err != nil {
		t.Fatalf("Enumerate returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Enumerate returned nil result")
	}
	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.flextext" {
		t.Errorf("Expected Path='test.flextext', got '%s'", entry.Path)
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected SizeBytes=%d, got %d", len(content), entry.SizeBytes)
	}
	if entry.IsDir {
		t.Errorf("Expected IsDir=false, got true")
	}
}

func TestEnumerate_FileNotFound(t *testing.T) {
	handler := &Handler{}
	result, err := handler.Enumerate("/nonexistent/path/file.flextext")

	if err == nil {
		t.Errorf("Expected error for non-existent file, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result on error, got %v", result)
	}
}

func TestExtractIR_ReturnsError(t *testing.T) {
	handler := &Handler{}
	tmpDir := t.TempDir()

	result, err := handler.ExtractIR("/some/path", tmpDir)

	if err == nil {
		t.Errorf("Expected error from ExtractIR, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result from ExtractIR, got %v", result)
	}
	expectedMsg := "flex format does not support IR extraction"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestEmitNative_ReturnsError(t *testing.T) {
	handler := &Handler{}
	tmpDir := t.TempDir()

	result, err := handler.EmitNative("/some/path", tmpDir)

	if err == nil {
		t.Errorf("Expected error from EmitNative, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result from EmitNative, got %v", result)
	}
	expectedMsg := "flex format does not support native emission"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest == nil {
		t.Fatal("Manifest returned nil")
	}
	if manifest.PluginID != "format.flex" {
		t.Errorf("Expected PluginID='format.flex', got '%s'", manifest.PluginID)
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("Expected Version='1.0.0', got '%s'", manifest.Version)
	}
	if manifest.Kind != "format" {
		t.Errorf("Expected Kind='format', got '%s'", manifest.Kind)
	}
	if manifest.Entrypoint != "format-flex" {
		t.Errorf("Expected Entrypoint='format-flex', got '%s'", manifest.Entrypoint)
	}
	if len(manifest.Capabilities.Inputs) != 1 || manifest.Capabilities.Inputs[0] != "file" {
		t.Errorf("Expected Inputs=['file'], got %v", manifest.Capabilities.Inputs)
	}
	if len(manifest.Capabilities.Outputs) != 1 || manifest.Capabilities.Outputs[0] != "artifact.kind:flex" {
		t.Errorf("Expected Outputs=['artifact.kind:flex'], got %v", manifest.Capabilities.Outputs)
	}
}

func TestRegister(t *testing.T) {
	// This test verifies that Register can be called without panicking
	// The actual registration is tested through init() being called
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Register panicked: %v", r)
		}
	}()
	Register()
}

func TestHandler_ImplementsInterface(t *testing.T) {
	// Verify that Handler implements EmbeddedFormatHandler
	var _ plugins.EmbeddedFormatHandler = (*Handler)(nil)
}
