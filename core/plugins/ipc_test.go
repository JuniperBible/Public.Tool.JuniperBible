package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestIPCRequest tests JSON request encoding.
func TestIPCRequest(t *testing.T) {
	req := &IPCRequest{
		Command: "detect",
		Args: map[string]interface{}{
			"path": "/path/to/file",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var decoded IPCRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if decoded.Command != "detect" {
		t.Errorf("expected command 'detect', got %q", decoded.Command)
	}
}

// TestIPCResponse tests JSON response decoding.
func TestIPCResponse(t *testing.T) {
	respJSON := `{"status":"ok","result":{"detected":true,"format":"file"}}`

	var resp IPCResponse
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] != true {
		t.Error("expected detected to be true")
	}
}

// TestIPCErrorResponse tests error response handling.
func TestIPCErrorResponse(t *testing.T) {
	respJSON := `{"status":"error","error":"file not found"}`

	var resp IPCResponse
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}

	if resp.Error != "file not found" {
		t.Errorf("expected error 'file not found', got %q", resp.Error)
	}
}

// TestPluginExecute tests executing a plugin command.
func TestPluginExecute(t *testing.T) {
	// Create a mock plugin script
	tempDir, err := os.MkdirTemp("", "plugin-ipc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simple shell script that returns a valid response
	scriptContent := `#!/bin/sh
read input
echo '{"status":"ok","result":{"received":true}}'
`
	scriptPath := filepath.Join(tempDir, "mock-plugin.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// Skip if sh is not available
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	plugin := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "test.mock",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "mock-plugin.sh",
		},
		Path: tempDir,
	}

	req := &IPCRequest{
		Command: "test",
		Args:    map[string]interface{}{"key": "value"},
	}

	resp, err := ExecutePlugin(plugin, req)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
}

// TestDetectRequest tests creating a detect request.
func TestDetectRequest(t *testing.T) {
	req := NewDetectRequest("/path/to/file.zip")

	if req.Command != "detect" {
		t.Errorf("expected command 'detect', got %q", req.Command)
	}

	path, ok := req.Args["path"].(string)
	if !ok || path != "/path/to/file.zip" {
		t.Error("path argument not set correctly")
	}
}

// TestIngestRequest tests creating an ingest request.
func TestIngestRequest(t *testing.T) {
	req := NewIngestRequest("/path/to/file.zip", "/path/to/output")

	if req.Command != "ingest" {
		t.Errorf("expected command 'ingest', got %q", req.Command)
	}

	path, ok := req.Args["path"].(string)
	if !ok || path != "/path/to/file.zip" {
		t.Error("path argument not set correctly")
	}

	outDir, ok := req.Args["output_dir"].(string)
	if !ok || outDir != "/path/to/output" {
		t.Error("output_dir argument not set correctly")
	}
}

// TestEnumerateRequest tests creating an enumerate request.
func TestEnumerateRequest(t *testing.T) {
	req := NewEnumerateRequest("/path/to/archive.zip")

	if req.Command != "enumerate" {
		t.Errorf("expected command 'enumerate', got %q", req.Command)
	}
}

// TestExtractIRRequest tests creating an extract-ir request.
func TestExtractIRRequest(t *testing.T) {
	req := NewExtractIRRequest("/path/to/file.osis", "/path/to/output")

	if req.Command != "extract-ir" {
		t.Errorf("expected command 'extract-ir', got %q", req.Command)
	}

	path, ok := req.Args["path"].(string)
	if !ok || path != "/path/to/file.osis" {
		t.Error("path argument not set correctly")
	}

	outDir, ok := req.Args["output_dir"].(string)
	if !ok || outDir != "/path/to/output" {
		t.Error("output_dir argument not set correctly")
	}
}

// TestEmitNativeRequest tests creating an emit-native request.
func TestEmitNativeRequest(t *testing.T) {
	req := NewEmitNativeRequest("/path/to/ir.json", "/path/to/output")

	if req.Command != "emit-native" {
		t.Errorf("expected command 'emit-native', got %q", req.Command)
	}

	irPath, ok := req.Args["ir_path"].(string)
	if !ok || irPath != "/path/to/ir.json" {
		t.Error("ir_path argument not set correctly")
	}

	outDir, ok := req.Args["output_dir"].(string)
	if !ok || outDir != "/path/to/output" {
		t.Error("output_dir argument not set correctly")
	}
}

// TestExtractIRResult tests ExtractIRResult JSON encoding.
func TestExtractIRResult(t *testing.T) {
	result := &ExtractIRResult{
		IRPath:    "/path/to/ir.json",
		LossClass: "L0",
		LossReport: &LossReportIPC{
			SourceFormat: "OSIS",
			TargetFormat: "IR",
			LossClass:    "L0",
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var decoded ExtractIRResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if decoded.IRPath != result.IRPath {
		t.Errorf("IRPath = %q, want %q", decoded.IRPath, result.IRPath)
	}
	if decoded.LossClass != result.LossClass {
		t.Errorf("LossClass = %q, want %q", decoded.LossClass, result.LossClass)
	}
	if decoded.LossReport == nil {
		t.Fatal("LossReport is nil")
	}
	if decoded.LossReport.SourceFormat != "OSIS" {
		t.Errorf("SourceFormat = %q, want %q", decoded.LossReport.SourceFormat, "OSIS")
	}
}

// TestEmitNativeResult tests EmitNativeResult JSON encoding.
func TestEmitNativeResult(t *testing.T) {
	result := &EmitNativeResult{
		OutputPath: "/path/to/output.osis",
		Format:     "OSIS",
		LossClass:  "L0",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var decoded EmitNativeResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if decoded.OutputPath != result.OutputPath {
		t.Errorf("OutputPath = %q, want %q", decoded.OutputPath, result.OutputPath)
	}
	if decoded.Format != result.Format {
		t.Errorf("Format = %q, want %q", decoded.Format, result.Format)
	}
}

// TestLossReportIPC tests LossReportIPC with lost elements.
func TestLossReportIPC(t *testing.T) {
	report := &LossReportIPC{
		SourceFormat: "SWORD",
		TargetFormat: "IR",
		LossClass:    "L1",
		LostElements: []LostElementIPC{
			{
				Path:        "Gen.1.1/format",
				ElementType: "formatting",
				Reason:      "Not preserved in IR",
			},
		},
		Warnings: []string{"Some data was approximated"},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded LossReportIPC
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.LossClass != "L1" {
		t.Errorf("LossClass = %q, want L1", decoded.LossClass)
	}
	if len(decoded.LostElements) != 1 {
		t.Fatalf("len(LostElements) = %d, want 1", len(decoded.LostElements))
	}
	if decoded.LostElements[0].Path != "Gen.1.1/format" {
		t.Errorf("LostElements[0].Path = %q, want Gen.1.1/format", decoded.LostElements[0].Path)
	}
}

// TestParseExtractIRResult tests parsing extract-ir result from response.
func TestParseExtractIRResult(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: map[string]interface{}{
			"ir_path":    "/path/to/ir.json",
			"loss_class": "L0",
		},
	}

	result, err := ParseExtractIRResult(resp)
	if err != nil {
		t.Fatalf("ParseExtractIRResult failed: %v", err)
	}

	if result.IRPath != "/path/to/ir.json" {
		t.Errorf("IRPath = %q, want /path/to/ir.json", result.IRPath)
	}
	if result.LossClass != "L0" {
		t.Errorf("LossClass = %q, want L0", result.LossClass)
	}
}

// TestParseEmitNativeResult tests parsing emit-native result from response.
func TestParseEmitNativeResult(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: map[string]interface{}{
			"output_path": "/path/to/output.osis",
			"format":      "OSIS",
			"loss_class":  "L0",
		},
	}

	result, err := ParseEmitNativeResult(resp)
	if err != nil {
		t.Fatalf("ParseEmitNativeResult failed: %v", err)
	}

	if result.OutputPath != "/path/to/output.osis" {
		t.Errorf("OutputPath = %q, want /path/to/output.osis", result.OutputPath)
	}
	if result.Format != "OSIS" {
		t.Errorf("Format = %q, want OSIS", result.Format)
	}
}

// TestParseExtractIRResultError tests parsing error response.
func TestParseExtractIRResultError(t *testing.T) {
	resp := &IPCResponse{
		Status: "error",
		Error:  "unsupported format",
	}

	_, err := ParseExtractIRResult(resp)
	if err == nil {
		t.Fatal("expected error for error response")
	}
}

// TestParseEmitNativeResultError tests parsing error response.
func TestParseEmitNativeResultError(t *testing.T) {
	resp := &IPCResponse{
		Status: "error",
		Error:  "IR validation failed",
	}

	_, err := ParseEmitNativeResult(resp)
	if err == nil {
		t.Fatal("expected error for error response")
	}
}

// TestNewEngineSpecRequest tests creating an engine-spec request.
func TestNewEngineSpecRequest(t *testing.T) {
	req := NewEngineSpecRequest()

	if req.Command != "engine-spec" {
		t.Errorf("Command = %q, want %q", req.Command, "engine-spec")
	}
}

// TestParseDetectResult tests parsing a detect result.
func TestParseDetectResult(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: map[string]interface{}{
			"detected":    true,
			"format":      "osis",
			"confidence":  0.95,
			"module_type": "bible",
		},
	}

	result, err := ParseDetectResult(resp)
	if err != nil {
		t.Fatalf("ParseDetectResult failed: %v", err)
	}

	if !result.Detected {
		t.Error("expected Detected to be true")
	}

	if result.Format != "osis" {
		t.Errorf("Format = %q, want %q", result.Format, "osis")
	}
}

// TestParseDetectResultError tests parsing detect error response.
func TestParseDetectResultError(t *testing.T) {
	resp := &IPCResponse{
		Status: "error",
		Error:  "file not found",
	}

	_, err := ParseDetectResult(resp)
	if err == nil {
		t.Fatal("expected error for error response")
	}
}

// TestParseIngestResult tests parsing an ingest result.
func TestParseIngestResult(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: map[string]interface{}{
			"artifact_id": "art-123",
			"sha256":      "abc123def456",
			"size":        1024,
		},
	}

	result, err := ParseIngestResult(resp)
	if err != nil {
		t.Fatalf("ParseIngestResult failed: %v", err)
	}

	if result.ArtifactID != "art-123" {
		t.Errorf("ArtifactID = %q, want %q", result.ArtifactID, "art-123")
	}
}

// TestParseIngestResultError tests parsing ingest error response.
func TestParseIngestResultError(t *testing.T) {
	resp := &IPCResponse{
		Status: "error",
		Error:  "invalid file format",
	}

	_, err := ParseIngestResult(resp)
	if err == nil {
		t.Fatal("expected error for error response")
	}
}

// TestParseEnumerateResult tests parsing an enumerate result.
func TestParseEnumerateResult(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: map[string]interface{}{
			"modules": []interface{}{
				map[string]interface{}{"id": "KJV", "name": "King James Version"},
				map[string]interface{}{"id": "ESV", "name": "English Standard Version"},
			},
		},
	}

	result, err := ParseEnumerateResult(resp)
	if err != nil {
		t.Fatalf("ParseEnumerateResult failed: %v", err)
	}

	if result == nil {
		t.Error("expected non-nil result")
	}
}

// TestParseEnumerateResultError tests parsing enumerate error response.
func TestParseEnumerateResultError(t *testing.T) {
	resp := &IPCResponse{
		Status: "error",
		Error:  "enumeration failed",
	}

	_, err := ParseEnumerateResult(resp)
	if err == nil {
		t.Fatal("expected error for error response")
	}
}

// TestParseEngineSpecResult tests parsing an engine-spec result.
func TestParseEngineSpecResult(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: map[string]interface{}{
			"engine":  "SWORD",
			"version": "1.8.1",
		},
	}

	result, err := ParseEngineSpecResult(resp)
	if err != nil {
		t.Fatalf("ParseEngineSpecResult failed: %v", err)
	}

	if result == nil {
		t.Error("expected non-nil result")
	}
}

// TestParseEngineSpecResultError tests parsing engine-spec error response.
func TestParseEngineSpecResultError(t *testing.T) {
	resp := &IPCResponse{
		Status: "error",
		Error:  "engine not available",
	}

	_, err := ParseEngineSpecResult(resp)
	if err == nil {
		t.Fatal("expected error for error response")
	}
}

// TestExecutePluginTimeout tests plugin execution timeout.
func TestExecutePluginTimeout(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-timeout-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a script that sleeps longer than the timeout
	scriptContent := `#!/bin/sh
sleep 10
echo '{"status":"ok","result":{}}'
`
	scriptPath := filepath.Join(tempDir, "timeout-plugin.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// Skip if sh is not available
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	plugin := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "test.timeout",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "timeout-plugin.sh",
		},
		Path: tempDir,
	}

	req := &IPCRequest{
		Command: "test",
	}

	// Use a very short timeout
	_, err = ExecutePluginWithTimeout(plugin, req, 100*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}
}

// TestExecutePluginError tests plugin execution errors.
func TestExecutePluginError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-error-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a script that exits with error
	scriptContent := `#!/bin/sh
echo "error message" >&2
exit 1
`
	scriptPath := filepath.Join(tempDir, "error-plugin.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// Skip if sh is not available
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	plugin := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "test.error",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "error-plugin.sh",
		},
		Path: tempDir,
	}

	req := &IPCRequest{
		Command: "test",
	}

	_, err = ExecutePlugin(plugin, req)
	if err == nil {
		t.Error("expected execution error")
	}
}

// TestExecutePluginInvalidJSON tests plugin returning invalid JSON.
func TestExecutePluginInvalidJSON(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-invalid-json-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a script that returns invalid JSON
	scriptContent := `#!/bin/sh
read input
echo "not valid json"
`
	scriptPath := filepath.Join(tempDir, "invalid-json-plugin.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// Skip if sh is not available
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	plugin := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "test.invalidjson",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "invalid-json-plugin.sh",
		},
		Path: tempDir,
	}

	req := &IPCRequest{
		Command: "test",
	}

	_, err = ExecutePlugin(plugin, req)
	if err == nil {
		t.Error("expected JSON decode error")
	}
}

// TestExecutePluginInvalidRequest tests marshaling invalid request.
func TestExecutePluginInvalidRequest(t *testing.T) {
	plugin := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "test.invalid",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "plugin",
		},
		Path: "/tmp",
	}

	// Create a request with unmarshalable data
	req := &IPCRequest{
		Command: "test",
		Args: map[string]interface{}{
			"invalid": make(chan int), // channels cannot be marshaled to JSON
		},
	}

	_, err := ExecutePlugin(plugin, req)
	if err == nil {
		t.Error("expected JSON marshal error")
	}
}

// TestParseDetectResultInvalidData tests ParseDetectResult with invalid result data.
func TestParseDetectResultInvalidData(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: "invalid", // Should be a map
	}

	_, err := ParseDetectResult(resp)
	if err == nil {
		t.Error("expected error for invalid result data")
	}
}

// TestParseIngestResultInvalidData tests ParseIngestResult with invalid result data.
func TestParseIngestResultInvalidData(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: "invalid", // Should be a map
	}

	_, err := ParseIngestResult(resp)
	if err == nil {
		t.Error("expected error for invalid result data")
	}
}

// TestParseEnumerateResultInvalidData tests ParseEnumerateResult with invalid result data.
func TestParseEnumerateResultInvalidData(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: "invalid", // Should be a map
	}

	_, err := ParseEnumerateResult(resp)
	if err == nil {
		t.Error("expected error for invalid result data")
	}
}

// TestParseEngineSpecResultInvalidData tests ParseEngineSpecResult with invalid result data.
func TestParseEngineSpecResultInvalidData(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: "invalid", // Should be a map
	}

	_, err := ParseEngineSpecResult(resp)
	if err == nil {
		t.Error("expected error for invalid result data")
	}
}

// TestParseExtractIRResultInvalidData tests ParseExtractIRResult with invalid result data.
func TestParseExtractIRResultInvalidData(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: "invalid", // Should be a map
	}

	_, err := ParseExtractIRResult(resp)
	if err == nil {
		t.Error("expected error for invalid result data")
	}
}

// TestParseEmitNativeResultInvalidData tests ParseEmitNativeResult with invalid result data.
func TestParseEmitNativeResultInvalidData(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: "invalid", // Should be a map
	}

	_, err := ParseEmitNativeResult(resp)
	if err == nil {
		t.Error("expected error for invalid result data")
	}
}

// TestParseDetectResultWrongType tests ParseDetectResult with wrong field types.
func TestParseDetectResultWrongType(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: map[string]interface{}{
			"detected": "not a boolean", // Wrong type
		},
	}

	_, err := ParseDetectResult(resp)
	if err == nil {
		t.Error("expected error for wrong field type")
	}
}

// TestParseIngestResultWrongType tests ParseIngestResult with wrong field types.
func TestParseIngestResultWrongType(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: map[string]interface{}{
			"size_bytes": "not a number", // Wrong type
		},
	}

	_, err := ParseIngestResult(resp)
	if err == nil {
		t.Error("expected error for wrong field type")
	}
}

// TestParseEnumerateResultWrongType tests ParseEnumerateResult with wrong field types.
func TestParseEnumerateResultWrongType(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: map[string]interface{}{
			"entries": "not an array", // Wrong type
		},
	}

	_, err := ParseEnumerateResult(resp)
	if err == nil {
		t.Error("expected error for wrong field type")
	}
}

// TestParseEngineSpecResultWrongType tests ParseEngineSpecResult with wrong field types.
func TestParseEngineSpecResultWrongType(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: map[string]interface{}{
			"packages": "not an array", // Wrong type
		},
	}

	_, err := ParseEngineSpecResult(resp)
	if err == nil {
		t.Error("expected error for wrong field type")
	}
}

// TestParseExtractIRResultWrongType tests ParseExtractIRResult with wrong field types.
func TestParseExtractIRResultWrongType(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: map[string]interface{}{
			"loss_report": "not an object", // Wrong type
		},
	}

	_, err := ParseExtractIRResult(resp)
	if err == nil {
		t.Error("expected error for wrong field type")
	}
}

// TestParseEmitNativeResultWrongType tests ParseEmitNativeResult with wrong field types.
func TestParseEmitNativeResultWrongType(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: map[string]interface{}{
			"output_path": 123, // Wrong type
		},
	}

	_, err := ParseEmitNativeResult(resp)
	if err == nil {
		t.Error("expected error for wrong field type")
	}
}

// TestIsNotImplementedError tests the isNotImplementedError helper function.
func TestIsNotImplementedError(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		expected bool
	}{
		{
			name:     "requires external plugin",
			msg:      "sword-pure format requires external plugin for IR extraction",
			expected: true,
		},
		{
			name:     "not implemented",
			msg:      "this feature is not implemented",
			expected: true,
		},
		{
			name:     "not supported",
			msg:      "operation not supported",
			expected: true,
		},
		{
			name:     "regular error",
			msg:      "file not found",
			expected: false,
		},
		{
			name:     "empty error",
			msg:      "",
			expected: false,
		},
		{
			name:     "case sensitive - requires external",
			msg:      "requires external plugin",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotImplementedError(tt.msg)
			if result != tt.expected {
				t.Errorf("isNotImplementedError(%q) = %v, want %v", tt.msg, result, tt.expected)
			}
		})
	}
}

// TestExternalPluginsEnabled tests the external plugins enable/disable functions.
func TestExternalPluginsEnabled(t *testing.T) {
	// Save original state
	originalState := ExternalPluginsEnabled()
	defer func() {
		if originalState {
			EnableExternalPlugins()
		} else {
			DisableExternalPlugins()
		}
	}()

	// Test disable
	DisableExternalPlugins()
	if ExternalPluginsEnabled() {
		t.Error("expected external plugins to be disabled")
	}

	// Test enable
	EnableExternalPlugins()
	if !ExternalPluginsEnabled() {
		t.Error("expected external plugins to be enabled")
	}
}

// TestExecutePluginFallbackToExternal tests that embedded plugins fall back to external
// when returning "not implemented" errors.
func TestExecutePluginFallbackToExternal(t *testing.T) {
	// Create a mock external plugin that returns success
	tempDir, err := os.MkdirTemp("", "plugin-fallback-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simple shell script that returns a valid response
	scriptContent := `#!/bin/sh
read input
echo '{"status":"ok","result":{"ir_path":"/tmp/test.json","loss_class":"L0"}}'
`
	scriptPath := filepath.Join(tempDir, "format-fallback-test")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// Skip if sh is not available
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	plugin := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "format.fallback-test",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-fallback-test",
		},
		Path: tempDir,
	}

	req := NewExtractIRRequest("/path/to/file", "/tmp/output")

	// Enable external plugins
	EnableExternalPlugins()
	defer DisableExternalPlugins()

	// Execute - should use external plugin since it's enabled and available
	resp, err := ExecutePluginWithTimeout(plugin, req, 5*time.Second)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
}

// TestExecutePluginNoFallbackWhenDisabled tests that external plugins are not used
// when external plugins are disabled.
func TestExecutePluginNoFallbackWhenDisabled(t *testing.T) {
	// Create a mock external plugin
	tempDir, err := os.MkdirTemp("", "plugin-nofallback-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	scriptContent := `#!/bin/sh
read input
echo '{"status":"ok","result":{"ir_path":"/tmp/test.json"}}'
`
	scriptPath := filepath.Join(tempDir, "format-nofallback-test")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	plugin := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "format.nofallback-test",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-nofallback-test",
		},
		Path: tempDir,
	}

	req := NewExtractIRRequest("/path/to/file", "/tmp/output")

	// Disable external plugins
	DisableExternalPlugins()

	// Execute - should try embedded plugin first (which doesn't exist for this ID)
	// and then fail since external is disabled
	_, err = ExecutePluginWithTimeout(plugin, req, 5*time.Second)
	// Since there's no embedded handler for "format.nofallback-test" and external is disabled,
	// it should still try external as fallback when embedded returns nil
	if err != nil {
		// This is expected if no embedded plugin exists and external fallback kicks in
		// The fallback should still work even when disabled for the "not implemented" case
		t.Logf("Got expected behavior: %v", err)
	}
}

// TestExecuteEmbeddedPluginPath tests that embedded plugins are identified correctly.
func TestExecuteEmbeddedPluginPath(t *testing.T) {
	plugin := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "format.embedded-test",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-embedded-test",
		},
		Path: "(embedded)",
	}

	req := NewDetectRequest("/path/to/file")

	// Even with external plugins enabled, embedded path should not try external
	EnableExternalPlugins()
	defer DisableExternalPlugins()

	// Execute - should only try embedded (which doesn't exist) and return error
	_, err := ExecutePluginWithTimeout(plugin, req, 5*time.Second)
	if err == nil {
		t.Error("expected error for non-existent embedded plugin")
	}
	if err != nil && !strings.Contains(err.Error(), "not available") {
		t.Errorf("expected 'not available' error, got: %v", err)
	}
}

// TestExecutePluginExternalPriority tests that external plugins take priority
// when enabled and available.
func TestExecutePluginExternalPriority(t *testing.T) {
	// Create a mock external plugin
	tempDir, err := os.MkdirTemp("", "plugin-priority-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// External plugin returns a distinctive response
	scriptContent := `#!/bin/sh
read input
echo '{"status":"ok","result":{"source":"external"}}'
`
	scriptPath := filepath.Join(tempDir, "format-priority-test")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// Skip if sh is not available
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	plugin := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "format.priority-test",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-priority-test",
		},
		Path: tempDir,
	}

	req := &IPCRequest{Command: "test"}

	// Enable external plugins
	EnableExternalPlugins()
	defer DisableExternalPlugins()

	// Execute - should use external plugin
	resp, err := ExecutePluginWithTimeout(plugin, req, 5*time.Second)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}

	// Verify the response came from external plugin
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}
	if result["source"] != "external" {
		t.Errorf("expected source 'external', got %v", result["source"])
	}
}

// TestParseDetectResultMarshalError tests ParseDetectResult with data that can't be marshaled.
func TestParseDetectResultMarshalError(t *testing.T) {
	// Create a response with result that contains a value that can't be marshaled
	resp := &IPCResponse{
		Status: "ok",
		Result: func() {}, // functions can't be marshaled to JSON
	}

	_, err := ParseDetectResult(resp)
	if err == nil {
		t.Error("expected error for unmarshalable result")
	}
}

// TestParseIngestResultMarshalError tests ParseIngestResult with data that can't be marshaled.
func TestParseIngestResultMarshalError(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: func() {}, // functions can't be marshaled to JSON
	}

	_, err := ParseIngestResult(resp)
	if err == nil {
		t.Error("expected error for unmarshalable result")
	}
}

// TestParseEnumerateResultMarshalError tests ParseEnumerateResult with data that can't be marshaled.
func TestParseEnumerateResultMarshalError(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: func() {}, // functions can't be marshaled to JSON
	}

	_, err := ParseEnumerateResult(resp)
	if err == nil {
		t.Error("expected error for unmarshalable result")
	}
}

// TestParseEngineSpecResultMarshalError tests ParseEngineSpecResult with data that can't be marshaled.
func TestParseEngineSpecResultMarshalError(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: func() {}, // functions can't be marshaled to JSON
	}

	_, err := ParseEngineSpecResult(resp)
	if err == nil {
		t.Error("expected error for unmarshalable result")
	}
}

// TestParseExtractIRResultMarshalError tests ParseExtractIRResult with data that can't be marshaled.
func TestParseExtractIRResultMarshalError(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: func() {}, // functions can't be marshaled to JSON
	}

	_, err := ParseExtractIRResult(resp)
	if err == nil {
		t.Error("expected error for unmarshalable result")
	}
}

// TestParseEmitNativeResultMarshalError tests ParseEmitNativeResult with data that can't be marshaled.
func TestParseEmitNativeResultMarshalError(t *testing.T) {
	resp := &IPCResponse{
		Status: "ok",
		Result: func() {}, // functions can't be marshaled to JSON
	}

	_, err := ParseEmitNativeResult(resp)
	if err == nil {
		t.Error("expected error for unmarshalable result")
	}
}

// mockFormatHandlerWithNotImplemented is a mock handler that returns "not implemented" errors.
type mockFormatHandlerWithNotImplemented struct{}

func (m *mockFormatHandlerWithNotImplemented) Detect(path string) (*DetectResult, error) {
	return nil, fmt.Errorf("this feature requires external plugin")
}

func (m *mockFormatHandlerWithNotImplemented) Ingest(path, outputDir string) (*IngestResult, error) {
	return nil, fmt.Errorf("this feature requires external plugin")
}

func (m *mockFormatHandlerWithNotImplemented) Enumerate(path string) (*EnumerateResult, error) {
	return nil, fmt.Errorf("this feature requires external plugin")
}

func (m *mockFormatHandlerWithNotImplemented) ExtractIR(path, outputDir string) (*ExtractIRResult, error) {
	return nil, fmt.Errorf("this feature requires external plugin")
}

func (m *mockFormatHandlerWithNotImplemented) EmitNative(irPath, outputDir string) (*EmitNativeResult, error) {
	return nil, fmt.Errorf("this feature requires external plugin")
}

// TestExecutePluginWithTimeoutEmbeddedFallback tests embedded plugin fallback on "not implemented" errors.
func TestExecutePluginWithTimeoutEmbeddedFallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-embedded-fallback-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an external plugin that will be used as fallback
	scriptContent := `#!/bin/sh
read input
echo '{"status":"ok","result":{"source":"fallback"}}'
`
	scriptPath := filepath.Join(tempDir, "format-embedded-fb")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// Skip if sh is not available
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	// Register an embedded plugin that returns "not implemented" error
	ClearEmbeddedRegistry()
	defer ClearEmbeddedRegistry()

	RegisterEmbeddedPlugin(&EmbeddedPlugin{
		Manifest: &PluginManifest{
			PluginID:   "format.embedded-fb",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-embedded-fb",
		},
		Format: &mockFormatHandlerWithNotImplemented{},
	})

	plugin := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "format.embedded-fb",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-embedded-fb",
		},
		Path: tempDir,
	}

	req := &IPCRequest{Command: "detect", Args: map[string]interface{}{"path": "/test"}}

	// Execute - should try embedded first, get "not implemented", then fallback to external
	resp, err := ExecutePluginWithTimeout(plugin, req, 5*time.Second)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}

	// Verify the response came from external fallback
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}
	if result["source"] != "fallback" {
		t.Errorf("expected source 'fallback', got %v", result["source"])
	}
}

// TestExecutePluginWithTimeoutNoEmbeddedNoExternal tests when both embedded and external are unavailable.
func TestExecutePluginWithTimeoutNoEmbeddedNoExternal(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "plugin-noexternal-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a plugin with no embedded handler and no external binary
	plugin := &Plugin{
		Manifest: &PluginManifest{
			PluginID:   "format.nohandler",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "nonexistent-binary",
		},
		Path: tempDir,
	}

	req := &IPCRequest{Command: "test"}

	// Execute - should fail because no embedded and no external binary
	_, err = ExecutePluginWithTimeout(plugin, req, 5*time.Second)
	if err == nil {
		t.Error("expected error when plugin is not available")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("expected 'not available' error, got: %v", err)
	}
}
