//go:build !sdk

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
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

	if info.PluginID != "tool.hugo" {
		t.Errorf("expected plugin_id 'tool.hugo', got '%s'", info.PluginID)
	}
	if info.Kind != "tool" {
		t.Errorf("expected kind 'tool', got '%s'", info.Kind)
	}
}

// TestUnknownCommand tests handling of unknown commands.
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

// TestGenerateMissingInput tests generate with missing input argument.
func TestGenerateMissingInput(t *testing.T) {
	pluginPath := buildPlugin(t)

	req := ipc.Request{
		Command: "generate",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, pluginPath, &req)

	if resp.Status != "error" {
		t.Errorf("expected error status for missing input, got %s", resp.Status)
	}
}

// TestGenerateMissingOutput tests generate with missing output argument.
func TestGenerateMissingOutput(t *testing.T) {
	pluginPath := buildPlugin(t)

	req := ipc.Request{
		Command: "generate",
		Args:    map[string]interface{}{"input": "/some/path"},
	}

	resp := executePlugin(t, pluginPath, &req)

	if resp.Status != "error" {
		t.Errorf("expected error status for missing output, got %s", resp.Status)
	}
}

// buildPlugin builds the plugin binary for testing.
func buildPlugin(t *testing.T) string {
	t.Helper()

	pluginPath := "./juniper-hugo"
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
		if err := buildCmd.Run(); err != nil {
			t.Fatalf("failed to build plugin: %v", err)
		}
	}

	return pluginPath
}

// executePlugin runs the plugin with a request and returns the response.
func executePlugin(t *testing.T, pluginPath string, req *ipc.Request) *ipc.Response {
	t.Helper()

	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	cmd := exec.Command(pluginPath, "ipc")
	cmd.Stdin = bytes.NewReader(reqData)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stdout.Len() > 0 {
			var resp ipc.Response
			if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
				return &resp
			}
		}
		t.Fatalf("plugin execution failed: %v\nstderr: %s", err, stderr.String())
	}

	var resp ipc.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\noutput: %s", err, stdout.String())
	}

	return &resp
}
