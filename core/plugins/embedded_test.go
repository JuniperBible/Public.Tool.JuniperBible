package plugins

import (
	"errors"
	"testing"
)

// mockFormatHandler is a mock implementation of EmbeddedFormatHandler for testing.
type mockFormatHandler struct {
	detectErr     error
	ingestErr     error
	enumerateErr  error
	extractIRErr  error
	emitNativeErr error
}

func (m *mockFormatHandler) Detect(path string) (*DetectResult, error) {
	if m.detectErr != nil {
		return nil, m.detectErr
	}
	return &DetectResult{
		Detected: true,
		Format:   "mock.format",
		Reason:   "test detection",
	}, nil
}

func (m *mockFormatHandler) Ingest(path, outputDir string) (*IngestResult, error) {
	if m.ingestErr != nil {
		return nil, m.ingestErr
	}
	return &IngestResult{
		ArtifactID: "artifact123",
		BlobSHA256: "sha256hash",
		SizeBytes:  1024,
	}, nil
}

func (m *mockFormatHandler) Enumerate(path string) (*EnumerateResult, error) {
	if m.enumerateErr != nil {
		return nil, m.enumerateErr
	}
	return &EnumerateResult{
		Entries: []EnumerateEntry{{Path: "test.txt", SizeBytes: 100}},
	}, nil
}

func (m *mockFormatHandler) ExtractIR(path, outputDir string) (*ExtractIRResult, error) {
	if m.extractIRErr != nil {
		return nil, m.extractIRErr
	}
	return &ExtractIRResult{
		IRPath:    outputDir + "/ir.json",
		LossClass: "lossless",
	}, nil
}

func (m *mockFormatHandler) EmitNative(irPath, outputDir string) (*EmitNativeResult, error) {
	if m.emitNativeErr != nil {
		return nil, m.emitNativeErr
	}
	return &EmitNativeResult{
		OutputPath: outputDir + "/output.native",
		Format:     "mock",
	}, nil
}

// mockToolHandler is a mock implementation of EmbeddedToolHandler for testing.
type mockToolHandler struct {
	executeErr error
	result     interface{}
}

func (m *mockToolHandler) Execute(command string, args map[string]interface{}) (interface{}, error) {
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	if m.result != nil {
		return m.result, nil
	}
	return map[string]interface{}{"command": command, "executed": true}, nil
}

// TestRegisterEmbeddedPlugin tests the RegisterEmbeddedPlugin function.
func TestRegisterEmbeddedPlugin(t *testing.T) {
	// Clear registry before test
	ClearEmbeddedRegistry()
	defer ClearEmbeddedRegistry()

	// Register a format plugin
	formatPlugin := &EmbeddedPlugin{
		Manifest: &PluginManifest{
			PluginID: "test.format",
			Version:  "1.0.0",
			Kind:     "format",
		},
		Format: &mockFormatHandler{},
	}
	RegisterEmbeddedPlugin(formatPlugin)

	// Verify it was registered
	retrieved := GetEmbeddedPlugin("test.format")
	if retrieved == nil {
		t.Fatal("expected to retrieve registered plugin")
	}
	if retrieved.Manifest.PluginID != "test.format" {
		t.Errorf("expected plugin ID 'test.format', got '%s'", retrieved.Manifest.PluginID)
	}

	// Register a tool plugin
	toolPlugin := &EmbeddedPlugin{
		Manifest: &PluginManifest{
			PluginID: "test.tool",
			Version:  "1.0.0",
			Kind:     "tool",
		},
		Tool: &mockToolHandler{},
	}
	RegisterEmbeddedPlugin(toolPlugin)

	// Verify it was registered
	retrieved = GetEmbeddedPlugin("test.tool")
	if retrieved == nil {
		t.Fatal("expected to retrieve registered tool plugin")
	}
	if retrieved.Manifest.PluginID != "test.tool" {
		t.Errorf("expected plugin ID 'test.tool', got '%s'", retrieved.Manifest.PluginID)
	}

	// Test registering plugin with nil manifest (should not panic, just be a no-op)
	nilManifest := &EmbeddedPlugin{
		Manifest: nil,
		Format:   &mockFormatHandler{},
	}
	RegisterEmbeddedPlugin(nilManifest)

	// Test registering plugin with empty plugin ID (should be a no-op)
	emptyID := &EmbeddedPlugin{
		Manifest: &PluginManifest{
			PluginID: "",
			Version:  "1.0.0",
		},
		Format: &mockFormatHandler{},
	}
	RegisterEmbeddedPlugin(emptyID)
}

// TestHasEmbeddedPlugin tests the HasEmbeddedPlugin function.
func TestHasEmbeddedPlugin(t *testing.T) {
	ClearEmbeddedRegistry()
	defer ClearEmbeddedRegistry()

	// Should not have any plugins initially
	if HasEmbeddedPlugin("nonexistent") {
		t.Error("expected HasEmbeddedPlugin to return false for nonexistent plugin")
	}

	// Register a plugin
	RegisterEmbeddedPlugin(&EmbeddedPlugin{
		Manifest: &PluginManifest{
			PluginID: "test.exists",
			Version:  "1.0.0",
		},
	})

	// Should now exist
	if !HasEmbeddedPlugin("test.exists") {
		t.Error("expected HasEmbeddedPlugin to return true for registered plugin")
	}

	// Should still not exist for other IDs
	if HasEmbeddedPlugin("test.other") {
		t.Error("expected HasEmbeddedPlugin to return false for different ID")
	}
}

// TestClearEmbeddedRegistry tests the ClearEmbeddedRegistry function.
func TestClearEmbeddedRegistry(t *testing.T) {
	ClearEmbeddedRegistry()

	// Register multiple plugins
	for i := 0; i < 5; i++ {
		RegisterEmbeddedPlugin(&EmbeddedPlugin{
			Manifest: &PluginManifest{
				PluginID: "test.plugin" + string(rune('0'+i)),
				Version:  "1.0.0",
			},
		})
	}

	// Verify plugins were registered
	list := ListEmbeddedPlugins()
	if len(list) != 5 {
		t.Fatalf("expected 5 plugins, got %d", len(list))
	}

	// Clear registry
	ClearEmbeddedRegistry()

	// Verify registry is empty
	list = ListEmbeddedPlugins()
	if len(list) != 0 {
		t.Errorf("expected 0 plugins after clear, got %d", len(list))
	}

	// Verify HasEmbeddedPlugin returns false for all
	for i := 0; i < 5; i++ {
		if HasEmbeddedPlugin("test.plugin" + string(rune('0'+i))) {
			t.Errorf("expected plugin %d to not exist after clear", i)
		}
	}
}

// TestListEmbeddedPlugins tests the ListEmbeddedPlugins function.
func TestListEmbeddedPlugins(t *testing.T) {
	ClearEmbeddedRegistry()
	defer ClearEmbeddedRegistry()

	// Empty registry should return empty list
	list := ListEmbeddedPlugins()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}

	// Register plugins
	plugins := []string{"format.a", "format.b", "tool.c"}
	for _, id := range plugins {
		RegisterEmbeddedPlugin(&EmbeddedPlugin{
			Manifest: &PluginManifest{
				PluginID: id,
				Version:  "1.0.0",
			},
		})
	}

	// List should have all plugins
	list = ListEmbeddedPlugins()
	if len(list) != len(plugins) {
		t.Errorf("expected %d plugins, got %d", len(plugins), len(list))
	}

	// Verify all plugins are in the list
	found := make(map[string]bool)
	for _, p := range list {
		found[p.Manifest.PluginID] = true
	}
	for _, id := range plugins {
		if !found[id] {
			t.Errorf("plugin %s not found in list", id)
		}
	}
}

// TestExecuteEmbeddedPluginNotFound tests ExecuteEmbeddedPlugin with non-existent plugin.
func TestExecuteEmbeddedPluginNotFound(t *testing.T) {
	ClearEmbeddedRegistry()
	defer ClearEmbeddedRegistry()

	resp, err := ExecuteEmbeddedPlugin("nonexistent", &IPCRequest{Command: "detect"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response for non-existent plugin, got %v", resp)
	}
}

// TestExecuteEmbeddedPluginFormat tests ExecuteEmbeddedPlugin with a format plugin.
func TestExecuteEmbeddedPluginFormat(t *testing.T) {
	ClearEmbeddedRegistry()
	defer ClearEmbeddedRegistry()

	RegisterEmbeddedPlugin(&EmbeddedPlugin{
		Manifest: &PluginManifest{
			PluginID: "test.format",
			Version:  "1.0.0",
			Kind:     "format",
		},
		Format: &mockFormatHandler{},
	})

	// Test detect command
	resp, err := ExecuteEmbeddedPlugin("test.format", &IPCRequest{
		Command: "detect",
		Args:    map[string]interface{}{"path": "/test/path"},
	})
	if err != nil {
		t.Fatalf("detect error: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}
}

// TestExecuteEmbeddedPluginTool tests ExecuteEmbeddedPlugin with a tool plugin.
func TestExecuteEmbeddedPluginTool(t *testing.T) {
	ClearEmbeddedRegistry()
	defer ClearEmbeddedRegistry()

	RegisterEmbeddedPlugin(&EmbeddedPlugin{
		Manifest: &PluginManifest{
			PluginID: "test.tool",
			Version:  "1.0.0",
			Kind:     "tool",
		},
		Tool: &mockToolHandler{},
	})

	resp, err := ExecuteEmbeddedPlugin("test.tool", &IPCRequest{
		Command: "run",
		Args:    map[string]interface{}{"input": "test"},
	})
	if err != nil {
		t.Fatalf("tool execution error: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}
}

// TestExecuteEmbeddedPluginNoHandler tests ExecuteEmbeddedPlugin with no handler.
func TestExecuteEmbeddedPluginNoHandler(t *testing.T) {
	ClearEmbeddedRegistry()
	defer ClearEmbeddedRegistry()

	// Register a plugin with neither Format nor Tool handler
	RegisterEmbeddedPlugin(&EmbeddedPlugin{
		Manifest: &PluginManifest{
			PluginID: "test.empty",
			Version:  "1.0.0",
		},
		Format: nil,
		Tool:   nil,
	})

	resp, err := ExecuteEmbeddedPlugin("test.empty", &IPCRequest{Command: "detect"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response for plugin with no handler, got %v", resp)
	}
}

// TestExecuteEmbeddedFormatAllCommands tests executeEmbeddedFormat with all commands.
func TestExecuteEmbeddedFormatAllCommands(t *testing.T) {
	ClearEmbeddedRegistry()
	defer ClearEmbeddedRegistry()

	handler := &mockFormatHandler{}
	RegisterEmbeddedPlugin(&EmbeddedPlugin{
		Manifest: &PluginManifest{
			PluginID: "test.format.all",
			Version:  "1.0.0",
			Kind:     "format",
		},
		Format: handler,
	})

	tests := []struct {
		name    string
		command string
		args    map[string]interface{}
		wantOK  bool
	}{
		{
			name:    "detect",
			command: "detect",
			args:    map[string]interface{}{"path": "/test/path"},
			wantOK:  true,
		},
		{
			name:    "ingest",
			command: "ingest",
			args:    map[string]interface{}{"path": "/test/path", "output_dir": "/output"},
			wantOK:  true,
		},
		{
			name:    "enumerate",
			command: "enumerate",
			args:    map[string]interface{}{"path": "/test/path"},
			wantOK:  true,
		},
		{
			name:    "extract-ir",
			command: "extract-ir",
			args:    map[string]interface{}{"path": "/test/path", "output_dir": "/output"},
			wantOK:  true,
		},
		{
			name:    "emit-native",
			command: "emit-native",
			args:    map[string]interface{}{"ir_path": "/test/ir", "output_dir": "/output"},
			wantOK:  true,
		},
		{
			name:    "unknown command",
			command: "unknown",
			args:    nil,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ExecuteEmbeddedPlugin("test.format.all", &IPCRequest{
				Command: tt.command,
				Args:    tt.args,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantOK && resp.Status != "ok" {
				t.Errorf("expected status 'ok', got '%s': %s", resp.Status, resp.Error)
			}
			if !tt.wantOK && resp.Status != "error" {
				t.Errorf("expected status 'error', got '%s'", resp.Status)
			}
		})
	}
}

// TestExecuteEmbeddedFormatErrors tests executeEmbeddedFormat with handler errors.
func TestExecuteEmbeddedFormatErrors(t *testing.T) {
	ClearEmbeddedRegistry()
	defer ClearEmbeddedRegistry()

	testErr := errors.New("test error")

	tests := []struct {
		name    string
		command string
		handler *mockFormatHandler
	}{
		{
			name:    "detect error",
			command: "detect",
			handler: &mockFormatHandler{detectErr: testErr},
		},
		{
			name:    "ingest error",
			command: "ingest",
			handler: &mockFormatHandler{ingestErr: testErr},
		},
		{
			name:    "enumerate error",
			command: "enumerate",
			handler: &mockFormatHandler{enumerateErr: testErr},
		},
		{
			name:    "extract-ir error",
			command: "extract-ir",
			handler: &mockFormatHandler{extractIRErr: testErr},
		},
		{
			name:    "emit-native error",
			command: "emit-native",
			handler: &mockFormatHandler{emitNativeErr: testErr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ClearEmbeddedRegistry()
			RegisterEmbeddedPlugin(&EmbeddedPlugin{
				Manifest: &PluginManifest{
					PluginID: "test.format.err",
					Version:  "1.0.0",
					Kind:     "format",
				},
				Format: tt.handler,
			})

			resp, err := ExecuteEmbeddedPlugin("test.format.err", &IPCRequest{
				Command: tt.command,
				Args:    map[string]interface{}{"path": "/test", "output_dir": "/out", "ir_path": "/ir"},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Status != "error" {
				t.Errorf("expected status 'error', got '%s'", resp.Status)
			}
			if resp.Error != "test error" {
				t.Errorf("expected error message 'test error', got '%s'", resp.Error)
			}
		})
	}
}

// TestExecuteEmbeddedToolError tests executeEmbeddedTool with handler error.
func TestExecuteEmbeddedToolError(t *testing.T) {
	ClearEmbeddedRegistry()
	defer ClearEmbeddedRegistry()

	RegisterEmbeddedPlugin(&EmbeddedPlugin{
		Manifest: &PluginManifest{
			PluginID: "test.tool.err",
			Version:  "1.0.0",
			Kind:     "tool",
		},
		Tool: &mockToolHandler{executeErr: errors.New("tool error")},
	})

	resp, err := ExecuteEmbeddedPlugin("test.tool.err", &IPCRequest{
		Command: "run",
		Args:    map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", resp.Status)
	}
	if resp.Error != "tool error" {
		t.Errorf("expected error message 'tool error', got '%s'", resp.Error)
	}
}

// TestExecuteEmbeddedToolWithResult tests executeEmbeddedTool with custom result.
func TestExecuteEmbeddedToolWithResult(t *testing.T) {
	ClearEmbeddedRegistry()
	defer ClearEmbeddedRegistry()

	customResult := map[string]interface{}{"custom": "result", "value": 42}
	RegisterEmbeddedPlugin(&EmbeddedPlugin{
		Manifest: &PluginManifest{
			PluginID: "test.tool.result",
			Version:  "1.0.0",
			Kind:     "tool",
		},
		Tool: &mockToolHandler{result: customResult},
	})

	resp, err := ExecuteEmbeddedPlugin("test.tool.result", &IPCRequest{
		Command: "run",
		Args:    map[string]interface{}{"key": "value"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}
	if resp.Result == nil {
		t.Error("expected non-nil result")
	}
}
