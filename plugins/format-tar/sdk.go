// Plugin format-tar handles TAR archive ingestion.
// It detects tar, tar.gz, tar.xz files and can enumerate contents.
package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/ulikunitz/xz"
)

func runSDK() {
	if err := format.Run(&format.Config{
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

			// Generate artifact ID
			base := filepath.Base(path)
			return data, map[string]string{
				"original_name": base,
				"format":        "tar",
				"compression":   compression,
				"entry_count":   fmt.Sprintf("%d", entryCount),
			}, nil
		},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectTAR(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not a file",
		}, nil
	}

	// Check file extension first
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".tar") ||
		strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".tar.xz") ||
		strings.HasSuffix(lower, ".txz") {

		return &ipc.DetectResult{
			Detected: true,
			Format:   "tar",
			Reason:   "tar file extension detected",
		}, nil
	}

	// Try to open as tar
	f, err := os.Open(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open: %v", err),
		}, nil
	}
	defer f.Close()

	// Try plain tar
	tr := tar.NewReader(f)
	_, err = tr.Next()
	if err == nil {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "tar",
			Reason:   "valid tar header found",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "not a tar file",
	}, nil
}

func enumerateTAR(path string) (*ipc.EnumerateResult, error) {
	entries, err := enumerateTarEntries(path)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate: %w", err)
	}

	return &ipc.EnumerateResult{
		Entries: entries,
	}, nil
}

func detectCompression(path string, data []byte) string {
	lower := strings.ToLower(path)

	if strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tgz") {
		return "gzip"
	}
	if strings.HasSuffix(lower, ".xz") || strings.HasSuffix(lower, ".txz") {
		return "xz"
	}

	// Check magic bytes
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		return "gzip"
	}
	if len(data) >= 6 && data[0] == 0xfd && string(data[1:6]) == "7zXZ\x00" {
		return "xz"
	}

	return "none"
}

func countTarEntries(path string) int {
	entries, err := enumerateTarEntries(path)
	if err != nil {
		return 0
	}
	return len(entries)
}

func enumerateTarEntries(path string) ([]ipc.EnumerateEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reader io.Reader = f

	// Detect and handle compression
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tgz") {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("gzip error: %w", err)
		}
		defer gr.Close()
		reader = gr
	} else if strings.HasSuffix(lower, ".xz") || strings.HasSuffix(lower, ".txz") {
		xr, err := xz.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("xz error: %w", err)
		}
		reader = xr
	}

	tr := tar.NewReader(reader)
	var entries []ipc.EnumerateEntry

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
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

	return entries, nil
}
