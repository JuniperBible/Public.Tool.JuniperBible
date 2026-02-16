//go:build !sdk

// Plugin format-file handles single file ingestion.
// It stores files verbatim in the CAS without any transformation.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
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
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if info.IsDir() {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "path is a directory, not a file",
		})
		return
	}

	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   "file",
		Reason:   "single file detected",
	})
}

func handleIngest(args map[string]interface{}) {
	ipc.StandardIngest(args, "file", func(path string, data []byte) map[string]string {
		return map[string]string{
			"format":        "file",
			"original_name": filepath.Base(path),
		}
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		ipc.RespondErrorf("failed to stat: %v", err)
		return
	}

	// Single file just returns itself
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: []ipc.EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
			},
		},
	})
}
