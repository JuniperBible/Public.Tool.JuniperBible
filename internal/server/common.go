// Package server provides shared utilities for HTTP servers.
package server

import (
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// AbsPath returns the absolute path of a file, or the original path if it fails.
func AbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

// EnablePlugins configures external plugin loading and logs the result.
func EnablePlugins(enabled bool, pluginsDir string) {
	if enabled {
		plugins.EnableExternalPlugins()
		log.Printf("External plugins: ENABLED (loading from %s)", AbsPath(pluginsDir))
	} else {
		log.Printf("External plugins: disabled (using embedded plugins only)")
	}
}

// CORSConfig holds CORS middleware configuration.
type CORSConfig struct {
	AllowedOrigins []string // List of allowed origins, empty = allow all (*)
}

// CORSMiddleware adds CORS headers to responses.
// Deprecated: Use CORSMiddlewareWithConfig instead.
// This function maintains backward compatibility but allows all origins.
func CORSMiddleware(next http.Handler) http.Handler {
	return CORSMiddlewareWithConfig(CORSConfig{}, next)
}

// CORSMiddlewareWithConfig adds CORS headers to responses with configurable origins.
// If AllowedOrigins is empty, it defaults to "*" (allow all origins).
// If AllowedOrigins contains specific origins, it validates the request Origin header.
func CORSMiddlewareWithConfig(cfg CORSConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowedOrigin := determineAllowedOrigin(cfg, origin)

		if allowedOrigin == "" {
			handleUnallowedOrigin(w, r, next)
			return
		}

		setCORSHeaders(w, allowedOrigin)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// determineAllowedOrigin checks if origin is allowed and returns the allowed value.
func determineAllowedOrigin(cfg CORSConfig, origin string) string {
	if len(cfg.AllowedOrigins) == 0 {
		return "*"
	}
	for _, allowed := range cfg.AllowedOrigins {
		if origin == allowed {
			return origin
		}
	}
	return ""
}

// handleUnallowedOrigin handles requests from non-allowed origins.
func handleUnallowedOrigin(w http.ResponseWriter, r *http.Request, next http.Handler) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	next.ServeHTTP(w, r)
}

// setCORSHeaders sets the CORS headers on the response.
func setCORSHeaders(w http.ResponseWriter, allowedOrigin string) {
	w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
	if allowedOrigin != "*" {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
}

// SecurityHeadersMiddleware adds security headers to all responses.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		// CSP allows 'unsafe-inline' for scripts to support inline event handlers (onchange, onsubmit)
		// used in interactive components (chapter dropdown, theme toggle, dev menu)
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self'; img-src 'self' data:; font-src 'self'")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// TimingMiddleware logs request duration for profiling.
func TimingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		// Log slow requests (>100ms) with warning
		if duration > 100*time.Millisecond {
			log.Printf("[SLOW] %s %s took %v", r.Method, r.URL.Path, duration)
		} else {
			log.Printf("[TIME] %s %s took %v", r.Method, r.URL.Path, duration)
		}
	})
}
