//go:build !sdk

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

// TestOSISDetect tests the detect command.
func TestOSISDetect(t *testing.T) {
	// Create a temporary OSIS file
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	osisContent := `<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="Test">
    <div type="book" osisID="Gen">
      <p><verse osisID="Gen.1.1"/>In the beginning.</p>
    </div>
  </osisText>
</osis>`

	osisPath := filepath.Join(tmpDir, "test.osis")
	if err := os.WriteFile(osisPath, []byte(osisContent), 0600); err != nil {
		t.Fatalf("failed to write OSIS file: %v", err)
	}

	// Test detect command
	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": osisPath},
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
	if result["format"] != "OSIS" {
		t.Errorf("expected format OSIS, got %v", result["format"])
	}
}

// TestOSISDetectNonOSIS tests detect command on non-OSIS file.
func TestOSISDetectNonOSIS(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
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
		t.Error("expected detected to be false for non-OSIS file")
	}
}

// TestOSISExtractIR tests the extract-ir command.
func TestOSISExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	osisContent := `<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="TestBible" xml:lang="en">
    <header>
      <work osisWork="TestBible">
        <title>Test Bible</title>
        <language>en</language>
      </work>
    </header>
    <div type="book" osisID="Gen">
      <title>Genesis</title>
      <p><verse osisID="Gen.1.1"/>In the beginning.</p>
    </div>
  </osisText>
</osis>`

	osisPath := filepath.Join(tmpDir, "test.osis")
	if err := os.WriteFile(osisPath, []byte(osisContent), 0600); err != nil {
		t.Fatalf("failed to write OSIS file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       osisPath,
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

	// Verify loss class is L0
	if result["loss_class"] != "L0" {
		t.Errorf("expected loss_class L0, got %v", result["loss_class"])
	}

	// Verify IR file was created
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

	if corpus.ID != "TestBible" {
		t.Errorf("expected ID TestBible, got %s", corpus.ID)
	}
	if corpus.Title != "Test Bible" {
		t.Errorf("expected title 'Test Bible', got %s", corpus.Title)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	if corpus.Documents[0].ID != "Gen" {
		t.Errorf("expected document ID Gen, got %s", corpus.Documents[0].ID)
	}
}

// TestOSISEmitNative tests the emit-native command.
func TestOSISEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create an IR file
	corpus := ipc.Corpus{
		ID:         "TestBible",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Language:   "en",
		Title:      "Test Bible",
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

	// Verify format is OSIS
	if result["format"] != "OSIS" {
		t.Errorf("expected format OSIS, got %v", result["format"])
	}

	// Verify OSIS file was created
	osisPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	osisData, err := os.ReadFile(osisPath)
	if err != nil {
		t.Fatalf("failed to read OSIS file: %v", err)
	}

	// Verify it's valid OSIS
	if !bytes.Contains(osisData, []byte("<osis")) {
		t.Error("output does not contain <osis tag")
	}
	if !bytes.Contains(osisData, []byte("TestBible")) {
		t.Error("output does not contain TestBible")
	}
}

// TestOSISRoundTrip tests L0 lossless round-trip.
func TestOSISRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Original OSIS content
	originalContent := `<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="TestBible" xml:lang="en">
    <header>
      <work osisWork="TestBible">
        <title>Test Bible</title>
      </work>
    </header>
    <div type="book" osisID="Gen">
      <title>Genesis</title>
      <p><verse osisID="Gen.1.1"/>In the beginning.</p>
    </div>
  </osisText>
</osis>
`

	osisPath := filepath.Join(tmpDir, "original.osis")
	if err := os.WriteFile(osisPath, []byte(originalContent), 0600); err != nil {
		t.Fatalf("failed to write OSIS file: %v", err)
	}

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)

	// Extract IR
	extractReq := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       osisPath,
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
	originalData, err := os.ReadFile(osisPath)
	if err != nil {
		t.Fatalf("failed to read original: %v", err)
	}

	outputData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	// Compute hashes
	originalHash := sha256.Sum256(originalData)
	outputHash := sha256.Sum256(outputData)

	if originalHash != outputHash {
		t.Errorf("L0 round-trip failed: hashes differ\noriginal: %s\noutput:   %s",
			hex.EncodeToString(originalHash[:]),
			hex.EncodeToString(outputHash[:]))
	}
}

// TestOSISIngest tests the ingest command.
func TestOSISIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	osisContent := `<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="TestBible">
    <div type="book" osisID="Gen">
      <p><verse osisID="Gen.1.1"/>In the beginning.</p>
    </div>
  </osisText>
</osis>`

	osisPath := filepath.Join(tmpDir, "test.osis")
	if err := os.WriteFile(osisPath, []byte(osisContent), 0600); err != nil {
		t.Fatalf("failed to write OSIS file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       osisPath,
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

	// Verify artifact ID was derived from osisIDWork
	if result["artifact_id"] != "TestBible" {
		t.Errorf("expected artifact_id TestBible, got %v", result["artifact_id"])
	}

	// Verify blob was created
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

	// Find plugin binary
	pluginPath := "./format-osis"
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		// Try building if not exists
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
		// Check if it's an expected error (plugin returns error status)
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

// TestParseOSISToIR tests the parseOSISToIR function directly.
func TestParseOSISToIR(t *testing.T) {
	osisData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="TestBible" xml:lang="en">
    <header>
      <work osisWork="TestBible">
        <title>Test Bible</title>
        <description>A test description</description>
        <publisher>Test Publisher</publisher>
        <rights>Public Domain</rights>
        <language>en</language>
        <refSystem>KJV</refSystem>
      </work>
    </header>
    <div type="book" osisID="Gen">
      <title>Genesis</title>
      <p><verse osisID="Gen.1.1"/>In the beginning God created.</p>
      <lg><l>Poetry line</l></lg>
    </div>
  </osisText>
</osis>`)

	corpus, err := parseOSISToIR(osisData)
	if err != nil {
		t.Fatalf("parseOSISToIR failed: %v", err)
	}

	// Check corpus metadata
	if corpus.ID != "TestBible" {
		t.Errorf("expected ID TestBible, got %s", corpus.ID)
	}
	if corpus.Title != "Test Bible" {
		t.Errorf("expected title 'Test Bible', got %s", corpus.Title)
	}
	if corpus.Description != "A test description" {
		t.Errorf("expected description, got %s", corpus.Description)
	}
	if corpus.Publisher != "Test Publisher" {
		t.Errorf("expected publisher, got %s", corpus.Publisher)
	}
	if corpus.Rights != "Public Domain" {
		t.Errorf("expected rights, got %s", corpus.Rights)
	}
	if corpus.Language != "en" {
		t.Errorf("expected language en, got %s", corpus.Language)
	}
	if corpus.Versification != "KJV" {
		t.Errorf("expected versification KJV, got %s", corpus.Versification)
	}
	if corpus.SourceFormat != "OSIS" {
		t.Errorf("expected source format OSIS, got %s", corpus.SourceFormat)
	}
	if corpus.LossClass != "L0" {
		t.Errorf("expected loss class L0, got %s", corpus.LossClass)
	}

	// Check documents
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	doc := corpus.Documents[0]
	if doc.ID != "Gen" {
		t.Errorf("expected document ID Gen, got %s", doc.ID)
	}
	if doc.Title != "Genesis" {
		t.Errorf("expected document title Genesis, got %s", doc.Title)
	}

	// Check content blocks
	if len(doc.ContentBlocks) < 1 {
		t.Fatalf("expected at least 1 content block, got %d", len(doc.ContentBlocks))
	}

	// Check that raw OSIS is stored for L0 round-trip
	if _, ok := corpus.Attributes["_osis_raw"]; !ok {
		t.Error("expected _osis_raw attribute for L0 round-trip")
	}
}

// TestEmitOSISFromIR tests the emitOSISFromIR function directly.
func TestEmitOSISFromIR(t *testing.T) {
	corpus := &ipc.Corpus{
		ID:         "TestBible",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Language:   "en",
		Title:      "Test Bible",
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

	osisData, err := emitOSISFromIR(corpus)
	if err != nil {
		t.Fatalf("emitOSISFromIR failed: %v", err)
	}

	osisStr := string(osisData)
	if !strings.Contains(osisStr, "<osis") {
		t.Error("output does not contain <osis tag")
	}
	if !strings.Contains(osisStr, "TestBible") {
		t.Error("output does not contain TestBible")
	}
	if !strings.Contains(osisStr, "Genesis") {
		t.Error("output does not contain Genesis")
	}
	if !strings.Contains(osisStr, "In the beginning.") {
		t.Error("output does not contain verse text")
	}
	if !strings.Contains(osisStr, `osisID="Gen.1.1"`) {
		t.Error("output does not contain verse osisID")
	}
}

// TestParseOSISRef tests the parseOSISRef function.
func TestParseOSISRef(t *testing.T) {
	tests := []struct {
		osisID   string
		wantBook string
		wantCh   int
		wantV    int
		wantVEnd int
	}{
		{"Gen.1.1", "Gen", 1, 1, 0},
		{"Matt.5.3", "Matt", 5, 3, 0},
		{"John.3.16-17", "John", 3, 16, 17},
		{"Ps.23.1", "Ps", 23, 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.osisID, func(t *testing.T) {
			ref := parseOSISRef(tt.osisID)
			if ref.Book != tt.wantBook {
				t.Errorf("expected book %s, got %s", tt.wantBook, ref.Book)
			}
			if ref.Chapter != tt.wantCh {
				t.Errorf("expected chapter %d, got %d", tt.wantCh, ref.Chapter)
			}
			if ref.Verse != tt.wantV {
				t.Errorf("expected verse %d, got %d", tt.wantV, ref.Verse)
			}
			if ref.VerseEnd != tt.wantVEnd {
				t.Errorf("expected verseEnd %d, got %d", tt.wantVEnd, ref.VerseEnd)
			}
			if ref.OSISID != tt.osisID {
				t.Errorf("expected OSISID %s, got %s", tt.osisID, ref.OSISID)
			}
		})
	}
}

// TestIsBookID tests the isBookID function.
func TestIsBookID(t *testing.T) {
	tests := []struct {
		osisID string
		want   bool
	}{
		{"Gen", true},
		{"Matt", true},
		{"Rev", true},
		{"1Cor", true},
		{"Gen.1", false},
		{"Gen.1.1", false},
		{"NotABook", false},
	}

	for _, tt := range tests {
		t.Run(tt.osisID, func(t *testing.T) {
			got := isBookID(tt.osisID)
			if got != tt.want {
				t.Errorf("isBookID(%s) = %v, want %v", tt.osisID, got, tt.want)
			}
		})
	}
}

// TestEscapeXML tests the escapeXML function.
func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"<text>", "&lt;text&gt;"},
		{"&", "&amp;"},
		{"\"quoted\"", "&quot;quoted&quot;"},
		{"'quoted'", "&apos;quoted&apos;"},
		{"a & b < c > d", "a &amp; b &lt; c &gt; d"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeXML(tt.input)
			if got != tt.want {
				t.Errorf("escapeXML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestHandleDetectErrors tests error cases in handleDetect.
func TestHandleDetectErrors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test directory
	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": tmpDir},
	}
	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}
	result := resp.Result.(map[string]interface{})
	if result["detected"] == true {
		t.Error("expected detected to be false for directory")
	}

	// Test non-existent file
	req = ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": "/nonexistent/file.xml"},
	}
	resp = executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}
	result = resp.Result.(map[string]interface{})
	if result["detected"] == true {
		t.Error("expected detected to be false for nonexistent file")
	}
}

// TestOSISDetectMalformedXML tests detection of malformed OSIS XML.
func TestOSISDetectMalformedXML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test file with OSIS markers but invalid XML
	malformedContent := `<osis><osisText>unclosed tags`
	malformedPath := filepath.Join(tmpDir, "malformed.osis")
	if err := os.WriteFile(malformedPath, []byte(malformedContent), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": malformedPath},
	}
	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}
	result := resp.Result.(map[string]interface{})
	// Should detect as OSIS based on markers even if XML is malformed
	if result["detected"] != true {
		t.Error("expected detected to be true for file with OSIS markers")
	}
}

// TestOSISDetectEmptyFile tests detection of empty file.
func TestOSISDetectEmptyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	emptyPath := filepath.Join(tmpDir, "empty.xml")
	if err := os.WriteFile(emptyPath, []byte(""), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": emptyPath},
	}
	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}
	result := resp.Result.(map[string]interface{})
	if result["detected"] == true {
		t.Error("expected detected to be false for empty file")
	}
}

// TestParseOSISRefEdgeCases tests edge cases in OSIS reference parsing.
func TestParseOSISRefEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		osisID   string
		wantBook string
		wantCh   int
		wantV    int
		wantVEnd int
	}{
		{"book only", "Gen", "Gen", 0, 0, 0},
		{"book and chapter", "Gen.1", "Gen", 1, 0, 0},
		{"multiple digit chapter", "Ps.119", "Ps", 119, 0, 0},
		{"verse range with dash", "John.3.16-18", "John", 3, 16, 18},
		{"single verse", "Matt.1.1", "Matt", 1, 1, 0},
		{"three digit verse", "Ps.119.176", "Ps", 119, 176, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := parseOSISRef(tt.osisID)
			if ref.Book != tt.wantBook {
				t.Errorf("expected book %s, got %s", tt.wantBook, ref.Book)
			}
			if ref.Chapter != tt.wantCh {
				t.Errorf("expected chapter %d, got %d", tt.wantCh, ref.Chapter)
			}
			if ref.Verse != tt.wantV {
				t.Errorf("expected verse %d, got %d", tt.wantV, ref.Verse)
			}
			if ref.VerseEnd != tt.wantVEnd {
				t.Errorf("expected verseEnd %d, got %d", tt.wantVEnd, ref.VerseEnd)
			}
		})
	}
}

// TestExtractContentBlocksWithNestedDivs tests nested div parsing.
func TestExtractContentBlocksWithNestedDivs(t *testing.T) {
	osisData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="Test">
    <div type="book" osisID="Gen">
      <div type="chapter">
        <chapter osisID="Gen.1" sID="Gen.1"/>
        <p><verse osisID="Gen.1.1"/>In the beginning.</p>
        <chapter eID="Gen.1"/>
      </div>
    </div>
  </osisText>
</osis>`)

	corpus, err := parseOSISToIR(osisData)
	if err != nil {
		t.Fatalf("parseOSISToIR failed: %v", err)
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) < 1 {
		t.Error("expected at least 1 content block from nested divs")
	}
}

// TestEmitOSISFromIRWithPoetry tests emitting OSIS with poetry blocks.
func TestEmitOSISFromIRWithPoetry(t *testing.T) {
	corpus := &ipc.Corpus{
		ID:         "Psalms",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Language:   "en",
		Title:      "Psalms",
		Documents: []*ipc.Document{
			{
				ID:    "Ps",
				Title: "Psalms",
				Order: 1,
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "The LORD is my shepherd",
						Attributes: map[string]interface{}{
							"type": "poetry",
						},
					},
				},
			},
		},
	}

	osisData, err := emitOSISFromIR(corpus)
	if err != nil {
		t.Fatalf("emitOSISFromIR failed: %v", err)
	}

	osisStr := string(osisData)
	if !strings.Contains(osisStr, "<lg>") {
		t.Error("output does not contain poetry line group marker <lg>")
	}
	if !strings.Contains(osisStr, "<l>") {
		t.Error("output does not contain poetry line marker <l>")
	}
}

// TestOSISEmitNativeWithEmptyCorpus tests emit-native with empty corpus.
func TestOSISEmitNativeWithEmptyCorpus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := ipc.Corpus{
		ID:         "Empty",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Documents:  []*ipc.Document{},
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

	osisPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	osisData, err := os.ReadFile(osisPath)
	if err != nil {
		t.Fatalf("failed to read OSIS file: %v", err)
	}

	// Should still produce valid OSIS structure
	if !bytes.Contains(osisData, []byte("<osis")) {
		t.Error("output does not contain <osis tag")
	}
}
