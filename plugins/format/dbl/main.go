//go:build !sdk

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
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
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
	if ext != ".zip" && ext != ".dbl" {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not a .zip or .dbl file",
		})
		return
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open as zip: %v", err),
		})
		return
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
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "DBL",
			Reason:   "Digital Bible Library bundle detected",
		})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{
		Detected: false,
		Reason:   "no metadata.xml found in bundle",
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
			"format": "DBL",
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
		ipc.RespondErrorf("failed to open zip: %v", err)
		return
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
	ipc.MustRespond(&ipc.EnumerateResult{Entries: entries})
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
		ipc.RespondErrorf("failed to open DBL: %v", err)
		return
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
			SourceFormat: "DBL",
			TargetFormat: "IR",
			LossClass:    "L1",
		},
	})
}

func parseUSXContent(content, filename string, order int) *ipc.Document {
	// Extract book code from filename
	bookCode := strings.TrimSuffix(filepath.Base(filename), ".usx")
	// Handle patterns like "GEN.usx" or "01GEN.usx"
	bookPattern := regexp.MustCompile(`(?:^\d+)?([A-Z]{2,3})(?:\.usx)?$`)
	if match := bookPattern.FindStringSubmatch(bookCode); len(match) > 1 {
		bookCode = match[1]
	}

	doc := &ipc.Document{
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

	outputPath := filepath.Join(outputDir, corpus.ID+".zip")

	// Check for raw DBL for round-trip
	if raw, ok := corpus.Attributes["_dbl_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
				ipc.RespondErrorf("failed to write DBL: %v", err)
				return
			}
			ipc.MustRespond(&ipc.EmitNativeResult{
				OutputPath: outputPath,
				Format:     "DBL",
				LossClass:  "L0",
				LossReport: &ipc.LossReport{
					SourceFormat: "IR",
					TargetFormat: "DBL",
					LossClass:    "L0",
				},
			})
			return
		}
	}

	// Generate DBL from IR
	zipFile, err := os.Create(outputPath)
	if err != nil {
		ipc.RespondErrorf("failed to create zip: %v", err)
		return
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
		chapterMap := make(map[int][]*ipc.ContentBlock)
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
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "DBL",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "DBL",
			LossClass:    "L1",
		},
	})
}
