package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

// Pre-compiled regexes for performance (avoid recompilation on every request)
var (
	// chapterVerseRegex matches patterns like "BookID.chapter.verse" (e.g., "Gen.1.1")
	chapterVerseRegex = regexp.MustCompile(`\.(\d+)\.(\d+)`)
	// strongsSearchRegex matches Strong's number format (H1234 or G5678)
	strongsSearchRegex = regexp.MustCompile(`^[HG]\d+$`)
)

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
	bibleCache.ttl = 5 * time.Minute // Cache for 5 minutes
	corpusCache.corpora = make(map[string]*corpusCacheEntry)
	corpusCache.ttl = 10 * time.Minute // Cache corpora longer since they're expensive to load
	manageableBiblesCache.ttl = 5 * time.Minute // Cache manageable bibles list
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
		id := strings.TrimSuffix(c.Name, ".capsule.tar.xz")
		id = strings.TrimSuffix(id, ".tar.xz")
		id = strings.TrimSuffix(id, ".tar.gz")
		id = strings.TrimSuffix(id, ".tar")
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
func PreWarmCaches() {
	go func() {
		log.Println("[CACHE] Pre-warming Bible cache...")
		getCachedBibles()
		log.Println("[CACHE] Pre-warm complete")
	}()
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
func listManageableBiblesUncached() (installed, installable []ManageableBible) {
	capsules := listCapsules()

	// Build a set of installed Bible IDs for quick lookup
	installedIDs := make(map[string]bool)
	for _, bible := range getCachedBibles() {
		installedIDs[bible.ID] = true
	}

	// Process capsules using cached metadata for performance
	for _, c := range capsules {
		capsuleID := strings.TrimSuffix(c.Name, ".capsule.tar.xz")
		capsuleID = strings.TrimSuffix(capsuleID, ".tar.xz")
		capsuleID = strings.TrimSuffix(capsuleID, ".tar.gz")
		capsuleID = strings.TrimSuffix(capsuleID, ".tar")

		fullPath := filepath.Join(ServerConfig.CapsulesDir, c.Path)
		// Use getCapsuleMetadata for cached HasIR check (avoids re-reading archive)
		meta := getCapsuleMetadata(fullPath)
		hasIR := meta.HasIR

		// Build tags for this capsule
		tags := []string{"capsule"}
		if c.Format != "" {
			tags = append(tags, c.Format)
		}
		if hasIR {
			tags = append(tags, "identified")
		}

		mb := ManageableBible{
			ID:          capsuleID,
			Name:        capsuleID,
			Source:      "capsule",
			SourcePath:  c.Path,
			IsInstalled: hasIR,
			Format:      c.Format,
			Tags:        tags,
		}

		if hasIR {
			installed = append(installed, mb)
		} else {
			installable = append(installable, mb)
		}
	}

	// List SWORD modules from ~/.sword/
	swordModules := listSWORDModules()
	for _, sm := range swordModules {
		// Check if already in installed list (by ID)
		if installedIDs[sm.ID] {
			continue
		}
		installable = append(installable, sm)
	}

	// Sort both lists by name
	sort.Slice(installed, func(i, j int) bool {
		return installed[i].Name < installed[j].Name
	})
	sort.Slice(installable, func(i, j int) bool {
		return installable[i].Name < installable[j].Name
	})

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

// listSWORDModulesUncached finds SWORD Bible modules in ~/.sword/ using parallel processing.
func listSWORDModulesUncached() []ManageableBible {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	swordDir := filepath.Join(homeDir, ".sword")
	modsDir := filepath.Join(swordDir, "mods.d")

	// Check if mods.d directory exists
	if _, err := os.Stat(modsDir); os.IsNotExist(err) {
		return nil
	}

	// Read .conf files from mods.d
	files, err := os.ReadDir(modsDir)
	if err != nil {
		return nil
	}

	// Collect conf files to process
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

	if len(confFiles) == 0 {
		return nil
	}

	// Process conf files in parallel using worker pool
	type result struct {
		module *ManageableBible
	}

	pool := NewWorkerPool[swordConfFile, result](32, len(confFiles))
	pool.Start(func(cf swordConfFile) result {
		confContent, err := os.ReadFile(cf.path)
		if err != nil {
			return result{module: nil}
		}

		// Use existing parseSWORDConf from handlers.go
		module := parseSWORDConf(string(confContent), cf.name)

		// Check if it's a Bible module by looking at the Category field
		// SWORD modules with Category=Biblical Texts are Bibles
		if module.ID == "" || module.Category != "Biblical Texts" {
			return result{module: nil}
		}

		name := module.Description
		if name == "" {
			name = module.ID
		}
		// SWORD modules are always "identified" (properly parsed conf file)
		tags := []string{"sword", "identified"}

		// Add versification-based tags
		switch strings.ToLower(module.Versification) {
		case "catholic", "catholic2", "vulg", "lxx":
			tags = append(tags, "catholic")
		case "kjv", "nrsv", "luther", "german", "leningrad":
			tags = append(tags, "protestant")
		case "orthodox", "synodal", "synodalprot":
			tags = append(tags, "orthodox")
		default:
			// If no versification specified, default to protestant (most common)
			if module.Versification == "" {
				tags = append(tags, "protestant")
			}
		}

		// Add feature-based tags
		for _, feature := range module.Features {
			switch strings.ToLower(feature) {
			case "strongsnumbers":
				tags = append(tags, "strongs")
			case "images":
				tags = append(tags, "images")
			case "greekdef", "hebrewdef":
				tags = append(tags, "definitions")
			case "greekparse":
				tags = append(tags, "parsing")
			case "morphology":
				tags = append(tags, "morphology")
			case "footnotes":
				tags = append(tags, "footnotes")
			case "headings":
				tags = append(tags, "headings")
			}
		}

		return result{module: &ManageableBible{
			ID:          module.ID,
			Name:        name,
			Source:      "sword",
			SourcePath:  cf.path,
			IsInstalled: false,
			Format:      "sword",
			Tags:        tags,
			Language:    module.Language,
		}}
	})

	// Submit jobs
	for _, cf := range confFiles {
		pool.Submit(cf)
	}
	pool.Close()

	// Collect results
	var modules []ManageableBible
	for r := range pool.Results() {
		if r.module != nil {
			modules = append(modules, *r.module)
		}
	}

	// Sort by name
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
	InstalledBibles       []ManageableBible
	InstallableBibles     []ManageableBible
	AllInstalledBibles    []ManageableBible // Unfiltered for counts
	AllInstallableBibles  []ManageableBible // Unfiltered for counts
	ManageTagFilter        string   // Selected tag filter (e.g., "sword", "capsule", "tar.gz", "identified")
	ManageAvailableTags    []string // All unique tags available for filtering
	ManageLanguageFilter   string   // Selected language filter
	ManageAvailableLanguages []string // All unique languages available for filtering
	InstalledPage         int
	InstalledTotalPages   int
	InstallablePage       int
	InstallableTotalPages int
	ManagePerPage         int
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
		// Route to specific Bible
		handleBibleRouting(w, r)
		return
	}

	allBibles := getCachedBibles()

	// If no capsules, show empty state
	if len(allBibles) == 0 {
		data := BibleIndexData{
			PageData: PageData{Title: "Bible"},
			Bibles:   nil,
		}
		if err := Templates.ExecuteTemplate(w, "bible_empty.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Look for DRC capsule (case-insensitive)
	for _, b := range allBibles {
		if strings.EqualFold(b.ID, "DRC") {
			// Redirect to DRC Genesis 1
			http.Redirect(w, r, "/bible/DRC/Gen/1", http.StatusFound)
			return
		}
	}

	// No DRC found, redirect to first Bible's first book
	// Get the first Bible's books to find the first chapter
	if len(allBibles) > 0 {
		bible, books, err := loadBibleWithBooks(allBibles[0].ID)
		if err == nil && len(books) > 0 {
			http.Redirect(w, r, fmt.Sprintf("/bible/%s/%s/1", bible.ID, books[0].ID), http.StatusFound)
			return
		}
	}

	// Fallback: redirect to library
	http.Redirect(w, r, "/library/bibles/", http.StatusFound)
}

// handleLibraryBibles shows the Bible browsing/search/compare interface.
func handleLibraryBibles(w http.ResponseWriter, r *http.Request) {
	allBibles := getCachedBibles()

	// Collect unique languages and features
	langMap := make(map[string]bool)
	featMap := make(map[string]bool)
	for _, b := range allBibles {
		if b.Language != "" {
			langMap[b.Language] = true
		}
		for _, f := range b.Features {
			featMap[f] = true
		}
	}

	var languages, features []string
	for l := range langMap {
		languages = append(languages, l)
	}
	for f := range featMap {
		features = append(features, f)
	}
	sort.Strings(languages)
	sort.Strings(features)

	// Handle tab parameter
	tab := r.URL.Query().Get("tab")

	// Handle search if query parameter is present
	query := r.URL.Query().Get("q")
	bibleID := r.URL.Query().Get("bible")
	caseSensitive := r.URL.Query().Get("case") == "1"
	wholeWord := r.URL.Query().Get("word") == "1"

	var results []SearchResult
	var total int

	if query != "" {
		if bibleID != "" {
			// Search specific Bible
			results, total = searchBible(bibleID, query, 100)
		} else {
			// Search all Bibles
			for _, b := range allBibles {
				r, t := searchBible(b.ID, query, 100-len(results))
				results = append(results, r...)
				total += t
				if len(results) >= 100 {
					break
				}
			}
		}
	}

	// Pagination (only for browse tab, not when searching)
	perPageOptions := []int{11, 22, 33, 44, 55, 66}
	perPage := 11 // default
	page := 1

	if perPageStr := r.URL.Query().Get("perPage"); perPageStr != "" {
		fmt.Sscanf(perPageStr, "%d", &perPage)
		// Validate perPage is one of the allowed options
		valid := false
		for _, opt := range perPageOptions {
			if perPage == opt {
				valid = true
				break
			}
		}
		if !valid {
			perPage = 11
		}
	}

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		fmt.Sscanf(pageStr, "%d", &page)
		if page < 1 {
			page = 1
		}
	}

	// Calculate pagination for browse tab (not search)
	var paginatedBibles []BibleInfo
	totalPages := 1
	if query == "" && tab != "compare" {
		totalItems := len(allBibles)
		totalPages = (totalItems + perPage - 1) / perPage
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
		if start < totalItems {
			paginatedBibles = allBibles[start:end]
		}
	} else {
		paginatedBibles = allBibles
	}

	// Populate manage tab data only when needed
	var installedBibles, installableBibles []ManageableBible
	var allInstalledBibles, allInstallableBibles []ManageableBible
	var manageTagFilter string
	var manageAvailableTags []string
	var installedPage, installedTotalPages, installablePage, installableTotalPages int
	managePerPage := 10

	var manageLanguageFilter string
	var manageAvailableLanguages []string

	if tab == "manage" {
		allInstalledBibles, allInstallableBibles = getCachedManageableBibles()

		// Collect all unique tags and languages from both lists
		tagSet := make(map[string]bool)
		langSet := make(map[string]bool)
		for _, b := range allInstalledBibles {
			for _, tag := range b.Tags {
				tagSet[tag] = true
			}
			if b.Language != "" {
				langSet[b.Language] = true
			}
		}
		for _, b := range allInstallableBibles {
			for _, tag := range b.Tags {
				tagSet[tag] = true
			}
			if b.Language != "" {
				langSet[b.Language] = true
			}
		}
		for tag := range tagSet {
			manageAvailableTags = append(manageAvailableTags, tag)
		}
		sort.Strings(manageAvailableTags)
		for lang := range langSet {
			manageAvailableLanguages = append(manageAvailableLanguages, lang)
		}
		sort.Strings(manageAvailableLanguages)

		// Get tag filter parameter
		manageTagFilter = r.URL.Query().Get("tag")
		if manageTagFilter == "" {
			manageTagFilter = "all"
		}

		// Get language filter parameter
		manageLanguageFilter = r.URL.Query().Get("lang")
		if manageLanguageFilter == "" {
			manageLanguageFilter = "all"
		}

		// Filter by tag and language
		var filteredInstalled, filteredInstallable []ManageableBible
		for _, b := range allInstalledBibles {
			tagMatch := manageTagFilter == "all" || b.HasTag(manageTagFilter)
			langMatch := manageLanguageFilter == "all" || b.Language == manageLanguageFilter
			if tagMatch && langMatch {
				filteredInstalled = append(filteredInstalled, b)
			}
		}
		for _, b := range allInstallableBibles {
			tagMatch := manageTagFilter == "all" || b.HasTag(manageTagFilter)
			langMatch := manageLanguageFilter == "all" || b.Language == manageLanguageFilter
			if tagMatch && langMatch {
				filteredInstallable = append(filteredInstallable, b)
			}
		}

		// Pagination for installed
		installedPage = 1
		if ipStr := r.URL.Query().Get("ipage"); ipStr != "" {
			fmt.Sscanf(ipStr, "%d", &installedPage)
			if installedPage < 1 {
				installedPage = 1
			}
		}
		installedTotalPages = (len(filteredInstalled) + managePerPage - 1) / managePerPage
		if installedTotalPages < 1 {
			installedTotalPages = 1
		}
		if installedPage > installedTotalPages {
			installedPage = installedTotalPages
		}
		iStart := (installedPage - 1) * managePerPage
		iEnd := iStart + managePerPage
		if iEnd > len(filteredInstalled) {
			iEnd = len(filteredInstalled)
		}
		if iStart < len(filteredInstalled) {
			installedBibles = filteredInstalled[iStart:iEnd]
		}

		// Pagination for installable
		installablePage = 1
		if upStr := r.URL.Query().Get("upage"); upStr != "" {
			fmt.Sscanf(upStr, "%d", &installablePage)
			if installablePage < 1 {
				installablePage = 1
			}
		}
		installableTotalPages = (len(filteredInstallable) + managePerPage - 1) / managePerPage
		if installableTotalPages < 1 {
			installableTotalPages = 1
		}
		if installablePage > installableTotalPages {
			installablePage = installableTotalPages
		}
		uStart := (installablePage - 1) * managePerPage
		uEnd := uStart + managePerPage
		if uEnd > len(filteredInstallable) {
			uEnd = len(filteredInstallable)
		}
		if uStart < len(filteredInstallable) {
			installableBibles = filteredInstallable[uStart:uEnd]
		}
	}

	data := BibleIndexData{
		PageData:              PageData{Title: "Bible"},
		Bibles:                paginatedBibles,
		AllBibles:             allBibles,
		Languages:             languages,
		Features:              features,
		Tab:                   tab,
		Query:                 query,
		BibleID:               bibleID,
		CaseSensitive:         caseSensitive,
		WholeWord:             wholeWord,
		Results:               results,
		Total:                 total,
		Page:                  page,
		PerPage:               perPage,
		TotalPages:            totalPages,
		PerPageOptions:        perPageOptions,
		InstalledBibles:       installedBibles,
		InstallableBibles:     installableBibles,
		AllInstalledBibles:    allInstalledBibles,
		AllInstallableBibles:  allInstallableBibles,
		ManageTagFilter:          manageTagFilter,
		ManageAvailableTags:      manageAvailableTags,
		ManageLanguageFilter:     manageLanguageFilter,
		ManageAvailableLanguages: manageAvailableLanguages,
		InstalledPage:            installedPage,
		InstalledTotalPages:   installedTotalPages,
		InstallablePage:       installablePage,
		InstallableTotalPages: installableTotalPages,
		ManagePerPage:         managePerPage,
	}

	if err := Templates.ExecuteTemplate(w, "bible_index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleBibleInstall handles POST requests to install a Bible (generate IR).
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

	var err error
	switch source {
	case "capsule":
		err = installCapsuleBible(id, sourcePath)
	case "sword":
		err = installSWORDBible(id, sourcePath)
	default:
		http.Error(w, "Unknown source type", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("[INSTALL] Failed to install %s: %v", id, err)
		http.Redirect(w, r, "/library/bibles/?tab=manage&error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	// Invalidate caches
	invalidateBibleCache()
	invalidateCorpusCache()
	invalidateManageableBiblesCache()

	log.Printf("[INSTALL] Successfully installed %s", id)
	http.Redirect(w, r, "/library/bibles/?tab=manage", http.StatusSeeOther)
}

// handleBibleDelete handles POST requests to delete a Bible (remove IR).
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

	var err error
	switch source {
	case "capsule":
		err = deleteCapsuleBibleIR(id)
	default:
		http.Error(w, "Cannot delete non-capsule Bibles", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("[DELETE] Failed to delete %s: %v", id, err)
		http.Redirect(w, r, "/library/bibles/?tab=manage&error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	// Invalidate caches
	invalidateBibleCache()
	invalidateCorpusCache()
	invalidateManageableBiblesCache()

	log.Printf("[DELETE] Successfully deleted IR for %s", id)
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
		capsuleID := strings.TrimSuffix(c.Name, ".capsule.tar.xz")
		capsuleID = strings.TrimSuffix(capsuleID, ".tar.xz")
		capsuleID = strings.TrimSuffix(capsuleID, ".tar.gz")
		capsuleID = strings.TrimSuffix(capsuleID, ".tar")
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

// handleChapterView shows a chapter's verses.
func handleChapterView(w http.ResponseWriter, r *http.Request, capsuleID, bookID, chapterStr string) {
	var requestedChapter int
	fmt.Sscanf(chapterStr, "%d", &requestedChapter)
	if requestedChapter < 1 {
		requestedChapter = 1
	}

	bible, books, err := loadBibleWithBooks(capsuleID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Bible not found: %v", err), http.StatusNotFound)
		return
	}

	// Get all bibles for dropdown
	allBibles := getCachedBibles()

	// Find the requested book
	var book *BookInfo
	for _, b := range books {
		if strings.EqualFold(b.ID, bookID) {
			book = &b
			break
		}
	}

	var notFoundMessage string
	chapter := requestedChapter

	// Handle book not found - show first book with message
	if book == nil {
		notFoundMessage = fmt.Sprintf("The book \"%s\" does not exist in %s. Showing %s instead.", bookID, bible.Title, books[0].Name)
		book = &books[0]
		chapter = 1
	} else if chapter > book.ChapterCount {
		// Handle chapter out of range
		notFoundMessage = fmt.Sprintf("Chapter %d does not exist in %s (max: %d). Showing chapter %d instead.", requestedChapter, book.Name, book.ChapterCount, book.ChapterCount)
		chapter = book.ChapterCount
	}

	verses, err := loadChapterVerses(capsuleID, book.ID, chapter)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load chapter: %v", err), http.StatusInternalServerError)
		return
	}

	// Build prev/next URLs
	var prevURL, nextURL string
	if chapter > 1 {
		prevURL = fmt.Sprintf("/bible/%s/%s/%d", capsuleID, book.ID, chapter-1)
	}
	if chapter < book.ChapterCount {
		nextURL = fmt.Sprintf("/bible/%s/%s/%d", capsuleID, book.ID, chapter+1)
	}

	// Build chapter list
	chapters := make([]int, book.ChapterCount)
	for i := 0; i < book.ChapterCount; i++ {
		chapters[i] = i + 1
	}

	data := ChapterViewData{
		PageData:         PageData{Title: fmt.Sprintf("%s %d - %s", book.Name, chapter, bible.Title)},
		Bible:            *bible,
		Book:             *book,
		Chapter:          chapter,
		Verses:           verses,
		PrevURL:          prevURL,
		NextURL:          nextURL,
		AllBibles:        allBibles,
		AllBooks:         books,
		Chapters:         chapters,
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
		// List all Bibles
		bibles := getCachedBibles()
		json.NewEncoder(w).Encode(bibles)
		return
	}

	parts := strings.Split(path, "/")
	capsuleID := parts[0]

	switch {
	case len(parts) == 1:
		// Get Bible info with books
		bible, books, err := loadBibleWithBooks(capsuleID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"bible": bible,
			"books": books,
		})
	case len(parts) == 2:
		// Get book info
		bookID := parts[1]
		bible, books, err := loadBibleWithBooks(capsuleID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		for _, book := range books {
			if strings.EqualFold(book.ID, bookID) {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"bible": bible,
					"book":  book,
				})
				return
			}
		}
		http.Error(w, "Book not found", http.StatusNotFound)
	case len(parts) >= 3:
		// Get chapter verses
		bookID := parts[1]
		var chapter int
		fmt.Sscanf(parts[2], "%d", &chapter)

		verses, err := loadChapterVerses(capsuleID, bookID, chapter)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(ChapterData{
			BibleID: capsuleID,
			Book:    bookID,
			Chapter: chapter,
			Verses:  verses,
		})
	}
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

// listBiblesUncached returns all Bible capsules without caching.
// Uses goroutines for parallel processing.
func listBiblesUncached() []BibleInfo {
	capsules := listCapsules()
	if len(capsules) == 0 {
		return nil
	}

	type result struct {
		bible BibleInfo
		ok    bool
	}

	// Create and start worker pool
	pool := NewWorkerPool[CapsuleInfo, result](maxWorkers, len(capsules))
	pool.Start(func(c CapsuleInfo) result {
		irContent, err := readIRContent(filepath.Join(ServerConfig.CapsulesDir, c.Path))
		if err != nil {
			return result{ok: false}
		}

		corpus := parseIRToCorpus(irContent)
		if corpus == nil || corpus.ModuleType != ir.ModuleBible {
			return result{ok: false}
		}

		capsuleID := strings.TrimSuffix(c.Name, ".capsule.tar.xz")
		capsuleID = strings.TrimSuffix(capsuleID, ".tar.xz")
		capsuleID = strings.TrimSuffix(capsuleID, ".tar.gz")
		capsuleID = strings.TrimSuffix(capsuleID, ".tar")

		bible := BibleInfo{
			ID:            capsuleID,
			Title:         corpus.Title,
			Abbrev:        corpus.ID,
			Language:      corpus.Language,
			Versification: corpus.Versification,
			BookCount:     len(corpus.Documents),
			CapsulePath:   c.Path,
		}

		// Check for Strong's numbers
		for _, doc := range corpus.Documents {
			for _, cb := range doc.ContentBlocks {
				for _, tok := range cb.Tokens {
					if len(tok.Strongs) > 0 {
						bible.Features = append(bible.Features, "Strong's Numbers")
						break
					}
				}
				if len(bible.Features) > 0 {
					break
				}
			}
			if len(bible.Features) > 0 {
				break
			}
		}

		return result{bible: bible, ok: true}
	})

	// Submit jobs
	for _, c := range capsules {
		pool.Submit(c)
	}
	pool.Close()

	// Collect results
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

	// Find the book
	var doc *ir.Document
	for _, d := range corpus.Documents {
		if strings.EqualFold(d.ID, bookID) {
			doc = d
			break
		}
	}
	if doc == nil {
		return nil, fmt.Errorf("book not found: %s", bookID)
	}

	// Extract verses for the chapter
	var verses []VerseData
	verseRe := regexp.MustCompile(`^` + regexp.QuoteMeta(doc.ID) + `\.(\d+)\.(\d+)`)

	for _, cb := range doc.ContentBlocks {
		// Try to parse verse reference from content block ID
		matches := verseRe.FindStringSubmatch(cb.ID)
		if len(matches) == 3 {
			var cbChapter, cbVerse int
			fmt.Sscanf(matches[1], "%d", &cbChapter)
			fmt.Sscanf(matches[2], "%d", &cbVerse)

			if cbChapter == chapter {
				verses = append(verses, VerseData{
					Number: cbVerse,
					Text:   cb.Text,
				})
			}
		}
	}

	// Sort by verse number
	sort.Slice(verses, func(i, j int) bool {
		return verses[i].Number < verses[j].Number
	})

	return verses, nil
}

// searchBible searches for text in a Bible.
// Uses corpus cache for better performance.
func searchBible(bibleID, query string, limit int) ([]SearchResult, int) {
	corpus, _, err := getCachedCorpus(bibleID)
	if err != nil {
		return nil, 0
	}

	var results []SearchResult
	total := 0
	queryLower := strings.ToLower(query)

	// Check if it's a phrase search (quoted)
	isPhrase := strings.HasPrefix(query, "\"") && strings.HasSuffix(query, "\"")
	if isPhrase {
		query = strings.Trim(query, "\"")
		queryLower = strings.ToLower(query)
	}

	// Check if it's a Strong's number search (use pre-compiled regex)
	isStrongs := strongsSearchRegex.MatchString(strings.ToUpper(query))

	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			var matched bool

			if isStrongs {
				// Search for Strong's number in tokens
				for _, tok := range cb.Tokens {
					for _, s := range tok.Strongs {
						if strings.EqualFold(s, query) {
							matched = true
							break
						}
					}
					if matched {
						break
					}
				}
			} else if isPhrase {
				// Exact phrase search
				matched = strings.Contains(strings.ToLower(cb.Text), queryLower)
			} else {
				// Word search
				matched = strings.Contains(strings.ToLower(cb.Text), queryLower)
			}

			if matched {
				total++
				if len(results) < limit {
					// Parse reference from content block ID
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
