package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultCSPConfig(t *testing.T) {
	cfg := DefaultCSPConfig()

	if len(cfg.DefaultSrc) != 1 || cfg.DefaultSrc[0] != "'self'" {
		t.Errorf("DefaultSrc should be ['self'], got %v", cfg.DefaultSrc)
	}

	if len(cfg.FrameAncestors) != 1 || cfg.FrameAncestors[0] != "'none'" {
		t.Errorf("FrameAncestors should be ['none'], got %v", cfg.FrameAncestors)
	}
}

func TestWebUICSPConfig(t *testing.T) {
	cfg := WebUICSPConfig()

	if len(cfg.ScriptSrc) != 1 || cfg.ScriptSrc[0] != "'self'" {
		t.Errorf("ScriptSrc should be ['self'], got %v", cfg.ScriptSrc)
	}

	if len(cfg.ImgSrc) != 2 {
		t.Errorf("ImgSrc should have 2 entries, got %d", len(cfg.ImgSrc))
	}
}

func TestAPICSPConfig(t *testing.T) {
	cfg := APICSPConfig()

	if len(cfg.DefaultSrc) != 1 || cfg.DefaultSrc[0] != "'none'" {
		t.Errorf("API DefaultSrc should be ['none'], got %v", cfg.DefaultSrc)
	}
}

func TestBuildCSPHeader(t *testing.T) {
	tests := []struct {
		name     string
		cfg      CSPConfig
		expected string
	}{
		{
			name: "simple config",
			cfg: CSPConfig{
				DefaultSrc: []string{"'self'"},
				ScriptSrc:  []string{"'self'"},
			},
			expected: "default-src 'self'; script-src 'self'",
		},
		{
			name: "with upgrade-insecure-requests",
			cfg: CSPConfig{
				DefaultSrc:              []string{"'self'"},
				UpgradeInsecureRequests: true,
			},
			expected: "default-src 'self'; upgrade-insecure-requests",
		},
		{
			name: "multiple sources",
			cfg: CSPConfig{
				DefaultSrc: []string{"'self'"},
				ImgSrc:     []string{"'self'", "data:", "https://example.com"},
			},
			expected: "default-src 'self'; img-src 'self' data: https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.BuildCSPHeader()
			if result != tt.expected {
				t.Errorf("Expected CSP header:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestCSPMiddleware(t *testing.T) {
	cfg := CSPConfig{
		DefaultSrc: []string{"'self'"},
		ScriptSrc:  []string{"'self'"},
	}

	handler := CSPMiddleware(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	expected := "default-src 'self'; script-src 'self'"

	if csp != expected {
		t.Errorf("Expected CSP header '%s', got '%s'", expected, csp)
	}
}

func TestSecurityHeadersWithCSP(t *testing.T) {
	cfg := WebUICSPConfig()

	handler := SecurityHeadersWithCSP(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check all security headers are present
	headers := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Referrer-Policy",
		"Content-Security-Policy",
	}

	for _, header := range headers {
		if w.Header().Get(header) == "" {
			t.Errorf("Expected header '%s' to be set", header)
		}
	}

	// Verify specific values
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("X-Frame-Options should be DENY")
	}

	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("X-Content-Type-Options should be nosniff")
	}
}

func TestSanitizeHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "<script>alert('xss')</script>",
			expected: "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			input:    "Hello <b>World</b>",
			expected: "Hello &lt;b&gt;World&lt;/b&gt;",
		},
		{
			input:    `<img src="x" onerror="alert('xss')">`,
			expected: "&lt;img src=&#34;x&#34; onerror=&#34;alert(&#39;xss&#39;)&#34;&gt;",
		},
		{
			input:    "Plain text",
			expected: "Plain text",
		},
	}

	for _, tt := range tests {
		result := SanitizeHTML(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeHTML(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "javascript:alert('xss')",
			expected: "",
		},
		{
			input:    "JAVASCRIPT:alert('xss')",
			expected: "",
		},
		{
			input:    "https://example.com",
			expected: "https://example.com",
		},
		{
			input:    "http://example.com",
			expected: "http://example.com",
		},
		{
			input:    "/relative/path",
			expected: "/relative/path",
		},
		{
			input:    "./relative/path",
			expected: "./relative/path",
		},
		{
			input:    "../relative/path",
			expected: "../relative/path",
		},
		{
			input:    "/path/with/javascript:inurl",
			expected: "",
		},
		{
			input:    "https://example.com/with/javascript:inurl",
			expected: "",
		},
		{
			input:    "file:///etc/passwd",
			expected: "",
		},
		{
			input:    "ftp://example.com",
			expected: "",
		},
		{
			input:    "",
			expected: "",
		},
		{
			input:    "  https://example.com  ",
			expected: "https://example.com",
		},
	}

	for _, tt := range tests {
		result := SanitizeURL(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeURL(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestValidateAlphanumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"valid-name_123", true},
		{"ValidName", true},
		{"123", true},
		{"name with spaces", false},
		{"name/with/slashes", false},
		{"", false},
		{"name@domain", false},
		{"_underscore", true},
		{"-hyphen", true},
	}

	for _, tt := range tests {
		result := ValidateAlphanumeric(tt.input)
		if result != tt.expected {
			t.Errorf("ValidateAlphanumeric(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"validName", true},
		{"_validName", true},
		{"valid_name_123", true},
		{"valid-name", true},
		{"123invalid", false}, // starts with number
		{"-invalid", false},   // starts with hyphen
		{"", false},
		{"a", true},
		{"name with spaces", false},
		{"name@domain", false},
	}

	for _, tt := range tests {
		result := ValidateIdentifier(tt.input)
		if result != tt.expected {
			t.Errorf("ValidateIdentifier(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestSanitizeUserInput(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "  normal text  ",
			expected: "normal text",
		},
		{
			input:    "text\x00with\x00nulls",
			expected: "textwithnulls",
		},
		{
			input:    "text\nwith\nnewlines",
			expected: "text\nwith\nnewlines",
		},
		{
			input:    "text\twith\ttabs",
			expected: "text\twith\ttabs",
		},
		{
			input:    "text\x01with\x02control",
			expected: "textwithcontrol",
		},
	}

	for _, tt := range tests {
		result := SanitizeUserInput(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeUserInput(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestLimitStringLength(t *testing.T) {
	tests := []struct {
		input     string
		maxLength int
		expected  string
	}{
		{"short", 10, "short"},
		{"exactly ten!", 12, "exactly ten!"},
		{"this is too long", 10, "this is to"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := LimitStringLength(tt.input, tt.maxLength)
		if result != tt.expected {
			t.Errorf("LimitStringLength(%q, %d) = %q, want %q", tt.input, tt.maxLength, result, tt.expected)
		}
	}
}

func TestValidateContentType(t *testing.T) {
	allowed := []string{"application/json", "text/plain", "application/xml"}

	tests := []struct {
		contentType string
		expected    bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"text/plain", true},
		{"text/plain; charset=utf-8", true},
		{"application/xml", true},
		{"text/html", false},
		{"application/javascript", false},
		{"", false},
	}

	for _, tt := range tests {
		result := ValidateContentType(tt.contentType, allowed)
		if result != tt.expected {
			t.Errorf("ValidateContentType(%q) = %v, want %v", tt.contentType, result, tt.expected)
		}
	}
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "<p>Hello <b>World</b></p>",
			expected: "Hello World",
		},
		{
			input:    "Plain text",
			expected: "Plain text",
		},
		{
			input:    "<script>alert('xss')</script>",
			expected: "alert('xss')",
		},
		{
			input:    "Text with &lt;escaped&gt; entities",
			expected: "Text with <escaped> entities",
		},
		{
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		result := StripHTMLTags(tt.input)
		if result != tt.expected {
			t.Errorf("StripHTMLTags(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestSanitizeHTMLAttribute(t *testing.T) {
	tests := []struct {
		input       string
		shouldBlock bool
	}{
		{
			input:       "normal value",
			shouldBlock: false,
		},
		{
			input:       "javascript:alert('xss')",
			shouldBlock: true,
		},
		{
			input:       "data:text/html,<script>alert('xss')</script>",
			shouldBlock: true,
		},
	}

	for _, tt := range tests {
		result := SanitizeHTMLAttribute(tt.input)
		isEmpty := result == ""
		if isEmpty != tt.shouldBlock {
			t.Errorf("SanitizeHTMLAttribute(%q): blocked=%v, want blocked=%v", tt.input, isEmpty, tt.shouldBlock)
		}
	}
}

func TestSanitizeQueryParam(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "normal value",
			expected: "normal value",
		},
		{
			input:    "  leading/trailing  ",
			expected: "leading/trailing",
		},
		{
			input:    "text\x00with\x00nulls",
			expected: "textwithnulls",
		},
		{
			input:    "<script>alert('xss')</script>",
			expected: "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			input:    "text\x01with\x02control",
			expected: "textwithcontrol",
		},
		{
			input:    "text & special <chars>",
			expected: "text &amp; special &lt;chars&gt;",
		},
	}

	for _, tt := range tests {
		result := SanitizeQueryParam(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeQueryParam(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
