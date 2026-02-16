//go:build !standalone

package dbl

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// embeddedHandler adapts the SDK Config to the EmbeddedFormatHandler interface.
type embeddedHandler struct{}

func (h *embeddedHandler) Detect(path string) (*plugins.DetectResult, error) {
	result, err := detectDBL(path)
	if err != nil {
		return nil, err
	}
	return &plugins.DetectResult{
		Detected: result.Detected,
		Format:   result.Format,
		Reason:   result.Reason,
	}, nil
}

func (h *embeddedHandler) Ingest(path, outputDir string) (*plugins.IngestResult, error) {
	data, metadata, err := Config.IngestTransform(path)
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
		Metadata:   metadata,
	}, nil
}

func (h *embeddedHandler) Enumerate(path string) (*plugins.EnumerateResult, error) {
	result, err := enumerateDBL(path)
	if err != nil {
		return nil, err
	}

	entries := make([]plugins.EnumerateEntry, len(result.Entries))
	for i, e := range result.Entries {
		entries[i] = plugins.EnumerateEntry{
			Path:      e.Path,
			SizeBytes: e.SizeBytes,
			IsDir:     e.IsDir,
			Metadata:  e.Metadata,
		}
	}

	return &plugins.EnumerateResult{
		Entries: entries,
	}, nil
}

func (h *embeddedHandler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	// TODO: Implement DBL to IR conversion
	return nil, fmt.Errorf("DBL IR extraction not yet implemented in canonical package")
}

func (h *embeddedHandler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	// TODO: Implement IR to DBL conversion
	return nil, fmt.Errorf("DBL native emission not yet implemented in canonical package")
}

func init() {
	plugins.RegisterEmbeddedPlugin(&plugins.EmbeddedPlugin{
		Manifest: &plugins.PluginManifest{
			PluginID:   "format.dbl",
			Version:    "1.0.0",
			Kind:       "format",
			Entrypoint: "format-dbl",
			Capabilities: plugins.Capabilities{
				Inputs:  []string{"file"},
				Outputs: []string{"artifact.kind:dbl"},
			},
		},
		Format: &embeddedHandler{},
	})
}
