//go:build !sdk

// Plugin format-olive handles Olive Tree Bible file detection.
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
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"os"
	"path/filepath"
	"strings"
)

// Palm Database (PDB) header structure - used by older Olive Tree versions
type PDBHeader struct {
	Name          [32]byte // Database name
	Attributes    uint16   // File attributes
	Version       uint16   // File version
	CreationTime  uint32   // Creation time (seconds since 1904)
	ModTime       uint32   // Modification time
	BackupTime    uint32   // Backup time
	ModNumber     uint32   // Modification number
	AppInfoOffset uint32   // Offset to app info
	SortInfoOffset uint32  // Offset to sort info
	Type          [4]byte  // Database type
	Creator       [4]byte  // Creator ID
	UniqueIDSeed  uint32   // Unique ID seed
	NextRecordList uint32  // Next record list ID
	NumRecords    uint16   // Number of records
}

func main() {
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorf("failed to decode request: %v", err)
		return
	}

	switch req.Command {
	case "detect":
		handleDetect(req.Args)
	case "ingest":
		handleIngest(req.Args)
	case "enumerate":
		handleEnumerate(req.Args)
	case "extract-ir":
		handleExtractIR(req.Args)
	case "emit-native":
		handleEmitNative(req.Args)
	default:
		ipc.RespondErrorf("unknown command: %s", req.Command)
	}
}

func handleDetect(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if info.IsDir() {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "path is a directory, not a file",
		})
		return
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
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("extension %s is not a known Olive Tree format", ext),
		})
		return
	}

	// For .pdb files, try to verify it's an Olive Tree PDB
	if ext == ".pdb" {
		if detected, reason := detectPDBFormat(path); !detected {
			ipc.MustRespond(&ipc.DetectResult{
				Detected: false,
				Reason:   reason,
			})
			return
		}
	}

	// For modern formats (.ot4i, .oti, .otm), check if it's a valid file
	if ext == ".ot4i" || ext == ".oti" || ext == ".otm" {
		// These are typically encrypted SQLite or proprietary binary formats
		// We can do basic validation
		data, err := os.Open(path)
		if err != nil {
			ipc.MustRespond(&ipc.DetectResult{
				Detected: false,
				Reason:   fmt.Sprintf("cannot open file: %v", err),
			})
			return
		}
		defer data.Close()

		// Read first few bytes to check for SQLite signature or other markers
		header := make([]byte, 16)
		n, err := data.Read(header)
		if err != nil || n < 16 {
			ipc.MustRespond(&ipc.DetectResult{
				Detected: false,
				Reason:   "file too small or unreadable",
			})
			return
		}

		// Check for SQLite signature (may be encrypted)
		// SQLite files start with "SQLite format 3\x00"
		isSQLite := string(header[:15]) == "SQLite format 3"

		if isSQLite {
			moduleType = moduleType + " (SQLite-based)"
		} else {
			moduleType = moduleType + " (proprietary/encrypted)"
		}
	}

	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   "OliveTree",
		Reason:   fmt.Sprintf("Olive Tree format detected: %s (proprietary - conversion not supported)", moduleType),
	})
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

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		ipc.RespondErrorf("failed to create blob dir: %v", err)
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		ipc.RespondErrorf("failed to write blob: %v", err)
		return
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format":  "OliveTree",
			"warning": "Format ingested but conversion not supported (proprietary)",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		ipc.RespondErrorf("failed to stat: %v", err)
		return
	}

	entries := []ipc.EnumerateEntry{
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
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: entries,
	})
}

func handleExtractIR(args map[string]interface{}) {
	// Respond with error but use MustRespond instead of RespondError to avoid os.Exit in tests
	resp := ipc.Response{
		Status: "error",
		Error:  "extract-ir not supported: Olive Tree format is proprietary/encrypted. Format specification not publicly documented. Contact Olive Tree Bible Software for conversion options.",
	}
	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		ipc.RespondErrorf("failed to encode error response: %v", err)
	}
}

func handleEmitNative(args map[string]interface{}) {
	// Respond with error but use MustRespond instead of RespondError to avoid os.Exit in tests
	resp := ipc.Response{
		Status: "error",
		Error:  "emit-native not supported: Olive Tree format is proprietary/encrypted. Format specification not publicly documented. Cannot generate Olive Tree files without format documentation.",
	}
	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		ipc.RespondErrorf("failed to encode error response: %v", err)
	}
}
