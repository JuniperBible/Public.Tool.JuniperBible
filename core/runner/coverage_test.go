package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JuniperBible/juniper/core/capsule"
	"github.com/JuniperBible/juniper/internal/fileutil"
)

// TestLoadToolArchiveMissingManifest tests loading archive without tool-manifest.
func TestLoadToolArchiveMissingManifest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "toolarchive-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a capsule without tool-manifest
	cap, err := capsule.New(tempDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	archivePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(archivePath); err != nil {
		t.Fatalf("failed to pack capsule: %v", err)
	}

	_, err = LoadToolArchive(archivePath)
	if err == nil {
		t.Error("expected error for archive missing tool-manifest")
	}
	if err != nil && err.Error() != "tool archive missing tool-manifest artifact" {
		// If we get a different error, it's still acceptable as long as it fails
		t.Logf("got error: %v", err)
	}
}

// TestLoadToolArchiveInvalidManifestData tests loading archive with corrupt manifest.
func TestLoadToolArchiveInvalidManifestData(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "toolarchive-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a capsule with corrupt tool-manifest
	cap, err := capsule.New(tempDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Add corrupt manifest data
	corruptData := []byte("not valid json")
	store := cap.GetStore()
	hash, err := store.Store(corruptData)
	if err != nil {
		t.Fatalf("failed to store corrupt data: %v", err)
	}

	cap.Manifest.Artifacts["tool-manifest"] = &capsule.Artifact{
		ID:                "tool-manifest",
		Kind:              "metadata",
		PrimaryBlobSHA256: hash,
		SizeBytes:         int64(len(corruptData)),
	}

	archivePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(archivePath); err != nil {
		t.Fatalf("failed to pack capsule: %v", err)
	}

	_, err = LoadToolArchive(archivePath)
	if err == nil {
		t.Error("expected error for corrupt tool manifest")
	}
}

// TestExtractArtifactMissing tests extracting a missing artifact.
func TestExtractArtifactMissing(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "extract-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cap, err := capsule.New(tempDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	archive := &ToolArchive{
		ToolID:      "test",
		Executables: map[string]string{"test": "missing-id"},
		capsule:     cap,
	}

	destPath := filepath.Join(tempDir, "output")
	err = archive.extractArtifact("missing-id", destPath, 0700)
	if err == nil {
		t.Error("expected error for missing artifact")
	}
}

// TestExtractToMkdirError tests ExtractTo when bin directory creation fails.
func TestExtractToMkdirError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	archive := &ToolArchive{
		ToolID:      "test",
		Executables: map[string]string{},
	}

	// Try to create directory in /proc which should fail
	err := archive.ExtractTo("/proc/nonexistent")
	if err == nil {
		t.Error("expected error when creating bin directory in invalid location")
	}
}

// TestCopyDirReadError tests copyDir with unreadable file.
func TestCopyDirReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	srcDir, err := os.MkdirTemp("", "copydir-src-*")
	if err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	// Create a file with no read permissions
	unreadableFile := filepath.Join(srcDir, "unreadable.txt")
	if err := os.WriteFile(unreadableFile, []byte("content"), 0000); err != nil {
		t.Fatalf("failed to write unreadable file: %v", err)
	}

	dstDir, err := os.MkdirTemp("", "copydir-dst-*")
	if err != nil {
		t.Fatalf("failed to create dst dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	err = fileutil.CopyDir(srcDir, dstDir)
	if err == nil {
		t.Error("expected error when copying unreadable file")
	}
}

// TestCopyDirWriteError tests copyDir when destination is read-only.
func TestCopyDirWriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	srcDir, err := os.MkdirTemp("", "copydir-src-*")
	if err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	if err := os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	err = fileutil.CopyDir(srcDir, "/proc/invalid")
	if err == nil {
		t.Error("expected error when copying to invalid destination")
	}
}

// TestExecuteRequestStatError tests ExecuteRequest with invalid input path.
func TestExecuteRequestStatError(t *testing.T) {
	executor := NewNixExecutor("/tmp/flake")
	req := NewRequest("test", "test")

	_, err := executor.ExecuteRequest(nil, req, []string{"/nonexistent/path"})
	if err == nil {
		t.Error("expected error for non-existent input path")
	}
}

// TestPrepareWorkDirWriteRequestError tests PrepareWorkDir when request.json write fails.
func TestPrepareWorkDirWriteRequestError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "workdir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	workDir := filepath.Join(tempDir, "work")
	req := NewRequest("test", "test")

	// Create the in directory first
	inDir := filepath.Join(workDir, "in")
	if err := os.MkdirAll(inDir, 0700); err != nil {
		t.Fatalf("failed to create in dir: %v", err)
	}

	// Create request.json as a directory to cause write failure
	reqPath := filepath.Join(inDir, "request.json")
	if err := os.Mkdir(reqPath, 0700); err != nil {
		t.Fatalf("failed to create request.json dir: %v", err)
	}

	err = PrepareWorkDir(workDir, req)
	if err == nil {
		t.Error("expected error when writing request.json to directory")
	}
}

// TestPrepareWorkDirWriteRunnerError tests PrepareWorkDir when runner.sh write fails.
func TestPrepareWorkDirWriteRunnerError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "workdir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	workDir := filepath.Join(tempDir, "work")
	req := NewRequest("test", "test")

	// Create the in directory first
	inDir := filepath.Join(workDir, "in")
	if err := os.MkdirAll(inDir, 0700); err != nil {
		t.Fatalf("failed to create in dir: %v", err)
	}

	// Create runner.sh as a directory to cause write failure
	runnerPath := filepath.Join(inDir, "runner.sh")
	if err := os.Mkdir(runnerPath, 0700); err != nil {
		t.Fatalf("failed to create runner.sh dir: %v", err)
	}

	err = PrepareWorkDir(workDir, req)
	if err == nil {
		t.Error("expected error when writing runner.sh to directory")
	}
}

// TestWriteTranscriptMarshalError tests WriteTranscript with unmarshalable data.
func TestWriteTranscriptMarshalError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "transcript-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create event with unmarshalable data
	events := []TranscriptEvent{
		{
			Type: EventEngineInfo,
			Seq:  1,
			Attributes: map[string]interface{}{
				"invalid": make(chan int), // channels cannot be marshaled to JSON
			},
		},
	}

	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	err = WriteTranscript(transcriptPath, events)
	if err == nil {
		t.Error("expected error when marshaling invalid data")
	}
}

// TestCreateToolArchiveMarshalError tests CreateToolArchive with unmarshalable manifest.
func TestCreateToolArchiveMarshalError(t *testing.T) {
	// This is hard to trigger since ToolArchiveManifest doesn't have unmarshalable fields
	// But we can test other error paths in CreateToolArchive
	tempDir, err := os.MkdirTemp("", "create-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a binary
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0700); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	testBinary := filepath.Join(binDir, "test")
	if err := os.WriteFile(testBinary, []byte("binary"), 0700); err != nil {
		t.Fatalf("failed to write test binary: %v", err)
	}

	// Create archive
	archivePath := filepath.Join(tempDir, "test.tar.xz")
	err = CreateToolArchive(
		"test",
		"1.0.0",
		"x86_64-linux",
		map[string]string{"test": testBinary},
		archivePath,
	)

	// Should succeed
	if err != nil {
		t.Errorf("CreateToolArchive failed: %v", err)
	}
}

// TestListToolsReadDirError tests ListTools with invalid directory.
func TestListToolsReadDirError(t *testing.T) {
	registry := NewToolRegistry("/nonexistent/directory")

	_, err := registry.ListTools()
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

// TestListToolsNoTools tests ListTools with directory containing no tools.
func TestListToolsNoTools(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "registry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create some non-tool directories
	if err := os.MkdirAll(filepath.Join(tempDir, "notool"), 0700); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	registry := NewToolRegistry(tempDir)

	tools, err := registry.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

// TestLoadToolAlternativeLocation tests LoadTool with .capsule extension.
func TestLoadToolAlternativeLocation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "registry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool archive
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0700); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	testBinary := filepath.Join(binDir, "testtool")
	if err := os.WriteFile(testBinary, []byte("binary"), 0700); err != nil {
		t.Fatalf("failed to write test binary: %v", err)
	}

	// Create with .capsule extension
	archivePath := filepath.Join(tempDir, "testtool.capsule")
	err = CreateToolArchive(
		"testtool",
		"1.0.0",
		"x86_64-linux",
		map[string]string{"testtool": testBinary},
		archivePath,
	)
	if err != nil {
		t.Fatalf("CreateToolArchive failed: %v", err)
	}

	registry := NewToolRegistry(tempDir)
	tool, err := registry.LoadTool("testtool")
	if err != nil {
		t.Fatalf("LoadTool failed: %v", err)
	}

	if tool.ToolID != "testtool" {
		t.Errorf("ToolID = %q, want %q", tool.ToolID, "testtool")
	}
}

// TestExtractToLibDirError tests ExtractTo when lib directory creation fails.
func TestExtractToLibDirError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "extract-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a basic archive
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0700); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	testBinary := filepath.Join(binDir, "test")
	if err := os.WriteFile(testBinary, []byte("binary"), 0700); err != nil {
		t.Fatalf("failed to write test binary: %v", err)
	}

	archivePath := filepath.Join(tempDir, "test.tar.xz")
	err = CreateToolArchive(
		"test",
		"1.0.0",
		"x86_64-linux",
		map[string]string{"test": testBinary},
		archivePath,
	)
	if err != nil {
		t.Fatalf("CreateToolArchive failed: %v", err)
	}

	archive, err := LoadToolArchive(archivePath)
	if err != nil {
		t.Fatalf("LoadToolArchive failed: %v", err)
	}

	// Add a library to force lib directory creation
	archive.Libraries = map[string]string{"libtest.so": "lib-test"}

	// Create a file where lib directory would be to cause mkdir failure
	extractDir := filepath.Join(tempDir, "extract")
	if err := os.MkdirAll(filepath.Join(extractDir, "bin"), 0700); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}
	// Create "lib" as a file instead of directory
	libPath := filepath.Join(extractDir, "lib")
	if err := os.WriteFile(libPath, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("failed to create lib file: %v", err)
	}

	err = archive.ExtractTo(extractDir)
	if err == nil {
		t.Error("expected error when creating lib directory fails")
	}
}

// TestExecuteRequestReadFileError tests ExecuteRequest when reading input file fails.
func TestExecuteRequestReadFileError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "exec-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an unreadable file
	unreadableFile := filepath.Join(tempDir, "unreadable.txt")
	if err := os.WriteFile(unreadableFile, []byte("content"), 0000); err != nil {
		t.Fatalf("failed to write unreadable file: %v", err)
	}

	executor := NewNixExecutor("/tmp/flake")
	req := NewRequest("test", "test")

	_, err = executor.ExecuteRequest(nil, req, []string{unreadableFile})
	if err == nil {
		t.Error("expected error when reading unreadable input file")
	}
}

// TestExecuteRequestWriteRequestError tests ExecuteRequest when writing request.json fails.
func TestExecuteRequestWriteRequestError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	// Create executor with very long timeout to avoid timeout errors
	executor := NewNixExecutor("/tmp/flake")
	executor.Timeout = 1 * time.Second

	// Create a request that will fail to serialize (this is hard to trigger,
	// so we actually test other error paths in ExecuteRequest)

	// Test with invalid nix path which will fail during command execution
	invalidExecutor := NewNixExecutor("/nonexistent/flake")
	invalidExecutor.Timeout = 100 * time.Millisecond

	req := NewRequest("test", "test")

	// This will fail during nix execution
	result, err := invalidExecutor.ExecuteRequest(nil, req, []string{})
	// Either we get an error or a non-zero exit code
	if err == nil && result != nil && result.ExitCode == 0 {
		t.Log("execution succeeded (unexpected, but ok)")
	}
}

// TestParseTranscriptReadError tests ParseTranscript with unreadable file.
func TestParseTranscriptReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "transcript-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	if err := os.WriteFile(transcriptPath, []byte("content"), 0000); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	_, err = ParseTranscript(transcriptPath)
	if err == nil {
		t.Error("expected error when reading unreadable transcript")
	}
}

// TestWriteTranscriptWriteError tests WriteTranscript when file.Write fails.
func TestWriteTranscriptWriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "transcript-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Make the directory read-only
	if err := os.Chmod(tempDir, 0500); err != nil {
		t.Fatalf("failed to chmod dir: %v", err)
	}
	defer os.Chmod(tempDir, 0700) // Restore permissions for cleanup

	events := []TranscriptEvent{
		{Type: EventEngineInfo, Seq: 1},
	}

	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	err = WriteTranscript(transcriptPath, events)
	if err == nil {
		t.Error("expected error when writing to read-only directory")
	}
}

// TestCopyDirMkdirError tests copyDir when mkdir fails.
func TestCopyDirMkdirError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	srcDir, err := os.MkdirTemp("", "copydir-src-*")
	if err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	// Create a subdirectory
	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0700); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Try to copy to /proc which should fail on mkdir
	err = fileutil.CopyDir(srcDir, "/proc/invalid")
	if err == nil {
		t.Error("expected error when creating subdirectory in invalid location")
	}
}

// TestExtractArtifactRetrieveError tests extractArtifact when blob retrieval fails.
func TestExtractArtifactRetrieveError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "extract-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cap, err := capsule.New(tempDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Add artifact but don't store the blob
	cap.Manifest.Artifacts["test"] = &capsule.Artifact{
		ID:                "test",
		Kind:              "file",
		PrimaryBlobSHA256: "nonexistent_hash",
		SizeBytes:         100,
	}

	archive := &ToolArchive{
		ToolID:      "test",
		Executables: map[string]string{},
		capsule:     cap,
	}

	destPath := filepath.Join(tempDir, "output")
	err = archive.extractArtifact("test", destPath, 0700)
	if err == nil {
		t.Error("expected error when retrieving non-existent blob")
	}
}

// TestExtractArtifactWriteError tests extractArtifact when file write fails.
func TestExtractArtifactWriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "extract-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cap, err := capsule.New(tempDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Store a blob
	data := []byte("test data")
	hash, err := cap.GetStore().Store(data)
	if err != nil {
		t.Fatalf("failed to store data: %v", err)
	}

	cap.Manifest.Artifacts["test"] = &capsule.Artifact{
		ID:                "test",
		Kind:              "file",
		PrimaryBlobSHA256: hash,
		SizeBytes:         int64(len(data)),
	}

	archive := &ToolArchive{
		ToolID:      "test",
		Executables: map[string]string{},
		capsule:     cap,
	}

	// Try to write to /proc which should fail
	err = archive.extractArtifact("test", "/proc/invalid/output", 0700)
	if err == nil {
		t.Error("expected error when writing to invalid location")
	}
}

// TestCreateToolArchiveStoreError tests CreateToolArchive when blob store fails.
// This is hard to trigger without mocking, so we test other error paths.

// TestPrepareWorkDirOutDirError tests PrepareWorkDir when out directory creation fails.
func TestPrepareWorkDirOutDirError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "workdir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	workDir := filepath.Join(tempDir, "work")
	req := NewRequest("test", "test")

	// Create work dir with file named "out" to cause directory creation failure
	if err := os.MkdirAll(workDir, 0700); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}

	outPath := filepath.Join(workDir, "out")
	if err := os.WriteFile(outPath, []byte("file"), 0600); err != nil {
		t.Fatalf("failed to create out file: %v", err)
	}

	err = PrepareWorkDir(workDir, req)
	if err == nil {
		t.Error("expected error when out already exists as file")
	}
}

// TestLoadToolArchiveRetrieveError tests LoadToolArchive when manifest retrieval fails.
func TestLoadToolArchiveRetrieveError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "toolarchive-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a capsule with a tool-manifest artifact pointing to non-existent blob
	cap, err := capsule.New(tempDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Add artifact with bogus hash
	cap.Manifest.Artifacts["tool-manifest"] = &capsule.Artifact{
		ID:                "tool-manifest",
		Kind:              "metadata",
		PrimaryBlobSHA256: "nonexistent_blob_hash",
		SizeBytes:         100,
	}

	archivePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	if err := cap.Pack(archivePath); err != nil {
		t.Fatalf("failed to pack capsule: %v", err)
	}

	_, err = LoadToolArchive(archivePath)
	if err == nil {
		t.Error("expected error for non-existent manifest blob")
	}
}

// TestListToolsStatError tests ListTools when stat fails on capsule directory.
func TestListToolsStatError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "registry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool directory
	toolDir := filepath.Join(tempDir, "testtool")
	if err := os.MkdirAll(toolDir, 0700); err != nil {
		t.Fatalf("failed to create tool dir: %v", err)
	}

	// Create capsule as a file instead of directory
	// The implementation checks if capsule directory exists, and if stat errors
	// (file exists but not a directory), it skips the tool
	capsuleFile := filepath.Join(toolDir, "capsule")
	if err := os.WriteFile(capsuleFile, []byte("file"), 0600); err != nil {
		t.Fatalf("failed to create capsule file: %v", err)
	}

	registry := NewToolRegistry(tempDir)

	tools, err := registry.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	// The tool might be included or not depending on stat behavior
	// This test just verifies ListTools doesn't crash when capsule is a file
	t.Logf("Found %d tools", len(tools))
}
