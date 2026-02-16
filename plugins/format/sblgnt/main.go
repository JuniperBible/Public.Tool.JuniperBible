//go:build !sdk

// Plugin format-sblgnt handles SBL Greek New Testament format.
// SBLGNT is a morphologically analyzed Greek New Testament in text format
// with parsing codes for each word.
//
// IR Support:
// - extract-ir: Reads SBLGNT to IR (L1)
// - emit-native: Converts IR to SBLGNT format (L1 or L0 with raw storage)
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

func main() {
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorf("failed to decode: %v", err)
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
		ipc.RespondError("unknown command")
	}
}

func handleDetect(args map[string]interface{}) {
	path := args["path"].(string)
	base := strings.ToLower(filepath.Base(path))
	if strings.Contains(base, "sblgnt") || strings.Contains(base, "sbl-gnt") {
		ipc.MustRespond(&ipc.DetectResult{Detected: true, Format: "SBLGNT", Reason: "SBLGNT filename"})
		return
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".txt" || ext == ".tsv" {
		data, _ := os.ReadFile(path)
		content := string(data)
		greekPattern := regexp.MustCompile(`[\x{0370}-\x{03FF}]`)
		refPattern := regexp.MustCompile(`\d{8}`)
		if greekPattern.MatchString(content) && refPattern.MatchString(content) {
			ipc.MustRespond(&ipc.DetectResult{Detected: true, Format: "SBLGNT", Reason: "Greek NT format detected"})
			return
		}
	}
	ipc.MustRespond(&ipc.DetectResult{Detected: false, Reason: "not SBLGNT"})
}

func handleIngest(args map[string]interface{}) {
	path := args["path"].(string)
	outputDir := args["output_dir"].(string)
	data, _ := os.ReadFile(path)
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])
	blobDir := filepath.Join(outputDir, hashHex[:2])
	os.MkdirAll(blobDir, 0755)
	os.WriteFile(filepath.Join(blobDir, hashHex), data, 0600)
	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata:   map[string]string{"format": "SBLGNT"},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path := args["path"].(string)
	info, _ := os.Stat(path)
	ipc.MustRespond(&ipc.EnumerateResult{Entries: []ipc.EnumerateEntry{{Path: filepath.Base(path), SizeBytes: info.Size()}}})
}

func handleExtractIR(args map[string]interface{}) {
	path := args["path"].(string)
	outputDir := args["output_dir"].(string)
	data, _ := os.ReadFile(path)
	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ipc.Corpus{
		ID: artifactID, Version: "1.0.0", ModuleType: "BIBLE", Language: "grc",
		Title: "SBL Greek NT", SourceFormat: "SBLGNT", SourceHash: hex.EncodeToString(sourceHash[:]),
		LossClass: "L1", Attributes: map[string]string{"_sblgnt_raw": hex.EncodeToString(data)},
	}
	corpus.Documents = extractContent(string(data), artifactID)
	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ipc.Document{{ID: artifactID, Title: artifactID, Order: 1}}
	}
	irData, _ := json.MarshalIndent(corpus, "", "  ")
	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	os.WriteFile(irPath, irData, 0600)
	ipc.MustRespond(&ipc.ExtractIRResult{IRPath: irPath, LossClass: "L1"})
}

func extractContent(content, artifactID string) []*ipc.Document {
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
	bookDocs := make(map[string]*ipc.Document)
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
			bookDocs[book] = &ipc.Document{ID: book, Title: book, Order: bookOrder[book]}
		}
		text := strings.Join(words, " ")
		sequence++
		hash := sha256.Sum256([]byte(text))
		osisID := fmt.Sprintf("%s.%d.%d", book, chapter, verse)
		cb := &ipc.ContentBlock{
			ID: fmt.Sprintf("cb-%d", sequence), Sequence: chapter*1000 + verse, Text: text,
			Hash: hex.EncodeToString(hash[:]),
			Anchors: []*ipc.Anchor{{ID: fmt.Sprintf("a-%d", sequence), Position: 0,
				Spans: []*ipc.Span{{ID: fmt.Sprintf("s-%s", osisID), Type: "VERSE", StartAnchorID: fmt.Sprintf("a-%d", sequence),
					Ref: &ipc.Ref{Book: book, Chapter: chapter, Verse: verse, OSISID: osisID}}}}},
		}
		bookDocs[book].ContentBlocks = append(bookDocs[book].ContentBlocks, cb)
	}
	var documents []*ipc.Document
	for _, doc := range bookDocs {
		documents = append(documents, doc)
	}
	return documents
}

func handleEmitNative(args map[string]interface{}) {
	irPath := args["ir_path"].(string)
	outputDir := args["output_dir"].(string)
	data, _ := os.ReadFile(irPath)
	var corpus ipc.Corpus
	json.Unmarshal(data, &corpus)
	outputPath := filepath.Join(outputDir, corpus.ID+".txt")
	if raw, ok := corpus.Attributes["_sblgnt_raw"]; ok && raw != "" {
		rawData, _ := hex.DecodeString(raw)
		os.WriteFile(outputPath, rawData, 0600)
		ipc.MustRespond(&ipc.EmitNativeResult{OutputPath: outputPath, Format: "SBLGNT", LossClass: "L0"})
		return
	}
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
	os.WriteFile(outputPath, buf.Bytes(), 0600)
	ipc.MustRespond(&ipc.EmitNativeResult{OutputPath: outputPath, Format: "SBLGNT", LossClass: "L1"})
}
