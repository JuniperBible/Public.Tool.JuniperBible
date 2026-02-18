// Package plugins provides plugin loading and management for Juniper Bible.
// This file defines the embedded plugin interface for plugins that are
// compiled directly into the binary instead of running as subprocesses.
package plugins

// EmbeddedFormatHandler defines the interface for embedded format plugins.
// This mirrors the IPC commands that format plugins handle.
type EmbeddedFormatHandler interface {
	// Detect checks if the given path is handled by this format.
	Detect(path string) (*DetectResult, error)

	// Ingest ingests a file into content-addressed blobs.
	Ingest(path, outputDir string) (*IngestResult, error)

	// Enumerate lists the contents of a file/archive.
	Enumerate(path string) (*EnumerateResult, error)

	// ExtractIR extracts the intermediate representation from a file.
	ExtractIR(path, outputDir string) (*ExtractIRResult, error)

	// EmitNative converts IR back to native format.
	EmitNative(irPath, outputDir string) (*EmitNativeResult, error)
}

// EmbeddedToolHandler defines the interface for embedded tool plugins.
type EmbeddedToolHandler interface {
	// Execute runs a tool command with the given arguments.
	Execute(command string, args map[string]interface{}) (interface{}, error)
}

// EmbeddedPlugin wraps an embedded handler with its manifest.
type EmbeddedPlugin struct {
	Manifest *PluginManifest
	Format   EmbeddedFormatHandler // Non-nil for format plugins
	Tool     EmbeddedToolHandler   // Non-nil for tool plugins
}

// embeddedRegistry holds all embedded plugins.
var embeddedRegistry = make(map[string]*EmbeddedPlugin)

// RegisterEmbeddedPlugin registers an embedded plugin by its plugin ID.
func RegisterEmbeddedPlugin(p *EmbeddedPlugin) {
	if p.Manifest != nil && p.Manifest.PluginID != "" {
		embeddedRegistry[p.Manifest.PluginID] = p
	}
}

// GetEmbeddedPlugin returns an embedded plugin by ID, or nil if not found.
func GetEmbeddedPlugin(id string) *EmbeddedPlugin {
	return embeddedRegistry[id]
}

// ListEmbeddedPlugins returns all registered embedded plugins.
func ListEmbeddedPlugins() []*EmbeddedPlugin {
	result := make([]*EmbeddedPlugin, 0, len(embeddedRegistry))
	for _, p := range embeddedRegistry {
		result = append(result, p)
	}
	return result
}

// HasEmbeddedPlugin checks if an embedded plugin with the given ID exists.
func HasEmbeddedPlugin(id string) bool {
	_, ok := embeddedRegistry[id]
	return ok
}

// ClearEmbeddedRegistry clears all registered embedded plugins (for testing).
func ClearEmbeddedRegistry() {
	embeddedRegistry = make(map[string]*EmbeddedPlugin)
}

// ExecuteEmbeddedPlugin executes an embedded plugin with the given request.
// Returns nil, nil if the plugin doesn't exist or isn't embedded.
func ExecuteEmbeddedPlugin(pluginID string, req *IPCRequest) (*IPCResponse, error) {
	ep := GetEmbeddedPlugin(pluginID)
	if ep == nil {
		return nil, nil // Not an embedded plugin
	}

	// Handle format plugins
	if ep.Format != nil {
		return executeEmbeddedFormat(ep.Format, req)
	}

	// Handle tool plugins
	if ep.Tool != nil {
		return executeEmbeddedTool(ep.Tool, req)
	}

	return nil, nil
}

func formatResult(result interface{}, err error) (*IPCResponse, error) {
	if err != nil {
		return &IPCResponse{Status: "error", Error: err.Error()}, nil
	}
	return &IPCResponse{Status: "ok", Result: result}, nil
}

func formatCommandHandlers(h EmbeddedFormatHandler, args map[string]interface{}) map[string]func() (*IPCResponse, error) {
	return map[string]func() (*IPCResponse, error){
		"detect": func() (*IPCResponse, error) {
			path, _ := args["path"].(string)
			return formatResult(h.Detect(path))
		},
		"ingest": func() (*IPCResponse, error) {
			path, _ := args["path"].(string)
			outputDir, _ := args["output_dir"].(string)
			return formatResult(h.Ingest(path, outputDir))
		},
		"enumerate": func() (*IPCResponse, error) {
			path, _ := args["path"].(string)
			return formatResult(h.Enumerate(path))
		},
		"extract-ir": func() (*IPCResponse, error) {
			path, _ := args["path"].(string)
			outputDir, _ := args["output_dir"].(string)
			return formatResult(h.ExtractIR(path, outputDir))
		},
		"emit-native": func() (*IPCResponse, error) {
			irPath, _ := args["ir_path"].(string)
			outputDir, _ := args["output_dir"].(string)
			return formatResult(h.EmitNative(irPath, outputDir))
		},
	}
}

func executeEmbeddedFormat(h EmbeddedFormatHandler, req *IPCRequest) (*IPCResponse, error) {
	handlers := formatCommandHandlers(h, req.Args)
	handler, ok := handlers[req.Command]
	if !ok {
		return &IPCResponse{Status: "error", Error: "unknown command: " + req.Command}, nil
	}
	return handler()
}

// executeEmbeddedTool executes a tool plugin request.
func executeEmbeddedTool(h EmbeddedToolHandler, req *IPCRequest) (*IPCResponse, error) {
	result, err := h.Execute(req.Command, req.Args)
	if err != nil {
		return &IPCResponse{Status: "error", Error: err.Error()}, nil
	}
	return &IPCResponse{Status: "ok", Result: result}, nil
}
