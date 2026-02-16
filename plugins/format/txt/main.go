//go:build !sdk

// Plugin format-txt handles plain text Bible format.
// Produces verse-per-line text output.
//
// IR Support:
// - extract-ir: Reads plain text Bible format to IR (L3)
// - emit-native: Converts IR to plain text format (L3)
// Note: L3 means text is preserved but all markup is lost.
package main

import (
	"bufio"
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
	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".txt" && ext != ".text" {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not a .txt file",
		})
		return
	}

	// Check for Bible-like content (verse references)
	data, err := os.ReadFile(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		})
		return
	}

	content := string(data)
	// Look for verse patterns like "Gen 1:1" or "1:1" at line starts
	versePattern := regexp.MustCompile(`(?m)^(\w+\s+)?(\d+):(\d+)\s+`)
	if versePattern.MatchString(content) {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "TXT",
			Reason:   "Plain text Bible format detected",
		})
		return
	}

	ipc.MustRespond(&ipc.DetectResult{
		Detected: false,
		Reason:   "no verse patterns found",
	})
}

func handleIngest(args map[string]interface{}) {
	ipc.StandardIngest(args, "TXT", nil)
}

func handleEnumerate(args map[string]interface{}) {
	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondError(err.Error())
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
			},
		},
	})
}

func handleExtractIR(args map[string]interface{}) {
	path, outputDir, err := ipc.PathAndOutputDir(args)
	if err != nil {
		ipc.RespondError(err.Error())
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
		SourceFormat: "TXT",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L3",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip (L0 when raw available)
	corpus.Attributes["_txt_raw"] = string(data)

	// Parse text content
	corpus.Documents = parseTXTContent(string(data), artifactID)

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
		LossClass: "L3",
		LossReport: &ipc.LossReport{
			SourceFormat: "TXT",
			TargetFormat: "IR",
			LossClass:    "L3",
			Warnings:     []string{"Plain text format loses all markup information"},
		},
	})
}

func parseTXTContent(content, artifactID string) []*ipc.Document {
	doc := &ipc.Document{
		ID:    artifactID,
		Title: artifactID,
		Order: 1,
	}

	// Parse verses: look for patterns like "Book C:V text" or "C:V text"
	// Examples: "Gen 1:1 In the beginning..." or "1:1 In the beginning..."
	versePattern := regexp.MustCompile(`^(\w+)?\s*(\d+):(\d+)\s+(.+)$`)

	scanner := bufio.NewScanner(strings.NewReader(content))
	sequence := 0
	currentBook := artifactID

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		matches := versePattern.FindStringSubmatch(line)
		if len(matches) > 0 {
			if matches[1] != "" {
				currentBook = matches[1]
			}
			chapter, _ := strconv.Atoi(matches[2])
			verse, _ := strconv.Atoi(matches[3])
			text := strings.TrimSpace(matches[4])

			sequence++
			hash := sha256.Sum256([]byte(text))
			osisID := fmt.Sprintf("%s.%d.%d", currentBook, chapter, verse)

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
									Book:    currentBook,
									Chapter: chapter,
									Verse:   verse,
									OSISID:  osisID,
								},
							},
						},
					},
				},
			}
			doc.ContentBlocks = append(doc.ContentBlocks, cb)
		}
	}

	// Update document ID if we found a book name
	if currentBook != artifactID {
		doc.ID = currentBook
		doc.Title = currentBook
	}

	return []*ipc.Document{doc}
}

func handleEmitNative(args map[string]interface{}) {
	irPath, err := ipc.StringArg(args, "ir_path")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	outputDir, err := ipc.StringArg(args, "output_dir")
	if err != nil {
		ipc.RespondError(err.Error())
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

	outputPath := filepath.Join(outputDir, corpus.ID+".txt")

	// Check for raw text for round-trip
	if raw, ok := corpus.Attributes["_txt_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			ipc.RespondErrorf("failed to write text: %v", err)
			return
		}

		ipc.MustRespond(&ipc.EmitNativeResult{
			OutputPath: outputPath,
			Format:     "TXT",
			LossClass:  "L0",
			LossReport: &ipc.LossReport{
				SourceFormat: "IR",
				TargetFormat: "TXT",
				LossClass:    "L0",
			},
		})
		return
	}

	// Generate text from IR
	var buf strings.Builder

	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						buf.WriteString(fmt.Sprintf("%s %d:%d %s\n",
							span.Ref.Book,
							span.Ref.Chapter,
							span.Ref.Verse,
							cb.Text))
					}
				}
			}
		}
	}

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0644); err != nil {
		ipc.RespondErrorf("failed to write text: %v", err)
		return
	}

	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "TXT",
		LossClass:  "L3",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "TXT",
			LossClass:    "L3",
			Warnings:     []string{"All markup and formatting lost in plain text output"},
		},
	})
}
