//go:build !sdk

// Plugin format-odf handles Open Document Format Bible files.
// Produces .odt files with styled content.
//
// IR Support:
// - extract-ir: Reads ODF Bible format to IR (L1)
// - emit-native: Converts IR to ODF format (L1)
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	if ext != ".odt" && ext != ".ods" && ext != ".odp" {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not an ODF file",
		})
		return
	}

	// Try to open as ZIP and verify ODF structure
	r, err := zip.OpenReader(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open as ZIP: %v", err),
		})
		return
	}
	defer r.Close()

	hasMimetype := false
	hasContent := false

	for _, f := range r.File {
		if f.Name == "mimetype" {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			if strings.Contains(string(data), "opendocument") {
				hasMimetype = true
			}
		}
		if f.Name == "content.xml" {
			hasContent = true
		}
	}

	if hasMimetype && hasContent {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "ODF",
			Reason:   "Open Document Format detected",
		})
	} else {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "Missing ODF required files",
		})
	}
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
			"format": "ODF",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		ipc.RespondErrorf("failed to open ODF: %v", err)
		return
	}
	defer r.Close()

	var entries []ipc.EnumerateEntry
	for _, f := range r.File {
		entries = append(entries, ipc.EnumerateEntry{
			Path:      f.Name,
			SizeBytes: int64(f.UncompressedSize64),
			IsDir:     f.FileInfo().IsDir(),
			Metadata:  map[string]string{"format": "ODF"},
		})
	}
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: entries,
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
		SourceFormat: "ODF",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L1",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_odf_raw"] = hex.EncodeToString(data)

	// Parse ODF
	r, err := zip.OpenReader(path)
	if err != nil {
		ipc.RespondErrorf("failed to open ODF: %v", err)
		return
	}
	defer r.Close()

	var contentXML string
	for _, f := range r.File {
		if f.Name == "content.xml" {
			rc, _ := f.Open()
			contentData, _ := io.ReadAll(rc)
			rc.Close()
			contentXML = string(contentData)
		}
	}

	// Parse verses from content.xml
	corpus.Documents = parseODFContent(contentXML, artifactID)

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
			SourceFormat: "ODF",
			TargetFormat: "IR",
			LossClass:    "L1",
		},
	})
}

func parseODFContent(content, artifactID string) []*ipc.Document {
	doc := &ipc.Document{
		ID:    artifactID,
		Title: artifactID,
		Order: 1,
	}

	// Extract book title from <text:h> element
	bookPattern := regexp.MustCompile(`<text:h[^>]*text:style-name="BookTitle"[^>]*>([^<]+)</text:h>`)
	if bookMatch := bookPattern.FindStringSubmatch(content); len(bookMatch) > 1 {
		doc.Title = strings.TrimSpace(bookMatch[1])
		doc.ID = strings.TrimSpace(bookMatch[1])
	}

	// Extract chapter number
	chapterPattern := regexp.MustCompile(`<text:h[^>]*text:style-name="ChapterTitle"[^>]*>Chapter\s+(\d+)</text:h>`)
	currentChapter := 1
	if chapterMatch := chapterPattern.FindStringSubmatch(content); len(chapterMatch) > 1 {
		currentChapter, _ = strconv.Atoi(chapterMatch[1])
	}

	// Extract styled verses: <text:p style="Verse"><text:span style="VerseNum">N</text:span> text</text:p>
	versePattern := regexp.MustCompile(`<text:p[^>]*text:style-name="Verse"[^>]*><text:span[^>]*text:style-name="VerseNum"[^>]*>(\d+)</text:span>\s*([^<]+)</text:p>`)
	matches := versePattern.FindAllStringSubmatch(content, -1)

	sequence := 0
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		verse, _ := strconv.Atoi(match[1])
		text := strings.TrimSpace(match[2])
		if text == "" {
			continue
		}

		sequence++
		hash := sha256.Sum256([]byte(text))
		osisID := fmt.Sprintf("%s.%d.%d", doc.ID, currentChapter, verse)

		cb := &ipc.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     text,
			Hash:     hex.EncodeToString(hash[:]),
			Anchors: []*ipc.Anchor{
				{
					ID:       fmt.Sprintf("a-%d-0", sequence),
					Position: 0,
					Spans: []*ipc.Span{
						{
							ID:            fmt.Sprintf("s-%s", osisID),
							Type:          "VERSE",
							StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
							Ref: &ipc.Ref{
								Book:    doc.ID,
								Chapter: currentChapter,
								Verse:   verse,
								OSISID:  osisID,
							},
						},
					},
				},
			},
		}
		doc.ContentBlocks = append(doc.ContentBlocks, cb)
	}

	// Fallback: try simple text:p pattern for non-styled content
	if len(doc.ContentBlocks) == 0 {
		simplePattern := regexp.MustCompile(`<text:p[^>]*>([^<]+)</text:p>`)
		refPattern := regexp.MustCompile(`^(\w+)?\s*(\d+):(\d+)\s+(.+)$`)

		for _, match := range simplePattern.FindAllStringSubmatch(content, -1) {
			line := strings.TrimSpace(match[1])
			if line == "" {
				continue
			}

			if refMatch := refPattern.FindStringSubmatch(line); len(refMatch) > 0 {
				book := doc.ID
				if refMatch[1] != "" {
					book = refMatch[1]
					doc.ID = book
					doc.Title = book
				}
				chapter, _ := strconv.Atoi(refMatch[2])
				verse, _ := strconv.Atoi(refMatch[3])
				text := strings.TrimSpace(refMatch[4])

				sequence++
				hash := sha256.Sum256([]byte(text))
				osisID := fmt.Sprintf("%s.%d.%d", book, chapter, verse)

				cb := &ipc.ContentBlock{
					ID:       fmt.Sprintf("cb-%d", sequence),
					Sequence: sequence,
					Text:     text,
					Hash:     hex.EncodeToString(hash[:]),
					Anchors: []*ipc.Anchor{
						{
							ID:       fmt.Sprintf("a-%d-0", sequence),
							Position: 0,
							Spans: []*ipc.Span{
								{
									ID:            fmt.Sprintf("s-%s", osisID),
									Type:          "VERSE",
									StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
									Ref: &ipc.Ref{
										Book:    book,
										Chapter: chapter,
										Verse:   verse,
										OSISID:  osisID,
									},
								},
							},
						},
					},
				}
				doc.ContentBlocks = append(doc.ContentBlocks, cb)
			}
		}
	}

	return []*ipc.Document{doc}
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

	outputPath := filepath.Join(outputDir, corpus.ID+".odt")

	// Check for raw ODF for round-trip
	if raw, ok := corpus.Attributes["_odf_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0644); err != nil {
				ipc.RespondErrorf("failed to write ODF: %v", err)
				return
			}
			ipc.MustRespond(&ipc.EmitNativeResult{
				OutputPath: outputPath,
				Format:     "ODF",
				LossClass:  "L0",
				LossReport: &ipc.LossReport{
					SourceFormat: "IR",
					TargetFormat: "ODF",
					LossClass:    "L0",
				},
			})
			return
		}
	}

	// Generate ODF from IR
	if err := generateODF(outputPath, &corpus); err != nil {
		ipc.RespondErrorf("failed to generate ODF: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "ODF",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "ODF",
			LossClass:    "L1",
		},
	})
}

func generateODF(outputPath string, corpus *ipc.Corpus) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// mimetype (must be first, uncompressed)
	mimetypeWriter, err := w.CreateHeader(&zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	})
	if err != nil {
		return err
	}
	mimetypeWriter.Write([]byte("application/vnd.oasis.opendocument.text"))

	// META-INF/manifest.xml
	manifestWriter, _ := w.Create("META-INF/manifest.xml")
	manifestWriter.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0">
  <manifest:file-entry manifest:full-path="/" manifest:media-type="application/vnd.oasis.opendocument.text"/>
  <manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/>
  <manifest:file-entry manifest:full-path="styles.xml" manifest:media-type="text/xml"/>
</manifest:manifest>`))

	// styles.xml
	stylesWriter, _ := w.Create("styles.xml")
	stylesWriter.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<office:document-styles xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
  xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0"
  xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0">
  <office:styles>
    <style:style style:name="Title" style:family="paragraph">
      <style:paragraph-properties fo:text-align="center"/>
      <style:text-properties fo:font-size="18pt" fo:font-weight="bold"/>
    </style:style>
    <style:style style:name="Heading" style:family="paragraph">
      <style:text-properties fo:font-size="14pt" fo:font-weight="bold"/>
    </style:style>
    <style:style style:name="VerseNum" style:family="text">
      <style:text-properties fo:font-weight="bold" fo:color="#666666"/>
    </style:style>
  </office:styles>
</office:document-styles>`))

	// content.xml
	var contentBuf bytes.Buffer
	contentBuf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
  xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0"
  xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0">
<office:body>
<office:text>
`)

	// Title
	if corpus.Title != "" {
		contentBuf.WriteString(fmt.Sprintf(`<text:p text:style-name="Title">%s</text:p>
`, escapeXML(corpus.Title)))
	}

	for _, doc := range corpus.Documents {
		contentBuf.WriteString(fmt.Sprintf(`<text:p text:style-name="Heading">%s</text:p>
`, escapeXML(doc.Title)))

		currentChapter := 0
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						if span.Ref.Chapter != currentChapter {
							currentChapter = span.Ref.Chapter
							contentBuf.WriteString(fmt.Sprintf(`<text:p text:style-name="Heading">Chapter %d</text:p>
`, currentChapter))
						}
						contentBuf.WriteString(fmt.Sprintf(`<text:p><text:span text:style-name="VerseNum">%d </text:span>%s</text:p>
`, span.Ref.Verse, escapeXML(cb.Text)))
					}
				}
			}
		}
	}

	contentBuf.WriteString(`</office:text>
</office:body>
</office:document-content>`)

	contentWriter, _ := w.Create("content.xml")
	contentWriter.Write(contentBuf.Bytes())

	return nil
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
