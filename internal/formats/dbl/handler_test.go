package dbl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func TestDetect_ValidDBLFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.dbl")
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
	if result.Format != "dbl" {
		t.Errorf("Expected format 'dbl', got %s", result.Format)
	}
	if result.Reason != "DBL file detected" {
		t.Errorf("Expected reason 'DBL file detected', got %s", result.Reason)
	}
}

func TestDetect_InvalidExtension(t *testing.T) {
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

	if result.Detected {
		t.Error("Expected detection to fail for non-.dbl file")
	}
	if result.Reason != "not a .dbl file" {
		t.Errorf("Expected reason 'not a .dbl file', got %s", result.Reason)
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
		t.Error("Expected detection to fail for directory")
	}
	if result.Reason != "path is a directory" {
		t.Errorf("Expected reason 'path is a directory', got %s", result.Reason)
	}
}

func TestDetect_FileNotFound(t *testing.T) {
	h := &Handler{}
	result, err := h.Detect("/nonexistent/path/file.dbl")
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for nonexistent file")
	}
	if !strings.Contains(result.Reason, "cannot stat") {
		t.Errorf("Expected reason to contain 'cannot stat', got %s", result.Reason)
	}
}

func TestDetect_CaseInsensitiveExtension(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.DBL")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed for uppercase extension, got: %s", result.Reason)
	}
	if result.Format != "dbl" {
		t.Errorf("Expected format 'dbl', got %s", result.Format)
	}
}

func TestIngest_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-artifact.dbl")
	content := []byte("test content for ingestion")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify artifact ID is correctly extracted (basename without extension)
	if result.ArtifactID != "test-artifact" {
		t.Errorf("Expected artifact ID 'test-artifact', got %s", result.ArtifactID)
	}

	// Verify size
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}

	// Verify metadata
	if result.Metadata["format"] != "dbl" {
		t.Errorf("Expected format 'dbl', got %s", result.Metadata["format"])
	}

	// Verify blob hash is not empty
	if result.BlobSHA256 == "" {
		t.Error("Expected BlobSHA256 to be set")
	}

	// Verify blob was written to correct location
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}

	// Verify blob content matches original
	blobData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(blobData) != string(content) {
		t.Error("Blob content does not match original file content")
	}
}

func TestIngest_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	_, err := h.Ingest("/nonexistent/path/file.dbl", outputDir)
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected error to contain 'failed to read file', got %s", err.Error())
	}
}

func TestIngest_OutputDirCreation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.dbl")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Use a nested output directory that doesn't exist yet
	outputDir := filepath.Join(tmpDir, "output", "nested", "dir")

	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the nested directory structure was created
	blobDir := filepath.Join(outputDir, result.BlobSHA256[:2])
	if _, err := os.Stat(blobDir); os.IsNotExist(err) {
		t.Error("Expected blob directory to be created")
	}
}

func TestIngest_BlobDirCreationError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.dbl")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file where the blob directory should be, causing MkdirAll to fail
	// First, we need to know what the hash prefix will be
	h := &Handler{}

	// Create output directory as a read-only file to cause MkdirAll to fail
	outputDir := filepath.Join(tmpDir, "readonly-output")
	if err := os.WriteFile(outputDir, []byte("block"), 0444); err != nil {
		t.Fatal(err)
	}

	_, err := h.Ingest(testFile, outputDir)
	if err == nil {
		t.Fatal("Expected error when blob dir creation fails")
	}
	if !strings.Contains(err.Error(), "failed to create blob dir") {
		t.Errorf("Expected error to contain 'failed to create blob dir', got %s", err.Error())
	}
}

func TestIngest_BlobWriteError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.dbl")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Create output directory structure but make the blob file location read-only
	outputDir := filepath.Join(tmpDir, "output")
	h := &Handler{}

	// First ingest to know the hash
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	// Remove the blob file first, then make the directory read-only
	blobDir := filepath.Join(outputDir, result.BlobSHA256[:2])
	blobPath := filepath.Join(blobDir, result.BlobSHA256)
	if err := os.Remove(blobPath); err != nil {
		t.Fatal(err)
	}

	// Now make the blob directory read-only
	if err := os.Chmod(blobDir, 0555); err != nil {
		t.Fatal(err)
	}

	// Ensure we restore permissions for cleanup
	defer func() {
		os.Chmod(blobDir, 0755)
	}()

	// Try to ingest again, should fail to write blob
	_, err = h.Ingest(testFile, outputDir)
	if err == nil {
		t.Fatal("Expected error when blob write fails")
	}
	if !strings.Contains(err.Error(), "failed to write blob") {
		t.Errorf("Expected error to contain 'failed to write blob', got %s", err.Error())
	}
}

func TestEnumerate_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.dbl")
	content := []byte("test content for enumeration")
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
	if entry.Path != "test.dbl" {
		t.Errorf("Expected path 'test.dbl', got %s", entry.Path)
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
	if entry.IsDir {
		t.Error("Expected IsDir to be false")
	}
}

func TestEnumerate_FileNotFound(t *testing.T) {
	h := &Handler{}
	_, err := h.Enumerate("/nonexistent/path/file.dbl")
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to stat") {
		t.Errorf("Expected error to contain 'failed to stat', got %s", err.Error())
	}
}

func TestExtractIR(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.dbl")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	result, err := h.ExtractIR(testFile, outputDir)

	// DBL format does not support IR extraction, should return error
	if err == nil {
		t.Fatal("Expected error for unsupported IR extraction")
	}
	if !strings.Contains(err.Error(), "does not support IR extraction") {
		t.Errorf("Expected error to mention unsupported IR extraction, got %s", err.Error())
	}
	if result != nil {
		t.Error("Expected nil result for unsupported operation")
	}
}

func TestEmitNative(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "ir.json")
	if err := os.WriteFile(irPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	result, err := h.EmitNative(irPath, outputDir)

	// DBL format does not support native emission, should return error
	if err == nil {
		t.Fatal("Expected error for unsupported native emission")
	}
	if !strings.Contains(err.Error(), "does not support native emission") {
		t.Errorf("Expected error to mention unsupported native emission, got %s", err.Error())
	}
	if result != nil {
		t.Error("Expected nil result for unsupported operation")
	}
}

func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest.PluginID != "format.dbl" {
		t.Errorf("Expected PluginID 'format.dbl', got %s", manifest.PluginID)
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", manifest.Version)
	}
	if manifest.Kind != "format" {
		t.Errorf("Expected Kind 'format', got %s", manifest.Kind)
	}
	if manifest.Entrypoint != "format-dbl" {
		t.Errorf("Expected Entrypoint 'format-dbl', got %s", manifest.Entrypoint)
	}

	// Verify capabilities
	if len(manifest.Capabilities.Inputs) != 1 || manifest.Capabilities.Inputs[0] != "file" {
		t.Errorf("Expected Inputs ['file'], got %v", manifest.Capabilities.Inputs)
	}
	if len(manifest.Capabilities.Outputs) != 1 || manifest.Capabilities.Outputs[0] != "artifact.kind:dbl" {
		t.Errorf("Expected Outputs ['artifact.kind:dbl'], got %v", manifest.Capabilities.Outputs)
	}
}

func TestRegister(t *testing.T) {
	// Get the current count of registered plugins
	allPlugins := plugins.ListEmbeddedPlugins()
	initialCount := len(allPlugins)

	// Register should be called in init(), but let's call it again to ensure it's idempotent
	Register()

	// Verify the plugin is registered
	allPlugins = plugins.ListEmbeddedPlugins()

	// The count might be the same if already registered, but should not decrease
	if len(allPlugins) < initialCount {
		t.Error("Expected plugin count to not decrease after registration")
	}

	// Verify the DBL plugin is in the list
	found := false
	for _, p := range allPlugins {
		if p.Manifest.PluginID == "format.dbl" {
			found = true
			if p.Manifest.Kind != "format" {
				t.Errorf("Expected plugin kind 'format', got %s", p.Manifest.Kind)
			}
			break
		}
	}

	if !found {
		t.Error("Expected to find format.dbl plugin in registered plugins")
	}
}
