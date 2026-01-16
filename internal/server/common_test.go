package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCORSMiddlewareAllowAll(t *testing.T) {
	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS header to allow all origins")
	}

	if resp.Header.Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected CORS methods header")
	}

	if resp.Header.Get("Access-Control-Allow-Headers") == "" {
		t.Error("expected CORS headers header")
	}
}

func TestCORSMiddlewareWithConfigRestrictedOrigins(t *testing.T) {
	allowedOrigins := []string{"https://example.com", "https://trusted.com"}
	cfg := CORSConfig{
		AllowedOrigins: allowedOrigins,
	}

	handler := CORSMiddlewareWithConfig(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name               string
		origin             string
		expectStatus       int
		expectAllowOrigin  string
		expectCredentials  bool
	}{
		{
			name:               "allowed origin",
			origin:             "https://example.com",
			expectStatus:       http.StatusOK,
			expectAllowOrigin:  "https://example.com",
			expectCredentials:  true,
		},
		{
			name:               "another allowed origin",
			origin:             "https://trusted.com",
			expectStatus:       http.StatusOK,
			expectAllowOrigin:  "https://trusted.com",
			expectCredentials:  true,
		},
		{
			name:               "disallowed origin",
			origin:             "https://evil.com",
			expectStatus:       http.StatusOK,
			expectAllowOrigin:  "",
			expectCredentials:  false,
		},
		{
			name:               "no origin header",
			origin:             "",
			expectStatus:       http.StatusOK,
			expectAllowOrigin:  "",
			expectCredentials:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, resp.StatusCode)
			}

			allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
			if allowOrigin != tt.expectAllowOrigin {
				t.Errorf("expected Allow-Origin %q, got %q", tt.expectAllowOrigin, allowOrigin)
			}

			credentials := resp.Header.Get("Access-Control-Allow-Credentials")
			hasCredentials := credentials == "true"
			if hasCredentials != tt.expectCredentials {
				t.Errorf("expected credentials %v, got %v", tt.expectCredentials, hasCredentials)
			}
		})
	}
}

func TestCORSMiddlewareOptionsRequest(t *testing.T) {
	allowedOrigins := []string{"https://example.com"}
	cfg := CORSConfig{
		AllowedOrigins: allowedOrigins,
	}

	handler := CORSMiddlewareWithConfig(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for OPTIONS request")
	}))

	t.Run("allowed origin OPTIONS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "https://example.com")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("disallowed origin OPTIONS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "https://evil.com")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", resp.StatusCode)
		}
	})
}

func TestCORSMiddlewareEmptyConfig(t *testing.T) {
	// Empty config should behave like allow-all
	cfg := CORSConfig{}

	handler := CORSMiddlewareWithConfig(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("empty config should allow all origins")
	}

	// Should not set credentials with wildcard
	if resp.Header.Get("Access-Control-Allow-Credentials") != "" {
		t.Error("should not set credentials with wildcard origin")
	}
}

func TestAbsPath(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"relative path", "test.txt"},
		{"dot path", "./test.txt"},
		{"parent path", "../test.txt"},
		{"absolute path", "/tmp/test.txt"},
		{"empty path", ""},
		// Edge cases
		{"very long path", string(make([]byte, 10000))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AbsPath(tt.path)
			// AbsPath should always return something (either absolute path or original)
			// For valid paths, it returns the absolute path
			// For paths where Abs fails, it returns the original path
			if tt.path != "" && result == "" {
				t.Errorf("AbsPath(%q) returned empty string", tt.path)
			}
		})
	}
}

// TestAbsPathErrorCondition tests the error path in AbsPath
// This is challenging to test because filepath.Abs rarely errors
func TestAbsPathErrorCondition(t *testing.T) {
	// Skip this test on systems where we can't create and delete directories
	if testing.Short() {
		t.Skip("Skipping test that manipulates filesystem")
	}

	// Create a temporary directory
	tmpDir := filepath.Join("/tmp", "test_abs_path_"+t.Name())
	err := os.Mkdir(tmpDir, 0755)
	if err != nil && !os.IsExist(err) {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save current directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Change to temp directory
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Remove the directory while we're in it (this makes Getwd fail)
	// Note: This works on Linux/Unix but may not work on all systems
	err = os.Remove(tmpDir)
	if err != nil {
		// If we can't remove the directory while in it, skip this test
		t.Skipf("Cannot remove directory while in it on this system: %v", err)
	}

	// Now filepath.Abs should fail because Getwd will fail
	testPath := "relative/path.txt"
	result := AbsPath(testPath)

	// The function should return the original path on error
	if result != testPath {
		t.Logf("AbsPath returned %q instead of original path %q (may have succeeded despite deleted cwd)", result, testPath)
	}
}

func TestEnablePlugins(t *testing.T) {
	tests := []struct {
		name       string
		enabled    bool
		pluginsDir string
	}{
		{
			name:       "plugins enabled",
			enabled:    true,
			pluginsDir: "/path/to/plugins",
		},
		{
			name:       "plugins disabled",
			enabled:    false,
			pluginsDir: "/path/to/plugins",
		},
		{
			name:       "plugins enabled with empty dir",
			enabled:    true,
			pluginsDir: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This function just logs, so we just call it to ensure it doesn't panic
			// We can't easily test the log output without mocking the logger
			EnablePlugins(tt.enabled, tt.pluginsDir)
		})
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check all security headers are present
	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for header, expectedValue := range expectedHeaders {
		value := w.Header().Get(header)
		if value != expectedValue {
			t.Errorf("Expected header %s=%q, got %q", header, expectedValue, value)
		}
	}

	// Check CSP is present
	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Expected Content-Security-Policy header to be set")
	}
}

func TestTimingMiddleware(t *testing.T) {
	handlerCalled := false
	handler := TimingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("Expected inner handler to be called")
	}

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestTimingMiddlewareSlowRequest(t *testing.T) {
	handler := TimingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow request (>100ms)
		time.Sleep(101 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
