// Package tools provides shared logic for tool management operations.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/capsule"
	"github.com/JuniperBible/Public.Tool.JuniperBible/core/runner"
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

func resolveInputPath(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("failed to resolve input path: %w", err)
	}
	return abs, nil
}

func resolveOutputDir(raw string) (string, bool, error) {
	if raw == "" {
		dir, err := os.MkdirTemp("", "capsule-run-*")
		if err != nil {
			return "", false, fmt.Errorf("failed to create temp output directory: %w", err)
		}
		return dir, true, nil
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", false, fmt.Errorf("failed to resolve output directory: %w", err)
	}
	if err := os.MkdirAll(abs, 0700); err != nil {
		return "", false, fmt.Errorf("failed to create output directory: %w", err)
	}
	return abs, false, nil
}

func writeTranscript(outDir string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	path := filepath.Join(outDir, "transcript.jsonl")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write transcript: %w", err)
	}
	return path, nil
}

func Run(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	inputPath, err := resolveInputPath(cfg.InputPath)
	if err != nil {
		return nil, err
	}

	outDir, cleanup, err := resolveOutputDir(cfg.OutDir)
	if err != nil {
		return nil, err
	}
	if cleanup {
		defer os.RemoveAll(outDir)
	}

	req := runner.NewRequest(cfg.ToolID, cfg.Profile)
	var inputPaths []string
	if inputPath != "" {
		req.Inputs = []string{inputPath}
		inputPaths = []string{inputPath}
	}

	result, err := runner.NewNixExecutor(cfg.FlakePath).ExecuteRequest(ctx, req, inputPaths)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	transcriptPath, err := writeTranscript(outDir, result.TranscriptData)
	if err != nil {
		return nil, err
	}

	return &RunResult{
		ExitCode:       result.ExitCode,
		Duration:       result.Duration.String(),
		TranscriptHash: result.TranscriptHash,
		TranscriptPath: transcriptPath,
		OutputDir:      outDir,
	}, nil
}

func unpackAndExportArtifact(capsulePath, artifactID, tempDir string) (*capsule.Capsule, string, error) {
	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		return nil, "", fmt.Errorf("failed to unpack capsule: %w", err)
	}

	artifact, ok := cap.Manifest.Artifacts[artifactID]
	if !ok {
		return nil, "", fmt.Errorf("artifact not found: %s", artifactID)
	}

	inputPath := filepath.Join(tempDir, "input", artifact.OriginalName)
	if err := os.MkdirAll(filepath.Dir(inputPath), 0700); err != nil {
		return nil, "", fmt.Errorf("failed to create input dir: %w", err)
	}
	if err := cap.Export(artifactID, capsule.ExportModeIdentity, inputPath); err != nil {
		return nil, "", fmt.Errorf("failed to export artifact: %w", err)
	}

	return cap, inputPath, nil
}

func executeToolRequest(ctx context.Context, cfg ExecuteConfig, inputPath string) (*runner.ExecutionResult, error) {
	req := runner.NewRequest(cfg.ToolID, cfg.Profile)
	req.Inputs = []string{inputPath}

	result, err := runner.NewNixExecutor(cfg.FlakePath).ExecuteRequest(ctx, req, []string{inputPath})
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}
	if len(result.TranscriptData) == 0 {
		return nil, fmt.Errorf("no transcript generated")
	}
	return result, nil
}

func buildRunRecord(cfg ExecuteConfig, runIndex int) (string, *capsule.Run) {
	runID := fmt.Sprintf("run-%s-%s-%d", cfg.ToolID, cfg.Profile, runIndex)
	run := &capsule.Run{
		ID: runID,
		Plugin: &capsule.PluginInfo{
			PluginID: cfg.ToolID,
			Kind:     "tool",
		},
		Inputs:  []capsule.RunInput{{ArtifactID: cfg.ArtifactID}},
		Command: &capsule.Command{Profile: cfg.Profile},
		Status:  "completed",
	}
	return runID, run
}

func persistRunResult(cap *capsule.Capsule, run *capsule.Run, transcriptData []byte, capsulePath string) error {
	if err := cap.AddRun(run, transcriptData); err != nil {
		return fmt.Errorf("failed to add run: %w", err)
	}
	if err := cap.SaveManifest(); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}
	if err := cap.Pack(capsulePath); err != nil {
		return fmt.Errorf("failed to repack capsule: %w", err)
	}
	return nil
}

func Execute(ctx context.Context, cfg ExecuteConfig) (*ExecuteResult, error) {
	tempDir, err := os.MkdirTemp("", "capsule-tool-run-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	cap, inputPath, err := unpackAndExportArtifact(cfg.CapsulePath, cfg.ArtifactID, tempDir)
	if err != nil {
		return nil, err
	}

	result, err := executeToolRequest(ctx, cfg, inputPath)
	if err != nil {
		return nil, err
	}

	runID, run := buildRunRecord(cfg, len(cap.Manifest.Runs)+1)
	if err := persistRunResult(cap, run, result.TranscriptData, cfg.CapsulePath); err != nil {
		return nil, err
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
