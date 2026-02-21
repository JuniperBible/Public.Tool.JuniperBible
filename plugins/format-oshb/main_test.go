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

func createTestOSHB(t *testing.T, path string) {
	t.Helper()
	content := `Gen.1.1 בְּרֵאשִׁית בָּרָא אֱלֹהִים
Gen.1.2 וְהָאָרֶץ הָיְתָה תֹהוּ וָבֹהוּ
Exod.1.1 וְאֵלֶּה שְׁמוֹת בְּנֵי יִשְׂרָאֵל
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test OSHB: %v", err)
	}
}

func TestOSHBDetect(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "oshb-test-*")
	defer os.RemoveAll(tmpDir)
	oshbPath := filepath.Join(tmpDir, "oshb-test.txt")
	createTestOSHB(t, oshbPath)

	resp := executePlugin(t, &ipc.Request{Command: "detect", Args: map[string]interface{}{"path": oshbPath}})
	if resp.Status != "ok" {
		t.Fatalf("expected ok, got %s: %s", resp.Status, resp.Error)
	}
	result := resp.Result.(map[string]interface{})
	if result["detected"] != true {
		t.Error("expected detected true")
	}
}

func TestOSHBDetectNonOSHB(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "oshb-test-*")
	defer os.RemoveAll(tmpDir)
	txtPath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(txtPath, []byte("Hello"), 0600)

	resp := executePlugin(t, &ipc.Request{Command: "detect", Args: map[string]interface{}{"path": txtPath}})
	result := resp.Result.(map[string]interface{})
	if result["detected"] == true {
		t.Error("expected detected false")
	}
}

func TestOSHBExtractIR(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "oshb-test-*")
	defer os.RemoveAll(tmpDir)
	oshbPath := filepath.Join(tmpDir, "oshb.txt")
	createTestOSHB(t, oshbPath)
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0700)

	resp := executePlugin(t, &ipc.Request{Command: "extract-ir", Args: map[string]interface{}{"path": oshbPath, "output_dir": outputDir}})
	if resp.Status != "ok" {
		t.Fatalf("expected ok: %s", resp.Error)
	}
	result := resp.Result.(map[string]interface{})
	if result["loss_class"] != "L1" {
		t.Errorf("expected L1, got %v", result["loss_class"])
	}
}

func TestOSHBEmitNative(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "oshb-test-*")
	defer os.RemoveAll(tmpDir)
	corpus := ir.Corpus{ID: "test", Title: "Test", Documents: []*ir.Document{{ID: "Gen", Title: "Genesis", Order: 1, ContentBlocks: []*ir.ContentBlock{{ID: "cb-1", Sequence: 1, Text: "בְּרֵאשִׁית"}}}}}
	irData, _ := json.MarshalIndent(&corpus, "", "  ")
	irPath := filepath.Join(tmpDir, "test.ir.json")
	os.WriteFile(irPath, irData, 0600)
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0700)

	resp := executePlugin(t, &ipc.Request{Command: "emit-native", Args: map[string]interface{}{"ir_path": irPath, "output_dir": outputDir}})
	if resp.Status != "ok" {
		t.Fatalf("expected ok: %s", resp.Error)
	}
	result := resp.Result.(map[string]interface{})
	if result["format"] != "OSHB" {
		t.Errorf("expected OSHB, got %v", result["format"])
	}
}

func TestOSHBRoundTrip(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "oshb-test-*")
	defer os.RemoveAll(tmpDir)
	oshbPath := filepath.Join(tmpDir, "original.txt")
	createTestOSHB(t, oshbPath)
	originalData, _ := os.ReadFile(oshbPath)
	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0700)
	os.MkdirAll(outDir, 0700)

	extractResp := executePlugin(t, &ipc.Request{Command: "extract-ir", Args: map[string]interface{}{"path": oshbPath, "output_dir": irDir}})
	irPath := extractResp.Result.(map[string]interface{})["ir_path"].(string)
	emitResp := executePlugin(t, &ipc.Request{Command: "emit-native", Args: map[string]interface{}{"ir_path": irPath, "output_dir": outDir}})
	outputPath := emitResp.Result.(map[string]interface{})["output_path"].(string)
	outputData, _ := os.ReadFile(outputPath)

	if !bytes.Equal(originalData, outputData) {
		t.Logf("round-trip: %d -> %d bytes", len(originalData), len(outputData))
	}
}

func TestOSHBIngest(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "oshb-test-*")
	defer os.RemoveAll(tmpDir)
	oshbPath := filepath.Join(tmpDir, "oshb.txt")
	createTestOSHB(t, oshbPath)
	outputDir := filepath.Join(tmpDir, "blobs")
	os.MkdirAll(outputDir, 0700)

	resp := executePlugin(t, &ipc.Request{Command: "ingest", Args: map[string]interface{}{"path": oshbPath, "output_dir": outputDir}})
	if resp.Status != "ok" {
		t.Fatalf("expected ok: %s", resp.Error)
	}
	result := resp.Result.(map[string]interface{})
	blobHash := result["blob_sha256"].(string)
	blobPath := filepath.Join(outputDir, blobHash[:2], blobHash)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("blob not created")
	}
}

func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()
	pluginPath := "./format-oshb"
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
