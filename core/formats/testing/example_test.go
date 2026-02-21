package testing_test

import (
	"os"
	"path/filepath"
	"testing"

	ftesting "github.com/JuniperBible/juniper/core/formats/testing"
	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
	"github.com/JuniperBible/juniper/plugins/sdk/ir"
)

// This example shows how to use the format testing framework.
// In practice, this would be in a format-specific package like core/formats/json/format_test.go

// Example: Simple TXT format plugin configuration
var exampleTXTConfig = &format.Config{
	Name:       "TXT",
	Extensions: []string{".txt"},
	Parse:      parseExampleTXT,
	Emit:       emitExampleTXT,
}

// parseExampleTXT is a simplified parser for demonstration
func parseExampleTXT(path string) (*ir.Corpus, error) {
	// In a real implementation, this would parse the TXT file
	// For this example, we return a minimal corpus
	corpus := &ir.Corpus{
		ID:         "example",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Example Bible",
		LossClass:  "L3", // TXT format has significant loss
		Documents: []*ir.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "In the beginning.",
					},
				},
			},
		},
	}
	return corpus, nil
}

// emitExampleTXT is a simplified emitter for demonstration
func emitExampleTXT(corpus *ir.Corpus, outputDir string) (string, error) {
	// In a real implementation, this would convert IR to TXT format
	// For this example, we create a simple output file
	outputPath := filepath.Join(outputDir, "example.txt")
	content := "Gen 1:1 In the beginning.\nGen 1:2 And the earth was void.\n"
	if err := os.WriteFile(outputPath, []byte(content), 0600); err != nil {
		return "", err
	}
	return outputPath, nil
}

// TestExampleFormat demonstrates how to use the testing framework
func TestExampleFormat(t *testing.T) {
	ftesting.RunFormatTests(t, ftesting.FormatTestCase{
		Config: exampleTXTConfig,
		SampleContent: `Gen 1:1 In the beginning.
Gen 1:2 And the earth was void.
`,
		ExpectedIR: &ftesting.IRExpectations{
			ID:               "example",
			Title:            "Example Bible",
			MinDocuments:     1,
			MinContentBlocks: 1,
		},
		ExpectedLossClass: "L3",
		RoundTrip:         false, // TXT typically has loss
		// Note: NegativeDetection is not used here because TXT uses extension-based
		// detection which can't distinguish content types within .txt files.
	})
}

// TestExampleFormatWithCustomValidation shows custom IR validation
func TestExampleFormatWithCustomValidation(t *testing.T) {
	ftesting.RunFormatTests(t, ftesting.FormatTestCase{
		Config: exampleTXTConfig,
		SampleContent: `Gen 1:1 In the beginning.
Gen 1:2 And the earth was void.
`,
		ExpectedIR: &ftesting.IRExpectations{
			MinDocuments:     1,
			MinContentBlocks: 1,
			CustomValidation: func(t *testing.T, corpus *ipc.Corpus) {
				if corpus.ModuleType != "BIBLE" {
					t.Errorf("expected module_type BIBLE, got %s", corpus.ModuleType)
				}
				if len(corpus.Documents) > 0 {
					doc := corpus.Documents[0]
					if doc.ID != "Gen" {
						t.Errorf("expected first document ID Gen, got %s", doc.ID)
					}
				}
			},
		},
		ExpectedLossClass: "L3",
	})
}

// TestExampleFormatWithSkippedTests shows how to skip specific tests
func TestExampleFormatWithSkippedTests(t *testing.T) {
	ftesting.RunFormatTests(t, ftesting.FormatTestCase{
		Config:        exampleTXTConfig,
		SampleContent: `Gen 1:1 In the beginning.`,
		ExpectedIR: &ftesting.IRExpectations{
			MinDocuments: 1,
		},
		SkipTests: []string{"RoundTrip", "EmitNative"}, // Skip these subtests
	})
}

// Example: JSON format with round-trip testing
var exampleJSONConfig = &format.Config{
	Name:       "JSON",
	Extensions: []string{".json"},
	Parse:      parseExampleJSON,
	Emit:       emitExampleJSON,
}

func parseExampleJSON(path string) (*ir.Corpus, error) {
	// Simplified JSON parser
	return &ir.Corpus{
		ID:         "test",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Test Bible",
		LossClass:  "L0", // JSON is lossless
	}, nil
}

func emitExampleJSON(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, "test.json")
	content := `{
  "meta": {"id": "test", "title": "Test Bible"},
  "books": []
}`
	if err := os.WriteFile(outputPath, []byte(content), 0600); err != nil {
		return "", err
	}
	return outputPath, nil
}

// TestExampleJSONWithRoundTrip demonstrates L0 round-trip testing
func TestExampleJSONWithRoundTrip(t *testing.T) {
	ftesting.RunFormatTests(t, ftesting.FormatTestCase{
		Config: exampleJSONConfig,
		SampleContent: `{
  "meta": {"id": "test", "title": "Test Bible"},
  "books": []
}`,
		ExpectedIR: &ftesting.IRExpectations{
			ID:    "test",
			Title: "Test Bible",
		},
		ExpectedLossClass: "L0",
		RoundTrip:         true, // Enable L0 round-trip testing
	})
}

// TestExampleDetectOnly shows testing just detection
func TestExampleDetectOnly(t *testing.T) {
	ftesting.RunFormatTests(t, ftesting.FormatTestCase{
		Config: &format.Config{
			Name:       "XML",
			Extensions: []string{".xml"},
		},
		SampleContent:     `<?xml version="1.0"?><bible></bible>`,
		NegativeDetection: "plain text",
		SkipTests:         []string{"ExtractIR", "EmitNative", "RoundTrip"},
	})
}
