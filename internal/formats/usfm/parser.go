// Package usfm provides the embedded handler for USFM Bible format plugin.
package usfm

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

// USFM parsing helpers
var (
	markerRegex   = regexp.MustCompile(`\\([a-zA-Z0-9]+)\*?(?:\s|$)`)
	verseNumRegex = regexp.MustCompile(`^(\d+)(?:-(\d+))?`)
	chapterRegex  = regexp.MustCompile(`^(\d+)`)
)

// Common USFM book IDs
var bookNames = map[string]string{
	"GEN": "Genesis", "EXO": "Exodus", "LEV": "Leviticus", "NUM": "Numbers",
	"DEU": "Deuteronomy", "JOS": "Joshua", "JDG": "Judges", "RUT": "Ruth",
	"1SA": "1 Samuel", "2SA": "2 Samuel", "1KI": "1 Kings", "2KI": "2 Kings",
	"1CH": "1 Chronicles", "2CH": "2 Chronicles", "EZR": "Ezra", "NEH": "Nehemiah",
	"EST": "Esther", "JOB": "Job", "PSA": "Psalms", "PRO": "Proverbs",
	"ECC": "Ecclesiastes", "SNG": "Song of Solomon", "ISA": "Isaiah", "JER": "Jeremiah",
	"LAM": "Lamentations", "EZK": "Ezekiel", "DAN": "Daniel", "HOS": "Hosea",
	"JOL": "Joel", "AMO": "Amos", "OBA": "Obadiah", "JON": "Jonah",
	"MIC": "Micah", "NAM": "Nahum", "HAB": "Habakkuk", "ZEP": "Zephaniah",
	"HAG": "Haggai", "ZEC": "Zechariah", "MAL": "Malachi",
	"MAT": "Matthew", "MRK": "Mark", "LUK": "Luke", "JHN": "John",
	"ACT": "Acts", "ROM": "Romans", "1CO": "1 Corinthians", "2CO": "2 Corinthians",
	"GAL": "Galatians", "EPH": "Ephesians", "PHP": "Philippians", "COL": "Colossians",
	"1TH": "1 Thessalonians", "2TH": "2 Thessalonians", "1TI": "1 Timothy", "2TI": "2 Timothy",
	"TIT": "Titus", "PHM": "Philemon", "HEB": "Hebrews", "JAS": "James",
	"1PE": "1 Peter", "2PE": "2 Peter", "1JN": "1 John", "2JN": "2 John",
	"3JN": "3 John", "JUD": "Jude", "REV": "Revelation",
}

// parserState holds the parsing context
type parserState struct {
	corpus     *ir.Corpus
	currentDoc *ir.Document
	blockSeq   int
}

// markerHandler is a function that processes a specific marker
type markerHandler func(state *parserState, value string)

// markerHandlers maps marker types to their handler functions
var markerHandlers = map[string]markerHandler{
	"id":   handleIDMarker,
	"h":    handleHeaderMarker,
	"toc1": handleHeaderMarker,
	"toc2": handleHeaderMarker,
	"toc3": handleHeaderMarker,
	"mt":   handleTitleMarker,
	"mt1":  handleTitleMarker,
	"mt2":  handleTitleMarker,
	"mt3":  handleTitleMarker,
	"c":    handleChapterMarker,
	"v":    handleVerseMarker,
	"p":    handleTextBlockMarker,
	"m":    handleTextBlockMarker,
	"pi":   handleTextBlockMarker,
	"mi":   handleTextBlockMarker,
	"nb":   handleTextBlockMarker,
	"q":    handleTextBlockMarker,
	"q1":   handleTextBlockMarker,
	"q2":   handleTextBlockMarker,
	"q3":   handleTextBlockMarker,
	"qr":   handleTextBlockMarker,
	"qc":   handleTextBlockMarker,
	"qm":   handleTextBlockMarker,
}

// handleIDMarker processes book ID markers
func handleIDMarker(state *parserState, value string) {
	idParts := strings.Fields(value)
	if len(idParts) == 0 {
		return
	}

	bookID := strings.ToUpper(idParts[0])
	state.corpus.ID = bookID
	state.currentDoc = &ir.Document{
		ID:            bookID,
		Order:         len(state.corpus.Documents) + 1,
		ContentBlocks: []*ir.ContentBlock{},
	}
	if name, ok := bookNames[bookID]; ok {
		state.currentDoc.Title = name
	}
	state.corpus.Documents = append(state.corpus.Documents, state.currentDoc)
}

// handleHeaderMarker processes header and TOC markers
func handleHeaderMarker(state *parserState, value string) {
	if state.currentDoc != nil && value != "" && state.currentDoc.Title == "" {
		state.currentDoc.Title = value
	}
}

// handleTitleMarker processes main title markers
func handleTitleMarker(state *parserState, value string) {
	if state.corpus.Title == "" && value != "" {
		state.corpus.Title = value
	}
}

// handleChapterMarker processes chapter markers
func handleChapterMarker(state *parserState, value string) {
	// Chapter marker (parsing simplified for now)
	_ = value
}

// handleVerseMarker processes verse markers
func handleVerseMarker(state *parserState, value string) {
	if state.currentDoc == nil {
		return
	}

	verseText := extractVerseText(value)
	if verseText == "" {
		return
	}

	state.blockSeq++
	block := createContentBlock(state.blockSeq, verseText, true)
	state.currentDoc.ContentBlocks = append(state.currentDoc.ContentBlocks, block)
}

// handleTextBlockMarker processes paragraph and poetry markers
func handleTextBlockMarker(state *parserState, value string) {
	if state.currentDoc == nil || value == "" {
		return
	}

	state.blockSeq++
	block := createContentBlock(state.blockSeq, value, false)
	state.currentDoc.ContentBlocks = append(state.currentDoc.ContentBlocks, block)
}

// extractVerseText extracts the text portion from a verse marker value
func extractVerseText(value string) string {
	if matches := verseNumRegex.FindStringSubmatch(value); len(matches) > 0 {
		return strings.TrimSpace(value[len(matches[0]):])
	}
	return value
}

// createContentBlock creates a content block with optional anchor
func createContentBlock(seq int, text string, withAnchor bool) *ir.ContentBlock {
	block := &ir.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", seq),
		Sequence: seq,
		Text:     text,
	}

	if withAnchor {
		block.Anchors = []*ir.Anchor{
			{
				ID:             fmt.Sprintf("a-%d-0", seq),
				ContentBlockID: fmt.Sprintf("cb-%d", seq),
				CharOffset:     0,
			},
		}
	}

	block.ComputeHash()
	return block
}

// processMarkerLine parses and processes a line containing a USFM marker
func processMarkerLine(state *parserState, line string) {
	parts := strings.SplitN(line, " ", 2)
	marker := strings.TrimPrefix(parts[0], "\\")

	var value string
	if len(parts) > 1 {
		value = parts[1]
	}

	if handler, ok := markerHandlers[marker]; ok {
		handler(state, value)
	}
}

// parseUSFMToIR converts USFM text to IR Corpus
func parseUSFMToIR(data []byte) (*ir.Corpus, error) {
	state := &parserState{
		corpus: &ir.Corpus{
			Version:    "1.0.0",
			ModuleType: ir.ModuleBible,
			LossClass:  ir.LossL0,
			Documents:  []*ir.Document{},
		},
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "\\") {
			continue
		}
		processMarkerLine(state, line)
	}

	// Compute source hash
	h := sha256.Sum256(data)
	state.corpus.SourceHash = hex.EncodeToString(h[:])

	return state.corpus, nil
}

// emitUSFMFromIR converts IR Corpus back to USFM text
func emitUSFMFromIR(corpus *ir.Corpus) ([]byte, error) {
	var buf bytes.Buffer

	for _, doc := range corpus.Documents {
		// Write book ID
		buf.WriteString(fmt.Sprintf("\\id %s\n", doc.ID))

		// Write header
		if doc.Title != "" {
			buf.WriteString(fmt.Sprintf("\\h %s\n", doc.Title))
		}

		for _, block := range doc.ContentBlocks {
			// Check for verse spans to determine chapter/verse
			hasVerse := false
			for _, anchor := range block.Anchors {
				// Simple heuristic: if we have anchors at start, assume verse content
				if anchor.CharOffset == 0 {
					// Try to extract chapter/verse from block ID or infer from content
					// For now, just mark as having verse
					hasVerse = true
					break
				}
			}

			if hasVerse {
				// Try to infer chapter/verse from text or use simple increment
				// This is simplified - in real implementation we'd need better tracking
				buf.WriteString(fmt.Sprintf("\\v %s\n", block.Text))
			} else {
				// Non-verse block
				buf.WriteString(fmt.Sprintf("\\p %s\n", block.Text))
			}
		}
	}

	return buf.Bytes(), nil
}
