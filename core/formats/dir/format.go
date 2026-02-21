// Package dir provides the canonical directory format handler.
// Directories are detected by checking if the path is a directory.
// This is a container format that does not support IR extraction.
package dir

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
)

// Config defines the directory format plugin configuration.
var Config = &format.Config{
	PluginID:   "format.dir",
	Name:       "dir",
	Extensions: []string{}, // No extensions for directories
	Detect:     detectDir,
	Enumerate:  enumerateDir,
	IngestTransform: func(path string) ([]byte, map[string]string, error) {
		// For directories, create a manifest of all files
		var files []string
		var totalSize int64

		err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				files = append(files, p)
				totalSize += info.Size()
			}
			return nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to walk directory: %w", err)
		}

		// Create a simple manifest blob listing all files
		manifest := struct {
			RootPath string   `json:"root_path"`
			Files    []string `json:"files"`
		}{
			RootPath: filepath.Base(path),
			Files:    make([]string, len(files)),
		}

		for i, f := range files {
			rel, err := filepath.Rel(path, f)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to compute relative path for %s: %w", f, err)
			}
			manifest.Files[i] = rel
		}

		manifestData, err := json.Marshal(manifest)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal manifest: %w", err)
		}

		metadata := map[string]string{
			"format":      "dir",
			"file_count":  fmt.Sprintf("%d", len(files)),
			"total_bytes": fmt.Sprintf("%d", totalSize),
		}

		return manifestData, metadata, nil
	},
}

// detectDir detects if a path is a directory.
func detectDir(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		}, nil
	}

	if !info.IsDir() {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "path is not a directory",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: true,
		Format:   "dir",
		Reason:   "directory detected",
	}, nil
}

// enumerateDir lists all files in a directory recursively.
func enumerateDir(path string) (*ipc.EnumerateResult, error) {
	var entries []ipc.EnumerateEntry

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(path, p)
		if relErr != nil {
			return fmt.Errorf("failed to compute relative path: %w", relErr)
		}
		if rel == "." {
			return nil // Skip root
		}

		entries = append(entries, ipc.EnumerateEntry{
			Path:      rel,
			SizeBytes: info.Size(),
			IsDir:     info.IsDir(),
		})
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to enumerate: %w", err)
	}

	return &ipc.EnumerateResult{
		Entries: entries,
	}, nil
}
