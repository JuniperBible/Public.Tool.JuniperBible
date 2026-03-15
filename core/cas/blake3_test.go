package cas

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zeebo/blake3"
)

// TestBlake3Store tests that storing with BLAKE3 creates pointer files.
func TestBlake3Store(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-blake3-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	testData := []byte("BLAKE3 test data")

	// Store with BLAKE3 enabled
	result, err := store.StoreWithBlake3(ctx, testData)
	if err != nil {
		t.Fatalf("failed to store with BLAKE3: %v", err)
	}

	// Verify SHA-256 hash is correct
	expectedSha256 := Hash(testData)
	if result.SHA256 != expectedSha256 {
		t.Errorf("SHA-256 mismatch: got %s, want %s", result.SHA256, expectedSha256)
	}

	// Verify BLAKE3 hash is correct
	h := blake3.Sum256(testData)
	expectedBlake3 := hashToHex(h[:])
	if result.BLAKE3 != expectedBlake3 {
		t.Errorf("BLAKE3 mismatch: got %s, want %s", result.BLAKE3, expectedBlake3)
	}

	// Verify pointer file exists
	pointerPath := filepath.Join(tempDir, "blobs", "blake3", result.BLAKE3[:2], result.BLAKE3+".json")
	if _, err := os.Stat(pointerPath); os.IsNotExist(err) {
		t.Errorf("BLAKE3 pointer file should exist at %s", pointerPath)
	}
}

// TestBlake3Lookup tests looking up a blob by its BLAKE3 hash.
func TestBlake3Lookup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-blake3-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	testData := []byte("BLAKE3 lookup test")

	result, err := store.StoreWithBlake3(ctx, testData)
	if err != nil {
		t.Fatalf("failed to store with BLAKE3: %v", err)
	}

	// Look up by BLAKE3 hash
	sha256Hash, err := store.LookupBlake3(ctx, result.BLAKE3)
	if err != nil {
		t.Fatalf("failed to lookup by BLAKE3: %v", err)
	}

	if sha256Hash != result.SHA256 {
		t.Errorf("BLAKE3 lookup returned wrong SHA-256: got %s, want %s", sha256Hash, result.SHA256)
	}

	// Retrieve by the looked-up SHA-256
	retrieved, err := store.Retrieve(ctx, sha256Hash)
	if err != nil {
		t.Fatalf("failed to retrieve: %v", err)
	}

	if string(retrieved) != string(testData) {
		t.Errorf("data mismatch: got %q, want %q", retrieved, testData)
	}
}

// TestBlake3LookupNonExistent tests that looking up non-existent BLAKE3 hash fails.
func TestBlake3LookupNonExistent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-blake3-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	fakeHash := "0000000000000000000000000000000000000000000000000000000000000000"
	_, err = store.LookupBlake3(ctx, fakeHash)
	if err == nil {
		t.Error("expected error when looking up non-existent BLAKE3 hash")
	}
}

// TestBlake3RetrieveByBlake3 tests retrieving data directly by BLAKE3 hash.
func TestBlake3RetrieveByBlake3(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cas-blake3-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	testData := []byte("Direct BLAKE3 retrieval test")

	result, err := store.StoreWithBlake3(ctx, testData)
	if err != nil {
		t.Fatalf("failed to store with BLAKE3: %v", err)
	}

	// Retrieve directly by BLAKE3 hash
	retrieved, err := store.RetrieveByBlake3(ctx, result.BLAKE3)
	if err != nil {
		t.Fatalf("failed to retrieve by BLAKE3: %v", err)
	}

	if string(retrieved) != string(testData) {
		t.Errorf("data mismatch: got %q, want %q", retrieved, testData)
	}
}

// hashToHex is a helper to convert hash bytes to hex string.
func hashToHex(b []byte) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, len(b)*2)
	for i, v := range b {
		result[i*2] = hexChars[v>>4]
		result[i*2+1] = hexChars[v&0x0f]
	}
	return string(result)
}
