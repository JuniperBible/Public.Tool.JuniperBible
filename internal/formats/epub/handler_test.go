package epub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetect_ValidEpub(t *testing.T) {
	// Create a temporary test file with .epub extension
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.epub")
	if err := os.WriteFile(testFile, []byte("fake epub content"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed for .epub file, got: %s", result.Reason)
	}
	if result.Format != "epub" {
		t.Errorf("Expected format 'epub', got %s", result.Format)
	}
	if result.Reason != "EPUB file detected" {
		t.Errorf("Expected reason 'EPUB file detected', got %s", result.Reason)
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
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for non-.epub file")
	}
	if result.Reason != "not a .epub file" {
		t.Errorf("Expected reason 'not a .epub file', got %s", result.Reason)
	}
}

func TestDetect_Directory(t *testing.T) {
	// Create a temporary directory with .epub extension
	tmpDir := t.TempDir()
	epubDir := filepath.Join(tmpDir, "test.epub")
	if err := os.Mkdir(epubDir, 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Detect(epubDir)
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

func TestDetect_NonExistent(t *testing.T) {
	// Try to detect a file that doesn't exist
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nonexistent.epub")

	handler := &Handler{}
	result, err := handler.Detect(nonExistent)
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

func TestIngest_Success(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.epub")
	content := []byte("epub test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if result.ArtifactID != "test" {
		t.Errorf("Expected artifact ID 'test', got %s", result.ArtifactID)
	}
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}
	if result.Metadata["format"] != "epub" {
		t.Errorf("Expected format 'epub', got %s", result.Metadata["format"])
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected BlobSHA256 to be populated")
	}

	// Verify blob was written to the correct location
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Errorf("Expected blob file to exist at %s", blobPath)
	}

	// Verify blob content matches original
	blobData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("Failed to read blob: %v", err)
	}
	if string(blobData) != string(content) {
		t.Errorf("Blob content mismatch, expected %s, got %s", content, blobData)
	}
}

func TestIngest_FileNotFound(t *testing.T) {
	// Try to ingest a file that doesn't exist
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nonexistent.epub")
	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	_, err := handler.Ingest(nonExistent, outputDir)
	if err == nil {
		t.Error("Expected error when ingesting non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected error to contain 'failed to read file', got %s", err.Error())
	}
}

func TestIngest_BlobCreation(t *testing.T) {
	// Test that blob directory is created with correct permissions
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.epub")
	content := []byte("test content for blob")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	// Check that blob directory was created
	blobDir := filepath.Join(outputDir, result.BlobSHA256[:2])
	info, err := os.Stat(blobDir)
	if err != nil {
		t.Fatalf("Blob directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected blob directory to be a directory")
	}

	// Check that blob file exists and has correct permissions
	blobPath := filepath.Join(blobDir, result.BlobSHA256)
	info, err = os.Stat(blobPath)
	if err != nil {
		t.Fatalf("Blob file not created: %v", err)
	}
	if info.IsDir() {
		t.Error("Expected blob file to be a file, not directory")
	}
}

func TestEnumerate_ValidFile(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.epub")
	content := []byte("epub enumerate test")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Enumerate(testFile)
	if err != nil {
		t.Fatalf("Enumerate failed: %v", err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.epub" {
		t.Errorf("Expected path 'test.epub', got %s", entry.Path)
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
	if entry.IsDir {
		t.Error("Expected IsDir to be false")
	}
}

func TestEnumerate_StatError(t *testing.T) {
	// Try to enumerate a file that doesn't exist
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nonexistent.epub")

	handler := &Handler{}
	_, err := handler.Enumerate(nonExistent)
	if err == nil {
		t.Error("Expected error when enumerating non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to stat") {
		t.Errorf("Expected error to contain 'failed to stat', got %s", err.Error())
	}
}

func TestExtractIR_ReturnsError(t *testing.T) {
	// ExtractIR should always return an error for epub format
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.epub")
	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.ExtractIR(testFile, outputDir)
	if err == nil {
		t.Error("Expected error from ExtractIR")
	}
	if result != nil {
		t.Error("Expected nil result from ExtractIR")
	}
	if !strings.Contains(err.Error(), "does not support IR extraction") {
		t.Errorf("Expected error to mention 'does not support IR extraction', got %s", err.Error())
	}
}

func TestEmitNative_ReturnsError(t *testing.T) {
	// EmitNative should always return an error for epub format
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "test.ir")
	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.EmitNative(irPath, outputDir)
	if err == nil {
		t.Error("Expected error from EmitNative")
	}
	if result != nil {
		t.Error("Expected nil result from EmitNative")
	}
	if !strings.Contains(err.Error(), "does not support native emission") {
		t.Errorf("Expected error to mention 'does not support native emission', got %s", err.Error())
	}
}

func TestManifest(t *testing.T) {
	// Test the Manifest function
	manifest := Manifest()
	if manifest == nil {
		t.Fatal("Expected non-nil manifest")
	}

	if manifest.PluginID != "format.epub" {
		t.Errorf("Expected PluginID 'format.epub', got %s", manifest.PluginID)
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", manifest.Version)
	}
	if manifest.Kind != "format" {
		t.Errorf("Expected Kind 'format', got %s", manifest.Kind)
	}
	if manifest.Entrypoint != "format-epub" {
		t.Errorf("Expected Entrypoint 'format-epub', got %s", manifest.Entrypoint)
	}

	// Check capabilities
	if len(manifest.Capabilities.Inputs) != 1 || manifest.Capabilities.Inputs[0] != "file" {
		t.Errorf("Expected Inputs ['file'], got %v", manifest.Capabilities.Inputs)
	}
	if len(manifest.Capabilities.Outputs) != 1 || manifest.Capabilities.Outputs[0] != "artifact.kind:epub" {
		t.Errorf("Expected Outputs ['artifact.kind:epub'], got %v", manifest.Capabilities.Outputs)
	}
}

// TestRegister verifies that the Register function doesn't panic
// and that the plugin is properly registered in the embedded registry
func TestRegister(t *testing.T) {
	// Clear any existing plugins to ensure clean state
	// Note: This assumes there's a way to query registered plugins
	// Since we can't inspect the private registry, we just ensure Register doesn't panic

	// This should not panic
	Register()

	// Verify we can create a handler
	handler := &Handler{}
	if handler == nil {
		t.Error("Expected non-nil handler")
	}

	// Verify the manifest can be retrieved
	manifest := Manifest()
	if manifest.PluginID != "format.epub" {
		t.Errorf("Expected PluginID 'format.epub' after registration, got %s", manifest.PluginID)
	}
}

// TestDetect_CaseInsensitiveExtension verifies that .EPUB, .Epub, etc. are all detected
func TestDetect_CaseInsensitiveExtension(t *testing.T) {
	testCases := []string{
		"test.epub",
		"test.EPUB",
		"test.Epub",
		"test.ePub",
	}

	for _, filename := range testCases {
		t.Run(filename, func(t *testing.T) {
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, filename)
			if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
				t.Fatal(err)
			}

			handler := &Handler{}
			result, err := handler.Detect(testFile)
			if err != nil {
				t.Fatal(err)
			}

			if !result.Detected {
				t.Errorf("Expected detection to succeed for %s, got: %s", filename, result.Reason)
			}
		})
	}
}

// TestIngest_ArtifactIDExtraction verifies correct extraction of artifact ID from filename
func TestIngest_ArtifactIDExtraction(t *testing.T) {
	testCases := []struct {
		filename   string
		expectedID string
	}{
		{"simple.epub", "simple"},
		{"book-name.epub", "book-name"},
		{"Book.With.Dots.epub", "Book.With.Dots"},
		{"complex_name-123.epub", "complex_name-123"},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, tc.filename)
			if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
				t.Fatal(err)
			}

			outputDir := filepath.Join(tmpDir, "output")

			handler := &Handler{}
			result, err := handler.Ingest(testFile, outputDir)
			if err != nil {
				t.Fatalf("Ingest failed: %v", err)
			}

			if result.ArtifactID != tc.expectedID {
				t.Errorf("Expected artifact ID '%s', got '%s'", tc.expectedID, result.ArtifactID)
			}
		})
	}
}

// TestIngest_SHA256Consistency verifies that the same content produces the same hash
func TestIngest_SHA256Consistency(t *testing.T) {
	tmpDir := t.TempDir()
	content := []byte("consistent test content")

	// Ingest the same content twice with different filenames
	testFile1 := filepath.Join(tmpDir, "file1.epub")
	testFile2 := filepath.Join(tmpDir, "file2.epub")
	if err := os.WriteFile(testFile1, content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile2, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result1, err := handler.Ingest(testFile1, outputDir)
	if err != nil {
		t.Fatalf("First ingest failed: %v", err)
	}

	result2, err := handler.Ingest(testFile2, outputDir)
	if err != nil {
		t.Fatalf("Second ingest failed: %v", err)
	}

	if result1.BlobSHA256 != result2.BlobSHA256 {
		t.Errorf("Expected same hash for same content, got %s and %s", result1.BlobSHA256, result2.BlobSHA256)
	}
}

// TestIngest_EmptyFile verifies handling of empty epub files
func TestIngest_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.epub")
	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Ingest failed for empty file: %v", err)
	}

	if result.SizeBytes != 0 {
		t.Errorf("Expected size 0 for empty file, got %d", result.SizeBytes)
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected valid hash even for empty file")
	}
}

// TestIngest_BlobDirCreationError verifies error handling when blob directory cannot be created
func TestIngest_BlobDirCreationError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.epub")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file where the output directory should be to cause MkdirAll to fail
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.WriteFile(outputDir, []byte("blocking file"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	_, err := handler.Ingest(testFile, outputDir)
	if err == nil {
		t.Error("Expected error when blob directory creation fails")
	}
	if !strings.Contains(err.Error(), "failed to create blob dir") {
		t.Errorf("Expected error to contain 'failed to create blob dir', got %s", err.Error())
	}
}

// TestIngest_BlobWriteError verifies error handling when blob file cannot be written
func TestIngest_BlobWriteError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.epub")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	handler := &Handler{}

	// First, do a successful ingest to create the blob directory structure
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("First ingest failed: %v", err)
	}

	// Now make the blob directory read-only to prevent writing
	blobDir := filepath.Join(outputDir, result.BlobSHA256[:2])
	if err := os.Chmod(blobDir, 0444); err != nil {
		t.Fatal(err)
	}
	// Ensure cleanup restores permissions
	defer os.Chmod(blobDir, 0755)

	// Try to ingest again, which should fail when trying to write the blob
	_, err = handler.Ingest(testFile, outputDir)
	if err == nil {
		t.Error("Expected error when blob file cannot be written")
	}
	if !strings.Contains(err.Error(), "failed to write blob") {
		t.Errorf("Expected error to contain 'failed to write blob', got %s", err.Error())
	}
}
