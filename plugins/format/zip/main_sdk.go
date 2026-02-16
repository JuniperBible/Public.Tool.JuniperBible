//go:build sdk

// Plugin format-zip handles ZIP archive ingestion using the SDK pattern.
// It detects ZIP files by magic bytes and can enumerate contents.
//
// Archive Support:
// - detect: Identifies ZIP archives by magic bytes and extension
// - ingest: Stores ZIP archive as blob with metadata
// - enumerate: Lists all entries in the archive
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

func main() {
	if err := format.Run(&format.Config{
		Name:       "zip",
		Extensions: []string{".zip"},
		Detect:     detectZip,
		Enumerate:  enumerateZip,
		IngestTransform: func(path string) ([]byte, map[string]string, error) {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to read file: %w", err)
			}

			entryCount := countZipEntries(path)

			metadata := map[string]string{
				"original_name": filepath.Base(path),
				"format":        "zip",
				"entry_count":   fmt.Sprintf("%d", entryCount),
			}

			return data, metadata, nil
		},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// detectZip checks if the file is a ZIP archive
func detectZip(path string) (*ipc.DetectResult, error) {
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

	// Check file extension first
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".zip") {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "zip",
			Reason:   "zip file extension detected",
		}, nil
	}

	// Check magic bytes
	f, err := os.Open(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open: %v", err),
		}, nil
	}
	defer f.Close()

	header := make([]byte, len(zipMagic))
	n, err := f.Read(header)
	if err != nil || n < len(zipMagic) {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "cannot read magic bytes",
		}, nil
	}

	// Compare magic bytes
	for i := range zipMagic {
		if header[i] != zipMagic[i] {
			return &ipc.DetectResult{
				Detected: false,
				Reason:   "magic bytes do not match",
			}, nil
		}
	}

	return &ipc.DetectResult{
		Detected: true,
		Format:   "zip",
		Reason:   "zip magic bytes detected",
	}, nil
}

// enumerateZip lists all entries in a ZIP archive
func enumerateZip(path string) (*ipc.EnumerateResult, error) {
	entries, err := enumerateZipEntries(path)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate: %w", err)
	}

	return &ipc.EnumerateResult{
		Entries: entries,
	}, nil
}

// countZipEntries counts the number of entries in a ZIP archive
func countZipEntries(path string) int {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return 0
	}
	defer zr.Close()
	return len(zr.File)
}

// enumerateZipEntries reads all entries from a ZIP archive
func enumerateZipEntries(path string) ([]ipc.EnumerateEntry, error) {
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

	return entries, nil
}
