// Package zip provides the canonical ZIP archive format handler.
// ZIP files are detected by magic bytes and can be enumerated.
// This is a container format that does not support IR extraction.
package zip

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
)

// ZIP magic bytes: PK\x03\x04
var zipMagic = []byte{0x50, 0x4b, 0x03, 0x04}

// Config defines the ZIP format plugin configuration.
var Config = &format.Config{
	PluginID:   "format.zip",
	Name:       "zip",
	Extensions: []string{".zip"},
	MagicBytes: zipMagic,
	Detect:     detectZIP,
	Enumerate:  enumerateZIP,
	IngestTransform: func(path string) ([]byte, map[string]string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}

		// Count entries
		zr, err := zip.OpenReader(path)
		entryCount := 0
		if err == nil {
			entryCount = len(zr.File)
			zr.Close()
		}

		metadata := map[string]string{
			"format":        "zip",
			"original_name": filepath.Base(path),
			"entry_count":   fmt.Sprintf("%d", entryCount),
		}

		return data, metadata, nil
	},
}

// detectZIP performs ZIP-specific detection using magic bytes and validation.
func detectZIP(path string) (*ipc.DetectResult, error) {
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
			Reason:   "path is a directory",
		}, nil
	}

	// Check magic bytes
	f, err := os.Open(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open file: %v", err),
		}, nil
	}
	defer f.Close()

	magic := make([]byte, 4)
	n, err := f.Read(magic)
	if err != nil || n < 4 {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "cannot read magic bytes",
		}, nil
	}

	// ZIP magic: PK\x03\x04
	if magic[0] != 0x50 || magic[1] != 0x4b || magic[2] != 0x03 || magic[3] != 0x04 {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not a ZIP file (wrong magic bytes)",
		}, nil
	}

	// Verify it's actually readable as ZIP
	_, err = zip.OpenReader(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("not a valid ZIP archive: %v", err),
		}, nil
	}

	return &ipc.DetectResult{
		Detected: true,
		Format:   "zip",
		Reason:   "valid ZIP archive",
	}, nil
}

// enumerateZIP lists all entries in a ZIP archive.
func enumerateZIP(path string) (*ipc.EnumerateResult, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open ZIP: %w", err)
	}
	defer reader.Close()

	var entries []ipc.EnumerateEntry
	for _, f := range reader.File {
		entries = append(entries, ipc.EnumerateEntry{
			Path:      f.Name,
			SizeBytes: int64(f.UncompressedSize64),
			IsDir:     f.FileInfo().IsDir(),
		})
	}

	return &ipc.EnumerateResult{
		Entries: entries,
	}, nil
}
