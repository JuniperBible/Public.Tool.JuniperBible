// Package usx provides the embedded handler for USX Bible format plugin.
package usx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
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

func parseUSXToIR(data []byte) (*ir.Corpus, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))

	state := &parseState{
		corpus: &ir.Corpus{
			Version:    "1.0.0",
			ModuleType: ir.ModuleBible,
			LossClass:  ir.LossL0,
			Documents:  []*ir.Document{},
		},
	}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if err := processToken(token, state); err != nil {
			return nil, err
		}
	}

	flushFinalVerse(state)
	finalizeCorpus(state, data)

	return state.corpus, nil
}

type parseState struct {
	corpus         *ir.Corpus
	doc            *ir.Document
	currentChapter int
	currentVerse   int
	sequence       int
	textBuf        strings.Builder
}

func processToken(token xml.Token, state *parseState) error {
	switch t := token.(type) {
	case xml.StartElement:
		return handleStartElement(t, state)
	case xml.CharData:
		handleCharData(t, state)
	}
	return nil
}

func handleStartElement(elem xml.StartElement, state *parseState) error {
	switch elem.Name.Local {
	case "book":
		handleBookElement(elem, state)
	case "chapter":
		handleChapterElement(elem, state)
	case "verse":
		handleVerseElement(elem, state)
	}
	return nil
}

func handleBookElement(elem xml.StartElement, state *parseState) {
	code := getAttrValue(elem.Attr, "code")
	if code != "" {
		state.corpus.ID = code
		state.doc = &ir.Document{
			ID:            code,
			Title:         code,
			Order:         1,
			ContentBlocks: []*ir.ContentBlock{},
		}
	}
}

func handleChapterElement(elem xml.StartElement, state *parseState) {
	flushVerse(state)
	number := getAttrValue(elem.Attr, "number")
	if number != "" {
		state.currentChapter, _ = strconv.Atoi(number)
		state.currentVerse = 0
	}
}

func handleVerseElement(elem xml.StartElement, state *parseState) {
	flushVerse(state)
	number := getAttrValue(elem.Attr, "number")
	if number != "" {
		state.currentVerse, _ = strconv.Atoi(number)
	}
}

func handleCharData(charData xml.CharData, state *parseState) {
	text := strings.TrimSpace(string(charData))
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
		cb := createContentBlock(state.sequence, state.textBuf.String(), state.doc.ID, state.currentChapter, state.currentVerse)
		state.doc.ContentBlocks = append(state.doc.ContentBlocks, cb)
		state.textBuf.Reset()
	}
}

func flushFinalVerse(state *parseState) {
	flushVerse(state)
}

func finalizeCorpus(state *parseState, data []byte) {
	if state.doc != nil {
		state.corpus.Documents = []*ir.Document{state.doc}
		state.corpus.Title = state.doc.Title
	}

	h := sha256.Sum256(data)
	state.corpus.SourceHash = hex.EncodeToString(h[:])
}

func getAttrValue(attrs []xml.Attr, name string) string {
	for _, attr := range attrs {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func createContentBlock(sequence int, text, book string, chapter, verse int) *ir.ContentBlock {
	text = strings.TrimSpace(text)

	block := &ir.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", sequence),
		Sequence: sequence,
		Text:     text,
		Anchors: []*ir.Anchor{
			{
				ID:             fmt.Sprintf("a-%d-0", sequence),
				ContentBlockID: fmt.Sprintf("cb-%d", sequence),
				CharOffset:     0,
			},
		},
	}

	block.ComputeHash()
	return block
}

func emitUSXFromIR(corpus *ir.Corpus) string {
	var buf strings.Builder

	version := "3.0"

	buf.WriteString(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<usx version="%s">
`, version))

	for _, doc := range corpus.Documents {
		buf.WriteString(fmt.Sprintf(`  <book code="%s" style="id">%s</book>
`, doc.ID, doc.Title))

		for _, cb := range doc.ContentBlocks {
			// Simple heuristic: extract chapter/verse from anchors or infer from sequence
			// This is simplified - in real implementation we'd track chapter/verse properly
			if len(cb.Anchors) > 0 {
				// For now, just write as paragraph with verse markers
				// In full implementation, we'd parse the OSIS ID or track chapter/verse
				buf.WriteString(fmt.Sprintf(`  <para style="p">%s</para>
`, escapeXML(cb.Text)))
			}
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
