package html

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

func TestDetect_ValidHtmlFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.html")

	if err := os.WriteFile(testFile, []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)

	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if !result.Detected {
		t.Errorf("Expected Detected=true, got false. Reason: %s", result.Reason)
	}
	if result.Format != "html" {
		t.Errorf("Expected Format=html, got %s", result.Format)
	}
	if result.Reason != "HTML file detected" {
		t.Errorf("Expected Reason='HTML file detected', got %s", result.Reason)
	}
}

func TestDetect_InvalidExtension(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)

	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if result.Detected {
		t.Errorf("Expected Detected=false, got true")
	}
	if result.Reason != "not a .html file" {
		t.Errorf("Expected Reason='not a .html file', got %s", result.Reason)
	}
}

func TestDetect_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	h := &Handler{}
	result, err := h.Detect(tmpDir)

	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if result.Detected {
		t.Errorf("Expected Detected=false for directory, got true")
	}
	if result.Reason != "path is a directory" {
		t.Errorf("Expected Reason='path is a directory', got %s", result.Reason)
	}
}

func TestDetect_NonExistentFile(t *testing.T) {
	h := &Handler{}
	result, err := h.Detect("/nonexistent/file.html")

	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if result.Detected {
		t.Errorf("Expected Detected=false for non-existent file, got true")
	}
	if result.Reason == "" {
		t.Errorf("Expected non-empty Reason for non-existent file")
	}
}

func TestIngest_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "genesis.html")
	testContent := []byte("<html><title>Genesis</title><body>Test content</body></html>")

	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "blobs")
	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)

	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}
	if result.ArtifactID != "genesis" {
		t.Errorf("Expected ArtifactID='genesis', got %s", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Errorf("Expected non-empty BlobSHA256")
	}
	if result.SizeBytes != int64(len(testContent)) {
		t.Errorf("Expected SizeBytes=%d, got %d", len(testContent), result.SizeBytes)
	}
	if result.Metadata["format"] != "html" {
		t.Errorf("Expected Metadata[format]='html', got %s", result.Metadata["format"])
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Errorf("Expected blob to exist at %s", blobPath)
	}
}

func TestIngest_InvalidPath(t *testing.T) {
	h := &Handler{}
	_, err := h.Ingest("/nonexistent/file.html", t.TempDir())

	if err == nil {
		t.Errorf("Expected error for non-existent file, got nil")
	}
}

func TestIngest_InvalidOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.html")
	testContent := []byte("<html></html>")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	h := &Handler{}

	// Test 1: MkdirAll failure - create a file where we need a directory
	badOutputDir := filepath.Join(tmpDir, "baddir")
	if err := os.WriteFile(badOutputDir, []byte("not a dir"), 0644); err != nil {
		t.Fatalf("Failed to create bad output: %v", err)
	}

	_, err := h.Ingest(testFile, badOutputDir)
	if err == nil {
		t.Errorf("Expected error for invalid output dir, got nil")
	}
}

func TestIngest_BlobWriteFailure(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.html")
	testContent := []byte("<html></html>")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	h := &Handler{}
	outputDir := tmpDir

	// Pre-compute the hash to know where the blob will be written
	hash := sha256.Sum256(testContent)
	hashHex := hex.EncodeToString(hash[:])
	blobDir := filepath.Join(outputDir, hashHex[:2])

	// Create the blob directory first
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		t.Fatalf("Failed to create blob dir: %v", err)
	}

	// Create a file (not writable) at the blob path
	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, []byte("existing"), 0444); err != nil {
		t.Fatalf("Failed to create read-only blob: %v", err)
	}
	// Make it truly read-only
	if err := os.Chmod(blobPath, 0000); err != nil {
		t.Fatalf("Failed to chmod blob: %v", err)
	}

	// Make the directory read-only to prevent overwrites
	if err := os.Chmod(blobDir, 0555); err != nil {
		t.Fatalf("Failed to chmod blob dir: %v", err)
	}

	_, err := h.Ingest(testFile, outputDir)

	// Clean up permissions before checking
	os.Chmod(blobDir, 0755)
	os.Chmod(blobPath, 0644)

	if err == nil {
		t.Errorf("Expected error for blob write failure, got nil")
	}
}

func TestEnumerate_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.html")
	testContent := []byte("<html></html>")

	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	h := &Handler{}
	result, err := h.Enumerate(testFile)

	if err != nil {
		t.Fatalf("Enumerate returned error: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}
	if result.Entries[0].Path != "test.html" {
		t.Errorf("Expected Path='test.html', got %s", result.Entries[0].Path)
	}
	if result.Entries[0].SizeBytes != int64(len(testContent)) {
		t.Errorf("Expected SizeBytes=%d, got %d", len(testContent), result.Entries[0].SizeBytes)
	}
	if result.Entries[0].IsDir {
		t.Errorf("Expected IsDir=false, got true")
	}
}

func TestEnumerate_InvalidPath(t *testing.T) {
	h := &Handler{}
	_, err := h.Enumerate("/nonexistent/file.html")

	if err == nil {
		t.Errorf("Expected error for non-existent file, got nil")
	}
}

func TestExtractIR_WithVersePattern1(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "genesis.html")
	testContent := `<html>
<head><title>Genesis</title></head>
<body>
<h2>Chapter 1</h2>
<p class="verse" data-verse="1"><span class="verse-text">In the beginning God created</span></p>
<p class="verse" data-verse="2"><span class="verse-text">And the earth was without form</span></p>
</body>
</html>`

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	outputDir := tmpDir
	h := &Handler{}
	result, err := h.ExtractIR(testFile, outputDir)

	if err != nil {
		t.Fatalf("ExtractIR returned error: %v", err)
	}
	if result.LossClass != "L1" {
		t.Errorf("Expected LossClass=L1, got %s", result.LossClass)
	}

	// Verify IR file was created
	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatalf("Failed to read IR file: %v", err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("Failed to parse IR JSON: %v", err)
	}

	if corpus.Title != "Genesis" {
		t.Errorf("Expected Title='Genesis', got %s", corpus.Title)
	}
	if corpus.SourceFormat != "HTML" {
		t.Errorf("Expected SourceFormat='HTML', got %s", corpus.SourceFormat)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("Expected 2 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

func TestExtractIR_WithVersePattern2(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.html")
	testContent := `<html>
<head><title>Test Book</title></head>
<body>
<span class="verse" data-verse="1">First verse text</span>
<span class="verse" data-verse="2">Second verse text</span>
</body>
</html>`

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	outputDir := tmpDir
	h := &Handler{}
	result, err := h.ExtractIR(testFile, outputDir)

	if err != nil {
		t.Fatalf("ExtractIR returned error: %v", err)
	}

	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatalf("Failed to read IR file: %v", err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("Failed to parse IR JSON: %v", err)
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("Expected 2 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

func TestExtractIR_WithVersePattern3(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.html")
	testContent := `<html>
<head><title>Test Book</title></head>
<body>
<span class="v">1</span> First verse text
<span class="v">2</span> Second verse text
</body>
</html>`

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	outputDir := tmpDir
	h := &Handler{}
	result, err := h.ExtractIR(testFile, outputDir)

	if err != nil {
		t.Fatalf("ExtractIR returned error: %v", err)
	}

	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatalf("Failed to read IR file: %v", err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("Failed to parse IR JSON: %v", err)
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("Expected 2 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

func TestExtractIR_MissingTitle(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.html")
	testContent := `<html>
<body>
<span class="verse" data-verse="1">Test verse</span>
</body>
</html>`

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	outputDir := tmpDir
	h := &Handler{}
	result, err := h.ExtractIR(testFile, outputDir)

	if err != nil {
		t.Fatalf("ExtractIR returned error: %v", err)
	}

	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatalf("Failed to read IR file: %v", err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("Failed to parse IR JSON: %v", err)
	}

	if corpus.Title != "" {
		t.Errorf("Expected empty Title, got %s", corpus.Title)
	}
}

func TestExtractIR_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.html")
	testContent := `<html>
<head><title>Empty Book</title></head>
<body>
<span class="verse" data-verse="1"></span>
<span class="verse" data-verse="2">   </span>
</body>
</html>`

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	outputDir := tmpDir
	h := &Handler{}
	result, err := h.ExtractIR(testFile, outputDir)

	if err != nil {
		t.Fatalf("ExtractIR returned error: %v", err)
	}

	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatalf("Failed to read IR file: %v", err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("Failed to parse IR JSON: %v", err)
	}

	// Empty verses should be skipped
	if len(corpus.Documents[0].ContentBlocks) != 0 {
		t.Errorf("Expected 0 content blocks for empty verses, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

func TestExtractIR_InvalidPath(t *testing.T) {
	h := &Handler{}
	_, err := h.ExtractIR("/nonexistent/file.html", t.TempDir())

	if err == nil {
		t.Errorf("Expected error for non-existent file, got nil")
	}
}

func TestExtractIR_InvalidOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.html")
	testContent := `<html><title>Test</title></html>`

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	h := &Handler{}
	// Use a path that will cause WriteFile to fail
	badOutputDir := "/dev/null/cant/write/here"
	_, err := h.ExtractIR(testFile, badOutputDir)

	if err == nil {
		t.Errorf("Expected error for invalid output dir, got nil")
	}
}

func TestEmitNative_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create original HTML content
	originalHTML := `<html>
<head><title>Genesis</title></head>
<body>
<p class="verse" data-verse="1"><span class="verse-text">In the beginning</span></p>
</body>
</html>`

	testFile := filepath.Join(tmpDir, "genesis.html")
	if err := os.WriteFile(testFile, []byte(originalHTML), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Extract to IR
	h := &Handler{}
	irResult, err := h.ExtractIR(testFile, tmpDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	// Emit back to HTML
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	emitResult, err := h.EmitNative(irResult.IRPath, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	if emitResult.Format != "HTML" {
		t.Errorf("Expected Format='HTML', got %s", emitResult.Format)
	}
	if emitResult.LossClass != "L0" {
		t.Errorf("Expected LossClass='L0' for round-trip, got %s", emitResult.LossClass)
	}

	// Verify output matches original (round-trip preservation)
	outputHTML, err := os.ReadFile(emitResult.OutputPath)
	if err != nil {
		t.Fatalf("Failed to read output HTML: %v", err)
	}

	if string(outputHTML) != originalHTML {
		t.Errorf("Round-trip HTML does not match original.\nExpected:\n%s\nGot:\n%s", originalHTML, string(outputHTML))
	}
}

func TestEmitNative_HtmlEscaping(t *testing.T) {
	tmpDir := t.TempDir()

	// Create IR with special characters
	corpus := &ir.Corpus{
		ID:           "test",
		Version:      "1.0.0",
		ModuleType:   ir.ModuleBible,
		SourceFormat: "HTML",
		SourceHash:   "test",
		LossClass:    ir.LossL1,
		Title:        "Test & <Special> \"Characters\"",
		Attributes:   make(map[string]string),
		Documents: []*ir.Document{
			{
				ID:         "test",
				Title:      "Test & <Doc>",
				Order:      1,
				Attributes: make(map[string]string),
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "Text with <tags> & \"quotes\"",
						Hash:     "test",
						Anchors: []*ir.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*ir.Span{
									{
										ID:            "s-test.1.1",
										Type:          ir.SpanVerse,
										StartAnchorID: "a-1-0",
										Ref: &ir.Ref{
											Book:    "test",
											Chapter: 1,
											Verse:   1,
											OSISID:  "test.1.1",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal IR: %v", err)
	}
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("Failed to write IR: %v", err)
	}

	h := &Handler{}
	result, err := h.EmitNative(irPath, tmpDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	html, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatalf("Failed to read output HTML: %v", err)
	}

	htmlStr := string(html)

	// Check that special characters are properly escaped
	if !contains(htmlStr, "Test &amp; &lt;Special&gt; &quot;Characters&quot;") {
		t.Errorf("Title not properly escaped in HTML")
	}
	if !contains(htmlStr, "Test &amp; &lt;Doc&gt;") {
		t.Errorf("Document title not properly escaped in HTML")
	}
	if !contains(htmlStr, "Text with &lt;tags&gt; &amp; &quot;quotes&quot;") {
		t.Errorf("Content text not properly escaped in HTML")
	}
}

func TestEmitNative_InvalidIRPath(t *testing.T) {
	h := &Handler{}
	_, err := h.EmitNative("/nonexistent/file.ir.json", t.TempDir())

	if err == nil {
		t.Errorf("Expected error for non-existent IR file, got nil")
	}
}

func TestEmitNative_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "invalid.ir.json")

	if err := os.WriteFile(irPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	h := &Handler{}
	_, err := h.EmitNative(irPath, tmpDir)

	if err == nil {
		t.Errorf("Expected error for invalid JSON, got nil")
	}
}

func TestEmitNative_InvalidOutputDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid IR
	corpus := &ir.Corpus{
		ID:           "test",
		Version:      "1.0.0",
		ModuleType:   ir.ModuleBible,
		SourceFormat: "HTML",
		SourceHash:   "test",
		LossClass:    ir.LossL1,
		Title:        "Test",
		Attributes:   make(map[string]string),
		Documents:    []*ir.Document{},
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal IR: %v", err)
	}
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("Failed to write IR: %v", err)
	}

	h := &Handler{}
	// Use a path that will cause WriteFile to fail
	badOutputDir := "/dev/null/cant/write/here"
	_, err = h.EmitNative(irPath, badOutputDir)

	if err == nil {
		t.Errorf("Expected error for invalid output dir, got nil")
	}
}

func TestEmitNative_RoundTripWriteFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create IR with _html_raw for round-trip
	corpus := &ir.Corpus{
		ID:           "test",
		Version:      "1.0.0",
		ModuleType:   ir.ModuleBible,
		SourceFormat: "HTML",
		SourceHash:   "test",
		LossClass:    ir.LossL1,
		Title:        "Test",
		Attributes:   map[string]string{"_html_raw": "<html>raw content</html>"},
		Documents:    []*ir.Document{},
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal IR: %v", err)
	}
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("Failed to write IR: %v", err)
	}

	h := &Handler{}
	// Use a path that will cause WriteFile to fail
	badOutputDir := "/dev/null/cant/write/here"
	_, err = h.EmitNative(irPath, badOutputDir)

	if err == nil {
		t.Errorf("Expected error for round-trip write failure, got nil")
	}
}

func TestParseHTMLContent_MultiplePatterns(t *testing.T) {
	// Test that parseHTMLContent handles all verse patterns
	testCases := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "Pattern1",
			content:  `<p class="verse" data-verse="1"><span class="verse-text">Text 1</span></p>`,
			expected: 1,
		},
		{
			name:     "Pattern2",
			content:  `<span class="verse" data-verse="1">Text 1</span>`,
			expected: 1,
		},
		{
			name:     "Pattern3",
			content:  `<span class="v">1</span> Text 1`,
			expected: 1,
		},
		{
			name:     "WithChapter",
			content:  `<h2>Chapter 5</h2><span class="v">1</span> Text 1`,
			expected: 1,
		},
		{
			name:     "NoVerses",
			content:  `<p>Some text without verses</p>`,
			expected: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			docs := parseHTMLContent(tc.content, "test")
			if len(docs) != 1 {
				t.Fatalf("Expected 1 document, got %d", len(docs))
			}
			if len(docs[0].ContentBlocks) != tc.expected {
				t.Errorf("Expected %d content blocks, got %d", tc.expected, len(docs[0].ContentBlocks))
			}
		})
	}
}

func TestEscapeHTML(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "plain text",
			expected: "plain text",
		},
		{
			input:    "text & more",
			expected: "text &amp; more",
		},
		{
			input:    "<tag>",
			expected: "&lt;tag&gt;",
		},
		{
			input:    "quote \"this\"",
			expected: "quote &quot;this&quot;",
		},
		{
			input:    "all & <special> \"chars\"",
			expected: "all &amp; &lt;special&gt; &quot;chars&quot;",
		},
		{
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := escapeHTML(tc.input)
			if result != tc.expected {
				t.Errorf("escapeHTML(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest.PluginID != "format.html" {
		t.Errorf("Expected PluginID='format.html', got %s", manifest.PluginID)
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("Expected Version='1.0.0', got %s", manifest.Version)
	}
	if manifest.Kind != "format" {
		t.Errorf("Expected Kind='format', got %s", manifest.Kind)
	}
	if manifest.Entrypoint != "format-html" {
		t.Errorf("Expected Entrypoint='format-html', got %s", manifest.Entrypoint)
	}
	if len(manifest.Capabilities.Inputs) != 1 || manifest.Capabilities.Inputs[0] != "file" {
		t.Errorf("Expected Inputs=['file'], got %v", manifest.Capabilities.Inputs)
	}
	if len(manifest.Capabilities.Outputs) != 1 || manifest.Capabilities.Outputs[0] != "artifact.kind:html" {
		t.Errorf("Expected Outputs=['artifact.kind:html'], got %v", manifest.Capabilities.Outputs)
	}
}

func TestRegister(t *testing.T) {
	// Test that Register doesn't panic
	// The actual registration happens in init(), but we can call it again
	Register()
}

func TestEmitNative_WithLanguage(t *testing.T) {
	tmpDir := t.TempDir()

	// Create IR with language attribute
	corpus := &ir.Corpus{
		ID:           "test",
		Version:      "1.0.0",
		ModuleType:   ir.ModuleBible,
		SourceFormat: "HTML",
		SourceHash:   "test",
		LossClass:    ir.LossL1,
		Title:        "Test",
		Language:     "es",
		Attributes:   make(map[string]string),
		Documents:    []*ir.Document{},
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal IR: %v", err)
	}
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("Failed to write IR: %v", err)
	}

	h := &Handler{}
	result, err := h.EmitNative(irPath, tmpDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	html, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatalf("Failed to read output HTML: %v", err)
	}

	if !contains(string(html), `<html lang="es">`) {
		t.Errorf("Expected HTML to have lang='es'")
	}
}

func TestEmitNative_DefaultLanguage(t *testing.T) {
	tmpDir := t.TempDir()

	// Create IR without language attribute
	corpus := &ir.Corpus{
		ID:           "test",
		Version:      "1.0.0",
		ModuleType:   ir.ModuleBible,
		SourceFormat: "HTML",
		SourceHash:   "test",
		LossClass:    ir.LossL1,
		Title:        "Test",
		Attributes:   make(map[string]string),
		Documents:    []*ir.Document{},
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal IR: %v", err)
	}
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("Failed to write IR: %v", err)
	}

	h := &Handler{}
	result, err := h.EmitNative(irPath, tmpDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	html, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatalf("Failed to read output HTML: %v", err)
	}

	if !contains(string(html), `<html lang="en">`) {
		t.Errorf("Expected HTML to have default lang='en'")
	}
}

func TestEmitNative_MultipleChapters(t *testing.T) {
	tmpDir := t.TempDir()

	// Create IR with multiple chapters
	corpus := &ir.Corpus{
		ID:           "test",
		Version:      "1.0.0",
		ModuleType:   ir.ModuleBible,
		SourceFormat: "HTML",
		SourceHash:   "test",
		LossClass:    ir.LossL1,
		Title:        "Test",
		Attributes:   make(map[string]string),
		Documents: []*ir.Document{
			{
				ID:         "test",
				Title:      "Test Book",
				Order:      1,
				Attributes: make(map[string]string),
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "Chapter 1 Verse 1",
						Hash:     "test1",
						Anchors: []*ir.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*ir.Span{
									{
										ID:            "s-test.1.1",
										Type:          ir.SpanVerse,
										StartAnchorID: "a-1-0",
										Ref: &ir.Ref{
											Book:    "test",
											Chapter: 1,
											Verse:   1,
											OSISID:  "test.1.1",
										},
									},
								},
							},
						},
					},
					{
						ID:       "cb-2",
						Sequence: 2,
						Text:     "Chapter 2 Verse 1",
						Hash:     "test2",
						Anchors: []*ir.Anchor{
							{
								ID:       "a-2-0",
								Position: 0,
								Spans: []*ir.Span{
									{
										ID:            "s-test.2.1",
										Type:          ir.SpanVerse,
										StartAnchorID: "a-2-0",
										Ref: &ir.Ref{
											Book:    "test",
											Chapter: 2,
											Verse:   1,
											OSISID:  "test.2.1",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal IR: %v", err)
	}
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("Failed to write IR: %v", err)
	}

	h := &Handler{}
	result, err := h.EmitNative(irPath, tmpDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	html, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatalf("Failed to read output HTML: %v", err)
	}

	htmlStr := string(html)

	// Verify both chapters are present
	if !contains(htmlStr, "Chapter 1") {
		t.Errorf("Expected HTML to contain 'Chapter 1'")
	}
	if !contains(htmlStr, "Chapter 2") {
		t.Errorf("Expected HTML to contain 'Chapter 2'")
	}
	if !contains(htmlStr, "</section>") {
		t.Errorf("Expected HTML to contain closing section tags")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
