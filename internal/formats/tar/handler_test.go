package tar

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ulikunitz/xz"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.tar" {
		t.Errorf("Expected PluginID 'format.tar', got %s", m.PluginID)
	}
	if m.Kind != "format" {
		t.Errorf("Expected Kind 'format', got %s", m.Kind)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", m.Version)
	}
}

func TestRegister(t *testing.T) {
	// Register should not panic
	Register()
}

func createTarFile(t *testing.T, dir, filename string) string {
	t.Helper()
	tarPath := filepath.Join(dir, filename)
	f, err := os.Create(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	content := []byte("test content")
	hdr := &tar.Header{
		Name: "test.txt",
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	return tarPath
}

func createTarGzFile(t *testing.T, dir, filename string) string {
	t.Helper()
	tarGzPath := filepath.Join(dir, filename)
	f, err := os.Create(tarGzPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gzw := gzip.NewWriter(f)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	content := []byte("gzipped content")
	hdr := &tar.Header{
		Name: "gzipped.txt",
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	return tarGzPath
}

func createTarXzFile(t *testing.T, dir, filename string) string {
	t.Helper()
	tarXzPath := filepath.Join(dir, filename)
	f, err := os.Create(tarXzPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	xzw, err := xz.NewWriter(f)
	if err != nil {
		t.Fatal(err)
	}
	defer xzw.Close()

	tw := tar.NewWriter(xzw)
	defer tw.Close()

	content := []byte("xz content")
	hdr := &tar.Header{
		Name: "xz.txt",
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	return tarXzPath
}

func TestDetect_TarFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	tarPath := createTarFile(t, tmpDir, "test.tar")
	result, err := h.Detect(tarPath)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected tar file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "tar" {
		t.Errorf("Expected format 'tar', got %s", result.Format)
	}
}

func TestDetect_TarGzFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	tarGzPath := createTarGzFile(t, tmpDir, "test.tar.gz")
	result, err := h.Detect(tarGzPath)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected tar.gz file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "tar.gz" {
		t.Errorf("Expected format 'tar.gz', got %s", result.Format)
	}
}

func TestDetect_TgzFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	tgzPath := createTarGzFile(t, tmpDir, "test.tgz")
	result, err := h.Detect(tgzPath)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected .tgz file to be detected, reason: %s", result.Reason)
	}
}

func TestDetect_TarXzFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	tarXzPath := createTarXzFile(t, tmpDir, "test.tar.xz")
	result, err := h.Detect(tarXzPath)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected tar.xz file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "tar.xz" {
		t.Errorf("Expected format 'tar.xz', got %s", result.Format)
	}
}

func TestDetect_TxzFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	txzPath := createTarXzFile(t, tmpDir, "test.txz")
	result, err := h.Detect(txzPath)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected .txz file to be detected, reason: %s", result.Reason)
	}
}

func TestDetect_NonTarFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	txtFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtFile, []byte("not a tar"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(txtFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-tar file to not be detected")
	}
}

func TestDetect_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	result, err := h.Detect(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected directory to not be detected")
	}
	if !strings.Contains(result.Reason, "directory") {
		t.Errorf("Expected reason to mention directory, got: %s", result.Reason)
	}
}

func TestDetect_NonExistentFile(t *testing.T) {
	h := &Handler{}

	result, err := h.Detect("/nonexistent/file.tar")
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-existent file to not be detected")
	}
	if !strings.Contains(result.Reason, "cannot stat") {
		t.Errorf("Expected reason to mention stat error, got: %s", result.Reason)
	}
}

func TestDetect_InvalidTarContent(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	// Create a file with .tar extension but invalid content
	invalidTar := filepath.Join(tmpDir, "invalid.tar")
	if err := os.WriteFile(invalidTar, []byte("not a valid tar"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(invalidTar)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected invalid tar content to not be detected")
	}
	if !strings.Contains(result.Reason, "not a valid tar") {
		t.Errorf("Expected reason to mention invalid tar, got: %s", result.Reason)
	}
}

func TestDetect_InvalidGzipContent(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	// Create a file with .tar.gz extension but invalid gzip content
	invalidGz := filepath.Join(tmpDir, "invalid.tar.gz")
	if err := os.WriteFile(invalidGz, []byte("not gzip"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(invalidGz)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected invalid gzip to not be detected")
	}
	if !strings.Contains(result.Reason, "not valid gzip") {
		t.Errorf("Expected reason to mention invalid gzip, got: %s", result.Reason)
	}
}

func TestDetect_InvalidXzContent(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	// Create a file with .tar.xz extension but invalid xz content
	invalidXz := filepath.Join(tmpDir, "invalid.tar.xz")
	if err := os.WriteFile(invalidXz, []byte("not xz"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(invalidXz)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected invalid xz to not be detected")
	}
	if !strings.Contains(result.Reason, "not valid xz") {
		t.Errorf("Expected reason to mention invalid xz, got: %s", result.Reason)
	}
}

func TestIngest_TarFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	tarPath := createTarFile(t, tmpDir, "test.tar")
	outputDir := filepath.Join(tmpDir, "output")

	result, err := h.Ingest(tarPath, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "test" {
		t.Errorf("Expected artifact ID 'test', got %s", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected blob hash to be set")
	}
	if result.Metadata["format"] != "tar" {
		t.Errorf("Expected format 'tar', got %s", result.Metadata["format"])
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

func TestIngest_TarGzFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	tarGzPath := createTarGzFile(t, tmpDir, "archive.tar.gz")
	outputDir := filepath.Join(tmpDir, "output")

	result, err := h.Ingest(tarGzPath, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "archive" {
		t.Errorf("Expected artifact ID 'archive', got %s", result.ArtifactID)
	}
	if result.Metadata["format"] != "tar.gz" {
		t.Errorf("Expected format 'tar.gz', got %s", result.Metadata["format"])
	}
}

func TestIngest_TarXzFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	tarXzPath := createTarXzFile(t, tmpDir, "archive.tar.xz")
	outputDir := filepath.Join(tmpDir, "output")

	result, err := h.Ingest(tarXzPath, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "archive" {
		t.Errorf("Expected artifact ID 'archive', got %s", result.ArtifactID)
	}
	if result.Metadata["format"] != "tar.xz" {
		t.Errorf("Expected format 'tar.xz', got %s", result.Metadata["format"])
	}
}

func TestIngest_NonExistentFile(t *testing.T) {
	h := &Handler{}
	outputDir := t.TempDir()

	_, err := h.Ingest("/nonexistent/file.tar", outputDir)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected 'failed to read file' error, got: %v", err)
	}
}

func TestEnumerate_TarFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	tarPath := createTarFile(t, tmpDir, "test.tar")

	result, err := h.Enumerate(tarPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.txt" {
		t.Errorf("Expected path 'test.txt', got %s", entry.Path)
	}
	if entry.IsDir {
		t.Error("Expected entry to not be a directory")
	}
}

func TestEnumerate_TarGzFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	tarGzPath := createTarGzFile(t, tmpDir, "test.tar.gz")

	result, err := h.Enumerate(tarGzPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	if result.Entries[0].Path != "gzipped.txt" {
		t.Errorf("Expected path 'gzipped.txt', got %s", result.Entries[0].Path)
	}
}

func TestEnumerate_TarXzFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	tarXzPath := createTarXzFile(t, tmpDir, "test.tar.xz")

	result, err := h.Enumerate(tarXzPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	if result.Entries[0].Path != "xz.txt" {
		t.Errorf("Expected path 'xz.txt', got %s", result.Entries[0].Path)
	}
}

func TestEnumerate_NonExistentFile(t *testing.T) {
	h := &Handler{}

	_, err := h.Enumerate("/nonexistent/file.tar")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to open file") {
		t.Errorf("Expected 'failed to open file' error, got: %v", err)
	}
}

func TestEnumerate_InvalidGzip(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	invalidGz := filepath.Join(tmpDir, "invalid.tar.gz")
	if err := os.WriteFile(invalidGz, []byte("not gzip"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := h.Enumerate(invalidGz)
	if err == nil {
		t.Error("Expected error for invalid gzip")
	}
	if !strings.Contains(err.Error(), "failed to create gzip reader") {
		t.Errorf("Expected gzip reader error, got: %v", err)
	}
}

func TestEnumerate_InvalidXz(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	invalidXz := filepath.Join(tmpDir, "invalid.tar.xz")
	if err := os.WriteFile(invalidXz, []byte("not xz"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := h.Enumerate(invalidXz)
	if err == nil {
		t.Error("Expected error for invalid xz")
	}
	if !strings.Contains(err.Error(), "failed to create xz reader") {
		t.Errorf("Expected xz reader error, got: %v", err)
	}
}

func TestExtractIR(t *testing.T) {
	h := &Handler{}

	_, err := h.ExtractIR("test.tar", "/output")
	if err == nil {
		t.Error("Expected error for unsupported ExtractIR")
	}
	if !strings.Contains(err.Error(), "does not support IR extraction") {
		t.Errorf("Expected IR extraction error, got: %v", err)
	}
}

func TestEmitNative(t *testing.T) {
	h := &Handler{}

	_, err := h.EmitNative("ir.json", "/output")
	if err == nil {
		t.Error("Expected error for unsupported EmitNative")
	}
	if !strings.Contains(err.Error(), "does not support native emission") {
		t.Errorf("Expected native emission error, got: %v", err)
	}
}

func TestIngest_TgzFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	tgzPath := createTarGzFile(t, tmpDir, "archive.tgz")
	outputDir := filepath.Join(tmpDir, "output")

	result, err := h.Ingest(tgzPath, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Metadata["format"] != "tar.gz" {
		t.Errorf("Expected format 'tar.gz' for .tgz, got %s", result.Metadata["format"])
	}
}

func TestIngest_TxzFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	txzPath := createTarXzFile(t, tmpDir, "archive.txz")
	outputDir := filepath.Join(tmpDir, "output")

	result, err := h.Ingest(txzPath, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Metadata["format"] != "tar.xz" {
		t.Errorf("Expected format 'tar.xz' for .txz, got %s", result.Metadata["format"])
	}
}

func TestEnumerate_Tgz(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	tgzPath := createTarGzFile(t, tmpDir, "test.tgz")

	result, err := h.Enumerate(tgzPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) == 0 {
		t.Error("Expected at least one entry")
	}
}

func TestEnumerate_Txz(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	txzPath := createTarXzFile(t, tmpDir, "test.txz")

	result, err := h.Enumerate(txzPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) == 0 {
		t.Error("Expected at least one entry")
	}
}
