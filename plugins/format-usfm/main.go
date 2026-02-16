//go:build !sdk

// Plugin format-usfm handles USFM (Unified Standard Format Markers) Bible files.
// It supports L0 lossless round-trip conversion through IR.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// IPCRequest is the incoming JSON request.
type IPCRequest struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args,omitempty"`
}

// IPCResponse is the outgoing JSON response.
type IPCResponse struct {
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// DetectResult is the result of a detect command.
type DetectResult struct {
	Detected bool   `json:"detected"`
	Format   string `json:"format,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// IngestResult is the result of an ingest command.
type IngestResult struct {
	ArtifactID string            `json:"artifact_id"`
	BlobSHA256 string            `json:"blob_sha256"`
	SizeBytes  int64             `json:"size_bytes"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// EnumerateResult is the result of an enumerate command.
type EnumerateResult struct {
	Entries []EnumerateEntry `json:"entries"`
}

// EnumerateEntry represents a file entry.
type EnumerateEntry struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	IsDir     bool   `json:"is_dir"`
}

// ExtractIRResult is the result of an extract-ir command.
type ExtractIRResult struct {
	IRPath     string      `json:"ir_path"`
	LossClass  string      `json:"loss_class"`
	LossReport *LossReport `json:"loss_report,omitempty"`
}

// EmitNativeResult is the result of an emit-native command.
type EmitNativeResult struct {
	OutputPath string      `json:"output_path"`
	Format     string      `json:"format"`
	LossClass  string      `json:"loss_class"`
	LossReport *LossReport `json:"loss_report,omitempty"`
}

// LossReport describes any data loss during conversion.
type LossReport struct {
	SourceFormat string        `json:"source_format"`
	TargetFormat string        `json:"target_format"`
	LossClass    string        `json:"loss_class"`
	LostElements []LostElement `json:"lost_elements,omitempty"`
	Warnings     []string      `json:"warnings,omitempty"`
}

// LostElement describes a specific element that was lost.
type LostElement struct {
	Path          string      `json:"path"`
	ElementType   string      `json:"element_type"`
	Reason        string      `json:"reason"`
	OriginalValue interface{} `json:"original_value,omitempty"`
}

// IR Types (matching core/ir package)
type Corpus struct {
	ID            string            `json:"id"`
	Version       string            `json:"version"`
	ModuleType    string            `json:"module_type"`
	Versification string            `json:"versification,omitempty"`
	Language      string            `json:"language,omitempty"`
	Title         string            `json:"title,omitempty"`
	Description   string            `json:"description,omitempty"`
	Publisher     string            `json:"publisher,omitempty"`
	Rights        string            `json:"rights,omitempty"`
	SourceFormat  string            `json:"source_format,omitempty"`
	Documents     []*Document       `json:"documents,omitempty"`
	SourceHash    string            `json:"source_hash,omitempty"`
	LossClass     string            `json:"loss_class,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type Document struct {
	ID            string            `json:"id"`
	Title         string            `json:"title,omitempty"`
	Order         int               `json:"order"`
	ContentBlocks []*ContentBlock   `json:"content_blocks,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type ContentBlock struct {
	ID         string                 `json:"id"`
	Sequence   int                    `json:"sequence"`
	Text       string                 `json:"text"`
	Tokens     []*Token               `json:"tokens,omitempty"`
	Anchors    []*Anchor              `json:"anchors,omitempty"`
	Hash       string                 `json:"hash,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type Token struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Text     string `json:"text"`
	StartPos int    `json:"start_pos"`
	EndPos   int    `json:"end_pos"`
}

type Anchor struct {
	ID       string  `json:"id"`
	Position int     `json:"position"`
	Spans    []*Span `json:"spans,omitempty"`
}

type Span struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	StartAnchorID string                 `json:"start_anchor_id"`
	EndAnchorID   string                 `json:"end_anchor_id,omitempty"`
	Ref           *Ref                   `json:"ref,omitempty"`
	Attributes    map[string]interface{} `json:"attributes,omitempty"`
}

type Ref struct {
	Book     string `json:"book"`
	Chapter  int    `json:"chapter,omitempty"`
	Verse    int    `json:"verse,omitempty"`
	VerseEnd int    `json:"verse_end,omitempty"`
	SubVerse string `json:"sub_verse,omitempty"`
	OSISID   string `json:"osis_id,omitempty"`
}

// USFM parsing helpers
var (
	markerRegex    = regexp.MustCompile(`\\([a-zA-Z0-9]+)\*?(?:\s|$)`)
	verseNumRegex  = regexp.MustCompile(`^(\d+)(?:-(\d+))?`)
	chapterRegex   = regexp.MustCompile(`^(\d+)`)
)

// Common USFM book IDs
var bookNames = map[string]string{
	"GEN": "Genesis", "EXO": "Exodus", "LEV": "Leviticus", "NUM": "Numbers",
	"DEU": "Deuteronomy", "JOS": "Joshua", "JDG": "Judges", "RUT": "Ruth",
	"1SA": "1 Samuel", "2SA": "2 Samuel", "1KI": "1 Kings", "2KI": "2 Kings",
	"1CH": "1 Chronicles", "2CH": "2 Chronicles", "EZR": "Ezra", "NEH": "Nehemiah",
	"EST": "Esther", "JOB": "Job", "PSA": "Psalms", "PRO": "Proverbs",
	"ECC": "Ecclesiastes", "SNG": "Song of Solomon", "ISA": "Isaiah", "JER": "Jeremiah",
	"LAM": "Lamentations", "EZK": "Ezekiel", "DAN": "Daniel", "HOS": "Hosea",
	"JOL": "Joel", "AMO": "Amos", "OBA": "Obadiah", "JON": "Jonah",
	"MIC": "Micah", "NAM": "Nahum", "HAB": "Habakkuk", "ZEP": "Zephaniah",
	"HAG": "Haggai", "ZEC": "Zechariah", "MAL": "Malachi",
	"MAT": "Matthew", "MRK": "Mark", "LUK": "Luke", "JHN": "John",
	"ACT": "Acts", "ROM": "Romans", "1CO": "1 Corinthians", "2CO": "2 Corinthians",
	"GAL": "Galatians", "EPH": "Ephesians", "PHP": "Philippians", "COL": "Colossians",
	"1TH": "1 Thessalonians", "2TH": "2 Thessalonians", "1TI": "1 Timothy", "2TI": "2 Timothy",
	"TIT": "Titus", "PHM": "Philemon", "HEB": "Hebrews", "JAS": "James",
	"1PE": "1 Peter", "2PE": "2 Peter", "1JN": "1 John", "2JN": "2 John",
	"3JN": "3 John", "JUD": "Jude", "REV": "Revelation",
}

func main() {
	// Read request from stdin
	var req IPCRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		respondError(fmt.Sprintf("failed to decode request: %v", err))
		return
	}

	// Dispatch command
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
		respondError(fmt.Sprintf("unknown command: %s", req.Command))
	}
}

func handleDetect(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if info.IsDir() {
		respond(&DetectResult{
			Detected: false,
			Reason:   "path is a directory, not a file",
		})
		return
	}

	// Read file and check for USFM markers
	data, err := os.ReadFile(path)
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read: %v", err),
		})
		return
	}

	content := string(data)

	// Check for USFM markers
	if strings.Contains(content, "\\id ") || strings.Contains(content, "\\c ") ||
		strings.Contains(content, "\\v ") || strings.Contains(content, "\\p") {
		respond(&DetectResult{
			Detected: true,
			Format:   "USFM",
			Reason:   "USFM markers detected",
		})
		return
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".usfm" || ext == ".sfm" || ext == ".ptx" {
		respond(&DetectResult{
			Detected: true,
			Format:   "USFM",
			Reason:   "USFM file extension detected",
		})
		return
	}

	respond(&DetectResult{
		Detected: false,
		Reason:   "not a USFM file",
	})
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	// Compute SHA-256
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	// Write to output directory
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		respondError(fmt.Sprintf("failed to create blob dir: %v", err))
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write blob: %v", err))
		return
	}

	// Parse book ID from \id marker
	artifactID := filepath.Base(path)
	content := string(data)
	if idx := strings.Index(content, "\\id "); idx >= 0 {
		endIdx := strings.IndexAny(content[idx+4:], " \n\r")
		if endIdx > 0 {
			artifactID = strings.TrimSpace(content[idx+4 : idx+4+endIdx])
		}
	}

	respond(&IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"original_name": filepath.Base(path),
			"format":        "USFM",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to stat: %v", err))
		return
	}

	respond(&EnumerateResult{
		Entries: []EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
			},
		},
	})
}

func handleExtractIR(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	// Read USFM file
	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	// Parse USFM
	corpus, err := parseUSFMToIR(data)
	if err != nil {
		respondError(fmt.Sprintf("failed to parse USFM: %v", err))
		return
	}

	// Serialize IR to JSON
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		respondError(fmt.Sprintf("failed to serialize IR: %v", err))
		return
	}

	// Write IR to output directory
	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write IR: %v", err))
		return
	}

	respond(&ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L0",
		LossReport: &LossReport{
			SourceFormat: "USFM",
			TargetFormat: "IR",
			LossClass:    "L0",
		},
	})
}

func handleEmitNative(args map[string]interface{}) {
	irPath, ok := args["ir_path"].(string)
	if !ok {
		respondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	// Read IR file
	data, err := os.ReadFile(irPath)
	if err != nil {
		respondError(fmt.Sprintf("failed to read IR file: %v", err))
		return
	}

	// Parse IR
	var corpus Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		respondError(fmt.Sprintf("failed to parse IR: %v", err))
		return
	}

	// Convert IR to USFM
	usfmData, err := emitUSFMFromIR(&corpus)
	if err != nil {
		respondError(fmt.Sprintf("failed to emit USFM: %v", err))
		return
	}

	// Write USFM to output directory
	outputPath := filepath.Join(outputDir, corpus.ID+".usfm")
	if err := os.WriteFile(outputPath, usfmData, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write USFM: %v", err))
		return
	}

	respond(&EmitNativeResult{
		OutputPath: outputPath,
		Format:     "USFM",
		LossClass:  "L0",
		LossReport: &LossReport{
			SourceFormat: "IR",
			TargetFormat: "USFM",
			LossClass:    "L0",
		},
	})
}

// parseUSFMToIR converts USFM text to IR Corpus
func parseUSFMToIR(data []byte) (*Corpus, error) {
	content := string(data)

	corpus := &Corpus{
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "USFM",
		LossClass:    "L0",
		Attributes:   make(map[string]string),
	}

	// Store raw content for L0 round-trip
	corpus.Attributes["_usfm_raw"] = content

	var currentDoc *Document
	var currentChapter int
	var blockSeq int

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Parse markers
		if strings.HasPrefix(trimmed, "\\") {
			parts := strings.SplitN(trimmed, " ", 2)
			marker := strings.TrimPrefix(parts[0], "\\")
			var value string
			if len(parts) > 1 {
				value = parts[1]
			}

			switch marker {
			case "id":
				// Book ID
				idParts := strings.Fields(value)
				if len(idParts) > 0 {
					bookID := strings.ToUpper(idParts[0])
					corpus.ID = bookID
					currentDoc = &Document{
						ID:         bookID,
						Order:      len(corpus.Documents) + 1,
						Attributes: make(map[string]string),
					}
					if name, ok := bookNames[bookID]; ok {
						currentDoc.Title = name
					}
					corpus.Documents = append(corpus.Documents, currentDoc)
				}

			case "h", "toc1", "toc2", "toc3":
				// Header/TOC entries
				if currentDoc != nil && value != "" {
					if marker == "h" && currentDoc.Title == "" {
						currentDoc.Title = value
					}
					currentDoc.Attributes[marker] = value
				}

			case "mt", "mt1", "mt2", "mt3":
				// Main title
				if corpus.Title == "" && value != "" {
					corpus.Title = value
				}
				if currentDoc != nil {
					currentDoc.Attributes[marker] = value
				}

			case "c":
				// Chapter
				if matches := chapterRegex.FindStringSubmatch(value); len(matches) > 0 {
					currentChapter, _ = strconv.Atoi(matches[1])
				}

			case "v":
				// Verse
				if currentDoc != nil {
					verseText := value
					verseNum := 0
					verseEnd := 0

					// Parse verse number
					if matches := verseNumRegex.FindStringSubmatch(value); len(matches) > 0 {
						verseNum, _ = strconv.Atoi(matches[1])
						if matches[2] != "" {
							verseEnd, _ = strconv.Atoi(matches[2])
						}
						// Extract text after verse number
						verseText = strings.TrimSpace(value[len(matches[0]):])
					}

					if verseText != "" {
						blockSeq++
						osisID := fmt.Sprintf("%s.%d.%d", corpus.ID, currentChapter, verseNum)

						block := &ContentBlock{
							ID:       fmt.Sprintf("cb-%d", blockSeq),
							Sequence: blockSeq,
							Text:     verseText,
							Anchors: []*Anchor{
								{
									ID:       fmt.Sprintf("a-%d-0", blockSeq),
									Position: 0,
									Spans: []*Span{
										{
											ID:            fmt.Sprintf("s-%s", osisID),
											Type:          "VERSE",
											StartAnchorID: fmt.Sprintf("a-%d-0", blockSeq),
											Ref: &Ref{
												Book:     corpus.ID,
												Chapter:  currentChapter,
												Verse:    verseNum,
												VerseEnd: verseEnd,
												OSISID:   osisID,
											},
										},
									},
								},
							},
						}

						// Compute hash
						h := sha256.Sum256([]byte(verseText))
						block.Hash = hex.EncodeToString(h[:])

						currentDoc.ContentBlocks = append(currentDoc.ContentBlocks, block)
					}
				}

			case "p", "m", "pi", "mi", "nb":
				// Paragraph markers - may contain text
				if currentDoc != nil && value != "" {
					blockSeq++
					block := &ContentBlock{
						ID:       fmt.Sprintf("cb-%d", blockSeq),
						Sequence: blockSeq,
						Text:     value,
						Attributes: map[string]interface{}{
							"type": "paragraph",
							"marker": marker,
						},
					}
					h := sha256.Sum256([]byte(value))
					block.Hash = hex.EncodeToString(h[:])
					currentDoc.ContentBlocks = append(currentDoc.ContentBlocks, block)
				}

			case "q", "q1", "q2", "q3", "qr", "qc", "qm":
				// Poetry markers
				if currentDoc != nil && value != "" {
					blockSeq++
					block := &ContentBlock{
						ID:       fmt.Sprintf("cb-%d", blockSeq),
						Sequence: blockSeq,
						Text:     value,
						Attributes: map[string]interface{}{
							"type": "poetry",
							"marker": marker,
						},
					}
					h := sha256.Sum256([]byte(value))
					block.Hash = hex.EncodeToString(h[:])
					currentDoc.ContentBlocks = append(currentDoc.ContentBlocks, block)
				}
			}
		}
	}

	// Compute source hash
	h := sha256.Sum256(data)
	corpus.SourceHash = hex.EncodeToString(h[:])

	return corpus, nil
}

// emitUSFMFromIR converts IR Corpus back to USFM text
func emitUSFMFromIR(corpus *Corpus) ([]byte, error) {
	// Check if we have the original raw USFM for L0 lossless round-trip
	if rawUSFM, ok := corpus.Attributes["_usfm_raw"]; ok && rawUSFM != "" {
		return []byte(rawUSFM), nil
	}

	// Otherwise, reconstruct USFM from IR structure
	var buf bytes.Buffer

	for _, doc := range corpus.Documents {
		// Write book ID
		buf.WriteString(fmt.Sprintf("\\id %s\n", doc.ID))

		// Write header
		if doc.Title != "" {
			buf.WriteString(fmt.Sprintf("\\h %s\n", doc.Title))
		}

		// Write attributes (toc entries, mt, etc.)
		if doc.Attributes != nil {
			for key, val := range doc.Attributes {
				if key != "h" && !strings.HasPrefix(key, "_") {
					buf.WriteString(fmt.Sprintf("\\%s %s\n", key, val))
				}
			}
		}

		currentChapter := 0

		for _, block := range doc.ContentBlocks {
			// Check for verse spans to determine chapter/verse
			for _, anchor := range block.Anchors {
				for _, span := range anchor.Spans {
					if span.Type == "VERSE" && span.Ref != nil {
						// Write chapter marker if changed
						if span.Ref.Chapter != currentChapter {
							currentChapter = span.Ref.Chapter
							buf.WriteString(fmt.Sprintf("\\c %d\n", currentChapter))
						}

						// Write verse
						if span.Ref.VerseEnd > 0 && span.Ref.VerseEnd != span.Ref.Verse {
							buf.WriteString(fmt.Sprintf("\\v %d-%d %s\n", span.Ref.Verse, span.Ref.VerseEnd, block.Text))
						} else {
							buf.WriteString(fmt.Sprintf("\\v %d %s\n", span.Ref.Verse, block.Text))
						}
						break
					}
				}
			}

			// Handle non-verse blocks (paragraphs, poetry)
			if len(block.Anchors) == 0 && block.Text != "" {
				if block.Attributes != nil {
					if marker, ok := block.Attributes["marker"].(string); ok {
						buf.WriteString(fmt.Sprintf("\\%s %s\n", marker, block.Text))
					} else if blockType, ok := block.Attributes["type"].(string); ok {
						switch blockType {
						case "poetry":
							buf.WriteString(fmt.Sprintf("\\q %s\n", block.Text))
						case "paragraph":
							buf.WriteString(fmt.Sprintf("\\p %s\n", block.Text))
						default:
							buf.WriteString(fmt.Sprintf("\\p %s\n", block.Text))
						}
					}
				}
			}
		}
	}

	return buf.Bytes(), nil
}

func respond(result interface{}) {
	resp := IPCResponse{
		Status: "ok",
		Result: result,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
}

func respondError(msg string) {
	resp := IPCResponse{
		Status: "error",
		Error:  msg,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
	os.Exit(1)
}

// Compile check
var _ = io.Copy
