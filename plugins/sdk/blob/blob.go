// Package blob provides content-addressed storage helpers for SDK plugins.
package blob

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Store stores data in content-addressed storage and returns the SHA-256 hash.
// The blob is stored at outputDir/hash[:2]/hash.
func Store(outputDir string, data []byte) (hash string, size int64, err error) {
	// Calculate SHA-256
	h := sha256.Sum256(data)
	hash = hex.EncodeToString(h[:])

	// Create directory structure (hash[:2]/)
	blobDir := filepath.Join(outputDir, hash[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create blob directory: %w", err)
	}

	// Write blob
	blobPath := filepath.Join(blobDir, hash)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		return "", 0, fmt.Errorf("failed to write blob: %w", err)
	}

	return hash, int64(len(data)), nil
}

// StoreFile stores a file in content-addressed storage.
// Returns the SHA-256 hash and size.
func StoreFile(outputDir, srcPath string) (hash string, size int64, err error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read source file: %w", err)
	}
	return Store(outputDir, data)
}

// StoreReader stores data from a reader in content-addressed storage.
func StoreReader(outputDir string, r io.Reader) (hash string, size int64, err error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read data: %w", err)
	}
	return Store(outputDir, data)
}

// Retrieve retrieves a blob by its hash from content-addressed storage.
func Retrieve(outputDir, hash string) ([]byte, error) {
	if len(hash) < 2 {
		return nil, fmt.Errorf("invalid hash: too short")
	}
	blobPath := filepath.Join(outputDir, hash[:2], hash)
	data, err := os.ReadFile(blobPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read blob: %w", err)
	}
	return data, nil
}

// Exists checks if a blob exists in content-addressed storage.
func Exists(outputDir, hash string) bool {
	if len(hash) < 2 {
		return false
	}
	blobPath := filepath.Join(outputDir, hash[:2], hash)
	_, err := os.Stat(blobPath)
	return err == nil
}

// Path returns the path where a blob would be stored.
func Path(outputDir, hash string) string {
	if len(hash) < 2 {
		return ""
	}
	return filepath.Join(outputDir, hash[:2], hash)
}

// Hash computes the SHA-256 hash of data without storing it.
func Hash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// HashFile computes the SHA-256 hash of a file.
func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return Hash(data), nil
}

// HashReader computes the SHA-256 hash from a reader.
func HashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ArtifactIDFromPath derives an artifact ID from a file path.
// It strips the extension and converts to lowercase.
func ArtifactIDFromPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return strings.ToLower(name)
}

// ArtifactIDFromFilename derives an artifact ID from a filename.
func ArtifactIDFromFilename(filename string) string {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	return strings.ToLower(name)
}
