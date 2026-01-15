package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestDefaultWebSocketSecurityConfig(t *testing.T) {
	config := DefaultWebSocketSecurityConfig()

	if len(config.AllowedOrigins) == 0 {
		t.Error("Expected default allowed origins to be set")
	}

	if config.MaxMessageRate <= 0 {
		t.Error("Expected positive max message rate")
	}

	if config.MaxMessageSize <= 0 {
		t.Error("Expected positive max message size")
	}
}

func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		name           string
		origin         string
		allowedOrigins []string
		expected       bool
	}{
		{
			name:           "Empty origin should be denied",
			origin:         "",
			allowedOrigins: []string{"*"},
			expected:       false,
		},
		{
			name:           "Wildcard allows any origin",
			origin:         "https://example.com",
			allowedOrigins: []string{"*"},
			expected:       true,
		},
		{
			name:           "Exact match allows origin",
			origin:         "https://example.com",
			allowedOrigins: []string{"https://example.com"},
			expected:       true,
		},
		{
			name:           "Different origin denied",
			origin:         "https://evil.com",
			allowedOrigins: []string{"https://example.com"},
			expected:       false,
		},
		{
			name:           "Subdomain wildcard allows subdomain",
			origin:         "https://app.example.com",
			allowedOrigins: []string{"*.example.com"},
			expected:       true,
		},
		{
			name:           "Subdomain wildcard denies different domain",
			origin:         "https://example.org",
			allowedOrigins: []string{"*.example.com"},
			expected:       false,
		},
		{
			name:           "Multiple allowed origins - first match",
			origin:         "https://app1.example.com",
			allowedOrigins: []string{"https://app1.example.com", "https://app2.example.com"},
			expected:       true,
		},
		{
			name:           "Multiple allowed origins - second match",
			origin:         "https://app2.example.com",
			allowedOrigins: []string{"https://app1.example.com", "https://app2.example.com"},
			expected:       true,
		},
		{
			name:           "Multiple allowed origins - no match",
			origin:         "https://evil.com",
			allowedOrigins: []string{"https://app1.example.com", "https://app2.example.com"},
			expected:       false,
		},
		// SECURITY: Test for subdomain spoofing attack
		{
			name:           "Subdomain wildcard blocks attacker domain",
			origin:         "https://attackerexample.com",
			allowedOrigins: []string{"*.example.com"},
			expected:       false,
		},
		{
			name:           "Subdomain wildcard blocks spoofed domain",
			origin:         "https://evilexample.com",
			allowedOrigins: []string{"*.example.com"},
			expected:       false,
		},
		{
			name:           "Subdomain wildcard allows deep subdomain",
			origin:         "https://sub.app.example.com",
			allowedOrigins: []string{"*.example.com"},
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isOriginAllowed(tt.origin, tt.allowedOrigins)
			if result != tt.expected {
				t.Errorf("isOriginAllowed(%q, %v) = %v, expected %v",
					tt.origin, tt.allowedOrigins, result, tt.expected)
			}
		})
	}
}

func TestCheckOriginWithConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   WebSocketSecurityConfig
		origin   string
		expected bool
	}{
		{
			name: "Allowed origin accepted",
			config: WebSocketSecurityConfig{
				AllowedOrigins: []string{"https://example.com"},
			},
			origin:   "https://example.com",
			expected: true,
		},
		{
			name: "Disallowed origin rejected",
			config: WebSocketSecurityConfig{
				AllowedOrigins: []string{"https://example.com"},
			},
			origin:   "https://evil.com",
			expected: false,
		},
		{
			name: "Wildcard accepts all",
			config: WebSocketSecurityConfig{
				AllowedOrigins: []string{"*"},
			},
			origin:   "https://any-origin.com",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkOrigin := CheckOriginWithConfig(tt.config)

			req := &http.Request{
				Header: http.Header{
					"Origin": []string{tt.origin},
				},
			}

			result := checkOrigin(req)
			if result != tt.expected {
				t.Errorf("CheckOrigin with origin %q = %v, expected %v",
					tt.origin, result, tt.expected)
			}
		})
	}
}

func TestValidateAuthForWebSocket(t *testing.T) {
	validAPIKey := "test-api-key-12345678"

	tests := []struct {
		name        string
		config      WebSocketSecurityConfig
		apiKey      string
		useQuery    bool
		expectError bool
	}{
		{
			name: "Auth not required - should pass",
			config: WebSocketSecurityConfig{
				RequireAuth: false,
			},
			apiKey:      "",
			expectError: false,
		},
		{
			name: "Auth required with valid key in header",
			config: WebSocketSecurityConfig{
				RequireAuth: true,
				AuthConfig: AuthConfig{
					Enabled: true,
					APIKey:  validAPIKey,
				},
			},
			apiKey:      validAPIKey,
			useQuery:    false,
			expectError: false,
		},
		{
			name: "Auth required with valid key in query param",
			config: WebSocketSecurityConfig{
				RequireAuth: true,
				AuthConfig: AuthConfig{
					Enabled: true,
					APIKey:  validAPIKey,
				},
			},
			apiKey:      validAPIKey,
			useQuery:    true,
			expectError: false,
		},
		{
			name: "Auth required with invalid key",
			config: WebSocketSecurityConfig{
				RequireAuth: true,
				AuthConfig: AuthConfig{
					Enabled: true,
					APIKey:  validAPIKey,
				},
			},
			apiKey:      "wrong-key",
			expectError: true,
		},
		{
			name: "Auth required with no key",
			config: WebSocketSecurityConfig{
				RequireAuth: true,
				AuthConfig: AuthConfig{
					Enabled: true,
					APIKey:  validAPIKey,
				},
			},
			apiKey:      "",
			expectError: true,
		},
		{
			name: "Auth required but not configured",
			config: WebSocketSecurityConfig{
				RequireAuth: true,
				AuthConfig: AuthConfig{
					Enabled: false,
				},
			},
			apiKey:      "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header: http.Header{},
				URL:    &url.URL{},
			}

			if tt.apiKey != "" {
				if tt.useQuery {
					req.URL.RawQuery = fmt.Sprintf("api_key=%s", tt.apiKey)
				} else {
					req.Header.Set("X-API-Key", tt.apiKey)
				}
			}

			errMsg := ValidateAuthForWebSocket(req, tt.config)

			if tt.expectError && errMsg == "" {
				t.Error("Expected authentication error but got none")
			}

			if !tt.expectError && errMsg != "" {
				t.Errorf("Expected no error but got: %s", errMsg)
			}
		})
	}
}

func TestWebSocketRateLimiter(t *testing.T) {
	limiter := NewWebSocketRateLimiter()

	// Create mock hub and client
	hub := NewHub()
	client := &Client{
		hub:  hub,
		conn: nil, // Not needed for this test
		send: make(chan []byte, 256),
	}

	// Register client with 5 messages per second
	limiter.Register(client, 5)

	// Should allow burst of 10 messages (2x capacity)
	allowedCount := 0
	for i := 0; i < 15; i++ {
		if limiter.Allow(client) {
			allowedCount++
		}
	}

	if allowedCount < 10 {
		t.Errorf("Expected at least 10 messages allowed in burst, got %d", allowedCount)
	}

	if allowedCount > 11 {
		t.Errorf("Expected at most 11 messages allowed, got %d", allowedCount)
	}

	// Wait for refill
	time.Sleep(1 * time.Second)

	// Should allow more messages after refill
	if !limiter.Allow(client) {
		t.Error("Expected message to be allowed after refill period")
	}

	// Unregister
	limiter.Unregister(client)

	// After unregister, should not allow
	if limiter.Allow(client) {
		t.Error("Expected message to be denied after unregister")
	}
}

func TestSecureWebSocketHandler_OriginValidation(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	rateLimiter := NewWebSocketRateLimiter()

	config := WebSocketSecurityConfig{
		AllowedOrigins: []string{"https://example.com"},
		MaxMessageRate: 10,
		MaxMessageSize: 4096,
		RequireAuth:    false,
	}

	handler := SecureWebSocketHandler(hub, config, rateLimiter)
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Test 1: Valid origin should connect
	t.Run("Valid origin connects", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("Origin", "https://example.com")

		dialer := websocket.Dialer{}
		conn, resp, err := dialer.Dial(wsURL, headers)

		if err != nil {
			t.Fatalf("Failed to connect with valid origin: %v", err)
		}
		defer conn.Close()

		if resp.StatusCode != http.StatusSwitchingProtocols {
			t.Errorf("Expected status 101, got %d", resp.StatusCode)
		}
	})

	// Test 2: Invalid origin should be rejected
	t.Run("Invalid origin rejected", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("Origin", "https://evil.com")

		dialer := websocket.Dialer{}
		_, resp, err := dialer.Dial(wsURL, headers)

		if err == nil {
			t.Fatal("Expected connection to fail with invalid origin")
		}

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", resp.StatusCode)
		}
	})
}

func TestSecureWebSocketHandler_Authentication(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	rateLimiter := NewWebSocketRateLimiter()

	validAPIKey := "secure-test-key-1234567890"

	config := WebSocketSecurityConfig{
		AllowedOrigins: []string{"*"},
		MaxMessageRate: 10,
		MaxMessageSize: 4096,
		RequireAuth:    true,
		AuthConfig: AuthConfig{
			Enabled: true,
			APIKey:  validAPIKey,
		},
	}

	handler := SecureWebSocketHandler(hub, config, rateLimiter)
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Test 1: Valid API key should connect
	t.Run("Valid API key connects", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("X-API-Key", validAPIKey)
		headers.Set("Origin", "http://localhost")

		dialer := websocket.Dialer{}
		conn, resp, err := dialer.Dial(wsURL, headers)

		if err != nil {
			t.Fatalf("Failed to connect with valid API key: %v", err)
		}
		defer conn.Close()

		if resp.StatusCode != http.StatusSwitchingProtocols {
			t.Errorf("Expected status 101, got %d", resp.StatusCode)
		}
	})

	// Test 2: Invalid API key should be rejected
	t.Run("Invalid API key rejected", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("X-API-Key", "invalid-key")
		headers.Set("Origin", "http://localhost")

		dialer := websocket.Dialer{}
		_, resp, err := dialer.Dial(wsURL, headers)

		if err == nil {
			t.Fatal("Expected connection to fail with invalid API key")
		}

		if resp != nil && resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", resp.StatusCode)
		}
	})

	// Test 3: No API key should be rejected
	t.Run("No API key rejected", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("Origin", "http://localhost")

		dialer := websocket.Dialer{}
		_, resp, err := dialer.Dial(wsURL, headers)

		if err == nil {
			t.Fatal("Expected connection to fail without API key")
		}

		if resp != nil && resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", resp.StatusCode)
		}
	})

	// Test 4: API key in query parameter should work
	t.Run("API key in query parameter", func(t *testing.T) {
		wsURLWithKey := fmt.Sprintf("%s?api_key=%s", wsURL, validAPIKey)

		headers := http.Header{}
		headers.Set("Origin", "http://localhost")

		dialer := websocket.Dialer{}
		conn, resp, err := dialer.Dial(wsURLWithKey, headers)

		if err != nil {
			t.Fatalf("Failed to connect with API key in query: %v", err)
		}
		defer conn.Close()

		if resp.StatusCode != http.StatusSwitchingProtocols {
			t.Errorf("Expected status 101, got %d", resp.StatusCode)
		}
	})
}

func TestSecureWebSocketHandler_MessageSizeLimit(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	rateLimiter := NewWebSocketRateLimiter()

	config := WebSocketSecurityConfig{
		AllowedOrigins: []string{"*"},
		MaxMessageRate: 100,
		MaxMessageSize: 1024, // 1KB limit
		RequireAuth:    false,
	}

	handler := SecureWebSocketHandler(hub, config, rateLimiter)
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	headers := http.Header{}
	headers.Set("Origin", "http://localhost")

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Try to send a message larger than the limit
	largeMessage := make([]byte, 2048) // 2KB
	for i := range largeMessage {
		largeMessage[i] = 'A'
	}

	// Set a read deadline so we don't hang
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	err = conn.WriteMessage(websocket.TextMessage, largeMessage)
	if err != nil {
		// Some implementations might reject immediately
		return
	}

	// Try to read response - connection should be closed
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("Expected connection to be closed due to message size limit")
	}
}

func TestMessageRateBucket(t *testing.T) {
	bucket := newMessageRateBucket(10) // 10 messages per second

	// Should allow initial burst
	allowedCount := 0
	for i := 0; i < 25; i++ {
		if bucket.allow() {
			allowedCount++
		}
	}

	// Burst capacity is 2x, so should allow about 20
	if allowedCount < 19 || allowedCount > 21 {
		t.Errorf("Expected ~20 messages in burst, got %d", allowedCount)
	}

	// Wait for refill
	time.Sleep(500 * time.Millisecond)

	// Should allow more messages (about 5 after 0.5 seconds at 10/sec)
	allowedAfterRefill := 0
	for i := 0; i < 10; i++ {
		if bucket.allow() {
			allowedAfterRefill++
		}
	}

	if allowedAfterRefill < 4 || allowedAfterRefill > 6 {
		t.Errorf("Expected ~5 messages after refill, got %d", allowedAfterRefill)
	}
}
