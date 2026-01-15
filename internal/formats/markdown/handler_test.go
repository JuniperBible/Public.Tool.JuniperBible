package markdown

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func TestDetect_ValidMarkdownFile(t *testing.T) {
	// Create a temporary test file with .md extension
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test Markdown"), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed, got: %s", result.Reason)
	}
	if result.Format != "markdown" {
		t.Errorf("Expected format 'markdown', got %s", result.Format)
	}
	if result.Reason != "Markdown file detected" {
		t.Errorf("Expected reason 'Markdown file detected', got %s", result.Reason)
	}
}

func TestDetect_InvalidExtension(t *testing.T) {
	// Create a temporary test file with wrong extension
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("# Test Content"), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for non-.md extension")
	}
	if result.Reason != "not a .md file" {
		t.Errorf("Expected reason 'not a .md file', got %s", result.Reason)
	}
}

func TestDetect_Directory(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	h := &Handler{}
	result, err := h.Detect(tmpDir)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for directory")
	}
	if result.Reason != "path is a directory" {
		t.Errorf("Expected reason 'path is a directory', got %s", result.Reason)
	}
}

func TestDetect_StatError(t *testing.T) {
	// Use a non-existent file path
	nonExistentPath := "/nonexistent/path/to/file.md"

	h := &Handler{}
	result, err := h.Detect(nonExistentPath)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for non-existent file")
	}
	if !strings.Contains(result.Reason, "cannot stat:") {
		t.Errorf("Expected reason to contain 'cannot stat:', got %s", result.Reason)
	}
}

func TestIngest_Success(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sample.md")
	content := []byte("# Sample Markdown\n\nThis is a test.")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	// Verify artifact ID is correct (filename without extension)
	if result.ArtifactID != "sample" {
		t.Errorf("Expected artifact ID 'sample', got %s", result.ArtifactID)
	}

	// Verify size matches
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}

	// Verify metadata
	if result.Metadata["format"] != "markdown" {
		t.Errorf("Expected format 'markdown', got %s", result.Metadata["format"])
	}

	// Verify hash is correct
	expectedHash := sha256.Sum256(content)
	expectedHashHex := hex.EncodeToString(expectedHash[:])
	if result.BlobSHA256 != expectedHashHex {
		t.Errorf("Expected hash %s, got %s", expectedHashHex, result.BlobSHA256)
	}
}

func TestIngest_BlobCreation(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.md")
	content := []byte("# Test Content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	// Verify blob directory was created
	blobDir := filepath.Join(outputDir, result.BlobSHA256[:2])
	if info, err := os.Stat(blobDir); err != nil || !info.IsDir() {
		t.Errorf("Expected blob directory to exist at %s", blobDir)
	}

	// Verify blob file was written
	blobPath := filepath.Join(blobDir, result.BlobSHA256)
	blobContent, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("Failed to read blob file: %v", err)
	}

	if string(blobContent) != string(content) {
		t.Errorf("Expected blob content %s, got %s", content, blobContent)
	}
}

func TestIngest_ReadError(t *testing.T) {
	// Use a non-existent file to trigger read error
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.md")
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	_, err := h.Ingest(nonExistentFile, outputDir)
	if err == nil {
		t.Error("Expected Ingest to return error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected error to contain 'failed to read file', got %v", err)
	}
}

func TestIngest_MkdirAllError(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.md")
	content := []byte("# Test Content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file where the blob directory should be
	// This will cause MkdirAll to fail
	hash := sha256.Sum256(content)
	hashHex := hex.EncodeToString(hash[:])
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file with the name of what should be a directory
	blobDirPath := filepath.Join(outputDir, hashHex[:2])
	if err := os.WriteFile(blobDirPath, []byte("blocking file"), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	_, err := h.Ingest(testFile, outputDir)
	if err == nil {
		t.Error("Expected Ingest to return error when blob dir creation fails")
	}
	if !strings.Contains(err.Error(), "failed to create blob dir") {
		t.Errorf("Expected error to contain 'failed to create blob dir', got %v", err)
	}
}

func TestIngest_WriteError(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.md")
	content := []byte("# Test Content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Create output directory structure where the blob file would be a directory
	// This will cause WriteFile to fail
	hash := sha256.Sum256(content)
	hashHex := hex.EncodeToString(hash[:])
	outputDir := filepath.Join(tmpDir, "output")
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a directory with the name of what should be the blob file
	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.Mkdir(blobPath, 0755); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	_, err := h.Ingest(testFile, outputDir)
	if err == nil {
		t.Error("Expected Ingest to return error when blob path is a directory")
	}
	if !strings.Contains(err.Error(), "failed to write blob") {
		t.Errorf("Expected error to contain 'failed to write blob', got %v", err)
	}
}

func TestEnumerate_ValidFile(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "document.md")
	content := []byte("# Document")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Enumerate(testFile)
	if err != nil {
		t.Fatalf("Enumerate returned error: %v", err)
	}

	// Verify we got exactly one entry
	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]

	// Verify entry path is the basename
	if entry.Path != "document.md" {
		t.Errorf("Expected path 'document.md', got %s", entry.Path)
	}

	// Verify size matches
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}

	// Verify it's not marked as a directory
	if entry.IsDir {
		t.Error("Expected IsDir to be false")
	}
}

func TestEnumerate_StatError(t *testing.T) {
	// Use a non-existent file path
	nonExistentPath := "/nonexistent/path/to/file.md"

	h := &Handler{}
	_, err := h.Enumerate(nonExistentPath)
	if err == nil {
		t.Error("Expected Enumerate to return error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to stat") {
		t.Errorf("Expected error to contain 'failed to stat', got %v", err)
	}
}

func TestExtractIR_ReturnsError(t *testing.T) {
	h := &Handler{}
	result, err := h.ExtractIR("/some/path.md", "/output/dir")
	if err == nil {
		t.Error("Expected ExtractIR to return error")
	}
	if result != nil {
		t.Error("Expected ExtractIR to return nil result")
	}
	expectedMsg := "markdown format does not support IR extraction"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestEmitNative_ReturnsError(t *testing.T) {
	h := &Handler{}
	result, err := h.EmitNative("/some/ir.json", "/output/dir")
	if err == nil {
		t.Error("Expected EmitNative to return error")
	}
	if result != nil {
		t.Error("Expected EmitNative to return nil result")
	}
	expectedMsg := "markdown format does not support native emission"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestManifest(t *testing.T) {
	manifest := Manifest()

	// Verify manifest fields
	if manifest.PluginID != "format.markdown" {
		t.Errorf("Expected PluginID 'format.markdown', got %s", manifest.PluginID)
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", manifest.Version)
	}
	if manifest.Kind != "format" {
		t.Errorf("Expected Kind 'format', got %s", manifest.Kind)
	}
	if manifest.Entrypoint != "format-markdown" {
		t.Errorf("Expected Entrypoint 'format-markdown', got %s", manifest.Entrypoint)
	}

	// Verify capabilities
	if len(manifest.Capabilities.Inputs) != 1 || manifest.Capabilities.Inputs[0] != "file" {
		t.Errorf("Expected Inputs ['file'], got %v", manifest.Capabilities.Inputs)
	}
	if len(manifest.Capabilities.Outputs) != 1 || manifest.Capabilities.Outputs[0] != "artifact.kind:markdown" {
		t.Errorf("Expected Outputs ['artifact.kind:markdown'], got %v", manifest.Capabilities.Outputs)
	}
}

func TestRegister(t *testing.T) {
	// Save the original embedded plugins registry
	originalPlugins := plugins.ListEmbeddedPlugins()
	originalCount := len(originalPlugins)

	// Register should be called by init(), but we can test it directly
	// The markdown plugin should already be registered, so count should include it
	currentPlugins := plugins.ListEmbeddedPlugins()

	// Verify markdown plugin is registered
	found := false
	for _, p := range currentPlugins {
		if p.Manifest.PluginID == "format.markdown" {
			found = true
			// Verify it has a Format handler
			if p.Format == nil {
				t.Error("Expected Format handler to be non-nil")
			}
			// Verify the handler is of the correct type
			if _, ok := p.Format.(*Handler); !ok {
				t.Error("Expected Format handler to be *Handler")
			}
			break
		}
	}

	if !found {
		t.Error("Expected markdown plugin to be registered")
	}

	// Verify we have at least one plugin registered
	if len(currentPlugins) < 1 {
		t.Error("Expected at least 1 embedded plugin to be registered")
	}

	// The count should be at least what it was originally
	if len(currentPlugins) < originalCount {
		t.Errorf("Expected at least %d plugins, got %d", originalCount, len(currentPlugins))
	}
}
