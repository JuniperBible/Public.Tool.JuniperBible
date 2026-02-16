//

 Plugin format-dbl handles Digital Bible Library format.
// DBL is a bundle format containing USX files and metadata.
//
// IR Support:
// - extract-ir: Reads DBL bundle to IR (L1)
// - emit-native: Converts IR to DBL bundle (L1)
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

// DBL Metadata structure
type DBLMetadata struct {
	XMLName        xml.Name `xml:"DBLMetadata"`
	ID             string   `xml:"id,attr"`
	Revision       string   `xml:"revision,attr"`
	Identification struct {
		Name        string `xml:"name"`
		NameLocal   string `xml:"nameLocal"`
		Description string `xml:"description"`
	} `xml:"identification"`
	Language struct {
		ISO  string `xml:"iso"`
		Name string `xml:"name"`
	} `xml:"language"`
	Copyright struct {
		Statement string `xml:"statement"`
	} `xml:"copyright"`
}

func runSDK() {
	if err := format.Run(&format.Config{
		Name:       "DBL",
		Extensions: []string{".zip", ".dbl"},
		Detect:     detectDBL,
		Parse:      parseDBL,
		Emit:       emitDBL,
		Enumerate:  enumerateDBL,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectDBL(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".zip" && ext != ".dbl" {
		return &ipc.DetectResult{Detected: false, Reason: "not a .zip or .dbl file"}, nil
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot open as zip: %v", err)}, nil
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "metadata.xml" || strings.HasSuffix(f.Name, "/metadata.xml") {
			return &ipc.DetectResult{Detected: true, Format: "DBL", Reason: "Digital Bible Library bundle detected"}, nil
		}
	}

	return &ipc.DetectResult{Detected: false, Reason: "no metadata.xml found in bundle"}, nil
}

func enumerateDBL(path string) (*ipc.EnumerateResult, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	var entries []ipc.EnumerateEntry
	for _, f := range r.File {
		entries = append(entries, ipc.EnumerateEntry{
			Path:      f.Name,
			SizeBytes: int64(f.UncompressedSize64),
			IsDir:     f.FileInfo().IsDir(),
			Metadata:  map[string]string{"format": "DBL"},
		})
	}

	return &ipc.EnumerateResult{Entries: entries}, nil
}

func parseDBL(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "DBL"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L1"
	corpus.Attributes = map[string]string{"_dbl_raw": hex.EncodeToString(data)}

	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open DBL: %w", err)
	}
	defer r.Close()

	// Parse metadata
	for _, f := range r.File {
		if f.Name == "metadata.xml" || strings.HasSuffix(f.Name, "/metadata.xml") {
			rc, _ := f.Open()
			metaData, _ := io.ReadAll(rc)
			rc.Close()

			var meta DBLMetadata
			if err := xml.Unmarshal(metaData, &meta); err == nil {
				corpus.Title = meta.Identification.Name
				corpus.Description = meta.Identification.Description
				corpus.Language = meta.Language.ISO
				corpus.Rights = meta.Copyright.Statement
			}
		}
	}

	// Parse USX files
	bookOrder := 0
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, ".usx") {
			rc, _ := f.Open()
			usxData, _ := io.ReadAll(rc)
			rc.Close()

			bookOrder++
			doc := parseUSXContent(string(usxData), f.Name, bookOrder)
			if doc != nil && len(doc.ContentBlocks) > 0 {
				corpus.Documents = append(corpus.Documents, doc)
			}
		}
	}

	return corpus, nil
}

func parseUSXContent(content, filename string, order int) *ir.Document {
	bookCode := strings.TrimSuffix(filepath.Base(filename), ".usx")
	bookPattern := regexp.MustCompile(`(?:^\d+)?([A-Z]{2,3})(?:\.usx)?$`)
	if match := bookPattern.FindStringSubmatch(bookCode); len(match) > 1 {
		bookCode = match[1]
	}

	doc := ir.NewDocument(bookCode, bookCode, order)

	versePattern := regexp.MustCompile(`<verse\s+number="(\d+)"[^/]*/?>([^<]*)`)
	chapterPattern := regexp.MustCompile(`<chapter\s+number="(\d+)"`)

	currentChapter := 1
	for _, chMatch := range chapterPattern.FindAllStringSubmatch(content, -1) {
		if len(chMatch) > 1 {
			currentChapter, _ = strconv.Atoi(chMatch[1])
		}
	}

	sequence := 0
	matches := versePattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		verse, _ := strconv.Atoi(match[1])
		text := ""
		if len(match) > 2 {
			text = strings.TrimSpace(match[2])
		}

		if text == "" {
			continue
		}

		sequence++
		hash := sha256.Sum256([]byte(text))
		osisID := fmt.Sprintf("%s.%d.%d", bookCode, currentChapter, verse)

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
					Ref:           &ir.Ref{Book: bookCode, Chapter: currentChapter, Verse: verse, OSISID: osisID},
				}},
			}},
		}
		doc.ContentBlocks = append(doc.ContentBlocks, cb)
	}

	return doc
}

func emitDBL(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".zip")

	// Check for raw DBL for round-trip
	if raw, ok := corpus.Attributes["_dbl_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
				return "", fmt.Errorf("failed to write DBL: %w", err)
			}
			return outputPath, nil
		}
	}

	// Generate DBL from IR
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// Create metadata.xml
	metaWriter, _ := zipWriter.Create("metadata.xml")
	metadata := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<DBLMetadata id="%s" revision="1" type="text" typeVersion="3.0">
  <identification>
    <name>%s</name>
    <description>%s</description>
  </identification>
  <language>
    <iso>%s</iso>
  </language>
  <copyright>
    <statement>%s</statement>
  </copyright>
</DBLMetadata>`, corpus.ID, escapeXML(corpus.Title), escapeXML(corpus.Description), corpus.Language, escapeXML(corpus.Rights))
	metaWriter.Write([]byte(metadata))

	// Create USX files
	for _, doc := range corpus.Documents {
		usxWriter, _ := zipWriter.Create(fmt.Sprintf("release/%s.usx", doc.ID))
		usxContent := generateUSX(doc)
		usxWriter.Write([]byte(usxContent))
	}

	zipWriter.Close()

	if err := os.WriteFile(outputPath, buf.Bytes(), 0600); err != nil {
		return "", fmt.Errorf("failed to write DBL: %w", err)
	}

	return outputPath, nil
}

func generateUSX(doc *ir.Document) string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="%s" style="id">%s</book>
`, doc.ID, doc.Title))

	chapterMap := make(map[int][]*ir.ContentBlock)
	for _, cb := range doc.ContentBlocks {
		for _, anchor := range cb.Anchors {
			for _, span := range anchor.Spans {
				if span.Ref != nil && span.Type == "VERSE" {
					chapterMap[span.Ref.Chapter] = append(chapterMap[span.Ref.Chapter], cb)
				}
			}
		}
	}

	for chNum := 1; chNum <= len(chapterMap); chNum++ {
		blocks, ok := chapterMap[chNum]
		if !ok {
			continue
		}

		buf.WriteString(fmt.Sprintf(`  <chapter number="%d" style="c"/>
`, chNum))

		for _, cb := range blocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						buf.WriteString(fmt.Sprintf(`  <para style="p"><verse number="%d" style="v"/>%s</para>
`, span.Ref.Verse, escapeXML(cb.Text)))
					}
				}
			}
		}
	}

	buf.WriteString(`</usx>`)
	return buf.String()
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
