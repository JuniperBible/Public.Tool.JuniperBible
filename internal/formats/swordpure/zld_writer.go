// zld_writer.go implements zLD format writing for SWORD lexicon/dictionary modules.
// This enables round-trip conversion: SWORD → IR → SWORD for lexicons.
//
// zLD format:
// - .idx - Key index (4-byte big-endian offset + null-terminated key string)
// - .dat - Key data (null-terminated keys)
// - .zdx - Compressed data index (8 bytes per entry: block[4] + offset[4])
// - .zdt - Compressed text data (4-byte size + zlib compressed)
package swordpure

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ZLDWriter writes zLD format SWORD lexicon/dictionary modules.
type ZLDWriter struct {
	dataPath string

	// Entries to write
	entries []zldWriteEntry

	// Compression state
	blockSize     int
	currentBlock  bytes.Buffer
	compressedBuf bytes.Buffer
	blockIndex    []zldBlockIndex
}

type zldWriteEntry struct {
	Key  string
	Text string
}

type zldBlockIndex struct {
	BlockNum uint32
	Offset   uint32
}

// NewZLDWriter creates a new zLD writer for the given data path.
func NewZLDWriter(dataPath string) *ZLDWriter {
	return &ZLDWriter{
		dataPath:  dataPath,
		blockSize: 4096, // 4KB blocks
	}
}

// AddEntry adds an entry to be written.
func (w *ZLDWriter) AddEntry(key, text string) {
	w.entries = append(w.entries, zldWriteEntry{Key: key, Text: text})
}

// WriteModule writes the complete zLD module.
// Returns the number of entries written.
func (w *ZLDWriter) WriteModule() (int, error) {
	// Create data directory
	if err := os.MkdirAll(w.dataPath, 0700); err != nil {
		return 0, fmt.Errorf("failed to create data path: %w", err)
	}

	// Sort entries by key for proper index ordering
	sort.Slice(w.entries, func(i, j int) bool {
		return w.entries[i].Key < w.entries[j].Key
	})

	// Process entries into blocks
	entryIndices := make([]zldBlockIndex, len(w.entries))
	currentBlockNum := uint32(0)

	for i, entry := range w.entries {
		// Check if we need to start a new block
		if w.currentBlock.Len() > 0 && w.currentBlock.Len()+len(entry.Text)+1 > w.blockSize {
			if err := w.flushBlock(); err != nil {
				return 0, err
			}
			currentBlockNum++
		}

		// Record index for this entry
		entryIndices[i] = zldBlockIndex{
			BlockNum: currentBlockNum,
			Offset:   uint32(w.currentBlock.Len()),
		}

		// Add text to current block (null-terminated)
		w.currentBlock.WriteString(entry.Text)
		w.currentBlock.WriteByte(0)
	}

	// Flush remaining block
	if w.currentBlock.Len() > 0 {
		if err := w.flushBlock(); err != nil {
			return 0, err
		}
	}

	// Write all files
	if err := w.writeFiles(entryIndices); err != nil {
		return 0, err
	}

	return len(w.entries), nil
}

// flushBlock compresses the current block and adds it to the compressed buffer.
func (w *ZLDWriter) flushBlock() error {
	if w.currentBlock.Len() == 0 {
		return nil
	}

	// Record block start offset
	blockOffset := uint32(w.compressedBuf.Len())

	// Compress with zlib
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(w.currentBlock.Bytes()); err != nil {
		return fmt.Errorf("zlib compression failed: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("zlib close failed: %w", err)
	}

	// Write size header + compressed data
	compSize := uint32(compressed.Len())
	sizeBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(sizeBuf, compSize)
	w.compressedBuf.Write(sizeBuf)
	w.compressedBuf.Write(compressed.Bytes())

	// Track block for index
	w.blockIndex = append(w.blockIndex, zldBlockIndex{
		BlockNum: uint32(len(w.blockIndex)),
		Offset:   blockOffset,
	})

	// Reset current block
	w.currentBlock.Reset()

	return nil
}

// writeFiles writes the .idx, .dat, .zdx, and .zdt files.
func (w *ZLDWriter) writeFiles(entryIndices []zldBlockIndex) error {
	// Write .zdt (compressed data)
	zdtPath := filepath.Join(w.dataPath, "dict.zdt")
	if err := os.WriteFile(zdtPath, w.compressedBuf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write zdt: %w", err)
	}

	// Write .zdx (compressed index - maps entry index to block/offset)
	zdxPath := filepath.Join(w.dataPath, "dict.zdx")
	zdxData := make([]byte, len(entryIndices)*8)
	for i, idx := range entryIndices {
		offset := i * 8
		binary.LittleEndian.PutUint32(zdxData[offset:], idx.BlockNum)
		binary.LittleEndian.PutUint32(zdxData[offset+4:], idx.Offset)
	}
	if err := os.WriteFile(zdxPath, zdxData, 0600); err != nil {
		return fmt.Errorf("failed to write zdx: %w", err)
	}

	// Write .idx (key index - big-endian offset + null-terminated key)
	var idxBuf bytes.Buffer
	var datBuf bytes.Buffer

	for i, entry := range w.entries {
		// .idx entry: 4-byte big-endian offset + null-terminated key
		offset := uint32(i) // Entry index (also offset into .dat for compatibility)
		offsetBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(offsetBytes, offset)
		idxBuf.Write(offsetBytes)
		idxBuf.WriteString(entry.Key)
		idxBuf.WriteByte(0)

		// .dat entry: null-terminated key (for compatibility)
		datBuf.WriteString(entry.Key)
		datBuf.WriteByte(0)
	}

	idxPath := filepath.Join(w.dataPath, "dict.idx")
	if err := os.WriteFile(idxPath, idxBuf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write idx: %w", err)
	}

	datPath := filepath.Join(w.dataPath, "dict.dat")
	if err := os.WriteFile(datPath, datBuf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write dat: %w", err)
	}

	return nil
}

// EmitZLD writes a complete SWORD lexicon module from IR corpus.
// Creates mods.d/*.conf and modules/lexdict/zld/*/ structure.
func EmitZLD(corpus *IRCorpus, outputDir string) (*EmitResult, error) {
	result := &EmitResult{
		ModuleID: corpus.ID,
	}

	// Create directory structure
	modsDir := filepath.Join(outputDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create mods.d: %w", err)
	}

	dataPath := filepath.Join(outputDir, "modules", "lexdict", "zld", stringToLower(corpus.ID))
	if err := os.MkdirAll(dataPath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data path: %w", err)
	}

	// Write zLD data
	writer := NewZLDWriter(dataPath)

	// Add entries from corpus
	for _, doc := range corpus.Documents {
		for _, block := range doc.ContentBlocks {
			// Use block ID as key, text as definition
			text := block.RawMarkup
			if text == "" {
				text = block.Text
			}
			writer.AddEntry(block.ID, text)
		}
	}

	entriesWritten, err := writer.WriteModule()
	if err != nil {
		return nil, fmt.Errorf("failed to write zLD: %w", err)
	}
	result.VersesWritten = entriesWritten

	// Generate and write .conf file
	confContent := generateLexiconConf(corpus)
	confPath := filepath.Join(modsDir, stringToLower(corpus.ID)+".conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		return nil, fmt.Errorf("failed to write conf: %w", err)
	}
	result.ConfPath = confPath
	result.DataPath = dataPath

	return result, nil
}

// generateLexiconConf generates a SWORD .conf file for a lexicon module.
func generateLexiconConf(corpus *IRCorpus) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("[%s]\n", corpus.ID))
	buf.WriteString(fmt.Sprintf("Description=%s\n", corpus.Title))
	buf.WriteString(fmt.Sprintf("Lang=%s\n", corpus.Language))
	buf.WriteString("ModDrv=zLD\n")
	buf.WriteString("Encoding=UTF-8\n")
	buf.WriteString(fmt.Sprintf("DataPath=./modules/lexdict/zld/%s/dict\n", stringToLower(corpus.ID)))

	return buf.String()
}
