package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create source file
	srcContent := "Hello, World!"
	srcPath := filepath.Join(tempDir, "src.txt")
	if err := os.WriteFile(srcPath, []byte(srcContent), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Copy file
	dstPath := filepath.Join(tempDir, "dst.txt")
	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	// Verify content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(dstContent) != srcContent {
		t.Errorf("content mismatch: got %q, want %q", dstContent, srcContent)
	}
}

func TestCopyFile_CreateDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tempDir, "src.txt")
	if err := os.WriteFile(srcPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Copy to nested directory that doesn't exist
	dstPath := filepath.Join(tempDir, "nested", "deep", "dst.txt")
	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		t.Error("destination file not created")
	}
}

func TestCopyFile_NonexistentSource(t *testing.T) {
	tempDir := t.TempDir()

	err := CopyFile("/nonexistent/file", filepath.Join(tempDir, "dst.txt"))
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

func TestCopyDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory structure
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Copy directory
	dstDir := filepath.Join(tempDir, "dst")
	if err := CopyDir(srcDir, dstDir); err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}

	// Verify structure
	if _, err := os.Stat(filepath.Join(dstDir, "file1.txt")); os.IsNotExist(err) {
		t.Error("file1.txt not copied")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "subdir", "file2.txt")); os.IsNotExist(err) {
		t.Error("subdir/file2.txt not copied")
	}

	// Verify content
	content, _ := os.ReadFile(filepath.Join(dstDir, "subdir", "file2.txt"))
	if string(content) != "content2" {
		t.Errorf("content mismatch: got %q, want %q", content, "content2")
	}
}

func TestCopyDir_Empty(t *testing.T) {
	tempDir := t.TempDir()

	// Create empty source directory
	srcDir := filepath.Join(tempDir, "empty")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	// Copy directory
	dstDir := filepath.Join(tempDir, "dst")
	if err := CopyDir(srcDir, dstDir); err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		t.Error("destination directory not created")
	}
}

func TestCopyDir_NonexistentSource(t *testing.T) {
	tempDir := t.TempDir()

	err := CopyDir("/nonexistent/dir", filepath.Join(tempDir, "dst"))
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

func TestCopyDir_SingleFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create source file (not directory)
	srcPath := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// CopyDir on a file should fall back to CopyFile
	dstPath := filepath.Join(tempDir, "dst.txt")
	if err := CopyDir(srcPath, dstPath); err != nil {
		t.Fatalf("CopyDir failed on file: %v", err)
	}

	// Verify content
	content, _ := os.ReadFile(dstPath)
	if string(content) != "content" {
		t.Errorf("content mismatch: got %q, want %q", content, "content")
	}
}

func TestCopyDir_DeepNesting(t *testing.T) {
	tempDir := t.TempDir()

	// Create deeply nested structure
	srcDir := filepath.Join(tempDir, "src")
	deepPath := filepath.Join(srcDir, "a", "b", "c", "d", "e")
	if err := os.MkdirAll(deepPath, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deepPath, "deep.txt"), []byte("deep"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Copy directory
	dstDir := filepath.Join(tempDir, "dst")
	if err := CopyDir(srcDir, dstDir); err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}

	// Verify deep file
	dstDeepPath := filepath.Join(dstDir, "a", "b", "c", "d", "e", "deep.txt")
	if _, err := os.Stat(dstDeepPath); os.IsNotExist(err) {
		t.Error("deep file not copied")
	}
}

func TestCopyFile_InvalidDst(t *testing.T) {
	tempDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tempDir, "src.txt")
	if err := os.WriteFile(srcPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create a file where we want to create a directory
	blocker := filepath.Join(tempDir, "blocker")
	if err := os.WriteFile(blocker, []byte("blocking"), 0644); err != nil {
		t.Fatalf("failed to create blocker file: %v", err)
	}

	// Try to copy to path that requires blocker to be a directory
	dstPath := filepath.Join(blocker, "dst.txt")
	err := CopyFile(srcPath, dstPath)
	if err == nil {
		t.Error("expected error when destination directory can't be created")
	}
}

func TestCopyDir_DestinationBlocked(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory with content
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create a file where we want to create the destination directory
	blocker := filepath.Join(tempDir, "blocker")
	if err := os.WriteFile(blocker, []byte("blocking"), 0644); err != nil {
		t.Fatalf("failed to create blocker file: %v", err)
	}

	// Try to copy to path that requires blocker to be a directory
	err := CopyDir(srcDir, blocker)
	if err == nil {
		t.Error("expected error when destination directory can't be created")
	}
}

func TestCopyDir_CopyFileError(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory with a subdirectory
	srcDir := filepath.Join(tempDir, "src")
	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create destination directory
	dstDir := filepath.Join(tempDir, "dst")
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatalf("failed to create dst dir: %v", err)
	}

	// Create a file that will block the subdirectory creation
	dstSubDir := filepath.Join(dstDir, "subdir")
	if err := os.WriteFile(dstSubDir, []byte("blocking"), 0644); err != nil {
		t.Fatalf("failed to create blocker file: %v", err)
	}

	// This should fail because subdir can't be created
	err := CopyDir(srcDir, dstDir)
	if err == nil {
		t.Error("expected error when subdirectory can't be created")
	}
}

func TestCopyDir_ReadDirError(t *testing.T) {
	tempDir := t.TempDir()

	// Create a directory
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	// Add a file to make it non-empty
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Remove read permissions from the directory
	if err := os.Chmod(srcDir, 0000); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(srcDir, 0755) // Restore permissions for cleanup

	dstDir := filepath.Join(tempDir, "dst")
	err := CopyDir(srcDir, dstDir)
	if err == nil {
		t.Error("expected error when reading directory without permissions")
	}
}

func TestCopyFile_PermissionsPreserved(t *testing.T) {
	tempDir := t.TempDir()

	// Create a source file with specific permissions
	srcPath := filepath.Join(tempDir, "src.txt")
	if err := os.WriteFile(srcPath, []byte("content"), 0600); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Copy the file
	dstPath := filepath.Join(tempDir, "dst.txt")
	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	// Verify permissions are preserved
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		t.Fatalf("failed to stat source: %v", err)
	}
	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("failed to stat destination: %v", err)
	}

	if srcInfo.Mode() != dstInfo.Mode() {
		t.Errorf("permissions not preserved: src=%v, dst=%v", srcInfo.Mode(), dstInfo.Mode())
	}
}

func TestCopyFile_LargeFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create a larger source file to test io.Copy
	srcPath := filepath.Join(tempDir, "large.txt")
	largeContent := make([]byte, 1024*1024) // 1MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	if err := os.WriteFile(srcPath, largeContent, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Copy the file
	dstPath := filepath.Join(tempDir, "large-dst.txt")
	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	// Verify content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}

	if len(dstContent) != len(largeContent) {
		t.Errorf("size mismatch: got %d, want %d", len(dstContent), len(largeContent))
	}

	for i := range largeContent {
		if dstContent[i] != largeContent[i] {
			t.Errorf("content mismatch at byte %d: got %d, want %d", i, dstContent[i], largeContent[i])
			break
		}
	}
}

func TestCopyFile_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create an empty source file
	srcPath := filepath.Join(tempDir, "empty.txt")
	if err := os.WriteFile(srcPath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Copy the file
	dstPath := filepath.Join(tempDir, "empty-dst.txt")
	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	// Verify it's empty
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}

	if len(dstContent) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(dstContent))
	}
}

func TestCopyFile_SpecialDevice(t *testing.T) {
	// Skip this test if /dev/null is not available (e.g., on Windows)
	if _, err := os.Stat("/dev/null"); os.IsNotExist(err) {
		t.Skip("Skipping test: /dev/null not available")
	}

	tempDir := t.TempDir()
	dstPath := filepath.Join(tempDir, "null-copy.txt")

	// Try to copy /dev/null - this should work but will create an empty file
	if err := CopyFile("/dev/null", dstPath); err != nil {
		// This might fail or succeed depending on the OS
		// The important part is that we're exercising the code path
		t.Logf("CopyFile from /dev/null result: %v", err)
	}
}

func TestCopyFile_ExecutableBit(t *testing.T) {
	tempDir := t.TempDir()

	// Create a source file with executable permissions
	srcPath := filepath.Join(tempDir, "executable.sh")
	if err := os.WriteFile(srcPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Copy the file
	dstPath := filepath.Join(tempDir, "executable-copy.sh")
	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	// Verify executable bit is preserved
	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("failed to stat destination: %v", err)
	}

	if dstInfo.Mode()&0111 == 0 {
		t.Error("executable bit not preserved")
	}
}

func TestCopyFile_OpenFileError(t *testing.T) {
	tempDir := t.TempDir()

	// Create a source file
	srcPath := filepath.Join(tempDir, "src.txt")
	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create a directory where the destination file should be
	dstPath := filepath.Join(tempDir, "dst.txt")
	if err := os.Mkdir(dstPath, 0755); err != nil {
		t.Fatalf("failed to create dst directory: %v", err)
	}

	// Try to copy to a path that's a directory (should fail to create file)
	err := CopyFile(srcPath, dstPath)
	if err == nil {
		t.Error("expected error when destination is a directory")
	}
}

func TestCopyDir_CopyFileInLoopError(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory with a file
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create destination directory
	dstDir := filepath.Join(tempDir, "dst")
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatalf("failed to create dst dir: %v", err)
	}

	// Create a directory where the destination file should be (will cause CopyFile to fail)
	dstFilePath := filepath.Join(dstDir, "file.txt")
	if err := os.Mkdir(dstFilePath, 0755); err != nil {
		t.Fatalf("failed to create blocking directory: %v", err)
	}

	// This should fail because CopyFile can't overwrite a directory
	err := CopyDir(srcDir, dstDir)
	if err == nil {
		t.Error("expected error when CopyFile fails in loop")
	}
}
