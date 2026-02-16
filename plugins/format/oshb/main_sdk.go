//go:build sdk

// Plugin format-oshb handles OpenScriptures Hebrew Bible format.
// OSHB is a morphologically parsed Hebrew Bible in XML/TSV format.
//
// IR Support:
// - extract-ir: Reads OSHB to IR (L1)
// - emit-native: Converts IR to OSHB format (L1)
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

func main() {
	format.Run(&format.Config{
		Name:       "format-oshb",
		Extensions: []string{".xml", ".txt", ".tsv"},
		Detect:     detect,
		Parse:      parse,
		Emit:       emit,
		Enumerate:  enumerate,
	})
}

func detect(path string) (*format.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	// OSHB files typically have specific names
	if strings.Contains(base, "oshb") || strings.Contains(base, "openscriptures") {
		return &format.DetectResult{
			Detected: true,
			Format:   "OSHB",
			Reason:   "OSHB filename detected",
		}, nil
	}

	// Check for TSV/TXT with Hebrew morphology
	if ext == ".txt" || ext == ".tsv" || ext == ".xml" {
		data, err := os.ReadFile(path)
		if err != nil {
			return &format.DetectResult{
				Detected: false,
				Reason:   "not an OSHB file",
			}, nil
		}

		content := string(data)
		// OSHB format patterns
		oshbPattern := regexp.MustCompile(`(?:Gen|Exod?|Lev|Num|Deut)\.\d+\.\d+`)
		hebrewPattern := regexp.MustCompile(`[\x{0590}-\x{05FF}]`)

		if oshbPattern.MatchString(content) && hebrewPattern.MatchString(content) {
			return &format.DetectResult{
				Detected: true,
				Format:   "OSHB",
				Reason:   "OSHB Hebrew morphology detected",
			}, nil
		}

		// Check for OSIS-style Hebrew
		if strings.Contains(content, "<osis") && hebrewPattern.MatchString(content) {
			return &format.DetectResult{
				Detected: true,
				Format:   "OSHB",
				Reason:   "OSHB OSIS Hebrew format detected",
			}, nil
		}
	}
	return &format.DetectResult{
		Detected: false,
		Reason:   "no OSHB structure found",
	}, nil
}

func parse(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ir.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		Language:     "hbo",
		Title:        "OpenScriptures Hebrew Bible",
		SourceFormat: "OSHB",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L1",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_oshb_raw"] = hex.EncodeToString(data)

	// Extract content
	corpus.Documents = extractOSHBContent(string(data), artifactID)

	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ir.Document{
			{
				ID:    artifactID,
				Title: artifactID,
				Order: 1,
			},
		}
	}

	return corpus, nil
}

func extractOSHBContent(content, artifactID string) []*ir.Document {
	verseWords := make(map[string][]string)
	bookOrder := make(map[string]int)

	// Parse OSHB-style references: Book.Chapter.Verse Word
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

	// Create documents
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
			bookDocs[book] = &ir.Document{
				ID:    book,
				Title: book,
				Order: bookOrder[book],
			}
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

func emit(corpus *ir.Corpus, outputPath string) error {
	// Check for raw OSHB for round-trip
	if raw, ok := corpus.Attributes["_oshb_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
				return fmt.Errorf("failed to write OSHB: %w", err)
			}
			return nil
		}
	}

	// Generate OSHB-style format from IR
	var buf bytes.Buffer

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

			fmt.Fprintf(&buf, "%s.%d.%d %s\n", doc.ID, chapter, verse, cb.Text)
		}
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write OSHB: %w", err)
	}
	return nil
}

func enumerate(path string) ([]ipc.EnumerateEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	return []ipc.EnumerateEntry{
		{
			Path:      filepath.Base(path),
			SizeBytes: info.Size(),
			IsDir:     false,
			Metadata:  map[string]string{"format": "OSHB"},
		},
	}, nil
}
