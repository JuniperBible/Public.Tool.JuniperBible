
// Plugin format-sblgnt handles SBL Greek New Testament format.
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

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
	"github.com/JuniperBible/juniper/plugins/sdk/ir"
)

func runSDK() {
	if err := format.Run(&format.Config{
		Name:       "SBLGNT",
		Extensions: []string{".txt", ".tsv"},
		Detect:     detectSBLGNT,
		Parse:      parseSBLGNT,
		Emit:       emitSBLGNT,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectSBLGNT(path string) (*ipc.DetectResult, error) {
	base := strings.ToLower(filepath.Base(path))
	if strings.Contains(base, "sblgnt") || strings.Contains(base, "sbl-gnt") {
		return &ipc.DetectResult{Detected: true, Format: "SBLGNT", Reason: "SBLGNT filename"}, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".txt" || ext == ".tsv" {
		data, err := os.ReadFile(path)
		if err != nil {
			return &ipc.DetectResult{Detected: false, Reason: "not SBLGNT"}, nil
		}

		content := string(data)
		greekPattern := regexp.MustCompile(`[\x{0370}-\x{03FF}]`)
		refPattern := regexp.MustCompile(`\d{8}`)
		if greekPattern.MatchString(content) && refPattern.MatchString(content) {
			return &ipc.DetectResult{Detected: true, Format: "SBLGNT", Reason: "Greek NT format detected"}, nil
		}
	}

	return &ipc.DetectResult{Detected: false, Reason: "not SBLGNT"}, nil
}

func parseSBLGNT(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.Language = "grc"
	corpus.Title = "SBL Greek NT"
	corpus.SourceFormat = "SBLGNT"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L1"
	corpus.Attributes = map[string]string{"_sblgnt_raw": hex.EncodeToString(data)}

	corpus.Documents = extractContent(string(data), artifactID)
	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ir.Document{ir.NewDocument(artifactID, artifactID, 1)}
	}

	return corpus, nil
}

func extractContent(content, artifactID string) []*ir.Document {
	verseWords := make(map[string][]string)
	bookOrder := make(map[string]int)
	scanner := bufio.NewScanner(strings.NewReader(content))
	orderCounter := 0

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}

		ref := fields[0]
		if len(ref) < 6 {
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

		verseWords[verseRef] = append(verseWords[verseRef], fields[len(fields)-1])
	}

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
			bookDocs[book] = ir.NewDocument(book, book, bookOrder[book])
		}

		text := strings.Join(words, " ")
		sequence++
		hash := sha256.Sum256([]byte(text))
		osisID := fmt.Sprintf("%s.%d.%d", book, chapter, verse)

		cb := &ir.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: chapter*1000 + verse,
			Text:     text,
			Hash:     hex.EncodeToString(hash[:]),
			Anchors: []*ir.Anchor{{
				ID:       fmt.Sprintf("a-%d", sequence),
				Position: 0,
				Spans: []*ir.Span{{
					ID:            fmt.Sprintf("s-%s", osisID),
					Type:          "VERSE",
					StartAnchorID: fmt.Sprintf("a-%d", sequence),
					Ref:           &ir.Ref{Book: book, Chapter: chapter, Verse: verse, OSISID: osisID},
				}},
			}},
		}

		bookDocs[book].ContentBlocks = append(bookDocs[book].ContentBlocks, cb)
	}

	var documents []*ir.Document
	for _, doc := range bookDocs {
		documents = append(documents, doc)
	}

	return documents
}

func emitSBLGNT(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".txt")

	// Check for raw SBLGNT for round-trip
	if raw, ok := corpus.Attributes["_sblgnt_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
				return "", fmt.Errorf("failed to write SBLGNT: %w", err)
			}
			return outputPath, nil
		}
	}

	// Generate SBLGNT-style format from IR
	var buf bytes.Buffer
	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			chapter, verse := 1, cb.Sequence%1000
			if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 && cb.Anchors[0].Spans[0].Ref != nil {
				chapter = cb.Anchors[0].Spans[0].Ref.Chapter
				verse = cb.Anchors[0].Spans[0].Ref.Verse
			}
			fmt.Fprintf(&buf, "%s%02d%02d001\t%s\n", doc.ID, chapter, verse, cb.Text)
		}
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0600); err != nil {
		return "", fmt.Errorf("failed to write SBLGNT: %w", err)
	}

	return outputPath, nil
}
