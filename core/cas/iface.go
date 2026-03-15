package cas

import "context"

// BlobStore is the interface for content-addressed blob storage.
// Both the filesystem Store and VeronicaStore implement this interface.
type BlobStore interface {
	// Store stores the given data and returns its SHA-256 hash.
	Store(ctx context.Context, data []byte) (string, error)

	// Retrieve retrieves the blob with the given SHA-256 hash.
	Retrieve(ctx context.Context, hash string) ([]byte, error)

	// Exists checks if a blob with the given hash exists in the store.
	Exists(ctx context.Context, hash string) bool

	// StoreWithBlake3 stores the given data and returns both SHA-256 and BLAKE3 hashes.
	StoreWithBlake3(ctx context.Context, data []byte) (*HashResult, error)

	// LookupBlake3 looks up a SHA-256 hash by its corresponding BLAKE3 hash.
	LookupBlake3(ctx context.Context, blake3Hash string) (string, error)

	// RetrieveByBlake3 retrieves a blob by its BLAKE3 hash.
	RetrieveByBlake3(ctx context.Context, blake3Hash string) ([]byte, error)
}

// Verify that Store implements BlobStore at compile time.
var _ BlobStore = (*Store)(nil)
