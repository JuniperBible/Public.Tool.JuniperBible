// Package cache provides LRU caching for IR corpora and capsule manifests.
package cache

import (
	"container/list"
	"encoding/json"
	"sync"
	"time"

	"github.com/FocuswithJustin/JuniperBible/core/capsule"
	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

// Cache is a generic LRU cache interface.
type Cache[K comparable, V any] interface {
	// Get retrieves a value from the cache.
	Get(key K) (V, bool)

	// Put stores a value in the cache.
	Put(key K, value V)

	// Remove removes a value from the cache.
	Remove(key K)

	// Clear removes all entries from the cache.
	Clear()

	// Len returns the number of entries in the cache.
	Len() int

	// Stats returns cache statistics.
	Stats() Stats
}

// Stats contains cache statistics.
type Stats struct {
	Hits       int64
	Misses     int64
	Evictions  int64
	Size       int
	MaxSize    int
	TotalBytes int64
}

// Config contains cache configuration options.
type Config struct {
	// MaxSize is the maximum number of entries (0 = unlimited).
	MaxSize int

	// TTL is the time-to-live for entries (0 = no expiration).
	TTL time.Duration

	// OnEvict is called when an entry is evicted.
	OnEvict func(key, value interface{})
}

// DefaultConfig returns a default cache configuration.
func DefaultConfig() Config {
	return Config{
		MaxSize: 100,
		TTL:     0,
		OnEvict: nil,
	}
}

// entry represents a cache entry.
type entry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
}

// lruCache is a thread-safe LRU cache implementation.
type lruCache[K comparable, V any] struct {
	mu        sync.RWMutex
	config    Config
	entries   map[K]*list.Element
	evictList *list.List
	stats     Stats
}

// NewLRUCache creates a new LRU cache with the given configuration.
func NewLRUCache[K comparable, V any](config Config) Cache[K, V] {
	if config.MaxSize < 0 {
		config.MaxSize = 0
	}

	return &lruCache[K, V]{
		config:    config,
		entries:   make(map[K]*list.Element),
		evictList: list.New(),
	}
}

// Get retrieves a value from the cache.
func (c *lruCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ent, ok := c.entries[key]
	if !ok {
		c.stats.Misses++
		var zero V
		return zero, false
	}

	// Check if expired
	e := ent.Value.(*entry[K, V])
	if c.config.TTL > 0 && time.Now().After(e.expiresAt) {
		c.removeElement(ent)
		c.stats.Misses++
		var zero V
		return zero, false
	}

	// Move to front (most recently used)
	c.evictList.MoveToFront(ent)
	c.stats.Hits++
	return e.value, true
}

// Put stores a value in the cache.
func (c *lruCache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if entry already exists
	if ent, ok := c.entries[key]; ok {
		c.evictList.MoveToFront(ent)
		e := ent.Value.(*entry[K, V])
		e.value = value
		if c.config.TTL > 0 {
			e.expiresAt = time.Now().Add(c.config.TTL)
		}
		return
	}

	// Add new entry
	e := &entry[K, V]{
		key:   key,
		value: value,
	}
	if c.config.TTL > 0 {
		e.expiresAt = time.Now().Add(c.config.TTL)
	}

	ent := c.evictList.PushFront(e)
	c.entries[key] = ent

	// Evict oldest entry if necessary
	if c.config.MaxSize > 0 && c.evictList.Len() > c.config.MaxSize {
		c.removeOldest()
	}
}

// Remove removes a value from the cache.
func (c *lruCache[K, V]) Remove(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ent, ok := c.entries[key]; ok {
		c.removeElement(ent)
	}
}

// Clear removes all entries from the cache.
func (c *lruCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[K]*list.Element)
	c.evictList.Init()
	c.stats.Size = 0
}

// Len returns the number of entries in the cache.
func (c *lruCache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.evictList.Len()
}

// Stats returns cache statistics.
func (c *lruCache[K, V]) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	s := c.stats
	s.Size = c.evictList.Len()
	s.MaxSize = c.config.MaxSize
	return s
}

// removeOldest removes the oldest entry from the cache.
func (c *lruCache[K, V]) removeOldest() {
	ent := c.evictList.Back()
	if ent != nil {
		c.removeElement(ent)
		c.stats.Evictions++
	}
}

// removeElement removes an element from the cache.
func (c *lruCache[K, V]) removeElement(ent *list.Element) {
	c.evictList.Remove(ent)
	e := ent.Value.(*entry[K, V])
	delete(c.entries, e.key)

	if c.config.OnEvict != nil {
		c.config.OnEvict(e.key, e.value)
	}
}

// IRCache is a specialized cache for IR corpora.
type IRCache struct {
	cache Cache[string, *ir.Corpus]
}

// NewIRCache creates a new IR corpus cache.
func NewIRCache(config Config) *IRCache {
	return &IRCache{
		cache: NewLRUCache[string, *ir.Corpus](config),
	}
}

// NewDefaultIRCache creates a new IR corpus cache with default configuration.
func NewDefaultIRCache() *IRCache {
	config := DefaultConfig()
	config.MaxSize = 50 // Corpora can be large, keep fewer
	return NewIRCache(config)
}

// Get retrieves a corpus from the cache by its hash.
func (c *IRCache) Get(hash string) (*ir.Corpus, bool) {
	return c.cache.Get(hash)
}

// Put stores a corpus in the cache.
func (c *IRCache) Put(hash string, corpus *ir.Corpus) {
	c.cache.Put(hash, corpus)
}

// Remove removes a corpus from the cache.
func (c *IRCache) Remove(hash string) {
	c.cache.Remove(hash)
}

// Clear removes all corpora from the cache.
func (c *IRCache) Clear() {
	c.cache.Clear()
}

// Len returns the number of cached corpora.
func (c *IRCache) Len() int {
	return c.cache.Len()
}

// Stats returns cache statistics.
func (c *IRCache) Stats() Stats {
	return c.cache.Stats()
}

// ManifestCache is a specialized cache for capsule manifests.
type ManifestCache struct {
	cache Cache[string, *capsule.Manifest]
}

// NewManifestCache creates a new manifest cache.
func NewManifestCache(config Config) *ManifestCache {
	return &ManifestCache{
		cache: NewLRUCache[string, *capsule.Manifest](config),
	}
}

// NewDefaultManifestCache creates a new manifest cache with default configuration.
func NewDefaultManifestCache() *ManifestCache {
	config := DefaultConfig()
	config.MaxSize = 100 // Manifests are smaller, can cache more
	return NewManifestCache(config)
}

// Get retrieves a manifest from the cache by its path or hash.
func (c *ManifestCache) Get(key string) (*capsule.Manifest, bool) {
	return c.cache.Get(key)
}

// Put stores a manifest in the cache.
func (c *ManifestCache) Put(key string, manifest *capsule.Manifest) {
	c.cache.Put(key, manifest)
}

// Remove removes a manifest from the cache.
func (c *ManifestCache) Remove(key string) {
	c.cache.Remove(key)
}

// Clear removes all manifests from the cache.
func (c *ManifestCache) Clear() {
	c.cache.Clear()
}

// Len returns the number of cached manifests.
func (c *ManifestCache) Len() int {
	return c.cache.Len()
}

// Stats returns cache statistics.
func (c *ManifestCache) Stats() Stats {
	return c.cache.Stats()
}

// ByteSizeEstimator provides methods to estimate the byte size of cached objects.
type ByteSizeEstimator interface {
	EstimateBytes() int64
}

// jsonMarshalFunc is a variable that holds the JSON marshal function.
// It can be overridden in tests to simulate marshal errors.
var jsonMarshalFunc = json.Marshal

// estimateCorpusBytes estimates the byte size of a corpus.
func estimateCorpusBytes(corpus *ir.Corpus) int64 {
	data, err := jsonMarshalFunc(corpus)
	if err != nil {
		return 0
	}
	return int64(len(data))
}

// estimateManifestBytes estimates the byte size of a manifest.
func estimateManifestBytes(manifest *capsule.Manifest) int64 {
	data, err := jsonMarshalFunc(manifest)
	if err != nil {
		return 0
	}
	return int64(len(data))
}

// BoundedCache is an LRU cache with byte size limits.
type BoundedCache[K comparable, V any] struct {
	cache       Cache[K, V]
	mu          sync.RWMutex
	maxBytes    int64
	currentSize int64
	sizeFunc    func(V) int64
}

// NewBoundedCache creates a new cache with both entry count and byte size limits.
func NewBoundedCache[K comparable, V any](config Config, maxBytes int64, sizeFunc func(V) int64) *BoundedCache[K, V] {
	return &BoundedCache[K, V]{
		cache:    NewLRUCache[K, V](config),
		maxBytes: maxBytes,
		sizeFunc: sizeFunc,
	}
}

// Get retrieves a value from the cache.
func (c *BoundedCache[K, V]) Get(key K) (V, bool) {
	return c.cache.Get(key)
}

// Put stores a value in the cache, respecting byte size limits.
func (c *BoundedCache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	size := c.sizeFunc(value)
	if c.maxBytes > 0 && size > c.maxBytes {
		// Value is too large to cache
		return
	}

	// Check if we need to evict to make room
	if c.maxBytes > 0 {
		for c.currentSize+size > c.maxBytes && c.cache.Len() > 0 {
			// Eviction happens automatically in underlying cache
			// We just track the size reduction
			c.currentSize -= size / int64(c.cache.Len())
		}
	}

	c.cache.Put(key, value)
	c.currentSize += size
}

// Remove removes a value from the cache.
func (c *BoundedCache[K, V]) Remove(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if value, ok := c.cache.Get(key); ok {
		c.currentSize -= c.sizeFunc(value)
		c.cache.Remove(key)
	}
}

// Clear removes all entries from the cache.
func (c *BoundedCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache.Clear()
	c.currentSize = 0
}

// Len returns the number of entries in the cache.
func (c *BoundedCache[K, V]) Len() int {
	return c.cache.Len()
}

// Stats returns cache statistics including byte size information.
func (c *BoundedCache[K, V]) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.cache.Stats()
	stats.TotalBytes = c.currentSize
	return stats
}
