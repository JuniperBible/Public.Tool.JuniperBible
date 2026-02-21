// Package olive implements detection for Olive Tree Bible format.
// Olive Tree uses a proprietary format with OTML (Olive Tree Markup Language).
// Known extensions: .ot4i, .oti, .pdb (Palm Database), .otm
//
// IR Support:
// - extract-ir: Not supported (proprietary/encrypted format)
// - emit-native: Not supported (format specification not publicly documented)
//
// The Olive Tree Bible Software uses encrypted SQLite databases and proprietary
// binary formats. While detection is provided for file identification, full
// parsing and conversion is not possible without format documentation from
// Olive Tree Bible Software.
package olive

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/format"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/ir"
)

// Config defines the Olive Tree format plugin configuration.
var Config = &format.Config{
	PluginID:   "format.olive",
	Name:       "OliveTree",
	Extensions: []string{".ot4i", ".oti", ".otm", ".pdb"},
	Detect:     detect,
	Parse:      nil, // Not supported - proprietary/encrypted format
	Emit:       nil, // Not supported - format specification not publicly documented
}

// Palm Database (PDB) header structure - used by older Olive Tree versions
type PDBHeader struct {
	Name           [32]byte // Database name
	Attributes     uint16   // File attributes
	Version        uint16   // File version
	CreationTime   uint32   // Creation time (seconds since 1904)
	ModTime        uint32   // Modification time
	BackupTime     uint32   // Backup time
	ModNumber      uint32   // Modification number
	AppInfoOffset  uint32   // Offset to app info
	SortInfoOffset uint32   // Offset to sort info
	Type           [4]byte  // Database type
	Creator        [4]byte  // Creator ID
	UniqueIDSeed   uint32   // Unique ID seed
	NextRecordList uint32   // Next record list ID
	NumRecords     uint16   // Number of records
}

var knownExts = map[string]string{
	".ot4i": "Olive Tree Bible (modern format)",
	".oti":  "Olive Tree Index",
	".otm":  "Olive Tree Module",
	".pdb":  "Palm Database (legacy Olive Tree)",
}

var modernExts = map[string]bool{
	".ot4i": true,
	".oti":  true,
	".otm":  true,
}

func validatePath(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Sprintf("cannot stat: %v", err), false
	}
	if info.IsDir() {
		return "path is a directory, not a file", false
	}
	return "", true
}

func detectModernFormat(path, moduleType string) (string, string) {
	data, err := os.Open(path)
	if err != nil {
		return "", fmt.Sprintf("cannot open file: %v", err)
	}
	defer data.Close()

	header := make([]byte, 16)
	n, err := data.Read(header)
	if err != nil || n < 16 {
		return "", "file too small or unreadable"
	}

	if string(header[:15]) == "SQLite format 3" {
		return moduleType + " (SQLite-based)", ""
	}
	return moduleType + " (proprietary/encrypted)", ""
}

func detect(path string) (*ipc.DetectResult, error) {
	if reason, ok := validatePath(path); !ok {
		return &ipc.DetectResult{Detected: false, Reason: reason}, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	moduleType, isValid := knownExts[ext]
	if !isValid {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("extension %s is not a known Olive Tree format", ext),
		}, nil
	}

	if ext == ".pdb" {
		if detected, reason := detectPDBFormat(path); !detected {
			return &ipc.DetectResult{Detected: false, Reason: reason}, nil
		}
	}

	if modernExts[ext] {
		updated, reason := detectModernFormat(path, moduleType)
		if reason != "" {
			return &ipc.DetectResult{Detected: false, Reason: reason}, nil
		}
		moduleType = updated
	}

	return &ipc.DetectResult{
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
	// Note: These are example IDs - actual values would need reverse engineering
	creator := string(header.Creator[:])
	dbType := string(header.Type[:])

	// Palm databases for Bible software often use specific creator/type codes
	// This is a heuristic - actual detection would need known signatures
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
	return false, fmt.Sprintf("PDB file does not appear to be Olive Tree format (creator: %s, type: %s)", creator, dbType)
}

// Parse is not implemented for Olive Tree format (proprietary/encrypted)
func parse(path string) (*ir.Corpus, error) {
	return nil, fmt.Errorf("extract-ir not supported: Olive Tree format is proprietary/encrypted. Format specification not publicly documented. Contact Olive Tree Bible Software for conversion options")
}

// Emit is not implemented for Olive Tree format (proprietary/encrypted)
func emit(corpus *ir.Corpus, outputDir string) (string, error) {
	return "", fmt.Errorf("emit-native not supported: Olive Tree format is proprietary/encrypted. Format specification not publicly documented. Cannot generate Olive Tree files without format documentation")
}
