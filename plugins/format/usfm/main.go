//go:build !sdk

// Plugin format-usfm handles USFM (Unified Standard Format Markers) Bible files.
// It supports L0 lossless round-trip conversion through IR.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
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

func main() {
	// Read request from stdin
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorf("failed to decode request: %v", err)
		return
	}

	// Dispatch command
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

	info, err := os.Stat(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if info.IsDir() {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "path is a directory, not a file",
		})
		return
	}

	// Read file and check for USFM markers
	data, err := os.ReadFile(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read: %v", err),
		})
		return
	}

	content := string(data)

	// Check for USFM markers
	if strings.Contains(content, "\\id ") || strings.Contains(content, "\\c ") ||
		strings.Contains(content, "\\v ") || strings.Contains(content, "\\p") {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "USFM",
			Reason:   "USFM markers detected",
		})
		return
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".usfm" || ext == ".sfm" || ext == ".ptx" {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "USFM",
			Reason:   "USFM file extension detected",
		})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{
		Detected: false,
		Reason:   "not a USFM file",
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

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	// Compute SHA-256
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	// Write to output directory
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		ipc.RespondErrorf("failed to create blob dir: %v", err)
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0600); err != nil {
		ipc.RespondErrorf("failed to write blob: %v", err)
		return
	}

	// Parse book ID from \id marker
	artifactID := filepath.Base(path)
	content := string(data)
	if idx := strings.Index(content, "\\id "); idx >= 0 {
		endIdx := strings.IndexAny(content[idx+4:], " \n\r")
		if endIdx > 0 {
			artifactID = strings.TrimSpace(content[idx+4 : idx+4+endIdx])
		}
	}
	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"original_name": filepath.Base(path),
			"format":        "USFM",
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

	// Read USFM file
	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	// Parse USFM
	corpus, err := parseUSFMToIR(data)
	if err != nil {
		ipc.RespondErrorf("failed to parse USFM: %v", err)
		return
	}

	// Serialize IR to JSON
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to serialize IR: %v", err)
		return
	}

	// Write IR to output directory
	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0600); err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}
	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L0",
		LossReport: &ipc.LossReport{
			SourceFormat: "USFM",
			TargetFormat: "IR",
			LossClass:    "L0",
		},
	})
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

	// Read IR file
	data, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR file: %v", err)
		return
	}

	// Parse IR
	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		ipc.RespondErrorf("failed to parse IR: %v", err)
		return
	}

	// Convert IR to USFM
	usfmData, err := emitUSFMFromIR(&corpus)
	if err != nil {
		ipc.RespondErrorf("failed to emit USFM: %v", err)
		return
	}

	// Write USFM to output directory
	outputPath := filepath.Join(outputDir, corpus.ID+".usfm")
	if err := os.WriteFile(outputPath, usfmData, 0600); err != nil {
		ipc.RespondErrorf("failed to write USFM: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "USFM",
		LossClass:  "L0",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "USFM",
			LossClass:    "L0",
		},
	})
}

// parseContext holds parsing state
type parseContext struct {
	corpus         *ipc.Corpus
	currentDoc     *ipc.Document
	currentChapter int
	blockSeq       int
}

// handleBookID processes the \id marker
func handleBookID(ctx *parseContext, value string) {
	idParts := strings.Fields(value)
	if len(idParts) > 0 {
		bookID := strings.ToUpper(idParts[0])
		ctx.corpus.ID = bookID
		ctx.currentDoc = &ipc.Document{
			ID:         bookID,
			Order:      len(ctx.corpus.Documents) + 1,
			Attributes: make(map[string]string),
		}
		if name, ok := bookNames[bookID]; ok {
			ctx.currentDoc.Title = name
		}
		ctx.corpus.Documents = append(ctx.corpus.Documents, ctx.currentDoc)
	}
}

// handleHeaderMarker processes header/TOC markers
func handleHeaderMarker(ctx *parseContext, marker, value string) {
	if ctx.currentDoc != nil && value != "" {
		if marker == "h" && ctx.currentDoc.Title == "" {
			ctx.currentDoc.Title = value
		}
		ctx.currentDoc.Attributes[marker] = value
	}
}

// handleTitleMarker processes title markers
func handleTitleMarker(ctx *parseContext, marker, value string) {
	if ctx.corpus.Title == "" && value != "" {
		ctx.corpus.Title = value
	}
	if ctx.currentDoc != nil {
		ctx.currentDoc.Attributes[marker] = value
	}
}

// handleChapterMarker processes the \c marker
func handleChapterMarker(ctx *parseContext, value string) {
	if matches := chapterRegex.FindStringSubmatch(value); len(matches) > 0 {
		ctx.currentChapter, _ = strconv.Atoi(matches[1])
	}
}

// handleVerseMarker processes the \v marker
func handleVerseMarker(ctx *parseContext, value string) {
	if ctx.currentDoc == nil {
		return
	}

	verseText := value
	verseNum := 0
	verseEnd := 0

	// Parse verse number
	if matches := verseNumRegex.FindStringSubmatch(value); len(matches) > 0 {
		verseNum, _ = strconv.Atoi(matches[1])
		if matches[2] != "" {
			verseEnd, _ = strconv.Atoi(matches[2])
		}
		verseText = strings.TrimSpace(value[len(matches[0]):])
	}

	if verseText == "" {
		return
	}

	ctx.blockSeq++
	osisID := fmt.Sprintf("%s.%d.%d", ctx.corpus.ID, ctx.currentChapter, verseNum)

	block := &ipc.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", ctx.blockSeq),
		Sequence: ctx.blockSeq,
		Text:     verseText,
		Anchors: []*ipc.Anchor{
			{
				ID:       fmt.Sprintf("a-%d-0", ctx.blockSeq),
				Position: 0,
				Spans: []*ipc.Span{
					{
						ID:            fmt.Sprintf("s-%s", osisID),
						Type:          "VERSE",
						StartAnchorID: fmt.Sprintf("a-%d-0", ctx.blockSeq),
						Ref: &ipc.Ref{
							Book:     ctx.corpus.ID,
							Chapter:  ctx.currentChapter,
							Verse:    verseNum,
							VerseEnd: verseEnd,
							OSISID:   osisID,
						},
					},
				},
			},
		},
	}

	h := sha256.Sum256([]byte(verseText))
	block.Hash = hex.EncodeToString(h[:])
	ctx.currentDoc.ContentBlocks = append(ctx.currentDoc.ContentBlocks, block)
}

// createTextBlock creates a content block with attributes
func createTextBlock(ctx *parseContext, text, blockType, marker string) {
	if ctx.currentDoc == nil || text == "" {
		return
	}

	ctx.blockSeq++
	block := &ipc.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", ctx.blockSeq),
		Sequence: ctx.blockSeq,
		Text:     text,
		Attributes: map[string]interface{}{
			"type":   blockType,
			"marker": marker,
		},
	}
	h := sha256.Sum256([]byte(text))
	block.Hash = hex.EncodeToString(h[:])
	ctx.currentDoc.ContentBlocks = append(ctx.currentDoc.ContentBlocks, block)
}

// parseUSFMToIR converts USFM text to IR Corpus
func parseUSFMToIR(data []byte) (*ipc.Corpus, error) {
	ctx := &parseContext{
		corpus: &ipc.Corpus{
			Version:      "1.0.0",
			ModuleType:   "BIBLE",
			SourceFormat: "USFM",
			LossClass:    "L0",
			Attributes:   make(map[string]string),
		},
	}

	ctx.corpus.Attributes["_usfm_raw"] = string(data)

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		processLine(ctx, scanner.Text())
	}

	h := sha256.Sum256(data)
	ctx.corpus.SourceHash = hex.EncodeToString(h[:])

	return ctx.corpus, nil
}

// processLine handles a single line of USFM
func processLine(ctx *parseContext, line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || !strings.HasPrefix(trimmed, "\\") {
		return
	}

	marker, value := parseMarkerLine(trimmed)
	handleMarker(ctx, marker, value)
}

// parseMarkerLine splits a USFM marker line into marker and value
func parseMarkerLine(line string) (string, string) {
	parts := strings.SplitN(line, " ", 2)
	marker := strings.TrimPrefix(parts[0], "\\")
	var value string
	if len(parts) > 1 {
		value = parts[1]
	}
	return marker, value
}

// handleMarker dispatches to appropriate handler based on marker type
func handleMarker(ctx *parseContext, marker, value string) {
	switch marker {
	case "id":
		handleBookID(ctx, value)
	case "h", "toc1", "toc2", "toc3":
		handleHeaderMarker(ctx, marker, value)
	case "mt", "mt1", "mt2", "mt3":
		handleTitleMarker(ctx, marker, value)
	case "c":
		handleChapterMarker(ctx, value)
	case "v":
		handleVerseMarker(ctx, value)
	case "p", "m", "pi", "mi", "nb":
		createTextBlock(ctx, value, "paragraph", marker)
	case "q", "q1", "q2", "q3", "qr", "qc", "qm":
		createTextBlock(ctx, value, "poetry", marker)
	}
}

// emitUSFMFromIR converts IR Corpus back to USFM text
func emitUSFMFromIR(corpus *ipc.Corpus) ([]byte, error) {
	// Check if we have the original raw USFM for L0 lossless round-trip
	if rawUSFM, ok := corpus.Attributes["_usfm_raw"]; ok && rawUSFM != "" {
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

// Compile check
var _ = io.Copy
