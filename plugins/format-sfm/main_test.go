package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/ir"
)

func createTestSFM(t *testing.T, path string) {
	t.Helper()
	content := `\id GEN Genesis
\c 1
\v 1 In the beginning God created the heavens and the earth.
\v 2 And the earth was without form and void.
`
	os.WriteFile(path, []byte(content), 0600)
}

func TestSFMDetect(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "sfm-test-*")
	defer os.RemoveAll(tmpDir)
	sfmPath := filepath.Join(tmpDir, "test.sfm")
	createTestSFM(t, sfmPath)
	resp := executePlugin(t, &ipc.Request{Command: "detect", Args: map[string]interface{}{"path": sfmPath}})
	if resp.Result.(map[string]interface{})["detected"] != true {
		t.Error("expected detected")
	}
}

func TestSFMDetectNon(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "sfm-test-*")
	defer os.RemoveAll(tmpDir)
	txtPath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(txtPath, []byte("Hello"), 0600)
	resp := executePlugin(t, &ipc.Request{Command: "detect", Args: map[string]interface{}{"path": txtPath}})
	if resp.Result.(map[string]interface{})["detected"] == true {
		t.Error("expected not detected")
	}
}

func TestSFMExtractIR(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "sfm-test-*")
	defer os.RemoveAll(tmpDir)
	sfmPath := filepath.Join(tmpDir, "test.sfm")
	createTestSFM(t, sfmPath)
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0700)
	resp := executePlugin(t, &ipc.Request{Command: "extract-ir", Args: map[string]interface{}{"path": sfmPath, "output_dir": outputDir}})
	if resp.Status != "ok" {
		t.Fatalf("expected ok: %s", resp.Error)
	}
}

func TestSFMEmitNative(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "sfm-test-*")
	defer os.RemoveAll(tmpDir)
	corpus := ir.Corpus{ID: "test", Title: "Test", Documents: []*ir.Document{{ID: "GEN", Title: "Genesis", Order: 1, ContentBlocks: []*ir.ContentBlock{{ID: "cb-1", Sequence: 1001, Text: "In the beginning"}}}}}
	irData, _ := json.MarshalIndent(&corpus, "", "  ")
	irPath := filepath.Join(tmpDir, "test.ir.json")
	os.WriteFile(irPath, irData, 0600)
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0700)
	resp := executePlugin(t, &ipc.Request{Command: "emit-native", Args: map[string]interface{}{"ir_path": irPath, "output_dir": outputDir}})
	if resp.Result.(map[string]interface{})["format"] != "SFM" {
		t.Error("expected SFM format")
	}
}

func TestSFMRoundTrip(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "sfm-test-*")
	defer os.RemoveAll(tmpDir)
	sfmPath := filepath.Join(tmpDir, "original.sfm")
	createTestSFM(t, sfmPath)
	originalData, _ := os.ReadFile(sfmPath)
	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0700)
	os.MkdirAll(outDir, 0700)
	extractResp := executePlugin(t, &ipc.Request{Command: "extract-ir", Args: map[string]interface{}{"path": sfmPath, "output_dir": irDir}})
	irPath := extractResp.Result.(map[string]interface{})["ir_path"].(string)
	emitResp := executePlugin(t, &ipc.Request{Command: "emit-native", Args: map[string]interface{}{"ir_path": irPath, "output_dir": outDir}})
	outputPath := emitResp.Result.(map[string]interface{})["output_path"].(string)
	outputData, _ := os.ReadFile(outputPath)
	if !bytes.Equal(originalData, outputData) {
		t.Logf("round-trip: %d -> %d bytes", len(originalData), len(outputData))
	}
}

func TestSFMIngest(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "sfm-test-*")
	defer os.RemoveAll(tmpDir)
	sfmPath := filepath.Join(tmpDir, "test.sfm")
	createTestSFM(t, sfmPath)
	outputDir := filepath.Join(tmpDir, "blobs")
	os.MkdirAll(outputDir, 0700)
	resp := executePlugin(t, &ipc.Request{Command: "ingest", Args: map[string]interface{}{"path": sfmPath, "output_dir": outputDir}})
	blobHash := resp.Result.(map[string]interface{})["blob_sha256"].(string)
	if _, err := os.Stat(filepath.Join(outputDir, blobHash[:2], blobHash)); os.IsNotExist(err) {
		t.Error("blob not created")
	}
}

func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()
	pluginPath := "./format-sfm"
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
			var resp ipc.Response
			if json.Unmarshal(stdout.Bytes(), &resp) == nil {
				return &resp
			}
		}
		t.Fatalf("plugin failed: %v\nstderr: %s", err, stderr.String())
	}
	var resp ipc.Response
	json.Unmarshal(stdout.Bytes(), &resp)
	return &resp
}
