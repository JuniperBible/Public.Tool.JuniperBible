//go:build !sdk

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// Helper function to execute plugin command via stdin/stdout simulation
func executeCommand(t *testing.T, req ipc.Request) map[string]interface{} {
	t.Helper()

	// Marshal request
	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	// Save current stdin/stdout
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	// Create pipes for stdin/stdout
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdin pipe: %v", err)
	}

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}

	// Replace stdin/stdout
	os.Stdin = stdinReader
	os.Stdout = stdoutWriter

	// Write request to stdin
	go func() {
		stdinWriter.Write(reqData)
		stdinWriter.Close()
	}()

	// Run main in goroutine
	done := make(chan bool)
	go func() {
		main()
		stdoutWriter.Close()
		done <- true
	}()

	// Read response from stdout
	var buf bytes.Buffer
	buf.ReadFrom(stdoutReader)
	<-done

	// Parse response
	var resp ipc.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v\nRaw output: %s", err, buf.String())
	}

	if resp.Status == "error" {
		// For commands that are expected to error (extract-ir, emit-native)
		return map[string]interface{}{
			"_error": resp.Error,
		}
	}

	// Type assert the result to map
	resultMap, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be map[string]interface{}, got %T", resp.Result)
	}

	return resultMap
}

func TestDetect_NotOliveTree(t *testing.T) {
	// Create temporary non-Olive Tree file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("not an olive tree file"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args: map[string]interface{}{
			"path": testFile,
		},
	}

	result := executeCommand(t, req)
	detected, ok := result["detected"].(bool)
	if !ok {
		t.Fatalf("detected field missing or wrong type")
	}

	if detected {
		t.Errorf("expected detected=false for .txt file")
	}
}

func TestDetect_OT4IFile(t *testing.T) {
	// Create temporary .ot4i file with SQLite header
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ot4i")

	// Create a file with SQLite signature
	content := []byte("SQLite format 3\x00\x00\x00")
	content = append(content, make([]byte, 100)...) // Pad with zeros
	if err := os.WriteFile(testFile, content, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args: map[string]interface{}{
			"path": testFile,
		},
	}

	result := executeCommand(t, req)
	detected, ok := result["detected"].(bool)
	if !ok {
		t.Fatalf("detected field missing or wrong type")
	}

	if !detected {
		reason := result["reason"].(string)
		t.Errorf("expected detected=true for .ot4i file, got reason: %s", reason)
	}

	format, ok := result["format"].(string)
	if !ok || format != "OliveTree" {
		t.Errorf("expected format=OliveTree, got %v", format)
	}
}

func TestDetect_OTIFile(t *testing.T) {
	// Create temporary .oti file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.oti")

	// Create a proprietary format file (non-SQLite)
	content := make([]byte, 100)
	content[0] = 0x4F // 'O'
	content[1] = 0x54 // 'T'
	if err := os.WriteFile(testFile, content, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args: map[string]interface{}{
			"path": testFile,
		},
	}

	result := executeCommand(t, req)
	detected, ok := result["detected"].(bool)
	if !ok {
		t.Fatalf("detected field missing or wrong type")
	}

	if !detected {
		reason := result["reason"].(string)
		t.Errorf("expected detected=true for .oti file, got reason: %s", reason)
	}
}

func TestDetect_PDBFile(t *testing.T) {
	// Create temporary .pdb file with Palm Database header
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.pdb")

	// Create a minimal PDB header (78 bytes)
	header := make([]byte, 78)
	// Database name: "KJV Bible"
	copy(header[0:32], []byte("KJV Bible"))
	// Attributes, version, times (zeros are fine for test)
	// Type: "DATA"
	copy(header[60:64], []byte("DATA"))
	// Creator: "OlTr" (Olive Tree)
	copy(header[64:68], []byte("OlTr"))

	if err := os.WriteFile(testFile, header, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args: map[string]interface{}{
			"path": testFile,
		},
	}

	result := executeCommand(t, req)
	detected, ok := result["detected"].(bool)
	if !ok {
		t.Fatalf("detected field missing or wrong type")
	}

	if !detected {
		reason := result["reason"].(string)
		t.Errorf("expected detected=true for .pdb file with OlTr creator, got reason: %s", reason)
	}
}

func TestIngest(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ot4i")
	outputDir := filepath.Join(tmpDir, "blobs")

	// Create test file
	testContent := []byte("SQLite format 3\x00test content")
	if err := os.WriteFile(testFile, testContent, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       testFile,
			"output_dir": outputDir,
		},
	}

	result := executeCommand(t, req)

	artifactID, ok := result["artifact_id"].(string)
	if !ok || artifactID != "test" {
		t.Errorf("expected artifact_id='test', got %v", artifactID)
	}

	blobSHA256, ok := result["blob_sha256"].(string)
	if !ok || len(blobSHA256) != 64 {
		t.Errorf("expected valid SHA256 hash, got %v", blobSHA256)
	}

	// Verify blob was created
	blobPath := filepath.Join(outputDir, blobSHA256[:2], blobSHA256)
	if _, err := os.Stat(blobPath); err != nil {
		t.Errorf("blob file not created at %s: %v", blobPath, err)
	}
}

func TestEnumerate(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ot4i")

	// Create test file
	testContent := []byte("SQLite format 3\x00test")
	if err := os.WriteFile(testFile, testContent, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	req := ipc.Request{
		Command: "enumerate",
		Args: map[string]interface{}{
			"path": testFile,
		},
	}

	result := executeCommand(t, req)

	entries, ok := result["entries"].([]interface{})
	if !ok {
		t.Fatalf("entries field missing or wrong type")
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0].(map[string]interface{})
	path := entry["path"].(string)
	if path != "test.ot4i" {
		t.Errorf("expected path='test.ot4i', got %v", path)
	}
}

func TestExtractIR_NotSupported(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ot4i")
	outputDir := filepath.Join(tmpDir, "output")

	// Create test file
	testContent := []byte("SQLite format 3\x00test")
	if err := os.WriteFile(testFile, testContent, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       testFile,
			"output_dir": outputDir,
		},
	}

	result := executeCommand(t, req)

	// Should return error
	errMsg, ok := result["_error"].(string)
	if !ok {
		t.Fatalf("expected error response for extract-ir")
	}

	if !strings.Contains(errMsg, "not supported") && !strings.Contains(errMsg, "proprietary") {
		t.Errorf("expected error about proprietary format, got: %s", errMsg)
	}
}

func TestEmitNative_NotSupported(t *testing.T) {
	tmpDir := t.TempDir()
	irFile := filepath.Join(tmpDir, "test.ir.json")
	outputDir := filepath.Join(tmpDir, "output")

	// Create dummy IR file
	irContent := []byte(`{"id":"test","version":"1.0.0"}`)
	if err := os.WriteFile(irFile, irContent, 0600); err != nil {
		t.Fatalf("failed to create IR file: %v", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irFile,
			"output_dir": outputDir,
		},
	}

	result := executeCommand(t, req)

	// Should return error
	errMsg, ok := result["_error"].(string)
	if !ok {
		t.Fatalf("expected error response for emit-native")
	}

	if !strings.Contains(errMsg, "not supported") && !strings.Contains(errMsg, "proprietary") {
		t.Errorf("expected error about proprietary format, got: %s", errMsg)
	}
}

func TestDetect_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	req := ipc.Request{
		Command: "detect",
		Args: map[string]interface{}{
			"path": tmpDir,
		},
	}

	result := executeCommand(t, req)
	detected, ok := result["detected"].(bool)
	if !ok {
		t.Fatalf("detected field missing or wrong type")
	}

	if detected {
		t.Errorf("expected detected=false for directory")
	}
}
