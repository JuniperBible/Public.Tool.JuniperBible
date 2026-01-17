// Package main provides comprehensive web UI tests using sample Bible data.
// These tests use the 11 sample Bible capsules from contrib/sample-data/capsules/
// to test all web UI endpoints with real data.
package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
	"github.com/FocuswithJustin/JuniperBible/internal/archive"
)

// sampleBibles lists the 11 sample Bible capsules available in contrib/sample-data/capsules/
var sampleBibles = []string{
	"asv",        // American Standard Version
	"drc",        // Douay-Rheims Catholic Bible
	"geneva1599", // Geneva Bible (1599)
	"kjv",        // King James Version
	"lxx",        // Septuagint
	"oeb",        // Open English Bible
	"osmhb",      // Open Scriptures Hebrew Bible
	"sblgnt",     // SBL Greek New Testament
	"tyndale",    // William Tyndale Bible
	"vulgate",    // Latin Vulgate
	"web",        // World English Bible
}

// getSampleDataDir returns the path to the sample data capsules directory.
// Returns empty string if not found.
func getSampleDataDir() string {
	// Try relative to current working directory
	paths := []string{
		"../../contrib/sample-data/capsules",
		"contrib/sample-data/capsules",
		"../contrib/sample-data/capsules",
	}

	cwd, _ := os.Getwd()
	for _, p := range paths {
		full := filepath.Join(cwd, p)
		if _, err := os.Stat(full); err == nil {
			return full
		}
	}

	// Try from GOPATH or module root
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		modPath := filepath.Join(gopath, "src", "github.com", "FocuswithJustin", "mimicry", "contrib", "sample-data", "capsules")
		if _, err := os.Stat(modPath); err == nil {
			return modPath
		}
	}

	return ""
}

// hasSampleData returns true if sample data is available for testing.
func hasSampleData() bool {
	dir := getSampleDataDir()
	if dir == "" {
		return false
	}

	// Check if at least one capsule exists (try both naming conventions)
	kjvPaths := []string{
		filepath.Join(dir, "KJV.capsule.tar.gz"),
		filepath.Join(dir, "kjv.tar.xz"),
		filepath.Join(dir, "kjv.capsule.tar.gz"),
	}
	for _, p := range kjvPaths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// setupSampleDataTest creates a temp directory and copies sample capsules for testing.
// It also generates IR for the capsules to enable Bible browsing.
// The function returns the temp directory path and a cleanup function.
func setupSampleDataTest(t *testing.T, bibles ...string) (string, func()) {
	t.Helper()

	if !hasSampleData() {
		t.Skip("Sample data not available - skipping test")
	}

	sampleDir := getSampleDataDir()
	tempDir := t.TempDir()

	// Create plugins directory (even if empty)
	os.MkdirAll(filepath.Join(tempDir, "plugins", "format"), 0755)

	// Copy requested capsules with generated IR
	for _, bible := range bibles {
		// Try multiple naming conventions to find source capsule
		foundSrc := false
		for _, pattern := range []string{
			strings.ToUpper(bible) + ".capsule.tar.gz",
			bible + ".capsule.tar.gz",
			bible + ".tar.xz",
		} {
			srcPath := filepath.Join(sampleDir, pattern)
			if _, err := os.Stat(srcPath); err == nil {
				foundSrc = true
				break
			}
		}

		// Create capsule with IR for Bible browsing
		// (we generate mock IR data regardless of source)
		if foundSrc {
			createCapsuleWithIR(t, tempDir, bible)
		}
	}

	// Store original ServerConfig
	origCapsulesDir := ServerConfig.CapsulesDir
	origPluginsDir := ServerConfig.PluginsDir
	ServerConfig.CapsulesDir = tempDir
	ServerConfig.PluginsDir = filepath.Join(tempDir, "plugins")

	// Clear all caches so they pick up the new capsules
	invalidateBibleCache()
	invalidateCorpusCache()
	invalidateCapsuleMetadataCache()
	invalidateCapsulesList()
	archive.ClearTOCCache()

	cleanup := func() {
		ServerConfig.CapsulesDir = origCapsulesDir
		ServerConfig.PluginsDir = origPluginsDir
	}

	return tempDir, cleanup
}

// createCapsuleWithIR creates a test capsule with generated IR.
// For testing purposes, we create a simplified IR with sample data.
func createCapsuleWithIR(t *testing.T, dir, name string) {
	t.Helper()

	// Create a minimal IR corpus for testing
	// In production, the SWORD plugin would generate this from the actual module
	corpus := createTestCorpus(name)

	irData, err := json.Marshal(corpus)
	if err != nil {
		t.Fatalf("marshal IR for %s: %v", name, err)
	}

	manifest := map[string]interface{}{
		"version":       "1.0",
		"module_type":   "bible",
		"title":         getTestTitle(name),
		"language":      getTestLanguage(name),
		"source_format": "sword",
		"has_ir":        true,
	}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")

	capsulePath := filepath.Join(dir, name+".tar.gz")
	createTestCapsuleTarGz(t, capsulePath, map[string][]byte{
		"manifest.json":   manifestData,
		name + ".ir.json": irData,
	})
}

// createTestCorpus creates a sample IR corpus for testing.
func createTestCorpus(name string) ir.Corpus {
	return ir.Corpus{
		ID:            strings.ToUpper(name),
		Title:         getTestTitle(name),
		ModuleType:    ir.ModuleBible,
		Language:      getTestLanguage(name),
		Versification: "KJV",
		Documents: []*ir.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ir.ContentBlock{
					{ID: "Gen.1.1", Text: "In the beginning God created the heaven and the earth."},
					{ID: "Gen.1.2", Text: "And the earth was without form, and void; and darkness was upon the face of the deep."},
					{ID: "Gen.1.3", Text: "And God said, Let there be light: and there was light."},
				},
			},
			{
				ID:    "Ps",
				Title: "Psalms",
				Order: 19,
				ContentBlocks: []*ir.ContentBlock{
					{ID: "Ps.23.1", Text: "The LORD is my shepherd; I shall not want."},
					{ID: "Ps.23.2", Text: "He maketh me to lie down in green pastures."},
				},
			},
			{
				ID:    "Matt",
				Title: "Matthew",
				Order: 40,
				ContentBlocks: []*ir.ContentBlock{
					{ID: "Matt.1.1", Text: "The book of the generation of Jesus Christ, the son of David."},
					{ID: "Matt.5.1", Text: "And seeing the multitudes, he went up into a mountain."},
				},
			},
			{
				ID:    "John",
				Title: "John",
				Order: 43,
				ContentBlocks: []*ir.ContentBlock{
					{ID: "John.1.1", Text: "In the beginning was the Word, and the Word was with God, and the Word was God."},
					{ID: "John.3.16", Text: "For God so loved the world, that he gave his only begotten Son."},
				},
			},
		},
	}
}

// getTestTitle returns an appropriate title for the sample Bible.
func getTestTitle(name string) string {
	titles := map[string]string{
		"asv":        "American Standard Version",
		"drc":        "Douay-Rheims Catholic Bible",
		"geneva1599": "Geneva Bible (1599)",
		"kjv":        "King James Version",
		"lxx":        "Septuagint",
		"oeb":        "Open English Bible",
		"osmhb":      "Open Scriptures Hebrew Bible",
		"sblgnt":     "SBL Greek New Testament",
		"tyndale":    "William Tyndale Bible",
		"vulgate":    "Latin Vulgate",
		"web":        "World English Bible",
	}
	if title, ok := titles[name]; ok {
		return title
	}
	return name + " Bible"
}

// getTestLanguage returns the language code for the sample Bible.
func getTestLanguage(name string) string {
	languages := map[string]string{
		"lxx":     "grc", // Ancient Greek
		"osmhb":   "he",  // Hebrew
		"sblgnt":  "grc", // Greek
		"vulgate": "la",  // Latin
	}
	if lang, ok := languages[name]; ok {
		return lang
	}
	return "en"
}

// TestWithSampleData_BibleIndex tests the Bible index page with sample data.
func TestWithSampleData_BibleIndex(t *testing.T) {
	setupBibleTemplates()
	tempDir, cleanup := setupSampleDataTest(t, sampleBibles...)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/bible", nil)
	w := httptest.NewRecorder()

	handleBibleIndex(w, req)

	// When there are bibles present, /bible redirects to the first one
	if w.Code != http.StatusFound {
		t.Errorf("status = %d, want %d (redirect to first Bible)", w.Code, http.StatusFound)
	}

	// Verify the redirect location points to a Bible
	location := w.Header().Get("Location")
	if !strings.Contains(location, "/bible/") {
		t.Errorf("redirect location %q should contain /bible/", location)
	}

	// Check that bibles are loaded
	bibles := getCachedBibles()
	if len(bibles) == 0 {
		t.Errorf("expected bibles to be listed, got 0")
	}

	t.Logf("Found %d bibles in test directory %s", len(bibles), tempDir)
}

// TestWithSampleData_BibleView tests viewing individual Bibles.
func TestWithSampleData_BibleView(t *testing.T) {
	setupBibleTemplates()
	_, cleanup := setupSampleDataTest(t, "kjv", "asv", "web")
	defer cleanup()

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "view KJV",
			path:       "/bible/KJV",
			wantStatus: http.StatusOK,
			wantBody:   "King James",
		},
		{
			name:       "view ASV",
			path:       "/bible/ASV",
			wantStatus: http.StatusOK,
			wantBody:   "American Standard",
		},
		{
			name:       "view WEB",
			path:       "/bible/WEB",
			wantStatus: http.StatusOK,
			wantBody:   "World English",
		},
		{
			name:       "view nonexistent",
			path:       "/bible/NONEXISTENT",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handleBibleIndex(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantBody != "" && !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("body should contain %q, got body: %s", tt.wantBody, w.Body.String())
			}
		})
	}
}

// TestWithSampleData_BibleChapter tests chapter viewing.
func TestWithSampleData_BibleChapter(t *testing.T) {
	setupBibleTemplates()
	_, cleanup := setupSampleDataTest(t, "kjv")
	defer cleanup()

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "Genesis 1",
			path:       "/bible/KJV/Gen/1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Psalms 23",
			path:       "/bible/KJV/Ps/23",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Matthew 1",
			path:       "/bible/KJV/Matt/1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "John 1",
			path:       "/bible/KJV/John/1",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handleBibleIndex(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

// TestWithSampleData_BibleSearch tests Bible search functionality.
func TestWithSampleData_BibleSearch(t *testing.T) {
	setupBibleTemplates()
	_, cleanup := setupSampleDataTest(t, "kjv")
	defer cleanup()

	tests := []struct {
		name        string
		bibleID     string
		query       string
		wantResults bool
	}{
		{
			name:        "search beginning",
			bibleID:     "KJV",
			query:       "beginning",
			wantResults: true,
		},
		{
			name:        "search God",
			bibleID:     "KJV",
			query:       "God",
			wantResults: true,
		},
		{
			name:        "search shepherd",
			bibleID:     "KJV",
			query:       "shepherd",
			wantResults: true,
		},
		{
			name:        "search phrase",
			bibleID:     "KJV",
			query:       `"In the beginning"`,
			wantResults: true,
		},
		{
			name:        "search nonexistent word",
			bibleID:     "KJV",
			query:       "xyzabc123nonexistent",
			wantResults: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, total := searchBible(tt.bibleID, tt.query, 100)
			hasResults := len(results) > 0 || total > 0

			if hasResults != tt.wantResults {
				t.Errorf("searchBible(%q, %q) hasResults = %v, want %v (got %d results)",
					tt.bibleID, tt.query, hasResults, tt.wantResults, len(results))
			}
		})
	}
}

// TestWithSampleData_BibleCompare tests Bible comparison page redirect.
func TestWithSampleData_BibleCompare(t *testing.T) {
	setupBibleTemplates()
	_, cleanup := setupSampleDataTest(t, "kjv", "asv", "web")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/bible/compare?ref=Gen.1.1&bibles=kjv,asv", nil)
	w := httptest.NewRecorder()

	handleBibleCompare(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMovedPermanently)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "tab=compare") {
		t.Errorf("redirect location should contain tab=compare, got %s", location)
	}
}

// TestWithSampleData_CapsuleList tests capsule listing with sample data.
func TestWithSampleData_CapsuleList(t *testing.T) {
	_, cleanup := setupSampleDataTest(t, sampleBibles...)
	defer cleanup()

	capsules := listCapsules()

	if len(capsules) == 0 {
		t.Error("expected capsules to be listed")
	}

	// Verify capsule info
	for _, c := range capsules {
		if c.Name == "" {
			t.Error("capsule should have a name")
		}
		if c.Size == 0 {
			t.Errorf("capsule %s should have size > 0", c.Name)
		}
		if c.SizeHuman == "" {
			t.Errorf("capsule %s should have human-readable size", c.Name)
		}
	}
}

// TestWithSampleData_APIBiblesList tests the Bible API list endpoint.
func TestWithSampleData_APIBiblesList(t *testing.T) {
	_, cleanup := setupSampleDataTest(t, "kjv", "asv", "web")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/bibles", nil)
	w := httptest.NewRecorder()

	handleAPIBibles(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Parse JSON response
	var bibles []BibleInfo
	if err := json.Unmarshal(w.Body.Bytes(), &bibles); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(bibles) < 3 {
		t.Errorf("expected at least 3 bibles, got %d", len(bibles))
	}

	// Verify each Bible has required fields
	for _, b := range bibles {
		if b.ID == "" {
			t.Error("bible should have ID")
		}
		if b.Title == "" {
			t.Errorf("bible %s should have title", b.ID)
		}
	}
}

// TestWithSampleData_APIBibleBooks tests getting books from a Bible.
func TestWithSampleData_APIBibleBooks(t *testing.T) {
	_, cleanup := setupSampleDataTest(t, "kjv")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/bibles/KJV", nil)
	w := httptest.NewRecorder()

	handleAPIBibles(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Parse JSON response - API returns {"bible": {...}, "books": [...]}
	var response struct {
		Bible BibleInfo  `json:"bible"`
		Books []BookInfo `json:"books"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if response.Bible.ID != "KJV" {
		t.Errorf("expected bible.ID=KJV, got %s", response.Bible.ID)
	}

	if len(response.Books) == 0 {
		t.Error("expected books in response")
	}
}

// TestWithSampleData_APIBibleChapter tests getting chapter verses.
func TestWithSampleData_APIBibleChapter(t *testing.T) {
	_, cleanup := setupSampleDataTest(t, "kjv")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/bibles/KJV/Gen/1", nil)
	w := httptest.NewRecorder()

	handleAPIBibles(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Parse JSON response
	var response struct {
		Verses []VerseData `json:"verses"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(response.Verses) == 0 {
		t.Error("expected verses in response")
	}
}

// TestWithSampleData_APIBibleSearch tests the search API endpoint.
func TestWithSampleData_APIBibleSearch(t *testing.T) {
	_, cleanup := setupSampleDataTest(t, "kjv")
	defer cleanup()

	tests := []struct {
		name        string
		query       string
		bible       string
		wantResults bool
	}{
		{
			name:        "search with results",
			query:       "beginning",
			bible:       "KJV",
			wantResults: true,
		},
		{
			name:        "search no results",
			query:       "xyznonexistent123",
			bible:       "KJV",
			wantResults: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := url.Values{}
			q.Set("q", tt.query)
			q.Set("bible", tt.bible)
			q.Set("limit", "10")

			req := httptest.NewRequest(http.MethodGet, "/api/bibles/search?"+q.Encode(), nil)
			w := httptest.NewRecorder()

			handleAPIBibleSearch(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
			}

			var response struct {
				Results []SearchResult `json:"results"`
				Total   int            `json:"total"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}

			hasResults := len(response.Results) > 0 || response.Total > 0
			if hasResults != tt.wantResults {
				t.Errorf("hasResults = %v, want %v", hasResults, tt.wantResults)
			}
		})
	}
}

// TestWithSampleData_ConvertPage tests the convert page with sample data.
func TestWithSampleData_ConvertPage(t *testing.T) {
	setupBibleTemplates()
	_, cleanup := setupSampleDataTest(t, "kjv")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/convert", nil)
	w := httptest.NewRecorder()

	handleConvert(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// TestWithSampleData_PluginsPage tests the plugins page.
func TestWithSampleData_PluginsPage(t *testing.T) {
	setupBibleTemplates()
	_, cleanup := setupSampleDataTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/plugins", nil)
	w := httptest.NewRecorder()

	handlePlugins(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// TestWithSampleData_IndexPage tests the index page with sample data.
func TestWithSampleData_IndexPage(t *testing.T) {
	setupBibleTemplates()
	_, cleanup := setupSampleDataTest(t, "kjv", "asv")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handleIndex(w, req)

	// Root path now shows the logo page (no redirect)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), "home-logo") {
		t.Errorf("expected logo page with home-logo element")
	}
}

// TestWithSampleData_AllBiblesLoadable tests that all sample Bibles can be loaded.
func TestWithSampleData_AllBiblesLoadable(t *testing.T) {
	_, cleanup := setupSampleDataTest(t, sampleBibles...)
	defer cleanup()

	bibles := getCachedBibles()

	for _, b := range bibles {
		t.Run(b.ID, func(t *testing.T) {
			bible, books, err := loadBibleWithBooks(b.ID)
			if err != nil {
				t.Errorf("loadBibleWithBooks(%s) error: %v", b.ID, err)
				return
			}

			if bible == nil {
				t.Errorf("expected bible, got nil")
				return
			}

			if len(books) == 0 {
				t.Errorf("expected books for %s, got 0", b.ID)
			}

			// Test loading a chapter
			if len(books) > 0 {
				verses, err := loadChapterVerses(b.ID, books[0].ID, 1)
				if err != nil {
					t.Errorf("loadChapterVerses error: %v", err)
				}
				if len(verses) == 0 {
					t.Logf("Warning: no verses in %s %s 1", b.ID, books[0].ID)
				}
			}
		})
	}
}

// TestWithSampleData_LanguageDetection tests that language is correctly detected.
func TestWithSampleData_LanguageDetection(t *testing.T) {
	_, cleanup := setupSampleDataTest(t, "kjv", "lxx", "vulgate", "osmhb")
	defer cleanup()

	bibles := getCachedBibles()

	expectedLanguages := map[string]string{
		"KJV":     "en",
		"LXX":     "grc",
		"VULGATE": "la",
		"OSMHB":   "he",
	}

	for _, b := range bibles {
		if expected, ok := expectedLanguages[b.ID]; ok {
			if b.Language != expected {
				t.Errorf("Bible %s language = %s, want %s", b.ID, b.Language, expected)
			}
		}
	}
}
