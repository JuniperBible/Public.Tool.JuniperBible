package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// IPCRequest is the JSON request sent to plugins.
type IPCRequest struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args,omitempty"`
}

// IPCResponse is the JSON response from plugins.
type IPCResponse struct {
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// DetectResult is the result of a detect command.
type DetectResult struct {
	Detected bool   `json:"detected"`
	Format   string `json:"format,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// IngestResult is the result of an ingest command.
type IngestResult struct {
	ArtifactID string            `json:"artifact_id"`
	BlobSHA256 string            `json:"blob_sha256"`
	SizeBytes  int64             `json:"size_bytes"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// EnumerateResult is the result of an enumerate command.
type EnumerateResult struct {
	Entries []EnumerateEntry `json:"entries"`
}

// EnumerateEntry represents a file entry in an archive.
type EnumerateEntry struct {
	Path      string            `json:"path"`
	SizeBytes int64             `json:"size_bytes"`
	IsDir     bool              `json:"is_dir"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// EngineSpecResult is the result of an engine-spec command.
type EngineSpecResult struct {
	EngineType string   `json:"engine_type"`
	NixFlake   string   `json:"nix_flake,omitempty"`
	Packages   []string `json:"packages,omitempty"`
}

// ExtractIRResult is the result of an extract-ir command.
type ExtractIRResult struct {
	IRPath     string         `json:"ir_path"`
	LossClass  string         `json:"loss_class,omitempty"`
	LossReport *LossReportIPC `json:"loss_report,omitempty"`
}

// EmitNativeResult is the result of an emit-native command.
type EmitNativeResult struct {
	OutputPath string         `json:"output_path"`
	Format     string         `json:"format"`
	LossClass  string         `json:"loss_class,omitempty"`
	LossReport *LossReportIPC `json:"loss_report,omitempty"`
}

// LossReportIPC represents loss information in IPC messages.
type LossReportIPC struct {
	SourceFormat string           `json:"source_format"`
	TargetFormat string           `json:"target_format"`
	LossClass    string           `json:"loss_class"`
	LostElements []LostElementIPC `json:"lost_elements,omitempty"`
	Warnings     []string         `json:"warnings,omitempty"`
}

// LostElementIPC represents a lost element in IPC messages.
type LostElementIPC struct {
	Path          string      `json:"path"`
	ElementType   string      `json:"element_type"`
	Reason        string      `json:"reason"`
	OriginalValue interface{} `json:"original_value,omitempty"`
}

// DefaultTimeout is the default timeout for plugin execution.
const DefaultTimeout = 60 * time.Second

// externalPluginsEnabled controls whether external plugins can be loaded.
// When false (default), only embedded plugins are used.
var externalPluginsEnabled = false

// EnableExternalPlugins enables loading of external plugins from the filesystem.
// This should be called before loading plugins if external plugin support is needed.
func EnableExternalPlugins() {
	externalPluginsEnabled = true
}

// DisableExternalPlugins disables loading of external plugins.
// Only embedded plugins will be used.
func DisableExternalPlugins() {
	externalPluginsEnabled = false
}

// ExternalPluginsEnabled returns whether external plugins are enabled.
func ExternalPluginsEnabled() bool {
	return externalPluginsEnabled
}

// ExecutePlugin executes a plugin with the given request and returns the response.
// It first tries to use an embedded plugin if available, then falls back to
// external plugin execution if external plugins are enabled.
func ExecutePlugin(plugin *Plugin, req *IPCRequest) (*IPCResponse, error) {
	return ExecutePluginWithTimeout(plugin, req, DefaultTimeout)
}

// ExecutePluginWithTimeout executes a plugin with a timeout.
// Priority: external plugin (when enabled and available) > embedded plugin > external fallback.
// This ensures external plugins override embedded ones when the user has them enabled.
func ExecutePluginWithTimeout(plugin *Plugin, req *IPCRequest, timeout time.Duration) (*IPCResponse, error) {
	entrypoint, hasExternalBinary := resolveEntrypoint(plugin)

	if externalPluginsEnabled && hasExternalBinary {
		return executeExternalPlugin(plugin, req, entrypoint, timeout)
	}

	resp, err := tryEmbeddedWithFallback(plugin, req, entrypoint, hasExternalBinary, timeout)
	if resp != nil || err != nil {
		return resp, err
	}

	return nil, fmt.Errorf("plugin %s is not available as an embedded plugin and no external binary found", plugin.Manifest.PluginID)
}

func resolveEntrypoint(plugin *Plugin) (string, bool) {
	if plugin.Path == "(embedded)" {
		return "", false
	}
	entrypoint := plugin.EntrypointPath()
	_, err := os.Stat(entrypoint)
	return entrypoint, err == nil
}

func tryEmbeddedWithFallback(plugin *Plugin, req *IPCRequest, entrypoint string, hasExternalBinary bool, timeout time.Duration) (*IPCResponse, error) {
	resp, err := ExecuteEmbeddedPlugin(plugin.Manifest.PluginID, req)
	if resp == nil && err == nil {
		if hasExternalBinary {
			return executeExternalPlugin(plugin, req, entrypoint, timeout)
		}
		return nil, nil
	}
	if resp != nil && resp.Status == "error" && isNotImplementedError(resp.Error) && hasExternalBinary {
		return executeExternalPlugin(plugin, req, entrypoint, timeout)
	}
	return resp, err
}

// isNotImplementedError checks if an error message indicates an unimplemented feature.
func isNotImplementedError(msg string) bool {
	return strings.Contains(msg, "requires external plugin") ||
		strings.Contains(msg, "not implemented") ||
		strings.Contains(msg, "not supported")
}

// executeExternalPlugin runs a plugin as an external process.
func executeExternalPlugin(plugin *Plugin, req *IPCRequest, entrypoint string, timeout time.Duration) (*IPCResponse, error) {

	// Use external plugin execution

	// Encode request as JSON
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	// Create context with timeout - this handles cancellation properly
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create command with context - process is killed when context is cancelled
	cmd := exec.CommandContext(ctx, entrypoint)
	cmd.Dir = plugin.Path

	// Set up stdin
	cmd.Stdin = bytes.NewReader(reqData)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command - CommandContext handles process cleanup on timeout
	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("plugin execution timed out after %v", timeout)
	}
	if err != nil {
		return nil, fmt.Errorf("plugin execution failed: %w (stderr: %s)", err, stderr.String())
	}

	// Decode response
	var resp IPCResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w (output: %s)", err, stdout.String())
	}

	return &resp, nil
}

// NewDetectRequest creates a detect request.
func NewDetectRequest(path string) *IPCRequest {
	return &IPCRequest{
		Command: "detect",
		Args: map[string]interface{}{
			"path": path,
		},
	}
}

// NewIngestRequest creates an ingest request.
func NewIngestRequest(path, outputDir string) *IPCRequest {
	return &IPCRequest{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       path,
			"output_dir": outputDir,
		},
	}
}

// NewEnumerateRequest creates an enumerate request.
func NewEnumerateRequest(path string) *IPCRequest {
	return &IPCRequest{
		Command: "enumerate",
		Args: map[string]interface{}{
			"path": path,
		},
	}
}

// NewEngineSpecRequest creates an engine-spec request.
func NewEngineSpecRequest() *IPCRequest {
	return &IPCRequest{
		Command: "engine-spec",
	}
}

// ParseDetectResult parses a detect result from a response.
func ParseDetectResult(resp *IPCResponse) (*DetectResult, error) {
	if resp.Status == "error" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal result: %w", err)
	}

	var result DetectResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse detect result: %w", err)
	}

	return &result, nil
}

// ParseIngestResult parses an ingest result from a response.
func ParseIngestResult(resp *IPCResponse) (*IngestResult, error) {
	if resp.Status == "error" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal result: %w", err)
	}

	var result IngestResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ingest result: %w", err)
	}

	return &result, nil
}

// ParseEnumerateResult parses an enumerate result from a response.
func ParseEnumerateResult(resp *IPCResponse) (*EnumerateResult, error) {
	if resp.Status == "error" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal result: %w", err)
	}

	var result EnumerateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse enumerate result: %w", err)
	}

	return &result, nil
}

// ParseEngineSpecResult parses an engine-spec result from a response.
func ParseEngineSpecResult(resp *IPCResponse) (*EngineSpecResult, error) {
	if resp.Status == "error" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal result: %w", err)
	}

	var result EngineSpecResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse engine-spec result: %w", err)
	}

	return &result, nil
}

// NewExtractIRRequest creates an extract-ir request.
func NewExtractIRRequest(path, outputDir string) *IPCRequest {
	return &IPCRequest{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       path,
			"output_dir": outputDir,
		},
	}
}

// NewEmitNativeRequest creates an emit-native request.
func NewEmitNativeRequest(irPath, outputDir string) *IPCRequest {
	return &IPCRequest{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irPath,
			"output_dir": outputDir,
		},
	}
}

// ParseExtractIRResult parses an extract-ir result from a response.
func ParseExtractIRResult(resp *IPCResponse) (*ExtractIRResult, error) {
	if resp.Status == "error" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal result: %w", err)
	}

	var result ExtractIRResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse extract-ir result: %w", err)
	}

	return &result, nil
}

// ParseEmitNativeResult parses an emit-native result from a response.
func ParseEmitNativeResult(resp *IPCResponse) (*EmitNativeResult, error) {
	if resp.Status == "error" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}

	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal result: %w", err)
	}

	var result EmitNativeResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse emit-native result: %w", err)
	}

	return &result, nil
}
