package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// createTestEPUB creates a minimal EPUB Bible file for testing.
func createTestEPUB(t *testing.T, path string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create EPUB: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// mimetype (must be first, uncompressed)
	mimetypeWriter, _ := w.CreateHeader(&zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	})
	mimetypeWriter.Write([]byte("application/epub+zip"))

	// META-INF/container.xml
	containerWriter, _ := w.Create("META-INF/container.xml")
	containerWriter.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`))

	// OEBPS/content.opf
	opfWriter, _ := w.Create("OEBPS/content.opf")
	opfWriter.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:uuid:test</dc:identifier>
    <dc:title>Test Bible</dc:title>
    <dc:language>en</dc:language>
  </metadata>
  <manifest>
    <item id="chapter1" href="chapter1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="chapter1"/>
  </spine>
</package>`))

	// OEBPS/chapter1.xhtml
	chapterWriter, _ := w.Create("OEBPS/chapter1.xhtml")
	chapterWriter.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><title>Genesis 1</title></head>
<body>
<h1>Genesis</h1>
<p class="verse"><span class="verse" data-verse="1">1</span> In the beginning God created.</p>
<p class="verse"><span class="verse" data-verse="2">2</span> And the earth was void.</p>
</body>
</html>`))
}

// TestEPUBDetect tests the detect command.
func TestEPUBDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "epub-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	epubPath := filepath.Join(tmpDir, "test.epub")
	createTestEPUB(t, epubPath)

	req := IPCRequest{
		Command: "detect",
		Args:    map[string]interface{}{"path": epubPath},
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
	if result["format"] != "EPUB" {
		t.Errorf("expected format EPUB, got %v", result["format"])
	}
}

// TestEPUBDetectNonEPUB tests detect command on non-EPUB file.
func TestEPUBDetectNonEPUB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "epub-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	txtPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtPath, []byte("Hello world"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	req := IPCRequest{
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
		t.Error("expected detected to be false for non-EPUB file")
	}
}

// TestEPUBExtractIR tests the extract-ir command.
func TestEPUBExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "epub-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	epubPath := filepath.Join(tmpDir, "test.epub")
	createTestEPUB(t, epubPath)

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := IPCRequest{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       epubPath,
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

	var corpus Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	if corpus.Title != "Test Bible" {
		t.Errorf("expected title Test Bible, got %s", corpus.Title)
	}
	if corpus.Language != "en" {
		t.Errorf("expected language en, got %s", corpus.Language)
	}
}

// TestEPUBEmitNative tests the emit-native command.
func TestEPUBEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "epub-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := Corpus{
		ID:         "test",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Test Bible",
		Language:   "en",
		Documents: []*Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "In the beginning.",
						Anchors: []*Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*Span{
									{
										ID:            "s-Gen.1.1",
										Type:          "VERSE",
										StartAnchorID: "a-1-0",
										Ref: &Ref{
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

	req := IPCRequest{
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

	if result["format"] != "EPUB" {
		t.Errorf("expected format EPUB, got %v", result["format"])
	}

	epubPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	// Verify the output is a valid EPUB
	r, err := zip.OpenReader(epubPath)
	if err != nil {
		t.Fatalf("failed to open output EPUB: %v", err)
	}
	defer r.Close()

	hasMimetype := false
	hasContainer := false
	for _, f := range r.File {
		if f.Name == "mimetype" {
			hasMimetype = true
		}
		if f.Name == "META-INF/container.xml" {
			hasContainer = true
		}
	}

	if !hasMimetype {
		t.Error("EPUB missing mimetype file")
	}
	if !hasContainer {
		t.Error("EPUB missing container.xml")
	}
}

// TestEPUBRoundTrip tests L0 lossless round-trip via raw storage.
func TestEPUBRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "epub-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	epubPath := filepath.Join(tmpDir, "original.epub")
	createTestEPUB(t, epubPath)

	originalData, _ := os.ReadFile(epubPath)

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)

	// Extract IR
	extractReq := IPCRequest{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       epubPath,
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
	emitReq := IPCRequest{
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

	if !bytes.Equal(originalData, outputData) {
		t.Errorf("round-trip mismatch: original %d bytes, output %d bytes", len(originalData), len(outputData))
	}

	// Verify loss class is L0 (lossless via raw storage)
	if emitResult["loss_class"] != "L0" {
		t.Errorf("expected loss_class L0 for round-trip, got %v", emitResult["loss_class"])
	}
}

// TestEPUBEnumerate tests the enumerate command.
func TestEPUBEnumerate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "epub-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	epubPath := filepath.Join(tmpDir, "test.epub")
	createTestEPUB(t, epubPath)

	req := IPCRequest{
		Command: "enumerate",
		Args:    map[string]interface{}{"path": epubPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	entries, ok := result["entries"].([]interface{})
	if !ok {
		t.Fatal("entries is not an array")
	}

	if len(entries) < 3 {
		t.Errorf("expected at least 3 entries, got %d", len(entries))
	}
}

// TestEPUBIngest tests the ingest command.
func TestEPUBIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "epub-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	epubPath := filepath.Join(tmpDir, "test.epub")
	createTestEPUB(t, epubPath)

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := IPCRequest{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       epubPath,
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
func executePlugin(t *testing.T, req *IPCRequest) *IPCResponse {
	t.Helper()

	pluginPath := "./format-epub"
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
			var resp IPCResponse
			if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
				return &resp
			}
		}
		t.Fatalf("plugin execution failed: %v\nstderr: %s", err, stderr.String())
	}

	var resp IPCResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\noutput: %s", err, stdout.String())
	}

	return &resp
}
