package runner

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestNewRequest tests creating a new tool run request.
func TestNewRequest(t *testing.T) {
	req := NewRequest("tools.libsword", "osis->html")

	if req.PluginID != "tools.libsword" {
		t.Errorf("expected plugin_id 'tools.libsword', got %q", req.PluginID)
	}

	if req.Profile != "osis->html" {
		t.Errorf("expected profile 'osis->html', got %q", req.Profile)
	}

	if req.Env.TZ != "UTC" {
		t.Errorf("expected TZ 'UTC', got %q", req.Env.TZ)
	}

	if req.Env.LCALL != "C.UTF-8" {
		t.Errorf("expected LC_ALL 'C.UTF-8', got %q", req.Env.LCALL)
	}
}

// TestRequestToJSON tests serializing a request to JSON.
func TestRequestToJSON(t *testing.T) {
	req := NewRequest("tools.libsword", "raw")
	req.Inputs = []string{"artifact-1", "artifact-2"}

	data, err := req.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize request: %v", err)
	}

	// Parse back and verify
	var parsed Request
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse request JSON: %v", err)
	}

	if parsed.PluginID != req.PluginID {
		t.Errorf("plugin_id mismatch after round-trip")
	}

	if len(parsed.Inputs) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(parsed.Inputs))
	}
}

// TestPrepareWorkDir tests preparing the work directory for VM execution.
func TestPrepareWorkDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	req := NewRequest("tools.libsword", "raw")
	req.Inputs = []string{"test-input"}

	workDir := filepath.Join(tempDir, "work")
	if err := PrepareWorkDir(workDir, req); err != nil {
		t.Fatalf("failed to prepare work dir: %v", err)
	}

	// Verify request.json exists
	reqPath := filepath.Join(workDir, "in", "request.json")
	if _, err := os.Stat(reqPath); os.IsNotExist(err) {
		t.Error("request.json should exist")
	}

	// Verify runner.sh exists
	runnerPath := filepath.Join(workDir, "in", "runner.sh")
	if _, err := os.Stat(runnerPath); os.IsNotExist(err) {
		t.Error("runner.sh should exist")
	}

	// Verify out directory exists
	outDir := filepath.Join(workDir, "out")
	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		t.Error("out directory should exist")
	}
}

// TestRunnerScript tests that the runner script has correct content.
func TestRunnerScript(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	req := NewRequest("tools.test", "test-profile")
	workDir := filepath.Join(tempDir, "work")

	if err := PrepareWorkDir(workDir, req); err != nil {
		t.Fatalf("failed to prepare work dir: %v", err)
	}

	// Read runner script
	runnerPath := filepath.Join(workDir, "in", "runner.sh")
	content, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatalf("failed to read runner.sh: %v", err)
	}

	script := string(content)

	// Verify it sets TZ
	if !contains(script, "TZ=UTC") {
		t.Error("runner.sh should set TZ=UTC")
	}

	// Verify it sets LC_ALL
	if !contains(script, "LC_ALL=C.UTF-8") {
		t.Error("runner.sh should set LC_ALL=C.UTF-8")
	}

	// Verify it sets LANG
	if !contains(script, "LANG=C.UTF-8") {
		t.Error("runner.sh should set LANG=C.UTF-8")
	}
}

// TestTranscriptParsing tests parsing a transcript JSONL file.
func TestTranscriptParsing(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "transcript-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a sample transcript
	transcriptContent := `{"t":"ENGINE_INFO","seq":0,"engine_id":"test-engine","plugin_id":"tools.test","plugin_version":"1.0.0"}
{"t":"MODULE_DISCOVERED","seq":1,"module":"TestModule"}
{"t":"KEY_ENUM","seq":2,"module":"TestModule","sha256":"abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234","bytes":100}
{"t":"ENTRY_RENDERED","seq":3,"module":"TestModule","key":"Gen.1.1","profile":"raw","sha256":"1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd","bytes":50}
`
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(transcriptContent), 0600); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	// Parse the transcript
	events, err := ParseTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("failed to parse transcript: %v", err)
	}

	if len(events) != 4 {
		t.Errorf("expected 4 events, got %d", len(events))
	}

	// Verify first event
	if events[0].Type != "ENGINE_INFO" {
		t.Errorf("first event should be ENGINE_INFO, got %s", events[0].Type)
	}

	if events[0].Seq != 0 {
		t.Errorf("first event seq should be 0, got %d", events[0].Seq)
	}

	// Verify module discovery
	if events[1].Module != "TestModule" {
		t.Errorf("expected module 'TestModule', got %q", events[1].Module)
	}
}

// TestTranscriptValidation tests that invalid transcripts are rejected.
func TestTranscriptValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "transcript-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an invalid transcript (missing required fields)
	invalidContent := `{"t":"ENGINE_INFO"}
{"invalid": "json without required fields"}
`
	transcriptPath := filepath.Join(tempDir, "invalid.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(invalidContent), 0600); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	// This should still parse but with validation warnings
	events, err := ParseTranscript(transcriptPath)
	if err != nil {
		// Parsing errors are acceptable for invalid content
		return
	}

	// If it parses, check that we got some events
	if len(events) == 0 {
		t.Error("expected at least some events to be parsed")
	}
}

// TestEngineSpec tests creating an engine specification.
func TestEngineSpec(t *testing.T) {
	spec := NewEngineSpec("capsule-engine-v1")

	if spec.EngineID != "capsule-engine-v1" {
		t.Errorf("expected engine_id 'capsule-engine-v1', got %q", spec.EngineID)
	}

	if spec.Type != "nixos-vm" {
		t.Errorf("expected type 'nixos-vm', got %q", spec.Type)
	}

	if spec.Env.TZ != "UTC" {
		t.Errorf("expected TZ 'UTC', got %q", spec.Env.TZ)
	}
}

// helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestWriteTranscript tests writing a transcript to a file.
func TestWriteTranscript(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	events := []TranscriptEvent{
		{Type: EventEngineInfo, Seq: 1, EngineID: "test-engine"},
		{Type: EventModuleDiscovered, Seq: 2, Module: "KJV"},
		{Type: EventWarn, Seq: 3, Message: "Test warning"},
		{Type: EventError, Seq: 4, Message: "Test error"},
	}

	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	if err := WriteTranscript(transcriptPath, events); err != nil {
		t.Fatalf("WriteTranscript failed: %v", err)
	}

	// Read back and verify
	readEvents, err := ParseTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("ParseTranscript failed: %v", err)
	}

	if len(readEvents) != len(events) {
		t.Errorf("read %d events, want %d", len(readEvents), len(events))
	}
}

// TestLoadTranscript tests loading a transcript file.
func TestLoadTranscript(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	events := []TranscriptEvent{
		{Type: EventEngineInfo, Seq: 1, EngineID: "test-engine"},
		{Type: EventModuleDiscovered, Seq: 2, Module: "KJV"},
	}

	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	if err := WriteTranscript(transcriptPath, events); err != nil {
		t.Fatalf("WriteTranscript failed: %v", err)
	}

	transcript, err := LoadTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("LoadTranscript failed: %v", err)
	}

	if transcript.Path != transcriptPath {
		t.Errorf("Path = %q, want %q", transcript.Path, transcriptPath)
	}

	if len(transcript.Events) != len(events) {
		t.Errorf("len(Events) = %d, want %d", len(transcript.Events), len(events))
	}
}

// TestLoadTranscriptNotFound tests loading a non-existent transcript.
func TestLoadTranscriptNotFound(t *testing.T) {
	_, err := LoadTranscript("/nonexistent/path/transcript.jsonl")
	if err == nil {
		t.Error("LoadTranscript should return error for non-existent file")
	}
}

// TestTranscriptGetEngineInfo tests getting engine info from transcript.
func TestTranscriptGetEngineInfo(t *testing.T) {
	transcript := &Transcript{
		Events: []TranscriptEvent{
			{Type: EventModuleDiscovered, Seq: 1, Module: "KJV"},
			{Type: EventEngineInfo, Seq: 2, EngineID: "test-engine"},
			{Type: EventWarn, Seq: 3, Message: "Warning"},
		},
	}

	info := transcript.GetEngineInfo()
	if info == nil {
		t.Fatal("GetEngineInfo returned nil")
	}
	if info.EngineID != "test-engine" {
		t.Errorf("EngineID = %q, want %q", info.EngineID, "test-engine")
	}
}

// TestTranscriptGetEngineInfoNotFound tests GetEngineInfo when not present.
func TestTranscriptGetEngineInfoNotFound(t *testing.T) {
	transcript := &Transcript{
		Events: []TranscriptEvent{
			{Type: EventModuleDiscovered, Seq: 1, Module: "KJV"},
		},
	}

	info := transcript.GetEngineInfo()
	if info != nil {
		t.Error("GetEngineInfo should return nil when not present")
	}
}

// TestTranscriptGetModules tests getting discovered modules.
func TestTranscriptGetModules(t *testing.T) {
	transcript := &Transcript{
		Events: []TranscriptEvent{
			{Type: EventModuleDiscovered, Seq: 1, Module: "KJV"},
			{Type: EventModuleDiscovered, Seq: 2, Module: "ESV"},
			{Type: EventModuleDiscovered, Seq: 3, Module: "KJV"}, // Duplicate
			{Type: EventWarn, Seq: 4, Message: "Warning"},
		},
	}

	modules := transcript.GetModules()
	if len(modules) != 2 {
		t.Errorf("len(modules) = %d, want 2", len(modules))
	}

	// Check that KJV and ESV are present
	found := make(map[string]bool)
	for _, m := range modules {
		found[m] = true
	}
	if !found["KJV"] || !found["ESV"] {
		t.Errorf("expected KJV and ESV, got %v", modules)
	}
}

// TestTranscriptGetErrors tests getting error events.
func TestTranscriptGetErrors(t *testing.T) {
	transcript := &Transcript{
		Events: []TranscriptEvent{
			{Type: EventEngineInfo, Seq: 1},
			{Type: EventError, Seq: 2, Message: "Error 1"},
			{Type: EventWarn, Seq: 3, Message: "Warning"},
			{Type: EventError, Seq: 4, Message: "Error 2"},
		},
	}

	errors := transcript.GetErrors()
	if len(errors) != 2 {
		t.Errorf("len(errors) = %d, want 2", len(errors))
	}

	if errors[0].Message != "Error 1" || errors[1].Message != "Error 2" {
		t.Error("errors have wrong messages")
	}
}

// TestTranscriptGetWarnings tests getting warning events.
func TestTranscriptGetWarnings(t *testing.T) {
	transcript := &Transcript{
		Events: []TranscriptEvent{
			{Type: EventEngineInfo, Seq: 1},
			{Type: EventWarn, Seq: 2, Message: "Warning 1"},
			{Type: EventError, Seq: 3, Message: "Error"},
			{Type: EventWarn, Seq: 4, Message: "Warning 2"},
		},
	}

	warnings := transcript.GetWarnings()
	if len(warnings) != 2 {
		t.Errorf("len(warnings) = %d, want 2", len(warnings))
	}
}

// TestTranscriptHasErrors tests checking for errors.
func TestTranscriptHasErrors(t *testing.T) {
	noErrors := &Transcript{
		Events: []TranscriptEvent{
			{Type: EventEngineInfo, Seq: 1},
			{Type: EventWarn, Seq: 2, Message: "Warning"},
		},
	}

	if noErrors.HasErrors() {
		t.Error("HasErrors should return false when no errors")
	}

	withErrors := &Transcript{
		Events: []TranscriptEvent{
			{Type: EventEngineInfo, Seq: 1},
			{Type: EventError, Seq: 2, Message: "Error"},
		},
	}

	if !withErrors.HasErrors() {
		t.Error("HasErrors should return true when errors present")
	}
}

// TestTranscriptEventCount tests counting events.
func TestTranscriptEventCount(t *testing.T) {
	transcript := &Transcript{
		Events: []TranscriptEvent{
			{Type: EventEngineInfo, Seq: 1},
			{Type: EventModuleDiscovered, Seq: 2},
			{Type: EventWarn, Seq: 3},
		},
	}

	if transcript.EventCount() != 3 {
		t.Errorf("EventCount() = %d, want 3", transcript.EventCount())
	}

	empty := &Transcript{Events: []TranscriptEvent{}}
	if empty.EventCount() != 0 {
		t.Errorf("EventCount() = %d, want 0", empty.EventCount())
	}
}

// TestCollectResults tests collecting results from a completed run.
func TestCollectResults(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outDir := filepath.Join(tempDir, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}

	// Create test files
	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	if err := os.WriteFile(transcriptPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	stdoutPath := filepath.Join(outDir, "stdout")
	if err := os.WriteFile(stdoutPath, []byte("stdout content"), 0600); err != nil {
		t.Fatalf("failed to write stdout: %v", err)
	}

	stderrPath := filepath.Join(outDir, "stderr")
	if err := os.WriteFile(stderrPath, []byte("stderr content"), 0600); err != nil {
		t.Fatalf("failed to write stderr: %v", err)
	}

	result, err := CollectResults(tempDir)
	if err != nil {
		t.Fatalf("CollectResults failed: %v", err)
	}

	if result.OutputDir != outDir {
		t.Errorf("OutputDir = %q, want %q", result.OutputDir, outDir)
	}

	if result.TranscriptPath != transcriptPath {
		t.Errorf("TranscriptPath = %q, want %q", result.TranscriptPath, transcriptPath)
	}

	if result.StdoutPath != stdoutPath {
		t.Errorf("StdoutPath = %q, want %q", result.StdoutPath, stdoutPath)
	}

	if result.StderrPath != stderrPath {
		t.Errorf("StderrPath = %q, want %q", result.StderrPath, stderrPath)
	}
}

// TestCollectResultsEmpty tests collecting results with no files.
func TestCollectResultsEmpty(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outDir := filepath.Join(tempDir, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}

	result, err := CollectResults(tempDir)
	if err != nil {
		t.Fatalf("CollectResults failed: %v", err)
	}

	if result.TranscriptPath != "" {
		t.Errorf("TranscriptPath should be empty, got %q", result.TranscriptPath)
	}

	if result.StdoutPath != "" {
		t.Errorf("StdoutPath should be empty, got %q", result.StdoutPath)
	}

	if result.StderrPath != "" {
		t.Errorf("StderrPath should be empty, got %q", result.StderrPath)
	}
}

// TestPrepareWorkDirErrors tests error handling in PrepareWorkDir.
func TestPrepareWorkDirErrors(t *testing.T) {
	// Test with invalid directory (read-only parent)
	if os.Getuid() != 0 { // Skip if running as root
		req := NewRequest("test", "test")
		err := PrepareWorkDir("/proc/invalid/work", req)
		if err == nil {
			t.Error("expected error when creating directories in invalid location")
		}
	}
}

// TestRequestWithArgs tests request with custom arguments.
func TestRequestWithArgs(t *testing.T) {
	req := NewRequest("test", "profile")
	req.Args = map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}

	data, err := req.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	var parsed Request
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(parsed.Args) != 3 {
		t.Errorf("expected 3 args, got %d", len(parsed.Args))
	}
}

// TestRequestWithVersion tests request with plugin version.
func TestRequestWithVersion(t *testing.T) {
	req := NewRequest("test", "profile")
	req.PluginVersion = "1.0.0"

	data, err := req.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	var parsed Request
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed.PluginVersion != "1.0.0" {
		t.Errorf("PluginVersion = %q, want %q", parsed.PluginVersion, "1.0.0")
	}
}

// TestParseTranscriptEmptyLines tests parsing with empty lines.
func TestParseTranscriptEmptyLines(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "transcript-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create transcript with empty lines
	content := `{"t":"ENGINE_INFO","seq":0}

{"t":"WARN","seq":1,"message":"test"}

`
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	events, err := ParseTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("ParseTranscript failed: %v", err)
	}

	// Empty lines should be skipped
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

// TestParseTranscriptInvalidJSON tests parsing with invalid JSON.
func TestParseTranscriptInvalidJSON(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "transcript-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	content := `{"t":"ENGINE_INFO","seq":0}
invalid json line
{"t":"WARN","seq":1}
`
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	_, err = ParseTranscript(transcriptPath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestWriteTranscriptErrors tests error handling in WriteTranscript.
func TestWriteTranscriptErrors(t *testing.T) {
	// Test with invalid path (read-only parent)
	if os.Getuid() != 0 {
		events := []TranscriptEvent{
			{Type: EventEngineInfo, Seq: 1},
		}
		err := WriteTranscript("/proc/invalid/transcript.jsonl", events)
		if err == nil {
			t.Error("expected error for invalid path")
		}
	}
}

// TestTranscriptGetModulesEmpty tests GetModules with no modules.
func TestTranscriptGetModulesEmpty(t *testing.T) {
	transcript := &Transcript{
		Events: []TranscriptEvent{
			{Type: EventEngineInfo, Seq: 1},
			{Type: EventWarn, Seq: 2},
		},
	}

	modules := transcript.GetModules()
	if len(modules) != 0 {
		t.Errorf("expected 0 modules, got %d", len(modules))
	}
}

// TestTranscriptGetErrorsEmpty tests GetErrors with no errors.
func TestTranscriptGetErrorsEmpty(t *testing.T) {
	transcript := &Transcript{
		Events: []TranscriptEvent{
			{Type: EventEngineInfo, Seq: 1},
			{Type: EventWarn, Seq: 2},
		},
	}

	errors := transcript.GetErrors()
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errors))
	}
}

// TestTranscriptGetWarningsEmpty tests GetWarnings with no warnings.
func TestTranscriptGetWarningsEmpty(t *testing.T) {
	transcript := &Transcript{
		Events: []TranscriptEvent{
			{Type: EventEngineInfo, Seq: 1},
			{Type: EventError, Seq: 2},
		},
	}

	warnings := transcript.GetWarnings()
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}
}

// TestWriteTranscriptInjectedMarshalError tests WriteTranscript with injected marshal error.
func TestWriteTranscriptInjectedMarshalError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Inject error
	origMarshal := jsonMarshal
	jsonMarshal = func(v interface{}) ([]byte, error) {
		return nil, fmt.Errorf("injected marshal error")
	}
	defer func() { jsonMarshal = origMarshal }()

	events := []TranscriptEvent{{Type: EventEngineInfo, Seq: 1}}
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")

	err = WriteTranscript(transcriptPath, events)
	if err == nil {
		t.Error("expected error for marshal failure")
	}
	if !contains(err.Error(), "failed to marshal event") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestWriteTranscriptInjectedWriteError tests WriteTranscript with injected write error.
func TestWriteTranscriptInjectedWriteError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Inject error
	origWrite := fileWrite
	fileWrite = func(w io.Writer, data []byte) (int, error) {
		return 0, fmt.Errorf("injected write error")
	}
	defer func() { fileWrite = origWrite }()

	events := []TranscriptEvent{{Type: EventEngineInfo, Seq: 1}}
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")

	err = WriteTranscript(transcriptPath, events)
	if err == nil {
		t.Error("expected error for write failure")
	}
	if !contains(err.Error(), "failed to write event") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestWriteTranscriptInjectedNewlineError tests WriteTranscript with injected newline write error.
func TestWriteTranscriptInjectedNewlineError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "runner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Inject error only on WriteString
	origWriteString := writeString
	writeString = func(w io.StringWriter, s string) (int, error) {
		return 0, fmt.Errorf("injected newline error")
	}
	defer func() { writeString = origWriteString }()

	events := []TranscriptEvent{{Type: EventEngineInfo, Seq: 1}}
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")

	err = WriteTranscript(transcriptPath, events)
	if err == nil {
		t.Error("expected error for newline write failure")
	}
	if !contains(err.Error(), "failed to write newline") {
		t.Errorf("unexpected error: %v", err)
	}
}
