//go:build !sdk

// Plugin format-usx handles USX (Unified Scripture XML) file ingestion.
// USX is an XML representation of USFM developed by United Bible Societies.
//
// IR Support:
// - extract-ir: Extracts IR from USX (L0 lossless via raw storage)
// - emit-native: Converts IR back to USX format (L0)
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

// USX XML types
type USX struct {
	XMLName xml.Name    `xml:"usx"`
	Version string      `xml:"version,attr"`
	Book    *USXBook    `xml:"book"`
	Content []USXNode   `xml:",any"`
}

type USXBook struct {
	XMLName xml.Name `xml:"book"`
	Code    string   `xml:"code,attr"`
	Style   string   `xml:"style,attr"`
	Content string   `xml:",chardata"`
}

type USXNode struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Content string     `xml:",chardata"`
	Nodes   []USXNode  `xml:",any"`
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

	// Read file content
	data, err := os.ReadFile(path)
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		})
		return
	}

	// Check for USX XML structure
	content := string(data)
	if !strings.Contains(content, "<usx") {
		respond(&DetectResult{
			Detected: false,
			Reason:   "not a USX file (no <usx> element)",
		})
		return
	}

	// Try to parse as XML
	var usx USX
	if err := xml.Unmarshal(data, &usx); err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("invalid XML: %v", err),
		})
		return
	}

	respond(&DetectResult{
		Detected: true,
		Format:   "USX",
		Reason:   fmt.Sprintf("USX %s detected", usx.Version),
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

	// Try to get book ID from content
	var usx USX
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if err := xml.Unmarshal(data, &usx); err == nil && usx.Book != nil {
		artifactID = usx.Book.Code
	}

	respond(&IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "USX",
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

	entries := []EnumerateEntry{
		{
			Path:      filepath.Base(path),
			SizeBytes: info.Size(),
			IsDir:     false,
			Metadata: map[string]string{
				"format": "USX",
			},
		},
	}

	respond(&EnumerateResult{
		Entries: entries,
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

	corpus, err := parseUSXToIR(data)
	if err != nil {
		respondError(fmt.Sprintf("failed to parse USX: %v", err))
		return
	}

	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L0"

	// Store raw USX for L0 round-trip
	if corpus.Attributes == nil {
		corpus.Attributes = make(map[string]string)
	}
	corpus.Attributes["_usx_raw"] = string(data)

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
		LossClass: "L0",
		LossReport: &LossReport{
			SourceFormat: "USX",
			TargetFormat: "IR",
			LossClass:    "L0",
		},
	})
}

func parseUSXToIR(data []byte) (*Corpus, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))

	corpus := &Corpus{
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "USX",
		Attributes:   make(map[string]string),
	}

	var doc *Document
	var currentChapter, currentVerse int
	sequence := 0
	var textBuf strings.Builder

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "usx":
				for _, attr := range t.Attr {
					if attr.Name.Local == "version" {
						corpus.Attributes["usx_version"] = attr.Value
					}
				}

			case "book":
				for _, attr := range t.Attr {
					if attr.Name.Local == "code" {
						corpus.ID = attr.Value
						doc = &Document{
							ID:         attr.Value,
							Title:      attr.Value,
							Order:      1,
							Attributes: make(map[string]string),
						}
					}
					if attr.Name.Local == "style" {
						if doc != nil {
							doc.Attributes["style"] = attr.Value
						}
					}
				}

			case "chapter":
				// Flush any pending text
				if textBuf.Len() > 0 && currentVerse > 0 {
					sequence++
					cb := createContentBlock(sequence, textBuf.String(), doc.ID, currentChapter, currentVerse)
					doc.ContentBlocks = append(doc.ContentBlocks, cb)
					textBuf.Reset()
				}

				for _, attr := range t.Attr {
					if attr.Name.Local == "number" {
						currentChapter, _ = strconv.Atoi(attr.Value)
						currentVerse = 0
					}
				}

			case "verse":
				// Flush previous verse
				if textBuf.Len() > 0 && currentVerse > 0 {
					sequence++
					cb := createContentBlock(sequence, textBuf.String(), doc.ID, currentChapter, currentVerse)
					doc.ContentBlocks = append(doc.ContentBlocks, cb)
					textBuf.Reset()
				}

				for _, attr := range t.Attr {
					if attr.Name.Local == "number" {
						currentVerse, _ = strconv.Atoi(attr.Value)
					}
				}

			case "para":
				// Handle paragraph styles
				for _, attr := range t.Attr {
					if attr.Name.Local == "style" {
						switch attr.Value {
						case "h", "toc1", "toc2", "toc3", "mt", "mt1", "mt2":
							// Header content - captured in text
						}
					}
				}
			}

		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text != "" && currentVerse > 0 {
				if textBuf.Len() > 0 {
					textBuf.WriteString(" ")
				}
				textBuf.WriteString(text)
			}
		}
	}

	// Flush final verse
	if textBuf.Len() > 0 && currentVerse > 0 && doc != nil {
		sequence++
		cb := createContentBlock(sequence, textBuf.String(), doc.ID, currentChapter, currentVerse)
		doc.ContentBlocks = append(doc.ContentBlocks, cb)
	}

	if doc != nil {
		corpus.Documents = []*Document{doc}
		corpus.Title = doc.Title
	}

	return corpus, nil
}

func createContentBlock(sequence int, text, book string, chapter, verse int) *ContentBlock {
	text = strings.TrimSpace(text)
	hash := sha256.Sum256([]byte(text))
	osisID := fmt.Sprintf("%s.%d.%d", book, chapter, verse)

	return &ContentBlock{
		ID:       fmt.Sprintf("cb-%d", sequence),
		Sequence: sequence,
		Text:     text,
		Hash:     hex.EncodeToString(hash[:]),
		Anchors: []*Anchor{
			{
				ID:       fmt.Sprintf("a-%d-0", sequence),
				Position: 0,
				Spans: []*Span{
					{
						ID:            fmt.Sprintf("s-%s", osisID),
						Type:          "VERSE",
						StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
						Ref: &Ref{
							Book:    book,
							Chapter: chapter,
							Verse:   verse,
							OSISID:  osisID,
						},
					},
				},
			},
		},
	}
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

	outputPath := filepath.Join(outputDir, corpus.ID+".usx")

	// Check for raw USX for L0 round-trip
	if raw, ok := corpus.Attributes["_usx_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			respondError(fmt.Sprintf("failed to write USX: %v", err))
			return
		}

		respond(&EmitNativeResult{
			OutputPath: outputPath,
			Format:     "USX",
			LossClass:  "L0",
			LossReport: &LossReport{
				SourceFormat: "IR",
				TargetFormat: "USX",
				LossClass:    "L0",
			},
		})
		return
	}

	// Generate USX from IR
	usxContent := emitUSXFromIR(&corpus)
	if err := os.WriteFile(outputPath, []byte(usxContent), 0644); err != nil {
		respondError(fmt.Sprintf("failed to write USX: %v", err))
		return
	}

	respond(&EmitNativeResult{
		OutputPath: outputPath,
		Format:     "USX",
		LossClass:  "L1",
		LossReport: &LossReport{
			SourceFormat: "IR",
			TargetFormat: "USX",
			LossClass:    "L1",
			Warnings: []string{
				"USX regenerated from IR - some formatting may differ",
			},
		},
	})
}

func emitUSXFromIR(corpus *Corpus) string {
	var buf strings.Builder

	version := "3.0"
	if v, ok := corpus.Attributes["usx_version"]; ok {
		version = v
	}

	buf.WriteString(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<usx version="%s">
`, version))

	for _, doc := range corpus.Documents {
		buf.WriteString(fmt.Sprintf(`  <book code="%s" style="id">%s</book>
`, doc.ID, doc.Title))

		currentChapter := 0
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						if span.Ref.Chapter != currentChapter {
							if currentChapter > 0 {
								buf.WriteString("  </para>\n")
							}
							currentChapter = span.Ref.Chapter
							buf.WriteString(fmt.Sprintf(`  <chapter number="%d" style="c" sid="%s.%d"/>
  <para style="p">
`, currentChapter, doc.ID, currentChapter))
						}
						buf.WriteString(fmt.Sprintf(`    <verse number="%d" style="v" sid="%s"/>%s<verse eid="%s"/>
`, span.Ref.Verse, span.Ref.OSISID, escapeXML(cb.Text), span.Ref.OSISID))
					}
				}
			}
		}

		if currentChapter > 0 {
			buf.WriteString("  </para>\n")
		}
	}

	buf.WriteString("</usx>\n")
	return buf.String()
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
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
