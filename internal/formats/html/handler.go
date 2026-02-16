// Package html provides the embedded handler for HTML format plugin.
package html

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// Handler implements the EmbeddedFormatHandler interface for HTML.
type Handler struct{}

// IR types are now imported from core/ir package

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.html",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-html",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"file"},
			Outputs: []string{"artifact.kind:html"},
		},
	}
}

// Register registers this plugin with the embedded registry.
func Register() {
	plugins.RegisterEmbeddedPlugin(&plugins.EmbeddedPlugin{
		Manifest: Manifest(),
		Format:   &Handler{},
	})
}

func init() {
	Register()
}

// Detect implements EmbeddedFormatHandler.Detect.
func (h *Handler) Detect(path string) (*plugins.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &plugins.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}, nil
	}

	if info.IsDir() {
		return &plugins.DetectResult{Detected: false, Reason: "path is a directory"}, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".html" {
		return &plugins.DetectResult{Detected: false, Reason: "not a .html file"}, nil
	}

	return &plugins.DetectResult{
		Detected: true,
		Format:   "html",
		Reason:   "HTML file detected",
	}, nil
}

// Ingest implements EmbeddedFormatHandler.Ingest.
func (h *Handler) Ingest(path, outputDir string) (*plugins.IngestResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create blob dir: %w", err)
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write blob: %w", err)
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return &plugins.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata:   map[string]string{"format": "html"},
	}, nil
}

// Enumerate implements EmbeddedFormatHandler.Enumerate.
func (h *Handler) Enumerate(path string) (*plugins.EnumerateResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	return &plugins.EnumerateResult{
		Entries: []plugins.EnumerateEntry{
			{Path: filepath.Base(path), SizeBytes: info.Size(), IsDir: false},
		},
	}, nil
}

// ExtractIR implements EmbeddedFormatHandler.ExtractIR.
func (h *Handler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ir.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   ir.ModuleBible,
		SourceFormat: "HTML",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    ir.LossL1,
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

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize IR: %w", err)
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0600); err != nil {
		return nil, fmt.Errorf("failed to write IR: %w", err)
	}

	return &plugins.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "HTML",
			TargetFormat: "IR",
			LossClass:    "L1",
		},
	}, nil
}

func parseHTMLContent(content, artifactID string) []*ir.Document {
	doc := &ir.Document{
		ID:         artifactID,
		Title:      artifactID,
		Order:      1,
		Attributes: make(map[string]string),
	}

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
							Type:          ir.SpanVerse,
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

// EmitNative implements EmbeddedFormatHandler.EmitNative.
func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	data, err := os.ReadFile(irPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read IR file: %w", err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return nil, fmt.Errorf("failed to parse IR: %w", err)
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".html")

	// Check for raw HTML for round-trip
	if raw, ok := corpus.Attributes["_html_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
			return nil, fmt.Errorf("failed to write HTML: %w", err)
		}
		return &plugins.EmitNativeResult{
			OutputPath: outputPath,
			Format:     "HTML",
			LossClass:  "L0",
			LossReport: &plugins.LossReportIPC{
				SourceFormat: "IR",
				TargetFormat: "HTML",
				LossClass:    "L0",
			},
		}, nil
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
					if span.Ref != nil && span.Type == ir.SpanVerse {
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

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0600); err != nil {
		return nil, fmt.Errorf("failed to write HTML: %w", err)
	}

	return &plugins.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "HTML",
		LossClass:  "L1",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "IR",
			TargetFormat: "HTML",
			LossClass:    "L1",
		},
	}, nil
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
