// Package html handles HTML Bible format.
// Produces static HTML site with navigation.
//
// IR Support:
// - extract-ir: Reads HTML Bible format to IR (L1)
// - emit-native: Converts IR to HTML format (L1)
package html

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// Config defines the HTML format plugin.
var Config = &format.Config{
	PluginID:   "format.html",
	Name:       "HTML",
	Extensions: []string{".html", ".htm"},
	Detect:     detectHTML,
	Parse:      parseHTML,
	Emit:       emitHTML,
}

func detectHTML(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return notDetected(fmt.Sprintf("cannot stat: %v", err)), nil
	}

	path, err = resolveHTMLPath(path, info)
	if err != nil {
		return notDetected(err.Error()), nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".html" && ext != ".htm" {
		return notDetected("not an .html file"), nil
	}

	return detectHTMLContent(path)
}

func resolveHTMLPath(path string, info os.FileInfo) (string, error) {
	if !info.IsDir() {
		return path, nil
	}
	indexPath := filepath.Join(path, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		return indexPath, nil
	}
	matches, _ := filepath.Glob(filepath.Join(path, "*.html"))
	if len(matches) == 0 {
		return "", fmt.Errorf("no .html files found")
	}
	return matches[0], nil
}

func detectHTMLContent(path string) (*ipc.DetectResult, error) {
	data, err := safefile.ReadFile(path)
	if err != nil {
		return notDetected(fmt.Sprintf("cannot read file: %v", err)), nil
	}
	if hasVerseMarkers(string(data)) {
		return &ipc.DetectResult{Detected: true, Format: "HTML", Reason: "HTML Bible format detected"}, nil
	}
	return notDetected("no verse markers found"), nil
}

func hasVerseMarkers(content string) bool {
	return strings.Contains(content, "class=\"verse\"") ||
		strings.Contains(content, "data-verse=") ||
		strings.Contains(content, "<span class=\"v\">")
}

func notDetected(reason string) *ipc.DetectResult {
	return &ipc.DetectResult{Detected: false, Reason: reason}
}

func parseHTML(path string) (*ir.Corpus, error) {
	data, err := safefile.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "HTML"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L1"
	corpus.Attributes = make(map[string]string)

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

func parseHTMLContent(content, artifactID string) []*ir.Document {
	doc := ir.NewDocument(artifactID, artifactID, 1)

	// Parse verses: multiple patterns for different HTML structures
	versePattern1 := regexp.MustCompile(`<p[^>]*class="verse"[^>]*data-verse="(\d+)"[^>]*>.*?<span[^>]*class="verse-text"[^>]*>([^<]+)</span>`)
	versePattern2 := regexp.MustCompile(`<span[^>]*class="verse"[^>]*data-verse="(\d+)"[^>]*>([^<]*)</span>`)
	versePattern3 := regexp.MustCompile(`<span class="v">(\d+)</span>\s*([^<]+)`)
	chapterPattern := regexp.MustCompile(`<h[23][^>]*>Chapter\s+(\d+)</h[23]>`)

	currentChapter := 1
	sequence := 0

	// Check for chapter markers
	if matches := chapterPattern.FindAllStringSubmatch(content, -1); len(matches) > 0 {
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

func emitHTML(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".html")

	// Check for raw HTML for round-trip
	if raw, ok := corpus.Attributes["_html_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
			return "", fmt.Errorf("failed to write HTML: %w", err)
		}
		return outputPath, nil
	}

	// Generate HTML from IR
	var buf strings.Builder
	writeHTMLHeader(&buf, corpus)

	buf.WriteString(fmt.Sprintf("<h1>%s</h1>\n", escapeHTML(corpus.Title)))
	for _, doc := range corpus.Documents {
		writeDocumentHTML(&buf, doc)
	}

	buf.WriteString("</body>\n")
	buf.WriteString("</html>\n")

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0600); err != nil {
		return "", fmt.Errorf("failed to write HTML: %w", err)
	}
	return outputPath, nil
}

// writeHTMLHeader emits the DOCTYPE, <html>, <head>, and opening <body> tags.
func writeHTMLHeader(buf *strings.Builder, corpus *ir.Corpus) {
	lang := corpus.Language
	if lang == "" {
		lang = "en"
	}
	buf.WriteString("<!DOCTYPE html>\n")
	buf.WriteString(fmt.Sprintf("<html lang=\"%s\">\n", lang))
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
}

// writeDocumentHTML emits the <article> element for a single document,
// including all chapter sections and verse paragraphs.
func writeDocumentHTML(buf *strings.Builder, doc *ir.Document) {
	buf.WriteString(fmt.Sprintf("<article id=\"%s\">\n", doc.ID))
	buf.WriteString(fmt.Sprintf("<h2>%s</h2>\n", escapeHTML(doc.Title)))

	currentChapter := 0
	for _, cb := range doc.ContentBlocks {
		for _, anchor := range cb.Anchors {
			for _, span := range anchor.Spans {
				currentChapter = emitVerseSpan(buf, cb, span, currentChapter)
			}
		}
	}
	if currentChapter > 0 {
		buf.WriteString("</section>\n")
	}
	buf.WriteString("</article>\n")
}

// emitVerseSpan writes the chapter section header (when the chapter changes)
// and the verse <p> element for a single VERSE span. It returns the (possibly
// updated) current chapter number.
func emitVerseSpan(buf *strings.Builder, cb *ir.ContentBlock, span *ir.Span, currentChapter int) int {
	if span.Ref == nil || span.Type != "VERSE" {
		return currentChapter
	}
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
	return currentChapter
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
