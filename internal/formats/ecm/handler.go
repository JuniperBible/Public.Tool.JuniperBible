// Package ecm provides the embedded handler for ECM (Editio Critica Maior) critical apparatus format.
// ECM is an XML-based format for scholarly editions of the New Testament,
// containing variant readings and manuscript witnesses.
package ecm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// Handler implements the EmbeddedFormatHandler interface for ECM format.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.ecm",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-ecm",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"file"},
			Outputs: []string{"artifact.kind:ecm"},
		},
	}
}

// Register registers this plugin with the embedded registry.
func Register() {
	plugins.RegisterEmbeddedPlugin(&plugins.EmbeddedPlugin{
		Manifest: Manifest(),
		Format:   &Handler{},
	})
}

func init() {
	Register()
}

// Detect implements EmbeddedFormatHandler.Detect.
func (h *Handler) Detect(path string) (*plugins.DetectResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &plugins.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot read file: %v", err)}, nil
	}

	// Check for ECM XML markers
	content := string(data)
	if strings.Contains(content, "<ECM") &&
		(strings.Contains(content, "<apparatus") || strings.Contains(content, "variant")) {
		return &plugins.DetectResult{Detected: true, Format: "ECM"}, nil
	}

	return &plugins.DetectResult{Detected: false, Reason: "not an ECM XML file"}, nil
}

// Ingest implements EmbeddedFormatHandler.Ingest.
func (h *Handler) Ingest(path, outputDir string) (*plugins.IngestResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashStr[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create blob dir: %w", err)
	}

	blobPath := filepath.Join(blobDir, hashStr)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write blob: %w", err)
	}

	return &plugins.IngestResult{
		ArtifactID: hashStr,
		BlobSHA256: hashStr,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format":   "ecm",
			"filename": filepath.Base(path),
		},
	}, nil
}

// Enumerate implements EmbeddedFormatHandler.Enumerate.
func (h *Handler) Enumerate(path string) (*plugins.EnumerateResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	return &plugins.EnumerateResult{
		Entries: []plugins.EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
			},
		},
	}, nil
}

// ExtractIR implements EmbeddedFormatHandler.ExtractIR.
func (h *Handler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var ecm ECMXML
	if err := xml.Unmarshal(data, &ecm); err != nil {
		return nil, fmt.Errorf("failed to parse ECM XML: %w", err)
	}

	corpus := ecmToIR(&ecm)

	// Compute source hash
	hash := sha256.Sum256(data)
	corpus["source_hash"] = hex.EncodeToString(hash[:])
	corpus["source_format"] = "ecm"
	corpus["loss_class"] = "L1"

	irPath := filepath.Join(outputDir, "corpus.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal IR: %w", err)
	}

	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write IR: %w", err)
	}

	return &plugins.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &plugins.LossReportIPC{
			Warnings: []string{"ECM critical apparatus preserved in IR annotations"},
		},
	}, nil
}

// EmitNative implements EmbeddedFormatHandler.EmitNative.
func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	irData, err := os.ReadFile(irPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read IR: %w", err)
	}

	var corpus map[string]interface{}
	if err := json.Unmarshal(irData, &corpus); err != nil {
		return nil, fmt.Errorf("failed to unmarshal IR: %w", err)
	}

	ecm := irToECM(corpus)

	// Marshal to XML
	output, err := xml.MarshalIndent(ecm, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ECM XML: %w", err)
	}

	// Add XML header
	xmlContent := []byte(xml.Header + string(output))

	// Write output file
	outputPath := filepath.Join(outputDir, "output.ecm.xml")
	if err := os.WriteFile(outputPath, xmlContent, 0644); err != nil {
		return nil, fmt.Errorf("failed to write output file: %w", err)
	}

	return &plugins.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "ecm",
		LossClass:  "L1",
		LossReport: &plugins.LossReportIPC{
			Warnings: []string{"ECM critical apparatus reconstructed from IR"},
		},
	}, nil
}

// ECM XML structures
type ECMXML struct {
	XMLName   xml.Name     `xml:"ECM"`
	Book      string       `xml:"book,attr,omitempty"`
	Chapter   string       `xml:"chapter,attr,omitempty"`
	Edition   string       `xml:"edition,attr,omitempty"`
	Header    *ECMHeader   `xml:"header,omitempty"`
	Apparatus []*Apparatus `xml:"apparatus,omitempty"`
}

type ECMHeader struct {
	Title       string `xml:"title,omitempty"`
	Description string `xml:"description,omitempty"`
	Publisher   string `xml:"publisher,omitempty"`
	Rights      string `xml:"rights,omitempty"`
}

type Apparatus struct {
	ID          string        `xml:"id,attr,omitempty"`
	Verse       string        `xml:"verse,attr,omitempty"`
	Unit        string        `xml:"unit,attr,omitempty"`
	BaseText    string        `xml:"baseText,omitempty"`
	Variants    []*Variant    `xml:"variant,omitempty"`
	Witnesses   []*Witness    `xml:"witness,omitempty"`
	Commentary  string        `xml:"commentary,omitempty"`
	Annotations []*Annotation `xml:"annotation,omitempty"`
}

type Variant struct {
	ID        string   `xml:"id,attr,omitempty"`
	Reading   string   `xml:"reading,omitempty"`
	Witnesses []string `xml:"witness,omitempty"`
	Type      string   `xml:"type,attr,omitempty"`
}

type Witness struct {
	ID          string `xml:"id,attr,omitempty"`
	Siglum      string `xml:"siglum,omitempty"`
	Name        string `xml:"name,omitempty"`
	Date        string `xml:"date,omitempty"`
	Description string `xml:"description,omitempty"`
}

type Annotation struct {
	Type  string `xml:"type,attr,omitempty"`
	Value string `xml:",chardata"`
}

func ecmToIR(ecm *ECMXML) map[string]interface{} {
	corpus := map[string]interface{}{
		"id":          "ecm-corpus",
		"version":     "1.0",
		"module_type": "bible",
		"language":    "grc",
		"attributes":  make(map[string]string),
	}

	attrs := make(map[string]string)
	if ecm.Book != "" {
		attrs["book"] = ecm.Book
	}
	if ecm.Chapter != "" {
		attrs["chapter"] = ecm.Chapter
	}
	if ecm.Edition != "" {
		attrs["edition"] = ecm.Edition
	}
	corpus["attributes"] = attrs

	if ecm.Header != nil {
		corpus["title"] = ecm.Header.Title
		corpus["description"] = ecm.Header.Description
		corpus["publisher"] = ecm.Header.Publisher
		corpus["rights"] = ecm.Header.Rights
	}

	// Convert apparatus entries to documents
	var documents []interface{}
	for i, app := range ecm.Apparatus {
		doc := map[string]interface{}{
			"id":    fmt.Sprintf("apparatus-%d", i+1),
			"title": fmt.Sprintf("Verse %s Unit %s", app.Verse, app.Unit),
			"order": i + 1,
		}

		docAttrs := make(map[string]string)
		if app.ID != "" {
			docAttrs["apparatus_id"] = app.ID
		}
		if app.Verse != "" {
			docAttrs["verse"] = app.Verse
		}
		if app.Unit != "" {
			docAttrs["unit"] = app.Unit
		}
		doc["attributes"] = docAttrs

		// Create content block with base text
		block := map[string]interface{}{
			"id":       fmt.Sprintf("block-%d-1", i+1),
			"sequence": 1,
			"text":     app.BaseText,
		}

		blockAttrs := make(map[string]interface{})
		if len(app.Variants) > 0 {
			blockAttrs["variants"] = variantsToJSON(app.Variants)
		}
		if len(app.Witnesses) > 0 {
			blockAttrs["witnesses"] = witnessesToJSON(app.Witnesses)
		}
		if app.Commentary != "" {
			blockAttrs["commentary"] = app.Commentary
		}
		if len(app.Annotations) > 0 {
			blockAttrs["annotations"] = annotationsToJSON(app.Annotations)
		}
		block["attributes"] = blockAttrs

		doc["content_blocks"] = []interface{}{block}
		documents = append(documents, doc)
	}

	corpus["documents"] = documents
	return corpus
}

func variantsToJSON(variants []*Variant) []interface{} {
	result := make([]interface{}, len(variants))
	for i, v := range variants {
		result[i] = map[string]interface{}{
			"id":        v.ID,
			"reading":   v.Reading,
			"witnesses": v.Witnesses,
			"type":      v.Type,
		}
	}
	return result
}

func witnessesToJSON(witnesses []*Witness) []interface{} {
	result := make([]interface{}, len(witnesses))
	for i, w := range witnesses {
		result[i] = map[string]interface{}{
			"id":          w.ID,
			"siglum":      w.Siglum,
			"name":        w.Name,
			"date":        w.Date,
			"description": w.Description,
		}
	}
	return result
}

func annotationsToJSON(annotations []*Annotation) []interface{} {
	result := make([]interface{}, len(annotations))
	for i, a := range annotations {
		result[i] = map[string]interface{}{
			"type":  a.Type,
			"value": a.Value,
		}
	}
	return result
}

func irToECM(corpus map[string]interface{}) *ECMXML {
	ecm := &ECMXML{}

	// Handle both map[string]interface{} and map[string]string
	if attrs, ok := corpus["attributes"].(map[string]interface{}); ok {
		if book, ok := attrs["book"].(string); ok {
			ecm.Book = book
		}
		if chapter, ok := attrs["chapter"].(string); ok {
			ecm.Chapter = chapter
		}
		if edition, ok := attrs["edition"].(string); ok {
			ecm.Edition = edition
		}
	} else if attrs, ok := corpus["attributes"].(map[string]string); ok {
		ecm.Book = attrs["book"]
		ecm.Chapter = attrs["chapter"]
		ecm.Edition = attrs["edition"]
	}

	title, _ := corpus["title"].(string)
	description, _ := corpus["description"].(string)
	publisher, _ := corpus["publisher"].(string)
	rights, _ := corpus["rights"].(string)

	if title != "" || description != "" || publisher != "" || rights != "" {
		ecm.Header = &ECMHeader{
			Title:       title,
			Description: description,
			Publisher:   publisher,
			Rights:      rights,
		}
	}

	// Convert documents back to apparatus entries
	if docs, ok := corpus["documents"].([]interface{}); ok {
		for _, docIface := range docs {
			if doc, ok := docIface.(map[string]interface{}); ok {
				app := &Apparatus{}

				// Handle both map[string]interface{} and map[string]string
				if attrs, ok := doc["attributes"].(map[string]interface{}); ok {
					if id, ok := attrs["apparatus_id"].(string); ok {
						app.ID = id
					}
					if verse, ok := attrs["verse"].(string); ok {
						app.Verse = verse
					}
					if unit, ok := attrs["unit"].(string); ok {
						app.Unit = unit
					}
				} else if attrs, ok := doc["attributes"].(map[string]string); ok {
					app.ID = attrs["apparatus_id"]
					app.Verse = attrs["verse"]
					app.Unit = attrs["unit"]
				}

				if blocks, ok := doc["content_blocks"].([]interface{}); ok && len(blocks) > 0 {
					if block, ok := blocks[0].(map[string]interface{}); ok {
						if text, ok := block["text"].(string); ok {
							app.BaseText = text
						}

						if attrs, ok := block["attributes"].(map[string]interface{}); ok {
							if variantsData, ok := attrs["variants"]; ok {
								app.Variants = jsonToVariants(variantsData)
							}
							if witnessesData, ok := attrs["witnesses"]; ok {
								app.Witnesses = jsonToWitnesses(witnessesData)
							}
							if commentary, ok := attrs["commentary"].(string); ok {
								app.Commentary = commentary
							}
							if annotationsData, ok := attrs["annotations"]; ok {
								app.Annotations = jsonToAnnotations(annotationsData)
							}
						}
					}
				}

				ecm.Apparatus = append(ecm.Apparatus, app)
			}
		}
	}

	return ecm
}

func jsonToVariants(data interface{}) []*Variant {
	var result []*Variant

	if arr, ok := data.([]interface{}); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				variant := &Variant{
					ID:      getString(m, "id"),
					Reading: getString(m, "reading"),
					Type:    getString(m, "type"),
				}
				if witnesses, ok := m["witnesses"].([]interface{}); ok {
					for _, w := range witnesses {
						if s, ok := w.(string); ok {
							variant.Witnesses = append(variant.Witnesses, s)
						}
					}
				}
				result = append(result, variant)
			}
		}
	}

	return result
}

func jsonToWitnesses(data interface{}) []*Witness {
	var result []*Witness

	if arr, ok := data.([]interface{}); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				witness := &Witness{
					ID:          getString(m, "id"),
					Siglum:      getString(m, "siglum"),
					Name:        getString(m, "name"),
					Date:        getString(m, "date"),
					Description: getString(m, "description"),
				}
				result = append(result, witness)
			}
		}
	}

	return result
}

func jsonToAnnotations(data interface{}) []*Annotation {
	var result []*Annotation

	if arr, ok := data.([]interface{}); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				annotation := &Annotation{
					Type:  getString(m, "type"),
					Value: getString(m, "value"),
				}
				result = append(result, annotation)
			}
		}
	}

	return result
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
