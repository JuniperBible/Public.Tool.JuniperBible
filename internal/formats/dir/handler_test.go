package dir

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func TestDetect_ValidDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	h := &Handler{}
	result, err := h.Detect(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Detected {
		t.Errorf("Expected directory to be detected, got: %s", result.Reason)
	}
	if result.Format != "directory" {
		t.Errorf("Expected format 'directory', got: %s", result.Format)
	}
	if result.Reason != "is a directory" {
		t.Errorf("Expected reason 'is a directory', got: %s", result.Reason)
	}
}

func TestDetect_File(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Detected {
		t.Error("Expected file not to be detected as directory")
	}
	if result.Reason != "not a directory" {
		t.Errorf("Expected reason 'not a directory', got: %s", result.Reason)
	}
}

func TestDetect_NonExistent(t *testing.T) {
	nonExistentPath := "/tmp/nonexistent_dir_12345678"

	h := &Handler{}
	result, err := h.Detect(nonExistentPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Detected {
		t.Error("Expected non-existent path not to be detected")
	}
	if result.Reason == "" {
		t.Error("Expected reason to be set for non-existent path")
	}
}

func TestIngest_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	result, err := h.Ingest(tmpDir, outputDir)

	if err == nil {
		t.Error("Expected Ingest to return an error")
	}
	if result != nil {
		t.Errorf("Expected nil result, got: %v", result)
	}
	expectedMsg := "directory format does not support direct ingest"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got: %s", expectedMsg, err.Error())
	}
}

func TestEnumerate_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	h := &Handler{}
	result, err := h.Enumerate(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if len(result.Entries) != 0 {
		t.Errorf("Expected 0 entries in empty directory, got: %d", len(result.Entries))
	}
}

func TestEnumerate_WithFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	content1 := []byte("content1")
	content2 := []byte("content2")

	if err := os.WriteFile(file1, content1, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, content2, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Enumerate(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if len(result.Entries) != 2 {
		t.Fatalf("Expected 2 entries, got: %d", len(result.Entries))
	}

	// Check entries
	for _, entry := range result.Entries {
		if entry.Path != "file1.txt" && entry.Path != "file2.txt" {
			t.Errorf("Unexpected entry path: %s", entry.Path)
		}
		if entry.IsDir {
			t.Errorf("Expected IsDir to be false for file: %s", entry.Path)
		}
		expectedSize := int64(len(content1))
		if entry.SizeBytes != expectedSize {
			t.Errorf("Expected size %d for %s, got: %d", expectedSize, entry.Path, entry.SizeBytes)
		}
	}
}

func TestEnumerate_NestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested structure
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	file1 := filepath.Join(tmpDir, "root.txt")
	file2 := filepath.Join(subDir, "nested.txt")

	if err := os.WriteFile(file1, []byte("root content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("nested content"), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Enumerate(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	// Should have: root.txt, subdir, subdir/nested.txt = 3 entries
	if len(result.Entries) != 3 {
		t.Fatalf("Expected 3 entries (1 file + 1 dir + 1 nested file), got: %d", len(result.Entries))
	}

	// Verify we have the expected entries
	foundRootFile := false
	foundSubDir := false
	foundNestedFile := false

	for _, entry := range result.Entries {
		switch entry.Path {
		case "root.txt":
			foundRootFile = true
			if entry.IsDir {
				t.Error("Expected root.txt to not be a directory")
			}
		case "subdir":
			foundSubDir = true
			if !entry.IsDir {
				t.Error("Expected subdir to be a directory")
			}
		case filepath.Join("subdir", "nested.txt"):
			foundNestedFile = true
			if entry.IsDir {
				t.Error("Expected nested.txt to not be a directory")
			}
		}
	}

	if !foundRootFile {
		t.Error("Did not find root.txt in entries")
	}
	if !foundSubDir {
		t.Error("Did not find subdir in entries")
	}
	if !foundNestedFile {
		t.Error("Did not find nested.txt in entries")
	}
}

func TestEnumerate_SymlinkHandling(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file
	targetFile := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(targetFile, []byte("target content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink to the file
	symlinkPath := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(targetFile, symlinkPath); err != nil {
		// Skip test if symlinks are not supported (e.g., on some Windows systems)
		t.Skipf("Symlinks not supported: %v", err)
	}

	h := &Handler{}
	result, err := h.Enumerate(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Should enumerate both the target file and the symlink
	if len(result.Entries) < 2 {
		t.Errorf("Expected at least 2 entries, got: %d", len(result.Entries))
	}

	// Verify entries exist
	foundTarget := false
	foundLink := false

	for _, entry := range result.Entries {
		if entry.Path == "target.txt" {
			foundTarget = true
		}
		if entry.Path == "link.txt" {
			foundLink = true
		}
	}

	if !foundTarget {
		t.Error("Did not find target.txt in entries")
	}
	if !foundLink {
		t.Error("Did not find link.txt in entries")
	}
}

func TestEnumerate_ErrorDuringWalk(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file in the subdirectory
	testFile := filepath.Join(subDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Remove read permissions from the subdirectory to cause an error during walk
	if err := os.Chmod(subDir, 0000); err != nil {
		t.Fatal(err)
	}
	// Restore permissions after test
	defer os.Chmod(subDir, 0755)

	h := &Handler{}
	result, err := h.Enumerate(tmpDir)

	// On some systems, this may succeed even without permissions,
	// but on most Unix systems it should fail
	if err != nil {
		// Expected error case
		if result != nil {
			t.Errorf("Expected nil result on error, got: %v", result)
		}
		// Verify error message contains expected text
		if err.Error() == "" {
			t.Error("Expected non-empty error message")
		}
	} else {
		// If it succeeded despite no permissions, that's also valid on some systems
		t.Logf("Enumerate succeeded despite permission restrictions (system-dependent behavior)")
	}
}

func TestEnumerate_NonExistentDirectory(t *testing.T) {
	nonExistentPath := "/tmp/nonexistent_dir_for_enumerate_12345678"

	h := &Handler{}
	result, err := h.Enumerate(nonExistentPath)

	if err == nil {
		t.Error("Expected error when enumerating non-existent directory")
	}
	if result != nil {
		t.Errorf("Expected nil result on error, got: %v", result)
	}
	if err != nil && err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestExtractIR_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	result, err := h.ExtractIR(tmpDir, outputDir)

	if err == nil {
		t.Error("Expected ExtractIR to return an error")
	}
	if result != nil {
		t.Errorf("Expected nil result, got: %v", result)
	}
	expectedMsg := "directory format does not support IR extraction"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got: %s", expectedMsg, err.Error())
	}
}

func TestEmitNative_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "ir")
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	result, err := h.EmitNative(irPath, outputDir)

	if err == nil {
		t.Error("Expected EmitNative to return an error")
	}
	if result != nil {
		t.Errorf("Expected nil result, got: %v", result)
	}
	expectedMsg := "directory format does not support native emission"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got: %s", expectedMsg, err.Error())
	}
}

func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest == nil {
		t.Fatal("Expected non-nil manifest")
	}

	if manifest.PluginID != "format.dir" {
		t.Errorf("Expected PluginID 'format.dir', got: %s", manifest.PluginID)
	}

	if manifest.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got: %s", manifest.Version)
	}

	if manifest.Kind != "format" {
		t.Errorf("Expected Kind 'format', got: %s", manifest.Kind)
	}

	if manifest.Entrypoint != "format-dir" {
		t.Errorf("Expected Entrypoint 'format-dir', got: %s", manifest.Entrypoint)
	}

	// Check capabilities
	if len(manifest.Capabilities.Inputs) != 1 {
		t.Fatalf("Expected 1 input capability, got: %d", len(manifest.Capabilities.Inputs))
	}
	if manifest.Capabilities.Inputs[0] != "directory" {
		t.Errorf("Expected input 'directory', got: %s", manifest.Capabilities.Inputs[0])
	}

	if len(manifest.Capabilities.Outputs) != 1 {
		t.Fatalf("Expected 1 output capability, got: %d", len(manifest.Capabilities.Outputs))
	}
	if manifest.Capabilities.Outputs[0] != "artifact.kind:directory" {
		t.Errorf("Expected output 'artifact.kind:directory', got: %s", manifest.Capabilities.Outputs[0])
	}
}

// TestRegister verifies that the plugin is registered with the embedded registry
func TestRegister(t *testing.T) {
	// Get all registered plugins
	plugins := plugins.ListEmbeddedPlugins()

	// Look for our plugin
	found := false
	for _, p := range plugins {
		if p.Manifest.PluginID == "format.dir" {
			found = true
			if p.Format == nil {
				t.Error("Expected Format handler to be set")
			}
			break
		}
	}

	if !found {
		t.Error("Expected format.dir plugin to be registered")
	}
}
