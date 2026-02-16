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
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// USX XML types
type USX struct {
	XMLName xml.Name `xml:"usx"`
	Version string   `xml:"version,attr"`
	Book    *USXBook `xml:"book"`
}

type USXBook struct {
	XMLName xml.Name `xml:"book"`
	Code    string   `xml:"code,attr"`
	Style   string   `xml:"style,attr"`
	Content string   `xml:",chardata"`
}

func runSDK() {
	if err := format.Run(&format.Config{
		Name:       "USX",
		Extensions: []string{".usx", ".xml"},
		Detect:     detectUSX,
		Parse:      parseUSX,
		Emit:       emitUSX,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectUSX(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{Detected: false, Reason: "path is a directory"}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot read file: %v", err)}, nil
	}

	content := string(data)
	if !strings.Contains(content, "<usx") {
		return &ipc.DetectResult{Detected: false, Reason: "not a USX file (no <usx> element)"}, nil
	}

	var usx USX
	if err := xml.Unmarshal(data, &usx); err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("invalid XML: %v", err)}, nil
	}

	return &ipc.DetectResult{Detected: true, Format: "USX", Reason: fmt.Sprintf("USX %s detected", usx.Version)}, nil
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

	if corpus.Attributes == nil {
		corpus.Attributes = make(map[string]string)
	}
	corpus.Attributes["_usx_raw"] = string(data)

	return corpus, nil
}

type parseState struct {
	doc            *ir.Document
	currentChapter int
	currentVerse   int
	sequence       int
	textBuf        strings.Builder
}

func parseUSXToIR(data []byte) (*ir.Corpus, error) {
	corpus := ir.NewCorpus("", "BIBLE", "")
	corpus.SourceFormat = "USX"
	corpus.Attributes = make(map[string]string)

	state := &parseState{}
	decoder := xml.NewDecoder(bytes.NewReader(data))

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if err := processToken(token, corpus, state); err != nil {
			return nil, err
		}
	}

	flushTextBuffer(state)
	finalizeCorpus(corpus, state.doc)

	return corpus, nil
}

func processToken(token xml.Token, corpus *ir.Corpus, state *parseState) error {
	switch t := token.(type) {
	case xml.StartElement:
		processStartElement(t, corpus, state)
	case xml.CharData:
		processCharData(t, state)
	}
	return nil
}

func processStartElement(elem xml.StartElement, corpus *ir.Corpus, state *parseState) {
	switch elem.Name.Local {
	case "usx":
		processUSXElement(elem, corpus)
	case "book":
		processBookElement(elem, corpus, state)
	case "chapter":
		processChapterElement(elem, state)
	case "verse":
		processVerseElement(elem, state)
	}
}

func processUSXElement(elem xml.StartElement, corpus *ir.Corpus) {
	for _, attr := range elem.Attr {
		if attr.Name.Local == "version" {
			corpus.Attributes["usx_version"] = attr.Value
		}
	}
}

func processBookElement(elem xml.StartElement, corpus *ir.Corpus, state *parseState) {
	for _, attr := range elem.Attr {
		if attr.Name.Local == "code" {
			corpus.ID = attr.Value
			state.doc = ir.NewDocument(attr.Value, attr.Value, 1)
		}
		if attr.Name.Local == "style" && state.doc != nil {
			if state.doc.Attributes == nil {
				state.doc.Attributes = make(map[string]string)
			}
			state.doc.Attributes["style"] = attr.Value
		}
	}
}

func processChapterElement(elem xml.StartElement, state *parseState) {
	flushTextBuffer(state)

	for _, attr := range elem.Attr {
		if attr.Name.Local == "number" {
			state.currentChapter, _ = strconv.Atoi(attr.Value)
			state.currentVerse = 0
		}
	}
}

func processVerseElement(elem xml.StartElement, state *parseState) {
	flushTextBuffer(state)

	for _, attr := range elem.Attr {
		if attr.Name.Local == "number" {
			state.currentVerse, _ = strconv.Atoi(attr.Value)
		}
	}
}

func processCharData(charData xml.CharData, state *parseState) {
	text := strings.TrimSpace(string(charData))
	if text != "" && state.currentVerse > 0 {
		if state.textBuf.Len() > 0 {
			state.textBuf.WriteString(" ")
		}
		state.textBuf.WriteString(text)
	}
}

func flushTextBuffer(state *parseState) {
	if state.textBuf.Len() > 0 && state.currentVerse > 0 && state.doc != nil {
		state.sequence++
		cb := createContentBlock(state.sequence, state.textBuf.String(), state.doc.ID, state.currentChapter, state.currentVerse)
		state.doc.ContentBlocks = append(state.doc.ContentBlocks, cb)
		state.textBuf.Reset()
	}
}

func finalizeCorpus(corpus *ir.Corpus, doc *ir.Document) {
	if doc != nil {
		corpus.Documents = []*ir.Document{doc}
		corpus.Title = doc.Title
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
		Anchors: []*ir.Anchor{{
			ID:       fmt.Sprintf("a-%d-0", sequence),
			Position: 0,
			Spans: []*ir.Span{{
				ID:            fmt.Sprintf("s-%s", osisID),
				Type:          "VERSE",
				StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
				Ref:           &ir.Ref{Book: book, Chapter: chapter, Verse: verse, OSISID: osisID},
			}},
		}},
	}
}

func emitUSX(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".usx")

	// Check for raw USX for L0 round-trip
	if raw, ok := corpus.Attributes["_usx_raw"]; ok && raw != "" {
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

func emitUSXFromIR(corpus *ir.Corpus) string {
	var buf strings.Builder

	version := "3.0"
	if v, ok := corpus.Attributes["usx_version"]; ok {
		version = v
	}

	buf.WriteString(fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<usx version=\"%s\">\n", version))

	for _, doc := range corpus.Documents {
		buf.WriteString(fmt.Sprintf("  <book code=\"%s\" style=\"id\">%s</book>\n", doc.ID, doc.Title))

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
							buf.WriteString(fmt.Sprintf("  <chapter number=\"%d\" style=\"c\" sid=\"%s.%d\"/>\n  <para style=\"p\">\n",
								currentChapter, doc.ID, currentChapter))
						}
						buf.WriteString(fmt.Sprintf("    <verse number=\"%d\" style=\"v\" sid=\"%s\"/>%s<verse eid=\"%s\"/>\n",
							span.Ref.Verse, span.Ref.OSISID, escapeXML(cb.Text), span.Ref.OSISID))
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
