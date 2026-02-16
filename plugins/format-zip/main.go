//go:build !sdk

// Plugin format-zip handles ZIP archive ingestion.
// It detects ZIP files by magic bytes and can enumerate contents.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ZIP magic bytes: PK\x03\x04
var zipMagic = []byte{0x50, 0x4b, 0x03, 0x04}

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

	f, err := os.Open(path)
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open: %v", err),
		})
		return
	}
	defer f.Close()

	// Check for directory
	info, err := f.Stat()
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if info.IsDir() {
		respond(&DetectResult{
			Detected: false,
			Reason:   "path is a directory",
		})
		return
	}

	// Read first 4 bytes to check magic
	header := make([]byte, 4)
	n, err := f.Read(header)
	if err != nil || n < 4 {
		respond(&DetectResult{
			Detected: false,
			Reason:   "file too small for ZIP",
		})
		return
	}

	// Check ZIP magic bytes
	if header[0] == zipMagic[0] && header[1] == zipMagic[1] &&
		header[2] == zipMagic[2] && header[3] == zipMagic[3] {
		respond(&DetectResult{
			Detected: true,
			Format:   "zip",
			Reason:   "ZIP magic bytes detected",
		})
		return
	}

	respond(&DetectResult{
		Detected: false,
		Reason:   "not a ZIP file (magic mismatch)",
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
	if err := os.WriteFile(blobPath, data, 0600); err != nil {
		respondError(fmt.Sprintf("failed to write blob: %v", err))
		return
	}

	// Generate artifact ID
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	artifactID := base[:len(base)-len(ext)]
	if artifactID == "" {
		artifactID = base
	}

	// Count entries
	zr, err := zip.OpenReader(path)
	entryCount := 0
	if err == nil {
		entryCount = len(zr.File)
		zr.Close()
	}

	respond(&IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"original_name": base,
			"format":        "zip",
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

	zr, err := zip.OpenReader(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to open zip: %v", err))
		return
	}
	defer zr.Close()

	var entries []EnumerateEntry
	for _, f := range zr.File {
		entries = append(entries, EnumerateEntry{
			Path:      f.Name,
			SizeBytes: int64(f.UncompressedSize64),
			IsDir:     f.FileInfo().IsDir(),
		})
	}

	respond(&EnumerateResult{
		Entries: entries,
	})
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
