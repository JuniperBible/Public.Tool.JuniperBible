//go:build !sdk

// Plugin format-sblgnt handles SBL Greek New Testament format.
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

type IPCRequest struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args,omitempty"`
}

type IPCResponse struct {
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

type DetectResult struct {
	Detected bool   `json:"detected"`
	Format   string `json:"format,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type IngestResult struct {
	ArtifactID string            `json:"artifact_id"`
	BlobSHA256 string            `json:"blob_sha256"`
	SizeBytes  int64             `json:"size_bytes"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type EnumerateResult struct {
	Entries []EnumerateEntry `json:"entries"`
}

type EnumerateEntry struct {
	Path      string            `json:"path"`
	SizeBytes int64             `json:"size_bytes"`
	IsDir     bool              `json:"is_dir"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type ExtractIRResult struct {
	IRPath     string      `json:"ir_path"`
	LossClass  string      `json:"loss_class"`
	LossReport *LossReport `json:"loss_report,omitempty"`
}

type EmitNativeResult struct {
	OutputPath string      `json:"output_path"`
	Format     string      `json:"format"`
	LossClass  string      `json:"loss_class"`
	LossReport *LossReport `json:"loss_report,omitempty"`
}

type LossReport struct {
	SourceFormat string   `json:"source_format"`
	TargetFormat string   `json:"target_format"`
	LossClass    string   `json:"loss_class"`
	Warnings     []string `json:"warnings,omitempty"`
}

type Corpus struct {
	ID           string            `json:"id"`
	Version      string            `json:"version"`
	ModuleType   string            `json:"module_type"`
	Language     string            `json:"language,omitempty"`
	Title        string            `json:"title,omitempty"`
	SourceFormat string            `json:"source_format,omitempty"`
	Documents    []*Document       `json:"documents,omitempty"`
	SourceHash   string            `json:"source_hash,omitempty"`
	LossClass    string            `json:"loss_class,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
}

type Document struct {
	ID            string          `json:"id"`
	Title         string          `json:"title,omitempty"`
	Order         int             `json:"order"`
	ContentBlocks []*ContentBlock `json:"content_blocks,omitempty"`
}

type ContentBlock struct {
	ID       string    `json:"id"`
	Sequence int       `json:"sequence"`
	Text     string    `json:"text"`
	Anchors  []*Anchor `json:"anchors,omitempty"`
	Hash     string    `json:"hash,omitempty"`
}

type Anchor struct {
	ID       string  `json:"id"`
	Position int     `json:"position"`
	Spans    []*Span `json:"spans,omitempty"`
}

type Span struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	StartAnchorID string `json:"start_anchor_id"`
	Ref           *Ref   `json:"ref,omitempty"`
}

type Ref struct {
	Book    string `json:"book"`
	Chapter int    `json:"chapter,omitempty"`
	Verse   int    `json:"verse,omitempty"`
	OSISID  string `json:"osis_id,omitempty"`
}

func main() {
	var req IPCRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		respondError(fmt.Sprintf("failed to decode: %v", err))
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
		respondError("unknown command")
	}
}

func handleDetect(args map[string]interface{}) {
	path := args["path"].(string)
	base := strings.ToLower(filepath.Base(path))
	if strings.Contains(base, "sblgnt") || strings.Contains(base, "sbl-gnt") {
		respond(&DetectResult{Detected: true, Format: "SBLGNT", Reason: "SBLGNT filename"})
		return
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".txt" || ext == ".tsv" {
		data, _ := os.ReadFile(path)
		content := string(data)
		greekPattern := regexp.MustCompile(`[\x{0370}-\x{03FF}]`)
		refPattern := regexp.MustCompile(`\d{8}`)
		if greekPattern.MatchString(content) && refPattern.MatchString(content) {
			respond(&DetectResult{Detected: true, Format: "SBLGNT", Reason: "Greek NT format detected"})
			return
		}
	}
	respond(&DetectResult{Detected: false, Reason: "not SBLGNT"})
}

func handleIngest(args map[string]interface{}) {
	path := args["path"].(string)
	outputDir := args["output_dir"].(string)
	data, _ := os.ReadFile(path)
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])
	blobDir := filepath.Join(outputDir, hashHex[:2])
	os.MkdirAll(blobDir, 0755)
	os.WriteFile(filepath.Join(blobDir, hashHex), data, 0644)
	respond(&IngestResult{
		ArtifactID: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata:   map[string]string{"format": "SBLGNT"},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path := args["path"].(string)
	info, _ := os.Stat(path)
	respond(&EnumerateResult{Entries: []EnumerateEntry{{Path: filepath.Base(path), SizeBytes: info.Size()}}})
}

func handleExtractIR(args map[string]interface{}) {
	path := args["path"].(string)
	outputDir := args["output_dir"].(string)
	data, _ := os.ReadFile(path)
	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &Corpus{
		ID: artifactID, Version: "1.0.0", ModuleType: "BIBLE", Language: "grc",
		Title: "SBL Greek NT", SourceFormat: "SBLGNT", SourceHash: hex.EncodeToString(sourceHash[:]),
		LossClass: "L1", Attributes: map[string]string{"_sblgnt_raw": hex.EncodeToString(data)},
	}
	corpus.Documents = extractContent(string(data), artifactID)
	if len(corpus.Documents) == 0 {
		corpus.Documents = []*Document{{ID: artifactID, Title: artifactID, Order: 1}}
	}
	irData, _ := json.MarshalIndent(corpus, "", "  ")
	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	os.WriteFile(irPath, irData, 0644)
	respond(&ExtractIRResult{IRPath: irPath, LossClass: "L1"})
}

func extractContent(content, artifactID string) []*Document {
	verseWords := make(map[string][]string)
	bookOrder := make(map[string]int)
	scanner := bufio.NewScanner(strings.NewReader(content))
	orderCounter := 0
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		ref := fields[0]
		if len(ref) < 6 {
			continue
		}
		book := ref[0:2]
		chapter, _ := strconv.Atoi(ref[2:4])
		verse, _ := strconv.Atoi(ref[4:6])
		verseRef := fmt.Sprintf("%s.%d.%d", book, chapter, verse)
		if _, exists := bookOrder[book]; !exists {
			orderCounter++
			bookOrder[book] = orderCounter
		}
		verseWords[verseRef] = append(verseWords[verseRef], fields[len(fields)-1])
	}
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
			bookDocs[book] = &Document{ID: book, Title: book, Order: bookOrder[book]}
		}
		text := strings.Join(words, " ")
		sequence++
		hash := sha256.Sum256([]byte(text))
		osisID := fmt.Sprintf("%s.%d.%d", book, chapter, verse)
		cb := &ContentBlock{
			ID: fmt.Sprintf("cb-%d", sequence), Sequence: chapter*1000 + verse, Text: text,
			Hash: hex.EncodeToString(hash[:]),
			Anchors: []*Anchor{{ID: fmt.Sprintf("a-%d", sequence), Position: 0,
				Spans: []*Span{{ID: fmt.Sprintf("s-%s", osisID), Type: "VERSE", StartAnchorID: fmt.Sprintf("a-%d", sequence),
					Ref: &Ref{Book: book, Chapter: chapter, Verse: verse, OSISID: osisID}}}}},
		}
		bookDocs[book].ContentBlocks = append(bookDocs[book].ContentBlocks, cb)
	}
	var documents []*Document
	for _, doc := range bookDocs {
		documents = append(documents, doc)
	}
	return documents
}

func handleEmitNative(args map[string]interface{}) {
	irPath := args["ir_path"].(string)
	outputDir := args["output_dir"].(string)
	data, _ := os.ReadFile(irPath)
	var corpus Corpus
	json.Unmarshal(data, &corpus)
	outputPath := filepath.Join(outputDir, corpus.ID+".txt")
	if raw, ok := corpus.Attributes["_sblgnt_raw"]; ok && raw != "" {
		rawData, _ := hex.DecodeString(raw)
		os.WriteFile(outputPath, rawData, 0644)
		respond(&EmitNativeResult{OutputPath: outputPath, Format: "SBLGNT", LossClass: "L0"})
		return
	}
	var buf bytes.Buffer
	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			chapter, verse := 1, cb.Sequence%1000
			if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 && cb.Anchors[0].Spans[0].Ref != nil {
				chapter = cb.Anchors[0].Spans[0].Ref.Chapter
				verse = cb.Anchors[0].Spans[0].Ref.Verse
			}
			fmt.Fprintf(&buf, "%s%02d%02d001\t%s\n", doc.ID, chapter, verse, cb.Text)
		}
	}
	os.WriteFile(outputPath, buf.Bytes(), 0644)
	respond(&EmitNativeResult{OutputPath: outputPath, Format: "SBLGNT", LossClass: "L1"})
}

func respond(result interface{}) {
	json.NewEncoder(os.Stdout).Encode(IPCResponse{Status: "ok", Result: result})
}

func respondError(msg string) {
	json.NewEncoder(os.Stdout).Encode(IPCResponse{Status: "error", Error: msg})
	os.Exit(1)
}
