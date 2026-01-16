// zcom.go implements zCom format parsing for SWORD commentary modules.
// zCom uses the same file structure as zText (.bzs, .bzv, .bzz) but stores
// commentary entries instead of Bible text. Each entry can cover a verse,
// range of verses, or entire chapters.
//
// File structure:
// - .bzs - Block section index (12 bytes per entry: offset[4], size[4], ucsize[4])
// - .bzv - Verse index (10 bytes per entry: block[4], offset[4], size[2])
// - .bzz - Compressed text data (zlib compressed blocks)
//
// Index layout (SWORD header scheme):
// - [0] = empty placeholder
// - [1] = module header
// - [2] = book intro
// - [3] = chapter heading
// - [4+] = verse entries
package swordpure

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// readBlock reads and decompresses a block from a .bzz file.
func readBlock(bzzPath string, block BlockEntry) ([]byte, error) {
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

// CommentaryEntry represents a single commentary entry.
type CommentaryEntry struct {
	Reference Ref    // The verse reference this entry covers
	Text      string // The commentary text
	Source    string // The source module name
}

// ChapterEntries holds all commentary entries for a chapter.
type ChapterEntries struct {
	Book    string
	Chapter int
	Entries []*CommentaryEntry
}

// BookEntries holds all commentary entries for a book.
type BookEntries struct {
	Book     string
	Chapters []*ChapterEntries
}

// ZComParser handles parsing of zCom format SWORD commentary modules.
type ZComParser struct {
	module   *ConfFile
	basePath string
	dataPath string
	otBlocks []BlockEntry
	ntBlocks []BlockEntry
	otVerses []VerseEntry
	ntVerses []VerseEntry
	loaded   bool
}

// NewZComParser creates a new parser for a zCom commentary module.
func NewZComParser(conf *ConfFile, swordPath string) *ZComParser {
	return &ZComParser{
		module:   conf,
		basePath: swordPath,
	}
}

// Load reads the index files for the commentary module.
func (p *ZComParser) Load() error {
	// Construct the full data path
	dataPath := p.module.DataPath
	if !filepath.IsAbs(dataPath) {
		dataPath = filepath.Join(p.basePath, dataPath)
	}
	p.dataPath = filepath.Clean(dataPath)

	// Load OT index files if they exist
	otBzsPath := filepath.Join(p.dataPath, "ot.bzs")
	otBzvPath := filepath.Join(p.dataPath, "ot.bzv")
	if _, err := os.Stat(otBzsPath); err == nil {
		var err error
		p.otBlocks, err = readBlockIndex(otBzsPath)
		if err != nil {
			return fmt.Errorf("failed to read OT block index: %w", err)
		}
		p.otVerses, err = readVerseIndex(otBzvPath)
		if err != nil {
			return fmt.Errorf("failed to read OT verse index: %w", err)
		}
	}

	// Load NT index files if they exist
	ntBzsPath := filepath.Join(p.dataPath, "nt.bzs")
	ntBzvPath := filepath.Join(p.dataPath, "nt.bzv")
	if _, err := os.Stat(ntBzsPath); err == nil {
		var err error
		p.ntBlocks, err = readBlockIndex(ntBzsPath)
		if err != nil {
			return fmt.Errorf("failed to read NT block index: %w", err)
		}
		p.ntVerses, err = readVerseIndex(ntBzvPath)
		if err != nil {
			return fmt.Errorf("failed to read NT verse index: %w", err)
		}
	}

	p.loaded = true
	return nil
}

// GetEntry retrieves a commentary entry for a specific verse reference.
func (p *ZComParser) GetEntry(ref Ref) (*CommentaryEntry, error) {
	if !p.loaded {
		return nil, fmt.Errorf("module not loaded")
	}

	// Determine which testament using book OSIS ID
	isNT := ntBookSet[ref.Book]

	var blocks []BlockEntry
	var verses []VerseEntry
	var bzzPath string

	if isNT {
		blocks = p.ntBlocks
		verses = p.ntVerses
		bzzPath = filepath.Join(p.dataPath, "nt.bzz")
	} else {
		blocks = p.otBlocks
		verses = p.otVerses
		bzzPath = filepath.Join(p.dataPath, "ot.bzz")
	}

	if len(blocks) == 0 || len(verses) == 0 {
		testament := "OT"
		if isNT {
			testament = "NT"
		}
		return nil, fmt.Errorf("no data for %s testament", testament)
	}

	// Calculate verse index using versification
	vers, err := VersificationFromConf(p.module)
	if err != nil {
		return nil, fmt.Errorf("failed to get versification: %w", err)
	}

	verseIdx, err := vers.CalculateIndex(&ref, isNT)
	if err != nil {
		return nil, err
	}

	if verseIdx < 0 || verseIdx >= len(verses) {
		return nil, fmt.Errorf("verse index out of range: %d", verseIdx)
	}

	verse := verses[verseIdx]
	if verse.Size == 0 {
		return &CommentaryEntry{
			Reference: ref,
			Text:      "",
			Source:    p.module.ModuleName,
		}, nil
	}

	if int(verse.BlockNum) >= len(blocks) {
		return nil, fmt.Errorf("block number out of range: %d", verse.BlockNum)
	}

	block := blocks[verse.BlockNum]

	// Read and decompress the block
	blockData, err := readBlock(bzzPath, block)
	if err != nil {
		return nil, fmt.Errorf("failed to read block: %w", err)
	}

	// Extract entry text
	if int(verse.Offset)+int(verse.Size) > len(blockData) {
		return nil, fmt.Errorf("entry data exceeds block size")
	}

	text := string(blockData[verse.Offset : verse.Offset+uint32(verse.Size)])
	text = strings.TrimRight(text, "\x00")

	return &CommentaryEntry{
		Reference: ref,
		Text:      text,
		Source:    p.module.ModuleName,
	}, nil
}

// GetChapterEntries retrieves all commentary entries for a chapter.
func (p *ZComParser) GetChapterEntries(book string, chapter int) (*ChapterEntries, error) {
	if !p.loaded {
		return nil, fmt.Errorf("module not loaded")
	}

	vers, err := VersificationFromConf(p.module)
	if err != nil {
		return nil, fmt.Errorf("failed to get versification: %w", err)
	}

	bookIdx := BookIndex(book)
	if bookIdx < 0 {
		return nil, fmt.Errorf("unknown book: %s", book)
	}

	verseCount := vers.GetVerseCount(book, chapter)
	if verseCount <= 0 {
		return nil, fmt.Errorf("invalid chapter: %s %d", book, chapter)
	}

	result := &ChapterEntries{
		Book:    book,
		Chapter: chapter,
		Entries: make([]*CommentaryEntry, 0, verseCount),
	}

	for verse := 1; verse <= verseCount; verse++ {
		ref := Ref{Book: book, Chapter: chapter, Verse: verse}
		entry, err := p.GetEntry(ref)
		if err != nil {
			continue // Skip verses with errors
		}
		if entry.Text != "" { // Only include non-empty entries
			result.Entries = append(result.Entries, entry)
		}
	}

	return result, nil
}

// GetBookEntries retrieves all commentary entries for a book.
func (p *ZComParser) GetBookEntries(book string) (*BookEntries, error) {
	if !p.loaded {
		return nil, fmt.Errorf("module not loaded")
	}

	vers, err := VersificationFromConf(p.module)
	if err != nil {
		return nil, fmt.Errorf("failed to get versification: %w", err)
	}

	chapterCount := vers.GetChapterCount(book)
	if chapterCount <= 0 {
		return nil, fmt.Errorf("unknown book: %s", book)
	}

	result := &BookEntries{
		Book:     book,
		Chapters: make([]*ChapterEntries, 0, chapterCount),
	}

	for chapter := 1; chapter <= chapterCount; chapter++ {
		chapterEntries, err := p.GetChapterEntries(book, chapter)
		if err != nil {
			continue // Skip chapters with errors
		}
		if len(chapterEntries.Entries) > 0 {
			result.Chapters = append(result.Chapters, chapterEntries)
		}
	}

	return result, nil
}

// HasOT returns true if the module has Old Testament commentary data.
func (p *ZComParser) HasOT() bool {
	return len(p.otBlocks) > 0
}

// HasNT returns true if the module has New Testament commentary data.
func (p *ZComParser) HasNT() bool {
	return len(p.ntBlocks) > 0
}

// GetModuleInfo returns information about the commentary module.
func (p *ZComParser) GetModuleInfo() ModuleInfo {
	return ModuleInfo{
		Name:        p.module.ModuleName,
		Description: p.module.Description,
		Type:        "Commentary",
		Language:    p.module.Lang,
		Version:     p.module.Version,
		Encoding:    p.module.Encoding,
		DataPath:    p.dataPath,
		Compressed:  true,
		Encrypted:   p.module.IsEncrypted(),
	}
}
