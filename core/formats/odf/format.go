// Package odf provides the canonical Open Document Format handler.
// ODF files are ZIP archives with specific structure (mimetype, content.xml).
// This is a container format. Full IR support (Parse/Emit) can be added later.
package odf

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
)

// Config defines the ODF format plugin configuration.
var Config = &format.Config{
	PluginID:   "format.odf",
	Name:       "ODF",
	Extensions: []string{".odt", ".ods", ".odp"},
	Detect:     detectODF,
	Enumerate:  enumerateODF,
	IngestTransform: func(path string) ([]byte, map[string]string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}

		metadata := map[string]string{
			"format": "ODF",
		}

		return data, metadata, nil
	},
}

// detectODF performs ODF-specific detection.
func detectODF(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".odt" && ext != ".ods" && ext != ".odp" {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not an ODF file",
		}, nil
	}

	// Try to open as ZIP and verify ODF structure
	r, err := zip.OpenReader(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open as ZIP: %v", err),
		}, nil
	}
	defer r.Close()

	hasMimetype := false
	hasContent := false

	for _, f := range r.File {
		if f.Name == "mimetype" {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			if strings.Contains(string(data), "opendocument") {
				hasMimetype = true
			}
		}
		if f.Name == "content.xml" {
			hasContent = true
		}
	}

	if hasMimetype && hasContent {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "ODF",
			Reason:   "Open Document Format detected",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "Missing ODF required files",
	}, nil
}

// enumerateODF lists all entries in an ODF file.
func enumerateODF(path string) (*ipc.EnumerateResult, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open ODF: %w", err)
	}
	defer r.Close()

	var entries []ipc.EnumerateEntry
	for _, f := range r.File {
		entries = append(entries, ipc.EnumerateEntry{
			Path:      f.Name,
			SizeBytes: int64(f.UncompressedSize64),
			IsDir:     f.FileInfo().IsDir(),
			Metadata:  map[string]string{"format": "ODF"},
		})
	}

	return &ipc.EnumerateResult{
		Entries: entries,
	}, nil
}
