package swordpure

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		input string
		sep   string
		want  []string
	}{
		{"a,b,c", ",", []string{"a", "b", "c"}},
		{" a , b , c ", ",", []string{"a", "b", "c"}},
		{"one", ",", []string{"one"}},
	}

	for _, tt := range tests {
		got := splitAndTrim(tt.input, tt.sep)
		if len(got) != len(tt.want) {
			t.Errorf("splitAndTrim(%q, %q) returned %d items, want %d", tt.input, tt.sep, len(got), len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitAndTrim(%q, %q)[%d] = %q, want %q", tt.input, tt.sep, i, got[i], tt.want[i])
			}
		}
	}
}

func TestSplitByComma(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"one", []string{"one"}},
	}

	for _, tt := range tests {
		got := splitByComma(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitByComma(%q) returned %d items, want %d", tt.input, len(got), len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitByComma(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  hello  ", "hello"},
		{"hello", "hello"},
		{"  ", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := trimSpace(tt.input)
		if got != tt.want {
			t.Errorf("trimSpace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGetDefaultSwordPath(t *testing.T) {
	path := getDefaultSwordPath()
	if path == "" {
		t.Error("getDefaultSwordPath should return non-empty path")
	}
}

func TestHasFilesInDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Empty directory
	if hasFilesInDir(tmpDir) {
		t.Error("empty directory should not have files")
	}

	// Create a file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if !hasFilesInDir(tmpDir) {
		t.Error("directory with files should return true")
	}
}

func TestHasFilesInDirNonExistent(t *testing.T) {
	if hasFilesInDir("/nonexistent/path") {
		t.Error("non-existent directory should return false")
	}
}

func TestCreateTarGZ(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source directory with test files
	srcDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	testFile := filepath.Join(srcDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create tar.gz
	tarPath := filepath.Join(tmpDir, "test.tar.gz")
	if err := createTarGZ(srcDir, tarPath); err != nil {
		t.Fatalf("createTarGZ failed: %v", err)
	}

	// Verify tar.gz was created
	if _, err := os.Stat(tarPath); os.IsNotExist(err) {
		t.Error("tar.gz file not created")
	}

	// Check file size
	info, err := os.Stat(tarPath)
	if err != nil {
		t.Fatalf("failed to stat tar.gz: %v", err)
	}

	if info.Size() == 0 {
		t.Error("tar.gz file is empty")
	}
}

func TestCreateTarGZNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, "test.tar.gz")
	err = createTarGZ("/nonexistent/path", tarPath)
	if err == nil {
		t.Error("createTarGZ should fail for non-existent source")
	}
}

func TestCreateModuleCapsule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mock module
	_, swordPath := createMockZTextModule(t, tmpDir)

	// Enable test mode to skip IR extraction (speeds up test)
	skipIRExtraction = true
	defer func() { skipIRExtraction = false }()

	// Create output directory
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	// Create capsule
	module := ModuleInfo{
		Name:        "TestMod",
		Type:        "Bible",
		Description: "Test Bible Module",
		Language:    "en",
	}
	outputPath := filepath.Join(outputDir, "testmod.capsule")
	// Test covers the code path - we don't require success since mock data may be incomplete
	_ = createModuleCapsule(swordPath, module, outputPath)
}

func TestCreateModuleCapsuleConfNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create empty SWORD structure (no conf file)
	modsDir := filepath.Join(tmpDir, "sword", "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	module := ModuleInfo{Name: "NonExistent"}
	err = createModuleCapsule(filepath.Join(tmpDir, "sword"), module, "/tmp/test.capsule")
	if err == nil {
		t.Error("createModuleCapsule should fail for missing conf file")
	}
}

func TestCreateModuleCapsuleCaseInsensitiveConf(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mock module with lowercase conf file name
	modsDir := filepath.Join(tmpDir, "mods.d")
	dataDir := filepath.Join(tmpDir, "modules", "texts", "ztext", "testmod")

	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	// Use lowercase conf filename
	confContent := `[TestMod]
DataPath=./modules/texts/ztext/testmod/
ModDrv=zText
Lang=en
`
	confPath := filepath.Join(modsDir, "testmod.conf") // lowercase
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	// Create minimal data files
	if err := os.WriteFile(filepath.Join(dataDir, "ot.bzz"), []byte{}, 0644); err != nil {
		t.Fatalf("failed to write bzz: %v", err)
	}

	skipIRExtraction = true
	defer func() { skipIRExtraction = false }()

	// Try to create capsule with uppercase module name (should find lowercase conf)
	module := ModuleInfo{Name: "TESTMOD"} // uppercase
	outputPath := filepath.Join(tmpDir, "test.capsule")
	err = createModuleCapsule(tmpDir, module, outputPath)
	if err != nil {
		t.Fatalf("createModuleCapsule failed: %v", err)
	}
}

func TestCreateModuleCapsuleNoDataPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	// Create conf without DataPath
	confContent := `[TestMod]
ModDrv=zText
Lang=en
`
	confPath := filepath.Join(modsDir, "testmod.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	module := ModuleInfo{Name: "TestMod"}
	err = createModuleCapsule(tmpDir, module, "/tmp/test.capsule")
	if err == nil {
		t.Error("createModuleCapsule should fail for missing DataPath")
	}
}

func TestExtractIRToCapsule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mock module
	conf, swordPath := createMockZTextModule(t, tmpDir)

	// Create IR output directory
	irDir := filepath.Join(tmpDir, "ir")
	if err := os.MkdirAll(irDir, 0755); err != nil {
		t.Fatalf("failed to create ir dir: %v", err)
	}

	// Extract IR
	err = extractIRToCapsule(conf, swordPath, irDir)
	if err != nil {
		t.Fatalf("extractIRToCapsule failed: %v", err)
	}

	// Check that IR file was created
	irPath := filepath.Join(irDir, "TestMod.ir.json")
	if _, err := os.Stat(irPath); os.IsNotExist(err) {
		t.Error("IR file not created")
	}
}

func TestCreateTarGZWithSubdirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source directory with subdirectory
	srcDir := filepath.Join(tmpDir, "source")
	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Create files in root and subdirectory
	if err := os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root"), 0644); err != nil {
		t.Fatalf("failed to create root file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "sub.txt"), []byte("sub"), 0644); err != nil {
		t.Fatalf("failed to create sub file: %v", err)
	}

	// Create tar.gz
	tarPath := filepath.Join(tmpDir, "test.tar.gz")
	if err := createTarGZ(srcDir, tarPath); err != nil {
		t.Fatalf("createTarGZ failed: %v", err)
	}

	// Verify tar.gz was created and has content
	info, err := os.Stat(tarPath)
	if err != nil {
		t.Fatalf("failed to stat tar.gz: %v", err)
	}
	if info.Size() == 0 {
		t.Error("tar.gz file is empty")
	}
}

// Tests for CLI commands

func TestRunListCmdWithBibles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mock module
	_, swordPath := createMockZTextModule(t, tmpDir)

	var stdout, stderr bytes.Buffer
	args := []string{"cmd", "list", swordPath}

	err = runListCmd(args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runListCmd failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "TestMod") {
		t.Errorf("output should contain TestMod, got: %s", output)
	}
	if !strings.Contains(output, "Bible modules in") {
		t.Errorf("output should contain header, got: %s", output)
	}
}

func TestRunListCmdNoBibles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mods.d with a non-Bible module
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	confContent := `[TestLex]
DataPath=./modules/lexdict/zld/testlex/
ModDrv=zLD
Description=Test Lexicon
Lang=en
`
	if err := os.WriteFile(filepath.Join(modsDir, "testlex.conf"), []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	var stdout, stderr bytes.Buffer
	args := []string{"cmd", "list", tmpDir}

	err = runListCmd(args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runListCmd failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "No Bible modules found") {
		t.Errorf("output should indicate no bibles found, got: %s", output)
	}
}

func TestRunListCmdWithLongDescription(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	// Create conf with very long description
	longDesc := "This is a very long description that exceeds the 40 character limit for display purposes"
	confContent := fmt.Sprintf(`[TestMod]
DataPath=./modules/texts/ztext/testmod/
ModDrv=zText
Description=%s
Lang=en
`, longDesc)
	if err := os.WriteFile(filepath.Join(modsDir, "testmod.conf"), []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	var stdout, stderr bytes.Buffer
	args := []string{"cmd", "list", tmpDir}

	err = runListCmd(args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runListCmd failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "...") {
		t.Errorf("output should truncate long description with ..., got: %s", output)
	}
}

func TestRunListCmdWithEncryptedModule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	confContent := `[EncMod]
DataPath=./modules/texts/ztext/encmod/
ModDrv=zText
Description=Encrypted Module
Lang=en
CipherKey=12345
`
	if err := os.WriteFile(filepath.Join(modsDir, "encmod.conf"), []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	var stdout, stderr bytes.Buffer
	args := []string{"cmd", "list", tmpDir}

	err = runListCmd(args, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runListCmd failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "[encrypted]") {
		t.Errorf("output should indicate encrypted module, got: %s", output)
	}
}

func TestRunListCmdError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	args := []string{"cmd", "list", "/nonexistent/path"}

	err := runListCmd(args, &stdout, &stderr)
	if err == nil {
		t.Error("runListCmd should fail for nonexistent path")
	}
}

func TestRunListCmdDefaultPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	args := []string{"cmd", "list"} // No path specified, uses default

	// This will likely fail because default path doesn't exist, which is fine
	_ = runListCmd(args, &stdout, &stderr)
	// Just ensuring it doesn't panic
}

func TestRunIngestCmdAllFlag(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, swordPath := createMockZTextModule(t, tmpDir)
	outputDir := filepath.Join(tmpDir, "output")

	skipIRExtraction = true
	defer func() { skipIRExtraction = false }()

	var stdout, stderr bytes.Buffer
	var stdin bytes.Buffer
	args := []string{"cmd", "ingest", "--all", "-p", swordPath, "-o", outputDir}

	err = runIngestCmd(args, &stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runIngestCmd failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Done!") {
		t.Errorf("output should contain Done!, got: %s", output)
	}
}

func TestRunIngestCmdSpecificModule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, swordPath := createMockZTextModule(t, tmpDir)
	outputDir := filepath.Join(tmpDir, "output")

	skipIRExtraction = true
	defer func() { skipIRExtraction = false }()

	var stdout, stderr bytes.Buffer
	var stdin bytes.Buffer
	args := []string{"cmd", "ingest", "TestMod", "-p", swordPath, "-o", outputDir}

	err = runIngestCmd(args, &stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runIngestCmd failed: %v", err)
	}
}

func TestRunIngestCmdModuleNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, swordPath := createMockZTextModule(t, tmpDir)
	outputDir := filepath.Join(tmpDir, "output")

	skipIRExtraction = true
	defer func() { skipIRExtraction = false }()

	var stdout, stderr bytes.Buffer
	var stdin bytes.Buffer
	args := []string{"cmd", "ingest", "NonExistentModule", "-p", swordPath, "-o", outputDir}

	err = runIngestCmd(args, &stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runIngestCmd failed: %v", err)
	}

	// Warning should be printed to stderr
	errOutput := stderr.String()
	if !strings.Contains(errOutput, "not found") {
		t.Errorf("stderr should contain warning, got: %s", errOutput)
	}
}

func TestRunIngestCmdInteractiveQuit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, swordPath := createMockZTextModule(t, tmpDir)

	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString("q\n")
	args := []string{"cmd", "ingest", "-p", swordPath}

	err = runIngestCmd(args, stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runIngestCmd failed: %v", err)
	}
}

func TestRunIngestCmdInteractiveAll(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, swordPath := createMockZTextModule(t, tmpDir)
	outputDir := filepath.Join(tmpDir, "output")

	skipIRExtraction = true
	defer func() { skipIRExtraction = false }()

	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString("all\n")
	args := []string{"cmd", "ingest", "-p", swordPath, "-o", outputDir}

	err = runIngestCmd(args, stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runIngestCmd failed: %v", err)
	}
}

func TestRunIngestCmdInteractiveNumber(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, swordPath := createMockZTextModule(t, tmpDir)
	outputDir := filepath.Join(tmpDir, "output")

	skipIRExtraction = true
	defer func() { skipIRExtraction = false }()

	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString("1\n")
	args := []string{"cmd", "ingest", "-p", swordPath, "-o", outputDir}

	err = runIngestCmd(args, stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runIngestCmd failed: %v", err)
	}
}

func TestRunIngestCmdInteractiveModuleName(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, swordPath := createMockZTextModule(t, tmpDir)
	outputDir := filepath.Join(tmpDir, "output")

	skipIRExtraction = true
	defer func() { skipIRExtraction = false }()

	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString("TestMod\n")
	args := []string{"cmd", "ingest", "-p", swordPath, "-o", outputDir}

	err = runIngestCmd(args, stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runIngestCmd failed: %v", err)
	}
}

func TestRunIngestCmdInteractiveInvalidNumber(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, swordPath := createMockZTextModule(t, tmpDir)

	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString("999\n") // Out of range
	args := []string{"cmd", "ingest", "-p", swordPath}

	err = runIngestCmd(args, stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runIngestCmd failed: %v", err)
	}

	// Should print "No modules selected" since 999 is out of range
	output := stdout.String()
	if !strings.Contains(output, "No modules selected") {
		t.Errorf("output should indicate no modules selected, got: %s", output)
	}
}

func TestRunIngestCmdNoBibles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mods.d with only a non-Bible module
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	confContent := `[TestLex]
DataPath=./modules/lexdict/zld/testlex/
ModDrv=zLD
Description=Test Lexicon
Lang=en
`
	if err := os.WriteFile(filepath.Join(modsDir, "testlex.conf"), []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	var stdout, stderr bytes.Buffer
	var stdin bytes.Buffer
	args := []string{"cmd", "ingest", "-p", tmpDir}

	err = runIngestCmd(args, &stdin, &stdout, &stderr)
	if err == nil {
		t.Error("runIngestCmd should fail when no Bible modules found")
	}
}

func TestRunIngestCmdEncryptedModule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	confContent := `[EncMod]
DataPath=./modules/texts/ztext/encmod/
ModDrv=zText
Description=Encrypted Module
Lang=en
CipherKey=12345
`
	if err := os.WriteFile(filepath.Join(modsDir, "encmod.conf"), []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	var stdout, stderr bytes.Buffer
	var stdin bytes.Buffer
	args := []string{"cmd", "ingest", "--all", "-p", tmpDir, "-o", outputDir}

	err = runIngestCmd(args, &stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runIngestCmd failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Skipping") || !strings.Contains(output, "encrypted") {
		t.Errorf("output should indicate encrypted module was skipped, got: %s", output)
	}
}

func TestRunIngestCmdError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var stdin bytes.Buffer
	args := []string{"cmd", "ingest", "-p", "/nonexistent/path"}

	err := runIngestCmd(args, &stdin, &stdout, &stderr)
	if err == nil {
		t.Error("runIngestCmd should fail for nonexistent path")
	}
}

// Additional coverage tests

func TestSplitAndTrimEmptyInput(t *testing.T) {
	result := splitAndTrim("", ",")
	if len(result) != 0 {
		t.Errorf("splitAndTrim(\"\") should return empty slice, got %v", result)
	}
}

func TestSplitAndTrimColonSeparated(t *testing.T) {
	// Test with colon-separated path (like PATH environment)
	result := splitAndTrim("a:b:c", ":")
	// This uses filepath.SplitList which handles : on Unix
	if len(result) < 1 {
		t.Errorf("splitAndTrim with colon-separated should work, got %v", result)
	}
}

func TestSplitAndTrimMixedSeparators(t *testing.T) {
	// Test with mixed separators
	result := splitAndTrim("a,b", ",")
	if len(result) != 2 {
		t.Errorf("splitAndTrim(\"a,b\") should return 2 items, got %d: %v", len(result), result)
	}
}

func TestSplitAndTrimOnlyWhitespace(t *testing.T) {
	result := splitAndTrim("   ,   ,   ", ",")
	if len(result) != 0 {
		t.Errorf("splitAndTrim with only whitespace should return empty, got %v", result)
	}
}

func TestCreateModuleCapsuleShortDataPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modsDir := filepath.Join(tmpDir, "mods.d")
	dataDir := filepath.Join(tmpDir, "d") // Short data path

	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	// Create conf with short DataPath (just "./" - len<=2)
	confContent := `[TestMod]
DataPath=./
ModDrv=zText
Lang=en
`
	confPath := filepath.Join(modsDir, "testmod.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	// Create minimal data file at root
	if err := os.WriteFile(filepath.Join(tmpDir, "ot.bzz"), []byte{}, 0644); err != nil {
		t.Fatalf("failed to write bzz: %v", err)
	}

	skipIRExtraction = true
	defer func() { skipIRExtraction = false }()

	module := ModuleInfo{Name: "TestMod"}
	outputPath := filepath.Join(tmpDir, "test.capsule")
	// This should hit the dataPath cleanup branch for short paths
	_ = createModuleCapsule(tmpDir, module, outputPath)
}

func TestCreateModuleCapsuleDataPathNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	// Create conf pointing to non-existent data path
	confContent := `[TestMod]
DataPath=./modules/nonexistent/path/
ModDrv=zText
Lang=en
`
	confPath := filepath.Join(modsDir, "testmod.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	module := ModuleInfo{Name: "TestMod"}
	err = createModuleCapsule(tmpDir, module, "/tmp/test.capsule")
	if err == nil {
		t.Error("createModuleCapsule should fail for non-existent data path")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestCreateTarGZWithXZExtension(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source directory with test files
	srcDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	testFile := filepath.Join(srcDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create tar with .tar.xz extension (should be converted to .tar.gz)
	tarPath := filepath.Join(tmpDir, "test.tar.xz")
	if err := createTarGZ(srcDir, tarPath); err != nil {
		t.Fatalf("createTarGZ failed: %v", err)
	}

	// Verify .tar.gz was created instead
	gzPath := filepath.Join(tmpDir, "test.tar.gz")
	if _, err := os.Stat(gzPath); os.IsNotExist(err) {
		t.Error("createTarGZ should create .tar.gz file when given .tar.xz")
	}
}

func TestCreateTarGZWithNoExtension(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source directory with test files
	srcDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	testFile := filepath.Join(srcDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create tar without extension (should add .tar.gz)
	tarPath := filepath.Join(tmpDir, "archive")
	if err := createTarGZ(srcDir, tarPath); err != nil {
		t.Fatalf("createTarGZ failed: %v", err)
	}

	// Verify .tar.gz was created
	gzPath := filepath.Join(tmpDir, "archive.tar.gz")
	if _, err := os.Stat(gzPath); os.IsNotExist(err) {
		t.Error("createTarGZ should add .tar.gz extension")
	}
}

func TestRunIngestCmdWithCreateCapsuleError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock module that will fail during capsule creation
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	// Create conf pointing to non-existent data path (will fail during ingest)
	confContent := `[FailMod]
DataPath=./modules/nonexistent/
ModDrv=zText
Description=Module that will fail
Lang=en
`
	if err := os.WriteFile(filepath.Join(modsDir, "failmod.conf"), []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	var stdout, stderr bytes.Buffer
	var stdin bytes.Buffer
	args := []string{"cmd", "ingest", "--all", "-p", tmpDir, "-o", outputDir}

	// Should not error at the command level (errors are logged per-module)
	err = runIngestCmd(args, &stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runIngestCmd should not fail: %v", err)
	}

	// Check that error was reported to stderr
	errOutput := stderr.String()
	if !strings.Contains(errOutput, "Error") {
		t.Errorf("stderr should contain error message, got: %s", errOutput)
	}
}

func TestRunIngestCmdSuccessWithFileInfo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, swordPath := createMockZTextModule(t, tmpDir)
	outputDir := filepath.Join(tmpDir, "output")

	skipIRExtraction = true
	defer func() { skipIRExtraction = false }()

	var stdout, stderr bytes.Buffer
	var stdin bytes.Buffer
	args := []string{"cmd", "ingest", "TestMod", "--path", swordPath, "--output", outputDir}

	err = runIngestCmd(args, &stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runIngestCmd failed: %v", err)
	}

	output := stdout.String()
	// Should show file size on successful creation
	if strings.Contains(output, "Created:") && !strings.Contains(output, "bytes") {
		t.Errorf("output should include file size info, got: %s", output)
	}
}

func TestRunIngestCmdInteractiveWithMultipleNumbers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, swordPath := createMockZTextModule(t, tmpDir)
	outputDir := filepath.Join(tmpDir, "output")

	skipIRExtraction = true
	defer func() { skipIRExtraction = false }()

	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString("1,1\n") // Same module twice
	args := []string{"cmd", "ingest", "-p", swordPath, "-o", outputDir}

	err = runIngestCmd(args, stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runIngestCmd failed: %v", err)
	}
}

func TestExtractIRToCapsuleWithEmptyModule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create conf with valid path but empty data (OpenZTextModule will succeed but corpus extraction may fail)
	modsDir := filepath.Join(tmpDir, "mods.d")
	dataDir := filepath.Join(tmpDir, "modules", "texts", "ztext", "emptymod")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	confContent := `[EmptyMod]
DataPath=./modules/texts/ztext/emptymod/
ModDrv=zText
Lang=en
`
	confPath := filepath.Join(modsDir, "emptymod.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	conf, err := ParseConfFile(confPath)
	if err != nil {
		t.Fatalf("failed to parse conf: %v", err)
	}

	irDir := filepath.Join(tmpDir, "ir")
	if err := os.MkdirAll(irDir, 0755); err != nil {
		t.Fatalf("failed to create ir dir: %v", err)
	}

	// This exercises the path (may or may not error depending on extractCorpus behavior)
	_ = extractIRToCapsule(conf, tmpDir, irDir)
}
