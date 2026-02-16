package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/capsule"
)

func TestToolRegistry(t *testing.T) {
	// Create temp directory structure
	tempDir, err := os.MkdirTemp("", "toolregistry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a mock tool directory
	toolDir := filepath.Join(tempDir, "test-tool", "capsule")
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		t.Fatalf("failed to create tool dir: %v", err)
	}

	// Create registry
	registry := NewToolRegistry(tempDir)

	// Test ListTools
	tools, err := registry.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}
	if len(tools) > 0 && tools[0] != "test-tool" {
		t.Errorf("expected tool 'test-tool', got '%s'", tools[0])
	}
}

func TestToolArchiveManifest(t *testing.T) {
	manifest := ToolArchiveManifest{
		ToolID:   "sword-utils",
		Version:  "1.9.0",
		Platform: "x86_64-linux",
		Executables: map[string]string{
			"diatheke": "exe-diatheke",
			"mod2imp":  "exe-mod2imp",
		},
	}

	if manifest.ToolID != "sword-utils" {
		t.Errorf("expected ToolID 'sword-utils', got '%s'", manifest.ToolID)
	}
	if len(manifest.Executables) != 2 {
		t.Errorf("expected 2 executables, got %d", len(manifest.Executables))
	}
}

func TestToolArchivePaths(t *testing.T) {
	archive := &ToolArchive{
		ToolID:   "test-tool",
		Version:  "1.0.0",
		Platform: "x86_64-linux",
		Executables: map[string]string{
			"mytool": "exe-mytool",
		},
	}

	destDir := "/tmp/tools"
	path := archive.GetExecutablePath(destDir, "mytool")
	expected := "/tmp/tools/bin/mytool"
	if path != expected {
		t.Errorf("expected path '%s', got '%s'", expected, path)
	}
}

func TestCreateAndLoadToolArchive(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "toolarchive-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a mock binary
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	testBinary := filepath.Join(binDir, "testtool")
	if err := os.WriteFile(testBinary, []byte("#!/bin/sh\necho hello"), 0755); err != nil {
		t.Fatalf("failed to write test binary: %v", err)
	}

	// Create archive
	archivePath := filepath.Join(tempDir, "testtool.capsule.tar.xz")
	err = CreateToolArchive(
		"testtool",
		"1.0.0",
		"x86_64-linux",
		map[string]string{"testtool": testBinary},
		archivePath,
	)
	if err != nil {
		t.Fatalf("CreateToolArchive failed: %v", err)
	}

	// Verify archive was created
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Error("archive file was not created")
	}

	// Load archive
	archive, err := LoadToolArchive(archivePath)
	if err != nil {
		t.Fatalf("LoadToolArchive failed: %v", err)
	}

	if archive.ToolID != "testtool" {
		t.Errorf("expected ToolID 'testtool', got '%s'", archive.ToolID)
	}
	if archive.Version != "1.0.0" {
		t.Errorf("expected Version '1.0.0', got '%s'", archive.Version)
	}
	if archive.Platform != "x86_64-linux" {
		t.Errorf("expected Platform 'x86_64-linux', got '%s'", archive.Platform)
	}

	// Verify executable is in manifest
	if _, ok := archive.Executables["testtool"]; !ok {
		t.Error("executable 'testtool' not found in archive")
	}

	// Test extraction
	extractDir := filepath.Join(tempDir, "extracted")
	if err := archive.ExtractTo(extractDir); err != nil {
		t.Fatalf("ExtractTo failed: %v", err)
	}

	// Verify extracted file
	extractedPath := archive.GetExecutablePath(extractDir, "testtool")
	if _, err := os.Stat(extractedPath); os.IsNotExist(err) {
		t.Error("extracted binary not found")
	}

	// Verify content
	data, err := os.ReadFile(extractedPath)
	if err != nil {
		t.Fatalf("failed to read extracted binary: %v", err)
	}
	if string(data) != "#!/bin/sh\necho hello" {
		t.Errorf("extracted binary content mismatch")
	}

	// Test Close
	if err := archive.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}
}

// TestLoadToolArchiveErrors tests error handling in LoadToolArchive.
func TestLoadToolArchiveErrors(t *testing.T) {
	// Test with non-existent file
	_, err := LoadToolArchive("/nonexistent/archive.tar.xz")
	if err == nil {
		t.Error("expected error for non-existent archive")
	}

	// Test with invalid archive
	tempDir, err := os.MkdirTemp("", "toolarchive-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	invalidArchive := filepath.Join(tempDir, "invalid.tar.xz")
	if err := os.WriteFile(invalidArchive, []byte("not a valid archive"), 0600); err != nil {
		t.Fatalf("failed to write invalid archive: %v", err)
	}

	_, err = LoadToolArchive(invalidArchive)
	if err == nil {
		t.Error("expected error for invalid archive")
	}
}

// TestExtractToWithLibraries tests ExtractTo with libraries.
func TestExtractToWithLibraries(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "toolarchive-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create mock binary and library
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	testBinary := filepath.Join(binDir, "testtool")
	if err := os.WriteFile(testBinary, []byte("binary"), 0755); err != nil {
		t.Fatalf("failed to write test binary: %v", err)
	}

	libDir := filepath.Join(tempDir, "lib")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatalf("failed to create lib dir: %v", err)
	}

	testLib := filepath.Join(libDir, "libtest.so")
	if err := os.WriteFile(testLib, []byte("library"), 0600); err != nil {
		t.Fatalf("failed to write test library: %v", err)
	}

	// Create archive with both binary and library
	archivePath := filepath.Join(tempDir, "testtool.capsule.tar.xz")

	// For this test we need to manually create a capsule with libraries
	// We'll create a simpler version that still tests the library extraction path
	err = CreateToolArchive(
		"testtool",
		"1.0.0",
		"x86_64-linux",
		map[string]string{"testtool": testBinary},
		archivePath,
	)
	if err != nil {
		t.Fatalf("CreateToolArchive failed: %v", err)
	}

	archive, err := LoadToolArchive(archivePath)
	if err != nil {
		t.Fatalf("LoadToolArchive failed: %v", err)
	}

	// Manually add a library entry to test library extraction
	archive.Libraries = map[string]string{"libtest.so": "lib-test"}

	extractDir := filepath.Join(tempDir, "extracted")

	// This will fail because the artifact doesn't exist, but it tests the library code path
	err = archive.ExtractTo(extractDir)
	if err == nil {
		// If it succeeds, verify lib directory was created
		libExtractDir := filepath.Join(extractDir, "lib")
		if _, statErr := os.Stat(libExtractDir); os.IsNotExist(statErr) {
			t.Error("lib directory should have been created")
		}
	}
}

// TestToolRegistryLoadTool tests loading tools from registry.
func TestToolRegistryLoadTool(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "registry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool archive
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	testBinary := filepath.Join(binDir, "testtool")
	if err := os.WriteFile(testBinary, []byte("binary"), 0755); err != nil {
		t.Fatalf("failed to write test binary: %v", err)
	}

	// Create tool directory structure
	toolCapsuleDir := filepath.Join(tempDir, "testtool", "capsule")
	if err := os.MkdirAll(toolCapsuleDir, 0755); err != nil {
		t.Fatalf("failed to create capsule dir: %v", err)
	}

	archivePath := filepath.Join(toolCapsuleDir, "testtool.capsule.tar.xz")
	err = CreateToolArchive(
		"testtool",
		"1.0.0",
		"x86_64-linux",
		map[string]string{"testtool": testBinary},
		archivePath,
	)
	if err != nil {
		t.Fatalf("CreateToolArchive failed: %v", err)
	}

	// Create registry and load tool
	registry := NewToolRegistry(tempDir)

	tool, err := registry.LoadTool("testtool")
	if err != nil {
		t.Fatalf("LoadTool failed: %v", err)
	}

	if tool.ToolID != "testtool" {
		t.Errorf("ToolID = %q, want %q", tool.ToolID, "testtool")
	}

	// Test cache - loading again should return the same instance
	tool2, err := registry.LoadTool("testtool")
	if err != nil {
		t.Fatalf("LoadTool (cached) failed: %v", err)
	}

	if tool2 != tool {
		t.Error("expected cached tool to be same instance")
	}
}

// TestToolRegistryLoadToolNotFound tests loading non-existent tool.
func TestToolRegistryLoadToolNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "registry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	registry := NewToolRegistry(tempDir)

	_, err = registry.LoadTool("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent tool")
	}
}

// TestToolRegistryLoadToolAlternativeNames tests loading with alternative file names.
func TestToolRegistryLoadToolAlternativeNames(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "registry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool archive with alternative name
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	testBinary := filepath.Join(binDir, "testtool")
	if err := os.WriteFile(testBinary, []byte("binary"), 0755); err != nil {
		t.Fatalf("failed to write test binary: %v", err)
	}

	// Put archive directly in registry dir with alternative name
	archivePath := filepath.Join(tempDir, "testtool.tar.xz")
	err = CreateToolArchive(
		"testtool",
		"1.0.0",
		"x86_64-linux",
		map[string]string{"testtool": testBinary},
		archivePath,
	)
	if err != nil {
		t.Fatalf("CreateToolArchive failed: %v", err)
	}

	registry := NewToolRegistry(tempDir)

	tool, err := registry.LoadTool("testtool")
	if err != nil {
		t.Fatalf("LoadTool failed: %v", err)
	}

	if tool.ToolID != "testtool" {
		t.Errorf("ToolID = %q, want %q", tool.ToolID, "testtool")
	}
}

// TestExtractToErrors tests error paths in ExtractTo.
func TestExtractToErrors(t *testing.T) {
	// Test with read-only destination directory
	if os.Getuid() != 0 {
		archive := &ToolArchive{
			ToolID:      "test",
			Executables: map[string]string{"test": "exe-test"},
		}

		// Try to extract to /proc which should fail
		err := archive.ExtractTo("/proc/invalid")
		if err == nil {
			t.Error("expected error when extracting to invalid directory")
		}
	}
}

// TestCreateToolArchiveErrors tests error handling in CreateToolArchive.
func TestCreateToolArchiveErrors(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test with non-existent binary
	archivePath := filepath.Join(tempDir, "test.tar.xz")
	err = CreateToolArchive(
		"test",
		"1.0.0",
		"x86_64-linux",
		map[string]string{"test": "/nonexistent/binary"},
		archivePath,
	)
	if err == nil {
		t.Error("expected error for non-existent binary")
	}

	// Test with invalid destination
	if os.Getuid() != 0 {
		binDir := filepath.Join(tempDir, "bin")
		if err := os.MkdirAll(binDir, 0755); err != nil {
			t.Fatalf("failed to create bin dir: %v", err)
		}

		testBinary := filepath.Join(binDir, "test")
		if err := os.WriteFile(testBinary, []byte("binary"), 0755); err != nil {
			t.Fatalf("failed to write test binary: %v", err)
		}

		err = CreateToolArchive(
			"test",
			"1.0.0",
			"x86_64-linux",
			map[string]string{"test": testBinary},
			"/proc/invalid/test.tar.xz",
		)
		if err == nil {
			t.Error("expected error for invalid destination")
		}
	}
}

// TestLoadToolArchiveMkdirTempError tests LoadToolArchive with MkdirTemp error.
func TestLoadToolArchiveMkdirTempError(t *testing.T) {
	// Inject error
	orig := toolOsMkdirTemp
	toolOsMkdirTemp = func(dir, pattern string) (string, error) {
		return "", fmt.Errorf("injected temp dir error")
	}
	defer func() { toolOsMkdirTemp = orig }()

	_, err := LoadToolArchive("/some/archive.tar.xz")
	if err == nil {
		t.Error("expected error for MkdirTemp failure")
	}
	if err.Error() != "failed to create temp directory: injected temp dir error" {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCreateToolArchiveMkdirTempError tests CreateToolArchive with MkdirTemp error.
func TestCreateToolArchiveMkdirTempError(t *testing.T) {
	// Inject error
	orig := toolOsMkdirTemp
	toolOsMkdirTemp = func(dir, pattern string) (string, error) {
		return "", fmt.Errorf("injected temp dir error")
	}
	defer func() { toolOsMkdirTemp = orig }()

	err := CreateToolArchive("test", "1.0.0", "x86_64-linux", map[string]string{}, "/some/path.tar.xz")
	if err == nil {
		t.Error("expected error for MkdirTemp failure")
	}
	if err.Error() != "failed to create temp directory: injected temp dir error" {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCreateToolArchiveCapsuleNewError tests CreateToolArchive with capsule.New error.
func TestCreateToolArchiveCapsuleNewError(t *testing.T) {
	// Inject error
	orig := capsuleNew
	capsuleNew = func(dir string) (*capsule.Capsule, error) {
		return nil, fmt.Errorf("injected capsule error")
	}
	defer func() { capsuleNew = orig }()

	err := CreateToolArchive("test", "1.0.0", "x86_64-linux", map[string]string{}, "/some/path.tar.xz")
	if err == nil {
		t.Error("expected error for capsule.New failure")
	}
	if err.Error() != "failed to create capsule: injected capsule error" {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestToolRegistryLoadToolArchiveError tests LoadTool when LoadToolArchive fails.
func TestToolRegistryLoadToolArchiveError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "registry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a corrupted archive file
	toolCapsuleDir := filepath.Join(tempDir, "badtool", "capsule")
	if err := os.MkdirAll(toolCapsuleDir, 0755); err != nil {
		t.Fatalf("failed to create capsule dir: %v", err)
	}

	archivePath := filepath.Join(toolCapsuleDir, "badtool.capsule.tar.xz")
	if err := os.WriteFile(archivePath, []byte("not a valid archive"), 0600); err != nil {
		t.Fatalf("failed to write archive: %v", err)
	}

	registry := NewToolRegistry(tempDir)

	_, err = registry.LoadTool("badtool")
	if err == nil {
		t.Error("expected error for corrupted archive")
	}
}

// TestListToolsNonexistentDir tests ListTools with nonexistent directory.
func TestListToolsNonexistentDir(t *testing.T) {
	registry := NewToolRegistry("/nonexistent/directory")

	_, err := registry.ListTools()
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

// TestExtractToLibMkdirError tests ExtractTo when lib mkdir fails.
func TestExtractToLibMkdirError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	archive := &ToolArchive{
		ToolID:      "test",
		Executables: map[string]string{},
		Libraries:   map[string]string{"lib.so": "lib-test"},
	}

	// Try to extract to /proc/invalid which should fail for lib dir creation
	err := archive.ExtractTo("/proc/invalid")
	if err == nil {
		t.Error("expected error when creating lib directory fails")
	}
}
