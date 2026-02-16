//go:build !sdk

// Plugin format-onlinebible handles OnlineBible .ont format.
// OnlineBible uses a text-based format with structured verse references.
//
// IR Support:
// - extract-ir: Reads OnlineBible format to IR (L2)
// - emit-native: Converts IR to OnlineBible-compatible format (L2)
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

	ext := strings.ToLower(filepath.Ext(path))
	// OnlineBible uses .ont extension
	if ext == ".ont" {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "OnlineBible",
			Reason:   "OnlineBible file extension detected",
		})
		return
	}

	// Check for OnlineBible content structure
	data, err := os.ReadFile(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not an OnlineBible file",
		})
		return
	}

	content := string(data)
	// OnlineBible format typically has verse references like "Gen 1:1" or "$Gen 1:1"
	versePattern := regexp.MustCompile(`(?m)^\$?[A-Z][a-z]{1,2}\s+\d+:\d+`)
	if versePattern.MatchString(content) {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "OnlineBible",
			Reason:   "OnlineBible verse reference pattern detected",
		})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{
		Detected: false,
		Reason:   "no OnlineBible structure found",
	})
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		ipc.RespondErrorf("failed to create blob dir: %v", err)
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		ipc.RespondErrorf("failed to write blob: %v", err)
		return
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "OnlineBible",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		ipc.RespondErrorf("failed to stat: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: []ipc.EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
				Metadata:  map[string]string{"format": "OnlineBible"},
			},
		},
	})
}

func handleExtractIR(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ipc.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "OnlineBible",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L2",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_onlinebible_raw"] = hex.EncodeToString(data)

	// Extract content from OnlineBible format
	corpus.Documents = extractOnlineBibleContent(string(data), artifactID)

	// If no documents extracted, create minimal structure
	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ipc.Document{
			{
				ID:    artifactID,
				Title: artifactID,
				Order: 1,
			},
		}
	}

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to serialize IR: %v", err)
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}
	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L2",
		LossReport: &ipc.LossReport{
			SourceFormat: "OnlineBible",
			TargetFormat: "IR",
			LossClass:    "L2",
			Warnings:     []string{"OnlineBible format - some formatting may be lost"},
		},
	})
}

func extractOnlineBibleContent(content, artifactID string) []*ipc.Document {
	// Group verses by book
	books := make(map[string][]*ipc.ContentBlock)
	bookOrder := make(map[string]int)

	// Parse verse references: "$Book Chapter:Verse Text" or "Book Chapter:Verse Text"
	versePattern := regexp.MustCompile(`(?m)^\$?([A-Z][a-z]{1,2}(?:\s+[A-Z][a-z]+)?)\s+(\d+):(\d+)\s+(.*)$`)

	scanner := bufio.NewScanner(strings.NewReader(content))
	sequence := 0
	orderCounter := 0

	for scanner.Scan() {
		line := scanner.Text()
		matches := versePattern.FindStringSubmatch(line)
		if len(matches) == 5 {
			book := matches[1]
			chapter, _ := strconv.Atoi(matches[2])
			verse, _ := strconv.Atoi(matches[3])
			text := matches[4]

			if _, exists := bookOrder[book]; !exists {
				orderCounter++
				bookOrder[book] = orderCounter
			}

			sequence++
			hash := sha256.Sum256([]byte(text))
			osisID := fmt.Sprintf("%s.%d.%d", book, chapter, verse)

			cb := &ipc.ContentBlock{
				ID:       fmt.Sprintf("cb-%d", sequence),
				Sequence: sequence,
				Text:     text,
				Hash:     hex.EncodeToString(hash[:]),
				Anchors: []*ipc.Anchor{
					{
						ID:       fmt.Sprintf("a-%d-0", sequence),
						Position: 0,
						Spans: []*ipc.Span{
							{
								ID:            fmt.Sprintf("s-%s", osisID),
								Type:          "VERSE",
								StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
								Ref: &ipc.Ref{
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

			books[book] = append(books[book], cb)
		}
	}

	// Convert to documents
	var documents []*ipc.Document
	for book, blocks := range books {
		doc := &ipc.Document{
			ID:            book,
			Title:         book,
			Order:         bookOrder[book],
			ContentBlocks: blocks,
		}
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

	// If no verses found, try simpler parsing
	if len(documents) == 0 {
		doc := &ipc.Document{
			ID:    artifactID,
			Title: artifactID,
			Order: 1,
		}

		lines := strings.Split(content, "\n")
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if len(line) > 5 {
				hash := sha256.Sum256([]byte(line))
				doc.ContentBlocks = append(doc.ContentBlocks, &ipc.ContentBlock{
					ID:       fmt.Sprintf("cb-%d", i+1),
					Sequence: i + 1,
					Text:     line,
					Hash:     hex.EncodeToString(hash[:]),
				})
			}
		}

		documents = []*ipc.Document{doc}
	}

	return documents
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

	data, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR file: %v", err)
		return
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		ipc.RespondErrorf("failed to parse IR: %v", err)
		return
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".ont")

	// Check for raw OnlineBible for round-trip
	if raw, ok := corpus.Attributes["_onlinebible_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0644); err != nil {
				ipc.RespondErrorf("failed to write OnlineBible: %v", err)
				return
			}
			ipc.MustRespond(&ipc.EmitNativeResult{
				OutputPath: outputPath,
				Format:     "OnlineBible",
				LossClass:  "L0",
				LossReport: &ipc.LossReport{
					SourceFormat: "IR",
					TargetFormat: "OnlineBible",
					LossClass:    "L0",
				},
			})
			return
		}
	}

	// Generate OnlineBible format from IR
	var buf bytes.Buffer

	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			book := doc.ID
			chapter := 1
			verse := cb.Sequence

			if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 {
				if ref := cb.Anchors[0].Spans[0].Ref; ref != nil {
					book = ref.Book
					chapter = ref.Chapter
					verse = ref.Verse
				}
			}

			fmt.Fprintf(&buf, "$%s %d:%d %s\n", book, chapter, verse, cb.Text)
		}
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		ipc.RespondErrorf("failed to write OnlineBible: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "OnlineBible",
		LossClass:  "L2",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "OnlineBible",
			LossClass:    "L2",
			Warnings:     []string{"Generated OnlineBible-compatible format"},
		},
	})
}
