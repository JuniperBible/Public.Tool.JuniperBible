// Plugin format-html handles HTML Bible format (SDK version).
// Produces static HTML site with navigation.
//
// IR Support:
// - extract-ir: Reads HTML Bible format to IR (L1)
// - emit-native: Converts IR to HTML format (L1)
package main

import (
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
		Name:       "HTML",
		Extensions: []string{".html", ".htm"},
		Detect:     detectHTML,
		Parse:      parseHTML,
		Emit:       emitHTML,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// detectHTML performs custom HTML Bible format detection.
func detectHTML(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		}, nil
	}

	// Handle both files and directories
	if info.IsDir() {
		// Check for index.html or html files
		indexPath := filepath.Join(path, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			path = indexPath
		} else {
			matches, _ := filepath.Glob(filepath.Join(path, "*.html"))
			if len(matches) == 0 {
				return &ipc.DetectResult{
					Detected: false,
					Reason:   "no .html files found",
				}, nil
			}
			path = matches[0]
		}
	}

	// Two-stage detection: extension + content
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".html" && ext != ".htm" {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not an .html or .htm file",
		}, nil
	}

	// Check content for Bible markers
	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		}, nil
	}

	content := string(data)
	markers := []string{"class=\"verse\"", "data-verse=", "<span class=\"v\">"}
	for _, marker := range markers {
		if strings.Contains(content, marker) {
			return &ipc.DetectResult{
				Detected: true,
				Format:   "HTML",
				Reason:   "HTML Bible format detected via content markers",
			}, nil
		}
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "no HTML Bible format markers found",
	}, nil
}

// parseHTML parses an HTML Bible file and returns an IR Corpus.
func parseHTML(path string) (*ir.Corpus, error) {
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
		SourceFormat: "HTML",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L1",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_html_raw"] = string(data)

	// Parse HTML content
	content := string(data)

	// Extract title
	titlePattern := regexp.MustCompile(`<title>([^<]+)</title>`)
	if matches := titlePattern.FindStringSubmatch(content); len(matches) > 1 {
		corpus.Title = strings.TrimSpace(matches[1])
	}

	// Parse verses
	corpus.Documents = parseHTMLContent(content, artifactID)

	return corpus, nil
}

// parseHTMLContent parses HTML content and extracts verses into documents.
func parseHTMLContent(content, artifactID string) []*ir.Document {
	doc := &ir.Document{
		ID:    artifactID,
		Title: artifactID,
		Order: 1,
	}

	// Parse verses: multiple patterns for different HTML structures
	// Pattern 1: <p class="verse" data-verse="1">...<span class="verse-text">text</span></p>
	versePattern1 := regexp.MustCompile(`<p[^>]*class="verse"[^>]*data-verse="(\d+)"[^>]*>.*?<span[^>]*class="verse-text"[^>]*>([^<]+)</span>`)
	// Pattern 2: <span class="verse" data-verse="1">text</span>
	versePattern2 := regexp.MustCompile(`<span[^>]*class="verse"[^>]*data-verse="(\d+)"[^>]*>([^<]*)</span>`)
	// Pattern 3: <span class="v">1</span> text
	versePattern3 := regexp.MustCompile(`<span class="v">(\d+)</span>\s*([^<]+)`)
	chapterPattern := regexp.MustCompile(`<h[23][^>]*>Chapter\s+(\d+)</h[23]>`)

	currentChapter := 1
	sequence := 0

	// Check for chapter markers
	if matches := chapterPattern.FindAllStringSubmatch(content, -1); len(matches) > 0 {
		// Process chapters
		for _, match := range matches {
			currentChapter, _ = strconv.Atoi(match[1])
		}
	}

	// Try patterns in order
	matches := versePattern1.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		matches = versePattern2.FindAllStringSubmatch(content, -1)
	}
	if len(matches) == 0 {
		matches = versePattern3.FindAllStringSubmatch(content, -1)
	}

	for _, match := range matches {
		verse, _ := strconv.Atoi(match[1])
		text := strings.TrimSpace(match[2])

		if text == "" {
			continue
		}

		sequence++
		hash := sha256.Sum256([]byte(text))
		osisID := fmt.Sprintf("%s.%d.%d", doc.ID, currentChapter, verse)

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
								Book:    doc.ID,
								Chapter: currentChapter,
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

	return []*ir.Document{doc}
}

// emitHTML converts an IR Corpus to HTML format.
func emitHTML(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".html")

	// Check for raw HTML for round-trip
	if raw, ok := corpus.Attributes["_html_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			return "", fmt.Errorf("failed to write HTML: %w", err)
		}
		return outputPath, nil
	}

	// Generate HTML from IR
	var buf strings.Builder

	// HTML header
	buf.WriteString("<!DOCTYPE html>\n")
	buf.WriteString("<html lang=\"")
	if corpus.Language != "" {
		buf.WriteString(corpus.Language)
	} else {
		buf.WriteString("en")
	}
	buf.WriteString("\">\n")
	buf.WriteString("<head>\n")
	buf.WriteString("  <meta charset=\"UTF-8\">\n")
	buf.WriteString("  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	buf.WriteString(fmt.Sprintf("  <title>%s</title>\n", escapeHTML(corpus.Title)))
	buf.WriteString("  <style>\n")
	buf.WriteString("    body { font-family: Georgia, serif; max-width: 800px; margin: 0 auto; padding: 20px; }\n")
	buf.WriteString("    h1 { text-align: center; }\n")
	buf.WriteString("    h2 { margin-top: 2em; border-bottom: 1px solid #ccc; }\n")
	buf.WriteString("    .verse { margin: 0.5em 0; }\n")
	buf.WriteString("    .verse-num { font-weight: bold; color: #666; margin-right: 0.5em; }\n")
	buf.WriteString("  </style>\n")
	buf.WriteString("</head>\n")
	buf.WriteString("<body>\n")

	buf.WriteString(fmt.Sprintf("<h1>%s</h1>\n", escapeHTML(corpus.Title)))

	for _, doc := range corpus.Documents {
		buf.WriteString(fmt.Sprintf("<article id=\"%s\">\n", doc.ID))
		buf.WriteString(fmt.Sprintf("<h2>%s</h2>\n", escapeHTML(doc.Title)))

		currentChapter := 0
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						if span.Ref.Chapter != currentChapter {
							if currentChapter > 0 {
								buf.WriteString("</section>\n")
							}
							currentChapter = span.Ref.Chapter
							buf.WriteString(fmt.Sprintf("<section class=\"chapter\" id=\"ch%d\">\n", currentChapter))
							buf.WriteString(fmt.Sprintf("<h3>Chapter %d</h3>\n", currentChapter))
						}
						buf.WriteString(fmt.Sprintf("<p class=\"verse\" data-verse=\"%d\">", span.Ref.Verse))
						buf.WriteString(fmt.Sprintf("<span class=\"verse-num\">%d</span>", span.Ref.Verse))
						buf.WriteString(fmt.Sprintf("<span class=\"verse-text\">%s</span>", escapeHTML(cb.Text)))
						buf.WriteString("</p>\n")
					}
				}
			}
		}
		if currentChapter > 0 {
			buf.WriteString("</section>\n")
		}
		buf.WriteString("</article>\n")
	}

	buf.WriteString("</body>\n")
	buf.WriteString("</html>\n")

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write HTML: %w", err)
	}

	return outputPath, nil
}

// escapeHTML escapes HTML special characters.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
