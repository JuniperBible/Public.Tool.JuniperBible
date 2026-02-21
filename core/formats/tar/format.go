// Package tar provides the canonical TAR archive format handler.
// TAR files are detected by extension and can enumerate contents.
// Supports compressed tar files (.tar.gz, .tar.xz).
// This is a container format that does not support IR extraction.
package tar

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
	"github.com/ulikunitz/xz"
)

// Config defines the TAR format plugin configuration.
var Config = &format.Config{
	PluginID:   "format.tar",
	Name:       "tar",
	Extensions: []string{".tar", ".tar.gz", ".tgz", ".tar.xz", ".txz"},
	Detect:     detectTAR,
	Enumerate:  enumerateTAR,
	IngestTransform: func(path string) ([]byte, map[string]string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}

		// Detect compression type
		compression := detectCompression(path, data)

		// Count entries
		entryCount := countTarEntries(path)

		metadata := map[string]string{
			"format":        "tar",
			"original_name": filepath.Base(path),
			"compression":   compression,
			"entry_count":   fmt.Sprintf("%d", entryCount),
		}

		return data, metadata, nil
	},
}

// detectTAR performs TAR-specific detection.
var tarExtensions = []string{".tar", ".tar.gz", ".tgz", ".tar.xz", ".txz"}

func hasTarExtension(path string) bool {
	lower := strings.ToLower(path)
	for _, ext := range tarExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func detectTAR(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return tarNotDetected("not a file"), nil
	}

	if hasTarExtension(path) {
		return tarDetected("tar file extension detected"), nil
	}

	return detectTarByContent(path)
}

func tarDetected(reason string) *ipc.DetectResult {
	return &ipc.DetectResult{Detected: true, Format: "tar", Reason: reason}
}

func tarNotDetected(reason string) *ipc.DetectResult {
	return &ipc.DetectResult{Detected: false, Reason: reason}
}

func detectTarByContent(path string) (*ipc.DetectResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return tarNotDetected(fmt.Sprintf("cannot open: %v", err)), nil
	}
	defer f.Close()

	tr := tar.NewReader(f)
	_, err = tr.Next()
	if err == nil {
		return tarDetected("valid tar header found"), nil
	}

	return tarNotDetected("not a tar file"), nil
}

// enumerateTAR lists all entries in a TAR archive.
func enumerateTAR(path string) (*ipc.EnumerateResult, error) {
	entries, err := enumerateTarImpl(path)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate: %w", err)
	}

	return &ipc.EnumerateResult{
		Entries: entries,
	}, nil
}

var extCompressionMap = map[string]string{
	".gz":  "gzip",
	".tgz": "gzip",
	".xz":  "xz",
	".txz": "xz",
}

func compressionFromMagic(data []byte) string {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		return "gzip"
	}
	if len(data) >= 6 && data[0] == 0xfd && string(data[1:6]) == "7zXZ\x00" {
		return "xz"
	}
	return "none"
}

func detectCompression(path string, data []byte) string {
	lower := strings.ToLower(path)
	for ext, kind := range extCompressionMap {
		if strings.HasSuffix(lower, ext) {
			return kind
		}
	}
	return compressionFromMagic(data)
}

// countTarEntries counts the number of entries in a TAR archive.
func countTarEntries(path string) int {
	entries, err := enumerateTarImpl(path)
	if err != nil {
		return 0
	}
	return len(entries)
}

func wrapCompressedReader(f *os.File, path string) (io.Reader, func(), error) {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tgz") {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return nil, nil, fmt.Errorf("gzip error: %w", err)
		}
		return gr, func() { gr.Close() }, nil
	}
	if strings.HasSuffix(lower, ".xz") || strings.HasSuffix(lower, ".txz") {
		xr, err := xz.NewReader(f)
		if err != nil {
			return nil, nil, fmt.Errorf("xz error: %w", err)
		}
		return xr, func() {}, nil
	}
	return f, func() {}, nil
}

func readTarEntries(tr *tar.Reader) ([]ipc.EnumerateEntry, error) {
	var entries []ipc.EnumerateEntry
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return entries, nil
		}
		if err != nil {
			return nil, fmt.Errorf("tar error: %w", err)
		}
		entries = append(entries, ipc.EnumerateEntry{
			Path:      hdr.Name,
			SizeBytes: hdr.Size,
			IsDir:     hdr.Typeflag == tar.TypeDir,
		})
	}
}

func enumerateTarImpl(path string) ([]ipc.EnumerateEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader, cleanup, err := wrapCompressedReader(f, path)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	return readTarEntries(tar.NewReader(reader))
}
