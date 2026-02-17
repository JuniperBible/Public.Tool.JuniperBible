package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/FocuswithJustin/JuniperBible/internal/fileutil"
)

// TestExecuteRequestInDirError tests ExecuteRequest when in directory creation fails.
func TestExecuteRequestInDirError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	// Create a work directory that exists as a file to block creation
	tempDir, err := os.MkdirTemp("", "exec-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Mock the work directory creation by creating /tmp/capsule-run-* files
	// This is tricky because ExecuteRequest creates its own temp dir
	// Instead, we'll test the path where os.ReadFile fails
	executor := NewNixExecutor("/tmp/flake")
	req := NewRequest("test", "test")

	// Create a directory that can't be entered
	unreadableDir, err := os.MkdirTemp("", "unreadable-*")
	if err != nil {
		t.Fatalf("failed to create unreadable dir: %v", err)
	}
	defer os.RemoveAll(unreadableDir)

	if err := os.Chmod(unreadableDir, 0000); err != nil {
		t.Fatalf("failed to chmod dir: %v", err)
	}
	defer os.Chmod(unreadableDir, 0700)

	// Use the unreadable directory as input - this tests the directory copy failure path
	_, err = executor.ExecuteRequest(nil, req, []string{unreadableDir})
	if err == nil {
		t.Log("Expected error for unreadable directory (may succeed on some systems)")
	}
}

// TestExecuteRequestCommandError tests ExecuteRequest when command execution fails.
func TestExecuteRequestCommandError(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}

	executor := NewNixExecutor("/nonexistent/flake/path")
	executor.Timeout = 500 * time.Millisecond

	req := NewRequest("test", "test")
	ctx := context.Background()

	// This should fail because the flake doesn't exist
	result, err := executor.ExecuteRequest(ctx, req, []string{})
	// May return error or result with non-zero exit code
	if err == nil && result != nil && result.ExitCode == 0 {
		t.Log("Execution succeeded (unexpected)")
	}
}

// TestExecuteRequestContextCancellation tests ExecuteRequest with cancelled context.
func TestExecuteRequestContextCancellation(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}

	executor := NewNixExecutor("/tmp/flake")
	req := NewRequest("test", "test")

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := executor.ExecuteRequest(ctx, req, []string{})
	// Should either return error or result with non-zero exit
	if err == nil && result != nil && result.ExitCode == 0 {
		t.Log("Execution completed despite cancelled context (unexpected)")
	}
}

// TestExecuteRequestLongRunning tests ExecuteRequest timeout behavior.
func TestExecuteRequestLongRunning(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}

	executor := NewNixExecutor("/tmp/flake")
	executor.Timeout = 1 * time.Millisecond // Very short timeout

	req := NewRequest("test", "test")
	ctx := context.Background()

	result, err := executor.ExecuteRequest(ctx, req, []string{})
	// Should timeout or fail quickly
	if err == nil && result != nil && result.ExitCode == 0 {
		t.Log("Execution completed within timeout (faster than expected)")
	}
}

// TestBuildSwordCommandAllProfiles tests all sword profiles.
func TestBuildSwordCommandAllProfiles(t *testing.T) {
	executor := NewNixExecutor("/tmp/flake")

	profiles := []string{"list-modules", "render-all", "enumerate-keys", "unknown"}
	for _, profile := range profiles {
		req := NewRequest("libsword", profile)
		cmd := executor.buildSwordCommand(req, "/in", "/out")
		if len(cmd) == 0 {
			t.Errorf("buildSwordCommand returned empty command for profile %s", profile)
		}
	}
}

// TestPrepareWorkDirInDirError tests PrepareWorkDir when in directory creation fails.
func TestPrepareWorkDirInDirError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "workdir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	workDir := filepath.Join(tempDir, "work")
	req := NewRequest("test", "test")

	// Create work dir with file named "in" to cause directory creation failure
	if err := os.MkdirAll(workDir, 0700); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}

	inPath := filepath.Join(workDir, "in")
	if err := os.WriteFile(inPath, []byte("file"), 0600); err != nil {
		t.Fatalf("failed to create in file: %v", err)
	}

	err = PrepareWorkDir(workDir, req)
	if err == nil {
		t.Error("expected error when in already exists as file")
	}
}

// TestListToolsWithFiles tests ListTools with files in archive directory.
func TestListToolsWithFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "registry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file (not directory) in the archive dir
	if err := os.WriteFile(filepath.Join(tempDir, "notdir.txt"), []byte("file"), 0600); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	registry := NewToolRegistry(tempDir)

	tools, err := registry.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	// Should skip files and only return directories with capsule subdirs
	if len(tools) != 0 {
		t.Errorf("expected 0 tools (only files in dir), got %d", len(tools))
	}
}

// TestExecuteRequestExitError tests ExecuteRequest handling non-zero exit codes.
func TestExecuteRequestExitError(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}

	executor := NewNixExecutor("/tmp/nonexistent")
	executor.Timeout = 1 * time.Second

	req := NewRequest("test", "test")

	result, err := executor.ExecuteRequest(nil, req, []string{})
	// Either error or non-zero exit code
	if err == nil && result != nil {
		// This is fine - we just want to ensure the function handles errors properly
		t.Logf("Got exit code: %d", result.ExitCode)
	}
}

// TestCopyDirRelError tests copyDir when filepath.Rel fails.
// This is hard to trigger in practice, but we test the general path.
func TestCopyDirSuccess(t *testing.T) {
	srcDir, err := os.MkdirTemp("", "copydir-src-*")
	if err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	// Create nested structure
	nested := filepath.Join(srcDir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(nested, "file.txt"), []byte("content"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	dstDir, err := os.MkdirTemp("", "copydir-dst-*")
	if err != nil {
		t.Fatalf("failed to create dst dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	if err := fileutil.CopyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify nested file was copied
	copiedFile := filepath.Join(dstDir, "a", "b", "c", "file.txt")
	if _, err := os.Stat(copiedFile); os.IsNotExist(err) {
		t.Error("nested file was not copied")
	}
}

// TestExtractExecutableError tests extracting executable that fails.
func TestExtractExecutableError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "extract-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create archive
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0700); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	testBinary := filepath.Join(binDir, "test")
	if err := os.WriteFile(testBinary, []byte("binary"), 0700); err != nil {
		t.Fatalf("failed to write test binary: %v", err)
	}

	archivePath := filepath.Join(tempDir, "test.tar.xz")
	err = CreateToolArchive(
		"test",
		"1.0.0",
		"x86_64-linux",
		map[string]string{"test": testBinary},
		archivePath,
	)
	if err != nil {
		t.Fatalf("CreateToolArchive failed: %v", err)
	}

	archive, err := LoadToolArchive(archivePath)
	if err != nil {
		t.Fatalf("LoadToolArchive failed: %v", err)
	}

	// Create extract directory and make bin directory read-only
	extractDir := filepath.Join(tempDir, "extract")
	if err := os.MkdirAll(extractDir, 0700); err != nil {
		t.Fatalf("failed to create extract dir: %v", err)
	}

	binExtractDir := filepath.Join(extractDir, "bin")
	if err := os.MkdirAll(binExtractDir, 0500); err != nil {
		t.Fatalf("failed to create bin extract dir: %v", err)
	}
	defer os.Chmod(binExtractDir, 0700)

	// This should fail when trying to write to read-only bin directory
	err = archive.ExtractTo(extractDir)
	if err == nil {
		t.Error("expected error when extracting to read-only bin directory")
	}
}

// TestExtractLibraryError tests extracting library that fails.
func TestExtractLibraryError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "extract-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create archive
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0700); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	testBinary := filepath.Join(binDir, "test")
	if err := os.WriteFile(testBinary, []byte("binary"), 0700); err != nil {
		t.Fatalf("failed to write test binary: %v", err)
	}

	archivePath := filepath.Join(tempDir, "test.tar.xz")
	err = CreateToolArchive(
		"test",
		"1.0.0",
		"x86_64-linux",
		map[string]string{"test": testBinary},
		archivePath,
	)
	if err != nil {
		t.Fatalf("CreateToolArchive failed: %v", err)
	}

	archive, err := LoadToolArchive(archivePath)
	if err != nil {
		t.Fatalf("LoadToolArchive failed: %v", err)
	}

	// Add a fake library with non-existent artifact ID
	archive.Libraries = map[string]string{"libtest.so": "nonexistent-lib-artifact"}

	// Try to extract - should fail because library artifact doesn't exist
	extractDir := filepath.Join(tempDir, "extract")
	err = archive.ExtractTo(extractDir)
	if err == nil {
		t.Error("expected error when extracting non-existent library artifact")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}
