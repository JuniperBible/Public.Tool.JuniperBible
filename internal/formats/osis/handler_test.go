package osis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.osis" {
		t.Errorf("PluginID = %q, want format.osis", m.PluginID)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", m.Version)
	}
	if m.Kind != "format" {
		t.Errorf("Kind = %q, want format", m.Kind)
	}
}

func TestDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("osis xml detected by content", func(t *testing.T) {
		osisFile := filepath.Join(tmpDir, "test.txt")
		content := `<?xml version="1.0"?><osis><osisText osisIDWork="Test"></osisText></osis>`
		if err := os.WriteFile(osisFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Detect(osisFile)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if !result.Detected {
			t.Error("Expected OSIS to be detected")
		}
		if result.Format != "OSIS" {
			t.Errorf("Format = %q, want OSIS", result.Format)
		}
	})

	t.Run("osis extension with valid structure", func(t *testing.T) {
		osisFile := filepath.Join(tmpDir, "test.osis")
		content := `<?xml version="1.0"?><osis><osisText osisIDWork="Test"></osisText></osis>`
		if err := os.WriteFile(osisFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Detect(osisFile)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if !result.Detected {
			t.Error("Expected OSIS extension with valid structure to be detected")
		}
	})

	t.Run("xml extension with valid osis structure", func(t *testing.T) {
		xmlFile := filepath.Join(tmpDir, "test.xml")
		content := `<?xml version="1.0"?><osis><osisText osisIDWork="Test"></osisText></osis>`
		if err := os.WriteFile(xmlFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Detect(xmlFile)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if !result.Detected {
			t.Error("Expected XML with valid OSIS structure to be detected")
		}
	})

	t.Run("not osis file", func(t *testing.T) {
		txtFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(txtFile, []byte("plain text without markers"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Detect(txtFile)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if result.Detected {
			t.Error("Expected non-OSIS file to not be detected")
		}
	})

	t.Run("xml file with non-osis content", func(t *testing.T) {
		xmlFile := filepath.Join(tmpDir, "other.xml")
		content := `<?xml version="1.0"?><html><body>Not OSIS</body></html>`
		if err := os.WriteFile(xmlFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Detect(xmlFile)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if result.Detected {
			t.Error("XML file without OSIS structure should not be detected")
		}
	})

	t.Run("directory path", func(t *testing.T) {
		result, err := h.Detect(tmpDir)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if result.Detected {
			t.Error("Directory should not be detected as OSIS")
		}
		if result.Reason != "path is a directory, not a file" {
			t.Errorf("Reason = %q, expected directory message", result.Reason)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		result, err := h.Detect(filepath.Join(tmpDir, "nonexistent.osis"))
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if result.Detected {
			t.Error("Nonexistent file should not be detected")
		}
	})
}

func TestIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("basic ingest with work ID", func(t *testing.T) {
		osisFile := filepath.Join(tmpDir, "kjv.osis")
		content := `<?xml version="1.0"?><osis><osisText osisIDWork="KJV"></osisText></osis>`
		if err := os.WriteFile(osisFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		outputDir := filepath.Join(tmpDir, "output")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			t.Fatalf("failed to create output dir: %v", err)
		}

		result, err := h.Ingest(osisFile, outputDir)
		if err != nil {
			t.Fatalf("Ingest failed: %v", err)
		}

		if result.ArtifactID != "KJV" {
			t.Errorf("ArtifactID = %q, want KJV", result.ArtifactID)
		}
		if result.BlobSHA256 == "" {
			t.Error("BlobSHA256 should not be empty")
		}
		if result.Metadata["format"] != "OSIS" {
			t.Errorf("Metadata format = %q, want OSIS", result.Metadata["format"])
		}
	})

	t.Run("ingest without work ID", func(t *testing.T) {
		osisFile := filepath.Join(tmpDir, "noid.osis")
		content := `<?xml version="1.0"?><root>Not proper OSIS</root>`
		if err := os.WriteFile(osisFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		outputDir := filepath.Join(tmpDir, "output2")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			t.Fatalf("failed to create output dir: %v", err)
		}

		result, err := h.Ingest(osisFile, outputDir)
		if err != nil {
			t.Fatalf("Ingest failed: %v", err)
		}

		// Should fall back to filename
		if result.ArtifactID != "noid.osis" {
			t.Errorf("ArtifactID = %q, want noid.osis", result.ArtifactID)
		}
	})

	t.Run("ingest nonexistent file", func(t *testing.T) {
		outputDir := filepath.Join(tmpDir, "output3")
		_, err := h.Ingest(filepath.Join(tmpDir, "nonexistent.osis"), outputDir)
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("ingest to non-writable output", func(t *testing.T) {
		osisFile := filepath.Join(tmpDir, "test.osis")
		if err := os.WriteFile(osisFile, []byte("<osis/>"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		_, err := h.Ingest(osisFile, "/nonexistent/path/output")
		if err == nil {
			t.Error("Expected error for non-writable output")
		}
	})
}

func TestEnumerate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("enumerate file", func(t *testing.T) {
		osisFile := filepath.Join(tmpDir, "bible.osis")
		content := "test content"
		if err := os.WriteFile(osisFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Enumerate(osisFile)
		if err != nil {
			t.Fatalf("Enumerate failed: %v", err)
		}

		if len(result.Entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
		}
		if result.Entries[0].Path != "bible.osis" {
			t.Errorf("Path = %q, want bible.osis", result.Entries[0].Path)
		}
		if result.Entries[0].SizeBytes != int64(len(content)) {
			t.Errorf("SizeBytes = %d, want %d", result.Entries[0].SizeBytes, len(content))
		}
		if result.Entries[0].IsDir {
			t.Error("IsDir should be false")
		}
	})

	t.Run("enumerate nonexistent", func(t *testing.T) {
		_, err := h.Enumerate(filepath.Join(tmpDir, "nonexistent.osis"))
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})
}

func TestExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("extract ir success", func(t *testing.T) {
		osisFile := filepath.Join(tmpDir, "bible.osis")
		content := `<?xml version="1.0"?><osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="KJV">
    <header><work osisWork="KJV"><title>KJV</title></work></header>
    <div type="book" osisID="Gen"><p>In the beginning</p></div>
  </osisText>
</osis>`
		if err := os.WriteFile(osisFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		outputDir := filepath.Join(tmpDir, "ir_output")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			t.Fatalf("failed to create output dir: %v", err)
		}

		result, err := h.ExtractIR(osisFile, outputDir)
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
		_, err := h.ExtractIR(filepath.Join(tmpDir, "nonexistent.osis"), outputDir)
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("extract ir invalid xml", func(t *testing.T) {
		invalidFile := filepath.Join(tmpDir, "invalid.osis")
		if err := os.WriteFile(invalidFile, []byte("not valid xml"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		outputDir := filepath.Join(tmpDir, "ir_output3")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			t.Fatalf("failed to create output dir: %v", err)
		}

		_, err := h.ExtractIR(invalidFile, outputDir)
		if err == nil {
			t.Error("Expected error for invalid XML")
		}
	})

	t.Run("extract ir write error", func(t *testing.T) {
		osisFile := filepath.Join(tmpDir, "test2.osis")
		content := `<?xml version="1.0"?><osis><osisText osisIDWork="Test"></osisText></osis>`
		if err := os.WriteFile(osisFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		_, err := h.ExtractIR(osisFile, "/nonexistent/path/output")
		if err == nil {
			t.Error("Expected error for non-writable output")
		}
	})
}

func TestEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "osis-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("emit native success", func(t *testing.T) {
		// Create IR file
		corpus := &ir.Corpus{
			ID:       "KJV",
			Title:    "King James Version",
			Language: "en",
			Documents: []*ir.Document{
				{
					ID:    "Gen",
					Title: "Genesis",
					ContentBlocks: []*ir.ContentBlock{
						{
							ID:   "block1",
							Text: "In the beginning God created the heaven and the earth.",
						},
					},
				},
			},
		}
		irData, err := json.MarshalIndent(corpus, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal IR: %v", err)
		}

		irFile := filepath.Join(tmpDir, "kjv.ir.json")
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
		if result.Format != "OSIS" {
			t.Errorf("Format = %q, want OSIS", result.Format)
		}

		// Verify output file was written
		if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
			t.Error("Output file was not created")
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
