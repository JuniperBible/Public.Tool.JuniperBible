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
		Detect:     detectZefania,
		Parse:      parseZefania,
		Emit:       emitZefania,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectZefania(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{Detected: false, Reason: "path is a directory, not a file"}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot read file: %v", err)}, nil
	}

	content := string(data)
	if !strings.Contains(content, "<XMLBIBLE") && !strings.Contains(content, "<xmlbible") {
		return &ipc.DetectResult{Detected: false, Reason: "not a Zefania XML file (no <XMLBIBLE> element)"}, nil
	}

	return &ipc.DetectResult{Detected: true, Format: "Zefania", Reason: "Zefania XML detected"}, nil
}

func parseZefania(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	corpus, err := parseZefaniaToIR(data)
	if err != nil {
		return nil, err
	}

	sourceHash := sha256.Sum256(data)
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L0"

	if corpus.Attributes == nil {
		corpus.Attributes = make(map[string]string)
	}
	corpus.Attributes["_zefania_raw"] = string(data)

	return corpus, nil
}

// parserState holds the parsing context during XML processing
type parserState struct {
	corpus         *ir.Corpus
	currentBook    *ir.Document
	currentChapter int
	sequence       int
}

func parseZefaniaToIR(data []byte) (*ir.Corpus, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	state := &parserState{
		corpus: ir.NewCorpus("", "BIBLE", ""),
	}
	state.corpus.SourceFormat = "Zefania"

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if startElem, ok := token.(xml.StartElement); ok {
			processElement(decoder, startElem, state)
		}
	}

	if state.corpus.ID == "" {
		state.corpus.ID = "zefania"
	}

	return state.corpus, nil
}

// processElement dispatches element processing based on element name
func processElement(decoder *xml.Decoder, elem xml.StartElement, state *parserState) {
	elementHandlers := map[string]func(*xml.Decoder, xml.StartElement, *parserState){
		"XMLBIBLE":   processXMLBibleElement,
		"BIBLEBOOK":  processBibleBookElement,
		"CHAPTER":    processChapterElement,
		"VERS":       processVerseElement,
	}

	if handler, ok := elementHandlers[strings.ToUpper(elem.Name.Local)]; ok {
		handler(decoder, elem, state)
	}
}

// processXMLBibleElement handles XMLBIBLE root element
func processXMLBibleElement(decoder *xml.Decoder, elem xml.StartElement, state *parserState) {
	for _, attr := range elem.Attr {
		switch strings.ToLower(attr.Name.Local) {
		case "biblename":
			state.corpus.Title = attr.Value
			state.corpus.ID = sanitizeID(attr.Value)
		case "language":
			state.corpus.Language = attr.Value
		}
	}
}

// processBibleBookElement handles BIBLEBOOK element
func processBibleBookElement(decoder *xml.Decoder, elem xml.StartElement, state *parserState) {
	var bookNum int
	var bookName string

	for _, attr := range elem.Attr {
		switch strings.ToLower(attr.Name.Local) {
		case "bnumber":
			bookNum, _ = strconv.Atoi(attr.Value)
		case "bname":
			bookName = attr.Value
		}
	}

	osisID := zefaniaBookToOSIS[bookNum]
	if osisID == "" {
		osisID = sanitizeID(bookName)
	}

	state.currentBook = ir.NewDocument(osisID, bookName, bookNum)
	state.currentBook.Attributes = map[string]string{"bnumber": strconv.Itoa(bookNum)}
	state.corpus.Documents = append(state.corpus.Documents, state.currentBook)
}

// processChapterElement handles CHAPTER element
func processChapterElement(decoder *xml.Decoder, elem xml.StartElement, state *parserState) {
	for _, attr := range elem.Attr {
		if strings.ToLower(attr.Name.Local) == "cnumber" {
			state.currentChapter, _ = strconv.Atoi(attr.Value)
			return
		}
	}
}

// processVerseElement handles VERS element
func processVerseElement(decoder *xml.Decoder, elem xml.StartElement, state *parserState) {
	verseNum := extractVerseNumber(elem)
	text := readVerseContent(decoder)

	if text != "" && state.currentBook != nil {
		state.sequence++
		cb := createContentBlock(text, state.currentBook.ID, state.currentChapter, verseNum, state.sequence)
		state.currentBook.ContentBlocks = append(state.currentBook.ContentBlocks, cb)
	}
}

// extractVerseNumber extracts the verse number from VERS element attributes
func extractVerseNumber(elem xml.StartElement) int {
	for _, attr := range elem.Attr {
		if strings.ToLower(attr.Name.Local) == "vnumber" {
			verseNum, _ := strconv.Atoi(attr.Value)
			return verseNum
		}
	}
	return 0
}

// readVerseContent reads the text content from a VERS element
func readVerseContent(decoder *xml.Decoder) string {
	var textContent strings.Builder
	for {
		innerToken, err := decoder.Token()
		if err != nil {
			break
		}
		if end, ok := innerToken.(xml.EndElement); ok && strings.ToUpper(end.Name.Local) == "VERS" {
			break
		}
		if charData, ok := innerToken.(xml.CharData); ok {
			textContent.Write(charData)
		}
	}
	return strings.TrimSpace(textContent.String())
}

// createContentBlock creates an IR ContentBlock for a verse
func createContentBlock(text, bookID string, chapter, verse, sequence int) *ir.ContentBlock {
	hash := sha256.Sum256([]byte(text))
	osisID := fmt.Sprintf("%s.%d.%d", bookID, chapter, verse)

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
							Book:    bookID,
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

func sanitizeID(s string) string {
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, s)
	return strings.Trim(result, "-")
}

func emitZefania(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".xml")

	// Check for raw Zefania for L0 round-trip
	if raw, ok := corpus.Attributes["_zefania_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			return "", fmt.Errorf("failed to write Zefania: %w", err)
		}
		return outputPath, nil
	}

	// Generate Zefania from IR
	zefaniaContent := emitZefaniaFromIR(corpus)
	if err := os.WriteFile(outputPath, []byte(zefaniaContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write Zefania: %w", err)
	}

	return outputPath, nil
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
