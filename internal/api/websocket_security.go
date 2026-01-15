package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketSecurityConfig holds WebSocket-specific security configuration.
type WebSocketSecurityConfig struct {
	// AllowedOrigins is a list of allowed origin patterns.
	// Use "*" to allow all origins (not recommended for production).
	// Use specific domains like "https://example.com" for production.
	AllowedOrigins []string

	// MaxMessageRate is the maximum number of messages per second per client.
	MaxMessageRate int

	// MaxMessageSize is the maximum message size in bytes.
	MaxMessageSize int64

	// RequireAuth indicates whether authentication is required for WebSocket connections.
	RequireAuth bool

	// AuthConfig is the authentication configuration to use.
	AuthConfig AuthConfig
}

// DefaultWebSocketSecurityConfig returns a secure default configuration.
func DefaultWebSocketSecurityConfig() WebSocketSecurityConfig {
	return WebSocketSecurityConfig{
		AllowedOrigins: []string{"*"}, // Override in production
		MaxMessageRate: 10,             // 10 messages per second
		MaxMessageSize: 4096,           // 4KB max message size
		RequireAuth:    false,          // Set to true in production
	}
}

// secureUpgrader is a WebSocket upgrader with security measures.
var secureUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     nil, // Set dynamically via SecurityConfig
}

// WebSocketRateLimiter tracks message rates per client.
type WebSocketRateLimiter struct {
	clients map[*Client]*messageRateBucket
	mu      sync.RWMutex
}

// messageRateBucket implements a token bucket for message rate limiting.
type messageRateBucket struct {
	tokens         float64
	capacity       float64
	refillRate     float64 // tokens per second
	lastRefillTime time.Time
	mu             sync.Mutex
}

// NewWebSocketRateLimiter creates a new WebSocket rate limiter.
func NewWebSocketRateLimiter() *WebSocketRateLimiter {
	return &WebSocketRateLimiter{
		clients: make(map[*Client]*messageRateBucket),
	}
}

// newMessageRateBucket creates a new message rate bucket.
func newMessageRateBucket(messagesPerSecond int) *messageRateBucket {
	capacity := float64(messagesPerSecond) * 2.0 // Allow burst of 2x
	refillRate := float64(messagesPerSecond)

	return &messageRateBucket{
		tokens:         capacity,
		capacity:       capacity,
		refillRate:     refillRate,
		lastRefillTime: time.Now(),
	}
}

// allow checks if a message can be allowed (returns true if token available).
func (mb *messageRateBucket) allow() bool {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(mb.lastRefillTime).Seconds()

	// Refill tokens based on time elapsed
	mb.tokens = minFloat(mb.capacity, mb.tokens+elapsed*mb.refillRate)
	mb.lastRefillTime = now

	// Check if we have a token available
	if mb.tokens >= 1.0 {
		mb.tokens--
		return true
	}

	return false
}

// Register registers a client for rate limiting.
func (rl *WebSocketRateLimiter) Register(client *Client, messagesPerSecond int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.clients[client] = newMessageRateBucket(messagesPerSecond)
}

// Unregister removes a client from rate limiting.
func (rl *WebSocketRateLimiter) Unregister(client *Client) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.clients, client)
}

// Allow checks if a message from the client should be allowed.
func (rl *WebSocketRateLimiter) Allow(client *Client) bool {
	rl.mu.RLock()
	bucket, exists := rl.clients[client]
	rl.mu.RUnlock()

	if !exists {
		// If not registered, deny by default
		return false
	}

	return bucket.allow()
}

// isOriginAllowed checks if the origin is in the allowed list.
// Supports exact matches and wildcard "*".
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	// If no origin header, deny (browsers always send Origin for WebSocket)
	if origin == "" {
		return false
	}

	// Check each allowed origin pattern
	for _, allowed := range allowedOrigins {
		// Wildcard allows all (not recommended for production)
		if allowed == "*" {
			return true
		}

		// Exact match
		if origin == allowed {
			return true
		}

		// Support wildcard subdomains: *.example.com
		// SECURITY: Must check for ".domain" suffix to prevent attackerexample.com matching *.example.com
		if strings.HasPrefix(allowed, "*.") {
			domain := allowed[1:] // Remove "*" but keep the leading "." (becomes ".example.com")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}

	return false
}

// CheckOriginWithConfig creates a CheckOrigin function based on security config.
func CheckOriginWithConfig(config WebSocketSecurityConfig) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		allowed := isOriginAllowed(origin, config.AllowedOrigins)
		if !allowed {
			log.Printf("[WEBSOCKET] Rejected connection from origin: %s (not in allowed list)", origin)
		}

		return allowed
	}
}

// ValidateAuthForWebSocket checks authentication before WebSocket upgrade.
// Returns an error message if authentication fails, empty string if success.
func ValidateAuthForWebSocket(r *http.Request, config WebSocketSecurityConfig) string {
	// If auth not required, allow
	if !config.RequireAuth {
		return ""
	}

	// Check if auth is configured properly
	if !config.AuthConfig.Enabled {
		return "Authentication required but not configured"
	}

	// Check for API key
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		// Also check query parameter as fallback for WebSocket (some clients can't set headers easily)
		apiKey = r.URL.Query().Get("api_key")
		if apiKey == "" {
			return "Missing API key (X-API-Key header or api_key query parameter)"
		}
	}

	// Validate API key using constant-time comparison
	if !constantTimeCompare(apiKey, config.AuthConfig.APIKey) {
		return "Invalid API key"
	}

	return ""
}

// SecureWebSocketHandler creates a secure WebSocket handler with all security measures applied.
func SecureWebSocketHandler(hub *Hub, config WebSocketSecurityConfig, rateLimiter *WebSocketRateLimiter) http.HandlerFunc {
	// Configure the upgrader with the security settings
	secureUpgrader.CheckOrigin = CheckOriginWithConfig(config)

	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Validate authentication before upgrade
		if authError := ValidateAuthForWebSocket(r, config); authError != "" {
			log.Printf("[WEBSOCKET] Authentication failed: %s from %s", authError, getClientIP(r))
			http.Error(w, fmt.Sprintf("Unauthorized: %s", authError), http.StatusUnauthorized)
			return
		}

		// 2. Upgrade connection (includes origin validation)
		conn, err := secureUpgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[WEBSOCKET] Upgrade failed: %v", err)
			return
		}

		// 3. Set message size limit
		conn.SetReadLimit(config.MaxMessageSize)

		// 4. Create client
		client := &Client{
			hub:  hub,
			conn: conn,
			send: make(chan []byte, 256),
		}

		// 5. Register client for message rate limiting
		rateLimiter.Register(client, config.MaxMessageRate)

		// 6. Register with hub
		hub.register <- client

		// 7. Log secure connection
		log.Printf("[WEBSOCKET] Secure connection established from %s (origin: %s)",
			getClientIP(r), r.Header.Get("Origin"))

		// 8. Start client pumps with rate limiting
		go client.secureWritePump()
		go client.secureReadPump(rateLimiter)
	}
}

// secureReadPump reads messages with rate limiting.
func (c *Client) secureReadPump(rateLimiter *WebSocketRateLimiter) {
	defer func() {
		rateLimiter.Unregister(c)
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WEBSOCKET] Unexpected close: %v", err)
			}
			break
		}

		// Apply rate limiting to incoming messages
		if !rateLimiter.Allow(c) {
			log.Printf("[WEBSOCKET] Message rate limit exceeded, closing connection")
			c.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Rate limit exceeded"))
			break
		}

		// Log message for security audit (in production, you might want to sample this)
		log.Printf("[WEBSOCKET] Received message (%d bytes)", len(message))

		// Note: Current implementation is broadcast-only, so client messages are ignored.
		// If you need to handle client messages, process them here.
	}
}

// secureWritePump writes messages with proper error handling.
func (c *Client) secureWritePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Flush any additional queued messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// minFloat returns the minimum of two float64 values.
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
