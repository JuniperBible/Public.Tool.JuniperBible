// Package cas provides content-addressed storage for blobs.
// All blobs are stored by their SHA-256 hash, ensuring deduplication
// and enabling verification of content integrity.
package cas

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// osRename is a variable to allow testing of rename errors.
var osRename = os.Rename

// tempFileWrite is a function variable for writing to temp files (for testing).
var tempFileWrite = func(f *os.File, data []byte) (int, error) {
	return f.Write(data)
}

// tempFileClose is a function variable for closing temp files (for testing).
var tempFileClose = func(f io.Closer) error {
	return f.Close()
}

// ErrBlobNotFound is returned when a blob with the given hash does not exist.
var ErrBlobNotFound = errors.New("blob not found")

// ErrInvalidHash is returned when a hash string is not a valid SHA-256 hex string.
var ErrInvalidHash = errors.New("invalid hash format")

// sha256Pattern matches a valid lowercase SHA-256 hex string (64 characters).
var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Store provides content-addressed storage for blobs using SHA-256 hashing.
type Store struct {
	root string
}

// NewStore creates a new content-addressed store at the given root directory.
// The directory structure will be created if it doesn't exist.
func NewStore(root string) (*Store, error) {
	// Create the blobs/sha256 directory structure
	blobDir := filepath.Join(root, "blobs", "sha256")
	if err := os.MkdirAll(blobDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create blob directory: %w", err)
	}

	return &Store{root: root}, nil
}

// Store stores the given data and returns its SHA-256 hash.
// If the blob already exists (same hash), this is a no-op and returns the hash.
func (s *Store) Store(data []byte) (string, error) {
	// Calculate SHA-256 hash
	h := sha256.Sum256(data)
	hash := hex.EncodeToString(h[:])

	// Check if blob already exists (deduplication)
	blobPath := s.pathForHash(hash)
	if _, err := os.Stat(blobPath); err == nil {
		// Blob already exists, return hash
		return hash, nil
	}

	// Create the prefix directory if needed
	prefixDir := filepath.Dir(blobPath)
	if err := os.MkdirAll(prefixDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create prefix directory: %w", err)
	}

	// Write the blob atomically using a temp file
	tempFile, err := os.CreateTemp(prefixDir, ".blob-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Write data
	if _, err := tempFileWrite(tempFile, data); err != nil {
		tempFileClose(tempFile)
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to write blob: %w", err)
	}

	if err := tempFileClose(tempFile); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	// Rename to final path (atomic on POSIX)
	if err := osRename(tempPath, blobPath); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to rename blob: %w", err)
	}

	return hash, nil
}

// Retrieve retrieves the blob with the given SHA-256 hash.
// Returns ErrBlobNotFound if the blob does not exist.
// Returns ErrInvalidHash if the hash format is invalid.
func (s *Store) Retrieve(hash string) ([]byte, error) {
	// Validate hash format
	if !isValidHash(hash) {
		return nil, ErrInvalidHash
	}

	blobPath := s.pathForHash(hash)
	data, err := os.ReadFile(blobPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrBlobNotFound
		}
		return nil, fmt.Errorf("failed to read blob: %w", err)
	}

	return data, nil
}

// Exists checks if a blob with the given hash exists in the store.
func (s *Store) Exists(hash string) bool {
	if !isValidHash(hash) {
		return false
	}
	blobPath := s.pathForHash(hash)
	_, err := os.Stat(blobPath)
	return err == nil
}

// pathForHash returns the file path for a blob with the given hash.
// Blobs are stored at: <root>/blobs/sha256/<first2>/<hash>
func (s *Store) pathForHash(hash string) string {
	prefix := hash[:2]
	return filepath.Join(s.root, "blobs", "sha256", prefix, hash)
}

// isValidHash checks if a hash string is a valid SHA-256 hex string.
func isValidHash(hash string) bool {
	return sha256Pattern.MatchString(hash)
}

// Hash computes the SHA-256 hash of the given data without storing it.
func Hash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
