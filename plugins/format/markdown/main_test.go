//go:build !sdk

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// createTestMarkdown creates a minimal Markdown Bible file for testing.
func createTestMarkdown(t *testing.T, path string) {
	t.Helper()

	content := `---
title: "Test Bible"
language: en
book: Gen
---

# Genesis

## Chapter 1

**1** In the beginning God created the heavens and the earth.
**2** And the earth was without form and void.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test markdown: %v", err)
	}
}

// TestMarkdownDetect tests the detect command.
func TestMarkdownDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markdown-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mdPath := filepath.Join(tmpDir, "test.md")
	createTestMarkdown(t, mdPath)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": mdPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] != true {
		t.Error("expected detected to be true")
	}
	if result["format"] != "Markdown" {
		t.Errorf("expected format Markdown, got %v", result["format"])
	}
}

// TestMarkdownDetectNonMarkdown tests detect command on non-Markdown file.
func TestMarkdownDetectNonMarkdown(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markdown-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	txtPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtPath, []byte("Hello world"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": txtPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] == true {
		t.Error("expected detected to be false for non-Markdown file")
	}
}

// TestMarkdownDetectNoFrontmatter tests detect command on markdown without frontmatter.
func TestMarkdownDetectNoFrontmatter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markdown-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mdPath := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdPath, []byte("# Just a heading\nSome text."), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": mdPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] == true {
		t.Error("expected detected to be false for markdown without frontmatter")
	}
}

// TestMarkdownExtractIR tests the extract-ir command.
func TestMarkdownExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markdown-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mdPath := filepath.Join(tmpDir, "test.md")
	createTestMarkdown(t, mdPath)

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       mdPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["loss_class"] != "L1" {
		t.Errorf("expected loss_class L1, got %v", result["loss_class"])
	}

	irPath, ok := result["ir_path"].(string)
	if !ok {
		t.Fatal("ir_path is not a string")
	}

	irData, err := os.ReadFile(irPath)
	if err != nil {
		t.Fatalf("failed to read IR file: %v", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	if corpus.Title != "Test Bible" {
		t.Errorf("expected title Test Bible, got %s", corpus.Title)
	}
	if corpus.Language != "en" {
		t.Errorf("expected language en, got %s", corpus.Language)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("expected 2 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

// TestMarkdownEmitNative tests the emit-native command.
func TestMarkdownEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markdown-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := ipc.Corpus{
		ID:         "test",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Test Bible",
		Language:   "en",
		Documents: []*ipc.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "In the beginning.",
						Anchors: []*ipc.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*ipc.Span{
									{
										ID:            "s-Gen.1.1",
										Type:          "VERSE",
										StartAnchorID: "a-1-0",
										Ref: &ipc.Ref{
											Book:    "Gen",
											Chapter: 1,
											Verse:   1,
											OSISID:  "Gen.1.1",
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

	irData, err := json.MarshalIndent(&corpus, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal IR: %v", err)
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["format"] != "Markdown" {
		t.Errorf("expected format Markdown, got %v", result["format"])
	}

	mdPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	// Verify the output file
	mdData, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	content := string(mdData)
	if !strings.Contains(content, "title: \"Test Bible\"") {
		t.Error("expected title in frontmatter")
	}
	if !strings.Contains(content, "**1** In the beginning.") {
		t.Error("expected verse content")
	}
}

// TestMarkdownRoundTrip tests L0 lossless round-trip via raw storage.
func TestMarkdownRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markdown-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mdPath := filepath.Join(tmpDir, "original.md")
	createTestMarkdown(t, mdPath)

	originalData, _ := os.ReadFile(mdPath)

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)

	// Extract IR
	extractReq := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       mdPath,
			"output_dir": irDir,
		},
	}

	extractResp := executePlugin(t, &extractReq)
	if extractResp.Status != "ok" {
		t.Fatalf("extract-ir failed: %s", extractResp.Error)
	}

	extractResult := extractResp.Result.(map[string]interface{})
	irPath := extractResult["ir_path"].(string)

	// Emit native
	emitReq := ipc.Request{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irPath,
			"output_dir": outDir,
		},
	}

	emitResp := executePlugin(t, &emitReq)
	if emitResp.Status != "ok" {
		t.Fatalf("emit-native failed: %s", emitResp.Error)
	}

	emitResult := emitResp.Result.(map[string]interface{})
	outputPath := emitResult["output_path"].(string)

	// Verify L0 lossless round-trip
	outputData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if string(originalData) != string(outputData) {
		t.Errorf("round-trip mismatch:\noriginal: %q\noutput: %q", string(originalData), string(outputData))
	}

	// Verify loss class is L0 (lossless via raw storage)
	if emitResult["loss_class"] != "L0" {
		t.Errorf("expected loss_class L0 for round-trip, got %v", emitResult["loss_class"])
	}
}

// TestMarkdownIngest tests the ingest command.
func TestMarkdownIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markdown-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mdPath := filepath.Join(tmpDir, "test.md")
	createTestMarkdown(t, mdPath)

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       mdPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	blobHash, ok := result["blob_sha256"].(string)
	if !ok {
		t.Fatal("blob_sha256 is not a string")
	}

	blobPath := filepath.Join(outputDir, blobHash[:2], blobHash)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("blob file was not created")
	}
}

// executePlugin runs the plugin with a request and returns the response.
func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()

	pluginPath := "./format-markdown"
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
		if err := buildCmd.Run(); err != nil {
			t.Fatalf("failed to build plugin: %v", err)
		}
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	cmd := exec.Command(pluginPath)
	cmd.Stdin = bytes.NewReader(reqData)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stdout.Len() > 0 {
			var resp ipc.Response
			if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
				return &resp
			}
		}
		t.Fatalf("plugin execution failed: %v\nstderr: %s", err, stderr.String())
	}

	var resp ipc.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\noutput: %s", err, stdout.String())
	}

	return &resp
}

// TestParseMarkdownContent tests the parseMarkdownContent function directly.
func TestParseMarkdownContent(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		bookID         string
		wantDocCount   int
		wantBlockCount int
	}{
		{
			name: "simple verses",
			body: `**1** In the beginning.
**2** And the earth was void.`,
			bookID:         "GEN",
			wantDocCount:   1,
			wantBlockCount: 2,
		},
		{
			name: "with chapter heading",
			body: `## Chapter 1
**1** In the beginning.
## Chapter 2
**1** Thus the heavens.`,
			bookID:         "GEN",
			wantDocCount:   1,
			wantBlockCount: 2,
		},
		{
			name:           "empty content",
			body:           "",
			bookID:         "GEN",
			wantDocCount:   1,
			wantBlockCount: 0,
		},
		{
			name:           "no verses",
			body:           "This is plain text without verse markers.",
			bookID:         "GEN",
			wantDocCount:   1,
			wantBlockCount: 0,
		},
		{
			name:           "empty book ID",
			body:           `**1** Some text.`,
			bookID:         "",
			wantDocCount:   1,
			wantBlockCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			docs := parseMarkdownContent(tt.body, tt.bookID)
			if len(docs) != tt.wantDocCount {
				t.Errorf("expected %d documents, got %d", tt.wantDocCount, len(docs))
			}
			if len(docs) > 0 && len(docs[0].ContentBlocks) != tt.wantBlockCount {
				t.Errorf("expected %d content blocks, got %d", tt.wantBlockCount, len(docs[0].ContentBlocks))
			}
		})
	}
}

// TestMarkdownChapterParsing tests chapter number parsing.
func TestMarkdownChapterParsing(t *testing.T) {
	body := `## Chapter 5
**3** Blessed are the poor.
**4** Blessed are they that mourn.
## Chapter 6
**9** Our Father.`

	docs := parseMarkdownContent(body, "MAT")
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}

	if len(docs[0].ContentBlocks) < 3 {
		t.Fatalf("expected at least 3 content blocks, got %d", len(docs[0].ContentBlocks))
	}

	// Check that chapter 5 was captured
	foundChapter5 := false
	foundChapter6 := false
	for _, cb := range docs[0].ContentBlocks {
		if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 {
			ref := cb.Anchors[0].Spans[0].Ref
			if ref != nil {
				if ref.Chapter == 5 {
					foundChapter5 = true
				}
				if ref.Chapter == 6 {
					foundChapter6 = true
				}
			}
		}
	}

	if !foundChapter5 {
		t.Error("expected to find verses from chapter 5")
	}
	if !foundChapter6 {
		t.Error("expected to find verses from chapter 6")
	}
}

// TestMarkdownDetectRequiresFrontmatter tests that detection requires YAML frontmatter.
func TestMarkdownDetectRequiresFrontmatter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markdown-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name: "with frontmatter and verses",
			content: `---
book_id: GEN
---
**1** In the beginning.
**2** And the earth was void.`,
			want: true,
		},
		{
			name: "with frontmatter and chapters",
			content: `---
book_id: GEN
---
## Chapter 1
**1** In the beginning.`,
			want: true,
		},
		{
			name: "without frontmatter",
			content: `**1** In the beginning.
**2** And the earth was void.`,
			want: false,
		},
		{
			name:    "plain text no frontmatter",
			content: "This is just plain text.",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, "test.md")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}

			req := ipc.Request{
				Command: "detect",
				Args:    map[string]interface{}{"path": path},
			}

			resp := executePlugin(t, &req)
			if resp.Status != "ok" {
				t.Fatalf("expected status ok, got %s", resp.Status)
			}

			result := resp.Result.(map[string]interface{})
			detected := result["detected"] == true
			if detected != tt.want {
				t.Errorf("expected detected=%v, got %v", tt.want, detected)
			}
		})
	}
}

// TestMarkdownEmitNativeFormatting tests that emit-native produces proper markdown formatting.
func TestMarkdownEmitNativeFormatting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "markdown-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := ipc.Corpus{
		ID:         "GEN",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Genesis",
		Documents: []*ipc.Document{
			{
				ID:    "GEN",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "In the beginning God created.",
						Anchors: []*ipc.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*ipc.Span{
									{
										Type: "VERSE",
										Ref: &ipc.Ref{
											Book:    "GEN",
											Chapter: 1,
											Verse:   1,
										},
									},
								},
							},
						},
					},
					{
						ID:       "cb-2",
						Sequence: 2,
						Text:     "And the earth was void.",
						Anchors: []*ipc.Anchor{
							{
								ID:       "a-2-0",
								Position: 0,
								Spans: []*ipc.Span{
									{
										Type: "VERSE",
										Ref: &ipc.Ref{
											Book:    "GEN",
											Chapter: 1,
											Verse:   2,
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

	irData, err := json.MarshalIndent(&corpus, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal IR: %v", err)
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result := resp.Result.(map[string]interface{})
	mdPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	mdData, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("failed to read markdown file: %v", err)
	}

	mdStr := string(mdData)
	if !strings.Contains(mdStr, "**1**") {
		t.Error("output should contain verse number in bold **1**")
	}
	if !strings.Contains(mdStr, "In the beginning God created.") {
		t.Error("output should contain verse text")
	}
	if !strings.Contains(mdStr, "## Chapter 1") {
		t.Error("output should contain chapter heading")
	}
}
