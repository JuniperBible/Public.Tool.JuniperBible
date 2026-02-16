//go:build !sdk

// Plugin format-epub handles EPUB Bible format.
// Produces EPUB3 with NCX navigation and spine.
//
// IR Support:
// - extract-ir: Reads EPUB Bible format to IR (L1)
// - emit-native: Converts IR to EPUB format (L1)
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// IPCRequest is the incoming JSON request.
type IPCRequest struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args,omitempty"`
}

// IPCResponse is the outgoing JSON response.
type IPCResponse struct {
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// DetectResult is the result of a detect command.
type DetectResult struct {
	Detected bool   `json:"detected"`
	Format   string `json:"format,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// IngestResult is the result of an ingest command.
type IngestResult struct {
	ArtifactID string            `json:"artifact_id"`
	BlobSHA256 string            `json:"blob_sha256"`
	SizeBytes  int64             `json:"size_bytes"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// EnumerateResult is the result of an enumerate command.
type EnumerateResult struct {
	Entries []EnumerateEntry `json:"entries"`
}

// EnumerateEntry represents a file entry.
type EnumerateEntry struct {
	Path      string            `json:"path"`
	SizeBytes int64             `json:"size_bytes"`
	IsDir     bool              `json:"is_dir"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ExtractIRResult is the result of an extract-ir command.
type ExtractIRResult struct {
	IRPath     string      `json:"ir_path"`
	LossClass  string      `json:"loss_class"`
	LossReport *LossReport `json:"loss_report,omitempty"`
}

// EmitNativeResult is the result of an emit-native command.
type EmitNativeResult struct {
	OutputPath string      `json:"output_path"`
	Format     string      `json:"format"`
	LossClass  string      `json:"loss_class"`
	LossReport *LossReport `json:"loss_report,omitempty"`
}

// LossReport describes any data loss during conversion.
type LossReport struct {
	SourceFormat string        `json:"source_format"`
	TargetFormat string        `json:"target_format"`
	LossClass    string        `json:"loss_class"`
	LostElements []LostElement `json:"lost_elements,omitempty"`
	Warnings     []string      `json:"warnings,omitempty"`
}

// LostElement describes a specific element that was lost.
type LostElement struct {
	Path          string      `json:"path"`
	ElementType   string      `json:"element_type"`
	Reason        string      `json:"reason"`
	OriginalValue interface{} `json:"original_value,omitempty"`
}

// IR Types
type Corpus struct {
	ID            string            `json:"id"`
	Version       string            `json:"version"`
	ModuleType    string            `json:"module_type"`
	Versification string            `json:"versification,omitempty"`
	Language      string            `json:"language,omitempty"`
	Title         string            `json:"title,omitempty"`
	Description   string            `json:"description,omitempty"`
	Publisher     string            `json:"publisher,omitempty"`
	Rights        string            `json:"rights,omitempty"`
	SourceFormat  string            `json:"source_format,omitempty"`
	Documents     []*Document       `json:"documents,omitempty"`
	SourceHash    string            `json:"source_hash,omitempty"`
	LossClass     string            `json:"loss_class,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type Document struct {
	ID            string            `json:"id"`
	Title         string            `json:"title,omitempty"`
	Order         int               `json:"order"`
	ContentBlocks []*ContentBlock   `json:"content_blocks,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type ContentBlock struct {
	ID         string                 `json:"id"`
	Sequence   int                    `json:"sequence"`
	Text       string                 `json:"text"`
	Tokens     []*Token               `json:"tokens,omitempty"`
	Anchors    []*Anchor              `json:"anchors,omitempty"`
	Hash       string                 `json:"hash,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type Token struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Text     string `json:"text"`
	StartPos int    `json:"start_pos"`
	EndPos   int    `json:"end_pos"`
}

type Anchor struct {
	ID       string  `json:"id"`
	Position int     `json:"position"`
	Spans    []*Span `json:"spans,omitempty"`
}

type Span struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	StartAnchorID string                 `json:"start_anchor_id"`
	EndAnchorID   string                 `json:"end_anchor_id,omitempty"`
	Ref           *Ref                   `json:"ref,omitempty"`
	Attributes    map[string]interface{} `json:"attributes,omitempty"`
}

type Ref struct {
	Book     string `json:"book"`
	Chapter  int    `json:"chapter,omitempty"`
	Verse    int    `json:"verse,omitempty"`
	VerseEnd int    `json:"verse_end,omitempty"`
	SubVerse string `json:"sub_verse,omitempty"`
	OSISID   string `json:"osis_id,omitempty"`
}

// EPUB XML types
type Container struct {
	XMLName   xml.Name   `xml:"container"`
	Rootfiles []Rootfile `xml:"rootfiles>rootfile"`
}

type Rootfile struct {
	FullPath  string `xml:"full-path,attr"`
	MediaType string `xml:"media-type,attr"`
}

type Package struct {
	XMLName  xml.Name `xml:"package"`
	Metadata Metadata `xml:"metadata"`
	Manifest Manifest `xml:"manifest"`
	Spine    Spine    `xml:"spine"`
}

type Metadata struct {
	Title    string `xml:"title"`
	Creator  string `xml:"creator,omitempty"`
	Language string `xml:"language"`
}

type Manifest struct {
	Items []Item `xml:"item"`
}

type Item struct {
	ID        string `xml:"id,attr"`
	Href      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
}

type Spine struct {
	Itemrefs []Itemref `xml:"itemref"`
}

type Itemref struct {
	Idref string `xml:"idref,attr"`
}

func main() {
	var req IPCRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		respondError(fmt.Sprintf("failed to decode request: %v", err))
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
		respondError(fmt.Sprintf("unknown command: %s", req.Command))
	}
}

func handleDetect(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".epub" {
		respond(&DetectResult{
			Detected: false,
			Reason:   "not an .epub file",
		})
		return
	}

	// Try to open as ZIP and verify EPUB structure
	r, err := zip.OpenReader(path)
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open as ZIP: %v", err),
		})
		return
	}
	defer r.Close()

	hasMimetype := false
	hasContainer := false

	for _, f := range r.File {
		if f.Name == "mimetype" {
			hasMimetype = true
		}
		if f.Name == "META-INF/container.xml" {
			hasContainer = true
		}
	}

	if hasMimetype && hasContainer {
		respond(&DetectResult{
			Detected: true,
			Format:   "EPUB",
			Reason:   "Valid EPUB structure detected",
		})
	} else {
		respond(&DetectResult{
			Detected: false,
			Reason:   "Missing EPUB required files",
		})
	}
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		respondError(fmt.Sprintf("failed to create blob dir: %v", err))
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0600); err != nil {
		respondError(fmt.Sprintf("failed to write blob: %v", err))
		return
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	respond(&IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "EPUB",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to open EPUB: %v", err))
		return
	}
	defer r.Close()

	var entries []EnumerateEntry
	for _, f := range r.File {
		entries = append(entries, EnumerateEntry{
			Path:      f.Name,
			SizeBytes: int64(f.UncompressedSize64),
			IsDir:     f.FileInfo().IsDir(),
			Metadata:  map[string]string{"format": "EPUB"},
		})
	}

	respond(&EnumerateResult{
		Entries: entries,
	})
}

func handleExtractIR(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "EPUB",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L1",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_epub_raw"] = hex.EncodeToString(data)

	// Parse EPUB
	r, err := zip.OpenReader(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to open EPUB: %v", err))
		return
	}
	defer r.Close()

	// Find and parse content.opf
	var opfPath string
	for _, f := range r.File {
		if f.Name == "META-INF/container.xml" {
			rc, _ := f.Open()
			containerData, _ := io.ReadAll(rc)
			rc.Close()

			var container Container
			xml.Unmarshal(containerData, &container)
			if len(container.Rootfiles) > 0 {
				opfPath = container.Rootfiles[0].FullPath
			}
		}
	}

	if opfPath != "" {
		for _, f := range r.File {
			if f.Name == opfPath {
				rc, _ := f.Open()
				opfData, _ := io.ReadAll(rc)
				rc.Close()

				var pkg Package
				xml.Unmarshal(opfData, &pkg)
				corpus.Title = pkg.Metadata.Title
				corpus.Language = pkg.Metadata.Language
			}
		}
	}

	// Parse XHTML content files
	corpus.Documents = parseEPUBContent(r, opfPath)

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		respondError(fmt.Sprintf("failed to serialize IR: %v", err))
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0600); err != nil {
		respondError(fmt.Sprintf("failed to write IR: %v", err))
		return
	}

	respond(&ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &LossReport{
			SourceFormat: "EPUB",
			TargetFormat: "IR",
			LossClass:    "L1",
		},
	})
}

func parseEPUBContent(r *zip.ReadCloser, opfPath string) []*Document {
	var docs []*Document
	opfDir := filepath.Dir(opfPath)

	versePattern := regexp.MustCompile(`<span[^>]*class="verse"[^>]*>(\d+)</span>\s*([^<]+)`)
	versePattern2 := regexp.MustCompile(`<span[^>]*data-verse="(\d+)"[^>]*>([^<]+)</span>`)

	docOrder := 0
	for _, f := range r.File {
		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext != ".xhtml" && ext != ".html" && ext != ".htm" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		content, _ := io.ReadAll(rc)
		rc.Close()

		contentStr := string(content)

		// Try to extract verses
		matches := versePattern.FindAllStringSubmatch(contentStr, -1)
		if len(matches) == 0 {
			matches = versePattern2.FindAllStringSubmatch(contentStr, -1)
		}

		if len(matches) == 0 {
			continue
		}

		docOrder++
		relPath := strings.TrimPrefix(f.Name, opfDir+"/")
		docID := strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))

		doc := &Document{
			ID:    docID,
			Title: docID,
			Order: docOrder,
		}

		sequence := 0
		for _, match := range matches {
			verse, _ := strconv.Atoi(match[1])
			text := strings.TrimSpace(match[2])

			if text == "" {
				continue
			}

			sequence++
			hash := sha256.Sum256([]byte(text))
			osisID := fmt.Sprintf("%s.1.%d", docID, verse)

			cb := &ContentBlock{
				ID:       fmt.Sprintf("cb-%d", sequence),
				Sequence: sequence,
				Text:     text,
				Hash:     hex.EncodeToString(hash[:]),
				Anchors: []*Anchor{
					{
						ID:       fmt.Sprintf("a-%d-0", sequence),
						Position: 0,
						Spans: []*Span{
							{
								ID:            fmt.Sprintf("s-%s", osisID),
								Type:          "VERSE",
								StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
								Ref: &Ref{
									Book:    docID,
									Chapter: 1,
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

		docs = append(docs, doc)
	}

	return docs
}

func handleEmitNative(args map[string]interface{}) {
	irPath, ok := args["ir_path"].(string)
	if !ok {
		respondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(irPath)
	if err != nil {
		respondError(fmt.Sprintf("failed to read IR file: %v", err))
		return
	}

	var corpus Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		respondError(fmt.Sprintf("failed to parse IR: %v", err))
		return
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".epub")

	// Check for raw EPUB for round-trip
	if raw, ok := corpus.Attributes["_epub_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
				respondError(fmt.Sprintf("failed to write EPUB: %v", err))
				return
			}

			respond(&EmitNativeResult{
				OutputPath: outputPath,
				Format:     "EPUB",
				LossClass:  "L0",
				LossReport: &LossReport{
					SourceFormat: "IR",
					TargetFormat: "EPUB",
					LossClass:    "L0",
				},
			})
			return
		}
	}

	// Generate EPUB from IR
	if err := generateEPUB(outputPath, &corpus); err != nil {
		respondError(fmt.Sprintf("failed to generate EPUB: %v", err))
		return
	}

	respond(&EmitNativeResult{
		OutputPath: outputPath,
		Format:     "EPUB",
		LossClass:  "L1",
		LossReport: &LossReport{
			SourceFormat: "IR",
			TargetFormat: "EPUB",
			LossClass:    "L1",
		},
	})
}

func generateEPUB(outputPath string, corpus *Corpus) error {
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
	mimetypeWriter.Write([]byte("application/epub+zip"))

	// META-INF/container.xml
	containerWriter, _ := w.Create("META-INF/container.xml")
	containerWriter.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`))

	// Collect document info for manifest and spine
	var manifestItems []string
	var spineItems []string

	// Generate XHTML for each document
	for i, doc := range corpus.Documents {
		filename := fmt.Sprintf("chapter%d.xhtml", i+1)
		manifestItems = append(manifestItems, fmt.Sprintf(`    <item id="chapter%d" href="%s" media-type="application/xhtml+xml"/>`, i+1, filename))
		spineItems = append(spineItems, fmt.Sprintf(`    <itemref idref="chapter%d"/>`, i+1))

		chapterWriter, _ := w.Create("OEBPS/" + filename)
		chapterWriter.Write([]byte(generateXHTML(doc, corpus.Language)))
	}

	// OEBPS/content.opf
	lang := corpus.Language
	if lang == "" {
		lang = "en"
	}

	opfContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:uuid:%s</dc:identifier>
    <dc:title>%s</dc:title>
    <dc:language>%s</dc:language>
    <meta property="dcterms:modified">2024-01-01T00:00:00Z</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
%s
  </manifest>
  <spine>
%s
  </spine>
</package>`,
		corpus.ID,
		escapeXML(corpus.Title),
		lang,
		strings.Join(manifestItems, "\n"),
		strings.Join(spineItems, "\n"))

	opfWriter, _ := w.Create("OEBPS/content.opf")
	opfWriter.Write([]byte(opfContent))

	// OEBPS/nav.xhtml (navigation document)
	var navItems bytes.Buffer
	for i, doc := range corpus.Documents {
		navItems.WriteString(fmt.Sprintf(`      <li><a href="chapter%d.xhtml">%s</a></li>
`, i+1, escapeXML(doc.Title)))
	}

	navContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head>
  <title>%s - Navigation</title>
</head>
<body>
  <nav epub:type="toc">
    <h1>Table of Contents</h1>
    <ol>
%s    </ol>
  </nav>
</body>
</html>`, escapeXML(corpus.Title), navItems.String())

	navWriter, _ := w.Create("OEBPS/nav.xhtml")
	navWriter.Write([]byte(navContent))

	return nil
}

func generateXHTML(doc *Document, language string) string {
	if language == "" {
		language = "en"
	}

	var content bytes.Buffer
	content.WriteString(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" lang="%s">
<head>
  <title>%s</title>
  <style>
    body { font-family: Georgia, serif; margin: 2em; }
    .verse { margin: 0.3em 0; }
    .verse-num { font-weight: bold; color: #666; margin-right: 0.3em; }
  </style>
</head>
<body>
<h1>%s</h1>
`, language, escapeXML(doc.Title), escapeXML(doc.Title)))

	currentChapter := 0
	for _, cb := range doc.ContentBlocks {
		for _, anchor := range cb.Anchors {
			for _, span := range anchor.Spans {
				if span.Ref != nil && span.Type == "VERSE" {
					if span.Ref.Chapter != currentChapter {
						if currentChapter > 0 {
							content.WriteString("</section>\n")
						}
						currentChapter = span.Ref.Chapter
						content.WriteString(fmt.Sprintf("<section class=\"chapter\">\n<h2>Chapter %d</h2>\n", currentChapter))
					}
					content.WriteString(fmt.Sprintf("<p class=\"verse\" data-verse=\"%d\"><span class=\"verse-num\">%d</span>%s</p>\n",
						span.Ref.Verse, span.Ref.Verse, escapeXML(cb.Text)))
				}
			}
		}
	}
	if currentChapter > 0 {
		content.WriteString("</section>\n")
	}

	content.WriteString("</body>\n</html>\n")
	return content.String()
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func respond(result interface{}) {
	resp := IPCResponse{
		Status: "ok",
		Result: result,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
}

func respondError(msg string) {
	resp := IPCResponse{
		Status: "error",
		Error:  msg,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
	os.Exit(1)
}
