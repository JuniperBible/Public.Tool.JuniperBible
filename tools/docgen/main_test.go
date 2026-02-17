package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "docgen - Juniper Bible Documentation Generator") {
		t.Errorf("expected usage in stdout, got: %s", stdout.String())
	}
}

func TestRunHelpFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-h"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestRunHelpLongFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestRunNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"unknown"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Unknown command: unknown") {
		t.Errorf("expected unknown command error, got: %s", stderr.String())
	}
}

func TestRunPlugins(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "docgen-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a minimal plugin directory structure
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0700); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	outputDir := filepath.Join(tempDir, "docs")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"plugins", "-o", outputDir, "-p", pluginDir}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Generating PLUGINS.md") {
		t.Errorf("expected generating message, got: %s", stdout.String())
	}
}

func TestRunFormats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "docgen-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0700); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	outputDir := filepath.Join(tempDir, "docs")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"formats", "--output", outputDir, "--plugins", pluginDir}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Generating FORMATS.md") {
		t.Errorf("expected generating message, got: %s", stdout.String())
	}
}

func TestRunCLI(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "docgen-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0700); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	outputDir := filepath.Join(tempDir, "docs")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"cli", "-o", outputDir, "-p", pluginDir}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Generating CLI_REFERENCE.md") {
		t.Errorf("expected generating message, got: %s", stdout.String())
	}
}

func TestRunAll(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "docgen-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0700); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	outputDir := filepath.Join(tempDir, "docs")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"all", "-o", outputDir, "-p", pluginDir}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Generating all documentation") {
		t.Errorf("expected generating message, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Documentation generated successfully!") {
		t.Errorf("expected success message, got: %s", stdout.String())
	}
}

func TestPrintUsage(t *testing.T) {
	var buf bytes.Buffer
	printUsageTo(&buf)
	output := buf.String()
	if !strings.Contains(output, "docgen - Juniper Bible Documentation Generator") {
		t.Error("expected title in usage")
	}
	if !strings.Contains(output, "plugins") {
		t.Error("expected plugins command in usage")
	}
	if !strings.Contains(output, "formats") {
		t.Error("expected formats command in usage")
	}
	if !strings.Contains(output, "cli") {
		t.Error("expected cli command in usage")
	}
	if !strings.Contains(output, "all") {
		t.Error("expected all command in usage")
	}
}

func TestPrintUsageWrapper(t *testing.T) {
	// This tests the printUsage wrapper function
	// We can't easily capture os.Stdout, but we can at least call it to cover the code
	// Redirect stdout temporarily
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printUsage()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "docgen") {
		t.Error("expected docgen in output")
	}
}

// TestMainFunction tests the main function via subprocess.
// This is the only way to test main() since it calls os.Exit.
func TestMainFunction(t *testing.T) {
	if os.Getenv("TEST_MAIN") == "1" {
		// We're in the subprocess - run main()
		os.Args = []string{"docgen", "help"}
		main()
		return
	}

	// Run ourselves as a subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestMainFunction")
	cmd.Env = append(os.Environ(), "TEST_MAIN=1")
	output, err := cmd.CombinedOutput()

	// The subprocess should exit 0 for help
	if err != nil {
		// Check if it's an exit error with code 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 0 {
				t.Errorf("expected exit code 0, got %d, output: %s", exitErr.ExitCode(), output)
			}
		}
	}
}

// TestMainFunctionNoArgs tests main() with no args via subprocess.
func TestMainFunctionNoArgs(t *testing.T) {
	if os.Getenv("TEST_MAIN_NOARGS") == "1" {
		os.Args = []string{"docgen"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainFunctionNoArgs")
	cmd.Env = append(os.Environ(), "TEST_MAIN_NOARGS=1")
	err := cmd.Run()

	if err == nil {
		t.Error("expected non-zero exit")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
		}
	}
}

// mockGenerator implements generator interface for testing error paths.
type mockGenerator struct {
	pluginsErr error
	formatsErr error
	cliErr     error
	allErr     error
}

func (m *mockGenerator) GeneratePlugins() error { return m.pluginsErr }
func (m *mockGenerator) GenerateFormats() error { return m.formatsErr }
func (m *mockGenerator) GenerateCLI() error     { return m.cliErr }
func (m *mockGenerator) GenerateAll() error     { return m.allErr }

func TestRunPluginsError(t *testing.T) {
	// Save and restore original generator
	orig := newGenerator
	defer func() { newGenerator = orig }()

	newGenerator = func(_, _ string) generator {
		return &mockGenerator{pluginsErr: errors.New("plugins generation failed")}
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"plugins"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "plugins generation failed") {
		t.Errorf("expected error message, got: %s", stderr.String())
	}
}

func TestRunFormatsError(t *testing.T) {
	orig := newGenerator
	defer func() { newGenerator = orig }()

	newGenerator = func(_, _ string) generator {
		return &mockGenerator{formatsErr: errors.New("formats generation failed")}
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"formats"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "formats generation failed") {
		t.Errorf("expected error message, got: %s", stderr.String())
	}
}

func TestRunCLIError(t *testing.T) {
	orig := newGenerator
	defer func() { newGenerator = orig }()

	newGenerator = func(_, _ string) generator {
		return &mockGenerator{cliErr: errors.New("cli generation failed")}
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"cli"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "cli generation failed") {
		t.Errorf("expected error message, got: %s", stderr.String())
	}
}

func TestRunAllError(t *testing.T) {
	orig := newGenerator
	defer func() { newGenerator = orig }()

	newGenerator = func(_, _ string) generator {
		return &mockGenerator{allErr: errors.New("all generation failed")}
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"all"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "all generation failed") {
		t.Errorf("expected error message, got: %s", stderr.String())
	}
}
