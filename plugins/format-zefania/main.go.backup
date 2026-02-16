//go:build !sdk

// Plugin format-zefania handles Zefania XML Bible file ingestion.
// Zefania XML is a Bible format used primarily in German-speaking regions.
//
// IR Support:
// - extract-ir: Extracts IR from Zefania XML (L0 lossless via raw storage)
// - emit-native: Converts IR back to Zefania format (L0)
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	Path      string            `json:"path"`
	SizeBytes int64             `json:"size_bytes"`
	IsDir     bool              `json:"is_dir"`
	Metadata  map[string]string `json:"metadata,omitempty"`
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

// Zefania book number to OSIS ID mapping
var zefaniaBookToOSIS = map[int]string{
	1: "Gen", 2: "Exod", 3: "Lev", 4: "Num", 5: "Deut",
	6: "Josh", 7: "Judg", 8: "Ruth", 9: "1Sam", 10: "2Sam",
	11: "1Kgs", 12: "2Kgs", 13: "1Chr", 14: "2Chr", 15: "Ezra",
	16: "Neh", 17: "Esth", 18: "Job", 19: "Ps", 20: "Prov",
	21: "Eccl", 22: "Song", 23: "Isa", 24: "Jer", 25: "Lam",
	26: "Ezek", 27: "Dan", 28: "Hos", 29: "Joel", 30: "Amos",
	31: "Obad", 32: "Jonah", 33: "Mic", 34: "Nah", 35: "Hab",
	36: "Zeph", 37: "Hag", 38: "Zech", 39: "Mal",
	40: "Matt", 41: "Mark", 42: "Luke", 43: "John", 44: "Acts",
	45: "Rom", 46: "1Cor", 47: "2Cor", 48: "Gal", 49: "Eph",
	50: "Phil", 51: "Col", 52: "1Thess", 53: "2Thess",
	54: "1Tim", 55: "2Tim", 56: "Titus", 57: "Phlm", 58: "Heb",
	59: "Jas", 60: "1Pet", 61: "2Pet", 62: "1John", 63: "2John",
	64: "3John", 65: "Jude", 66: "Rev",
}

var osisToZefaniaBook = func() map[string]int {
	m := make(map[string]int)
	for k, v := range zefaniaBookToOSIS {
		m[v] = k
	}
	return m
}()

func main() {
	var req IPCRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		respondError(fmt.Sprintf("failed to decode request: %v", err))
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

	data, err := os.ReadFile(path)
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot read file: %v", err),
		})
		return
	}

	content := string(data)
	// Check for Zefania XML markers
	if !strings.Contains(content, "<XMLBIBLE") && !strings.Contains(content, "<xmlbible") {
		respond(&DetectResult{
			Detected: false,
			Reason:   "not a Zefania XML file (no <XMLBIBLE> element)",
		})
		return
	}

	respond(&DetectResult{
		Detected: true,
		Format:   "Zefania",
		Reason:   "Zefania XML detected",
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

	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0700); err != nil {
		respondError(fmt.Sprintf("failed to create blob dir: %v", err))
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0600); err != nil {
		respondError(fmt.Sprintf("failed to write blob: %v", err))
		return
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	respond(&IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "Zefania",
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

	entries := []EnumerateEntry{
		{
			Path:      filepath.Base(path),
			SizeBytes: info.Size(),
			IsDir:     false,
			Metadata: map[string]string{
				"format": "Zefania",
			},
		},
	}

	respond(&EnumerateResult{
		Entries: entries,
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

	data, err := os.ReadFile(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to read file: %v", err))
		return
	}

	sourceHash := sha256.Sum256(data)

	corpus, err := parseZefaniaToIR(data)
	if err != nil {
		respondError(fmt.Sprintf("failed to parse Zefania: %v", err))
		return
	}

	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L0"

	if corpus.Attributes == nil {
		corpus.Attributes = make(map[string]string)
	}
	corpus.Attributes["_zefania_raw"] = string(data)

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		respondError(fmt.Sprintf("failed to serialize IR: %v", err))
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0600); err != nil {
		respondError(fmt.Sprintf("failed to write IR: %v", err))
		return
	}

	respond(&ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L0",
		LossReport: &LossReport{
			SourceFormat: "Zefania",
			TargetFormat: "IR",
			LossClass:    "L0",
		},
	})
}

// parserState holds the parsing context for Zefania XML parsing
type parserState struct {
	corpus         *Corpus
	currentBook    *Document
	currentChapter int
	sequence       int
}

// elementHandler is a function that handles a specific XML element
type elementHandler func(*parserState, xml.StartElement, *xml.Decoder) error

// elementHandlers maps element names to their handlers
var elementHandlers = map[string]elementHandler{
	"XMLBIBLE":   handleXMLBible,
	"BIBLEBOOK":  handleBibleBook,
	"CHAPTER":    handleChapter,
	"VERS":       handleVerse,
}

func parseZefaniaToIR(data []byte) (*Corpus, error) {
	state := &parserState{
		corpus: &Corpus{
			Version:      "1.0.0",
			ModuleType:   "BIBLE",
			SourceFormat: "Zefania",
			Attributes:   make(map[string]string),
		},
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if startElem, ok := token.(xml.StartElement); ok {
			if err := dispatchElement(state, startElem, decoder); err != nil {
				return nil, err
			}
		}
	}

	finalizeCorpus(state.corpus)
	return state.corpus, nil
}

// dispatchElement routes an XML element to its appropriate handler
func dispatchElement(state *parserState, elem xml.StartElement, decoder *xml.Decoder) error {
	elementName := strings.ToUpper(elem.Name.Local)
	if handler, ok := elementHandlers[elementName]; ok {
		return handler(state, elem, decoder)
	}
	return nil
}

// handleXMLBible processes the root XMLBIBLE element
func handleXMLBible(state *parserState, elem xml.StartElement, decoder *xml.Decoder) error {
	for _, attr := range elem.Attr {
		attrName := strings.ToLower(attr.Name.Local)
		if attrName == "biblename" {
			state.corpus.Title = attr.Value
			state.corpus.ID = sanitizeID(attr.Value)
		} else if attrName == "language" {
			state.corpus.Language = attr.Value
		}
	}
	return nil
}

// handleBibleBook processes a BIBLEBOOK element
func handleBibleBook(state *parserState, elem xml.StartElement, decoder *xml.Decoder) error {
	bookNum, bookName := extractBookAttributes(elem.Attr)

	osisID := zefaniaBookToOSIS[bookNum]
	if osisID == "" {
		osisID = sanitizeID(bookName)
	}

	state.currentBook = &Document{
		ID:    osisID,
		Title: bookName,
		Order: bookNum,
		Attributes: map[string]string{
			"bnumber": strconv.Itoa(bookNum),
		},
	}
	state.corpus.Documents = append(state.corpus.Documents, state.currentBook)
	return nil
}

// extractBookAttributes extracts book number and name from attributes
func extractBookAttributes(attrs []xml.Attr) (int, string) {
	var bookNum int
	var bookName string
	for _, attr := range attrs {
		attrName := strings.ToLower(attr.Name.Local)
		if attrName == "bnumber" {
			bookNum, _ = strconv.Atoi(attr.Value)
		} else if attrName == "bname" {
			bookName = attr.Value
		}
	}
	return bookNum, bookName
}

// handleChapter processes a CHAPTER element
func handleChapter(state *parserState, elem xml.StartElement, decoder *xml.Decoder) error {
	for _, attr := range elem.Attr {
		if strings.ToLower(attr.Name.Local) == "cnumber" {
			state.currentChapter, _ = strconv.Atoi(attr.Value)
			break
		}
	}
	return nil
}

// handleVerse processes a VERS element
func handleVerse(state *parserState, elem xml.StartElement, decoder *xml.Decoder) error {
	verseNum := extractVerseNumber(elem.Attr)
	text, err := readVerseContent(decoder)
	if err != nil {
		return err
	}

	if text != "" && state.currentBook != nil {
		addVerseToBook(state, verseNum, text)
	}
	return nil
}

// extractVerseNumber extracts the verse number from attributes
func extractVerseNumber(attrs []xml.Attr) int {
	for _, attr := range attrs {
		if strings.ToLower(attr.Name.Local) == "vnumber" {
			verseNum, _ := strconv.Atoi(attr.Value)
			return verseNum
		}
	}
	return 0
}

// readVerseContent reads all text content within a verse element
func readVerseContent(decoder *xml.Decoder) (string, error) {
	var textContent strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		if end, ok := token.(xml.EndElement); ok && strings.ToUpper(end.Name.Local) == "VERS" {
			break
		}
		if charData, ok := token.(xml.CharData); ok {
			textContent.Write(charData)
		}
	}
	return strings.TrimSpace(textContent.String()), nil
}

// addVerseToBook creates a ContentBlock for the verse and adds it to the current book
func addVerseToBook(state *parserState, verseNum int, text string) {
	state.sequence++
	hash := sha256.Sum256([]byte(text))
	osisID := fmt.Sprintf("%s.%d.%d", state.currentBook.ID, state.currentChapter, verseNum)

	cb := &ContentBlock{
		ID:       fmt.Sprintf("cb-%d", state.sequence),
		Sequence: state.sequence,
		Text:     text,
		Hash:     hex.EncodeToString(hash[:]),
		Anchors:  createVerseAnchors(state.sequence, osisID, state.currentBook.ID, state.currentChapter, verseNum),
	}
	state.currentBook.ContentBlocks = append(state.currentBook.ContentBlocks, cb)
}

// createVerseAnchors creates the anchor structure for a verse
func createVerseAnchors(sequence int, osisID, bookID string, chapter, verse int) []*Anchor {
	return []*Anchor{
		{
			ID:       fmt.Sprintf("a-%d-0", sequence),
			Position: 0,
			Spans: []*Span{
				{
					ID:            fmt.Sprintf("s-%s", osisID),
					Type:          "VERSE",
					StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
					Ref: &Ref{
						Book:    bookID,
						Chapter: chapter,
						Verse:   verse,
						OSISID:  osisID,
					},
				},
			},
		},
	}
}

// finalizeCorpus sets default values for the corpus if needed
func finalizeCorpus(corpus *Corpus) {
	if corpus.ID == "" {
		corpus.ID = "zefania"
	}
}

func sanitizeID(s string) string {
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, s)
	return strings.Trim(result, "-")
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

	data, err := os.ReadFile(irPath)
	if err != nil {
		respondError(fmt.Sprintf("failed to read IR file: %v", err))
		return
	}

	var corpus Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		respondError(fmt.Sprintf("failed to parse IR: %v", err))
		return
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".xml")

	// Check for raw Zefania for L0 round-trip
	if raw, ok := corpus.Attributes["_zefania_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
			respondError(fmt.Sprintf("failed to write Zefania: %v", err))
			return
		}

		respond(&EmitNativeResult{
			OutputPath: outputPath,
			Format:     "Zefania",
			LossClass:  "L0",
			LossReport: &LossReport{
				SourceFormat: "IR",
				TargetFormat: "Zefania",
				LossClass:    "L0",
			},
		})
		return
	}

	// Generate Zefania from IR
	zefaniaContent := emitZefaniaFromIR(&corpus)
	if err := os.WriteFile(outputPath, []byte(zefaniaContent), 0600); err != nil {
		respondError(fmt.Sprintf("failed to write Zefania: %v", err))
		return
	}

	respond(&EmitNativeResult{
		OutputPath: outputPath,
		Format:     "Zefania",
		LossClass:  "L1",
		LossReport: &LossReport{
			SourceFormat: "IR",
			TargetFormat: "Zefania",
			LossClass:    "L1",
			Warnings: []string{
				"Zefania regenerated from IR - some formatting may differ",
			},
		},
	})
}

func emitZefaniaFromIR(corpus *Corpus) string {
	var buf strings.Builder

	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
`)
	buf.WriteString(fmt.Sprintf(`<XMLBIBLE biblename="%s"`, escapeXML(corpus.Title)))
	if corpus.Language != "" {
		buf.WriteString(fmt.Sprintf(` language="%s"`, escapeXML(corpus.Language)))
	}
	buf.WriteString(">\n")

	for _, doc := range corpus.Documents {
		bookNum := osisToZefaniaBook[doc.ID]
		if bookNum == 0 {
			bookNum = doc.Order
		}
		buf.WriteString(fmt.Sprintf(`  <BIBLEBOOK bnumber="%d" bname="%s">
`, bookNum, escapeXML(doc.Title)))

		currentChapter := 0
		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						if span.Ref.Chapter != currentChapter {
							if currentChapter > 0 {
								buf.WriteString("    </CHAPTER>\n")
							}
							currentChapter = span.Ref.Chapter
							buf.WriteString(fmt.Sprintf(`    <CHAPTER cnumber="%d">
`, currentChapter))
						}
						buf.WriteString(fmt.Sprintf(`      <VERS vnumber="%d">%s</VERS>
`, span.Ref.Verse, escapeXML(cb.Text)))
					}
				}
			}
		}
		if currentChapter > 0 {
			buf.WriteString("    </CHAPTER>\n")
		}
		buf.WriteString("  </BIBLEBOOK>\n")
	}

	buf.WriteString("</XMLBIBLE>\n")
	return buf.String()
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
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
