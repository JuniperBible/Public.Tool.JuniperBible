// Package xml handles generic XML Bible format.
// Uses simple XML structure with verses as elements.
//
// IR Support:
// - extract-ir: Reads generic XML Bible format to IR (L1)
// - emit-native: Converts IR to generic XML format (L1)
package xml

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
	"github.com/JuniperBible/juniper/plugins/sdk/ir"
)

// XML Types for generic Bible XML
type XMLBible struct {
	XMLName  xml.Name  `xml:"bible"`
	Title    string    `xml:"title,attr,omitempty"`
	Language string    `xml:"language,attr,omitempty"`
	Books    []XMLBook `xml:"book"`
}

type XMLBook struct {
	ID       string       `xml:"id,attr"`
	Name     string       `xml:"name,attr,omitempty"`
	Chapters []XMLChapter `xml:"chapter"`
}

type XMLChapter struct {
	Number int        `xml:"number,attr"`
	Verses []XMLVerse `xml:"verse"`
}

type XMLVerse struct {
	Number int    `xml:"number,attr"`
	Text   string `xml:",chardata"`
}

// Config defines the XML format plugin.
var Config = &format.Config{
	PluginID:   "format.xml",
	Name:       "XML",
	Extensions: []string{".xml"},
	Detect:     detectXML,
	Parse:      parseXML,
	Emit:       emitXML,
}

func detectXML(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".xml" {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not an .xml file",
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		}, nil
	}

	content := string(data)
	// Check for <bible> or similar root element
	if strings.Contains(content, "<bible") ||
		(strings.Contains(content, "<book") && strings.Contains(content, "<verse")) {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "XML",
			Reason:   "Generic XML Bible format detected",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "no Bible XML structure found",
	}, nil
}

func parseXML(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "XML"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L1"
	corpus.Attributes = make(map[string]string)

	// Store raw for round-trip
	corpus.Attributes["_xml_raw"] = string(data)

	// Parse XML
	var bible XMLBible
	if err := xml.Unmarshal(data, &bible); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	corpus.Title = bible.Title
	corpus.Language = bible.Language

	// Convert to IR
	for i, book := range bible.Books {
		doc := ir.NewDocument(book.ID, book.Name, i+1)
		if doc.Title == "" {
			doc.Title = book.ID
		}

		sequence := 0
		for _, chapter := range book.Chapters {
			for _, verse := range chapter.Verses {
				text := strings.TrimSpace(verse.Text)
				if text == "" {
					continue
				}

				sequence++
				hash := sha256.Sum256([]byte(text))
				osisID := fmt.Sprintf("%s.%d.%d", book.ID, chapter.Number, verse.Number)

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
										Book:    book.ID,
										Chapter: chapter.Number,
										Verse:   verse.Number,
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
		corpus.Documents = append(corpus.Documents, doc)
	}

	return corpus, nil
}

func emitXML(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".xml")

	// Check for raw XML for round-trip
	if raw, ok := corpus.Attributes["_xml_raw"]; ok && raw != "" {
		return writeRawXML(outputPath, raw)
	}

	// Generate XML from IR
	bible := buildXMLBible(corpus)

	// Marshal and write
	return writeXMLBible(outputPath, bible)
}

// writeRawXML writes the raw XML content to the output path
func writeRawXML(outputPath, raw string) (string, error) {
	if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
		return "", fmt.Errorf("failed to write XML: %w", err)
	}
	return outputPath, nil
}

// buildXMLBible constructs an XMLBible from the IR corpus
func buildXMLBible(corpus *ir.Corpus) XMLBible {
	bible := XMLBible{
		Title:    corpus.Title,
		Language: corpus.Language,
	}

	for _, doc := range corpus.Documents {
		book := buildXMLBook(doc)
		bible.Books = append(bible.Books, book)
	}

	return bible
}

// buildXMLBook constructs an XMLBook from an IR document
func buildXMLBook(doc *ir.Document) XMLBook {
	book := XMLBook{
		ID:   doc.ID,
		Name: doc.Title,
	}

	// Group content blocks by chapter
	chapterMap := groupContentBlocksByChapter(doc.ContentBlocks)

	// Sort chapters numerically
	book.Chapters = sortChaptersFromMap(chapterMap)

	return book
}

// groupContentBlocksByChapter groups content blocks into chapters based on verse references
func groupContentBlocksByChapter(blocks []*ir.ContentBlock) map[int]*XMLChapter {
	chapterMap := make(map[int]*XMLChapter)

	for _, cb := range blocks {
		processContentBlockAnchors(cb, chapterMap)
	}

	return chapterMap
}

// processContentBlockAnchors processes all anchors in a content block and updates the chapter map
func processContentBlockAnchors(cb *ir.ContentBlock, chapterMap map[int]*XMLChapter) {
	for _, anchor := range cb.Anchors {
		processAnchorSpans(anchor, cb, chapterMap)
	}
}

// processAnchorSpans processes all spans in an anchor and updates the chapter map
func processAnchorSpans(anchor *ir.Anchor, cb *ir.ContentBlock, chapterMap map[int]*XMLChapter) {
	for _, span := range anchor.Spans {
		if isVerseSpan(span) {
			addVerseToChapter(span, cb, chapterMap)
		}
	}
}

// isVerseSpan checks if a span represents a verse reference
func isVerseSpan(span *ir.Span) bool {
	return span.Ref != nil && span.Type == "VERSE"
}

// addVerseToChapter adds a verse to the appropriate chapter in the map
func addVerseToChapter(span *ir.Span, cb *ir.ContentBlock, chapterMap map[int]*XMLChapter) {
	chapter := getOrCreateChapter(chapterMap, span.Ref.Chapter)
	chapter.Verses = append(chapter.Verses, XMLVerse{
		Number: span.Ref.Verse,
		Text:   cb.Text,
	})
}

// getOrCreateChapter retrieves or creates a chapter from the map
func getOrCreateChapter(chapterMap map[int]*XMLChapter, chapterNum int) *XMLChapter {
	chapter, exists := chapterMap[chapterNum]
	if !exists {
		chapter = &XMLChapter{Number: chapterNum}
		chapterMap[chapterNum] = chapter
	}
	return chapter
}

// sortChaptersFromMap sorts chapters numerically from the map
func sortChaptersFromMap(chapterMap map[int]*XMLChapter) []XMLChapter {
	var chapters []XMLChapter
	for i := 1; i <= len(chapterMap); i++ {
		if chapter, ok := chapterMap[i]; ok {
			chapters = append(chapters, *chapter)
		}
	}
	return chapters
}

// writeXMLBible marshals and writes the XMLBible to the output path
func writeXMLBible(outputPath string, bible XMLBible) (string, error) {
	xmlData, err := xml.MarshalIndent(bible, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal XML: %w", err)
	}

	xmlContent := xml.Header + string(xmlData)
	if err := os.WriteFile(outputPath, []byte(xmlContent), 0600); err != nil {
		return "", fmt.Errorf("failed to write XML: %w", err)
	}

	return outputPath, nil
}
