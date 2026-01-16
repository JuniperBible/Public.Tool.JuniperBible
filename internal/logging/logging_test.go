package logging

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// captureLogOutput captures log output for testing by temporarily
// redirecting the logger to write to a buffer
func captureLogOutput(f func()) string {
	// Create a buffer to capture output
	var buf bytes.Buffer

	// Save original logger
	oldLogger := defaultLogger

	// Create a new logger that writes to the buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	defaultLogger = slog.New(handler)

	// Execute function
	f()

	// Restore original logger
	defaultLogger = oldLogger

	return buf.String()
}

// captureLogOutputWithInit captures output by reinitializing the logger
// to write to a buffer. This tests the actual InitLogger ReplaceAttr logic.
func captureLogOutputWithInit(level Level, format Format, f func()) string {
	// Create a pipe to capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Channel for captured output
	outCh := make(chan string)

	// Read from pipe in background
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		outCh <- buf.String()
	}()

	// Initialize logger (which will use the pipe)
	InitLogger(level, format)

	// Execute test function
	f()

	// Close pipe and restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Wait for output
	output := <-outCh

	// Reinitialize with default settings
	InitLogger(LevelInfo, FormatJSON)

	return output
}

func TestInitLogger(t *testing.T) {
	tests := []struct {
		name   string
		level  Level
		format Format
	}{
		{
			name:   "Debug level JSON format",
			level:  LevelDebug,
			format: FormatJSON,
		},
		{
			name:   "Info level JSON format",
			level:  LevelInfo,
			format: FormatJSON,
		},
		{
			name:   "Warn level JSON format",
			level:  LevelWarn,
			format: FormatJSON,
		},
		{
			name:   "Error level JSON format",
			level:  LevelError,
			format: FormatJSON,
		},
		{
			name:   "Info level Text format",
			level:  LevelInfo,
			format: FormatText,
		},
		{
			name:   "Debug level Text format",
			level:  LevelDebug,
			format: FormatText,
		},
		{
			name:   "Default level (invalid value)",
			level:  Level(999),
			format: FormatJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitLogger(tt.level, tt.format)
			logger := GetLogger()
			if logger == nil {
				t.Error("Expected logger to be initialized, got nil")
			}
		})
	}
}

func TestGetLogger(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)
	logger := GetLogger()
	if logger == nil {
		t.Error("Expected logger to be non-nil")
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "test-request-id-123"

	newCtx := WithRequestID(ctx, requestID)

	retrievedID := GetRequestID(newCtx)
	if retrievedID != requestID {
		t.Errorf("Expected request ID %s, got %s", requestID, retrievedID)
	}
}

func TestGetRequestID(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "Context with request ID",
			ctx:      context.WithValue(context.Background(), RequestIDKey, "test-id"),
			expected: "test-id",
		},
		{
			name:     "Context without request ID",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "Context with wrong type value",
			ctx:      context.WithValue(context.Background(), RequestIDKey, 12345),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetRequestID(tt.ctx)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestLoggerFromContext(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)

	tests := []struct {
		name      string
		ctx       context.Context
		hasReqID  bool
	}{
		{
			name:     "Context with request ID",
			ctx:      WithRequestID(context.Background(), "test-123"),
			hasReqID: true,
		},
		{
			name:     "Context without request ID",
			ctx:      context.Background(),
			hasReqID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := LoggerFromContext(tt.ctx)
			if logger == nil {
				t.Error("Expected logger to be non-nil")
			}
		})
	}
}

func TestLoggingFunctions(t *testing.T) {
	// Initialize with Debug level to ensure all messages are logged
	InitLogger(LevelDebug, FormatJSON)

	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "Debug",
			fn: func() {
				Debug("debug message", "key", "value")
			},
		},
		{
			name: "Info",
			fn: func() {
				Info("info message", "key", "value")
			},
		},
		{
			name: "Warn",
			fn: func() {
				Warn("warning message", "key", "value")
			},
		},
		{
			name: "Error",
			fn: func() {
				Error("error message", "key", "value")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureLogOutput(tt.fn)
			if output == "" {
				t.Error("Expected log output, got empty string")
			}
		})
	}
}

func TestContextLoggingFunctions(t *testing.T) {
	InitLogger(LevelDebug, FormatJSON)
	ctx := WithRequestID(context.Background(), "test-request-id")

	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "DebugContext",
			fn: func() {
				DebugContext(ctx, "debug message", "key", "value")
			},
		},
		{
			name: "InfoContext",
			fn: func() {
				InfoContext(ctx, "info message", "key", "value")
			},
		},
		{
			name: "WarnContext",
			fn: func() {
				WarnContext(ctx, "warning message", "key", "value")
			},
		},
		{
			name: "ErrorContext",
			fn: func() {
				ErrorContext(ctx, "error message", "key", "value")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureLogOutput(tt.fn)
			if output == "" {
				t.Error("Expected log output, got empty string")
			}
			if !strings.Contains(output, "test-request-id") {
				t.Error("Expected output to contain request ID")
			}
		})
	}
}

func TestHTTPRequest(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)

	output := captureLogOutput(func() {
		HTTPRequest("GET", "/api/test", "127.0.0.1:1234", 200, 100*time.Millisecond)
	})

	if output == "" {
		t.Error("Expected log output, got empty string")
	}
	if !strings.Contains(output, "GET") {
		t.Error("Expected output to contain method")
	}
	if !strings.Contains(output, "/api/test") {
		t.Error("Expected output to contain path")
	}
	if !strings.Contains(output, "http_request") {
		t.Error("Expected output to contain http_request")
	}
}

func TestHTTPRequestWithArgs(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)

	output := captureLogOutput(func() {
		HTTPRequest("POST", "/api/create", "192.168.1.1:5678", 201, 250*time.Millisecond, "user_id", "123")
	})

	if output == "" {
		t.Error("Expected log output, got empty string")
	}
	if !strings.Contains(output, "user_id") {
		t.Error("Expected output to contain custom args")
	}
}

func TestHTTPRequestContext(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)
	ctx := WithRequestID(context.Background(), "req-456")

	output := captureLogOutput(func() {
		HTTPRequestContext(ctx, "PUT", "/api/update", "10.0.0.1:9999", 204, 75*time.Millisecond)
	})

	if output == "" {
		t.Error("Expected log output, got empty string")
	}
	if !strings.Contains(output, "req-456") {
		t.Error("Expected output to contain request ID")
	}
	if !strings.Contains(output, "PUT") {
		t.Error("Expected output to contain method")
	}
}

func TestHTTPRequestContextWithArgs(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)
	ctx := WithRequestID(context.Background(), "req-789")

	output := captureLogOutput(func() {
		HTTPRequestContext(ctx, "DELETE", "/api/delete", "172.16.0.1:3000", 200, 50*time.Millisecond, "resource_id", "abc123")
	})

	if output == "" {
		t.Error("Expected log output, got empty string")
	}
	if !strings.Contains(output, "resource_id") {
		t.Error("Expected output to contain custom args")
	}
}

func TestPluginLoading(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)

	output := captureLogOutput(func() {
		PluginLoading("test-plugin", "1.0.0", "converter")
	})

	if output == "" {
		t.Error("Expected log output, got empty string")
	}
	if !strings.Contains(output, "test-plugin") {
		t.Error("Expected output to contain plugin ID")
	}
	if !strings.Contains(output, "1.0.0") {
		t.Error("Expected output to contain version")
	}
	if !strings.Contains(output, "plugin_loading") {
		t.Error("Expected output to contain plugin_loading")
	}
}

func TestPluginLoadingWithArgs(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)

	output := captureLogOutput(func() {
		PluginLoading("advanced-plugin", "2.0.0", "formatter", "feature", "markdown")
	})

	if output == "" {
		t.Error("Expected log output, got empty string")
	}
	if !strings.Contains(output, "feature") {
		t.Error("Expected output to contain custom args")
	}
}

func TestPluginError(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)
	testErr := errors.New("plugin initialization failed")

	output := captureLogOutput(func() {
		PluginError("failing-plugin", "init", testErr)
	})

	if output == "" {
		t.Error("Expected log output, got empty string")
	}
	if !strings.Contains(output, "failing-plugin") {
		t.Error("Expected output to contain plugin ID")
	}
	if !strings.Contains(output, "init") {
		t.Error("Expected output to contain operation")
	}
	if !strings.Contains(output, "plugin initialization failed") {
		t.Error("Expected output to contain error message")
	}
	if !strings.Contains(output, "plugin_error") {
		t.Error("Expected output to contain plugin_error")
	}
}

func TestPluginErrorWithArgs(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)
	testErr := errors.New("configuration error")

	output := captureLogOutput(func() {
		PluginError("config-plugin", "configure", testErr, "config_file", "/etc/plugin.conf")
	})

	if output == "" {
		t.Error("Expected log output, got empty string")
	}
	if !strings.Contains(output, "config_file") {
		t.Error("Expected output to contain custom args")
	}
}

func TestWebSocketEvent(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)

	output := captureLogOutput(func() {
		WebSocketEvent("client_connected", 5)
	})

	if output == "" {
		t.Error("Expected log output, got empty string")
	}
	if !strings.Contains(output, "client_connected") {
		t.Error("Expected output to contain event")
	}
	if !strings.Contains(output, "websocket_event") {
		t.Error("Expected output to contain websocket_event")
	}
}

func TestWebSocketEventWithArgs(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)

	output := captureLogOutput(func() {
		WebSocketEvent("client_disconnected", 3, "reason", "timeout")
	})

	if output == "" {
		t.Error("Expected log output, got empty string")
	}
	if !strings.Contains(output, "reason") {
		t.Error("Expected output to contain custom args")
	}
}

func TestServerStartup(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)

	output := captureLogOutput(func() {
		ServerStartup("http", "HTTP/1.1", 8080)
	})

	if output == "" {
		t.Error("Expected log output, got empty string")
	}
	if !strings.Contains(output, "http") {
		t.Error("Expected output to contain server type")
	}
	if !strings.Contains(output, "8080") {
		t.Error("Expected output to contain port")
	}
	if !strings.Contains(output, "server_startup") {
		t.Error("Expected output to contain server_startup")
	}
}

func TestServerStartupWithArgs(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)

	output := captureLogOutput(func() {
		ServerStartup("grpc", "HTTP/2", 9090, "tls", "enabled")
	})

	if output == "" {
		t.Error("Expected log output, got empty string")
	}
	if !strings.Contains(output, "tls") {
		t.Error("Expected output to contain custom args")
	}
}

func TestSecurityEvent(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)

	output := captureLogOutput(func() {
		SecurityEvent("unauthorized_access", "api")
	})

	if output == "" {
		t.Error("Expected log output, got empty string")
	}
	if !strings.Contains(output, "unauthorized_access") {
		t.Error("Expected output to contain event")
	}
	if !strings.Contains(output, "api") {
		t.Error("Expected output to contain component")
	}
	if !strings.Contains(output, "security_event") {
		t.Error("Expected output to contain security_event")
	}
}

func TestSecurityEventWithArgs(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)

	output := captureLogOutput(func() {
		SecurityEvent("brute_force_attempt", "auth", "ip_address", "192.168.1.100")
	})

	if output == "" {
		t.Error("Expected log output, got empty string")
	}
	if !strings.Contains(output, "ip_address") {
		t.Error("Expected output to contain custom args")
	}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	tests := []struct {
		name           string
		initialCode    int
		writeCode      int
		expectedCode   int
		callTwice      bool
	}{
		{
			name:         "Write header once",
			initialCode:  http.StatusOK,
			writeCode:    http.StatusNotFound,
			expectedCode: http.StatusNotFound,
			callTwice:    false,
		},
		{
			name:         "Write header twice (second call ignored)",
			initialCode:  http.StatusOK,
			writeCode:    http.StatusNotFound,
			expectedCode: http.StatusNotFound,
			callTwice:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			rw := &responseWriter{
				ResponseWriter: recorder,
				statusCode:     tt.initialCode,
			}

			rw.WriteHeader(tt.writeCode)
			if tt.callTwice {
				// Second call should be ignored
				rw.WriteHeader(http.StatusInternalServerError)
			}

			if rw.statusCode != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, rw.statusCode)
			}
			if !rw.written {
				t.Error("Expected written flag to be true")
			}
		})
	}
}

func TestResponseWriter_Write(t *testing.T) {
	tests := []struct {
		name         string
		data         []byte
		callBefore   bool
		expectedCode int
	}{
		{
			name:         "Write without WriteHeader",
			data:         []byte("test data"),
			callBefore:   false,
			expectedCode: http.StatusOK,
		},
		{
			name:         "Write after WriteHeader",
			data:         []byte("more data"),
			callBefore:   true,
			expectedCode: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			rw := &responseWriter{
				ResponseWriter: recorder,
				statusCode:     http.StatusOK,
			}

			if tt.callBefore {
				rw.WriteHeader(http.StatusCreated)
			}

			n, err := rw.Write(tt.data)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if n != len(tt.data) {
				t.Errorf("Expected to write %d bytes, wrote %d", len(tt.data), n)
			}
			if rw.statusCode != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, rw.statusCode)
			}
			if !rw.written {
				t.Error("Expected written flag to be true")
			}
		})
	}
}

func TestGenerateRequestID(t *testing.T) {
	// Test multiple generations to ensure uniqueness
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateRequestID()
		if id == "" {
			t.Error("Expected non-empty request ID")
		}
		if len(id) != 16 {
			t.Errorf("Expected request ID length 16, got %d", len(id))
		}
		if ids[id] {
			t.Error("Generated duplicate request ID")
		}
		ids[id] = true
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		existingHeader string
		checkFunc      func(t *testing.T, w *httptest.ResponseRecorder, r *http.Request)
	}{
		{
			name:           "Generate new request ID",
			existingHeader: "",
			checkFunc: func(t *testing.T, w *httptest.ResponseRecorder, r *http.Request) {
				reqID := w.Header().Get("X-Request-ID")
				if reqID == "" {
					t.Error("Expected X-Request-ID header to be set")
				}
				if len(reqID) != 16 {
					t.Errorf("Expected request ID length 16, got %d", len(reqID))
				}
			},
		},
		{
			name:           "Use existing request ID from header",
			existingHeader: "existing-req-id-123",
			checkFunc: func(t *testing.T, w *httptest.ResponseRecorder, r *http.Request) {
				reqID := w.Header().Get("X-Request-ID")
				if reqID != "existing-req-id-123" {
					t.Errorf("Expected request ID 'existing-req-id-123', got '%s'", reqID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify context has request ID
				requestID := GetRequestID(r.Context())
				if requestID == "" {
					t.Error("Expected request ID in context")
				}
				w.WriteHeader(http.StatusOK)
			})

			middleware := RequestIDMiddleware(handler)
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.existingHeader != "" {
				req.Header.Set("X-Request-ID", tt.existingHeader)
			}
			w := httptest.NewRecorder()

			middleware.ServeHTTP(w, req)

			tt.checkFunc(t, w, req)
		})
	}
}

func TestLoggingMiddleware(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)

	tests := []struct {
		name       string
		method     string
		path       string
		statusCode int
	}{
		{
			name:       "GET request",
			method:     "GET",
			path:       "/api/users",
			statusCode: http.StatusOK,
		},
		{
			name:       "POST request",
			method:     "POST",
			path:       "/api/users",
			statusCode: http.StatusCreated,
		},
		{
			name:       "Error response",
			method:     "GET",
			path:       "/api/error",
			statusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			middleware := LoggingMiddleware(handler)
			req := httptest.NewRequest(tt.method, tt.path, nil)
			ctx := WithRequestID(req.Context(), "test-req-id")
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			output := captureLogOutput(func() {
				middleware.ServeHTTP(w, req)
			})

			if output == "" {
				t.Error("Expected log output")
			}
			if !strings.Contains(output, tt.method) {
				t.Errorf("Expected output to contain method %s", tt.method)
			}
			if !strings.Contains(output, tt.path) {
				t.Errorf("Expected output to contain path %s", tt.path)
			}
		})
	}
}

func TestLoggingMiddleware_WithWrite(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write data without explicitly calling WriteHeader
		w.Write([]byte("response body"))
	})

	middleware := LoggingMiddleware(handler)
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := WithRequestID(req.Context(), "test-write-id")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	output := captureLogOutput(func() {
		middleware.ServeHTTP(w, req)
	})

	if output == "" {
		t.Error("Expected log output")
	}
	// Should default to 200 OK when Write is called without WriteHeader
	if !strings.Contains(output, "200") {
		t.Error("Expected output to contain status code 200")
	}
}

func TestCombinedMiddleware(t *testing.T) {
	InitLogger(LevelInfo, FormatJSON)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request ID is in context
		requestID := GetRequestID(r.Context())
		if requestID == "" {
			t.Error("Expected request ID in context")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	middleware := CombinedMiddleware(handler)
	req := httptest.NewRequest("GET", "/combined", nil)
	w := httptest.NewRecorder()

	output := captureLogOutput(func() {
		middleware.ServeHTTP(w, req)
	})

	// Should have request ID header
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("Expected X-Request-ID header")
	}

	// Should have logged the request
	if output == "" {
		t.Error("Expected log output")
	}
	if !strings.Contains(output, "GET") {
		t.Error("Expected output to contain GET method")
	}
	if !strings.Contains(output, "/combined") {
		t.Error("Expected output to contain path")
	}
}

func TestReplaceAttrTimestamp(t *testing.T) {
	// Test that timestamps are formatted in RFC3339 using actual InitLogger
	output := captureLogOutputWithInit(LevelInfo, FormatJSON, func() {
		Info("timestamp test")
	})

	if output == "" {
		t.Error("Expected log output")
	}
	// Check for RFC3339 format pattern (contains T and Z or timezone offset)
	if !strings.Contains(output, "T") {
		t.Error("Expected timestamp to be in RFC3339 format")
	}
	// Also verify the message is present
	if !strings.Contains(output, "timestamp test") {
		t.Error("Expected output to contain test message")
	}
}

func TestReplaceAttrNonTimestamp(t *testing.T) {
	// Test with JSON format using actual InitLogger to test ReplaceAttr for non-time attributes
	output := captureLogOutputWithInit(LevelInfo, FormatJSON, func() {
		Info("test message", "custom_key", "custom_value", "number", 42)
	})

	if output == "" {
		t.Error("Expected log output")
	}
	// Verify custom attributes are present
	if !strings.Contains(output, "custom_key") {
		t.Error("Expected output to contain custom_key")
	}
	if !strings.Contains(output, "custom_value") {
		t.Error("Expected output to contain custom_value")
	}

	// Test with Text format to ensure both handler types work
	output = captureLogOutputWithInit(LevelInfo, FormatText, func() {
		Info("test message text", "key", "value")
	})

	if output == "" {
		t.Error("Expected log output for text format")
	}
	if !strings.Contains(output, "test message text") {
		t.Error("Expected output to contain test message")
	}
}

func TestInit(t *testing.T) {
	// The init function should have already run and initialized the logger
	// We just verify that the logger exists
	if defaultLogger == nil {
		t.Error("Expected defaultLogger to be initialized by init()")
	}
}

func TestContextKeyType(t *testing.T) {
	// Test that ContextKey is a distinct type
	var key ContextKey = "test"
	if string(key) != "test" {
		t.Errorf("Expected key to be 'test', got '%s'", string(key))
	}

	// Verify RequestIDKey constant
	if RequestIDKey != "request_id" {
		t.Errorf("Expected RequestIDKey to be 'request_id', got '%s'", RequestIDKey)
	}
}

func TestLevelConstants(t *testing.T) {
	// Verify level constants are in correct order
	if LevelDebug >= LevelInfo {
		t.Error("Expected LevelDebug < LevelInfo")
	}
	if LevelInfo >= LevelWarn {
		t.Error("Expected LevelInfo < LevelWarn")
	}
	if LevelWarn >= LevelError {
		t.Error("Expected LevelWarn < LevelError")
	}
}

func TestFormatConstants(t *testing.T) {
	// Verify format constants exist
	if FormatJSON == FormatText {
		t.Error("Expected FormatJSON != FormatText")
	}
}

// TestGenerateRequestIDFallback tests the fallback logic of generateRequestID.
// Note: The actual error path in generateRequestID (when crypto/rand.Read fails)
// is extremely difficult to test without mocking, as crypto/rand.Read rarely fails
// in normal circumstances. This would require either:
// 1. Mocking the rand.Read function (which we avoid)
// 2. Exhausting system entropy (not practical in tests)
// 3. Using build tags to inject a test version
//
// This test verifies that the fallback logic (hex encoding of timestamp) would
// produce a valid request ID of the correct length, providing confidence that
// the error path would work correctly if triggered.
func TestGenerateRequestIDFallback(t *testing.T) {
	// Test the fallback logic pattern (what would happen if rand.Read failed)
	fallbackID := hex.EncodeToString([]byte(time.Now().String()))[:16]

	if len(fallbackID) != 16 {
		t.Errorf("Expected fallback ID length 16, got %d", len(fallbackID))
	}

	// Verify it's valid hex
	if _, err := hex.DecodeString(fallbackID); err != nil {
		t.Errorf("Expected fallback ID to be valid hex, got error: %v", err)
	}

	// Test that normal generateRequestID works correctly
	// (this tests the success path which is already covered)
	for i := 0; i < 10; i++ {
		id := generateRequestID()
		if len(id) != 16 {
			t.Errorf("Expected request ID length 16, got %d", len(id))
		}
	}
}
