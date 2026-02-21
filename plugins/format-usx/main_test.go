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

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/ir"
)

// TestUSXDetect tests the detect command.
func TestUSXDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usxContent := `<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
  <chapter number="1" style="c" sid="GEN.1"/>
  <para style="p">
    <verse number="1" style="v" sid="GEN.1.1"/>In the beginning.<verse eid="GEN.1.1"/>
  </para>
</usx>
`

	usxPath := filepath.Join(tmpDir, "test.usx")
	if err := os.WriteFile(usxPath, []byte(usxContent), 0600); err != nil {
		t.Fatalf("failed to write USX file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": usxPath},
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
	if result["format"] != "USX" {
		t.Errorf("expected format USX, got %v", result["format"])
	}
}

// TestUSXDetectNonUSX tests detect command on non-USX file.
func TestUSXDetectNonUSX(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
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
		t.Error("expected detected to be false for non-USX file")
	}
}

// TestUSXExtractIR tests the extract-ir command.
func TestUSXExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usxContent := `<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
  <chapter number="1" style="c" sid="GEN.1"/>
  <para style="p">
    <verse number="1" style="v" sid="GEN.1.1"/>In the beginning God created.<verse eid="GEN.1.1"/>
    <verse number="2" style="v" sid="GEN.1.2"/>And the earth was void.<verse eid="GEN.1.2"/>
  </para>
</usx>
`

	usxPath := filepath.Join(tmpDir, "test.usx")
	if err := os.WriteFile(usxPath, []byte(usxContent), 0600); err != nil {
		t.Fatalf("failed to write USX file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       usxPath,
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

	var corpus ir.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	if corpus.ID != "GEN" {
		t.Errorf("expected ID GEN, got %s", corpus.ID)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("expected 2 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

// TestUSXEmitNative tests the emit-native command.
func TestUSXEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := ir.Corpus{
		ID:         "GEN",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Genesis",
		Documents: []*ir.Document{
			{
				ID:    "GEN",
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
										ID:            "s-GEN.1.1",
										Type:          "VERSE",
										StartAnchorID: "a-1-0",
										Ref: &ir.Ref{
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

	if result["format"] != "USX" {
		t.Errorf("expected format USX, got %v", result["format"])
	}

	usxPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	usxData, err := os.ReadFile(usxPath)
	if err != nil {
		t.Fatalf("failed to read USX file: %v", err)
	}

	if !bytes.Contains(usxData, []byte("<usx")) {
		t.Error("output does not contain <usx> element")
	}
	if !bytes.Contains(usxData, []byte(`code="GEN"`)) {
		t.Error("output does not contain book code")
	}
	if !bytes.Contains(usxData, []byte("In the beginning.")) {
		t.Error("output does not contain verse text")
	}
}

// TestUSXRoundTrip tests L0 lossless round-trip.
func TestUSXRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalContent := `<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
  <chapter number="1" style="c" sid="GEN.1"/>
  <para style="p">
    <verse number="1" style="v" sid="GEN.1.1"/>In the beginning God created the heaven and the earth.<verse eid="GEN.1.1"/>
    <verse number="2" style="v" sid="GEN.1.2"/>And the earth was without form, and void.<verse eid="GEN.1.2"/>
  </para>
  <chapter number="2" style="c" sid="GEN.2"/>
  <para style="p">
    <verse number="1" style="v" sid="GEN.2.1"/>Thus the heavens and the earth were finished.<verse eid="GEN.2.1"/>
  </para>
</usx>
`

	usxPath := filepath.Join(tmpDir, "original.usx")
	if err := os.WriteFile(usxPath, []byte(originalContent), 0600); err != nil {
		t.Fatalf("failed to write USX file: %v", err)
	}

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0700)
	os.MkdirAll(outDir, 0700)

	// Extract IR
	extractReq := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       usxPath,
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

	// Compare original and output
	originalData, err := os.ReadFile(usxPath)
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

// TestUSXIngest tests the ingest command.
func TestUSXIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usxContent := `<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
  <chapter number="1" style="c" sid="GEN.1"/>
  <para style="p">
    <verse number="1" style="v" sid="GEN.1.1"/>In the beginning.<verse eid="GEN.1.1"/>
  </para>
</usx>
`

	usxPath := filepath.Join(tmpDir, "test.usx")
	if err := os.WriteFile(usxPath, []byte(usxContent), 0600); err != nil {
		t.Fatalf("failed to write USX file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       usxPath,
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

	// artifact_id is derived from filename (test.usx -> test), not from USX book code
	if result["artifact_id"] != "test" {
		t.Errorf("expected artifact_id test, got %v", result["artifact_id"])
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

	pluginPath := "./format-usx"
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
