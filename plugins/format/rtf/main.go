//go:build !sdk

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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

func main() {
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorf("failed to decode request: %v", err)
		return
	}

	switch req.Command {
	case "detect":
		handleDetect(req.Args)
	case "ingest":
		handleIngest(req.Args)
	case "enumerate":
		handleEnumerate(req.Args)
	case "extract-ir":
		handleExtractIR(req.Args)
	case "emit-native":
		handleEmitNative(req.Args)
	default:
		ipc.RespondErrorf("unknown command: %s", req.Command)
	}
}

func handleDetect(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".rtf" {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not an .rtf file",
		})
		return
	}

	// Check for RTF signature
	data, err := os.ReadFile(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		})
		return
	}

	if !strings.HasPrefix(string(data), "{\\rtf") {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "missing RTF signature",
		})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   "RTF",
		Reason:   "Rich Text Format detected",
	})
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		ipc.RespondErrorf("failed to create blob dir: %v", err)
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		ipc.RespondErrorf("failed to write blob: %v", err)
		return
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "RTF",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		ipc.RespondErrorf("failed to stat: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: []ipc.EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
				Metadata:  map[string]string{"format": "RTF"},
			},
		},
	})
}

func handleExtractIR(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ipc.Corpus{
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

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to serialize IR: %v", err)
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}
	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L2",
		LossReport: &ipc.LossReport{
			SourceFormat: "RTF",
			TargetFormat: "IR",
			LossClass:    "L2",
			Warnings:     []string{"RTF formatting codes may not fully translate to IR"},
		},
	})
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

func parseRTFContent(rtf, artifactID string) []*ipc.Document {
	doc := &ipc.Document{
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

			cb := &ipc.ContentBlock{
				ID:       fmt.Sprintf("cb-%d", sequence),
				Sequence: sequence,
				Text:     text,
				Hash:     hex.EncodeToString(hash[:]),
				Anchors: []*ipc.Anchor{
					{
						ID:       fmt.Sprintf("a-%d-0", sequence),
						Position: 0,
						Spans: []*ipc.Span{
							{
								ID:            fmt.Sprintf("s-%s", osisID),
								Type:          "VERSE",
								StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
								Ref: &ipc.Ref{
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

	return []*ipc.Document{doc}
}

func handleEmitNative(args map[string]interface{}) {
	irPath, ok := args["ir_path"].(string)
	if !ok {
		ipc.RespondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR file: %v", err)
		return
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		ipc.RespondErrorf("failed to parse IR: %v", err)
		return
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".rtf")

	// Check for raw RTF for round-trip
	if raw, ok := corpus.Attributes["_rtf_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			ipc.RespondErrorf("failed to write RTF: %v", err)
			return
		}
		ipc.MustRespond(&ipc.EmitNativeResult{
			OutputPath: outputPath,
			Format:     "RTF",
			LossClass:  "L0",
			LossReport: &ipc.LossReport{
				SourceFormat: "IR",
				TargetFormat: "RTF",
				LossClass:    "L0",
			},
		})
		return
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
		ipc.RespondErrorf("failed to write RTF: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "RTF",
		LossClass:  "L2",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "RTF",
			LossClass:    "L2",
			Warnings:     []string{"Generated RTF uses basic formatting only"},
		},
	})
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
