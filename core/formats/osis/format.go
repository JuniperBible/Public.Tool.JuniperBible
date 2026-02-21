// Package osis provides the canonical OSIS Bible format implementation.
package osis

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
	"github.com/JuniperBible/juniper/plugins/sdk/ir"
)

// Config defines the OSIS format plugin configuration.
var Config = &format.Config{
	PluginID:   "format.osis",
	Name:       "OSIS",
	Extensions: []string{".osis", ".xml"},
	Detect:     detectFunc,
	Parse:      parseFunc,
	Emit:       emitFunc,
}

// OSIS XML Types

// OSISDoc represents the root OSIS document structure.
type OSISDoc struct {
	XMLName   xml.Name `xml:"osis"`
	Namespace string   `xml:"xmlns,attr,omitempty"`
	OsisText  OSISText `xml:"osisText"`
	RawXML    string   `xml:"-"` // Store original for L0 round-trip
}

// OSISText represents the main text container in OSIS.
type OSISText struct {
	OsisIDWork  string      `xml:"osisIDWork,attr"`
	OsisRefWork string      `xml:"osisRefWork,attr,omitempty"`
	Lang        string      `xml:"lang,attr,omitempty"`
	XMLLang     string      `xml:"http://www.w3.org/XML/1998/namespace lang,attr,omitempty"`
	Header      *OSISHeader `xml:"header,omitempty"`
	Divs        []OSISDiv   `xml:"div"`
}

// OSISHeader contains metadata about the work.
type OSISHeader struct {
	Work []OSISWork `xml:"work"`
}

// OSISWork represents metadata for a specific work.
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

// OSISDiv represents a division in the text (book, chapter, etc.).
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

// OSISChapter represents a chapter marker.
type OSISChapter struct {
	OsisID string `xml:"osisID,attr"`
	SID    string `xml:"sID,attr,omitempty"`
	EID    string `xml:"eID,attr,omitempty"`
}

// OSISVerse represents a verse marker.
type OSISVerse struct {
	OsisID string `xml:"osisID,attr,omitempty"`
	SID    string `xml:"sID,attr,omitempty"`
	EID    string `xml:"eID,attr,omitempty"`
}

// OSISP represents a paragraph.
type OSISP struct {
	Verses  []OSISVerse `xml:"verse"`
	Content string      `xml:",chardata"`
}

// OSISLg represents a line group (poetry).
type OSISLg struct {
	Ls []OSISL `xml:"l"`
}

// OSISL represents a line of poetry.
type OSISL struct {
	Content string `xml:",chardata"`
}

// detectFunc performs custom OSIS format detection.
func osisDetected(reason string) *ipc.DetectResult {
	return &ipc.DetectResult{Detected: true, Format: "OSIS", Reason: reason}
}

func osisNotDetected(reason string) *ipc.DetectResult {
	return &ipc.DetectResult{Detected: false, Reason: reason}
}

func detectFunc(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return osisNotDetected(fmt.Sprintf("cannot stat: %v", err)), nil
	}

	if info.IsDir() {
		return osisNotDetected("path is a directory, not a file"), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return osisNotDetected(fmt.Sprintf("cannot read: %v", err)), nil
	}

	return detectOSISContent(path, data), nil
}

func detectOSISContent(path string, data []byte) *ipc.DetectResult {
	content := string(data)
	if strings.Contains(content, "<osis") && strings.Contains(content, "osisText") {
		return osisDetected("OSIS XML detected")
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".osis" || ext == ".xml" {
		if tryParseOSIS(data) {
			return osisDetected("Valid OSIS XML structure")
		}
	}

	return osisNotDetected("not an OSIS XML file")
}

func tryParseOSIS(data []byte) bool {
	var doc OSISDoc
	return xml.Unmarshal(data, &doc) == nil && doc.OsisText.OsisIDWork != ""
}

// parseFunc parses an OSIS file and returns an IR Corpus.
func parseFunc(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	corpus, err := parseOSISToIR(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OSIS: %w", err)
	}

	return corpus, nil
}

// emitFunc converts an IR Corpus to OSIS format.
func emitFunc(corpus *ir.Corpus, outputDir string) (string, error) {
	osisData, err := emitOSISFromIR(corpus)
	if err != nil {
		return "", fmt.Errorf("failed to emit OSIS: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".osis")
	if err := os.WriteFile(outputPath, osisData, 0600); err != nil {
		return "", fmt.Errorf("failed to write OSIS: %w", err)
	}

	return outputPath, nil
}

// applyCorpusLanguage sets corpus.Language from the osisText attributes,
// preferring xml:lang over lang.
func applyCorpusLanguage(corpus *ir.Corpus, text OSISText) {
	if text.Lang != "" {
		corpus.Language = text.Lang
	}
	if text.XMLLang != "" {
		corpus.Language = text.XMLLang
	}
}

// applyHeaderWork copies metadata from a single OSISWork into corpus when the
// work matches the corpus ID or carries a title.
func applyHeaderWork(corpus *ir.Corpus, work OSISWork, corpusID string) {
	if work.OsisWork != corpusID && work.Title == "" {
		return
	}
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

// applyOSISHeader applies all work entries from the OSIS header to corpus.
func applyOSISHeader(corpus *ir.Corpus, header *OSISHeader, corpusID string) {
	if header == nil {
		return
	}
	for _, work := range header.Work {
		applyHeaderWork(corpus, work, corpusID)
	}
}

// parseOSISToIR converts OSIS XML to IR Corpus.
func parseOSISToIR(data []byte) (*ir.Corpus, error) {
	var doc OSISDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("xml unmarshal failed: %w", err)
	}
	doc.RawXML = string(data)

	corpus := &ir.Corpus{
		ID:           doc.OsisText.OsisIDWork,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "OSIS",
		LossClass:    "L0",
		Attributes:   make(map[string]string),
	}

	applyCorpusLanguage(corpus, doc.OsisText)
	applyOSISHeader(corpus, doc.OsisText.Header, corpus.ID)

	// Store original XML for L0 lossless round-trip.
	corpus.Attributes["_format_raw"] = doc.RawXML

	docOrder := 0
	for _, div := range doc.OsisText.Divs {
		corpus.Documents = append(corpus.Documents, parseOSISDiv(&div, &docOrder)...)
	}

	h := sha256.Sum256(data)
	corpus.SourceHash = hex.EncodeToString(h[:])

	return corpus, nil
}

// parseOSISDiv recursively parses OSIS div elements.
func parseOSISDiv(div *OSISDiv, order *int) []*ir.Document {
	var docs []*ir.Document

	// If this is a book-level div
	if div.Type == "book" || (div.OsisID != "" && isBookID(div.OsisID)) {
		*order++
		doc := &ir.Document{
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

// hashText returns the SHA-256 hex digest of text.
func hashText(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

// newContentBlock allocates a ContentBlock with the next sequence number.
func newContentBlock(seq *int, text string) *ir.ContentBlock {
	*seq++
	return &ir.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", *seq),
		Sequence: *seq,
		Text:     text,
		Hash:     hashText(text),
	}
}

// buildVerseAnchor constructs an Anchor+Span for a single OSIS verse marker.
func buildVerseAnchor(v OSISVerse, seq int) *ir.Anchor {
	osisID := v.OsisID
	if osisID == "" {
		osisID = v.SID
	}
	ref := parseOSISRef(osisID)
	return &ir.Anchor{
		ID:       fmt.Sprintf("a-%d-0", seq),
		Position: 0,
		Spans: []*ir.Span{
			{
				ID:            fmt.Sprintf("s-%s", osisID),
				Type:          "VERSE",
				StartAnchorID: fmt.Sprintf("a-%d-0", seq),
				Ref:           ref,
			},
		},
	}
}

// extractParagraphBlocks converts OSIS <p> elements into content blocks.
func extractParagraphBlocks(ps []OSISP, seq *int) []*ir.ContentBlock {
	var blocks []*ir.ContentBlock
	for _, p := range ps {
		text := strings.TrimSpace(p.Content)
		if text == "" {
			continue
		}
		block := newContentBlock(seq, text)
		block.Anchors = []*ir.Anchor{}
		for _, v := range p.Verses {
			if v.OsisID == "" && v.SID == "" {
				continue
			}
			block.Anchors = append(block.Anchors, buildVerseAnchor(v, *seq))
		}
		blocks = append(blocks, block)
	}
	return blocks
}

// extractPoetryBlocks converts OSIS <lg>/<l> elements into poetry content blocks.
func extractPoetryBlocks(lgs []OSISLg, seq *int) []*ir.ContentBlock {
	var blocks []*ir.ContentBlock
	for _, lg := range lgs {
		for _, l := range lg.Ls {
			text := strings.TrimSpace(l.Content)
			if text == "" {
				continue
			}
			block := newContentBlock(seq, text)
			block.Attributes = map[string]interface{}{"type": "poetry"}
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// extractDirectContent converts inline chardata of a div into a content block (if non-empty).
func extractDirectContent(content string, seq *int) []*ir.ContentBlock {
	text := strings.TrimSpace(content)
	if text == "" {
		return nil
	}
	return []*ir.ContentBlock{newContentBlock(seq, text)}
}

// extractContentBlocks extracts content blocks from an OSIS div.
func extractContentBlocks(div *OSISDiv, seq *int) []*ir.ContentBlock {
	blocks := extractParagraphBlocks(div.Ps, seq)
	blocks = append(blocks, extractPoetryBlocks(div.Lgs, seq)...)
	blocks = append(blocks, extractDirectContent(div.Content, seq)...)
	for _, childDiv := range div.Divs {
		blocks = append(blocks, extractContentBlocks(&childDiv, seq)...)
	}
	return blocks
}

// parseOSISRef parses an OSIS reference like "Gen.1.1" or "Matt.5.3-12".
func parseOSISRef(osisID string) *ir.Ref {
	ref := &ir.Ref{OSISID: osisID}

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

// isBookID checks if an OSIS ID is a book identifier.
func isBookID(osisID string) bool {
	// Common Bible book abbreviations
	books := regexp.MustCompile(`^(Gen|Exod|Lev|Num|Deut|Josh|Judg|Ruth|1Sam|2Sam|1Kgs|2Kgs|1Chr|2Chr|Ezra|Neh|Esth|Job|Ps|Prov|Eccl|Song|Isa|Jer|Lam|Ezek|Dan|Hos|Joel|Amos|Obad|Jonah|Mic|Nah|Hab|Zeph|Hag|Zech|Mal|Matt|Mark|Luke|John|Acts|Rom|1Cor|2Cor|Gal|Eph|Phil|Col|1Thess|2Thess|1Tim|2Tim|Titus|Phlm|Heb|Jas|1Pet|2Pet|1John|2John|3John|Jude|Rev)$`)
	return books.MatchString(osisID)
}

// emitOSISHeader writes the <header> block for the corpus into buf.
func emitOSISHeader(buf *bytes.Buffer, corpus *ir.Corpus) {
	buf.WriteString("    <header>\n")
	buf.WriteString(fmt.Sprintf("      <work osisWork=\"%s\">\n", escapeXML(corpus.ID)))
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
}

// emitOSISVerseMarkers writes inline <verse> tags for all VERSE spans in a block.
func emitOSISVerseMarkers(buf *bytes.Buffer, block *ir.ContentBlock) {
	for _, anchor := range block.Anchors {
		for _, span := range anchor.Spans {
			if span.Type != "VERSE" || span.Ref == nil {
				continue
			}
			osisID := span.Ref.OSISID
			if osisID == "" {
				osisID = fmt.Sprintf("%s.%d.%d", span.Ref.Book, span.Ref.Chapter, span.Ref.Verse)
			}
			buf.WriteString(fmt.Sprintf(`<verse osisID="%s"/>`, escapeXML(osisID)))
		}
	}
}

// isPoetryBlock reports whether a content block carries the "poetry" type attribute.
func isPoetryBlock(block *ir.ContentBlock) bool {
	if block.Attributes == nil {
		return false
	}
	blockType, ok := block.Attributes["type"].(string)
	return ok && blockType == "poetry"
}

// emitOSISContentBlock writes a single content block (poetry or paragraph) into buf.
func emitOSISContentBlock(buf *bytes.Buffer, block *ir.ContentBlock) {
	if isPoetryBlock(block) {
		buf.WriteString("      <lg>\n")
		buf.WriteString(fmt.Sprintf("        <l>%s</l>\n", escapeXML(block.Text)))
		buf.WriteString("      </lg>\n")
		return
	}
	buf.WriteString("      <p>")
	emitOSISVerseMarkers(buf, block)
	buf.WriteString(escapeXML(block.Text))
	buf.WriteString("</p>\n")
}

// emitOSISDocument writes a single <div type="book"> element into buf.
func emitOSISDocument(buf *bytes.Buffer, doc *ir.Document) {
	buf.WriteString(fmt.Sprintf("    <div type=\"book\" osisID=\"%s\">\n", escapeXML(doc.ID)))
	if doc.Title != "" {
		buf.WriteString(fmt.Sprintf("      <title>%s</title>\n", escapeXML(doc.Title)))
	}
	for _, block := range doc.ContentBlocks {
		emitOSISContentBlock(buf, block)
	}
	buf.WriteString("    </div>\n")
}

// emitOSISFromIR converts IR Corpus back to OSIS XML.
func emitOSISFromIR(corpus *ir.Corpus) ([]byte, error) {
	// L0 lossless round-trip: return original XML verbatim when available.
	if rawXML, ok := corpus.Attributes["_format_raw"]; ok && rawXML != "" {
		return []byte(rawXML), nil
	}

	var buf bytes.Buffer
	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buf.WriteString("<osis xmlns=\"http://www.bibletechnologies.net/2003/OSIS/namespace\">\n")
	buf.WriteString(fmt.Sprintf("  <osisText osisIDWork=\"%s\"", escapeXML(corpus.ID)))
	if corpus.Language != "" {
		buf.WriteString(fmt.Sprintf(` xml:lang="%s"`, escapeXML(corpus.Language)))
	}
	buf.WriteString(">\n")

	emitOSISHeader(&buf, corpus)

	for _, doc := range corpus.Documents {
		emitOSISDocument(&buf, doc)
	}

	buf.WriteString("  </osisText>\n")
	buf.WriteString("</osis>\n")

	return buf.Bytes(), nil
}

// escapeXML escapes special characters for XML.
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
