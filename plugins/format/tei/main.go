//go:build !sdk

// Plugin format-tei handles TEI (Text Encoding Initiative) XML format.
// TEI is a scholarly XML format for encoding texts in the humanities.
//
// IR Support:
// - extract-ir: Reads TEI XML to IR (L1)
// - emit-native: Converts IR to TEI XML (L1)
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

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
		ipc.RespondError("path argument required")
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	// TEI commonly uses .xml or .tei extension
	if ext == ".tei" {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "TEI",
			Reason:   "TEI file extension detected",
		})
		return
	}

	// Check for TEI XML structure
	data, err := os.ReadFile(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not a TEI file",
		})
		return
	}

	content := string(data)
	// TEI documents have <TEI> or <TEI.2> root element and teiHeader
	if strings.Contains(content, "<TEI") && strings.Contains(content, "teiHeader") {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "TEI",
			Reason:   "TEI XML structure detected",
		})
		return
	}

	// Check for TEI namespace
	if strings.Contains(content, "http://www.tei-c.org/ns/") {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "TEI",
			Reason:   "TEI namespace detected",
		})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{
		Detected: false,
		Reason:   "no TEI structure found",
	})
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		ipc.RespondErrorf("failed to create blob dir: %v", err)
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		ipc.RespondErrorf("failed to write blob: %v", err)
		return
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "TEI",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		ipc.RespondErrorf("failed to stat: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: []ipc.EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
				Metadata:  map[string]string{"format": "TEI"},
			},
		},
	})
}

func handleExtractIR(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ipc.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "TEI",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L1",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_tei_raw"] = hex.EncodeToString(data)

	// Extract metadata and content from TEI XML
	content := string(data)
	extractTEIMetadata(content, corpus)
	corpus.Documents = extractTEIContent(content, artifactID)

	// If no documents extracted, create minimal structure
	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ipc.Document{
			{
				ID:    artifactID,
				Title: corpus.Title,
				Order: 1,
			},
		}
	}

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to serialize IR: %v", err)
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}
	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "TEI",
			TargetFormat: "IR",
			LossClass:    "L1",
			Warnings:     []string{"TEI scholarly format - semantically lossless"},
		},
	})
}

func extractTEIMetadata(content string, corpus *ipc.Corpus) {
	// Extract title
	titlePattern := regexp.MustCompile(`<title[^>]*>([^<]+)</title>`)
	if matches := titlePattern.FindStringSubmatch(content); len(matches) > 1 {
		corpus.Title = strings.TrimSpace(matches[1])
	}

	// Extract language
	langPattern := regexp.MustCompile(`<language[^>]*ident="([^"]+)"`)
	if matches := langPattern.FindStringSubmatch(content); len(matches) > 1 {
		corpus.Language = matches[1]
	}

	// Extract publisher
	pubPattern := regexp.MustCompile(`<publisher>([^<]+)</publisher>`)
	if matches := pubPattern.FindStringSubmatch(content); len(matches) > 1 {
		corpus.Publisher = strings.TrimSpace(matches[1])
	}
}

func extractTEIContent(content, artifactID string) []*ipc.Document {
	var documents []*ipc.Document

	// Extract div elements (books/chapters)
	divPattern := regexp.MustCompile(`<div[^>]*type="([^"]*)"[^>]*n="([^"]*)"[^>]*>([\s\S]*?)</div>`)
	divMatches := divPattern.FindAllStringSubmatch(content, -1)

	sequence := 0
	docOrder := 0

	for _, match := range divMatches {
		if len(match) < 4 {
			continue
		}

		divType := match[1]
		divN := match[2]
		divContent := match[3]

		// Skip nested divs if type is "book" or "chapter"
		if divType == "book" || divType == "chapter" {
			docOrder++
			doc := &ipc.Document{
				ID:    divN,
				Title: divN,
				Order: docOrder,
			}

			// Extract verses/segments from div content
			segPattern := regexp.MustCompile(`<seg[^>]*n="([^"]*)"[^>]*>([^<]*)</seg>`)
			versePattern := regexp.MustCompile(`<ab[^>]*n="([^"]*)"[^>]*>([^<]*)</ab>`)

			for _, segMatch := range segPattern.FindAllStringSubmatch(divContent, -1) {
				if len(segMatch) >= 3 {
					sequence++
					text := strings.TrimSpace(segMatch[2])
					if len(text) > 0 {
						hash := sha256.Sum256([]byte(text))
						doc.ContentBlocks = append(doc.ContentBlocks, &ipc.ContentBlock{
							ID:       fmt.Sprintf("cb-%d", sequence),
							Sequence: sequence,
							Text:     text,
							Hash:     hex.EncodeToString(hash[:]),
						})
					}
				}
			}

			for _, verseMatch := range versePattern.FindAllStringSubmatch(divContent, -1) {
				if len(verseMatch) >= 3 {
					sequence++
					text := strings.TrimSpace(verseMatch[2])
					if len(text) > 0 {
						hash := sha256.Sum256([]byte(text))
						doc.ContentBlocks = append(doc.ContentBlocks, &ipc.ContentBlock{
							ID:       fmt.Sprintf("cb-%d", sequence),
							Sequence: sequence,
							Text:     text,
							Hash:     hex.EncodeToString(hash[:]),
						})
					}
				}
			}

			if len(doc.ContentBlocks) > 0 {
				documents = append(documents, doc)
			}
		}
	}

	// If no structured content, extract paragraphs
	if len(documents) == 0 {
		doc := &ipc.Document{
			ID:    artifactID,
			Title: artifactID,
			Order: 1,
		}

		pPattern := regexp.MustCompile(`<p[^>]*>([^<]+)</p>`)
		for _, match := range pPattern.FindAllStringSubmatch(content, -1) {
			if len(match) >= 2 {
				sequence++
				text := strings.TrimSpace(match[1])
				if len(text) > 5 {
					hash := sha256.Sum256([]byte(text))
					doc.ContentBlocks = append(doc.ContentBlocks, &ipc.ContentBlock{
						ID:       fmt.Sprintf("cb-%d", sequence),
						Sequence: sequence,
						Text:     text,
						Hash:     hex.EncodeToString(hash[:]),
					})
				}
			}
		}

		if len(doc.ContentBlocks) > 0 {
			documents = []*ipc.Document{doc}
		}
	}

	return documents
}

func handleEmitNative(args map[string]interface{}) {
	irPath, ok := args["ir_path"].(string)
	if !ok {
		ipc.RespondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR file: %v", err)
		return
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		ipc.RespondErrorf("failed to parse IR: %v", err)
		return
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".tei.xml")

	// Check for raw TEI for round-trip
	if raw, ok := corpus.Attributes["_tei_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0644); err != nil {
				ipc.RespondErrorf("failed to write TEI: %v", err)
				return
			}
			ipc.MustRespond(&ipc.EmitNativeResult{
				OutputPath: outputPath,
				Format:     "TEI",
				LossClass:  "L0",
				LossReport: &ipc.LossReport{
					SourceFormat: "IR",
					TargetFormat: "TEI",
					LossClass:    "L0",
				},
			})
			return
		}
	}

	// Generate TEI XML from IR
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString("\n")
	buf.WriteString(`<TEI xmlns="http://www.tei-c.org/ns/1.0">`)
	buf.WriteString("\n")
	buf.WriteString("  <teiHeader>\n")
	buf.WriteString("    <fileDesc>\n")
	buf.WriteString("      <titleStmt>\n")
	fmt.Fprintf(&buf, "        <title>%s</title>\n", escapeXML(corpus.Title))
	buf.WriteString("      </titleStmt>\n")
	buf.WriteString("      <publicationStmt>\n")
	if corpus.Publisher != "" {
		fmt.Fprintf(&buf, "        <publisher>%s</publisher>\n", escapeXML(corpus.Publisher))
	} else {
		buf.WriteString("        <p>Generated from IR</p>\n")
	}
	buf.WriteString("      </publicationStmt>\n")
	buf.WriteString("      <sourceDesc>\n")
	buf.WriteString("        <p>Converted from Capsule IR</p>\n")
	buf.WriteString("      </sourceDesc>\n")
	buf.WriteString("    </fileDesc>\n")
	if corpus.Language != "" {
		buf.WriteString("    <profileDesc>\n")
		buf.WriteString("      <langUsage>\n")
		fmt.Fprintf(&buf, "        <language ident=\"%s\"/>\n", corpus.Language)
		buf.WriteString("      </langUsage>\n")
		buf.WriteString("    </profileDesc>\n")
	}
	buf.WriteString("  </teiHeader>\n")
	buf.WriteString("  <text>\n")
	buf.WriteString("    <body>\n")

	for _, doc := range corpus.Documents {
		fmt.Fprintf(&buf, "      <div type=\"book\" n=\"%s\">\n", escapeXML(doc.ID))
		for _, cb := range doc.ContentBlocks {
			fmt.Fprintf(&buf, "        <ab n=\"%d\">%s</ab>\n", cb.Sequence, escapeXML(cb.Text))
		}
		buf.WriteString("      </div>\n")
	}

	buf.WriteString("    </body>\n")
	buf.WriteString("  </text>\n")
	buf.WriteString("</TEI>\n")

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		ipc.RespondErrorf("failed to write TEI: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "TEI",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "TEI",
			LossClass:    "L1",
			Warnings:     []string{"Generated TEI XML - semantically complete"},
		},
	})
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
