package web

import (
	"path/filepath"
	"strings"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/plugins"
)

// FormatDetector provides a unified interface for detecting various format types.
type FormatDetector struct {
	loader *plugins.Loader
}

// NewFormatDetector creates a new format detector with plugin support.
func NewFormatDetector(loader *plugins.Loader) *FormatDetector {
	return &FormatDetector{loader: loader}
}

// DetectFileFormat detects the format of a file using plugins and extension fallback.
func (fd *FormatDetector) DetectFileFormat(path string) string {
	// Try plugin-based detection first
	if fd.loader != nil {
		for _, plugin := range fd.loader.GetPluginsByKind("format") {
			req := plugins.NewDetectRequest(path)
			resp, err := plugins.ExecutePlugin(plugin, req)
			if err != nil {
				continue
			}
			result, err := plugins.ParseDetectResult(resp)
			if err != nil {
				continue
			}
			if result.Detected {
				return strings.TrimPrefix(plugin.Manifest.PluginID, "format.")
			}
		}
	}

	// Fallback to extension-based detection
	return fd.DetectByExtension(path)
}

// DetectByExtension detects format based solely on file extension.
func (fd *FormatDetector) DetectByExtension(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if format, ok := fileExtensionFormats[ext]; ok {
		return format
	}
	return "file"
}

// DetectCapsuleFormat detects the compression format of a capsule archive.
func (fd *FormatDetector) DetectCapsuleFormat(path string) string {
	switch {
	case strings.HasSuffix(path, ".tar.xz"):
		return "tar.xz"
	case strings.HasSuffix(path, ".tar.gz"):
		return "tar.gz"
	case strings.HasSuffix(path, ".tar"):
		return "tar"
	default:
		return "unknown"
	}
}

// DetectSourceFormat detects the source format from a capsule path.
// It checks the manifest first, then falls back to filename heuristics.
func (fd *FormatDetector) DetectSourceFormat(capsulePath string, manifest *CapsuleManifest) string {
	// Try manifest first
	if manifest != nil && manifest.SourceFormat != "" {
		return manifest.SourceFormat
	}

	// Check filename for hints
	name := strings.ToLower(filepath.Base(capsulePath))
	switch {
	case strings.Contains(name, "sword") || strings.HasSuffix(name, ".sword.tar.gz"):
		return "sword"
	case strings.Contains(name, "osis"):
		return "osis"
	case strings.Contains(name, "usfm"):
		return "usfm"
	default:
		return "unknown"
	}
}

// detectFileFormat is a package-level wrapper for backward compatibility.
func detectFileFormat(path string) string {
	loader := getLoader()
	detector := NewFormatDetector(loader)
	return detector.DetectFileFormat(path)
}

// detectFileFormatByExtension is a package-level wrapper for backward compatibility.
func detectFileFormatByExtension(path string) string {
	detector := NewFormatDetector(nil)
	return detector.DetectByExtension(path)
}

// detectCapsuleFormat is a package-level wrapper for backward compatibility.
func detectCapsuleFormat(path string) string {
	detector := NewFormatDetector(nil)
	return detector.DetectCapsuleFormat(path)
}

// detectSourceFormat is a package-level wrapper for backward compatibility.
func detectSourceFormat(capsulePath string) string {
	fullPath := filepath.Join(ServerConfig.CapsulesDir, capsulePath)
	manifest := readCapsuleManifest(fullPath)
	detector := NewFormatDetector(nil)
	return detector.DetectSourceFormat(capsulePath, manifest)
}
