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
		Attributes:    make(map[string]string),
	}

	// Store raw conf for potential L0 round-trip
	if conf.About != "" {
		corpus.Attributes["about"] = conf.About
	}
	if conf.Copyright != "" {
		corpus.Attributes["copyright"] = conf.Copyright
	}
	if conf.License != "" {
		corpus.Attributes["license"] = conf.License
	}
	if conf.SourceType != "" {
		corpus.Attributes["source_type"] = conf.SourceType
	}

	stats := &ExtractionStats{}
	sequence := 0

	// Determine which testaments are available
	hasOT := zt.HasOT()
	hasNT := zt.HasNT()

	// Extract all verses using versification
	for bookIdx, book := range vers.Books {
		// Skip books from missing testaments
		isNT := bookIdx >= 39
		if isNT && !hasNT {
			continue
		}
		if !isNT && !hasOT {
			continue
		}

		doc := &IRDocument{
			ID:    book.OSIS,
			Title: book.Name,
			Order: bookIdx + 1,
		}

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
				plainText := stripMarkup(rawText)
				if plainText == "" {
					continue
				}

				// Parse verse content (keeps raw + extracts structured data)
				block := parseVerseContent(ref.String(), rawText, sequence)
				sequence++

				doc.ContentBlocks = append(doc.ContentBlocks, block)
				stats.Verses++
				stats.Tokens += len(block.Tokens)
				stats.Annotations += len(block.Annotations)
			}
		}

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

// parseTokensFromMarkup extracts tokens with Strong's numbers and morphology from OSIS/ThML.
// Parses: <w lemma="strong:H1234" morph="...">word</w>
func parseTokensFromMarkup(text string) []*IRToken {
	var tokens []*IRToken
	tokenIdx := 0
	charPos := 0

	// Simple state machine to parse <w> tags
	i := 0
	for i < len(text) {
		// Look for <w ... > tags
		if i+2 < len(text) && text[i] == '<' && text[i+1] == 'w' && (text[i+2] == ' ' || text[i+2] == '>') {
			// Find the end of the opening tag
			tagEnd := i
			for tagEnd < len(text) && text[tagEnd] != '>' {
				tagEnd++
			}
			if tagEnd >= len(text) {
				break
			}

			// Extract attributes from the tag
			tagContent := text[i : tagEnd+1]
			lemma := extractAttr(tagContent, "lemma")
			morph := extractAttr(tagContent, "morph")

			// Find the word text (between > and </w>)
			wordStart := tagEnd + 1
			wordEnd := wordStart
			for wordEnd < len(text) {
				if wordEnd+3 < len(text) && text[wordEnd:wordEnd+4] == "</w>" {
					break
				}
				wordEnd++
			}

			if wordEnd < len(text) {
				wordText := text[wordStart:wordEnd]
				plainWord := stripMarkup(wordText)

				if plainWord != "" {
					token := &IRToken{
						ID:        fmt.Sprintf("t%d", tokenIdx),
						Index:     tokenIdx,
						CharStart: charPos,
						CharEnd:   charPos + len(plainWord),
						Text:      plainWord,
						Type:      "word",
					}

					// Parse Strong's numbers from lemma
					if lemma != "" {
						token.Strongs = parseStrongs(lemma)
						token.Lemma = lemma
					}

					// Store morphology
					if morph != "" {
						token.Morphology = morph
					}

					tokens = append(tokens, token)
					tokenIdx++
					charPos += len(plainWord) + 1 // +1 for space
				}

				i = wordEnd + 4 // Skip past </w>
				continue
			}
		}
		i++
	}

	// If no <w> tags found, tokenize plain text
	if len(tokens) == 0 {
		plainText := stripMarkup(text)
		tokens = tokenizePlainText(plainText)
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

	// Split by spaces for multiple numbers
	parts := strings.Fields(lemma)
	for _, part := range parts {
		// Remove "strong:" prefix if present
		part = strings.TrimPrefix(part, "strong:")

		// Check if it looks like a Strong's number (H/G followed by digits)
		if len(part) >= 2 && (part[0] == 'H' || part[0] == 'G') {
			// Verify rest is digits
			isNum := true
			for j := 1; j < len(part); j++ {
				if part[j] < '0' || part[j] > '9' {
					isNum = false
					break
				}
			}
			if isNum {
				strongs = append(strongs, part)
			}
		}
	}

	return strongs
}

// tokenizePlainText tokenizes plain text into words.
// Uses unicode.IsLetter and unicode.IsDigit for proper multi-byte character handling.
func tokenizePlainText(text string) []*IRToken {
	var tokens []*IRToken
	var tokenStart int
	var tokenText strings.Builder
	tokenIdx := 0

	inWord := false
	// Use range to iterate over runes, not bytes
	for i, r := range text {
		// Use unicode package for proper character classification
		isWordChar := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '\''

		if isWordChar {
			if !inWord {
				tokenStart = i
				inWord = true
			}
			tokenText.WriteRune(r)
		} else if inWord {
			// End of word
			word := tokenText.String()
			tokens = append(tokens, &IRToken{
				ID:        fmt.Sprintf("t%d", tokenIdx),
				Index:     tokenIdx,
				CharStart: tokenStart,
				CharEnd:   i,
				Text:      word,
				Type:      "word",
			})
			tokenIdx++
			tokenText.Reset()
			inWord = false
		}
	}

	// Handle last word
	if inWord {
		word := tokenText.String()
		tokens = append(tokens, &IRToken{
			ID:        fmt.Sprintf("t%d", tokenIdx),
			Index:     tokenIdx,
			CharStart: tokenStart,
			CharEnd:   len(text),
			Text:      word,
			Type:      "word",
		})
	}

	return tokens
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
		// Look for <reference
		refStart := strings.Index(text[i:], "<reference")
		if refStart < 0 {
			break
		}
		refStart += i

		// Find end of opening tag
		tagEnd := refStart
		for tagEnd < len(text) && text[tagEnd] != '>' {
			tagEnd++
		}
		if tagEnd >= len(text) {
			break
		}

		// Extract osisRef attribute
		tag := text[refStart : tagEnd+1]
		osisRef := extractAttr(tag, "osisRef")

		// Find </reference>
		closeTag := strings.Index(text[tagEnd:], "</reference>")
		if closeTag < 0 {
			i = tagEnd + 1
			continue
		}
		closeTag += tagEnd

		// Use osisRef if available, otherwise use content
		value := osisRef
		if value == "" {
			refContent := text[tagEnd+1 : closeTag]
			value = stripMarkup(refContent)
		}

		annotations = append(annotations, &IRAnnotation{
			ID:       fmt.Sprintf("r%d", *idx),
			Type:     "CROSS_REF",
			StartPos: refStart,
			EndPos:   closeTag + 12,
			Value:    value,
		})
		*idx++

		i = closeTag + 12
	}

	return annotations
}

// writeCorpusJSON writes the corpus to a JSON file.
func writeCorpusJSON(corpus *IRCorpus, path string) error {
	data, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
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
func ParseChapterMarkers(rawMarkup string) []ChapterMarker {
	var markers []ChapterMarker

	// Parse <chapter osisID="..." sID="..." /> start markers
	i := 0
	for i < len(rawMarkup) {
		chapterStart := strings.Index(rawMarkup[i:], "<chapter")
		if chapterStart < 0 {
			break
		}
		chapterStart += i

		// Find end of tag
		tagEnd := chapterStart
		for tagEnd < len(rawMarkup) && rawMarkup[tagEnd] != '>' {
			tagEnd++
		}
		if tagEnd >= len(rawMarkup) {
			break
		}

		tagContent := rawMarkup[chapterStart : tagEnd+1]

		marker := ChapterMarker{}

		// Extract osisID
		if osisID := extractAttr(tagContent, "osisID"); osisID != "" {
			marker.OsisID = osisID
		}

		// Extract sID (start marker)
		if sID := extractAttr(tagContent, "sID"); sID != "" {
			marker.SID = sID
			marker.IsStart = true
		}

		// Extract eID (end marker)
		if eID := extractAttr(tagContent, "eID"); eID != "" {
			marker.EID = eID
			marker.IsEnd = true
		}

		// Only add if we got meaningful data
		if marker.OsisID != "" || marker.SID != "" || marker.EID != "" {
			markers = append(markers, marker)
		}

		i = tagEnd + 1
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
