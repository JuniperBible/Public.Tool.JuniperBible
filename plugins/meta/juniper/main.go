//go:build !sdk

// Package main implements the Juniper meta-plugin.
// Juniper provides a unified CLI for working with Bible modules,
// delegating to specialized format and tool plugins.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// PluginInfo contains plugin metadata.
type PluginInfo struct {
	PluginID    string   `json:"plugin_id"`
	Version     string   `json:"version"`
	Kind        string   `json:"kind"`
	Description string   `json:"description"`
	Delegates   []string `json:"delegates"`
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "info":
			printInfo()
			return
		case "ipc":
			runIPC()
			return
		case "help":
			printUsage()
			return
		case "version":
			fmt.Println("juniper 0.1.0")
			return
		// CLI commands - delegate to specialized plugins
		case "list":
			cmdList()
			return
		case "ingest":
			cmdIngest()
			return
		case "repoman":
			cmdRepoman()
			return
		case "hugo":
			cmdHugo()
			return
		}
	}

	// Default: print usage
	printUsage()
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

func runIPC() {
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
	// This would delegate to format.sword-pure plugin
	// For now, return a placeholder
	sendResult(map[string]interface{}{
		"message": "list-modules delegates to format.sword-pure",
		"args":    req.Args,
	})
}

func handleIngest(req *ipc.Request) {
	// This would delegate to format.sword-pure plugin
	sendResult(map[string]interface{}{
		"message": "ingest delegates to format.sword-pure",
		"args":    req.Args,
	})
}

func handleRepoman(req *ipc.Request) {
	// This would delegate to tool.repoman plugin
	sendResult(map[string]interface{}{
		"message": "repoman delegates to tool.repoman",
		"args":    req.Args,
	})
}

func handleHugoGenerate(req *ipc.Request) {
	// This would delegate to tool.hugo plugin
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
// These commands directly invoke the corresponding plugins when run from CLI

func cmdList() {
	// For CLI usage, we could either:
	// 1. Spawn the format.sword-pure plugin
	// 2. Import and call the sword parsing code directly
	// For now, print guidance
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
