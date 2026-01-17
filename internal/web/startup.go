// Package web provides the Juniper Bible web UI server.
package web

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleStartupStatus returns the current startup status as JSON.
func handleStartupStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := GetStartupStatus()
	json.NewEncoder(w).Encode(status)
}

// SplashMiddleware serves the splash screen during startup warmup.
// Once warmup is complete, it passes requests through to the next handler.
func SplashMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always allow API requests through (including startup status polling)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// Always allow static file requests through (for splash screen assets if any)
		if strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}

		// Check if startup is complete
		if IsStartupReady() {
			next.ServeHTTP(w, r)
			return
		}

		// Serve splash screen during warmup
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		err := Templates.ExecuteTemplate(w, "splash.html", nil)
		if err != nil {
			// Fallback if template fails
			http.Error(w, "Starting up...", http.StatusServiceUnavailable)
		}
	})
}
