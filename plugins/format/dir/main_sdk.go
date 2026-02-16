//go:build sdk

// Plugin format-dir handles directory ingestion using the SDK pattern.
// It recursively enumerates and ingests all files in a directory.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// Detect checks if the given path is a directory
func Detect(path string) (*ipc.DetectResult, error) {
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

// Parse walks the directory and creates an IR corpus with the directory manifest
func Parse(path string) (*ir.Corpus, error) {
	// Walk the directory to collect all files
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
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	// Create a manifest of all files
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
			return nil, fmt.Errorf("failed to compute relative path for %s: %w", f, err)
		}
		manifest.Files[i] = rel
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Create the corpus
	corpus := &ir.Corpus{
		ID:         filepath.Base(path),
		Version:    "1.0",
		ModuleType: "directory",
		Attributes: map[string]string{
			"format":        "dir",
			"file_count":    fmt.Sprintf("%d", len(files)),
			"total_bytes":   fmt.Sprintf("%d", totalSize),
			"manifest_json": string(manifestData),
		},
	}

	return corpus, nil
}

// Emit stores the directory manifest blob and returns its hash
func Emit(corpus *ir.Corpus, outputDir string) (string, error) {
	manifestJSON, ok := corpus.Attributes["manifest_json"]
	if !ok {
		return "", fmt.Errorf("corpus has no manifest_json in attributes")
	}

	hashHex, err := ipc.StoreBlob(outputDir, []byte(manifestJSON))
	if err != nil {
		return "", fmt.Errorf("failed to store blob: %w", err)
	}

	return hashHex, nil
}

// Enumerate walks the directory and returns all file entries
func Enumerate(path string) (*ipc.EnumerateResult, error) {
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

func main() {
	if err := format.Run(&format.Config{
		Name:       "dir",
		Extensions: []string{},
		Detect:     Detect,
		Parse:      Parse,
		Emit:       Emit,
		Enumerate:  Enumerate,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
