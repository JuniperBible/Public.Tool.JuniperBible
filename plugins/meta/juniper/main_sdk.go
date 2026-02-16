//go:build sdk

// Package main implements the Juniper meta-plugin.
// Juniper provides a unified CLI for working with Bible modules,
// delegating to specialized format and tool plugins.
//
// SDK version - uses the tool SDK pattern for IPC mode.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// TranscriptEvent extends the base event with juniper-specific fields.
type TranscriptEvent struct {
	ipc.TranscriptEvent
	Command string `json:"command,omitempty"`
}

// PluginInfo contains plugin metadata.
type PluginInfo struct {
	PluginID    string   `json:"plugin_id"`
	Version     string   `json:"version"`
	Kind        string   `json:"kind"`
	Description string   `json:"description"`
	Delegates   []string `json:"delegates"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	config := &ipc.ToolConfig{
		PluginName: "meta-juniper",
		Info: ipc.ToolInfo{
			Name:        "meta-juniper",
			Version:     "0.1.0",
			Type:        "meta",
			Description: "Unified CLI for Bible module processing",
			Profiles: []ipc.ProfileInfo{
				{ID: "list-modules", Description: "List available SWORD modules"},
				{ID: "ingest", Description: "Create capsules from SWORD modules"},
				{ID: "repoman", Description: "SWORD repository management"},
				{ID: "hugo-generate", Description: "Generate Hugo JSON output"},
			},
			Requires: []string{},
		},
		Profiles: map[string]ipc.ProfileHandler{
			"list-modules":  profileListModules,
			"ingest":        profileIngest,
			"repoman":       profileRepoman,
			"hugo-generate": profileHugoGenerate,
		},
	}

	switch os.Args[1] {
	case "info":
		printInfo()
	case "ipc":
		runIPCLegacy() // Keep legacy IPC for backward compatibility
	case "run":
		reqPath, outDir := ipc.ParseToolFlags()
		req := ipc.LoadToolRequest(reqPath, outDir)
		ipc.ExecuteWithTranscript(req, config)
	case "help":
		printUsage()
	case "version":
		fmt.Println("juniper 0.1.0")
	// CLI commands - delegate to specialized plugins
	case "list":
		cmdList()
	case "ingest":
		cmdIngest()
	case "repoman":
		cmdRepoman()
	case "hugo":
		cmdHugo()
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Print(`juniper - Bible Module Processing Tool

Usage:
  juniper list [path]               List Bible modules (SWORD format)
  juniper ingest [options]          Ingest modules into capsules
  juniper repoman <command>         SWORD repository management
  juniper hugo <input> <output>     Generate Hugo JSON output

  juniper info                      Print plugin info as JSON
  juniper ipc                       Run in IPC mode (for plugin system)
  juniper run                       Run in SDK tool mode
  juniper help                      Print this help
  juniper version                   Print version

Commands:
  list      List available SWORD modules at the specified path
            Default path: ~/.sword

  ingest    Create capsules from SWORD modules
            Options:
              --all              Ingest all Bible modules
              --output <dir>     Output directory (default: capsules/)

  repoman   Repository management (InstallMgr replacement)
            Subcommands: list-sources, refresh, list, install, uninstall

  hugo      Generate Hugo-compatible JSON from Bible modules
            Creates bibles.json index and chapter JSON files

Delegates to:
  format.sword-pure    Pure Go SWORD module parser
  format.esword        e-Sword database parser
  tool.repoman         SWORD repository management
  tool.hugo            Hugo JSON generator
`)
}

func printInfo() {
	info := PluginInfo{
		PluginID:    "meta.juniper",
		Version:     "0.1.0",
		Kind:        "meta",
		Description: "Unified CLI for Bible module processing",
		Delegates: []string{
			"format.sword-pure",
			"format.esword",
			"tool.repoman",
			"tool.hugo",
		},
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(info)
}

// SDK Profile Handlers

func profileListModules(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "list_modules_start"},
		Command:         "list-modules",
	})

	// Delegates to format.sword-pure plugin
	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "list_modules_end",
			Data: map[string]interface{}{
				"message": "list-modules delegates to format.sword-pure",
				"args":    req.Args,
			},
		},
	})

	return nil
}

func profileIngest(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "ingest_start"},
		Command:         "ingest",
	})

	// Delegates to format.sword-pure plugin
	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "ingest_end",
			Data: map[string]interface{}{
				"message": "ingest delegates to format.sword-pure",
				"args":    req.Args,
			},
		},
	})

	return nil
}

func profileRepoman(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "repoman_start"},
		Command:         "repoman",
	})

	// Delegates to tool.repoman plugin
	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "repoman_end",
			Data: map[string]interface{}{
				"message": "repoman delegates to tool.repoman",
				"args":    req.Args,
			},
		},
	})

	return nil
}

func profileHugoGenerate(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "hugo_generate_start"},
		Command:         "hugo-generate",
	})

	// Delegates to tool.hugo plugin
	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "hugo_generate_end",
			Data: map[string]interface{}{
				"message": "hugo-generate delegates to tool.hugo",
				"args":    req.Args,
			},
		},
	})

	return nil
}

// Legacy IPC mode for backward compatibility

func runIPCLegacy() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req ipc.Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			sendError(fmt.Sprintf("invalid JSON: %v", err))
			continue
		}

		handleRequest(&req)
	}

	if err := scanner.Err(); err != nil {
		sendError(fmt.Sprintf("stdin read error: %v", err))
	}
}

func handleRequest(req *ipc.Request) {
	switch req.Command {
	case "list-modules":
		handleListModules(req)
	case "ingest":
		handleIngest(req)
	case "repoman":
		handleRepoman(req)
	case "hugo-generate":
		handleHugoGenerate(req)
	default:
		sendError(fmt.Sprintf("unknown command: %s", req.Command))
	}
}

func handleListModules(req *ipc.Request) {
	sendResult(map[string]interface{}{
		"message": "list-modules delegates to format.sword-pure",
		"args":    req.Args,
	})
}

func handleIngest(req *ipc.Request) {
	sendResult(map[string]interface{}{
		"message": "ingest delegates to format.sword-pure",
		"args":    req.Args,
	})
}

func handleRepoman(req *ipc.Request) {
	sendResult(map[string]interface{}{
		"message": "repoman delegates to tool.repoman",
		"args":    req.Args,
	})
}

func handleHugoGenerate(req *ipc.Request) {
	sendResult(map[string]interface{}{
		"message": "hugo-generate delegates to tool.hugo",
		"args":    req.Args,
	})
}

func sendResult(result interface{}) {
	resp := ipc.Response{
		Status: "ok",
		Result: result,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
}

func sendError(msg string) {
	resp := ipc.Response{
		Status: "error",
		Error:  msg,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
}

// CLI command implementations

func cmdList() {
	fmt.Println("juniper list - List SWORD modules")
	fmt.Println()
	fmt.Println("Use 'format-sword-pure list [path]' directly or run:")
	fmt.Println("  capsule run format.sword-pure list-modules")
}

func cmdIngest() {
	fmt.Println("juniper ingest - Create capsules from SWORD modules")
	fmt.Println()
	fmt.Println("Use 'format-sword-pure ingest [modules...]' directly or run:")
	fmt.Println("  capsule run format.sword-pure ingest")
}

func cmdRepoman() {
	fmt.Println("juniper repoman - SWORD repository management")
	fmt.Println()
	fmt.Println("Use 'tool-repoman [command]' directly or run:")
	fmt.Println("  capsule run tool.repoman [command]")
	fmt.Println()
	fmt.Println("Available commands: list-sources, refresh, list, install, uninstall, verify")
}

func cmdHugo() {
	fmt.Println("juniper hugo - Generate Hugo JSON output")
	fmt.Println()
	fmt.Println("Use 'tool-hugo [input] [output]' directly or run:")
	fmt.Println("  capsule run tool.hugo generate")
}
