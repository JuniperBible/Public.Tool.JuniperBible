package cas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
)

// mockVeronicaCAS is a test double for VeronicaCAS.
type mockVeronicaCAS struct {
	blobs map[string][]byte // keyed by "sha256:hex"
}

func newMockVeronicaCAS() *mockVeronicaCAS {
	return &mockVeronicaCAS{blobs: make(map[string][]byte)}
}

func (m *mockVeronicaCAS) Put(_ context.Context, data []byte) (string, error) {
	h := sha256.Sum256(data)
	digest := "sha256:" + hex.EncodeToString(h[:])
	m.blobs[digest] = append([]byte(nil), data...)
	return digest, nil
}

func (m *mockVeronicaCAS) Get(_ context.Context, digest string) ([]byte, error) {
	data, ok := m.blobs[digest]
	if !ok {
		return nil, fmt.Errorf("not found: %s", digest)
	}
	return data, nil
}

func TestVeronicaStoreAndRetrieve(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "veronica-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	mock := newMockVeronicaCAS()
	store, err := NewVeronicaStore(mock, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("hello veronica")
	hash, err := store.Store(ctx, data)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Verify hash is raw hex (no prefix)
	if len(hash) != 64 {
		t.Errorf("expected 64 char hex hash, got %d: %s", len(hash), hash)
	}

	// Retrieve
	got, err := store.Retrieve(ctx, hash)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("data mismatch: got %q, want %q", got, data)
	}
}

func TestVeronicaStoreExists(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "veronica-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	mock := newMockVeronicaCAS()
	store, err := NewVeronicaStore(mock, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("exists test")
	hash, err := store.Store(ctx, data)
	if err != nil {
		t.Fatal(err)
	}

	if !store.Exists(ctx, hash) {
		t.Error("Exists should return true for stored blob")
	}

	if store.Exists(ctx, "0000000000000000000000000000000000000000000000000000000000000000") {
		t.Error("Exists should return false for non-existent blob")
	}
}

func TestVeronicaStoreWithBlake3(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "veronica-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	mock := newMockVeronicaCAS()
	store, err := NewVeronicaStore(mock, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("blake3 test")
	result, err := store.StoreWithBlake3(ctx, data)
	if err != nil {
		t.Fatalf("StoreWithBlake3 failed: %v", err)
	}

	if len(result.SHA256) != 64 {
		t.Errorf("expected 64 char SHA256, got %d", len(result.SHA256))
	}
	if len(result.BLAKE3) != 64 {
		t.Errorf("expected 64 char BLAKE3, got %d", len(result.BLAKE3))
	}

	// Verify BLAKE3 lookup
	sha256Hash, err := store.LookupBlake3(ctx, result.BLAKE3)
	if err != nil {
		t.Fatalf("LookupBlake3 failed: %v", err)
	}
	if sha256Hash != result.SHA256 {
		t.Errorf("LookupBlake3 returned %s, want %s", sha256Hash, result.SHA256)
	}

	// Verify RetrieveByBlake3
	got, err := store.RetrieveByBlake3(ctx, result.BLAKE3)
	if err != nil {
		t.Fatalf("RetrieveByBlake3 failed: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("data mismatch: got %q, want %q", got, data)
	}
}

func TestVeronicaStoreInvalidHash(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "veronica-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	mock := newMockVeronicaCAS()
	store, err := NewVeronicaStore(mock, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Retrieve(ctx, "invalid")
	if err != ErrInvalidHash {
		t.Errorf("expected ErrInvalidHash, got %v", err)
	}

	if store.Exists(ctx, "invalid") {
		t.Error("Exists should return false for invalid hash")
	}
}

func TestVeronicaStoreDeduplicate(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "veronica-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	mock := newMockVeronicaCAS()
	store, err := NewVeronicaStore(mock, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("dedup test")
	hash1, err := store.Store(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
	hash2, err := store.Store(ctx, data)
	if err != nil {
		t.Fatal(err)
	}

	if hash1 != hash2 {
		t.Errorf("same data should produce same hash: %s != %s", hash1, hash2)
	}
}

func TestHash(t *testing.T) {
	data := []byte("hello world")
	hash := Hash(data)
	if len(hash) != 64 {
		t.Errorf("expected 64 char hex hash, got %d: %s", len(hash), hash)
	}

	// Same data should produce same hash
	hash2 := Hash(data)
	if hash != hash2 {
		t.Errorf("same data produced different hashes: %s != %s", hash, hash2)
	}

	// Different data should produce different hash
	hash3 := Hash([]byte("different"))
	if hash == hash3 {
		t.Error("different data produced same hash")
	}
}

func TestBlake3Hash(t *testing.T) {
	data := []byte("hello world")
	hash := Blake3Hash(data)
	if len(hash) != 64 {
		t.Errorf("expected 64 char hex hash, got %d: %s", len(hash), hash)
	}

	// Same data should produce same hash
	hash2 := Blake3Hash(data)
	if hash != hash2 {
		t.Errorf("same data produced different hashes: %s != %s", hash, hash2)
	}

	// SHA-256 and BLAKE3 should differ
	sha256Hash := Hash(data)
	if hash == sha256Hash {
		t.Error("BLAKE3 and SHA-256 should produce different hashes")
	}
}

func TestIsValidHash(t *testing.T) {
	valid := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if !isValidHash(valid) {
		t.Error("expected valid hash to pass")
	}
	if isValidHash("invalid") {
		t.Error("expected invalid hash to fail")
	}
	if isValidHash("") {
		t.Error("expected empty hash to fail")
	}
	if isValidHash("E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855") {
		t.Error("expected uppercase hash to fail")
	}
}

func TestStripAndAddPrefix(t *testing.T) {
	hash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	digest := "sha256:" + hash

	if got := stripPrefix(digest); got != hash {
		t.Errorf("stripPrefix(%q) = %q, want %q", digest, got, hash)
	}

	if got := addPrefix(hash); got != digest {
		t.Errorf("addPrefix(%q) = %q, want %q", hash, got, digest)
	}

	// stripPrefix with no prefix returns as-is
	if got := stripPrefix(hash); got != hash {
		t.Errorf("stripPrefix(%q) = %q, want %q", hash, got, hash)
	}
}
