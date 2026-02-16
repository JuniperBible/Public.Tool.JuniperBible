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
	if err := format.Run(&format.Config{
		Name:       "USX",
		Extensions: []string{".usx"},
		Detect:     detectUSX,
		Parse:      parseUSX,
		Emit:       emitUSX,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
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
	corpus.Attributes["_usx_raw"] = string(data)

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

	var doc *ir.Document
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
						doc = &ir.Document{
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
		corpus.Documents = []*ir.Document{doc}
		corpus.Title = doc.Title
	}

	return corpus, nil
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

	outputPath := filepath.Join(outputDir, corpus.ID+".usx")

	// Check for raw USX for L0 round-trip
	if raw, ok := corpus.Attributes["_usx_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			return "", fmt.Errorf("failed to write USX: %w", err)
		}
		return outputPath, nil
	}

	// Generate USX from IR
	usxContent := emitUSXFromIR(corpus)
	if err := os.WriteFile(outputPath, []byte(usxContent), 0644); err != nil {
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
