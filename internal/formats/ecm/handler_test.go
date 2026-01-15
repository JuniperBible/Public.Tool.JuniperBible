package ecm

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	manifest := Manifest()
	if manifest == nil {
		t.Fatal("Expected manifest to be non-nil")
	}

	if manifest.PluginID != "format.ecm" {
		t.Errorf("Expected plugin ID 'format.ecm', got %s", manifest.PluginID)
	}
	if manifest.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", manifest.Version)
	}
	if manifest.Kind != "format" {
		t.Errorf("Expected kind 'format', got %s", manifest.Kind)
	}
	if manifest.Entrypoint != "format-ecm" {
		t.Errorf("Expected entrypoint 'format-ecm', got %s", manifest.Entrypoint)
	}

	// Verify capabilities
	if len(manifest.Capabilities.Inputs) == 0 {
		t.Error("Expected inputs to be non-empty")
	}
	if len(manifest.Capabilities.Outputs) == 0 {
		t.Error("Expected outputs to be non-empty")
	}
}

func TestRegister(t *testing.T) {
	// Test that register doesn't panic
	Register()
}

func TestHandler_Detect_ValidECM(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ecm.xml")

	content := `<?xml version="1.0" encoding="UTF-8"?>
<ECM book="John" chapter="1" edition="ECM2">
  <header>
    <title>John Chapter 1</title>
  </header>
  <apparatus id="app1" verse="1" unit="1">
    <baseText>In the beginning was the Word</baseText>
    <variant id="v1" type="omission">
      <reading>was</reading>
      <witness>P66</witness>
      <witness>P75</witness>
    </variant>
  </apparatus>
</ECM>`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed, got: %s", result.Reason)
	}
	if result.Format != "ECM" {
		t.Errorf("Expected format 'ECM', got %s", result.Format)
	}
}

func TestHandler_Detect_ValidECMWithVariant(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test2.ecm.xml")

	// Test with 'variant' keyword instead of <apparatus>
	content := `<?xml version="1.0" encoding="UTF-8"?>
<ECM>
  <variant id="v1">
    <reading>test</reading>
  </variant>
</ECM>`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if !result.Detected {
		t.Errorf("Expected detection to succeed for variant keyword, got: %s", result.Reason)
	}
}

func TestHandler_Detect_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xml")

	content := `<?xml version="1.0"?>
<root>
  <data>Not an ECM file</data>
</root>`

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for non-ECM file")
	}
	if !strings.Contains(result.Reason, "not an ECM XML file") {
		t.Errorf("Expected reason to mention 'not an ECM XML file', got: %s", result.Reason)
	}
}

func TestHandler_Detect_NonExistentFile(t *testing.T) {
	handler := &Handler{}
	result, err := handler.Detect("/nonexistent/file.xml")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if result.Detected {
		t.Error("Expected detection to fail for non-existent file")
	}
	if !strings.Contains(result.Reason, "cannot read file") {
		t.Errorf("Expected reason to mention 'cannot read file', got: %s", result.Reason)
	}
}

func TestHandler_Ingest(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ecm.xml")
	content := []byte(`<?xml version="1.0"?>
<ECM book="John" chapter="1">
  <apparatus verse="1">
    <baseText>Test</baseText>
  </apparatus>
</ECM>`)

	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if result.ArtifactID == "" {
		t.Error("Expected non-empty artifact ID")
	}
	if result.BlobSHA256 == "" {
		t.Error("Expected non-empty blob SHA256")
	}
	if result.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), result.SizeBytes)
	}
	if result.Metadata["format"] != "ecm" {
		t.Errorf("Expected format 'ecm', got %s", result.Metadata["format"])
	}
	if result.Metadata["filename"] != "test.ecm.xml" {
		t.Errorf("Expected filename 'test.ecm.xml', got %s", result.Metadata["filename"])
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}

	// Verify blob content
	blobData, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(blobData) != string(content) {
		t.Error("Blob content doesn't match original")
	}
}

func TestHandler_Ingest_NonExistentFile(t *testing.T) {
	handler := &Handler{}
	_, err := handler.Ingest("/nonexistent/file.xml", "/tmp/output")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected error about reading file, got: %v", err)
	}
}

func TestHandler_Enumerate(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ecm.xml")
	content := []byte("test content")

	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Enumerate(testFile)
	if err != nil {
		t.Fatalf("Enumerate failed: %v", err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.ecm.xml" {
		t.Errorf("Expected path 'test.ecm.xml', got %s", entry.Path)
	}
	if entry.SizeBytes != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), entry.SizeBytes)
	}
	if entry.IsDir {
		t.Error("Expected IsDir to be false")
	}
}

func TestHandler_Enumerate_NonExistentFile(t *testing.T) {
	handler := &Handler{}
	_, err := handler.Enumerate("/nonexistent/file.xml")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to stat file") {
		t.Errorf("Expected error about stat, got: %v", err)
	}
}

func TestHandler_ExtractIR(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.ecm.xml")

	ecm := &ECMXML{
		Book:    "John",
		Chapter: "1",
		Edition: "ECM2",
		Header: &ECMHeader{
			Title:       "Gospel of John",
			Description: "Critical edition",
			Publisher:   "Test Publisher",
			Rights:      "CC BY 4.0",
		},
		Apparatus: []*Apparatus{
			{
				ID:       "app1",
				Verse:    "1",
				Unit:     "1",
				BaseText: "In the beginning was the Word",
				Variants: []*Variant{
					{
						ID:        "v1",
						Reading:   "the Word was",
						Witnesses: []string{"P66", "P75"},
						Type:      "transposition",
					},
				},
				Witnesses: []*Witness{
					{
						ID:          "P66",
						Siglum:      "𝔓66",
						Name:        "Papyrus 66",
						Date:        "c. 200",
						Description: "Bodmer II",
					},
				},
				Commentary: "Important variant",
				Annotations: []*Annotation{
					{
						Type:  "note",
						Value: "Early witness",
					},
				},
			},
		},
	}

	xmlData, err := xml.MarshalIndent(ecm, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	content := []byte(xml.Header + string(xmlData))
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.ExtractIR(testFile, outputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	if result.LossClass != "L1" {
		t.Errorf("Expected loss class 'L1', got %s", result.LossClass)
	}
	if result.LossReport == nil {
		t.Fatal("Expected loss report to be non-nil")
	}
	if len(result.LossReport.Warnings) == 0 {
		t.Error("Expected at least one warning")
	}

	// Verify IR file was written
	if _, err := os.Stat(result.IRPath); os.IsNotExist(err) {
		t.Error("Expected IR file to exist")
	}

	// Verify IR content
	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatal(err)
	}

	var corpus map[string]interface{}
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("Failed to parse IR JSON: %v", err)
	}

	// Verify corpus structure
	if corpus["id"] != "ecm-corpus" {
		t.Errorf("Expected corpus ID 'ecm-corpus', got %v", corpus["id"])
	}
	if corpus["source_format"] != "ecm" {
		t.Errorf("Expected source format 'ecm', got %v", corpus["source_format"])
	}
	if corpus["loss_class"] != "L1" {
		t.Errorf("Expected loss class 'L1', got %v", corpus["loss_class"])
	}
	if corpus["title"] != "Gospel of John" {
		t.Errorf("Expected title 'Gospel of John', got %v", corpus["title"])
	}
	if corpus["language"] != "grc" {
		t.Errorf("Expected language 'grc', got %v", corpus["language"])
	}

	// Verify attributes
	attrs, ok := corpus["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected attributes to be a map")
	}
	if attrs["book"] != "John" {
		t.Errorf("Expected book 'John', got %v", attrs["book"])
	}
	if attrs["chapter"] != "1" {
		t.Errorf("Expected chapter '1', got %v", attrs["chapter"])
	}

	// Verify documents
	docs, ok := corpus["documents"].([]interface{})
	if !ok {
		t.Fatal("Expected documents to be an array")
	}
	if len(docs) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(docs))
	}
}

func TestHandler_ExtractIR_NonExistentFile(t *testing.T) {
	handler := &Handler{}
	_, err := handler.ExtractIR("/nonexistent/file.xml", "/tmp/output")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("Expected error about reading file, got: %v", err)
	}
}

func TestHandler_ExtractIR_InvalidXML(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "invalid.xml")

	content := []byte("not valid xml {{{")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	_, err := handler.ExtractIR(testFile, outputDir)
	if err == nil {
		t.Error("Expected error for invalid XML")
	}
	if !strings.Contains(err.Error(), "failed to parse ECM XML") {
		t.Errorf("Expected error about parsing XML, got: %v", err)
	}
}

func TestHandler_EmitNative(t *testing.T) {
	tmpDir := t.TempDir()

	// Create IR file
	corpus := map[string]interface{}{
		"id":            "ecm-corpus",
		"version":       "1.0",
		"module_type":   "bible",
		"language":      "grc",
		"title":         "Test Title",
		"description":   "Test Description",
		"publisher":     "Test Publisher",
		"rights":        "Test Rights",
		"source_hash":   "abc123",
		"source_format": "ecm",
		"loss_class":    "L1",
		"attributes": map[string]string{
			"book":    "John",
			"chapter": "1",
			"edition": "ECM2",
		},
		"documents": []interface{}{
			map[string]interface{}{
				"id":    "apparatus-1",
				"title": "Verse 1 Unit 1",
				"order": 1,
				"attributes": map[string]string{
					"apparatus_id": "app1",
					"verse":        "1",
					"unit":         "1",
				},
				"content_blocks": []interface{}{
					map[string]interface{}{
						"id":       "block-1-1",
						"sequence": 1,
						"text":     "In the beginning",
						"attributes": map[string]interface{}{
							"commentary": "Test commentary",
							"variants": []map[string]interface{}{
								{
									"id":        "v1",
									"reading":   "beginning was",
									"witnesses": []interface{}{"P66"},
									"type":      "transposition",
								},
							},
							"witnesses": []map[string]interface{}{
								{
									"id":          "P66",
									"siglum":      "𝔓66",
									"name":        "Papyrus 66",
									"date":        "c. 200",
									"description": "Bodmer II",
								},
							},
							"annotations": []map[string]interface{}{
								{
									"type":  "note",
									"value": "Important",
								},
							},
						},
					},
				},
			},
		},
	}

	irPath := filepath.Join(tmpDir, "corpus.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.EmitNative(irPath, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	if result.Format != "ecm" {
		t.Errorf("Expected format 'ecm', got %s", result.Format)
	}
	if result.LossClass != "L1" {
		t.Errorf("Expected loss class 'L1', got %s", result.LossClass)
	}
	if result.LossReport == nil {
		t.Fatal("Expected loss report to be non-nil")
	}

	// Verify output file exists
	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Error("Expected output file to exist")
	}

	// Verify output is valid XML
	outputData, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatal(err)
	}

	var ecm ECMXML
	if err := xml.Unmarshal(outputData, &ecm); err != nil {
		t.Fatalf("Failed to parse output XML: %v", err)
	}

	// Verify XML content
	if ecm.Book != "John" {
		t.Errorf("Expected book 'John', got %s", ecm.Book)
	}
	if ecm.Chapter != "1" {
		t.Errorf("Expected chapter '1', got %s", ecm.Chapter)
	}
	if ecm.Header == nil {
		t.Fatal("Expected header to be non-nil")
	}
	if ecm.Header.Title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got %s", ecm.Header.Title)
	}
	if len(ecm.Apparatus) != 1 {
		t.Fatalf("Expected 1 apparatus entry, got %d", len(ecm.Apparatus))
	}

	app := ecm.Apparatus[0]
	if app.BaseText != "In the beginning" {
		t.Errorf("Expected base text 'In the beginning', got %s", app.BaseText)
	}
	if len(app.Variants) != 1 {
		t.Fatalf("Expected 1 variant, got %d", len(app.Variants))
	}
	if app.Commentary != "Test commentary" {
		t.Errorf("Expected commentary 'Test commentary', got %s", app.Commentary)
	}
}

func TestHandler_EmitNative_NonExistentFile(t *testing.T) {
	handler := &Handler{}
	_, err := handler.EmitNative("/nonexistent/ir.json", "/tmp/output")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read IR") {
		t.Errorf("Expected error about reading IR, got: %v", err)
	}
}

func TestHandler_EmitNative_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "invalid.json")

	if err := os.WriteFile(irPath, []byte("not valid json {{{"), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	_, err := handler.EmitNative(irPath, outputDir)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to unmarshal IR") {
		t.Errorf("Expected error about unmarshaling IR, got: %v", err)
	}
}

func TestECMToIR(t *testing.T) {
	ecm := &ECMXML{
		Book:    "Mark",
		Chapter: "1",
		Edition: "ECM1",
		Header: &ECMHeader{
			Title:       "Gospel of Mark",
			Description: "Critical apparatus",
			Publisher:   "Institute",
			Rights:      "Open",
		},
		Apparatus: []*Apparatus{
			{
				ID:       "app1",
				Verse:    "1",
				Unit:     "1",
				BaseText: "The beginning",
				Variants: []*Variant{
					{
						ID:        "v1",
						Reading:   "A beginning",
						Witnesses: []string{"Sinaiticus"},
						Type:      "substitution",
					},
				},
				Witnesses: []*Witness{
					{
						ID:          "Sinaiticus",
						Siglum:      "א",
						Name:        "Codex Sinaiticus",
						Date:        "4th century",
						Description: "One of the oldest manuscripts",
					},
				},
				Commentary: "Significant reading",
				Annotations: []*Annotation{
					{Type: "scribal", Value: "Corrected"},
				},
			},
		},
	}

	corpus := ecmToIR(ecm)

	if corpus["id"] != "ecm-corpus" {
		t.Errorf("Expected corpus ID 'ecm-corpus', got %v", corpus["id"])
	}
	if corpus["language"] != "grc" {
		t.Errorf("Expected language 'grc', got %v", corpus["language"])
	}
	if corpus["title"] != "Gospel of Mark" {
		t.Errorf("Expected title, got %v", corpus["title"])
	}

	attrs := corpus["attributes"].(map[string]string)
	if attrs["book"] != "Mark" {
		t.Errorf("Expected book 'Mark', got %s", attrs["book"])
	}

	docsIface := corpus["documents"].([]interface{})
	if len(docsIface) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(docsIface))
	}

	doc := docsIface[0].(map[string]interface{})
	if doc["id"] != "apparatus-1" {
		t.Errorf("Expected document ID 'apparatus-1', got %v", doc["id"])
	}
}

func TestIRToECM(t *testing.T) {
	corpus := map[string]interface{}{
		"id":          "test-corpus",
		"title":       "Test Title",
		"description": "Test Description",
		"publisher":   "Test Publisher",
		"rights":      "Test Rights",
		"attributes": map[string]interface{}{
			"book":    "Luke",
			"chapter": "2",
			"edition": "ECM3",
		},
		"documents": []interface{}{
			map[string]interface{}{
				"id": "doc1",
				"attributes": map[string]interface{}{
					"apparatus_id": "app2",
					"verse":        "2",
					"unit":         "2",
				},
				"content_blocks": []interface{}{
					map[string]interface{}{
						"id":   "block1",
						"text": "Test text",
						"attributes": map[string]interface{}{
							"commentary": "Test note",
							"variants": []interface{}{
								map[string]interface{}{
									"id":        "var1",
									"reading":   "Different text",
									"type":      "omission",
									"witnesses": []interface{}{"P75"},
								},
							},
							"witnesses": []interface{}{
								map[string]interface{}{
									"id":          "P75",
									"siglum":      "𝔓75",
									"name":        "Papyrus 75",
									"date":        "3rd century",
									"description": "Early papyrus",
								},
							},
							"annotations": []interface{}{
								map[string]interface{}{
									"type":  "textual",
									"value": "Note here",
								},
							},
						},
					},
				},
			},
		},
	}

	ecm := irToECM(corpus)

	if ecm.Book != "Luke" {
		t.Errorf("Expected book 'Luke', got %s", ecm.Book)
	}
	if ecm.Chapter != "2" {
		t.Errorf("Expected chapter '2', got %s", ecm.Chapter)
	}
	if ecm.Header == nil {
		t.Fatal("Expected header to be non-nil")
	}
	if ecm.Header.Title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got %s", ecm.Header.Title)
	}
	if len(ecm.Apparatus) != 1 {
		t.Fatalf("Expected 1 apparatus entry, got %d", len(ecm.Apparatus))
	}

	app := ecm.Apparatus[0]
	if app.ID != "app2" {
		t.Errorf("Expected apparatus ID 'app2', got %s", app.ID)
	}
	if app.BaseText != "Test text" {
		t.Errorf("Expected base text 'Test text', got %s", app.BaseText)
	}
	if app.Commentary != "Test note" {
		t.Errorf("Expected commentary, got %s", app.Commentary)
	}
}

func TestVariantsToJSON(t *testing.T) {
	variants := []*Variant{
		{
			ID:        "v1",
			Reading:   "text",
			Witnesses: []string{"P66", "P75"},
			Type:      "addition",
		},
		{
			ID:      "v2",
			Reading: "other",
		},
	}

	result := variantsToJSON(variants)

	if len(result) != 2 {
		t.Fatalf("Expected 2 variants, got %d", len(result))
	}
	v0 := result[0].(map[string]interface{})
	if v0["id"] != "v1" {
		t.Errorf("Expected id 'v1', got %v", v0["id"])
	}
	if v0["type"] != "addition" {
		t.Errorf("Expected type 'addition', got %v", v0["type"])
	}
}

func TestWitnessesToJSON(t *testing.T) {
	witnesses := []*Witness{
		{
			ID:          "P66",
			Siglum:      "𝔓66",
			Name:        "Papyrus 66",
			Date:        "c. 200",
			Description: "Bodmer II",
		},
	}

	result := witnessesToJSON(witnesses)

	if len(result) != 1 {
		t.Fatalf("Expected 1 witness, got %d", len(result))
	}
	w0 := result[0].(map[string]interface{})
	if w0["id"] != "P66" {
		t.Errorf("Expected id 'P66', got %v", w0["id"])
	}
	if w0["name"] != "Papyrus 66" {
		t.Errorf("Expected name 'Papyrus 66', got %v", w0["name"])
	}
}

func TestAnnotationsToJSON(t *testing.T) {
	annotations := []*Annotation{
		{Type: "note", Value: "Important"},
		{Type: "reference", Value: "See also"},
	}

	result := annotationsToJSON(annotations)

	if len(result) != 2 {
		t.Fatalf("Expected 2 annotations, got %d", len(result))
	}
	a0 := result[0].(map[string]interface{})
	a1 := result[1].(map[string]interface{})
	if a0["type"] != "note" {
		t.Errorf("Expected type 'note', got %v", a0["type"])
	}
	if a1["value"] != "See also" {
		t.Errorf("Expected value 'See also', got %v", a1["value"])
	}
}

func TestJSONToVariants(t *testing.T) {
	data := []interface{}{
		map[string]interface{}{
			"id":        "v1",
			"reading":   "test",
			"type":      "omission",
			"witnesses": []interface{}{"P66", "P75"},
		},
	}

	result := jsonToVariants(data)

	if len(result) != 1 {
		t.Fatalf("Expected 1 variant, got %d", len(result))
	}
	if result[0].ID != "v1" {
		t.Errorf("Expected ID 'v1', got %s", result[0].ID)
	}
	if len(result[0].Witnesses) != 2 {
		t.Fatalf("Expected 2 witnesses, got %d", len(result[0].Witnesses))
	}
}

func TestJSONToVariants_Empty(t *testing.T) {
	result := jsonToVariants(nil)
	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d items", len(result))
	}

	result = jsonToVariants("not an array")
	if len(result) != 0 {
		t.Errorf("Expected empty result for invalid data, got %d items", len(result))
	}
}

func TestJSONToWitnesses(t *testing.T) {
	data := []interface{}{
		map[string]interface{}{
			"id":          "P66",
			"siglum":      "𝔓66",
			"name":        "Papyrus 66",
			"date":        "c. 200",
			"description": "Bodmer II",
		},
	}

	result := jsonToWitnesses(data)

	if len(result) != 1 {
		t.Fatalf("Expected 1 witness, got %d", len(result))
	}
	if result[0].ID != "P66" {
		t.Errorf("Expected ID 'P66', got %s", result[0].ID)
	}
	if result[0].Siglum != "𝔓66" {
		t.Errorf("Expected siglum '𝔓66', got %s", result[0].Siglum)
	}
}

func TestJSONToWitnesses_Empty(t *testing.T) {
	result := jsonToWitnesses(nil)
	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d items", len(result))
	}
}

func TestJSONToAnnotations(t *testing.T) {
	data := []interface{}{
		map[string]interface{}{
			"type":  "note",
			"value": "Important",
		},
	}

	result := jsonToAnnotations(data)

	if len(result) != 1 {
		t.Fatalf("Expected 1 annotation, got %d", len(result))
	}
	if result[0].Type != "note" {
		t.Errorf("Expected type 'note', got %s", result[0].Type)
	}
	if result[0].Value != "Important" {
		t.Errorf("Expected value 'Important', got %s", result[0].Value)
	}
}

func TestJSONToAnnotations_Empty(t *testing.T) {
	result := jsonToAnnotations(nil)
	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d items", len(result))
	}
}

func TestGetString(t *testing.T) {
	m := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}

	if getString(m, "key1") != "value1" {
		t.Error("Expected to get string value")
	}
	if getString(m, "key2") != "" {
		t.Error("Expected empty string for non-string value")
	}
	if getString(m, "missing") != "" {
		t.Error("Expected empty string for missing key")
	}
}

func TestRoundTrip(t *testing.T) {
	// Create original ECM
	original := &ECMXML{
		Book:    "Romans",
		Chapter: "1",
		Edition: "ECM",
		Header: &ECMHeader{
			Title:       "Romans 1",
			Description: "Test description",
			Publisher:   "Test",
			Rights:      "Open",
		},
		Apparatus: []*Apparatus{
			{
				ID:       "app1",
				Verse:    "1",
				Unit:     "1",
				BaseText: "Paul, a servant",
				Variants: []*Variant{
					{
						ID:        "v1",
						Reading:   "a slave",
						Witnesses: []string{"P46"},
						Type:      "substitution",
					},
				},
				Witnesses: []*Witness{
					{
						ID:          "P46",
						Siglum:      "𝔓46",
						Name:        "Chester Beatty II",
						Date:        "c. 200",
						Description: "Pauline epistles",
					},
				},
				Commentary: "Key variant",
				Annotations: []*Annotation{
					{Type: "note", Value: "See commentary"},
				},
			},
		},
	}

	// Convert to IR
	corpus := ecmToIR(original)

	// Convert back to ECM
	result := irToECM(corpus)

	// Compare
	if result.Book != original.Book {
		t.Errorf("Book mismatch: %s != %s", result.Book, original.Book)
	}
	if result.Chapter != original.Chapter {
		t.Errorf("Chapter mismatch: %s != %s", result.Chapter, original.Chapter)
	}
	if len(result.Apparatus) != len(original.Apparatus) {
		t.Fatalf("Apparatus count mismatch: %d != %d", len(result.Apparatus), len(original.Apparatus))
	}

	origApp := original.Apparatus[0]
	resApp := result.Apparatus[0]

	if resApp.BaseText != origApp.BaseText {
		t.Errorf("BaseText mismatch: %s != %s", resApp.BaseText, origApp.BaseText)
	}
	if len(resApp.Variants) != len(origApp.Variants) {
		t.Fatalf("Variants count mismatch: %d != %d", len(resApp.Variants), len(origApp.Variants))
	}
	if resApp.Variants[0].Reading != origApp.Variants[0].Reading {
		t.Errorf("Variant reading mismatch: %s != %s", resApp.Variants[0].Reading, origApp.Variants[0].Reading)
	}
}
