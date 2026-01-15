package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

// Note: Tests in this package use LoadFromDirAlways to test plugin discovery
// without depending on the global ExternalPluginsEnabled setting.

// TestParsePluginManifest tests parsing a plugin.json manifest.
func TestParsePluginManifest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a sample plugin.json
	manifestContent := `{
		"plugin_id": "format.zip",
		"version": "1.0.0",
		"kind": "format",
		"entrypoint": "bin/zip-plugin",
		"capabilities": {
			"inputs": ["file"],
			"outputs": ["artifact.kind:zip"]
		}
	}`

	pluginDir := filepath.Join(tempDir, "plugins", "format-zip")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifestPath := filepath.Join(pluginDir, "plugin.json")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	manifest, err := ParsePluginManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	if manifest.PluginID != "format.zip" {
		t.Errorf("expected plugin_id 'format.zip', got %q", manifest.PluginID)
	}

	if manifest.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", manifest.Version)
	}

	if manifest.Kind != "format" {
		t.Errorf("expected kind 'format', got %q", manifest.Kind)
	}
}

// TestDiscoverPlugins tests discovering plugins from a directory.
func TestDiscoverPlugins(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create multiple plugin directories
	plugins := []struct {
		name    string
		content string
	}{
		{
			name:    "format-zip",
			content: `{"plugin_id": "format.zip", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`,
		},
		{
			name:    "format-tar",
			content: `{"plugin_id": "format.tar", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`,
		},
		{
			name:    "tools-libsword",
			content: `{"plugin_id": "tools.libsword", "version": "0.1.0", "kind": "tool", "entrypoint": "bin/plugin"}`,
		},
	}

	pluginsDir := filepath.Join(tempDir, "plugins")
	for _, p := range plugins {
		dir := filepath.Join(pluginsDir, p.name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create %s: %v", p.name, err)
		}
		manifestPath := filepath.Join(dir, "plugin.json")
		if err := os.WriteFile(manifestPath, []byte(p.content), 0644); err != nil {
			t.Fatalf("failed to write manifest for %s: %v", p.name, err)
		}
	}

	// Discover plugins
	discovered, err := DiscoverPlugins(pluginsDir)
	if err != nil {
		t.Fatalf("failed to discover plugins: %v", err)
	}

	if len(discovered) != 3 {
		t.Errorf("expected 3 plugins, got %d", len(discovered))
	}

	// Verify we found specific plugins
	ids := make(map[string]bool)
	for _, p := range discovered {
		ids[p.Manifest.PluginID] = true
	}

	if !ids["format.zip"] {
		t.Error("format.zip plugin not found")
	}
	if !ids["format.tar"] {
		t.Error("format.tar plugin not found")
	}
	if !ids["tools.libsword"] {
		t.Error("tools.libsword plugin not found")
	}
}

// TestLoaderGetPlugin tests getting a plugin by ID.
func TestLoaderGetPlugin(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a plugin
	pluginDir := filepath.Join(tempDir, "plugins", "format-zip")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifestContent := `{"plugin_id": "format.zip", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`
	manifestPath := filepath.Join(pluginDir, "plugin.json")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create loader and load plugins (using LoadFromDirAlways to test discovery
	// regardless of ExternalPluginsEnabled setting)
	loader := NewLoader()
	if err := loader.LoadFromDirAlways(filepath.Join(tempDir, "plugins")); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Get plugin by ID
	plugin, err := loader.GetPlugin("format.zip")
	if err != nil {
		t.Fatalf("failed to get plugin: %v", err)
	}

	if plugin.Manifest.PluginID != "format.zip" {
		t.Errorf("expected plugin_id 'format.zip', got %q", plugin.Manifest.PluginID)
	}

	// Try to get non-existent plugin
	_, err = loader.GetPlugin("nonexistent")
	if err == nil {
		t.Error("expected error when getting nonexistent plugin")
	}
}

// TestPluginKindFiltering tests filtering plugins by kind.
func TestPluginKindFiltering(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create format and tool plugins
	plugins := []struct {
		name    string
		content string
	}{
		{
			name:    "format-zip",
			content: `{"plugin_id": "format.zip", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`,
		},
		{
			name:    "format-tar",
			content: `{"plugin_id": "format.tar", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`,
		},
		{
			name:    "tools-libsword",
			content: `{"plugin_id": "tools.libsword", "version": "0.1.0", "kind": "tool", "entrypoint": "bin/plugin"}`,
		},
	}

	pluginsDir := filepath.Join(tempDir, "plugins")
	for _, p := range plugins {
		dir := filepath.Join(pluginsDir, p.name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create %s: %v", p.name, err)
		}
		manifestPath := filepath.Join(dir, "plugin.json")
		if err := os.WriteFile(manifestPath, []byte(p.content), 0644); err != nil {
			t.Fatalf("failed to write manifest for %s: %v", p.name, err)
		}
	}

	loader := NewLoader()
	if err := loader.LoadFromDirAlways(pluginsDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Get format plugins
	formatPlugins := loader.GetPluginsByKind("format")
	if len(formatPlugins) != 2 {
		t.Errorf("expected 2 format plugins, got %d", len(formatPlugins))
	}

	// Get tool plugins
	toolPlugins := loader.GetPluginsByKind("tool")
	if len(toolPlugins) != 1 {
		t.Errorf("expected 1 tool plugin, got %d", len(toolPlugins))
	}
}

// TestPluginManifestWithIRSupport tests parsing a plugin manifest with IR support.
func TestPluginManifestWithIRSupport(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-ir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a plugin.json with IR support
	manifestContent := `{
		"plugin_id": "format.osis",
		"version": "1.0.0",
		"kind": "format",
		"entrypoint": "bin/osis-plugin",
		"capabilities": {
			"inputs": ["file"],
			"outputs": ["artifact.kind:osis"]
		},
		"ir_support": {
			"can_extract": true,
			"can_emit": true,
			"loss_class": "L0",
			"formats": ["OSIS"]
		}
	}`

	pluginDir := filepath.Join(tempDir, "format-osis")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifestPath := filepath.Join(pluginDir, "plugin.json")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	manifest, err := ParsePluginManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	// Verify IR support was parsed
	if manifest.IRSupport == nil {
		t.Fatal("IRSupport is nil")
	}
	if !manifest.IRSupport.CanExtract {
		t.Error("expected CanExtract to be true")
	}
	if !manifest.IRSupport.CanEmit {
		t.Error("expected CanEmit to be true")
	}
	if manifest.IRSupport.LossClass != "L0" {
		t.Errorf("LossClass = %q, want L0", manifest.IRSupport.LossClass)
	}
	if len(manifest.IRSupport.Formats) != 1 || manifest.IRSupport.Formats[0] != "OSIS" {
		t.Errorf("Formats = %v, want [OSIS]", manifest.IRSupport.Formats)
	}
}

// TestPluginSupportsIR tests the SupportsIR method.
func TestPluginSupportsIR(t *testing.T) {
	// Plugin without IR support
	pluginNoIR := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "format.zip",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "bin/plugin",
		},
	}
	if pluginNoIR.SupportsIR() {
		t.Error("expected SupportsIR() to be false for plugin without IR support")
	}

	// Plugin with IR support
	pluginWithIR := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "format.osis",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "bin/plugin",
			IRSupport: &IRCapabilities{
				CanExtract: true,
				CanEmit:    true,
				LossClass:  "L0",
			},
		},
	}
	if !pluginWithIR.SupportsIR() {
		t.Error("expected SupportsIR() to be true for plugin with IR support")
	}
}

// TestPluginCanExtractIR tests the CanExtractIR method.
func TestPluginCanExtractIR(t *testing.T) {
	plugin := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "format.osis",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "bin/plugin",
			IRSupport: &IRCapabilities{
				CanExtract: true,
				CanEmit:    false,
				LossClass:  "L0",
			},
		},
	}

	if !plugin.CanExtractIR() {
		t.Error("expected CanExtractIR() to be true")
	}
	if plugin.CanEmitIR() {
		t.Error("expected CanEmitIR() to be false")
	}
}

// TestLoaderGetIRCapablePlugins tests getting plugins that support IR.
func TestLoaderGetIRCapablePlugins(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-ir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create plugins, one with IR support
	plugins := []struct {
		name    string
		content string
	}{
		{
			name:    "format-zip",
			content: `{"plugin_id": "format.zip", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`,
		},
		{
			name: "format-osis",
			content: `{
				"plugin_id": "format.osis",
				"version": "1.0.0",
				"kind": "format",
				"entrypoint": "bin/plugin",
				"ir_support": {"can_extract": true, "can_emit": true, "loss_class": "L0"}
			}`,
		},
		{
			name: "format-usfm",
			content: `{
				"plugin_id": "format.usfm",
				"version": "1.0.0",
				"kind": "format",
				"entrypoint": "bin/plugin",
				"ir_support": {"can_extract": true, "can_emit": true, "loss_class": "L1"}
			}`,
		},
	}

	pluginsDir := filepath.Join(tempDir, "plugins")
	for _, p := range plugins {
		dir := filepath.Join(pluginsDir, p.name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create %s: %v", p.name, err)
		}
		manifestPath := filepath.Join(dir, "plugin.json")
		if err := os.WriteFile(manifestPath, []byte(p.content), 0644); err != nil {
			t.Fatalf("failed to write manifest for %s: %v", p.name, err)
		}
	}

	loader := NewLoader()
	if err := loader.LoadFromDirAlways(pluginsDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Get IR-capable plugins
	irPlugins := loader.GetIRCapablePlugins()
	if len(irPlugins) != 2 {
		t.Errorf("expected 2 IR-capable plugins, got %d", len(irPlugins))
	}

	// Verify specific plugins
	ids := make(map[string]bool)
	for _, p := range irPlugins {
		ids[p.Manifest.PluginID] = true
	}

	if !ids["format.osis"] {
		t.Error("format.osis plugin not in IR-capable list")
	}
	if !ids["format.usfm"] {
		t.Error("format.usfm plugin not in IR-capable list")
	}
	if ids["format.zip"] {
		t.Error("format.zip plugin should not be in IR-capable list")
	}
}

// TestNestedPluginDiscovery tests loading plugins from nested directory structure.
func TestNestedPluginDiscovery(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nested-plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create nested structure: plugins/format/osis, plugins/tool/libsword
	formatDir := filepath.Join(tempDir, "plugins", "format")
	toolDir := filepath.Join(tempDir, "plugins", "tool")

	// Create format plugins in nested structure
	for _, name := range []string{"osis", "usfm"} {
		dir := filepath.Join(formatDir, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
		content := `{"plugin_id": "format.` + name + `", "version": "1.0.0", "kind": "format", "entrypoint": "format-` + name + `"}`
		if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write manifest: %v", err)
		}
	}

	// Create tool plugin in nested structure
	libswordDir := filepath.Join(toolDir, "libsword")
	if err := os.MkdirAll(libswordDir, 0755); err != nil {
		t.Fatalf("failed to create libsword dir: %v", err)
	}
	content := `{"plugin_id": "tool.libsword", "version": "1.0.0", "kind": "tool", "entrypoint": "tool-libsword"}`
	if err := os.WriteFile(filepath.Join(libswordDir, "plugin.json"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Load plugins
	loader := NewLoader()
	pluginsDir := filepath.Join(tempDir, "plugins")
	if err := loader.LoadFromDirAlways(pluginsDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Verify format plugins loaded
	formatPlugins := loader.GetPluginsByKind("format")
	if len(formatPlugins) != 2 {
		t.Errorf("expected 2 format plugins, got %d", len(formatPlugins))
	}

	// Verify tool plugins loaded
	toolPlugins := loader.GetPluginsByKind("tool")
	if len(toolPlugins) != 1 {
		t.Errorf("expected 1 tool plugin, got %d", len(toolPlugins))
	}

	// Verify specific plugins are accessible
	plugin, err := loader.GetPlugin("format.osis")
	if err != nil {
		t.Errorf("failed to get format.osis: %v", err)
	} else if plugin.Manifest.PluginID != "format.osis" {
		t.Errorf("expected format.osis, got %s", plugin.Manifest.PluginID)
	}

	plugin, err = loader.GetPlugin("tool.libsword")
	if err != nil {
		t.Errorf("failed to get tool.libsword: %v", err)
	} else if plugin.Manifest.PluginID != "tool.libsword" {
		t.Errorf("expected tool.libsword, got %s", plugin.Manifest.PluginID)
	}
}

// TestExamplePluginKind tests that example is a valid plugin kind.
//
// ADDING A NEW PLUGIN KIND TEST:
// When adding a new plugin kind, copy this test and modify:
//
//  1. Rename the function to Test<Kind>PluginKind (e.g., TestMyKindPluginKind)
//  2. Change the temp dir prefix (e.g., "mykind-plugin-test-*")
//  3. Change the directory path (e.g., "plugins", "mykind", "test-plugin")
//  4. Update the plugin.json content:
//     - plugin_id: "<kind>.test" (e.g., "mykind.test")
//     - kind: "<kind>" (e.g., "mykind")
//     - entrypoint: "<kind>-test" (e.g., "mykind-test")
//  5. Update GetPluginsByKind call to use your kind
//  6. Update GetPlugin call to use your plugin_id
//  7. Add a check for your Is<Kind>() helper method
//
// This test verifies:
// - The kind is recognized by isKindDirectory()
// - Plugins are discovered in the nested structure
// - GetPluginsByKind returns the correct plugins
// - GetPlugin can retrieve by plugin_id
// - The Is<Kind>() helper returns true
func TestExamplePluginKind(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "example-plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create example plugin in nested structure: plugins/example/test-plugin/
	exampleDir := filepath.Join(tempDir, "plugins", "example", "test-plugin")
	if err := os.MkdirAll(exampleDir, 0755); err != nil {
		t.Fatalf("failed to create example dir: %v", err)
	}

	// Create plugin.json with kind="example"
	content := `{"plugin_id": "example.test", "version": "1.0.0", "kind": "example", "entrypoint": "example-test"}`
	if err := os.WriteFile(filepath.Join(exampleDir, "plugin.json"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Load plugins from the plugins directory
	loader := NewLoader()
	pluginsDir := filepath.Join(tempDir, "plugins")
	if err := loader.LoadFromDirAlways(pluginsDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Verify example plugin loaded via GetPluginsByKind
	examplePlugins := loader.GetPluginsByKind("example")
	if len(examplePlugins) != 1 {
		t.Errorf("expected 1 example plugin, got %d", len(examplePlugins))
	}

	// Verify plugin is accessible via GetPlugin
	plugin, err := loader.GetPlugin("example.test")
	if err != nil {
		t.Errorf("failed to get example.test: %v", err)
	} else {
		// Verify plugin_id matches
		if plugin.Manifest.PluginID != "example.test" {
			t.Errorf("expected example.test, got %s", plugin.Manifest.PluginID)
		}
		// Verify kind matches
		if plugin.Manifest.Kind != "example" {
			t.Errorf("expected kind 'example', got %s", plugin.Manifest.Kind)
		}
		// Verify IsExample() helper returns true
		if !plugin.IsExample() {
			t.Error("expected IsExample() to return true")
		}
		// Verify other Is*() helpers return false
		if plugin.IsFormat() {
			t.Error("expected IsFormat() to return false")
		}
		if plugin.IsTool() {
			t.Error("expected IsTool() to return false")
		}
	}
}

// TestJuniperPluginKind tests that juniper is a valid plugin kind.
func TestJuniperPluginKind(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "juniper-plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create juniper plugin in nested structure
	juniperDir := filepath.Join(tempDir, "plugins", "juniper", "test-plugin")
	if err := os.MkdirAll(juniperDir, 0755); err != nil {
		t.Fatalf("failed to create juniper dir: %v", err)
	}
	content := `{"plugin_id": "juniper.test", "version": "1.0.0", "kind": "juniper", "entrypoint": "juniper-test"}`
	if err := os.WriteFile(filepath.Join(juniperDir, "plugin.json"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Load plugins
	loader := NewLoader()
	pluginsDir := filepath.Join(tempDir, "plugins")
	if err := loader.LoadFromDirAlways(pluginsDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Verify juniper plugin loaded
	juniperPlugins := loader.GetPluginsByKind("juniper")
	if len(juniperPlugins) != 1 {
		t.Errorf("expected 1 juniper plugin, got %d", len(juniperPlugins))
	}

	// Verify specific plugin is accessible
	plugin, err := loader.GetPlugin("juniper.test")
	if err != nil {
		t.Errorf("failed to get juniper.test: %v", err)
	} else if plugin.Manifest.PluginID != "juniper.test" {
		t.Errorf("expected juniper.test, got %s", plugin.Manifest.PluginID)
	}

	// Verify IsJuniper helper
	if plugin.Manifest.Kind != "juniper" {
		t.Errorf("expected kind 'juniper', got %s", plugin.Manifest.Kind)
	}
	if !plugin.IsJuniper() {
		t.Error("expected IsJuniper() to return true")
	}
	if plugin.IsFormat() {
		t.Error("expected IsFormat() to return false")
	}
	if plugin.IsTool() {
		t.Error("expected IsTool() to return false")
	}
}

// TestListPlugins tests listing all loaded plugins.
func TestListPlugins(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-list-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create multiple plugins
	plugins := []struct {
		name    string
		content string
	}{
		{
			name:    "format-zip",
			content: `{"plugin_id": "format.zip", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`,
		},
		{
			name:    "format-tar",
			content: `{"plugin_id": "format.tar", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`,
		},
		{
			name:    "tools-libsword",
			content: `{"plugin_id": "tools.libsword", "version": "0.1.0", "kind": "tool", "entrypoint": "bin/plugin"}`,
		},
	}

	pluginsDir := filepath.Join(tempDir, "plugins")
	for _, p := range plugins {
		dir := filepath.Join(pluginsDir, p.name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create %s: %v", p.name, err)
		}
		manifestPath := filepath.Join(dir, "plugin.json")
		if err := os.WriteFile(manifestPath, []byte(p.content), 0644); err != nil {
			t.Fatalf("failed to write manifest for %s: %v", p.name, err)
		}
	}

	loader := NewLoader()
	if err := loader.LoadFromDirAlways(pluginsDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	// Test ListPlugins
	allPlugins := loader.ListPlugins()
	if len(allPlugins) != 3 {
		t.Errorf("expected 3 plugins, got %d", len(allPlugins))
	}

	// Verify all plugins are present
	ids := make(map[string]bool)
	for _, p := range allPlugins {
		ids[p.Manifest.PluginID] = true
	}

	if !ids["format.zip"] {
		t.Error("format.zip plugin not found in list")
	}
	if !ids["format.tar"] {
		t.Error("format.tar plugin not found in list")
	}
	if !ids["tools.libsword"] {
		t.Error("tools.libsword plugin not found in list")
	}
}

// TestParsePluginManifestMissingFields tests validation of required fields.
func TestParsePluginManifestMissingFields(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-validation-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name     string
		content  string
		errField string
	}{
		{
			name:     "missing plugin_id",
			content:  `{"version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`,
			errField: "plugin_id",
		},
		{
			name:     "missing version",
			content:  `{"plugin_id": "test", "kind": "format", "entrypoint": "bin/plugin"}`,
			errField: "version",
		},
		{
			name:     "missing kind",
			content:  `{"plugin_id": "test", "version": "1.0.0", "entrypoint": "bin/plugin"}`,
			errField: "kind",
		},
		{
			name:     "missing entrypoint",
			content:  `{"plugin_id": "test", "version": "1.0.0", "kind": "format"}`,
			errField: "entrypoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifestPath := filepath.Join(tempDir, "plugin.json")
			if err := os.WriteFile(manifestPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write manifest: %v", err)
			}

			_, err := ParsePluginManifest(manifestPath)
			if err == nil {
				t.Errorf("expected error for missing %s", tt.errField)
			}
		})
	}
}

// TestParsePluginManifestInvalidJSON tests handling of invalid JSON.
func TestParsePluginManifestInvalidJSON(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-invalid-json-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manifestPath := filepath.Join(tempDir, "plugin.json")
	if err := os.WriteFile(manifestPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	_, err = ParsePluginManifest(manifestPath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestParsePluginManifestNonExistent tests handling of non-existent file.
func TestParsePluginManifestNonExistent(t *testing.T) {
	_, err := ParsePluginManifest("/nonexistent/path/plugin.json")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

// TestDiscoverPluginsNonExistentDir tests discovering plugins in non-existent directory.
func TestDiscoverPluginsNonExistentDir(t *testing.T) {
	plugins, err := DiscoverPlugins("/nonexistent/directory")
	if err != nil {
		t.Errorf("expected no error for non-existent directory, got %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("expected empty list for non-existent directory, got %d plugins", len(plugins))
	}
}

// TestDiscoverPluginsWithInvalidPlugins tests that invalid plugins are skipped.
func TestDiscoverPluginsWithInvalidPlugins(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-invalid-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginsDir := filepath.Join(tempDir, "plugins")

	// Create a valid plugin
	validDir := filepath.Join(pluginsDir, "format-valid")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("failed to create valid dir: %v", err)
	}
	validContent := `{"plugin_id": "format.valid", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`
	if err := os.WriteFile(filepath.Join(validDir, "plugin.json"), []byte(validContent), 0644); err != nil {
		t.Fatalf("failed to write valid manifest: %v", err)
	}

	// Create an invalid plugin (missing required field)
	invalidDir := filepath.Join(pluginsDir, "format-invalid")
	if err := os.MkdirAll(invalidDir, 0755); err != nil {
		t.Fatalf("failed to create invalid dir: %v", err)
	}
	invalidContent := `{"version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`
	if err := os.WriteFile(filepath.Join(invalidDir, "plugin.json"), []byte(invalidContent), 0644); err != nil {
		t.Fatalf("failed to write invalid manifest: %v", err)
	}

	// Create a directory without plugin.json
	noManifestDir := filepath.Join(pluginsDir, "format-nomanifest")
	if err := os.MkdirAll(noManifestDir, 0755); err != nil {
		t.Fatalf("failed to create no-manifest dir: %v", err)
	}

	// Discover should succeed but only return valid plugin
	plugins, err := DiscoverPlugins(pluginsDir)
	if err != nil {
		t.Fatalf("DiscoverPlugins failed: %v", err)
	}

	if len(plugins) != 1 {
		t.Errorf("expected 1 valid plugin, got %d", len(plugins))
	}

	if len(plugins) > 0 && plugins[0].Manifest.PluginID != "format.valid" {
		t.Errorf("expected valid plugin to be format.valid, got %s", plugins[0].Manifest.PluginID)
	}
}

// TestLoadFromDirAlwaysError tests LoadFromDirAlways error handling.
func TestLoadFromDirAlwaysError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-loaddir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file instead of a directory to cause read error
	filePath := filepath.Join(tempDir, "notadir")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	loader := NewLoader()
	err = loader.LoadFromDirAlways(filePath)
	if err == nil {
		t.Error("expected error when loading from file instead of directory")
	}
}

// TestDiscoverPluginsWithNestedInvalid tests nested structure with invalid plugins.
func TestDiscoverPluginsWithNestedInvalid(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-nested-invalid-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginsDir := filepath.Join(tempDir, "plugins")
	formatDir := filepath.Join(pluginsDir, "format")

	// Create valid plugin in nested structure
	validDir := filepath.Join(formatDir, "valid")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("failed to create valid dir: %v", err)
	}
	validContent := `{"plugin_id": "format.valid", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`
	if err := os.WriteFile(filepath.Join(validDir, "plugin.json"), []byte(validContent), 0644); err != nil {
		t.Fatalf("failed to write valid manifest: %v", err)
	}

	// Create invalid plugin in nested structure
	invalidDir := filepath.Join(formatDir, "invalid")
	if err := os.MkdirAll(invalidDir, 0755); err != nil {
		t.Fatalf("failed to create invalid dir: %v", err)
	}
	invalidContent := `{"plugin_id": "", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`
	if err := os.WriteFile(filepath.Join(invalidDir, "plugin.json"), []byte(invalidContent), 0644); err != nil {
		t.Fatalf("failed to write invalid manifest: %v", err)
	}

	// Discover should succeed and only return valid plugin
	plugins, err := DiscoverPlugins(pluginsDir)
	if err != nil {
		t.Fatalf("DiscoverPlugins failed: %v", err)
	}

	if len(plugins) != 1 {
		t.Errorf("expected 1 valid plugin, got %d", len(plugins))
	}
}

// TestIsKindDirectory tests the isKindDirectory function.
func TestIsKindDirectory(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"format", true},
		{"tool", true},
		{"juniper", true},
		{"example", true},
		{"unknown", false},
		{"random", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isKindDirectory(tt.name)
			if result != tt.expected {
				t.Errorf("isKindDirectory(%q) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

// TestLoadPluginFromDirNoManifest tests loadPluginFromDir with missing manifest.
func TestLoadPluginFromDirNoManifest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-noload-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginDir := filepath.Join(tempDir, "plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	_, err = loadPluginFromDir(pluginDir)
	if err == nil {
		t.Error("expected error for missing plugin.json")
	}
}

// TestDiscoverPluginsWithFiles tests that non-directory entries are skipped.
func TestDiscoverPluginsWithFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-files-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginsDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}

	// Create a valid plugin
	validDir := filepath.Join(pluginsDir, "format-valid")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("failed to create valid dir: %v", err)
	}
	validContent := `{"plugin_id": "format.valid", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`
	if err := os.WriteFile(filepath.Join(validDir, "plugin.json"), []byte(validContent), 0644); err != nil {
		t.Fatalf("failed to write valid manifest: %v", err)
	}

	// Create a file (not directory) in plugins dir
	if err := os.WriteFile(filepath.Join(pluginsDir, "somefile.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Discover should skip the file and only find the valid plugin
	plugins, err := DiscoverPlugins(pluginsDir)
	if err != nil {
		t.Fatalf("DiscoverPlugins failed: %v", err)
	}

	if len(plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(plugins))
	}
}

// TestDiscoverPluginsInKindDirWithFiles tests that files in kind dirs are skipped.
func TestDiscoverPluginsInKindDirWithFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-kindfiles-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginsDir := filepath.Join(tempDir, "plugins")
	formatDir := filepath.Join(pluginsDir, "format")
	if err := os.MkdirAll(formatDir, 0755); err != nil {
		t.Fatalf("failed to create format dir: %v", err)
	}

	// Create a valid plugin
	validDir := filepath.Join(formatDir, "valid")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("failed to create valid dir: %v", err)
	}
	validContent := `{"plugin_id": "format.valid", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`
	if err := os.WriteFile(filepath.Join(validDir, "plugin.json"), []byte(validContent), 0644); err != nil {
		t.Fatalf("failed to write valid manifest: %v", err)
	}

	// Create a file (not directory) in format dir
	if err := os.WriteFile(filepath.Join(formatDir, "README.md"), []byte("readme"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Discover should skip the file and only find the valid plugin
	plugins, err := DiscoverPlugins(pluginsDir)
	if err != nil {
		t.Fatalf("DiscoverPlugins failed: %v", err)
	}

	if len(plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(plugins))
	}
}

// TestAddPlugin tests adding a plugin directly to the loader.
func TestAddPlugin(t *testing.T) {
	loader := NewLoader()

	// Add a plugin directly
	plugin := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "test.plugin",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "bin/plugin",
		},
		Path: "/some/path",
	}

	loader.AddPlugin(plugin)

	// Verify plugin was added
	retrieved, err := loader.GetPlugin("test.plugin")
	if err != nil {
		t.Fatalf("failed to retrieve added plugin: %v", err)
	}

	if retrieved.Manifest.PluginID != "test.plugin" {
		t.Errorf("expected test.plugin, got %s", retrieved.Manifest.PluginID)
	}
}

// TestLoadFromDirAlwaysIncompatiblePlugins tests that incompatible plugins are skipped with a warning.
func TestLoadFromDirAlwaysIncompatiblePlugins(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-incompat-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginsDir := filepath.Join(tempDir, "plugins")

	// Create a plugin with a very high minimum host version requirement
	// This should fail compatibility check
	incompatDir := filepath.Join(pluginsDir, "format-incompat")
	if err := os.MkdirAll(incompatDir, 0755); err != nil {
		t.Fatalf("failed to create incompat dir: %v", err)
	}
	// Require host version 99.0.0 which will fail compatibility
	incompatContent := `{"plugin_id": "format.incompat", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin", "min_host_version": "99.0.0"}`
	if err := os.WriteFile(filepath.Join(incompatDir, "plugin.json"), []byte(incompatContent), 0644); err != nil {
		t.Fatalf("failed to write incompat manifest: %v", err)
	}

	// Create a compatible plugin
	compatDir := filepath.Join(pluginsDir, "format-compat")
	if err := os.MkdirAll(compatDir, 0755); err != nil {
		t.Fatalf("failed to create compat dir: %v", err)
	}
	compatContent := `{"plugin_id": "format.compat", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`
	if err := os.WriteFile(filepath.Join(compatDir, "plugin.json"), []byte(compatContent), 0644); err != nil {
		t.Fatalf("failed to write compat manifest: %v", err)
	}

	loader := NewLoader()
	err = loader.LoadFromDirAlways(pluginsDir)
	if err != nil {
		t.Fatalf("LoadFromDirAlways failed: %v", err)
	}

	// Should only have the compatible plugin
	plugins := loader.ListPlugins()
	if len(plugins) != 1 {
		t.Errorf("expected 1 plugin (compatible only), got %d", len(plugins))
	}

	// Verify only the compatible plugin was loaded
	_, err = loader.GetPlugin("format.compat")
	if err != nil {
		t.Error("expected format.compat to be loaded")
	}

	_, err = loader.GetPlugin("format.incompat")
	if err == nil {
		t.Error("expected format.incompat to NOT be loaded due to incompatibility")
	}
}

// TestDiscoverPluginsKindDirReadError tests error handling when kind dir can't be read.
func TestDiscoverPluginsKindDirReadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-kinderr-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginsDir := filepath.Join(tempDir, "plugins")
	formatDir := filepath.Join(pluginsDir, "format")

	// Create a valid plugin first
	validDir := filepath.Join(pluginsDir, "valid-plugin")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("failed to create valid dir: %v", err)
	}
	validContent := `{"plugin_id": "test.valid", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`
	if err := os.WriteFile(filepath.Join(validDir, "plugin.json"), []byte(validContent), 0644); err != nil {
		t.Fatalf("failed to write valid manifest: %v", err)
	}

	// Create format directory as a file instead of directory to cause read error
	if err := os.WriteFile(formatDir, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write format file: %v", err)
	}

	// Should skip the unreadable kind dir but return the valid plugin
	plugins, err := DiscoverPlugins(pluginsDir)
	if err != nil {
		t.Fatalf("DiscoverPlugins failed: %v", err)
	}

	// Should have found the valid plugin despite the error in format dir
	if len(plugins) != 1 {
		t.Errorf("expected 1 plugin (valid), got %d", len(plugins))
	}
}

// TestLoadFromDirExternalDisabled tests LoadFromDir when external plugins are disabled.
func TestLoadFromDirExternalDisabled(t *testing.T) {
	// Save original state
	originalState := ExternalPluginsEnabled()
	defer func() {
		if originalState {
			EnableExternalPlugins()
		} else {
			DisableExternalPlugins()
		}
	}()

	// Disable external plugins
	DisableExternalPlugins()

	tempDir, err := os.MkdirTemp("", "plugin-disabled-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginsDir := filepath.Join(tempDir, "plugins")
	pluginDir := filepath.Join(pluginsDir, "format-test")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}
	content := `{"plugin_id": "format.test", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	loader := NewLoader()

	// LoadFromDir should return early when external plugins are disabled
	err = loader.LoadFromDir(pluginsDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	// Plugin should NOT be loaded because external plugins are disabled
	_, err = loader.GetPlugin("format.test")
	if err == nil {
		t.Error("expected plugin to NOT be loaded when external plugins are disabled")
	}
}

// TestLoadFromDirExternalEnabled tests LoadFromDir when external plugins are enabled.
func TestLoadFromDirExternalEnabled(t *testing.T) {
	// Save original state
	originalState := ExternalPluginsEnabled()
	defer func() {
		if originalState {
			EnableExternalPlugins()
		} else {
			DisableExternalPlugins()
		}
	}()

	// Enable external plugins
	EnableExternalPlugins()

	tempDir, err := os.MkdirTemp("", "plugin-enabled-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginsDir := filepath.Join(tempDir, "plugins")
	pluginDir := filepath.Join(pluginsDir, "format-test")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}
	content := `{"plugin_id": "format.enabled", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	loader := NewLoader()

	// LoadFromDir should load plugins when external plugins are enabled
	err = loader.LoadFromDir(pluginsDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	// Plugin should be loaded
	plugin, err := loader.GetPlugin("format.enabled")
	if err != nil {
		t.Errorf("expected plugin to be loaded: %v", err)
	}
	if plugin != nil && plugin.Manifest.PluginID != "format.enabled" {
		t.Errorf("expected plugin ID 'format.enabled', got '%s'", plugin.Manifest.PluginID)
	}
}

// TestLoadFromDirNonExistent tests LoadFromDir with a non-existent directory.
// DiscoverPlugins returns empty list (not error) for non-existent directories.
func TestLoadFromDirNonExistent(t *testing.T) {
	// Save original state
	originalState := ExternalPluginsEnabled()
	defer func() {
		if originalState {
			EnableExternalPlugins()
		} else {
			DisableExternalPlugins()
		}
	}()

	// Enable external plugins
	EnableExternalPlugins()

	loader := NewLoader()

	// LoadFromDir with non-existent directory should return nil (empty list behavior)
	err := loader.LoadFromDir("/nonexistent/path/plugins")
	if err != nil {
		t.Errorf("expected no error for non-existent directory, got: %v", err)
	}

	// Verify no plugins were loaded (except embedded ones)
	all := loader.ListPlugins()
	for _, p := range all {
		if p.Path != "" && p.Manifest.PluginID == "nonexistent" {
			t.Error("expected no plugins from non-existent directory")
		}
	}
}

// TestNewLoaderWithEmbeddedPlugins tests that NewLoader includes embedded plugins.
func TestNewLoaderWithEmbeddedPlugins(t *testing.T) {
	// Clear and register test embedded plugin
	ClearEmbeddedRegistry()
	defer ClearEmbeddedRegistry()

	RegisterEmbeddedPlugin(&EmbeddedPlugin{
		Manifest: &PluginManifest{
			PluginID: "test.embedded.loader",
			Version:  "1.0.0",
			Kind:     "format",
		},
	})

	// Create loader - should include embedded plugins
	loader := NewLoader()

	// Get embedded plugin from loader
	plugin, err := loader.GetPlugin("test.embedded.loader")
	if err != nil {
		t.Errorf("expected embedded plugin to be in loader: %v", err)
	}
	if plugin != nil && plugin.Manifest.PluginID != "test.embedded.loader" {
		t.Errorf("expected plugin ID 'test.embedded.loader', got '%s'", plugin.Manifest.PluginID)
	}
}

// TestLoadFromDirIncompatiblePlugins tests LoadFromDir with incompatible plugins when external is enabled.
func TestLoadFromDirIncompatiblePlugins(t *testing.T) {
	// Save original state
	originalState := ExternalPluginsEnabled()
	defer func() {
		if originalState {
			EnableExternalPlugins()
		} else {
			DisableExternalPlugins()
		}
	}()

	// Enable external plugins
	EnableExternalPlugins()

	tempDir, err := os.MkdirTemp("", "plugin-incompatible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginsDir := filepath.Join(tempDir, "plugins")

	// Create a plugin with very high minimum host version
	incompatDir := filepath.Join(pluginsDir, "format-incompat")
	if err := os.MkdirAll(incompatDir, 0755); err != nil {
		t.Fatalf("failed to create incompat dir: %v", err)
	}
	incompatContent := `{"plugin_id": "format.incompat2", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin", "min_host_version": "99.0.0"}`
	if err := os.WriteFile(filepath.Join(incompatDir, "plugin.json"), []byte(incompatContent), 0644); err != nil {
		t.Fatalf("failed to write incompat manifest: %v", err)
	}

	loader := NewLoader()
	err = loader.LoadFromDir(pluginsDir)
	if err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	// Incompatible plugin should NOT be loaded
	_, err = loader.GetPlugin("format.incompat2")
	if err == nil {
		t.Error("expected incompatible plugin to NOT be loaded")
	}
}

// TestDiscoverPluginsAbsolutePathConversion tests that DiscoverPlugins converts to absolute paths.
func TestDiscoverPluginsAbsolutePathConversion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-abs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginsDir := filepath.Join(tempDir, "plugins")
	pluginDir := filepath.Join(pluginsDir, "format-test")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}
	content := `{"plugin_id": "format.abs-test", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Use a relative path (if we're in a different directory)
	// Save current directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Change to temp directory and use relative path
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	plugins, err := DiscoverPlugins("./plugins")
	if err != nil {
		t.Fatalf("DiscoverPlugins failed: %v", err)
	}

	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	// Verify path was converted to absolute
	if !filepath.IsAbs(plugins[0].Path) {
		t.Errorf("expected absolute path, got %s", plugins[0].Path)
	}
}

// TestDiscoverPluginsReadDirError tests DiscoverPlugins with I/O error on ReadDir.
func TestDiscoverPluginsReadDirError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-readerror-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file instead of a directory
	filePath := filepath.Join(tempDir, "notadirectory")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Try to discover plugins from a file (not a directory)
	_, err = DiscoverPlugins(filePath)
	if err == nil {
		t.Error("expected error when discovering from a file instead of directory")
	}
}

// TestLoadFromDirDiscoverError tests LoadFromDir when DiscoverPlugins returns an error.
func TestLoadFromDirDiscoverError(t *testing.T) {
	// Save original state
	originalState := ExternalPluginsEnabled()
	defer func() {
		if originalState {
			EnableExternalPlugins()
		} else {
			DisableExternalPlugins()
		}
	}()

	// Enable external plugins
	EnableExternalPlugins()

	tempDir, err := os.MkdirTemp("", "plugin-discover-error-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file instead of directory to cause error
	filePath := filepath.Join(tempDir, "notadirectory")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	loader := NewLoader()
	err = loader.LoadFromDir(filePath)
	if err == nil {
		t.Error("expected error from LoadFromDir when DiscoverPlugins fails")
	}
}

// TestDiscoverPluginsMixedValidInvalid tests discovering plugins with a mix of valid and invalid plugins.
func TestDiscoverPluginsMixedValidInvalid(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-mixed-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginsDir := filepath.Join(tempDir, "plugins")

	// Create a valid flat-structure plugin
	validFlatDir := filepath.Join(pluginsDir, "format-valid-flat")
	if err := os.MkdirAll(validFlatDir, 0755); err != nil {
		t.Fatalf("failed to create valid flat dir: %v", err)
	}
	validContent := `{"plugin_id": "format.valid-flat", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`
	if err := os.WriteFile(filepath.Join(validFlatDir, "plugin.json"), []byte(validContent), 0644); err != nil {
		t.Fatalf("failed to write valid manifest: %v", err)
	}

	// Create a valid nested plugin
	formatDir := filepath.Join(pluginsDir, "format")
	validNestedDir := filepath.Join(formatDir, "valid-nested")
	if err := os.MkdirAll(validNestedDir, 0755); err != nil {
		t.Fatalf("failed to create valid nested dir: %v", err)
	}
	validNestedContent := `{"plugin_id": "format.valid-nested", "version": "1.0.0", "kind": "format", "entrypoint": "bin/plugin"}`
	if err := os.WriteFile(filepath.Join(validNestedDir, "plugin.json"), []byte(validNestedContent), 0644); err != nil {
		t.Fatalf("failed to write valid nested manifest: %v", err)
	}

	// Create an invalid nested plugin (no plugin.json)
	invalidNestedDir := filepath.Join(formatDir, "invalid-nested")
	if err := os.MkdirAll(invalidNestedDir, 0755); err != nil {
		t.Fatalf("failed to create invalid nested dir: %v", err)
	}

	// Discover should return only valid plugins
	plugins, err := DiscoverPlugins(pluginsDir)
	if err != nil {
		t.Fatalf("DiscoverPlugins failed: %v", err)
	}

	if len(plugins) != 2 {
		t.Errorf("expected 2 valid plugins, got %d", len(plugins))
	}

	// Verify plugin IDs
	ids := make(map[string]bool)
	for _, p := range plugins {
		ids[p.Manifest.PluginID] = true
	}

	if !ids["format.valid-flat"] {
		t.Error("format.valid-flat plugin not found")
	}
	if !ids["format.valid-nested"] {
		t.Error("format.valid-nested plugin not found")
	}
}
