// zcom_writer.go implements zCom format writing for SWORD commentary modules.
// This enables round-trip conversion: SWORD → IR → SWORD for commentaries.
//
// zCom format uses the same file structure as zText:
// - .bzs - Block section index (12 bytes per entry: offset[4], size[4], ucsize[4])
// - .bzv - Entry index (10 bytes per entry: block[4], offset[4], size[2])
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

// ZComWriter writes zCom format SWORD commentary modules.
type ZComWriter struct {
	dataPath string
	vers     *Versification

	// Block accumulation
	currentBlock  bytes.Buffer
	blockEntries  []BlockEntry
	entryEntries  []VerseEntry // Reuse VerseEntry for commentary entries
	compressedBuf bytes.Buffer

	// Current block state
	currentBlockNum  uint32
	currentBlockSize uint32
}

// NewZComWriter creates a new zCom writer for the given data path.
func NewZComWriter(dataPath string, vers *Versification) *ZComWriter {
	return &ZComWriter{
		dataPath: dataPath,
		vers:     vers,
	}
}

// WriteModule writes a complete zCom module from IR corpus.
// Returns the number of entries written.
func (w *ZComWriter) WriteModule(corpus *IRCorpus) (int, error) {
	// Create data directory
	if err := os.MkdirAll(w.dataPath, 0700); err != nil {
		return 0, fmt.Errorf("failed to create data path: %w", err)
	}

	// Build entry map from corpus for quick lookup
	entryMap := make(map[string]string) // ref -> text (with markup)
	for _, doc := range corpus.Documents {
		for _, block := range doc.ContentBlocks {
			// Use RawMarkup if available, otherwise Text
			text := block.RawMarkup
			if text == "" {
				text = block.Text
			}
			entryMap[block.ID] = text
		}
	}

	// Write OT and NT separately
	otEntries, err := w.writeTestament(false, entryMap)
	if err != nil {
		return 0, fmt.Errorf("failed to write OT: %w", err)
	}

	ntEntries, err := w.writeTestament(true, entryMap)
	if err != nil {
		return 0, fmt.Errorf("failed to write NT: %w", err)
	}

	return otEntries + ntEntries, nil
}

// writeTestament writes either OT or NT data files.
func (w *ZComWriter) writeTestament(isNT bool, entryMap map[string]string) (int, error) {
	// Reset state
	w.currentBlock.Reset()
	w.blockEntries = nil
	w.entryEntries = nil
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

	entriesWritten := 0
	entryIndex := 0

	// SWORD index scheme: [0]=empty, [1]=module header, then per-book/chapter/verse
	// [0] = empty slot
	w.addEntryEntry(0, 0, 0)
	entryIndex++

	// [1] = module header (empty)
	w.addEntryEntry(0, 0, 0)
	entryIndex++

	// Process each book
	for bookIdx := startBook; bookIdx < endBook; bookIdx++ {
		book := w.vers.Books[bookIdx]

		// Book intro (empty)
		w.addEntryEntry(w.currentBlockNum, w.currentBlockSize, 0)
		entryIndex++

		// Process each chapter
		for chIdx, verseCount := range book.Chapters {
			chapter := chIdx + 1

			// Chapter heading (empty)
			w.addEntryEntry(w.currentBlockNum, w.currentBlockSize, 0)
			entryIndex++

			// Process each verse
			for verse := 1; verse <= verseCount; verse++ {
				ref := fmt.Sprintf("%s.%d.%d", book.OSIS, chapter, verse)
				text := entryMap[ref]

				if text != "" {
					// Add entry to current block
					textBytes := []byte(text)
					offset := w.currentBlockSize
					size := uint16(len(textBytes))

					w.currentBlock.Write(textBytes)
					w.currentBlockSize += uint32(size)
					w.addEntryEntry(w.currentBlockNum, offset, size)
					entriesWritten++
				} else {
					// Empty entry
					w.addEntryEntry(w.currentBlockNum, w.currentBlockSize, 0)
				}
				entryIndex++

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

	return entriesWritten, nil
}

// addEntryEntry adds an entry to the index.
func (w *ZComWriter) addEntryEntry(blockNum, offset uint32, size uint16) {
	w.entryEntries = append(w.entryEntries, VerseEntry{
		BlockNum: blockNum,
		Offset:   offset,
		Size:     size,
	})
}

// flushBlock compresses the current block and adds it to the compressed buffer.
func (w *ZComWriter) flushBlock() error {
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
func (w *ZComWriter) writeFiles(prefix string) error {
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

	// Write .bzv (entry index)
	bzvPath := filepath.Join(w.dataPath, prefix+".bzv")
	bzvData := make([]byte, len(w.entryEntries)*10)
	for i, entry := range w.entryEntries {
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

// EmitZCom writes a complete SWORD commentary module from IR corpus.
// Creates mods.d/*.conf and modules/comments/zcom/*/ structure.
func EmitZCom(corpus *IRCorpus, outputDir string) (*EmitResult, error) {
	result := &EmitResult{
		ModuleID: corpus.ID,
	}

	// Create directory structure
	modsDir := filepath.Join(outputDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create mods.d: %w", err)
	}

	dataPath := filepath.Join(outputDir, "modules", "comments", "zcom", stringToLower(corpus.ID))
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

	// Write zCom data
	writer := NewZComWriter(dataPath, vers)
	entriesWritten, err := writer.WriteModule(corpus)
	if err != nil {
		return nil, fmt.Errorf("failed to write zCom: %w", err)
	}
	result.VersesWritten = entriesWritten

	// Generate and write .conf file
	confContent := generateCommentaryConf(corpus)
	confPath := filepath.Join(modsDir, stringToLower(corpus.ID)+".conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		return nil, fmt.Errorf("failed to write conf: %w", err)
	}
	result.ConfPath = confPath
	result.DataPath = dataPath

	return result, nil
}

// generateCommentaryConf generates a SWORD .conf file for a commentary module.
func generateCommentaryConf(corpus *IRCorpus) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("[%s]\n", corpus.ID))
	buf.WriteString(fmt.Sprintf("Description=%s\n", corpus.Title))
	buf.WriteString(fmt.Sprintf("Lang=%s\n", corpus.Language))
	buf.WriteString("ModDrv=zCom\n")
	buf.WriteString("Encoding=UTF-8\n")
	buf.WriteString(fmt.Sprintf("DataPath=./modules/comments/zcom/%s/\n", stringToLower(corpus.ID)))

	if corpus.Versification != "" {
		buf.WriteString(fmt.Sprintf("Versification=%s\n", corpus.Versification))
	}

	return buf.String()
}
