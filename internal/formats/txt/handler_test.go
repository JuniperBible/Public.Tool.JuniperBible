package txt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.txt" {
		t.Errorf("Expected PluginID 'format.txt', got %s", m.PluginID)
	}
	if m.Kind != "format" {
		t.Errorf("Expected Kind 'format', got %s", m.Kind)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", m.Version)
	}
}

func TestRegister(t *testing.T) {
	// Register should not panic
	Register()
}

func TestDetect_TxtFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	file := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(file)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "txt" {
		t.Errorf("Expected format 'txt', got %s", result.Format)
	}
}

func TestDetect_TextFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	file := filepath.Join(tmpDir, "test.text")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(file)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected .text file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "txt" {
		t.Errorf("Expected format 'txt', got %s", result.Format)
	}
}

func TestDetect_NonTxtFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	file := filepath.Join(tmpDir, "test.xml")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(file)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-txt file to not be detected")
	}
}

func TestDetect_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	result, err := h.Detect(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected directory to not be detected")
	}
	if !strings.Contains(result.Reason, "directory") {
		t.Errorf("Expected reason to mention directory, got: %s", result.Reason)
	}
}

func TestDetect_NonExistentFile(t *testing.T) {
	h := &Handler{}

	result, err := h.Detect("/nonexistent/file.txt")
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-existent file to not be detected")
	}
	if !strings.Contains(result.Reason, "cannot stat") {
		t.Errorf("Expected reason to mention stat error, got: %s", result.Reason)
	}
}

func TestIngest_TxtFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	file := filepath.Join(tmpDir, "test.txt")
	content := []byte("test text content")
	if err := os.WriteFile(file, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	result, err := h.Ingest(file, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "test" {
		t.Errorf("Expected artifact ID 'test', got %s", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected blob hash to be set")
	}
	if result.Metadata["format"] != "txt" {
		t.Errorf("Expected format 'txt', got %s", result.Metadata["format"])
	}
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

func TestIngest_NonExistentFile(t *testing.T) {
	h := &Handler{}
	outputDir := t.TempDir()

	_, err := h.Ingest("/nonexistent/file.txt", outputDir)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected 'failed to read file' error, got: %v", err)
	}
}

func TestEnumerate_TxtFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	file := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(file, content, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Enumerate(file)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.txt" {
		t.Errorf("Expected path 'test.txt', got %s", entry.Path)
	}
	if entry.IsDir {
		t.Error("Expected entry to not be a directory")
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
}

func TestEnumerate_NonExistentFile(t *testing.T) {
	h := &Handler{}

	_, err := h.Enumerate("/nonexistent/file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to stat") {
		t.Errorf("Expected 'failed to stat' error, got: %v", err)
	}
}

func TestExtractIR_TxtFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	file := filepath.Join(tmpDir, "test.txt")
	content := []byte("Gen 1:1 In the beginning")
	if err := os.WriteFile(file, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := h.ExtractIR(file, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.LossClass != "L3" {
		t.Errorf("Expected loss class 'L3', got %s", result.LossClass)
	}
	if result.IRPath == "" {
		t.Error("Expected IR path to be set")
	}

	// Verify IR file was written
	if _, err := os.Stat(result.IRPath); os.IsNotExist(err) {
		t.Error("Expected IR file to exist")
	}
}

func TestExtractIR_NonExistentFile(t *testing.T) {
	h := &Handler{}
	outputDir := t.TempDir()

	_, err := h.ExtractIR("/nonexistent/file.txt", outputDir)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected 'failed to read file' error, got: %v", err)
	}
}

func TestEmitNative_FromIR(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	// Create a simple IR file with raw text for round-trip
	irContent := `{
		"id": "test",
		"version": "1.0.0",
		"module_type": "bible",
		"attributes": {
			"_txt_raw": "Gen 1:1 In the beginning"
		},
		"documents": []
	}`
	irFile := filepath.Join(tmpDir, "test.ir.json")
	if err := os.WriteFile(irFile, []byte(irContent), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := h.EmitNative(irFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Format != "TXT" {
		t.Errorf("Expected format 'TXT', got %s", result.Format)
	}
	if result.LossClass != "L0" {
		t.Errorf("Expected loss class 'L0' for round-trip, got %s", result.LossClass)
	}
	if result.OutputPath == "" {
		t.Error("Expected output path to be set")
	}

	// Verify output file was written
	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Error("Expected output file to exist")
	}
}

func TestEmitNative_NonExistentIRFile(t *testing.T) {
	h := &Handler{}
	outputDir := t.TempDir()

	_, err := h.EmitNative("/nonexistent/ir.json", outputDir)
	if err == nil {
		t.Error("Expected error for non-existent IR file")
	}
	if !strings.Contains(err.Error(), "failed to read IR file") {
		t.Errorf("Expected 'failed to read IR file' error, got: %v", err)
	}
}

func TestEmitNative_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	irFile := filepath.Join(tmpDir, "invalid.ir.json")
	if err := os.WriteFile(irFile, []byte("invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := h.EmitNative(irFile, outputDir)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse IR") {
		t.Errorf("Expected 'failed to parse IR' error, got: %v", err)
	}
}

func TestEmitNative_GenerateFromIR(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	// Create an IR file with proper document structure (no _txt_raw)
	irContent := `{
		"id": "gen-test",
		"version": "1.0.0",
		"module_type": "bible",
		"attributes": {},
		"documents": [
			{
				"id": "Gen",
				"title": "Genesis",
				"content_blocks": [
					{
						"id": "cb1",
						"text": "In the beginning God created the heavens and the earth.",
						"anchors": [
							{
								"id": "a1",
								"spans": [
									{
										"type": "VERSE",
										"ref": {
											"book": "Gen",
											"chapter": 1,
											"verse": 1
										}
									}
								]
							}
						]
					},
					{
						"id": "cb2",
						"text": "And the earth was without form, and void.",
						"anchors": [
							{
								"id": "a2",
								"spans": [
									{
										"type": "VERSE",
										"ref": {
											"book": "Gen",
											"chapter": 1,
											"verse": 2
										}
									}
								]
							}
						]
					}
				]
			}
		]
	}`
	irFile := filepath.Join(tmpDir, "gen-test.ir.json")
	if err := os.WriteFile(irFile, []byte(irContent), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := h.EmitNative(irFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Format != "TXT" {
		t.Errorf("Expected format 'TXT', got %s", result.Format)
	}
	if result.LossClass != "L3" {
		t.Errorf("Expected loss class 'L3' for generated output, got %s", result.LossClass)
	}

	// Verify output file was written
	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Error("Expected output file to exist")
	}

	// Verify content
	content, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Gen 1:1") {
		t.Errorf("Expected content to contain 'Gen 1:1', got: %s", string(content))
	}
}
