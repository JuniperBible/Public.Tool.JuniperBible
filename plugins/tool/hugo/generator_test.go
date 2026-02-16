package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestNewGenerator tests creating a new generator.
func TestNewGenerator(t *testing.T) {
	gen := NewGenerator("/tmp/output", "chapter")
	if gen == nil {
		t.Fatal("NewGenerator returned nil")
	}
	if gen.OutputDir != "/tmp/output" {
		t.Errorf("expected OutputDir '/tmp/output', got %q", gen.OutputDir)
	}
	if gen.Granularity != "chapter" {
		t.Errorf("expected Granularity 'chapter', got %q", gen.Granularity)
	}
}

// TestNewGeneratorDefaultGranularity tests default granularity.
func TestNewGeneratorDefaultGranularity(t *testing.T) {
	gen := NewGenerator("/tmp/output", "")
	if gen.Granularity != "chapter" {
		t.Errorf("expected default Granularity 'chapter', got %q", gen.Granularity)
	}
}

// TestIsPlaceholderText tests placeholder detection.
func TestIsPlaceholderText(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		{"", true},
		{"abc", true}, // too short
		{"Genesis 1:1:", true},
		{"II Chronicles 19:2:", true},
		{"1 John 3:16:", true},
		{"In the beginning God created", false},
		{"For God so loved the world", false},
		{"The Lord is my shepherd", false},
	}

	for _, tt := range tests {
		got := isPlaceholderText(tt.text)
		if got != tt.expected {
			t.Errorf("isPlaceholderText(%q) = %v, want %v", tt.text, got, tt.expected)
		}
	}
}

// TestGenerateFromInput tests generating Hugo JSON from input data.
func TestGenerateFromInput(t *testing.T) {
	tmpDir := t.TempDir()

	gen := NewGenerator(tmpDir, "chapter")

	bibles := []InputBible{
		{
			ID:          "TST",
			Title:       "Test Bible",
			Description: "A test Bible translation",
			Language:    "en",
			License:     "CC-PDDC",
			Books: []InputBook{
				{
					ID:        "Gen",
					Name:      "Genesis",
					Testament: "OT",
					Chapters: []InputChapter{
						{
							Number: 1,
							Verses: []InputVerse{
								{Number: 1, Text: "In the beginning God created the heaven and the earth."},
								{Number: 2, Text: "And the earth was without form, and void."},
								{Number: 3, Text: "And God said, Let there be light: and there was light."},
							},
						},
						{
							Number: 2,
							Verses: []InputVerse{
								{Number: 1, Text: "Thus the heavens and the earth were finished."},
							},
						},
					},
				},
			},
		},
	}

	result, err := gen.GenerateFromInput(bibles)
	if err != nil {
		t.Fatalf("GenerateFromInput failed: %v", err)
	}

	if result.BiblesGenerated != 1 {
		t.Errorf("expected 1 bible generated, got %d", result.BiblesGenerated)
	}
	if result.ChaptersWritten != 2 {
		t.Errorf("expected 2 chapters written, got %d", result.ChaptersWritten)
	}

	// Verify bibles.json was created
	metaPath := filepath.Join(tmpDir, "bibles.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("bibles.json was not created")
	}

	// Verify auxiliary file was created
	auxPath := filepath.Join(tmpDir, "bibles_auxiliary", "TST.json")
	if _, err := os.Stat(auxPath); os.IsNotExist(err) {
		t.Error("TST.json auxiliary file was not created")
	}

	// Verify bibles.json content
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("reading bibles.json: %v", err)
	}

	var metadata HugoBibleMetadata
	if err := json.Unmarshal(metaData, &metadata); err != nil {
		t.Fatalf("parsing bibles.json: %v", err)
	}

	if len(metadata.Bibles) != 1 {
		t.Errorf("expected 1 bible in metadata, got %d", len(metadata.Bibles))
	}
	if metadata.Bibles[0].ID != "TST" {
		t.Errorf("expected bible ID 'TST', got %q", metadata.Bibles[0].ID)
	}
	if metadata.Meta.Granularity != "chapter" {
		t.Errorf("expected granularity 'chapter', got %q", metadata.Meta.Granularity)
	}
}

// TestGenerateFromInputWithPlaceholders tests filtering placeholder verses.
func TestGenerateFromInputWithPlaceholders(t *testing.T) {
	tmpDir := t.TempDir()

	gen := NewGenerator(tmpDir, "chapter")

	bibles := []InputBible{
		{
			ID:       "TST",
			Title:    "Test Bible",
			Language: "en",
			Books: []InputBook{
				{
					ID:        "Gen",
					Name:      "Genesis",
					Testament: "OT",
					Chapters: []InputChapter{
						{
							Number: 1,
							Verses: []InputVerse{
								{Number: 1, Text: "Genesis 1:1:"}, // placeholder
								{Number: 2, Text: "Real verse content here."},
								{Number: 3, Text: "abc"}, // too short
							},
						},
					},
				},
			},
		},
	}

	result, err := gen.GenerateFromInput(bibles)
	if err != nil {
		t.Fatalf("GenerateFromInput failed: %v", err)
	}

	// Read auxiliary file to check verse count
	auxPath := filepath.Join(tmpDir, "bibles_auxiliary", "TST.json")
	auxData, err := os.ReadFile(auxPath)
	if err != nil {
		t.Fatalf("reading auxiliary file: %v", err)
	}

	var content HugoBibleContent
	if err := json.Unmarshal(auxData, &content); err != nil {
		t.Fatalf("parsing auxiliary file: %v", err)
	}

	// Should only have 1 verse (the real content one)
	if len(content.Books[0].Chapters[0].Verses) != 1 {
		t.Errorf("expected 1 verse after filtering, got %d", len(content.Books[0].Chapters[0].Verses))
	}

	_ = result
}

// TestGenerateFromInputWithEmptyBook tests handling books with no content.
func TestGenerateFromInputWithEmptyBook(t *testing.T) {
	tmpDir := t.TempDir()

	gen := NewGenerator(tmpDir, "chapter")

	bibles := []InputBible{
		{
			ID:       "TST",
			Title:    "Test Bible",
			Language: "en",
			Books: []InputBook{
				{
					ID:        "Gen",
					Name:      "Genesis",
					Testament: "OT",
					Chapters: []InputChapter{
						{
							Number: 1,
							Verses: []InputVerse{
								{Number: 1, Text: "Genesis 1:1:"}, // placeholder only
							},
						},
					},
				},
				{
					ID:        "Exod",
					Name:      "Exodus",
					Testament: "OT",
					Chapters: []InputChapter{
						{
							Number: 1,
							Verses: []InputVerse{
								{Number: 1, Text: "Real Exodus content."},
							},
						},
					},
				},
			},
		},
	}

	result, err := gen.GenerateFromInput(bibles)
	if err != nil {
		t.Fatalf("GenerateFromInput failed: %v", err)
	}

	// Read auxiliary file
	auxPath := filepath.Join(tmpDir, "bibles_auxiliary", "TST.json")
	auxData, err := os.ReadFile(auxPath)
	if err != nil {
		t.Fatalf("reading auxiliary file: %v", err)
	}

	var content HugoBibleContent
	if err := json.Unmarshal(auxData, &content); err != nil {
		t.Fatalf("parsing auxiliary file: %v", err)
	}

	// Should have 1 book (Exodus) and 1 excluded (Genesis)
	if len(content.Books) != 1 {
		t.Errorf("expected 1 book, got %d", len(content.Books))
	}
	if content.Books[0].ID != "Exod" {
		t.Errorf("expected book ID 'Exod', got %q", content.Books[0].ID)
	}
	if len(content.ExcludedBooks) != 1 {
		t.Errorf("expected 1 excluded book, got %d", len(content.ExcludedBooks))
	}
	if content.ExcludedBooks[0].ID != "Gen" {
		t.Errorf("expected excluded book ID 'Gen', got %q", content.ExcludedBooks[0].ID)
	}

	_ = result
}

// TestGenerateFromFile tests generating from a JSON file.
func TestGenerateFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create input JSON file
	inputPath := filepath.Join(tmpDir, "input.json")
	inputData := `{
		"id": "TST",
		"title": "Test Bible",
		"language": "en",
		"license": "CC-PDDC",
		"books": [
			{
				"id": "Gen",
				"name": "Genesis",
				"testament": "OT",
				"chapters": [
					{
						"number": 1,
						"verses": [
							{"number": 1, "text": "In the beginning."}
						]
					}
				]
			}
		]
	}`
	if err := os.WriteFile(inputPath, []byte(inputData), 0600); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	gen := NewGenerator(outputDir, "chapter")

	result, err := gen.GenerateFromFile(inputPath)
	if err != nil {
		t.Fatalf("GenerateFromFile failed: %v", err)
	}

	if result.BiblesGenerated != 1 {
		t.Errorf("expected 1 bible generated, got %d", result.BiblesGenerated)
	}
}

// TestGenerateFromFileArray tests generating from JSON array.
func TestGenerateFromFileArray(t *testing.T) {
	tmpDir := t.TempDir()

	// Create input JSON file with array
	inputPath := filepath.Join(tmpDir, "input.json")
	inputData := `[
		{
			"id": "TST1",
			"title": "Test Bible 1",
			"language": "en",
			"books": []
		},
		{
			"id": "TST2",
			"title": "Test Bible 2",
			"language": "en",
			"books": []
		}
	]`
	if err := os.WriteFile(inputPath, []byte(inputData), 0600); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	gen := NewGenerator(outputDir, "chapter")

	result, err := gen.GenerateFromFile(inputPath)
	if err != nil {
		t.Fatalf("GenerateFromFile failed: %v", err)
	}

	if result.BiblesGenerated != 2 {
		t.Errorf("expected 2 bibles generated, got %d", result.BiblesGenerated)
	}
}

// TestGenerateFromFileNotFound tests handling missing file.
func TestGenerateFromFileNotFound(t *testing.T) {
	gen := NewGenerator("/tmp/output", "chapter")

	_, err := gen.GenerateFromFile("/nonexistent/file.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// TestGenerateFromFileInvalidJSON tests handling invalid JSON.
func TestGenerateFromFileInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	inputPath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(inputPath, []byte("not valid json"), 0600); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(filepath.Join(tmpDir, "output"), "chapter")

	_, err := gen.GenerateFromFile(inputPath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestGenerate tests the main Generate function.
func TestGenerate(t *testing.T) {
	tmpDir := t.TempDir()

	// Create input file
	inputPath := filepath.Join(tmpDir, "input.json")
	inputData := `{"id": "TST", "title": "Test", "language": "en", "books": []}`
	if err := os.WriteFile(inputPath, []byte(inputData), 0600); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	result, err := Generate(&GenerateOptions{
		InputPath:  inputPath,
		OutputPath: outputDir,
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.BiblesGenerated != 1 {
		t.Errorf("expected 1 bible generated, got %d", result.BiblesGenerated)
	}
}

// TestGenerateOptionsNoOutput tests Generate with missing output path.
func TestGenerateOptionsNoOutput(t *testing.T) {
	_, err := Generate(&GenerateOptions{
		InputPath: "/some/input.json",
	})
	if err == nil {
		t.Error("expected error for missing output path")
	}
}
