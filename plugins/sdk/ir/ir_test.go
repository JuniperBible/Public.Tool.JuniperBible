package ir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewCorpus(t *testing.T) {
	corpus := NewCorpus("kjv", "bible", "en")

	if corpus.ID != "kjv" {
		t.Errorf("ID = %q, want %q", corpus.ID, "kjv")
	}
	if corpus.ModuleType != "bible" {
		t.Errorf("ModuleType = %q, want %q", corpus.ModuleType, "bible")
	}
	if corpus.Language != "en" {
		t.Errorf("Language = %q, want %q", corpus.Language, "en")
	}
	if corpus.Version != "1.0" {
		t.Errorf("Version = %q, want %q", corpus.Version, "1.0")
	}
	if corpus.Documents == nil {
		t.Error("Documents is nil")
	}
}

func TestNewDocument(t *testing.T) {
	doc := NewDocument("Gen", "Genesis", 1)

	if doc.ID != "Gen" {
		t.Errorf("ID = %q, want %q", doc.ID, "Gen")
	}
	if doc.Title != "Genesis" {
		t.Errorf("Title = %q, want %q", doc.Title, "Genesis")
	}
	if doc.Order != 1 {
		t.Errorf("Order = %d, want %d", doc.Order, 1)
	}
}

func TestNewContentBlock(t *testing.T) {
	cb := NewContentBlock("Gen.1.1", 1, "In the beginning...")

	if cb.ID != "Gen.1.1" {
		t.Errorf("ID = %q, want %q", cb.ID, "Gen.1.1")
	}
	if cb.Sequence != 1 {
		t.Errorf("Sequence = %d, want %d", cb.Sequence, 1)
	}
	if cb.Text != "In the beginning..." {
		t.Errorf("Text = %q, want %q", cb.Text, "In the beginning...")
	}
}

func TestAddDocument(t *testing.T) {
	corpus := NewCorpus("test", "bible", "en")
	doc := NewDocument("Gen", "Genesis", 1)

	AddDocument(corpus, doc)

	if len(corpus.Documents) != 1 {
		t.Errorf("len(Documents) = %d, want 1", len(corpus.Documents))
	}
	if corpus.Documents[0].ID != "Gen" {
		t.Errorf("Documents[0].ID = %q, want %q", corpus.Documents[0].ID, "Gen")
	}
}

func TestAddContentBlock(t *testing.T) {
	doc := NewDocument("Gen", "Genesis", 1)
	cb := NewContentBlock("Gen.1.1", 1, "In the beginning...")

	AddContentBlock(doc, cb)

	if len(doc.ContentBlocks) != 1 {
		t.Errorf("len(ContentBlocks) = %d, want 1", len(doc.ContentBlocks))
	}
}

func TestWriteAndRead(t *testing.T) {
	tmpDir := t.TempDir()

	// Create corpus
	corpus := NewCorpus("test", "bible", "en")
	corpus.Title = "Test Bible"
	doc := NewDocument("Gen", "Genesis", 1)
	AddContentBlock(doc, NewContentBlock("Gen.1.1", 1, "In the beginning..."))
	AddDocument(corpus, doc)

	// Write
	path, err := Write(corpus, tmpDir)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "test.json")
	if path != expectedPath {
		t.Errorf("Write() path = %q, want %q", path, expectedPath)
	}

	// Read back
	loaded, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if loaded.ID != corpus.ID {
		t.Errorf("loaded.ID = %q, want %q", loaded.ID, corpus.ID)
	}
	if loaded.Title != corpus.Title {
		t.Errorf("loaded.Title = %q, want %q", loaded.Title, corpus.Title)
	}
	if len(loaded.Documents) != 1 {
		t.Errorf("len(loaded.Documents) = %d, want 1", len(loaded.Documents))
	}
}

func TestWriteCompact(t *testing.T) {
	tmpDir := t.TempDir()

	corpus := NewCorpus("test", "bible", "en")

	path, err := WriteCompact(corpus, tmpDir)
	if err != nil {
		t.Fatalf("WriteCompact() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}

	// Verify it's compact (no indentation)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	// Compact JSON should not have newlines within the object
	// (except possibly trailing newline)
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	for _, b := range data {
		if b == '\n' {
			t.Error("WriteCompact() output contains newlines")
			break
		}
	}
}

func TestHash(t *testing.T) {
	corpus := NewCorpus("test", "bible", "en")

	hash1, err := Hash(corpus)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	if len(hash1) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash1))
	}

	// Same corpus should produce same hash
	hash2, _ := Hash(corpus)
	if hash1 != hash2 {
		t.Errorf("Hash not deterministic: %q != %q", hash1, hash2)
	}

	// Different corpus should produce different hash
	corpus.ID = "different"
	hash3, _ := Hash(corpus)
	if hash1 == hash3 {
		t.Error("Different corpus produced same hash")
	}
}

func TestHashContentBlocks(t *testing.T) {
	corpus := NewCorpus("test", "bible", "en")
	doc := NewDocument("Gen", "Genesis", 1)
	AddContentBlock(doc, NewContentBlock("Gen.1.1", 1, "In the beginning..."))
	AddDocument(corpus, doc)

	hash1 := HashContentBlocks(corpus)

	if len(hash1) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash1))
	}

	// Changing metadata should not change content hash
	corpus.Title = "New Title"
	hash2 := HashContentBlocks(corpus)
	if hash1 != hash2 {
		t.Error("Metadata change affected content hash")
	}

	// Changing content should change hash
	corpus.Documents[0].ContentBlocks[0].Text = "Different text"
	hash3 := HashContentBlocks(corpus)
	if hash1 == hash3 {
		t.Error("Content change did not affect content hash")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		corpus  *Corpus
		wantErr bool
	}{
		{
			name:    "nil corpus",
			corpus:  nil,
			wantErr: true,
		},
		{
			name:    "empty ID",
			corpus:  &Corpus{ModuleType: "bible"},
			wantErr: true,
		},
		{
			name:    "empty module type",
			corpus:  &Corpus{ID: "test"},
			wantErr: true,
		},
		{
			name:    "valid corpus",
			corpus:  NewCorpus("test", "bible", "en"),
			wantErr: false,
		},
		{
			name: "document without ID",
			corpus: &Corpus{
				ID:         "test",
				ModuleType: "bible",
				Documents:  []*Document{{Title: "Genesis"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.corpus)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCountVerses(t *testing.T) {
	corpus := NewCorpus("test", "bible", "en")

	if CountVerses(corpus) != 0 {
		t.Errorf("CountVerses(empty) = %d, want 0", CountVerses(corpus))
	}

	doc := NewDocument("Gen", "Genesis", 1)
	AddContentBlock(doc, NewContentBlock("Gen.1.1", 1, "Verse 1"))
	AddContentBlock(doc, NewContentBlock("Gen.1.2", 2, "Verse 2"))
	AddDocument(corpus, doc)

	if CountVerses(corpus) != 2 {
		t.Errorf("CountVerses() = %d, want 2", CountVerses(corpus))
	}
}

func TestCountDocuments(t *testing.T) {
	corpus := NewCorpus("test", "bible", "en")

	if CountDocuments(corpus) != 0 {
		t.Errorf("CountDocuments(empty) = %d, want 0", CountDocuments(corpus))
	}

	AddDocument(corpus, NewDocument("Gen", "Genesis", 1))
	AddDocument(corpus, NewDocument("Exo", "Exodus", 2))

	if CountDocuments(corpus) != 2 {
		t.Errorf("CountDocuments() = %d, want 2", CountDocuments(corpus))
	}
}
