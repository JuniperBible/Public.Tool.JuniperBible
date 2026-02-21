package tool

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/runtime"
)

func TestMakeInfoHandler(t *testing.T) {
	cfg := &Config{
		Name:        "test-tool",
		Version:     "1.0.0",
		Description: "A test tool",
		Profiles: []Profile{
			{ID: "default", Description: "Default profile"},
			{ID: "fast", Description: "Fast profile"},
		},
		Requires: []string{"external-tool"},
	}

	handler := makeInfoHandler(cfg)
	result, err := handler(map[string]interface{}{})
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}

	info, ok := result.(*ipc.ToolInfo)
	if !ok {
		t.Fatalf("result type = %T, want *ipc.ToolInfo", result)
	}

	if info.Name != "test-tool" {
		t.Errorf("Name = %q, want %q", info.Name, "test-tool")
	}
	if info.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", info.Version, "1.0.0")
	}
	if info.Type != "tool" {
		t.Errorf("Type = %q, want %q", info.Type, "tool")
	}
	if len(info.Profiles) != 2 {
		t.Errorf("len(Profiles) = %d, want 2", len(info.Profiles))
	}
	if len(info.Requires) != 1 {
		t.Errorf("len(Requires) = %d, want 1", len(info.Requires))
	}
}

func TestMakeCheckHandler_NoCheck(t *testing.T) {
	cfg := &Config{
		Name:    "test-tool",
		Version: "1.0.0",
	}

	handler := makeCheckHandler(cfg)
	result, err := handler(map[string]interface{}{})
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}

	checkResult, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map[string]interface{}", result)
	}

	if !checkResult["success"].(bool) {
		t.Error("success = false, want true")
	}
}

func TestMakeCheckHandler_WithCheck(t *testing.T) {
	cfg := &Config{
		Name:    "test-tool",
		Version: "1.0.0",
		Check: func() (bool, error) {
			return true, nil
		},
	}

	handler := makeCheckHandler(cfg)
	result, err := handler(map[string]interface{}{})
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}

	checkResult, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map[string]interface{}", result)
	}

	if !checkResult["success"].(bool) {
		t.Error("success = false, want true")
	}
}

func TestMakeCheckHandler_CheckFails(t *testing.T) {
	cfg := &Config{
		Name:    "test-tool",
		Version: "1.0.0",
		Check: func() (bool, error) {
			return false, nil
		},
	}

	handler := makeCheckHandler(cfg)
	result, err := handler(map[string]interface{}{})
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}

	checkResult, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map[string]interface{}", result)
	}

	if checkResult["success"].(bool) {
		t.Error("success = true, want false")
	}
}

func TestExecCheck(t *testing.T) {
	// Check for common commands that should exist
	if !ExecCheck("ls") && !ExecCheck("dir") {
		// Neither ls nor dir exists - that's unexpected
		t.Log("Neither ls nor dir found in PATH (may be expected on some systems)")
	}

	// Check for a command that definitely doesn't exist
	if ExecCheck("definitely-not-a-real-command-12345") {
		t.Error("ExecCheck() returned true for nonexistent command")
	}
}

func TestExecCheckAll(t *testing.T) {
	// All nonexistent - should fail
	if ExecCheckAll("fake-cmd-1", "fake-cmd-2") {
		t.Error("ExecCheckAll() returned true for nonexistent commands")
	}

	// Empty list - should succeed
	if !ExecCheckAll() {
		t.Error("ExecCheckAll() returned false for empty list")
	}
}

func TestToolPluginIntegration(t *testing.T) {
	cfg := &Config{
		Name:        "test-tool",
		Version:     "1.0.0",
		Description: "A test tool",
		Profiles: []Profile{
			{ID: "default", Description: "Default profile"},
		},
	}

	// Create dispatcher with handlers
	d := runtime.NewDispatcher()
	d.Register("info", makeInfoHandler(cfg))
	d.Register("check", makeCheckHandler(cfg))

	// Test info command
	infoReq := ipc.Request{Command: "info"}
	reqBytes, _ := json.Marshal(infoReq)

	var output bytes.Buffer
	err := runtime.RunWithIO(d, strings.NewReader(string(reqBytes)+"\n"), &output)
	if err != nil {
		t.Fatalf("RunWithIO() error = %v", err)
	}

	var resp ipc.Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("Status = %q, want %q", resp.Status, "ok")
	}
}
