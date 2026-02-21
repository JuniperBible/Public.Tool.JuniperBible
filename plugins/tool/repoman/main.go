//go:build !sdk

// Package main implements a SWORD repository management plugin.
// Provides InstallMgr-like functionality for managing SWORD modules.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/JuniperBible/juniper/plugins/ipc"
)

// PluginInfo contains plugin metadata.
type PluginInfo struct {
	PluginID    string   `json:"plugin_id"`
	Version     string   `json:"version"`
	Kind        string   `json:"kind"`
	Description string   `json:"description"`
	Sources     []Source `json:"sources"`
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
		}
	}

	// Default: run IPC mode (read from stdin)
	runIPC()
}

func printInfo() {
	info := PluginInfo{
		PluginID:    "tool.repoman",
		Version:     "0.1.0",
		Kind:        "tool",
		Description: "SWORD repository management (InstallMgr replacement)",
		Sources: []Source{
			{Name: "CrossWire", URL: "https://www.crosswire.org/ftpmirror/pub/sword/packages/rawzip/"},
			{Name: "eBible", URL: "https://ebible.org/sword/"},
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
	case "list-sources":
		handleListSources(req)
	case "refresh":
		handleRefresh(req)
	case "list":
		handleList(req)
	case "install":
		handleInstall(req)
	case "installed":
		handleInstalled(req)
	case "uninstall":
		handleUninstall(req)
	case "verify":
		handleVerify(req)
	default:
		sendError(fmt.Sprintf("unknown command: %s", req.Command))
	}
}

func handleListSources(req *ipc.Request) {
	sources := ListSources()
	sendResult(map[string]interface{}{
		"sources": sources,
		"count":   len(sources),
	})
}

func handleRefresh(req *ipc.Request) {
	source, _ := req.Args["source"].(string)

	err := RefreshSource(source)
	if err != nil {
		sendError(fmt.Sprintf("failed to refresh: %v", err))
		return
	}

	sendResult(map[string]interface{}{
		"refreshed": true,
		"source":    source,
	})
}

func handleList(req *ipc.Request) {
	source, _ := req.Args["source"].(string)

	modules, err := ListAvailable(source)
	if err != nil {
		sendError(fmt.Sprintf("failed to list modules: %v", err))
		return
	}

	sendResult(map[string]interface{}{
		"modules": modules,
		"count":   len(modules),
	})
}

func handleInstall(req *ipc.Request) {
	source, _ := req.Args["source"].(string)
	module, _ := req.Args["module"].(string)
	destPath, _ := req.Args["dest"].(string)

	if module == "" {
		sendError("missing required argument: module")
		return
	}

	err := Install(source, module, destPath)
	if err != nil {
		sendError(fmt.Sprintf("failed to install: %v", err))
		return
	}

	sendResult(map[string]interface{}{
		"installed": true,
		"module":    module,
	})
}

func handleInstalled(req *ipc.Request) {
	path, _ := req.Args["path"].(string)

	modules, err := ListInstalled(path)
	if err != nil {
		sendError(fmt.Sprintf("failed to list installed: %v", err))
		return
	}

	sendResult(map[string]interface{}{
		"modules": modules,
		"count":   len(modules),
	})
}

func handleUninstall(req *ipc.Request) {
	module, _ := req.Args["module"].(string)
	path, _ := req.Args["path"].(string)

	if module == "" {
		sendError("missing required argument: module")
		return
	}

	err := Uninstall(module, path)
	if err != nil {
		sendError(fmt.Sprintf("failed to uninstall: %v", err))
		return
	}

	sendResult(map[string]interface{}{
		"uninstalled": true,
		"module":      module,
	})
}

func handleVerify(req *ipc.Request) {
	module, _ := req.Args["module"].(string)
	path, _ := req.Args["path"].(string)

	if module == "" {
		sendError("missing required argument: module")
		return
	}

	result, err := Verify(module, path)
	if err != nil {
		sendError(fmt.Sprintf("failed to verify: %v", err))
		return
	}

	sendResult(result)
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
