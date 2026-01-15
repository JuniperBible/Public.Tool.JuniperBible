package sword

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.sword" {
		t.Errorf("Expected PluginID 'format.sword', got %s", m.PluginID)
	}
	if m.Kind != "format" {
		t.Errorf("Expected Kind 'format', got %s", m.Kind)
	}
}

func TestRegister(t *testing.T) {
	Register()
}

func TestDetect_SWORDFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	confFile := filepath.Join(tmpDir, "kjv.conf")
	if err := os.WriteFile(confFile, []byte("[KJV]\nDataPath=./modules/texts/kjv/"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(confFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected SWORD conf file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "sword" {
		t.Errorf("Expected format 'sword', got %s", result.Format)
	}
}

func TestDetect_NonSWORDFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	txtFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(txtFile, []byte("not SWORD"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(txtFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-SWORD file to not be detected")
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

func TestDetect_NonExistent(t *testing.T) {
	h := &Handler{}

	result, err := h.Detect("/nonexistent/file.conf")
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-existent file to not be detected")
	}
}

func TestIngest(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	confFile := filepath.Join(tmpDir, "kjv.conf")
	content := []byte("[KJV]\nDataPath=./modules/texts/kjv/")
	if err := os.WriteFile(confFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	result, err := h.Ingest(confFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "kjv" {
		t.Errorf("Expected artifact ID 'kjv', got %s", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected blob hash to be set")
	}
	if result.Metadata["format"] != "sword" {
		t.Errorf("Expected format 'sword', got %s", result.Metadata["format"])
	}

	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

func TestIngest_NonExistent(t *testing.T) {
	h := &Handler{}

	_, err := h.Ingest("/nonexistent/file.conf", t.TempDir())
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestEnumerate(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	confFile := filepath.Join(tmpDir, "kjv.conf")
	content := []byte("[KJV]\nDataPath=./modules/texts/kjv/")
	if err := os.WriteFile(confFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Enumerate(confFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}
	if result.Entries[0].Path != "kjv.conf" {
		t.Errorf("Expected path 'kjv.conf', got %s", result.Entries[0].Path)
	}
}

func TestEnumerate_NonExistent(t *testing.T) {
	h := &Handler{}

	_, err := h.Enumerate("/nonexistent/file.conf")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestExtractIR(t *testing.T) {
	h := &Handler{}

	_, err := h.ExtractIR("kjv.conf", "/output")
	if err == nil {
		t.Error("Expected error for unsupported ExtractIR")
	}
}

func TestEmitNative(t *testing.T) {
	h := &Handler{}

	_, err := h.EmitNative("ir.json", "/output")
	if err == nil {
		t.Error("Expected error for unsupported EmitNative")
	}
}
