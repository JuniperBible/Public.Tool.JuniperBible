package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/JuniperBible/juniper/plugins/ipc"
)

// TestPluginInfo tests the info command.
func TestPluginInfo(t *testing.T) {
	pluginPath := buildPlugin(t)

	cmd := exec.Command(pluginPath, "info")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("info command failed: %v", err)
	}

	var info PluginInfo
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		t.Fatalf("failed to parse info output: %v", err)
	}

	if info.PluginID != "meta.juniper" {
		t.Errorf("expected plugin_id 'meta.juniper', got '%s'", info.PluginID)
	}
	if info.Kind != "meta" {
		t.Errorf("expected kind 'meta', got '%s'", info.Kind)
	}
	if len(info.Delegates) != 4 {
		t.Errorf("expected 4 delegates, got %d", len(info.Delegates))
	}
}

// TestHelp tests the help command.
func TestHelp(t *testing.T) {
	pluginPath := buildPlugin(t)

	cmd := exec.Command(pluginPath, "help")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("help command failed: %v", err)
	}

	output := stdout.String()
	if len(output) == 0 {
		t.Error("help should produce output")
	}
	if !bytes.Contains(stdout.Bytes(), []byte("juniper")) {
		t.Error("help output should contain 'juniper'")
	}
}

// TestVersion tests the version command.
func TestVersion(t *testing.T) {
	pluginPath := buildPlugin(t)

	cmd := exec.Command(pluginPath, "version")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := stdout.String()
	if !bytes.Contains(stdout.Bytes(), []byte("0.1.0")) {
		t.Errorf("version output should contain '0.1.0', got: %s", output)
	}
}

// TestUnknownCommand tests handling of unknown commands via IPC.
func TestUnknownCommand(t *testing.T) {
	pluginPath := buildPlugin(t)

	req := ipc.Request{
		Command: "unknown-command",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, pluginPath, &req)

	if resp.Status != "error" {
		t.Errorf("expected error status for unknown command, got %s", resp.Status)
	}
}

// buildPlugin builds the plugin binary and returns its path.
func buildPlugin(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "meta-juniper")

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build plugin: %v", err)
	}

	return binPath
}

// executePlugin runs the plugin with the given IPC request.
func executePlugin(t *testing.T, pluginPath string, req *ipc.Request) *ipc.Response {
	t.Helper()

	reqJSON, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	cmd := exec.Command(pluginPath, "ipc")
	cmd.Stdin = bytes.NewReader(append(reqJSON, '\n'))

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("plugin execution failed: %v", err)
	}

	var resp ipc.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v, output: %s", err, stdout.String())
	}

	return &resp
}
