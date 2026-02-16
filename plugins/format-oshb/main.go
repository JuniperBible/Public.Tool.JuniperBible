//go:build !sdk

// Plugin format-oshb handles OpenScriptures Hebrew Bible format.
// OSHB is a morphologically parsed Hebrew Bible in XML/TSV format.
//
// IR Support:
// - extract-ir: Reads OSHB to IR (L1)
// - emit-native: Converts IR to OSHB format (L1)
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	base := strings.ToLower(filepath.Base(path))

	// OSHB files typically have specific names
	if strings.Contains(base, "oshb") || strings.Contains(base, "openscriptures") {
		respond(&DetectResult{
			Detected: true,
			Format:   "OSHB",
			Reason:   "OSHB filename detected",
		})
		return
	}

	// Check for TSV/TXT with Hebrew morphology
	if ext == ".txt" || ext == ".tsv" || ext == ".xml" {
		data, err := os.ReadFile(path)
		if err != nil {
			respond(&DetectResult{
				Detected: false,
				Reason:   "not an OSHB file",
			})
			return
		}

		content := string(data)
		// OSHB format patterns
		oshbPattern := regexp.MustCompile(`(?:Gen|Exod?|Lev|Num|Deut)\.\d+\.\d+`)
		hebrewPattern := regexp.MustCompile(`[\x{0590}-\x{05FF}]`)

		if oshbPattern.MatchString(content) && hebrewPattern.MatchString(content) {
			respond(&DetectResult{
				Detected: true,
				Format:   "OSHB",
				Reason:   "OSHB Hebrew morphology detected",
			})
			return
		}

		// Check for OSIS-style Hebrew
		if strings.Contains(content, "<osis") && hebrewPattern.MatchString(content) {
			respond(&DetectResult{
				Detected: true,
				Format:   "OSHB",
				Reason:   "OSHB OSIS Hebrew format detected",
			})
			return
		}
	}

	respond(&DetectResult{
		Detected: false,
		Reason:   "no OSHB structure found",
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
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		respondError(fmt.Sprintf("failed to create blob dir: %v", err))
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write blob: %v", err))
		return
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	respond(&IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format":   "OSHB",
			"language": "hbo",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to stat: %v", err))
		return
	}

	respond(&EnumerateResult{
		Entries: []EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
				Metadata:  map[string]string{"format": "OSHB"},
			},
		},
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
		Language:     "hbo",
		Title:        "OpenScriptures Hebrew Bible",
		SourceFormat: "OSHB",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L1",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_oshb_raw"] = hex.EncodeToString(data)

	// Extract content
	corpus.Documents = extractOSHBContent(string(data), artifactID)

	if len(corpus.Documents) == 0 {
		corpus.Documents = []*Document{
			{
				ID:    artifactID,
				Title: artifactID,
				Order: 1,
			},
		}
	}

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		respondError(fmt.Sprintf("failed to serialize IR: %v", err))
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write IR: %v", err))
		return
	}

	respond(&ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
		LossReport: &LossReport{
			SourceFormat: "OSHB",
			TargetFormat: "IR",
			LossClass:    "L1",
			Warnings:     []string{"OSHB morphological analysis preserved"},
		},
	})
}

func extractOSHBContent(content, artifactID string) []*Document {
	verseWords := make(map[string][]string)
	bookOrder := make(map[string]int)

	// Parse OSHB-style references: Book.Chapter.Verse Word
	versePattern := regexp.MustCompile(`(Gen|Exod?|Lev|Num|Deut|Josh|Judg|Ruth|[12]Sam|[12]Kgs|[12]Chr|Ezra|Neh|Esth|Job|Ps|Prov|Eccl|Song|Isa|Jer|Lam|Ezek|Dan|Hos|Joel|Amos|Obad|Jonah|Mic|Nah|Hab|Zeph|Hag|Zech|Mal)\.(\d+)\.(\d+)\s+(.+)`)

	scanner := bufio.NewScanner(strings.NewReader(content))
	orderCounter := 0

	for scanner.Scan() {
		line := scanner.Text()
		matches := versePattern.FindStringSubmatch(line)
		if len(matches) >= 5 {
			book := matches[1]
			chapter := matches[2]
			verse := matches[3]
			text := matches[4]

			verseRef := fmt.Sprintf("%s.%s.%s", book, chapter, verse)

			if _, exists := bookOrder[book]; !exists {
				orderCounter++
				bookOrder[book] = orderCounter
			}

			verseWords[verseRef] = append(verseWords[verseRef], text)
		}
	}

	// Create documents
	bookDocs := make(map[string]*Document)
	sequence := 0

	for verseRef, words := range verseWords {
		parts := strings.Split(verseRef, ".")
		if len(parts) < 3 {
			continue
		}

		book := parts[0]
		chapter, _ := strconv.Atoi(parts[1])
		verse, _ := strconv.Atoi(parts[2])

		if _, exists := bookDocs[book]; !exists {
			bookDocs[book] = &Document{
				ID:    book,
				Title: book,
				Order: bookOrder[book],
			}
		}

		text := strings.Join(words, " ")
		sequence++
		hash := sha256.Sum256([]byte(text))
		osisID := fmt.Sprintf("%s.%d.%d", book, chapter, verse)

		cb := &ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: chapter*1000 + verse,
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
								Book:    book,
								Chapter: chapter,
								Verse:   verse,
								OSISID:  osisID,
							},
						},
					},
				},
			},
		}

		bookDocs[book].ContentBlocks = append(bookDocs[book].ContentBlocks, cb)
	}

	var documents []*Document
	for _, doc := range bookDocs {
		documents = append(documents, doc)
	}

	// Sort by order
	for i := 0; i < len(documents); i++ {
		for j := i + 1; j < len(documents); j++ {
			if documents[i].Order > documents[j].Order {
				documents[i], documents[j] = documents[j], documents[i]
			}
		}
	}

	return documents
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

	outputPath := filepath.Join(outputDir, corpus.ID+".txt")

	// Check for raw OSHB for round-trip
	if raw, ok := corpus.Attributes["_oshb_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0644); err != nil {
				respondError(fmt.Sprintf("failed to write OSHB: %v", err))
				return
			}

			respond(&EmitNativeResult{
				OutputPath: outputPath,
				Format:     "OSHB",
				LossClass:  "L0",
				LossReport: &LossReport{
					SourceFormat: "IR",
					TargetFormat: "OSHB",
					LossClass:    "L0",
				},
			})
			return
		}
	}

	// Generate OSHB-style format from IR
	var buf bytes.Buffer

	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			chapter := 1
			verse := cb.Sequence % 1000

			if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 {
				if ref := cb.Anchors[0].Spans[0].Ref; ref != nil {
					chapter = ref.Chapter
					verse = ref.Verse
				}
			}

			fmt.Fprintf(&buf, "%s.%d.%d %s\n", doc.ID, chapter, verse, cb.Text)
		}
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		respondError(fmt.Sprintf("failed to write OSHB: %v", err))
		return
	}

	respond(&EmitNativeResult{
		OutputPath: outputPath,
		Format:     "OSHB",
		LossClass:  "L1",
		LossReport: &LossReport{
			SourceFormat: "IR",
			TargetFormat: "OSHB",
			LossClass:    "L1",
			Warnings:     []string{"Generated OSHB-compatible format"},
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
