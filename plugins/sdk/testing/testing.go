// Package testing provides test utilities for SDK-based plugins.
// This eliminates boilerplate from plugin test files.
package testing

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/ir"
)

// PluginTest provides test helpers for SDK plugins.
type PluginTest struct {
	t          *testing.T
	tempDir    string
	pluginPath string
}

// New creates a new PluginTest instance.
// It automatically creates a temporary directory for test files.
func New(t *testing.T) *PluginTest {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	return &PluginTest{
		t:       t,
		tempDir: tmpDir,
	}
}

// SetPluginPath sets the path to the plugin executable.
// If not set, executePlugin will attempt to build from current directory.
func (pt *PluginTest) SetPluginPath(path string) {
	pt.pluginPath = path
}

// TempDir returns the temporary directory for this test.
func (pt *PluginTest) TempDir() string {
	return pt.tempDir
}

// WriteFile creates a file in the temp directory with the given content.
// Returns the absolute path to the created file.
func (pt *PluginTest) WriteFile(name, content string) string {
	pt.t.Helper()

	path := filepath.Join(pt.tempDir, name)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		pt.t.Fatalf("failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		pt.t.Fatalf("failed to write file %s: %v", path, err)
	}

	return path
}

// Detect runs the detect command on the given path.
func (pt *PluginTest) Detect(path string) *ipc.DetectResult {
	pt.t.Helper()

	req := &ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": path},
	}

	resp := pt.executePlugin(req)
	if resp.Status != "ok" {
		pt.t.Fatalf("detect failed: %s", resp.Error)
	}

	var result ipc.DetectResult
	if err := mapToStruct(resp.Result, &result); err != nil {
		pt.t.Fatalf("failed to parse detect result: %v", err)
	}

	return &result
}

// Ingest runs the ingest command on the given path.
func (pt *PluginTest) Ingest(path string) *ipc.IngestResult {
	pt.t.Helper()

	outputDir := filepath.Join(pt.tempDir, "blobs")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		pt.t.Fatalf("failed to create output dir: %v", err)
	}

	req := &ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       path,
			"output_dir": outputDir,
		},
	}

	resp := pt.executePlugin(req)
	if resp.Status != "ok" {
		pt.t.Fatalf("ingest failed: %s", resp.Error)
	}

	var result ipc.IngestResult
	if err := mapToStruct(resp.Result, &result); err != nil {
		pt.t.Fatalf("failed to parse ingest result: %v", err)
	}

	return &result
}

// ExtractIR runs the extract-ir command on the given path.
// Returns the ExtractIRResult and the loaded Corpus.
func (pt *PluginTest) ExtractIR(path string) (*ipc.ExtractIRResult, *ir.Corpus) {
	pt.t.Helper()

	outputDir := filepath.Join(pt.tempDir, "ir")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		pt.t.Fatalf("failed to create output dir: %v", err)
	}

	req := &ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       path,
			"output_dir": outputDir,
		},
	}

	resp := pt.executePlugin(req)
	if resp.Status != "ok" {
		pt.t.Fatalf("extract-ir failed: %s", resp.Error)
	}

	var result ipc.ExtractIRResult
	if err := mapToStruct(resp.Result, &result); err != nil {
		pt.t.Fatalf("failed to parse extract-ir result: %v", err)
	}

	// Load the corpus
	corpus, err := ir.Read(result.IRPath)
	if err != nil {
		pt.t.Fatalf("failed to read IR from %s: %v", result.IRPath, err)
	}

	return &result, corpus
}

// EmitNative runs the emit-native command on the given IR path.
func (pt *PluginTest) EmitNative(irPath string) *ipc.EmitNativeResult {
	pt.t.Helper()

	outputDir := filepath.Join(pt.tempDir, "output")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		pt.t.Fatalf("failed to create output dir: %v", err)
	}

	req := &ipc.Request{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irPath,
			"output_dir": outputDir,
		},
	}

	resp := pt.executePlugin(req)
	if resp.Status != "ok" {
		pt.t.Fatalf("emit-native failed: %s", resp.Error)
	}

	var result ipc.EmitNativeResult
	if err := mapToStruct(resp.Result, &result); err != nil {
		pt.t.Fatalf("failed to parse emit-native result: %v", err)
	}

	return &result
}

// Enumerate runs the enumerate command on the given path.
func (pt *PluginTest) Enumerate(path string) *ipc.EnumerateResult {
	pt.t.Helper()

	req := &ipc.Request{
		Command: "enumerate",
		Args:    map[string]interface{}{"path": path},
	}

	resp := pt.executePlugin(req)
	if resp.Status != "ok" {
		pt.t.Fatalf("enumerate failed: %s", resp.Error)
	}

	var result ipc.EnumerateResult
	if err := mapToStruct(resp.Result, &result); err != nil {
		pt.t.Fatalf("failed to parse enumerate result: %v", err)
	}

	return &result
}

// AssertDetected asserts that the given path is detected as the expected format.
func (pt *PluginTest) AssertDetected(path, expectedFormat string) {
	pt.t.Helper()

	result := pt.Detect(path)
	if !result.Detected {
		pt.t.Errorf("expected file to be detected as %s, but was not detected", expectedFormat)
	}
	if result.Format != expectedFormat {
		pt.t.Errorf("expected format %s, got %s", expectedFormat, result.Format)
	}
}

// AssertNotDetected asserts that the given path is not detected.
func (pt *PluginTest) AssertNotDetected(path string) {
	pt.t.Helper()

	result := pt.Detect(path)
	if result.Detected {
		pt.t.Errorf("expected file not to be detected, but was detected as %s", result.Format)
	}
}

// AssertRoundTrip performs a round-trip test (extract-ir → emit-native).
// Verifies L0 lossless conversion by comparing file hashes.
func (pt *PluginTest) AssertRoundTrip(path string) {
	pt.t.Helper()

	// Read original
	originalData, err := os.ReadFile(path)
	if err != nil {
		pt.t.Fatalf("failed to read original file: %v", err)
	}
	originalHash := sha256.Sum256(originalData)

	// Extract IR
	extractResult, _ := pt.ExtractIR(path)

	// Emit native
	emitResult := pt.EmitNative(extractResult.IRPath)

	// Read output
	outputData, err := os.ReadFile(emitResult.OutputPath)
	if err != nil {
		pt.t.Fatalf("failed to read output file: %v", err)
	}
	outputHash := sha256.Sum256(outputData)

	// Compare
	if originalHash != outputHash {
		pt.t.Errorf("L0 round-trip failed: hashes differ\noriginal: %s\noutput:   %s",
			hex.EncodeToString(originalHash[:]),
			hex.EncodeToString(outputHash[:]))
	}
}

// resolvePluginPath returns the plugin path, building if needed.
func (pt *PluginTest) resolvePluginPath() string {
	if pt.pluginPath != "" {
		return pt.pluginPath
	}
	pluginName := filepath.Base(pt.t.Name())
	pluginPath := filepath.Join(".", "plugin-"+pluginName)
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
		if err := buildCmd.Run(); err != nil {
			pt.t.Fatalf("failed to build plugin: %v", err)
		}
	}
	return pluginPath
}

// runPluginCommand executes the plugin and returns stdout/stderr.
func (pt *PluginTest) runPluginCommand(pluginPath string, reqData []byte) (*bytes.Buffer, *bytes.Buffer, error) {
	cmd := exec.Command(pluginPath)
	cmd.Stdin = bytes.NewReader(reqData)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return &stdout, &stderr, err
}

// executePlugin executes the plugin with a request and returns the response.
func (pt *PluginTest) executePlugin(req *ipc.Request) *ipc.Response {
	pt.t.Helper()

	pluginPath := pt.resolvePluginPath()
	reqData, err := json.Marshal(req)
	if err != nil {
		pt.t.Fatalf("failed to marshal request: %v", err)
	}

	stdout, stderr, runErr := pt.runPluginCommand(pluginPath, reqData)
	if runErr != nil {
		if resp := pt.tryParseResponse(stdout); resp != nil {
			return resp
		}
		pt.t.Fatalf("plugin execution failed: %v\nstderr: %s", runErr, stderr.String())
	}

	return pt.parseResponse(stdout)
}

// tryParseResponse attempts to parse a response from stdout.
func (pt *PluginTest) tryParseResponse(stdout *bytes.Buffer) *ipc.Response {
	if stdout.Len() == 0 {
		return nil
	}
	var resp ipc.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
		return &resp
	}
	return nil
}

// parseResponse parses the response from stdout.
func (pt *PluginTest) parseResponse(stdout *bytes.Buffer) *ipc.Response {
	var resp ipc.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		pt.t.Fatalf("failed to parse response: %v\noutput: %s", err, stdout.String())
	}
	return &resp
}

// mapToStruct converts a map[string]interface{} to a struct using JSON round-trip.
func mapToStruct(m interface{}, v interface{}) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// ReadFile reads a file from the temp directory.
func (pt *PluginTest) ReadFile(path string) []byte {
	pt.t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		pt.t.Fatalf("failed to read file %s: %v", path, err)
	}
	return data
}

// MkdirAll creates a directory and all parents in the temp directory.
func (pt *PluginTest) MkdirAll(path string) string {
	pt.t.Helper()

	fullPath := filepath.Join(pt.tempDir, path)
	if err := os.MkdirAll(fullPath, 0700); err != nil {
		pt.t.Fatalf("failed to create directory %s: %v", fullPath, err)
	}
	return fullPath
}
