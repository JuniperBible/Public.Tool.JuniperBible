//go:build !sdk

// Plugin format-html handles HTML Bible format.
// Produces static HTML site with navigation.
//
// IR Support:
// - extract-ir: Reads HTML Bible format to IR (L1)
// - emit-native: Converts IR to HTML format (L1)
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Parallel Corpus Types
func main() {
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorf("failed to decode request: %v", err)
		return
	}

	switch req.Command {
	case "detect":
		handleDetect(req.Args)
	case "ingest":
		handleIngest(req.Args)
	case "enumerate":
		handleEnumerate(req.Args)
	case "extract-ir":
		handleExtractIR(req.Args)
	case "emit-native":
		handleEmitNative(req.Args)
	case "emit-parallel":
		handleEmitParallel(req.Args)
	case "emit-interlinear":
		handleEmitInterlinear(req.Args)
	default:
		ipc.RespondErrorf("unknown command: %s", req.Command)
	}
}

func handleDetect(args map[string]interface{}) {
	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		ipc.MustRespond(ipc.DetectFailure(fmt.Sprintf("cannot stat: %v", err)))
		return
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
				ipc.MustRespond(ipc.DetectFailure("no .html files found"))
				return
			}
			path = matches[0]
		}
	}

	// Two-stage detection: extension + content
	result := ipc.StandardDetect(path, "HTML",
		[]string{".html", ".htm"},
		[]string{"class=\"verse\"", "data-verse=", "<span class=\"v\">"},
	)
	ipc.MustRespond(result)
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		ipc.RespondErrorf("failed to create blob dir: %v", err)
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		ipc.RespondErrorf("failed to write blob: %v", err)
		return
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "HTML",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		ipc.RespondErrorf("failed to stat: %v", err)
		return
	}

	var entries []ipc.EnumerateEntry

	if info.IsDir() {
		filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext == ".html" || ext == ".htm" {
				rel, _ := filepath.Rel(path, p)
				entries = append(entries, ipc.EnumerateEntry{
					Path:      rel,
					SizeBytes: info.Size(),
					IsDir:     false,
					Metadata:  map[string]string{"format": "HTML"},
				})
			}
			return nil
		})
	} else {
		entries = []ipc.EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
				Metadata:  map[string]string{"format": "HTML"},
			},
		}
	}
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: entries,
	})
}

func handleExtractIR(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ipc.Corpus{
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

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to serialize IR: %v", err)
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}
	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "HTML",
			TargetFormat: "IR",
			LossClass:    "L1",
		},
	})
}

func parseHTMLContent(content, artifactID string) []*ipc.Document {
	doc := &ipc.Document{
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

		cb := &ipc.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     text,
			Hash:     hex.EncodeToString(hash[:]),
			Anchors: []*ipc.Anchor{
				{
					ID:       fmt.Sprintf("a-%d-0", sequence),
					Position: 0,
					Spans: []*ipc.Span{
						{
							ID:            fmt.Sprintf("s-%s", osisID),
							Type:          "VERSE",
							StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
							Ref: &ipc.Ref{
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

	return []*ipc.Document{doc}
}

func handleEmitNative(args map[string]interface{}) {
	irPath, ok := args["ir_path"].(string)
	if !ok {
		ipc.RespondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR file: %v", err)
		return
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		ipc.RespondErrorf("failed to parse IR: %v", err)
		return
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".html")

	// Check for raw HTML for round-trip
	if raw, ok := corpus.Attributes["_html_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			ipc.RespondErrorf("failed to write HTML: %v", err)
			return
		}
		ipc.MustRespond(&ipc.EmitNativeResult{
			OutputPath: outputPath,
			Format:     "HTML",
			LossClass:  "L0",
			LossReport: &ipc.LossReport{
				SourceFormat: "IR",
				TargetFormat: "HTML",
				LossClass:    "L0",
			},
		})
		return
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
		ipc.RespondErrorf("failed to write HTML: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "HTML",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "HTML",
			LossClass:    "L1",
		},
	})
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func handleEmitParallel(args map[string]interface{}) {
	irPath, ok := args["ir_path"].(string)
	if !ok {
		ipc.RespondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR file: %v", err)
		return
	}

	var parallel ipc.ParallelCorpus
	if err := json.Unmarshal(data, &parallel); err != nil {
		ipc.RespondErrorf("failed to parse parallel corpus: %v", err)
		return
	}

	var buf strings.Builder

	// HTML header with parallel view styles
	buf.WriteString("<!DOCTYPE html>\n")
	buf.WriteString("<html lang=\"en\">\n")
	buf.WriteString("<head>\n")
	buf.WriteString("  <meta charset=\"UTF-8\">\n")
	buf.WriteString("  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	buf.WriteString(fmt.Sprintf("  <title>%s - Parallel View</title>\n", escapeHTML(parallel.ID)))
	buf.WriteString("  <style>\n")
	buf.WriteString("    body { font-family: Georgia, serif; margin: 0; padding: 20px; }\n")
	buf.WriteString("    h1 { text-align: center; }\n")
	buf.WriteString("    .translations { display: flex; gap: 10px; margin-bottom: 20px; justify-content: center; }\n")
	buf.WriteString("    .translation-tag { padding: 5px 15px; background: #f0f0f0; border-radius: 4px; }\n")
	buf.WriteString("    .parallel-table { width: 100%; border-collapse: collapse; }\n")
	buf.WriteString("    .parallel-table th, .parallel-table td { border: 1px solid #ddd; padding: 10px; vertical-align: top; }\n")
	buf.WriteString("    .parallel-table th { background: #f5f5f5; font-weight: bold; }\n")
	buf.WriteString("    .ref { font-weight: bold; color: #666; white-space: nowrap; }\n")
	buf.WriteString("    .verse-text { line-height: 1.6; }\n")
	buf.WriteString("  </style>\n")
	buf.WriteString("</head>\n")
	buf.WriteString("<body>\n")

	buf.WriteString(fmt.Sprintf("<h1>%s</h1>\n", escapeHTML(parallel.ID)))

	// Translation tags
	buf.WriteString("<div class=\"translations\">\n")
	for _, ref := range parallel.Corpora {
		title := ref.Title
		if title == "" {
			title = ref.ID
		}
		buf.WriteString(fmt.Sprintf("  <span class=\"translation-tag\">%s (%s)</span>\n", escapeHTML(title), ref.Language))
	}
	buf.WriteString("</div>\n")

	// Parallel table
	buf.WriteString("<table class=\"parallel-table\">\n")
	buf.WriteString("<thead><tr>\n")
	buf.WriteString("  <th>Ref</th>\n")
	for _, ref := range parallel.Corpora {
		title := ref.Title
		if title == "" {
			title = ref.ID
		}
		buf.WriteString(fmt.Sprintf("  <th>%s</th>\n", escapeHTML(title)))
	}
	buf.WriteString("</tr></thead>\n")
	buf.WriteString("<tbody>\n")

	for _, alignment := range parallel.Alignments {
		for _, unit := range alignment.Units {
			buf.WriteString("<tr>\n")
			ref := ""
			if unit.Ref != nil {
				ref = unit.Ref.OSISID
			}
			buf.WriteString(fmt.Sprintf("  <td class=\"ref\">%s</td>\n", escapeHTML(ref)))
			for _, corpusRef := range parallel.Corpora {
				text := unit.Texts[corpusRef.ID]
				buf.WriteString(fmt.Sprintf("  <td class=\"verse-text\">%s</td>\n", escapeHTML(text)))
			}
			buf.WriteString("</tr>\n")
		}
	}

	buf.WriteString("</tbody>\n")
	buf.WriteString("</table>\n")
	buf.WriteString("</body>\n")
	buf.WriteString("</html>\n")

	outputPath := filepath.Join(outputDir, parallel.ID+".parallel.html")
	if err := os.WriteFile(outputPath, []byte(buf.String()), 0644); err != nil {
		ipc.RespondErrorf("failed to write HTML: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "HTML-Parallel",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "ParallelCorpus",
			TargetFormat: "HTML-Parallel",
			LossClass:    "L1",
		},
	})
}

func handleEmitInterlinear(args map[string]interface{}) {
	irPath, ok := args["ir_path"].(string)
	if !ok {
		ipc.RespondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR file: %v", err)
		return
	}

	var lines []ipc.InterlinearLine
	if err := json.Unmarshal(data, &lines); err != nil {
		ipc.RespondErrorf("failed to parse interlinear data: %v", err)
		return
	}

	// Collect all layer names
	layerNames := make(map[string]bool)
	for _, line := range lines {
		for name := range line.Layers {
			layerNames[name] = true
		}
	}

	var buf strings.Builder

	// HTML header with interlinear styles
	buf.WriteString("<!DOCTYPE html>\n")
	buf.WriteString("<html lang=\"en\">\n")
	buf.WriteString("<head>\n")
	buf.WriteString("  <meta charset=\"UTF-8\">\n")
	buf.WriteString("  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	buf.WriteString("  <title>Interlinear Text</title>\n")
	buf.WriteString("  <style>\n")
	buf.WriteString("    body { font-family: Georgia, serif; margin: 0; padding: 20px; }\n")
	buf.WriteString("    h1 { text-align: center; }\n")
	buf.WriteString("    .interlinear-line { margin: 20px 0; padding: 15px; border: 1px solid #eee; border-radius: 4px; }\n")
	buf.WriteString("    .ref { font-weight: bold; color: #666; margin-bottom: 10px; }\n")
	buf.WriteString("    .tokens { display: flex; flex-wrap: wrap; gap: 20px; }\n")
	buf.WriteString("    .token { display: flex; flex-direction: column; align-items: center; }\n")
	buf.WriteString("    .token-layer { padding: 2px 5px; }\n")
	buf.WriteString("    .token-layer:first-child { font-size: 1.2em; font-weight: bold; }\n")
	buf.WriteString("    .token-layer:last-child { font-size: 0.9em; color: #666; }\n")
	buf.WriteString("  </style>\n")
	buf.WriteString("</head>\n")
	buf.WriteString("<body>\n")

	buf.WriteString("<h1>Interlinear Text</h1>\n")

	for _, line := range lines {
		buf.WriteString("<div class=\"interlinear-line\">\n")

		ref := ""
		if line.Ref != nil {
			ref = line.Ref.OSISID
		}
		buf.WriteString(fmt.Sprintf("  <div class=\"ref\">%s</div>\n", escapeHTML(ref)))

		// Find max tokens
		maxTokens := 0
		for _, layer := range line.Layers {
			if len(layer.Tokens) > maxTokens {
				maxTokens = len(layer.Tokens)
			}
		}

		buf.WriteString("  <div class=\"tokens\">\n")
		for i := 0; i < maxTokens; i++ {
			buf.WriteString("    <div class=\"token\">\n")
			for name := range layerNames {
				layer := line.Layers[name]
				text := ""
				if layer != nil && i < len(layer.Tokens) {
					text = layer.Tokens[i]
				}
				buf.WriteString(fmt.Sprintf("      <span class=\"token-layer\" data-layer=\"%s\">%s</span>\n",
					escapeHTML(name), escapeHTML(text)))
			}
			buf.WriteString("    </div>\n")
		}
		buf.WriteString("  </div>\n")
		buf.WriteString("</div>\n")
	}

	buf.WriteString("</body>\n")
	buf.WriteString("</html>\n")

	outputPath := filepath.Join(outputDir, "interlinear.html")
	if err := os.WriteFile(outputPath, []byte(buf.String()), 0644); err != nil {
		ipc.RespondErrorf("failed to write HTML: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "HTML-Interlinear",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "InterlinearLine",
			TargetFormat: "HTML-Interlinear",
			LossClass:    "L1",
		},
	})
}
