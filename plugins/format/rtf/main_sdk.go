//go:build sdk

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

func main() {
	format.Run(Detect, Parse, Emit)
}

// Detect checks if the file is a valid RTF Bible file
func Detect(path string) (*ipc.DetectResult, error) {
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

// Parse reads an RTF file and converts it to IR
func Parse(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ir.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "RTF",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L2",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_rtf_raw"] = string(data)

	// Parse RTF content
	corpus.Documents = parseRTFContent(string(data), artifactID)

	return corpus, nil
}

// Emit converts IR to RTF format
func Emit(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".rtf")

	// Check for raw RTF for round-trip
	if raw, ok := corpus.Attributes["_rtf_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			return "", fmt.Errorf("failed to write RTF: %w", err)
		}
		return outputPath, nil
	}

	// Generate RTF from IR
	var buf strings.Builder

	// RTF header
	buf.WriteString("{\\rtf1\\ansi\\deff0\n")
	buf.WriteString("{\\fonttbl{\\f0 Times New Roman;}}\n")
	buf.WriteString("{\\colortbl;\\red0\\green0\\blue0;}\n")
	buf.WriteString("\\viewkind4\\uc1\\pard\\f0\\fs24\n")

	// Title
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

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write RTF: %w", err)
	}

	return outputPath, nil
}

// stripRTF extracts plain text from RTF content
func stripRTF(rtf string) string {
	// Remove RTF groups and control words
	var result strings.Builder
	inGroup := 0
	skipNext := false

	for i := 0; i < len(rtf); i++ {
		if skipNext {
			skipNext = false
			continue
		}

		ch := rtf[i]
		switch ch {
		case '{':
			inGroup++
		case '}':
			inGroup--
		case '\\':
			// Skip control word
			if i+1 < len(rtf) {
				if rtf[i+1] == '\'' {
					// Hex escape like \'e9 - skip 3 chars
					i += 3
				} else if rtf[i+1] == '\\' || rtf[i+1] == '{' || rtf[i+1] == '}' {
					// Escaped special char
					result.WriteByte(rtf[i+1])
					i++
				} else {
					// Skip control word
					j := i + 1
					for j < len(rtf) && ((rtf[j] >= 'a' && rtf[j] <= 'z') || (rtf[j] >= 'A' && rtf[j] <= 'Z')) {
						j++
					}
					// Skip optional numeric parameter
					for j < len(rtf) && (rtf[j] >= '0' && rtf[j] <= '9' || rtf[j] == '-') {
						j++
					}
					// Skip optional space after control word
					if j < len(rtf) && rtf[j] == ' ' {
						j++
					}
					// Check for line break
					word := rtf[i+1 : min(j, len(rtf))]
					if strings.HasPrefix(word, "par") || strings.HasPrefix(word, "line") {
						result.WriteByte('\n')
					}
					i = j - 1
				}
			}
		case '\n', '\r':
			// Ignore newlines in RTF
		default:
			if inGroup <= 1 { // Only output text at top level or first group
				result.WriteByte(ch)
			}
		}
	}

	return strings.TrimSpace(result.String())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func parseRTFContent(rtf, artifactID string) []*ir.Document {
	doc := &ir.Document{
		ID:    artifactID,
		Title: artifactID,
		Order: 1,
	}

	// Strip RTF to plain text
	plainText := stripRTF(rtf)

	// Parse verses from plain text
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

func escapeRTF(s string) string {
	var buf strings.Builder
	for _, r := range s {
		switch r {
		case '\\', '{', '}':
			buf.WriteRune('\\')
			buf.WriteRune(r)
		default:
			if r > 127 {
				// Unicode escape
				buf.WriteString(fmt.Sprintf("\\u%d?", r))
			} else {
				buf.WriteRune(r)
			}
		}
	}
	return buf.String()
}
