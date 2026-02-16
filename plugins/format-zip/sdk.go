// Plugin format-zip handles ZIP archive ingestion.
// It detects ZIP files by magic bytes and can enumerate contents.
package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
)

// ZIP magic bytes: PK\x03\x04
var zipMagic = []byte{0x50, 0x4b, 0x03, 0x04}

func runSDK() {
	if err := format.Run(&format.Config{
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

			base := filepath.Base(path)
			return data, map[string]string{
				"original_name": base,
				"format":        "zip",
				"entry_count":   fmt.Sprintf("%d", entryCount),
			}, nil
		},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectZIP(path string) (*ipc.DetectResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open: %v", err),
		}, nil
	}
	defer f.Close()

	// Check for directory
	info, err := f.Stat()
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

	// Read first 4 bytes to check magic
	header := make([]byte, 4)
	n, err := f.Read(header)
	if err != nil || n < 4 {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "file too small for ZIP",
		}, nil
	}

	// Check ZIP magic bytes
	if header[0] == zipMagic[0] && header[1] == zipMagic[1] &&
		header[2] == zipMagic[2] && header[3] == zipMagic[3] {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "zip",
			Reason:   "ZIP magic bytes detected",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "not a ZIP file (magic mismatch)",
	}, nil
}

func enumerateZIP(path string) (*ipc.EnumerateResult, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}
	defer zr.Close()

	var entries []ipc.EnumerateEntry
	for _, f := range zr.File {
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

// Compile check
var _ = strings.TrimSpace
