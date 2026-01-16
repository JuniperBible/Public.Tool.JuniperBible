package archive

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateTarGz(t *testing.T) {
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

	// Create archive
	dstPath := filepath.Join(tempDir, "output", "test.tar.gz")
	if err := CreateTarGz(srcDir, dstPath, "myarchive", true); err != nil {
		t.Fatalf("CreateTarGz failed: %v", err)
	}

	// Verify archive exists
	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		t.Error("archive file not created")
	}

	// Verify archive content (directories have trailing slashes)
	files := readTarGzFiles(t, dstPath)
	expected := map[string]bool{
		"myarchive/file1.txt":        false,
		"myarchive/subdir/":          false,
		"myarchive/subdir/file2.txt": false,
	}
	for _, f := range files {
		if _, ok := expected[f]; ok {
			expected[f] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("missing file in archive: %s (got: %v)", name, files)
		}
	}
}

func TestCreateTarGz_NoParentDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create archive without creating parent dir (should fail if parent doesn't exist)
	dstPath := filepath.Join(tempDir, "nonexistent", "test.tar.gz")
	err := CreateTarGz(srcDir, dstPath, "test", false)
	if err == nil {
		t.Error("expected error when parent directory doesn't exist")
	}
}

func TestCreateTarGz_EmptyDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create empty source directory
	srcDir := filepath.Join(tempDir, "empty")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	// Create archive
	dstPath := filepath.Join(tempDir, "empty.tar.gz")
	if err := CreateTarGz(srcDir, dstPath, "empty", false); err != nil {
		t.Fatalf("CreateTarGz failed: %v", err)
	}

	// Verify archive exists
	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		t.Error("archive file not created")
	}
}

func TestCreateCapsuleTarGz(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory
	srcDir := filepath.Join(tempDir, "mycapsule")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "manifest.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create capsule archive
	dstPath := filepath.Join(tempDir, "output", "mycapsule.tar.gz")
	if err := CreateCapsuleTarGz(srcDir, dstPath); err != nil {
		t.Fatalf("CreateCapsuleTarGz failed: %v", err)
	}

	// Verify base dir is derived from srcDir
	files := readTarGzFiles(t, dstPath)
	found := false
	for _, f := range files {
		if f == "mycapsule/manifest.json" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected mycapsule/manifest.json in archive")
	}
}

func TestCreateTarGz_NonexistentSource(t *testing.T) {
	tempDir := t.TempDir()

	err := CreateTarGz("/nonexistent/source", filepath.Join(tempDir, "test.tar.gz"), "test", false)
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

func TestCreateCapsuleTarGzFromPath(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory
	srcDir := filepath.Join(tempDir, "srcdata")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "manifest.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create capsule archive - base dir should be derived from dstPath
	dstPath := filepath.Join(tempDir, "mytest.capsule.tar.gz")
	if err := CreateCapsuleTarGzFromPath(srcDir, dstPath); err != nil {
		t.Fatalf("CreateCapsuleTarGzFromPath failed: %v", err)
	}

	// Verify base dir is "mytest" (derived from dstPath, not srcDir)
	files := readTarGzFiles(t, dstPath)
	found := false
	for _, f := range files {
		if f == "mytest/manifest.json" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mytest/manifest.json in archive, got: %v", files)
	}
}

// readTarGzFiles is a helper to read file names from a tar.gz archive.
func readTarGzFiles(t *testing.T, path string) []string {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	var files []string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read tar header: %v", err)
		}
		files = append(files, header.Name)
	}

	return files
}

func TestCreateTarGz_InvalidDestination(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Try to create archive in a location where we cannot write
	// Use a file path as the parent directory (invalid)
	invalidParent := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(invalidParent, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	dstPath := filepath.Join(invalidParent, "test.tar.gz")
	err := CreateTarGz(srcDir, dstPath, "test", true)
	if err == nil {
		t.Error("expected error when creating archive with invalid parent")
	}
}

func TestCreateTarGz_FileOpenError(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory with a file we can't open
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	// Create a subdirectory with a file
	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	testFile := filepath.Join(subDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Make the file unreadable
	if err := os.Chmod(testFile, 0000); err != nil {
		t.Fatalf("failed to chmod file: %v", err)
	}
	defer os.Chmod(testFile, 0644) // cleanup

	dstPath := filepath.Join(tempDir, "test.tar.gz")
	err := CreateTarGz(srcDir, dstPath, "test", false)
	if err == nil {
		t.Error("expected error when archiving unreadable file")
	}
}

func TestCreateTarGz_DeepNesting(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory with deeply nested structure
	srcDir := filepath.Join(tempDir, "src")
	deepDir := filepath.Join(srcDir, "a", "b", "c", "d", "e")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	// Create a file in the deep directory
	if err := os.WriteFile(filepath.Join(deepDir, "deep.txt"), []byte("deep content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	dstPath := filepath.Join(tempDir, "test.tar.gz")
	if err := CreateTarGz(srcDir, dstPath, "test", false); err != nil {
		t.Fatalf("CreateTarGz failed: %v", err)
	}

	// Verify the deep file is in the archive
	files := readTarGzFiles(t, dstPath)
	found := false
	for _, f := range files {
		if f == "test/a/b/c/d/e/deep.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected deep file in archive, got: %v", files)
	}
}

func TestCreateTarGz_WithMultipleFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory with multiple files and directories
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(filepath.Join(srcDir, "dir1"), 0755); err != nil {
		t.Fatalf("failed to create dir1: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "dir2"), 0755); err != nil {
		t.Fatalf("failed to create dir2: %v", err)
	}

	// Create multiple files
	files := []string{"file1.txt", "dir1/file2.txt", "dir2/file3.txt"}
	for _, f := range files {
		path := filepath.Join(srcDir, f)
		if err := os.WriteFile(path, []byte("content of "+f), 0644); err != nil {
			t.Fatalf("failed to create %s: %v", f, err)
		}
	}

	dstPath := filepath.Join(tempDir, "test.tar.gz")
	if err := CreateTarGz(srcDir, dstPath, "test", false); err != nil {
		t.Fatalf("CreateTarGz failed: %v", err)
	}

	// Verify all files are in the archive
	archiveFiles := readTarGzFiles(t, dstPath)
	expectedFiles := []string{
		"test/dir1/",
		"test/dir1/file2.txt",
		"test/dir2/",
		"test/dir2/file3.txt",
		"test/file1.txt",
	}

	for _, expected := range expectedFiles {
		found := false
		for _, actual := range archiveFiles {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s in archive, got: %v", expected, archiveFiles)
		}
	}
}
