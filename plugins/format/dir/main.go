//go:build !sdk

// Plugin format-dir handles directory ingestion.
// It recursively enumerates and ingests all files in a directory.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

func main() {
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorfAndExit("failed to decode request: %v", err)
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
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if !info.IsDir() {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "path is not a directory",
		})
		return
	}

	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   "dir",
		Reason:   "directory detected",
	})
}

func handleIngest(args map[string]interface{}) {
	path, outputDir, err := ipc.PathAndOutputDir(args)
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	// For directories, we create a manifest of all files
	// and store each file individually
	var files []string
	var totalSize int64

	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
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
		ipc.RespondErrorf("failed to walk directory: %v", err)
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
		rel, err := filepath.Rel(path, f)
		if err != nil {
			ipc.RespondErrorf("failed to compute relative path for %s: %v", f, err)
			return
		}
		manifest.Files[i] = rel
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		ipc.RespondErrorf("failed to marshal manifest: %v", err)
		return
	}

	hashHex, err := ipc.StoreBlob(outputDir, manifestData)
	if err != nil {
		ipc.RespondErrorf("failed to store blob: %v", err)
		return
	}

	ipc.MustRespond(&ipc.IngestResult{
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
	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	var entries []ipc.EnumerateEntry

	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(path, p)
		if relErr != nil {
			return fmt.Errorf("failed to compute relative path: %w", relErr)
		}
		if rel == "." {
			return nil // Skip root
		}

		entries = append(entries, ipc.EnumerateEntry{
			Path:      rel,
			SizeBytes: info.Size(),
			IsDir:     info.IsDir(),
		})
		return nil
	})

	if err != nil {
		ipc.RespondErrorf("failed to enumerate: %v", err)
		return
	}

	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: entries,
	})
}
