// Package epub provides the canonical EPUB format handler.
// EPUB files are ZIP archives with specific structure (mimetype, META-INF/container.xml).
// This is a container format. Full IR support (Parse/Emit) can be added later.
package epub

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

// Config defines the EPUB format plugin configuration.
var Config = &format.Config{
	Name:       "EPUB",
	Extensions: []string{".epub"},
	Detect:     detectEPUB,
	Enumerate:  enumerateEPUB,
	IngestTransform: func(path string) ([]byte, map[string]string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}

		metadata := map[string]string{
			"format": "EPUB",
		}

		return data, metadata, nil
	},
}

// detectEPUB performs EPUB-specific detection.
func detectEPUB(path string) (*ipc.DetectResult, error) {
	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".epub" {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not an .epub file",
		}, nil
	}

	// Try to open as ZIP and verify EPUB structure
	r, err := zip.OpenReader(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open as ZIP: %v", err),
		}, nil
	}
	defer r.Close()

	hasMimetype := false
	hasContainer := false

	for _, f := range r.File {
		if f.Name == "mimetype" {
			hasMimetype = true
		}
		if f.Name == "META-INF/container.xml" {
			hasContainer = true
		}
	}

	if hasMimetype && hasContainer {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "EPUB",
			Reason:   "Valid EPUB structure detected",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "Missing EPUB required files",
	}, nil
}

// enumerateEPUB lists all entries in an EPUB file.
func enumerateEPUB(path string) (*ipc.EnumerateResult, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open EPUB: %w", err)
	}
	defer r.Close()

	var entries []ipc.EnumerateEntry
	for _, f := range r.File {
		// Extract mimetype to metadata
		metadata := map[string]string{"format": "EPUB"}
		if f.Name == "mimetype" {
			rc, _ := f.Open()
			mimeData, _ := io.ReadAll(rc)
			rc.Close()
			if len(mimeData) > 0 {
				metadata["mimetype"] = string(mimeData)
			}
		}

		entries = append(entries, ipc.EnumerateEntry{
			Path:      f.Name,
			SizeBytes: int64(f.UncompressedSize64),
			IsDir:     f.FileInfo().IsDir(),
			Metadata:  metadata,
		})
	}

	return &ipc.EnumerateResult{
		Entries: entries,
	}, nil
}
