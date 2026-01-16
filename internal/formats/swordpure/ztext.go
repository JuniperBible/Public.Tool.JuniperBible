// ztext.go implements zText format parsing for SWORD modules.
// zText is a compressed Bible text format using zlib compression.
//
// File structure:
// - .bzs - Block section index (12 bytes per entry: offset[4], size[4], ucsize[4])
// - .bzv - Verse index (10 bytes per entry: block[4], offset[4], size[2])
// - .bzz - Compressed text data (zlib compressed blocks)
package swordpure

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Index entry sizes for SWORD binary file formats.
const (
	// BlockIndexEntrySize is the size of each entry in .bzs block index files.
	// Format: offset[4 bytes] + compressed_size[4 bytes] + uncompressed_size[4 bytes]
	BlockIndexEntrySize = 12

	// VerseIndexEntrySize is the size of each entry in .bzv verse index files.
	// Format: block_num[4 bytes] + offset[4 bytes] + size[2 bytes]
	VerseIndexEntrySize = 10
)

// ZTextModule represents a parsed zText SWORD module.
type ZTextModule struct {
	conf     *ConfFile
	dataPath string
	otBlocks []BlockEntry
	ntBlocks []BlockEntry
	otVerses []VerseEntry
	ntVerses []VerseEntry
}

// BlockEntry represents an entry in the .bzs block index.
type BlockEntry struct {
	Offset         uint32 // Offset in .bzz file
	CompressedSize uint32 // Size of compressed data
	UncompSize     uint32 // Size after decompression
}

// VerseEntry represents an entry in the .bzv verse index.
type VerseEntry struct {
	BlockNum uint32 // Which block contains this verse
	Offset   uint32 // Offset within the decompressed block
	Size     uint16 // Size of verse text
}

// OpenZTextModule opens a zText module for reading.
func OpenZTextModule(conf *ConfFile, swordPath string) (*ZTextModule, error) {
	// Construct the full data path
	dataPath := conf.DataPath
	if !filepath.IsAbs(dataPath) {
		dataPath = filepath.Join(swordPath, dataPath)
	}

	// Clean the path
	dataPath = filepath.Clean(dataPath)

	mod := &ZTextModule{
		conf:     conf,
		dataPath: dataPath,
	}

	// Load OT index files if they exist
	otBzsPath := filepath.Join(dataPath, "ot.bzs")
	otBzvPath := filepath.Join(dataPath, "ot.bzv")
	if _, err := os.Stat(otBzsPath); err == nil {
		mod.otBlocks, err = readBlockIndex(otBzsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read OT block index: %w", err)
		}
		mod.otVerses, err = readVerseIndex(otBzvPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read OT verse index: %w", err)
		}
	}

	// Load NT index files if they exist
	ntBzsPath := filepath.Join(dataPath, "nt.bzs")
	ntBzvPath := filepath.Join(dataPath, "nt.bzv")
	if _, err := os.Stat(ntBzsPath); err == nil {
		mod.ntBlocks, err = readBlockIndex(ntBzsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read NT block index: %w", err)
		}
		mod.ntVerses, err = readVerseIndex(ntBzvPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read NT verse index: %w", err)
		}
	}

	return mod, nil
}

// readBlockIndex reads a .bzs block index file.
func readBlockIndex(path string) ([]BlockEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(data)%BlockIndexEntrySize != 0 {
		return nil, fmt.Errorf("invalid block index size: %d", len(data))
	}

	count := len(data) / BlockIndexEntrySize
	entries := make([]BlockEntry, count)

	for i := 0; i < count; i++ {
		offset := i * BlockIndexEntrySize
		entries[i] = BlockEntry{
			Offset:         binary.LittleEndian.Uint32(data[offset:]),
			CompressedSize: binary.LittleEndian.Uint32(data[offset+4:]),
			UncompSize:     binary.LittleEndian.Uint32(data[offset+8:]),
		}
	}

	return entries, nil
}

// readVerseIndex reads a .bzv verse index file.
func readVerseIndex(path string) ([]VerseEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(data)%VerseIndexEntrySize != 0 {
		return nil, fmt.Errorf("invalid verse index size: %d", len(data))
	}

	count := len(data) / VerseIndexEntrySize
	entries := make([]VerseEntry, count)

	for i := 0; i < count; i++ {
		offset := i * VerseIndexEntrySize
		entries[i] = VerseEntry{
			BlockNum: binary.LittleEndian.Uint32(data[offset:]),
			Offset:   binary.LittleEndian.Uint32(data[offset+4:]),
			Size:     binary.LittleEndian.Uint16(data[offset+8:]),
		}
	}

	return entries, nil
}

// GetVerseText retrieves the text for a specific verse.
func (m *ZTextModule) GetVerseText(ref *Ref) (string, error) {
	// Determine which testament using book OSIS ID
	isNT := ntBookSet[ref.Book]

	var blocks []BlockEntry
	var verses []VerseEntry
	var bzzPath string

	if isNT {
		blocks = m.ntBlocks
		verses = m.ntVerses
		bzzPath = filepath.Join(m.dataPath, "nt.bzz")
	} else {
		blocks = m.otBlocks
		verses = m.otVerses
		bzzPath = filepath.Join(m.dataPath, "ot.bzz")
	}

	if len(blocks) == 0 || len(verses) == 0 {
		return "", fmt.Errorf("no data for %s testament", map[bool]string{true: "NT", false: "OT"}[isNT])
	}

	// Calculate verse index
	verseIdx, err := m.calculateVerseIndex(ref, isNT)
	if err != nil {
		return "", err
	}

	if verseIdx < 0 || verseIdx >= len(verses) {
		return "", fmt.Errorf("verse index out of range: %d", verseIdx)
	}

	verse := verses[verseIdx]
	if verse.Size == 0 {
		return "", nil // Empty verse
	}

	if int(verse.BlockNum) >= len(blocks) {
		return "", fmt.Errorf("block number out of range: %d", verse.BlockNum)
	}

	block := blocks[verse.BlockNum]

	// Read and decompress the block
	blockData, err := m.readBlock(bzzPath, block)
	if err != nil {
		return "", fmt.Errorf("failed to read block: %w", err)
	}

	// Extract verse text
	if int(verse.Offset)+int(verse.Size) > len(blockData) {
		return "", fmt.Errorf("verse data exceeds block size")
	}

	text := string(blockData[verse.Offset : verse.Offset+uint32(verse.Size)])

	// Clean up the text (remove null terminators, etc.)
	text = strings.TrimRight(text, "\x00")

	return text, nil
}

// readBlock reads and decompresses a block from the .bzz file.
func (m *ZTextModule) readBlock(bzzPath string, block BlockEntry) ([]byte, error) {
	f, err := os.Open(bzzPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Seek to block offset
	if _, err := f.Seek(int64(block.Offset), io.SeekStart); err != nil {
		return nil, err
	}

	// Read compressed data
	compressed := make([]byte, block.CompressedSize)
	if _, err := io.ReadFull(f, compressed); err != nil {
		return nil, err
	}

	// Decompress using zlib
	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("zlib init failed: %w", err)
	}
	defer reader.Close()

	decompressed := make([]byte, block.UncompSize)
	if _, err := io.ReadFull(reader, decompressed); err != nil {
		return nil, fmt.Errorf("decompression failed: %w", err)
	}

	return decompressed, nil
}

// calculateVerseIndex calculates the index into the verse array for a reference.
// This uses the versification system from the module's conf file.
func (m *ZTextModule) calculateVerseIndex(ref *Ref, isNT bool) (int, error) {
	vers, err := VersificationFromConf(m.conf)
	if err != nil {
		return -1, fmt.Errorf("failed to get versification: %w", err)
	}

	return vers.CalculateIndex(ref, isNT)
}

// HasOT returns true if the module has Old Testament data.
func (m *ZTextModule) HasOT() bool {
	return len(m.otBlocks) > 0
}

// HasNT returns true if the module has New Testament data.
func (m *ZTextModule) HasNT() bool {
	return len(m.ntBlocks) > 0
}

// GetModuleInfo returns information about the module.
func (m *ZTextModule) GetModuleInfo() ModuleInfo {
	return ModuleInfo{
		Name:        m.conf.ModuleName,
		Description: m.conf.Description,
		Type:        m.conf.ModuleType(),
		Language:    m.conf.Lang,
		Version:     m.conf.Version,
		Encoding:    m.conf.Encoding,
		DataPath:    m.dataPath,
		Compressed:  true,
		Encrypted:   m.conf.IsEncrypted(),
	}
}
