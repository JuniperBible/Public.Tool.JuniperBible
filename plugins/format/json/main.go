//go:build !sdk

// Plugin format-json handles JSON Bible format.
// Provides clean JSON structure for programmatic access.
//
// IR Support:
// - extract-ir: Reads JSON Bible format back to IR (L0)
// - emit-native: Converts IR to clean JSON format (L0)
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// Parallel Corpus Types
// JSONBible is the clean output format
type JSONBible struct {
	Meta   JSONMeta    `json:"meta"`
	Books  []JSONBook  `json:"books"`
	Verses []JSONVerse `json:"verses,omitempty"` // Flat verse list for easy access
}

type JSONMeta struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Language    string `json:"language,omitempty"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

type JSONBook struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Order    int           `json:"order"`
	Chapters []JSONChapter `json:"chapters"`
}

type JSONChapter struct {
	Number int         `json:"number"`
	Verses []JSONVerse `json:"verses"`
}

type JSONVerse struct {
	Book    string `json:"book"`
	Chapter int    `json:"chapter"`
	Verse   int    `json:"verse"`
	Text    string `json:"text"`
	ID      string `json:"id"` // OSIS ID like "Gen.1.1"
}

// JSONParallel is the parallel Bible output format
type JSONParallel struct {
	Meta         JSONParallelMeta    `json:"meta"`
	Translations []JSONTranslation   `json:"translations"`
	Verses       []JSONParallelVerse `json:"verses"`
}

type JSONParallelMeta struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	BaseID     string   `json:"base_id,omitempty"`
	Languages  []string `json:"languages"`
	AlignLevel string   `json:"align_level"`
}

type JSONTranslation struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Language string `json:"language"`
}

type JSONParallelVerse struct {
	Ref   string            `json:"ref"`   // OSIS ID
	Texts map[string]string `json:"texts"` // translation ID -> text
}

// JSONInterlinear is the interlinear output format
type JSONInterlinear struct {
	Meta  JSONInterlinearMeta   `json:"meta"`
	Lines []JSONInterlinearLine `json:"lines"`
}

type JSONInterlinearMeta struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Layers []string `json:"layers"`
}

type JSONInterlinearLine struct {
	Ref    string                 `json:"ref"`
	Tokens []JSONInterlinearToken `json:"tokens"`
}

type JSONInterlinearToken struct {
	Layers map[string]string `json:"layers"` // layer name -> text
}

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
	case "emit-parallel":
		handleEmitParallel(req.Args)
	case "emit-interlinear":
		handleEmitInterlinear(req.Args)
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
			Reason:   "path is a directory",
		})
		return
	}

	// Check extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".json" {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not a .json file",
		})
		return
	}

	// Try to parse as JSONBible
	data, err := os.ReadFile(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		})
		return
	}

	var jsonBible JSONBible
	if err := json.Unmarshal(data, &jsonBible); err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not valid JSON",
		})
		return
	}

	// Check for our format markers
	if jsonBible.Meta.ID == "" && len(jsonBible.Books) == 0 && len(jsonBible.Verses) == 0 {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not a Capsule JSON Bible format",
		})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   "JSON",
		Reason:   "Capsule JSON Bible format detected",
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
			"format": "JSON",
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
				"format": "JSON",
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

	var jsonBible JSONBible
	if err := json.Unmarshal(data, &jsonBible); err != nil {
		ipc.RespondErrorf("failed to parse JSON: %v", err)
		return
	}

	// Convert JSONBible to IR Corpus
	corpus := &ipc.Corpus{
		ID:           jsonBible.Meta.ID,
		Version:      jsonBible.Meta.Version,
		ModuleType:   "BIBLE",
		Title:        jsonBible.Meta.Title,
		Language:     jsonBible.Meta.Language,
		Description:  jsonBible.Meta.Description,
		SourceFormat: "JSON",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L0",
		Attributes:   make(map[string]string),
	}

	// Store raw for L0 round-trip
	corpus.Attributes["_json_raw"] = string(data)

	sequence := 0
	for _, book := range jsonBible.Books {
		doc := &ipc.Document{
			ID:    book.ID,
			Title: book.Name,
			Order: book.Order,
		}

		for _, chapter := range book.Chapters {
			for _, verse := range chapter.Verses {
				sequence++
				hash := sha256.Sum256([]byte(verse.Text))
				osisID := fmt.Sprintf("%s.%d.%d", book.ID, chapter.Number, verse.Verse)

				cb := &ipc.ContentBlock{
					ID:       fmt.Sprintf("cb-%d", sequence),
					Sequence: sequence,
					Text:     verse.Text,
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
										Book:    book.ID,
										Chapter: chapter.Number,
										Verse:   verse.Verse,
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

		corpus.Documents = append(corpus.Documents, doc)
	}

	// Also handle flat verses if present
	if len(jsonBible.Books) == 0 && len(jsonBible.Verses) > 0 {
		bookDocs := make(map[string]*ipc.Document)
		for _, verse := range jsonBible.Verses {
			doc, ok := bookDocs[verse.Book]
			if !ok {
				doc = &ipc.Document{
					ID:    verse.Book,
					Title: verse.Book,
					Order: len(bookDocs) + 1,
				}
				bookDocs[verse.Book] = doc
				corpus.Documents = append(corpus.Documents, doc)
			}

			sequence++
			hash := sha256.Sum256([]byte(verse.Text))

			cb := &ipc.ContentBlock{
				ID:       fmt.Sprintf("cb-%d", sequence),
				Sequence: sequence,
				Text:     verse.Text,
				Hash:     hex.EncodeToString(hash[:]),
				Anchors: []*ipc.Anchor{
					{
						ID:       fmt.Sprintf("a-%d-0", sequence),
						Position: 0,
						Spans: []*ipc.Span{
							{
								ID:            fmt.Sprintf("s-%s", verse.ID),
								Type:          "VERSE",
								StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
								Ref: &ipc.Ref{
									Book:    verse.Book,
									Chapter: verse.Chapter,
									Verse:   verse.Verse,
									OSISID:  verse.ID,
								},
							},
						},
					},
				},
			}
			doc.ContentBlocks = append(doc.ContentBlocks, cb)
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
			SourceFormat: "JSON",
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

	outputPath := filepath.Join(outputDir, corpus.ID+".json")

	// Check for raw JSON for L0 round-trip
	if raw, ok := corpus.Attributes["_json_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0644); err != nil {
			ipc.RespondErrorf("failed to write JSON: %v", err)
			return
		}
		ipc.MustRespond(&ipc.EmitNativeResult{
			OutputPath: outputPath,
			Format:     "JSON",
			LossClass:  "L0",
			LossReport: &ipc.LossReport{
				SourceFormat: "IR",
				TargetFormat: "JSON",
				LossClass:    "L0",
			},
		})
		return
	}

	// Generate JSON from IR
	jsonBible := JSONBible{
		Meta: JSONMeta{
			ID:          corpus.ID,
			Title:       corpus.Title,
			Language:    corpus.Language,
			Description: corpus.Description,
			Version:     corpus.Version,
		},
	}

	for _, doc := range corpus.Documents {
		book := JSONBook{
			ID:    doc.ID,
			Name:  doc.Title,
			Order: doc.Order,
		}

		chapterMap := make(map[int]*JSONChapter)
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						chapter, ok := chapterMap[span.Ref.Chapter]
						if !ok {
							chapter = &JSONChapter{Number: span.Ref.Chapter}
							chapterMap[span.Ref.Chapter] = chapter
						}

						verse := JSONVerse{
							Book:    doc.ID,
							Chapter: span.Ref.Chapter,
							Verse:   span.Ref.Verse,
							Text:    cb.Text,
							ID:      span.Ref.OSISID,
						}
						chapter.Verses = append(chapter.Verses, verse)
						jsonBible.Verses = append(jsonBible.Verses, verse)
					}
				}
			}
		}

		// Sort chapters and add to book
		for i := 1; i <= 200; i++ {
			if ch, ok := chapterMap[i]; ok {
				book.Chapters = append(book.Chapters, *ch)
			}
		}

		jsonBible.Books = append(jsonBible.Books, book)
	}

	jsonData, err := json.MarshalIndent(jsonBible, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to serialize JSON: %v", err)
		return
	}

	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		ipc.RespondErrorf("failed to write JSON: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "JSON",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "JSON",
			LossClass:    "L1",
		},
	})
}

func handleEmitParallel(args map[string]interface{}) {
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

	var parallel ipc.ParallelCorpus
	if err := json.Unmarshal(data, &parallel); err != nil {
		ipc.RespondErrorf("failed to parse parallel corpus: %v", err)
		return
	}

	// Build JSONParallel output
	output := JSONParallel{
		Meta: JSONParallelMeta{
			ID:         parallel.ID,
			Title:      parallel.ID,
			AlignLevel: parallel.DefaultAlignment,
		},
	}

	// Add translations
	langSet := make(map[string]bool)
	for _, ref := range parallel.Corpora {
		output.Translations = append(output.Translations, JSONTranslation{
			ID:       ref.ID,
			Title:    ref.Title,
			Language: ref.Language,
		})
		langSet[ref.Language] = true
	}

	for lang := range langSet {
		output.Meta.Languages = append(output.Meta.Languages, lang)
	}

	if parallel.BaseCorpus != nil {
		output.Meta.BaseID = parallel.BaseCorpus.ID
	}

	// Add aligned verses
	for _, alignment := range parallel.Alignments {
		for _, unit := range alignment.Units {
			ref := ""
			if unit.Ref != nil {
				ref = unit.Ref.OSISID
			}
			output.Verses = append(output.Verses, JSONParallelVerse{
				Ref:   ref,
				Texts: unit.Texts,
			})
		}
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to serialize JSON: %v", err)
		return
	}

	outputPath := filepath.Join(outputDir, parallel.ID+".parallel.json")
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		ipc.RespondErrorf("failed to write JSON: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "JSON-Parallel",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "ParallelCorpus",
			TargetFormat: "JSON-Parallel",
			LossClass:    "L1",
		},
	})
}

func handleEmitInterlinear(args map[string]interface{}) {
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

	// Try to parse as array of InterlinearLine
	var lines []ipc.InterlinearLine
	if err := json.Unmarshal(data, &lines); err != nil {
		ipc.RespondErrorf("failed to parse interlinear data: %v", err)
		return
	}

	// Build JSONInterlinear output
	layerSet := make(map[string]bool)
	output := JSONInterlinear{
		Meta: JSONInterlinearMeta{
			ID:    "interlinear",
			Title: "Interlinear Text",
		},
	}

	for _, line := range lines {
		ref := ""
		if line.Ref != nil {
			ref = line.Ref.OSISID
		}

		// Get all layers
		for name := range line.Layers {
			layerSet[name] = true
		}

		// Build token list - align by index
		maxTokens := 0
		for _, layer := range line.Layers {
			if len(layer.Tokens) > maxTokens {
				maxTokens = len(layer.Tokens)
			}
		}

		jsonLine := JSONInterlinearLine{
			Ref:    ref,
			Tokens: make([]JSONInterlinearToken, maxTokens),
		}

		for i := 0; i < maxTokens; i++ {
			jsonLine.Tokens[i] = JSONInterlinearToken{
				Layers: make(map[string]string),
			}
			for name, layer := range line.Layers {
				if i < len(layer.Tokens) {
					jsonLine.Tokens[i].Layers[name] = layer.Tokens[i]
				}
			}
		}

		output.Lines = append(output.Lines, jsonLine)
	}

	for layer := range layerSet {
		output.Meta.Layers = append(output.Meta.Layers, layer)
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to serialize JSON: %v", err)
		return
	}

	outputPath := filepath.Join(outputDir, "interlinear.json")
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		ipc.RespondErrorf("failed to write JSON: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "JSON-Interlinear",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "InterlinearLine",
			TargetFormat: "JSON-Interlinear",
			LossClass:    "L1",
		},
	})
}
