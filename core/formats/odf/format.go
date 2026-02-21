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

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/format"
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

var odfExtensions = map[string]bool{
	".odt": true,
	".ods": true,
	".odp": true,
}

func isODFExtension(path string) bool {
	return odfExtensions[strings.ToLower(filepath.Ext(path))]
}

func isMimetypeODF(f *zip.File) bool {
	rc, err := f.Open()
	if err != nil {
		return false
	}
	defer rc.Close()
	data, _ := io.ReadAll(rc)
	return strings.Contains(string(data), "opendocument")
}

func scanODFZip(r *zip.ReadCloser) (hasMimetype, hasContent bool) {
	for _, f := range r.File {
		switch f.Name {
		case "mimetype":
			hasMimetype = isMimetypeODF(f)
		case "content.xml":
			hasContent = true
		}
	}
	return
}

func detectODF(path string) (*ipc.DetectResult, error) {
	if !isODFExtension(path) {
		return &ipc.DetectResult{Detected: false, Reason: "not an ODF file"}, nil
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot open as ZIP: %v", err)}, nil
	}
	defer r.Close()

	hasMimetype, hasContent := scanODFZip(r)
	if hasMimetype && hasContent {
		return &ipc.DetectResult{Detected: true, Format: "ODF", Reason: "Open Document Format detected"}, nil
	}

	return &ipc.DetectResult{Detected: false, Reason: "Missing ODF required files"}, nil
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
