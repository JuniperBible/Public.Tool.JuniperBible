package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewClient tests creating a new client.
func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.httpClient == nil {
		t.Error("httpClient is nil")
	}
	if client.userAgent == "" {
		t.Error("userAgent is empty")
	}
}

// TestDownload tests downloading content from a URL.
func TestDownload(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test content"))
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	data, err := client.Download(ctx, server.URL)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if string(data) != "test content" {
		t.Errorf("expected 'test content', got %q", string(data))
	}
}

// TestDownloadEmptyURL tests downloading with empty URL.
func TestDownloadEmptyURL(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	_, err := client.Download(ctx, "")
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

// TestDownloadUnsupportedScheme tests downloading with unsupported scheme.
func TestDownloadUnsupportedScheme(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	_, err := client.Download(ctx, "ftp://example.com/file")
	if err == nil {
		t.Error("expected error for FTP URL")
	}
}

// TestDownload404 tests handling 404 responses.
func TestDownload404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	_, err := client.Download(ctx, server.URL)
	if err == nil {
		t.Error("expected error for 404")
	}

	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if !httpErr.IsNotFound() {
		t.Errorf("expected 404, got %d", httpErr.StatusCode)
	}
}

// TestListDirectory tests parsing directory listings.
func TestListDirectory(t *testing.T) {
	html := `<html><body>
		<a href="file1.zip">file1.zip</a>
		<a href="file2.tar.gz">file2.tar.gz</a>
		<a href="../">Parent</a>
		<a href="/absolute">Absolute</a>
	</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(html))
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	files, err := client.ListDirectory(ctx, server.URL)
	if err != nil {
		t.Fatalf("ListDirectory failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	// Should contain file1.zip and file2.tar.gz
	hasFile1 := false
	hasFile2 := false
	for _, f := range files {
		if f == "file1.zip" {
			hasFile1 = true
		}
		if f == "file2.tar.gz" {
			hasFile2 = true
		}
	}
	if !hasFile1 || !hasFile2 {
		t.Errorf("missing expected files in %v", files)
	}
}

// TestSourceConfig tests source URL generation.
func TestSourceConfig(t *testing.T) {
	source := SourceConfig{
		Name:      "CrossWire",
		URL:       "https://www.crosswire.org/ftpmirror",
		Directory: "/pub/sword/raw",
	}

	modsURL := source.ModsIndexURL()
	expected := "https://www.crosswire.org/ftpmirror/pub/sword/raw/mods.d.tar.gz"
	if modsURL != expected {
		t.Errorf("ModsIndexURL() = %q, want %q", modsURL, expected)
	}

	packageURLs := source.ModulePackageURLs("KJV")
	if len(packageURLs) == 0 {
		t.Error("ModulePackageURLs returned empty")
	}
	// Should include packages/rawzip path for CrossWire-style
	found := false
	for _, url := range packageURLs {
		if strings.Contains(url, "packages/rawzip/KJV.zip") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected packages/rawzip URL, got %v", packageURLs)
	}
}

// TestDefaultSources tests default source configuration.
func TestDefaultSources(t *testing.T) {
	sources := DefaultSources()
	if len(sources) < 2 {
		t.Errorf("expected at least 2 sources, got %d", len(sources))
	}

	// Check CrossWire source exists
	found := false
	for _, s := range sources {
		if s.Name == "CrossWire" {
			found = true
			break
		}
	}
	if !found {
		t.Error("CrossWire source not found")
	}
}

// TestGetSource tests source lookup.
func TestGetSource(t *testing.T) {
	source, ok := GetSource("CrossWire")
	if !ok {
		t.Fatal("CrossWire source not found")
	}
	if source.Name != "CrossWire" {
		t.Errorf("expected name 'CrossWire', got %q", source.Name)
	}

	_, ok = GetSource("NonExistent")
	if ok {
		t.Error("expected not found for NonExistent source")
	}
}

// TestParseModuleConf tests parsing module conf files.
func TestParseModuleConf(t *testing.T) {
	conf := `[KJV]
Description=King James Version (1769)
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
Lang=en
Version=2.3
`

	module, err := ParseModuleConf([]byte(conf))
	if err != nil {
		t.Fatalf("ParseModuleConf failed: %v", err)
	}

	if module.Name != "KJV" {
		t.Errorf("expected name 'KJV', got %q", module.Name)
	}
	if module.Description != "King James Version (1769)" {
		t.Errorf("expected description 'King James Version (1769)', got %q", module.Description)
	}
	if module.Language != "en" {
		t.Errorf("expected language 'en', got %q", module.Language)
	}
	if module.Version != "2.3" {
		t.Errorf("expected version '2.3', got %q", module.Version)
	}
	if module.Type != "Bible" {
		t.Errorf("expected type 'Bible', got %q", module.Type)
	}
}

// TestParseModuleConfEmpty tests parsing empty conf.
func TestParseModuleConfEmpty(t *testing.T) {
	_, err := ParseModuleConf([]byte{})
	if err == nil {
		t.Error("expected error for empty conf")
	}
}

// TestParseModuleConfNoHeader tests parsing conf without section header.
func TestParseModuleConfNoHeader(t *testing.T) {
	conf := `Description=Test
Version=1.0`

	_, err := ParseModuleConf([]byte(conf))
	if err == nil {
		t.Error("expected error for conf without section header")
	}
}

// TestParseModsArchive tests parsing mods.d.tar.gz archives.
func TestParseModsArchive(t *testing.T) {
	// Create a test tar.gz archive
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Add a conf file
	confContent := `[TestMod]
Description=Test Module
ModDrv=zText
Lang=en
Version=1.0
`
	hdr := &tar.Header{
		Name: "mods.d/testmod.conf",
		Mode: 0600,
		Size: int64(len(confContent)),
	}
	tw.WriteHeader(hdr)
	tw.Write([]byte(confContent))

	tw.Close()
	gw.Close()

	modules, err := ParseModsArchive(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseModsArchive failed: %v", err)
	}

	if len(modules) != 1 {
		t.Errorf("expected 1 module, got %d", len(modules))
	}
	if modules[0].Name != "TestMod" {
		t.Errorf("expected name 'TestMod', got %q", modules[0].Name)
	}
}

// TestExtractZipArchive tests extracting zip archives.
func TestExtractZipArchive(t *testing.T) {
	// Create a test zip archive
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Add a file
	fw, _ := zw.Create("test/file.txt")
	fw.Write([]byte("test content"))

	// Add a directory
	zw.Create("test/subdir/")

	zw.Close()

	// Extract to temp dir
	tmpDir := t.TempDir()
	err := ExtractZipArchive(buf.Bytes(), tmpDir)
	if err != nil {
		t.Fatalf("ExtractZipArchive failed: %v", err)
	}

	// Verify file was extracted
	content, err := os.ReadFile(filepath.Join(tmpDir, "test", "file.txt"))
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("expected 'test content', got %q", string(content))
	}
}

// TestExtractZipArchivePathTraversal tests protection against path traversal.
func TestExtractZipArchivePathTraversal(t *testing.T) {
	// Create a zip with path traversal attempt
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Try to add a file with path traversal
	fw, _ := zw.Create("../../../etc/passwd")
	fw.Write([]byte("evil content"))

	zw.Close()

	tmpDir := t.TempDir()
	err := ExtractZipArchive(buf.Bytes(), tmpDir)
	if err == nil {
		t.Error("expected error for path traversal attempt")
	}
}

// TestModuleTypeFromDriver tests driver type detection.
func TestModuleTypeFromDriver(t *testing.T) {
	tests := []struct {
		driver   string
		expected string
	}{
		{"zText", "Bible"},
		{"RawText", "Bible"},
		{"zCom", "Commentary"},
		{"RawCom", "Commentary"},
		{"zLD", "Dictionary"},
		{"RawLD", "Dictionary"},
		{"RawGenBook", "GenBook"},
		{"Unknown", "Unknown"},
	}

	for _, tt := range tests {
		got := moduleTypeFromDriver(tt.driver)
		if got != tt.expected {
			t.Errorf("moduleTypeFromDriver(%q) = %q, want %q", tt.driver, got, tt.expected)
		}
	}
}

// TestDownloadToFile tests downloading to a file.
func TestDownloadToFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("file content"))
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "subdir", "test.txt")

	err := client.DownloadToFile(ctx, server.URL, destPath)
	if err != nil {
		t.Fatalf("DownloadToFile failed: %v", err)
	}

	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(content) != "file content" {
		t.Errorf("expected 'file content', got %q", string(content))
	}
}

// TestListInstalled tests listing installed modules.
func TestListInstalled(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mods.d directory with a conf file
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		t.Fatal(err)
	}

	confContent := `[TestMod]
Description=Test Module
ModDrv=zText
Lang=en
Version=1.0
DataPath=./modules/texts/ztext/testmod/
`
	confPath := filepath.Join(modsDir, "testmod.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		t.Fatal(err)
	}

	modules, err := ListInstalled(tmpDir)
	if err != nil {
		t.Fatalf("ListInstalled failed: %v", err)
	}

	if len(modules) != 1 {
		t.Errorf("expected 1 module, got %d", len(modules))
	}
	if modules[0].Name != "TestMod" {
		t.Errorf("expected name 'TestMod', got %q", modules[0].Name)
	}
}

// TestListInstalledEmpty tests listing installed modules when none exist.
func TestListInstalledEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	modules, err := ListInstalled(tmpDir)
	if err != nil {
		t.Fatalf("ListInstalled failed: %v", err)
	}

	if len(modules) != 0 {
		t.Errorf("expected 0 modules, got %d", len(modules))
	}
}

// TestVerifyValid tests verifying a valid installed module.
func TestVerifyValid(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mods.d directory with conf
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Create data directory with a file
	dataDir := filepath.Join(tmpDir, "modules", "texts", "ztext", "testmod")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "ot.bzs"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	confContent := `[TestMod]
Description=Test Module
ModDrv=zText
DataPath=./modules/texts/ztext/testmod/
`
	confPath := filepath.Join(modsDir, "testmod.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := Verify("TestMod", tmpDir)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}
}

// TestVerifyMissingData tests verifying a module with missing data.
func TestVerifyMissingData(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mods.d directory with conf
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		t.Fatal(err)
	}

	confContent := `[TestMod]
Description=Test Module
ModDrv=zText
DataPath=./modules/texts/ztext/testmod/
`
	confPath := filepath.Join(modsDir, "testmod.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := Verify("TestMod", tmpDir)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid due to missing data")
	}
	if len(result.Errors) == 0 {
		t.Error("expected errors for missing data")
	}
}

// TestUninstallModule tests uninstalling a module.
func TestUninstallModule(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mods.d directory with conf
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Create data directory
	dataDir := filepath.Join(tmpDir, "modules", "texts", "ztext", "testmod")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "ot.bzs"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	confContent := `[TestMod]
Description=Test Module
ModDrv=zText
DataPath=./modules/texts/ztext/testmod/
`
	confPath := filepath.Join(modsDir, "testmod.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Uninstall
	err := Uninstall("TestMod", tmpDir)
	if err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}

	// Verify conf file is removed
	if _, err := os.Stat(confPath); !os.IsNotExist(err) {
		t.Error("conf file should be removed")
	}
}

// TestParseModuleConfWithDataPath tests parsing conf with DataPath.
func TestParseModuleConfWithDataPath(t *testing.T) {
	conf := `[KJV]
Description=King James Version
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
`

	module, err := ParseModuleConf([]byte(conf))
	if err != nil {
		t.Fatalf("ParseModuleConf failed: %v", err)
	}

	if module.DataPath != "./modules/texts/ztext/kjv/" {
		t.Errorf("expected DataPath './modules/texts/ztext/kjv/', got %q", module.DataPath)
	}
}
