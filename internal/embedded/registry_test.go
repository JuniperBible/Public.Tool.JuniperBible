package embedded_test

import (
	"testing"

	"github.com/FocuswithJustin/JuniperBible/internal/embedded"
	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// TestPluginRegistrations verifies that all embedded plugins are registered.
// This test ensures that importing the embedded package triggers all init() functions
// and registers all format and tool plugins with the plugin registry.
func TestPluginRegistrations(t *testing.T) {
	// Expected format plugins that should be registered (by plugin ID)
	// Note: format.swordpure is imported but doesn't have registration yet
	expectedFormats := []string{
		"format.accordance",
		"format.bibletime",
		"format.dbl",
		"format.dir",
		"format.ecm",
		"format.epub",
		"format.esword",
		"format.file",
		"format.flex",
		"format.gobible",
		"format.html",
		"format.json",
		"format.logos",
		"format.markdown",
		"format.morphgnt",
		"format.mybible",
		"format.mysword",
		"format.na28app",
		"format.odf",
		"format.olive",
		"format.onlinebible",
		"format.oshb",
		"format.osis",
		"format.pdb",
		"format.rtf",
		"format.sblgnt",
		"format.sfm",
		"format.sword",
		// "format.swordpure", // Not yet registered
		"format.tar",
		"format.tei",
		"format.theword",
		"format.txt",
		"format.usfm",
		"format.usx",
		"format.xml",
		"format.zefania",
	}

	// Expected tool plugins that should be registered (by plugin ID)
	// Note: tool.gobiblecreator is imported but doesn't have registration yet
	expectedTools := []string{
		"tool.calibre",
		// "tool.gobiblecreator", // Not yet registered
		"tool.hugo",
		"tool.libsword",
		"tool.libxml2",
		"tool.pandoc",
		"tool.repoman",
		"tool.sqlite",
		"tool.unrtf",
		"tool.usfm2osis",
	}

	t.Run("FormatPluginsRegistered", func(t *testing.T) {
		for _, format := range expectedFormats {
			t.Run(format, func(t *testing.T) {
				plugin := plugins.GetEmbeddedPlugin(format)
				if plugin == nil {
					t.Errorf("format plugin %q not registered", format)
				} else if plugin.Format == nil {
					t.Errorf("format plugin %q has nil Format handler", format)
				}
			})
		}
	})

	t.Run("ToolPluginsRegistered", func(t *testing.T) {
		for _, tool := range expectedTools {
			t.Run(tool, func(t *testing.T) {
				plugin := plugins.GetEmbeddedPlugin(tool)
				if plugin == nil {
					t.Errorf("tool plugin %q not registered", tool)
				} else if plugin.Tool == nil {
					t.Errorf("tool plugin %q has nil Tool handler", tool)
				}
			})
		}
	})

	t.Run("AllPluginsListed", func(t *testing.T) {
		// Get all registered plugins
		allPlugins := plugins.ListEmbeddedPlugins()

		// Create a map for quick lookup
		pluginMap := make(map[string]*plugins.EmbeddedPlugin)
		for _, plugin := range allPlugins {
			if plugin.Manifest != nil {
				pluginMap[plugin.Manifest.PluginID] = plugin
			}
		}

		// Verify all expected formats are in the list
		for _, format := range expectedFormats {
			if _, ok := pluginMap[format]; !ok {
				t.Errorf("expected format plugin %q not found in ListEmbeddedPlugins()", format)
			}
		}

		// Verify all expected tools are in the list
		for _, tool := range expectedTools {
			if _, ok := pluginMap[tool]; !ok {
				t.Errorf("expected tool plugin %q not found in ListEmbeddedPlugins()", tool)
			}
		}

		// Check we have at least the expected number of plugins
		expectedTotal := len(expectedFormats) + len(expectedTools)
		if len(allPlugins) < expectedTotal {
			t.Errorf("expected at least %d plugins, got %d", expectedTotal, len(allPlugins))
		}
	})

	t.Run("HasEmbeddedPlugin", func(t *testing.T) {
		// Test HasEmbeddedPlugin for a few known plugins
		if !plugins.HasEmbeddedPlugin("format.txt") {
			t.Error("HasEmbeddedPlugin returned false for known plugin format.txt")
		}

		if !plugins.HasEmbeddedPlugin("tool.sqlite") {
			t.Error("HasEmbeddedPlugin returned false for known plugin tool.sqlite")
		}

		if plugins.HasEmbeddedPlugin("nonexistent.plugin") {
			t.Error("HasEmbeddedPlugin returned true for non-existent plugin")
		}
	})
}

// TestPackageImport simply imports the package to ensure all init() functions run.
// This achieves coverage for the import statements themselves.
func TestPackageImport(t *testing.T) {
	// The embedded package has been imported at the top of this file.
	// This test verifies that the import doesn't panic or cause errors.

	// Verify the registry is functional
	allPlugins := plugins.ListEmbeddedPlugins()
	if len(allPlugins) == 0 {
		t.Error("no plugins registered after importing embedded package")
	}

	// Count formats and tools
	var formatCount, toolCount int
	for _, plugin := range allPlugins {
		if plugin.Format != nil {
			formatCount++
		}
		if plugin.Tool != nil {
			toolCount++
		}
	}

	t.Logf("Successfully registered %d total plugins (%d formats, %d tools)",
		len(allPlugins), formatCount, toolCount)

	if formatCount == 0 {
		t.Error("no format plugins registered")
	}

	if toolCount == 0 {
		t.Error("no tool plugins registered")
	}
}

// TestIsInitialized tests the IsInitialized function.
func TestIsInitialized(t *testing.T) {
	// Since we've imported the package, it should be initialized
	if !embedded.IsInitialized() {
		t.Error("IsInitialized() returned false, expected true")
	}
}

// TestPluginCount tests the PluginCount function.
func TestPluginCount(t *testing.T) {
	count := embedded.PluginCount()

	// We should have at least 45 plugins (36 formats + 9 tools)
	// Note: The actual count may be higher if more plugins are added
	if count < 45 {
		t.Errorf("PluginCount() returned %d, expected at least 45", count)
	}

	// Verify it matches the actual number of registered plugins
	actualPlugins := plugins.ListEmbeddedPlugins()
	if count != len(actualPlugins) {
		t.Errorf("PluginCount() returned %d, but ListEmbeddedPlugins() has %d plugins",
			count, len(actualPlugins))
	}

	t.Logf("PluginCount() correctly reports %d plugins", count)
}
