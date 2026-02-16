//go:build sdk

// Plugin format-txt handles plain text Bible format.
// Produces verse-per-line text output.
//
// IR Support:
// - extract-ir: Reads plain text Bible format to IR (L3)
// - emit-native: Converts IR to plain text format (L3)
// Note: L3 means text is preserved but all markup is lost.
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
	if err := format.Run(&format.Config{
		Name:       "TXT",
		Extensions: []string{".txt", ".text"},
		Detect:     detectFunc,
		Parse:      parseFunc,
		Emit:       emitFunc,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// detectFunc performs custom format detection for TXT files.
func detectFunc(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".txt" && ext != ".text" {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not a .txt file",
		}, nil
	}

	// Check for Bible-like content (verse references)
	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		}, nil
	}

	content := string(data)
	// Look for verse patterns like "Gen 1:1" or "1:1" at line starts
	versePattern := regexp.MustCompile(`(?m)^(\w+\s+)?(\d+):(\d+)\s+`)
	if versePattern.MatchString(content) {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "TXT",
			Reason:   "Plain text Bible format detected",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "no verse patterns found",
	}, nil
}

// parseFunc parses a TXT file and returns an IR Corpus.
func parseFunc(path string) (*ir.Corpus, error) {
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
		SourceFormat: "TXT",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L3",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip (L0 when raw available)
	corpus.Attributes["_txt_raw"] = string(data)

	// Parse text content
	corpus.Documents = parseTXTContent(string(data), artifactID)

	return corpus, nil
}

// parseTXTContent parses the text content into IR documents.
func parseTXTContent(content, artifactID string) []*ir.Document {
	doc := &ir.Document{
		ID:    artifactID,
		Title: artifactID,
		Order: 1,
	}

	// Parse verses: look for patterns like "Book C:V text" or "C:V text"
	// Examples: "Gen 1:1 In the beginning..." or "1:1 In the beginning..."
	versePattern := regexp.MustCompile(`^(\w+)?\s*(\d+):(\d+)\s+(.+)$`)

	scanner := bufio.NewScanner(strings.NewReader(content))
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

	// Update document ID if we found a book name
	if currentBook != artifactID {
		doc.ID = currentBook
		doc.Title = currentBook
	}

	return []*ir.Document{doc}
}

// emitFunc converts an IR Corpus to TXT format.
func emitFunc(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".txt")

	// Check for raw text for round-trip
	if raw, ok := corpus.Attributes["_txt_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
			return "", fmt.Errorf("failed to write text: %w", err)
		}
		return outputPath, nil
	}

	// Generate text from IR
	var buf strings.Builder

	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						buf.WriteString(fmt.Sprintf("%s %d:%d %s\n",
							span.Ref.Book,
							span.Ref.Chapter,
							span.Ref.Verse,
							cb.Text))
					}
				}
			}
		}
	}

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0600); err != nil {
		return "", fmt.Errorf("failed to write text: %w", err)
	}

	return outputPath, nil
}
