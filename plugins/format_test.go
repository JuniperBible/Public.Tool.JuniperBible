package plugins_test

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/JuniperBible/juniper/core/plugins"
)

func init() {
	// Enable external plugins for testing
	plugins.EnableExternalPlugins()
}

// getProjectRoot returns the project root directory
func getProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filename))
}

// ensurePluginBuilt ensures a plugin is built before testing
// Plugin names are like "format-file" or "tool-libsword"
// They are resolved to "plugins/format-file" or "plugins/tool-libsword" (standalone plugins)
func ensurePluginBuilt(t *testing.T, pluginName string) string {
	t.Helper()
	root := getProjectRoot()

	// Plugin directory is directly plugins/<pluginName> (e.g., plugins/format-file)
	pluginDir := filepath.Join(root, "plugins", pluginName)
	pluginBin := filepath.Join(pluginDir, pluginName)

	// Check if plugin exists, build if not
	if _, err := os.Stat(pluginBin); os.IsNotExist(err) {
		cmd := exec.Command("go", "build", "-o", pluginBin, ".")
		cmd.Dir = pluginDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to build plugin %s: %v\n%s", pluginName, err, out)
		}
	}
	return pluginDir
}

// TestFormatFileDetect tests the format.file plugin detect command.
func TestFormatFileDetect(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	pluginDir := ensurePluginBuilt(t, "format-file")
	plugin := &plugins.Plugin{
		Manifest: &plugins.PluginManifest{
			PluginID:   "format.file",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-file",
		},
		Path: pluginDir,
	}

	// Test detect on file
	req := plugins.NewDetectRequest(testFile)
	resp, err := plugins.ExecutePlugin(plugin, req)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result, err := plugins.ParseDetectResult(resp)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if !result.Detected {
		t.Error("expected file to be detected")
	}
	if result.Format != "file" {
		t.Errorf("expected format 'file', got %q", result.Format)
	}

	// Test detect on directory (should not match)
	req = plugins.NewDetectRequest(tempDir)
	resp, err = plugins.ExecutePlugin(plugin, req)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result, err = plugins.ParseDetectResult(resp)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if result.Detected {
		t.Error("directory should not be detected as file")
	}
}

// TestFormatFileIngest tests the format.file plugin ingest command.
func TestFormatFileIngest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	content := []byte("test content for ingest")
	testFile := filepath.Join(tempDir, "sample.txt")
	if err := os.WriteFile(testFile, content, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	outputDir := filepath.Join(tempDir, "output")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	pluginDir := ensurePluginBuilt(t, "format-file")
	plugin := &plugins.Plugin{
		Manifest: &plugins.PluginManifest{
			PluginID:   "format.file",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-file",
		},
		Path: pluginDir,
	}

	req := plugins.NewIngestRequest(testFile, outputDir)
	resp, err := plugins.ExecutePlugin(plugin, req)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result, err := plugins.ParseIngestResult(resp)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if result.ArtifactID != "sample" {
		t.Errorf("expected artifact ID 'sample', got %q", result.ArtifactID)
	}

	if result.BlobSHA256 == "" {
		t.Error("expected non-empty blob hash")
	}

	if result.SizeBytes != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), result.SizeBytes)
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	data, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("failed to read blob: %v", err)
	}

	if string(data) != string(content) {
		t.Error("blob content mismatch")
	}
}

// TestFormatZipDetect tests the format.zip plugin detect command.
func TestFormatZipDetect(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test zip file
	zipFile := filepath.Join(tempDir, "test.zip")
	zf, err := os.Create(zipFile)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}

	zw := zip.NewWriter(zf)
	w, _ := zw.Create("hello.txt")
	w.Write([]byte("hello world"))
	zw.Close()
	zf.Close()

	pluginDir := ensurePluginBuilt(t, "format-zip")
	plugin := &plugins.Plugin{
		Manifest: &plugins.PluginManifest{
			PluginID:   "format.zip",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-zip",
		},
		Path: pluginDir,
	}

	req := plugins.NewDetectRequest(zipFile)
	resp, err := plugins.ExecutePlugin(plugin, req)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result, err := plugins.ParseDetectResult(resp)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if !result.Detected {
		t.Error("expected zip to be detected")
	}
	if result.Format != "zip" {
		t.Errorf("expected format 'zip', got %q", result.Format)
	}
}

// TestFormatZipEnumerate tests the format.zip plugin enumerate command.
func TestFormatZipEnumerate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test zip with multiple files
	zipFile := filepath.Join(tempDir, "multi.zip")
	zf, err := os.Create(zipFile)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}

	zw := zip.NewWriter(zf)
	files := []string{"a.txt", "b.txt", "subdir/c.txt"}
	for _, name := range files {
		w, _ := zw.Create(name)
		w.Write([]byte("content of " + name))
	}
	zw.Close()
	zf.Close()

	pluginDir := ensurePluginBuilt(t, "format-zip")
	plugin := &plugins.Plugin{
		Manifest: &plugins.PluginManifest{
			PluginID:   "format.zip",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-zip",
		},
		Path: pluginDir,
	}

	req := plugins.NewEnumerateRequest(zipFile)
	resp, err := plugins.ExecutePlugin(plugin, req)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result, err := plugins.ParseEnumerateResult(resp)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(result.Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(result.Entries))
	}
}

// TestFormatDirDetect tests the format.dir plugin detect command.
func TestFormatDirDetect(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginDir := ensurePluginBuilt(t, "format-dir")
	plugin := &plugins.Plugin{
		Manifest: &plugins.PluginManifest{
			PluginID:   "format.dir",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-dir",
		},
		Path: pluginDir,
	}

	// Test detect on directory
	req := plugins.NewDetectRequest(tempDir)
	resp, err := plugins.ExecutePlugin(plugin, req)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result, err := plugins.ParseDetectResult(resp)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if !result.Detected {
		t.Error("expected directory to be detected")
	}
	if result.Format != "dir" {
		t.Errorf("expected format 'dir', got %q", result.Format)
	}

	// Test detect on file (should not match)
	testFile := filepath.Join(tempDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0600)

	req = plugins.NewDetectRequest(testFile)
	resp, err = plugins.ExecutePlugin(plugin, req)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result, err = plugins.ParseDetectResult(resp)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if result.Detected {
		t.Error("file should not be detected as directory")
	}
}

// TestFormatDirEnumerate tests the format.dir plugin enumerate command.
func TestFormatDirEnumerate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create some files and subdirs
	os.WriteFile(filepath.Join(tempDir, "a.txt"), []byte("a"), 0600)
	os.WriteFile(filepath.Join(tempDir, "b.txt"), []byte("b"), 0600)
	os.MkdirAll(filepath.Join(tempDir, "subdir"), 0700)
	os.WriteFile(filepath.Join(tempDir, "subdir", "c.txt"), []byte("c"), 0600)

	pluginDir := ensurePluginBuilt(t, "format-dir")
	plugin := &plugins.Plugin{
		Manifest: &plugins.PluginManifest{
			PluginID:   "format.dir",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-dir",
		},
		Path: pluginDir,
	}

	req := plugins.NewEnumerateRequest(tempDir)
	resp, err := plugins.ExecutePlugin(plugin, req)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result, err := plugins.ParseEnumerateResult(resp)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Should have: a.txt, b.txt, subdir, subdir/c.txt = 4 entries
	if len(result.Entries) != 4 {
		t.Errorf("expected 4 entries, got %d", len(result.Entries))
	}
}
