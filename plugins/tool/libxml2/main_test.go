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

	cmd := exec.Command("./tool-libxml2", "info")
	cmd.Dir = pluginDir(t)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("info command failed: %v", err)
	}

	var info map[string]interface{}
	json.Unmarshal(output, &info)

	if info["name"] != "tool-libxml2" {
		t.Errorf("expected name 'tool-libxml2', got %v", info["name"])
	}

	profiles := info["profiles"].([]interface{})
	if len(profiles) != 4 {
		t.Errorf("expected 4 profiles, got %d", len(profiles))
	}
}

func TestValidateProfile(t *testing.T) {
	if !hasXmllint() {
		t.Skip("xmllint not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	// Create valid XML
	xmlFile := filepath.Join(tempDir, "test.xml")
	os.WriteFile(xmlFile, []byte(`<?xml version="1.0"?><root><child>text</child></root>`), 0600)

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "validate",
		"args":    map[string]string{"input": xmlFile},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0600)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0700)

	cmd := exec.Command("./tool-libxml2", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	cmd.Run()

	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
		t.Error("transcript.jsonl not created")
	}
}

func TestXPathProfile(t *testing.T) {
	if !hasXmllint() {
		t.Skip("xmllint not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	xmlFile := filepath.Join(tempDir, "test.xml")
	os.WriteFile(xmlFile, []byte(`<?xml version="1.0"?><root><item>one</item><item>two</item></root>`), 0600)

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "xpath",
		"args":    map[string]string{"input": xmlFile, "xpath": "//item/text()"},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0600)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0700)

	cmd := exec.Command("./tool-libxml2", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	cmd.Run()

	// Check result file
	resultPath := filepath.Join(outDir, "xpath_result.txt")
	if _, err := os.Stat(resultPath); os.IsNotExist(err) {
		t.Error("xpath_result.txt not created")
	}
}

func TestFormatProfile(t *testing.T) {
	if !hasXmllint() {
		t.Skip("xmllint not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	xmlFile := filepath.Join(tempDir, "test.xml")
	os.WriteFile(xmlFile, []byte(`<root><child>text</child></root>`), 0600)

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "format",
		"args":    map[string]string{"input": xmlFile},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0600)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0700)

	cmd := exec.Command("./tool-libxml2", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	cmd.Run()

	formattedPath := filepath.Join(outDir, "formatted.xml")
	if _, err := os.Stat(formattedPath); os.IsNotExist(err) {
		t.Error("formatted.xml not created")
	}
}

func buildPlugin(t *testing.T) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", "tool-libxml2", ".")
	cmd.Dir = pluginDir(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build: %v\n%s", err, output)
	}
}

func pluginDir(t *testing.T) string { return "." }

func hasXmllint() bool {
	_, err := exec.LookPath("xmllint")
	return err == nil
}
