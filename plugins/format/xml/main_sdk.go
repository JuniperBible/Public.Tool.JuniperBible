// Plugin format-xml handles generic XML Bible format using the SDK pattern.
// Uses simple XML structure with verses as elements.
//
// IR Support:
// - extract-ir: Reads generic XML Bible format to IR (L1)
// - emit-native: Converts IR to generic XML format (L1)
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
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

func main() {
	if err := format.Run(&format.Config{
		Name:       "XML",
		Extensions: []string{".xml"},
		Detect:     detectXML,
		Parse:      parseXML,
		Emit:       emitXML,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// detectXML checks if the file is a generic XML Bible format
func detectXML(path string) (*ipc.DetectResult, error) {
	// Two-stage detection: extension + content
	result := ipc.StandardDetect(path, "XML",
		[]string{".xml"},
		[]string{"<bible", "<book", "<verse"},
	)
	return result, nil
}

// parseXML converts XML Bible format to IR
func parseXML(path string) (*ir.Corpus, error) {
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
		SourceFormat: "XML",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L1",
		Attributes:   make(map[string]string),
	}

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
		doc := &ir.Document{
			ID:    book.ID,
			Title: book.Name,
			Order: i + 1,
		}
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

// emitXML converts IR back to XML Bible format
func emitXML(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".xml")

	// Check for raw XML for round-trip
	if raw, ok := corpus.Attributes["_xml_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			return "", fmt.Errorf("failed to write XML: %w", err)
		}
		return outputPath, nil
	}

	// Generate XML from IR
	bible := XMLBible{
		Title:    corpus.Title,
		Language: corpus.Language,
	}

	for _, doc := range corpus.Documents {
		book := XMLBook{
			ID:   doc.ID,
			Name: doc.Title,
		}

		// Group by chapter
		chapterMap := make(map[int]*XMLChapter)
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						chapter, exists := chapterMap[span.Ref.Chapter]
						if !exists {
							chapter = &XMLChapter{Number: span.Ref.Chapter}
							chapterMap[span.Ref.Chapter] = chapter
						}
						chapter.Verses = append(chapter.Verses, XMLVerse{
							Number: span.Ref.Verse,
							Text:   cb.Text,
						})
					}
				}
			}
		}

		// Sort chapters
		for i := 1; i <= len(chapterMap); i++ {
			if chapter, ok := chapterMap[i]; ok {
				book.Chapters = append(book.Chapters, *chapter)
			}
		}

		bible.Books = append(bible.Books, book)
	}

	xmlData, err := xml.MarshalIndent(bible, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal XML: %w", err)
	}

	xmlContent := xml.Header + string(xmlData)
	if err := os.WriteFile(outputPath, []byte(xmlContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write XML: %w", err)
	}

	return outputPath, nil
}
