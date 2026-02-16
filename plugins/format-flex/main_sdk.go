//go:build sdk

// Plugin format-flex handles FLEx (Fieldworks Language Explorer) format.
// FLEx uses XML-based linguistic database format with phrase/word structures.
//
// IR Support:
// - extract-ir: Extracts IR from FLEx XML (L1 text preserved)
// - emit-native: Converts IR to simplified FLEx format (L1)
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// FLEx XML types
type FlexText struct {
	XMLName    xml.Name   `xml:"document"`
	Title      string     `xml:"interlinear-text>item"`
	Paragraphs []FlexPara `xml:"interlinear-text>paragraphs>paragraph"`
}

type FlexPara struct {
	Phrases []FlexPhrase `xml:"phrases>phrase"`
}

type FlexPhrase struct {
	Words []FlexWord `xml:"words>word"`
	Item  string     `xml:"item"`
}

type FlexWord struct {
	Items []FlexItem `xml:"item"`
}

type FlexItem struct {
	Type  string `xml:"type,attr"`
	Lang  string `xml:"lang,attr"`
	Value string `xml:",chardata"`
}

func main() {
	if err := format.Run(&format.Config{
		Name:       "FLEx",
		Extensions: []string{".flextext", ".xml"},
		Detect:     detectFlex,
		Parse:      parseFlex,
		Emit:       emitFlex,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectFlex(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{Detected: false, Reason: "path is a directory"}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot read: %v", err)}, nil
	}

	content := string(data)
	if strings.Contains(content, "<interlinear-text") || strings.Contains(content, "flextext") {
		return &ipc.DetectResult{Detected: true, Format: "FLEx", Reason: "FLEx XML format detected"}, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".flextext" {
		return &ipc.DetectResult{Detected: true, Format: "FLEx", Reason: "FLEx file extension detected"}, nil
	}

	return &ipc.DetectResult{Detected: false, Reason: "not a FLEx file"}, nil
}

func parseFlex(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "FLEx"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L1"
	corpus.Attributes = map[string]string{"_flex_raw": string(data)}

	// Parse FLEx XML
	var flexDoc FlexText
	if err := xml.Unmarshal(data, &flexDoc); err != nil {
		// Return corpus with raw data even if parsing fails
		doc := ir.NewDocument(artifactID, artifactID, 1)
		corpus.Documents = []*ir.Document{doc}
		return corpus, nil
	}

	corpus.Title = flexDoc.Title
	doc := ir.NewDocument(artifactID, flexDoc.Title, 1)
	sequence := 0

	for _, para := range flexDoc.Paragraphs {
		for _, phrase := range para.Phrases {
			var wordTexts []string
			for _, word := range phrase.Words {
				for _, item := range word.Items {
					if item.Type == "txt" || item.Type == "punct" {
						wordTexts = append(wordTexts, item.Value)
					}
				}
			}

			text := strings.Join(wordTexts, " ")
			if phrase.Item != "" && text == "" {
				text = phrase.Item
			}

			if text == "" {
				continue
			}

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

	corpus.Documents = []*ir.Document{doc}
	return corpus, nil
}

func emitFlex(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".flextext")

	// Check for raw FLEx for round-trip
	if raw, ok := corpus.Attributes["_flex_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
			return "", fmt.Errorf("failed to write FLEx file: %w", err)
		}
		return outputPath, nil
	}

	// Generate simplified FLEx XML from IR
	var buf strings.Builder
	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buf.WriteString("<document version=\"2\">\n")
	buf.WriteString("  <interlinear-text>\n")
	buf.WriteString(fmt.Sprintf("    <item type=\"title\">%s</item>\n", escapeXML(corpus.Title)))
	buf.WriteString("    <paragraphs>\n")

	for _, doc := range corpus.Documents {
		buf.WriteString("      <paragraph>\n")
		buf.WriteString("        <phrases>\n")
		for _, cb := range doc.ContentBlocks {
			buf.WriteString("          <phrase>\n")
			buf.WriteString(fmt.Sprintf("            <item type=\"txt\">%s</item>\n", escapeXML(cb.Text)))
			buf.WriteString("          </phrase>\n")
		}
		buf.WriteString("        </phrases>\n")
		buf.WriteString("      </paragraph>\n")
	}

	buf.WriteString("    </paragraphs>\n")
	buf.WriteString("  </interlinear-text>\n")
	buf.WriteString("</document>\n")

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0600); err != nil {
		return "", fmt.Errorf("failed to write FLEx file: %w", err)
	}

	return outputPath, nil
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
