package repoman

import (
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest == nil {
		t.Fatal("Manifest() returned nil")
	}

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"PluginID", manifest.PluginID, "tool.repoman"},
		{"Version", manifest.Version, "1.0.0"},
		{"Kind", manifest.Kind, "tool"},
		{"Entrypoint", manifest.Entrypoint, "tool-repoman"},
		{"InputsLength", len(manifest.Capabilities.Inputs), 0},
		{"OutputsLength", len(manifest.Capabilities.Outputs), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("Manifest().%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestRegister(t *testing.T) {
	// Clear registry to ensure clean state
	plugins.ClearEmbeddedRegistry()

	// Call Register
	Register()

	// Verify plugin was registered
	plugin := plugins.GetEmbeddedPlugin("tool.repoman")
	if plugin == nil {
		t.Fatal("Register() did not register plugin")
	}

	// Verify manifest
	if plugin.Manifest == nil {
		t.Fatal("Registered plugin has nil Manifest")
	}
	if plugin.Manifest.PluginID != "tool.repoman" {
		t.Errorf("Registered plugin ID = %s, want tool.repoman", plugin.Manifest.PluginID)
	}

	// Verify Tool handler is set
	if plugin.Tool == nil {
		t.Fatal("Registered plugin has nil Tool handler")
	}

	// Verify Format handler is not set (this is a tool, not a format)
	if plugin.Format != nil {
		t.Error("Registered tool plugin should not have Format handler")
	}

	// Verify the handler is of correct type
	_, ok := plugin.Tool.(*Handler)
	if !ok {
		t.Errorf("Registered Tool handler has wrong type: %T", plugin.Tool)
	}
}

func TestExecute_ListSources(t *testing.T) {
	h := &Handler{}

	result, err := h.Execute("list-sources", map[string]interface{}{})

	if err != nil {
		t.Fatalf("Execute('list-sources') returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Execute('list-sources') returned nil result")
	}

	// Cast result to expected type
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Execute('list-sources') returned wrong type: %T, want map[string]interface{}", result)
	}

	// Check sources field exists
	sources, ok := resultMap["sources"]
	if !ok {
		t.Fatal("Result missing 'sources' field")
	}

	// Cast sources to expected type
	sourcesList, ok := sources.([]map[string]string)
	if !ok {
		t.Fatalf("sources field has wrong type: %T, want []map[string]string", sources)
	}

	// Verify we have expected sources
	expectedSources := []struct {
		name string
		url  string
	}{
		{"CrossWire", "https://www.crosswire.org/ftpmirror/pub/sword/packages/rawzip/"},
		{"eBible", "https://ebible.org/sword/"},
	}

	if len(sourcesList) != len(expectedSources) {
		t.Fatalf("sources list length = %d, want %d", len(sourcesList), len(expectedSources))
	}

	for i, expected := range expectedSources {
		if sourcesList[i]["name"] != expected.name {
			t.Errorf("sources[%d].name = %s, want %s", i, sourcesList[i]["name"], expected.name)
		}
		if sourcesList[i]["url"] != expected.url {
			t.Errorf("sources[%d].url = %s, want %s", i, sourcesList[i]["url"], expected.url)
		}
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name    string
		command string
		args    map[string]interface{}
	}{
		{
			name:    "EmptyCommand",
			command: "",
			args:    map[string]interface{}{},
		},
		{
			name:    "InstallCommand",
			command: "install",
			args:    map[string]interface{}{"module": "KJV"},
		},
		{
			name:    "UpdateCommand",
			command: "update",
			args:    nil,
		},
		{
			name:    "RemoveCommand",
			command: "remove",
			args:    map[string]interface{}{"module": "ESV"},
		},
		{
			name:    "SearchCommand",
			command: "search",
			args:    map[string]interface{}{"query": "bible"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := h.Execute(tt.command, tt.args)

			if err == nil {
				t.Fatalf("Execute('%s') should return error, got nil", tt.command)
			}

			expectedError := "repoman command '" + tt.command + "' requires external plugin"
			if err.Error() != expectedError {
				t.Errorf("Execute('%s') error = %q, want %q", tt.command, err.Error(), expectedError)
			}

			if result != nil {
				t.Errorf("Execute('%s') returned non-nil result: %v", tt.command, result)
			}
		})
	}
}

func TestExecute_WithVariousArgs(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name string
		args map[string]interface{}
	}{
		{
			name: "EmptyArgs",
			args: map[string]interface{}{},
		},
		{
			name: "NilArgs",
			args: nil,
		},
		{
			name: "StringArgs",
			args: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "MixedArgs",
			args: map[string]interface{}{
				"string": "test",
				"number": 42,
				"bool":   true,
				"slice":  []string{"a", "b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := h.Execute("list-sources", tt.args)

			if err != nil {
				t.Fatalf("Execute('list-sources') returned error: %v", err)
			}

			if result == nil {
				t.Fatal("Execute('list-sources') returned nil result")
			}

			// Verify result structure is correct regardless of args
			resultMap, ok := result.(map[string]interface{})
			if !ok {
				t.Fatalf("Result has wrong type: %T", result)
			}

			sources, ok := resultMap["sources"]
			if !ok {
				t.Fatal("Result missing 'sources' field")
			}

			sourcesList, ok := sources.([]map[string]string)
			if !ok {
				t.Fatalf("sources field has wrong type: %T", sources)
			}

			if len(sourcesList) != 2 {
				t.Errorf("sources list length = %d, want 2", len(sourcesList))
			}
		})
	}
}

func TestHandler_Type(t *testing.T) {
	// Verify Handler implements EmbeddedToolHandler interface
	var _ plugins.EmbeddedToolHandler = (*Handler)(nil)
}

func TestInit(t *testing.T) {
	// The init() function is called automatically when the package is imported.
	// We can verify its effect by checking if the plugin is registered.

	// Since init() has already run, we should find the plugin registered
	plugin := plugins.GetEmbeddedPlugin("tool.repoman")
	if plugin == nil {
		t.Error("init() should have registered the plugin, but it was not found")
	}

	// Verify the plugin has the correct manifest and handler
	if plugin != nil {
		if plugin.Manifest == nil {
			t.Error("Plugin registered by init() has nil Manifest")
		} else if plugin.Manifest.PluginID != "tool.repoman" {
			t.Errorf("Plugin registered by init() has wrong ID: %s", plugin.Manifest.PluginID)
		}

		if plugin.Tool == nil {
			t.Error("Plugin registered by init() has nil Tool handler")
		}
	}
}
