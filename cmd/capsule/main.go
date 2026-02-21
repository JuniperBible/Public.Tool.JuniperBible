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

	"github.com/JuniperBible/juniper/core/capsule"
	"github.com/JuniperBible/juniper/core/cas"
	"github.com/JuniperBible/juniper/core/docgen"
	"github.com/JuniperBible/juniper/core/ir"
	"github.com/JuniperBible/juniper/core/plugins"
	"github.com/JuniperBible/juniper/core/runner"
	"github.com/JuniperBible/juniper/core/selfcheck"
	"github.com/JuniperBible/juniper/internal/api"
	"github.com/JuniperBible/juniper/internal/archive"
	"github.com/JuniperBible/juniper/internal/fileutil"
	"github.com/JuniperBible/juniper/internal/juniper"
	"github.com/JuniperBible/juniper/internal/safefile"
	"github.com/JuniperBible/juniper/internal/validation"
	"github.com/JuniperBible/juniper/internal/web"

	// Import embedded plugins registry to register all embedded plugins
	_ "github.com/JuniperBible/juniper/internal/embedded"
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
	data, err := safefile.ReadFile(outputPath)
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
	tempDir, err := os.MkdirTemp("", "capsule-selfcheck-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	cap, err := capsule.Unpack(c.Capsule, tempDir)
	if err != nil {
		return fmt.Errorf("failed to unpack capsule: %w", err)
	}

	plan, err := resolveSelfcheckPlan(cap, c.Plan)
	if err != nil {
		return err
	}

	executor := selfcheck.NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		return fmt.Errorf("selfcheck execution failed: %w", err)
	}

	if err := printSelfcheckReport(report, c.JSON); err != nil {
		return err
	}

	if report.Status != selfcheck.StatusPass {
		return fmt.Errorf("selfcheck failed")
	}
	return nil
}

// resolveSelfcheckPlan determines which selfcheck plan to run based on the
// provided planID. When planID is empty it defaults to identity-bytes on the
// first artifact in the capsule.
func resolveSelfcheckPlan(cap *capsule.Capsule, planID string) (*selfcheck.Plan, error) {
	if planID == "" {
		return resolveDefaultPlan(cap)
	}
	if planID == "identity-bytes" {
		return resolveIdentityBytesPlan(cap)
	}
	return resolveNamedManifestPlan(cap, planID)
}

func resolveDefaultPlan(cap *capsule.Capsule) (*selfcheck.Plan, error) {
	for id := range cap.Manifest.Artifacts {
		return selfcheck.IdentityBytesPlan(id), nil
	}
	return nil, fmt.Errorf("no artifacts in capsule")
}

func resolveIdentityBytesPlan(cap *capsule.Capsule) (*selfcheck.Plan, error) {
	for id := range cap.Manifest.Artifacts {
		return selfcheck.IdentityBytesPlan(id), nil
	}
	return nil, fmt.Errorf("plan not found: identity-bytes")
}

func resolveNamedManifestPlan(cap *capsule.Capsule, planID string) (*selfcheck.Plan, error) {
	if cap.Manifest.RoundtripPlans == nil {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}
	p, ok := cap.Manifest.RoundtripPlans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}
	return convertManifestPlan(planID, p), nil
}

func convertManifestPlan(planID string, p *capsule.Plan) *selfcheck.Plan {
	plan := &selfcheck.Plan{
		ID:          planID,
		Description: p.Description,
	}
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
	return plan
}

// printSelfcheckReport writes the selfcheck report to stdout in the requested
// format (JSON or human-readable text).
func printSelfcheckReport(report *selfcheck.Report, jsonOutput bool) error {
	if jsonOutput {
		return printSelfcheckReportJSON(report)
	}
	printSelfcheckReportText(report)
	return nil
}

func printSelfcheckReportJSON(report *selfcheck.Report) error {
	data, err := report.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize report: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printSelfcheckReportText(report *selfcheck.Report) {
	fmt.Printf("Self-Check Report\n")
	fmt.Printf("  Plan: %s\n", report.PlanID)
	fmt.Printf("  Status: %s\n", report.Status)
	fmt.Printf("  Created: %s\n", report.CreatedAt)
	fmt.Println()
	for _, result := range report.Results {
		printSelfcheckResult(result)
	}
	fmt.Println()
	if report.Status == selfcheck.StatusPass {
		fmt.Println("All checks passed!")
	} else {
		fmt.Println("Some checks failed.")
	}
}

func printSelfcheckResult(result selfcheck.CheckResult) {
	status := "[PASS]"
	if !result.Pass {
		status = "[FAIL]"
	}
	fmt.Printf("  %s %s\n", status, result.Label)
	if result.Pass || result.Expected == nil || result.Actual == nil {
		return
	}
	fmt.Printf("    Expected: %s\n", result.Expected.SHA256)
	fmt.Printf("    Actual:   %s\n", result.Actual.SHA256)
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

func detectFormatPlugin(path string, formatPlugins []*plugins.Plugin) *plugins.Plugin {
	for _, p := range formatPlugins {
		resp, err := plugins.ExecutePlugin(p, plugins.NewDetectRequest(path))
		if err != nil {
			continue
		}
		result, err := plugins.ParseDetectResult(resp)
		if err != nil {
			continue
		}
		if result.Detected {
			return p
		}
	}
	return nil
}

func entryTypeStr(isDir bool) string {
	if isDir {
		return "D"
	}
	return "F"
}

func (c *EnumerateCmd) Run(ctx *kong.Context) error {
	path, err := filepath.Abs(c.Path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(getPluginDir()); err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	matchedPlugin := detectFormatPlugin(path, loader.GetPluginsByKind("format"))
	if matchedPlugin == nil {
		return fmt.Errorf("no matching format plugin found for: %s", path)
	}

	fmt.Printf("Enumerating: %s (using %s)\n\n", path, matchedPlugin.Manifest.PluginID)

	resp, err := plugins.ExecutePlugin(matchedPlugin, plugins.NewEnumerateRequest(path))
	if err != nil {
		return fmt.Errorf("enumerate failed: %w", err)
	}

	result, err := plugins.ParseEnumerateResult(resp)
	if err != nil {
		return fmt.Errorf("failed to parse result: %w", err)
	}

	for _, entry := range result.Entries {
		fmt.Printf("  [%s] %s (%d bytes)\n", entryTypeStr(entry.IsDir), entry.Path, entry.SizeBytes)
	}

	fmt.Printf("\nTotal: %d entries\n", len(result.Entries))
	return nil
}

// TestCmd runs tests against golden hashes.
type TestCmd struct {
	FixturesDir string `arg:"" help:"Path to fixtures directory" type:"existingdir"`
	Golden      string `help:"Path to golden hashes directory" type:"path"`
}

func resolveGoldenDir(fixturesDir, golden string) (string, error) {
	if golden == "" {
		return filepath.Join(fixturesDir, "goldens"), nil
	}
	abs, err := filepath.Abs(golden)
	if err != nil {
		return "", fmt.Errorf("failed to resolve golden directory path: %w", err)
	}
	return abs, nil
}

func recordTestResult(ok bool, err error, label string, passed, failed *int, failures *[]string) {
	if err != nil {
		fmt.Printf("  [FAIL] %s: %v\n", label, err)
		*failed++
		*failures = append(*failures, fmt.Sprintf("%s: %v", label, err))
		return
	}
	if ok {
		fmt.Printf("  [PASS] %s\n", label)
		*passed++
		return
	}
	fmt.Printf("  [FAIL] %s: hash mismatch\n", label)
	*failed++
	*failures = append(*failures, fmt.Sprintf("%s: hash mismatch", label))
}

func testCapsuleFiles(capsuleFiles []string, goldenDir string, passed, failed *int, failures *[]string) {
	for _, capsulePath := range capsuleFiles {
		name := filepath.Base(capsulePath)
		name = name[:len(name)-len(".capsule.tar.xz")]
		result, err := runCapsuleTest(capsulePath, goldenDir, name)
		recordTestResult(result, err, name, passed, failed, failures)
	}
}

func testInputFiles(inputFiles []string, goldenDir string, passed, failed *int, failures *[]string) {
	for _, inputPath := range inputFiles {
		info, err := os.Stat(inputPath)
		if err != nil || info.IsDir() {
			continue
		}
		name := filepath.Base(inputPath)
		ext := filepath.Ext(name)
		testName := name[:len(name)-len(ext)]
		result, err := runIngestTest(inputPath, goldenDir, testName)
		recordTestResult(result, err, testName+" (ingest)", passed, failed, failures)
	}
}

func (c *TestCmd) Run() error {
	fixturesDir, err := filepath.Abs(c.FixturesDir)
	if err != nil {
		return fmt.Errorf("invalid fixtures path: %w", err)
	}

	goldenDir, err := resolveGoldenDir(fixturesDir, c.Golden)
	if err != nil {
		return err
	}

	capsuleFiles, err := filepath.Glob(filepath.Join(fixturesDir, "*.capsule.tar.xz"))
	if err != nil {
		return fmt.Errorf("failed to find capsules: %w", err)
	}

	inputFiles, _ := filepath.Glob(filepath.Join(fixturesDir, "inputs", "*"))

	fmt.Printf("Capsule Test Runner\n")
	fmt.Printf("  Fixtures: %s\n", fixturesDir)
	fmt.Printf("  Goldens:  %s\n", goldenDir)
	fmt.Println()

	passed, failed := 0, 0
	var failures []string

	testCapsuleFiles(capsuleFiles, goldenDir, &passed, &failed, &failures)
	testInputFiles(inputFiles, goldenDir, &passed, &failed, &failures)

	fmt.Println()
	fmt.Printf("Results: %d passed, %d failed\n", passed, failed)

	if failed == 0 {
		return nil
	}

	fmt.Println("\nFailures:")
	for _, f := range failures {
		fmt.Printf("  - %s\n", f)
	}
	return fmt.Errorf("%d test(s) failed", failed)
}

// RunCmd runs a tool plugin with Nix executor.
type RunCmd struct {
	Tool    string `arg:"" help:"Tool plugin ID"`
	Profile string `arg:"" help:"Profile to run"`
	Input   string `help:"Input file path" type:"existingfile"`
	Out     string `help:"Output directory" type:"path"`
}

// resolveRunOutputDir resolves or creates the output directory for a run.
// When outDir is empty a temporary directory is created and a cleanup function
// that removes it is returned; otherwise the directory is created at the given
// absolute path and a no-op cleanup function is returned.
func resolveRunOutputDir(outDir string) (string, func(), error) {
	if outDir == "" {
		tmp, err := os.MkdirTemp("", "capsule-run-*")
		if err != nil {
			return "", nil, fmt.Errorf("failed to create temporary output directory: %w", err)
		}
		return tmp, func() { os.RemoveAll(tmp) }, nil
	}
	abs, err := filepath.Abs(outDir)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve output directory path: %w", err)
	}
	if err := os.MkdirAll(abs, 0700); err != nil {
		return "", nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	return abs, func() {}, nil
}

// printRunTranscript displays transcript events and writes the transcript file.
func printRunTranscript(result *runner.ExecutionResult, outDir string) {
	fmt.Printf("  Transcript hash: %s\n", result.TranscriptHash)

	events, err := runner.ParseNixTranscript(result.TranscriptData)
	if err == nil {
		fmt.Println("\nTranscript events:")
		for _, e := range events {
			eventJSON, _ := json.Marshal(e)
			fmt.Printf("  %s\n", eventJSON)
		}
	}

	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	if err := os.WriteFile(transcriptPath, result.TranscriptData, 0600); err == nil {
		fmt.Printf("\nTranscript written to: %s\n", transcriptPath)
	}
}

// printRunResult prints execution summary, transcript, stdout, and stderr.
func printRunResult(result *runner.ExecutionResult, outDir string) {
	fmt.Printf("Execution completed\n")
	fmt.Printf("  Exit code: %d\n", result.ExitCode)
	fmt.Printf("  Duration: %v\n", result.Duration)

	if len(result.TranscriptData) > 0 {
		printRunTranscript(result, outDir)
	}
	if len(result.Stdout) > 0 {
		fmt.Printf("\nStdout:\n%s\n", result.Stdout)
	}
	if len(result.Stderr) > 0 {
		fmt.Printf("\nStderr:\n%s\n", result.Stderr)
	}
}

func (c *RunCmd) Run() error {
	toolID := c.Tool
	profile := c.Profile
	inputPath := c.Input

	if inputPath != "" {
		abs, err := filepath.Abs(inputPath)
		if err != nil {
			return fmt.Errorf("failed to resolve input path: %w", err)
		}
		inputPath = abs
	}

	outDir, cleanup, err := resolveRunOutputDir(c.Out)
	if err != nil {
		return err
	}
	defer cleanup()

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

	req := runner.NewRequest(toolID, profile)
	var inputPaths []string
	if inputPath != "" {
		inputPaths = []string{inputPath}
		req.Inputs = inputPaths
	}

	executor := runner.NewNixExecutor(flakePath)
	result, err := executor.ExecuteRequest(context.Background(), req, inputPaths)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	printRunResult(result, outDir)

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

func toolRunExportArtifact(cap *capsule.Capsule, tempDir, artifactID string) (string, error) {
	artifact, ok := cap.Manifest.Artifacts[artifactID]
	if !ok {
		return "", fmt.Errorf("artifact not found: %s", artifactID)
	}
	inputPath := filepath.Join(tempDir, "input", artifact.OriginalName)
	if err := os.MkdirAll(filepath.Dir(inputPath), 0700); err != nil {
		return "", fmt.Errorf("failed to create input dir: %w", err)
	}
	if err := cap.Export(artifactID, capsule.ExportModeIdentity, inputPath); err != nil {
		return "", fmt.Errorf("failed to export artifact: %w", err)
	}
	return inputPath, nil
}

func toolRunExecute(toolID, profile, flakePath, inputPath string) (*runner.ExecutionResult, error) {
	req := runner.NewRequest(toolID, profile)
	req.Inputs = []string{inputPath}
	executor := runner.NewNixExecutor(flakePath)
	result, err := executor.ExecuteRequest(context.Background(), req, []string{inputPath})
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}
	fmt.Printf("Tool execution completed\n")
	fmt.Printf("  Exit code: %d\n", result.ExitCode)
	fmt.Printf("  Duration: %v\n", result.Duration)
	if len(result.TranscriptData) == 0 {
		return nil, fmt.Errorf("no transcript generated")
	}
	fmt.Printf("  Transcript hash: %s\n", result.TranscriptHash)
	return result, nil
}

func toolRunStoreResult(cap *capsule.Capsule, capsulePath, toolID, profile, artifactID string, result *runner.ExecutionResult) (string, error) {
	runID := fmt.Sprintf("run-%s-%s-%d", toolID, profile, len(cap.Manifest.Runs)+1)
	run := &capsule.Run{
		ID:     runID,
		Plugin: &capsule.PluginInfo{PluginID: toolID, Kind: "tool"},
		Inputs: []capsule.RunInput{{ArtifactID: artifactID}},
		Command: &capsule.Command{Profile: profile},
		Status: "completed",
	}
	if err := cap.AddRun(run, result.TranscriptData); err != nil {
		return "", fmt.Errorf("failed to add run: %w", err)
	}
	if err := cap.SaveManifest(); err != nil {
		return "", fmt.Errorf("failed to save manifest: %w", err)
	}
	if err := cap.Pack(capsulePath); err != nil {
		return "", fmt.Errorf("failed to repack capsule: %w", err)
	}
	return runID, nil
}

func toolRunPrintTranscript(transcriptData []byte) {
	events, err := runner.ParseNixTranscript(transcriptData)
	if err != nil || len(events) == 0 {
		return
	}
	fmt.Println("\nTranscript events:")
	for _, e := range events {
		eventJSON, _ := json.Marshal(e)
		fmt.Printf("  %s\n", eventJSON)
	}
}

func (c *ToolRunCmd) Run() error {
	tempDir, err := os.MkdirTemp("", "capsule-tool-run-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	cap, err := capsule.Unpack(c.Capsule, tempDir)
	if err != nil {
		return fmt.Errorf("failed to unpack capsule: %w", err)
	}

	fmt.Printf("Running tool on capsule artifact\n")
	fmt.Printf("  Capsule: %s\n", c.Capsule)
	fmt.Printf("  Artifact: %s\n", c.Artifact)
	fmt.Printf("  Tool: %s\n", c.Tool)
	fmt.Printf("  Profile: %s\n", c.Profile)
	fmt.Println()

	inputPath, err := toolRunExportArtifact(cap, tempDir, c.Artifact)
	if err != nil {
		return err
	}

	flakePath := getFlakePath()
	if flakePath == "" {
		return fmt.Errorf("nix flake not found")
	}

	result, err := toolRunExecute(c.Tool, c.Profile, flakePath, inputPath)
	if err != nil {
		return err
	}

	runID, err := toolRunStoreResult(cap, c.Capsule, c.Tool, c.Profile, c.Artifact, result)
	if err != nil {
		return err
	}

	fmt.Printf("\nRun stored: %s\n", runID)
	fmt.Printf("Capsule updated: %s\n", c.Capsule)

	toolRunPrintTranscript(result.TranscriptData)

	return nil
}

// CompareCmd compares transcripts between two runs.
type CompareCmd struct {
	Capsule string `arg:"" help:"Path to capsule" type:"existingfile"`
	Run1    string `arg:"" help:"First run ID"`
	Run2    string `arg:"" help:"Second run ID"`
}

func (c *CompareCmd) Run() error {
	cap, cleanup, err := compareUnpackAndLookup(c.Capsule, c.Run1, c.Run2)
	if err != nil {
		return err
	}
	defer cleanup()

	fmt.Printf("Comparing transcripts\n")
	fmt.Printf("  Capsule: %s\n", c.Capsule)
	fmt.Printf("  Run 1: %s\n", c.Run1)
	fmt.Printf("  Run 2: %s\n", c.Run2)
	fmt.Println()

	hash1, hash2, err := compareComputeHashes(cap, c.Run1, c.Run2)
	if err != nil {
		return err
	}

	if hash1 == hash2 {
		fmt.Println("Result: IDENTICAL")
		fmt.Println("  Transcripts are byte-for-byte identical.")
		return nil
	}

	fmt.Println("Result: DIFFERENT")
	fmt.Println("  Transcripts differ. Showing diff:")
	fmt.Println()

	return compareDiff(cap, c.Run1, c.Run2)
}

func compareUnpackAndLookup(capsulePath, run1ID, run2ID string) (*capsule.Capsule, func(), error) {
	tempDir, err := os.MkdirTemp("", "capsule-compare-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	cleanup := func() { os.RemoveAll(tempDir) }

	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to unpack capsule: %w", err)
	}

	if _, ok := cap.Manifest.Runs[run1ID]; !ok {
		cleanup()
		return nil, nil, fmt.Errorf("run not found: %s", run1ID)
	}
	if _, ok := cap.Manifest.Runs[run2ID]; !ok {
		cleanup()
		return nil, nil, fmt.Errorf("run not found: %s", run2ID)
	}

	return cap, cleanup, nil
}

func compareComputeHashes(cap *capsule.Capsule, run1ID, run2ID string) (string, string, error) {
	hash1, err := transcriptHash(cap.Manifest.Runs[run1ID], run1ID)
	if err != nil {
		return "", "", err
	}
	hash2, err := transcriptHash(cap.Manifest.Runs[run2ID], run2ID)
	if err != nil {
		return "", "", err
	}

	fmt.Printf("Transcript hashes:\n")
	fmt.Printf("  Run 1: %s\n", hash1)
	fmt.Printf("  Run 2: %s\n", hash2)
	fmt.Println()

	return hash1, hash2, nil
}

func compareDiff(cap *capsule.Capsule, run1ID, run2ID string) error {
	transcript1, err := cap.GetTranscript(run1ID)
	if err != nil {
		return fmt.Errorf("failed to get transcript 1: %w", err)
	}
	transcript2, err := cap.GetTranscript(run2ID)
	if err != nil {
		return fmt.Errorf("failed to get transcript 2: %w", err)
	}

	events1, err := runner.ParseNixTranscript(transcript1)
	if err != nil {
		return fmt.Errorf("failed to parse transcript 1: %w", err)
	}
	events2, err := runner.ParseNixTranscript(transcript2)
	if err != nil {
		return fmt.Errorf("failed to parse transcript 2: %w", err)
	}

	fmt.Printf("Event counts: Run 1=%d, Run 2=%d\n\n", len(events1), len(events2))
	printTranscriptDiff(events1, events2)

	return fmt.Errorf("transcripts differ")
}

// transcriptHash returns the transcript blob SHA-256 for the given run, or an
// error if no transcript is recorded.
func transcriptHash(run *capsule.Run, runID string) (string, error) {
	if run.Outputs != nil && run.Outputs.TranscriptBlobSHA256 != "" {
		return run.Outputs.TranscriptBlobSHA256, nil
	}
	return "", fmt.Errorf("run %s has no transcript", runID)
}

// printTranscriptDiff prints a line-by-line diff of two event slices to
// stdout. Events are compared by their JSON representation.
func printTranscriptDiff(events1, events2 []runner.NixTranscriptEvent) {
	maxLen := len(events1)
	if len(events2) > maxLen {
		maxLen = len(events2)
	}
	for i := 0; i < maxLen; i++ {
		e1 := marshalEventAt(events1, i)
		e2 := marshalEventAt(events2, i)
		if e1 != e2 {
			printEventDiff(i, e1, e2)
		}
	}
}

// marshalEventAt returns the JSON representation of events[i], or an empty
// string when i is out of range.
func marshalEventAt(events []runner.NixTranscriptEvent, i int) string {
	if i >= len(events) {
		return ""
	}
	data, _ := json.Marshal(events[i])
	return string(data)
}

// printEventDiff prints a single differing event pair at position i.
func printEventDiff(i int, e1, e2 string) {
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

// RunsListCmd lists all runs in a capsule.
type RunsListCmd struct {
	Capsule string `arg:"" help:"Path to capsule" type:"existingfile"`
}

func printRunInputs(inputs []capsule.RunInput) {
	if len(inputs) == 0 {
		return
	}
	ids := make([]string, len(inputs))
	for i, input := range inputs {
		ids[i] = input.ArtifactID
	}
	fmt.Printf("    Inputs: %s\n", strings.Join(ids, ", "))
}

func printRunEntry(id string, run *capsule.Run) {
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
	printRunInputs(run.Inputs)
	fmt.Println()
}

func (c *RunsListCmd) Run() error {
	tempDir, err := os.MkdirTemp("", "capsule-runs-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	cap, err := capsule.Unpack(c.Capsule, tempDir)
	if err != nil {
		return fmt.Errorf("failed to unpack capsule: %w", err)
	}

	fmt.Printf("Runs in capsule: %s\n\n", c.Capsule)

	if len(cap.Manifest.Runs) == 0 {
		fmt.Println("No runs recorded.")
		return nil
	}

	for id, run := range cap.Manifest.Runs {
		printRunEntry(id, run)
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
	if err := os.WriteFile(saveFile, []byte(goldenContent), 0600); err != nil {
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
	goldenData, err := safefile.ReadFile(checkFile)
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
	outputPath, _ := filepath.Abs(c.Out)
	plugin, err := loadFormatPlugin(c.Format)
	if err != nil {
		return err
	}
	c.printExtractHeader(inputPath, outputPath)
	result, err := executeExtractIR(plugin, inputPath)
	if err != nil {
		return err
	}
	if err := writeExtractOutput(result.IRPath, outputPath); err != nil {
		return err
	}
	c.printExtractResult(outputPath, result.LossClass)
	return nil
}

func (c *ExtractIRCmd) printExtractHeader(inputPath, outputPath string) {
	fmt.Printf("Extracting IR from: %s\n", inputPath)
	fmt.Printf("  Format: %s\n", c.Format)
	fmt.Printf("  Output: %s\n", outputPath)
	fmt.Println()
}

func (c *ExtractIRCmd) printExtractResult(outputPath, lossClass string) {
	fmt.Printf("IR extracted successfully\n")
	fmt.Printf("  Output: %s\n", outputPath)
	if lossClass != "" {
		fmt.Printf("  Loss class: %s\n", lossClass)
	}
}

func loadFormatPlugin(format string) (*plugins.Plugin, error) {
	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(getPluginDir()); err != nil {
		return nil, fmt.Errorf("failed to load plugins: %w", err)
	}
	pluginID := "format." + format
	plugin, err := loader.GetPlugin(pluginID)
	if err != nil {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}
	return plugin, nil
}

func executeExtractIR(plugin *plugins.Plugin, inputPath string) (*plugins.ExtractIRResult, error) {
	tempDir, err := os.MkdirTemp("", "capsule-extract-ir-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)
	req := plugins.NewExtractIRRequest(inputPath, tempDir)
	resp, err := plugins.ExecutePlugin(plugin, req)
	if err != nil {
		return nil, fmt.Errorf("extract-ir failed: %w", err)
	}
	return plugins.ParseExtractIRResult(resp)
}

func writeExtractOutput(irPath, outputPath string) error {
	irData, err := safefile.ReadFile(irPath)
	if err != nil {
		return fmt.Errorf("failed to read IR: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}
	return os.WriteFile(outputPath, irData, 0600)
}

// EmitNativeCmd emits native format from IR.
type EmitNativeCmd struct {
	IR     string `arg:"" help:"Path to IR JSON file" type:"existingfile"`
	Format string `required:"" help:"Target format (e.g., osis, html)"`
	Out    string `required:"" help:"Output path" type:"path"`
}

func (c *EmitNativeCmd) Run() error {
	irPath, _ := filepath.Abs(c.IR)
	outputPath, _ := filepath.Abs(c.Out)
	plugin, err := loadFormatPlugin(c.Format)
	if err != nil {
		return err
	}
	c.printEmitHeader(irPath, outputPath)
	result, err := executeEmitNative(plugin, irPath)
	if err != nil {
		return err
	}
	if err := writeEmitOutput(result.OutputPath, outputPath); err != nil {
		return err
	}
	c.printEmitResult(outputPath, result.LossClass)
	return nil
}

func (c *EmitNativeCmd) printEmitHeader(irPath, outputPath string) {
	fmt.Printf("Emitting native format from IR: %s\n", irPath)
	fmt.Printf("  Format: %s\n", c.Format)
	fmt.Printf("  Output: %s\n", outputPath)
	fmt.Println()
}

func (c *EmitNativeCmd) printEmitResult(outputPath, lossClass string) {
	fmt.Printf("Native format emitted successfully\n")
	fmt.Printf("  Output: %s\n", outputPath)
	if lossClass != "" {
		fmt.Printf("  Loss class: %s\n", lossClass)
	}
}

func executeEmitNative(plugin *plugins.Plugin, irPath string) (*plugins.EmitNativeResult, error) {
	tempDir, err := os.MkdirTemp("", "capsule-emit-native-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)
	req := plugins.NewEmitNativeRequest(irPath, tempDir)
	resp, err := plugins.ExecutePlugin(plugin, req)
	if err != nil {
		return nil, fmt.Errorf("emit-native failed: %w", err)
	}
	return plugins.ParseEmitNativeResult(resp)
}

func writeEmitOutput(sourcePath, outputPath string) error {
	outputData, err := safefile.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read output: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}
	return os.WriteFile(outputPath, outputData, 0600)
}

// ConvertCmd converts file to different format via IR.
type ConvertCmd struct {
	Path string `arg:"" help:"Path to input file" type:"existingfile"`
	To   string `required:"" help:"Target format"`
	Out  string `required:"" help:"Output path" type:"path"`
}

// detectSourcePlugin iterates format plugins and returns the first that detects
// the given file, together with its short format name.
func detectSourcePlugin(loader *plugins.Loader, inputPath string) (*plugins.Plugin, string) {
	for _, p := range loader.GetPluginsByKind("format") {
		req := plugins.NewDetectRequest(inputPath)
		resp, err := plugins.ExecutePlugin(p, req)
		if err != nil {
			continue
		}
		result, err := plugins.ParseDetectResult(resp)
		if err != nil || !result.Detected {
			continue
		}
		return p, strings.TrimPrefix(p.Manifest.PluginID, "format.")
	}
	return nil, ""
}

// convertExtractIR runs the extract-ir step and returns the extract result.
func convertExtractIR(sourcePlugin *plugins.Plugin, inputPath, irDir string) (*plugins.ExtractIRResult, error) {
	fmt.Println("Step 1: Extracting IR from source...")
	os.MkdirAll(irDir, 0700)

	extractResp, err := plugins.ExecutePlugin(sourcePlugin, plugins.NewExtractIRRequest(inputPath, irDir))
	if err != nil {
		return nil, fmt.Errorf("extract-ir failed: %w", err)
	}
	extractResult, err := plugins.ParseExtractIRResult(extractResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse extract-ir result: %w", err)
	}

	fmt.Printf("  IR path: %s\n", extractResult.IRPath)
	if extractResult.LossClass != "" {
		fmt.Printf("  Loss class: %s\n", extractResult.LossClass)
	}
	return extractResult, nil
}

// convertEmitNative runs the emit-native step and returns the emit result.
func convertEmitNative(targetPlugin *plugins.Plugin, irPath, emitDir string) (*plugins.EmitNativeResult, error) {
	fmt.Println("\nStep 2: Emitting native format...")
	os.MkdirAll(emitDir, 0700)

	emitResp, err := plugins.ExecutePlugin(targetPlugin, plugins.NewEmitNativeRequest(irPath, emitDir))
	if err != nil {
		return nil, fmt.Errorf("emit-native failed: %w", err)
	}
	emitResult, err := plugins.ParseEmitNativeResult(emitResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse emit-native result: %w", err)
	}

	fmt.Printf("  Output path: %s\n", emitResult.OutputPath)
	if emitResult.LossClass != "" {
		fmt.Printf("  Loss class: %s\n", emitResult.LossClass)
	}
	return emitResult, nil
}

func (c *ConvertCmd) Run() error {
	inputPath, _ := filepath.Abs(c.Path)
	outputPath, _ := filepath.Abs(c.Out)

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(getPluginDir()); err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	printConvertHeader(inputPath, c.To, outputPath)

	sourcePlugin, sourceFormat, err := resolveSourcePlugin(loader, inputPath)
	if err != nil {
		return err
	}

	targetPlugin, err := resolveTargetPlugin(loader, c.To)
	if err != nil {
		return err
	}

	if err := executeConversion(sourcePlugin, targetPlugin, inputPath, outputPath); err != nil {
		return err
	}

	printConvertSuccess(inputPath, sourceFormat, outputPath, c.To)
	return nil
}

func printConvertHeader(inputPath, toFormat, outputPath string) {
	fmt.Printf("Converting: %s\n", inputPath)
	fmt.Printf("  To format: %s\n", toFormat)
	fmt.Printf("  Output: %s\n", outputPath)
	fmt.Println()
}

func resolveSourcePlugin(loader *plugins.Loader, inputPath string) (*plugins.Plugin, string, error) {
	sourcePlugin, sourceFormat := detectSourcePlugin(loader, inputPath)
	if sourcePlugin == nil {
		return nil, "", fmt.Errorf("could not detect source format")
	}
	fmt.Printf("Detected source format: %s\n", sourceFormat)
	fmt.Println()
	return sourcePlugin, sourceFormat, nil
}

func resolveTargetPlugin(loader *plugins.Loader, toFormat string) (*plugins.Plugin, error) {
	targetPluginID := "format." + toFormat
	targetPlugin, err := loader.GetPlugin(targetPluginID)
	if err != nil {
		return nil, fmt.Errorf("target plugin not found: %s", targetPluginID)
	}
	return targetPlugin, nil
}

func executeConversion(sourcePlugin, targetPlugin *plugins.Plugin, inputPath, outputPath string) error {
	tempDir, err := os.MkdirTemp("", "capsule-convert-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	extractResult, err := convertExtractIR(sourcePlugin, inputPath, filepath.Join(tempDir, "ir"))
	if err != nil {
		return err
	}

	emitResult, err := convertEmitNative(targetPlugin, extractResult.IRPath, filepath.Join(tempDir, "output"))
	if err != nil {
		return err
	}

	return copyConvertOutput(emitResult.OutputPath, outputPath)
}

func copyConvertOutput(srcPath, dstPath string) error {
	outputData, err := safefile.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read output: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0700); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}
	return os.WriteFile(dstPath, outputData, 0600)
}

func printConvertSuccess(inputPath, sourceFormat, outputPath, toFormat string) {
	fmt.Println()
	fmt.Printf("Conversion complete!\n")
	fmt.Printf("  Input: %s (%s)\n", inputPath, sourceFormat)
	fmt.Printf("  Output: %s (%s)\n", outputPath, toFormat)
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
	data, err := safefile.ReadFile(irPath)
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

type docgenFunc func(*docgen.Generator, string) error

var docgenHandlers = map[string]docgenFunc{
	"plugins": generatePluginsDocs,
	"formats": generateFormatsDocs,
	"cli":     generateCLIDocs,
	"all":     generateAllDocs,
}

func (c *DocgenCmd) Run() error {
	outputDir, _ := filepath.Abs(c.Output)
	gen := docgen.NewGenerator(getPluginDir(), outputDir)

	if handler, ok := docgenHandlers[c.Subcommand]; ok {
		return handler(gen, outputDir)
	}
	return nil
}

func generatePluginsDocs(gen *docgen.Generator, outputDir string) error {
	fmt.Printf("Generating PLUGINS.md...\n")
	if err := gen.GeneratePlugins(); err != nil {
		return err
	}
	fmt.Printf("Generated: %s/PLUGINS.md\n", outputDir)
	return nil
}

func generateFormatsDocs(gen *docgen.Generator, outputDir string) error {
	fmt.Printf("Generating FORMATS.md...\n")
	if err := gen.GenerateFormats(); err != nil {
		return err
	}
	fmt.Printf("Generated: %s/FORMATS.md\n", outputDir)
	return nil
}

func generateCLIDocs(gen *docgen.Generator, outputDir string) error {
	fmt.Printf("Generating CLI_REFERENCE.md...\n")
	if err := gen.GenerateCLI(); err != nil {
		return err
	}
	fmt.Printf("Generated: %s/CLI_REFERENCE.md\n", outputDir)
	return nil
}

func generateAllDocs(gen *docgen.Generator, outputDir string) error {
	fmt.Printf("Generating all documentation...\n  Output dir: %s\n\n", outputDir)
	if err := gen.GenerateAll(); err != nil {
		return err
	}
	fmt.Printf("Generated:\n  - %s/PLUGINS.md\n  - %s/FORMATS.md\n  - %s/CLI_REFERENCE.md\n", outputDir, outputDir, outputDir)
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

func truncateDesc(desc string) string {
	if len(desc) > 40 {
		return desc[:37] + "..."
	}
	return desc
}

func encryptedSuffix(encrypted bool) string {
	if encrypted {
		return " [encrypted]"
	}
	return ""
}

func printBibleModules(modsDir string) (int, error) {
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read mods.d: %w", err)
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}
		module := parseConfForList(filepath.Join(modsDir, e.Name()))
		if module == nil || module.modType != "Bible" {
			continue
		}
		fmt.Printf("%-15s %-8s %-40s%s\n", module.name, module.lang, truncateDesc(module.description), encryptedSuffix(module.encrypted))
		count++
	}
	return count, nil
}

func (c *JuniperListCmd) Run() error {
	swordPath, err := resolveSwordPath(c.Path)
	if err != nil {
		return err
	}

	modsDir := filepath.Join(swordPath, "mods.d")
	if _, err := os.Stat(modsDir); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("SWORD installation not found at %s (missing mods.d)", swordPath)
	}

	fmt.Printf("Bible modules in %s:\n\n", swordPath)
	fmt.Printf("%-15s %-8s %-40s\n", "MODULE", "LANG", "DESCRIPTION")
	fmt.Printf("%-15s %-8s %-40s\n", "------", "----", "-----------")

	count, err := printBibleModules(modsDir)
	if err != nil {
		return err
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

func resolveSwordPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".sword"), nil
}

func loadBibleModules(swordPath string) ([]*juniperModule, error) {
	modsDir := filepath.Join(swordPath, "mods.d")
	entries, err := readModsDir(modsDir, swordPath)
	if err != nil {
		return nil, err
	}
	modules := filterBibleModules(modsDir, entries)
	if len(modules) == 0 {
		return nil, fmt.Errorf("no Bible modules found in %s", swordPath)
	}
	return modules, nil
}

func readModsDir(modsDir, swordPath string) ([]os.DirEntry, error) {
	if _, err := os.Stat(modsDir); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("SWORD installation not found at %s", swordPath)
	}
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read mods.d: %w", err)
	}
	return entries, nil
}

func filterBibleModules(modsDir string, entries []os.DirEntry) []*juniperModule {
	var modules []*juniperModule
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}
		confPath := filepath.Join(modsDir, e.Name())
		m := parseConfForList(confPath)
		if m != nil && m.modType == "Bible" {
			m.confPath = confPath
			modules = append(modules, m)
		}
	}
	return modules
}

func selectModulesToIngest(all bool, names []string, modules []*juniperModule) ([]*juniperModule, error) {
	if all {
		return modules, nil
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("specify module names or use --all")
	}
	moduleMap := make(map[string]*juniperModule, len(modules))
	for _, m := range modules {
		moduleMap[m.name] = m
	}
	var selected []*juniperModule
	for _, name := range names {
		if m, ok := moduleMap[name]; ok {
			selected = append(selected, m)
		} else {
			fmt.Printf("Warning: module '%s' not found\n", name)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no modules to ingest")
	}
	return selected, nil
}

func ingestModules(swordPath, output string, toIngest []*juniperModule) {
	for _, m := range toIngest {
		if m.encrypted {
			fmt.Printf("Skipping %s (encrypted)\n", m.name)
			continue
		}
		capsulePath := filepath.Join(output, m.name+".capsule.tar.gz")
		fmt.Printf("Creating %s...\n", capsulePath)
		if err := ingestSwordModule(swordPath, m, capsulePath); err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}
		if info, _ := os.Stat(capsulePath); info != nil {
			fmt.Printf("  Created: %s (%d bytes)\n", capsulePath, info.Size())
		}
	}
}

func (c *JuniperIngestCmd) Run() error {
	swordPath, err := resolveSwordPath(c.Path)
	if err != nil {
		return err
	}
	modules, err := loadBibleModules(swordPath)
	if err != nil {
		return err
	}
	toIngest, err := selectModulesToIngest(c.All, c.Modules, modules)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.Output, 0700); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	fmt.Printf("Ingesting %d module(s) to %s/\n\n", len(toIngest), c.Output)
	ingestModules(swordPath, c.Output, toIngest)
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

func selectModulesToInstall(all bool, names []string, modules []*juniperModule) ([]*juniperModule, error) {
	if all {
		return modules, nil
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("specify module names or use --all")
	}
	moduleMap := make(map[string]*juniperModule, len(modules))
	for _, m := range modules {
		moduleMap[m.name] = m
	}
	var selected []*juniperModule
	for _, name := range names {
		if m, ok := moduleMap[name]; ok {
			selected = append(selected, m)
		} else {
			fmt.Printf("Warning: module '%s' not found\n", name)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no modules to install")
	}
	return selected, nil
}

func installOneModule(swordPath string, m *juniperModule, output string) bool {
	if m.encrypted {
		fmt.Printf("Skipping %s (encrypted)\n", m.name)
		return false
	}
	capsulePath := filepath.Join(output, m.name+".capsule.tar.gz")
	fmt.Printf("Installing %s...\n", m.name)
	fmt.Printf("  Ingesting SWORD module...\n")
	if err := ingestSwordModule(swordPath, m, capsulePath); err != nil {
		fmt.Printf("  Error during ingest: %v\n", err)
		return false
	}
	fmt.Printf("  Generating IR...\n")
	irCmd := &GenerateIRCmd{Capsule: capsulePath}
	if err := irCmd.Run(); err != nil {
		fmt.Printf("  Error during IR generation: %v\n", err)
		fmt.Printf("  (Capsule created but without IR)\n")
		return false
	}
	info, _ := os.Stat(capsulePath)
	if info != nil {
		fmt.Printf("  Done: %s (%d bytes)\n", capsulePath, info.Size())
	}
	return true
}

func (c *JuniperInstallCmd) Run() error {
	swordPath, err := resolveSwordPath(c.Path)
	if err != nil {
		return err
	}
	modules, err := loadBibleModules(swordPath)
	if err != nil {
		return err
	}
	toInstall, err := selectModulesToInstall(c.All, c.Modules, modules)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.Output, 0700); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	fmt.Printf("Installing %d module(s) to %s/ (with IR generation)\n\n", len(toInstall), c.Output)
	successful := 0
	for _, m := range toInstall {
		if installOneModule(swordPath, m, c.Output) {
			successful++
		}
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

// modDrvToType maps a SWORD ModDrv value to a human-readable module type.
func modDrvToType(drv string) string {
	switch drv {
	case "zText", "RawText", "zText4", "RawText4":
		return "Bible"
	case "zCom", "RawCom", "zCom4", "RawCom4":
		return "Commentary"
	case "zLD", "RawLD", "RawLD4":
		return "Dictionary"
	case "RawGenBook":
		return "GenBook"
	default:
		return "Unknown"
	}
}

// applyConfKeyValue sets the appropriate field on module for the given conf
// key=value pair.
func applyConfKeyValue(key, value string, module *juniperModule) {
	switch key {
	case "Description":
		module.description = value
	case "Lang":
		module.lang = value
	case "ModDrv":
		module.modType = modDrvToType(value)
	case "DataPath":
		module.dataPath = value
	case "CipherKey":
		module.encrypted = value != ""
	}
}

// parseConfForList parses a SWORD conf file for listing.
func parseConfForList(path string) *juniperModule {
	data, err := safefile.ReadFile(path)
	if err != nil {
		return nil
	}

	module := &juniperModule{}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || line[0] == '#' {
			continue
		}
		if line[0] == '[' && line[len(line)-1] == ']' {
			module.name = line[1 : len(line)-1]
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		applyConfKeyValue(
			strings.TrimSpace(line[:idx]),
			strings.TrimSpace(line[idx+1:]),
			module,
		)
	}
	return module
}

// resolveModuleDataPath validates and resolves the module data path.
func resolveModuleDataPath(module *juniperModule, swordPath string) (string, string, error) {
	if module.dataPath == "" {
		return "", "", fmt.Errorf("no DataPath in conf file")
	}
	dataPath := strings.TrimPrefix(module.dataPath, "./")
	fullDataPath := filepath.Join(swordPath, dataPath)
	if _, err := os.Stat(fullDataPath); errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("data path not found: %s", fullDataPath)
	}
	return dataPath, fullDataPath, nil
}

// populateCapsuleDir creates the capsule directory structure and copies module files.
func populateCapsuleDir(capsuleDir string, module *juniperModule, confData []byte, dataPath, fullDataPath string) error {
	modsDir := filepath.Join(capsuleDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		return fmt.Errorf("failed to create mods.d: %w", err)
	}
	confName := strings.ToLower(module.name) + ".conf"
	if err := os.WriteFile(filepath.Join(modsDir, confName), confData, 0600); err != nil {
		return fmt.Errorf("failed to write conf: %w", err)
	}
	destDataPath := filepath.Join(capsuleDir, dataPath)
	if err := os.MkdirAll(filepath.Dir(destDataPath), 0700); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}
	if err := fileutil.CopyDir(fullDataPath, destDataPath); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}
	return nil
}

// writeSwordManifest marshals and writes the capsule manifest.json file.
func writeSwordManifest(capsuleDir string, module *juniperModule) error {
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
	if err := os.WriteFile(filepath.Join(capsuleDir, "manifest.json"), manifestData, 0600); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}
	return nil
}

// ingestSwordModule creates a capsule from a SWORD module.
func ingestSwordModule(swordPath string, module *juniperModule, outputPath string) error {
	confData, err := safefile.ReadFile(module.confPath)
	if err != nil {
		return fmt.Errorf("failed to read conf: %w", err)
	}
	dataPath, fullDataPath, err := resolveModuleDataPath(module, swordPath)
	if err != nil {
		return err
	}
	tempDir, err := os.MkdirTemp("", "sword-capsule-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)
	capsuleDir := filepath.Join(tempDir, "capsule")
	if err := populateCapsuleDir(capsuleDir, module, confData, dataPath, fullDataPath); err != nil {
		return err
	}
	if err := writeSwordManifest(capsuleDir, module); err != nil {
		return err
	}
	return archive.CreateCapsuleTarGz(capsuleDir, outputPath)
}

// JuniperCASToSwordCmd converts a CAS capsule to a SWORD module.
type JuniperCASToSwordCmd struct {
	Capsule string `arg:"" help:"Path to CAS capsule to convert" type:"existingfile"`
	Output  string `short:"o" help:"Output directory for SWORD module (default: .sword in home)"`
	Name    string `short:"n" help:"Module name (default: derived from capsule)"`
}

func resolveOutputDir(output string) (string, error) {
	if output != "" {
		return output, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".sword"), nil
}

func deriveModuleName(capsulePath, name string) string {
	if name != "" {
		return name
	}
	base := filepath.Base(capsulePath)
	for _, suffix := range []string{".capsule.tar.xz", ".capsule.tar.gz", ".tar.xz", ".tar.gz"} {
		base = strings.TrimSuffix(base, suffix)
	}
	return strings.ToUpper(base)
}

func firstIRRecord(extractions map[string]*capsule.IRRecord) *capsule.IRRecord {
	for _, rec := range extractions {
		return rec
	}
	return nil
}

func corpusDefaults(corpus ir.Corpus, moduleName string) (lang, description, versification string) {
	lang = corpus.Language
	if lang == "" {
		lang = "en"
	}
	description = corpus.Title
	if description == "" {
		description = moduleName + " Bible Module"
	}
	versification = corpus.Versification
	if versification == "" {
		versification = "KJV"
	}
	return lang, description, versification
}

func createSwordStructure(outputDir, moduleName, lang, description, versification string) error {
	modsDir := filepath.Join(outputDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		return fmt.Errorf("failed to create mods.d: %w", err)
	}
	dataPath := filepath.Join("modules", "texts", "ztext", strings.ToLower(moduleName))
	fullDataPath := filepath.Join(outputDir, dataPath)
	if err := os.MkdirAll(fullDataPath, 0700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
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
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
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

func (c *JuniperCASToSwordCmd) Run() error {
	capsulePath, _ := filepath.Abs(c.Capsule)

	outputDir, err := resolveOutputDir(c.Output)
	if err != nil {
		return err
	}
	moduleName := deriveModuleName(capsulePath, c.Name)

	fmt.Printf("Converting CAS capsule to SWORD module:\n")
	fmt.Printf("  Input:  %s\n", capsulePath)
	fmt.Printf("  Output: %s\n", outputDir)
	fmt.Printf("  Module: %s\n", moduleName)
	fmt.Println()

	tempDir, err := os.MkdirTemp("", "cas-to-sword-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		return fmt.Errorf("failed to unpack capsule: %w", err)
	}

	if len(cap.Manifest.IRExtractions) == 0 {
		return fmt.Errorf("capsule has no IR - run 'capsule format ir generate' first")
	}

	irRecord := firstIRRecord(cap.Manifest.IRExtractions)

	irBlobData, err := cap.GetStore().Retrieve(irRecord.IRBlobSHA256)
	if err != nil {
		return fmt.Errorf("failed to retrieve IR blob: %w", err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(irBlobData, &corpus); err != nil {
		return fmt.Errorf("failed to parse IR corpus: %w", err)
	}

	lang, description, versification := corpusDefaults(corpus, moduleName)

	fmt.Printf("  IR ID:       %s\n", corpus.ID)
	fmt.Printf("  Language:    %s\n", lang)
	fmt.Printf("  Title:       %s\n", description)
	fmt.Printf("  Versification: %s\n", versification)
	fmt.Printf("  Documents:   %d\n", len(corpus.Documents))
	fmt.Println()

	return createSwordStructure(outputDir, moduleName, lang, description, versification)
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

func capsuleConvertExtractIR(loader *plugins.Loader, sourceFormat, contentPath, irDir string) (*plugins.ExtractIRResult, error) {
	sourcePlugin, err := loader.GetPlugin("format." + sourceFormat)
	if err != nil {
		return nil, fmt.Errorf("no plugin for source format '%s': %w", sourceFormat, err)
	}
	extractResp, err := plugins.ExecutePlugin(sourcePlugin, plugins.NewExtractIRRequest(contentPath, irDir))
	if err != nil {
		return nil, fmt.Errorf("IR extraction failed: %w", err)
	}
	result, err := plugins.ParseExtractIRResult(extractResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse extract result: %w", err)
	}
	return result, nil
}

func capsuleConvertEmitFormat(loader *plugins.Loader, targetFormat, irPath, emitDir string) (*plugins.EmitNativeResult, error) {
	targetPlugin, err := loader.GetPlugin("format." + targetFormat)
	if err != nil {
		return nil, fmt.Errorf("no plugin for target format '%s': %w", targetFormat, err)
	}
	emitResp, err := plugins.ExecutePlugin(targetPlugin, plugins.NewEmitNativeRequest(irPath, emitDir))
	if err != nil {
		return nil, fmt.Errorf("emit failed: %w", err)
	}
	result, err := plugins.ParseEmitNativeResult(emitResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse emit result: %w", err)
	}
	return result, nil
}

func capsuleConvertBuildDir(newCapsuleDir, capsulePath, sourceFormat, targetFormat string, extractResult *plugins.ExtractIRResult, emitResult *plugins.EmitNativeResult) error {
	outputData, err := safefile.ReadFile(emitResult.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to read output: %w", err)
	}
	os.WriteFile(filepath.Join(newCapsuleDir, filepath.Base(emitResult.OutputPath)), outputData, 0600)
	irData, _ := safefile.ReadFile(extractResult.IRPath)
	baseName := filepath.Base(capsulePath)
	for _, suffix := range []string{".capsule.tar.gz", ".capsule.tar.xz", ".tar.gz", ".tar.xz"} {
		baseName = strings.TrimSuffix(baseName, suffix)
	}
	os.WriteFile(filepath.Join(newCapsuleDir, baseName+".ir.json"), irData, 0600)
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
	os.WriteFile(filepath.Join(newCapsuleDir, "manifest.json"), manifestData, 0600)
	return nil
}

func capsuleConvertFinalize(newCapsuleDir, capsulePath string) (string, error) {
	oldPath := renameCapsuleToOld(capsulePath)
	if oldPath == "" {
		return "", fmt.Errorf("failed to rename original capsule")
	}
	if err := archive.CreateCapsuleTarGz(newCapsuleDir, capsulePath); err != nil {
		os.Rename(oldPath, capsulePath)
		return "", fmt.Errorf("failed to create capsule: %w", err)
	}
	return oldPath, nil
}

func (c *CapsuleConvertCmd) Run() error {
	capsulePath, _ := filepath.Abs(c.Capsule)
	targetFormat := c.Format

	fmt.Printf("Converting capsule: %s\n", capsulePath)
	fmt.Printf("Target format: %s\n", targetFormat)
	fmt.Println()

	tempDir, err := os.MkdirTemp("", "capsule-convert-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	return runCapsuleConversion(capsulePath, targetFormat, tempDir)
}

// runCapsuleConversion performs the actual capsule conversion steps.
func runCapsuleConversion(capsulePath, targetFormat, tempDir string) error {
	extractDir := filepath.Join(tempDir, "extract")
	if err := extractCapsuleArchive(capsulePath, extractDir); err != nil {
		return fmt.Errorf("failed to extract capsule: %w", err)
	}

	contentPath, sourceFormat := findConvertibleContent(extractDir)
	if contentPath == "" {
		return fmt.Errorf("no convertible content found in capsule (supported: OSIS, USFM, USX, JSON, SWORD)")
	}

	fmt.Printf("Detected source format: %s\n", sourceFormat)
	fmt.Printf("Content file: %s\n", filepath.Base(contentPath))
	fmt.Println()

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(getPluginDir()); err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	extractResult, emitResult, err := runConversionPipeline(loader, sourceFormat, targetFormat, contentPath, tempDir)
	if err != nil {
		return err
	}

	return finalizeConversion(capsulePath, sourceFormat, targetFormat, tempDir, extractResult, emitResult)
}

// runConversionPipeline runs IR extraction and format emission.
func runConversionPipeline(loader *plugins.Loader, sourceFormat, targetFormat, contentPath, tempDir string) (*plugins.ExtractIRResult, *plugins.EmitNativeResult, error) {
	fmt.Println("Step 1: Extracting IR...")
	irDir := filepath.Join(tempDir, "ir")
	os.MkdirAll(irDir, 0700)
	extractResult, err := capsuleConvertExtractIR(loader, sourceFormat, contentPath, irDir)
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("  IR extracted (loss class: %s)\n", extractResult.LossClass)

	fmt.Printf("Step 2: Emitting %s...\n", targetFormat)
	emitDir := filepath.Join(tempDir, "output")
	os.MkdirAll(emitDir, 0700)
	emitResult, err := capsuleConvertEmitFormat(loader, targetFormat, extractResult.IRPath, emitDir)
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("  Output generated (loss class: %s)\n", emitResult.LossClass)

	return extractResult, emitResult, nil
}

// finalizeConversion creates the final capsule and backs up the original.
func finalizeConversion(capsulePath, sourceFormat, targetFormat, tempDir string, extractResult *plugins.ExtractIRResult, emitResult *plugins.EmitNativeResult) error {
	fmt.Println("Step 3: Creating new capsule...")
	newCapsuleDir := filepath.Join(tempDir, "new-capsule")
	os.MkdirAll(newCapsuleDir, 0700)
	if err := capsuleConvertBuildDir(newCapsuleDir, capsulePath, sourceFormat, targetFormat, extractResult, emitResult); err != nil {
		return err
	}

	fmt.Println("Step 4: Finalizing...")
	oldPath, err := capsuleConvertFinalize(newCapsuleDir, capsulePath)
	if err != nil {
		return err
	}
	fmt.Printf("  Original backed up: %s\n", filepath.Base(oldPath))

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

func generateIRPrepareExtract(capsulePath string) (tempDir, extractDir, contentPath, sourceFormat string, err error) {
	tempDir, err = os.MkdirTemp("", "capsule-ir-*")
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	extractDir = filepath.Join(tempDir, "extract")
	if err = extractCapsuleArchive(capsulePath, extractDir); err != nil {
		os.RemoveAll(tempDir)
		return "", "", "", "", fmt.Errorf("failed to extract capsule: %w", err)
	}
	contentPath, sourceFormat = findConvertibleContent(extractDir)
	if contentPath == "" {
		os.RemoveAll(tempDir)
		return "", "", "", "", fmt.Errorf("no convertible content found (supported: OSIS, USFM, USX, JSON, SWORD)")
	}
	return tempDir, extractDir, contentPath, sourceFormat, nil
}

func generateIRRunExtraction(pluginDir, sourceFormat, contentPath, irDir string) (*plugins.ExtractIRResult, error) {
	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		return nil, fmt.Errorf("failed to load plugins: %w", err)
	}
	sourcePlugin, err := loader.GetPlugin("format." + sourceFormat)
	if err != nil {
		return nil, fmt.Errorf("no plugin for format '%s': %w", sourceFormat, err)
	}
	extractResp, err := plugins.ExecutePlugin(sourcePlugin, plugins.NewExtractIRRequest(contentPath, irDir))
	if err != nil {
		return nil, fmt.Errorf("IR extraction failed: %w", err)
	}
	extractResult, err := plugins.ParseExtractIRResult(extractResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse result: %w", err)
	}
	return extractResult, nil
}

func generateIRBuildAndFinalize(extractDir, newCapsuleDir, capsulePath, sourceFormat string, extractResult *plugins.ExtractIRResult) (string, error) {
	if err := fileutil.CopyDir(extractDir, newCapsuleDir); err != nil {
		return "", fmt.Errorf("failed to copy contents: %w", err)
	}
	irData, err := safefile.ReadFile(extractResult.IRPath)
	if err != nil {
		return "", fmt.Errorf("failed to read IR: %w", err)
	}
	baseName := filepath.Base(capsulePath)
	for _, suffix := range []string{".capsule.tar.gz", ".capsule.tar.xz", ".tar.gz", ".tar.xz"} {
		baseName = strings.TrimSuffix(baseName, suffix)
	}
	os.WriteFile(filepath.Join(newCapsuleDir, baseName+".ir.json"), irData, 0600)
	manifestPath := filepath.Join(newCapsuleDir, "manifest.json")
	manifest := make(map[string]interface{})
	if data, readErr := safefile.ReadFile(manifestPath); readErr == nil {
		json.Unmarshal(data, &manifest)
	}
	manifest["has_ir"] = true
	manifest["ir_generated"] = time.Now().Format(time.RFC3339)
	manifest["ir_loss_class"] = extractResult.LossClass
	manifest["source_format"] = sourceFormat
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(manifestPath, manifestData, 0600)
	oldPath := renameCapsuleToOld(capsulePath)
	if oldPath == "" {
		return "", fmt.Errorf("failed to rename original")
	}
	if err := archive.CreateCapsuleTarGz(newCapsuleDir, capsulePath); err != nil {
		os.Rename(oldPath, capsulePath)
		return "", fmt.Errorf("failed to create capsule: %w", err)
	}
	return oldPath, nil
}

func (c *GenerateIRCmd) Run() error {
	capsulePath, _ := filepath.Abs(c.Capsule)

	fmt.Printf("Generating IR for: %s\n", capsulePath)
	fmt.Println()

	if capsuleContainsIR(capsulePath) {
		return fmt.Errorf("capsule already contains IR")
	}

	tempDir, extractDir, contentPath, sourceFormat, err := generateIRPrepareExtract(capsulePath)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("Detected format: %s\n", sourceFormat)
	fmt.Printf("Content: %s\n", filepath.Base(contentPath))
	fmt.Println()

	fmt.Println("Extracting IR...")
	irDir := filepath.Join(tempDir, "ir")
	os.MkdirAll(irDir, 0700)
	extractResult, err := generateIRRunExtraction(getPluginDir(), sourceFormat, contentPath, irDir)
	if err != nil {
		return err
	}
	fmt.Printf("  Loss class: %s\n", extractResult.LossClass)

	fmt.Println("Creating new capsule with IR...")
	newCapsuleDir := filepath.Join(tempDir, "new-capsule")
	oldPath, err := generateIRBuildAndFinalize(extractDir, newCapsuleDir, capsulePath, sourceFormat, extractResult)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("IR generation complete!")
	fmt.Printf("  New capsule: %s\n", capsulePath)
	fmt.Printf("  Backup: %s\n", oldPath)
	fmt.Printf("  Loss class: %s\n", extractResult.LossClass)

	return nil
}

func openTarReader(capsulePath string, f *os.File) (*tar.Reader, io.Closer, error) {
	if strings.HasSuffix(capsulePath, ".tar.xz") {
		cmd := exec.Command("xz", "-dc", capsulePath)
		output, err := cmd.Output()
		if err != nil {
			return nil, nil, fmt.Errorf("xz decompress failed: %w", err)
		}
		return tar.NewReader(strings.NewReader(string(output))), io.NopCloser(strings.NewReader("")), nil
	}
	if strings.HasSuffix(capsulePath, ".tar.gz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return nil, nil, err
		}
		return tar.NewReader(gzr), gzr, nil
	}
	return tar.NewReader(f), io.NopCloser(strings.NewReader("")), nil
}

func stripFirstComponent(name string) string {
	if idx := strings.Index(name, "/"); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func extractTarEntry(tr *tar.Reader, header *tar.Header, destDir string) error {
	name := stripFirstComponent(header.Name)
	if name == "" {
		return nil
	}
	destPath := filepath.Join(destDir, name)
	if header.FileInfo().IsDir() {
		return os.MkdirAll(destPath, 0700)
	}
	os.MkdirAll(filepath.Dir(destPath), 0700)
	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	io.Copy(outFile, tr)
	return nil
}

func extractCapsuleArchive(capsulePath, destDir string) error {
	f, err := os.Open(capsulePath)
	if err != nil {
		return err
	}
	defer f.Close()

	tr, closer, err := openTarReader(capsulePath, f)
	if err != nil {
		return err
	}
	defer closer.Close()

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := extractTarEntry(tr, header, destDir); err != nil {
			return err
		}
	}
	return nil
}

var capsuleFormatPatterns = []struct {
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

func isSkippedFile(name string) bool {
	return name == "manifest.json" || strings.HasSuffix(name, ".ir.json")
}

func findSWORDModule(extractDir string) string {
	modsDir := filepath.Join(extractDir, "mods.d")
	if _, err := os.Stat(modsDir); err != nil {
		return ""
	}
	entries, _ := os.ReadDir(modsDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".conf") {
			return filepath.Join(modsDir, e.Name())
		}
	}
	return ""
}

// findConvertibleContent finds content in extracted capsule.
func findConvertibleContent(extractDir string) (string, string) {
	var found, format string

	filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := strings.ToLower(filepath.Base(path))
		if isSkippedFile(name) {
			return nil
		}
		for _, p := range capsuleFormatPatterns {
			if strings.HasSuffix(name, p.ext) {
				found = path
				format = p.format
				return filepath.SkipAll
			}
		}
		return nil
	})

	if found == "" {
		if path := findSWORDModule(extractDir); path != "" {
			return path, "sword"
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

func findMainArtifact(manifest casManifest) (*casArtifact, error) {
	for i := range manifest.Artifacts {
		if manifest.Artifacts[i].ID == "main" || manifest.Artifacts[i].ID == manifest.MainArtifact {
			return &manifest.Artifacts[i], nil
		}
	}
	if len(manifest.Artifacts) > 0 {
		return &manifest.Artifacts[0], nil
	}
	return nil, fmt.Errorf("no artifacts found in manifest")
}

func blobPathForFile(file casFile, extractDir string) string {
	if file.Blake3 != "" && validation.IsValidHexHash(file.Blake3) {
		return filepath.Join(extractDir, "blobs", "blake3", file.Blake3[:2], file.Blake3)
	}
	if file.SHA256 != "" && validation.IsValidHexHash(file.SHA256) {
		return filepath.Join(extractDir, "blobs", "sha256", file.SHA256[:2], file.SHA256)
	}
	return ""
}

func extractArtifactFiles(files []casFile, extractDir, swordDir string) int {
	extracted := 0
	for _, file := range files {
		blobPath := blobPathForFile(file, extractDir)
		if blobPath == "" {
			continue
		}
		content, err := safefile.ReadFile(blobPath)
		if err != nil {
			continue
		}
		destPath := filepath.Join(swordDir, file.Path)
		os.MkdirAll(filepath.Dir(destPath), 0700)
		os.WriteFile(destPath, content, 0600)
		extracted++
	}
	return extracted
}

func writeSWORDManifest(swordDir string, manifest casManifest) {
	swordManifest := map[string]interface{}{
		"capsule_version": "1.0",
		"module_type":     manifest.ModuleType,
		"id":              manifest.ID,
		"title":           manifest.Title,
		"source_format":   "cas-converted",
		"original_format": manifest.SourceFormat,
	}
	data, _ := json.MarshalIndent(swordManifest, "", "  ")
	os.WriteFile(filepath.Join(swordDir, "manifest.json"), data, 0600)
}

func swordOutputPath(capsulePath string) string {
	p := strings.TrimSuffix(capsulePath, ".tar.xz")
	if strings.HasSuffix(p, ".tar.gz") {
		return p
	}
	return p + ".tar.gz"
}

func (c *CASToSWORDCmd) Run() error {
	capsulePath, _ := filepath.Abs(c.Capsule)

	fmt.Printf("Converting CAS capsule to SWORD format: %s\n", capsulePath)
	fmt.Println()

	if !isCASCapsule(capsulePath) {
		return fmt.Errorf("not a CAS capsule (no blobs/ directory found)")
	}

	tempDir, err := os.MkdirTemp("", "capsule-cas-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	return runCASToSWORDConversion(capsulePath, tempDir)
}

// runCASToSWORDConversion performs the CAS to SWORD conversion steps.
func runCASToSWORDConversion(capsulePath, tempDir string) error {
	fmt.Println("Extracting CAS capsule...")
	extractDir := filepath.Join(tempDir, "extract")
	if err := extractCapsuleArchive(capsulePath, extractDir); err != nil {
		return fmt.Errorf("failed to extract: %w", err)
	}

	manifest, err := loadCASManifest(extractDir)
	if err != nil {
		return err
	}

	fmt.Printf("  ID: %s\n", manifest.ID)
	fmt.Printf("  Title: %s\n", manifest.Title)
	fmt.Println()

	mainArtifact, err := findMainArtifact(manifest)
	if err != nil {
		return err
	}

	swordDir := filepath.Join(tempDir, "sword")
	os.MkdirAll(swordDir, 0700)

	extractCASArtifact(mainArtifact, extractDir, swordDir, manifest)
	return createSWORDCapsule(capsulePath, swordDir)
}

// loadCASManifest loads and parses the CAS manifest.
func loadCASManifest(extractDir string) (casManifest, error) {
	manifestData, err := safefile.ReadFile(filepath.Join(extractDir, "manifest.json"))
	if err != nil {
		return casManifest{}, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest casManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return casManifest{}, fmt.Errorf("failed to parse manifest: %w", err)
	}
	return manifest, nil
}

// extractCASArtifact extracts artifact files and writes SWORD manifest.
func extractCASArtifact(mainArtifact *casArtifact, extractDir, swordDir string, manifest casManifest) {
	fmt.Printf("Extracting artifact: %s (%d files)\n", mainArtifact.ID, len(mainArtifact.Files))
	extracted := extractArtifactFiles(mainArtifact.Files, extractDir, swordDir)
	fmt.Printf("  Extracted %d files\n", extracted)
	writeSWORDManifest(swordDir, manifest)
}

// createSWORDCapsule backs up original and creates new SWORD capsule.
func createSWORDCapsule(capsulePath, swordDir string) error {
	fmt.Println()
	fmt.Println("Creating new capsule...")

	oldPath := renameCapsuleToOld(capsulePath)
	if oldPath == "" {
		return fmt.Errorf("failed to rename original")
	}
	fmt.Printf("  Original backed up: %s\n", filepath.Base(oldPath))

	newPath := swordOutputPath(capsulePath)
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
	pids := findPIDsOnPort(port)
	if len(pids) == 0 {
		return nil
	}
	killPIDs(port, pids)
	time.Sleep(500 * time.Millisecond)
	return nil
}

// findPIDsOnPort tries ss, fuser, and lsof in order, returning the first
// non-empty PID list found.
func findPIDsOnPort(port int) []int {
	if pids := findPIDsViaSS(port); len(pids) > 0 {
		return pids
	}
	if pids := findPIDsViaFuser(port); len(pids) > 0 {
		return pids
	}
	return findPIDsViaLsof(port)
}

// findPIDsViaSS uses the ss command to find PIDs listening on port.
func findPIDsViaSS(port int) []int {
	out, err := exec.Command("ss", "-tlnp", fmt.Sprintf("sport = :%d", port)).Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, line := range strings.Split(string(out), "\n") {
		idx := strings.Index(line, "pid=")
		if idx == -1 {
			continue
		}
		rest := line[idx+4:]
		end := strings.IndexAny(rest, ",) \t")
		if end == -1 {
			end = len(rest)
		}
		if pid, err := strconv.Atoi(rest[:end]); err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}

// findPIDsViaFuser uses the fuser command to find PIDs listening on port.
func findPIDsViaFuser(port int) []int {
	out, err := exec.Command("fuser", fmt.Sprintf("%d/tcp", port)).Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, p := range strings.Fields(string(out)) {
		if pid, err := strconv.Atoi(p); err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}

// findPIDsViaLsof uses the lsof command to find PIDs listening on port.
func findPIDsViaLsof(port int) []int {
	out, err := exec.Command("lsof", "-t", "-i", fmt.Sprintf(":%d", port)).Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, p := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if pid, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}

// killPIDs sends SIGKILL to each PID in the list, logging progress and
// non-fatal errors.
func killPIDs(port int, pids []int) {
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
	if path := getFlakePathFromExecutable(); path != "" {
		return path
	}

	// Look in current working directory and walk up
	return getFlakePathFromCwd()
}

// getFlakePathFromExecutable looks for flake.nix relative to executable.
func getFlakePathFromExecutable() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	flakePath := filepath.Join(filepath.Dir(exe), "nix")
	if _, err := os.Stat(filepath.Join(flakePath, "flake.nix")); err == nil {
		return flakePath
	}
	return ""
}

// getFlakePathFromCwd looks for flake.nix from cwd and walks up.
func getFlakePathFromCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Check current directory first
	flakePath := filepath.Join(cwd, "nix")
	if _, err := os.Stat(filepath.Join(flakePath, "flake.nix")); err == nil {
		return flakePath
	}

	// Walk up from current directory
	for dir := cwd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		flakePath := filepath.Join(dir, "nix")
		if _, err := os.Stat(filepath.Join(flakePath, "flake.nix")); err == nil {
			return flakePath
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
		os.MkdirAll(goldenDir, 0700)
		os.WriteFile(goldenPath, reportJSON, 0600)
		fmt.Printf("  [NEW]  %s: created golden file\n", name)
		return true, nil
	}

	// Compare report hash to golden
	goldenData, err := safefile.ReadFile(goldenPath)
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
		os.MkdirAll(goldenDir, 0700)
		os.WriteFile(goldenPath, []byte(artifact.Hashes.SHA256+"\n"), 0600)
		fmt.Printf("  [NEW]  %s: created golden hash\n", name)
		return true, nil
	}

	// Compare hash
	goldenData, err := safefile.ReadFile(goldenPath)
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

// extractIRStringField is a helper that copies a string field from a JSON map
// into a pointer target when the key is present and the value is a string.
func extractIRStringField(m map[string]interface{}, key string, dst *string) {
	if v, ok := m[key].(string); ok {
		*dst = v
	}
}

// extractIRTopLevelFields populates the scalar string fields of info from the
// top-level IR map.
func extractIRTopLevelFields(ir map[string]interface{}, info *IRInfo) {
	extractIRStringField(ir, "id", &info.ID)
	extractIRStringField(ir, "version", &info.Version)
	extractIRStringField(ir, "module_type", &info.ModuleType)
	extractIRStringField(ir, "versification", &info.Versification)
	extractIRStringField(ir, "language", &info.Language)
	extractIRStringField(ir, "title", &info.Title)
	extractIRStringField(ir, "loss_class", &info.LossClass)
	extractIRStringField(ir, "source_hash", &info.SourceHash)
}

// countBlockChars sums the character counts of the "text" field across all
// content blocks in a document map.
func countBlockChars(blocks []interface{}) int {
	total := 0
	for _, block := range blocks {
		if blockMap, ok := block.(map[string]interface{}); ok {
			if text, ok := blockMap["text"].(string); ok {
				total += len(text)
			}
		}
	}
	return total
}

// processIRDocument incorporates a single document map into the running totals
// held by info.
func processIRDocument(docMap map[string]interface{}, info *IRInfo) {
	if docID, ok := docMap["id"].(string); ok {
		info.Documents = append(info.Documents, docID)
	}
	if blocks, ok := docMap["content_blocks"].([]interface{}); ok {
		info.TotalBlocks += len(blocks)
		info.TotalChars += countBlockChars(blocks)
	}
	if annotations, ok := docMap["annotations"].([]interface{}); ok && len(annotations) > 0 {
		info.HasAnnotations = true
	}
}

func buildIRInfo(ir map[string]interface{}) *IRInfo {
	info := &IRInfo{}
	extractIRTopLevelFields(ir, info)

	docs, ok := ir["documents"].([]interface{})
	if !ok {
		return info
	}

	info.DocumentCount = len(docs)
	for _, doc := range docs {
		if docMap, ok := doc.(map[string]interface{}); ok {
			processIRDocument(docMap, info)
		}
	}
	return info
}

type irOptField struct {
	label string
	value string
}

func printIROptFields(fields []irOptField) {
	for _, f := range fields {
		if f.value != "" {
			fmt.Printf("  %-16s%s\n", f.label+":", f.value)
		}
	}
}

func printIRDocuments(docs []string) {
	if len(docs) == 0 {
		return
	}
	fmt.Println()
	fmt.Println("Documents")
	fmt.Println("---------")
	limit := len(docs)
	if limit > 10 {
		limit = 10
	}
	for i, docID := range docs[:limit] {
		fmt.Printf("  %d. %s\n", i+1, docID)
	}
	if len(docs) > 10 {
		fmt.Printf("  ... and %d more\n", len(docs)-10)
	}
}

func printIRInfo(info *IRInfo) {
	fmt.Println("IR Corpus Information")
	fmt.Println("=====================")
	fmt.Println()

	sourceHash := ""
	if info.SourceHash != "" {
		sourceHash = info.SourceHash[:16] + "..."
	}

	printIROptFields([]irOptField{
		{"ID", info.ID},
		{"Title", info.Title},
		{"IR Version", info.Version},
		{"Module Type", info.ModuleType},
		{"Language", info.Language},
		{"Versification", info.Versification},
		{"Loss Class", info.LossClass},
		{"Source Hash", sourceHash},
	})

	fmt.Println()
	fmt.Println("Content Summary")
	fmt.Println("---------------")
	fmt.Printf("  Documents:      %d\n", info.DocumentCount)
	fmt.Printf("  Content Blocks: %d\n", info.TotalBlocks)
	fmt.Printf("  Total Chars:    %d\n", info.TotalChars)
	fmt.Printf("  Annotations:    %v\n", info.HasAnnotations)

	printIRDocuments(info.Documents)
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
