// Package main implements a pure Go SWORD module parser plugin using the SDK pattern.
// This plugin provides SWORD module parsing without CGO dependencies.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

func main() {
	if err := format.Run(&format.Config{
		Name:       "SWORD",
		Extensions: []string{},
		Detect:     detectFunc,
		Parse:      parseFunc,
		Emit:       emitFunc,
		Enumerate:  enumerateFunc,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// detectFunc performs SWORD module detection
func detectFunc(path string) (*ipc.DetectResult, error) {
	detected, formatStr, err := Detect(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("detection error: %v", err),
		}, nil
	}

	if !detected {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not a SWORD module installation",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: true,
		Format:   formatStr,
		Reason:   "SWORD module installation detected",
	}, nil
}

// parseFunc parses a SWORD module and returns an IR Corpus
func parseFunc(path string) (*ir.Corpus, error) {
	// Load modules from the SWORD installation path
	confs, err := LoadModulesFromPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load modules: %w", err)
	}

	if len(confs) == 0 {
		return nil, fmt.Errorf("no modules found at path: %s", path)
	}

	// For now, parse the first Bible module we find
	// TODO: Support selecting specific modules or parsing multiple modules
	var selectedConf *ConfFile
	for _, conf := range confs {
		// Skip encrypted modules
		if conf.IsEncrypted() {
			continue
		}

		// Only handle Bible modules for now
		if conf.ModuleType() == "Bible" && conf.IsCompressed() {
			selectedConf = conf
			break
		}
	}

	if selectedConf == nil {
		return nil, fmt.Errorf("no supported Bible modules found (need unencrypted zText)")
	}

	// Open the zText module
	zt, err := OpenZTextModule(selectedConf, path)
	if err != nil {
		return nil, fmt.Errorf("failed to open module: %w", err)
	}

	// Extract corpus with full verse text
	corpus, _, err := extractCorpus(zt, selectedConf)
	if err != nil {
		return nil, fmt.Errorf("failed to extract corpus: %w", err)
	}

	// Convert local IR types to SDK IR types
	sdkCorpus := convertToSDKCorpus(corpus)

	return sdkCorpus, nil
}

// emitFunc converts an IR Corpus to SWORD format
func emitFunc(corpus *ir.Corpus, outputDir string) (string, error) {
	// Convert SDK IR types back to local IR types
	localCorpus := convertFromSDKCorpus(corpus)

	// Use EmitZText for full binary generation
	result, err := EmitZText(localCorpus, outputDir)
	if err != nil {
		return "", fmt.Errorf("failed to emit zText: %w", err)
	}

	return result.ConfPath, nil
}

// enumerateFunc lists modules in a SWORD installation
func enumerateFunc(path string) (*ipc.EnumerateResult, error) {
	confs, err := LoadModulesFromPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load modules: %w", err)
	}

	var entries []ipc.EnumerateEntry
	for _, conf := range confs {
		// Get module directory size if possible
		dataPath := conf.DataPath
		if !filepath.IsAbs(dataPath) {
			dataPath = filepath.Join(path, dataPath)
		}

		var sizeBytes int64
		if info, err := os.Stat(dataPath); err == nil {
			if !info.IsDir() {
				sizeBytes = info.Size()
			}
		}

		entries = append(entries, ipc.EnumerateEntry{
			Path:      conf.ModuleName,
			SizeBytes: sizeBytes,
			IsDir:     false,
			Metadata: map[string]string{
				"description": conf.Description,
				"type":        conf.ModuleType(),
				"language":    conf.Lang,
				"version":     conf.Version,
				"format":      conf.ModDrv,
				"compressed":  fmt.Sprintf("%t", conf.IsCompressed()),
				"encrypted":   fmt.Sprintf("%t", conf.IsEncrypted()),
			},
		})
	}

	return &ipc.EnumerateResult{
		Entries: entries,
	}, nil
}

// convertToSDKCorpus converts local IRCorpus to SDK Corpus
func convertToSDKCorpus(local *IRCorpus) *ir.Corpus {
	corpus := &ir.Corpus{
		ID:            local.ID,
		Version:       local.Version,
		ModuleType:    local.ModuleType,
		Versification: local.Versification,
		Language:      local.Language,
		Title:         local.Title,
		SourceHash:    local.SourceHash,
		LossClass:     local.LossClass,
		SourceFormat:  "SWORD",
		Attributes:    make(map[string]string),
	}

	// Copy attributes
	for k, v := range local.Attributes {
		corpus.Attributes[k] = v
	}

	// Convert documents
	for _, localDoc := range local.Documents {
		doc := &ir.Document{
			ID:         localDoc.ID,
			Title:      localDoc.Title,
			Order:      localDoc.Order,
			Attributes: make(map[string]string),
		}

		// Convert content blocks
		for _, localBlock := range localDoc.ContentBlocks {
			block := &ir.ContentBlock{
				ID:       localBlock.ID,
				Sequence: localBlock.Sequence,
				Text:     localBlock.Text,
				Hash:     localBlock.Hash,
				Attributes: map[string]interface{}{
					"raw_markup": localBlock.RawMarkup,
				},
			}

			// Convert tokens to anchors/spans
			// SWORD uses inline annotations, but SDK uses stand-off markup
			// We'll store tokens as attributes for now
			if len(localBlock.Tokens) > 0 {
				// Store token data in attributes for preservation
				tokenData := make([]map[string]interface{}, len(localBlock.Tokens))
				for i, token := range localBlock.Tokens {
					tokenData[i] = map[string]interface{}{
						"id":         token.ID,
						"index":      token.Index,
						"char_start": token.CharStart,
						"char_end":   token.CharEnd,
						"text":       token.Text,
						"type":       token.Type,
						"lemma":      token.Lemma,
						"strongs":    token.Strongs,
						"morphology": token.Morphology,
					}
				}
				block.Attributes["tokens"] = tokenData
			}

			// Convert annotations
			if len(localBlock.Annotations) > 0 {
				annotData := make([]map[string]interface{}, len(localBlock.Annotations))
				for i, annot := range localBlock.Annotations {
					annotData[i] = map[string]interface{}{
						"id":         annot.ID,
						"type":       annot.Type,
						"start_pos":  annot.StartPos,
						"end_pos":    annot.EndPos,
						"value":      annot.Value,
						"confidence": annot.Confidence,
					}
				}
				block.Attributes["annotations"] = annotData
			}

			doc.ContentBlocks = append(doc.ContentBlocks, block)
		}

		corpus.Documents = append(corpus.Documents, doc)
	}

	return corpus
}

// convertFromSDKCorpus converts SDK Corpus back to local IRCorpus
func convertFromSDKCorpus(sdk *ir.Corpus) *IRCorpus {
	corpus := &IRCorpus{
		ID:            sdk.ID,
		Version:       sdk.Version,
		ModuleType:    sdk.ModuleType,
		Versification: sdk.Versification,
		Language:      sdk.Language,
		Title:         sdk.Title,
		SourceHash:    sdk.SourceHash,
		LossClass:     sdk.LossClass,
		Attributes:    make(map[string]string),
	}

	// Copy attributes
	for k, v := range sdk.Attributes {
		corpus.Attributes[k] = v
	}

	// Convert documents
	for _, sdkDoc := range sdk.Documents {
		doc := &IRDocument{
			ID:    sdkDoc.ID,
			Title: sdkDoc.Title,
			Order: sdkDoc.Order,
		}

		// Convert content blocks
		for _, sdkBlock := range sdkDoc.ContentBlocks {
			block := &IRContentBlock{
				ID:       sdkBlock.ID,
				Sequence: sdkBlock.Sequence,
				Text:     sdkBlock.Text,
				Hash:     sdkBlock.Hash,
			}

			// Restore raw markup from attributes
			if sdkBlock.Attributes != nil {
				if rawMarkup, ok := sdkBlock.Attributes["raw_markup"].(string); ok {
					block.RawMarkup = rawMarkup
				}

				// Restore tokens from attributes
				if tokenData, ok := sdkBlock.Attributes["tokens"].([]map[string]interface{}); ok {
					for _, td := range tokenData {
						token := &IRToken{
							ID:    getStringField(td, "id"),
							Index: getIntField(td, "index"),
							CharStart: getIntField(td, "char_start"),
							CharEnd:   getIntField(td, "char_end"),
							Text:      getStringField(td, "text"),
							Type:      getStringField(td, "type"),
							Lemma:     getStringField(td, "lemma"),
							Morphology: getStringField(td, "morphology"),
						}
						if strongs, ok := td["strongs"].([]string); ok {
							token.Strongs = strongs
						}
						block.Tokens = append(block.Tokens, token)
					}
				}

				// Restore annotations from attributes
				if annotData, ok := sdkBlock.Attributes["annotations"].([]map[string]interface{}); ok {
					for _, ad := range annotData {
						annot := &IRAnnotation{
							ID:       getStringField(ad, "id"),
							Type:     getStringField(ad, "type"),
							StartPos: getIntField(ad, "start_pos"),
							EndPos:   getIntField(ad, "end_pos"),
							Value:    getStringField(ad, "value"),
							Confidence: getFloat64Field(ad, "confidence"),
						}
						block.Annotations = append(block.Annotations, annot)
					}
				}
			}

			doc.ContentBlocks = append(doc.ContentBlocks, block)
		}

		corpus.Documents = append(corpus.Documents, doc)
	}

	return corpus
}

// Helper functions for type conversion
func getStringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getIntField(m map[string]interface{}, key string) int {
	if v, ok := m[key].(int); ok {
		return v
	}
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

func getFloat64Field(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	if v, ok := m[key].(int); ok {
		return float64(v)
	}
	return 0.0
}
