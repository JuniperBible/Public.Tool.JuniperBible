package json

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

// TestDetect_ValidJsonBible tests detection of a valid JSON Bible file.
func TestDetect_ValidJsonBible(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")

	validJSON := JSONBible{
		Meta: JSONMeta{
			ID:       "TEST",
			Title:    "Test Bible",
			Language: "en",
			Version:  "1.0.0",
		},
		Books: []JSONBook{
			{
				ID:    "Gen",
				Name:  "Genesis",
				Order: 1,
				Chapters: []JSONChapter{
					{
						Number: 1,
						Verses: []JSONVerse{
							{
								Book:    "Gen",
								Chapter: 1,
								Verse:   1,
								Text:    "In the beginning...",
								ID:      "Gen.1.1",
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(validJSON)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed, got: %s", result.Reason)
	}
	if result.Format != "JSON" {
		t.Errorf("Expected format JSON, got %s", result.Format)
	}
	if !strings.Contains(result.Reason, "Capsule JSON Bible format") {
		t.Errorf("Expected reason to mention Capsule JSON Bible format, got: %s", result.Reason)
	}
}

// TestDetect_InvalidJson tests detection with invalid JSON content.
func TestDetect_InvalidJson(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")

	// Write invalid JSON
	if err := os.WriteFile(testFile, []byte("{invalid json}"), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for invalid JSON")
	}
	if !strings.Contains(result.Reason, "not valid JSON") {
		t.Errorf("Expected reason to mention invalid JSON, got: %s", result.Reason)
	}
}

// TestDetect_WrongExtension tests detection with wrong file extension.
func TestDetect_WrongExtension(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xml")

	if err := os.WriteFile(testFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for non-.json extension")
	}
	if !strings.Contains(result.Reason, "not a .json file") {
		t.Errorf("Expected reason to mention wrong extension, got: %s", result.Reason)
	}
}

// TestDetect_MissingFields tests detection when required fields are missing.
func TestDetect_MissingFields(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")

	// Valid JSON but not a Bible format (missing required fields)
	emptyJSON := JSONBible{}
	data, err := json.Marshal(emptyJSON)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for missing fields")
	}
	if !strings.Contains(result.Reason, "not a Capsule JSON Bible format") {
		t.Errorf("Expected reason to mention missing format, got: %s", result.Reason)
	}
}

// TestDetect_Directory tests that detection fails for directories.
func TestDetect_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	h := &Handler{}
	result, err := h.Detect(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for directory")
	}
	if !strings.Contains(result.Reason, "directory") {
		t.Errorf("Expected reason to mention directory, got: %s", result.Reason)
	}
}

// TestDetect_NonexistentFile tests detection with a nonexistent file.
func TestDetect_NonexistentFile(t *testing.T) {
	h := &Handler{}
	result, err := h.Detect("/nonexistent/path/file.json")
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for nonexistent file")
	}
	if !strings.Contains(result.Reason, "cannot stat") {
		t.Errorf("Expected reason to mention stat error, got: %s", result.Reason)
	}
}

// TestIngest_Success tests successful ingestion of a JSON Bible file.
func TestIngest_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-bible.json")
	outputDir := filepath.Join(tmpDir, "output")

	validJSON := JSONBible{
		Meta: JSONMeta{
			ID:    "TEST",
			Title: "Test Bible",
		},
		Books: []JSONBook{
			{
				ID:   "Gen",
				Name: "Genesis",
			},
		},
	}

	data, err := json.Marshal(validJSON)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if result.ArtifactID != "test-bible" {
		t.Errorf("Expected artifact ID 'test-bible', got %s", result.ArtifactID)
	}
	if result.SizeBytes != int64(len(data)) {
		t.Errorf("Expected size %d, got %d", len(data), result.SizeBytes)
	}
	if result.Metadata["format"] != "JSON" {
		t.Errorf("Expected format JSON, got %s", result.Metadata["format"])
	}

	// Verify blob was created
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

// TestIngest_HashVerification verifies the SHA256 hash is calculated correctly.
func TestIngest_HashVerification(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	outputDir := filepath.Join(tmpDir, "output")

	content := []byte(`{"meta":{"id":"TEST","title":"Test"},"books":[]}`)
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	expectedHash := sha256.Sum256(content)
	expectedHashHex := hex.EncodeToString(expectedHash[:])

	h := &Handler{}
	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if result.BlobSHA256 != expectedHashHex {
		t.Errorf("Expected hash %s, got %s", expectedHashHex, result.BlobSHA256)
	}

	// Verify the blob content matches
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	blobData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(blobData) != string(content) {
		t.Error("Blob content does not match original")
	}
}

// TestEnumerate_ValidFile tests enumeration of a JSON Bible file.
func TestEnumerate_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")

	content := []byte(`{"meta":{"id":"TEST"},"books":[]}`)
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Enumerate(testFile)
	if err != nil {
		t.Fatalf("Enumerate failed: %v", err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.json" {
		t.Errorf("Expected path 'test.json', got %s", entry.Path)
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
	if entry.IsDir {
		t.Error("Expected IsDir to be false")
	}
	if entry.Metadata["format"] != "JSON" {
		t.Errorf("Expected format JSON, got %s", entry.Metadata["format"])
	}
}

// TestExtractIR_StructuredBooks tests IR extraction from structured book format.
func TestExtractIR_StructuredBooks(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	validJSON := JSONBible{
		Meta: JSONMeta{
			ID:          "TEST",
			Title:       "Test Bible",
			Language:    "en",
			Description: "A test Bible",
			Version:     "1.0.0",
		},
		Books: []JSONBook{
			{
				ID:    "Gen",
				Name:  "Genesis",
				Order: 1,
				Chapters: []JSONChapter{
					{
						Number: 1,
						Verses: []JSONVerse{
							{
								Book:    "Gen",
								Chapter: 1,
								Verse:   1,
								Text:    "In the beginning...",
								ID:      "Gen.1.1",
							},
							{
								Book:    "Gen",
								Chapter: 1,
								Verse:   2,
								Text:    "And the earth was without form...",
								ID:      "Gen.1.2",
							},
						},
					},
					{
						Number: 2,
						Verses: []JSONVerse{
							{
								Book:    "Gen",
								Chapter: 2,
								Verse:   1,
								Text:    "Thus the heavens and the earth were finished...",
								ID:      "Gen.2.1",
							},
						},
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(validJSON, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.ExtractIR(testFile, outputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	if result.LossClass != "L0" {
		t.Errorf("Expected loss class L0, got %s", result.LossClass)
	}
	if result.LossReport.SourceFormat != "JSON" {
		t.Errorf("Expected source format JSON, got %s", result.LossReport.SourceFormat)
	}
	if result.LossReport.TargetFormat != "IR" {
		t.Errorf("Expected target format IR, got %s", result.LossReport.TargetFormat)
	}

	// Verify IR file was created
	if _, err := os.Stat(result.IRPath); os.IsNotExist(err) {
		t.Error("Expected IR file to exist")
	}

	// Read and verify IR content
	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatal(err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("Failed to parse IR: %v", err)
	}

	if corpus.ID != "TEST" {
		t.Errorf("Expected corpus ID TEST, got %s", corpus.ID)
	}
	if corpus.Title != "Test Bible" {
		t.Errorf("Expected title 'Test Bible', got %s", corpus.Title)
	}
	if corpus.Language != "en" {
		t.Errorf("Expected language en, got %s", corpus.Language)
	}
	if corpus.SourceFormat != "JSON" {
		t.Errorf("Expected source format JSON, got %s", corpus.SourceFormat)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(corpus.Documents))
	}

	doc := corpus.Documents[0]
	if doc.ID != "Gen" {
		t.Errorf("Expected document ID Gen, got %s", doc.ID)
	}
	if doc.Title != "Genesis" {
		t.Errorf("Expected document title Genesis, got %s", doc.Title)
	}
	if len(doc.ContentBlocks) != 3 {
		t.Fatalf("Expected 3 content blocks, got %d", len(doc.ContentBlocks))
	}

	// Verify _json_raw is stored for L0 round-trip
	if _, ok := corpus.Attributes["_json_raw"]; !ok {
		t.Error("Expected _json_raw attribute to be stored")
	}
}

// TestExtractIR_FlatVerses tests IR extraction from flat verse list format.
func TestExtractIR_FlatVerses(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	validJSON := JSONBible{
		Meta: JSONMeta{
			ID:      "TEST",
			Title:   "Test Bible",
			Version: "1.0.0",
		},
		Verses: []JSONVerse{
			{
				Book:    "Gen",
				Chapter: 1,
				Verse:   1,
				Text:    "In the beginning...",
				ID:      "Gen.1.1",
			},
			{
				Book:    "Gen",
				Chapter: 1,
				Verse:   2,
				Text:    "And the earth was without form...",
				ID:      "Gen.1.2",
			},
			{
				Book:    "Exod",
				Chapter: 1,
				Verse:   1,
				Text:    "Now these are the names...",
				ID:      "Exod.1.1",
			},
		},
	}

	data, err := json.MarshalIndent(validJSON, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.ExtractIR(testFile, outputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	// Read and verify IR content
	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatal(err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("Failed to parse IR: %v", err)
	}

	// Should create 2 documents (Gen and Exod)
	if len(corpus.Documents) != 2 {
		t.Fatalf("Expected 2 documents, got %d", len(corpus.Documents))
	}

	// Check first book
	if corpus.Documents[0].ID != "Gen" {
		t.Errorf("Expected first document ID Gen, got %s", corpus.Documents[0].ID)
	}
	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("Expected 2 content blocks in Gen, got %d", len(corpus.Documents[0].ContentBlocks))
	}

	// Check second book
	if corpus.Documents[1].ID != "Exod" {
		t.Errorf("Expected second document ID Exod, got %s", corpus.Documents[1].ID)
	}
	if len(corpus.Documents[1].ContentBlocks) != 1 {
		t.Errorf("Expected 1 content block in Exod, got %d", len(corpus.Documents[1].ContentBlocks))
	}
}

// TestExtractIR_IDGeneration tests that IDs are generated correctly.
func TestExtractIR_IDGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	validJSON := JSONBible{
		Meta: JSONMeta{
			ID:      "TEST",
			Title:   "Test Bible",
			Version: "1.0.0",
		},
		Books: []JSONBook{
			{
				ID:    "Gen",
				Name:  "Genesis",
				Order: 1,
				Chapters: []JSONChapter{
					{
						Number: 1,
						Verses: []JSONVerse{
							{
								Book:    "Gen",
								Chapter: 1,
								Verse:   1,
								Text:    "In the beginning...",
								ID:      "Gen.1.1",
							},
						},
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(validJSON, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.ExtractIR(testFile, outputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatal(err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("Failed to parse IR: %v", err)
	}

	// Check content block ID
	cb := corpus.Documents[0].ContentBlocks[0]
	if cb.ID != "cb-1" {
		t.Errorf("Expected content block ID 'cb-1', got %s", cb.ID)
	}
	if cb.Sequence != 1 {
		t.Errorf("Expected sequence 1, got %d", cb.Sequence)
	}

	// Check anchor ID
	if len(cb.Anchors) != 1 {
		t.Fatalf("Expected 1 anchor, got %d", len(cb.Anchors))
	}
	anchor := cb.Anchors[0]
	if anchor.ID != "a-1-0" {
		t.Errorf("Expected anchor ID 'a-1-0', got %s", anchor.ID)
	}

	// Check span ID and OSIS ID
	if len(anchor.Spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(anchor.Spans))
	}
	span := anchor.Spans[0]
	if span.ID != "s-Gen.1.1" {
		t.Errorf("Expected span ID 's-Gen.1.1', got %s", span.ID)
	}
	if span.Ref.OSISID != "Gen.1.1" {
		t.Errorf("Expected OSIS ID 'Gen.1.1', got %s", span.Ref.OSISID)
	}
}

// TestEmitNative_L0RoundTrip tests L0 round-trip emission using raw JSON.
func TestEmitNative_L0RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	irOutputDir := filepath.Join(tmpDir, "ir-output")
	emitOutputDir := filepath.Join(tmpDir, "emit-output")
	if err := os.MkdirAll(irOutputDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(emitOutputDir, 0755); err != nil {
		t.Fatal(err)
	}

	originalJSON := JSONBible{
		Meta: JSONMeta{
			ID:      "TEST",
			Title:   "Test Bible",
			Version: "1.0.0",
		},
		Books: []JSONBook{
			{
				ID:    "Gen",
				Name:  "Genesis",
				Order: 1,
				Chapters: []JSONChapter{
					{
						Number: 1,
						Verses: []JSONVerse{
							{
								Book:    "Gen",
								Chapter: 1,
								Verse:   1,
								Text:    "In the beginning...",
								ID:      "Gen.1.1",
							},
						},
					},
				},
			},
		},
	}

	originalData, err := json.MarshalIndent(originalJSON, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, originalData, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}

	// Extract IR
	extractResult, err := h.ExtractIR(testFile, irOutputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	// Emit native
	emitResult, err := h.EmitNative(extractResult.IRPath, emitOutputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	if emitResult.LossClass != "L0" {
		t.Errorf("Expected loss class L0, got %s", emitResult.LossClass)
	}
	if emitResult.Format != "JSON" {
		t.Errorf("Expected format JSON, got %s", emitResult.Format)
	}

	// Verify the emitted file matches the original (L0 round-trip)
	emittedData, err := os.ReadFile(emitResult.OutputPath)
	if err != nil {
		t.Fatal(err)
	}

	// Compare as JSON to handle formatting differences
	var originalParsed, emittedParsed JSONBible
	if err := json.Unmarshal(originalData, &originalParsed); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(emittedData, &emittedParsed); err != nil {
		t.Fatal(err)
	}

	if originalParsed.Meta.ID != emittedParsed.Meta.ID {
		t.Error("L0 round-trip: Meta ID does not match")
	}
	if len(originalParsed.Books) != len(emittedParsed.Books) {
		t.Error("L0 round-trip: Number of books does not match")
	}
}

// TestEmitNative_L1Generation tests L1 generation from IR without raw JSON.
func TestEmitNative_L1Generation(t *testing.T) {
	tmpDir := t.TempDir()
	irFile := filepath.Join(tmpDir, "test.ir.json")
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create IR without _json_raw attribute
	corpus := ir.Corpus{
		ID:           "TEST",
		Version:      "1.0.0",
		ModuleType:   ir.ModuleBible,
		Title:        "Test Bible",
		Language:     "en",
		Description:  "A test Bible",
		SourceFormat: "JSON",
		LossClass:    ir.LossL0,
		Attributes:   make(map[string]string), // No _json_raw
		Documents: []*ir.Document{
			{
				ID:         "Gen",
				Title:      "Genesis",
				Order:      1,
				Attributes: make(map[string]string),
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "In the beginning...",
						Hash:     "somehash",
						Anchors: []*ir.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*ir.Span{
									{
										ID:            "s-Gen.1.1",
										Type:          ir.SpanVerse,
										StartAnchorID: "a-1-0",
										Ref: &ir.Ref{
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

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(irFile, irData, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.EmitNative(irFile, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	if result.LossClass != "L1" {
		t.Errorf("Expected loss class L1 for generation, got %s", result.LossClass)
	}
	if result.Format != "JSON" {
		t.Errorf("Expected format JSON, got %s", result.Format)
	}

	// Verify the output file exists and is valid JSON
	outputData, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatal(err)
	}

	var generatedJSON JSONBible
	if err := json.Unmarshal(outputData, &generatedJSON); err != nil {
		t.Fatalf("Failed to parse generated JSON: %v", err)
	}

	if generatedJSON.Meta.ID != "TEST" {
		t.Errorf("Expected meta ID TEST, got %s", generatedJSON.Meta.ID)
	}
	if len(generatedJSON.Books) != 1 {
		t.Fatalf("Expected 1 book, got %d", len(generatedJSON.Books))
	}
	if generatedJSON.Books[0].ID != "Gen" {
		t.Errorf("Expected book ID Gen, got %s", generatedJSON.Books[0].ID)
	}
}

// TestManifest tests the plugin manifest.
func TestManifest(t *testing.T) {
	manifest := Manifest()

	if manifest.PluginID != "format.json" {
		t.Errorf("Expected plugin ID 'format.json', got %s", manifest.PluginID)
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", manifest.Version)
	}
	if manifest.Kind != "format" {
		t.Errorf("Expected kind 'format', got %s", manifest.Kind)
	}
	if manifest.Entrypoint != "format-json" {
		t.Errorf("Expected entrypoint 'format-json', got %s", manifest.Entrypoint)
	}

	// Check capabilities
	if len(manifest.Capabilities.Inputs) != 1 || manifest.Capabilities.Inputs[0] != "file" {
		t.Errorf("Expected inputs ['file'], got %v", manifest.Capabilities.Inputs)
	}
	if len(manifest.Capabilities.Outputs) != 1 || manifest.Capabilities.Outputs[0] != "artifact.kind:json-bible" {
		t.Errorf("Expected outputs ['artifact.kind:json-bible'], got %v", manifest.Capabilities.Outputs)
	}

	// Check IR support
	if manifest.IRSupport == nil {
		t.Fatal("Expected IRSupport to be set")
	}
	if !manifest.IRSupport.CanExtract {
		t.Error("Expected CanExtract to be true")
	}
	if !manifest.IRSupport.CanEmit {
		t.Error("Expected CanEmit to be true")
	}
	if manifest.IRSupport.LossClass != "L0" {
		t.Errorf("Expected LossClass 'L0', got %s", manifest.IRSupport.LossClass)
	}
	if len(manifest.IRSupport.Formats) != 1 || manifest.IRSupport.Formats[0] != "JSON" {
		t.Errorf("Expected formats ['JSON'], got %v", manifest.IRSupport.Formats)
	}
}

// TestExtractIR_EmptyFile tests handling of empty or minimal JSON files.
func TestExtractIR_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	minimalJSON := JSONBible{
		Meta: JSONMeta{
			ID:      "TEST",
			Version: "1.0.0",
		},
	}

	data, err := json.Marshal(minimalJSON)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.ExtractIR(testFile, outputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	// Read and verify IR content
	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatal(err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("Failed to parse IR: %v", err)
	}

	if len(corpus.Documents) != 0 {
		t.Errorf("Expected 0 documents for empty Bible, got %d", len(corpus.Documents))
	}
}

// TestIngest_ReadError tests ingest with unreadable file.
func TestIngest_ReadError(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	_, err := h.Ingest("/nonexistent/file.json", outputDir)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected 'failed to read file' error, got: %v", err)
	}
}

// TestEnumerate_StatError tests enumerate with nonexistent file.
func TestEnumerate_StatError(t *testing.T) {
	h := &Handler{}
	_, err := h.Enumerate("/nonexistent/file.json")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to stat") {
		t.Errorf("Expected 'failed to stat' error, got: %v", err)
	}
}

// TestExtractIR_ReadError tests ExtractIR with unreadable file.
func TestExtractIR_ReadError(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	_, err := h.ExtractIR("/nonexistent/file.json", outputDir)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected 'failed to read file' error, got: %v", err)
	}
}

// TestExtractIR_InvalidJson tests ExtractIR with invalid JSON.
func TestExtractIR_InvalidJson(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(testFile, []byte("{invalid json}"), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	_, err := h.ExtractIR(testFile, outputDir)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse JSON") {
		t.Errorf("Expected 'failed to parse JSON' error, got: %v", err)
	}
}

// TestEmitNative_ReadError tests EmitNative with unreadable IR file.
func TestEmitNative_ReadError(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	h := &Handler{}
	_, err := h.EmitNative("/nonexistent/ir.json", outputDir)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to read IR file") {
		t.Errorf("Expected 'failed to read IR file' error, got: %v", err)
	}
}

// TestEmitNative_InvalidIR tests EmitNative with invalid IR JSON.
func TestEmitNative_InvalidIR(t *testing.T) {
	tmpDir := t.TempDir()
	irFile := filepath.Join(tmpDir, "test.ir.json")
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(irFile, []byte("{invalid json}"), 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	_, err := h.EmitNative(irFile, outputDir)
	if err == nil {
		t.Error("Expected error for invalid IR JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse IR") {
		t.Errorf("Expected 'failed to parse IR' error, got: %v", err)
	}
}

// TestDetect_ValidJsonBibleWithVerses tests detection with flat verse list.
func TestDetect_ValidJsonBibleWithVerses(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")

	validJSON := JSONBible{
		Meta: JSONMeta{
			ID:      "TEST",
			Title:   "Test Bible",
			Version: "1.0.0",
		},
		Verses: []JSONVerse{
			{
				Book:    "Gen",
				Chapter: 1,
				Verse:   1,
				Text:    "In the beginning...",
				ID:      "Gen.1.1",
			},
		},
	}

	data, err := json.Marshal(validJSON)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed for flat verses, got: %s", result.Reason)
	}
}

// TestDetect_UnreadableFile tests detection when file cannot be read.
func TestDetect_UnreadableFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")

	// Create a file but make it unreadable
	if err := os.WriteFile(testFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(testFile, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(testFile, 0644) // Restore permissions for cleanup

	h := &Handler{}
	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for unreadable file")
	}
	if !strings.Contains(result.Reason, "cannot read file") {
		t.Errorf("Expected reason to mention read error, got: %s", result.Reason)
	}
}

// TestExtractIR_FlatVersesIDUsage tests that flat verses use verse.ID correctly.
func TestExtractIR_FlatVersesIDUsage(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	validJSON := JSONBible{
		Meta: JSONMeta{
			ID:      "TEST",
			Version: "1.0.0",
		},
		Verses: []JSONVerse{
			{
				Book:    "Gen",
				Chapter: 1,
				Verse:   1,
				Text:    "In the beginning...",
				ID:      "Gen.1.1",
			},
		},
	}

	data, err := json.MarshalIndent(validJSON, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.ExtractIR(testFile, outputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatal(err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("Failed to parse IR: %v", err)
	}

	// Check that the span ID uses the verse ID from the flat format
	span := corpus.Documents[0].ContentBlocks[0].Anchors[0].Spans[0]
	if span.ID != "s-Gen.1.1" {
		t.Errorf("Expected span ID 's-Gen.1.1', got %s", span.ID)
	}
}

// TestEmitNative_NoAnchors tests emission when content blocks have no anchors.
func TestEmitNative_NoAnchors(t *testing.T) {
	tmpDir := t.TempDir()
	irFile := filepath.Join(tmpDir, "test.ir.json")
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	corpus := ir.Corpus{
		ID:           "TEST",
		Version:      "1.0.0",
		ModuleType:   ir.ModuleBible,
		Title:        "Test Bible",
		SourceFormat: "JSON",
		Attributes:   make(map[string]string),
		Documents: []*ir.Document{
			{
				ID:         "Gen",
				Title:      "Genesis",
				Order:      1,
				Attributes: make(map[string]string),
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "Some text without anchors",
						Anchors:  []*ir.Anchor{},
					},
				},
			},
		},
	}

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(irFile, irData, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.EmitNative(irFile, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	outputData, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatal(err)
	}

	var generatedJSON JSONBible
	if err := json.Unmarshal(outputData, &generatedJSON); err != nil {
		t.Fatalf("Failed to parse generated JSON: %v", err)
	}

	// Should still generate the book structure even without verse spans
	if len(generatedJSON.Books) != 1 {
		t.Fatalf("Expected 1 book, got %d", len(generatedJSON.Books))
	}
}

// TestIngest_WriteError tests ingest when blob directory cannot be created.
func TestIngest_WriteError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")

	// Create test file
	content := []byte(`{"meta":{"id":"TEST"},"books":[]}`)
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Use /dev/null as output dir which can't have subdirectories created
	h := &Handler{}
	_, err := h.Ingest(testFile, "/dev/null/invalid")
	if err == nil {
		t.Error("Expected error when blob directory cannot be created")
	}
	if !strings.Contains(err.Error(), "failed to create blob dir") {
		t.Errorf("Expected 'failed to create blob dir' error, got: %v", err)
	}
}

// TestIngest_BlobWriteError tests ingest when blob file cannot be written.
func TestIngest_BlobWriteError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	outputDir := filepath.Join(tmpDir, "output")

	// Create test file
	content := []byte(`{"meta":{"id":"TEST"},"books":[]}`)
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Create output directory structure that will cause write to fail
	hash := sha256.Sum256(content)
	hashHex := hex.EncodeToString(hash[:])
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create the blob path as a directory to cause write failure
	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.MkdirAll(blobPath, 0755); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	_, err := h.Ingest(testFile, outputDir)
	if err == nil {
		t.Error("Expected error when blob file cannot be written")
	}
	if !strings.Contains(err.Error(), "failed to write blob") {
		t.Errorf("Expected 'failed to write blob' error, got: %v", err)
	}
}

// TestEmitNative_NoSpans tests emission when anchors have no spans.
func TestEmitNative_NoSpans(t *testing.T) {
	tmpDir := t.TempDir()
	irFile := filepath.Join(tmpDir, "test.ir.json")
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	corpus := ir.Corpus{
		ID:           "TEST",
		Version:      "1.0.0",
		ModuleType:   ir.ModuleBible,
		Title:        "Test Bible",
		SourceFormat: "JSON",
		Attributes:   make(map[string]string),
		Documents: []*ir.Document{
			{
				ID:         "Gen",
				Title:      "Genesis",
				Order:      1,
				Attributes: make(map[string]string),
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "Some text",
						Anchors: []*ir.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans:    []*ir.Span{}, // Empty spans
							},
						},
					},
				},
			},
		},
	}

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(irFile, irData, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.EmitNative(irFile, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	outputData, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatal(err)
	}

	var generatedJSON JSONBible
	if err := json.Unmarshal(outputData, &generatedJSON); err != nil {
		t.Fatalf("Failed to parse generated JSON: %v", err)
	}

	// Should still generate the book structure
	if len(generatedJSON.Books) != 1 {
		t.Fatalf("Expected 1 book, got %d", len(generatedJSON.Books))
	}
}

// TestEmitNative_NonVerseSpan tests emission when span is not a verse type.
func TestEmitNative_NonVerseSpan(t *testing.T) {
	tmpDir := t.TempDir()
	irFile := filepath.Join(tmpDir, "test.ir.json")
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	corpus := ir.Corpus{
		ID:           "TEST",
		Version:      "1.0.0",
		ModuleType:   ir.ModuleBible,
		Title:        "Test Bible",
		SourceFormat: "JSON",
		Attributes:   make(map[string]string),
		Documents: []*ir.Document{
			{
				ID:         "Gen",
				Title:      "Genesis",
				Order:      1,
				Attributes: make(map[string]string),
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "Some text",
						Anchors: []*ir.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*ir.Span{
									{
										ID:            "s-note-1",
										Type:          "note", // Not a verse type
										StartAnchorID: "a-1-0",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(irFile, irData, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.EmitNative(irFile, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	outputData, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatal(err)
	}

	var generatedJSON JSONBible
	if err := json.Unmarshal(outputData, &generatedJSON); err != nil {
		t.Fatalf("Failed to parse generated JSON: %v", err)
	}

	// Should still generate the book structure but no verses
	if len(generatedJSON.Books) != 1 {
		t.Fatalf("Expected 1 book, got %d", len(generatedJSON.Books))
	}
}

// TestEmitNative_NilRef tests emission when span has nil reference.
func TestEmitNative_NilRef(t *testing.T) {
	tmpDir := t.TempDir()
	irFile := filepath.Join(tmpDir, "test.ir.json")
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	corpus := ir.Corpus{
		ID:           "TEST",
		Version:      "1.0.0",
		ModuleType:   ir.ModuleBible,
		Title:        "Test Bible",
		SourceFormat: "JSON",
		Attributes:   make(map[string]string),
		Documents: []*ir.Document{
			{
				ID:         "Gen",
				Title:      "Genesis",
				Order:      1,
				Attributes: make(map[string]string),
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "Some text",
						Anchors: []*ir.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*ir.Span{
									{
										ID:            "s-1",
										Type:          ir.SpanVerse,
										StartAnchorID: "a-1-0",
										Ref:           nil, // Nil reference
									},
								},
							},
						},
					},
				},
			},
		},
	}

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(irFile, irData, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.EmitNative(irFile, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	outputData, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatal(err)
	}

	var generatedJSON JSONBible
	if err := json.Unmarshal(outputData, &generatedJSON); err != nil {
		t.Fatalf("Failed to parse generated JSON: %v", err)
	}

	// Should still generate the book structure
	if len(generatedJSON.Books) != 1 {
		t.Fatalf("Expected 1 book, got %d", len(generatedJSON.Books))
	}
}

// TestEmitNative_MultipleChapters tests emission with multiple chapters.
func TestEmitNative_MultipleChapters(t *testing.T) {
	tmpDir := t.TempDir()
	irFile := filepath.Join(tmpDir, "test.ir.json")
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	corpus := ir.Corpus{
		ID:           "TEST",
		Version:      "1.0.0",
		ModuleType:   ir.ModuleBible,
		Title:        "Test Bible",
		SourceFormat: "JSON",
		Attributes:   make(map[string]string),
		Documents: []*ir.Document{
			{
				ID:         "Gen",
				Title:      "Genesis",
				Order:      1,
				Attributes: make(map[string]string),
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "First verse",
						Anchors: []*ir.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*ir.Span{
									{
										ID:            "s-Gen.1.1",
										Type:          ir.SpanVerse,
										StartAnchorID: "a-1-0",
										Ref: &ir.Ref{
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
					{
						ID:       "cb-2",
						Sequence: 2,
						Text:     "Second chapter verse",
						Anchors: []*ir.Anchor{
							{
								ID:       "a-2-0",
								Position: 0,
								Spans: []*ir.Span{
									{
										ID:            "s-Gen.2.1",
										Type:          ir.SpanVerse,
										StartAnchorID: "a-2-0",
										Ref: &ir.Ref{
											Book:    "Gen",
											Chapter: 2,
											Verse:   1,
											OSISID:  "Gen.2.1",
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

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(irFile, irData, 0644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{}
	result, err := h.EmitNative(irFile, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	outputData, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatal(err)
	}

	var generatedJSON JSONBible
	if err := json.Unmarshal(outputData, &generatedJSON); err != nil {
		t.Fatalf("Failed to parse generated JSON: %v", err)
	}

	if len(generatedJSON.Books) != 1 {
		t.Fatalf("Expected 1 book, got %d", len(generatedJSON.Books))
	}
	if len(generatedJSON.Books[0].Chapters) != 2 {
		t.Fatalf("Expected 2 chapters, got %d", len(generatedJSON.Books[0].Chapters))
	}
	if generatedJSON.Books[0].Chapters[0].Number != 1 {
		t.Errorf("Expected chapter 1, got %d", generatedJSON.Books[0].Chapters[0].Number)
	}
	if generatedJSON.Books[0].Chapters[1].Number != 2 {
		t.Errorf("Expected chapter 2, got %d", generatedJSON.Books[0].Chapters[1].Number)
	}
}
