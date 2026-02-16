//go:build !sdk

// Plugin format-flex handles FLEx/Fieldworks linguistic database format.
// FLEx (FieldWorks Language Explorer) is a linguistic database for language documentation
// and analysis, containing lexical, grammatical, and text corpus data.
//
// IR Support:
// - extract-ir: Reads FLEx format to IR (L2)
// - emit-native: Converts IR to FLEx format (L2 or L0 with raw storage)
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

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

func main() {
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorf("failed to decode: %v", err)
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
		ipc.RespondError("unknown command")
	}
}

func handleDetect(args map[string]interface{}) {
	path := args["path"].(string)
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".flextext" || ext == ".fwbackup" || ext == ".fwdata" {
		ipc.MustRespond(&ipc.DetectResult{Detected: true, Format: "FLEx", Reason: "FLEx extension"})
		return
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	// FLEx XML patterns
	flexPattern := regexp.MustCompile(`<(document|interlinear-text|paragraphs|phrase)[^>]*>`)
	if flexPattern.MatchString(content) && (strings.Contains(content, "flextext") || strings.Contains(content, "FieldWorks")) {
		ipc.MustRespond(&ipc.DetectResult{Detected: true, Format: "FLEx", Reason: "FLEx XML structure"})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{Detected: false, Reason: "not FLEx"})
}

func handleIngest(args map[string]interface{}) {
	path := args["path"].(string)
	outputDir := args["output_dir"].(string)
	data, _ := os.ReadFile(path)
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])
	blobDir := filepath.Join(outputDir, hashHex[:2])
	os.MkdirAll(blobDir, 0755)
	os.WriteFile(filepath.Join(blobDir, hashHex), data, 0600)
	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata:   map[string]string{"format": "FLEx"},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path := args["path"].(string)
	info, _ := os.Stat(path)
	ipc.MustRespond(&ipc.EnumerateResult{Entries: []ipc.EnumerateEntry{{Path: filepath.Base(path), SizeBytes: info.Size()}}})
}

func handleExtractIR(args map[string]interface{}) {
	path := args["path"].(string)
	outputDir := args["output_dir"].(string)
	data, _ := os.ReadFile(path)
	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ipc.Corpus{
		ID: artifactID, Version: "1.0.0", ModuleType: "BIBLE",
		Title: artifactID, SourceFormat: "FLEx", SourceHash: hex.EncodeToString(sourceHash[:]),
		LossClass: "L2", Attributes: map[string]string{"_flex_raw": hex.EncodeToString(data)},
	}
	corpus.Documents = extractFLExContent(string(data), artifactID)
	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ipc.Document{{ID: artifactID, Title: artifactID, Order: 1}}
	}
	irData, _ := json.MarshalIndent(corpus, "", "  ")
	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	os.WriteFile(irPath, irData, 0600)
	ipc.MustRespond(&ipc.ExtractIRResult{IRPath: irPath, LossClass: "L2"})
}

func extractFLExContent(content, artifactID string) []*ipc.Document {
	doc := &ipc.Document{ID: artifactID, Title: artifactID, Order: 1}

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
			cb := &ipc.ContentBlock{
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
					doc.ContentBlocks = append(doc.ContentBlocks, &ipc.ContentBlock{
						ID: fmt.Sprintf("cb-%d", sequence), Sequence: sequence, Text: text,
						Hash: hex.EncodeToString(hash[:]),
					})
				}
			}
		}
	}

	return []*ipc.Document{doc}
}

func handleEmitNative(args map[string]interface{}) {
	irPath := args["ir_path"].(string)
	outputDir := args["output_dir"].(string)
	data, _ := os.ReadFile(irPath)
	var corpus ipc.Corpus
	json.Unmarshal(data, &corpus)
	outputPath := filepath.Join(outputDir, corpus.ID+".flextext")
	if raw, ok := corpus.Attributes["_flex_raw"]; ok && raw != "" {
		rawData, _ := hex.DecodeString(raw)
		os.WriteFile(outputPath, rawData, 0600)
		ipc.MustRespond(&ipc.EmitNativeResult{OutputPath: outputPath, Format: "FLEx", LossClass: "L0"})
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
	os.WriteFile(outputPath, buf.Bytes(), 0600)
	ipc.MustRespond(&ipc.EmitNativeResult{OutputPath: outputPath, Format: "FLEx", LossClass: "L2"})
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
