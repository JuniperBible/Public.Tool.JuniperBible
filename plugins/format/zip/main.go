//go:build !sdk

// Plugin format-zip handles ZIP archive ingestion.
// It detects ZIP files by magic bytes and can enumerate contents.
package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// ZIP magic bytes: PK\x03\x04
var zipMagic = []byte{0x50, 0x4b, 0x03, 0x04}

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

	// Check if it's a directory
	info, err := os.Stat(path)
	if err != nil {
		ipc.MustRespond(ipc.DetectFailure(fmt.Sprintf("cannot stat: %v", err)))
		return
	}

	if info.IsDir() {
		ipc.MustRespond(ipc.DetectFailure("path is a directory"))
		return
	}

	// Magic byte detection
	result := ipc.DetectByMagicBytes(path, "zip", zipMagic)
	ipc.MustRespond(result)
}

func handleIngest(args map[string]interface{}) {
	ipc.StandardIngest(args, "zip", func(path string, data []byte) map[string]string {
		// Count entries
		zr, err := zip.OpenReader(path)
		entryCount := 0
		if err == nil {
			entryCount = len(zr.File)
			zr.Close()
		}

		return map[string]string{
			"format":        "zip",
			"original_name": filepath.Base(path),
			"entry_count":   fmt.Sprintf("%d", entryCount),
		}
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		ipc.RespondErrorf("failed to open zip: %v", err)
		return
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

	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: entries,
	})
}
