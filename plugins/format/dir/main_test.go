package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// TestHandleDetect tests the detect() function with various scenarios
func TestHandleDetect(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(t *testing.T) string
		args           map[string]interface{}
		wantDetected   bool
		wantFormat     string
		wantReasonLike string
		wantError      bool
	}{
		{
			name: "valid directory",
			setupFunc: func(t *testing.T) string {
				return t.TempDir()
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": ""}
			}(),
			wantDetected:   true,
			wantFormat:     "dir",
			wantReasonLike: "directory detected",
			wantError:      false,
		},
		{
			name: "file not directory",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				file := filepath.Join(dir, "test.txt")
				if err := os.WriteFile(file, []byte("test"), 0600); err != nil {
					t.Fatal(err)
				}
				return file
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": ""}
			}(),
			wantDetected:   false,
			wantFormat:     "",
			wantReasonLike: "not a directory",
			wantError:      false,
		},
		{
			name: "non-existent path",
			setupFunc: func(t *testing.T) string {
				return "/nonexistent/path/that/does/not/exist"
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": ""}
			}(),
			wantDetected:   false,
			wantFormat:     "",
			wantReasonLike: "cannot stat",
			wantError:      false,
		},
		{
			name: "missing path argument",
			setupFunc: func(t *testing.T) string {
				return ""
			},
			args:      map[string]interface{}{},
			wantError: true,
		},
		{
			name: "invalid path type",
			setupFunc: func(t *testing.T) string {
				return ""
			},
			args: map[string]interface{}{
				"path": 123,
			},
			wantError: true,
		},
		{
			name: "empty directory",
			setupFunc: func(t *testing.T) string {
				return t.TempDir()
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": ""}
			}(),
			wantDetected:   true,
			wantFormat:     "dir",
			wantReasonLike: "directory detected",
			wantError:      false,
		},
		{
			name: "nested directory",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				subdir := filepath.Join(dir, "sub1", "sub2")
				os.MkdirAll(subdir, 0755)
				return subdir
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": ""}
			}(),
			wantDetected:   true,
			wantFormat:     "dir",
			wantReasonLike: "directory detected",
			wantError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setupFunc(t)
			if path != "" {
				tt.args["path"] = path
			}

			req := ipc.Request{
				Command: "detect",
				Args:    tt.args,
			}

			resp := executePlugin(t, &req)

			if tt.wantError {
				if resp.Status == "ok" {
					t.Fatal("expected error, got ok")
				}
				return
			}

			if resp.Status != "ok" {
				t.Fatalf("unexpected error: %s", resp.Error)
			}

			result, ok := resp.Result.(map[string]interface{})
			if !ok {
				t.Fatal("result is not a map")
			}

			detected := result["detected"].(bool)
			if detected != tt.wantDetected {
				t.Errorf("Detected = %v, want %v", detected, tt.wantDetected)
			}

			format, _ := result["format"].(string)
			if format != tt.wantFormat {
				t.Errorf("Format = %v, want %v", format, tt.wantFormat)
			}

			if tt.wantReasonLike != "" {
				reason, _ := result["reason"].(string)
				if !strings.Contains(reason, tt.wantReasonLike) {
					t.Errorf("Reason = %q, want to contain %q", reason, tt.wantReasonLike)
				}
			}
		})
	}
}

// TestHandleEnumerate tests the enumerate() function
func TestHandleEnumerate(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) string
		args      map[string]interface{}
		wantCount int
		wantError bool
		checkFunc func(t *testing.T, entries []interface{})
	}{
		{
			name: "empty directory",
			setupFunc: func(t *testing.T) string {
				return t.TempDir()
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": ""}
			}(),
			wantCount: 0,
			wantError: false,
		},
		{
			name: "directory with files",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0600)
				os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("content2"), 0600)
				return dir
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": ""}
			}(),
			wantCount: 2,
			wantError: false,
			checkFunc: func(t *testing.T, entries []interface{}) {
				foundFile1 := false
				foundFile2 := false
				for _, e := range entries {
					entry := e.(map[string]interface{})
					path := entry["path"].(string)
					if path == "file1.txt" {
						foundFile1 = true
						if entry["is_dir"].(bool) {
							t.Error("file1.txt should not be a directory")
						}
						if int64(entry["size_bytes"].(float64)) != 8 {
							t.Errorf("file1.txt size = %v, want 8", entry["size_bytes"])
						}
					}
					if path == "file2.txt" {
						foundFile2 = true
						if entry["is_dir"].(bool) {
							t.Error("file2.txt should not be a directory")
						}
					}
				}
				if !foundFile1 {
					t.Error("file1.txt not found in entries")
				}
				if !foundFile2 {
					t.Error("file2.txt not found in entries")
				}
			},
		},
		{
			name: "nested directory structure",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				subdir := filepath.Join(dir, "subdir")
				os.MkdirAll(subdir, 0755)
				os.WriteFile(filepath.Join(dir, "root.txt"), []byte("root"), 0600)
				os.WriteFile(filepath.Join(subdir, "nested.txt"), []byte("nested"), 0600)
				return dir
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": ""}
			}(),
			wantCount: 3, // subdir, root.txt, subdir/nested.txt
			wantError: false,
			checkFunc: func(t *testing.T, entries []interface{}) {
				paths := make(map[string]bool)
				for _, e := range entries {
					entry := e.(map[string]interface{})
					path := entry["path"].(string)
					isDir := entry["is_dir"].(bool)
					paths[path] = isDir
				}
				if !paths["subdir"] {
					t.Error("subdir not found or not marked as directory")
				}
				if _, ok := paths["root.txt"]; !ok {
					t.Error("root.txt not found")
				}
				if _, ok := paths[filepath.Join("subdir", "nested.txt")]; !ok {
					t.Error("subdir/nested.txt not found")
				}
			},
		},
		{
			name: "multiple levels of nesting",
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				level1 := filepath.Join(dir, "level1")
				level2 := filepath.Join(level1, "level2")
				level3 := filepath.Join(level2, "level3")
				os.MkdirAll(level3, 0755)
				os.WriteFile(filepath.Join(level3, "deep.txt"), []byte("deep"), 0600)
				return dir
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": ""}
			}(),
			wantCount: 4, // level1, level1/level2, level1/level2/level3, level1/level2/level3/deep.txt
			wantError: false,
		},
		{
			name: "non-existent path",
			setupFunc: func(t *testing.T) string {
				return "/nonexistent/path"
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": ""}
			}(),
			wantError: true,
		},
		{
			name: "missing path argument",
			setupFunc: func(t *testing.T) string {
				return ""
			},
			args:      map[string]interface{}{},
			wantError: true,
		},
		{
			name: "invalid path type",
			setupFunc: func(t *testing.T) string {
				return ""
			},
			args: map[string]interface{}{
				"path": []string{"invalid"},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setupFunc(t)
			if path != "" {
				tt.args["path"] = path
			}

			req := ipc.Request{
				Command: "enumerate",
				Args:    tt.args,
			}

			resp := executePlugin(t, &req)

			if tt.wantError {
				if resp.Status == "ok" {
					t.Fatal("expected error, got ok")
				}
				return
			}

			if resp.Status != "ok" {
				t.Fatalf("unexpected error: %s", resp.Error)
			}

			result, ok := resp.Result.(map[string]interface{})
			if !ok {
				t.Fatal("result is not a map")
			}

			// Handle both null and empty array for entries
			var entries []interface{}
			if result["entries"] != nil {
				entries, ok = result["entries"].([]interface{})
				if !ok {
					t.Fatal("entries is not an array")
				}
			}

			if len(entries) != tt.wantCount {
				t.Errorf("got %d entries, want %d", len(entries), tt.wantCount)
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, entries)
			}
		})
	}
}

// TestHandleIngest tests the ingest() function
func TestHandleIngest(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) (string, string)
		args      map[string]interface{}
		wantError bool
		checkFunc func(t *testing.T, result map[string]interface{}, outputDir string)
	}{
		{
			name: "empty directory",
			setupFunc: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				outputDir := t.TempDir()
				return dir, outputDir
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": "", "output_dir": ""}
			}(),
			wantError: false,
			checkFunc: func(t *testing.T, result map[string]interface{}, outputDir string) {
				blobSHA := result["blob_sha256"].(string)
				if blobSHA == "" {
					t.Error("expected non-empty blob SHA256")
				}

				metadata := result["metadata"].(map[string]interface{})
				if metadata["format"] != "dir" {
					t.Errorf("format = %v, want dir", metadata["format"])
				}
				if metadata["file_count"] != "0" {
					t.Errorf("file_count = %v, want 0", metadata["file_count"])
				}

				// Verify blob was written
				blobPath := filepath.Join(outputDir, blobSHA[:2], blobSHA)
				if _, err := os.Stat(blobPath); err != nil {
					t.Errorf("blob file not found: %v", err)
				}
			},
		},
		{
			name: "directory with files",
			setupFunc: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				outputDir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0600)
				os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("content2"), 0600)
				return dir, outputDir
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": "", "output_dir": ""}
			}(),
			wantError: false,
			checkFunc: func(t *testing.T, result map[string]interface{}, outputDir string) {
				blobSHA := result["blob_sha256"].(string)
				if blobSHA == "" {
					t.Error("expected non-empty blob SHA256")
				}

				metadata := result["metadata"].(map[string]interface{})
				if metadata["file_count"] != "2" {
					t.Errorf("file_count = %v, want 2", metadata["file_count"])
				}

				// Verify blob was written and contains manifest
				blobPath := filepath.Join(outputDir, blobSHA[:2], blobSHA)
				data, err := os.ReadFile(blobPath)
				if err != nil {
					t.Fatalf("failed to read blob: %v", err)
				}

				var manifest struct {
					RootPath string   `json:"root_path"`
					Files    []string `json:"files"`
				}
				if err := json.Unmarshal(data, &manifest); err != nil {
					t.Fatalf("failed to unmarshal manifest: %v", err)
				}

				if len(manifest.Files) != 2 {
					t.Errorf("manifest has %d files, want 2", len(manifest.Files))
				}
			},
		},
		{
			name: "nested directory",
			setupFunc: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				outputDir := t.TempDir()
				subdir := filepath.Join(dir, "subdir")
				os.MkdirAll(subdir, 0755)
				os.WriteFile(filepath.Join(dir, "root.txt"), []byte("root content"), 0600)
				os.WriteFile(filepath.Join(subdir, "nested.txt"), []byte("nested content"), 0600)
				return dir, outputDir
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": "", "output_dir": ""}
			}(),
			wantError: false,
			checkFunc: func(t *testing.T, result map[string]interface{}, outputDir string) {
				metadata := result["metadata"].(map[string]interface{})
				if metadata["file_count"] != "2" {
					t.Errorf("file_count = %v, want 2", metadata["file_count"])
				}

				// Verify manifest contains both files
				blobSHA := result["blob_sha256"].(string)
				blobPath := filepath.Join(outputDir, blobSHA[:2], blobSHA)
				data, err := os.ReadFile(blobPath)
				if err != nil {
					t.Fatalf("failed to read blob: %v", err)
				}

				var manifest struct {
					RootPath string   `json:"root_path"`
					Files    []string `json:"files"`
				}
				if err := json.Unmarshal(data, &manifest); err != nil {
					t.Fatalf("failed to unmarshal manifest: %v", err)
				}

				foundRoot := false
				foundNested := false
				for _, f := range manifest.Files {
					if f == "root.txt" {
						foundRoot = true
					}
					if f == filepath.Join("subdir", "nested.txt") {
						foundNested = true
					}
				}
				if !foundRoot {
					t.Error("root.txt not found in manifest")
				}
				if !foundNested {
					t.Error("subdir/nested.txt not found in manifest")
				}
			},
		},
		{
			name: "verify artifact ID",
			setupFunc: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				testDir := filepath.Join(dir, "myartifact")
				os.MkdirAll(testDir, 0755)
				outputDir := t.TempDir()
				return testDir, outputDir
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": "", "output_dir": ""}
			}(),
			wantError: false,
			checkFunc: func(t *testing.T, result map[string]interface{}, outputDir string) {
				artifactID := result["artifact_id"].(string)
				if artifactID != "myartifact" {
					t.Errorf("artifact_id = %v, want myartifact", artifactID)
				}
			},
		},
		{
			name: "verify total_bytes metadata",
			setupFunc: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				outputDir := t.TempDir()
				os.WriteFile(filepath.Join(dir, "file.txt"), []byte("12345"), 0600)
				return dir, outputDir
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": "", "output_dir": ""}
			}(),
			wantError: false,
			checkFunc: func(t *testing.T, result map[string]interface{}, outputDir string) {
				metadata := result["metadata"].(map[string]interface{})
				if metadata["total_bytes"] != "5" {
					t.Errorf("total_bytes = %v, want 5", metadata["total_bytes"])
				}
			},
		},
		{
			name: "missing path argument",
			setupFunc: func(t *testing.T) (string, string) {
				return "", t.TempDir()
			},
			args: map[string]interface{}{
				"output_dir": "",
			},
			wantError: true,
		},
		{
			name: "missing output_dir argument",
			setupFunc: func(t *testing.T) (string, string) {
				return t.TempDir(), ""
			},
			args: map[string]interface{}{
				"path": "",
			},
			wantError: true,
		},
		{
			name: "non-existent path",
			setupFunc: func(t *testing.T) (string, string) {
				return "/nonexistent/path", t.TempDir()
			},
			args: func() map[string]interface{} {
				return map[string]interface{}{"path": "", "output_dir": ""}
			}(),
			wantError: true,
		},
		{
			name: "invalid path type",
			setupFunc: func(t *testing.T) (string, string) {
				return "", t.TempDir()
			},
			args: map[string]interface{}{
				"path":       123,
				"output_dir": "",
			},
			wantError: true,
		},
		{
			name: "invalid output_dir type",
			setupFunc: func(t *testing.T) (string, string) {
				return t.TempDir(), ""
			},
			args: map[string]interface{}{
				"path":       "",
				"output_dir": true,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, outputDir := tt.setupFunc(t)
			if path != "" {
				tt.args["path"] = path
			}
			if outputDir != "" {
				tt.args["output_dir"] = outputDir
			}

			req := ipc.Request{
				Command: "ingest",
				Args:    tt.args,
			}

			resp := executePlugin(t, &req)

			if tt.wantError {
				if resp.Status == "ok" {
					t.Fatal("expected error, got ok")
				}
				return
			}

			if resp.Status != "ok" {
				t.Fatalf("unexpected error: %s", resp.Error)
			}

			result, ok := resp.Result.(map[string]interface{})
			if !ok {
				t.Fatal("result is not a map")
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, result, outputDir)
			}
		})
	}
}

// TestIPCMessageHandling tests all IPC message handling
func TestIPCMessageHandling(t *testing.T) {
	tests := []struct {
		name       string
		input      ipc.Request
		setupFunc  func(t *testing.T) map[string]interface{}
		wantStatus string
		wantError  bool
	}{
		{
			name: "detect command",
			input: ipc.Request{
				Command: "detect",
			},
			setupFunc: func(t *testing.T) map[string]interface{} {
				dir := t.TempDir()
				return map[string]interface{}{
					"path": dir,
				}
			},
			wantStatus: "ok",
			wantError:  false,
		},
		{
			name: "enumerate command",
			input: ipc.Request{
				Command: "enumerate",
			},
			setupFunc: func(t *testing.T) map[string]interface{} {
				dir := t.TempDir()
				return map[string]interface{}{
					"path": dir,
				}
			},
			wantStatus: "ok",
			wantError:  false,
		},
		{
			name: "ingest command",
			input: ipc.Request{
				Command: "ingest",
			},
			setupFunc: func(t *testing.T) map[string]interface{} {
				dir := t.TempDir()
				outputDir := t.TempDir()
				return map[string]interface{}{
					"path":       dir,
					"output_dir": outputDir,
				}
			},
			wantStatus: "ok",
			wantError:  false,
		},
		{
			name: "unknown command",
			input: ipc.Request{
				Command: "unknown",
			},
			setupFunc: func(t *testing.T) map[string]interface{} {
				return map[string]interface{}{}
			},
			wantStatus: "error",
			wantError:  true,
		},
		{
			name: "empty command",
			input: ipc.Request{
				Command: "",
			},
			setupFunc: func(t *testing.T) map[string]interface{} {
				return map[string]interface{}{}
			},
			wantStatus: "error",
			wantError:  true,
		},
		{
			name: "invalid command type",
			input: ipc.Request{
				Command: "extractIR",
			},
			setupFunc: func(t *testing.T) map[string]interface{} {
				return map[string]interface{}{}
			},
			wantStatus: "error",
			wantError:  true,
		},
		{
			name: "emitNative command",
			input: ipc.Request{
				Command: "emitNative",
			},
			setupFunc: func(t *testing.T) map[string]interface{} {
				return map[string]interface{}{}
			},
			wantStatus: "error",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.setupFunc(t)
			tt.input.Args = args

			resp := executePlugin(t, &tt.input)

			if resp.Status != tt.wantStatus {
				t.Errorf("status = %v, want %v", resp.Status, tt.wantStatus)
			}

			if tt.wantError && resp.Error == "" {
				t.Error("expected error message, got empty")
			}

			if !tt.wantError && resp.Error != "" {
				t.Errorf("unexpected error: %s", resp.Error)
			}
		})
	}
}

// TestIPCInvalidJSON tests handling of invalid JSON input
func TestIPCInvalidJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "malformed JSON",
			input: "{invalid json}",
		},
		{
			name:  "empty input",
			input: "",
		},
		{
			name:  "partial JSON",
			input: "{\"command\":\"detect\"",
		},
		{
			name:  "wrong type",
			input: "[]",
		},
		{
			name:  "null input",
			input: "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginPath := "./format-dir"
			if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
				buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
				if err := buildCmd.Run(); err != nil {
					t.Fatalf("failed to build plugin: %v", err)
				}
			}

			cmd := exec.Command(pluginPath)
			cmd.Stdin = strings.NewReader(tt.input)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			// Expect non-zero exit
			err := cmd.Run()
			if err == nil && tt.input != "null" {
				t.Error("expected error for invalid JSON, got success")
			}

			// Try to parse response if there is output
			if stdout.Len() > 0 {
				var resp ipc.Response
				if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
					if resp.Status != "error" {
						t.Errorf("status = %v, want error", resp.Status)
					}
				}
			}
		})
	}
}

// TestExtractIRNotImplemented tests that extractIR is not implemented (plugin doesn't support it)
func TestExtractIRNotImplemented(t *testing.T) {
	req := ipc.Request{
		Command: "extractIR",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "error" {
		t.Error("extractIR should not be implemented")
	}
	if !strings.Contains(resp.Error, "unknown command") {
		t.Errorf("expected 'unknown command' error, got: %s", resp.Error)
	}
}

// TestEmitNativeNotImplemented tests that emitNative is not implemented (plugin doesn't support it)
func TestEmitNativeNotImplemented(t *testing.T) {
	req := ipc.Request{
		Command: "emitNative",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "error" {
		t.Error("emitNative should not be implemented")
	}
	if !strings.Contains(resp.Error, "unknown command") {
		t.Errorf("expected 'unknown command' error, got: %s", resp.Error)
	}
}

// TestDetectResultSerialization tests ipc.DetectResult JSON serialization
func TestDetectResultSerialization(t *testing.T) {
	result := ipc.DetectResult{
		Detected: true,
		Format:   "dir",
		Reason:   "test reason",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ipc.DetectResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Detected != result.Detected {
		t.Errorf("Detected = %v, want %v", decoded.Detected, result.Detected)
	}
	if decoded.Format != result.Format {
		t.Errorf("Format = %v, want %v", decoded.Format, result.Format)
	}
	if decoded.Reason != result.Reason {
		t.Errorf("Reason = %v, want %v", decoded.Reason, result.Reason)
	}
}

// TestEnumerateResultSerialization tests ipc.EnumerateResult JSON serialization
func TestEnumerateResultSerialization(t *testing.T) {
	result := ipc.EnumerateResult{
		Entries: []ipc.EnumerateEntry{
			{Path: "test1.txt", SizeBytes: 100, IsDir: false},
			{Path: "subdir", SizeBytes: 0, IsDir: true},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ipc.EnumerateResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(decoded.Entries) != len(result.Entries) {
		t.Errorf("got %d entries, want %d", len(decoded.Entries), len(result.Entries))
	}

	for i, entry := range decoded.Entries {
		if entry.Path != result.Entries[i].Path {
			t.Errorf("entry %d: Path = %v, want %v", i, entry.Path, result.Entries[i].Path)
		}
		if entry.SizeBytes != result.Entries[i].SizeBytes {
			t.Errorf("entry %d: SizeBytes = %v, want %v", i, entry.SizeBytes, result.Entries[i].SizeBytes)
		}
		if entry.IsDir != result.Entries[i].IsDir {
			t.Errorf("entry %d: IsDir = %v, want %v", i, entry.IsDir, result.Entries[i].IsDir)
		}
	}
}

// TestIngestResultSerialization tests ipc.IngestResult JSON serialization
func TestIngestResultSerialization(t *testing.T) {
	result := ipc.IngestResult{
		ArtifactID: "test-artifact",
		BlobSHA256: "abcd1234",
		SizeBytes:  1024,
		Metadata: map[string]string{
			"format":     "dir",
			"file_count": "5",
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ipc.IngestResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ArtifactID != result.ArtifactID {
		t.Errorf("ArtifactID = %v, want %v", decoded.ArtifactID, result.ArtifactID)
	}
	if decoded.BlobSHA256 != result.BlobSHA256 {
		t.Errorf("BlobSHA256 = %v, want %v", decoded.BlobSHA256, result.BlobSHA256)
	}
	if decoded.SizeBytes != result.SizeBytes {
		t.Errorf("SizeBytes = %v, want %v", decoded.SizeBytes, result.SizeBytes)
	}
	if len(decoded.Metadata) != len(result.Metadata) {
		t.Errorf("got %d metadata entries, want %d", len(decoded.Metadata), len(result.Metadata))
	}
}

// TestIPCRequestSerialization tests ipc.Request JSON serialization
func TestIPCRequestSerialization(t *testing.T) {
	req := ipc.Request{
		Command: "detect",
		Args: map[string]interface{}{
			"path": "/test/path",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ipc.Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Command != req.Command {
		t.Errorf("Command = %v, want %v", decoded.Command, req.Command)
	}
}

// TestIPCResponseSerialization tests ipc.Response JSON serialization
func TestIPCResponseSerialization(t *testing.T) {
	resp := ipc.Response{
		Status: "ok",
		Result: map[string]interface{}{
			"detected": true,
			"format":   "dir",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ipc.Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Status != resp.Status {
		t.Errorf("Status = %v, want %v", decoded.Status, resp.Status)
	}
}

// TestSymlinkHandling tests handling of symlinks (if any)
func TestSymlinkHandling(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping symlink test in CI")
	}

	dir := t.TempDir()
	file := filepath.Join(dir, "original.txt")
	os.WriteFile(file, []byte("content"), 0600)

	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(file, link); err != nil {
		t.Skipf("Cannot create symlink: %v", err)
	}

	req := ipc.Request{
		Command: "enumerate",
		Args: map[string]interface{}{
			"path": dir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("enumerate failed: %s", resp.Error)
	}

	result := resp.Result.(map[string]interface{})
	entries := result["entries"].([]interface{})

	// Should enumerate both the original file and the symlink
	if len(entries) < 2 {
		t.Logf("Warning: symlink may not be enumerated")
	}
}

// executePlugin runs the plugin with a request and returns the response.
func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()

	pluginPath := "./format-dir"
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

	if err := cmd.Run(); err != nil {
		if stdout.Len() > 0 {
			var resp ipc.Response
			if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
				return &resp
			}
		}
		t.Logf("plugin execution failed: %v\nstderr: %s", err, stderr.String())
	}

	var resp ipc.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\noutput: %s", err, stdout.String())
	}

	return &resp
}
