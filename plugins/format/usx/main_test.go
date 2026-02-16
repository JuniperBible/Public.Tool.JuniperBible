package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// TestUSXDetect tests the detect command with valid USX file.
func TestUSXDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usxContent := `<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
  <chapter number="1" style="c" sid="GEN.1"/>
  <para style="p">
    <verse number="1" style="v" sid="GEN.1.1"/>In the beginning.<verse eid="GEN.1.1"/>
  </para>
</usx>
`

	usxPath := filepath.Join(tmpDir, "test.usx")
	if err := os.WriteFile(usxPath, []byte(usxContent), 0600); err != nil {
		t.Fatalf("failed to write USX file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": usxPath},
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
	if result["format"] != "USX" {
		t.Errorf("expected format USX, got %v", result["format"])
	}
	if !strings.Contains(result["reason"].(string), "USX") {
		t.Errorf("expected reason to mention USX, got %v", result["reason"])
	}
}

// TestUSXDetectNonUSX tests detect command on non-USX file.
func TestUSXDetectNonUSX(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
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
		t.Error("expected detected to be false for non-USX file")
	}
	if !strings.Contains(result["reason"].(string), "no <usx> element") {
		t.Errorf("expected reason to mention missing usx element, got %v", result["reason"])
	}
}

// TestUSXDetectInvalidXML tests detect command on invalid XML.
func TestUSXDetectInvalidXML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invalidXML := `<usx version="3.0"><book code="GEN">Genesis</book><unclosed>`
	xmlPath := filepath.Join(tmpDir, "invalid.xml")
	if err := os.WriteFile(xmlPath, []byte(invalidXML), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": xmlPath},
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
		t.Error("expected detected to be false for invalid XML")
	}
	if !strings.Contains(result["reason"].(string), "invalid XML") {
		t.Errorf("expected reason to mention invalid XML, got %v", result["reason"])
	}
}

// TestUSXDetectDirectory tests detect command on a directory.
func TestUSXDetectDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": tmpDir},
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
		t.Error("expected detected to be false for directory")
	}
	if !strings.Contains(result["reason"].(string), "directory") {
		t.Errorf("expected reason to mention directory, got %v", result["reason"])
	}
}

// TestUSXDetectNonExistent tests detect command on non-existent file.
func TestUSXDetectNonExistent(t *testing.T) {
	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": "/nonexistent/file.usx"},
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
		t.Error("expected detected to be false for nonexistent file")
	}
	if !strings.Contains(result["reason"].(string), "cannot stat") {
		t.Errorf("expected reason to mention stat error, got %v", result["reason"])
	}
}

// TestUSXDetectMissingPath tests detect command without path argument.
func TestUSXDetectMissingPath(t *testing.T) {
	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "error" {
		t.Fatalf("expected status error, got %s", resp.Status)
	}

	if !strings.Contains(resp.Error, "path argument required") {
		t.Errorf("expected error about missing path, got %s", resp.Error)
	}
}

// TestUSXExtractIR tests the extract-ir command.
func TestUSXExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usxContent := `<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
  <chapter number="1" style="c" sid="GEN.1"/>
  <para style="p">
    <verse number="1" style="v" sid="GEN.1.1"/>In the beginning God created.<verse eid="GEN.1.1"/>
    <verse number="2" style="v" sid="GEN.1.2"/>And the earth was void.<verse eid="GEN.1.2"/>
  </para>
</usx>
`

	usxPath := filepath.Join(tmpDir, "test.usx")
	if err := os.WriteFile(usxPath, []byte(usxContent), 0600); err != nil {
		t.Fatalf("failed to write USX file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       usxPath,
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
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("expected 2 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}

	// Verify raw USX is stored for L0
	if _, ok := corpus.Attributes["_usx_raw"]; !ok {
		t.Error("expected _usx_raw attribute for L0 round-trip")
	}

	// Verify source hash
	if corpus.SourceHash == "" {
		t.Error("expected non-empty source hash")
	}

	// Verify loss report
	lossReport := result["loss_report"].(map[string]interface{})
	if lossReport["source_format"] != "USX" {
		t.Errorf("expected source_format USX, got %v", lossReport["source_format"])
	}
	if lossReport["target_format"] != "IR" {
		t.Errorf("expected target_format IR, got %v", lossReport["target_format"])
	}
}

// TestUSXExtractIRMultipleChapters tests extract-ir with multiple chapters.
func TestUSXExtractIRMultipleChapters(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usxContent := `<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
  <chapter number="1" style="c" sid="GEN.1"/>
  <para style="p">
    <verse number="1" style="v" sid="GEN.1.1"/>Chapter 1 verse 1.<verse eid="GEN.1.1"/>
    <verse number="2" style="v" sid="GEN.1.2"/>Chapter 1 verse 2.<verse eid="GEN.1.2"/>
  </para>
  <chapter number="2" style="c" sid="GEN.2"/>
  <para style="p">
    <verse number="1" style="v" sid="GEN.2.1"/>Chapter 2 verse 1.<verse eid="GEN.2.1"/>
  </para>
</usx>
`

	usxPath := filepath.Join(tmpDir, "test.usx")
	if err := os.WriteFile(usxPath, []byte(usxContent), 0600); err != nil {
		t.Fatalf("failed to write USX file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       usxPath,
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

	if len(corpus.Documents[0].ContentBlocks) != 3 {
		t.Errorf("expected 3 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}

	// Verify chapter/verse references
	cb1 := corpus.Documents[0].ContentBlocks[0]
	if cb1.Anchors[0].Spans[0].Ref.Chapter != 1 || cb1.Anchors[0].Spans[0].Ref.Verse != 1 {
		t.Errorf("expected chapter 1 verse 1, got %d:%d",
			cb1.Anchors[0].Spans[0].Ref.Chapter, cb1.Anchors[0].Spans[0].Ref.Verse)
	}

	cb3 := corpus.Documents[0].ContentBlocks[2]
	if cb3.Anchors[0].Spans[0].Ref.Chapter != 2 || cb3.Anchors[0].Spans[0].Ref.Verse != 1 {
		t.Errorf("expected chapter 2 verse 1, got %d:%d",
			cb3.Anchors[0].Spans[0].Ref.Chapter, cb3.Anchors[0].Spans[0].Ref.Verse)
	}
}

// TestUSXExtractIRMissingArgs tests extract-ir with missing arguments.
func TestUSXExtractIRMissingArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want string
	}{
		{
			name: "missing path",
			args: map[string]interface{}{"output_dir": "/tmp"},
			want: "path argument required",
		},
		{
			name: "missing output_dir",
			args: map[string]interface{}{"path": "/tmp/test.usx"},
			want: "output_dir argument required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ipc.Request{
				Command: "extract-ir",
				Args:    tt.args,
			}

			resp := executePlugin(t, &req)
			if resp.Status != "error" {
				t.Fatalf("expected status error, got %s", resp.Status)
			}

			if !strings.Contains(resp.Error, tt.want) {
				t.Errorf("expected error to contain %q, got %s", tt.want, resp.Error)
			}
		})
	}
}

// TestUSXExtractIRInvalidFile tests extract-ir with invalid file.
func TestUSXExtractIRInvalidFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invalidXML := `<usx><book>broken`
	xmlPath := filepath.Join(tmpDir, "invalid.usx")
	if err := os.WriteFile(xmlPath, []byte(invalidXML), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       xmlPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "error" {
		t.Fatalf("expected status error, got %s", resp.Status)
	}

	if !strings.Contains(resp.Error, "failed to parse USX") {
		t.Errorf("expected error about parsing USX, got %s", resp.Error)
	}
}

// TestUSXEmitNative tests the emit-native command.
func TestUSXEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
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

	if result["format"] != "USX" {
		t.Errorf("expected format USX, got %v", result["format"])
	}

	if result["loss_class"] != "L1" {
		t.Errorf("expected loss_class L1 for regenerated USX, got %v", result["loss_class"])
	}

	usxPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	usxData, err := os.ReadFile(usxPath)
	if err != nil {
		t.Fatalf("failed to read USX file: %v", err)
	}

	if !bytes.Contains(usxData, []byte("<usx")) {
		t.Error("output does not contain <usx> element")
	}
	if !bytes.Contains(usxData, []byte(`code="GEN"`)) {
		t.Error("output does not contain book code")
	}
	if !bytes.Contains(usxData, []byte("In the beginning.")) {
		t.Error("output does not contain verse text")
	}

	// Verify it's valid XML
	var usx USX
	if err := xml.Unmarshal(usxData, &usx); err != nil {
		t.Errorf("generated USX is not valid XML: %v", err)
	}
}

// TestUSXEmitNativeWithRaw tests emit-native with raw USX (L0 round-trip).
func TestUSXEmitNativeWithRaw(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	rawUSX := `<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
  <chapter number="1" style="c" sid="GEN.1"/>
  <para style="p">
    <verse number="1" style="v" sid="GEN.1.1"/>Original text.<verse eid="GEN.1.1"/>
  </para>
</usx>
`

	corpus := ipc.Corpus{
		ID:         "GEN",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Genesis",
		Attributes: map[string]string{
			"_usx_raw": rawUSX,
		},
		Documents: []*ipc.Document{
			{
				ID:    "GEN",
				Title: "Genesis",
				Order: 1,
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

	if result["loss_class"] != "L0" {
		t.Errorf("expected loss_class L0 for raw USX, got %v", result["loss_class"])
	}

	usxPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	usxData, err := os.ReadFile(usxPath)
	if err != nil {
		t.Fatalf("failed to read USX file: %v", err)
	}

	if string(usxData) != rawUSX {
		t.Error("L0 round-trip did not preserve raw USX exactly")
	}
}

// TestUSXEmitNativeMissingArgs tests emit-native with missing arguments.
func TestUSXEmitNativeMissingArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want string
	}{
		{
			name: "missing ir_path",
			args: map[string]interface{}{"output_dir": "/tmp"},
			want: "ir_path argument required",
		},
		{
			name: "missing output_dir",
			args: map[string]interface{}{"ir_path": "/tmp/test.ir.json"},
			want: "output_dir argument required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ipc.Request{
				Command: "emit-native",
				Args:    tt.args,
			}

			resp := executePlugin(t, &req)
			if resp.Status != "error" {
				t.Fatalf("expected status error, got %s", resp.Status)
			}

			if !strings.Contains(resp.Error, tt.want) {
				t.Errorf("expected error to contain %q, got %s", tt.want, resp.Error)
			}
		})
	}
}

// TestUSXEmitNativeInvalidIR tests emit-native with invalid IR file.
func TestUSXEmitNativeInvalidIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invalidJSON := `{"invalid": json`
	irPath := filepath.Join(tmpDir, "invalid.ir.json")
	if err := os.WriteFile(irPath, []byte(invalidJSON), 0600); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)

	req := ipc.Request{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "error" {
		t.Fatalf("expected status error, got %s", resp.Status)
	}

	if !strings.Contains(resp.Error, "failed to parse IR") {
		t.Errorf("expected error about parsing IR, got %s", resp.Error)
	}
}

// TestUSXRoundTrip tests L0 lossless round-trip.
func TestUSXRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalContent := `<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
  <chapter number="1" style="c" sid="GEN.1"/>
  <para style="p">
    <verse number="1" style="v" sid="GEN.1.1"/>In the beginning God created the heaven and the earth.<verse eid="GEN.1.1"/>
    <verse number="2" style="v" sid="GEN.1.2"/>And the earth was without form, and void.<verse eid="GEN.1.2"/>
  </para>
  <chapter number="2" style="c" sid="GEN.2"/>
  <para style="p">
    <verse number="1" style="v" sid="GEN.2.1"/>Thus the heavens and the earth were finished.<verse eid="GEN.2.1"/>
  </para>
</usx>
`

	usxPath := filepath.Join(tmpDir, "original.usx")
	if err := os.WriteFile(usxPath, []byte(originalContent), 0600); err != nil {
		t.Fatalf("failed to write USX file: %v", err)
	}

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)

	// Extract IR
	extractReq := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       usxPath,
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

	// Verify L0 loss class
	if emitResult["loss_class"] != "L0" {
		t.Errorf("expected loss_class L0, got %v", emitResult["loss_class"])
	}

	// Compare original and output
	originalData, err := os.ReadFile(usxPath)
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

// TestUSXIngest tests the ingest command.
func TestUSXIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usxContent := `<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
  <chapter number="1" style="c" sid="GEN.1"/>
  <para style="p">
    <verse number="1" style="v" sid="GEN.1.1"/>In the beginning.<verse eid="GEN.1.1"/>
  </para>
</usx>
`

	usxPath := filepath.Join(tmpDir, "test.usx")
	if err := os.WriteFile(usxPath, []byte(usxContent), 0600); err != nil {
		t.Fatalf("failed to write USX file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       usxPath,
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

	// Verify blob was created in correct location
	blobPath := filepath.Join(outputDir, blobHash[:2], blobHash)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("blob file was not created")
	}

	// Verify blob content
	blobData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("failed to read blob: %v", err)
	}

	if string(blobData) != usxContent {
		t.Error("blob content does not match original")
	}

	// Verify size
	if int64(len(usxContent)) != int64(result["size_bytes"].(float64)) {
		t.Errorf("expected size %d, got %v", len(usxContent), result["size_bytes"])
	}

	// Verify metadata
	metadata := result["metadata"].(map[string]interface{})
	if metadata["format"] != "USX" {
		t.Errorf("expected format USX, got %v", metadata["format"])
	}
}

// TestUSXIngestWithoutBookCode tests ingest command on USX without book code.
func TestUSXIngestWithoutBookCode(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usxContent := `<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <para style="p">Some text</para>
</usx>
`

	usxPath := filepath.Join(tmpDir, "myfile.usx")
	if err := os.WriteFile(usxPath, []byte(usxContent), 0600); err != nil {
		t.Fatalf("failed to write USX file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "blobs")
	os.MkdirAll(outputDir, 0755)

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       usxPath,
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

	// Should fall back to filename
	if result["artifact_id"] != "myfile" {
		t.Errorf("expected artifact_id myfile, got %v", result["artifact_id"])
	}
}

// TestUSXIngestMissingArgs tests ingest with missing arguments.
func TestUSXIngestMissingArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want string
	}{
		{
			name: "missing path",
			args: map[string]interface{}{"output_dir": "/tmp"},
			want: "path argument required",
		},
		{
			name: "missing output_dir",
			args: map[string]interface{}{"path": "/tmp/test.usx"},
			want: "output_dir argument required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ipc.Request{
				Command: "ingest",
				Args:    tt.args,
			}

			resp := executePlugin(t, &req)
			if resp.Status != "error" {
				t.Fatalf("expected status error, got %s", resp.Status)
			}

			if !strings.Contains(resp.Error, tt.want) {
				t.Errorf("expected error to contain %q, got %s", tt.want, resp.Error)
			}
		})
	}
}

// TestUSXEnumerate tests the enumerate command.
func TestUSXEnumerate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	usxContent := `<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
</usx>
`

	usxPath := filepath.Join(tmpDir, "test.usx")
	if err := os.WriteFile(usxPath, []byte(usxContent), 0600); err != nil {
		t.Fatalf("failed to write USX file: %v", err)
	}

	req := ipc.Request{
		Command: "enumerate",
		Args:    map[string]interface{}{"path": usxPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	entries := result["entries"].([]interface{})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0].(map[string]interface{})
	if entry["path"] != "test.usx" {
		t.Errorf("expected path test.usx, got %v", entry["path"])
	}
	if entry["is_dir"] == true {
		t.Error("expected is_dir to be false")
	}

	metadata := entry["metadata"].(map[string]interface{})
	if metadata["format"] != "USX" {
		t.Errorf("expected format USX, got %v", metadata["format"])
	}
}

// TestUSXEnumerateMissingPath tests enumerate with missing path.
func TestUSXEnumerateMissingPath(t *testing.T) {
	req := ipc.Request{
		Command: "enumerate",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "error" {
		t.Fatalf("expected status error, got %s", resp.Status)
	}

	if !strings.Contains(resp.Error, "path argument required") {
		t.Errorf("expected error about missing path, got %s", resp.Error)
	}
}

// TestUnknownCommand tests handling of unknown command.
func TestUnknownCommand(t *testing.T) {
	req := ipc.Request{
		Command: "unknown",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "error" {
		t.Fatalf("expected status error, got %s", resp.Status)
	}

	if !strings.Contains(resp.Error, "unknown command") {
		t.Errorf("expected error about unknown command, got %s", resp.Error)
	}
}

// TestInvalidJSON tests handling of invalid JSON request.
func TestInvalidJSON(t *testing.T) {
	pluginPath := "./format-usx"
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
		if err := buildCmd.Run(); err != nil {
			t.Fatalf("failed to build plugin: %v", err)
		}
	}

	cmd := exec.Command(pluginPath)
	cmd.Stdin = bytes.NewReader([]byte(`{invalid json`))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		t.Fatal("expected plugin to fail with invalid JSON")
	}

	if stdout.Len() > 0 {
		var resp ipc.Response
		if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
			if resp.Status != "error" {
				t.Errorf("expected error status, got %s", resp.Status)
			}
			if !strings.Contains(resp.Error, "failed to decode request") {
				t.Errorf("expected error about decoding, got %s", resp.Error)
			}
		}
	}
}

// Unit tests for helper functions

// TestParseUSXToIR tests the parseUSXToIR function.
func TestParseUSXToIR(t *testing.T) {
	usxData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
  <chapter number="1" style="c" sid="GEN.1"/>
  <para style="p">
    <verse number="1" style="v" sid="GEN.1.1"/>In the beginning God created.<verse eid="GEN.1.1"/>
    <verse number="2" style="v" sid="GEN.1.2"/>And the earth was void.<verse eid="GEN.1.2"/>
  </para>
</usx>`)

	corpus, err := parseUSXToIR(usxData)
	if err != nil {
		t.Fatalf("parseUSXToIR failed: %v", err)
	}

	if corpus.ID != "GEN" {
		t.Errorf("expected ID GEN, got %s", corpus.ID)
	}
	if corpus.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", corpus.Version)
	}
	if corpus.ModuleType != "BIBLE" {
		t.Errorf("expected module type BIBLE, got %s", corpus.ModuleType)
	}
	if corpus.SourceFormat != "USX" {
		t.Errorf("expected source format USX, got %s", corpus.SourceFormat)
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}

	doc := corpus.Documents[0]
	if doc.ID != "GEN" {
		t.Errorf("expected document ID GEN, got %s", doc.ID)
	}
	if doc.Title != "GEN" {
		t.Errorf("expected document title GEN, got %s", doc.Title)
	}

	if len(doc.ContentBlocks) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(doc.ContentBlocks))
	}

	cb1 := doc.ContentBlocks[0]
	if cb1.Text != "In the beginning God created." {
		t.Errorf("unexpected text in content block 1: %s", cb1.Text)
	}
	if len(cb1.Anchors) != 1 {
		t.Fatalf("expected 1 anchor, got %d", len(cb1.Anchors))
	}
	if len(cb1.Anchors[0].Spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(cb1.Anchors[0].Spans))
	}

	span := cb1.Anchors[0].Spans[0]
	if span.Type != "VERSE" {
		t.Errorf("expected span type VERSE, got %s", span.Type)
	}
	if span.Ref == nil {
		t.Fatal("expected ref to be non-nil")
	}
	if span.Ref.Book != "GEN" {
		t.Errorf("expected book GEN, got %s", span.Ref.Book)
	}
	if span.Ref.Chapter != 1 {
		t.Errorf("expected chapter 1, got %d", span.Ref.Chapter)
	}
	if span.Ref.Verse != 1 {
		t.Errorf("expected verse 1, got %d", span.Ref.Verse)
	}
	if span.Ref.OSISID != "GEN.1.1" {
		t.Errorf("expected OSISID GEN.1.1, got %s", span.Ref.OSISID)
	}
}

// TestEmitUSXFromIR tests the emitUSXFromIR function.
func TestEmitUSXFromIR(t *testing.T) {
	corpus := &ipc.Corpus{
		ID:         "GEN",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Attributes: map[string]string{
			"usx_version": "3.0",
		},
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
										ID:            "s-GEN.1.2",
										Type:          "VERSE",
										StartAnchorID: "a-2-0",
										Ref: &ipc.Ref{
											Book:    "GEN",
											Chapter: 1,
											Verse:   2,
											OSISID:  "GEN.1.2",
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

	usxStr := emitUSXFromIR(corpus)

	if !strings.Contains(usxStr, `<usx version="3.0">`) {
		t.Error("output does not contain correct USX version")
	}
	if !strings.Contains(usxStr, `<book code="GEN"`) {
		t.Error("output does not contain book element")
	}
	if !strings.Contains(usxStr, `<chapter number="1"`) {
		t.Error("output does not contain chapter element")
	}
	if !strings.Contains(usxStr, `<verse number="1"`) {
		t.Error("output does not contain verse 1")
	}
	if !strings.Contains(usxStr, `<verse number="2"`) {
		t.Error("output does not contain verse 2")
	}
	if !strings.Contains(usxStr, "In the beginning.") {
		t.Error("output does not contain verse 1 text")
	}
	if !strings.Contains(usxStr, "And the earth was void.") {
		t.Error("output does not contain verse 2 text")
	}

	// Verify it's valid XML
	var usx USX
	if err := xml.Unmarshal([]byte(usxStr), &usx); err != nil {
		t.Errorf("generated USX is not valid XML: %v", err)
	}
}

// TestEmitUSXFromIRMultipleChapters tests emitting USX with multiple chapters.
func TestEmitUSXFromIRMultipleChapters(t *testing.T) {
	corpus := &ipc.Corpus{
		ID:         "GEN",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Documents: []*ipc.Document{
			{
				ID:    "GEN",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "Chapter 1 verse 1.",
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
					{
						ID:       "cb-2",
						Sequence: 2,
						Text:     "Chapter 2 verse 1.",
						Anchors: []*ipc.Anchor{
							{
								ID:       "a-2-0",
								Position: 0,
								Spans: []*ipc.Span{
									{
										ID:            "s-GEN.2.1",
										Type:          "VERSE",
										StartAnchorID: "a-2-0",
										Ref: &ipc.Ref{
											Book:    "GEN",
											Chapter: 2,
											Verse:   1,
											OSISID:  "GEN.2.1",
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

	usxStr := emitUSXFromIR(corpus)

	if !strings.Contains(usxStr, `<chapter number="1"`) {
		t.Error("output does not contain chapter 1")
	}
	if !strings.Contains(usxStr, `<chapter number="2"`) {
		t.Error("output does not contain chapter 2")
	}
	if !strings.Contains(usxStr, "Chapter 1 verse 1.") {
		t.Error("output does not contain chapter 1 text")
	}
	if !strings.Contains(usxStr, "Chapter 2 verse 1.") {
		t.Error("output does not contain chapter 2 text")
	}
}

// TestCreateContentBlock tests the createContentBlock function.
func TestCreateContentBlock(t *testing.T) {
	cb := createContentBlock(1, "Test verse text", "GEN", 1, 1)

	if cb.ID != "cb-1" {
		t.Errorf("expected ID cb-1, got %s", cb.ID)
	}
	if cb.Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", cb.Sequence)
	}
	if cb.Text != "Test verse text" {
		t.Errorf("unexpected text: %s", cb.Text)
	}
	if cb.Hash == "" {
		t.Error("expected non-empty hash")
	}

	if len(cb.Anchors) != 1 {
		t.Fatalf("expected 1 anchor, got %d", len(cb.Anchors))
	}

	anchor := cb.Anchors[0]
	if anchor.ID != "a-1-0" {
		t.Errorf("expected anchor ID a-1-0, got %s", anchor.ID)
	}
	if anchor.Position != 0 {
		t.Errorf("expected position 0, got %d", anchor.Position)
	}

	if len(anchor.Spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(anchor.Spans))
	}

	span := anchor.Spans[0]
	if span.ID != "s-GEN.1.1" {
		t.Errorf("expected span ID s-GEN.1.1, got %s", span.ID)
	}
	if span.Type != "VERSE" {
		t.Errorf("expected span type VERSE, got %s", span.Type)
	}
	if span.StartAnchorID != "a-1-0" {
		t.Errorf("expected start anchor ID a-1-0, got %s", span.StartAnchorID)
	}

	if span.Ref == nil {
		t.Fatal("expected ref to be non-nil")
	}
	if span.Ref.Book != "GEN" {
		t.Errorf("expected book GEN, got %s", span.Ref.Book)
	}
	if span.Ref.Chapter != 1 {
		t.Errorf("expected chapter 1, got %d", span.Ref.Chapter)
	}
	if span.Ref.Verse != 1 {
		t.Errorf("expected verse 1, got %d", span.Ref.Verse)
	}
	if span.Ref.OSISID != "GEN.1.1" {
		t.Errorf("expected OSISID GEN.1.1, got %s", span.Ref.OSISID)
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
		{`"quoted"`, "&quot;quoted&quot;"},
		{"'quoted'", "&apos;quoted&apos;"},
		{"a & b < c > d", "a &amp; b &lt; c &gt; d"},
		{"normal text", "normal text"},
		{"", ""},
		{"<>&\"'", "&lt;&gt;&amp;&quot;&apos;"},
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

// executePlugin runs the plugin with a request and returns the response.
func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()

	pluginPath := "./format-usx"
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
