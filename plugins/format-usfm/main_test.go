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

// TestUSFMDetect tests the detect command.
func TestUSFMDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usfmContent := `\id GEN
\h Genesis
\c 1
\v 1 In the beginning.
`

	usfmPath := filepath.Join(tmpDir, "test.usfm")
	if err := os.WriteFile(usfmPath, []byte(usfmContent), 0600); err != nil {
		t.Fatalf("failed to write USFM file: %v", err)
	}

	req := IPCRequest{
		Command: "detect",
		Args:    map[string]interface{}{"path": usfmPath},
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
	if result["format"] != "USFM" {
		t.Errorf("expected format USFM, got %v", result["format"])
	}
}

// TestUSFMDetectNonUSFM tests detect command on non-USFM file.
func TestUSFMDetectNonUSFM(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
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
		t.Error("expected detected to be false for non-USFM file")
	}
}

// TestUSFMExtractIR tests the extract-ir command.
func TestUSFMExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usfmContent := `\id GEN
\h Genesis
\mt Genesis
\c 1
\v 1 In the beginning God created.
\v 2 And the earth was void.
`

	usfmPath := filepath.Join(tmpDir, "test.usfm")
	if err := os.WriteFile(usfmPath, []byte(usfmContent), 0600); err != nil {
		t.Fatalf("failed to write USFM file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := IPCRequest{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       usfmPath,
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

	if result["loss_class"] != "L0" {
		t.Errorf("expected loss_class L0, got %v", result["loss_class"])
	}

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

	if corpus.ID != "GEN" {
		t.Errorf("expected ID GEN, got %s", corpus.ID)
	}
	if corpus.Title != "Genesis" {
		t.Errorf("expected title Genesis, got %s", corpus.Title)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("expected 2 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

// TestUSFMEmitNative tests the emit-native command.
func TestUSFMEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := Corpus{
		ID:         "GEN",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Genesis",
		Documents: []*Document{
			{
				ID:    "GEN",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "In the beginning.",
						Anchors: []*Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*Span{
									{
										ID:            "s-GEN.1.1",
										Type:          "VERSE",
										StartAnchorID: "a-1-0",
										Ref: &Ref{
											Book:    "GEN",
											Chapter: 1,
											Verse:   1,
											OSISID:  "GEN.1.1",
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

	if result["format"] != "USFM" {
		t.Errorf("expected format USFM, got %v", result["format"])
	}

	usfmPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	usfmData, err := os.ReadFile(usfmPath)
	if err != nil {
		t.Fatalf("failed to read USFM file: %v", err)
	}

	if !bytes.Contains(usfmData, []byte("\\id GEN")) {
		t.Error("output does not contain \\id marker")
	}
	if !bytes.Contains(usfmData, []byte("\\v 1")) {
		t.Error("output does not contain verse marker")
	}
}

// TestUSFMRoundTrip tests L0 lossless round-trip.
func TestUSFMRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalContent := `\id GEN
\h Genesis
\toc1 Genesis
\mt Genesis
\c 1
\v 1 In the beginning God created the heaven and the earth.
\v 2 And the earth was without form, and void.
\c 2
\v 1 Thus the heavens and the earth were finished.
`

	usfmPath := filepath.Join(tmpDir, "original.usfm")
	if err := os.WriteFile(usfmPath, []byte(originalContent), 0600); err != nil {
		t.Fatalf("failed to write USFM file: %v", err)
	}

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)

	// Extract IR
	extractReq := IPCRequest{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       usfmPath,
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
	originalData, err := os.ReadFile(usfmPath)
	if err != nil {
		t.Fatalf("failed to read original: %v", err)
	}

	outputData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	originalHash := sha256.Sum256(originalData)
	outputHash := sha256.Sum256(outputData)

	if originalHash != outputHash {
		t.Errorf("L0 round-trip failed: hashes differ\noriginal: %s\noutput:   %s",
			hex.EncodeToString(originalHash[:]),
			hex.EncodeToString(outputHash[:]))
	}
}

// TestUSFMIngest tests the ingest command.
func TestUSFMIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usfmContent := `\id GEN
\c 1
\v 1 In the beginning.
`

	usfmPath := filepath.Join(tmpDir, "test.usfm")
	if err := os.WriteFile(usfmPath, []byte(usfmContent), 0600); err != nil {
		t.Fatalf("failed to write USFM file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := IPCRequest{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       usfmPath,
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

	if result["artifact_id"] != "GEN" {
		t.Errorf("expected artifact_id GEN, got %v", result["artifact_id"])
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
func executePlugin(t *testing.T, req *IPCRequest) *IPCResponse {
	t.Helper()

	pluginPath := "./format-usfm"
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
