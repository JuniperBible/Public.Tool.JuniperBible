//go:build !sdk

// Plugin format-pdb handles Palm Bible+ PDB format.
// PDB is a binary database format used by Palm OS Bible applications.
//
// IR Support:
// - extract-ir: Reads PDB format to IR (L3)
// - emit-native: Converts IR to PDB-compatible format (L3)
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// PDB Header structure
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

// PDB Record entry
type PDBRecordEntry struct {
	Offset     uint32
	Attributes uint8
	UniqueID   [3]byte
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

	ext := strings.ToLower(filepath.Ext(path))
	// PDB uses .pdb extension
	if ext == ".pdb" {
		// Verify it's actually a PDB file by reading header
		data, err := os.ReadFile(path)
		if err == nil && len(data) >= 78 {
			// Check for Bible-related type/creator
			typeCode := string(data[60:64])
			creatorCode := string(data[64:68])
			if isBiblePDB(typeCode, creatorCode) {
				ipc.MustRespond(&ipc.DetectResult{
					Detected: true,
					Format:   "PDB",
					Reason:   "Palm Bible PDB format detected",
				})
				return
			}
		}
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "PDB",
			Reason:   "PDB file extension detected",
		})
		return
	}

	// Check for PDB magic/structure
	data, err := os.ReadFile(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not a PDB file",
		})
		return
	}

	if len(data) >= 78 {
		// Check for valid PDB structure
		numRecords := binary.BigEndian.Uint16(data[76:78])
		if numRecords > 0 && numRecords < 10000 {
			typeCode := string(data[60:64])
			creatorCode := string(data[64:68])
			if isBiblePDB(typeCode, creatorCode) {
				ipc.MustRespond(&ipc.DetectResult{
					Detected: true,
					Format:   "PDB",
					Reason:   "Palm Bible PDB structure detected",
				})
				return
			}
		}
	}
	ipc.MustRespond(&ipc.DetectResult{
		Detected: false,
		Reason:   "no PDB structure found",
	})
}

func isBiblePDB(typeCode, creatorCode string) bool {
	// Known Bible PDB type/creator combinations
	bibleTypes := []string{"Bibl", "BiBl", "bibl", "BIBL", "PNRd", "BDoc"}
	bibleCreators := []string{"Bibl", "BiBl", "bibl", "BIBL", "PNRd", "GoBi", "Plkr"}

	for _, t := range bibleTypes {
		if typeCode == t {
			return true
		}
	}
	for _, c := range bibleCreators {
		if creatorCode == c {
			return true
		}
	}
	return false
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
			"format": "PDB",
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
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: []ipc.EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
				Metadata:  map[string]string{"format": "PDB"},
			},
		},
	})
}

func handleExtractIR(args map[string]interface{}) {
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

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ipc.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "PDB",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L3",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_pdb_raw"] = hex.EncodeToString(data)

	// Extract title from header
	if len(data) >= 32 {
		name := strings.TrimRight(string(data[0:32]), "\x00")
		corpus.Title = name
	}

	// Extract content from PDB records
	corpus.Documents = extractPDBContent(data, artifactID)

	// If no documents extracted, create minimal structure
	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ipc.Document{
			{
				ID:    artifactID,
				Title: corpus.Title,
				Order: 1,
			},
		}
	}

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to serialize IR: %v", err)
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}
	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L3",
		LossReport: &ipc.LossReport{
			SourceFormat: "PDB",
			TargetFormat: "IR",
			LossClass:    "L3",
			Warnings:     []string{"PDB binary format - text extraction only, formatting lost"},
		},
	})
}

func extractPDBContent(data []byte, artifactID string) []*ipc.Document {
	doc := &ipc.Document{
		ID:    artifactID,
		Title: artifactID,
		Order: 1,
	}

	if len(data) < 78 {
		return []*ipc.Document{doc}
	}

	// Extract title from header
	name := strings.TrimRight(string(data[0:32]), "\x00")
	if name != "" {
		doc.Title = name
	}

	// Read header
	numRecords := binary.BigEndian.Uint16(data[76:78])
	if numRecords == 0 || numRecords > 10000 {
		return []*ipc.Document{doc}
	}

	// Parse record entries
	recordListStart := 78
	var records []PDBRecordEntry

	for i := uint16(0); i < numRecords; i++ {
		offset := recordListStart + int(i)*8
		if offset+8 > len(data) {
			break
		}

		var entry PDBRecordEntry
		entry.Offset = binary.BigEndian.Uint32(data[offset : offset+4])
		entry.Attributes = data[offset+4]
		copy(entry.UniqueID[:], data[offset+5:offset+8])
		records = append(records, entry)
	}

	// Extract text from records
	sequence := 0
	for i, rec := range records {
		start := int(rec.Offset)
		end := len(data)
		if i+1 < len(records) {
			end = int(records[i+1].Offset)
		}

		if start >= end || start >= len(data) {
			continue
		}
		if end > len(data) {
			end = len(data)
		}

		recordData := data[start:end]
		texts := extractTextFromRecord(recordData)

		for _, text := range texts {
			if len(text) > 5 {
				sequence++
				hash := sha256.Sum256([]byte(text))
				doc.ContentBlocks = append(doc.ContentBlocks, &ipc.ContentBlock{
					ID:       fmt.Sprintf("cb-%d", sequence),
					Sequence: sequence,
					Text:     text,
					Hash:     hex.EncodeToString(hash[:]),
				})
			}
		}
	}

	return []*ipc.Document{doc}
}

func extractTextFromRecord(data []byte) []string {
	var texts []string
	var current strings.Builder

	// Look for runs of printable text
	for i := 0; i < len(data); i++ {
		b := data[i]
		if b >= 32 && b < 127 {
			current.WriteByte(b)
		} else if b == '\n' || b == '\r' {
			if current.Len() > 5 {
				texts = append(texts, current.String())
			}
			current.Reset()
		} else {
			if current.Len() > 10 {
				texts = append(texts, current.String())
			}
			current.Reset()
		}
	}

	if current.Len() > 5 {
		texts = append(texts, current.String())
	}

	return texts
}

func handleEmitNative(args map[string]interface{}) {
	irPath, ok := args["ir_path"].(string)
	if !ok {
		ipc.RespondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR file: %v", err)
		return
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		ipc.RespondErrorf("failed to parse IR: %v", err)
		return
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".pdb")

	// Check for raw PDB for round-trip
	if raw, ok := corpus.Attributes["_pdb_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0644); err != nil {
				ipc.RespondErrorf("failed to write PDB: %v", err)
				return
			}
			ipc.MustRespond(&ipc.EmitNativeResult{
				OutputPath: outputPath,
				Format:     "PDB",
				LossClass:  "L0",
				LossReport: &ipc.LossReport{
					SourceFormat: "IR",
					TargetFormat: "PDB",
					LossClass:    "L0",
				},
			})
			return
		}
	}

	// Generate PDB-compatible file from IR
	pdbData := createPDBFromCorpus(&corpus)

	if err := os.WriteFile(outputPath, pdbData, 0644); err != nil {
		ipc.RespondErrorf("failed to write PDB: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "PDB",
		LossClass:  "L3",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "PDB",
			LossClass:    "L3",
			Warnings:     []string{"Generated PDB-compatible format - simplified structure"},
		},
	})
}

func createPDBFromCorpus(corpus *ipc.Corpus) []byte {
	var buf bytes.Buffer

	// Collect all text content
	var records [][]byte
	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			records = append(records, []byte(cb.Text+"\n"))
		}
	}

	if len(records) == 0 {
		records = append(records, []byte("Empty Bible\n"))
	}

	// Write PDB header
	var header PDBHeader
	name := corpus.Title
	if name == "" {
		name = corpus.ID
	}
	if len(name) > 31 {
		name = name[:31]
	}
	copy(header.Name[:], name)
	header.NumRecords = uint16(len(records))
	copy(header.Type[:], "Bibl")
	copy(header.Creator[:], "Bibl")

	binary.Write(&buf, binary.BigEndian, header)

	// Calculate record offsets
	recordListSize := len(records) * 8
	dataStart := 78 + recordListSize + 2 // +2 for padding

	offset := uint32(dataStart)
	for i, rec := range records {
		binary.Write(&buf, binary.BigEndian, offset)
		buf.WriteByte(0)       // attributes
		buf.WriteByte(0)       // unique ID byte 1
		buf.WriteByte(0)       // unique ID byte 2
		buf.WriteByte(byte(i)) // unique ID byte 3
		offset += uint32(len(rec))
	}

	// Padding
	buf.WriteByte(0)
	buf.WriteByte(0)

	// Write records
	for _, rec := range records {
		buf.Write(rec)
	}

	return buf.Bytes()
}
