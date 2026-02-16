//go:build !sdk

package main

import (
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectECM(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		detected bool
		format   string
	}{
		{
			name: "valid ECM XML",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<ECM book="John" chapter="1" edition="ECM.2">
  <apparatus verse="1" unit="1">
    <baseText>In the beginning was the Word</baseText>
    <variant id="v1">
      <reading>In beginning was the Word</reading>
      <witness>P66</witness>
    </variant>
  </apparatus>
</ECM>`,
			detected: true,
			format:   "ECM",
		},
		{
			name: "ECM with apparatus only",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<ECM>
  <apparatus verse="5" unit="2">
    <baseText>And the light shines in darkness</baseText>
  </apparatus>
</ECM>`,
			detected: true,
			format:   "ECM",
		},
		{
			name:     "not ECM - generic XML",
			content:  `<?xml version="1.0"?><root><data>test</data></root>`,
			detected: false,
		},
		{
			name:     "not ECM - plain text",
			content:  "This is just plain text",
			detected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "ecm-test-*.xml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(tt.content)); err != nil {
				t.Fatal(err)
			}
			tmpfile.Close()

			args := map[string]interface{}{"path": tmpfile.Name()}

			// Note: In a real test, we'd capture stdout and parse the JSON response
			// For now, we just verify the function doesn't panic
			handleDetect(args)
		})
	}
}

func TestECMToIR(t *testing.T) {
	ecm := &ECMXML{
		Book:    "John",
		Chapter: "1",
		Edition: "ECM.2",
		Header: &ECMHeader{
			Title:       "Gospel of John - ECM Edition",
			Description: "Critical apparatus for John 1",
			Publisher:   "INTF",
			Rights:      "CC BY-SA 4.0",
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
						Reading:   "In beginning was the Word",
						Witnesses: []string{"P66", "P75"},
						Type:      "omission",
					},
					{
						ID:        "v2",
						Reading:   "In the beginning was a Word",
						Witnesses: []string{"Aleph", "B"},
						Type:      "variation",
					},
				},
				Witnesses: []*Witness{
					{
						ID:          "w1",
						Siglum:      "P66",
						Name:        "Papyrus 66",
						Date:        "c. 200",
						Description: "Bodmer II papyrus",
					},
					{
						ID:          "w2",
						Siglum:      "P75",
						Name:        "Papyrus 75",
						Date:        "early 3rd century",
						Description: "Bodmer XIV-XV",
					},
				},
				Commentary: "The article is omitted in early papyri",
				Annotations: []*Annotation{
					{Type: "note", Value: "Critical decision based on CBGM"},
				},
			},
		},
	}

	corpus := ecmToIR(ecm)

	// Verify corpus metadata
	if corpus.Title != "Gospel of John - ECM Edition" {
		t.Errorf("expected title 'Gospel of John - ECM Edition', got %q", corpus.Title)
	}
	if corpus.Publisher != "INTF" {
		t.Errorf("expected publisher 'INTF', got %q", corpus.Publisher)
	}
	if corpus.Attributes["book"] != "John" {
		t.Errorf("expected book 'John', got %q", corpus.Attributes["book"])
	}
	if corpus.Attributes["chapter"] != "1" {
		t.Errorf("expected chapter '1', got %q", corpus.Attributes["chapter"])
	}

	// Verify document structure
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}

	doc := corpus.Documents[0]
	if doc.Attributes["verse"] != "1" {
		t.Errorf("expected verse '1', got %q", doc.Attributes["verse"])
	}
	if doc.Attributes["unit"] != "1" {
		t.Errorf("expected unit '1', got %q", doc.Attributes["unit"])
	}

	// Verify content block
	if len(doc.ContentBlocks) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(doc.ContentBlocks))
	}

	block := doc.ContentBlocks[0]
	if block.Text != "In the beginning was the Word" {
		t.Errorf("expected text 'In the beginning was the Word', got %q", block.Text)
	}

	// Verify variants are preserved
	if _, ok := block.Attributes["variants"]; !ok {
		t.Error("expected variants in block attributes")
	}

	// Verify witnesses are preserved
	if _, ok := block.Attributes["witnesses"]; !ok {
		t.Error("expected witnesses in block attributes")
	}

	// Verify commentary is preserved
	if commentary, ok := block.Attributes["commentary"].(string); !ok || commentary != "The article is omitted in early papyri" {
		t.Errorf("expected commentary, got %v", block.Attributes["commentary"])
	}

	// Verify anchors for variants
	if len(block.Anchors) != 2 {
		t.Errorf("expected 2 anchors for variants, got %d", len(block.Anchors))
	}
}

func TestIRToECM(t *testing.T) {
	corpus := &ipc.Corpus{
		ID:          "ecm-test",
		Version:     "1.0",
		ModuleType:  "bible",
		Title:       "Test ECM Corpus",
		Description: "Test description",
		Publisher:   "Test Publisher",
		Rights:      "Test Rights",
		Attributes: map[string]string{
			"book":    "John",
			"chapter": "1",
			"edition": "ECM.2",
		},
		Documents: []*ipc.Document{
			{
				ID:    "doc1",
				Title: "Verse 1 Unit 1",
				Order: 1,
				Attributes: map[string]string{
					"apparatus_id": "app1",
					"verse":        "1",
					"unit":         "1",
				},
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "block1",
						Sequence: 1,
						Text:     "In the beginning was the Word",
						Attributes: map[string]interface{}{
							"variants": []interface{}{
								map[string]interface{}{
									"id":        "v1",
									"reading":   "In beginning was the Word",
									"witnesses": []interface{}{"P66", "P75"},
									"type":      "omission",
								},
							},
							"witnesses": []interface{}{
								map[string]interface{}{
									"id":          "w1",
									"siglum":      "P66",
									"name":        "Papyrus 66",
									"date":        "c. 200",
									"description": "Bodmer II papyrus",
								},
							},
							"commentary": "Test commentary",
						},
					},
				},
			},
		},
	}

	ecm := irToECM(corpus)

	// Verify header
	if ecm.Header == nil {
		t.Fatal("expected header to be set")
	}
	if ecm.Header.Title != "Test ECM Corpus" {
		t.Errorf("expected title 'Test ECM Corpus', got %q", ecm.Header.Title)
	}

	// Verify attributes
	if ecm.Book != "John" {
		t.Errorf("expected book 'John', got %q", ecm.Book)
	}
	if ecm.Chapter != "1" {
		t.Errorf("expected chapter '1', got %q", ecm.Chapter)
	}

	// Verify apparatus
	if len(ecm.Apparatus) != 1 {
		t.Fatalf("expected 1 apparatus entry, got %d", len(ecm.Apparatus))
	}

	app := ecm.Apparatus[0]
	if app.ID != "app1" {
		t.Errorf("expected apparatus ID 'app1', got %q", app.ID)
	}
	if app.Verse != "1" {
		t.Errorf("expected verse '1', got %q", app.Verse)
	}
	if app.BaseText != "In the beginning was the Word" {
		t.Errorf("expected base text 'In the beginning was the Word', got %q", app.BaseText)
	}

	// Verify variants
	if len(app.Variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(app.Variants))
	}
	if app.Variants[0].ID != "v1" {
		t.Errorf("expected variant ID 'v1', got %q", app.Variants[0].ID)
	}

	// Verify witnesses
	if len(app.Witnesses) != 1 {
		t.Fatalf("expected 1 witness, got %d", len(app.Witnesses))
	}
	if app.Witnesses[0].Siglum != "P66" {
		t.Errorf("expected witness siglum 'P66', got %q", app.Witnesses[0].Siglum)
	}

	// Verify commentary
	if app.Commentary != "Test commentary" {
		t.Errorf("expected commentary 'Test commentary', got %q", app.Commentary)
	}
}

func TestRoundTripECM(t *testing.T) {
	originalXML := `<?xml version="1.0" encoding="UTF-8"?>
<ECM book="John" chapter="1" edition="ECM.2">
  <header>
    <title>Gospel of John</title>
    <publisher>INTF</publisher>
  </header>
  <apparatus id="app1" verse="1" unit="1">
    <baseText>In the beginning was the Word</baseText>
    <variant id="v1" type="omission">
      <reading>In beginning was the Word</reading>
      <witness>P66</witness>
      <witness>P75</witness>
    </variant>
    <witness id="w1">
      <siglum>P66</siglum>
      <name>Papyrus 66</name>
      <date>c. 200</date>
    </witness>
    <commentary>The article is omitted in early papyri</commentary>
  </apparatus>
</ECM>`

	// Parse original
	var ecm1 ECMXML
	if err := xml.Unmarshal([]byte(originalXML), &ecm1); err != nil {
		t.Fatalf("failed to parse original XML: %v", err)
	}

	// Convert to IR
	corpus := ecmToIR(&ecm1)

	// Convert back to ECM
	ecm2 := irToECM(corpus)

	// Verify key fields are preserved
	if ecm2.Book != ecm1.Book {
		t.Errorf("book mismatch: got %q, want %q", ecm2.Book, ecm1.Book)
	}
	if ecm2.Chapter != ecm1.Chapter {
		t.Errorf("chapter mismatch: got %q, want %q", ecm2.Chapter, ecm1.Chapter)
	}
	if len(ecm2.Apparatus) != len(ecm1.Apparatus) {
		t.Errorf("apparatus count mismatch: got %d, want %d", len(ecm2.Apparatus), len(ecm1.Apparatus))
	}

	if len(ecm2.Apparatus) > 0 && len(ecm1.Apparatus) > 0 {
		app1 := ecm1.Apparatus[0]
		app2 := ecm2.Apparatus[0]

		if app2.BaseText != app1.BaseText {
			t.Errorf("base text mismatch: got %q, want %q", app2.BaseText, app1.BaseText)
		}
		if app2.Commentary != app1.Commentary {
			t.Errorf("commentary mismatch: got %q, want %q", app2.Commentary, app1.Commentary)
		}
	}
}

func TestIngestECM(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8"?>
<ECM book="John" chapter="1">
  <apparatus verse="1" unit="1">
    <baseText>Test text</baseText>
  </apparatus>
</ECM>`

	tmpfile, err := os.CreateTemp("", "ecm-ingest-*.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	args := map[string]interface{}{"path": tmpfile.Name()}

	// Note: In a real test, we'd capture stdout and verify the JSON response
	// For now, we just verify the function doesn't panic
	handleIngest(args)
}

func TestExtractIRFromFile(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8"?>
<ECM book="John" chapter="1" edition="ECM.2">
  <header>
    <title>Gospel of John - Chapter 1</title>
    <description>Critical apparatus for John 1</description>
    <publisher>INTF</publisher>
    <rights>CC BY-SA 4.0</rights>
  </header>
  <apparatus id="app1" verse="1" unit="1">
    <baseText>In the beginning was the Word</baseText>
    <variant id="v1" type="omission">
      <reading>In beginning was the Word</reading>
      <witness>P66</witness>
    </variant>
  </apparatus>
  <apparatus id="app2" verse="1" unit="2">
    <baseText>and the Word was with God</baseText>
    <variant id="v2" type="variation">
      <reading>and the Word was God</reading>
      <witness>D</witness>
    </variant>
  </apparatus>
</ECM>`

	tmpfile, err := os.CreateTemp("", "ecm-extract-*.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	args := map[string]interface{}{"artifact_path": tmpfile.Name()}

	// Note: In a real test, we'd capture stdout and parse the JSON response
	handleExtractIR(args)
}

func TestEmitNativeECM(t *testing.T) {
	corpus := &ipc.Corpus{
		ID:         "test-corpus",
		Version:    "1.0",
		ModuleType: "bible",
		Title:      "Test ECM",
		Publisher:  "Test Publisher",
		Attributes: map[string]string{
			"book":    "John",
			"chapter": "1",
			"edition": "ECM.2",
		},
		Documents: []*ipc.Document{
			{
				ID:    "doc1",
				Title: "Verse 1",
				Order: 1,
				Attributes: map[string]string{
					"verse": "1",
					"unit":  "1",
				},
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "block1",
						Sequence: 1,
						Text:     "In the beginning was the Word",
						Attributes: map[string]interface{}{
							"variants": []interface{}{
								map[string]interface{}{
									"id":        "v1",
									"reading":   "In beginning was the Word",
									"witnesses": []interface{}{"P66"},
									"type":      "omission",
								},
							},
						},
					},
				},
			},
		},
	}

	tmpdir, err := os.MkdirTemp("", "ecm-emit-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	// Convert corpus to interface{} as the handler expects
	corpusJSON, err := json.Marshal(corpus)
	if err != nil {
		t.Fatal(err)
	}
	var corpusInterface interface{}
	if err := json.Unmarshal(corpusJSON, &corpusInterface); err != nil {
		t.Fatal(err)
	}

	args := map[string]interface{}{
		"ir":         corpusInterface,
		"output_dir": tmpdir,
	}

	// Note: In a real test, we'd capture stdout and verify the output file
	handleEmitNative(args)

	// Verify output file was created
	outputPath := filepath.Join(tmpdir, "output.ecm.xml")
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("expected output file to be created")
	}
}

func TestVariantConversion(t *testing.T) {
	// Create test data as interface{} like it would come from JSON
	jsonData := []interface{}{
		map[string]interface{}{
			"id":        "v1",
			"reading":   "test reading",
			"witnesses": []interface{}{"P66", "P75"},
			"type":      "omission",
		},
		map[string]interface{}{
			"id":        "v2",
			"reading":   "another reading",
			"witnesses": []interface{}{"Aleph"},
			"type":      "addition",
		},
	}

	// Convert to Variant structs
	converted := jsonToVariants(jsonData)

	if len(converted) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(converted))
	}

	if converted[0].ID != "v1" {
		t.Errorf("variant 0: ID mismatch: got %q, want %q", converted[0].ID, "v1")
	}
	if converted[0].Reading != "test reading" {
		t.Errorf("variant 0: reading mismatch: got %q, want %q", converted[0].Reading, "test reading")
	}
	if len(converted[0].Witnesses) != 2 {
		t.Errorf("variant 0: witness count mismatch: got %d, want 2", len(converted[0].Witnesses))
	}
}

func TestWitnessConversion(t *testing.T) {
	// Create test data as interface{} like it would come from JSON
	jsonData := []interface{}{
		map[string]interface{}{
			"id":          "w1",
			"siglum":      "P66",
			"name":        "Papyrus 66",
			"date":        "c. 200",
			"description": "Bodmer II",
		},
	}

	// Convert to Witness structs
	converted := jsonToWitnesses(jsonData)

	if len(converted) != 1 {
		t.Fatalf("expected 1 witnesses, got %d", len(converted))
	}

	if converted[0].Siglum != "P66" {
		t.Errorf("expected siglum 'P66', got %q", converted[0].Siglum)
	}
	if converted[0].Name != "Papyrus 66" {
		t.Errorf("expected name 'Papyrus 66', got %q", converted[0].Name)
	}
}
