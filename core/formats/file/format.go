// Package file provides the canonical single file format handler.
// Stores files verbatim in the CAS without any transformation.
// This is a catch-all format that does not support IR extraction.
package file

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
)

// Config defines the single file format plugin configuration.
var Config = &format.Config{
	PluginID:   "format.file",
	Name:       "file",
	Extensions: []string{}, // No specific extensions - catch-all
	Detect:     detectFile,
	Enumerate:  enumerateFile,
	IngestTransform: func(path string) ([]byte, map[string]string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}

		metadata := map[string]string{
			"format":        "file",
			"original_name": filepath.Base(path),
		}

		return data, metadata, nil
	},
}

// detectFile detects if a path is a regular file.
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

// enumerateFile returns the single file as an entry.
func enumerateFile(path string) (*ipc.EnumerateResult, error) {
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
