//go:build sdk

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
	if err := format.Run(&format.Config{
		Name:       "tischendorf",
		Extensions: []string{".txt"},
		Detect:     Detect,
		Parse:      Parse,
		Emit:       Emit,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Detect checks if the given path is a Tischendorf format file.
func Detect(path string) (*ipc.DetectResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: "cannot read file"}, nil
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
		return &ipc.DetectResult{
			Detected: true,
			Format:   "tischendorf",
			Reason:   "detected Greek text with critical apparatus and verse references",
		}, nil
	}

	return &ipc.DetectResult{Detected: false, Reason: "not a Tischendorf format file"}, nil
}

// Parse reads a Tischendorf format file and converts it to IR.
func Parse(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	corpus := parseTischendorfToIR(data)

	// Set IR metadata
	corpus.SourceFormat = "tischendorf"
	corpus.LossClass = "L2"

	return corpus, nil
}

// Emit converts an IR corpus to Tischendorf format and writes it to outputDir.
func Emit(corpus *ir.Corpus, outputDir string) (string, error) {
	output := emitTischendorfFromIR(corpus)

	// Create output file path
	outputPath := filepath.Join(outputDir, "output.txt")

	if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
		return "", fmt.Errorf("failed to write output: %w", err)
	}

	return outputPath, nil
}

// parseTischendorfToIR parses Tischendorf format data into an IR corpus.
func parseTischendorfToIR(data []byte) *ir.Corpus {
	corpus := &ir.Corpus{
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
	var currentDoc *ir.Document
	var currentBlock *ir.ContentBlock
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
			currentDoc = &ir.Document{
				ID:            bookName,
				Title:         bookName,
				Order:         docOrder,
				ContentBlocks: []*ir.ContentBlock{},
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

			currentBlock = &ir.ContentBlock{
				ID:       fmt.Sprintf("block_%d", sequence),
				Sequence: sequence,
				Text:     text,
				Anchors:  []*ir.Anchor{},
				Attributes: map[string]interface{}{
					"verse_ref": ref,
				},
			}

			// Add verse span
			anchor := &ir.Anchor{
				ID:       fmt.Sprintf("anchor_%d_0", sequence),
				Position: 0,
				Spans: []*ir.Span{
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

// isBookHeader checks if a line is a book header.
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

// extractBookName extracts the book name from a header line.
func extractBookName(line string) string {
	// Extract book name from header
	parts := strings.Fields(line)
	if len(parts) > 0 {
		return parts[0]
	}
	return "Unknown"
}

// extractReference extracts the verse reference from a line.
func extractReference(line string) string {
	// Extract verse reference (e.g., "1:1", "2:3")
	re := regexp.MustCompile(`(\d+):(\d+)`)
	if match := re.FindString(line); match != "" {
		return match
	}
	return ""
}

// extractText extracts the Greek text from a line, removing apparatus markers.
func extractText(line string) string {
	// Remove reference markers and extract Greek text
	// Strip apparatus markers in brackets
	text := regexp.MustCompile(`\[.*?\]`).ReplaceAllString(line, "")
	text = regexp.MustCompile(`\d+:\d+`).ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

// parseRefString parses a chapter:verse reference string into an IR Ref.
func parseRefString(refStr string) *ir.Ref {
	// Parse "chapter:verse" format
	parts := strings.Split(refStr, ":")
	if len(parts) != 2 {
		return &ir.Ref{}
	}

	chapter, _ := strconv.Atoi(parts[0])
	verse, _ := strconv.Atoi(parts[1])

	return &ir.Ref{
		Chapter: chapter,
		Verse:   verse,
	}
}

// emitTischendorfFromIR converts an IR corpus back to Tischendorf format.
func emitTischendorfFromIR(corpus *ir.Corpus) string {
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
