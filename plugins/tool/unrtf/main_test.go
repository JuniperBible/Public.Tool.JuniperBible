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

	cmd := exec.Command("./tool-unrtf", "info")
	cmd.Dir = pluginDir(t)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("info command failed: %v", err)
	}

	var info map[string]interface{}
	json.Unmarshal(output, &info)

	if info["name"] != "tool-unrtf" {
		t.Errorf("expected name 'tool-unrtf', got %v", info["name"])
	}

	profiles := info["profiles"].([]interface{})
	if len(profiles) != 3 {
		t.Errorf("expected 3 profiles, got %d", len(profiles))
	}
}

func TestToHtmlProfile(t *testing.T) {
	if !hasUnrtf() {
		t.Skip("unrtf not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	// Create minimal RTF file
	rtfFile := filepath.Join(tempDir, "test.rtf")
	os.WriteFile(rtfFile, []byte(`{\rtf1\ansi Hello World}`), 0644)

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "to-html",
		"args":    map[string]string{"input": rtfFile},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0644)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-unrtf", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	cmd.Run()

	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
		t.Error("transcript.jsonl not created")
	}

	outputPath := filepath.Join(outDir, "output.html")
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("output.html not created")
	}
}

func TestToTextProfile(t *testing.T) {
	if !hasUnrtf() {
		t.Skip("unrtf not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	rtfFile := filepath.Join(tempDir, "test.rtf")
	os.WriteFile(rtfFile, []byte(`{\rtf1\ansi Test content}`), 0644)

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "to-text",
		"args":    map[string]string{"input": rtfFile},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0644)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-unrtf", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	cmd.Run()

	outputPath := filepath.Join(outDir, "output.txt")
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("output.txt not created")
	}
}

func buildPlugin(t *testing.T) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", "tool-unrtf", ".")
	cmd.Dir = pluginDir(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build: %v\n%s", err, output)
	}
}

func pluginDir(t *testing.T) string { return "." }

func hasUnrtf() bool {
	_, err := exec.LookPath("unrtf")
	return err == nil
}
