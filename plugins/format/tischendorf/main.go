//go:build !sdk

// Plugin format-tischendorf handles Tischendorf 8th edition critical apparatus format.
// Tischendorf's Novum Testamentum Graece (8th edition) is a historical critical text
// with extensive apparatus criticus documenting textual variants.
//
// IR Support:
// - extract-ir: Reads Tischendorf text to IR (L2)
// - emit-native: Converts IR to Tischendorf format (L2)
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ExtractIRResult is the result of an extract-ir command.

// EmitNativeResult is the result of an emit-native command.

// LossReport describes any data loss during conversion.

// LostElement describes a specific element that was lost.

func main() {
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorf("failed to read request: %v", err)
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
	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondErrorf("detect: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{Detected: false, Reason: "cannot read file"})
		return
	}

	// Detect Tischendorf format by checking for:
	// 1. Greek text markers
	// 2. Critical apparatus markers (brackets, parentheses)
	// 3. Verse references in format like "Matt 1:1" or just "1:1" at line start
	content := string(data)

	hasGreek := regexp.MustCompile(`[\p{Greek}]+`).MatchString(content)
	hasApparatus := strings.Contains(content, "[") || strings.Contains(content, "]")
	// Look for verse references with book names OR standalone verse refs at line start
	hasVerseRefWithBook := regexp.MustCompile(`(?i)(Matt|Mark|Luke|John|Rom|Cor|Gal|Eph|Phil|Col|Thess|Tim|Tit|Philem|Heb|James|Pet|Rev)\s+\d+:\d+`).MatchString(content)
	hasVerseRefStandalone := regexp.MustCompile(`(?m)^\d+:\d+\s`).MatchString(content)
	hasVerseRef := hasVerseRefWithBook || hasVerseRefStandalone

	if hasGreek && hasApparatus && hasVerseRef {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "tischendorf",
			Reason:   "detected Greek text with critical apparatus and verse references",
		})
		return
	}

	ipc.MustRespond(&ipc.DetectResult{Detected: false, Reason: "not a Tischendorf format file"})
}

func handleIngest(args map[string]interface{}) {
	path, outputDir, err := ipc.PathAndOutputDir(args)
	if err != nil {
		ipc.RespondErrorf("ingest: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("ingest: failed to read file: %v", err)
	}

	hashHex, err := ipc.StoreBlob(outputDir, data)
	if err != nil {
		ipc.RespondErrorf("ingest: %v", err)
	}

	artifactID := ipc.ArtifactIDFromPath(path)

	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format":   "tischendorf",
			"encoding": "utf-8",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondErrorf("enumerate: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		ipc.RespondErrorf("enumerate: failed to stat file: %v", err)
	}

	entries := []ipc.EnumerateEntry{
		{
			Path:      filepath.Base(path),
			SizeBytes: info.Size(),
			IsDir:     false,
			ModTime:   info.ModTime().Format("2006-01-02T15:04:05Z"),
		},
	}

	ipc.MustRespond(&ipc.EnumerateResult{Entries: entries})
}

func handleExtractIR(args map[string]interface{}) {
	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondErrorf("extract-ir: %v", err)
	}

	outputPath, err := ipc.StringArg(args, "output_path")
	if err != nil {
		ipc.RespondErrorf("extract-ir: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("extract-ir: failed to read file: %v", err)
	}

	corpus := parseTischendorfToIR(data)

	// Compute source hash
	hash := sha256.Sum256(data)
	corpus.SourceHash = hex.EncodeToString(hash[:])
	corpus.SourceFormat = "tischendorf"
	corpus.LossClass = "L2"

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		ipc.RespondErrorf("extract-ir: failed to marshal IR: %v", err)
	}

	if err := os.WriteFile(outputPath, irData, 0644); err != nil {
		ipc.RespondErrorf("extract-ir: failed to write IR file: %v", err)
	}

	lossReport := &ipc.LossReport{
		SourceFormat: "tischendorf",
		TargetFormat: "IR",
		LossClass:    "L2",
		Warnings: []string{
			"critical apparatus structure may be simplified",
			"manuscript sigla may be normalized",
		},
	}

	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:     outputPath,
		LossClass:  "L2",
		LossReport: lossReport,
	})
}

func parseTischendorfToIR(data []byte) *ipc.Corpus {
	corpus := &ipc.Corpus{
		ID:            "tischendorf-nt",
		Version:       "8.0",
		ModuleType:    "bible",
		Versification: "KJV",
		Language:      "grc",
		Title:         "Tischendorf Greek New Testament",
		Description:   "Critical edition with apparatus",
		Publisher:     "Constantin von Tischendorf",
		Attributes:    make(map[string]string),
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var currentDoc *ipc.Document
	var currentBlock *ipc.ContentBlock
	sequence := 0
	docOrder := 0

	// Parse line by line
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Check for book/chapter headers
		if isBookHeader(line) {
			bookName := extractBookName(line)
			currentDoc = &ipc.Document{
				ID:            bookName,
				Title:         bookName,
				Order:         docOrder,
				ContentBlocks: []*ipc.ContentBlock{},
				Attributes:    make(map[string]string),
			}
			corpus.Documents = append(corpus.Documents, currentDoc)
			docOrder++
			sequence = 0
			continue
		}

		// Parse verse content
		if currentDoc != nil {
			ref := extractReference(line)
			text := extractText(line)

			currentBlock = &ipc.ContentBlock{
				ID:       fmt.Sprintf("block_%d", sequence),
				Sequence: sequence,
				Text:     text,
				Anchors:  []*ipc.Anchor{},
				Attributes: map[string]interface{}{
					"verse_ref": ref,
				},
			}

			// Add verse span
			anchor := &ipc.Anchor{
				ID:       fmt.Sprintf("anchor_%d_0", sequence),
				Position: 0,
				Spans: []*ipc.Span{
					{
						ID:            fmt.Sprintf("span_%d_verse", sequence),
						Type:          "verse",
						StartAnchorID: fmt.Sprintf("anchor_%d_0", sequence),
						Ref:           parseRefString(ref),
					},
				},
			}
			currentBlock.Anchors = append(currentBlock.Anchors, anchor)

			currentDoc.ContentBlocks = append(currentDoc.ContentBlocks, currentBlock)
			sequence++
		}
	}

	return corpus
}

func isBookHeader(line string) bool {
	bookNames := []string{"Matthew", "Mark", "Luke", "John", "Acts", "Romans",
		"1 Corinthians", "2 Corinthians", "Galatians", "Ephesians", "Philippians",
		"Colossians", "1 Thessalonians", "2 Thessalonians", "1 Timothy", "2 Timothy",
		"Titus", "Philemon", "Hebrews", "James", "1 Peter", "2 Peter", "1 John",
		"2 John", "3 John", "Jude", "Revelation"}

	for _, name := range bookNames {
		if strings.Contains(line, name) {
			return true
		}
	}
	return false
}

func extractBookName(line string) string {
	// Extract book name from header
	parts := strings.Fields(line)
	if len(parts) > 0 {
		return parts[0]
	}
	return "Unknown"
}

func extractReference(line string) string {
	// Extract verse reference (e.g., "1:1", "2:3")
	re := regexp.MustCompile(`(\d+):(\d+)`)
	if match := re.FindString(line); match != "" {
		return match
	}
	return ""
}

func extractText(line string) string {
	// Remove reference markers and extract Greek text
	// Strip apparatus markers in brackets
	text := regexp.MustCompile(`\[.*?\]`).ReplaceAllString(line, "")
	text = regexp.MustCompile(`\d+:\d+`).ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

func parseRefString(refStr string) *ipc.Ref {
	// Parse "chapter:verse" format
	parts := strings.Split(refStr, ":")
	if len(parts) != 2 {
		return &ipc.Ref{}
	}

	chapter, _ := strconv.Atoi(parts[0])
	verse, _ := strconv.Atoi(parts[1])

	return &ipc.Ref{
		Chapter: chapter,
		Verse:   verse,
	}
}

func handleEmitNative(args map[string]interface{}) {
	irPath, err := ipc.StringArg(args, "ir_path")
	if err != nil {
		ipc.RespondErrorf("emit-native: %v", err)
	}

	outputPath, err := ipc.StringArg(args, "output_path")
	if err != nil {
		ipc.RespondErrorf("emit-native: %v", err)
	}

	irData, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("emit-native: failed to read IR file: %v", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		ipc.RespondErrorf("emit-native: failed to unmarshal IR: %v", err)
	}

	output := emitTischendorfFromIR(&corpus)

	if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
		ipc.RespondErrorf("emit-native: failed to write output: %v", err)
	}

	lossReport := &ipc.LossReport{
		SourceFormat: "IR",
		TargetFormat: "tischendorf",
		LossClass:    "L2",
		Warnings: []string{
			"complex critical apparatus may be simplified",
			"manuscript variants not fully preserved",
		},
	}

	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "tischendorf",
		LossClass:  "L2",
		LossReport: lossReport,
	})
}

func emitTischendorfFromIR(corpus *ipc.Corpus) string {
	var buf bytes.Buffer

	// Write header
	buf.WriteString(fmt.Sprintf("# %s\n", corpus.Title))
	buf.WriteString(fmt.Sprintf("# Version: %s\n", corpus.Version))
	buf.WriteString(fmt.Sprintf("# Language: %s\n\n", corpus.Language))

	for _, doc := range corpus.Documents {
		buf.WriteString(fmt.Sprintf("## %s\n\n", doc.Title))

		for _, block := range doc.ContentBlocks {
			// Extract verse reference from attributes
			verseRef := ""
			if v, ok := block.Attributes["verse_ref"]; ok {
				verseRef = fmt.Sprintf("%v", v)
			}

			// Write verse with reference
			if verseRef != "" {
				buf.WriteString(fmt.Sprintf("%s %s\n", verseRef, block.Text))
			} else {
				buf.WriteString(fmt.Sprintf("%s\n", block.Text))
			}
		}
		buf.WriteString("\n")
	}

	return buf.String()
}
