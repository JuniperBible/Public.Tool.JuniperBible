// Plugin format-dir handles directory ingestion.
// It recursively enumerates and ingests all files in a directory.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
)

func runSDK() {
	if err := format.Run(&format.Config{
		Name:       "dir",
		Extensions: []string{}, // Matches directories
		Detect:     detectDir,
		Enumerate:  enumerateDir,
		IngestTransform: func(path string) ([]byte, map[string]string, error) {
			// For directories, we create a manifest of all files
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
				rel, _ := filepath.Rel(path, f)
				manifest.Files[i] = rel
			}

			manifestData, _ := json.Marshal(manifest)

			return manifestData, map[string]string{
				"format":      "dir",
				"file_count":  fmt.Sprintf("%d", len(files)),
				"total_bytes": fmt.Sprintf("%d", totalSize),
			}, nil
		},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

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

func enumerateDir(path string) (*ipc.EnumerateResult, error) {
	var entries []ipc.EnumerateEntry

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(path, p)
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
