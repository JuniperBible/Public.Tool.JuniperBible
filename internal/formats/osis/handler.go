// Package osis provides the embedded handler for OSIS Bible format plugin.
package osis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// Handler implements the EmbeddedFormatHandler interface for OSIS Bible.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.osis",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-osis",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"file"},
			Outputs: []string{"artifact.kind:osis"},
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

	if info.IsDir() {
		return &plugins.DetectResult{Detected: false, Reason: "path is a directory, not a file"}, nil
	}

	// Read file and check for OSIS XML
	data, err := os.ReadFile(path)
	if err != nil {
		return &plugins.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot read: %v", err)}, nil
	}

	// Check for OSIS markers
	content := string(data)
	if strings.Contains(content, "<osis") && strings.Contains(content, "osisText") {
		return &plugins.DetectResult{
			Detected: true,
			Format:   "OSIS",
			Reason:   "OSIS XML detected",
		}, nil
	}

	// Check file extension as fallback
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".osis" || ext == ".xml" {
		// Try to parse as OSIS
		var doc OSISDoc
		if err := xml.Unmarshal(data, &doc); err == nil && doc.OsisText.OsisIDWork != "" {
			return &plugins.DetectResult{
				Detected: true,
				Format:   "OSIS",
				Reason:   "Valid OSIS XML structure",
			}, nil
		}
	}

	return &plugins.DetectResult{Detected: false, Reason: "not an OSIS XML file"}, nil
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

	// Generate artifact ID from work ID if possible
	var doc OSISDoc
	artifactID := filepath.Base(path)
	if err := xml.Unmarshal(data, &doc); err == nil && doc.OsisText.OsisIDWork != "" {
		artifactID = doc.OsisText.OsisIDWork
	}

	return &plugins.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"original_name": filepath.Base(path),
			"format":        "OSIS",
		},
	}, nil
}

// Enumerate implements EmbeddedFormatHandler.Enumerate.
func (h *Handler) Enumerate(path string) (*plugins.EnumerateResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	return &plugins.EnumerateResult{
		Entries: []plugins.EnumerateEntry{
			{Path: filepath.Base(path), SizeBytes: info.Size(), IsDir: false},
		},
	}, nil
}

// ExtractIR implements EmbeddedFormatHandler.ExtractIR.
func (h *Handler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	// Read OSIS file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse OSIS XML
	corpus, err := parseOSISToIR(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OSIS: %w", err)
	}

	// Serialize IR to JSON
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize IR: %w", err)
	}

	// Write IR to output directory
	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0600); err != nil {
		return nil, fmt.Errorf("failed to write IR: %w", err)
	}

	return &plugins.ExtractIRResult{
		IRPath:    irPath,
		LossClass: string(corpus.LossClass),
	}, nil
}

// EmitNative implements EmbeddedFormatHandler.EmitNative.
func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	// Read IR file
	data, err := os.ReadFile(irPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read IR file: %w", err)
	}

	// Parse IR
	var corpus ir.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return nil, fmt.Errorf("failed to parse IR: %w", err)
	}

	// Convert IR to OSIS
	osisData, err := emitOSISFromIR(&corpus)
	if err != nil {
		return nil, fmt.Errorf("failed to emit OSIS: %w", err)
	}

	// Write OSIS to output directory
	outputPath := filepath.Join(outputDir, corpus.ID+".osis")
	if err := os.WriteFile(outputPath, osisData, 0600); err != nil {
		return nil, fmt.Errorf("failed to write OSIS: %w", err)
	}

	return &plugins.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "OSIS",
		LossClass:  string(corpus.LossClass),
	}, nil
}
