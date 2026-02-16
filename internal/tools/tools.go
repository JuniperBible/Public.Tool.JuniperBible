// Package tools provides shared logic for tool management operations.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/core/capsule"
	"github.com/FocuswithJustin/JuniperBible/core/runner"
)

// ListConfig configures the List operation.
type ListConfig struct {
	ContribDir string
}

// ArchiveConfig configures the Archive operation.
type ArchiveConfig struct {
	ToolID   string
	Version  string
	Platform string
	Binaries map[string]string
	Output   string
}

// RunConfig configures the Run operation for standalone tool execution.
type RunConfig struct {
	ToolID    string
	Profile   string
	InputPath string
	OutDir    string
	FlakePath string
}

// ExecuteConfig configures the Execute operation for running tools on capsule artifacts.
type ExecuteConfig struct {
	CapsulePath string
	ArtifactID  string
	ToolID      string
	Profile     string
	FlakePath   string
}

// ListResult contains the results of a List operation.
type ListResult struct {
	Tools []string
}

// ArchiveResult contains the results of an Archive operation.
type ArchiveResult struct {
	OutputPath string
}

// RunResult contains the results of a Run operation.
type RunResult struct {
	ExitCode       int
	Duration       string
	TranscriptHash string
	TranscriptPath string
	OutputDir      string
}

// ExecuteResult contains the results of an Execute operation.
type ExecuteResult struct {
	RunID          string
	ExitCode       int
	Duration       string
	TranscriptHash string
	CapsulePath    string
}

// List lists available tools in the contrib/tool directory.
func List(cfg ListConfig) (*ListResult, error) {
	registry := runner.NewToolRegistry(cfg.ContribDir)
	tools, err := registry.ListTools()
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	return &ListResult{Tools: tools}, nil
}

// Archive creates a tool archive capsule from binaries.
func Archive(cfg ArchiveConfig) (*ArchiveResult, error) {
	// Verify binaries exist
	for name, path := range cfg.Binaries {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil, fmt.Errorf("binary not found: %s at %s", name, path)
		}
	}

	// Use default platform if not specified
	platform := cfg.Platform
	if platform == "" {
		platform = "x86_64-linux"
	}

	// Create the tool archive
	if err := runner.CreateToolArchive(cfg.ToolID, cfg.Version, platform, cfg.Binaries, cfg.Output); err != nil {
		return nil, fmt.Errorf("failed to create tool archive: %w", err)
	}

	return &ArchiveResult{OutputPath: cfg.Output}, nil
}

// Run executes a tool plugin with the Nix executor.
func Run(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	// Resolve input path if provided
	inputPath := cfg.InputPath
	if inputPath != "" {
		var err error
		inputPath, err = filepath.Abs(inputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve input path: %w", err)
		}
	}

	// Handle output directory
	outDir := cfg.OutDir
	cleanupOutDir := false
	if outDir == "" {
		var err error
		outDir, err = os.MkdirTemp("", "capsule-run-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp output directory: %w", err)
		}
		cleanupOutDir = true
	} else {
		var err error
		outDir, err = filepath.Abs(outDir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve output directory: %w", err)
		}
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Cleanup temp directory if we created it
	if cleanupOutDir {
		defer os.RemoveAll(outDir)
	}

	// Create runner request
	req := runner.NewRequest(cfg.ToolID, cfg.Profile)
	if inputPath != "" {
		req.Inputs = []string{inputPath}
	}

	// Execute with Nix
	executor := runner.NewNixExecutor(cfg.FlakePath)

	var inputPaths []string
	if inputPath != "" {
		inputPaths = []string{inputPath}
	}

	result, err := executor.ExecuteRequest(ctx, req, inputPaths)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	// Write transcript to output directory if we have one
	transcriptPath := ""
	if len(result.TranscriptData) > 0 {
		transcriptPath = filepath.Join(outDir, "transcript.jsonl")
		if err := os.WriteFile(transcriptPath, result.TranscriptData, 0600); err != nil {
			return nil, fmt.Errorf("failed to write transcript: %w", err)
		}
	}

	return &RunResult{
		ExitCode:       result.ExitCode,
		Duration:       result.Duration.String(),
		TranscriptHash: result.TranscriptHash,
		TranscriptPath: transcriptPath,
		OutputDir:      outDir,
	}, nil
}

// Execute runs a tool on a capsule artifact and stores the transcript.
func Execute(ctx context.Context, cfg ExecuteConfig) (*ExecuteResult, error) {
	// Create temporary directory for unpacking
	tempDir, err := os.MkdirTemp("", "capsule-tool-run-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Unpack the capsule
	cap, err := capsule.Unpack(cfg.CapsulePath, tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack capsule: %w", err)
	}

	// Check artifact exists
	artifact, ok := cap.Manifest.Artifacts[cfg.ArtifactID]
	if !ok {
		return nil, fmt.Errorf("artifact not found: %s", cfg.ArtifactID)
	}

	// Export artifact to temp directory for tool input
	inputPath := filepath.Join(tempDir, "input", artifact.OriginalName)
	if err := os.MkdirAll(filepath.Dir(inputPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create input dir: %w", err)
	}
	if err := cap.Export(cfg.ArtifactID, capsule.ExportModeIdentity, inputPath); err != nil {
		return nil, fmt.Errorf("failed to export artifact: %w", err)
	}

	// Create runner request
	req := runner.NewRequest(cfg.ToolID, cfg.Profile)
	req.Inputs = []string{inputPath}

	// Execute with Nix
	executor := runner.NewNixExecutor(cfg.FlakePath)
	result, err := executor.ExecuteRequest(ctx, req, []string{inputPath})
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	if len(result.TranscriptData) == 0 {
		return nil, fmt.Errorf("no transcript generated")
	}

	// Create run record
	runID := fmt.Sprintf("run-%s-%s-%d", cfg.ToolID, cfg.Profile, len(cap.Manifest.Runs)+1)
	run := &capsule.Run{
		ID: runID,
		Plugin: &capsule.PluginInfo{
			PluginID: cfg.ToolID,
			Kind:     "tool",
		},
		Inputs: []capsule.RunInput{
			{ArtifactID: cfg.ArtifactID},
		},
		Command: &capsule.Command{
			Profile: cfg.Profile,
		},
		Status: "completed",
	}

	// Add run to capsule
	if err := cap.AddRun(run, result.TranscriptData); err != nil {
		return nil, fmt.Errorf("failed to add run: %w", err)
	}

	// Save manifest
	if err := cap.SaveManifest(); err != nil {
		return nil, fmt.Errorf("failed to save manifest: %w", err)
	}

	// Repack the capsule
	if err := cap.Pack(cfg.CapsulePath); err != nil {
		return nil, fmt.Errorf("failed to repack capsule: %w", err)
	}

	return &ExecuteResult{
		RunID:          runID,
		ExitCode:       result.ExitCode,
		Duration:       result.Duration.String(),
		TranscriptHash: result.TranscriptHash,
		CapsulePath:    cfg.CapsulePath,
	}, nil
}

// ParseTranscriptEvents parses transcript data into events for display.
func ParseTranscriptEvents(transcriptData []byte) ([]interface{}, error) {
	events, err := runner.ParseNixTranscript(transcriptData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transcript: %w", err)
	}

	// Convert to generic interface slice for JSON marshaling
	result := make([]interface{}, len(events))
	for i, e := range events {
		result[i] = e
	}

	return result, nil
}

// FormatTranscriptEvent formats a single transcript event as JSON.
func FormatTranscriptEvent(event interface{}) (string, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event: %w", err)
	}
	return string(data), nil
}
