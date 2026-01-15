package sqlite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.sqlite" {
		t.Errorf("Expected PluginID 'format.sqlite', got %s", m.PluginID)
	}
	if m.Kind != "format" {
		t.Errorf("Expected Kind 'format', got %s", m.Kind)
	}
}

func TestRegister(t *testing.T) {
	Register()
}

func TestDetect_SQLiteFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	sqliteFile := filepath.Join(tmpDir, "bible.sqlite")
	// SQLite files start with "SQLite format 3"
	if err := os.WriteFile(sqliteFile, []byte("SQLite format 3\x00data"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(sqliteFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected SQLite file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "sqlite" {
		t.Errorf("Expected format 'sqlite', got %s", result.Format)
	}
}

func TestDetect_NonSQLiteFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	txtFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(txtFile, []byte("not SQLite"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(txtFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-SQLite file to not be detected")
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

	result, err := h.Detect("/nonexistent/file.sqlite")
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

	sqliteFile := filepath.Join(tmpDir, "bible.sqlite")
	content := []byte("SQLite format 3\x00data")
	if err := os.WriteFile(sqliteFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	result, err := h.Ingest(sqliteFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "bible" {
		t.Errorf("Expected artifact ID 'bible', got %s", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected blob hash to be set")
	}
	if result.Metadata["format"] != "sqlite" {
		t.Errorf("Expected format 'sqlite', got %s", result.Metadata["format"])
	}

	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

func TestIngest_NonExistent(t *testing.T) {
	h := &Handler{}

	_, err := h.Ingest("/nonexistent/file.sqlite", t.TempDir())
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestEnumerate(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	sqliteFile := filepath.Join(tmpDir, "bible.sqlite")
	content := []byte("SQLite format 3\x00data")
	if err := os.WriteFile(sqliteFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Enumerate(sqliteFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}
	if result.Entries[0].Path != "bible.sqlite" {
		t.Errorf("Expected path 'bible.sqlite', got %s", result.Entries[0].Path)
	}
}

func TestEnumerate_NonExistent(t *testing.T) {
	h := &Handler{}

	_, err := h.Enumerate("/nonexistent/file.sqlite")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestExtractIR(t *testing.T) {
	h := &Handler{}

	_, err := h.ExtractIR("bible.sqlite", "/output")
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
