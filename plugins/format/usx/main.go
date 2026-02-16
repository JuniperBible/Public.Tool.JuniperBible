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

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// USX XML types
type USX struct {
	XMLName xml.Name  `xml:"usx"`
	Version string    `xml:"version,attr"`
	Book    *USXBook  `xml:"book"`
	Content []USXNode `xml:",any"`
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

func main() {
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorfAndExit("failed to decode request: %v", err)
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

	// Read file content
	data, err := os.ReadFile(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		})
		return
	}

	// Check for USX XML structure
	content := string(data)
	if !strings.Contains(content, "<usx") {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not a USX file (no <usx> element)",
		})
		return
	}

	// Try to parse as XML
	var usx USX
	if err := xml.Unmarshal(data, &usx); err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("invalid XML: %v", err),
		})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   "USX",
		Reason:   fmt.Sprintf("USX %s detected", usx.Version),
	})
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
	if err := os.MkdirAll(blobDir, 0700); err != nil {
		ipc.RespondErrorf("failed to create blob dir: %v", err)
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0600); err != nil {
		ipc.RespondErrorf("failed to write blob: %v", err)
		return
	}

	// Try to get book ID from content
	var usx USX
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if err := xml.Unmarshal(data, &usx); err == nil && usx.Book != nil {
		artifactID = usx.Book.Code
	}
	ipc.MustRespond(&ipc.IngestResult{
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
				"format": "USX",
			},
		},
	}
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: entries,
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

	corpus, err := parseUSXToIR(data)
	if err != nil {
		ipc.RespondErrorf("failed to parse USX: %v", err)
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
		ipc.RespondErrorf("failed to serialize IR: %v", err)
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0600); err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}
	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L0",
		LossReport: &ipc.LossReport{
			SourceFormat: "USX",
			TargetFormat: "IR",
			LossClass:    "L0",
		},
	})
}

func parseUSXToIR(data []byte) (*ipc.Corpus, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	corpus := createEmptyCorpus()
	state := newParseState()

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
			handleStartElement(t, corpus, state)
		case xml.CharData:
			handleCharData(t, state)
		}
	}

	flushFinalVerse(state, corpus)
	finalizeCorpus(corpus, state)
	return corpus, nil
}

type parseState struct {
	doc            *ipc.Document
	currentChapter int
	currentVerse   int
	sequence       int
	textBuf        strings.Builder
}

func newParseState() *parseState {
	return &parseState{}
}

func createEmptyCorpus() *ipc.Corpus {
	return &ipc.Corpus{
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "USX",
		Attributes:   make(map[string]string),
	}
}

func handleStartElement(elem xml.StartElement, corpus *ipc.Corpus, state *parseState) {
	switch elem.Name.Local {
	case "usx":
		handleUSXElement(elem, corpus)
	case "book":
		handleBookElement(elem, corpus, state)
	case "chapter":
		handleChapterElement(elem, state)
	case "verse":
		handleVerseElement(elem, state)
	}
}

func handleUSXElement(elem xml.StartElement, corpus *ipc.Corpus) {
	for _, attr := range elem.Attr {
		if attr.Name.Local == "version" {
			corpus.Attributes["usx_version"] = attr.Value
		}
	}
}

func handleBookElement(elem xml.StartElement, corpus *ipc.Corpus, state *parseState) {
	for _, attr := range elem.Attr {
		if attr.Name.Local == "code" {
			corpus.ID = attr.Value
			state.doc = &ipc.Document{
				ID:         attr.Value,
				Title:      attr.Value,
				Order:      1,
				Attributes: make(map[string]string),
			}
		}
		if attr.Name.Local == "style" && state.doc != nil {
			state.doc.Attributes["style"] = attr.Value
		}
	}
}

func handleChapterElement(elem xml.StartElement, state *parseState) {
	flushPendingVerse(state)
	for _, attr := range elem.Attr {
		if attr.Name.Local == "number" {
			state.currentChapter, _ = strconv.Atoi(attr.Value)
			state.currentVerse = 0
		}
	}
}

func handleVerseElement(elem xml.StartElement, state *parseState) {
	flushPendingVerse(state)
	for _, attr := range elem.Attr {
		if attr.Name.Local == "number" {
			state.currentVerse, _ = strconv.Atoi(attr.Value)
		}
	}
}

func handleCharData(data xml.CharData, state *parseState) {
	text := strings.TrimSpace(string(data))
	if text != "" && state.currentVerse > 0 {
		if state.textBuf.Len() > 0 {
			state.textBuf.WriteString(" ")
		}
		state.textBuf.WriteString(text)
	}
}

func flushPendingVerse(state *parseState) {
	if state.textBuf.Len() > 0 && state.currentVerse > 0 && state.doc != nil {
		state.sequence++
		cb := createContentBlock(state.sequence, state.textBuf.String(), state.doc.ID, state.currentChapter, state.currentVerse)
		state.doc.ContentBlocks = append(state.doc.ContentBlocks, cb)
		state.textBuf.Reset()
	}
}

func flushFinalVerse(state *parseState, corpus *ipc.Corpus) {
	flushPendingVerse(state)
}

func finalizeCorpus(corpus *ipc.Corpus, state *parseState) {
	if state.doc != nil {
		corpus.Documents = []*ipc.Document{state.doc}
		corpus.Title = state.doc.Title
	}
}

func createContentBlock(sequence int, text, book string, chapter, verse int) *ipc.ContentBlock {
	text = strings.TrimSpace(text)
	hash := sha256.Sum256([]byte(text))
	osisID := fmt.Sprintf("%s.%d.%d", book, chapter, verse)

	return &ipc.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", sequence),
		Sequence: sequence,
		Text:     text,
		Hash:     hex.EncodeToString(hash[:]),
		Anchors: []*ipc.Anchor{
			{
				ID:       fmt.Sprintf("a-%d-0", sequence),
				Position: 0,
				Spans: []*ipc.Span{
					{
						ID:            fmt.Sprintf("s-%s", osisID),
						Type:          "VERSE",
						StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
						Ref: &ipc.Ref{
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

	outputPath := filepath.Join(outputDir, corpus.ID+".usx")

	// Check for raw USX for L0 round-trip
	if raw, ok := corpus.Attributes["_usx_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
			ipc.RespondErrorf("failed to write USX: %v", err)
			return
		}
		ipc.MustRespond(&ipc.EmitNativeResult{
			OutputPath: outputPath,
			Format:     "USX",
			LossClass:  "L0",
			LossReport: &ipc.LossReport{
				SourceFormat: "IR",
				TargetFormat: "USX",
				LossClass:    "L0",
			},
		})
		return
	}

	// Generate USX from IR
	usxContent := emitUSXFromIR(&corpus)
	if err := os.WriteFile(outputPath, []byte(usxContent), 0600); err != nil {
		ipc.RespondErrorf("failed to write USX: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "USX",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "USX",
			LossClass:    "L1",
			Warnings: []string{
				"USX regenerated from IR - some formatting may differ",
			},
		},
	})
}

func emitUSXFromIR(corpus *ipc.Corpus) string {
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
