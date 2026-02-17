// Package zefania provides the canonical Zefania XML Bible format implementation.
package zefania

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

// Config defines the Zefania format plugin configuration.
var Config = &format.Config{
	PluginID:   "format.zefania",
	Name:       "Zefania",
	Extensions: []string{".xml"},
	Detect:     detectZefania,
	Parse:      parseZefania,
	Emit:       emitZefania,
}

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

type parseState struct {
	corpus         *ir.Corpus
	currentBook    *ir.Document
	currentChapter int
	sequence       int
}

func detectZefania(path string) (*ipc.DetectResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		}, nil
	}

	content := string(data)
	// Check for Zefania XML markers
	if !strings.Contains(content, "<XMLBIBLE") && !strings.Contains(content, "<xmlbible") {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not a Zefania XML file (no <XMLBIBLE> element)",
		}, nil
	}
	return &ipc.DetectResult{
		Detected: true,
		Format:   "Zefania",
		Reason:   "Zefania XML detected",
	}, nil
}

func parseZefania(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)

	corpus, err := parseZefaniaToIR(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Zefania: %w", err)
	}

	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L0"

	if corpus.Attributes == nil {
		corpus.Attributes = make(map[string]string)
	}
	corpus.Attributes["_format_raw"] = string(data)

	return corpus, nil
}

func parseZefaniaToIR(data []byte) (*ir.Corpus, error) {
	state := initializeParseState()
	decoder := xml.NewDecoder(bytes.NewReader(data))

	if err := processXMLTokens(decoder, state); err != nil {
		return nil, err
	}

	finalizeCorpus(state.corpus)
	return state.corpus, nil
}

func initializeParseState() *parseState {
	return &parseState{
		corpus: &ir.Corpus{
			Version:      "1.0.0",
			ModuleType:   "BIBLE",
			SourceFormat: "Zefania",
			Attributes:   make(map[string]string),
		},
	}
}

func processXMLTokens(decoder *xml.Decoder, state *parseState) error {
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if startElem, ok := token.(xml.StartElement); ok {
			if err := processStartElement(decoder, state, startElem); err != nil {
				return err
			}
		}
	}
}

func finalizeCorpus(corpus *ir.Corpus) {
	if corpus.ID == "" {
		corpus.ID = "zefania"
	}
}

func processStartElement(decoder *xml.Decoder, state *parseState, elem xml.StartElement) error {
	handlers := map[string]func(*xml.Decoder, *parseState, xml.StartElement) error{
		"XMLBIBLE":  handleXMLBible,
		"BIBLEBOOK": handleBibleBook,
		"CHAPTER":   handleChapter,
		"VERS":      handleVerse,
	}

	if handler, ok := handlers[strings.ToUpper(elem.Name.Local)]; ok {
		return handler(decoder, state, elem)
	}
	return nil
}

func handleXMLBible(decoder *xml.Decoder, state *parseState, elem xml.StartElement) error {
	for _, attr := range elem.Attr {
		switch strings.ToLower(attr.Name.Local) {
		case "biblename":
			state.corpus.Title = attr.Value
			state.corpus.ID = sanitizeID(attr.Value)
		case "language":
			state.corpus.Language = attr.Value
		}
	}
	return nil
}

func handleBibleBook(decoder *xml.Decoder, state *parseState, elem xml.StartElement) error {
	bookNum, bookName := extractBookAttributes(elem.Attr)
	osisID := resolveOSISID(bookNum, bookName)

	state.currentBook = &ir.Document{
		ID:    osisID,
		Title: bookName,
		Order: bookNum,
		Attributes: map[string]string{
			"bnumber": strconv.Itoa(bookNum),
		},
	}
	state.corpus.Documents = append(state.corpus.Documents, state.currentBook)
	return nil
}

func handleChapter(decoder *xml.Decoder, state *parseState, elem xml.StartElement) error {
	for _, attr := range elem.Attr {
		if strings.ToLower(attr.Name.Local) == "cnumber" {
			state.currentChapter, _ = strconv.Atoi(attr.Value)
		}
	}
	return nil
}

func handleVerse(decoder *xml.Decoder, state *parseState, elem xml.StartElement) error {
	verseNum := extractVerseNumber(elem.Attr)
	text, err := readVerseContent(decoder)
	if err != nil {
		return err
	}

	if text != "" && state.currentBook != nil {
		state.sequence++
		cb := createContentBlock(state, text, verseNum)
		state.currentBook.ContentBlocks = append(state.currentBook.ContentBlocks, cb)
	}
	return nil
}

func extractBookAttributes(attrs []xml.Attr) (bookNum int, bookName string) {
	for _, attr := range attrs {
		switch strings.ToLower(attr.Name.Local) {
		case "bnumber":
			bookNum, _ = strconv.Atoi(attr.Value)
		case "bname":
			bookName = attr.Value
		}
	}
	return
}

func resolveOSISID(bookNum int, bookName string) string {
	if osisID := zefaniaBookToOSIS[bookNum]; osisID != "" {
		return osisID
	}
	return sanitizeID(bookName)
}

func extractVerseNumber(attrs []xml.Attr) int {
	for _, attr := range attrs {
		if strings.ToLower(attr.Name.Local) == "vnumber" {
			verseNum, _ := strconv.Atoi(attr.Value)
			return verseNum
		}
	}
	return 0
}

func readVerseContent(decoder *xml.Decoder) (string, error) {
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
	return strings.TrimSpace(textContent.String()), nil
}

func createContentBlock(state *parseState, text string, verseNum int) *ir.ContentBlock {
	hash := sha256.Sum256([]byte(text))
	osisID := fmt.Sprintf("%s.%d.%d", state.currentBook.ID, state.currentChapter, verseNum)

	return &ir.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", state.sequence),
		Sequence: state.sequence,
		Text:     text,
		Hash:     hex.EncodeToString(hash[:]),
		Anchors: []*ir.Anchor{
			{
				ID:       fmt.Sprintf("a-%d-0", state.sequence),
				Position: 0,
				Spans: []*ir.Span{
					{
						ID:            fmt.Sprintf("s-%s", osisID),
						Type:          "VERSE",
						StartAnchorID: fmt.Sprintf("a-%d-0", state.sequence),
						Ref: &ir.Ref{
							Book:    state.currentBook.ID,
							Chapter: state.currentChapter,
							Verse:   verseNum,
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
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".xml")

	// Check for raw Zefania for L0 round-trip
	if raw, ok := corpus.Attributes["_format_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
			return "", fmt.Errorf("failed to write Zefania: %w", err)
		}
		return outputPath, nil
	}

	// Generate Zefania from IR
	zefaniaContent := emitZefaniaFromIR(corpus)
	if err := os.WriteFile(outputPath, []byte(zefaniaContent), 0600); err != nil {
		return "", fmt.Errorf("failed to write Zefania: %w", err)
	}

	return outputPath, nil
}

func emitZefaniaFromIR(corpus *ir.Corpus) string {
	var buf strings.Builder
	writeXMLBibleHeader(&buf, corpus)
	for _, doc := range corpus.Documents {
		writeBook(&buf, doc)
	}
	buf.WriteString("</XMLBIBLE>\n")
	return buf.String()
}

func writeXMLBibleHeader(buf *strings.Builder, corpus *ir.Corpus) {
	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buf.WriteString(fmt.Sprintf(`<XMLBIBLE biblename="%s"`, escapeXML(corpus.Title)))
	if corpus.Language != "" {
		buf.WriteString(fmt.Sprintf(` language="%s"`, escapeXML(corpus.Language)))
	}
	buf.WriteString(">\n")
}

func resolveBookNumber(doc *ir.Document) int {
	if n := osisToZefaniaBook[doc.ID]; n != 0 {
		return n
	}
	return doc.Order
}

func writeBook(buf *strings.Builder, doc *ir.Document) {
	bookNum := resolveBookNumber(doc)
	buf.WriteString(fmt.Sprintf("  <BIBLEBOOK bnumber=\"%d\" bname=\"%s\">\n", bookNum, escapeXML(doc.Title)))
	currentChapter := writeChaptersFromBlocks(buf, doc.ContentBlocks)
	if currentChapter > 0 {
		buf.WriteString("    </CHAPTER>\n")
	}
	buf.WriteString("  </BIBLEBOOK>\n")
}

func writeChaptersFromBlocks(buf *strings.Builder, blocks []*ir.ContentBlock) int {
	currentChapter := 0
	for _, cb := range blocks {
		for _, anchor := range cb.Anchors {
			for _, span := range anchor.Spans {
				currentChapter = writeVerseSpan(buf, cb.Text, span, currentChapter)
			}
		}
	}
	return currentChapter
}

func writeVerseSpan(buf *strings.Builder, text string, span *ir.Span, currentChapter int) int {
	if span.Ref == nil || span.Type != "VERSE" {
		return currentChapter
	}
	if span.Ref.Chapter != currentChapter {
		if currentChapter > 0 {
			buf.WriteString("    </CHAPTER>\n")
		}
		currentChapter = span.Ref.Chapter
		buf.WriteString(fmt.Sprintf("    <CHAPTER cnumber=\"%d\">\n", currentChapter))
	}
	buf.WriteString(fmt.Sprintf("      <VERS vnumber=\"%d\">%s</VERS>\n", span.Ref.Verse, escapeXML(text)))
	return currentChapter
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
