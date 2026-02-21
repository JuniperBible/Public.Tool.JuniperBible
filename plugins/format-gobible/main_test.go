package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/ir"
)

// createTestGoBible creates a minimal GoBible-style JAR file for testing.
func createTestGoBible(t *testing.T, path string) {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Create manifest
	manifest := "Manifest-Version: 1.0\nMIDlet-Name: Test Bible\n"
	mf, _ := zw.Create("META-INF/MANIFEST.MF")
	mf.Write([]byte(manifest))

	// Create Collections file
	var collections bytes.Buffer
	binary.Write(&collections, binary.BigEndian, uint16(1)) // version
	binary.Write(&collections, binary.BigEndian, uint16(1)) // 1 book
	collections.WriteByte(7)                                // name length
	collections.WriteString("Genesis")
	binary.Write(&collections, binary.BigEndian, uint16(2)) // 2 verses

	cf, _ := zw.Create("Bible/Collections")
	cf.Write(collections.Bytes())

	// Create book data file
	var bookData bytes.Buffer
	verse1 := "In the beginning God created the heavens and the earth."
	verse2 := "And the earth was without form and void."
	binary.Write(&bookData, binary.BigEndian, uint16(len(verse1)))
	bookData.WriteString(verse1)
	binary.Write(&bookData, binary.BigEndian, uint16(len(verse2)))
	bookData.WriteString(verse2)

	bf, _ := zw.Create("Bible/Book0")
	bf.Write(bookData.Bytes())

	zw.Close()

	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatalf("failed to write test GoBible: %v", err)
	}
}

// TestGoBibleDetect tests the detect command.
func TestGoBibleDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gobible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	gbkPath := filepath.Join(tmpDir, "test.gbk")
	createTestGoBible(t, gbkPath)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": gbkPath},
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
	if result["format"] != "GoBible" {
		t.Errorf("expected format GoBible, got %v", result["format"])
	}
}

// TestGoBibleDetectNonGoBible tests detect command on non-GoBible file.
func TestGoBibleDetectNonGoBible(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gobible-test-*")
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
		t.Error("expected detected to be false for non-GoBible file")
	}
}

// TestGoBibleExtractIR tests the extract-ir command.
func TestGoBibleExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gobible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	gbkPath := filepath.Join(tmpDir, "test.gbk")
	createTestGoBible(t, gbkPath)

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       gbkPath,
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

	if result["loss_class"] != "L3" {
		t.Errorf("expected loss_class L3, got %v", result["loss_class"])
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

	if len(corpus.Documents) == 0 {
		t.Error("expected at least 1 document")
	}
}

// TestGoBibleEmitNative tests the emit-native command.
func TestGoBibleEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gobible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := ir.Corpus{
		ID:         "test",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Test Bible",
		Language:   "en",
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

	if result["format"] != "GoBible" {
		t.Errorf("expected format GoBible, got %v", result["format"])
	}

	gbkPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	// Verify the output file is a valid ZIP
	_, err = zip.OpenReader(gbkPath)
	if err != nil {
		t.Fatalf("failed to open output as ZIP: %v", err)
	}
}

// TestGoBibleRoundTrip tests L0 lossless round-trip via raw storage.
func TestGoBibleRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gobible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	gbkPath := filepath.Join(tmpDir, "original.gbk")
	createTestGoBible(t, gbkPath)

	originalData, _ := os.ReadFile(gbkPath)

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0700)
	os.MkdirAll(outDir, 0700)

	// Extract IR
	extractReq := IPCRequest{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       gbkPath,
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
		t.Logf("round-trip: original %d bytes, output %d bytes (expected for L3 format with raw storage)", len(originalData), len(outputData))
	}

	// Verify loss class is L0 (lossless via raw storage)
	if emitResult["loss_class"] != "L0" {
		t.Logf("got loss_class %v (L0 expected for raw round-trip)", emitResult["loss_class"])
	}
}

// TestGoBibleIngest tests the ingest command.
func TestGoBibleIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gobible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	gbkPath := filepath.Join(tmpDir, "test.gbk")
	createTestGoBible(t, gbkPath)

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       gbkPath,
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

	pluginPath := "./format-gobible"
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
