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
	return os.WriteFile(cacheFile, data, 0644)
}

// preloadCapsuleMetadata preloads metadata for all capsules in parallel.
// Uses a disk cache to avoid scanning archives on subsequent startups.
// This should be called during startup warmup.
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
	currentFiles := make(map[string]os.FileInfo)
	for _, c := range capsules {
		fullPath := filepath.Join(ServerConfig.CapsulesDir, c.Path)
		if info, err := os.Stat(fullPath); err == nil {
			currentFiles[c.Name] = info
		}
	}

	// Find capsules that need scanning (not in cache or modified)
	var needsScan []CapsuleInfo
	cacheHits := 0
	for _, c := range capsules {
		info := currentFiles[c.Name]
		if info == nil {
			continue
		}

		cached, ok := diskCache[c.Name]
		if ok && cached.ModTime == info.ModTime().Unix() && cached.Size == info.Size() {
			// Cache hit - use cached values
			fullPath := filepath.Join(ServerConfig.CapsulesDir, c.Path)
			capsuleMetadataCache.Lock()
			capsuleMetadataCache.data[fullPath] = CapsuleMetadata{
				IsCAS: cached.IsCAS,
				HasIR: cached.HasIR,
			}
			capsuleMetadataCache.Unlock()
			cacheHits++
		} else {
			// Cache miss - needs scanning
			needsScan = append(needsScan, c)
		}
	}

	log.Printf("[CACHE] Disk metadata cache: %d hits, %d need scanning", cacheHits, len(needsScan))

	if len(needsScan) == 0 {
		capsuleMetadataCache.Lock()
		capsuleMetadataCache.timestamp = time.Now()
		capsuleMetadataCache.Unlock()
		log.Printf("[CACHE] All %d capsules loaded from disk cache", cacheHits)
		return
	}

	// Scan capsules that need it
	type metaResult struct {
		name     string
		path     string
		meta     CapsuleMetadata
		diskMeta DiskCapsuleMetadata
	}

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
	for r := range pool.Results() {
		capsuleMetadataCache.Lock()
		capsuleMetadataCache.data[r.path] = r.meta
		capsuleMetadataCache.Unlock()
		diskCache[r.name] = r.diskMeta
	}

	capsuleMetadataCache.Lock()
	capsuleMetadataCache.timestamp = time.Now()
	capsuleMetadataCache.Unlock()

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
	// URL format: /{prefix}/capsule-path or /{prefix}/capsule-path/download
	capsulePath := strings.TrimPrefix(r.URL.Path, prefix)
	if capsulePath == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Check if this is a download request
	isDownload := strings.HasSuffix(capsulePath, "/download")
	if isDownload {
		capsulePath = strings.TrimSuffix(capsulePath, "/download")
	}

	// Sanitize path to prevent path traversal attacks
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, capsulePath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)

	// Handle download request
	if isDownload {
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
		return
	}

	// Get or create CSRF token
	csrfToken := getOrCreateCSRFToken(w, r)

	data := IRData{
		PageData: PageData{Title: "IR View: " + capsulePath, CSRFToken: csrfToken},
		Capsule: CapsuleInfo{
			Name: filepath.Base(capsulePath),
			Path: capsulePath,
		},
		URLPrefix: strings.TrimSuffix(prefix, "/"),
	}

	// Handle POST request for generating IR
	if r.Method == http.MethodPost {
		// Validate CSRF token
		if !validateCSRFToken(r) {
			data.PageData.Error = "Invalid CSRF token. Please try again."
		} else {
			action := r.FormValue("action")
			if action == "generate" {
				result := performIRGeneration(capsulePath)
				if result.Success {
					data.PageData.Message = result.Message
				} else {
					data.PageData.Error = result.Message
				}
			}
		}
	}

	// Check if capsule file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		data.PageData.Title = "IR View"
		data.PageData.Error = fmt.Sprintf("Capsule not found: %s", capsulePath)
		Templates.ExecuteTemplate(w, "ir.html", data)
		return
	}

	irContent, err := readIRContent(fullPath)
	if err != nil {
		if data.PageData.Error == "" {
			data.PageData.Error = "No IR found in capsule. Use 'Generate IR' to create one."
		}
		Templates.ExecuteTemplate(w, "ir.html", data)
		return
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

	if err := Templates.ExecuteTemplate(w, "ir.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
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

func handleConvert(w http.ResponseWriter, r *http.Request) {
	allCapsules := listCapsules()

	// Categorize capsules in parallel
	capsulesNoIR, capsulesCAS, _ := categorizeCapsules(allCapsules)

	// Get or create CSRF token
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
		// Validate CSRF token
		if !validateCSRFToken(r) {
			data.PageData.Error = "Invalid CSRF token. Please try again."
		} else {
			action := r.FormValue("action")

			switch action {
			case "convert":
				data.ActiveTab = "convert"
				source := r.FormValue("source")
				targetFormat := r.FormValue("format")

				if source != "" && targetFormat != "" {
					result := performConversion(source, targetFormat)
					data.Result = result
				}

			case "generate-ir":
				data.ActiveTab = "generate-ir"
				source := r.FormValue("source")

				if source != "" {
					result := performIRGeneration(source)
					data.Result = result
				}

			case "cas-to-sword":
				data.ActiveTab = "cas-to-sword"
				source := r.FormValue("source")

				if source != "" {
					result := performCASToSWORD(source)
					data.Result = result
				}
			}
		}
	}

	if err := Templates.ExecuteTemplate(w, "convert.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

// performConversion converts a capsule to a different format.
// It creates a new capsule with the converted content and renames the original.
func performConversion(sourcePath, targetFormat string) *ConvertResult {
	// Sanitize path to prevent path traversal attacks
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, sourcePath)
	if err != nil {
		return &ConvertResult{
			Success: false,
			Message: "Invalid path",
		}
	}
	fullPath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)

	// Check if capsule exists
	if _, err := os.Stat(fullPath); errors.Is(err, os.ErrNotExist) {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Capsule not found: %s", sourcePath),
		}
	}

	// Detect source format
	sourceFormat := detectSourceFormat(sourcePath)

	// Create temp directory for extraction
	tempDir, err := secureMkdirTemp("", "capsule-convert-*")
	if err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create temp directory: %v", err),
		}
	}
	defer os.RemoveAll(tempDir)

	// Extract capsule
	extractDir := filepath.Join(tempDir, "extract")
	if err := extractCapsule(fullPath, extractDir); err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to extract capsule: %v", err),
		}
	}

	// Load plugins
	loader := getLoader()
	if err := loader.LoadFromDir(ServerConfig.PluginsDir); err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to load plugins: %v", err),
		}
	}

	// Find source content file
	contentPath, detectedFormat := findContentFile(extractDir)
	if contentPath == "" {
		return &ConvertResult{
			Success: false,
			Message: "No convertible content found in capsule. Supported formats: OSIS, USFM, USX, JSON, SWORD.",
		}
	}

	if sourceFormat == "unknown" {
		sourceFormat = detectedFormat
	}

	// Step 1: Extract IR from source
	irDir := filepath.Join(tempDir, "ir")
	os.MkdirAll(irDir, 0755)

	sourcePlugin, err := loader.GetPlugin("format." + sourceFormat)
	if err != nil {
		return &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("No plugin found for source format '%s'. Install the format.%s plugin.", sourceFormat, sourceFormat),
			SourceFormat: sourceFormat,
		}
	}

	extractReq := plugins.NewExtractIRRequest(contentPath, irDir)
	extractResp, err := plugins.ExecutePlugin(sourcePlugin, extractReq)
	if err != nil {
		return &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("Failed to extract IR: %v", err),
			SourceFormat: sourceFormat,
		}
	}

	extractResult, err := plugins.ParseExtractIRResult(extractResp)
	if err != nil {
		return &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("Failed to parse extract result: %v", err),
			SourceFormat: sourceFormat,
		}
	}

	// Step 2: Emit native format
	targetPlugin, err := loader.GetPlugin("format." + targetFormat)
	if err != nil {
		return &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("No plugin found for target format '%s'. Install the format.%s plugin.", targetFormat, targetFormat),
			SourceFormat: sourceFormat,
		}
	}

	emitDir := filepath.Join(tempDir, "output")
	os.MkdirAll(emitDir, 0755)

	emitReq := plugins.NewEmitNativeRequest(extractResult.IRPath, emitDir)
	emitResp, err := plugins.ExecutePlugin(targetPlugin, emitReq)
	if err != nil {
		return &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("Failed to emit %s: %v", targetFormat, err),
			SourceFormat: sourceFormat,
			LossClass:    extractResult.LossClass,
		}
	}

	emitResult, err := plugins.ParseEmitNativeResult(emitResp)
	if err != nil {
		return &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("Failed to parse emit result: %v", err),
			SourceFormat: sourceFormat,
		}
	}

	// Step 3: Create new capsule with converted content
	newCapsuleDir := filepath.Join(tempDir, "new-capsule")
	os.MkdirAll(newCapsuleDir, 0755)

	// Copy converted output
	outputData, err := os.ReadFile(emitResult.OutputPath)
	if err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to read converted output: %v", err),
		}
	}

	outputName := filepath.Base(emitResult.OutputPath)
	if err := os.WriteFile(filepath.Join(newCapsuleDir, outputName), outputData, 0644); err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to write converted output: %v", err),
		}
	}

	// Copy IR to new capsule
	irData, err := os.ReadFile(extractResult.IRPath)
	if err == nil {
		irName := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath)) + ".ir.json"
		os.WriteFile(filepath.Join(newCapsuleDir, irName), irData, 0644)
	}

	// Create manifest for new capsule
	manifest := map[string]interface{}{
		"capsule_version": "1.0",
		"module_type":     "bible",
		"source_format":   targetFormat,
		"converted_from":  sourceFormat,
		"conversion_date": time.Now().Format(time.RFC3339),
		"has_ir":          true,
		"extraction_loss": extractResult.LossClass,
		"emission_loss":   emitResult.LossClass,
	}

	// Try to preserve original manifest metadata
	origManifest := readCapsuleManifest(fullPath)
	if origManifest != nil {
		if origManifest.Title != "" {
			manifest["title"] = origManifest.Title
		}
		if origManifest.Language != "" {
			manifest["language"] = origManifest.Language
		}
	}

	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(newCapsuleDir, "manifest.json"), manifestData, 0644)

	// Step 4: Rename original and create new capsule
	oldPath := renameToOld(fullPath)
	if oldPath == "" {
		return &ConvertResult{
			Success: false,
			Message: "Failed to rename original capsule",
		}
	}

	if err := archive.CreateCapsuleTarGzFromPath(newCapsuleDir, fullPath); err != nil {
		// Try to restore original
		os.Rename(oldPath, fullPath)
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create new capsule: %v", err),
		}
	}

	// Determine combined loss class
	lossClass := combineLossClass(extractResult.LossClass, emitResult.LossClass)

	return &ConvertResult{
		Success:      true,
		OutputPath:   sourcePath,
		OldPath:      filepath.Base(oldPath),
		LossClass:    lossClass,
		Message:      fmt.Sprintf("Successfully converted from %s to %s", sourceFormat, targetFormat),
		SourceFormat: sourceFormat,
	}
}

// performIRGeneration generates IR for a capsule that doesn't have one.
func performIRGeneration(sourcePath string) *ConvertResult {
	// Sanitize path to prevent path traversal attacks
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, sourcePath)
	if err != nil {
		return &ConvertResult{
			Success: false,
			Message: "Invalid path",
		}
	}
	fullPath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)

	// Check if capsule exists
	if _, err := os.Stat(fullPath); errors.Is(err, os.ErrNotExist) {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Capsule not found: %s", sourcePath),
		}
	}

	// Check if it already has IR
	if archive.HasIR(fullPath) {
		return &ConvertResult{
			Success: false,
			Message: "This capsule already contains IR. No generation needed.",
		}
	}

	// Create temp directory
	tempDir, err := secureMkdirTemp("", "capsule-ir-gen-*")
	if err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create temp directory: %v", err),
		}
	}
	defer os.RemoveAll(tempDir)

	// Extract capsule
	extractDir := filepath.Join(tempDir, "extract")
	if err := extractCapsule(fullPath, extractDir); err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to extract capsule: %v", err),
		}
	}

	// Find source content file
	contentPath, detectedFormat := findContentFile(extractDir)
	if contentPath == "" {
		// Check if this is a CAS capsule (has blobs/ directory)
		blobsDir := filepath.Join(extractDir, "blobs")
		if _, err := os.Stat(blobsDir); err == nil {
			return &ConvertResult{
				Success: false,
				Message: "This capsule uses Content-Addressed Storage (CAS) format with blobs.\n\n" +
					"CAS capsules require extraction via the CLI:\n" +
					"  capsule export <capsule> --artifact main --out extracted/\n\n" +
					"Then use 'capsule juniper ingest' to create a SWORD capsule that can be converted.",
			}
		}

		// List what was found for debugging
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

	// Load plugins
	loader := getLoader()
	if err := loader.LoadFromDir(ServerConfig.PluginsDir); err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to load plugins: %v", err),
		}
	}

	// Extract IR
	irDir := filepath.Join(tempDir, "ir")
	os.MkdirAll(irDir, 0755)

	sourcePlugin, err := loader.GetPlugin("format." + detectedFormat)
	if err != nil {
		return &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("No plugin found for format '%s'. Install the format.%s plugin.", detectedFormat, detectedFormat),
			SourceFormat: detectedFormat,
		}
	}

	extractReq := plugins.NewExtractIRRequest(contentPath, irDir)
	extractResp, err := plugins.ExecutePlugin(sourcePlugin, extractReq)
	if err != nil {
		return &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("Failed to extract IR: %v", err),
			SourceFormat: detectedFormat,
		}
	}

	extractResult, err := parseExtractIRResultFlexible(extractResp)
	if err != nil {
		return &ConvertResult{
			Success:      false,
			Message:      fmt.Sprintf("Failed to parse extract result: %v", err),
			SourceFormat: detectedFormat,
		}
	}

	// Create new capsule with IR added
	newCapsuleDir := filepath.Join(tempDir, "new-capsule")

	// Copy all original files
	if err := fileutil.CopyDir(extractDir, newCapsuleDir); err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to copy capsule contents: %v", err),
		}
	}

	// Add IR file - handle case where IRPath is a directory
	irPath := extractResult.IRPath
	info, err := os.Stat(irPath)
	if err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to stat IR path: %v", err),
		}
	}
	if info.IsDir() {
		// Find first .ir.json file in directory
		entries, err := os.ReadDir(irPath)
		if err != nil {
			return &ConvertResult{
				Success: false,
				Message: fmt.Sprintf("Failed to read IR directory: %v", err),
			}
		}
		found := false
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".ir.json") {
				irPath = filepath.Join(irPath, entry.Name())
				found = true
				break
			}
		}
		if !found {
			return &ConvertResult{
				Success: false,
				Message: "No .ir.json file found in IR directory",
			}
		}
	}
	irData, err := os.ReadFile(irPath)
	if err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to read IR: %v", err),
		}
	}

	irName := trimArchiveSuffix(filepath.Base(sourcePath)) + ".ir.json"

	if err := os.WriteFile(filepath.Join(newCapsuleDir, irName), irData, 0644); err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to write IR file: %v", err),
		}
	}

	// Update manifest to indicate IR presence
	manifestPath := filepath.Join(newCapsuleDir, "manifest.json")
	manifest := make(map[string]interface{})

	if data, err := os.ReadFile(manifestPath); err == nil {
		json.Unmarshal(data, &manifest)
	}

	manifest["has_ir"] = true
	manifest["ir_generated"] = time.Now().Format(time.RFC3339)
	manifest["ir_loss_class"] = extractResult.LossClass
	manifest["source_format"] = detectedFormat

	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(manifestPath, manifestData, 0644)

	// Rename original and create new capsule
	oldPath := renameToOld(fullPath)
	if oldPath == "" {
		return &ConvertResult{
			Success: false,
			Message: "Failed to rename original capsule",
		}
	}

	if err := archive.CreateCapsuleTarGzFromPath(newCapsuleDir, fullPath); err != nil {
		// Try to restore original
		os.Rename(oldPath, fullPath)
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create new capsule: %v", err),
		}
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

// performCASToSWORD converts a CAS capsule to SWORD format.
func performCASToSWORD(sourcePath string) *ConvertResult {
	// Sanitize path to prevent path traversal attacks
	cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, sourcePath)
	if err != nil {
		return &ConvertResult{
			Success: false,
			Message: "Invalid path",
		}
	}
	fullPath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)

	// Check if capsule exists
	if _, err := os.Stat(fullPath); errors.Is(err, os.ErrNotExist) {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Capsule not found: %s", sourcePath),
		}
	}

	// Check if it's actually a CAS capsule
	if !archive.IsCASCapsule(fullPath) {
		return &ConvertResult{
			Success: false,
			Message: "This is not a CAS capsule. CAS capsules have a blobs/ directory.",
		}
	}

	// Create temp directory
	tempDir, err := secureMkdirTemp("", "capsule-cas-convert-*")
	if err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create temp directory: %v", err),
		}
	}
	defer os.RemoveAll(tempDir)

	// Extract CAS capsule
	extractDir := filepath.Join(tempDir, "extract")
	if err := extractCapsule(fullPath, extractDir); err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to extract capsule: %v", err),
		}
	}

	// Read manifest to find artifacts
	manifestPath := filepath.Join(extractDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to read manifest: %v", err),
		}
	}

	var manifest CASManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to parse manifest: %v", err),
		}
	}

	// Find the main artifact
	var mainArtifact *CASArtifact
	for i := range manifest.Artifacts {
		if manifest.Artifacts[i].ID == "main" || manifest.Artifacts[i].ID == manifest.MainArtifact {
			mainArtifact = &manifest.Artifacts[i]
			break
		}
	}
	if mainArtifact == nil && len(manifest.Artifacts) > 0 {
		mainArtifact = &manifest.Artifacts[0]
	}
	if mainArtifact == nil {
		return &ConvertResult{
			Success: false,
			Message: "No artifacts found in CAS capsule",
		}
	}

	// Create new SWORD capsule directory
	swordDir := filepath.Join(tempDir, "sword-capsule")
	os.MkdirAll(swordDir, 0755)

	// Extract artifact content from blobs
	for _, file := range mainArtifact.Files {
		// Find the blob
		blobPath := ""
		if file.Blake3 != "" {
			blobPath = filepath.Join(extractDir, "blobs", "blake3", file.Blake3[:2], file.Blake3)
		} else if file.SHA256 != "" {
			blobPath = filepath.Join(extractDir, "blobs", "sha256", file.SHA256[:2], file.SHA256)
		}

		if blobPath == "" {
			continue
		}

		// Read blob content
		content, err := os.ReadFile(blobPath)
		if err != nil {
			continue
		}

		// Write to SWORD capsule
		destPath := filepath.Join(swordDir, file.Path)
		os.MkdirAll(filepath.Dir(destPath), 0755)
		os.WriteFile(destPath, content, 0644)
	}

	// If no files were extracted, try to extract all blobs as-is
	if _, err := os.ReadDir(swordDir); err != nil || isDirEmpty(swordDir) {
		// Copy all blobs with their hash as filename (fallback)
		blobsDir := filepath.Join(extractDir, "blobs")
		filepath.Walk(blobsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			content, _ := os.ReadFile(path)
			if len(content) > 0 {
				// Try to determine file type from content
				destName := filepath.Base(path)
				if isJSONContent(content) {
					destName += ".json"
				}
				os.WriteFile(filepath.Join(swordDir, destName), content, 0644)
			}
			return nil
		})
	}

	// Create manifest for SWORD capsule
	swordManifest := map[string]interface{}{
		"capsule_version": "1.0",
		"module_type":     manifest.ModuleType,
		"id":              manifest.ID,
		"title":           manifest.Title,
		"source_format":   "cas-converted",
		"original_format": manifest.SourceFormat,
		"converted_from":  "cas",
	}
	swordManifestData, _ := json.MarshalIndent(swordManifest, "", "  ")
	os.WriteFile(filepath.Join(swordDir, "manifest.json"), swordManifestData, 0644)

	// Check if we have SWORD module structure
	hasSwordData := false
	if _, err := os.Stat(filepath.Join(swordDir, "mods.d")); err == nil {
		hasSwordData = true
	}

	// Rename original and create new capsule
	oldPath := renameToOld(fullPath)
	if oldPath == "" {
		return &ConvertResult{
			Success: false,
			Message: "Failed to rename original capsule",
		}
	}

	// Create new .tar.gz capsule
	newPath := strings.TrimSuffix(fullPath, ".tar.xz") + ".tar.gz"
	if strings.HasSuffix(fullPath, ".tar.gz") {
		newPath = fullPath
	}

	if err := archive.CreateCapsuleTarGzFromPath(swordDir, newPath); err != nil {
		os.Rename(oldPath, fullPath) // Restore on failure
		return &ConvertResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create SWORD capsule: %v", err),
		}
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
	// Skip whitespace
	for i := 0; i < len(content); i++ {
		if content[i] == ' ' || content[i] == '\t' || content[i] == '\n' || content[i] == '\r' {
			continue
		}
		return content[i] == '{' || content[i] == '['
	}
	return false
}

// extractCapsule extracts a capsule archive to a directory.
func extractCapsule(capsulePath, destDir string) error {
	f, err := os.Open(capsulePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var tr *tar.Reader

	if strings.HasSuffix(capsulePath, ".tar.xz") {
		xzr, err := xz.NewReader(f)
		if err != nil {
			return err
		}
		tr = tar.NewReader(xzr)
	} else if strings.HasSuffix(capsulePath, ".tar.gz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gzr.Close()
		tr = tar.NewReader(gzr)
	} else {
		return fmt.Errorf("unsupported archive format")
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Clean the path and remove any leading directory
		name := header.Name
		if idx := strings.Index(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		if name == "" {
			continue
		}

		destPath := filepath.Join(destDir, name)

		if header.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0755)
			continue
		}

		os.MkdirAll(filepath.Dir(destPath), 0755)

		outFile, err := os.Create(destPath)
		if err != nil {
			return err
		}

		if _, err := io.Copy(outFile, tr); err != nil {
			outFile.Close()
			return err
		}
		// Check Close() error for writes to detect disk full, etc.
		if err := outFile.Close(); err != nil {
			return err
		}
	}

	return nil
}

// parseExtractIRResultFlexible parses extract-ir results from both single and multi-module formats.
// Some plugins (like sword-pure) return {"modules": [...]} while others return {"ir_path": "..."}.
func parseExtractIRResultFlexible(resp *plugins.IPCResponse) (*plugins.ExtractIRResult, error) {
	if resp.Status == "error" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}

	// Re-marshal result to JSON
	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal result: %w", err)
	}

	// First try standard format: {"ir_path": "...", "loss_class": "..."}
	var standard plugins.ExtractIRResult
	if err := json.Unmarshal(data, &standard); err == nil && standard.IRPath != "" {
		return &standard, nil
	}

	// Try multi-module format: {"modules": [{"ir_path": "...", ...}], "count": N}
	var multiModule struct {
		Modules []struct {
			Module    string `json:"module"`
			Status    string `json:"status"`
			IRPath    string `json:"ir_path"`
			LossClass string `json:"loss_class"`
			Error     string `json:"error"`
			Reason    string `json:"reason"`
		} `json:"modules"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(data, &multiModule); err == nil && len(multiModule.Modules) > 0 {
		// Find first successful module with IR path
		for _, m := range multiModule.Modules {
			if m.Status == "ok" && m.IRPath != "" {
				return &plugins.ExtractIRResult{
					IRPath:    m.IRPath,
					LossClass: m.LossClass,
				}, nil
			}
		}
		// If no successful modules, report the first error
		for _, m := range multiModule.Modules {
			if m.Status == "error" {
				return nil, fmt.Errorf("module %s: %s", m.Module, m.Error)
			}
			if m.Status == "skipped" {
				return nil, fmt.Errorf("module %s skipped: %s", m.Module, m.Reason)
			}
		}
		return nil, fmt.Errorf("no IR generated from any module")
	}

	return nil, fmt.Errorf("unable to parse extract-ir result")
}

// findContentFile finds a convertible content file in the extracted capsule.
func findContentFile(extractDir string) (string, string) {
	// Check for SWORD module FIRST (has mods.d/*.conf)
	// Try both direct and nested capsule/ paths
	for _, base := range []string{extractDir, filepath.Join(extractDir, "capsule")} {
		modsDir := filepath.Join(base, "mods.d")
		if _, err := os.Stat(modsDir); err == nil {
			entries, _ := os.ReadDir(modsDir)
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".conf") {
					// Return the base directory (parent of mods.d), not just the .conf file
					return base, "sword-pure"
				}
			}
		}
	}

	// Priority order for other formats: OSIS, USX, USFM
	patterns := []struct {
		ext    string
		format string
	}{
		{".osis", "osis"},
		{".osis.xml", "osis"},
		{".usx", "usx"},
		{".usfm", "usfm"},
		{".sfm", "usfm"},
	}

	var found string
	var format string

	filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		name := strings.ToLower(filepath.Base(path))

		// Skip manifest and IR files
		if name == "manifest.json" || strings.HasSuffix(name, ".ir.json") {
			return nil
		}

		for _, p := range patterns {
			if strings.HasSuffix(name, p.ext) {
				found = path
				format = p.format
				return filepath.SkipAll
			}
		}

		return nil
	})

	// Legacy fallback for SWORD detection (shouldn't reach here)
	if found == "" {
		for _, base := range []string{extractDir, filepath.Join(extractDir, "capsule")} {
			modsDir := filepath.Join(base, "mods.d")
			if _, err := os.Stat(modsDir); err == nil {
				entries, _ := os.ReadDir(modsDir)
				for _, e := range entries {
					if strings.HasSuffix(e.Name(), ".conf") {
						found = base
						format = "sword-pure"
						break
					}
				}
			}
			if found != "" {
				break
			}
		}
	}

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

// readCapsuleManifest reads the manifest from a capsule archive.
func readCapsuleManifest(capsulePath string) *CapsuleManifest {
	f, err := os.Open(capsulePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var tr *tar.Reader

	if strings.HasSuffix(capsulePath, ".tar.xz") {
		xzr, err := xz.NewReader(f)
		if err != nil {
			return nil
		}
		tr = tar.NewReader(xzr)
	} else if strings.HasSuffix(capsulePath, ".tar.gz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return nil
		}
		defer gzr.Close()
		tr = tar.NewReader(gzr)
	} else {
		return nil
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}

		if strings.HasSuffix(header.Name, "manifest.json") {
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

	return nil
}

func handleStatic(w http.ResponseWriter, r *http.Request) {
	// Serve static files from cache (populated at startup)
	path := strings.TrimPrefix(r.URL.Path, "/static/")

	// Determine content type
	var contentType string
	switch path {
	case "base.css", "style.css":
		contentType = "text/css"
	case "app.js":
		contentType = "application/javascript"
	default:
		http.NotFound(w, r)
		return
	}

	// Try to get from cache first
	content, etag, ok := getStaticFile(path)
	if !ok {
		// Fallback to direct read if cache not populated
		var err error
		content, err = staticFS.ReadFile("static/" + path)
		if err != nil {
			http.Error(w, path+" not found", http.StatusNotFound)
			return
		}
	}

	// Check If-None-Match for conditional request (304 Not Modified)
	if etag != "" {
		if match := r.Header.Get("If-None-Match"); match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

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
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)
		if ext == ".xz" || ext == ".gz" || ext == ".tar" {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			fullPath := filepath.Join(ServerConfig.CapsulesDir, name)
			capsules = append(capsules, CapsuleInfo{
				Name:      name,
				Path:      name, // Flat directory, path == name
				Size:      info.Size(),
				SizeHuman: humanSize(info.Size()),
				Format:    detectCapsuleFormat(fullPath),
			})
		}
	}

	sort.Slice(capsules, func(i, j int) bool {
		return capsules[i].Name < capsules[j].Name
	})

	return capsules
}

func listFormatPlugins() []PluginInfo {
	var pluginInfos []PluginInfo

	// Use plugin loader to find all format plugins
	loader := getLoader()

	for _, p := range loader.ListPlugins() {
		if strings.HasPrefix(p.Manifest.PluginID, "format.") {
			source := "unloaded"
			hasBinary := false
			hasExternalBinary := false
			capabilities := ""

			// Check if this is an external plugin (has real path, not "(embedded)")
			if p.Path != "(embedded)" && p.Manifest.Entrypoint != "" {
				binPath := filepath.Join(p.Path, p.Manifest.Entrypoint)
				if _, err := os.Stat(binPath); err == nil {
					hasBinary = true
					hasExternalBinary = true
					source = "external"
				}
			}

			// Check if embedded plugin exists
			if !hasBinary && plugins.HasEmbeddedPlugin(p.Manifest.PluginID) {
				hasBinary = true
				source = "internal"
			}

			// Determine capability status for internal plugins
			if source == "internal" {
				if hasExternalBinary {
					capabilities = "IR: external fallback"
				} else {
					// Check if this plugin has stub IR implementations
					capabilities = getEmbeddedPluginCapabilities(p.Manifest.PluginID, hasExternalBinary)
				}
			}

			name := strings.TrimPrefix(p.Manifest.PluginID, "format.")

			pluginInfos = append(pluginInfos, PluginInfo{
				PluginID:     p.Manifest.PluginID,
				Name:         name,
				Version:      p.Manifest.Version,
				Type:         "format",
				Description:  fmt.Sprintf("Format plugin for %s", name),
				HasBinary:    hasBinary,
				Source:       source,
				Capabilities: capabilities,
				License:      getPluginLicense(p),
			})
		}
	}

	return pluginInfos
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
func parseLicenseText(content string) string {
	// Check for common license patterns
	contentLower := strings.ToLower(content)

	// Check for SPDX identifier in first few lines
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if i > 5 {
			break
		}
		if strings.Contains(line, "SPDX-License-Identifier:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	// Pattern matching for common licenses
	switch {
	case strings.Contains(contentLower, "mit license") || strings.Contains(contentLower, "permission is hereby granted, free of charge"):
		return "MIT"
	case strings.Contains(contentLower, "apache license") && strings.Contains(contentLower, "version 2.0"):
		return "Apache-2.0"
	case strings.Contains(contentLower, "gnu general public license") && strings.Contains(contentLower, "version 3"):
		return "GPL-3.0"
	case strings.Contains(contentLower, "gpl-3") || strings.Contains(contentLower, "gplv3"):
		return "GPL-3.0"
	case strings.Contains(contentLower, "gnu general public license") && strings.Contains(contentLower, "version 2"):
		return "GPL-2.0"
	case strings.Contains(contentLower, "gpl-2") || strings.Contains(contentLower, "gplv2"):
		return "GPL-2.0"
	case strings.Contains(contentLower, "gnu lesser general public license"):
		return "LGPL"
	case strings.Contains(contentLower, "bsd 3-clause") || strings.Contains(contentLower, "redistribution and use in source and binary forms"):
		return "BSD-3-Clause"
	case strings.Contains(contentLower, "bsd 2-clause"):
		return "BSD-2-Clause"
	case strings.Contains(contentLower, "mozilla public license") && strings.Contains(contentLower, "2.0"):
		return "MPL-2.0"
	case strings.Contains(contentLower, "unlicense") || strings.Contains(contentLower, "this is free and unencumbered software"):
		return "Unlicense"
	case strings.Contains(contentLower, "public domain"):
		return "Public Domain"
	case strings.Contains(contentLower, "isc license"):
		return "ISC"
	}

	return "See LICENSE file"
}

func listToolPlugins() []PluginInfo {
	var pluginInfos []PluginInfo

	// Use plugin loader to find all tool plugins (tool.* and tools.* prefixes)
	loader := getLoader()

	for _, p := range loader.ListPlugins() {
		if strings.HasPrefix(p.Manifest.PluginID, "tool.") || strings.HasPrefix(p.Manifest.PluginID, "tools.") {
			source := "unloaded"
			hasBinary := false

			// Check if this is an external plugin (has real path, not "(embedded)")
			if p.Path != "(embedded)" && p.Manifest.Entrypoint != "" {
				binPath := filepath.Join(p.Path, p.Manifest.Entrypoint)
				if _, err := os.Stat(binPath); err == nil {
					hasBinary = true
					source = "external"
				}
			}

			// Check if embedded plugin exists
			if !hasBinary && plugins.HasEmbeddedPlugin(p.Manifest.PluginID) {
				hasBinary = true
				source = "internal"
			}

			name := p.Manifest.PluginID
			name = strings.TrimPrefix(name, "tool.")
			name = strings.TrimPrefix(name, "tools.")

			pluginInfos = append(pluginInfos, PluginInfo{
				PluginID:    p.Manifest.PluginID,
				Name:        name,
				Version:     p.Manifest.Version,
				Type:        "tool",
				Description: fmt.Sprintf("Tool plugin for %s", name),
				HasBinary:   hasBinary,
				Source:      source,
				License:     getPluginLicense(p),
			})
		}
	}

	return pluginInfos
}

// detectCapsuleFormat is implemented in format_detection.go

func readCapsule(path string) (*CapsuleManifest, []ArtifactInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var reader io.Reader = f

	// Handle compression
	if strings.HasSuffix(path, ".xz") {
		xzReader, err := xz.NewReader(reader)
		if err != nil {
			return nil, nil, fmt.Errorf("xz decompress: %w", err)
		}
		reader = xzReader
	} else if strings.HasSuffix(path, ".gz") {
		gzReader, err := gzip.NewReader(reader)
		if err != nil {
			return nil, nil, fmt.Errorf("gzip decompress: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	tarReader := tar.NewReader(reader)
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
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, nil, err
			}
			manifest = &CapsuleManifest{}
			if err := json.Unmarshal(data, manifest); err != nil {
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

func readArtifactContent(capsulePath, artifactID string) (string, string, error) {
	f, err := os.Open(capsulePath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	var reader io.Reader = f

	if strings.HasSuffix(capsulePath, ".xz") {
		xzReader, err := xz.NewReader(reader)
		if err != nil {
			return "", "", err
		}
		reader = xzReader
	} else if strings.HasSuffix(capsulePath, ".gz") {
		gzReader, err := gzip.NewReader(reader)
		if err != nil {
			return "", "", err
		}
		defer gzReader.Close()
		reader = gzReader
	}

	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", err
		}

		if header.Name == artifactID {
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return "", "", err
			}
			contentType := detectContentType(header.Name, data)
			return string(data), contentType, nil
		}
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

func detectContentType(name string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".xml", ".osis", ".usx":
		return "application/xml"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html"
	case ".md":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".usfm", ".sfm":
		return "text/plain"
	default:
		if len(data) > 0 {
			for _, b := range data[:min(512, len(data))] {
				if b < 32 && b != '\t' && b != '\n' && b != '\r' {
					return "application/octet-stream"
				}
			}
		}
		return "text/plain"
	}
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

// performIngest creates a capsule from an uploaded file.
func performIngest(file io.Reader, filename string, size int64) *IngestResult {
	// Validate and sanitize filename
	safeFilename, err := validation.SanitizeFilename(filename)
	if err != nil {
		return &IngestResult{
			Success: false,
			Error:   fmt.Sprintf("Invalid filename: %v", err),
		}
	}

	// Create temp directory for processing
	tempDir, err := secureMkdirTemp("", "capsule-ingest-*")
	if err != nil {
		return &IngestResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to create temp directory: %v", err),
		}
	}
	defer os.RemoveAll(tempDir)

	// Save uploaded file with sanitized name
	uploadPath := filepath.Join(tempDir, safeFilename)
	outFile, err := os.Create(uploadPath)
	if err != nil {
		return &IngestResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to save file: %v", err),
		}
	}

	written, err := io.Copy(outFile, file)
	if err != nil {
		outFile.Close()
		return &IngestResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to write file: %v", err),
		}
	}
	// Check Close() error for writes to detect disk full, etc.
	if err := outFile.Close(); err != nil {
		return &IngestResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to close file: %v", err),
		}
	}

	// Detect format
	detectedFormat := detectFileFormat(uploadPath)

	// Create capsule directory
	capsuleDir := filepath.Join(tempDir, "capsule")
	os.MkdirAll(capsuleDir, 0755)

	// Copy file to capsule
	data, _ := os.ReadFile(uploadPath)
	os.WriteFile(filepath.Join(capsuleDir, filename), data, 0644)

	// Create manifest
	manifest := map[string]interface{}{
		"capsule_version": "1.0",
		"source_format":   detectedFormat,
		"original_file":   filename,
		"ingested_at":     time.Now().Format(time.RFC3339),
	}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(capsuleDir, "manifest.json"), manifestData, 0644)

	// Create capsule archive - sanitize filename to prevent path traversal
	baseName := filepath.Base(strings.TrimSuffix(filename, filepath.Ext(filename)))
	if baseName == "." || baseName == ".." || baseName == "" {
		baseName = "uploaded"
	}
	capsuleName := baseName + ".capsule.tar.gz"
	cleanName, err := validation.SanitizePath(ServerConfig.CapsulesDir, capsuleName)
	if err != nil {
		return &IngestResult{
			Success: false,
			Error:   "Invalid filename",
		}
	}
	capsulePath := filepath.Join(ServerConfig.CapsulesDir, cleanName)

	if err := archive.CreateCapsuleTarGzFromPath(capsuleDir, capsulePath); err != nil {
		return &IngestResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to create capsule: %v", err),
		}
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
	// Save to temp file
	tempDir, err := secureMkdirTemp("", "capsule-detect-*")
	if err != nil {
		return &DetectResult{
			Format:  "unknown",
			Details: fmt.Sprintf("Error: %v", err),
		}
	}
	defer os.RemoveAll(tempDir)

	tempPath := filepath.Join(tempDir, filename)
	outFile, err := os.Create(tempPath)
	if err != nil {
		return &DetectResult{
			Format:  "unknown",
			Details: fmt.Sprintf("Error: %v", err),
		}
	}
	if _, err := io.Copy(outFile, file); err != nil {
		outFile.Close()
		return &DetectResult{
			Format:  "unknown",
			Details: fmt.Sprintf("Error copying file: %v", err),
		}
	}
	if err := outFile.Close(); err != nil {
		return &DetectResult{
			Format:  "unknown",
			Details: fmt.Sprintf("Error closing file: %v", err),
		}
	}

	// Load plugins
	loader := getLoader()
	if err := loader.LoadFromDir(ServerConfig.PluginsDir); err != nil {
		format := detectFileFormatByExtension(tempPath)
		return &DetectResult{
			Format:     format,
			PluginID:   "(extension-based)",
			Confidence: "low",
			Details:    "Plugin detection unavailable, used extension-based detection",
		}
	}

	// Try each format plugin
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

	// Fallback
	format := detectFileFormatByExtension(tempPath)
	return &DetectResult{
		Format:     format,
		PluginID:   "(fallback)",
		Confidence: "low",
		Details:    "No plugin detected this format, using extension-based detection",
	}
}

// handleExport handles capsule artifact export.
func handleExport(w http.ResponseWriter, r *http.Request) {
	capsulePath := strings.TrimPrefix(r.URL.Path, "/export/")
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

	// Check if this is a download request
	artifactID := r.URL.Query().Get("artifact")
	if artifactID != "" && r.URL.Query().Get("download") == "true" {
		// Stream artifact content as download
		content, contentType, err := readArtifactContent(fullPath, artifactID)
		if err != nil {
			httpError(w, err, http.StatusNotFound)
			return
		}

		filename := filepath.Base(artifactID)
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		w.Write([]byte(content))
		return
	}

	// Show export page
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

	// Extract capsule to temp dir
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

	var checks []SelfcheckCheck

	switch planName {
	case "identity-bytes":
		// Read manifest and artifacts
		_, artifacts, err := readCapsule(capsulePath)
		if err != nil {
			return &SelfcheckResult{
				Success:  false,
				PlanName: planName,
				Message:  fmt.Sprintf("Failed to read capsule: %v", err),
			}
		}

		allPassed := true
		for _, artifact := range artifacts {
			if artifact.Name == "manifest.json" {
				continue
			}

			check := SelfcheckCheck{
				Name:     fmt.Sprintf("Identity check: %s", artifact.Name),
				Expected: fmt.Sprintf("%d bytes", artifact.Size),
			}

			// Read extracted file
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

		return &SelfcheckResult{
			Success:      allPassed,
			PlanName:     planName,
			CheckResults: checks,
			Message:      fmt.Sprintf("Completed %d checks", len(checks)),
			Duration:     time.Since(start).String(),
		}

	case "verify-hashes":
		// Verify all files exist and are readable
		_, artifacts, err := readCapsule(capsulePath)
		if err != nil {
			return &SelfcheckResult{
				Success:  false,
				PlanName: planName,
				Message:  fmt.Sprintf("Failed to read capsule: %v", err),
			}
		}

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

		return &SelfcheckResult{
			Success:      allPassed,
			PlanName:     planName,
			CheckResults: checks,
			Message:      fmt.Sprintf("Verified %d artifacts", len(checks)),
			Duration:     time.Since(start).String(),
		}

	default:
		return &SelfcheckResult{
			Success:  false,
			PlanName: planName,
			Message:  fmt.Sprintf("Unknown plan: %s", planName),
		}
	}
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

		if run.Outputs != nil && run.Outputs.TranscriptBlobSHA256 != "" {
			hash := run.Outputs.TranscriptBlobSHA256
			if len(hash) > 16 {
				runInfo.TranscriptHash = hash[:16] + "..."
			} else {
				runInfo.TranscriptHash = hash
			}
		}

		for _, input := range run.Inputs {
			runInfo.InputIDs = append(runInfo.InputIDs, input.ArtifactID)
		}

		runs = append(runs, runInfo)
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

// handleRunsCompare handles the runs compare page.
func handleRunsCompare(w http.ResponseWriter, r *http.Request) {
	capsulePath := strings.TrimPrefix(r.URL.Path, "/runs/compare/")
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
	tempDir, err := secureMkdirTemp("", "capsule-compare-*")
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

	// Get or create CSRF token
	csrfToken := getOrCreateCSRFToken(w, r)

	data := RunsCompareData{
		PageData: PageData{Title: "Compare Runs: " + capsulePath, CSRFToken: csrfToken},
		Capsule: CapsuleInfo{
			Name:      filepath.Base(capsulePath),
			Path:      capsulePath,
			Size:      info.Size(),
			SizeHuman: humanSize(info.Size()),
		},
		Runs: runs,
	}

	if r.Method == http.MethodPost {
		if !validateCSRFToken(r) {
			data.PageData.Error = "Invalid CSRF token. Please try again."
		} else {
			run1ID := r.FormValue("run1")
			run2ID := r.FormValue("run2")
			if run1ID != "" && run2ID != "" {
				result := performRunsCompare(tempDir, run1ID, run2ID)
				data.Result = result
			}
		}
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

	// Read manifest.json to get transcript hashes
	manifestPath := filepath.Join(extractDir, "capsule", "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return result
	}

	var manifest struct {
		Runs map[string]struct {
			Outputs *struct {
				TranscriptBlobSHA256 string `json:"transcript_blob_sha256"`
			} `json:"outputs"`
		} `json:"runs"`
		Blobs map[string]struct {
			Path string `json:"path"`
		} `json:"blobs"`
	}

	if err := json.Unmarshal(data, &manifest); err != nil {
		return result
	}

	// Get transcript hashes
	run1, ok := manifest.Runs[run1ID]
	if !ok {
		return result
	}
	run2, ok := manifest.Runs[run2ID]
	if !ok {
		return result
	}

	if run1.Outputs != nil {
		result.Hash1 = run1.Outputs.TranscriptBlobSHA256
	}
	if run2.Outputs != nil {
		result.Hash2 = run2.Outputs.TranscriptBlobSHA256
	}

	if result.Hash1 == "" || result.Hash2 == "" {
		return result
	}

	// Check if identical
	if result.Hash1 == result.Hash2 {
		result.Identical = true
		return result
	}

	// Read transcript contents
	var transcript1, transcript2 []byte
	if blob1, ok := manifest.Blobs[result.Hash1]; ok {
		transcript1 = readTranscriptBlob(extractDir, result.Hash1, blob1.Path)
	}
	if blob2, ok := manifest.Blobs[result.Hash2]; ok {
		transcript2 = readTranscriptBlob(extractDir, result.Hash2, blob2.Path)
	}

	// Parse transcripts as JSON lines
	events1 := parseTranscriptEvents(transcript1)
	events2 := parseTranscriptEvents(transcript2)

	result.EventCount1 = len(events1)
	result.EventCount2 = len(events2)

	// Find differences
	maxLen := len(events1)
	if len(events2) > maxLen {
		maxLen = len(events2)
	}

	for i := 0; i < maxLen; i++ {
		var e1, e2 string
		if i < len(events1) {
			e1 = events1[i]
		}
		if i < len(events2) {
			e2 = events2[i]
		}

		if e1 != e2 {
			result.Differences = append(result.Differences, DiffEntry{
				Index:  i,
				Event1: e1,
				Event2: e2,
			})
		}
	}

	return result
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
	// Read contrib/tool directory
	contribDir := "contrib/tool"
	tools := getToolsList(contribDir)

	data := ToolsData{
		PageData: PageData{Title: "Tool Plugins"},
	}

	// If contrib/tool has tools, show those
	if len(tools) > 0 {
		data.Tools = tools
		data.Source = "contrib/tool"
	} else {
		// Otherwise, load tool plugins from plugins directory
		loader := getLoader()
		if err := loader.LoadFromDir(ServerConfig.PluginsDir); err == nil {
			var toolPlugins []PluginInfo
			for _, p := range loader.ListPlugins() {
				if strings.HasPrefix(p.Manifest.PluginID, "tool.") || strings.HasPrefix(p.Manifest.PluginID, "tools.") {
					// Check if binary exists
					hasBinary := false
					if p.Manifest.Entrypoint != "" {
						binPath := filepath.Join(p.Path, p.Manifest.Entrypoint)
						if _, err := os.Stat(binPath); err == nil {
							hasBinary = true
						}
					}
					name := p.Manifest.PluginID
					name = strings.TrimPrefix(name, "tool.")
					name = strings.TrimPrefix(name, "tools.")
					toolPlugins = append(toolPlugins, PluginInfo{
						PluginID:  p.Manifest.PluginID,
						Name:      name,
						Version:   p.Manifest.Version,
						HasBinary: hasBinary,
					})
				}
			}
			data.ToolPlugins = toolPlugins
			data.Source = ServerConfig.PluginsDir + "/tool"
		}
	}

	if err := Templates.ExecuteTemplate(w, "tools.html", data); err != nil {
		httpError(w, err, http.StatusInternalServerError)
	}
}

// getToolsList lists available tools in contrib/tool.
func getToolsList(contribDir string) []ToolItem {
	var tools []ToolItem

	entries, err := os.ReadDir(contribDir)
	if err != nil {
		return tools
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		toolDir := filepath.Join(contribDir, name)

		// Check for README to confirm it's a tool
		readmePath := filepath.Join(toolDir, "README.md")
		if _, err := os.Stat(readmePath); errors.Is(err, os.ErrNotExist) {
			continue
		}

		tool := ToolItem{
			Name: name,
		}

		// Parse README for purpose
		readmeData, err := os.ReadFile(readmePath)
		if err == nil {
			tool.Purpose = extractPurposeFromReadme(string(readmeData))
		}

		// Check for license
		licensePath := filepath.Join(toolDir, "LICENSE.txt")
		if _, err := os.Stat(licensePath); err == nil {
			licenseData, err := os.ReadFile(licensePath)
			if err == nil {
				tool.License = extractLicenseType(string(licenseData))
			}
		}

		// Check for Nix definition
		nixPath := filepath.Join(toolDir, "nixos", "default.nix")
		if _, err := os.Stat(nixPath); err == nil {
			tool.HasNix = true
		}

		// Check for capsule
		capsuleDir := filepath.Join(toolDir, "capsule")
		if info, err := os.Stat(capsuleDir); err == nil && info.IsDir() {
			capsuleFiles, _ := os.ReadDir(capsuleDir)
			tool.HasCapsule = len(capsuleFiles) > 0
		}

		// Check for binaries
		binDir := filepath.Join(toolDir, "bin")
		if info, err := os.Stat(binDir); err == nil && info.IsDir() {
			binFiles, _ := os.ReadDir(binDir)
			tool.HasBin = len(binFiles) > 0
		}

		tools = append(tools, tool)
	}

	// Sort by name
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})

	return tools
}

// extractPurposeFromReadme extracts the purpose line from a README.
func extractPurposeFromReadme(readme string) string {
	lines := strings.Split(readme, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		// Skip the title
		if strings.HasPrefix(line, "# ") {
			// Check if next non-empty line is a description
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				nextLine := strings.TrimSpace(lines[j])
				if nextLine == "" {
					continue
				}
				if !strings.HasPrefix(nextLine, "#") && !strings.HasPrefix(nextLine, "[") {
					// Truncate if too long
					if len(nextLine) > 100 {
						nextLine = nextLine[:97] + "..."
					}
					return nextLine
				}
				break
			}
		}
	}
	return ""
}

// extractLicenseType extracts the license type from license text.
func extractLicenseType(license string) string {
	licenseLower := strings.ToLower(license)

	// Check against known patterns
	for _, lp := range licensePatterns {
		allMatch := true
		for _, pattern := range lp.patterns {
			if !strings.Contains(licenseLower, pattern) {
				allMatch = false
				break
			}
		}
		if allMatch {
			return lp.license
		}
	}

	// Fallback: try first line
	lines := strings.Split(license, "\n")
	for _, line := range lines {
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

			switch strings.ToLower(key) {
			case "description":
				module.Description = value
			case "lang":
				module.Language = value
			case "version":
				module.Version = value
			case "category":
				module.Category = value
			case "copyright":
				module.Copyright = value
			case "distributionlicense":
				module.License = value
			case "datapath":
				module.DataPath = value
			case "versification":
				module.Versification = value
			case "feature":
				module.Features = append(module.Features, value)
			case "globaloptionffilter":
				// GlobalOptionFilter can indicate features like StrongsNumbers
				if strings.Contains(strings.ToLower(value), "strongs") {
					module.Features = append(module.Features, "StrongsNumbers")
				}
				if strings.Contains(strings.ToLower(value), "morph") {
					module.Features = append(module.Features, "Morphology")
				}
				if strings.Contains(strings.ToLower(value), "footnotes") {
					module.Features = append(module.Features, "Footnotes")
				}
				if strings.Contains(strings.ToLower(value), "headings") {
					module.Features = append(module.Features, "Headings")
				}
			}
		}
	}

	return module
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
	if err := os.MkdirAll(filepath.Join(capsuleDir, "mods.d"), 0755); err != nil {
		result.Error = fmt.Sprintf("Failed to create capsule structure: %v", err)
		return result
	}

	// Copy conf file (preserve original filename for consistency)
	destConf := filepath.Join(capsuleDir, "mods.d", confFilename)
	if err := fileutil.CopyFile(confPath, destConf); err != nil {
		result.Error = fmt.Sprintf("Failed to copy conf: %v", err)
		return result
	}

	// Copy module data
	if module.DataPath != "" {
		srcData := filepath.Join(swordDir, module.DataPath)
		destData := filepath.Join(capsuleDir, module.DataPath)

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
				result.Error = fmt.Sprintf("Failed to find module data: %v", err)
				return result
			}
		}

		if !info.IsDir() {
			result.Error = fmt.Sprintf("Module data path is not a directory: %s", srcData)
			return result
		}

		if err := os.MkdirAll(filepath.Dir(destData), 0755); err != nil {
			result.Error = fmt.Sprintf("Failed to create data directory: %v", err)
			return result
		}

		if err := fileutil.CopyDir(srcData, destData); err != nil {
			result.Error = fmt.Sprintf("Failed to copy module data: %v", err)
			return result
		}
	}

	// Create output capsule archive
	outputPath := filepath.Join(ServerConfig.CapsulesDir, moduleID+".capsule.tar.gz")
	if err := archive.CreateTarGz(tempDir, outputPath, "capsule", true); err != nil {
		result.Error = fmt.Sprintf("Failed to create capsule: %v", err)
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

// runToolPlugin executes a tool plugin with the given profile.
func runToolPlugin(pluginID, profile, capsulePath, artifactID string) *ToolRunResult {
	start := time.Now()
	result := &ToolRunResult{
		PluginID: pluginID,
		Profile:  profile,
	}

	// Create output directory
	outputDir, err := secureMkdirTemp("", "tool-run-*")
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create output directory: %v", err)
		return result
	}
	result.OutputDir = outputDir

	// Load plugin
	loader := getLoader()
	if err := loader.LoadFromDir(ServerConfig.PluginsDir); err != nil {
		result.Error = fmt.Sprintf("Failed to load plugins: %v", err)
		return result
	}

	plugin, err := loader.GetPlugin(pluginID)
	if err != nil {
		result.Error = fmt.Sprintf("Plugin not found: %v", err)
		return result
	}

	// Create request file
	reqPath := filepath.Join(outputDir, "request.json")
	reqData := map[string]interface{}{
		"profile": profile,
		"args":    map[string]string{},
	}

	// If a capsule/artifact was selected, add input path
	if capsulePath != "" && artifactID != "" {
		// Sanitize path to prevent path traversal attacks
		cleanPath, err := validation.SanitizePath(ServerConfig.CapsulesDir, capsulePath)
		if err != nil {
			result.Error = "Invalid capsule path"
			return result
		}
		fullCapsulePath := filepath.Join(ServerConfig.CapsulesDir, cleanPath)
		inputPath := filepath.Join(outputDir, "input")
		if err := os.MkdirAll(inputPath, 0755); err == nil {
			// Extract the artifact
			if extractedPath, err := extractArtifact(fullCapsulePath, artifactID, inputPath); err == nil {
				reqData["args"] = map[string]string{"input": extractedPath}
			}
		}
	}

	reqJSON, _ := json.Marshal(reqData)
	if err := os.WriteFile(reqPath, reqJSON, 0644); err != nil {
		result.Error = fmt.Sprintf("Failed to write request: %v", err)
		return result
	}

	// Execute plugin with secure path validation
	entrypoint, err := plugin.SecureEntrypointPath()
	if err != nil {
		result.Error = fmt.Sprintf("Plugin validation failed: %v", err)
		return result
	}
	cmd := exec.Command(entrypoint, "run", "--request", reqPath, "--out", outputDir)
	cmd.Dir = plugin.Path
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Error = fmt.Sprintf("Tool execution failed: %v\nOutput: %s", err, string(output))
		return result
	}

	// Read transcript
	transcriptPath := filepath.Join(outputDir, "transcript.jsonl")
	if transcriptData, err := os.ReadFile(transcriptPath); err == nil {
		result.Transcript = string(transcriptData)
	}

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

// handleJuniper handles the unified Juniper SWORD modules page.
func handleJuniper(w http.ResponseWriter, r *http.Request) {
	csrfToken := getOrCreateCSRFToken(w, r)

	data := JuniperRepomanData{
		PageData: PageData{Title: "SWORD Modules", CSRFToken: csrfToken},
		Sources: []RepoSource{
			{Name: "CrossWire", URL: "https://www.crosswire.org/ftpmirror/pub/sword/raw"},
			{Name: "eBible", URL: "https://ebible.org/sword"},
		},
		SwordDir:    ServerConfig.SwordDir,
		CapsulesDir: ServerConfig.CapsulesDir,
		Formats:     []string{"osis", "usfm", "usx", "json", "html", "epub", "markdown", "sqlite", "txt"},
	}

	// Get tab and subtab from query params
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "installed" // Default to installed tab
	}
	data.Tab = tab

	subtab := r.URL.Query().Get("subtab")
	if tab == "capsules" && subtab == "" {
		subtab = "list" // Default to list subtab for capsules
	}
	data.SubTab = subtab

	// Check for message query param (used for success messages after redirects)
	if message := r.URL.Query().Get("message"); message != "" {
		data.PageData.Message = message
	}

	// Pagination for installed modules tab (22 per page)
	const installedPageSize = 22
	installedPage := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			installedPage = p
		}
	}
	data.InstalledPage = installedPage
	data.InstalledPageSize = installedPageSize

	// Get selected source from query param or form
	selectedSource := r.URL.Query().Get("source")
	if selectedSource == "" {
		selectedSource = r.FormValue("source")
	}
	if selectedSource == "" {
		selectedSource = "CrossWire" // Default
	}
	data.SelectedSource = selectedSource

	// Handle custom URL for "Custom" source
	customURL := r.URL.Query().Get("custom_url")
	if customURL == "" {
		customURL = r.FormValue("custom_url")
	}
	data.CustomURL = customURL

	// Determine the actual source name to use for plugin calls
	// When "Custom" is selected with a URL, use the URL as the source
	effectiveSource := selectedSource
	if selectedSource == "Custom" && customURL != "" {
		effectiveSource = customURL
	}

	// Check if modules should be loaded (via query param after POST redirect)
	shouldLoadModules := r.URL.Query().Get("loaded") == "1"

	// Handle POST actions
	if r.Method == http.MethodPost {
		// Check content type to determine if this is a file upload
		contentType := r.Header.Get("Content-Type")
		isMultipart := len(contentType) > 0 && contentType[:19] == "multipart/form-data"

		if isMultipart {
			// Handle file upload for detect action
			r.Body = http.MaxBytesReader(w, r.Body, validation.MaxFileSize)
			if err := r.ParseMultipartForm(MaxFormMemory); err != nil {
				data.PageData.Error = fmt.Sprintf("Failed to parse form: %v", err)
			} else if !validateCSRFToken(r) {
				data.PageData.Error = "Invalid CSRF token. Please try again."
			} else if r.FormValue("action") == "detect" {
				file, header, err := r.FormFile("file")
				if err != nil {
					data.PageData.Error = fmt.Sprintf("No file uploaded: %v", err)
				} else {
					defer file.Close()
					if header.Size > validation.MaxFileSize {
						data.PageData.Error = fmt.Sprintf("File too large: %d bytes (max: %d bytes)", header.Size, validation.MaxFileSize)
					} else {
						data.DetectResult = performDetect(file, header.Filename)
					}
				}
				tab = "detect"
			}
		} else {
			r.ParseForm()
		}
		if data.PageData.Error == "" && !isMultipart {
			if !validateCSRFToken(r) {
				data.PageData.Error = "Invalid CSRF token. Please try again."
			} else {
				action := r.FormValue("action")
				switch action {
				case "load":
					// Just mark modules as loaded, will load below
					shouldLoadModules = true
					tab = "repository"
				case "refresh":
					result := refreshRepoSource(effectiveSource)
					data.RefreshResult = result
					shouldLoadModules = true
					tab = "repository"
				case "install":
					moduleID := r.FormValue("module")
					if moduleID != "" {
						result := installModule(effectiveSource, moduleID, ServerConfig.SwordDir)
						data.InstallResult = result
					}
					shouldLoadModules = true
					tab = "repository"
				case "delete":
					moduleID := r.FormValue("module")
					if moduleID != "" {
						result := deleteModule(moduleID, ServerConfig.SwordDir)
						data.InstallResult = result
					}
					tab = "installed"
				case "ingest":
					selectedModules := r.Form["modules"]
					if len(selectedModules) > 0 {
						result := performJuniperIngest(ServerConfig.SwordDir, selectedModules)
						data.IngestResult = result
						// Redirect to capsules tab with success message if ingest was successful
						if result.Success {
							redirectURL := fmt.Sprintf("/juniper?tab=capsules&subtab=list&source=%s&message=%s",
								selectedSource, url.QueryEscape(fmt.Sprintf("Successfully created %d capsule(s)", result.ModulesOK)))
							http.Redirect(w, r, redirectURL, http.StatusSeeOther)
							return
						}
					}
					tab = "capsules"
				}
			}
		}

		// Redirect to GET with appropriate params to prevent form resubmission
		if data.PageData.Error == "" {
			redirectURL := fmt.Sprintf("/juniper?tab=%s&source=%s", tab, selectedSource)
			if shouldLoadModules {
				redirectURL += "&loaded=1"
			}
			if selectedSource == "Custom" && customURL != "" {
				redirectURL += "&custom_url=" + url.QueryEscape(customURL)
			}
			http.Redirect(w, r, redirectURL, http.StatusSeeOther)
			return
		}
	}

	// Load available modules for selected source only if requested
	if shouldLoadModules {
		modules, err := listRepoModules(effectiveSource)
		if err != nil {
			data.PageData.Error = fmt.Sprintf("Failed to load modules: %v", err)
		} else {
			data.Modules = modules
			data.ModulesLoaded = true

			// Collect unique languages and types for filtering
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
	}

	// Load installed modules with pagination
	installed := getInstalledModules(ServerConfig.SwordDir)
	data.InstalledTotal = len(installed)
	data.InstalledTotalPages = (len(installed) + installedPageSize - 1) / installedPageSize
	if data.InstalledTotalPages == 0 {
		data.InstalledTotalPages = 1
	}
	// Collect unique types and languages from all installed modules
	installedTypeSet := make(map[string]bool)
	installedLangSet := make(map[string]bool)
	for _, mod := range installed {
		if mod.Type != "" {
			installedTypeSet[mod.Type] = true
		}
		if mod.Language != "" {
			installedLangSet[mod.Language] = true
		}
	}
	for t := range installedTypeSet {
		data.InstalledTypes = append(data.InstalledTypes, t)
	}
	for l := range installedLangSet {
		data.InstalledLanguages = append(data.InstalledLanguages, l)
	}
	sort.Strings(data.InstalledTypes)
	sort.Strings(data.InstalledLanguages)
	// Clamp page to valid range
	if installedPage > data.InstalledTotalPages {
		installedPage = data.InstalledTotalPages
		data.InstalledPage = installedPage
	}
	// Slice installed modules for current page
	start := (installedPage - 1) * installedPageSize
	end := start + installedPageSize
	if end > len(installed) {
		end = len(installed)
	}
	if start < len(installed) {
		data.Installed = installed[start:end]
	}

	// Load local modules for ingest tab (convert SWORDModule to RepoModule)
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
	// Collect unique types and languages for local modules
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

	// Load capsules data for capsules tab
	if tab == "capsules" {
		allCapsules := listCapsules()
		capsulesNoIR, _, capsulesWithIR := categorizeCapsules(allCapsules)
		data.Capsules = allCapsules
		data.CapsulesNoIR = capsulesNoIR
		data.CapsulesWithIR = capsulesWithIR
	}

	// Load plugins data for plugins tab
	if tab == "plugins" {
		data.FormatPlugins = listFormatPlugins()
		data.ToolPlugins = listToolPlugins()
	}

	// Load info tab data
	if tab == "info" {
		data.FormatPlugins = listFormatPlugins()
		data.ToolPlugins = listToolPlugins()
		data.PluginsExternal = ServerConfig.PluginsExternal
		data.BibleCount = len(getCachedBibles())
		data.Capsules = listCapsules()
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

// parseRepoModuleConf parses a SWORD module .conf file.
func parseRepoModuleConf(conf, filename string) RepoModule {
	module := RepoModule{
		Name: strings.TrimSuffix(filename, ".conf"),
	}

	lines := strings.Split(conf, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			module.Name = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}

		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])

			switch strings.ToLower(key) {
			case "description":
				module.Description = value
			case "lang":
				module.Language = value
			case "version":
				module.Version = value
			case "moddrv":
				module.Type = moduleTypeFromModDrv(value)
			}
		}
	}

	return module
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

