//go:build sdk

// Plugin format-na28app handles NA28 critical apparatus format using the SDK pattern.
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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

func main() {
	if err := format.Run(&format.Config{
		Name:       "NA28App",
		Extensions: []string{".na28", ".apparatus", ".app"},
		Detect:     detectNA28,
		Parse:      parseNA28,
		Emit:       emitNA28,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// detectNA28 checks if the file is a NA28 apparatus format
func detectNA28(path string) (*ipc.DetectResult, error) {
	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".na28" || ext == ".apparatus" || ext == ".app" {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "NA28App",
			Reason:   "NA28 apparatus file extension detected",
		}, nil
	}

	// Check file content for NA28 apparatus markers
	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not a NA28 apparatus file",
		}, nil
	}

	content := string(data)

	// NA28 apparatus typically contains:
	// - Textual variant markers
	// - Manuscript witness sigla (e.g., ℵ א A B C D)
	// - Critical apparatus notation
	if containsNA28Markers(content) {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "NA28App",
			Reason:   "NA28 apparatus structure detected",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "not a NA28 apparatus file",
	}, nil
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

// parseNA28 converts NA28 apparatus format to IR
func parseNA28(path string) (*ir.Corpus, error) {
	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	content := string(data)

	// Create a basic corpus structure
	corpus := &ir.Corpus{
		ID:            "na28app",
		Version:       "1.0",
		ModuleType:    "apparatus",
		Versification: "NA28",
		Language:      "grc",
		Title:         "Nestle-Aland 28th Edition Critical Apparatus",
		Publisher:     "Deutsche Bibelgesellschaft",
		SourceFormat:  "NA28App",
		SourceHash:    hex.EncodeToString(sourceHash[:]),
		LossClass:     "L1",
		Attributes: map[string]string{
			"edition": "NA28",
			"type":    "critical-apparatus",
		},
	}

	// Parse the content into documents
	// For now, create a single document with the apparatus content
	doc := &ir.Document{
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

	corpus.Documents = []*ir.Document{doc}
	return corpus, nil
}

func parseApparatusLine(line string, seq int) *ir.ContentBlock {
	// Hash the content
	hash := sha256.Sum256([]byte(line))
	hashStr := hex.EncodeToString(hash[:])

	block := &ir.ContentBlock{
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

func parseVariants(block *ir.ContentBlock, line string) {
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

// emitNA28 converts IR back to NA28 apparatus format
func emitNA28(corpus *ir.Corpus, outputDir string) (string, error) {
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
	if err := os.WriteFile(outputPath, buf.Bytes(), 0600); err != nil {
		return "", fmt.Errorf("failed to write output: %w", err)
	}

	return outputPath, nil
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
