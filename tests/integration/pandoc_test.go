// Pandoc tool integration tests.
// These tests require pandoc to be installed.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPandocAvailable checks if pandoc is installed.
func TestPandocAvailable(t *testing.T) {
	if !HasTool(ToolPandoc) {
		t.Skip("pandoc not installed")
	}

	output, err := RunTool(t, ToolPandoc, "--version")
	if err != nil {
		t.Fatalf("pandoc --version failed: %v", err)
	}

	if !strings.Contains(output, "pandoc") {
		t.Errorf("unexpected pandoc output: %s", output)
	}

	t.Logf("pandoc version: %s", strings.Split(output, "\n")[0])
}

// TestPandocListFormats tests listing supported formats.
func TestPandocListFormats(t *testing.T) {
	RequireTool(t, ToolPandoc)

	// List input formats
	inputOutput, err := RunTool(t, ToolPandoc, "--list-input-formats")
	if err != nil {
		t.Fatalf("failed to list input formats: %v", err)
	}

	inputFormats := strings.Split(strings.TrimSpace(inputOutput), "\n")
	t.Logf("pandoc supports %d input formats", len(inputFormats))

	// List output formats
	outputOutput, err := RunTool(t, ToolPandoc, "--list-output-formats")
	if err != nil {
		t.Fatalf("failed to list output formats: %v", err)
	}

	outputFormats := strings.Split(strings.TrimSpace(outputOutput), "\n")
	t.Logf("pandoc supports %d output formats", len(outputFormats))

	// Check for expected formats
	expectedInputs := []string{"markdown", "html", "docx", "epub"}
	for _, fmt := range expectedInputs {
		found := false
		for _, f := range inputFormats {
			if f == fmt {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected input format %s not found", fmt)
		}
	}
}

// TestPandocMarkdownToHTML tests converting Markdown to HTML.
func TestPandocMarkdownToHTML(t *testing.T) {
	RequireTool(t, ToolPandoc)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "pandoc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write Markdown file
	mdContent := `# Test Document

This is a test paragraph with **bold** and *italic* text.

## Section One

- Item 1
- Item 2
- Item 3

## Section Two

> This is a blockquote.

` + "```go\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```"

	mdPath := filepath.Join(tempDir, "test.md")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0600); err != nil {
		t.Fatalf("failed to write markdown: %v", err)
	}

	// Convert to HTML
	htmlPath := filepath.Join(tempDir, "test.html")
	cmd := exec.Command("pandoc", "-f", "markdown", "-t", "html", "-o", htmlPath, mdPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pandoc conversion failed: %v\nOutput: %s", err, output)
	}

	// Read and verify HTML
	htmlContent, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("failed to read HTML: %v", err)
	}

	html := string(htmlContent)
	checks := []string{
		"<h1",
		"<h2",
		"<strong>bold</strong>",
		"<em>italic</em>",
		"<li>",
		"<blockquote>",
	}

	for _, check := range checks {
		if !strings.Contains(html, check) {
			t.Errorf("HTML missing expected element: %s", check)
		}
	}

	t.Logf("successfully converted markdown to HTML (%d bytes)", len(htmlContent))
}

// TestPandocMarkdownToEPUB tests creating an EPUB from Markdown.
func TestPandocMarkdownToEPUB(t *testing.T) {
	RequireTool(t, ToolPandoc)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "pandoc-epub-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write Markdown file
	mdContent := `---
title: Test Book
author: Test Author
---

# Chapter One

This is the first chapter of our test book.

# Chapter Two

This is the second chapter.
`

	mdPath := filepath.Join(tempDir, "book.md")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0600); err != nil {
		t.Fatalf("failed to write markdown: %v", err)
	}

	// Convert to EPUB
	epubPath := filepath.Join(tempDir, "book.epub")
	cmd := exec.Command("pandoc", "-f", "markdown", "-t", "epub", "-o", epubPath, mdPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pandoc EPUB conversion failed: %v\nOutput: %s", err, output)
	}

	// Verify EPUB was created
	info, err := os.Stat(epubPath)
	if err != nil {
		t.Fatalf("EPUB not created: %v", err)
	}

	if info.Size() < 1000 {
		t.Errorf("EPUB seems too small: %d bytes", info.Size())
	}

	t.Logf("successfully created EPUB (%d bytes)", info.Size())
}

// TestPandocExtractMetadata tests extracting document metadata.
func TestPandocExtractMetadata(t *testing.T) {
	RequireTool(t, ToolPandoc)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "pandoc-meta-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write Markdown with YAML frontmatter
	mdContent := `---
title: "Test Title"
author: "Test Author"
date: "2024-01-01"
keywords:
  - test
  - metadata
---

Content goes here.
`

	mdPath := filepath.Join(tempDir, "doc.md")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0600); err != nil {
		t.Fatalf("failed to write markdown: %v", err)
	}

	// Extract metadata template
	templatePath := filepath.Join(tempDir, "meta.tpl")
	template := `$title$
$author$
$date$`
	if err := os.WriteFile(templatePath, []byte(template), 0600); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	// Run pandoc with template
	cmd := exec.Command("pandoc", "--template", templatePath, "-t", "plain", mdPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pandoc metadata extraction failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Test Title") {
		t.Errorf("title not found in metadata output: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Test Author") {
		t.Errorf("author not found in metadata output: %s", outputStr)
	}

	t.Log("successfully extracted metadata")
}
