package calibre

import (
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// TestManifest tests the Manifest function.
func TestManifest(t *testing.T) {
	manifest := Manifest()

	// Test non-nil manifest
	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	// Test PluginID
	if manifest.PluginID != "tool.calibre" {
		t.Errorf("expected PluginID 'tool.calibre', got '%s'", manifest.PluginID)
	}

	// Test Version
	if manifest.Version != "1.0.0" {
		t.Errorf("expected Version '1.0.0', got '%s'", manifest.Version)
	}

	// Test Kind
	if manifest.Kind != "tool" {
		t.Errorf("expected Kind 'tool', got '%s'", manifest.Kind)
	}

	// Test Entrypoint
	if manifest.Entrypoint != "tool-calibre" {
		t.Errorf("expected Entrypoint 'tool-calibre', got '%s'", manifest.Entrypoint)
	}

	// Test Capabilities - Inputs
	if manifest.Capabilities.Inputs == nil {
		t.Error("expected non-nil Capabilities.Inputs")
	}
	if len(manifest.Capabilities.Inputs) != 0 {
		t.Errorf("expected 0 inputs, got %d", len(manifest.Capabilities.Inputs))
	}

	// Test Capabilities - Outputs
	if manifest.Capabilities.Outputs == nil {
		t.Error("expected non-nil Capabilities.Outputs")
	}
	if len(manifest.Capabilities.Outputs) != 0 {
		t.Errorf("expected 0 outputs, got %d", len(manifest.Capabilities.Outputs))
	}
}

// TestRegister tests the Register function.
func TestRegister(t *testing.T) {
	// Clear registry before test
	plugins.ClearEmbeddedRegistry()
	defer plugins.ClearEmbeddedRegistry()

	// Register the plugin
	Register()

	// Verify it was registered
	plugin := plugins.GetEmbeddedPlugin("tool.calibre")
	if plugin == nil {
		t.Fatal("expected plugin to be registered")
	}

	// Verify manifest
	if plugin.Manifest == nil {
		t.Fatal("expected non-nil manifest")
	}
	if plugin.Manifest.PluginID != "tool.calibre" {
		t.Errorf("expected PluginID 'tool.calibre', got '%s'", plugin.Manifest.PluginID)
	}

	// Verify tool handler exists
	if plugin.Tool == nil {
		t.Fatal("expected non-nil Tool handler")
	}

	// Verify format handler is nil
	if plugin.Format != nil {
		t.Error("expected nil Format handler for tool plugin")
	}

	// Verify handler is of correct type
	if _, ok := plugin.Tool.(*Handler); !ok {
		t.Errorf("expected Tool to be *Handler, got %T", plugin.Tool)
	}
}

// TestRegisterMultipleCalls tests that Register can be called multiple times safely.
func TestRegisterMultipleCalls(t *testing.T) {
	plugins.ClearEmbeddedRegistry()
	defer plugins.ClearEmbeddedRegistry()

	// Register multiple times
	Register()
	Register()
	Register()

	// Should still be registered correctly
	plugin := plugins.GetEmbeddedPlugin("tool.calibre")
	if plugin == nil {
		t.Fatal("expected plugin to be registered after multiple Register calls")
	}

	// Count should still be 1 in the registry
	pluginsList := plugins.ListEmbeddedPlugins()
	calibreCount := 0
	for _, p := range pluginsList {
		if p.Manifest.PluginID == "tool.calibre" {
			calibreCount++
		}
	}
	if calibreCount != 1 {
		t.Errorf("expected 1 calibre plugin in registry, got %d", calibreCount)
	}
}

// TestInit verifies that init() registers the plugin.
// Since init() is called automatically, we test by clearing and re-registering.
func TestInit(t *testing.T) {
	// Clear registry to simulate fresh start
	plugins.ClearEmbeddedRegistry()

	// Call Register (which is what init() does)
	Register()

	// Verify the plugin is now registered
	plugin := plugins.GetEmbeddedPlugin("tool.calibre")
	if plugin == nil {
		t.Fatal("expected plugin to be registered after calling Register")
	}

	if plugin.Manifest.PluginID != "tool.calibre" {
		t.Errorf("expected PluginID 'tool.calibre', got '%s'", plugin.Manifest.PluginID)
	}

	// Clean up
	plugins.ClearEmbeddedRegistry()
}

// TestHandlerExecute tests the Handler.Execute method.
func TestHandlerExecute(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name            string
		command         string
		args            map[string]interface{}
		expectError     bool
		expectedErrText string
	}{
		{
			name:            "empty command",
			command:         "",
			args:            nil,
			expectError:     true,
			expectedErrText: "calibre command '' requires external plugin",
		},
		{
			name:            "convert command",
			command:         "convert",
			args:            map[string]interface{}{"input": "test.epub", "output": "test.pdf"},
			expectError:     true,
			expectedErrText: "calibre command 'convert' requires external plugin",
		},
		{
			name:            "ebook-meta command",
			command:         "ebook-meta",
			args:            map[string]interface{}{"file": "test.epub"},
			expectError:     true,
			expectedErrText: "calibre command 'ebook-meta' requires external plugin",
		},
		{
			name:            "any command",
			command:         "any-command",
			args:            map[string]interface{}{},
			expectError:     true,
			expectedErrText: "calibre command 'any-command' requires external plugin",
		},
		{
			name:            "nil args",
			command:         "test",
			args:            nil,
			expectError:     true,
			expectedErrText: "calibre command 'test' requires external plugin",
		},
		{
			name:            "empty args",
			command:         "test",
			args:            map[string]interface{}{},
			expectError:     true,
			expectedErrText: "calibre command 'test' requires external plugin",
		},
		{
			name:            "complex args",
			command:         "process",
			args:            map[string]interface{}{"key1": "value1", "key2": 42, "key3": []string{"a", "b"}},
			expectError:     true,
			expectedErrText: "calibre command 'process' requires external plugin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.Execute(tt.command, tt.args)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if err.Error() != tt.expectedErrText {
					t.Errorf("expected error '%s', got '%s'", tt.expectedErrText, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}

			// Result should always be nil
			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
		})
	}
}

// TestHandlerExecuteCommandVariations tests Execute with various command formats.
func TestHandlerExecuteCommandVariations(t *testing.T) {
	handler := &Handler{}

	commands := []string{
		"simple",
		"with-dash",
		"with_underscore",
		"MixedCase",
		"UPPERCASE",
		"lowercase",
		"with spaces",
		"with.dots",
		"123numeric",
		"special!@#$%^&*()",
		"unicode-文字",
		"very-long-command-name-that-goes-on-and-on",
		" leading-space",
		"trailing-space ",
		"  multiple  spaces  ",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			result, err := handler.Execute(cmd, nil)

			// Should always return error
			if err == nil {
				t.Error("expected error, got nil")
			}

			// Should always return nil result
			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}

			// Error message should contain the command
			if err != nil {
				expectedErr := "calibre command '" + cmd + "' requires external plugin"
				if err.Error() != expectedErr {
					t.Errorf("expected error '%s', got '%s'", expectedErr, err.Error())
				}
			}
		})
	}
}

// TestHandlerExecuteArgsTypes tests Execute with various argument types.
func TestHandlerExecuteArgsTypes(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name string
		args map[string]interface{}
	}{
		{
			name: "nil args",
			args: nil,
		},
		{
			name: "empty args",
			args: map[string]interface{}{},
		},
		{
			name: "string args",
			args: map[string]interface{}{"key": "value"},
		},
		{
			name: "int args",
			args: map[string]interface{}{"number": 42},
		},
		{
			name: "float args",
			args: map[string]interface{}{"float": 3.14},
		},
		{
			name: "bool args",
			args: map[string]interface{}{"flag": true},
		},
		{
			name: "slice args",
			args: map[string]interface{}{"list": []string{"a", "b", "c"}},
		},
		{
			name: "map args",
			args: map[string]interface{}{"nested": map[string]string{"key": "value"}},
		},
		{
			name: "mixed args",
			args: map[string]interface{}{
				"string": "value",
				"int":    42,
				"bool":   true,
				"slice":  []int{1, 2, 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.Execute("test", tt.args)

			// Should always return error
			if err == nil {
				t.Error("expected error, got nil")
			}

			// Should always return nil result
			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
		})
	}
}

// TestHandlerIntegration tests the full integration with the plugin system.
func TestHandlerIntegration(t *testing.T) {
	plugins.ClearEmbeddedRegistry()
	defer plugins.ClearEmbeddedRegistry()

	// Register the plugin
	Register()

	// Execute through the plugin system
	resp, err := plugins.ExecuteEmbeddedPlugin("tool.calibre", &plugins.IPCRequest{
		Command: "convert",
		Args:    map[string]interface{}{"input": "test.epub"},
	})

	// Should not return a fatal error
	if err != nil {
		t.Fatalf("unexpected error from ExecuteEmbeddedPlugin: %v", err)
	}

	// Should return error response
	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	if resp.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", resp.Status)
	}

	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}

	expectedErrText := "calibre command 'convert' requires external plugin"
	if resp.Error != expectedErrText {
		t.Errorf("expected error '%s', got '%s'", expectedErrText, resp.Error)
	}
}

// TestHandlerTypeAssertion tests that the registered handler is the correct type.
func TestHandlerTypeAssertion(t *testing.T) {
	plugins.ClearEmbeddedRegistry()
	defer plugins.ClearEmbeddedRegistry()

	Register()

	plugin := plugins.GetEmbeddedPlugin("tool.calibre")
	if plugin == nil {
		t.Fatal("expected plugin to be registered")
	}

	// Type assertion
	handler, ok := plugin.Tool.(*Handler)
	if !ok {
		t.Fatalf("expected Tool to be *Handler, got %T", plugin.Tool)
	}

	// Verify it works
	result, err := handler.Execute("test", nil)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

// TestManifestImmutability tests that multiple calls to Manifest return equivalent data.
func TestManifestImmutability(t *testing.T) {
	m1 := Manifest()
	m2 := Manifest()

	// Both should be non-nil
	if m1 == nil || m2 == nil {
		t.Fatal("expected non-nil manifests")
	}

	// Should have same values (but may be different objects)
	if m1.PluginID != m2.PluginID {
		t.Errorf("PluginID mismatch: '%s' vs '%s'", m1.PluginID, m2.PluginID)
	}
	if m1.Version != m2.Version {
		t.Errorf("Version mismatch: '%s' vs '%s'", m1.Version, m2.Version)
	}
	if m1.Kind != m2.Kind {
		t.Errorf("Kind mismatch: '%s' vs '%s'", m1.Kind, m2.Kind)
	}
	if m1.Entrypoint != m2.Entrypoint {
		t.Errorf("Entrypoint mismatch: '%s' vs '%s'", m1.Entrypoint, m2.Entrypoint)
	}
}

// TestHandlerZeroValue tests that a zero value Handler still works correctly.
func TestHandlerZeroValue(t *testing.T) {
	var handler Handler

	result, err := handler.Execute("test", nil)

	if err == nil {
		t.Error("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

// TestConcurrentExecution tests that Execute can be called concurrently.
func TestConcurrentExecution(t *testing.T) {
	handler := &Handler{}
	done := make(chan bool)

	// Run multiple goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			result, err := handler.Execute("concurrent-test", map[string]interface{}{"id": id})
			if err == nil {
				t.Errorf("goroutine %d: expected error, got nil", id)
			}
			if result != nil {
				t.Errorf("goroutine %d: expected nil result, got %v", id, result)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestPluginRegisteredByDefault tests that Register() properly registers the plugin.
func TestPluginRegisteredByDefault(t *testing.T) {
	// Clear and re-register to test registration behavior
	plugins.ClearEmbeddedRegistry()
	defer plugins.ClearEmbeddedRegistry()

	// Register the plugin
	Register()

	// Check that it's registered
	if !plugins.HasEmbeddedPlugin("tool.calibre") {
		t.Error("expected plugin to be registered after calling Register")
	}

	plugin := plugins.GetEmbeddedPlugin("tool.calibre")
	if plugin == nil {
		t.Fatal("expected to retrieve calibre plugin")
	}

	if plugin.Manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	if plugin.Tool == nil {
		t.Fatal("expected non-nil tool handler")
	}
}
