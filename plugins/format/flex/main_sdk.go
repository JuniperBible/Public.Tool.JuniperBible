//go:build sdk

// Plugin format-flex handles FLEx/Fieldworks linguistic database format.
// FLEx (FieldWorks Language Explorer) is a linguistic database for language documentation
// and analysis, containing lexical, grammatical, and text corpus data.
//
// IR Support:
// - extract-ir: Reads FLEx format to IR (L2)
// - emit-native: Converts IR to FLEx format (L2 or L0 with raw storage)
package main

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

func main() {
	format.Run(&format.Config{
		Name:       "FLEx",
		Extensions: []string{".flextext", ".fwbackup", ".fwdata"},
		Detect:     detectFLEx,
		Parse:      parseFLEx,
		Emit:       emitFLEx,
	})
}

// detectFLEx performs custom format detection for FLEx files.
func detectFLEx(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".flextext" || ext == ".fwbackup" || ext == ".fwdata" {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "FLEx",
			Reason:   "FLEx extension",
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "failed to read file",
		}, nil
	}

	content := string(data)
	// FLEx XML patterns
	flexPattern := regexp.MustCompile(`<(document|interlinear-text|paragraphs|phrase)[^>]*>`)
	if flexPattern.MatchString(content) && (strings.Contains(content, "flextext") || strings.Contains(content, "FieldWorks")) {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "FLEx",
			Reason:   "FLEx XML structure",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "not FLEx",
	}, nil
}

// parseFLEx parses a FLEx file and returns an IR Corpus.
func parseFLEx(path string) (*ir.Corpus, error) {
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
		Title:        artifactID,
		SourceFormat: "FLEx",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L2",
		Attributes:   map[string]string{"_flex_raw": hex.EncodeToString(data)},
	}

	corpus.Documents = extractFLExContent(string(data), artifactID)
	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ir.Document{{
			ID:    artifactID,
			Title: artifactID,
			Order: 1,
		}}
	}

	return corpus, nil
}

// extractFLExContent extracts content from FLEx XML format.
func extractFLExContent(content, artifactID string) []*ir.Document {
	doc := &ir.Document{
		ID:    artifactID,
		Title: artifactID,
		Order: 1,
	}

	// Extract text from phrase elements
	phrasePattern := regexp.MustCompile(`<phrase[^>]*>([\s\S]*?)</phrase>`)
	wordPattern := regexp.MustCompile(`<item[^>]*type="txt"[^>]*>([^<]+)</item>`)

	sequence := 0
	for _, phraseMatch := range phrasePattern.FindAllStringSubmatch(content, -1) {
		if len(phraseMatch) < 2 {
			continue
		}
		phraseContent := phraseMatch[1]

		var words []string
		for _, wordMatch := range wordPattern.FindAllStringSubmatch(phraseContent, -1) {
			if len(wordMatch) >= 2 {
				words = append(words, strings.TrimSpace(wordMatch[1]))
			}
		}

		if len(words) > 0 {
			text := strings.Join(words, " ")
			sequence++
			hash := sha256.Sum256([]byte(text))
			cb := &ir.ContentBlock{
				ID:       fmt.Sprintf("cb-%d", sequence),
				Sequence: sequence,
				Text:     text,
				Hash:     hex.EncodeToString(hash[:]),
			}
			doc.ContentBlocks = append(doc.ContentBlocks, cb)
		}
	}

	// If no phrases, try simpler extraction
	if len(doc.ContentBlocks) == 0 {
		txtPattern := regexp.MustCompile(`<item[^>]*type="txt"[^>]*>([^<]+)</item>`)
		for _, match := range txtPattern.FindAllStringSubmatch(content, -1) {
			if len(match) >= 2 {
				text := strings.TrimSpace(match[1])
				if len(text) > 3 {
					sequence++
					hash := sha256.Sum256([]byte(text))
					doc.ContentBlocks = append(doc.ContentBlocks, &ir.ContentBlock{
						ID:       fmt.Sprintf("cb-%d", sequence),
						Sequence: sequence,
						Text:     text,
						Hash:     hex.EncodeToString(hash[:]),
					})
				}
			}
		}
	}

	return []*ir.Document{doc}
}

// emitFLEx converts an IR Corpus to FLEx format.
func emitFLEx(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".flextext")

	// If we have the raw FLEx data stored, use it for lossless round-trip (L0)
	if raw, ok := corpus.Attributes["_flex_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err != nil {
			return "", fmt.Errorf("failed to decode raw FLEx data: %w", err)
		}
		if err := os.WriteFile(outputPath, rawData, 0644); err != nil {
			return "", fmt.Errorf("failed to write output file: %w", err)
		}
		return outputPath, nil
	}

	// Otherwise, reconstruct FLEx XML from IR (L2)
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString("\n<document>\n")
	buf.WriteString("  <interlinear-text>\n")
	buf.WriteString("    <paragraphs>\n")

	for _, doc := range corpus.Documents {
		buf.WriteString("      <paragraph>\n")
		buf.WriteString("        <phrases>\n")
		for _, cb := range doc.ContentBlocks {
			buf.WriteString("          <phrase>\n")
			buf.WriteString("            <words>\n")
			words := strings.Fields(cb.Text)
			for _, word := range words {
				fmt.Fprintf(&buf, "              <word>\n")
				fmt.Fprintf(&buf, "                <item type=\"txt\">%s</item>\n", escapeXML(word))
				fmt.Fprintf(&buf, "              </word>\n")
			}
			buf.WriteString("            </words>\n")
			buf.WriteString("          </phrase>\n")
		}
		buf.WriteString("        </phrases>\n")
		buf.WriteString("      </paragraph>\n")
	}

	buf.WriteString("    </paragraphs>\n")
	buf.WriteString("  </interlinear-text>\n")
	buf.WriteString("</document>\n")

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write output file: %w", err)
	}

	return outputPath, nil
}

// escapeXML escapes special XML characters.
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
