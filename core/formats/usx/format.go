// Package usx provides the canonical USX (Unified Scripture XML) Bible format implementation.
package usx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/format"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/ir"
)

// Config defines the USX format plugin configuration.
var Config = &format.Config{
	PluginID:   "format.usx",
	Name:       "USX",
	Extensions: []string{".usx"},
	Detect:     detectUSXWrapper,
	Parse:      parseUSX,
	Emit:       emitUSX,
}

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

type parseState struct {
	corpus                       *ir.Corpus
	doc                          *ir.Document
	currentChapter, currentVerse int
	sequence                     int
	textBuf                      strings.Builder
}

func detectUSXWrapper(path string) (*ipc.DetectResult, error) {
	detected, reason, err := detectUSX(path)
	if err != nil {
		return nil, err
	}
	return &ipc.DetectResult{
		Detected: detected,
		Format:   "USX",
		Reason:   reason,
	}, nil
}

func detectUSX(path string) (bool, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Sprintf("cannot read file: %v", err), nil
	}

	// Check for USX XML structure
	content := string(data)
	if !strings.Contains(content, "<usx") {
		return false, "not a USX file (no <usx> element)", nil
	}

	// Try to parse as XML
	var usx USX
	if err := xml.Unmarshal(data, &usx); err != nil {
		return false, fmt.Sprintf("invalid XML: %v", err), nil
	}

	return true, fmt.Sprintf("USX %s detected", usx.Version), nil
}

func parseUSX(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)

	corpus, err := parseUSXToIR(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse USX: %w", err)
	}

	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L0"

	// Store raw USX for L0 round-trip
	if corpus.Attributes == nil {
		corpus.Attributes = make(map[string]string)
	}
	corpus.Attributes["_format_raw"] = string(data)

	return corpus, nil
}

func parseUSXToIR(data []byte) (*ir.Corpus, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	corpus := &ir.Corpus{
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "USX",
		Attributes:   make(map[string]string),
	}

	state := &parseState{
		corpus: corpus,
	}

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
			processStartElement(t, state)
		case xml.CharData:
			processCharData(t, state)
		}
	}

	flushFinalVerse(state)
	finalizeCorpus(state)

	return corpus, nil
}

func processStartElement(elem xml.StartElement, state *parseState) {
	switch elem.Name.Local {
	case "usx":
		processUSXElement(elem, state)
	case "book":
		processBookElement(elem, state)
	case "chapter":
		processChapterElement(elem, state)
	case "verse":
		processVerseElement(elem, state)
	}
}

func processUSXElement(elem xml.StartElement, state *parseState) {
	for _, attr := range elem.Attr {
		if attr.Name.Local == "version" {
			state.corpus.Attributes["usx_version"] = attr.Value
		}
	}
}

func processBookElement(elem xml.StartElement, state *parseState) {
	var code, style string
	for _, attr := range elem.Attr {
		if attr.Name.Local == "code" {
			code = attr.Value
		} else if attr.Name.Local == "style" {
			style = attr.Value
		}
	}

	if code != "" {
		state.corpus.ID = code
		state.doc = &ir.Document{
			ID:         code,
			Title:      code,
			Order:      1,
			Attributes: make(map[string]string),
		}
		if style != "" {
			state.doc.Attributes["style"] = style
		}
	}
}

func processChapterElement(elem xml.StartElement, state *parseState) {
	flushVerse(state)
	for _, attr := range elem.Attr {
		if attr.Name.Local == "number" {
			state.currentChapter, _ = strconv.Atoi(attr.Value)
			state.currentVerse = 0
		}
	}
}

func processVerseElement(elem xml.StartElement, state *parseState) {
	flushVerse(state)
	for _, attr := range elem.Attr {
		if attr.Name.Local == "number" {
			state.currentVerse, _ = strconv.Atoi(attr.Value)
		}
	}
}

func processCharData(data xml.CharData, state *parseState) {
	text := strings.TrimSpace(string(data))
	if text != "" && state.currentVerse > 0 {
		if state.textBuf.Len() > 0 {
			state.textBuf.WriteString(" ")
		}
		state.textBuf.WriteString(text)
	}
}

func flushVerse(state *parseState) {
	if state.textBuf.Len() > 0 && state.currentVerse > 0 && state.doc != nil {
		state.sequence++
		cb := createContentBlock(state.sequence, state.textBuf.String(),
			state.doc.ID, state.currentChapter, state.currentVerse)
		state.doc.ContentBlocks = append(state.doc.ContentBlocks, cb)
		state.textBuf.Reset()
	}
}

func flushFinalVerse(state *parseState) {
	flushVerse(state)
}

func finalizeCorpus(state *parseState) {
	if state.doc != nil {
		state.corpus.Documents = []*ir.Document{state.doc}
		state.corpus.Title = state.doc.Title
	}
}

func createContentBlock(sequence int, text, book string, chapter, verse int) *ir.ContentBlock {
	text = strings.TrimSpace(text)
	hash := sha256.Sum256([]byte(text))
	osisID := fmt.Sprintf("%s.%d.%d", book, chapter, verse)

	return &ir.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", sequence),
		Sequence: sequence,
		Text:     text,
		Hash:     hex.EncodeToString(hash[:]),
		Anchors: []*ir.Anchor{
			{
				ID:       fmt.Sprintf("a-%d-0", sequence),
				Position: 0,
				Spans: []*ir.Span{
					{
						ID:            fmt.Sprintf("s-%s", osisID),
						Type:          "VERSE",
						StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
						Ref: &ir.Ref{
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

func emitUSX(corpus *ir.Corpus, outputDir string) (string, error) {
	if corpus.ID == "" {
		return "", fmt.Errorf("corpus ID is required")
	}

	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".usx")

	// Check for raw USX for L0 round-trip
	if raw, ok := corpus.Attributes["_format_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
			return "", fmt.Errorf("failed to write USX: %w", err)
		}
		return outputPath, nil
	}

	// Generate USX from IR
	usxContent := emitUSXFromIR(corpus)
	if err := os.WriteFile(outputPath, []byte(usxContent), 0600); err != nil {
		return "", fmt.Errorf("failed to write USX: %w", err)
	}

	return outputPath, nil
}

func getUSXVersion(corpus *ir.Corpus) string {
	if v, ok := corpus.Attributes["usx_version"]; ok {
		return v
	}
	return "3.0"
}

func emitVerseSpan(buf *strings.Builder, span *ir.Span, text string, docID string, currentChapter *int) {
	if span.Ref == nil || span.Type != "VERSE" {
		return
	}
	if span.Ref.Chapter != *currentChapter {
		if *currentChapter > 0 {
			buf.WriteString("  </para>\n")
		}
		*currentChapter = span.Ref.Chapter
		buf.WriteString(fmt.Sprintf("  <chapter number=\"%d\" style=\"c\" sid=\"%s.%d\"/>\n  <para style=\"p\">\n", *currentChapter, docID, *currentChapter))
	}
	buf.WriteString(fmt.Sprintf("    <verse number=\"%d\" style=\"v\" sid=\"%s\"/>%s<verse eid=\"%s\"/>\n", span.Ref.Verse, span.Ref.OSISID, escapeXML(text), span.Ref.OSISID))
}

func emitDocumentBlocks(buf *strings.Builder, doc *ir.Document) {
	buf.WriteString(fmt.Sprintf("  <book code=\"%s\" style=\"id\">%s</book>\n", doc.ID, doc.Title))
	currentChapter := 0
	for _, cb := range doc.ContentBlocks {
		for _, anchor := range cb.Anchors {
			for _, span := range anchor.Spans {
				emitVerseSpan(buf, span, cb.Text, doc.ID, &currentChapter)
			}
		}
	}
	if currentChapter > 0 {
		buf.WriteString("  </para>\n")
	}
}

func emitUSXFromIR(corpus *ir.Corpus) string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<usx version=\"%s\">\n", getUSXVersion(corpus)))
	for _, doc := range corpus.Documents {
		emitDocumentBlocks(&buf, doc)
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
