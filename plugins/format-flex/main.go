//go:build !sdk

// Plugin format-flex handles FLEx/Fieldworks linguistic database format.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".flextext" || ext == ".fwbackup" || ext == ".fwdata" {
		respond(&DetectResult{Detected: true, Format: "FLEx", Reason: "FLEx extension"})
		return
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	// FLEx XML patterns
	flexPattern := regexp.MustCompile(`<(document|interlinear-text|paragraphs|phrase)[^>]*>`)
	if flexPattern.MatchString(content) && (strings.Contains(content, "flextext") || strings.Contains(content, "FieldWorks")) {
		respond(&DetectResult{Detected: true, Format: "FLEx", Reason: "FLEx XML structure"})
		return
	}
	respond(&DetectResult{Detected: false, Reason: "not FLEx"})
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
		Metadata:   map[string]string{"format": "FLEx"},
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
		ID: artifactID, Version: "1.0.0", ModuleType: "BIBLE",
		Title: artifactID, SourceFormat: "FLEx", SourceHash: hex.EncodeToString(sourceHash[:]),
		LossClass: "L2", Attributes: map[string]string{"_flex_raw": hex.EncodeToString(data)},
	}
	corpus.Documents = extractFLExContent(string(data), artifactID)
	if len(corpus.Documents) == 0 {
		corpus.Documents = []*Document{{ID: artifactID, Title: artifactID, Order: 1}}
	}
	irData, _ := json.MarshalIndent(corpus, "", "  ")
	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	os.WriteFile(irPath, irData, 0644)
	respond(&ExtractIRResult{IRPath: irPath, LossClass: "L2"})
}

func extractFLExContent(content, artifactID string) []*Document {
	doc := &Document{ID: artifactID, Title: artifactID, Order: 1}

	// Extract text from phrase elements
	phrasePattern := regexp.MustCompile(`<phrase[^>]*>([\s\S]*?)</phrase>`)
	wordPattern := regexp.MustCompile(`<item[^>]*type="txt"[^>]*>([^<]+)</item>`)

	sequence := 0
	for _, phraseMatch := range phrasePattern.FindAllStringSubmatch(content, -1) {
		if len(phraseMatch) < 2 {
			continue
		}
		phraseContent := phraseMatch[1]

		var words []string
		for _, wordMatch := range wordPattern.FindAllStringSubmatch(phraseContent, -1) {
			if len(wordMatch) >= 2 {
				words = append(words, strings.TrimSpace(wordMatch[1]))
			}
		}

		if len(words) > 0 {
			text := strings.Join(words, " ")
			sequence++
			hash := sha256.Sum256([]byte(text))
			cb := &ContentBlock{
				ID: fmt.Sprintf("cb-%d", sequence), Sequence: sequence, Text: text,
				Hash: hex.EncodeToString(hash[:]),
			}
			doc.ContentBlocks = append(doc.ContentBlocks, cb)
		}
	}

	// If no phrases, try simpler extraction
	if len(doc.ContentBlocks) == 0 {
		txtPattern := regexp.MustCompile(`<item[^>]*type="txt"[^>]*>([^<]+)</item>`)
		for _, match := range txtPattern.FindAllStringSubmatch(content, -1) {
			if len(match) >= 2 {
				text := strings.TrimSpace(match[1])
				if len(text) > 3 {
					sequence++
					hash := sha256.Sum256([]byte(text))
					doc.ContentBlocks = append(doc.ContentBlocks, &ContentBlock{
						ID: fmt.Sprintf("cb-%d", sequence), Sequence: sequence, Text: text,
						Hash: hex.EncodeToString(hash[:]),
					})
				}
			}
		}
	}

	return []*Document{doc}
}

func handleEmitNative(args map[string]interface{}) {
	irPath := args["ir_path"].(string)
	outputDir := args["output_dir"].(string)
	data, _ := os.ReadFile(irPath)
	var corpus Corpus
	json.Unmarshal(data, &corpus)
	outputPath := filepath.Join(outputDir, corpus.ID+".flextext")
	if raw, ok := corpus.Attributes["_flex_raw"]; ok && raw != "" {
		rawData, _ := hex.DecodeString(raw)
		os.WriteFile(outputPath, rawData, 0644)
		respond(&EmitNativeResult{OutputPath: outputPath, Format: "FLEx", LossClass: "L0"})
		return
	}
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString("\n<document>\n")
	buf.WriteString("  <interlinear-text>\n")
	buf.WriteString("    <paragraphs>\n")
	for _, doc := range corpus.Documents {
		buf.WriteString("      <paragraph>\n")
		buf.WriteString("        <phrases>\n")
		for _, cb := range doc.ContentBlocks {
			buf.WriteString("          <phrase>\n")
			buf.WriteString("            <words>\n")
			words := strings.Fields(cb.Text)
			for _, word := range words {
				fmt.Fprintf(&buf, "              <word>\n")
				fmt.Fprintf(&buf, "                <item type=\"txt\">%s</item>\n", escapeXML(word))
				fmt.Fprintf(&buf, "              </word>\n")
			}
			buf.WriteString("            </words>\n")
			buf.WriteString("          </phrase>\n")
		}
		buf.WriteString("        </phrases>\n")
		buf.WriteString("      </paragraph>\n")
	}
	buf.WriteString("    </paragraphs>\n")
	buf.WriteString("  </interlinear-text>\n")
	buf.WriteString("</document>\n")
	os.WriteFile(outputPath, buf.Bytes(), 0644)
	respond(&EmitNativeResult{OutputPath: outputPath, Format: "FLEx", LossClass: "L2"})
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func respond(result interface{}) {
	json.NewEncoder(os.Stdout).Encode(IPCResponse{Status: "ok", Result: result})
}

func respondError(msg string) {
	json.NewEncoder(os.Stdout).Encode(IPCResponse{Status: "error", Error: msg})
	os.Exit(1)
}
