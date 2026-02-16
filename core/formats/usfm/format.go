// Package usfm provides the canonical USFM (Unified Standard Format Markers) Bible format implementation.
package usfm

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// Config defines the USFM format plugin configuration.
var Config = &format.Config{
	Name:       "USFM",
	Extensions: []string{".usfm", ".sfm", ".ptx"},
	Detect:     detectUSFM,
	Parse:      parseUSFM,
	Emit:       emitUSFM,
}

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

// parseState tracks the current parsing context
type parseState struct {
	currentDoc     *ir.Document
	currentChapter int
	blockSeq       int
}

// detectUSFM performs custom USFM format detection
func detectUSFM(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "path is a directory, not a file",
		}, nil
	}

	// Read file and check for USFM markers
	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read: %v", err),
		}, nil
	}

	content := string(data)

	// Check for USFM markers
	if strings.Contains(content, "\\id ") || strings.Contains(content, "\\c ") ||
		strings.Contains(content, "\\v ") || strings.Contains(content, "\\p") {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "USFM",
			Reason:   "USFM markers detected",
		}, nil
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".usfm" || ext == ".sfm" || ext == ".ptx" {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "USFM",
			Reason:   "USFM file extension detected",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "not a USFM file",
	}, nil
}

// parseUSFM parses a USFM file and returns an IR Corpus
func parseUSFM(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	corpus, err := parseUSFMToIR(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse USFM: %w", err)
	}

	return corpus, nil
}

// emitUSFM converts an IR Corpus to USFM format
func emitUSFM(corpus *ir.Corpus, outputDir string) (string, error) {
	usfmData, err := emitUSFMFromIR(corpus)
	if err != nil {
		return "", fmt.Errorf("failed to emit USFM: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".usfm")
	if err := os.WriteFile(outputPath, usfmData, 0600); err != nil {
		return "", fmt.Errorf("failed to write USFM: %w", err)
	}

	return outputPath, nil
}

// parseUSFMToIR converts USFM text to IR Corpus
func parseUSFMToIR(data []byte) (*ir.Corpus, error) {
	corpus := initializeCorpus(string(data))
	state := &parseState{}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "\\") {
			processUSFMMarker(line, corpus, state)
		}
	}

	corpus.SourceHash = computeHash(data)
	return corpus, nil
}

// initializeCorpus creates a new corpus with default values
func initializeCorpus(content string) *ir.Corpus {
	corpus := &ir.Corpus{
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "USFM",
		LossClass:    "L0",
		Attributes:   make(map[string]string),
	}
	corpus.Attributes["_format_raw"] = content
	return corpus
}

// processUSFMMarker handles a single USFM marker line
func processUSFMMarker(line string, corpus *ir.Corpus, state *parseState) {
	marker, value := parseMarkerAndValue(line)

	switch marker {
	case "id":
		handleIDMarker(value, corpus, state)
	case "h", "toc1", "toc2", "toc3":
		handleHeaderMarker(marker, value, state)
	case "mt", "mt1", "mt2", "mt3":
		handleTitleMarker(marker, value, corpus, state)
	case "c":
		handleChapterMarker(value, state)
	case "v":
		handleVerseMarker(value, corpus, state)
	case "p", "m", "pi", "mi", "nb":
		handleParagraphMarker(marker, value, state)
	case "q", "q1", "q2", "q3", "qr", "qc", "qm":
		handlePoetryMarker(marker, value, state)
	}
}

// parseMarkerAndValue splits a USFM line into marker and value
func parseMarkerAndValue(line string) (string, string) {
	parts := strings.SplitN(line, " ", 2)
	marker := strings.TrimPrefix(parts[0], "\\")
	value := ""
	if len(parts) > 1 {
		value = parts[1]
	}
	return marker, value
}

// handleIDMarker processes book ID markers
func handleIDMarker(value string, corpus *ir.Corpus, state *parseState) {
	idParts := strings.Fields(value)
	if len(idParts) == 0 {
		return
	}

	bookID := strings.ToUpper(idParts[0])
	corpus.ID = bookID
	state.currentDoc = &ir.Document{
		ID:         bookID,
		Order:      len(corpus.Documents) + 1,
		Attributes: make(map[string]string),
	}
	if name, ok := bookNames[bookID]; ok {
		state.currentDoc.Title = name
	}
	corpus.Documents = append(corpus.Documents, state.currentDoc)
}

// handleHeaderMarker processes header and TOC markers
func handleHeaderMarker(marker, value string, state *parseState) {
	if state.currentDoc == nil || value == "" {
		return
	}
	if marker == "h" && state.currentDoc.Title == "" {
		state.currentDoc.Title = value
	}
	state.currentDoc.Attributes[marker] = value
}

// handleTitleMarker processes main title markers
func handleTitleMarker(marker, value string, corpus *ir.Corpus, state *parseState) {
	if corpus.Title == "" && value != "" {
		corpus.Title = value
	}
	if state.currentDoc != nil {
		state.currentDoc.Attributes[marker] = value
	}
}

// handleChapterMarker processes chapter markers
func handleChapterMarker(value string, state *parseState) {
	if matches := chapterRegex.FindStringSubmatch(value); len(matches) > 0 {
		state.currentChapter, _ = strconv.Atoi(matches[1])
	}
}

// handleVerseMarker processes verse markers
func handleVerseMarker(value string, corpus *ir.Corpus, state *parseState) {
	if state.currentDoc == nil {
		return
	}

	verseNum, verseEnd, verseText := parseVerseContent(value)
	if verseText == "" {
		return
	}

	state.blockSeq++
	block := createVerseBlock(state.blockSeq, verseText, corpus.ID, state.currentChapter, verseNum, verseEnd)
	state.currentDoc.ContentBlocks = append(state.currentDoc.ContentBlocks, block)
}

// parseVerseContent extracts verse number and text from verse marker value
func parseVerseContent(value string) (verseNum, verseEnd int, text string) {
	text = value
	if matches := verseNumRegex.FindStringSubmatch(value); len(matches) > 0 {
		verseNum, _ = strconv.Atoi(matches[1])
		if matches[2] != "" {
			verseEnd, _ = strconv.Atoi(matches[2])
		}
		text = strings.TrimSpace(value[len(matches[0]):])
	}
	return verseNum, verseEnd, text
}

// createVerseBlock creates a content block for a verse
func createVerseBlock(blockSeq int, text, bookID string, chapter, verseNum, verseEnd int) *ir.ContentBlock {
	osisID := fmt.Sprintf("%s.%d.%d", bookID, chapter, verseNum)
	block := &ir.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", blockSeq),
		Sequence: blockSeq,
		Text:     text,
		Hash:     computeHash([]byte(text)),
		Anchors: []*ir.Anchor{
			{
				ID:       fmt.Sprintf("a-%d-0", blockSeq),
				Position: 0,
				Spans: []*ir.Span{
					{
						ID:            fmt.Sprintf("s-%s", osisID),
						Type:          "VERSE",
						StartAnchorID: fmt.Sprintf("a-%d-0", blockSeq),
						Ref: &ir.Ref{
							Book:     bookID,
							Chapter:  chapter,
							Verse:    verseNum,
							VerseEnd: verseEnd,
							OSISID:   osisID,
						},
					},
				},
			},
		},
	}
	return block
}

// handleParagraphMarker processes paragraph markers
func handleParagraphMarker(marker, value string, state *parseState) {
	if state.currentDoc == nil || value == "" {
		return
	}
	state.blockSeq++
	block := createTextBlock(state.blockSeq, value, "paragraph", marker)
	state.currentDoc.ContentBlocks = append(state.currentDoc.ContentBlocks, block)
}

// handlePoetryMarker processes poetry markers
func handlePoetryMarker(marker, value string, state *parseState) {
	if state.currentDoc == nil || value == "" {
		return
	}
	state.blockSeq++
	block := createTextBlock(state.blockSeq, value, "poetry", marker)
	state.currentDoc.ContentBlocks = append(state.currentDoc.ContentBlocks, block)
}

// createTextBlock creates a content block for text with type and marker
func createTextBlock(blockSeq int, text, blockType, marker string) *ir.ContentBlock {
	return &ir.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", blockSeq),
		Sequence: blockSeq,
		Text:     text,
		Hash:     computeHash([]byte(text)),
		Attributes: map[string]interface{}{
			"type":   blockType,
			"marker": marker,
		},
	}
}

// computeHash generates a SHA256 hash for the given data
func computeHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// emitUSFMFromIR converts IR Corpus back to USFM text
func emitUSFMFromIR(corpus *ir.Corpus) ([]byte, error) {
	// Check if we have the original raw USFM for L0 lossless round-trip
	if rawUSFM, ok := corpus.Attributes["_format_raw"]; ok && rawUSFM != "" {
		return []byte(rawUSFM), nil
	}

	// Otherwise, reconstruct USFM from IR structure
	var buf bytes.Buffer

	for _, doc := range corpus.Documents {
		// Write book ID
		buf.WriteString(fmt.Sprintf("\\id %s\n", doc.ID))

		// Write header
		if doc.Title != "" {
			buf.WriteString(fmt.Sprintf("\\h %s\n", doc.Title))
		}

		// Write attributes (toc entries, mt, etc.)
		if doc.Attributes != nil {
			for key, val := range doc.Attributes {
				if key != "h" && !strings.HasPrefix(key, "_") {
					buf.WriteString(fmt.Sprintf("\\%s %s\n", key, val))
				}
			}
		}

		currentChapter := 0

		for _, block := range doc.ContentBlocks {
			// Check for verse spans to determine chapter/verse
			for _, anchor := range block.Anchors {
				for _, span := range anchor.Spans {
					if span.Type == "VERSE" && span.Ref != nil {
						// Write chapter marker if changed
						if span.Ref.Chapter != currentChapter {
							currentChapter = span.Ref.Chapter
							buf.WriteString(fmt.Sprintf("\\c %d\n", currentChapter))
						}

						// Write verse
						if span.Ref.VerseEnd > 0 && span.Ref.VerseEnd != span.Ref.Verse {
							buf.WriteString(fmt.Sprintf("\\v %d-%d %s\n", span.Ref.Verse, span.Ref.VerseEnd, block.Text))
						} else {
							buf.WriteString(fmt.Sprintf("\\v %d %s\n", span.Ref.Verse, block.Text))
						}
						break
					}
				}
			}

			// Handle non-verse blocks (paragraphs, poetry)
			if len(block.Anchors) == 0 && block.Text != "" {
				if block.Attributes != nil {
					if marker, ok := block.Attributes["marker"].(string); ok {
						buf.WriteString(fmt.Sprintf("\\%s %s\n", marker, block.Text))
					} else if blockType, ok := block.Attributes["type"].(string); ok {
						switch blockType {
						case "poetry":
							buf.WriteString(fmt.Sprintf("\\q %s\n", block.Text))
						case "paragraph":
							buf.WriteString(fmt.Sprintf("\\p %s\n", block.Text))
						default:
							buf.WriteString(fmt.Sprintf("\\p %s\n", block.Text))
						}
					}
				}
			}
		}
	}

	return buf.Bytes(), nil
}
