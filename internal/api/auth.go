package api

import (
	"crypto/subtle"
	"fmt"
	"net/http"

	"github.com/JuniperBible/juniper/internal/logging"
)

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Enabled bool
	APIKey  string
}

// AuthMiddleware checks for API key authentication when enabled.
// If auth is disabled, all requests pass through.
// If auth is enabled, requests must include X-API-Key header with the correct key.
// Health endpoints (/, /health) always bypass authentication.
func AuthMiddleware(authCfg AuthConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for public endpoints
		if isPublicEndpoint(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// If auth is disabled, allow all requests
		if !authCfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Auth is enabled - check for API key
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			logging.SecurityEvent("unauthorized_request", "auth",
				"path", r.URL.Path,
				"reason", "missing API key")
			respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing X-API-Key header")
			return
		}

		// SEC-003 FIX: Use constant-time comparison to prevent timing attacks
		if !constantTimeCompare(apiKey, authCfg.APIKey) {
			logging.SecurityEvent("unauthorized_request", "auth",
				"path", r.URL.Path,
				"reason", "invalid API key")
			respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid API key")
			return
		}

		// Valid API key - proceed
		next.ServeHTTP(w, r)
	})
}

// isPublicEndpoint returns true if the endpoint should always be accessible
// without authentication (health checks, root info).
func isPublicEndpoint(path string) bool {
	publicPaths := []string{
		"/",
		"/health",
	}

	for _, publicPath := range publicPaths {
		if path == publicPath {
			return true
		}
	}

	return false
}

// ValidateAuthConfig validates the authentication configuration.
func ValidateAuthConfig(cfg AuthConfig) error {
	if cfg.Enabled && cfg.APIKey == "" {
		return fmt.Errorf("API key is required when authentication is enabled")
	}
	if cfg.Enabled && len(cfg.APIKey) < 16 {
		return fmt.Errorf("API key must be at least 16 characters (got %d)", len(cfg.APIKey))
	}
	return nil
}

// GenerateAPIKeyExample returns an example API key format.
func GenerateAPIKeyExample() string {
	return "Example: export CAPSULE_API_KEY=$(openssl rand -base64 32)"
}

// constantTimeCompare performs a constant-time comparison of two strings.
// This prevents timing attacks by ensuring the comparison always takes
// the same amount of time regardless of where the strings differ.
func constantTimeCompare(a, b string) bool {
	// subtle.ConstantTimeCompare requires equal length inputs
	// If lengths differ, still use constant-time comparison with equal-length slices
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
