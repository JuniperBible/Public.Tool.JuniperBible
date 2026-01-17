// Package archive provides utilities for reading compressed tar archives.
// It supports tar.gz and tar.xz formats used by capsule files.
package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ulikunitz/xz"
)

// tocCache caches the list of file names in archives to avoid repeated decompression.
// This is especially important for .tar.xz files which are slow to decompress.
var tocCache struct {
	sync.RWMutex
	entries   map[string]*tocEntry
	maxSize   int           // Maximum number of entries
	ttl       time.Duration // Time-to-live for cache entries
}

type tocEntry struct {
	files     []string  // List of file names in the archive
	timestamp time.Time // When this entry was cached
	modTime   time.Time // File modification time when cached
}

func init() {
	tocCache.entries = make(map[string]*tocEntry)
	tocCache.maxSize = 500 // Cache up to 500 archives
	tocCache.ttl = 30 * time.Minute
}

// getTOC returns the cached table of contents for an archive, or nil if not cached.
func getTOC(path string) []string {
	tocCache.RLock()
	entry, ok := tocCache.entries[path]
	tocCache.RUnlock()

	if !ok {
		return nil
	}

	// Check TTL
	if time.Since(entry.timestamp) > tocCache.ttl {
		return nil
	}

	// Check if file was modified since caching
	info, err := os.Stat(path)
	if err != nil || !info.ModTime().Equal(entry.modTime) {
		return nil
	}

	return entry.files
}

// setTOC caches the table of contents for an archive.
func setTOC(path string, files []string) {
	info, err := os.Stat(path)
	if err != nil {
		return // Don't cache if we can't stat the file
	}

	tocCache.Lock()
	defer tocCache.Unlock()

	// Evict old entries if cache is full (simple LRU would be better, but this is good enough)
	if len(tocCache.entries) >= tocCache.maxSize {
		// Remove oldest 20% of entries
		toRemove := tocCache.maxSize / 5
		removed := 0
		for k := range tocCache.entries {
			delete(tocCache.entries, k)
			removed++
			if removed >= toRemove {
				break
			}
		}
	}

	tocCache.entries[path] = &tocEntry{
		files:     files,
		timestamp: time.Now(),
		modTime:   info.ModTime(),
	}
}

// InvalidateTOC removes a specific archive from the TOC cache.
func InvalidateTOC(path string) {
	tocCache.Lock()
	delete(tocCache.entries, path)
	tocCache.Unlock()
}

// ClearTOCCache clears the entire TOC cache.
func ClearTOCCache() {
	tocCache.Lock()
	tocCache.entries = make(map[string]*tocEntry)
	tocCache.Unlock()
}

// Reader wraps a tar.Reader with automatic decompression handling.
type Reader struct {
	*tar.Reader
	file         *os.File
	decompressor io.Closer
}

// NewReader creates a new archive reader for the given path.
// It automatically detects and handles .tar.gz and .tar.xz compression.
func NewReader(path string) (*Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}

	var reader io.Reader = f
	var decompressor io.Closer

	switch {
	case strings.HasSuffix(path, ".tar.xz"):
		xzr, err := xz.NewReader(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("xz reader: %w", err)
		}
		reader = xzr
		decompressor = nil // xz reader doesn't need closing
	case strings.HasSuffix(path, ".tar.gz"):
		gzr, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		reader = gzr
		decompressor = gzr
	default:
		f.Close()
		return nil, fmt.Errorf("unsupported archive format: %s", path)
	}

	return &Reader{
		Reader:       tar.NewReader(reader),
		file:         f,
		decompressor: decompressor,
	}, nil
}

// Close closes the archive reader and any underlying decompressors.
func (r *Reader) Close() error {
	var errs []error
	if r.decompressor != nil {
		if err := r.decompressor.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := r.file.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Visitor is a callback function for iterating archive entries.
// Return true to stop iteration, false to continue.
type Visitor func(header *tar.Header, content io.Reader) (stop bool, err error)

// Iterate walks through all entries in the archive, calling the visitor for each.
func (r *Reader) Iterate(visitor Visitor) error {
	for {
		header, err := r.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read header: %w", err)
		}

		stop, err := visitor(header, r)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
}

// IterateCapsule opens an archive and iterates through its entries.
func IterateCapsule(path string, visitor Visitor) error {
	r, err := NewReader(path)
	if err != nil {
		return err
	}
	defer r.Close()
	return r.Iterate(visitor)
}

// ContainsPath checks if the archive contains a path matching the predicate.
func ContainsPath(path string, predicate func(name string) bool) (bool, error) {
	var found bool
	err := IterateCapsule(path, func(header *tar.Header, _ io.Reader) (bool, error) {
		if predicate(header.Name) {
			found = true
			return true, nil // stop iteration
		}
		return false, nil
	})
	return found, err
}

// ReadFile reads a specific file from the archive.
func ReadFile(archivePath, filename string) ([]byte, error) {
	var content []byte
	err := IterateCapsule(archivePath, func(header *tar.Header, r io.Reader) (bool, error) {
		// Handle archives with or without leading directory
		name := header.Name
		if idx := strings.Index(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		if name == filename || header.Name == filename {
			var err error
			content, err = io.ReadAll(r)
			return true, err
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, fmt.Errorf("file not found: %s", filename)
	}
	return content, nil
}

// FindFile finds the first file matching the predicate and returns its content.
func FindFile(archivePath string, predicate func(name string) bool) ([]byte, string, error) {
	var content []byte
	var foundName string
	err := IterateCapsule(archivePath, func(header *tar.Header, r io.Reader) (bool, error) {
		if predicate(header.Name) {
			var err error
			content, err = io.ReadAll(r)
			foundName = header.Name
			return true, err
		}
		return false, nil
	})
	if err != nil {
		return nil, "", err
	}
	if content == nil {
		return nil, "", fmt.Errorf("no matching file found")
	}
	return content, foundName, nil
}
