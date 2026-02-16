// Plugin format-usfm handles USFM (Unified Standard Format Markers) Bible files.
// It supports L0 lossless round-trip conversion through IR.
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

var (
	verseNumRegex = regexp.MustCompile(`^(\d+)(?:-(\d+))?`)
	chapterRegex  = regexp.MustCompile(`^(\d+)`)
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
	if err := format.Run(&format.Config{
		Name:       "USFM",
		Extensions: []string{".usfm", ".sfm", ".ptx"},
		Detect:     detectUSFM,
		Parse:      parseUSFM,
		Emit:       emitUSFM,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectUSFM(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{Detected: false, Reason: "path is a directory"}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot read: %v", err)}, nil
	}

	content := string(data)
	if strings.Contains(content, "\\id ") || strings.Contains(content, "\\c ") ||
		strings.Contains(content, "\\v ") || strings.Contains(content, "\\p") {
		return &ipc.DetectResult{Detected: true, Format: "USFM", Reason: "USFM markers detected"}, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".usfm" || ext == ".sfm" || ext == ".ptx" {
		return &ipc.DetectResult{Detected: true, Format: "USFM", Reason: "USFM file extension detected"}, nil
	}

	return &ipc.DetectResult{Detected: false, Reason: "not a USFM file"}, nil
}

func parseUSFM(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)
	sourceHash := sha256.Sum256(data)

	corpus := ir.NewCorpus("", "BIBLE", "")
	corpus.SourceFormat = "USFM"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L0"
	corpus.Attributes = map[string]string{"_usfm_raw": content}

	var currentDoc *ir.Document
	var currentChapter int
	var blockSeq int

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "\\") {
			parts := strings.SplitN(trimmed, " ", 2)
			marker := strings.TrimPrefix(parts[0], "\\")
			var value string
			if len(parts) > 1 {
				value = parts[1]
			}

			switch marker {
			case "id":
				idParts := strings.Fields(value)
				if len(idParts) > 0 {
					bookID := strings.ToUpper(idParts[0])
					corpus.ID = bookID
					currentDoc = ir.NewDocument(bookID, "", len(corpus.Documents)+1)
					if name, ok := bookNames[bookID]; ok {
						currentDoc.Title = name
					}
					corpus.Documents = append(corpus.Documents, currentDoc)
				}

			case "h", "toc1", "toc2", "toc3":
				if currentDoc != nil && value != "" {
					if marker == "h" && currentDoc.Title == "" {
						currentDoc.Title = value
					}
					if currentDoc.Attributes == nil {
						currentDoc.Attributes = make(map[string]string)
					}
					currentDoc.Attributes[marker] = value
				}

			case "mt", "mt1", "mt2", "mt3":
				if corpus.Title == "" && value != "" {
					corpus.Title = value
				}

			case "c":
				if matches := chapterRegex.FindStringSubmatch(value); len(matches) > 0 {
					currentChapter, _ = strconv.Atoi(matches[1])
				}

			case "v":
				if currentDoc != nil {
					verseText := value
					verseNum := 0
					verseEnd := 0

					if matches := verseNumRegex.FindStringSubmatch(value); len(matches) > 0 {
						verseNum, _ = strconv.Atoi(matches[1])
						if matches[2] != "" {
							verseEnd, _ = strconv.Atoi(matches[2])
						}
						verseText = strings.TrimSpace(value[len(matches[0]):])
					}

					if verseText != "" {
						blockSeq++
						osisID := fmt.Sprintf("%s.%d.%d", corpus.ID, currentChapter, verseNum)
						hash := sha256.Sum256([]byte(verseText))

						cb := &ir.ContentBlock{
							ID:       fmt.Sprintf("cb-%d", blockSeq),
							Sequence: blockSeq,
							Text:     verseText,
							Hash:     hex.EncodeToString(hash[:]),
							Anchors: []*ir.Anchor{{
								ID:       fmt.Sprintf("a-%d-0", blockSeq),
								Position: 0,
								Spans: []*ir.Span{{
									ID:            fmt.Sprintf("s-%s", osisID),
									Type:          "VERSE",
									StartAnchorID: fmt.Sprintf("a-%d-0", blockSeq),
									Ref:           &ir.Ref{Book: corpus.ID, Chapter: currentChapter, Verse: verseNum, VerseEnd: verseEnd, OSISID: osisID},
								}},
							}},
						}

						currentDoc.ContentBlocks = append(currentDoc.ContentBlocks, cb)
					}
				}

			case "p", "m", "pi", "mi", "nb":
				if currentDoc != nil && value != "" {
					blockSeq++
					hash := sha256.Sum256([]byte(value))
					cb := &ir.ContentBlock{
						ID:       fmt.Sprintf("cb-%d", blockSeq),
						Sequence: blockSeq,
						Text:     value,
						Hash:     hex.EncodeToString(hash[:]),
					}
					currentDoc.ContentBlocks = append(currentDoc.ContentBlocks, cb)
				}
			}
		}
	}

	return corpus, nil
}

func emitUSFM(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".usfm")

	// Check for raw USFM for L0 round-trip
	if rawUSFM, ok := corpus.Attributes["_usfm_raw"]; ok && rawUSFM != "" {
		if err := os.WriteFile(outputPath, []byte(rawUSFM), 0644); err != nil {
			return "", fmt.Errorf("failed to write USFM: %w", err)
		}
		return outputPath, nil
	}

	// Generate USFM from IR
	var buf bytes.Buffer

	for _, doc := range corpus.Documents {
		buf.WriteString(fmt.Sprintf("\\id %s\n", doc.ID))

		if doc.Title != "" {
			buf.WriteString(fmt.Sprintf("\\h %s\n", doc.Title))
		}

		if doc.Attributes != nil {
			for key, val := range doc.Attributes {
				if key != "h" && !strings.HasPrefix(key, "_") {
					buf.WriteString(fmt.Sprintf("\\%s %s\n", key, val))
				}
			}
		}

		currentChapter := 0

		for _, block := range doc.ContentBlocks {
			for _, anchor := range block.Anchors {
				for _, span := range anchor.Spans {
					if span.Type == "VERSE" && span.Ref != nil {
						if span.Ref.Chapter != currentChapter {
							currentChapter = span.Ref.Chapter
							buf.WriteString(fmt.Sprintf("\\c %d\n", currentChapter))
						}

						if span.Ref.VerseEnd > 0 && span.Ref.VerseEnd != span.Ref.Verse {
							buf.WriteString(fmt.Sprintf("\\v %d-%d %s\n", span.Ref.Verse, span.Ref.VerseEnd, block.Text))
						} else {
							buf.WriteString(fmt.Sprintf("\\v %d %s\n", span.Ref.Verse, block.Text))
						}
						break
					}
				}
			}

			if len(block.Anchors) == 0 && block.Text != "" {
				buf.WriteString(fmt.Sprintf("\\p %s\n", block.Text))
			}
		}
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write USFM: %w", err)
	}

	return outputPath, nil
}
