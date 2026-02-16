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

func detect(path string) (bool, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Sprintf("cannot stat: %v", err), nil
	}

	if info.IsDir() {
		return false, "path is a directory, not a file", nil
	}

	return true, "single file detected", nil
}

func parse(path string) (*ir.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	doc := &ir.Document{
		Metadata: map[string]string{
			"format":        "file",
			"original_name": filepath.Base(path),
		},
		CASBlobs: []ir.CASBlob{
			{
				Data: data,
				Metadata: map[string]string{
					"format":        "file",
					"original_name": filepath.Base(path),
				},
			},
		},
	}

	return doc, nil
}

func emit(doc *ir.Document) ([]byte, error) {
	// File format doesn't support emit (read-only format)
	return nil, fmt.Errorf("file format does not support emit")
}

func enumerate(path string) ([]ipc.EnumerateEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	// Single file just returns itself
	return []ipc.EnumerateEntry{
		{
			Path:      filepath.Base(path),
			SizeBytes: info.Size(),
			IsDir:     false,
		},
	}, nil
}
