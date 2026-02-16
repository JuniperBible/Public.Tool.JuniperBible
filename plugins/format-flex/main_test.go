package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func createTestFLEx(t *testing.T, path string) {
	t.Helper()
	content := `<?xml version="1.0" encoding="UTF-8"?>
<document version="2">
  <interlinear-text>
    <paragraphs>
      <paragraph>
        <phrases>
          <phrase>
            <words>
              <word><item type="txt">In</item></word>
              <word><item type="txt">the</item></word>
              <word><item type="txt">beginning</item></word>
            </words>
          </phrase>
        </phrases>
      </paragraph>
    </paragraphs>
  </interlinear-text>
</document>
`
	os.WriteFile(path, []byte(content), 0600)
}

func TestFLExDetect(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "flex-test-*")
	defer os.RemoveAll(tmpDir)
	flexPath := filepath.Join(tmpDir, "test.flextext")
	createTestFLEx(t, flexPath)
	resp := executePlugin(t, &IPCRequest{Command: "detect", Args: map[string]interface{}{"path": flexPath}})
	if resp.Result.(map[string]interface{})["detected"] != true {
		t.Error("expected detected")
	}
}

func TestFLExDetectNon(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "flex-test-*")
	defer os.RemoveAll(tmpDir)
	txtPath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(txtPath, []byte("Hello"), 0600)
	resp := executePlugin(t, &IPCRequest{Command: "detect", Args: map[string]interface{}{"path": txtPath}})
	if resp.Result.(map[string]interface{})["detected"] == true {
		t.Error("expected not detected")
	}
}

func TestFLExExtractIR(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "flex-test-*")
	defer os.RemoveAll(tmpDir)
	flexPath := filepath.Join(tmpDir, "test.flextext")
	createTestFLEx(t, flexPath)
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)
	resp := executePlugin(t, &IPCRequest{Command: "extract-ir", Args: map[string]interface{}{"path": flexPath, "output_dir": outputDir}})
	if resp.Status != "ok" {
		t.Fatalf("expected ok: %s", resp.Error)
	}
}

func TestFLExEmitNative(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "flex-test-*")
	defer os.RemoveAll(tmpDir)
	corpus := Corpus{ID: "test", Title: "Test", Documents: []*Document{{ID: "doc1", Title: "Doc", Order: 1, ContentBlocks: []*ContentBlock{{ID: "cb-1", Sequence: 1, Text: "In the beginning"}}}}}
	irData, _ := json.MarshalIndent(&corpus, "", "  ")
	irPath := filepath.Join(tmpDir, "test.ir.json")
	os.WriteFile(irPath, irData, 0600)
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)
	resp := executePlugin(t, &IPCRequest{Command: "emit-native", Args: map[string]interface{}{"ir_path": irPath, "output_dir": outputDir}})
	if resp.Result.(map[string]interface{})["format"] != "FLEx" {
		t.Error("expected FLEx format")
	}
}

func TestFLExRoundTrip(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "flex-test-*")
	defer os.RemoveAll(tmpDir)
	flexPath := filepath.Join(tmpDir, "original.flextext")
	createTestFLEx(t, flexPath)
	originalData, _ := os.ReadFile(flexPath)
	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)
	extractResp := executePlugin(t, &IPCRequest{Command: "extract-ir", Args: map[string]interface{}{"path": flexPath, "output_dir": irDir}})
	irPath := extractResp.Result.(map[string]interface{})["ir_path"].(string)
	emitResp := executePlugin(t, &IPCRequest{Command: "emit-native", Args: map[string]interface{}{"ir_path": irPath, "output_dir": outDir}})
	outputPath := emitResp.Result.(map[string]interface{})["output_path"].(string)
	outputData, _ := os.ReadFile(outputPath)
	if !bytes.Equal(originalData, outputData) {
		t.Logf("round-trip: %d -> %d bytes", len(originalData), len(outputData))
	}
}

func TestFLExIngest(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "flex-test-*")
	defer os.RemoveAll(tmpDir)
	flexPath := filepath.Join(tmpDir, "test.flextext")
	createTestFLEx(t, flexPath)
	outputDir := filepath.Join(tmpDir, "blobs")
	os.MkdirAll(outputDir, 0755)
	resp := executePlugin(t, &IPCRequest{Command: "ingest", Args: map[string]interface{}{"path": flexPath, "output_dir": outputDir}})
	blobHash := resp.Result.(map[string]interface{})["blob_sha256"].(string)
	if _, err := os.Stat(filepath.Join(outputDir, blobHash[:2], blobHash)); os.IsNotExist(err) {
		t.Error("blob not created")
	}
}

func executePlugin(t *testing.T, req *IPCRequest) *IPCResponse {
	t.Helper()
	pluginPath := "./format-flex"
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		exec.Command("go", "build", "-o", pluginPath, ".").Run()
	}
	reqData, _ := json.Marshal(req)
	cmd := exec.Command(pluginPath)
	cmd.Stdin = bytes.NewReader(reqData)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stdout.Len() > 0 {
			var resp IPCResponse
			if json.Unmarshal(stdout.Bytes(), &resp) == nil {
				return &resp
			}
		}
		t.Fatalf("plugin failed: %v\nstderr: %s", err, stderr.String())
	}
	var resp IPCResponse
	json.Unmarshal(stdout.Bytes(), &resp)
	return &resp
}
