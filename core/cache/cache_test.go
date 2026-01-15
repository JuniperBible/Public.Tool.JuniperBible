package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/FocuswithJustin/JuniperBible/core/capsule"
	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

func TestLRUCache_BasicOperations(t *testing.T) {
	config := Config{
		MaxSize: 3,
		TTL:     0,
	}
	cache := NewLRUCache[string, int](config)

	// Test Put and Get
	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	if v, ok := cache.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a) = %d, %v; want 1, true", v, ok)
	}
	if v, ok := cache.Get("b"); !ok || v != 2 {
		t.Errorf("Get(b) = %d, %v; want 2, true", v, ok)
	}
	if v, ok := cache.Get("c"); !ok || v != 3 {
		t.Errorf("Get(c) = %d, %v; want 3, true", v, ok)
	}

	// Test non-existent key
	if _, ok := cache.Get("d"); ok {
		t.Error("Get(d) should return false")
	}

	// Test Len
	if len := cache.Len(); len != 3 {
		t.Errorf("Len() = %d; want 3", len)
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	config := Config{
		MaxSize: 2,
		TTL:     0,
	}
	cache := NewLRUCache[string, int](config)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3) // Should evict "a" (least recently used)

	// "a" should be evicted
	if _, ok := cache.Get("a"); ok {
		t.Error("Get(a) should return false after eviction")
	}

	// "b" and "c" should still be present
	if v, ok := cache.Get("b"); !ok || v != 2 {
		t.Errorf("Get(b) = %d, %v; want 2, true", v, ok)
	}
	if v, ok := cache.Get("c"); !ok || v != 3 {
		t.Errorf("Get(c) = %d, %v; want 3, true", v, ok)
	}

	// Test that accessing moves to front
	cache.Get("b")    // Move "b" to front
	cache.Put("d", 4) // Should evict "c" (now least recently used)

	if _, ok := cache.Get("c"); ok {
		t.Error("Get(c) should return false after eviction")
	}
	if v, ok := cache.Get("b"); !ok || v != 2 {
		t.Errorf("Get(b) = %d, %v; want 2, true", v, ok)
	}
	if v, ok := cache.Get("d"); !ok || v != 4 {
		t.Errorf("Get(d) = %d, %v; want 4, true", v, ok)
	}
}

func TestLRUCache_Update(t *testing.T) {
	config := Config{
		MaxSize: 2,
		TTL:     0,
	}
	cache := NewLRUCache[string, int](config)

	cache.Put("a", 1)
	cache.Put("a", 2) // Update existing key

	if v, ok := cache.Get("a"); !ok || v != 2 {
		t.Errorf("Get(a) = %d, %v; want 2, true", v, ok)
	}

	// Should still have only 1 entry
	if len := cache.Len(); len != 1 {
		t.Errorf("Len() = %d; want 1", len)
	}
}

func TestLRUCache_Remove(t *testing.T) {
	config := Config{
		MaxSize: 3,
		TTL:     0,
	}
	cache := NewLRUCache[string, int](config)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	cache.Remove("b")

	if _, ok := cache.Get("b"); ok {
		t.Error("Get(b) should return false after Remove")
	}

	if len := cache.Len(); len != 2 {
		t.Errorf("Len() = %d; want 2", len)
	}

	// Other entries should still be present
	if v, ok := cache.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a) = %d, %v; want 1, true", v, ok)
	}
	if v, ok := cache.Get("c"); !ok || v != 3 {
		t.Errorf("Get(c) = %d, %v; want 3, true", v, ok)
	}
}

func TestLRUCache_Clear(t *testing.T) {
	config := Config{
		MaxSize: 3,
		TTL:     0,
	}
	cache := NewLRUCache[string, int](config)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	cache.Clear()

	if len := cache.Len(); len != 0 {
		t.Errorf("Len() = %d; want 0", len)
	}

	if _, ok := cache.Get("a"); ok {
		t.Error("Get(a) should return false after Clear")
	}
}

func TestLRUCache_TTL(t *testing.T) {
	config := Config{
		MaxSize: 3,
		TTL:     50 * time.Millisecond,
	}
	cache := NewLRUCache[string, int](config)

	cache.Put("a", 1)

	// Should be present immediately
	if v, ok := cache.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a) = %d, %v; want 1, true", v, ok)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	if _, ok := cache.Get("a"); ok {
		t.Error("Get(a) should return false after TTL expiration")
	}
}

func TestLRUCache_Stats(t *testing.T) {
	config := Config{
		MaxSize: 2,
		TTL:     0,
	}
	cache := NewLRUCache[string, int](config)

	cache.Put("a", 1)
	cache.Put("b", 2)

	// Test hits
	cache.Get("a")
	cache.Get("b")

	// Test misses
	cache.Get("c")
	cache.Get("d")

	// Test eviction
	cache.Put("c", 3) // Evicts "a"

	stats := cache.Stats()

	if stats.Hits != 2 {
		t.Errorf("Hits = %d; want 2", stats.Hits)
	}
	if stats.Misses != 2 {
		t.Errorf("Misses = %d; want 2", stats.Misses)
	}
	if stats.Evictions != 1 {
		t.Errorf("Evictions = %d; want 1", stats.Evictions)
	}
	if stats.Size != 2 {
		t.Errorf("Size = %d; want 2", stats.Size)
	}
	if stats.MaxSize != 2 {
		t.Errorf("MaxSize = %d; want 2", stats.MaxSize)
	}
}

func TestLRUCache_OnEvict(t *testing.T) {
	var evictedKey string
	var evictedValue int

	config := Config{
		MaxSize: 2,
		TTL:     0,
		OnEvict: func(key, value interface{}) {
			evictedKey = key.(string)
			evictedValue = value.(int)
		},
	}
	cache := NewLRUCache[string, int](config)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3) // Should evict "a"

	if evictedKey != "a" {
		t.Errorf("evictedKey = %s; want a", evictedKey)
	}
	if evictedValue != 1 {
		t.Errorf("evictedValue = %d; want 1", evictedValue)
	}
}

func TestLRUCache_Concurrency(t *testing.T) {
	config := Config{
		MaxSize: 100,
		TTL:     0,
	}
	cache := NewLRUCache[int, int](config)

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := id*numOperations + j
				cache.Put(key, key)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := id*numOperations + j
				cache.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Cache should be in a valid state
	if len := cache.Len(); len > config.MaxSize {
		t.Errorf("Len() = %d; want <= %d", len, config.MaxSize)
	}
}

func TestIRCache_BasicOperations(t *testing.T) {
	cache := NewDefaultIRCache()

	corpus := &ir.Corpus{
		ID:            "TEST",
		Version:       "1.0.0",
		ModuleType:    ir.ModuleBible,
		Versification: "KJV",
		Language:      "en",
		Title:         "Test Bible",
	}

	hash := "abc123"

	// Test Put and Get
	cache.Put(hash, corpus)

	retrieved, ok := cache.Get(hash)
	if !ok {
		t.Error("Get should return true for cached corpus")
	}
	if retrieved.ID != corpus.ID {
		t.Errorf("Retrieved corpus ID = %s; want %s", retrieved.ID, corpus.ID)
	}

	// Test Len
	if len := cache.Len(); len != 1 {
		t.Errorf("Len() = %d; want 1", len)
	}

	// Test Remove
	cache.Remove(hash)
	if _, ok := cache.Get(hash); ok {
		t.Error("Get should return false after Remove")
	}
}

func TestIRCache_MultipleCorpora(t *testing.T) {
	cache := NewDefaultIRCache()

	for i := 0; i < 10; i++ {
		corpus := &ir.Corpus{
			ID:            string(rune('A' + i)),
			Version:       "1.0.0",
			ModuleType:    ir.ModuleBible,
			Versification: "KJV",
			Language:      "en",
		}
		hash := string(rune('a' + i))
		cache.Put(hash, corpus)
	}

	if len := cache.Len(); len != 10 {
		t.Errorf("Len() = %d; want 10", len)
	}

	// Verify all corpora are retrievable
	for i := 0; i < 10; i++ {
		hash := string(rune('a' + i))
		corpus, ok := cache.Get(hash)
		if !ok {
			t.Errorf("Get(%s) should return true", hash)
		}
		expectedID := string(rune('A' + i))
		if corpus.ID != expectedID {
			t.Errorf("Corpus ID = %s; want %s", corpus.ID, expectedID)
		}
	}
}

func TestManifestCache_BasicOperations(t *testing.T) {
	cache := NewDefaultManifestCache()

	manifest := &capsule.Manifest{
		CapsuleVersion: "1.0.0",
		CreatedAt:      "2024-01-01T00:00:00Z",
		Tool: capsule.ToolInfo{
			Name:    "capsule",
			Version: "1.0.0",
		},
	}

	key := "/path/to/manifest.json"

	// Test Put and Get
	cache.Put(key, manifest)

	retrieved, ok := cache.Get(key)
	if !ok {
		t.Error("Get should return true for cached manifest")
	}
	if retrieved.CapsuleVersion != manifest.CapsuleVersion {
		t.Errorf("Retrieved manifest version = %s; want %s", retrieved.CapsuleVersion, manifest.CapsuleVersion)
	}

	// Test Len
	if len := cache.Len(); len != 1 {
		t.Errorf("Len() = %d; want 1", len)
	}

	// Test Remove
	cache.Remove(key)
	if _, ok := cache.Get(key); ok {
		t.Error("Get should return false after Remove")
	}
}

func TestManifestCache_MultipleManifests(t *testing.T) {
	cache := NewDefaultManifestCache()

	for i := 0; i < 20; i++ {
		manifest := &capsule.Manifest{
			CapsuleVersion: "1.0.0",
			CreatedAt:      "2024-01-01T00:00:00Z",
			Tool: capsule.ToolInfo{
				Name:    "capsule",
				Version: "1.0.0",
			},
		}
		manifest.Tool.Attributes = capsule.Attributes{
			"index": i,
		}
		key := string(rune('a' + i))
		cache.Put(key, manifest)
	}

	if len := cache.Len(); len != 20 {
		t.Errorf("Len() = %d; want 20", len)
	}

	// Verify all manifests are retrievable
	for i := 0; i < 20; i++ {
		key := string(rune('a' + i))
		manifest, ok := cache.Get(key)
		if !ok {
			t.Errorf("Get(%s) should return true", key)
		}
		idx, ok := manifest.Tool.Attributes["index"].(int)
		if !ok || idx != i {
			t.Errorf("Manifest index = %v; want %d", manifest.Tool.Attributes["index"], i)
		}
	}
}

func TestBoundedCache_ByteLimit(t *testing.T) {
	config := Config{
		MaxSize: 100,
		TTL:     0,
	}

	sizeFunc := func(s string) int64 {
		return int64(len(s))
	}

	cache := NewBoundedCache[string, string](config, 100, sizeFunc)

	// Add strings totaling 100 bytes
	cache.Put("a", "12345678901234567890") // 20 bytes
	cache.Put("b", "12345678901234567890") // 20 bytes
	cache.Put("c", "12345678901234567890") // 20 bytes
	cache.Put("d", "12345678901234567890") // 20 bytes
	cache.Put("e", "12345678901234567890") // 20 bytes

	stats := cache.Stats()
	if stats.Size < 1 {
		t.Errorf("Size = %d; want > 0", stats.Size)
	}

	// Try to add a value that's too large
	cache.Put("f", string(make([]byte, 200)))
	if _, ok := cache.Get("f"); ok {
		t.Error("Oversized value should not be cached")
	}
}

func TestBoundedCache_Stats(t *testing.T) {
	config := Config{
		MaxSize: 10,
		TTL:     0,
	}

	sizeFunc := func(s string) int64 {
		return int64(len(s))
	}

	cache := NewBoundedCache[string, string](config, 1000, sizeFunc)

	cache.Put("a", "hello")
	cache.Put("b", "world")

	stats := cache.Stats()

	if stats.TotalBytes <= 0 {
		t.Errorf("TotalBytes = %d; want > 0", stats.TotalBytes)
	}
}

func TestEstimateCorpusBytes(t *testing.T) {
	corpus := &ir.Corpus{
		ID:            "TEST",
		Version:       "1.0.0",
		ModuleType:    ir.ModuleBible,
		Versification: "KJV",
		Language:      "en",
		Title:         "Test Bible",
		Documents: []*ir.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
			},
		},
	}

	size := estimateCorpusBytes(corpus)
	if size <= 0 {
		t.Errorf("estimateCorpusBytes = %d; want > 0", size)
	}
}

func TestEstimateManifestBytes(t *testing.T) {
	manifest := &capsule.Manifest{
		CapsuleVersion: "1.0.0",
		CreatedAt:      "2024-01-01T00:00:00Z",
		Tool: capsule.ToolInfo{
			Name:    "capsule",
			Version: "1.0.0",
		},
		Blobs: capsule.BlobIndex{
			BySHA256: make(map[string]*capsule.BlobRecord),
		},
	}

	size := estimateManifestBytes(manifest)
	if size <= 0 {
		t.Errorf("estimateManifestBytes = %d; want > 0", size)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxSize != 100 {
		t.Errorf("DefaultConfig.MaxSize = %d; want 100", config.MaxSize)
	}
	if config.TTL != 0 {
		t.Errorf("DefaultConfig.TTL = %v; want 0", config.TTL)
	}
	if config.OnEvict != nil {
		t.Error("DefaultConfig.OnEvict should be nil")
	}
}

func TestLRUCache_UnlimitedSize(t *testing.T) {
	config := Config{
		MaxSize: 0, // Unlimited
		TTL:     0,
	}
	cache := NewLRUCache[string, int](config)

	// Add many entries
	for i := 0; i < 1000; i++ {
		cache.Put(fmt.Sprintf("%c%d", rune('a'+i%26), i), i)
	}

	// All should be present (no eviction)
	if len := cache.Len(); len != 1000 {
		t.Errorf("Len() = %d; want 1000", len)
	}
}

func BenchmarkLRUCache_Put(b *testing.B) {
	config := Config{
		MaxSize: 100,
		TTL:     0,
	}
	cache := NewLRUCache[int, int](config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(i, i)
	}
}

func BenchmarkLRUCache_Get(b *testing.B) {
	config := Config{
		MaxSize: 100,
		TTL:     0,
	}
	cache := NewLRUCache[int, int](config)

	// Populate cache
	for i := 0; i < 100; i++ {
		cache.Put(i, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(i % 100)
	}
}

func BenchmarkLRUCache_PutGet(b *testing.B) {
	config := Config{
		MaxSize: 100,
		TTL:     0,
	}
	cache := NewLRUCache[int, int](config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(i, i)
		cache.Get(i)
	}
}

// TestIRCache_ClearAndStats tests Clear and Stats methods on IRCache.
func TestIRCache_ClearAndStats(t *testing.T) {
	cache := NewDefaultIRCache()

	corpus := &ir.Corpus{
		ID:            "TEST",
		Version:       "1.0.0",
		ModuleType:    ir.ModuleBible,
		Versification: "KJV",
		Language:      "en",
		Title:         "Test Bible",
	}

	// Add corpora
	cache.Put("hash1", corpus)
	cache.Put("hash2", corpus)
	cache.Put("hash3", corpus)

	// Test Stats
	stats := cache.Stats()
	if stats.Size != 3 {
		t.Errorf("Stats.Size = %d; want 3", stats.Size)
	}

	// Test Clear
	cache.Clear()

	if len := cache.Len(); len != 0 {
		t.Errorf("Len() after Clear = %d; want 0", len)
	}

	// Stats after clear should show 0 size
	stats = cache.Stats()
	if stats.Size != 0 {
		t.Errorf("Stats.Size after Clear = %d; want 0", stats.Size)
	}
}

// TestManifestCache_ClearAndStats tests Clear and Stats methods on ManifestCache.
func TestManifestCache_ClearAndStats(t *testing.T) {
	cache := NewDefaultManifestCache()

	manifest := &capsule.Manifest{
		CapsuleVersion: "1.0.0",
		CreatedAt:      "2024-01-01T00:00:00Z",
		Tool: capsule.ToolInfo{
			Name:    "capsule",
			Version: "1.0.0",
		},
	}

	// Add manifests
	cache.Put("key1", manifest)
	cache.Put("key2", manifest)
	cache.Put("key3", manifest)

	// Test Stats
	stats := cache.Stats()
	if stats.Size != 3 {
		t.Errorf("Stats.Size = %d; want 3", stats.Size)
	}

	// Test Clear
	cache.Clear()

	if len := cache.Len(); len != 0 {
		t.Errorf("Len() after Clear = %d; want 0", len)
	}

	// Stats after clear should show 0 size
	stats = cache.Stats()
	if stats.Size != 0 {
		t.Errorf("Stats.Size after Clear = %d; want 0", stats.Size)
	}
}

// TestBoundedCache_RemoveClearLen tests Remove, Clear, and Len methods on BoundedCache.
func TestBoundedCache_RemoveClearLen(t *testing.T) {
	config := Config{
		MaxSize: 100,
		TTL:     0,
	}

	sizeFunc := func(s string) int64 {
		return int64(len(s))
	}

	cache := NewBoundedCache[string, string](config, 1000, sizeFunc)

	// Add entries
	cache.Put("a", "hello")
	cache.Put("b", "world")
	cache.Put("c", "test")

	// Test Len
	if len := cache.Len(); len != 3 {
		t.Errorf("Len() = %d; want 3", len)
	}

	// Test Remove
	cache.Remove("b")
	if _, ok := cache.Get("b"); ok {
		t.Error("Get(b) should return false after Remove")
	}
	if len := cache.Len(); len != 2 {
		t.Errorf("Len() after Remove = %d; want 2", len)
	}

	// Test Clear
	cache.Clear()
	if len := cache.Len(); len != 0 {
		t.Errorf("Len() after Clear = %d; want 0", len)
	}

	// Verify all entries are gone
	if _, ok := cache.Get("a"); ok {
		t.Error("Get(a) should return false after Clear")
	}
	if _, ok := cache.Get("c"); ok {
		t.Error("Get(c) should return false after Clear")
	}

	// Stats should show 0 bytes
	stats := cache.Stats()
	if stats.TotalBytes != 0 {
		t.Errorf("Stats.TotalBytes after Clear = %d; want 0", stats.TotalBytes)
	}
}

// TestNewLRUCache_NegativeMaxSize tests NewLRUCache with negative MaxSize.
func TestNewLRUCache_NegativeMaxSize(t *testing.T) {
	config := Config{
		MaxSize: -1,
		TTL:     0,
	}
	cache := NewLRUCache[string, int](config)

	// Add many entries - should not evict (MaxSize normalized to 0 = unlimited)
	for i := 0; i < 100; i++ {
		cache.Put(fmt.Sprintf("key%d", i), i)
	}

	if len := cache.Len(); len != 100 {
		t.Errorf("Len() = %d; want 100", len)
	}
}

// TestEstimateCorpusBytesNil tests estimateCorpusBytes with nil.
func TestEstimateCorpusBytesNil(t *testing.T) {
	size := estimateCorpusBytes(nil)
	// nil marshals to "null" which is 4 bytes, but let's just check it doesn't panic
	if size < 0 {
		t.Errorf("estimateCorpusBytes(nil) = %d; want >= 0", size)
	}
}

// TestEstimateManifestBytesNil tests estimateManifestBytes with nil.
func TestEstimateManifestBytesNil(t *testing.T) {
	size := estimateManifestBytes(nil)
	// nil marshals to "null" which is 4 bytes, but let's just check it doesn't panic
	if size < 0 {
		t.Errorf("estimateManifestBytes(nil) = %d; want >= 0", size)
	}
}

// TestBoundedCache_RemoveNonexistent tests removing a key that doesn't exist.
func TestBoundedCache_RemoveNonexistent(t *testing.T) {
	config := Config{
		MaxSize: 100,
		TTL:     0,
	}

	sizeFunc := func(s string) int64 {
		return int64(len(s))
	}

	cache := NewBoundedCache[string, string](config, 1000, sizeFunc)

	// Add one entry
	cache.Put("a", "hello")

	// Remove non-existent key - should not panic
	cache.Remove("nonexistent")

	// Original entry should still exist
	if _, ok := cache.Get("a"); !ok {
		t.Error("Get(a) should return true after removing nonexistent key")
	}
}

// TestLRUCache_UpdateWithTTL tests updating an entry with TTL.
func TestLRUCache_UpdateWithTTL(t *testing.T) {
	config := Config{
		MaxSize: 10,
		TTL:     time.Hour, // Long TTL so it won't expire during test
	}
	cache := NewLRUCache[string, int](config)

	// Add entry
	cache.Put("a", 1)

	// Update entry (should refresh expiration time)
	cache.Put("a", 2)

	if v, ok := cache.Get("a"); !ok || v != 2 {
		t.Errorf("Get(a) = %d, %v; want 2, true", v, ok)
	}
}

// TestBoundedCache_ByteLimitEviction tests the eviction loop in BoundedCache.Put.
func TestBoundedCache_ByteLimitEviction(t *testing.T) {
	config := Config{
		MaxSize: 100,
		TTL:     0,
	}

	sizeFunc := func(s string) int64 {
		return int64(len(s))
	}

	// Create cache with 50 byte limit
	cache := NewBoundedCache[string, string](config, 50, sizeFunc)

	// Add entries until we're near the limit
	cache.Put("a", "1234567890") // 10 bytes
	cache.Put("b", "1234567890") // 10 bytes
	cache.Put("c", "1234567890") // 10 bytes
	cache.Put("d", "1234567890") // 10 bytes

	// At this point we have ~40 bytes
	// Adding another entry should trigger the eviction loop
	cache.Put("e", "12345678901234567890") // 20 bytes - would exceed 50 byte limit

	// Cache should have evicted some entries to make room
	// Total can't exceed maxBytes (50)
	stats := cache.Stats()
	if stats.Size < 1 {
		t.Errorf("Stats.Size = %d; want > 0", stats.Size)
	}
}

// TestEstimateCorpusBytesWithLargeData tests estimateCorpusBytes with larger data.
func TestEstimateCorpusBytesWithLargeData(t *testing.T) {
	// Create a corpus with more data to test a different code path
	corpus := &ir.Corpus{
		ID:            "TEST",
		Version:       "1.0.0",
		ModuleType:    ir.ModuleBible,
		Versification: "KJV",
		Language:      "en",
		Title:         "Test Bible",
		Description:   "A test bible with extra data",
		Publisher:     "Test Publisher",
		Rights:        "Public Domain",
		SourceFormat:  "USFM",
		Documents: []*ir.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
			},
			{
				ID:    "Exo",
				Title: "Exodus",
				Order: 2,
			},
		},
	}

	// Test that it returns a reasonable size
	size := estimateCorpusBytes(corpus)
	if size <= 0 {
		t.Errorf("estimateCorpusBytes = %d; want > 0", size)
	}
}

// TestEstimateManifestBytesWithLargeData tests estimateManifestBytes with larger data.
func TestEstimateManifestBytesWithLargeData(t *testing.T) {
	// Create a manifest with more data
	manifest := &capsule.Manifest{
		CapsuleVersion: "1.0.0",
		CreatedAt:      "2024-01-01T00:00:00Z",
		Tool: capsule.ToolInfo{
			Name:    "capsule",
			Version: "1.0.0",
			Attributes: capsule.Attributes{
				"key1": "value1",
				"key2": "value2",
			},
		},
		Blobs: capsule.BlobIndex{
			BySHA256: map[string]*capsule.BlobRecord{
				"abc123": {
					SHA256:    "abc123",
					SizeBytes: 1000,
					Path:      "/path/to/blob",
					MIME:      "application/json",
				},
			},
		},
	}

	// Test that it returns a reasonable size
	size := estimateManifestBytes(manifest)
	if size <= 0 {
		t.Errorf("estimateManifestBytes = %d; want > 0", size)
	}
}

// TestLRUCache_RemoveNonexistent tests removing a non-existent key.
func TestLRUCache_RemoveNonexistent(t *testing.T) {
	config := Config{
		MaxSize: 10,
		TTL:     0,
	}
	cache := NewLRUCache[string, int](config)

	cache.Put("a", 1)

	// Remove non-existent key - should not panic
	cache.Remove("nonexistent")

	// Original entry should still exist
	if v, ok := cache.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a) = %d, %v; want 1, true after removing nonexistent key", v, ok)
	}
}

// TestEstimateCorpusBytes_MarshalError tests the error path when json.Marshal fails.
func TestEstimateCorpusBytes_MarshalError(t *testing.T) {
	// Save original function and restore after test
	originalFunc := jsonMarshalFunc
	defer func() { jsonMarshalFunc = originalFunc }()

	// Override jsonMarshalFunc to return an error
	jsonMarshalFunc = func(v interface{}) ([]byte, error) {
		return nil, fmt.Errorf("simulated marshal error")
	}

	corpus := &ir.Corpus{
		ID:      "TEST",
		Version: "1.0.0",
	}

	// Should return 0 when marshal fails
	size := estimateCorpusBytes(corpus)
	if size != 0 {
		t.Errorf("estimateCorpusBytes with marshal error = %d; want 0", size)
	}
}

// TestEstimateManifestBytes_MarshalError tests the error path when json.Marshal fails.
func TestEstimateManifestBytes_MarshalError(t *testing.T) {
	// Save original function and restore after test
	originalFunc := jsonMarshalFunc
	defer func() { jsonMarshalFunc = originalFunc }()

	// Override jsonMarshalFunc to return an error
	jsonMarshalFunc = func(v interface{}) ([]byte, error) {
		return nil, fmt.Errorf("simulated marshal error")
	}

	manifest := &capsule.Manifest{
		CapsuleVersion: "1.0.0",
	}

	// Should return 0 when marshal fails
	size := estimateManifestBytes(manifest)
	if size != 0 {
		t.Errorf("estimateManifestBytes with marshal error = %d; want 0", size)
	}
}
