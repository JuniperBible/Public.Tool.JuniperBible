package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
	"github.com/FocuswithJustin/JuniperBible/internal/archive"
)

// clearAllCaches clears all web caches for clean tests.
// This should be called at the start of tests that need a clean state.
func clearAllCaches() {
	invalidateBibleCache()
	invalidateCapsulesList()
	invalidateCapsuleMetadataCache()
	invalidateCorpusCache()
	archive.ClearTOCCache()
}

func setupBibleTemplates() {
	// Add Bible Templates for testing
	if Templates == nil {
		Templates = template.New("").Funcs(template.FuncMap{
			"iterate": func(n int) []int {
				result := make([]int, n)
				for i := range result {
					result[i] = i
				}
				return result
			},
			"add": func(a, b int) int {
				return a + b
			},
		})
	}

	// Add Templates if they don't exist
	if Templates.Lookup("bible_index.html") == nil {
		template.Must(Templates.New("bible_index.html").Parse(`<!DOCTYPE html><html><body>Bible Index: {{len .Bibles}} bibles<div id="tab-browse">Browse</div><div id="tab-compare">Compare</div></body></html>`))
	}
	if Templates.Lookup("bible_view.html") == nil {
		template.Must(Templates.New("bible_view.html").Parse(`<!DOCTYPE html><html><body>{{.Bible.Title}}</body></html>`))
	}
	if Templates.Lookup("bible_book.html") == nil {
		template.Must(Templates.New("bible_book.html").Parse(`<!DOCTYPE html><html><body>{{.Book.Name}}</body></html>`))
	}
	if Templates.Lookup("bible_chapter.html") == nil {
		template.Must(Templates.New("bible_chapter.html").Parse(`<!DOCTYPE html><html><body>{{.Chapter}}</body></html>`))
	}
	if Templates.Lookup("bible_compare.html") == nil {
		template.Must(Templates.New("bible_compare.html").Parse(`<!DOCTYPE html><html><body>Compare: {{.DefaultRef}}</body></html>`))
	}
	if Templates.Lookup("bible_search.html") == nil {
		template.Must(Templates.New("bible_search.html").Parse(`<!DOCTYPE html><html><body>Search: {{.Query}}</body></html>`))
	}
	if Templates.Lookup("header") == nil {
		template.Must(Templates.New("header").Parse(`<!DOCTYPE html><html><head><title>Test</title></head><body>`))
	}
	if Templates.Lookup("footer") == nil {
		template.Must(Templates.New("footer").Parse(`</body></html>`))
	}
	if Templates.Lookup("index.html") == nil {
		template.Must(Templates.New("index.html").Parse(`{{template "header" .}}<svg id="home-logo" viewBox="0 0 100 100"></svg>{{template "footer" .}}`))
	}
	if Templates.Lookup("juniper.html") == nil {
		template.Must(Templates.New("juniper.html").Parse(`{{template "header" .}}
<h1>Settings</h1>
<div id="tab-plugins">Format Plugins</div>
<div id="tab-detect">Detect</div>
<div id="tab-convert">Convert</div>
<div id="tab-info">Info</div>
{{template "footer" .}}`))
	}
	if Templates.Lookup("convert.html") == nil {
		template.Must(Templates.New("convert.html").Parse(`{{template "header" .}}<h1>Convert</h1>{{template "footer" .}}`))
	}
}

func createTestBibleCapsule(t *testing.T, dir, name string) string {
	t.Helper()

	// Create a minimal IR corpus
	corpus := ir.Corpus{
		ID:            name,
		Title:         name + " Bible",
		ModuleType:    ir.ModuleBible,
		Language:      "en",
		Versification: "KJV",
		Documents: []*ir.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ir.ContentBlock{
					{ID: "Gen.1.1", Text: "In the beginning God created the heaven and the earth."},
					{ID: "Gen.1.2", Text: "And the earth was without form, and void."},
					{ID: "Gen.2.1", Text: "Thus the heavens and the earth were finished."},
				},
			},
			{
				ID:    "Matt",
				Title: "Matthew",
				Order: 40,
				ContentBlocks: []*ir.ContentBlock{
					{ID: "Matt.1.1", Text: "The book of the generation of Jesus Christ."},
				},
			},
		},
	}

	irData, err := json.Marshal(corpus)
	if err != nil {
		t.Fatalf("marshal IR: %v", err)
	}

	// Use .tar.gz without .capsule so ID extraction works correctly
	capsulePath := filepath.Join(dir, name+".tar.gz")
	createTestCapsuleTarGz(t, capsulePath, map[string][]byte{
		"manifest.json":   []byte(`{"version":"1.0","module_type":"bible","title":"` + name + ` Bible"}`),
		name + ".ir.json": irData,
	})

	return capsulePath
}

func TestIsNewTestament(t *testing.T) {
	tests := []struct {
		bookID string
		want   bool
	}{
		{"Matt", true},
		{"Mark", true},
		{"Luke", true},
		{"John", true},
		{"Acts", true},
		{"Rom", true},
		{"1Cor", true},
		{"2Cor", true},
		{"Gal", true},
		{"Eph", true},
		{"Phil", true},
		{"Col", true},
		{"1Thess", true},
		{"2Thess", true},
		{"1Tim", true},
		{"2Tim", true},
		{"Titus", true},
		{"Phlm", true},
		{"Heb", true},
		{"Jas", true},
		{"1Pet", true},
		{"2Pet", true},
		{"1John", true},
		{"2John", true},
		{"3John", true},
		{"Jude", true},
		{"Rev", true},
		{"Gen", false},
		{"Exod", false},
		{"Ps", false},
		{"Isa", false},
		{"Mal", false},
	}

	for _, tt := range tests {
		t.Run(tt.bookID, func(t *testing.T) {
			got := isNewTestament(tt.bookID)
			if got != tt.want {
				t.Errorf("isNewTestament(%q) = %v, want %v", tt.bookID, got, tt.want)
			}
		})
	}
}

func TestCountChapters(t *testing.T) {
	tests := []struct {
		name string
		doc  *ir.Document
		want int
	}{
		{
			name: "single chapter",
			doc: &ir.Document{
				ID: "Gen",
				ContentBlocks: []*ir.ContentBlock{
					{ID: "Gen.1.1"},
					{ID: "Gen.1.2"},
					{ID: "Gen.1.3"},
				},
			},
			want: 1,
		},
		{
			name: "multiple chapters",
			doc: &ir.Document{
				ID: "Gen",
				ContentBlocks: []*ir.ContentBlock{
					{ID: "Gen.1.1"},
					{ID: "Gen.2.1"},
					{ID: "Gen.3.1"},
				},
			},
			want: 3,
		},
		{
			name: "empty document",
			doc: &ir.Document{
				ID:            "Gen",
				ContentBlocks: []*ir.ContentBlock{},
			},
			want: 0,
		},
		{
			name: "non-standard IDs",
			doc: &ir.Document{
				ID: "Gen",
				ContentBlocks: []*ir.ContentBlock{
					{ID: "intro"},
					{ID: "title"},
				},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countChapters(tt.doc)
			if got != tt.want {
				t.Errorf("countChapters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseContentBlockRef(t *testing.T) {
	tests := []struct {
		cbID        string
		bookID      string
		wantChapter int
		wantVerse   int
	}{
		{"Gen.1.1", "Gen", 1, 1},
		{"Gen.50.26", "Gen", 50, 26},
		{"Matt.1.1", "Matt", 1, 1},
		{"Rev.22.21", "Rev", 22, 21},
		{"invalid", "Gen", 1, 1}, // returns defaults
		{"", "Gen", 1, 1},        // returns defaults
	}

	for _, tt := range tests {
		t.Run(tt.cbID, func(t *testing.T) {
			gotChapter, gotVerse := parseContentBlockRef(tt.cbID, tt.bookID)
			if gotChapter != tt.wantChapter || gotVerse != tt.wantVerse {
				t.Errorf("parseContentBlockRef(%q, %q) = (%d, %d), want (%d, %d)",
					tt.cbID, tt.bookID, gotChapter, gotVerse, tt.wantChapter, tt.wantVerse)
			}
		})
	}
}

func TestParseIRToCorpus(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]interface{}
		wantNil bool
	}{
		{
			name: "valid corpus",
			input: map[string]interface{}{
				"id":          "KJV",
				"title":       "King James Version",
				"module_type": "bible",
				"language":    "en",
			},
			wantNil: false,
		},
		{
			name:    "empty input",
			input:   map[string]interface{}{},
			wantNil: false, // empty corpus is still valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseIRToCorpus(tt.input)
			if (got == nil) != tt.wantNil {
				t.Errorf("parseIRToCorpus() = %v, wantNil %v", got, tt.wantNil)
			}
		})
	}
}

func TestParseIRToCorpusNil(t *testing.T) {
	// Separate test for nil input since parseIRToCorpus marshals nil to "null"
	got := parseIRToCorpus(nil)
	// json.Marshal(nil) produces "null", which unmarshals to empty corpus
	if got == nil {
		t.Error("parseIRToCorpus(nil) unexpectedly returned nil")
	}
}

func TestHandleBibleIndex(t *testing.T) {
	setupBibleTemplates()
	// Create temp directory with test capsules
	tempDir := t.TempDir()
	ServerConfig.CapsulesDir = tempDir

	// Add bible_empty.html template for testing
	if Templates.Lookup("bible_empty.html") == nil {
		template.Must(Templates.New("bible_empty.html").Parse(`<!DOCTYPE html><html><body>No Bibles installed</body></html>`))
	}

	tests := []struct {
		name       string
		path       string
		setup      func()
		wantStatus int
		wantBody   string
	}{
		{
			name: "bible index with no capsules",
			path: "/bible",
			setup: func() {
				clearAllCaches()
			},
			wantStatus: http.StatusOK,
			wantBody:   "No Bibles installed",
		},
		{
			name: "bible index with capsules redirects",
			path: "/bible/",
			setup: func() {
				createTestBibleCapsule(t, tempDir, "KJV")
				clearAllCaches()
			},
			wantStatus: http.StatusFound, // 302 redirect to first Bible
			wantBody:   "/bible/KJV/Gen/1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up and setup
			os.RemoveAll(tempDir)
			os.MkdirAll(tempDir, 0755)
			tt.setup()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handleBibleIndex(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want to contain %q", w.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestHandleBibleCompare(t *testing.T) {
	setupBibleTemplates()
	tempDir := t.TempDir()
	ServerConfig.CapsulesDir = tempDir

	req := httptest.NewRequest(http.MethodGet, "/bible/compare?ref=Gen.1.1", nil)
	w := httptest.NewRecorder()

	handleBibleCompare(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMovedPermanently)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "tab=compare") {
		t.Errorf("redirect location should contain tab=compare, got %s", location)
	}
	if !strings.Contains(location, "ref=Gen.1.1") {
		t.Errorf("redirect location should contain ref=Gen.1.1, got %s", location)
	}
}

func TestHandleBibleSearch(t *testing.T) {
	setupBibleTemplates()
	tempDir := t.TempDir()
	ServerConfig.CapsulesDir = tempDir

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "empty search",
			query:      "/bible/search",
			wantStatus: http.StatusOK,
		},
		{
			name:       "search with query",
			query:      "/bible/search?q=test&bible=KJV",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.query, nil)
			w := httptest.NewRecorder()

			handleBibleSearch(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleAPIBibles(t *testing.T) {
	tempDir := t.TempDir()
	ServerConfig.CapsulesDir = tempDir

	// Clear all caches for clean test
	clearAllCaches()

	// Create a test Bible capsule
	createTestBibleCapsule(t, tempDir, "KJV")

	// Clear caches so they pick up the new capsule
	clearAllCaches()

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "list all bibles",
			path:       "/api/bibles",
			wantStatus: http.StatusOK,
		},
		{
			name:       "get specific bible",
			path:       "/api/bibles/KJV",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handleAPIBibles(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			// Verify it returns JSON
			contentType := w.Header().Get("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", contentType)
			}
		})
	}
}

func TestHandleAPIBibleSearch(t *testing.T) {
	tempDir := t.TempDir()
	ServerConfig.CapsulesDir = tempDir

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "missing parameters",
			query:      "/api/bibles/search",
			wantStatus: http.StatusOK,
		},
		{
			name:       "with parameters",
			query:      "/api/bibles/search?q=test&bible=KJV&limit=10",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.query, nil)
			w := httptest.NewRecorder()

			handleAPIBibleSearch(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestListBibles(t *testing.T) {
	tempDir := t.TempDir()
	ServerConfig.CapsulesDir = tempDir

	// Clear all caches for clean test
	clearAllCaches()

	// Initially empty
	bibles := getCachedBibles()
	if len(bibles) != 0 {
		t.Errorf("expected 0 bibles, got %d", len(bibles))
	}

	// Create a Bible capsule
	createTestBibleCapsule(t, tempDir, "KJV")

	// Clear caches so they pick up the new capsule
	clearAllCaches()

	bibles = getCachedBibles()
	if len(bibles) != 1 {
		t.Errorf("expected 1 bible, got %d", len(bibles))
	}

	if bibles[0].ID != "KJV" {
		t.Errorf("expected ID=KJV, got %s", bibles[0].ID)
	}
}

func TestLoadBibleWithBooks(t *testing.T) {
	tempDir := t.TempDir()
	ServerConfig.CapsulesDir = tempDir

	// Create a Bible capsule
	createTestBibleCapsule(t, tempDir, "KJV")

	tests := []struct {
		name      string
		capsuleID string
		wantErr   bool
		wantBooks int
	}{
		{
			name:      "valid capsule",
			capsuleID: "KJV",
			wantErr:   false,
			wantBooks: 2,
		},
		{
			name:      "nonexistent capsule",
			capsuleID: "NONEXISTENT",
			wantErr:   true,
			wantBooks: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bible, books, err := loadBibleWithBooks(tt.capsuleID)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadBibleWithBooks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if bible == nil {
					t.Error("expected bible, got nil")
				}
				if len(books) != tt.wantBooks {
					t.Errorf("expected %d books, got %d", tt.wantBooks, len(books))
				}
			}
		})
	}
}

func TestLoadChapterVerses(t *testing.T) {
	tempDir := t.TempDir()
	ServerConfig.CapsulesDir = tempDir

	// Create a Bible capsule
	createTestBibleCapsule(t, tempDir, "KJV")

	tests := []struct {
		name       string
		capsuleID  string
		bookID     string
		chapter    int
		wantErr    bool
		wantVerses int
	}{
		{
			name:       "valid chapter 1",
			capsuleID:  "KJV",
			bookID:     "Gen",
			chapter:    1,
			wantErr:    false,
			wantVerses: 2, // Gen.1.1 and Gen.1.2
		},
		{
			name:       "valid chapter 2",
			capsuleID:  "KJV",
			bookID:     "Gen",
			chapter:    2,
			wantErr:    false,
			wantVerses: 1, // Gen.2.1
		},
		{
			name:       "nonexistent capsule",
			capsuleID:  "NONEXISTENT",
			bookID:     "Gen",
			chapter:    1,
			wantErr:    true,
			wantVerses: 0,
		},
		{
			name:       "nonexistent book",
			capsuleID:  "KJV",
			bookID:     "NonBook",
			chapter:    1,
			wantErr:    true,
			wantVerses: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verses, err := loadChapterVerses(tt.capsuleID, tt.bookID, tt.chapter)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadChapterVerses() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(verses) != tt.wantVerses {
				t.Errorf("expected %d verses, got %d", tt.wantVerses, len(verses))
			}
		})
	}
}

func TestSearchBible(t *testing.T) {
	tempDir := t.TempDir()
	ServerConfig.CapsulesDir = tempDir

	// Create a Bible capsule
	createTestBibleCapsule(t, tempDir, "KJV")

	tests := []struct {
		name        string
		bibleID     string
		query       string
		limit       int
		wantResults bool
	}{
		{
			name:        "search for 'beginning'",
			bibleID:     "KJV",
			query:       "beginning",
			limit:       10,
			wantResults: true,
		},
		{
			name:        "search for 'Jesus'",
			bibleID:     "KJV",
			query:       "Jesus",
			limit:       10,
			wantResults: true,
		},
		{
			name:        "search for nonexistent text",
			bibleID:     "KJV",
			query:       "xyzabc123",
			limit:       10,
			wantResults: false,
		},
		{
			name:        "phrase search",
			bibleID:     "KJV",
			query:       `"In the beginning"`,
			limit:       10,
			wantResults: true,
		},
		{
			name:        "nonexistent bible",
			bibleID:     "NONEXISTENT",
			query:       "test",
			limit:       10,
			wantResults: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, total := searchBible(tt.bibleID, tt.query, tt.limit)
			hasResults := len(results) > 0 || total > 0
			if hasResults != tt.wantResults {
				t.Errorf("searchBible() hasResults = %v, want %v (got %d results, %d total)",
					hasResults, tt.wantResults, len(results), total)
			}
		})
	}
}

func TestBibleRouting(t *testing.T) {
	setupBibleTemplates()
	tempDir := t.TempDir()
	ServerConfig.CapsulesDir = tempDir

	// Create a Bible capsule
	createTestBibleCapsule(t, tempDir, "KJV")

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "bible view",
			path:       "/bible/KJV",
			wantStatus: http.StatusOK,
		},
		{
			name:       "book view",
			path:       "/bible/KJV/Gen",
			wantStatus: http.StatusOK,
		},
		{
			name:       "chapter view",
			path:       "/bible/KJV/Gen/1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "nonexistent bible",
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
		})
	}
}

// NOTE: TestPreWarmCaches and TestStartBackgroundCacheRefresh are not included
// because these background goroutines interfere with other tests (specifically
// TestAPIBiblesJSON). The functions are simple enough that calling them in
// Start() provides sufficient coverage. To get full coverage of these functions
// without side effects, we would need to refactor them to accept a context for
// cancellation, which is out of scope for this test effort.
