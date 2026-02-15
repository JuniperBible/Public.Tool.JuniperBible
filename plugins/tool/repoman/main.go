// Package main implements a SWORD repository management plugin.
// Provides InstallMgr-like functionality for managing SWORD modules.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// PluginInfo contains plugin metadata.
type PluginInfo struct {
	PluginID    string   `json:"plugin_id"`
	Version     string   `json:"version"`
	Kind        string   `json:"kind"`
	Description string   `json:"description"`
	Sources     []Source `json:"sources"`
}

// Source represents a SWORD repository source.
type Source struct {
	Name string `json:"name"`
	URL  string `json:"url"`
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

// ListSources returns the list of configured sources.
func ListSources() []Source {
	sources := DefaultSources()
	result := make([]Source, len(sources))
	for i, s := range sources {
		result[i] = Source{Name: s.Name, URL: s.URL}
	}
	return result
}

// RefreshSource refreshes the module index for a source.
func RefreshSource(sourceName string) error {
	source, ok := GetSource(sourceName)
	if !ok {
		return fmt.Errorf("unknown source: %s", sourceName)
	}

	client := NewClient()
	ctx := context.Background()

	indexURL := source.ModsIndexURL()
	_, err := client.Download(ctx, indexURL)
	if err != nil {
		return fmt.Errorf("downloading index: %w", err)
	}

	return nil
}

// ListAvailable lists available modules from a source.
func ListAvailable(sourceName string) ([]ModuleInfo, error) {
	source, ok := GetSource(sourceName)
	if !ok {
		return nil, fmt.Errorf("unknown source: %s", sourceName)
	}

	client := NewClient()
	ctx := context.Background()

	indexURL := source.ModsIndexURL()
	data, err := client.Download(ctx, indexURL)
	if err != nil {
		return nil, fmt.Errorf("downloading index: %w", err)
	}

	return ParseModsArchive(data)
}

// Install installs a module from a source.
func Install(sourceName, moduleID, destPath string) error {
	source, ok := GetSource(sourceName)
	if !ok {
		return fmt.Errorf("unknown source: %s", sourceName)
	}

	client := NewClient()
	ctx := context.Background()

	// Try multiple package URLs
	packageURLs := source.ModulePackageURLs(moduleID)
	var data []byte
	var lastErr error

	for _, url := range packageURLs {
		data, lastErr = client.Download(ctx, url)
		if lastErr == nil {
			break
		}
	}

	if data == nil {
		return fmt.Errorf("downloading module: %w", lastErr)
	}

	// Extract to destination
	if destPath == "" {
		destPath = "."
	}

	if err := ExtractZipArchive(data, destPath); err != nil {
		return fmt.Errorf("extracting module: %w", err)
	}

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

	// Find and remove conf file
	confPath := filepath.Join(swordPath, "mods.d", strings.ToLower(moduleID)+".conf")
	if _, err := os.Stat(confPath); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("module %s not installed", moduleID)
	}

	// Read conf to find data path
	data, err := safefile.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("reading conf: %w", err)
	}

	module, err := ParseModuleConf(data)
	if err != nil {
		return fmt.Errorf("parsing conf: %w", err)
	}

	// Remove data directory
	if module.DataPath != "" {
		dataPath := filepath.Join(swordPath, module.DataPath)
		dataPath = strings.TrimPrefix(dataPath, "./")
		if err := os.RemoveAll(dataPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing data: %w", err)
		}
	}

	// Remove conf file
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

	// Check conf file exists
	confPath := filepath.Join(swordPath, "mods.d", strings.ToLower(moduleID)+".conf")
	if _, err := os.Stat(confPath); errors.Is(err, os.ErrNotExist) {
		result.Errors = append(result.Errors, "module not installed")
		return result, nil
	}

	// Read and parse conf
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

	// Check data path exists
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

		// Check for data files
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
