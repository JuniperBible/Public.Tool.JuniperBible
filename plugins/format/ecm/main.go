//go:build !sdk

// Plugin format-ecm handles ECM (Editio Critica Maior) critical apparatus format.
// ECM is an XML-based format for scholarly editions of the New Testament,
// containing variant readings and manuscript witnesses.
//
// IR Support:
// - extract-ir: Reads ECM XML to IR with apparatus annotations (L1)
// - emit-native: Converts IR to ECM XML with critical apparatus (L1)
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

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

func main() {
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorf("failed to decode request: %v", err)
		return
	}

	switch req.Command {
	case "detect":
		handleDetect(req.Args)
	case "ingest":
		handleIngest(req.Args)
	case "enumerate":
		handleEnumerate(req.Args)
	case "extract-ir":
		handleExtractIR(req.Args)
	case "emit-native":
		handleEmitNative(req.Args)
	default:
		ipc.RespondErrorf("unknown command: %s", req.Command)
	}
}

func handleDetect(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("missing or invalid path argument")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.MustRespond(ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot read file: %v", err)})
		return
	}

	// Check for ECM XML markers
	content := string(data)
	if strings.Contains(content, "<ECM") &&
		(strings.Contains(content, "<apparatus") || strings.Contains(content, "variant")) {
		ipc.MustRespond(ipc.DetectResult{Detected: true, Format: "ECM"})
		return
	}

	ipc.MustRespond(ipc.DetectResult{Detected: false, Reason: "not an ECM XML file"})
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("missing or invalid path argument")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	result := ipc.IngestResult{
		ArtifactID: hashStr,
		BlobSHA256: hashStr,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format":   "ecm",
			"filename": filepath.Base(path),
		},
	}

	ipc.MustRespond(result)
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["artifact_path"].(string)
	if !ok {
		ipc.RespondError("missing or invalid artifact_path argument")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		ipc.RespondErrorf("failed to stat file: %v", err)
		return
	}

	result := ipc.EnumerateResult{
		Entries: []ipc.EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
				ModTime:   info.ModTime().Format("2006-01-02T15:04:05Z"),
			},
		},
	}

	ipc.MustRespond(result)
}

func handleExtractIR(args map[string]interface{}) {
	artifactPath, ok := args["artifact_path"].(string)
	if !ok {
		ipc.RespondError("missing or invalid artifact_path argument")
		return
	}

	data, err := os.ReadFile(artifactPath)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	var ecm ECMXML
	if err := xml.Unmarshal(data, &ecm); err != nil {
		ipc.RespondErrorf("failed to parse ECM XML: %v", err)
		return
	}

	corpus := ecmToIR(&ecm)

	// Compute source hash
	hash := sha256.Sum256(data)
	corpus.SourceHash = hex.EncodeToString(hash[:])
	corpus.SourceFormat = "ecm"
	corpus.LossClass = "L1"

	result := ipc.ExtractIRResult{
		IR: corpus,
		LossReport: &ipc.LossReport{
			Warnings: []string{"ECM critical apparatus preserved in IR annotations"},
		},
	}

	ipc.MustRespond(result)
}

func ecmToIR(ecm *ECMXML) *ipc.Corpus {
	corpus := &ipc.Corpus{
		ID:         "ecm-corpus",
		Version:    "1.0",
		ModuleType: "bible",
		Language:   "grc",
		Attributes: make(map[string]string),
	}

	if ecm.Book != "" {
		corpus.Attributes["book"] = ecm.Book
	}
	if ecm.Chapter != "" {
		corpus.Attributes["chapter"] = ecm.Chapter
	}
	if ecm.Edition != "" {
		corpus.Attributes["edition"] = ecm.Edition
	}

	if ecm.Header != nil {
		corpus.Title = ecm.Header.Title
		corpus.Description = ecm.Header.Description
		corpus.Publisher = ecm.Header.Publisher
		corpus.Rights = ecm.Header.Rights
	}

	// Convert apparatus entries to documents
	for i, app := range ecm.Apparatus {
		doc := &ipc.Document{
			ID:         fmt.Sprintf("apparatus-%d", i+1),
			Title:      fmt.Sprintf("Verse %s Unit %s", app.Verse, app.Unit),
			Order:      i + 1,
			Attributes: make(map[string]string),
		}

		if app.ID != "" {
			doc.Attributes["apparatus_id"] = app.ID
		}
		if app.Verse != "" {
			doc.Attributes["verse"] = app.Verse
		}
		if app.Unit != "" {
			doc.Attributes["unit"] = app.Unit
		}

		// Create content block with base text
		block := &ipc.ContentBlock{
			ID:         fmt.Sprintf("block-%d-1", i+1),
			Sequence:   1,
			Text:       app.BaseText,
			Attributes: make(map[string]interface{}),
		}

		// Add variant readings as annotations
		if len(app.Variants) > 0 {
			block.Attributes["variants"] = variantsToJSON(app.Variants)
		}

		// Add witness information
		if len(app.Witnesses) > 0 {
			block.Attributes["witnesses"] = witnessesToJSON(app.Witnesses)
		}

		// Add commentary
		if app.Commentary != "" {
			block.Attributes["commentary"] = app.Commentary
		}

		// Add annotations
		if len(app.Annotations) > 0 {
			block.Attributes["annotations"] = annotationsToJSON(app.Annotations)
		}

		// Create anchors and spans for variant readings
		anchorPos := 0
		var anchors []*ipc.Anchor
		spanID := 0

		for _, variant := range app.Variants {
			anchor := &ipc.Anchor{
				ID:       fmt.Sprintf("anchor-%d-%d", i+1, anchorPos),
				Position: anchorPos,
			}

			span := &ipc.Span{
				ID:            fmt.Sprintf("span-%d", spanID),
				Type:          "variant",
				StartAnchorID: anchor.ID,
				Attributes: map[string]interface{}{
					"variant_id": variant.ID,
					"reading":    variant.Reading,
					"witnesses":  variant.Witnesses,
				},
			}
			if variant.Type != "" {
				span.Attributes["variant_type"] = variant.Type
			}

			anchor.Spans = []*ipc.Span{span}
			anchors = append(anchors, anchor)
			anchorPos++
			spanID++
		}

		block.Anchors = anchors
		doc.ContentBlocks = []*ipc.ContentBlock{block}
		corpus.Documents = append(corpus.Documents, doc)
	}

	return corpus
}

func variantsToJSON(variants []*Variant) interface{} {
	result := make([]map[string]interface{}, len(variants))
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

func witnessesToJSON(witnesses []*Witness) interface{} {
	result := make([]map[string]interface{}, len(witnesses))
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

func annotationsToJSON(annotations []*Annotation) interface{} {
	result := make([]map[string]interface{}, len(annotations))
	for i, a := range annotations {
		result[i] = map[string]interface{}{
			"type":  a.Type,
			"value": a.Value,
		}
	}
	return result
}

func handleEmitNative(args map[string]interface{}) {
	irData, ok := args["ir"]
	if !ok {
		ipc.RespondError("missing ir argument")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("missing or invalid output_dir argument")
		return
	}

	// Parse IR from JSON
	irBytes, err := json.Marshal(irData)
	if err != nil {
		ipc.RespondErrorf("failed to marshal IR: %v", err)
		return
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(irBytes, &corpus); err != nil {
		ipc.RespondErrorf("failed to unmarshal IR: %v", err)
		return
	}

	ecm := irToECM(&corpus)

	// Marshal to XML
	output, err := xml.MarshalIndent(ecm, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to marshal ECM XML: %v", err)
		return
	}

	// Add XML header
	xmlContent := []byte(xml.Header + string(output))

	// Write output file
	outputPath := filepath.Join(outputDir, "output.ecm.xml")
	if err := os.WriteFile(outputPath, xmlContent, 0644); err != nil {
		ipc.RespondErrorf("failed to write output file: %v", err)
		return
	}

	result := ipc.EmitResult{
		Files: []ipc.EmittedFile{
			{Path: outputPath},
		},
		LossReport: &ipc.LossReport{
			Warnings: []string{"ECM critical apparatus reconstructed from IR"},
		},
	}

	ipc.MustRespond(result)
}

func irToECM(corpus *ipc.Corpus) *ECMXML {
	ecm := &ECMXML{
		Book:    corpus.Attributes["book"],
		Chapter: corpus.Attributes["chapter"],
		Edition: corpus.Attributes["edition"],
	}

	if corpus.Title != "" || corpus.Description != "" || corpus.Publisher != "" || corpus.Rights != "" {
		ecm.Header = &ECMHeader{
			Title:       corpus.Title,
			Description: corpus.Description,
			Publisher:   corpus.Publisher,
			Rights:      corpus.Rights,
		}
	}

	// Convert documents back to apparatus entries
	for _, doc := range corpus.Documents {
		app := &Apparatus{
			ID:    doc.Attributes["apparatus_id"],
			Verse: doc.Attributes["verse"],
			Unit:  doc.Attributes["unit"],
		}

		if len(doc.ContentBlocks) > 0 {
			block := doc.ContentBlocks[0]
			app.BaseText = block.Text

			// Extract variants from attributes
			if variantsData, ok := block.Attributes["variants"]; ok {
				app.Variants = jsonToVariants(variantsData)
			}

			// Extract witnesses from attributes
			if witnessesData, ok := block.Attributes["witnesses"]; ok {
				app.Witnesses = jsonToWitnesses(witnessesData)
			}

			// Extract commentary
			if commentary, ok := block.Attributes["commentary"].(string); ok {
				app.Commentary = commentary
			}

			// Extract annotations
			if annotationsData, ok := block.Attributes["annotations"]; ok {
				app.Annotations = jsonToAnnotations(annotationsData)
			}
		}

		ecm.Apparatus = append(ecm.Apparatus, app)
	}

	return ecm
}

func jsonToVariants(data interface{}) []*Variant {
	var result []*Variant

	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
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

	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
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

	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
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
