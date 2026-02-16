// zld.go implements zLD format parsing for SWORD lexicon/dictionary modules.
// zLD uses compressed data storage with key-based indexing for dictionary entries.
//
// File structure:
// - .idx - Key index (4-byte big-endian offset + null-terminated key string)
// - .dat - Optional key data
// - .zdx - Compressed index (8 bytes per entry: block[4] + offset[4])
// - .zdt - Compressed text data (zlib compressed blocks)
package swordpure

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// ZLDEntry represents a single lexicon/dictionary entry.
type ZLDEntry struct {
	Key        string // The dictionary key (e.g., "G2316")
	Definition string // The definition/content
	Offset     uint32 // Offset in data file
	Size       uint32 // Size of entry data
}

// ZLDIndexEntry represents an entry in the compressed index.
type ZLDIndexEntry struct {
	BlockNum uint32 // Which compressed block contains this entry
	Offset   uint32 // Offset within the decompressed block
}

// Conf represents module configuration for zLD modules.
// This is a simplified version of ConfFile for lexicon modules.
type Conf struct {
	ModuleName  string
	Description string
	Lang        string
	Version     string
	SourceType  string
}

// ZLDModuleInfo contains information about a zLD module.
type ZLDModuleInfo struct {
	Name       string
	Type       string
	EntryCount int
}

// ZLDParser handles parsing of zLD format SWORD lexicon/dictionary modules.
type ZLDParser struct {
	modulePath string
	conf       *Conf
	entries    map[string]*ZLDEntry
}

// NewZLDParser creates a new parser for a zLD lexicon module.
func NewZLDParser(modulePath string) (*ZLDParser, error) {
	return &ZLDParser{
		modulePath: modulePath,
		entries:    make(map[string]*ZLDEntry),
	}, nil
}

// parseZLDKeyIndex parses a .idx key index file.
// Format: 4-byte big-endian offset + null-terminated key string
func parseZLDKeyIndex(data []byte) ([]*ZLDEntry, error) {
	var entries []*ZLDEntry
	pos := 0

	for pos < len(data) {
		// Need at least 4 bytes for offset
		if pos+4 > len(data) {
			break
		}

		// Read 4-byte big-endian offset
		offset := binary.BigEndian.Uint32(data[pos:])
		pos += 4

		// Find null terminator
		nullPos := bytes.IndexByte(data[pos:], 0)
		if nullPos < 0 {
			break
		}

		key := string(data[pos : pos+nullPos])
		pos += nullPos + 1 // Skip past null terminator

		entries = append(entries, &ZLDEntry{
			Key:    key,
			Offset: offset,
		})
	}

	return entries, nil
}

// parseZLDCompressedIndex parses a .zdx compressed index file.
// Format: 8 bytes per entry (4-byte block number + 4-byte offset within block)
func parseZLDCompressedIndex(data []byte) ([]ZLDIndexEntry, error) {
	if len(data)%8 != 0 {
		return nil, fmt.Errorf("invalid compressed index size: %d", len(data))
	}

	count := len(data) / 8
	entries := make([]ZLDIndexEntry, count)

	for i := 0; i < count; i++ {
		offset := i * 8
		entries[i] = ZLDIndexEntry{
			BlockNum: binary.LittleEndian.Uint32(data[offset:]),
			Offset:   binary.LittleEndian.Uint32(data[offset+4:]),
		}
	}

	return entries, nil
}

// decompressZLDBlock decompresses a zLD data block.
// Format: 4-byte little-endian size + zlib compressed data
func decompressZLDBlock(data []byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("block data too short")
	}

	// Read 4-byte compressed size
	compressedSize := binary.LittleEndian.Uint32(data[:4])

	if len(data) < int(4+compressedSize) {
		return nil, fmt.Errorf("block data truncated")
	}

	// Decompress using zlib
	compressed := data[4 : 4+compressedSize]
	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("zlib init failed: %w", err)
	}
	defer reader.Close()

	var decompressed bytes.Buffer
	if _, err := io.Copy(&decompressed, reader); err != nil {
		return nil, fmt.Errorf("decompression failed: %w", err)
	}

	return decompressed.Bytes(), nil
}

// GetEntry retrieves a dictionary entry by key.
func (p *ZLDParser) GetEntry(key string) (*ZLDEntry, error) {
	entry, ok := p.entries[key]
	if !ok {
		return nil, fmt.Errorf("entry not found: %s", key)
	}
	return entry, nil
}

// ListKeys returns all dictionary keys.
func (p *ZLDParser) ListKeys() []string {
	keys := make([]string, 0, len(p.entries))
	for k := range p.entries {
		keys = append(keys, k)
	}
	return keys
}

// SearchKeys returns keys matching the given prefix.
func (p *ZLDParser) SearchKeys(prefix string) []string {
	var matches []string
	for k := range p.entries {
		if strings.HasPrefix(k, prefix) {
			matches = append(matches, k)
		}
	}
	return matches
}

// ModuleInfo returns information about the lexicon module.
func (p *ZLDParser) ModuleInfo() ZLDModuleInfo {
	name := ""
	sourceType := ""
	if p.conf != nil {
		name = p.conf.ModuleName
		sourceType = p.conf.SourceType
	}
	return ZLDModuleInfo{
		Name:       name,
		Type:       sourceType,
		EntryCount: len(p.entries),
	}
}
