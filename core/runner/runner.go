// Package runner provides the execution harness for running tool plugins
// in a deterministic environment.
package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Request represents a tool run request sent to the VM.
type Request struct {
	PluginID      string                 `json:"plugin_id"`
	PluginVersion string                 `json:"plugin_version,omitempty"`
	Profile       string                 `json:"profile"`
	Inputs        []string               `json:"inputs"`
	Args          map[string]interface{} `json:"args,omitempty"`
	Env           EnvConfig              `json:"env"`
}

// EnvConfig contains environment configuration for deterministic execution.
type EnvConfig struct {
	TZ    string `json:"TZ"`
	LCALL string `json:"LC_ALL"`
	LANG  string `json:"LANG"`
}

// NewRequest creates a new tool run request with default environment settings.
func NewRequest(pluginID, profile string) *Request {
	return &Request{
		PluginID: pluginID,
		Profile:  profile,
		Inputs:   []string{},
		Env: EnvConfig{
			TZ:    "UTC",
			LCALL: "C.UTF-8",
			LANG:  "C.UTF-8",
		},
	}
}

// ToJSON serializes the request to JSON.
func (r *Request) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// EngineSpec describes the execution environment specification.
type EngineSpec struct {
	EngineID string    `json:"engine_id"`
	Type     string    `json:"type"`
	Nix      NixConfig `json:"nix"`
	Env      EnvConfig `json:"env"`
}

// NixConfig contains Nix-specific configuration.
type NixConfig struct {
	FlakeLockSHA256 string   `json:"flake_lock_sha256"`
	System          string   `json:"system"`
	Derivations     []string `json:"derivations"`
}

// NewEngineSpec creates a new engine specification with default settings.
func NewEngineSpec(engineID string) *EngineSpec {
	return &EngineSpec{
		EngineID: engineID,
		Type:     "nixos-vm",
		Nix: NixConfig{
			System:      "x86_64-linux",
			Derivations: []string{},
		},
		Env: EnvConfig{
			TZ:    "UTC",
			LCALL: "C.UTF-8",
			LANG:  "C.UTF-8",
		},
	}
}

// PrepareWorkDir prepares the work directory structure for VM execution.
// Creates:
//   - <workDir>/in/request.json
//   - <workDir>/in/runner.sh
//   - <workDir>/out/
func PrepareWorkDir(workDir string, req *Request) error {
	inDir := filepath.Join(workDir, "in")
	outDir := filepath.Join(workDir, "out")

	// Create directories
	if err := os.MkdirAll(inDir, 0755); err != nil {
		return fmt.Errorf("failed to create in directory: %w", err)
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create out directory: %w", err)
	}

	// Write request.json
	reqData, err := req.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize request: %w", err)
	}
	reqPath := filepath.Join(inDir, "request.json")
	if err := os.WriteFile(reqPath, reqData, 0600); err != nil {
		return fmt.Errorf("failed to write request.json: %w", err)
	}

	// Write runner.sh
	runnerScript := generateRunnerScript(req)
	runnerPath := filepath.Join(inDir, "runner.sh")
	if err := os.WriteFile(runnerPath, []byte(runnerScript), 0755); err != nil {
		return fmt.Errorf("failed to write runner.sh: %w", err)
	}

	return nil
}

// generateRunnerScript generates the shell script that runs inside the VM.
func generateRunnerScript(req *Request) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

# Deterministic environment settings
export TZ=UTC
export LC_ALL=C.UTF-8
export LANG=C.UTF-8
umask 022

# Paths
IN=/work/in
OUT=/work/out

# Ensure output directory exists
mkdir -p "$OUT"

# Verify request exists
REQ="$IN/request.json"
if [ ! -f "$REQ" ]; then
    echo "ERROR: missing request.json" >&2
    exit 2
fi

# Check for tool executable
TOOL="$IN/tool"
if [ -x "$TOOL" ]; then
    # Execute the tool plugin
    "$TOOL" run --request "$REQ" --out "$OUT"
else
    # No tool provided - just validate environment
    echo "No tool executable found at $TOOL"
    echo "Environment validated:"
    echo "  TZ=$TZ"
    echo "  LC_ALL=$LC_ALL"
    echo "  LANG=$LANG"
    echo "  Plugin: %s"
    echo "  Profile: %s"
fi
`, req.PluginID, req.Profile)
}

// Result represents the result of a tool run.
type Result struct {
	TranscriptPath string
	StdoutPath     string
	StderrPath     string
	OutputDir      string
	ExitCode       int
}

// CollectResults collects the results from a completed VM run.
func CollectResults(workDir string) (*Result, error) {
	outDir := filepath.Join(workDir, "out")

	result := &Result{
		OutputDir: outDir,
	}

	// Check for transcript
	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	if _, err := os.Stat(transcriptPath); err == nil {
		result.TranscriptPath = transcriptPath
	}

	// Check for stdout
	stdoutPath := filepath.Join(outDir, "stdout")
	if _, err := os.Stat(stdoutPath); err == nil {
		result.StdoutPath = stdoutPath
	}

	// Check for stderr
	stderrPath := filepath.Join(outDir, "stderr")
	if _, err := os.Stat(stderrPath); err == nil {
		result.StderrPath = stderrPath
	}

	return result, nil
}
