// Package tei provides the canonical TEI (Text Encoding Initiative) XML format implementation.
package tei

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// Config defines the TEI format plugin configuration.
var Config = &format.Config{
	PluginID:   "format.tei",
	Name:       "TEI",
	Extensions: []string{".tei", ".xml"},
	Detect:     detectTEI,
	Parse:      parseTEI,
	Emit:       emitTEI,
}

func detectTEI(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".tei" {
		return &ipc.DetectResult{Detected: true, Format: "TEI", Reason: "TEI file extension detected"}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: "not a TEI file"}, nil
	}

	content := string(data)
	if strings.Contains(content, "<TEI") && strings.Contains(content, "teiHeader") {
		return &ipc.DetectResult{Detected: true, Format: "TEI", Reason: "TEI XML structure detected"}, nil
	}

	if strings.Contains(content, "http://www.tei-c.org/ns/") {
		return &ipc.DetectResult{Detected: true, Format: "TEI", Reason: "TEI namespace detected"}, nil
	}

	return &ipc.DetectResult{Detected: false, Reason: "no TEI structure found"}, nil
}

func parseTEI(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "TEI"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L1"
	corpus.Attributes = make(map[string]string)

	// Store raw for round-trip
	corpus.Attributes["_format_raw"] = hex.EncodeToString(data)

	// Extract metadata and content from TEI XML
	content := string(data)
	extractTEIMetadata(content, corpus)
	corpus.Documents = extractTEIContent(content, artifactID)

	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ir.Document{ir.NewDocument(artifactID, corpus.Title, 1)}
	}

	return corpus, nil
}

func extractTEIMetadata(content string, corpus *ir.Corpus) {
	titlePattern := regexp.MustCompile(`<title[^>]*>([^<]+)</title>`)
	if matches := titlePattern.FindStringSubmatch(content); len(matches) > 1 {
		corpus.Title = strings.TrimSpace(matches[1])
	}

	langPattern := regexp.MustCompile(`<language[^>]*ident="([^"]+)"`)
	if matches := langPattern.FindStringSubmatch(content); len(matches) > 1 {
		corpus.Language = matches[1]
	}

	pubPattern := regexp.MustCompile(`<publisher>([^<]+)</publisher>`)
	if matches := pubPattern.FindStringSubmatch(content); len(matches) > 1 {
		corpus.Publisher = strings.TrimSpace(matches[1])
	}
}

func extractTEIContent(content, artifactID string) []*ir.Document {
	sequence := 0
	documents := extractStructuredDocuments(content, &sequence)
	if len(documents) > 0 {
		return documents
	}
	return extractFallbackDocument(content, artifactID, &sequence)
}

// newContentBlock builds a ContentBlock for the given text and sequence counter.
// It returns nil when text is empty or shorter than minLen, leaving sequence unchanged.
func newContentBlock(seq *int, text string, minLen int) *ir.ContentBlock {
	text = strings.TrimSpace(text)
	if len(text) <= minLen {
		return nil
	}
	*seq++
	hash := sha256.Sum256([]byte(text))
	return &ir.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", *seq),
		Sequence: *seq,
		Text:     text,
		Hash:     hex.EncodeToString(hash[:]),
	}
}

// appendBlocksFromMatches appends a ContentBlock to doc for every regex match
// whose captured text (index captureIdx) is longer than minLen characters.
func appendBlocksFromMatches(doc *ir.Document, matches [][]string, captureIdx int, seq *int, minLen int) {
	for _, m := range matches {
		if len(m) <= captureIdx {
			continue
		}
		if cb := newContentBlock(seq, m[captureIdx], minLen); cb != nil {
			doc.ContentBlocks = append(doc.ContentBlocks, cb)
		}
	}
}

// extractStructuredDocuments parses <div type="book|chapter"> elements and
// returns one Document per qualifying div, or nil when none are found.
func extractStructuredDocuments(content string, seq *int) []*ir.Document {
	divPattern := regexp.MustCompile(`<div[^>]*type="([^"]*)"[^>]*n="([^"]*)"[^>]*>([\s\S]*?)</div>`)
	segPattern := regexp.MustCompile(`<seg[^>]*n="([^"]*)"[^>]*>([^<]*)</seg>`)
	versePattern := regexp.MustCompile(`<ab[^>]*n="([^"]*)"[^>]*>([^<]*)</ab>`)

	var documents []*ir.Document
	docOrder := 0

	for _, match := range divPattern.FindAllStringSubmatch(content, -1) {
		if len(match) < 4 {
			continue
		}
		divType, divN, divContent := match[1], match[2], match[3]
		if divType != "book" && divType != "chapter" {
			continue
		}

		docOrder++
		doc := ir.NewDocument(divN, divN, docOrder)
		appendBlocksFromMatches(doc, segPattern.FindAllStringSubmatch(divContent, -1), 2, seq, 0)
		appendBlocksFromMatches(doc, versePattern.FindAllStringSubmatch(divContent, -1), 2, seq, 0)

		if len(doc.ContentBlocks) > 0 {
			documents = append(documents, doc)
		}
	}

	return documents
}

// extractFallbackDocument builds a single Document from <p> elements when no
// structured divs were found. Returns an empty slice when no paragraphs exist.
func extractFallbackDocument(content, artifactID string, seq *int) []*ir.Document {
	doc := ir.NewDocument(artifactID, artifactID, 1)
	pPattern := regexp.MustCompile(`<p[^>]*>([^<]+)</p>`)
	appendBlocksFromMatches(doc, pPattern.FindAllStringSubmatch(content, -1), 1, seq, 5)
	if len(doc.ContentBlocks) == 0 {
		return nil
	}
	return []*ir.Document{doc}
}

func emitTEI(corpus *ir.Corpus, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".tei.xml")

	// Check for raw TEI for round-trip
	if raw, ok := corpus.Attributes["_format_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
				return "", fmt.Errorf("failed to write TEI: %w", err)
			}
			return outputPath, nil
		}
	}

	// Generate TEI XML from IR
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString("\n")
	buf.WriteString(`<TEI xmlns="http://www.tei-c.org/ns/1.0">`)
	buf.WriteString("\n")
	buf.WriteString("  <teiHeader>\n")
	buf.WriteString("    <fileDesc>\n")
	buf.WriteString("      <titleStmt>\n")
	fmt.Fprintf(&buf, "        <title>%s</title>\n", escapeXML(corpus.Title))
	buf.WriteString("      </titleStmt>\n")
	buf.WriteString("      <publicationStmt>\n")
	if corpus.Publisher != "" {
		fmt.Fprintf(&buf, "        <publisher>%s</publisher>\n", escapeXML(corpus.Publisher))
	} else {
		buf.WriteString("        <p>Generated from IR</p>\n")
	}
	buf.WriteString("      </publicationStmt>\n")
	buf.WriteString("      <sourceDesc>\n")
	buf.WriteString("        <p>Converted from Capsule IR</p>\n")
	buf.WriteString("      </sourceDesc>\n")
	buf.WriteString("    </fileDesc>\n")
	if corpus.Language != "" {
		buf.WriteString("    <profileDesc>\n")
		buf.WriteString("      <langUsage>\n")
		fmt.Fprintf(&buf, "        <language ident=\"%s\"/>\n", corpus.Language)
		buf.WriteString("      </langUsage>\n")
		buf.WriteString("    </profileDesc>\n")
	}
	buf.WriteString("  </teiHeader>\n")
	buf.WriteString("  <text>\n")
	buf.WriteString("    <body>\n")

	for _, doc := range corpus.Documents {
		fmt.Fprintf(&buf, "      <div type=\"book\" n=\"%s\">\n", escapeXML(doc.ID))
		for _, cb := range doc.ContentBlocks {
			fmt.Fprintf(&buf, "        <ab n=\"%d\">%s</ab>\n", cb.Sequence, escapeXML(cb.Text))
		}
		buf.WriteString("      </div>\n")
	}

	buf.WriteString("    </body>\n")
	buf.WriteString("  </text>\n")
	buf.WriteString("</TEI>\n")

	if err := os.WriteFile(outputPath, buf.Bytes(), 0600); err != nil {
		return "", fmt.Errorf("failed to write TEI: %w", err)
	}

	return outputPath, nil
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
