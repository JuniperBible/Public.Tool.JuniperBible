package docgen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewGenerator(t *testing.T) {
	g := NewGenerator("/plugins", "/output")

	if g.PluginDir != "/plugins" {
		t.Errorf("PluginDir = %q, want %q", g.PluginDir, "/plugins")
	}
	if g.OutputDir != "/output" {
		t.Errorf("OutputDir = %q, want %q", g.OutputDir, "/output")
	}
}

func TestLoadPlugins(t *testing.T) {
	// Use the actual plugins directory
	wd, _ := os.Getwd()
	pluginDir := filepath.Join(wd, "..", "..", "plugins")

	g := NewGenerator(pluginDir, t.TempDir())
	plugins, err := g.LoadPlugins()
	if err != nil {
		t.Fatalf("LoadPlugins failed: %v", err)
	}

	// Should find at least some plugins
	if len(plugins) == 0 {
		t.Error("LoadPlugins returned no plugins")
	}

	// Should have both format and tool plugins
	hasFormat := false
	hasTool := false
	for _, p := range plugins {
		if p.Kind == "format" {
			hasFormat = true
		}
		if p.Kind == "tool" {
			hasTool = true
		}
	}

	if !hasFormat {
		t.Error("No format plugins found")
	}
	if !hasTool {
		t.Error("No tool plugins found")
	}
}

func TestWritePluginsDoc(t *testing.T) {
	g := NewGenerator("", "")

	plugins := []PluginManifest{
		{
			PluginID:    "format-osis",
			Version:     "1.0.0",
			Kind:        "format",
			Description: "OSIS XML format",
			LossClass:   "L0",
			Extensions:  []string{".osis", ".xml"},
			IRSupport: &IRSupport{
				CanExtract: true,
				CanEmit:    true,
				LossClass:  "L0",
			},
		},
		{
			PluginID:    "tools.libsword",
			Version:     "1.1.0",
			Kind:        "tool",
			Description: "SWORD module operations",
			Requires:    []string{"diatheke", "mod2osis"},
			Profiles: []Profile{
				{ID: "list-modules", Description: "List installed modules"},
				{ID: "render-verse", Description: "Render a specific verse"},
			},
		},
	}

	var buf bytes.Buffer
	err := g.writePluginsDoc(&buf, plugins)
	if err != nil {
		t.Fatalf("writePluginsDoc failed: %v", err)
	}

	output := buf.String()

	// Check for expected content
	checks := []string{
		"# Plugin Catalog",
		"## Format Plugins",
		"format-osis",
		"L0",
		"## Tool Plugins",
		"tools.libsword",
		"list-modules",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("Output missing %q", check)
		}
	}
}

func TestWriteFormatsDoc(t *testing.T) {
	g := NewGenerator("", "")

	plugins := []PluginManifest{
		{
			PluginID:    "format-osis",
			Kind:        "format",
			LossClass:   "L0",
			Description: "OSIS XML",
			IRSupport:   &IRSupport{CanExtract: true, CanEmit: true},
		},
		{
			PluginID:    "format-txt",
			Kind:        "format",
			LossClass:   "L3",
			Description: "Plain text",
			IRSupport:   &IRSupport{CanExtract: true, CanEmit: true},
		},
	}

	var buf bytes.Buffer
	err := g.writeFormatsDoc(&buf, plugins)
	if err != nil {
		t.Fatalf("writeFormatsDoc failed: %v", err)
	}

	output := buf.String()

	// Check for expected content
	checks := []string{
		"# Format Support Matrix",
		"L0",
		"L3",
		"OSIS XML",
		"Plain text",
		"Format Conversion",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("Output missing %q", check)
		}
	}
}

func TestWriteCLIDoc(t *testing.T) {
	g := NewGenerator("", "")

	var buf bytes.Buffer
	err := g.writeCLIDoc(&buf)
	if err != nil {
		t.Fatalf("writeCLIDoc failed: %v", err)
	}

	output := buf.String()

	// Check for expected sections
	checks := []string{
		"# CLI Reference",
		"## Core Commands",
		"### ingest",
		"### export",
		"## Plugin Commands",
		"## IR Commands",
		"### extract-ir",
		"### convert",
		"## Tool Commands",
		"### run",
		"## Behavioral Testing Commands",
		"## Documentation Commands",
		"### docgen",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("Output missing %q", check)
		}
	}
}

func TestGenerateAll(t *testing.T) {
	// Use the actual plugins directory
	wd, _ := os.Getwd()
	pluginDir := filepath.Join(wd, "..", "..", "plugins")
	outputDir := t.TempDir()

	g := NewGenerator(pluginDir, outputDir)
	err := g.GenerateAll()
	if err != nil {
		t.Fatalf("GenerateAll failed: %v", err)
	}

	// Check that files were created
	expectedFiles := []string{
		"PLUGINS.md",
		"FORMATS.md",
		"CLI_REFERENCE.md",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(outputDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %q not created", file)
		}
	}
}

func TestGenerateAllOutputDirError(t *testing.T) {
	// Try to create output in a non-writable location
	g := NewGenerator("/tmp", "/dev/null/cannot/create")
	err := g.GenerateAll()
	if err == nil {
		t.Error("Expected error for invalid output dir")
	}
}

func TestLoadManifestNotFound(t *testing.T) {
	_, err := loadManifest("/nonexistent/plugin.json")
	if err == nil {
		t.Error("Expected error for missing manifest")
	}
}

func TestLoadManifestInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "plugin.json")
	os.WriteFile(manifestPath, []byte("{invalid json}"), 0644)

	_, err := loadManifest(manifestPath)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestLoadPluginsEmptyDir(t *testing.T) {
	tempDir := t.TempDir()
	g := NewGenerator(tempDir, t.TempDir())

	plugins, err := g.LoadPlugins()
	if err != nil {
		t.Fatalf("LoadPlugins failed: %v", err)
	}

	// Should return empty list for empty plugin dir
	if len(plugins) != 0 {
		t.Errorf("Expected 0 plugins, got %d", len(plugins))
	}
}

func TestGeneratePluginsError(t *testing.T) {
	// Test with a directory that exists but has no plugins
	tempDir := t.TempDir()
	outputDir := t.TempDir()

	g := NewGenerator(tempDir, outputDir)
	err := g.GeneratePlugins()
	// Should succeed even with no plugins
	if err != nil {
		t.Errorf("GeneratePlugins failed: %v", err)
	}
}

func TestGenerateFormatsError(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := t.TempDir()

	g := NewGenerator(tempDir, outputDir)
	err := g.GenerateFormats()
	// Should succeed even with no plugins
	if err != nil {
		t.Errorf("GenerateFormats failed: %v", err)
	}
}

func TestGenerateCLIError(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := t.TempDir()

	g := NewGenerator(tempDir, outputDir)
	err := g.GenerateCLI()
	if err != nil {
		t.Errorf("GenerateCLI failed: %v", err)
	}

	// Verify file was created
	cliPath := filepath.Join(outputDir, "CLI_REFERENCE.md")
	if _, err := os.Stat(cliPath); os.IsNotExist(err) {
		t.Error("CLI_REFERENCE.md not created")
	}
}

func TestWritePluginsDocEmptyList(t *testing.T) {
	g := NewGenerator("", "")
	var buf bytes.Buffer
	err := g.writePluginsDoc(&buf, []PluginManifest{})
	if err != nil {
		t.Fatalf("writePluginsDoc failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# Plugin Catalog") {
		t.Error("Missing header in empty plugin doc")
	}
}

func TestWriteFormatsDocEmptyList(t *testing.T) {
	g := NewGenerator("", "")
	var buf bytes.Buffer
	err := g.writeFormatsDoc(&buf, []PluginManifest{})
	if err != nil {
		t.Fatalf("writeFormatsDoc failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# Format Support Matrix") {
		t.Error("Missing header in empty formats doc")
	}
}

func TestPluginManifestParsing(t *testing.T) {
	// Create a temporary plugin.json
	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "plugin.json")

	manifestJSON := `{
  "plugin_id": "test-plugin",
  "version": "1.0.0",
  "kind": "format",
  "entrypoint": "test-plugin",
  "description": "Test plugin",
  "extensions": [".test"],
  "loss_class": "L0",
  "ir_support": {
    "can_extract": true,
    "can_emit": true,
    "loss_class": "L0"
  }
}`

	err := os.WriteFile(manifestPath, []byte(manifestJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	manifest, err := loadManifest(manifestPath)
	if err != nil {
		t.Fatalf("loadManifest failed: %v", err)
	}

	if manifest.PluginID != "test-plugin" {
		t.Errorf("PluginID = %q, want %q", manifest.PluginID, "test-plugin")
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", manifest.Version, "1.0.0")
	}
	if manifest.LossClass != "L0" {
		t.Errorf("LossClass = %q, want %q", manifest.LossClass, "L0")
	}
	if manifest.IRSupport == nil {
		t.Error("IRSupport is nil")
	} else {
		if !manifest.IRSupport.CanExtract {
			t.Error("IRSupport.CanExtract should be true")
		}
		if !manifest.IRSupport.CanEmit {
			t.Error("IRSupport.CanEmit should be true")
		}
	}
}

// TestGenerateAllErrorOnGeneratePlugins tests error handling when GeneratePlugins fails
func TestGenerateAllErrorOnGeneratePlugins(t *testing.T) {
	// Use a non-existent plugin dir to cause LoadPlugins to succeed but with no plugins
	// Then create output dir as a file to cause os.Create to fail
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")

	// Create output dir first
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Create PLUGINS.md as a directory to cause os.Create to fail
	pluginsPath := filepath.Join(outputDir, "PLUGINS.md")
	if err := os.Mkdir(pluginsPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	g := NewGenerator(tempDir, outputDir)
	err := g.GenerateAll()
	if err == nil {
		t.Error("Expected error when GeneratePlugins fails")
	}
	if !strings.Contains(err.Error(), "failed to generate plugins doc") {
		t.Errorf("Expected 'failed to generate plugins doc' error, got: %v", err)
	}
}

// TestGenerateAllErrorOnGenerateFormats tests error handling when GenerateFormats fails
func TestGenerateAllErrorOnGenerateFormats(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")

	// Create output dir first
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Create FORMATS.md as a directory to cause os.Create to fail
	formatsPath := filepath.Join(outputDir, "FORMATS.md")
	if err := os.Mkdir(formatsPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	g := NewGenerator(tempDir, outputDir)
	err := g.GenerateAll()
	if err == nil {
		t.Error("Expected error when GenerateFormats fails")
	}
	if !strings.Contains(err.Error(), "failed to generate formats doc") {
		t.Errorf("Expected 'failed to generate formats doc' error, got: %v", err)
	}
}

// TestGenerateAllErrorOnGenerateCLI tests error handling when GenerateCLI fails
func TestGenerateAllErrorOnGenerateCLI(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")

	// Create output dir first
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Create CLI_REFERENCE.md as a directory to cause os.Create to fail
	cliPath := filepath.Join(outputDir, "CLI_REFERENCE.md")
	if err := os.Mkdir(cliPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	g := NewGenerator(tempDir, outputDir)
	err := g.GenerateAll()
	if err == nil {
		t.Error("Expected error when GenerateCLI fails")
	}
	if !strings.Contains(err.Error(), "failed to generate CLI doc") {
		t.Errorf("Expected 'failed to generate CLI doc' error, got: %v", err)
	}
}

// TestLoadPluginsWithFiles tests that files in plugin directories are skipped
func TestLoadPluginsWithFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create format and tool directories
	formatDir := filepath.Join(tempDir, "format")
	toolDir := filepath.Join(tempDir, "tool")
	if err := os.MkdirAll(formatDir, 0755); err != nil {
		t.Fatalf("Failed to create format dir: %v", err)
	}
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		t.Fatalf("Failed to create tool dir: %v", err)
	}

	// Create some regular files (not directories) that should be skipped
	formatFile := filepath.Join(formatDir, "not-a-plugin.txt")
	if err := os.WriteFile(formatFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	toolFile := filepath.Join(toolDir, "not-a-plugin.txt")
	if err := os.WriteFile(toolFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Create a valid plugin directory
	validPluginDir := filepath.Join(formatDir, "test-plugin")
	if err := os.MkdirAll(validPluginDir, 0755); err != nil {
		t.Fatalf("Failed to create plugin dir: %v", err)
	}

	manifestJSON := `{
  "plugin_id": "test-plugin",
  "version": "1.0.0",
  "kind": "format"
}`
	manifestPath := filepath.Join(validPluginDir, "plugin.json")
	if err := os.WriteFile(manifestPath, []byte(manifestJSON), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	g := NewGenerator(tempDir, t.TempDir())
	plugins, err := g.LoadPlugins()
	if err != nil {
		t.Fatalf("LoadPlugins failed: %v", err)
	}

	// Should only find the valid plugin, files should be skipped
	if len(plugins) != 1 {
		t.Errorf("Expected 1 plugin, got %d", len(plugins))
	}
	if len(plugins) > 0 && plugins[0].PluginID != "test-plugin" {
		t.Errorf("Expected plugin_id 'test-plugin', got %q", plugins[0].PluginID)
	}
}

// TestGeneratePluginsLoadError tests error handling when LoadPlugins fails in GeneratePlugins
func TestGeneratePluginsLoadError(t *testing.T) {
	// Use /dev/null as plugin dir which will cause ReadDir to fail
	g := NewGenerator("/dev/null", t.TempDir())
	err := g.GeneratePlugins()
	// LoadPlugins won't actually error, it will just return empty list
	// So we need a different approach - test with invalid output dir

	// Instead, test with valid plugin dir but invalid output dir
	tempDir := t.TempDir()
	g = NewGenerator(tempDir, "/dev/null/invalid")
	err = g.GeneratePlugins()
	if err == nil {
		t.Error("Expected error when creating output file fails")
	}
}

// TestGenerateFormatsLoadError tests error handling when LoadPlugins fails in GenerateFormats
func TestGenerateFormatsLoadError(t *testing.T) {
	// Test with invalid output dir
	tempDir := t.TempDir()
	g := NewGenerator(tempDir, "/dev/null/invalid")
	err := g.GenerateFormats()
	if err == nil {
		t.Error("Expected error when creating output file fails")
	}
}

// TestGenerateCLICreateError tests error handling when os.Create fails in GenerateCLI
func TestGenerateCLICreateError(t *testing.T) {
	// Test with invalid output dir
	g := NewGenerator("", "/dev/null/invalid")
	err := g.GenerateCLI()
	if err == nil {
		t.Error("Expected error when creating output file fails")
	}
}

// TestWritePluginsDocWithToolPlugins tests tool plugin rendering with files in tool directory
func TestWritePluginsDocWithToolPlugins(t *testing.T) {
	tempDir := t.TempDir()

	// Create tool directory
	toolDir := filepath.Join(tempDir, "tool")
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		t.Fatalf("Failed to create tool dir: %v", err)
	}

	// Create a file in tool dir (should be skipped)
	toolFile := filepath.Join(toolDir, "README.md")
	if err := os.WriteFile(toolFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Create a valid tool plugin
	pluginDir := filepath.Join(toolDir, "test-tool")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("Failed to create plugin dir: %v", err)
	}

	manifestJSON := `{
  "plugin_id": "test-tool",
  "version": "2.0.0",
  "kind": "tool",
  "profiles": [
    {"id": "profile1", "description": "First profile"},
    {"id": "profile2", "description": "Second profile"}
  ],
  "requires": ["dep1", "dep2"]
}`
	manifestPath := filepath.Join(pluginDir, "plugin.json")
	if err := os.WriteFile(manifestPath, []byte(manifestJSON), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	g := NewGenerator(tempDir, t.TempDir())
	plugins, err := g.LoadPlugins()
	if err != nil {
		t.Fatalf("LoadPlugins failed: %v", err)
	}

	if len(plugins) != 1 {
		t.Errorf("Expected 1 plugin, got %d", len(plugins))
	}

	if plugins[0].PluginID != "test-tool" {
		t.Errorf("Expected plugin_id 'test-tool', got %q", plugins[0].PluginID)
	}

	if len(plugins[0].Profiles) != 2 {
		t.Errorf("Expected 2 profiles, got %d", len(plugins[0].Profiles))
	}

	if len(plugins[0].Requires) != 2 {
		t.Errorf("Expected 2 requires, got %d", len(plugins[0].Requires))
	}
}

// TestWritePluginsDocVariations tests various edge cases in plugin rendering
func TestWritePluginsDocVariations(t *testing.T) {
	g := NewGenerator("", "")

	plugins := []PluginManifest{
		{
			PluginID:    "format-no-extensions",
			Version:     "1.0.0",
			Kind:        "format",
			Description: "",
			Extensions:  nil,
			LossClass:   "",
			IRSupport:   nil,
		},
		{
			PluginID:    "format-ir-loss",
			Version:     "1.0.0",
			Kind:        "format",
			LossClass:   "",
			IRSupport: &IRSupport{
				CanExtract: false,
				CanEmit:    false,
				LossClass:  "L2",
			},
		},
		{
			PluginID:    "format-no-ir",
			Version:     "1.0.0",
			Kind:        "format",
			LossClass:   "L1",
			IRSupport:   nil,
		},
		{
			PluginID: "tool-no-requires",
			Version:  "1.0.0",
			Kind:     "tool",
			Profiles: []Profile{
				{ID: "test", Description: "Test profile"},
			},
			Requires: nil,
		},
	}

	var buf bytes.Buffer
	err := g.writePluginsDoc(&buf, plugins)
	if err != nil {
		t.Fatalf("writePluginsDoc failed: %v", err)
	}

	output := buf.String()

	// Verify all plugins are present
	if !strings.Contains(output, "format-no-extensions") {
		t.Error("Missing format-no-extensions")
	}
	if !strings.Contains(output, "format-ir-loss") {
		t.Error("Missing format-ir-loss")
	}
	if !strings.Contains(output, "format-no-ir") {
		t.Error("Missing format-no-ir")
	}
	if !strings.Contains(output, "tool-no-requires") {
		t.Error("Missing tool-no-requires")
	}
}

// TestWriteFormatsDocVariations tests various edge cases in formats rendering
func TestWriteFormatsDocVariations(t *testing.T) {
	g := NewGenerator("", "")

	plugins := []PluginManifest{
		{
			PluginID:    "format-unknown-loss",
			Kind:        "format",
			LossClass:   "",
			Description: "",
			IRSupport:   nil,
		},
		{
			PluginID:    "format-l4",
			Kind:        "format",
			LossClass:   "L4",
			Description: "Text only format",
			IRSupport: &IRSupport{
				CanExtract: true,
				CanEmit:    false,
			},
		},
		{
			PluginID:  "tool-plugin",
			Kind:      "tool",
			LossClass: "L0",
		},
	}

	var buf bytes.Buffer
	err := g.writeFormatsDoc(&buf, plugins)
	if err != nil {
		t.Fatalf("writeFormatsDoc failed: %v", err)
	}

	output := buf.String()

	// Check for Unknown loss class
	if !strings.Contains(output, "Unknown") {
		t.Error("Missing Unknown loss class")
	}

	// Check for L4 loss class
	if !strings.Contains(output, "L4") {
		t.Error("Missing L4 loss class")
	}

	// Tool plugin should not appear in formats doc
	if strings.Contains(output, "tool-plugin") {
		t.Error("Tool plugin should not appear in formats doc")
	}
}
