package usx

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.usx" {
		t.Errorf("PluginID = %q, want format.usx", m.PluginID)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", m.Version)
	}
	if m.Kind != "format" {
		t.Errorf("Kind = %q, want format", m.Kind)
	}
}

func TestDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("valid usx detected", func(t *testing.T) {
		usxFile := filepath.Join(tmpDir, "test.xml")
		content := `<?xml version="1.0"?><usx version="3.0"><book code="GEN"/></usx>`
		if err := os.WriteFile(usxFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Detect(usxFile)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if !result.Detected {
			t.Error("Expected USX to be detected")
		}
		if result.Format != "USX" {
			t.Errorf("Format = %q, want USX", result.Format)
		}
	})

	t.Run("not usx file", func(t *testing.T) {
		txtFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(txtFile, []byte("plain text without markers"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Detect(txtFile)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if result.Detected {
			t.Error("Expected non-USX file to not be detected")
		}
	})

	t.Run("invalid xml with usx tag", func(t *testing.T) {
		badFile := filepath.Join(tmpDir, "bad.xml")
		if err := os.WriteFile(badFile, []byte("<usx>not closed properly"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Detect(badFile)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if result.Detected {
			t.Error("Invalid XML should not be detected as USX")
		}
	})

	t.Run("directory path", func(t *testing.T) {
		result, err := h.Detect(tmpDir)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if result.Detected {
			t.Error("Directory should not be detected as USX")
		}
		if result.Reason != "path is a directory, not a file" {
			t.Errorf("Reason = %q, expected directory message", result.Reason)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		result, err := h.Detect(filepath.Join(tmpDir, "nonexistent.usx"))
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if result.Detected {
			t.Error("Nonexistent file should not be detected")
		}
	})
}

func TestIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("basic ingest with book code", func(t *testing.T) {
		usxFile := filepath.Join(tmpDir, "genesis.usx")
		content := `<?xml version="1.0"?><usx version="3.0"><book code="GEN" style="id">Genesis</book></usx>`
		if err := os.WriteFile(usxFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		outputDir := filepath.Join(tmpDir, "output")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			t.Fatalf("failed to create output dir: %v", err)
		}

		result, err := h.Ingest(usxFile, outputDir)
		if err != nil {
			t.Fatalf("Ingest failed: %v", err)
		}

		if result.ArtifactID != "GEN" {
			t.Errorf("ArtifactID = %q, want GEN", result.ArtifactID)
		}
		if result.BlobSHA256 == "" {
			t.Error("BlobSHA256 should not be empty")
		}
		if result.Metadata["format"] != "USX" {
			t.Errorf("Metadata format = %q, want USX", result.Metadata["format"])
		}
	})

	t.Run("ingest without book code", func(t *testing.T) {
		usxFile := filepath.Join(tmpDir, "nocode.usx")
		content := `<?xml version="1.0"?><usx version="3.0"></usx>`
		if err := os.WriteFile(usxFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		outputDir := filepath.Join(tmpDir, "output2")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			t.Fatalf("failed to create output dir: %v", err)
		}

		result, err := h.Ingest(usxFile, outputDir)
		if err != nil {
			t.Fatalf("Ingest failed: %v", err)
		}

		// Should fall back to filename without extension
		if result.ArtifactID != "nocode" {
			t.Errorf("ArtifactID = %q, want nocode", result.ArtifactID)
		}
	})

	t.Run("ingest nonexistent file", func(t *testing.T) {
		outputDir := filepath.Join(tmpDir, "output3")
		_, err := h.Ingest(filepath.Join(tmpDir, "nonexistent.usx"), outputDir)
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("ingest to non-writable output", func(t *testing.T) {
		usxFile := filepath.Join(tmpDir, "test.usx")
		if err := os.WriteFile(usxFile, []byte("<usx/>"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		_, err := h.Ingest(usxFile, "/nonexistent/path/output")
		if err == nil {
			t.Error("Expected error for non-writable output")
		}
	})
}

func TestEnumerate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("enumerate file", func(t *testing.T) {
		usxFile := filepath.Join(tmpDir, "bible.usx")
		content := "test content"
		if err := os.WriteFile(usxFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Enumerate(usxFile)
		if err != nil {
			t.Fatalf("Enumerate failed: %v", err)
		}

		if len(result.Entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
		}
		if result.Entries[0].Path != "bible.usx" {
			t.Errorf("Path = %q, want bible.usx", result.Entries[0].Path)
		}
		if result.Entries[0].SizeBytes != int64(len(content)) {
			t.Errorf("SizeBytes = %d, want %d", result.Entries[0].SizeBytes, len(content))
		}
		if result.Entries[0].IsDir {
			t.Error("IsDir should be false")
		}
	})

	t.Run("enumerate nonexistent", func(t *testing.T) {
		_, err := h.Enumerate(filepath.Join(tmpDir, "nonexistent.usx"))
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})
}

func TestExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("extract ir success", func(t *testing.T) {
		usxFile := filepath.Join(tmpDir, "bible.usx")
		content := `<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
  <chapter number="1"/>
  <verse number="1"/>In the beginning God created the heaven and the earth.
</usx>`
		if err := os.WriteFile(usxFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		outputDir := filepath.Join(tmpDir, "ir_output")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			t.Fatalf("failed to create output dir: %v", err)
		}

		result, err := h.ExtractIR(usxFile, outputDir)
		if err != nil {
			t.Fatalf("ExtractIR failed: %v", err)
		}

		if result.IRPath == "" {
			t.Error("IRPath should not be empty")
		}

		// Verify IR file was written
		if _, err := os.Stat(result.IRPath); os.IsNotExist(err) {
			t.Error("IR file was not created")
		}
	})

	t.Run("extract ir nonexistent file", func(t *testing.T) {
		outputDir := filepath.Join(tmpDir, "ir_output2")
		_, err := h.ExtractIR(filepath.Join(tmpDir, "nonexistent.usx"), outputDir)
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("extract ir write error", func(t *testing.T) {
		usxFile := filepath.Join(tmpDir, "test2.usx")
		content := `<?xml version="1.0"?><usx version="3.0"><book code="GEN"/></usx>`
		if err := os.WriteFile(usxFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		_, err := h.ExtractIR(usxFile, "/nonexistent/path/output")
		if err == nil {
			t.Error("Expected error for non-writable output")
		}
	})
}

func TestEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("emit native success", func(t *testing.T) {
		corpus := &ir.Corpus{
			ID:    "GEN",
			Title: "Genesis",
			Documents: []*ir.Document{
				{
					ID:    "GEN",
					Title: "Genesis",
					ContentBlocks: []*ir.ContentBlock{
						{
							ID:   "block1",
							Text: "In the beginning God created the heaven and the earth.",
							Anchors: []*ir.Anchor{
								{ID: "anchor1", CharOffset: 0},
							},
						},
					},
				},
			},
		}
		irData, err := json.MarshalIndent(corpus, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal IR: %v", err)
		}

		irFile := filepath.Join(tmpDir, "gen.ir.json")
		if err := os.WriteFile(irFile, irData, 0644); err != nil {
			t.Fatalf("failed to write IR file: %v", err)
		}

		outputDir := filepath.Join(tmpDir, "native_output")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			t.Fatalf("failed to create output dir: %v", err)
		}

		result, err := h.EmitNative(irFile, outputDir)
		if err != nil {
			t.Fatalf("EmitNative failed: %v", err)
		}

		if result.OutputPath == "" {
			t.Error("OutputPath should not be empty")
		}
		if result.Format != "USX" {
			t.Errorf("Format = %q, want USX", result.Format)
		}

		// Verify output file was written
		if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
			t.Error("Output file was not created")
		}

		// Verify content
		data, _ := os.ReadFile(result.OutputPath)
		if !strings.Contains(string(data), "<usx") {
			t.Error("Output should contain USX element")
		}
	})

	t.Run("emit native nonexistent ir file", func(t *testing.T) {
		outputDir := filepath.Join(tmpDir, "native_output2")
		_, err := h.EmitNative(filepath.Join(tmpDir, "nonexistent.ir.json"), outputDir)
		if err == nil {
			t.Error("Expected error for nonexistent IR file")
		}
	})

	t.Run("emit native invalid json", func(t *testing.T) {
		invalidFile := filepath.Join(tmpDir, "invalid.ir.json")
		if err := os.WriteFile(invalidFile, []byte("not valid json"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		outputDir := filepath.Join(tmpDir, "native_output3")
		_, err := h.EmitNative(invalidFile, outputDir)
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})

	t.Run("emit native write error", func(t *testing.T) {
		corpus := &ir.Corpus{ID: "Test"}
		irData, _ := json.Marshal(corpus)

		irFile := filepath.Join(tmpDir, "test.ir.json")
		if err := os.WriteFile(irFile, irData, 0644); err != nil {
			t.Fatalf("failed to write IR file: %v", err)
		}

		_, err := h.EmitNative(irFile, "/nonexistent/path/output")
		if err == nil {
			t.Error("Expected error for non-writable output")
		}
	})
}

func TestRegister(t *testing.T) {
	// Register is called in init(), so just verify it doesn't panic
	// when called again
	Register()
}

func TestParseUSXToIR_Simple(t *testing.T) {
	usxContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="GEN" style="id">Genesis</book>
  <chapter number="1"/>
  <verse number="1"/>In the beginning God created the heaven and the earth.
  <verse number="2"/>And the earth was without form, and void.
</usx>`)

	corpus, err := parseUSXToIR(usxContent)
	if err != nil {
		t.Fatalf("parseUSXToIR failed: %v", err)
	}

	if corpus.ID != "GEN" {
		t.Errorf("ID = %q, want GEN", corpus.ID)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}

	doc := corpus.Documents[0]
	if doc.ID != "GEN" {
		t.Errorf("Document ID = %q, want GEN", doc.ID)
	}
	if len(doc.ContentBlocks) < 2 {
		t.Errorf("Expected at least 2 content blocks, got %d", len(doc.ContentBlocks))
	}
}

func TestParseUSXToIR_MultipleChapters(t *testing.T) {
	usxContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="MAT" style="id">Matthew</book>
  <chapter number="1"/>
  <verse number="1"/>Verse one
  <chapter number="2"/>
  <verse number="1"/>Chapter two verse one
</usx>`)

	corpus, err := parseUSXToIR(usxContent)
	if err != nil {
		t.Fatalf("parseUSXToIR failed: %v", err)
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}

	// Should have 2 content blocks (one per verse)
	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("Expected 2 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

func TestParseUSXToIR_Invalid(t *testing.T) {
	// Note: The XML decoder returns EOF for invalid XML rather than an error
	// in some cases, so we test that no documents are created
	usxContent := []byte(`not valid xml at all`)
	corpus, err := parseUSXToIR(usxContent)
	// Either an error or no documents is acceptable
	if err == nil && len(corpus.Documents) > 0 {
		t.Error("Expected error or empty documents for invalid XML")
	}
}

func TestParseUSXToIR_Empty(t *testing.T) {
	usxContent := []byte(`<?xml version="1.0"?><usx version="3.0"></usx>`)
	corpus, err := parseUSXToIR(usxContent)
	if err != nil {
		t.Fatalf("parseUSXToIR failed: %v", err)
	}

	if len(corpus.Documents) != 0 {
		t.Errorf("Expected 0 documents for empty USX, got %d", len(corpus.Documents))
	}
}

func TestEmitUSXFromIR(t *testing.T) {
	corpus := &ir.Corpus{
		ID:    "GEN",
		Title: "Genesis",
		Documents: []*ir.Document{
			{
				ID:    "GEN",
				Title: "Genesis",
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:   "cb-1",
						Text: "In the beginning",
						Anchors: []*ir.Anchor{
							{ID: "a-1-0", CharOffset: 0},
						},
					},
				},
			},
		},
	}

	output := emitUSXFromIR(corpus)

	if !strings.Contains(output, `<usx version="3.0">`) {
		t.Error("Output should contain USX version")
	}
	if !strings.Contains(output, `<book code="GEN"`) {
		t.Error("Output should contain book code")
	}
	if !strings.Contains(output, "In the beginning") {
		t.Error("Output should contain content")
	}
}

func TestEmitUSXFromIR_Empty(t *testing.T) {
	corpus := &ir.Corpus{
		ID: "Empty",
	}

	output := emitUSXFromIR(corpus)

	if !strings.Contains(output, `<usx version="3.0">`) {
		t.Error("Output should contain USX element")
	}
	if !strings.Contains(output, "</usx>") {
		t.Error("Output should close USX element")
	}
}

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"plain text", "plain text"},
		{"<tag>", "&lt;tag&gt;"},
		{"&ampersand", "&amp;ampersand"},
		{`"quotes"`, "&quot;quotes&quot;"},
		{"'apostrophe'", "&apos;apostrophe&apos;"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := escapeXML(tt.input); got != tt.want {
				t.Errorf("escapeXML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCreateContentBlock(t *testing.T) {
	cb := createContentBlock(1, "  test text  ", "GEN", 1, 1)

	if cb.ID != "cb-1" {
		t.Errorf("ID = %q, want cb-1", cb.ID)
	}
	if cb.Sequence != 1 {
		t.Errorf("Sequence = %d, want 1", cb.Sequence)
	}
	if cb.Text != "test text" {
		t.Errorf("Text = %q, want 'test text' (trimmed)", cb.Text)
	}
	if len(cb.Anchors) != 1 {
		t.Errorf("Expected 1 anchor, got %d", len(cb.Anchors))
	}
	if cb.Hash == "" {
		t.Error("Hash should not be empty")
	}
}
