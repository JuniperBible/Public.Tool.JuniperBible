// Package runner provides execution harnesses for running tool plugins.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/FocuswithJustin/JuniperBible/core/cas"
	"github.com/FocuswithJustin/JuniperBible/internal/fileutil"
)

// validIdentifierRegex validates plugin and profile identifiers.
// Only allows alphanumeric, hyphen, underscore, and dot characters.
var validIdentifierRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// validateIdentifier checks if a string is a safe identifier for shell use.
// Returns an error if the identifier contains potentially dangerous characters.
func validateIdentifier(id, name string) error {
	if id == "" {
		return fmt.Errorf("%s cannot be empty", name)
	}
	if len(id) > 64 {
		return fmt.Errorf("%s too long (max 64 characters)", name)
	}
	if !validIdentifierRegex.MatchString(id) {
		return fmt.Errorf("%s contains invalid characters (only alphanumeric, hyphen, underscore, dot allowed)", name)
	}
	return nil
}

// Injectable functions for testing.
var (
	osMkdirTemp = os.MkdirTemp
	osMkdirAll  = os.MkdirAll
	osWriteFile = os.WriteFile
	osReadFile  = os.ReadFile
	copyDir     = fileutil.CopyDir
)

// NixExecutor runs tools in a Nix-based deterministic environment.
type NixExecutor struct {
	FlakePath string // Path to the flake directory
	Timeout   time.Duration
}

// NewNixExecutor creates a new Nix-based executor.
func NewNixExecutor(flakePath string) *NixExecutor {
	return &NixExecutor{
		FlakePath: flakePath,
		Timeout:   5 * time.Minute,
	}
}

// ExecuteRequest runs a tool request and returns the result with transcript.
func (e *NixExecutor) ExecuteRequest(ctx context.Context, req *Request, inputPaths []string) (*ExecutionResult, error) {
	// SECURITY: Validate plugin ID and profile to prevent shell injection
	if err := validateIdentifier(req.PluginID, "plugin ID"); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.Profile != "" {
		if err := validateIdentifier(req.Profile, "profile"); err != nil {
			return nil, fmt.Errorf("invalid request: %w", err)
		}
	}

	// Create work directory
	workDir, err := osMkdirTemp("", "capsule-run-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	inDir := filepath.Join(workDir, "in")
	outDir := filepath.Join(workDir, "out")

	if err := osMkdirAll(inDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create in dir: %w", err)
	}
	if err := osMkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create out dir: %w", err)
	}

	// Copy input files/directories
	for i, path := range inputPaths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("failed to stat input %d: %w", i, err)
		}

		if info.IsDir() {
			// Copy directory recursively
			if err := copyDir(path, inDir); err != nil {
				return nil, fmt.Errorf("failed to copy input dir %d: %w", i, err)
			}
		} else {
			// Copy single file
			data, err := osReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read input %d: %w", i, err)
			}
			dest := filepath.Join(inDir, filepath.Base(path))
			if err := osWriteFile(dest, data, 0644); err != nil {
				return nil, fmt.Errorf("failed to write input %d: %w", i, err)
			}
		}
	}

	// Write request
	reqData, err := req.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}
	if err := osWriteFile(filepath.Join(inDir, "request.json"), reqData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Build the command based on plugin type
	var cmd *exec.Cmd
	if ctx == nil {
		ctx = context.Background()
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	// Use nix develop or nix shell to get deterministic environment
	nixArgs := []string{
		"shell",
		e.FlakePath + "#engine-tools",
		"--command",
	}

	// Determine tool command based on plugin
	toolCmd := e.buildToolCommand(req, inDir, outDir)
	nixArgs = append(nixArgs, "sh", "-c", toolCmd)

	cmd = exec.CommandContext(ctxWithTimeout, "nix", nixArgs...)
	cmd.Dir = workDir

	// Set deterministic environment
	cmd.Env = []string{
		"TZ=UTC",
		"LC_ALL=C.UTF-8",
		"LANG=C.UTF-8",
		"HOME=" + workDir,
		"PATH=/usr/bin:/bin",
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	runErr := cmd.Run()
	duration := time.Since(startTime)

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to run command: %w", runErr)
		}
	}

	// Build result
	result := &ExecutionResult{
		ExitCode:  exitCode,
		Duration:  duration,
		Stdout:    stdout.Bytes(),
		Stderr:    stderr.Bytes(),
		OutputDir: outDir,
	}

	// Read transcript if present
	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	if data, err := os.ReadFile(transcriptPath); err == nil {
		result.TranscriptData = data
		result.TranscriptHash = cas.Hash(data)
	}

	// Collect output blobs
	result.OutputBlobs = make(map[string][]byte)
	entries, _ := os.ReadDir(outDir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "transcript.jsonl" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(outDir, name))
		if err == nil {
			result.OutputBlobs[name] = data
		}
	}

	return result, nil
}

// buildToolCommand builds the shell command for a given plugin.
func (e *NixExecutor) buildToolCommand(req *Request, inDir, outDir string) string {
	// Read request to get profile and args
	switch req.PluginID {
	case "libsword":
		return e.buildSwordCommand(req, inDir, outDir)
	default:
		// Generic plugin execution
		return fmt.Sprintf(`
			cd %q
			echo '{"event":"start","plugin":"%s","profile":"%s"}' > %q/transcript.jsonl
			echo '{"event":"end","exit_code":0}' >> %q/transcript.jsonl
		`, inDir, req.PluginID, req.Profile, outDir, outDir)
	}
}

// buildSwordCommand builds the command for libsword operations.
func (e *NixExecutor) buildSwordCommand(req *Request, inDir, outDir string) string {
	transcriptPath := filepath.Join(outDir, "transcript.jsonl")

	// Base command that sets up SWORD environment
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`
set -eu
export SWORD_PATH=%q

# Initialize transcript
echo '{"event":"start","plugin":"libsword","profile":"%s","timestamp":"'$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)'"}' > %q
`, inDir, req.Profile, transcriptPath))

	switch req.Profile {
	case "list-modules":
		sb.WriteString(fmt.Sprintf(`
# List available modules (extract module name from [ModuleName] header in conf)
modules=""
for conf in %q/mods.d/*.conf; do
    if [ -f "$conf" ]; then
        modname=$(grep -m1 '^\[' "$conf" | tr -d '[]' || basename "$conf" .conf)
        if [ -z "$modules" ]; then
            modules="\"$modname\""
        else
            modules="$modules,\"$modname\""
        fi
    fi
done
echo '{"event":"list_modules","modules":['"$modules"']}' >> %q
`, inDir, transcriptPath))

	case "render-all":
		sb.WriteString(fmt.Sprintf(`
# Render sample verses using diatheke
for conf in %q/mods.d/*.conf; do
    if [ -f "$conf" ]; then
        modname=$(grep -m1 '^\[' "$conf" | tr -d '[]' || basename "$conf" .conf)
        echo '{"event":"module_start","module":"'"$modname"'"}' >> %q

        # Get sample verses
        verse=$(diatheke -b "$modname" -k "Gen 1:1" 2>/dev/null | head -1 || echo "")
        verse_escaped=$(printf '%%s' "$verse" | sed 's/\\/\\\\/g; s/"/\\"/g' | tr -d '\n')
        echo '{"event":"verse","ref":"Gen 1:1","text":"'"$verse_escaped"'"}' >> %q

        echo '{"event":"module_end","module":"'"$modname"'"}' >> %q
    fi
done
`, inDir, transcriptPath, transcriptPath, transcriptPath))

	case "enumerate-keys":
		sb.WriteString(fmt.Sprintf(`
# Enumerate all keys in modules
for conf in %q/mods.d/*.conf; do
    if [ -f "$conf" ]; then
        modname=$(grep -m1 '^\[' "$conf" | tr -d '[]' || basename "$conf" .conf)
        echo '{"event":"enumerate_start","module":"'"$modname"'"}' >> %q

        # Use mod2imp to get sample keys
        mod2imp "$modname" 2>/dev/null | head -100 > %q/"$modname".keys.txt || true
        count=$(wc -l < %q/"$modname".keys.txt 2>/dev/null || echo "0")
        echo '{"event":"enumerate_done","module":"'"$modname"'","key_count":'"$count"'}' >> %q
    fi
done
`, inDir, transcriptPath, outDir, outDir, transcriptPath))

	default:
		sb.WriteString(fmt.Sprintf(`
echo '{"event":"unknown_profile","profile":"%s"}' >> %q
`, req.Profile, transcriptPath))
	}

	// Finalize transcript
	sb.WriteString(fmt.Sprintf(`
echo '{"event":"end","exit_code":0,"timestamp":"'$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)'"}' >> %q
`, transcriptPath))

	return sb.String()
}

// ExecutionResult contains the results of a tool execution.
type ExecutionResult struct {
	ExitCode       int
	Duration       time.Duration
	Stdout         []byte
	Stderr         []byte
	TranscriptData []byte
	TranscriptHash string
	OutputDir      string
	OutputBlobs    map[string][]byte
}

// ToRunOutputs converts the result to manifest run outputs.
func (r *ExecutionResult) ToRunOutputs() *RunOutputs {
	return &RunOutputs{
		TranscriptBlobSHA256: r.TranscriptHash,
		ExitCode:             r.ExitCode,
		DurationMs:           int64(r.Duration.Milliseconds()),
	}
}

// RunOutputs matches the manifest structure for run outputs.
type RunOutputs struct {
	TranscriptBlobSHA256 string `json:"transcript_blob_sha256"`
	ExitCode             int    `json:"exit_code"`
	DurationMs           int64  `json:"duration_ms"`
}

// NixTranscriptEvent represents a single event in a Nix executor transcript.
// This is a simpler format used during shell script execution.
type NixTranscriptEvent struct {
	Event     string                 `json:"event"`
	Timestamp string                 `json:"timestamp,omitempty"`
	Plugin    string                 `json:"plugin,omitempty"`
	Profile   string                 `json:"profile,omitempty"`
	Module    string                 `json:"module,omitempty"`
	Ref       string                 `json:"ref,omitempty"`
	Text      string                 `json:"text,omitempty"`
	Error     string                 `json:"error,omitempty"`
	ExitCode  int                    `json:"exit_code,omitempty"`
	Modules   []string               `json:"modules,omitempty"`
	KeyCount  int                    `json:"key_count,omitempty"`
	Extra     map[string]interface{} `json:"-"`
}

// ParseNixTranscript parses a JSONL transcript from Nix executor into events.
func ParseNixTranscript(data []byte) ([]NixTranscriptEvent, error) {
	var events []NixTranscriptEvent
	lines := bytes.Split(data, []byte("\n"))

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var event NixTranscriptEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return nil, fmt.Errorf("failed to parse transcript line: %w", err)
		}
		events = append(events, event)
	}

	return events, nil
}
