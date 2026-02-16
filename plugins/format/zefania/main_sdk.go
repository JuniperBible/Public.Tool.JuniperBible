//go:build sdk

// Plugin format-zefania handles Zefania XML Bible file ingestion.
// Zefania XML is a Bible format used primarily in German-speaking regions.
//
// IR Support:
// - extract-ir: Extracts IR from Zefania XML (L0 lossless via raw storage)
// - emit-native: Converts IR back to Zefania format (L0)
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

// Zefania book number to OSIS ID mapping
var zefaniaBookToOSIS = map[int]string{
	1: "Gen", 2: "Exod", 3: "Lev", 4: "Num", 5: "Deut",
	6: "Josh", 7: "Judg", 8: "Ruth", 9: "1Sam", 10: "2Sam",
	11: "1Kgs", 12: "2Kgs", 13: "1Chr", 14: "2Chr", 15: "Ezra",
	16: "Neh", 17: "Esth", 18: "Job", 19: "Ps", 20: "Prov",
	21: "Eccl", 22: "Song", 23: "Isa", 24: "Jer", 25: "Lam",
	26: "Ezek", 27: "Dan", 28: "Hos", 29: "Joel", 30: "Amos",
	31: "Obad", 32: "Jonah", 33: "Mic", 34: "Nah", 35: "Hab",
	36: "Zeph", 37: "Hag", 38: "Zech", 39: "Mal",
	40: "Matt", 41: "Mark", 42: "Luke", 43: "John", 44: "Acts",
	45: "Rom", 46: "1Cor", 47: "2Cor", 48: "Gal", 49: "Eph",
	50: "Phil", 51: "Col", 52: "1Thess", 53: "2Thess",
	54: "1Tim", 55: "2Tim", 56: "Titus", 57: "Phlm", 58: "Heb",
	59: "Jas", 60: "1Pet", 61: "2Pet", 62: "1John", 63: "2John",
	64: "3John", 65: "Jude", 66: "Rev",
}

var osisToZefaniaBook = func() map[string]int {
	m := make(map[string]int)
	for k, v := range zefaniaBookToOSIS {
		m[v] = k
	}
	return m
}()

func main() {
	if err := format.Run(&format.Config{
		Name:       "Zefania",
		Extensions: []string{".xml"},
		Detect:     detectZefaniaWrapper,
		Parse:      parseZefaniaWrapper,
		Emit:       emitZefaniaWrapper,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// detectZefaniaWrapper reads the file and returns SDK-compatible DetectResult
func detectZefaniaWrapper(path string) (*ipc.DetectResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	detected, reason := detectZefania(data)
	return &ipc.DetectResult{
		Detected: detected,
		Format:   "Zefania",
		Reason:   reason,
	}, nil
}

// parseZefaniaWrapper reads the file and calls the existing parser
func parseZefaniaWrapper(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseZefania(data)
}

// emitZefaniaWrapper writes the corpus to the output directory
func emitZefaniaWrapper(corpus *ir.Corpus, outputDir string) (string, error) {
	data, lossReport, err := emitZefania(corpus)
	if err != nil {
		return "", err
	}

	// Determine output filename
	filename := corpus.ID
	if filename == "" {
		filename = "output"
	}
	filename = sanitizeID(filename) + ".xml"

	outputPath := filepath.Join(outputDir, filename)

	// Write the file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return "", err
	}

	// Log loss report if needed
	if lossReport != nil && len(lossReport.Warnings) > 0 {
		fmt.Fprintf(os.Stderr, "Loss report: %s\n", strings.Join(lossReport.Warnings, "; "))
	}

	return outputPath, nil
}

func detectZefania(data []byte) (bool, string) {
	content := string(data)
	// Check for Zefania XML markers
	if !strings.Contains(content, "<XMLBIBLE") && !strings.Contains(content, "<xmlbible") {
		return false, "not a Zefania XML file (no <XMLBIBLE> element)"
	}
	return true, "Zefania XML detected"
}

// parseContext holds state during XML parsing
type parseContext struct {
	corpus         *ir.Corpus
	currentBook    *ir.Document
	currentChapter int
	sequence       int
}

func parseZefania(data []byte) (*ir.Corpus, error) {
	ctx := &parseContext{
		corpus: &ir.Corpus{
			Version:      "1.0.0",
			ModuleType:   "BIBLE",
			SourceFormat: "Zefania",
			Attributes:   make(map[string]string),
		},
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	if err := parseXMLTokens(decoder, ctx); err != nil {
		return nil, err
	}

	finalizeCorpus(ctx.corpus, data)
	return ctx.corpus, nil
}

func parseXMLTokens(decoder *xml.Decoder, ctx *parseContext) error {
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if startElem, ok := token.(xml.StartElement); ok {
			if err := handleStartElement(decoder, startElem, ctx); err != nil {
				return err
			}
		}
	}
}

func handleStartElement(decoder *xml.Decoder, elem xml.StartElement, ctx *parseContext) error {
	switch strings.ToUpper(elem.Name.Local) {
	case "XMLBIBLE":
		parseXMLBibleAttrs(elem.Attr, ctx.corpus)
	case "BIBLEBOOK":
		parseBookElement(elem.Attr, ctx)
	case "CHAPTER":
		parseChapterElement(elem.Attr, ctx)
	case "VERS":
		return parseVerseElement(decoder, elem.Attr, ctx)
	}
	return nil
}

func parseXMLBibleAttrs(attrs []xml.Attr, corpus *ir.Corpus) {
	for _, attr := range attrs {
		switch strings.ToLower(attr.Name.Local) {
		case "biblename":
			corpus.Title = attr.Value
			corpus.ID = sanitizeID(attr.Value)
		case "language":
			corpus.Language = attr.Value
		}
	}
}

func parseBookElement(attrs []xml.Attr, ctx *parseContext) {
	bookNum, bookName := extractBookAttrs(attrs)
	osisID := zefaniaBookToOSIS[bookNum]
	if osisID == "" {
		osisID = sanitizeID(bookName)
	}

	ctx.currentBook = &ir.Document{
		ID:    osisID,
		Title: bookName,
		Order: bookNum,
		Attributes: map[string]string{
			"bnumber": strconv.Itoa(bookNum),
		},
	}
	ctx.corpus.Documents = append(ctx.corpus.Documents, ctx.currentBook)
}

func extractBookAttrs(attrs []xml.Attr) (int, string) {
	var bookNum int
	var bookName string
	for _, attr := range attrs {
		switch strings.ToLower(attr.Name.Local) {
		case "bnumber":
			bookNum, _ = strconv.Atoi(attr.Value)
		case "bname":
			bookName = attr.Value
		}
	}
	return bookNum, bookName
}

func parseChapterElement(attrs []xml.Attr, ctx *parseContext) {
	for _, attr := range attrs {
		if strings.ToLower(attr.Name.Local) == "cnumber" {
			ctx.currentChapter, _ = strconv.Atoi(attr.Value)
		}
	}
}

func parseVerseElement(decoder *xml.Decoder, attrs []xml.Attr, ctx *parseContext) error {
	verseNum := extractVerseNumber(attrs)
	text := readVerseText(decoder)

	if text != "" && ctx.currentBook != nil {
		ctx.sequence++
		contentBlock := createContentBlock(text, ctx, verseNum)
		ctx.currentBook.ContentBlocks = append(ctx.currentBook.ContentBlocks, contentBlock)
	}
	return nil
}

func extractVerseNumber(attrs []xml.Attr) int {
	for _, attr := range attrs {
		if strings.ToLower(attr.Name.Local) == "vnumber" {
			num, _ := strconv.Atoi(attr.Value)
			return num
		}
	}
	return 0
}

func readVerseText(decoder *xml.Decoder) string {
	var textContent strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		if end, ok := token.(xml.EndElement); ok && strings.ToUpper(end.Name.Local) == "VERS" {
			break
		}
		if charData, ok := token.(xml.CharData); ok {
			textContent.Write(charData)
		}
	}
	return strings.TrimSpace(textContent.String())
}

func createContentBlock(text string, ctx *parseContext, verseNum int) *ir.ContentBlock {
	hash := sha256.Sum256([]byte(text))
	osisID := fmt.Sprintf("%s.%d.%d", ctx.currentBook.ID, ctx.currentChapter, verseNum)

	return &ir.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", ctx.sequence),
		Sequence: ctx.sequence,
		Text:     text,
		Hash:     hex.EncodeToString(hash[:]),
		Anchors: []*ir.Anchor{
			{
				ID:       fmt.Sprintf("a-%d-0", ctx.sequence),
				Position: 0,
				Spans: []*ir.Span{
					{
						ID:            fmt.Sprintf("s-%s", osisID),
						Type:          "VERSE",
						StartAnchorID: fmt.Sprintf("a-%d-0", ctx.sequence),
						Ref: &ir.Ref{
							Book:    ctx.currentBook.ID,
							Chapter: ctx.currentChapter,
							Verse:   verseNum,
							OSISID:  osisID,
						},
					},
				},
			},
		},
	}
}

func finalizeCorpus(corpus *ir.Corpus, data []byte) {
	if corpus.ID == "" {
		corpus.ID = "zefania"
	}

	sourceHash := sha256.Sum256(data)
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L0"
	corpus.Attributes["_zefania_raw"] = string(data)
}

func sanitizeID(s string) string {
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, s)
	return strings.Trim(result, "-")
}

func emitZefania(corpus *ir.Corpus) ([]byte, *ipc.LossReport, error) {
	// Check for raw Zefania for L0 round-trip
	if raw, ok := corpus.Attributes["_zefania_raw"]; ok && raw != "" {
		return []byte(raw), &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "Zefania",
			LossClass:    "L0",
		}, nil
	}

	// Generate Zefania from IR
	zefaniaContent := emitZefaniaFromIR(corpus)
	return []byte(zefaniaContent), &ipc.LossReport{
		SourceFormat: "IR",
		TargetFormat: "Zefania",
		LossClass:    "L1",
		Warnings: []string{
			"Zefania regenerated from IR - some formatting may differ",
		},
	}, nil
}

func emitZefaniaFromIR(corpus *ir.Corpus) string {
	var buf strings.Builder

	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
`)
	buf.WriteString(fmt.Sprintf(`<XMLBIBLE biblename="%s"`, escapeXML(corpus.Title)))
	if corpus.Language != "" {
		buf.WriteString(fmt.Sprintf(` language="%s"`, escapeXML(corpus.Language)))
	}
	buf.WriteString(">\n")

	for _, doc := range corpus.Documents {
		bookNum := osisToZefaniaBook[doc.ID]
		if bookNum == 0 {
			bookNum = doc.Order
		}
		buf.WriteString(fmt.Sprintf(`  <BIBLEBOOK bnumber="%d" bname="%s">
`, bookNum, escapeXML(doc.Title)))

		currentChapter := 0
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						if span.Ref.Chapter != currentChapter {
							if currentChapter > 0 {
								buf.WriteString("    </CHAPTER>\n")
							}
							currentChapter = span.Ref.Chapter
							buf.WriteString(fmt.Sprintf(`    <CHAPTER cnumber="%d">
`, currentChapter))
						}
						buf.WriteString(fmt.Sprintf(`      <VERS vnumber="%d">%s</VERS>
`, span.Ref.Verse, escapeXML(cb.Text)))
					}
				}
			}
		}
		if currentChapter > 0 {
			buf.WriteString("    </CHAPTER>\n")
		}
		buf.WriteString("  </BIBLEBOOK>\n")
	}

	buf.WriteString("</XMLBIBLE>\n")
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
