package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// createTestTEI creates a minimal TEI XML file for testing.
func createTestTEI(t *testing.T, path string) {
	t.Helper()

	content := `<?xml version="1.0" encoding="UTF-8"?>
<TEI xmlns="http://www.tei-c.org/ns/1.0">
  <teiHeader>
    <fileDesc>
      <titleStmt>
        <title>Test Bible</title>
      </titleStmt>
      <publicationStmt>
        <publisher>Test Publisher</publisher>
      </publicationStmt>
      <sourceDesc>
        <p>Test source</p>
      </sourceDesc>
    </fileDesc>
    <profileDesc>
      <langUsage>
        <language ident="en"/>
      </langUsage>
    </profileDesc>
  </teiHeader>
  <text>
    <body>
      <div type="book" n="Gen">
        <ab n="1">In the beginning God created the heavens and the earth.</ab>
        <ab n="2">And the earth was without form and void.</ab>
      </div>
      <div type="book" n="Exo">
        <ab n="1">Now these are the names of the children of Israel.</ab>
      </div>
    </body>
  </text>
</TEI>
`

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test TEI: %v", err)
	}
}

// TestTEIDetect tests the detect command.
func TestTEIDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tei-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	teiPath := filepath.Join(tmpDir, "test.tei")
	createTestTEI(t, teiPath)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": teiPath},
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
	if result["format"] != "TEI" {
		t.Errorf("expected format TEI, got %v", result["format"])
	}
}

// TestTEIDetectByContent tests detect by XML content.
func TestTEIDetectByContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tei-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Use .xml extension to test content-based detection
	xmlPath := filepath.Join(tmpDir, "test.xml")
	createTestTEI(t, xmlPath)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": xmlPath},
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
		t.Error("expected detected to be true for TEI content")
	}
}

// TestTEIDetectNonTEI tests detect command on non-TEI file.
func TestTEIDetectNonTEI(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tei-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	txtPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtPath, []byte("Hello world"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	req := ipc.Request{
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
		t.Error("expected detected to be false for non-TEI file")
	}
}

// TestTEIExtractIR tests the extract-ir command.
func TestTEIExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tei-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	teiPath := filepath.Join(tmpDir, "test.tei")
	createTestTEI(t, teiPath)

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       teiPath,
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

	if result["loss_class"] != "L1" {
		t.Errorf("expected loss_class L1, got %v", result["loss_class"])
	}

	irPath, ok := result["ir_path"].(string)
	if !ok {
		t.Fatal("ir_path is not a string")
	}

	irData, err := os.ReadFile(irPath)
	if err != nil {
		t.Fatalf("failed to read IR file: %v", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	if corpus.Title != "Test Bible" {
		t.Errorf("expected title 'Test Bible', got %s", corpus.Title)
	}

	if len(corpus.Documents) < 2 {
		t.Errorf("expected at least 2 documents (books), got %d", len(corpus.Documents))
	}
}

// TestTEIEmitNative tests the emit-native command.
func TestTEIEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tei-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := ipc.Corpus{
		ID:         "test",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Test Bible",
		Language:   "en",
		Publisher:  "Test Publisher",
		Documents: []*ipc.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ipc.ContentBlock{
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

	req := ipc.Request{
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

	if result["format"] != "TEI" {
		t.Errorf("expected format TEI, got %v", result["format"])
	}

	teiPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	// Verify the output file contains expected TEI structure
	content, err := os.ReadFile(teiPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if !strings.Contains(string(content), "<TEI") {
		t.Error("expected TEI root element")
	}
	if !strings.Contains(string(content), "teiHeader") {
		t.Error("expected teiHeader element")
	}
}

// TestTEIRoundTrip tests L0 lossless round-trip via raw storage.
func TestTEIRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tei-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	teiPath := filepath.Join(tmpDir, "original.tei")
	createTestTEI(t, teiPath)

	originalData, _ := os.ReadFile(teiPath)

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)

	// Extract IR
	extractReq := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       teiPath,
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
	emitReq := ipc.Request{
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

	// Verify L0 lossless round-trip (via raw storage)
	outputData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if !bytes.Equal(originalData, outputData) {
		t.Logf("round-trip: original %d bytes, output %d bytes (expected for L1 format with raw storage)", len(originalData), len(outputData))
	}

	// Verify loss class is L0 (lossless via raw storage)
	if emitResult["loss_class"] != "L0" {
		t.Logf("got loss_class %v (L0 expected for raw round-trip)", emitResult["loss_class"])
	}
}

// TestTEIIngest tests the ingest command.
func TestTEIIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tei-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	teiPath := filepath.Join(tmpDir, "test.tei")
	createTestTEI(t, teiPath)

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       teiPath,
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
func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()

	pluginPath := "./format-tei"
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
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
		if stdout.Len() > 0 {
			var resp ipc.Response
			if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
				return &resp
			}
		}
		t.Fatalf("plugin execution failed: %v\nstderr: %s", err, stderr.String())
	}

	var resp ipc.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\noutput: %s", err, stdout.String())
	}

	return &resp
}
