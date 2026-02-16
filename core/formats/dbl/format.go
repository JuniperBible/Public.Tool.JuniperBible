// Package dbl provides the canonical Digital Bible Library format handler.
// DBL is a ZIP bundle format containing USX files and metadata.xml.
// This is a container format. Full IR support (Parse/Emit) can be added later.
package dbl

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
)

// Config defines the DBL format plugin configuration.
var Config = &format.Config{
	Name:       "DBL",
	Extensions: []string{".zip", ".dbl"},
	Detect:     detectDBL,
	Enumerate:  enumerateDBL,
	IngestTransform: func(path string) ([]byte, map[string]string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}

		metadata := map[string]string{
			"format": "DBL",
		}

		return data, metadata, nil
	},
}

// detectDBL performs DBL-specific detection.
func detectDBL(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".zip" && ext != ".dbl" {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not a .zip or .dbl file",
		}, nil
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open as zip: %v", err),
		}, nil
	}
	defer r.Close()

	// Check for metadata.xml (DBL indicator)
	hasMetadata := false
	for _, f := range r.File {
		if f.Name == "metadata.xml" || strings.HasSuffix(f.Name, "/metadata.xml") {
			hasMetadata = true
			break
		}
	}

	if hasMetadata {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "DBL",
			Reason:   "Digital Bible Library bundle detected",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "no metadata.xml found in bundle",
	}, nil
}

// enumerateDBL lists all entries in a DBL bundle.
func enumerateDBL(path string) (*ipc.EnumerateResult, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	var entries []ipc.EnumerateEntry
	for _, f := range r.File {
		entries = append(entries, ipc.EnumerateEntry{
			Path:      f.Name,
			SizeBytes: int64(f.UncompressedSize64),
			IsDir:     f.FileInfo().IsDir(),
			Metadata:  map[string]string{"format": "DBL"},
		})
	}

	return &ipc.EnumerateResult{
		Entries: entries,
	}, nil
}
