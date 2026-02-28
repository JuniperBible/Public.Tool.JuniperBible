// Package sword provides parsers for SWORD Bible module format.
package sword

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ZComParser parses zCom format SWORD modules (compressed commentaries).
//
// The zCom format is similar to zText but contains commentary entries
// keyed to Bible verses. It uses the same file structure:
//   - .bzs (book index): Offsets into the verse index for each book
//   - .bzv (verse index): Entries per verse (blockNum, verseStart, verseLen)
//   - .bzz (compressed data): zlib-compressed commentary text blocks
type ZComParser struct {
	module   *Module
	dataPath string

	// Index data (loaded on demand)
	bookIndex  []uint32
	verseIndex []ZComVerseEntry
	loaded     bool

	// Testament tracking for NT-only or OT-only modules
	hasOT bool
	hasNT bool
}

// ZComVerseEntry represents a verse index entry for commentary format.
// This is the same format as zText.
type ZComVerseEntry struct {
	BlockNum   uint32 // Block number (index into .bzs)
	VerseStart uint32 // Offset within uncompressed block
	VerseLen   uint16 // Entry length in bytes
}

// CommentaryEntry represents a single commentary entry for a verse.
type CommentaryEntry struct {
	Reference Reference
	Text      string
	Source    string // Commentary source/author if available
}

// CommentaryChapter represents commentary entries for a chapter.
type CommentaryChapter struct {
	Number  int
	Entries []CommentaryEntry
}

// CommentaryBook represents commentary entries for a book.
type CommentaryBook struct {
	ID        string
	Name      string
	Testament string
	Chapters  []CommentaryChapter
}

// NewZComParser creates a new zCom format parser.
func NewZComParser(module *Module, swordDir string) *ZComParser {
	return &ZComParser{
		module:   module,
		dataPath: module.ResolveDataPath(swordDir),
	}
}

// loadIndices loads the book and verse index files.
func (p *ZComParser) loadIndices() error {
	if p.loaded {
		return nil
	}

	// Load OT indices if they exist
	if err := p.loadTestamentIndices("ot"); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("loading OT indices: %w", err)
		}
	} else {
		p.hasOT = true
	}

	// Load NT indices if they exist
	if err := p.loadTestamentIndices("nt"); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("loading NT indices: %w", err)
		}
	} else {
		p.hasNT = true
	}

	p.loaded = true
	return nil
}

// loadTestamentIndices loads index files for a specific testament.
func (p *ZComParser) loadTestamentIndices(testament string) error {
	// Try .bzs/.bzv format first (block format)
	bzsPath := filepath.Join(p.dataPath, testament+".bzs")
	bzvPath := filepath.Join(p.dataPath, testament+".bzv")

	// Check if compressed format exists (.czs/.czv/.czz)
	if _, err := os.Stat(bzsPath); os.IsNotExist(err) {
		czsPath := filepath.Join(p.dataPath, testament+".czs")
		if _, err := os.Stat(czsPath); err == nil {
			bzsPath = czsPath
			bzvPath = filepath.Join(p.dataPath, testament+".czv")
		}
	}

	// Read book index (.bzs)
	bzsData, err := os.ReadFile(bzsPath) // #nosec G304 -- internal path construction
	if err != nil {
		return err
	}

	numBooks := len(bzsData) / 12
	bookIndex := make([]uint32, numBooks)
	for i := 0; i < numBooks; i++ {
		offset := i * 12
		bookIndex[i] = binary.LittleEndian.Uint32(bzsData[offset:])
	}

	// Read verse index (.bzv)
	bzvData, err := os.ReadFile(bzvPath) // #nosec G304 -- internal path construction
	if err != nil {
		return err
	}

	numVerses := len(bzvData) / 10
	verseIndex := make([]ZComVerseEntry, numVerses)
	for i := 0; i < numVerses; i++ {
		offset := i * 10
		verseIndex[i] = ZComVerseEntry{
			BlockNum:   binary.LittleEndian.Uint32(bzvData[offset:]),
			VerseStart: binary.LittleEndian.Uint32(bzvData[offset+4:]),
			VerseLen:   binary.LittleEndian.Uint16(bzvData[offset+8:]),
		}
	}

	// Merge with existing indices
	if testament == "ot" {
		p.bookIndex = bookIndex
		p.verseIndex = verseIndex
	} else {
		p.bookIndex = append(p.bookIndex, bookIndex...)
		p.verseIndex = append(p.verseIndex, verseIndex...)
	}

	return nil
}

// GetEntry retrieves a commentary entry for a specific verse.
func (p *ZComParser) GetEntry(ref Reference) (*CommentaryEntry, error) {
	if err := p.loadIndices(); err != nil {
		return nil, fmt.Errorf("loading indices: %w", err)
	}

	bookInfo, ok := GetBookInfo(ref.Book)
	if !ok {
		return nil, fmt.Errorf("unknown book: %s", ref.Book)
	}

	verseIdx, err := p.calculateVerseIndex(bookInfo, ref.Chapter, ref.Verse)
	if err != nil {
		return nil, err
	}

	if verseIdx >= len(p.verseIndex) {
		return nil, fmt.Errorf("verse index out of range: %d", verseIdx)
	}

	entry := p.verseIndex[verseIdx]
	if entry.VerseLen == 0 {
		return &CommentaryEntry{
			Reference: ref,
			Text:      "",
		}, nil
	}

	text, err := p.readEntryText(bookInfo.Testament, entry)
	if err != nil {
		return nil, fmt.Errorf("reading entry text: %w", err)
	}

	return &CommentaryEntry{
		Reference: ref,
		Text:      text,
		Source:    p.module.Title,
	}, nil
}

// calculateVerseIndex calculates the index into the verse array for a reference.
// Uses the same versification-based calculation as zText for consistency.
func (p *ZComParser) calculateVerseIndex(bookInfo *BookInfo, chapter, verse int) (int, error) {
	var verseIdx int

	// Handle NT-only modules: use NT-specific verse indexing
	if p.hasNT && !p.hasOT {
		if bookInfo.Testament != "NT" {
			return 0, fmt.Errorf("book %s is not in NT (NT-only module)", bookInfo.ID)
		}
		verseIdx = CalculateNTVerseIndex(bookInfo.ID, chapter, verse)
		if verseIdx < 0 {
			return 0, fmt.Errorf("could not calculate NT verse index for %s %d:%d", bookInfo.ID, chapter, verse)
		}
		return verseIdx, nil
	}

	// Handle OT-only modules: use OT-specific verse indexing
	if p.hasOT && !p.hasNT {
		if bookInfo.Testament != "OT" {
			return 0, fmt.Errorf("book %s is not in OT (OT-only module)", bookInfo.ID)
		}
		verseIdx = CalculateVerseIndex(bookInfo.ID, chapter, verse)
		if verseIdx < 0 {
			return 0, fmt.Errorf("could not calculate verse index for %s %d:%d", bookInfo.ID, chapter, verse)
		}
		return verseIdx, nil
	}

	// Full Bible modules: use testament-appropriate indexing
	if bookInfo.Testament == "NT" {
		// For full Bible, NT verses continue after OT
		// But if this is a combined module, the NT verseIndex starts fresh
		verseIdx = CalculateNTVerseIndex(bookInfo.ID, chapter, verse)
	} else {
		verseIdx = CalculateVerseIndex(bookInfo.ID, chapter, verse)
	}

	if verseIdx < 0 {
		return 0, fmt.Errorf("could not calculate verse index for %s %d:%d", bookInfo.ID, chapter, verse)
	}

	return verseIdx, nil
}

// readEntryText reads and decompresses entry text from the data file.
// Note: zCom format uses BlockNum as block index, VerseStart as offset in decompressed block,
// and VerseLen as the length of the entry. This requires reading the block index (.bzs) first.
func (p *ZComParser) readEntryText(testament string, entry ZComVerseEntry) (string, error) {
	var prefix string
	if testament == "OT" {
		prefix = "ot"
	} else {
		prefix = "nt"
	}

	// Read block index to get block offset and compressed size
	// Try .bzs first, fall back to .czs
	bzsPath := filepath.Join(p.dataPath, prefix+".bzs")
	if _, err := os.Stat(bzsPath); os.IsNotExist(err) {
		bzsPath = filepath.Join(p.dataPath, prefix+".czs")
	}
	bzsData, err := os.ReadFile(bzsPath)
	if err != nil {
		return "", fmt.Errorf("reading block index: %w", err)
	}

	// Each block index entry is 12 bytes: offset (4), compSize (4), uncompSize (4)
	blockIdx := int(entry.BlockNum)
	if blockIdx*12+12 > len(bzsData) {
		return "", fmt.Errorf("block number out of range: %d", blockIdx)
	}

	blockOffset := binary.LittleEndian.Uint32(bzsData[blockIdx*12:])
	compSize := binary.LittleEndian.Uint32(bzsData[blockIdx*12+4:])

	if compSize == 0 {
		return "", nil
	}

	// Read compressed block from .bzz or .czz
	bzzPath := filepath.Join(p.dataPath, prefix+".bzz")
	if _, err := os.Stat(bzzPath); os.IsNotExist(err) {
		bzzPath = filepath.Join(p.dataPath, prefix+".czz")
	}
	file, err := os.Open(bzzPath) // #nosec G304 -- internal path construction
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := file.Seek(int64(blockOffset), io.SeekStart); err != nil {
		return "", err
	}

	compressedData := make([]byte, compSize)
	if _, err := io.ReadFull(file, compressedData); err != nil {
		return "", err
	}

	reader, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return "", fmt.Errorf("creating zlib reader: %w", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("decompressing: %w", err)
	}

	// Extract entry text from decompressed block
	start := int(entry.VerseStart)
	length := int(entry.VerseLen)
	if start+length > len(decompressed) {
		return "", fmt.Errorf("entry extends beyond block: start=%d len=%d blockLen=%d",
			start, length, len(decompressed))
	}

	return string(decompressed[start : start+length]), nil
}

// GetChapterEntries retrieves all commentary entries for a chapter.
func (p *ZComParser) GetChapterEntries(book string, chapter int) (*CommentaryChapter, error) {
	bookInfo, ok := GetBookInfo(book)
	if !ok {
		return nil, fmt.Errorf("unknown book: %s", book)
	}

	if chapter < 1 || chapter > bookInfo.Chapters {
		return nil, fmt.Errorf("invalid chapter %d for %s", chapter, book)
	}

	numVerses := GetVersesInChapter(book, chapter)
	entries := make([]CommentaryEntry, 0, numVerses)

	for v := 1; v <= numVerses; v++ {
		ref := Reference{Book: book, Chapter: chapter, Verse: v}
		entry, err := p.GetEntry(ref)
		if err != nil {
			continue
		}
		if entry.Text != "" {
			entries = append(entries, *entry)
		}
	}

	return &CommentaryChapter{
		Number:  chapter,
		Entries: entries,
	}, nil
}

// GetBookEntries retrieves all commentary entries for a book.
func (p *ZComParser) GetBookEntries(book string) (*CommentaryBook, error) {
	bookInfo, ok := GetBookInfo(book)
	if !ok {
		return nil, fmt.Errorf("unknown book: %s", book)
	}

	chapters := make([]CommentaryChapter, 0, bookInfo.Chapters)

	for ch := 1; ch <= bookInfo.Chapters; ch++ {
		chapter, err := p.GetChapterEntries(book, ch)
		if err != nil {
			continue
		}
		if len(chapter.Entries) > 0 {
			chapters = append(chapters, *chapter)
		}
	}

	return &CommentaryBook{
		ID:        bookInfo.ID,
		Name:      bookInfo.Name,
		Testament: bookInfo.Testament,
		Chapters:  chapters,
	}, nil
}

// GetAllEntries retrieves all commentary entries in the module.
func (p *ZComParser) GetAllEntries() ([]*CommentaryBook, error) {
	books := make([]*CommentaryBook, 0, len(KJVBooks))

	for _, bookInfo := range KJVBooks {
		book, err := p.GetBookEntries(bookInfo.ID)
		if err != nil {
			continue
		}
		if len(book.Chapters) > 0 {
			books = append(books, book)
		}
	}

	return books, nil
}
