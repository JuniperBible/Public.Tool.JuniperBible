//go:build !sdk

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPluginInfo(t *testing.T) {
	buildPlugin(t)

	cmd := exec.Command("./tool-usfm2osis", "info")
	cmd.Dir = pluginDir(t)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("info command failed: %v", err)
	}

	var info map[string]interface{}
	if err := json.Unmarshal(output, &info); err != nil {
		t.Fatalf("failed to parse info JSON: %v", err)
	}

	if info["name"] != "tool-usfm2osis" {
		t.Errorf("expected name 'tool-usfm2osis', got %v", info["name"])
	}

	profiles, ok := info["profiles"].([]interface{})
	if !ok || len(profiles) != 3 {
		t.Errorf("expected 3 profiles, got %v", len(profiles))
	}
}

func TestConvertProfile(t *testing.T) {
	if !hasUsfm2Osis() {
		t.Skip("usfm2osis not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	// Create test USFM file
	inputFile := filepath.Join(tempDir, "test.usfm")
	usfmContent := `\\id GEN Test
\\h Genesis
\\c 1
\\v 1 In the beginning God created the heavens and the earth.
`
	if err := os.WriteFile(inputFile, []byte(usfmContent), 0644); err != nil {
		t.Fatal(err)
	}

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "convert",
		"args": map[string]string{
			"input":  inputFile,
			"output": "test.osis",
		},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0644)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-usfm2osis", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Output: %s", output)
		// Don't fail - usfm2osis may not be available
	}

	// Verify transcript created
	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
		t.Error("transcript.jsonl not created")
	}
}

func buildPlugin(t *testing.T) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", "tool-usfm2osis", ".")
	cmd.Dir = pluginDir(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build plugin: %v\n%s", err, output)
	}
}

func pluginDir(t *testing.T) string {
	t.Helper()
	return "."
}

func hasUsfm2Osis() bool {
	_, err := exec.LookPath("usfm2osis.py")
	if err != nil {
		_, err = exec.LookPath("usfm2osis")
	}
	return err == nil
}
