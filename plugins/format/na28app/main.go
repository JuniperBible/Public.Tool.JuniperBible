//go:build !sdk

// Plugin format-na28app handles NA28 critical apparatus format.
// NA28 (Nestle-Aland 28th edition) apparatus contains textual variants
// and manuscript witnesses for the Greek New Testament.
//
// IR Support:
// - extract-ir: Reads NA28 apparatus to IR (L1)
// - emit-native: Converts IR to NA28 apparatus format (L1)
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

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

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

	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".na28" || ext == ".apparatus" || ext == ".app" {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "NA28App",
			Reason:   "NA28 apparatus file extension detected",
		})
		return
	}

	// Check file content for NA28 apparatus markers
	data, err := safefile.ReadFile(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not a NA28 apparatus file",
		})
		return
	}

	content := string(data)

	// NA28 apparatus typically contains:
	// - Textual variant markers
	// - Manuscript witness sigla (e.g., ℵ א A B C D)
	// - Critical apparatus notation
	if containsNA28Markers(content) {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "NA28App",
			Reason:   "NA28 apparatus structure detected",
		})
		return
	}

	ipc.MustRespond(&ipc.DetectResult{
		Detected: false,
		Reason:   "not a NA28 apparatus file",
	})
}

func containsNA28Markers(content string) bool {
	// Look for typical NA28 apparatus markers:
	// - Manuscript sigla patterns
	// - Variant reading markers
	// - Critical apparatus notation
	markers := []string{
		"<app>",     // TEI-based apparatus
		"<rdg",      // Reading element
		"<wit>",     // Witness element
		"txt",       // Text reading marker
		"vid",       // Videtur (seemingly)
		"*",         // Corrector marks
		"א",         // Aleph (Sinaiticus)
		"ℵ",         // Alternative Aleph
		"apparatus", // Generic apparatus marker
	}

	for _, marker := range markers {
		if strings.Contains(content, marker) {
			return true
		}
	}

	return false
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	casDir, ok := args["cas_dir"].(string)
	if !ok {
		ipc.RespondError("cas_dir argument required")
		return
	}

	// Read the file
	data, err := safefile.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	// Store in CAS
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	// Store the blob
	blobPath := filepath.Join(casDir, "blobs", hashStr[:2], hashStr[2:4], hashStr)
	if err := os.MkdirAll(filepath.Dir(blobPath), 0755); err != nil {
		ipc.RespondErrorf("failed to create blob directory: %v", err)
		return
	}

	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		ipc.RespondErrorf("failed to write blob: %v", err)
		return
	}

	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: "na28app",
		BlobSHA256: hashStr,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "NA28App",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	blobHash, ok := args["blob_hash"].(string)
	if !ok {
		ipc.RespondError("blob_hash argument required")
		return
	}

	casDir, ok := args["cas_dir"].(string)
	if !ok {
		ipc.RespondError("cas_dir argument required")
		return
	}

	// Read the blob
	blobPath := filepath.Join(casDir, "blobs", blobHash[:2], blobHash[2:4], blobHash)
	data, err := safefile.ReadFile(blobPath)
	if err != nil {
		ipc.RespondErrorf("failed to read blob: %v", err)
		return
	}

	// For NA28 apparatus, we return a single entry
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: []ipc.EnumerateEntry{
			{
				Path:      "apparatus.na28",
				SizeBytes: int64(len(data)),
				IsDir:     false,
				Metadata: map[string]string{
					"media_type": "application/xml+na28",
					"blob_hash":  blobHash,
				},
			},
		},
	})
}

func handleExtractIR(args map[string]interface{}) {
	blobHash, ok := args["blob_hash"].(string)
	if !ok {
		ipc.RespondError("blob_hash argument required")
		return
	}

	casDir, ok := args["cas_dir"].(string)
	if !ok {
		ipc.RespondError("cas_dir argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	// Read the blob
	blobPath := filepath.Join(casDir, "blobs", blobHash[:2], blobHash[2:4], blobHash)
	data, err := safefile.ReadFile(blobPath)
	if err != nil {
		ipc.RespondErrorf("failed to read blob: %v", err)
		return
	}

	// Parse NA28 apparatus to IR
	corpus := parseNA28ToIR(string(data), blobHash)

	// Write IR to output
	irPath := filepath.Join(outputDir, "corpus.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to marshal IR: %v", err)
		return
	}

	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}

	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
	})
}

func parseNA28ToIR(content string, sourceHash string) *ipc.Corpus {
	// Create a basic corpus structure
	corpus := &ipc.Corpus{
		ID:            "na28app",
		Version:       "1.0",
		ModuleType:    "apparatus",
		Versification: "NA28",
		Language:      "grc",
		Title:         "Nestle-Aland 28th Edition Critical Apparatus",
		Publisher:     "Deutsche Bibelgesellschaft",
		SourceFormat:  "NA28App",
		SourceHash:    sourceHash,
		LossClass:     "L1",
		Attributes: map[string]string{
			"edition": "NA28",
			"type":    "critical-apparatus",
		},
	}

	// Parse the content into documents
	// For now, create a single document with the apparatus content
	doc := &ipc.Document{
		ID:    "apparatus",
		Title: "Critical Apparatus",
		Order: 1,
		Attributes: map[string]string{
			"type": "apparatus",
		},
	}

	// Parse content into content blocks
	// Simple line-based parsing for now
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		block := parseApparatusLine(line, i)
		doc.ContentBlocks = append(doc.ContentBlocks, block)
	}

	corpus.Documents = []*ipc.Document{doc}
	return corpus
}

func parseApparatusLine(line string, seq int) *ipc.ContentBlock {
	// Hash the content
	hash := sha256.Sum256([]byte(line))
	hashStr := hex.EncodeToString(hash[:])

	block := &ipc.ContentBlock{
		ID:       fmt.Sprintf("block-%d", seq),
		Sequence: seq,
		Text:     line,
		Hash:     hashStr,
		Attributes: map[string]interface{}{
			"type": "apparatus-entry",
		},
	}

	// Parse textual variants and witnesses
	parseVariants(block, line)

	return block
}

func parseVariants(block *ipc.ContentBlock, line string) {
	// Extract reference if present (e.g., "Matt.1.1")
	refPattern := regexp.MustCompile(`([A-Za-z0-9]+)\.(\d+)\.(\d+)`)
	if matches := refPattern.FindStringSubmatch(line); len(matches) > 0 {
		if block.Attributes == nil {
			block.Attributes = make(map[string]interface{})
		}
		block.Attributes["reference"] = matches[0]
		block.Attributes["book"] = matches[1]
		block.Attributes["chapter"] = matches[2]
		block.Attributes["verse"] = matches[3]
	}

	// Extract manuscript witnesses (common sigla)
	witnessPattern := regexp.MustCompile(`[א-ת]|[A-Z]\d*|ℵ|P\d+`)
	witnesses := witnessPattern.FindAllString(line, -1)
	if len(witnesses) > 0 {
		if block.Attributes == nil {
			block.Attributes = make(map[string]interface{})
		}
		block.Attributes["witnesses"] = witnesses
	}
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

	// Read the IR
	irData, err := safefile.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR: %v", err)
		return
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		ipc.RespondErrorf("failed to unmarshal IR: %v", err)
		return
	}

	// Convert IR to NA28 apparatus format
	var buf bytes.Buffer
	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buf.WriteString("<apparatus edition=\"NA28\">\n")

	for _, doc := range corpus.Documents {
		for _, block := range doc.ContentBlocks {
			// Write apparatus entry
			buf.WriteString("  <entry")

			// Add reference if present
			if ref, ok := block.Attributes["reference"].(string); ok {
				buf.WriteString(fmt.Sprintf(" ref=\"%s\"", ref))
			}

			buf.WriteString(">\n")
			buf.WriteString(fmt.Sprintf("    <text>%s</text>\n", xmlEscape(block.Text)))

			// Add witnesses if present
			if witnesses, ok := block.Attributes["witnesses"].([]interface{}); ok {
				buf.WriteString("    <witnesses>\n")
				for _, w := range witnesses {
					if ws, ok := w.(string); ok {
						buf.WriteString(fmt.Sprintf("      <wit>%s</wit>\n", xmlEscape(ws)))
					}
				}
				buf.WriteString("    </witnesses>\n")
			}

			buf.WriteString("  </entry>\n")
		}
	}

	buf.WriteString("</apparatus>\n")

	// Write output
	outputPath := filepath.Join(outputDir, "apparatus.na28.xml")
	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		ipc.RespondErrorf("failed to write output: %v", err)
		return
	}

	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "NA28App",
		LossClass:  "L1",
	})
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
