package usfm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.usfm" {
		t.Errorf("PluginID = %q, want format.usfm", m.PluginID)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", m.Version)
	}
	if m.Kind != "format" {
		t.Errorf("Kind = %q, want format", m.Kind)
	}
}

func TestDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("usfm markers detected", func(t *testing.T) {
		usfmFile := filepath.Join(tmpDir, "test.txt")
		content := `\id GEN
\c 1
\v 1 In the beginning`
		if err := os.WriteFile(usfmFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Detect(usfmFile)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if !result.Detected {
			t.Error("Expected USFM to be detected")
		}
		if result.Format != "USFM" {
			t.Errorf("Format = %q, want USFM", result.Format)
		}
	})

	t.Run("usfm extension detected", func(t *testing.T) {
		usfmFile := filepath.Join(tmpDir, "test.usfm")
		if err := os.WriteFile(usfmFile, []byte("plain text"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Detect(usfmFile)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if !result.Detected {
			t.Error("Expected USFM extension to be detected")
		}
	})

	t.Run("sfm extension detected", func(t *testing.T) {
		sfmFile := filepath.Join(tmpDir, "test.sfm")
		if err := os.WriteFile(sfmFile, []byte("plain text"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Detect(sfmFile)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if !result.Detected {
			t.Error("Expected SFM extension to be detected")
		}
	})

	t.Run("ptx extension detected", func(t *testing.T) {
		ptxFile := filepath.Join(tmpDir, "test.ptx")
		if err := os.WriteFile(ptxFile, []byte("plain text"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Detect(ptxFile)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if !result.Detected {
			t.Error("Expected PTX extension to be detected")
		}
	})

	t.Run("not usfm file", func(t *testing.T) {
		txtFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(txtFile, []byte("plain text without markers"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Detect(txtFile)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if result.Detected {
			t.Error("Expected non-USFM file to not be detected")
		}
	})

	t.Run("directory path", func(t *testing.T) {
		result, err := h.Detect(tmpDir)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if result.Detected {
			t.Error("Directory should not be detected as USFM")
		}
		if result.Reason != "path is a directory, not a file" {
			t.Errorf("Reason = %q, expected directory message", result.Reason)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		result, err := h.Detect(filepath.Join(tmpDir, "nonexistent.usfm"))
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if result.Detected {
			t.Error("Nonexistent file should not be detected")
		}
	})
}

func TestIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("basic ingest", func(t *testing.T) {
		usfmFile := filepath.Join(tmpDir, "genesis.usfm")
		content := `\id GEN English
\c 1
\v 1 In the beginning`
		if err := os.WriteFile(usfmFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		outputDir := filepath.Join(tmpDir, "output")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			t.Fatalf("failed to create output dir: %v", err)
		}

		result, err := h.Ingest(usfmFile, outputDir)
		if err != nil {
			t.Fatalf("Ingest failed: %v", err)
		}

		if result.ArtifactID != "GEN" {
			t.Errorf("ArtifactID = %q, want GEN", result.ArtifactID)
		}
		if result.BlobSHA256 == "" {
			t.Error("BlobSHA256 should not be empty")
		}
		if result.SizeBytes != int64(len(content)) {
			t.Errorf("SizeBytes = %d, want %d", result.SizeBytes, len(content))
		}
		if result.Metadata["format"] != "USFM" {
			t.Errorf("Metadata format = %q, want USFM", result.Metadata["format"])
		}
	})

	t.Run("ingest without id marker", func(t *testing.T) {
		usfmFile := filepath.Join(tmpDir, "noid.usfm")
		content := `\c 1
\v 1 Text without id marker`
		if err := os.WriteFile(usfmFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		outputDir := filepath.Join(tmpDir, "output2")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			t.Fatalf("failed to create output dir: %v", err)
		}

		result, err := h.Ingest(usfmFile, outputDir)
		if err != nil {
			t.Fatalf("Ingest failed: %v", err)
		}

		// Should fall back to filename
		if result.ArtifactID != "noid.usfm" {
			t.Errorf("ArtifactID = %q, want noid.usfm", result.ArtifactID)
		}
	})

	t.Run("ingest nonexistent file", func(t *testing.T) {
		outputDir := filepath.Join(tmpDir, "output3")
		_, err := h.Ingest(filepath.Join(tmpDir, "nonexistent.usfm"), outputDir)
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})
}

func TestEnumerate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("enumerate file", func(t *testing.T) {
		usfmFile := filepath.Join(tmpDir, "genesis.usfm")
		content := "test content"
		if err := os.WriteFile(usfmFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result, err := h.Enumerate(usfmFile)
		if err != nil {
			t.Fatalf("Enumerate failed: %v", err)
		}

		if len(result.Entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
		}
		if result.Entries[0].Path != "genesis.usfm" {
			t.Errorf("Path = %q, want genesis.usfm", result.Entries[0].Path)
		}
		if result.Entries[0].SizeBytes != int64(len(content)) {
			t.Errorf("SizeBytes = %d, want %d", result.Entries[0].SizeBytes, len(content))
		}
		if result.Entries[0].IsDir {
			t.Error("IsDir should be false")
		}
	})

	t.Run("enumerate nonexistent", func(t *testing.T) {
		_, err := h.Enumerate(filepath.Join(tmpDir, "nonexistent.usfm"))
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})
}

func TestExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("extract ir success", func(t *testing.T) {
		usfmFile := filepath.Join(tmpDir, "genesis.usfm")
		content := `\id GEN
\h Genesis
\mt Genesis
\c 1
\v 1 In the beginning God created the heaven and the earth.`
		if err := os.WriteFile(usfmFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		outputDir := filepath.Join(tmpDir, "ir_output")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			t.Fatalf("failed to create output dir: %v", err)
		}

		result, err := h.ExtractIR(usfmFile, outputDir)
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
		_, err := h.ExtractIR(filepath.Join(tmpDir, "nonexistent.usfm"), outputDir)
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})
}

func TestEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	t.Run("emit native success", func(t *testing.T) {
		// Create IR file
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

		irFile := filepath.Join(tmpDir, "genesis.ir.json")
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
		if result.Format != "USFM" {
			t.Errorf("Format = %q, want USFM", result.Format)
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
}

func TestRegister(t *testing.T) {
	// Register is called in init(), so just verify it doesn't panic
	// when called again
	Register()
}

func TestIngestBlobDirError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	// Create USFM file
	usfmFile := filepath.Join(tmpDir, "test.usfm")
	if err := os.WriteFile(usfmFile, []byte("\\id GEN\n\\v 1 test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Use non-writable directory (will fail on blob dir creation)
	_, err = h.Ingest(usfmFile, "/nonexistent/path/output")
	if err == nil {
		t.Error("Expected error for non-writable output directory")
	}
}

func TestExtractIRWriteError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	// Create USFM file
	usfmFile := filepath.Join(tmpDir, "test.usfm")
	if err := os.WriteFile(usfmFile, []byte("\\id GEN\n\\v 1 test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Use non-writable output directory
	_, err = h.ExtractIR(usfmFile, "/nonexistent/path/output")
	if err == nil {
		t.Error("Expected error for non-writable output directory")
	}
}

func TestEmitNativeWriteError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "usfm-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	h := &Handler{}

	// Create valid IR file
	corpus := &ir.Corpus{
		ID:    "GEN",
		Title: "Genesis",
	}
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal IR: %v", err)
	}

	irFile := filepath.Join(tmpDir, "test.ir.json")
	if err := os.WriteFile(irFile, irData, 0644); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	// Use non-writable output directory
	_, err = h.EmitNative(irFile, "/nonexistent/path/output")
	if err == nil {
		t.Error("Expected error for non-writable output directory")
	}
}
