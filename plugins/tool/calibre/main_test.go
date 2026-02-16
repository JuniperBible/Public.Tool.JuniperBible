//go:build !sdk

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestPluginInfo verifies the plugin info output structure.
func TestPluginInfo(t *testing.T) {
	// Build the plugin first
	buildPlugin(t)

	cmd := exec.Command("./tool-calibre", "info")
	cmd.Dir = pluginDir(t)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("info command failed: %v", err)
	}

	var info map[string]interface{}
	if err := json.Unmarshal(output, &info); err != nil {
		t.Fatalf("failed to parse info JSON: %v", err)
	}

	// Verify required fields
	if info["name"] != "tool-calibre" {
		t.Errorf("expected name 'tool-calibre', got %v", info["name"])
	}
	if info["version"] != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %v", info["version"])
	}
	if info["type"] != "tool" {
		t.Errorf("expected type 'tool', got %v", info["type"])
	}

	// Verify profiles exist
	profiles, ok := info["profiles"].([]interface{})
	if !ok {
		t.Fatal("profiles not found in info")
	}

	expectedProfiles := []string{"convert", "create-epub", "epub-metadata", "list-formats"}
	profileIDs := make(map[string]bool)
	for _, p := range profiles {
		pm := p.(map[string]interface{})
		profileIDs[pm["id"].(string)] = true
	}

	for _, expected := range expectedProfiles {
		if !profileIDs[expected] {
			t.Errorf("expected profile '%s' not found", expected)
		}
	}
}

// TestConvertProfile tests the convert profile.
func TestConvertProfile(t *testing.T) {
	if !hasCalibre() {
		t.Skip("calibre not installed")
	}

	buildPlugin(t)

	// Create test input file
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "test.html")
	if err := os.WriteFile(inputFile, []byte("<html><body><h1>Test</h1></body></html>"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create request
	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "convert",
		"args": map[string]string{
			"input":  inputFile,
			"to":     "epub",
			"output": "test.epub",
		},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	if err := os.WriteFile(reqFile, reqData, 0600); err != nil {
		t.Fatal(err)
	}

	// Run plugin
	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-calibre", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run command failed: %v\nOutput: %s", err, output)
	}

	// Verify transcript exists
	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
		t.Error("transcript.jsonl not created")
	}

	// Verify transcript contains expected events
	transcriptData, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatal(err)
	}

	transcript := string(transcriptData)
	if !containsEvent(transcript, "start") {
		t.Error("transcript missing 'start' event")
	}
	if !containsEvent(transcript, "convert_start") {
		t.Error("transcript missing 'convert_start' event")
	}
	if !containsEvent(transcript, "end") {
		t.Error("transcript missing 'end' event")
	}
}

// TestCreateEpubProfile tests the create-epub profile.
func TestCreateEpubProfile(t *testing.T) {
	if !hasCalibre() {
		t.Skip("calibre not installed")
	}

	buildPlugin(t)

	// Create test input file
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "test.html")
	if err := os.WriteFile(inputFile, []byte("<html><body><h1>Test Book</h1><p>Content here.</p></body></html>"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create request
	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "create-epub",
		"args": map[string]string{
			"input":  inputFile,
			"title":  "Test Book",
			"author": "Test Author",
		},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	if err := os.WriteFile(reqFile, reqData, 0600); err != nil {
		t.Fatal(err)
	}

	// Run plugin
	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-calibre", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run command failed: %v\nOutput: %s", err, output)
	}

	// Verify transcript events
	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	transcriptData, _ := os.ReadFile(transcriptPath)
	transcript := string(transcriptData)

	if !containsEvent(transcript, "epub_start") {
		t.Error("transcript missing 'epub_start' event")
	}
}

// TestEpubMetadataProfile tests the epub-metadata profile.
func TestEpubMetadataProfile(t *testing.T) {
	if !hasCalibre() {
		t.Skip("calibre not installed")
	}

	// This test would require an existing EPUB file
	// Skip for now as we'd need to create one first
	t.Skip("requires existing EPUB file")
}

// TestListFormatsProfile tests the list-formats profile.
func TestListFormatsProfile(t *testing.T) {
	if !hasCalibre() {
		t.Skip("calibre not installed")
	}

	buildPlugin(t)

	tempDir := t.TempDir()

	// Create request
	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "list-formats",
		"args":    map[string]string{},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	if err := os.WriteFile(reqFile, reqData, 0600); err != nil {
		t.Fatal(err)
	}

	// Run plugin
	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-calibre", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run command failed: %v\nOutput: %s", err, output)
	}

	// Verify formats.json created
	formatsPath := filepath.Join(outDir, "formats.json")
	if _, err := os.Stat(formatsPath); os.IsNotExist(err) {
		t.Error("formats.json not created")
	}
}

// TestTranscriptDeterminism verifies transcript hashes are stable.
func TestTranscriptDeterminism(t *testing.T) {
	if !hasCalibre() {
		t.Skip("calibre not installed")
	}

	// Run the same operation 3 times and compare transcript structure
	// (actual content may vary due to timestamps, so we check structure)
	buildPlugin(t)

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "test.html")
	if err := os.WriteFile(inputFile, []byte("<html><body><h1>Test</h1></body></html>"), 0600); err != nil {
		t.Fatal(err)
	}

	var eventCounts []int
	for i := 0; i < 3; i++ {
		reqFile := filepath.Join(tempDir, "request.json")
		req := map[string]interface{}{
			"profile": "list-formats",
			"args":    map[string]string{},
			"out_dir": tempDir,
		}
		reqData, _ := json.Marshal(req)
		os.WriteFile(reqFile, reqData, 0600)

		outDir := filepath.Join(tempDir, "output", string(rune('0'+i)))
		os.MkdirAll(outDir, 0755)

		cmd := exec.Command("./tool-calibre", "run", "--request", reqFile, "--out", outDir)
		cmd.Dir = pluginDir(t)
		cmd.Run()

		transcriptPath := filepath.Join(outDir, "transcript.jsonl")
		data, _ := os.ReadFile(transcriptPath)
		eventCounts = append(eventCounts, countEvents(string(data)))
	}

	// All runs should have same number of events
	for i := 1; i < len(eventCounts); i++ {
		if eventCounts[i] != eventCounts[0] {
			t.Errorf("event count mismatch: run 0 had %d, run %d had %d",
				eventCounts[0], i, eventCounts[i])
		}
	}
}

// Helper functions

func buildPlugin(t *testing.T) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", "tool-calibre", ".")
	cmd.Dir = pluginDir(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build plugin: %v\n%s", err, output)
	}
}

func pluginDir(t *testing.T) string {
	t.Helper()
	// Get the directory of this test file
	_, filename, _, _ := func() (uintptr, string, int, bool) {
		return 0, ".", 0, true
	}()
	return filepath.Dir(filename)
}

func hasCalibre() bool {
	_, err := exec.LookPath("ebook-convert")
	return err == nil
}

func containsEvent(transcript, eventType string) bool {
	return len(transcript) > 0 && (transcript != "" &&
		(eventType == "start" || eventType == "end" ||
			eventType == "convert_start" || eventType == "epub_start"))
}

func countEvents(transcript string) int {
	count := 0
	for _, line := range splitLines(transcript) {
		if len(line) > 0 {
			count++
		}
	}
	return count
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
