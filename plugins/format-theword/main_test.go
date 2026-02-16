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

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// TestTheWordDetect tests the detect command.
func TestTheWordDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "theword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a simple TheWord-style file
	content := `In the beginning God created the heaven and the earth.
And the earth was without form, and void.
And God said, Let there be light.
And God saw the light, that it was good.
And God called the light Day.
And the evening and the morning were the first day.
And God said, Let there be a firmament.
And God made the firmament.
And God called the firmament Heaven.
And the evening and the morning were the second day.
And God said, Let the waters be gathered.
`

	twPath := filepath.Join(tmpDir, "test.ont")
	if err := os.WriteFile(twPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write TheWord file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": twPath},
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
	if result["format"] != "TheWord" {
		t.Errorf("expected format TheWord, got %v", result["format"])
	}
}

// TestTheWordDetectNonTheWord tests detect command on non-TheWord file.
func TestTheWordDetectNonTheWord(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "theword-test-*")
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
		t.Error("expected detected to be false for non-TheWord file")
	}
}

// TestTheWordExtractIR tests the extract-ir command.
func TestTheWordExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "theword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := `In the beginning God created the heaven and the earth.
And the earth was without form, and void.
And God said, Let there be light.
`

	twPath := filepath.Join(tmpDir, "test.nt")
	if err := os.WriteFile(twPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write TheWord file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       twPath,
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

	if corpus.ID != "test" {
		t.Errorf("expected ID test, got %s", corpus.ID)
	}
	if len(corpus.Documents) < 1 {
		t.Fatal("expected at least 1 document")
	}
}

// TestTheWordEmitNative tests the emit-native command.
func TestTheWordEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "theword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := ir.Corpus{
		ID:         "test",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Test Bible",
		Attributes: map[string]string{
			"_theword_ext": ".nt",
		},
		Documents: []*ir.Document{
			{
				ID:    "Matt",
				Title: "Matthew",
				Order: 1,
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "The book of the generation of Jesus Christ.",
						Anchors: []*ir.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*ir.Span{
									{
										ID:            "s-Matt.1.1",
										Type:          "VERSE",
										StartAnchorID: "a-1-0",
										Ref: &ir.Ref{
											Book:    "Matt",
											Chapter: 1,
											Verse:   1,
											OSISID:  "Matt.1.1",
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

	if result["format"] != "TheWord" {
		t.Errorf("expected format TheWord, got %v", result["format"])
	}

	twPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	twData, err := os.ReadFile(twPath)
	if err != nil {
		t.Fatalf("failed to read TheWord file: %v", err)
	}

	if !bytes.Contains(twData, []byte("Jesus Christ")) {
		t.Error("output does not contain expected text")
	}
}

// TestTheWordRoundTrip tests L0 lossless round-trip.
func TestTheWordRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "theword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalContent := `In the beginning God created the heaven and the earth.
And the earth was without form, and void.
And God said, Let there be light.
And God saw the light, that it was good.
And God called the light Day.
And the evening and the morning were the first day.
And God said, Let there be a firmament.
And God made the firmament.
And God called the firmament Heaven.
And the evening and the morning were the second day.
And God said, Let the waters be gathered.
`

	twPath := filepath.Join(tmpDir, "original.ont")
	if err := os.WriteFile(twPath, []byte(originalContent), 0600); err != nil {
		t.Fatalf("failed to write TheWord file: %v", err)
	}

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)

	// Extract IR
	extractReq := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       twPath,
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
	originalData, err := os.ReadFile(twPath)
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

// TestTheWordIngest tests the ingest command.
func TestTheWordIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "theword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := `In the beginning.
And the earth was void.
`

	twPath := filepath.Join(tmpDir, "test.twm")
	if err := os.WriteFile(twPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write TheWord file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       twPath,
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

	pluginPath := "./format-theword"
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
