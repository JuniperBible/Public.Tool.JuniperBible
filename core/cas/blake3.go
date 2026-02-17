package cas

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
	"github.com/zeebo/blake3"
)

// HashResult contains both SHA-256 and BLAKE3 hashes for a stored blob.
type HashResult struct {
	SHA256 string `json:"sha256"`
	BLAKE3 string `json:"blake3"`
}

// blake3Pointer is the structure stored in BLAKE3 pointer files.
type blake3Pointer struct {
	SHA256 string `json:"sha256"`
}

// StoreWithBlake3 stores the given data and returns both SHA-256 and BLAKE3 hashes.
// It creates a pointer file that maps the BLAKE3 hash to the SHA-256 hash.
func (s *Store) StoreWithBlake3(data []byte) (*HashResult, error) {
	// First, store using SHA-256 (the primary hash)
	sha256Hash, err := s.Store(data)
	if err != nil {
		return nil, err
	}

	// Calculate BLAKE3 hash
	b3 := blake3.Sum256(data)
	blake3Hash := hex.EncodeToString(b3[:])

	// Create the BLAKE3 pointer file
	if err := s.createBlake3Pointer(blake3Hash, sha256Hash); err != nil {
		return nil, fmt.Errorf("failed to create BLAKE3 pointer: %w", err)
	}

	return &HashResult{
		SHA256: sha256Hash,
		BLAKE3: blake3Hash,
	}, nil
}

// createBlake3Pointer creates a pointer file that maps a BLAKE3 hash to a SHA-256 hash.
// Pointer files are stored at: <root>/blobs/blake3/<first2>/<blake3>.json
func (s *Store) createBlake3Pointer(blake3Hash, sha256Hash string) error {
	prefix := blake3Hash[:2]
	pointerDir := filepath.Join(s.root, "blobs", "blake3", prefix)

	if err := os.MkdirAll(pointerDir, 0700); err != nil {
		return fmt.Errorf("failed to create blake3 directory: %w", err)
	}

	pointerPath := filepath.Join(pointerDir, blake3Hash+".json")

	// Check if pointer already exists
	if _, err := os.Stat(pointerPath); err == nil {
		return nil // Already exists
	}

	pointer := blake3Pointer{SHA256: sha256Hash}
	data, err := json.Marshal(pointer)
	if err != nil {
		return fmt.Errorf("failed to marshal pointer: %w", err)
	}

	// Write atomically
	tempFile, err := os.CreateTemp(pointerDir, ".pointer-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFileWrite(tempFile, data); err != nil {
		tempFileClose(tempFile)
		os.Remove(tempPath)
		return fmt.Errorf("failed to write pointer: %w", err)
	}

	if err := tempFileClose(tempFile); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := osRename(tempPath, pointerPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename pointer: %w", err)
	}

	return nil
}

// LookupBlake3 looks up a SHA-256 hash by its corresponding BLAKE3 hash.
// Returns ErrBlobNotFound if no pointer file exists for the BLAKE3 hash.
func (s *Store) LookupBlake3(blake3Hash string) (string, error) {
	if !isValidHash(blake3Hash) {
		return "", ErrInvalidHash
	}

	prefix := blake3Hash[:2]
	pointerPath := filepath.Join(s.root, "blobs", "blake3", prefix, blake3Hash+".json")

	data, err := safefile.ReadFile(pointerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrBlobNotFound
		}
		return "", fmt.Errorf("failed to read pointer: %w", err)
	}

	var pointer blake3Pointer
	if err := json.Unmarshal(data, &pointer); err != nil {
		return "", fmt.Errorf("failed to parse pointer: %w", err)
	}

	return pointer.SHA256, nil
}

// RetrieveByBlake3 retrieves a blob by its BLAKE3 hash.
// It first looks up the SHA-256 hash, then retrieves the blob.
func (s *Store) RetrieveByBlake3(blake3Hash string) ([]byte, error) {
	sha256Hash, err := s.LookupBlake3(blake3Hash)
	if err != nil {
		return nil, err
	}

	return s.Retrieve(sha256Hash)
}

// Blake3Hash computes the BLAKE3 hash of the given data without storing it.
func Blake3Hash(data []byte) string {
	h := blake3.Sum256(data)
	return hex.EncodeToString(h[:])
}
