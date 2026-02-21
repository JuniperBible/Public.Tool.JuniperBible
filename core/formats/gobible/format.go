// Package gobible provides canonical GoBible J2ME format support.
// GoBible uses JAR-based .gbk files containing binary verse data.
//
// IR Support:
// - extract-ir: Reads GoBible format to IR (L3)
// - emit-native: Converts IR to GoBible-compatible format (L3)
package gobible

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
	"github.com/JuniperBible/juniper/plugins/sdk/ir"
)

// Config defines the GoBible format plugin.
var Config = &format.Config{
	PluginID:   "format.gobible",
	Name:       "GoBible",
	Extensions: []string{".gbk"},
	Detect:     detectGoBible,
	Parse:      parseGoBible,
	Emit:       emitGoBible,
	Enumerate:  enumerateGoBible,
}

func goBibleDetected(reason string) *ipc.DetectResult {
	return &ipc.DetectResult{Detected: true, Format: "GoBible", Reason: reason}
}

func goBibleNotDetected(reason string) *ipc.DetectResult {
	return &ipc.DetectResult{Detected: false, Reason: reason}
}

func detectGoBible(path string) (*ipc.DetectResult, error) {
	if strings.ToLower(filepath.Ext(path)) == ".gbk" {
		return goBibleDetected("GoBible file extension detected"), nil
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		return goBibleNotDetected("not a GoBible file"), nil
	}
	defer zr.Close()

	if hasGoBibleStructure(zr.File) {
		return goBibleDetected("GoBible JAR structure detected"), nil
	}

	return goBibleNotDetected("no GoBible structure found"), nil
}

func hasGoBibleStructure(files []*zip.File) bool {
	for _, f := range files {
		name := strings.ToLower(f.Name)
		if name == "collections" || name == "bible/collections" {
			return true
		}
		if strings.HasPrefix(name, "bible/") || strings.Contains(name, "verse") {
			return true
		}
	}
	return false
}

func enumerateGoBible(path string) (*ipc.EnumerateResult, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("failed to stat: %w", err)
		}
		return &ipc.EnumerateResult{
			Entries: []ipc.EnumerateEntry{
				{Path: filepath.Base(path), SizeBytes: info.Size(), IsDir: false, Metadata: map[string]string{"format": "GoBible"}},
			},
		}, nil
	}
	defer zr.Close()

	var entries []ipc.EnumerateEntry
	for _, f := range zr.File {
		entries = append(entries, ipc.EnumerateEntry{
			Path:      f.Name,
			SizeBytes: int64(f.UncompressedSize64),
			IsDir:     f.FileInfo().IsDir(),
		})
	}

	return &ipc.EnumerateResult{Entries: entries}, nil
}

func parseGoBible(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "GoBible"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L3"
	corpus.Attributes = map[string]string{"_gobible_raw": hex.EncodeToString(data)}

	corpus.Documents = extractGoBibleContent(data, artifactID)

	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ir.Document{ir.NewDocument(artifactID, artifactID, 1)}
	}

	return corpus, nil
}

func extractGoBibleContent(data []byte, artifactID string) []*ir.Document {
	doc := ir.NewDocument(artifactID, artifactID, 1)

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return []*ir.Document{doc}
	}

	sequence := 0
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".txt") || strings.Contains(f.Name, "verse") {
			appendTextLines(doc, f, &sequence)
		}
		if strings.HasPrefix(f.Name, "Bible/") || f.Name == "Collections" {
			appendBinaryTexts(doc, f, &sequence)
		}
	}

	return []*ir.Document{doc}
}

func readZipFileContent(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func appendContentBlock(doc *ir.Document, text string, sequence *int) {
	*sequence++
	hash := sha256.Sum256([]byte(text))
	doc.ContentBlocks = append(doc.ContentBlocks, &ir.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", *sequence),
		Sequence: *sequence,
		Text:     text,
		Hash:     hex.EncodeToString(hash[:]),
	})
}

func appendTextLines(doc *ir.Document, f *zip.File, sequence *int) {
	content, err := readZipFileContent(f)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if len(line) > 5 {
			appendContentBlock(doc, line, sequence)
		}
	}
}

func appendBinaryTexts(doc *ir.Document, f *zip.File, sequence *int) {
	content, err := readZipFileContent(f)
	if err != nil {
		return
	}
	for _, text := range extractTextFromBinary(content) {
		if len(text) > 5 {
			appendContentBlock(doc, text, sequence)
		}
	}
}

func extractTextFromBinary(data []byte) []string {
	var texts []string
	var current strings.Builder

	for i := 0; i < len(data); i++ {
		i = processTextByte(data, i, &current, &texts)
	}

	flushTextIfLongEnough(&current, &texts)
	return texts
}

func processTextByte(data []byte, i int, current *strings.Builder, texts *[]string) int {
	b := data[i]
	if isASCIIPrintable(b) {
		current.WriteByte(b)
		return i
	}
	if isUTF8TwoByteStart(b) && i+1 < len(data) {
		current.WriteByte(b)
		i++
		current.WriteByte(data[i])
		return i
	}
	flushTextIfLongEnough(current, texts)
	current.Reset()
	return i
}

func isASCIIPrintable(b byte) bool {
	return b >= 32 && b < 127
}

func isUTF8TwoByteStart(b byte) bool {
	return b >= 0xC0 && b < 0xE0
}

func flushTextIfLongEnough(current *strings.Builder, texts *[]string) {
	if current.Len() > 10 {
		*texts = append(*texts, current.String())
	}
}

func emitGoBible(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".gbk")

	if raw, ok := corpus.Attributes["_gobible_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
				return "", fmt.Errorf("failed to write GoBible: %w", err)
			}
			return outputPath, nil
		}
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	manifest := "Manifest-Version: 1.0\nMIDlet-Name: " + corpus.Title + "\n"
	mf, _ := zw.Create("META-INF/MANIFEST.MF")
	mf.Write([]byte(manifest))

	collectionsData := createCollectionsFile(corpus)
	cf, _ := zw.Create("Bible/Collections")
	cf.Write(collectionsData)

	for i, doc := range corpus.Documents {
		bookData := createBookDataFile(doc)
		bf, _ := zw.Create(fmt.Sprintf("Bible/Book%d", i))
		bf.Write(bookData)
	}

	zw.Close()

	if err := os.WriteFile(outputPath, buf.Bytes(), 0600); err != nil {
		return "", fmt.Errorf("failed to write GoBible: %w", err)
	}

	return outputPath, nil
}

func createCollectionsFile(corpus *ir.Corpus) []byte {
	var buf bytes.Buffer

	binary.Write(&buf, binary.BigEndian, uint16(1))
	binary.Write(&buf, binary.BigEndian, uint16(len(corpus.Documents)))

	for _, doc := range corpus.Documents {
		name := []byte(doc.Title)
		binary.Write(&buf, binary.BigEndian, uint8(len(name)))
		buf.Write(name)
		binary.Write(&buf, binary.BigEndian, uint16(len(doc.ContentBlocks)))
	}

	return buf.Bytes()
}

func createBookDataFile(doc *ir.Document) []byte {
	var buf bytes.Buffer

	for _, cb := range doc.ContentBlocks {
		text := []byte(cb.Text)
		binary.Write(&buf, binary.BigEndian, uint16(len(text)))
		buf.Write(text)
	}

	return buf.Bytes()
}
