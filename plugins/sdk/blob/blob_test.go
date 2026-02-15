package blob

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore(t *testing.T) {
	tmpDir := t.TempDir()
	data := []byte("test content")

	hash, size, err := Store(tmpDir, data)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Check hash is valid
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}

	// Check size
	if size != int64(len(data)) {
		t.Errorf("size = %d, want %d", size, len(data))
	}

	// Verify blob was stored
	blobPath := filepath.Join(tmpDir, hash[:2], hash)
	stored, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("failed to read stored blob: %v", err)
	}
	if string(stored) != string(data) {
		t.Errorf("stored content = %q, want %q", stored, data)
	}
}

func TestRetrieve(t *testing.T) {
	tmpDir := t.TempDir()
	data := []byte("test content for retrieval")

	// Store first
	hash, _, err := Store(tmpDir, data)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Retrieve
	retrieved, err := Retrieve(tmpDir, hash)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}

	if string(retrieved) != string(data) {
		t.Errorf("retrieved content = %q, want %q", retrieved, data)
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	data := []byte("test content")

	hash, _, err := Store(tmpDir, data)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	if !Exists(tmpDir, hash) {
		t.Error("Exists() = false for stored blob")
	}

	if Exists(tmpDir, "nonexistent") {
		t.Error("Exists() = true for nonexistent blob")
	}
}

func TestPath(t *testing.T) {
	hash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	path := Path("/output", hash)
	expected := filepath.Join("/output", "e3", hash)

	if path != expected {
		t.Errorf("Path() = %q, want %q", path, expected)
	}
}

func TestHash(t *testing.T) {
	// Empty content has a well-known hash
	emptyHash := Hash([]byte{})
	expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	if emptyHash != expected {
		t.Errorf("Hash(empty) = %q, want %q", emptyHash, expected)
	}

	// Same content always produces same hash
	data := []byte("test content")
	hash1 := Hash(data)
	hash2 := Hash(data)

	if hash1 != hash2 {
		t.Errorf("Hash not deterministic: %q != %q", hash1, hash2)
	}
}

func TestHashFile(t *testing.T) {
	tmpDir := t.TempDir()
	data := []byte("file content for hashing")
	path := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fileHash, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile() error = %v", err)
	}

	dataHash := Hash(data)
	if fileHash != dataHash {
		t.Errorf("HashFile() = %q, want %q", fileHash, dataHash)
	}
}

func TestArtifactIDFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/KJV.json", "kjv"},
		{"/path/to/Sample-Bible.xml", "sample-bible"},
		{"MyBible.txt", "mybible"},
		{"file.TAR.GZ", "file.tar"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ArtifactIDFromPath(tt.path)
			if got != tt.expected {
				t.Errorf("ArtifactIDFromPath(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestArtifactIDFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"KJV.json", "kjv"},
		{"Sample-Bible.xml", "sample-bible"},
		{"MyBible.txt", "mybible"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := ArtifactIDFromFilename(tt.filename)
			if got != tt.expected {
				t.Errorf("ArtifactIDFromFilename(%q) = %q, want %q", tt.filename, got, tt.expected)
			}
		})
	}
}

func TestStoreFile(t *testing.T) {
	tmpDir := t.TempDir()
	data := []byte("file content")
	srcPath := filepath.Join(tmpDir, "source.txt")

	if err := os.WriteFile(srcPath, data, 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	hash, size, err := StoreFile(outputDir, srcPath)
	if err != nil {
		t.Fatalf("StoreFile() error = %v", err)
	}

	if size != int64(len(data)) {
		t.Errorf("size = %d, want %d", size, len(data))
	}

	// Verify blob was stored
	if !Exists(outputDir, hash) {
		t.Error("blob not found after StoreFile()")
	}
}
