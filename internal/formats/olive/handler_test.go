package olive

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	manifest := Manifest()
	if manifest == nil {
		t.Fatal("Manifest() returned nil")
	}

	if manifest.PluginID != "format.olive" {
		t.Errorf("Expected PluginID 'format.olive', got '%s'", manifest.PluginID)
	}

	if manifest.Kind != "format" {
		t.Errorf("Expected Kind 'format', got '%s'", manifest.Kind)
	}

	if manifest.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got '%s'", manifest.Version)
	}

	if manifest.Entrypoint != "format-olive" {
		t.Errorf("Expected Entrypoint 'format-olive', got '%s'", manifest.Entrypoint)
	}
}

func TestDetect_OT4I(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ot4i")

	// Create a test file with SQLite signature
	sqliteHeader := []byte("SQLite format 3\x00")
	if err := os.WriteFile(testFile, sqliteHeader, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}

	if !result.Detected {
		t.Errorf("Expected Detected=true, got false. Reason: %s", result.Reason)
	}

	if result.Format != "OliveTree" {
		t.Errorf("Expected Format 'OliveTree', got '%s'", result.Format)
	}

	if !strings.Contains(result.Reason, "SQLite-based") {
		t.Errorf("Expected reason to contain 'SQLite-based', got: %s", result.Reason)
	}
}

func TestDetect_OT4I_Encrypted(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ot4i")

	// Create a test file without SQLite signature (encrypted)
	encryptedData := []byte("ENCRYPTED_DATA_HERE_NOT_SQLITE")
	if err := os.WriteFile(testFile, encryptedData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}

	if !result.Detected {
		t.Errorf("Expected Detected=true, got false. Reason: %s", result.Reason)
	}

	if !strings.Contains(result.Reason, "proprietary/encrypted") {
		t.Errorf("Expected reason to contain 'proprietary/encrypted', got: %s", result.Reason)
	}
}

func TestDetect_OTI(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.oti")

	// Create a test file
	data := []byte("Test Olive Tree Index")
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}

	if !result.Detected {
		t.Errorf("Expected Detected=true, got false. Reason: %s", result.Reason)
	}

	if result.Format != "OliveTree" {
		t.Errorf("Expected Format 'OliveTree', got '%s'", result.Format)
	}
}

func TestDetect_OTM(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.otm")

	// Create a test file
	data := []byte("Test Olive Tree Module")
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}

	if !result.Detected {
		t.Errorf("Expected Detected=true, got false. Reason: %s", result.Reason)
	}

	if result.Format != "OliveTree" {
		t.Errorf("Expected Format 'OliveTree', got '%s'", result.Format)
	}
}

func TestDetect_PDB_OliveTree(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.pdb")

	// Create a valid PDB header with Olive Tree creator
	header := PDBHeader{
		Attributes: 0,
		Version:    1,
	}
	copy(header.Name[:], "KJV Bible")
	copy(header.Creator[:], "OlTr")
	copy(header.Type[:], "DATA")
	header.NumRecords = 1

	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer file.Close()

	if err := binary.Write(file, binary.BigEndian, &header); err != nil {
		t.Fatalf("Failed to write PDB header: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}

	if !result.Detected {
		t.Errorf("Expected Detected=true, got false. Reason: %s", result.Reason)
	}

	if result.Format != "OliveTree" {
		t.Errorf("Expected Format 'OliveTree', got '%s'", result.Format)
	}
}

func TestDetect_PDB_BibleKeyword(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.pdb")

	// Create a valid PDB header with Bible keyword in name
	header := PDBHeader{
		Attributes: 0,
		Version:    1,
	}
	copy(header.Name[:], "NIV Bible Translation")
	copy(header.Creator[:], "TEST")
	copy(header.Type[:], "DATA")
	header.NumRecords = 1

	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer file.Close()

	if err := binary.Write(file, binary.BigEndian, &header); err != nil {
		t.Fatalf("Failed to write PDB header: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}

	if !result.Detected {
		t.Errorf("Expected Detected=true, got false. Reason: %s", result.Reason)
	}
}

func TestDetect_PDB_NotOliveTree(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.pdb")

	// Create a valid PDB header without Olive Tree identifiers
	header := PDBHeader{
		Attributes: 0,
		Version:    1,
	}
	copy(header.Name[:], "Random PDB File")
	copy(header.Creator[:], "TEST")
	copy(header.Type[:], "DATA")
	header.NumRecords = 1

	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer file.Close()

	if err := binary.Write(file, binary.BigEndian, &header); err != nil {
		t.Fatalf("Failed to write PDB header: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}

	if result.Detected {
		t.Errorf("Expected Detected=false for non-Olive Tree PDB")
	}

	if !strings.Contains(result.Reason, "does not appear to be Olive Tree") {
		t.Errorf("Expected reason to mention PDB not being Olive Tree format, got: %s", result.Reason)
	}
}

func TestDetect_InvalidExtension(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}

	if result.Detected {
		t.Errorf("Expected Detected=false for .txt file")
	}

	if !strings.Contains(result.Reason, "not a known Olive Tree format") {
		t.Errorf("Expected reason to mention unknown extension, got: %s", result.Reason)
	}
}

func TestDetect_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	handler := &Handler{}
	result, err := handler.Detect(tmpDir)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}

	if result.Detected {
		t.Errorf("Expected Detected=false for directory")
	}

	if !strings.Contains(result.Reason, "directory") {
		t.Errorf("Expected reason to mention directory, got: %s", result.Reason)
	}
}

func TestDetect_NonExistent(t *testing.T) {
	handler := &Handler{}
	result, err := handler.Detect("/nonexistent/path/test.ot4i")
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}

	if result.Detected {
		t.Errorf("Expected Detected=false for non-existent file")
	}

	if !strings.Contains(result.Reason, "cannot stat") {
		t.Errorf("Expected reason to mention stat error, got: %s", result.Reason)
	}
}

func TestDetect_TooSmall(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ot4i")

	// Create a file that's too small (less than 16 bytes)
	if err := os.WriteFile(testFile, []byte("small"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}

	if result.Detected {
		t.Errorf("Expected Detected=false for file too small")
	}

	if !strings.Contains(result.Reason, "too small") {
		t.Errorf("Expected reason to mention file too small, got: %s", result.Reason)
	}
}

func TestDetectPDBFormat_InvalidFile(t *testing.T) {
	detected, reason := detectPDBFormat("/nonexistent/file.pdb")
	if detected {
		t.Errorf("Expected detected=false for non-existent file")
	}
	if !strings.Contains(reason, "cannot open") {
		t.Errorf("Expected reason to mention 'cannot open', got: %s", reason)
	}
}

func TestDetectPDBFormat_InvalidHeader(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.pdb")

	// Create a file with invalid/incomplete PDB header
	if err := os.WriteFile(testFile, []byte("invalid"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	detected, reason := detectPDBFormat(testFile)
	if detected {
		t.Errorf("Expected detected=false for invalid header")
	}
	if !strings.Contains(reason, "failed to read PDB header") {
		t.Errorf("Expected reason to mention header read failure, got: %s", reason)
	}
}

func TestDetectPDBFormat_BbRdCreator(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.pdb")

	// Create a valid PDB header with BbRd creator
	header := PDBHeader{
		Attributes: 0,
		Version:    1,
	}
	copy(header.Name[:], "Test Bible")
	copy(header.Creator[:], "BbRd")
	copy(header.Type[:], "DATA")
	header.NumRecords = 1

	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer file.Close()

	if err := binary.Write(file, binary.BigEndian, &header); err != nil {
		t.Fatalf("Failed to write PDB header: %v", err)
	}

	detected, _ := detectPDBFormat(testFile)
	if !detected {
		t.Errorf("Expected detected=true for BbRd creator")
	}
}

func TestIngest(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ot4i")
	outputDir := filepath.Join(tmpDir, "output")

	testData := []byte("Test Olive Tree Bible file content")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Ingest() returned error: %v", err)
	}

	if result.ArtifactID != "test" {
		t.Errorf("Expected ArtifactID 'test', got '%s'", result.ArtifactID)
	}

	if result.BlobSHA256 == "" {
		t.Errorf("Expected non-empty BlobSHA256")
	}

	if result.SizeBytes != int64(len(testData)) {
		t.Errorf("Expected SizeBytes %d, got %d", len(testData), result.SizeBytes)
	}

	if result.Metadata["format"] != "OliveTree" {
		t.Errorf("Expected Metadata format 'OliveTree', got '%s'", result.Metadata["format"])
	}

	if result.Metadata["warning"] == "" {
		t.Errorf("Expected warning in metadata")
	}

	// Verify blob was written correctly
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	blobData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("Failed to read blob file: %v", err)
	}

	if string(blobData) != string(testData) {
		t.Errorf("Blob data doesn't match original data")
	}
}

func TestIngest_NonExistentFile(t *testing.T) {
	handler := &Handler{}
	_, err := handler.Ingest("/nonexistent/file.ot4i", t.TempDir())
	if err == nil {
		t.Errorf("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected error message about failed read, got: %v", err)
	}
}

func TestIngest_InvalidOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ot4i")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	handler := &Handler{}
	// Use a file as output dir (should fail)
	invalidDir := filepath.Join(tmpDir, "notadir")
	os.WriteFile(invalidDir, []byte("file"), 0644)

	_, err := handler.Ingest(testFile, filepath.Join(invalidDir, "subdir"))
	if err == nil {
		t.Errorf("Expected error for invalid output directory")
	}
}

func TestIngest_CannotWriteBlob(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ot4i")
	outputDir := filepath.Join(tmpDir, "output")

	testData := []byte("Test data for write failure")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Create a read-only output directory to force write failure
	if err := os.Chmod(outputDir, 0555); err != nil {
		t.Fatalf("Failed to chmod output dir: %v", err)
	}
	defer os.Chmod(outputDir, 0755) // Cleanup

	handler := &Handler{}
	_, err := handler.Ingest(testFile, outputDir)
	if err == nil {
		t.Errorf("Expected error when unable to write blob")
	}
	if !strings.Contains(err.Error(), "failed to") {
		t.Errorf("Expected error message about write failure, got: %v", err)
	}
}

func TestEnumerate(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ot4i")

	testData := []byte("Test Olive Tree Bible file content")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	handler := &Handler{}
	result, err := handler.Enumerate(testFile)
	if err != nil {
		t.Fatalf("Enumerate() returned error: %v", err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.ot4i" {
		t.Errorf("Expected Path 'test.ot4i', got '%s'", entry.Path)
	}

	if entry.SizeBytes != int64(len(testData)) {
		t.Errorf("Expected SizeBytes %d, got %d", len(testData), entry.SizeBytes)
	}

	if entry.IsDir {
		t.Errorf("Expected IsDir=false")
	}

	if entry.Metadata["format"] != "OliveTree" {
		t.Errorf("Expected Metadata format 'OliveTree', got '%s'", entry.Metadata["format"])
	}

	if entry.Metadata["warning"] == "" {
		t.Errorf("Expected warning in metadata")
	}
}

func TestEnumerate_NonExistent(t *testing.T) {
	handler := &Handler{}
	_, err := handler.Enumerate("/nonexistent/file.ot4i")
	if err == nil {
		t.Errorf("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to stat") {
		t.Errorf("Expected error message about stat failure, got: %v", err)
	}
}

func TestExtractIR(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ot4i")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	handler := &Handler{}
	result, err := handler.ExtractIR(testFile, outputDir)

	// Should return an error
	if err == nil {
		t.Errorf("Expected error from ExtractIR")
	}

	if result != nil {
		t.Errorf("Expected nil result from ExtractIR")
	}

	expectedMsg := "extract-ir not supported"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain '%s', got: %v", expectedMsg, err)
	}

	if !strings.Contains(err.Error(), "proprietary/encrypted") {
		t.Errorf("Expected error message to mention proprietary/encrypted format, got: %v", err)
	}
}

func TestEmitNative(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "test.ir")
	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.EmitNative(irPath, outputDir)

	// Should return an error
	if err == nil {
		t.Errorf("Expected error from EmitNative")
	}

	if result != nil {
		t.Errorf("Expected nil result from EmitNative")
	}

	expectedMsg := "emit-native not supported"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain '%s', got: %v", expectedMsg, err)
	}

	if !strings.Contains(err.Error(), "proprietary/encrypted") {
		t.Errorf("Expected error message to mention proprietary/encrypted format, got: %v", err)
	}
}

func TestRegister(t *testing.T) {
	// Test that Register() doesn't panic
	// This is called in init(), but we can call it again to ensure it's safe
	Register()
}

func TestHandler_AllPDBKeywords(t *testing.T) {
	// Test all biblical keywords
	keywords := []string{"Bible", "Testament", "Scripture", "KJV", "NIV", "ESV", "NKJV"}

	for _, keyword := range keywords {
		t.Run(keyword, func(t *testing.T) {
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test.pdb")

			header := PDBHeader{
				Attributes: 0,
				Version:    1,
			}
			copy(header.Name[:], keyword+" Text")
			copy(header.Creator[:], "TEST")
			copy(header.Type[:], "DATA")
			header.NumRecords = 1

			file, err := os.Create(testFile)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			if err := binary.Write(file, binary.BigEndian, &header); err != nil {
				file.Close()
				t.Fatalf("Failed to write PDB header: %v", err)
			}
			file.Close()

			handler := &Handler{}
			result, err := handler.Detect(testFile)
			if err != nil {
				t.Fatalf("Detect() returned error: %v", err)
			}

			if !result.Detected {
				t.Errorf("Expected Detected=true for keyword '%s', got false. Reason: %s", keyword, result.Reason)
			}
		})
	}
}

func TestHandler_IntegrationWorkflow(t *testing.T) {
	// Test a complete workflow: Detect -> Ingest -> Enumerate
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "ESV.ot4i")
	outputDir := filepath.Join(tmpDir, "output")

	// Create test file with SQLite signature
	sqliteHeader := []byte("SQLite format 3\x00 some additional data here")
	if err := os.WriteFile(testFile, sqliteHeader, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	handler := &Handler{}

	// Step 1: Detect
	detectResult, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}
	if !detectResult.Detected {
		t.Fatalf("Expected file to be detected as Olive Tree format")
	}
	if detectResult.Format != "OliveTree" {
		t.Errorf("Expected Format 'OliveTree', got '%s'", detectResult.Format)
	}

	// Step 2: Ingest
	ingestResult, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Ingest() returned error: %v", err)
	}
	if ingestResult.ArtifactID != "ESV" {
		t.Errorf("Expected ArtifactID 'ESV', got '%s'", ingestResult.ArtifactID)
	}
	if ingestResult.BlobSHA256 == "" {
		t.Errorf("Expected non-empty BlobSHA256")
	}

	// Step 3: Enumerate
	enumResult, err := handler.Enumerate(testFile)
	if err != nil {
		t.Fatalf("Enumerate() returned error: %v", err)
	}
	if len(enumResult.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(enumResult.Entries))
	}
	if enumResult.Entries[0].Path != "ESV.ot4i" {
		t.Errorf("Expected Path 'ESV.ot4i', got '%s'", enumResult.Entries[0].Path)
	}

	// Step 4: Try ExtractIR (should fail)
	_, err = handler.ExtractIR(testFile, outputDir)
	if err == nil {
		t.Errorf("Expected ExtractIR to return error for proprietary format")
	}

	// Step 5: Try EmitNative (should fail)
	irPath := filepath.Join(tmpDir, "test.ir")
	_, err = handler.EmitNative(irPath, outputDir)
	if err == nil {
		t.Errorf("Expected EmitNative to return error for proprietary format")
	}
}

func TestDetect_PDB_CannotOpen(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.pdb")

	// Create file with correct extension
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Make file unreadable
	if err := os.Chmod(testFile, 0000); err != nil {
		t.Fatalf("Failed to chmod file: %v", err)
	}
	defer os.Chmod(testFile, 0644) // Cleanup

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}

	if result.Detected {
		t.Errorf("Expected Detected=false for unreadable PDB")
	}
}

func TestDetect_OT4I_CannotOpen(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ot4i")

	// Create file
	if err := os.WriteFile(testFile, []byte("test data here"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Make file unreadable
	if err := os.Chmod(testFile, 0000); err != nil {
		t.Fatalf("Failed to chmod file: %v", err)
	}
	defer os.Chmod(testFile, 0644) // Cleanup

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}

	if result.Detected {
		t.Errorf("Expected Detected=false for unreadable file")
	}

	if !strings.Contains(result.Reason, "cannot open") {
		t.Errorf("Expected reason to mention 'cannot open', got: %s", result.Reason)
	}
}
