package hugo

import (
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// TestManifest tests the Manifest function returns correct values.
func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	if manifest.PluginID != "tool.hugo" {
		t.Errorf("expected PluginID 'tool.hugo', got '%s'", manifest.PluginID)
	}

	if manifest.Version != "1.0.0" {
		t.Errorf("expected Version '1.0.0', got '%s'", manifest.Version)
	}

	if manifest.Kind != "tool" {
		t.Errorf("expected Kind 'tool', got '%s'", manifest.Kind)
	}

	if manifest.Entrypoint != "tool-hugo" {
		t.Errorf("expected Entrypoint 'tool-hugo', got '%s'", manifest.Entrypoint)
	}

	if manifest.Capabilities.Inputs == nil {
		t.Error("expected non-nil Inputs slice")
	}

	if len(manifest.Capabilities.Inputs) != 0 {
		t.Errorf("expected empty Inputs slice, got %d items", len(manifest.Capabilities.Inputs))
	}

	if manifest.Capabilities.Outputs == nil {
		t.Error("expected non-nil Outputs slice")
	}

	if len(manifest.Capabilities.Outputs) != 0 {
		t.Errorf("expected empty Outputs slice, got %d items", len(manifest.Capabilities.Outputs))
	}
}

// TestRegister tests the Register function.
func TestRegister(t *testing.T) {
	// Clear registry before test
	plugins.ClearEmbeddedRegistry()
	defer plugins.ClearEmbeddedRegistry()

	// Call Register
	Register()

	// Verify plugin was registered
	if !plugins.HasEmbeddedPlugin("tool.hugo") {
		t.Error("expected plugin 'tool.hugo' to be registered")
	}

	// Retrieve the plugin
	plugin := plugins.GetEmbeddedPlugin("tool.hugo")
	if plugin == nil {
		t.Fatal("expected to retrieve registered plugin")
	}

	if plugin.Manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	if plugin.Manifest.PluginID != "tool.hugo" {
		t.Errorf("expected PluginID 'tool.hugo', got '%s'", plugin.Manifest.PluginID)
	}

	if plugin.Tool == nil {
		t.Error("expected non-nil Tool handler")
	}

	if plugin.Format != nil {
		t.Error("expected nil Format handler for tool plugin")
	}
}

// TestHandler_Execute tests the Execute method with various commands and args.
func TestHandler_Execute(t *testing.T) {
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
			name:    "empty command with args",
			command: "",
			args:    map[string]interface{}{"key": "value"},
		},
		{
			name:    "simple command",
			command: "generate",
			args:    nil,
		},
		{
			name:    "simple command with args",
			command: "generate",
			args:    map[string]interface{}{"input": "test"},
		},
		{
			name:    "command with multiple args",
			command: "convert",
			args: map[string]interface{}{
				"input":  "/path/to/input",
				"output": "/path/to/output",
				"format": "json",
			},
		},
		{
			name:    "command with complex args",
			command: "export",
			args: map[string]interface{}{
				"config": map[string]interface{}{
					"nested": "value",
				},
				"array": []string{"a", "b", "c"},
			},
		},
		{
			name:    "long command name",
			command: "very-long-command-name-that-should-still-work",
			args:    map[string]interface{}{},
		},
		{
			name:    "special characters in command",
			command: "command-with-dashes_and_underscores",
			args:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.Execute(tt.command, tt.args)

			// Result should always be nil
			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}

			// Should always return an error
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			// Error message should contain the command and mention external plugin
			errMsg := err.Error()
			if errMsg == "" {
				t.Error("expected non-empty error message")
			}

			// Error message should contain "hugo"
			if len(errMsg) < 4 || errMsg[:4] != "hugo" {
				t.Errorf("expected error message to start with 'hugo', got '%s'", errMsg)
			}

			// Error message should contain "external plugin"
			expectedSubstr := "requires external plugin"
			found := false
			for i := 0; i <= len(errMsg)-len(expectedSubstr); i++ {
				if errMsg[i:i+len(expectedSubstr)] == expectedSubstr {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error message to contain '%s', got '%s'", expectedSubstr, errMsg)
			}
		})
	}
}

// TestHandler_Execute_ErrorContainsCommand tests that Execute error messages contain the command.
func TestHandler_Execute_ErrorContainsCommand(t *testing.T) {
	handler := &Handler{}

	commands := []string{
		"generate",
		"convert",
		"export",
		"test",
		"validate",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			_, err := handler.Execute(cmd, nil)

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			errMsg := err.Error()

			// Check if command appears in the error message
			found := false
			for i := 0; i <= len(errMsg)-len(cmd); i++ {
				if errMsg[i:i+len(cmd)] == cmd {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("expected error message to contain command '%s', got '%s'", cmd, errMsg)
			}
		})
	}
}

// TestInit tests that the init function registers the plugin automatically.
func TestInit(t *testing.T) {
	// The init function should have already run when the package was imported.
	// We test that the plugin is registered by checking if it exists.
	// Note: We cannot re-run init(), but we can verify its effect.

	// Clear and re-register to test the Register function
	plugins.ClearEmbeddedRegistry()
	Register() // This simulates what init() does

	if !plugins.HasEmbeddedPlugin("tool.hugo") {
		t.Error("expected plugin 'tool.hugo' to be registered after init/Register")
	}

	plugin := plugins.GetEmbeddedPlugin("tool.hugo")
	if plugin == nil {
		t.Fatal("expected to retrieve registered plugin after init/Register")
	}

	// Verify the handler is properly set up
	if plugin.Tool == nil {
		t.Error("expected non-nil Tool handler after init/Register")
	}

	// Verify we can execute through the plugin (should return error)
	if h, ok := plugin.Tool.(*Handler); ok {
		_, err := h.Execute("test", nil)
		if err == nil {
			t.Error("expected error from Execute, got nil")
		}
	} else {
		t.Error("expected Tool to be *Handler type")
	}
}

// TestHandler_Execute_NilArgs tests Execute with nil args map.
func TestHandler_Execute_NilArgs(t *testing.T) {
	handler := &Handler{}

	commands := []string{"generate", "convert", "export", ""}

	for _, cmd := range commands {
		t.Run("command_"+cmd, func(t *testing.T) {
			result, err := handler.Execute(cmd, nil)

			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// TestHandler_Execute_EmptyArgs tests Execute with empty args map.
func TestHandler_Execute_EmptyArgs(t *testing.T) {
	handler := &Handler{}

	commands := []string{"generate", "convert", "export"}

	for _, cmd := range commands {
		t.Run("command_"+cmd, func(t *testing.T) {
			result, err := handler.Execute(cmd, map[string]interface{}{})

			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// TestManifest_Immutability tests that Manifest returns a new object each time.
func TestManifest_Immutability(t *testing.T) {
	manifest1 := Manifest()
	manifest2 := Manifest()

	// Should be different objects (different pointers)
	if manifest1 == manifest2 {
		t.Error("expected Manifest() to return different object instances")
	}

	// But should have same values
	if manifest1.PluginID != manifest2.PluginID {
		t.Error("expected PluginID to be the same across calls")
	}

	if manifest1.Version != manifest2.Version {
		t.Error("expected Version to be the same across calls")
	}

	if manifest1.Kind != manifest2.Kind {
		t.Error("expected Kind to be the same across calls")
	}

	if manifest1.Entrypoint != manifest2.Entrypoint {
		t.Error("expected Entrypoint to be the same across calls")
	}
}

// TestHandler_MultipleInstances tests that multiple Handler instances behave the same.
func TestHandler_MultipleInstances(t *testing.T) {
	handler1 := &Handler{}
	handler2 := &Handler{}

	command := "test-command"
	args := map[string]interface{}{"key": "value"}

	result1, err1 := handler1.Execute(command, args)
	result2, err2 := handler2.Execute(command, args)

	if result1 != result2 {
		t.Errorf("expected same result from both handlers, got %v and %v", result1, result2)
	}

	if (err1 == nil) != (err2 == nil) {
		t.Errorf("expected both handlers to return error or both to return nil, got %v and %v", err1, err2)
	}

	if err1 != nil && err2 != nil && err1.Error() != err2.Error() {
		t.Errorf("expected same error message from both handlers, got '%s' and '%s'", err1.Error(), err2.Error())
	}
}

// TestRegister_Idempotent tests that calling Register multiple times is safe.
func TestRegister_Idempotent(t *testing.T) {
	plugins.ClearEmbeddedRegistry()
	defer plugins.ClearEmbeddedRegistry()

	// Register multiple times
	Register()
	Register()
	Register()

	// Should still only have one plugin registered
	if !plugins.HasEmbeddedPlugin("tool.hugo") {
		t.Error("expected plugin 'tool.hugo' to be registered")
	}

	plugin := plugins.GetEmbeddedPlugin("tool.hugo")
	if plugin == nil {
		t.Fatal("expected to retrieve registered plugin")
	}

	// Verify it's still functional
	if h, ok := plugin.Tool.(*Handler); ok {
		_, err := h.Execute("test", nil)
		if err == nil {
			t.Error("expected error from Execute after multiple Register calls")
		}
	}
}

// TestHandler_Execute_AllErrorsNonNil tests that Execute never returns a nil error.
func TestHandler_Execute_AllErrorsNonNil(t *testing.T) {
	handler := &Handler{}

	// Test with many different inputs
	testCases := []struct {
		command string
		args    map[string]interface{}
	}{
		{"", nil},
		{"", map[string]interface{}{}},
		{"test", nil},
		{"test", map[string]interface{}{}},
		{"test", map[string]interface{}{"a": 1}},
		{"generate", map[string]interface{}{"input": "test", "output": "test"}},
		{"x", nil},
		{"very-long-command-name-with-many-dashes-and-underscores", map[string]interface{}{"key": "value"}},
	}

	for _, tc := range testCases {
		_, err := handler.Execute(tc.command, tc.args)
		if err == nil {
			t.Errorf("expected non-nil error for command '%s' with args %v", tc.command, tc.args)
		}
	}
}

// TestManifest_CapabilitiesNotNil tests that Capabilities fields are never nil.
func TestManifest_CapabilitiesNotNil(t *testing.T) {
	manifest := Manifest()

	if manifest.Capabilities.Inputs == nil {
		t.Error("expected Capabilities.Inputs to be non-nil (empty slice, not nil)")
	}

	if manifest.Capabilities.Outputs == nil {
		t.Error("expected Capabilities.Outputs to be non-nil (empty slice, not nil)")
	}

	// Verify they're empty slices, not nil
	if len(manifest.Capabilities.Inputs) != 0 {
		t.Errorf("expected Capabilities.Inputs to be empty, got %d items", len(manifest.Capabilities.Inputs))
	}

	if len(manifest.Capabilities.Outputs) != 0 {
		t.Errorf("expected Capabilities.Outputs to be empty, got %d items", len(manifest.Capabilities.Outputs))
	}
}

// TestHandler_ZeroValue tests that a zero-value Handler still works correctly.
func TestHandler_ZeroValue(t *testing.T) {
	var handler Handler // zero value

	result, err := handler.Execute("test", nil)

	if result != nil {
		t.Errorf("expected nil result from zero-value handler, got %v", result)
	}

	if err == nil {
		t.Error("expected error from zero-value handler, got nil")
	}
}
