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

// createTestHTML creates a minimal HTML Bible file for testing.
func createTestHTML(t *testing.T, path string) {
	t.Helper()

	content := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Test Bible</title>
</head>
<body>
<h1>Test Bible</h1>
<article id="Gen">
<h2>Genesis</h2>
<h3>Chapter 1</h3>
<p class="verse" data-verse="1"><span class="verse-num">1</span><span class="verse-text">In the beginning God created.</span></p>
<p class="verse" data-verse="2"><span class="verse-num">2</span><span class="verse-text">And the earth was void.</span></p>
</article>
</body>
</html>
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test HTML: %v", err)
	}
}

// TestHTMLDetect tests the detect command.
func TestHTMLDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "html-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	htmlPath := filepath.Join(tmpDir, "test.html")
	createTestHTML(t, htmlPath)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": htmlPath},
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
	if result["format"] != "HTML" {
		t.Errorf("expected format HTML, got %v", result["format"])
	}
}

// TestHTMLDetectNonHTML tests detect command on non-HTML file.
func TestHTMLDetectNonHTML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "html-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	txtPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtPath, []byte("Hello world"), 0600); err != nil {
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
		t.Error("expected detected to be false for non-HTML file")
	}
}

// TestHTMLDetectNoVerses tests detect command on HTML without verse markers.
func TestHTMLDetectNoVerses(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "html-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	htmlPath := filepath.Join(tmpDir, "test.html")
	if err := os.WriteFile(htmlPath, []byte("<html><body><p>Just a paragraph.</p></body></html>"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": htmlPath},
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
		t.Error("expected detected to be false for HTML without verse markers")
	}
}

// TestHTMLExtractIR tests the extract-ir command.
func TestHTMLExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "html-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	htmlPath := filepath.Join(tmpDir, "test.html")
	createTestHTML(t, htmlPath)

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       htmlPath,
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
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("expected 2 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

// TestHTMLEmitNative tests the emit-native command.
func TestHTMLEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "html-test-*")
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
	if err := os.WriteFile(irPath, irData, 0600); err != nil {
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

	if result["format"] != "HTML" {
		t.Errorf("expected format HTML, got %v", result["format"])
	}

	htmlPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	// Verify the output file
	htmlData, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	content := string(htmlData)
	if !strings.Contains(content, "<title>Test Bible</title>") {
		t.Error("expected title in HTML")
	}
	if !strings.Contains(content, "In the beginning.") {
		t.Error("expected verse content")
	}
}

// TestHTMLRoundTrip tests L0 lossless round-trip via raw storage.
func TestHTMLRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "html-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	htmlPath := filepath.Join(tmpDir, "original.html")
	createTestHTML(t, htmlPath)

	originalData, _ := os.ReadFile(htmlPath)

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)

	// Extract IR
	extractReq := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       htmlPath,
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

// TestHTMLIngest tests the ingest command.
func TestHTMLIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "html-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	htmlPath := filepath.Join(tmpDir, "test.html")
	createTestHTML(t, htmlPath)

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       htmlPath,
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

// TestHTMLEmitParallel tests the emit-parallel command.
func TestHTMLEmitParallel(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "html-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create parallel corpus
	parallel := ipc.ParallelCorpus{
		ID:      "kjv-niv",
		Version: "1.0.0",
		Corpora: []*ipc.CorpusRef{
			{ID: "kjv", Language: "en", Title: "King James Version"},
			{ID: "niv", Language: "en", Title: "New International Version"},
		},
		DefaultAlignment: "verse",
		Alignments: []*ipc.Alignment{
			{
				ID:    "verse",
				Level: "verse",
				Units: []*ipc.AlignedUnit{
					{
						ID:    "unit-1",
						Ref:   &ipc.Ref{Book: "Gen", Chapter: 1, Verse: 1, OSISID: "Gen.1.1"},
						Level: "verse",
						Texts: map[string]string{
							"kjv": "In the beginning God created the heaven and the earth.",
							"niv": "In the beginning God created the heavens and the earth.",
						},
					},
					{
						ID:    "unit-2",
						Ref:   &ipc.Ref{Book: "Gen", Chapter: 1, Verse: 2, OSISID: "Gen.1.2"},
						Level: "verse",
						Texts: map[string]string{
							"kjv": "And the earth was without form, and void.",
							"niv": "Now the earth was formless and empty.",
						},
					},
				},
			},
		},
	}

	parallelData, err := json.MarshalIndent(&parallel, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal parallel corpus: %v", err)
	}

	parallelPath := filepath.Join(tmpDir, "parallel.json")
	if err := os.WriteFile(parallelPath, parallelData, 0600); err != nil {
		t.Fatalf("failed to write parallel corpus: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "emit-parallel",
		Args: map[string]interface{}{
			"ir_path":    parallelPath,
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

	if result["format"] != "HTML-Parallel" {
		t.Errorf("expected format HTML-Parallel, got %v", result["format"])
	}

	outputPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	// Verify the output file
	htmlData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	content := string(htmlData)
	if !strings.Contains(content, "<table class=\"parallel-table\">") {
		t.Error("expected parallel table in HTML")
	}
	if !strings.Contains(content, "King James Version") {
		t.Error("expected KJV title in HTML")
	}
	if !strings.Contains(content, "New International Version") {
		t.Error("expected NIV title in HTML")
	}
	if !strings.Contains(content, "Gen.1.1") {
		t.Error("expected verse reference in HTML")
	}
	if !strings.Contains(content, "In the beginning God created the heaven") {
		t.Error("expected KJV verse text in HTML")
	}
}

// TestHTMLEmitInterlinear tests the emit-interlinear command.
func TestHTMLEmitInterlinear(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "html-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create interlinear lines
	lines := []ipc.InterlinearLine{
		{
			Ref: &ipc.Ref{Book: "Gen", Chapter: 1, Verse: 1, OSISID: "Gen.1.1"},
			Layers: map[string]*ipc.InterlinearLayer{
				"hebrew": {
					CorpusID: "osmhb",
					Label:    "Hebrew",
					Tokens:   []string{"בְּרֵאשִׁית", "בָּרָא", "אֱלֹהִים"},
				},
				"english": {
					CorpusID: "kjv",
					Label:    "English",
					Tokens:   []string{"In-beginning", "created", "God"},
				},
			},
		},
		{
			Ref: &ipc.Ref{Book: "Gen", Chapter: 1, Verse: 2, OSISID: "Gen.1.2"},
			Layers: map[string]*ipc.InterlinearLayer{
				"hebrew": {
					CorpusID: "osmhb",
					Label:    "Hebrew",
					Tokens:   []string{"וְהָאָרֶץ", "הָיְתָה", "תֹהוּ"},
				},
				"english": {
					CorpusID: "kjv",
					Label:    "English",
					Tokens:   []string{"And-the-earth", "was", "without-form"},
				},
			},
		},
	}

	linesData, err := json.MarshalIndent(&lines, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal interlinear lines: %v", err)
	}

	linesPath := filepath.Join(tmpDir, "interlinear.json")
	if err := os.WriteFile(linesPath, linesData, 0600); err != nil {
		t.Fatalf("failed to write interlinear lines: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "emit-interlinear",
		Args: map[string]interface{}{
			"ir_path":    linesPath,
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

	if result["format"] != "HTML-Interlinear" {
		t.Errorf("expected format HTML-Interlinear, got %v", result["format"])
	}

	outputPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	// Verify the output file
	htmlData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	content := string(htmlData)
	if !strings.Contains(content, "<div class=\"interlinear-line\">") {
		t.Error("expected interlinear line div in HTML")
	}
	if !strings.Contains(content, "Gen.1.1") {
		t.Error("expected verse reference in HTML")
	}
	if !strings.Contains(content, "בְּרֵאשִׁית") {
		t.Error("expected Hebrew token in HTML")
	}
	if !strings.Contains(content, "In-beginning") {
		t.Error("expected English gloss in HTML")
	}
	if !strings.Contains(content, "<div class=\"tokens\">") {
		t.Error("expected tokens container in HTML")
	}
}

// executePlugin runs the plugin with a request and returns the response.
func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()

	pluginPath := "./format-html"
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

// TestParseHTMLContent tests the parseHTMLContent function directly.
func TestParseHTMLContent(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		artifactID     string
		wantDocCount   int
		wantBlockCount int
	}{
		{
			name: "pattern 1 - verse class with data-verse",
			content: `<p class="verse" data-verse="1"><span class="verse-text">In the beginning.</span></p>
<p class="verse" data-verse="2"><span class="verse-text">And the earth was void.</span></p>`,
			artifactID:     "GEN",
			wantDocCount:   1,
			wantBlockCount: 2,
		},
		{
			name: "pattern 2 - span verse",
			content: `<span class="verse" data-verse="1">In the beginning.</span>
<span class="verse" data-verse="2">And the earth was void.</span>`,
			artifactID:     "GEN",
			wantDocCount:   1,
			wantBlockCount: 2,
		},
		{
			name: "pattern 3 - span v",
			content: `<span class="v">1</span> In the beginning.
<span class="v">2</span> And the earth was void.`,
			artifactID:     "GEN",
			wantDocCount:   1,
			wantBlockCount: 2,
		},
		{
			name:           "empty content",
			content:        "",
			artifactID:     "GEN",
			wantDocCount:   1,
			wantBlockCount: 0,
		},
		{
			name:           "no verses",
			content:        "<p>This is plain text without verse markers.</p>",
			artifactID:     "GEN",
			wantDocCount:   1,
			wantBlockCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			docs := parseHTMLContent(tt.content, tt.artifactID)
			if len(docs) != tt.wantDocCount {
				t.Errorf("expected %d documents, got %d", tt.wantDocCount, len(docs))
			}
			if len(docs) > 0 && len(docs[0].ContentBlocks) != tt.wantBlockCount {
				t.Errorf("expected %d content blocks, got %d", tt.wantBlockCount, len(docs[0].ContentBlocks))
			}
		})
	}
}

// TestEscapeHTML tests the escapeHTML function.
func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"<text>", "&lt;text&gt;"},
		{"&", "&amp;"},
		{"\"quoted\"", "&quot;quoted&quot;"},
		{"a & b < c > d", "a &amp; b &lt; c &gt; d"},
		{"normal text", "normal text"},
		{"", ""},
		{"<script>alert('xss')</script>", "&lt;script&gt;alert('xss')&lt;/script&gt;"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeHTML(tt.input)
			if got != tt.want {
				t.Errorf("escapeHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestHTMLChapterParsing tests chapter number parsing from HTML headings.
func TestHTMLChapterParsing(t *testing.T) {
	content := `<h2>Chapter 5</h2>
<span class="verse" data-verse="3">Blessed are the poor.</span>
<h2>Chapter 6</h2>
<span class="verse" data-verse="9">Our Father.</span>`

	docs := parseHTMLContent(content, "MAT")
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}

	// Note: parseHTMLContent captures last chapter number found
	// This is a limitation of the current implementation
	if len(docs[0].ContentBlocks) != 2 {
		t.Errorf("expected 2 content blocks, got %d", len(docs[0].ContentBlocks))
	}
}

// TestHTMLDetectVariousPatterns tests detection of different HTML verse patterns.
func TestHTMLDetectVariousPatterns(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "html-test-*")
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
			name: "verse class pattern",
			content: `<html><body>
<p class="verse" data-verse="1"><span class="verse-text">In the beginning.</span></p>
</body></html>`,
			want: true,
		},
		{
			name: "span verse pattern",
			content: `<html><body>
<span class="verse" data-verse="1">In the beginning.</span>
</body></html>`,
			want: true,
		},
		{
			name: "span v pattern",
			content: `<html><body>
<span class="v">1</span> In the beginning.
</body></html>`,
			want: true,
		},
		{
			name:    "plain HTML",
			content: "<html><body><p>This is plain text.</p></body></html>",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, "test.html")
			if err := os.WriteFile(path, []byte(tt.content), 0600); err != nil {
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
				t.Errorf("expected detected=%v, got %v for %s", tt.want, detected, tt.name)
			}
		})
	}
}

// TestHTMLEmitNativeStructure tests that emit-native produces proper HTML structure.
func TestHTMLEmitNativeStructure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "html-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := ipc.Corpus{
		ID:         "GEN",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Language:   "en",
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
						Text:     "In the beginning.",
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
				},
			},
		},
	}

	irData, err := json.MarshalIndent(&corpus, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal IR: %v", err)
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	if err := os.WriteFile(irPath, irData, 0600); err != nil {
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
	htmlPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	htmlData, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("failed to read HTML file: %v", err)
	}

	htmlStr := string(htmlData)
	// Check for proper HTML structure
	if !strings.Contains(htmlStr, "<!DOCTYPE html>") {
		t.Error("output should contain DOCTYPE declaration")
	}
	if !strings.Contains(htmlStr, "<html lang=\"en\">") {
		t.Error("output should contain html tag with language")
	}
	if !strings.Contains(htmlStr, "<title>Genesis</title>") {
		t.Error("output should contain title tag")
	}
	if !strings.Contains(htmlStr, "class=\"verse\"") {
		t.Error("output should contain verse class")
	}
	if !strings.Contains(htmlStr, "data-verse=\"1\"") {
		t.Error("output should contain data-verse attribute")
	}
	if !strings.Contains(htmlStr, "In the beginning.") {
		t.Error("output should contain verse text")
	}
}

// TestHTMLEmitNativeEscaping tests that special characters are properly escaped.
func TestHTMLEmitNativeEscaping(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "html-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := ipc.Corpus{
		ID:         "TEST",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Test & <Special> \"Characters\"",
		Documents: []*ipc.Document{
			{
				ID:    "TEST",
				Title: "Book with <tags>",
				Order: 1,
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "Text with & < > \" characters.",
						Anchors: []*ipc.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*ipc.Span{
									{
										Type: "VERSE",
										Ref: &ipc.Ref{
											Book:    "TEST",
											Chapter: 1,
											Verse:   1,
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
	if err := os.WriteFile(irPath, irData, 0600); err != nil {
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
	htmlPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	htmlData, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("failed to read HTML file: %v", err)
	}

	htmlStr := string(htmlData)
	// Check that special characters are escaped
	if !strings.Contains(htmlStr, "&amp;") {
		t.Error("output should contain escaped &")
	}
	if !strings.Contains(htmlStr, "&lt;") {
		t.Error("output should contain escaped <")
	}
	if !strings.Contains(htmlStr, "&gt;") {
		t.Error("output should contain escaped >")
	}
	if !strings.Contains(htmlStr, "&quot;") {
		t.Error("output should contain escaped \"")
	}
	// Make sure raw characters aren't present in content (only in tags)
	if strings.Contains(htmlStr, "Text with & < > \" characters.") {
		t.Error("output should not contain unescaped special characters in text")
	}
}
