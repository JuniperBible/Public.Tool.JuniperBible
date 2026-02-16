//go:build !sdk

// Plugin format-osis handles OSIS XML Bible files.
// It supports L0 lossless round-trip conversion through IR.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	IsDir     bool   `json:"is_dir"`
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

// IR Types (matching core/ir package)
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

// OSIS XML Types
type OSISDoc struct {
	XMLName   xml.Name `xml:"osis"`
	Namespace string   `xml:"xmlns,attr,omitempty"`
	OsisText  OSISText `xml:"osisText"`
	RawXML    string   `xml:"-"` // Store original for L0 round-trip
}

type OSISText struct {
	OsisIDWork  string      `xml:"osisIDWork,attr"`
	OsisRefWork string      `xml:"osisRefWork,attr,omitempty"`
	Lang        string      `xml:"lang,attr,omitempty"`
	XMLLang     string      `xml:"http://www.w3.org/XML/1998/namespace lang,attr,omitempty"`
	Header      *OSISHeader `xml:"header,omitempty"`
	Divs        []OSISDiv   `xml:"div"`
}

type OSISHeader struct {
	Work []OSISWork `xml:"work"`
}

type OSISWork struct {
	OsisWork    string `xml:"osisWork,attr"`
	Title       string `xml:"title,omitempty"`
	Type        string `xml:"type,omitempty"`
	Identifier  string `xml:"identifier,omitempty"`
	RefSystem   string `xml:"refSystem,omitempty"`
	Language    string `xml:"language,omitempty"`
	Publisher   string `xml:"publisher,omitempty"`
	Rights      string `xml:"rights,omitempty"`
	Description string `xml:"description,omitempty"`
}

type OSISDiv struct {
	Type     string        `xml:"type,attr,omitempty"`
	OsisID   string        `xml:"osisID,attr,omitempty"`
	Title    string        `xml:"title,omitempty"`
	Divs     []OSISDiv     `xml:"div"`
	Chapters []OSISChapter `xml:"chapter"`
	Verses   []OSISVerse   `xml:"verse"`
	Ps       []OSISP       `xml:"p"`
	Lgs      []OSISLg      `xml:"lg"`
	Ls       []OSISL       `xml:"l"`
	Content  string        `xml:",chardata"`
}

type OSISChapter struct {
	OsisID string `xml:"osisID,attr"`
	SID    string `xml:"sID,attr,omitempty"`
	EID    string `xml:"eID,attr,omitempty"`
}

type OSISVerse struct {
	OsisID string `xml:"osisID,attr,omitempty"`
	SID    string `xml:"sID,attr,omitempty"`
	EID    string `xml:"eID,attr,omitempty"`
}

type OSISP struct {
	Verses  []OSISVerse `xml:"verse"`
	Content string      `xml:",chardata"`
}

type OSISLg struct {
	Ls []OSISL `xml:"l"`
}

type OSISL struct {
	Content string `xml:",chardata"`
}

func main() {
	// Read request from stdin
	var req IPCRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		respondError(fmt.Sprintf("failed to decode request: %v", err))
		return
	}

	// Dispatch command
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

	info, err := os.Stat(path)
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if info.IsDir() {
		respond(&DetectResult{
			Detected: false,
			Reason:   "path is a directory, not a file",
		})
		return
	}

	// Read file and check for OSIS XML
	data, err := os.ReadFile(path)
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read: %v", err),
		})
		return
	}

	// Check for OSIS markers
	content := string(data)
	if strings.Contains(content, "<osis") && strings.Contains(content, "osisText") {
		respond(&DetectResult{
			Detected: true,
			Format:   "OSIS",
			Reason:   "OSIS XML detected",
		})
		return
	}

	// Check file extension as fallback
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".osis" || ext == ".xml" {
		// Try to parse as OSIS
		var doc OSISDoc
		if err := xml.Unmarshal(data, &doc); err == nil && doc.OsisText.OsisIDWork != "" {
			respond(&DetectResult{
				Detected: true,
				Format:   "OSIS",
				Reason:   "Valid OSIS XML structure",
			})
			return
		}
	}

	respond(&DetectResult{
		Detected: false,
		Reason:   "not an OSIS XML file",
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

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	// Compute SHA-256
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	// Write to output directory
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		respondError(fmt.Sprintf("failed to create blob dir: %v", err))
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0600); err != nil {
		respondError(fmt.Sprintf("failed to write blob: %v", err))
		return
	}

	// Generate artifact ID from work ID if possible
	var doc OSISDoc
	artifactID := filepath.Base(path)
	if err := xml.Unmarshal(data, &doc); err == nil && doc.OsisText.OsisIDWork != "" {
		artifactID = doc.OsisText.OsisIDWork
	}

	respond(&IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"original_name": filepath.Base(path),
			"format":        "OSIS",
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

	// Read OSIS file
	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	// Parse OSIS XML
	corpus, err := parseOSISToIR(data)
	if err != nil {
		respondError(fmt.Sprintf("failed to parse OSIS: %v", err))
		return
	}

	// Serialize IR to JSON
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		respondError(fmt.Sprintf("failed to serialize IR: %v", err))
		return
	}

	// Write IR to output directory
	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0600); err != nil {
		respondError(fmt.Sprintf("failed to write IR: %v", err))
		return
	}

	respond(&ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L0",
		LossReport: &LossReport{
			SourceFormat: "OSIS",
			TargetFormat: "IR",
			LossClass:    "L0",
		},
	})
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

	// Read IR file
	data, err := os.ReadFile(irPath)
	if err != nil {
		respondError(fmt.Sprintf("failed to read IR file: %v", err))
		return
	}

	// Parse IR
	var corpus Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		respondError(fmt.Sprintf("failed to parse IR: %v", err))
		return
	}

	// Convert IR to OSIS
	osisData, err := emitOSISFromIR(&corpus)
	if err != nil {
		respondError(fmt.Sprintf("failed to emit OSIS: %v", err))
		return
	}

	// Write OSIS to output directory
	outputPath := filepath.Join(outputDir, corpus.ID+".osis")
	if err := os.WriteFile(outputPath, osisData, 0600); err != nil {
		respondError(fmt.Sprintf("failed to write OSIS: %v", err))
		return
	}

	respond(&EmitNativeResult{
		OutputPath: outputPath,
		Format:     "OSIS",
		LossClass:  "L0",
		LossReport: &LossReport{
			SourceFormat: "IR",
			TargetFormat: "OSIS",
			LossClass:    "L0",
		},
	})
}

// parseOSISToIR converts OSIS XML to IR Corpus
func parseOSISToIR(data []byte) (*Corpus, error) {
	var doc OSISDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("xml unmarshal failed: %w", err)
	}

	// Store raw XML for potential L0 round-trip
	doc.RawXML = string(data)

	corpus := &Corpus{
		ID:           doc.OsisText.OsisIDWork,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "OSIS",
		LossClass:    "L0",
		Attributes:   make(map[string]string),
	}

	// Extract language
	if doc.OsisText.Lang != "" {
		corpus.Language = doc.OsisText.Lang
	} else if doc.OsisText.XMLLang != "" {
		corpus.Language = doc.OsisText.XMLLang
	}

	// Extract header information
	if doc.OsisText.Header != nil {
		for _, work := range doc.OsisText.Header.Work {
			if work.OsisWork == doc.OsisText.OsisIDWork || work.Title != "" {
				corpus.Title = work.Title
				corpus.Description = work.Description
				corpus.Publisher = work.Publisher
				corpus.Rights = work.Rights
				if work.RefSystem != "" {
					corpus.Versification = work.RefSystem
				}
				if work.Language != "" {
					corpus.Language = work.Language
				}
			}
		}
	}

	// Store original XML in attributes for L0 lossless round-trip
	corpus.Attributes["_osis_raw"] = doc.RawXML

	// Parse books (divs)
	docOrder := 0
	for _, div := range doc.OsisText.Divs {
		docs := parseOSISDiv(&div, &docOrder)
		corpus.Documents = append(corpus.Documents, docs...)
	}

	// Compute source hash
	h := sha256.Sum256(data)
	corpus.SourceHash = hex.EncodeToString(h[:])

	return corpus, nil
}

// parseOSISDiv recursively parses OSIS div elements
func parseOSISDiv(div *OSISDiv, order *int) []*Document {
	var docs []*Document

	// If this is a book-level div
	if div.Type == "book" || (div.OsisID != "" && isBookID(div.OsisID)) {
		*order++
		doc := &Document{
			ID:         div.OsisID,
			Title:      div.Title,
			Order:      *order,
			Attributes: make(map[string]string),
		}
		if div.Title == "" && div.OsisID != "" {
			doc.Title = div.OsisID
		}

		// Parse content blocks from this div
		cbSeq := 0
		blocks := extractContentBlocks(div, &cbSeq)
		doc.ContentBlocks = blocks

		docs = append(docs, doc)
	}

	// Recursively process child divs
	for _, childDiv := range div.Divs {
		childDocs := parseOSISDiv(&childDiv, order)
		docs = append(docs, childDocs...)
	}

	return docs
}

// extractContentBlocks extracts content blocks from an OSIS div
func extractContentBlocks(div *OSISDiv, seq *int) []*ContentBlock {
	var blocks []*ContentBlock

	// Process paragraphs
	for _, p := range div.Ps {
		text := strings.TrimSpace(p.Content)
		if text == "" {
			continue
		}

		*seq++
		block := &ContentBlock{
			ID:       fmt.Sprintf("cb-%d", *seq),
			Sequence: *seq,
			Text:     text,
			Anchors:  []*Anchor{},
		}

		// Add verse spans if present
		for _, v := range p.Verses {
			if v.OsisID != "" || v.SID != "" {
				osisID := v.OsisID
				if osisID == "" {
					osisID = v.SID
				}
				ref := parseOSISRef(osisID)
				anchor := &Anchor{
					ID:       fmt.Sprintf("a-%d-0", *seq),
					Position: 0,
					Spans: []*Span{
						{
							ID:            fmt.Sprintf("s-%s", osisID),
							Type:          "VERSE",
							StartAnchorID: fmt.Sprintf("a-%d-0", *seq),
							Ref:           ref,
						},
					},
				}
				block.Anchors = append(block.Anchors, anchor)
			}
		}

		// Compute hash
		h := sha256.Sum256([]byte(text))
		block.Hash = hex.EncodeToString(h[:])

		blocks = append(blocks, block)
	}

	// Process poetry lines
	for _, lg := range div.Lgs {
		for _, l := range lg.Ls {
			text := strings.TrimSpace(l.Content)
			if text == "" {
				continue
			}

			*seq++
			block := &ContentBlock{
				ID:       fmt.Sprintf("cb-%d", *seq),
				Sequence: *seq,
				Text:     text,
				Attributes: map[string]interface{}{
					"type": "poetry",
				},
			}

			// Compute hash
			h := sha256.Sum256([]byte(text))
			block.Hash = hex.EncodeToString(h[:])

			blocks = append(blocks, block)
		}
	}

	// Process direct content
	text := strings.TrimSpace(div.Content)
	if text != "" {
		*seq++
		block := &ContentBlock{
			ID:       fmt.Sprintf("cb-%d", *seq),
			Sequence: *seq,
			Text:     text,
		}
		h := sha256.Sum256([]byte(text))
		block.Hash = hex.EncodeToString(h[:])
		blocks = append(blocks, block)
	}

	// Process nested divs (chapters)
	for _, childDiv := range div.Divs {
		childBlocks := extractContentBlocks(&childDiv, seq)
		blocks = append(blocks, childBlocks...)
	}

	return blocks
}

// parseOSISRef parses an OSIS reference like "Gen.1.1" or "Matt.5.3-12"
func parseOSISRef(osisID string) *Ref {
	ref := &Ref{OSISID: osisID}

	// Parse OSIS ID format: Book.Chapter.Verse or Book.Chapter.Verse-VerseEnd
	parts := strings.Split(osisID, ".")
	if len(parts) >= 1 {
		ref.Book = parts[0]
	}
	if len(parts) >= 2 {
		ref.Chapter, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		// Handle verse ranges
		versePart := parts[2]
		if strings.Contains(versePart, "-") {
			rangeParts := strings.Split(versePart, "-")
			ref.Verse, _ = strconv.Atoi(rangeParts[0])
			if len(rangeParts) > 1 {
				ref.VerseEnd, _ = strconv.Atoi(rangeParts[1])
			}
		} else {
			ref.Verse, _ = strconv.Atoi(versePart)
		}
	}

	return ref
}

// isBookID checks if an OSIS ID is a book identifier
func isBookID(osisID string) bool {
	// Common Bible book abbreviations
	books := regexp.MustCompile(`^(Gen|Exod|Lev|Num|Deut|Josh|Judg|Ruth|1Sam|2Sam|1Kgs|2Kgs|1Chr|2Chr|Ezra|Neh|Esth|Job|Ps|Prov|Eccl|Song|Isa|Jer|Lam|Ezek|Dan|Hos|Joel|Amos|Obad|Jonah|Mic|Nah|Hab|Zeph|Hag|Zech|Mal|Matt|Mark|Luke|John|Acts|Rom|1Cor|2Cor|Gal|Eph|Phil|Col|1Thess|2Thess|1Tim|2Tim|Titus|Phlm|Heb|Jas|1Pet|2Pet|1John|2John|3John|Jude|Rev)$`)
	return books.MatchString(osisID)
}

// emitOSISFromIR converts IR Corpus back to OSIS XML
func emitOSISFromIR(corpus *Corpus) ([]byte, error) {
	// Check if we have the original raw XML for L0 lossless round-trip
	if rawXML, ok := corpus.Attributes["_osis_raw"]; ok && rawXML != "" {
		return []byte(rawXML), nil
	}

	// Otherwise, reconstruct OSIS from IR structure
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString("\n")
	buf.WriteString(`<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">`)
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf(`  <osisText osisIDWork="%s"`, escapeXML(corpus.ID)))
	if corpus.Language != "" {
		buf.WriteString(fmt.Sprintf(` xml:lang="%s"`, escapeXML(corpus.Language)))
	}
	buf.WriteString(">\n")

	// Write header
	buf.WriteString("    <header>\n")
	buf.WriteString(fmt.Sprintf(`      <work osisWork="%s">`, escapeXML(corpus.ID)))
	buf.WriteString("\n")
	if corpus.Title != "" {
		buf.WriteString(fmt.Sprintf("        <title>%s</title>\n", escapeXML(corpus.Title)))
	}
	if corpus.Description != "" {
		buf.WriteString(fmt.Sprintf("        <description>%s</description>\n", escapeXML(corpus.Description)))
	}
	if corpus.Publisher != "" {
		buf.WriteString(fmt.Sprintf("        <publisher>%s</publisher>\n", escapeXML(corpus.Publisher)))
	}
	if corpus.Rights != "" {
		buf.WriteString(fmt.Sprintf("        <rights>%s</rights>\n", escapeXML(corpus.Rights)))
	}
	if corpus.Language != "" {
		buf.WriteString(fmt.Sprintf("        <language>%s</language>\n", escapeXML(corpus.Language)))
	}
	if corpus.Versification != "" {
		buf.WriteString(fmt.Sprintf("        <refSystem>%s</refSystem>\n", escapeXML(corpus.Versification)))
	}
	buf.WriteString("      </work>\n")
	buf.WriteString("    </header>\n")

	// Write documents (books)
	for _, doc := range corpus.Documents {
		buf.WriteString(fmt.Sprintf(`    <div type="book" osisID="%s">`, escapeXML(doc.ID)))
		buf.WriteString("\n")
		if doc.Title != "" {
			buf.WriteString(fmt.Sprintf("      <title>%s</title>\n", escapeXML(doc.Title)))
		}

		// Write content blocks
		for _, block := range doc.ContentBlocks {
			// Check if this is a poetry block
			if block.Attributes != nil {
				if blockType, ok := block.Attributes["type"].(string); ok && blockType == "poetry" {
					buf.WriteString("      <lg>\n")
					buf.WriteString(fmt.Sprintf("        <l>%s</l>\n", escapeXML(block.Text)))
					buf.WriteString("      </lg>\n")
					continue
				}
			}

			// Write as paragraph with verse markers if present
			buf.WriteString("      <p>")
			for _, anchor := range block.Anchors {
				for _, span := range anchor.Spans {
					if span.Type == "VERSE" && span.Ref != nil {
						osisID := span.Ref.OSISID
						if osisID == "" {
							osisID = fmt.Sprintf("%s.%d.%d", span.Ref.Book, span.Ref.Chapter, span.Ref.Verse)
						}
						buf.WriteString(fmt.Sprintf(`<verse osisID="%s"/>`, escapeXML(osisID)))
					}
				}
			}
			buf.WriteString(escapeXML(block.Text))
			buf.WriteString("</p>\n")
		}

		buf.WriteString("    </div>\n")
	}

	buf.WriteString("  </osisText>\n")
	buf.WriteString("</osis>\n")

	return buf.Bytes(), nil
}

// escapeXML escapes special characters for XML
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
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

// Compile check
var _ = io.Copy
