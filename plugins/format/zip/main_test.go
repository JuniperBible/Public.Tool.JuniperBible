//go:build !sdk

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// Helper functions for creating test files

// createTestZip creates a test ZIP file with multiple entries.
func createTestZip(t *testing.T, path string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// Add a file
	w, err := zw.Create("test.txt")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	w.Write([]byte("Hello, World!"))

	// Add another file in a subdirectory
	w, err = zw.Create("subdir/file.txt")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	w.Write([]byte("Nested content"))
}

// createEmptyZip creates an empty ZIP file with no entries.
func createEmptyZip(t *testing.T, path string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	zw.Close()
}

// createZipWithDirectory creates a ZIP file with directory entries.
func createZipWithDirectory(t *testing.T, path string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// Add a directory entry
	_, err = zw.Create("docs/")
	if err != nil {
		t.Fatalf("failed to create directory entry: %v", err)
	}

	// Add a file in the directory
	w, err := zw.Create("docs/readme.txt")
	if err != nil {
		t.Fatalf("failed to create file entry: %v", err)
	}
	w.Write([]byte("ipc.Documentation"))
}

// Integration Tests - IPC command testing
// These tests exercise the full IPC interface by running the plugin as a subprocess

// Unit tests - direct function calls for coverage

// TestDetectZipDirect tests handleDetect directly.
func TestDetectZipDirect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath)

	// Capture stdout to check response
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	handleDetect(map[string]interface{}{"path": zipPath})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var resp ipc.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status ok, got %s", resp.Status)
	}
}

// TestDetectNonZipDirect tests handleDetect with non-ZIP file.
func TestDetectNonZipDirect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	txtPath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(txtPath, []byte("not a zip"), 0644)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	handleDetect(map[string]interface{}{"path": txtPath})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var resp ipc.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status ok, got %s", resp.Status)
	}
}

// TestIngestDirect tests handleIngest directly.
func TestIngestDirect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath)

	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	handleIngest(map[string]interface{}{
		"path":       zipPath,
		"output_dir": outputDir,
	})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var resp ipc.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}
}

// TestEnumerateDirect tests handleEnumerate directly.
func TestEnumerateDirect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	handleEnumerate(map[string]interface{}{"path": zipPath})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var resp ipc.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}
}

// Note: respond and respondError cannot be tested directly because they use
// ipc.MustRespond/RespondError which write to stdout. They are tested
// indirectly via the IPC tests (TestIPCDetectMissingPath, etc.)

// Integration Tests - IPC command testing via subprocess
// These tests exercise the full IPC interface by running the plugin as a subprocess

// TestIPCDetect tests the detect command via IPC.
func TestIPCDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": zipPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] != true {
		t.Error("expected detected to be true")
	}
	if result["format"] != "zip" {
		t.Errorf("expected format 'zip', got %v", result["format"])
	}
	if result["reason"] != "zip magic bytes detected" {
		t.Errorf("expected reason 'zip magic bytes detected', got %v", result["reason"])
	}
}

// TestIPCDetectNonZip tests detect on non-ZIP file via IPC.
func TestIPCDetectNonZip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	txtPath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(txtPath, []byte("not a zip file"), 0644)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": txtPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] == true {
		t.Error("expected detected to be false for non-ZIP file")
	}
	if result["reason"] != "not a zip file (magic bytes mismatch)" {
		t.Errorf("expected magic mismatch reason, got %v", result["reason"])
	}
}

// TestIPCIngest tests the ingest command via IPC.
func TestIPCIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath)

	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       zipPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["artifact_id"] != "test" {
		t.Errorf("expected artifact_id 'test', got %v", result["artifact_id"])
	}

	blobHash, ok := result["blob_sha256"].(string)
	if !ok {
		t.Fatal("blob_sha256 is not a string")
	}

	// Verify blob exists
	blobPath := filepath.Join(outputDir, blobHash[:2], blobHash)
	if _, err := os.Stat(blobPath); err != nil {
		t.Errorf("blob file not found: %v", err)
	}

	// Verify blob content matches original
	originalData, _ := os.ReadFile(zipPath)
	blobData, _ := os.ReadFile(blobPath)

	originalHash := sha256.Sum256(originalData)
	blobHashBytes := sha256.Sum256(blobData)

	if hex.EncodeToString(originalHash[:]) != hex.EncodeToString(blobHashBytes[:]) {
		t.Error("blob content does not match original file")
	}

	// Verify metadata
	metadata, ok := result["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata is not a map")
	}

	if metadata["format"] != "zip" {
		t.Errorf("expected format 'zip' in metadata, got %v", metadata["format"])
	}
	if metadata["original_name"] != "test.zip" {
		t.Errorf("expected original_name 'test.zip', got %v", metadata["original_name"])
	}
	if metadata["entry_count"] != "2" {
		t.Errorf("expected entry_count '2', got %v", metadata["entry_count"])
	}

	// Verify size
	sizeBytes, ok := result["size_bytes"].(float64)
	if !ok {
		t.Fatal("size_bytes is not a number")
	}
	if sizeBytes != float64(len(originalData)) {
		t.Errorf("expected size_bytes %d, got %v", len(originalData), sizeBytes)
	}
}

// TestIPCEnumerate tests the enumerate command via IPC.
func TestIPCEnumerate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath)

	req := ipc.Request{
		Command: "enumerate",
		Args:    map[string]interface{}{"path": zipPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	entries, ok := result["entries"].([]interface{})
	if !ok {
		t.Fatal("entries is not an array")
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	// Verify first entry
	entry1, ok := entries[0].(map[string]interface{})
	if !ok {
		t.Fatal("entry 1 is not a map")
	}
	if entry1["path"] != "test.txt" {
		t.Errorf("expected path 'test.txt', got %v", entry1["path"])
	}
	if entry1["size_bytes"] != float64(13) {
		t.Errorf("expected size_bytes 13, got %v", entry1["size_bytes"])
	}
	if entry1["is_dir"] != false {
		t.Errorf("expected is_dir false, got %v", entry1["is_dir"])
	}

	// Verify second entry
	entry2, ok := entries[1].(map[string]interface{})
	if !ok {
		t.Fatal("entry 2 is not a map")
	}
	if entry2["path"] != "subdir/file.txt" {
		t.Errorf("expected path 'subdir/file.txt', got %v", entry2["path"])
	}
}

// TestIPCUnknownCommand tests handling of unknown commands.
func TestIPCUnknownCommand(t *testing.T) {
	req := ipc.Request{
		Command: "unknown",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	if resp.Error != "unknown command: unknown" {
		t.Errorf("expected unknown command error, got %s", resp.Error)
	}
}

// TestIPCInvalidJSON tests handling of invalid JSON input.
func TestIPCInvalidJSON(t *testing.T) {
	pluginPath := buildPlugin(t)

	cmd := exec.Command(pluginPath)
	cmd.Stdin = bytes.NewReader([]byte("{invalid json}"))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Error("expected plugin to fail with invalid JSON")
	}

	// Should still return JSON error response
	var resp ipc.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
		if resp.Status != "error" {
			t.Errorf("expected error status, got %s", resp.Status)
		}
	}
}

// TestIPCDetectDirectory tests detecting a directory.
func TestIPCDetectDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": tmpDir},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] != false {
		t.Error("expected detected to be false for directory")
	}
	if result["reason"] != "path is a directory" {
		t.Errorf("expected directory reason, got %v", result["reason"])
	}
}

// TestIPCDetectTooSmall tests detecting a file too small to be ZIP.
func TestIPCDetectTooSmall(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	smallPath := filepath.Join(tmpDir, "small.zip")
	os.WriteFile(smallPath, []byte("PK"), 0644)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": smallPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] != false {
		t.Error("expected detected to be false for too small file")
	}
	if result["reason"] != "not a zip file (magic bytes mismatch)" {
		t.Errorf("expected magic mismatch reason, got %v", result["reason"])
	}
}

// TestIPCDetectNonExistent tests detecting a non-existent file.
func TestIPCDetectNonExistent(t *testing.T) {
	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": "/nonexistent/test.zip"},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] != false {
		t.Error("expected detected to be false for non-existent file")
	}
	// Reason should mention cannot open
	reason, _ := result["reason"].(string)
	if reason == "" {
		t.Error("expected reason to be set")
	}
}

// TestIPCDetectMissingPath tests detect with missing path argument.
func TestIPCDetectMissingPath(t *testing.T) {
	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{},
	}

	resp := executePluginExpectError(t, &req)
	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	if resp.Error != "path argument required" {
		t.Errorf("expected path required error, got %s", resp.Error)
	}
}

// TestIPCDetectInvalidPath tests detect with invalid path type.
func TestIPCDetectInvalidPath(t *testing.T) {
	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": 123},
	}

	resp := executePluginExpectError(t, &req)
	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	if resp.Error != "path argument required" {
		t.Errorf("expected path required error, got %s", resp.Error)
	}
}

// TestIPCEnumerateEmpty tests enumerating an empty ZIP file.
func TestIPCEnumerateEmpty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "empty.zip")
	createEmptyZip(t, zipPath)

	req := ipc.Request{
		Command: "enumerate",
		Args:    map[string]interface{}{"path": zipPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	// entries may be nil for empty zip
	entries := result["entries"]
	if entries == nil {
		// nil is acceptable for empty zip
		return
	}

	entriesArr, ok := entries.([]interface{})
	if !ok {
		t.Fatalf("entries is not an array: %T %+v", entries, entries)
	}

	if len(entriesArr) != 0 {
		t.Errorf("expected 0 entries for empty zip, got %d", len(entriesArr))
	}
}

// TestIPCEnumerateWithDirectories tests enumerating ZIP with directory entries.
func TestIPCEnumerateWithDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "dirs.zip")
	createZipWithDirectory(t, zipPath)

	req := ipc.Request{
		Command: "enumerate",
		Args:    map[string]interface{}{"path": zipPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	entries, ok := result["entries"].([]interface{})
	if !ok {
		t.Fatal("entries is not an array")
	}

	// Should have directory entry and file entry
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	// Find directory entry
	foundDir := false
	for _, e := range entries {
		entry := e.(map[string]interface{})
		if entry["path"] == "docs/" && entry["is_dir"] == true {
			foundDir = true
			break
		}
	}
	if !foundDir {
		t.Error("expected to find directory entry 'docs/'")
	}
}

// TestIPCEnumerateNonZip tests enumerating a non-ZIP file.
func TestIPCEnumerateNonZip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	txtPath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(txtPath, []byte("not a zip"), 0644)

	req := ipc.Request{
		Command: "enumerate",
		Args:    map[string]interface{}{"path": txtPath},
	}

	resp := executePluginExpectError(t, &req)
	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	// Error should mention failed to open zip
	if resp.Error == "" {
		t.Error("expected error message")
	}
}

// TestIPCEnumerateMissingPath tests enumerate with missing path.
func TestIPCEnumerateMissingPath(t *testing.T) {
	req := ipc.Request{
		Command: "enumerate",
		Args:    map[string]interface{}{},
	}

	resp := executePluginExpectError(t, &req)
	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	if resp.Error != "path argument required" {
		t.Errorf("expected path required error, got %s", resp.Error)
	}
}

// TestIPCIngestEmptyZip tests ingesting an empty ZIP file.
func TestIPCIngestEmptyZip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "empty.zip")
	createEmptyZip(t, zipPath)

	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       zipPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	// Verify metadata shows 0 entries
	metadata, ok := result["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata is not a map")
	}

	if metadata["entry_count"] != "0" {
		t.Errorf("expected entry_count '0', got %v", metadata["entry_count"])
	}
}

// TestIPCIngestNoExtension tests ingesting ZIP without .zip extension.
func TestIPCIngestNoExtension(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "archive")
	createTestZip(t, zipPath)

	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       zipPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	// Artifact ID should be the full filename
	if result["artifact_id"] != "archive" {
		t.Errorf("expected artifact_id 'archive', got %v", result["artifact_id"])
	}
}

// TestIPCIngestMissingPath tests ingest with missing path.
func TestIPCIngestMissingPath(t *testing.T) {
	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"output_dir": "/tmp/output",
		},
	}

	resp := executePluginExpectError(t, &req)
	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	if resp.Error != "path argument required" {
		t.Errorf("expected path required error, got %s", resp.Error)
	}
}

// TestIPCIngestMissingOutputDir tests ingest with missing output_dir.
func TestIPCIngestMissingOutputDir(t *testing.T) {
	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path": "/tmp/test.zip",
		},
	}

	resp := executePluginExpectError(t, &req)
	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	if resp.Error != "output_dir argument required" {
		t.Errorf("expected output_dir required error, got %s", resp.Error)
	}
}

// TestIPCIngestNonExistentFile tests ingesting non-existent file.
func TestIPCIngestNonExistentFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       "/nonexistent/test.zip",
			"output_dir": tmpDir,
		},
	}

	resp := executePluginExpectError(t, &req)
	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	// Error should mention failed to read file
	if resp.Error == "" {
		t.Error("expected error message")
	}
}

// TestIPCIngestInvalidOutputDir tests ingest with invalid output directory.
func TestIPCIngestInvalidOutputDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath)

	// Use a file as output dir (should fail)
	invalidDir := filepath.Join(tmpDir, "invalid")
	os.WriteFile(invalidDir, []byte("file"), 0644)

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       zipPath,
			"output_dir": filepath.Join(invalidDir, "subdir"),
		},
	}

	resp := executePluginExpectError(t, &req)
	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	// Error should mention failed to create blob dir
	if resp.Error == "" {
		t.Error("expected error message")
	}
}

// TestIPCIngestCorruptedZip tests ingesting a corrupted ZIP file.
func TestIPCIngestCorruptedZip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file with ZIP magic but corrupted data
	corruptPath := filepath.Join(tmpDir, "corrupt.zip")
	os.WriteFile(corruptPath, []byte("PK\x03\x04corrupted data"), 0644)

	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       corruptPath,
			"output_dir": outputDir,
		},
	}

	// Ingest should succeed (it just stores the file verbatim)
	// but entry_count will be 0 because zip.OpenReader fails
	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	// Check that metadata shows entry_count 0 (because zip reading failed)
	metadata, ok := result["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata is not a map")
	}

	if metadata["entry_count"] != "0" {
		t.Errorf("expected entry_count '0' for corrupted zip, got %v", metadata["entry_count"])
	}

	// But the blob should still be stored
	blobHash, _ := result["blob_sha256"].(string)
	blobPath := filepath.Join(outputDir, blobHash[:2], blobHash)
	if _, err := os.Stat(blobPath); err != nil {
		t.Errorf("blob file not found: %v", err)
	}
}

// Helper functions

// buildPlugin builds the plugin binary if it doesn't exist.
func buildPlugin(t *testing.T) string {
	t.Helper()

	pluginPath := "./format-zip"
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
		if err := buildCmd.Run(); err != nil {
			t.Fatalf("failed to build plugin: %v", err)
		}
	}
	return pluginPath
}

// executePlugin runs the plugin with a request and returns the response.
func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()

	pluginPath := buildPlugin(t)

	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	cmd := exec.Command(pluginPath)
	cmd.Stdin = bytes.NewReader(reqData)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stdout.Len() > 0 {
			var resp ipc.Response
			if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
				return &resp
			}
		}
		t.Fatalf("plugin execution failed: %v\nstderr: %s", err, stderr.String())
	}

	var resp ipc.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\noutput: %s", err, stdout.String())
	}

	return &resp
}

// executePluginExpectError runs the plugin expecting an error response.
func executePluginExpectError(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()

	pluginPath := buildPlugin(t)

	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	cmd := exec.Command(pluginPath)
	cmd.Stdin = bytes.NewReader(reqData)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Error is expected, so we don't fail on non-zero exit
	cmd.Run()

	var resp ipc.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\noutput: %s", err, stdout.String())
	}

	return &resp
}
