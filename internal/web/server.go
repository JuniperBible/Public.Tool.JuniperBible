// Package web provides the Juniper Bible web UI server.
package web

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
	"github.com/FocuswithJustin/JuniperBible/internal/logging"
	"github.com/FocuswithJustin/JuniperBible/internal/server"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Templates is the parsed template set.
var Templates *template.Template

// Config holds server configuration.
type Config struct {
	Port            int
	CapsulesDir     string
	PluginsDir      string
	SwordDir        string
	PluginsExternal bool
	TLS             TLSConfig // TLS configuration
}

// TLSConfig holds TLS/HTTPS configuration.
type TLSConfig struct {
	Enabled  bool   // Enable HTTPS
	CertFile string // Path to TLS certificate file
	KeyFile  string // Path to TLS private key file
}

// ServerConfig is the active server configuration.
var ServerConfig Config

// Start starts the web server with the given configuration.
func Start(cfg Config) error {
	ServerConfig = cfg

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

	// Default SWORD directory to ~/.sword if not specified
	if ServerConfig.SwordDir == "" {
		if home, _ := os.UserHomeDir(); home != "" {
			ServerConfig.SwordDir = filepath.Join(home, ".sword")
		}
	}

	// Warn if capsules directory doesn't exist
	if _, err := os.Stat(ServerConfig.CapsulesDir); errors.Is(err, os.ErrNotExist) {
		logging.Warn("capsules directory does not exist", "path", ServerConfig.CapsulesDir)
	}

	// Parse templates with helper functions
	var err error
	Templates, err = template.New("").Funcs(templateFuncs()).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

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
		logging.SecurityEvent("plugin_security_configured", "web",
			"mode", "restricted",
			"allowed_dir", server.AbsPath(ServerConfig.PluginsDir))
	} else {
		logging.SecurityEvent("plugin_security_configured", "web",
			"mode", "permissive",
			"note", "embedded plugins only")
	}

	// Log server startup with appropriate protocol
	protocol := "http"
	if cfg.TLS.Enabled {
		protocol = "https"
		logging.Info("TLS enabled", "cert_file", cfg.TLS.CertFile)
	} else {
		logging.Warn("TLS disabled - using plain HTTP",
			"recommendation", "consider using TLS or reverse proxy for production")
	}
	logging.ServerStartup("web_ui", protocol, ServerConfig.Port,
		"capsules_dir", server.AbsPath(ServerConfig.CapsulesDir),
		"sword_dir", server.AbsPath(ServerConfig.SwordDir))

	// Initialize static file cache (must be done before serving requests)
	initStaticFileCache()

	// Pre-warm caches in background and start background refresh
	PreWarmCaches()
	StartBackgroundCacheRefresh()

	// Apply middleware chain: splash -> logging -> timing -> security headers with CSP
	// Splash middleware serves the splash screen during startup warmup
	cspConfig := server.WebUICSPConfig()
	handler := SplashMiddleware(logging.CombinedMiddleware(server.TimingMiddleware(server.SecurityHeadersWithCSP(cspConfig, mux))))

	// Start server with or without TLS
	addr := fmt.Sprintf(":%d", ServerConfig.Port)
	if cfg.TLS.Enabled {
		return http.ListenAndServeTLS(addr, cfg.TLS.CertFile, cfg.TLS.KeyFile, handler)
	}
	return http.ListenAndServe(addr, handler)
}

// cachedTemplateFuncs is initialized once at package load time.
var cachedTemplateFuncs = template.FuncMap{
	"iterate": func(n int) []int {
		result := make([]int, n)
		for i := range result {
			result[i] = i
		}
		return result
	},
	"add": func(a, b int) int {
		return a + b
	},
	"subtract": func(a, b int) int {
		return a - b
	},
	"truncate": func(s string, n int) string {
		if len(s) <= n {
			return s
		}
		return s[:n] + "..."
	},
	"escapeJS": func(s string) string {
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "'", "\\'")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		s = strings.ReplaceAll(s, "\n", "\\n")
		s = strings.ReplaceAll(s, "\r", "\\r")
		s = strings.ReplaceAll(s, "<", "\\u003c")
		s = strings.ReplaceAll(s, ">", "\\u003e")
		return s
	},
	// dict creates a map from key-value pairs for passing to templates.
	// Usage: {{template "name" dict "key1" val1 "key2" val2}}
	"dict": func(values ...any) map[string]any {
		if len(values)%2 != 0 {
			return nil
		}
		m := make(map[string]any, len(values)/2)
		for i := 0; i < len(values); i += 2 {
			key, ok := values[i].(string)
			if !ok {
				continue
			}
			m[key] = values[i+1]
		}
		return m
	},
}

// templateFuncs returns the cached template helper functions.
func templateFuncs() template.FuncMap {
	return cachedTemplateFuncs
}

// setupRoutes configures all HTTP routes.
func setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Core routes
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/capsules", handleCapsules)
	mux.HandleFunc("/capsules/delete", handleCapsuleDelete)
	mux.HandleFunc("/capsule/", handleCapsule)
	mux.HandleFunc("/artifact/", handleArtifact)
	mux.HandleFunc("/ir/", handleIR)
	mux.HandleFunc("/transcript/", handleTranscript)
	mux.HandleFunc("/plugins", handlePluginsRedirect) // Legacy redirect to /juniper?tab=plugins
	mux.HandleFunc("/convert", handleConvert)
	mux.HandleFunc("/static/", handleStatic)

	// Capsule operations
	mux.HandleFunc("/ingest", handleIngest)
	mux.HandleFunc("/verify/", handleVerify)
	mux.HandleFunc("/detect", handleDetectRedirect) // Legacy redirect to /juniper?tab=detect
	mux.HandleFunc("/export/", handleExport)
	mux.HandleFunc("/dev", handleDevInfo)
	mux.HandleFunc("/selfcheck/", handleSelfcheck)
	mux.HandleFunc("/runs/compare/", handleRunsCompare)
	mux.HandleFunc("/runs/", handleRuns)
	mux.HandleFunc("/tools", handleTools)
	mux.HandleFunc("/tools/run", handleToolRun)

	// SWORD/Juniper routes
	mux.HandleFunc("/sword", handleSWORDBrowser)
	mux.HandleFunc("/juniper/ingest", handleJuniperRedirect)  // Legacy redirect
	mux.HandleFunc("/juniper/repoman", handleJuniperRedirect) // Legacy redirect
	mux.HandleFunc("/juniper", handleJuniper)

	// Bible browsing
	mux.HandleFunc("/bible/compare", handleBibleCompare)
	mux.HandleFunc("/bible/search", handleBibleSearch)
	mux.HandleFunc("/bible/", handleBibleIndex)
	mux.HandleFunc("/bible", handleBibleIndex)

	// Library (multi-Bible browse/search/compare)
	mux.HandleFunc("/library/bibles/install", handleBibleInstall)
	mux.HandleFunc("/library/bibles/delete", handleBibleDelete)
	mux.HandleFunc("/library/bibles/", handleLibraryBibles)
	mux.HandleFunc("/library/bibles", handleLibraryBibles)

	// Task queue API
	mux.HandleFunc("/api/tasks/add", handleTaskAdd)
	mux.HandleFunc("/api/tasks/status", handleTaskStatus)
	mux.HandleFunc("/api/tasks/clear", handleTaskClear)

	// Startup status API (for splash screen)
	mux.HandleFunc("/api/startup/status", handleStartupStatus)

	// Bible API
	mux.HandleFunc("/api/bibles/search", handleAPIBibleSearch)
	mux.HandleFunc("/api/bibles/", handleAPIBibles)
	mux.HandleFunc("/api/bibles", handleAPIBibles)

	return mux
}
