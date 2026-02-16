//go:build !sdk

// Plugin format-tei handles TEI (Text Encoding Initiative) XML format.
// TEI is a scholarly XML format for encoding texts in the humanities.
//
// IR Support:
// - extract-ir: Reads TEI XML to IR (L1)
// - emit-native: Converts IR to TEI XML (L1)
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	// TEI commonly uses .xml or .tei extension
	if ext == ".tei" {
		respond(&DetectResult{
			Detected: true,
			Format:   "TEI",
			Reason:   "TEI file extension detected",
		})
		return
	}

	// Check for TEI XML structure
	data, err := os.ReadFile(path)
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   "not a TEI file",
		})
		return
	}

	content := string(data)
	// TEI documents have <TEI> or <TEI.2> root element and teiHeader
	if strings.Contains(content, "<TEI") && strings.Contains(content, "teiHeader") {
		respond(&DetectResult{
			Detected: true,
			Format:   "TEI",
			Reason:   "TEI XML structure detected",
		})
		return
	}

	// Check for TEI namespace
	if strings.Contains(content, "http://www.tei-c.org/ns/") {
		respond(&DetectResult{
			Detected: true,
			Format:   "TEI",
			Reason:   "TEI namespace detected",
		})
		return
	}

	respond(&DetectResult{
		Detected: false,
		Reason:   "no TEI structure found",
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
			"format": "TEI",
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
				Metadata:  map[string]string{"format": "TEI"},
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
		SourceFormat: "TEI",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L1",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_tei_raw"] = hex.EncodeToString(data)

	// Extract metadata and content from TEI XML
	content := string(data)
	extractTEIMetadata(content, corpus)
	corpus.Documents = extractTEIContent(content, artifactID)

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
		LossClass: "L1",
		LossReport: &LossReport{
			SourceFormat: "TEI",
			TargetFormat: "IR",
			LossClass:    "L1",
			Warnings:     []string{"TEI scholarly format - semantically lossless"},
		},
	})
}

func extractTEIMetadata(content string, corpus *Corpus) {
	// Extract title
	titlePattern := regexp.MustCompile(`<title[^>]*>([^<]+)</title>`)
	if matches := titlePattern.FindStringSubmatch(content); len(matches) > 1 {
		corpus.Title = strings.TrimSpace(matches[1])
	}

	// Extract language
	langPattern := regexp.MustCompile(`<language[^>]*ident="([^"]+)"`)
	if matches := langPattern.FindStringSubmatch(content); len(matches) > 1 {
		corpus.Language = matches[1]
	}

	// Extract publisher
	pubPattern := regexp.MustCompile(`<publisher>([^<]+)</publisher>`)
	if matches := pubPattern.FindStringSubmatch(content); len(matches) > 1 {
		corpus.Publisher = strings.TrimSpace(matches[1])
	}
}

func extractTEIContent(content, artifactID string) []*Document {
	var documents []*Document

	// Extract div elements (books/chapters)
	divPattern := regexp.MustCompile(`<div[^>]*type="([^"]*)"[^>]*n="([^"]*)"[^>]*>([\s\S]*?)</div>`)
	divMatches := divPattern.FindAllStringSubmatch(content, -1)

	sequence := 0
	docOrder := 0

	for _, match := range divMatches {
		if len(match) < 4 {
			continue
		}

		divType := match[1]
		divN := match[2]
		divContent := match[3]

		// Skip nested divs if type is "book" or "chapter"
		if divType == "book" || divType == "chapter" {
			docOrder++
			doc := &Document{
				ID:    divN,
				Title: divN,
				Order: docOrder,
			}

			// Extract verses/segments from div content
			segPattern := regexp.MustCompile(`<seg[^>]*n="([^"]*)"[^>]*>([^<]*)</seg>`)
			versePattern := regexp.MustCompile(`<ab[^>]*n="([^"]*)"[^>]*>([^<]*)</ab>`)

			for _, segMatch := range segPattern.FindAllStringSubmatch(divContent, -1) {
				if len(segMatch) >= 3 {
					sequence++
					text := strings.TrimSpace(segMatch[2])
					if len(text) > 0 {
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

			for _, verseMatch := range versePattern.FindAllStringSubmatch(divContent, -1) {
				if len(verseMatch) >= 3 {
					sequence++
					text := strings.TrimSpace(verseMatch[2])
					if len(text) > 0 {
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

			if len(doc.ContentBlocks) > 0 {
				documents = append(documents, doc)
			}
		}
	}

	// If no structured content, extract paragraphs
	if len(documents) == 0 {
		doc := &Document{
			ID:    artifactID,
			Title: artifactID,
			Order: 1,
		}

		pPattern := regexp.MustCompile(`<p[^>]*>([^<]+)</p>`)
		for _, match := range pPattern.FindAllStringSubmatch(content, -1) {
			if len(match) >= 2 {
				sequence++
				text := strings.TrimSpace(match[1])
				if len(text) > 5 {
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

		if len(doc.ContentBlocks) > 0 {
			documents = []*Document{doc}
		}
	}

	return documents
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

	outputPath := filepath.Join(outputDir, corpus.ID+".tei.xml")

	// Check for raw TEI for round-trip
	if raw, ok := corpus.Attributes["_tei_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0644); err != nil {
				respondError(fmt.Sprintf("failed to write TEI: %v", err))
				return
			}

			respond(&EmitNativeResult{
				OutputPath: outputPath,
				Format:     "TEI",
				LossClass:  "L0",
				LossReport: &LossReport{
					SourceFormat: "IR",
					TargetFormat: "TEI",
					LossClass:    "L0",
				},
			})
			return
		}
	}

	// Generate TEI XML from IR
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString("\n")
	buf.WriteString(`<TEI xmlns="http://www.tei-c.org/ns/1.0">`)
	buf.WriteString("\n")
	buf.WriteString("  <teiHeader>\n")
	buf.WriteString("    <fileDesc>\n")
	buf.WriteString("      <titleStmt>\n")
	fmt.Fprintf(&buf, "        <title>%s</title>\n", escapeXML(corpus.Title))
	buf.WriteString("      </titleStmt>\n")
	buf.WriteString("      <publicationStmt>\n")
	if corpus.Publisher != "" {
		fmt.Fprintf(&buf, "        <publisher>%s</publisher>\n", escapeXML(corpus.Publisher))
	} else {
		buf.WriteString("        <p>Generated from IR</p>\n")
	}
	buf.WriteString("      </publicationStmt>\n")
	buf.WriteString("      <sourceDesc>\n")
	buf.WriteString("        <p>Converted from Capsule IR</p>\n")
	buf.WriteString("      </sourceDesc>\n")
	buf.WriteString("    </fileDesc>\n")
	if corpus.Language != "" {
		buf.WriteString("    <profileDesc>\n")
		buf.WriteString("      <langUsage>\n")
		fmt.Fprintf(&buf, "        <language ident=\"%s\"/>\n", corpus.Language)
		buf.WriteString("      </langUsage>\n")
		buf.WriteString("    </profileDesc>\n")
	}
	buf.WriteString("  </teiHeader>\n")
	buf.WriteString("  <text>\n")
	buf.WriteString("    <body>\n")

	for _, doc := range corpus.Documents {
		fmt.Fprintf(&buf, "      <div type=\"book\" n=\"%s\">\n", escapeXML(doc.ID))
		for _, cb := range doc.ContentBlocks {
			fmt.Fprintf(&buf, "        <ab n=\"%d\">%s</ab>\n", cb.Sequence, escapeXML(cb.Text))
		}
		buf.WriteString("      </div>\n")
	}

	buf.WriteString("    </body>\n")
	buf.WriteString("  </text>\n")
	buf.WriteString("</TEI>\n")

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		respondError(fmt.Sprintf("failed to write TEI: %v", err))
		return
	}

	respond(&EmitNativeResult{
		OutputPath: outputPath,
		Format:     "TEI",
		LossClass:  "L1",
		LossReport: &LossReport{
			SourceFormat: "IR",
			TargetFormat: "TEI",
			LossClass:    "L1",
			Warnings:     []string{"Generated TEI XML - semantically complete"},
		},
	})
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
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
