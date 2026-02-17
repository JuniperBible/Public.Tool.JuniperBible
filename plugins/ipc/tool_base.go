// Package ipc provides shared boilerplate for tool plugins.
// This eliminates ~800 lines of duplicate code across 7+ tool plugins.
package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// ToolInfo represents plugin metadata for info command.
type ToolInfo struct {
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	Type        string        `json:"type"` // Always "tool"
	Description string        `json:"description"`
	Profiles    []ProfileInfo `json:"profiles,omitempty"`
	Requires    []string      `json:"requires,omitempty"` // External tools required
}

// ProfileInfo describes a single tool profile.
type ProfileInfo struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// ToolRunRequest is the standard request format for tool execution.
type ToolRunRequest struct {
	Profile string            `json:"profile"`
	Args    map[string]string `json:"args,omitempty"`
	OutDir  string            `json:"out_dir"`
	// Tool-specific fields can be added in custom structs that embed this
}

// ToolIPCRequest is the standard IPC request format.
type ToolIPCRequest struct {
	Command string            `json:"command"`
	Path    string            `json:"path,omitempty"`
	Args    map[string]string `json:"args,omitempty"`
}

// ToolIPCResponse is the standard IPC response format.
type ToolIPCResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// TranscriptEvent is a standard event structure for transcripts.
// Tool plugins can define custom event types that include these fields.
type TranscriptEvent struct {
	Event     string      `json:"event"`
	Timestamp string      `json:"timestamp,omitempty"`
	Plugin    string      `json:"plugin,omitempty"`
	Profile   string      `json:"profile,omitempty"`
	Error     string      `json:"error,omitempty"`
	ExitCode  int         `json:"exit_code,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// ProfileHandler is a function that executes a tool profile.
// It receives the request and transcript, and returns an error if execution fails.
type ProfileHandler func(*ToolRunRequest, *Transcript) error

// ToolConfig configures the standard tool plugin behavior.
type ToolConfig struct {
	PluginName string                                // e.g., "tool-pandoc"
	Info       ToolInfo                              // Metadata returned by info command
	Profiles   map[string]ProfileHandler             // Map profile ID to handler function
	IPCHandler func(*ToolIPCRequest) ToolIPCResponse // Optional custom IPC handler
}

// PrintToolInfo outputs plugin metadata as JSON (info command).
func PrintToolInfo(info ToolInfo) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(info)
}

// RunStandardToolIPC runs the standard tool IPC loop.
// Reads JSON requests from stdin, processes them, and writes responses to stdout.
// If config.IPCHandler is nil, uses the default handler (info and check commands).
func RunStandardToolIPC(config *ToolConfig) {
	reader := bufio.NewReader(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		var req ToolIPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			encoder.Encode(ToolIPCResponse{Success: false, Error: err.Error()})
			continue
		}

		var resp ToolIPCResponse
		if config.IPCHandler != nil {
			resp = config.IPCHandler(&req)
		} else {
			resp = defaultIPCHandler(&req, config)
		}

		encoder.Encode(resp)
	}
}

// defaultIPCHandler handles standard info and check commands.
func defaultIPCHandler(req *ToolIPCRequest, config *ToolConfig) ToolIPCResponse {
	switch req.Command {
	case "info":
		return ToolIPCResponse{
			Success: true,
			Data: map[string]interface{}{
				"name":    config.Info.Name,
				"version": config.Info.Version,
				"type":    config.Info.Type,
			},
		}
	case "check":
		// Default implementation: check if all required tools are available
		available := true
		for _, tool := range config.Info.Requires {
			if !commandExists(tool) {
				available = false
				break
			}
		}
		return ToolIPCResponse{
			Success: available,
			Data:    map[string]bool{"tools_available": available},
		}
	default:
		return ToolIPCResponse{Success: false, Error: "unknown command: " + req.Command}
	}
}

// ParseToolFlags parses standard tool flags from os.Args.
// Returns reqPath (--request) and outDir (--out), or exits with error.
// Usage: reqPath, outDir := ipc.ParseToolFlags()
func ParseToolFlags() (reqPath, outDir string) {
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--request":
			if i+1 < len(os.Args) {
				reqPath = os.Args[i+1]
				i++
			}
		case "--out":
			if i+1 < len(os.Args) {
				outDir = os.Args[i+1]
				i++
			}
		}
	}

	if reqPath == "" || outDir == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s run --request <path> --out <dir>\n", os.Args[0])
		os.Exit(1)
	}

	return reqPath, outDir
}

// LoadToolRequest reads and parses a ToolRunRequest from a file.
// Returns the request with OutDir set, or exits with error.
func LoadToolRequest(reqPath, outDir string) *ToolRunRequest {
	reqData, err := os.ReadFile(reqPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read request: %v\n", err)
		os.Exit(1)
	}

	var req ToolRunRequest
	if err := json.Unmarshal(reqData, &req); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse request: %v\n", err)
		os.Exit(1)
	}

	req.OutDir = outDir
	if err := os.MkdirAll(outDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output dir: %v\n", err)
		os.Exit(1)
	}

	return &req
}

// ExecuteWithTranscript runs a profile handler with standard transcript wrapping.
// Creates transcript, writes start/end events, handles errors, and exits on failure.
// This is the standard execution pattern for all tool plugins.
func ExecuteWithTranscript(req *ToolRunRequest, config *ToolConfig) {
	transcript := NewTranscript(req.OutDir)
	defer transcript.Close()

	// Write start event
	transcript.WriteEvent(TranscriptEvent{
		Event:     "start",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Plugin:    config.PluginName,
		Profile:   req.Profile,
	})

	// Look up profile handler
	handler, ok := config.Profiles[req.Profile]
	if !ok {
		err := fmt.Errorf("unknown profile: %s", req.Profile)
		transcript.WriteEvent(TranscriptEvent{
			Event: "error",
			Error: err.Error(),
		})
		transcript.WriteEvent(TranscriptEvent{
			Event:     "end",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			ExitCode:  1,
		})
		fmt.Fprintf(os.Stderr, "Profile execution failed: %v\n", err)
		os.Exit(1)
	}

	// Execute profile
	err := handler(req, transcript)

	exitCode := 0
	if err != nil {
		exitCode = 1
		transcript.WriteEvent(TranscriptEvent{
			Event: "error",
			Error: err.Error(),
		})
	}

	// Write end event
	transcript.WriteEvent(TranscriptEvent{
		Event:     "end",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		ExitCode:  exitCode,
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Profile execution failed: %v\n", err)
		os.Exit(1)
	}
}

// commandExists checks if a command is available in PATH.
func commandExists(cmd string) bool {
	// Simple check: try to look up the command
	// This is a basic implementation that can be enhanced
	_, err := os.Stat("/usr/bin/" + cmd)
	if err == nil {
		return true
	}
	_, err = os.Stat("/usr/local/bin/" + cmd)
	return err == nil
}
