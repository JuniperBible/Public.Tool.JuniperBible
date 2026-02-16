//go:build sdk

// Plugin format-onlinebible handles Online Bible text format.
// Format uses $BookName Chapter:Verse Text pattern.
//
// IR Support:
// - extract-ir: Extracts IR from Online Bible text (L0 lossless via raw storage)
// - emit-native: Converts IR back to Online Bible format (L0)
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

var verseLineRegex = regexp.MustCompile(`^\$(\w+)\s+(\d+):(\d+)\s+(.*)$`)

func main() {
	if err := format.Run(&format.Config{
		Name:       "OnlineBible",
		Extensions: []string{".txt", ".obt"},
		Detect:     detectOnlineBible,
		Parse:      parseOnlineBible,
		Emit:       emitOnlineBible,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectOnlineBible(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{Detected: false, Reason: "path is a directory"}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot open: %v", err)}, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	matchCount := 0

	for scanner.Scan() && lineCount < 100 {
		line := scanner.Text()
		lineCount++

		if verseLineRegex.MatchString(line) {
			matchCount++
		}
	}

	if matchCount > 5 {
		return &ipc.DetectResult{Detected: true, Format: "OnlineBible", Reason: "Online Bible verse format detected"}, nil
	}

	return &ipc.DetectResult{Detected: false, Reason: "no Online Bible format pattern found"}, nil
}

func parseOnlineBible(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "OnlineBible"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L0"
	corpus.Attributes = map[string]string{"_onlinebible_raw": string(data)}

	// Parse verses
	bookDocs := make(map[string]*ir.Document)
	sequence := 0

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		matches := verseLineRegex.FindStringSubmatch(line)
		if len(matches) < 5 {
			continue
		}

		book := matches[1]
		chapter, _ := strconv.Atoi(matches[2])
		verse, _ := strconv.Atoi(matches[3])
		text := strings.TrimSpace(matches[4])

		if text == "" {
			continue
		}

		doc, ok := bookDocs[book]
		if !ok {
			doc = ir.NewDocument(book, book, len(bookDocs)+1)
			bookDocs[book] = doc
			corpus.Documents = append(corpus.Documents, doc)
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

	return corpus, nil
}

func emitOnlineBible(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".txt")

	// Check for raw content for L0 round-trip
	if raw, ok := corpus.Attributes["_onlinebible_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
			return "", fmt.Errorf("failed to write Online Bible file: %w", err)
		}
		return outputPath, nil
	}

	// Generate Online Bible format from IR
	var buf strings.Builder
	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						buf.WriteString(fmt.Sprintf("$%s %d:%d %s\n",
							span.Ref.Book, span.Ref.Chapter, span.Ref.Verse, cb.Text))
					}
				}
			}
		}
	}

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0600); err != nil {
		return "", fmt.Errorf("failed to write Online Bible file: %w", err)
	}

	return outputPath, nil
}
