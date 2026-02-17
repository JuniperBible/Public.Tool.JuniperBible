// rawgenbook_writer.go implements RawGenBook format writing for SWORD general book modules.
// This enables round-trip conversion: SWORD -> IR -> SWORD for general books.
//
// RawGenBook format:
// - .bdt - Tree key data (12 bytes per entry: parent[4], firstChild[4], nextSibling[4] + null-terminated name)
// - .idx - Data index (8 bytes per entry: offset[4] + size[4])
// - .dat - Raw content data
package swordpure

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RawGenBookWriter writes RawGenBook format SWORD general book modules.
type RawGenBookWriter struct {
	dataPath string

	// Tree nodes to write
	nodes []rawGenBookNode
}

type rawGenBookNode struct {
	Name        string // Node name (without path)
	Path        string // Full path (e.g., "/WCF/Chapter 1")
	Content     string // Content for this node
	Parent      int    // Parent node index (-1 for root)
	FirstChild  int    // First child node index (-1 for none)
	NextSibling int    // Next sibling node index (-1 for none)
}

// NewRawGenBookWriter creates a new RawGenBook writer for the given data path.
func NewRawGenBookWriter(dataPath string) *RawGenBookWriter {
	return &RawGenBookWriter{
		dataPath: dataPath,
	}
}

// AddEntry adds an entry with the given hierarchical path and content.
// Path should be like "/WCF/Chapter 1/Article 1".
func (w *RawGenBookWriter) AddEntry(path, content string) {
	w.nodes = append(w.nodes, rawGenBookNode{
		Path:        path,
		Content:     content,
		Parent:      -1,
		FirstChild:  -1,
		NextSibling: -1,
	})
}

// WriteModule writes the complete RawGenBook module.
// Returns the number of entries written.
func (w *RawGenBookWriter) WriteModule() (int, error) {
	if len(w.nodes) == 0 {
		return 0, nil
	}

	// Create data directory
	if err := os.MkdirAll(w.dataPath, 0700); err != nil {
		return 0, fmt.Errorf("failed to create data path: %w", err)
	}

	// Build tree structure from flat paths
	if err := w.buildTree(); err != nil {
		return 0, err
	}

	// Write files
	if err := w.writeFiles(); err != nil {
		return 0, err
	}

	return len(w.nodes), nil
}

// buildTree builds the tree structure (parent/child/sibling links) from flat paths.
func (w *RawGenBookWriter) buildTree() error {
	// Sort nodes by path for consistent ordering
	sort.Slice(w.nodes, func(i, j int) bool {
		return w.nodes[i].Path < w.nodes[j].Path
	})

	// Extract names from paths
	for i := range w.nodes {
		parts := strings.Split(strings.TrimPrefix(w.nodes[i].Path, "/"), "/")
		if len(parts) > 0 {
			w.nodes[i].Name = parts[len(parts)-1]
		}
	}

	// Build path -> index map
	pathIndex := make(map[string]int)
	for i, node := range w.nodes {
		pathIndex[node.Path] = i
	}

	// Set parent links
	for i := range w.nodes {
		parentPath := getParentPath(w.nodes[i].Path)
		if parentPath != "" {
			if parentIdx, ok := pathIndex[parentPath]; ok {
				w.nodes[i].Parent = parentIdx
			}
		}
	}

	// Set child and sibling links
	for i := range w.nodes {
		// Find first child
		for j := range w.nodes {
			if w.nodes[j].Parent == i {
				w.nodes[i].FirstChild = j
				break
			}
		}
	}

	// Set sibling links (nodes with same parent)
	for i := range w.nodes {
		parentIdx := w.nodes[i].Parent
		// Find next sibling (next node with same parent, after this one)
		for j := i + 1; j < len(w.nodes); j++ {
			if w.nodes[j].Parent == parentIdx {
				w.nodes[i].NextSibling = j
				break
			}
		}
	}

	return nil
}

// getParentPath returns the parent path of the given path.
func getParentPath(path string) string {
	path = strings.TrimPrefix(path, "/")
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash < 0 {
		return ""
	}
	return "/" + path[:lastSlash]
}

// writeFiles writes the .bdt, .idx, and .dat files.
func (w *RawGenBookWriter) writeFiles() error {
	// Build buffers for all three files
	var bdtBuf bytes.Buffer // Tree key data
	var idxBuf bytes.Buffer // Data index
	var datBuf bytes.Buffer // Raw content

	for _, node := range w.nodes {
		// Write .bdt entry: parent[4], firstChild[4], nextSibling[4] + null-terminated name
		writeInt32(&bdtBuf, int32(node.Parent))
		writeInt32(&bdtBuf, int32(node.FirstChild))
		writeInt32(&bdtBuf, int32(node.NextSibling))
		bdtBuf.WriteString(node.Name)
		bdtBuf.WriteByte(0)

		// Write .idx entry: offset[4] + size[4]
		offset := uint32(datBuf.Len())
		size := uint32(len(node.Content))
		writeUint32(&idxBuf, offset)
		writeUint32(&idxBuf, size)

		// Write .dat entry: raw content
		datBuf.WriteString(node.Content)
	}

	// Write all files
	bdtPath := filepath.Join(w.dataPath, "book.bdt")
	if err := os.WriteFile(bdtPath, bdtBuf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write bdt: %w", err)
	}

	idxPath := filepath.Join(w.dataPath, "book.idx")
	if err := os.WriteFile(idxPath, idxBuf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write idx: %w", err)
	}

	datPath := filepath.Join(w.dataPath, "book.dat")
	if err := os.WriteFile(datPath, datBuf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write dat: %w", err)
	}

	return nil
}

// writeInt32 writes a 32-bit signed integer in little-endian format.
func writeInt32(buf *bytes.Buffer, v int32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(v))
	buf.Write(b)
}

// writeUint32 writes a 32-bit unsigned integer in little-endian format.
func writeUint32(buf *bytes.Buffer, v uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	buf.Write(b)
}

// EmitRawGenBook writes a complete SWORD general book module from IR corpus.
// Creates mods.d/*.conf and modules/genbook/rawgenbook/*/ structure.
func EmitRawGenBook(corpus *IRCorpus, outputDir string) (*EmitResult, error) {
	result := &EmitResult{
		ModuleID: corpus.ID,
	}

	// Create directory structure
	modsDir := filepath.Join(outputDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create mods.d: %w", err)
	}

	dataPath := filepath.Join(outputDir, "modules", "genbook", "rawgenbook", stringToLower(corpus.ID))
	if err := os.MkdirAll(dataPath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data path: %w", err)
	}

	// Write RawGenBook data
	writer := NewRawGenBookWriter(dataPath)

	// Add entries from corpus
	for _, doc := range corpus.Documents {
		for _, block := range doc.ContentBlocks {
			// Use block ID as path, text as content
			path := block.ID
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			text := block.RawMarkup
			if text == "" {
				text = block.Text
			}
			writer.AddEntry(path, text)
		}
	}

	entriesWritten, err := writer.WriteModule()
	if err != nil {
		return nil, fmt.Errorf("failed to write RawGenBook: %w", err)
	}
	result.VersesWritten = entriesWritten

	// Generate and write .conf file
	confContent := generateGenBookConf(corpus)
	confPath := filepath.Join(modsDir, stringToLower(corpus.ID)+".conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		return nil, fmt.Errorf("failed to write conf: %w", err)
	}
	result.ConfPath = confPath
	result.DataPath = dataPath

	return result, nil
}

// generateGenBookConf generates a SWORD .conf file for a general book module.
func generateGenBookConf(corpus *IRCorpus) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("[%s]\n", corpus.ID))
	buf.WriteString(fmt.Sprintf("Description=%s\n", corpus.Title))
	buf.WriteString(fmt.Sprintf("Lang=%s\n", corpus.Language))
	buf.WriteString("ModDrv=RawGenBook\n")
	buf.WriteString("Encoding=UTF-8\n")
	buf.WriteString(fmt.Sprintf("DataPath=./modules/genbook/rawgenbook/%s/book\n", stringToLower(corpus.ID)))

	return buf.String()
}
