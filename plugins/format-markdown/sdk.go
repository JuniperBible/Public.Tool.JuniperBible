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

func runSDK() {
	if err := format.Run(&format.Config{
		Name:       "Markdown",
		Extensions: []string{".md"},
		Detect:     detectMarkdown,
		Parse:      parseMarkdown,
		Emit:       emitMarkdown,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectMarkdown(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}, nil
	}

	// Handle both files and directories
	if info.IsDir() {
		matches, _ := filepath.Glob(filepath.Join(path, "*.md"))
		if len(matches) == 0 {
			matches, _ = filepath.Glob(filepath.Join(path, "*/*.md"))
		}
		if len(matches) == 0 {
			return &ipc.DetectResult{Detected: false, Reason: "no .md files found"}, nil
		}
		path = matches[0]
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" {
		return &ipc.DetectResult{Detected: false, Reason: "not a .md file"}, nil
	}

	// Check for YAML frontmatter
	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot read file: %v", err)}, nil
	}

	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return &ipc.DetectResult{Detected: false, Reason: "no YAML frontmatter found"}, nil
	}

	return &ipc.DetectResult{Detected: true, Format: "Markdown", Reason: "Markdown with YAML frontmatter detected"}, nil
}

func parseMarkdown(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "Markdown"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L1"
	corpus.Attributes = map[string]string{"_markdown_raw": string(data)}

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

	return corpus, nil
}

func parseMarkdownContent(body, bookID string) []*ir.Document {
	doc := ir.NewDocument(bookID, bookID, 1)

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

				cb := &ir.ContentBlock{
					ID:       fmt.Sprintf("cb-%d", sequence),
					Sequence: sequence,
					Text:     text,
					Hash:     hex.EncodeToString(hash[:]),
					Anchors: []*ir.Anchor{{
						ID:       fmt.Sprintf("a-%d-0", sequence),
						Position: 0,
						Spans: []*ir.Span{{
							ID:            fmt.Sprintf("s-%s", osisID),
							Type:          "VERSE",
							StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
							Ref:           &ir.Ref{Book: doc.ID, Chapter: currentChapter, Verse: verse, OSISID: osisID},
						}},
					}},
				}
				doc.ContentBlocks = append(doc.ContentBlocks, cb)
			}
		}
	}

	return []*ir.Document{doc}
}

func emitMarkdown(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".md")

	// Check for raw markdown for round-trip
	if raw, ok := corpus.Attributes["_markdown_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
			return "", fmt.Errorf("failed to write Markdown: %w", err)
		}
		return outputPath, nil
	}

	// Generate Markdown from IR
	var buf strings.Builder

	// Hugo frontmatter
	buf.WriteString("---\n")
	buf.WriteString(fmt.Sprintf("title: \"%s\"\n", corpus.Title))
	if corpus.Language != "" {
		buf.WriteString(fmt.Sprintf("language: \"%s\"\n", corpus.Language))
	}
	buf.WriteString("date: \"2024-01-01\"\n")
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

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0600); err != nil {
		return "", fmt.Errorf("failed to write Markdown: %w", err)
	}

	return outputPath, nil
}
