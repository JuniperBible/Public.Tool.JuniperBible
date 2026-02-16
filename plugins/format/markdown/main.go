//go:build !sdk

// Plugin format-markdown handles Markdown Bible format.
// Produces Hugo-compatible markdown with YAML frontmatter.
//
// IR Support:
// - extract-ir: Reads Markdown Bible format to IR (L1)
// - emit-native: Converts IR to Markdown format (L1)
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

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
	default:
		ipc.RespondErrorf("unknown command: %s", req.Command)
	}
}

func handleDetect(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	// Handle both files and directories
	if info.IsDir() {
		// Check for markdown files with frontmatter
		matches, _ := filepath.Glob(filepath.Join(path, "*.md"))
		if len(matches) == 0 {
			matches, _ = filepath.Glob(filepath.Join(path, "*/*.md"))
		}
		if len(matches) == 0 {
			ipc.MustRespond(&ipc.DetectResult{
				Detected: false,
				Reason:   "no .md files found",
			})
			return
		}
		// Check first file for frontmatter
		path = matches[0]
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not a .md file",
		})
		return
	}

	// Check for YAML frontmatter
	data, err := safefile.ReadFile(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		})
		return
	}

	content := string(data)
	if !strings.HasPrefix(content, "---") {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "no YAML frontmatter found",
		})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   "Markdown",
		Reason:   "Markdown with YAML frontmatter detected",
	})
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

	data, err := safefile.ReadFile(path)
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
			"format": "Markdown",
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
			if strings.HasSuffix(p, ".md") {
				rel, relErr := filepath.Rel(path, p)
				if relErr != nil {
					return nil // Skip files with path errors
				}
				entries = append(entries, ipc.EnumerateEntry{
					Path:      rel,
					SizeBytes: info.Size(),
					IsDir:     false,
					Metadata:  map[string]string{"format": "Markdown"},
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
				Metadata:  map[string]string{"format": "Markdown"},
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

	data, err := safefile.ReadFile(path)
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
		SourceFormat: "Markdown",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L1",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_markdown_raw"] = string(data)

	// Parse frontmatter and content
	content := string(data)
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) == 2 {
			frontmatter := parts[0]
			body := parts[1]

			// Parse simple YAML frontmatter
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if idx := strings.Index(line, ":"); idx > 0 {
					key := strings.TrimSpace(line[:idx])
					value := strings.TrimSpace(line[idx+1:])
					value = strings.Trim(value, "\"'")
					switch key {
					case "title":
						corpus.Title = value
					case "language":
						corpus.Language = value
					case "book":
						corpus.Attributes["book"] = value
					}
				}
			}

			// Parse verses from body
			corpus.Documents = parseMarkdownContent(body, corpus.Attributes["book"])
		}
	}

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
			SourceFormat: "Markdown",
			TargetFormat: "IR",
			LossClass:    "L1",
		},
	})
}

func parseMarkdownContent(body, bookID string) []*ipc.Document {
	doc := &ipc.Document{
		ID:    bookID,
		Title: bookID,
		Order: 1,
	}

	if bookID == "" {
		doc.ID = "content"
		doc.Title = "Content"
	}

	// Parse verses: look for patterns like "**1** text" or "1. text"
	versePattern := regexp.MustCompile(`\*\*(\d+)\*\*\s+(.+?)(?:\n|$)`)
	chapterPattern := regexp.MustCompile(`##\s+Chapter\s+(\d+)`)

	currentChapter := 1
	sequence := 0

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()

		// Check for chapter heading
		if matches := chapterPattern.FindStringSubmatch(line); len(matches) > 1 {
			currentChapter, _ = strconv.Atoi(matches[1])
			continue
		}

		// Check for verses
		if matches := versePattern.FindAllStringSubmatch(line, -1); len(matches) > 0 {
			for _, match := range matches {
				verse, _ := strconv.Atoi(match[1])
				text := strings.TrimSpace(match[2])

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
		}
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

	data, err := safefile.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR file: %v", err)
		return
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		ipc.RespondErrorf("failed to parse IR: %v", err)
		return
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".md")

	// Check for raw markdown for round-trip
	if raw, ok := corpus.Attributes["_markdown_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			ipc.RespondErrorf("failed to write Markdown: %v", err)
			return
		}
		ipc.MustRespond(&ipc.EmitNativeResult{
			OutputPath: outputPath,
			Format:     "Markdown",
			LossClass:  "L0",
			LossReport: &ipc.LossReport{
				SourceFormat: "IR",
				TargetFormat: "Markdown",
				LossClass:    "L0",
			},
		})
		return
	}

	// Generate Markdown from IR
	var buf strings.Builder

	// Hugo frontmatter
	buf.WriteString("---\n")
	buf.WriteString(fmt.Sprintf("title: \"%s\"\n", corpus.Title))
	if corpus.Language != "" {
		buf.WriteString(fmt.Sprintf("language: \"%s\"\n", corpus.Language))
	}
	buf.WriteString(fmt.Sprintf("date: \"%s\"\n", "2024-01-01"))
	buf.WriteString("type: \"bible\"\n")
	buf.WriteString("---\n\n")

	for _, doc := range corpus.Documents {
		buf.WriteString(fmt.Sprintf("# %s\n\n", doc.Title))

		currentChapter := 0
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						if span.Ref.Chapter != currentChapter {
							currentChapter = span.Ref.Chapter
							buf.WriteString(fmt.Sprintf("\n## Chapter %d\n\n", currentChapter))
						}
						buf.WriteString(fmt.Sprintf("**%d** %s\n", span.Ref.Verse, cb.Text))
					}
				}
			}
		}
		buf.WriteString("\n")
	}

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0644); err != nil {
		ipc.RespondErrorf("failed to write Markdown: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "Markdown",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "Markdown",
			LossClass:    "L1",
		},
	})
}
