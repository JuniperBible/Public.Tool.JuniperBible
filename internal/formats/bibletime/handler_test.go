package bibletime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func TestDetect_XBELFile_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xbel")
	content := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE xbel>
<xbel version="1.0">
  <bookmark href="bibletime://module/KJV/Gen.1.1">
    <title>In the beginning</title>
  </bookmark>
</xbel>`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed, got: %s", result.Reason)
	}
	if result.Format != "BIBLETIME" {
		t.Errorf("Expected format BIBLETIME, got %s", result.Format)
	}
	if !strings.Contains(result.Reason, "bookmark") {
		t.Errorf("Expected reason to mention bookmark, got: %s", result.Reason)
	}
}

func TestDetect_XBELFile_NoContent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xbel")
	content := `<?xml version="1.0" encoding="UTF-8"?>`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for XBEL without bibletime content")
	}
}

func TestDetect_SWORDModuleDirectory_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	confFile := filepath.Join(modsDir, "kjv.conf")
	confContent := `[KJV]
Description=King James Version
ModulePath=./modules/texts/ztext/kjv/
`
	if err := os.WriteFile(confFile, []byte(confContent), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Detect(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed, got: %s", result.Reason)
	}
	if result.Format != "BIBLETIME" {
		t.Errorf("Expected format BIBLETIME, got %s", result.Format)
	}
	if !strings.Contains(result.Reason, "SWORD") {
		t.Errorf("Expected reason to mention SWORD, got: %s", result.Reason)
	}
}

func TestDetect_DirectoryNoModsD(t *testing.T) {
	tmpDir := t.TempDir()

	h := &Handler{}
	result, err := h.Detect(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for directory without mods.d")
	}
}

func TestDetect_NonExistentFile(t *testing.T) {
	h := &Handler{}
	result, err := h.Detect("/nonexistent/path")
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for non-existent file")
	}
	if !strings.Contains(result.Reason, "stat") {
		t.Errorf("Expected reason to mention stat error, got: %s", result.Reason)
	}
}

func TestIngest_File(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xbel")
	content := []byte(`<?xml version="1.0"?><xbel><bookmark href="bibletime://test"/></xbel>`)
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "test" {
		t.Errorf("Expected artifact ID 'test', got %s", result.ArtifactID)
	}
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}
	if result.Metadata["format"] != "BIBLETIME" {
		t.Errorf("Expected format BIBLETIME, got %s", result.Metadata["format"])
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected blob hash to be set")
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}

	// Verify blob content matches
	blobData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(blobData) != string(content) {
		t.Error("Blob content does not match original")
	}
}

func TestIngest_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file in the root directory so readFileOrDir finds it
	testFile := filepath.Join(tmpDir, "test.conf")
	confContent := []byte("[KJV]\nDescription=King James Version\n")
	if err := os.WriteFile(testFile, confContent, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	h := &Handler{}
	result, err := h.Ingest(tmpDir, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID == "" {
		t.Error("Expected artifact ID to be set")
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected blob hash to be set")
	}
	if result.SizeBytes != int64(len(confContent)) {
		t.Errorf("Expected size %d, got %d", len(confContent), result.SizeBytes)
	}
}

func TestIngest_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	emptyDir := filepath.Join(tmpDir, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	h := &Handler{}
	result, err := h.Ingest(emptyDir, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.SizeBytes != 0 {
		t.Errorf("Expected size 0 for empty directory, got %d", result.SizeBytes)
	}
}

func TestEnumerate_File(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xbel")
	content := []byte(`<?xml version="1.0"?><xbel/>`)
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Enumerate(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.xbel" {
		t.Errorf("Expected path 'test.xbel', got %s", entry.Path)
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
	if entry.IsDir {
		t.Error("Expected IsDir to be false")
	}
}

func TestEnumerate_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create multiple .conf files
	for _, name := range []string{"kjv.conf", "esv.conf", "nasb.conf"} {
		confFile := filepath.Join(modsDir, name)
		if err := os.WriteFile(confFile, []byte("[MODULE]"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a non-.conf file (should be ignored)
	otherFile := filepath.Join(modsDir, "readme.txt")
	if err := os.WriteFile(otherFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Enumerate(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(result.Entries))
	}

	// Verify all entries are .conf files
	for _, entry := range result.Entries {
		if !strings.HasSuffix(entry.Path, ".conf") {
			t.Errorf("Expected .conf file, got %s", entry.Path)
		}
		if entry.IsDir {
			t.Errorf("Expected file entry, got directory for %s", entry.Path)
		}
	}
}

func TestEnumerate_NonExistent(t *testing.T) {
	h := &Handler{}
	_, err := h.Enumerate("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}

func TestExtractIR_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xbel")
	content := `<?xml version="1.0"?><xbel><bookmark href="bibletime://test"/></xbel>`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.ExtractIR(testFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.IRPath == "" {
		t.Error("Expected IR path to be set")
	}
	if result.LossClass != "L1" {
		t.Errorf("Expected loss class L1, got %s", result.LossClass)
	}
	if result.LossReport == nil {
		t.Fatal("Expected loss report to be set")
	}
	if result.LossReport.SourceFormat != "BIBLETIME" {
		t.Errorf("Expected source format BIBLETIME, got %s", result.LossReport.SourceFormat)
	}
	if result.LossReport.TargetFormat != "IR" {
		t.Errorf("Expected target format IR, got %s", result.LossReport.TargetFormat)
	}

	// Verify IR file was created
	if _, err := os.Stat(result.IRPath); os.IsNotExist(err) {
		t.Error("Expected IR file to exist")
	}

	// Verify IR content
	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatal(err)
	}

	var corpus map[string]interface{}
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("Failed to parse IR JSON: %v", err)
	}

	// Check required fields
	if corpus["id"] == nil {
		t.Error("Expected ID field in corpus")
	}
	if corpus["version"] == nil {
		t.Error("Expected version field in corpus")
	}
	if corpus["module_type"] == nil {
		t.Error("Expected module_type field in corpus")
	}
	if corpus["title"] == nil {
		t.Error("Expected title field in corpus")
	}
	if corpus["source_format"] != "BIBLETIME" {
		t.Errorf("Expected source_format BIBLETIME, got %v", corpus["source_format"])
	}
	if corpus["loss_class"] != "L1" {
		t.Errorf("Expected loss_class L1, got %v", corpus["loss_class"])
	}

	// Verify attributes contain source path
	if attrs, ok := corpus["attributes"].(map[string]interface{}); ok {
		if sourcePath, ok := attrs["source_path"].(string); !ok || sourcePath != testFile {
			t.Errorf("Expected source_path to be %s, got %v", testFile, attrs["source_path"])
		}
	} else {
		t.Error("Expected attributes to be a map")
	}
}

func TestExtractIR_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.ExtractIR(tmpDir, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.IRPath == "" {
		t.Error("Expected IR path to be set")
	}
	if result.LossClass != "L1" {
		t.Errorf("Expected loss class L1, got %s", result.LossClass)
	}
}

func TestEmitNative_ValidIR(t *testing.T) {
	tmpDir := t.TempDir()

	// Create IR corpus
	corpus := map[string]interface{}{
		"id":            "test-module",
		"version":       "1.0",
		"module_type":   "Bible",
		"title":         "Test Bible",
		"source_format": "BIBLETIME",
		"loss_class":    "L1",
		"documents":     []interface{}{},
		"attributes": map[string]string{
			"language": "en",
		},
	}

	irPath := filepath.Join(tmpDir, "corpus.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.EmitNative(irPath, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.OutputPath == "" {
		t.Error("Expected output path to be set")
	}
	if result.Format != "BIBLETIME" {
		t.Errorf("Expected format BIBLETIME, got %s", result.Format)
	}
	if result.LossClass != "L1" {
		t.Errorf("Expected loss class L1, got %s", result.LossClass)
	}
	if result.LossReport == nil {
		t.Fatal("Expected loss report to be set")
	}

	// Verify output directory structure
	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Error("Expected output directory to exist")
	}

	modsDir := filepath.Join(result.OutputPath, "mods.d")
	if _, err := os.Stat(modsDir); os.IsNotExist(err) {
		t.Error("Expected mods.d directory to exist")
	}

	confFile := filepath.Join(modsDir, "module.conf")
	if _, err := os.Stat(confFile); os.IsNotExist(err) {
		t.Error("Expected module.conf to exist")
	}

	// Verify .conf file content
	confData, err := os.ReadFile(confFile)
	if err != nil {
		t.Fatal(err)
	}
	confContent := string(confData)
	if !strings.Contains(confContent, "[test-module]") {
		t.Error("Expected .conf to contain module ID section")
	}
	if !strings.Contains(confContent, "Description=Test Bible") {
		t.Error("Expected .conf to contain module title")
	}
	if !strings.Contains(confContent, "ModulePath=") {
		t.Error("Expected .conf to contain ModulePath")
	}
}

func TestEmitNative_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial XBEL file
	testFile := filepath.Join(tmpDir, "test.xbel")
	content := `<?xml version="1.0"?><xbel><bookmark href="bibletime://test"/></xbel>`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Extract to IR
	irOutputDir := filepath.Join(tmpDir, "ir")
	if err := os.MkdirAll(irOutputDir, 0755); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	irResult, err := h.ExtractIR(testFile, irOutputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	// Emit back to native
	nativeOutputDir := filepath.Join(tmpDir, "native")
	if err := os.MkdirAll(nativeOutputDir, 0755); err != nil {
		t.Fatal(err)
	}

	nativeResult, err := h.EmitNative(irResult.IRPath, nativeOutputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	// Verify the output exists
	if _, err := os.Stat(nativeResult.OutputPath); os.IsNotExist(err) {
		t.Error("Expected round-trip output to exist")
	}

	// Verify SWORD structure was created
	modsDir := filepath.Join(nativeResult.OutputPath, "mods.d")
	if _, err := os.Stat(modsDir); os.IsNotExist(err) {
		t.Error("Expected mods.d directory after round-trip")
	}
}

func TestEmitNative_InvalidIR(t *testing.T) {
	tmpDir := t.TempDir()

	// Create invalid IR file
	irPath := filepath.Join(tmpDir, "corpus.json")
	if err := os.WriteFile(irPath, []byte("invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	_, err := h.EmitNative(irPath, outputDir)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("Expected unmarshal error, got: %v", err)
	}
}

func TestEmitNative_MissingIR(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	_, err := h.EmitNative("/nonexistent/corpus.json", outputDir)
	if err == nil {
		t.Error("Expected error for missing IR file")
	}
}

func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest == nil {
		t.Fatal("Expected manifest to be non-nil")
	}
	if manifest.PluginID != "format.bibletime" {
		t.Errorf("Expected plugin ID 'format.bibletime', got %s", manifest.PluginID)
	}
	if manifest.Kind != "format" {
		t.Errorf("Expected kind 'format', got %s", manifest.Kind)
	}
	if manifest.Version == "" {
		t.Error("Expected version to be set")
	}
}

func TestRegister(t *testing.T) {
	// Clear registry before test
	plugins.ClearEmbeddedRegistry()

	// Register should be called in init, but call it explicitly for test
	Register()

	if !plugins.HasEmbeddedPlugin("format.bibletime") {
		t.Error("Expected bibletime plugin to be registered")
	}

	plugin := plugins.GetEmbeddedPlugin("format.bibletime")
	if plugin == nil {
		t.Fatal("Expected to get bibletime plugin")
	}
	if plugin.Format == nil {
		t.Error("Expected Format handler to be set")
	}
}

func TestReadFileOrDir_File(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	data, err := readFileOrDir(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != string(content) {
		t.Errorf("Expected content %s, got %s", content, data)
	}
}

func TestReadFileOrDir_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	data, err := readFileOrDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Should read first file
	if string(data) != string(content) {
		t.Errorf("Expected content %s, got %s", content, data)
	}
}

func TestReadFileOrDir_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	data, err := readFileOrDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != 0 {
		t.Errorf("Expected empty data for empty directory, got %d bytes", len(data))
	}
}

func TestReadFileOrDir_DirectoryOnlySubdirs(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	data, err := readFileOrDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != 0 {
		t.Errorf("Expected empty data for directory with only subdirs, got %d bytes", len(data))
	}
}
