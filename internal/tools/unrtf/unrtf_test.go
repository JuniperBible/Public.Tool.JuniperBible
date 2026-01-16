package unrtf

import (
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest == nil {
		t.Fatal("Manifest() returned nil")
	}

	// Test all manifest fields
	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"PluginID", manifest.PluginID, "tool.unrtf"},
		{"Version", manifest.Version, "1.0.0"},
		{"Kind", manifest.Kind, "tool"},
		{"Entrypoint", manifest.Entrypoint, "tool-unrtf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}

	// Test capabilities
	if manifest.Capabilities.Inputs == nil {
		t.Error("Capabilities.Inputs is nil, expected empty slice")
	}
	if len(manifest.Capabilities.Inputs) != 0 {
		t.Errorf("Capabilities.Inputs length = %d, want 0", len(manifest.Capabilities.Inputs))
	}

	if manifest.Capabilities.Outputs == nil {
		t.Error("Capabilities.Outputs is nil, expected empty slice")
	}
	if len(manifest.Capabilities.Outputs) != 0 {
		t.Errorf("Capabilities.Outputs length = %d, want 0", len(manifest.Capabilities.Outputs))
	}
}

func TestRegister(t *testing.T) {
	// Save the current registry state
	originalRegistry := plugins.ListEmbeddedPlugins()

	// Clear the registry for a clean test
	plugins.ClearEmbeddedRegistry()
	defer func() {
		// Restore the original registry after the test
		plugins.ClearEmbeddedRegistry()
		for _, p := range originalRegistry {
			plugins.RegisterEmbeddedPlugin(p)
		}
	}()

	// Call Register
	Register()

	// Verify the plugin was registered
	registered := plugins.ListEmbeddedPlugins()

	found := false
	for _, p := range registered {
		if p.Manifest != nil && p.Manifest.PluginID == "tool.unrtf" {
			found = true

			// Verify the registered plugin has the correct manifest
			if p.Manifest.Version != "1.0.0" {
				t.Errorf("Registered plugin version = %s, want 1.0.0", p.Manifest.Version)
			}
			if p.Manifest.Kind != "tool" {
				t.Errorf("Registered plugin kind = %s, want tool", p.Manifest.Kind)
			}
			if p.Manifest.Entrypoint != "tool-unrtf" {
				t.Errorf("Registered plugin entrypoint = %s, want tool-unrtf", p.Manifest.Entrypoint)
			}

			// Verify the tool handler is set
			if p.Tool == nil {
				t.Error("Registered plugin Tool is nil")
			} else if _, ok := p.Tool.(*Handler); !ok {
				t.Errorf("Registered plugin Tool type = %T, want *Handler", p.Tool)
			}

			break
		}
	}

	if !found {
		t.Error("Plugin tool.unrtf was not registered")
	}
}

func TestExecute(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name    string
		command string
		args    map[string]interface{}
	}{
		{
			name:    "empty command",
			command: "",
			args:    nil,
		},
		{
			name:    "convert command",
			command: "convert",
			args:    map[string]interface{}{"input": "test.rtf"},
		},
		{
			name:    "process command with args",
			command: "process",
			args:    map[string]interface{}{"file": "test.rtf", "output": "test.txt"},
		},
		{
			name:    "arbitrary command",
			command: "arbitrary",
			args:    map[string]interface{}{"key": "value"},
		},
		{
			name:    "command with nil args",
			command: "test",
			args:    nil,
		},
		{
			name:    "command with empty args",
			command: "test",
			args:    map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.Execute(tt.command, tt.args)

			// Verify result is nil
			if result != nil {
				t.Errorf("Execute() result = %v, want nil", result)
			}

			// Verify error is returned
			if err == nil {
				t.Fatal("Execute() error = nil, want error")
			}

			// Verify error message contains the command name
			expectedMsg := "unrtf command '" + tt.command + "' requires external plugin"
			if err.Error() != expectedMsg {
				t.Errorf("Execute() error = %q, want %q", err.Error(), expectedMsg)
			}
		})
	}
}

func TestHandler_ImplementsInterface(t *testing.T) {
	// Verify that Handler implements the EmbeddedToolHandler interface
	var _ plugins.EmbeddedToolHandler = (*Handler)(nil)
}

func TestInit(t *testing.T) {
	// The init() function is automatically called when the package is imported.
	// Since we're in the same package, it has already been called.
	// We can verify that it registered the plugin by checking the registry.

	registered := plugins.ListEmbeddedPlugins()

	found := false
	for _, p := range registered {
		if p.Manifest != nil && p.Manifest.PluginID == "tool.unrtf" {
			found = true
			break
		}
	}

	if !found {
		t.Error("init() did not register the plugin")
	}
}
