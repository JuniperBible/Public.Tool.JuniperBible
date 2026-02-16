//go:build !sdk

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		filename string
		detected bool
	}{
		{
			name:     "NA28 extension",
			content:  "any content",
			filename: "test.na28",
			detected: true,
		},
		{
			name:     "apparatus extension",
			filename: "test.apparatus",
			content:  "any content",
			detected: true,
		},
		{
			name:     "XML with apparatus markers",
			filename: "test.xml",
			content:  `<?xml version="1.0"?><app><rdg wit="א B">text</rdg></app>`,
			detected: true,
		},
		{
			name:     "Plain text file",
			filename: "test.txt",
			content:  "Just plain text",
			detected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, tt.filename)

			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			args := map[string]interface{}{
				"path": testFile,
			}

			handleDetect(args)
		})
	}
}

func TestIngestAndEnumerate(t *testing.T) {
	tmpDir := t.TempDir()
	casDir := filepath.Join(tmpDir, "cas")

	// Create test apparatus content
	content := `<?xml version="1.0" encoding="UTF-8"?>
<apparatus edition="NA28">
  <entry ref="Matt.1.1">
    <lem>βίβλος</lem>
    <rdg wit="א B">βιβλος</rdg>
  </entry>
</apparatus>`

	// Test ingest
	args := map[string]interface{}{
		"path":    filepath.Join(tmpDir, "test.na28"),
		"cas_dir": casDir,
	}

	if err := os.WriteFile(args["path"].(string), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	handleIngest(args)
}

func TestExtractIR(t *testing.T) {
	tmpDir := t.TempDir()
	casDir := filepath.Join(tmpDir, "cas")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	// Create test apparatus content
	content := `Matt.1.1 txt א B C: βίβλος
Matt.1.2 txt D E F: γενέσεως`

	// Create blob
	blobHash := "abc123"
	blobPath := filepath.Join(casDir, "blobs", "ab", "c1", blobHash)
	if err := os.MkdirAll(filepath.Dir(blobPath), 0755); err != nil {
		t.Fatalf("failed to create blob dir: %v", err)
	}
	if err := os.WriteFile(blobPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write blob: %v", err)
	}

	args := map[string]interface{}{
		"blob_hash":  blobHash,
		"cas_dir":    casDir,
		"output_dir": outputDir,
	}

	handleExtractIR(args)

	// Verify IR was created
	irPath := filepath.Join(outputDir, "corpus.json")
	data, err := os.ReadFile(irPath)
	if err != nil {
		t.Fatalf("failed to read IR: %v", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("failed to unmarshal IR: %v", err)
	}

	if corpus.SourceFormat != "NA28App" {
		t.Errorf("expected SourceFormat NA28App, got %s", corpus.SourceFormat)
	}

	if len(corpus.Documents) == 0 {
		t.Errorf("expected documents, got none")
	}
}

func TestEmitNative(t *testing.T) {
	tmpDir := t.TempDir()
	irDir := filepath.Join(tmpDir, "ir")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(irDir, 0755); err != nil {
		t.Fatalf("failed to create ir dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	// Create test IR
	corpus := &ipc.Corpus{
		ID:           "na28app",
		Version:      "1.0",
		ModuleType:   "apparatus",
		SourceFormat: "NA28App",
		Documents: []*ipc.Document{
			{
				ID:    "apparatus",
				Title: "Test Apparatus",
				Order: 1,
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "block-0",
						Sequence: 0,
						Text:     "Test apparatus entry",
						Attributes: map[string]interface{}{
							"reference": "Matt.1.1",
							"witnesses": []interface{}{"א", "B", "C"},
						},
					},
				},
			},
		},
	}

	irPath := filepath.Join(irDir, "corpus.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal IR: %v", err)
	}
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("failed to write IR: %v", err)
	}

	args := map[string]interface{}{
		"ir_path":    irPath,
		"output_dir": outputDir,
	}

	handleEmitNative(args)

	// Verify output was created
	outputPath := filepath.Join(outputDir, "apparatus.na28.xml")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	content := string(data)
	if !containsNA28Markers(content) {
		t.Errorf("output does not contain NA28 markers")
	}
}

func TestParseVariants(t *testing.T) {
	block := &ipc.ContentBlock{
		Attributes: make(map[string]interface{}),
	}

	line := "Matt.1.1 txt א B C: βίβλος"
	parseVariants(block, line)

	if ref, ok := block.Attributes["reference"].(string); !ok || ref != "Matt.1.1" {
		t.Errorf("expected reference Matt.1.1, got %v", block.Attributes["reference"])
	}

	if book, ok := block.Attributes["book"].(string); !ok || book != "Matt" {
		t.Errorf("expected book Matt, got %v", block.Attributes["book"])
	}
}

func TestContainsNA28Markers(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "XML apparatus",
			content:  `<app><rdg wit="א">text</rdg></app>`,
			expected: true,
		},
		{
			name:     "Witness Aleph",
			content:  "text א variant",
			expected: true,
		},
		{
			name:     "No markers",
			content:  "plain text",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsNA28Markers(tt.content)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
