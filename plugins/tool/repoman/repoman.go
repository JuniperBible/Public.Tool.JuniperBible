// Package main implements a SWORD repository management plugin.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/safefile"
)

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

// Source represents a SWORD repository source.
type Source struct {
	Name string `json:"name"`
	URL  string `json:"url"`
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
