// Package api provides the Juniper Bible REST API server.
package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
	"github.com/FocuswithJustin/JuniperBible/internal/logging"
	"github.com/FocuswithJustin/JuniperBible/internal/server"
)

// Start starts the API server with the given configuration.
func Start(cfg Config) error {
	ServerConfig = cfg

	if err := ValidateAuthConfig(cfg.Auth); err != nil {
		return fmt.Errorf("invalid auth config: %w", err)
	}
	if err := validateTLSConfig(cfg.TLS); err != nil {
		return err
	}
	if err := os.MkdirAll(ServerConfig.CapsulesDir, 0700); err != nil {
		return fmt.Errorf("failed to create capsules directory: %w", err)
	}

	GlobalHub = NewHub()
	go GlobalHub.Run()

	mux := setupRoutes()
	server.EnablePlugins(ServerConfig.PluginsExternal, ServerConfig.PluginsDir)
	configurePluginSecurity()
	logProtocolInfo(cfg)

	handler := buildMiddlewareChain(cfg, mux)
	return listenAndServe(cfg, handler)
}

// validateTLSConfig checks that TLS files are specified and present when TLS is enabled.
func validateTLSConfig(tls TLSConfig) error {
	if !tls.Enabled {
		return nil
	}
	if tls.CertFile == "" || tls.KeyFile == "" {
		return fmt.Errorf("TLS enabled but cert or key file not specified")
	}
	if _, err := os.Stat(tls.CertFile); err != nil {
		return fmt.Errorf("TLS cert file not found: %w", err)
	}
	if _, err := os.Stat(tls.KeyFile); err != nil {
		return fmt.Errorf("TLS key file not found: %w", err)
	}
	return nil
}

// configurePluginSecurity sets up plugin security based on the current ServerConfig.
func configurePluginSecurity() {
	if ServerConfig.PluginsExternal && ServerConfig.PluginsDir != "" {
		pluginSecurityCfg := plugins.SecurityConfig{
			AllowedPluginDirs:    []string{ServerConfig.PluginsDir},
			RequireManifest:      true,
			RestrictToKnownKinds: true,
		}
		plugins.SetSecurityConfig(pluginSecurityCfg)
		logging.SecurityEvent("plugin_security_configured", "api",
			"mode", "restricted",
			"allowed_dir", server.AbsPath(ServerConfig.PluginsDir))
		return
	}
	logging.SecurityEvent("plugin_security_configured", "api",
		"mode", "permissive",
		"note", "embedded plugins only")
}

// logProtocolInfo logs startup details for the chosen protocol.
func logProtocolInfo(cfg Config) {
	protocol, wsProtocol := "http", "ws"
	if cfg.TLS.Enabled {
		protocol, wsProtocol = "https", "wss"
		logging.Info("TLS enabled", "cert_file", cfg.TLS.CertFile)
	} else {
		logging.Warn("TLS disabled - using plain HTTP",
			"recommendation", "consider using TLS or reverse proxy for production")
	}
	logging.ServerStartup("rest_api", protocol, ServerConfig.Port,
		"websocket_protocol", wsProtocol,
		"capsules_dir", server.AbsPath(ServerConfig.CapsulesDir))
}

// buildMiddlewareChain assembles the full middleware stack around mux.
func buildMiddlewareChain(cfg Config, mux *http.ServeMux) http.Handler {
	cspConfig := server.APICSPConfig()
	handler := http.Handler(server.SecurityHeadersWithCSP(cspConfig, mux))

	handler = applyAuthMiddleware(cfg, handler)
	handler = applyRateLimitMiddleware(cfg, handler)

	corsConfig := server.CORSConfig{AllowedOrigins: cfg.AllowedOrigins}
	handler = server.CORSMiddlewareWithConfig(corsConfig, handler)
	if len(cfg.AllowedOrigins) > 0 {
		logging.SecurityEvent("cors_configured", "api",
			"mode", "restricted",
			"allowed_origins_count", len(cfg.AllowedOrigins))
	} else {
		logging.SecurityEvent("cors_configured", "api",
			"mode", "permissive",
			"note", "allowing all origins (*) - consider restricting for production")
	}

	return logging.CombinedMiddleware(handler)
}

// applyAuthMiddleware wraps handler with authentication middleware when auth is enabled.
func applyAuthMiddleware(cfg Config, handler http.Handler) http.Handler {
	if cfg.Auth.Enabled {
		logging.SecurityEvent("authentication_configured", "api",
			"enabled", true,
			"note", "API key required")
		return AuthMiddleware(cfg.Auth, handler)
	}
	logging.SecurityEvent("authentication_configured", "api",
		"enabled", false,
		"note", "all requests allowed")
	return handler
}

// applyRateLimitMiddleware wraps handler with rate limiting middleware when configured.
func applyRateLimitMiddleware(cfg Config, handler http.Handler) http.Handler {
	if cfg.RateLimitRequests <= 0 {
		return handler
	}
	rateLimitConfig := RateLimiterConfig{
		RequestsPerMinute: cfg.RateLimitRequests,
		BurstSize:         cfg.RateLimitBurst,
	}
	if rateLimitConfig.BurstSize == 0 {
		rateLimitConfig.BurstSize = 10
	}
	rateLimiter := NewRateLimiter(rateLimitConfig)
	logging.Info("rate limiting enabled",
		"requests_per_minute", rateLimitConfig.RequestsPerMinute,
		"burst_size", rateLimitConfig.BurstSize)
	return rateLimiter.Middleware(handler)
}

// listenAndServe starts the HTTP or HTTPS server.
func listenAndServe(cfg Config, handler http.Handler) error {
	addr := fmt.Sprintf(":%d", ServerConfig.Port)
	if cfg.TLS.Enabled {
		return http.ListenAndServeTLS(addr, cfg.TLS.CertFile, cfg.TLS.KeyFile, handler)
	}
	return http.ListenAndServe(addr, handler)
}

// setupRoutes configures all HTTP routes.
func setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/capsules", handleCapsules)
	mux.HandleFunc("/capsules/", handleCapsuleByID)
	mux.HandleFunc("/convert", handleConvert)
	mux.HandleFunc("/plugins", handlePlugins)
	mux.HandleFunc("/formats", handleFormats)
	mux.HandleFunc("/ws", handleWebSocket)
	mux.HandleFunc("/jobs", handleJobs)
	mux.HandleFunc("/jobs/", handleJobByID)

	return mux
}
