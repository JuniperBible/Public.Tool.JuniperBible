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

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".zip" && ext != ".dbl" {
		respond(&DetectResult{
			Detected: false,
			Reason:   "not a .zip or .dbl file",
		})
		return
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		respond(&DetectResult{
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
		respond(&DetectResult{
			Detected: true,
			Format:   "DBL",
			Reason:   "Digital Bible Library bundle detected",
		})
		return
	}

	respond(&DetectResult{
		Detected: false,
		Reason:   "no metadata.xml found in bundle",
	})
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
	if err := os.MkdirAll(blobDir, 0700); err != nil {
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
			"format": "DBL",
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
		respondError(fmt.Sprintf("failed to open zip: %v", err))
		return
	}
	defer r.Close()

	var entries []EnumerateEntry
	for _, f := range r.File {
		entries = append(entries, EnumerateEntry{
			Path:      f.Name,
			SizeBytes: int64(f.UncompressedSize64),
			IsDir:     f.FileInfo().IsDir(),
			Metadata:  map[string]string{"format": "DBL"},
		})
	}

	respond(&EnumerateResult{Entries: entries})
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
		respondError(fmt.Sprintf("failed to open DBL: %v", err))
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
			SourceFormat: "DBL",
			TargetFormat: "IR",
			LossClass:    "L1",
		},
	})
}

func parseUSXContent(content, filename string, order int) *Document {
	// Extract book code from filename
	bookCode := strings.TrimSuffix(filepath.Base(filename), ".usx")
	// Handle patterns like "GEN.usx" or "01GEN.usx"
	bookPattern := regexp.MustCompile(`(?:^\d+)?([A-Z]{2,3})(?:\.usx)?$`)
	if match := bookPattern.FindStringSubmatch(bookCode); len(match) > 1 {
		bookCode = match[1]
	}

	doc := &Document{
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

	outputPath := filepath.Join(outputDir, corpus.ID+".zip")

	// Check for raw DBL for round-trip
	if raw, ok := corpus.Attributes["_dbl_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
				respondError(fmt.Sprintf("failed to write DBL: %v", err))
				return
			}

			respond(&EmitNativeResult{
				OutputPath: outputPath,
				Format:     "DBL",
				LossClass:  "L0",
				LossReport: &LossReport{
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
		respondError(fmt.Sprintf("failed to create zip: %v", err))
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
		chapterMap := make(map[int][]*ContentBlock)
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

	respond(&EmitNativeResult{
		OutputPath: outputPath,
		Format:     "DBL",
		LossClass:  "L1",
		LossReport: &LossReport{
			SourceFormat: "IR",
			TargetFormat: "DBL",
			LossClass:    "L1",
		},
	})
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
