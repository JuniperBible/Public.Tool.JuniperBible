package capsule

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/cas"
	"github.com/JuniperBible/Public.Tool.JuniperBible/core/ir"
	"github.com/ulikunitz/xz"
)

// TestNewCapsule tests creating a new capsule with basic settings.
func TestNewCapsule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsule, err := New(tempDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	if capsule.Manifest == nil {
		t.Error("manifest should not be nil")
	}

	if capsule.Manifest.CapsuleVersion == "" {
		t.Error("capsule version should be set")
	}
}

// TestIngestFile tests ingesting a single file into a capsule.
func TestIngestFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := []byte("Test file content for ingestion")
	if err := os.WriteFile(testFilePath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create capsule in a separate directory
	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Ingest the file
	artifact, err := capsule.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	if artifact.ID == "" {
		t.Error("artifact ID should not be empty")
	}

	if artifact.Kind != "file" {
		t.Errorf("artifact kind should be 'file', got %q", artifact.Kind)
	}

	if artifact.Hashes.SHA256 == "" {
		t.Error("artifact SHA256 hash should not be empty")
	}

	// Verify the blob exists
	if !capsule.store.Exists(artifact.Hashes.SHA256) {
		t.Error("blob should exist in store")
	}

	// Verify manifest has the artifact
	if _, ok := capsule.Manifest.Artifacts[artifact.ID]; !ok {
		t.Error("artifact should be in manifest")
	}
}

// TestPackAndUnpack tests packing a capsule to tar.xz and unpacking it.
func TestPackAndUnpack(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := []byte("Content for pack/unpack test")
	if err := os.WriteFile(testFilePath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create and populate capsule
	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	artifact, err := capsule.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Pack the capsule
	archivePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := capsule.Pack(archivePath); err != nil {
		t.Fatalf("failed to pack capsule: %v", err)
	}

	// Verify archive exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Fatal("archive should exist")
	}

	// Unpack to a new location
	unpackDir := filepath.Join(tempDir, "unpacked")
	unpacked, err := Unpack(archivePath, unpackDir)
	if err != nil {
		t.Fatalf("failed to unpack capsule: %v", err)
	}

	// Verify manifest
	if unpacked.Manifest.CapsuleVersion != capsule.Manifest.CapsuleVersion {
		t.Error("capsule version should match")
	}

	// Verify artifact exists in unpacked capsule
	unpackedArtifact, ok := unpacked.Manifest.Artifacts[artifact.ID]
	if !ok {
		t.Fatal("artifact should exist in unpacked manifest")
	}

	if unpackedArtifact.Hashes.SHA256 != artifact.Hashes.SHA256 {
		t.Error("artifact hash should match")
	}

	// Verify blob can be retrieved
	data, err := unpacked.store.Retrieve(artifact.Hashes.SHA256)
	if err != nil {
		t.Fatalf("failed to retrieve blob: %v", err)
	}

	if !bytes.Equal(data, testContent) {
		t.Error("retrieved content should match original")
	}
}

// TestPackPreservesAllBlobs tests that all blobs are preserved during pack/unpack.
func TestPackPreservesAllBlobs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create multiple test files
	files := map[string][]byte{
		"file1.txt": []byte("First file content"),
		"file2.txt": []byte("Second file content"),
		"file3.bin": []byte{0x00, 0x01, 0x02, 0x03, 0x04},
	}

	for name, content := range files {
		path := filepath.Join(tempDir, name)
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	// Create and populate capsule
	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	artifacts := make(map[string]*Artifact)
	for name := range files {
		path := filepath.Join(tempDir, name)
		artifact, err := capsule.IngestFile(path)
		if err != nil {
			t.Fatalf("failed to ingest %s: %v", name, err)
		}
		artifacts[name] = artifact
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

	// Verify all files
	for name, content := range files {
		artifact := artifacts[name]
		data, err := unpacked.store.Retrieve(artifact.Hashes.SHA256)
		if err != nil {
			t.Errorf("failed to retrieve %s: %v", name, err)
			continue
		}

		if !bytes.Equal(data, content) {
			t.Errorf("content mismatch for %s", name)
		}
	}
}

// TestManifestIncludedInPack tests that manifest.json is included in the archive.
func TestManifestIncludedInPack(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFilePath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create and populate capsule
	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	if _, err := capsule.IngestFile(testFilePath); err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Pack
	archivePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := capsule.Pack(archivePath); err != nil {
		t.Fatalf("failed to pack: %v", err)
	}

	// Unpack and verify manifest exists
	unpackDir := filepath.Join(tempDir, "unpacked")
	if _, err := Unpack(archivePath, unpackDir); err != nil {
		t.Fatalf("failed to unpack: %v", err)
	}

	manifestPath := filepath.Join(unpackDir, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("manifest.json should exist in unpacked capsule")
	}
}

// TestAddRun tests adding a tool run with transcript to a capsule.
func TestAddRun(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create capsule
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file and ingest it
	testFilePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Create transcript data
	transcript := []byte(`{"event":"start","plugin":"test-tool","profile":"test"}
{"event":"data","value":"test data"}
{"event":"end","exit_code":0}`)

	// Create run
	run := &Run{
		ID: "test-run-1",
		Plugin: &PluginInfo{
			PluginID: "test-tool",
			Kind:     "tool",
		},
		Inputs: []RunInput{
			{ArtifactID: artifact.ID},
		},
		Command: &Command{
			Profile: "test",
		},
		Status: "completed",
	}

	// Add run
	if err := cap.AddRun(run, transcript); err != nil {
		t.Fatalf("failed to add run: %v", err)
	}

	// Verify run is in manifest
	storedRun, ok := cap.Manifest.Runs["test-run-1"]
	if !ok {
		t.Fatal("run should be in manifest")
	}

	if storedRun.Outputs == nil {
		t.Fatal("run outputs should not be nil")
	}

	if storedRun.Outputs.TranscriptBlobSHA256 == "" {
		t.Error("transcript hash should be set")
	}

	// Verify transcript can be retrieved
	retrieved, err := cap.GetTranscript("test-run-1")
	if err != nil {
		t.Fatalf("failed to get transcript: %v", err)
	}

	if !bytes.Equal(retrieved, transcript) {
		t.Error("retrieved transcript should match original")
	}
}

// TestAddRunPackUnpack tests that runs survive pack/unpack.
func TestAddRunPackUnpack(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create capsule
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Ingest a file
	testFilePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFilePath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Add a run
	transcript := []byte(`{"event":"test"}`)
	run := &Run{
		ID:     "run-1",
		Plugin: &PluginInfo{PluginID: "tool"},
		Inputs: []RunInput{{ArtifactID: artifact.ID}},
		Status: "completed",
	}
	if err := cap.AddRun(run, transcript); err != nil {
		t.Fatalf("failed to add run: %v", err)
	}

	// Pack
	archivePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(archivePath); err != nil {
		t.Fatalf("failed to pack: %v", err)
	}

	// Unpack
	unpackDir := filepath.Join(tempDir, "unpacked")
	unpacked, err := Unpack(archivePath, unpackDir)
	if err != nil {
		t.Fatalf("failed to unpack: %v", err)
	}

	// Verify run exists
	unpackedRun, ok := unpacked.Manifest.Runs["run-1"]
	if !ok {
		t.Fatal("run should exist after unpack")
	}

	if unpackedRun.Outputs.TranscriptBlobSHA256 == "" {
		t.Error("transcript hash should be preserved")
	}

	// Verify transcript can be retrieved
	retrieved, err := unpacked.GetTranscript("run-1")
	if err != nil {
		t.Fatalf("failed to get transcript: %v", err)
	}

	if !bytes.Equal(retrieved, transcript) {
		t.Error("transcript content should be preserved")
	}
}

// TestEmptyCapsulePack tests packing an empty capsule (no artifacts).
func TestEmptyCapsulePack(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create empty capsule
	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Pack should succeed even with no artifacts
	archivePath := filepath.Join(tempDir, "empty.capsule.tar.xz")
	if err := capsule.Pack(archivePath); err != nil {
		t.Fatalf("failed to pack empty capsule: %v", err)
	}

	// Unpack should succeed
	unpackDir := filepath.Join(tempDir, "unpacked")
	unpacked, err := Unpack(archivePath, unpackDir)
	if err != nil {
		t.Fatalf("failed to unpack: %v", err)
	}

	if len(unpacked.Manifest.Artifacts) != 0 {
		t.Error("unpacked capsule should have no artifacts")
	}
}

// TestPackWithGzipCompression tests packing with gzip compression.
func TestPackWithGzipCompression(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := []byte("Content for gzip compression test")
	if err := os.WriteFile(testFilePath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create and populate capsule
	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	artifact, err := capsule.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Pack with gzip compression
	archivePath := filepath.Join(tempDir, "test.capsule.tar.gz")
	opts := &PackOptions{Compression: CompressionGzip}
	if err := capsule.PackWithOptions(archivePath, opts); err != nil {
		t.Fatalf("failed to pack capsule with gzip: %v", err)
	}

	// Verify archive exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Fatal("archive should exist")
	}

	// Verify compression type is detected as gzip
	compression, err := DetectCompression(archivePath)
	if err != nil {
		t.Fatalf("failed to detect compression: %v", err)
	}
	if compression != CompressionGzip {
		t.Errorf("expected gzip compression, got %s", compression)
	}

	// Unpack to a new location
	unpackDir := filepath.Join(tempDir, "unpacked")
	unpacked, err := Unpack(archivePath, unpackDir)
	if err != nil {
		t.Fatalf("failed to unpack gzip capsule: %v", err)
	}

	// Verify manifest
	if unpacked.Manifest.CapsuleVersion != capsule.Manifest.CapsuleVersion {
		t.Error("capsule version should match")
	}

	// Verify artifact exists and content matches
	unpackedArtifact, ok := unpacked.Manifest.Artifacts[artifact.ID]
	if !ok {
		t.Fatal("artifact should exist in unpacked manifest")
	}

	if unpackedArtifact.Hashes.SHA256 != artifact.Hashes.SHA256 {
		t.Error("artifact hash should match")
	}

	data, err := unpacked.store.Retrieve(artifact.Hashes.SHA256)
	if err != nil {
		t.Fatalf("failed to retrieve blob: %v", err)
	}

	if !bytes.Equal(data, testContent) {
		t.Error("retrieved content should match original")
	}
}

// TestDetectCompression tests compression format detection.
func TestDetectCompression(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test capsule
	testFilePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFilePath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	if _, err := capsule.IngestFile(testFilePath); err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	tests := []struct {
		name        string
		compression CompressionType
		extension   string
	}{
		{"XZ", CompressionXZ, ".tar.xz"},
		{"Gzip", CompressionGzip, ".tar.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archivePath := filepath.Join(tempDir, "test"+tt.extension)
			opts := &PackOptions{Compression: tt.compression}
			if err := capsule.PackWithOptions(archivePath, opts); err != nil {
				t.Fatalf("failed to pack: %v", err)
			}

			detected, err := DetectCompression(archivePath)
			if err != nil {
				t.Fatalf("failed to detect: %v", err)
			}

			if detected != tt.compression {
				t.Errorf("expected %s, got %s", tt.compression, detected)
			}
		})
	}
}

// TestDefaultPackOptionsUsesXZ verifies default packing uses XZ compression.
func TestDefaultPackOptionsUsesXZ(t *testing.T) {
	opts := DefaultPackOptions()
	if opts.Compression != CompressionXZ {
		t.Errorf("default compression should be XZ, got %s", opts.Compression)
	}
}

// TestGetStore tests that GetStore returns the blob store.
func TestGetStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	store := cap.GetStore()
	if store == nil {
		t.Error("GetStore should return non-nil store")
	}
}

// TestGetRoot tests that GetRoot returns the capsule root directory.
func TestGetRoot(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	root := cap.GetRoot()
	if root != capsuleDir {
		t.Errorf("GetRoot = %q, want %q", root, capsuleDir)
	}
}

// TestSaveManifest tests saving the manifest to disk.
func TestSaveManifest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Ingest a file to have some content
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if _, err := cap.IngestFile(testPath); err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Save manifest
	if err := cap.SaveManifest(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	// Verify manifest file exists
	manifestPath := filepath.Join(capsuleDir, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("manifest.json should exist")
	}

	// Verify content contains expected fields
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}

	if !bytes.Contains(data, []byte("capsule_version")) {
		t.Error("manifest should contain capsule_version")
	}
}

// TestExportToBytes tests exporting an artifact to bytes.
func TestExportToBytes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create capsule and ingest file
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content for export to bytes")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Export to bytes
	data, err := cap.ExportToBytes(artifact.ID, ExportModeIdentity)
	if err != nil {
		t.Fatalf("failed to export to bytes: %v", err)
	}

	if !bytes.Equal(data, testContent) {
		t.Error("exported bytes should match original content")
	}
}

// TestExportToBytesNotFound tests ExportToBytes with missing artifact.
func TestExportToBytesNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	_, err = cap.ExportToBytes("nonexistent", ExportModeIdentity)
	if err == nil {
		t.Error("expected error for missing artifact")
	}
}

// TestGetIRRecord tests retrieving an IR record.
func TestGetIRRecord(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create and store IR using proper API
	testContent := []byte("Test content for IR")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Manually add IR record to manifest for testing
	if cap.Manifest.IRExtractions == nil {
		cap.Manifest.IRExtractions = make(map[string]*IRRecord)
	}
	cap.Manifest.IRExtractions[artifact.ID] = &IRRecord{
		SourceArtifactID: artifact.ID,
		IRBlobSHA256:     "test-hash",
	}

	// Get IR record
	retrieved, err := cap.GetIRRecord(artifact.ID)
	if err != nil {
		t.Fatalf("failed to get IR record: %v", err)
	}

	if retrieved.SourceArtifactID != artifact.ID {
		t.Errorf("SourceArtifactID = %q, want %q", retrieved.SourceArtifactID, artifact.ID)
	}
}

// TestGetIRRecordNotFound tests GetIRRecord with missing record.
func TestGetIRRecordNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Initialize IRExtractions to non-nil so we test the "not found" path
	cap.Manifest.IRExtractions = make(map[string]*IRRecord)

	_, err = cap.GetIRRecord("nonexistent")
	if err == nil {
		t.Error("expected error for missing IR record")
	}
}

// TestGetTranscriptNotFound tests GetTranscript with missing run.
func TestGetTranscriptNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	_, err = cap.GetTranscript("nonexistent-run")
	if err == nil {
		t.Error("expected error for missing run")
	}
}

// TestExportMissingArtifact tests Export with missing artifact.
func TestExportMissingArtifact(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	err = cap.Export("nonexistent", ExportModeIdentity, filepath.Join(tempDir, "out"))
	if err == nil {
		t.Error("expected error for missing artifact")
	}
}

// TestIngestNonexistentFile tests ingesting a file that doesn't exist.
func TestIngestNonexistentFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	_, err = cap.IngestFile("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// TestUnpackNonexistentArchive tests unpacking a file that doesn't exist.
func TestUnpackNonexistentArchive(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	_, err = Unpack("/nonexistent/archive.tar.xz", tempDir)
	if err == nil {
		t.Error("expected error for nonexistent archive")
	}
}

// TestDetectCompressionNonexistent tests detecting compression of nonexistent file.
func TestDetectCompressionNonexistent(t *testing.T) {
	_, err := DetectCompression("/nonexistent/archive.tar.xz")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// TestNewCapsuleInvalidPath tests creating a capsule in an invalid location.
func TestNewCapsuleInvalidPath(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	_, err := New("/proc/invalid/capsule")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

// TestPackWithNilOptions tests packing with nil options defaults to XZ.
func TestPackWithNilOptions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	archivePath := filepath.Join(tempDir, "test.tar.xz")
	if err := cap.PackWithOptions(archivePath, nil); err != nil {
		t.Fatalf("failed to pack with nil options: %v", err)
	}

	// Verify it's XZ
	compression, err := DetectCompression(archivePath)
	if err != nil {
		t.Fatalf("failed to detect compression: %v", err)
	}
	if compression != CompressionXZ {
		t.Errorf("expected XZ, got %s", compression)
	}
}

// TestPackInvalidPath tests packing to an invalid path.
func TestPackInvalidPath(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	err = cap.Pack("/proc/invalid/test.tar.xz")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

// TestDetectCompressionUnknownFormat tests detection with unknown format.
func TestDetectCompressionUnknownFormat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file with unknown magic bytes
	unknownPath := filepath.Join(tempDir, "unknown.dat")
	if err := os.WriteFile(unknownPath, []byte("not a compressed file format"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err = DetectCompression(unknownPath)
	if err == nil {
		t.Error("expected error for unknown compression format")
	}
}

// TestDetectCompressionFileTooSmall tests detection with tiny file.
func TestDetectCompressionFileTooSmall(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file too small to detect
	smallPath := filepath.Join(tempDir, "small.dat")
	if err := os.WriteFile(smallPath, []byte{0x00}, 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err = DetectCompression(smallPath)
	if err == nil {
		t.Error("expected error for file too small")
	}
}

// TestUnpackInvalidArchive tests unpacking a corrupted archive.
func TestUnpackInvalidArchive(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file with gzip magic but invalid content
	invalidPath := filepath.Join(tempDir, "invalid.tar.gz")
	// Write gzip magic followed by garbage
	if err := os.WriteFile(invalidPath, []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00}, 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err = Unpack(invalidPath, filepath.Join(tempDir, "out"))
	if err == nil {
		t.Error("expected error for invalid archive")
	}
}

// TestAddRunEmptyID tests adding a run with empty ID.
func TestAddRunEmptyID(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	run := &Run{ID: ""}
	err = cap.AddRun(run, []byte("transcript"))
	if err == nil {
		t.Error("expected error for empty run ID")
	}
}

// TestGetTranscriptNoOutputs tests getting transcript when run has no outputs.
func TestGetTranscriptNoOutputs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Add run directly to manifest without outputs
	cap.Manifest.Runs = map[string]*Run{
		"test-run": {ID: "test-run"},
	}

	_, err = cap.GetTranscript("test-run")
	if err == nil {
		t.Error("expected error for run with no outputs")
	}
}

// TestSaveManifestInvalidPath tests saving manifest to invalid path.
func TestSaveManifestInvalidPath(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Change root to invalid location
	cap.root = "/proc/invalid"

	err = cap.SaveManifest()
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

// TestGenerateArtifactIDEmpty tests generating ID from empty name.
func TestGenerateArtifactIDEmpty(t *testing.T) {
	id := generateArtifactID("")
	if id != "artifact" {
		t.Errorf("expected 'artifact', got %q", id)
	}
}

// TestGenerateArtifactIDSpecialChars tests generating ID from name with special chars.
func TestGenerateArtifactIDSpecialChars(t *testing.T) {
	id := generateArtifactID("test@#$%file.txt")
	// Should contain only allowed characters
	for _, c := range id {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' || c == ':') {
			t.Errorf("invalid character in ID: %c", c)
		}
	}
}

// TestIngestFileDuplicateIDs tests ingesting files that would have duplicate IDs.
func TestIngestFileDuplicateIDs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create two files with the same name in different directories
	dir1 := filepath.Join(tempDir, "dir1")
	dir2 := filepath.Join(tempDir, "dir2")
	if err := os.MkdirAll(dir1, 0700); err != nil {
		t.Fatalf("failed to create dir1: %v", err)
	}
	if err := os.MkdirAll(dir2, 0700); err != nil {
		t.Fatalf("failed to create dir2: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir1, "test.txt"), []byte("content1"), 0600); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "test.txt"), []byte("content2"), 0600); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Ingest both files - should get unique IDs
	artifact1, err := cap.IngestFile(filepath.Join(dir1, "test.txt"))
	if err != nil {
		t.Fatalf("failed to ingest file1: %v", err)
	}

	artifact2, err := cap.IngestFile(filepath.Join(dir2, "test.txt"))
	if err != nil {
		t.Fatalf("failed to ingest file2: %v", err)
	}

	if artifact1.ID == artifact2.ID {
		t.Error("artifacts should have unique IDs")
	}
}

// TestNewCapsuleStoreError tests New with CAS store creation error.
func TestNewCapsuleStoreError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Inject error
	origNewStore := casNewStore
	casNewStore = func(root string) (*cas.Store, error) {
		return nil, errors.New("injected store error")
	}
	defer func() { casNewStore = origNewStore }()

	_, err = New(filepath.Join(tempDir, "capsule"))
	if err == nil {
		t.Error("expected error for store creation failure")
	}
}

// TestStoreIRMarshalError tests StoreIR with JSON marshal error.
func TestStoreIRMarshalError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Inject error
	origMarshal := jsonMarshalCapsule
	jsonMarshalCapsule = func(v any) ([]byte, error) {
		return nil, errors.New("injected marshal error")
	}
	defer func() { jsonMarshalCapsule = origMarshal }()

	corpus := &ir.Corpus{ID: "test"}
	_, err = cap.StoreIR(corpus, "source")
	if err == nil {
		t.Error("expected error for marshal failure")
	}
}

// TestLoadIRUnmarshalError tests LoadIR with JSON unmarshal error.
func TestLoadIRUnmarshalError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Store a valid IR first
	corpus := &ir.Corpus{ID: "test-corpus", Version: "1.0"}
	artifact, err := cap.StoreIR(corpus, "source")
	if err != nil {
		t.Fatalf("failed to store IR: %v", err)
	}

	// Inject error
	origUnmarshal := jsonUnmarshalCapsule
	jsonUnmarshalCapsule = func(data []byte, v any) error {
		return errors.New("injected unmarshal error")
	}
	defer func() { jsonUnmarshalCapsule = origUnmarshal }()

	_, err = cap.LoadIR(artifact.ID)
	if err == nil {
		t.Error("expected error for unmarshal failure")
	}
}

// TestLoadIRArtifactNotFound tests LoadIR with nonexistent artifact.
func TestLoadIRArtifactNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	_, err = cap.LoadIR("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent artifact")
	}
}

// TestLoadIRWrongArtifactKind tests LoadIR with non-IR artifact.
func TestLoadIRWrongArtifactKind(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Ingest a regular file
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Try to load it as IR
	_, err = cap.LoadIR(artifact.ID)
	if err == nil {
		t.Error("expected error for wrong artifact kind")
	}
}

// TestStoreIRAndLoadIR tests storing and loading IR.
func TestStoreIRAndLoadIR(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create and store corpus
	corpus := &ir.Corpus{
		ID:        "test-corpus",
		Version:   "1.0",
		LossClass: ir.LossL1,
	}
	artifact, err := cap.StoreIR(corpus, "source-artifact")
	if err != nil {
		t.Fatalf("failed to store IR: %v", err)
	}

	if artifact.Kind != ArtifactKindIR {
		t.Errorf("expected kind %s, got %s", ArtifactKindIR, artifact.Kind)
	}

	// Load and verify
	loaded, err := cap.LoadIR(artifact.ID)
	if err != nil {
		t.Fatalf("failed to load IR: %v", err)
	}

	if loaded.ID != corpus.ID {
		t.Errorf("expected corpus ID %s, got %s", corpus.ID, loaded.ID)
	}
	if loaded.Version != corpus.Version {
		t.Errorf("expected version %s, got %s", corpus.Version, loaded.Version)
	}
}

// TestStoreIRDuplicateIDs tests StoreIR generates unique IDs for duplicates.
func TestStoreIRDuplicateIDs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Store two corpora with the same source artifact
	corpus1 := &ir.Corpus{ID: "test", Version: "1.0"}
	corpus2 := &ir.Corpus{ID: "test", Version: "2.0"}

	artifact1, err := cap.StoreIR(corpus1, "source")
	if err != nil {
		t.Fatalf("failed to store first IR: %v", err)
	}

	artifact2, err := cap.StoreIR(corpus2, "source")
	if err != nil {
		t.Fatalf("failed to store second IR: %v", err)
	}

	if artifact1.ID == artifact2.ID {
		t.Error("expected unique artifact IDs")
	}
}

// TestIngestFileStatError tests IngestFile with stat error after read succeeds.
func TestIngestFileStatError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Inject stat error
	origStat := osStatCapsule
	osStatCapsule = func(name string) (os.FileInfo, error) {
		return nil, errors.New("injected stat error")
	}
	defer func() { osStatCapsule = origStat }()

	_, err = cap.IngestFile(testPath)
	if err == nil {
		t.Error("expected error for stat failure")
	}
}

// TestGetIRRecordNilExtractions tests GetIRRecord when IRExtractions is nil.
func TestGetIRRecordNilExtractions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Ensure IRExtractions is nil
	cap.Manifest.IRExtractions = nil

	_, err = cap.GetIRRecord("any-id")
	if err == nil {
		t.Error("expected error for nil IRExtractions")
	}
}

// TestIngestFileStoreError tests IngestFile with store error.
func TestIngestFileStoreError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Inject store error
	origStore := storeStoreWithBlake3
	storeStoreWithBlake3 = func(s *cas.Store, data []byte) (*cas.HashResult, error) {
		return nil, errors.New("injected store error")
	}
	defer func() { storeStoreWithBlake3 = origStore }()

	_, err = cap.IngestFile(testPath)
	if err == nil {
		t.Error("expected error for store failure")
	}
}

// TestAddRunStoreError tests AddRun with store error.
func TestAddRunStoreError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Inject store error
	origStore := storeStoreWithBlake3
	storeStoreWithBlake3 = func(s *cas.Store, data []byte) (*cas.HashResult, error) {
		return nil, errors.New("injected store error")
	}
	defer func() { storeStoreWithBlake3 = origStore }()

	run := &Run{ID: "test-run"}
	err = cap.AddRun(run, []byte("transcript"))
	if err == nil {
		t.Error("expected error for store failure")
	}
}

// TestAddRunNilRunsMap tests AddRun when Runs map is nil.
func TestAddRunNilRunsMap(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Explicitly set Runs to nil
	cap.Manifest.Runs = nil

	run := &Run{ID: "test-run"}
	err = cap.AddRun(run, []byte("transcript"))
	if err != nil {
		t.Fatalf("failed to add run: %v", err)
	}

	// Verify run was added
	if cap.Manifest.Runs["test-run"] == nil {
		t.Error("run should exist in manifest")
	}
}

// TestStoreIRStoreError tests StoreIR with store error.
func TestStoreIRStoreError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Inject store error
	origStore := storeStoreWithBlake3
	storeStoreWithBlake3 = func(s *cas.Store, data []byte) (*cas.HashResult, error) {
		return nil, errors.New("injected store error")
	}
	defer func() { storeStoreWithBlake3 = origStore }()

	corpus := &ir.Corpus{ID: "test"}
	_, err = cap.StoreIR(corpus, "source")
	if err == nil {
		t.Error("expected error for store failure")
	}
}

// TestLoadIRRetrieveError tests LoadIR with retrieve error.
func TestLoadIRRetrieveError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Store IR first
	corpus := &ir.Corpus{ID: "test", Version: "1.0"}
	artifact, err := cap.StoreIR(corpus, "source")
	if err != nil {
		t.Fatalf("failed to store IR: %v", err)
	}

	// Inject retrieve error
	origRetrieve := storeRetrieve
	storeRetrieve = func(s *cas.Store, hash string) ([]byte, error) {
		return nil, errors.New("injected retrieve error")
	}
	defer func() { storeRetrieve = origRetrieve }()

	_, err = cap.LoadIR(artifact.ID)
	if err == nil {
		t.Error("expected error for retrieve failure")
	}
}

// TestSaveManifestMarshalError tests SaveManifest with marshal error.
func TestSaveManifestMarshalError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Inject marshal error via Manifest.ToJSON
	// Since ToJSON uses json.MarshalIndent internally, we add an unmarshalable value
	// Instead, we'll test the WriteFile error path
	origWrite := osWriteFileCapsule
	osWriteFileCapsule = func(name string, data []byte, perm os.FileMode) error {
		return errors.New("injected write error")
	}
	defer func() { osWriteFileCapsule = origWrite }()

	err = cap.SaveManifest()
	if err == nil {
		t.Error("expected error for write failure")
	}
}

// TestUnpackMissingManifest tests Unpack with archive missing manifest.json.
func TestUnpackMissingManifest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a gzip tar without manifest
	archivePath := filepath.Join(tempDir, "no-manifest.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}

	gzWriter, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	tarWriter := tar.NewWriter(gzWriter)

	// Write a dummy file, no manifest
	header := &tar.Header{Name: "dummy.txt", Mode: 0600, Size: 4}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}
	if _, err := tarWriter.Write([]byte("test")); err != nil {
		t.Fatalf("failed to write data: %v", err)
	}

	tarWriter.Close()
	gzWriter.Close()
	file.Close()

	// Try to unpack
	unpackDir := filepath.Join(tempDir, "unpack")
	_, err = Unpack(archivePath, unpackDir)
	if err == nil {
		t.Error("expected error for missing manifest")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("manifest")) {
		t.Errorf("error should mention manifest: %v", err)
	}
}

// TestUnpackInvalidManifest tests Unpack with invalid manifest JSON.
func TestUnpackInvalidManifest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a gzip tar with invalid manifest
	archivePath := filepath.Join(tempDir, "invalid-manifest.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}

	gzWriter, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	tarWriter := tar.NewWriter(gzWriter)

	// Write invalid manifest
	invalidManifest := []byte("not valid json{{{")
	header := &tar.Header{Name: "manifest.json", Mode: 0600, Size: int64(len(invalidManifest))}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}
	if _, err := tarWriter.Write(invalidManifest); err != nil {
		t.Fatalf("failed to write data: %v", err)
	}

	tarWriter.Close()
	gzWriter.Close()
	file.Close()

	// Try to unpack
	unpackDir := filepath.Join(tempDir, "unpack")
	_, err = Unpack(archivePath, unpackDir)
	if err == nil {
		t.Error("expected error for invalid manifest")
	}
}

// TestUnpackPathTraversal tests Unpack rejects path traversal attempts.
func TestUnpackPathTraversal(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a gzip tar with path traversal attempt
	archivePath := filepath.Join(tempDir, "traversal.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}

	gzWriter, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	tarWriter := tar.NewWriter(gzWriter)

	// Write path traversal file (should be skipped)
	header := &tar.Header{Name: "../escape.txt", Mode: 0600, Size: 4}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}
	if _, err := tarWriter.Write([]byte("evil")); err != nil {
		t.Fatalf("failed to write data: %v", err)
	}

	// Write valid manifest
	manifest := NewManifest()
	manifestData, _ := manifest.ToJSON()
	header = &tar.Header{Name: "manifest.json", Mode: 0600, Size: int64(len(manifestData))}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("failed to write manifest header: %v", err)
	}
	if _, err := tarWriter.Write(manifestData); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	tarWriter.Close()
	gzWriter.Close()
	file.Close()

	// Unpack should succeed but skip the malicious file
	unpackDir := filepath.Join(tempDir, "unpack")
	_, err = Unpack(archivePath, unpackDir)
	if err != nil {
		t.Fatalf("unpack failed: %v", err)
	}

	// Verify escape.txt was NOT created outside
	if _, err := os.Stat(filepath.Join(tempDir, "escape.txt")); err == nil {
		t.Error("path traversal file should not have been created")
	}
}

// TestUnpackWithDirectories tests Unpack handles directory entries.
func TestUnpackWithDirectories(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a gzip tar with directory entries
	archivePath := filepath.Join(tempDir, "with-dirs.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}

	gzWriter, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	tarWriter := tar.NewWriter(gzWriter)

	// Write a directory entry
	header := &tar.Header{Name: "subdir/", Mode: 0700, Typeflag: tar.TypeDir}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("failed to write dir header: %v", err)
	}

	// Write a file in that directory
	content := []byte("file in subdir")
	header = &tar.Header{Name: "subdir/file.txt", Mode: 0600, Size: int64(len(content))}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("failed to write file header: %v", err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Write valid manifest
	manifest := NewManifest()
	manifestData, _ := manifest.ToJSON()
	header = &tar.Header{Name: "manifest.json", Mode: 0600, Size: int64(len(manifestData))}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("failed to write manifest header: %v", err)
	}
	if _, err := tarWriter.Write(manifestData); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	tarWriter.Close()
	gzWriter.Close()
	file.Close()

	// Unpack
	unpackDir := filepath.Join(tempDir, "unpack")
	_, err = Unpack(archivePath, unpackDir)
	if err != nil {
		t.Fatalf("unpack failed: %v", err)
	}

	// Verify directory and file exist
	if info, err := os.Stat(filepath.Join(unpackDir, "subdir")); err != nil || !info.IsDir() {
		t.Error("subdir should exist")
	}
	if _, err := os.Stat(filepath.Join(unpackDir, "subdir", "file.txt")); err != nil {
		t.Error("file.txt should exist in subdir")
	}
}

// TestDetectCompressionOpenError tests DetectCompression with open error.
func TestDetectCompressionOpenError(t *testing.T) {
	_, err := DetectCompression("/nonexistent/path/to/archive.tar.xz")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// TestUnpackOpenError tests Unpack with archive open error.
func TestUnpackOpenError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	_, err = Unpack("/nonexistent/archive.tar.xz", filepath.Join(tempDir, "unpack"))
	if err == nil {
		t.Error("expected error for nonexistent archive")
	}
}

// TestUnpackInvalidXZ tests Unpack with corrupted XZ data.
func TestUnpackInvalidXZ(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create file with XZ magic but invalid content
	archivePath := filepath.Join(tempDir, "invalid.tar.xz")
	// XZ magic: 0xFD 0x37 0x7A 0x58 0x5A 0x00
	xzMagic := []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00, 0xFF, 0xFF}
	if err := os.WriteFile(archivePath, xzMagic, 0600); err != nil {
		t.Fatalf("failed to write invalid xz: %v", err)
	}

	_, err = Unpack(archivePath, filepath.Join(tempDir, "unpack"))
	if err == nil {
		t.Error("expected error for invalid XZ data")
	}
}

// TestUnpackInvalidGzip tests Unpack with corrupted gzip data.
func TestUnpackInvalidGzip(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create file with gzip magic but invalid content
	archivePath := filepath.Join(tempDir, "invalid.tar.gz")
	// Gzip magic: 0x1F 0x8B
	gzipMagic := []byte{0x1F, 0x8B, 0xFF, 0xFF, 0xFF, 0xFF}
	if err := os.WriteFile(archivePath, gzipMagic, 0600); err != nil {
		t.Fatalf("failed to write invalid gzip: %v", err)
	}

	_, err = Unpack(archivePath, filepath.Join(tempDir, "unpack"))
	if err == nil {
		t.Error("expected error for invalid gzip data")
	}
}

// =============================================================================
// Guard tests: These verify that "impossible" error conditions remain impossible.
// If any of these tests fail, the corresponding error handling code needs review.
// =============================================================================

// TestGzipNewWriterLevelNeverFails is a guard test verifying gzip.NewWriterLevel
// with valid compression levels never returns an error. The error check in
// PackWithOptions exists defensively but cannot be triggered with BestCompression.
func TestGzipNewWriterLevelNeverFails(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "guard-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	file, err := os.Create(filepath.Join(tempDir, "test.gz"))
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	defer file.Close()

	// All valid compression levels should never fail
	levels := []int{gzip.NoCompression, gzip.BestSpeed, gzip.BestCompression, gzip.DefaultCompression, gzip.HuffmanOnly}
	for _, level := range levels {
		w, err := gzip.NewWriterLevel(file, level)
		if err != nil {
			t.Errorf("gzip.NewWriterLevel with level %d should never fail: %v", level, err)
		}
		if w != nil {
			w.Close()
		}
	}
}

// TestWriteToTarCanFail verifies writeToTar properly handles WriteHeader errors.
// This is tested by closing the tar writer before use.
func TestWriteToTarCanFail(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "guard-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	file, err := os.Create(filepath.Join(tempDir, "test.tar"))
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	tw := tar.NewWriter(file)
	tw.Close() // Close the tar writer
	file.Close()

	// Now writeToTarImpl should fail
	err = writeToTarImpl(tw, "test.txt", []byte("data"))
	if err == nil {
		t.Error("writeToTarImpl should fail on closed tar writer")
	}
}

// TestUnpackUnsupportedCompression tests Unpack with unsupported compression.
// This validates the default case in the compression switch.
func TestUnpackUnsupportedCompression(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create file with unrecognized magic bytes
	archivePath := filepath.Join(tempDir, "unknown.bin")
	unknownMagic := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	if err := os.WriteFile(archivePath, unknownMagic, 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err = Unpack(archivePath, filepath.Join(tempDir, "unpack"))
	if err == nil {
		t.Error("expected error for unsupported compression")
	}
}

// TestExportIdentityRetrieveError tests exportIdentity with retrieve error.
func TestExportIdentityRetrieveError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create an artifact with a nonexistent blob hash
	artifactID := "broken-artifact"
	cap.Manifest.Artifacts[artifactID] = &Artifact{
		ID:                artifactID,
		PrimaryBlobSHA256: "0000000000000000000000000000000000000000000000000000000000000000",
	}

	// Try to export - should fail on retrieve
	_, err = cap.ExportToBytes(artifactID, ExportModeIdentity)
	if err == nil {
		t.Error("expected error for blob not found")
	}
}

// TestExportToFile tests Export to a file path with retrieve error.
func TestExportToFileRetrieveError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create an artifact with a nonexistent blob hash
	artifactID := "broken-artifact"
	cap.Manifest.Artifacts[artifactID] = &Artifact{
		ID:                artifactID,
		PrimaryBlobSHA256: "0000000000000000000000000000000000000000000000000000000000000000",
	}

	// Try to export - should fail on retrieve
	err = cap.Export(artifactID, ExportModeIdentity, filepath.Join(tempDir, "output.txt"))
	if err == nil {
		t.Error("expected error for blob not found")
	}
}

// =============================================================================
// Guard tests for compression library error paths.
// These verify that compression library errors cannot occur with valid inputs,
// documenting why those error paths remain uncovered.
// =============================================================================

// TestXZWriterNeverFailsWithValidWriter verifies xz.NewWriter doesn't fail
// with a valid io.Writer. The error check in PackWithOptions is defensive.
func TestXZWriterNeverFailsWithValidWriter(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "guard-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	file, err := os.Create(filepath.Join(tempDir, "test.xz"))
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	defer file.Close()

	// xz.NewWriter with valid io.Writer should never fail
	w, err := xz.NewWriter(file)
	if err != nil {
		t.Errorf("xz.NewWriter with valid io.Writer should never fail: %v", err)
	}
	if w != nil {
		w.Close()
	}
}

// TestFilepathRelNeverFailsWithAbsolutePaths verifies filepath.Rel doesn't fail
// when both paths are valid absolute paths within the same root.
func TestFilepathRelNeverFailsWithAbsolutePaths(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "guard-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	absRoot := tempDir
	absPath := filepath.Join(tempDir, "subdir", "file.txt")

	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		t.Errorf("filepath.Rel with valid absolute paths should never fail: %v", err)
	}
	if rel == "" {
		t.Error("relative path should not be empty")
	}
}

// TestManifestToJSONNeverFails verifies that Manifest.ToJSON() doesn't fail
// with valid manifest data. The error check exists defensively.
func TestManifestToJSONNeverFails(t *testing.T) {
	manifest := NewManifest()

	// Add some data
	manifest.Artifacts["test"] = &Artifact{ID: "test", Kind: "file"}
	manifest.Blobs.BySHA256["abc"] = &BlobRecord{SHA256: "abc"}
	manifest.Runs = map[string]*Run{"run1": {ID: "run1"}}

	data, err := manifest.ToJSON()
	if err != nil {
		t.Errorf("Manifest.ToJSON with valid data should never fail: %v", err)
	}
	if len(data) == 0 {
		t.Error("manifest JSON should not be empty")
	}
}

// TestUnpackDetectCompressionError tests that Unpack fails on unknown compression
// at the DetectCompression stage (before opening for decompression).
func TestUnpackDetectCompressionError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create file with unrecognized magic bytes that won't be detected
	archivePath := filepath.Join(tempDir, "bad.capsule")
	badMagic := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x00, 0x00, 0x00}
	if err := os.WriteFile(archivePath, badMagic, 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err = Unpack(archivePath, filepath.Join(tempDir, "unpack"))
	if err == nil {
		t.Error("expected error for unknown compression")
	}
}

// =============================================================================
// Injection tests for 100% coverage of all error paths
// =============================================================================

// TestPackWithOptionsGzipWriterError tests PackWithOptions gzip.NewWriterLevel error.
func TestPackWithOptionsGzipWriterError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Inject error
	orig := gzipNewWriterLevel
	gzipNewWriterLevel = func(w io.Writer, level int) (*gzip.Writer, error) {
		return nil, errors.New("injected gzip writer error")
	}
	defer func() { gzipNewWriterLevel = orig }()

	err = cap.PackWithOptions(filepath.Join(tempDir, "test.tar.gz"), &PackOptions{Compression: CompressionGzip})
	if err == nil {
		t.Error("expected error for gzip writer failure")
	}
}

// TestPackWithOptionsXzWriterError tests PackWithOptions xz.NewWriter error.
func TestPackWithOptionsXzWriterError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Inject error
	orig := xzNewWriter
	xzNewWriter = func(w io.Writer) (*xz.Writer, error) {
		return nil, errors.New("injected xz writer error")
	}
	defer func() { xzNewWriter = orig }()

	err = cap.PackWithOptions(filepath.Join(tempDir, "test.tar.xz"), nil)
	if err == nil {
		t.Error("expected error for xz writer failure")
	}
}

// TestPackWithOptionsManifestError tests PackWithOptions manifest.ToJSON error.
func TestPackWithOptionsManifestError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Inject error
	orig := manifestToJSONPack
	manifestToJSONPack = func(m *Manifest) ([]byte, error) {
		return nil, errors.New("injected manifest error")
	}
	defer func() { manifestToJSONPack = orig }()

	err = cap.PackWithOptions(filepath.Join(tempDir, "test.tar.xz"), nil)
	if err == nil {
		t.Error("expected error for manifest serialization failure")
	}
}

// TestPackWithOptionsWalkError tests PackWithOptions filepath.Walk error.
func TestPackWithOptionsWalkError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Ingest a file so blobs dir exists
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := cap.IngestFile(testPath); err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Inject walk error
	orig := filepathWalk
	filepathWalk = func(root string, fn filepath.WalkFunc) error {
		// Call the function with an error to trigger the callback error path
		return fn(root, nil, errors.New("injected walk error"))
	}
	defer func() { filepathWalk = orig }()

	err = cap.PackWithOptions(filepath.Join(tempDir, "test.tar.xz"), nil)
	if err == nil {
		t.Error("expected error for walk callback failure")
	}
}

// TestPackWithOptionsRelError tests PackWithOptions filepath.Rel error.
func TestPackWithOptionsRelError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Ingest a file so blobs dir exists
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := cap.IngestFile(testPath); err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Inject rel error
	orig := filepathRel
	filepathRel = func(basepath, targpath string) (string, error) {
		return "", errors.New("injected rel error")
	}
	defer func() { filepathRel = orig }()

	err = cap.PackWithOptions(filepath.Join(tempDir, "test.tar.xz"), nil)
	if err == nil {
		t.Error("expected error for rel failure")
	}
}

// TestPackWithOptionsReadFileError tests PackWithOptions os.ReadFile error.
func TestPackWithOptionsReadFileError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Ingest a file so blobs dir exists
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := cap.IngestFile(testPath); err != nil {
		t.Fatalf("failed to ingest: %v", err)
	}

	// Inject read error
	orig := osReadFileWalk
	osReadFileWalk = func(name string) ([]byte, error) {
		return nil, errors.New("injected read error")
	}
	defer func() { osReadFileWalk = orig }()

	err = cap.PackWithOptions(filepath.Join(tempDir, "test.tar.xz"), nil)
	if err == nil {
		t.Error("expected error for read file failure")
	}
}

// TestDetectCompressionReadError tests DetectCompression file.Read error.
func TestDetectCompressionReadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a valid file
	archivePath := filepath.Join(tempDir, "test.tar.xz")
	if err := os.WriteFile(archivePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Inject read error
	orig := fileReadDetect
	fileReadDetect = func(r io.Reader, b []byte) (int, error) {
		return 0, errors.New("injected read error")
	}
	defer func() { fileReadDetect = orig }()

	_, err = DetectCompression(archivePath)
	if err == nil {
		t.Error("expected error for read failure")
	}
}

// TestUnpackMkdirAllDestError tests Unpack osMkdirAllUnpack dest error.
func TestUnpackMkdirAllDestError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a valid gzip archive first
	archivePath := filepath.Join(tempDir, "test.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	gzWriter, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	tarWriter := tar.NewWriter(gzWriter)
	manifest := NewManifest()
	manifestData, _ := manifest.ToJSON()
	header := &tar.Header{Name: "manifest.json", Mode: 0600, Size: int64(len(manifestData))}
	tarWriter.WriteHeader(header)
	tarWriter.Write(manifestData)
	tarWriter.Close()
	gzWriter.Close()
	file.Close()

	// Inject mkdir error
	orig := osMkdirAllUnpack
	osMkdirAllUnpack = func(path string, perm os.FileMode) error {
		return errors.New("injected mkdir error")
	}
	defer func() { osMkdirAllUnpack = orig }()

	_, err = Unpack(archivePath, filepath.Join(tempDir, "unpack"))
	if err == nil {
		t.Error("expected error for mkdir failure")
	}
}

// TestUnpackTypeDirMkdirAllError tests Unpack TypeDir MkdirAll error.
func TestUnpackTypeDirMkdirAllError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a valid gzip archive with a directory entry
	archivePath := filepath.Join(tempDir, "test.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	gzWriter, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	tarWriter := tar.NewWriter(gzWriter)

	// Add directory entry
	dirHeader := &tar.Header{Name: "subdir/", Mode: 0700, Typeflag: tar.TypeDir}
	tarWriter.WriteHeader(dirHeader)

	// Add manifest
	manifest := NewManifest()
	manifestData, _ := manifest.ToJSON()
	header := &tar.Header{Name: "manifest.json", Mode: 0600, Size: int64(len(manifestData))}
	tarWriter.WriteHeader(header)
	tarWriter.Write(manifestData)
	tarWriter.Close()
	gzWriter.Close()
	file.Close()

	// Inject mkdir error only for subdirectories
	orig := osMkdirAllUnpack
	callCount := 0
	osMkdirAllUnpack = func(path string, perm os.FileMode) error {
		callCount++
		if callCount == 1 {
			// Allow first call (dest dir creation)
			return os.MkdirAll(path, perm)
		}
		// Fail subsequent calls (directory entries)
		return errors.New("injected mkdir error")
	}
	defer func() { osMkdirAllUnpack = orig }()

	_, err = Unpack(archivePath, filepath.Join(tempDir, "unpack"))
	if err == nil {
		t.Error("expected error for TypeDir mkdir failure")
	}
}

// TestUnpackTypeRegParentMkdirAllError tests Unpack TypeReg parent MkdirAll error.
func TestUnpackTypeRegParentMkdirAllError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a valid gzip archive with nested file
	archivePath := filepath.Join(tempDir, "test.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	gzWriter, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	tarWriter := tar.NewWriter(gzWriter)

	// Add nested file
	fileContent := []byte("content")
	fileHeader := &tar.Header{Name: "nested/file.txt", Mode: 0600, Size: int64(len(fileContent))}
	tarWriter.WriteHeader(fileHeader)
	tarWriter.Write(fileContent)

	tarWriter.Close()
	gzWriter.Close()
	file.Close()

	// Inject mkdir error only for nested parent directories
	orig := osMkdirAllUnpack
	callCount := 0
	osMkdirAllUnpack = func(path string, perm os.FileMode) error {
		callCount++
		if callCount == 1 {
			// Allow first call (dest dir creation)
			return os.MkdirAll(path, perm)
		}
		// Fail subsequent calls (parent dir for file)
		return errors.New("injected mkdir error")
	}
	defer func() { osMkdirAllUnpack = orig }()

	_, err = Unpack(archivePath, filepath.Join(tempDir, "unpack"))
	if err == nil {
		t.Error("expected error for TypeReg parent mkdir failure")
	}
}

// TestUnpackReadAllError tests Unpack ioReadAllUnpack error.
func TestUnpackReadAllError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a valid gzip archive
	archivePath := filepath.Join(tempDir, "test.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	gzWriter, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	tarWriter := tar.NewWriter(gzWriter)

	// Add file
	fileContent := []byte("content")
	fileHeader := &tar.Header{Name: "file.txt", Mode: 0600, Size: int64(len(fileContent))}
	tarWriter.WriteHeader(fileHeader)
	tarWriter.Write(fileContent)

	tarWriter.Close()
	gzWriter.Close()
	file.Close()

	// Inject read all error
	orig := ioReadAllUnpack
	ioReadAllUnpack = func(r io.Reader) ([]byte, error) {
		return nil, errors.New("injected read all error")
	}
	defer func() { ioReadAllUnpack = orig }()

	_, err = Unpack(archivePath, filepath.Join(tempDir, "unpack"))
	if err == nil {
		t.Error("expected error for read all failure")
	}
}

// TestUnpackWriteFileError tests Unpack osWriteFileUnpack error.
func TestUnpackWriteFileError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a valid gzip archive
	archivePath := filepath.Join(tempDir, "test.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	gzWriter, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	tarWriter := tar.NewWriter(gzWriter)

	// Add file
	fileContent := []byte("content")
	fileHeader := &tar.Header{Name: "file.txt", Mode: 0600, Size: int64(len(fileContent))}
	tarWriter.WriteHeader(fileHeader)
	tarWriter.Write(fileContent)

	tarWriter.Close()
	gzWriter.Close()
	file.Close()

	// Inject write file error
	orig := osWriteFileUnpack
	osWriteFileUnpack = func(name string, data []byte, perm os.FileMode) error {
		return errors.New("injected write file error")
	}
	defer func() { osWriteFileUnpack = orig }()

	_, err = Unpack(archivePath, filepath.Join(tempDir, "unpack"))
	if err == nil {
		t.Error("expected error for write file failure")
	}
}

// TestUnpackCASStoreError tests Unpack casNewStoreUnpack error.
func TestUnpackCASStoreError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a valid gzip archive with manifest
	archivePath := filepath.Join(tempDir, "test.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	gzWriter, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	tarWriter := tar.NewWriter(gzWriter)

	// Add manifest
	manifest := NewManifest()
	manifestData, _ := manifest.ToJSON()
	header := &tar.Header{Name: "manifest.json", Mode: 0600, Size: int64(len(manifestData))}
	tarWriter.WriteHeader(header)
	tarWriter.Write(manifestData)

	tarWriter.Close()
	gzWriter.Close()
	file.Close()

	// Inject CAS store error
	orig := casNewStoreUnpack
	casNewStoreUnpack = func(root string) (*cas.Store, error) {
		return nil, errors.New("injected store error")
	}
	defer func() { casNewStoreUnpack = orig }()

	_, err = Unpack(archivePath, filepath.Join(tempDir, "unpack"))
	if err == nil {
		t.Error("expected error for CAS store failure")
	}
}

// TestSaveManifestToJSONError tests SaveManifest manifestToJSONSave error.
func TestSaveManifestToJSONError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Inject ToJSON error
	orig := manifestToJSONSave
	manifestToJSONSave = func(m *Manifest) ([]byte, error) {
		return nil, errors.New("injected ToJSON error")
	}
	defer func() { manifestToJSONSave = orig }()

	err = cap.SaveManifest()
	if err == nil {
		t.Error("expected error for ToJSON failure")
	}
}

// TestUnpackTarHeaderError tests Unpack tar header read error.
func TestUnpackTarHeaderError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a gzip file with invalid tar content
	archivePath := filepath.Join(tempDir, "test.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	gzWriter, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	// Write invalid tar data
	gzWriter.Write([]byte("not valid tar data"))
	gzWriter.Close()
	file.Close()

	_, err = Unpack(archivePath, filepath.Join(tempDir, "unpack"))
	if err == nil {
		t.Error("expected error for invalid tar data")
	}
}

// TestPackWithOptionsWriteToTarManifestError tests writeToTarFunc error for manifest.
func TestPackWithOptionsWriteToTarManifestError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	archivePath := filepath.Join(tempDir, "test.tar.gz")

	// Inject writeToTar error
	orig := writeToTarFunc
	callCount := 0
	writeToTarFunc = func(tw *tar.Writer, name string, data []byte) error {
		callCount++
		if callCount == 1 && name == "manifest.json" {
			return errors.New("injected tar write error")
		}
		return orig(tw, name, data)
	}
	defer func() { writeToTarFunc = orig }()

	opts := &PackOptions{Compression: CompressionGzip}
	err = cap.PackWithOptions(archivePath, opts)
	if err == nil {
		t.Error("expected error for writeToTar manifest failure")
	}
	if !strings.Contains(err.Error(), "failed to write manifest") {
		t.Errorf("error = %v, want to contain 'failed to write manifest'", err)
	}
}

// TestUnpackOsOpenInjectError tests osOpenUnpack injected error.
func TestUnpackOsOpenInjectError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a valid gzip archive first for DetectCompression to succeed
	archivePath := filepath.Join(tempDir, "test.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	gzWriter, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	gzWriter.Write([]byte("test"))
	gzWriter.Close()
	file.Close()

	// Inject open error
	orig := osOpenUnpack
	osOpenUnpack = func(name string) (*os.File, error) {
		return nil, errors.New("injected open error")
	}
	defer func() { osOpenUnpack = orig }()

	_, err = Unpack(archivePath, filepath.Join(tempDir, "unpack"))
	if err == nil {
		t.Error("expected error for open failure")
	}
	if !strings.Contains(err.Error(), "failed to open archive") {
		t.Errorf("error = %v, want to contain 'failed to open archive'", err)
	}
}

// TestUnpackDefaultCompressionGuard is a guard test documenting that the
// default case in Unpack's compression switch is unreachable with valid DetectCompression.
// DetectCompression only returns CompressionGzip, CompressionXZ, or an error.
// This test verifies the defensive coding pattern is in place.
func TestUnpackDefaultCompressionGuard(t *testing.T) {
	// This is a guard test - the default case (line 339-340) in Unpack is
	// unreachable because DetectCompression only returns valid CompressionType
	// values or an error. We verify this by checking that DetectCompression
	// only produces known types or errors.

	testCases := []struct {
		name     string
		magic    []byte
		wantType CompressionType
		wantErr  bool
	}{
		{"gzip", []byte{0x1f, 0x8b}, CompressionGzip, false},
		{"xz", []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}, CompressionXZ, false},
		{"unknown", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, "", true},
		{"partial", []byte{0x1f}, "", true}, // too small
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "detect-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			archivePath := filepath.Join(tempDir, "test.bin")
			if err := os.WriteFile(archivePath, tc.magic, 0600); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			result, err := DetectCompression(archivePath)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %s magic", tc.name)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tc.wantType {
					t.Errorf("got %v, want %v", result, tc.wantType)
				}
			}
		})
	}
}
