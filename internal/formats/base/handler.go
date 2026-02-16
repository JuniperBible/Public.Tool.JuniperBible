// Package base provides common functionality and utilities for format handlers.
// It reduces code duplication by abstracting common patterns found across
// different format handlers.
package base

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// DetectConfig contains configuration for format detection.
type DetectConfig struct {
	// Extensions is a list of valid file extensions (e.g., ".osis", ".xml")
	Extensions []string
	// ContentMarkers are strings that must be present in the file content
	ContentMarkers []string
	// FormatName is the name to return in DetectResult
	FormatName string
	// CheckContent determines if the file content should be read for detection
	CheckContent bool
	// CustomValidator is an optional function for additional validation
	CustomValidator func(path string, data []byte) (bool, string, error)
}

// IngestConfig contains configuration for file ingestion.
type IngestConfig struct {
	// FormatName is stored in metadata
	FormatName string
	// ArtifactIDExtractor is a function that extracts the artifact ID from data
	// If nil, uses the base filename without extension
	ArtifactIDExtractor func(path string, data []byte) string
	// AdditionalMetadata is merged into the result metadata
	AdditionalMetadata map[string]string
}

// FileInfo holds common file information used across methods.
type FileInfo struct {
	Path      string
	Data      []byte
	Hash      string
	Size      int64
	Extension string
}

// DetectFile performs common file detection logic.
// It checks if the path exists, is a file, has the right extension,
// and optionally validates content.
func DetectFile(path string, config DetectConfig) (*plugins.DetectResult, error) {
	// Check if path exists and is a file
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
			Reason:   "path is a directory, not a file",
		}, nil
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	extensionMatch := false
	for _, validExt := range config.Extensions {
		if ext == strings.ToLower(validExt) {
			extensionMatch = true
			break
		}
	}

	// If we need to check content or have content markers
	if config.CheckContent || len(config.ContentMarkers) > 0 {
		data, err := os.ReadFile(path)
		if err != nil {
			return &plugins.DetectResult{
				Detected: false,
				Reason:   fmt.Sprintf("cannot read: %v", err),
			}, nil
		}

		content := string(data)

		// Check for content markers
		if len(config.ContentMarkers) > 0 {
			allMarkersFound := true
			for _, marker := range config.ContentMarkers {
				if !strings.Contains(content, marker) {
					allMarkersFound = false
					break
				}
			}

			if allMarkersFound {
				return &plugins.DetectResult{
					Detected: true,
					Format:   config.FormatName,
					Reason:   fmt.Sprintf("%s markers detected", config.FormatName),
				}, nil
			}
		}

		// Run custom validator if provided
		if config.CustomValidator != nil {
			detected, reason, err := config.CustomValidator(path, data)
			if err != nil {
				return &plugins.DetectResult{
					Detected: false,
					Reason:   fmt.Sprintf("validation error: %v", err),
				}, nil
			}
			if detected {
				return &plugins.DetectResult{
					Detected: true,
					Format:   config.FormatName,
					Reason:   reason,
				}, nil
			}
		}
	}

	// Check if extension matched
	if extensionMatch {
		return &plugins.DetectResult{
			Detected: true,
			Format:   config.FormatName,
			Reason:   fmt.Sprintf("%s file extension detected", config.FormatName),
		}, nil
	}

	return &plugins.DetectResult{
		Detected: false,
		Reason:   fmt.Sprintf("not a %s file", config.FormatName),
	}, nil
}

// IngestFile performs common file ingestion logic.
// It reads the file, computes SHA256 hash, stores as content-addressed blob,
// and returns the ingest result.
func IngestFile(path, outputDir string, config IngestConfig) (*plugins.IngestResult, error) {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Compute SHA256 hash
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	// Create blob directory (first 2 chars of hash)
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create blob dir: %w", err)
	}

	// Write blob
	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write blob: %w", err)
	}

	// Extract artifact ID
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if config.ArtifactIDExtractor != nil {
		artifactID = config.ArtifactIDExtractor(path, data)
	}

	// Build metadata
	metadata := map[string]string{
		"original_name": filepath.Base(path),
		"format":        config.FormatName,
	}
	for k, v := range config.AdditionalMetadata {
		metadata[k] = v
	}

	return &plugins.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata:   metadata,
	}, nil
}

// EnumerateFile performs common file enumeration logic.
// It returns a single-entry result for a file.
func EnumerateFile(path string, metadata map[string]string) (*plugins.EnumerateResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	return &plugins.EnumerateResult{
		Entries: []plugins.EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
				Metadata:  metadata,
			},
		},
	}, nil
}

// ReadFileInfo reads a file and returns common file information.
func ReadFileInfo(path string) (*FileInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	hash := sha256.Sum256(data)

	return &FileInfo{
		Path:      path,
		Data:      data,
		Hash:      hex.EncodeToString(hash[:]),
		Size:      int64(len(data)),
		Extension: filepath.Ext(path),
	}, nil
}

// WriteOutput writes data to a file in the output directory with the given name.
func WriteOutput(outputDir, filename string, data []byte) (string, error) {
	outputPath := filepath.Join(outputDir, filename)
	if err := os.WriteFile(outputPath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	return outputPath, nil
}

// UnsupportedOperationError returns a standard error for unsupported operations.
func UnsupportedOperationError(operation, format string) error {
	return fmt.Errorf("%s format does not support %s", format, operation)
}
