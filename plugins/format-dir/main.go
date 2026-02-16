//go:build !sdk

// Plugin format-dir handles directory ingestion.
// It recursively enumerates and ingests all files in a directory.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if !info.IsDir() {
		respond(&DetectResult{
			Detected: false,
			Reason:   "path is not a directory",
		})
		return
	}

	respond(&DetectResult{
		Detected: true,
		Format:   "dir",
		Reason:   "directory detected",
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

	// For directories, we create a manifest of all files
	// and store each file individually
	var files []string
	var totalSize int64

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, p)
			totalSize += info.Size()
		}
		return nil
	})
	if err != nil {
		respondError(fmt.Sprintf("failed to walk directory: %v", err))
		return
	}

	// Create a simple manifest blob listing all files
	manifest := struct {
		RootPath string   `json:"root_path"`
		Files    []string `json:"files"`
	}{
		RootPath: filepath.Base(path),
		Files:    make([]string, len(files)),
	}

	for i, f := range files {
		rel, _ := filepath.Rel(path, f)
		manifest.Files[i] = rel
	}

	manifestData, _ := json.Marshal(manifest)
	hash := sha256.Sum256(manifestData)
	hashHex := hex.EncodeToString(hash[:])

	// Write manifest blob
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		respondError(fmt.Sprintf("failed to create blob dir: %v", err))
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, manifestData, 0600); err != nil {
		respondError(fmt.Sprintf("failed to write blob: %v", err))
		return
	}

	respond(&IngestResult{
		ArtifactID: filepath.Base(path),
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(manifestData)),
		Metadata: map[string]string{
			"format":      "dir",
			"file_count":  fmt.Sprintf("%d", len(files)),
			"total_bytes": fmt.Sprintf("%d", totalSize),
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	var entries []EnumerateEntry

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(path, p)
		if rel == "." {
			return nil // Skip root
		}

		entries = append(entries, EnumerateEntry{
			Path:      rel,
			SizeBytes: info.Size(),
			IsDir:     info.IsDir(),
		})
		return nil
	})

	if err != nil {
		respondError(fmt.Sprintf("failed to enumerate: %v", err))
		return
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
