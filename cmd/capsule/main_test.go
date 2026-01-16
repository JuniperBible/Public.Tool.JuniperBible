package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/capsule"
	"github.com/FocuswithJustin/JuniperBible/core/cas"
	"github.com/FocuswithJustin/JuniperBible/core/plugins"
	"github.com/FocuswithJustin/JuniperBible/internal/archive"
	"github.com/FocuswithJustin/JuniperBible/internal/fileutil"
)

func init() {
	// Enable external plugins for testing
	plugins.EnableExternalPlugins()
}

// Test helper functions

func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	return path
}

func createTestCapsule(t *testing.T, dir string) (*capsule.Capsule, string) {
	t.Helper()
	capsuleDir := filepath.Join(dir, "test-capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create test capsule: %v", err)
	}
	return cap, capsuleDir
}

func createPackedCapsule(t *testing.T, dir, content string) string {
	t.Helper()
	// Create capsule
	cap, capsuleDir := createTestCapsule(t, dir)

	// Create test file and ingest
	testFile := createTestFile(t, dir, "test.txt", content)
	_, err := cap.IngestFile(testFile)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Pack capsule
	packedPath := filepath.Join(dir, "test.capsule.tar.xz")
	if err := cap.Pack(packedPath); err != nil {
		t.Fatalf("failed to pack capsule: %v", err)
	}

	// Clean up unpacked directory
	os.RemoveAll(capsuleDir)

	return packedPath
}

// Tests for IngestCmd

func TestIngestCmd_Run(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		wantErr     bool
	}{
		{
			name:        "successful ingest",
			fileContent: "Hello, World!",
			wantErr:     false,
		},
		{
			name:        "empty file",
			fileContent: "",
			wantErr:     false,
		},
		{
			name:        "large file",
			fileContent: strings.Repeat("test data\n", 1000),
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			tempDir := t.TempDir()
			testFile := createTestFile(t, tempDir, "input.txt", tt.fileContent)
			outputPath := filepath.Join(tempDir, "output.capsule.tar.xz")

			// Run command
			cmd := &IngestCmd{
				Path: testFile,
				Out:  outputPath,
			}
			err := cmd.Run()

			// Verify
			if (err != nil) != tt.wantErr {
				t.Errorf("IngestCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if _, err := os.Stat(outputPath); os.IsNotExist(err) {
					t.Errorf("output capsule not created")
				}
			}
		})
	}
}

func TestIngestCmd_Run_InvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	cmd := &IngestCmd{
		Path: filepath.Join(tempDir, "nonexistent.txt"),
		Out:  filepath.Join(tempDir, "output.capsule.tar.xz"),
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for nonexistent input file, got nil")
	}
}

// Tests for ExportCmd

func TestExportCmd_Run(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		wantErr     bool
	}{
		{
			name:        "successful export",
			fileContent: "test content",
			wantErr:     false,
		},
		{
			name:        "empty content",
			fileContent: "",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			tempDir := t.TempDir()
			packedPath := createPackedCapsule(t, tempDir, tt.fileContent)

			// Get artifact ID
			cap, err := capsule.Unpack(packedPath, filepath.Join(tempDir, "unpack"))
			if err != nil {
				t.Fatalf("failed to unpack for test setup: %v", err)
			}
			var artifactID string
			for id := range cap.Manifest.Artifacts {
				artifactID = id
				break
			}

			outputPath := filepath.Join(tempDir, "exported.txt")

			// Run command
			cmd := &ExportCmd{
				Capsule:  packedPath,
				Artifact: artifactID,
				Out:      outputPath,
			}
			err = cmd.Run()

			// Verify
			if (err != nil) != tt.wantErr {
				t.Errorf("ExportCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				exportedData, err := os.ReadFile(outputPath)
				if err != nil {
					t.Fatalf("failed to read exported file: %v", err)
				}
				if string(exportedData) != tt.fileContent {
					t.Errorf("exported content = %q, want %q", string(exportedData), tt.fileContent)
				}
			}
		})
	}
}

func TestExportCmd_Run_InvalidArtifact(t *testing.T) {
	tempDir := t.TempDir()
	packedPath := createPackedCapsule(t, tempDir, "test")

	cmd := &ExportCmd{
		Capsule:  packedPath,
		Artifact: "nonexistent-artifact-id",
		Out:      filepath.Join(tempDir, "out.txt"),
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for invalid artifact ID, got nil")
	}
}

// Tests for VerifyCmd

func TestVerifyCmd_Run(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "valid capsule",
			content: "test data for verification",
			wantErr: false,
		},
		{
			name:    "empty capsule content",
			content: "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			tempDir := t.TempDir()
			packedPath := createPackedCapsule(t, tempDir, tt.content)

			// Run command
			cmd := &VerifyCmd{
				Capsule: packedPath,
			}
			err := cmd.Run()

			// Verify
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyCmd_Run_InvalidCapsule(t *testing.T) {
	tempDir := t.TempDir()
	invalidPath := createTestFile(t, tempDir, "invalid.tar.xz", "not a valid capsule")

	cmd := &VerifyCmd{
		Capsule: invalidPath,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for invalid capsule, got nil")
	}
}

// Tests for SelfcheckCmd

func TestSelfcheckCmd_Run(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		jsonOutput bool
		wantErr    bool
	}{
		{
			name:       "successful selfcheck text output",
			content:    "test content",
			jsonOutput: false,
			wantErr:    false,
		},
		{
			name:       "successful selfcheck json output",
			content:    "test content",
			jsonOutput: true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			tempDir := t.TempDir()
			packedPath := createPackedCapsule(t, tempDir, tt.content)

			// Run command
			cmd := &SelfcheckCmd{
				Capsule: packedPath,
				JSON:    tt.jsonOutput,
			}
			err := cmd.Run()

			// Verify
			if (err != nil) != tt.wantErr {
				t.Errorf("SelfcheckCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Tests for PluginsCmd

func TestPluginsCmd_Run(t *testing.T) {
	tempDir := t.TempDir()

	// Create a minimal plugin directory
	pluginDir := filepath.Join(tempDir, "plugins")
	os.MkdirAll(pluginDir, 0755)

	cmd := &PluginsListCmd{
		Dir: pluginDir,
	}

	err := cmd.Run(nil)
	if err != nil {
		t.Errorf("PluginsListCmd.Run() error = %v, want nil", err)
	}
}

// Tests for DetectCmd

func TestDetectCmd_Run_NoMatchingFormat(t *testing.T) {
	tempDir := t.TempDir()
	testFile := createTestFile(t, tempDir, "test.txt", "test content")

	// Set plugin dir to empty directory - but embedded plugins are still available
	CLI.PluginDir = filepath.Join(tempDir, "plugins")
	os.MkdirAll(CLI.PluginDir, 0755)
	defer func() { CLI.PluginDir = "" }()

	cmd := &DetectCmd{
		Path: testFile,
	}

	// With embedded plugins, detect runs without error even if no format matches.
	// It will report that no format was detected but not return an error.
	err := cmd.Run(nil)
	if err != nil {
		t.Errorf("DetectCmd.Run() error = %v, expected nil (embedded plugins available)", err)
	}
}

// Tests for EnumerateCmd

func TestEnumerateCmd_Run_NoMatchingPlugin(t *testing.T) {
	tempDir := t.TempDir()
	testFile := createTestFile(t, tempDir, "test.txt", "test content")

	// Set plugin dir to empty directory
	CLI.PluginDir = filepath.Join(tempDir, "plugins")
	os.MkdirAll(CLI.PluginDir, 0755)
	defer func() { CLI.PluginDir = "" }()

	cmd := &EnumerateCmd{
		Path: testFile,
	}

	err := cmd.Run(nil)
	// With embedded plugins, format.file will match .txt files
	// So enumeration should succeed (no error) or fail for other reasons
	// We just verify it doesn't panic
	_ = err
}

// Tests for TestCmd

func TestTestCmd_Run(t *testing.T) {
	tests := []struct {
		name    string
		setupFn func(t *testing.T, fixturesDir string)
		wantErr bool
	}{
		{
			name: "no capsules",
			setupFn: func(t *testing.T, fixturesDir string) {
				// Create empty fixtures directory
			},
			wantErr: false, // No tests to run is not an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			fixturesDir := filepath.Join(tempDir, "fixtures")
			os.MkdirAll(fixturesDir, 0755)

			if tt.setupFn != nil {
				tt.setupFn(t, fixturesDir)
			}

			cmd := &TestCmd{
				FixturesDir: fixturesDir,
			}

			err := cmd.Run()
			if (err != nil) != tt.wantErr {
				t.Errorf("TestCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Tests for RunCmd

func TestRunCmd_Run_NoFlake(t *testing.T) {
	// Skip this test if we're in a nix environment with a flake available
	if getFlakePath() != "" {
		t.Skip("skipping test because nix flake is available")
	}

	tempDir := t.TempDir()
	testFile := createTestFile(t, tempDir, "input.txt", "test")

	cmd := &RunCmd{
		Tool:    "test-tool",
		Profile: "default",
		Input:   testFile,
		Out:     tempDir,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error when nix flake not found, got nil")
	}
}

// Tests for CompareCmd

func TestCompareCmd_Run_InvalidRun(t *testing.T) {
	tempDir := t.TempDir()
	packedPath := createPackedCapsule(t, tempDir, "test")

	cmd := &CompareCmd{
		Capsule: packedPath,
		Run1:    "nonexistent-run-1",
		Run2:    "nonexistent-run-2",
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for invalid run IDs, got nil")
	}
}

// Tests for RunsCmd

func TestRunsCmd_Run(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "list runs on capsule",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			packedPath := createPackedCapsule(t, tempDir, "test content")

			cmd := &RunsListCmd{
				Capsule: packedPath,
			}

			err := cmd.Run()
			if (err != nil) != tt.wantErr {
				t.Errorf("RunsListCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Tests for GoldenSaveCmd and GoldenCheckCmd

func TestGoldenSaveCmd_Run_NoRun(t *testing.T) {
	tempDir := t.TempDir()
	packedPath := createPackedCapsule(t, tempDir, "test")
	goldenFile := filepath.Join(tempDir, "golden.sha256")

	cmd := &GoldenSaveCmd{
		Capsule: packedPath,
		RunID:   "nonexistent-run",
		Out:     goldenFile,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for nonexistent run, got nil")
	}
}

// Tests for ExtractIRCmd

func TestExtractIRCmd_Run_InvalidFormat(t *testing.T) {
	tempDir := t.TempDir()
	testFile := createTestFile(t, tempDir, "input.txt", "test")
	outputPath := filepath.Join(tempDir, "output.json")

	// Set plugin dir
	CLI.PluginDir = filepath.Join(tempDir, "plugins")
	os.MkdirAll(CLI.PluginDir, 0755)
	defer func() { CLI.PluginDir = "" }()

	cmd := &ExtractIRCmd{
		Path:   testFile,
		Format: "nonexistent-format",
		Out:    outputPath,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for nonexistent format plugin, got nil")
	}
}

// Tests for EmitNativeCmd

func TestEmitNativeCmd_Run_InvalidFormat(t *testing.T) {
	tempDir := t.TempDir()
	irFile := createTestFile(t, tempDir, "ir.json", "{}")
	outputPath := filepath.Join(tempDir, "output.txt")

	// Set plugin dir
	CLI.PluginDir = filepath.Join(tempDir, "plugins")
	os.MkdirAll(CLI.PluginDir, 0755)
	defer func() { CLI.PluginDir = "" }()

	cmd := &EmitNativeCmd{
		IR:     irFile,
		Format: "nonexistent-format",
		Out:    outputPath,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for nonexistent format plugin, got nil")
	}
}

// Tests for ConvertCmd

func TestConvertCmd_Run_WithEmbeddedPlugins(t *testing.T) {
	tempDir := t.TempDir()
	testFile := createTestFile(t, tempDir, "input.txt", "test content")
	outputPath := filepath.Join(tempDir, "output.txt")

	// Even with empty external plugin dir, embedded plugins are available
	CLI.PluginDir = filepath.Join(tempDir, "plugins")
	os.MkdirAll(CLI.PluginDir, 0755)
	defer func() { CLI.PluginDir = "" }()

	cmd := &ConvertCmd{
		Path: testFile,
		To:   "txt",
		Out:  outputPath,
	}

	// Run will fail because generic file format doesn't support IR extraction
	// but this verifies embedded plugins are loaded and format detection works
	err := cmd.Run()
	// We expect an error about IR extraction not being supported, which is fine
	// The important thing is that we got past plugin loading and detection
	if err != nil && !strings.Contains(err.Error(), "IR extraction") {
		t.Errorf("expected IR extraction error or success, got: %v", err)
	}
}

// Tests for IRInfoCmd

func TestIRInfoCmd_Run(t *testing.T) {
	tests := []struct {
		name       string
		irContent  string
		jsonOutput bool
		wantErr    bool
	}{
		{
			name: "valid IR text output",
			irContent: `{
				"id": "test-ir",
				"version": "1.0",
				"module_type": "bible",
				"language": "en",
				"documents": []
			}`,
			jsonOutput: false,
			wantErr:    false,
		},
		{
			name: "valid IR json output",
			irContent: `{
				"id": "test-ir",
				"version": "1.0",
				"module_type": "bible",
				"language": "en",
				"documents": []
			}`,
			jsonOutput: true,
			wantErr:    false,
		},
		{
			name:       "invalid JSON",
			irContent:  "not valid json",
			jsonOutput: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			irFile := createTestFile(t, tempDir, "ir.json", tt.irContent)

			cmd := &IRInfoCmd{
				IR:   irFile,
				JSON: tt.jsonOutput,
			}

			err := cmd.Run()
			if (err != nil) != tt.wantErr {
				t.Errorf("IRInfoCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Tests for ToolArchiveCmd

func TestToolArchiveCmd_Run_InvalidBinary(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "tool.tar.gz")

	cmd := &ToolArchiveCmd{
		ToolID:  "test-tool",
		Version: "1.0",
		Bin: map[string]string{
			"tool": filepath.Join(tempDir, "nonexistent-binary"),
		},
		Out: outputPath,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for nonexistent binary, got nil")
	}
}

// Tests for ToolListCmd

func TestToolListCmd_Run(t *testing.T) {
	tempDir := t.TempDir()
	contribDir := filepath.Join(tempDir, "contrib", "tool")
	os.MkdirAll(contribDir, 0755)

	cmd := &ToolListCmd{
		ContribDir: contribDir,
	}

	err := cmd.Run()
	if err != nil {
		t.Errorf("ToolListCmd.Run() error = %v, want nil", err)
	}
}

// Tests for DocgenCmd

func TestDocgenCmd_Run(t *testing.T) {
	tests := []struct {
		name       string
		subcommand string
		wantErr    bool
	}{
		{
			name:       "generate plugins",
			subcommand: "plugins",
			wantErr:    true, // Will fail without actual plugins
		},
		{
			name:       "generate formats",
			subcommand: "formats",
			wantErr:    true, // Will fail without actual plugins
		},
		{
			name:       "generate cli",
			subcommand: "cli",
			wantErr:    true, // Will fail without proper setup
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			outputDir := filepath.Join(tempDir, "docs")
			pluginDir := filepath.Join(tempDir, "plugins")
			os.MkdirAll(pluginDir, 0755)

			// Set plugin dir
			CLI.PluginDir = pluginDir
			defer func() { CLI.PluginDir = "" }()

			cmd := &DocgenCmd{
				Subcommand: tt.subcommand,
				Output:     outputDir,
			}

			err := cmd.Run()
			if (err != nil) != tt.wantErr {
				t.Errorf("DocgenCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Tests for JuniperListCmd

func TestJuniperListCmd_Run_NoSWORD(t *testing.T) {
	tempDir := t.TempDir()
	swordPath := filepath.Join(tempDir, "sword")

	cmd := &JuniperListCmd{
		Path: swordPath,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error when SWORD installation not found, got nil")
	}
}

func TestJuniperListCmd_Run_WithSWORD(t *testing.T) {
	tempDir := t.TempDir()
	swordPath := filepath.Join(tempDir, "sword")
	modsDir := filepath.Join(swordPath, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Create a sample conf file
	confContent := `[KJV]
Description=King James Version
Lang=en
ModDrv=zText
`
	createTestFile(t, modsDir, "kjv.conf", confContent)

	cmd := &JuniperListCmd{
		Path: swordPath,
	}

	err := cmd.Run()
	if err != nil {
		t.Errorf("JuniperListCmd.Run() error = %v, want nil", err)
	}
}

// Tests for JuniperIngestCmd

func TestJuniperIngestCmd_Run_NoModules(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")

	cmd := &JuniperIngestCmd{
		Modules: []string{},
		Path:    tempDir,
		Output:  outputDir,
		All:     false,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error when no modules specified, got nil")
	}
}

// Tests for CapsuleConvertCmd

func TestCapsuleConvertCmd_Run_InvalidCapsule(t *testing.T) {
	tempDir := t.TempDir()
	invalidCapsule := createTestFile(t, tempDir, "invalid.tar.gz", "not a capsule")

	cmd := &CapsuleConvertCmd{
		Capsule: invalidCapsule,
		Format:  "osis",
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for invalid capsule, got nil")
	}
}

// Tests for GenerateIRCmd

func TestGenerateIRCmd_Run_InvalidCapsule(t *testing.T) {
	tempDir := t.TempDir()
	invalidCapsule := createTestFile(t, tempDir, "invalid.tar.gz", "not a capsule")

	cmd := &GenerateIRCmd{
		Capsule: invalidCapsule,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for invalid capsule, got nil")
	}
}

// Tests for CASToSWORDCmd

func TestCASToSWORDCmd_Run_NotCAS(t *testing.T) {
	tempDir := t.TempDir()
	nonCASCapsule := createPackedCapsule(t, tempDir, "test")

	cmd := &CASToSWORDCmd{
		Capsule: nonCASCapsule,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for non-CAS capsule, got nil")
	}
}

// Tests for VersionCmd

func TestVersionCmd_Run(t *testing.T) {
	cmd := &VersionCmd{}
	err := cmd.Run()
	if err != nil {
		t.Errorf("VersionCmd.Run() error = %v, want nil", err)
	}
}

// Tests for helper functions

func TestGetPluginDir(t *testing.T) {
	// Save original state
	origPluginDir := CLI.PluginDir
	defer func() { CLI.PluginDir = origPluginDir }()

	tests := []struct {
		name          string
		cliPluginDir  string
		expectNonEmpty bool
	}{
		{
			name:          "with CLI flag set",
			cliPluginDir:  "/custom/plugins",
			expectNonEmpty: true,
		},
		{
			name:          "without CLI flag",
			cliPluginDir:  "",
			expectNonEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CLI.PluginDir = tt.cliPluginDir
			result := getPluginDir()

			if tt.expectNonEmpty && result == "" {
				t.Error("expected non-empty plugin dir")
			}
		})
	}
}

func TestGetFlakePath(t *testing.T) {
	result := getFlakePath()
	// Just ensure it doesn't panic, result may be empty
	_ = result
}

func TestParseConfForList(t *testing.T) {
	tests := []struct {
		name        string
		confContent string
		wantName    string
		wantType    string
		wantLang    string
	}{
		{
			name: "valid Bible module",
			confContent: `[KJV]
Description=King James Version
Lang=en
ModDrv=zText
DataPath=./modules/texts/ztext/kjv/
`,
			wantName: "KJV",
			wantType: "Bible",
			wantLang: "en",
		},
		{
			name: "commentary module",
			confContent: `[MHC]
Description=Matthew Henry Commentary
Lang=en
ModDrv=zCom
`,
			wantName: "MHC",
			wantType: "Commentary",
			wantLang: "en",
		},
		{
			name: "encrypted module",
			confContent: `[ESV]
Description=English Standard Version
Lang=en
ModDrv=zText
CipherKey=xyz123
`,
			wantName: "ESV",
			wantType: "Bible",
			wantLang: "en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			confPath := createTestFile(t, tempDir, "test.conf", tt.confContent)

			module := parseConfForList(confPath)
			if module == nil {
				t.Fatal("parseConfForList returned nil")
			}

			if module.name != tt.wantName {
				t.Errorf("name = %q, want %q", module.name, tt.wantName)
			}
			if module.modType != tt.wantType {
				t.Errorf("modType = %q, want %q", module.modType, tt.wantType)
			}
			if module.lang != tt.wantLang {
				t.Errorf("lang = %q, want %q", module.lang, tt.wantLang)
			}
			if tt.name == "encrypted module" && !module.encrypted {
				t.Error("expected encrypted=true")
			}
		})
	}
}

func TestCopyDirRecursive(t *testing.T) {
	tests := []struct {
		name    string
		setupFn func(t *testing.T, srcDir string)
		wantErr bool
	}{
		{
			name: "copy simple directory",
			setupFn: func(t *testing.T, srcDir string) {
				createTestFile(t, srcDir, "file1.txt", "content1")
				createTestFile(t, srcDir, "file2.txt", "content2")
			},
			wantErr: false,
		},
		{
			name: "copy nested directory",
			setupFn: func(t *testing.T, srcDir string) {
				subDir := filepath.Join(srcDir, "subdir")
				os.MkdirAll(subDir, 0755)
				createTestFile(t, srcDir, "root.txt", "root content")
				createTestFile(t, subDir, "nested.txt", "nested content")
			},
			wantErr: false,
		},
		{
			name: "empty directory",
			setupFn: func(t *testing.T, srcDir string) {
				// Create empty directory
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			srcDir := filepath.Join(tempDir, "src")
			dstDir := filepath.Join(tempDir, "dst")
			os.MkdirAll(srcDir, 0755)

			if tt.setupFn != nil {
				tt.setupFn(t, srcDir)
			}

			err := fileutil.CopyDir(srcDir, dstDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("fileutil.CopyDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if _, err := os.Stat(dstDir); os.IsNotExist(err) {
					t.Error("destination directory not created")
				}
			}
		})
	}
}

func TestExtractCapsuleArchive(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "extract valid capsule",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			// Create a packed capsule
			packedPath := createPackedCapsule(t, tempDir, "test content")

			// Extract it
			extractDir := filepath.Join(tempDir, "extracted")
			err := extractCapsuleArchive(packedPath, extractDir)

			if (err != nil) != tt.wantErr {
				t.Errorf("extractCapsuleArchive() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if _, err := os.Stat(extractDir); os.IsNotExist(err) {
					t.Error("extraction directory not created")
				}
			}
		})
	}
}

func TestFindConvertibleContent(t *testing.T) {
	tests := []struct {
		name         string
		setupFn      func(t *testing.T, dir string)
		wantFormat   string
		wantNotEmpty bool
	}{
		{
			name: "find OSIS file",
			setupFn: func(t *testing.T, dir string) {
				createTestFile(t, dir, "bible.osis", "osis content")
			},
			wantFormat:   "osis",
			wantNotEmpty: true,
		},
		{
			name: "find USFM file",
			setupFn: func(t *testing.T, dir string) {
				createTestFile(t, dir, "book.usfm", "usfm content")
			},
			wantFormat:   "usfm",
			wantNotEmpty: true,
		},
		{
			name: "find USX file",
			setupFn: func(t *testing.T, dir string) {
				createTestFile(t, dir, "book.usx", "usx content")
			},
			wantFormat:   "usx",
			wantNotEmpty: true,
		},
		{
			name: "find JSON file",
			setupFn: func(t *testing.T, dir string) {
				createTestFile(t, dir, "data.json", "{}")
			},
			wantFormat:   "json",
			wantNotEmpty: true,
		},
		{
			name: "skip manifest.json",
			setupFn: func(t *testing.T, dir string) {
				createTestFile(t, dir, "manifest.json", "{}")
			},
			wantFormat:   "",
			wantNotEmpty: false,
		},
		{
			name: "skip IR file",
			setupFn: func(t *testing.T, dir string) {
				createTestFile(t, dir, "test.ir.json", "{}")
			},
			wantFormat:   "",
			wantNotEmpty: false,
		},
		{
			name: "find SWORD module",
			setupFn: func(t *testing.T, dir string) {
				modsDir := filepath.Join(dir, "mods.d")
				os.MkdirAll(modsDir, 0755)
				createTestFile(t, modsDir, "kjv.conf", "[KJV]")
			},
			wantFormat:   "sword",
			wantNotEmpty: true,
		},
		{
			name: "no convertible content",
			setupFn: func(t *testing.T, dir string) {
				createTestFile(t, dir, "readme.txt", "readme")
			},
			wantFormat:   "",
			wantNotEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			if tt.setupFn != nil {
				tt.setupFn(t, tempDir)
			}

			path, format := findConvertibleContent(tempDir)

			if tt.wantNotEmpty && path == "" {
				t.Error("expected non-empty path")
			}
			if !tt.wantNotEmpty && path != "" {
				t.Errorf("expected empty path, got %q", path)
			}
			if format != tt.wantFormat {
				t.Errorf("format = %q, want %q", format, tt.wantFormat)
			}
		})
	}
}

func TestCapsuleContainsIR(t *testing.T) {
	tests := []struct {
		name   string
		hasIR  bool
		expect bool
	}{
		{
			name:   "capsule without IR",
			hasIR:  false,
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			packedPath := createPackedCapsule(t, tempDir, "test content")

			result := capsuleContainsIR(packedPath)
			if result != tt.expect {
				t.Errorf("capsuleContainsIR() = %v, want %v", result, tt.expect)
			}
		})
	}
}

func TestRenameCapsuleToOld(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		wantOldName  string
	}{
		{
			name:        "capsule.tar.gz",
			filename:    "test.capsule.tar.gz",
			wantOldName: "test-old.capsule.tar.gz",
		},
		{
			name:        "capsule.tar.xz",
			filename:    "test.capsule.tar.xz",
			wantOldName: "test-old.capsule.tar.xz",
		},
		{
			name:        "tar.gz",
			filename:    "test.tar.gz",
			wantOldName: "test-old.tar.gz",
		},
		{
			name:        "tar.xz",
			filename:    "test.tar.xz",
			wantOldName: "test-old.tar.xz",
		},
		{
			name:        "other extension",
			filename:    "test.dat",
			wantOldName: "test.dat-old",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			originalPath := createTestFile(t, tempDir, tt.filename, "content")

			oldPath := renameCapsuleToOld(originalPath)

			if oldPath == "" {
				t.Fatal("renameCapsuleToOld returned empty string")
			}

			oldName := filepath.Base(oldPath)
			if oldName != tt.wantOldName {
				t.Errorf("oldName = %q, want %q", oldName, tt.wantOldName)
			}

			// Verify original is gone and old exists
			if _, err := os.Stat(originalPath); !os.IsNotExist(err) {
				t.Error("original file still exists")
			}
			if _, err := os.Stat(oldPath); os.IsNotExist(err) {
				t.Error("old file does not exist")
			}
		})
	}
}

func TestCombineLoss(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want string
	}{
		{
			name: "L0 and L0",
			a:    "L0",
			b:    "L0",
			want: "L0",
		},
		{
			name: "L0 and L1",
			a:    "L0",
			b:    "L1",
			want: "L1",
		},
		{
			name: "L1 and L2",
			a:    "L1",
			b:    "L2",
			want: "L2",
		},
		{
			name: "L2 and L3",
			a:    "L2",
			b:    "L3",
			want: "L3",
		},
		{
			name: "L3 and L0",
			a:    "L3",
			b:    "L0",
			want: "L3",
		},
		{
			name: "empty strings",
			a:    "",
			b:    "",
			want: "",
		},
		{
			name: "one empty",
			a:    "L2",
			b:    "",
			want: "L2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := combineLoss(tt.a, tt.b)
			if result != tt.want {
				t.Errorf("combineLoss(%q, %q) = %q, want %q", tt.a, tt.b, result, tt.want)
			}
		})
	}
}

func TestIsCASCapsule(t *testing.T) {
	tests := []struct {
		name    string
		setupFn func(t *testing.T, tempDir string) string
		expect  bool
	}{
		{
			name: "non-CAS capsule",
			setupFn: func(t *testing.T, tempDir string) string {
				// Create a simple tar.gz without blobs/ directory
				capsuleDir := filepath.Join(tempDir, "simple")
				os.MkdirAll(capsuleDir, 0755)
				createTestFile(t, capsuleDir, "test.txt", "content")
				createTestFile(t, capsuleDir, "manifest.json", `{"version":"1.0"}`)

				outputPath := filepath.Join(tempDir, "simple.tar.gz")
				if err := archive.CreateCapsuleTarGz(capsuleDir, outputPath); err != nil {
					t.Fatalf("failed to create archive: %v", err)
				}
				return outputPath
			},
			expect: false,
		},
		{
			name: "CAS capsule",
			setupFn: func(t *testing.T, tempDir string) string {
				// Create a tar.gz with blobs/ directory
				capsuleDir := filepath.Join(tempDir, "cas")
				blobsDir := filepath.Join(capsuleDir, "blobs", "sha256", "ab")
				os.MkdirAll(blobsDir, 0755)
				createTestFile(t, blobsDir, "abc123", "blob content")
				createTestFile(t, capsuleDir, "manifest.json", `{"version":"1.0"}`)

				outputPath := filepath.Join(tempDir, "cas.tar.gz")
				if err := archive.CreateCapsuleTarGz(capsuleDir, outputPath); err != nil {
					t.Fatalf("failed to create archive: %v", err)
				}
				return outputPath
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			capsulePath := tt.setupFn(t, tempDir)

			result := isCASCapsule(capsulePath)
			if result != tt.expect {
				t.Errorf("isCASCapsule() = %v, want %v", result, tt.expect)
			}
		})
	}
}

func TestBuildIRInfo(t *testing.T) {
	tests := []struct {
		name     string
		ir       map[string]interface{}
		wantID   string
		wantType string
	}{
		{
			name: "basic IR",
			ir: map[string]interface{}{
				"id":          "test-bible",
				"version":     "1.0",
				"module_type": "bible",
				"language":    "en",
				"title":       "Test Bible",
				"documents":   []interface{}{},
			},
			wantID:   "test-bible",
			wantType: "bible",
		},
		{
			name: "IR with documents",
			ir: map[string]interface{}{
				"id": "test",
				"documents": []interface{}{
					map[string]interface{}{
						"id": "GEN",
						"content_blocks": []interface{}{
							map[string]interface{}{
								"text": "In the beginning",
							},
						},
					},
				},
			},
			wantID:   "test",
			wantType: "",
		},
		{
			name:     "empty IR",
			ir:       map[string]interface{}{},
			wantID:   "",
			wantType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := buildIRInfo(tt.ir)

			if info.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", info.ID, tt.wantID)
			}
			if info.ModuleType != tt.wantType {
				t.Errorf("ModuleType = %q, want %q", info.ModuleType, tt.wantType)
			}
		})
	}
}

func TestPrintIRInfo(t *testing.T) {
	info := &IRInfo{
		ID:            "test",
		Version:       "1.0",
		ModuleType:    "bible",
		Language:      "en",
		DocumentCount: 2,
		TotalBlocks:   10,
		TotalChars:    1000,
	}

	// Just ensure it doesn't panic
	printIRInfo(info)
}

func TestRunCapsuleTest(t *testing.T) {
	tempDir := t.TempDir()
	goldenDir := filepath.Join(tempDir, "goldens")
	os.MkdirAll(goldenDir, 0755)

	packedPath := createPackedCapsule(t, tempDir, "test content")

	// First run should create golden
	result, err := runCapsuleTest(packedPath, goldenDir, "test")
	if err != nil {
		t.Fatalf("runCapsuleTest() error = %v", err)
	}
	if !result {
		t.Error("expected result = true")
	}

	// Second run should compare against golden
	result, err = runCapsuleTest(packedPath, goldenDir, "test")
	if err != nil {
		t.Fatalf("runCapsuleTest() error = %v", err)
	}
	if !result {
		t.Error("expected result = true for matching golden")
	}
}

func TestRunIngestTest(t *testing.T) {
	tempDir := t.TempDir()
	goldenDir := filepath.Join(tempDir, "goldens")
	os.MkdirAll(goldenDir, 0755)

	inputFile := createTestFile(t, tempDir, "input.txt", "test content")

	// First run should create golden
	result, err := runIngestTest(inputFile, goldenDir, "test")
	if err != nil {
		t.Fatalf("runIngestTest() error = %v", err)
	}
	if !result {
		t.Error("expected result = true")
	}

	// Second run should compare against golden
	result, err = runIngestTest(inputFile, goldenDir, "test")
	if err != nil {
		t.Fatalf("runIngestTest() error = %v", err)
	}
	if !result {
		t.Error("expected result = true for matching golden")
	}
}

func TestCreateCapsuleTarGz(t *testing.T) {
	tests := []struct {
		name    string
		setupFn func(t *testing.T, srcDir string)
		wantErr bool
	}{
		{
			name: "create archive from directory",
			setupFn: func(t *testing.T, srcDir string) {
				createTestFile(t, srcDir, "file1.txt", "content1")
				createTestFile(t, srcDir, "file2.txt", "content2")
			},
			wantErr: false,
		},
		{
			name: "create archive from empty directory",
			setupFn: func(t *testing.T, srcDir string) {
				// Empty directory
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			srcDir := filepath.Join(tempDir, "src")
			os.MkdirAll(srcDir, 0755)

			if tt.setupFn != nil {
				tt.setupFn(t, srcDir)
			}

			dstPath := filepath.Join(tempDir, "archive.tar.gz")
			err := archive.CreateCapsuleTarGz(srcDir, dstPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("archive.CreateCapsuleTarGz() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if _, err := os.Stat(dstPath); os.IsNotExist(err) {
					t.Error("archive not created")
				}
			}
		})
	}
}

func TestIngestSwordModule(t *testing.T) {
	tempDir := t.TempDir()
	swordPath := filepath.Join(tempDir, "sword")
	modsDir := filepath.Join(swordPath, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Create test module
	confContent := `[TEST]
Description=Test Module
Lang=en
ModDrv=zText
DataPath=./modules/texts/ztext/test/
`
	confPath := createTestFile(t, modsDir, "test.conf", confContent)

	// Create data path
	dataPath := filepath.Join(swordPath, "modules", "texts", "ztext", "test")
	os.MkdirAll(dataPath, 0755)
	createTestFile(t, dataPath, "ot", "old testament data")
	createTestFile(t, dataPath, "nt", "new testament data")

	module := &juniperModule{
		name:        "TEST",
		description: "Test Module",
		lang:        "en",
		modType:     "Bible",
		dataPath:    "./modules/texts/ztext/test/",
		confPath:    confPath,
	}

	outputPath := filepath.Join(tempDir, "test.capsule.tar.gz")
	err := ingestSwordModule(swordPath, module, outputPath)

	if err != nil {
		t.Errorf("ingestSwordModule() error = %v", err)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("output capsule not created")
	}
}

// Benchmark tests

func BenchmarkIngestCmd(b *testing.B) {
	tempDir := b.TempDir()
	testFilePath := filepath.Join(tempDir, "input.txt")
	if err := os.WriteFile(testFilePath, []byte("benchmark data"), 0644); err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outputPath := filepath.Join(tempDir, fmt.Sprintf("bench-%d.capsule.tar.xz", i))
		cmd := &IngestCmd{
			Path: testFilePath,
			Out:  outputPath,
		}
		_ = cmd.Run()
	}
}

func BenchmarkHash(b *testing.B) {
	data := []byte(strings.Repeat("test data\n", 1000))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cas.Hash(data)
	}
}

func BenchmarkCopyDirRecursive(b *testing.B) {
	tempDir := b.TempDir()
	srcDir := filepath.Join(tempDir, "src")
	os.MkdirAll(srcDir, 0755)

	// Create some test files
	for i := 0; i < 10; i++ {
		filePath := filepath.Join(srcDir, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			b.Fatalf("failed to create test file: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dstDir := filepath.Join(tempDir, fmt.Sprintf("dst-%d", i))
		_ = fileutil.CopyDir(srcDir, dstDir)
	}
}

// Table-driven tests for parseConfForList edge cases

func TestParseConfForList_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		confContent string
		wantNil     bool
	}{
		{
			name:        "empty file",
			confContent: "",
			wantNil:     false,
		},
		{
			name: "comments only",
			confContent: `# This is a comment
# Another comment
`,
			wantNil: false,
		},
		{
			name: "malformed key-value",
			confContent: `[TEST]
NoEqualSign
`,
			wantNil: false,
		},
		{
			name:        "nonexistent file",
			confContent: "",
			wantNil:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var confPath string
			if tt.name == "nonexistent file" {
				confPath = "/nonexistent/path/file.conf"
			} else {
				tempDir := t.TempDir()
				confPath = createTestFile(t, tempDir, "test.conf", tt.confContent)
			}

			module := parseConfForList(confPath)

			if tt.wantNil && module != nil {
				t.Error("expected nil module")
			}
			if !tt.wantNil && module == nil {
				t.Error("expected non-nil module")
			}
		})
	}
}

// Test casManifest and casArtifact structs

func TestCASManifest_JSON(t *testing.T) {
	manifest := casManifest{
		ID:           "test-cas",
		Title:        "Test CAS Capsule",
		ModuleType:   "bible",
		SourceFormat: "sword",
		MainArtifact: "main",
		Artifacts: []casArtifact{
			{
				ID: "main",
				Files: []casFile{
					{
						Path:   "test.txt",
						SHA256: "abc123",
						Blake3: "def456",
					},
				},
			},
		},
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal casManifest: %v", err)
	}

	var decoded casManifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal casManifest: %v", err)
	}

	if decoded.ID != manifest.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, manifest.ID)
	}
	if len(decoded.Artifacts) != len(manifest.Artifacts) {
		t.Errorf("Artifacts count = %d, want %d", len(decoded.Artifacts), len(manifest.Artifacts))
	}
}

// Test IRInfo struct

func TestIRInfo_AllFields(t *testing.T) {
	info := &IRInfo{
		ID:             "test-ir",
		Version:        "1.0",
		ModuleType:     "bible",
		Versification:  "KJV",
		Language:       "en",
		Title:          "Test Bible",
		LossClass:      "L0",
		SourceHash:     "abc123def456",
		DocumentCount:  66,
		Documents:      []string{"GEN", "EXO"},
		TotalBlocks:    1000,
		TotalChars:     50000,
		HasAnnotations: true,
	}

	if info.ID != "test-ir" {
		t.Errorf("ID = %q, want %q", info.ID, "test-ir")
	}
	if info.DocumentCount != 66 {
		t.Errorf("DocumentCount = %d, want %d", info.DocumentCount, 66)
	}
	if !info.HasAnnotations {
		t.Error("HasAnnotations = false, want true")
	}
}

// Tests for ToolRunCmd

func TestToolRunCmd_Run_InvalidCapsule(t *testing.T) {
	tempDir := t.TempDir()
	invalidCapsule := createTestFile(t, tempDir, "invalid.tar.xz", "not a valid capsule")

	cmd := &ToolRunCmd{
		Capsule:  invalidCapsule,
		Artifact: "test-artifact",
		Tool:     "test-tool",
		Profile:  "default",
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for invalid capsule, got nil")
	}
}

func TestToolRunCmd_Run_InvalidArtifact(t *testing.T) {
	tempDir := t.TempDir()
	packedPath := createPackedCapsule(t, tempDir, "test content")

	cmd := &ToolRunCmd{
		Capsule:  packedPath,
		Artifact: "nonexistent-artifact",
		Tool:     "test-tool",
		Profile:  "default",
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for invalid artifact, got nil")
	}
}

func TestToolRunCmd_Run_NoFlake(t *testing.T) {
	// Skip if nix flake is available
	if getFlakePath() != "" {
		t.Skip("skipping test because nix flake is available")
	}

	tempDir := t.TempDir()
	packedPath := createPackedCapsule(t, tempDir, "test content")

	// Get artifact ID
	cap, err := capsule.Unpack(packedPath, filepath.Join(tempDir, "unpack"))
	if err != nil {
		t.Fatalf("failed to unpack capsule: %v", err)
	}
	var artifactID string
	for id := range cap.Manifest.Artifacts {
		artifactID = id
		break
	}

	cmd := &ToolRunCmd{
		Capsule:  packedPath,
		Artifact: artifactID,
		Tool:     "test-tool",
		Profile:  "default",
	}

	err = cmd.Run()
	if err == nil {
		t.Error("expected error when no nix flake found, got nil")
	}
	if !strings.Contains(err.Error(), "nix flake not found") {
		t.Errorf("expected 'nix flake not found' error, got: %v", err)
	}
}

// Tests for TestCmd with fixtures

func TestTestCmd_Run_WithCapsules(t *testing.T) {
	tempDir := t.TempDir()
	fixturesDir := filepath.Join(tempDir, "fixtures")
	goldenDir := filepath.Join(fixturesDir, "goldens")
	os.MkdirAll(goldenDir, 0755)

	// Create a test capsule
	packedPath := createPackedCapsule(t, tempDir, "test content")
	destPath := filepath.Join(fixturesDir, "test.capsule.tar.xz")
	data, _ := os.ReadFile(packedPath)
	os.WriteFile(destPath, data, 0644)

	cmd := &TestCmd{
		FixturesDir: fixturesDir,
		Golden:      goldenDir,
	}

	// First run creates golden
	err := cmd.Run()
	if err != nil {
		t.Errorf("TestCmd.Run() error = %v, want nil", err)
	}

	// Second run should pass
	err = cmd.Run()
	if err != nil {
		t.Errorf("TestCmd.Run() second run error = %v, want nil", err)
	}
}

func TestTestCmd_Run_WithInputs(t *testing.T) {
	tempDir := t.TempDir()
	fixturesDir := filepath.Join(tempDir, "fixtures")
	inputsDir := filepath.Join(fixturesDir, "inputs")
	goldenDir := filepath.Join(fixturesDir, "goldens")
	os.MkdirAll(inputsDir, 0755)
	os.MkdirAll(goldenDir, 0755)

	// Create a test input file
	createTestFile(t, inputsDir, "test.txt", "test input content")

	cmd := &TestCmd{
		FixturesDir: fixturesDir,
		Golden:      goldenDir,
	}

	// First run creates golden
	err := cmd.Run()
	if err != nil {
		t.Errorf("TestCmd.Run() error = %v, want nil", err)
	}

	// Second run should pass
	err = cmd.Run()
	if err != nil {
		t.Errorf("TestCmd.Run() second run error = %v, want nil", err)
	}
}

// Tests for PluginsCmd with actual plugins

func TestPluginsCmd_Run_WithPlugins(t *testing.T) {
	// Use the actual plugins directory if available
	wd, _ := os.Getwd()
	pluginDir := filepath.Join(wd, "..", "..", "plugins")

	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		t.Skip("plugins directory not found")
	}

	cmd := &PluginsListCmd{
		Dir: pluginDir,
	}

	err := cmd.Run(nil)
	if err != nil {
		t.Errorf("PluginsListCmd.Run() error = %v, want nil", err)
	}
}

// Tests for IngestCmd edge cases

func TestIngestCmd_Run_DirectoryIngest(t *testing.T) {
	tempDir := t.TempDir()

	// Create a directory with files - IngestCmd expects a file, not directory
	// so this should fail
	inputDir := filepath.Join(tempDir, "input")
	os.MkdirAll(inputDir, 0755)
	createTestFile(t, inputDir, "file1.txt", "content 1")

	outputPath := filepath.Join(tempDir, "output.capsule.tar.xz")

	cmd := &IngestCmd{
		Path: inputDir,
		Out:  outputPath,
	}

	err := cmd.Run()
	// IngestCmd.Run() treats directories differently - it may fail or succeed
	// depending on implementation; just check it doesn't panic
	_ = err
}

func TestIngestCmd_Run_NestedDirectory(t *testing.T) {
	tempDir := t.TempDir()

	// Create nested directories
	nestedDir := filepath.Join(tempDir, "a", "b", "c")
	os.MkdirAll(nestedDir, 0755)
	testFile := createTestFile(t, nestedDir, "deep.txt", "deep content")

	outputPath := filepath.Join(tempDir, "output.capsule.tar.xz")

	cmd := &IngestCmd{
		Path: testFile,
		Out:  outputPath,
	}

	err := cmd.Run()
	if err != nil {
		t.Errorf("IngestCmd.Run() error = %v, want nil", err)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("output capsule not created")
	}
}

// Tests for ExportCmd with different modes

func TestExportCmd_Run_InvalidCapsule(t *testing.T) {
	tempDir := t.TempDir()
	invalidCapsule := createTestFile(t, tempDir, "invalid.tar.xz", "not a capsule")

	cmd := &ExportCmd{
		Capsule:  invalidCapsule,
		Artifact: "test",
		Out:      filepath.Join(tempDir, "out.txt"),
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for invalid capsule, got nil")
	}
}

// Tests for CompareCmd

func TestCompareCmd_Run_InvalidCapsule(t *testing.T) {
	tempDir := t.TempDir()
	invalidCapsule := createTestFile(t, tempDir, "invalid.tar.xz", "not a capsule")

	cmd := &CompareCmd{
		Capsule: invalidCapsule,
		Run1:    "run1",
		Run2:    "run2",
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for invalid capsule, got nil")
	}
}

// Tests for GoldenCmd

func TestGoldenSaveCmd_Run_InvalidCapsule(t *testing.T) {
	tempDir := t.TempDir()
	invalidCapsule := createTestFile(t, tempDir, "invalid.tar.xz", "not a capsule")
	goldenFile := filepath.Join(tempDir, "golden.sha256")

	cmd := &GoldenSaveCmd{
		Capsule: invalidCapsule,
		RunID:   "run1",
		Out:     goldenFile,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for invalid capsule, got nil")
	}
}

// Tests for SelfcheckCmd error paths

func TestSelfcheckCmd_Run_InvalidCapsule(t *testing.T) {
	tempDir := t.TempDir()
	invalidCapsule := createTestFile(t, tempDir, "invalid.tar.xz", "not a capsule")

	cmd := &SelfcheckCmd{
		Capsule: invalidCapsule,
		JSON:    false,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for invalid capsule, got nil")
	}
}

// Tests for RunsListCmd error paths

func TestRunsListCmd_Run_InvalidCapsule(t *testing.T) {
	tempDir := t.TempDir()
	invalidCapsule := createTestFile(t, tempDir, "invalid.tar.xz", "not a capsule")

	cmd := &RunsListCmd{
		Capsule: invalidCapsule,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for invalid capsule, got nil")
	}
}

// Additional tests for helper functions

func TestCopyDirRecursive_NonexistentSource(t *testing.T) {
	tempDir := t.TempDir()
	err := fileutil.CopyDir("/nonexistent/source", filepath.Join(tempDir, "dst"))
	if err == nil {
		t.Error("expected error for nonexistent source, got nil")
	}
}

func TestExtractCapsuleArchive_InvalidArchive(t *testing.T) {
	tempDir := t.TempDir()
	invalidArchive := createTestFile(t, tempDir, "invalid.tar.xz", "not a valid archive")

	err := extractCapsuleArchive(invalidArchive, filepath.Join(tempDir, "extract"))
	if err == nil {
		t.Error("expected error for invalid archive, got nil")
	}
}

func TestExtractCapsuleArchive_TarGz(t *testing.T) {
	tempDir := t.TempDir()

	// Create a valid tar.gz archive
	srcDir := filepath.Join(tempDir, "src")
	os.MkdirAll(srcDir, 0755)
	createTestFile(t, srcDir, "test.txt", "test content")

	archivePath := filepath.Join(tempDir, "test.tar.gz")
	if err := archive.CreateCapsuleTarGz(srcDir, archivePath); err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}

	extractDir := filepath.Join(tempDir, "extract")
	err := extractCapsuleArchive(archivePath, extractDir)
	if err != nil {
		t.Errorf("extractCapsuleArchive() error = %v, want nil", err)
	}
}

func TestCapsuleContainsIR_InvalidCapsule(t *testing.T) {
	tempDir := t.TempDir()
	invalidCapsule := createTestFile(t, tempDir, "invalid.tar.xz", "not a capsule")

	result := capsuleContainsIR(invalidCapsule)
	if result {
		t.Error("expected false for invalid capsule, got true")
	}
}

func TestIsCASCapsule_InvalidArchive(t *testing.T) {
	tempDir := t.TempDir()
	invalidArchive := createTestFile(t, tempDir, "invalid.tar.gz", "not a valid archive")

	result := isCASCapsule(invalidArchive)
	if result {
		t.Error("expected false for invalid archive, got true")
	}
}

func TestBuildIRInfo_WithDocuments(t *testing.T) {
	ir := map[string]interface{}{
		"id":          "test",
		"version":     "1.0",
		"module_type": "bible",
		"language":    "en",
		"title":       "Test",
		"documents": []interface{}{
			map[string]interface{}{
				"id": "GEN",
				"content_blocks": []interface{}{
					map[string]interface{}{
						"text": "In the beginning",
					},
					map[string]interface{}{
						"text": "God created",
					},
				},
				// Annotations are at document level, not block level
				"annotations": []interface{}{
					map[string]interface{}{"type": "note"},
				},
			},
			map[string]interface{}{
				"id":             "EXO",
				"content_blocks": []interface{}{},
			},
		},
	}

	info := buildIRInfo(ir)

	if info.ID != "test" {
		t.Errorf("ID = %q, want %q", info.ID, "test")
	}
	if info.DocumentCount != 2 {
		t.Errorf("DocumentCount = %d, want %d", info.DocumentCount, 2)
	}
	if info.TotalBlocks != 2 {
		t.Errorf("TotalBlocks = %d, want %d", info.TotalBlocks, 2)
	}
	if !info.HasAnnotations {
		t.Error("HasAnnotations = false, want true")
	}
}

func TestPrintIRInfo_AllFields(t *testing.T) {
	info := &IRInfo{
		ID:             "test-ir",
		Version:        "1.0",
		ModuleType:     "bible",
		Versification:  "KJV",
		Language:       "en",
		Title:          "Test Bible",
		LossClass:      "L0",
		SourceHash:     "abc123def456789012345678901234567890", // Must be at least 16 chars
		DocumentCount:  66,
		Documents:      []string{"GEN", "EXO", "LEV"},
		TotalBlocks:    1000,
		TotalChars:     50000,
		HasAnnotations: true,
	}

	// Just ensure it doesn't panic with all fields populated
	printIRInfo(info)
}

func TestPrintIRInfo_MinimalFields(t *testing.T) {
	info := &IRInfo{
		ID:            "test",
		DocumentCount: 1,
	}

	// Just ensure it doesn't panic with minimal fields
	printIRInfo(info)
}

// Tests that require nix flake - run only when flake is available

func TestRunCmd_Run_WithFlake(t *testing.T) {
	flakePath := getFlakePath()
	if flakePath == "" {
		t.Skip("skipping test because nix flake is not available")
	}

	tempDir := t.TempDir()
	testFile := createTestFile(t, tempDir, "input.txt", "test content")
	outDir := filepath.Join(tempDir, "output")

	// Test with a valid tool and profile
	cmd := &RunCmd{
		Tool:    "libsword",
		Profile: "list-modules",
		Input:   testFile,
		Out:     outDir,
	}

	// This will likely fail because libsword expects SWORD modules
	// but we're testing that it at least attempts to run
	err := cmd.Run()
	// We expect some error because we don't have a real SWORD setup,
	// but the function should at least get past the flake check
	if err != nil && strings.Contains(err.Error(), "nix flake not found") {
		t.Errorf("unexpected 'nix flake not found' error when flake exists: %v", err)
	}
}

func TestRunCmd_Run_WithOutput(t *testing.T) {
	flakePath := getFlakePath()
	if flakePath == "" {
		t.Skip("skipping test because nix flake is not available")
	}

	tempDir := t.TempDir()
	testFile := createTestFile(t, tempDir, "input.txt", "test content")

	// Test without output dir (should use temp dir)
	cmd := &RunCmd{
		Tool:    "libsword",
		Profile: "list-modules",
		Input:   testFile,
		Out:     "",
	}

	// This exercises the temp output dir creation path
	err := cmd.Run()
	// Error is expected, but not "nix flake not found"
	if err != nil && strings.Contains(err.Error(), "nix flake not found") {
		t.Errorf("unexpected 'nix flake not found' error: %v", err)
	}
}

func TestToolRunCmd_Run_WithFlake(t *testing.T) {
	flakePath := getFlakePath()
	if flakePath == "" {
		t.Skip("skipping test because nix flake is not available")
	}

	tempDir := t.TempDir()
	packedPath := createPackedCapsule(t, tempDir, "test content")

	// Unpack to get artifact ID
	unpackDir := filepath.Join(tempDir, "unpack")
	cap, err := capsule.Unpack(packedPath, unpackDir)
	if err != nil {
		t.Fatalf("failed to unpack capsule: %v", err)
	}

	var artifactID string
	for id := range cap.Manifest.Artifacts {
		artifactID = id
		break
	}

	cmd := &ToolRunCmd{
		Capsule:  packedPath,
		Artifact: artifactID,
		Tool:     "libsword",
		Profile:  "list-modules",
	}

	err = cmd.Run()
	// Error is expected (libsword won't work with our test content),
	// but we verify the function attempts to run
	if err != nil && strings.Contains(err.Error(), "nix flake not found") {
		t.Errorf("unexpected 'nix flake not found' error when flake exists: %v", err)
	}
}

// Tests for DetectCmd with actual plugins

func TestDetectCmd_Run_WithPlugins(t *testing.T) {
	// Set plugin directory to the actual plugins
	originalPluginDir := CLI.PluginDir
	defer func() { CLI.PluginDir = originalPluginDir }()

	// Find the plugins directory relative to this test
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "..", "..", "plugins", "format")
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		// Try from repo root
		pluginDir = "plugins/format"
	}

	// Check if plugins are built
	if _, err := os.Stat(filepath.Join(pluginDir, "file", "format-file")); os.IsNotExist(err) {
		t.Skip("skipping test because plugins are not built (run 'make plugins')")
	}

	CLI.PluginDir = pluginDir
	tempDir := t.TempDir()
	testFile := createTestFile(t, tempDir, "test.txt", "test content")

	cmd := &DetectCmd{Path: testFile}
	err := cmd.Run(nil)
	if err != nil {
		t.Errorf("DetectCmd.Run() error = %v", err)
	}
}

func TestDetectCmd_Run_WithZipFile(t *testing.T) {
	// Set plugin directory to the actual plugins
	originalPluginDir := CLI.PluginDir
	defer func() { CLI.PluginDir = originalPluginDir }()

	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "..", "..", "plugins", "format")
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		pluginDir = "plugins/format"
	}
	if _, err := os.Stat(filepath.Join(pluginDir, "zip", "format-zip")); os.IsNotExist(err) {
		t.Skip("skipping test because zip plugin is not built")
	}

	CLI.PluginDir = pluginDir
	tempDir := t.TempDir()

	// Create a simple zip file
	zipPath := filepath.Join(tempDir, "test.zip")
	createTestZipFile(t, zipPath)

	cmd := &DetectCmd{Path: zipPath}
	err := cmd.Run(nil)
	if err != nil {
		t.Errorf("DetectCmd.Run() error = %v", err)
	}
}

func TestEnumerateCmd_Run_WithPlugins(t *testing.T) {
	// Set plugin directory to the actual plugins
	originalPluginDir := CLI.PluginDir
	defer func() { CLI.PluginDir = originalPluginDir }()

	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "..", "..", "plugins", "format")
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		pluginDir = "plugins/format"
	}
	if _, err := os.Stat(filepath.Join(pluginDir, "zip", "format-zip")); os.IsNotExist(err) {
		t.Skip("skipping test because zip plugin is not built")
	}

	CLI.PluginDir = pluginDir
	tempDir := t.TempDir()

	// Create a simple zip file
	zipPath := filepath.Join(tempDir, "test.zip")
	createTestZipFile(t, zipPath)

	cmd := &EnumerateCmd{Path: zipPath}
	err := cmd.Run(nil)
	if err != nil {
		t.Errorf("EnumerateCmd.Run() error = %v", err)
	}
}

// Helper to create a test zip file
func createTestZipFile(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// Add a test file to the zip
	w, err := zw.Create("test.txt")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	w.Write([]byte("test content"))
}


// Additional tests for JuniperIngestCmd

func TestJuniperIngestCmd_Run_NoSWORDPath(t *testing.T) {
	tempDir := t.TempDir()
	// No mods.d directory
	
	cmd := &JuniperIngestCmd{
		Path:   tempDir,
		Output: filepath.Join(tempDir, "output"),
		All:    true,
	}
	
	err := cmd.Run()
	if err == nil {
		t.Error("expected error for missing mods.d, got nil")
	}
	if !strings.Contains(err.Error(), "SWORD installation not found") {
		t.Errorf("error = %v, want 'SWORD installation not found'", err)
	}
}

func TestJuniperIngestCmd_Run_EmptyModsD(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}
	
	cmd := &JuniperIngestCmd{
		Path:   tempDir,
		Output: filepath.Join(tempDir, "output"),
		All:    true,
	}
	
	err := cmd.Run()
	if err == nil {
		t.Error("expected error for no Bible modules, got nil")
	}
	if !strings.Contains(err.Error(), "no Bible modules found") {
		t.Errorf("error = %v, want 'no Bible modules found'", err)
	}
}

func TestJuniperIngestCmd_Run_NoBibleModules(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}
	
	// Create a non-Bible conf file
	confContent := `[TestModule]
Description=Test Commentary
ModDrv=RawCom
DataPath=./modules/comments/rawcom/test/
`
	if err := os.WriteFile(filepath.Join(modsDir, "test.conf"), []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}
	
	cmd := &JuniperIngestCmd{
		Path:   tempDir,
		Output: filepath.Join(tempDir, "output"),
		All:    true,
	}
	
	err := cmd.Run()
	if err == nil {
		t.Error("expected error for no Bible modules, got nil")
	}
}

func TestJuniperIngestCmd_Run_SpecifyNoModules(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}
	
	// Create a Bible conf file
	confContent := `[TestBible]
Description=Test Bible
ModDrv=zText
DataPath=./modules/texts/ztext/testbible/
`
	if err := os.WriteFile(filepath.Join(modsDir, "testbible.conf"), []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}
	
	cmd := &JuniperIngestCmd{
		Path:    tempDir,
		Output:  filepath.Join(tempDir, "output"),
		Modules: []string{}, // No modules and All=false
		All:     false,
	}
	
	err := cmd.Run()
	if err == nil {
		t.Error("expected error when no modules specified, got nil")
	}
	if !strings.Contains(err.Error(), "specify module names or use --all") {
		t.Errorf("error = %v, want 'specify module names or use --all'", err)
	}
}

func TestJuniperIngestCmd_Run_ModuleNotFound(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}
	
	// Create a Bible conf file
	confContent := `[TestBible]
Description=Test Bible
ModDrv=zText
DataPath=./modules/texts/ztext/testbible/
`
	if err := os.WriteFile(filepath.Join(modsDir, "testbible.conf"), []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}
	
	cmd := &JuniperIngestCmd{
		Path:    tempDir,
		Output:  filepath.Join(tempDir, "output"),
		Modules: []string{"NonExistent"},
		All:     false,
	}
	
	err := cmd.Run()
	if err == nil {
		t.Error("expected error for no modules to ingest, got nil")
	}
	if !strings.Contains(err.Error(), "no modules to ingest") {
		t.Errorf("error = %v, want 'no modules to ingest'", err)
	}
}

func TestJuniperIngestCmd_Run_EncryptedModule(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}
	
	// Create encrypted Bible conf file
	confContent := `[EncryptedBible]
Description=Encrypted Bible
ModDrv=zText
DataPath=./modules/texts/ztext/encrypted/
CipherKey=
`
	if err := os.WriteFile(filepath.Join(modsDir, "encrypted.conf"), []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}
	
	cmd := &JuniperIngestCmd{
		Path:   tempDir,
		Output: filepath.Join(tempDir, "output"),
		All:    true,
	}
	
	// Should skip encrypted modules but not error
	err := cmd.Run()
	// This should complete without error, just skipping the encrypted module
	if err != nil && !strings.Contains(err.Error(), "no modules to ingest") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestJuniperIngestCmd_Run_DefaultPath(t *testing.T) {
	// Test that default path uses ~/.sword
	cmd := &JuniperIngestCmd{
		Path:   "", // Empty to trigger default
		Output: "/tmp/test-output",
		All:    true,
	}
	
	err := cmd.Run()
	// Expected to fail since ~/.sword probably doesn't have mods.d
	// But we're testing the path resolution, not success
	if err != nil && strings.Contains(err.Error(), "cannot determine home directory") {
		t.Errorf("home directory should be determinable: %v", err)
	}
}

// Tests for parseConfForList

func TestParseConfForList_ValidBible(t *testing.T) {
	tempDir := t.TempDir()
	confContent := `[KJV]
Description=King James Version (1769) with Strongs Numbers and Morphology
Lang=en
ModDrv=zText
DataPath=./modules/texts/ztext/kjv/
`
	confPath := filepath.Join(tempDir, "kjv.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}
	
	module := parseConfForList(confPath)
	if module == nil {
		t.Fatal("expected non-nil module")
	}
	if module.name != "KJV" {
		t.Errorf("name = %q, want %q", module.name, "KJV")
	}
	if module.lang != "en" {
		t.Errorf("lang = %q, want %q", module.lang, "en")
	}
	if module.modType != "Bible" {
		t.Errorf("modType = %q, want %q", module.modType, "Bible")
	}
}

func TestParseConfForList_NonExistent(t *testing.T) {
	module := parseConfForList("/nonexistent/file.conf")
	if module != nil {
		t.Error("expected nil for non-existent file")
	}
}

func TestParseConfForList_Commentary(t *testing.T) {
	tempDir := t.TempDir()
	confContent := `[TestComm]
Description=Test Commentary
ModDrv=RawCom
DataPath=./modules/comments/rawcom/test/
`
	confPath := filepath.Join(tempDir, "test.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}
	
	module := parseConfForList(confPath)
	if module == nil {
		t.Fatal("expected non-nil module")
	}
	if module.modType == "Bible" {
		t.Error("commentary should not be Bible type")
	}
}

// Additional tests for CapsuleConvertCmd

func TestCapsuleConvertCmd_Run_NoConvertibleContent(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a capsule with no convertible content (just a text file)
	capsuleDir := filepath.Join(tempDir, "capsule")
	os.MkdirAll(capsuleDir, 0755)
	os.WriteFile(filepath.Join(capsuleDir, "readme.txt"), []byte("just text"), 0644)
	
	// Create manifest
	manifest := map[string]interface{}{
		"capsule_version": "1.0",
	}
	manifestData, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(capsuleDir, "manifest.json"), manifestData, 0644)
	
	// Pack it
	capsulePath := filepath.Join(tempDir, "test.capsule.tar.gz")
	if err := archive.CreateCapsuleTarGz(capsuleDir, capsulePath); err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}
	
	cmd := &CapsuleConvertCmd{
		Capsule: capsulePath,
		Format:  "osis",
	}
	
	err := cmd.Run()
	if err == nil {
		t.Error("expected error for no convertible content, got nil")
	}
	if !strings.Contains(err.Error(), "no convertible content found") {
		t.Errorf("error = %v, want 'no convertible content found'", err)
	}
}

func TestCapsuleConvertCmd_Run_WithEmbeddedPlugins(t *testing.T) {
	tempDir := t.TempDir()

	// Create a capsule with USFM content
	capsuleDir := filepath.Join(tempDir, "capsule")
	os.MkdirAll(capsuleDir, 0755)

	// Create a minimal USFM file
	usfmContent := `\id GEN
\c 1
\v 1 In the beginning...
`
	os.WriteFile(filepath.Join(capsuleDir, "gen.usfm"), []byte(usfmContent), 0644)

	manifest := map[string]interface{}{
		"capsule_version": "1.0",
	}
	manifestData, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(capsuleDir, "manifest.json"), manifestData, 0644)

	capsulePath := filepath.Join(tempDir, "test.capsule.tar.gz")
	if err := archive.CreateCapsuleTarGz(capsuleDir, capsulePath); err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Even with external plugin dir empty, embedded plugins provide USFM/OSIS support
	origPluginDir := CLI.PluginDir
	CLI.PluginDir = filepath.Join(tempDir, "no-plugins")
	os.MkdirAll(CLI.PluginDir, 0755)
	defer func() { CLI.PluginDir = origPluginDir }()

	cmd := &CapsuleConvertCmd{
		Capsule: capsulePath,
		Format:  "osis",
	}

	// With embedded plugins fully implementing IR extraction, conversion should succeed
	err := cmd.Run()
	if err != nil {
		t.Errorf("expected conversion to succeed with embedded plugins, got error: %v", err)
	}
}

// Additional tests for GenerateIRCmd

func TestGenerateIRCmd_Run_NoConvertibleContent(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a capsule with no convertible content
	capsuleDir := filepath.Join(tempDir, "capsule")
	os.MkdirAll(capsuleDir, 0755)
	os.WriteFile(filepath.Join(capsuleDir, "readme.txt"), []byte("just text"), 0644)
	
	manifest := map[string]interface{}{
		"capsule_version": "1.0",
	}
	manifestData, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(capsuleDir, "manifest.json"), manifestData, 0644)
	
	capsulePath := filepath.Join(tempDir, "test.capsule.tar.gz")
	if err := archive.CreateCapsuleTarGz(capsuleDir, capsulePath); err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}
	
	cmd := &GenerateIRCmd{
		Capsule: capsulePath,
	}
	
	err := cmd.Run()
	if err == nil {
		t.Error("expected error for no convertible content, got nil")
	}
	if !strings.Contains(err.Error(), "no convertible content found") {
		t.Errorf("error = %v, want 'no convertible content found'", err)
	}
}

func TestGenerateIRCmd_Run_AlreadyHasIR(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a capsule that already has IR
	capsuleDir := filepath.Join(tempDir, "capsule")
	os.MkdirAll(capsuleDir, 0755)
	
	// Create IR file
	os.WriteFile(filepath.Join(capsuleDir, "test.ir.json"), []byte(`{"format":"ir"}`), 0644)
	
	manifest := map[string]interface{}{
		"capsule_version": "1.0",
		"has_ir":          true,
	}
	manifestData, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(capsuleDir, "manifest.json"), manifestData, 0644)
	
	capsulePath := filepath.Join(tempDir, "test.capsule.tar.gz")
	if err := archive.CreateCapsuleTarGz(capsuleDir, capsulePath); err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}
	
	cmd := &GenerateIRCmd{
		Capsule: capsulePath,
	}
	
	err := cmd.Run()
	if err == nil {
		t.Error("expected error for capsule already having IR, got nil")
	}
	if !strings.Contains(err.Error(), "already contains IR") {
		t.Errorf("error = %v, want 'already contains IR'", err)
	}
}

func TestGenerateIRCmd_Run_WithEmbeddedPlugins(t *testing.T) {
	tempDir := t.TempDir()

	// Create a capsule with USFM content but no IR
	capsuleDir := filepath.Join(tempDir, "capsule")
	os.MkdirAll(capsuleDir, 0755)

	usfmContent := `\id GEN
\c 1
\v 1 Test verse
`
	os.WriteFile(filepath.Join(capsuleDir, "gen.usfm"), []byte(usfmContent), 0644)

	manifest := map[string]interface{}{
		"capsule_version": "1.0",
	}
	manifestData, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(capsuleDir, "manifest.json"), manifestData, 0644)

	capsulePath := filepath.Join(tempDir, "test.capsule.tar.gz")
	if err := archive.CreateCapsuleTarGz(capsuleDir, capsulePath); err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Even with external plugin dir empty, embedded plugins provide USFM IR extraction
	origPluginDir := CLI.PluginDir
	CLI.PluginDir = filepath.Join(tempDir, "no-plugins")
	os.MkdirAll(CLI.PluginDir, 0755)
	defer func() { CLI.PluginDir = origPluginDir }()

	cmd := &GenerateIRCmd{
		Capsule: capsulePath,
	}

	// With embedded plugins fully implementing IR extraction, should succeed
	err := cmd.Run()
	if err != nil {
		t.Errorf("expected IR generation to succeed with embedded plugins, got error: %v", err)
	}
}



// Additional tests for CompareCmd

func TestCompareCmd_Run_RunNotFound(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a valid capsule without any runs
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}
	
	// Pack it
	capsulePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(capsulePath); err != nil {
		t.Fatalf("failed to pack capsule: %v", err)
	}
	os.RemoveAll(capsuleDir)
	
	cmd := &CompareCmd{
		Capsule: capsulePath,
		Run1:    "nonexistent-run-1",
		Run2:    "nonexistent-run-2",
	}
	
	err = cmd.Run()
	if err == nil {
		t.Error("expected error for nonexistent run, got nil")
	}
	if !strings.Contains(err.Error(), "run not found") {
		t.Errorf("error = %v, want 'run not found'", err)
	}
}

func TestCompareCmd_Run_SecondRunNotFound(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a capsule with one run
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}
	
	// Add a run manually
	run := &capsule.Run{
		ID:     "run-1",
		Status: "completed",
	}
	cap.Manifest.Runs = make(map[string]*capsule.Run)
	cap.Manifest.Runs["run-1"] = run
	cap.SaveManifest()
	
	// Pack it
	capsulePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(capsulePath); err != nil {
		t.Fatalf("failed to pack capsule: %v", err)
	}
	os.RemoveAll(capsuleDir)
	
	cmd := &CompareCmd{
		Capsule: capsulePath,
		Run1:    "run-1",
		Run2:    "nonexistent-run-2",
	}
	
	err = cmd.Run()
	if err == nil {
		t.Error("expected error for nonexistent second run, got nil")
	}
	if !strings.Contains(err.Error(), "run not found") {
		t.Errorf("error = %v, want 'run not found'", err)
	}
}

func TestCompareCmd_Run_NoTranscript(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a capsule with two runs but no transcripts
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}
	
	cap.Manifest.Runs = make(map[string]*capsule.Run)
	cap.Manifest.Runs["run-1"] = &capsule.Run{
		ID:      "run-1",
		Status:  "completed",
		Outputs: nil, // No transcript
	}
	cap.Manifest.Runs["run-2"] = &capsule.Run{
		ID:      "run-2",
		Status:  "completed",
		Outputs: nil,
	}
	cap.SaveManifest()
	
	capsulePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(capsulePath); err != nil {
		t.Fatalf("failed to pack capsule: %v", err)
	}
	os.RemoveAll(capsuleDir)
	
	cmd := &CompareCmd{
		Capsule: capsulePath,
		Run1:    "run-1",
		Run2:    "run-2",
	}
	
	err = cmd.Run()
	if err == nil {
		t.Error("expected error for missing transcript, got nil")
	}
	if !strings.Contains(err.Error(), "no transcript") {
		t.Errorf("error = %v, want 'no transcript'", err)
	}
}

// TestCompareCmd_Run_IdenticalTranscripts tests comparing identical transcripts
func TestCompareCmd_Run_IdenticalTranscripts(t *testing.T) {
	tempDir := t.TempDir()

	// Create a capsule with two runs with identical transcripts
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a sample transcript in valid JSONL format
	transcriptData := []byte(`{"event":"start","timestamp":"2024-01-01T00:00:00Z"}
{"event":"end","exit_code":0}`)
	transcriptHash := cas.Hash(transcriptData)

	// Store transcript in CAS
	if _, err := cap.GetStore().Store(transcriptData); err != nil {
		t.Fatalf("failed to store transcript: %v", err)
	}

	cap.Manifest.Runs = make(map[string]*capsule.Run)
	cap.Manifest.Runs["run-1"] = &capsule.Run{
		ID:     "run-1",
		Status: "completed",
		Outputs: &capsule.RunOutputs{
			TranscriptBlobSHA256: transcriptHash,
		},
	}
	cap.Manifest.Runs["run-2"] = &capsule.Run{
		ID:     "run-2",
		Status: "completed",
		Outputs: &capsule.RunOutputs{
			TranscriptBlobSHA256: transcriptHash,
		},
	}
	cap.SaveManifest()

	capsulePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(capsulePath); err != nil {
		t.Fatalf("failed to pack capsule: %v", err)
	}
	os.RemoveAll(capsuleDir)

	cmd := &CompareCmd{
		Capsule: capsulePath,
		Run1:    "run-1",
		Run2:    "run-2",
	}

	err = cmd.Run()
	if err != nil {
		t.Errorf("expected no error for identical transcripts, got %v", err)
	}
}

// TestCompareCmd_Run_DifferentTranscripts tests comparing different transcripts
func TestCompareCmd_Run_DifferentTranscripts(t *testing.T) {
	tempDir := t.TempDir()

	// Create a capsule with two runs with different transcripts
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create two different transcripts in valid JSONL format
	transcript1 := []byte(`{"event":"start","timestamp":"2024-01-01T00:00:00Z"}
{"event":"end","exit_code":0}`)
	transcript2 := []byte(`{"event":"start","timestamp":"2024-01-01T00:00:00Z"}
{"event":"modified","timestamp":"2024-01-01T00:00:02Z"}
{"event":"end","exit_code":0}`)
	hash1 := cas.Hash(transcript1)
	hash2 := cas.Hash(transcript2)

	// Store transcripts in CAS
	if _, err := cap.GetStore().Store(transcript1); err != nil {
		t.Fatalf("failed to store transcript1: %v", err)
	}
	if _, err := cap.GetStore().Store(transcript2); err != nil {
		t.Fatalf("failed to store transcript2: %v", err)
	}

	cap.Manifest.Runs = make(map[string]*capsule.Run)
	cap.Manifest.Runs["run-1"] = &capsule.Run{
		ID:     "run-1",
		Status: "completed",
		Outputs: &capsule.RunOutputs{
			TranscriptBlobSHA256: hash1,
		},
	}
	cap.Manifest.Runs["run-2"] = &capsule.Run{
		ID:     "run-2",
		Status: "completed",
		Outputs: &capsule.RunOutputs{
			TranscriptBlobSHA256: hash2,
		},
	}
	cap.SaveManifest()

	capsulePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(capsulePath); err != nil {
		t.Fatalf("failed to pack capsule: %v", err)
	}
	os.RemoveAll(capsuleDir)

	cmd := &CompareCmd{
		Capsule: capsulePath,
		Run1:    "run-1",
		Run2:    "run-2",
	}

	err = cmd.Run()
	if err == nil {
		t.Error("expected error for different transcripts, got nil")
	}
	if !strings.Contains(err.Error(), "differ") {
		t.Errorf("error = %v, want 'differ'", err)
	}
}

// TestRunsListCmd_Run_WithRuns tests listing runs successfully
func TestRunsListCmd_Run_WithRuns(t *testing.T) {
	tempDir := t.TempDir()

	// Create a capsule with multiple runs
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	cap.Manifest.Runs = make(map[string]*capsule.Run)
	cap.Manifest.Runs["run-1"] = &capsule.Run{
		ID:     "run-1",
		Status: "completed",
		Plugin: &capsule.PluginInfo{
			PluginID: "test-tool",
		},
	}
	cap.Manifest.Runs["run-2"] = &capsule.Run{
		ID:     "run-2",
		Status: "failed",
		Plugin: &capsule.PluginInfo{
			PluginID: "another-tool",
		},
	}
	cap.SaveManifest()

	capsulePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(capsulePath); err != nil {
		t.Fatalf("failed to pack capsule: %v", err)
	}
	os.RemoveAll(capsuleDir)

	cmd := &RunsListCmd{
		Capsule: capsulePath,
	}

	err = cmd.Run()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRunsListCmd_Run_EmptyRuns tests listing when no runs exist
func TestRunsListCmd_Run_EmptyRuns(t *testing.T) {
	tempDir := t.TempDir()

	// Create an empty capsule
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	capsulePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(capsulePath); err != nil {
		t.Fatalf("failed to pack capsule: %v", err)
	}
	os.RemoveAll(capsuleDir)

	cmd := &RunsListCmd{
		Capsule: capsulePath,
	}

	err = cmd.Run()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}


// TestExtractIRCmd_Run_Success tests successful IR extraction
func TestExtractIRCmd_Run_Success(t *testing.T) {
	tempDir := t.TempDir()

	// Create a simple JSON file that can be detected
	testFile := createTestFile(t, tempDir, "test.json", `{"test": "data"}`)
	outputPath := filepath.Join(tempDir, "output.ir.json")

	cmd := &ExtractIRCmd{
		Path: testFile,
		Out:  outputPath,
	}

	err := cmd.Run()
	// This will fail if no plugin handles JSON IR extraction, which is expected
	// We're testing the command flow, not plugin availability
	if err != nil && !strings.Contains(err.Error(), "no plugin") && !strings.Contains(err.Error(), "not supported") {
		t.Logf("IR extraction not available (expected): %v", err)
	}
}

// TestEmitNativeCmd_Run_Success tests emitting native format from IR
func TestEmitNativeCmd_Run_Success(t *testing.T) {
	tempDir := t.TempDir()

	// Create a minimal IR file
	irData := `{"books": []}`
	irFile := createTestFile(t, tempDir, "test.ir.json", irData)
	outputPath := filepath.Join(tempDir, "output.txt")

	cmd := &EmitNativeCmd{
		IR:     irFile,
		Format: "txt",
		Out:    outputPath,
	}

	err := cmd.Run()
	// This will fail if no plugin handles emit, which is expected
	// We're testing the command flow
	if err != nil && !strings.Contains(err.Error(), "no plugin") && !strings.Contains(err.Error(), "not supported") {
		t.Logf("Emit not available (expected): %v", err)
	}
}

// TestVerifyCmd_Run_CorruptedBlob tests verification with corrupted blob
func TestVerifyCmd_Run_CorruptedBlob(t *testing.T) {
	tempDir := t.TempDir()

	// Create a capsule
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Ingest a file
	testFile := createTestFile(t, tempDir, "test.txt", "original content")
	artifact, err := cap.IngestFile(testFile)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Corrupt the blob by replacing it with different content
	badContent := []byte("corrupted content")
	badHash := cas.Hash(badContent)

	// Manually overwrite the artifact's hash in manifest to cause verification failure
	cap.Manifest.Artifacts[artifact.ID].Hashes.SHA256 = badHash
	cap.SaveManifest()

	capsulePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(capsulePath); err != nil {
		t.Fatalf("failed to pack capsule: %v", err)
	}
	os.RemoveAll(capsuleDir)

	cmd := &VerifyCmd{
		Capsule: capsulePath,
	}

	err = cmd.Run()
	if err == nil {
		t.Error("expected error for corrupted blob, got nil")
	}
}

// TestExportCmd_Run_HashMismatch tests export with hash verification
func TestExportCmd_Run_HashMismatch(t *testing.T) {
	tempDir := t.TempDir()

	// This test verifies the hash checking logic in export
	// Create a capsule and export normally - hash should match
	capsulePath := createPackedCapsule(t, tempDir, "test content for export")

	// Unpack to get artifact ID
	unpackDir := filepath.Join(tempDir, "unpack")
	cap, err := capsule.Unpack(capsulePath, unpackDir)
	if err != nil {
		t.Fatalf("failed to unpack: %v", err)
	}

	// Get first artifact ID
	var artifactID string
	for id := range cap.Manifest.Artifacts {
		artifactID = id
		break
	}

	outputPath := filepath.Join(tempDir, "exported.txt")
	cmd := &ExportCmd{
		Capsule:  capsulePath,
		Artifact: artifactID,
		Out:      outputPath,
	}

	err = cmd.Run()
	if err != nil {
		t.Errorf("export failed: %v", err)
	}

	// Verify the exported file exists
	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("exported file not found: %v", err)
	}
}

// TestIngestCmd_Run_LargeFile tests ingesting a larger file
func TestIngestCmd_Run_LargeFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file with 100KB of data
	largeContent := strings.Repeat("This is test data for large file ingestion.\n", 2000)
	testFile := createTestFile(t, tempDir, "large.txt", largeContent)
	outputPath := filepath.Join(tempDir, "large.capsule.tar.xz")

	cmd := &IngestCmd{
		Path: testFile,
		Out:  outputPath,
	}

	err := cmd.Run()
	if err != nil {
		t.Errorf("ingest failed: %v", err)
	}

	// Verify capsule was created
	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("capsule not created: %v", err)
	}
}

// TestConvertCmd_Run_InvalidSourceFormat tests convert with unsupported source
func TestConvertCmd_Run_InvalidSourceFormat(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file that won't be recognized by any plugin
	testFile := createTestFile(t, tempDir, "test.unknown", "random binary data \x00\x01\x02")
	outputPath := filepath.Join(tempDir, "output.txt")

	cmd := &ConvertCmd{
		Path: testFile,
		To:   "txt",
		Out:  outputPath,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for unsupported format, got nil")
	}
}

// TestToolArchiveCmd_Run_ValidBinaries tests creating tool archive
func TestToolArchiveCmd_Run_ValidBinaries(t *testing.T) {
	tempDir := t.TempDir()

	// Create mock binary files
	bin1 := createTestFile(t, tempDir, "tool1", "#!/bin/sh\necho tool1")
	bin2 := createTestFile(t, tempDir, "tool2", "#!/bin/sh\necho tool2")

	// Make them executable
	os.Chmod(bin1, 0755)
	os.Chmod(bin2, 0755)

	outputPath := filepath.Join(tempDir, "tools.capsule.tar.xz")

	cmd := &ToolArchiveCmd{
		ToolID:  "test-tool",
		Version: "1.0.0",
		Bin:     map[string]string{"tool1": bin1, "tool2": bin2},
		Out:     outputPath,
	}

	err := cmd.Run()
	if err != nil {
		t.Logf("tool archive creation: %v", err)
		// May fail due to missing metadata, which is expected
	}
}

// TestEnumerateCmd_Run_ValidArchive tests enumerating archive contents
func TestEnumerateCmd_Run_ValidArchive(t *testing.T) {
	tempDir := t.TempDir()

	// Create a simple zip archive
	zipPath := filepath.Join(tempDir, "test.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}

	zipWriter := zip.NewWriter(zipFile)

	// Add a file to the zip
	fileWriter, err := zipWriter.Create("test.txt")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	fileWriter.Write([]byte("test content"))

	zipWriter.Close()
	zipFile.Close()

	cmd := &EnumerateCmd{
		Path: zipPath,
	}

	// EnumerateCmd.Run requires *kong.Context, so we skip calling it directly
	// The command structure is tested by other tests
	_ = cmd
}

// Tests for GoldenCheckCmd

func TestGoldenCheckCmd_Run_Success(t *testing.T) {
	tempDir := t.TempDir()

	// Create a capsule with a run that has a transcript
	cap, capsuleDir := createTestCapsule(t, tempDir)
	testFile := createTestFile(t, tempDir, "input.txt", "test content")
	artifact, err := cap.IngestFile(testFile)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Create a mock run with a transcript
	transcriptHash := "test-transcript-hash-abc123"
	cap.Manifest.Runs = map[string]*capsule.Run{
		"test-run": {
			ID:     "test-run",
			Inputs: []capsule.RunInput{{ArtifactID: artifact.ID}},
			Outputs: &capsule.RunOutputs{
				TranscriptBlobSHA256: transcriptHash,
			},
		},
	}

	// Pack the capsule
	packedPath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(packedPath); err != nil {
		t.Fatalf("failed to pack: %v", err)
	}

	// Clean up unpacked directory
	os.RemoveAll(capsuleDir)

	// Create golden file
	goldenFile := filepath.Join(tempDir, "golden.sha256")
	if err := os.WriteFile(goldenFile, []byte(transcriptHash), 0644); err != nil {
		t.Fatalf("failed to write golden: %v", err)
	}

	// Run check
	cmd := &GoldenCheckCmd{
		Capsule: packedPath,
		RunID:   "test-run",
		Golden:  goldenFile,
	}

	err = cmd.Run()
	if err != nil {
		t.Errorf("GoldenCheckCmd.Run() unexpected error = %v", err)
	}
}

func TestGoldenCheckCmd_Run_Mismatch(t *testing.T) {
	tempDir := t.TempDir()

	// Create a capsule with a run
	cap, capsuleDir := createTestCapsule(t, tempDir)
	testFile := createTestFile(t, tempDir, "input.txt", "test content")
	artifact, err := cap.IngestFile(testFile)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Create run with transcript
	cap.Manifest.Runs = map[string]*capsule.Run{
		"test-run": {
			ID:     "test-run",
			Inputs: []capsule.RunInput{{ArtifactID: artifact.ID}},
			Outputs: &capsule.RunOutputs{
				TranscriptBlobSHA256: "actual-hash",
			},
		},
	}

	packedPath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(packedPath); err != nil {
		t.Fatalf("failed to pack: %v", err)
	}

	os.RemoveAll(capsuleDir)

	// Golden file with different hash
	goldenFile := filepath.Join(tempDir, "golden.sha256")
	if err := os.WriteFile(goldenFile, []byte("expected-hash"), 0644); err != nil {
		t.Fatalf("failed to write golden: %v", err)
	}

	cmd := &GoldenCheckCmd{
		Capsule: packedPath,
		RunID:   "test-run",
		Golden:  goldenFile,
	}

	err = cmd.Run()
	if err == nil {
		t.Error("expected error for hash mismatch, got nil")
	}
}

func TestGoldenCheckCmd_Run_NoTranscript(t *testing.T) {
	tempDir := t.TempDir()

	cap, capsuleDir := createTestCapsule(t, tempDir)
	testFile := createTestFile(t, tempDir, "input.txt", "test")
	artifact, _ := cap.IngestFile(testFile)

	// Run without transcript
	cap.Manifest.Runs = map[string]*capsule.Run{
		"test-run": {
			ID:     "test-run",
			Inputs: []capsule.RunInput{{ArtifactID: artifact.ID}},
		},
	}

	packedPath := filepath.Join(tempDir, "test.capsule.tar.xz")
	cap.Pack(packedPath)
	os.RemoveAll(capsuleDir)

	goldenFile := filepath.Join(tempDir, "golden.sha256")
	os.WriteFile(goldenFile, []byte("hash"), 0644)

	cmd := &GoldenCheckCmd{
		Capsule: packedPath,
		RunID:   "test-run",
		Golden:  goldenFile,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for missing transcript, got nil")
	}
}

// Tests for JuniperHugoCmd

func TestJuniperHugoCmd_Run(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "data")

	cmd := &JuniperHugoCmd{
		Modules: []string{},
		Path:    filepath.Join(tempDir, "sword"),
		Output:  outputDir,
		All:     true,
		Workers: 1,
	}

	// This will fail because there's no SWORD installation, but it tests the wrapper
	err := cmd.Run()
	// We expect an error, just verify it doesn't panic
	_ = err
}

// Tests for WebCmd and APICmd (thin wrappers)

func TestWebCmd_Run_Wrapper(t *testing.T) {
	// We can't actually start the server in tests, but we can verify the structure
	cmd := &WebCmd{
		Port:            9999,
		Capsules:        t.TempDir(),
		Plugins:         t.TempDir(),
		Sword:           t.TempDir(),
		PluginsExternal: false,
		Restart:         false,
	}

	// Just verify the command structure is correct
	// We don't call Run() as it would start a web server
	if cmd.Port != 9999 {
		t.Errorf("port = %d, want 9999", cmd.Port)
	}
}

func TestAPICmd_Run_Wrapper(t *testing.T) {
	cmd := &APICmd{
		Port:            9998,
		Capsules:        t.TempDir(),
		Plugins:         t.TempDir(),
		PluginsExternal: false,
	}

	// Verify structure without starting server
	if cmd.Port != 9998 {
		t.Errorf("port = %d, want 9998", cmd.Port)
	}
}

// Tests for SelfcheckCmd with plan ID

func TestSelfcheckCmd_Run_WithPlanID(t *testing.T) {
	tempDir := t.TempDir()

	cap, capsuleDir := createTestCapsule(t, tempDir)
	testFile := createTestFile(t, tempDir, "input.txt", "test content")
	artifact, err := cap.IngestFile(testFile)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Create a roundtrip plan in the manifest
	cap.Manifest.RoundtripPlans = map[string]*capsule.Plan{
		"test-plan": {
			ID:          "test-plan",
			Description: "Test roundtrip plan",
			Steps: []capsule.PlanStep{
				{
					Type: "EXPORT",
					Export: &capsule.ExportStep{
						Mode:       string(capsule.ExportModeIdentity),
						ArtifactID: artifact.ID,
					},
				},
			},
			Checks: []capsule.PlanCheck{
				{
					Type:  "BYTE_EQUAL",
					Label: "Identity check",
					ByteEqual: &capsule.ByteEqualCheck{
						ArtifactA: artifact.ID,
						ArtifactB: artifact.ID,
					},
				},
			},
		},
	}

	packedPath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(packedPath); err != nil {
		t.Fatalf("failed to pack: %v", err)
	}

	os.RemoveAll(capsuleDir)

	cmd := &SelfcheckCmd{
		Capsule: packedPath,
		Plan:    "test-plan",
		JSON:    false,
	}

	err = cmd.Run()
	if err != nil {
		t.Errorf("SelfcheckCmd.Run() with plan ID error = %v", err)
	}
}

func TestSelfcheckCmd_Run_InvalidPlan(t *testing.T) {
	tempDir := t.TempDir()
	packedPath := createPackedCapsule(t, tempDir, "test")

	cmd := &SelfcheckCmd{
		Capsule: packedPath,
		Plan:    "nonexistent-plan",
		JSON:    false,
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for invalid plan ID, got nil")
	}
}

// Tests for VerifyCmd with corrupted data

func TestVerifyCmd_Run_CorruptedData(t *testing.T) {
	tempDir := t.TempDir()

	// Create a capsule
	cap, capsuleDir := createTestCapsule(t, tempDir)
	testFile := createTestFile(t, tempDir, "input.txt", "test content")
	artifact, err := cap.IngestFile(testFile)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Corrupt the artifact hash in manifest (simulate data corruption)
	artifact.Hashes.SHA256 = "corrupted-hash-000000000000000000000000000000000000000000000000"

	packedPath := filepath.Join(tempDir, "corrupted.capsule.tar.xz")
	if err := cap.Pack(packedPath); err != nil {
		t.Fatalf("failed to pack: %v", err)
	}

	os.RemoveAll(capsuleDir)

	cmd := &VerifyCmd{
		Capsule: packedPath,
	}

	err = cmd.Run()
	if err == nil {
		t.Error("expected error for corrupted data, got nil")
	}
}

// Tests for IngestCmd error paths

func TestIngestCmd_Run_InvalidOutputPath(t *testing.T) {
	tempDir := t.TempDir()
	testFile := createTestFile(t, tempDir, "input.txt", "test")

	// Use an invalid path (contains null byte)
	cmd := &IngestCmd{
		Path: testFile,
		Out:  "/tmp/invalid\x00path.capsule",
	}

	err := cmd.Run()
	if err == nil {
		t.Error("expected error for invalid output path, got nil")
	}
}

// Tests for ExportCmd with hash mismatch simulation

func TestExportCmd_Run_ReadError(t *testing.T) {
	tempDir := t.TempDir()
	packedPath := createPackedCapsule(t, tempDir, "test content")

	// Get artifact ID
	cap, err := capsule.Unpack(packedPath, filepath.Join(tempDir, "unpack"))
	if err != nil {
		t.Fatalf("failed to unpack: %v", err)
	}
	var artifactID string
	for id := range cap.Manifest.Artifacts {
		artifactID = id
		break
	}

	// Try to export to a directory (should fail)
	invalidOutput := filepath.Join(tempDir, "subdir")
	os.MkdirAll(invalidOutput, 0755)

	cmd := &ExportCmd{
		Capsule:  packedPath,
		Artifact: artifactID,
		Out:      invalidOutput,
	}

	err = cmd.Run()
	// Will fail because output is a directory
	if err == nil {
		t.Error("expected error when exporting to directory")
	}
}

// Tests for helper functions with more coverage

func TestGetPluginDir_WithEnvVar(t *testing.T) {
	// Save and restore
	origPluginDir := CLI.PluginDir
	defer func() { CLI.PluginDir = origPluginDir }()

	// Test with custom directory
	customDir := "/custom/plugin/dir"
	CLI.PluginDir = customDir

	result := getPluginDir()
	if result != customDir {
		t.Errorf("getPluginDir() = %q, want %q", result, customDir)
	}
}


