//go:build sdk

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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
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
	if err := format.Run(&format.Config{
		Name:       "PDB",
		Extensions: []string{".pdb"},
		Detect:     detectPDB,
		Parse:      parsePDB,
		Emit:       emitPDB,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectPDB(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".pdb" {
		data, err := os.ReadFile(path)
		if err == nil && len(data) >= 78 {
			typeCode := string(data[60:64])
			creatorCode := string(data[64:68])
			if isBiblePDB(typeCode, creatorCode) {
				return &ipc.DetectResult{Detected: true, Format: "PDB", Reason: "Palm Bible PDB format detected"}, nil
			}
		}
		return &ipc.DetectResult{Detected: true, Format: "PDB", Reason: "PDB file extension detected"}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: "not a PDB file"}, nil
	}

	if len(data) >= 78 {
		numRecords := binary.BigEndian.Uint16(data[76:78])
		if numRecords > 0 && numRecords < 10000 {
			typeCode := string(data[60:64])
			creatorCode := string(data[64:68])
			if isBiblePDB(typeCode, creatorCode) {
				return &ipc.DetectResult{Detected: true, Format: "PDB", Reason: "Palm Bible PDB structure detected"}, nil
			}
		}
	}

	return &ipc.DetectResult{Detected: false, Reason: "no PDB structure found"}, nil
}

func isBiblePDB(typeCode, creatorCode string) bool {
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

func parsePDB(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "PDB"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L3"
	corpus.Attributes = map[string]string{"_pdb_raw": hex.EncodeToString(data)}

	// Extract title from header
	if len(data) >= 32 {
		name := strings.TrimRight(string(data[0:32]), "\x00")
		corpus.Title = name
	}

	// Extract content from PDB records
	corpus.Documents = extractPDBContent(data, artifactID)

	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ir.Document{ir.NewDocument(artifactID, corpus.Title, 1)}
	}

	return corpus, nil
}

func extractPDBContent(data []byte, artifactID string) []*ir.Document {
	doc := ir.NewDocument(artifactID, artifactID, 1)

	if len(data) < 78 {
		return []*ir.Document{doc}
	}

	name := strings.TrimRight(string(data[0:32]), "\x00")
	if name != "" {
		doc.Title = name
	}

	numRecords := binary.BigEndian.Uint16(data[76:78])
	if numRecords == 0 || numRecords > 10000 {
		return []*ir.Document{doc}
	}

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
				doc.ContentBlocks = append(doc.ContentBlocks, &ir.ContentBlock{
					ID:       fmt.Sprintf("cb-%d", sequence),
					Sequence: sequence,
					Text:     text,
					Hash:     hex.EncodeToString(hash[:]),
				})
			}
		}
	}

	return []*ir.Document{doc}
}

func extractTextFromRecord(data []byte) []string {
	var texts []string
	var current strings.Builder

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

func emitPDB(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".pdb")

	// Check for raw PDB for round-trip
	if raw, ok := corpus.Attributes["_pdb_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0644); err != nil {
				return "", fmt.Errorf("failed to write PDB: %w", err)
			}
			return outputPath, nil
		}
	}

	// Generate PDB-compatible file from IR
	pdbData := createPDBFromCorpus(corpus)
	if err := os.WriteFile(outputPath, pdbData, 0644); err != nil {
		return "", fmt.Errorf("failed to write PDB: %w", err)
	}

	return outputPath, nil
}

func createPDBFromCorpus(corpus *ir.Corpus) []byte {
	var buf bytes.Buffer

	var records [][]byte
	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			records = append(records, []byte(cb.Text+"\n"))
		}
	}

	if len(records) == 0 {
		records = append(records, []byte("Empty Bible\n"))
	}

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

	recordListSize := len(records) * 8
	dataStart := 78 + recordListSize + 2

	offset := uint32(dataStart)
	for i, rec := range records {
		binary.Write(&buf, binary.BigEndian, offset)
		buf.WriteByte(0)
		buf.WriteByte(0)
		buf.WriteByte(0)
		buf.WriteByte(byte(i))
		offset += uint32(len(rec))
	}

	buf.WriteByte(0)
	buf.WriteByte(0)

	for _, rec := range records {
		buf.Write(rec)
	}

	return buf.Bytes()
}
