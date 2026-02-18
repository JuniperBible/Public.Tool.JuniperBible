// Package rtf provides pure Go RTF parsing and conversion.
// This replaces external unrtf dependency with native Go implementation.
package rtf

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/FocuswithJustin/JuniperBible/core/encoding"
)

// Document represents a parsed RTF document.
type Document struct {
	root     *Group
	metadata DocumentMetadata
}

// DocumentMetadata contains RTF document metadata.
type DocumentMetadata struct {
	Title   string
	Author  string
	Subject string
	Created string
}

// Group represents an RTF group (content within braces).
type Group struct {
	children []interface{} // can be *Group, ControlWord, or string
}

// ControlWord represents an RTF control word.
type ControlWord struct {
	word  string
	param int
	has   bool
}

// Parse parses RTF data and returns a Document.
func Parse(data []byte) (*Document, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty RTF data")
	}

	// Check for RTF header
	if !bytes.HasPrefix(data, []byte("{\\rtf")) {
		return nil, fmt.Errorf("not a valid RTF document: missing \\rtf header")
	}

	parser := &rtfParser{data: data, pos: 0}
	root, err := parser.parseGroup()
	if err != nil {
		return nil, err
	}

	doc := &Document{root: root}
	doc.extractMetadata()

	return doc, nil
}

type rtfParser struct {
	data []byte
	pos  int
}

func (p *rtfParser) parseGroup() (*Group, error) {
	if p.pos >= len(p.data) || p.data[p.pos] != '{' {
		return nil, fmt.Errorf("expected '{' at position %d", p.pos)
	}
	p.pos++

	group := &Group{}
	for p.pos < len(p.data) {
		done, err := p.parseGroupStep(group)
		if err != nil {
			return nil, err
		}
		if done {
			return group, nil
		}
	}
	return nil, fmt.Errorf("unclosed group")
}

func (p *rtfParser) parseGroupStep(group *Group) (done bool, err error) {
	ch := p.data[p.pos]
	switch {
	case ch == '}':
		p.pos++
		return true, nil
	case ch == '\r' || ch == '\n':
		p.pos++
		return false, nil
	case ch == '{':
		return p.parseNestedGroup(group)
	case ch == '\\':
		return p.parseControlWordInGroup(group)
	default:
		return p.parseTextInGroup(group)
	}
}

func (p *rtfParser) parseNestedGroup(group *Group) (bool, error) {
	nested, err := p.parseGroup()
	if err != nil {
		return false, err
	}
	group.children = append(group.children, nested)
	return false, nil
}

func (p *rtfParser) parseControlWordInGroup(group *Group) (bool, error) {
	cw, err := p.parseControlWord()
	if err != nil {
		return false, err
	}
	group.children = append(group.children, cw)
	return false, nil
}

func (p *rtfParser) parseTextInGroup(group *Group) (bool, error) {
	text := p.parseText()
	if text != "" {
		group.children = append(group.children, text)
	}
	return false, nil
}

func (p *rtfParser) parseControlWord() (ControlWord, error) {
	p.pos++ // consume '\'
	if p.pos >= len(p.data) {
		return ControlWord{}, fmt.Errorf("unexpected end after backslash")
	}

	ch := p.data[p.pos]

	if ch == '{' || ch == '}' || ch == '\\' {
		return p.parseSpecialChar(), nil
	}

	if isLetter(ch) {
		return p.parseLetterControlWord(), nil
	}

	// Control symbol (single non-letter character after \)
	p.pos++
	return ControlWord{word: string(ch)}, nil
}

// parseSpecialChar handles the escaped special characters \{, \}, and \\.
// The caller must have already verified that p.data[p.pos] is one of those.
func (p *rtfParser) parseSpecialChar() ControlWord {
	ch := p.data[p.pos]
	p.pos++
	return ControlWord{word: string(ch)}
}

// parseLetterControlWord reads a letter-based RTF control word together with
// its optional numeric parameter and the optional trailing delimiter space.
// The caller must have already verified that p.data[p.pos] is a letter.
func (p *rtfParser) parseLetterControlWord() ControlWord {
	start := p.pos
	for p.pos < len(p.data) && isLetter(p.data[p.pos]) {
		p.pos++
	}
	word := string(p.data[start:p.pos])

	param, hasParam := p.parseNumericParam()

	if p.pos < len(p.data) && p.data[p.pos] == ' ' {
		p.pos++ // skip delimiter space
	}

	return ControlWord{word: word, param: param, has: hasParam}
}

// parseNumericParam reads an optional signed integer that follows a control
// word and returns its value together with a flag indicating whether a
// parameter was present. The parser position is advanced past the digits.
func (p *rtfParser) parseNumericParam() (param int, hasParam bool) {
	if p.pos >= len(p.data) {
		return 0, false
	}
	if p.data[p.pos] != '-' && !isDigit(p.data[p.pos]) {
		return 0, false
	}

	numStart := p.pos
	if p.data[p.pos] == '-' {
		p.pos++
	}
	for p.pos < len(p.data) && isDigit(p.data[p.pos]) {
		p.pos++
	}
	param, _ = strconv.Atoi(string(p.data[numStart:p.pos]))
	return param, true
}

func (p *rtfParser) parseText() string {
	var buf bytes.Buffer
	for p.pos < len(p.data) {
		ch := p.data[p.pos]
		if ch == '{' || ch == '}' || ch == '\\' {
			break
		}
		if ch != '\r' && ch != '\n' {
			buf.WriteByte(ch)
		}
		p.pos++
	}
	return buf.String()
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// extractMetadata extracts document metadata from info group.
func (doc *Document) extractMetadata() {
	if doc.root == nil {
		return
	}

	for _, child := range doc.root.children {
		if group, ok := child.(*Group); ok {
			doc.findInfoGroup(group)
		}
	}
}

func (doc *Document) findInfoGroup(group *Group) {
	for _, child := range group.children {
		if cw, ok := child.(ControlWord); ok {
			if cw.word == "info" {
				doc.parseInfoGroup(group)
				return
			}
		}
		if nested, ok := child.(*Group); ok {
			// Check if this is an info group
			for _, c := range nested.children {
				if cw, ok := c.(ControlWord); ok && cw.word == "info" {
					doc.parseInfoGroup(nested)
					return
				}
			}
			doc.findInfoGroup(nested)
		}
	}
}

var infoFieldNames = map[string]bool{
	"title":   true,
	"author":  true,
	"subject": true,
}

func extractInfoField(nested *Group) (fieldName, fieldValue string) {
	var value strings.Builder
	for _, c := range nested.children {
		if cw, ok := c.(ControlWord); ok && infoFieldNames[cw.word] {
			fieldName = cw.word
		}
		if text, ok := c.(string); ok {
			value.WriteString(text)
		}
	}
	return fieldName, strings.TrimSpace(value.String())
}

func (doc *Document) parseInfoGroup(group *Group) {
	for _, child := range group.children {
		nested, ok := child.(*Group)
		if !ok {
			continue
		}
		name, value := extractInfoField(nested)
		switch name {
		case "title":
			doc.metadata.Title = value
		case "author":
			doc.metadata.Author = value
		case "subject":
			doc.metadata.Subject = value
		}
	}
}

// specialGroupWords is the set of RTF control words that introduce groups whose
// content must be silently skipped during extraction.
var specialGroupWords = map[string]bool{
	"fonttbl":    true,
	"colortbl":   true,
	"stylesheet": true,
	"info":       true,
	"pict":       true,
	"object":     true,
}

// isSpecialGroup returns true when the group begins with a control word that
// marks it as metadata / non-content (font tables, colour tables, etc.).
func isSpecialGroup(group *Group) bool {
	for _, c := range group.children {
		if cw, ok := c.(ControlWord); ok && specialGroupWords[cw.word] {
			return true
		}
	}
	return false
}

// groupFormatting scans the immediate children of a group and returns whether
// it carries explicit bold or italic formatting control words.
func groupFormatting(group *Group) (bold, italic bool) {
	for _, c := range group.children {
		cw, ok := c.(ControlWord)
		if !ok {
			continue
		}
		if isFormattingEnabled(cw) {
			bold, italic = applyFormatting(cw.word, bold, italic)
		}
	}
	return
}

// isFormattingEnabled returns true if the control word enables formatting.
func isFormattingEnabled(cw ControlWord) bool {
	return cw.param != 0 || !cw.has
}

// applyFormatting applies bold/italic based on control word.
func applyFormatting(word string, bold, italic bool) (bool, bool) {
	if word == "b" {
		return true, italic
	}
	if word == "i" {
		return bold, true
	}
	return bold, italic
}

// ---------------------------------------------------------------------------
// Plain-text extraction
// ---------------------------------------------------------------------------

// textControlStrings maps simple RTF control words to their plain-text
// equivalents. Words not in this map (e.g. "u") are handled separately.
var textControlStrings = map[string]string{
	"par":  "\n",
	"line": "\n",
	"tab":  "\t",
	"{":    "{",
	"}":    "}",
	"\\":   "\\",
}

// ToText converts the document to plain text.
func (doc *Document) ToText() string {
	if doc.root == nil {
		return ""
	}
	var buf strings.Builder
	doc.extractText(doc.root, &buf)
	return strings.TrimSpace(buf.String())
}

// ToTextBytes converts the document to plain text bytes.
func (doc *Document) ToTextBytes() []byte {
	return []byte(doc.ToText())
}

func (doc *Document) extractText(group *Group, buf *strings.Builder) {
	for _, child := range group.children {
		switch v := child.(type) {
		case string:
			buf.WriteString(v)
		case ControlWord:
			doc.handleTextControlWord(v, buf)
		case *Group:
			if !isSpecialGroup(v) {
				doc.extractText(v, buf)
			}
		}
	}
}

// handleTextControlWord writes the plain-text representation of a single
// RTF control word into buf.
func (doc *Document) handleTextControlWord(cw ControlWord, buf *strings.Builder) {
	if s, ok := textControlStrings[cw.word]; ok {
		buf.WriteString(s)
		return
	}
	if cw.word == "u" && cw.has && cw.param > 0 {
		buf.WriteRune(rune(cw.param))
	}
}

// ---------------------------------------------------------------------------
// HTML extraction
// ---------------------------------------------------------------------------

// htmlControlHandlers maps RTF control words to functions that emit HTML.
// The handler receives the full state so it can mutate inParagraph, localBold,
// and localItalic via pointer.
type htmlState struct {
	buf         *strings.Builder
	inParagraph bool
	localBold   bool
	localItalic bool
}

// htmlControlHandler is the function signature for per-word HTML handlers.
type htmlControlHandler func(cw ControlWord, s *htmlState)

func htmlHandleBold(cw ControlWord, s *htmlState) {
	if !s.localBold && (cw.param != 0 || !cw.has) {
		s.buf.WriteString("<b>")
		s.localBold = true
	} else if s.localBold && cw.param == 0 {
		s.buf.WriteString("</b>")
		s.localBold = false
	}
}

func htmlHandleItalic(cw ControlWord, s *htmlState) {
	if !s.localItalic && (cw.param != 0 || !cw.has) {
		s.buf.WriteString("<i>")
		s.localItalic = true
	} else if s.localItalic && cw.param == 0 {
		s.buf.WriteString("</i>")
		s.localItalic = false
	}
}

func htmlHandlePar(cw ControlWord, s *htmlState) {
	if s.inParagraph {
		s.buf.WriteString("</p>\n<p>")
	} else {
		s.buf.WriteString("<p>")
		s.inParagraph = true
	}
}

func htmlHandleLine(_ ControlWord, s *htmlState) { s.buf.WriteString("<br/>") }
func htmlHandleTab(_ ControlWord, s *htmlState)  { s.buf.WriteString("&nbsp;&nbsp;&nbsp;&nbsp;") }

func htmlHandleUnicode(cw ControlWord, s *htmlState) {
	if cw.has && cw.param > 0 {
		s.buf.WriteRune(rune(cw.param))
	}
}

// htmlControlDispatch is the lookup table replacing the switch in extractHTML.
var htmlControlDispatch = map[string]htmlControlHandler{
	"b":    htmlHandleBold,
	"i":    htmlHandleItalic,
	"par":  htmlHandlePar,
	"line": htmlHandleLine,
	"tab":  htmlHandleTab,
	"u":    htmlHandleUnicode,
}

// ToHTML converts the document to HTML.
func (doc *Document) ToHTML() string {
	if doc.root == nil {
		return ""
	}
	var buf strings.Builder
	buf.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	if doc.metadata.Title != "" {
		buf.WriteString("<title>")
		buf.WriteString(encoding.EscapeHTML(doc.metadata.Title))
		buf.WriteString("</title>\n")
	}
	buf.WriteString("</head>\n<body>\n")

	doc.extractHTML(doc.root, &buf, false, false)

	buf.WriteString("\n</body>\n</html>")
	return buf.String()
}

// ToHTMLBytes converts the document to HTML bytes.
func (doc *Document) ToHTMLBytes() []byte {
	return []byte(doc.ToHTML())
}

func (doc *Document) extractHTML(group *Group, buf *strings.Builder, inBold, inItalic bool) {
	s := &htmlState{buf: buf, inParagraph: false, localBold: inBold, localItalic: inItalic}

	for _, child := range group.children {
		switch v := child.(type) {
		case string:
			doc.handleHTMLText(v, s)
		case ControlWord:
			if handler, ok := htmlControlDispatch[v.word]; ok {
				handler(v, s)
			}
		case *Group:
			doc.handleHTMLGroup(v, s)
		}
	}

	if s.inParagraph {
		buf.WriteString("</p>")
	}
}

// handleHTMLText opens a paragraph when needed, then writes escaped text.
func (doc *Document) handleHTMLText(v string, s *htmlState) {
	if !s.inParagraph && strings.TrimSpace(v) != "" {
		s.buf.WriteString("<p>")
		s.inParagraph = true
	}
	s.buf.WriteString(encoding.EscapeHTML(v))
}

func resolveGroupFormatting(fmtBold, fmtItalic, localBold, localItalic bool) (needsBold, needsItalic, effectiveBold, effectiveItalic bool) {
	effectiveBold = fmtBold || localBold
	effectiveItalic = fmtItalic || localItalic
	needsBold = effectiveBold && !localBold
	needsItalic = effectiveItalic && !localItalic
	return
}

func writeOpenGroupTags(needsBold, needsItalic bool, buf *strings.Builder) {
	if needsBold {
		buf.WriteString("<b>")
	}
	if needsItalic {
		buf.WriteString("<i>")
	}
}

func writeCloseGroupTags(needsBold, needsItalic bool, buf *strings.Builder) {
	if needsItalic {
		buf.WriteString("</i>")
	}
	if needsBold {
		buf.WriteString("</b>")
	}
}

func (doc *Document) handleHTMLGroup(v *Group, s *htmlState) {
	if isSpecialGroup(v) {
		return
	}
	fmtBold, fmtItalic := groupFormatting(v)
	needsBold, needsItalic, effectiveBold, effectiveItalic := resolveGroupFormatting(fmtBold, fmtItalic, s.localBold, s.localItalic)
	writeOpenGroupTags(needsBold, needsItalic, s.buf)
	doc.extractHTML(v, s.buf, effectiveBold, effectiveItalic)
	writeCloseGroupTags(needsBold, needsItalic, s.buf)
}

// ---------------------------------------------------------------------------
// LaTeX extraction
// ---------------------------------------------------------------------------

// latexControlStrings maps simple RTF control words to their LaTeX equivalents.
var latexControlStrings = map[string]string{
	"par":  "\n\n",
	"line": "\\\\\n",
	"tab":  "\\quad ",
}

// ToLaTeX converts the document to LaTeX.
func (doc *Document) ToLaTeX() string {
	if doc.root == nil {
		return ""
	}
	var buf strings.Builder
	buf.WriteString("\\documentclass{article}\n")
	buf.WriteString("\\usepackage[utf8]{inputenc}\n")
	if doc.metadata.Title != "" {
		buf.WriteString("\\title{")
		buf.WriteString(encoding.EscapeLaTeX(doc.metadata.Title))
		buf.WriteString("}\n")
	}
	if doc.metadata.Author != "" {
		buf.WriteString("\\author{")
		buf.WriteString(encoding.EscapeLaTeX(doc.metadata.Author))
		buf.WriteString("}\n")
	}
	buf.WriteString("\\begin{document}\n")
	if doc.metadata.Title != "" {
		buf.WriteString("\\maketitle\n")
	}

	doc.extractLaTeX(doc.root, &buf, false, false)

	buf.WriteString("\n\\end{document}\n")
	return buf.String()
}

// ToLaTeXBytes converts the document to LaTeX bytes.
func (doc *Document) ToLaTeXBytes() []byte {
	return []byte(doc.ToLaTeX())
}

func (doc *Document) extractLaTeX(group *Group, buf *strings.Builder, inBold, inItalic bool) {
	for _, child := range group.children {
		switch v := child.(type) {
		case string:
			buf.WriteString(encoding.EscapeLaTeX(v))
		case ControlWord:
			doc.handleLaTeXControlWord(v, buf)
		case *Group:
			doc.handleLaTeXGroup(v, buf, inBold, inItalic)
		}
	}
}

// handleLaTeXControlWord writes the LaTeX representation of a single RTF
// control word into buf.
func (doc *Document) handleLaTeXControlWord(cw ControlWord, buf *strings.Builder) {
	if s, ok := latexControlStrings[cw.word]; ok {
		buf.WriteString(s)
		return
	}
	if cw.word == "u" && cw.has && cw.param > 0 {
		doc.writeLaTeXUnicode(cw.param, buf)
	}
}

// writeLaTeXUnicode emits a Unicode code-point as printable ASCII or a
// \symbol{N} fallback.
func (doc *Document) writeLaTeXUnicode(param int, buf *strings.Builder) {
	r := rune(param)
	if r < 128 && unicode.IsPrint(r) {
		buf.WriteRune(r)
	} else {
		buf.WriteString(fmt.Sprintf("\\symbol{%d}", param))
	}
}

// handleLaTeXGroup processes a nested RTF group, wrapping with \textbf{} /
// \textit{} as needed.
func (doc *Document) handleLaTeXGroup(v *Group, buf *strings.Builder, inBold, inItalic bool) {
	if isSpecialGroup(v) {
		return
	}

	groupBold, groupItalic := groupFormatting(v)

	if groupBold {
		buf.WriteString("\\textbf{")
	}
	if groupItalic {
		buf.WriteString("\\textit{")
	}

	doc.extractLaTeX(v, buf, groupBold, groupItalic)

	if groupItalic {
		buf.WriteString("}")
	}
	if groupBold {
		buf.WriteString("}")
	}
}

// Metadata returns the document metadata.
func (doc *Document) Metadata() DocumentMetadata {
	return doc.metadata
}
