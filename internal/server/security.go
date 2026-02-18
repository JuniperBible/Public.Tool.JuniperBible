// Package server provides security utilities for HTTP servers.
package server

import (
	"html"
	"net/http"
	"regexp"
	"strings"
)

// CSPConfig holds Content-Security-Policy configuration.
type CSPConfig struct {
	// DefaultSrc specifies default source for all directives
	DefaultSrc []string
	// ScriptSrc specifies valid sources for JavaScript
	ScriptSrc []string
	// StyleSrc specifies valid sources for CSS
	StyleSrc []string
	// ImgSrc specifies valid sources for images
	ImgSrc []string
	// FontSrc specifies valid sources for fonts
	FontSrc []string
	// ConnectSrc specifies valid sources for fetch, XMLHttpRequest, WebSocket
	ConnectSrc []string
	// FrameAncestors specifies valid parents that may embed the page
	FrameAncestors []string
	// BaseURI restricts URLs that can be used in <base> element
	BaseURI []string
	// FormAction restricts URLs that can be used as form action targets
	FormAction []string
	// UpgradeInsecureRequests forces HTTPS
	UpgradeInsecureRequests bool
}

// DefaultCSPConfig returns a secure default CSP configuration.
// This configuration:
// - Allows resources only from same origin ('self')
// - Allows data: URIs for images (needed for inline SVG/base64 images)
// - Blocks all frame embedding (clickjacking protection)
// - Restricts base URI and form actions to same origin
func DefaultCSPConfig() CSPConfig {
	return CSPConfig{
		DefaultSrc:              []string{"'self'"},
		ScriptSrc:               []string{"'self'"},
		StyleSrc:                []string{"'self'"},
		ImgSrc:                  []string{"'self'", "data:"},
		FontSrc:                 []string{"'self'"},
		ConnectSrc:              []string{"'self'"},
		FrameAncestors:          []string{"'none'"},
		BaseURI:                 []string{"'self'"},
		FormAction:              []string{"'self'"},
		UpgradeInsecureRequests: false, // Set to true in production with HTTPS
	}
}

// WebUICSPConfig returns a CSP configuration suitable for the web UI.
// This is more permissive than the API CSP to support dynamic web features.
func WebUICSPConfig() CSPConfig {
	return CSPConfig{
		DefaultSrc:              []string{"'self'"},
		ScriptSrc:               []string{"'self'"},
		StyleSrc:                []string{"'self'"},
		ImgSrc:                  []string{"'self'", "data:"},
		FontSrc:                 []string{"'self'"},
		ConnectSrc:              []string{"'self'"},
		FrameAncestors:          []string{"'none'"},
		BaseURI:                 []string{"'self'"},
		FormAction:              []string{"'self'"},
		UpgradeInsecureRequests: false,
	}
}

// APICSPConfig returns a strict CSP configuration for REST API endpoints.
// APIs typically don't need to load resources, so this is very restrictive.
func APICSPConfig() CSPConfig {
	return CSPConfig{
		DefaultSrc:              []string{"'none'"},
		FrameAncestors:          []string{"'none'"},
		BaseURI:                 []string{"'none'"},
		FormAction:              []string{"'none'"},
		UpgradeInsecureRequests: false,
	}
}

func (cfg CSPConfig) cspSourceDirectives() []struct{ name string; values []string } {
	return []struct{ name string; values []string }{
		{"default-src", cfg.DefaultSrc},
		{"script-src", cfg.ScriptSrc},
		{"style-src", cfg.StyleSrc},
		{"img-src", cfg.ImgSrc},
		{"font-src", cfg.FontSrc},
		{"connect-src", cfg.ConnectSrc},
		{"frame-ancestors", cfg.FrameAncestors},
		{"base-uri", cfg.BaseURI},
		{"form-action", cfg.FormAction},
	}
}

func (cfg CSPConfig) BuildCSPHeader() string {
	var directives []string

	for _, d := range cfg.cspSourceDirectives() {
		if len(d.values) > 0 {
			directives = append(directives, d.name+" "+strings.Join(d.values, " "))
		}
	}

	if cfg.UpgradeInsecureRequests {
		directives = append(directives, "upgrade-insecure-requests")
	}

	return strings.Join(directives, "; ")
}

// CSPMiddleware adds Content-Security-Policy headers with custom configuration.
func CSPMiddleware(cfg CSPConfig, next http.Handler) http.Handler {
	cspHeader := cfg.BuildCSPHeader()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cspHeader != "" {
			w.Header().Set("Content-Security-Policy", cspHeader)
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeadersWithCSP adds comprehensive security headers including CSP.
// This combines the existing SecurityHeadersMiddleware with configurable CSP.
func SecurityHeadersWithCSP(cfg CSPConfig, next http.Handler) http.Handler {
	cspHeader := cfg.BuildCSPHeader()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Standard security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Configurable CSP
		if cspHeader != "" {
			w.Header().Set("Content-Security-Policy", cspHeader)
		}

		next.ServeHTTP(w, r)
	})
}

// SanitizeHTML sanitizes HTML content to prevent XSS attacks.
// It escapes HTML special characters: <, >, &, ", '
func SanitizeHTML(input string) string {
	return html.EscapeString(input)
}

// SanitizeHTMLAttribute sanitizes HTML attribute values.
// This is stricter than SanitizeHTML as it also handles quotes and other special cases.
func SanitizeHTMLAttribute(input string) string {
	// HTML escape first
	escaped := html.EscapeString(input)

	// Remove any potential event handlers or javascript: URIs
	escaped = removeJavaScriptURIs(escaped)

	return escaped
}

// removeJavaScriptURIs removes javascript: and data: URIs that could execute code.
func removeJavaScriptURIs(input string) string {
	lower := strings.ToLower(input)

	// Block javascript: URIs
	if strings.Contains(lower, "javascript:") {
		return ""
	}

	// Block data: URIs with script content
	if strings.Contains(lower, "data:") && strings.Contains(lower, "script") {
		return ""
	}

	return input
}

// SanitizeURL validates and sanitizes a URL to prevent injection attacks.
// It only allows http, https, and relative URLs.
func SanitizeURL(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	if isRelativeURL(input) {
		return sanitizeRelativeURL(input)
	}
	return sanitizeAbsoluteURL(input)
}

func isRelativeURL(input string) bool {
	return strings.HasPrefix(input, "/") || strings.HasPrefix(input, "./") || strings.HasPrefix(input, "../")
}

func sanitizeRelativeURL(input string) string {
	if containsJavascript(input) {
		return ""
	}
	return input
}

func sanitizeAbsoluteURL(input string) string {
	lower := strings.ToLower(input)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return ""
	}
	if containsJavascript(input) {
		return ""
	}
	return input
}

func containsJavascript(input string) bool {
	return strings.Contains(strings.ToLower(input), "javascript:")
}

// ValidateAlphanumeric checks if a string contains only alphanumeric characters, hyphens, and underscores.
// This is useful for validating identifiers like capsule names, plugin names, etc.
func ValidateAlphanumeric(input string) bool {
	if input == "" {
		return false
	}

	match, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, input)
	return match
}

// ValidateIdentifier validates that a string is a valid identifier.
// Identifiers must:
// - Start with a letter or underscore
// - Contain only letters, numbers, underscores, and hyphens
// - Be between 1 and 64 characters
func ValidateIdentifier(input string) bool {
	if len(input) == 0 || len(input) > 64 {
		return false
	}

	match, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_-]*$`, input)
	return match
}

// SanitizeUserInput performs general sanitization on user input.
// It trims whitespace and removes control characters.
func SanitizeUserInput(input string) string {
	// Trim whitespace
	input = strings.TrimSpace(input)

	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// Remove other control characters except newline and tab
	var result strings.Builder
	for _, r := range input {
		// Allow printable characters, newline, and tab
		if r >= 0x20 || r == '\n' || r == '\t' {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// LimitStringLength truncates a string to a maximum length.
// This helps prevent buffer overflow and DoS attacks.
func LimitStringLength(input string, maxLength int) string {
	if len(input) <= maxLength {
		return input
	}
	return input[:maxLength]
}

// SanitizeQueryParam sanitizes a query parameter value.
// It combines input sanitization with HTML escaping.
func SanitizeQueryParam(input string) string {
	// First sanitize general input
	sanitized := SanitizeUserInput(input)

	// Then HTML escape for safe output
	return html.EscapeString(sanitized)
}

// ValidateContentType checks if a Content-Type header is in the allowed list.
// This prevents content-type confusion attacks.
func ValidateContentType(contentType string, allowed []string) bool {
	// Extract just the media type, ignore parameters
	parts := strings.Split(contentType, ";")
	mediaType := strings.TrimSpace(parts[0])

	for _, allowedType := range allowed {
		if strings.EqualFold(mediaType, allowedType) {
			return true
		}
	}

	return false
}

// AllowedUploadContentTypes returns the list of allowed content types for file uploads.
var AllowedUploadContentTypes = []string{
	"application/zip",
	"application/x-tar",
	"application/gzip",
	"application/x-gzip",
	"application/x-xz",
	"application/xml",
	"text/xml",
	"application/json",
	"text/plain",
	"application/octet-stream", // Generic binary, validated by magic bytes
}

// StripHTMLTags removes all HTML tags from a string, leaving only text content.
// This is useful for displaying user input safely in contexts where HTML is not desired.
func StripHTMLTags(input string) string {
	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	stripped := re.ReplaceAllString(input, "")

	// Decode HTML entities
	stripped = html.UnescapeString(stripped)

	return stripped
}
