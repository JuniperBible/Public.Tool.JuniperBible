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
