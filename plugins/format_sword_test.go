package plugins_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func init() {
	// Enable external plugins for testing
	plugins.EnableExternalPlugins()
}

// TestFormatSwordDetect tests the format.sword plugin detect command.
func TestFormatSwordDetect(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create SWORD module structure
	modsD := filepath.Join(tempDir, "mods.d")
	modulesDir := filepath.Join(tempDir, "modules", "texts", "ztext", "kjv")
	os.MkdirAll(modsD, 0755)
	os.MkdirAll(modulesDir, 0755)

	confContent := `[KJV]
DataPath=./modules/texts/ztext/kjv/
Description=King James Version
Version=2.9
`
	os.WriteFile(filepath.Join(modsD, "kjv.conf"), []byte(confContent), 0600)
	os.WriteFile(filepath.Join(modulesDir, "nt.bzz"), []byte("data"), 0600)

	pluginDir := ensurePluginBuilt(t, "format-sword")
	plugin := &plugins.Plugin{
		Manifest: &plugins.PluginManifest{
			PluginID:   "format.sword",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-sword",
		},
		Path: pluginDir,
	}

	// Test detect on SWORD module
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
		t.Error("expected SWORD module to be detected")
	}
	if result.Format != "sword" {
		t.Errorf("expected format 'sword', got %q", result.Format)
	}
}

// TestFormatSwordEnumerate tests the format.sword plugin enumerate command.
func TestFormatSwordEnumerate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create SWORD module structure
	modsD := filepath.Join(tempDir, "mods.d")
	modulesDir := filepath.Join(tempDir, "modules", "texts", "ztext", "kjv")
	os.MkdirAll(modsD, 0755)
	os.MkdirAll(modulesDir, 0755)

	confContent := `[KJV]
DataPath=./modules/texts/ztext/kjv/
Description=King James Version
Version=2.9
`
	os.WriteFile(filepath.Join(modsD, "kjv.conf"), []byte(confContent), 0600)
	os.WriteFile(filepath.Join(modulesDir, "nt.bzz"), []byte("data"), 0600)
	os.WriteFile(filepath.Join(modulesDir, "ot.bzz"), []byte("data"), 0600)

	pluginDir := ensurePluginBuilt(t, "format-sword")
	plugin := &plugins.Plugin{
		Manifest: &plugins.PluginManifest{
			PluginID:   "format.sword",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-sword",
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

	// Should have: mods.d, mods.d/kjv.conf, modules, modules/texts,
	// modules/texts/ztext, modules/texts/ztext/kjv, nt.bzz, ot.bzz
	if len(result.Entries) < 5 {
		t.Errorf("expected at least 5 entries, got %d", len(result.Entries))
	}

	// Check that .conf file has metadata
	var foundConf bool
	for _, e := range result.Entries {
		if e.Path == "mods.d/kjv.conf" {
			foundConf = true
			if e.Metadata == nil || e.Metadata["module_name"] != "KJV" {
				t.Error("expected module_name metadata on .conf file")
			}
		}
	}
	if !foundConf {
		t.Error("kjv.conf not found in enumeration")
	}
}

// TestFormatSwordNotDetected tests that non-SWORD directories are not detected.
func TestFormatSwordNotDetected(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a regular directory (not SWORD)
	os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("test"), 0600)

	pluginDir := ensurePluginBuilt(t, "format-sword")
	plugin := &plugins.Plugin{
		Manifest: &plugins.PluginManifest{
			PluginID:   "format.sword",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-sword",
		},
		Path: pluginDir,
	}

	req := plugins.NewDetectRequest(tempDir)
	resp, err := plugins.ExecutePlugin(plugin, req)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	result, err := plugins.ParseDetectResult(resp)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if result.Detected {
		t.Error("regular directory should not be detected as SWORD")
	}
}
