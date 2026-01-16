package usfm2osis

import (
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func TestManifest(t *testing.T) {
	manifest := Manifest()

	// Test all manifest fields
	if manifest.PluginID != "tool.usfm2osis" {
		t.Errorf("expected PluginID to be 'tool.usfm2osis', got %q", manifest.PluginID)
	}

	if manifest.Version != "1.0.0" {
		t.Errorf("expected Version to be '1.0.0', got %q", manifest.Version)
	}

	if manifest.Kind != "tool" {
		t.Errorf("expected Kind to be 'tool', got %q", manifest.Kind)
	}

	if manifest.Entrypoint != "tool-usfm2osis" {
		t.Errorf("expected Entrypoint to be 'tool-usfm2osis', got %q", manifest.Entrypoint)
	}

	// Test capabilities
	if len(manifest.Capabilities.Inputs) != 0 {
		t.Errorf("expected Inputs to be empty, got %v", manifest.Capabilities.Inputs)
	}

	if len(manifest.Capabilities.Outputs) != 0 {
		t.Errorf("expected Outputs to be empty, got %v", manifest.Capabilities.Outputs)
	}
}

func TestRegister(t *testing.T) {
	// Clear the registry before testing
	plugins.ClearEmbeddedRegistry()

	// Call Register
	Register()

	// Verify the plugin was registered
	if !plugins.HasEmbeddedPlugin("tool.usfm2osis") {
		t.Error("expected plugin to be registered, but it was not found")
	}

	// Verify the registered plugin has the correct manifest
	ep := plugins.GetEmbeddedPlugin("tool.usfm2osis")
	if ep == nil {
		t.Fatal("expected to retrieve registered plugin, got nil")
	}

	if ep.Manifest == nil {
		t.Fatal("expected plugin to have a manifest, got nil")
	}

	if ep.Manifest.PluginID != "tool.usfm2osis" {
		t.Errorf("expected registered plugin ID to be 'tool.usfm2osis', got %q", ep.Manifest.PluginID)
	}

	// Verify the Tool handler is set
	if ep.Tool == nil {
		t.Error("expected plugin to have a Tool handler, got nil")
	}

	// Verify Format handler is not set (this is a tool plugin, not format)
	if ep.Format != nil {
		t.Error("expected plugin Format handler to be nil for tool plugin, got non-nil")
	}
}

func TestInit(t *testing.T) {
	// The init() function is called automatically when the package is loaded.
	// By the time this test runs, the plugin should already be registered.
	// We verify that the init() function worked by checking if the plugin exists.

	if !plugins.HasEmbeddedPlugin("tool.usfm2osis") {
		t.Error("expected plugin to be registered by init(), but it was not found")
	}
}

func TestHandlerExecute(t *testing.T) {
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
			name:    "simple command",
			command: "convert",
			args:    nil,
		},
		{
			name:    "command with args",
			command: "convert",
			args: map[string]interface{}{
				"input":  "test.usfm",
				"output": "test.osis",
			},
		},
		{
			name:    "unknown command",
			command: "unknown",
			args: map[string]interface{}{
				"foo": "bar",
			},
		},
		{
			name:    "command with complex args",
			command: "validate",
			args: map[string]interface{}{
				"input":   "test.usfm",
				"verbose": true,
				"level":   5,
				"options": map[string]interface{}{
					"strict": true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.Execute(tt.command, tt.args)

			// Execute should always return an error
			if err == nil {
				t.Error("expected Execute to return an error, got nil")
			}

			// Result should be nil
			if result != nil {
				t.Errorf("expected result to be nil, got %v", result)
			}

			// Error message should mention the command and external plugin requirement
			expectedSubstring := "requires external plugin"
			if err.Error() != "usfm2osis command '"+tt.command+"' requires external plugin" {
				t.Errorf("expected error message to be 'usfm2osis command '%s' requires external plugin', got %q",
					tt.command, err.Error())
			}

			// Verify the error contains the command name
			if tt.command != "" {
				commandInError := false
				for i := 0; i <= len(err.Error())-len(tt.command); i++ {
					if err.Error()[i:i+len(tt.command)] == tt.command {
						commandInError = true
						break
					}
				}
				if !commandInError {
					t.Errorf("expected error message to contain command %q, got %q", tt.command, err.Error())
				}
			}

			// Verify the error mentions "requires external plugin"
			requiresExternalPlugin := false
			for i := 0; i <= len(err.Error())-len(expectedSubstring); i++ {
				if err.Error()[i:i+len(expectedSubstring)] == expectedSubstring {
					requiresExternalPlugin = true
					break
				}
			}
			if !requiresExternalPlugin {
				t.Errorf("expected error message to contain %q, got %q", expectedSubstring, err.Error())
			}
		})
	}
}

func TestHandlerExecuteErrorFormat(t *testing.T) {
	// Test that the error format is exactly as expected
	handler := &Handler{}

	testCommand := "test-command"
	_, err := handler.Execute(testCommand, nil)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedError := "usfm2osis command 'test-command' requires external plugin"
	if err.Error() != expectedError {
		t.Errorf("expected error %q, got %q", expectedError, err.Error())
	}
}

func TestManifestConsistency(t *testing.T) {
	// Verify that calling Manifest() multiple times returns consistent results
	manifest1 := Manifest()
	manifest2 := Manifest()

	if manifest1.PluginID != manifest2.PluginID {
		t.Error("Manifest() should return consistent PluginID")
	}

	if manifest1.Version != manifest2.Version {
		t.Error("Manifest() should return consistent Version")
	}

	if manifest1.Kind != manifest2.Kind {
		t.Error("Manifest() should return consistent Kind")
	}

	if manifest1.Entrypoint != manifest2.Entrypoint {
		t.Error("Manifest() should return consistent Entrypoint")
	}
}

func TestHandlerType(t *testing.T) {
	// Verify that Handler is a struct type
	var _ plugins.EmbeddedToolHandler = (*Handler)(nil)

	// Verify that Handler has no fields (it's an empty struct)
	handler := &Handler{}
	if handler == nil {
		t.Fatal("expected handler to be non-nil")
	}
}

func TestRegisterIdempotency(t *testing.T) {
	// Clear the registry
	plugins.ClearEmbeddedRegistry()

	// Register multiple times
	Register()
	Register()
	Register()

	// Should still only have one registration
	if !plugins.HasEmbeddedPlugin("tool.usfm2osis") {
		t.Error("expected plugin to be registered")
	}

	// Get the plugin and verify it's correct
	ep := plugins.GetEmbeddedPlugin("tool.usfm2osis")
	if ep == nil {
		t.Fatal("expected to retrieve plugin")
	}

	if ep.Manifest.PluginID != "tool.usfm2osis" {
		t.Error("plugin ID mismatch after multiple registrations")
	}
}
