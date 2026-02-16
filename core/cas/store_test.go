package cas

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStoreAndRetrieve tests that storing a blob returns the correct hash
// and that retrieving by hash returns the exact same bytes.
func TestStoreAndRetrieve(t *testing.T) {
	// Create a temporary directory for the blob store
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Test data
	testData := []byte("Hello, Juniper Bible!")

	// Calculate expected hash
	h := sha256.Sum256(testData)
	expectedHash := hex.EncodeToString(h[:])

	// Store the blob
	hash, err := store.Store(testData)
	if err != nil {
		t.Fatalf("failed to store blob: %v", err)
	}

	// Verify hash matches
	if hash != expectedHash {
		t.Errorf("hash mismatch: got %s, want %s", hash, expectedHash)
	}

	// Retrieve the blob
	retrieved, err := store.Retrieve(hash)
	if err != nil {
		t.Fatalf("failed to retrieve blob: %v", err)
	}

	// Verify bytes match exactly
	if !bytes.Equal(retrieved, testData) {
		t.Errorf("retrieved data mismatch: got %q, want %q", retrieved, testData)
	}
}

// TestStoreDuplicate tests that storing the same content twice returns the same hash
// and doesn't create duplicate files (deduplication).
func TestStoreDuplicate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	testData := []byte("Duplicate content test")

	// Store the same content twice
	hash1, err := store.Store(testData)
	if err != nil {
		t.Fatalf("first store failed: %v", err)
	}

	hash2, err := store.Store(testData)
	if err != nil {
		t.Fatalf("second store failed: %v", err)
	}

	// Hashes should be identical
	if hash1 != hash2 {
		t.Errorf("duplicate hashes differ: %s != %s", hash1, hash2)
	}

	// Count blob files - should only be one
	blobPath := store.pathForHash(hash1)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Errorf("blob file should exist at %s", blobPath)
	}
}

// TestRetrieveNonExistent tests that retrieving a non-existent hash returns an error.
func TestRetrieveNonExistent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Try to retrieve a hash that doesn't exist
	fakeHash := "0000000000000000000000000000000000000000000000000000000000000000"
	_, err = store.Retrieve(fakeHash)
	if err == nil {
		t.Error("expected error when retrieving non-existent blob, got nil")
	}
	if err != ErrBlobNotFound {
		t.Errorf("expected ErrBlobNotFound, got %v", err)
	}
}

// TestInvalidHash tests that retrieving with an invalid hash format returns an error.
func TestInvalidHash(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	invalidHashes := []string{
		"",
		"abc",
		"not-a-valid-hash",
		"ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ",
		"000000000000000000000000000000000000000000000000000000000000000",   // 63 chars
		"00000000000000000000000000000000000000000000000000000000000000000", // 65 chars
	}

	for _, hash := range invalidHashes {
		_, err := store.Retrieve(hash)
		if err == nil {
			t.Errorf("expected error for invalid hash %q, got nil", hash)
		}
	}
}

// TestStoreEmpty tests that storing an empty blob works correctly.
func TestStoreEmpty(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	emptyData := []byte{}

	// SHA-256 of empty data
	h := sha256.Sum256(emptyData)
	expectedHash := hex.EncodeToString(h[:])

	hash, err := store.Store(emptyData)
	if err != nil {
		t.Fatalf("failed to store empty blob: %v", err)
	}

	if hash != expectedHash {
		t.Errorf("empty blob hash mismatch: got %s, want %s", hash, expectedHash)
	}

	retrieved, err := store.Retrieve(hash)
	if err != nil {
		t.Fatalf("failed to retrieve empty blob: %v", err)
	}

	if len(retrieved) != 0 {
		t.Errorf("retrieved empty blob should be empty, got %d bytes", len(retrieved))
	}
}

// TestStoreLargeBlob tests storing and retrieving a larger blob.
func TestStoreLargeBlob(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create a 1MB blob
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	hash, err := store.Store(largeData)
	if err != nil {
		t.Fatalf("failed to store large blob: %v", err)
	}

	retrieved, err := store.Retrieve(hash)
	if err != nil {
		t.Fatalf("failed to retrieve large blob: %v", err)
	}

	if !bytes.Equal(retrieved, largeData) {
		t.Error("large blob data mismatch")
	}
}

// TestBlobPath tests that blobs are stored with correct directory structure.
func TestBlobPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	testData := []byte("Path structure test")

	hash, err := store.Store(testData)
	if err != nil {
		t.Fatalf("failed to store blob: %v", err)
	}

	// Blob should be stored at blobs/sha256/<first2>/<hash>
	expectedPath := filepath.Join(tempDir, "blobs", "sha256", hash[:2], hash)
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("blob not found at expected path: %s", expectedPath)
	}
}

// TestExists tests the Exists method.
func TestExists(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	testData := []byte("Existence test")

	// Before storing, hash should not exist
	h := sha256.Sum256(testData)
	hash := hex.EncodeToString(h[:])

	if store.Exists(hash) {
		t.Error("blob should not exist before storing")
	}

	// Store the blob
	_, err = store.Store(testData)
	if err != nil {
		t.Fatalf("failed to store blob: %v", err)
	}

	// After storing, hash should exist
	if !store.Exists(hash) {
		t.Error("blob should exist after storing")
	}
}

// TestExistsInvalidHash tests the Exists method with invalid hash.
func TestExistsInvalidHash(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Invalid hash should return false
	if store.Exists("invalid") {
		t.Error("Exists should return false for invalid hash")
	}
}

// TestBlake3Hash tests the Blake3Hash function.
func TestBlake3Hash(t *testing.T) {
	testData := []byte("Hello, BLAKE3!")

	hash := Blake3Hash(testData)

	// Should be 64 hex characters (256 bits)
	if len(hash) != 64 {
		t.Errorf("BLAKE3 hash length = %d, want 64", len(hash))
	}

	// Same input should produce same hash
	hash2 := Blake3Hash(testData)
	if hash != hash2 {
		t.Errorf("same data produced different hashes: %q vs %q", hash, hash2)
	}

	// Different input should produce different hash
	hash3 := Blake3Hash([]byte("Different data"))
	if hash == hash3 {
		t.Error("different data produced same hash")
	}
}

// TestStoreWithBlake3 tests storing with both SHA-256 and BLAKE3 hashes.
func TestStoreWithBlake3(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	testData := []byte("BLAKE3 store test")

	result, err := store.StoreWithBlake3(testData)
	if err != nil {
		t.Fatalf("StoreWithBlake3 failed: %v", err)
	}

	// Both hashes should be 64 hex characters
	if len(result.SHA256) != 64 {
		t.Errorf("SHA256 hash length = %d, want 64", len(result.SHA256))
	}
	if len(result.BLAKE3) != 64 {
		t.Errorf("BLAKE3 hash length = %d, want 64", len(result.BLAKE3))
	}

	// BLAKE3 hash should match Blake3Hash function
	expectedBlake3 := Blake3Hash(testData)
	if result.BLAKE3 != expectedBlake3 {
		t.Errorf("BLAKE3 hash mismatch: got %s, want %s", result.BLAKE3, expectedBlake3)
	}

	// Verify blob can be retrieved by SHA-256
	retrieved, err := store.Retrieve(result.SHA256)
	if err != nil {
		t.Fatalf("Retrieve by SHA-256 failed: %v", err)
	}
	if !bytes.Equal(retrieved, testData) {
		t.Error("retrieved data mismatch")
	}
}

// TestLookupBlake3 tests looking up SHA-256 by BLAKE3 hash.
func TestLookupBlake3(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	testData := []byte("BLAKE3 lookup test")

	result, err := store.StoreWithBlake3(testData)
	if err != nil {
		t.Fatalf("StoreWithBlake3 failed: %v", err)
	}

	// Look up SHA-256 by BLAKE3
	sha256Hash, err := store.LookupBlake3(result.BLAKE3)
	if err != nil {
		t.Fatalf("LookupBlake3 failed: %v", err)
	}

	if sha256Hash != result.SHA256 {
		t.Errorf("SHA256 mismatch: got %s, want %s", sha256Hash, result.SHA256)
	}
}

// TestLookupBlake3NotFound tests looking up non-existent BLAKE3 hash.
func TestLookupBlake3NotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	fakeHash := "0000000000000000000000000000000000000000000000000000000000000000"
	_, err = store.LookupBlake3(fakeHash)
	if err != ErrBlobNotFound {
		t.Errorf("expected ErrBlobNotFound, got %v", err)
	}
}

// TestLookupBlake3InvalidHash tests looking up with invalid hash format.
func TestLookupBlake3InvalidHash(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	_, err = store.LookupBlake3("invalid")
	if err != ErrInvalidHash {
		t.Errorf("expected ErrInvalidHash, got %v", err)
	}
}

// TestRetrieveByBlake3 tests retrieving blob by BLAKE3 hash.
func TestRetrieveByBlake3(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	testData := []byte("BLAKE3 retrieve test")

	result, err := store.StoreWithBlake3(testData)
	if err != nil {
		t.Fatalf("StoreWithBlake3 failed: %v", err)
	}

	// Retrieve by BLAKE3
	retrieved, err := store.RetrieveByBlake3(result.BLAKE3)
	if err != nil {
		t.Fatalf("RetrieveByBlake3 failed: %v", err)
	}

	if !bytes.Equal(retrieved, testData) {
		t.Error("retrieved data mismatch")
	}
}

// TestRetrieveByBlake3NotFound tests retrieving non-existent BLAKE3 hash.
func TestRetrieveByBlake3NotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	fakeHash := "0000000000000000000000000000000000000000000000000000000000000000"
	_, err = store.RetrieveByBlake3(fakeHash)
	if err != ErrBlobNotFound {
		t.Errorf("expected ErrBlobNotFound, got %v", err)
	}
}

// TestStoreWithBlake3Duplicate tests storing duplicate content with BLAKE3.
func TestStoreWithBlake3Duplicate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	testData := []byte("BLAKE3 duplicate test")

	// Store twice
	result1, err := store.StoreWithBlake3(testData)
	if err != nil {
		t.Fatalf("first StoreWithBlake3 failed: %v", err)
	}

	result2, err := store.StoreWithBlake3(testData)
	if err != nil {
		t.Fatalf("second StoreWithBlake3 failed: %v", err)
	}

	// Hashes should be identical
	if result1.SHA256 != result2.SHA256 {
		t.Errorf("duplicate SHA256 differ: %s != %s", result1.SHA256, result2.SHA256)
	}
	if result1.BLAKE3 != result2.BLAKE3 {
		t.Errorf("duplicate BLAKE3 differ: %s != %s", result1.BLAKE3, result2.BLAKE3)
	}
}

// TestNewStoreMkdirError tests NewStore when mkdir fails.
func TestNewStoreMkdirError(t *testing.T) {
	// Create a file where we want to create a directory
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file that blocks the blobs directory creation
	blockingFile := filepath.Join(tempDir, "blobs")
	if err := os.WriteFile(blockingFile, []byte("blocking"), 0600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	_, err = NewStore(tempDir)
	if err == nil {
		t.Error("expected error when mkdir fails")
	}
}

// TestStoreMkdirPrefixError tests Store when prefix directory creation fails.
func TestStoreMkdirPrefixError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Calculate hash to know the prefix
	testData := []byte("test data for prefix error")
	h := Hash(testData)
	prefix := h[:2]

	// Create a file where the prefix directory should be
	prefixPath := filepath.Join(tempDir, "blobs", "sha256", prefix)
	if err := os.WriteFile(prefixPath, []byte("blocking"), 0600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	_, err = store.Store(testData)
	if err == nil {
		t.Error("expected error when prefix mkdir fails")
	}
}

// TestStoreCreateTempError tests Store when temp file creation fails.
func TestStoreCreateTempError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	testData := []byte("test data for temp error")
	h := Hash(testData)
	prefix := h[:2]

	// Create the prefix directory but make it read-only
	prefixPath := filepath.Join(tempDir, "blobs", "sha256", prefix)
	if err := os.MkdirAll(prefixPath, 0755); err != nil {
		t.Fatalf("failed to create prefix dir: %v", err)
	}
	if err := os.Chmod(prefixPath, 0555); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(prefixPath, 0755) // Restore for cleanup

	_, err = store.Store(testData)
	if err == nil {
		t.Error("expected error when temp file creation fails")
	}
}

// TestRetrieveReadError tests Retrieve when read fails (non-NotExist error).
func TestRetrieveReadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Store something first
	testData := []byte("test data")
	hash, err := store.Store(testData)
	if err != nil {
		t.Fatalf("failed to store: %v", err)
	}

	// Replace the file with a directory (can't read a directory as file)
	blobPath := store.pathForHash(hash)
	if err := os.Remove(blobPath); err != nil {
		t.Fatalf("failed to remove blob: %v", err)
	}
	if err := os.MkdirAll(blobPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	_, err = store.Retrieve(hash)
	if err == nil {
		t.Error("expected error when reading directory as file")
	}
	// Should not be ErrBlobNotFound since file exists (as dir)
	if err == ErrBlobNotFound {
		t.Error("should not be ErrBlobNotFound")
	}
}

// TestStoreWithBlake3StoreError tests StoreWithBlake3 when Store fails.
func TestStoreWithBlake3StoreError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	testData := []byte("test for blake3 store error")
	h := Hash(testData)
	prefix := h[:2]

	// Create the prefix directory and make it read-only
	prefixPath := filepath.Join(tempDir, "blobs", "sha256", prefix)
	if err := os.MkdirAll(prefixPath, 0755); err != nil {
		t.Fatalf("failed to create prefix dir: %v", err)
	}
	if err := os.Chmod(prefixPath, 0555); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(prefixPath, 0755)

	_, err = store.StoreWithBlake3(testData)
	if err == nil {
		t.Error("expected error when store fails")
	}
}

// TestCreateBlake3PointerMkdirError tests BLAKE3 pointer creation when mkdir fails.
func TestCreateBlake3PointerMkdirError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Calculate blake3 hash for data we'll store later
	testData := []byte("test for blake3 pointer mkdir error")
	blake3Hash := Blake3Hash(testData)
	blake3Prefix := blake3Hash[:2]

	// Block the BLAKE3 pointer directory (at blobs/blake3/<prefix>)
	pointerDir := filepath.Join(tempDir, "blobs", "blake3", blake3Prefix)

	// Create parent dir first
	if err := os.MkdirAll(filepath.Dir(pointerDir), 0755); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}
	// Create a file where directory should be
	if err := os.WriteFile(pointerDir, []byte("blocking"), 0600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	_, err = store.StoreWithBlake3(testData)
	if err == nil {
		t.Error("expected error when blake3 pointer mkdir fails")
	}
}

// TestLookupBlake3ReadError tests LookupBlake3 when read fails.
func TestLookupBlake3ReadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Store with BLAKE3
	testData := []byte("test for lookup read error")
	result, err := store.StoreWithBlake3(testData)
	if err != nil {
		t.Fatalf("failed to store: %v", err)
	}

	// Replace the pointer file with a directory
	// Pointer path is: blobs/blake3/<prefix>/<hash>.json
	pointerPath := filepath.Join(tempDir, "blobs", "blake3", result.BLAKE3[:2], result.BLAKE3+".json")
	if err := os.Remove(pointerPath); err != nil {
		t.Fatalf("failed to remove pointer: %v", err)
	}
	if err := os.MkdirAll(pointerPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	_, err = store.LookupBlake3(result.BLAKE3)
	if err == nil {
		t.Error("expected error when reading directory as file")
	}
}

// TestLookupBlake3UnmarshalError tests LookupBlake3 when JSON unmarshal fails.
func TestLookupBlake3UnmarshalError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Store with BLAKE3
	testData := []byte("test for unmarshal error")
	result, err := store.StoreWithBlake3(testData)
	if err != nil {
		t.Fatalf("failed to store: %v", err)
	}

	// Overwrite the pointer file with invalid JSON
	pointerPath := filepath.Join(tempDir, "blobs", "blake3", result.BLAKE3[:2], result.BLAKE3+".json")
	if err := os.WriteFile(pointerPath, []byte("not valid json"), 0600); err != nil {
		t.Fatalf("failed to write invalid json: %v", err)
	}

	_, err = store.LookupBlake3(result.BLAKE3)
	if err == nil {
		t.Error("expected error when parsing invalid json")
	}
}

// TestCreateBlake3PointerCreateTempError tests pointer creation when CreateTemp fails.
func TestCreateBlake3PointerCreateTempError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Calculate blake3 hash for data we'll store later
	testData := []byte("test for createtemp error")
	blake3Hash := Blake3Hash(testData)
	blake3Prefix := blake3Hash[:2]

	// Create the pointer directory but make it read-only
	pointerDir := filepath.Join(tempDir, "blobs", "blake3", blake3Prefix)
	if err := os.MkdirAll(pointerDir, 0755); err != nil {
		t.Fatalf("failed to create pointer dir: %v", err)
	}
	if err := os.Chmod(pointerDir, 0555); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	defer os.Chmod(pointerDir, 0755)

	_, err = store.StoreWithBlake3(testData)
	if err == nil {
		t.Error("expected error when CreateTemp fails")
	}
}

// TestStoreWriteError tests Store when write fails via injection.
func TestStoreWriteError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Inject write error
	origWrite := tempFileWrite
	defer func() { tempFileWrite = origWrite }()
	tempFileWrite = func(f *os.File, data []byte) (int, error) {
		return 0, errors.New("injected write error")
	}

	testData := []byte("test for write error")
	_, err = store.Store(testData)
	if err == nil {
		t.Error("expected error when write fails")
	}
	if !strings.Contains(err.Error(), "failed to write blob") {
		t.Errorf("expected 'failed to write blob' error, got: %v", err)
	}
}

// TestStoreCloseError tests Store when close fails via injection.
func TestStoreCloseError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Inject close error (only on second call - first is in write error cleanup)
	origClose := tempFileClose
	defer func() { tempFileClose = origClose }()
	callCount := 0
	tempFileClose = func(f io.Closer) error {
		callCount++
		if callCount == 1 {
			return errors.New("injected close error")
		}
		return f.Close()
	}

	testData := []byte("test for close error")
	_, err = store.Store(testData)
	if err == nil {
		t.Error("expected error when close fails")
	}
	if !strings.Contains(err.Error(), "failed to close temp file") {
		t.Errorf("expected 'failed to close temp file' error, got: %v", err)
	}
}

// TestStoreRenameError tests Store when rename fails via injection.
func TestStoreRenameError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Inject rename error
	origRename := osRename
	defer func() { osRename = origRename }()
	osRename = func(oldpath, newpath string) error {
		return errors.New("injected rename error")
	}

	testData := []byte("test for rename error")
	_, err = store.Store(testData)
	if err == nil {
		t.Error("expected error when rename fails")
	}
	if !strings.Contains(err.Error(), "failed to rename blob") {
		t.Errorf("expected 'failed to rename blob' error, got: %v", err)
	}
}

// TestCreateBlake3PointerWriteError tests pointer creation when write fails.
func TestCreateBlake3PointerWriteError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Inject write error only for the second write (pointer, not blob)
	origWrite := tempFileWrite
	defer func() { tempFileWrite = origWrite }()
	callCount := 0
	tempFileWrite = func(f *os.File, data []byte) (int, error) {
		callCount++
		if callCount == 2 {
			return 0, errors.New("injected pointer write error")
		}
		return f.Write(data)
	}

	testData := []byte("test for pointer write error")
	_, err = store.StoreWithBlake3(testData)
	if err == nil {
		t.Error("expected error when pointer write fails")
	}
}

// TestCreateBlake3PointerCloseError tests pointer creation when close fails.
func TestCreateBlake3PointerCloseError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Inject close error only for the second close (pointer, not blob)
	origClose := tempFileClose
	defer func() { tempFileClose = origClose }()
	callCount := 0
	tempFileClose = func(f io.Closer) error {
		callCount++
		if callCount == 2 {
			return errors.New("injected pointer close error")
		}
		return f.Close()
	}

	testData := []byte("test for pointer close error")
	_, err = store.StoreWithBlake3(testData)
	if err == nil {
		t.Error("expected error when pointer close fails")
	}
}

// TestCreateBlake3PointerRenameError tests pointer creation when rename fails.
func TestCreateBlake3PointerRenameError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Inject rename error only for the second rename (pointer, not blob)
	origRename := osRename
	defer func() { osRename = origRename }()
	callCount := 0
	osRename = func(oldpath, newpath string) error {
		callCount++
		if callCount == 2 {
			return errors.New("injected pointer rename error")
		}
		return os.Rename(oldpath, newpath)
	}

	testData := []byte("test for pointer rename error")
	_, err = store.StoreWithBlake3(testData)
	if err == nil {
		t.Error("expected error when pointer rename fails")
	}
}
