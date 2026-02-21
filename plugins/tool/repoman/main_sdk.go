//go:build sdk

// Package main implements a SWORD repository management plugin.
// Provides InstallMgr-like functionality for managing SWORD modules.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
)

// TranscriptEvent extends the base event with repoman-specific fields.
type TranscriptEvent struct {
	ipc.TranscriptEvent
	Source string `json:"source,omitempty"`
	Module string `json:"module,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tool-repoman <command> [args]")
		os.Exit(1)
	}

	config := &ipc.ToolConfig{
		PluginName: "tool-repoman",
		Info: ipc.ToolInfo{
			Name:        "tool-repoman",
			Version:     "1.0.0",
			Type:        "tool",
			Description: "SWORD repository management (InstallMgr replacement)",
			Profiles: []ipc.ProfileInfo{
				{ID: "list-sources", Description: "List configured repository sources"},
				{ID: "refresh", Description: "Refresh module index from source"},
				{ID: "list", Description: "List available modules from source"},
				{ID: "install", Description: "Install a module from source"},
				{ID: "installed", Description: "List installed modules"},
				{ID: "uninstall", Description: "Uninstall a module"},
				{ID: "verify", Description: "Verify module integrity"},
			},
			Requires: []string{},
		},
		Profiles: map[string]ipc.ProfileHandler{
			"list-sources": profileListSources,
			"refresh":      profileRefresh,
			"list":         profileList,
			"install":      profileInstall,
			"installed":    profileInstalled,
			"uninstall":    profileUninstall,
			"verify":       profileVerify,
		},
	}

	switch os.Args[1] {
	case "info":
		ipc.PrintToolInfo(config.Info)
	case "run":
		reqPath, outDir := ipc.ParseToolFlags()
		req := ipc.LoadToolRequest(reqPath, outDir)
		ipc.ExecuteWithTranscript(req, config)
	case "ipc":
		ipc.RunStandardToolIPC(config)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func profileListSources(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "list_sources_start"},
	})

	sources := ListSources()

	// Write sources to file
	sourcesFile := filepath.Join(req.OutDir, "sources.json")
	data, _ := json.MarshalIndent(map[string]interface{}{
		"sources": sources,
		"count":   len(sources),
	}, "", "  ")
	os.WriteFile(sourcesFile, data, 0600)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "list_sources_end",
			Data: map[string]interface{}{
				"sources": sources,
				"count":   len(sources),
			},
		},
	})

	return nil
}

func profileRefresh(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	source := req.Args["source"]

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "refresh_start"},
		Source:          source,
	})

	err := RefreshSource(source)
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "refresh_error",
				Error: err.Error(),
			},
			Source: source,
		})
		return fmt.Errorf("failed to refresh: %w", err)
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "refresh_end",
			Data: map[string]interface{}{
				"refreshed": true,
				"source":    source,
			},
		},
		Source: source,
	})

	return nil
}

func profileList(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	source := req.Args["source"]

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "list_start"},
		Source:          source,
	})

	modules, err := ListAvailable(source)
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "list_error",
				Error: err.Error(),
			},
			Source: source,
		})
		return fmt.Errorf("failed to list modules: %w", err)
	}

	// Write modules to file
	modulesFile := filepath.Join(req.OutDir, "modules.json")
	data, _ := json.MarshalIndent(map[string]interface{}{
		"modules": modules,
		"count":   len(modules),
	}, "", "  ")
	os.WriteFile(modulesFile, data, 0600)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "list_end",
			Data: map[string]interface{}{
				"modules": modules,
				"count":   len(modules),
			},
		},
		Source: source,
	})

	return nil
}

func profileInstall(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	source := req.Args["source"]
	module := req.Args["module"]
	destPath := req.Args["dest"]

	if module == "" {
		return fmt.Errorf("missing required argument: module")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "install_start"},
		Source:          source,
		Module:          module,
	})

	err := Install(source, module, destPath)
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "install_error",
				Error: err.Error(),
			},
			Source: source,
			Module: module,
		})
		return fmt.Errorf("failed to install: %w", err)
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "install_end",
			Data: map[string]interface{}{
				"installed": true,
				"module":    module,
			},
		},
		Source: source,
		Module: module,
	})

	return nil
}

func profileInstalled(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	path := req.Args["path"]

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "installed_start"},
	})

	modules, err := ListInstalled(path)
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "installed_error",
				Error: err.Error(),
			},
		})
		return fmt.Errorf("failed to list installed: %w", err)
	}

	// Write installed to file
	installedFile := filepath.Join(req.OutDir, "installed.json")
	data, _ := json.MarshalIndent(map[string]interface{}{
		"modules": modules,
		"count":   len(modules),
	}, "", "  ")
	os.WriteFile(installedFile, data, 0600)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "installed_end",
			Data: map[string]interface{}{
				"modules": modules,
				"count":   len(modules),
			},
		},
	})

	return nil
}

func profileUninstall(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	module := req.Args["module"]
	path := req.Args["path"]

	if module == "" {
		return fmt.Errorf("missing required argument: module")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "uninstall_start"},
		Module:          module,
	})

	err := Uninstall(module, path)
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "uninstall_error",
				Error: err.Error(),
			},
			Module: module,
		})
		return fmt.Errorf("failed to uninstall: %w", err)
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "uninstall_end",
			Data: map[string]interface{}{
				"uninstalled": true,
				"module":      module,
			},
		},
		Module: module,
	})

	return nil
}

func profileVerify(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	module := req.Args["module"]
	path := req.Args["path"]

	if module == "" {
		return fmt.Errorf("missing required argument: module")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "verify_start"},
		Module:          module,
	})

	result, err := Verify(module, path)
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "verify_error",
				Error: err.Error(),
			},
			Module: module,
		})
		return fmt.Errorf("failed to verify: %w", err)
	}

	// Write result to file
	resultFile := filepath.Join(req.OutDir, "verify.json")
	data, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile(resultFile, data, 0600)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "verify_end",
			Data:  result,
		},
		Module: module,
	})

	return nil
}
