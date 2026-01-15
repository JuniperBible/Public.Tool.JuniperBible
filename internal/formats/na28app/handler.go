// Package na28app provides the embedded handler for NA28 critical apparatus format.
// NA28 (Nestle-Aland 28th edition) apparatus contains textual variants
// and manuscript witnesses for the Greek New Testament.
package na28app

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

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// Handler implements the EmbeddedFormatHandler interface for NA28 apparatus format.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.na28app",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-na28app",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"file"},
			Outputs: []string{"artifact.kind:na28app"},
		},
	}
}

// Register registers this plugin with the embedded registry.
func Register() {
	plugins.RegisterEmbeddedPlugin(&plugins.EmbeddedPlugin{
		Manifest: Manifest(),
		Format:   &Handler{},
	})
}

func init() {
	Register()
}

// Detect implements EmbeddedFormatHandler.Detect.
func (h *Handler) Detect(path string) (*plugins.DetectResult, error) {
	// Check if file exists first
	info, err := os.Stat(path)
	if err != nil {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		}, nil
	}

	if info.IsDir() {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   "path is a directory",
		}, nil
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".na28" || ext == ".apparatus" || ext == ".app" {
		return &plugins.DetectResult{
			Detected: true,
			Format:   "NA28App",
			Reason:   "NA28 apparatus file extension detected",
		}, nil
	}

	// Check file content for NA28 apparatus markers
	data, err := os.ReadFile(path)
	if err != nil {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   "not a NA28 apparatus file",
		}, nil
	}

	content := string(data)

	if containsNA28Markers(content) {
		return &plugins.DetectResult{
			Detected: true,
			Format:   "NA28App",
			Reason:   "NA28 apparatus structure detected",
		}, nil
	}

	return &plugins.DetectResult{
		Detected: false,
		Reason:   "not a NA28 apparatus file",
	}, nil
}

func containsNA28Markers(content string) bool {
	// Look for typical NA28 apparatus markers
	// Use more specific markers to avoid false positives
	markers := []string{
		"<app>",
		"<rdg",
		"<wit>",
		"א", // Hebrew Aleph (Sinaiticus)
		"ℵ", // Alternative Aleph symbol
	}

	for _, marker := range markers {
		if strings.Contains(content, marker) {
			return true
		}
	}

	// Check for apparatus-specific patterns (avoiding common words)
	// Look for patterns like "txt " or " vid " with spaces to avoid false positives
	apparatusPatterns := []string{
		" txt ",
		" vid ",
		"*txt",
		"*vid",
	}

	for _, pattern := range apparatusPatterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}

	return false
}

// Ingest implements EmbeddedFormatHandler.Ingest.
func (h *Handler) Ingest(path, outputDir string) (*plugins.IngestResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	blobPath := filepath.Join(outputDir, hashStr[:2], hashStr)
	if err := os.MkdirAll(filepath.Dir(blobPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create blob directory: %w", err)
	}

	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write blob: %w", err)
	}

	return &plugins.IngestResult{
		ArtifactID: "na28app",
		BlobSHA256: hashStr,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "NA28App",
		},
	}, nil
}

// Enumerate implements EmbeddedFormatHandler.Enumerate.
func (h *Handler) Enumerate(path string) (*plugins.EnumerateResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	return &plugins.EnumerateResult{
		Entries: []plugins.EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
			},
		},
	}, nil
}

// ExtractIR implements EmbeddedFormatHandler.ExtractIR.
func (h *Handler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse NA28 apparatus to IR
	corpus := parseNA28ToIR(string(data), data)

	// Write IR to output
	irPath := filepath.Join(outputDir, "corpus.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal IR: %w", err)
	}

	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write IR: %w", err)
	}

	return &plugins.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1",
	}, nil
}

func parseNA28ToIR(content string, sourceData []byte) map[string]interface{} {
	hash := sha256.Sum256(sourceData)
	hashHex := hex.EncodeToString(hash[:])

	corpus := map[string]interface{}{
		"id":            "na28app",
		"version":       "1.0",
		"module_type":   "apparatus",
		"versification": "NA28",
		"language":      "grc",
		"title":         "Nestle-Aland 28th Edition Critical Apparatus",
		"publisher":     "Deutsche Bibelgesellschaft",
		"source_format": "NA28App",
		"source_hash":   hashHex,
		"loss_class":    "L1",
		"attributes": map[string]string{
			"edition": "NA28",
			"type":    "critical-apparatus",
		},
	}

	// Parse content into documents
	doc := map[string]interface{}{
		"id":    "apparatus",
		"title": "Critical Apparatus",
		"order": 1,
		"attributes": map[string]string{
			"type": "apparatus",
		},
	}

	// Parse content into content blocks
	var contentBlocks []map[string]interface{}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		block := parseApparatusLine(line, i)
		contentBlocks = append(contentBlocks, block)
	}

	doc["content_blocks"] = contentBlocks
	corpus["documents"] = []map[string]interface{}{doc}

	return corpus
}

func parseApparatusLine(line string, seq int) map[string]interface{} {
	hash := sha256.Sum256([]byte(line))
	hashStr := hex.EncodeToString(hash[:])

	block := map[string]interface{}{
		"id":       fmt.Sprintf("block-%d", seq),
		"sequence": seq,
		"text":     line,
		"hash":     hashStr,
	}

	attrs := map[string]interface{}{
		"type": "apparatus-entry",
	}

	// Extract reference if present
	refPattern := regexp.MustCompile(`([A-Za-z0-9]+)\.(\d+)\.(\d+)`)
	if matches := refPattern.FindStringSubmatch(line); len(matches) > 0 {
		attrs["reference"] = matches[0]
		attrs["book"] = matches[1]
		attrs["chapter"] = matches[2]
		attrs["verse"] = matches[3]
	}

	// Extract manuscript witnesses
	witnessPattern := regexp.MustCompile(`[א-ת]|[A-Z]\d*|ℵ|P\d+`)
	witnesses := witnessPattern.FindAllString(line, -1)
	if len(witnesses) > 0 {
		attrs["witnesses"] = witnesses
	}

	block["attributes"] = attrs
	return block
}

// EmitNative implements EmbeddedFormatHandler.EmitNative.
func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	irData, err := os.ReadFile(irPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read IR: %w", err)
	}

	var corpus map[string]interface{}
	if err := json.Unmarshal(irData, &corpus); err != nil {
		return nil, fmt.Errorf("failed to unmarshal IR: %w", err)
	}

	// Convert IR to NA28 apparatus format
	var buf bytes.Buffer
	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buf.WriteString("<apparatus edition=\"NA28\">\n")

	if docs, ok := corpus["documents"].([]interface{}); ok {
		for _, docIface := range docs {
			if doc, ok := docIface.(map[string]interface{}); ok {
				if blocks, ok := doc["content_blocks"].([]interface{}); ok {
					for _, blockIface := range blocks {
						if block, ok := blockIface.(map[string]interface{}); ok {
							// Write apparatus entry
							buf.WriteString("  <entry")

							if attrs, ok := block["attributes"].(map[string]interface{}); ok {
								if ref, ok := attrs["reference"].(string); ok {
									buf.WriteString(fmt.Sprintf(" ref=\"%s\"", ref))
								}
							}

							buf.WriteString(">\n")

							if text, ok := block["text"].(string); ok {
								buf.WriteString(fmt.Sprintf("    <text>%s</text>\n", xmlEscape(text)))
							}

							// Add witnesses if present
							if attrs, ok := block["attributes"].(map[string]interface{}); ok {
								if witnesses, ok := attrs["witnesses"].([]interface{}); ok {
									buf.WriteString("    <witnesses>\n")
									for _, w := range witnesses {
										if ws, ok := w.(string); ok {
											buf.WriteString(fmt.Sprintf("      <wit>%s</wit>\n", xmlEscape(ws)))
										}
									}
									buf.WriteString("    </witnesses>\n")
								}
							}

							buf.WriteString("  </entry>\n")
						}
					}
				}
			}
		}
	}

	buf.WriteString("</apparatus>\n")

	// Write output
	outputPath := filepath.Join(outputDir, "apparatus.na28.xml")
	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return nil, fmt.Errorf("failed to write output: %w", err)
	}

	return &plugins.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "NA28App",
		LossClass:  "L1",
	}, nil
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
