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
	"sort"
	"strings"
)

// ZLDParser parses zLD format SWORD modules (compressed dictionaries/lexicons).
//
// The zLD format stores key-value pairs for dictionary/lexicon entries:
//   - .idx (key index): Maps keys to data positions
//   - .dat (key data): Contains the actual key strings
//   - .zdx (compressed index): Maps entries to compressed blocks
//   - .zdt (compressed data): zlib-compressed entry text
type ZLDParser struct {
	module   *Module
	dataPath string

	// Index data (loaded on demand)
	entries map[string]DictIndexEntry
	keys    []string
	loaded  bool

	// Resolved file paths (set during loadIndices)
	zdxPath string
	zdtPath string
	idxPath string
	datPath string
}

// DictIndexEntry represents an entry in the dictionary index.
type DictIndexEntry struct {
	Key            string
	Offset         uint32
	Size           uint32
	CompressedSize uint32
}

// DictEntry represents a dictionary/lexicon entry.
type DictEntry struct {
	Key        string
	Definition string
	StrongsNum string // For Strong's lexicons (e.g., "H430", "G2316")
}

// NewZLDParser creates a new zLD format parser.
func NewZLDParser(module *Module, swordDir string) *ZLDParser {
	return &ZLDParser{
		module:   module,
		dataPath: module.ResolveDataPath(swordDir),
		entries:  make(map[string]DictIndexEntry),
	}
}

// loadIndices loads the dictionary index files.
func (p *ZLDParser) loadIndices() error {
	if p.loaded {
		return nil
	}

	// Try compressed format first (.zdx/.zdt)
	if err := p.loadCompressedIndices(); err != nil {
		// Fall back to uncompressed format (.idx/.dat)
		if err := p.loadUncompressedIndices(); err != nil {
			return fmt.Errorf("failed to load indices: %w", err)
		}
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

// loadCompressedIndices loads compressed dictionary indices.
func (p *ZLDParser) loadCompressedIndices() error {
	// SWORD dictionary DataPath can be either:
	// 1. A directory path where files are named dict.zdx, dict.zdt
	// 2. A file prefix path like "modules/lexdict/zld/strongsgreek/dict"
	//    where the actual files are dict.zdx, dict.zdt (using prefix as base)

	// Try file prefix format first (more common in SWORD modules)
	// DataPath ends with "dict" and files are dict.zdx, dict.zdt
	zdxPath := p.dataPath + ".zdx"
	zdtPath := p.dataPath + ".zdt"
	idxPath := p.dataPath + ".idx"
	datPath := p.dataPath + ".dat"

	if _, err := os.Stat(zdxPath); os.IsNotExist(err) {
		// Try directory format (DataPath is directory, files inside)
		zdxPath = filepath.Join(p.dataPath, "dict.zdx")
		zdtPath = filepath.Join(p.dataPath, "dict.zdt")
		idxPath = filepath.Join(p.dataPath, "dict.idx")
		datPath = filepath.Join(p.dataPath, "dict.dat")
		if _, err := os.Stat(zdxPath); os.IsNotExist(err) {
			return err
		}
	}

	// Store resolved paths for later use
	p.zdxPath = zdxPath
	p.zdtPath = zdtPath
	p.idxPath = idxPath
	p.datPath = datPath

	zdxData, err := os.ReadFile(zdxPath) // #nosec G304 -- internal path construction
	if err != nil {
		return err
	}

	// Parse the compressed index (.zdx)
	// Format: 4-byte offset into .zdt, 4-byte compressed size per entry
	numCompressedEntries := len(zdxData) / 8
	if numCompressedEntries == 0 {
		return fmt.Errorf("empty compressed index file")
	}

	// Read key index file (.idx) - binary format
	// Each entry is 8 bytes: 4-byte offset into .dat, 4-byte size
	idxData, err := os.ReadFile(idxPath) // #nosec G304 -- internal path construction
	if err != nil {
		return fmt.Errorf("could not find key index: %w", err)
	}

	// Read key data file (.dat) - contains actual key strings
	datData, err := os.ReadFile(datPath) // #nosec G304 -- internal path construction
	if err != nil {
		return fmt.Errorf("could not find key data: %w", err)
	}

	// Parse .idx as binary: 8-byte entries (4-byte offset + 4-byte size)
	numKeys := len(idxData) / 8
	if numKeys == 0 {
		return fmt.Errorf("empty key index file")
	}

	// Extract keys from .dat using offsets from .idx
	for i := 0; i < numKeys; i++ {
		idxOffset := i * 8
		datOffset := binary.LittleEndian.Uint32(idxData[idxOffset:])
		datSize := binary.LittleEndian.Uint32(idxData[idxOffset+4:])

		if int(datOffset)+int(datSize) > len(datData) {
			continue // Skip invalid entries
		}

		// Read key string from .dat
		keyData := datData[datOffset : datOffset+datSize]

		// Key ends at CRLF or first null byte
		keyEnd := bytes.IndexAny(keyData, "\r\n\x00")
		if keyEnd == -1 {
			keyEnd = len(keyData)
		}
		key := strings.TrimSpace(string(keyData[:keyEnd]))
		if key == "" {
			continue
		}

		// Get compressed data offset from .zdx
		// The .zdx index corresponds to entries in .idx
		var compOffset, compSize uint32
		if i < numCompressedEntries {
			zdxOffset := i * 8
			compOffset = binary.LittleEndian.Uint32(zdxData[zdxOffset:])
			compSize = binary.LittleEndian.Uint32(zdxData[zdxOffset+4:])
		}

		entry := DictIndexEntry{
			Key:    key,
			Offset: compOffset,
			Size:   compSize,
		}
		p.entries[strings.ToLower(key)] = entry
	}

	return nil
}

// loadUncompressedIndices loads uncompressed dictionary indices.
func (p *ZLDParser) loadUncompressedIndices() error {
	// Try file prefix format first
	idxPath := p.dataPath + ".idx"
	datPath := p.dataPath + ".dat"

	if _, err := os.Stat(idxPath); os.IsNotExist(err) {
		// Try directory format
		idxPath = filepath.Join(p.dataPath, "dict.idx")
		datPath = filepath.Join(p.dataPath, "dict.dat")
		if _, err := os.Stat(idxPath); os.IsNotExist(err) {
			return err
		}
	}

	// Store resolved paths
	p.idxPath = idxPath
	p.datPath = datPath

	idxData, err := os.ReadFile(idxPath)
	if err != nil {
		return err
	}

	// Parse index entries
	// Format varies, but commonly: key string, offset, size
	keys := p.parseKeyIndex(idxData)

	datData, err := os.ReadFile(datPath)
	if err != nil {
		return err
	}

	// Simple parsing: assume sequential entries
	for i, key := range keys {
		entry := DictIndexEntry{
			Key:    key,
			Offset: uint32(i * 1000), // Placeholder - actual format varies
			Size:   1000,
		}
		p.entries[strings.ToLower(key)] = entry
		_ = datData // Will be used for actual reading
	}

	return nil
}

// parseKeyIndex parses a key index file into a list of keys.
func (p *ZLDParser) parseKeyIndex(data []byte) []string {
	var keys []string

	// Keys are typically null-terminated strings
	start := 0
	for i, b := range data {
		if b == 0 {
			if i > start {
				key := string(data[start:i])
				key = strings.TrimSpace(key)
				if key != "" {
					keys = append(keys, key)
				}
			}
			start = i + 1
		}
	}

	// Handle last key if no null terminator
	if start < len(data) {
		key := string(data[start:])
		key = strings.TrimSpace(key)
		if key != "" {
			keys = append(keys, key)
		}
	}

	return keys
}

// GetEntry retrieves a dictionary entry by key.
func (p *ZLDParser) GetEntry(key string) (*DictEntry, error) {
	if err := p.loadIndices(); err != nil {
		return nil, fmt.Errorf("loading indices: %w", err)
	}

	normalizedKey := strings.ToLower(strings.TrimSpace(key))
	indexEntry, ok := p.entries[normalizedKey]
	if !ok {
		return nil, fmt.Errorf("entry not found: %s", key)
	}

	definition, err := p.readEntryText(indexEntry)
	if err != nil {
		return nil, fmt.Errorf("reading entry: %w", err)
	}

	entry := &DictEntry{
		Key:        indexEntry.Key,
		Definition: definition,
	}

	// Extract Strong's number if this is a Strong's lexicon
	if p.isStrongsLexicon() {
		entry.StrongsNum = p.extractStrongsNumber(indexEntry.Key)
	}

	return entry, nil
}

// readEntryText reads the definition text for an entry.
func (p *ZLDParser) readEntryText(entry DictIndexEntry) (string, error) {
	// Use stored paths from loadIndices
	if p.zdtPath != "" {
		if _, err := os.Stat(p.zdtPath); err == nil {
			return p.readCompressedEntry(p.zdtPath, entry)
		}
	}

	// Fall back to uncompressed using stored path
	if p.datPath != "" {
		return p.readUncompressedEntry(p.datPath, entry)
	}

	// Last resort: try prefix format
	zdtPath := p.dataPath + ".zdt"
	if _, err := os.Stat(zdtPath); err == nil {
		return p.readCompressedEntry(zdtPath, entry)
	}

	datPath := p.dataPath + ".dat"
	return p.readUncompressedEntry(datPath, entry)
}

// readCompressedEntry reads from a compressed data file.
func (p *ZLDParser) readCompressedEntry(path string, entry DictIndexEntry) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := file.Seek(int64(entry.Offset), io.SeekStart); err != nil {
		return "", err
	}

	compressedData := make([]byte, entry.Size)
	if _, err := io.ReadFull(file, compressedData); err != nil {
		return "", err
	}

	reader, err := zlib.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		// Data might not be compressed, return as-is
		return string(compressedData), nil
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("decompressing: %w", err)
	}

	return string(decompressed), nil
}

// readUncompressedEntry reads from an uncompressed data file.
func (p *ZLDParser) readUncompressedEntry(path string, entry DictIndexEntry) (string, error) {
	file, err := os.Open(path)
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

	return string(data[:n]), nil
}

// isStrongsLexicon checks if this module is a Strong's lexicon.
func (p *ZLDParser) isStrongsLexicon() bool {
	id := strings.ToLower(p.module.ID)
	return strings.Contains(id, "strong") ||
		strings.Contains(id, "strongs")
}

// extractStrongsNumber extracts the Strong's number from a key.
func (p *ZLDParser) extractStrongsNumber(key string) string {
	key = strings.TrimSpace(key)

	// Handle formats like "H0430", "G2316", "0430", "2316"
	if len(key) > 0 {
		if key[0] == 'H' || key[0] == 'G' {
			return key
		}
		// Try to determine Hebrew vs Greek from module
		if strings.Contains(strings.ToLower(p.module.ID), "hebrew") {
			return "H" + key
		}
		if strings.Contains(strings.ToLower(p.module.ID), "greek") {
			return "G" + key
		}
	}

	return key
}

// GetAllKeys returns all keys in the dictionary.
func (p *ZLDParser) GetAllKeys() ([]string, error) {
	if err := p.loadIndices(); err != nil {
		return nil, fmt.Errorf("loading indices: %w", err)
	}

	return p.keys, nil
}

// GetAllEntries retrieves all dictionary entries.
func (p *ZLDParser) GetAllEntries() ([]*DictEntry, error) {
	if err := p.loadIndices(); err != nil {
		return nil, fmt.Errorf("loading indices: %w", err)
	}

	entries := make([]*DictEntry, 0, len(p.keys))
	for _, key := range p.keys {
		entry, err := p.GetEntry(key)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// SearchKeys searches for keys matching a pattern.
func (p *ZLDParser) SearchKeys(pattern string) ([]string, error) {
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
