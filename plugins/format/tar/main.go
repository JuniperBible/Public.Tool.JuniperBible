//go:build !sdk

// Plugin format-tar handles TAR archive ingestion.
// It detects tar, tar.gz, tar.xz files and can enumerate contents.
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/ulikunitz/xz"
)

func main() {
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorf("failed to decode request: %v", err)
		return
	}

	switch req.Command {
	case "detect":
		handleDetect(req.Args)
	case "ingest":
		handleIngest(req.Args)
	case "enumerate":
		handleEnumerate(req.Args)
	default:
		ipc.RespondErrorf("unknown command: %s", req.Command)
	}
}

func handleDetect(args map[string]interface{}) {
	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not a file",
		})
		return
	}

	// Check file extension first
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".tar") ||
		strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".tar.xz") ||
		strings.HasSuffix(lower, ".txz") {

		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "tar",
			Reason:   "tar file extension detected",
		})
		return
	}

	// Try to open as tar
	f, err := os.Open(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open: %v", err),
		})
		return
	}
	defer f.Close()

	// Try plain tar
	tr := tar.NewReader(f)
	_, err = tr.Next()
	if err == nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "tar",
			Reason:   "valid tar header found",
		})
		return
	}

	ipc.MustRespond(&ipc.DetectResult{
		Detected: false,
		Reason:   "not a tar file",
	})
}

func handleIngest(args map[string]interface{}) {
	path, outputDir, err := ipc.PathAndOutputDir(args)
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	// Read entire file (store verbatim)
	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	// Compute SHA-256
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	// Write to output directory
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		ipc.RespondErrorf("failed to create blob dir: %v", err)
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		ipc.RespondErrorf("failed to write blob: %v", err)
		return
	}

	// Detect compression type
	compression := detectCompression(path, data)

	// Count entries
	entryCount := countTarEntries(path)

	// Generate artifact ID
	artifactID := ipc.ArtifactIDFromPath(path)

	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"original_name": filepath.Base(path),
			"format":        "tar",
			"compression":   compression,
			"entry_count":   fmt.Sprintf("%d", entryCount),
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	entries, err := enumerateTar(path)
	if err != nil {
		ipc.RespondErrorf("failed to enumerate: %v", err)
		return
	}

	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: entries,
	})
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
	entries, err := enumerateTar(path)
	if err != nil {
		return 0
	}
	return len(entries)
}

func enumerateTar(path string) ([]ipc.EnumerateEntry, error) {
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
