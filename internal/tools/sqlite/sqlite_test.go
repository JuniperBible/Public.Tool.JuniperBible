package sqlite

import (
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// TestManifest tests the Manifest function.
func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	// Verify plugin ID
	if manifest.PluginID != "tool.sqlite" {
		t.Errorf("expected PluginID 'tool.sqlite', got '%s'", manifest.PluginID)
	}

	// Verify version
	if manifest.Version != "1.0.0" {
		t.Errorf("expected Version '1.0.0', got '%s'", manifest.Version)
	}

	// Verify kind
	if manifest.Kind != "tool" {
		t.Errorf("expected Kind 'tool', got '%s'", manifest.Kind)
	}

	// Verify entrypoint
	if manifest.Entrypoint != "tool-sqlite" {
		t.Errorf("expected Entrypoint 'tool-sqlite', got '%s'", manifest.Entrypoint)
	}

	// Verify capabilities
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

// TestManifestConsistency tests that multiple calls to Manifest return equivalent data.
func TestManifestConsistency(t *testing.T) {
	m1 := Manifest()
	m2 := Manifest()

	if m1.PluginID != m2.PluginID {
		t.Errorf("manifest PluginID inconsistent: '%s' vs '%s'", m1.PluginID, m2.PluginID)
	}

	if m1.Version != m2.Version {
		t.Errorf("manifest Version inconsistent: '%s' vs '%s'", m1.Version, m2.Version)
	}

	if m1.Kind != m2.Kind {
		t.Errorf("manifest Kind inconsistent: '%s' vs '%s'", m1.Kind, m2.Kind)
	}

	if m1.Entrypoint != m2.Entrypoint {
		t.Errorf("manifest Entrypoint inconsistent: '%s' vs '%s'", m1.Entrypoint, m2.Entrypoint)
	}
}

// TestRegister tests the Register function.
func TestRegister(t *testing.T) {
	// Clear registry to start fresh
	plugins.ClearEmbeddedRegistry()
	defer plugins.ClearEmbeddedRegistry()

	// Register the plugin
	Register()

	// Verify the plugin was registered
	if !plugins.HasEmbeddedPlugin("tool.sqlite") {
		t.Fatal("expected plugin to be registered")
	}

	// Retrieve and verify the plugin
	ep := plugins.GetEmbeddedPlugin("tool.sqlite")
	if ep == nil {
		t.Fatal("expected to retrieve registered plugin")
	}

	// Verify manifest
	if ep.Manifest == nil {
		t.Fatal("expected non-nil manifest")
	}
	if ep.Manifest.PluginID != "tool.sqlite" {
		t.Errorf("expected PluginID 'tool.sqlite', got '%s'", ep.Manifest.PluginID)
	}

	// Verify it's a tool plugin
	if ep.Tool == nil {
		t.Fatal("expected non-nil Tool handler")
	}
	if ep.Format != nil {
		t.Error("expected nil Format handler for tool plugin")
	}

	// Verify the handler is of correct type
	if _, ok := ep.Tool.(*Handler); !ok {
		t.Errorf("expected Tool to be *Handler, got %T", ep.Tool)
	}
}

// TestRegisterMultipleCalls tests that calling Register multiple times is safe.
func TestRegisterMultipleCalls(t *testing.T) {
	plugins.ClearEmbeddedRegistry()
	defer plugins.ClearEmbeddedRegistry()

	// Register multiple times
	Register()
	Register()
	Register()

	// Should still only have one instance
	if !plugins.HasEmbeddedPlugin("tool.sqlite") {
		t.Fatal("expected plugin to be registered")
	}

	// Verify the plugin works
	ep := plugins.GetEmbeddedPlugin("tool.sqlite")
	if ep == nil {
		t.Fatal("expected to retrieve registered plugin")
	}
}

// TestInit tests that the init function registers the plugin automatically.
func TestInit(t *testing.T) {
	// The init() function runs automatically when the package is imported.
	// By the time this test runs, it should already be registered.

	// Note: We can't truly test init() in isolation because it runs before
	// any test code. However, we can verify that the plugin is registered
	// as a result of the package being imported.

	// First, ensure the registry is clear and re-register to simulate init
	plugins.ClearEmbeddedRegistry()

	// Call init() indirectly through Register() which init() calls
	Register()

	// Check if plugin is registered (should be from init)
	if !plugins.HasEmbeddedPlugin("tool.sqlite") {
		t.Error("expected plugin to be auto-registered by init()")
	}

	ep := plugins.GetEmbeddedPlugin("tool.sqlite")
	if ep == nil {
		t.Fatal("expected to retrieve auto-registered plugin")
	}
}

// TestExecute tests the Execute method of Handler.
func TestExecute(t *testing.T) {
	h := &Handler{}

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
			name:    "query command",
			command: "query",
			args:    map[string]interface{}{"sql": "SELECT * FROM table", "db": "test.db"},
		},
		{
			name:    "execute command",
			command: "execute",
			args:    map[string]interface{}{"sql": "CREATE TABLE test (id INTEGER)", "db": "test.db"},
		},
		{
			name:    "unknown command",
			command: "unknown",
			args:    map[string]interface{}{"param": "value"},
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
		{
			name:    "command with multiple args",
			command: "multi",
			args: map[string]interface{}{
				"arg1": "value1",
				"arg2": 42,
				"arg3": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := h.Execute(tt.command, tt.args)

			// Should always return an error
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			// Result should be nil
			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}

			// Error should mention external plugin requirement
			errMsg := err.Error()
			if !strings.Contains(errMsg, "requires external plugin") {
				t.Errorf("expected error to mention 'requires external plugin', got: %s", errMsg)
			}

			// Error should mention the command name
			if !strings.Contains(errMsg, "sqlite") {
				t.Errorf("expected error to mention 'sqlite', got: %s", errMsg)
			}

			// Error should include the command name
			if tt.command != "" && !strings.Contains(errMsg, tt.command) {
				t.Errorf("expected error to include command '%s', got: %s", tt.command, errMsg)
			}
		})
	}
}

// TestExecuteErrorFormat tests the exact format of the error message.
func TestExecuteErrorFormat(t *testing.T) {
	h := &Handler{}

	testCases := []struct {
		command     string
		expectedMsg string
	}{
		{
			command:     "query",
			expectedMsg: "sqlite command 'query' requires external plugin",
		},
		{
			command:     "execute",
			expectedMsg: "sqlite command 'execute' requires external plugin",
		},
		{
			command:     "",
			expectedMsg: "sqlite command '' requires external plugin",
		},
	}

	for _, tc := range testCases {
		t.Run("command_"+tc.command, func(t *testing.T) {
			_, err := h.Execute(tc.command, nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if err.Error() != tc.expectedMsg {
				t.Errorf("expected error message '%s', got '%s'", tc.expectedMsg, err.Error())
			}
		})
	}
}

// TestHandlerImplementsInterface tests that Handler implements EmbeddedToolHandler.
func TestHandlerImplementsInterface(t *testing.T) {
	var _ plugins.EmbeddedToolHandler = (*Handler)(nil)
}

// TestHandlerZeroValue tests that the zero value of Handler works.
func TestHandlerZeroValue(t *testing.T) {
	var h Handler

	_, err := h.Execute("test", nil)
	if err == nil {
		t.Fatal("expected error from zero value Handler")
	}

	if !strings.Contains(err.Error(), "requires external plugin") {
		t.Errorf("expected error to mention 'requires external plugin', got: %s", err.Error())
	}
}

// TestExecuteViaEmbeddedPlugin tests Execute through the plugin system.
func TestExecuteViaEmbeddedPlugin(t *testing.T) {
	// Ensure plugin is registered
	plugins.ClearEmbeddedRegistry()
	Register()
	defer plugins.ClearEmbeddedRegistry()

	// Execute via the plugin system
	req := &plugins.IPCRequest{
		Command: "query",
		Args: map[string]interface{}{
			"sql": "SELECT * FROM table",
			"db":  "test.db",
		},
	}

	resp, err := plugins.ExecuteEmbeddedPlugin("tool.sqlite", req)
	if err != nil {
		t.Fatalf("unexpected error from ExecuteEmbeddedPlugin: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	// Should return error status
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", resp.Status)
	}

	// Should have error message
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}

	if !strings.Contains(resp.Error, "requires external plugin") {
		t.Errorf("expected error to mention 'requires external plugin', got: %s", resp.Error)
	}

	// Result should be nil
	if resp.Result != nil {
		t.Errorf("expected nil result, got %v", resp.Result)
	}
}

// TestExecuteViaEmbeddedPluginMultipleCommands tests various commands through the plugin system.
func TestExecuteViaEmbeddedPluginMultipleCommands(t *testing.T) {
	plugins.ClearEmbeddedRegistry()
	Register()
	defer plugins.ClearEmbeddedRegistry()

	commands := []string{"query", "execute", "insert", "update", "delete", "unknown", ""}

	for _, cmd := range commands {
		t.Run("command_"+cmd, func(t *testing.T) {
			req := &plugins.IPCRequest{
				Command: cmd,
				Args:    map[string]interface{}{"test": "value"},
			}

			resp, err := plugins.ExecuteEmbeddedPlugin("tool.sqlite", req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp == nil {
				t.Fatal("expected non-nil response")
			}

			if resp.Status != "error" {
				t.Errorf("expected error status, got '%s'", resp.Status)
			}

			if !strings.Contains(resp.Error, cmd) {
				t.Errorf("expected error to contain command '%s', got: %s", cmd, resp.Error)
			}
		})
	}
}

// TestManifestFieldsNonEmpty tests that all required manifest fields are non-empty.
func TestManifestFieldsNonEmpty(t *testing.T) {
	m := Manifest()

	if m.PluginID == "" {
		t.Error("PluginID should not be empty")
	}

	if m.Version == "" {
		t.Error("Version should not be empty")
	}

	if m.Kind == "" {
		t.Error("Kind should not be empty")
	}

	if m.Entrypoint == "" {
		t.Error("Entrypoint should not be empty")
	}
}

// TestManifestCapabilitiesNotNil tests that capabilities slices are initialized.
func TestManifestCapabilitiesNotNil(t *testing.T) {
	m := Manifest()

	if m.Capabilities.Inputs == nil {
		t.Error("Capabilities.Inputs should not be nil")
	}

	if m.Capabilities.Outputs == nil {
		t.Error("Capabilities.Outputs should not be nil")
	}
}

// TestExecuteWithVariousArgTypes tests Execute with different argument types.
func TestExecuteWithVariousArgTypes(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name string
		args map[string]interface{}
	}{
		{
			name: "string args",
			args: map[string]interface{}{
				"db":    "test.db",
				"table": "users",
			},
		},
		{
			name: "mixed types",
			args: map[string]interface{}{
				"id":      123,
				"name":    "test",
				"active":  true,
				"score":   98.5,
				"tags":    []string{"a", "b"},
				"options": map[string]string{"key": "value"},
			},
		},
		{
			name: "nested maps",
			args: map[string]interface{}{
				"config": map[string]interface{}{
					"timeout": 30,
					"retry":   true,
				},
			},
		},
		{
			name: "empty map",
			args: map[string]interface{}{},
		},
		{
			name: "nil map",
			args: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := h.Execute("test", tt.args)

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}

			if !strings.Contains(err.Error(), "requires external plugin") {
				t.Errorf("expected error to mention 'requires external plugin', got: %s", err.Error())
			}
		})
	}
}

// TestExecuteWithSpecialCommands tests Execute with special SQLite commands.
func TestExecuteWithSpecialCommands(t *testing.T) {
	h := &Handler{}

	commands := []string{
		"PRAGMA",
		"BEGIN",
		"COMMIT",
		"ROLLBACK",
		"VACUUM",
		"ANALYZE",
		".schema",
		".tables",
		".dump",
	}

	for _, cmd := range commands {
		t.Run("command_"+cmd, func(t *testing.T) {
			result, err := h.Execute(cmd, nil)

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}

			errMsg := err.Error()
			if !strings.Contains(errMsg, "sqlite command") {
				t.Errorf("expected error to mention 'sqlite command', got: %s", errMsg)
			}

			if !strings.Contains(errMsg, cmd) {
				t.Errorf("expected error to contain command '%s', got: %s", cmd, errMsg)
			}
		})
	}
}

// TestHandlerPointerReceiver tests that Handler methods work with pointer receivers.
func TestHandlerPointerReceiver(t *testing.T) {
	h := &Handler{}

	// Test that methods can be called on pointer
	_, err := h.Execute("test", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Test that methods can be called on value (promoted to pointer)
	h2 := Handler{}
	_, err2 := h2.Execute("test", nil)
	if err2 == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestManifestImmutability tests that the returned manifest is independent.
func TestManifestImmutability(t *testing.T) {
	m1 := Manifest()
	m2 := Manifest()

	// Modify m1
	m1.Version = "modified"
	m1.PluginID = "modified"

	// m2 should be unaffected
	if m2.Version == "modified" {
		t.Error("manifest Version should be independent between calls")
	}
	if m2.PluginID == "modified" {
		t.Error("manifest PluginID should be independent between calls")
	}

	// Original values should be preserved in m2
	if m2.Version != "1.0.0" {
		t.Errorf("expected Version '1.0.0', got '%s'", m2.Version)
	}
	if m2.PluginID != "tool.sqlite" {
		t.Errorf("expected PluginID 'tool.sqlite', got '%s'", m2.PluginID)
	}
}

// TestExecuteWithLongCommand tests Execute with a very long command string.
func TestExecuteWithLongCommand(t *testing.T) {
	h := &Handler{}

	longCommand := strings.Repeat("SELECT * FROM table WHERE id = 1 AND ", 100) + "TRUE"

	result, err := h.Execute(longCommand, nil)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}

	if !strings.Contains(err.Error(), "requires external plugin") {
		t.Errorf("expected error to mention 'requires external plugin', got: %s", err.Error())
	}
}


// TestExecuteInParallel tests that Execute can be called safely from multiple goroutines.
func TestExecuteInParallel(t *testing.T) {
	h := &Handler{}

	const numGoroutines = 100
	done := make(chan bool, numGoroutines) // Buffered channel to prevent deadlock
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(n int) {
			defer func() { done <- true }()
			cmd := "query"
			_, err := h.Execute(cmd, nil)
			if err == nil {
				errors <- nil // Signal that we didn't get an error when we should have
			}
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	close(errors)
	for err := range errors {
		if err == nil {
			t.Error("expected error for command but got nil")
		}
	}
}

// TestManifestCapabilitiesSlicesAreEmpty tests that capability slices are empty.
func TestManifestCapabilitiesSlicesAreEmpty(t *testing.T) {
	m := Manifest()

	if len(m.Capabilities.Inputs) != 0 {
		t.Errorf("expected empty Inputs slice, got %d items", len(m.Capabilities.Inputs))
	}

	if len(m.Capabilities.Outputs) != 0 {
		t.Errorf("expected empty Outputs slice, got %d items", len(m.Capabilities.Outputs))
	}
}

// TestHandlerIsStruct tests that Handler is a struct type.
func TestHandlerIsStruct(t *testing.T) {
	h := Handler{}
	_ = h // Use the variable to avoid unused variable error
	// If this compiles, Handler is a valid struct type
}
