
// Plugin format-rtf handles Rich Text Format Bible files.
//
// IR Support:
// - extract-ir: Reads RTF Bible format to IR (L2)
// - emit-native: Converts IR to RTF format (L2)
// Note: L2 means basic formatting preserved, some structure may be lost.
package main

import (
	"bufio"
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

func runSDK() {
	if err := format.Run(&format.Config{
		Name:       "RTF",
		Extensions: []string{".rtf"},
		Detect:     detectRTF,
		Parse:      parseRTF,
		Emit:       emitRTF,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectRTF(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".rtf" {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not an .rtf file",
		}, nil
	}

	// Check for RTF signature
	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		}, nil
	}

	if !strings.HasPrefix(string(data), "{\\rtf") {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "missing RTF signature",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: true,
		Format:   "RTF",
		Reason:   "Rich Text Format detected",
	}, nil
}

func parseRTF(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "RTF"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L2"
	corpus.Attributes = make(map[string]string)

	// Store raw for round-trip
	corpus.Attributes["_rtf_raw"] = string(data)

	// Parse RTF content
	corpus.Documents = parseRTFContent(string(data), artifactID)

	return corpus, nil
}

// rtfStripState holds the state during RTF stripping
type rtfStripState struct {
	result  strings.Builder
	inGroup int
}

// isLetter checks if a byte is an ASCII letter
func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// isDigitOrMinus checks if a byte is a digit or minus sign
func isDigitOrMinus(b byte) bool {
	return (b >= '0' && b <= '9') || b == '-'
}

// handleEscapedChar processes escaped special characters
func handleEscapedChar(rtf string, i int, state *rtfStripState) int {
	if i+1 >= len(rtf) {
		return i
	}

	nextChar := rtf[i+1]
	if nextChar == '\\' || nextChar == '{' || nextChar == '}' {
		state.result.WriteByte(nextChar)
		return i + 1
	}
	return i
}

// parseControlWord extracts a control word starting at position i+1
func parseControlWord(rtf string, i int) (word string, endPos int) {
	j := i + 1
	for j < len(rtf) && isLetter(rtf[j]) {
		j++
	}
	for j < len(rtf) && isDigitOrMinus(rtf[j]) {
		j++
	}
	if j < len(rtf) && rtf[j] == ' ' {
		j++
	}
	word = rtf[i+1 : min(j, len(rtf))]
	return word, j
}

// isLineBreakWord checks if a control word represents a line break
func isLineBreakWord(word string) bool {
	return strings.HasPrefix(word, "par") || strings.HasPrefix(word, "line")
}

// handleControlSequence processes a backslash control sequence
func handleControlSequence(rtf string, i int, state *rtfStripState) int {
	if i+1 >= len(rtf) {
		return i
	}

	if rtf[i+1] == '\'' {
		return i + 3
	}

	newPos := handleEscapedChar(rtf, i, state)
	if newPos > i {
		return newPos
	}

	word, endPos := parseControlWord(rtf, i)
	if isLineBreakWord(word) {
		state.result.WriteByte('\n')
	}
	return endPos - 1
}

// shouldOutputChar determines if a character should be added to output
func shouldOutputChar(ch byte, inGroup int) bool {
	return ch != '\n' && ch != '\r' && inGroup <= 1
}

// stripRTF extracts plain text from RTF content
func stripRTF(rtf string) string {
	state := &rtfStripState{}

	for i := 0; i < len(rtf); i++ {
		ch := rtf[i]
		switch ch {
		case '{':
			state.inGroup++
		case '}':
			state.inGroup--
		case '\\':
			i = handleControlSequence(rtf, i, state)
		case '\n', '\r':
			// Ignore newlines in RTF
		default:
			if shouldOutputChar(ch, state.inGroup) {
				state.result.WriteByte(ch)
			}
		}
	}

	return strings.TrimSpace(state.result.String())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func parseRTFContent(rtf, artifactID string) []*ir.Document {
	doc := ir.NewDocument(artifactID, artifactID, 1)

	plainText := stripRTF(rtf)
	versePattern := regexp.MustCompile(`(?m)^(\w+)?\s*(\d+):(\d+)\s+(.+)$`)

	scanner := bufio.NewScanner(strings.NewReader(plainText))
	sequence := 0
	currentBook := artifactID

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		matches := versePattern.FindStringSubmatch(line)
		if len(matches) > 0 {
			if matches[1] != "" {
				currentBook = matches[1]
			}
			chapter, _ := strconv.Atoi(matches[2])
			verse, _ := strconv.Atoi(matches[3])
			text := strings.TrimSpace(matches[4])

			sequence++
			hash := sha256.Sum256([]byte(text))
			osisID := fmt.Sprintf("%s.%d.%d", currentBook, chapter, verse)

			cb := &ir.ContentBlock{
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
									Book:    currentBook,
									Chapter: chapter,
									Verse:   verse,
									OSISID:  osisID,
								},
							},
						},
					},
				},
			}
			doc.ContentBlocks = append(doc.ContentBlocks, cb)
		}
	}

	if currentBook != artifactID {
		doc.ID = currentBook
		doc.Title = currentBook
	}

	return []*ir.Document{doc}
}

func emitRTF(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".rtf")

	// Check for raw RTF for round-trip
	if raw, ok := corpus.Attributes["_rtf_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
			return "", fmt.Errorf("failed to write RTF: %w", err)
		}
		return outputPath, nil
	}

	// Generate RTF from IR
	var buf strings.Builder

	buf.WriteString("{\\rtf1\\ansi\\deff0\n")
	buf.WriteString("{\\fonttbl{\\f0 Times New Roman;}}\n")
	buf.WriteString("{\\colortbl;\\red0\\green0\\blue0;}\n")
	buf.WriteString("\\viewkind4\\uc1\\pard\\f0\\fs24\n")

	if corpus.Title != "" {
		buf.WriteString(fmt.Sprintf("\\qc\\b\\fs32 %s\\b0\\fs24\\par\\par\n", escapeRTF(corpus.Title)))
	}

	for _, doc := range corpus.Documents {
		buf.WriteString(fmt.Sprintf("\\b %s\\b0\\par\n", escapeRTF(doc.Title)))

		currentChapter := 0
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						if span.Ref.Chapter != currentChapter {
							if currentChapter > 0 {
								buf.WriteString("\\par\n")
							}
							currentChapter = span.Ref.Chapter
							buf.WriteString(fmt.Sprintf("\\b Chapter %d\\b0\\par\n", currentChapter))
						}
						buf.WriteString(fmt.Sprintf("\\b %d\\b0  %s\\par\n",
							span.Ref.Verse, escapeRTF(cb.Text)))
					}
				}
			}
		}
		buf.WriteString("\\par\n")
	}

	buf.WriteString("}")

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0600); err != nil {
		return "", fmt.Errorf("failed to write RTF: %w", err)
	}

	return outputPath, nil
}

func escapeRTF(s string) string {
	var buf strings.Builder
	for _, r := range s {
		switch r {
		case '\\', '{', '}':
			buf.WriteRune('\\')
			buf.WriteRune(r)
		default:
			if r > 127 {
				buf.WriteString(fmt.Sprintf("\\u%d?", r))
			} else {
				buf.WriteRune(r)
			}
		}
	}
	return buf.String()
}
