package morphgnt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func TestDetect_ValidMorphGNTFile(t *testing.T) {
	// Create a temporary test file with .morphgnt extension
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.morphgnt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
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
	if result.Format != "morphgnt" {
		t.Errorf("Expected format 'morphgnt', got %s", result.Format)
	}
	if result.Reason != "MorphGNT file detected" {
		t.Errorf("Expected reason 'MorphGNT file detected', got %s", result.Reason)
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
	if result.Reason != "not a .morphgnt file" {
		t.Errorf("Expected reason 'not a .morphgnt file', got %s", result.Reason)
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
	nonExistent := filepath.Join(tmpDir, "nonexistent.morphgnt")

	handler := &Handler{}
	result, err := handler.Detect(nonExistent)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for non-existent file")
	}
	if !strings.Contains(result.Reason, "cannot stat") {
		t.Errorf("Expected reason to mention 'cannot stat', got %s", result.Reason)
	}
}

func TestDetect_CaseInsensitiveExtension(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.MORPHGNT")
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
	testFile := filepath.Join(tmpDir, "matthew.morphgnt")
	content := []byte("test morphgnt content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify artifact ID
	if result.ArtifactID != "matthew" {
		t.Errorf("Expected artifact ID 'matthew', got %s", result.ArtifactID)
	}

	// Verify size
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}

	// Verify metadata
	if result.Metadata["format"] != "morphgnt" {
		t.Errorf("Expected format 'morphgnt', got %s", result.Metadata["format"])
	}

	// Verify blob was written
	if result.BlobSHA256 == "" {
		t.Error("Expected BlobSHA256 to be set")
	}
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

func TestIngest_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nonexistent.morphgnt")
	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	_, err := handler.Ingest(nonExistent, outputDir)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected error about reading file, got %v", err)
	}
}

func TestIngest_InvalidOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.morphgnt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file where the output directory should be
	invalidOutput := filepath.Join(tmpDir, "invalid")
	if err := os.WriteFile(invalidOutput, []byte("blocking"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	_, err := handler.Ingest(testFile, filepath.Join(invalidOutput, "aa"))
	if err == nil {
		t.Error("Expected error for invalid output directory")
	}
	if !strings.Contains(err.Error(), "failed to create blob dir") {
		t.Errorf("Expected error about creating blob dir, got %v", err)
	}
}

func TestIngest_FailedWriteBlob(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.morphgnt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}

	// First get the hash by doing a successful ingest
	tempOutput := filepath.Join(tmpDir, "temp")
	tempResult, err := handler.Ingest(testFile, tempOutput)
	if err != nil {
		t.Fatal(err)
	}

	// Now create output directory with the blob subdirectory
	outputDir := filepath.Join(tmpDir, "output")
	blobSubdir := filepath.Join(outputDir, tempResult.BlobSHA256[:2])
	if err := os.MkdirAll(blobSubdir, 0755); err != nil {
		t.Fatal(err)
	}

	// Make the blob subdirectory read-only to trigger write error
	if err := os.Chmod(blobSubdir, 0555); err != nil {
		t.Fatal(err)
	}
	// Ensure we restore permissions for cleanup
	defer os.Chmod(blobSubdir, 0755)

	// Now try to ingest again - should fail to write blob
	_, err = handler.Ingest(testFile, outputDir)
	if err == nil {
		t.Error("Expected error when writing blob fails")
	}
	if !strings.Contains(err.Error(), "failed to write blob") {
		t.Errorf("Expected error about writing blob, got %v", err)
	}
}

func TestEnumerate_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.morphgnt")
	content := []byte("test content for enumeration")
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
	if entry.Path != "test.morphgnt" {
		t.Errorf("Expected path 'test.morphgnt', got %s", entry.Path)
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
	nonExistent := filepath.Join(tmpDir, "nonexistent.morphgnt")

	handler := &Handler{}
	_, err := handler.Enumerate(nonExistent)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to stat") {
		t.Errorf("Expected error about stat, got %v", err)
	}
}

func TestExtractIR_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.morphgnt")
	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.ExtractIR(testFile, outputDir)
	if err == nil {
		t.Error("Expected error for unsupported operation")
	}
	if result != nil {
		t.Error("Expected nil result for unsupported operation")
	}
	if !strings.Contains(err.Error(), "does not support IR extraction") {
		t.Errorf("Expected error about IR extraction not supported, got %v", err)
	}
}

func TestEmitNative_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "test.ir")
	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.EmitNative(irPath, outputDir)
	if err == nil {
		t.Error("Expected error for unsupported operation")
	}
	if result != nil {
		t.Error("Expected nil result for unsupported operation")
	}
	if !strings.Contains(err.Error(), "does not support native emission") {
		t.Errorf("Expected error about native emission not supported, got %v", err)
	}
}

func TestManifest(t *testing.T) {
	manifest := Manifest()
	if manifest == nil {
		t.Fatal("Expected non-nil manifest")
	}

	if manifest.PluginID != "format.morphgnt" {
		t.Errorf("Expected plugin ID 'format.morphgnt', got %s", manifest.PluginID)
	}

	if manifest.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", manifest.Version)
	}

	if manifest.Kind != "format" {
		t.Errorf("Expected kind 'format', got %s", manifest.Kind)
	}

	if manifest.Entrypoint != "format-morphgnt" {
		t.Errorf("Expected entrypoint 'format-morphgnt', got %s", manifest.Entrypoint)
	}

	// Verify capabilities
	if len(manifest.Capabilities.Inputs) != 1 || manifest.Capabilities.Inputs[0] != "file" {
		t.Errorf("Expected inputs ['file'], got %v", manifest.Capabilities.Inputs)
	}

	if len(manifest.Capabilities.Outputs) != 1 || manifest.Capabilities.Outputs[0] != "artifact.kind:morphgnt" {
		t.Errorf("Expected outputs ['artifact.kind:morphgnt'], got %v", manifest.Capabilities.Outputs)
	}
}

func TestRegister(t *testing.T) {
	// Clear registry before test
	plugins.ClearEmbeddedRegistry()

	// Register the plugin
	Register()

	// Verify plugin is registered
	if !plugins.HasEmbeddedPlugin("format.morphgnt") {
		t.Error("Expected plugin to be registered")
	}

	// Get the plugin and verify it
	plugin := plugins.GetEmbeddedPlugin("format.morphgnt")
	if plugin == nil {
		t.Fatal("Expected non-nil plugin")
	}

	if plugin.Manifest == nil {
		t.Error("Expected non-nil manifest")
	}

	if plugin.Format == nil {
		t.Error("Expected non-nil format handler")
	}

	if plugin.Tool != nil {
		t.Error("Expected nil tool handler for format plugin")
	}

	// Verify manifest matches
	if plugin.Manifest.PluginID != "format.morphgnt" {
		t.Errorf("Expected plugin ID 'format.morphgnt', got %s", plugin.Manifest.PluginID)
	}
}

func TestIngest_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.morphgnt")
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "empty" {
		t.Errorf("Expected artifact ID 'empty', got %s", result.ArtifactID)
	}

	if result.SizeBytes != 0 {
		t.Errorf("Expected size 0, got %d", result.SizeBytes)
	}

	// Verify blob exists
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist even for empty file")
	}
}

func TestIngest_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.morphgnt")
	// Create a 1MB file
	content := make([]byte, 1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}

	// Verify blob content
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	blobData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(blobData) != len(content) {
		t.Errorf("Expected blob size %d, got %d", len(content), len(blobData))
	}
}
