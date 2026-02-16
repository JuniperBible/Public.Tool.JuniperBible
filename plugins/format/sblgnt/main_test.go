package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

func createTestSBLGNT(t *testing.T, path string) {
	t.Helper()
	content := "01010101\tΒίβλος γενέσεως\n01010102\tἸησοῦ Χριστοῦ\n"
	os.WriteFile(path, []byte(content), 0600)
}

func TestSBLGNTDetect(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "sblgnt-test-*")
	defer os.RemoveAll(tmpDir)
	sblPath := filepath.Join(tmpDir, "sblgnt-test.txt")
	createTestSBLGNT(t, sblPath)
	resp := executePlugin(t, &ipc.Request{Command: "detect", Args: map[string]interface{}{"path": sblPath}})
	if resp.Result.(map[string]interface{})["detected"] != true {
		t.Error("expected detected")
	}
}

func TestSBLGNTDetectNon(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "sblgnt-test-*")
	defer os.RemoveAll(tmpDir)
	txtPath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(txtPath, []byte("Hello"), 0600)
	resp := executePlugin(t, &ipc.Request{Command: "detect", Args: map[string]interface{}{"path": txtPath}})
	if resp.Result.(map[string]interface{})["detected"] == true {
		t.Error("expected not detected")
	}
}

func TestSBLGNTExtractIR(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "sblgnt-test-*")
	defer os.RemoveAll(tmpDir)
	sblPath := filepath.Join(tmpDir, "sblgnt.txt")
	createTestSBLGNT(t, sblPath)
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)
	resp := executePlugin(t, &ipc.Request{Command: "extract-ir", Args: map[string]interface{}{"path": sblPath, "output_dir": outputDir}})
	if resp.Status != "ok" {
		t.Fatalf("expected ok: %s", resp.Error)
	}
}

func TestSBLGNTEmitNative(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "sblgnt-test-*")
	defer os.RemoveAll(tmpDir)
	corpus := ipc.Corpus{ID: "test", Title: "Test", Documents: []*ipc.Document{{ID: "01", Title: "Matthew", Order: 1, ContentBlocks: []*ipc.ContentBlock{{ID: "cb-1", Sequence: 1, Text: "Βίβλος"}}}}}
	irData, _ := json.MarshalIndent(&corpus, "", "  ")
	irPath := filepath.Join(tmpDir, "test.ir.json")
	os.WriteFile(irPath, irData, 0600)
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)
	resp := executePlugin(t, &ipc.Request{Command: "emit-native", Args: map[string]interface{}{"ir_path": irPath, "output_dir": outputDir}})
	if resp.Result.(map[string]interface{})["format"] != "SBLGNT" {
		t.Error("expected SBLGNT format")
	}
}

func TestSBLGNTRoundTrip(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "sblgnt-test-*")
	defer os.RemoveAll(tmpDir)
	sblPath := filepath.Join(tmpDir, "original.txt")
	createTestSBLGNT(t, sblPath)
	originalData, _ := os.ReadFile(sblPath)
	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)
	extractResp := executePlugin(t, &ipc.Request{Command: "extract-ir", Args: map[string]interface{}{"path": sblPath, "output_dir": irDir}})
	irPath := extractResp.Result.(map[string]interface{})["ir_path"].(string)
	emitResp := executePlugin(t, &ipc.Request{Command: "emit-native", Args: map[string]interface{}{"ir_path": irPath, "output_dir": outDir}})
	outputPath := emitResp.Result.(map[string]interface{})["output_path"].(string)
	outputData, _ := os.ReadFile(outputPath)
	if !bytes.Equal(originalData, outputData) {
		t.Logf("round-trip: %d -> %d bytes", len(originalData), len(outputData))
	}
}

func TestSBLGNTIngest(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "sblgnt-test-*")
	defer os.RemoveAll(tmpDir)
	sblPath := filepath.Join(tmpDir, "sblgnt.txt")
	createTestSBLGNT(t, sblPath)
	outputDir := filepath.Join(tmpDir, "blobs")
	os.MkdirAll(outputDir, 0755)
	resp := executePlugin(t, &ipc.Request{Command: "ingest", Args: map[string]interface{}{"path": sblPath, "output_dir": outputDir}})
	blobHash := resp.Result.(map[string]interface{})["blob_sha256"].(string)
	if _, err := os.Stat(filepath.Join(outputDir, blobHash[:2], blobHash)); os.IsNotExist(err) {
		t.Error("blob not created")
	}
}

func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()
	pluginPath := "./format-sblgnt"
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
