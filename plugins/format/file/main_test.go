package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// executePlugin runs the plugin with a request and returns the response.
func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()

	pluginPath := "./format-file"
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
		if err := buildCmd.Run(); err != nil {
			t.Fatalf("failed to build plugin: %v", err)
		}
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	cmd := exec.Command(pluginPath)
	cmd.Stdin = bytes.NewReader(reqData)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command - don't fail on non-zero exit as error responses are valid
	cmd.Run()

	// Try to parse response even if there was an error exit code
	if stdout.Len() > 0 {
		var resp ipc.Response
		if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
			return &resp
		}
	}

	t.Fatalf("plugin execution failed to produce valid output\nstderr: %s\nstdout: %s", stderr.String(), stdout.String())
	return nil
}

// TestDetect tests the detect command with various scenarios
func TestDetect(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func() (string, func())
		wantDetected   bool
		wantFormat     string
		wantReasonText string
	}{
		{
			name: "regular file detected",
			setupFunc: func() (string, func()) {
				tmpFile, err := os.CreateTemp("", "test-*.txt")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile.WriteString("test content")
				tmpFile.Close()
				return tmpFile.Name(), func() { os.Remove(tmpFile.Name()) }
			},
			wantDetected:   true,
			wantFormat:     "file",
			wantReasonText: "single file detected",
		},
		{
			name: "empty file detected",
			setupFunc: func() (string, func()) {
				tmpFile, err := os.CreateTemp("", "test-*.txt")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile.Close()
				return tmpFile.Name(), func() { os.Remove(tmpFile.Name()) }
			},
			wantDetected:   true,
			wantFormat:     "file",
			wantReasonText: "single file detected",
		},
		{
			name: "binary file detected",
			setupFunc: func() (string, func()) {
				tmpFile, err := os.CreateTemp("", "test-*.bin")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile.Write([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE})
				tmpFile.Close()
				return tmpFile.Name(), func() { os.Remove(tmpFile.Name()) }
			},
			wantDetected:   true,
			wantFormat:     "file",
			wantReasonText: "single file detected",
		},
		{
			name: "directory not detected",
			setupFunc: func() (string, func()) {
				tmpDir, err := os.MkdirTemp("", "test-dir-*")
				if err != nil {
					t.Fatal(err)
				}
				return tmpDir, func() { os.RemoveAll(tmpDir) }
			},
			wantDetected:   false,
			wantFormat:     "",
			wantReasonText: "path is a directory, not a file",
		},
		{
			name: "nonexistent file not detected",
			setupFunc: func() (string, func()) {
				return "/nonexistent/path/to/file.txt", func() {}
			},
			wantDetected:   false,
			wantFormat:     "",
			wantReasonText: "cannot stat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, cleanup := tt.setupFunc()
			defer cleanup()

			req := ipc.Request{
				Command: "detect",
				Args:    map[string]interface{}{"path": path},
			}

			resp := executePlugin(t, &req)

			if resp.Status != "ok" {
				t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
			}

			result, ok := resp.Result.(map[string]interface{})
			if !ok {
				t.Fatal("result is not a map")
			}

			detected, _ := result["detected"].(bool)
			if detected != tt.wantDetected {
				t.Errorf("detected = %v, want %v", detected, tt.wantDetected)
			}

			format, _ := result["format"].(string)
			if format != tt.wantFormat {
				t.Errorf("format = %v, want %v", format, tt.wantFormat)
			}

			reason, _ := result["reason"].(string)
			if tt.wantReasonText != "" {
				if tt.name == "nonexistent file not detected" {
					// Just check prefix for error messages
					if !strings.HasPrefix(reason, "cannot stat") {
						t.Errorf("reason does not start with expected text: got %v", reason)
					}
				} else if reason != tt.wantReasonText {
					t.Errorf("reason = %v, want %v", reason, tt.wantReasonText)
				}
			}
		})
	}
}

// TestDetectMissingPath tests detect with missing path argument
func TestDetectMissingPath(t *testing.T) {
	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, &req)

	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	if resp.Error != "path argument required" {
		t.Errorf("error = %v, want 'path argument required'", resp.Error)
	}
}

// TestDetectInvalidPathType tests detect with non-string path
func TestDetectInvalidPathType(t *testing.T) {
	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": 123},
	}

	resp := executePlugin(t, &req)

	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	if resp.Error != "path argument required" {
		t.Errorf("error = %v, want 'path argument required'", resp.Error)
	}
}

// TestIngest tests the ingest function
func TestIngest(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() (path string, outputDir string, expectedFilename string, cleanup func())
		fileContent string
		wantErr     bool
		errPrefix   string
	}{
		{
			name: "successful ingest",
			setupFunc: func() (string, string, string, func()) {
				tmpDir, err := os.MkdirTemp("", "test-*")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile := filepath.Join(tmpDir, "myfile.txt")
				if err := os.WriteFile(tmpFile, []byte("hello world"), 0600); err != nil {
					t.Fatal(err)
				}

				outputDir, err := os.MkdirTemp("", "output-*")
				if err != nil {
					t.Fatal(err)
				}

				return tmpFile, outputDir, "myfile", func() {
					os.RemoveAll(tmpDir)
					os.RemoveAll(outputDir)
				}
			},
			fileContent: "hello world",
			wantErr:     false,
		},
		{
			name: "ingest file without extension",
			setupFunc: func() (string, string, string, func()) {
				tmpDir, err := os.MkdirTemp("", "test-*")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile := filepath.Join(tmpDir, "README")
				if err := os.WriteFile(tmpFile, []byte("content"), 0600); err != nil {
					t.Fatal(err)
				}

				outputDir, err := os.MkdirTemp("", "output-*")
				if err != nil {
					t.Fatal(err)
				}

				return tmpFile, outputDir, "README", func() {
					os.RemoveAll(tmpDir)
					os.RemoveAll(outputDir)
				}
			},
			fileContent: "content",
			wantErr:     false,
		},
		{
			name: "ingest file with multiple dots",
			setupFunc: func() (string, string, string, func()) {
				tmpDir, err := os.MkdirTemp("", "test-*")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile := filepath.Join(tmpDir, "archive.tar.gz")
				if err := os.WriteFile(tmpFile, []byte("archive"), 0600); err != nil {
					t.Fatal(err)
				}

				outputDir, err := os.MkdirTemp("", "output-*")
				if err != nil {
					t.Fatal(err)
				}

				return tmpFile, outputDir, "archive.tar", func() {
					os.RemoveAll(tmpDir)
					os.RemoveAll(outputDir)
				}
			},
			fileContent: "archive",
			wantErr:     false,
		},
		{
			name: "ingest hidden file",
			setupFunc: func() (string, string, string, func()) {
				tmpDir, err := os.MkdirTemp("", "test-*")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile := filepath.Join(tmpDir, ".gitignore")
				if err := os.WriteFile(tmpFile, []byte("*.log"), 0600); err != nil {
					t.Fatal(err)
				}

				outputDir, err := os.MkdirTemp("", "output-*")
				if err != nil {
					t.Fatal(err)
				}

				return tmpFile, outputDir, ".gitignore", func() {
					os.RemoveAll(tmpDir)
					os.RemoveAll(outputDir)
				}
			},
			fileContent: "*.log",
			wantErr:     false,
		},
		{
			name: "ingest empty file",
			setupFunc: func() (string, string, string, func()) {
				tmpDir, err := os.MkdirTemp("", "test-*")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile := filepath.Join(tmpDir, "empty.txt")
				if err := os.WriteFile(tmpFile, []byte(""), 0600); err != nil {
					t.Fatal(err)
				}

				outputDir, err := os.MkdirTemp("", "output-*")
				if err != nil {
					t.Fatal(err)
				}

				return tmpFile, outputDir, "empty", func() {
					os.RemoveAll(tmpDir)
					os.RemoveAll(outputDir)
				}
			},
			fileContent: "",
			wantErr:     false,
		},
		{
			name: "ingest large file",
			setupFunc: func() (string, string, string, func()) {
				tmpDir, err := os.MkdirTemp("", "test-*")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile := filepath.Join(tmpDir, "large.bin")
				// Write 1MB of data
				data := bytes.Repeat([]byte("A"), 1024*1024)
				if err := os.WriteFile(tmpFile, data, 0600); err != nil {
					t.Fatal(err)
				}

				outputDir, err := os.MkdirTemp("", "output-*")
				if err != nil {
					t.Fatal(err)
				}

				return tmpFile, outputDir, "large", func() {
					os.RemoveAll(tmpDir)
					os.RemoveAll(outputDir)
				}
			},
			fileContent: string(bytes.Repeat([]byte("A"), 1024*1024)),
			wantErr:     false,
		},
		{
			name: "ingest nonexistent file",
			setupFunc: func() (string, string, string, func()) {
				outputDir, err := os.MkdirTemp("", "output-*")
				if err != nil {
					t.Fatal(err)
				}
				return "/nonexistent/file.txt", outputDir, "", func() {
					os.RemoveAll(outputDir)
				}
			},
			wantErr:   true,
			errPrefix: "failed to read file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, outputDir, expectedFilename, cleanup := tt.setupFunc()
			defer cleanup()

			req := ipc.Request{
				Command: "ingest",
				Args: map[string]interface{}{
					"path":       path,
					"output_dir": outputDir,
				},
			}

			resp := executePlugin(t, &req)

			if tt.wantErr {
				if resp.Status != "error" {
					t.Errorf("expected status error, got %s", resp.Status)
				}
				if tt.errPrefix != "" && !strings.HasPrefix(resp.Error, tt.errPrefix) {
					t.Errorf("error does not start with expected prefix: got %v, want prefix %v", resp.Error, tt.errPrefix)
				}
			} else {
				if resp.Status != "ok" {
					t.Errorf("expected status ok, got %s: %s", resp.Status, resp.Error)
				}

				result, ok := resp.Result.(map[string]interface{})
				if !ok {
					t.Fatal("result is not a map")
				}

				// Verify artifact ID
				artifactID, _ := result["artifact_id"].(string)
				if artifactID != expectedFilename {
					t.Errorf("artifact_id = %v, want %v", artifactID, expectedFilename)
				}

				// Verify blob SHA256
				blobSHA256, _ := result["blob_sha256"].(string)
				expectedHash := sha256.Sum256([]byte(tt.fileContent))
				expectedHashHex := hex.EncodeToString(expectedHash[:])
				if blobSHA256 != expectedHashHex {
					t.Errorf("BlobSHA256 = %v, want %v", blobSHA256, expectedHashHex)
				}

				// Verify size
				sizeBytes, _ := result["size_bytes"].(float64)
				if int64(sizeBytes) != int64(len(tt.fileContent)) {
					t.Errorf("SizeBytes = %v, want %v", sizeBytes, len(tt.fileContent))
				}

				// Verify blob was written
				blobPath := filepath.Join(outputDir, blobSHA256[:2], blobSHA256)
				blobData, err := os.ReadFile(blobPath)
				if err != nil {
					t.Errorf("failed to read blob: %v", err)
				}
				if string(blobData) != tt.fileContent {
					t.Errorf("blob content length = %v, want %v", len(blobData), len(tt.fileContent))
				}

				// Verify metadata
				metadata, ok := result["metadata"].(map[string]interface{})
				if !ok {
					t.Error("metadata is not a map")
				} else if _, ok := metadata["original_name"]; !ok {
					t.Error("metadata missing original_name")
				}
			}
		})
	}
}

// TestIngestMissingArguments tests ingest with missing arguments
func TestIngestMissingArguments(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr string
	}{
		{
			name:    "missing path",
			args:    map[string]interface{}{"output_dir": "/tmp"},
			wantErr: "path argument required",
		},
		{
			name:    "missing output_dir",
			args:    map[string]interface{}{"path": "/tmp/file.txt"},
			wantErr: "output_dir argument required",
		},
		{
			name:    "missing both arguments",
			args:    map[string]interface{}{},
			wantErr: "path argument required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ipc.Request{
				Command: "ingest",
				Args:    tt.args,
			}

			resp := executePlugin(t, &req)

			if resp.Status != "error" {
				t.Errorf("expected status error, got %s", resp.Status)
			}
			if resp.Error != tt.wantErr {
				t.Errorf("error = %v, want %v", resp.Error, tt.wantErr)
			}
		})
	}
}

// TestIngestInvalidArgTypes tests ingest with invalid argument types
func TestIngestInvalidArgTypes(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr string
	}{
		{
			name:    "path is not string",
			args:    map[string]interface{}{"path": 123, "output_dir": "/tmp"},
			wantErr: "path argument required",
		},
		{
			name:    "output_dir is not string",
			args:    map[string]interface{}{"path": "/tmp/file.txt", "output_dir": 456},
			wantErr: "output_dir argument required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ipc.Request{
				Command: "ingest",
				Args:    tt.args,
			}

			resp := executePlugin(t, &req)

			if resp.Status != "error" {
				t.Errorf("expected status error, got %s", resp.Status)
			}
			if resp.Error != tt.wantErr {
				t.Errorf("error = %v, want %v", resp.Error, tt.wantErr)
			}
		})
	}
}

// TestEnumerate tests the enumerate function
func TestEnumerate(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() (path string, cleanup func())
		wantErr     bool
		errPrefix   string
		fileContent string
		fileName    string
	}{
		{
			name: "enumerate single file",
			setupFunc: func() (string, func()) {
				tmpFile, err := os.CreateTemp("", "test-*.txt")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile.WriteString("test data")
				tmpFile.Close()
				return tmpFile.Name(), func() { os.Remove(tmpFile.Name()) }
			},
			wantErr:     false,
			fileContent: "test data",
		},
		{
			name: "enumerate empty file",
			setupFunc: func() (string, func()) {
				tmpFile, err := os.CreateTemp("", "test-*.txt")
				if err != nil {
					t.Fatal(err)
				}
				tmpFile.Close()
				return tmpFile.Name(), func() { os.Remove(tmpFile.Name()) }
			},
			wantErr:     false,
			fileContent: "",
		},
		{
			name: "enumerate large file",
			setupFunc: func() (string, func()) {
				tmpFile, err := os.CreateTemp("", "test-*.bin")
				if err != nil {
					t.Fatal(err)
				}
				data := bytes.Repeat([]byte("X"), 10*1024*1024) // 10MB
				tmpFile.Write(data)
				tmpFile.Close()
				return tmpFile.Name(), func() { os.Remove(tmpFile.Name()) }
			},
			wantErr:     false,
			fileContent: string(bytes.Repeat([]byte("X"), 10*1024*1024)),
		},
		{
			name: "enumerate nonexistent file",
			setupFunc: func() (string, func()) {
				return "/nonexistent/file.txt", func() {}
			},
			wantErr:   true,
			errPrefix: "failed to stat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, cleanup := tt.setupFunc()
			defer cleanup()

			req := ipc.Request{
				Command: "enumerate",
				Args:    map[string]interface{}{"path": path},
			}

			resp := executePlugin(t, &req)

			if tt.wantErr {
				if resp.Status != "error" {
					t.Errorf("expected status error, got %s", resp.Status)
				}
				if tt.errPrefix != "" && !strings.HasPrefix(resp.Error, tt.errPrefix) {
					t.Errorf("error does not start with expected prefix: got %v, want prefix %v", resp.Error, tt.errPrefix)
				}
			} else {
				if resp.Status != "ok" {
					t.Errorf("expected status ok, got %s: %s", resp.Status, resp.Error)
				}

				result, ok := resp.Result.(map[string]interface{})
				if !ok {
					t.Fatal("result is not a map")
				}

				entries, ok := result["entries"].([]interface{})
				if !ok {
					t.Fatal("entries is not an array")
				}

				if len(entries) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(entries))
				}

				entry := entries[0].(map[string]interface{})

				// Verify path is just the basename
				entryPath, _ := entry["path"].(string)
				if entryPath != filepath.Base(path) {
					t.Errorf("Path = %v, want %v", entryPath, filepath.Base(path))
				}

				// Verify size
				sizeBytes, _ := entry["size_bytes"].(float64)
				if int64(sizeBytes) != int64(len(tt.fileContent)) {
					t.Errorf("SizeBytes = %v, want %v", sizeBytes, len(tt.fileContent))
				}

				// Verify is_dir is false
				isDir, _ := entry["is_dir"].(bool)
				if isDir {
					t.Error("IsDir should be false")
				}
			}
		})
	}
}

// TestEnumerateMissingPath tests enumerate with missing path argument
func TestEnumerateMissingPath(t *testing.T) {
	req := ipc.Request{
		Command: "enumerate",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, &req)

	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	if resp.Error != "path argument required" {
		t.Errorf("error = %v, want 'path argument required'", resp.Error)
	}
}

// TestEnumerateInvalidPathType tests enumerate with non-string path
func TestEnumerateInvalidPathType(t *testing.T) {
	req := ipc.Request{
		Command: "enumerate",
		Args:    map[string]interface{}{"path": []string{"invalid"}},
	}

	resp := executePlugin(t, &req)

	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	if resp.Error != "path argument required" {
		t.Errorf("error = %v, want 'path argument required'", resp.Error)
	}
}

// TestIPCInvalidJSON tests handling of invalid JSON input
func TestIPCInvalidJSON(t *testing.T) {
	pluginPath := "./format-file"
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
		if err := buildCmd.Run(); err != nil {
			t.Fatalf("failed to build plugin: %v", err)
		}
	}

	cmd := exec.Command(pluginPath)
	cmd.Stdin = bytes.NewReader([]byte("{invalid json}"))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Run()

	var resp ipc.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	if !strings.HasPrefix(resp.Error, "failed to decode request") {
		t.Errorf("error = %v, want 'failed to decode request' prefix", resp.Error)
	}
}

// TestIPCUnknownCommand tests handling of unknown commands
func TestIPCUnknownCommand(t *testing.T) {
	req := ipc.Request{
		Command: "unknown_command",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, &req)

	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	if resp.Error != "unknown command: unknown_command" {
		t.Errorf("error = %v, want 'unknown command: unknown_command'", resp.Error)
	}
}

// TestIPCEmptyCommand tests handling of empty command
func TestIPCEmptyCommand(t *testing.T) {
	req := ipc.Request{
		Command: "",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, &req)

	if resp.Status != "error" {
		t.Errorf("expected status error, got %s", resp.Status)
	}
	if resp.Error != "unknown command: " {
		t.Errorf("error = %v, want 'unknown command: '", resp.Error)
	}
}

// TestBlobHashingConsistency verifies that the same content always produces the same hash
func TestBlobHashingConsistency(t *testing.T) {
	content := "test content for hashing"

	tmpFile1, err := os.CreateTemp("", "test1-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile1.WriteString(content)
	tmpFile1.Close()
	defer os.Remove(tmpFile1.Name())

	tmpFile2, err := os.CreateTemp("", "test2-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile2.WriteString(content)
	tmpFile2.Close()
	defer os.Remove(tmpFile2.Name())

	outputDir, err := os.MkdirTemp("", "output-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outputDir)

	// Ingest first file
	req1 := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       tmpFile1.Name(),
			"output_dir": outputDir,
		},
	}
	resp1 := executePlugin(t, &req1)

	// Ingest second file
	req2 := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       tmpFile2.Name(),
			"output_dir": outputDir,
		},
	}
	resp2 := executePlugin(t, &req2)

	result1 := resp1.Result.(map[string]interface{})
	result2 := resp2.Result.(map[string]interface{})

	hash1, _ := result1["blob_sha256"].(string)
	hash2, _ := result2["blob_sha256"].(string)

	if hash1 != hash2 {
		t.Errorf("same content produced different hashes: %v vs %v", hash1, hash2)
	}
}

// TestBlobDeduplication verifies that identical content is stored only once
func TestBlobDeduplication(t *testing.T) {
	content := "duplicate content"

	tmpFile1, err := os.CreateTemp("", "dup1-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile1.WriteString(content)
	tmpFile1.Close()
	defer os.Remove(tmpFile1.Name())

	tmpFile2, err := os.CreateTemp("", "dup2-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile2.WriteString(content)
	tmpFile2.Close()
	defer os.Remove(tmpFile2.Name())

	outputDir, err := os.MkdirTemp("", "output-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outputDir)

	// Ingest both files
	req1 := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       tmpFile1.Name(),
			"output_dir": outputDir,
		},
	}
	resp1 := executePlugin(t, &req1)

	req2 := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       tmpFile2.Name(),
			"output_dir": outputDir,
		},
	}
	resp2 := executePlugin(t, &req2)

	result1 := resp1.Result.(map[string]interface{})
	result2 := resp2.Result.(map[string]interface{})

	hash1, _ := result1["blob_sha256"].(string)
	hash2, _ := result2["blob_sha256"].(string)

	// Verify same hash
	if hash1 != hash2 {
		t.Errorf("duplicate content produced different hashes")
	}

	// Verify blob exists and wasn't corrupted by second write
	blobPath := filepath.Join(outputDir, hash1[:2], hash1)
	blobData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("failed to read blob: %v", err)
	}
	if string(blobData) != content {
		t.Error("blob content was corrupted")
	}
}

// TestSpecialCharactersInFilename tests handling of special characters
func TestSpecialCharactersInFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantID   string
	}{
		{
			name:     "spaces in name",
			filename: "file with spaces.txt",
			wantID:   "file with spaces",
		},
		{
			name:     "unicode characters",
			filename: "файл.txt",
			wantID:   "файл",
		},
		{
			name:     "dashes and underscores",
			filename: "my-test_file.dat",
			wantID:   "my-test_file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "test-*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)

			tmpFile := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(tmpFile, []byte("content"), 0600); err != nil {
				t.Fatal(err)
			}

			outputDir, err := os.MkdirTemp("", "output-*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(outputDir)

			req := ipc.Request{
				Command: "ingest",
				Args: map[string]interface{}{
					"path":       tmpFile,
					"output_dir": outputDir,
				},
			}

			resp := executePlugin(t, &req)

			if resp.Status != "ok" {
				t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
			}

			result := resp.Result.(map[string]interface{})
			artifactID, _ := result["artifact_id"].(string)

			if artifactID != tt.wantID {
				t.Errorf("artifact_id = %v, want %v", artifactID, tt.wantID)
			}
		})
	}
}
