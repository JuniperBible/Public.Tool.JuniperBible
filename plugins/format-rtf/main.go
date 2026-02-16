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
)

// IPCRequest is the incoming JSON request.
type IPCRequest struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args,omitempty"`
}

// IPCResponse is the outgoing JSON response.
type IPCResponse struct {
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// DetectResult is the result of a detect command.
type DetectResult struct {
	Detected bool   `json:"detected"`
	Format   string `json:"format,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// IngestResult is the result of an ingest command.
type IngestResult struct {
	ArtifactID string            `json:"artifact_id"`
	BlobSHA256 string            `json:"blob_sha256"`
	SizeBytes  int64             `json:"size_bytes"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// EnumerateResult is the result of an enumerate command.
type EnumerateResult struct {
	Entries []EnumerateEntry `json:"entries"`
}

// EnumerateEntry represents a file entry.
type EnumerateEntry struct {
	Path      string            `json:"path"`
	SizeBytes int64             `json:"size_bytes"`
	IsDir     bool              `json:"is_dir"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ExtractIRResult is the result of an extract-ir command.
type ExtractIRResult struct {
	IRPath     string      `json:"ir_path"`
	LossClass  string      `json:"loss_class"`
	LossReport *LossReport `json:"loss_report,omitempty"`
}

// EmitNativeResult is the result of an emit-native command.
type EmitNativeResult struct {
	OutputPath string      `json:"output_path"`
	Format     string      `json:"format"`
	LossClass  string      `json:"loss_class"`
	LossReport *LossReport `json:"loss_report,omitempty"`
}

// LossReport describes any data loss during conversion.
type LossReport struct {
	SourceFormat string        `json:"source_format"`
	TargetFormat string        `json:"target_format"`
	LossClass    string        `json:"loss_class"`
	LostElements []LostElement `json:"lost_elements,omitempty"`
	Warnings     []string      `json:"warnings,omitempty"`
}

// LostElement describes a specific element that was lost.
type LostElement struct {
	Path          string      `json:"path"`
	ElementType   string      `json:"element_type"`
	Reason        string      `json:"reason"`
	OriginalValue interface{} `json:"original_value,omitempty"`
}

// IR Types
type Corpus struct {
	ID            string            `json:"id"`
	Version       string            `json:"version"`
	ModuleType    string            `json:"module_type"`
	Versification string            `json:"versification,omitempty"`
	Language      string            `json:"language,omitempty"`
	Title         string            `json:"title,omitempty"`
	Description   string            `json:"description,omitempty"`
	Publisher     string            `json:"publisher,omitempty"`
	Rights        string            `json:"rights,omitempty"`
	SourceFormat  string            `json:"source_format,omitempty"`
	Documents     []*Document       `json:"documents,omitempty"`
	SourceHash    string            `json:"source_hash,omitempty"`
	LossClass     string            `json:"loss_class,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type Document struct {
	ID            string            `json:"id"`
	Title         string            `json:"title,omitempty"`
	Order         int               `json:"order"`
	ContentBlocks []*ContentBlock   `json:"content_blocks,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type ContentBlock struct {
	ID         string                 `json:"id"`
	Sequence   int                    `json:"sequence"`
	Text       string                 `json:"text"`
	Tokens     []*Token               `json:"tokens,omitempty"`
	Anchors    []*Anchor              `json:"anchors,omitempty"`
	Hash       string                 `json:"hash,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type Token struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Text     string `json:"text"`
	StartPos int    `json:"start_pos"`
	EndPos   int    `json:"end_pos"`
}

type Anchor struct {
	ID       string  `json:"id"`
	Position int     `json:"position"`
	Spans    []*Span `json:"spans,omitempty"`
}

type Span struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	StartAnchorID string                 `json:"start_anchor_id"`
	EndAnchorID   string                 `json:"end_anchor_id,omitempty"`
	Ref           *Ref                   `json:"ref,omitempty"`
	Attributes    map[string]interface{} `json:"attributes,omitempty"`
}

type Ref struct {
	Book     string `json:"book"`
	Chapter  int    `json:"chapter,omitempty"`
	Verse    int    `json:"verse,omitempty"`
	VerseEnd int    `json:"verse_end,omitempty"`
	SubVerse string `json:"sub_verse,omitempty"`
	OSISID   string `json:"osis_id,omitempty"`
}

func main() {
	var req IPCRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		respondError(fmt.Sprintf("failed to decode request: %v", err))
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
		respondError(fmt.Sprintf("unknown command: %s", req.Command))
	}
}

func handleDetect(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".rtf" {
		respond(&DetectResult{
			Detected: false,
			Reason:   "not an .rtf file",
		})
		return
	}

	// Check for RTF signature
	data, err := os.ReadFile(path)
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		})
		return
	}

	if !strings.HasPrefix(string(data), "{\\rtf") {
		respond(&DetectResult{
			Detected: false,
			Reason:   "missing RTF signature",
		})
		return
	}

	respond(&DetectResult{
		Detected: true,
		Format:   "RTF",
		Reason:   "Rich Text Format detected",
	})
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		respondError(fmt.Sprintf("failed to create blob dir: %v", err))
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write blob: %v", err))
		return
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	respond(&IngestResult{
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
		respondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to stat: %v", err))
		return
	}

	respond(&EnumerateResult{
		Entries: []EnumerateEntry{
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
		respondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &Corpus{
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
		respondError(fmt.Sprintf("failed to serialize IR: %v", err))
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write IR: %v", err))
		return
	}

	respond(&ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L2",
		LossReport: &LossReport{
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

func parseRTFContent(rtf, artifactID string) []*Document {
	doc := &Document{
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

			cb := &ContentBlock{
				ID:       fmt.Sprintf("cb-%d", sequence),
				Sequence: sequence,
				Text:     text,
				Hash:     hex.EncodeToString(hash[:]),
				Anchors: []*Anchor{
					{
						ID:       fmt.Sprintf("a-%d-0", sequence),
						Position: 0,
						Spans: []*Span{
							{
								ID:            fmt.Sprintf("s-%s", osisID),
								Type:          "VERSE",
								StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
								Ref: &Ref{
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

	return []*Document{doc}
}

func handleEmitNative(args map[string]interface{}) {
	irPath, ok := args["ir_path"].(string)
	if !ok {
		respondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(irPath)
	if err != nil {
		respondError(fmt.Sprintf("failed to read IR file: %v", err))
		return
	}

	var corpus Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		respondError(fmt.Sprintf("failed to parse IR: %v", err))
		return
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".rtf")

	// Check for raw RTF for round-trip
	if raw, ok := corpus.Attributes["_rtf_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			respondError(fmt.Sprintf("failed to write RTF: %v", err))
			return
		}

		respond(&EmitNativeResult{
			OutputPath: outputPath,
			Format:     "RTF",
			LossClass:  "L0",
			LossReport: &LossReport{
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
		respondError(fmt.Sprintf("failed to write RTF: %v", err))
		return
	}

	respond(&EmitNativeResult{
		OutputPath: outputPath,
		Format:     "RTF",
		LossClass:  "L2",
		LossReport: &LossReport{
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

func respond(result interface{}) {
	resp := IPCResponse{
		Status: "ok",
		Result: result,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
}

func respondError(msg string) {
	resp := IPCResponse{
		Status: "error",
		Error:  msg,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
	os.Exit(1)
}
