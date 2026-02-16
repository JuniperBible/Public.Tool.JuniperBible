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

	cmd := exec.Command("./tool-pandoc", "info")
	cmd.Dir = pluginDir(t)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("info command failed: %v", err)
	}

	var info map[string]interface{}
	json.Unmarshal(output, &info)

	if info["name"] != "tool-pandoc" {
		t.Errorf("expected name 'tool-pandoc', got %v", info["name"])
	}

	profiles := info["profiles"].([]interface{})
	if len(profiles) != 4 {
		t.Errorf("expected 4 profiles, got %d", len(profiles))
	}
}

func TestConvertProfile(t *testing.T) {
	if !hasPandoc() {
		t.Skip("pandoc not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	// Create markdown file
	mdFile := filepath.Join(tempDir, "test.md")
	os.WriteFile(mdFile, []byte("# Hello World\n\nThis is a test."), 0644)

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "convert",
		"args": map[string]string{
			"input":  mdFile,
			"from":   "markdown",
			"to":     "html",
			"output": "output.html",
		},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0644)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-pandoc", "run", "--request", reqFile, "--out", outDir)
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

func TestListFormatsProfile(t *testing.T) {
	if !hasPandoc() {
		t.Skip("pandoc not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "list-formats",
		"args":    map[string]string{},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0644)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-pandoc", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	cmd.Run()

	formatsPath := filepath.Join(outDir, "formats.json")
	if _, err := os.Stat(formatsPath); os.IsNotExist(err) {
		t.Error("formats.json not created")
	}
}

func TestExtractMetadataProfile(t *testing.T) {
	if !hasPandoc() {
		t.Skip("pandoc not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	// Create markdown file with metadata
	mdFile := filepath.Join(tempDir, "test.md")
	os.WriteFile(mdFile, []byte(`---
title: Test Document
author: Test Author
---

# Content

Some text here.
`), 0644)

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "extract-metadata",
		"args":    map[string]string{"input": mdFile},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0644)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-pandoc", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	cmd.Run()

	metadataPath := filepath.Join(outDir, "metadata.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Error("metadata.json not created")
	}
}

func buildPlugin(t *testing.T) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", "tool-pandoc", ".")
	cmd.Dir = pluginDir(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build: %v\n%s", err, output)
	}
}

func pluginDir(t *testing.T) string { return "." }

func hasPandoc() bool {
	_, err := exec.LookPath("pandoc")
	return err == nil
}
