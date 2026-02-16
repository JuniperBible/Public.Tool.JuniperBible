//go:build sdk

// Plugin format-sfm handles Standard Format Markers (SFM/Paratext) format.
// SFM is a text-based markup format used by Paratext and SIL translation tools,
// using backslash markers for structure and formatting.
// This version uses the SDK pattern.
//
// IR Support:
// - Parse: Reads SFM to IR (L1)
// - Emit: Converts IR to SFM format (L1 or L0 with raw storage)
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

// SFM marker pattern
var sfmPattern = regexp.MustCompile(`(?m)^\\(id|c|v|p|s|h)\s`)

func main() {
	if err := format.Run(&format.Config{
		Name:       "SFM",
		Extensions: []string{".sfm", ".ptx"},
		Detect:     detectSFM,
		Parse:      parseSFM,
		Emit:       emitSFM,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// detectSFM performs custom SFM format detection
func detectSFM(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "path is a directory, not a file",
		}, nil
	}

	// Check file extension first
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".sfm" || ext == ".ptx" {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "SFM",
			Reason:   "SFM extension",
		}, nil
	}

	// Read file and check for SFM markers
	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read: %v", err),
		}, nil
	}

	content := string(data)

	// SFM uses backslash markers like \id, \c, \v
	if sfmPattern.MatchString(content) {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "SFM",
			Reason:   "SFM markers detected",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "not SFM",
	}, nil
}

// parseSFM parses an SFM file and returns an IR Corpus
func parseSFM(path string) (*ir.Corpus, error) {
	// Read SFM file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Get artifact ID from filename
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	// Parse SFM to IR
	corpus, err := parseSFMToIR(data, artifactID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SFM: %w", err)
	}

	return corpus, nil
}

// emitSFM converts an IR Corpus to SFM format
func emitSFM(corpus *ir.Corpus, outputDir string) (string, error) {
	// Convert IR to SFM
	sfmData, err := emitSFMFromIR(corpus)
	if err != nil {
		return "", fmt.Errorf("failed to emit SFM: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write SFM to output directory
	outputPath := filepath.Join(outputDir, corpus.ID+".sfm")
	if err := os.WriteFile(outputPath, sfmData, 0600); err != nil {
		return "", fmt.Errorf("failed to write SFM: %w", err)
	}

	return outputPath, nil
}

// parseSFMToIR converts SFM text to IR Corpus
func parseSFMToIR(data []byte, artifactID string) (*ir.Corpus, error) {
	content := string(data)

	// Compute source hash
	sourceHash := sha256.Sum256(data)

	corpus := &ir.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		Title:        artifactID,
		SourceFormat: "SFM",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L1",
		Attributes:   make(map[string]string),
	}

	// Store raw content for L0 round-trip
	corpus.Attributes["_sfm_raw"] = hex.EncodeToString(data)

	// Extract SFM content
	documents := extractSFMContent(content, artifactID)
	if len(documents) == 0 {
		// Create empty document if no content found
		documents = []*ir.Document{
			{
				ID:    artifactID,
				Title: artifactID,
				Order: 1,
			},
		}
	}

	corpus.Documents = documents

	return corpus, nil
}

// extractSFMContent extracts content from SFM text
func extractSFMContent(content, artifactID string) []*ir.Document {
	doc := &ir.Document{
		ID:    artifactID,
		Title: artifactID,
		Order: 1,
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	chapter, verse, sequence := 1, 0, 0
	var verseText strings.Builder

	flushVerse := func() {
		if verseText.Len() > 0 && verse > 0 {
			text := strings.TrimSpace(verseText.String())
			if text != "" {
				sequence++
				hash := sha256.Sum256([]byte(text))
				osisID := fmt.Sprintf("%s.%d.%d", artifactID, chapter, verse)

				cb := &ir.ContentBlock{
					ID:       fmt.Sprintf("cb-%d", sequence),
					Sequence: chapter*1000 + verse,
					Text:     text,
					Hash:     hex.EncodeToString(hash[:]),
					Anchors: []*ir.Anchor{
						{
							ID:       fmt.Sprintf("a-%d", sequence),
							Position: 0,
							Spans: []*ir.Span{
								{
									ID:            fmt.Sprintf("s-%s", osisID),
									Type:          "VERSE",
									StartAnchorID: fmt.Sprintf("a-%d", sequence),
									Ref: &ir.Ref{
										Book:    artifactID,
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
			verseText.Reset()
		}
	}

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "\\c ") {
			flushVerse()
			c, _ := strconv.Atoi(strings.TrimSpace(line[3:]))
			if c > 0 {
				chapter = c
			}
		} else if strings.HasPrefix(line, "\\v ") {
			flushVerse()
			parts := strings.SplitN(line[3:], " ", 2)
			v, _ := strconv.Atoi(parts[0])
			if v > 0 {
				verse = v
				if len(parts) > 1 {
					verseText.WriteString(parts[1])
				}
			}
		} else if !strings.HasPrefix(line, "\\") {
			if verse > 0 {
				if verseText.Len() > 0 {
					verseText.WriteString(" ")
				}
				verseText.WriteString(strings.TrimSpace(line))
			}
		}
	}

	flushVerse()

	return []*ir.Document{doc}
}

// emitSFMFromIR converts IR Corpus back to SFM text
func emitSFMFromIR(corpus *ir.Corpus) ([]byte, error) {
	// Check if we have the original raw SFM for L0 lossless round-trip
	if rawHex, ok := corpus.Attributes["_sfm_raw"]; ok && rawHex != "" {
		rawData, err := hex.DecodeString(rawHex)
		if err == nil {
			return rawData, nil
		}
	}

	// Otherwise, reconstruct SFM from IR structure
	var buf bytes.Buffer

	for _, doc := range corpus.Documents {
		fmt.Fprintf(&buf, "\\id %s\n", doc.ID)

		lastChapter := 0

		for _, cb := range doc.ContentBlocks {
			chapter, verse := 1, cb.Sequence%1000

			// Extract chapter and verse from anchors/spans if available
			if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 && cb.Anchors[0].Spans[0].Ref != nil {
				chapter = cb.Anchors[0].Spans[0].Ref.Chapter
				verse = cb.Anchors[0].Spans[0].Ref.Verse
			}

			// Write chapter marker if changed
			if chapter != lastChapter {
				fmt.Fprintf(&buf, "\\c %d\n", chapter)
				lastChapter = chapter
			}

			// Write verse
			fmt.Fprintf(&buf, "\\v %d %s\n", verse, cb.Text)
		}
	}

	return buf.Bytes(), nil
}
