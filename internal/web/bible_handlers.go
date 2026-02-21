package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/ir"
	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/archive"
)

// Pre-compiled regexes for performance (avoid recompilation on every request)
var (
	// chapterVerseRegex matches patterns like "BookID.chapter.verse" (e.g., "Gen.1.1")
	chapterVerseRegex = regexp.MustCompile(`\.(\d+)\.(\d+)`)
	// strongsSearchRegex matches Strong's number format (H1234 or G5678)
	strongsSearchRegex = regexp.MustCompile(`^[HG]\d+$`)
)

// startupReady is an atomic bool for lock-free ready checks (hot path optimization).
var startupReady atomic.Bool

// startupState tracks server startup progress for the splash screen.
// Uses RWMutex only for message/detail/progress (not checked on hot path).
var startupState struct {
	sync.RWMutex
	message  string
	detail   string
	progress int // 0-100
}

// StartupStatus returns the current startup state for the splash screen.
type StartupStatus struct {
	Ready    bool   `json:"ready"`
	Message  string `json:"message,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Progress int    `json:"progress,omitempty"`
}

// GetStartupStatus returns the current startup status.
func GetStartupStatus() StartupStatus {
	startupState.RLock()
	defer startupState.RUnlock()
	return StartupStatus{
		Ready:    startupReady.Load(),
		Message:  startupState.message,
		Detail:   startupState.detail,
		Progress: startupState.progress,
	}
}

// IsStartupReady returns true if the server has finished warming up.
// Uses atomic load for lock-free hot path performance.
func IsStartupReady() bool {
	return startupReady.Load()
}

func setStartupProgress(message, detail string, progress int) {
	startupState.Lock()
	startupState.message = message
	startupState.detail = detail
	startupState.progress = progress
	startupState.Unlock()
}

func setStartupReady() {
	startupState.Lock()
	startupState.message = "Ready"
	startupState.detail = ""
	startupState.progress = 100
	startupState.Unlock()
	startupReady.Store(true) // Atomic store - lock-free
}

// bibleCache caches Bible info to avoid re-parsing capsules on every request.
var bibleCache struct {
	sync.RWMutex
	bibles    []BibleInfo
	populated bool // true if cache has been populated (even if empty)
	timestamp time.Time
	ttl       time.Duration
}

// corpusCache caches parsed IR corpora for individual Bibles.
// This speeds up Bible detail, book, and chapter views significantly.
var corpusCache struct {
	sync.RWMutex
	corpora map[string]*corpusCacheEntry
	ttl     time.Duration
}

type corpusCacheEntry struct {
	corpus    *ir.Corpus
	capsuleID string
	timestamp time.Time
}

// manageableBiblesCache caches the installed/installable lists for the Manage tab.
// This is expensive to compute since it calls archive.HasIR() and reads SWORD conf files.
var manageableBiblesCache struct {
	sync.RWMutex
	installed   []ManageableBible
	installable []ManageableBible
	populated   bool
	timestamp   time.Time
	ttl         time.Duration
}

func init() {
	bibleCache.ttl = 30 * time.Minute // Cache for 30 minutes (expensive to rebuild)
	corpusCache.corpora = make(map[string]*corpusCacheEntry)
	corpusCache.ttl = 60 * time.Minute           // Cache corpora for 1 hour since they're very expensive to load
	manageableBiblesCache.ttl = 10 * time.Minute // Cache manageable bibles list
}

// getCachedBibles returns cached Bible list or rebuilds if expired.
func getCachedBibles() []BibleInfo {
	bibleCache.RLock()
	if bibleCache.populated && time.Since(bibleCache.timestamp) < bibleCache.ttl {
		bibles := bibleCache.bibles
		bibleCache.RUnlock()
		return bibles
	}
	bibleCache.RUnlock()

	// Rebuild cache
	bibleCache.Lock()
	defer bibleCache.Unlock()

	// Double-check after acquiring write lock
	if bibleCache.populated && time.Since(bibleCache.timestamp) < bibleCache.ttl {
		return bibleCache.bibles
	}

	start := time.Now()
	bibleCache.bibles = listBiblesUncached()
	bibleCache.populated = true
	bibleCache.timestamp = time.Now()
	log.Printf("[CACHE] Rebuilt Bible cache: %d bibles in %v", len(bibleCache.bibles), time.Since(start))

	return bibleCache.bibles
}

// invalidateBibleCache forces a cache rebuild on next access.
func invalidateBibleCache() {
	bibleCache.Lock()
	bibleCache.populated = false
	bibleCache.timestamp = time.Time{}
	bibleCache.Unlock()
}

// invalidateCorpusCache clears all cached corpora.
func invalidateCorpusCache() {
	corpusCache.Lock()
	corpusCache.corpora = make(map[string]*corpusCacheEntry)
	corpusCache.Unlock()
}

// invalidateManageableBiblesCache forces a cache rebuild on next access.
func invalidateManageableBiblesCache() {
	manageableBiblesCache.Lock()
	manageableBiblesCache.populated = false
	manageableBiblesCache.timestamp = time.Time{}
	manageableBiblesCache.Unlock()
}

// getCachedManageableBibles returns cached manageable bibles lists or rebuilds if expired.
func getCachedManageableBibles() (installed, installable []ManageableBible) {
	manageableBiblesCache.RLock()
	if manageableBiblesCache.populated && time.Since(manageableBiblesCache.timestamp) < manageableBiblesCache.ttl {
		installed = manageableBiblesCache.installed
		installable = manageableBiblesCache.installable
		manageableBiblesCache.RUnlock()
		return installed, installable
	}
	manageableBiblesCache.RUnlock()

	// Rebuild cache
	manageableBiblesCache.Lock()
	defer manageableBiblesCache.Unlock()

	// Double-check after acquiring write lock
	if manageableBiblesCache.populated && time.Since(manageableBiblesCache.timestamp) < manageableBiblesCache.ttl {
		return manageableBiblesCache.installed, manageableBiblesCache.installable
	}

	start := time.Now()
	manageableBiblesCache.installed, manageableBiblesCache.installable = listManageableBiblesUncached()
	manageableBiblesCache.populated = true
	manageableBiblesCache.timestamp = time.Now()
	log.Printf("[CACHE] Rebuilt manageable bibles cache: %d installed, %d installable in %v",
		len(manageableBiblesCache.installed), len(manageableBiblesCache.installable), time.Since(start))

	return manageableBiblesCache.installed, manageableBiblesCache.installable
}

// getCachedCorpus returns a cached corpus or loads it from disk.
func getCachedCorpus(capsuleID string) (*ir.Corpus, string, error) {
	corpusCache.RLock()
	if entry, ok := corpusCache.corpora[capsuleID]; ok {
		if time.Since(entry.timestamp) < corpusCache.ttl {
			corpus := entry.corpus
			path := entry.capsuleID
			corpusCache.RUnlock()
			return corpus, path, nil
		}
	}
	corpusCache.RUnlock()

	// Load from disk
	capsules := listCapsules()
	var capsulePath string
	for _, c := range capsules {
		id := archive.ExtractCapsuleID(c.Name)
		if strings.EqualFold(id, capsuleID) {
			capsulePath = c.Path
			break
		}
	}
	if capsulePath == "" {
		return nil, "", fmt.Errorf("capsule not found: %s", capsuleID)
	}

	irContent, err := readIRContent(filepath.Join(ServerConfig.CapsulesDir, capsulePath))
	if err != nil {
		return nil, "", err
	}

	corpus := parseIRToCorpus(irContent)
	if corpus == nil {
		return nil, "", fmt.Errorf("invalid IR content")
	}

	// Store in cache
	corpusCache.Lock()
	corpusCache.corpora[capsuleID] = &corpusCacheEntry{
		corpus:    corpus,
		capsuleID: capsulePath,
		timestamp: time.Now(),
	}
	corpusCache.Unlock()

	log.Printf("[CACHE] Loaded corpus for %s", capsuleID)
	return corpus, capsulePath, nil
}

// PreWarmCaches pre-populates caches on server startup.
// This runs in a goroutine so it doesn't block server startup.
// Progress is tracked via startupState for the splash screen.
func PreWarmCaches() {
	go func() {
		start := time.Now()
		setStartupProgress("Initializing", "Starting cache warmup...", 0)
		log.Println("[CACHE] Pre-warming caches...")

		// 1. First preload capsule metadata (HasIR checks) - this is fast and needed for other operations
		setStartupProgress("Scanning capsules", "Reading capsule metadata...", 5)
		preloadCapsuleMetadata()

		var wg sync.WaitGroup
		var bibles []BibleInfo
		var installed, installable []ManageableBible

		// 2. Warm Bible list and manageable caches in parallel
		setStartupProgress("Loading Bibles", "Scanning Bible library...", 15)
		wg.Add(2)

		go func() {
			defer wg.Done()
			bibles = getCachedBibles()
			log.Printf("[CACHE] Bible list cache warmed: %d Bibles", len(bibles))
		}()

		go func() {
			defer wg.Done()
			installed, installable = getCachedManageableBibles()
			log.Printf("[CACHE] Manageable Bibles cache warmed: %d installed, %d installable", len(installed), len(installable))
		}()

		wg.Wait()
		setStartupProgress("Loading Bibles", fmt.Sprintf("Found %d installed Bibles", len(bibles)), 30)

		// 3. Pre-load all installed Bible corpora in parallel with progress tracking
		if len(bibles) > 0 {
			log.Printf("[CACHE] Pre-loading %d Bible corpora...", len(bibles))
			preloadCorporaWithProgress(bibles)
		}

		setStartupReady()
		log.Printf("[CACHE] Pre-warm complete in %v", time.Since(start))
	}()
}

// preloadCorpora loads all Bible corpora into cache in parallel (without progress tracking).
func preloadCorpora(bibles []BibleInfo) {
	preloadCorporaWithProgress(bibles)
}

// preloadCorporaWithProgress loads all Bible corpora into cache in parallel with startup progress tracking.
func preloadCorporaWithProgress(bibles []BibleInfo) {
	var wg sync.WaitGroup
	// Use more workers for I/O-bound work (decompressing archives)
	// Cap at 16 to avoid resource exhaustion
	numWorkers := 16
	sem := make(chan struct{}, numWorkers)
	var loaded, failed int32
	totalBibles := int32(len(bibles))

	// Pre-calculate log interval (every 10% or so)
	logInterval := int32((totalBibles + 9) / 10)
	if logInterval < 1 {
		logInterval = 1
	}

	for _, bible := range bibles {
		wg.Add(1)
		go func(b BibleInfo) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire semaphore
			defer func() { <-sem }() // Release semaphore

			_, _, err := getCachedCorpus(b.ID)
			if err != nil {
				atomic.AddInt32(&failed, 1)
				log.Printf("[CACHE] Failed to preload corpus for %s: %v", b.ID, err)
			} else {
				count := atomic.AddInt32(&loaded, 1)
				// Update progress for splash screen (30% to 95% range)
				progress := 30 + int((float64(count)/float64(totalBibles))*65)
				setStartupProgress("Loading corpora", fmt.Sprintf("Loaded %d/%d Bibles", count, totalBibles), progress)

				if count%logInterval == 0 || count == totalBibles {
					log.Printf("[CACHE] Preloaded %d/%d corpora", count, totalBibles)
				}
			}
		}(bible)
	}

	wg.Wait()
	log.Printf("[CACHE] Corpus preload complete: %d loaded, %d failed", loaded, failed)
}

// StartBackgroundCacheRefresh starts a goroutine that refreshes caches
// before they expire to ensure users never experience cache miss latency.
func StartBackgroundCacheRefresh() {
	go func() {
		// Refresh at 80% of TTL to ensure cache is always warm
		refreshInterval := time.Duration(float64(bibleCache.ttl) * 0.8)
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		for range ticker.C {
			bibleCache.RLock()
			needsRefresh := bibleCache.populated && time.Since(bibleCache.timestamp) > refreshInterval
			bibleCache.RUnlock()

			if needsRefresh {
				log.Println("[CACHE] Background refresh starting...")
				invalidateBibleCache()
				getCachedBibles()
			}
		}
	}()
}

// BibleInfo describes a Bible for the index page.
type BibleInfo struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Abbrev        string   `json:"abbrev"`
	Language      string   `json:"language"`
	Versification string   `json:"versification"`
	BookCount     int      `json:"book_count"`
	Features      []string `json:"features"`
	CapsulePath   string   `json:"capsule_path"`
}

// BookInfo describes a book in a Bible.
type BookInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Order        int    `json:"order"`
	ChapterCount int    `json:"chapter_count"`
	Testament    string `json:"testament"`
}

// ChapterInfo describes a chapter.
type ChapterInfo struct {
	Number     int `json:"number"`
	VerseCount int `json:"verse_count"`
}

// VerseData represents a single verse.
type VerseData struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

// ChapterData contains the verses of a chapter.
type ChapterData struct {
	BibleID string      `json:"bible_id"`
	Book    string      `json:"book"`
	Chapter int         `json:"chapter"`
	Verses  []VerseData `json:"verses"`
}

// SearchResult represents a search match.
type SearchResult struct {
	BibleID   string `json:"bible_id"`
	Reference string `json:"reference"`
	Book      string `json:"book"`
	Chapter   int    `json:"chapter"`
	Verse     int    `json:"verse"`
	Text      string `json:"text"`
}

// ManageableBible represents a Bible that can be installed or is already installed.
type ManageableBible struct {
	ID          string   // Capsule ID or SWORD module name
	Name        string   // Display name
	Source      string   // "capsule" or "sword"
	SourcePath  string   // Path to capsule or SWORD module
	IsInstalled bool     // Has IR generated
	Format      string   // Detected format
	Tags        []string // Tags for filtering (source, format, identified, etc.)
	Language    string   // Language code (e.g., "en", "es", "de")
}

// HasTag checks if a ManageableBible has a specific tag.
func (mb ManageableBible) HasTag(tag string) bool {
	for _, t := range mb.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// listManageableBiblesUncached returns lists of installed and installable Bibles without caching.
// Installed = capsules with IR
// Installable = capsules without IR + SWORD modules from ~/.sword/
// Uses parallel processing for metadata lookups.
type capsuleResult struct {
	mb          ManageableBible
	isInstalled bool
}

func buildCapsuleResult(c CapsuleInfo, installedBibleInfo map[string]BibleInfo) capsuleResult {
	capsuleID := archive.ExtractCapsuleID(c.Name)
	fullPath := filepath.Join(ServerConfig.CapsulesDir, c.Path)
	hasIR := getCapsuleMetadata(fullPath).HasIR

	tags := []string{"capsule"}
	if c.Format != "" {
		tags = append(tags, c.Format)
	}
	if hasIR {
		tags = append(tags, "identified")
	}

	name := capsuleID
	var language string
	if bibleInfo, ok := installedBibleInfo[capsuleID]; ok {
		language = bibleInfo.Language
		if bibleInfo.Title != "" {
			name = bibleInfo.Title
		}
	}

	mb := ManageableBible{
		ID:          capsuleID,
		Name:        name,
		Source:      "capsule",
		SourcePath:  c.Path,
		IsInstalled: hasIR,
		Format:      c.Format,
		Tags:        tags,
		Language:    language,
	}
	return capsuleResult{mb: mb, isInstalled: hasIR}
}

func appendNewSWORDModules(installable []ManageableBible, installedBibleInfo map[string]BibleInfo) []ManageableBible {
	for _, sm := range listSWORDModules() {
		if _, exists := installedBibleInfo[sm.ID]; !exists {
			installable = append(installable, sm)
		}
	}
	return installable
}

func sortManageableBibles(installed, installable []ManageableBible) {
	sort.Slice(installed, func(i, j int) bool { return installed[i].Name < installed[j].Name })
	sort.Slice(installable, func(i, j int) bool { return installable[i].Name < installable[j].Name })
}

func listManageableBiblesUncached() (installed, installable []ManageableBible) {
	capsules := listCapsules()
	if len(capsules) == 0 {
		return nil, listSWORDModules()
	}

	installedBibleInfo := make(map[string]BibleInfo)
	for _, bible := range getCachedBibles() {
		installedBibleInfo[bible.ID] = bible
	}

	pool := NewWorkerPool[CapsuleInfo, capsuleResult](maxWorkers, len(capsules))
	pool.Start(func(c CapsuleInfo) capsuleResult {
		return buildCapsuleResult(c, installedBibleInfo)
	})

	for _, c := range capsules {
		pool.Submit(c)
	}
	pool.Close()

	for r := range pool.Results() {
		if r.isInstalled {
			installed = append(installed, r.mb)
		} else {
			installable = append(installable, r.mb)
		}
	}

	installable = appendNewSWORDModules(installable, installedBibleInfo)
	sortManageableBibles(installed, installable)

	return installed, installable
}

// swordModulesCache caches SWORD modules list to avoid re-reading conf files.
// SWORD modules don't change often, so we use a longer TTL.
var swordModulesCache struct {
	sync.RWMutex
	modules   []ManageableBible
	populated bool
	timestamp time.Time
	ttl       time.Duration
}

func init() {
	swordModulesCache.ttl = 10 * time.Minute // SWORD modules rarely change
}

// getCachedSWORDModules returns cached SWORD modules or rebuilds if expired.
func getCachedSWORDModules() []ManageableBible {
	swordModulesCache.RLock()
	if swordModulesCache.populated && time.Since(swordModulesCache.timestamp) < swordModulesCache.ttl {
		modules := swordModulesCache.modules
		swordModulesCache.RUnlock()
		return modules
	}
	swordModulesCache.RUnlock()

	// Rebuild cache
	swordModulesCache.Lock()
	defer swordModulesCache.Unlock()

	// Double-check after acquiring write lock
	if swordModulesCache.populated && time.Since(swordModulesCache.timestamp) < swordModulesCache.ttl {
		return swordModulesCache.modules
	}

	start := time.Now()
	swordModulesCache.modules = listSWORDModulesUncached()
	swordModulesCache.populated = true
	swordModulesCache.timestamp = time.Now()
	log.Printf("[CACHE] Rebuilt SWORD modules cache: %d modules in %v", len(swordModulesCache.modules), time.Since(start))

	return swordModulesCache.modules
}

// listSWORDModules returns cached SWORD Bible modules.
func listSWORDModules() []ManageableBible {
	return getCachedSWORDModules()
}

// swordConfFile represents a conf file to process.
type swordConfFile struct {
	path string
	name string
}

var swordVersificationTags = map[string]string{
	"catholic":    "catholic",
	"catholic2":   "catholic",
	"vulg":        "catholic",
	"lxx":         "catholic",
	"kjv":         "protestant",
	"nrsv":        "protestant",
	"luther":      "protestant",
	"german":      "protestant",
	"leningrad":   "protestant",
	"orthodox":    "orthodox",
	"synodal":     "orthodox",
	"synodalprot": "orthodox",
}

var swordFeatureTags = map[string]string{
	"strongsnumbers": "strongs",
	"images":         "images",
	"greekdef":       "definitions",
	"hebrewdef":      "definitions",
	"greekparse":     "parsing",
	"morphology":     "morphology",
	"footnotes":      "footnotes",
	"headings":       "headings",
}

func swordModuleTags(module SWORDModule) []string {
	tags := []string{"sword", "identified"}
	versTag := swordVersificationTags[strings.ToLower(module.Versification)]
	if versTag == "" {
		versTag = "protestant"
	}
	tags = append(tags, versTag)
	for _, feature := range module.Features {
		if tag, ok := swordFeatureTags[strings.ToLower(feature)]; ok {
			tags = append(tags, tag)
		}
	}
	return tags
}

func parseSWORDConfFile(cf swordConfFile) *ManageableBible {
	confContent, err := os.ReadFile(cf.path)
	if err != nil {
		return nil
	}
	module := parseSWORDConf(string(confContent), cf.name)
	if module.ID == "" || module.Category != "Biblical Texts" {
		return nil
	}
	name := module.Description
	if name == "" {
		name = module.ID
	}
	return &ManageableBible{
		ID:          module.ID,
		Name:        name,
		Source:      "sword",
		SourcePath:  cf.path,
		IsInstalled: false,
		Format:      "sword",
		Tags:        swordModuleTags(module),
		Language:    module.Language,
	}
}

func collectSWORDConfFiles(modsDir string) []swordConfFile {
	files, err := os.ReadDir(modsDir)
	if err != nil {
		return nil
	}
	var confFiles []swordConfFile
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".conf") {
			continue
		}
		confFiles = append(confFiles, swordConfFile{
			path: filepath.Join(modsDir, f.Name()),
			name: f.Name(),
		})
	}
	return confFiles
}

func listSWORDModulesUncached() []ManageableBible {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	modsDir := filepath.Join(homeDir, ".sword", "mods.d")
	if _, err := os.Stat(modsDir); os.IsNotExist(err) {
		return nil
	}

	confFiles := collectSWORDConfFiles(modsDir)
	if len(confFiles) == 0 {
		return nil
	}

	type result struct{ module *ManageableBible }
	pool := NewWorkerPool[swordConfFile, result](32, len(confFiles))
	pool.Start(func(cf swordConfFile) result { return result{module: parseSWORDConfFile(cf)} })
	for _, cf := range confFiles {
		pool.Submit(cf)
	}
	pool.Close()

	var modules []ManageableBible
	for r := range pool.Results() {
		if r.module != nil {
			modules = append(modules, *r.module)
		}
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Name < modules[j].Name
	})
	return modules
}

// BibleIndexData is the data for the Bible index page.
type BibleIndexData struct {
	PageData
	Bibles        []BibleInfo
	AllBibles     []BibleInfo // All bibles for compare tab
	Languages     []string
	Features      []string
	Tab           string
	Query         string
	BibleID       string
	CaseSensitive bool
	WholeWord     bool
	Results       []SearchResult
	Total         int
	// Pagination
	Page           int
	PerPage        int
	TotalPages     int
	PerPageOptions []int
	// Manage tab
	InstalledBibles          []ManageableBible
	InstallableBibles        []ManageableBible
	AllInstalledBibles       []ManageableBible // Unfiltered for counts
	AllInstallableBibles     []ManageableBible // Unfiltered for counts
	ManageTagFilter          string            // Selected tag filter (e.g., "sword", "capsule", "tar.gz", "identified")
	ManageAvailableTags      []string          // All unique tags available for filtering
	ManageLanguageFilter     string            // Selected language filter
	ManageAvailableLanguages []string          // All unique languages available for filtering
	InstalledPage            int
	InstalledTotalPages      int
	InstallablePage          int
	InstallableTotalPages    int
	ManagePerPage            int
}

// BibleViewData is the data for viewing a single Bible.
type BibleViewData struct {
	PageData
	Bible BibleInfo
	Books []BookInfo
}

// BookViewData is the data for viewing a single book.
type BookViewData struct {
	PageData
	Bible    BibleInfo
	Book     BookInfo
	Chapters []ChapterInfo
}

// ChapterViewData is the data for viewing a chapter.
type ChapterViewData struct {
	PageData
	Bible     BibleInfo
	Book      BookInfo
	Chapter   int
	Verses    []VerseData
	PrevURL   string
	NextURL   string
	AllBibles []BibleInfo // For Bible dropdown
	AllBooks  []BookInfo  // For Book dropdown
	Chapters  []int       // For Chapter dropdown
	// For handling missing content
	RequestedBook    string // Original book ID requested
	RequestedChapter int    // Original chapter requested
	NotFoundMessage  string // Message when content doesn't exist
}

// SearchData is the data for the search page.
type SearchData struct {
	PageData
	Bibles  []BibleInfo
	Query   string
	BibleID string
	Results []SearchResult
	Total   int
}

// handleBibleIndex shows the Bible reader.
// - If no capsules: show empty state
// - If DRC.capsule exists: redirect to /bible/DRC/Gen/1
// - For browsing multiple capsules: use /library/bibles/
func handleBibleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/bible" && r.URL.Path != "/bible/" {
		handleBibleRouting(w, r)
		return
	}
	allBibles := getCachedBibles()
	if len(allBibles) == 0 {
		showEmptyBibleState(w)
		return
	}
	if url := findDefaultBibleURL(allBibles); url != "" {
		http.Redirect(w, r, url, http.StatusFound)
		return
	}
	http.Redirect(w, r, "/library/bibles/", http.StatusFound)
}

// showEmptyBibleState renders the empty Bible state page.
func showEmptyBibleState(w http.ResponseWriter) {
	data := BibleIndexData{
		PageData: PageData{Title: "Bible"},
		Bibles:   nil,
	}
	if err := Templates.ExecuteTemplate(w, "bible_empty.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// findDefaultBibleURL returns the URL for the default Bible to display.
func findDefaultBibleURL(allBibles []BibleInfo) string {
	for _, b := range allBibles {
		if strings.EqualFold(b.ID, "DRC") {
			return "/bible/DRC/Gen/1"
		}
	}
	if len(allBibles) > 0 {
		bible, books, err := loadBibleWithBooks(allBibles[0].ID)
		if err == nil && len(books) > 0 {
			return fmt.Sprintf("/bible/%s/%s/1", bible.ID, books[0].ID)
		}
	}
	return ""
}

// paginationParams holds pagination configuration and results.
type paginationParams struct {
	Page           int
	PerPage        int
	TotalPages     int
	PerPageOptions []int
}

// manageTabData holds all data for the manage tab.
type manageTabData struct {
	InstalledBibles          []ManageableBible
	InstallableBibles        []ManageableBible
	AllInstalledBibles       []ManageableBible
	AllInstallableBibles     []ManageableBible
	TagFilter                string
	AvailableTags            []string
	LanguageFilter           string
	AvailableLanguages       []string
	InstalledPage            int
	InstalledTotalPages      int
	InstallablePage          int
	InstallableTotalPages    int
	PerPage                  int
}

// collectTagsAndLanguages extracts unique tags and languages from manageable bibles.
func collectTagsAndLanguages(installed, installable []ManageableBible) (tags, languages []string) {
	tagSet := make(map[string]bool)
	langSet := make(map[string]bool)

	collectFromBibles(installed, tagSet, langSet)
	collectFromBibles(installable, tagSet, langSet)

	return sortedKeys(tagSet), sortedKeys(langSet)
}

func collectFromBibles(bibles []ManageableBible, tagSet, langSet map[string]bool) {
	for _, b := range bibles {
		for _, tag := range b.Tags {
			tagSet[tag] = true
		}
		if b.Language != "" {
			langSet[b.Language] = true
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// filterManageableBibles filters bibles by tag and language.
func filterManageableBibles(bibles []ManageableBible, tagFilter, langFilter string) []ManageableBible {
	var filtered []ManageableBible
	for _, b := range bibles {
		tagMatch := tagFilter == "all" || b.HasTag(tagFilter)
		langMatch := langFilter == "all" || b.Language == langFilter
		if tagMatch && langMatch {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

// paginateManageableBibles paginates a list of manageable bibles.
func paginateManageableBibles(r *http.Request, bibles []ManageableBible, pageParam string, perPage int) (paginated []ManageableBible, page, totalPages int) {
	page = 1
	if pageStr := r.URL.Query().Get(pageParam); pageStr != "" {
		fmt.Sscanf(pageStr, "%d", &page)
		if page < 1 {
			page = 1
		}
	}

	totalPages = (len(bibles) + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * perPage
	end := start + perPage
	if end > len(bibles) {
		end = len(bibles)
	}
	if start < len(bibles) {
		paginated = bibles[start:end]
	}

	return paginated, page, totalPages
}

// processManageTab handles all logic for the manage tab.
func processManageTab(r *http.Request) manageTabData {
	data := manageTabData{PerPage: 10}

	allInstalled, allInstallable := getCachedManageableBibles()
	data.AllInstalledBibles = allInstalled
	data.AllInstallableBibles = allInstallable

	data.AvailableTags, data.AvailableLanguages = collectTagsAndLanguages(allInstalled, allInstallable)

	data.TagFilter = r.URL.Query().Get("tag")
	if data.TagFilter == "" {
		data.TagFilter = "all"
	}

	data.LanguageFilter = r.URL.Query().Get("lang")
	if data.LanguageFilter == "" {
		data.LanguageFilter = "all"
	}

	filteredInstalled := filterManageableBibles(allInstalled, data.TagFilter, data.LanguageFilter)
	filteredInstallable := filterManageableBibles(allInstallable, data.TagFilter, data.LanguageFilter)

	data.InstalledBibles, data.InstalledPage, data.InstalledTotalPages = paginateManageableBibles(r, filteredInstalled, "ipage", data.PerPage)
	data.InstallableBibles, data.InstallablePage, data.InstallableTotalPages = paginateManageableBibles(r, filteredInstallable, "upage", data.PerPage)

	return data
}

func parsePerPage(r *http.Request, options []int) int {
	perPageStr := r.URL.Query().Get("perPage")
	if perPageStr == "" {
		return options[0]
	}
	var v int
	fmt.Sscanf(perPageStr, "%d", &v)
	for _, opt := range options {
		if v == opt {
			return v
		}
	}
	return options[0]
}

func parsePage(r *http.Request) int {
	pageStr := r.URL.Query().Get("page")
	if pageStr == "" {
		return 1
	}
	var v int
	fmt.Sscanf(pageStr, "%d", &v)
	if v < 1 {
		return 1
	}
	return v
}

func sliceBibles(allBibles []BibleInfo, page, perPage int) ([]BibleInfo, int, int) {
	totalItems := len(allBibles)
	totalPages := (totalItems + perPage - 1) / perPage
	if page > totalPages {
		page = totalPages
	}
	if page < 1 {
		page = 1
	}
	start := (page - 1) * perPage
	end := start + perPage
	if end > totalItems {
		end = totalItems
	}
	if start >= totalItems {
		return nil, page, totalPages
	}
	return allBibles[start:end], page, totalPages
}

func calculateBrowsePagination(r *http.Request, allBibles []BibleInfo, query, tab string) (paginatedBibles []BibleInfo, params paginationParams) {
	params.PerPageOptions = []int{11, 22, 33, 44, 55, 66}
	params.PerPage = parsePerPage(r, params.PerPageOptions)
	params.Page = parsePage(r)

	if query != "" || tab == "compare" {
		return allBibles, paginationParams{PerPageOptions: params.PerPageOptions, PerPage: params.PerPage, Page: 1, TotalPages: 1}
	}

	paginatedBibles, params.Page, params.TotalPages = sliceBibles(allBibles, params.Page, params.PerPage)
	return paginatedBibles, params
}

// performSearch executes Bible search across one or all Bibles.
func performSearch(query, bibleID string, allBibles []BibleInfo) ([]SearchResult, int) {
	if query == "" {
		return nil, 0
	}

	if bibleID != "" {
		return searchBible(bibleID, query, 100)
	}

	// Search all Bibles
	var results []SearchResult
	var total int
	for _, b := range allBibles {
		r, t := searchBible(b.ID, query, 100-len(results))
		results = append(results, r...)
		total += t
		if len(results) >= 100 {
			break
		}
	}
	return results, total
}

// collectLanguagesAndFeatures extracts unique languages and features from Bible list.
func collectLanguagesAndFeatures(bibles []BibleInfo) (languages, features []string) {
	langMap := make(map[string]bool)
	featMap := make(map[string]bool)

	for _, b := range bibles {
		if b.Language != "" {
			langMap[b.Language] = true
		}
		for _, f := range b.Features {
			featMap[f] = true
		}
	}

	for l := range langMap {
		languages = append(languages, l)
	}
	for f := range featMap {
		features = append(features, f)
	}
	sort.Strings(languages)
	sort.Strings(features)
	return languages, features
}

// handleLibraryBibles shows the Bible browsing/search/compare interface.
func handleLibraryBibles(w http.ResponseWriter, r *http.Request) {
	allBibles := getCachedBibles()
	languages, features := collectLanguagesAndFeatures(allBibles)

	tab := r.URL.Query().Get("tab")
	query := r.URL.Query().Get("q")
	bibleID := r.URL.Query().Get("bible")
	caseSensitive := r.URL.Query().Get("case") == "1"
	wholeWord := r.URL.Query().Get("word") == "1"

	results, total := performSearch(query, bibleID, allBibles)
	paginatedBibles, pagination := calculateBrowsePagination(r, allBibles, query, tab)

	var manageData manageTabData
	if tab == "manage" {
		manageData = processManageTab(r)
	}

	data := BibleIndexData{
		PageData:                 PageData{Title: "Bible"},
		Bibles:                   paginatedBibles,
		AllBibles:                allBibles,
		Languages:                languages,
		Features:                 features,
		Tab:                      tab,
		Query:                    query,
		BibleID:                  bibleID,
		CaseSensitive:            caseSensitive,
		WholeWord:                wholeWord,
		Results:                  results,
		Total:                    total,
		Page:                     pagination.Page,
		PerPage:                  pagination.PerPage,
		TotalPages:               pagination.TotalPages,
		PerPageOptions:           pagination.PerPageOptions,
		InstalledBibles:          manageData.InstalledBibles,
		InstallableBibles:        manageData.InstallableBibles,
		AllInstalledBibles:       manageData.AllInstalledBibles,
		AllInstallableBibles:     manageData.AllInstallableBibles,
		ManageTagFilter:          manageData.TagFilter,
		ManageAvailableTags:      manageData.AvailableTags,
		ManageLanguageFilter:     manageData.LanguageFilter,
		ManageAvailableLanguages: manageData.AvailableLanguages,
		InstalledPage:            manageData.InstalledPage,
		InstalledTotalPages:      manageData.InstalledTotalPages,
		InstallablePage:          manageData.InstallablePage,
		InstallableTotalPages:    manageData.InstallableTotalPages,
		ManagePerPage:            manageData.PerPage,
	}

	if err := Templates.ExecuteTemplate(w, "bible_index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleBibleInstall handles POST requests to install a Bible (generate IR).
// This is now fully async - it queues the task and returns immediately.
func handleBibleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	id := r.FormValue("id")
	source := r.FormValue("source")
	sourcePath := r.FormValue("path")

	if id == "" || source == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Validate source type
	if source != "capsule" && source != "sword" {
		http.Error(w, "Unknown source type", http.StatusBadRequest)
		return
	}

	// Queue the install task asynchronously
	params := map[string]string{
		"id":     id,
		"source": source,
		"path":   sourcePath,
	}
	taskQueue.AddTask(TaskInstall, id, params)

	log.Printf("[INSTALL] Queued install task for %s", id)

	// Redirect back immediately - task runs in background
	http.Redirect(w, r, "/library/bibles/?tab=manage", http.StatusSeeOther)
}

// handleBibleDelete handles POST requests to delete a Bible (remove IR).
// This is now fully async - it queues the task and returns immediately.
func handleBibleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	id := r.FormValue("id")
	source := r.FormValue("source")

	if id == "" {
		http.Error(w, "Missing Bible ID", http.StatusBadRequest)
		return
	}

	// Validate source type - only capsules can be deleted
	if source != "capsule" {
		http.Error(w, "Cannot delete non-capsule Bibles", http.StatusBadRequest)
		return
	}

	// Queue the delete task asynchronously
	params := map[string]string{
		"id":     id,
		"source": source,
	}
	taskQueue.AddTask(TaskDelete, id, params)

	log.Printf("[DELETE] Queued delete task for %s", id)

	// Redirect back immediately - task runs in background
	http.Redirect(w, r, "/library/bibles/?tab=manage", http.StatusSeeOther)
}

// installCapsuleBible generates IR for a capsule Bible.
func installCapsuleBible(id, sourcePath string) error {
	// Use the direct IR generation function from handlers.go
	result := performIRGeneration(sourcePath)
	if !result.Success {
		return fmt.Errorf("IR generation failed: %s", result.Message)
	}

	log.Printf("[INSTALL] IR generation successful for %s", id)
	return nil
}

// installSWORDBible converts a SWORD module to a capsule with IR.
func installSWORDBible(id, confPath string) error {
	// Resolve SWORD directory from conf path (confPath is like ~/.sword/mods.d/kjv.conf)
	// We need the parent of mods.d, i.e., ~/.sword
	swordDir := filepath.Dir(filepath.Dir(confPath))

	// Step 1: Ingest SWORD module into a capsule
	result := ingestSWORDModule(swordDir, id)
	if !result.Success {
		return fmt.Errorf("SWORD ingest failed: %s", result.Error)
	}
	log.Printf("[INSTALL] SWORD module %s ingested as %s", id, result.CapsulePath)

	// Step 2: Generate IR for the new capsule
	// The capsule path is relative to CapsulesDir, so extract just the filename
	capsuleFilename := filepath.Base(result.CapsulePath)
	irResult := performIRGeneration(capsuleFilename)
	if !irResult.Success {
		// Capsule was created but IR generation failed - still a partial success
		log.Printf("[INSTALL] Warning: IR generation failed for %s: %s", id, irResult.Message)
		return fmt.Errorf("capsule created but IR generation failed: %s", irResult.Message)
	}

	log.Printf("[INSTALL] SWORD module %s installed successfully with IR", id)
	return nil
}

// deleteCapsuleBibleIR removes the IR file from a capsule.
func deleteCapsuleBibleIR(id string) error {
	// Find the capsule
	capsules := listCapsules()
	var capsulePath string
	for _, c := range capsules {
		capsuleID := archive.ExtractCapsuleID(c.Name)
		if strings.EqualFold(capsuleID, id) {
			capsulePath = filepath.Join(ServerConfig.CapsulesDir, c.Path)
			break
		}
	}

	if capsulePath == "" {
		return fmt.Errorf("capsule not found: %s", id)
	}

	// For now, return an error indicating this feature is not yet implemented
	// In a full implementation, this would:
	// 1. Open the capsule archive
	// 2. Remove the .ir.json file
	// 3. Rewrite the archive
	return fmt.Errorf("IR deletion not yet implemented - the capsule format requires archive rewrite")
}

// handleBibleRouting routes requests to the appropriate handler.
func handleBibleRouting(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/bible/")
	parts := strings.Split(strings.Trim(path, "/"), "/")

	switch {
	case len(parts) == 1 && parts[0] != "":
		// /bible/{capsule}
		handleBibleView(w, r, parts[0])
	case len(parts) == 2:
		// /bible/{capsule}/{book}
		handleBookView(w, r, parts[0], parts[1])
	case len(parts) >= 3:
		// /bible/{capsule}/{book}/{chapter}
		handleChapterView(w, r, parts[0], parts[1], parts[2])
	default:
		http.Redirect(w, r, "/bible", http.StatusFound)
	}
}

// handleBibleView shows a single Bible's books.
func handleBibleView(w http.ResponseWriter, r *http.Request, capsuleID string) {
	bible, books, err := loadBibleWithBooks(capsuleID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Bible not found: %v", err), http.StatusNotFound)
		return
	}

	data := BibleViewData{
		PageData: PageData{Title: bible.Title},
		Bible:    *bible,
		Books:    books,
	}

	if err := Templates.ExecuteTemplate(w, "bible_view.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleBookView shows a book's chapters.
func handleBookView(w http.ResponseWriter, r *http.Request, capsuleID, bookID string) {
	bible, books, err := loadBibleWithBooks(capsuleID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Bible not found: %v", err), http.StatusNotFound)
		return
	}

	var book *BookInfo
	for _, b := range books {
		if strings.EqualFold(b.ID, bookID) {
			book = &b
			break
		}
	}
	if book == nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	chapters := make([]ChapterInfo, book.ChapterCount)
	for i := 0; i < book.ChapterCount; i++ {
		chapters[i] = ChapterInfo{Number: i + 1}
	}

	data := BookViewData{
		PageData: PageData{Title: fmt.Sprintf("%s - %s", book.Name, bible.Title)},
		Bible:    *bible,
		Book:     *book,
		Chapters: chapters,
	}

	if err := Templates.ExecuteTemplate(w, "bible_book.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func parseChapterNum(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	if n < 1 {
		return 1
	}
	return n
}

func findBookByID(books []BookInfo, id string) *BookInfo {
	for i := range books {
		if strings.EqualFold(books[i].ID, id) {
			return &books[i]
		}
	}
	return nil
}

func resolveChapterBook(books []BookInfo, bookID string, requestedChapter int, bibleTitle string) (*BookInfo, int, string) {
	book := findBookByID(books, bookID)
	if book == nil {
		msg := fmt.Sprintf("The book \"%s\" does not exist in %s. Showing %s instead.", bookID, bibleTitle, books[0].Name)
		return &books[0], 1, msg
	}
	if requestedChapter > book.ChapterCount {
		msg := fmt.Sprintf("Chapter %d does not exist in %s (max: %d). Showing chapter %d instead.", requestedChapter, book.Name, book.ChapterCount, book.ChapterCount)
		return book, book.ChapterCount, msg
	}
	return book, requestedChapter, ""
}

func buildChapterNavURLs(capsuleID, bookID string, chapter, chapterCount int) (string, string) {
	var prevURL, nextURL string
	if chapter > 1 {
		prevURL = fmt.Sprintf("/bible/%s/%s/%d", capsuleID, bookID, chapter-1)
	}
	if chapter < chapterCount {
		nextURL = fmt.Sprintf("/bible/%s/%s/%d", capsuleID, bookID, chapter+1)
	}
	return prevURL, nextURL
}

func buildChapterList(count int) []int {
	chapters := make([]int, count)
	for i := range chapters {
		chapters[i] = i + 1
	}
	return chapters
}

func handleChapterView(w http.ResponseWriter, r *http.Request, capsuleID, bookID, chapterStr string) {
	requestedChapter := parseChapterNum(chapterStr)

	bible, books, err := loadBibleWithBooks(capsuleID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Bible not found: %v", err), http.StatusNotFound)
		return
	}

	book, chapter, notFoundMessage := resolveChapterBook(books, bookID, requestedChapter, bible.Title)

	verses, err := loadChapterVerses(capsuleID, book.ID, chapter)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load chapter: %v", err), http.StatusInternalServerError)
		return
	}

	prevURL, nextURL := buildChapterNavURLs(capsuleID, book.ID, chapter, book.ChapterCount)

	data := ChapterViewData{
		PageData:         PageData{Title: fmt.Sprintf("%s %d - %s", book.Name, chapter, bible.Title)},
		Bible:            *bible,
		Book:             *book,
		Chapter:          chapter,
		Verses:           verses,
		PrevURL:          prevURL,
		NextURL:          nextURL,
		AllBibles:        getCachedBibles(),
		AllBooks:         books,
		Chapters:         buildChapterList(book.ChapterCount),
		RequestedBook:    bookID,
		RequestedChapter: requestedChapter,
		NotFoundMessage:  notFoundMessage,
	}

	if err := Templates.ExecuteTemplate(w, "bible_chapter.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleBibleCompare redirects to /library/bibles/?tab=compare
func handleBibleCompare(w http.ResponseWriter, r *http.Request) {
	// Build redirect URL preserving query parameters
	redirectURL := "/library/bibles/?tab=compare"
	if ref := r.URL.Query().Get("ref"); ref != "" {
		redirectURL += "&ref=" + ref
	}
	if bibles := r.URL.Query().Get("bibles"); bibles != "" {
		redirectURL += "&bibles=" + bibles
	}
	http.Redirect(w, r, redirectURL, http.StatusMovedPermanently)
}

// handleBibleSearch shows the search page and handles search requests.
func handleBibleSearch(w http.ResponseWriter, r *http.Request) {
	bibles := getCachedBibles()

	query := r.URL.Query().Get("q")
	bibleID := r.URL.Query().Get("bible")

	var results []SearchResult
	var total int

	if query != "" && bibleID != "" {
		results, total = searchBible(bibleID, query, 100)
	}

	data := SearchData{
		PageData: PageData{Title: "Search Bible"},
		Bibles:   bibles,
		Query:    query,
		BibleID:  bibleID,
		Results:  results,
		Total:    total,
	}

	if err := Templates.ExecuteTemplate(w, "bible_search.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleAPIBibles returns JSON list of Bibles.
func handleAPIBibles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/api/bibles")
	path = strings.Trim(path, "/")
	if path == "" {
		json.NewEncoder(w).Encode(getCachedBibles())
		return
	}
	parts := strings.Split(path, "/")
	dispatchAPIBibles(w, parts)
}

// dispatchAPIBibles routes the API request based on path parts.
func dispatchAPIBibles(w http.ResponseWriter, parts []string) {
	switch len(parts) {
	case 1:
		handleAPIBibleInfo(w, parts[0])
	case 2:
		handleAPIBookInfo(w, parts[0], parts[1])
	default:
		handleAPIChapterVerses(w, parts)
	}
}

// handleAPIBibleInfo returns Bible info with books.
func handleAPIBibleInfo(w http.ResponseWriter, capsuleID string) {
	bible, books, err := loadBibleWithBooks(capsuleID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"bible": bible, "books": books})
}

// handleAPIBookInfo returns book info for a Bible.
func handleAPIBookInfo(w http.ResponseWriter, capsuleID, bookID string) {
	bible, books, err := loadBibleWithBooks(capsuleID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	for _, book := range books {
		if strings.EqualFold(book.ID, bookID) {
			json.NewEncoder(w).Encode(map[string]interface{}{"bible": bible, "book": book})
			return
		}
	}
	http.Error(w, "Book not found", http.StatusNotFound)
}

// handleAPIChapterVerses returns verses for a chapter.
func handleAPIChapterVerses(w http.ResponseWriter, parts []string) {
	capsuleID, bookID := parts[0], parts[1]
	var chapter int
	fmt.Sscanf(parts[2], "%d", &chapter)
	verses, err := loadChapterVerses(capsuleID, bookID, chapter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(ChapterData{BibleID: capsuleID, Book: bookID, Chapter: chapter, Verses: verses})
}

// handleAPIBibleSearch handles search API requests.
func handleAPIBibleSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query := r.URL.Query().Get("q")
	bibleID := r.URL.Query().Get("bible")
	limitStr := r.URL.Query().Get("limit")

	limit := 100
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	if query == "" || bibleID == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []SearchResult{},
			"total":   0,
			"error":   "query and bible parameters required",
		})
		return
	}

	results, total := searchBible(bibleID, query, limit)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
		"total":   total,
	})
}

// filterCapsulesWithIR returns only those capsules that have an IR file present.
// This avoids attempting to read IR from capsules that have none.
func filterCapsulesWithIR(capsules []CapsuleInfo) []CapsuleInfo {
	var withIR []CapsuleInfo
	for _, c := range capsules {
		fullPath := filepath.Join(ServerConfig.CapsulesDir, c.Path)
		if getCapsuleMetadata(fullPath).HasIR {
			withIR = append(withIR, c)
		}
	}
	return withIR
}

// loadCorpusForCapsule returns a *ir.Corpus for the given capsule, reading from
// the corpus cache when possible and falling back to the archive on a miss.
// Returns nil when the corpus cannot be loaded or is not a Bible module.
func loadCorpusForCapsule(c CapsuleInfo, capsuleID string) *ir.Corpus {
	corpusCache.RLock()
	entry, cached := corpusCache.corpora[capsuleID]
	corpusCache.RUnlock()

	if cached {
		return entry.corpus
	}

	irContent, err := readIRContent(filepath.Join(ServerConfig.CapsulesDir, c.Path))
	if err != nil {
		return nil
	}
	corpus := parseIRToCorpus(irContent)
	if corpus == nil || corpus.ModuleType != ir.ModuleBible {
		return nil
	}
	return corpus
}

// sampleDocIndices builds the set of document indices to probe for Strong's numbers.
// It samples up to 3 OT books (indices 0-2) and up to 3 NT books (indices 39-41).
func sampleDocIndices(numDocs int) []int {
	var indices []int
	for i := 0; i < 3 && i < numDocs; i++ {
		indices = append(indices, i)
	}
	for i := 39; i < 42 && i < numDocs; i++ {
		indices = append(indices, i)
	}
	return indices
}

// detectStrongsNumbers returns true when any sampled document in corpus contains
// at least one token with a Strong's number annotation.
func detectStrongsNumbers(corpus *ir.Corpus) bool {
	for _, idx := range sampleDocIndices(len(corpus.Documents)) {
		for _, cb := range corpus.Documents[idx].ContentBlocks {
			for _, tok := range cb.Tokens {
				if len(tok.Strongs) > 0 {
					return true
				}
			}
		}
	}
	return false
}

// processCapsuleForBibleInfo converts a single CapsuleInfo into a BibleInfo result.
// It is the worker function executed inside the listBiblesUncached pool.
func processCapsuleForBibleInfo(c CapsuleInfo) (BibleInfo, bool) {
	capsuleID := archive.ExtractCapsuleID(c.Name)

	corpus := loadCorpusForCapsule(c, capsuleID)
	if corpus == nil {
		return BibleInfo{}, false
	}

	bible := BibleInfo{
		ID:            capsuleID,
		Title:         corpus.Title,
		Abbrev:        corpus.ID,
		Language:      corpus.Language,
		Versification: corpus.Versification,
		BookCount:     len(corpus.Documents),
		CapsulePath:   c.Path,
	}

	if detectStrongsNumbers(corpus) {
		bible.Features = append(bible.Features, "Strong's Numbers")
	}

	return bible, true
}

// listBiblesUncached returns all Bible capsules without caching.
// Uses goroutines for parallel processing and leverages corpus cache when available.
func listBiblesUncached() []BibleInfo {
	capsules := listCapsules()
	if len(capsules) == 0 {
		return nil
	}

	capsulesWithIR := filterCapsulesWithIR(capsules)
	if len(capsulesWithIR) == 0 {
		return nil
	}

	type result struct {
		bible BibleInfo
		ok    bool
	}

	pool := NewWorkerPool[CapsuleInfo, result](maxWorkers, len(capsulesWithIR))
	pool.Start(func(c CapsuleInfo) result {
		bible, ok := processCapsuleForBibleInfo(c)
		return result{bible: bible, ok: ok}
	})

	for _, c := range capsulesWithIR {
		pool.Submit(c)
	}
	pool.Close()

	var bibles []BibleInfo
	for r := range pool.Results() {
		if r.ok {
			bibles = append(bibles, r.bible)
		}
	}

	sort.Slice(bibles, func(i, j int) bool {
		return bibles[i].Title < bibles[j].Title
	})

	return bibles
}

// loadBibleWithBooks loads a Bible and its books from a capsule.
// Uses corpus cache for better performance.
func loadBibleWithBooks(capsuleID string) (*BibleInfo, []BookInfo, error) {
	corpus, capsulePath, err := getCachedCorpus(capsuleID)
	if err != nil {
		return nil, nil, err
	}

	bible := &BibleInfo{
		ID:            capsuleID,
		Title:         corpus.Title,
		Abbrev:        corpus.ID,
		Language:      corpus.Language,
		Versification: corpus.Versification,
		BookCount:     len(corpus.Documents),
		CapsulePath:   capsulePath,
	}

	var books []BookInfo
	for _, doc := range corpus.Documents {
		testament := "OT"
		if isNewTestament(doc.ID) {
			testament = "NT"
		}

		chapterCount := countChapters(doc)

		books = append(books, BookInfo{
			ID:           doc.ID,
			Name:         doc.Title,
			Order:        doc.Order,
			ChapterCount: chapterCount,
			Testament:    testament,
		})
	}

	sort.Slice(books, func(i, j int) bool {
		return books[i].Order < books[j].Order
	})

	return bible, books, nil
}

// loadChapterVerses loads verses for a specific chapter.
// Uses corpus cache for better performance.
func loadChapterVerses(capsuleID, bookID string, chapter int) ([]VerseData, error) {
	corpus, _, err := getCachedCorpus(capsuleID)
	if err != nil {
		return nil, err
	}

	doc := findBookDocument(corpus, bookID)
	if doc == nil {
		return nil, fmt.Errorf("book not found: %s", bookID)
	}

	verses := extractChapterVerses(doc, chapter)
	sort.Slice(verses, func(i, j int) bool {
		return verses[i].Number < verses[j].Number
	})
	return verses, nil
}

func findBookDocument(corpus *ir.Corpus, bookID string) *ir.Document {
	for _, d := range corpus.Documents {
		if strings.EqualFold(d.ID, bookID) {
			return d
		}
	}
	return nil
}

func extractChapterVerses(doc *ir.Document, chapter int) []VerseData {
	var verses []VerseData
	docPrefix := doc.ID + "."

	for _, cb := range doc.ContentBlocks {
		if verse := parseChapterVerse(cb, docPrefix, chapter); verse != nil {
			verses = append(verses, *verse)
		}
	}
	return verses
}

func parseChapterVerse(cb *ir.ContentBlock, docPrefix string, targetChapter int) *VerseData {
	if !strings.HasPrefix(cb.ID, docPrefix) {
		return nil
	}
	suffix := cb.ID[len(docPrefix):]
	matches := chapterVerseRegex.FindStringSubmatch("." + suffix)
	if len(matches) != 3 {
		return nil
	}
	var cbChapter, cbVerse int
	fmt.Sscanf(matches[1], "%d", &cbChapter)
	fmt.Sscanf(matches[2], "%d", &cbVerse)

	if cbChapter != targetChapter {
		return nil
	}
	return &VerseData{Number: cbVerse, Text: cb.Text}
}

func normalizeQuery(query string) (string, string, bool) {
	isPhrase := strings.HasPrefix(query, "\"") && strings.HasSuffix(query, "\"")
	if isPhrase {
		query = strings.Trim(query, "\"")
	}
	isStrongs := strongsSearchRegex.MatchString(strings.ToUpper(query))
	return query, strings.ToLower(query), isStrongs
}

func matchesStrongsQuery(cb *ir.ContentBlock, query string) bool {
	for _, tok := range cb.Tokens {
		for _, s := range tok.Strongs {
			if strings.EqualFold(s, query) {
				return true
			}
		}
	}
	return false
}

func contentBlockMatches(cb *ir.ContentBlock, query, queryLower string, isStrongs bool) bool {
	if isStrongs {
		return matchesStrongsQuery(cb, query)
	}
	return strings.Contains(strings.ToLower(cb.Text), queryLower)
}

// searchBible searches for text in a Bible.
// Uses corpus cache for better performance.
func searchBible(bibleID, query string, limit int) ([]SearchResult, int) {
	corpus, _, err := getCachedCorpus(bibleID)
	if err != nil {
		return nil, 0
	}

	query, queryLower, isStrongs := normalizeQuery(query)
	var results []SearchResult
	total := 0

	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			if !contentBlockMatches(cb, query, queryLower, isStrongs) {
				continue
			}
			total++
			if len(results) < limit {
				chapter, verse := parseContentBlockRef(cb.ID, doc.ID)
				results = append(results, SearchResult{
					BibleID:   bibleID,
					Reference: fmt.Sprintf("%s %d:%d", doc.Title, chapter, verse),
					Book:      doc.ID,
					Chapter:   chapter,
					Verse:     verse,
					Text:      cb.Text,
				})
			}
		}
	}

	return results, total
}

// parseIRToCorpus converts raw IR JSON to a Corpus.
func parseIRToCorpus(irContent map[string]interface{}) *ir.Corpus {
	data, err := json.Marshal(irContent)
	if err != nil {
		return nil
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return nil
	}

	return &corpus
}

// countChapters counts the number of chapters in a document.
func countChapters(doc *ir.Document) int {
	chapters := make(map[int]bool)

	for _, cb := range doc.ContentBlocks {
		matches := chapterVerseRegex.FindStringSubmatch(cb.ID)
		if len(matches) >= 2 {
			var ch int
			fmt.Sscanf(matches[1], "%d", &ch)
			chapters[ch] = true
		}
	}

	return len(chapters)
}

// isNewTestament checks if a book ID is from the New Testament.
func isNewTestament(bookID string) bool {
	ntBooks := map[string]bool{
		"Matt": true, "Mark": true, "Luke": true, "John": true,
		"Acts": true, "Rom": true, "1Cor": true, "2Cor": true,
		"Gal": true, "Eph": true, "Phil": true, "Col": true,
		"1Thess": true, "2Thess": true, "1Tim": true, "2Tim": true,
		"Titus": true, "Phlm": true, "Heb": true, "Jas": true,
		"1Pet": true, "2Pet": true, "1John": true, "2John": true,
		"3John": true, "Jude": true, "Rev": true,
	}
	return ntBooks[bookID]
}

// parseContentBlockRef parses chapter and verse from a content block ID.
// Uses pre-compiled regex and string prefix matching for better performance.
func parseContentBlockRef(cbID, bookID string) (int, int) {
	// Fast path: check if cbID starts with bookID
	if !strings.HasPrefix(cbID, bookID) {
		return 1, 1
	}
	// Use pre-compiled regex on the suffix after bookID
	suffix := cbID[len(bookID):]
	matches := chapterVerseRegex.FindStringSubmatch(suffix)
	if len(matches) >= 3 {
		var chapter, verse int
		fmt.Sscanf(matches[1], "%d", &chapter)
		fmt.Sscanf(matches[2], "%d", &verse)
		return chapter, verse
	}
	return 1, 1
}
