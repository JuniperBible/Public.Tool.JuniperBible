// Package ir provides IR (Intermediate Representation) read/write helpers for SDK plugins.
package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// Corpus type alias for convenience
type Corpus = ipc.Corpus
type Document = ipc.Document
type ContentBlock = ipc.ContentBlock
type Token = ipc.Token
type Anchor = ipc.Anchor
type Span = ipc.Span
type Ref = ipc.Ref

// Read reads a Corpus from a JSON file.
func Read(path string) (*Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read IR file: %w", err)
	}

	var corpus Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return nil, fmt.Errorf("failed to parse IR: %w", err)
	}

	return &corpus, nil
}

// Write writes a Corpus to a JSON file.
// Returns the path to the written file.
func Write(corpus *Corpus, outputDir string) (string, error) {
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Determine filename
	filename := "corpus.json"
	if corpus.ID != "" {
		filename = corpus.ID + ".json"
	}
	path := filepath.Join(outputDir, filename)

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal IR: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write IR file: %w", err)
	}

	return path, nil
}

// WriteCompact writes a Corpus to a JSON file without indentation.
// More compact but less readable.
func WriteCompact(corpus *Corpus, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := "corpus.json"
	if corpus.ID != "" {
		filename = corpus.ID + ".json"
	}
	path := filepath.Join(outputDir, filename)

	data, err := json.Marshal(corpus)
	if err != nil {
		return "", fmt.Errorf("failed to marshal IR: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write IR file: %w", err)
	}

	return path, nil
}

// Hash computes the SHA-256 hash of a Corpus.
// This is used to track content changes across conversions.
func Hash(corpus *Corpus) (string, error) {
	data, err := json.Marshal(corpus)
	if err != nil {
		return "", fmt.Errorf("failed to marshal corpus for hashing: %w", err)
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// HashContentBlocks computes a hash of all content blocks in a corpus.
// This provides a content-only hash that ignores metadata changes.
func HashContentBlocks(corpus *Corpus) string {
	h := sha256.New()
	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			h.Write([]byte(cb.ID))
			h.Write([]byte(cb.Text))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Validate performs basic validation on a Corpus.
func Validate(corpus *Corpus) error {
	if corpus == nil {
		return fmt.Errorf("corpus is nil")
	}
	if corpus.ID == "" {
		return fmt.Errorf("corpus ID is required")
	}
	if corpus.ModuleType == "" {
		return fmt.Errorf("corpus module_type is required")
	}

	// Validate documents
	for i, doc := range corpus.Documents {
		if doc.ID == "" {
			return fmt.Errorf("document %d: ID is required", i)
		}
	}

	return nil
}

// NewCorpus creates a new Corpus with required fields.
func NewCorpus(id, moduleType, language string) *Corpus {
	return &Corpus{
		ID:         id,
		Version:    "1.0",
		ModuleType: moduleType,
		Language:   language,
		Documents:  []*Document{},
	}
}

// NewDocument creates a new Document.
func NewDocument(id, title string, order int) *Document {
	return &Document{
		ID:            id,
		Title:         title,
		Order:         order,
		ContentBlocks: []*ContentBlock{},
	}
}

// NewContentBlock creates a new ContentBlock.
func NewContentBlock(id string, sequence int, text string) *ContentBlock {
	return &ContentBlock{
		ID:       id,
		Sequence: sequence,
		Text:     text,
	}
}

// AddDocument adds a document to a corpus.
func AddDocument(corpus *Corpus, doc *Document) {
	corpus.Documents = append(corpus.Documents, doc)
}

// AddContentBlock adds a content block to a document.
func AddContentBlock(doc *Document, cb *ContentBlock) {
	doc.ContentBlocks = append(doc.ContentBlocks, cb)
}

// CountVerses counts total content blocks (verses) in a corpus.
func CountVerses(corpus *Corpus) int {
	count := 0
	for _, doc := range corpus.Documents {
		count += len(doc.ContentBlocks)
	}
	return count
}

// CountDocuments returns the number of documents in a corpus.
func CountDocuments(corpus *Corpus) int {
	return len(corpus.Documents)
}
