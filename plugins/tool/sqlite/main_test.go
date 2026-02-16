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

	cmd := exec.Command("./tool-sqlite", "info")
	cmd.Dir = pluginDir(t)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("info command failed: %v", err)
	}

	var info map[string]interface{}
	if err := json.Unmarshal(output, &info); err != nil {
		t.Fatalf("failed to parse info JSON: %v", err)
	}

	if info["name"] != "tool-sqlite" {
		t.Errorf("expected name 'tool-sqlite', got %v", info["name"])
	}

	profiles, ok := info["profiles"].([]interface{})
	if !ok || len(profiles) != 4 {
		t.Errorf("expected 4 profiles, got %v", len(profiles))
	}
}

func TestQueryProfile(t *testing.T) {
	if !hasSqlite() {
		t.Skip("sqlite3 not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	// Create test database
	dbFile := filepath.Join(tempDir, "test.db")
	cmd := exec.Command("sqlite3", dbFile, "CREATE TABLE test(id INTEGER, name TEXT); INSERT INTO test VALUES(1, 'hello');")
	if err := cmd.Run(); err != nil {
		t.Skip("could not create test database")
	}

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "query",
		"args": map[string]string{
			"database": dbFile,
			"sql":      "SELECT * FROM test",
		},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0600)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	pluginCmd := exec.Command("./tool-sqlite", "run", "--request", reqFile, "--out", outDir)
	pluginCmd.Dir = pluginDir(t)
	output, err := pluginCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\nOutput: %s", err, output)
	}

	// Verify transcript
	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
		t.Error("transcript.jsonl not created")
	}
}

func TestTablesProfile(t *testing.T) {
	if !hasSqlite() {
		t.Skip("sqlite3 not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	dbFile := filepath.Join(tempDir, "test.db")
	exec.Command("sqlite3", dbFile, "CREATE TABLE verses(id INTEGER); CREATE TABLE books(id INTEGER);").Run()

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "tables",
		"args":    map[string]string{"database": dbFile},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0600)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	pluginCmd := exec.Command("./tool-sqlite", "run", "--request", reqFile, "--out", outDir)
	pluginCmd.Dir = pluginDir(t)
	pluginCmd.Run()

	// Check tables.json created
	tablesPath := filepath.Join(outDir, "tables.json")
	if _, err := os.Stat(tablesPath); os.IsNotExist(err) {
		t.Error("tables.json not created")
	}
}

func buildPlugin(t *testing.T) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", "tool-sqlite", ".")
	cmd.Dir = pluginDir(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build: %v\n%s", err, output)
	}
}

func pluginDir(t *testing.T) string { return "." }

func hasSqlite() bool {
	_, err := exec.LookPath("sqlite3")
	return err == nil
}
