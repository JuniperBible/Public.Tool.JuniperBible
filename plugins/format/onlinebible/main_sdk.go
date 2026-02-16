// Plugin format-onlinebible handles OnlineBible .ont format using the SDK pattern.
// OnlineBible uses a text-based format with structured verse references.
//
// IR Support:
// - extract-ir: Reads OnlineBible format to IR (L2)
// - emit-native: Converts IR to OnlineBible-compatible format (L2)
package main

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

func main() {
	format.Run(
		detect,
		parse,
		emit,
	)
}

// detect determines if the file is in OnlineBible format
func detect(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	// OnlineBible uses .ont extension
	if ext == ".ont" {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "OnlineBible",
			Reason:   "OnlineBible file extension detected",
		}, nil
	}

	// Check for OnlineBible content structure
	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not an OnlineBible file",
		}, nil
	}

	content := string(data)
	// OnlineBible format typically has verse references like "Gen 1:1" or "$Gen 1:1"
	versePattern := regexp.MustCompile(`(?m)^\$?[A-Z][a-z]{1,2}\s+\d+:\d+`)
	if versePattern.MatchString(content) {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "OnlineBible",
			Reason:   "OnlineBible verse reference pattern detected",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "no OnlineBible structure found",
	}, nil
}

// parse reads OnlineBible format and converts to IR
func parse(path string) (*ir.Corpus, error) {
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
		SourceFormat: "OnlineBible",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L2",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_onlinebible_raw"] = hex.EncodeToString(data)

	// Extract content from OnlineBible format
	corpus.Documents = extractOnlineBibleContent(string(data), artifactID)

	// If no documents extracted, create minimal structure
	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ir.Document{
			{
				ID:    artifactID,
				Title: artifactID,
				Order: 1,
			},
		}
	}

	return corpus, nil
}

// extractOnlineBibleContent parses OnlineBible text format into IR documents
func extractOnlineBibleContent(content, artifactID string) []*ir.Document {
	// Group verses by book
	books := make(map[string][]*ir.ContentBlock)
	bookOrder := make(map[string]int)

	// Parse verse references: "$Book Chapter:Verse Text" or "Book Chapter:Verse Text"
	versePattern := regexp.MustCompile(`(?m)^\$?([A-Z][a-z]{1,2}(?:\s+[A-Z][a-z]+)?)\s+(\d+):(\d+)\s+(.*)$`)

	scanner := bufio.NewScanner(strings.NewReader(content))
	sequence := 0
	orderCounter := 0

	for scanner.Scan() {
		line := scanner.Text()
		matches := versePattern.FindStringSubmatch(line)
		if len(matches) == 5 {
			book := matches[1]
			chapter, _ := strconv.Atoi(matches[2])
			verse, _ := strconv.Atoi(matches[3])
			text := matches[4]

			if _, exists := bookOrder[book]; !exists {
				orderCounter++
				bookOrder[book] = orderCounter
			}

			sequence++
			hash := sha256.Sum256([]byte(text))
			osisID := fmt.Sprintf("%s.%d.%d", book, chapter, verse)

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
									Book:    book,
									Chapter: chapter,
									Verse:   verse,
									OSISID:  osisID,
								},
							},
						},
					},
				},
			}

			books[book] = append(books[book], cb)
		}
	}

	// Convert to documents
	var documents []*ir.Document
	for book, blocks := range books {
		doc := &ir.Document{
			ID:            book,
			Title:         book,
			Order:         bookOrder[book],
			ContentBlocks: blocks,
		}
		documents = append(documents, doc)
	}

	// Sort by order
	for i := 0; i < len(documents); i++ {
		for j := i + 1; j < len(documents); j++ {
			if documents[i].Order > documents[j].Order {
				documents[i], documents[j] = documents[j], documents[i]
			}
		}
	}

	// If no verses found, try simpler parsing
	if len(documents) == 0 {
		doc := &ir.Document{
			ID:    artifactID,
			Title: artifactID,
			Order: 1,
		}

		lines := strings.Split(content, "\n")
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if len(line) > 5 {
				hash := sha256.Sum256([]byte(line))
				doc.ContentBlocks = append(doc.ContentBlocks, &ir.ContentBlock{
					ID:       fmt.Sprintf("cb-%d", i+1),
					Sequence: i + 1,
					Text:     line,
					Hash:     hex.EncodeToString(hash[:]),
				})
			}
		}

		documents = []*ir.Document{doc}
	}

	return documents
}

// emit converts IR back to OnlineBible format
func emit(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".ont")

	// Check for raw OnlineBible for round-trip
	if raw, ok := corpus.Attributes["_onlinebible_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0644); err != nil {
				return "", fmt.Errorf("failed to write OnlineBible: %w", err)
			}
			return outputPath, nil
		}
	}

	// Generate OnlineBible format from IR
	var buf bytes.Buffer

	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			book := doc.ID
			chapter := 1
			verse := cb.Sequence

			if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 {
				if ref := cb.Anchors[0].Spans[0].Ref; ref != nil {
					book = ref.Book
					chapter = ref.Chapter
					verse = ref.Verse
				}
			}

			fmt.Fprintf(&buf, "$%s %d:%d %s\n", book, chapter, verse, cb.Text)
		}
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write OnlineBible: %w", err)
	}

	return outputPath, nil
}
