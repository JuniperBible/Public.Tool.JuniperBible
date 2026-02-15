// Package bibletime provides the embedded handler for BibleTime Bible study format.
// BibleTime uses SWORD modules with additional bookmarks/notes metadata.
package bibletime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
)

// Handler implements the EmbeddedFormatHandler interface for BibleTime format.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.bibletime",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-bibletime",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"file", "directory"},
			Outputs: []string{"artifact.kind:bibletime"},
		},
	}
}

// Register registers this plugin with the embedded registry.
func Register() {
	plugins.RegisterEmbeddedPlugin(&plugins.EmbeddedPlugin{
		Manifest: Manifest(),
		Format:   &Handler{},
	})
}

func init() {
	Register()
}

// Detect implements EmbeddedFormatHandler.Detect.
func (h *Handler) Detect(path string) (*plugins.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &plugins.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}, nil
	}

	// Check for BibleTime bookmark files
	if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".xbel") {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), "bibletime") {
			return &plugins.DetectResult{
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
						return &plugins.DetectResult{
							Detected: true,
							Format:   "BIBLETIME",
							Reason:   "BibleTime/SWORD module directory detected",
						}, nil
					}
				}
			}
		}
	}

	return &plugins.DetectResult{
		Detected: false,
		Reason:   "not a BibleTime format",
	}, nil
}

// Ingest implements EmbeddedFormatHandler.Ingest.
func (h *Handler) Ingest(path, outputDir string) (*plugins.IngestResult, error) {
	// For BibleTime, we ingest the SWORD module directory
	// For now, treat as opaque blob
	data, err := readFileOrDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read: %w", err)
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create blob dir: %w", err)
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write blob: %w", err)
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return &plugins.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "BIBLETIME",
		},
	}, nil
}

// Enumerate implements EmbeddedFormatHandler.Enumerate.
func (h *Handler) Enumerate(path string) (*plugins.EnumerateResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	var entries []plugins.EnumerateEntry
	if info.IsDir() {
		// Enumerate SWORD modules
		modsDir := filepath.Join(path, "mods.d")
		if stat, err := os.Stat(modsDir); err == nil && stat.IsDir() {
			dirEntries, err := os.ReadDir(modsDir)
			if err == nil {
				for _, entry := range dirEntries {
					if strings.HasSuffix(strings.ToLower(entry.Name()), ".conf") {
						info, _ := entry.Info()
						entries = append(entries, plugins.EnumerateEntry{
							Path:      entry.Name(),
							SizeBytes: info.Size(),
							IsDir:     false,
						})
					}
				}
			}
		}
	} else {
		entries = append(entries, plugins.EnumerateEntry{
			Path:      filepath.Base(path),
			SizeBytes: info.Size(),
			IsDir:     false,
		})
	}

	return &plugins.EnumerateResult{
		Entries: entries,
	}, nil
}

// ExtractIR implements EmbeddedFormatHandler.ExtractIR.
func (h *Handler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	// For BibleTime, we extract to a simple IR format
	// In a full implementation, this would parse SWORD modules
	// For now, create a minimal IR corpus
	corpus := map[string]interface{}{
		"id":            "bibletime-stub",
		"version":       "1.0",
		"module_type":   "Bible",
		"title":         "BibleTime Module",
		"source_format": "BIBLETIME",
		"loss_class":    "L1",
		"documents":     []interface{}{},
		"attributes": map[string]string{
			"source_path": path,
		},
	}

	irPath := filepath.Join(outputDir, "corpus.json")
	data, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal IR: %w", err)
	}

	if err := os.WriteFile(irPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write IR: %w", err)
	}

	return &plugins.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "BIBLETIME",
			TargetFormat: "IR",
			LossClass:    "L1",
			Warnings:     []string{"Stub implementation - full parsing not yet implemented"},
		},
	}, nil
}

// EmitNative implements EmbeddedFormatHandler.EmitNative.
func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	// Read IR
	data, err := os.ReadFile(irPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read IR: %w", err)
	}

	var corpus map[string]interface{}
	if err := json.Unmarshal(data, &corpus); err != nil {
		return nil, fmt.Errorf("failed to unmarshal IR: %w", err)
	}

	// For BibleTime, we would emit SWORD module format
	// For now, create a placeholder
	outputPath := filepath.Join(outputDir, "bibletime-module")
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output dir: %w", err)
	}

	// Create a minimal mods.d directory
	modsDir := filepath.Join(outputPath, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create mods.d: %w", err)
	}

	// Create a stub .conf file
	confPath := filepath.Join(modsDir, "module.conf")
	id := "unknown"
	title := "Unknown"
	if v, ok := corpus["id"].(string); ok {
		id = v
	}
	if v, ok := corpus["title"].(string); ok {
		title = v
	}

	confContent := fmt.Sprintf("[%s]\nDescription=%s\nModulePath=./modules/%s\n", id, title, id)
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write conf: %w", err)
	}

	return &plugins.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "BIBLETIME",
		LossClass:  "L1",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "IR",
			TargetFormat: "BIBLETIME",
			LossClass:    "L1",
			Warnings:     []string{"Stub implementation - full emission not yet implemented"},
		},
	}, nil
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
