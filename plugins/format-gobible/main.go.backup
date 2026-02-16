//go:build !sdk

// Plugin format-gobible handles GoBible J2ME format.
// GoBible uses JAR-based .gbk files containing binary verse data.
//
// IR Support:
// - extract-ir: Reads GoBible format to IR (L3)
// - emit-native: Converts IR to GoBible-compatible format (L3)
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	// GoBible uses .gbk extension
	if ext == ".gbk" {
		respond(&DetectResult{
			Detected: true,
			Format:   "GoBible",
			Reason:   "GoBible file extension detected",
		})
		return
	}

	// Check if it's a JAR/ZIP with GoBible structure
	zr, err := zip.OpenReader(path)
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   "not a GoBible file",
		})
		return
	}
	defer zr.Close()

	// Look for GoBible-specific files
	hasCollections := false
	hasBibleData := false
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if name == "collections" || name == "bible/collections" {
			hasCollections = true
		}
		if strings.HasPrefix(name, "bible/") || strings.Contains(name, "verse") {
			hasBibleData = true
		}
	}

	if hasCollections || hasBibleData {
		respond(&DetectResult{
			Detected: true,
			Format:   "GoBible",
			Reason:   "GoBible JAR structure detected",
		})
		return
	}

	respond(&DetectResult{
		Detected: false,
		Reason:   "no GoBible structure found",
	})
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
	if err := os.MkdirAll(blobDir, 0700); err != nil {
		respondError(fmt.Sprintf("failed to create blob dir: %v", err))
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0600); err != nil {
		respondError(fmt.Sprintf("failed to write blob: %v", err))
		return
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	respond(&IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "GoBible",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		// Single file
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
					Metadata:  map[string]string{"format": "GoBible"},
				},
			},
		})
		return
	}
	defer zr.Close()

	var entries []EnumerateEntry
	for _, f := range zr.File {
		entries = append(entries, EnumerateEntry{
			Path:      f.Name,
			SizeBytes: int64(f.UncompressedSize64),
			IsDir:     f.FileInfo().IsDir(),
		})
	}

	respond(&EnumerateResult{Entries: entries})
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
		SourceFormat: "GoBible",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L3",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_gobible_raw"] = hex.EncodeToString(data)

	// Try to extract content from JAR
	corpus.Documents = extractGoBibleContent(data, artifactID)

	// If no documents extracted, create minimal structure
	if len(corpus.Documents) == 0 {
		corpus.Documents = []*Document{
			{
				ID:    artifactID,
				Title: artifactID,
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
	if err := os.WriteFile(irPath, irData, 0600); err != nil {
		respondError(fmt.Sprintf("failed to write IR: %v", err))
		return
	}

	respond(&ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L3",
		LossReport: &LossReport{
			SourceFormat: "GoBible",
			TargetFormat: "IR",
			LossClass:    "L3",
			Warnings:     []string{"GoBible binary format - text extraction only, formatting lost"},
		},
	})
}

func extractGoBibleContent(data []byte, artifactID string) []*Document {
	doc := &Document{
		ID:    artifactID,
		Title: artifactID,
		Order: 1,
	}

	// Try to open as ZIP
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return []*Document{doc}
	}

	sequence := 0
	for _, f := range zr.File {
		// Look for text content files
		if strings.HasSuffix(f.Name, ".txt") || strings.Contains(f.Name, "verse") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			// Parse lines as verses
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if len(line) > 5 {
					sequence++
					hash := sha256.Sum256([]byte(line))
					doc.ContentBlocks = append(doc.ContentBlocks, &ContentBlock{
						ID:       fmt.Sprintf("cb-%d", sequence),
						Sequence: sequence,
						Text:     line,
						Hash:     hex.EncodeToString(hash[:]),
					})
				}
			}
		}

		// Try to parse binary Bible data
		if strings.HasPrefix(f.Name, "Bible/") || f.Name == "Collections" {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			// Extract text from binary (simplified - real format is more complex)
			extracted := extractTextFromBinary(content)
			for _, text := range extracted {
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
	}

	return []*Document{doc}
}

func extractTextFromBinary(data []byte) []string {
	var texts []string
	var current strings.Builder

	// Simple heuristic: look for runs of printable ASCII/UTF-8
	for i := 0; i < len(data); i++ {
		b := data[i]
		if b >= 32 && b < 127 {
			current.WriteByte(b)
		} else if b >= 0xC0 && i+1 < len(data) {
			// UTF-8 multibyte - try to decode
			if b < 0xE0 && i+1 < len(data) {
				// 2-byte sequence
				current.WriteByte(b)
				i++
				current.WriteByte(data[i])
			}
		} else {
			if current.Len() > 10 {
				texts = append(texts, current.String())
			}
			current.Reset()
		}
	}

	if current.Len() > 10 {
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

	outputPath := filepath.Join(outputDir, corpus.ID+".gbk")

	// Check for raw GoBible for round-trip
	if raw, ok := corpus.Attributes["_gobible_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
				respondError(fmt.Sprintf("failed to write GoBible: %v", err))
				return
			}

			respond(&EmitNativeResult{
				OutputPath: outputPath,
				Format:     "GoBible",
				LossClass:  "L0",
				LossReport: &LossReport{
					SourceFormat: "IR",
					TargetFormat: "GoBible",
					LossClass:    "L0",
				},
			})
			return
		}
	}

	// Generate GoBible-compatible JAR from IR
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Create manifest
	manifest := "Manifest-Version: 1.0\nMIDlet-Name: " + corpus.Title + "\n"
	mf, _ := zw.Create("META-INF/MANIFEST.MF")
	mf.Write([]byte(manifest))

	// Create Collections file (simplified binary format)
	collectionsData := createCollectionsFile(&corpus)
	cf, _ := zw.Create("Bible/Collections")
	cf.Write(collectionsData)

	// Create verse data files
	for i, doc := range corpus.Documents {
		bookData := createBookDataFile(doc)
		bf, _ := zw.Create(fmt.Sprintf("Bible/Book%d", i))
		bf.Write(bookData)
	}

	zw.Close()

	if err := os.WriteFile(outputPath, buf.Bytes(), 0600); err != nil {
		respondError(fmt.Sprintf("failed to write GoBible: %v", err))
		return
	}

	respond(&EmitNativeResult{
		OutputPath: outputPath,
		Format:     "GoBible",
		LossClass:  "L3",
		LossReport: &LossReport{
			SourceFormat: "IR",
			TargetFormat: "GoBible",
			LossClass:    "L3",
			Warnings:     []string{"Generated GoBible-compatible format - simplified structure"},
		},
	})
}

func createCollectionsFile(corpus *Corpus) []byte {
	var buf bytes.Buffer

	// GoBible Collections format (simplified)
	// Header: version, book count
	binary.Write(&buf, binary.BigEndian, uint16(1)) // version
	binary.Write(&buf, binary.BigEndian, uint16(len(corpus.Documents)))

	for _, doc := range corpus.Documents {
		// Book name length + name
		name := []byte(doc.Title)
		binary.Write(&buf, binary.BigEndian, uint8(len(name)))
		buf.Write(name)
		// Chapter count
		binary.Write(&buf, binary.BigEndian, uint16(len(doc.ContentBlocks)))
	}

	return buf.Bytes()
}

func createBookDataFile(doc *Document) []byte {
	var buf bytes.Buffer

	for _, cb := range doc.ContentBlocks {
		// Simple format: length-prefixed text
		text := []byte(cb.Text)
		binary.Write(&buf, binary.BigEndian, uint16(len(text)))
		buf.Write(text)
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
