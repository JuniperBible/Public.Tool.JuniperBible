package tischendorf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.tischendorf" {
		t.Errorf("Expected PluginID 'format.tischendorf', got %s", m.PluginID)
	}
	if m.Kind != "format" {
		t.Errorf("Expected Kind 'format', got %s", m.Kind)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", m.Version)
	}
}

func TestRegister(t *testing.T) {
	// Register should not panic
	Register()
}

func TestDetect_TischendorfFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	// Create a file with Greek text, apparatus markers, and verse references
	content := "Matt 1:1 Βίβλος [γενέσεως] Ἰησοῦ Χριστοῦ"
	file := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(file)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected file to be detected, reason: %s", result.Reason)
	}
	if result.Format != "tischendorf" {
		t.Errorf("Expected format 'tischendorf', got %s", result.Format)
	}
}

func TestDetect_NonTischendorfFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	file := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(file, []byte("plain text"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(file)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-Tischendorf file to not be detected")
	}
}

func TestDetect_NonExistentFile(t *testing.T) {
	h := &Handler{}

	result, err := h.Detect("/nonexistent/file.txt")
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected non-existent file to not be detected")
	}
	if !strings.Contains(result.Reason, "cannot read") {
		t.Errorf("Expected reason to mention read error, got: %s", result.Reason)
	}
}

func TestIngest_TischendorfFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	file := filepath.Join(tmpDir, "test.txt")
	content := []byte("Matt 1:1 Βίβλος [γενέσεως] Ἰησοῦ Χριστοῦ")
	if err := os.WriteFile(file, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	result, err := h.Ingest(file, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.ArtifactID != "test" {
		t.Errorf("Expected artifact ID 'test', got %s", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected blob hash to be set")
	}
	if result.Metadata["format"] != "tischendorf" {
		t.Errorf("Expected format 'tischendorf', got %s", result.Metadata["format"])
	}
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

func TestIngest_NonExistentFile(t *testing.T) {
	h := &Handler{}
	outputDir := t.TempDir()

	_, err := h.Ingest("/nonexistent/file.txt", outputDir)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected 'failed to read file' error, got: %v", err)
	}
}

func TestEnumerate_TischendorfFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	file := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	if err := os.WriteFile(file, content, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Enumerate(file)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.txt" {
		t.Errorf("Expected path 'test.txt', got %s", entry.Path)
	}
	if entry.IsDir {
		t.Error("Expected entry to not be a directory")
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
}

func TestEnumerate_NonExistentFile(t *testing.T) {
	h := &Handler{}

	_, err := h.Enumerate("/nonexistent/file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to stat") {
		t.Errorf("Expected 'failed to stat' error, got: %v", err)
	}
}

func TestExtractIR_TischendorfFile(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	file := filepath.Join(tmpDir, "test.txt")
	content := []byte("Matt 1:1 Βίβλος [γενέσεως] Ἰησοῦ Χριστοῦ")
	if err := os.WriteFile(file, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := h.ExtractIR(file, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.LossClass != "L2" {
		t.Errorf("Expected loss class 'L2', got %s", result.LossClass)
	}
	if result.IRPath == "" {
		t.Error("Expected IR path to be set")
	}

	// Verify IR file was written
	if _, err := os.Stat(result.IRPath); os.IsNotExist(err) {
		t.Error("Expected IR file to exist")
	}
}

func TestExtractIR_NonExistentFile(t *testing.T) {
	h := &Handler{}
	outputDir := t.TempDir()

	_, err := h.ExtractIR("/nonexistent/file.txt", outputDir)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected 'failed to read file' error, got: %v", err)
	}
}

func TestEmitNative_FromIR(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	// Create a simple IR file
	irContent := `{
		"id": "test",
		"version": "1.0",
		"title": "Test",
		"documents": []
	}`
	irFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(irFile, []byte(irContent), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := h.EmitNative(irFile, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Format != "tischendorf" {
		t.Errorf("Expected format 'tischendorf', got %s", result.Format)
	}
	if result.LossClass != "L2" {
		t.Errorf("Expected loss class 'L2', got %s", result.LossClass)
	}
	if result.OutputPath == "" {
		t.Error("Expected output path to be set")
	}

	// Verify output file was written
	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Error("Expected output file to exist")
	}
}

func TestEmitNative_NonExistentIRFile(t *testing.T) {
	h := &Handler{}
	outputDir := t.TempDir()

	_, err := h.EmitNative("/nonexistent/ir.json", outputDir)
	if err == nil {
		t.Error("Expected error for non-existent IR file")
	}
	if !strings.Contains(err.Error(), "failed to read IR file") {
		t.Errorf("Expected 'failed to read IR file' error, got: %v", err)
	}
}

func TestEmitNative_InvalidJSON(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()

	irFile := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(irFile, []byte("not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := h.EmitNative(irFile, outputDir)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to unmarshal IR") {
		t.Errorf("Expected 'failed to unmarshal IR' error, got: %v", err)
	}
}

func TestEmitNative_NonWritableOutput(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()

	irFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(irFile, []byte(`{"id": "test"}`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := h.EmitNative(irFile, "/nonexistent/path/output")
	if err == nil {
		t.Error("Expected error for non-writable output")
	}
	if !strings.Contains(err.Error(), "failed to write output") {
		t.Errorf("Expected 'failed to write output' error, got: %v", err)
	}
}

func TestIngest_NonWritableOutputDir(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()

	file := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := h.Ingest(file, "/nonexistent/path/output")
	if err == nil {
		t.Error("Expected error for non-writable output dir")
	}
	if !strings.Contains(err.Error(), "failed to create blob dir") {
		t.Errorf("Expected 'failed to create blob dir' error, got: %v", err)
	}
}

func TestExtractIR_NonWritableOutputDir(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()

	file := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(file, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := h.ExtractIR(file, "/nonexistent/path/output")
	if err == nil {
		t.Error("Expected error for non-writable output dir")
	}
	if !strings.Contains(err.Error(), "failed to write IR file") {
		t.Errorf("Expected 'failed to write IR file' error, got: %v", err)
	}
}

func TestIsBookHeader(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"Matthew Chapter 1", true},
		{"Mark 1:1", true},
		{"Luke Introduction", true},
		{"John", true},
		{"Acts of the Apostles", true},
		{"Romans", true},
		{"1 Corinthians", true},
		{"2 Corinthians", true},
		{"Galatians", true},
		{"Ephesians", true},
		{"Philippians", true},
		{"Colossians", true},
		{"1 Thessalonians", true},
		{"2 Thessalonians", true},
		{"1 Timothy", true},
		{"2 Timothy", true},
		{"Titus", true},
		{"Philemon", true},
		{"Hebrews", true},
		{"James", true},
		{"1 Peter", true},
		{"2 Peter", true},
		{"1 John", true},
		{"2 John", true},
		{"3 John", true},
		{"Jude", true},
		{"Revelation", true},
		{"Some random text", false},
		{"1:1 verse text", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := isBookHeader(tt.line)
			if result != tt.expected {
				t.Errorf("isBookHeader(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

func TestExtractBookName(t *testing.T) {
	tests := []struct {
		line     string
		expected string
	}{
		{"Matthew Chapter 1", "Matthew"},
		{"John", "John"},
		{"1 Corinthians", "1"},
		{"", "Unknown"},
		{"   ", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := extractBookName(tt.line)
			if result != tt.expected {
				t.Errorf("extractBookName(%q) = %q, want %q", tt.line, result, tt.expected)
			}
		})
	}
}

func TestExtractReference(t *testing.T) {
	tests := []struct {
		line     string
		expected string
	}{
		{"1:1 In the beginning", "1:1"},
		{"Some text 3:16 more text", "3:16"},
		{"10:25 verse text", "10:25"},
		{"no reference here", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := extractReference(tt.line)
			if result != tt.expected {
				t.Errorf("extractReference(%q) = %q, want %q", tt.line, result, tt.expected)
			}
		})
	}
}

func TestExtractText(t *testing.T) {
	tests := []struct {
		line     string
		expected string
	}{
		{"1:1 Βίβλος [γενέσεως] Ἰησοῦ", "Βίβλος  Ἰησοῦ"},
		{"3:16 For God so [loved] the world", "For God so  the world"},
		{"plain text without markers", "plain text without markers"},
		{"1:1 [all apparatus]", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := extractText(tt.line)
			if result != tt.expected {
				t.Errorf("extractText(%q) = %q, want %q", tt.line, result, tt.expected)
			}
		})
	}
}

func TestParseRefString(t *testing.T) {
	tests := []struct {
		refStr      string
		wantChapter int
		wantVerse   int
		wantEmpty   bool
	}{
		{"1:1", 1, 1, false},
		{"3:16", 3, 16, false},
		{"10:25", 10, 25, false},
		{"invalid", 0, 0, true},
		{"", 0, 0, true},
		{"1", 0, 0, true},
		{"1:2:3", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.refStr, func(t *testing.T) {
			result := parseRefString(tt.refStr)
			if tt.wantEmpty {
				if len(result) != 0 {
					t.Errorf("parseRefString(%q) = %v, want empty map", tt.refStr, result)
				}
			} else {
				if result["chapter"] != tt.wantChapter {
					t.Errorf("parseRefString(%q) chapter = %v, want %d", tt.refStr, result["chapter"], tt.wantChapter)
				}
				if result["verse"] != tt.wantVerse {
					t.Errorf("parseRefString(%q) verse = %v, want %d", tt.refStr, result["verse"], tt.wantVerse)
				}
			}
		})
	}
}

func TestParseTischendorfToIR_MultipleBooks(t *testing.T) {
	content := `Matthew Chapter 1
1:1 Βίβλος γενέσεως Ἰησοῦ Χριστοῦ
1:2 Ἀβραὰμ ἐγέννησεν τὸν Ἰσαάκ

Mark Chapter 1
1:1 Ἀρχὴ τοῦ εὐαγγελίου
1:2 Καθὼς γέγραπται

Luke Chapter 1
1:1 Ἐπειδήπερ πολλοὶ ἐπεχείρησαν
`

	corpus := parseTischendorfToIR([]byte(content))

	// Check corpus fields
	if corpus["id"] != "tischendorf-nt" {
		t.Errorf("corpus id = %v, want tischendorf-nt", corpus["id"])
	}
	if corpus["language"] != "grc" {
		t.Errorf("corpus language = %v, want grc", corpus["language"])
	}

	docs, ok := corpus["documents"].([]map[string]interface{})
	if !ok {
		t.Fatal("documents should be []map[string]interface{}")
	}

	if len(docs) != 3 {
		t.Fatalf("Expected 3 documents, got %d", len(docs))
	}

	// Check first document (Matthew)
	if docs[0]["id"] != "Matthew" {
		t.Errorf("doc[0] id = %v, want Matthew", docs[0]["id"])
	}
	blocks0, ok := docs[0]["content_blocks"].([]map[string]interface{})
	if !ok {
		t.Fatal("content_blocks should be []map[string]interface{}")
	}
	if len(blocks0) != 2 {
		t.Errorf("Matthew should have 2 blocks, got %d", len(blocks0))
	}

	// Check second document (Mark)
	if docs[1]["id"] != "Mark" {
		t.Errorf("doc[1] id = %v, want Mark", docs[1]["id"])
	}

	// Check third document (Luke)
	if docs[2]["id"] != "Luke" {
		t.Errorf("doc[2] id = %v, want Luke", docs[2]["id"])
	}
}

func TestParseTischendorfToIR_EmptyInput(t *testing.T) {
	corpus := parseTischendorfToIR([]byte(""))

	docs, ok := corpus["documents"].([]map[string]interface{})
	if ok && len(docs) != 0 {
		t.Errorf("Expected 0 documents for empty input, got %d", len(docs))
	}
}

func TestParseTischendorfToIR_NoBookHeader(t *testing.T) {
	// Content without a book header - verses should be ignored
	content := `1:1 Βίβλος γενέσεως
1:2 Ἀβραὰμ ἐγέννησεν
`
	corpus := parseTischendorfToIR([]byte(content))

	docs := corpus["documents"]
	if docs != nil {
		docsSlice, ok := docs.([]map[string]interface{})
		if ok && len(docsSlice) > 0 {
			t.Error("Expected no documents without book header")
		}
	}
}

func TestEmitTischendorfFromIR_FullCorpus(t *testing.T) {
	corpus := map[string]interface{}{
		"id":       "test-corpus",
		"title":    "Test Greek NT",
		"version":  "1.0",
		"language": "grc",
		"documents": []interface{}{
			map[string]interface{}{
				"id":    "Matt",
				"title": "Matthew",
				"content_blocks": []interface{}{
					map[string]interface{}{
						"id":   "block1",
						"text": "Βίβλος γενέσεως",
						"attributes": map[string]interface{}{
							"verse_ref": "1:1",
						},
					},
					map[string]interface{}{
						"id":   "block2",
						"text": "Ἀβραὰμ ἐγέννησεν",
						"attributes": map[string]interface{}{
							"verse_ref": "1:2",
						},
					},
				},
			},
			map[string]interface{}{
				"id":    "Mark",
				"title": "Mark",
				"content_blocks": []interface{}{
					map[string]interface{}{
						"id":   "block1",
						"text": "Ἀρχὴ τοῦ εὐαγγελίου",
						"attributes": map[string]interface{}{
							"verse_ref": "1:1",
						},
					},
				},
			},
		},
	}

	output := emitTischendorfFromIR(corpus)

	// Check header
	if !strings.Contains(output, "# Test Greek NT") {
		t.Error("Expected title in output")
	}
	if !strings.Contains(output, "# Version: 1.0") {
		t.Error("Expected version in output")
	}
	if !strings.Contains(output, "# Language: grc") {
		t.Error("Expected language in output")
	}

	// Check document headers
	if !strings.Contains(output, "## Matthew") {
		t.Error("Expected Matthew header")
	}
	if !strings.Contains(output, "## Mark") {
		t.Error("Expected Mark header")
	}

	// Check verse content
	if !strings.Contains(output, "1:1 Βίβλος γενέσεως") {
		t.Error("Expected Matt 1:1 content")
	}
	if !strings.Contains(output, "1:2 Ἀβραὰμ ἐγέννησεν") {
		t.Error("Expected Matt 1:2 content")
	}
	if !strings.Contains(output, "1:1 Ἀρχὴ τοῦ εὐαγγελίου") {
		t.Error("Expected Mark 1:1 content")
	}
}

func TestEmitTischendorfFromIR_NoVerseRef(t *testing.T) {
	corpus := map[string]interface{}{
		"title": "Test",
		"documents": []interface{}{
			map[string]interface{}{
				"title": "Book",
				"content_blocks": []interface{}{
					map[string]interface{}{
						"text": "Plain text without ref",
					},
				},
			},
		},
	}

	output := emitTischendorfFromIR(corpus)

	if !strings.Contains(output, "Plain text without ref") {
		t.Error("Expected text without reference")
	}
}

func TestEmitTischendorfFromIR_EmptyCorpus(t *testing.T) {
	corpus := map[string]interface{}{}

	output := emitTischendorfFromIR(corpus)

	// Should return empty or minimal output
	if len(output) > 10 {
		t.Errorf("Expected minimal output for empty corpus, got: %s", output)
	}
}

func TestDetect_StandaloneVerseRef(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	// File with standalone verse reference format (no book prefix)
	content := "1:1 Βίβλος [γενέσεως] Ἰησοῦ"
	file := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := h.Detect(file)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected file with standalone verse ref to be detected, reason: %s", result.Reason)
	}
}

func TestExtractIR_WithBookHeaders(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	content := `Matthew
1:1 Βίβλος γενέσεως Ἰησοῦ Χριστοῦ

Mark
1:1 Ἀρχὴ τοῦ εὐαγγελίου
`
	file := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := h.ExtractIR(file, outputDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.LossReport == nil {
		t.Error("Expected loss report to be set")
	}
	if result.LossReport.SourceFormat != "tischendorf" {
		t.Errorf("LossReport source format = %s, want tischendorf", result.LossReport.SourceFormat)
	}

	// Read and verify IR content
	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(irData), "Matthew") {
		t.Error("Expected IR to contain Matthew document")
	}
	if !strings.Contains(string(irData), "Mark") {
		t.Error("Expected IR to contain Mark document")
	}
}
