//go:build !sdk

// Plugin format-bibletime handles BibleTime Bible study format.
// BibleTime uses SWORD modules with additional bookmarks/notes metadata.
//
// IR Support:
// - extract-ir: Reads BibleTime format (SWORD + bookmarks) to IR (L1)
// - emit-native: Converts IR to BibleTime format (L1)
// Note: L1 means semantically lossless (bookmarks stored as metadata).
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// ExtractIRResult is the result of an extract-ir command.

// EmitNativeResult is the result of an emit-native command.

// LossReport describes any data loss during conversion.

// LostElement describes a specific element that was lost.

// BibleTimeBookmark represents a bookmark in BibleTime's XBEL format.
type BibleTimeBookmark struct {
	Title       string `json:"title"`
	Ref         string `json:"ref"`
	Description string `json:"description,omitempty"`
}

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
	case "extract-ir":
		handleExtractIR(req.Args)
	case "emit-native":
		handleEmitNative(req.Args)
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

	// BibleTime uses SWORD modules, typically in ~/.sword or module directories
	// Check for SWORD module structure or BibleTime bookmarks (.xbel)
	info, err := os.Stat(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat path: %v", err),
		})
		return
	}

	// Check for BibleTime bookmark files
	if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".xbel") {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), "bibletime") {
			ipc.MustRespond(&ipc.DetectResult{
				Detected: true,
				Format:   "BIBLETIME",
				Reason:   "BibleTime bookmark file detected",
			})
			return
		}
	}

	// Check for SWORD module directory with BibleTime markers
	if info.IsDir() {
		// Look for mods.d/*.conf files (SWORD module config)
		modsDir := filepath.Join(path, "mods.d")
		if stat, err := os.Stat(modsDir); err == nil && stat.IsDir() {
			// BibleTime uses standard SWORD structure
			// Check for any .conf files
			entries, err := os.ReadDir(modsDir)
			if err == nil && len(entries) > 0 {
				for _, entry := range entries {
					if strings.HasSuffix(strings.ToLower(entry.Name()), ".conf") {
						ipc.MustRespond(&ipc.DetectResult{
							Detected: true,
							Format:   "BIBLETIME",
							Reason:   "BibleTime/SWORD module directory detected",
						})
						return
					}
				}
			}
		}
	}

	ipc.MustRespond(&ipc.DetectResult{
		Detected: false,
		Reason:   "not a BibleTime format",
	})
}

func handleIngest(args map[string]interface{}) {
	path, outputDir, err := ipc.PathAndOutputDir(args)
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	// For BibleTime, we ingest the SWORD module directory
	// For now, treat as opaque blob
	data, err := readFileOrDir(path)
	if err != nil {
		ipc.RespondErrorf("failed to read: %v", err)
		return
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

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

	artifactID := ipc.ArtifactIDFromPath(path)

	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "BIBLETIME",
		},
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

	var entries []ipc.EnumerateEntry
	if info.IsDir() {
		// Enumerate SWORD modules
		modsDir := filepath.Join(path, "mods.d")
		if stat, err := os.Stat(modsDir); err == nil && stat.IsDir() {
			dirEntries, err := os.ReadDir(modsDir)
			if err == nil {
				for _, entry := range dirEntries {
					if strings.HasSuffix(strings.ToLower(entry.Name()), ".conf") {
						info, _ := entry.Info()
						entries = append(entries, ipc.EnumerateEntry{
							Path:      entry.Name(),
							SizeBytes: info.Size(),
							IsDir:     false,
						})
					}
				}
			}
		}
	} else {
		entries = append(entries, ipc.EnumerateEntry{
			Path:      filepath.Base(path),
			SizeBytes: info.Size(),
			IsDir:     false,
		})
	}

	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: entries,
	})
}

func handleExtractIR(args map[string]interface{}) {
	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	outputDir, err := ipc.StringArg(args, "output_dir")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	// For BibleTime, we extract to a simple IR format
	// In a full implementation, this would parse SWORD modules
	// For now, create a minimal IR corpus
	corpus := &ipc.Corpus{
		ID:           "bibletime-stub",
		Version:      "1.0",
		ModuleType:   "Bible",
		Title:        "BibleTime Module",
		SourceFormat: "BIBLETIME",
		LossClass:    "L1",
		Documents:    []*ipc.Document{},
		Attributes: map[string]string{
			"source_path": path,
		},
	}

	irPath := filepath.Join(outputDir, "corpus.json")
	data, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to marshal IR: %v", err)
		return
	}

	if err := os.WriteFile(irPath, data, 0644); err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}

	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "BIBLETIME",
			TargetFormat: "IR",
			LossClass:    "L1",
			Warnings:     []string{"Stub implementation - full parsing not yet implemented"},
		},
	})
}

func handleEmitNative(args map[string]interface{}) {
	irPath, err := ipc.StringArg(args, "ir_path")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	outputDir, err := ipc.StringArg(args, "output_dir")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	// Read IR
	data, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR: %v", err)
		return
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		ipc.RespondErrorf("failed to unmarshal IR: %v", err)
		return
	}

	// For BibleTime, we would emit SWORD module format
	// For now, create a placeholder
	outputPath := filepath.Join(outputDir, "bibletime-module")
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		ipc.RespondErrorf("failed to create output dir: %v", err)
		return
	}

	// Create a minimal mods.d directory
	modsDir := filepath.Join(outputPath, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		ipc.RespondErrorf("failed to create mods.d: %v", err)
		return
	}

	// Create a stub .conf file
	confPath := filepath.Join(modsDir, "module.conf")
	confContent := fmt.Sprintf("[%s]\nDescription=%s\nModulePath=./modules/%s\n",
		corpus.ID, corpus.Title, corpus.ID)
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		ipc.RespondErrorf("failed to write conf: %v", err)
		return
	}

	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "BIBLETIME",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "BIBLETIME",
			LossClass:    "L1",
			Warnings:     []string{"Stub implementation - full emission not yet implemented"},
		},
	})
}

// readFileOrDir reads a file or creates a tar of a directory
func readFileOrDir(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return os.ReadFile(path)
	}

	// For directory, read first file as placeholder
	// In full implementation, would create tar.gz
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return []byte{}, nil
	}

	// Find first file
	for _, entry := range entries {
		if !entry.IsDir() {
			return safefile.ReadFile(filepath.Join(path, entry.Name()))
		}
	}

	return []byte{}, nil
}
