package cas

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeebo/blake3"
)

// VeronicaCAS is the interface for Veronica's CAS client, matching the
// Put/Get methods of veronica.CASClient. This allows testing without
// importing the veronica package directly.
type VeronicaCAS interface {
	Put(ctx context.Context, data []byte) (string, error)
	Get(ctx context.Context, digest string) ([]byte, error)
}

// VeronicaStore implements BlobStore using a Veronica CAS backend.
// It adapts between Juniper's raw hex hash format and Veronica's
// "sha256:hex" prefixed digest format.
type VeronicaStore struct {
	cas VeronicaCAS
	// blake3Root stores BLAKE3 pointer files locally since Veronica
	// only supports SHA-256 addressing.
	blake3Root string
}

// NewVeronicaStore creates a new VeronicaStore wrapping a Veronica CAS client.
// blake3Root is a local directory for BLAKE3-to-SHA256 pointer files.
func NewVeronicaStore(cas VeronicaCAS, blake3Root string) (*VeronicaStore, error) {
	pointerDir := filepath.Join(blake3Root, "blobs", "blake3")
	if err := os.MkdirAll(pointerDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create blake3 pointer directory: %w", err)
	}
	return &VeronicaStore{cas: cas, blake3Root: blake3Root}, nil
}

// Store stores data via Veronica and returns the raw SHA-256 hex hash.
func (v *VeronicaStore) Store(ctx context.Context, data []byte) (string, error) {
	digest, err := v.cas.Put(ctx, data)
	if err != nil {
		return "", fmt.Errorf("veronica put failed: %w", err)
	}
	return stripPrefix(digest), nil
}

// Retrieve retrieves data by raw SHA-256 hex hash via Veronica.
func (v *VeronicaStore) Retrieve(ctx context.Context, hash string) ([]byte, error) {
	if !isValidHash(hash) {
		return nil, ErrInvalidHash
	}
	data, err := v.cas.Get(ctx, addPrefix(hash))
	if err != nil {
		return nil, fmt.Errorf("veronica get failed: %w", err)
	}
	return data, nil
}

// Exists checks if a blob exists in Veronica by attempting to retrieve it.
func (v *VeronicaStore) Exists(ctx context.Context, hash string) bool {
	if !isValidHash(hash) {
		return false
	}
	_, err := v.cas.Get(ctx, addPrefix(hash))
	return err == nil
}

// StoreWithBlake3 stores data via Veronica and computes both SHA-256 and BLAKE3 hashes.
// The BLAKE3 pointer mapping is maintained locally.
func (v *VeronicaStore) StoreWithBlake3(ctx context.Context, data []byte) (*HashResult, error) {
	sha256Hash, err := v.Store(ctx, data)
	if err != nil {
		return nil, err
	}

	b3 := blake3.Sum256(data)
	blake3Hash := hex.EncodeToString(b3[:])

	if err := v.createBlake3Pointer(blake3Hash, sha256Hash); err != nil {
		return nil, fmt.Errorf("failed to create BLAKE3 pointer: %w", err)
	}

	return &HashResult{
		SHA256: sha256Hash,
		BLAKE3: blake3Hash,
	}, nil
}

// LookupBlake3 looks up a SHA-256 hash by BLAKE3 hash using local pointer files.
func (v *VeronicaStore) LookupBlake3(_ context.Context, blake3Hash string) (string, error) {
	if !isValidHash(blake3Hash) {
		return "", ErrInvalidHash
	}

	prefix := blake3Hash[:2]
	pointerPath := filepath.Join(v.blake3Root, "blobs", "blake3", prefix, blake3Hash+".json")

	data, err := os.ReadFile(pointerPath)
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
func (v *VeronicaStore) RetrieveByBlake3(ctx context.Context, blake3Hash string) ([]byte, error) {
	sha256Hash, err := v.LookupBlake3(ctx, blake3Hash)
	if err != nil {
		return nil, err
	}
	return v.Retrieve(ctx, sha256Hash)
}

// createBlake3Pointer creates a local pointer file mapping BLAKE3 to SHA-256.
func (v *VeronicaStore) createBlake3Pointer(blake3Hash, sha256Hash string) error {
	prefix := blake3Hash[:2]
	pointerDir := filepath.Join(v.blake3Root, "blobs", "blake3", prefix)

	if err := os.MkdirAll(pointerDir, 0700); err != nil {
		return fmt.Errorf("failed to create blake3 directory: %w", err)
	}

	pointerPath := filepath.Join(pointerDir, blake3Hash+".json")

	if _, err := os.Stat(pointerPath); err == nil {
		return nil // Already exists
	}

	pointer := blake3Pointer{SHA256: sha256Hash}
	data, err := json.Marshal(pointer)
	if err != nil {
		return fmt.Errorf("failed to marshal pointer: %w", err)
	}

	if err := os.WriteFile(pointerPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write pointer: %w", err)
	}

	return nil
}

// stripPrefix removes the "sha256:" prefix from a Veronica digest string.
func stripPrefix(digest string) string {
	if after, ok := strings.CutPrefix(digest, "sha256:"); ok {
		return after
	}
	return digest
}

// addPrefix adds the "sha256:" prefix for Veronica digest format.
func addPrefix(hash string) string {
	return "sha256:" + hash
}

// Verify VeronicaStore implements BlobStore at compile time.
var _ BlobStore = (*VeronicaStore)(nil)
