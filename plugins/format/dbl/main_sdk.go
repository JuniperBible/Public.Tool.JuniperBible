//go:build sdk

// Plugin format-dbl handles Digital Bible Library format.
// DBL is a bundle format containing USX files and metadata.
//
// IR Support:
// - extract-ir: Reads DBL bundle to IR (L1)
// - emit-native: Converts IR to DBL bundle (L1)
package main

import (
	"archive/zip"
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
	Type           string   `xml:"type,attr"`
	TypeVersion    string   `xml:"typeVersion,attr"`
	Identification struct {
		Name        string `xml:"name"`
		NameLocal   string `xml:"nameLocal"`
		Description string `xml:"description"`
		Scope       string `xml:"scope"`
	} `xml:"identification"`
	Language struct {
		ISO    string `xml:"iso"`
		Name   string `xml:"name"`
		Script string `xml:"script"`
	} `xml:"language"`
	Copyright struct {
		Statement string `xml:"statement"`
	} `xml:"copyright"`
	Publications struct {
		Publication []struct {
			ID        string `xml:"id,attr"`
			Default   bool   `xml:"default,attr"`
			Name      string `xml:"name"`
			Canonspec struct {
				Component []struct {
					Name string `xml:"name,attr"`
				} `xml:"component"`
			} `xml:"canonicalContent"`
		} `xml:"publication"`
	} `xml:"publications"`
	Names struct {
		Name []struct {
			ID    string `xml:"id,attr"`
			Short string `xml:"short"`
			Abbr  string `xml:"abbr"`
			Long  string `xml:"long"`
		} `xml:"name"`
	} `xml:"names"`
}

func main() {
	format.Run(&format.Config{
		Name:       "format-dbl",
		Extensions: []string{".zip", ".dbl"},
		Detect:     detectWrapper,
		Parse:      parse,
		Emit:       emit,
		Enumerate:  enumerate,
	})
}

func detectWrapper(path string) (*ipc.DetectResult, error) {
	detected, reason, err := detect(path)
	if err != nil {
		return nil, err
	}
	return &ipc.DetectResult{
		Detected: detected,
		Format:   "DBL",
		Reason:   reason,
	}, nil
}

func detect(path string) (bool, string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".zip" && ext != ".dbl" {
		return false, "not a .zip or .dbl file", nil
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		return false, fmt.Sprintf("cannot open as zip: %v", err), nil
	}
	defer r.Close()

	// Check for metadata.xml (DBL indicator)
	hasMetadata := false
	for _, f := range r.File {
		if f.Name == "metadata.xml" || strings.HasSuffix(f.Name, "/metadata.xml") {
			hasMetadata = true
			break
		}
	}

	if hasMetadata {
		return true, "Digital Bible Library bundle detected", nil
	}
	return false, "no metadata.xml found in bundle", nil
}

func parse(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ir.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "DBL",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L1",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_dbl_raw"] = hex.EncodeToString(data)

	// Parse DBL bundle
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

	// Parse USX files for content
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
	// Extract book code from filename
	bookCode := strings.TrimSuffix(filepath.Base(filename), ".usx")
	// Handle patterns like "GEN.usx" or "01GEN.usx"
	bookPattern := regexp.MustCompile(`(?:^\d+)?([A-Z]{2,3})(?:\.usx)?$`)
	if match := bookPattern.FindStringSubmatch(bookCode); len(match) > 1 {
		bookCode = match[1]
	}

	doc := &ir.Document{
		ID:    bookCode,
		Title: bookCode,
		Order: order,
	}

	// Parse verses from USX - simplified pattern
	// <verse number="1" style="v"/>text content
	versePattern := regexp.MustCompile(`<verse\s+number="(\d+)"[^>]*/>([^<]+)`)
	chapterPattern := regexp.MustCompile(`<chapter\s+number="(\d+)"`)

	currentChapter := 1
	sequence := 0

	// Find chapter markers
	for _, chMatch := range chapterPattern.FindAllStringSubmatch(content, -1) {
		if len(chMatch) > 1 {
			currentChapter, _ = strconv.Atoi(chMatch[1])
		}
	}

	// Simple approach: find all verses
	// USX often has: <verse number="1" style="v"/>In the beginning...
	simpleVerse := regexp.MustCompile(`<verse\s+number="(\d+)"[^/]*/?>([^<]*)`)
	matches := simpleVerse.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		matches = versePattern.FindAllStringSubmatch(content, -1)
	}

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
			Anchors: []*ir.Anchor{
				{
					ID:       fmt.Sprintf("a-%d-0", sequence),
					Position: 0,
					Spans: []*ir.Span{
						{
							ID:            fmt.Sprintf("s-%s", osisID),
							Type:          "VERSE",
							StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
							Ref: &ir.Ref{
								Book:    bookCode,
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

	return doc
}

func emit(corpus *ir.Corpus, outputPath string) error {
	// Check for raw DBL for round-trip
	if raw, ok := corpus.Attributes["_dbl_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
				return fmt.Errorf("failed to write DBL: %w", err)
			}
			return nil
		}
	}

	// Generate DBL from IR
	zipFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create zip: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Create metadata.xml
	metaWriter, _ := zipWriter.Create("metadata.xml")
	metadata := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<DBLMetadata id="%s" revision="1" type="text" typeVersion="3.0">
  <identification>
    <name>%s</name>
    <nameLocal>%s</nameLocal>
    <description>%s</description>
    <scope>Bible</scope>
  </identification>
  <language>
    <iso>%s</iso>
    <name>%s</name>
    <script>Latn</script>
  </language>
  <copyright>
    <statement>%s</statement>
  </copyright>
  <publications>
    <publication id="default" default="true">
      <name>%s</name>
    </publication>
  </publications>
</DBLMetadata>`, corpus.ID, corpus.Title, corpus.Title, corpus.Description, corpus.Language, corpus.Language, corpus.Rights, corpus.Title)
	metaWriter.Write([]byte(metadata))

	// Create USX files for each book
	for _, doc := range corpus.Documents {
		usxWriter, _ := zipWriter.Create(fmt.Sprintf("release/%s.usx", doc.ID))

		usxContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<usx version="3.0">
  <book code="%s" style="id">%s</book>
`, doc.ID, doc.Title)

		// Group by chapter
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

			usxContent += fmt.Sprintf(`  <chapter number="%d" style="c"/>
`, chNum)

			for _, cb := range blocks {
				for _, anchor := range cb.Anchors {
					for _, span := range anchor.Spans {
						if span.Ref != nil && span.Type == "VERSE" {
							usxContent += fmt.Sprintf(`  <para style="p"><verse number="%d" style="v"/>%s</para>
`, span.Ref.Verse, cb.Text)
						}
					}
				}
			}
		}

		usxContent += `</usx>`
		usxWriter.Write([]byte(usxContent))
	}

	return nil
}

func enumerate(path string) ([]ipc.EnumerateEntry, error) {
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
	return entries, nil
}
