package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FocuswithJustin/JuniperBible/internal/server"
)

func TestSetupRoutes(t *testing.T) {
	mux := setupRoutes()
	if mux == nil {
		t.Fatal("setupRoutes returned nil")
	}

	// Test that routes are registered by making requests
	routes := []struct {
		path   string
		method string
	}{
		{"/", http.MethodGet},
		{"/health", http.MethodGet},
		{"/capsules", http.MethodGet},
		{"/plugins", http.MethodGet},
		{"/formats", http.MethodGet},
		{"/jobs", http.MethodGet},
	}

	for _, route := range routes {
		t.Run(route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			// Should not return 404 for registered routes (any other status is fine)
			if w.Code == http.StatusNotFound {
				t.Errorf("route %s not registered", route.path)
			}
		})
	}
}

func TestStart_InvalidAuthConfig(t *testing.T) {
	cfg := Config{
		Port:        0,
		CapsulesDir: t.TempDir(),
		Auth: AuthConfig{
			Enabled: true,
			APIKey:  "", // Empty key when auth is enabled
		},
	}

	err := Start(cfg)
	if err == nil {
		t.Error("expected error for invalid auth config")
	}
	if !strings.Contains(err.Error(), "invalid auth config") {
		t.Errorf("expected 'invalid auth config' error, got: %v", err)
	}
}

func TestStart_AuthKeyTooShort(t *testing.T) {
	cfg := Config{
		Port:        0,
		CapsulesDir: t.TempDir(),
		Auth: AuthConfig{
			Enabled: true,
			APIKey:  "short", // Too short (< 16 chars)
		},
	}

	err := Start(cfg)
	if err == nil {
		t.Error("expected error for short API key")
	}
	if !strings.Contains(err.Error(), "invalid auth config") {
		t.Errorf("expected 'invalid auth config' error, got: %v", err)
	}
}

func TestStart_TLSMissingCertFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Port:        0,
		CapsulesDir: tmpDir,
		TLS: TLSConfig{
			Enabled:  true,
			CertFile: "", // Missing cert file
			KeyFile:  "/tmp/key.pem",
		},
	}

	err := Start(cfg)
	if err == nil {
		t.Error("expected error for missing TLS cert file")
	}
	if !strings.Contains(err.Error(), "cert or key file not specified") {
		t.Errorf("expected 'cert or key file not specified' error, got: %v", err)
	}
}

func TestStart_TLSMissingKeyFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Port:        0,
		CapsulesDir: tmpDir,
		TLS: TLSConfig{
			Enabled:  true,
			CertFile: "/tmp/cert.pem",
			KeyFile:  "", // Missing key file
		},
	}

	err := Start(cfg)
	if err == nil {
		t.Error("expected error for missing TLS key file")
	}
	if !strings.Contains(err.Error(), "cert or key file not specified") {
		t.Errorf("expected 'cert or key file not specified' error, got: %v", err)
	}
}

func TestStart_TLSCertFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Port:        0,
		CapsulesDir: tmpDir,
		TLS: TLSConfig{
			Enabled:  true,
			CertFile: "/nonexistent/cert.pem",
			KeyFile:  "/nonexistent/key.pem",
		},
	}

	err := Start(cfg)
	if err == nil {
		t.Error("expected error for missing TLS cert file")
	}
	if !strings.Contains(err.Error(), "TLS cert file not found") {
		t.Errorf("expected 'TLS cert file not found' error, got: %v", err)
	}
}

func TestStart_TLSKeyFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a cert file but not a key file
	certFile := filepath.Join(tmpDir, "cert.pem")
	if err := os.WriteFile(certFile, []byte("fake cert"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Port:        0,
		CapsulesDir: tmpDir,
		TLS: TLSConfig{
			Enabled:  true,
			CertFile: certFile,
			KeyFile:  "/nonexistent/key.pem",
		},
	}

	err := Start(cfg)
	if err == nil {
		t.Error("expected error for missing TLS key file")
	}
	if !strings.Contains(err.Error(), "TLS key file not found") {
		t.Errorf("expected 'TLS key file not found' error, got: %v", err)
	}
}

// TestServerIntegration tests the full server with middleware chain using httptest
func TestServerIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up server config
	ServerConfig = Config{
		Port:        0,
		CapsulesDir: tmpDir,
	}

	// Initialize WebSocket hub (required by handlers)
	GlobalHub = NewHub()
	go GlobalHub.Run()

	// Build the handler chain as Start() does
	mux := setupRoutes()
	cspConfig := server.APICSPConfig()
	handler := server.SecurityHeadersWithCSP(cspConfig, mux)
	corsConfig := server.CORSConfig{}
	handler = server.CORSMiddlewareWithConfig(corsConfig, handler)

	// Create test server
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Test health endpoint
	t.Run("health endpoint", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatalf("failed to get health: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var apiResp APIResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if !apiResp.Success {
			t.Error("expected success to be true")
		}
	})

	// Test root endpoint
	t.Run("root endpoint", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/")
		if err != nil {
			t.Fatalf("failed to get root: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		// Check security headers are present
		if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Error("expected X-Content-Type-Options header")
		}
		if resp.Header.Get("X-Frame-Options") != "DENY" {
			t.Error("expected X-Frame-Options header")
		}
	})

	// Test CORS headers
	t.Run("CORS headers", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
		req.Header.Set("Origin", "https://example.com")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed request: %v", err)
		}
		defer resp.Body.Close()

		if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
			t.Error("expected CORS header")
		}
	})
}

// TestServerIntegrationWithAuth tests server with authentication enabled
func TestServerIntegrationWithAuth(t *testing.T) {
	tmpDir := t.TempDir()

	apiKey := "test-api-key-12345678"

	// Set up server config with auth
	ServerConfig = Config{
		Port:        0,
		CapsulesDir: tmpDir,
		Auth: AuthConfig{
			Enabled: true,
			APIKey:  apiKey,
		},
	}

	// Initialize WebSocket hub
	GlobalHub = NewHub()
	go GlobalHub.Run()

	// Build the handler chain with auth
	mux := setupRoutes()
	cspConfig := server.APICSPConfig()
	handler := server.SecurityHeadersWithCSP(cspConfig, mux)
	handler = AuthMiddleware(ServerConfig.Auth, handler)
	corsConfig := server.CORSConfig{}
	handler = server.CORSMiddlewareWithConfig(corsConfig, handler)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Test that unauthenticated request to non-public endpoint fails
	t.Run("unauthenticated request fails", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/capsules")
		if err != nil {
			t.Fatalf("failed request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})

	// Test that authenticated request succeeds
	t.Run("authenticated request succeeds", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/capsules", nil)
		req.Header.Set("X-API-Key", apiKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})

	// Test that public endpoints work without auth
	t.Run("public endpoint without auth", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatalf("failed request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})
}

// TestServerIntegrationWithRateLimit tests server with rate limiting
func TestServerIntegrationWithRateLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up server config with rate limiting
	ServerConfig = Config{
		Port:              0,
		CapsulesDir:       tmpDir,
		RateLimitRequests: 60,
		RateLimitBurst:    5,
	}

	// Initialize WebSocket hub
	GlobalHub = NewHub()
	go GlobalHub.Run()

	// Build the handler chain with rate limiting
	mux := setupRoutes()
	cspConfig := server.APICSPConfig()
	handler := server.SecurityHeadersWithCSP(cspConfig, mux)

	rateLimitConfig := RateLimiterConfig{
		RequestsPerMinute: ServerConfig.RateLimitRequests,
		BurstSize:         ServerConfig.RateLimitBurst,
	}
	rateLimiter := NewRateLimiter(rateLimitConfig)
	handler = rateLimiter.Middleware(handler)

	corsConfig := server.CORSConfig{}
	handler = server.CORSMiddlewareWithConfig(corsConfig, handler)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Test that rate limit headers are present
	t.Run("rate limit headers", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatalf("failed request: %v", err)
		}
		defer resp.Body.Close()

		if resp.Header.Get("X-RateLimit-Limit") == "" {
			t.Error("expected X-RateLimit-Limit header")
		}
		if resp.Header.Get("X-RateLimit-Remaining") == "" {
			t.Error("expected X-RateLimit-Remaining header")
		}
	})
}

// TestStartServerAndConnect starts the actual server and makes a connection
func TestStartServerAndConnect(t *testing.T) {
	// Find a free port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	tmpDir := t.TempDir()

	cfg := Config{
		Port:        port,
		CapsulesDir: tmpDir,
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- Start(cfg)
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Check if server started (should not have errored yet)
	select {
	case err := <-serverErr:
		t.Fatalf("server failed to start: %v", err)
	default:
		// Server is running
	}

	// Make a request to the running server
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err != nil {
		t.Fatalf("failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !apiResp.Success {
		t.Error("expected success to be true")
	}

	// Test security headers
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected security headers")
	}
}
