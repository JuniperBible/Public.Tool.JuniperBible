// ztext_writer.go implements zText format writing for SWORD modules.
// This enables round-trip conversion: SWORD → IR → SWORD.
//
// zText format:
// - .bzs - Block section index (12 bytes per entry: offset[4], size[4], ucsize[4])
// - .bzv - Verse index (10 bytes per entry: block[4], offset[4], size[2])
// - .bzz - Compressed text data (zlib compressed blocks)
package swordpure

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

// ZTextWriter writes zText format SWORD modules.
type ZTextWriter struct {
	dataPath string
	vers     *Versification

	// Block accumulation
	currentBlock  bytes.Buffer
	blockEntries  []BlockEntry
	verseEntries  []VerseEntry
	compressedBuf bytes.Buffer

	// Current block state
	currentBlockNum  uint32
	currentBlockSize uint32
}

// NewZTextWriter creates a new zText writer for the given data path.
func NewZTextWriter(dataPath string, vers *Versification) *ZTextWriter {
	return &ZTextWriter{
		dataPath: dataPath,
		vers:     vers,
	}
}

// WriteModule writes a complete zText module from IR corpus.
// Returns the number of verses written.
func (w *ZTextWriter) WriteModule(corpus *IRCorpus) (int, error) {
	// Create data directory
	if err := os.MkdirAll(w.dataPath, 0700); err != nil {
		return 0, fmt.Errorf("failed to create data path: %w", err)
	}

	// Build verse map from corpus for quick lookup
	verseMap := make(map[string]string) // ref -> text (with markup)
	for _, doc := range corpus.Documents {
		for _, block := range doc.ContentBlocks {
			// Use RawMarkup if available, otherwise Text
			text := block.RawMarkup
			if text == "" {
				text = block.Text
			}
			verseMap[block.ID] = text
		}
	}

	// Write OT and NT separately
	otVerses, err := w.writeTestament(false, verseMap)
	if err != nil {
		return 0, fmt.Errorf("failed to write OT: %w", err)
	}

	ntVerses, err := w.writeTestament(true, verseMap)
	if err != nil {
		return 0, fmt.Errorf("failed to write NT: %w", err)
	}

	return otVerses + ntVerses, nil
}

// writeTestament writes either OT or NT data files.
func (w *ZTextWriter) writeTestament(isNT bool, verseMap map[string]string) (int, error) {
	// Reset state
	w.currentBlock.Reset()
	w.blockEntries = nil
	w.verseEntries = nil
	w.compressedBuf.Reset()
	w.currentBlockNum = 0
	w.currentBlockSize = 0

	prefix := "ot"
	startBook := 0
	endBook := 39
	if isNT {
		prefix = "nt"
		startBook = 39
		endBook = len(w.vers.Books)
	}

	versesWritten := 0
	verseIndex := 0

	// SWORD index scheme: [0]=empty, [1]=module header, then per-book/chapter/verse
	// We need to maintain the same indexing structure

	// [0] = empty slot
	w.addVerseEntry(0, 0, 0)
	verseIndex++

	// [1] = module header (empty)
	w.addVerseEntry(0, 0, 0)
	verseIndex++

	// Process each book
	for bookIdx := startBook; bookIdx < endBook; bookIdx++ {
		book := w.vers.Books[bookIdx]

		// Book intro (empty)
		w.addVerseEntry(w.currentBlockNum, w.currentBlockSize, 0)
		verseIndex++

		// Process each chapter
		for chIdx, verseCount := range book.Chapters {
			chapter := chIdx + 1

			// Chapter heading (empty)
			w.addVerseEntry(w.currentBlockNum, w.currentBlockSize, 0)
			verseIndex++

			// Process each verse
			for verse := 1; verse <= verseCount; verse++ {
				ref := fmt.Sprintf("%s.%d.%d", book.OSIS, chapter, verse)
				text := verseMap[ref]

				if text != "" {
					// Add verse to current block
					textBytes := []byte(text)
					offset := w.currentBlockSize
					size := uint16(len(textBytes))

					w.currentBlock.Write(textBytes)
					w.currentBlockSize += uint32(size)
					w.addVerseEntry(w.currentBlockNum, offset, size)
					versesWritten++
				} else {
					// Empty verse
					w.addVerseEntry(w.currentBlockNum, w.currentBlockSize, 0)
				}
				verseIndex++

				// Flush block if it gets too large (4KB threshold)
				if w.currentBlock.Len() > 4096 {
					if err := w.flushBlock(); err != nil {
						return 0, err
					}
				}
			}
		}

		// Flush block at end of each book
		if w.currentBlock.Len() > 0 {
			if err := w.flushBlock(); err != nil {
				return 0, err
			}
		}
	}

	// Flush any remaining data
	if w.currentBlock.Len() > 0 {
		if err := w.flushBlock(); err != nil {
			return 0, err
		}
	}

	// Write files
	if err := w.writeFiles(prefix); err != nil {
		return 0, err
	}

	return versesWritten, nil
}

// addVerseEntry adds a verse entry to the index.
func (w *ZTextWriter) addVerseEntry(blockNum, offset uint32, size uint16) {
	w.verseEntries = append(w.verseEntries, VerseEntry{
		BlockNum: blockNum,
		Offset:   offset,
		Size:     size,
	})
}

// flushBlock compresses the current block and adds it to the compressed buffer.
func (w *ZTextWriter) flushBlock() error {
	if w.currentBlock.Len() == 0 {
		return nil
	}

	uncompSize := uint32(w.currentBlock.Len())
	offset := uint32(w.compressedBuf.Len())

	// Compress with zlib
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(w.currentBlock.Bytes()); err != nil {
		return fmt.Errorf("zlib compression failed: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("zlib close failed: %w", err)
	}

	compSize := uint32(compressed.Len())

	// Add to compressed buffer
	w.compressedBuf.Write(compressed.Bytes())

	// Add block entry
	w.blockEntries = append(w.blockEntries, BlockEntry{
		Offset:         offset,
		CompressedSize: compSize,
		UncompSize:     uncompSize,
	})

	// Reset for next block
	w.currentBlock.Reset()
	w.currentBlockNum++
	w.currentBlockSize = 0

	return nil
}

// writeFiles writes the .bzs, .bzv, and .bzz files.
func (w *ZTextWriter) writeFiles(prefix string) error {
	// Write .bzz (compressed data)
	bzzPath := filepath.Join(w.dataPath, prefix+".bzz")
	if err := os.WriteFile(bzzPath, w.compressedBuf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write bzz: %w", err)
	}

	// Write .bzs (block index)
	bzsPath := filepath.Join(w.dataPath, prefix+".bzs")
	bzsData := make([]byte, len(w.blockEntries)*12)
	for i, entry := range w.blockEntries {
		offset := i * 12
		binary.LittleEndian.PutUint32(bzsData[offset:], entry.Offset)
		binary.LittleEndian.PutUint32(bzsData[offset+4:], entry.CompressedSize)
		binary.LittleEndian.PutUint32(bzsData[offset+8:], entry.UncompSize)
	}
	if err := os.WriteFile(bzsPath, bzsData, 0600); err != nil {
		return fmt.Errorf("failed to write bzs: %w", err)
	}

	// Write .bzv (verse index)
	bzvPath := filepath.Join(w.dataPath, prefix+".bzv")
	bzvData := make([]byte, len(w.verseEntries)*10)
	for i, entry := range w.verseEntries {
		offset := i * 10
		binary.LittleEndian.PutUint32(bzvData[offset:], entry.BlockNum)
		binary.LittleEndian.PutUint32(bzvData[offset+4:], entry.Offset)
		binary.LittleEndian.PutUint16(bzvData[offset+8:], entry.Size)
	}
	if err := os.WriteFile(bzvPath, bzvData, 0600); err != nil {
		return fmt.Errorf("failed to write bzv: %w", err)
	}

	return nil
}

// EmitZText writes a complete SWORD module from IR corpus.
// Creates mods.d/*.conf and modules/texts/ztext/*/ structure.
func EmitZText(corpus *IRCorpus, outputDir string) (*EmitResult, error) {
	result := &EmitResult{
		ModuleID: corpus.ID,
	}

	// Create directory structure
	modsDir := filepath.Join(outputDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create mods.d: %w", err)
	}

	dataPath := filepath.Join(outputDir, "modules", "texts", "ztext", stringToLower(corpus.ID))
	if err := os.MkdirAll(dataPath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data path: %w", err)
	}

	// Determine versification
	versID := VersificationID(corpus.Versification)
	if versID == "" {
		versID = VersKJV
	}
	vers, err := NewVersification(versID)
	if err != nil {
		return nil, fmt.Errorf("failed to get versification: %w", err)
	}

	// Write zText data
	writer := NewZTextWriter(dataPath, vers)
	versesWritten, err := writer.WriteModule(corpus)
	if err != nil {
		return nil, fmt.Errorf("failed to write zText: %w", err)
	}
	result.VersesWritten = versesWritten

	// Generate and write .conf file
	confContent := generateConfFromIR(corpus)
	// Update DataPath to match actual location
	confContent = updateConfDataPath(confContent, corpus.ID)
	confPath := filepath.Join(modsDir, stringToLower(corpus.ID)+".conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		return nil, fmt.Errorf("failed to write conf: %w", err)
	}
	result.ConfPath = confPath
	result.DataPath = dataPath

	return result, nil
}

// EmitResult contains the result of emitting a SWORD module.
type EmitResult struct {
	ModuleID      string
	ConfPath      string
	DataPath      string
	VersesWritten int
}

// updateConfDataPath updates the DataPath in conf content to match the module ID.
func updateConfDataPath(conf, moduleID string) string {
	// Find and replace DataPath line
	lines := splitLines(conf)
	var result []string
	for _, line := range lines {
		if len(line) > 9 && line[:9] == "DataPath=" {
			line = fmt.Sprintf("DataPath=./modules/texts/ztext/%s/", stringToLower(moduleID))
		}
		result = append(result, line)
	}
	return joinLines(result)
}

// splitLines splits a string into lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// joinLines joins lines with newlines.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	result := lines[0]
	for i := 1; i < len(lines); i++ {
		result += "\n" + lines[i]
	}
	return result
}

// stringToLower converts a string to lowercase (avoiding strings import in this file).
func stringToLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
