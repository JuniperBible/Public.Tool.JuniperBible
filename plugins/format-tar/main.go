//go:build !sdk

// Plugin format-tar handles TAR archive ingestion.
// It detects tar, tar.gz, tar.xz files and can enumerate contents.
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

// IPCRequest is the incoming JSON request.
type IPCRequest struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args,omitempty"`
}

// IPCResponse is the outgoing JSON response.
type IPCResponse struct {
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// DetectResult is the result of a detect command.
type DetectResult struct {
	Detected bool   `json:"detected"`
	Format   string `json:"format,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// IngestResult is the result of an ingest command.
type IngestResult struct {
	ArtifactID string            `json:"artifact_id"`
	BlobSHA256 string            `json:"blob_sha256"`
	SizeBytes  int64             `json:"size_bytes"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// EnumerateResult is the result of an enumerate command.
type EnumerateResult struct {
	Entries []EnumerateEntry `json:"entries"`
}

// EnumerateEntry represents a file entry.
type EnumerateEntry struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	IsDir     bool   `json:"is_dir"`
}

func main() {
	var req IPCRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		respondError(fmt.Sprintf("failed to decode request: %v", err))
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
		respondError(fmt.Sprintf("unknown command: %s", req.Command))
	}
}

func handleDetect(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		respond(&DetectResult{
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

		respond(&DetectResult{
			Detected: true,
			Format:   "tar",
			Reason:   "tar file extension detected",
		})
		return
	}

	// Try to open as tar
	f, err := os.Open(path)
	if err != nil {
		respond(&DetectResult{
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
		respond(&DetectResult{
			Detected: true,
			Format:   "tar",
			Reason:   "valid tar header found",
		})
		return
	}

	respond(&DetectResult{
		Detected: false,
		Reason:   "not a tar file",
	})
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	// Read entire file (store verbatim)
	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	// Compute SHA-256
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	// Write to output directory
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		respondError(fmt.Sprintf("failed to create blob dir: %v", err))
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write blob: %v", err))
		return
	}

	// Detect compression type
	compression := detectCompression(path, data)

	// Count entries
	entryCount := countTarEntries(path)

	// Generate artifact ID
	base := filepath.Base(path)
	artifactID := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(
		strings.TrimSuffix(strings.TrimSuffix(base, ".tar"), ".gz"), ".xz"), ".tgz"), ".txz")
	if artifactID == "" {
		artifactID = base
	}

	respond(&IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"original_name": base,
			"format":        "tar",
			"compression":   compression,
			"entry_count":   fmt.Sprintf("%d", entryCount),
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	entries, err := enumerateTar(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to enumerate: %v", err))
		return
	}

	respond(&EnumerateResult{
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

func enumerateTar(path string) ([]EnumerateEntry, error) {
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
	var entries []EnumerateEntry

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar error: %w", err)
		}

		entries = append(entries, EnumerateEntry{
			Path:      hdr.Name,
			SizeBytes: hdr.Size,
			IsDir:     hdr.Typeflag == tar.TypeDir,
		})
	}

	return entries, nil
}

func respond(result interface{}) {
	resp := IPCResponse{
		Status: "ok",
		Result: result,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
}

func respondError(msg string) {
	resp := IPCResponse{
		Status: "error",
		Error:  msg,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
	os.Exit(1)
}
