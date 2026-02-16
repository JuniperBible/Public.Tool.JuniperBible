package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// TestUSFMDetect tests the detect command.
func TestUSFMDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usfmContent := `\id GEN
\h Genesis
\c 1
\v 1 In the beginning.
`

	usfmPath := filepath.Join(tmpDir, "test.usfm")
	if err := os.WriteFile(usfmPath, []byte(usfmContent), 0600); err != nil {
		t.Fatalf("failed to write USFM file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": usfmPath},
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
	if result["format"] != "USFM" {
		t.Errorf("expected format USFM, got %v", result["format"])
	}
}

// TestUSFMDetectNonUSFM tests detect command on non-USFM file.
func TestUSFMDetectNonUSFM(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
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
		t.Error("expected detected to be false for non-USFM file")
	}
}

// TestUSFMExtractIR tests the extract-ir command.
func TestUSFMExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usfmContent := `\id GEN
\h Genesis
\mt Genesis
\c 1
\v 1 In the beginning God created.
\v 2 And the earth was void.
`

	usfmPath := filepath.Join(tmpDir, "test.usfm")
	if err := os.WriteFile(usfmPath, []byte(usfmContent), 0600); err != nil {
		t.Fatalf("failed to write USFM file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       usfmPath,
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

	if result["loss_class"] != "L0" {
		t.Errorf("expected loss_class L0, got %v", result["loss_class"])
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

	if corpus.ID != "GEN" {
		t.Errorf("expected ID GEN, got %s", corpus.ID)
	}
	if corpus.Title != "Genesis" {
		t.Errorf("expected title Genesis, got %s", corpus.Title)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("expected 2 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

// TestUSFMEmitNative tests the emit-native command.
func TestUSFMEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
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
						Text:     "In the beginning.",
						Anchors: []*ipc.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*ipc.Span{
									{
										ID:            "s-GEN.1.1",
										Type:          "VERSE",
										StartAnchorID: "a-1-0",
										Ref: &ipc.Ref{
											Book:    "GEN",
											Chapter: 1,
											Verse:   1,
											OSISID:  "GEN.1.1",
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

	if result["format"] != "USFM" {
		t.Errorf("expected format USFM, got %v", result["format"])
	}

	usfmPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	usfmData, err := os.ReadFile(usfmPath)
	if err != nil {
		t.Fatalf("failed to read USFM file: %v", err)
	}

	if !bytes.Contains(usfmData, []byte("\\id GEN")) {
		t.Error("output does not contain \\id marker")
	}
	if !bytes.Contains(usfmData, []byte("\\v 1")) {
		t.Error("output does not contain verse marker")
	}
}

// TestUSFMRoundTrip tests L0 lossless round-trip.
func TestUSFMRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalContent := `\id GEN
\h Genesis
\toc1 Genesis
\mt Genesis
\c 1
\v 1 In the beginning God created the heaven and the earth.
\v 2 And the earth was without form, and void.
\c 2
\v 1 Thus the heavens and the earth were finished.
`

	usfmPath := filepath.Join(tmpDir, "original.usfm")
	if err := os.WriteFile(usfmPath, []byte(originalContent), 0600); err != nil {
		t.Fatalf("failed to write USFM file: %v", err)
	}

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)

	// Extract IR
	extractReq := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       usfmPath,
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

	// Compare original and output
	originalData, err := os.ReadFile(usfmPath)
	if err != nil {
		t.Fatalf("failed to read original: %v", err)
	}

	outputData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	originalHash := sha256.Sum256(originalData)
	outputHash := sha256.Sum256(outputData)

	if originalHash != outputHash {
		t.Errorf("L0 round-trip failed: hashes differ\noriginal: %s\noutput:   %s",
			hex.EncodeToString(originalHash[:]),
			hex.EncodeToString(outputHash[:]))
	}
}

// TestUSFMIngest tests the ingest command.
func TestUSFMIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usfmContent := `\id GEN
\c 1
\v 1 In the beginning.
`

	usfmPath := filepath.Join(tmpDir, "test.usfm")
	if err := os.WriteFile(usfmPath, []byte(usfmContent), 0600); err != nil {
		t.Fatalf("failed to write USFM file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       usfmPath,
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

	if result["artifact_id"] != "GEN" {
		t.Errorf("expected artifact_id GEN, got %v", result["artifact_id"])
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

	pluginPath := "./format-usfm"
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

// Unit tests for internal functions

// TestParseUSFMToIR tests the parseUSFMToIR function directly.
func TestParseUSFMToIR(t *testing.T) {
	usfmData := []byte(`\id GEN
\h Genesis
\toc1 The Book of Genesis
\mt Genesis
\c 1
\v 1 In the beginning God created the heavens and the earth.
\v 2 And the earth was without form and void.
\p And God said, Let there be light.
\q Poetry line
`)

	corpus, err := parseUSFMToIR(usfmData)
	if err != nil {
		t.Fatalf("parseUSFMToIR failed: %v", err)
	}

	// Check corpus metadata
	if corpus.ID != "GEN" {
		t.Errorf("expected ID GEN, got %s", corpus.ID)
	}
	if corpus.Title != "Genesis" {
		t.Errorf("expected title Genesis, got %s", corpus.Title)
	}
	if corpus.SourceFormat != "USFM" {
		t.Errorf("expected source format USFM, got %s", corpus.SourceFormat)
	}
	if corpus.LossClass != "L0" {
		t.Errorf("expected loss class L0, got %s", corpus.LossClass)
	}

	// Check documents
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	doc := corpus.Documents[0]
	if doc.ID != "GEN" {
		t.Errorf("expected document ID GEN, got %s", doc.ID)
	}

	// Check content blocks (should have verses, paragraph, and poetry)
	if len(doc.ContentBlocks) < 2 {
		t.Fatalf("expected at least 2 content blocks, got %d", len(doc.ContentBlocks))
	}

	// Check that raw USFM is stored for L0 round-trip
	if _, ok := corpus.Attributes["_usfm_raw"]; !ok {
		t.Error("expected _usfm_raw attribute for L0 round-trip")
	}
}

// TestEmitUSFMFromIR tests the emitUSFMFromIR function directly.
func TestEmitUSFMFromIR(t *testing.T) {
	corpus := &ipc.Corpus{
		ID:         "GEN",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Genesis",
		Documents: []*ipc.Document{
			{
				ID:    "GEN",
				Title: "Genesis",
				Order: 1,
				Attributes: map[string]string{
					"toc1": "The Book of Genesis",
				},
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
										ID:            "s-GEN.1.1",
										Type:          "VERSE",
										StartAnchorID: "a-1-0",
										Ref: &ipc.Ref{
											Book:    "GEN",
											Chapter: 1,
											Verse:   1,
											OSISID:  "GEN.1.1",
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

	usfmData, err := emitUSFMFromIR(corpus)
	if err != nil {
		t.Fatalf("emitUSFMFromIR failed: %v", err)
	}

	usfmStr := string(usfmData)
	if !strings.Contains(usfmStr, "\\id GEN") {
		t.Error("output does not contain \\id marker")
	}
	if !strings.Contains(usfmStr, "\\h Genesis") {
		t.Error("output does not contain \\h marker")
	}
	if !strings.Contains(usfmStr, "\\c 1") {
		t.Error("output does not contain chapter marker")
	}
	if !strings.Contains(usfmStr, "\\v 1") {
		t.Error("output does not contain verse marker")
	}
	if !strings.Contains(usfmStr, "In the beginning.") {
		t.Error("output does not contain verse text")
	}
}

// TestUSFMMarkerParsing tests the parsing of various USFM markers.
func TestUSFMMarkerParsing(t *testing.T) {
	usfmData := []byte(`\id MAT
\c 5
\v 3-5 Blessed are the poor in spirit.
`)

	corpus, err := parseUSFMToIR(usfmData)
	if err != nil {
		t.Fatalf("parseUSFMToIR failed: %v", err)
	}

	// Check that verse range is parsed
	if len(corpus.Documents) > 0 && len(corpus.Documents[0].ContentBlocks) > 0 {
		cb := corpus.Documents[0].ContentBlocks[0]
		if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 {
			ref := cb.Anchors[0].Spans[0].Ref
			if ref.Verse != 3 {
				t.Errorf("expected verse 3, got %d", ref.Verse)
			}
			if ref.VerseEnd != 5 {
				t.Errorf("expected verseEnd 5, got %d", ref.VerseEnd)
			}
		}
	}
}

// TestUSFMDetectVariousExtensions tests detection of USFM files with different extensions.
func TestUSFMDetectVariousExtensions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usfmContent := `\id GEN
\c 1
\v 1 In the beginning.
`

	extensions := []string{".usfm", ".sfm", ".txt", ".usx"}
	for _, ext := range extensions {
		path := filepath.Join(tmpDir, "test"+ext)
		if err := os.WriteFile(path, []byte(usfmContent), 0600); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		req := ipc.Request{
			Command: "detect",
			Args:    map[string]interface{}{"path": path},
		}

		resp := executePlugin(t, &req)
		if resp.Status != "ok" {
			t.Fatalf("expected status ok for %s, got %s", ext, resp.Status)
		}

		result := resp.Result.(map[string]interface{})
		if result["detected"] != true {
			t.Errorf("expected detected to be true for %s extension", ext)
		}
	}
}

// TestUSFMParsePoetryMarkers tests parsing of poetry markers.
func TestUSFMParsePoetryMarkers(t *testing.T) {
	usfmData := []byte(`\id PSA
\c 23
\q1 The LORD is my shepherd;
\q2 I shall not want.
\q1 He makes me lie down in green pastures.
`)

	corpus, err := parseUSFMToIR(usfmData)
	if err != nil {
		t.Fatalf("parseUSFMToIR failed: %v", err)
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}

	// Check that poetry lines were parsed
	foundPoetry := false
	for _, cb := range corpus.Documents[0].ContentBlocks {
		if cb.Attributes != nil {
			if blockType, ok := cb.Attributes["type"].(string); ok && strings.Contains(blockType, "poetry") {
				foundPoetry = true
				break
			}
		}
	}

	if !foundPoetry {
		t.Error("expected to find poetry blocks in Psalms")
	}
}

// TestUSFMEmptyIdMarker tests handling of USFM with empty id marker.
func TestUSFMEmptyIdMarker(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usfmContent := `\id
\c 1
\v 1 Some text.
`

	usfmPath := filepath.Join(tmpDir, "test.usfm")
	if err := os.WriteFile(usfmPath, []byte(usfmContent), 0600); err != nil {
		t.Fatalf("failed to write USFM file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": usfmPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}

	result := resp.Result.(map[string]interface{})
	if result["detected"] != true {
		t.Error("expected detected to be true even with empty id")
	}
}

// TestUSFMMultipleChapters tests parsing multiple chapters.
func TestUSFMMultipleChapters(t *testing.T) {
	usfmData := []byte(`\id GEN
\h Genesis
\c 1
\v 1 In the beginning.
\v 2 And the earth was void.
\c 2
\v 1 Thus the heavens were finished.
\v 2 And on the seventh day.
\c 3
\v 1 Now the serpent was subtle.
`)

	corpus, err := parseUSFMToIR(usfmData)
	if err != nil {
		t.Fatalf("parseUSFMToIR failed: %v", err)
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}

	doc := corpus.Documents[0]
	// Should have content blocks for verses across all 3 chapters
	if len(doc.ContentBlocks) < 5 {
		t.Errorf("expected at least 5 content blocks for 5 verses, got %d", len(doc.ContentBlocks))
	}

	// Check that chapters are properly tracked in verse refs
	foundChapter1 := false
	foundChapter3 := false
	for _, cb := range doc.ContentBlocks {
		if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 {
			ref := cb.Anchors[0].Spans[0].Ref
			if ref != nil {
				if ref.Chapter == 1 {
					foundChapter1 = true
				}
				if ref.Chapter == 3 {
					foundChapter3 = true
				}
			}
		}
	}

	if !foundChapter1 || !foundChapter3 {
		t.Error("expected to find verses from chapters 1 and 3")
	}
}

// TestUSFMBookNames tests book name lookup.
func TestUSFMBookNames(t *testing.T) {
	tests := []struct {
		bookID   string
		wantName string
	}{
		{"GEN", "Genesis"},
		{"PSA", "Psalms"},
		{"MAT", "Matthew"},
		{"REV", "Revelation"},
		{"1CO", "1 Corinthians"},
		{"XXX", ""}, // Unknown book
	}

	for _, tt := range tests {
		t.Run(tt.bookID, func(t *testing.T) {
			gotName := bookNames[tt.bookID]
			if gotName != tt.wantName {
				t.Errorf("bookNames[%s] = %q, want %q", tt.bookID, gotName, tt.wantName)
			}
		})
	}
}

// TestUSFMEmitWithAttributes tests emitting USFM with document attributes.
func TestUSFMEmitWithAttributes(t *testing.T) {
	corpus := &ipc.Corpus{
		ID:         "GEN",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Genesis",
		Documents: []*ipc.Document{
			{
				ID:    "GEN",
				Title: "Genesis",
				Order: 1,
				Attributes: map[string]string{
					"toc1": "The First Book of Moses",
					"toc2": "Genesis",
					"toc3": "Gen",
				},
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
										ID:            "s-GEN.1.1",
										Type:          "VERSE",
										StartAnchorID: "a-1-0",
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

	usfmData, err := emitUSFMFromIR(corpus)
	if err != nil {
		t.Fatalf("emitUSFMFromIR failed: %v", err)
	}

	usfmStr := string(usfmData)
	if !strings.Contains(usfmStr, "\\toc1") {
		t.Error("output should contain toc1 marker from attributes")
	}
	if !strings.Contains(usfmStr, "\\toc2") {
		t.Error("output should contain toc2 marker from attributes")
	}
}
