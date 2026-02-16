//go:build !sdk

package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

func TestDetectDirectory(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create mods.d directory with a conf file
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	confPath := filepath.Join(modsDir, "test.conf")
	confContent := `[TestModule]
Description=Test Module
ModDrv=zText
`
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	// Create modules directory
	modulesDir := filepath.Join(tmpDir, "modules")
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		t.Fatalf("failed to create modules: %v", err)
	}

	// Test detection
	result := detectDirectory(tmpDir)

	if !result.Detected {
		t.Errorf("expected detection to succeed, got: %s", result.Reason)
	}

	if result.Format != "crosswire-directory" {
		t.Errorf("expected format=crosswire-directory, got: %s", result.Format)
	}
}

func TestDetectDirectoryMissingModsD(t *testing.T) {
	tmpDir := t.TempDir()

	// Only create modules directory (missing mods.d)
	modulesDir := filepath.Join(tmpDir, "modules")
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		t.Fatalf("failed to create modules: %v", err)
	}

	result := detectDirectory(tmpDir)

	if result.Detected {
		t.Error("expected detection to fail for directory missing mods.d")
	}
}

func TestDetectDirectoryMissingConf(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mods.d but no .conf files
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	// Create modules directory
	modulesDir := filepath.Join(tmpDir, "modules")
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		t.Fatalf("failed to create modules: %v", err)
	}

	result := detectDirectory(tmpDir)

	if result.Detected {
		t.Error("expected detection to fail for directory with no .conf files")
	}
}

func TestDetectZipArchive(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	// Create a zip with mods.d and modules
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)

	// Add mods.d/test.conf
	confContent := `[TestModule]
Description=Test Module
ModDrv=zText
`
	w, err := zw.Create("mods.d/test.conf")
	if err != nil {
		t.Fatalf("failed to create conf in zip: %v", err)
	}
	if _, err := w.Write([]byte(confContent)); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	// Add modules directory marker
	if _, err := zw.Create("modules/.keep"); err != nil {
		t.Fatalf("failed to create modules marker: %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}

	// Test detection
	result := detectZipArchive(zipPath)

	if !result.Detected {
		t.Errorf("expected detection to succeed, got: %s", result.Reason)
	}

	if result.Format != "crosswire-zip" {
		t.Errorf("expected format=crosswire-zip, got: %s", result.Format)
	}
}

func TestDetectZipArchiveMissingStructure(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	// Create a zip without SWORD structure
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)

	// Add random file
	w, err := zw.Create("random.txt")
	if err != nil {
		t.Fatalf("failed to create file in zip: %v", err)
	}
	if _, err := w.Write([]byte("random content")); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}

	result := detectZipArchive(zipPath)

	if result.Detected {
		t.Error("expected detection to fail for zip without SWORD structure")
	}
}

func TestDetectCrossWire(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) string
		expected bool
	}{
		{
			name: "valid directory",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()

				modsDir := filepath.Join(tmpDir, "mods.d")
				os.MkdirAll(modsDir, 0755)
				os.WriteFile(filepath.Join(modsDir, "test.conf"), []byte("[Test]\n"), 0600)

				modulesDir := filepath.Join(tmpDir, "modules")
				os.MkdirAll(modulesDir, 0755)

				return tmpDir
			},
			expected: true,
		},
		{
			name: "valid zip",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				zipPath := filepath.Join(tmpDir, "test.zip")

				zf, _ := os.Create(zipPath)
				zw := zip.NewWriter(zf)

				w, _ := zw.Create("mods.d/test.conf")
				w.Write([]byte("[Test]\n"))

				zw.Create("modules/.keep")
				zw.Close()
				zf.Close()

				return zipPath
			},
			expected: true,
		},
		{
			name: "invalid directory",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			result := detectCrossWire(path)

			if result.Detected != tt.expected {
				t.Errorf("expected detected=%v, got detected=%v (reason: %s)",
					tt.expected, result.Detected, result.Reason)
			}
		})
	}
}

func TestExtractZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	destDir := filepath.Join(tmpDir, "extracted")

	// Create a test zip
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}

	zw := zip.NewWriter(zf)

	// Add test files
	files := map[string]string{
		"mods.d/test.conf":      "[Test]\nDescription=Test\n",
		"modules/test.txt":      "test content",
		"modules/data/file.dat": "data content",
	}

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create %s in zip: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	zw.Close()
	zf.Close()

	// Extract
	if err := extractZip(zipPath, destDir); err != nil {
		t.Fatalf("failed to extract zip: %v", err)
	}

	// Verify extracted files
	for name, expectedContent := range files {
		extractedPath := filepath.Join(destDir, name)
		content, err := os.ReadFile(extractedPath)
		if err != nil {
			t.Errorf("failed to read extracted file %s: %v", name, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("file %s: expected content %q, got %q", name, expectedContent, string(content))
		}
	}
}

func TestCopyDir(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")

	// Create source directory with files
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0600)
	os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0600)

	// Copy
	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("failed to copy directory: %v", err)
	}

	// Verify copied files
	files := []string{
		"file1.txt",
		"subdir/file2.txt",
	}

	for _, file := range files {
		srcPath := filepath.Join(srcDir, file)
		dstPath := filepath.Join(dstDir, file)

		srcContent, err := os.ReadFile(srcPath)
		if err != nil {
			t.Errorf("failed to read source %s: %v", file, err)
			continue
		}

		dstContent, err := os.ReadFile(dstPath)
		if err != nil {
			t.Errorf("failed to read destination %s: %v", file, err)
			continue
		}

		if string(srcContent) != string(dstContent) {
			t.Errorf("file %s: content mismatch", file)
		}
	}
}

func TestCreateZipArchive(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	zipPath := filepath.Join(tmpDir, "archive.zip")

	// Create source directory with files
	os.MkdirAll(filepath.Join(srcDir, "mods.d"), 0755)
	os.MkdirAll(filepath.Join(srcDir, "modules"), 0755)
	os.WriteFile(filepath.Join(srcDir, "mods.d", "test.conf"), []byte("[Test]\n"), 0600)
	os.WriteFile(filepath.Join(srcDir, "modules", "data.txt"), []byte("data"), 0600)

	// Create zip
	if err := createZipArchive(srcDir, zipPath); err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}

	// Verify zip contents
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer zr.Close()

	expectedFiles := map[string]bool{
		"mods.d/test.conf": false,
		"modules/data.txt": false,
	}

	for _, f := range zr.File {
		if _, ok := expectedFiles[f.Name]; ok {
			expectedFiles[f.Name] = true
		}
	}

	for file, found := range expectedFiles {
		if !found {
			t.Errorf("expected file %s not found in zip", file)
		}
	}
}

func TestHandleDetect(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid SWORD directory
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.MkdirAll(modsDir, 0755)
	os.WriteFile(filepath.Join(modsDir, "test.conf"), []byte("[Test]\n"), 0600)

	modulesDir := filepath.Join(tmpDir, "modules")
	os.MkdirAll(modulesDir, 0755)

	req := &ipc.Request{
		Command: "detect",
		Args: map[string]interface{}{
			"path": tmpDir,
		},
	}

	// Note: This test would need to capture stdout to verify the response
	// For now, we just ensure it doesn't panic
	handleDetect(req)
}

func TestPluginInfo(t *testing.T) {
	info := PluginInfo{
		PluginID:    "format.crosswire",
		Version:     "0.1.0",
		Kind:        "format",
		Description: "CrossWire native SWORD module distribution format (.zip archives with mods.d/ and modules/)",
		Formats:     []string{"crosswire-zip", "sword-distribution"},
	}

	// Verify it can be marshaled to JSON
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal plugin info: %v", err)
	}

	var decoded PluginInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal plugin info: %v", err)
	}

	if decoded.PluginID != "format.crosswire" {
		t.Errorf("expected plugin_id=format.crosswire, got: %s", decoded.PluginID)
	}

	if decoded.Kind != "format" {
		t.Errorf("expected kind=format, got: %s", decoded.Kind)
	}

	if len(decoded.Formats) != 2 {
		t.Errorf("expected 2 formats, got: %d", len(decoded.Formats))
	}
}

func TestZipSlipPrevention(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	// Create a zip with path traversal attempt
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}

	zw := zip.NewWriter(zf)

	// Try to create a file with ../ in the path
	_, err = zw.Create("../../../etc/passwd")
	if err != nil {
		t.Fatalf("failed to create file in zip: %v", err)
	}

	zw.Close()
	zf.Close()

	// Try to extract - should be safe
	destDir := filepath.Join(tmpDir, "extracted")
	if err := extractZip(zipPath, destDir); err != nil {
		// This should fail due to path validation
		// If it succeeds, verify the file is not outside destDir
		extractedPath := filepath.Join(destDir, "../../../etc/passwd")
		if _, err := os.Stat(extractedPath); err == nil {
			t.Error("zip slip vulnerability: file extracted outside destination directory")
		}
	}
}
