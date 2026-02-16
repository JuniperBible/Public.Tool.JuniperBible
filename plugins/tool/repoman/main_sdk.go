//go:build sdk

// Package main implements a SWORD repository management plugin.
// Provides InstallMgr-like functionality for managing SWORD modules.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// TranscriptEvent extends the base event with repoman-specific fields.
type TranscriptEvent struct {
	ipc.TranscriptEvent
	Source string `json:"source,omitempty"`
	Module string `json:"module,omitempty"`
}

// Source represents a SWORD repository source.
type Source struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ModuleInfo contains metadata about a SWORD module.
type ModuleInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Language    string `json:"language"`
	Version     string `json:"version"`
	Size        int64  `json:"size"`
	DataPath    string `json:"data_path,omitempty"`
}

// VerifyResult contains the result of module verification.
type VerifyResult struct {
	Module   string   `json:"module"`
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
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
	os.WriteFile(sourcesFile, data, 0644)

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
	os.WriteFile(modulesFile, data, 0644)

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
	os.WriteFile(installedFile, data, 0644)

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
	os.WriteFile(resultFile, data, 0644)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "verify_end",
			Data:  result,
		},
		Module: module,
	})

	return nil
}

// ListInstalled lists installed modules.
func ListInstalled(swordPath string) ([]ModuleInfo, error) {
	if swordPath == "" {
		swordPath = "."
	}

	modsDir := filepath.Join(swordPath, "mods.d")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []ModuleInfo{}, nil
		}
		return nil, fmt.Errorf("reading mods.d: %w", err)
	}

	var modules []ModuleInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") {
			continue
		}

		confPath := filepath.Join(modsDir, entry.Name())
		data, err := safefile.ReadFile(confPath)
		if err != nil {
			continue
		}

		module, err := ParseModuleConf(data)
		if err != nil {
			continue
		}

		modules = append(modules, module)
	}

	return modules, nil
}

// Uninstall removes an installed module.
func Uninstall(moduleID, swordPath string) error {
	if swordPath == "" {
		swordPath = "."
	}

	confPath := filepath.Join(swordPath, "mods.d", strings.ToLower(moduleID)+".conf")
	if _, err := os.Stat(confPath); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("module %s not installed", moduleID)
	}

	data, err := safefile.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("reading conf: %w", err)
	}

	module, err := ParseModuleConf(data)
	if err != nil {
		return fmt.Errorf("parsing conf: %w", err)
	}

	if module.DataPath != "" {
		dataPath := filepath.Join(swordPath, module.DataPath)
		dataPath = strings.TrimPrefix(dataPath, "./")
		if err := os.RemoveAll(dataPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing data: %w", err)
		}
	}

	if err := os.Remove(confPath); err != nil {
		return fmt.Errorf("removing conf: %w", err)
	}

	return nil
}

// Verify verifies a module's integrity.
func Verify(moduleID, swordPath string) (*VerifyResult, error) {
	if swordPath == "" {
		swordPath = "."
	}

	result := &VerifyResult{
		Module: moduleID,
		Valid:  false,
	}

	confPath := filepath.Join(swordPath, "mods.d", strings.ToLower(moduleID)+".conf")
	if _, err := os.Stat(confPath); errors.Is(err, os.ErrNotExist) {
		result.Errors = append(result.Errors, "module not installed")
		return result, nil
	}

	data, err := safefile.ReadFile(confPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("cannot read conf: %v", err))
		return result, nil
	}

	module, err := ParseModuleConf(data)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("invalid conf: %v", err))
		return result, nil
	}

	if module.DataPath != "" {
		dataPath := filepath.Join(swordPath, module.DataPath)
		dataPath = strings.TrimPrefix(dataPath, "./")
		info, err := os.Stat(dataPath)
		if errors.Is(err, os.ErrNotExist) {
			result.Errors = append(result.Errors, "data directory missing")
			return result, nil
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("cannot stat data: %v", err))
			return result, nil
		}
		if !info.IsDir() {
			result.Errors = append(result.Errors, "data path is not a directory")
			return result, nil
		}

		entries, err := os.ReadDir(dataPath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("cannot read data dir: %v", err))
			return result, nil
		}
		if len(entries) == 0 {
			result.Warnings = append(result.Warnings, "data directory is empty")
		}
	}

	result.Valid = len(result.Errors) == 0
	return result, nil
}
