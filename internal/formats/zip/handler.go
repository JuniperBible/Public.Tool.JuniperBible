// Package zip provides the embedded handler for ZIP archive format plugin.
// It implements the EmbeddedFormatHandler interface from core/plugins.
package zip

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// Handler implements the EmbeddedFormatHandler interface for ZIP format.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.zip",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-zip",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"file"},
			Outputs: []string{"artifact.kind:zip-archive"},
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

// init automatically registers this plugin when the package is imported.
func init() {
	Register()
}

// Detect implements EmbeddedFormatHandler.Detect.
func (h *Handler) Detect(path string) (*plugins.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		}, nil
	}

	if info.IsDir() {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   "path is a directory",
		}, nil
	}

	// Check magic bytes: PK\x03\x04
	f, err := os.Open(path)
	if err != nil {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open file: %v", err),
		}, nil
	}
	defer f.Close()

	magic := make([]byte, 4)
	n, err := f.Read(magic)
	if err != nil || n < 4 {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   "cannot read magic bytes",
		}, nil
	}

	// ZIP magic: PK\x03\x04
	if magic[0] != 0x50 || magic[1] != 0x4b || magic[2] != 0x03 || magic[3] != 0x04 {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   "not a ZIP file (wrong magic bytes)",
		}, nil
	}

	// Verify it's actually readable as ZIP
	_, err = zip.OpenReader(path)
	if err != nil {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("not a valid ZIP archive: %v", err),
		}, nil
	}

	return &plugins.DetectResult{
		Detected: true,
		Format:   "zip",
		Reason:   "valid ZIP archive",
	}, nil
}

// Ingest implements EmbeddedFormatHandler.Ingest.
func (h *Handler) Ingest(path, outputDir string) (*plugins.IngestResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create blob dir: %w", err)
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write blob: %w", err)
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	return &plugins.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "zip",
		},
	}, nil
}

// Enumerate implements EmbeddedFormatHandler.Enumerate.
func (h *Handler) Enumerate(path string) (*plugins.EnumerateResult, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open ZIP: %w", err)
	}
	defer reader.Close()

	var entries []plugins.EnumerateEntry

	for _, f := range reader.File {
		entry := plugins.EnumerateEntry{
			Path:      f.Name,
			SizeBytes: int64(f.UncompressedSize64),
			IsDir:     f.FileInfo().IsDir(),
		}
		entries = append(entries, entry)
	}

	return &plugins.EnumerateResult{
		Entries: entries,
	}, nil
}

// ExtractIR implements EmbeddedFormatHandler.ExtractIR.
// ZIP archives don't have a meaningful IR representation.
func (h *Handler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	return nil, fmt.Errorf("ZIP format does not support IR extraction")
}

// EmitNative implements EmbeddedFormatHandler.EmitNative.
// ZIP archives don't have a meaningful IR to emit from.
func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	return nil, fmt.Errorf("ZIP format does not support native emission from IR")
}
