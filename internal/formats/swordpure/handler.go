// Package swordpure provides the embedded handler for pure Go SWORD format plugin.
package swordpure

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/plugins"
)

// Handler implements the EmbeddedFormatHandler interface for SWORD (pure Go).
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.sword-pure",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-sword-pure",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"dir"},
			Outputs: []string{"artifact.kind:sword-module"},
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
// Detects SWORD module directories by looking for mods.d/*.conf files.
func (h *Handler) Detect(path string) (*plugins.DetectResult, error) {
	if err := checkIsDirectory(path); err != nil {
		return err, nil
	}

	modsDir := filepath.Join(path, "mods.d")
	if err := checkModsDir(modsDir); err != nil {
		return err, nil
	}

	return checkConfFiles(modsDir)
}

func checkIsDirectory(path string) *plugins.DetectResult {
	info, err := os.Stat(path)
	if err != nil {
		return &plugins.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}
	}
	if !info.IsDir() {
		return &plugins.DetectResult{Detected: false, Reason: "not a directory"}
	}
	return nil
}

func checkModsDir(modsDir string) *plugins.DetectResult {
	info, err := os.Stat(modsDir)
	if err != nil || !info.IsDir() {
		return &plugins.DetectResult{Detected: false, Reason: "no mods.d directory"}
	}
	return nil
}

func checkConfFiles(modsDir string) (*plugins.DetectResult, error) {
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return &plugins.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot read mods.d: %v", err)}, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".conf") {
			return &plugins.DetectResult{
				Detected: true,
				Format:   "sword-pure",
				Reason:   "SWORD module directory detected (mods.d/*.conf found)",
			}, nil
		}
	}

	return &plugins.DetectResult{Detected: false, Reason: "no .conf files in mods.d"}, nil
}

// Ingest implements EmbeddedFormatHandler.Ingest.
func (h *Handler) Ingest(path, outputDir string) (*plugins.IngestResult, error) {
	// For SWORD directories, we create a hash of all files
	var totalSize int64
	var allData []byte

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			allData = append(allData, data...)
			totalSize += info.Size()
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	hash := sha256.Sum256(allData)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create blob dir: %w", err)
	}

	// Store the directory as a tar archive (for simplicity, just note the hash)
	artifactID := filepath.Base(path)
	return &plugins.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  totalSize,
		Metadata:   map[string]string{"format": "sword-pure"},
	}, nil
}

// Enumerate implements EmbeddedFormatHandler.Enumerate.
func (h *Handler) Enumerate(path string) (*plugins.EnumerateResult, error) {
	var entries []plugins.EnumerateEntry

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(path, p)
		if relPath == "." {
			return nil
		}
		entries = append(entries, plugins.EnumerateEntry{
			Path:      relPath,
			SizeBytes: info.Size(),
			IsDir:     info.IsDir(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate: %w", err)
	}

	return &plugins.EnumerateResult{Entries: entries}, nil
}

// ExtractIR implements EmbeddedFormatHandler.ExtractIR.
func (h *Handler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create output dir: %w", err)
	}

	// Determine the SWORD root directory from the path
	// The path might be:
	// 1. A conf file path (e.g., /path/to/mods.d/kjv.conf)
	// 2. A SWORD root directory (e.g., /path/to/sword)
	swordRoot := path
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}
	if !info.IsDir() {
		// path is a conf file - go up to the SWORD root
		// /path/to/mods.d/kjv.conf -> /path/to
		swordRoot = filepath.Dir(filepath.Dir(path))
	}

	confs, err := LoadModulesFromPath(swordRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load modules: %w", err)
	}

	var irPath string
	for _, conf := range confs {
		result := processModule(conf, swordRoot, outputDir)
		if result["status"] == "ok" {
			if p, ok := result["ir_path"].(string); ok && irPath == "" {
				irPath = p
			}
		}
	}

	if irPath == "" {
		return nil, fmt.Errorf("no modules could be processed")
	}

	return &plugins.ExtractIRResult{IRPath: irPath, LossClass: "L1"}, nil
}

// processModule processes a single SWORD module for IR extraction.
func processModule(conf *ConfFile, path, outputDir string) map[string]interface{} {
	if skipReason := shouldSkipModule(conf); skipReason != "" {
		return map[string]interface{}{"module": conf.ModuleName, "status": "skipped", "reason": skipReason}
	}
	zt, err := OpenZTextModule(conf, path)
	if err != nil {
		return map[string]interface{}{"module": conf.ModuleName, "status": "error", "error": err.Error()}
	}
	corpus, stats, err := extractCorpus(zt, conf)
	if err != nil {
		return map[string]interface{}{"module": conf.ModuleName, "status": "error", "error": err.Error()}
	}
	irPath := filepath.Join(outputDir, conf.ModuleName+".ir.json")
	if err := writeCorpusJSON(corpus, irPath); err != nil {
		return map[string]interface{}{"module": conf.ModuleName, "status": "error", "error": fmt.Sprintf("failed to write IR: %v", err)}
	}
	return map[string]interface{}{
		"module": conf.ModuleName, "status": "ok", "ir_path": irPath,
		"documents": stats.Documents, "verses": stats.Verses, "tokens": stats.Tokens,
		"annotations": stats.Annotations, "loss_class": string(corpus.LossClass),
	}
}

// shouldSkipModule returns a reason to skip the module, or empty string to process.
// Note: Encrypted modules are now supported via Sapphire II cipher decryption.
func shouldSkipModule(conf *ConfFile) string {
	if conf.ModuleType() != "Bible" || !conf.IsCompressed() {
		return fmt.Sprintf("unsupported type: %s/%s", conf.ModuleType(), conf.ModDrv)
	}
	return ""
}

// EmitNative implements EmbeddedFormatHandler.EmitNative.
func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	// Load IR corpus
	data, err := os.ReadFile(irPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read IR file: %w", err)
	}

	var corpus IRCorpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return nil, fmt.Errorf("failed to parse IR: %w", err)
	}

	// Use EmitZText for full binary generation
	_, err = EmitZText(&corpus, outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to emit zText: %w", err)
	}

	return &plugins.EmitNativeResult{
		OutputPath: outputDir,
		Format:     "sword-pure",
		LossClass:  "L1",
	}, nil
}
