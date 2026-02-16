//go:build !sdk

// Plugin format-zefania handles Zefania XML Bible file ingestion.
// Zefania XML is a Bible format used primarily in German-speaking regions.
//
// IR Support:
// - extract-ir: Extracts IR from Zefania XML (L0 lossless via raw storage)
// - emit-native: Converts IR back to Zefania format (L0)
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// Zefania book number to OSIS ID mapping
var zefaniaBookToOSIS = map[int]string{
	1: "Gen", 2: "Exod", 3: "Lev", 4: "Num", 5: "Deut",
	6: "Josh", 7: "Judg", 8: "Ruth", 9: "1Sam", 10: "2Sam",
	11: "1Kgs", 12: "2Kgs", 13: "1Chr", 14: "2Chr", 15: "Ezra",
	16: "Neh", 17: "Esth", 18: "Job", 19: "Ps", 20: "Prov",
	21: "Eccl", 22: "Song", 23: "Isa", 24: "Jer", 25: "Lam",
	26: "Ezek", 27: "Dan", 28: "Hos", 29: "Joel", 30: "Amos",
	31: "Obad", 32: "Jonah", 33: "Mic", 34: "Nah", 35: "Hab",
	36: "Zeph", 37: "Hag", 38: "Zech", 39: "Mal",
	40: "Matt", 41: "Mark", 42: "Luke", 43: "John", 44: "Acts",
	45: "Rom", 46: "1Cor", 47: "2Cor", 48: "Gal", 49: "Eph",
	50: "Phil", 51: "Col", 52: "1Thess", 53: "2Thess",
	54: "1Tim", 55: "2Tim", 56: "Titus", 57: "Phlm", 58: "Heb",
	59: "Jas", 60: "1Pet", 61: "2Pet", 62: "1John", 63: "2John",
	64: "3John", 65: "Jude", 66: "Rev",
}

var osisToZefaniaBook = func() map[string]int {
	m := make(map[string]int)
	for k, v := range zefaniaBookToOSIS {
		m[v] = k
	}
	return m
}()

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

	info, err := os.Stat(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if info.IsDir() {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "path is a directory, not a file",
		})
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		})
		return
	}

	content := string(data)
	// Check for Zefania XML markers
	if !strings.Contains(content, "<XMLBIBLE") && !strings.Contains(content, "<xmlbible") {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not a Zefania XML file (no <XMLBIBLE> element)",
		})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   "Zefania",
		Reason:   "Zefania XML detected",
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
			"format": "Zefania",
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

	entries := []ipc.EnumerateEntry{
		{
			Path:      filepath.Base(path),
			SizeBytes: info.Size(),
			IsDir:     false,
			Metadata: map[string]string{
				"format": "Zefania",
			},
		},
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

	corpus, err := parseZefaniaToIR(data)
	if err != nil {
		ipc.RespondErrorf("failed to parse Zefania: %v", err)
		return
	}

	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L0"

	if corpus.Attributes == nil {
		corpus.Attributes = make(map[string]string)
	}
	corpus.Attributes["_zefania_raw"] = string(data)

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
		LossClass: "L0",
		LossReport: &ipc.LossReport{
			SourceFormat: "Zefania",
			TargetFormat: "IR",
			LossClass:    "L0",
		},
	})
}

func parseZefaniaToIR(data []byte) (*ipc.Corpus, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))

	corpus := &ipc.Corpus{
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "Zefania",
		Attributes:   make(map[string]string),
	}

	var currentBook *ipc.Document
	var currentChapter int
	sequence := 0

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch strings.ToUpper(t.Name.Local) {
			case "XMLBIBLE":
				for _, attr := range t.Attr {
					switch strings.ToLower(attr.Name.Local) {
					case "biblename":
						corpus.Title = attr.Value
						corpus.ID = sanitizeID(attr.Value)
					case "language":
						corpus.Language = attr.Value
					}
				}

			case "BIBLEBOOK":
				var bookNum int
				var bookName string
				for _, attr := range t.Attr {
					switch strings.ToLower(attr.Name.Local) {
					case "bnumber":
						bookNum, _ = strconv.Atoi(attr.Value)
					case "bname":
						bookName = attr.Value
					}
				}

				osisID := zefaniaBookToOSIS[bookNum]
				if osisID == "" {
					osisID = sanitizeID(bookName)
				}

				currentBook = &ipc.Document{
					ID:    osisID,
					Title: bookName,
					Order: bookNum,
					Attributes: map[string]string{
						"bnumber": strconv.Itoa(bookNum),
					},
				}
				corpus.Documents = append(corpus.Documents, currentBook)

			case "CHAPTER":
				for _, attr := range t.Attr {
					if strings.ToLower(attr.Name.Local) == "cnumber" {
						currentChapter, _ = strconv.Atoi(attr.Value)
					}
				}

			case "VERS":
				var verseNum int
				for _, attr := range t.Attr {
					if strings.ToLower(attr.Name.Local) == "vnumber" {
						verseNum, _ = strconv.Atoi(attr.Value)
					}
				}

				// Read verse content
				var textContent strings.Builder
				for {
					innerToken, err := decoder.Token()
					if err != nil {
						break
					}
					if end, ok := innerToken.(xml.EndElement); ok && strings.ToUpper(end.Name.Local) == "VERS" {
						break
					}
					if charData, ok := innerToken.(xml.CharData); ok {
						textContent.Write(charData)
					}
				}

				text := strings.TrimSpace(textContent.String())
				if text != "" && currentBook != nil {
					sequence++
					hash := sha256.Sum256([]byte(text))
					osisID := fmt.Sprintf("%s.%d.%d", currentBook.ID, currentChapter, verseNum)

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
											Book:    currentBook.ID,
											Chapter: currentChapter,
											Verse:   verseNum,
											OSISID:  osisID,
										},
									},
								},
							},
						},
					}
					currentBook.ContentBlocks = append(currentBook.ContentBlocks, cb)
				}
			}
		}
	}

	if corpus.ID == "" {
		corpus.ID = "zefania"
	}

	return corpus, nil
}

func sanitizeID(s string) string {
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, s)
	return strings.Trim(result, "-")
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

	// Check for raw Zefania for L0 round-trip
	if raw, ok := corpus.Attributes["_zefania_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			ipc.RespondErrorf("failed to write Zefania: %v", err)
			return
		}
		ipc.MustRespond(&ipc.EmitNativeResult{
			OutputPath: outputPath,
			Format:     "Zefania",
			LossClass:  "L0",
			LossReport: &ipc.LossReport{
				SourceFormat: "IR",
				TargetFormat: "Zefania",
				LossClass:    "L0",
			},
		})
		return
	}

	// Generate Zefania from IR
	zefaniaContent := emitZefaniaFromIR(&corpus)
	if err := os.WriteFile(outputPath, []byte(zefaniaContent), 0644); err != nil {
		ipc.RespondErrorf("failed to write Zefania: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "Zefania",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "Zefania",
			LossClass:    "L1",
			Warnings: []string{
				"Zefania regenerated from IR - some formatting may differ",
			},
		},
	})
}

func emitZefaniaFromIR(corpus *ipc.Corpus) string {
	var buf strings.Builder

	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
`)
	buf.WriteString(fmt.Sprintf(`<XMLBIBLE biblename="%s"`, escapeXML(corpus.Title)))
	if corpus.Language != "" {
		buf.WriteString(fmt.Sprintf(` language="%s"`, escapeXML(corpus.Language)))
	}
	buf.WriteString(">\n")

	for _, doc := range corpus.Documents {
		bookNum := osisToZefaniaBook[doc.ID]
		if bookNum == 0 {
			bookNum = doc.Order
		}
		buf.WriteString(fmt.Sprintf(`  <BIBLEBOOK bnumber="%d" bname="%s">
`, bookNum, escapeXML(doc.Title)))

		currentChapter := 0
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						if span.Ref.Chapter != currentChapter {
							if currentChapter > 0 {
								buf.WriteString("    </CHAPTER>\n")
							}
							currentChapter = span.Ref.Chapter
							buf.WriteString(fmt.Sprintf(`    <CHAPTER cnumber="%d">
`, currentChapter))
						}
						buf.WriteString(fmt.Sprintf(`      <VERS vnumber="%d">%s</VERS>
`, span.Ref.Verse, escapeXML(cb.Text)))
					}
				}
			}
		}
		if currentChapter > 0 {
			buf.WriteString("    </CHAPTER>\n")
		}
		buf.WriteString("  </BIBLEBOOK>\n")
	}

	buf.WriteString("</XMLBIBLE>\n")
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
