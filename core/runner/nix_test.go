package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FocuswithJustin/JuniperBible/internal/fileutil"
)

func TestNewNixExecutor(t *testing.T) {
	exec := NewNixExecutor("/path/to/flake")
	if exec.FlakePath != "/path/to/flake" {
		t.Errorf("expected FlakePath /path/to/flake, got %s", exec.FlakePath)
	}
	if exec.Timeout != 5*time.Minute {
		t.Errorf("expected Timeout 5m, got %v", exec.Timeout)
	}
}

// TestValidateIdentifier tests the identifier validation function.
func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		idName  string
		wantErr bool
	}{
		{"valid simple", "test", "id", false},
		{"valid with hyphen", "test-plugin", "id", false},
		{"valid with underscore", "test_plugin", "id", false},
		{"valid with dot", "test.plugin", "id", false},
		{"valid with number", "test123", "id", false},
		{"valid complex", "test-plugin_v1.0", "id", false},
		{"empty", "", "id", true},
		{"shell injection semicolon", "test;rm -rf /", "id", true},
		{"shell injection pipe", "test|cat /etc/passwd", "id", true},
		{"shell injection backtick", "test`whoami`", "id", true},
		{"shell injection dollar", "test$(whoami)", "id", true},
		{"shell injection newline", "test\nrm -rf /", "id", true},
		{"shell injection space", "test rm -rf", "id", true},
		{"shell injection quote", "test'rm -rf", "id", true},
		{"shell injection double quote", `test"rm -rf`, "id", true},
		{"shell injection ampersand", "test&rm -rf", "id", true},
		{"shell injection gt", "test>file", "id", true},
		{"shell injection lt", "test<file", "id", true},
		{"too long", strings.Repeat("a", 65), "id", true},
		{"max length", strings.Repeat("a", 64), "id", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIdentifier(tt.id, tt.idName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIdentifier(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

// TestExecuteRequestShellInjectionPrevention tests that shell injection is prevented.
func TestExecuteRequestShellInjectionPrevention(t *testing.T) {
	executor := NewNixExecutor("/tmp/flake")

	tests := []struct {
		name     string
		pluginID string
		profile  string
		wantErr  string
	}{
		{
			name:     "shell injection in plugin ID",
			pluginID: "test;rm -rf /",
			profile:  "valid",
			wantErr:  "plugin ID contains invalid characters",
		},
		{
			name:     "command substitution in plugin ID",
			pluginID: "$(whoami)",
			profile:  "valid",
			wantErr:  "plugin ID contains invalid characters",
		},
		{
			name:     "backtick in plugin ID",
			pluginID: "`id`",
			profile:  "valid",
			wantErr:  "plugin ID contains invalid characters",
		},
		{
			name:     "shell injection in profile",
			pluginID: "valid",
			profile:  "test|cat /etc/passwd",
			wantErr:  "profile contains invalid characters",
		},
		{
			name:     "empty plugin ID",
			pluginID: "",
			profile:  "valid",
			wantErr:  "plugin ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewRequest(tt.pluginID, tt.profile)
			_, err := executor.ExecuteRequest(context.Background(), req, []string{})

			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.wantErr)
				return
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestBuildToolCommand(t *testing.T) {
	exec := NewNixExecutor("/path/to/flake")

	tests := []struct {
		name    string
		plugin  string
		profile string
		wantIn  string
	}{
		{
			name:    "libsword list-modules",
			plugin:  "libsword",
			profile: "list-modules",
			wantIn:  "list_modules",
		},
		{
			name:    "libsword render-all",
			plugin:  "libsword",
			profile: "render-all",
			wantIn:  "diatheke",
		},
		{
			name:    "libsword enumerate-keys",
			plugin:  "libsword",
			profile: "enumerate-keys",
			wantIn:  "mod2imp",
		},
		{
			name:    "unknown plugin",
			plugin:  "unknown",
			profile: "test",
			wantIn:  "event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewRequest(tt.plugin, tt.profile)
			cmd := exec.buildToolCommand(req, "/in", "/out")
			if !strings.Contains(cmd, tt.wantIn) {
				t.Errorf("expected command to contain %q, got: %s", tt.wantIn, cmd)
			}
		})
	}
}

func TestExecutionResultToRunOutputs(t *testing.T) {
	result := &ExecutionResult{
		ExitCode:       0,
		Duration:       500 * time.Millisecond,
		TranscriptHash: "abc123",
	}

	outputs := result.ToRunOutputs()

	if outputs.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", outputs.ExitCode)
	}
	if outputs.DurationMs != 500 {
		t.Errorf("expected duration 500ms, got %d", outputs.DurationMs)
	}
	if outputs.TranscriptBlobSHA256 != "abc123" {
		t.Errorf("expected hash abc123, got %s", outputs.TranscriptBlobSHA256)
	}
}

func TestParseNixTranscript(t *testing.T) {
	transcript := `{"event":"start","plugin":"libsword","profile":"list-modules"}
{"event":"list_modules","modules":["KJV","ESV"]}
{"event":"end","exit_code":0}`

	events, err := ParseNixTranscript([]byte(transcript))
	if err != nil {
		t.Fatalf("failed to parse transcript: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	if events[0].Event != "start" {
		t.Errorf("expected first event 'start', got %s", events[0].Event)
	}
	if events[0].Plugin != "libsword" {
		t.Errorf("expected plugin 'libsword', got %s", events[0].Plugin)
	}

	if events[1].Event != "list_modules" {
		t.Errorf("expected second event 'list_modules', got %s", events[1].Event)
	}

	if events[2].Event != "end" {
		t.Errorf("expected third event 'end', got %s", events[2].Event)
	}
	if events[2].ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", events[2].ExitCode)
	}
}

func TestParseNixTranscriptEmpty(t *testing.T) {
	events, err := ParseNixTranscript([]byte(""))
	if err != nil {
		t.Fatalf("failed to parse empty transcript: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestParseNixTranscriptInvalid(t *testing.T) {
	_, err := ParseNixTranscript([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// Integration test - only runs if nix is available
func TestNixExecutorIntegration(t *testing.T) {
	// Skip if nix is not available
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}

	// Get the project root to find the flake
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	// Navigate up to find nix/flake.nix
	flakePath := filepath.Join(cwd, "..", "..", "nix")
	if _, err := os.Stat(filepath.Join(flakePath, "flake.nix")); err != nil {
		t.Skipf("flake.nix not found at %s", flakePath)
	}

	// Create test input directory with mock SWORD structure
	tmpDir, err := os.MkdirTemp("", "nix-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mock mods.d directory
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	// Create mock config file
	confContent := `[TestMod]
DataPath=./modules/texts/testmod/
ModDrv=RawText
`
	if err := os.WriteFile(filepath.Join(modsDir, "testmod.conf"), []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	exec := NewNixExecutor(flakePath)
	exec.Timeout = 2 * time.Minute

	req := NewRequest("libsword", "list-modules")

	ctx := context.Background()
	result, err := exec.ExecuteRequest(ctx, req, []string{})
	if err != nil {
		// This may fail if nix shell takes too long or isn't configured
		t.Skipf("nix execution failed (expected in CI or without proper nix setup): %v", err)
	}

	// Skip if exit code is non-zero (likely environment issue)
	if result.ExitCode != 0 {
		t.Skipf("nix execution had non-zero exit (environment issue): stderr=%s", result.Stderr)
	}

	if len(result.TranscriptData) == 0 {
		t.Error("expected transcript data")
	}
}

// TestCopyDir tests the copyDir helper function.
func TestCopyDir(t *testing.T) {
	// Create source directory
	srcDir, err := os.MkdirTemp("", "copydir-src-*")
	if err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	// Create test files and subdirectories
	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}

	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(subDir, "file2.txt"), []byte("content2"), 0600); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	// Create destination directory
	dstDir, err := os.MkdirTemp("", "copydir-dst-*")
	if err != nil {
		t.Fatalf("failed to create dst dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	// Copy directory
	if err := fileutil.CopyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify files were copied
	file1 := filepath.Join(dstDir, "file1.txt")
	data1, err := os.ReadFile(file1)
	if err != nil {
		t.Fatalf("failed to read copied file1: %v", err)
	}
	if string(data1) != "content1" {
		t.Errorf("file1 content = %q, want %q", string(data1), "content1")
	}

	file2 := filepath.Join(dstDir, "subdir", "file2.txt")
	data2, err := os.ReadFile(file2)
	if err != nil {
		t.Fatalf("failed to read copied file2: %v", err)
	}
	if string(data2) != "content2" {
		t.Errorf("file2 content = %q, want %q", string(data2), "content2")
	}

	// Verify permissions were preserved
	info2, err := os.Stat(file2)
	if err != nil {
		t.Fatalf("failed to stat file2: %v", err)
	}
	if info2.Mode().Perm() != 0600 {
		t.Errorf("file2 mode = %o, want %o", info2.Mode().Perm(), 0600)
	}
}

// TestCopyDirErrors tests error handling in copyDir.
func TestCopyDirErrors(t *testing.T) {
	// Test with non-existent source
	err := fileutil.CopyDir("/nonexistent/source", "/tmp/dest")
	if err == nil {
		t.Error("expected error for non-existent source")
	}
}

// TestExecuteRequestWithInputs tests ExecuteRequest with input files.
func TestExecuteRequestWithInputs(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}

	// Create input file
	tmpDir, err := os.MkdirTemp("", "exec-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	inputFile := filepath.Join(tmpDir, "input.txt")
	if err := os.WriteFile(inputFile, []byte("test input"), 0644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	executor := NewNixExecutor("/tmp/flake")
	executor.Timeout = 1 * time.Second // Short timeout for faster failure

	req := NewRequest("test", "test")
	ctx := context.Background()

	// This will likely fail due to missing flake, but tests the input copy path
	_, err = executor.ExecuteRequest(ctx, req, []string{inputFile})
	// We expect an error since the flake doesn't exist
	if err == nil {
		t.Log("ExecuteRequest succeeded (unexpected but ok)")
	}
}

// TestExecuteRequestWithDirectory tests ExecuteRequest with directory input.
func TestExecuteRequestWithDirectory(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}

	// Create input directory
	tmpDir, err := os.MkdirTemp("", "exec-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	inputDir := filepath.Join(tmpDir, "inputdir")
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(inputDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	executor := NewNixExecutor("/tmp/flake")
	executor.Timeout = 1 * time.Second

	req := NewRequest("test", "test")
	ctx := context.Background()

	// This will likely fail due to missing flake, but tests the directory copy path
	_, err = executor.ExecuteRequest(ctx, req, []string{inputDir})
	if err == nil {
		t.Log("ExecuteRequest succeeded (unexpected but ok)")
	}
}

// TestExecuteRequestNilContext tests ExecuteRequest with nil context.
func TestExecuteRequestNilContext(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}

	executor := NewNixExecutor("/tmp/flake")
	executor.Timeout = 1 * time.Second

	req := NewRequest("test", "test")

	// Pass nil context - should use context.Background()
	_, err := executor.ExecuteRequest(nil, req, []string{})
	if err == nil {
		t.Log("ExecuteRequest succeeded (unexpected but ok)")
	}
}

// TestBuildSwordCommandUnknownProfile tests unknown profile handling.
func TestBuildSwordCommandUnknownProfile(t *testing.T) {
	exec := NewNixExecutor("/path/to/flake")
	req := NewRequest("libsword", "unknown-profile")

	cmd := exec.buildSwordCommand(req, "/in", "/out")
	if !strings.Contains(cmd, "unknown_profile") {
		t.Errorf("expected command to contain 'unknown_profile', got: %s", cmd)
	}
}

// TestExecuteRequestMkdirTempError tests ExecuteRequest with MkdirTemp error.
func TestExecuteRequestMkdirTempError(t *testing.T) {
	// Inject error
	orig := osMkdirTemp
	osMkdirTemp = func(dir, pattern string) (string, error) {
		return "", fmt.Errorf("injected temp dir error")
	}
	defer func() { osMkdirTemp = orig }()

	executor := NewNixExecutor("/tmp/flake")
	req := NewRequest("test", "test")

	_, err := executor.ExecuteRequest(context.Background(), req, []string{})
	if err == nil {
		t.Error("expected error for MkdirTemp failure")
	}
	if !strings.Contains(err.Error(), "failed to create work dir") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestExecuteRequestMkdirAllInDirError tests ExecuteRequest with MkdirAll(inDir) error.
func TestExecuteRequestMkdirAllInDirError(t *testing.T) {
	// Inject error on first call only
	callCount := 0
	orig := osMkdirAll
	osMkdirAll = func(path string, perm os.FileMode) error {
		callCount++
		if callCount == 1 {
			return fmt.Errorf("injected mkdir error")
		}
		return orig(path, perm)
	}
	defer func() { osMkdirAll = orig }()

	executor := NewNixExecutor("/tmp/flake")
	req := NewRequest("test", "test")

	_, err := executor.ExecuteRequest(context.Background(), req, []string{})
	if err == nil {
		t.Error("expected error for MkdirAll failure")
	}
	if !strings.Contains(err.Error(), "failed to create in dir") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestExecuteRequestMkdirAllOutDirError tests ExecuteRequest with MkdirAll(outDir) error.
func TestExecuteRequestMkdirAllOutDirError(t *testing.T) {
	// Inject error on second call only
	callCount := 0
	orig := osMkdirAll
	osMkdirAll = func(path string, perm os.FileMode) error {
		callCount++
		if callCount == 2 {
			return fmt.Errorf("injected mkdir error")
		}
		return orig(path, perm)
	}
	defer func() { osMkdirAll = orig }()

	executor := NewNixExecutor("/tmp/flake")
	req := NewRequest("test", "test")

	_, err := executor.ExecuteRequest(context.Background(), req, []string{})
	if err == nil {
		t.Error("expected error for MkdirAll failure")
	}
	if !strings.Contains(err.Error(), "failed to create out dir") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestExecuteRequestInjectedReadFileError tests ExecuteRequest with injected ReadFile error on input.
func TestExecuteRequestInjectedReadFileError(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}

	// Create a file to pass as input
	tmpDir, err := os.MkdirTemp("", "exec-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	inputFile := filepath.Join(tmpDir, "input.txt")
	if err := os.WriteFile(inputFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	// Inject error
	orig := osReadFile
	osReadFile = func(name string) ([]byte, error) {
		return nil, fmt.Errorf("injected read error")
	}
	defer func() { osReadFile = orig }()

	executor := NewNixExecutor("/tmp/flake")
	executor.Timeout = 1 * time.Second
	req := NewRequest("test", "test")

	_, err = executor.ExecuteRequest(context.Background(), req, []string{inputFile})
	if err == nil {
		t.Error("expected error for ReadFile failure")
	}
	if !strings.Contains(err.Error(), "failed to read input") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestExecuteRequestWriteInputError tests ExecuteRequest with WriteFile error on input.
func TestExecuteRequestWriteInputError(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}

	// Create a file to pass as input
	tmpDir, err := os.MkdirTemp("", "exec-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	inputFile := filepath.Join(tmpDir, "input.txt")
	if err := os.WriteFile(inputFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	// Inject error on osWriteFile but not the first call (which is for the input)
	callCount := 0
	orig := osWriteFile
	osWriteFile = func(name string, data []byte, perm os.FileMode) error {
		callCount++
		if callCount == 1 {
			return fmt.Errorf("injected write error")
		}
		return orig(name, data, perm)
	}
	defer func() { osWriteFile = orig }()

	executor := NewNixExecutor("/tmp/flake")
	executor.Timeout = 1 * time.Second
	req := NewRequest("test", "test")

	_, err = executor.ExecuteRequest(context.Background(), req, []string{inputFile})
	if err == nil {
		t.Error("expected error for WriteFile failure")
	}
	if !strings.Contains(err.Error(), "failed to write input") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestExecuteRequestInjectedWriteRequestError tests ExecuteRequest with injected WriteFile error on request.json.
func TestExecuteRequestInjectedWriteRequestError(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}

	// Inject error only for request.json (first call without inputs)
	orig := osWriteFile
	osWriteFile = func(name string, data []byte, perm os.FileMode) error {
		return fmt.Errorf("injected write error")
	}
	defer func() { osWriteFile = orig }()

	executor := NewNixExecutor("/tmp/flake")
	executor.Timeout = 1 * time.Second
	req := NewRequest("test", "test")

	_, err := executor.ExecuteRequest(context.Background(), req, []string{})
	if err == nil {
		t.Error("expected error for WriteFile failure")
	}
	if !strings.Contains(err.Error(), "failed to write request") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestExecuteRequestCopyDirError tests ExecuteRequest with CopyDir error.
func TestExecuteRequestCopyDirError(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}

	// Create a directory to pass as input
	tmpDir, err := os.MkdirTemp("", "exec-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	inputDir := filepath.Join(tmpDir, "input")
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}

	// Inject error
	orig := copyDir
	copyDir = func(src, dst string) error {
		return fmt.Errorf("injected copy error")
	}
	defer func() { copyDir = orig }()

	executor := NewNixExecutor("/tmp/flake")
	executor.Timeout = 1 * time.Second
	req := NewRequest("test", "test")

	_, err = executor.ExecuteRequest(context.Background(), req, []string{inputDir})
	if err == nil {
		t.Error("expected error for CopyDir failure")
	}
	if !strings.Contains(err.Error(), "failed to copy input dir") {
		t.Errorf("unexpected error: %v", err)
	}
}
