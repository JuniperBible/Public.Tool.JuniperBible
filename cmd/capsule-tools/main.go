// Package main provides a standalone CLI for tool archive operations.
// This binary wraps the core/runner package for standalone usage.
//
// Usage:
//
//	capsule-tools list [--dir contrib/tool]
//	capsule-tools archive <toolID> --version <version> --bin <name>=<path> --out <output.tar.xz>
//	capsule-tools run <tool> <profile> [--input <file>] [--out <dir>]
//	capsule-tools execute <capsule> <artifact> <tool> <profile>
//
// Prefer using `capsule tools` instead of this standalone binary.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/core/capsule"
	"github.com/FocuswithJustin/JuniperBible/core/runner"

	// Import embedded plugins registry to register all embedded plugins
	_ "github.com/FocuswithJustin/JuniperBible/internal/embedded"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "list":
		runList(os.Args[2:])
	case "archive":
		runArchive(os.Args[2:])
	case "run":
		runRun(os.Args[2:])
	case "execute":
		runExecute(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	contribDir := fs.String("dir", "contrib/tool", "Path to contrib/tool directory")
	fs.Parse(args)

	registry := runner.NewToolRegistry(*contribDir)
	tools, err := registry.ListTools()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(tools) == 0 {
		fmt.Printf("No tools found in %s\n", *contribDir)
		return
	}

	fmt.Printf("Available tools in %s:\n", *contribDir)
	for _, tool := range tools {
		fmt.Printf("  - %s\n", tool)
	}
}

func runArchive(args []string) {
	fs := flag.NewFlagSet("archive", flag.ExitOnError)
	version := fs.String("version", "", "Tool version (required)")
	out := fs.String("out", "", "Output capsule path (required)")
	fs.Parse(args)

	if len(fs.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "Error: tool ID required\n")
		fs.Usage()
		os.Exit(1)
	}

	toolID := fs.Args()[0]

	if *version == "" {
		fmt.Fprintf(os.Stderr, "Error: --version is required\n")
		fs.Usage()
		os.Exit(1)
	}

	if *out == "" {
		fmt.Fprintf(os.Stderr, "Error: --out is required\n")
		fs.Usage()
		os.Exit(1)
	}

	// This is a simplified version - the full kong-based CLI handles this better
	// For now, just provide an error message directing users to the main CLI
	fmt.Fprintf(os.Stderr, "Error: archive command requires binary specifications\n")
	fmt.Fprintf(os.Stderr, "Use: capsule tools archive %s --version %s --bin name=path --out %s\n", toolID, *version, *out)
	fmt.Fprintf(os.Stderr, "\nPrefer using 'capsule tools archive' instead of this standalone binary.\n")
	os.Exit(1)
}

func runRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	input := fs.String("input", "", "Input file path")
	outDir := fs.String("out", "", "Output directory")
	fs.Parse(args)

	if len(fs.Args()) < 2 {
		exitError("tool and profile required\nUsage: capsule-tools run <tool> <profile> [--input <file>] [--out <dir>]")
	}

	toolID, profile := fs.Args()[0], fs.Args()[1]
	inputPath := resolveInputPath(*input)
	workDir, cleanup := prepareWorkDir(*outDir)
	if cleanup != nil {
		defer cleanup()
	}

	printRunInfo(toolID, profile, inputPath, workDir)
	prepareAndPrintRequest(toolID, profile, inputPath, workDir)
}

// resolveInputPath converts input path to absolute if non-empty
func resolveInputPath(input string) string {
	if input == "" {
		return ""
	}
	absPath, err := filepath.Abs(input)
	if err != nil {
		exitError(err.Error())
	}
	return absPath
}

// prepareWorkDir creates or uses the work directory
func prepareWorkDir(outDir string) (string, func()) {
	if outDir == "" {
		workDir, err := os.MkdirTemp("", "capsule-run-*")
		if err != nil {
			exitError("failed to create temp directory: " + err.Error())
		}
		return workDir, func() { os.RemoveAll(workDir) }
	}
	if err := os.MkdirAll(outDir, 0700); err != nil {
		exitError("failed to create output directory: " + err.Error())
	}
	return outDir, nil
}

// printRunInfo prints the run info header
func printRunInfo(toolID, profile, inputPath, workDir string) {
	fmt.Printf("Running tool: %s, profile: %s\n", toolID, profile)
	fmt.Printf("  Input: %s\n", inputPath)
	fmt.Printf("  Output: %s\n", workDir)
}

// prepareAndPrintRequest prepares the work directory and prints instructions
func prepareAndPrintRequest(toolID, profile, inputPath, workDir string) {
	req := runner.NewRequest(toolID, profile)
	if inputPath != "" {
		req.Inputs = []string{inputPath}
	}
	if err := runner.PrepareWorkDir(workDir, req); err != nil {
		exitError(err.Error())
	}
	fmt.Printf("\nWork directory prepared: %s\n", workDir)
	fmt.Println("Use Nix VM executor to run the tool:")
	fmt.Printf("  nix run .#capsule-vm -- %s/in\n", workDir)
}

// exitError prints an error and exits
func exitError(msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	os.Exit(1)
}

func runExecute(args []string) {
	fs := flag.NewFlagSet("execute", flag.ExitOnError)
	fs.Parse(args)

	if len(fs.Args()) < 4 {
		exitError("capsule, artifact, tool, and profile required\nUsage: capsule-tools execute <capsule> <artifact> <tool> <profile>")
	}

	capsulePath, artifactID := fs.Args()[0], fs.Args()[1]
	toolID, profile := fs.Args()[2], fs.Args()[3]

	tempDir := createTempDir("capsule-tool-run-*")
	defer os.RemoveAll(tempDir)

	artifactPath := unpackAndFindArtifact(capsulePath, artifactID, tempDir)
	workDir := createTempDir("capsule-run-*")
	defer os.RemoveAll(workDir)

	printExecuteInfo(artifactID, capsulePath, toolID, profile)
	prepareAndPrintRequest(toolID, profile, artifactPath, workDir)
}

// createTempDir creates a temporary directory or exits on error
func createTempDir(pattern string) string {
	tempDir, err := os.MkdirTemp("", pattern)
	if err != nil {
		exitError("failed to create temp directory: " + err.Error())
	}
	return tempDir
}

// unpackAndFindArtifact unpacks the capsule and finds the artifact path
func unpackAndFindArtifact(capsulePath, artifactID, tempDir string) string {
	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		exitError("failed to unpack capsule: " + err.Error())
	}
	for _, art := range cap.Manifest.Artifacts {
		if art.ID == artifactID {
			return filepath.Join(tempDir, "artifacts", art.ID)
		}
	}
	exitError("artifact not found: " + artifactID)
	return "" // unreachable
}

// printExecuteInfo prints the execute info header
func printExecuteInfo(artifactID, capsulePath, toolID, profile string) {
	fmt.Printf("Executing tool on artifact: %s\n", artifactID)
	fmt.Printf("  Capsule: %s\n", capsulePath)
	fmt.Printf("  Tool: %s\n", toolID)
	fmt.Printf("  Profile: %s\n", profile)
}

func printUsage() {
	fmt.Println(`capsule-tools - Tool archive operations

Usage:
  capsule-tools <command> [options]

Commands:
  list          List available tools in contrib/tool directory
  archive       Create tool archive capsule from binaries
  run           Run a tool plugin with Nix executor
  execute       Run tool on artifact and store transcript

Options for 'list':
  --dir         Path to contrib/tool directory (default: contrib/tool)

Options for 'archive':
  --version     Tool version (required)
  --out         Output capsule path (required)
  Note: Use 'capsule tools archive' for full functionality

Options for 'run':
  --input       Input file path
  --out         Output directory

Options for 'execute':
  (no flags - all arguments positional)

Examples:
  capsule-tools list
  capsule-tools run pandoc default --input input.md --out output/
  capsule-tools execute capsule.tar.xz artifact1 pandoc default

Prefer using 'capsule tools' instead of this standalone binary.`)
}
