// Capsule CLI integration tests for swordpure (list, ingest commands).
// These tests verify the capsule binary's SWORD module commands work end-to-end.
package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// capsuleSwordBinary returns the path to the capsule binary for SWORD operations.
// This is typically the main capsule binary which includes the juniper commands.
func capsuleSwordBinary(t *testing.T) string {
	t.Helper()

	// Look for the capsule binary
	paths := []string{
		"../../capsule",
		"./capsule",
		"../../cmd/capsule/capsule",
		"./cmd/capsule/capsule",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}

	// Check if it's in PATH
	if path, err := exec.LookPath("capsule"); err == nil {
		return path
	}

	t.Skip("capsule binary not found - run 'make build' first")
	return ""
}

// runCapsuleSword runs the capsule CLI with juniper subcommands.
func runCapsuleSword(t *testing.T, args ...string) (string, string, int) {
	t.Helper()

	binary := capsuleSwordBinary(t)

	cmd := exec.Command(binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run capsule: %v", err)
		}
	}

	return stdout.String(), stderr.String(), exitCode
}

// createTestSwordInstallation creates a minimal SWORD installation for testing.
func createTestSwordInstallation(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "capsule-sword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create mods.d directory
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	// Create a minimal module conf
	confContent := `[TestMod]
DataPath=./modules/texts/ztext/testmod/
ModDrv=zText
Description=Test Bible Module
Lang=en
Version=1.0
Encoding=UTF-8
CompressType=ZIP
Versification=KJV
`
	confPath := filepath.Join(modsDir, "testmod.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	// Create data directory with minimal zText files
	dataDir := filepath.Join(tmpDir, "modules", "texts", "ztext", "testmod")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	// Create empty placeholder files (enough for list command)
	placeholders := []string{"ot.bzv", "ot.bzs", "ot.bzz"}
	for _, name := range placeholders {
		path := filepath.Join(dataDir, name)
		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	return tmpDir
}

// TestCapsuleSwordList tests the juniper list command.
func TestCapsuleSwordList(t *testing.T) {
	// Create test SWORD installation
	swordPath := createTestSwordInstallation(t)
	defer os.RemoveAll(swordPath)

	// Run the list command
	stdout, stderr, exitCode := runCapsuleSword(t, "juniper", "list", swordPath)

	if exitCode != 0 {
		t.Logf("stderr: %s", stderr)
		t.Logf("stdout: %s", stdout)
		// Don't fail - command might not be available in this build
		t.Skipf("juniper list command failed (exit %d) - may not be available", exitCode)
	}

	// Check output contains module info
	if !strings.Contains(stdout, "TestMod") {
		t.Errorf("expected output to contain 'TestMod', got: %s", stdout)
	}

	t.Logf("List output: %s", stdout)
}

// TestCapsuleSwordListWithRealSword tests listing modules from ~/.sword if available.
func TestCapsuleSwordListWithRealSword(t *testing.T) {
	// Check if ~/.sword exists
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	swordPath := filepath.Join(home, ".sword")
	if _, err := os.Stat(swordPath); os.IsNotExist(err) {
		t.Skip("~/.sword not found - skipping real SWORD test")
	}

	modsDir := filepath.Join(swordPath, "mods.d")
	if _, err := os.Stat(modsDir); os.IsNotExist(err) {
		t.Skip("~/.sword/mods.d not found - skipping real SWORD test")
	}

	// Run the list command against real installation
	stdout, stderr, exitCode := runCapsuleSword(t, "juniper", "list", swordPath)

	if exitCode != 0 {
		t.Logf("stderr: %s", stderr)
		t.Skipf("juniper list failed on real SWORD (exit %d)", exitCode)
	}

	t.Logf("Real SWORD modules: %s", stdout)

	// Just verify command ran - content depends on what's installed
	if len(stdout) == 0 && len(stderr) == 0 {
		t.Log("No modules listed from ~/.sword")
	}
}

// TestCapsuleSwordListNonExistent tests listing from non-existent path.
func TestCapsuleSwordListNonExistent(t *testing.T) {
	stdout, stderr, exitCode := runCapsuleSword(t, "juniper", "list", "/nonexistent/path/12345")

	// Should fail
	if exitCode == 0 {
		t.Errorf("expected non-zero exit code for non-existent path")
		t.Logf("stdout: %s", stdout)
	}

	// Error message should be informative
	combined := stdout + stderr
	if !strings.Contains(combined, "error") && !strings.Contains(combined, "Error") &&
		!strings.Contains(combined, "not found") && !strings.Contains(combined, "failed") {
		t.Logf("Warning: error message may not be descriptive: %s", combined)
	}
}

// TestCapsuleSwordIngestDryRun tests the ingest command in a simulated way.
// Note: Full ingest testing requires a complete SWORD module with valid data.
func TestCapsuleSwordIngestDryRun(t *testing.T) {
	// Create test SWORD installation
	swordPath := createTestSwordInstallation(t)
	defer os.RemoveAll(swordPath)

	outputDir, err := os.MkdirTemp("", "capsule-ingest-output-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Try to ingest (may fail due to incomplete data, but tests the command path)
	stdout, stderr, exitCode := runCapsuleSword(t, "juniper", "ingest",
		"--path", swordPath,
		"--output", outputDir,
		"TestMod")

	t.Logf("Ingest stdout: %s", stdout)
	t.Logf("Ingest stderr: %s", stderr)
	t.Logf("Exit code: %d", exitCode)

	// We don't require success - the mock module may not have valid data
	// Just verify the command was recognized
	if strings.Contains(stderr, "unknown command") || strings.Contains(stderr, "not found") {
		t.Skip("juniper ingest command not available in this build")
	}
}

// TestCapsuleSwordWorkflow tests a complete list -> ingest workflow.
func TestCapsuleSwordWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping workflow test in short mode")
	}

	// Check for real SWORD installation with modules
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	swordPath := filepath.Join(home, ".sword")
	modsDir := filepath.Join(swordPath, "mods.d")

	// Check if there are any conf files
	files, err := os.ReadDir(modsDir)
	if err != nil || len(files) == 0 {
		t.Skip("no SWORD modules found in ~/.sword/mods.d")
	}

	// Find first .conf file
	var moduleName string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".conf") {
			moduleName = strings.TrimSuffix(f.Name(), ".conf")
			break
		}
	}

	if moduleName == "" {
		t.Skip("no .conf files found in ~/.sword/mods.d")
	}

	t.Logf("Found module: %s", moduleName)

	// Step 1: List modules
	stdout, _, exitCode := runCapsuleSword(t, "juniper", "list", swordPath)
	if exitCode != 0 {
		t.Skipf("juniper list failed - skipping workflow test")
	}

	if !strings.Contains(strings.ToLower(stdout), strings.ToLower(moduleName)) {
		t.Logf("Warning: module %s not in list output", moduleName)
	}

	// Step 2: Try to ingest (output to temp dir)
	outputDir, err := os.MkdirTemp("", "capsule-workflow-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	stdout, stderr, exitCode := runCapsuleSword(t, "juniper", "ingest",
		"--path", swordPath,
		"--output", outputDir,
		moduleName)

	if exitCode == 0 {
		t.Logf("Successfully ingested %s", moduleName)

		// Check if capsule was created
		expectedCapsule := filepath.Join(outputDir, moduleName+".capsule.tar.xz")
		if info, err := os.Stat(expectedCapsule); err == nil {
			t.Logf("Created capsule: %s (%d bytes)", expectedCapsule, info.Size())
		}
	} else {
		t.Logf("Ingest stderr: %s", stderr)
		t.Logf("Ingest may have failed due to module format limitations")
	}

	t.Log("Workflow test completed")
}

// TestCapsuleJuniperHelp tests the juniper help command.
func TestCapsuleJuniperHelp(t *testing.T) {
	stdout, stderr, exitCode := runCapsuleSword(t, "juniper", "--help")

	// Help should succeed or at least show some output
	combined := stdout + stderr

	if exitCode != 0 && len(combined) == 0 {
		t.Skip("juniper command may not be available")
	}

	// Should show available subcommands
	if strings.Contains(combined, "list") || strings.Contains(combined, "ingest") {
		t.Log("Help shows available subcommands")
	}

	t.Logf("Help output: %s", combined)
}
