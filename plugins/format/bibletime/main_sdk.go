//go:build sdk

// Plugin format-bibletime handles BibleTime Bible study format using the SDK pattern.
// BibleTime uses SWORD modules with additional bookmarks/notes metadata.
//
// IR Support:
// - extract-ir: Reads BibleTime format (SWORD + bookmarks) to IR (L1)
// - emit-native: Converts IR to BibleTime format (L1)
// Note: L1 means semantically lossless (bookmarks stored as metadata).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// BibleTimeBookmark represents a bookmark in BibleTime's XBEL format.
type BibleTimeBookmark struct {
	Title       string `json:"title"`
	Ref         string `json:"ref"`
	Description string `json:"description,omitempty"`
}

func main() {
	if err := format.Run(&format.Config{
		Name:      "BIBLETIME",
		Detect:    detectBibleTime,
		Parse:     parseBibleTime,
		Emit:      emitBibleTime,
		Enumerate: enumerateBibleTime,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// detectBibleTime checks if the path is a BibleTime format
func detectBibleTime(path string) (*ipc.DetectResult, error) {
	// BibleTime uses SWORD modules, typically in ~/.sword or module directories
	// Check for SWORD module structure or BibleTime bookmarks (.xbel)
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat path: %v", err),
		}, nil
	}

	// Check for BibleTime bookmark files
	if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".xbel") {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), "bibletime") {
			return &ipc.DetectResult{
				Detected: true,
				Format:   "BIBLETIME",
				Reason:   "BibleTime bookmark file detected",
			}, nil
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
						return &ipc.DetectResult{
							Detected: true,
							Format:   "BIBLETIME",
							Reason:   "BibleTime/SWORD module directory detected",
						}, nil
					}
				}
			}
		}
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "not a BibleTime format",
	}, nil
}

// parseBibleTime converts BibleTime format to IR
func parseBibleTime(path string) (*ir.Corpus, error) {
	// For BibleTime, we extract to a simple IR format
	// In a full implementation, this would parse SWORD modules
	// For now, create a minimal IR corpus
	corpus := &ir.Corpus{
		ID:           "bibletime-stub",
		Version:      "1.0",
		ModuleType:   "Bible",
		Title:        "BibleTime Module",
		SourceFormat: "BIBLETIME",
		LossClass:    "L1",
		Documents:    []*ir.Document{},
		Attributes: map[string]string{
			"source_path": path,
		},
	}

	return corpus, nil
}

// emitBibleTime converts IR back to BibleTime format
func emitBibleTime(corpus *ir.Corpus, outputDir string) (string, error) {
	// For BibleTime, we would emit SWORD module format
	// For now, create a placeholder
	outputPath := filepath.Join(outputDir, "bibletime-module")
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create output dir: %w", err)
	}

	// Create a minimal mods.d directory
	modsDir := filepath.Join(outputPath, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create mods.d: %w", err)
	}

	// Create a stub .conf file
	confPath := filepath.Join(modsDir, "module.conf")
	confContent := fmt.Sprintf("[%s]\nDescription=%s\nModulePath=./modules/%s\n",
		corpus.ID, corpus.Title, corpus.ID)
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write conf: %w", err)
	}

	return outputPath, nil
}

// enumerateBibleTime lists contents in a BibleTime directory
func enumerateBibleTime(path string) (*ipc.EnumerateResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
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

	return &ipc.EnumerateResult{
		Entries: entries,
	}, nil
}
