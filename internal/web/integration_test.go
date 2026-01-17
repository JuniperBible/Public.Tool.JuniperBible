package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

// TestWebUIIntegration tests the full web UI with a real HTTP server.
func TestWebUIIntegration(t *testing.T) {
	setupBibleTemplates()

	// Create a temp directory with test capsules
	tempDir := t.TempDir()
	origCapsulesDir := ServerConfig.CapsulesDir
	origPluginsDir := ServerConfig.PluginsDir
	ServerConfig.CapsulesDir = tempDir
	ServerConfig.PluginsDir = filepath.Join(tempDir, "plugins")
	defer func() {
		ServerConfig.CapsulesDir = origCapsulesDir
		ServerConfig.PluginsDir = origPluginsDir
	}()

	// Clear all caches for clean test
	clearAllCaches()

	// Create test capsules
	createTestBibleCapsule(t, tempDir, "KJV")
	createTestBibleCapsule(t, tempDir, "NIV")

	// Clear caches so they pick up the new capsules
	clearAllCaches()

	// Create a minimal plugins directory
	os.MkdirAll(filepath.Join(ServerConfig.PluginsDir, "format", "sword"), 0755)
	os.WriteFile(filepath.Join(ServerConfig.PluginsDir, "format", "sword", "plugin.json"),
		[]byte(`{"plugin_id":"format.sword","version":"1.0.0","kind":"format","entrypoint":"format-sword"}`), 0644)

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/capsule/", handleCapsule)
	mux.HandleFunc("/artifact/", handleArtifact)
	mux.HandleFunc("/ir/", handleIR)
	mux.HandleFunc("/plugins", handlePluginsRedirect)
	mux.HandleFunc("/juniper", handleJuniper)
	mux.HandleFunc("/convert", handleConvert)
	mux.HandleFunc("/bible/compare", handleBibleCompare)
	mux.HandleFunc("/bible/search", handleBibleSearch)
	mux.HandleFunc("/bible/", handleBibleIndex)
	mux.HandleFunc("/bible", handleBibleIndex)
	mux.HandleFunc("/library/bibles/", handleLibraryBibles)
	mux.HandleFunc("/library/bibles", handleLibraryBibles)
	mux.HandleFunc("/api/bibles/search", handleAPIBibleSearch)
	mux.HandleFunc("/api/bibles/", handleAPIBibles)
	mux.HandleFunc("/api/bibles", handleAPIBibles)

	server := httptest.NewServer(mux)
	defer server.Close()

	tests := []struct {
		name           string
		path           string
		method         string
		wantStatus     int
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:         "home page shows logo",
			path:         "/",
			method:       "GET",
			wantStatus:   http.StatusOK,
			wantContains: []string{"home-logo"}, // Logo page with easter egg
		},
		{
			name:         "plugins page redirects",
			path:         "/plugins",
			method:       "GET",
			wantStatus:   http.StatusOK, // Client follows redirect to /juniper?tab=plugins
			wantContains: []string{"Format Plugins"},
		},
		{
			name:         "convert page",
			path:         "/convert",
			method:       "GET",
			wantStatus:   http.StatusOK,
			wantContains: []string{"Convert"},
		},
		{
			name:         "bible index redirects to first bible",
			path:         "/bible",
			method:       "GET",
			wantStatus:   http.StatusFound, // 302 redirect
			wantContains: []string{"/bible/KJV"}, // Location header contains KJV
		},
		{
			name:         "bible compare tab is in library",
			path:         "/library/bibles?tab=compare",
			method:       "GET",
			wantStatus:   http.StatusOK,
			wantContains: []string{"tab-compare"},
		},
		{
			name:         "bible search page",
			path:         "/bible/search",
			method:       "GET",
			wantStatus:   http.StatusOK,
			wantContains: []string{"Search"},
		},
		{
			name:         "bible view specific",
			path:         "/bible/KJV",
			method:       "GET",
			wantStatus:   http.StatusOK,
			wantContains: []string{"KJV"},
		},
		{
			name:       "bible book view",
			path:       "/bible/KJV/Gen",
			method:     "GET",
			wantStatus: http.StatusOK,
		},
		{
			name:       "bible chapter view",
			path:       "/bible/KJV/Gen/1",
			method:     "GET",
			wantStatus: http.StatusOK,
		},
		{
			name:       "api list bibles",
			path:       "/api/bibles",
			method:     "GET",
			wantStatus: http.StatusOK,
		},
		{
			name:       "api get specific bible",
			path:       "/api/bibles/KJV",
			method:     "GET",
			wantStatus: http.StatusOK,
		},
		{
			name:         "api search",
			path:         "/api/bibles/search?q=beginning&bible=KJV",
			method:       "GET",
			wantStatus:   http.StatusOK,
			wantContains: []string{"results"},
		},
	}

	// Create a client that doesn't follow redirects for redirect tests
	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, server.URL+tt.path, nil)
			if err != nil {
				t.Fatalf("create request: %v", err)
			}

			// Use noRedirectClient for redirect tests to check the actual redirect response
			client := http.DefaultClient
			if tt.wantStatus == http.StatusFound || tt.wantStatus == http.StatusMovedPermanently {
				client = noRedirectClient
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want %d\nbody: %s", resp.StatusCode, tt.wantStatus, string(body))
				return
			}

			// For redirect responses, check Location header instead of body
			if tt.wantStatus == http.StatusFound || tt.wantStatus == http.StatusMovedPermanently {
				location := resp.Header.Get("Location")
				for _, want := range tt.wantContains {
					if !strings.Contains(location, want) {
						t.Errorf("Location header %q should contain %q", location, want)
					}
				}
				return
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			bodyStr := string(body)

			for _, want := range tt.wantContains {
				if !strings.Contains(bodyStr, want) {
					t.Errorf("body should contain %q", want)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(bodyStr, notWant) {
					t.Errorf("body should not contain %q", notWant)
				}
			}
		})
	}
}

// TestAPIBiblesJSON tests that the API returns valid JSON.
func TestAPIBiblesJSON(t *testing.T) {
	setupBibleTemplates()

	tempDir := t.TempDir()
	origCapsulesDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tempDir
	defer func() {
		ServerConfig.CapsulesDir = origCapsulesDir
	}()

	// Clear all caches for clean test
	clearAllCaches()

	// Create test Bible
	createTestBibleCapsule(t, tempDir, "KJV")

	// Clear caches so they pick up the new capsule
	clearAllCaches()

	// Test list bibles
	req := httptest.NewRequest(http.MethodGet, "/api/bibles", nil)
	w := httptest.NewRecorder()
	handleAPIBibles(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var bibles []BibleInfo
	if err := json.Unmarshal(w.Body.Bytes(), &bibles); err != nil {
		t.Fatalf("unmarshal bibles: %v", err)
	}

	if len(bibles) != 1 {
		t.Errorf("expected 1 bible, got %d", len(bibles))
	}

	if bibles[0].ID != "KJV" {
		t.Errorf("expected KJV, got %s", bibles[0].ID)
	}
}

// TestSearchBibleIntegration tests Bible search functionality.
func TestSearchBibleIntegration(t *testing.T) {
	tempDir := t.TempDir()
	origCapsulesDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tempDir
	defer func() {
		ServerConfig.CapsulesDir = origCapsulesDir
	}()

	// Create test Bible
	createTestBibleCapsule(t, tempDir, "KJV")

	// Test search for "beginning"
	results, total := searchBible("KJV", "beginning", 10)
	if total == 0 {
		t.Error("expected search results for 'beginning'")
	}
	if len(results) == 0 {
		t.Error("expected non-empty results")
	}

	// Verify result content
	found := false
	for _, r := range results {
		if strings.Contains(r.Text, "beginning") {
			found = true
			break
		}
	}
	if !found {
		t.Error("search results should contain 'beginning'")
	}

	// Test phrase search
	results, total = searchBible("KJV", `"In the beginning"`, 10)
	if total == 0 {
		t.Error("expected search results for phrase 'In the beginning'")
	}
}

// TestBibleNavigationIntegration tests Bible navigation.
func TestBibleNavigationIntegration(t *testing.T) {
	setupBibleTemplates()

	tempDir := t.TempDir()
	origCapsulesDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tempDir
	defer func() {
		ServerConfig.CapsulesDir = origCapsulesDir
	}()

	// Create test Bible
	createTestBibleCapsule(t, tempDir, "KJV")

	// Test load bible with books
	bible, books, err := loadBibleWithBooks("KJV")
	if err != nil {
		t.Fatalf("load bible: %v", err)
	}

	if bible.ID != "KJV" {
		t.Errorf("expected ID=KJV, got %s", bible.ID)
	}

	if len(books) != 2 {
		t.Errorf("expected 2 books, got %d", len(books))
	}

	// Test book order
	if books[0].ID != "Gen" {
		t.Errorf("expected first book Gen, got %s", books[0].ID)
	}

	// Test chapter verses
	verses, err := loadChapterVerses("KJV", "Gen", 1)
	if err != nil {
		t.Fatalf("load chapter verses: %v", err)
	}

	if len(verses) != 2 {
		t.Errorf("expected 2 verses in Gen 1, got %d", len(verses))
	}

	// Verify verse content
	if !strings.Contains(verses[0].Text, "beginning") {
		t.Errorf("Gen 1:1 should contain 'beginning': %s", verses[0].Text)
	}
}

// TestCapsuleListingIntegration tests capsule listing functionality.
func TestCapsuleListingIntegration(t *testing.T) {
	tempDir := t.TempDir()
	origCapsulesDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tempDir
	defer func() {
		ServerConfig.CapsulesDir = origCapsulesDir
	}()

	// Clear all caches for clean test
	clearAllCaches()

	// Initially empty
	capsules := listCapsules()
	if len(capsules) != 0 {
		t.Errorf("expected 0 capsules, got %d", len(capsules))
	}

	// Create a Bible capsule
	createTestBibleCapsule(t, tempDir, "KJV")

	// Clear caches so they pick up the new capsule
	clearAllCaches()

	capsules = listCapsules()
	if len(capsules) != 1 {
		t.Errorf("expected 1 capsule, got %d", len(capsules))
	}

	// Create another
	createTestBibleCapsule(t, tempDir, "NIV")

	// Clear caches so they pick up the new capsule
	clearAllCaches()

	capsules = listCapsules()
	if len(capsules) != 2 {
		t.Errorf("expected 2 capsules, got %d", len(capsules))
	}
}

// TestIRViewIntegration tests IR viewing functionality.
func TestIRViewIntegration(t *testing.T) {
	setupBibleTemplates()

	tempDir := t.TempDir()
	origCapsulesDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tempDir
	defer func() {
		ServerConfig.CapsulesDir = origCapsulesDir
	}()

	// Create test capsule
	createTestBibleCapsule(t, tempDir, "KJV")

	// Read IR content
	irContent, err := readIRContent(filepath.Join(tempDir, "KJV.tar.gz"))
	if err != nil {
		t.Fatalf("read IR: %v", err)
	}

	if irContent == nil {
		t.Fatal("IR content is nil")
	}

	// Check IR structure
	if _, ok := irContent["id"]; !ok {
		t.Error("IR should have 'id' field")
	}

	if _, ok := irContent["documents"]; !ok {
		t.Error("IR should have 'documents' field")
	}
}

// TestMultipleBiblesComparison tests comparing multiple Bibles.
func TestMultipleBiblesComparison(t *testing.T) {
	tempDir := t.TempDir()
	origCapsulesDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tempDir
	defer func() {
		ServerConfig.CapsulesDir = origCapsulesDir
	}()

	// Clear all caches for clean test
	clearAllCaches()

	// Create multiple Bible capsules
	createTestBibleCapsule(t, tempDir, "KJV")
	createTestBibleCapsule(t, tempDir, "NIV")
	createTestBibleCapsule(t, tempDir, "ESV")

	// Clear all caches so they pick up the new capsules
	clearAllCaches()

	bibles := getCachedBibles()
	if len(bibles) != 3 {
		t.Errorf("expected 3 bibles, got %d", len(bibles))
	}

	// Verify each can be loaded
	for _, b := range bibles {
		bible, books, err := loadBibleWithBooks(b.ID)
		if err != nil {
			t.Errorf("load %s: %v", b.ID, err)
			continue
		}
		if bible.ID != b.ID {
			t.Errorf("expected ID=%s, got %s", b.ID, bible.ID)
		}
		if len(books) == 0 {
			t.Errorf("Bible %s has no books", b.ID)
		}
	}
}

// createTestBibleCapsuleWithStrongs creates a test capsule with Strong's numbers.
func createTestBibleCapsuleWithStrongs(t *testing.T, dir, name string) string {
	t.Helper()

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
					{
						ID:   "Gen.1.1",
						Text: "In the beginning God created the heaven and the earth.",
						Tokens: []*ir.Token{
							{Text: "In", Type: ir.TokenWord},
							{Text: " ", Type: ir.TokenWhitespace},
							{Text: "the", Type: ir.TokenWord},
							{Text: " ", Type: ir.TokenWhitespace},
							{Text: "beginning", Type: ir.TokenWord, Strongs: []string{"H7225"}},
							{Text: " ", Type: ir.TokenWhitespace},
							{Text: "God", Type: ir.TokenWord, Strongs: []string{"H430"}},
						},
					},
				},
			},
		},
	}

	irData, err := json.Marshal(corpus)
	if err != nil {
		t.Fatalf("marshal IR: %v", err)
	}

	capsulePath := filepath.Join(dir, name+".tar.gz")
	createTestCapsuleTarGz(t, capsulePath, map[string][]byte{
		"manifest.json":   []byte(`{"version":"1.0","module_type":"bible","title":"` + name + ` Bible"}`),
		name + ".ir.json": irData,
	})

	return capsulePath
}

// TestStrongsSearch tests searching by Strong's numbers.
func TestStrongsSearch(t *testing.T) {
	tempDir := t.TempDir()
	origCapsulesDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = tempDir
	defer func() {
		ServerConfig.CapsulesDir = origCapsulesDir
	}()

	// Clear corpus cache to avoid picking up cached corpus from previous tests
	invalidateCorpusCache()

	// Create test Bible with Strong's numbers
	createTestBibleCapsuleWithStrongs(t, tempDir, "KJV")

	// Search by Strong's number
	results, total := searchBible("KJV", "H7225", 10)
	if total == 0 {
		t.Error("expected search results for Strong's number H7225")
	}
	if len(results) == 0 {
		t.Error("expected non-empty results for Strong's search")
	}
}
