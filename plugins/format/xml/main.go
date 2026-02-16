//go:build !sdk

// Plugin format-xml handles generic XML Bible format.
// Uses simple XML structure with verses as elements.
//
// IR Support:
// - extract-ir: Reads generic XML Bible format to IR (L1)
// - emit-native: Converts IR to generic XML format (L1)
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
	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	// Two-stage detection: extension + content
	result := ipc.StandardDetect(path, "XML",
		[]string{".xml"},
		[]string{"<bible", "<book", "<verse"},
	)
	ipc.MustRespond(result)
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
	if err := os.WriteFile(blobPath, data, 0600); err != nil {
		ipc.RespondErrorf("failed to write blob: %v", err)
		return
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "XML",
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
				Metadata:  map[string]string{"format": "XML"},
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
		ipc.RespondErrorf("failed to parse XML: %v", err)
		return
	}

	corpus.Title = bible.Title
	corpus.Language = bible.Language

	// Convert to IR
	for i, book := range bible.Books {
		doc := &ipc.Document{
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

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to serialize IR: %v", err)
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0600); err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}
	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "XML",
			TargetFormat: "IR",
			LossClass:    "L1",
		},
	})
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

	outputPath := filepath.Join(outputDir, corpus.ID+".xml")

	// Check for raw XML for round-trip
	if raw, ok := corpus.Attributes["_xml_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
			ipc.RespondErrorf("failed to write XML: %v", err)
			return
		}
		ipc.MustRespond(&ipc.EmitNativeResult{
			OutputPath: outputPath,
			Format:     "XML",
			LossClass:  "L0",
			LossReport: &ipc.LossReport{
				SourceFormat: "IR",
				TargetFormat: "XML",
				LossClass:    "L0",
			},
		})
		return
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
		ipc.RespondErrorf("failed to marshal XML: %v", err)
		return
	}

	xmlContent := xml.Header + string(xmlData)
	if err := os.WriteFile(outputPath, []byte(xmlContent), 0600); err != nil {
		ipc.RespondErrorf("failed to write XML: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "XML",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "XML",
			LossClass:    "L1",
		},
	})
}
