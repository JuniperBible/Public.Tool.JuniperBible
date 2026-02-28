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
	"sync"
)

// ZTextParser parses zText format SWORD modules (compressed Bible text).
//
// The zText format consists of three files per testament:
//   - .bzs (block index): 12-byte entries (offset, compSize, uncompSize)
//   - .bzv (verse index): 10-byte entries per verse (blockNum, verseStart, verseLen)
//   - .bzz (compressed data): zlib-compressed text blocks
type ZTextParser struct {
	module        *Module
	dataPath      string
	versification *VersificationSystem // Versification system for this module

	// Index data (loaded on demand)
	otBZS     []BlockIndexEntry
	otBZV     []VerseIndexEntry
	ntBZS     []BlockIndexEntry
	ntBZV     []VerseIndexEntry
	loaded    bool
	loadMutex sync.Mutex

	// Block cache for decompressed blocks
	blockCache   map[string][]byte // key: "ot:N" or "nt:N"
	blockCacheMu sync.RWMutex
}

// BlockIndexEntry represents an entry in the block index (.bzs file).
type BlockIndexEntry struct {
	Offset     uint32 // Offset into .bzz file
	CompSize   uint32 // Compressed size in bytes
	UncompSize uint32 // Uncompressed size (informational)
}

// VerseIndexEntry represents an entry in the verse index (.bzv file).
type VerseIndexEntry struct {
	BlockNum   uint32 // Block number (index into .bzs)
	VerseStart uint32 // Offset within uncompressed block
	VerseLen   uint16 // Verse length in bytes
}

// NewZTextParser creates a new zText format parser.
// It automatically detects the versification system from the module configuration.
func NewZTextParser(module *Module, swordDir string) *ZTextParser {
	// Determine versification system from module config
	versName := module.Versification
	if versName == "" {
		versName = "KJV" // Default to KJV
	}
	versName = NormalizeVersificationName(versName)

	versification := GetVersification(versName)
	if versification == nil {
		// Fall back to KJV if unknown versification
		versification = GetVersification("KJV")
	}

	return &ZTextParser{
		module:        module,
		dataPath:      module.ResolveDataPath(swordDir),
		versification: versification,
		blockCache:    make(map[string][]byte),
	}
}

// NewZTextParserWithVersification creates a new zText format parser with explicit versification.
func NewZTextParserWithVersification(module *Module, swordDir string, versification *VersificationSystem) *ZTextParser {
	if versification == nil {
		versification = GetVersification("KJV")
	}
	return &ZTextParser{
		module:        module,
		dataPath:      module.ResolveDataPath(swordDir),
		versification: versification,
		blockCache:    make(map[string][]byte),
	}
}

// Versification returns the versification system used by this parser.
func (p *ZTextParser) Versification() *VersificationSystem {
	return p.versification
}

// loadIndices loads the book and verse index files.
func (p *ZTextParser) loadIndices() error {
	p.loadMutex.Lock()
	defer p.loadMutex.Unlock()

	if p.loaded {
		return nil
	}

	// Load OT indices if they exist
	otBZS, otBZV, err := p.loadTestamentIndices("ot")
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("loading OT indices: %w", err)
	}
	p.otBZS = otBZS
	p.otBZV = otBZV

	// Load NT indices if they exist
	ntBZS, ntBZV, err := p.loadTestamentIndices("nt")
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("loading NT indices: %w", err)
	}
	p.ntBZS = ntBZS
	p.ntBZV = ntBZV

	p.loaded = true
	return nil
}

// loadTestamentIndices loads index files for a specific testament.
func (p *ZTextParser) loadTestamentIndices(testament string) ([]BlockIndexEntry, []VerseIndexEntry, error) {
	bzsPath := filepath.Join(p.dataPath, testament+".bzs")
	bzvPath := filepath.Join(p.dataPath, testament+".bzv")

	// Read block index (.bzs) - 12 bytes per entry
	bzsData, err := os.ReadFile(bzsPath) // #nosec G304 -- internal path construction
	if err != nil {
		return nil, nil, err
	}

	numBlocks := len(bzsData) / 12
	blockIndex := make([]BlockIndexEntry, numBlocks)
	for i := 0; i < numBlocks; i++ {
		offset := i * 12
		blockIndex[i] = BlockIndexEntry{
			Offset:     binary.LittleEndian.Uint32(bzsData[offset:]),
			CompSize:   binary.LittleEndian.Uint32(bzsData[offset+4:]),
			UncompSize: binary.LittleEndian.Uint32(bzsData[offset+8:]),
		}
	}

	// Read verse index (.bzv) - 10 bytes per entry
	bzvData, err := os.ReadFile(bzvPath) // #nosec G304 -- internal path construction
	if err != nil {
		return nil, nil, err
	}

	numVerses := len(bzvData) / 10
	verseIndex := make([]VerseIndexEntry, numVerses)
	for i := 0; i < numVerses; i++ {
		offset := i * 10
		verseIndex[i] = VerseIndexEntry{
			BlockNum:   binary.LittleEndian.Uint32(bzvData[offset:]),
			VerseStart: binary.LittleEndian.Uint32(bzvData[offset+4:]),
			VerseLen:   binary.LittleEndian.Uint16(bzvData[offset+8:]),
		}
	}

	return blockIndex, verseIndex, nil
}

// GetVerse retrieves a single verse from the module.
func (p *ZTextParser) GetVerse(ref Reference) (*Verse, error) {
	if err := p.loadIndices(); err != nil {
		return nil, fmt.Errorf("loading indices: %w", err)
	}

	// Get book info from the versification system
	book, ok := p.versification.GetBook(ref.Book)
	if !ok {
		// Fallback to KJV book info for backward compatibility
		bookInfo, ok := GetBookInfo(ref.Book)
		if !ok {
			return nil, fmt.Errorf("unknown book: %s", ref.Book)
		}
		// Use legacy calculation
		return p.getVerseWithLegacyIndex(ref, bookInfo)
	}

	// Determine which testament and calculate verse index
	var bzs []BlockIndexEntry
	var bzv []VerseIndexEntry
	var testament string
	var verseIdx int

	if book.Testament == "OT" || book.Testament == "AP" {
		// Apocrypha is typically stored with OT in SWORD modules
		bzs = p.otBZS
		bzv = p.otBZV
		testament = "ot"
		verseIdx = p.versification.CalculateVerseIndexForSystem(ref.Book, ref.Chapter, ref.Verse)
	} else {
		bzs = p.ntBZS
		bzv = p.ntBZV
		testament = "nt"
		verseIdx = p.versification.CalculateVerseIndexForSystem(ref.Book, ref.Chapter, ref.Verse)
	}

	if verseIdx < 0 || verseIdx >= len(bzv) {
		return nil, fmt.Errorf("verse index out of range: %d (max %d)", verseIdx, len(bzv)-1)
	}

	entry := bzv[verseIdx]
	if entry.VerseLen == 0 {
		// Empty verse
		return &Verse{
			Reference: ref,
			Text:      "",
		}, nil
	}

	// Get the decompressed block
	block, err := p.getBlock(testament, int(entry.BlockNum), bzs)
	if err != nil {
		return nil, fmt.Errorf("getting block %d: %w", entry.BlockNum, err)
	}

	// Extract verse text from block
	if int(entry.VerseStart)+int(entry.VerseLen) > len(block) {
		return nil, fmt.Errorf("verse extends beyond block: start=%d len=%d blockLen=%d",
			entry.VerseStart, entry.VerseLen, len(block))
	}

	text := string(block[entry.VerseStart : entry.VerseStart+uint32(entry.VerseLen)])

	return &Verse{
		Reference: ref,
		Text:      text,
	}, nil
}

// getVerseWithLegacyIndex retrieves a verse using the legacy KJV-based index calculation.
// This is used as a fallback when the book is not found in the versification system.
func (p *ZTextParser) getVerseWithLegacyIndex(ref Reference, bookInfo *BookInfo) (*Verse, error) {
	var bzs []BlockIndexEntry
	var bzv []VerseIndexEntry
	var testament string
	var verseIdx int

	if bookInfo.Testament == "OT" {
		bzs = p.otBZS
		bzv = p.otBZV
		testament = "ot"
		verseIdx = CalculateOTVerseIndex(ref.Book, ref.Chapter, ref.Verse)
	} else {
		bzs = p.ntBZS
		bzv = p.ntBZV
		testament = "nt"
		verseIdx = CalculateNTVerseIndex(ref.Book, ref.Chapter, ref.Verse)
	}

	if verseIdx < 0 || verseIdx >= len(bzv) {
		return nil, fmt.Errorf("verse index out of range: %d (max %d)", verseIdx, len(bzv)-1)
	}

	entry := bzv[verseIdx]
	if entry.VerseLen == 0 {
		return &Verse{Reference: ref, Text: ""}, nil
	}

	block, err := p.getBlock(testament, int(entry.BlockNum), bzs)
	if err != nil {
		return nil, fmt.Errorf("getting block %d: %w", entry.BlockNum, err)
	}

	if int(entry.VerseStart)+int(entry.VerseLen) > len(block) {
		return nil, fmt.Errorf("verse extends beyond block: start=%d len=%d blockLen=%d",
			entry.VerseStart, entry.VerseLen, len(block))
	}

	text := string(block[entry.VerseStart : entry.VerseStart+uint32(entry.VerseLen)])
	return &Verse{Reference: ref, Text: text}, nil
}

// getBlock retrieves and decompresses a block, using cache.
func (p *ZTextParser) getBlock(testament string, blockNum int, bzs []BlockIndexEntry) ([]byte, error) {
	cacheKey := fmt.Sprintf("%s:%d", testament, blockNum)

	// Check cache
	p.blockCacheMu.RLock()
	if cached, ok := p.blockCache[cacheKey]; ok {
		p.blockCacheMu.RUnlock()
		return cached, nil
	}
	p.blockCacheMu.RUnlock()

	// Load and decompress
	if blockNum >= len(bzs) {
		return nil, fmt.Errorf("block number out of range: %d", blockNum)
	}

	entry := bzs[blockNum]
	if entry.CompSize == 0 {
		return nil, fmt.Errorf("empty block: %d", blockNum)
	}

	// Read compressed data
	bzzPath := filepath.Join(p.dataPath, testament+".bzz")
	file, err := os.Open(bzzPath) // #nosec G304 -- internal path construction
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if _, err := file.Seek(int64(entry.Offset), io.SeekStart); err != nil {
		return nil, err
	}

	compressedData := make([]byte, entry.CompSize)
	if _, err := io.ReadFull(file, compressedData); err != nil {
		return nil, fmt.Errorf("reading compressed data: %w", err)
	}

	// Decompress using zlib
	reader, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, fmt.Errorf("creating zlib reader: %w", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("decompressing: %w", err)
	}

	// Cache the result
	p.blockCacheMu.Lock()
	p.blockCache[cacheKey] = decompressed
	p.blockCacheMu.Unlock()

	return decompressed, nil
}

// GetChapter retrieves all verses in a chapter.
func (p *ZTextParser) GetChapter(book string, chapter int) (*Chapter, error) {
	// Try versification system first
	versBook, ok := p.versification.GetBook(book)
	if ok {
		if chapter < 1 || chapter > versBook.Chapters() {
			return nil, fmt.Errorf("invalid chapter %d for %s (has %d chapters)",
				chapter, book, versBook.Chapters())
		}

		numVerses := versBook.Verses(chapter)
		verses := make([]Verse, 0, numVerses)

		for v := 1; v <= numVerses; v++ {
			ref := Reference{Book: book, Chapter: chapter, Verse: v}
			verse, err := p.GetVerse(ref)
			if err != nil {
				continue
			}
			if verse.Text != "" {
				verses = append(verses, *verse)
			}
		}

		return &Chapter{
			Number: chapter,
			Verses: verses,
		}, nil
	}

	// Fallback to legacy KJV-based lookup
	bookInfo, ok := GetBookInfo(book)
	if !ok {
		return nil, fmt.Errorf("unknown book: %s", book)
	}

	if chapter < 1 || chapter > bookInfo.Chapters {
		return nil, fmt.Errorf("invalid chapter %d for %s (has %d chapters)",
			chapter, book, bookInfo.Chapters)
	}

	numVerses := GetVersesInChapter(book, chapter)
	verses := make([]Verse, 0, numVerses)

	for v := 1; v <= numVerses; v++ {
		ref := Reference{Book: book, Chapter: chapter, Verse: v}
		verse, err := p.GetVerse(ref)
		if err != nil {
			continue
		}
		if verse.Text != "" {
			verses = append(verses, *verse)
		}
	}

	return &Chapter{
		Number: chapter,
		Verses: verses,
	}, nil
}

// GetBook retrieves all chapters in a book.
func (p *ZTextParser) GetBook(book string) (*Book, error) {
	// Try versification system first
	versBook, ok := p.versification.GetBook(book)
	if ok {
		chapters := make([]Chapter, 0, versBook.Chapters())

		for ch := 1; ch <= versBook.Chapters(); ch++ {
			chapter, err := p.GetChapter(book, ch)
			if err != nil {
				continue
			}
			chapters = append(chapters, *chapter)
		}

		return &Book{
			ID:        versBook.ID,
			Name:      versBook.Name,
			Abbrev:    versBook.Abbrev,
			Testament: versBook.Testament,
			Chapters:  chapters,
		}, nil
	}

	// Fallback to legacy KJV-based lookup
	bookInfo, ok := GetBookInfo(book)
	if !ok {
		return nil, fmt.Errorf("unknown book: %s", book)
	}

	chapters := make([]Chapter, 0, bookInfo.Chapters)

	for ch := 1; ch <= bookInfo.Chapters; ch++ {
		chapter, err := p.GetChapter(book, ch)
		if err != nil {
			continue
		}
		chapters = append(chapters, *chapter)
	}

	return &Book{
		ID:        bookInfo.ID,
		Name:      bookInfo.Name,
		Abbrev:    bookInfo.Abbrev,
		Testament: bookInfo.Testament,
		Chapters:  chapters,
	}, nil
}

// GetAllBooks retrieves all books in the module.
func (p *ZTextParser) GetAllBooks() ([]*Book, error) {
	books := make([]*Book, 0, len(p.versification.Books))

	for _, versBook := range p.versification.Books {
		book, err := p.GetBook(versBook.ID)
		if err != nil {
			continue
		}
		if len(book.Chapters) > 0 {
			books = append(books, book)
		}
	}

	return books, nil
}

// ClearCache clears the block cache.
func (p *ZTextParser) ClearCache() {
	p.blockCacheMu.Lock()
	p.blockCache = make(map[string][]byte)
	p.blockCacheMu.Unlock()
}

// Ensure ZTextParser implements Parser interface
var _ Parser = (*ZTextParser)(nil)

// ParseConfig implements Parser interface - delegates to ParseConf.
func (p *ZTextParser) ParseConfig(confPath string) (*Module, error) {
	return ParseConf(confPath)
}
