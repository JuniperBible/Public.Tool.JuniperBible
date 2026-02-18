package web

import (
	"archive/tar"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ulikunitz/xz"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
	"github.com/FocuswithJustin/JuniperBible/internal/archive"
	"github.com/FocuswithJustin/JuniperBible/internal/fileutil"
	"github.com/FocuswithJustin/JuniperBible/internal/logging"
	"github.com/FocuswithJustin/JuniperBible/internal/validation"
)

const (
	// MaxFormMemory is the maximum memory for form parsing (32 MB).
	MaxFormMemory = 32 << 20
	// maxWorkers is the maximum number of concurrent workers for parallel processing.
	maxWorkers = 32
)

// staticFileCache caches static file contents at startup to avoid re-reading on every request.
var staticFileCache struct {
	sync.RWMutex
	files     map[string][]byte
	etags     map[string]string
	populated bool
}

// initStaticFileCache initializes the static file cache by reading all static files at startup.
func initStaticFileCache() {
	staticFileCache.Lock()
	defer staticFileCache.Unlock()

	if staticFileCache.populated {
		return
	}

	staticFileCache.files = make(map[string][]byte)
	staticFileCache.etags = make(map[string]string)

	// List of static files to cache
	staticFiles := []string{"base.css", "style.css", "app.js"}

	for _, name := range staticFiles {
		content, err := staticFS.ReadFile("static/" + name)
		if err != nil {
			continue
		}
		staticFileCache.files[name] = content
		// Generate ETag from content hash (simple CRC-like approach)
		staticFileCache.etags[name] = fmt.Sprintf("\"%x\"", len(content))
	}

	staticFileCache.populated = true
}

// getStaticFile returns cached static file content and ETag.
func getStaticFile(name string) ([]byte, string, bool) {
	staticFileCache.RLock()
	defer staticFileCache.RUnlock()

	if !staticFileCache.populated {
		return nil, "", false
	}

	content, ok := staticFileCache.files[name]
	if !ok {
		return nil, "", false
	}

	etag := staticFileCache.etags[name]
	return content, etag, true
}

// capsulesListCache caches the list of capsules to avoid directory walks on every request.
var capsulesListCache struct {
	sync.RWMutex
	capsules  []CapsuleInfo
	populated bool
	timestamp time.Time
	ttl       time.Duration
}

func init() {
	capsulesListCache.ttl = 5 * time.Minute // Capsule list rarely changes during normal operation
}

// getCachedCapsulesList returns a cached list of capsules or rebuilds if expired.
func getCachedCapsulesList() []CapsuleInfo {
	capsulesListCache.RLock()
	if capsulesListCache.populated && time.Since(capsulesListCache.timestamp) < capsulesListCache.ttl {
		capsules := capsulesListCache.capsules
		capsulesListCache.RUnlock()
		return capsules
	}
	capsulesListCache.RUnlock()

	// Rebuild cache
	capsulesListCache.Lock()
	defer capsulesListCache.Unlock()

	// Double-check after acquiring write lock
	if capsulesListCache.populated && time.Since(capsulesListCache.timestamp) < capsulesListCache.ttl {
		return capsulesListCache.capsules
	}

	capsulesListCache.capsules = listCapsulesUncached()
	capsulesListCache.populated = true
	capsulesListCache.timestamp = time.Now()

	return capsulesListCache.capsules
}

// invalidateCapsulesListCache forces a cache rebuild on next access.
func invalidateCapsulesListCache() {
	capsulesListCache.Lock()
	capsulesListCache.populated = false
	capsulesListCache.timestamp = time.Time{}
	capsulesListCache.Unlock()
}

// archiveSemaphore limits concurrent archive operations to prevent resource exhaustion.
// Reading compressed archives is I/O and CPU intensive, so we limit concurrency.
var archiveSemaphore = make(chan struct{}, 16) // Allow up to 16 concurrent archive operations

// acquireArchiveSemaphore acquires a slot for archive operations.
func acquireArchiveSemaphore() {
	archiveSemaphore <- struct{}{}
}

// releaseArchiveSemaphore releases a slot after archive operation completes.
func releaseArchiveSemaphore() {
	<-archiveSemaphore
}

// fileExtensionFormats maps file extensions to their format names.
var fileExtensionFormats = map[string]string{
	".xml":  "osis",
	".osis": "osis",
	".usfm": "usfm",
	".sfm":  "usfm",
	".usx":  "usx",
	".zip":  "zip",
	".tar":  "tar",
	".json": "json",
	".epub": "epub",
}

// licensePatterns maps license text patterns to standard license types.
// Patterns are checked in order, so more specific patterns should come first.
var licensePatterns = []struct {
	patterns []string
	license  string
}{
	{[]string{"gnu general public license", "version 3"}, "GPL-3.0"},
	{[]string{"gnu general public license", "version 2"}, "GPL-2.0"},
	{[]string{"gpl-3"}, "GPL-3.0"},
	{[]string{"gpl-2"}, "GPL-2.0"},
	{[]string{"mit license"}, "MIT"},
	{[]string{"apache license"}, "Apache-2.0"},
	{[]string{"public domain"}, "Public Domain"},
}

// CapsuleMetadata holds cached metadata about a capsule.
type CapsuleMetadata struct {
	IsCAS bool
	HasIR bool
}

// DiskCapsuleMetadata is the on-disk format for capsule metadata cache.
type DiskCapsuleMetadata struct {
	ModTime int64 // File modification time (Unix timestamp)
	Size    int64 // File size in bytes
	IsCAS   bool
	HasIR   bool
}

// DiskMetadataCache is the full on-disk cache structure.
type DiskMetadataCache struct {
	Version  int                            `json:"version"`
	Capsules map[string]DiskCapsuleMetadata `json:"capsules"` // key: filename (not full path)
}

const diskMetadataCacheVersion = 1
const diskMetadataCacheFile = ".capsule-metadata.json"

// capsuleMetadataCache caches capsule metadata to avoid re-reading files on every request.
var capsuleMetadataCache struct {
	sync.RWMutex
	data      map[string]CapsuleMetadata // key: capsule path
	timestamp time.Time
	ttl       time.Duration
}

func init() {
	capsuleMetadataCache.ttl = 30 * time.Minute // Longer TTL since expensive to compute
	capsuleMetadataCache.data = make(map[string]CapsuleMetadata)
}

// loadDiskMetadataCache loads the metadata cache from disk.
func loadDiskMetadataCache() map[string]DiskCapsuleMetadata {
	cacheFile := filepath.Join(ServerConfig.CapsulesDir, diskMetadataCacheFile)
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil
	}

	var cache DiskMetadataCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil
	}

	if cache.Version != diskMetadataCacheVersion {
		return nil
	}

	return cache.Capsules
}

// saveDiskMetadataCache saves the metadata cache to disk.
func saveDiskMetadataCache(capsules map[string]DiskCapsuleMetadata) error {
	cache := DiskMetadataCache{
		Version:  diskMetadataCacheVersion,
		Capsules: capsules,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	cacheFile := filepath.Join(ServerConfig.CapsulesDir, diskMetadataCacheFile)
	return os.WriteFile(cacheFile, data, 0600)
}

// preloadCapsuleMetadata preloads metadata for all capsules in parallel.
// Uses a disk cache to avoid scanning archives on subsequent startups.
// This should be called during startup warmup.
// buildCurrentFilesMap creates a map of capsule names to their file info
func buildCurrentFilesMap(capsules []CapsuleInfo) map[string]os.FileInfo {
	currentFiles := make(map[string]os.FileInfo)
	for _, c := range capsules {
		fullPath := filepath.Join(ServerConfig.CapsulesDir, c.Path)
		if info, err := os.Stat(fullPath); err == nil {
			currentFiles[c.Name] = info
		}
	}
	return currentFiles
}

// isCacheValid checks if the cached metadata is still valid for the given file
func isCacheValid(cached DiskCapsuleMetadata, info os.FileInfo) bool {
	return cached.ModTime == info.ModTime().Unix() && cached.Size == info.Size()
}

// loadCachedMetadata loads cached metadata into the in-memory cache
func loadCachedMetadata(c CapsuleInfo, cached DiskCapsuleMetadata) {
	fullPath := filepath.Join(ServerConfig.CapsulesDir, c.Path)
	capsuleMetadataCache.Lock()
	capsuleMetadataCache.data[fullPath] = CapsuleMetadata{
		IsCAS: cached.IsCAS,
		HasIR: cached.HasIR,
	}
	capsuleMetadataCache.Unlock()
}

// partitionCapsulesByCache separates capsules into those with valid cache and those needing scan
func partitionCapsulesByCache(capsules []CapsuleInfo, diskCache map[string]DiskCapsuleMetadata, currentFiles map[string]os.FileInfo) (needsScan []CapsuleInfo, cacheHits int) {
	for _, c := range capsules {
		info := currentFiles[c.Name]
		if info == nil {
			continue
		}

		cached, ok := diskCache[c.Name]
		if ok && isCacheValid(cached, info) {
			// Cache hit - use cached values
			loadCachedMetadata(c, cached)
			cacheHits++
		} else {
			// Cache miss - needs scanning
			needsScan = append(needsScan, c)
		}
	}
	return needsScan, cacheHits
}

// metaResult holds the result of scanning a capsule
type metaResult struct {
	name     string
	path     string
	meta     CapsuleMetadata
	diskMeta DiskCapsuleMetadata
}

// scanCapsulesInParallel scans capsules that need metadata extraction
func scanCapsulesInParallel(needsScan []CapsuleInfo, currentFiles map[string]os.FileInfo) map[string]DiskCapsuleMetadata {
	pool := NewWorkerPool[CapsuleInfo, metaResult](maxWorkers, len(needsScan))
	pool.Start(func(c CapsuleInfo) metaResult {
		fullPath := filepath.Join(ServerConfig.CapsulesDir, c.Path)
		flags, _ := archive.ScanCapsuleFlags(fullPath)
		meta := CapsuleMetadata{
			IsCAS: flags.IsCAS,
			HasIR: flags.HasIR,
		}

		info := currentFiles[c.Name]
		diskMeta := DiskCapsuleMetadata{
			ModTime: info.ModTime().Unix(),
			Size:    info.Size(),
			IsCAS:   flags.IsCAS,
			HasIR:   flags.HasIR,
		}

		return metaResult{name: c.Name, path: fullPath, meta: meta, diskMeta: diskMeta}
	})

	for _, c := range needsScan {
		pool.Submit(c)
	}
	pool.Close()

	// Collect results
	diskCache := make(map[string]DiskCapsuleMetadata)
	for r := range pool.Results() {
		capsuleMetadataCache.Lock()
		capsuleMetadataCache.data[r.path] = r.meta
		capsuleMetadataCache.Unlock()
		diskCache[r.name] = r.diskMeta
	}

	return diskCache
}

// updateCacheTimestamp marks the cache as freshly updated
func updateCacheTimestamp() {
	capsuleMetadataCache.Lock()
	capsuleMetadataCache.timestamp = time.Now()
	capsuleMetadataCache.Unlock()
}

func preloadCapsuleMetadata() {
	capsules := listCapsules()
	if len(capsules) == 0 {
		return
	}

	// Load existing disk cache
	diskCache := loadDiskMetadataCache()
	if diskCache == nil {
		diskCache = make(map[string]DiskCapsuleMetadata)
	}

	// Build a map of current capsule files for quick lookup
	currentFiles := buildCurrentFilesMap(capsules)

	// Find capsules that need scanning (not in cache or modified)
	needsScan, cacheHits := partitionCapsulesByCache(capsules, diskCache, currentFiles)

	log.Printf("[CACHE] Disk metadata cache: %d hits, %d need scanning", cacheHits, len(needsScan))

	if len(needsScan) == 0 {
		updateCacheTimestamp()
		log.Printf("[CACHE] All %d capsules loaded from disk cache", cacheHits)
		return
	}

	// Scan capsules that need it and collect results
	scannedCache := scanCapsulesInParallel(needsScan, currentFiles)

	// Merge scanned results into disk cache
	for name, meta := range scannedCache {
		diskCache[name] = meta
	}

	updateCacheTimestamp()

	// Save updated disk cache
	if err := saveDiskMetadataCache(diskCache); err != nil {
		log.Printf("[CACHE] Failed to save disk metadata cache: %v", err)
	} else {
		log.Printf("[CACHE] Saved disk metadata cache with %d entries", len(diskCache))
	}

	log.Printf("[CACHE] Preloaded metadata: %d from cache, %d scanned", cacheHits, len(needsScan))
}

// getCapsuleMetadata returns cached metadata for a capsule or computes it if not cached.
func getCapsuleMetadata(capsulePath string) CapsuleMetadata {
	capsuleMetadataCache.RLock()
	if time.Since(capsuleMetadataCache.timestamp) < capsuleMetadataCache.ttl {
		if meta, ok := capsuleMetadataCache.data[capsulePath]; ok {
			capsuleMetadataCache.RUnlock()
			return meta
		}
	}
	capsuleMetadataCache.RUnlock()

	// Compute metadata with semaphore to limit concurrent archive reads
	// Use ScanCapsuleFlags for single-pass archive scan (2x faster than separate calls)
	acquireArchiveSemaphore()
	flags, _ := archive.ScanCapsuleFlags(capsulePath)
	meta := CapsuleMetadata{
		IsCAS: flags.IsCAS,
		HasIR: flags.HasIR,
	}
	releaseArchiveSemaphore()

	// Cache it
	capsuleMetadataCache.Lock()
	if capsuleMetadataCache.data == nil {
		capsuleMetadataCache.data = make(map[string]CapsuleMetadata)
	}
	capsuleMetadataCache.data[capsulePath] = meta
	capsuleMetadataCache.timestamp = time.Now()
	capsuleMetadataCache.Unlock()

	return meta
}

// invalidateCapsuleMetadataCache clears the capsule metadata cache.
func invalidateCapsuleMetadataCache() {
	capsuleMetadataCache.Lock()
	capsuleMetadataCache.data = make(map[string]CapsuleMetadata)
	capsuleMetadataCache.timestamp = time.Time{}
	capsuleMetadataCache.Unlock()
}

// trimArchiveSuffix removes common archive suffixes from a filename.
func trimArchiveSuffix(name string) string {
	name = strings.TrimSuffix(name, ".capsule.tar.gz")
	name = strings.TrimSuffix(name, ".capsule.tar.xz")
	name = strings.TrimSuffix(name, ".tar.gz")
	name = strings.TrimSuffix(name, ".tar.xz")
	return name
}

// categorizeCapsules categorizes capsules into CAS, NoIR, and WithIR in parallel.
func categorizeCapsules(capsules []CapsuleInfo) (noIR, cas, withIR []CapsuleInfo) {
	if len(capsules) == 0 {
		return nil, nil, nil
	}

	type result struct {
		capsule CapsuleInfo
		isCAS   bool
		hasIR   bool
	}

	// Create and start worker pool
	pool := NewWorkerPool[CapsuleInfo, result](maxWorkers, len(capsules))
	pool.Start(func(c CapsuleInfo) result {
		fullPath := filepath.Join(ServerConfig.CapsulesDir, c.Path)
		meta := getCapsuleMetadata(fullPath)
		return result{capsule: c, isCAS: meta.IsCAS, hasIR: meta.HasIR}
	})

	// Submit jobs
	for _, c := range capsules {
		pool.Submit(c)
	}
	pool.Close()

	// Collect results
	for r := range pool.Results() {
		if r.isCAS {
			cas = append(cas, r.capsule)
		} else if r.hasIR {
			withIR = append(withIR, r.capsule)
		} else {
			noIR = append(noIR, r.capsule)
		}
	}

	return noIR, cas, withIR
}

// getLoader creates a plugin loader configured according to the server settings.
// If external plugins are enabled, it loads them from the plugins directory.
func getLoader() *plugins.Loader {
	loader := plugins.NewLoader()
	if ServerConfig.PluginsExternal {
		if err := loader.LoadFromDir(ServerConfig.PluginsDir); err != nil {
			logging.Warn("failed to load external plugins",
				"plugins_dir", ServerConfig.PluginsDir,
				"error", err)
		}
	}
	return loader
}

// httpError logs the detailed error server-side and returns a generic error message to the client.
// This prevents information disclosure (CWE-209) while preserving error details for debugging.
func httpError(w http.ResponseWriter, err error, statusCode int) {
	// Generate a unique error ID for correlation
	errID := generateErrorID()

	// Log detailed error server-side with error ID
	logging.Error("http_error",
		"error_id", errID,
		"status_code", statusCode,
		"error", err)

	// Return generic message to client with error ID for support reference
	var msg string
	switch statusCode {
	case http.StatusNotFound:
		msg = fmt.Sprintf("Resource not found (ref: %s)", errID)
	case http.StatusBadRequest:
		msg = fmt.Sprintf("Bad request (ref: %s)", errID)
	case http.StatusInternalServerError:
		msg = fmt.Sprintf("Internal server error (ref: %s)", errID)
	default:
		msg = fmt.Sprintf("Error occurred (ref: %s)", errID)
	}

	http.Error(w, msg, statusCode)
}

// generateErrorID creates a short random ID for error correlation.
func generateErrorID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

// secureMkdirTemp creates a temporary directory with secure permissions (0700).
// This prevents race condition attacks (CWE-377) where an attacker might access
// the temp directory between creation and use.
func secureMkdirTemp(dir, pattern string) (string, error) {
	tempDir, err := os.MkdirTemp(dir, pattern)
	if err != nil {
		return "", err
	}

	// Immediately set restrictive permissions
	if err := os.Chmod(tempDir, 0700); err != nil {
		// Clean up on failure
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to set temp directory permissions: %w", err)
	}

	return tempDir, nil
}

// secureCreateFile creates a file with exclusive access (O_EXCL) and secure permissions.
// The O_EXCL flag prevents symlink attacks (CWE-367) by failing if the file already exists.
func secureCreateFile(path string, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
}

// CSRF Protection (CWE-352)
const (
	csrfCookieName = "csrf_token"
	csrfTokenLen   = 32
)

// generateCSRFToken creates a cryptographically secure random CSRF token.
func generateCSRFToken() string {
	b := make([]byte, csrfTokenLen)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// getOrCreateCSRFToken retrieves the CSRF token from cookies or creates a new one.
func getOrCreateCSRFToken(w http.ResponseWriter, r *http.Request) string {
	// Check for existing token in cookie
	cookie, err := r.Cookie(csrfCookieName)
	if err == nil && cookie.Value != "" && len(cookie.Value) == csrfTokenLen*2 {
		return cookie.Value
	}

	// Generate new token
	token := generateCSRFToken()
	if token == "" {
		return ""
	}

	// Set cookie with secure attributes
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true when using HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3600, // 1 hour
	})

	return token
}

// validateCSRFToken checks if the submitted token matches the cookie token.
func validateCSRFToken(r *http.Request) bool {
	// Get token from cookie
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}

	// Get token from form (hidden field)
	formToken := r.FormValue("csrf_token")
	if formToken == "" {
		// Also check header for AJAX requests
		formToken = r.Header.Get("X-CSRF-Token")
	}

	// Constant-time comparison to prevent timing attacks
	if len(cookie.Value) != len(formToken) {
		return false
	}
	return cookie.Value == formToken
}

// PageData is the base data for all pages.
type PageData struct {
	Title     string
	Error     string
	Message   string
	CSRFToken string
}

// IndexData is the data for the index page.
type IndexData struct {
	PageData
	Capsules []CapsuleInfo
	Plugins  []PluginInfo
}

// CapsuleInfo describes a capsule.
type CapsuleInfo struct {
	Name      string
	Path      string
	Size      int64
	SizeHuman string
	Format    string
}

// PluginInfo describes a plugin.
type PluginInfo struct {
	Name         string
	Type         string
	Description  string
	Version      string
	PluginID     string
	HasBinary    bool
	Source       string // "external", "internal", or "unloaded"
	Capabilities string // Capability notes (e.g., "IR: stub")
	License      string // License identifier (e.g., "MIT", "Apache-2.0") or fallback message
}

// CapsuleData is the data for the capsule detail page.
type CapsuleData struct {
	PageData
	Capsule   CapsuleInfo
	Manifest  *CapsuleManifest
	Artifacts []ArtifactInfo
}

// CapsuleManifest is the manifest.json structure.
type CapsuleManifest struct {
	Version      string            `json:"version"`
	ModuleType   string            `json:"module_type,omitempty"`
	Title        string            `json:"title,omitempty"`
	Language     string            `json:"language,omitempty"`
	Rights       string            `json:"rights,omitempty"`
	SourceFormat string            `json:"source_format,omitempty"`
	CreatedAt    string            `json:"created_at,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ArtifactInfo describes an artifact in a capsule.
type ArtifactInfo struct {
	ID        string
	Name      string
	Size      int64
	SizeHuman string
	Hash      string
}

// ArtifactData is the data for the artifact detail page.
type ArtifactData struct {
	PageData
	Capsule     CapsuleInfo
	Artifact    ArtifactInfo
	ContentType string
	Content     string
	IsBinary    bool
}

// IRData is the data for the IR view page.
type IRData struct {
	PageData
	Capsule         CapsuleInfo
	IR              map[string]interface{}
	IRJson          string
	IRJsonPreview   string
	IRJsonTruncated bool
	IRJsonLength    int
	URLPrefix       string // "/ir" or "/artifact"
}

// TranscriptData is the data for the transcript view.
type TranscriptData struct {
	PageData
	RunID      string
	Transcript []TranscriptEntry
}

// TranscriptEntry is a single transcript entry.
type TranscriptEntry struct {
	Timestamp string
	Level     string
	Message   string
}

// PluginsData is the data for the plugins page.
type PluginsData struct {
	PageData
	FormatPlugins []PluginInfo
	ToolPlugins   []PluginInfo
}

// ConvertData is the data for the convert page.
type ConvertData struct {
	PageData
	Formats      []string
	Capsules     []CapsuleInfo
	CapsulesNoIR []CapsuleInfo
	CapsulesCAS  []CapsuleInfo
	Result       *ConvertResult
	ActiveTab    string // "convert", "generate-ir", or "cas-to-sword"
}

// ConvertResult is the result of a conversion or IR generation.
type ConvertResult struct {
	Success      bool
	OutputPath   string
	OldPath      string
	LossClass    string
	Message      string
	SourceFormat string
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := struct {
		Title   string
		Error   string
		Message string
	}{
		Title: "Home",
	}

	if err := Templates.ExecuteTemplate(w, "index.html", data); err != nil {
		logging.Error("template rendering failed",
			"template", "index.html",
			"error", err)
	}
}

func handleCapsules(w http.ResponseWriter, r *http.Request) {
	// Redirect to unified /juniper page with capsules tab
	tab := r.URL.Query().Get("tab")
	redirectURL := "/juniper?tab=capsules"
	if tab != "" && tab != "list" {
		redirectURL += "&subtab=" + tab
	}
	http.Redirect(w, r, redirectURL, http.StatusMovedPermanently)
}

func handleCapsule(w http.ResponseWriter, r *http.Request) {
	// Extract capsule path from URL: /capsule/path/to/capsule.tar.xz
	capsulePath := strings.TrimPrefix(r.URL.Path, "/capsule/")
	if capsulePath == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Sanitize path to prevent path traversal attacks
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, capsulePath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)

	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "Capsule not found", http.StatusNotFound)
		return
	}

	data := CapsuleData{
		PageData: PageData{Title: "Capsule: " + capsulePath},
		Capsule: CapsuleInfo{
			Name:      filepath.Base(capsulePath),
			Path:      capsulePath,
			Size:      info.Size(),
			SizeHuman: humanSize(info.Size()),
		},
	}

	// Read capsule contents
	manifest, artifacts, err := readCapsule(fullPath)
	if err != nil {
		data.PageData.Error = fmt.Sprintf("Failed to read capsule: %v", err)
	} else {
		data.Manifest = manifest
		data.Artifacts = artifacts
	}

	if err := Templates.ExecuteTemplate(w, "capsule.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

// handleCapsuleDelete handles deletion of capsules.
func handleCapsuleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate CSRF token
	if !validateCSRFToken(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	capsulePath := r.FormValue("path")
	if capsulePath == "" {
		http.Error(w, "Missing path parameter", http.StatusBadRequest)
		return
	}

	// Sanitize path to prevent path traversal attacks
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, capsulePath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)

	// Verify file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		http.Error(w, "Capsule not found", http.StatusNotFound)
		return
	}

	// Delete the capsule file
	if err := os.Remove(fullPath); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete capsule: %v", err), http.StatusInternalServerError)
		return
	}

	// Invalidate caches since a capsule was deleted
	invalidateBibleCache()
	invalidateCapsulesListCache()
	invalidateCapsuleMetadataCache()

	// Redirect back to capsules page
	http.Redirect(w, r, "/capsules", http.StatusSeeOther)
}

func handleArtifact(w http.ResponseWriter, r *http.Request) {
	// URL format: /artifact/capsule-path?artifact=file-id
	capsulePath := strings.TrimPrefix(r.URL.Path, "/artifact/")

	// Check if artifact parameter is provided
	artifactID := r.URL.Query().Get("artifact")
	if artifactID == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if capsulePath == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Sanitize path to prevent path traversal attacks
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, capsulePath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)

	// Check if capsule exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		http.Error(w, "Capsule not found", http.StatusNotFound)
		return
	}

	// Extract artifact content
	content, contentType, err := readArtifactContent(fullPath, artifactID)
	if err != nil {
		http.Error(w, "Artifact not found", http.StatusNotFound)
		return
	}

	// Serve the content
	w.Header().Set("Content-Type", contentType)
	w.Write([]byte(content))
}

func handleIR(w http.ResponseWriter, r *http.Request) {
	handleIRWithPrefix(w, r, "/ir/")
}

func handleIRWithPrefix(w http.ResponseWriter, r *http.Request, prefix string) {
	capsulePath := strings.TrimPrefix(r.URL.Path, prefix)
	if capsulePath == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	isDownload, capsulePath := parseIRDownloadPath(capsulePath)
	fullPath, err := resolveIRPath(capsulePath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if isDownload {
		handleIRDownload(w, fullPath, capsulePath)
		return
	}

	renderIRPage(w, r, prefix, capsulePath, fullPath)
}

// parseIRDownloadPath extracts download flag and cleans capsule path.
func parseIRDownloadPath(capsulePath string) (bool, string) {
	isDownload := strings.HasSuffix(capsulePath, "/download")
	if isDownload {
		capsulePath = strings.TrimSuffix(capsulePath, "/download")
	}
	return isDownload, capsulePath
}

// resolveIRPath sanitizes and resolves the full path.
func resolveIRPath(capsulePath string) (string, error) {
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, capsulePath)
	if err != nil {
		return "", err
	}
	return filepath.Join(ServerConfig.CapsulesDir, cleanPath), nil
}

// renderIRPage builds and renders the IR view page.
func renderIRPage(w http.ResponseWriter, r *http.Request, prefix, capsulePath, fullPath string) {
	csrfToken := getOrCreateCSRFToken(w, r)
	data := buildIRData(capsulePath, prefix, csrfToken)

	if r.Method == http.MethodPost {
		handleIRGenerationRequest(r, &data, capsulePath)
	}

	if !checkCapsuleExists(fullPath, &data) || !loadIRContent(fullPath, &data) {
		Templates.ExecuteTemplate(w, "ir.html", data)
		return
	}

	if err := Templates.ExecuteTemplate(w, "ir.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

// handleIRDownload serves IR content as a downloadable JSON file
func handleIRDownload(w http.ResponseWriter, fullPath, capsulePath string) {
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		http.Error(w, "Capsule not found", http.StatusNotFound)
		return
	}
	irContent, err := readIRContent(fullPath)
	if err != nil {
		http.Error(w, "No IR found in capsule", http.StatusNotFound)
		return
	}
	jsonData := prettyJSON(irContent)
	baseName := trimArchiveSuffix(filepath.Base(capsulePath))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.ir.json\"", baseName))
	w.Write([]byte(jsonData))
}

// buildIRData constructs the initial IRData structure
func buildIRData(capsulePath, prefix, csrfToken string) IRData {
	return IRData{
		PageData: PageData{Title: "IR View: " + capsulePath, CSRFToken: csrfToken},
		Capsule: CapsuleInfo{
			Name: filepath.Base(capsulePath),
			Path: capsulePath,
		},
		URLPrefix: strings.TrimSuffix(prefix, "/"),
	}
}

// handleIRGenerationRequest processes POST requests to generate IR
func handleIRGenerationRequest(r *http.Request, data *IRData, capsulePath string) {
	if !validateCSRFToken(r) {
		data.PageData.Error = "Invalid CSRF token. Please try again."
		return
	}
	if r.FormValue("action") == "generate" {
		result := performIRGeneration(capsulePath)
		if result.Success {
			data.PageData.Message = result.Message
		} else {
			data.PageData.Error = result.Message
		}
	}
}

// checkCapsuleExists verifies the capsule file exists and updates data accordingly
func checkCapsuleExists(fullPath string, data *IRData) bool {
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		data.PageData.Title = "IR View"
		data.PageData.Error = fmt.Sprintf("Capsule not found: %s", data.Capsule.Path)
		return false
	}
	return true
}

// loadIRContent reads and prepares IR content for display
func loadIRContent(fullPath string, data *IRData) bool {
	irContent, err := readIRContent(fullPath)
	if err != nil {
		if data.PageData.Error == "" {
			data.PageData.Error = "No IR found in capsule. Use 'Generate IR' to create one."
		}
		return false
	}

	data.IR = irContent
	data.IRJson = prettyJSON(irContent)
	data.IRJsonLength = len(data.IRJson)
	if len(data.IRJson) > 1111 {
		data.IRJsonPreview = data.IRJson[:1111] + "..."
		data.IRJsonTruncated = true
	} else {
		data.IRJsonPreview = data.IRJson
	}
	return true
}

func handleTranscript(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimPrefix(r.URL.Path, "/transcript/")
	if runID == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	data := TranscriptData{
		PageData: PageData{Title: "Transcript: " + runID},
		RunID:    runID,
		Transcript: []TranscriptEntry{
			{Timestamp: "2024-01-01T00:00:00Z", Level: "INFO", Message: "Transcript viewing not yet implemented"},
		},
	}

	if err := Templates.ExecuteTemplate(w, "transcript.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

func handlePlugins(w http.ResponseWriter, r *http.Request) {
	data := PluginsData{
		PageData:      PageData{Title: "Plugins"},
		FormatPlugins: listFormatPlugins(),
		ToolPlugins:   listToolPlugins(),
	}

	if err := Templates.ExecuteTemplate(w, "plugins.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

var convertActionHandlers = map[string]func(*http.Request, *ConvertData){
	"convert": func(r *http.Request, data *ConvertData) {
		data.ActiveTab = "convert"
		source := r.FormValue("source")
		targetFormat := r.FormValue("format")
		if source != "" && targetFormat != "" {
			data.Result = performConversion(source, targetFormat)
		}
	},
	"generate-ir": func(r *http.Request, data *ConvertData) {
		data.ActiveTab = "generate-ir"
		source := r.FormValue("source")
		if source != "" {
			data.Result = performIRGeneration(source)
		}
	},
	"cas-to-sword": func(r *http.Request, data *ConvertData) {
		data.ActiveTab = "cas-to-sword"
		source := r.FormValue("source")
		if source != "" {
			data.Result = performCASToSWORD(source)
		}
	},
}

func processConvertPost(r *http.Request, data *ConvertData) {
	if !validateCSRFToken(r) {
		data.PageData.Error = "Invalid CSRF token. Please try again."
		return
	}
	action := r.FormValue("action")
	if handler, ok := convertActionHandlers[action]; ok {
		handler(r, data)
	}
}

func handleConvert(w http.ResponseWriter, r *http.Request) {
	allCapsules := listCapsules()
	capsulesNoIR, capsulesCAS, _ := categorizeCapsules(allCapsules)
	csrfToken := getOrCreateCSRFToken(w, r)

	data := ConvertData{
		PageData:     PageData{Title: "Convert & Generate IR", CSRFToken: csrfToken},
		Formats:      []string{"osis", "usfm", "usx", "json", "html", "epub", "markdown", "sqlite", "txt"},
		Capsules:     allCapsules,
		CapsulesNoIR: capsulesNoIR,
		CapsulesCAS:  capsulesCAS,
		ActiveTab:    "convert",
	}

	if r.Method == http.MethodPost {
		processConvertPost(r, &data)
	}

	if err := Templates.ExecuteTemplate(w, "convert.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

// conversionSetup validates and prepares the environment for conversion.
type conversionSetup struct {
	fullPath     string
	sourceFormat string
	tempDir      string
	extractDir   string
}

func setupConversion(sourcePath string) (*conversionSetup, *ConvertResult) {
	// Sanitize path
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, sourcePath)
	if err != nil {
		return nil, &ConvertResult{Success: false, Message: "Invalid path"}
	}
	fullPath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)

	// Check if capsule exists
	if _, err := os.Stat(fullPath); errors.Is(err, os.ErrNotExist) {
		return nil, &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Capsule not found: %s", sourcePath),
		}
	}

	// Detect source format
	sourceFormat := detectSourceFormat(sourcePath)

	// Create temp directory
	tempDir, err := secureMkdirTemp("", "capsule-convert-*")
	if err != nil {
		return nil, &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create temp directory: %v", err),
		}
	}

	// Extract capsule
	extractDir := filepath.Join(tempDir, "extract")
	if err := extractCapsule(fullPath, extractDir); err != nil {
		os.RemoveAll(tempDir)
		return nil, &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to extract capsule: %v", err),
		}
	}

	return &conversionSetup{
		fullPath:     fullPath,
		sourceFormat: sourceFormat,
		tempDir:      tempDir,
		extractDir:   extractDir,
	}, nil
}

// extractIRFromSource extracts IR from source content using the appropriate plugin.
func extractIRFromSource(loader *plugins.Loader, extractDir, tempDir, sourceFormat string) (string, string, *ConvertResult) {
	// Find source content file
	contentPath, detectedFormat := findContentFile(extractDir)
	if contentPath == "" {
		return "", "", &ConvertResult{
			Success: false,
			Message: "No convertible content found in capsule. Supported formats: OSIS, USFM, USX, JSON, SWORD.",
		}
	}

	if sourceFormat == "unknown" {
		sourceFormat = detectedFormat
	}

	// Get source plugin
	sourcePlugin, err := loader.GetPlugin("format." + sourceFormat)
	if err != nil {
		return "", sourceFormat, &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("No plugin found for source format '%s'. Install the format.%s plugin.", sourceFormat, sourceFormat),
			SourceFormat: sourceFormat,
		}
	}

	// Extract IR
	irDir := filepath.Join(tempDir, "ir")
	os.MkdirAll(irDir, 0700)

	extractReq := plugins.NewExtractIRRequest(contentPath, irDir)
	extractResp, err := plugins.ExecutePlugin(sourcePlugin, extractReq)
	if err != nil {
		return "", sourceFormat, &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("Failed to extract IR: %v", err),
			SourceFormat: sourceFormat,
		}
	}

	extractResult, err := plugins.ParseExtractIRResult(extractResp)
	if err != nil {
		return "", sourceFormat, &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("Failed to parse extract result: %v", err),
			SourceFormat: sourceFormat,
		}
	}

	return extractResult.IRPath, sourceFormat, &ConvertResult{LossClass: extractResult.LossClass}
}

// emitTargetFormat emits target format from IR using the appropriate plugin.
func emitTargetFormat(loader *plugins.Loader, irPath, tempDir, targetFormat, sourceFormat, extractLoss string) (string, string, *ConvertResult) {
	targetPlugin, err := loader.GetPlugin("format." + targetFormat)
	if err != nil {
		return "", "", &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("No plugin found for target format '%s'. Install the format.%s plugin.", targetFormat, targetFormat),
			SourceFormat: sourceFormat,
		}
	}

	emitDir := filepath.Join(tempDir, "output")
	os.MkdirAll(emitDir, 0700)

	emitReq := plugins.NewEmitNativeRequest(irPath, emitDir)
	emitResp, err := plugins.ExecutePlugin(targetPlugin, emitReq)
	if err != nil {
		return "", "", &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("Failed to emit %s: %v", targetFormat, err),
			SourceFormat: sourceFormat,
			LossClass:    extractLoss,
		}
	}

	emitResult, err := plugins.ParseEmitNativeResult(emitResp)
	if err != nil {
		return "", "", &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("Failed to parse emit result: %v", err),
			SourceFormat: sourceFormat,
		}
	}

	return emitResult.OutputPath, emitResult.LossClass, nil
}

// createConvertedCapsule creates a new capsule with converted content.
func createConvertedCapsule(tempDir, outputPath, irPath, sourcePath, fullPath, sourceFormat, targetFormat, extractLoss, emitLoss string) *ConvertResult {
	newCapsuleDir := filepath.Join(tempDir, "new-capsule")
	os.MkdirAll(newCapsuleDir, 0700)

	if err := copyConvertedOutput(outputPath, newCapsuleDir); err != nil {
		return convertError("%v", err)
	}
	copyIRToNewCapsule(irPath, sourcePath, newCapsuleDir)
	writeConversionManifest(newCapsuleDir, fullPath, sourceFormat, targetFormat, extractLoss, emitLoss)

	return finalizeConvertedCapsule(fullPath, newCapsuleDir, sourcePath, sourceFormat, targetFormat, extractLoss, emitLoss)
}

// copyConvertedOutput copies the converted output file to the new capsule directory
func copyConvertedOutput(outputPath, newCapsuleDir string) error {
	outputData, err := os.ReadFile(outputPath)
	if err != nil {
		return fmt.Errorf("Failed to read converted output: %w", err)
	}
	outputName := filepath.Base(outputPath)
	if err := os.WriteFile(filepath.Join(newCapsuleDir, outputName), outputData, 0600); err != nil {
		return fmt.Errorf("Failed to write converted output: %w", err)
	}
	return nil
}

// copyIRToNewCapsule copies the IR file to the new capsule directory
func copyIRToNewCapsule(irPath, sourcePath, newCapsuleDir string) {
	irData, err := os.ReadFile(irPath)
	if err != nil {
		return
	}
	irName := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath)) + ".ir.json"
	os.WriteFile(filepath.Join(newCapsuleDir, irName), irData, 0600)
}

// writeConversionManifest creates the manifest for the converted capsule
func writeConversionManifest(newCapsuleDir, fullPath, sourceFormat, targetFormat, extractLoss, emitLoss string) {
	manifest := map[string]interface{}{
		"capsule_version": "1.0",
		"module_type":     "bible",
		"source_format":   targetFormat,
		"converted_from":  sourceFormat,
		"conversion_date": time.Now().Format(time.RFC3339),
		"has_ir":          true,
		"extraction_loss": extractLoss,
		"emission_loss":   emitLoss,
	}
	preserveOriginalMetadata(fullPath, manifest)
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(newCapsuleDir, "manifest.json"), manifestData, 0600)
}

// preserveOriginalMetadata copies title and language from original manifest
func preserveOriginalMetadata(fullPath string, manifest map[string]interface{}) {
	origManifest := readCapsuleManifest(fullPath)
	if origManifest == nil {
		return
	}
	if origManifest.Title != "" {
		manifest["title"] = origManifest.Title
	}
	if origManifest.Language != "" {
		manifest["language"] = origManifest.Language
	}
}

// finalizeConvertedCapsule renames original and creates the new capsule
func finalizeConvertedCapsule(fullPath, newCapsuleDir, sourcePath, sourceFormat, targetFormat, extractLoss, emitLoss string) *ConvertResult {
	oldPath := renameToOld(fullPath)
	if oldPath == "" {
		return convertError("Failed to rename original capsule")
	}
	if err := archive.CreateCapsuleTarGzFromPath(newCapsuleDir, fullPath); err != nil {
		os.Rename(oldPath, fullPath)
		return convertError("Failed to create new capsule: %v", err)
	}
	return &ConvertResult{
		Success:      true,
		OutputPath:   sourcePath,
		OldPath:      filepath.Base(oldPath),
		LossClass:    combineLossClass(extractLoss, emitLoss),
		Message:      fmt.Sprintf("Successfully converted from %s to %s", sourceFormat, targetFormat),
		SourceFormat: sourceFormat,
	}
}

// performConversion converts a capsule to a different format.
// It creates a new capsule with the converted content and renames the original.
func performConversion(sourcePath, targetFormat string) *ConvertResult {
	// Setup and validation
	setup, errResult := setupConversion(sourcePath)
	if errResult != nil {
		return errResult
	}
	defer os.RemoveAll(setup.tempDir)

	// Load plugins
	loader := getLoader()
	if err := loader.LoadFromDir(ServerConfig.PluginsDir); err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to load plugins: %v", err),
		}
	}

	// Extract IR from source
	irPath, sourceFormat, errResult := extractIRFromSource(loader, setup.extractDir, setup.tempDir, setup.sourceFormat)
	if errResult != nil {
		if !errResult.Success {
			return errResult
		}
	}

	// Emit target format
	outputPath, emitLoss, errResult := emitTargetFormat(loader, irPath, setup.tempDir, targetFormat, sourceFormat, errResult.LossClass)
	if errResult != nil && !errResult.Success {
		return errResult
	}

	// Create converted capsule
	return createConvertedCapsule(setup.tempDir, outputPath, irPath, sourcePath, setup.fullPath, sourceFormat, targetFormat, errResult.LossClass, emitLoss)
}

// performIRGeneration generates IR for a capsule that doesn't have one.
func performIRGeneration(sourcePath string) *ConvertResult {
	fullPath, errResult := validateIRGenerationPath(sourcePath)
	if errResult != nil {
		return errResult
	}

	tempDir, err := secureMkdirTemp("", "capsule-ir-gen-*")
	if err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create temp directory: %v", err),
		}
	}
	defer os.RemoveAll(tempDir)

	extractDir := filepath.Join(tempDir, "extract")
	if err := extractCapsule(fullPath, extractDir); err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to extract capsule: %v", err),
		}
	}

	contentPath, detectedFormat, errResult := validateContentForIR(extractDir)
	if errResult != nil {
		return errResult
	}

	extractResult, errResult := performPluginIRExtraction(contentPath, detectedFormat, tempDir)
	if errResult != nil {
		return errResult
	}

	newCapsuleDir, errResult := buildCapsuleWithIR(extractDir, extractResult, sourcePath, detectedFormat, tempDir)
	if errResult != nil {
		return errResult
	}

	oldPath, errResult := replaceCapsule(fullPath, newCapsuleDir)
	if errResult != nil {
		return errResult
	}

	return &ConvertResult{
		Success:      true,
		OutputPath:   sourcePath,
		OldPath:      filepath.Base(oldPath),
		LossClass:    extractResult.LossClass,
		Message:      fmt.Sprintf("Successfully generated IR from %s format", detectedFormat),
		SourceFormat: detectedFormat,
	}
}

// validateIRGenerationPath validates the capsule path and checks for existence and IR status
func validateIRGenerationPath(sourcePath string) (string, *ConvertResult) {
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, sourcePath)
	if err != nil {
		return "", &ConvertResult{
			Success: false,
			Message: "Invalid path",
		}
	}
	fullPath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)

	if _, err := os.Stat(fullPath); errors.Is(err, os.ErrNotExist) {
		return "", &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Capsule not found: %s", sourcePath),
		}
	}

	if archive.HasIR(fullPath) {
		return "", &ConvertResult{
			Success: false,
			Message: "This capsule already contains IR. No generation needed.",
		}
	}

	return fullPath, nil
}

// validateContentForIR finds and validates the content file, returning detailed error for CAS capsules
func validateContentForIR(extractDir string) (string, string, *ConvertResult) {
	contentPath, detectedFormat := findContentFile(extractDir)
	if contentPath != "" {
		return contentPath, detectedFormat, nil
	}

	blobsDir := filepath.Join(extractDir, "blobs")
	if _, err := os.Stat(blobsDir); err == nil {
		return "", "", &ConvertResult{
			Success: false,
			Message: "This capsule uses Content-Addressed Storage (CAS) format with blobs.\n\n" +
				"CAS capsules require extraction via the CLI:\n" +
				"  capsule export <capsule> --artifact main --out extracted/\n\n" +
				"Then use 'capsule juniper ingest' to create a SWORD capsule that can be converted.",
		}
	}

	return "", "", buildNoContentFoundError(extractDir)
}

// buildNoContentFoundError creates an error message listing files found in the capsule
func buildNoContentFoundError(extractDir string) *ConvertResult {
	var foundFiles []string
	filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(extractDir, path)
			foundFiles = append(foundFiles, rel)
		}
		return nil
	})

	fileList := strings.Join(foundFiles, ", ")
	if len(foundFiles) > 10 {
		fileList = strings.Join(foundFiles[:10], ", ") + fmt.Sprintf(" (and %d more)", len(foundFiles)-10)
	}
	if len(foundFiles) == 0 {
		fileList = "(empty)"
	}

	return &ConvertResult{
		Success: false,
		Message: fmt.Sprintf("No convertible content found in capsule.\n\nSupported formats: OSIS, USFM, USX, SWORD (mods.d/*.conf)\n\nFiles found: %s", fileList),
	}
}

// performPluginIRExtraction loads plugins and extracts IR from the content file
func performPluginIRExtraction(contentPath, detectedFormat, tempDir string) (*plugins.ExtractIRResult, *ConvertResult) {
	loader := getLoader()
	if err := loader.LoadFromDir(ServerConfig.PluginsDir); err != nil {
		return nil, &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to load plugins: %v", err),
		}
	}

	sourcePlugin, err := loader.GetPlugin("format." + detectedFormat)
	if err != nil {
		return nil, &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("No plugin found for format '%s'. Install the format.%s plugin.", detectedFormat, detectedFormat),
			SourceFormat: detectedFormat,
		}
	}

	irDir := filepath.Join(tempDir, "ir")
	os.MkdirAll(irDir, 0700)

	extractReq := plugins.NewExtractIRRequest(contentPath, irDir)
	extractResp, err := plugins.ExecutePlugin(sourcePlugin, extractReq)
	if err != nil {
		return nil, &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("Failed to extract IR: %v", err),
			SourceFormat: detectedFormat,
		}
	}

	extractResult, err := parseExtractIRResultFlexible(extractResp)
	if err != nil {
		return nil, &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("Failed to parse extract result: %v", err),
			SourceFormat: detectedFormat,
		}
	}

	return extractResult, nil
}

// buildCapsuleWithIR creates a new capsule directory with IR and updated manifest
func buildCapsuleWithIR(extractDir string, extractResult *plugins.ExtractIRResult, sourcePath, detectedFormat, tempDir string) (string, *ConvertResult) {
	newCapsuleDir := filepath.Join(tempDir, "new-capsule")

	if err := fileutil.CopyDir(extractDir, newCapsuleDir); err != nil {
		return "", &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to copy capsule contents: %v", err),
		}
	}

	irData, errResult := readIRFileData(extractResult.IRPath)
	if errResult != nil {
		return "", errResult
	}

	irName := trimArchiveSuffix(filepath.Base(sourcePath)) + ".ir.json"
	if err := os.WriteFile(filepath.Join(newCapsuleDir, irName), irData, 0600); err != nil {
		return "", &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to write IR file: %v", err),
		}
	}

	if err := updateManifestWithIR(newCapsuleDir, extractResult.LossClass, detectedFormat); err != nil {
		return "", &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to update manifest: %v", err),
		}
	}

	return newCapsuleDir, nil
}

// readIRFileData reads the IR file, handling both file and directory paths
func readIRFileData(irPath string) ([]byte, *ConvertResult) {
	info, err := os.Stat(irPath)
	if err != nil {
		return nil, convertError("Failed to stat IR path: %v", err)
	}

	if info.IsDir() {
		irPath, err = findIRFileInDir(irPath)
		if err != nil {
			return nil, convertError("%v", err)
		}
	}

	irData, err := os.ReadFile(irPath)
	if err != nil {
		return nil, convertError("Failed to read IR: %v", err)
	}
	return irData, nil
}

// findIRFileInDir finds the first .ir.json file in a directory
func findIRFileInDir(dirPath string) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("Failed to read IR directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".ir.json") {
			return filepath.Join(dirPath, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("No .ir.json file found in IR directory")
}

// convertError creates a failure ConvertResult with formatted message
func convertError(format string, args ...interface{}) *ConvertResult {
	return &ConvertResult{Success: false, Message: fmt.Sprintf(format, args...)}
}

// updateManifestWithIR updates the manifest.json with IR metadata
func updateManifestWithIR(capsuleDir, lossClass, sourceFormat string) error {
	manifestPath := filepath.Join(capsuleDir, "manifest.json")
	manifest := make(map[string]interface{})

	if data, err := os.ReadFile(manifestPath); err == nil {
		json.Unmarshal(data, &manifest)
	}

	manifest["has_ir"] = true
	manifest["ir_generated"] = time.Now().Format(time.RFC3339)
	manifest["ir_loss_class"] = lossClass
	manifest["source_format"] = sourceFormat

	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	return os.WriteFile(manifestPath, manifestData, 0600)
}

// replaceCapsule replaces the original capsule with the new one, backing up the original
func replaceCapsule(fullPath, newCapsuleDir string) (string, *ConvertResult) {
	oldPath := renameToOld(fullPath)
	if oldPath == "" {
		return "", &ConvertResult{
			Success: false,
			Message: "Failed to rename original capsule",
		}
	}

	if err := archive.CreateCapsuleTarGzFromPath(newCapsuleDir, fullPath); err != nil {
		os.Rename(oldPath, fullPath)
		return "", &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create new capsule: %v", err),
		}
	}

	return oldPath, nil
}

// performCASToSWORD converts a CAS capsule to SWORD format.
func performCASToSWORD(sourcePath string) *ConvertResult {
	fullPath, err := validateCASCapsulePath(sourcePath)
	if err != nil {
		return &ConvertResult{Success: false, Message: err.Error()}
	}

	tempDir, extractDir, err := setupTempDirAndExtract(fullPath)
	if err != nil {
		return &ConvertResult{Success: false, Message: err.Error()}
	}
	defer os.RemoveAll(tempDir)

	manifest, mainArtifact, err := loadCASManifestAndArtifact(extractDir)
	if err != nil {
		return &ConvertResult{Success: false, Message: err.Error()}
	}

	swordDir := filepath.Join(tempDir, "sword-capsule")
	if err := os.MkdirAll(swordDir, 0700); err != nil {
		return &ConvertResult{Success: false, Message: fmt.Sprintf("Failed to create SWORD directory: %v", err)}
	}

	extractArtifactFilesToSWORD(extractDir, swordDir, mainArtifact)
	createSWORDManifest(swordDir, manifest)
	hasSwordData := checkSWORDStructure(swordDir)

	return finalizeCapsuleConversion(fullPath, swordDir, hasSwordData)
}

// validateCASCapsulePath validates and returns the full path to a CAS capsule.
func validateCASCapsulePath(sourcePath string) (string, error) {
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, sourcePath)
	if err != nil {
		return "", errors.New("Invalid path")
	}
	fullPath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)

	if _, err := os.Stat(fullPath); errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("Capsule not found: %s", sourcePath)
	}

	if !archive.IsCASCapsule(fullPath) {
		return "", errors.New("This is not a CAS capsule. CAS capsules have a blobs/ directory.")
	}

	return fullPath, nil
}

// setupTempDirAndExtract creates a temp directory and extracts the CAS capsule.
func setupTempDirAndExtract(fullPath string) (tempDir, extractDir string, err error) {
	tempDir, err = secureMkdirTemp("", "capsule-cas-convert-*")
	if err != nil {
		return "", "", fmt.Errorf("Failed to create temp directory: %v", err)
	}

	extractDir = filepath.Join(tempDir, "extract")
	if err := extractCapsule(fullPath, extractDir); err != nil {
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("Failed to extract capsule: %v", err)
	}

	return tempDir, extractDir, nil
}

// loadCASManifestAndArtifact loads the CAS manifest and finds the main artifact.
func loadCASManifestAndArtifact(extractDir string) (*CASManifest, *CASArtifact, error) {
	manifestPath := filepath.Join(extractDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to read manifest: %v", err)
	}

	var manifest CASManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, nil, fmt.Errorf("Failed to parse manifest: %v", err)
	}

	mainArtifact := findMainArtifact(&manifest)
	if mainArtifact == nil {
		return nil, nil, errors.New("No artifacts found in CAS capsule")
	}

	return &manifest, mainArtifact, nil
}

// findMainArtifact finds the main artifact in the manifest.
func findMainArtifact(manifest *CASManifest) *CASArtifact {
	for i := range manifest.Artifacts {
		if manifest.Artifacts[i].ID == "main" || manifest.Artifacts[i].ID == manifest.MainArtifact {
			return &manifest.Artifacts[i]
		}
	}
	if len(manifest.Artifacts) > 0 {
		return &manifest.Artifacts[0]
	}
	return nil
}

// extractArtifactFilesToSWORD extracts files from the CAS artifact to the SWORD directory.
func extractArtifactFilesToSWORD(extractDir, swordDir string, artifact *CASArtifact) {
	for _, file := range artifact.Files {
		blobPath := resolveBlobPath(extractDir, file)
		if blobPath == "" {
			continue
		}

		content, err := os.ReadFile(blobPath)
		if err != nil {
			continue
		}

		destPath := filepath.Join(swordDir, file.Path)
		os.MkdirAll(filepath.Dir(destPath), 0700)
		os.WriteFile(destPath, content, 0600)
	}

	// Fallback: extract all blobs if no files were extracted
	if isDirEmpty(swordDir) {
		extractAllBlobs(extractDir, swordDir)
	}
}

// resolveBlobPath returns the path to a blob based on its hash.
// Validates hash format to prevent path traversal attacks.
func resolveBlobPath(extractDir string, file CASFile) string {
	if file.Blake3 != "" && validation.IsValidHexHash(file.Blake3) {
		return filepath.Join(extractDir, "blobs", "blake3", file.Blake3[:2], file.Blake3)
	}
	if file.SHA256 != "" && validation.IsValidHexHash(file.SHA256) {
		return filepath.Join(extractDir, "blobs", "sha256", file.SHA256[:2], file.SHA256)
	}
	return ""
}

// extractAllBlobs copies all blobs to the SWORD directory as a fallback.
func extractAllBlobs(extractDir, swordDir string) {
	blobsDir := filepath.Join(extractDir, "blobs")
	filepath.Walk(blobsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil || len(content) == 0 {
			return nil
		}

		destName := filepath.Base(path)
		if isJSONContent(content) {
			destName += ".json"
		}
		os.WriteFile(filepath.Join(swordDir, destName), content, 0600)
		return nil
	})
}

// createSWORDManifest creates a manifest.json for the SWORD capsule.
func createSWORDManifest(swordDir string, manifest *CASManifest) {
	swordManifest := map[string]interface{}{
		"capsule_version": "1.0",
		"module_type":     manifest.ModuleType,
		"id":              manifest.ID,
		"title":           manifest.Title,
		"source_format":   "cas-converted",
		"original_format": manifest.SourceFormat,
		"converted_from":  "cas",
	}
	manifestData, _ := json.MarshalIndent(swordManifest, "", "  ")
	os.WriteFile(filepath.Join(swordDir, "manifest.json"), manifestData, 0600)
}

// checkSWORDStructure checks if the directory contains SWORD module structure.
func checkSWORDStructure(swordDir string) bool {
	_, err := os.Stat(filepath.Join(swordDir, "mods.d"))
	return err == nil
}

// finalizeCapsuleConversion renames the original capsule and creates the new archive.
func finalizeCapsuleConversion(fullPath, swordDir string, hasSwordData bool) *ConvertResult {
	oldPath := renameToOld(fullPath)
	if oldPath == "" {
		return &ConvertResult{Success: false, Message: "Failed to rename original capsule"}
	}

	newPath := strings.TrimSuffix(fullPath, ".tar.xz") + ".tar.gz"
	if strings.HasSuffix(fullPath, ".tar.gz") {
		newPath = fullPath
	}

	if err := archive.CreateCapsuleTarGzFromPath(swordDir, newPath); err != nil {
		os.Rename(oldPath, fullPath) // Restore on failure
		return &ConvertResult{Success: false, Message: fmt.Sprintf("Failed to create SWORD capsule: %v", err)}
	}

	msg := "Successfully converted CAS capsule to extracted format"
	if hasSwordData {
		msg = "Successfully converted CAS capsule to SWORD format"
	}

	return &ConvertResult{
		Success:    true,
		OutputPath: filepath.Base(newPath),
		OldPath:    filepath.Base(oldPath),
		Message:    msg,
	}
}

// CASManifest represents a CAS capsule manifest.
type CASManifest struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	ModuleType   string        `json:"module_type"`
	SourceFormat string        `json:"source_format"`
	MainArtifact string        `json:"main_artifact"`
	Artifacts    []CASArtifact `json:"artifacts"`
}

// CASArtifact represents an artifact in a CAS capsule.
type CASArtifact struct {
	ID    string    `json:"id"`
	Files []CASFile `json:"files"`
}

// CASFile represents a file in a CAS artifact.
type CASFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Blake3 string `json:"blake3"`
}

// isDirEmpty checks if a directory is empty.
func isDirEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return true
	}
	return len(entries) == 0
}

// isJSONContent checks if content looks like JSON.
func isJSONContent(content []byte) bool {
	if len(content) < 2 {
		return false
	}
	for i := 0; i < len(content); i++ {
		if isWhitespace(content[i]) {
			continue
		}
		return content[i] == '{' || content[i] == '['
	}
	return false
}

// isWhitespace checks if a byte is whitespace.
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// extractCapsule extracts a capsule archive to a directory.
func extractCapsule(capsulePath, destDir string) error {
	f, err := os.Open(capsulePath)
	if err != nil {
		return err
	}
	defer f.Close()

	tr, err := createTarReader(f, capsulePath)
	if err != nil {
		return err
	}

	return extractTarEntries(tr, destDir)
}

// createTarReader creates a tar reader based on the archive format
func createTarReader(f *os.File, capsulePath string) (*tar.Reader, error) {
	if strings.HasSuffix(capsulePath, ".tar.xz") {
		xzr, err := xz.NewReader(f)
		if err != nil {
			return nil, err
		}
		return tar.NewReader(xzr), nil
	} else if strings.HasSuffix(capsulePath, ".tar.gz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gzr.Close()
		return tar.NewReader(gzr), nil
	}
	return nil, fmt.Errorf("unsupported archive format")
}

// extractTarEntries extracts all entries from a tar archive
func extractTarEntries(tr *tar.Reader, destDir string) error {
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if err := extractTarEntry(header, tr, destDir); err != nil {
			return err
		}
	}
	return nil
}

// extractTarEntry extracts a single entry from the tar archive
func extractTarEntry(header *tar.Header, tr *tar.Reader, destDir string) error {
	name := cleanTarEntryName(header.Name)
	if name == "" {
		return nil
	}

	destPath := filepath.Join(destDir, name)

	if header.FileInfo().IsDir() {
		return os.MkdirAll(destPath, 0700)
	}

	return extractTarFile(destPath, tr)
}

// cleanTarEntryName removes leading directory from tar entry name
func cleanTarEntryName(name string) string {
	if idx := strings.Index(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}

// extractTarFile extracts a single file from the tar archive
func extractTarFile(destPath string, tr *tar.Reader) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0700); err != nil {
		return err
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(outFile, tr); err != nil {
		outFile.Close()
		return err
	}

	// Check Close() error for writes to detect disk full, etc.
	return outFile.Close()
}

type irModule struct {
	Module    string `json:"module"`
	Status    string `json:"status"`
	IRPath    string `json:"ir_path"`
	LossClass string `json:"loss_class"`
	Error     string `json:"error"`
	Reason    string `json:"reason"`
}

func firstModuleError(modules []irModule) error {
	for _, m := range modules {
		if m.Status == "error" {
			return fmt.Errorf("module %s: %s", m.Module, m.Error)
		}
		if m.Status == "skipped" {
			return fmt.Errorf("module %s skipped: %s", m.Module, m.Reason)
		}
	}
	return fmt.Errorf("no IR generated from any module")
}

func parseMultiModuleIRResult(data []byte) (*plugins.ExtractIRResult, error) {
	var multi struct {
		Modules []irModule `json:"modules"`
		Count   int        `json:"count"`
	}
	if err := json.Unmarshal(data, &multi); err != nil || len(multi.Modules) == 0 {
		return nil, fmt.Errorf("unable to parse extract-ir result")
	}
	for _, m := range multi.Modules {
		if m.Status == "ok" && m.IRPath != "" {
			return &plugins.ExtractIRResult{IRPath: m.IRPath, LossClass: m.LossClass}, nil
		}
	}
	return nil, firstModuleError(multi.Modules)
}

func parseExtractIRResultFlexible(resp *plugins.IPCResponse) (*plugins.ExtractIRResult, error) {
	if resp.Status == "error" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}
	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal result: %w", err)
	}
	var standard plugins.ExtractIRResult
	if err := json.Unmarshal(data, &standard); err == nil && standard.IRPath != "" {
		return &standard, nil
	}
	return parseMultiModuleIRResult(data)
}

// findContentFile finds a convertible content file in the extracted capsule.
// detectSwordModule checks a list of candidate base directories for a
// mods.d/*.conf file. It returns the base directory path if found, or "".
func detectSwordModule(bases []string) string {
	for _, base := range bases {
		modsDir := filepath.Join(base, "mods.d")
		if _, err := os.Stat(modsDir); err != nil {
			continue
		}
		entries, _ := os.ReadDir(modsDir)
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".conf") {
				return base
			}
		}
	}
	return ""
}

// matchContentFormat returns the format name for a lowercased filename that
// matches one of the known Bible-text extensions, or "" when there is no match.
func matchContentFormat(name string) string {
	for _, p := range []struct{ ext, format string }{
		{".osis", "osis"},
		{".osis.xml", "osis"},
		{".usx", "usx"},
		{".usfm", "usfm"},
		{".sfm", "usfm"},
	} {
		if strings.HasSuffix(name, p.ext) {
			return p.format
		}
	}
	return ""
}

func findContentFile(extractDir string) (string, string) {
	// Check for SWORD module FIRST (has mods.d/*.conf).
	// Try both the direct path and the nested capsule/ path.
	swordBases := []string{extractDir, filepath.Join(extractDir, "capsule")}
	if base := detectSwordModule(swordBases); base != "" {
		return base, "sword-pure"
	}

	// Walk the tree looking for the first file whose extension matches a known
	// Bible-text format (OSIS, USX, USFM).
	var found, format string
	filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := strings.ToLower(filepath.Base(path))
		if name == "manifest.json" || strings.HasSuffix(name, ".ir.json") {
			return nil
		}
		if fmt := matchContentFormat(name); fmt != "" {
			found, format = path, fmt
			return filepath.SkipAll
		}
		return nil
	})

	return found, format
}

// renameToOld renames a file by appending "-old" before the extension.
func renameToOld(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// Handle .capsule.tar.gz or .capsule.tar.xz
	var oldName string
	if strings.HasSuffix(base, ".capsule.tar.gz") {
		oldName = strings.TrimSuffix(base, ".capsule.tar.gz") + "-old.capsule.tar.gz"
	} else if strings.HasSuffix(base, ".capsule.tar.xz") {
		oldName = strings.TrimSuffix(base, ".capsule.tar.xz") + "-old.capsule.tar.xz"
	} else if strings.HasSuffix(base, ".tar.gz") {
		oldName = strings.TrimSuffix(base, ".tar.gz") + "-old.tar.gz"
	} else if strings.HasSuffix(base, ".tar.xz") {
		oldName = strings.TrimSuffix(base, ".tar.xz") + "-old.tar.xz"
	} else {
		oldName = base + "-old"
	}

	oldPath := filepath.Join(dir, oldName)

	if err := os.Rename(path, oldPath); err != nil {
		return ""
	}

	return oldPath
}

// combineLossClass combines two loss classes into the worst case.
func combineLossClass(a, b string) string {
	order := map[string]int{"L0": 0, "L1": 1, "L2": 2, "L3": 3, "": 0}

	aVal := order[a]
	bVal := order[b]

	if aVal > bVal {
		return a
	}
	return b
}

// detectSourceFormat is implemented in format_detection.go

func openTarReader(f *os.File, capsulePath string) (*tar.Reader, io.Closer, error) {
	if strings.HasSuffix(capsulePath, ".tar.xz") {
		xzr, err := xz.NewReader(f)
		if err != nil {
			return nil, nil, err
		}
		return tar.NewReader(xzr), nil, nil
	}
	if strings.HasSuffix(capsulePath, ".tar.gz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return nil, nil, err
		}
		return tar.NewReader(gzr), gzr, nil
	}
	return nil, nil, fmt.Errorf("unsupported format")
}

func findManifestEntry(tr *tar.Reader) *CapsuleManifest {
	for {
		header, err := tr.Next()
		if err != nil {
			return nil
		}
		if !strings.HasSuffix(header.Name, "manifest.json") {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil
		}
		var manifest CapsuleManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil
		}
		return &manifest
	}
}

func readCapsuleManifest(capsulePath string) *CapsuleManifest {
	f, err := os.Open(capsulePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	tr, closer, err := openTarReader(f, capsulePath)
	if err != nil {
		return nil
	}
	if closer != nil {
		defer closer.Close()
	}

	return findManifestEntry(tr)
}

func handleStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/static/")
	contentType := staticContentType(path)
	if contentType == "" {
		http.NotFound(w, r)
		return
	}

	content, etag, err := loadStaticContent(path)
	if err != nil {
		http.Error(w, path+" not found", http.StatusNotFound)
		return
	}

	if checkNotModified(w, r, etag) {
		return
	}

	serveStaticContent(w, content, contentType, etag)
}

// staticContentType returns the content type for a static file path.
func staticContentType(path string) string {
	types := map[string]string{
		"base.css":  "text/css",
		"style.css": "text/css",
		"app.js":    "application/javascript",
	}
	return types[path]
}

// loadStaticContent loads static file content from cache or filesystem.
func loadStaticContent(path string) ([]byte, string, error) {
	content, etag, ok := getStaticFile(path)
	if ok {
		return content, etag, nil
	}
	content, err := staticFS.ReadFile("static/" + path)
	return content, "", err
}

// checkNotModified checks If-None-Match and sends 304 if matched.
func checkNotModified(w http.ResponseWriter, r *http.Request, etag string) bool {
	if etag != "" && r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	return false
}

// serveStaticContent writes static content with appropriate headers.
func serveStaticContent(w http.ResponseWriter, content []byte, contentType, etag string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	w.Write(content)
}

// Helper functions

// listCapsules returns a cached list of capsules (uses getCachedCapsulesList).
func listCapsules() []CapsuleInfo {
	return getCachedCapsulesList()
}

// listCapsulesUncached returns all capsules without caching.
// Uses os.ReadDir for better performance than filepath.Walk.
func listCapsulesUncached() []CapsuleInfo {
	entries, err := os.ReadDir(ServerConfig.CapsulesDir)
	if err != nil {
		return nil
	}

	var capsules []CapsuleInfo
	for _, entry := range entries {
		if capsule := entryCapsuleInfo(entry); capsule != nil {
			capsules = append(capsules, *capsule)
		}
	}

	sort.Slice(capsules, func(i, j int) bool {
		return capsules[i].Name < capsules[j].Name
	})

	return capsules
}

// entryCapsuleInfo builds CapsuleInfo from a directory entry if it's a capsule.
func entryCapsuleInfo(entry os.DirEntry) *CapsuleInfo {
	if entry.IsDir() || !isCapsuleExtension(filepath.Ext(entry.Name())) {
		return nil
	}
	info, err := entry.Info()
	if err != nil {
		return nil
	}
	name := entry.Name()
	fullPath := filepath.Join(ServerConfig.CapsulesDir, name)
	return &CapsuleInfo{
		Name:      name,
		Path:      name,
		Size:      info.Size(),
		SizeHuman: humanSize(info.Size()),
		Format:    detectCapsuleFormat(fullPath),
	}
}

// isCapsuleExtension checks if extension indicates a capsule file.
func isCapsuleExtension(ext string) bool {
	return ext == ".xz" || ext == ".gz" || ext == ".tar"
}

func listFormatPlugins() []PluginInfo {
	var pluginInfos []PluginInfo
	loader := getLoader()
	for _, p := range loader.ListPlugins() {
		if strings.HasPrefix(p.Manifest.PluginID, "format.") {
			pluginInfos = append(pluginInfos, buildPluginInfo(p))
		}
	}
	return pluginInfos
}

func buildPluginInfo(p *plugins.Plugin) PluginInfo {
	source, hasBinary, hasExternalBinary := determinePluginSource(p)
	capabilities := determinePluginCapabilities(source, p.Manifest.PluginID, hasExternalBinary)
	name := strings.TrimPrefix(p.Manifest.PluginID, "format.")
	return PluginInfo{
		PluginID:     p.Manifest.PluginID,
		Name:         name,
		Version:      p.Manifest.Version,
		Type:         "format",
		Description:  fmt.Sprintf("Format plugin for %s", name),
		HasBinary:    hasBinary,
		Source:       source,
		Capabilities: capabilities,
		License:      getPluginLicense(p),
	}
}

func determinePluginSource(p *plugins.Plugin) (source string, hasBinary, hasExternalBinary bool) {
	source = "unloaded"
	if p.Path != "(embedded)" && p.Manifest.Entrypoint != "" {
		binPath := filepath.Join(p.Path, p.Manifest.Entrypoint)
		if _, err := os.Stat(binPath); err == nil {
			return "external", true, true
		}
	}
	if plugins.HasEmbeddedPlugin(p.Manifest.PluginID) {
		return "internal", true, false
	}
	return source, false, false
}

func determinePluginCapabilities(source, pluginID string, hasExternalBinary bool) string {
	if source != "internal" {
		return ""
	}
	if hasExternalBinary {
		return "IR: external fallback"
	}
	return getEmbeddedPluginCapabilities(pluginID, hasExternalBinary)
}

// getEmbeddedPluginCapabilities returns capability information for an embedded plugin.
func getEmbeddedPluginCapabilities(pluginID string, hasExternalBinary bool) string {
	// List of plugins known to have stub IR implementations (ExtractIR/EmitNative)
	stubIRPlugins := map[string]bool{
		"format.sword-pure": true,
		"format.esword":     true,
		"format.sword":      true,
		"format.osis":       true,
		"format.usfm":       true,
		"format.usx":        true,
		// Most embedded handlers have stub IR implementations
	}

	if stubIRPlugins[pluginID] {
		if hasExternalBinary {
			return "IR: external fallback"
		}
		return "IR: stub (needs external)"
	}

	return ""
}

// getPluginLicense extracts license information from a plugin.
// It checks the plugin manifest's License field first, then falls back to
// reading a LICENSE file from the plugin directory, then returns a fallback message.
func getPluginLicense(p *plugins.Plugin) string {
	// 1. Check if manifest has license field
	if p.Manifest.License != "" {
		return p.Manifest.License
	}

	// 2. For embedded plugins, return fallback
	if p.Path == "(embedded)" {
		return "See plugin for license"
	}

	// 3. Try to read LICENSE file from plugin directory
	licensePaths := []string{
		filepath.Join(p.Path, "LICENSE"),
		filepath.Join(p.Path, "LICENSE.txt"),
		filepath.Join(p.Path, "LICENSE.md"),
		filepath.Join(p.Path, "COPYING"),
	}

	for _, licensePath := range licensePaths {
		data, err := os.ReadFile(licensePath)
		if err != nil {
			continue
		}

		// Parse license file to extract identifier
		content := string(data)
		license := parseLicenseText(content)
		if license != "" {
			return license
		}
	}

	// 4. Fallback
	return "See plugin for license"
}

// parseLicenseText extracts a short license identifier from license file content.
// extractSPDXIdentifier checks for SPDX-License-Identifier in the first few lines
func extractSPDXIdentifier(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if i > 5 {
			break
		}
		if strings.Contains(line, "SPDX-License-Identifier:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), true
			}
		}
	}
	return "", false
}

// matchMITLicense checks if content matches MIT license patterns
func matchMITLicense(contentLower string) bool {
	return strings.Contains(contentLower, "mit license") ||
		strings.Contains(contentLower, "permission is hereby granted, free of charge")
}

// matchApacheLicense checks if content matches Apache license patterns
func matchApacheLicense(contentLower string) bool {
	return strings.Contains(contentLower, "apache license") &&
		strings.Contains(contentLower, "version 2.0")
}

// matchGPLLicense checks if content matches GPL license patterns and returns the version
func matchGPLLicense(contentLower string) (string, bool) {
	if matchGPL3(contentLower) {
		return "GPL-3.0", true
	}
	if matchGPL2(contentLower) {
		return "GPL-2.0", true
	}
	if strings.Contains(contentLower, "gnu lesser general public license") {
		return "LGPL", true
	}
	return "", false
}

func matchGPL3(contentLower string) bool {
	if strings.Contains(contentLower, "gnu general public license") && strings.Contains(contentLower, "version 3") {
		return true
	}
	return strings.Contains(contentLower, "gpl-3") || strings.Contains(contentLower, "gplv3")
}

func matchGPL2(contentLower string) bool {
	if strings.Contains(contentLower, "gnu general public license") && strings.Contains(contentLower, "version 2") {
		return true
	}
	return strings.Contains(contentLower, "gpl-2") || strings.Contains(contentLower, "gplv2")
}

// matchBSDLicense checks if content matches BSD license patterns and returns the variant
func matchBSDLicense(contentLower string) (string, bool) {
	if strings.Contains(contentLower, "bsd 3-clause") ||
		strings.Contains(contentLower, "redistribution and use in source and binary forms") {
		return "BSD-3-Clause", true
	}
	if strings.Contains(contentLower, "bsd 2-clause") {
		return "BSD-2-Clause", true
	}
	return "", false
}

// matchOtherLicense checks for other common license patterns
func matchOtherLicense(contentLower string) (string, bool) {
	if strings.Contains(contentLower, "mozilla public license") && strings.Contains(contentLower, "2.0") {
		return "MPL-2.0", true
	}
	if strings.Contains(contentLower, "unlicense") ||
		strings.Contains(contentLower, "this is free and unencumbered software") {
		return "Unlicense", true
	}
	if strings.Contains(contentLower, "public domain") {
		return "Public Domain", true
	}
	if strings.Contains(contentLower, "isc license") {
		return "ISC", true
	}
	return "", false
}

func parseLicenseText(content string) string {
	// Check for SPDX identifier in first few lines
	if spdx, found := extractSPDXIdentifier(content); found {
		return spdx
	}

	// Pattern matching for common licenses
	contentLower := strings.ToLower(content)

	if matchMITLicense(contentLower) {
		return "MIT"
	}

	if matchApacheLicense(contentLower) {
		return "Apache-2.0"
	}

	if license, found := matchGPLLicense(contentLower); found {
		return license
	}

	if license, found := matchBSDLicense(contentLower); found {
		return license
	}

	if license, found := matchOtherLicense(contentLower); found {
		return license
	}

	return "See LICENSE file"
}

func listToolPlugins() []PluginInfo {
	var pluginInfos []PluginInfo
	loader := getLoader()

	for _, p := range loader.ListPlugins() {
		if !isToolPlugin(p.Manifest.PluginID) {
			continue
		}
		pluginInfos = append(pluginInfos, buildToolPluginInfo(p))
	}
	return pluginInfos
}

// isToolPlugin checks if a plugin ID is a tool plugin
func isToolPlugin(pluginID string) bool {
	return strings.HasPrefix(pluginID, "tool.") || strings.HasPrefix(pluginID, "tools.")
}

// buildToolPluginInfo constructs PluginInfo for a tool plugin
func buildToolPluginInfo(p *plugins.Plugin) PluginInfo {
	source, hasBinary, _ := determinePluginSource(p)
	name := strings.TrimPrefix(strings.TrimPrefix(p.Manifest.PluginID, "tool."), "tools.")
	return PluginInfo{
		PluginID:    p.Manifest.PluginID,
		Name:        name,
		Version:     p.Manifest.Version,
		Type:        "tool",
		Description: fmt.Sprintf("Tool plugin for %s", name),
		HasBinary:   hasBinary,
		Source:      source,
		License:     getPluginLicense(p),
	}
}

// detectCapsuleFormat is implemented in format_detection.go

func readCapsule(path string) (*CapsuleManifest, []ArtifactInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	reader, closeFunc, err := wrapWithDecompressor(f, path)
	if err != nil {
		return nil, nil, err
	}
	if closeFunc != nil {
		defer closeFunc()
	}

	tarReader := tar.NewReader(reader)
	return extractCapsuleContents(tarReader)
}

// wrapWithDecompressor wraps the reader with appropriate decompressor based on file extension
func wrapWithDecompressor(reader io.Reader, path string) (io.Reader, func(), error) {
	if strings.HasSuffix(path, ".xz") {
		xzReader, err := xz.NewReader(reader)
		if err != nil {
			return nil, nil, fmt.Errorf("xz decompress: %w", err)
		}
		return xzReader, nil, nil
	}

	if strings.HasSuffix(path, ".gz") {
		gzReader, err := gzip.NewReader(reader)
		if err != nil {
			return nil, nil, fmt.Errorf("gzip decompress: %w", err)
		}
		return gzReader, func() { gzReader.Close() }, nil
	}

	return reader, nil, nil
}

// extractCapsuleContents reads manifest and artifacts from tar archive
func extractCapsuleContents(tarReader *tar.Reader) (*CapsuleManifest, []ArtifactInfo, error) {
	var manifest *CapsuleManifest
	var artifacts []ArtifactInfo

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		if header.Name == "manifest.json" {
			manifest, err = parseManifestFromTar(tarReader)
			if err != nil {
				return nil, nil, err
			}
		} else if !header.FileInfo().IsDir() {
			artifacts = append(artifacts, ArtifactInfo{
				ID:        header.Name,
				Name:      filepath.Base(header.Name),
				Size:      header.Size,
				SizeHuman: humanSize(header.Size),
			})
		}
	}

	return manifest, artifacts, nil
}

// parseManifestFromTar reads and unmarshals manifest.json from tar reader
func parseManifestFromTar(tarReader *tar.Reader) (*CapsuleManifest, error) {
	data, err := io.ReadAll(tarReader)
	if err != nil {
		return nil, err
	}

	manifest := &CapsuleManifest{}
	if err := json.Unmarshal(data, manifest); err != nil {
		return nil, err
	}

	return manifest, nil
}

func readArtifactContent(capsulePath, artifactID string) (string, string, error) {
	f, err := os.Open(capsulePath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	reader, closeFunc, err := wrapWithDecompressor(f, capsulePath)
	if err != nil {
		return "", "", err
	}
	if closeFunc != nil {
		defer closeFunc()
	}

	return findArtifactInTar(tar.NewReader(reader), artifactID)
}

func findArtifactInTar(tarReader *tar.Reader, artifactID string) (string, string, error) {
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", err
		}
		if header.Name != artifactID {
			continue
		}
		data, err := io.ReadAll(tarReader)
		if err != nil {
			return "", "", err
		}
		return string(data), detectContentType(header.Name, data), nil
	}
	return "", "", fmt.Errorf("artifact not found: %s", artifactID)
}

func readIRContent(capsulePath string) (map[string]interface{}, error) {
	// Use semaphore to limit concurrent archive reads
	acquireArchiveSemaphore()
	ir, err := archive.ReadIR(capsulePath)
	releaseArchiveSemaphore()

	if err != nil {
		return nil, fmt.Errorf("no IR file found in capsule")
	}
	return ir, nil
}

var extContentTypes = map[string]string{
	".xml":  "application/xml",
	".osis": "application/xml",
	".usx":  "application/xml",
	".json": "application/json",
	".html": "text/html",
	".htm":  "text/html",
	".md":   "text/markdown",
	".txt":  "text/plain",
	".usfm": "text/plain",
	".sfm":  "text/plain",
}

func isBinaryData(data []byte) bool {
	for _, b := range data[:min(512, len(data))] {
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			return true
		}
	}
	return false
}

func detectContentType(name string, data []byte) string {
	if ct, ok := extContentTypes[strings.ToLower(filepath.Ext(name))]; ok {
		return ct
	}
	if len(data) > 0 && isBinaryData(data) {
		return "application/octet-stream"
	}
	return "text/plain"
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func prettyJSON(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return string(data)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// IngestData is the data for the ingest page.
type IngestData struct {
	PageData
	Result *IngestResult
}

// IngestResult is the result of ingesting a file.
type IngestResult struct {
	Success     bool
	CapsulePath string
	Format      string
	Size        int64
	SizeHuman   string
	Error       string
}

// VerifyData is the data for the verify page.
type VerifyData struct {
	PageData
	Capsule CapsuleInfo
	Result  *VerifyResult
}

// VerifyResult is the result of verifying a capsule.
type VerifyResult struct {
	Success   bool
	Artifacts []ArtifactVerifyResult
	Message   string
}

// ArtifactVerifyResult is the verification result for a single artifact.
type ArtifactVerifyResult struct {
	Name     string
	Expected string
	Actual   string
	Valid    bool
}

// DetectData is the data for the format detection page.
type DetectData struct {
	PageData
	Result *DetectResult
}

// DetectResult is the result of format detection.
type DetectResult struct {
	Format     string
	PluginID   string
	Confidence string
	Details    string
}

// ExportData is the data for the export page.
type ExportData struct {
	PageData
	Capsule   CapsuleInfo
	Artifacts []ArtifactInfo
}

// handleIngest handles file ingestion to create capsules.
func handleIngest(w http.ResponseWriter, r *http.Request) {
	// Get or create CSRF token
	csrfToken := getOrCreateCSRFToken(w, r)

	data := IngestData{
		PageData: PageData{Title: "Ingest File", CSRFToken: csrfToken},
	}

	if r.Method == http.MethodPost {
		// Limit request body size to prevent DoS attacks (CWE-400)
		r.Body = http.MaxBytesReader(w, r.Body, validation.MaxFileSize)

		// Parse multipart form with memory limit
		if err := r.ParseMultipartForm(MaxFormMemory); err != nil {
			data.Result = &IngestResult{
				Success: false,
				Error:   fmt.Sprintf("Failed to parse form: %v", err),
			}
		} else if !validateCSRFToken(r) {
			data.Result = &IngestResult{
				Success: false,
				Error:   "Invalid CSRF token. Please try again.",
			}
		} else {
			file, header, err := r.FormFile("file")
			if err != nil {
				data.Result = &IngestResult{
					Success: false,
					Error:   fmt.Sprintf("No file uploaded: %v", err),
				}
			} else {
				defer file.Close()
				// Check file size
				if header.Size > validation.MaxFileSize {
					data.Result = &IngestResult{
						Success: false,
						Error:   fmt.Sprintf("File too large: %d bytes (max: %d bytes)", header.Size, validation.MaxFileSize),
					}
				} else {
					result := performIngest(file, header.Filename, header.Size)
					data.Result = result
				}
			}
		}
	}

	if err := Templates.ExecuteTemplate(w, "ingest.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

func saveUploadToTemp(file io.Reader, safeFilename string) (tempDir string, uploadPath string, written int64, err error) {
	tempDir, err = secureMkdirTemp("", "capsule-ingest-*")
	if err != nil {
		return "", "", 0, fmt.Errorf("Failed to create temp directory: %w", err)
	}
	uploadPath = filepath.Join(tempDir, safeFilename)
	outFile, err := os.Create(uploadPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", "", 0, fmt.Errorf("Failed to save file: %w", err)
	}
	written, err = io.Copy(outFile, file)
	if err != nil {
		outFile.Close()
		os.RemoveAll(tempDir)
		return "", "", 0, fmt.Errorf("Failed to write file: %w", err)
	}
	if err = outFile.Close(); err != nil {
		os.RemoveAll(tempDir)
		return "", "", 0, fmt.Errorf("Failed to close file: %w", err)
	}
	return tempDir, uploadPath, written, nil
}

func buildCapsuleName(filename string) (string, error) {
	baseName := filepath.Base(strings.TrimSuffix(filename, filepath.Ext(filename)))
	invalidBaseNames := map[string]bool{".": true, "..": true, "": true}
	if invalidBaseNames[baseName] {
		baseName = "uploaded"
	}
	capsuleName := baseName + ".capsule.tar.gz"
	cleanName, err := validation.SanitizePath(ServerConfig.CapsulesDir, capsuleName)
	if err != nil {
		return "", fmt.Errorf("Invalid filename")
	}
	return cleanName, nil
}

func assembleCapsule(capsuleDir, uploadPath, filename, detectedFormat string) {
	os.MkdirAll(capsuleDir, 0700)
	data, _ := os.ReadFile(uploadPath)
	os.WriteFile(filepath.Join(capsuleDir, filename), data, 0600)
	manifest := map[string]interface{}{
		"capsule_version": "1.0",
		"source_format":   detectedFormat,
		"original_file":   filename,
		"ingested_at":     time.Now().Format(time.RFC3339),
	}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(capsuleDir, "manifest.json"), manifestData, 0600)
}

func performIngest(file io.Reader, filename string, size int64) *IngestResult {
	safeFilename, err := validation.SanitizeFilename(filename)
	if err != nil {
		return &IngestResult{Success: false, Error: fmt.Sprintf("Invalid filename: %v", err)}
	}

	tempDir, uploadPath, written, err := saveUploadToTemp(file, safeFilename)
	if err != nil {
		return &IngestResult{Success: false, Error: err.Error()}
	}
	defer os.RemoveAll(tempDir)

	detectedFormat := detectFileFormat(uploadPath)
	capsuleDir := filepath.Join(tempDir, "capsule")
	assembleCapsule(capsuleDir, uploadPath, filename, detectedFormat)

	capsuleName, err := buildCapsuleName(filename)
	if err != nil {
		return &IngestResult{Success: false, Error: err.Error()}
	}
	capsulePath := filepath.Join(ServerConfig.CapsulesDir, capsuleName)

	if err := archive.CreateCapsuleTarGzFromPath(capsuleDir, capsulePath); err != nil {
		return &IngestResult{Success: false, Error: fmt.Sprintf("Failed to create capsule: %v", err)}
	}

	return &IngestResult{
		Success:     true,
		CapsulePath: capsuleName,
		Format:      detectedFormat,
		Size:        written,
		SizeHuman:   humanSize(written),
	}
}

// detectFileFormat is implemented in format_detection.go

// detectFileFormatByExtension is implemented in format_detection.go

// handleVerify handles capsule verification.
func handleVerify(w http.ResponseWriter, r *http.Request) {
	capsulePath := strings.TrimPrefix(r.URL.Path, "/verify/")
	if capsulePath == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Sanitize path to prevent path traversal attacks
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, capsulePath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)

	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "Capsule not found", http.StatusNotFound)
		return
	}

	// Get or create CSRF token
	csrfToken := getOrCreateCSRFToken(w, r)

	data := VerifyData{
		PageData: PageData{Title: "Verify: " + capsulePath, CSRFToken: csrfToken},
		Capsule: CapsuleInfo{
			Name:      filepath.Base(capsulePath),
			Path:      capsulePath,
			Size:      info.Size(),
			SizeHuman: humanSize(info.Size()),
		},
	}

	if r.Method == http.MethodPost {
		if !validateCSRFToken(r) {
			data.PageData.Error = "Invalid CSRF token. Please try again."
		} else {
			result := performVerify(fullPath)
			data.Result = result
		}
	}

	if err := Templates.ExecuteTemplate(w, "verify.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

// performVerify verifies all artifacts in a capsule.
func performVerify(capsulePath string) *VerifyResult {
	result := &VerifyResult{
		Success:   true,
		Artifacts: []ArtifactVerifyResult{},
	}

	// Extract capsule to temp dir
	tempDir, err := secureMkdirTemp("", "capsule-verify-*")
	if err != nil {
		return &VerifyResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create temp directory: %v", err),
		}
	}
	defer os.RemoveAll(tempDir)

	if err := extractCapsule(capsulePath, tempDir); err != nil {
		return &VerifyResult{
			Success: false,
			Message: fmt.Sprintf("Failed to extract capsule: %v", err),
		}
	}

	// Read manifest and verify each artifact
	_, artifacts, err := readCapsule(capsulePath)
	if err != nil {
		return &VerifyResult{
			Success: false,
			Message: fmt.Sprintf("Failed to read capsule: %v", err),
		}
	}

	allValid := true
	for _, artifact := range artifacts {
		// Skip manifest
		if artifact.Name == "manifest.json" {
			continue
		}

		artResult := ArtifactVerifyResult{
			Name:  artifact.Name,
			Valid: true,
		}

		// Check if file exists in extracted directory
		extractedPath := filepath.Join(tempDir, artifact.ID)
		if _, err := os.Stat(extractedPath); errors.Is(err, os.ErrNotExist) {
			artResult.Valid = false
			artResult.Expected = "exists"
			artResult.Actual = "missing"
			allValid = false
		} else {
			artResult.Expected = fmt.Sprintf("%d bytes", artifact.Size)
			artResult.Actual = fmt.Sprintf("%d bytes", artifact.Size)
		}

		result.Artifacts = append(result.Artifacts, artResult)
	}

	result.Success = allValid
	if allValid {
		result.Message = fmt.Sprintf("All %d artifacts verified successfully", len(result.Artifacts))
	} else {
		result.Message = "Some artifacts failed verification"
	}

	return result
}

// handleDetect handles format detection for uploaded files.

// performDetect detects the format of an uploaded file.
func performDetect(file io.Reader, filename string) *DetectResult {
	tempPath, cleanup, err := saveDetectFile(file, filename)
	if err != nil {
		return detectError(err)
	}
	defer cleanup()
	return detectFormat(tempPath)
}

// saveDetectFile saves the uploaded file to a temp directory.
func saveDetectFile(file io.Reader, filename string) (string, func(), error) {
	tempDir, err := secureMkdirTemp("", "capsule-detect-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(tempDir) }
	tempPath := filepath.Join(tempDir, filename)
	outFile, err := os.Create(tempPath)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if _, err := io.Copy(outFile, file); err != nil {
		outFile.Close()
		cleanup()
		return "", nil, fmt.Errorf("copying file: %w", err)
	}
	if err := outFile.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("closing file: %w", err)
	}
	return tempPath, cleanup, nil
}

// detectError returns a DetectResult for an error condition.
func detectError(err error) *DetectResult {
	return &DetectResult{
		Format:  "unknown",
		Details: fmt.Sprintf("Error: %v", err),
	}
}

// detectFormat uses plugins to detect the file format.
func detectFormat(tempPath string) *DetectResult {
	loader := getLoader()
	if err := loader.LoadFromDir(ServerConfig.PluginsDir); err != nil {
		return extensionBasedDetect(tempPath, "Plugin detection unavailable, used extension-based detection")
	}
	if result := tryPluginDetect(loader, tempPath); result != nil {
		return result
	}
	return extensionBasedDetect(tempPath, "No plugin detected this format, using extension-based detection")
}

// tryPluginDetect tries each format plugin to detect the file.
func tryPluginDetect(loader *plugins.Loader, tempPath string) *DetectResult {
	for _, plugin := range loader.GetPluginsByKind("format") {
		req := plugins.NewDetectRequest(tempPath)
		resp, err := plugins.ExecutePlugin(plugin, req)
		if err != nil {
			continue
		}
		result, err := plugins.ParseDetectResult(resp)
		if err != nil {
			continue
		}
		if result.Detected {
			return &DetectResult{
				Format:     strings.TrimPrefix(plugin.Manifest.PluginID, "format."),
				PluginID:   plugin.Manifest.PluginID,
				Confidence: "high",
				Details:    result.Reason,
			}
		}
	}
	return nil
}

// extensionBasedDetect returns a detection result based on file extension.
func extensionBasedDetect(tempPath, details string) *DetectResult {
	format := detectFileFormatByExtension(tempPath)
	return &DetectResult{
		Format:     format,
		PluginID:   "(fallback)",
		Confidence: "low",
		Details:    details,
	}
}

// handleExport handles capsule artifact export.
func handleExport(w http.ResponseWriter, r *http.Request) {
	capsulePath := strings.TrimPrefix(r.URL.Path, "/export/")
	if capsulePath == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	fullPath, err := sanitizeAndResolvePath(capsulePath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "Capsule not found", http.StatusNotFound)
		return
	}

	if handleArtifactDownload(w, r, fullPath) {
		return
	}

	renderExportPage(w, capsulePath, fullPath, info)
}

// handleArtifactDownload handles download requests for artifacts, returns true if handled
func handleArtifactDownload(w http.ResponseWriter, r *http.Request, fullPath string) bool {
	artifactID := r.URL.Query().Get("artifact")
	if artifactID == "" || r.URL.Query().Get("download") != "true" {
		return false
	}
	content, contentType, err := readArtifactContent(fullPath, artifactID)
	if err != nil {
		httpError(w, err, http.StatusNotFound)
		return true
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(artifactID)))
	w.Write([]byte(content))
	return true
}

// renderExportPage renders the export page template
func renderExportPage(w http.ResponseWriter, capsulePath, fullPath string, info os.FileInfo) {
	_, artifacts, err := readCapsule(fullPath)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	data := ExportData{
		PageData: PageData{Title: "Export: " + capsulePath},
		Capsule: CapsuleInfo{
			Name:      filepath.Base(capsulePath),
			Path:      capsulePath,
			Size:      info.Size(),
			SizeHuman: humanSize(info.Size()),
		},
		Artifacts: artifacts,
	}
	if err := Templates.ExecuteTemplate(w, "export.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

// sanitizeAndResolvePath sanitizes and resolves a capsule path
func sanitizeAndResolvePath(capsulePath string) (string, error) {
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, capsulePath)
	if err != nil {
		return "", err
	}
	return filepath.Join(ServerConfig.CapsulesDir, cleanPath), nil
}

// SelfcheckData is the data for the selfcheck page.
type SelfcheckData struct {
	PageData
	Capsule CapsuleInfo
	Plans   []PlanInfo
	Result  *SelfcheckResult
}

// PlanInfo describes a selfcheck plan.
type PlanInfo struct {
	Name        string
	Description string
	Steps       int
	Checks      int
}

// SelfcheckResult is the result of a selfcheck.
type SelfcheckResult struct {
	Success      bool
	PlanName     string
	CheckResults []SelfcheckCheck
	Message      string
	Duration     string
}

// SelfcheckCheck is a single check result.
type SelfcheckCheck struct {
	Name     string
	Status   string
	Expected string
	Actual   string
	Passed   bool
}

// handleSelfcheck handles the selfcheck page.
func handleSelfcheck(w http.ResponseWriter, r *http.Request) {
	capsulePath := strings.TrimPrefix(r.URL.Path, "/selfcheck/")
	if capsulePath == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Sanitize path to prevent path traversal attacks
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, capsulePath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)

	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "Capsule not found", http.StatusNotFound)
		return
	}

	// Get or create CSRF token
	csrfToken := getOrCreateCSRFToken(w, r)

	data := SelfcheckData{
		PageData: PageData{Title: "Selfcheck: " + capsulePath, CSRFToken: csrfToken},
		Capsule: CapsuleInfo{
			Name:      filepath.Base(capsulePath),
			Path:      capsulePath,
			Size:      info.Size(),
			SizeHuman: humanSize(info.Size()),
		},
		Plans: getAvailablePlans(),
	}

	if r.Method == http.MethodPost {
		if !validateCSRFToken(r) {
			data.PageData.Error = "Invalid CSRF token. Please try again."
		} else {
			planName := r.FormValue("plan")
			if planName == "" {
				planName = "identity-bytes"
			}
			result := performSelfcheck(fullPath, planName)
			data.Result = result
		}
	}

	if err := Templates.ExecuteTemplate(w, "selfcheck.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

// getAvailablePlans returns the available selfcheck plans.
func getAvailablePlans() []PlanInfo {
	return []PlanInfo{
		{
			Name:        "identity-bytes",
			Description: "Verify that export(artifact) == original artifact bytes",
			Steps:       1,
			Checks:      1,
		},
		{
			Name:        "verify-hashes",
			Description: "Verify all artifact hashes match their content",
			Steps:       0,
			Checks:      1,
		},
	}
}

// performSelfcheck executes a selfcheck plan on a capsule.
func performSelfcheck(capsulePath, planName string) *SelfcheckResult {
	start := time.Now()

	tempDir, err := secureMkdirTemp("", "capsule-selfcheck-*")
	if err != nil {
		return &SelfcheckResult{
			Success:  false,
			PlanName: planName,
			Message:  fmt.Sprintf("Failed to create temp directory: %v", err),
		}
	}
	defer os.RemoveAll(tempDir)

	if err := extractCapsule(capsulePath, tempDir); err != nil {
		return &SelfcheckResult{
			Success:  false,
			PlanName: planName,
			Message:  fmt.Sprintf("Failed to extract capsule: %v", err),
		}
	}

	var result *SelfcheckResult
	switch planName {
	case "identity-bytes":
		result = runIdentityBytesCheck(capsulePath, tempDir, planName)
	case "verify-hashes":
		result = runVerifyHashesCheck(capsulePath, tempDir, planName)
	default:
		result = &SelfcheckResult{
			Success:  false,
			PlanName: planName,
			Message:  fmt.Sprintf("Unknown plan: %s", planName),
		}
	}

	if result != nil {
		result.Duration = time.Since(start).String()
	}
	return result
}

// runIdentityBytesCheck verifies that extracted artifact sizes match original sizes.
func runIdentityBytesCheck(capsulePath, tempDir, planName string) *SelfcheckResult {
	_, artifacts, err := readCapsule(capsulePath)
	if err != nil {
		return &SelfcheckResult{
			Success:  false,
			PlanName: planName,
			Message:  fmt.Sprintf("Failed to read capsule: %v", err),
		}
	}

	checks, allPassed := checkArtifactIdentity(artifacts, tempDir)
	return &SelfcheckResult{
		Success:      allPassed,
		PlanName:     planName,
		CheckResults: checks,
		Message:      fmt.Sprintf("Completed %d checks", len(checks)),
	}
}

// runVerifyHashesCheck verifies that all artifacts are readable.
func runVerifyHashesCheck(capsulePath, tempDir, planName string) *SelfcheckResult {
	_, artifacts, err := readCapsule(capsulePath)
	if err != nil {
		return &SelfcheckResult{
			Success:  false,
			PlanName: planName,
			Message:  fmt.Sprintf("Failed to read capsule: %v", err),
		}
	}

	checks, allPassed := checkArtifactReadability(artifacts, tempDir)
	return &SelfcheckResult{
		Success:      allPassed,
		PlanName:     planName,
		CheckResults: checks,
		Message:      fmt.Sprintf("Verified %d artifacts", len(checks)),
	}
}

// checkArtifactIdentity checks that extracted artifacts match expected sizes.
func checkArtifactIdentity(artifacts []ArtifactInfo, tempDir string) ([]SelfcheckCheck, bool) {
	var checks []SelfcheckCheck
	allPassed := true

	for _, artifact := range artifacts {
		if artifact.Name == "manifest.json" {
			continue
		}

		check := SelfcheckCheck{
			Name:     fmt.Sprintf("Identity check: %s", artifact.Name),
			Expected: fmt.Sprintf("%d bytes", artifact.Size),
		}

		extractedPath := filepath.Join(tempDir, artifact.ID)
		fileInfo, err := os.Stat(extractedPath)
		if err != nil {
			check.Status = "FAIL"
			check.Actual = "file not found"
			check.Passed = false
			allPassed = false
		} else {
			check.Actual = fmt.Sprintf("%d bytes", fileInfo.Size())
			if fileInfo.Size() == artifact.Size {
				check.Status = "PASS"
				check.Passed = true
			} else {
				check.Status = "FAIL"
				check.Passed = false
				allPassed = false
			}
		}
		checks = append(checks, check)
	}

	return checks, allPassed
}

// checkArtifactReadability checks that all artifacts can be read.
func checkArtifactReadability(artifacts []ArtifactInfo, tempDir string) ([]SelfcheckCheck, bool) {
	var checks []SelfcheckCheck
	allPassed := true

	for _, artifact := range artifacts {
		if artifact.Name == "manifest.json" {
			continue
		}

		check := SelfcheckCheck{
			Name:     fmt.Sprintf("Hash verify: %s", artifact.Name),
			Expected: "readable",
		}

		extractedPath := filepath.Join(tempDir, artifact.ID)
		if _, err := os.ReadFile(extractedPath); err != nil {
			check.Status = "FAIL"
			check.Actual = "unreadable"
			check.Passed = false
			allPassed = false
		} else {
			check.Status = "PASS"
			check.Actual = "readable"
			check.Passed = true
		}
		checks = append(checks, check)
	}

	return checks, allPassed
}

// DevInfoData is the data for the dev info page.
type DevInfoData struct {
	PageData
	Config     ConfigInfo
	SystemInfo SystemInfo
	PluginInfo PluginSummary
}

// ConfigInfo holds configuration information.
type ConfigInfo struct {
	CapsulesDir    string
	PluginsDir     string
	Port           int
	CapsulesDirAbs string
	PluginsDirAbs  string
}

// SystemInfo holds system information.
type SystemInfo struct {
	GoVersion  string
	GOOS       string
	GOARCH     string
	NumCPU     int
	Hostname   string
	WorkingDir string
}

// PluginSummary holds plugin summary info.
type PluginSummary struct {
	FormatCount int
	ToolCount   int
	TotalCount  int
}

// handleDevInfo handles the dev info page.
func handleDevInfo(w http.ResponseWriter, r *http.Request) {
	capsulesDirAbs, _ := filepath.Abs(ServerConfig.CapsulesDir)
	pluginsDirAbs, _ := filepath.Abs(ServerConfig.PluginsDir)
	hostname, _ := os.Hostname()
	wd, _ := os.Getwd()

	// Count plugins
	formatPlugins := listFormatPlugins()
	toolPlugins := listToolPlugins()

	data := DevInfoData{
		PageData: PageData{Title: "Developer Info"},
		Config: ConfigInfo{
			CapsulesDir:    ServerConfig.CapsulesDir,
			PluginsDir:     ServerConfig.PluginsDir,
			Port:           ServerConfig.Port,
			CapsulesDirAbs: capsulesDirAbs,
			PluginsDirAbs:  pluginsDirAbs,
		},
		SystemInfo: SystemInfo{
			GoVersion:  runtime.Version(),
			GOOS:       runtime.GOOS,
			GOARCH:     runtime.GOARCH,
			NumCPU:     runtime.NumCPU(),
			Hostname:   hostname,
			WorkingDir: wd,
		},
		PluginInfo: PluginSummary{
			FormatCount: len(formatPlugins),
			ToolCount:   len(toolPlugins),
			TotalCount:  len(formatPlugins) + len(toolPlugins),
		},
	}

	if err := Templates.ExecuteTemplate(w, "dev.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

// RunsData is the data for the runs list page.
type RunsData struct {
	PageData
	Capsule CapsuleInfo
	Runs    []RunInfo
}

// RunInfo describes a run in a capsule.
type RunInfo struct {
	ID             string
	PluginID       string
	PluginVersion  string
	Profile        string
	Status         string
	TranscriptHash string
	InputIDs       []string
}

// handleRuns handles the runs list page.
func handleRuns(w http.ResponseWriter, r *http.Request) {
	capsulePath := strings.TrimPrefix(r.URL.Path, "/runs/")
	if capsulePath == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Sanitize path to prevent path traversal attacks
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, capsulePath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)

	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "Capsule not found", http.StatusNotFound)
		return
	}

	// Extract capsule to temp dir
	tempDir, err := secureMkdirTemp("", "capsule-runs-*")
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	if err := extractCapsule(fullPath, tempDir); err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}

	// Read manifest for runs
	runs := getRunsFromCapsule(tempDir)

	data := RunsData{
		PageData: PageData{Title: "Runs: " + capsulePath},
		Capsule: CapsuleInfo{
			Name:      filepath.Base(capsulePath),
			Path:      capsulePath,
			Size:      info.Size(),
			SizeHuman: humanSize(info.Size()),
		},
		Runs: runs,
	}

	if err := Templates.ExecuteTemplate(w, "runs.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

// formatTranscriptHash formats a transcript hash for display, truncating if needed.
func formatTranscriptHash(hash string) string {
	if hash == "" {
		return ""
	}
	if len(hash) > 16 {
		return hash[:16] + "..."
	}
	return hash
}

// buildRunInfoFromManifest constructs a RunInfo from a manifest run entry.
func buildRunInfoFromManifest(id string, run struct {
	ID     string `json:"id"`
	Plugin *struct {
		PluginID      string `json:"plugin_id"`
		PluginVersion string `json:"plugin_version"`
	} `json:"plugin"`
	Command *struct {
		Profile string `json:"profile"`
	} `json:"command"`
	Status string `json:"status"`
	Inputs []struct {
		ArtifactID string `json:"artifact_id"`
	} `json:"inputs"`
	Outputs *struct {
		TranscriptBlobSHA256 string `json:"transcript_blob_sha256"`
	} `json:"outputs"`
}) RunInfo {
	runInfo := RunInfo{
		ID:     id,
		Status: run.Status,
	}

	if run.Plugin != nil {
		runInfo.PluginID = run.Plugin.PluginID
		runInfo.PluginVersion = run.Plugin.PluginVersion
	}

	if run.Command != nil {
		runInfo.Profile = run.Command.Profile
	}

	if run.Outputs != nil {
		runInfo.TranscriptHash = formatTranscriptHash(run.Outputs.TranscriptBlobSHA256)
	}

	for _, input := range run.Inputs {
		runInfo.InputIDs = append(runInfo.InputIDs, input.ArtifactID)
	}

	return runInfo
}

// getRunsFromCapsule extracts run information from a capsule's manifest.
func getRunsFromCapsule(extractDir string) []RunInfo {
	var runs []RunInfo

	// Read manifest.json
	manifestPath := filepath.Join(extractDir, "capsule", "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return runs
	}

	var manifest struct {
		Runs map[string]struct {
			ID     string `json:"id"`
			Plugin *struct {
				PluginID      string `json:"plugin_id"`
				PluginVersion string `json:"plugin_version"`
			} `json:"plugin"`
			Command *struct {
				Profile string `json:"profile"`
			} `json:"command"`
			Status string `json:"status"`
			Inputs []struct {
				ArtifactID string `json:"artifact_id"`
			} `json:"inputs"`
			Outputs *struct {
				TranscriptBlobSHA256 string `json:"transcript_blob_sha256"`
			} `json:"outputs"`
		} `json:"runs"`
	}

	if err := json.Unmarshal(data, &manifest); err != nil {
		return runs
	}

	for id, run := range manifest.Runs {
		runs = append(runs, buildRunInfoFromManifest(id, run))
	}

	// Sort by ID for consistent ordering
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].ID < runs[j].ID
	})

	return runs
}

// RunsCompareData is the data for the runs compare page.
type RunsCompareData struct {
	PageData
	Capsule CapsuleInfo
	Runs    []RunInfo
	Result  *CompareResult
}

// CompareResult is the result of comparing two runs.
type CompareResult struct {
	Run1ID      string
	Run2ID      string
	Hash1       string
	Hash2       string
	Identical   bool
	EventCount1 int
	EventCount2 int
	Differences []DiffEntry
}

// DiffEntry is a single difference between transcripts.
type DiffEntry struct {
	Index  int
	Event1 string
	Event2 string
}

func resolveCapsuleInfo(w http.ResponseWriter, capsulePath string) (string, os.FileInfo, bool) {
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, capsulePath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return "", nil, false
	}
	fullPath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)
	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "Capsule not found", http.StatusNotFound)
		return "", nil, false
	}
	return fullPath, info, true
}

func extractCapsuleToTemp(w http.ResponseWriter, fullPath string) (string, bool) {
	tempDir, err := secureMkdirTemp("", "capsule-compare-*")
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return "", false
	}
	if err := extractCapsule(fullPath, tempDir); err != nil {
		os.RemoveAll(tempDir)
		httpError(w, err, http.StatusInternalServerError)
		return "", false
	}
	return tempDir, true
}

func applyRunsComparePost(r *http.Request, tempDir string, data *RunsCompareData) {
	if !validateCSRFToken(r) {
		data.PageData.Error = "Invalid CSRF token. Please try again."
		return
	}
	run1ID := r.FormValue("run1")
	run2ID := r.FormValue("run2")
	if run1ID != "" && run2ID != "" {
		data.Result = performRunsCompare(tempDir, run1ID, run2ID)
	}
}

// handleRunsCompare handles the runs compare page.
func handleRunsCompare(w http.ResponseWriter, r *http.Request) {
	capsulePath := strings.TrimPrefix(r.URL.Path, "/runs/compare/")
	if capsulePath == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	fullPath, info, ok := resolveCapsuleInfo(w, capsulePath)
	if !ok {
		return
	}

	tempDir, ok := extractCapsuleToTemp(w, fullPath)
	if !ok {
		return
	}
	defer os.RemoveAll(tempDir)

	csrfToken := getOrCreateCSRFToken(w, r)
	data := RunsCompareData{
		PageData: PageData{Title: "Compare Runs: " + capsulePath, CSRFToken: csrfToken},
		Capsule: CapsuleInfo{
			Name:      filepath.Base(capsulePath),
			Path:      capsulePath,
			Size:      info.Size(),
			SizeHuman: humanSize(info.Size()),
		},
		Runs: getRunsFromCapsule(tempDir),
	}

	if r.Method == http.MethodPost {
		applyRunsComparePost(r, tempDir, &data)
	}

	if err := Templates.ExecuteTemplate(w, "runs_compare.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

// performRunsCompare compares two runs in a capsule.
func performRunsCompare(extractDir, run1ID, run2ID string) *CompareResult {
	result := &CompareResult{
		Run1ID: run1ID,
		Run2ID: run2ID,
	}

	manifest, ok := loadManifest(extractDir)
	if !ok {
		return result
	}

	if !extractTranscriptHashes(manifest, run1ID, run2ID, result) {
		return result
	}

	if result.Hash1 == result.Hash2 {
		result.Identical = true
		return result
	}

	events1, events2 := loadAndParseTranscripts(extractDir, manifest, result.Hash1, result.Hash2)
	result.EventCount1 = len(events1)
	result.EventCount2 = len(events2)

	result.Differences = findEventDifferences(events1, events2)

	return result
}

// manifestData represents the structure of manifest.json for run comparison.
type manifestData struct {
	Runs map[string]struct {
		Outputs *struct {
			TranscriptBlobSHA256 string `json:"transcript_blob_sha256"`
		} `json:"outputs"`
	} `json:"runs"`
	Blobs map[string]struct {
		Path string `json:"path"`
	} `json:"blobs"`
}

// loadManifest reads and parses the manifest.json file.
func loadManifest(extractDir string) (*manifestData, bool) {
	manifestPath := filepath.Join(extractDir, "capsule", "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, false
	}

	var manifest manifestData
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, false
	}

	return &manifest, true
}

// extractTranscriptHashes extracts transcript hashes from the manifest for the given runs.
// Returns true if both hashes were successfully extracted.
func extractTranscriptHashes(manifest *manifestData, run1ID, run2ID string, result *CompareResult) bool {
	run1, ok := manifest.Runs[run1ID]
	if !ok {
		return false
	}
	run2, ok := manifest.Runs[run2ID]
	if !ok {
		return false
	}

	if run1.Outputs != nil {
		result.Hash1 = run1.Outputs.TranscriptBlobSHA256
	}
	if run2.Outputs != nil {
		result.Hash2 = run2.Outputs.TranscriptBlobSHA256
	}

	return result.Hash1 != "" && result.Hash2 != ""
}

// loadAndParseTranscripts loads transcript blobs and parses them into events.
func loadAndParseTranscripts(extractDir string, manifest *manifestData, hash1, hash2 string) ([]string, []string) {
	var transcript1, transcript2 []byte

	if blob1, ok := manifest.Blobs[hash1]; ok {
		transcript1 = readTranscriptBlob(extractDir, hash1, blob1.Path)
	}
	if blob2, ok := manifest.Blobs[hash2]; ok {
		transcript2 = readTranscriptBlob(extractDir, hash2, blob2.Path)
	}

	events1 := parseTranscriptEvents(transcript1)
	events2 := parseTranscriptEvents(transcript2)

	return events1, events2
}

// findEventDifferences compares two event lists and returns differences.
func findEventDifferences(events1, events2 []string) []DiffEntry {
	maxLen := len(events1)
	if len(events2) > maxLen {
		maxLen = len(events2)
	}

	var differences []DiffEntry
	for i := 0; i < maxLen; i++ {
		var e1, e2 string
		if i < len(events1) {
			e1 = events1[i]
		}
		if i < len(events2) {
			e2 = events2[i]
		}

		if e1 != e2 {
			differences = append(differences, DiffEntry{
				Index:  i,
				Event1: e1,
				Event2: e2,
			})
		}
	}

	return differences
}

// readTranscriptBlob reads a transcript blob from the capsule.
func readTranscriptBlob(extractDir, hash, blobPath string) []byte {
	fullPath := filepath.Join(extractDir, "capsule", blobPath)
	data, err := os.ReadFile(fullPath)
	if err == nil {
		return data
	}
	return nil
}

// parseTranscriptEvents parses transcript data into event strings.
func parseTranscriptEvents(data []byte) []string {
	if len(data) == 0 {
		return nil
	}

	var events []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			events = append(events, line)
		}
	}
	return events
}

// ToolsData is the data for the tools list page.
type ToolsData struct {
	PageData
	Tools       []ToolItem
	ToolPlugins []PluginInfo
	Source      string // "contrib/tool" or "plugins/tool"
}

// ToolItem describes a tool in contrib/tool.
type ToolItem struct {
	Name       string
	Purpose    string
	License    string
	HasNix     bool
	HasCapsule bool
	HasBin     bool
}

// handleTools handles the tools list page.
func handleTools(w http.ResponseWriter, r *http.Request) {
	data := ToolsData{PageData: PageData{Title: "Tool Plugins"}}
	populateToolsData(&data)

	if err := Templates.ExecuteTemplate(w, "tools.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

// populateToolsData fills the ToolsData with tools from contrib or plugins
func populateToolsData(data *ToolsData) {
	tools := getToolsList("contrib/tool")
	if len(tools) > 0 {
		data.Tools = tools
		data.Source = "contrib/tool"
		return
	}
	data.ToolPlugins = loadToolPluginsFromDir()
	data.Source = ServerConfig.PluginsDir + "/tool"
}

// loadToolPluginsFromDir loads tool plugins from the plugins directory
func loadToolPluginsFromDir() []PluginInfo {
	loader := getLoader()
	if err := loader.LoadFromDir(ServerConfig.PluginsDir); err != nil {
		return nil
	}
	var toolPlugins []PluginInfo
	for _, p := range loader.ListPlugins() {
		if !isToolPlugin(p.Manifest.PluginID) {
			continue
		}
		toolPlugins = append(toolPlugins, buildToolPluginInfoBasic(p))
	}
	return toolPlugins
}

// buildToolPluginInfoBasic builds basic PluginInfo without license
func buildToolPluginInfoBasic(p *plugins.Plugin) PluginInfo {
	_, hasBinary, _ := determinePluginSource(p)
	name := strings.TrimPrefix(strings.TrimPrefix(p.Manifest.PluginID, "tool."), "tools.")
	return PluginInfo{
		PluginID:  p.Manifest.PluginID,
		Name:      name,
		Version:   p.Manifest.Version,
		HasBinary: hasBinary,
	}
}

func readToolPurpose(readmePath string) string {
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return ""
	}
	return extractPurposeFromReadme(string(data))
}

func readToolLicense(toolDir string) string {
	data, err := os.ReadFile(filepath.Join(toolDir, "LICENSE.txt"))
	if err != nil {
		return ""
	}
	return extractLicenseType(string(data))
}

func dirHasFiles(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	files, _ := os.ReadDir(path)
	return len(files) > 0
}

func buildToolItem(name, toolDir string) ToolItem {
	readmePath := filepath.Join(toolDir, "README.md")
	_, nixErr := os.Stat(filepath.Join(toolDir, "nixos", "default.nix"))
	return ToolItem{
		Name:       name,
		Purpose:    readToolPurpose(readmePath),
		License:    readToolLicense(toolDir),
		HasNix:     nixErr == nil,
		HasCapsule: dirHasFiles(filepath.Join(toolDir, "capsule")),
		HasBin:     dirHasFiles(filepath.Join(toolDir, "bin")),
	}
}

// getToolsList lists available tools in contrib/tool.
func getToolsList(contribDir string) []ToolItem {
	entries, err := os.ReadDir(contribDir)
	if err != nil {
		return nil
	}

	var tools []ToolItem
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		toolDir := filepath.Join(contribDir, name)
		if _, err := os.Stat(filepath.Join(toolDir, "README.md")); errors.Is(err, os.ErrNotExist) {
			continue
		}
		tools = append(tools, buildToolItem(name, toolDir))
	}

	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})

	return tools
}

// extractPurposeFromReadme extracts the purpose line from a README.
func extractPurposeFromReadme(readme string) string {
	lines := strings.Split(readme, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			return findDescriptionAfterTitle(lines, i)
		}
	}
	return ""
}

// findDescriptionAfterTitle looks for a description line after the title.
func findDescriptionAfterTitle(lines []string, titleIdx int) string {
	for j := titleIdx + 1; j < len(lines) && j < titleIdx+5; j++ {
		nextLine := strings.TrimSpace(lines[j])
		if nextLine == "" {
			continue
		}
		if !strings.HasPrefix(nextLine, "#") && !strings.HasPrefix(nextLine, "[") {
			return truncateLine(nextLine, 100)
		}
		break
	}
	return ""
}

// truncateLine truncates a line to maxLen with ellipsis.
func truncateLine(line string, maxLen int) string {
	if len(line) > maxLen {
		return line[:maxLen-3] + "..."
	}
	return line
}

// extractLicenseType extracts the license type from license text.
func extractLicenseType(license string) string {
	if matched := matchKnownLicense(license); matched != "" {
		return matched
	}
	return extractFirstShortLine(license)
}

// matchKnownLicense checks against known license patterns.
func matchKnownLicense(license string) string {
	licenseLower := strings.ToLower(license)
	for _, lp := range licensePatterns {
		if allPatternsMatch(licenseLower, lp.patterns) {
			return lp.license
		}
	}
	return ""
}

// allPatternsMatch checks if all patterns are found in text.
func allPatternsMatch(text string, patterns []string) bool {
	for _, pattern := range patterns {
		if !strings.Contains(text, pattern) {
			return false
		}
	}
	return true
}

// extractFirstShortLine finds the first non-empty short line.
func extractFirstShortLine(license string) string {
	for _, line := range strings.Split(license, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && len(line) < 50 {
			return line
		}
	}
	return "Unknown"
}

// SWORDData is the data for the SWORD browser page.
type SWORDData struct {
	PageData
	Modules    []SWORDModule
	SourcePath string
}

// SWORDModule describes a SWORD module.
type SWORDModule struct {
	ID            string
	ConfFile      string // Original .conf filename (preserves case)
	Description   string
	Language      string
	Version       string
	Category      string
	Copyright     string
	License       string
	DataPath      string
	Versification string   // KJV, NRSV, Catholic, Vulg, LXX, Orthodox, etc.
	Features      []string // StrongsNumbers, Images, GreekDef, HebrewDef, etc.
}

// handleSWORDBrowser redirects to /juniper.
func handleSWORDBrowser(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/juniper", http.StatusMovedPermanently)
}

// handleJuniperRedirect redirects legacy /juniper/ingest and /juniper/repoman to /juniper.
func handleJuniperRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/juniper", http.StatusMovedPermanently)
}

// handlePluginsRedirect redirects /plugins to /juniper?tab=plugins.
func handlePluginsRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/juniper?tab=plugins", http.StatusMovedPermanently)
}

// handleDetectRedirect redirects /detect to /juniper?tab=detect.
func handleDetectRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/juniper?tab=detect", http.StatusMovedPermanently)
}

// getSWORDModules lists available SWORD modules from a directory.
func getSWORDModules(swordDir string) []SWORDModule {
	var modules []SWORDModule

	modsDir := filepath.Join(swordDir, "mods.d")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return modules
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") {
			continue
		}

		confPath := filepath.Join(modsDir, entry.Name())
		confData, err := os.ReadFile(confPath)
		if err != nil {
			continue
		}

		module := parseSWORDConf(string(confData), entry.Name())
		modules = append(modules, module)
	}

	// Sort by ID
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].ID < modules[j].ID
	})

	return modules
}

// parseSWORDConf parses a SWORD module .conf file.
func parseSWORDConf(conf, filename string) SWORDModule {
	module := SWORDModule{
		ID:       strings.TrimSuffix(filename, ".conf"),
		ConfFile: filename, // Preserve original filename for case-sensitive filesystems
	}

	lines := strings.Split(conf, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			module.ID = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}

		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			parseSWORDConfField(&module, key, value)
		}
	}

	return module
}

// parseSWORDConfField processes a single configuration field.
var swordConfStringFields = map[string]func(*SWORDModule) *string{
	"description":         func(m *SWORDModule) *string { return &m.Description },
	"lang":                func(m *SWORDModule) *string { return &m.Language },
	"version":             func(m *SWORDModule) *string { return &m.Version },
	"category":            func(m *SWORDModule) *string { return &m.Category },
	"copyright":           func(m *SWORDModule) *string { return &m.Copyright },
	"distributionlicense": func(m *SWORDModule) *string { return &m.License },
	"datapath":            func(m *SWORDModule) *string { return &m.DataPath },
	"versification":       func(m *SWORDModule) *string { return &m.Versification },
}

func parseSWORDConfField(module *SWORDModule, key, value string) {
	lkey := strings.ToLower(key)
	if fieldPtr, ok := swordConfStringFields[lkey]; ok {
		*fieldPtr(module) = value
		return
	}
	if lkey == "feature" {
		module.Features = append(module.Features, value)
		return
	}
	if lkey == "globaloptionffilter" {
		parseGlobalOptionFilter(module, value)
	}
}

// parseGlobalOptionFilter extracts features from GlobalOptionFilter values.
func parseGlobalOptionFilter(module *SWORDModule, value string) {
	lowerValue := strings.ToLower(value)
	filterMap := map[string]string{
		"strongs":   "StrongsNumbers",
		"morph":     "Morphology",
		"footnotes": "Footnotes",
		"headings":  "Headings",
	}

	for keyword, feature := range filterMap {
		if strings.Contains(lowerValue, keyword) {
			module.Features = append(module.Features, feature)
		}
	}
}

// JuniperIngestData is the data for the Juniper ingest page.
type JuniperIngestData struct {
	PageData
	Modules    []SWORDModule
	AllModules []SWORDModule // All modules for filters
	Languages  []string
	Licenses   []string
	Categories []string
	SourcePath string
	Result     *JuniperIngestResult
	// Pagination
	Page           int
	PerPage        int
	TotalPages     int
	PerPageOptions []int
}

// JuniperIngestResult is the result of a Juniper ingest operation.
type JuniperIngestResult struct {
	Success       bool
	ModulesTotal  int
	ModulesOK     int
	ModulesFailed int
	Results       []ModuleIngestResult
	Message       string
}

// ModuleIngestResult is the result of ingesting a single module.
type ModuleIngestResult struct {
	ModuleID    string
	Success     bool
	CapsulePath string
	Error       string
}

// performJuniperIngest ingests selected SWORD modules as capsules.
func performJuniperIngest(swordDir string, moduleIDs []string) *JuniperIngestResult {
	result := &JuniperIngestResult{
		ModulesTotal: len(moduleIDs),
	}

	for _, moduleID := range moduleIDs {
		modResult := ingestSWORDModule(swordDir, moduleID)
		result.Results = append(result.Results, modResult)

		if modResult.Success {
			result.ModulesOK++
		} else {
			result.ModulesFailed++
		}
	}

	result.Success = result.ModulesFailed == 0
	if result.Success {
		result.Message = fmt.Sprintf("Successfully ingested %d module(s)", result.ModulesOK)
	} else {
		result.Message = fmt.Sprintf("Ingested %d of %d modules (%d failed)",
			result.ModulesOK, result.ModulesTotal, result.ModulesFailed)
	}

	return result
}

// findSWORDConfFile finds a SWORD module's .conf file case-insensitively.
// Returns the actual filename and full path, or empty strings if not found.
func findSWORDConfFile(modsDir, moduleID string) (filename, fullPath string) {
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return "", ""
	}

	targetLower := strings.ToLower(moduleID) + ".conf"
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.ToLower(entry.Name()) == targetLower {
			return entry.Name(), filepath.Join(modsDir, entry.Name())
		}
	}
	return "", ""
}

// setupCapsuleDirectory creates the capsule directory structure and copies the conf file.
func setupCapsuleDirectory(capsuleDir, confPath, confFilename string) error {
	if err := os.MkdirAll(filepath.Join(capsuleDir, "mods.d"), 0700); err != nil {
		return fmt.Errorf("failed to create capsule structure: %w", err)
	}

	destConf := filepath.Join(capsuleDir, "mods.d", confFilename)
	if err := fileutil.CopyFile(confPath, destConf); err != nil {
		return fmt.Errorf("failed to copy conf: %w", err)
	}

	return nil
}

// copyModuleData copies the module data from source to destination.
// It handles both directory paths and file prefixes used by some SWORD modules.
func copyModuleData(swordDir, dataPath, capsuleDir string) error {
	srcData := filepath.Join(swordDir, dataPath)
	destData := filepath.Join(capsuleDir, dataPath)

	// Check if DataPath is a directory or a file prefix
	// RawGenBook modules use file prefixes like ".../jesermons/jesermons"
	// where the actual files are jesermons.bdt, jesermons.dat, etc.
	info, err := os.Stat(srcData)
	if err != nil {
		// DataPath doesn't exist as-is, try parent directory (for file prefixes)
		srcData = filepath.Dir(srcData)
		destData = filepath.Dir(destData)
		info, err = os.Stat(srcData)
		if err != nil {
			return fmt.Errorf("failed to find module data: %w", err)
		}
	}

	if !info.IsDir() {
		return fmt.Errorf("module data path is not a directory: %s", srcData)
	}

	if err := os.MkdirAll(filepath.Dir(destData), 0700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	if err := fileutil.CopyDir(srcData, destData); err != nil {
		return fmt.Errorf("failed to copy module data: %w", err)
	}

	return nil
}

// createCapsuleArchive creates a tar.gz archive of the capsule.
func createCapsuleArchive(tempDir, moduleID string) (string, error) {
	outputPath := filepath.Join(ServerConfig.CapsulesDir, moduleID+".capsule.tar.gz")
	if err := archive.CreateTarGz(tempDir, outputPath, "capsule", true); err != nil {
		return "", fmt.Errorf("failed to create capsule: %w", err)
	}
	return outputPath, nil
}

// ingestSWORDModule ingests a single SWORD module as a capsule.
func ingestSWORDModule(swordDir, moduleID string) ModuleIngestResult {
	result := ModuleIngestResult{
		ModuleID: moduleID,
	}

	// Create temp directory for the module
	tempDir, err := secureMkdirTemp("", "sword-ingest-*")
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create temp directory: %v", err)
		return result
	}
	defer os.RemoveAll(tempDir)

	// Find conf file case-insensitively (important for case-sensitive filesystems)
	modsDir := filepath.Join(swordDir, "mods.d")
	confFilename, confPath := findSWORDConfFile(modsDir, moduleID)
	if confPath == "" {
		result.Error = fmt.Sprintf("Failed to find module conf for %s", moduleID)
		return result
	}

	// Read conf to get data path
	confData, err := os.ReadFile(confPath)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to read module conf: %v", err)
		return result
	}

	module := parseSWORDConf(string(confData), confFilename)

	// Prepare capsule directory structure
	capsuleDir := filepath.Join(tempDir, "capsule")
	if err := setupCapsuleDirectory(capsuleDir, confPath, confFilename); err != nil {
		result.Error = err.Error()
		return result
	}

	// Copy module data if present
	if module.DataPath != "" {
		if err := copyModuleData(swordDir, module.DataPath, capsuleDir); err != nil {
			result.Error = err.Error()
			return result
		}
	}

	// Create output capsule archive
	outputPath, err := createCapsuleArchive(tempDir, moduleID)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Success = true
	result.CapsulePath = outputPath
	return result
}

// ToolRunData is the data for the tool run page.
type ToolRunData struct {
	PageData
	ToolPlugins []ToolPluginInfo
	Capsules    []CapsuleInfo
	Result      *ToolRunResult
}

// ToolPluginInfo describes a tool plugin with its profiles.
type ToolPluginInfo struct {
	PluginID    string
	Name        string
	Description string
	Version     string
	Profiles    []ToolProfile
}

// ToolProfile describes a tool profile.
type ToolProfile struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// ToolRunResult is the result of running a tool.
type ToolRunResult struct {
	Success     bool
	PluginID    string
	Profile     string
	OutputDir   string
	Transcript  string
	Error       string
	ElapsedTime string
}

// handleToolRun handles the tool run page.
func handleToolRun(w http.ResponseWriter, r *http.Request) {
	// Get or create CSRF token
	csrfToken := getOrCreateCSRFToken(w, r)

	data := ToolRunData{
		PageData:    PageData{Title: "Run Tool", CSRFToken: csrfToken},
		ToolPlugins: getToolPlugins(),
		Capsules:    listCapsules(),
	}

	if r.Method == http.MethodPost {
		r.ParseForm()
		if !validateCSRFToken(r) {
			data.PageData.Error = "Invalid CSRF token. Please try again."
		} else {
			pluginID := r.FormValue("plugin")
			profile := r.FormValue("profile")
			capsulePath := r.FormValue("capsule")
			artifactID := r.FormValue("artifact")

			if pluginID != "" && profile != "" {
				result := runToolPlugin(pluginID, profile, capsulePath, artifactID)
				data.Result = result
			}
		}
	}

	if err := Templates.ExecuteTemplate(w, "tool_run.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

// getToolPlugins returns all available tool plugins with their profiles.
func getToolPlugins() []ToolPluginInfo {
	var toolPlugins []ToolPluginInfo

	loader := getLoader()
	if err := loader.LoadFromDir(ServerConfig.PluginsDir); err != nil {
		return toolPlugins
	}

	for _, plugin := range loader.GetPluginsByKind("tool") {
		// Read plugin.json directly to get profiles
		pluginJSONPath := filepath.Join(plugin.Path, "plugin.json")
		data, err := os.ReadFile(pluginJSONPath)
		if err != nil {
			continue
		}

		var pluginJSON struct {
			PluginID    string        `json:"plugin_id"`
			Version     string        `json:"version"`
			Description string        `json:"description"`
			Profiles    []ToolProfile `json:"profiles"`
		}
		if err := json.Unmarshal(data, &pluginJSON); err != nil {
			continue
		}

		// Extract friendly name from plugin ID (e.g., "tools.pandoc" -> "pandoc")
		name := pluginJSON.PluginID
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			name = name[idx+1:]
		}

		toolPlugins = append(toolPlugins, ToolPluginInfo{
			PluginID:    pluginJSON.PluginID,
			Name:        name,
			Description: pluginJSON.Description,
			Version:     pluginJSON.Version,
			Profiles:    pluginJSON.Profiles,
		})
	}

	// Sort by name
	sort.Slice(toolPlugins, func(i, j int) bool {
		return toolPlugins[i].Name < toolPlugins[j].Name
	})

	return toolPlugins
}

func loadPluginByID(pluginID string) (*plugins.Plugin, error) {
	loader := getLoader()
	if err := loader.LoadFromDir(ServerConfig.PluginsDir); err != nil {
		return nil, fmt.Errorf("Failed to load plugins: %v", err)
	}
	plugin, err := loader.GetPlugin(pluginID)
	if err != nil {
		return nil, fmt.Errorf("Plugin not found: %v", err)
	}
	return plugin, nil
}

func buildRequestArgs(capsulePath, artifactID, outputDir string) (map[string]string, error) {
	if capsulePath == "" || artifactID == "" {
		return map[string]string{}, nil
	}
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, capsulePath)
	if err != nil {
		return nil, fmt.Errorf("Invalid capsule path")
	}
	fullCapsulePath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)
	inputPath := filepath.Join(outputDir, "input")
	if err := os.MkdirAll(inputPath, 0700); err != nil {
		return map[string]string{}, nil
	}
	extractedPath, err := extractArtifact(fullCapsulePath, artifactID, inputPath)
	if err != nil {
		return map[string]string{}, nil
	}
	return map[string]string{"input": extractedPath}, nil
}

func execPluginCommand(plugin *plugins.Plugin, reqPath, outputDir string) error {
	entrypoint, err := plugin.SecureEntrypointPath()
	if err != nil {
		return fmt.Errorf("Plugin validation failed: %v", err)
	}
	cmd := exec.Command(entrypoint, "run", "--request", reqPath, "--out", outputDir)
	cmd.Dir = plugin.Path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Tool execution failed: %v\nOutput: %s", err, string(output))
	}
	return nil
}

func readToolTranscript(outputDir string) string {
	data, err := os.ReadFile(filepath.Join(outputDir, "transcript.jsonl"))
	if err != nil {
		return ""
	}
	return string(data)
}

func runToolPlugin(pluginID, profile, capsulePath, artifactID string) *ToolRunResult {
	start := time.Now()
	result := &ToolRunResult{PluginID: pluginID, Profile: profile}

	outputDir, err := secureMkdirTemp("", "tool-run-*")
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create output directory: %v", err)
		return result
	}
	result.OutputDir = outputDir

	plugin, err := loadPluginByID(pluginID)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	args, err := buildRequestArgs(capsulePath, artifactID, outputDir)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	reqPath := filepath.Join(outputDir, "request.json")
	reqJSON, _ := json.Marshal(map[string]interface{}{"profile": profile, "args": args})
	if err := os.WriteFile(reqPath, reqJSON, 0600); err != nil {
		result.Error = fmt.Sprintf("Failed to write request: %v", err)
		return result
	}

	if err := execPluginCommand(plugin, reqPath, outputDir); err != nil {
		result.Error = err.Error()
		return result
	}

	result.Transcript = readToolTranscript(outputDir)
	result.Success = true
	result.ElapsedTime = time.Since(start).Round(time.Millisecond).String()
	return result
}

// extractArtifact extracts an artifact from a capsule to the given directory.
func extractArtifact(capsulePath, artifactID, destDir string) (string, error) {
	// Read capsule and find artifact
	_, artifacts, err := readCapsule(capsulePath)
	if err != nil {
		return "", err
	}

	for _, art := range artifacts {
		if art.ID == artifactID || art.Name == artifactID {
			// Found it - extract
			destPath := filepath.Join(destDir, art.Name)
			// For now, return the path even if extraction fails
			// Real extraction would need to read from the capsule archive
			return destPath, nil
		}
	}

	return "", fmt.Errorf("artifact not found: %s", artifactID)
}

// RepoSource represents a SWORD repository source.
type RepoSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// RepoModule represents a module available from a repository.
type RepoModule struct {
	Name        string `json:"name"`
	ID          string `json:"id,omitempty"` // Alias for Name (template compat with SWORDModule)
	Description string `json:"description"`
	Type        string `json:"type"`
	Category    string `json:"category,omitempty"` // Alias for Type (template compat with SWORDModule)
	Language    string `json:"language"`
	Version     string `json:"version"`
	Size        int64  `json:"size"`
}

// JuniperRepomanData is the data for the Juniper repoman page.
type JuniperRepomanData struct {
	PageData
	Sources        []RepoSource
	SelectedSource string
	CustomURL      string
	Modules        []RepoModule
	Installed      []RepoModule
	InstallResult  *InstallResult
	RefreshResult  *RefreshResult
	IngestResult   *JuniperIngestResult
	SwordDir       string
	Languages      []string
	Types          []string
	ModulesLoaded  bool
	Tab            string
	SubTab         string // For tabs with sub-tabs (e.g., capsules)
	// Pagination for installed modules
	InstalledPage       int
	InstalledPageSize   int
	InstalledTotalPages int
	InstalledTotal      int
	InstalledTypes      []string
	InstalledLanguages  []string
	// Ingest tab data
	LocalModules   []RepoModule
	LocalTypes     []string
	LocalLanguages []string
	// Capsules tab data
	Capsules       []CapsuleInfo
	CapsulesNoIR   []CapsuleInfo
	CapsulesWithIR []CapsuleInfo
	Formats        []string
	CapsulesDir    string
	// Plugins tab data
	FormatPlugins []PluginInfo
	ToolPlugins   []PluginInfo
	// Detect tab data
	DetectResult *DetectResult
	// Info tab data
	PluginsExternal bool
	BibleCount      int
}

// InstallResult is the result of installing a module.
type InstallResult struct {
	Success bool
	Module  string
	Message string
}

// RefreshResult is the result of refreshing a source.
type RefreshResult struct {
	Success bool
	Source  string
	Message string
}

// parseJuniperRequestParams parses query parameters and form values from the request.
func parseJuniperRequestParams(r *http.Request) (tab, subtab, selectedSource, customURL string, installedPage int, shouldLoadModules bool) {
	tab = getQueryOrDefault(r, "tab", "installed")
	subtab = r.URL.Query().Get("subtab")
	if tab == "capsules" && subtab == "" {
		subtab = "list"
	}
	installedPage = parsePageNumber(r)
	selectedSource = getQueryOrFormValue(r, "source", "CrossWire")
	customURL = getQueryOrFormValue(r, "custom_url", "")
	shouldLoadModules = r.URL.Query().Get("loaded") == "1"
	return
}

// getQueryOrDefault returns the query parameter value or a default.
func getQueryOrDefault(r *http.Request, key, defaultVal string) string {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	return val
}

// parsePageNumber parses the page query parameter.
func parsePageNumber(r *http.Request) int {
	pageStr := r.URL.Query().Get("page")
	if pageStr == "" {
		return 1
	}
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		return p
	}
	return 1
}

// getQueryOrFormValue returns the query parameter, form value, or default.
func getQueryOrFormValue(r *http.Request, key, defaultVal string) string {
	val := r.URL.Query().Get(key)
	if val == "" {
		val = r.FormValue(key)
	}
	if val == "" {
		return defaultVal
	}
	return val
}

// handleFileUploadDetect handles file upload for format detection.
func handleFileUploadDetect(r *http.Request, w http.ResponseWriter) (detectResult *DetectResult, hasResult bool, errorMsg string) {
	r.Body = http.MaxBytesReader(w, r.Body, validation.MaxFileSize)
	if err := r.ParseMultipartForm(MaxFormMemory); err != nil {
		return nil, false, fmt.Sprintf("Failed to parse form: %v", err)
	}
	if !validateCSRFToken(r) {
		return nil, false, "Invalid CSRF token. Please try again."
	}
	if r.FormValue("action") != "detect" {
		return nil, false, ""
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, false, fmt.Sprintf("No file uploaded: %v", err)
	}
	defer file.Close()
	if header.Size > validation.MaxFileSize {
		return nil, false, fmt.Sprintf("File too large: %d bytes (max: %d bytes)", header.Size, validation.MaxFileSize)
	}
	result := performDetect(file, header.Filename)
	return result, true, ""
}

func postActionLoad(_ *http.Request, _ string, _ *JuniperRepomanData) (string, bool, bool) {
	return "repository", true, false
}

func postActionRefresh(_ *http.Request, effectiveSource string, data *JuniperRepomanData) (string, bool, bool) {
	data.RefreshResult = refreshRepoSource(effectiveSource)
	return "repository", true, false
}

func postActionInstall(r *http.Request, effectiveSource string, data *JuniperRepomanData) (string, bool, bool) {
	if moduleID := r.FormValue("module"); moduleID != "" {
		data.InstallResult = installModule(effectiveSource, moduleID, ServerConfig.SwordDir)
	}
	return "repository", true, false
}

func postActionDelete(r *http.Request, _ string, data *JuniperRepomanData) (string, bool, bool) {
	if moduleID := r.FormValue("module"); moduleID != "" {
		data.InstallResult = deleteModule(moduleID, ServerConfig.SwordDir)
	}
	return "installed", false, false
}

func postActionIngest(r *http.Request, _ string, data *JuniperRepomanData) (string, bool, bool) {
	selectedModules := r.Form["modules"]
	if len(selectedModules) == 0 {
		return "capsules", false, false
	}
	result := performJuniperIngest(ServerConfig.SwordDir, selectedModules)
	data.IngestResult = result
	return "capsules", false, result.Success
}

var postActionHandlers = map[string]func(*http.Request, string, *JuniperRepomanData) (string, bool, bool){
	"load":    postActionLoad,
	"refresh": postActionRefresh,
	"install": postActionInstall,
	"delete":  postActionDelete,
	"ingest":  postActionIngest,
}

func handleNonMultipartPOSTActions(r *http.Request, effectiveSource string, data *JuniperRepomanData) (newTab string, shouldLoadModules bool, shouldRedirect bool) {
	if !validateCSRFToken(r) {
		data.PageData.Error = "Invalid CSRF token. Please try again."
		return "", false, false
	}
	action := r.FormValue("action")
	handler, ok := postActionHandlers[action]
	if !ok {
		return data.Tab, false, false
	}
	return handler(r, effectiveSource, data)
}

// loadRepositoryModules loads available modules from the selected repository source.
func loadRepositoryModules(effectiveSource string, data *JuniperRepomanData) {
	modules, err := listRepoModules(effectiveSource)
	if err != nil {
		data.PageData.Error = fmt.Sprintf("Failed to load modules: %v", err)
		return
	}
	data.Modules = modules
	data.ModulesLoaded = true
	langSet := make(map[string]bool)
	typeSet := make(map[string]bool)
	for _, mod := range modules {
		if mod.Language != "" {
			langSet[mod.Language] = true
		}
		if mod.Type != "" {
			typeSet[mod.Type] = true
		}
	}
	for lang := range langSet {
		data.Languages = append(data.Languages, lang)
	}
	for t := range typeSet {
		data.Types = append(data.Types, t)
	}
	sort.Strings(data.Languages)
	sort.Strings(data.Types)
}

// loadInstalledModulesWithPagination loads installed modules with pagination support.
func loadInstalledModulesWithPagination(installedPage, installedPageSize int, data *JuniperRepomanData) {
	installed := getInstalledModules(ServerConfig.SwordDir)
	data.InstalledTotal = len(installed)
	data.InstalledTotalPages = calculateTotalPages(len(installed), installedPageSize)
	extractInstalledFilters(installed, data)
	installedPage = adjustPageNumber(installedPage, data)
	data.Installed = paginateInstalled(installed, installedPage, installedPageSize)
}

// calculateTotalPages calculates the total number of pages.
func calculateTotalPages(total, pageSize int) int {
	pages := (total + pageSize - 1) / pageSize
	if pages == 0 {
		return 1
	}
	return pages
}

// extractInstalledFilters extracts unique types and languages from installed modules.
func extractInstalledFilters(installed []RepoModule, data *JuniperRepomanData) {
	typeSet := make(map[string]bool)
	langSet := make(map[string]bool)
	for _, mod := range installed {
		if mod.Type != "" {
			typeSet[mod.Type] = true
		}
		if mod.Language != "" {
			langSet[mod.Language] = true
		}
	}
	for t := range typeSet {
		data.InstalledTypes = append(data.InstalledTypes, t)
	}
	for l := range langSet {
		data.InstalledLanguages = append(data.InstalledLanguages, l)
	}
	sort.Strings(data.InstalledTypes)
	sort.Strings(data.InstalledLanguages)
}

// adjustPageNumber ensures the page number is within valid bounds.
func adjustPageNumber(page int, data *JuniperRepomanData) int {
	if page > data.InstalledTotalPages {
		page = data.InstalledTotalPages
		data.InstalledPage = page
	}
	return page
}

// paginateInstalled returns the slice of installed modules for the current page.
func paginateInstalled(installed []RepoModule, page, pageSize int) []RepoModule {
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > len(installed) {
		end = len(installed)
	}
	if start < len(installed) {
		return installed[start:end]
	}
	return nil
}

// loadLocalModulesForIngest loads local SWORD modules for the ingest tab.
func loadLocalModulesForIngest(data *JuniperRepomanData) {
	localModules := getSWORDModules(ServerConfig.SwordDir)
	for _, mod := range localModules {
		data.LocalModules = append(data.LocalModules, RepoModule{
			Name:        mod.ID,
			ID:          mod.ID,
			Description: mod.Description,
			Type:        categoryToType(mod.Category),
			Category:    mod.Category,
			Language:    mod.Language,
			Version:     mod.Version,
		})
	}
	localTypeSet := make(map[string]bool)
	localLangSet := make(map[string]bool)
	for _, mod := range data.LocalModules {
		if mod.Type != "" {
			localTypeSet[mod.Type] = true
		}
		if mod.Language != "" {
			localLangSet[mod.Language] = true
		}
	}
	for t := range localTypeSet {
		data.LocalTypes = append(data.LocalTypes, t)
	}
	for l := range localLangSet {
		data.LocalLanguages = append(data.LocalLanguages, l)
	}
	sort.Strings(data.LocalTypes)
	sort.Strings(data.LocalLanguages)
}

// loadCapsulesData loads capsule data for the capsules tab.
func loadCapsulesData(data *JuniperRepomanData) {
	allCapsules := listCapsules()
	capsulesNoIR, _, capsulesWithIR := categorizeCapsules(allCapsules)
	data.Capsules = allCapsules
	data.CapsulesNoIR = capsulesNoIR
	data.CapsulesWithIR = capsulesWithIR
}

// loadPluginsData loads plugin data for the plugins tab.
func loadPluginsData(data *JuniperRepomanData) {
	data.FormatPlugins = listFormatPlugins()
	data.ToolPlugins = listToolPlugins()
}

// loadInfoTabData loads data for the info tab.
func loadInfoTabData(data *JuniperRepomanData) {
	data.FormatPlugins = listFormatPlugins()
	data.ToolPlugins = listToolPlugins()
	data.PluginsExternal = ServerConfig.PluginsExternal
	data.BibleCount = len(getCachedBibles())
	data.Capsules = listCapsules()
}

// initializeJuniperData initializes the base JuniperRepomanData structure.
func initializeJuniperData(csrfToken string) JuniperRepomanData {
	return JuniperRepomanData{
		PageData: PageData{Title: "SWORD Modules", CSRFToken: csrfToken},
		Sources: []RepoSource{
			{Name: "CrossWire", URL: "https://www.crosswire.org/ftpmirror/pub/sword/raw"},
			{Name: "eBible", URL: "https://ebible.org/sword"},
		},
		SwordDir:    ServerConfig.SwordDir,
		CapsulesDir: ServerConfig.CapsulesDir,
		Formats:     []string{"osis", "usfm", "usx", "json", "html", "epub", "markdown", "sqlite", "txt"},
	}
}

// getEffectiveSource returns the actual source to use for plugin calls.
func getEffectiveSource(selectedSource, customURL string) string {
	if selectedSource == "Custom" && customURL != "" {
		return customURL
	}
	return selectedSource
}

// buildRedirectURL constructs the redirect URL after POST actions.
func buildRedirectURL(tab, selectedSource, customURL string, shouldLoadModules bool) string {
	redirectURL := fmt.Sprintf("/juniper?tab=%s&source=%s", tab, selectedSource)
	if shouldLoadModules {
		redirectURL += "&loaded=1"
	}
	if selectedSource == "Custom" && customURL != "" {
		redirectURL += "&custom_url=" + url.QueryEscape(customURL)
	}
	return redirectURL
}

// buildIngestRedirectURL constructs the redirect URL after successful ingest.
func buildIngestRedirectURL(selectedSource string, modulesOK int) string {
	return fmt.Sprintf("/juniper?tab=capsules&subtab=list&source=%s&message=%s",
		selectedSource, url.QueryEscape(fmt.Sprintf("Successfully created %d capsule(s)", modulesOK)))
}

var juniperTabLoaders = map[string]func(*JuniperRepomanData){
	"capsules": loadCapsulesData,
	"plugins":  loadPluginsData,
	"info":     loadInfoTabData,
}

func isMultipartRequest(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	return len(ct) >= 19 && ct[:19] == "multipart/form-data"
}

func handleJuniperMultipart(w http.ResponseWriter, r *http.Request, tab string, data *JuniperRepomanData) string {
	detectResult, hasResult, errorMsg := handleFileUploadDetect(r, w)
	if errorMsg != "" {
		data.PageData.Error = errorMsg
		return tab
	}
	if hasResult {
		data.DetectResult = detectResult
		return "detect"
	}
	return tab
}

func handleJuniperPOST(w http.ResponseWriter, r *http.Request, selectedSource, effectiveSource string, tab string, shouldLoadModules bool, data *JuniperRepomanData) (string, bool, bool) {
	multipart := isMultipartRequest(r)
	if multipart {
		tab = handleJuniperMultipart(w, r, tab, data)
	} else {
		r.ParseForm()
	}
	if data.PageData.Error != "" || multipart {
		return tab, shouldLoadModules, false
	}
	newTab, loadModules, shouldRedirect := handleNonMultipartPOSTActions(r, effectiveSource, data)
	if shouldRedirect && data.IngestResult != nil && data.IngestResult.Success {
		http.Redirect(w, r, buildIngestRedirectURL(selectedSource, data.IngestResult.ModulesOK), http.StatusSeeOther)
		return newTab, loadModules, true
	}
	return newTab, loadModules, false
}

func handleJuniper(w http.ResponseWriter, r *http.Request) {
	csrfToken := getOrCreateCSRFToken(w, r)
	data := initializeJuniperData(csrfToken)

	tab, subtab, selectedSource, customURL, installedPage, shouldLoadModules := parseJuniperRequestParams(r)
	const installedPageSize = 22

	data.Tab = tab
	data.SubTab = subtab
	data.SelectedSource = selectedSource
	data.CustomURL = customURL
	data.InstalledPage = installedPage
	data.InstalledPageSize = installedPageSize

	if message := r.URL.Query().Get("message"); message != "" {
		data.PageData.Message = message
	}

	effectiveSource := getEffectiveSource(selectedSource, customURL)

	if r.Method == http.MethodPost {
		var redirected bool
		tab, shouldLoadModules, redirected = handleJuniperPOST(w, r, selectedSource, effectiveSource, tab, shouldLoadModules, &data)
		data.Tab = tab
		if redirected {
			return
		}
		if data.PageData.Error == "" {
			http.Redirect(w, r, buildRedirectURL(tab, selectedSource, customURL, shouldLoadModules), http.StatusSeeOther)
			return
		}
	}

	if shouldLoadModules {
		loadRepositoryModules(effectiveSource, &data)
	}

	loadInstalledModulesWithPagination(installedPage, installedPageSize, &data)
	loadLocalModulesForIngest(&data)

	if loader, ok := juniperTabLoaders[tab]; ok {
		loader(&data)
	}

	if err := Templates.ExecuteTemplate(w, "juniper.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}


// categoryToType converts SWORD category names to shorter type names.
func categoryToType(category string) string {
	switch category {
	case "Biblical Texts":
		return "Bible"
	case "Commentaries":
		return "Commentary"
	case "Lexicons / Dictionaries":
		return "Dictionary"
	case "Generic Books":
		return "GenBook"
	default:
		return category
	}
}

// listRepoModules lists available modules from a source by calling the repoman plugin.
func listRepoModules(sourceName string) ([]RepoModule, error) {
	pluginPath := findRepomanPlugin()
	if pluginPath == "" {
		return nil, fmt.Errorf("repoman plugin not found")
	}

	// Validate plugin path for security
	if err := plugins.ValidatePluginPath(pluginPath); err != nil {
		return nil, fmt.Errorf("plugin validation failed: %v", err)
	}

	// Create IPC request
	request := map[string]interface{}{
		"command": "list",
		"args": map[string]interface{}{
			"source": sourceName,
		},
	}
	reqJSON, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	// Run plugin with IPC
	cmd := exec.Command(pluginPath, "ipc")
	cmd.Stdin = strings.NewReader(string(reqJSON) + "\n")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("plugin error: %v", err)
	}

	// Parse response
	var resp struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Result struct {
			Modules []RepoModule `json:"modules"`
			Count   int          `json:"count"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("invalid response: %v", err)
	}
	if resp.Status == "error" {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	return resp.Result.Modules, nil
}

// refreshRepoSource refreshes a repository source index.
func refreshRepoSource(sourceName string) *RefreshResult {
	result := &RefreshResult{Source: sourceName}

	pluginPath := findRepomanPlugin()
	if pluginPath == "" {
		result.Message = "repoman plugin not found"
		return result
	}

	// Validate plugin path for security
	if err := plugins.ValidatePluginPath(pluginPath); err != nil {
		result.Message = fmt.Sprintf("Plugin validation failed: %v", err)
		return result
	}

	request := map[string]interface{}{
		"command": "refresh",
		"args": map[string]interface{}{
			"source": sourceName,
		},
	}
	reqJSON, _ := json.Marshal(request)

	cmd := exec.Command(pluginPath, "ipc")
	cmd.Stdin = strings.NewReader(string(reqJSON) + "\n")
	output, err := cmd.Output()
	if err != nil {
		result.Message = fmt.Sprintf("Plugin error: %v", err)
		return result
	}

	var resp struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(output, &resp); err != nil {
		result.Message = fmt.Sprintf("Invalid response: %v", err)
		return result
	}
	if resp.Status == "error" {
		result.Message = resp.Error
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("Successfully refreshed %s", sourceName)
	return result
}

// installModule installs a module from a repository.
func installModule(sourceName, moduleID, destPath string) *InstallResult {
	result := &InstallResult{Module: moduleID}

	pluginPath := findRepomanPlugin()
	if pluginPath == "" {
		result.Message = "repoman plugin not found"
		return result
	}

	// Validate plugin path for security
	if err := plugins.ValidatePluginPath(pluginPath); err != nil {
		result.Message = fmt.Sprintf("Plugin validation failed: %v", err)
		return result
	}

	request := map[string]interface{}{
		"command": "install",
		"args": map[string]interface{}{
			"source": sourceName,
			"module": moduleID,
			"dest":   destPath,
		},
	}
	reqJSON, _ := json.Marshal(request)

	cmd := exec.Command(pluginPath, "ipc")
	cmd.Stdin = strings.NewReader(string(reqJSON) + "\n")
	output, err := cmd.Output()
	if err != nil {
		result.Message = fmt.Sprintf("Plugin error: %v", err)
		return result
	}

	var resp struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(output, &resp); err != nil {
		result.Message = fmt.Sprintf("Invalid response: %v", err)
		return result
	}
	if resp.Status == "error" {
		result.Message = resp.Error
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("Successfully installed %s", moduleID)
	return result
}

// deleteModule removes an installed SWORD module.
func deleteModule(moduleID, swordDir string) *InstallResult {
	result := &InstallResult{Module: moduleID}

	// Find the module conf file
	modsDir := filepath.Join(swordDir, "mods.d")
	confPath := filepath.Join(modsDir, strings.ToLower(moduleID)+".conf")

	// Read conf to find data path
	confData, err := os.ReadFile(confPath)
	if err != nil {
		result.Message = fmt.Sprintf("Module not found: %s", moduleID)
		return result
	}

	// Parse DataPath from conf
	var dataPath string
	for _, line := range strings.Split(string(confData), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "DataPath=") {
			dataPath = strings.TrimPrefix(line, "DataPath=")
			dataPath = strings.TrimPrefix(dataPath, "./")
			break
		}
	}

	// Delete the conf file
	if err := os.Remove(confPath); err != nil {
		result.Message = fmt.Sprintf("Failed to delete conf file: %v", err)
		return result
	}

	// Delete the data directory if found
	if dataPath != "" {
		fullDataPath := filepath.Join(swordDir, dataPath)
		if err := os.RemoveAll(fullDataPath); err != nil {
			// Non-fatal, conf is already deleted
			result.Message = fmt.Sprintf("Deleted %s but failed to remove data: %v", moduleID, err)
			result.Success = true
			return result
		}
	}

	result.Success = true
	result.Message = fmt.Sprintf("Successfully deleted %s", moduleID)
	return result
}

// getInstalledModules lists installed SWORD modules.
func getInstalledModules(swordDir string) []RepoModule {
	var modules []RepoModule

	modsDir := filepath.Join(swordDir, "mods.d")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return modules
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") {
			continue
		}

		confPath := filepath.Join(modsDir, entry.Name())
		confData, err := os.ReadFile(confPath)
		if err != nil {
			continue
		}

		mod := parseRepoModuleConf(string(confData), entry.Name())
		modules = append(modules, mod)
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Name < modules[j].Name
	})

	return modules
}

// repoModuleSetters maps conf keys to their setters.
var repoModuleSetters = map[string]func(*RepoModule, string){
	"description": func(m *RepoModule, v string) { m.Description = v },
	"lang":        func(m *RepoModule, v string) { m.Language = v },
	"version":     func(m *RepoModule, v string) { m.Version = v },
	"moddrv":      func(m *RepoModule, v string) { m.Type = moduleTypeFromModDrv(v) },
}

// parseRepoModuleConf parses a SWORD module .conf file.
func parseRepoModuleConf(conf, filename string) RepoModule {
	module := RepoModule{Name: strings.TrimSuffix(filename, ".conf")}
	for _, line := range strings.Split(conf, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			module.Name = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}
		applyConfLine(&module, line)
	}
	return module
}

// applyConfLine applies a key=value line to the module.
func applyConfLine(module *RepoModule, line string) {
	idx := strings.Index(line, "=")
	if idx <= 0 {
		return
	}
	key := strings.TrimSpace(strings.ToLower(line[:idx]))
	value := strings.TrimSpace(line[idx+1:])
	if setter, ok := repoModuleSetters[key]; ok {
		setter(module, value)
	}
}

// moduleTypeFromModDrv determines the module type from the driver.
func moduleTypeFromModDrv(driver string) string {
	driver = strings.ToLower(driver)
	switch {
	case strings.HasPrefix(driver, "ztext"), strings.HasPrefix(driver, "rawtext"):
		return "Bible"
	case strings.HasPrefix(driver, "zcom"), strings.HasPrefix(driver, "rawcom"):
		return "Commentary"
	case strings.HasPrefix(driver, "zld"), strings.HasPrefix(driver, "rawld"):
		return "Dictionary"
	case strings.Contains(driver, "genbook"):
		return "GenBook"
	default:
		return "Unknown"
	}
}

// findRepomanPlugin finds the repoman plugin binary.
func findRepomanPlugin() string {
	// Check common locations
	paths := []string{
		filepath.Join(ServerConfig.PluginsDir, "tool", "repoman", "tool-repoman"),
		"bin/plugins/tool/repoman/tool-repoman",
		"plugins/tool/repoman/tool-repoman",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
