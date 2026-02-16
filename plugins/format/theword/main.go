//go:build !sdk

// Plugin format-theword handles TheWord/PalmBible+ Bible file ingestion.
// TheWord format uses line-based text files where each line is a verse.
// Supported extensions: .ont (OT), .nt (NT), .twm (full module)
//
// IR Support:
// - extract-ir: Extracts IR from TheWord text (L0 lossless via raw storage)
// - emit-native: Converts IR back to TheWord format (L0)
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"os"
	"path/filepath"
	"strings"
)

// Bible book verse counts for TheWord format (KJV versification)
var bookInfo = []struct {
	osisID   string
	name     string
	chapters []int // verse counts per chapter
}{
	{"Gen", "Genesis", []int{31, 25, 24, 26, 32, 22, 24, 22, 29, 32, 32, 20, 18, 24, 21, 16, 27, 33, 38, 18, 34, 24, 20, 67, 34, 35, 46, 22, 35, 43, 55, 32, 20, 31, 29, 43, 36, 30, 23, 23, 57, 38, 34, 34, 28, 34, 31, 22, 33, 26}},
	{"Exod", "Exodus", []int{22, 25, 22, 31, 23, 30, 25, 32, 35, 29, 10, 51, 22, 31, 27, 36, 16, 27, 25, 26, 36, 31, 33, 18, 40, 37, 21, 43, 46, 38, 18, 35, 23, 35, 35, 38, 29, 31, 43, 38}},
	{"Lev", "Leviticus", []int{17, 16, 17, 35, 19, 30, 38, 36, 24, 20, 47, 8, 59, 57, 33, 34, 16, 30, 37, 27, 24, 33, 44, 23, 55, 46, 34}},
	{"Num", "Numbers", []int{54, 34, 51, 49, 31, 27, 89, 26, 23, 36, 35, 16, 33, 45, 41, 50, 13, 32, 22, 29, 35, 41, 30, 25, 18, 65, 23, 31, 40, 16, 54, 42, 56, 29, 34, 13}},
	{"Deut", "Deuteronomy", []int{46, 37, 29, 49, 33, 25, 26, 20, 29, 22, 32, 32, 18, 29, 23, 22, 20, 22, 21, 20, 23, 30, 25, 22, 19, 19, 26, 68, 29, 20, 30, 52, 29, 12}},
}

// Simplified OT books for demo (full implementation would have all 66)
var otBooks = []string{"Gen", "Exod", "Lev", "Num", "Deut", "Josh", "Judg", "Ruth", "1Sam", "2Sam", "1Kgs", "2Kgs", "1Chr", "2Chr", "Ezra", "Neh", "Esth", "Job", "Ps", "Prov", "Eccl", "Song", "Isa", "Jer", "Lam", "Ezek", "Dan", "Hos", "Joel", "Amos", "Obad", "Jonah", "Mic", "Nah", "Hab", "Zeph", "Hag", "Zech", "Mal"}
var ntBooks = []string{"Matt", "Mark", "Luke", "John", "Acts", "Rom", "1Cor", "2Cor", "Gal", "Eph", "Phil", "Col", "1Thess", "2Thess", "1Tim", "2Tim", "Titus", "Phlm", "Heb", "Jas", "1Pet", "2Pet", "1John", "2John", "3John", "Jude", "Rev"}

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

	info, err := os.Stat(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if info.IsDir() {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "path is a directory, not a file",
		})
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	validExts := map[string]string{
		".ont": "Old Testament",
		".nt":  "New Testament",
		".twm": "Full Module",
	}

	moduleType, isValid := validExts[ext]
	if !isValid {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("extension %s is not a known TheWord format", ext),
		})
		return
	}

	// Check if file has reasonable line count for a Bible
	file, err := os.Open(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open file: %v", err),
		})
		return
	}
	defer file.Close()

	lineCount := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() && lineCount < 100 {
		lineCount++
	}

	if lineCount < 10 {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "file has too few lines for TheWord format",
		})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   "TheWord",
		Reason:   fmt.Sprintf("TheWord %s detected", moduleType),
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
			"format": "TheWord",
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

	entries := []ipc.EnumerateEntry{
		{
			Path:      filepath.Base(path),
			SizeBytes: info.Size(),
			IsDir:     false,
			Metadata: map[string]string{
				"format": "TheWord",
			},
		},
	}
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: entries,
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
	ext := strings.ToLower(filepath.Ext(path))

	corpus := &ipc.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "TheWord",
		LossClass:    "L0",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		Attributes:   make(map[string]string),
	}

	// Store raw content for L0 round-trip
	corpus.Attributes["_theword_raw"] = string(data)
	corpus.Attributes["_theword_ext"] = ext

	// Parse lines into verses
	lines := strings.Split(string(data), "\n")

	// Determine book list based on extension
	var books []string
	switch ext {
	case ".ont":
		books = otBooks
	case ".nt":
		books = ntBooks
	default:
		books = append(otBooks, ntBooks...)
	}

	// Simple parsing: each line is a verse
	sequence := 0
	bookDocs := make(map[string]*ipc.Document)
	lineIdx := 0

	for _, book := range books {
		if lineIdx >= len(lines) {
			break
		}

		doc, ok := bookDocs[book]
		if !ok {
			doc = &ipc.Document{
				ID:    book,
				Title: book,
				Order: len(bookDocs) + 1,
			}
			bookDocs[book] = doc
			corpus.Documents = append(corpus.Documents, doc)
		}

		// Simple: assign lines sequentially (real impl would use verse counts)
		for chapter := 1; chapter <= 3 && lineIdx < len(lines); chapter++ {
			for verse := 1; verse <= 10 && lineIdx < len(lines); verse++ {
				text := strings.TrimSpace(lines[lineIdx])
				lineIdx++

				if text == "" {
					continue
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
				doc.ContentBlocks = append(doc.ContentBlocks, cb)
			}
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
		LossClass: "L0",
		LossReport: &ipc.LossReport{
			SourceFormat: "TheWord",
			TargetFormat: "IR",
			LossClass:    "L0",
		},
	})
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

	// Determine output extension
	ext := ".twm"
	if e, ok := corpus.Attributes["_theword_ext"]; ok {
		ext = e
	}

	outputPath := filepath.Join(outputDir, corpus.ID+ext)

	// Check for raw content for L0 round-trip
	if raw, ok := corpus.Attributes["_theword_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			ipc.RespondErrorf("failed to write TheWord file: %v", err)
			return
		}
		ipc.MustRespond(&ipc.EmitNativeResult{
			OutputPath: outputPath,
			Format:     "TheWord",
			LossClass:  "L0",
			LossReport: &ipc.LossReport{
				SourceFormat: "IR",
				TargetFormat: "TheWord",
				LossClass:    "L0",
			},
		})
		return
	}

	// Generate TheWord format from IR
	var buf strings.Builder
	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			buf.WriteString(cb.Text)
			buf.WriteString("\n")
		}
	}

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0644); err != nil {
		ipc.RespondErrorf("failed to write TheWord file: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "TheWord",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "TheWord",
			LossClass:    "L1",
			Warnings: []string{
				"TheWord regenerated from IR - verse order may differ",
			},
		},
	})
}
