package runtime

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/JuniperBible/juniper/plugins/ipc"
)

func TestDispatcher(t *testing.T) {
	d := NewDispatcher()

	// Register a test handler
	d.Register("test", func(args map[string]interface{}) (interface{}, error) {
		name, _ := args["name"].(string)
		return map[string]string{"greeting": "Hello, " + name}, nil
	})

	// Test registered command
	result, err := d.Handle("test", map[string]interface{}{"name": "World"})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	m, ok := result.(map[string]string)
	if !ok {
		t.Fatalf("result type = %T, want map[string]string", result)
	}
	if m["greeting"] != "Hello, World" {
		t.Errorf("greeting = %q, want %q", m["greeting"], "Hello, World")
	}

	// Test unknown command
	_, err = d.Handle("unknown", nil)
	if err == nil {
		t.Error("Handle(unknown) should return error")
	}
}

func TestRunWithIO(t *testing.T) {
	d := NewDispatcher()
	d.Register("echo", func(args map[string]interface{}) (interface{}, error) {
		return args, nil
	})

	// Prepare input
	req := ipc.Request{
		Command: "echo",
		Args:    map[string]interface{}{"key": "value"},
	}
	reqBytes, _ := json.Marshal(req)
	input := string(reqBytes) + "\n"

	// Run
	var output bytes.Buffer
	err := RunWithIO(d, strings.NewReader(input), &output)
	if err != nil {
		t.Fatalf("RunWithIO() error = %v", err)
	}

	// Parse output
	var resp ipc.Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("Status = %q, want %q", resp.Status, "ok")
	}
}

func TestRunWithIO_Error(t *testing.T) {
	d := NewDispatcher()
	// No handlers registered

	req := ipc.Request{Command: "unknown"}
	reqBytes, _ := json.Marshal(req)
	input := string(reqBytes) + "\n"

	var output bytes.Buffer
	err := RunWithIO(d, strings.NewReader(input), &output)
	if err != nil {
		t.Fatalf("RunWithIO() error = %v", err)
	}

	var resp ipc.Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "error" {
		t.Errorf("Status = %q, want %q", resp.Status, "error")
	}
	if resp.Error == "" {
		t.Error("Error message is empty")
	}
}

func TestRunWithIO_InvalidJSON(t *testing.T) {
	d := NewDispatcher()

	input := "not valid json\n"

	var output bytes.Buffer
	err := RunWithIO(d, strings.NewReader(input), &output)
	if err != nil {
		t.Fatalf("RunWithIO() error = %v", err)
	}

	var resp ipc.Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "error" {
		t.Errorf("Status = %q, want %q", resp.Status, "error")
	}
}

func TestRunWithIO_MultipleRequests(t *testing.T) {
	d := NewDispatcher()
	counter := 0
	d.Register("count", func(args map[string]interface{}) (interface{}, error) {
		counter++
		return map[string]int{"count": counter}, nil
	})

	// Send multiple requests
	var input strings.Builder
	for i := 0; i < 3; i++ {
		req := ipc.Request{Command: "count"}
		reqBytes, _ := json.Marshal(req)
		input.Write(reqBytes)
		input.WriteString("\n")
	}

	var output bytes.Buffer
	err := RunWithIO(d, strings.NewReader(input.String()), &output)
	if err != nil {
		t.Fatalf("RunWithIO() error = %v", err)
	}

	// Parse responses
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d responses, want 3", len(lines))
	}

	if counter != 3 {
		t.Errorf("counter = %d, want 3", counter)
	}
}

func TestRunDispatcher(t *testing.T) {
	// This is a simple smoke test - RunDispatcher wraps Run
	// which reads from stdin, so we can't easily test it directly
	// The functionality is tested via RunWithIO

	// Just verify the function exists and has correct signature
	var _ = RunDispatcher
}

func TestHandlerFunc(t *testing.T) {
	fn := HandlerFunc(func(cmd string, args map[string]interface{}) (interface{}, error) {
		return cmd, nil
	})

	result, err := fn.Handle("test", nil)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if result != "test" {
		t.Errorf("result = %v, want %v", result, "test")
	}
}
