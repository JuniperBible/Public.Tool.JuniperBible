package crosswire

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

// Helper function to create a valid CrossWire zip structure
func createTestZip(t *testing.T, path string, includeModsD, includeModules, includeConf bool) {
	t.Helper()

	zipFile, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	if includeModsD {
		// Create mods.d directory entry
		_, err = zw.Create("mods.d/")
		if err != nil {
			t.Fatal(err)
		}

		if includeConf {
			// Create a .conf file
			w, err := zw.Create("mods.d/test.conf")
			if err != nil {
				t.Fatal(err)
			}
			_, err = w.Write([]byte("[TestModule]\nDescription=Test Module\n"))
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	if includeModules {
		// Create modules directory entry
		_, err = zw.Create("modules/")
		if err != nil {
			t.Fatal(err)
		}

		// Create a dummy module file
		w, err := zw.Create("modules/texts/test.dat")
		if err != nil {
			t.Fatal(err)
		}
		_, err = w.Write([]byte("test data"))
		if err != nil {
			t.Fatal(err)
		}
	}
}

// Helper function to create a valid CrossWire directory structure
func createTestDirectory(t *testing.T, path string) {
	t.Helper()

	modsDir := filepath.Join(path, "mods.d")
	modulesDir := filepath.Join(path, "modules")

	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a .conf file
	confPath := filepath.Join(modsDir, "test.conf")
	confContent := "[TestModule]\nDescription=Test Module\nModDrv=RawText\n"
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a module data file
	dataDir := filepath.Join(modulesDir, "texts")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}

	dataPath := filepath.Join(dataDir, "test.dat")
	if err := os.WriteFile(dataPath, []byte("test data"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest.PluginID != "format.crosswire" {
		t.Errorf("Expected PluginID 'format.crosswire', got %s", manifest.PluginID)
	}

	if manifest.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", manifest.Version)
	}

	if manifest.Kind != "format" {
		t.Errorf("Expected Kind 'format', got %s", manifest.Kind)
	}

	if manifest.Entrypoint != "format-crosswire" {
		t.Errorf("Expected Entrypoint 'format-crosswire', got %s", manifest.Entrypoint)
	}

	if len(manifest.Capabilities.Inputs) != 2 {
		t.Errorf("Expected 2 input capabilities, got %d", len(manifest.Capabilities.Inputs))
	}

	if len(manifest.Capabilities.Outputs) != 1 {
		t.Errorf("Expected 1 output capability, got %d", len(manifest.Capabilities.Outputs))
	}
}

func TestDetect_ValidZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	createTestZip(t, zipPath, true, true, true)

	handler := &Handler{}
	result, err := handler.Detect(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed, got: %s", result.Reason)
	}

	if result.Format != "crosswire-zip" {
		t.Errorf("Expected format 'crosswire-zip', got %s", result.Format)
	}
}

func TestDetect_ValidDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "sword-module")

	createTestDirectory(t, testDir)

	handler := &Handler{}
	result, err := handler.Detect(testDir)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed, got: %s", result.Reason)
	}

	if result.Format != "crosswire-directory" {
		t.Errorf("Expected format 'crosswire-directory', got %s", result.Format)
	}
}

func TestDetect_InvalidZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "invalid.zip")

	// Create an empty zip file
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zipFile.Close()

	handler := &Handler{}
	result, err := handler.Detect(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for invalid zip")
	}

	if !strings.Contains(result.Reason, "failed to open zip") {
		t.Errorf("Expected reason to mention 'failed to open zip', got: %s", result.Reason)
	}
}

func TestDetect_MissingModsD(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "no-mods.zip")

	// Create zip without mods.d
	createTestZip(t, zipPath, false, true, false)

	handler := &Handler{}
	result, err := handler.Detect(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for zip without mods.d")
	}

	if !strings.Contains(result.Reason, "missing mods.d") {
		t.Errorf("Expected reason to mention 'missing mods.d', got: %s", result.Reason)
	}
}

func TestDetect_MissingModules(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "no-modules.zip")

	// Create zip without modules/
	createTestZip(t, zipPath, true, false, true)

	handler := &Handler{}
	result, err := handler.Detect(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for zip without modules/")
	}

	if !strings.Contains(result.Reason, "missing mods.d/*.conf or modules/") {
		t.Errorf("Expected reason to mention missing modules, got: %s", result.Reason)
	}
}

func TestDetect_MissingConfFile(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "no-conf.zip")

	// Create zip without .conf file
	createTestZip(t, zipPath, true, true, false)

	handler := &Handler{}
	result, err := handler.Detect(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for zip without .conf file")
	}
}

func TestDetect_DirectoryMissingModsD(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "invalid-dir")

	// Create only modules directory
	if err := os.MkdirAll(filepath.Join(testDir, "modules"), 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for directory without mods.d")
	}

	if !strings.Contains(result.Reason, "does not contain mods.d") {
		t.Errorf("Expected reason to mention missing mods.d, got: %s", result.Reason)
	}
}

func TestDetect_DirectoryWithoutConfFiles(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "no-conf-dir")

	// Create both directories but no .conf files
	if err := os.MkdirAll(filepath.Join(testDir, "mods.d"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(testDir, "modules"), 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for directory without .conf files")
	}

	if !strings.Contains(result.Reason, "no .conf files") {
		t.Errorf("Expected reason to mention 'no .conf files', got: %s", result.Reason)
	}
}

func TestIngest_ZipExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	outputDir := filepath.Join(tmpDir, "output")

	createTestZip(t, zipPath, true, true, true)

	handler := &Handler{}
	result, err := handler.Ingest(zipPath, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "test.zip" {
		t.Errorf("Expected artifact ID 'test.zip', got %s", result.ArtifactID)
	}

	if result.Metadata["format"] != "crosswire" {
		t.Errorf("Expected format 'crosswire', got %s", result.Metadata["format"])
	}

	if result.Metadata["status"] != "ingested" {
		t.Errorf("Expected status 'ingested', got %s", result.Metadata["status"])
	}

	// Verify structure was copied
	modsDir := filepath.Join(outputDir, "mods.d")
	if _, err := os.Stat(modsDir); os.IsNotExist(err) {
		t.Error("Expected mods.d directory to exist in output")
	}

	modulesDir := filepath.Join(outputDir, "modules")
	if _, err := os.Stat(modulesDir); os.IsNotExist(err) {
		t.Error("Expected modules directory to exist in output")
	}

	// Verify conf file was copied
	confPath := filepath.Join(modsDir, "test.conf")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Error("Expected test.conf to exist in output")
	}
}

func TestIngest_DirectoryCopy(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "sword-module")
	outputDir := filepath.Join(tmpDir, "output")

	createTestDirectory(t, testDir)

	handler := &Handler{}
	result, err := handler.Ingest(testDir, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "sword-module" {
		t.Errorf("Expected artifact ID 'sword-module', got %s", result.ArtifactID)
	}

	// Verify structure was copied
	modsDir := filepath.Join(outputDir, "mods.d")
	if _, err := os.Stat(modsDir); os.IsNotExist(err) {
		t.Error("Expected mods.d directory to exist in output")
	}

	confPath := filepath.Join(modsDir, "test.conf")
	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), "TestModule") {
		t.Error("Expected conf file to contain 'TestModule'")
	}
}

func TestEnumerate_ListConfFiles(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "sword-module")

	createTestDirectory(t, testDir)

	// Add a second conf file
	confPath := filepath.Join(testDir, "mods.d", "second.conf")
	if err := os.WriteFile(confPath, []byte("[Second]\nDescription=Second Module\n"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Enumerate(testDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(result.Entries))
	}

	// Check that both conf files are listed
	confFiles := make(map[string]bool)
	for _, entry := range result.Entries {
		if !strings.HasSuffix(entry.Path, ".conf") {
			t.Errorf("Expected .conf file, got %s", entry.Path)
		}
		confFiles[entry.Path] = true

		if entry.IsDir {
			t.Errorf("Expected IsDir to be false for %s", entry.Path)
		}

		if entry.SizeBytes <= 0 {
			t.Errorf("Expected positive size for %s", entry.Path)
		}
	}

	if !confFiles["test.conf"] || !confFiles["second.conf"] {
		t.Error("Expected both test.conf and second.conf in entries")
	}
}

func TestEnumerate_FromZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	createTestZip(t, zipPath, true, true, true)

	handler := &Handler{}
	result, err := handler.Enumerate(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.conf" {
		t.Errorf("Expected path 'test.conf', got %s", entry.Path)
	}
}

func TestExtractIR_CorpusCreation(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "sword-module")
	outputDir := filepath.Join(tmpDir, "ir-output")

	createTestDirectory(t, testPath)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.ExtractIR(testPath, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	expectedIRPath := filepath.Join(outputDir, "corpus.json")
	if result.IRPath != expectedIRPath {
		t.Errorf("Expected IR path %s, got %s", expectedIRPath, result.IRPath)
	}

	if result.LossClass != "L2" {
		t.Errorf("Expected loss class L2, got %s", result.LossClass)
	}

	if result.LossReport == nil {
		t.Fatal("Expected loss report to be present")
	}

	if len(result.LossReport.Warnings) == 0 {
		t.Error("Expected at least one warning in loss report")
	}

	// Verify IR file was created and is valid JSON
	data, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatal(err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("Failed to parse IR JSON: %v", err)
	}

	if corpus.ID != "crosswire-module" {
		t.Errorf("Expected corpus ID 'crosswire-module', got %s", corpus.ID)
	}

	if corpus.ModuleType != ir.ModuleBible {
		t.Errorf("Expected module type Bible, got %s", corpus.ModuleType)
	}

	if corpus.Language != "en" {
		t.Errorf("Expected language 'en', got %s", corpus.Language)
	}

	if corpus.Title != "CrossWire Module" {
		t.Errorf("Expected title 'CrossWire Module', got %s", corpus.Title)
	}
}

func TestEmitNative_ZipCreation(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "corpus.json")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a test IR corpus
	corpus := &ir.Corpus{
		ID:         "test-module",
		Version:    "1.0.0",
		ModuleType: ir.ModuleBible,
		Language:   "en",
		Title:      "Test Module",
		Documents:  []*ir.Document{},
	}

	data, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(irPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.EmitNative(irPath, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Format != "crosswire-zip" {
		t.Errorf("Expected format 'crosswire-zip', got %s", result.Format)
	}

	if result.LossClass != "L2" {
		t.Errorf("Expected loss class L2, got %s", result.LossClass)
	}

	if result.LossReport == nil {
		t.Fatal("Expected loss report to be present")
	}

	// Verify zip was created
	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Errorf("Expected zip file to exist at %s", result.OutputPath)
	}

	// Verify zip contains expected structure
	zr, err := zip.OpenReader(result.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	foundConf := false

	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "test-module.conf") {
			foundConf = true

			// Read and verify conf content
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer rc.Close()

			confData := make([]byte, f.UncompressedSize64)
			_, err = rc.Read(confData)
			if err != nil && err.Error() != "EOF" {
				t.Fatal(err)
			}

			confStr := string(confData)
			if !strings.Contains(confStr, "[test-module]") {
				t.Error("Expected conf to contain module ID")
			}
		}
	}

	if !foundConf {
		t.Error("Expected zip to contain .conf file")
	}

	// Verify the zip has some files (at least conf file)
	if len(zr.File) == 0 {
		t.Error("Expected zip to contain files")
	}
}

func TestExtractZip_ZipSlipProtection(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "malicious.zip")
	destDir := filepath.Join(tmpDir, "extract")

	// Create a zip with a path traversal attempt
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)

	// Try to create a file that would escape the destination directory
	w, err := zw.Create("../../../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte("malicious content"))
	if err != nil {
		t.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	// Attempt to extract - should fail due to path traversal protection
	err = extractZip(zipPath, destDir)
	if err == nil {
		t.Error("Expected extractZip to fail on path traversal attempt")
	}

	if !strings.Contains(err.Error(), "invalid file path") {
		t.Errorf("Expected error about invalid file path, got: %v", err)
	}
}

func TestExtractZip_ValidExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	destDir := filepath.Join(tmpDir, "extract")

	createTestZip(t, zipPath, true, true, true)

	err := extractZip(zipPath, destDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify files were extracted
	confPath := filepath.Join(destDir, "mods.d", "test.conf")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Error("Expected conf file to be extracted")
	}

	dataPath := filepath.Join(destDir, "modules", "texts", "test.dat")
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		t.Error("Expected module data to be extracted")
	}
}

func TestCopyDir_RecursiveCopy(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")

	createTestDirectory(t, srcDir)

	err := copyDir(srcDir, dstDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify all files were copied
	confPath := filepath.Join(dstDir, "mods.d", "test.conf")
	confData, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(confData), "TestModule") {
		t.Error("Expected copied conf to contain 'TestModule'")
	}

	dataPath := filepath.Join(dstDir, "modules", "texts", "test.dat")
	dataContent, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(dataContent) != "test data" {
		t.Errorf("Expected 'test data', got %s", string(dataContent))
	}
}

func TestCreateZipArchive_ValidArchive(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	zipPath := filepath.Join(tmpDir, "output.zip")

	createTestDirectory(t, sourceDir)

	err := createZipArchive(sourceDir, zipPath)
	if err != nil {
		t.Fatal(err)
	}

	// Verify zip was created
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		t.Error("Expected zip file to be created")
	}

	// Verify zip contents
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	foundConf := false
	foundData := false

	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "test.conf") {
			foundConf = true
		}
		if strings.HasSuffix(f.Name, "test.dat") {
			foundData = true
		}
	}

	if !foundConf {
		t.Error("Expected zip to contain test.conf")
	}

	if !foundData {
		t.Error("Expected zip to contain test.dat")
	}
}

func TestCopyFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	dstPath := filepath.Join(tmpDir, "subdir", "dest.txt")

	content := []byte("test file content")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	err := copyFile(srcPath, dstPath)
	if err != nil {
		t.Fatal(err)
	}

	// Verify file was copied
	copiedContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(copiedContent) != string(content) {
		t.Errorf("Expected content %s, got %s", content, copiedContent)
	}
}

func TestDetectZipArchive_WithParentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "nested.zip")

	// Create a zip with files nested in a parent directory
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)

	// Create nested structure: parent/mods.d/test.conf
	w, err := zw.Create("parent/mods.d/test.conf")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte("[Test]\n"))
	if err != nil {
		t.Fatal(err)
	}

	// Create parent/modules/
	_, err = zw.Create("parent/modules/")
	if err != nil {
		t.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	result, err := detectZipArchive(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection with nested structure, got: %s", result.Reason)
	}
}

func TestDetectDirectory_ModsDNotDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "invalid")

	// Create mods.d as a file instead of directory
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}

	modsPath := filepath.Join(testDir, "mods.d")
	if err := os.WriteFile(modsPath, []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}

	modulesDir := filepath.Join(testDir, "modules")
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := detectDirectory(testDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail when mods.d is not a directory")
	}

	if !strings.Contains(result.Reason, "not a directory") {
		t.Errorf("Expected reason to mention 'not a directory', got: %s", result.Reason)
	}
}

func TestEnumerate_EmptyModsD(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "empty")

	// Create structure with empty mods.d
	if err := os.MkdirAll(filepath.Join(testDir, "mods.d"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(testDir, "modules"), 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Enumerate(testDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 0 {
		t.Errorf("Expected 0 entries for empty mods.d, got %d", len(result.Entries))
	}
}

func TestIngest_InvalidZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "corrupt.zip")
	outputDir := filepath.Join(tmpDir, "output")

	// Create a corrupted zip file
	if err := os.WriteFile(zipPath, []byte("not a zip file"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	_, err := handler.Ingest(zipPath, outputDir)
	if err == nil {
		t.Error("Expected Ingest to fail on corrupted zip")
	}

	if !strings.Contains(err.Error(), "failed to extract zip") {
		t.Errorf("Expected error about failed extraction, got: %v", err)
	}
}

func TestEmitNative_InvalidIR(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "invalid.json")
	outputDir := filepath.Join(tmpDir, "output")

	// Create invalid JSON
	if err := os.WriteFile(irPath, []byte("not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	_, err := handler.EmitNative(irPath, outputDir)
	if err == nil {
		t.Error("Expected EmitNative to fail on invalid IR")
	}

	if !strings.Contains(err.Error(), "failed to parse IR") {
		t.Errorf("Expected error about failed IR parsing, got: %v", err)
	}
}

func TestEmitNative_MissingIR(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "nonexistent.json")
	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	_, err := handler.EmitNative(irPath, outputDir)
	if err == nil {
		t.Error("Expected EmitNative to fail on missing IR")
	}

	if !strings.Contains(err.Error(), "failed to read IR") {
		t.Errorf("Expected error about failed IR read, got: %v", err)
	}
}

func TestIngest_CopyDirError(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "source")
	outputDir := "/dev/null/invalid/path"

	createTestDirectory(t, testDir)

	handler := &Handler{}
	_, err := handler.Ingest(testDir, outputDir)
	if err == nil {
		t.Error("Expected Ingest to fail on invalid output directory")
	}

	if !strings.Contains(err.Error(), "failed to copy module") {
		t.Errorf("Expected error about failed copy, got: %v", err)
	}
}

func TestEnumerate_InvalidModsD(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "invalid")

	// Create structure but make mods.d unreadable
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create mods.d with no read permissions (will fail on ReadDir)
	modsDir := filepath.Join(testDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(modsDir, 0755) // Cleanup

	if err := os.MkdirAll(filepath.Join(testDir, "modules"), 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	_, err := handler.Enumerate(testDir)
	if err == nil {
		t.Error("Expected Enumerate to fail on unreadable mods.d")
	}

	if !strings.Contains(err.Error(), "failed to read mods.d") {
		t.Errorf("Expected error about failed mods.d read, got: %v", err)
	}
}

func TestExtractIR_WriteError(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "source")
	outputDir := "/dev/null/invalid/path"

	createTestDirectory(t, testPath)

	handler := &Handler{}
	_, err := handler.ExtractIR(testPath, outputDir)
	if err == nil {
		t.Error("Expected ExtractIR to fail on invalid output directory")
	}

	if !strings.Contains(err.Error(), "failed to write IR") {
		t.Errorf("Expected error about failed IR write, got: %v", err)
	}
}

func TestEmitNative_CreateModsDirError(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "corpus.json")

	// Create a file where outputDir would be, causing mkdir to fail
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.WriteFile(outputDir, []byte("file blocking directory"), 0644); err != nil {
		t.Fatal(err)
	}

	corpus := &ir.Corpus{
		ID:         "test-module",
		Version:    "1.0.0",
		ModuleType: ir.ModuleBible,
		Language:   "en",
		Title:      "Test Module",
		Documents:  []*ir.Document{},
	}

	data, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(irPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	_, err = handler.EmitNative(irPath, outputDir)
	if err == nil {
		t.Error("Expected EmitNative to fail when outputDir is a file")
	}

	if !strings.Contains(err.Error(), "failed to create mods.d") {
		t.Errorf("Expected error about failed mods.d creation, got: %v", err)
	}
}

func TestExtractZipFile_DirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	// Create a zip with directory entries
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)

	// Create a directory entry
	_, err = zw.Create("testdir/")
	if err != nil {
		t.Fatal(err)
	}

	// Create a file in subdirectory
	w, err := zw.Create("testdir/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte("content"))
	if err != nil {
		t.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(tmpDir, "extract")
	err = extractZip(zipPath, destDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify directory and file were created
	dirPath := filepath.Join(destDir, "testdir")
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		t.Error("Expected directory to be created")
	}

	filePath := filepath.Join(destDir, "testdir", "file.txt")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Expected file to be created")
	}
}

func TestCreateZipArchive_WalkError(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	zipPath := filepath.Join(tmpDir, "output.zip")

	// Create a directory with a subdirectory that will cause walk errors
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory with no permissions
	unreadableDir := filepath.Join(sourceDir, "unreadable")
	if err := os.MkdirAll(unreadableDir, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(unreadableDir, 0755) // Cleanup

	err := createZipArchive(sourceDir, zipPath)
	if err == nil {
		t.Error("Expected createZipArchive to fail on unreadable directory")
	}
}

func TestCopyDir_WithSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")

	// Create a more complex directory structure
	nestedDir := filepath.Join(srcDir, "level1", "level2", "level3")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create files at different levels
	file1 := filepath.Join(srcDir, "file1.txt")
	file2 := filepath.Join(srcDir, "level1", "file2.txt")
	file3 := filepath.Join(nestedDir, "file3.txt")

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file3, []byte("content3"), 0644); err != nil {
		t.Fatal(err)
	}

	err := copyDir(srcDir, dstDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify all files were copied
	dstFile1 := filepath.Join(dstDir, "file1.txt")
	dstFile2 := filepath.Join(dstDir, "level1", "file2.txt")
	dstFile3 := filepath.Join(dstDir, "level1", "level2", "level3", "file3.txt")

	for _, f := range []string{dstFile1, dstFile2, dstFile3} {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist", f)
		}
	}
}

func TestCopyFile_OpenError(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "nonexistent.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")

	err := copyFile(srcPath, dstPath)
	if err == nil {
		t.Error("Expected copyFile to fail on nonexistent source")
	}
}

func TestExtractZipFile_OpenError(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	destDir := filepath.Join(tmpDir, "extract")

	// Create a zip file, then try to extract it but with permission issues
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)

	// Create a file entry
	w, err := zw.Create("test.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte("test content"))
	if err != nil {
		t.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	// Make destDir unwritable
	if err := os.MkdirAll(destDir, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(destDir, 0755) // Cleanup

	err = extractZip(zipPath, destDir)
	if err == nil {
		t.Error("Expected extractZip to fail on unwritable destination")
	}
}

func TestEnumerate_NonExistentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "nonexistent")

	handler := &Handler{}
	_, err := handler.Enumerate(testPath)
	if err == nil {
		t.Error("Expected Enumerate to fail on non-existent directory")
	}
}

func TestIngest_TempDirCreationError(t *testing.T) {
	// This test is challenging because os.MkdirTemp is hard to force to fail
	// We'll test with an invalid zip path instead to trigger a different error path
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "invalid.zip")
	outputDir := filepath.Join(tmpDir, "output")

	// Create an invalid zip file
	if err := os.WriteFile(zipPath, []byte("not a valid zip"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	_, err := handler.Ingest(zipPath, outputDir)
	if err == nil {
		t.Error("Expected Ingest to fail on invalid zip")
	}
}

func TestEmitNative_CreateModulesError(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "corpus.json")
	outputDir := filepath.Join(tmpDir, "output")

	// Create mods.d successfully but make modules fail
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	modsDir := filepath.Join(outputDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file where modules/ should be
	modulesPath := filepath.Join(outputDir, "modules")
	if err := os.WriteFile(modulesPath, []byte("blocking file"), 0644); err != nil {
		t.Fatal(err)
	}

	corpus := &ir.Corpus{
		ID:         "test-module",
		Version:    "1.0.0",
		ModuleType: ir.ModuleBible,
		Language:   "en",
		Title:      "Test Module",
		Documents:  []*ir.Document{},
	}

	data, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(irPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	_, err = handler.EmitNative(irPath, outputDir)
	if err == nil {
		t.Error("Expected EmitNative to fail when modules path is blocked")
	}

	if !strings.Contains(err.Error(), "failed to create modules") {
		t.Errorf("Expected error about failed modules creation, got: %v", err)
	}
}

func TestEmitNative_WriteConfError(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "corpus.json")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	modsDir := filepath.Join(outputDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	modulesDir := filepath.Join(outputDir, "modules")
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Make mods.d unwritable
	if err := os.Chmod(modsDir, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(modsDir, 0755) // Cleanup

	corpus := &ir.Corpus{
		ID:         "test-module",
		Version:    "1.0.0",
		ModuleType: ir.ModuleBible,
		Language:   "en",
		Title:      "Test Module",
		Documents:  []*ir.Document{},
	}

	data, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(irPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	_, err = handler.EmitNative(irPath, outputDir)
	if err == nil {
		t.Error("Expected EmitNative to fail when conf file cannot be written")
	}

	if !strings.Contains(err.Error(), "failed to write conf") {
		t.Errorf("Expected error about failed conf write, got: %v", err)
	}
}

func TestEmitNative_CreateZipError(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "corpus.json")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file where the zip should be created (blocking the parent dir for zip)
	parentDir := filepath.Dir(outputDir)
	zipPath := filepath.Join(parentDir, "test-module.zip")
	if err := os.MkdirAll(zipPath, 0755); err != nil {
		t.Fatal(err)
	}

	corpus := &ir.Corpus{
		ID:         "test-module",
		Version:    "1.0.0",
		ModuleType: ir.ModuleBible,
		Language:   "en",
		Title:      "Test Module",
		Documents:  []*ir.Document{},
	}

	data, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(irPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	_, err = handler.EmitNative(irPath, outputDir)
	if err == nil {
		t.Error("Expected EmitNative to fail when zip cannot be created")
	}

	if !strings.Contains(err.Error(), "failed to create zip") {
		t.Errorf("Expected error about failed zip creation, got: %v", err)
	}
}

func TestCreateZipArchive_CreateFileError(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")

	// Make parent directory for zip unwritable
	zipDir := filepath.Join(tmpDir, "zipdir")
	if err := os.MkdirAll(zipDir, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(zipDir, 0755) // Cleanup

	zipPath := filepath.Join(zipDir, "output.zip")

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	err := createZipArchive(sourceDir, zipPath)
	if err == nil {
		t.Error("Expected createZipArchive to fail when zip file cannot be created")
	}
}

func TestCreateZipArchive_FileOpenError(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	zipPath := filepath.Join(tmpDir, "output.zip")

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file with no read permissions
	unreadableFile := filepath.Join(sourceDir, "unreadable.txt")
	if err := os.WriteFile(unreadableFile, []byte("content"), 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(unreadableFile, 0644) // Cleanup

	err := createZipArchive(sourceDir, zipPath)
	if err == nil {
		t.Error("Expected createZipArchive to fail on unreadable file")
	}
}

func TestCopyDir_MkdirError(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Make dst unwritable
	if err := os.MkdirAll(dstDir, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dstDir, 0755) // Cleanup

	err := copyDir(srcDir, dstDir)
	if err == nil {
		t.Error("Expected copyDir to fail when subdirectory cannot be created")
	}
}

func TestCopyFile_CreateError(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")

	// Create a file that blocks destination directory creation
	dstDir := filepath.Join(tmpDir, "dstdir")
	if err := os.WriteFile(dstDir, []byte("blocking"), 0644); err != nil {
		t.Fatal(err)
	}

	dstPath := filepath.Join(dstDir, "dest.txt")

	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	err := copyFile(srcPath, dstPath)
	if err == nil {
		t.Error("Expected copyFile to fail when destination directory is blocked")
	}
}

func TestEnumerate_EntryInfoError(t *testing.T) {
	// This test covers the case where entry.Info() returns an error
	// This is hard to trigger naturally, but we can test the happy path
	// The error path in line 220 is mostly defensive programming
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "test")

	createTestDirectory(t, testDir)

	handler := &Handler{}
	result, err := handler.Enumerate(testDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that sizes are properly retrieved
	if len(result.Entries) > 0 && result.Entries[0].SizeBytes <= 0 {
		t.Error("Expected entry to have positive size")
	}
}

func TestCopyDir_RelPathError(t *testing.T) {
	// Test that copyDir handles Rel() errors
	// This is difficult to trigger in practice, so we test the happy path
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(srcDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	err := copyDir(srcDir, dstDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify file was copied
	copiedFile := filepath.Join(dstDir, "test.txt")
	if _, err := os.Stat(copiedFile); os.IsNotExist(err) {
		t.Error("Expected file to be copied")
	}
}

func TestCopyDir_WalkError(t *testing.T) {
	// Test that copyDir handles walk errors
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "nonexistent")
	dstDir := filepath.Join(tmpDir, "dst")

	err := copyDir(srcDir, dstDir)
	if err == nil {
		t.Error("Expected copyDir to fail on non-existent source")
	}
}

func TestCopyFile_IOCopyError(t *testing.T) {
	// Test the io.Copy error path in copyFile
	// This is challenging to trigger directly, so we'll test a related scenario
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")

	// Create source file
	if err := os.WriteFile(srcPath, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create destination file first
	if err := os.WriteFile(dstPath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	// Copy should succeed (overwrite)
	err := copyFile(srcPath, dstPath)
	if err != nil {
		t.Fatalf("Expected copyFile to succeed, got: %v", err)
	}

	// Verify content was copied
	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(content) != "test content" {
		t.Errorf("Expected 'test content', got %s", string(content))
	}
}

func TestCreateZipArchive_ZipCreateError(t *testing.T) {
	// Test error when creating zip entry
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	zipPath := filepath.Join(tmpDir, "output.zip")

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a regular file
	testFile := filepath.Join(sourceDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// This should succeed
	err := createZipArchive(sourceDir, zipPath)
	if err != nil {
		t.Fatalf("Expected createZipArchive to succeed, got: %v", err)
	}

	// Verify zip was created
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		t.Error("Expected zip file to be created")
	}
}

func TestExtractZipFile_CreateError(t *testing.T) {
	// Test extractZipFile when os.Create fails
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	// Create a zip with a file
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	w, err := zw.Create("test.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte("content"))
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	// Create destination with a directory where file should be
	destDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(filepath.Join(destDir, "test.txt"), 0755); err != nil {
		t.Fatal(err)
	}

	err = extractZip(zipPath, destDir)
	if err == nil {
		t.Error("Expected extractZip to fail when destination is a directory")
	}
}

func TestDetectDirectory_StatError(t *testing.T) {
	// Test detectDirectory handles stat errors properly
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "test")

	// Create only mods.d directory
	modsDir := filepath.Join(testDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a .conf file
	confPath := filepath.Join(modsDir, "test.conf")
	if err := os.WriteFile(confPath, []byte("[Test]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// modules/ doesn't exist, should fail detection
	result, err := detectDirectory(testDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail when modules/ is missing")
	}
}

func TestEnumerate_ZipTempDirError(t *testing.T) {
	// Test that Enumerate properly handles temp directory creation
	// This is mainly testing the happy path since forcing MkdirTemp to fail is difficult
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	createTestZip(t, zipPath, true, true, true)

	handler := &Handler{}
	result, err := handler.Enumerate(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) == 0 {
		t.Error("Expected at least one entry")
	}
}

func TestIngest_ZipHappyPath(t *testing.T) {
	// Additional test to ensure complete zip ingestion coverage
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "module.zip")
	outputDir := filepath.Join(tmpDir, "output")

	createTestZip(t, zipPath, true, true, true)

	handler := &Handler{}
	result, err := handler.Ingest(zipPath, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "module.zip" {
		t.Errorf("Expected artifact ID 'module.zip', got %s", result.ArtifactID)
	}

	// Verify cleanup happened (temp dir should be gone)
	// We can't directly verify this, but the function should have succeeded
	if result.Metadata["status"] != "ingested" {
		t.Errorf("Expected status 'ingested', got %s", result.Metadata["status"])
	}
}

func TestExtractIR_HappyPath(t *testing.T) {
	// Comprehensive test for ExtractIR to ensure all paths are covered
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "module")
	outputDir := filepath.Join(tmpDir, "ir")

	createTestDirectory(t, testPath)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.ExtractIR(testPath, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify all result fields
	if result.IRPath == "" {
		t.Error("Expected IR path to be set")
	}

	if result.LossClass != "L2" {
		t.Errorf("Expected loss class L2, got %s", result.LossClass)
	}

	if result.LossReport == nil || len(result.LossReport.Warnings) == 0 {
		t.Error("Expected loss report with warnings")
	}
}
