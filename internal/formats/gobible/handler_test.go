package gobible

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func TestDetect_ValidJarFile(t *testing.T) {
	// Create a temporary .jar file
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	if err := os.WriteFile(jarPath, []byte("fake jar content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	h := &Handler{}
	result, err := h.Detect(jarPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Detected {
		t.Errorf("expected detected=true, got false; reason: %s", result.Reason)
	}
	if result.Format != "gobible" {
		t.Errorf("expected format=gobible, got %s", result.Format)
	}
	if result.Reason != "GoBible file detected" {
		t.Errorf("expected reason='GoBible file detected', got %s", result.Reason)
	}
}

func TestDetect_InvalidExtension(t *testing.T) {
	// Create a temporary file with non-.jar extension
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtPath, []byte("not a jar"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	h := &Handler{}
	result, err := h.Detect(txtPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Detected {
		t.Errorf("expected detected=false, got true")
	}
	if result.Reason != "not a .jar file" {
		t.Errorf("expected reason='not a .jar file', got %s", result.Reason)
	}
}

func TestDetect_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	h := &Handler{}
	result, err := h.Detect(tmpDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Detected {
		t.Errorf("expected detected=false, got true")
	}
	if result.Reason != "path is a directory" {
		t.Errorf("expected reason='path is a directory', got %s", result.Reason)
	}
}

func TestDetect_NonExistentFile(t *testing.T) {
	h := &Handler{}
	result, err := h.Detect("/nonexistent/path/file.jar")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Detected {
		t.Errorf("expected detected=false, got true")
	}
	if result.Reason == "" {
		t.Errorf("expected non-empty reason for stat failure")
	}
}

func TestDetect_CaseInsensitiveExtension(t *testing.T) {
	// Test that .JAR (uppercase) is also detected
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.JAR")
	if err := os.WriteFile(jarPath, []byte("fake jar content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	h := &Handler{}
	result, err := h.Detect(jarPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Detected {
		t.Errorf("expected detected=true for .JAR extension, got false; reason: %s", result.Reason)
	}
}

func TestIngest_Success(t *testing.T) {
	// Create a temporary .jar file
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test-bible.jar")
	content := []byte("test jar content for ingestion")
	if err := os.WriteFile(jarPath, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	outputDir := t.TempDir()

	h := &Handler{}
	result, err := h.Ingest(jarPath, outputDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify artifact ID
	if result.ArtifactID != "test-bible" {
		t.Errorf("expected artifactID=test-bible, got %s", result.ArtifactID)
	}

	// Verify SHA256 hash
	hash := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(hash[:])
	if result.BlobSHA256 != expectedHash {
		t.Errorf("expected hash=%s, got %s", expectedHash, result.BlobSHA256)
	}

	// Verify size
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("expected size=%d, got %d", len(content), result.SizeBytes)
	}

	// Verify metadata
	if result.Metadata["format"] != "gobible" {
		t.Errorf("expected format=gobible in metadata, got %s", result.Metadata["format"])
	}

	// Verify blob file was created
	blobPath := filepath.Join(outputDir, expectedHash[:2], expectedHash)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Errorf("expected blob file at %s, but it does not exist", blobPath)
	}

	// Verify blob content
	blobContent, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("failed to read blob file: %v", err)
	}
	if string(blobContent) != string(content) {
		t.Errorf("blob content mismatch")
	}
}

func TestIngest_FileNotFound(t *testing.T) {
	h := &Handler{}
	_, err := h.Ingest("/nonexistent/file.jar", t.TempDir())

	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestIngest_InvalidOutputDir(t *testing.T) {
	// Create a temporary .jar file
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	if err := os.WriteFile(jarPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Use a path that cannot be created (file as directory)
	invalidDir := filepath.Join(tmpDir, "file-not-dir")
	if err := os.WriteFile(invalidDir, []byte("this is a file"), 0644); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	h := &Handler{}
	_, err := h.Ingest(jarPath, filepath.Join(invalidDir, "subdir"))

	if err == nil {
		t.Fatal("expected error for invalid output directory, got nil")
	}
}

func TestIngest_WriteFileFails(t *testing.T) {
	// Create a temporary .jar file
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	content := []byte("test content")
	if err := os.WriteFile(jarPath, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create output directory
	outputDir := t.TempDir()

	// Calculate the hash to know where the blob will be written
	hash := sha256.Sum256(content)
	hashHex := hex.EncodeToString(hash[:])

	// Create the blob directory
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		t.Fatalf("failed to create blob dir: %v", err)
	}

	// Create a file where the blob should be written, but make it read-only
	// by creating a directory instead of a file
	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.Mkdir(blobPath, 0755); err != nil {
		t.Fatalf("failed to create blocking directory: %v", err)
	}

	h := &Handler{}
	_, err := h.Ingest(jarPath, outputDir)

	if err == nil {
		t.Fatal("expected error when blob write fails, got nil")
	}
}

func TestEnumerate_ValidFile(t *testing.T) {
	// Create a temporary .jar file
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	content := []byte("test content")
	if err := os.WriteFile(jarPath, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	h := &Handler{}
	result, err := h.Enumerate(jarPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.jar" {
		t.Errorf("expected path=test.jar, got %s", entry.Path)
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("expected size=%d, got %d", len(content), entry.SizeBytes)
	}
	if entry.IsDir {
		t.Errorf("expected IsDir=false, got true")
	}
}

func TestEnumerate_NonExistentFile(t *testing.T) {
	h := &Handler{}
	_, err := h.Enumerate("/nonexistent/file.jar")

	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestExtractIR_ReturnsError(t *testing.T) {
	h := &Handler{}
	result, err := h.ExtractIR("/some/path.jar", "/some/output")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	expectedMsg := "gobible format does not support IR extraction"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestEmitNative_ReturnsError(t *testing.T) {
	h := &Handler{}
	result, err := h.EmitNative("/some/ir.json", "/some/output")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	expectedMsg := "gobible format does not support native emission"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	if manifest.PluginID != "format.gobible" {
		t.Errorf("expected pluginID=format.gobible, got %s", manifest.PluginID)
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("expected version=1.0.0, got %s", manifest.Version)
	}
	if manifest.Kind != "format" {
		t.Errorf("expected kind=format, got %s", manifest.Kind)
	}
	if manifest.Entrypoint != "format-gobible" {
		t.Errorf("expected entrypoint=format-gobible, got %s", manifest.Entrypoint)
	}

	// Verify capabilities
	if len(manifest.Capabilities.Inputs) != 1 || manifest.Capabilities.Inputs[0] != "file" {
		t.Errorf("expected inputs=[file], got %v", manifest.Capabilities.Inputs)
	}
	if len(manifest.Capabilities.Outputs) != 1 || manifest.Capabilities.Outputs[0] != "artifact.kind:gobible" {
		t.Errorf("expected outputs=[artifact.kind:gobible], got %v", manifest.Capabilities.Outputs)
	}
}

func TestRegister(t *testing.T) {
	// The Register function is called in init(), so we can't test it directly
	// without side effects. We can test that calling it doesn't panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Register() panicked: %v", r)
		}
	}()
	Register()
}

func TestHandler_InterfaceCompliance(t *testing.T) {
	// Verify that Handler implements the expected interface
	var _ plugins.EmbeddedFormatHandler = (*Handler)(nil)
}
