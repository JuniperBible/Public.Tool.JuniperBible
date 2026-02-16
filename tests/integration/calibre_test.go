// Calibre tool integration tests.
// These tests require Calibre's ebook-convert to be installed.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCalibreAvailable checks if Calibre is installed.
func TestCalibreAvailable(t *testing.T) {
	if !HasTool(ToolCalibre) {
		t.Skip("calibre (ebook-convert) not installed")
	}

	output, err := RunTool(t, ToolCalibre, "--version")
	if err != nil {
		t.Fatalf("ebook-convert --version failed: %v", err)
	}

	if !strings.Contains(output, "ebook-convert") && !strings.Contains(output, "calibre") {
		t.Errorf("unexpected calibre output: %s", output)
	}

	t.Logf("calibre version: %s", strings.TrimSpace(output))
}

// TestCalibreListFormats tests listing supported e-book formats.
func TestCalibreListFormats(t *testing.T) {
	RequireTool(t, ToolCalibre)

	// ebook-convert requires at least one argument
	// Use --help to get format information
	cmd := exec.Command("ebook-convert", "--help")
	output, _ := cmd.CombinedOutput() // May exit non-zero

	outputStr := string(output)

	// Check for common format mentions
	formats := []string{"epub", "mobi", "pdf", "azw3", "docx", "html"}
	found := 0
	for _, fmt := range formats {
		if strings.Contains(strings.ToLower(outputStr), fmt) {
			found++
		}
	}

	if found < 3 {
		t.Logf("calibre help output: %s", outputStr[:min(500, len(outputStr))])
	}

	t.Logf("calibre mentions at least %d common formats", found)
}

// TestCalibreHTMLToEPUB tests converting HTML to EPUB.
func TestCalibreHTMLToEPUB(t *testing.T) {
	RequireTool(t, ToolCalibre)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "calibre-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write HTML file
	htmlContent := `<!DOCTYPE html>
<html>
<head>
    <title>Test Book</title>
    <meta name="author" content="Test Author">
</head>
<body>
    <h1>Chapter One</h1>
    <p>This is the first chapter of our test book.</p>
    <p>It has multiple paragraphs.</p>

    <h1>Chapter Two</h1>
    <p>This is the second chapter.</p>
</body>
</html>`

	htmlPath := filepath.Join(tempDir, "book.html")
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0600); err != nil {
		t.Fatalf("failed to write HTML: %v", err)
	}

	// Convert to EPUB
	epubPath := filepath.Join(tempDir, "book.epub")
	cmd := exec.Command("ebook-convert", htmlPath, epubPath,
		"--title", "Test Book",
		"--authors", "Test Author",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ebook-convert failed: %v\nOutput: %s", err, output)
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

// TestCalibreEPUBToMOBI tests converting EPUB to MOBI.
func TestCalibreEPUBToMOBI(t *testing.T) {
	RequireTool(t, ToolCalibre)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "calibre-mobi-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// First create an EPUB
	htmlContent := `<!DOCTYPE html>
<html>
<head><title>MOBI Test</title></head>
<body><h1>Test</h1><p>Content</p></body>
</html>`

	htmlPath := filepath.Join(tempDir, "book.html")
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0600); err != nil {
		t.Fatalf("failed to write HTML: %v", err)
	}

	epubPath := filepath.Join(tempDir, "book.epub")
	cmd := exec.Command("ebook-convert", htmlPath, epubPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create EPUB: %v\nOutput: %s", err, output)
	}

	// Convert EPUB to MOBI
	mobiPath := filepath.Join(tempDir, "book.mobi")
	cmd = exec.Command("ebook-convert", epubPath, mobiPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("EPUB to MOBI conversion failed: %v\nOutput: %s", err, output)
	}

	// Verify MOBI was created
	info, err := os.Stat(mobiPath)
	if err != nil {
		t.Fatalf("MOBI not created: %v", err)
	}

	t.Logf("successfully created MOBI (%d bytes)", info.Size())
}

// TestCalibreMetadata tests extracting e-book metadata.
func TestCalibreMetadata(t *testing.T) {
	RequireTool(t, ToolCalibre)

	// Check if ebook-meta is available
	ebookMeta := Tool{
		Name:    "ebook-meta",
		Command: "ebook-meta",
		Args:    []string{"--help"},
	}
	if !HasTool(ebookMeta) {
		t.Skip("ebook-meta not installed")
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "calibre-meta-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an EPUB with metadata
	htmlContent := `<!DOCTYPE html>
<html>
<head><title>Metadata Test</title></head>
<body><p>Content</p></body>
</html>`

	htmlPath := filepath.Join(tempDir, "book.html")
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0600); err != nil {
		t.Fatalf("failed to write HTML: %v", err)
	}

	epubPath := filepath.Join(tempDir, "book.epub")
	cmd := exec.Command("ebook-convert", htmlPath, epubPath,
		"--title", "Metadata Test Book",
		"--authors", "John Doe",
		"--publisher", "Test Publisher",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create EPUB: %v\nOutput: %s", err, output)
	}

	// Extract metadata
	cmd = exec.Command("ebook-meta", epubPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ebook-meta failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Metadata Test Book") {
		t.Errorf("title not found in metadata: %s", outputStr)
	}
	if !strings.Contains(outputStr, "John Doe") {
		t.Errorf("author not found in metadata: %s", outputStr)
	}

	t.Log("successfully extracted e-book metadata")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
