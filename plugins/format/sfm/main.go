//go:build !sdk

// Plugin format-sfm handles Standard Format Markers (SFM/Paratext) format.
// SFM is a text-based markup format used by Paratext and SIL translation tools,
// using backslash markers for structure and formatting.
//
// IR Support:
// - extract-ir: Reads SFM to IR (L1)
// - emit-native: Converts IR to SFM format (L1 or L0 with raw storage)
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".sfm" || ext == ".ptx" {
		ipc.MustRespond(&ipc.DetectResult{Detected: true, Format: "SFM", Reason: "SFM extension"})
		return
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	// SFM uses backslash markers like \id, \c, \v
	sfmPattern := regexp.MustCompile(`(?m)^\\(id|c|v|p|s|h)\s`)
	if sfmPattern.MatchString(content) {
		ipc.MustRespond(&ipc.DetectResult{Detected: true, Format: "SFM", Reason: "SFM markers detected"})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{Detected: false, Reason: "not SFM"})
}

func handleIngest(args map[string]interface{}) {
	path := args["path"].(string)
	outputDir := args["output_dir"].(string)
	data, _ := os.ReadFile(path)
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])
	blobDir := filepath.Join(outputDir, hashHex[:2])
	os.MkdirAll(blobDir, 0755)
	os.WriteFile(filepath.Join(blobDir, hashHex), data, 0644)
	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata:   map[string]string{"format": "SFM"},
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
		ID: artifactID, Version: "1.0.0", ModuleType: "BIBLE",
		Title: artifactID, SourceFormat: "SFM", SourceHash: hex.EncodeToString(sourceHash[:]),
		LossClass: "L1", Attributes: map[string]string{"_sfm_raw": hex.EncodeToString(data)},
	}
	corpus.Documents = extractSFMContent(string(data), artifactID)
	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ipc.Document{{ID: artifactID, Title: artifactID, Order: 1}}
	}
	irData, _ := json.MarshalIndent(corpus, "", "  ")
	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	os.WriteFile(irPath, irData, 0644)
	ipc.MustRespond(&ipc.ExtractIRResult{IRPath: irPath, LossClass: "L1"})
}

func extractSFMContent(content, artifactID string) []*ipc.Document {
	doc := &ipc.Document{ID: artifactID, Title: artifactID, Order: 1}
	scanner := bufio.NewScanner(strings.NewReader(content))
	chapter, verse, sequence := 1, 0, 0
	var verseText strings.Builder

	flushVerse := func() {
		if verseText.Len() > 0 && verse > 0 {
			text := strings.TrimSpace(verseText.String())
			if text != "" {
				sequence++
				hash := sha256.Sum256([]byte(text))
				osisID := fmt.Sprintf("%s.%d.%d", artifactID, chapter, verse)
				cb := &ipc.ContentBlock{
					ID: fmt.Sprintf("cb-%d", sequence), Sequence: chapter*1000 + verse, Text: text,
					Hash: hex.EncodeToString(hash[:]),
					Anchors: []*ipc.Anchor{{ID: fmt.Sprintf("a-%d", sequence), Position: 0,
						Spans: []*ipc.Span{{ID: fmt.Sprintf("s-%s", osisID), Type: "VERSE", StartAnchorID: fmt.Sprintf("a-%d", sequence),
							Ref: &ipc.Ref{Book: artifactID, Chapter: chapter, Verse: verse, OSISID: osisID}}}}},
				}
				doc.ContentBlocks = append(doc.ContentBlocks, cb)
			}
			verseText.Reset()
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "\\c ") {
			flushVerse()
			c, _ := strconv.Atoi(strings.TrimSpace(line[3:]))
			if c > 0 {
				chapter = c
			}
		} else if strings.HasPrefix(line, "\\v ") {
			flushVerse()
			parts := strings.SplitN(line[3:], " ", 2)
			v, _ := strconv.Atoi(parts[0])
			if v > 0 {
				verse = v
				if len(parts) > 1 {
					verseText.WriteString(parts[1])
				}
			}
		} else if !strings.HasPrefix(line, "\\") {
			if verse > 0 {
				if verseText.Len() > 0 {
					verseText.WriteString(" ")
				}
				verseText.WriteString(strings.TrimSpace(line))
			}
		}
	}
	flushVerse()
	return []*ipc.Document{doc}
}

func handleEmitNative(args map[string]interface{}) {
	irPath := args["ir_path"].(string)
	outputDir := args["output_dir"].(string)
	data, _ := os.ReadFile(irPath)
	var corpus ipc.Corpus
	json.Unmarshal(data, &corpus)
	outputPath := filepath.Join(outputDir, corpus.ID+".sfm")
	if raw, ok := corpus.Attributes["_sfm_raw"]; ok && raw != "" {
		rawData, _ := hex.DecodeString(raw)
		os.WriteFile(outputPath, rawData, 0644)
		ipc.MustRespond(&ipc.EmitNativeResult{OutputPath: outputPath, Format: "SFM", LossClass: "L0"})
		return
	}
	var buf bytes.Buffer
	for _, doc := range corpus.Documents {
		fmt.Fprintf(&buf, "\\id %s\n", doc.ID)
		lastChapter := 0
		for _, cb := range doc.ContentBlocks {
			chapter, verse := 1, cb.Sequence%1000
			if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 && cb.Anchors[0].Spans[0].Ref != nil {
				chapter = cb.Anchors[0].Spans[0].Ref.Chapter
				verse = cb.Anchors[0].Spans[0].Ref.Verse
			}
			if chapter != lastChapter {
				fmt.Fprintf(&buf, "\\c %d\n", chapter)
				lastChapter = chapter
			}
			fmt.Fprintf(&buf, "\\v %d %s\n", verse, cb.Text)
		}
	}
	os.WriteFile(outputPath, buf.Bytes(), 0644)
	ipc.MustRespond(&ipc.EmitNativeResult{OutputPath: outputPath, Format: "SFM", LossClass: "L1"})
}
