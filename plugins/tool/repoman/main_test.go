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

	if info.PluginID != "tool.repoman" {
		t.Errorf("expected plugin_id 'tool.repoman', got '%s'", info.PluginID)
	}
	if info.Kind != "tool" {
		t.Errorf("expected kind 'tool', got '%s'", info.Kind)
	}
	if len(info.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(info.Sources))
	}
}

// TestListSources tests the list-sources command.
func TestListSources(t *testing.T) {
	pluginPath := buildPlugin(t)

	req := ipc.Request{
		Command: "list-sources",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, pluginPath, &req)

	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	count, ok := result["count"].(float64)
	if !ok || int(count) != 2 {
		t.Errorf("expected count 2, got %v", result["count"])
	}

	sources, ok := result["sources"].([]interface{})
	if !ok || len(sources) != 2 {
		t.Errorf("expected 2 sources, got %v", len(sources))
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

// TestInstallMissingModule tests install with missing module argument.
func TestInstallMissingModule(t *testing.T) {
	pluginPath := buildPlugin(t)

	req := ipc.Request{
		Command: "install",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, pluginPath, &req)

	if resp.Status != "error" {
		t.Errorf("expected error status for missing module, got %s", resp.Status)
	}
}

// TestUninstallMissingModule tests uninstall with missing module argument.
func TestUninstallMissingModule(t *testing.T) {
	pluginPath := buildPlugin(t)

	req := ipc.Request{
		Command: "uninstall",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, pluginPath, &req)

	if resp.Status != "error" {
		t.Errorf("expected error status for missing module, got %s", resp.Status)
	}
}

// TestVerifyMissingModule tests verify with missing module argument.
func TestVerifyMissingModule(t *testing.T) {
	pluginPath := buildPlugin(t)

	req := ipc.Request{
		Command: "verify",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, pluginPath, &req)

	if resp.Status != "error" {
		t.Errorf("expected error status for missing module, got %s", resp.Status)
	}
}

// buildPlugin builds the plugin binary for testing.
func buildPlugin(t *testing.T) string {
	t.Helper()

	pluginPath := "./juniper-repoman"
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
