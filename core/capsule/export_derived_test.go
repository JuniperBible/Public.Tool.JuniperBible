package capsule

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JuniperBible/juniper/core/ir"
	"github.com/JuniperBible/juniper/core/plugins"
)

func init() {
	// Enable external plugins for testing
	plugins.EnableExternalPlugins()
}

// Helper to create a test plugin loader with plugins
func setupTestPluginLoader(t *testing.T, pluginNames []string) (*plugins.Loader, string) {
	tempDir, err := os.MkdirTemp("", "plugin-loader-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	loader := plugins.NewLoader()

	// Create plugins
	for _, name := range pluginNames {
		createTestPlugin(t, tempDir, name)
	}

	// Load plugins from directory
	if err := loader.LoadFromDir(tempDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	return loader, tempDir
}

// createTestPlugin creates a minimal plugin directory
func createTestPlugin(t *testing.T, baseDir, name string) {
	pluginPath := filepath.Join(baseDir, name)
	if err := os.MkdirAll(pluginPath, 0700); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifest := &plugins.PluginManifest{
		PluginID:   name,
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "plugin.sh",
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}

	manifestPath := filepath.Join(pluginPath, "plugin.json")
	if err := os.WriteFile(manifestPath, manifestData, 0600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	entrypointPath := filepath.Join(pluginPath, "plugin.sh")
	if err := os.WriteFile(entrypointPath, []byte("#!/bin/bash\necho test"), 0700); err != nil {
		t.Fatalf("failed to write entrypoint: %v", err)
	}
}

// TestFindPluginForFormat tests the findPluginForFormat function.
func TestFindPluginForFormat(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		plugins     []string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty format",
			format:      "",
			plugins:     []string{},
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:    "plugin with format- prefix",
			format:  "osis",
			plugins: []string{"format-osis"},
			wantErr: false,
		},
		{
			name:    "plugin without prefix",
			format:  "usfm",
			plugins: []string{"usfm"},
			wantErr: false,
		},
		{
			name:        "plugin not found",
			format:      "nonexistent",
			plugins:     []string{"other-plugin"},
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader, tempDir := setupTestPluginLoader(t, tt.plugins)
			defer os.RemoveAll(tempDir)

			plugin, err := findPluginForFormat(loader, tt.format)

			if tt.wantErr {
				if err == nil {
					t.Errorf("findPluginForFormat() expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("findPluginForFormat() error = %v, want to contain %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("findPluginForFormat() unexpected error: %v", err)
				}
				if plugin == nil {
					t.Error("findPluginForFormat() returned nil plugin")
				}
			}
		})
	}
}

// TestConvertIPCLossReport tests the convertIPCLossReport function.
func TestConvertIPCLossReport(t *testing.T) {
	tests := []struct {
		name     string
		input    *plugins.LossReportIPC
		expected *ir.LossReport
	}{
		{
			name: "full loss report",
			input: &plugins.LossReportIPC{
				SourceFormat: "SWORD",
				TargetFormat: "IR",
				LossClass:    "L2",
				LostElements: []plugins.LostElementIPC{
					{
						Path:          "Gen.1.1",
						ElementType:   "strongs",
						Reason:        "not supported in IR",
						OriginalValue: "H1234",
					},
					{
						Path:        "Gen.1.2",
						ElementType: "morphology",
						Reason:      "complex morphology",
					},
				},
				Warnings: []string{"warning 1", "warning 2"},
			},
			expected: &ir.LossReport{
				SourceFormat: "SWORD",
				TargetFormat: "IR",
				LossClass:    ir.LossL2,
				LostElements: []ir.LostElement{
					{
						Path:          "Gen.1.1",
						ElementType:   "strongs",
						Reason:        "not supported in IR",
						OriginalValue: "H1234",
					},
					{
						Path:        "Gen.1.2",
						ElementType: "morphology",
						Reason:      "complex morphology",
					},
				},
				Warnings: []string{"warning 1", "warning 2"},
			},
		},
		{
			name: "minimal loss report",
			input: &plugins.LossReportIPC{
				SourceFormat: "OSIS",
				TargetFormat: "USFM",
				LossClass:    "L0",
			},
			expected: &ir.LossReport{
				SourceFormat: "OSIS",
				TargetFormat: "USFM",
				LossClass:    ir.LossL0,
			},
		},
		{
			name: "empty lost elements",
			input: &plugins.LossReportIPC{
				SourceFormat: "TEST",
				TargetFormat: "IR",
				LossClass:    "L1",
				LostElements: []plugins.LostElementIPC{},
				Warnings:     []string{},
			},
			expected: &ir.LossReport{
				SourceFormat: "TEST",
				TargetFormat: "IR",
				LossClass:    ir.LossL1,
				LostElements: []ir.LostElement{},
				Warnings:     []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertIPCLossReport(tt.input)

			if result.SourceFormat != tt.expected.SourceFormat {
				t.Errorf("SourceFormat = %q, want %q", result.SourceFormat, tt.expected.SourceFormat)
			}
			if result.TargetFormat != tt.expected.TargetFormat {
				t.Errorf("TargetFormat = %q, want %q", result.TargetFormat, tt.expected.TargetFormat)
			}
			if result.LossClass != tt.expected.LossClass {
				t.Errorf("LossClass = %q, want %q", result.LossClass, tt.expected.LossClass)
			}
			if len(result.LostElements) != len(tt.expected.LostElements) {
				t.Errorf("LostElements count = %d, want %d", len(result.LostElements), len(tt.expected.LostElements))
			}
			if len(result.Warnings) != len(tt.expected.Warnings) {
				t.Errorf("Warnings count = %d, want %d", len(result.Warnings), len(tt.expected.Warnings))
			}
		})
	}
}

// TestCombineLossClasses tests the combineLossClasses function.
func TestCombineLossClasses(t *testing.T) {
	tests := []struct {
		name     string
		reports  []*ir.LossReport
		expected ir.LossClass
	}{
		{
			name:     "empty reports",
			reports:  []*ir.LossReport{},
			expected: ir.LossL0,
		},
		{
			name: "single L0 report",
			reports: []*ir.LossReport{
				{LossClass: ir.LossL0},
			},
			expected: ir.LossL0,
		},
		{
			name: "single L1 report",
			reports: []*ir.LossReport{
				{LossClass: ir.LossL1},
			},
			expected: ir.LossL1,
		},
		{
			name: "multiple reports - worst is L2",
			reports: []*ir.LossReport{
				{LossClass: ir.LossL0},
				{LossClass: ir.LossL2},
				{LossClass: ir.LossL1},
			},
			expected: ir.LossL2,
		},
		{
			name: "multiple reports - worst is L4",
			reports: []*ir.LossReport{
				{LossClass: ir.LossL1},
				{LossClass: ir.LossL4},
				{LossClass: ir.LossL2},
			},
			expected: ir.LossL4,
		},
		{
			name: "nil reports in list",
			reports: []*ir.LossReport{
				{LossClass: ir.LossL1},
				nil,
				{LossClass: ir.LossL2},
			},
			expected: ir.LossL2,
		},
		{
			name: "all nil reports",
			reports: []*ir.LossReport{
				nil,
				nil,
			},
			expected: ir.LossL0,
		},
		{
			name: "all L3 reports",
			reports: []*ir.LossReport{
				{LossClass: ir.LossL3},
				{LossClass: ir.LossL3},
			},
			expected: ir.LossL3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := combineLossClasses(tt.reports)
			if result != tt.expected {
				t.Errorf("combineLossClasses() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestCombinedLossReport tests the CombinedLossReport function.
func TestCombinedLossReport(t *testing.T) {
	tests := []struct {
		name     string
		reports  []*ir.LossReport
		expected *ir.LossReport
	}{
		{
			name:    "empty reports",
			reports: []*ir.LossReport{},
			expected: &ir.LossReport{
				LossClass: ir.LossL0,
			},
		},
		{
			name: "single report",
			reports: []*ir.LossReport{
				{
					SourceFormat: "OSIS",
					TargetFormat: "IR",
					LossClass:    ir.LossL1,
					LostElements: []ir.LostElement{
						{Path: "Gen.1.1", ElementType: "test"},
					},
					Warnings: []string{"warning 1"},
				},
			},
			expected: &ir.LossReport{
				SourceFormat: "OSIS",
				TargetFormat: "IR",
				LossClass:    ir.LossL1,
				LostElements: []ir.LostElement{
					{Path: "Gen.1.1", ElementType: "test"},
				},
				Warnings: []string{"warning 1"},
			},
		},
		{
			name: "multiple reports - combines elements and warnings",
			reports: []*ir.LossReport{
				{
					SourceFormat: "OSIS",
					TargetFormat: "IR",
					LossClass:    ir.LossL1,
					LostElements: []ir.LostElement{
						{Path: "Gen.1.1", ElementType: "strongs"},
					},
					Warnings: []string{"warning 1"},
				},
				{
					SourceFormat: "IR",
					TargetFormat: "USFM",
					LossClass:    ir.LossL2,
					LostElements: []ir.LostElement{
						{Path: "Gen.1.2", ElementType: "morphology"},
					},
					Warnings: []string{"warning 2"},
				},
			},
			expected: &ir.LossReport{
				SourceFormat: "OSIS",
				TargetFormat: "USFM",
				LossClass:    ir.LossL2,
				LostElements: []ir.LostElement{
					{Path: "Gen.1.1", ElementType: "strongs"},
					{Path: "Gen.1.2", ElementType: "morphology"},
				},
				Warnings: []string{"warning 1", "warning 2"},
			},
		},
		{
			name: "handles nil reports",
			reports: []*ir.LossReport{
				{
					SourceFormat: "OSIS",
					TargetFormat: "IR",
					LossClass:    ir.LossL1,
				},
				nil,
				{
					SourceFormat: "IR",
					TargetFormat: "USFM",
					LossClass:    ir.LossL0,
				},
			},
			expected: &ir.LossReport{
				SourceFormat: "OSIS",
				TargetFormat: "USFM",
				LossClass:    ir.LossL1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CombinedLossReport(tt.reports)

			if result.SourceFormat != tt.expected.SourceFormat {
				t.Errorf("SourceFormat = %q, want %q", result.SourceFormat, tt.expected.SourceFormat)
			}
			if result.TargetFormat != tt.expected.TargetFormat {
				t.Errorf("TargetFormat = %q, want %q", result.TargetFormat, tt.expected.TargetFormat)
			}
			if result.LossClass != tt.expected.LossClass {
				t.Errorf("LossClass = %v, want %v", result.LossClass, tt.expected.LossClass)
			}
			if len(result.LostElements) != len(tt.expected.LostElements) {
				t.Errorf("LostElements count = %d, want %d", len(result.LostElements), len(tt.expected.LostElements))
			}
			if len(result.Warnings) != len(tt.expected.Warnings) {
				t.Errorf("Warnings count = %d, want %d", len(result.Warnings), len(tt.expected.Warnings))
			}
		})
	}
}

// TestExportDerivedValidation tests validation in ExportDerived.
func TestExportDerivedValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-derived-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testFilePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	tests := []struct {
		name        string
		artifactID  string
		opts        DerivedExportOptions
		wantErr     bool
		errContains string
	}{
		{
			name:       "missing plugin loader and plugins",
			artifactID: artifact.ID,
			opts: DerivedExportOptions{
				TargetFormat: "usfm",
			},
			wantErr:     true,
			errContains: "requires PluginLoader or both SourcePlugin and TargetPlugin",
		},
		{
			name:       "missing target format",
			artifactID: artifact.ID,
			opts: DerivedExportOptions{
				PluginLoader: plugins.NewLoader(),
			},
			wantErr:     true,
			errContains: "is required",
		},
		{
			name:       "artifact not found",
			artifactID: "nonexistent",
			opts: DerivedExportOptions{
				TargetFormat: "usfm",
				PluginLoader: plugins.NewLoader(),
			},
			wantErr:     true,
			errContains: "artifact not found",
		},
		{
			name:       "source plugin provided but not target",
			artifactID: artifact.ID,
			opts: DerivedExportOptions{
				TargetFormat: "usfm",
				SourcePlugin: &plugins.Plugin{
					Manifest: &plugins.PluginManifest{PluginID: "test"},
				},
			},
			wantErr:     true,
			errContains: "requires PluginLoader or both SourcePlugin and TargetPlugin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destPath := filepath.Join(tempDir, "output.txt")
			_, err := cap.ExportDerived(tt.artifactID, tt.opts, destPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ExportDerived() expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ExportDerived() error = %v, want to contain %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ExportDerived() unexpected error: %v", err)
				}
			}
		})
	}
}

// TestExportDerivedToBytes tests the ExportDerivedToBytes function.
func TestExportDerivedToBytes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-derived-bytes-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Test with nonexistent artifact
	loader := plugins.NewLoader()
	opts := DerivedExportOptions{
		TargetFormat: "test",
		PluginLoader: loader,
	}

	_, _, err = cap.ExportDerivedToBytes("nonexistent", opts)
	if err == nil {
		t.Error("ExportDerivedToBytes() expected error for nonexistent artifact, got nil")
	}

	// Test with invalid options
	_, _, err = cap.ExportDerivedToBytes("test-id", DerivedExportOptions{})
	if err == nil {
		t.Error("ExportDerivedToBytes() expected error for invalid options, got nil")
	}
}

// TestExtractIRFromPluginError tests error handling in extractIRFromPlugin.
func TestExtractIRFromPluginError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "extract-ir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a plugin with invalid entrypoint
	pluginDir := filepath.Join(tempDir, "plugin")
	if err := os.MkdirAll(pluginDir, 0700); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	plugin := &plugins.Plugin{
		Manifest: &plugins.PluginManifest{
			PluginID:   "test-plugin",
			Entrypoint: "nonexistent.sh",
		},
		Path: pluginDir,
	}

	sourcePath := filepath.Join(tempDir, "source.txt")
	if err := os.WriteFile(sourcePath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	outputDir := filepath.Join(tempDir, "output")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	_, _, err = extractIRFromPlugin(plugin, sourcePath, outputDir)
	if err == nil {
		t.Error("extractIRFromPlugin() expected error for invalid plugin, got nil")
	}
}

// TestEmitNativeFromPluginError tests error handling in emitNativeFromPlugin.
func TestEmitNativeFromPluginError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "emit-native-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a plugin with invalid entrypoint
	pluginDir := filepath.Join(tempDir, "plugin")
	if err := os.MkdirAll(pluginDir, 0700); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	plugin := &plugins.Plugin{
		Manifest: &plugins.PluginManifest{
			PluginID:   "test-plugin",
			Entrypoint: "nonexistent.sh",
		},
		Path: pluginDir,
	}

	irPath := filepath.Join(tempDir, "ir.json")
	if err := os.WriteFile(irPath, []byte("{}"), 0600); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputDir := filepath.Join(tempDir, "output")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	_, _, err = emitNativeFromPlugin(plugin, irPath, outputDir)
	if err == nil {
		t.Error("emitNativeFromPlugin() expected error for invalid plugin, got nil")
	}
}

// TestExportDerivedPluginNotFound tests when plugins cannot be found.
func TestExportDerivedPluginNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-derived-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testFilePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Set detected format
	artifact.Detected = &DetectionResult{
		FormatID: "unknown-format",
	}
	cap.Manifest.Artifacts[artifact.ID] = artifact

	loader := plugins.NewLoader()
	opts := DerivedExportOptions{
		TargetFormat: "test-format",
		PluginLoader: loader,
	}

	destPath := filepath.Join(tempDir, "output.txt")
	_, err = cap.ExportDerived(artifact.ID, opts, destPath)
	if err == nil {
		t.Error("ExportDerived() expected error when source plugin not found, got nil")
	}
	if !strings.Contains(err.Error(), "failed to find source plugin") {
		t.Errorf("ExportDerived() error = %v, want to contain 'failed to find source plugin'", err)
	}
}

// TestExportDerivedTargetPluginNotFound tests when target plugin cannot be found.
func TestExportDerivedTargetPluginNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-derived-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testFilePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Set detected format
	artifact.Detected = &DetectionResult{
		FormatID: "osis",
	}
	cap.Manifest.Artifacts[artifact.ID] = artifact

	loader, loaderDir := setupTestPluginLoader(t, []string{"format-osis"})
	defer os.RemoveAll(loaderDir)

	opts := DerivedExportOptions{
		TargetFormat: "unknown-target",
		PluginLoader: loader,
	}

	destPath := filepath.Join(tempDir, "output.txt")
	_, err = cap.ExportDerived(artifact.ID, opts, destPath)
	if err == nil {
		t.Error("ExportDerived() expected error when target plugin not found, got nil")
	}
	if !strings.Contains(err.Error(), "failed to find target plugin") {
		t.Errorf("ExportDerived() error = %v, want to contain 'failed to find target plugin'", err)
	}
}

// TestExportDerivedRetrieveError tests error when blob retrieval fails.
func TestExportDerivedRetrieveError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-derived-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create artifact with invalid blob hash
	artifact := &Artifact{
		ID:                "test-artifact",
		Kind:              "file",
		PrimaryBlobSHA256: "invalid-hash-that-does-not-exist",
	}
	cap.Manifest.Artifacts[artifact.ID] = artifact

	loader := plugins.NewLoader()
	opts := DerivedExportOptions{
		TargetFormat: "test",
		PluginLoader: loader,
	}

	destPath := filepath.Join(tempDir, "output.txt")
	_, err = cap.ExportDerived(artifact.ID, opts, destPath)
	if err == nil {
		t.Error("ExportDerived() expected error when blob retrieval fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to retrieve source blob") {
		t.Errorf("ExportDerived() error = %v, want to contain 'failed to retrieve source blob'", err)
	}
}

// TestFindPluginForFormatPriority tests plugin lookup priority.
func TestFindPluginForFormatPriority(t *testing.T) {
	loader, tempDir := setupTestPluginLoader(t, []string{"format-test", "test"})
	defer os.RemoveAll(tempDir)

	// Should prefer format-prefixed version
	plugin, err := findPluginForFormat(loader, "test")
	if err != nil {
		t.Fatalf("findPluginForFormat() unexpected error: %v", err)
	}

	if plugin.Manifest.PluginID != "format-test" {
		t.Errorf("findPluginForFormat() plugin ID = %q, want %q", plugin.Manifest.PluginID, "format-test")
	}
}

// TestCombinedLossReportWithEmptyElements tests combining reports with empty elements.
func TestCombinedLossReportWithEmptyElements(t *testing.T) {
	reports := []*ir.LossReport{
		{
			SourceFormat: "A",
			TargetFormat: "B",
			LossClass:    ir.LossL1,
			LostElements: []ir.LostElement{},
			Warnings:     []string{},
		},
		{
			SourceFormat: "B",
			TargetFormat: "C",
			LossClass:    ir.LossL0,
			LostElements: []ir.LostElement{},
			Warnings:     []string{},
		},
	}

	result := CombinedLossReport(reports)

	if result.SourceFormat != "A" {
		t.Errorf("SourceFormat = %q, want %q", result.SourceFormat, "A")
	}
	if result.TargetFormat != "C" {
		t.Errorf("TargetFormat = %q, want %q", result.TargetFormat, "C")
	}
	if result.LossClass != ir.LossL1 {
		t.Errorf("LossClass = %v, want %v", result.LossClass, ir.LossL1)
	}
	if len(result.LostElements) != 0 {
		t.Errorf("LostElements count = %d, want 0", len(result.LostElements))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("Warnings count = %d, want 0", len(result.Warnings))
	}
}

// TestExportDerivedNoDetectedFormat tests ExportDerived when artifact has no detected format.
func TestExportDerivedNoDetectedFormat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-derived-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testFilePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// artifact.Detected is nil by default
	loader := plugins.NewLoader()
	opts := DerivedExportOptions{
		TargetFormat: "test",
		PluginLoader: loader,
	}

	destPath := filepath.Join(tempDir, "output.txt")
	_, err = cap.ExportDerived(artifact.ID, opts, destPath)
	if err == nil {
		t.Error("ExportDerived() expected error when format detection is nil, got nil")
	}
}

// TestConvertIPCLossReportNilElements tests edge case with nil elements.
func TestConvertIPCLossReportNilElements(t *testing.T) {
	ipc := &plugins.LossReportIPC{
		SourceFormat: "test",
		TargetFormat: "ir",
		LossClass:    "L0",
		LostElements: nil,
		Warnings:     nil,
	}

	result := convertIPCLossReport(ipc)

	if result == nil {
		t.Fatal("convertIPCLossReport() returned nil")
	}

	// Should not panic and should handle nil slices
	if result.SourceFormat != "test" {
		t.Errorf("SourceFormat = %q, want %q", result.SourceFormat, "test")
	}
}

// TestCombineLossClassesEdgeCases tests edge cases in loss class combination.
func TestCombineLossClassesEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		reports  []*ir.LossReport
		expected ir.LossClass
	}{
		{
			name:     "nil slice",
			reports:  nil,
			expected: ir.LossL0,
		},
		{
			name: "invalid loss class defaults to level -1",
			reports: []*ir.LossReport{
				{LossClass: ir.LossClass("INVALID")},
			},
			expected: ir.LossL0, // Should default to L0 for invalid
		},
		{
			name: "mixed valid and invalid",
			reports: []*ir.LossReport{
				{LossClass: ir.LossL1},
				{LossClass: ir.LossClass("INVALID")},
				{LossClass: ir.LossL2},
			},
			expected: ir.LossL2, // Should use the valid highest
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := combineLossClasses(tt.reports)
			if result != tt.expected {
				t.Errorf("combineLossClasses() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestExportDerivedMkdirIRDirError tests osMkdirAllExport error for IR dir.
func TestExportDerivedMkdirIRDirError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-derived-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testFilePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	artifact.Detected = &DetectionResult{FormatID: "osis"}
	cap.Manifest.Artifacts[artifact.ID] = artifact

	loader, loaderDir := setupTestPluginLoader(t, []string{"format-osis", "format-usfm"})
	defer os.RemoveAll(loaderDir)

	// Inject MkdirAll error - fails on IR dir (contains "ir")
	orig := osMkdirAllExport
	osMkdirAllExport = func(path string, perm os.FileMode) error {
		if strings.Contains(path, "/ir") {
			return errors.New("injected mkdir ir error")
		}
		return orig(path, perm)
	}
	defer func() { osMkdirAllExport = orig }()

	opts := DerivedExportOptions{
		TargetFormat: "usfm",
		PluginLoader: loader,
	}

	destPath := filepath.Join(tempDir, "output.txt")
	_, err = cap.ExportDerived(artifact.ID, opts, destPath)
	if err == nil {
		t.Error("expected error for mkdir IR dir failure")
	}
	if !strings.Contains(err.Error(), "failed to create IR dir") {
		t.Errorf("error = %v, want to contain 'failed to create IR dir'", err)
	}
}

// TestExportDerivedMkdirOutputDirError tests osMkdirAllExport error for output dir.
func TestExportDerivedMkdirOutputDirError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-derived-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testFilePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	artifact.Detected = &DetectionResult{FormatID: "osis"}
	cap.Manifest.Artifacts[artifact.ID] = artifact

	loader, loaderDir := setupTestPluginLoader(t, []string{"format-osis", "format-usfm"})
	defer os.RemoveAll(loaderDir)

	// Mock plugin execution to succeed
	origExecute := pluginsExecutePlugin
	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		irPath := "/tmp/mock-ir.json"
		return &plugins.IPCResponse{
			Status: "success",
			Result: map[string]interface{}{
				"ir_path": irPath,
			},
		}, nil
	}
	defer func() { pluginsExecutePlugin = origExecute }()

	origParse := pluginsParseExtractIRResult
	pluginsParseExtractIRResult = func(resp *plugins.IPCResponse) (*plugins.ExtractIRResult, error) {
		return &plugins.ExtractIRResult{
			IRPath: "/tmp/mock-ir.json",
		}, nil
	}
	defer func() { pluginsParseExtractIRResult = origParse }()

	// Inject MkdirAll error - fails on output dir
	orig := osMkdirAllExport
	callCount := 0
	osMkdirAllExport = func(path string, perm os.FileMode) error {
		callCount++
		// First call is for IR dir (succeed), second is for output dir (fail)
		if callCount == 2 && strings.Contains(path, "output") {
			return errors.New("injected mkdir output error")
		}
		return orig(path, perm)
	}
	defer func() { osMkdirAllExport = orig }()

	opts := DerivedExportOptions{
		TargetFormat: "usfm",
		PluginLoader: loader,
	}

	destPath := filepath.Join(tempDir, "output.txt")
	_, err = cap.ExportDerived(artifact.ID, opts, destPath)
	if err == nil {
		t.Error("expected error for mkdir output dir failure")
	}
	if !strings.Contains(err.Error(), "failed to create output dir") {
		t.Errorf("error = %v, want to contain 'failed to create output dir'", err)
	}
}

// TestExportDerivedMkdirDestDirError tests osMkdirAllExport error for destination dir.
func TestExportDerivedMkdirDestDirError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-derived-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testFilePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	artifact.Detected = &DetectionResult{FormatID: "osis"}
	cap.Manifest.Artifacts[artifact.ID] = artifact

	loader, loaderDir := setupTestPluginLoader(t, []string{"format-osis", "format-usfm"})
	defer os.RemoveAll(loaderDir)

	// Mock plugin execution to succeed for both extract and emit
	origExecute := pluginsExecutePlugin
	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		return &plugins.IPCResponse{
			Status: "success",
			Result: map[string]interface{}{"ir_path": "/tmp/mock.json", "output_path": "/tmp/mock-out.txt"},
		}, nil
	}
	defer func() { pluginsExecutePlugin = origExecute }()

	origParseExtract := pluginsParseExtractIRResult
	pluginsParseExtractIRResult = func(resp *plugins.IPCResponse) (*plugins.ExtractIRResult, error) {
		return &plugins.ExtractIRResult{IRPath: "/tmp/mock.json"}, nil
	}
	defer func() { pluginsParseExtractIRResult = origParseExtract }()

	origParseEmit := pluginsParseEmitNativeResult
	pluginsParseEmitNativeResult = func(resp *plugins.IPCResponse) (*plugins.EmitNativeResult, error) {
		return &plugins.EmitNativeResult{OutputPath: "/tmp/mock-out.txt"}, nil
	}
	defer func() { pluginsParseEmitNativeResult = origParseEmit }()

	// Inject MkdirAll error - fails on third call (destination dir)
	orig := osMkdirAllExport
	callCount := 0
	osMkdirAllExport = func(path string, perm os.FileMode) error {
		callCount++
		if callCount == 3 { // Third call is for dest dir
			return errors.New("injected mkdir dest error")
		}
		return orig(path, perm)
	}
	defer func() { osMkdirAllExport = orig }()

	opts := DerivedExportOptions{
		TargetFormat: "usfm",
		PluginLoader: loader,
	}

	destPath := filepath.Join(tempDir, "nested/subdir/output.txt")
	_, err = cap.ExportDerived(artifact.ID, opts, destPath)
	if err == nil {
		t.Error("expected error for mkdir destination dir failure")
	}
	if !strings.Contains(err.Error(), "failed to create destination dir") {
		t.Errorf("error = %v, want to contain 'failed to create destination dir'", err)
	}
}

// TestExportDerivedToBytesReadFileError tests osReadFileExport error in ExportDerivedToBytes.
func TestExportDerivedToBytesReadFileError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-derived-bytes-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testFilePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	artifact.Detected = &DetectionResult{FormatID: "osis"}
	cap.Manifest.Artifacts[artifact.ID] = artifact

	loader, loaderDir := setupTestPluginLoader(t, []string{"format-osis", "format-usfm"})
	defer os.RemoveAll(loaderDir)

	// Mock plugin execution to succeed
	origExecute := pluginsExecutePlugin
	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		return &plugins.IPCResponse{
			Status: "success",
			Result: map[string]interface{}{"ir_path": "/tmp/mock.json", "output_path": "/tmp/mock-out.txt"},
		}, nil
	}
	defer func() { pluginsExecutePlugin = origExecute }()

	origParseExtract := pluginsParseExtractIRResult
	pluginsParseExtractIRResult = func(resp *plugins.IPCResponse) (*plugins.ExtractIRResult, error) {
		return &plugins.ExtractIRResult{IRPath: "/tmp/mock.json"}, nil
	}
	defer func() { pluginsParseExtractIRResult = origParseExtract }()

	origParseEmit := pluginsParseEmitNativeResult
	pluginsParseEmitNativeResult = func(resp *plugins.IPCResponse) (*plugins.EmitNativeResult, error) {
		// Create a real file so ExportDerived can write to it
		outPath := filepath.Join(tempDir, "output-temp.txt")
		os.WriteFile(outPath, []byte("output"), 0600)
		return &plugins.EmitNativeResult{OutputPath: outPath}, nil
	}
	defer func() { pluginsParseEmitNativeResult = origParseEmit }()

	// Inject ReadFile error in ExportDerivedToBytes - but need to allow reads for ExportDerived
	orig := osReadFileExport
	readCallCount := 0
	osReadFileExport = func(name string) ([]byte, error) {
		readCallCount++
		// ExportDerived reads output once, then ExportDerivedToBytes reads result once
		// We want to fail on the final read in ExportDerivedToBytes
		if strings.Contains(name, "capsule-derived-bytes") {
			return nil, errors.New("injected read file error")
		}
		return orig(name)
	}
	defer func() { osReadFileExport = orig }()

	opts := DerivedExportOptions{
		TargetFormat: "usfm",
		PluginLoader: loader,
	}

	_, _, err = cap.ExportDerivedToBytes(artifact.ID, opts)
	if err == nil {
		t.Error("expected error for read file failure")
	}
	if !strings.Contains(err.Error(), "failed to read output") {
		t.Errorf("error = %v, want to contain 'failed to read output'", err)
	}
}
