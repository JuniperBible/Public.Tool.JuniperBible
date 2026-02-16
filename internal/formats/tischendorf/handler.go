// Package tischendorf provides the embedded handler for Tischendorf 8th edition critical apparatus format.
// Tischendorf's Novum Testamentum Graece (8th edition) is a historical critical text
// with extensive apparatus criticus documenting textual variants.
package tischendorf

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

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// Handler implements the EmbeddedFormatHandler interface for Tischendorf format.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.tischendorf",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-tischendorf",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"file"},
			Outputs: []string{"artifact.kind:tischendorf"},
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
	data, err := os.ReadFile(path)
	if err != nil {
		return &plugins.DetectResult{Detected: false, Reason: "cannot read file"}, nil
	}

	content := string(data)

	hasGreek := regexp.MustCompile(`[\p{Greek}]+`).MatchString(content)
	hasApparatus := strings.Contains(content, "[") || strings.Contains(content, "]")
	hasVerseRefWithBook := regexp.MustCompile(`(?i)(Matt|Mark|Luke|John|Rom|Cor|Gal|Eph|Phil|Col|Thess|Tim|Tit|Philem|Heb|James|Pet|Rev)\s+\d+:\d+`).MatchString(content)
	hasVerseRefStandalone := regexp.MustCompile(`(?m)^\d+:\d+\s`).MatchString(content)
	hasVerseRef := hasVerseRefWithBook || hasVerseRefStandalone

	if hasGreek && hasApparatus && hasVerseRef {
		return &plugins.DetectResult{
			Detected: true,
			Format:   "tischendorf",
			Reason:   "detected Greek text with critical apparatus and verse references",
		}, nil
	}

	return &plugins.DetectResult{Detected: false, Reason: "not a Tischendorf format file"}, nil
}

// Ingest implements EmbeddedFormatHandler.Ingest.
func (h *Handler) Ingest(path, outputDir string) (*plugins.IngestResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create blob dir: %w", err)
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write blob: %w", err)
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	return &plugins.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format":   "tischendorf",
			"encoding": "utf-8",
		},
	}, nil
}

// Enumerate implements EmbeddedFormatHandler.Enumerate.
func (h *Handler) Enumerate(path string) (*plugins.EnumerateResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
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

	corpus := parseTischendorfToIR(data)

	// Compute source hash
	hash := sha256.Sum256(data)
	corpus["source_hash"] = hex.EncodeToString(hash[:])
	corpus["source_format"] = "tischendorf"
	corpus["loss_class"] = "L2"

	irPath := filepath.Join(outputDir, "corpus.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal IR: %w", err)
	}

	if err := os.WriteFile(irPath, irData, 0600); err != nil {
		return nil, fmt.Errorf("failed to write IR file: %w", err)
	}

	return &plugins.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L2",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "tischendorf",
			TargetFormat: "IR",
			LossClass:    "L2",
			Warnings: []string{
				"critical apparatus structure may be simplified",
				"manuscript sigla may be normalized",
			},
		},
	}, nil
}

func parseTischendorfToIR(data []byte) map[string]interface{} {
	corpus := map[string]interface{}{
		"id":            "tischendorf-nt",
		"version":       "8.0",
		"module_type":   "bible",
		"versification": "KJV",
		"language":      "grc",
		"title":         "Tischendorf Greek New Testament",
		"description":   "Critical edition with apparatus",
		"publisher":     "Constantin von Tischendorf",
		"attributes":    map[string]string{},
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var currentDoc map[string]interface{}
	var contentBlocks []map[string]interface{}
	sequence := 0
	docOrder := 0

	var documents []map[string]interface{}

	// Parse line by line
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Check for book/chapter headers
		if isBookHeader(line) {
			// Save previous document if exists
			if currentDoc != nil {
				currentDoc["content_blocks"] = contentBlocks
				documents = append(documents, currentDoc)
			}

			bookName := extractBookName(line)
			currentDoc = map[string]interface{}{
				"id":         bookName,
				"title":      bookName,
				"order":      docOrder,
				"attributes": map[string]string{},
			}
			contentBlocks = []map[string]interface{}{}
			docOrder++
			sequence = 0
			continue
		}

		// Parse verse content
		if currentDoc != nil {
			ref := extractReference(line)
			text := extractText(line)

			block := map[string]interface{}{
				"id":       fmt.Sprintf("block_%d", sequence),
				"sequence": sequence,
				"text":     text,
				"anchors":  []interface{}{},
				"attributes": map[string]interface{}{
					"verse_ref": ref,
				},
			}

			// Add verse span
			anchor := map[string]interface{}{
				"id":       fmt.Sprintf("anchor_%d_0", sequence),
				"position": 0,
				"spans": []map[string]interface{}{
					{
						"id":              fmt.Sprintf("span_%d_verse", sequence),
						"type":            "verse",
						"start_anchor_id": fmt.Sprintf("anchor_%d_0", sequence),
						"ref":             parseRefString(ref),
					},
				},
			}
			block["anchors"] = []interface{}{anchor}

			contentBlocks = append(contentBlocks, block)
			sequence++
		}
	}

	// Save last document
	if currentDoc != nil {
		currentDoc["content_blocks"] = contentBlocks
		documents = append(documents, currentDoc)
	}

	corpus["documents"] = documents
	return corpus
}

func isBookHeader(line string) bool {
	bookNames := []string{"Matthew", "Mark", "Luke", "John", "Acts", "Romans",
		"1 Corinthians", "2 Corinthians", "Galatians", "Ephesians", "Philippians",
		"Colossians", "1 Thessalonians", "2 Thessalonians", "1 Timothy", "2 Timothy",
		"Titus", "Philemon", "Hebrews", "James", "1 Peter", "2 Peter", "1 John",
		"2 John", "3 John", "Jude", "Revelation"}

	for _, name := range bookNames {
		if strings.Contains(line, name) {
			return true
		}
	}
	return false
}

func extractBookName(line string) string {
	parts := strings.Fields(line)
	if len(parts) > 0 {
		return parts[0]
	}
	return "Unknown"
}

func extractReference(line string) string {
	re := regexp.MustCompile(`(\d+):(\d+)`)
	if match := re.FindString(line); match != "" {
		return match
	}
	return ""
}

func extractText(line string) string {
	// Remove reference markers and extract Greek text
	// Strip apparatus markers in brackets
	text := regexp.MustCompile(`\[.*?\]`).ReplaceAllString(line, "")
	text = regexp.MustCompile(`\d+:\d+`).ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

func parseRefString(refStr string) map[string]interface{} {
	// Parse "chapter:verse" format
	parts := strings.Split(refStr, ":")
	if len(parts) != 2 {
		return map[string]interface{}{}
	}

	chapter, _ := strconv.Atoi(parts[0])
	verse, _ := strconv.Atoi(parts[1])

	return map[string]interface{}{
		"chapter": chapter,
		"verse":   verse,
	}
}

// EmitNative implements EmbeddedFormatHandler.EmitNative.
func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	irData, err := os.ReadFile(irPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read IR file: %w", err)
	}

	var corpus map[string]interface{}
	if err := json.Unmarshal(irData, &corpus); err != nil {
		return nil, fmt.Errorf("failed to unmarshal IR: %w", err)
	}

	output := emitTischendorfFromIR(corpus)

	outputPath := filepath.Join(outputDir, "tischendorf.txt")
	if err := os.WriteFile(outputPath, []byte(output), 0600); err != nil {
		return nil, fmt.Errorf("failed to write output: %w", err)
	}

	return &plugins.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "tischendorf",
		LossClass:  "L2",
		LossReport: &plugins.LossReportIPC{
			SourceFormat: "IR",
			TargetFormat: "tischendorf",
			LossClass:    "L2",
			Warnings: []string{
				"complex critical apparatus may be simplified",
				"manuscript variants not fully preserved",
			},
		},
	}, nil
}

func emitTischendorfFromIR(corpus map[string]interface{}) string {
	var buf bytes.Buffer

	// Write header
	if title, ok := corpus["title"].(string); ok {
		buf.WriteString(fmt.Sprintf("# %s\n", title))
	}
	if version, ok := corpus["version"].(string); ok {
		buf.WriteString(fmt.Sprintf("# Version: %s\n", version))
	}
	if language, ok := corpus["language"].(string); ok {
		buf.WriteString(fmt.Sprintf("# Language: %s\n\n", language))
	}

	if docs, ok := corpus["documents"].([]interface{}); ok {
		for _, docIface := range docs {
			if doc, ok := docIface.(map[string]interface{}); ok {
				if title, ok := doc["title"].(string); ok {
					buf.WriteString(fmt.Sprintf("## %s\n\n", title))
				}

				if blocks, ok := doc["content_blocks"].([]interface{}); ok {
					for _, blockIface := range blocks {
						if block, ok := blockIface.(map[string]interface{}); ok {
							verseRef := ""
							if attrs, ok := block["attributes"].(map[string]interface{}); ok {
								if v, ok := attrs["verse_ref"]; ok {
									verseRef = fmt.Sprintf("%v", v)
								}
							}

							text, _ := block["text"].(string)

							// Write verse with reference
							if verseRef != "" {
								buf.WriteString(fmt.Sprintf("%s %s\n", verseRef, text))
							} else {
								buf.WriteString(fmt.Sprintf("%s\n", text))
							}
						}
					}
				}
				buf.WriteString("\n")
			}
		}
	}

	return buf.String()
}
