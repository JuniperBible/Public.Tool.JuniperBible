// Package zip provides the canonical ZIP archive format handler.
// ZIP files are detected by magic bytes and can be enumerated.
// This is a container format that does not support IR extraction.
package zip

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
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

func notDetected(reason string) *ipc.DetectResult {
	return &ipc.DetectResult{Detected: false, Reason: reason}
}

func checkRegularFile(path string) *ipc.DetectResult {
	info, err := os.Stat(path)
	if err != nil {
		return notDetected(fmt.Sprintf("cannot stat: %v", err))
	}
	if info.IsDir() {
		return notDetected("path is a directory")
	}
	return nil
}

func readMagicBytes(path string) ([]byte, *ipc.DetectResult) {
	f, err := os.Open(path)
	if err != nil {
		return nil, notDetected(fmt.Sprintf("cannot open file: %v", err))
	}
	defer f.Close()

	magic := make([]byte, 4)
	n, err := f.Read(magic)
	if err != nil || n < 4 {
		return nil, notDetected("cannot read magic bytes")
	}
	return magic, nil
}

func matchesZIPMagic(magic []byte) bool {
	return len(magic) == 4 &&
		magic[0] == zipMagic[0] &&
		magic[1] == zipMagic[1] &&
		magic[2] == zipMagic[2] &&
		magic[3] == zipMagic[3]
}

func detectZIP(path string) (*ipc.DetectResult, error) {
	if r := checkRegularFile(path); r != nil {
		return r, nil
	}

	magic, r := readMagicBytes(path)
	if r != nil {
		return r, nil
	}

	if !matchesZIPMagic(magic) {
		return notDetected("not a ZIP file (wrong magic bytes)"), nil
	}

	if _, err := zip.OpenReader(path); err != nil {
		return notDetected(fmt.Sprintf("not a valid ZIP archive: %v", err)), nil
	}

	return &ipc.DetectResult{Detected: true, Format: "zip", Reason: "valid ZIP archive"}, nil
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
