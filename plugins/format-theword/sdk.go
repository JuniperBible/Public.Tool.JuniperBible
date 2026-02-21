// Plugin format-theword handles TheWord/PalmBible+ Bible file ingestion.
// TheWord format uses line-based text files where each line is a verse.
// Supported extensions: .ont (OT), .nt (NT), .twm (full module)
//
// IR Support:
// - extract-ir: Extracts IR from TheWord text (L0 lossless via raw storage)
// - emit-native: Converts IR back to TheWord format (L0)
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/format"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/ir"
)

// Simplified OT books for demo (full implementation would have all 66)
var otBooks = []string{"Gen", "Exod", "Lev", "Num", "Deut", "Josh", "Judg", "Ruth", "1Sam", "2Sam", "1Kgs", "2Kgs", "1Chr", "2Chr", "Ezra", "Neh", "Esth", "Job", "Ps", "Prov", "Eccl", "Song", "Isa", "Jer", "Lam", "Ezek", "Dan", "Hos", "Joel", "Amos", "Obad", "Jonah", "Mic", "Nah", "Hab", "Zeph", "Hag", "Zech", "Mal"}
var ntBooks = []string{"Matt", "Mark", "Luke", "John", "Acts", "Rom", "1Cor", "2Cor", "Gal", "Eph", "Phil", "Col", "1Thess", "2Thess", "1Tim", "2Tim", "Titus", "Phlm", "Heb", "Jas", "1Pet", "2Pet", "1John", "2John", "3John", "Jude", "Rev"}

func runSDK() {
	if err := format.Run(&format.Config{
		Name:       "TheWord",
		Extensions: []string{".ont", ".nt", ".twm"},
		Detect:     detectTheWord,
		Parse:      parseTheWord,
		Emit:       emitTheWord,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectTheWord(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{Detected: false, Reason: "path is a directory"}, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	validExts := map[string]string{
		".ont": "Old Testament",
		".nt":  "New Testament",
		".twm": "Full Module",
	}

	moduleType, isValid := validExts[ext]
	if !isValid {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("extension %s is not a known TheWord format", ext)}, nil
	}

	// Check if file has reasonable line count for a Bible
	file, err := os.Open(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot open file: %v", err)}, nil
	}
	defer file.Close()

	lineCount := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() && lineCount < 100 {
		lineCount++
	}

	if lineCount < 10 {
		return &ipc.DetectResult{Detected: false, Reason: "file has too few lines for TheWord format"}, nil
	}

	return &ipc.DetectResult{Detected: true, Format: "TheWord", Reason: fmt.Sprintf("TheWord %s detected", moduleType)}, nil
}

func parseTheWord(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ext := strings.ToLower(filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "TheWord"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L0"
	corpus.Attributes = map[string]string{
		"_theword_raw": string(data),
		"_theword_ext": ext,
	}

	// Parse lines into verses
	lines := strings.Split(string(data), "\n")

	// Determine book list based on extension
	var books []string
	switch ext {
	case ".ont":
		books = otBooks
	case ".nt":
		books = ntBooks
	default:
		books = append(otBooks, ntBooks...)
	}

	// Simple parsing: each line is a verse
	sequence := 0
	bookDocs := make(map[string]*ir.Document)
	lineIdx := 0

	for _, book := range books {
		if lineIdx >= len(lines) {
			break
		}

		doc, ok := bookDocs[book]
		if !ok {
			doc = ir.NewDocument(book, book, len(bookDocs)+1)
			bookDocs[book] = doc
			corpus.Documents = append(corpus.Documents, doc)
		}

		// Simple: assign lines sequentially (real impl would use verse counts)
		for chapter := 1; chapter <= 3 && lineIdx < len(lines); chapter++ {
			for verse := 1; verse <= 10 && lineIdx < len(lines); verse++ {
				text := strings.TrimSpace(lines[lineIdx])
				lineIdx++

				if text == "" {
					continue
				}

				sequence++
				hash := sha256.Sum256([]byte(text))
				osisID := fmt.Sprintf("%s.%d.%d", book, chapter, verse)

				cb := &ir.ContentBlock{
					ID:       fmt.Sprintf("cb-%d", sequence),
					Sequence: sequence,
					Text:     text,
					Hash:     hex.EncodeToString(hash[:]),
					Anchors: []*ir.Anchor{{
						ID:       fmt.Sprintf("a-%d-0", sequence),
						Position: 0,
						Spans: []*ir.Span{{
							ID:            fmt.Sprintf("s-%s", osisID),
							Type:          "VERSE",
							StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
							Ref:           &ir.Ref{Book: book, Chapter: chapter, Verse: verse, OSISID: osisID},
						}},
					}},
				}
				doc.ContentBlocks = append(doc.ContentBlocks, cb)
			}
		}
	}

	return corpus, nil
}

func emitTheWord(corpus *ir.Corpus, outputDir string) (string, error) {
	// Determine output extension
	ext := ".twm"
	if e, ok := corpus.Attributes["_theword_ext"]; ok {
		ext = e
	}

	outputPath := filepath.Join(outputDir, corpus.ID+ext)

	// Check for raw content for L0 round-trip
	if raw, ok := corpus.Attributes["_theword_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
			return "", fmt.Errorf("failed to write TheWord file: %w", err)
		}
		return outputPath, nil
	}

	// Generate TheWord format from IR
	var buf strings.Builder
	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			buf.WriteString(cb.Text)
			buf.WriteString("\n")
		}
	}

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0600); err != nil {
		return "", fmt.Errorf("failed to write TheWord file: %w", err)
	}

	return outputPath, nil
}
