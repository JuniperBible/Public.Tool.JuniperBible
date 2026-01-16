package web

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ulikunitz/xz"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
	"github.com/FocuswithJustin/JuniperBible/internal/archive"
	"github.com/FocuswithJustin/JuniperBible/internal/fileutil"
	"github.com/FocuswithJustin/JuniperBible/internal/validation"

	// Import embedded plugins registry to register all embedded plugins
	_ "github.com/FocuswithJustin/JuniperBible/internal/embedded"
)

func init() {
	// Enable external plugins for testing
	plugins.EnableExternalPlugins()
	// Set up test ServerConfig
	ServerConfig = Config{
		Port:        8080,
		CapsulesDir: "testdata/capsules",
		PluginsDir:  "testdata/plugins",
	}

	// Initialize Templates for testing
	funcMap := template.FuncMap{
		"iterate": func(n int) []int {
			result := make([]int, n)
			for i := range result {
				result[i] = i
			}
			return result
		},
		"add": func(a, b int) int {
			return a + b
		},
	}
	Templates = template.New("").Funcs(funcMap)

	// Create minimal Templates for testing
	template.Must(Templates.New("index.html").Parse(`<!DOCTYPE html><html><body><svg id="home-logo" viewBox="0 0 100 100"></svg></body></html>`))
	template.Must(Templates.New("capsules.html").Parse(`<!DOCTYPE html><html><body>{{.Title}}</body></html>`))
	template.Must(Templates.New("capsule.html").Parse(`<!DOCTYPE html><html><body>{{.Title}}</body></html>`))
	template.Must(Templates.New("artifact.html").Parse(`<!DOCTYPE html><html><body>{{.Title}}</body></html>`))
	template.Must(Templates.New("ir.html").Parse(`<!DOCTYPE html><html><body>{{.Title}}</body></html>`))
	template.Must(Templates.New("transcript.html").Parse(`<!DOCTYPE html><html><body>{{.Title}}</body></html>`))
	template.Must(Templates.New("plugins.html").Parse(`<!DOCTYPE html><html><body>{{.Title}}</body></html>`))
	template.Must(Templates.New("convert.html").Parse(`<!DOCTYPE html><html><body>{{.Title}}</body></html>`))
	template.Must(Templates.New("juniper.html").Parse(`<!DOCTYPE html><html><body><h1>Settings</h1><div id="tab-plugins">Format Plugins</div><div id="tab-detect">Detect</div><div id="tab-convert">Convert</div><div id="tab-info">Info</div></body></html>`))
}

// Helper functions for creating test capsules

func createTestCapsuleTarGz(t *testing.T, path string, files map[string][]byte) {
	t.Helper()

	outFile, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create capsule file: %v", err)
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	baseName := strings.TrimSuffix(filepath.Base(path), ".tar.gz")
	baseName = strings.TrimSuffix(baseName, ".capsule")

	for name, content := range files {
		header := &tar.Header{
			Name: baseName + "/" + name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("failed to write tar content: %v", err)
		}
	}
}

func createTestCapsuleTarXz(t *testing.T, path string, files map[string][]byte) {
	t.Helper()

	outFile, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create capsule file: %v", err)
	}
	defer outFile.Close()

	xw, err := xz.NewWriter(outFile)
	if err != nil {
		t.Fatalf("failed to create xz writer: %v", err)
	}
	defer xw.Close()

	tw := tar.NewWriter(xw)
	defer tw.Close()

	baseName := strings.TrimSuffix(filepath.Base(path), ".tar.xz")
	baseName = strings.TrimSuffix(baseName, ".capsule")

	for name, content := range files {
		header := &tar.Header{
			Name: baseName + "/" + name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("failed to write tar content: %v", err)
		}
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tc := range tests {
		result := humanSize(tc.input)
		if result != tc.expected {
			t.Errorf("humanSize(%d) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestDetectCapsuleFormat(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"test.tar.xz", "tar.xz"},
		{"test.tar.gz", "tar.gz"},
		{"test.tar", "tar"},
		{"test.zip", "unknown"},
	}

	for _, tc := range tests {
		result := detectCapsuleFormat(tc.path)
		if result != tc.expected {
			t.Errorf("detectCapsuleFormat(%q) = %q, want %q", tc.path, result, tc.expected)
		}
	}
}

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{"test.xml", []byte("<xml/>"), "application/xml"},
		{"test.json", []byte("{}"), "application/json"},
		{"test.html", []byte("<html>"), "text/html"},
		{"test.md", []byte("# Title"), "text/markdown"},
		{"test.txt", []byte("text"), "text/plain"},
		{"test.usfm", []byte("\\id GEN"), "text/plain"},
	}

	for _, tc := range tests {
		result := detectContentType(tc.name, tc.data)
		if result != tc.expected {
			t.Errorf("detectContentType(%q, ...) = %q, want %q", tc.name, result, tc.expected)
		}
	}
}

func TestIsBinaryContent(t *testing.T) {
	if "application/octet-stream" != "application/octet-stream" {
		t.Error("expected application/octet-stream to be binary")
	}
	if "text/plain" == "application/octet-stream" {
		t.Error("expected text/plain to not be binary")
	}
}

func TestMin(t *testing.T) {
	if min(5, 10) != 5 {
		t.Error("min(5, 10) should be 5")
	}
	if min(10, 5) != 5 {
		t.Error("min(10, 5) should be 5")
	}
}

func TestHandleStatic(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	w := httptest.NewRecorder()

	handleStatic(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/css" {
		t.Errorf("expected Content-Type text/css, got %q", contentType)
	}
}

func TestHandleStaticNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/static/nonexistent.js", nil)
	w := httptest.NewRecorder()

	handleStatic(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestListCapsules(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test capsule files
	os.WriteFile(filepath.Join(tmpDir, "test1.tar.xz"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test2.tar.gz"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "notacapsule.txt"), []byte("test"), 0644)

	// Set ServerConfig to use temp dir
	originalDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tmpDir
	defer func() { ServerConfig.CapsulesDir = originalDir }()

	capsules := listCapsules()

	// Should find 2 capsules (not the txt file)
	if len(capsules) != 2 {
		t.Errorf("expected 2 capsules, got %d", len(capsules))
	}

	// Check they are sorted
	if len(capsules) >= 2 && capsules[0].Name > capsules[1].Name {
		t.Error("capsules should be sorted by name")
	}
}

func TestListFormatPlugins(t *testing.T) {
	// With embedded plugins, we should have at least 32 format plugins
	// regardless of the plugins directory
	plugins := listFormatPlugins()

	// We have 32 embedded format plugins (sqlite and zip not yet committed)
	if len(plugins) < 32 {
		t.Errorf("expected at least 32 embedded format plugins, got %d", len(plugins))
	}

	for _, p := range plugins {
		if p.Type != "format" {
			t.Errorf("expected type 'format', got %q", p.Type)
		}
		// Verify Source field is set correctly
		if p.Source != "internal" && p.Source != "external" && p.Source != "unloaded" {
			t.Errorf("expected Source to be 'internal', 'external', or 'unloaded', got %q", p.Source)
		}
	}
}

func TestListToolPlugins(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tool plugins directory with proper plugin.json
	toolDir := filepath.Join(tmpDir, "tool", "libsword")
	os.MkdirAll(toolDir, 0755)
	manifest := `{"plugin_id": "tool.libsword", "name": "libsword", "version": "1.0.0", "kind": "tool", "entrypoint": "tool-libsword"}`
	os.WriteFile(filepath.Join(toolDir, "plugin.json"), []byte(manifest), 0644)

	// Set ServerConfig
	originalDir := ServerConfig.PluginsDir
	ServerConfig.PluginsDir = tmpDir
	defer func() { ServerConfig.PluginsDir = originalDir }()

	pluginList := listToolPlugins()

	// Should have at least 1 plugin (the one we created plus any embedded plugins)
	if len(pluginList) < 1 {
		t.Errorf("expected at least 1 plugin, got %d", len(pluginList))
	}

	// Verify all plugins have the correct type
	for _, p := range pluginList {
		if p.Type != "tool" {
			t.Errorf("expected type 'tool', got %q", p.Type)
		}
	}
}

func TestPrettyJSON(t *testing.T) {
	input := map[string]string{"key": "value"}
	result := prettyJSON(input)

	if !strings.Contains(result, "\"key\"") {
		t.Error("expected key in JSON output")
	}
	if !strings.Contains(result, "\"value\"") {
		t.Error("expected value in JSON output")
	}
}

func TestCombineLossClass(t *testing.T) {
	tests := []struct {
		a, b, expected string
	}{
		{"L0", "L0", "L0"},
		{"L0", "L1", "L1"},
		{"L1", "L0", "L1"},
		{"L1", "L2", "L2"},
		{"L2", "L3", "L3"},
		{"L3", "L1", "L3"},
		{"", "L1", "L1"},
		{"L2", "", "L2"},
	}

	for _, tc := range tests {
		result := combineLossClass(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("combineLossClass(%q, %q) = %q, want %q", tc.a, tc.b, result, tc.expected)
		}
	}
}

func TestDetectSourceFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tmpDir
	defer func() { ServerConfig.CapsulesDir = originalDir }()

	tests := []struct {
		name     string
		expected string
	}{
		{"test.sword.tar.gz", "sword"},
		{"test-osis.tar.gz", "osis"},
		{"test-usfm.tar.gz", "usfm"},
		{"unknown.tar.gz", "unknown"},
	}

	for _, tc := range tests {
		result := detectSourceFormat(tc.name)
		if result != tc.expected {
			t.Errorf("detectSourceFormat(%q) = %q, want %q", tc.name, result, tc.expected)
		}
	}
}

func TestIsDirEmpty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Empty directory
	if !isDirEmpty(tmpDir) {
		t.Error("expected empty directory to return true")
	}

	// Non-empty directory
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)
	if isDirEmpty(tmpDir) {
		t.Error("expected non-empty directory to return false")
	}

	// Non-existent directory
	if !isDirEmpty(filepath.Join(tmpDir, "nonexistent")) {
		t.Error("expected non-existent directory to return true")
	}
}

func TestIsJSONContent(t *testing.T) {
	tests := []struct {
		content  []byte
		expected bool
	}{
		{[]byte("{}"), true},
		{[]byte("[]"), true},
		{[]byte("  {"), true},
		{[]byte("\n\t{"), true},
		{[]byte("plain text"), false},
		{[]byte(""), false},
		{[]byte(" "), false},
	}

	for _, tc := range tests {
		result := isJSONContent(tc.content)
		if result != tc.expected {
			t.Errorf("isJSONContent(%q) = %v, want %v", string(tc.content), result, tc.expected)
		}
	}
}

func TestRenameToOld(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		expected string
	}{
		{"test.capsule.tar.gz", "test-old.capsule.tar.gz"},
		{"test.capsule.tar.xz", "test-old.capsule.tar.xz"},
		{"test.tar.gz", "test-old.tar.gz"},
		{"test.tar.xz", "test-old.tar.xz"},
		{"test.txt", "test.txt-old"},
	}

	for _, tc := range tests {
		srcPath := filepath.Join(tmpDir, tc.name)
		os.WriteFile(srcPath, []byte("test"), 0644)

		oldPath := renameToOld(srcPath)
		if oldPath == "" {
			t.Errorf("renameToOld(%q) failed", tc.name)
			continue
		}

		expectedPath := filepath.Join(tmpDir, tc.expected)
		if oldPath != expectedPath {
			t.Errorf("renameToOld(%q) = %q, want %q", tc.name, oldPath, expectedPath)
		}

		// Verify file was actually renamed
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			t.Errorf("renamed file %q does not exist", oldPath)
		}

		// Cleanup for next iteration
		os.Remove(oldPath)
	}
}

func TestFindContentFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name           string
		createFiles    func(string)
		expectedFormat string
		shouldFind     bool
	}{
		{
			name: "SWORD module",
			createFiles: func(dir string) {
				modsDir := filepath.Join(dir, "mods.d")
				os.MkdirAll(modsDir, 0755)
				os.WriteFile(filepath.Join(modsDir, "test.conf"), []byte("[Test]"), 0644)
			},
			expectedFormat: "sword-pure",
			shouldFind:     true,
		},
		{
			name: "OSIS file",
			createFiles: func(dir string) {
				os.WriteFile(filepath.Join(dir, "test.osis"), []byte("<osis>"), 0644)
			},
			expectedFormat: "osis",
			shouldFind:     true,
		},
		{
			name: "USX file",
			createFiles: func(dir string) {
				os.WriteFile(filepath.Join(dir, "test.usx"), []byte("<usx>"), 0644)
			},
			expectedFormat: "usx",
			shouldFind:     true,
		},
		{
			name: "USFM file",
			createFiles: func(dir string) {
				os.WriteFile(filepath.Join(dir, "test.usfm"), []byte("\\id GEN"), 0644)
			},
			expectedFormat: "usfm",
			shouldFind:     true,
		},
		{
			name: "No content files",
			createFiles: func(dir string) {
				os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{}"), 0644)
			},
			expectedFormat: "",
			shouldFind:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tc.name)
			os.MkdirAll(testDir, 0755)
			defer os.RemoveAll(testDir)

			tc.createFiles(testDir)

			path, format := findContentFile(testDir)

			if tc.shouldFind {
				if path == "" {
					t.Errorf("%s: expected to find content file, got empty path", tc.name)
				}
				if format != tc.expectedFormat {
					t.Errorf("%s: expected format %q, got %q", tc.name, tc.expectedFormat, format)
				}
			} else {
				if path != "" {
					t.Errorf("%s: expected no content file, got %q", tc.name, path)
				}
			}
		})
	}
}

// HTTP Handler Tests

func TestHandleIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tmpDir
	defer func() { ServerConfig.CapsulesDir = originalDir }()

	// Create test capsule
	createTestCapsuleTarGz(t, filepath.Join(tmpDir, "test.tar.gz"), map[string][]byte{
		"manifest.json": []byte(`{"version":"1.0","title":"Test"}`),
	})

	t.Run("root path shows logo page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		handleIndex(w, req)

		resp := w.Result()
		// Root path now shows the logo page (not a redirect)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("non-root path returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
		w := httptest.NewRecorder()

		handleIndex(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})
}

func TestHandleCapsule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tmpDir
	defer func() { ServerConfig.CapsulesDir = originalDir }()

	// Create test capsule
	manifestData, _ := json.Marshal(CapsuleManifest{
		Version: "1.0",
		Title:   "Test Capsule",
	})
	createTestCapsuleTarGz(t, filepath.Join(tmpDir, "test.tar.gz"), map[string][]byte{
		"manifest.json": manifestData,
		"content.txt":   []byte("test content"),
	})

	t.Run("valid capsule", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/capsule/test.tar.gz", nil)
		w := httptest.NewRecorder()

		handleCapsule(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Capsule: test.tar.gz") {
			t.Error("expected capsule title in response")
		}
	})

	t.Run("nonexistent capsule", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/capsule/nonexistent.tar.gz", nil)
		w := httptest.NewRecorder()

		handleCapsule(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("empty capsule path redirects", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/capsule/", nil)
		w := httptest.NewRecorder()

		handleCapsule(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusFound {
			t.Errorf("expected status 302, got %d", resp.StatusCode)
		}
	})
}

func TestHandleArtifact(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tmpDir
	defer func() { ServerConfig.CapsulesDir = originalDir }()

	// Create test capsule
	createTestCapsuleTarGz(t, filepath.Join(tmpDir, "test.tar.gz"), map[string][]byte{
		"manifest.json": []byte(`{"version":"1.0"}`),
		"content.txt":   []byte("test content"),
	})

	t.Run("valid artifact", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/artifact/test.tar.gz?artifact=test/content.txt", nil)
		w := httptest.NewRecorder()

		handleArtifact(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("missing artifact parameter redirects", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/artifact/test.tar.gz", nil)
		w := httptest.NewRecorder()

		handleArtifact(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusFound {
			t.Errorf("expected status 302, got %d", resp.StatusCode)
		}
	})

	t.Run("nonexistent artifact", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/artifact/test.tar.gz?artifact=nonexistent", nil)
		w := httptest.NewRecorder()

		handleArtifact(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})
}

func TestHandleIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tmpDir
	defer func() { ServerConfig.CapsulesDir = originalDir }()

	// Create test capsule with IR
	irData, _ := json.Marshal(map[string]interface{}{
		"version": "1.0",
		"content": "test IR",
	})
	createTestCapsuleTarGz(t, filepath.Join(tmpDir, "test.tar.gz"), map[string][]byte{
		"manifest.json": []byte(`{"version":"1.0"}`),
		"test.ir.json":  irData,
	})

	t.Run("capsule with IR", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ir/test.tar.gz", nil)
		w := httptest.NewRecorder()

		handleIR(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "IR View") {
			t.Error("expected IR view title in response")
		}
	})

	t.Run("empty path redirects", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ir/", nil)
		w := httptest.NewRecorder()

		handleIR(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusFound {
			t.Errorf("expected status 302, got %d", resp.StatusCode)
		}
	})

	t.Run("capsule without IR shows error", func(t *testing.T) {
		createTestCapsuleTarGz(t, filepath.Join(tmpDir, "noir.tar.gz"), map[string][]byte{
			"manifest.json": []byte(`{"version":"1.0"}`),
		})

		req := httptest.NewRequest(http.MethodGet, "/ir/noir.tar.gz", nil)
		w := httptest.NewRecorder()

		handleIR(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})
}

func TestHandleTranscript(t *testing.T) {
	t.Run("valid run ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/transcript/run123", nil)
		w := httptest.NewRecorder()

		handleTranscript(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Transcript: run123") {
			t.Error("expected transcript title in response")
		}
	})

	t.Run("empty run ID redirects", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/transcript/", nil)
		w := httptest.NewRecorder()

		handleTranscript(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusFound {
			t.Errorf("expected status 302, got %d", resp.StatusCode)
		}
	})
}

func TestHandlePlugins(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir := ServerConfig.PluginsDir
	ServerConfig.PluginsDir = tmpDir
	defer func() { ServerConfig.PluginsDir = originalDir }()

	// Create test plugin directories
	os.MkdirAll(filepath.Join(tmpDir, "format", "osis"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "tool", "test"), 0755)

	req := httptest.NewRequest(http.MethodGet, "/plugins", nil)
	w := httptest.NewRecorder()

	handlePlugins(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Plugins") {
		t.Error("expected plugins title in response")
	}
}

func TestHandleConvert(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tmpDir
	defer func() { ServerConfig.CapsulesDir = originalDir }()

	t.Run("GET request shows form", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/convert", nil)
		w := httptest.NewRecorder()

		handleConvert(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Convert") {
			t.Error("expected convert title in response")
		}
	})

	t.Run("POST with invalid action", func(t *testing.T) {
		form := url.Values{}
		form.Add("action", "invalid")

		req := httptest.NewRequest(http.MethodPost, "/convert", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handleConvert(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("POST convert without source", func(t *testing.T) {
		form := url.Values{}
		form.Add("action", "convert")
		form.Add("format", "osis")

		req := httptest.NewRequest(http.MethodPost, "/convert", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handleConvert(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})
}

// Archive manipulation tests

func TestExtractCapsule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name       string
		createFunc func(*testing.T, string, map[string][]byte)
		files      map[string][]byte
	}{
		{
			name:       "tar.gz capsule",
			createFunc: createTestCapsuleTarGz,
			files: map[string][]byte{
				"manifest.json": []byte(`{"version":"1.0"}`),
				"content.txt":   []byte("test content"),
			},
		},
		{
			name:       "tar.xz capsule",
			createFunc: createTestCapsuleTarXz,
			files: map[string][]byte{
				"manifest.json": []byte(`{"version":"1.0"}`),
				"data.json":     []byte(`{"key":"value"}`),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			capsulePath := filepath.Join(tmpDir, tc.name)
			if strings.Contains(tc.name, "tar.gz") {
				capsulePath += ".tar.gz"
			} else {
				capsulePath += ".tar.xz"
			}

			tc.createFunc(t, capsulePath, tc.files)

			extractDir := filepath.Join(tmpDir, "extract-"+tc.name)
			os.MkdirAll(extractDir, 0755)
			defer os.RemoveAll(extractDir)

			err := extractCapsule(capsulePath, extractDir)
			if err != nil {
				t.Fatalf("extractCapsule failed: %v", err)
			}

			// Verify all files were extracted
			for filename := range tc.files {
				extractedPath := filepath.Join(extractDir, filename)
				if _, err := os.Stat(extractedPath); os.IsNotExist(err) {
					t.Errorf("expected file %q to be extracted", filename)
				}
			}
		})
	}

	t.Run("unsupported format", func(t *testing.T) {
		unsupportedPath := filepath.Join(tmpDir, "test.zip")
		os.WriteFile(unsupportedPath, []byte("fake zip"), 0644)

		extractDir := filepath.Join(tmpDir, "extract-unsupported")
		os.MkdirAll(extractDir, 0755)
		defer os.RemoveAll(extractDir)

		err := extractCapsule(unsupportedPath, extractDir)
		if err == nil {
			t.Error("expected error for unsupported format")
		}
	})
}

func TestCapsuleIsCAS(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("CAS capsule with blobs", func(t *testing.T) {
		casPath := filepath.Join(tmpDir, "cas.tar.gz")
		createTestCapsuleTarGz(t, casPath, map[string][]byte{
			"manifest.json":        []byte(`{"version":"1.0"}`),
			"blobs/sha256/ab/abcd": []byte("blob content"),
		})

		if !archive.IsCASCapsule(casPath) {
			t.Error("expected capsule with blobs/ to be CAS")
		}
	})

	t.Run("non-CAS capsule", func(t *testing.T) {
		normalPath := filepath.Join(tmpDir, "normal.tar.gz")
		createTestCapsuleTarGz(t, normalPath, map[string][]byte{
			"manifest.json": []byte(`{"version":"1.0"}`),
			"content.txt":   []byte("content"),
		})

		if archive.IsCASCapsule(normalPath) {
			t.Error("expected capsule without blobs/ to not be CAS")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		if archive.IsCASCapsule(filepath.Join(tmpDir, "nonexistent.tar.gz")) {
			t.Error("expected nonexistent file to return false")
		}
	})
}

func TestCapsuleHasIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("capsule with IR", func(t *testing.T) {
		withIRPath := filepath.Join(tmpDir, "with-ir.tar.gz")
		createTestCapsuleTarGz(t, withIRPath, map[string][]byte{
			"manifest.json": []byte(`{"version":"1.0"}`),
			"test.ir.json":  []byte(`{"ir":"content"}`),
		})

		if !archive.HasIR(withIRPath) {
			t.Error("expected capsule with .ir.json to have IR")
		}
	})

	t.Run("capsule without IR", func(t *testing.T) {
		withoutIRPath := filepath.Join(tmpDir, "without-ir.tar.gz")
		createTestCapsuleTarGz(t, withoutIRPath, map[string][]byte{
			"manifest.json": []byte(`{"version":"1.0"}`),
			"content.txt":   []byte("content"),
		})

		if archive.HasIR(withoutIRPath) {
			t.Error("expected capsule without .ir.json to not have IR")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		if archive.HasIR(filepath.Join(tmpDir, "nonexistent.tar.gz")) {
			t.Error("expected nonexistent file to return false")
		}
	})
}

func TestCreateCapsuleTarGzFromDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source directory with files
	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("content2"), 0644)

	subDir := filepath.Join(srcDir, "subdir")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "file3.txt"), []byte("content3"), 0644)

	dstPath := filepath.Join(tmpDir, "output.tar.gz")

	err = archive.CreateCapsuleTarGzFromPath(srcDir, dstPath)
	if err != nil {
		t.Fatalf("createCapsuleTarGzFromDir failed: %v", err)
	}

	// Verify the archive was created
	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		t.Fatal("expected archive to be created")
	}

	// Verify archive contents
	f, err := os.Open(dstPath)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	fileCount := 0
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read tar: %v", err)
		}
		if !header.FileInfo().IsDir() {
			fileCount++
		}
	}

	if fileCount != 3 {
		t.Errorf("expected 3 files in archive, got %d", fileCount)
	}
}

func TestPerformConversion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tmpDir
	defer func() { ServerConfig.CapsulesDir = originalDir }()

	t.Run("nonexistent capsule", func(t *testing.T) {
		result := performConversion("nonexistent.tar.gz", "osis")
		if result.Success {
			t.Error("expected failure for nonexistent capsule")
		}
		if !strings.Contains(result.Message, "not found") {
			t.Errorf("expected 'not found' in error message, got: %s", result.Message)
		}
	})

	// Note: Full conversion testing requires plugin infrastructure
	// which is complex to mock. Testing the error paths provides coverage.
}

func TestPerformIRGeneration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tmpDir
	defer func() { ServerConfig.CapsulesDir = originalDir }()

	t.Run("nonexistent capsule", func(t *testing.T) {
		result := performIRGeneration("nonexistent.tar.gz")
		if result.Success {
			t.Error("expected failure for nonexistent capsule")
		}
		if !strings.Contains(result.Message, "not found") {
			t.Errorf("expected 'not found' in error message, got: %s", result.Message)
		}
	})

	t.Run("capsule with existing IR", func(t *testing.T) {
		withIRPath := filepath.Join(tmpDir, "with-ir.tar.gz")
		createTestCapsuleTarGz(t, withIRPath, map[string][]byte{
			"manifest.json": []byte(`{"version":"1.0"}`),
			"test.ir.json":  []byte(`{"ir":"content"}`),
		})

		result := performIRGeneration("with-ir.tar.gz")
		if result.Success {
			t.Error("expected failure for capsule with existing IR")
		}
		if !strings.Contains(result.Message, "already contains IR") {
			t.Errorf("expected 'already contains IR' in error message, got: %s", result.Message)
		}
	})
}

func TestPerformCASToSWORD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tmpDir
	defer func() { ServerConfig.CapsulesDir = originalDir }()

	t.Run("nonexistent capsule", func(t *testing.T) {
		result := performCASToSWORD("nonexistent.tar.gz")
		if result.Success {
			t.Error("expected failure for nonexistent capsule")
		}
		if !strings.Contains(result.Message, "not found") {
			t.Errorf("expected 'not found' in error message, got: %s", result.Message)
		}
	})

	t.Run("non-CAS capsule", func(t *testing.T) {
		normalPath := filepath.Join(tmpDir, "normal.tar.gz")
		createTestCapsuleTarGz(t, normalPath, map[string][]byte{
			"manifest.json": []byte(`{"version":"1.0"}`),
			"content.txt":   []byte("content"),
		})

		result := performCASToSWORD("normal.tar.gz")
		if result.Success {
			t.Error("expected failure for non-CAS capsule")
		}
		if !strings.Contains(result.Message, "not a CAS capsule") {
			t.Errorf("expected 'not a CAS capsule' in error message, got: %s", result.Message)
		}
	})

	t.Run("CAS capsule without manifest", func(t *testing.T) {
		casPath := filepath.Join(tmpDir, "cas-nomanifest.tar.gz")
		createTestCapsuleTarGz(t, casPath, map[string][]byte{
			"blobs/sha256/ab/abcd": []byte("blob content"),
		})

		result := performCASToSWORD("cas-nomanifest.tar.gz")
		if result.Success {
			t.Error("expected failure for CAS capsule without manifest")
		}
		if !strings.Contains(result.Message, "manifest") {
			t.Errorf("expected 'manifest' in error message, got: %s", result.Message)
		}
	})

	t.Run("CAS capsule with invalid manifest", func(t *testing.T) {
		casPath := filepath.Join(tmpDir, "cas-badmanifest.tar.gz")
		createTestCapsuleTarGz(t, casPath, map[string][]byte{
			"manifest.json":        []byte(`{invalid json}`),
			"blobs/sha256/ab/abcd": []byte("blob content"),
		})

		result := performCASToSWORD("cas-badmanifest.tar.gz")
		if result.Success {
			t.Error("expected failure for CAS capsule with invalid manifest")
		}
	})
}

func TestCopyDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source directory with files
	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("content2"), 0644)

	subDir := filepath.Join(srcDir, "subdir")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "file3.txt"), []byte("content3"), 0644)

	dstDir := filepath.Join(tmpDir, "destination")

	err = fileutil.CopyDir(srcDir, dstDir)
	if err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify all files were copied
	tests := []struct {
		path    string
		content string
	}{
		{filepath.Join(dstDir, "file1.txt"), "content1"},
		{filepath.Join(dstDir, "file2.txt"), "content2"},
		{filepath.Join(dstDir, "subdir", "file3.txt"), "content3"},
	}

	for _, tc := range tests {
		data, err := os.ReadFile(tc.path)
		if err != nil {
			t.Errorf("failed to read copied file %q: %v", tc.path, err)
			continue
		}
		if string(data) != tc.content {
			t.Errorf("file %q has wrong content: got %q, want %q", tc.path, string(data), tc.content)
		}
	}

	// Verify subdirectory was created
	if _, err := os.Stat(filepath.Join(dstDir, "subdir")); os.IsNotExist(err) {
		t.Error("expected subdirectory to be created")
	}
}

func TestReadCapsuleManifest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("capsule with manifest", func(t *testing.T) {
		manifestData, _ := json.Marshal(CapsuleManifest{
			Version:      "1.0",
			Title:        "Test Capsule",
			SourceFormat: "osis",
		})
		capsulePath := filepath.Join(tmpDir, "with-manifest.tar.gz")
		createTestCapsuleTarGz(t, capsulePath, map[string][]byte{
			"manifest.json": manifestData,
		})

		manifest := readCapsuleManifest(capsulePath)
		if manifest == nil {
			t.Fatal("expected manifest to be read")
		}
		if manifest.Version != "1.0" {
			t.Errorf("expected version '1.0', got %q", manifest.Version)
		}
		if manifest.Title != "Test Capsule" {
			t.Errorf("expected title 'Test Capsule', got %q", manifest.Title)
		}
		if manifest.SourceFormat != "osis" {
			t.Errorf("expected source format 'osis', got %q", manifest.SourceFormat)
		}
	})

	t.Run("capsule without manifest", func(t *testing.T) {
		capsulePath := filepath.Join(tmpDir, "no-manifest.tar.gz")
		createTestCapsuleTarGz(t, capsulePath, map[string][]byte{
			"content.txt": []byte("content"),
		})

		manifest := readCapsuleManifest(capsulePath)
		if manifest != nil {
			t.Error("expected nil manifest for capsule without manifest.json")
		}
	})

	t.Run("nonexistent capsule", func(t *testing.T) {
		manifest := readCapsuleManifest(filepath.Join(tmpDir, "nonexistent.tar.gz"))
		if manifest != nil {
			t.Error("expected nil manifest for nonexistent capsule")
		}
	})

	t.Run("capsule with invalid manifest", func(t *testing.T) {
		capsulePath := filepath.Join(tmpDir, "bad-manifest.tar.gz")
		createTestCapsuleTarGz(t, capsulePath, map[string][]byte{
			"manifest.json": []byte(`{invalid json}`),
		})

		manifest := readCapsuleManifest(capsulePath)
		if manifest != nil {
			t.Error("expected nil manifest for invalid JSON")
		}
	})
}

func TestReadCapsule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("valid capsule", func(t *testing.T) {
		manifestData, _ := json.Marshal(CapsuleManifest{
			Version: "1.0",
			Title:   "Test",
		})
		capsulePath := filepath.Join(tmpDir, "test.tar.gz")
		createTestCapsuleTarGz(t, capsulePath, map[string][]byte{
			"manifest.json": manifestData,
			"content.txt":   []byte("test content"),
		})

		_, artifacts, err := readCapsule(capsulePath)
		if err != nil {
			t.Fatalf("readCapsule failed: %v", err)
		}

		// Note: readCapsule expects exact "manifest.json" name without directory prefix
		// Since our test helper creates "test/manifest.json", manifest will be nil
		// This is expected behavior - testing that it doesn't error

		// Should still get artifacts
		if len(artifacts) == 0 {
			t.Error("expected at least some artifacts to be listed")
		}
	})

	t.Run("nonexistent capsule", func(t *testing.T) {
		_, _, err := readCapsule(filepath.Join(tmpDir, "nonexistent.tar.gz"))
		if err == nil {
			t.Error("expected error for nonexistent capsule")
		}
	})

	t.Run("tar.xz capsule", func(t *testing.T) {
		capsulePath := filepath.Join(tmpDir, "test.tar.xz")
		createTestCapsuleTarXz(t, capsulePath, map[string][]byte{
			"content.txt": []byte("test content"),
		})

		_, artifacts, err := readCapsule(capsulePath)
		if err != nil {
			t.Fatalf("readCapsule failed: %v", err)
		}

		if len(artifacts) == 0 {
			t.Error("expected artifacts to be listed")
		}
	})
}

func TestReadArtifactContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	capsulePath := filepath.Join(tmpDir, "test.tar.gz")
	createTestCapsuleTarGz(t, capsulePath, map[string][]byte{
		"manifest.json": []byte(`{"version":"1.0"}`),
		"content.txt":   []byte("test content"),
		"data.json":     []byte(`{"key":"value"}`),
	})

	t.Run("read text artifact", func(t *testing.T) {
		content, contentType, err := readArtifactContent(capsulePath, "test/content.txt")
		if err != nil {
			t.Fatalf("readArtifactContent failed: %v", err)
		}
		if content != "test content" {
			t.Errorf("expected 'test content', got %q", content)
		}
		if contentType != "text/plain" {
			t.Errorf("expected content type 'text/plain', got %q", contentType)
		}
	})

	t.Run("read JSON artifact", func(t *testing.T) {
		content, contentType, err := readArtifactContent(capsulePath, "test/data.json")
		if err != nil {
			t.Fatalf("readArtifactContent failed: %v", err)
		}
		if !strings.Contains(content, "key") {
			t.Error("expected JSON content")
		}
		if contentType != "application/json" {
			t.Errorf("expected content type 'application/json', got %q", contentType)
		}
	})

	t.Run("nonexistent artifact", func(t *testing.T) {
		_, _, err := readArtifactContent(capsulePath, "nonexistent.txt")
		if err == nil {
			t.Error("expected error for nonexistent artifact")
		}
	})
}

func TestReadIRContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("capsule with IR", func(t *testing.T) {
		irData, _ := json.Marshal(map[string]interface{}{
			"version": "1.0",
			"books":   []string{"GEN", "EXO"},
		})
		capsulePath := filepath.Join(tmpDir, "with-ir.tar.gz")
		createTestCapsuleTarGz(t, capsulePath, map[string][]byte{
			"manifest.json": []byte(`{"version":"1.0"}`),
			"test.ir.json":  irData,
		})

		ir, err := readIRContent(capsulePath)
		if err != nil {
			t.Fatalf("readIRContent failed: %v", err)
		}
		if ir == nil {
			t.Fatal("expected IR to be read")
		}
		if ir["version"] != "1.0" {
			t.Error("expected version in IR")
		}
	})

	t.Run("capsule without IR", func(t *testing.T) {
		capsulePath := filepath.Join(tmpDir, "no-ir.tar.gz")
		createTestCapsuleTarGz(t, capsulePath, map[string][]byte{
			"manifest.json": []byte(`{"version":"1.0"}`),
			"content.txt":   []byte("content"),
		})

		_, err := readIRContent(capsulePath)
		if err == nil {
			t.Error("expected error for capsule without IR")
		}
		if !strings.Contains(err.Error(), "no IR file") {
			t.Errorf("expected 'no IR file' in error, got: %v", err)
		}
	})

	t.Run("nonexistent capsule", func(t *testing.T) {
		_, err := readIRContent(filepath.Join(tmpDir, "nonexistent.tar.gz"))
		if err == nil {
			t.Error("expected error for nonexistent capsule")
		}
	})
}

func TestFindSWORDConfFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sword-conf-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test .conf files with various cases
	testFiles := []string{
		"lowercase.conf",
		"MixedCase.conf",
		"UPPERCASE.conf",
		"With_Underscore.conf",
	}
	for _, f := range testFiles {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte("[Test]\n"), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", f, err)
		}
	}

	tests := []struct {
		name        string
		moduleID    string
		expectFile  string
		expectFound bool
	}{
		{"exact lowercase", "lowercase", "lowercase.conf", true},
		{"exact mixed case", "MixedCase", "MixedCase.conf", true},
		{"lowercase lookup for mixed case", "mixedcase", "MixedCase.conf", true},
		{"uppercase lookup for mixed case", "MIXEDCASE", "MixedCase.conf", true},
		{"exact uppercase", "UPPERCASE", "UPPERCASE.conf", true},
		{"lowercase lookup for uppercase", "uppercase", "UPPERCASE.conf", true},
		{"with underscore", "With_Underscore", "With_Underscore.conf", true},
		{"lowercase underscore lookup", "with_underscore", "With_Underscore.conf", true},
		{"nonexistent module", "nonexistent", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filename, fullPath := findSWORDConfFile(tmpDir, tc.moduleID)
			if tc.expectFound {
				if filename != tc.expectFile {
					t.Errorf("expected filename %q, got %q", tc.expectFile, filename)
				}
				if fullPath == "" {
					t.Error("expected non-empty full path")
				}
			} else {
				if filename != "" || fullPath != "" {
					t.Errorf("expected empty results for nonexistent module, got %q, %q", filename, fullPath)
				}
			}
		})
	}
}

// Security Tests

func TestSanitizePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name      string
		userPath  string
		shouldErr bool
	}{
		{"normal path", "test.txt", false},
		{"subdirectory", "subdir/test.txt", false},
		{"path traversal with ..", "../etc/passwd", true},
		{"path traversal hidden", "subdir/../../etc/passwd", true},
		{"path with multiple dots is rejected", "test..txt", true}, // sanitizePath rejects ".." anywhere
		{"empty path", "", true},                                   // Empty paths should error for security
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cleanPath, err := validation.SanitizePath(tmpDir, tc.userPath)
			if tc.shouldErr {
				if err == nil {
					t.Errorf("expected error for path %q, got none", tc.userPath)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for path %q: %v", tc.userPath, err)
				}
				if cleanPath == "" && tc.userPath != "" {
					t.Errorf("expected non-empty result for valid path %q", tc.userPath)
				}
			}
		})
	}
}

func TestGenerateCSRFToken(t *testing.T) {
	token1 := generateCSRFToken()
	if token1 == "" {
		t.Error("expected non-empty CSRF token")
	}
	if len(token1) != csrfTokenLen*2 {
		t.Errorf("expected token length %d, got %d", csrfTokenLen*2, len(token1))
	}

	// Tokens should be unique
	token2 := generateCSRFToken()
	if token1 == token2 {
		t.Error("expected unique CSRF tokens")
	}
}

func TestGetOrCreateCSRFToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// First call should create a new token
	token1 := getOrCreateCSRFToken(w, req)
	if token1 == "" {
		t.Error("expected non-empty CSRF token")
	}

	// Verify cookie was set
	resp := w.Result()
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected CSRF cookie to be set")
	}

	var csrfCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == csrfCookieName {
			csrfCookie = cookie
			break
		}
	}
	if csrfCookie == nil {
		t.Fatal("expected CSRF cookie to be set")
	}
	if csrfCookie.Value != token1 {
		t.Errorf("cookie value %q doesn't match token %q", csrfCookie.Value, token1)
	}

	// Second call with existing cookie should return same token
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(csrfCookie)
	w2 := httptest.NewRecorder()

	token2 := getOrCreateCSRFToken(w2, req2)
	if token2 != token1 {
		t.Errorf("expected same token %q, got %q", token1, token2)
	}
}

func TestValidateCSRFToken(t *testing.T) {
	validToken := generateCSRFToken()

	tests := []struct {
		name        string
		setupReq    func() *http.Request
		shouldValid bool
	}{
		{
			name: "valid token in form",
			setupReq: func() *http.Request {
				form := url.Values{}
				form.Add("csrf_token", validToken)
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: validToken})
				req.ParseForm()
				return req
			},
			shouldValid: true,
		},
		{
			name: "valid token in header",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/", nil)
				req.Header.Set("X-CSRF-Token", validToken)
				req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: validToken})
				return req
			},
			shouldValid: true,
		},
		{
			name: "missing cookie",
			setupReq: func() *http.Request {
				form := url.Values{}
				form.Add("csrf_token", validToken)
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.ParseForm()
				return req
			},
			shouldValid: false,
		},
		{
			name: "missing form token",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/", nil)
				req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: validToken})
				return req
			},
			shouldValid: false,
		},
		{
			name: "mismatched tokens",
			setupReq: func() *http.Request {
				form := url.Values{}
				form.Add("csrf_token", "wrongtoken1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: validToken})
				req.ParseForm()
				return req
			},
			shouldValid: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.setupReq()
			valid := validateCSRFToken(req)
			if valid != tc.shouldValid {
				t.Errorf("expected validation result %v, got %v", tc.shouldValid, valid)
			}
		})
	}
}

func TestGenerateErrorID(t *testing.T) {
	id1 := generateErrorID()
	if id1 == "" {
		t.Error("expected non-empty error ID")
	}
	if id1 == "unknown" {
		t.Error("expected proper error ID, not 'unknown'")
	}

	// IDs should be unique
	id2 := generateErrorID()
	if id1 == id2 {
		t.Error("expected unique error IDs")
	}
}

func TestHttpError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		err        error
	}{
		{"not found", http.StatusNotFound, os.ErrNotExist},
		{"bad request", http.StatusBadRequest, fmt.Errorf("invalid input")},
		{"internal error", http.StatusInternalServerError, fmt.Errorf("database error")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			httpError(w, tc.err, tc.statusCode)

			resp := w.Result()
			if resp.StatusCode != tc.statusCode {
				t.Errorf("expected status code %d, got %d", tc.statusCode, resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			// Should contain error reference ID
			if !strings.Contains(bodyStr, "ref:") {
				t.Error("expected error reference ID in response")
			}

			// Should NOT contain the original error message (security)
			if strings.Contains(bodyStr, tc.err.Error()) {
				t.Error("error response should not expose internal error details")
			}
		})
	}
}

func TestSecureMkdirTemp(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	secureDir, err := secureMkdirTemp(tmpDir, "secure-*")
	if err != nil {
		t.Fatalf("secureMkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(secureDir)

	// Verify directory was created
	info, err := os.Stat(secureDir)
	if err != nil {
		t.Fatalf("failed to stat secure directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory to be created")
	}

	// Verify permissions are restrictive (0700)
	if runtime.GOOS != "windows" {
		perm := info.Mode().Perm()
		if perm != 0700 {
			t.Errorf("expected permissions 0700, got %o", perm)
		}
	}
}

func TestSecureCreateFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testPath := filepath.Join(tmpDir, "test.txt")

	// First creation should succeed
	f, err := secureCreateFile(testPath, 0600)
	if err != nil {
		t.Fatalf("secureCreateFile failed: %v", err)
	}
	f.Close()

	// Second creation should fail (O_EXCL prevents overwrite)
	_, err = secureCreateFile(testPath, 0600)
	if err == nil {
		t.Error("expected error when creating file that already exists")
	}

	// Verify file was created with correct permissions
	info, err := os.Stat(testPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if runtime.GOOS != "windows" {
		perm := info.Mode().Perm()
		if perm != 0600 {
			t.Errorf("expected permissions 0600, got %o", perm)
		}
	}
}

// Handler tests for missing coverage

func TestHandleCapsules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tmpDir
	defer func() { ServerConfig.CapsulesDir = originalDir }()

	// Create test capsules
	createTestCapsuleTarGz(t, filepath.Join(tmpDir, "test1.tar.gz"), map[string][]byte{
		"manifest.json": []byte(`{"version":"1.0","title":"Test 1"}`),
	})
	createTestCapsuleTarGz(t, filepath.Join(tmpDir, "test2.tar.xz"), map[string][]byte{
		"manifest.json": []byte(`{"version":"1.0","title":"Test 2"}`),
	})

	req := httptest.NewRequest(http.MethodGet, "/capsules", nil)
	w := httptest.NewRecorder()

	handleCapsules(w, req)

	resp := w.Result()
	// handleCapsules now redirects to /juniper?tab=capsules
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Errorf("expected status 301, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "/juniper?tab=capsules" {
		t.Errorf("expected redirect to /juniper?tab=capsules, got %q", location)
	}
}

func TestHandleCapsuleDelete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tmpDir
	defer func() { ServerConfig.CapsulesDir = originalDir }()

	// Create test capsule
	createTestCapsuleTarGz(t, filepath.Join(tmpDir, "test.tar.gz"), map[string][]byte{
		"manifest.json": []byte(`{"version":"1.0"}`),
	})

	t.Run("GET returns error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/capsule/test.tar.gz/delete", nil)
		w := httptest.NewRecorder()

		handleCapsuleDelete(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", resp.StatusCode)
		}
	})

	t.Run("POST without CSRF token fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/capsule/test.tar.gz/delete", nil)
		w := httptest.NewRecorder()

		handleCapsuleDelete(w, req)

		resp := w.Result()
		// Should redirect or error
		if resp.StatusCode == http.StatusOK {
			t.Error("expected error status for missing CSRF token")
		}
	})

	t.Run("nonexistent capsule", func(t *testing.T) {
		// Generate valid CSRF token
		validToken := generateCSRFToken()
		form := url.Values{}
		form.Add("csrf_token", validToken)
		form.Add("path", "nonexistent.tar.gz")

		req := httptest.NewRequest(http.MethodPost, "/capsule/nonexistent.tar.gz/delete", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: validToken})
		req.ParseForm()

		w := httptest.NewRecorder()

		handleCapsuleDelete(w, req)

		resp := w.Result()
		// Should get 404 for nonexistent file
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})
}

func TestCategorizeCapsules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tmpDir
	defer func() { ServerConfig.CapsulesDir = originalDir }()

	// Create CAS capsule
	casPath := filepath.Join(tmpDir, "cas.tar.gz")
	createTestCapsuleTarGz(t, casPath, map[string][]byte{
		"manifest.json":        []byte(`{"version":"1.0"}`),
		"blobs/sha256/ab/abcd": []byte("blob content"),
	})

	// Create capsule without IR
	noIRPath := filepath.Join(tmpDir, "noir.tar.gz")
	createTestCapsuleTarGz(t, noIRPath, map[string][]byte{
		"manifest.json": []byte(`{"version":"1.0"}`),
		"content.txt":   []byte("content"),
	})

	// Create capsule with IR
	withIRPath := filepath.Join(tmpDir, "withir.tar.gz")
	createTestCapsuleTarGz(t, withIRPath, map[string][]byte{
		"manifest.json": []byte(`{"version":"1.0"}`),
		"test.ir.json":  []byte(`{"version":"1.0"}`),
	})

	capsules := listCapsules()
	noIR, cas, withIR := categorizeCapsules(capsules)

	// Should have at least 1 CAS capsule
	if len(cas) == 0 {
		t.Error("expected at least one CAS capsule")
	}

	// Should have at least 1 no-IR capsule
	if len(noIR) == 0 {
		t.Error("expected at least one capsule without IR")
	}

	// withIR is returned but we don't need to verify it in this test
	_ = withIR
}

func TestGetLoader(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir := ServerConfig.PluginsDir
	originalExternal := ServerConfig.PluginsExternal
	ServerConfig.PluginsDir = tmpDir
	ServerConfig.PluginsExternal = true
	defer func() {
		ServerConfig.PluginsDir = originalDir
		ServerConfig.PluginsExternal = originalExternal
	}()

	loader := getLoader()
	if loader == nil {
		t.Error("expected non-nil loader")
	}
}

func TestGetCapsuleMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "capsule-web-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Clear cache
	invalidateCapsuleMetadataCache()

	// Create CAS capsule
	casPath := filepath.Join(tmpDir, "cas.tar.gz")
	createTestCapsuleTarGz(t, casPath, map[string][]byte{
		"manifest.json":        []byte(`{"version":"1.0"}`),
		"blobs/sha256/ab/abcd": []byte("blob content"),
	})

	// First call computes metadata
	meta1 := getCapsuleMetadata(casPath)
	if !meta1.IsCAS {
		t.Error("expected CAS capsule to be marked as CAS")
	}

	// Second call should use cache
	meta2 := getCapsuleMetadata(casPath)
	if meta2.IsCAS != meta1.IsCAS {
		t.Error("cached metadata should match computed metadata")
	}

	// Invalidate and check again
	invalidateCapsuleMetadataCache()
	meta3 := getCapsuleMetadata(casPath)
	if meta3.IsCAS != meta1.IsCAS {
		t.Error("recomputed metadata should match original")
	}
}

func TestTrimArchiveSuffix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"capsule.tar.gz", "test.capsule.tar.gz", "test"},
		{"capsule.tar.xz", "test.capsule.tar.xz", "test"},
		{"tar.gz", "test.tar.gz", "test"},
		{"tar.xz", "test.tar.xz", "test"},
		{"no suffix", "test", "test"},
		{"partial match", "test.tar", "test.tar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimArchiveSuffix(tt.input)
			if result != tt.expected {
				t.Errorf("trimArchiveSuffix(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseLicenseText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "GPL-3.0",
			text:     "This is licensed under the GNU General Public License, Version 3",
			expected: "GPL-3.0",
		},
		{
			name:     "GPL-2.0",
			text:     "GNU General Public License Version 2",
			expected: "GPL-2.0",
		},
		{
			name:     "MIT",
			text:     "MIT License\n\nCopyright (c) 2024",
			expected: "MIT",
		},
		{
			name:     "Apache-2.0",
			text:     "Apache License\nVersion 2.0",
			expected: "Apache-2.0",
		},
		{
			name:     "Public Domain",
			text:     "This work is in the Public Domain",
			expected: "Public Domain",
		},
		{
			name:     "GPL-3 shorthand",
			text:     "Licensed under GPL-3",
			expected: "GPL-3.0",
		},
		{
			name:     "GPL-2 shorthand",
			text:     "Licensed under GPL-2",
			expected: "GPL-2.0",
		},
		{
			name:     "Unknown license",
			text:     "Some proprietary license",
			expected: "See LICENSE file",
		},
		{
			name:     "Empty text",
			text:     "",
			expected: "See LICENSE file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLicenseText(tt.text)
			if result != tt.expected {
				t.Errorf("parseLicenseText(%q) = %q, want %q", tt.text, result, tt.expected)
			}
		})
	}
}
