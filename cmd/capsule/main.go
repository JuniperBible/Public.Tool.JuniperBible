// Command capsule is the CLI tool for Juniper Bible.
// It provides commands for ingesting files, running tools, and verifying behavior.
package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"

	"github.com/FocuswithJustin/JuniperBible/core/capsule"
	"github.com/FocuswithJustin/JuniperBible/core/cas"
	"github.com/FocuswithJustin/JuniperBible/core/docgen"
	"github.com/FocuswithJustin/JuniperBible/core/ir"
	"github.com/FocuswithJustin/JuniperBible/core/plugins"
	"github.com/FocuswithJustin/JuniperBible/core/runner"
	"github.com/FocuswithJustin/JuniperBible/core/selfcheck"
	"github.com/FocuswithJustin/JuniperBible/internal/archive"
	"github.com/FocuswithJustin/JuniperBible/internal/fileutil"
	"github.com/FocuswithJustin/JuniperBible/internal/api"
	"github.com/FocuswithJustin/JuniperBible/internal/juniper"
	"github.com/FocuswithJustin/JuniperBible/internal/validation"
	"github.com/FocuswithJustin/JuniperBible/internal/web"

	// Import embedded plugins registry to register all embedded plugins
	_ "github.com/FocuswithJustin/JuniperBible/internal/embedded"
)

const version = "0.3.0"

// CLI defines the command-line interface for capsule.
var CLI struct {
	// Global flags
	PluginDir string `name:"plugin-dir" short:"p" help:"Plugin directory path" type:"path"`

	// Command groups (noun-first organization)
	Capsule CapsuleGroup `cmd:"" help:"Capsule operations (ingest, export, verify, enumerate)"`
	Format  FormatGroup  `cmd:"" help:"Format detection and IR operations"`
	Plugins PluginsGroup `cmd:"" help:"Plugin management"`
	Tools   ToolsGroup   `cmd:"" help:"Tool execution and archives"`
	Runs    RunsGroup    `cmd:"" help:"Run transcripts and comparisons"`
	Juniper JuniperCmd   `cmd:"" help:"Bible/SWORD module tools"`
	Dev     DevGroup     `cmd:"" help:"Development and maintenance tools"`
	Web     WebCmd       `cmd:"" help:"Start web UI server"`
	API     APICmd       `cmd:"" help:"Start REST API server"`
	Version VersionCmd   `cmd:"" help:"Print version information"`
}

// CapsuleGroup contains capsule lifecycle operations.
type CapsuleGroup struct {
	Ingest    IngestCmd         `cmd:"" help:"Ingest a file into a new capsule"`
	Export    ExportCmd         `cmd:"" help:"Export an artifact from a capsule"`
	Verify    VerifyCmd         `cmd:"" help:"Verify capsule integrity"`
	Selfcheck SelfcheckCmd      `cmd:"" help:"Run self-check verification plan"`
	Enumerate EnumerateCmd      `cmd:"" help:"Enumerate contents of archive"`
	Convert   CapsuleConvertCmd `cmd:"" help:"Convert capsule content to different format"`
}

// FormatGroup contains format detection and IR operations.
type FormatGroup struct {
	Detect  DetectCmd  `cmd:"" help:"Detect file format using plugins"`
	Convert ConvertCmd `cmd:"" help:"Convert file to different format via IR"`
	IR      IRGroup    `cmd:"" help:"Intermediate Representation operations"`
}

// IRGroup contains IR-specific operations.
type IRGroup struct {
	Extract  ExtractIRCmd  `cmd:"" help:"Extract IR from a file"`
	Emit     EmitNativeCmd `cmd:"" help:"Emit native format from IR"`
	Generate GenerateIRCmd `cmd:"" help:"Generate IR for capsule without one"`
	Info     IRInfoCmd     `cmd:"" help:"Display IR structure summary"`
}

// PluginsGroup contains plugin management operations.
type PluginsGroup struct {
	List PluginsListCmd `cmd:"" help:"List available plugins"`
}

// ToolsGroup contains tool execution operations.
type ToolsGroup struct {
	List    ToolListCmd    `cmd:"" help:"List available tools in contrib/tool"`
	Archive ToolArchiveCmd `cmd:"" help:"Create tool archive capsule from binaries"`
	Run     RunCmd         `cmd:"" help:"Run a tool plugin with Nix executor"`
	Execute ToolRunCmd     `cmd:"" help:"Run tool on artifact and store transcript"`
}

// RunsGroup contains run/transcript operations.
type RunsGroup struct {
	List    RunsListCmd `cmd:"" help:"List all runs in a capsule"`
	Compare CompareCmd  `cmd:"" help:"Compare transcripts between two runs"`
	Golden  GoldenGroup `cmd:"" help:"Golden transcript hash operations"`
}

// GoldenGroup contains golden hash operations.
type GoldenGroup struct {
	Save  GoldenSaveCmd  `cmd:"" help:"Save golden transcript hash"`
	Check GoldenCheckCmd `cmd:"" help:"Check transcript against golden hash"`
}

// DevGroup contains development and maintenance tools.
type DevGroup struct {
	Test   TestCmd   `cmd:"" help:"Run tests against golden hashes"`
	Docgen DocgenCmd `cmd:"" help:"Generate documentation"`
}

// IngestCmd ingests a file into a new capsule.
type IngestCmd struct {
	Path string `arg:"" help:"Path to file to ingest" type:"existingfile"`
	Out  string `required:"" help:"Output capsule path" type:"path"`
}

func (c *IngestCmd) Run() error {
	inputPath := c.Path
	outputPath := c.Out

	// Validate paths
	if err := validation.ValidatePath(inputPath); err != nil {
		return fmt.Errorf("invalid input path: %w", err)
	}
	if err := validation.ValidatePath(outputPath); err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}

	// Create temporary directory for capsule
	tempDir, err := os.MkdirTemp("", "capsule-ingest-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Create capsule
	cap, err := capsule.New(tempDir)
	if err != nil {
		return fmt.Errorf("failed to create capsule: %w", err)
	}

	// Ingest the file
	artifact, err := cap.IngestFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to ingest file: %w", err)
	}

	fmt.Printf("Ingested: %s\n", inputPath)
	fmt.Printf("  Artifact ID: %s\n", artifact.ID)
	fmt.Printf("  SHA-256: %s\n", artifact.Hashes.SHA256)
	if artifact.Hashes.BLAKE3 != "" {
		fmt.Printf("  BLAKE3: %s\n", artifact.Hashes.BLAKE3)
	}
	fmt.Printf("  Size: %d bytes\n", artifact.SizeBytes)

	// Pack the capsule
	if err := cap.Pack(outputPath); err != nil {
		return fmt.Errorf("failed to pack capsule: %w", err)
	}

	fmt.Printf("Created: %s\n", outputPath)
	return nil
}

// ExportCmd exports an artifact from a capsule.
type ExportCmd struct {
	Capsule  string `arg:"" help:"Path to capsule" type:"existingfile"`
	Artifact string `required:"" help:"Artifact ID to export"`
	Out      string `required:"" help:"Output path" type:"path"`
}

func (c *ExportCmd) Run() error {
	capsulePath := c.Capsule
	artifactID := c.Artifact
	outputPath := c.Out

	// Create temporary directory for unpacking
	tempDir, err := os.MkdirTemp("", "capsule-export-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Unpack the capsule
	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		return fmt.Errorf("failed to unpack capsule: %w", err)
	}

	// Export the artifact
	if err := cap.Export(artifactID, capsule.ExportModeIdentity, outputPath); err != nil {
		return fmt.Errorf("failed to export artifact: %w", err)
	}

	// Verify hash
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return fmt.Errorf("failed to read exported file: %w", err)
	}
	hash := cas.Hash(data)

	artifact := cap.Manifest.Artifacts[artifactID]
	if hash != artifact.Hashes.SHA256 {
		return fmt.Errorf("HASH MISMATCH! Export may be corrupted")
	}

	fmt.Printf("Exported: %s\n", artifactID)
	fmt.Printf("  SHA-256: %s (verified)\n", hash)
	fmt.Printf("  Output: %s\n", outputPath)
	return nil
}

// VerifyCmd verifies capsule integrity.
type VerifyCmd struct {
	Capsule string `arg:"" help:"Path to capsule" type:"existingfile"`
}

func (c *VerifyCmd) Run() error {
	capsulePath := c.Capsule

	// Create temporary directory for unpacking
	tempDir, err := os.MkdirTemp("", "capsule-verify-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Unpack the capsule
	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		return fmt.Errorf("failed to unpack capsule: %w", err)
	}

	fmt.Printf("Capsule: %s\n", capsulePath)
	fmt.Printf("  Version: %s\n", cap.Manifest.CapsuleVersion)
	fmt.Printf("  Created: %s\n", cap.Manifest.CreatedAt)
	fmt.Printf("  Artifacts: %d\n", len(cap.Manifest.Artifacts))

	// Verify each artifact
	errors := 0
	for id, artifact := range cap.Manifest.Artifacts {
		data, err := cap.GetStore().Retrieve(artifact.PrimaryBlobSHA256)
		if err != nil {
			fmt.Printf("  [FAIL] %s: blob not found\n", id)
			errors++
			continue
		}

		hash := cas.Hash(data)
		if hash != artifact.Hashes.SHA256 {
			fmt.Printf("  [FAIL] %s: hash mismatch\n", id)
			errors++
			continue
		}

		fmt.Printf("  [OK] %s (%d bytes)\n", id, len(data))
	}

	if errors > 0 {
		return fmt.Errorf("verification failed: %d error(s)", errors)
	}

	fmt.Println("Verification passed!")
	return nil
}

// SelfcheckCmd runs self-check verification plan.
type SelfcheckCmd struct {
	Capsule string `arg:"" help:"Path to capsule" type:"existingfile"`
	Plan    string `help:"Plan ID to run"`
	JSON    bool   `help:"Output as JSON"`
}

func (c *SelfcheckCmd) Run() error {
	capsulePath := c.Capsule
	planID := c.Plan
	jsonOutput := c.JSON

	// Create temporary directory for unpacking
	tempDir, err := os.MkdirTemp("", "capsule-selfcheck-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Unpack the capsule
	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		return fmt.Errorf("failed to unpack capsule: %w", err)
	}

	// Determine which plan to run
	var plan *selfcheck.Plan
	if planID != "" {
		// Look up plan by ID
		if planID == "identity-bytes" {
			// Built-in identity-bytes plan - need an artifact ID
			for id := range cap.Manifest.Artifacts {
				plan = selfcheck.IdentityBytesPlan(id)
				break
			}
		} else {
			// Look for plan in manifest
			if cap.Manifest.RoundtripPlans != nil {
				if p, ok := cap.Manifest.RoundtripPlans[planID]; ok {
					plan = &selfcheck.Plan{
						ID:          planID,
						Description: p.Description,
					}
					// Convert manifest steps to selfcheck steps
					for i, s := range p.Steps {
						step := selfcheck.PlanStep{Type: s.Type}
						if s.Export != nil {
							step.Export = &selfcheck.ExportStep{
								Mode:       s.Export.Mode,
								ArtifactID: s.Export.ArtifactID,
								OutputKey:  fmt.Sprintf("step_%d_output", i),
							}
						}
						plan.Steps = append(plan.Steps, step)
					}
					// Convert manifest checks to selfcheck checks
					for _, ck := range p.Checks {
						check := selfcheck.PlanCheck{
							Type:  ck.Type,
							Label: ck.Label,
						}
						if ck.ByteEqual != nil {
							check.ByteEqual = &selfcheck.ByteEqualDef{
								ArtifactA: ck.ByteEqual.ArtifactA,
								ArtifactB: ck.ByteEqual.ArtifactB,
							}
						}
						plan.Checks = append(plan.Checks, check)
					}
				}
			}
		}
		if plan == nil {
			return fmt.Errorf("plan not found: %s", planID)
		}
	} else {
		// Default: run identity-bytes on first artifact
		for id := range cap.Manifest.Artifacts {
			plan = selfcheck.IdentityBytesPlan(id)
			break
		}
		if plan == nil {
			return fmt.Errorf("no artifacts in capsule")
		}
	}

	// Execute the plan
	executor := selfcheck.NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		return fmt.Errorf("selfcheck execution failed: %w", err)
	}

	// Output results
	if jsonOutput {
		data, err := report.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize report: %w", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("Self-Check Report\n")
		fmt.Printf("  Plan: %s\n", report.PlanID)
		fmt.Printf("  Status: %s\n", report.Status)
		fmt.Printf("  Created: %s\n", report.CreatedAt)
		fmt.Println()
		for _, result := range report.Results {
			status := "[PASS]"
			if !result.Pass {
				status = "[FAIL]"
			}
			fmt.Printf("  %s %s\n", status, result.Label)
			if !result.Pass && result.Expected != nil && result.Actual != nil {
				fmt.Printf("    Expected: %s\n", result.Expected.SHA256)
				fmt.Printf("    Actual:   %s\n", result.Actual.SHA256)
			}
		}
		fmt.Println()
		if report.Status == selfcheck.StatusPass {
			fmt.Println("All checks passed!")
		} else {
			fmt.Println("Some checks failed.")
		}
	}

	if report.Status != selfcheck.StatusPass {
		return fmt.Errorf("selfcheck failed")
	}
	return nil
}

// PluginsListCmd lists available plugins.
type PluginsListCmd struct {
	Dir string `help:"Plugin directory path" type:"path"`
}

func (c *PluginsListCmd) Run(ctx *kong.Context) error {
	pluginDir := c.Dir
	if pluginDir == "" {
		pluginDir = getPluginDir()
	}

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	allPlugins := loader.ListPlugins()
	if len(allPlugins) == 0 {
		fmt.Printf("No plugins found in %s\n", pluginDir)
		return nil
	}

	fmt.Printf("Plugins in %s:\n\n", pluginDir)

	formatPlugins := loader.GetPluginsByKind("format")
	if len(formatPlugins) > 0 {
		fmt.Println("Format Plugins:")
		for _, p := range formatPlugins {
			fmt.Printf("  %s v%s\n", p.Manifest.PluginID, p.Manifest.Version)
		}
		fmt.Println()
	}

	toolPlugins := loader.GetPluginsByKind("tool")
	if len(toolPlugins) > 0 {
		fmt.Println("Tool Plugins:")
		for _, p := range toolPlugins {
			fmt.Printf("  %s v%s\n", p.Manifest.PluginID, p.Manifest.Version)
		}
	}

	return nil
}

// DetectCmd detects file format using plugins.
type DetectCmd struct {
	Path string `arg:"" help:"Path to file to detect" type:"existingpath"`
}

func (c *DetectCmd) Run(ctx *kong.Context) error {
	path, err := filepath.Abs(c.Path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	pluginDir := getPluginDir()

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	formatPlugins := loader.GetPluginsByKind("format")
	if len(formatPlugins) == 0 {
		return fmt.Errorf("no format plugins found")
	}

	fmt.Printf("Detecting format of: %s\n\n", path)

	for _, p := range formatPlugins {
		req := plugins.NewDetectRequest(path)
		resp, err := plugins.ExecutePlugin(p, req)
		if err != nil {
			fmt.Printf("  %s: error (%v)\n", p.Manifest.PluginID, err)
			continue
		}

		result, err := plugins.ParseDetectResult(resp)
		if err != nil {
			fmt.Printf("  %s: parse error (%v)\n", p.Manifest.PluginID, err)
			continue
		}

		if result.Detected {
			fmt.Printf("  [MATCH] %s: %s\n", p.Manifest.PluginID, result.Reason)
		} else {
			fmt.Printf("  [no]    %s: %s\n", p.Manifest.PluginID, result.Reason)
		}
	}

	return nil
}

// EnumerateCmd enumerates contents of archive.
type EnumerateCmd struct {
	Path string `arg:"" help:"Path to archive" type:"existingpath"`
}

func (c *EnumerateCmd) Run(ctx *kong.Context) error {
	path, err := filepath.Abs(c.Path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	pluginDir := getPluginDir()

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	// First detect which plugin matches
	formatPlugins := loader.GetPluginsByKind("format")
	var matchedPlugin *plugins.Plugin

	for _, p := range formatPlugins {
		req := plugins.NewDetectRequest(path)
		resp, err := plugins.ExecutePlugin(p, req)
		if err != nil {
			continue
		}

		result, err := plugins.ParseDetectResult(resp)
		if err != nil {
			continue
		}

		if result.Detected {
			matchedPlugin = p
			break
		}
	}

	if matchedPlugin == nil {
		return fmt.Errorf("no matching format plugin found for: %s", path)
	}

	fmt.Printf("Enumerating: %s (using %s)\n\n", path, matchedPlugin.Manifest.PluginID)

	req := plugins.NewEnumerateRequest(path)
	resp, err := plugins.ExecutePlugin(matchedPlugin, req)
	if err != nil {
		return fmt.Errorf("enumerate failed: %w", err)
	}

	result, err := plugins.ParseEnumerateResult(resp)
	if err != nil {
		return fmt.Errorf("failed to parse result: %w", err)
	}

	for _, entry := range result.Entries {
		typeStr := "F"
		if entry.IsDir {
			typeStr = "D"
		}
		fmt.Printf("  [%s] %s (%d bytes)\n", typeStr, entry.Path, entry.SizeBytes)
	}

	fmt.Printf("\nTotal: %d entries\n", len(result.Entries))
	return nil
}

// TestCmd runs tests against golden hashes.
type TestCmd struct {
	FixturesDir string `arg:"" help:"Path to fixtures directory" type:"existingdir"`
	Golden      string `help:"Path to golden hashes directory" type:"path"`
}

func (c *TestCmd) Run() error {
	fixturesDir, err := filepath.Abs(c.FixturesDir)
	if err != nil {
		return fmt.Errorf("invalid fixtures path: %w", err)
	}

	goldenDir := c.Golden
	if goldenDir == "" {
		goldenDir = filepath.Join(fixturesDir, "goldens")
	} else {
		goldenDir, _ = filepath.Abs(goldenDir)
	}

	// Find all capsule files in fixtures directory
	capsuleFiles, err := filepath.Glob(filepath.Join(fixturesDir, "*.capsule.tar.xz"))
	if err != nil {
		return fmt.Errorf("failed to find capsules: %w", err)
	}

	// Also look for input files to ingest
	inputFiles, err := filepath.Glob(filepath.Join(fixturesDir, "inputs", "*"))
	if err != nil {
		inputFiles = nil
	}

	fmt.Printf("Capsule Test Runner\n")
	fmt.Printf("  Fixtures: %s\n", fixturesDir)
	fmt.Printf("  Goldens:  %s\n", goldenDir)
	fmt.Println()

	passed := 0
	failed := 0
	var failures []string

	// Test existing capsules
	for _, capsulePath := range capsuleFiles {
		name := filepath.Base(capsulePath)
		name = name[:len(name)-len(".capsule.tar.xz")]

		result, err := runCapsuleTest(capsulePath, goldenDir, name)
		if err != nil {
			fmt.Printf("  [FAIL] %s: %v\n", name, err)
			failed++
			failures = append(failures, fmt.Sprintf("%s: %v", name, err))
		} else if result {
			fmt.Printf("  [PASS] %s\n", name)
			passed++
		} else {
			fmt.Printf("  [FAIL] %s: hash mismatch\n", name)
			failed++
			failures = append(failures, fmt.Sprintf("%s: hash mismatch", name))
		}
	}

	// Test input files (ingest -> selfcheck -> compare)
	for _, inputPath := range inputFiles {
		info, err := os.Stat(inputPath)
		if err != nil || info.IsDir() {
			continue
		}

		name := filepath.Base(inputPath)
		ext := filepath.Ext(name)
		testName := name[:len(name)-len(ext)]

		result, err := runIngestTest(inputPath, goldenDir, testName)
		if err != nil {
			fmt.Printf("  [FAIL] %s (ingest): %v\n", testName, err)
			failed++
			failures = append(failures, fmt.Sprintf("%s: %v", testName, err))
		} else if result {
			fmt.Printf("  [PASS] %s (ingest)\n", testName)
			passed++
		} else {
			fmt.Printf("  [FAIL] %s (ingest): hash mismatch\n", testName)
			failed++
			failures = append(failures, fmt.Sprintf("%s: hash mismatch", testName))
		}
	}

	fmt.Println()
	fmt.Printf("Results: %d passed, %d failed\n", passed, failed)

	if failed > 0 {
		fmt.Println("\nFailures:")
		for _, f := range failures {
			fmt.Printf("  - %s\n", f)
		}
		return fmt.Errorf("%d test(s) failed", failed)
	}

	return nil
}

// RunCmd runs a tool plugin with Nix executor.
type RunCmd struct {
	Tool    string `arg:"" help:"Tool plugin ID"`
	Profile string `arg:"" help:"Profile to run"`
	Input   string `help:"Input file path" type:"existingfile"`
	Out     string `help:"Output directory" type:"path"`
}

func (c *RunCmd) Run() error {
	toolID := c.Tool
	profile := c.Profile
	inputPath := c.Input
	outDir := c.Out

	if inputPath != "" {
		inputPath, _ = filepath.Abs(inputPath)
	}

	if outDir == "" {
		outDir, _ = os.MkdirTemp("", "capsule-run-*")
		defer os.RemoveAll(outDir)
	} else {
		outDir, _ = filepath.Abs(outDir)
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Find the nix flake
	flakePath := getFlakePath()
	if flakePath == "" {
		return fmt.Errorf("nix flake not found (looked for nix/flake.nix)")
	}

	fmt.Printf("Running tool: %s\n", toolID)
	fmt.Printf("  Profile: %s\n", profile)
	fmt.Printf("  Input: %s\n", inputPath)
	fmt.Printf("  Output: %s\n", outDir)
	fmt.Printf("  Flake: %s\n", flakePath)
	fmt.Println()

	// Create request
	req := runner.NewRequest(toolID, profile)
	if inputPath != "" {
		req.Inputs = []string{inputPath}
	}

	// Execute with Nix
	executor := runner.NewNixExecutor(flakePath)
	ctx := context.Background()

	var inputPaths []string
	if inputPath != "" {
		inputPaths = []string{inputPath}
	}

	result, err := executor.ExecuteRequest(ctx, req, inputPaths)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	fmt.Printf("Execution completed\n")
	fmt.Printf("  Exit code: %d\n", result.ExitCode)
	fmt.Printf("  Duration: %v\n", result.Duration)

	if len(result.TranscriptData) > 0 {
		fmt.Printf("  Transcript hash: %s\n", result.TranscriptHash)

		// Parse and display transcript
		events, err := runner.ParseNixTranscript(result.TranscriptData)
		if err == nil {
			fmt.Println("\nTranscript events:")
			for _, e := range events {
				eventJSON, _ := json.Marshal(e)
				fmt.Printf("  %s\n", eventJSON)
			}
		}

		// Write transcript to output
		transcriptPath := filepath.Join(outDir, "transcript.jsonl")
		if err := os.WriteFile(transcriptPath, result.TranscriptData, 0644); err == nil {
			fmt.Printf("\nTranscript written to: %s\n", transcriptPath)
		}
	}

	if len(result.Stdout) > 0 {
		fmt.Printf("\nStdout:\n%s\n", result.Stdout)
	}
	if len(result.Stderr) > 0 {
		fmt.Printf("\nStderr:\n%s\n", result.Stderr)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("tool exited with code %d", result.ExitCode)
	}

	return nil
}

// ToolRunCmd runs tool on artifact and stores transcript.
type ToolRunCmd struct {
	Capsule  string `arg:"" help:"Path to capsule" type:"existingfile"`
	Artifact string `arg:"" help:"Artifact ID"`
	Tool     string `arg:"" help:"Tool plugin ID"`
	Profile  string `arg:"" help:"Profile to run"`
}

func (c *ToolRunCmd) Run() error {
	capsulePath := c.Capsule
	artifactID := c.Artifact
	toolID := c.Tool
	profile := c.Profile

	// Create temporary directory for unpacking
	tempDir, err := os.MkdirTemp("", "capsule-tool-run-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Unpack the capsule
	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		return fmt.Errorf("failed to unpack capsule: %w", err)
	}

	// Check artifact exists
	artifact, ok := cap.Manifest.Artifacts[artifactID]
	if !ok {
		return fmt.Errorf("artifact not found: %s", artifactID)
	}

	fmt.Printf("Running tool on capsule artifact\n")
	fmt.Printf("  Capsule: %s\n", capsulePath)
	fmt.Printf("  Artifact: %s\n", artifactID)
	fmt.Printf("  Tool: %s\n", toolID)
	fmt.Printf("  Profile: %s\n", profile)
	fmt.Println()

	// Export artifact to temp directory for tool input
	inputPath := filepath.Join(tempDir, "input", artifact.OriginalName)
	if err := os.MkdirAll(filepath.Dir(inputPath), 0755); err != nil {
		return fmt.Errorf("failed to create input dir: %w", err)
	}
	if err := cap.Export(artifactID, capsule.ExportModeIdentity, inputPath); err != nil {
		return fmt.Errorf("failed to export artifact: %w", err)
	}

	// Find the nix flake
	flakePath := getFlakePath()
	if flakePath == "" {
		return fmt.Errorf("nix flake not found")
	}

	// Create runner request
	req := runner.NewRequest(toolID, profile)
	req.Inputs = []string{inputPath}

	// Execute with Nix
	executor := runner.NewNixExecutor(flakePath)
	ctx := context.Background()

	result, err := executor.ExecuteRequest(ctx, req, []string{inputPath})
	if err != nil {
		return fmt.Errorf("tool execution failed: %w", err)
	}

	fmt.Printf("Tool execution completed\n")
	fmt.Printf("  Exit code: %d\n", result.ExitCode)
	fmt.Printf("  Duration: %v\n", result.Duration)

	if len(result.TranscriptData) == 0 {
		return fmt.Errorf("no transcript generated")
	}

	fmt.Printf("  Transcript hash: %s\n", result.TranscriptHash)

	// Create run record
	runID := fmt.Sprintf("run-%s-%s-%d", toolID, profile, len(cap.Manifest.Runs)+1)
	run := &capsule.Run{
		ID: runID,
		Plugin: &capsule.PluginInfo{
			PluginID: toolID,
			Kind:     "tool",
		},
		Inputs: []capsule.RunInput{
			{ArtifactID: artifactID},
		},
		Command: &capsule.Command{
			Profile: profile,
		},
		Status: "completed",
	}

	// Add run to capsule
	if err := cap.AddRun(run, result.TranscriptData); err != nil {
		return fmt.Errorf("failed to add run: %w", err)
	}

	// Save manifest
	if err := cap.SaveManifest(); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	// Repack capsule
	if err := cap.Pack(capsulePath); err != nil {
		return fmt.Errorf("failed to repack capsule: %w", err)
	}

	fmt.Printf("\nRun stored: %s\n", runID)
	fmt.Printf("Capsule updated: %s\n", capsulePath)

	// Display transcript events
	events, err := runner.ParseNixTranscript(result.TranscriptData)
	if err == nil && len(events) > 0 {
		fmt.Println("\nTranscript events:")
		for _, e := range events {
			eventJSON, _ := json.Marshal(e)
			fmt.Printf("  %s\n", eventJSON)
		}
	}

	return nil
}

// CompareCmd compares transcripts between two runs.
type CompareCmd struct {
	Capsule string `arg:"" help:"Path to capsule" type:"existingfile"`
	Run1    string `arg:"" help:"First run ID"`
	Run2    string `arg:"" help:"Second run ID"`
}

func (c *CompareCmd) Run() error {
	capsulePath := c.Capsule
	run1ID := c.Run1
	run2ID := c.Run2

	// Create temporary directory for unpacking
	tempDir, err := os.MkdirTemp("", "capsule-compare-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Unpack the capsule
	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		return fmt.Errorf("failed to unpack capsule: %w", err)
	}

	// Get run information
	run1, ok := cap.Manifest.Runs[run1ID]
	if !ok {
		return fmt.Errorf("run not found: %s", run1ID)
	}
	run2, ok := cap.Manifest.Runs[run2ID]
	if !ok {
		return fmt.Errorf("run not found: %s", run2ID)
	}

	fmt.Printf("Comparing transcripts\n")
	fmt.Printf("  Capsule: %s\n", capsulePath)
	fmt.Printf("  Run 1: %s\n", run1ID)
	fmt.Printf("  Run 2: %s\n", run2ID)
	fmt.Println()

	// Get transcript hashes
	hash1 := ""
	hash2 := ""
	if run1.Outputs != nil {
		hash1 = run1.Outputs.TranscriptBlobSHA256
	}
	if run2.Outputs != nil {
		hash2 = run2.Outputs.TranscriptBlobSHA256
	}

	if hash1 == "" {
		return fmt.Errorf("run %s has no transcript", run1ID)
	}
	if hash2 == "" {
		return fmt.Errorf("run %s has no transcript", run2ID)
	}

	fmt.Printf("Transcript hashes:\n")
	fmt.Printf("  Run 1: %s\n", hash1)
	fmt.Printf("  Run 2: %s\n", hash2)
	fmt.Println()

	if hash1 == hash2 {
		fmt.Println("Result: IDENTICAL")
		fmt.Println("  Transcripts are byte-for-byte identical.")
		return nil
	}

	fmt.Println("Result: DIFFERENT")
	fmt.Println("  Transcripts differ. Showing diff:")
	fmt.Println()

	// Get transcript contents
	transcript1, err := cap.GetTranscript(run1ID)
	if err != nil {
		return fmt.Errorf("failed to get transcript 1: %w", err)
	}
	transcript2, err := cap.GetTranscript(run2ID)
	if err != nil {
		return fmt.Errorf("failed to get transcript 2: %w", err)
	}

	// Parse and compare events
	events1, err := runner.ParseNixTranscript(transcript1)
	if err != nil {
		return fmt.Errorf("failed to parse transcript 1: %w", err)
	}
	events2, err := runner.ParseNixTranscript(transcript2)
	if err != nil {
		return fmt.Errorf("failed to parse transcript 2: %w", err)
	}

	fmt.Printf("Event counts: Run 1=%d, Run 2=%d\n\n", len(events1), len(events2))

	// Simple diff: show events that differ
	maxLen := len(events1)
	if len(events2) > maxLen {
		maxLen = len(events2)
	}

	for i := 0; i < maxLen; i++ {
		var e1, e2 string
		if i < len(events1) {
			data, _ := json.Marshal(events1[i])
			e1 = string(data)
		}
		if i < len(events2) {
			data, _ := json.Marshal(events2[i])
			e2 = string(data)
		}

		if e1 != e2 {
			fmt.Printf("[%d] DIFFERS:\n", i)
			if e1 != "" {
				fmt.Printf("  - %s\n", e1)
			} else {
				fmt.Printf("  - (missing)\n")
			}
			if e2 != "" {
				fmt.Printf("  + %s\n", e2)
			} else {
				fmt.Printf("  + (missing)\n")
			}
		}
	}

	return fmt.Errorf("transcripts differ")
}

// RunsListCmd lists all runs in a capsule.
type RunsListCmd struct {
	Capsule string `arg:"" help:"Path to capsule" type:"existingfile"`
}

func (c *RunsListCmd) Run() error {
	capsulePath := c.Capsule

	// Create temporary directory for unpacking
	tempDir, err := os.MkdirTemp("", "capsule-runs-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Unpack the capsule
	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		return fmt.Errorf("failed to unpack capsule: %w", err)
	}

	fmt.Printf("Runs in capsule: %s\n\n", capsulePath)

	if len(cap.Manifest.Runs) == 0 {
		fmt.Println("No runs recorded.")
		return nil
	}

	for id, run := range cap.Manifest.Runs {
		fmt.Printf("  %s\n", id)
		if run.Plugin != nil {
			fmt.Printf("    Plugin: %s\n", run.Plugin.PluginID)
		}
		if run.Command != nil && run.Command.Profile != "" {
			fmt.Printf("    Profile: %s\n", run.Command.Profile)
		}
		fmt.Printf("    Status: %s\n", run.Status)
		if run.Outputs != nil && run.Outputs.TranscriptBlobSHA256 != "" {
			fmt.Printf("    Transcript: %s\n", run.Outputs.TranscriptBlobSHA256[:16]+"...")
		}
		if len(run.Inputs) > 0 {
			fmt.Printf("    Inputs: ")
			for i, input := range run.Inputs {
				if i > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s", input.ArtifactID)
			}
			fmt.Println()
		}
		fmt.Println()
	}

	fmt.Printf("Total: %d run(s)\n", len(cap.Manifest.Runs))
	return nil
}

// GoldenSaveCmd saves golden transcript hash to a file.
type GoldenSaveCmd struct {
	Capsule string `arg:"" help:"Path to capsule" type:"existingfile"`
	RunID   string `arg:"" name:"run" help:"Run ID"`
	Out     string `arg:"" help:"Output golden hash file path" type:"path"`
}

func (c *GoldenSaveCmd) Run() error {
	capsulePath := c.Capsule
	runID := c.RunID
	saveFile := c.Out

	// Create temporary directory for unpacking
	tempDir, err := os.MkdirTemp("", "capsule-golden-save-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Unpack the capsule
	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		return fmt.Errorf("failed to unpack capsule: %w", err)
	}

	// Get run
	run, ok := cap.Manifest.Runs[runID]
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}

	if run.Outputs == nil || run.Outputs.TranscriptBlobSHA256 == "" {
		return fmt.Errorf("run has no transcript: %s", runID)
	}

	transcriptHash := run.Outputs.TranscriptBlobSHA256

	// Save the golden hash
	goldenContent := fmt.Sprintf("%s\n", transcriptHash)
	if err := os.WriteFile(saveFile, []byte(goldenContent), 0644); err != nil {
		return fmt.Errorf("failed to save golden: %w", err)
	}
	fmt.Printf("Golden saved: %s\n", saveFile)
	fmt.Printf("  Run: %s\n", runID)
	fmt.Printf("  Hash: %s\n", transcriptHash)
	return nil
}

// GoldenCheckCmd checks transcript against golden hash.
type GoldenCheckCmd struct {
	Capsule string `arg:"" help:"Path to capsule" type:"existingfile"`
	RunID   string `arg:"" name:"run" help:"Run ID"`
	Golden  string `arg:"" help:"Golden hash file to check against" type:"existingfile"`
}

func (c *GoldenCheckCmd) Run() error {
	capsulePath := c.Capsule
	runID := c.RunID
	checkFile := c.Golden

	// Create temporary directory for unpacking
	tempDir, err := os.MkdirTemp("", "capsule-golden-check-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Unpack the capsule
	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		return fmt.Errorf("failed to unpack capsule: %w", err)
	}

	// Get run
	run, ok := cap.Manifest.Runs[runID]
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}

	if run.Outputs == nil || run.Outputs.TranscriptBlobSHA256 == "" {
		return fmt.Errorf("run has no transcript: %s", runID)
	}

	transcriptHash := run.Outputs.TranscriptBlobSHA256

	// Check against golden
	goldenData, err := os.ReadFile(checkFile)
	if err != nil {
		return fmt.Errorf("failed to read golden: %w", err)
	}
	goldenHash := strings.TrimSpace(string(goldenData))

	fmt.Printf("Checking against golden: %s\n", checkFile)
	fmt.Printf("  Run: %s\n", runID)
	fmt.Printf("  Expected: %s\n", goldenHash)
	fmt.Printf("  Actual:   %s\n", transcriptHash)
	fmt.Println()

	if goldenHash == transcriptHash {
		fmt.Println("Result: PASS")
		fmt.Println("  Transcript matches golden.")
		return nil
	}

	fmt.Println("Result: FAIL")
	fmt.Println("  Transcript does not match golden!")
	return fmt.Errorf("golden mismatch")
}

// ExtractIRCmd extracts IR from a file.
type ExtractIRCmd struct {
	Path   string `arg:"" help:"Path to input file" type:"existingfile"`
	Format string `required:"" help:"Source format (e.g., usfm, osis)"`
	Out    string `required:"" help:"Output IR JSON path" type:"path"`
}

func (c *ExtractIRCmd) Run() error {
	inputPath, _ := filepath.Abs(c.Path)
	format := c.Format
	outputPath, _ := filepath.Abs(c.Out)

	// Load plugins
	pluginDir := getPluginDir()
	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	// Find format plugin
	pluginID := "format." + format
	plugin, err := loader.GetPlugin(pluginID)
	if err != nil {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	fmt.Printf("Extracting IR from: %s\n", inputPath)
	fmt.Printf("  Format: %s\n", format)
	fmt.Printf("  Output: %s\n", outputPath)
	fmt.Println()

	// Create temp directory for IR output
	tempDir, err := os.MkdirTemp("", "capsule-extract-ir-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Execute extract-ir
	req := plugins.NewExtractIRRequest(inputPath, tempDir)
	resp, err := plugins.ExecutePlugin(plugin, req)
	if err != nil {
		return fmt.Errorf("extract-ir failed: %w", err)
	}

	result, err := plugins.ParseExtractIRResult(resp)
	if err != nil {
		return fmt.Errorf("failed to parse result: %w", err)
	}

	// Copy IR to output
	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		return fmt.Errorf("failed to read IR: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}
	if err := os.WriteFile(outputPath, irData, 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	fmt.Printf("IR extracted successfully\n")
	fmt.Printf("  Output: %s\n", outputPath)
	if result.LossClass != "" {
		fmt.Printf("  Loss class: %s\n", result.LossClass)
	}

	return nil
}

// EmitNativeCmd emits native format from IR.
type EmitNativeCmd struct {
	IR     string `arg:"" help:"Path to IR JSON file" type:"existingfile"`
	Format string `required:"" help:"Target format (e.g., osis, html)"`
	Out    string `required:"" help:"Output path" type:"path"`
}

func (c *EmitNativeCmd) Run() error {
	irPath, _ := filepath.Abs(c.IR)
	format := c.Format
	outputPath, _ := filepath.Abs(c.Out)

	// Load plugins
	pluginDir := getPluginDir()
	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	// Find format plugin
	pluginID := "format." + format
	plugin, err := loader.GetPlugin(pluginID)
	if err != nil {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	fmt.Printf("Emitting native format from IR: %s\n", irPath)
	fmt.Printf("  Format: %s\n", format)
	fmt.Printf("  Output: %s\n", outputPath)
	fmt.Println()

	// Create temp directory for output
	tempDir, err := os.MkdirTemp("", "capsule-emit-native-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Execute emit-native
	req := plugins.NewEmitNativeRequest(irPath, tempDir)
	resp, err := plugins.ExecutePlugin(plugin, req)
	if err != nil {
		return fmt.Errorf("emit-native failed: %w", err)
	}

	result, err := plugins.ParseEmitNativeResult(resp)
	if err != nil {
		return fmt.Errorf("failed to parse result: %w", err)
	}

	// Copy output to destination
	outputData, err := os.ReadFile(result.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to read output: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}
	if err := os.WriteFile(outputPath, outputData, 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	fmt.Printf("Native format emitted successfully\n")
	fmt.Printf("  Output: %s\n", outputPath)
	if result.LossClass != "" {
		fmt.Printf("  Loss class: %s\n", result.LossClass)
	}

	return nil
}

// ConvertCmd converts file to different format via IR.
type ConvertCmd struct {
	Path string `arg:"" help:"Path to input file" type:"existingfile"`
	To   string `required:"" help:"Target format"`
	Out  string `required:"" help:"Output path" type:"path"`
}

func (c *ConvertCmd) Run() error {
	inputPath, _ := filepath.Abs(c.Path)
	toFormat := c.To
	outputPath, _ := filepath.Abs(c.Out)

	// Load plugins
	pluginDir := getPluginDir()
	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	fmt.Printf("Converting: %s\n", inputPath)
	fmt.Printf("  To format: %s\n", toFormat)
	fmt.Printf("  Output: %s\n", outputPath)
	fmt.Println()

	// Detect source format
	formatPlugins := loader.GetPluginsByKind("format")
	var sourcePlugin *plugins.Plugin
	var sourceFormat string

	for _, p := range formatPlugins {
		req := plugins.NewDetectRequest(inputPath)
		resp, err := plugins.ExecutePlugin(p, req)
		if err != nil {
			continue
		}
		result, err := plugins.ParseDetectResult(resp)
		if err != nil {
			continue
		}
		if result.Detected {
			sourcePlugin = p
			sourceFormat = strings.TrimPrefix(p.Manifest.PluginID, "format.")
			break
		}
	}

	if sourcePlugin == nil {
		return fmt.Errorf("could not detect source format")
	}

	fmt.Printf("Detected source format: %s\n", sourceFormat)
	fmt.Println()

	// Find target plugin
	targetPluginID := "format." + toFormat
	targetPlugin, err := loader.GetPlugin(targetPluginID)
	if err != nil {
		return fmt.Errorf("target plugin not found: %s", targetPluginID)
	}

	// Create temp directory for intermediate files
	tempDir, err := os.MkdirTemp("", "capsule-convert-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Step 1: Extract IR from source
	fmt.Println("Step 1: Extracting IR from source...")
	irDir := filepath.Join(tempDir, "ir")
	os.MkdirAll(irDir, 0755)

	extractReq := plugins.NewExtractIRRequest(inputPath, irDir)
	extractResp, err := plugins.ExecutePlugin(sourcePlugin, extractReq)
	if err != nil {
		return fmt.Errorf("extract-ir failed: %w", err)
	}

	extractResult, err := plugins.ParseExtractIRResult(extractResp)
	if err != nil {
		return fmt.Errorf("failed to parse extract-ir result: %w", err)
	}

	fmt.Printf("  IR path: %s\n", extractResult.IRPath)
	if extractResult.LossClass != "" {
		fmt.Printf("  Loss class: %s\n", extractResult.LossClass)
	}

	// Step 2: Emit native format from IR
	fmt.Println("\nStep 2: Emitting native format...")
	emitDir := filepath.Join(tempDir, "output")
	os.MkdirAll(emitDir, 0755)

	emitReq := plugins.NewEmitNativeRequest(extractResult.IRPath, emitDir)
	emitResp, err := plugins.ExecutePlugin(targetPlugin, emitReq)
	if err != nil {
		return fmt.Errorf("emit-native failed: %w", err)
	}

	emitResult, err := plugins.ParseEmitNativeResult(emitResp)
	if err != nil {
		return fmt.Errorf("failed to parse emit-native result: %w", err)
	}

	fmt.Printf("  Output path: %s\n", emitResult.OutputPath)
	if emitResult.LossClass != "" {
		fmt.Printf("  Loss class: %s\n", emitResult.LossClass)
	}

	// Copy output to destination
	outputData, err := os.ReadFile(emitResult.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to read output: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}
	if err := os.WriteFile(outputPath, outputData, 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	fmt.Println()
	fmt.Printf("Conversion complete!\n")
	fmt.Printf("  Input: %s (%s)\n", inputPath, sourceFormat)
	fmt.Printf("  Output: %s (%s)\n", outputPath, toFormat)

	return nil
}

// IRInfoCmd displays IR structure summary.
type IRInfoCmd struct {
	IR   string `arg:"" help:"Path to IR JSON file" type:"existingfile"`
	JSON bool   `help:"Output as JSON"`
}

func (c *IRInfoCmd) Run() error {
	irPath := c.IR
	jsonOutput := c.JSON

	// Read IR file
	data, err := os.ReadFile(irPath)
	if err != nil {
		return fmt.Errorf("failed to read IR file: %w", err)
	}

	// Parse IR structure (generic JSON parsing)
	var ir map[string]interface{}
	if err := json.Unmarshal(data, &ir); err != nil {
		return fmt.Errorf("failed to parse IR file: %w", err)
	}

	// Build info summary
	info := buildIRInfo(ir)

	if jsonOutput {
		output, _ := json.MarshalIndent(info, "", "  ")
		fmt.Println(string(output))
	} else {
		printIRInfo(info)
	}

	return nil
}

// ToolArchiveCmd creates tool archive capsule from binaries.
type ToolArchiveCmd struct {
	ToolID  string            `arg:"" help:"Tool ID"`
	Version string            `required:"" help:"Tool version"`
	Bin     map[string]string `required:"" help:"Binary name=path pairs"`
	Out     string            `required:"" help:"Output capsule path" type:"path"`
}

func (c *ToolArchiveCmd) Run() error {
	toolID := c.ToolID
	ver := c.Version
	binaries := c.Bin
	outputPath := c.Out

	// Verify binaries exist
	for name, path := range binaries {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("binary not found: %s at %s", name, path)
		}
	}

	fmt.Printf("Creating tool archive: %s v%s\n", toolID, ver)
	fmt.Printf("  Platform: x86_64-linux\n")
	fmt.Printf("  Binaries:\n")
	for name, path := range binaries {
		fmt.Printf("    %s: %s\n", name, path)
	}

	if err := runner.CreateToolArchive(toolID, ver, "x86_64-linux", binaries, outputPath); err != nil {
		return fmt.Errorf("failed to create tool archive: %w", err)
	}

	fmt.Printf("\nTool archive created: %s\n", outputPath)
	return nil
}

// ToolListCmd lists available tools in contrib/tool.
type ToolListCmd struct {
	ContribDir string `arg:"" default:"contrib/tool" help:"Path to contrib/tool directory"`
}

func (c *ToolListCmd) Run() error {
	contribDir := c.ContribDir

	registry := runner.NewToolRegistry(contribDir)
	tools, err := registry.ListTools()
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	if len(tools) == 0 {
		fmt.Printf("No tools found in %s\n", contribDir)
		return nil
	}

	fmt.Printf("Available tools in %s:\n", contribDir)
	for _, tool := range tools {
		fmt.Printf("  - %s\n", tool)
	}

	return nil
}

// DocgenCmd generates documentation.
type DocgenCmd struct {
	Subcommand string `arg:"" enum:"plugins,formats,cli,all" help:"Documentation type to generate"`
	Output     string `short:"o" help:"Output directory" type:"path" default:"docs"`
}

func (c *DocgenCmd) Run() error {
	subcmd := c.Subcommand
	outputDir, _ := filepath.Abs(c.Output)
	pluginDir := getPluginDir()

	gen := docgen.NewGenerator(pluginDir, outputDir)

	switch subcmd {
	case "plugins":
		fmt.Printf("Generating PLUGINS.md...\n")
		if err := gen.GeneratePlugins(); err != nil {
			return err
		}
		fmt.Printf("Generated: %s/PLUGINS.md\n", outputDir)

	case "formats":
		fmt.Printf("Generating FORMATS.md...\n")
		if err := gen.GenerateFormats(); err != nil {
			return err
		}
		fmt.Printf("Generated: %s/FORMATS.md\n", outputDir)

	case "cli":
		fmt.Printf("Generating CLI_REFERENCE.md...\n")
		if err := gen.GenerateCLI(); err != nil {
			return err
		}
		fmt.Printf("Generated: %s/CLI_REFERENCE.md\n", outputDir)

	case "all":
		fmt.Printf("Generating all documentation...\n")
		fmt.Printf("  Plugin dir: %s\n", pluginDir)
		fmt.Printf("  Output dir: %s\n", outputDir)
		fmt.Println()

		if err := gen.GenerateAll(); err != nil {
			return err
		}

		fmt.Println("Generated:")
		fmt.Printf("  - %s/PLUGINS.md\n", outputDir)
		fmt.Printf("  - %s/FORMATS.md\n", outputDir)
		fmt.Printf("  - %s/CLI_REFERENCE.md\n", outputDir)
	}

	return nil
}

// JuniperCmd provides SWORD module tools.
type JuniperCmd struct {
	List       JuniperListCmd       `cmd:"" help:"List Bible modules in SWORD installation"`
	Ingest     JuniperIngestCmd     `cmd:"" help:"Ingest SWORD modules into capsules (raw, no IR)"`
	Install    JuniperInstallCmd    `cmd:"" help:"Install SWORD modules as capsules with IR (recommended)"`
	CASToSword JuniperCASToSwordCmd `cmd:"cas-to-sword" help:"Convert CAS capsule to SWORD module"`
	Hugo       JuniperHugoCmd       `cmd:"" help:"Export SWORD modules to Hugo JSON data files"`
}

// JuniperListCmd lists Bible modules in a SWORD installation.
type JuniperListCmd struct {
	Path string `arg:"" optional:"" help:"Path to SWORD installation (default: ~/.sword)"`
}

func (c *JuniperListCmd) Run() error {
	swordPath := c.Path
	if swordPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		swordPath = filepath.Join(home, ".sword")
	}

	// Check if mods.d exists
	modsDir := filepath.Join(swordPath, "mods.d")
	if _, err := os.Stat(modsDir); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("SWORD installation not found at %s (missing mods.d)", swordPath)
	}

	// Find all .conf files
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return fmt.Errorf("failed to read mods.d: %w", err)
	}

	fmt.Printf("Bible modules in %s:\n\n", swordPath)
	fmt.Printf("%-15s %-8s %-40s\n", "MODULE", "LANG", "DESCRIPTION")
	fmt.Printf("%-15s %-8s %-40s\n", "------", "----", "-----------")

	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}

		confPath := filepath.Join(modsDir, e.Name())
		module := parseConfForList(confPath)
		if module == nil || module.modType != "Bible" {
			continue
		}

		desc := module.description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		encrypted := ""
		if module.encrypted {
			encrypted = " [encrypted]"
		}
		fmt.Printf("%-15s %-8s %-40s%s\n", module.name, module.lang, desc, encrypted)
		count++
	}

	fmt.Printf("\nTotal: %d Bible modules\n", count)
	return nil
}

// JuniperIngestCmd ingests SWORD modules into capsules.
type JuniperIngestCmd struct {
	Modules []string `arg:"" optional:"" help:"Module names to ingest (or --all)"`
	Path    string   `help:"Path to SWORD installation (default: ~/.sword)"`
	Output  string   `short:"o" help:"Output directory (default: capsules)" default:"capsules"`
	All     bool     `short:"a" help:"Ingest all Bible modules"`
}

func (c *JuniperIngestCmd) Run() error {
	swordPath := c.Path
	if swordPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		swordPath = filepath.Join(home, ".sword")
	}

	// Check if mods.d exists
	modsDir := filepath.Join(swordPath, "mods.d")
	if _, err := os.Stat(modsDir); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("SWORD installation not found at %s", swordPath)
	}

	// Get all modules
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return fmt.Errorf("failed to read mods.d: %w", err)
	}

	// Build module list
	var modules []*juniperModule
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}

		confPath := filepath.Join(modsDir, e.Name())
		module := parseConfForList(confPath)
		if module == nil || module.modType != "Bible" {
			continue
		}
		module.confPath = confPath
		modules = append(modules, module)
	}

	if len(modules) == 0 {
		return fmt.Errorf("no Bible modules found in %s", swordPath)
	}

	// Determine which modules to ingest
	var toIngest []*juniperModule
	if c.All {
		toIngest = modules
	} else if len(c.Modules) > 0 {
		moduleMap := make(map[string]*juniperModule)
		for _, m := range modules {
			moduleMap[m.name] = m
		}
		for _, name := range c.Modules {
			if m, ok := moduleMap[name]; ok {
				toIngest = append(toIngest, m)
			} else {
				fmt.Printf("Warning: module '%s' not found\n", name)
			}
		}
	} else {
		return fmt.Errorf("specify module names or use --all")
	}

	if len(toIngest) == 0 {
		return fmt.Errorf("no modules to ingest")
	}

	// Create output directory
	if err := os.MkdirAll(c.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("Ingesting %d module(s) to %s/\n\n", len(toIngest), c.Output)

	for _, m := range toIngest {
		if m.encrypted {
			fmt.Printf("Skipping %s (encrypted)\n", m.name)
			continue
		}

		capsulePath := filepath.Join(c.Output, m.name+".capsule.tar.gz")
		fmt.Printf("Creating %s...\n", capsulePath)

		if err := ingestSwordModule(swordPath, m, capsulePath); err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		info, _ := os.Stat(capsulePath)
		if info != nil {
			fmt.Printf("  Created: %s (%d bytes)\n", capsulePath, info.Size())
		}
	}

	fmt.Println("\nDone!")
	return nil
}

// JuniperInstallCmd installs SWORD modules as capsules with IR generated.
type JuniperInstallCmd struct {
	Modules []string `arg:"" optional:"" help:"Module names to install (or --all)"`
	Path    string   `help:"Path to SWORD installation (default: ~/.sword)"`
	Output  string   `short:"o" help:"Output directory (default: capsules)" default:"capsules"`
	All     bool     `short:"a" help:"Install all Bible modules"`
}

func (c *JuniperInstallCmd) Run() error {
	swordPath := c.Path
	if swordPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		swordPath = filepath.Join(home, ".sword")
	}

	// Check if mods.d exists
	modsDir := filepath.Join(swordPath, "mods.d")
	if _, err := os.Stat(modsDir); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("SWORD installation not found at %s", swordPath)
	}

	// Get all modules
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return fmt.Errorf("failed to read mods.d: %w", err)
	}

	// Build module list
	var modules []*juniperModule
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}

		confPath := filepath.Join(modsDir, e.Name())
		module := parseConfForList(confPath)
		if module == nil || module.modType != "Bible" {
			continue
		}
		module.confPath = confPath
		modules = append(modules, module)
	}

	if len(modules) == 0 {
		return fmt.Errorf("no Bible modules found in %s", swordPath)
	}

	// Determine which modules to install
	var toInstall []*juniperModule
	if c.All {
		toInstall = modules
	} else if len(c.Modules) > 0 {
		moduleMap := make(map[string]*juniperModule)
		for _, m := range modules {
			moduleMap[m.name] = m
		}
		for _, name := range c.Modules {
			if m, ok := moduleMap[name]; ok {
				toInstall = append(toInstall, m)
			} else {
				fmt.Printf("Warning: module '%s' not found\n", name)
			}
		}
	} else {
		return fmt.Errorf("specify module names or use --all")
	}

	if len(toInstall) == 0 {
		return fmt.Errorf("no modules to install")
	}

	// Create output directory
	if err := os.MkdirAll(c.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("Installing %d module(s) to %s/ (with IR generation)\n\n", len(toInstall), c.Output)

	successful := 0
	for _, m := range toInstall {
		if m.encrypted {
			fmt.Printf("Skipping %s (encrypted)\n", m.name)
			continue
		}

		capsulePath := filepath.Join(c.Output, m.name+".capsule.tar.gz")
		fmt.Printf("Installing %s...\n", m.name)

		// Step 1: Ingest
		fmt.Printf("  Ingesting SWORD module...\n")
		if err := ingestSwordModule(swordPath, m, capsulePath); err != nil {
			fmt.Printf("  Error during ingest: %v\n", err)
			continue
		}

		// Step 2: Generate IR
		fmt.Printf("  Generating IR...\n")
		irCmd := &GenerateIRCmd{Capsule: capsulePath}
		if err := irCmd.Run(); err != nil {
			fmt.Printf("  Error during IR generation: %v\n", err)
			fmt.Printf("  (Capsule created but without IR)\n")
			continue
		}

		info, _ := os.Stat(capsulePath)
		if info != nil {
			fmt.Printf("  Done: %s (%d bytes)\n", capsulePath, info.Size())
		}
		successful++
	}

	fmt.Printf("\nInstalled %d/%d modules successfully\n", successful, len(toInstall))
	return nil
}

// juniperModule holds parsed SWORD module metadata for CLI.
type juniperModule struct {
	name        string
	description string
	lang        string
	modType     string
	dataPath    string
	encrypted   bool
	confPath    string
}

// parseConfForList parses a SWORD conf file for listing.
func parseConfForList(path string) *juniperModule {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	module := &juniperModule{}
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}

		// Parse section header
		if line[0] == '[' && line[len(line)-1] == ']' {
			module.name = line[1 : len(line)-1]
			continue
		}

		// Parse key=value
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		switch key {
		case "Description":
			module.description = value
		case "Lang":
			module.lang = value
		case "ModDrv":
			switch value {
			case "zText", "RawText", "zText4", "RawText4":
				module.modType = "Bible"
			case "zCom", "RawCom", "zCom4", "RawCom4":
				module.modType = "Commentary"
			case "zLD", "RawLD", "RawLD4":
				module.modType = "Dictionary"
			case "RawGenBook":
				module.modType = "GenBook"
			default:
				module.modType = "Unknown"
			}
		case "DataPath":
			module.dataPath = value
		case "CipherKey":
			module.encrypted = value != ""
		}
	}

	return module
}

// ingestSwordModule creates a capsule from a SWORD module.
func ingestSwordModule(swordPath string, module *juniperModule, outputPath string) error {
	// Read conf file
	confData, err := os.ReadFile(module.confPath)
	if err != nil {
		return fmt.Errorf("failed to read conf: %w", err)
	}

	// Determine data path
	dataPath := module.dataPath
	if dataPath == "" {
		return fmt.Errorf("no DataPath in conf file")
	}

	// Clean up data path
	dataPath = strings.TrimPrefix(dataPath, "./")
	fullDataPath := filepath.Join(swordPath, dataPath)

	if _, err := os.Stat(fullDataPath); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("data path not found: %s", fullDataPath)
	}

	// Create temp directory for capsule contents
	tempDir, err := os.MkdirTemp("", "sword-capsule-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Create capsule structure
	capsuleDir := filepath.Join(tempDir, "capsule")
	modsDir := filepath.Join(capsuleDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		return fmt.Errorf("failed to create mods.d: %w", err)
	}

	// Write conf file
	confName := strings.ToLower(module.name) + ".conf"
	if err := os.WriteFile(filepath.Join(modsDir, confName), confData, 0644); err != nil {
		return fmt.Errorf("failed to write conf: %w", err)
	}

	// Copy module data
	destDataPath := filepath.Join(capsuleDir, dataPath)
	if err := os.MkdirAll(filepath.Dir(destDataPath), 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}
	if err := fileutil.CopyDir(fullDataPath, destDataPath); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	// Create manifest.json
	manifest := map[string]interface{}{
		"capsule_version": "1.0",
		"module_type":     "bible",
		"id":              module.name,
		"title":           module.description,
		"language":        module.lang,
		"source_format":   "sword",
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(capsuleDir, "manifest.json"), manifestData, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Create tar.gz archive
	return archive.CreateCapsuleTarGz(capsuleDir, outputPath)
}

// JuniperCASToSwordCmd converts a CAS capsule to a SWORD module.
type JuniperCASToSwordCmd struct {
	Capsule string `arg:"" help:"Path to CAS capsule to convert" type:"existingfile"`
	Output  string `short:"o" help:"Output directory for SWORD module (default: .sword in home)"`
	Name    string `short:"n" help:"Module name (default: derived from capsule)"`
}

func (c *JuniperCASToSwordCmd) Run() error {
	capsulePath, _ := filepath.Abs(c.Capsule)
	outputDir := c.Output
	moduleName := c.Name

	// Default output to ~/.sword
	if outputDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		outputDir = filepath.Join(home, ".sword")
	}

	// Derive module name from capsule filename if not provided
	if moduleName == "" {
		base := filepath.Base(capsulePath)
		moduleName = strings.TrimSuffix(base, ".capsule.tar.xz")
		moduleName = strings.TrimSuffix(moduleName, ".capsule.tar.gz")
		moduleName = strings.TrimSuffix(moduleName, ".tar.xz")
		moduleName = strings.TrimSuffix(moduleName, ".tar.gz")
		moduleName = strings.ToUpper(moduleName)
	}

	fmt.Printf("Converting CAS capsule to SWORD module:\n")
	fmt.Printf("  Input:  %s\n", capsulePath)
	fmt.Printf("  Output: %s\n", outputDir)
	fmt.Printf("  Module: %s\n", moduleName)
	fmt.Println()

	// Create temporary directory for unpacking
	tempDir, err := os.MkdirTemp("", "cas-to-sword-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Unpack the capsule
	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		return fmt.Errorf("failed to unpack capsule: %w", err)
	}

	// Check if capsule has IR extractions
	if len(cap.Manifest.IRExtractions) == 0 {
		return fmt.Errorf("capsule has no IR - run 'capsule format ir generate' first")
	}

	// Get the first IR extraction and directly read the IR blob
	var irRecord *capsule.IRRecord
	for _, rec := range cap.Manifest.IRExtractions {
		irRecord = rec
		break
	}

	// Directly retrieve the IR blob from CAS
	irBlobData, err := cap.GetStore().Retrieve(irRecord.IRBlobSHA256)
	if err != nil {
		return fmt.Errorf("failed to retrieve IR blob: %w", err)
	}

	// Parse the IR corpus directly
	var corpus ir.Corpus
	if err := json.Unmarshal(irBlobData, &corpus); err != nil {
		return fmt.Errorf("failed to parse IR corpus: %w", err)
	}

	// Get metadata directly from IR Corpus
	lang := corpus.Language
	if lang == "" {
		lang = "en"
	}
	description := corpus.Title
	if description == "" {
		description = moduleName + " Bible Module"
	}
	versification := corpus.Versification
	if versification == "" {
		versification = "KJV"
	}

	fmt.Printf("  IR ID:       %s\n", corpus.ID)
	fmt.Printf("  Language:    %s\n", lang)
	fmt.Printf("  Title:       %s\n", description)
	fmt.Printf("  Versification: %s\n", versification)
	fmt.Printf("  Documents:   %d\n", len(corpus.Documents))
	fmt.Println()

	// Ensure output directories exist
	modsDir := filepath.Join(outputDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		return fmt.Errorf("failed to create mods.d: %w", err)
	}

	// For now, implement a basic SWORD module creation
	// This creates a zText format module structure
	dataPath := filepath.Join("modules", "texts", "ztext", strings.ToLower(moduleName))
	fullDataPath := filepath.Join(outputDir, dataPath)
	if err := os.MkdirAll(fullDataPath, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create a basic .conf file with versification from IR
	confContent := fmt.Sprintf(`[%s]
DataPath=./%s/
ModDrv=zText
SourceType=OSIS
Encoding=UTF-8
Lang=%s
Description=%s
About=Converted from CAS capsule using Juniper Bible
Category=Biblical Texts
TextSource=Juniper Bible
Versification=%s
Version=1.0
LCSH=Bible.
DistributionLicense=Copyrighted; Free non-commercial distribution
`, moduleName, dataPath, lang, description, versification)

	confPath := filepath.Join(modsDir, strings.ToLower(moduleName)+".conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		return fmt.Errorf("failed to write conf file: %w", err)
	}

	fmt.Printf("Created SWORD module structure:\n")
	fmt.Printf("  Config: %s\n", confPath)
	fmt.Printf("  Data:   %s\n", fullDataPath)
	fmt.Println()
	fmt.Println("Note: To complete the conversion, use osis2mod or sword-utils to populate the module data.")
	fmt.Println("      This command creates the structure; use tool plugins for data conversion.")

	return nil
}

// JuniperHugoCmd exports SWORD modules to Hugo JSON data files.
type JuniperHugoCmd struct {
	Modules []string `arg:"" optional:"" help:"Module names to export (or --all)"`
	Path    string   `help:"Path to SWORD installation (default: ~/.sword)"`
	Output  string   `short:"o" help:"Output directory for Hugo data files" default:"data"`
	All     bool     `short:"a" help:"Export all Bible modules"`
	Workers int      `short:"w" help:"Number of parallel workers (default: number of CPUs)" default:"0"`
}

func (c *JuniperHugoCmd) Run() error {
	cfg := juniper.HugoConfig{
		Path:    c.Path,
		Output:  c.Output,
		All:     c.All,
		Modules: c.Modules,
		Workers: c.Workers,
	}
	return juniper.Hugo(cfg)
}

// CapsuleConvertCmd converts capsule content to a different format.
// It preserves the original by renaming it to filename-old.
type CapsuleConvertCmd struct {
	Capsule string `arg:"" help:"Path to capsule to convert" type:"existingfile"`
	Format  string `required:"" short:"f" help:"Target format (osis, usfm, usx, json, html, epub, markdown, sqlite, txt)"`
}

func (c *CapsuleConvertCmd) Run() error {
	capsulePath, _ := filepath.Abs(c.Capsule)
	targetFormat := c.Format

	fmt.Printf("Converting capsule: %s\n", capsulePath)
	fmt.Printf("Target format: %s\n", targetFormat)
	fmt.Println()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "capsule-convert-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract capsule
	extractDir := filepath.Join(tempDir, "extract")
	if err := extractCapsuleArchive(capsulePath, extractDir); err != nil {
		return fmt.Errorf("failed to extract capsule: %w", err)
	}

	// Find content file and detect format
	contentPath, sourceFormat := findConvertibleContent(extractDir)
	if contentPath == "" {
		return fmt.Errorf("no convertible content found in capsule (supported: OSIS, USFM, USX, JSON, SWORD)")
	}

	fmt.Printf("Detected source format: %s\n", sourceFormat)
	fmt.Printf("Content file: %s\n", filepath.Base(contentPath))
	fmt.Println()

	// Load plugins
	pluginDir := getPluginDir()
	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	// Step 1: Extract IR
	fmt.Println("Step 1: Extracting IR...")
	irDir := filepath.Join(tempDir, "ir")
	os.MkdirAll(irDir, 0755)

	sourcePlugin, err := loader.GetPlugin("format." + sourceFormat)
	if err != nil {
		return fmt.Errorf("no plugin for source format '%s': %w", sourceFormat, err)
	}

	extractReq := plugins.NewExtractIRRequest(contentPath, irDir)
	extractResp, err := plugins.ExecutePlugin(sourcePlugin, extractReq)
	if err != nil {
		return fmt.Errorf("IR extraction failed: %w", err)
	}

	extractResult, err := plugins.ParseExtractIRResult(extractResp)
	if err != nil {
		return fmt.Errorf("failed to parse extract result: %w", err)
	}
	fmt.Printf("  IR extracted (loss class: %s)\n", extractResult.LossClass)

	// Step 2: Emit target format
	fmt.Printf("Step 2: Emitting %s...\n", targetFormat)
	targetPlugin, err := loader.GetPlugin("format." + targetFormat)
	if err != nil {
		return fmt.Errorf("no plugin for target format '%s': %w", targetFormat, err)
	}

	emitDir := filepath.Join(tempDir, "output")
	os.MkdirAll(emitDir, 0755)

	emitReq := plugins.NewEmitNativeRequest(extractResult.IRPath, emitDir)
	emitResp, err := plugins.ExecutePlugin(targetPlugin, emitReq)
	if err != nil {
		return fmt.Errorf("emit failed: %w", err)
	}

	emitResult, err := plugins.ParseEmitNativeResult(emitResp)
	if err != nil {
		return fmt.Errorf("failed to parse emit result: %w", err)
	}
	fmt.Printf("  Output generated (loss class: %s)\n", emitResult.LossClass)

	// Step 3: Create new capsule
	fmt.Println("Step 3: Creating new capsule...")
	newCapsuleDir := filepath.Join(tempDir, "new-capsule")
	os.MkdirAll(newCapsuleDir, 0755)

	// Copy converted output
	outputData, err := os.ReadFile(emitResult.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to read output: %w", err)
	}
	outputName := filepath.Base(emitResult.OutputPath)
	os.WriteFile(filepath.Join(newCapsuleDir, outputName), outputData, 0644)

	// Copy IR
	irData, _ := os.ReadFile(extractResult.IRPath)
	baseName := filepath.Base(capsulePath)
	baseName = strings.TrimSuffix(baseName, ".capsule.tar.gz")
	baseName = strings.TrimSuffix(baseName, ".capsule.tar.xz")
	baseName = strings.TrimSuffix(baseName, ".tar.gz")
	baseName = strings.TrimSuffix(baseName, ".tar.xz")
	os.WriteFile(filepath.Join(newCapsuleDir, baseName+".ir.json"), irData, 0644)

	// Create manifest
	manifest := map[string]interface{}{
		"capsule_version": "1.0",
		"module_type":     "bible",
		"source_format":   targetFormat,
		"converted_from":  sourceFormat,
		"conversion_date": time.Now().Format(time.RFC3339),
		"has_ir":          true,
		"extraction_loss": extractResult.LossClass,
		"emission_loss":   emitResult.LossClass,
	}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(newCapsuleDir, "manifest.json"), manifestData, 0644)

	// Step 4: Rename original and create new
	fmt.Println("Step 4: Finalizing...")
	oldPath := renameCapsuleToOld(capsulePath)
	if oldPath == "" {
		return fmt.Errorf("failed to rename original capsule")
	}
	fmt.Printf("  Original backed up: %s\n", filepath.Base(oldPath))

	if err := archive.CreateCapsuleTarGz(newCapsuleDir, capsulePath); err != nil {
		os.Rename(oldPath, capsulePath) // Restore on failure
		return fmt.Errorf("failed to create capsule: %w", err)
	}

	fmt.Println()
	fmt.Println("Conversion complete!")
	fmt.Printf("  New capsule: %s\n", capsulePath)
	fmt.Printf("  Backup: %s\n", oldPath)
	fmt.Printf("  Combined loss class: %s\n", combineLoss(extractResult.LossClass, emitResult.LossClass))

	return nil
}

// GenerateIRCmd generates IR for a capsule that doesn't have one.
type GenerateIRCmd struct {
	Capsule string `arg:"" help:"Path to capsule" type:"existingfile"`
}

func (c *GenerateIRCmd) Run() error {
	capsulePath, _ := filepath.Abs(c.Capsule)

	fmt.Printf("Generating IR for: %s\n", capsulePath)
	fmt.Println()

	// Check if already has IR
	if capsuleContainsIR(capsulePath) {
		return fmt.Errorf("capsule already contains IR")
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "capsule-ir-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract capsule
	extractDir := filepath.Join(tempDir, "extract")
	if err := extractCapsuleArchive(capsulePath, extractDir); err != nil {
		return fmt.Errorf("failed to extract capsule: %w", err)
	}

	// Find content and detect format
	contentPath, sourceFormat := findConvertibleContent(extractDir)
	if contentPath == "" {
		return fmt.Errorf("no convertible content found (supported: OSIS, USFM, USX, JSON, SWORD)")
	}

	fmt.Printf("Detected format: %s\n", sourceFormat)
	fmt.Printf("Content: %s\n", filepath.Base(contentPath))
	fmt.Println()

	// Load plugins
	pluginDir := getPluginDir()
	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	// Extract IR
	fmt.Println("Extracting IR...")
	irDir := filepath.Join(tempDir, "ir")
	os.MkdirAll(irDir, 0755)

	sourcePlugin, err := loader.GetPlugin("format." + sourceFormat)
	if err != nil {
		return fmt.Errorf("no plugin for format '%s': %w", sourceFormat, err)
	}

	extractReq := plugins.NewExtractIRRequest(contentPath, irDir)
	extractResp, err := plugins.ExecutePlugin(sourcePlugin, extractReq)
	if err != nil {
		return fmt.Errorf("IR extraction failed: %w", err)
	}

	extractResult, err := plugins.ParseExtractIRResult(extractResp)
	if err != nil {
		return fmt.Errorf("failed to parse result: %w", err)
	}
	fmt.Printf("  Loss class: %s\n", extractResult.LossClass)

	// Create new capsule with IR
	fmt.Println("Creating new capsule with IR...")
	newCapsuleDir := filepath.Join(tempDir, "new-capsule")

	// Copy all original files
	if err := fileutil.CopyDir(extractDir, newCapsuleDir); err != nil {
		return fmt.Errorf("failed to copy contents: %w", err)
	}

	// Add IR file
	irData, err := os.ReadFile(extractResult.IRPath)
	if err != nil {
		return fmt.Errorf("failed to read IR: %w", err)
	}

	baseName := filepath.Base(capsulePath)
	baseName = strings.TrimSuffix(baseName, ".capsule.tar.gz")
	baseName = strings.TrimSuffix(baseName, ".capsule.tar.xz")
	baseName = strings.TrimSuffix(baseName, ".tar.gz")
	baseName = strings.TrimSuffix(baseName, ".tar.xz")
	os.WriteFile(filepath.Join(newCapsuleDir, baseName+".ir.json"), irData, 0644)

	// Update manifest
	manifestPath := filepath.Join(newCapsuleDir, "manifest.json")
	manifest := make(map[string]interface{})
	if data, err := os.ReadFile(manifestPath); err == nil {
		json.Unmarshal(data, &manifest)
	}
	manifest["has_ir"] = true
	manifest["ir_generated"] = time.Now().Format(time.RFC3339)
	manifest["ir_loss_class"] = extractResult.LossClass
	manifest["source_format"] = sourceFormat
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(manifestPath, manifestData, 0644)

	// Rename original and create new
	oldPath := renameCapsuleToOld(capsulePath)
	if oldPath == "" {
		return fmt.Errorf("failed to rename original")
	}

	if err := archive.CreateCapsuleTarGz(newCapsuleDir, capsulePath); err != nil {
		os.Rename(oldPath, capsulePath)
		return fmt.Errorf("failed to create capsule: %w", err)
	}

	fmt.Println()
	fmt.Println("IR generation complete!")
	fmt.Printf("  New capsule: %s\n", capsulePath)
	fmt.Printf("  Backup: %s\n", oldPath)
	fmt.Printf("  Loss class: %s\n", extractResult.LossClass)

	return nil
}

// extractCapsuleArchive extracts a capsule to a directory.
func extractCapsuleArchive(capsulePath, destDir string) error {
	f, err := os.Open(capsulePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var tr *tar.Reader

	if strings.HasSuffix(capsulePath, ".tar.xz") {
		// Use xz command for extraction
		cmd := exec.Command("xz", "-dc", capsulePath)
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("xz decompress failed: %w", err)
		}
		tr = tar.NewReader(strings.NewReader(string(output)))
	} else if strings.HasSuffix(capsulePath, ".tar.gz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gzr.Close()
		tr = tar.NewReader(gzr)
	} else {
		tr = tar.NewReader(f)
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Strip first directory component
		name := header.Name
		if idx := strings.Index(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		if name == "" {
			continue
		}

		destPath := filepath.Join(destDir, name)

		if header.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0755)
			continue
		}

		os.MkdirAll(filepath.Dir(destPath), 0755)
		outFile, err := os.Create(destPath)
		if err != nil {
			return err
		}
		io.Copy(outFile, tr)
		outFile.Close()
	}

	return nil
}

// findConvertibleContent finds content in extracted capsule.
func findConvertibleContent(extractDir string) (string, string) {
	patterns := []struct {
		ext    string
		format string
	}{
		{".osis", "osis"},
		{".osis.xml", "osis"},
		{".usx", "usx"},
		{".usfm", "usfm"},
		{".sfm", "usfm"},
		{".json", "json"},
	}

	var found string
	var format string

	filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		name := strings.ToLower(filepath.Base(path))
		if name == "manifest.json" || strings.HasSuffix(name, ".ir.json") {
			return nil
		}

		for _, p := range patterns {
			if strings.HasSuffix(name, p.ext) {
				found = path
				format = p.format
				return filepath.SkipAll
			}
		}
		return nil
	})

	// Check for SWORD module
	if found == "" {
		modsDir := filepath.Join(extractDir, "mods.d")
		if _, err := os.Stat(modsDir); err == nil {
			entries, _ := os.ReadDir(modsDir)
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".conf") {
					found = filepath.Join(modsDir, e.Name())
					format = "sword"
					break
				}
			}
		}
	}

	return found, format
}

// capsuleContainsIR checks if capsule has IR.
func capsuleContainsIR(capsulePath string) bool {
	f, err := os.Open(capsulePath)
	if err != nil {
		return false
	}
	defer f.Close()

	var tr *tar.Reader

	if strings.HasSuffix(capsulePath, ".tar.gz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return false
		}
		defer gzr.Close()
		tr = tar.NewReader(gzr)
	} else {
		tr = tar.NewReader(f)
	}

	for {
		header, err := tr.Next()
		if err != nil {
			break
		}
		if strings.HasSuffix(header.Name, ".ir.json") {
			return true
		}
	}
	return false
}

// renameCapsuleToOld renames capsule to -old version.
func renameCapsuleToOld(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	var oldName string
	if strings.HasSuffix(base, ".capsule.tar.gz") {
		oldName = strings.TrimSuffix(base, ".capsule.tar.gz") + "-old.capsule.tar.gz"
	} else if strings.HasSuffix(base, ".capsule.tar.xz") {
		oldName = strings.TrimSuffix(base, ".capsule.tar.xz") + "-old.capsule.tar.xz"
	} else if strings.HasSuffix(base, ".tar.gz") {
		oldName = strings.TrimSuffix(base, ".tar.gz") + "-old.tar.gz"
	} else if strings.HasSuffix(base, ".tar.xz") {
		oldName = strings.TrimSuffix(base, ".tar.xz") + "-old.tar.xz"
	} else {
		oldName = base + "-old"
	}

	oldPath := filepath.Join(dir, oldName)
	if err := os.Rename(path, oldPath); err != nil {
		return ""
	}
	return oldPath
}

// combineLoss returns the worst loss class.
func combineLoss(a, b string) string {
	order := map[string]int{"L0": 0, "L1": 1, "L2": 2, "L3": 3, "": 0}
	if order[a] > order[b] {
		return a
	}
	return b
}

// CASToSWORDCmd converts a CAS capsule to SWORD format.
type CASToSWORDCmd struct {
	Capsule string `arg:"" help:"Path to CAS capsule" type:"existingfile"`
}

func (c *CASToSWORDCmd) Run() error {
	capsulePath, _ := filepath.Abs(c.Capsule)

	fmt.Printf("Converting CAS capsule to SWORD format: %s\n", capsulePath)
	fmt.Println()

	// Check if it's a CAS capsule
	if !isCASCapsule(capsulePath) {
		return fmt.Errorf("not a CAS capsule (no blobs/ directory found)")
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "capsule-cas-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract capsule
	fmt.Println("Extracting CAS capsule...")
	extractDir := filepath.Join(tempDir, "extract")
	if err := extractCapsuleArchive(capsulePath, extractDir); err != nil {
		return fmt.Errorf("failed to extract: %w", err)
	}

	// Read manifest
	manifestData, err := os.ReadFile(filepath.Join(extractDir, "manifest.json"))
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest casManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	fmt.Printf("  ID: %s\n", manifest.ID)
	fmt.Printf("  Title: %s\n", manifest.Title)
	fmt.Println()

	// Find main artifact
	var mainArtifact *casArtifact
	for i := range manifest.Artifacts {
		if manifest.Artifacts[i].ID == "main" || manifest.Artifacts[i].ID == manifest.MainArtifact {
			mainArtifact = &manifest.Artifacts[i]
			break
		}
	}
	if mainArtifact == nil && len(manifest.Artifacts) > 0 {
		mainArtifact = &manifest.Artifacts[0]
	}
	if mainArtifact == nil {
		return fmt.Errorf("no artifacts found in manifest")
	}

	fmt.Printf("Extracting artifact: %s (%d files)\n", mainArtifact.ID, len(mainArtifact.Files))

	// Create SWORD capsule directory
	swordDir := filepath.Join(tempDir, "sword")
	os.MkdirAll(swordDir, 0755)

	// Extract files from blobs
	extracted := 0
	for _, file := range mainArtifact.Files {
		blobPath := ""
		if file.Blake3 != "" {
			blobPath = filepath.Join(extractDir, "blobs", "blake3", file.Blake3[:2], file.Blake3)
		} else if file.SHA256 != "" {
			blobPath = filepath.Join(extractDir, "blobs", "sha256", file.SHA256[:2], file.SHA256)
		}

		if blobPath == "" {
			continue
		}

		content, err := os.ReadFile(blobPath)
		if err != nil {
			continue
		}

		destPath := filepath.Join(swordDir, file.Path)
		os.MkdirAll(filepath.Dir(destPath), 0755)
		os.WriteFile(destPath, content, 0644)
		extracted++
	}

	fmt.Printf("  Extracted %d files\n", extracted)

	// Create manifest
	swordManifest := map[string]interface{}{
		"capsule_version": "1.0",
		"module_type":     manifest.ModuleType,
		"id":              manifest.ID,
		"title":           manifest.Title,
		"source_format":   "cas-converted",
		"original_format": manifest.SourceFormat,
	}
	swordManifestData, _ := json.MarshalIndent(swordManifest, "", "  ")
	os.WriteFile(filepath.Join(swordDir, "manifest.json"), swordManifestData, 0644)

	// Rename original and create new
	fmt.Println()
	fmt.Println("Creating new capsule...")

	oldPath := renameCapsuleToOld(capsulePath)
	if oldPath == "" {
		return fmt.Errorf("failed to rename original")
	}
	fmt.Printf("  Original backed up: %s\n", filepath.Base(oldPath))

	// Create .tar.gz
	newPath := strings.TrimSuffix(capsulePath, ".tar.xz")
	if !strings.HasSuffix(newPath, ".tar.gz") {
		newPath += ".tar.gz"
	}

	if err := archive.CreateCapsuleTarGz(swordDir, newPath); err != nil {
		os.Rename(oldPath, capsulePath)
		return fmt.Errorf("failed to create capsule: %w", err)
	}

	fmt.Printf("  Created: %s\n", newPath)
	fmt.Println()
	fmt.Println("Conversion complete!")

	return nil
}

// isCASCapsule checks if a capsule is CAS format.
func isCASCapsule(capsulePath string) bool {
	f, err := os.Open(capsulePath)
	if err != nil {
		return false
	}
	defer f.Close()

	var tr *tar.Reader

	if strings.HasSuffix(capsulePath, ".tar.gz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return false
		}
		defer gzr.Close()
		tr = tar.NewReader(gzr)
	} else {
		// Try xz via command
		cmd := exec.Command("xz", "-dc", capsulePath)
		output, err := cmd.Output()
		if err != nil {
			return false
		}
		tr = tar.NewReader(strings.NewReader(string(output)))
	}

	for {
		header, err := tr.Next()
		if err != nil {
			break
		}
		if strings.Contains(header.Name, "blobs/") {
			return true
		}
	}
	return false
}

// casManifest represents a CAS capsule manifest.
type casManifest struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	ModuleType   string        `json:"module_type"`
	SourceFormat string        `json:"source_format"`
	MainArtifact string        `json:"main_artifact"`
	Artifacts    []casArtifact `json:"artifacts"`
}

// casArtifact represents an artifact in a CAS capsule.
type casArtifact struct {
	ID    string    `json:"id"`
	Files []casFile `json:"files"`
}

// casFile represents a file in a CAS artifact.
type casFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Blake3 string `json:"blake3"`
}

// VersionCmd prints version information.
// WebCmd starts the web UI server.
type WebCmd struct {
	Port            int    `help:"HTTP server port" default:"8080"`
	Capsules        string `help:"Directory containing capsules" default:"./capsules" type:"path"`
	Plugins         string `help:"Directory containing plugins" default:"./bin/plugins" type:"path"`
	Sword           string `help:"Directory containing SWORD modules (default: ~/.sword)" type:"path"`
	PluginsExternal bool   `help:"Enable loading external plugins from plugins directory"`
	Restart         bool   `help:"Kill any existing process on the port and restart" short:"r"`
}

func (c *WebCmd) Run() error {
	if c.Restart {
		if err := killProcessOnPort(c.Port); err != nil {
			log.Printf("Warning: could not kill existing process: %v", err)
		}
	}
	cfg := web.Config{
		Port:            c.Port,
		CapsulesDir:     c.Capsules,
		PluginsDir:      c.Plugins,
		SwordDir:        c.Sword,
		PluginsExternal: c.PluginsExternal,
	}
	return web.Start(cfg)
}

// killProcessOnPort finds and kills any process listening on the given port.
func killProcessOnPort(port int) error {
	// Try multiple methods to find the process
	var pids []int

	// Method 1: Use ss command (commonly available on Linux)
	cmd := exec.Command("ss", "-tlnp", fmt.Sprintf("sport = :%d", port))
	output, err := cmd.Output()
	if err == nil {
		// Parse ss output for pid=NNNN
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if idx := strings.Index(line, "pid="); idx != -1 {
				rest := line[idx+4:]
				endIdx := strings.IndexAny(rest, ",) \t")
				if endIdx == -1 {
					endIdx = len(rest)
				}
				if pid, err := strconv.Atoi(rest[:endIdx]); err == nil && pid > 0 {
					pids = append(pids, pid)
				}
			}
		}
	}

	// Method 2: Fallback to fuser
	if len(pids) == 0 {
		cmd = exec.Command("fuser", fmt.Sprintf("%d/tcp", port))
		output, err = cmd.Output()
		if err == nil {
			for _, p := range strings.Fields(string(output)) {
				if pid, err := strconv.Atoi(p); err == nil && pid > 0 {
					pids = append(pids, pid)
				}
			}
		}
	}

	// Method 3: Fallback to lsof
	if len(pids) == 0 {
		cmd = exec.Command("lsof", "-t", "-i", fmt.Sprintf(":%d", port))
		output, err = cmd.Output()
		if err == nil {
			for _, p := range strings.Split(strings.TrimSpace(string(output)), "\n") {
				if pid, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && pid > 0 {
					pids = append(pids, pid)
				}
			}
		}
	}

	if len(pids) == 0 {
		return nil
	}

	// Kill the processes
	for _, pid := range pids {
		log.Printf("Killing existing process on port %d (PID %d)...", port, pid)
		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		if err := proc.Kill(); err != nil {
			log.Printf("Warning: failed to kill PID %d: %v", pid, err)
		}
	}

	// Give the process time to release the port
	time.Sleep(500 * time.Millisecond)
	return nil
}

// APICmd starts the REST API server.
type APICmd struct {
	Port            int    `help:"HTTP server port" default:"8081"`
	Capsules        string `help:"Directory containing capsules" default:"./capsules" type:"path"`
	Plugins         string `help:"Directory containing plugins" default:"./plugins" type:"path"`
	PluginsExternal bool   `help:"Enable loading external plugins from plugins directory"`
}

func (c *APICmd) Run() error {
	cfg := api.Config{
		Port:            c.Port,
		CapsulesDir:     c.Capsules,
		PluginsDir:      c.Plugins,
		PluginsExternal: c.PluginsExternal,
	}
	return api.Start(cfg)
}

type VersionCmd struct{}

func (c *VersionCmd) Run() error {
	fmt.Printf("capsule version %s\n", version)
	return nil
}

// Helper functions

func getPluginDir() string {
	// Check global CLI flag first
	if CLI.PluginDir != "" {
		absPath, err := filepath.Abs(CLI.PluginDir)
		if err == nil {
			return absPath
		}
		return CLI.PluginDir
	}

	// Default: look for plugins relative to executable
	exe, err := os.Executable()
	if err == nil {
		pluginDir := filepath.Join(filepath.Dir(exe), "plugins")
		if _, err := os.Stat(pluginDir); err == nil {
			return pluginDir
		}
	}
	// Fallback to current directory (use absolute path)
	absPlugins, err := filepath.Abs("plugins")
	if err == nil {
		return absPlugins
	}
	return "plugins"
}

func getFlakePath() string {
	// Look relative to executable
	exe, err := os.Executable()
	if err == nil {
		flakePath := filepath.Join(filepath.Dir(exe), "nix")
		if _, err := os.Stat(filepath.Join(flakePath, "flake.nix")); err == nil {
			return flakePath
		}
	}

	// Look in current working directory
	cwd, err := os.Getwd()
	if err == nil {
		flakePath := filepath.Join(cwd, "nix")
		if _, err := os.Stat(filepath.Join(flakePath, "flake.nix")); err == nil {
			return flakePath
		}
	}

	// Walk up from current directory
	if cwd != "" {
		for dir := cwd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
			flakePath := filepath.Join(dir, "nix")
			if _, err := os.Stat(filepath.Join(flakePath, "flake.nix")); err == nil {
				return flakePath
			}
		}
	}

	return ""
}

// runCapsuleTest runs selfcheck on a capsule and compares to golden hash.
func runCapsuleTest(capsulePath, goldenDir, name string) (bool, error) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tempDir)

	// Unpack capsule
	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		return false, fmt.Errorf("unpack failed: %w", err)
	}

	// Run identity-bytes plan on first artifact
	var artifactID string
	for id := range cap.Manifest.Artifacts {
		artifactID = id
		break
	}
	if artifactID == "" {
		return false, fmt.Errorf("no artifacts in capsule")
	}

	plan := selfcheck.IdentityBytesPlan(artifactID)
	executor := selfcheck.NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		return false, fmt.Errorf("selfcheck failed: %w", err)
	}

	// Check if golden file exists
	goldenPath := filepath.Join(goldenDir, name+".golden.json")
	if _, err := os.Stat(goldenPath); errors.Is(err, os.ErrNotExist) {
		// No golden file - create one
		reportJSON, _ := report.ToJSON()
		os.MkdirAll(goldenDir, 0755)
		os.WriteFile(goldenPath, reportJSON, 0644)
		fmt.Printf("  [NEW]  %s: created golden file\n", name)
		return true, nil
	}

	// Compare report hash to golden
	goldenData, err := os.ReadFile(goldenPath)
	if err != nil {
		return false, fmt.Errorf("failed to read golden: %w", err)
	}

	reportJSON, _ := report.ToJSON()
	reportHash := cas.Hash(reportJSON)
	goldenHash := cas.Hash(goldenData)

	return reportHash == goldenHash, nil
}

// runIngestTest ingests a file, runs selfcheck, and compares to golden.
func runIngestTest(inputPath, goldenDir, name string) (bool, error) {
	tempDir, err := os.MkdirTemp("", "capsule-ingest-test-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tempDir)

	// Create capsule
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		return false, fmt.Errorf("failed to create capsule: %w", err)
	}

	// Ingest file
	artifact, err := cap.IngestFile(inputPath)
	if err != nil {
		return false, fmt.Errorf("ingest failed: %w", err)
	}

	// Run identity-bytes plan
	plan := selfcheck.IdentityBytesPlan(artifact.ID)
	executor := selfcheck.NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		return false, fmt.Errorf("selfcheck failed: %w", err)
	}

	if report.Status != selfcheck.StatusPass {
		return false, fmt.Errorf("selfcheck did not pass")
	}

	// Check golden for artifact hash
	goldenPath := filepath.Join(goldenDir, name+".sha256")
	if _, err := os.Stat(goldenPath); errors.Is(err, os.ErrNotExist) {
		// Create golden
		os.MkdirAll(goldenDir, 0755)
		os.WriteFile(goldenPath, []byte(artifact.Hashes.SHA256+"\n"), 0644)
		fmt.Printf("  [NEW]  %s: created golden hash\n", name)
		return true, nil
	}

	// Compare hash
	goldenData, err := os.ReadFile(goldenPath)
	if err != nil {
		return false, err
	}

	expected := string(goldenData)
	expected = expected[:len(expected)-1] // Remove trailing newline

	return artifact.Hashes.SHA256 == expected, nil
}

// IRInfo holds summary information about an IR corpus.
type IRInfo struct {
	ID             string   `json:"id"`
	Version        string   `json:"version,omitempty"`
	ModuleType     string   `json:"module_type,omitempty"`
	Versification  string   `json:"versification,omitempty"`
	Language       string   `json:"language,omitempty"`
	Title          string   `json:"title,omitempty"`
	LossClass      string   `json:"loss_class,omitempty"`
	SourceHash     string   `json:"source_hash,omitempty"`
	DocumentCount  int      `json:"document_count"`
	Documents      []string `json:"documents,omitempty"`
	TotalBlocks    int      `json:"total_content_blocks"`
	TotalChars     int      `json:"total_characters"`
	HasAnnotations bool     `json:"has_annotations"`
}

func buildIRInfo(ir map[string]interface{}) *IRInfo {
	info := &IRInfo{}

	// Extract top-level fields
	if id, ok := ir["id"].(string); ok {
		info.ID = id
	}
	if ver, ok := ir["version"].(string); ok {
		info.Version = ver
	}
	if moduleType, ok := ir["module_type"].(string); ok {
		info.ModuleType = moduleType
	}
	if versification, ok := ir["versification"].(string); ok {
		info.Versification = versification
	}
	if language, ok := ir["language"].(string); ok {
		info.Language = language
	}
	if title, ok := ir["title"].(string); ok {
		info.Title = title
	}
	if lossClass, ok := ir["loss_class"].(string); ok {
		info.LossClass = lossClass
	}
	if sourceHash, ok := ir["source_hash"].(string); ok {
		info.SourceHash = sourceHash
	}

	// Count documents and content
	if docs, ok := ir["documents"].([]interface{}); ok {
		info.DocumentCount = len(docs)
		for _, doc := range docs {
			if docMap, ok := doc.(map[string]interface{}); ok {
				if docID, ok := docMap["id"].(string); ok {
					info.Documents = append(info.Documents, docID)
				}

				// Count content blocks
				if blocks, ok := docMap["content_blocks"].([]interface{}); ok {
					info.TotalBlocks += len(blocks)
					for _, block := range blocks {
						if blockMap, ok := block.(map[string]interface{}); ok {
							if text, ok := blockMap["text"].(string); ok {
								info.TotalChars += len(text)
							}
						}
					}
				}

				// Check for annotations
				if annotations, ok := docMap["annotations"].([]interface{}); ok && len(annotations) > 0 {
					info.HasAnnotations = true
				}
			}
		}
	}

	return info
}

func printIRInfo(info *IRInfo) {
	fmt.Println("IR Corpus Information")
	fmt.Println("=====================")
	fmt.Println()

	if info.ID != "" {
		fmt.Printf("  ID:             %s\n", info.ID)
	}
	if info.Title != "" {
		fmt.Printf("  Title:          %s\n", info.Title)
	}
	if info.Version != "" {
		fmt.Printf("  IR Version:     %s\n", info.Version)
	}
	if info.ModuleType != "" {
		fmt.Printf("  Module Type:    %s\n", info.ModuleType)
	}
	if info.Language != "" {
		fmt.Printf("  Language:       %s\n", info.Language)
	}
	if info.Versification != "" {
		fmt.Printf("  Versification:  %s\n", info.Versification)
	}
	if info.LossClass != "" {
		fmt.Printf("  Loss Class:     %s\n", info.LossClass)
	}
	if info.SourceHash != "" {
		fmt.Printf("  Source Hash:    %s\n", info.SourceHash[:16]+"...")
	}

	fmt.Println()
	fmt.Println("Content Summary")
	fmt.Println("---------------")
	fmt.Printf("  Documents:      %d\n", info.DocumentCount)
	fmt.Printf("  Content Blocks: %d\n", info.TotalBlocks)
	fmt.Printf("  Total Chars:    %d\n", info.TotalChars)
	fmt.Printf("  Annotations:    %v\n", info.HasAnnotations)

	if len(info.Documents) > 0 {
		fmt.Println()
		fmt.Println("Documents")
		fmt.Println("---------")
		for i, docID := range info.Documents {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(info.Documents)-10)
				break
			}
			fmt.Printf("  %d. %s\n", i+1, docID)
		}
	}
}

func main() {
	ctx := kong.Parse(&CLI,
		kong.Name("capsule"),
		kong.Description("Juniper Bible - Byte-for-byte preservation framework"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
	)
	err := ctx.Run(ctx)
	ctx.FatalIfErrorf(err)
}
