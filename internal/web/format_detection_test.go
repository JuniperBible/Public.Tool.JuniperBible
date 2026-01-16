package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func TestFormatDetector_DetectByExtension(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"XML file", "test.xml", "osis"},
		{"OSIS file", "test.osis", "osis"},
		{"USFM file", "test.usfm", "usfm"},
		{"SFM file", "test.sfm", "usfm"},
		{"USX file", "test.usx", "usx"},
		{"ZIP file", "test.zip", "zip"},
		{"TAR file", "test.tar", "tar"},
		{"JSON file", "test.json", "json"},
		{"EPUB file", "test.epub", "epub"},
		{"Unknown file", "test.unknown", "file"},
		{"No extension", "testfile", "file"},
		{"Uppercase extension", "TEST.XML", "osis"},
	}

	detector := NewFormatDetector(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectByExtension(tt.path)
			if result != tt.expected {
				t.Errorf("DetectByExtension(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestFormatDetector_DetectCapsuleFormat(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"tar.xz capsule", "test.capsule.tar.xz", "tar.xz"},
		{"tar.gz capsule", "test.capsule.tar.gz", "tar.gz"},
		{"tar capsule", "test.capsule.tar", "tar"},
		{"tar.xz without capsule", "test.tar.xz", "tar.xz"},
		{"tar.gz without capsule", "test.tar.gz", "tar.gz"},
		{"tar without capsule", "test.tar", "tar"},
		{"unknown format", "test.zip", "unknown"},
	}

	detector := NewFormatDetector(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectCapsuleFormat(tt.path)
			if result != tt.expected {
				t.Errorf("DetectCapsuleFormat(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestFormatDetector_DetectSourceFormat(t *testing.T) {
	tests := []struct {
		name         string
		capsulePath  string
		manifest     *CapsuleManifest
		expected     string
	}{
		{
			name:        "manifest with source format",
			capsulePath: "test.capsule.tar.xz",
			manifest:    &CapsuleManifest{SourceFormat: "sword"},
			expected:    "sword",
		},
		{
			name:        "manifest with osis format",
			capsulePath: "test.capsule.tar.xz",
			manifest:    &CapsuleManifest{SourceFormat: "osis"},
			expected:    "osis",
		},
		{
			name:        "filename contains sword",
			capsulePath: "bible-sword.capsule.tar.xz",
			manifest:    nil,
			expected:    "sword",
		},
		{
			name:        "filename with .sword.tar.gz",
			capsulePath: "kjv.sword.tar.gz",
			manifest:    nil,
			expected:    "sword",
		},
		{
			name:        "filename contains osis",
			capsulePath: "bible-osis.capsule.tar.xz",
			manifest:    nil,
			expected:    "osis",
		},
		{
			name:        "filename contains usfm",
			capsulePath: "bible-usfm.capsule.tar.xz",
			manifest:    nil,
			expected:    "usfm",
		},
		{
			name:        "unknown source format",
			capsulePath: "bible.capsule.tar.xz",
			manifest:    nil,
			expected:    "unknown",
		},
		{
			name:        "nil manifest, unknown format",
			capsulePath: "test.tar.xz",
			manifest:    nil,
			expected:    "unknown",
		},
	}

	detector := NewFormatDetector(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectSourceFormat(tt.capsulePath, tt.manifest)
			if result != tt.expected {
				t.Errorf("DetectSourceFormat(%q, %v) = %q, want %q", tt.capsulePath, tt.manifest, result, tt.expected)
			}
		})
	}
}

func TestFormatDetector_DetectFileFormat_NoPlugins(t *testing.T) {
	// Test without plugins (should fall back to extension detection)
	detector := NewFormatDetector(nil)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"XML file", "test.xml", "osis"},
		{"USFM file", "test.usfm", "usfm"},
		{"Unknown file", "test.unknown", "file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectFileFormat(tt.path)
			if result != tt.expected {
				t.Errorf("DetectFileFormat(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestFormatDetector_DetectFileFormat_WithPlugins(t *testing.T) {
	// Create a temporary plugin directory
	tmpDir := t.TempDir()

	// Create a mock plugin manifest
	pluginDir := filepath.Join(tmpDir, "format.test")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	// Create a simple manifest
	manifestContent := `{
  "plugin_id": "format.test",
  "kind": "format",
  "name": "Test Format Plugin",
  "version": "1.0.0",
  "binary": "format-test"
}`
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.json"), []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Load plugins
	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(tmpDir); err != nil {
		t.Logf("plugin load failed (expected): %v", err)
	}

	detector := NewFormatDetector(loader)

	// Should fall back to extension detection since plugin binary doesn't exist
	result := detector.DetectFileFormat("test.xml")
	if result != "osis" {
		t.Errorf("DetectFileFormat with plugins = %q, want %q", result, "osis")
	}
}

func TestPackageLevelDetectFileFormat(t *testing.T) {
	// Test package-level wrapper function
	result := detectFileFormat("test.xml")
	if result != "osis" {
		t.Errorf("detectFileFormat(%q) = %q, want %q", "test.xml", result, "osis")
	}
}

func TestPackageLevelDetectFileFormatByExtension(t *testing.T) {
	// Test package-level wrapper function
	tests := []struct {
		path     string
		expected string
	}{
		{"test.xml", "osis"},
		{"test.usfm", "usfm"},
		{"test.unknown", "file"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := detectFileFormatByExtension(tt.path)
			if result != tt.expected {
				t.Errorf("detectFileFormatByExtension(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestPackageLevelDetectCapsuleFormat(t *testing.T) {
	// Test package-level wrapper function
	tests := []struct {
		path     string
		expected string
	}{
		{"test.tar.xz", "tar.xz"},
		{"test.tar.gz", "tar.gz"},
		{"test.tar", "tar"},
		{"test.zip", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := detectCapsuleFormat(tt.path)
			if result != tt.expected {
				t.Errorf("detectCapsuleFormat(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestPackageLevelDetectSourceFormat(t *testing.T) {
	// Set up ServerConfig for testing
	oldConfig := ServerConfig
	defer func() { ServerConfig = oldConfig }()

	tmpDir := t.TempDir()
	ServerConfig.CapsulesDir = tmpDir

	// Test package-level wrapper function
	result := detectSourceFormat("bible-sword.capsule.tar.xz")
	if result != "sword" {
		t.Errorf("detectSourceFormat(%q) = %q, want %q", "bible-sword.capsule.tar.xz", result, "sword")
	}

	result = detectSourceFormat("bible-osis.capsule.tar.xz")
	if result != "osis" {
		t.Errorf("detectSourceFormat(%q) = %q, want %q", "bible-osis.capsule.tar.xz", result, "osis")
	}

	result = detectSourceFormat("bible.capsule.tar.xz")
	if result != "unknown" {
		t.Errorf("detectSourceFormat(%q) = %q, want %q", "bible.capsule.tar.xz", result, "unknown")
	}
}
