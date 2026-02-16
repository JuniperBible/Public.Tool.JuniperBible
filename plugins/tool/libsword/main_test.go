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

	cmd := exec.Command("./tool-libsword", "info")
	cmd.Dir = pluginDir(t)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("info command failed: %v", err)
	}

	var info map[string]interface{}
	json.Unmarshal(output, &info)

	if info["name"] != "tool-libsword" {
		t.Errorf("expected name 'tool-libsword', got %v", info["name"])
	}

	profiles := info["profiles"].([]interface{})
	if len(profiles) < 5 {
		t.Errorf("expected at least 5 profiles, got %d", len(profiles))
	}
}

func TestListModulesProfile(t *testing.T) {
	if !hasDigest() {
		t.Skip("diatheke not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "list-modules",
		"args":    map[string]string{},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0600)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-libsword", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	cmd.Run()

	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
		t.Error("transcript.jsonl not created")
	}
}

func TestRenderVerseProfile(t *testing.T) {
	if !hasDigest() {
		t.Skip("diatheke not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "render-verse",
		"args": map[string]string{
			"module": "KJV",
			"ref":    "Gen.1.1",
		},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0600)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-libsword", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	cmd.Run()

	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
		t.Error("transcript.jsonl not created")
	}
}

func TestEnumerateKeysProfile(t *testing.T) {
	if !hasDigest() {
		t.Skip("diatheke not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "enumerate-keys",
		"args": map[string]string{
			"module": "KJV",
		},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0600)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-libsword", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	cmd.Run()

	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
		t.Error("transcript.jsonl not created")
	}
}

func buildPlugin(t *testing.T) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", "tool-libsword", ".")
	cmd.Dir = pluginDir(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build: %v\n%s", err, output)
	}
}

func pluginDir(t *testing.T) string { return "." }

func hasDigest() bool {
	_, err := exec.LookPath("diatheke")
	return err == nil
}
