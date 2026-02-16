//go:build !sdk

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestOSISDetect tests the detect command.
func TestOSISDetect(t *testing.T) {
	// Create a temporary OSIS file
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	osisContent := `<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="Test">
    <div type="book" osisID="Gen">
      <p><verse osisID="Gen.1.1"/>In the beginning.</p>
    </div>
  </osisText>
</osis>`

	osisPath := filepath.Join(tmpDir, "test.osis")
	if err := os.WriteFile(osisPath, []byte(osisContent), 0600); err != nil {
		t.Fatalf("failed to write OSIS file: %v", err)
	}

	// Test detect command
	req := IPCRequest{
		Command: "detect",
		Args:    map[string]interface{}{"path": osisPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] != true {
		t.Error("expected detected to be true")
	}
	if result["format"] != "OSIS" {
		t.Errorf("expected format OSIS, got %v", result["format"])
	}
}

// TestOSISDetectNonOSIS tests detect command on non-OSIS file.
func TestOSISDetectNonOSIS(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	txtPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtPath, []byte("Hello world"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	req := IPCRequest{
		Command: "detect",
		Args:    map[string]interface{}{"path": txtPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] == true {
		t.Error("expected detected to be false for non-OSIS file")
	}
}

// TestOSISExtractIR tests the extract-ir command.
func TestOSISExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	osisContent := `<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="TestBible" xml:lang="en">
    <header>
      <work osisWork="TestBible">
        <title>Test Bible</title>
        <language>en</language>
      </work>
    </header>
    <div type="book" osisID="Gen">
      <title>Genesis</title>
      <p><verse osisID="Gen.1.1"/>In the beginning.</p>
    </div>
  </osisText>
</osis>`

	osisPath := filepath.Join(tmpDir, "test.osis")
	if err := os.WriteFile(osisPath, []byte(osisContent), 0600); err != nil {
		t.Fatalf("failed to write OSIS file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := IPCRequest{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       osisPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	// Verify loss class is L0
	if result["loss_class"] != "L0" {
		t.Errorf("expected loss_class L0, got %v", result["loss_class"])
	}

	// Verify IR file was created
	irPath, ok := result["ir_path"].(string)
	if !ok {
		t.Fatal("ir_path is not a string")
	}

	irData, err := os.ReadFile(irPath)
	if err != nil {
		t.Fatalf("failed to read IR file: %v", err)
	}

	var corpus Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	if corpus.ID != "TestBible" {
		t.Errorf("expected ID TestBible, got %s", corpus.ID)
	}
	if corpus.Title != "Test Bible" {
		t.Errorf("expected title 'Test Bible', got %s", corpus.Title)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	if corpus.Documents[0].ID != "Gen" {
		t.Errorf("expected document ID Gen, got %s", corpus.Documents[0].ID)
	}
}

// TestOSISEmitNative tests the emit-native command.
func TestOSISEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create an IR file
	corpus := Corpus{
		ID:         "TestBible",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Language:   "en",
		Title:      "Test Bible",
		Documents: []*Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "In the beginning.",
					},
				},
			},
		},
	}

	irData, err := json.MarshalIndent(&corpus, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal IR: %v", err)
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	if err := os.WriteFile(irPath, irData, 0600); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := IPCRequest{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	// Verify format is OSIS
	if result["format"] != "OSIS" {
		t.Errorf("expected format OSIS, got %v", result["format"])
	}

	// Verify OSIS file was created
	osisPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	osisData, err := os.ReadFile(osisPath)
	if err != nil {
		t.Fatalf("failed to read OSIS file: %v", err)
	}

	// Verify it's valid OSIS
	if !bytes.Contains(osisData, []byte("<osis")) {
		t.Error("output does not contain <osis tag")
	}
	if !bytes.Contains(osisData, []byte("TestBible")) {
		t.Error("output does not contain TestBible")
	}
}

// TestOSISRoundTrip tests L0 lossless round-trip.
func TestOSISRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Original OSIS content
	originalContent := `<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="TestBible" xml:lang="en">
    <header>
      <work osisWork="TestBible">
        <title>Test Bible</title>
      </work>
    </header>
    <div type="book" osisID="Gen">
      <title>Genesis</title>
      <p><verse osisID="Gen.1.1"/>In the beginning.</p>
    </div>
  </osisText>
</osis>
`

	osisPath := filepath.Join(tmpDir, "original.osis")
	if err := os.WriteFile(osisPath, []byte(originalContent), 0600); err != nil {
		t.Fatalf("failed to write OSIS file: %v", err)
	}

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)

	// Extract IR
	extractReq := IPCRequest{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       osisPath,
			"output_dir": irDir,
		},
	}

	extractResp := executePlugin(t, &extractReq)
	if extractResp.Status != "ok" {
		t.Fatalf("extract-ir failed: %s", extractResp.Error)
	}

	extractResult := extractResp.Result.(map[string]interface{})
	irPath := extractResult["ir_path"].(string)

	// Emit native
	emitReq := IPCRequest{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irPath,
			"output_dir": outDir,
		},
	}

	emitResp := executePlugin(t, &emitReq)
	if emitResp.Status != "ok" {
		t.Fatalf("emit-native failed: %s", emitResp.Error)
	}

	emitResult := emitResp.Result.(map[string]interface{})
	outputPath := emitResult["output_path"].(string)

	// Compare original and output
	originalData, err := os.ReadFile(osisPath)
	if err != nil {
		t.Fatalf("failed to read original: %v", err)
	}

	outputData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	// Compute hashes
	originalHash := sha256.Sum256(originalData)
	outputHash := sha256.Sum256(outputData)

	if originalHash != outputHash {
		t.Errorf("L0 round-trip failed: hashes differ\noriginal: %s\noutput:   %s",
			hex.EncodeToString(originalHash[:]),
			hex.EncodeToString(outputHash[:]))
	}
}

// TestOSISIngest tests the ingest command.
func TestOSISIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	osisContent := `<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="TestBible">
    <div type="book" osisID="Gen">
      <p><verse osisID="Gen.1.1"/>In the beginning.</p>
    </div>
  </osisText>
</osis>`

	osisPath := filepath.Join(tmpDir, "test.osis")
	if err := os.WriteFile(osisPath, []byte(osisContent), 0600); err != nil {
		t.Fatalf("failed to write OSIS file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := IPCRequest{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       osisPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	// Verify artifact ID was derived from osisIDWork
	if result["artifact_id"] != "TestBible" {
		t.Errorf("expected artifact_id TestBible, got %v", result["artifact_id"])
	}

	// Verify blob was created
	blobHash, ok := result["blob_sha256"].(string)
	if !ok {
		t.Fatal("blob_sha256 is not a string")
	}

	blobPath := filepath.Join(outputDir, blobHash[:2], blobHash)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("blob file was not created")
	}
}

// executePlugin runs the plugin with a request and returns the response.
func executePlugin(t *testing.T, req *IPCRequest) *IPCResponse {
	t.Helper()

	// Find plugin binary
	pluginPath := "./format-osis"
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		// Try building if not exists
		buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
		if err := buildCmd.Run(); err != nil {
			t.Fatalf("failed to build plugin: %v", err)
		}
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	cmd := exec.Command(pluginPath)
	cmd.Stdin = bytes.NewReader(reqData)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check if it's an expected error (plugin returns error status)
		if stdout.Len() > 0 {
			var resp IPCResponse
			if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
				return &resp
			}
		}
		t.Fatalf("plugin execution failed: %v\nstderr: %s", err, stderr.String())
	}

	var resp IPCResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\noutput: %s", err, stdout.String())
	}

	return &resp
}
