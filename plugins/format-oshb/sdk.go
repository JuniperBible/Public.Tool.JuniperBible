
// Plugin format-oshb handles OpenScriptures Hebrew Bible format.
// OSHB is a morphologically parsed Hebrew Bible in XML/TSV format.
//
// IR Support:
// - extract-ir: Reads OSHB to IR (L1)
// - emit-native: Converts IR to OSHB format (L1)
package main

import (
	"bufio"
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
		Name:       "OSHB",
		Extensions: []string{".txt", ".tsv", ".xml"},
		Detect:     detectOSHB,
		Parse:      parseOSHB,
		Emit:       emitOSHB,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectOSHB(path string) (*ipc.DetectResult, error) {
	base := strings.ToLower(filepath.Base(path))

	if strings.Contains(base, "oshb") || strings.Contains(base, "openscriptures") {
		return &ipc.DetectResult{Detected: true, Format: "OSHB", Reason: "OSHB filename detected"}, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".txt" || ext == ".tsv" || ext == ".xml" {
		data, err := os.ReadFile(path)
		if err != nil {
			return &ipc.DetectResult{Detected: false, Reason: "not an OSHB file"}, nil
		}

		content := string(data)
		oshbPattern := regexp.MustCompile(`(?:Gen|Exod?|Lev|Num|Deut)\.\d+\.\d+`)
		hebrewPattern := regexp.MustCompile(`[\x{0590}-\x{05FF}]`)

		if oshbPattern.MatchString(content) && hebrewPattern.MatchString(content) {
			return &ipc.DetectResult{Detected: true, Format: "OSHB", Reason: "OSHB Hebrew morphology detected"}, nil
		}

		if strings.Contains(content, "<osis") && hebrewPattern.MatchString(content) {
			return &ipc.DetectResult{Detected: true, Format: "OSHB", Reason: "OSHB OSIS Hebrew format detected"}, nil
		}
	}

	return &ipc.DetectResult{Detected: false, Reason: "no OSHB structure found"}, nil
}

func parseOSHB(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "OSHB"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L1"
	corpus.Language = "hbo"
	corpus.Title = "OpenScriptures Hebrew Bible"
	corpus.Attributes = map[string]string{"_oshb_raw": hex.EncodeToString(data)}

	corpus.Documents = extractOSHBContent(string(data), artifactID)

	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ir.Document{ir.NewDocument(artifactID, artifactID, 1)}
	}

	return corpus, nil
}

func extractOSHBContent(content, artifactID string) []*ir.Document {
	verseWords := make(map[string][]string)
	bookOrder := make(map[string]int)

	versePattern := regexp.MustCompile(`(Gen|Exod?|Lev|Num|Deut|Josh|Judg|Ruth|[12]Sam|[12]Kgs|[12]Chr|Ezra|Neh|Esth|Job|Ps|Prov|Eccl|Song|Isa|Jer|Lam|Ezek|Dan|Hos|Joel|Amos|Obad|Jonah|Mic|Nah|Hab|Zeph|Hag|Zech|Mal)\.(\d+)\.(\d+)\s+(.+)`)

	scanner := bufio.NewScanner(strings.NewReader(content))
	orderCounter := 0

	for scanner.Scan() {
		line := scanner.Text()
		matches := versePattern.FindStringSubmatch(line)
		if len(matches) >= 5 {
			book := matches[1]
			chapter := matches[2]
			verse := matches[3]
			text := matches[4]

			verseRef := fmt.Sprintf("%s.%s.%s", book, chapter, verse)

			if _, exists := bookOrder[book]; !exists {
				orderCounter++
				bookOrder[book] = orderCounter
			}

			verseWords[verseRef] = append(verseWords[verseRef], text)
		}
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

	var documents []*ir.Document
	for _, doc := range bookDocs {
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

	return documents
}

func emitOSHB(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".txt")

	// Check for raw OSHB for round-trip
	if raw, ok := corpus.Attributes["_oshb_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
				return "", fmt.Errorf("failed to write OSHB: %w", err)
			}
			return outputPath, nil
		}
	}

	// Generate OSHB-style format from IR
	var buf strings.Builder

	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			chapter := 1
			verse := cb.Sequence % 1000

			if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 {
				if ref := cb.Anchors[0].Spans[0].Ref; ref != nil {
					chapter = ref.Chapter
					verse = ref.Verse
				}
			}

			buf.WriteString(fmt.Sprintf("%s.%d.%d %s\n", doc.ID, chapter, verse, cb.Text))
		}
	}

	if err := os.WriteFile(outputPath, []byte(buf.String()), 0600); err != nil {
		return "", fmt.Errorf("failed to write OSHB: %w", err)
	}

	return outputPath, nil
}
