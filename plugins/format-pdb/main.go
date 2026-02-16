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
)

// IPCRequest is the incoming JSON request.
type IPCRequest struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args,omitempty"`
}

// IPCResponse is the outgoing JSON response.
type IPCResponse struct {
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// DetectResult is the result of a detect command.
type DetectResult struct {
	Detected bool   `json:"detected"`
	Format   string `json:"format,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// IngestResult is the result of an ingest command.
type IngestResult struct {
	ArtifactID string            `json:"artifact_id"`
	BlobSHA256 string            `json:"blob_sha256"`
	SizeBytes  int64             `json:"size_bytes"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// EnumerateResult is the result of an enumerate command.
type EnumerateResult struct {
	Entries []EnumerateEntry `json:"entries"`
}

// EnumerateEntry represents a file entry.
type EnumerateEntry struct {
	Path      string            `json:"path"`
	SizeBytes int64             `json:"size_bytes"`
	IsDir     bool              `json:"is_dir"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ExtractIRResult is the result of an extract-ir command.
type ExtractIRResult struct {
	IRPath     string      `json:"ir_path"`
	LossClass  string      `json:"loss_class"`
	LossReport *LossReport `json:"loss_report,omitempty"`
}

// EmitNativeResult is the result of an emit-native command.
type EmitNativeResult struct {
	OutputPath string      `json:"output_path"`
	Format     string      `json:"format"`
	LossClass  string      `json:"loss_class"`
	LossReport *LossReport `json:"loss_report,omitempty"`
}

// LossReport describes any data loss during conversion.
type LossReport struct {
	SourceFormat string        `json:"source_format"`
	TargetFormat string        `json:"target_format"`
	LossClass    string        `json:"loss_class"`
	LostElements []LostElement `json:"lost_elements,omitempty"`
	Warnings     []string      `json:"warnings,omitempty"`
}

// LostElement describes a specific element that was lost.
type LostElement struct {
	Path          string      `json:"path"`
	ElementType   string      `json:"element_type"`
	Reason        string      `json:"reason"`
	OriginalValue interface{} `json:"original_value,omitempty"`
}

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

// IR Types
type Corpus struct {
	ID            string            `json:"id"`
	Version       string            `json:"version"`
	ModuleType    string            `json:"module_type"`
	Versification string            `json:"versification,omitempty"`
	Language      string            `json:"language,omitempty"`
	Title         string            `json:"title,omitempty"`
	Description   string            `json:"description,omitempty"`
	Publisher     string            `json:"publisher,omitempty"`
	Rights        string            `json:"rights,omitempty"`
	SourceFormat  string            `json:"source_format,omitempty"`
	Documents     []*Document       `json:"documents,omitempty"`
	SourceHash    string            `json:"source_hash,omitempty"`
	LossClass     string            `json:"loss_class,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type Document struct {
	ID            string            `json:"id"`
	Title         string            `json:"title,omitempty"`
	Order         int               `json:"order"`
	ContentBlocks []*ContentBlock   `json:"content_blocks,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type ContentBlock struct {
	ID         string                 `json:"id"`
	Sequence   int                    `json:"sequence"`
	Text       string                 `json:"text"`
	Tokens     []*Token               `json:"tokens,omitempty"`
	Anchors    []*Anchor              `json:"anchors,omitempty"`
	Hash       string                 `json:"hash,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type Token struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Text     string `json:"text"`
	StartPos int    `json:"start_pos"`
	EndPos   int    `json:"end_pos"`
}

type Anchor struct {
	ID       string  `json:"id"`
	Position int     `json:"position"`
	Spans    []*Span `json:"spans,omitempty"`
}

type Span struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	StartAnchorID string                 `json:"start_anchor_id"`
	EndAnchorID   string                 `json:"end_anchor_id,omitempty"`
	Ref           *Ref                   `json:"ref,omitempty"`
	Attributes    map[string]interface{} `json:"attributes,omitempty"`
}

type Ref struct {
	Book     string `json:"book"`
	Chapter  int    `json:"chapter,omitempty"`
	Verse    int    `json:"verse,omitempty"`
	VerseEnd int    `json:"verse_end,omitempty"`
	SubVerse string `json:"sub_verse,omitempty"`
	OSISID   string `json:"osis_id,omitempty"`
}

func main() {
	var req IPCRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		respondError(fmt.Sprintf("failed to decode request: %v", err))
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
		respondError(fmt.Sprintf("unknown command: %s", req.Command))
	}
}

func handleDetect(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
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
				respond(&DetectResult{
					Detected: true,
					Format:   "PDB",
					Reason:   "Palm Bible PDB format detected",
				})
				return
			}
		}

		respond(&DetectResult{
			Detected: true,
			Format:   "PDB",
			Reason:   "PDB file extension detected",
		})
		return
	}

	// Check for PDB magic/structure
	data, err := os.ReadFile(path)
	if err != nil {
		respond(&DetectResult{
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
				respond(&DetectResult{
					Detected: true,
					Format:   "PDB",
					Reason:   "Palm Bible PDB structure detected",
				})
				return
			}
		}
	}

	respond(&DetectResult{
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
		respondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		respondError(fmt.Sprintf("failed to create blob dir: %v", err))
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write blob: %v", err))
		return
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	respond(&IngestResult{
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
		respondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to stat: %v", err))
		return
	}

	respond(&EnumerateResult{
		Entries: []EnumerateEntry{
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
		respondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &Corpus{
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
		corpus.Documents = []*Document{
			{
				ID:    artifactID,
				Title: corpus.Title,
				Order: 1,
			},
		}
	}

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		respondError(fmt.Sprintf("failed to serialize IR: %v", err))
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write IR: %v", err))
		return
	}

	respond(&ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L3",
		LossReport: &LossReport{
			SourceFormat: "PDB",
			TargetFormat: "IR",
			LossClass:    "L3",
			Warnings:     []string{"PDB binary format - text extraction only, formatting lost"},
		},
	})
}

func extractPDBContent(data []byte, artifactID string) []*Document {
	doc := &Document{
		ID:    artifactID,
		Title: artifactID,
		Order: 1,
	}

	if len(data) < 78 {
		return []*Document{doc}
	}

	// Extract title from header
	name := strings.TrimRight(string(data[0:32]), "\x00")
	if name != "" {
		doc.Title = name
	}

	// Read header
	numRecords := binary.BigEndian.Uint16(data[76:78])
	if numRecords == 0 || numRecords > 10000 {
		return []*Document{doc}
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
				doc.ContentBlocks = append(doc.ContentBlocks, &ContentBlock{
					ID:       fmt.Sprintf("cb-%d", sequence),
					Sequence: sequence,
					Text:     text,
					Hash:     hex.EncodeToString(hash[:]),
				})
			}
		}
	}

	return []*Document{doc}
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
		respondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(irPath)
	if err != nil {
		respondError(fmt.Sprintf("failed to read IR file: %v", err))
		return
	}

	var corpus Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		respondError(fmt.Sprintf("failed to parse IR: %v", err))
		return
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".pdb")

	// Check for raw PDB for round-trip
	if raw, ok := corpus.Attributes["_pdb_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0644); err != nil {
				respondError(fmt.Sprintf("failed to write PDB: %v", err))
				return
			}

			respond(&EmitNativeResult{
				OutputPath: outputPath,
				Format:     "PDB",
				LossClass:  "L0",
				LossReport: &LossReport{
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
		respondError(fmt.Sprintf("failed to write PDB: %v", err))
		return
	}

	respond(&EmitNativeResult{
		OutputPath: outputPath,
		Format:     "PDB",
		LossClass:  "L3",
		LossReport: &LossReport{
			SourceFormat: "IR",
			TargetFormat: "PDB",
			LossClass:    "L3",
			Warnings:     []string{"Generated PDB-compatible format - simplified structure"},
		},
	})
}

func createPDBFromCorpus(corpus *Corpus) []byte {
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

func respond(result interface{}) {
	resp := IPCResponse{
		Status: "ok",
		Result: result,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
}

func respondError(msg string) {
	resp := IPCResponse{
		Status: "error",
		Error:  msg,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
	os.Exit(1)
}
