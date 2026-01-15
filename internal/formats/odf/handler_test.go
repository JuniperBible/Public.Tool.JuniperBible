package odf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func TestDetect_ValidODTFile(t *testing.T) {
	// Create a temporary .odt file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.odt")
	if err := os.WriteFile(testFile, []byte("fake odt content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}

	if !result.Detected {
		t.Errorf("expected Detected=true, got false. Reason: %s", result.Reason)
	}
	if result.Format != "odf" {
		t.Errorf("expected Format=odf, got %s", result.Format)
	}
	if result.Reason != "ODF file detected" {
		t.Errorf("expected Reason='ODF file detected', got %s", result.Reason)
	}
}

func TestDetect_InvalidZip(t *testing.T) {
	// This test checks that we can still detect .odt files even if they're not valid zip files
	// The Detect method only checks the extension, not the actual content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "invalid.odt")
	if err := os.WriteFile(testFile, []byte("not a zip"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}

	// Since Detect only checks extension, this should still be detected
	if !result.Detected {
		t.Errorf("expected Detected=true for .odt file, got false. Reason: %s", result.Reason)
	}
}

func TestDetect_WrongExtension(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("not an odt"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}

	if result.Detected {
		t.Errorf("expected Detected=false, got true")
	}
	if result.Reason != "not a .odt file" {
		t.Errorf("expected Reason='not a .odt file', got %s", result.Reason)
	}
}

func TestDetect_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	h := &Handler{}
	result, err := h.Detect(tmpDir)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}

	if result.Detected {
		t.Errorf("expected Detected=false for directory, got true")
	}
	if result.Reason != "path is a directory" {
		t.Errorf("expected Reason='path is a directory', got %s", result.Reason)
	}
}

func TestDetect_NonExistentFile(t *testing.T) {
	h := &Handler{}
	result, err := h.Detect("/nonexistent/path/file.odt")
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}

	if result.Detected {
		t.Errorf("expected Detected=false for nonexistent file, got true")
	}
	if result.Reason == "" {
		t.Errorf("expected Reason to contain error message, got empty string")
	}
}

func TestIngest_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sample.odt")
	testContent := []byte("sample odt content")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	if result.ArtifactID != "sample" {
		t.Errorf("expected ArtifactID='sample', got %s", result.ArtifactID)
	}
	if result.SizeBytes != int64(len(testContent)) {
		t.Errorf("expected SizeBytes=%d, got %d", len(testContent), result.SizeBytes)
	}
	if result.Metadata["format"] != "odf" {
		t.Errorf("expected Metadata[format]='odf', got %s", result.Metadata["format"])
	}
	if result.BlobSHA256 == "" {
		t.Errorf("expected BlobSHA256 to be non-empty")
	}

	// Verify blob was written
	blobDir := filepath.Join(outputDir, result.BlobSHA256[:2])
	blobPath := filepath.Join(blobDir, result.BlobSHA256)
	if _, err := os.Stat(blobPath); err != nil {
		t.Errorf("blob file not created at %s: %v", blobPath, err)
	}

	// Verify blob content
	writtenData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("failed to read blob: %v", err)
	}
	if string(writtenData) != string(testContent) {
		t.Errorf("blob content mismatch: expected %q, got %q", testContent, writtenData)
	}
}

func TestIngest_PermissionError(t *testing.T) {
	// Create a file that we cannot read
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "unreadable.odt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Make it unreadable
	if err := os.Chmod(testFile, 0000); err != nil {
		t.Fatalf("failed to chmod file: %v", err)
	}
	defer os.Chmod(testFile, 0644) // cleanup

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	h := &Handler{}
	_, err := h.Ingest(testFile, outputDir)
	if err == nil {
		t.Errorf("expected error for unreadable file, got nil")
	}
}

func TestIngest_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	_, err := h.Ingest("/nonexistent/file.odt", outputDir)
	if err == nil {
		t.Errorf("expected error for nonexistent file, got nil")
	}
}

func TestIngest_InvalidOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.odt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Use an invalid output dir (e.g., a file instead of a directory)
	invalidDir := filepath.Join(tmpDir, "notadir")
	if err := os.WriteFile(invalidDir, []byte("file"), 0644); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	h := &Handler{}
	_, err := h.Ingest(testFile, invalidDir)
	if err == nil {
		t.Errorf("expected error for invalid output dir, got nil")
	}
}

func TestIngest_BlobWriteError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.odt")
	testContent := []byte("content for write error test")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create output dir
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	h := &Handler{}

	// First, do a dry run to calculate the hash and create the blob directory
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("first Ingest failed: %v", err)
	}

	// Get the blob directory path
	blobDir := filepath.Join(outputDir, result.BlobSHA256[:2])
	blobPath := filepath.Join(blobDir, result.BlobSHA256)

	// Remove the blob file first (while we can still write)
	if err := os.Remove(blobPath); err != nil {
		t.Fatalf("failed to remove blob: %v", err)
	}

	// Now make the blob directory read-only to prevent writing
	if err := os.Chmod(blobDir, 0555); err != nil {
		t.Fatalf("failed to chmod blob dir: %v", err)
	}
	defer os.Chmod(blobDir, 0755) // cleanup

	// Now try to ingest again, which should fail when trying to write the blob
	_, err = h.Ingest(testFile, outputDir)
	if err == nil {
		t.Errorf("expected error when blob directory is read-only, got nil")
	}
}

func TestEnumerate_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "document.odt")
	testContent := []byte("test content for enumeration")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	h := &Handler{}
	result, err := h.Enumerate(testFile)
	if err != nil {
		t.Fatalf("Enumerate returned error: %v", err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "document.odt" {
		t.Errorf("expected Path='document.odt', got %s", entry.Path)
	}
	if entry.SizeBytes != int64(len(testContent)) {
		t.Errorf("expected SizeBytes=%d, got %d", len(testContent), entry.SizeBytes)
	}
	if entry.IsDir {
		t.Errorf("expected IsDir=false, got true")
	}
}

func TestEnumerate_StatError(t *testing.T) {
	h := &Handler{}
	_, err := h.Enumerate("/nonexistent/path/file.odt")
	if err == nil {
		t.Errorf("expected error for nonexistent file, got nil")
	}
}

func TestExtractIR_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.odt")
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	result, err := h.ExtractIR(testFile, outputDir)
	if err == nil {
		t.Errorf("expected error from ExtractIR, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result from ExtractIR, got %v", result)
	}
	expectedMsg := "odf format does not support IR extraction"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestEmitNative_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "ir.json")
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	result, err := h.EmitNative(irPath, outputDir)
	if err == nil {
		t.Errorf("expected error from EmitNative, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result from EmitNative, got %v", result)
	}
	expectedMsg := "odf format does not support native emission"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest.PluginID != "format.odf" {
		t.Errorf("expected PluginID='format.odf', got %s", manifest.PluginID)
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("expected Version='1.0.0', got %s", manifest.Version)
	}
	if manifest.Kind != "format" {
		t.Errorf("expected Kind='format', got %s", manifest.Kind)
	}
	if manifest.Entrypoint != "format-odf" {
		t.Errorf("expected Entrypoint='format-odf', got %s", manifest.Entrypoint)
	}

	// Check capabilities
	if len(manifest.Capabilities.Inputs) != 1 || manifest.Capabilities.Inputs[0] != "file" {
		t.Errorf("expected Inputs=['file'], got %v", manifest.Capabilities.Inputs)
	}
	if len(manifest.Capabilities.Outputs) != 1 || manifest.Capabilities.Outputs[0] != "artifact.kind:odf" {
		t.Errorf("expected Outputs=['artifact.kind:odf'], got %v", manifest.Capabilities.Outputs)
	}
}

func TestRegister(t *testing.T) {
	// Get the current count of registered plugins
	initialCount := len(plugins.ListEmbeddedPlugins())

	// Register should be idempotent, but we can't easily test the actual registration
	// without affecting the global state. Instead, verify it doesn't panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Register() panicked: %v", r)
		}
	}()

	Register()

	// Verify the plugin is in the list
	found := false
	for _, p := range plugins.ListEmbeddedPlugins() {
		if p.Manifest.PluginID == "format.odf" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("plugin 'format.odf' not found in embedded plugins list")
	}

	// The count should be at least the initial count (it may be the same due to init())
	finalCount := len(plugins.ListEmbeddedPlugins())
	if finalCount < initialCount {
		t.Errorf("plugin count decreased after Register: initial=%d, final=%d", initialCount, finalCount)
	}
}

func TestInit(t *testing.T) {
	// The init() function calls Register(), which is tested above.
	// We just verify that the plugin was registered during package initialization.
	found := false
	for _, p := range plugins.ListEmbeddedPlugins() {
		if p.Manifest.PluginID == "format.odf" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("plugin 'format.odf' not found in embedded plugins list after init()")
	}
}

func TestDetect_CaseInsensitiveExtension(t *testing.T) {
	// Test uppercase extension
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ODT")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}

	if !result.Detected {
		t.Errorf("expected Detected=true for .ODT file, got false. Reason: %s", result.Reason)
	}
}

func TestIngest_ArtifactIDWithMultipleDots(t *testing.T) {
	// Test that artifact ID correctly removes only the extension
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "my.document.name.odt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	expectedID := "my.document.name"
	if result.ArtifactID != expectedID {
		t.Errorf("expected ArtifactID=%q, got %q", expectedID, result.ArtifactID)
	}
}
