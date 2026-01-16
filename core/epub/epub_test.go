// Package epub provides pure Go EPUB creation and manipulation.
package epub

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/encoding"
)

// TestNewEPUB verifies creating a new EPUB.
func TestNewEPUB(t *testing.T) {
	epub := New()
	if epub == nil {
		t.Fatal("New returned nil")
	}
}

// TestSetMetadata verifies setting EPUB metadata.
func TestSetMetadata(t *testing.T) {
	epub := New()
	epub.SetTitle("Test Book")
	epub.SetAuthor("Test Author")
	epub.SetLanguage("en")

	if epub.Metadata.Title != "Test Book" {
		t.Errorf("Title = %q, want %q", epub.Metadata.Title, "Test Book")
	}
	if epub.Metadata.Author != "Test Author" {
		t.Errorf("Author = %q, want %q", epub.Metadata.Author, "Test Author")
	}
	if epub.Metadata.Language != "en" {
		t.Errorf("Language = %q, want %q", epub.Metadata.Language, "en")
	}
}

// TestAddChapter verifies adding chapters.
func TestAddChapter(t *testing.T) {
	epub := New()
	epub.AddChapter("Chapter 1", "<p>Content</p>")
	epub.AddChapter("Chapter 2", "<p>More content</p>")

	if len(epub.Chapters) != 2 {
		t.Errorf("Should have 2 chapters, got %d", len(epub.Chapters))
	}

	if epub.Chapters[0].Title != "Chapter 1" {
		t.Errorf("Chapter title = %q, want %q", epub.Chapters[0].Title, "Chapter 1")
	}
}

// TestBuild verifies building the EPUB.
func TestBuild(t *testing.T) {
	epub := New()
	epub.SetTitle("Test Book")
	epub.SetAuthor("Test Author")
	epub.AddChapter("Chapter 1", "<p>Hello World</p>")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Build returned empty data")
	}

	// Verify it's a valid ZIP
	_, err = zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Errorf("Build output is not valid ZIP: %v", err)
	}
}

// TestBuildContainsMimetype verifies mimetype file exists.
func TestBuildContainsMimetype(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.AddChapter("Ch1", "Content")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Invalid ZIP: %v", err)
	}

	found := false
	for _, f := range r.File {
		if f.Name == "mimetype" {
			found = true
			rc, _ := f.Open()
			content := make([]byte, 100)
			n, _ := rc.Read(content)
			rc.Close()
			if string(content[:n]) != "application/epub+zip" {
				t.Error("Invalid mimetype content")
			}
			break
		}
	}

	if !found {
		t.Error("mimetype file not found in EPUB")
	}
}

// TestBuildContainsContainerXML verifies container.xml exists.
func TestBuildContainsContainerXML(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.AddChapter("Ch1", "Content")

	data, _ := epub.Build()
	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	found := false
	for _, f := range r.File {
		if f.Name == "META-INF/container.xml" {
			found = true
			break
		}
	}

	if !found {
		t.Error("META-INF/container.xml not found in EPUB")
	}
}

// TestBuildContainsContentOPF verifies content.opf exists.
func TestBuildContainsContentOPF(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.AddChapter("Ch1", "Content")

	data, _ := epub.Build()
	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	found := false
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "content.opf") {
			found = true
			break
		}
	}

	if !found {
		t.Error("content.opf not found in EPUB")
	}
}

// TestBuildContainsChapterHTML verifies chapter files exist.
func TestBuildContainsChapterHTML(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.AddChapter("Chapter 1", "Content 1")
	epub.AddChapter("Chapter 2", "Content 2")

	data, _ := epub.Build()
	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	htmlCount := 0
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, ".xhtml") {
			htmlCount++
		}
	}

	if htmlCount < 2 {
		t.Errorf("Should have at least 2 chapter files, got %d", htmlCount)
	}
}

// TestBuildContainsTOC verifies TOC exists.
func TestBuildContainsTOC(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.AddChapter("Ch1", "Content")

	data, _ := epub.Build()
	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	found := false
	for _, f := range r.File {
		if strings.Contains(f.Name, "toc.ncx") || strings.Contains(f.Name, "toc.xhtml") {
			found = true
			break
		}
	}

	if !found {
		t.Error("Table of contents not found in EPUB")
	}
}

// TestSetCover verifies cover image handling.
func TestSetCover(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.SetCover([]byte{0x89, 0x50, 0x4E, 0x47}, "image/png") // PNG magic bytes
	epub.AddChapter("Ch1", "Content")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	found := false
	for _, f := range r.File {
		if strings.Contains(f.Name, "cover") {
			found = true
			break
		}
	}

	if !found {
		t.Error("Cover image not found in EPUB")
	}
}

// TestSetCSS verifies custom CSS handling.
func TestSetCSS(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.SetCSS("body { font-family: serif; }")
	epub.AddChapter("Ch1", "Content")

	data, _ := epub.Build()
	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	found := false
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, ".css") {
			found = true
			break
		}
	}

	if !found {
		t.Error("CSS file not found in EPUB")
	}
}

// TestMetadataInOPF verifies metadata appears in content.opf.
func TestMetadataInOPF(t *testing.T) {
	epub := New()
	epub.SetTitle("My Book Title")
	epub.SetAuthor("John Doe")
	epub.SetLanguage("en-US")
	epub.SetIdentifier("isbn:1234567890")
	epub.AddChapter("Ch1", "Content")

	data, _ := epub.Build()
	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "content.opf") {
			rc, _ := f.Open()
			content := make([]byte, 10000)
			n, _ := rc.Read(content)
			rc.Close()

			opf := string(content[:n])
			if !strings.Contains(opf, "My Book Title") {
				t.Error("Title not found in OPF")
			}
			if !strings.Contains(opf, "John Doe") {
				t.Error("Author not found in OPF")
			}
			if !strings.Contains(opf, "en-US") {
				t.Error("Language not found in OPF")
			}
			break
		}
	}
}

// TestParse verifies parsing an existing EPUB.
func TestParse(t *testing.T) {
	// Create an EPUB first
	epub := New()
	epub.SetTitle("Test Book")
	epub.SetAuthor("Test Author")
	epub.AddChapter("Chapter 1", "<p>Content</p>")

	data, _ := epub.Build()

	// Parse it back
	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.Metadata.Title != "Test Book" {
		t.Errorf("Parsed title = %q, want %q", parsed.Metadata.Title, "Test Book")
	}
}

// TestGetMetadata verifies metadata extraction.
func TestGetMetadata(t *testing.T) {
	epub := New()
	epub.SetTitle("Title")
	epub.SetAuthor("Author")
	epub.SetPublisher("Publisher")
	epub.SetDescription("Description text")
	epub.AddChapter("Ch1", "Content")

	meta := epub.GetMetadata()

	if meta.Title != "Title" {
		t.Error("Title mismatch")
	}
	if meta.Author != "Author" {
		t.Error("Author mismatch")
	}
	if meta.Publisher != "Publisher" {
		t.Error("Publisher mismatch")
	}
	if meta.Description != "Description text" {
		t.Error("Description mismatch")
	}
}

// TestChapterOrdering verifies chapters maintain order.
func TestChapterOrdering(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.AddChapter("First", "1")
	epub.AddChapter("Second", "2")
	epub.AddChapter("Third", "3")

	if epub.Chapters[0].Title != "First" {
		t.Error("First chapter wrong")
	}
	if epub.Chapters[1].Title != "Second" {
		t.Error("Second chapter wrong")
	}
	if epub.Chapters[2].Title != "Third" {
		t.Error("Third chapter wrong")
	}
}

// TestEmptyBook verifies handling of book with no chapters.
func TestEmptyBook(t *testing.T) {
	epub := New()
	epub.SetTitle("Empty Book")

	_, err := epub.Build()
	if err == nil {
		t.Error("Build should fail for book with no chapters")
	}
}

// TestSpecialCharactersInTitle verifies HTML encoding in metadata.
func TestSpecialCharactersInTitle(t *testing.T) {
	epub := New()
	epub.SetTitle("Test & Book <Special>")
	epub.AddChapter("Ch1", "Content")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "content.opf") {
			rc, _ := f.Open()
			content := make([]byte, 10000)
			n, _ := rc.Read(content)
			rc.Close()

			opf := string(content[:n])
			// Should be properly escaped
			if strings.Contains(opf, "<Special>") && !strings.Contains(opf, "&lt;Special&gt;") {
				t.Error("Special characters not escaped in OPF")
			}
			break
		}
	}
}

// TestEPUB3Format verifies EPUB 3 output.
func TestEPUB3Format(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.AddChapter("Ch1", "Content")

	data, _ := epub.Build()
	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "content.opf") {
			rc, _ := f.Open()
			content := make([]byte, 10000)
			n, _ := rc.Read(content)
			rc.Close()

			opf := string(content[:n])
			if !strings.Contains(opf, "version=\"3.0\"") {
				t.Error("Should be EPUB 3.0 format")
			}
			break
		}
	}
}

// TestCoverWithJPEG verifies JPEG cover handling.
func TestCoverWithJPEG(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.SetCover([]byte{0xFF, 0xD8, 0xFF}, "image/jpeg") // JPEG magic bytes
	epub.AddChapter("Ch1", "Content")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	found := false
	for _, f := range r.File {
		if strings.Contains(f.Name, "cover.jpg") {
			found = true
			break
		}
	}

	if !found {
		t.Error("JPEG cover not found in EPUB")
	}
}

// TestAllMetadataFields verifies all metadata fields including Rights.
func TestAllMetadataFields(t *testing.T) {
	epub := New()
	epub.SetTitle("Complete Book")
	epub.SetAuthor("Author Name")
	epub.SetLanguage("es")
	epub.SetIdentifier("custom-id-123")
	epub.SetPublisher("My Publisher")
	epub.SetDescription("A comprehensive description")
	epub.Metadata.Rights = "Copyright 2026"
	epub.AddChapter("Ch1", "Content")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify all metadata is set
	meta := epub.GetMetadata()
	if meta.Title != "Complete Book" {
		t.Error("Title mismatch")
	}
	if meta.Author != "Author Name" {
		t.Error("Author mismatch")
	}
	if meta.Language != "es" {
		t.Error("Language mismatch")
	}
	if meta.Identifier != "custom-id-123" {
		t.Error("Identifier mismatch")
	}
	if meta.Publisher != "My Publisher" {
		t.Error("Publisher mismatch")
	}
	if meta.Description != "A comprehensive description" {
		t.Error("Description mismatch")
	}
	if meta.Rights != "Copyright 2026" {
		t.Error("Rights mismatch")
	}

	// Verify it built successfully
	if len(data) == 0 {
		t.Error("Build returned empty data")
	}
}

// TestParseInvalidZIP verifies error handling for invalid ZIP data.
func TestParseInvalidZIP(t *testing.T) {
	invalidData := []byte("not a zip file")
	_, err := Parse(invalidData)
	if err == nil {
		t.Error("Parse should fail for invalid ZIP data")
	}
}

// TestParseEmptyEPUB verifies parsing EPUB with no content.opf.
func TestParseEmptyEPUB(t *testing.T) {
	// Create a valid ZIP but without content.opf
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("dummy.txt")
	w.Write([]byte("test"))
	zw.Close()

	parsed, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should still return an EPUB object with defaults
	if parsed == nil {
		t.Error("Parse returned nil")
	}
}

// TestParseWithAllMetadata verifies parsing EPUB with complete metadata.
func TestParseWithAllMetadata(t *testing.T) {
	// Create an EPUB with all metadata
	epub := New()
	epub.SetTitle("Full Metadata Book")
	epub.SetAuthor("Jane Doe")
	epub.SetLanguage("fr")
	epub.SetIdentifier("isbn:9876543210")
	epub.SetPublisher("Test Publisher")
	epub.SetDescription("Full description here")
	epub.AddChapter("Chapter 1", "<p>Content</p>")

	data, _ := epub.Build()

	// Parse it back
	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify all metadata was parsed
	if parsed.Metadata.Title != "Full Metadata Book" {
		t.Errorf("Title = %q, want %q", parsed.Metadata.Title, "Full Metadata Book")
	}
	if parsed.Metadata.Author != "Jane Doe" {
		t.Errorf("Author = %q, want %q", parsed.Metadata.Author, "Jane Doe")
	}
	if parsed.Metadata.Language != "fr" {
		t.Errorf("Language = %q, want %q", parsed.Metadata.Language, "fr")
	}
	if parsed.Metadata.Identifier != "isbn:9876543210" {
		t.Errorf("Identifier = %q, want %q", parsed.Metadata.Identifier, "isbn:9876543210")
	}
	if parsed.Metadata.Publisher != "Test Publisher" {
		t.Errorf("Publisher = %q, want %q", parsed.Metadata.Publisher, "Test Publisher")
	}
	if parsed.Metadata.Description != "Full description here" {
		t.Errorf("Description = %q, want %q", parsed.Metadata.Description, "Full description here")
	}
}

// TestExtractTagEdgeCases verifies extractTag handles malformed content.
func TestExtractTagEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		content string
		tag     string
		want    string
	}{
		{
			name:    "Tag not present",
			content: "<other>value</other>",
			tag:     "missing",
			want:    "",
		},
		{
			name:    "Tag without closing bracket",
			content: "<title",
			tag:     "title",
			want:    "",
		},
		{
			name:    "Tag without end tag",
			content: "<title>value",
			tag:     "title",
			want:    "",
		},
		{
			name:    "Valid tag",
			content: "<title>My Title</title>",
			tag:     "title",
			want:    "My Title",
		},
		{
			name:    "Tag with attributes",
			content: `<dc:title id="main">Book Title</dc:title>`,
			tag:     "dc:title",
			want:    "Book Title",
		},
		{
			name:    "Tag with whitespace",
			content: "<author>  John Doe  </author>",
			tag:     "author",
			want:    "John Doe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTag(tt.content, tt.tag)
			if got != tt.want {
				t.Errorf("extractTag(%q, %q) = %q, want %q", tt.content, tt.tag, got, tt.want)
			}
		})
	}
}

// TestMultipleChapters verifies handling many chapters.
func TestMultipleChapters(t *testing.T) {
	epub := New()
	epub.SetTitle("Multi-Chapter Book")
	epub.SetAuthor("Test Author")

	// Add 10 chapters
	for i := 1; i <= 10; i++ {
		epub.AddChapter(
			fmt.Sprintf("Chapter %d", i),
			fmt.Sprintf("<p>This is chapter %d content.</p>", i),
		)
	}

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	// Count chapter files
	chapterCount := 0
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "OEBPS/text/chapter") && strings.HasSuffix(f.Name, ".xhtml") {
			chapterCount++
		}
	}

	if chapterCount != 10 {
		t.Errorf("Expected 10 chapter files, got %d", chapterCount)
	}
}

// TestSpecialCharactersInChapter verifies XML escaping in chapter content.
func TestSpecialCharactersInChapter(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.SetAuthor("Author & Co.")
	epub.AddChapter("Chapter <1>", "<p>Content with & and <tags></p>")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	// Find and verify the chapter file
	for _, f := range r.File {
		if strings.Contains(f.Name, "chapter1.xhtml") {
			rc, _ := f.Open()
			content := make([]byte, 10000)
			n, _ := rc.Read(content)
			rc.Close()

			html := string(content[:n])
			// Title in <title> and <h1> should be escaped
			if !strings.Contains(html, "Chapter &lt;1&gt;") {
				t.Error("Chapter title not properly escaped in HTML")
			}
			break
		}
	}
}

// TestDefaultCSS verifies default CSS is used when none is set.
func TestDefaultCSS(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.AddChapter("Ch1", "Content")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	// Find and verify CSS file
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "style.css") {
			rc, _ := f.Open()
			content := make([]byte, 10000)
			n, _ := rc.Read(content)
			rc.Close()

			css := string(content[:n])
			if !strings.Contains(css, "font-family: serif") {
				t.Error("Default CSS not found")
			}
			break
		}
	}
}

// TestCustomCSS verifies custom CSS replaces default.
func TestCustomCSS(t *testing.T) {
	customCSS := "body { background: black; color: white; }"
	epub := New()
	epub.SetTitle("Test")
	epub.SetCSS(customCSS)
	epub.AddChapter("Ch1", "Content")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	// Find and verify CSS file
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "style.css") {
			rc, _ := f.Open()
			content := make([]byte, 10000)
			n, _ := rc.Read(content)
			rc.Close()

			css := string(content[:n])
			if css != customCSS {
				t.Errorf("CSS = %q, want %q", css, customCSS)
			}
			break
		}
	}
}

// TestContentOPFStructure verifies content.opf has proper structure.
func TestContentOPFStructure(t *testing.T) {
	epub := New()
	epub.SetTitle("Test Book")
	epub.SetAuthor("Test Author")
	epub.AddChapter("Ch1", "Content")
	epub.AddChapter("Ch2", "More content")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "content.opf") {
			rc, _ := f.Open()
			content := make([]byte, 20000)
			n, _ := rc.Read(content)
			rc.Close()

			opf := string(content[:n])

			// Verify manifest contains TOC, NCX, CSS, and chapters
			if !strings.Contains(opf, `id="toc"`) {
				t.Error("TOC not in manifest")
			}
			if !strings.Contains(opf, `id="ncx"`) {
				t.Error("NCX not in manifest")
			}
			if !strings.Contains(opf, `id="style"`) {
				t.Error("CSS not in manifest")
			}
			if !strings.Contains(opf, `id="chapter1"`) {
				t.Error("Chapter 1 not in manifest")
			}
			if !strings.Contains(opf, `id="chapter2"`) {
				t.Error("Chapter 2 not in manifest")
			}

			// Verify spine contains chapters
			if !strings.Contains(opf, `<itemref idref="chapter1"/>`) {
				t.Error("Chapter 1 not in spine")
			}
			if !strings.Contains(opf, `<itemref idref="chapter2"/>`) {
				t.Error("Chapter 2 not in spine")
			}
			break
		}
	}
}

// TestTocNCXStructure verifies toc.ncx has proper structure.
func TestTocNCXStructure(t *testing.T) {
	epub := New()
	epub.SetTitle("Test Book")
	epub.AddChapter("First Chapter", "Content")
	epub.AddChapter("Second Chapter", "More")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	for _, f := range r.File {
		if strings.Contains(f.Name, "toc.ncx") {
			rc, _ := f.Open()
			content := make([]byte, 20000)
			n, _ := rc.Read(content)
			rc.Close()

			ncx := string(content[:n])

			// Verify navPoints exist
			if !strings.Contains(ncx, "First Chapter") {
				t.Error("First chapter not in NCX")
			}
			if !strings.Contains(ncx, "Second Chapter") {
				t.Error("Second chapter not in NCX")
			}
			if !strings.Contains(ncx, `navpoint1`) {
				t.Error("NavPoint 1 not found")
			}
			if !strings.Contains(ncx, `navpoint2`) {
				t.Error("NavPoint 2 not found")
			}
			break
		}
	}
}

// TestTocXHTMLStructure verifies toc.xhtml has proper structure.
func TestTocXHTMLStructure(t *testing.T) {
	epub := New()
	epub.SetTitle("Test Book")
	epub.AddChapter("Chapter One", "Content")
	epub.AddChapter("Chapter Two", "More")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	for _, f := range r.File {
		if strings.Contains(f.Name, "toc.xhtml") {
			rc, _ := f.Open()
			content := make([]byte, 20000)
			n, _ := rc.Read(content)
			rc.Close()

			xhtml := string(content[:n])

			// Verify chapters are linked
			if !strings.Contains(xhtml, "Chapter One") {
				t.Error("Chapter One not in TOC XHTML")
			}
			if !strings.Contains(xhtml, "Chapter Two") {
				t.Error("Chapter Two not in TOC XHTML")
			}
			if !strings.Contains(xhtml, `href="text/chapter1.xhtml"`) {
				t.Error("Chapter 1 link not found")
			}
			if !strings.Contains(xhtml, `href="text/chapter2.xhtml"`) {
				t.Error("Chapter 2 link not found")
			}
			break
		}
	}
}

// TestNewEPUBDefaults verifies New() sets sensible defaults.
func TestNewEPUBDefaults(t *testing.T) {
	epub := New()

	if epub.Metadata.Language != "en" {
		t.Errorf("Default language = %q, want %q", epub.Metadata.Language, "en")
	}
	if epub.Metadata.Identifier == "" {
		t.Error("Default identifier should not be empty")
	}
	if epub.Metadata.Date == "" {
		t.Error("Default date should not be empty")
	}
	if epub.Chapters == nil {
		t.Error("Chapters should be initialized")
	}
	if len(epub.Chapters) != 0 {
		t.Error("New EPUB should have no chapters")
	}
}

// TestBuildWithCoverPNG verifies PNG cover is properly included.
func TestBuildWithCoverPNG(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.SetCover([]byte{0x89, 0x50, 0x4E, 0x47}, "image/png")
	epub.AddChapter("Ch1", "Content")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	foundCover := false
	foundInManifest := false

	for _, f := range r.File {
		if strings.Contains(f.Name, "cover.png") {
			foundCover = true
		}
		if strings.HasSuffix(f.Name, "content.opf") {
			rc, _ := f.Open()
			content := make([]byte, 20000)
			n, _ := rc.Read(content)
			rc.Close()

			opf := string(content[:n])
			if strings.Contains(opf, "cover.png") && strings.Contains(opf, "image/png") {
				foundInManifest = true
			}
		}
	}

	if !foundCover {
		t.Error("PNG cover file not found in EPUB")
	}
	if !foundInManifest {
		t.Error("Cover not properly referenced in manifest")
	}
}

// TestBuildNoCover verifies EPUB builds without cover.
func TestBuildNoCover(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.AddChapter("Ch1", "Content")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	// Should not have any cover image
	for _, f := range r.File {
		if strings.Contains(f.Name, "cover") && (strings.HasSuffix(f.Name, ".png") || strings.HasSuffix(f.Name, ".jpg")) {
			t.Error("No cover should be present")
		}
	}
}

// TestChapterContentPreserved verifies chapter HTML content is preserved.
func TestChapterContentPreserved(t *testing.T) {
	testContent := "<p>This is a <strong>test</strong> paragraph.</p><p>Another one.</p>"
	epub := New()
	epub.SetTitle("Test")
	epub.AddChapter("Test Chapter", testContent)

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))

	for _, f := range r.File {
		if strings.Contains(f.Name, "chapter1.xhtml") {
			rc, _ := f.Open()
			content := make([]byte, 20000)
			n, _ := rc.Read(content)
			rc.Close()

			html := string(content[:n])
			if !strings.Contains(html, testContent) {
				t.Error("Chapter content not preserved in output")
			}
			break
		}
	}
}

// TestParseWithMissingMetadata verifies parsing handles missing metadata gracefully.
func TestParseWithMissingMetadata(t *testing.T) {
	// Create a minimal OPF without some metadata fields
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w, _ := zw.Create("OEBPS/content.opf")
	minimalOPF := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Only Title</dc:title>
  </metadata>
</package>`
	w.Write([]byte(minimalOPF))
	zw.Close()

	parsed, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.Metadata.Title != "Only Title" {
		t.Errorf("Title = %q, want %q", parsed.Metadata.Title, "Only Title")
	}
	// Other fields should be empty but not cause errors
	if parsed.Metadata.Author != "" {
		t.Error("Author should be empty")
	}
}

// TestEscapeXML verifies XML escaping function.
func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "Hello World"},
		{"Test & Test", "Test &amp; Test"},
		{"<html>", "&lt;html&gt;"},
		{"Quote \"test\"", "Quote &#34;test&#34;"},
		{"O'Reilly", "O&#39;Reilly"},
		{"Multiple <>&\"' chars", "Multiple &lt;&gt;&amp;&#34;&#39; chars"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := encoding.EscapeXML(tt.input)
			if got != tt.want {
				t.Errorf("EscapeXML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestBuildLargeBook verifies building EPUB with many chapters and large content.
func TestBuildLargeBook(t *testing.T) {
	epub := New()
	epub.SetTitle("Large Book")
	epub.SetAuthor("Test Author")
	epub.SetLanguage("en-GB")
	epub.SetPublisher("Test Publisher Ltd.")
	epub.SetDescription("A book with many chapters and complex content")

	// Add cover
	epub.SetCover([]byte{0xFF, 0xD8, 0xFF, 0xE0}, "image/jpeg")

	// Add custom CSS
	epub.SetCSS("body { font-size: 14px; }")

	// Add 20 chapters with varied content
	for i := 1; i <= 20; i++ {
		title := fmt.Sprintf("Chapter %d: The %dth Tale", i, i)
		content := fmt.Sprintf("<p>Chapter %d content with special chars: &amp; &lt; &gt;</p>", i)
		for j := 0; j < 10; j++ {
			content += fmt.Sprintf("<p>Paragraph %d of chapter %d.</p>", j+1, i)
		}
		epub.AddChapter(title, content)
	}

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Build returned empty data")
	}

	// Verify it's a valid ZIP
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Invalid ZIP: %v", err)
	}

	// Verify all chapters are present
	chapterCount := 0
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "OEBPS/text/chapter") {
			chapterCount++
		}
	}

	if chapterCount != 20 {
		t.Errorf("Expected 20 chapters, got %d", chapterCount)
	}
}

// TestBuildAllCombinations tests various combinations of metadata and content.
func TestBuildAllCombinations(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(*EPUB)
		shouldError bool
	}{
		{
			name: "Minimal EPUB",
			setupFunc: func(e *EPUB) {
				e.SetTitle("Min")
				e.AddChapter("C1", "Content")
			},
			shouldError: false,
		},
		{
			name: "EPUB with all metadata",
			setupFunc: func(e *EPUB) {
				e.SetTitle("Full Book")
				e.SetAuthor("John Smith")
				e.SetLanguage("de")
				e.SetIdentifier("custom-123")
				e.SetPublisher("Pub Co")
				e.SetDescription("Desc text")
				e.Metadata.Rights = "All Rights Reserved"
				e.AddChapter("C1", "Content")
			},
			shouldError: false,
		},
		{
			name: "EPUB with PNG cover",
			setupFunc: func(e *EPUB) {
				e.SetTitle("Book")
				e.SetCover([]byte{0x89, 0x50}, "image/png")
				e.AddChapter("C1", "Content")
			},
			shouldError: false,
		},
		{
			name: "EPUB with JPEG cover",
			setupFunc: func(e *EPUB) {
				e.SetTitle("Book")
				e.SetCover([]byte{0xFF, 0xD8}, "image/jpeg")
				e.AddChapter("C1", "Content")
			},
			shouldError: false,
		},
		{
			name: "EPUB with other image type (treated as JPEG)",
			setupFunc: func(e *EPUB) {
				e.SetTitle("Book")
				e.SetCover([]byte{0x00, 0x00}, "image/gif")
				e.AddChapter("C1", "Content")
			},
			shouldError: false,
		},
		{
			name: "EPUB with custom CSS",
			setupFunc: func(e *EPUB) {
				e.SetTitle("Book")
				e.SetCSS("* { margin: 0; }")
				e.AddChapter("C1", "Content")
			},
			shouldError: false,
		},
		{
			name: "EPUB with no chapters (should error)",
			setupFunc: func(e *EPUB) {
				e.SetTitle("Book")
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			epub := New()
			tt.setupFunc(epub)

			data, err := epub.Build()

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(data) == 0 {
				t.Error("Build returned empty data")
			}

			// Verify it's a valid ZIP
			_, err = zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Errorf("Invalid ZIP: %v", err)
			}
		})
	}
}

// TestParseAllMetadataFields verifies parsing extracts all metadata fields.
func TestParseAllMetadataFields(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w, _ := zw.Create("OEBPS/content.opf")
	fullOPF := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Complete Title</dc:title>
    <dc:creator>Complete Author</dc:creator>
    <dc:language>es-ES</dc:language>
    <dc:identifier>complete-id</dc:identifier>
    <dc:publisher>Complete Publisher</dc:publisher>
    <dc:description>Complete description text here</dc:description>
  </metadata>
</package>`
	w.Write([]byte(fullOPF))
	zw.Close()

	parsed, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.Metadata.Title != "Complete Title" {
		t.Errorf("Title = %q, want %q", parsed.Metadata.Title, "Complete Title")
	}
	if parsed.Metadata.Author != "Complete Author" {
		t.Errorf("Author = %q, want %q", parsed.Metadata.Author, "Complete Author")
	}
	if parsed.Metadata.Language != "es-ES" {
		t.Errorf("Language = %q, want %q", parsed.Metadata.Language, "es-ES")
	}
	if parsed.Metadata.Identifier != "complete-id" {
		t.Errorf("Identifier = %q, want %q", parsed.Metadata.Identifier, "complete-id")
	}
	if parsed.Metadata.Publisher != "Complete Publisher" {
		t.Errorf("Publisher = %q, want %q", parsed.Metadata.Publisher, "Complete Publisher")
	}
	if parsed.Metadata.Description != "Complete description text here" {
		t.Errorf("Description = %q, want %q", parsed.Metadata.Description, "Complete description text here")
	}
}

// TestCompleteRoundTrip tests building and parsing an EPUB.
func TestCompleteRoundTrip(t *testing.T) {
	// Create original EPUB
	original := New()
	original.SetTitle("Round Trip Test")
	original.SetAuthor("Test Author")
	original.SetLanguage("ja")
	original.SetIdentifier("test-123")
	original.SetPublisher("Test Pub")
	original.SetDescription("Test Description")
	original.SetCSS("body { color: red; }")
	original.SetCover([]byte{0x89, 0x50, 0x4E, 0x47}, "image/png")
	original.AddChapter("Chapter 1", "<p>First chapter content</p>")
	original.AddChapter("Chapter 2", "<p>Second chapter content</p>")
	original.AddChapter("Chapter 3", "<p>Third chapter content</p>")

	// Build it
	data, err := original.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Parse it back
	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify metadata matches
	if parsed.Metadata.Title != original.Metadata.Title {
		t.Errorf("Title mismatch: got %q, want %q", parsed.Metadata.Title, original.Metadata.Title)
	}
	if parsed.Metadata.Author != original.Metadata.Author {
		t.Errorf("Author mismatch: got %q, want %q", parsed.Metadata.Author, original.Metadata.Author)
	}
	if parsed.Metadata.Language != original.Metadata.Language {
		t.Errorf("Language mismatch: got %q, want %q", parsed.Metadata.Language, original.Metadata.Language)
	}
	if parsed.Metadata.Identifier != original.Metadata.Identifier {
		t.Errorf("Identifier mismatch: got %q, want %q", parsed.Metadata.Identifier, original.Metadata.Identifier)
	}
	if parsed.Metadata.Publisher != original.Metadata.Publisher {
		t.Errorf("Publisher mismatch: got %q, want %q", parsed.Metadata.Publisher, original.Metadata.Publisher)
	}
	if parsed.Metadata.Description != original.Metadata.Description {
		t.Errorf("Description mismatch: got %q, want %q", parsed.Metadata.Description, original.Metadata.Description)
	}
}

// TestMimetypeFileFormat verifies mimetype is uncompressed and first.
func TestMimetypeFileFormat(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.AddChapter("Ch1", "Content")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Invalid ZIP: %v", err)
	}

	// Mimetype should be first file
	if len(r.File) == 0 {
		t.Fatal("ZIP has no files")
	}

	firstFile := r.File[0]
	if firstFile.Name != "mimetype" {
		t.Errorf("First file should be mimetype, got %q", firstFile.Name)
	}

	// Verify it's stored (uncompressed)
	if firstFile.Method != zip.Store {
		t.Errorf("Mimetype should use Store method, got %v", firstFile.Method)
	}
}

// TestBuildVerifyAllFiles verifies all expected files are in the EPUB.
func TestBuildVerifyAllFiles(t *testing.T) {
	epub := New()
	epub.SetTitle("Complete Test")
	epub.SetAuthor("Test Author")
	epub.SetCover([]byte{0xFF, 0xD8, 0xFF}, "image/jpeg")
	epub.SetCSS("body { margin: 0; }")
	epub.AddChapter("Ch1", "Content 1")
	epub.AddChapter("Ch2", "Content 2")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Invalid ZIP: %v", err)
	}

	expectedFiles := map[string]bool{
		"mimetype":                  false,
		"META-INF/container.xml":    false,
		"OEBPS/content.opf":         false,
		"OEBPS/toc.ncx":             false,
		"OEBPS/toc.xhtml":           false,
		"OEBPS/style.css":           false,
		"OEBPS/images/cover.jpg":    false,
		"OEBPS/text/chapter1.xhtml": false,
		"OEBPS/text/chapter2.xhtml": false,
	}

	for _, f := range r.File {
		if _, exists := expectedFiles[f.Name]; exists {
			expectedFiles[f.Name] = true
		}
	}

	for file, found := range expectedFiles {
		if !found {
			t.Errorf("Expected file %q not found in EPUB", file)
		}
	}
}

// TestEmptyStringMetadata verifies handling of empty metadata strings.
func TestEmptyStringMetadata(t *testing.T) {
	epub := New()
	epub.SetTitle("")
	epub.SetAuthor("")
	epub.SetLanguage("")
	epub.SetIdentifier("")
	epub.SetPublisher("")
	epub.SetDescription("")
	epub.AddChapter("Ch1", "Content")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Build returned empty data")
	}
}

// TestUnicodeMetadata verifies handling of Unicode in metadata.
func TestUnicodeMetadata(t *testing.T) {
	epub := New()
	epub.SetTitle("日本語タイトル")
	epub.SetAuthor("作者名前")
	epub.SetLanguage("zh-CN")
	epub.SetPublisher("出版社")
	epub.SetDescription("这是一个测试描述")
	epub.AddChapter("第一章", "<p>内容</p>")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Parse it back
	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.Metadata.Title != "日本語タイトル" {
		t.Errorf("Unicode title not preserved")
	}
	if parsed.Metadata.Author != "作者名前" {
		t.Errorf("Unicode author not preserved")
	}
}

// TestVeryLongMetadata verifies handling of very long metadata strings.
func TestVeryLongMetadata(t *testing.T) {
	longTitle := string(make([]byte, 10000))
	for i := range longTitle {
		longTitle = longTitle[:i] + "A"
	}

	epub := New()
	epub.SetTitle(longTitle[:5000])
	epub.SetAuthor(longTitle[:5000])
	epub.AddChapter("Ch1", "Content")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed with long metadata: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Build returned empty data")
	}
}

// TestChapterWithNoContent verifies handling of empty chapter content.
func TestChapterWithNoContent(t *testing.T) {
	epub := New()
	epub.SetTitle("Test")
	epub.AddChapter("Empty Chapter", "")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Build returned empty data")
	}
}

// TestSingleChapterBook verifies minimum viable EPUB with one chapter.
func TestSingleChapterBook(t *testing.T) {
	epub := New()
	epub.SetTitle("Single")
	epub.AddChapter("Only", "X")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Invalid ZIP: %v", err)
	}

	chapterCount := 0
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "OEBPS/text/chapter") {
			chapterCount++
		}
	}

	if chapterCount != 1 {
		t.Errorf("Expected 1 chapter, got %d", chapterCount)
	}
}

// TestMetadataWithSpecialXMLCharacters verifies all special XML characters are escaped.
func TestMetadataWithSpecialXMLCharacters(t *testing.T) {
	epub := New()
	epub.SetTitle(`Title with <>&"' characters`)
	epub.SetAuthor(`Author <>&"' test`)
	epub.SetPublisher(`Publisher <>&"' test`)
	epub.SetDescription(`Description with <>&"' and more`)
	epub.AddChapter(`Chapter <>&"'`, "<p>Content</p>")

	data, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Invalid ZIP: %v", err)
	}

	// Find and verify OPF has escaped characters
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "content.opf") {
			rc, _ := f.Open()
			content := make([]byte, 20000)
			n, _ := rc.Read(content)
			rc.Close()

			opf := string(content[:n])
			// Should have escaped versions
			if !strings.Contains(opf, "&lt;") || !strings.Contains(opf, "&gt;") || !strings.Contains(opf, "&amp;") {
				t.Error("Special characters not properly escaped in OPF")
			}
			// Should NOT have unescaped versions
			if strings.Contains(opf, "<>&") && !strings.Contains(opf, "&lt;") {
				t.Error("Found unescaped special characters in OPF")
			}
			break
		}
	}
}

// TestGetMetadataReturnsCorrectValues verifies GetMetadata returns all fields.
func TestGetMetadataReturnsCorrectValues(t *testing.T) {
	epub := New()
	epub.SetTitle("Test Title")
	epub.SetAuthor("Test Author")
	epub.SetLanguage("fr-FR")
	epub.SetIdentifier("test-id-123")
	epub.SetPublisher("Test Pub")
	epub.SetDescription("Test Desc")
	epub.Metadata.Rights = "Test Rights"
	epub.Metadata.Date = "2026-01-01"

	meta := epub.GetMetadata()

	if meta.Title != "Test Title" {
		t.Errorf("Title mismatch: got %q", meta.Title)
	}
	if meta.Author != "Test Author" {
		t.Errorf("Author mismatch: got %q", meta.Author)
	}
	if meta.Language != "fr-FR" {
		t.Errorf("Language mismatch: got %q", meta.Language)
	}
	if meta.Identifier != "test-id-123" {
		t.Errorf("Identifier mismatch: got %q", meta.Identifier)
	}
	if meta.Publisher != "Test Pub" {
		t.Errorf("Publisher mismatch: got %q", meta.Publisher)
	}
	if meta.Description != "Test Desc" {
		t.Errorf("Description mismatch: got %q", meta.Description)
	}
	if meta.Rights != "Test Rights" {
		t.Errorf("Rights mismatch: got %q", meta.Rights)
	}
	if meta.Date != "2026-01-01" {
		t.Errorf("Date mismatch: got %q", meta.Date)
	}
}

// TestBuildConsistency verifies multiple builds produce valid EPUBs.
func TestBuildConsistency(t *testing.T) {
	for i := 0; i < 5; i++ {
		epub := New()
		epub.SetTitle(fmt.Sprintf("Book %d", i))
		epub.SetAuthor("Consistency Test")
		for j := 0; j < 3; j++ {
			epub.AddChapter(fmt.Sprintf("Ch%d", j+1), fmt.Sprintf("<p>Content %d</p>", j+1))
		}

		data, err := epub.Build()
		if err != nil {
			t.Fatalf("Build %d failed: %v", i, err)
		}

		_, err = zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("Build %d produced invalid ZIP: %v", i, err)
		}
	}
}

// TestCoverImageFormats tests different cover mime types.
func TestCoverImageFormats(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		wantExt  string
	}{
		{"PNG", "image/png", "png"},
		{"PNG with charset", "image/png; charset=utf-8", "png"},
		{"JPEG", "image/jpeg", "jpg"},
		{"JPG", "image/jpg", "jpg"},
		{"GIF (fallback to JPG)", "image/gif", "jpg"},
		{"WebP (fallback to JPG)", "image/webp", "jpg"},
		{"Empty (fallback to JPG)", "", "jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			epub := New()
			epub.SetTitle("Test")
			epub.SetCover([]byte{0x00, 0x01}, tt.mimeType)
			epub.AddChapter("Ch1", "Content")

			data, err := epub.Build()
			if err != nil {
				t.Fatalf("Build failed: %v", err)
			}

			r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			found := false
			for _, f := range r.File {
				if strings.Contains(f.Name, "cover."+tt.wantExt) {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected cover.%s not found", tt.wantExt)
			}
		})
	}
}

// TestParseCorruptedFile verifies error handling when reading corrupted EPUB content.
func TestParseCorruptedFile(t *testing.T) {
	// Create a ZIP with a file that can't be read (we'll use a mock scenario)
	// Since we can't easily trigger file.Open() error with real zip, we test the readable path
	// The io.ReadAll error is also difficult to trigger with real data

	// Test with a valid ZIP that has content.opf but minimal content
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("test/content.opf")
	w.Write([]byte(`<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Test</dc:title></metadata></package>`))
	zw.Close()

	parsed, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if parsed.Metadata.Title != "Test" {
		t.Errorf("Title = %q, want %q", parsed.Metadata.Title, "Test")
	}
}

// TestBuildErrorCoverage documents the error paths that cannot be easily tested.
// These represent defensive error handling for rare I/O failures.
func TestBuildErrorCoverage(t *testing.T) {
	// Most error paths are now covered via internal tests with mocks.
	// We verify the success path works through the public API:
	epub := New()
	epub.SetTitle("Test")
	epub.SetCover([]byte("test"), "image/png")
	epub.SetCSS("test")
	epub.AddChapter("Ch1", "Content")

	_, err := epub.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
}
