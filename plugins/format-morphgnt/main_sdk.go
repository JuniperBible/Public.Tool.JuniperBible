//go:build sdk

// Plugin format-morphgnt handles MorphGNT Greek NT format.
// MorphGNT is a morphologically parsed Greek New Testament in TSV format.
//
// IR Support:
// - extract-ir: Reads MorphGNT to IR (L1)
// - emit-native: Converts IR to MorphGNT format (L1)
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// MorphWord represents a morphologically analyzed word.
type MorphWord struct {
	Reference  string `json:"reference"`
	PartOfSp   string `json:"part_of_speech"`
	Parsing    string `json:"parsing"`
	Text       string `json:"text"`
	Word       string `json:"word"`
	Normalized string `json:"normalized"`
	Lemma      string `json:"lemma"`
}

func main() {
	if err := format.Run(&format.Config{
		Name:       "MorphGNT",
		Extensions: []string{".txt", ".tsv"},
		Detect:     detectMorphGNT,
		Parse:      parseMorphGNT,
		Emit:       emitMorphGNT,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectMorphGNT(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	// MorphGNT files typically have specific names
	if strings.Contains(base, "morphgnt") || strings.Contains(base, "sblgnt") {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "MorphGNT",
			Reason:   "MorphGNT filename detected",
		}, nil
	}

	// Check for .txt extension and MorphGNT content
	if ext == ".txt" || ext == ".tsv" {
		data, err := os.ReadFile(path)
		if err != nil {
			return &ipc.DetectResult{
				Detected: false,
				Reason:   "not a MorphGNT file",
			}, nil
		}

		content := string(data)
		// MorphGNT format: BBCCVVWWW TAB fields
		morphPattern := regexp.MustCompile(`^\d{8}\t[A-Z\-]+\t`)
		if morphPattern.MatchString(content) {
			return &ipc.DetectResult{
				Detected: true,
				Format:   "MorphGNT",
				Reason:   "MorphGNT TSV format detected",
			}, nil
		}
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "no MorphGNT structure found",
	}, nil
}

func parseMorphGNT(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.Language = "grc"
	corpus.Title = "MorphGNT Greek New Testament"
	corpus.SourceFormat = "MorphGNT"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L1"
	corpus.Attributes = map[string]string{"_morphgnt_raw": hex.EncodeToString(data)}

	// Extract content from MorphGNT format
	corpus.Documents = extractMorphGNTContent(string(data), artifactID)

	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ir.Document{ir.NewDocument(artifactID, artifactID, 1)}
	}

	return corpus, nil
}

func extractMorphGNTContent(content, artifactID string) []*ir.Document {
	// Group words by book/chapter/verse
	verseWords := make(map[string][]MorphWord)
	bookOrder := make(map[string]int)

	scanner := bufio.NewScanner(strings.NewReader(content))
	orderCounter := 0

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}

		// Reference format: BBCCVVWWW
		ref := fields[0]
		if len(ref) < 8 {
			continue
		}

		book := ref[0:2]
		chapter, _ := strconv.Atoi(ref[2:4])
		verse, _ := strconv.Atoi(ref[4:6])

		verseRef := fmt.Sprintf("%s.%d.%d", book, chapter, verse)

		if _, exists := bookOrder[book]; !exists {
			orderCounter++
			bookOrder[book] = orderCounter
		}

		word := MorphWord{
			Reference:  ref,
			PartOfSp:   fields[1],
			Parsing:    fields[2],
			Text:       fields[3],
			Word:       fields[4],
			Normalized: fields[5],
			Lemma:      fields[6],
		}

		verseWords[verseRef] = append(verseWords[verseRef], word)
	}

	// Create documents from verses
	bookDocs := make(map[string]*ir.Document)
	sequence := 0

	for verseRef, words := range verseWords {
		parts := strings.Split(verseRef, ".")
		if len(parts) < 3 {
			continue
		}

		book := parts[0]
		chapter, _ := strconv.Atoi(parts[1])
		verse, _ := strconv.Atoi(parts[2])

		if _, exists := bookDocs[book]; !exists {
			bookDocs[book] = ir.NewDocument(book, getBookName(book), bookOrder[book])
		}

		// Build verse text from words
		var verseText strings.Builder
		for i, w := range words {
			if i > 0 {
				verseText.WriteString(" ")
			}
			verseText.WriteString(w.Text)
		}

		text := verseText.String()
		sequence++
		hash := sha256.Sum256([]byte(text))
		osisID := fmt.Sprintf("%s.%d.%d", book, chapter, verse)

		cb := &ir.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: chapter*1000 + verse,
			Text:     text,
			Hash:     hex.EncodeToString(hash[:]),
			Anchors: []*ir.Anchor{{
				ID:       fmt.Sprintf("a-%d-0", sequence),
				Position: 0,
				Spans: []*ir.Span{{
					ID:            fmt.Sprintf("s-%s", osisID),
					Type:          "VERSE",
					StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
					Ref:           &ir.Ref{Book: book, Chapter: chapter, Verse: verse, OSISID: osisID},
				}},
			}},
		}

		bookDocs[book].ContentBlocks = append(bookDocs[book].ContentBlocks, cb)
	}

	// Sort content blocks within each document
	var documents []*ir.Document
	for _, doc := range bookDocs {
		// Sort by sequence
		for i := 0; i < len(doc.ContentBlocks); i++ {
			for j := i + 1; j < len(doc.ContentBlocks); j++ {
				if doc.ContentBlocks[i].Sequence > doc.ContentBlocks[j].Sequence {
					doc.ContentBlocks[i], doc.ContentBlocks[j] = doc.ContentBlocks[j], doc.ContentBlocks[i]
				}
			}
		}
		documents = append(documents, doc)
	}

	// Sort documents by order
	for i := 0; i < len(documents); i++ {
		for j := i + 1; j < len(documents); j++ {
			if documents[i].Order > documents[j].Order {
				documents[i], documents[j] = documents[j], documents[i]
			}
		}
	}

	return documents
}

func getBookName(code string) string {
	bookNames := map[string]string{
		"01": "Matthew", "02": "Mark", "03": "Luke", "04": "John",
		"05": "Acts", "06": "Romans", "07": "1 Corinthians", "08": "2 Corinthians",
		"09": "Galatians", "10": "Ephesians", "11": "Philippians", "12": "Colossians",
		"13": "1 Thessalonians", "14": "2 Thessalonians", "15": "1 Timothy", "16": "2 Timothy",
		"17": "Titus", "18": "Philemon", "19": "Hebrews", "20": "James",
		"21": "1 Peter", "22": "2 Peter", "23": "1 John", "24": "2 John",
		"25": "3 John", "26": "Jude", "27": "Revelation",
	}
	if name, ok := bookNames[code]; ok {
		return name
	}
	return code
}

func emitMorphGNT(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".txt")

	// Check for raw MorphGNT for round-trip
	if raw, ok := corpus.Attributes["_morphgnt_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0644); err != nil {
				return "", fmt.Errorf("failed to write MorphGNT: %w", err)
			}
			return outputPath, nil
		}
	}

	// Generate MorphGNT-style format from IR
	var buf bytes.Buffer

	bookCodes := map[string]string{
		"Matthew": "01", "Mark": "02", "Luke": "03", "John": "04",
		"Acts": "05", "Romans": "06", "1 Corinthians": "07", "2 Corinthians": "08",
		"Galatians": "09", "Ephesians": "10", "Philippians": "11", "Colossians": "12",
	}

	for _, doc := range corpus.Documents {
		bookCode := doc.ID
		if code, ok := bookCodes[doc.Title]; ok {
			bookCode = code
		}

		for _, cb := range doc.ContentBlocks {
			chapter := 1
			verse := cb.Sequence

			if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 {
				if ref := cb.Anchors[0].Spans[0].Ref; ref != nil {
					chapter = ref.Chapter
					verse = ref.Verse
				}
			}

			words := strings.Fields(cb.Text)
			for i, word := range words {
				ref := fmt.Sprintf("%s%02d%02d%03d", bookCode, chapter, verse, i+1)
				// Simplified output: ref, POS, parsing, text, word, normalized, lemma
				fmt.Fprintf(&buf, "%s\tN-\t----\t%s\t%s\t%s\t%s\n",
					ref, word, word, word, word)
			}
		}
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write MorphGNT: %w", err)
	}

	return outputPath, nil
}
