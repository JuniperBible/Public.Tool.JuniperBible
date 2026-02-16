// Package tar provides the embedded handler for TAR archive format plugin.
// It implements the EmbeddedFormatHandler interface from core/plugins.
package tar

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
	"github.com/ulikunitz/xz"
)

// Handler implements the EmbeddedFormatHandler interface for TAR format.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.tar",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-tar",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"file", "directory"},
			Outputs: []string{"artifact.kind:tar-archive"},
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

	ext := strings.ToLower(filepath.Ext(path))
	baseName := strings.ToLower(filepath.Base(path))

	// Check for tar extensions
	isTar := ext == ".tar"
	isTarGz := ext == ".gz" && strings.HasSuffix(strings.TrimSuffix(baseName, ".gz"), ".tar")
	isTgz := ext == ".tgz"
	isTarXz := ext == ".xz" && strings.HasSuffix(strings.TrimSuffix(baseName, ".xz"), ".tar")
	isTxz := ext == ".txz"

	if !isTar && !isTarGz && !isTgz && !isTarXz && !isTxz {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   "not a tar archive extension",
		}, nil
	}

	// Try to open and verify it's a valid tar
	f, err := os.Open(path)
	if err != nil {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open file: %v", err),
		}, nil
	}
	defer f.Close()

	var reader io.Reader = f

	// Handle compression
	if isTarGz || isTgz {
		gzReader, err := gzip.NewReader(f)
		if err != nil {
			return &plugins.DetectResult{
				Detected: false,
				Reason:   fmt.Sprintf("not valid gzip: %v", err),
			}, nil
		}
		defer gzReader.Close()
		reader = gzReader
	} else if isTarXz || isTxz {
		xzReader, err := xz.NewReader(f)
		if err != nil {
			return &plugins.DetectResult{
				Detected: false,
				Reason:   fmt.Sprintf("not valid xz: %v", err),
			}, nil
		}
		reader = xzReader
	}

	// Try to read at least one tar header
	tarReader := tar.NewReader(reader)
	_, err = tarReader.Next()
	if err != nil {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("not a valid tar archive: %v", err),
		}, nil
	}

	format := "tar"
	if isTarGz || isTgz {
		format = "tar.gz"
	} else if isTarXz || isTxz {
		format = "tar.xz"
	}

	return &plugins.DetectResult{
		Detected: true,
		Format:   format,
		Reason:   fmt.Sprintf("valid %s archive", format),
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

	ext := strings.ToLower(filepath.Ext(path))
	baseName := strings.ToLower(filepath.Base(path))
	format := "tar"
	if ext == ".gz" || ext == ".tgz" || strings.HasSuffix(baseName, ".tar.gz") {
		format = "tar.gz"
	} else if ext == ".xz" || ext == ".txz" || strings.HasSuffix(baseName, ".tar.xz") {
		format = "tar.xz"
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	// Handle double extensions
	if strings.HasSuffix(artifactID, ".tar") {
		artifactID = strings.TrimSuffix(artifactID, ".tar")
	}

	return &plugins.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": format,
		},
	}, nil
}

// Enumerate implements EmbeddedFormatHandler.Enumerate.
func (h *Handler) Enumerate(path string) (*plugins.EnumerateResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))
	baseName := strings.ToLower(filepath.Base(path))

	var reader io.Reader = f

	// Handle compression
	isTarGz := ext == ".gz" || ext == ".tgz" || strings.HasSuffix(baseName, ".tar.gz")
	isTarXz := ext == ".xz" || ext == ".txz" || strings.HasSuffix(baseName, ".tar.xz")

	if isTarGz {
		gzReader, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	} else if isTarXz {
		xzReader, err := xz.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("failed to create xz reader: %w", err)
		}
		reader = xzReader
	}

	tarReader := tar.NewReader(reader)
	var entries []plugins.EnumerateEntry

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		entry := plugins.EnumerateEntry{
			Path:      header.Name,
			SizeBytes: header.Size,
			IsDir:     header.Typeflag == tar.TypeDir,
		}
		entries = append(entries, entry)
	}

	return &plugins.EnumerateResult{
		Entries: entries,
	}, nil
}

// ExtractIR implements EmbeddedFormatHandler.ExtractIR.
// TAR archives don't have a meaningful IR representation.
func (h *Handler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	return nil, fmt.Errorf("TAR format does not support IR extraction")
}

// EmitNative implements EmbeddedFormatHandler.EmitNative.
// TAR archives don't have a meaningful IR to emit from.
func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	return nil, fmt.Errorf("TAR format does not support native emission from IR")
}
