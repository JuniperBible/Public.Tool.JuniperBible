//go:build sdk

// Plugin format-gobible handles GoBible J2ME format.
// GoBible uses JAR-based .gbk files containing binary verse data.
//
// IR Support:
// - extract-ir: Reads GoBible format to IR (L3)
// - emit-native: Converts IR to GoBible-compatible format (L3)
package main

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

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

func main() {
	if err := format.Run(&format.Config{
		Name:       "GoBible",
		Extensions: []string{".gbk"},
		Detect:     detectGoBible,
		Parse:      parseGoBible,
		Emit:       emitGoBible,
		Enumerate:  enumerateGoBible,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectGoBible(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".gbk" {
		return &ipc.DetectResult{Detected: true, Format: "GoBible", Reason: "GoBible file extension detected"}, nil
	}

	// Check if it's a JAR/ZIP with GoBible structure
	zr, err := zip.OpenReader(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: "not a GoBible file"}, nil
	}
	defer zr.Close()

	hasCollections := false
	hasBibleData := false
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if name == "collections" || name == "bible/collections" {
			hasCollections = true
		}
		if strings.HasPrefix(name, "bible/") || strings.Contains(name, "verse") {
			hasBibleData = true
		}
	}

	if hasCollections || hasBibleData {
		return &ipc.DetectResult{Detected: true, Format: "GoBible", Reason: "GoBible JAR structure detected"}, nil
	}

	return &ipc.DetectResult{Detected: false, Reason: "no GoBible structure found"}, nil
}

func enumerateGoBible(path string) (*ipc.EnumerateResult, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		// Single file
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

	// Try to extract content from JAR
	corpus.Documents = extractGoBibleContent(data, artifactID)

	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ir.Document{ir.NewDocument(artifactID, artifactID, 1)}
	}

	return corpus, nil
}

func extractGoBibleContent(data []byte, artifactID string) []*ir.Document {
	doc := ir.NewDocument(artifactID, artifactID, 1)

	// Try to open as ZIP
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return []*ir.Document{doc}
	}

	sequence := 0
	for _, f := range zr.File {
		// Look for text content files
		if strings.HasSuffix(f.Name, ".txt") || strings.Contains(f.Name, "verse") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			// Parse lines as verses
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if len(line) > 5 {
					sequence++
					hash := sha256.Sum256([]byte(line))
					doc.ContentBlocks = append(doc.ContentBlocks, &ir.ContentBlock{
						ID:       fmt.Sprintf("cb-%d", sequence),
						Sequence: sequence,
						Text:     line,
						Hash:     hex.EncodeToString(hash[:]),
					})
				}
			}
		}

		// Try to parse binary Bible data
		if strings.HasPrefix(f.Name, "Bible/") || f.Name == "Collections" {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			// Extract text from binary
			extracted := extractTextFromBinary(content)
			for _, text := range extracted {
				if len(text) > 5 {
					sequence++
					hash := sha256.Sum256([]byte(text))
					doc.ContentBlocks = append(doc.ContentBlocks, &ir.ContentBlock{
						ID:       fmt.Sprintf("cb-%d", sequence),
						Sequence: sequence,
						Text:     text,
						Hash:     hex.EncodeToString(hash[:]),
					})
				}
			}
		}
	}

	return []*ir.Document{doc}
}

func extractTextFromBinary(data []byte) []string {
	var texts []string
	var current strings.Builder

	// Simple heuristic: look for runs of printable ASCII/UTF-8
	for i := 0; i < len(data); i++ {
		b := data[i]
		if b >= 32 && b < 127 {
			current.WriteByte(b)
		} else if b >= 0xC0 && i+1 < len(data) {
			// UTF-8 multibyte - try to decode
			if b < 0xE0 && i+1 < len(data) {
				current.WriteByte(b)
				i++
				current.WriteByte(data[i])
			}
		} else {
			if current.Len() > 10 {
				texts = append(texts, current.String())
			}
			current.Reset()
		}
	}

	if current.Len() > 10 {
		texts = append(texts, current.String())
	}

	return texts
}

func emitGoBible(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".gbk")

	// Check for raw GoBible for round-trip
	if raw, ok := corpus.Attributes["_gobible_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
				return "", fmt.Errorf("failed to write GoBible: %w", err)
			}
			return outputPath, nil
		}
	}

	// Generate GoBible-compatible JAR from IR
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Create manifest
	manifest := "Manifest-Version: 1.0\nMIDlet-Name: " + corpus.Title + "\n"
	mf, _ := zw.Create("META-INF/MANIFEST.MF")
	mf.Write([]byte(manifest))

	// Create Collections file
	collectionsData := createCollectionsFile(corpus)
	cf, _ := zw.Create("Bible/Collections")
	cf.Write(collectionsData)

	// Create verse data files
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

	// GoBible Collections format (simplified)
	binary.Write(&buf, binary.BigEndian, uint16(1)) // version
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
