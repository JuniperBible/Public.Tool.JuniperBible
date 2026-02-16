// rawgenbook.go implements RawGenBook format parsing for SWORD general book modules.
// RawGenBook uses a tree structure for organizing hierarchical content like confessions,
// catechisms, and other non-Bible documents.
//
// File structure:
// - .bdt - Tree key data (parent/child/sibling pointers + null-terminated name)
// - .idx - Data index (4-byte offset + 4-byte size per entry)
// - .dat - Raw content data
//
// Special value 0xFFFFFFFF indicates "no link" for tree pointers.
package swordpure

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// NoLink is the marker value indicating no tree link (parent/child/sibling)
const NoLink = int(-1)

// RawGenBookEntry represents a single general book entry.
type RawGenBookEntry struct {
	Key     string // The hierarchical key path (e.g., "/WCF/Chapter 1")
	Content string // The entry content
	Offset  uint32 // Offset in data file
	Size    uint32 // Size of entry data
}

// TreeKey represents a node in the general book tree structure.
type TreeKey struct {
	Name        string // The node name
	Parent      int    // Parent node index (-1 for root)
	FirstChild  int    // First child node index (-1 for none)
	NextSibling int    // Next sibling node index (-1 for none)
	Offset      uint32 // Data offset
	Size        uint32 // Data size
}

// RawGenBookDataEntry represents an entry in the data index.
type RawGenBookDataEntry struct {
	Offset uint32
	Size   uint32
}

// RawGenBookModuleInfo contains information about a RawGenBook module.
type RawGenBookModuleInfo struct {
	Name       string
	Type       string
	EntryCount int
}

// RawGenBookParser handles parsing of RawGenBook format SWORD general book modules.
type RawGenBookParser struct {
	modulePath string
	conf       *Conf
	entries    map[string]*RawGenBookEntry
	treeKeys   []TreeKey
}

// NewRawGenBookParser creates a new parser for a RawGenBook module.
func NewRawGenBookParser(modulePath string) (*RawGenBookParser, error) {
	return &RawGenBookParser{
		modulePath: modulePath,
		entries:    make(map[string]*RawGenBookEntry),
	}, nil
}

// parseRawGenBookTreeIndex parses a .bdt tree index file.
// Format: 12 bytes per entry (parent[4], firstChild[4], nextSibling[4]) + null-terminated name
func parseRawGenBookTreeIndex(data []byte) ([]TreeKey, error) {
	var keys []TreeKey
	pos := 0

	for pos < len(data) {
		// Need at least 12 bytes for parent/child/sibling pointers
		if pos+12 > len(data) {
			break
		}

		parent := binary.LittleEndian.Uint32(data[pos:])
		firstChild := binary.LittleEndian.Uint32(data[pos+4:])
		nextSibling := binary.LittleEndian.Uint32(data[pos+8:])
		pos += 12

		// Find null terminator for name
		nullPos := bytes.IndexByte(data[pos:], 0)
		if nullPos < 0 {
			break
		}

		name := string(data[pos : pos+nullPos])
		pos += nullPos + 1 // Skip past null terminator

		// Convert 0xFFFFFFFF to -1
		parentInt := int(int32(parent))
		firstChildInt := int(int32(firstChild))
		nextSiblingInt := int(int32(nextSibling))

		keys = append(keys, TreeKey{
			Name:        name,
			Parent:      parentInt,
			FirstChild:  firstChildInt,
			NextSibling: nextSiblingInt,
		})
	}

	return keys, nil
}

// parseRawGenBookDataIndex parses a .idx data index file.
// Format: 8 bytes per entry (4-byte offset + 4-byte size)
func parseRawGenBookDataIndex(data []byte) ([]RawGenBookDataEntry, error) {
	if len(data)%8 != 0 {
		return nil, fmt.Errorf("invalid data index size: %d", len(data))
	}

	count := len(data) / 8
	entries := make([]RawGenBookDataEntry, count)

	for i := 0; i < count; i++ {
		offset := i * 8
		entries[i] = RawGenBookDataEntry{
			Offset: binary.LittleEndian.Uint32(data[offset:]),
			Size:   binary.LittleEndian.Uint32(data[offset+4:]),
		}
	}

	return entries, nil
}

// GetEntry retrieves an entry by its key path.
func (p *RawGenBookParser) GetEntry(key string) (*RawGenBookEntry, error) {
	entry, ok := p.entries[key]
	if !ok {
		return nil, fmt.Errorf("entry not found: %s", key)
	}
	return entry, nil
}

// ListKeys returns all entry key paths.
func (p *RawGenBookParser) ListKeys() []string {
	keys := make([]string, 0, len(p.entries))
	for k := range p.entries {
		keys = append(keys, k)
	}
	return keys
}

// GetRoot returns the root node of the tree.
func (p *RawGenBookParser) GetRoot() *TreeKey {
	if len(p.treeKeys) == 0 {
		return nil
	}
	return &p.treeKeys[0]
}

// GetChildren returns all child nodes of the node at the given index.
func (p *RawGenBookParser) GetChildren(idx int) []*TreeKey {
	if idx < 0 || idx >= len(p.treeKeys) {
		return nil
	}

	var children []*TreeKey
	childIdx := p.treeKeys[idx].FirstChild

	for childIdx >= 0 && childIdx < len(p.treeKeys) {
		children = append(children, &p.treeKeys[childIdx])
		childIdx = p.treeKeys[childIdx].NextSibling
	}

	return children
}

// BuildKeyPath builds the full key path for the node at the given index.
func (p *RawGenBookParser) BuildKeyPath(idx int) string {
	if idx < 0 || idx >= len(p.treeKeys) {
		return ""
	}

	// Build path by walking up to root
	var parts []string
	currentIdx := idx

	for currentIdx >= 0 && currentIdx < len(p.treeKeys) {
		parts = append([]string{p.treeKeys[currentIdx].Name}, parts...)
		currentIdx = p.treeKeys[currentIdx].Parent
	}

	return "/" + joinPath(parts)
}

// joinPath joins path parts with "/"
func joinPath(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += "/" + parts[i]
	}
	return result
}

// ModuleInfo returns information about the general book module.
func (p *RawGenBookParser) ModuleInfo() RawGenBookModuleInfo {
	name := ""
	sourceType := ""
	if p.conf != nil {
		name = p.conf.ModuleName
		sourceType = p.conf.SourceType
	}
	return RawGenBookModuleInfo{
		Name:       name,
		Type:       sourceType,
		EntryCount: len(p.entries),
	}
}
