package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/ir"
)

// createTestDBL creates a minimal DBL bundle for testing.
func createTestDBL(t *testing.T, path string) {
	t.Helper()

	zipFile, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create DBL file: %v", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Write metadata.xml
	metaWriter, _ := zipWriter.Create("metadata.xml")
	metaWriter.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<DBLMetadata id="TEST" revision="1" type="text" typeVersion="3.0">
  <identification>
    <name>Test Bible</name>
    <nameLocal>Test Bible</nameLocal>
    <description>A test Bible</description>
    <scope>Bible</scope>
  </identification>
  <language>
    <iso>eng</iso>
    <name>English</name>
    <script>Latn</script>
  </language>
  <copyright>
    <statement>Public Domain</statement>
  </copyright>
</DBLMetadata>`))

	// Write USX file
	usxWriter, _ := zipWriter.Create("release/GEN.usx")
	usxWriter.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
  <chapter number="1" style="c"/>
  <para style="p"><verse number="1" style="v"/>In the beginning God created the heavens and the earth.</para>
  <para style="p"><verse number="2" style="v"/>And the earth was without form and void.</para>
</usx>`))
}

// TestDBLDetect tests the detect command.
func TestDBLDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dbl-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dblPath := filepath.Join(tmpDir, "test.zip")
	createTestDBL(t, dblPath)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": dblPath},
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
	if result["format"] != "DBL" {
		t.Errorf("expected format DBL, got %v", result["format"])
	}
}

// TestDBLDetectNonDBL tests detect command on non-DBL file.
func TestDBLDetectNonDBL(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dbl-test-*")
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
		t.Error("expected detected to be false for non-DBL file")
	}
}

// TestDBLExtractIR tests the extract-ir command.
func TestDBLExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dbl-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dblPath := filepath.Join(tmpDir, "test.zip")
	createTestDBL(t, dblPath)

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       dblPath,
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

	var corpus ir.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	if corpus.Title != "Test Bible" {
		t.Errorf("expected title 'Test Bible', got %s", corpus.Title)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("expected 2 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

// TestDBLEmitNative tests the emit-native command.
func TestDBLEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dbl-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := ir.Corpus{
		ID:          "test",
		Version:     "1.0.0",
		ModuleType:  "BIBLE",
		Title:       "Test Bible",
		Description: "A test Bible",
		Language:    "en",
		Documents: []*ir.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "In the beginning.",
						Anchors: []*ir.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*ir.Span{
									{
										ID:            "s-Gen.1.1",
										Type:          "VERSE",
										StartAnchorID: "a-1-0",
										Ref: &ir.Ref{
											Book:    "Gen",
											Chapter: 1,
											Verse:   1,
											OSISID:  "Gen.1.1",
										},
									},
								},
							},
						},
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
	if err := os.MkdirAll(outputDir, 0700); err != nil {
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

	if result["format"] != "DBL" {
		t.Errorf("expected format DBL, got %v", result["format"])
	}

	dblPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	// Verify the output file is a valid zip
	zipReader, err := zip.OpenReader(dblPath)
	if err != nil {
		t.Fatalf("failed to open output as zip: %v", err)
	}
	defer zipReader.Close()

	// Check for required files
	hasMetadata := false
	for _, f := range zipReader.File {
		if f.Name == "metadata.xml" {
			hasMetadata = true
		}
	}

	if !hasMetadata {
		t.Error("expected metadata.xml in DBL bundle")
	}
}

// TestDBLRoundTrip tests L0 lossless round-trip via raw storage.
func TestDBLRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dbl-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dblPath := filepath.Join(tmpDir, "original.zip")
	createTestDBL(t, dblPath)

	originalData, _ := os.ReadFile(dblPath)

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0700)
	os.MkdirAll(outDir, 0700)

	// Extract IR
	extractReq := IPCRequest{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       dblPath,
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

	// Verify L0 lossless round-trip (via raw storage)
	outputData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if !bytes.Equal(originalData, outputData) {
		t.Errorf("round-trip mismatch: original %d bytes, output %d bytes", len(originalData), len(outputData))
	}

	// Verify loss class is L0 (lossless via raw storage)
	if emitResult["loss_class"] != "L0" {
		t.Errorf("expected loss_class L0 for round-trip, got %v", emitResult["loss_class"])
	}
}

// TestDBLIngest tests the ingest command.
func TestDBLIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dbl-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dblPath := filepath.Join(tmpDir, "test.zip")
	createTestDBL(t, dblPath)

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       dblPath,
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

// TestDBLEnumerate tests the enumerate command.
func TestDBLEnumerate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dbl-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dblPath := filepath.Join(tmpDir, "test.zip")
	createTestDBL(t, dblPath)

	req := ipc.Request{
		Command: "enumerate",
		Args:    map[string]interface{}{"path": dblPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	entries, ok := result["entries"].([]interface{})
	if !ok {
		t.Fatal("entries is not an array")
	}

	if len(entries) < 2 {
		t.Errorf("expected at least 2 entries, got %d", len(entries))
	}

	// Check for metadata.xml
	hasMetadata := false
	for _, entry := range entries {
		e := entry.(map[string]interface{})
		if strings.Contains(e["path"].(string), "metadata.xml") {
			hasMetadata = true
		}
	}
	if !hasMetadata {
		t.Error("expected metadata.xml in enumeration")
	}
}

// executePlugin runs the plugin with a request and returns the response.
func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()

	pluginPath := "./format-dbl"
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
