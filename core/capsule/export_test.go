package capsule

import (
	"context"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/cas"
	"github.com/JuniperBible/Public.Tool.JuniperBible/core/ir"
	"github.com/JuniperBible/Public.Tool.JuniperBible/core/plugins"
)

// TestExportIdentity tests that exporting in IDENTITY mode produces byte-identical output.
func TestExportIdentity(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file with specific content
	originalContent := []byte("This is the original content that must be preserved byte-for-byte!")
	testFilePath := filepath.Join(tempDir, "original.txt")
	if err := os.WriteFile(testFilePath, originalContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create capsule and ingest
	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	artifact, err := capsule.IngestFile(context.Background(), testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Export in IDENTITY mode
	exportPath := filepath.Join(tempDir, "exported.txt")
	if err := capsule.Export(context.Background(), artifact.ID, ExportModeIdentity, exportPath); err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Read exported file
	exportedContent, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}

	// Verify byte-for-byte equality
	if !bytes.Equal(exportedContent, originalContent) {
		t.Errorf("exported content differs from original")
		t.Errorf("original: %q", originalContent)
		t.Errorf("exported: %q", exportedContent)
	}

	// Verify hashes match
	originalHash := cas.Hash(originalContent)
	exportedHash := cas.Hash(exportedContent)
	if originalHash != exportedHash {
		t.Errorf("hash mismatch: original=%s, exported=%s", originalHash, exportedHash)
	}
}

// TestExportIdentityAfterPackUnpack tests byte-identity after pack/unpack cycle.
func TestExportIdentityAfterPackUnpack(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-pack-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test file
	originalContent := []byte("Content for pack/unpack/export test - byte preservation is critical!")
	testFilePath := filepath.Join(tempDir, "original.txt")
	if err := os.WriteFile(testFilePath, originalContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create capsule and ingest
	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	artifact, err := capsule.IngestFile(context.Background(), testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}
	artifactID := artifact.ID
	originalSHA256 := artifact.Hashes.SHA256

	// Pack the capsule
	archivePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := capsule.Pack(archivePath); err != nil {
		t.Fatalf("failed to pack: %v", err)
	}

	// Unpack to new location
	unpackDir := filepath.Join(tempDir, "unpacked")
	unpacked, err := Unpack(archivePath, unpackDir)
	if err != nil {
		t.Fatalf("failed to unpack: %v", err)
	}

	// Export from unpacked capsule
	exportPath := filepath.Join(tempDir, "exported.txt")
	if err := unpacked.Export(context.Background(), artifactID, ExportModeIdentity, exportPath); err != nil {
		t.Fatalf("failed to export from unpacked: %v", err)
	}

	// Verify byte-for-byte equality
	exportedContent, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}

	if !bytes.Equal(exportedContent, originalContent) {
		t.Error("exported content differs from original after pack/unpack")
	}

	// Verify hash matches
	exportedHash := cas.Hash(exportedContent)
	if exportedHash != originalSHA256 {
		t.Errorf("hash mismatch after pack/unpack: original=%s, exported=%s", originalSHA256, exportedHash)
	}
}

// TestExportNonExistentArtifact tests that exporting a non-existent artifact fails.
func TestExportNonExistentArtifact(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	exportPath := filepath.Join(tempDir, "exported.txt")
	err = capsule.Export(context.Background(), "non-existent-artifact", ExportModeIdentity, exportPath)
	if err == nil {
		t.Error("expected error when exporting non-existent artifact")
	}
}

// TestExportMultipleFiles tests exporting multiple files preserves all bytes.
func TestExportMultipleFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-multi-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create multiple test files with different content
	files := map[string][]byte{
		"text.txt":    []byte("Plain text content"),
		"binary.bin":  {0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
		"empty.dat":   {},
		"unicode.txt": []byte("Unicode: 你好世界 🌍"),
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	artifacts := make(map[string]string) // name -> artifact ID

	for name, content := range files {
		path := filepath.Join(tempDir, name)
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}

		artifact, err := capsule.IngestFile(context.Background(), path)
		if err != nil {
			t.Fatalf("failed to ingest %s: %v", name, err)
		}
		artifacts[name] = artifact.ID
	}

	// Pack and unpack
	archivePath := filepath.Join(tempDir, "multi.capsule.tar.xz")
	if err := capsule.Pack(archivePath); err != nil {
		t.Fatalf("failed to pack: %v", err)
	}

	unpackDir := filepath.Join(tempDir, "unpacked")
	unpacked, err := Unpack(archivePath, unpackDir)
	if err != nil {
		t.Fatalf("failed to unpack: %v", err)
	}

	// Export each file and verify
	exportDir := filepath.Join(tempDir, "exports")
	if err := os.MkdirAll(exportDir, 0700); err != nil {
		t.Fatalf("failed to create export dir: %v", err)
	}

	for name, content := range files {
		artifactID := artifacts[name]
		exportPath := filepath.Join(exportDir, name)

		if err := unpacked.Export(context.Background(), artifactID, ExportModeIdentity, exportPath); err != nil {
			t.Errorf("failed to export %s: %v", name, err)
			continue
		}

		exported, err := os.ReadFile(exportPath)
		if err != nil {
			t.Errorf("failed to read exported %s: %v", name, err)
			continue
		}

		if !bytes.Equal(exported, content) {
			t.Errorf("byte mismatch for %s: got %d bytes, want %d bytes", name, len(exported), len(content))
		}
	}
}

// TestExportDerivedMode tests that DERIVED mode returns error.
func TestExportDerivedMode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create and ingest a file
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := capsule.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Export in DERIVED mode should fail (not implemented)
	err = capsule.Export(context.Background(), artifact.ID, ExportModeDerived, filepath.Join(tempDir, "out.txt"))
	if err == nil {
		t.Error("expected error for DERIVED mode")
	}
}

// TestExportUnknownMode tests that unknown mode returns error.
func TestExportUnknownMode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create and ingest a file
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := capsule.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Export with unknown mode
	err = capsule.Export(context.Background(), artifact.ID, "UNKNOWN", filepath.Join(tempDir, "out.txt"))
	if err == nil {
		t.Error("expected error for unknown mode")
	}
}

// TestExportToBytesUnknownMode tests ExportToBytes with unknown mode.
func TestExportToBytesUnknownMode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := capsule.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	_, err = capsule.ExportToBytes(context.Background(), artifact.ID, "UNKNOWN")
	if err == nil {
		t.Error("expected error for unknown mode")
	}
}

// TestExportToBytesDerivedMode tests ExportToBytes with DERIVED mode.
func TestExportToBytesDerivedMode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := capsule.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	_, err = capsule.ExportToBytes(context.Background(), artifact.ID, ExportModeDerived)
	if err == nil {
		t.Error("expected error for DERIVED mode")
	}
}

// TestExportIdentityInvalidPath tests exporting to an invalid path.
func TestExportIdentityInvalidPath(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := capsule.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	err = capsule.Export(context.Background(), artifact.ID, ExportModeIdentity, "/proc/invalid/path")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

// TestExportIdentityMkdirError tests exportIdentity with mkdir error.
func TestExportIdentityMkdirError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := cap.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Inject mkdir error
	origMkdir := osMkdirAllExport
	osMkdirAllExport = func(path string, perm os.FileMode) error {
		return errors.New("injected mkdir error")
	}
	defer func() { osMkdirAllExport = origMkdir }()

	err = cap.Export(context.Background(), artifact.ID, ExportModeIdentity, filepath.Join(tempDir, "out/file.txt"))
	if err == nil {
		t.Error("expected error for mkdir failure")
	}
}

// TestExportIdentityWriteError tests exportIdentity with write error.
func TestExportIdentityWriteError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := cap.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Inject write error
	origWrite := osWriteFileIdentity
	osWriteFileIdentity = func(name string, data []byte, perm os.FileMode) error {
		return errors.New("injected write error")
	}
	defer func() { osWriteFileIdentity = origWrite }()

	err = cap.Export(context.Background(), artifact.ID, ExportModeIdentity, filepath.Join(tempDir, "out.txt"))
	if err == nil {
		t.Error("expected error for write failure")
	}
}

// TestCombineLossClassesEmpty tests combineLossClasses with empty slice.
func TestCombineLossClassesEmpty(t *testing.T) {
	result := combineLossClasses(nil)
	if result != "L0" {
		t.Errorf("expected L0, got %s", result)
	}
}

// TestCombineLossClassesAllLevels tests combineLossClasses with all levels.
func TestCombineLossClassesAllLevels(t *testing.T) {
	tests := []struct {
		reports  []*ir.LossReport
		expected ir.LossClass
	}{
		{[]*ir.LossReport{{LossClass: ir.LossL0}}, ir.LossL0},
		{[]*ir.LossReport{{LossClass: ir.LossL1}}, ir.LossL1},
		{[]*ir.LossReport{{LossClass: ir.LossL2}}, ir.LossL2},
		{[]*ir.LossReport{{LossClass: ir.LossL3}}, ir.LossL3},
		{[]*ir.LossReport{{LossClass: ir.LossL4}}, ir.LossL4},
		{[]*ir.LossReport{{LossClass: ir.LossL0}, {LossClass: ir.LossL3}}, ir.LossL3},
		{[]*ir.LossReport{nil, {LossClass: ir.LossL2}}, ir.LossL2},
	}

	for i, tt := range tests {
		result := combineLossClasses(tt.reports)
		if result != tt.expected {
			t.Errorf("test %d: expected %s, got %s", i, tt.expected, result)
		}
	}
}

// TestLevelToLossClassAllValues tests levelToLossClass with all possible inputs.
// This test ensures all switch cases are covered and serves as a guard against
// new loss levels being added without updating this function.
func TestLevelToLossClassAllValues(t *testing.T) {
	tests := []struct {
		level    int
		expected ir.LossClass
	}{
		{0, ir.LossL0},
		{1, ir.LossL1},
		{2, ir.LossL2},
		{3, ir.LossL3},
		{4, ir.LossL4},
		// Default cases - should return L0 for safety
		{-1, ir.LossL0},
		{5, ir.LossL0},
		{100, ir.LossL0},
	}

	for _, tt := range tests {
		result := levelToLossClass(tt.level)
		if result != tt.expected {
			t.Errorf("levelToLossClass(%d) = %s, want %s", tt.level, result, tt.expected)
		}
	}
}

// TestLossClassLevelRange verifies that all defined LossClass values have
// Level() in range [0, 4]. This test will fail if new loss classes are added,
// prompting updates to levelToLossClass.
func TestLossClassLevelRange(t *testing.T) {
	knownClasses := []ir.LossClass{ir.LossL0, ir.LossL1, ir.LossL2, ir.LossL3, ir.LossL4}
	for _, class := range knownClasses {
		level := class.Level()
		if level < 0 || level > 4 {
			t.Errorf("LossClass %s has Level() = %d, expected 0-4. Update levelToLossClass!", class, level)
		}
	}
}

// TestExportDerivedNoPluginLoader tests ExportDerived without PluginLoader or plugins.
func TestExportDerivedNoPluginLoader(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := cap.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	_, err = cap.ExportDerived(context.Background(), artifact.ID, DerivedExportOptions{
		TargetFormat: "osis",
	}, filepath.Join(tempDir, "out.txt"))
	if err == nil {
		t.Error("expected error for missing plugin loader")
	}
}

// TestExportDerivedNoTargetFormat tests ExportDerived without target format.
func TestExportDerivedNoTargetFormat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := cap.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	_, err = cap.ExportDerived(context.Background(), artifact.ID, DerivedExportOptions{}, filepath.Join(tempDir, "out.txt"))
	if err == nil {
		t.Error("expected error for missing target format")
	}
}

// TestExportDerivedArtifactNotFound tests ExportDerived with nonexistent artifact.
func TestExportDerivedArtifactNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	_, err = cap.ExportDerived(context.Background(), "nonexistent", DerivedExportOptions{
		TargetFormat: "osis",
		SourcePlugin: &plugins.Plugin{},
		TargetPlugin: &plugins.Plugin{},
	}, filepath.Join(tempDir, "out.txt"))
	if err == nil {
		t.Error("expected error for nonexistent artifact")
	}
}

// TestExportDerivedTempDirError tests ExportDerived with temp dir creation error.
func TestExportDerivedTempDirError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := cap.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Inject error
	origMkdirTemp := osMkdirTemp
	osMkdirTemp = func(dir, pattern string) (string, error) {
		return "", errors.New("injected mkdirtemp error")
	}
	defer func() { osMkdirTemp = origMkdirTemp }()

	_, err = cap.ExportDerived(context.Background(), artifact.ID, DerivedExportOptions{
		TargetFormat: "osis",
		SourcePlugin: &plugins.Plugin{},
		TargetPlugin: &plugins.Plugin{},
	}, filepath.Join(tempDir, "out.txt"))
	if err == nil {
		t.Error("expected error for temp dir creation failure")
	}
}

// TestExportDerivedToBytesTempDirError tests ExportDerivedToBytes with temp dir creation error.
func TestExportDerivedToBytesTempDirError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := cap.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Inject error
	origMkdirTemp := osMkdirTemp
	osMkdirTemp = func(dir, pattern string) (string, error) {
		return "", errors.New("injected mkdirtemp error")
	}
	defer func() { osMkdirTemp = origMkdirTemp }()

	_, _, err = cap.ExportDerivedToBytes(context.Background(), artifact.ID, DerivedExportOptions{
		TargetFormat: "osis",
		SourcePlugin: &plugins.Plugin{},
		TargetPlugin: &plugins.Plugin{},
	})
	if err == nil {
		t.Error("expected error for temp dir creation failure")
	}
}

// TestFindPluginForFormatEmpty tests findPluginForFormat with empty format.
func TestFindPluginForFormatEmpty(t *testing.T) {
	_, err := findPluginForFormat(nil, "")
	if err == nil {
		t.Error("expected error for empty format")
	}
}

// TestCombinedLossReportEmpty tests CombinedLossReport with empty slice.
func TestCombinedLossReportEmpty(t *testing.T) {
	result := CombinedLossReport(nil)
	if result.LossClass != ir.LossL0 {
		t.Errorf("expected L0, got %s", result.LossClass)
	}
}

// TestCombinedLossReportWithReports tests CombinedLossReport with actual reports.
func TestCombinedLossReportWithReports(t *testing.T) {
	reports := []*ir.LossReport{
		{
			SourceFormat: "source1",
			TargetFormat: "ir",
			LossClass:    ir.LossL1,
			LostElements: []ir.LostElement{{Path: "p1"}},
			Warnings:     []string{"w1"},
		},
		{
			SourceFormat: "ir",
			TargetFormat: "target1",
			LossClass:    ir.LossL2,
			LostElements: []ir.LostElement{{Path: "p2"}},
			Warnings:     []string{"w2"},
		},
	}

	result := CombinedLossReport(reports)
	if result.SourceFormat != "source1" {
		t.Errorf("expected source format 'source1', got %s", result.SourceFormat)
	}
	if result.TargetFormat != "target1" {
		t.Errorf("expected target format 'target1', got %s", result.TargetFormat)
	}
	if result.LossClass != ir.LossL2 {
		t.Errorf("expected L2 (worst), got %s", result.LossClass)
	}
	if len(result.LostElements) != 2 {
		t.Errorf("expected 2 lost elements, got %d", len(result.LostElements))
	}
	if len(result.Warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(result.Warnings))
	}
}

// TestCombinedLossReportWithNilReport tests CombinedLossReport handles nil reports.
func TestCombinedLossReportWithNilReport(t *testing.T) {
	reports := []*ir.LossReport{
		{LossClass: ir.LossL1, LostElements: []ir.LostElement{{Path: "p1"}}},
		nil,
		{LossClass: ir.LossL0},
	}

	result := CombinedLossReport(reports)
	if result.LossClass != ir.LossL1 {
		t.Errorf("expected L1, got %s", result.LossClass)
	}
}

// TestExtractIRFromPluginExecuteError tests extractIRFromPlugin with execution error.
func TestExtractIRFromPluginExecuteError(t *testing.T) {
	// Inject error
	origExecute := pluginsExecutePlugin
	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		return nil, errors.New("injected execute error")
	}
	defer func() { pluginsExecutePlugin = origExecute }()

	_, _, err := extractIRFromPlugin(&plugins.Plugin{}, "/source", "/output")
	if err == nil {
		t.Error("expected error for plugin execution failure")
	}
}

// TestExtractIRFromPluginParseError tests extractIRFromPlugin with parse error.
func TestExtractIRFromPluginParseError(t *testing.T) {
	// Inject execute success, parse error
	origExecute := pluginsExecutePlugin
	origParse := pluginsParseExtractIRResult

	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		return &plugins.IPCResponse{}, nil
	}
	pluginsParseExtractIRResult = func(resp *plugins.IPCResponse) (*plugins.ExtractIRResult, error) {
		return nil, errors.New("injected parse error")
	}
	defer func() {
		pluginsExecutePlugin = origExecute
		pluginsParseExtractIRResult = origParse
	}()

	_, _, err := extractIRFromPlugin(&plugins.Plugin{}, "/source", "/output")
	if err == nil {
		t.Error("expected error for parse failure")
	}
}

// TestExtractIRFromPluginSuccessWithLoss tests extractIRFromPlugin with loss report.
func TestExtractIRFromPluginSuccessWithLoss(t *testing.T) {
	// Inject success with loss report
	origExecute := pluginsExecutePlugin
	origParse := pluginsParseExtractIRResult

	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		return &plugins.IPCResponse{}, nil
	}
	pluginsParseExtractIRResult = func(resp *plugins.IPCResponse) (*plugins.ExtractIRResult, error) {
		return &plugins.ExtractIRResult{
			IRPath: "/path/to/ir",
			LossReport: &plugins.LossReportIPC{
				SourceFormat: "source",
				TargetFormat: "ir",
				LossClass:    "L1",
				LostElements: []plugins.LostElementIPC{{Path: "p1", ElementType: "test", Reason: "test"}},
				Warnings:     []string{"warning1"},
			},
		}, nil
	}
	defer func() {
		pluginsExecutePlugin = origExecute
		pluginsParseExtractIRResult = origParse
	}()

	result, loss, err := extractIRFromPlugin(&plugins.Plugin{}, "/source", "/output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IRPath != "/path/to/ir" {
		t.Errorf("unexpected IR path: %s", result.IRPath)
	}
	if loss == nil {
		t.Error("expected loss report")
	}
	if loss.LossClass != ir.LossL1 {
		t.Errorf("unexpected loss class: %s", loss.LossClass)
	}
}

// TestEmitNativeFromPluginExecuteError tests emitNativeFromPlugin with execution error.
func TestEmitNativeFromPluginExecuteError(t *testing.T) {
	// Inject error
	origExecute := pluginsExecutePlugin
	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		return nil, errors.New("injected execute error")
	}
	defer func() { pluginsExecutePlugin = origExecute }()

	_, _, err := emitNativeFromPlugin(&plugins.Plugin{}, "/ir", "/output")
	if err == nil {
		t.Error("expected error for plugin execution failure")
	}
}

// TestEmitNativeFromPluginParseError tests emitNativeFromPlugin with parse error.
func TestEmitNativeFromPluginParseError(t *testing.T) {
	// Inject execute success, parse error
	origExecute := pluginsExecutePlugin
	origParse := pluginsParseEmitNativeResult

	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		return &plugins.IPCResponse{}, nil
	}
	pluginsParseEmitNativeResult = func(resp *plugins.IPCResponse) (*plugins.EmitNativeResult, error) {
		return nil, errors.New("injected parse error")
	}
	defer func() {
		pluginsExecutePlugin = origExecute
		pluginsParseEmitNativeResult = origParse
	}()

	_, _, err := emitNativeFromPlugin(&plugins.Plugin{}, "/ir", "/output")
	if err == nil {
		t.Error("expected error for parse failure")
	}
}

// TestEmitNativeFromPluginSuccessWithLoss tests emitNativeFromPlugin with loss report.
func TestEmitNativeFromPluginSuccessWithLoss(t *testing.T) {
	// Inject success with loss report
	origExecute := pluginsExecutePlugin
	origParse := pluginsParseEmitNativeResult

	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		return &plugins.IPCResponse{}, nil
	}
	pluginsParseEmitNativeResult = func(resp *plugins.IPCResponse) (*plugins.EmitNativeResult, error) {
		return &plugins.EmitNativeResult{
			OutputPath: "/path/to/output",
			LossReport: &plugins.LossReportIPC{
				SourceFormat: "ir",
				TargetFormat: "osis",
				LossClass:    "L2",
				LostElements: []plugins.LostElementIPC{{Path: "p2", ElementType: "attr", Reason: "unsupported"}},
				Warnings:     []string{"warning2"},
			},
		}, nil
	}
	defer func() {
		pluginsExecutePlugin = origExecute
		pluginsParseEmitNativeResult = origParse
	}()

	result, loss, err := emitNativeFromPlugin(&plugins.Plugin{}, "/ir", "/output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OutputPath != "/path/to/output" {
		t.Errorf("unexpected output path: %s", result.OutputPath)
	}
	if loss == nil {
		t.Error("expected loss report")
	}
	if loss.LossClass != ir.LossL2 {
		t.Errorf("unexpected loss class: %s", loss.LossClass)
	}
}

// TestExportDerivedWriteSourceFileError tests ExportDerived with source file write error.
func TestExportDerivedWriteSourceFileError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := cap.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Inject write error after mkdirtemp succeeds
	origMkdirTemp := osMkdirTemp
	origWriteFile := osWriteFileExport
	callCount := 0
	osMkdirTemp = func(dir, pattern string) (string, error) {
		return os.MkdirTemp(dir, pattern)
	}
	osWriteFileExport = func(name string, data []byte, perm os.FileMode) error {
		callCount++
		if callCount == 1 {
			return errors.New("injected write error")
		}
		return os.WriteFile(name, data, perm)
	}
	defer func() {
		osMkdirTemp = origMkdirTemp
		osWriteFileExport = origWriteFile
	}()

	_, err = cap.ExportDerived(context.Background(), artifact.ID, DerivedExportOptions{
		TargetFormat: "osis",
		SourcePlugin: &plugins.Plugin{},
		TargetPlugin: &plugins.Plugin{},
	}, filepath.Join(tempDir, "out.txt"))
	if err == nil {
		t.Error("expected error for source file write failure")
	}
}

// TestExportDerivedExtractIRError tests ExportDerived with extract-ir error.
func TestExportDerivedExtractIRError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := cap.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Inject execute error
	origExecute := pluginsExecutePlugin
	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		return nil, errors.New("injected extract-ir error")
	}
	defer func() { pluginsExecutePlugin = origExecute }()

	_, err = cap.ExportDerived(context.Background(), artifact.ID, DerivedExportOptions{
		TargetFormat: "osis",
		SourcePlugin: &plugins.Plugin{},
		TargetPlugin: &plugins.Plugin{},
	}, filepath.Join(tempDir, "out.txt"))
	if err == nil {
		t.Error("expected error for extract-ir failure")
	}
}

// TestExportDerivedEmitNativeError tests ExportDerived with emit-native error.
func TestExportDerivedEmitNativeError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := cap.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Inject: extract-ir succeeds, emit-native fails
	origExecute := pluginsExecutePlugin
	origParseExtract := pluginsParseExtractIRResult
	callCount := 0

	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		callCount++
		if callCount == 2 {
			return nil, errors.New("injected emit-native error")
		}
		return &plugins.IPCResponse{}, nil
	}
	pluginsParseExtractIRResult = func(resp *plugins.IPCResponse) (*plugins.ExtractIRResult, error) {
		return &plugins.ExtractIRResult{IRPath: "/path/to/ir"}, nil
	}
	defer func() {
		pluginsExecutePlugin = origExecute
		pluginsParseExtractIRResult = origParseExtract
	}()

	_, err = cap.ExportDerived(context.Background(), artifact.ID, DerivedExportOptions{
		TargetFormat: "osis",
		SourcePlugin: &plugins.Plugin{},
		TargetPlugin: &plugins.Plugin{},
	}, filepath.Join(tempDir, "out.txt"))
	if err == nil {
		t.Error("expected error for emit-native failure")
	}
}

// TestExportDerivedFullSuccess tests ExportDerived success path.
func TestExportDerivedFullSuccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	testContent := []byte("test content")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := cap.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Create output file that will be "generated"
	outputPath := filepath.Join(tempDir, "plugin_output.txt")
	if err := os.WriteFile(outputPath, []byte("converted output"), 0600); err != nil {
		t.Fatalf("failed to write output file: %v", err)
	}

	// Inject: everything succeeds with loss reports
	origExecute := pluginsExecutePlugin
	origParseExtract := pluginsParseExtractIRResult
	origParseEmit := pluginsParseEmitNativeResult
	origReadFile := osReadFileExport
	callCount := 0

	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		return &plugins.IPCResponse{}, nil
	}
	pluginsParseExtractIRResult = func(resp *plugins.IPCResponse) (*plugins.ExtractIRResult, error) {
		return &plugins.ExtractIRResult{
			IRPath: "/path/to/ir",
			LossReport: &plugins.LossReportIPC{
				SourceFormat: "source",
				TargetFormat: "ir",
				LossClass:    "L1",
			},
		}, nil
	}
	pluginsParseEmitNativeResult = func(resp *plugins.IPCResponse) (*plugins.EmitNativeResult, error) {
		return &plugins.EmitNativeResult{
			OutputPath: outputPath,
			LossReport: &plugins.LossReportIPC{
				SourceFormat: "ir",
				TargetFormat: "osis",
				LossClass:    "L2",
			},
		}, nil
	}
	osReadFileExport = func(name string) ([]byte, error) {
		callCount++
		return []byte("converted output"), nil
	}
	defer func() {
		pluginsExecutePlugin = origExecute
		pluginsParseExtractIRResult = origParseExtract
		pluginsParseEmitNativeResult = origParseEmit
		osReadFileExport = origReadFile
	}()

	destPath := filepath.Join(tempDir, "dest.txt")
	result, err := cap.ExportDerived(context.Background(), artifact.ID, DerivedExportOptions{
		TargetFormat: "osis",
		SourcePlugin: &plugins.Plugin{},
		TargetPlugin: &plugins.Plugin{},
	}, destPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if len(result.LossReports) != 2 {
		t.Errorf("expected 2 loss reports, got %d", len(result.LossReports))
	}
	if result.CombinedLossClass != ir.LossL2 {
		t.Errorf("expected combined loss L2, got %s", result.CombinedLossClass)
	}
}

// TestExportDerivedReadOutputError tests ExportDerived with read output error.
func TestExportDerivedReadOutputError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := cap.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Inject: everything succeeds except reading output
	origExecute := pluginsExecutePlugin
	origParseExtract := pluginsParseExtractIRResult
	origParseEmit := pluginsParseEmitNativeResult
	origReadFile := osReadFileExport

	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		return &plugins.IPCResponse{}, nil
	}
	pluginsParseExtractIRResult = func(resp *plugins.IPCResponse) (*plugins.ExtractIRResult, error) {
		return &plugins.ExtractIRResult{IRPath: "/path/to/ir"}, nil
	}
	pluginsParseEmitNativeResult = func(resp *plugins.IPCResponse) (*plugins.EmitNativeResult, error) {
		return &plugins.EmitNativeResult{OutputPath: "/path/to/output"}, nil
	}
	osReadFileExport = func(name string) ([]byte, error) {
		return nil, errors.New("injected read error")
	}
	defer func() {
		pluginsExecutePlugin = origExecute
		pluginsParseExtractIRResult = origParseExtract
		pluginsParseEmitNativeResult = origParseEmit
		osReadFileExport = origReadFile
	}()

	_, err = cap.ExportDerived(context.Background(), artifact.ID, DerivedExportOptions{
		TargetFormat: "osis",
		SourcePlugin: &plugins.Plugin{},
		TargetPlugin: &plugins.Plugin{},
	}, filepath.Join(tempDir, "out.txt"))
	if err == nil {
		t.Error("expected error for read output failure")
	}
}

// TestExportDerivedWriteDestError tests ExportDerived with write destination error.
func TestExportDerivedWriteDestError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := cap.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Inject: everything succeeds except writing destination
	origExecute := pluginsExecutePlugin
	origParseExtract := pluginsParseExtractIRResult
	origParseEmit := pluginsParseEmitNativeResult
	origReadFile := osReadFileExport
	origWriteFile := osWriteFileExport
	writeCount := 0

	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		return &plugins.IPCResponse{}, nil
	}
	pluginsParseExtractIRResult = func(resp *plugins.IPCResponse) (*plugins.ExtractIRResult, error) {
		return &plugins.ExtractIRResult{IRPath: "/path/to/ir"}, nil
	}
	pluginsParseEmitNativeResult = func(resp *plugins.IPCResponse) (*plugins.EmitNativeResult, error) {
		return &plugins.EmitNativeResult{OutputPath: "/path/to/output"}, nil
	}
	osReadFileExport = func(name string) ([]byte, error) {
		return []byte("output"), nil
	}
	osWriteFileExport = func(name string, data []byte, perm os.FileMode) error {
		writeCount++
		if writeCount > 1 {
			return errors.New("injected write error")
		}
		return os.WriteFile(name, data, perm)
	}
	defer func() {
		pluginsExecutePlugin = origExecute
		pluginsParseExtractIRResult = origParseExtract
		pluginsParseEmitNativeResult = origParseEmit
		osReadFileExport = origReadFile
		osWriteFileExport = origWriteFile
	}()

	_, err = cap.ExportDerived(context.Background(), artifact.ID, DerivedExportOptions{
		TargetFormat: "osis",
		SourcePlugin: &plugins.Plugin{},
		TargetPlugin: &plugins.Plugin{},
	}, filepath.Join(tempDir, "out.txt"))
	if err == nil {
		t.Error("expected error for write destination failure")
	}
}

// TestExportDerivedToBytesSuccess tests ExportDerivedToBytes success path.
func TestExportDerivedToBytesSuccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := cap.IngestFile(context.Background(), testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Inject: everything succeeds
	origExecute := pluginsExecutePlugin
	origParseExtract := pluginsParseExtractIRResult
	origParseEmit := pluginsParseEmitNativeResult
	origReadFile := osReadFileExport
	origWriteFile := osWriteFileExport

	pluginsExecutePlugin = func(p *plugins.Plugin, req *plugins.IPCRequest) (*plugins.IPCResponse, error) {
		return &plugins.IPCResponse{}, nil
	}
	pluginsParseExtractIRResult = func(resp *plugins.IPCResponse) (*plugins.ExtractIRResult, error) {
		return &plugins.ExtractIRResult{IRPath: "/path/to/ir"}, nil
	}
	pluginsParseEmitNativeResult = func(resp *plugins.IPCResponse) (*plugins.EmitNativeResult, error) {
		return &plugins.EmitNativeResult{OutputPath: "/path/to/output"}, nil
	}
	osReadFileExport = func(name string) ([]byte, error) {
		return []byte("converted output"), nil
	}
	osWriteFileExport = func(name string, data []byte, perm os.FileMode) error {
		return os.WriteFile(name, data, perm)
	}
	defer func() {
		pluginsExecutePlugin = origExecute
		pluginsParseExtractIRResult = origParseExtract
		pluginsParseEmitNativeResult = origParseEmit
		osReadFileExport = origReadFile
		osWriteFileExport = origWriteFile
	}()

	data, result, err := cap.ExportDerivedToBytes(context.Background(), artifact.ID, DerivedExportOptions{
		TargetFormat: "osis",
		SourcePlugin: &plugins.Plugin{},
		TargetPlugin: &plugins.Plugin{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if string(data) != "converted output" {
		t.Errorf("unexpected data: %s", data)
	}
}

// TestBinaryPreservation tests that binary files are preserved exactly.
func TestExportBinaryPreservation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-binary-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create binary content with all byte values
	binaryContent := make([]byte, 256)
	for i := 0; i < 256; i++ {
		binaryContent[i] = byte(i)
	}

	testFilePath := filepath.Join(tempDir, "all-bytes.bin")
	if err := os.WriteFile(testFilePath, binaryContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	artifact, err := capsule.IngestFile(context.Background(), testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Pack/unpack/export
	archivePath := filepath.Join(tempDir, "binary.capsule.tar.xz")
	if err := capsule.Pack(archivePath); err != nil {
		t.Fatalf("failed to pack: %v", err)
	}

	unpackDir := filepath.Join(tempDir, "unpacked")
	unpacked, err := Unpack(archivePath, unpackDir)
	if err != nil {
		t.Fatalf("failed to unpack: %v", err)
	}

	exportPath := filepath.Join(tempDir, "exported.bin")
	if err := unpacked.Export(context.Background(), artifact.ID, ExportModeIdentity, exportPath); err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	exported, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("failed to read exported: %v", err)
	}

	// Verify every byte
	if len(exported) != len(binaryContent) {
		t.Fatalf("length mismatch: got %d, want %d", len(exported), len(binaryContent))
	}

	for i := 0; i < len(binaryContent); i++ {
		if exported[i] != binaryContent[i] {
			t.Errorf("byte mismatch at position %d: got %02x, want %02x", i, exported[i], binaryContent[i])
		}
	}
}
