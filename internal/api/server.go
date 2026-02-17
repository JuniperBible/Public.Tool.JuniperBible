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

	// Validate authentication configuration
	if err := ValidateAuthConfig(cfg.Auth); err != nil {
		return fmt.Errorf("invalid auth config: %w", err)
	}

	// Validate TLS configuration if enabled
	if cfg.TLS.Enabled {
		if cfg.TLS.CertFile == "" || cfg.TLS.KeyFile == "" {
			return fmt.Errorf("TLS enabled but cert or key file not specified")
		}
		// Verify TLS files exist
		if _, err := os.Stat(cfg.TLS.CertFile); err != nil {
			return fmt.Errorf("TLS cert file not found: %w", err)
		}
		if _, err := os.Stat(cfg.TLS.KeyFile); err != nil {
			return fmt.Errorf("TLS key file not found: %w", err)
		}
	}

	// Ensure capsules directory exists
	if err := os.MkdirAll(ServerConfig.CapsulesDir, 0700); err != nil {
		return fmt.Errorf("failed to create capsules directory: %w", err)
	}

	// Initialize WebSocket hub
	GlobalHub = NewHub()
	go GlobalHub.Run()

	// Setup routes
	mux := setupRoutes()

	// Configure plugins and log startup info
	server.EnablePlugins(ServerConfig.PluginsExternal, ServerConfig.PluginsDir)

	// Configure plugin security - restrict to plugins directory if external plugins enabled
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
	} else {
		logging.SecurityEvent("plugin_security_configured", "api",
			"mode", "permissive",
			"note", "embedded plugins only")
	}

	// Log server startup with appropriate protocol
	protocol := "http"
	wsProtocol := "ws"
	if cfg.TLS.Enabled {
		protocol = "https"
		wsProtocol = "wss"
		logging.Info("TLS enabled", "cert_file", cfg.TLS.CertFile)
	} else {
		logging.Warn("TLS disabled - using plain HTTP",
			"recommendation", "consider using TLS or reverse proxy for production")
	}
	logging.ServerStartup("rest_api", protocol, ServerConfig.Port,
		"websocket_protocol", wsProtocol,
		"capsules_dir", server.AbsPath(ServerConfig.CapsulesDir))

	// Build middleware chain with security headers
	cspConfig := server.APICSPConfig()
	var handler http.Handler = server.SecurityHeadersWithCSP(cspConfig, mux)

	// Apply authentication middleware if configured
	if cfg.Auth.Enabled {
		handler = AuthMiddleware(cfg.Auth, handler)
		logging.SecurityEvent("authentication_configured", "api",
			"enabled", true,
			"note", "API key required")
	} else {
		logging.SecurityEvent("authentication_configured", "api",
			"enabled", false,
			"note", "all requests allowed")
	}

	// Apply rate limiting if configured
	if cfg.RateLimitRequests > 0 {
		rateLimitConfig := RateLimiterConfig{
			RequestsPerMinute: cfg.RateLimitRequests,
			BurstSize:         cfg.RateLimitBurst,
		}
		if rateLimitConfig.BurstSize == 0 {
			rateLimitConfig.BurstSize = 10 // Default burst size
		}
		rateLimiter := NewRateLimiter(rateLimitConfig)
		handler = rateLimiter.Middleware(handler)
		logging.Info("rate limiting enabled",
			"requests_per_minute", rateLimitConfig.RequestsPerMinute,
			"burst_size", rateLimitConfig.BurstSize)
	}

	// Apply CORS middleware (outermost)
	corsConfig := server.CORSConfig{
		AllowedOrigins: cfg.AllowedOrigins,
	}
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

	// Apply logging middleware
	handler = logging.CombinedMiddleware(handler)

	// Start server with or without TLS
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
