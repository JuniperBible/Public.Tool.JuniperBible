//go:build sdk

// Plugin format-file handles single file ingestion.
// It stores files verbatim in the CAS without any transformation.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

func main() {
	format.Run(&format.Config{
		Name:       "format-file",
		Extensions: []string{"*"},
		Detect:     detect,
		Parse:      parse,
		Emit:       emit,
		Enumerate:  enumerate,
	})
}

func detect(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "path is a directory, not a file",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: true,
		Format:   "file",
		Reason:   "single file detected",
	}, nil
}

func parse(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	fileName := filepath.Base(path)
	corpus := &ir.Corpus{
		ID:           fileName,
		Version:      "1.0.0",
		ModuleType:   "FILE",
		SourceFormat: "file",
		Attributes: map[string]string{
			"original_name": fileName,
			"_file_raw":     string(data),
		},
	}

	// Create a single document containing the file
	doc := &ir.Document{
		ID:    fileName,
		Title: fileName,
		Order: 1,
		Attributes: map[string]string{
			"original_name": fileName,
		},
	}

	corpus.Documents = []*ir.Document{doc}

	return corpus, nil
}

func emit(corpus *ir.Corpus, outputDir string) (string, error) {
	// Check for raw data for round-trip
	if raw, ok := corpus.Attributes["_file_raw"]; ok && raw != "" {
		fileName := corpus.ID
		if name, ok := corpus.Attributes["original_name"]; ok && name != "" {
			fileName = name
		}
		outputPath := filepath.Join(outputDir, fileName)
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			return "", fmt.Errorf("failed to write file: %w", err)
		}
		return outputPath, nil
	}

	// No raw data available
	return "", fmt.Errorf("file format requires raw data for emit")
}

func enumerate(path string) (*ipc.EnumerateResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	// Single file just returns itself
	return &ipc.EnumerateResult{
		Entries: []ipc.EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
			},
		},
	}, nil
}
