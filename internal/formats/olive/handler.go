// Package olive provides the embedded handler for Olive Tree Bible file detection.
// Olive Tree uses a proprietary format with OTML (Olive Tree Markup Language).
package olive

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// Handler implements the EmbeddedFormatHandler interface for Olive Tree format.
type Handler struct{}

// PDBHeader represents Palm Database (PDB) header structure - used by older Olive Tree versions
type PDBHeader struct {
	Name           [32]byte
	Attributes     uint16
	Version        uint16
	CreationTime   uint32
	ModTime        uint32
	BackupTime     uint32
	ModNumber      uint32
	AppInfoOffset  uint32
	SortInfoOffset uint32
	Type           [4]byte
	Creator        [4]byte
	UniqueIDSeed   uint32
	NextRecordList uint32
	NumRecords     uint16
}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.olive",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-olive",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"file"},
			Outputs: []string{"artifact.kind:olive"},
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

	ext := strings.ToLower(filepath.Ext(path))

	// Check known Olive Tree extensions
	validExts := map[string]string{
		".ot4i": "Olive Tree Bible (modern format)",
		".oti":  "Olive Tree Index",
		".otm":  "Olive Tree Module",
		".pdb":  "Palm Database (legacy Olive Tree)",
	}

	moduleType, isValid := validExts[ext]
	if !isValid {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("extension %s is not a known Olive Tree format", ext),
		}, nil
	}

	// For .pdb files, try to verify it's an Olive Tree PDB
	if ext == ".pdb" {
		if detected, reason := detectPDBFormat(path); !detected {
			return &plugins.DetectResult{
				Detected: false,
				Reason:   reason,
			}, nil
		}
	}

	// For modern formats (.ot4i, .oti, .otm), check if it's a valid file
	if ext == ".ot4i" || ext == ".oti" || ext == ".otm" {
		data, err := os.Open(path)
		if err != nil {
			return &plugins.DetectResult{
				Detected: false,
				Reason:   fmt.Sprintf("cannot open file: %v", err),
			}, nil
		}
		defer data.Close()

		// Read first few bytes to check for SQLite signature or other markers
		header := make([]byte, 16)
		n, err := data.Read(header)
		if err != nil || n < 16 {
			return &plugins.DetectResult{
				Detected: false,
				Reason:   "file too small or unreadable",
			}, nil
		}

		// Check for SQLite signature
		isSQLite := string(header[:15]) == "SQLite format 3"

		if isSQLite {
			moduleType = moduleType + " (SQLite-based)"
		} else {
			moduleType = moduleType + " (proprietary/encrypted)"
		}
	}

	return &plugins.DetectResult{
		Detected: true,
		Format:   "OliveTree",
		Reason:   fmt.Sprintf("Olive Tree format detected: %s (proprietary - conversion not supported)", moduleType),
	}, nil
}

// detectPDBFormat checks if a PDB file is likely an Olive Tree Bible database
func detectPDBFormat(path string) (bool, string) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Sprintf("cannot open file: %v", err)
	}
	defer file.Close()

	var header PDBHeader
	if err := binary.Read(file, binary.BigEndian, &header); err != nil {
		return false, fmt.Sprintf("failed to read PDB header: %v", err)
	}

	// Check for common Olive Tree creator IDs
	creator := string(header.Creator[:])

	// Palm databases for Bible software often use specific creator/type codes
	if strings.Contains(creator, "OlTr") || strings.Contains(creator, "BbRd") {
		return true, ""
	}

	// Check database name for Bible-related keywords
	name := string(header.Name[:])
	name = strings.TrimRight(name, "\x00")

	biblicKeywords := []string{"Bible", "Testament", "Scripture", "KJV", "NIV", "ESV", "NKJV"}
	for _, keyword := range biblicKeywords {
		if strings.Contains(name, keyword) {
			return true, ""
		}
	}

	// If we can't definitively identify it, reject
	return false, fmt.Sprintf("PDB file does not appear to be Olive Tree format (creator: %s)", creator)
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
			"format":  "OliveTree",
			"warning": "Format ingested but conversion not supported (proprietary)",
		},
	}, nil
}

// Enumerate implements EmbeddedFormatHandler.Enumerate.
func (h *Handler) Enumerate(path string) (*plugins.EnumerateResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	entries := []plugins.EnumerateEntry{
		{
			Path:      filepath.Base(path),
			SizeBytes: info.Size(),
			IsDir:     false,
			Metadata: map[string]string{
				"format":  "OliveTree",
				"warning": "Proprietary format - content enumeration not available",
			},
		},
	}
	return &plugins.EnumerateResult{
		Entries: entries,
	}, nil
}

// ExtractIR implements EmbeddedFormatHandler.ExtractIR.
func (h *Handler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	return nil, fmt.Errorf("extract-ir not supported: Olive Tree format is proprietary/encrypted. Format specification not publicly documented. Contact Olive Tree Bible Software for conversion options")
}

// EmitNative implements EmbeddedFormatHandler.EmitNative.
func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	return nil, fmt.Errorf("emit-native not supported: Olive Tree format is proprietary/encrypted. Format specification not publicly documented. Cannot generate Olive Tree files without format documentation")
}
