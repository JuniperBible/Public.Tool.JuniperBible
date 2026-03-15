package cas

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"

	"github.com/zeebo/blake3"
)

// ErrBlobNotFound is returned when a blob with the given hash does not exist.
var ErrBlobNotFound = errors.New("blob not found")

// ErrInvalidHash is returned when a hash string is not a valid SHA-256 hex string.
var ErrInvalidHash = errors.New("invalid hash format")

// sha256Pattern matches a valid lowercase SHA-256 hex string (64 characters).
var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// isValidHash checks if a hash string is a valid SHA-256 hex string.
func isValidHash(hash string) bool {
	return sha256Pattern.MatchString(hash)
}

// Hash computes the SHA-256 hash of the given data without storing it.
func Hash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Blake3Hash computes the BLAKE3 hash of the given data without storing it.
func Blake3Hash(data []byte) string {
	h := blake3.Sum256(data)
	return hex.EncodeToString(h[:])
}
