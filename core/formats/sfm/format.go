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

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
	"github.com/JuniperBible/juniper/plugins/sdk/ir"
)

// Config defines the SFM format plugin configuration.
var Config = &format.Config{
	PluginID:   "format.sfm",
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

func buildContentBlock(artifactID string, chapter, verse, sequence int, text string) *ir.ContentBlock {
	hash := sha256.Sum256([]byte(text))
	osisID := fmt.Sprintf("%s.%d.%d", artifactID, chapter, verse)
	return &ir.ContentBlock{
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
}

func parseChapterLine(line string) int {
	c, _ := strconv.Atoi(strings.TrimSpace(line[3:]))
	return c
}

func parseVerseLine(line string) (int, string) {
	parts := strings.SplitN(line[3:], " ", 2)
	v, _ := strconv.Atoi(parts[0])
	if v <= 0 {
		return 0, ""
	}
	if len(parts) > 1 {
		return v, parts[1]
	}
	return v, ""
}

func appendContinuationText(sb *strings.Builder, line string, verse int) {
	if verse <= 0 {
		return
	}
	if sb.Len() > 0 {
		sb.WriteString(" ")
	}
	sb.WriteString(strings.TrimSpace(line))
}

type sfmParser struct {
	doc       *ir.Document
	artifactID string
	chapter   int
	verse     int
	sequence  int
	verseText strings.Builder
}

func extractSFMContent(content, artifactID string) []*ir.Document {
	p := &sfmParser{
		doc:        ir.NewDocument(artifactID, artifactID, 1),
		artifactID: artifactID,
		chapter:    1,
	}
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		p.processLine(scanner.Text())
	}
	p.flushVerse()
	return []*ir.Document{p.doc}
}

func (p *sfmParser) processLine(line string) {
	switch {
	case strings.HasPrefix(line, "\\c "):
		p.handleChapter(line)
	case strings.HasPrefix(line, "\\v "):
		p.handleVerse(line)
	case !strings.HasPrefix(line, "\\"):
		appendContinuationText(&p.verseText, line, p.verse)
	}
}

func (p *sfmParser) handleChapter(line string) {
	p.flushVerse()
	if c := parseChapterLine(line); c > 0 {
		p.chapter = c
	}
}

func (p *sfmParser) handleVerse(line string) {
	p.flushVerse()
	if v, inline := parseVerseLine(line); v > 0 {
		p.verse = v
		p.verseText.WriteString(inline)
	}
}

func (p *sfmParser) flushVerse() {
	if p.verseText.Len() == 0 || p.verse <= 0 {
		return
	}
	text := strings.TrimSpace(p.verseText.String())
	p.verseText.Reset()
	if text == "" {
		return
	}
	p.sequence++
	p.doc.ContentBlocks = append(p.doc.ContentBlocks, buildContentBlock(p.artifactID, p.chapter, p.verse, p.sequence, text))
}

func extractChapterVerse(cb *ir.ContentBlock) (int, int) {
	if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 && cb.Anchors[0].Spans[0].Ref != nil {
		ref := cb.Anchors[0].Spans[0].Ref
		return ref.Chapter, ref.Verse
	}
	return 1, cb.Sequence % 1000
}

func formatSFMDoc(doc *ir.Document) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\\id %s\n", doc.ID)
	lastChapter := 0
	for _, cb := range doc.ContentBlocks {
		chapter, verse := extractChapterVerse(cb)
		if chapter != lastChapter {
			fmt.Fprintf(&buf, "\\c %d\n", chapter)
			lastChapter = chapter
		}
		fmt.Fprintf(&buf, "\\v %d %s\n", verse, cb.Text)
	}
	return buf.String()
}

func writeSFMFile(outputPath string, corpus *ir.Corpus) error {
	if raw, ok := corpus.Attributes["_format_raw"]; ok && raw != "" {
		rawData, _ := hex.DecodeString(raw)
		return os.WriteFile(outputPath, rawData, 0600)
	}
	var buf bytes.Buffer
	for _, doc := range corpus.Documents {
		buf.WriteString(formatSFMDoc(doc))
	}
	return os.WriteFile(outputPath, buf.Bytes(), 0600)
}

func emitSFM(corpus *ir.Corpus, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}
	outputPath := filepath.Join(outputDir, corpus.ID+".sfm")
	if err := writeSFMFile(outputPath, corpus); err != nil {
		return "", fmt.Errorf("failed to write SFM: %w", err)
	}
	return outputPath, nil
}
