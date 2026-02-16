// Package sfm provides the canonical SFM (Standard Format Markers/Paratext) format implementation.
package sfm

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

// Config defines the SFM format plugin configuration.
var Config = &format.Config{
	Name:       "SFM",
	Extensions: []string{".sfm", ".ptx"},
	Detect:     detectSFM,
	Parse:      parseSFM,
	Emit:       emitSFM,
}

func detectSFM(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".sfm" || ext == ".ptx" {
		return &ipc.DetectResult{Detected: true, Format: "SFM", Reason: "SFM extension"}, nil
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	sfmPattern := regexp.MustCompile(`(?m)^\\(id|c|v|p|s|h)\s`)
	if sfmPattern.MatchString(content) {
		return &ipc.DetectResult{Detected: true, Format: "SFM", Reason: "SFM markers detected"}, nil
	}
	return &ipc.DetectResult{Detected: false, Reason: "not SFM"}, nil
}

func parseSFM(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.Title = artifactID
	corpus.SourceFormat = "SFM"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L1"
	corpus.Attributes = map[string]string{"_format_raw": hex.EncodeToString(data)}

	corpus.Documents = extractSFMContent(string(data), artifactID)
	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ir.Document{ir.NewDocument(artifactID, artifactID, 1)}
	}

	return corpus, nil
}

func extractSFMContent(content, artifactID string) []*ir.Document {
	doc := ir.NewDocument(artifactID, artifactID, 1)
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
					Anchors: []*ir.Anchor{{
						ID:       fmt.Sprintf("a-%d", sequence),
						Position: 0,
						Spans: []*ir.Span{{
							ID:            fmt.Sprintf("s-%s", osisID),
							Type:          "VERSE",
							StartAnchorID: fmt.Sprintf("a-%d", sequence),
							Ref:           &ir.Ref{Book: artifactID, Chapter: chapter, Verse: verse, OSISID: osisID},
						}},
					}},
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

func emitSFM(corpus *ir.Corpus, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".sfm")

	if raw, ok := corpus.Attributes["_format_raw"]; ok && raw != "" {
		rawData, _ := hex.DecodeString(raw)
		if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
			return "", fmt.Errorf("failed to write SFM: %w", err)
		}
		return outputPath, nil
	}

	var buf bytes.Buffer
	for _, doc := range corpus.Documents {
		fmt.Fprintf(&buf, "\\id %s\n", doc.ID)
		lastChapter := 0
		for _, cb := range doc.ContentBlocks {
			chapter, verse := 1, cb.Sequence%1000
			if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 && cb.Anchors[0].Spans[0].Ref != nil {
				chapter = cb.Anchors[0].Spans[0].Ref.Chapter
				verse = cb.Anchors[0].Spans[0].Ref.Verse
			}
			if chapter != lastChapter {
				fmt.Fprintf(&buf, "\\c %d\n", chapter)
				lastChapter = chapter
			}
			fmt.Fprintf(&buf, "\\v %d %s\n", verse, cb.Text)
		}
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0600); err != nil {
		return "", fmt.Errorf("failed to write SFM: %w", err)
	}
	return outputPath, nil
}
