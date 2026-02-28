// Package sword provides parsers for SWORD Bible module format.
package sword

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RawGenBookParser parses RawGenBook format SWORD modules (general books like
// commentaries, devotionals, Quran, 1 Enoch, etc.).
//
// The RawGenBook format stores hierarchical content with TreeKey navigation:
//   - .idx (offset index): 4-byte offsets into .bdt file per entry
//   - .dat (key data): TreeKey structure with entry keys and metadata
//   - .bdt (content data): Raw text content for each entry
type RawGenBookParser struct {
	module   *Module
	dataPath string

	// Index data (loaded on demand)
	entries map[string]GenBookEntry
	keys    []string
	loaded  bool
}

// GenBookEntry represents an entry in a general book.
type GenBookEntry struct {
	Key      string // Entry key/path (e.g., "Chapter 1" or "1.2.3")
	Offset   uint32 // Offset into .bdt file
	Size     uint32 // Size of content in bytes
	Level    int    // Hierarchy level (0 = root)
	Parent   string // Parent key (empty for root entries)
	Children []string
}

// GenBookContent represents the full content of a general book entry.
type GenBookContent struct {
	Key     string
	Title   string
	Content string
	Level   int
}

// NewRawGenBookParser creates a new RawGenBook format parser.
func NewRawGenBookParser(module *Module, swordDir string) *RawGenBookParser {
	return &RawGenBookParser{
		module:   module,
		dataPath: module.ResolveDataPath(swordDir),
		entries:  make(map[string]GenBookEntry),
	}
}

// loadIndices loads the book index files (.idx, .dat).
func (p *RawGenBookParser) loadIndices() error {
	if p.loaded {
		return nil
	}

	// Find the module files (typically named after module ID)
	baseName := strings.ToLower(p.module.ID)

	// Try common naming patterns
	patterns := []string{
		baseName,
		strings.ReplaceAll(baseName, " ", ""),
		strings.ReplaceAll(baseName, "-", ""),
	}

	var idxPath, datPath, bdtPath string
	for _, pattern := range patterns {
		testIdx := filepath.Join(p.dataPath, pattern+".idx")
		testDat := filepath.Join(p.dataPath, pattern+".dat")
		testBdt := filepath.Join(p.dataPath, pattern+".bdt")

		if _, err := os.Stat(testIdx); err == nil {
			idxPath = testIdx
			datPath = testDat
			bdtPath = testBdt
			break
		}
	}

	if idxPath == "" {
		// Try finding any .idx file in the directory
		entries, err := os.ReadDir(p.dataPath)
		if err != nil {
			return fmt.Errorf("reading data directory: %w", err)
		}
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".idx") {
				base := strings.TrimSuffix(entry.Name(), ".idx")
				idxPath = filepath.Join(p.dataPath, entry.Name())
				datPath = filepath.Join(p.dataPath, base+".dat")
				bdtPath = filepath.Join(p.dataPath, base+".bdt")
				break
			}
		}
	}

	if idxPath == "" {
		return fmt.Errorf("no index files found in %s", p.dataPath)
	}

	// Read the index file (.idx) - array of 4-byte offsets
	idxData, err := os.ReadFile(idxPath) // #nosec G304 -- internal path construction
	if err != nil {
		return fmt.Errorf("reading index file: %w", err)
	}

	// Read the key/metadata file (.dat)
	datData, err := os.ReadFile(datPath) // #nosec G304 -- internal path construction
	if err != nil {
		return fmt.Errorf("reading dat file: %w", err)
	}

	// Parse entries
	numEntries := len(idxData) / 4
	if numEntries == 0 {
		return fmt.Errorf("empty index file")
	}

	// Parse each entry from .dat file
	if err := p.parseTreeKeyData(datData, idxData, bdtPath); err != nil {
		return fmt.Errorf("parsing tree key data: %w", err)
	}

	// Sort keys for consistent ordering
	p.keys = make([]string, 0, len(p.entries))
	for key := range p.entries {
		p.keys = append(p.keys, key)
	}
	sort.Strings(p.keys)

	p.loaded = true
	return nil
}

// parseTreeKeyData parses the TreeKey structure from .dat file.
// Format per entry:
//   - 8 bytes: 0xFFFFFFFF 0xFFFFFFFF (marker)
//   - 4 bytes: offset into .bdt (or internal reference)
//   - 4 bytes: size (or 0)
//   - 4 bytes: unknown/flags
//   - variable: null-terminated UTF-8 key string
func (p *RawGenBookParser) parseTreeKeyData(datData, idxData []byte, bdtPath string) error {
	numEntries := len(idxData) / 4
	offsets := make([]uint32, numEntries)

	// Parse offsets from .idx file
	for i := 0; i < numEntries; i++ {
		offsets[i] = binary.LittleEndian.Uint32(idxData[i*4:])
	}

	// Calculate sizes from consecutive offsets
	sizes := make([]uint32, numEntries)
	for i := 0; i < numEntries; i++ {
		if i+1 < numEntries {
			sizes[i] = offsets[i+1] - offsets[i]
		} else {
			// Last entry - estimate size from file or use remaining
			if fi, err := os.Stat(bdtPath); err == nil {
				sizes[i] = uint32(fi.Size()) - offsets[i]
			} else {
				sizes[i] = 4096 // Default size for last entry
			}
		}
	}

	// Parse key names from .dat file
	// Look for patterns: 0xFF markers followed by metadata and null-terminated strings
	pos := 0
	entryIndex := 0

	for pos < len(datData) && entryIndex < numEntries {
		// Look for 0xFF marker sequence
		if pos+8 <= len(datData) {
			// Check for 8-byte 0xFF marker
			isMarker := true
			for i := 0; i < 8 && pos+i < len(datData); i++ {
				if datData[pos+i] != 0xFF {
					isMarker = false
					break
				}
			}

			if isMarker {
				pos += 8 // Skip 8-byte marker

				// Skip metadata bytes until we find readable text
				// Look for start of key name (non-null, non-0xFF byte)
				keyStart := pos
				for keyStart < len(datData) && (datData[keyStart] == 0x00 || datData[keyStart] == 0xFF) {
					keyStart++
				}

				// Skip any remaining metadata (usually 4-12 bytes of binary data)
				// Keys start after binary metadata
				metaEnd := keyStart
				for metaEnd < len(datData) && metaEnd < keyStart+20 {
					// Look for the start of UTF-8 text
					if metaEnd+1 < len(datData) {
						b := datData[metaEnd]
						// Check for printable ASCII or valid UTF-8 start byte
						if (b >= 0x20 && b <= 0x7E) || (b >= 0xC0 && b <= 0xFD) {
							break
						}
					}
					metaEnd++
				}

				// Find null-terminated key string
				keyEnd := metaEnd
				for keyEnd < len(datData) && datData[keyEnd] != 0x00 {
					keyEnd++
				}

				if keyEnd > metaEnd {
					key := string(datData[metaEnd:keyEnd])
					key = strings.TrimSpace(key)

					if key != "" && entryIndex < numEntries {
						entry := GenBookEntry{
							Key:    key,
							Offset: offsets[entryIndex],
							Size:   sizes[entryIndex],
							Level:  0, // Will be determined by key structure
						}
						p.entries[key] = entry
						entryIndex++
					}
				}

				pos = keyEnd + 1
				continue
			}
		}
		pos++
	}

	// If we didn't find entries via marker parsing, try simpler approach
	if len(p.entries) == 0 {
		return p.parseSimpleKeyData(datData, offsets, sizes)
	}

	return nil
}

// parseSimpleKeyData is a fallback parser for simpler .dat formats.
func (p *RawGenBookParser) parseSimpleKeyData(datData []byte, offsets, sizes []uint32) error {
	// Simple approach: look for null-terminated strings
	var keys []string
	start := 0

	for i, b := range datData {
		if b == 0x00 {
			if i > start {
				// Skip binary/non-printable sections
				segment := datData[start:i]
				// Check if it looks like text
				if p.isLikelyText(segment) {
					key := strings.TrimSpace(string(segment))
					if key != "" && len(key) > 1 {
						keys = append(keys, key)
					}
				}
			}
			start = i + 1
		}
	}

	// Match keys with offsets
	for i, key := range keys {
		if i < len(offsets) {
			entry := GenBookEntry{
				Key:    key,
				Offset: offsets[i],
				Size:   sizes[i],
			}
			p.entries[key] = entry
		}
	}

	return nil
}

// isLikelyText checks if a byte slice looks like text content.
func (p *RawGenBookParser) isLikelyText(data []byte) bool {
	if len(data) < 2 {
		return false
	}

	printableCount := 0
	for _, b := range data {
		// Count printable ASCII and valid UTF-8 bytes
		if (b >= 0x20 && b <= 0x7E) || b >= 0x80 {
			printableCount++
		}
	}

	// At least 50% should be printable
	return printableCount*2 >= len(data)
}

// GetEntry retrieves a book entry by key.
func (p *RawGenBookParser) GetEntry(key string) (*GenBookContent, error) {
	if err := p.loadIndices(); err != nil {
		return nil, fmt.Errorf("loading indices: %w", err)
	}

	// Try exact match first
	entry, ok := p.entries[key]
	if !ok {
		// Try case-insensitive match
		for k, e := range p.entries {
			if strings.EqualFold(k, key) {
				entry = e
				ok = true
				break
			}
		}
	}

	if !ok {
		return nil, fmt.Errorf("entry not found: %s", key)
	}

	content, err := p.readEntryContent(entry)
	if err != nil {
		return nil, fmt.Errorf("reading entry content: %w", err)
	}

	return &GenBookContent{
		Key:     entry.Key,
		Title:   entry.Key,
		Content: content,
		Level:   entry.Level,
	}, nil
}

// readEntryContent reads the content from the .bdt file.
func (p *RawGenBookParser) readEntryContent(entry GenBookEntry) (string, error) {
	// Find .bdt file
	baseName := strings.ToLower(p.module.ID)
	patterns := []string{baseName, strings.ReplaceAll(baseName, " ", ""), strings.ReplaceAll(baseName, "-", "")}

	var bdtPath string
	for _, pattern := range patterns {
		testPath := filepath.Join(p.dataPath, pattern+".bdt")
		if _, err := os.Stat(testPath); err == nil {
			bdtPath = testPath
			break
		}
	}

	if bdtPath == "" {
		// Try finding any .bdt file
		entries, err := os.ReadDir(p.dataPath)
		if err != nil {
			return "", err
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".bdt") {
				bdtPath = filepath.Join(p.dataPath, e.Name())
				break
			}
		}
	}

	if bdtPath == "" {
		return "", fmt.Errorf("no .bdt file found")
	}

	file, err := os.Open(bdtPath) // #nosec G304 -- internal path construction
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := file.Seek(int64(entry.Offset), io.SeekStart); err != nil {
		return "", err
	}

	data := make([]byte, entry.Size)
	n, err := file.Read(data)
	if err != nil && err != io.EOF {
		return "", err
	}

	// Trim any trailing nulls
	content := bytes.TrimRight(data[:n], "\x00")

	return string(content), nil
}

// GetAllKeys returns all entry keys in the book.
func (p *RawGenBookParser) GetAllKeys() ([]string, error) {
	if err := p.loadIndices(); err != nil {
		return nil, fmt.Errorf("loading indices: %w", err)
	}

	return p.keys, nil
}

// GetAllEntries retrieves all entries in the book.
func (p *RawGenBookParser) GetAllEntries() ([]*GenBookContent, error) {
	if err := p.loadIndices(); err != nil {
		return nil, fmt.Errorf("loading indices: %w", err)
	}

	entries := make([]*GenBookContent, 0, len(p.keys))
	for _, key := range p.keys {
		entry, err := p.GetEntry(key)
		if err != nil {
			continue // Skip entries we can't read
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// SearchKeys searches for keys matching a pattern.
func (p *RawGenBookParser) SearchKeys(pattern string) ([]string, error) {
	if err := p.loadIndices(); err != nil {
		return nil, fmt.Errorf("loading indices: %w", err)
	}

	pattern = strings.ToLower(pattern)
	var matches []string

	for _, key := range p.keys {
		if strings.Contains(strings.ToLower(key), pattern) {
			matches = append(matches, key)
		}
	}

	return matches, nil
}

// Module returns the underlying module metadata.
func (p *RawGenBookParser) Module() *Module {
	return p.module
}

// EntryCount returns the number of entries in the book.
func (p *RawGenBookParser) EntryCount() (int, error) {
	if err := p.loadIndices(); err != nil {
		return 0, err
	}
	return len(p.entries), nil
}
