
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
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

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
}

type Metadata struct {
	Title    string `xml:"title"`
	Language string `xml:"language"`
}

func runSDK() {
	if err := format.Run(&format.Config{
		Name:       "EPUB",
		Extensions: []string{".epub"},
		Detect:     detectEPUB,
		Parse:      parseEPUB,
		Emit:       emitEPUB,
		Enumerate:  enumerateEPUB,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectEPUB(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".epub" {
		return &ipc.DetectResult{Detected: false, Reason: "not an .epub file"}, nil
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot open as ZIP: %v", err)}, nil
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
		return &ipc.DetectResult{Detected: true, Format: "EPUB", Reason: "Valid EPUB structure detected"}, nil
	}

	return &ipc.DetectResult{Detected: false, Reason: "Missing EPUB required files"}, nil
}

func enumerateEPUB(path string) (*ipc.EnumerateResult, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open EPUB: %w", err)
	}
	defer r.Close()

	var entries []ipc.EnumerateEntry
	for _, f := range r.File {
		entries = append(entries, ipc.EnumerateEntry{
			Path:      f.Name,
			SizeBytes: int64(f.UncompressedSize64),
			IsDir:     f.FileInfo().IsDir(),
			Metadata:  map[string]string{"format": "EPUB"},
		})
	}

	return &ipc.EnumerateResult{Entries: entries}, nil
}

func parseEPUB(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "EPUB"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L1"
	corpus.Attributes = map[string]string{"_epub_raw": hex.EncodeToString(data)}

	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open EPUB: %w", err)
	}
	defer r.Close()

	// Find OPF path
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

	corpus.Documents = parseEPUBContent(r, opfPath)

	return corpus, nil
}

func parseEPUBContent(r *zip.ReadCloser, opfPath string) []*ir.Document {
	var docs []*ir.Document
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

		doc := ir.NewDocument(docID, docID, docOrder)

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

			cb := &ir.ContentBlock{
				ID:       fmt.Sprintf("cb-%d", sequence),
				Sequence: sequence,
				Text:     text,
				Hash:     hex.EncodeToString(hash[:]),
				Anchors: []*ir.Anchor{{
					ID:       fmt.Sprintf("a-%d-0", sequence),
					Position: 0,
					Spans: []*ir.Span{{
						ID:            fmt.Sprintf("s-%s", osisID),
						Type:          "VERSE",
						StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
						Ref:           &ir.Ref{Book: docID, Chapter: 1, Verse: verse, OSISID: osisID},
					}},
				}},
			}
			doc.ContentBlocks = append(doc.ContentBlocks, cb)
		}

		docs = append(docs, doc)
	}

	return docs
}

func emitEPUB(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".epub")

	// Check for raw EPUB for round-trip
	if raw, ok := corpus.Attributes["_epub_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
				return "", fmt.Errorf("failed to write EPUB: %w", err)
			}
			return outputPath, nil
		}
	}

	// Generate EPUB from IR
	if err := generateEPUB(outputPath, corpus); err != nil {
		return "", fmt.Errorf("failed to generate EPUB: %w", err)
	}

	return outputPath, nil
}

func generateEPUB(outputPath string, corpus *ir.Corpus) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// mimetype (must be first, uncompressed)
	mimetypeWriter, _ := w.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	mimetypeWriter.Write([]byte("application/epub+zip"))

	// META-INF/container.xml
	containerWriter, _ := w.Create("META-INF/container.xml")
	containerWriter.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`))

	var manifestItems []string
	var spineItems []string

	for i, doc := range corpus.Documents {
		filename := fmt.Sprintf("chapter%d.xhtml", i+1)
		manifestItems = append(manifestItems, fmt.Sprintf(`    <item id="chapter%d" href="%s" media-type="application/xhtml+xml"/>`, i+1, filename))
		spineItems = append(spineItems, fmt.Sprintf(`    <itemref idref="chapter%d"/>`, i+1))

		chapterWriter, _ := w.Create("OEBPS/" + filename)
		chapterWriter.Write([]byte(generateXHTML(doc, corpus.Language)))
	}

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
		corpus.ID, escapeXML(corpus.Title), lang,
		strings.Join(manifestItems, "\n"), strings.Join(spineItems, "\n"))

	opfWriter, _ := w.Create("OEBPS/content.opf")
	opfWriter.Write([]byte(opfContent))

	var navItems bytes.Buffer
	for i, doc := range corpus.Documents {
		navItems.WriteString(fmt.Sprintf(`      <li><a href="chapter%d.xhtml">%s</a></li>
`, i+1, escapeXML(doc.Title)))
	}

	navContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>%s</title></head>
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

func generateXHTML(doc *ir.Document, language string) string {
	if language == "" {
		language = "en"
	}

	var content bytes.Buffer
	content.WriteString(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" lang="%s">
<head>
  <title>%s</title>
  <style>body { font-family: Georgia, serif; margin: 2em; }</style>
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
						content.WriteString(fmt.Sprintf("<section><h2>Chapter %d</h2>\n", currentChapter))
					}
					content.WriteString(fmt.Sprintf("<p data-verse=\"%d\"><b>%d</b> %s</p>\n",
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
