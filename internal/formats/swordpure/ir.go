// Package main contains IR (Intermediate Representation) extraction code for SWORD modules.
package swordpure

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode"
)

// ExtractionStats holds statistics about an IR extraction.
type ExtractionStats struct {
	Documents   int
	Verses      int
	Tokens      int
	Annotations int
}

// IRCorpus is the IR Corpus structure for JSON output.
type IRCorpus struct {
	ID            string            `json:"id"`
	Version       string            `json:"version"`
	ModuleType    string            `json:"module_type"`
	Versification string            `json:"versification"`
	Language      string            `json:"language"`
	Title         string            `json:"title"`
	Documents     []*IRDocument     `json:"documents,omitempty"`
	SourceHash    string            `json:"source_hash,omitempty"`
	LossClass     string            `json:"loss_class,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

// IRDocument represents a book in the IR.
type IRDocument struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Order         int               `json:"order"`
	ContentBlocks []*IRContentBlock `json:"content_blocks,omitempty"`
}

// IRContentBlock represents a verse in the IR.
type IRContentBlock struct {
	ID          string          `json:"id"`
	Sequence    int             `json:"sequence"`
	Text        string          `json:"text"`
	RawMarkup   string          `json:"raw_markup,omitempty"`
	Tokens      []*IRToken      `json:"tokens,omitempty"`
	Annotations []*IRAnnotation `json:"annotations,omitempty"`
	Hash        string          `json:"hash,omitempty"`
}

// IRToken represents a word with linguistic annotations.
type IRToken struct {
	ID         string   `json:"id"`
	Index      int      `json:"index"`
	CharStart  int      `json:"char_start"`
	CharEnd    int      `json:"char_end"`
	Text       string   `json:"text"`
	Type       string   `json:"type"`
	Lemma      string   `json:"lemma,omitempty"`
	Strongs    []string `json:"strongs,omitempty"`
	Morphology string   `json:"morphology,omitempty"`
}

// IRAnnotation represents a footnote, cross-reference, or other annotation.
type IRAnnotation struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	StartPos   int     `json:"start_pos"`
	EndPos     int     `json:"end_pos"`
	Value      string  `json:"value,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

// buildCorpusAttributes collects optional metadata fields from a conf file
// into the attribute map stored on the corpus.
func buildCorpusAttributes(conf *ConfFile) map[string]string {
	attrs := make(map[string]string)
	if conf.About != "" {
		attrs["about"] = conf.About
	}
	if conf.Copyright != "" {
		attrs["copyright"] = conf.Copyright
	}
	if conf.License != "" {
		attrs["license"] = conf.License
	}
	if conf.SourceType != "" {
		attrs["source_type"] = conf.SourceType
	}
	return attrs
}

// shouldSkipBook returns true when a book should be omitted because the
// corresponding testament is not present in the module.
func shouldSkipBook(osisID string, hasOT, hasNT bool) bool {
	if ntBookSet[osisID] {
		return !hasNT
	}
	return !hasOT
}

// extractBookVerses iterates over every chapter/verse of book, appending
// parsed content blocks to doc and updating stats. sequence is advanced for
// each accepted verse.
func extractBookVerses(zt *ZTextModule, book BookData, doc *IRDocument, stats *ExtractionStats, sequence *int) {
	for ch := 1; ch <= len(book.Chapters); ch++ {
		for v := 1; v <= book.Chapters[ch-1]; v++ {
			ref := &Ref{Book: book.OSIS, Chapter: ch, Verse: v}
			rawText, err := zt.GetVerseText(ref)
			if err != nil || rawText == "" {
				continue
			}
			// Skip verses that only contain structural markup (chapter/book markers)
			// with no actual verse text. This handles versification differences where
			// some verses exist in one versification but not another.
			if stripMarkup(rawText) == "" {
				continue
			}

			block := parseVerseContent(ref.String(), rawText, *sequence)
			*sequence++
			doc.ContentBlocks = append(doc.ContentBlocks, block)
			stats.Verses++
			stats.Tokens += len(block.Tokens)
			stats.Annotations += len(block.Annotations)
		}
	}
}

// extractCorpus extracts a full IR corpus from a zText module.
func extractCorpus(zt *ZTextModule, conf *ConfFile) (*IRCorpus, *ExtractionStats, error) {
	vers, err := VersificationFromConf(conf)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get versification: %w", err)
	}

	corpus := &IRCorpus{
		ID:            conf.ModuleName,
		Version:       "1.0.0",
		ModuleType:    "BIBLE",
		Language:      conf.Lang,
		Title:         conf.Description,
		Versification: conf.Versification,
		LossClass:     "L1", // Full text extracted with parsed markup
		Attributes:    buildCorpusAttributes(conf),
	}

	stats := &ExtractionStats{}
	sequence := 0
	hasOT, hasNT := zt.HasOT(), zt.HasNT()

	for bookIdx, book := range vers.Books {
		if shouldSkipBook(book.OSIS, hasOT, hasNT) {
			continue
		}

		doc := &IRDocument{
			ID:    book.OSIS,
			Title: book.Name,
			Order: bookIdx + 1,
		}

		extractBookVerses(zt, book, doc, stats, &sequence)

		if len(doc.ContentBlocks) > 0 {
			corpus.Documents = append(corpus.Documents, doc)
			stats.Documents++
		}
	}

	return corpus, stats, nil
}

// parseVerseContent parses verse content, keeping raw markup AND extracting structured data.
// This achieves the user's goal of "both raw and parsed" for sanity/corruption checking.
func parseVerseContent(id, rawText string, sequence int) *IRContentBlock {
	block := &IRContentBlock{
		ID:        id,
		Sequence:  sequence,
		RawMarkup: rawText, // Keep original OSIS/ThML markup for L0 fidelity
	}

	// Strip markup to get plain text
	plainText := stripMarkup(rawText)
	block.Text = plainText

	// Compute hash of plain text for integrity checking
	block.Hash = computeHash(plainText)

	// Parse tokens with Strong's numbers and morphology
	block.Tokens = parseTokensFromMarkup(rawText)

	// Parse annotations (footnotes, cross-references)
	block.Annotations = parseAnnotationsFromMarkup(rawText)

	return block
}

// stripMarkup removes OSIS/ThML markup, returning plain text.
func stripMarkup(text string) string {
	var result strings.Builder
	inTag := false

	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteByte(c)
		}
	}

	return strings.TrimSpace(result.String())
}

// computeHash computes SHA-256 hash of text.
func computeHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

// isWTagAt returns true if text[i:] starts a <w ...> or <w> opening tag.
func isWTagAt(text string, i int) bool {
	return i+2 < len(text) && text[i] == '<' && text[i+1] == 'w' && (text[i+2] == ' ' || text[i+2] == '>')
}

// findTagClose scans forward from start for the '>' character that closes an
// XML/HTML opening tag. Returns the index of '>' or -1 if not found.
func findTagClose(text string, start int) int {
	for j := start; j < len(text); j++ {
		if text[j] == '>' {
			return j
		}
	}
	return -1
}

// findCloseW scans forward from start for the literal string "</w>".
// Returns the index of '<' in "</w>" or -1 if not found.
func findCloseW(text string, start int) int {
	for j := start; j < len(text); j++ {
		if j+3 < len(text) && text[j:j+4] == "</w>" {
			return j
		}
	}
	return -1
}

// buildWToken constructs a single IRToken from a parsed <w> word element.
func buildWToken(plainWord, lemma, morph string, idx, charPos int) *IRToken {
	token := &IRToken{
		ID:        fmt.Sprintf("t%d", idx),
		Index:     idx,
		CharStart: charPos,
		CharEnd:   charPos + len(plainWord),
		Text:      plainWord,
		Type:      "word",
	}
	if lemma != "" {
		token.Strongs = parseStrongs(lemma)
		token.Lemma = lemma
	}
	if morph != "" {
		token.Morphology = morph
	}
	return token
}

// parseTokensFromMarkup extracts tokens with Strong's numbers and morphology from OSIS/ThML.
// Parses: <w lemma="strong:H1234" morph="...">word</w>
func parseTokensFromMarkup(text string) []*IRToken {
	var tokens []*IRToken
	tokenIdx := 0
	charPos := 0

	for i := 0; i < len(text); {
		if !isWTagAt(text, i) {
			i++
			continue
		}

		tagClose := findTagClose(text, i)
		if tagClose < 0 {
			break
		}

		lemma := extractAttr(text[i:tagClose+1], "lemma")
		morph := extractAttr(text[i:tagClose+1], "morph")

		wordEnd := findCloseW(text, tagClose+1)
		if wordEnd < 0 {
			i = tagClose + 1
			continue
		}

		plainWord := stripMarkup(text[tagClose+1 : wordEnd])
		if plainWord != "" {
			tokens = append(tokens, buildWToken(plainWord, lemma, morph, tokenIdx, charPos))
			tokenIdx++
			charPos += len(plainWord) + 1 // +1 for space
		}
		i = wordEnd + 4 // skip past </w>
	}

	// If no <w> tags found, tokenize plain text
	if len(tokens) == 0 {
		return tokenizePlainText(stripMarkup(text))
	}

	return tokens
}

// extractAttr extracts an attribute value from an XML tag.
func extractAttr(tag, attr string) string {
	// Look for attr="value" or attr='value' with proper word boundary
	// Need to ensure we don't match "osisID" when looking for "sID"
	patterns := []string{" " + attr + `="`, " " + attr + `='`}
	for _, pattern := range patterns {
		idx := strings.Index(tag, pattern)
		if idx >= 0 {
			start := idx + len(pattern)
			quote := tag[start-1]
			end := start
			for end < len(tag) && tag[end] != quote {
				end++
			}
			if end < len(tag) {
				return tag[start:end]
			}
		}
	}
	return ""
}

// parseStrongs extracts Strong's numbers from a lemma attribute.
// Handles formats like: "strong:H1234", "strong:G2532", "H1234 H5678"
func parseStrongs(lemma string) []string {
	var strongs []string
	for _, part := range strings.Fields(lemma) {
		part = strings.TrimPrefix(part, "strong:")
		if isStrongsNumber(part) {
			strongs = append(strongs, part)
		}
	}
	return strongs
}

// isStrongsNumber checks if a string is a valid Strong's number (H/G followed by digits)
func isStrongsNumber(s string) bool {
	if len(s) < 2 || (s[0] != 'H' && s[0] != 'G') {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// tokenizePlainText tokenizes plain text into words.
// Uses unicode.IsLetter and unicode.IsDigit for proper multi-byte character handling.
func tokenizePlainText(text string) []*IRToken {
	t := &tokenizer{text: text}
	return t.tokenize()
}

// tokenizer holds state for text tokenization.
type tokenizer struct {
	text       string
	tokens     []*IRToken
	tokenStart int
	tokenText  strings.Builder
	tokenIdx   int
	inWord     bool
}

// tokenize processes the text and returns tokens.
func (t *tokenizer) tokenize() []*IRToken {
	for i, r := range t.text {
		t.processRune(i, r)
	}
	t.finalizeLastWord()
	return t.tokens
}

// processRune handles a single rune during tokenization.
func (t *tokenizer) processRune(i int, r rune) {
	isWordChar := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '\''
	if isWordChar {
		if !t.inWord {
			t.tokenStart = i
			t.inWord = true
		}
		t.tokenText.WriteRune(r)
	} else if t.inWord {
		t.emitToken(i)
	}
}

// emitToken creates a token and resets state.
func (t *tokenizer) emitToken(endPos int) {
	t.tokens = append(t.tokens, &IRToken{
		ID:        fmt.Sprintf("t%d", t.tokenIdx),
		Index:     t.tokenIdx,
		CharStart: t.tokenStart,
		CharEnd:   endPos,
		Text:      t.tokenText.String(),
		Type:      "word",
	})
	t.tokenIdx++
	t.tokenText.Reset()
	t.inWord = false
}

// finalizeLastWord handles any remaining word at end of text.
func (t *tokenizer) finalizeLastWord() {
	if t.inWord {
		t.emitToken(len(t.text))
	}
}

// parseAnnotationsFromMarkup extracts annotations (footnotes, cross-refs) from OSIS/ThML.
func parseAnnotationsFromMarkup(text string) []*IRAnnotation {
	var annotations []*IRAnnotation
	annotIdx := 0

	// Parse <note> tags (footnotes)
	annotations = append(annotations, extractNotes(text, &annotIdx)...)

	// Parse <reference> tags (cross-references)
	annotations = append(annotations, extractReferences(text, &annotIdx)...)

	return annotations
}

// extractNotes extracts <note>...</note> footnotes.
func extractNotes(text string, idx *int) []*IRAnnotation {
	var annotations []*IRAnnotation

	i := 0
	for i < len(text) {
		// Look for <note
		noteStart := strings.Index(text[i:], "<note")
		if noteStart < 0 {
			break
		}
		noteStart += i

		// Find end of opening tag
		tagEnd := noteStart
		for tagEnd < len(text) && text[tagEnd] != '>' {
			tagEnd++
		}
		if tagEnd >= len(text) {
			break
		}

		// Find </note>
		closeTag := strings.Index(text[tagEnd:], "</note>")
		if closeTag < 0 {
			i = tagEnd + 1
			continue
		}
		closeTag += tagEnd

		// Extract note content
		noteContent := text[tagEnd+1 : closeTag]
		plainContent := stripMarkup(noteContent)

		annotations = append(annotations, &IRAnnotation{
			ID:       fmt.Sprintf("n%d", *idx),
			Type:     "FOOTNOTE",
			StartPos: noteStart,
			EndPos:   closeTag + 7,
			Value:    plainContent,
		})
		*idx++

		i = closeTag + 7
	}

	return annotations
}

// extractReferences extracts <reference>...</reference> cross-references.
func extractReferences(text string, idx *int) []*IRAnnotation {
	var annotations []*IRAnnotation
	i := 0
	for i < len(text) {
		refStart, tagEnd, found := findReferenceTag(text, i)
		if !found {
			break
		}

		closeTag := strings.Index(text[tagEnd:], "</reference>")
		if closeTag < 0 {
			i = tagEnd + 1
			continue
		}
		closeTag += tagEnd

		annotation := buildReferenceAnnotation(text, refStart, tagEnd, closeTag, idx)
		annotations = append(annotations, annotation)
		i = closeTag + 12
	}
	return annotations
}

// findReferenceTag finds the next <reference> tag position.
func findReferenceTag(text string, start int) (refStart, tagEnd int, found bool) {
	refStart = strings.Index(text[start:], "<reference")
	if refStart < 0 {
		return 0, 0, false
	}
	refStart += start

	tagEnd = refStart
	for tagEnd < len(text) && text[tagEnd] != '>' {
		tagEnd++
	}
	if tagEnd >= len(text) {
		return 0, 0, false
	}
	return refStart, tagEnd, true
}

// buildReferenceAnnotation creates an annotation from reference tag positions.
func buildReferenceAnnotation(text string, refStart, tagEnd, closeTag int, idx *int) *IRAnnotation {
	tag := text[refStart : tagEnd+1]
	osisRef := extractAttr(tag, "osisRef")

	value := osisRef
	if value == "" {
		refContent := text[tagEnd+1 : closeTag]
		value = stripMarkup(refContent)
	}

	annotation := &IRAnnotation{
		ID:       fmt.Sprintf("r%d", *idx),
		Type:     "CROSS_REF",
		StartPos: refStart,
		EndPos:   closeTag + 12,
		Value:    value,
	}
	*idx++
	return annotation
}

// writeCorpusJSON writes the corpus to a JSON file.
func writeCorpusJSON(corpus *IRCorpus, path string) error {
	data, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// generateConfFromIR generates a SWORD .conf file from IR corpus.
func generateConfFromIR(corpus *IRCorpus) string {
	var buf strings.Builder

	buf.WriteString(fmt.Sprintf("[%s]\n", corpus.ID))
	buf.WriteString(fmt.Sprintf("Description=%s\n", corpus.Title))
	buf.WriteString(fmt.Sprintf("Lang=%s\n", corpus.Language))
	buf.WriteString("ModDrv=zText\n")
	buf.WriteString("Encoding=UTF-8\n")
	buf.WriteString(fmt.Sprintf("DataPath=./modules/texts/ztext/%s/\n", strings.ToLower(corpus.ID)))

	if corpus.Versification != "" {
		buf.WriteString(fmt.Sprintf("Versification=%s\n", corpus.Versification))
	}

	// Add attributes
	if about, ok := corpus.Attributes["about"]; ok {
		buf.WriteString(fmt.Sprintf("About=%s\n", about))
	}
	if copyright, ok := corpus.Attributes["copyright"]; ok {
		buf.WriteString(fmt.Sprintf("Copyright=%s\n", copyright))
	}
	if license, ok := corpus.Attributes["license"]; ok {
		buf.WriteString(fmt.Sprintf("DistributionLicense=%s\n", license))
	}

	return buf.String()
}

// ChapterMarker represents a chapter boundary marker found in OSIS/ThML markup.
type ChapterMarker struct {
	OsisID  string // e.g., "1Pet.5"
	SID     string // Start ID (milestone start)
	EID     string // End ID (milestone end)
	IsStart bool   // true if this is a chapter start marker
	IsEnd   bool   // true if this is a chapter end marker
}

// ParseChapterMarkers extracts chapter boundary markers from raw OSIS/ThML markup.
// This is useful for detecting versification differences and chapter boundaries.
func chapterTagEnd(rawMarkup string, start int) int {
	for i := start; i < len(rawMarkup); i++ {
		if rawMarkup[i] == '>' {
			return i
		}
	}
	return -1
}

func buildChapterMarker(tagContent string) (ChapterMarker, bool) {
	m := ChapterMarker{
		OsisID: extractAttr(tagContent, "osisID"),
		SID:    extractAttr(tagContent, "sID"),
		EID:    extractAttr(tagContent, "eID"),
	}
	m.IsStart = m.SID != ""
	m.IsEnd = m.EID != ""
	return m, m.OsisID != "" || m.SID != "" || m.EID != ""
}

func ParseChapterMarkers(rawMarkup string) []ChapterMarker {
	var markers []ChapterMarker
	i := 0
	for i < len(rawMarkup) {
		rel := strings.Index(rawMarkup[i:], "<chapter")
		if rel < 0 {
			break
		}
		start := i + rel
		end := chapterTagEnd(rawMarkup, start)
		if end < 0 {
			break
		}
		if m, ok := buildChapterMarker(rawMarkup[start : end+1]); ok {
			markers = append(markers, m)
		}
		i = end + 1
	}
	return markers
}

// IsChapterBoundaryOnly returns true if the raw markup contains only chapter
// boundary markers and no actual verse text. This indicates a versification
// difference where this verse doesn't exist in the source Bible.
func IsChapterBoundaryOnly(rawMarkup string) bool {
	markers := ParseChapterMarkers(rawMarkup)
	if len(markers) == 0 {
		return false
	}

	// Check if there's any text outside of tags
	plainText := stripMarkup(rawMarkup)
	return strings.TrimSpace(plainText) == ""
}

// DetectVersificationDifference analyzes raw markup to determine if a verse
// doesn't exist in the source due to versification differences.
// Returns a description of the difference if detected, empty string otherwise.
func DetectVersificationDifference(rawMarkup string) string {
	markers := ParseChapterMarkers(rawMarkup)
	if len(markers) == 0 {
		return ""
	}

	plainText := stripMarkup(rawMarkup)
	if strings.TrimSpace(plainText) != "" {
		return "" // Has actual text, not just a boundary
	}

	// Analyze the markers to explain the difference
	for _, m := range markers {
		if m.IsStart && m.OsisID != "" {
			return fmt.Sprintf("versification: chapter %s begins here in source", m.OsisID)
		}
		if m.IsEnd && m.OsisID != "" {
			return fmt.Sprintf("versification: chapter %s ends here in source", m.OsisID)
		}
	}

	return "versification: structural marker only"
}
