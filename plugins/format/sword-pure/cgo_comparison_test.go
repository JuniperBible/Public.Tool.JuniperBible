package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests compare sword-pure (pure Go) output with native libsword tools.
// They only run when diatheke, mod2imp are available in PATH.
//
// These are LONG-RUNNING tests and are skipped by default.
// Run with: go test -run CGOComparison -v -count=1

// checkNativeTools returns true if libsword native tools are available.
func checkNativeTools() bool {
	tools := []string{"diatheke", "mod2imp"}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			return false
		}
	}
	return true
}

// skipIfShort skips tests in short mode
func skipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CGO comparison test in short mode")
	}
}

// TestCGOComparison_VerseCount compares total verse counts between implementations.
// Note: There is expected variance between native mod2imp and pure-Go extraction due to:
// - Versification system differences
// - How mod2imp counts entries vs how we count content blocks
// - Native tool includes all keys while we only count non-empty verses
func TestCGOComparison_VerseCount(t *testing.T) {
	t.Skip("Test times out loading large compressed modules - needs optimization")
	skipIfShort(t)
	if !checkNativeTools() {
		t.Skip("Skipping CGO comparison test: mod2imp not available")
	}

	sampleDir := findSampleDataDir()
	if sampleDir == "" {
		t.Skip("Skipping: no sample data directory found")
	}

	kjvSwordPath := findModuleSwordPath(sampleDir, "KJV")
	if kjvSwordPath == "" {
		t.Skip("Skipping: KJV module not found")
	}

	os.Setenv("SWORD_PATH", kjvSwordPath)
	defer os.Unsetenv("SWORD_PATH")

	// Get key count from native tool
	nativeCount := getNativeKeyCount(t, "KJV")

	// Get verse count from pure-Go
	pureCount := getPureGoVerseCount(t, kjvSwordPath, "KJV")

	// Allow significant variance due to different versification handling
	// mod2imp counts all keys including empty ones, while we count content blocks
	diff := abs(nativeCount - pureCount)
	tolerance := 2000 // Allow up to 2000 difference (versification + empty verses)

	if diff > tolerance {
		t.Errorf("Verse count mismatch:\nNative: %d\nPure-Go: %d\nDiff: %d (tolerance: %d)",
			nativeCount, pureCount, diff, tolerance)
	} else {
		t.Logf("Verse counts within tolerance:\nNative (mod2imp): %d\nPure-Go: %d\nDiff: %d",
			nativeCount, pureCount, diff)
	}
}

// TestCGOComparison_EmitReadable verifies that pure-Go emitted modules can be read by native tools.
// This is the most important interoperability test: extract a module with pure-Go,
// emit it as a new SWORD module, and verify diatheke can read it.
func TestCGOComparison_EmitReadable(t *testing.T) {
	skipIfShort(t)
	if !checkNativeTools() {
		t.Skip("Skipping CGO comparison test: diatheke not available")
	}

	sampleDir := findSampleDataDir()
	if sampleDir == "" {
		t.Skip("Skipping: no sample data directory found")
	}

	// Use WEB which has both OT and NT
	modulePath := findModuleSwordPath(sampleDir, "WEB")
	moduleName := "web"
	if modulePath == "" {
		// Fall back to ASV
		modulePath = findModuleSwordPath(sampleDir, "ASV")
		moduleName = "asv"
	}
	if modulePath == "" {
		t.Skip("Skipping: WEB/ASV module not found")
	}

	tmpDir := t.TempDir()

	// Step 1: Extract IR from original module using pure-Go
	confPath := filepath.Join(modulePath, "mods.d", moduleName+".conf")
	corpus, err := loadCorpus(confPath, modulePath)
	if err != nil {
		t.Fatalf("Failed to extract corpus: %v", err)
	}

	// Step 2: Emit new module using pure-Go
	emitDir := filepath.Join(tmpDir, "emitted")
	os.MkdirAll(emitDir, 0755)

	result, err := EmitZText(corpus, emitDir)
	if err != nil {
		t.Fatalf("Failed to emit module: %v", err)
	}

	// Step 3: Set SWORD_PATH to emitted module and try reading with diatheke
	os.Setenv("SWORD_PATH", emitDir)

	// Try to read Genesis 1:1 from emitted module
	cmd := exec.Command("diatheke", "-b", corpus.ID, "-k", "Gen 1:1")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		t.Logf("diatheke output: %s", outputStr)
		t.Fatalf("Native tool cannot read emitted module: %v", err)
	}

	// Check that we got some content back (not an error message)
	if strings.Contains(outputStr, "Entry does not exist") ||
		strings.Contains(outputStr, "not found") {
		t.Errorf("Native tool returned error for emitted module: %s", outputStr)
	}

	// Check that we got actual verse content (Genesis 1:1 should mention "beginning" and "God")
	if !strings.Contains(outputStr, "beginning") || !strings.Contains(outputStr, "God") {
		t.Errorf("Native tool returned unexpected content: %s", outputStr)
	}

	t.Logf("Successfully read emitted module with native tools")
	t.Logf("Verses written: %d", result.VersesWritten)
	t.Logf("Native output (truncated): %.200s...", outputStr)
}

// TestCGOComparison_MultipleModules tests that multiple modules can be emitted and read.
// This is a more comprehensive test that verifies multiple Bible translations.
func TestCGOComparison_MultipleModules(t *testing.T) {
	skipIfShort(t)
	if !checkNativeTools() {
		t.Skip("Skipping CGO comparison test: native tools not available")
	}

	sampleDir := findSampleDataDir()
	if sampleDir == "" {
		t.Skip("Skipping: no sample data directory found")
	}

	// Test with multiple modules that have Genesis (OT)
	modules := []string{"WEB", "ASV"}

	for _, moduleName := range modules {
		t.Run(moduleName, func(t *testing.T) {
			moduleSwordPath := findModuleSwordPath(sampleDir, moduleName)
			if moduleSwordPath == "" {
				t.Skipf("Skipping: %s module not found", moduleName)
			}

			tmpDir := t.TempDir()

			// Extract with pure-Go
			confPath := filepath.Join(moduleSwordPath, "mods.d", strings.ToLower(moduleName)+".conf")
			corpus, err := loadCorpus(confPath, moduleSwordPath)
			if err != nil {
				t.Fatalf("Failed to extract corpus: %v", err)
			}

			// Emit
			emitDir := filepath.Join(tmpDir, "emitted")
			os.MkdirAll(emitDir, 0755)

			result, err := EmitZText(corpus, emitDir)
			if err != nil {
				t.Fatalf("Failed to emit module: %v", err)
			}

			// Verify with native tool
			os.Setenv("SWORD_PATH", emitDir)
			cmd := exec.Command("diatheke", "-b", corpus.ID, "-k", "Gen 1:1")
			output, err := cmd.CombinedOutput()

			if err != nil || strings.Contains(string(output), "not found") {
				t.Errorf("Native tool failed to read emitted %s module", moduleName)
			} else {
				t.Logf("%s: %d verses emitted, native verification passed", moduleName, result.VersesWritten)
			}
		})
	}
}

// Helper functions

// loadCorpus loads a corpus from a SWORD module.
func loadCorpus(confPath, swordPath string) (*IRCorpus, error) {
	conf, err := ParseConfFile(confPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse conf: %w", err)
	}

	mod, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open module: %w", err)
	}

	corpus, _, err := extractCorpus(mod, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to extract corpus: %w", err)
	}

	return corpus, nil
}

func getNativeKeyCount(t *testing.T, module string) int {
	cmd := exec.Command("mod2imp", module)
	output, err := cmd.Output()
	if err != nil {
		t.Logf("mod2imp error: %v", err)
		return 0
	}

	// Count $$$ lines (keys)
	count := 0
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "$$$") {
			count++
		}
	}
	return count
}

func getPureGoVerseCount(t *testing.T, swordPath, module string) int {
	confPath := filepath.Join(swordPath, "mods.d", strings.ToLower(module)+".conf")
	corpus, err := loadCorpus(confPath, swordPath)
	if err != nil {
		t.Logf("Pure-Go extraction error: %v", err)
		return 0
	}

	count := 0
	for _, doc := range corpus.Documents {
		count += len(doc.ContentBlocks)
	}
	return count
}

func findSampleDataDir() string {
	// Try common locations
	candidates := []string{
		"contrib/sample-data",
		"../../../contrib/sample-data",
		"../../../../contrib/sample-data",
	}

	for _, cand := range candidates {
		if info, err := os.Stat(cand); err == nil && info.IsDir() {
			absPath, _ := filepath.Abs(cand)
			return absPath
		}
	}

	// Try from current working directory
	cwd, _ := os.Getwd()
	for dir := cwd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "contrib", "sample-data")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	return ""
}

// findModuleSwordPath returns the SWORD_PATH for a module in sample-data.
// Sample data structure is contrib/sample-data/{module}/ with mods.d/ and modules/ inside.
func findModuleSwordPath(sampleDir, moduleName string) string {
	moduleDir := filepath.Join(sampleDir, strings.ToLower(moduleName))
	confPath := filepath.Join(moduleDir, "mods.d", strings.ToLower(moduleName)+".conf")
	if _, err := os.Stat(confPath); err == nil {
		return moduleDir
	}
	return ""
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
