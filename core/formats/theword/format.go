// Package theword implements the TheWord/PalmBible+ Bible format.
// TheWord format uses line-based text files where each line is a verse.
// Supported extensions: .ont (OT), .nt (NT), .twm (full module)
//
// IR Support:
// - extract-ir: Extracts IR from TheWord text (L0 lossless via raw storage)
// - emit-native: Converts IR back to TheWord format (L0)
package theword

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// Config defines the TheWord format plugin configuration.
var Config = &format.Config{
	PluginID:   "format.theword",
	Name:       "TheWord",
	Extensions: []string{".ont", ".nt", ".twm"},
	Detect:     detect,
	Parse:      parse,
	Emit:       emit,
}

// Simplified OT books for demo (full implementation would have all 66)
var otBooks = []string{"Gen", "Exod", "Lev", "Num", "Deut", "Josh", "Judg", "Ruth", "1Sam", "2Sam", "1Kgs", "2Kgs", "1Chr", "2Chr", "Ezra", "Neh", "Esth", "Job", "Ps", "Prov", "Eccl", "Song", "Isa", "Jer", "Lam", "Ezek", "Dan", "Hos", "Joel", "Amos", "Obad", "Jonah", "Mic", "Nah", "Hab", "Zeph", "Hag", "Zech", "Mal"}
var ntBooks = []string{"Matt", "Mark", "Luke", "John", "Acts", "Rom", "1Cor", "2Cor", "Gal", "Eph", "Phil", "Col", "1Thess", "2Thess", "1Tim", "2Tim", "Titus", "Phlm", "Heb", "Jas", "1Pet", "2Pet", "1John", "2John", "3John", "Jude", "Rev"}

func detect(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "path is a directory, not a file",
		}, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	validExts := map[string]string{
		".ont": "Old Testament",
		".nt":  "New Testament",
		".twm": "Full Module",
	}

	moduleType, isValid := validExts[ext]
	if !isValid {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("extension %s is not a known TheWord format", ext),
		}, nil
	}

	// Check if file has reasonable line count for a Bible
	file, err := os.Open(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open file: %v", err),
		}, nil
	}
	defer file.Close()

	lineCount := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() && lineCount < 100 {
		lineCount++
	}

	if lineCount < 10 {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "file has too few lines for TheWord format",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: true,
		Format:   "TheWord",
		Reason:   fmt.Sprintf("TheWord %s detected", moduleType),
	}, nil
}

var extBooks = map[string]func() []string{
	".ont": func() []string { return otBooks },
	".nt":  func() []string { return ntBooks },
}

func booksForExt(ext string) []string {
	if fn, ok := extBooks[ext]; ok {
		return fn()
	}
	return append(otBooks, ntBooks...)
}

func newContentBlock(sequence, chapter, verse int, book, text string) *ir.ContentBlock {
	hash := sha256.Sum256([]byte(text))
	osisID := fmt.Sprintf("%s.%d.%d", book, chapter, verse)
	return &ir.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", sequence),
		Sequence: sequence,
		Text:     text,
		Hash:     hex.EncodeToString(hash[:]),
		Anchors: []*ir.Anchor{
			{
				ID:       fmt.Sprintf("a-%d-0", sequence),
				Position: 0,
				Spans: []*ir.Span{
					{
						ID:            fmt.Sprintf("s-%s", osisID),
						Type:          "VERSE",
						StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
						Ref: &ir.Ref{
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
}

func parseVerses(lines []string, books []string, corpus *ir.Corpus) {
	state := &parseState{
		lines:    lines,
		bookDocs: make(map[string]*ir.Document),
		corpus:   corpus,
	}
	for _, book := range books {
		if state.lineIdx >= len(lines) {
			break
		}
		state.parseBook(book)
	}
}

type parseState struct {
	lines    []string
	lineIdx  int
	sequence int
	bookDocs map[string]*ir.Document
	corpus   *ir.Corpus
}

func (ps *parseState) parseBook(book string) {
	doc := ps.getOrCreateDoc(book)
	for chapter := 1; chapter <= 3 && ps.lineIdx < len(ps.lines); chapter++ {
		ps.parseChapter(doc, book, chapter)
	}
}

func (ps *parseState) getOrCreateDoc(book string) *ir.Document {
	if doc, ok := ps.bookDocs[book]; ok {
		return doc
	}
	doc := &ir.Document{ID: book, Title: book, Order: len(ps.bookDocs) + 1}
	ps.bookDocs[book] = doc
	ps.corpus.Documents = append(ps.corpus.Documents, doc)
	return doc
}

func (ps *parseState) parseChapter(doc *ir.Document, book string, chapter int) {
	for verse := 1; verse <= 10 && ps.lineIdx < len(ps.lines); verse++ {
		text := strings.TrimSpace(ps.lines[ps.lineIdx])
		ps.lineIdx++
		if text == "" {
			continue
		}
		ps.sequence++
		doc.ContentBlocks = append(doc.ContentBlocks, newContentBlock(ps.sequence, chapter, verse, book, text))
	}
}

func parse(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	ext := strings.ToLower(filepath.Ext(path))
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ir.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "TheWord",
		LossClass:    "L0",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		Attributes:   make(map[string]string),
	}
	corpus.Attributes["_theword_raw"] = string(data)
	corpus.Attributes["_theword_ext"] = ext

	parseVerses(strings.Split(string(data), "\n"), booksForExt(ext), corpus)

	return corpus, nil
}

func emit(corpus *ir.Corpus, outputDir string) (string, error) {
	// Determine output extension
	ext := ".twm"
	if e, ok := corpus.Attributes["_theword_ext"]; ok {
		ext = e
	}

	outputPath := filepath.Join(outputDir, corpus.ID+ext)

	// Check for raw content for L0 round-trip
	if raw, ok := corpus.Attributes["_theword_raw"]; ok && raw != "" {
		if err := os.WriteFile(outputPath, []byte(raw), 0600); err != nil {
			return "", fmt.Errorf("failed to write TheWord file: %w", err)
		}
		return outputPath, nil
	}

	// Generate TheWord format from IR
	var buf strings.Builder
	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			buf.WriteString(cb.Text)
			buf.WriteString("\n")
		}
	}

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0600); err != nil {
		return "", fmt.Errorf("failed to write TheWord file: %w", err)
	}

	return outputPath, nil
}
