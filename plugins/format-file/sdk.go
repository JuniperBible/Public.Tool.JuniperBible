
// Plugin format-file handles single file ingestion.
// It stores files verbatim in the CAS without any transformation.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
)

func runSDK() {
	if err := format.Run(&format.Config{
		Name:       "file",
		Extensions: []string{}, // Matches any file
		Detect:     detectFile,
		IngestTransform: func(path string) ([]byte, map[string]string, error) {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, nil, err
			}

			base := filepath.Base(path)
			return data, map[string]string{
				"original_name": base,
			}, nil
		},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectFile(path string) (*ipc.DetectResult, error) {
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
