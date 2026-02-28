// Package main provides tests for the security test tool.
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTestRunner_CaptchaWidgetDetection(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		shouldPass bool
	}{
		{
			name:       "Turnstile widget present",
			body:       `<div class="cf-turnstile"></div><input name="captcha_provider" value="turnstile">`,
			shouldPass: true,
		},
		{
			name:       "reCAPTCHA widget present",
			body:       `<div class="g-recaptcha"></div><input name="captcha_provider" value="recaptcha">`,
			shouldPass: true,
		},
		{
			name:       "hCaptcha widget present",
			body:       `<div class="h-captcha"></div><input name="captcha_provider" value="hcaptcha">`,
			shouldPass: true,
		},
		{
			name:       "Friendly Captcha present",
			body:       `<div class="frc-captcha"></div><input name="captcha_provider" value="friendlycaptcha">`,
			shouldPass: true,
		},
		{
			name:       "No CAPTCHA widget",
			body:       `<form><input type="submit"></form>`,
			shouldPass: false,
		},
		{
			name:       "Widget but no provider field",
			body:       `<div class="cf-turnstile"></div>`,
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			runner := &TestRunner{
				baseURL:     server.URL,
				contactPage: server.URL + "/contact/",
				client:      server.Client(),
				results:     make([]TestResult, 0),
				verbose:     false,
			}

			runner.testCaptchaWidgetRenders()

			if len(runner.results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(runner.results))
			}

			if runner.results[0].Passed != tt.shouldPass {
				t.Errorf("expected passed=%v, got passed=%v: %s",
					tt.shouldPass, runner.results[0].Passed, runner.results[0].Message)
			}
		})
	}
}

func TestTestRunner_SubmitButtonDisabled(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		shouldPass bool
	}{
		{
			name:       "Button with disabled attribute",
			body:       `<button id="submit-btn" disabled>Submit</button>`,
			shouldPass: true,
		},
		{
			name:       "Button with disabled in class context",
			body:       `<button id="submit-btn" class="btn disabled" disabled>Submit</button>`,
			shouldPass: true,
		},
		{
			name:       "Button without disabled",
			body:       `<button id="submit-btn">Submit</button>`,
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			runner := &TestRunner{
				baseURL:     server.URL,
				contactPage: server.URL + "/contact/",
				client:      server.Client(),
				results:     make([]TestResult, 0),
				verbose:     false,
			}

			runner.testSubmitButtonDisabled()

			if len(runner.results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(runner.results))
			}

			if runner.results[0].Passed != tt.shouldPass {
				t.Errorf("expected passed=%v, got passed=%v: %s",
					tt.shouldPass, runner.results[0].Passed, runner.results[0].Message)
			}
		})
	}
}

func TestTestRunner_APIRequiresCaptcha(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		shouldPass bool
	}{
		{
			name:       "400 with CAPTCHA required message",
			status:     400,
			body:       `{"error": "CAPTCHA verification required"}`,
			shouldPass: true,
		},
		{
			name:       "400 without expected message",
			status:     400,
			body:       `{"error": "Bad request"}`,
			shouldPass: false,
		},
		{
			name:       "200 OK (security hole)",
			status:     200,
			body:       `{"success": true}`,
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			runner := &TestRunner{
				baseURL:    server.URL,
				contactAPI: server.URL + "/api/contact",
				client:     server.Client(),
				results:    make([]TestResult, 0),
				verbose:    false,
			}

			runner.testAPIRequiresCaptchaToken()

			if len(runner.results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(runner.results))
			}

			if runner.results[0].Passed != tt.shouldPass {
				t.Errorf("expected passed=%v, got passed=%v: %s",
					tt.shouldPass, runner.results[0].Passed, runner.results[0].Message)
			}
		})
	}
}

func TestTestRunner_WorkerRejectsUnsigned(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		shouldPass bool
	}{
		{
			name:       "401 Unauthorized",
			status:     401,
			body:       `{"error": "Missing signature"}`,
			shouldPass: true,
		},
		{
			name:       "200 OK (security hole)",
			status:     200,
			body:       `{"success": true}`,
			shouldPass: false,
		},
		{
			name:       "500 misconfigured",
			status:     500,
			body:       `{"error": "Worker misconfigured"}`,
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			runner := &TestRunner{
				workerURL: server.URL,
				client:    server.Client(),
				results:   make([]TestResult, 0),
				verbose:   false,
			}

			runner.testWorkerRejectsUnsigned()

			if len(runner.results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(runner.results))
			}

			if runner.results[0].Passed != tt.shouldPass {
				t.Errorf("expected passed=%v, got passed=%v: %s",
					tt.shouldPass, runner.results[0].Passed, runner.results[0].Message)
			}
		})
	}
}

func TestTestRunner_WorkerRejectsExpired(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		shouldPass bool
	}{
		{
			name:       "401 with expired message",
			status:     401,
			body:       `{"error": "Request expired"}`,
			shouldPass: true,
		},
		{
			name:       "401 without expired message",
			status:     401,
			body:       `{"error": "Unauthorized"}`,
			shouldPass: false,
		},
		{
			name:       "200 OK (security hole)",
			status:     200,
			body:       `{"success": true}`,
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			runner := &TestRunner{
				workerURL: server.URL,
				client:    server.Client(),
				results:   make([]TestResult, 0),
				verbose:   false,
			}

			runner.testWorkerRejectsExpired()

			if len(runner.results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(runner.results))
			}

			if runner.results[0].Passed != tt.shouldPass {
				t.Errorf("expected passed=%v, got passed=%v: %s",
					tt.shouldPass, runner.results[0].Passed, runner.results[0].Message)
			}
		})
	}
}

// =============================================================================
// Additional Test Coverage
// =============================================================================

func TestTestRunner_APIRejectsFakeToken(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		shouldPass bool
	}{
		{
			name:       "400 with verification failed message",
			status:     400,
			body:       `{"error": "CAPTCHA verification failed"}`,
			shouldPass: true,
		},
		{
			name:       "400 without expected message",
			status:     400,
			body:       `{"error": "Some other error"}`,
			shouldPass: false,
		},
		{
			name:       "200 OK (security hole)",
			status:     200,
			body:       `{"success": true}`,
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			runner := &TestRunner{
				baseURL:    server.URL,
				contactAPI: server.URL + "/api/contact",
				client:     server.Client(),
				results:    make([]TestResult, 0),
				verbose:    false,
			}

			runner.testAPIRejectsFakeToken()

			if len(runner.results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(runner.results))
			}

			if runner.results[0].Passed != tt.shouldPass {
				t.Errorf("expected passed=%v, got passed=%v: %s",
					tt.shouldPass, runner.results[0].Passed, runner.results[0].Message)
			}
		})
	}
}

func TestTestRunner_APIRequiresProviderField(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		shouldPass bool
	}{
		{
			name:       "303 redirect (bypass possible)",
			status:     303,
			body:       ``,
			shouldPass: false,
		},
		{
			name:       "400 rejected",
			status:     400,
			body:       `{"error": "Missing captcha provider"}`,
			shouldPass: true,
		},
		{
			name:       "500 server error",
			status:     500,
			body:       `{"error": "Internal error"}`,
			shouldPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			runner := &TestRunner{
				baseURL:    server.URL,
				contactAPI: server.URL + "/api/contact",
				client:     server.Client(),
				results:    make([]TestResult, 0),
				verbose:    false,
			}

			runner.testAPIRequiresProviderField()

			if len(runner.results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(runner.results))
			}

			if runner.results[0].Passed != tt.shouldPass {
				t.Errorf("expected passed=%v, got passed=%v: %s",
					tt.shouldPass, runner.results[0].Passed, runner.results[0].Message)
			}
		})
	}
}

func TestTestRunner_WorkerRejectsBadSignature(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		shouldPass bool
	}{
		{
			name:       "401 with invalid signature message",
			status:     401,
			body:       `{"error": "Invalid signature"}`,
			shouldPass: true,
		},
		{
			name:       "401 without expected message",
			status:     401,
			body:       `{"error": "Unauthorized"}`,
			shouldPass: false,
		},
		{
			name:       "200 OK (security hole)",
			status:     200,
			body:       `{"success": true}`,
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			runner := &TestRunner{
				workerURL: server.URL,
				client:    server.Client(),
				results:   make([]TestResult, 0),
				verbose:   false,
			}

			runner.testWorkerRejectsBadSignature()

			if len(runner.results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(runner.results))
			}

			if runner.results[0].Passed != tt.shouldPass {
				t.Errorf("expected passed=%v, got passed=%v: %s",
					tt.shouldPass, runner.results[0].Passed, runner.results[0].Message)
			}
		})
	}
}

// =============================================================================
// TestResult Tests
// =============================================================================

func TestTestResult_Structure(t *testing.T) {
	result := TestResult{
		Name:    "Test Name",
		Passed:  true,
		Message: "Test passed",
		Details: "Additional details",
	}

	if result.Name == "" {
		t.Error("Name should not be empty")
	}
	if !result.Passed {
		t.Error("Passed should be true")
	}
	if result.Message == "" {
		t.Error("Message should not be empty")
	}
}

// =============================================================================
// TestRunner Tests
// =============================================================================

func TestTestRunner_AddResult(t *testing.T) {
	runner := &TestRunner{
		results: make([]TestResult, 0),
		verbose: false,
	}

	runner.addResult("Test 1", true, "Passed", "")
	runner.addResult("Test 2", false, "Failed", "Error details")

	if len(runner.results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(runner.results))
	}

	if runner.results[0].Name != "Test 1" {
		t.Errorf("first result name = %q, want 'Test 1'", runner.results[0].Name)
	}
	if !runner.results[0].Passed {
		t.Error("first result should pass")
	}

	if runner.results[1].Passed {
		t.Error("second result should fail")
	}
	if runner.results[1].Details != "Error details" {
		t.Errorf("second result details = %q, want 'Error details'", runner.results[1].Details)
	}
}

func TestTestRunner_Initialization(t *testing.T) {
	runner := &TestRunner{
		baseURL:     "https://example.com",
		workerURL:   "https://worker.example.com",
		contactAPI:  "https://example.com/api/contact",
		contactPage: "https://example.com/contact/",
		verbose:     true,
		client:      &http.Client{},
		results:     make([]TestResult, 0),
	}

	if runner.baseURL != "https://example.com" {
		t.Errorf("baseURL = %q, want 'https://example.com'", runner.baseURL)
	}
	if runner.workerURL != "https://worker.example.com" {
		t.Errorf("workerURL = %q, want 'https://worker.example.com'", runner.workerURL)
	}
	if !runner.verbose {
		t.Error("verbose should be true")
	}
	if runner.client == nil {
		t.Error("client should not be nil")
	}
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestTestRunner_NetworkError(t *testing.T) {
	// Create a runner pointing to a non-existent server
	runner := &TestRunner{
		baseURL:     "http://127.0.0.1:1",
		contactPage: "http://127.0.0.1:1/contact/",
		client: &http.Client{
			Timeout: 100 * time.Millisecond,
		},
		results: make([]TestResult, 0),
		verbose: false,
	}

	runner.testCaptchaWidgetRenders()

	if len(runner.results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(runner.results))
	}

	if runner.results[0].Passed {
		t.Error("test should fail on network error")
	}
}

func TestTestRunner_NetworkErrorSubmitButton(t *testing.T) {
	runner := &TestRunner{
		baseURL:     "http://127.0.0.1:1",
		contactPage: "http://127.0.0.1:1/contact/",
		client: &http.Client{
			Timeout: 100 * time.Millisecond,
		},
		results: make([]TestResult, 0),
		verbose: false,
	}

	runner.testSubmitButtonDisabled()

	if len(runner.results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(runner.results))
	}

	if runner.results[0].Passed {
		t.Error("test should fail on network error")
	}
}

func TestTestRunner_NetworkErrorAPI(t *testing.T) {
	runner := &TestRunner{
		baseURL:    "http://127.0.0.1:1",
		contactAPI: "http://127.0.0.1:1/api/contact",
		client: &http.Client{
			Timeout: 100 * time.Millisecond,
		},
		results: make([]TestResult, 0),
		verbose: false,
	}

	runner.testAPIRequiresCaptchaToken()

	if len(runner.results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(runner.results))
	}

	if runner.results[0].Passed {
		t.Error("test should fail on network error")
	}
}

func TestTestRunner_NetworkErrorWorker(t *testing.T) {
	runner := &TestRunner{
		workerURL: "http://127.0.0.1:1",
		client: &http.Client{
			Timeout: 100 * time.Millisecond,
		},
		results: make([]TestResult, 0),
		verbose: false,
	}

	runner.testWorkerRejectsUnsigned()

	if len(runner.results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(runner.results))
	}

	if runner.results[0].Passed {
		t.Error("test should fail on network error")
	}
}

// =============================================================================
// Color Constants Tests
// =============================================================================

func TestColorConstants(t *testing.T) {
	if colorRed == "" {
		t.Error("colorRed should not be empty")
	}
	if colorGreen == "" {
		t.Error("colorGreen should not be empty")
	}
	if colorYellow == "" {
		t.Error("colorYellow should not be empty")
	}
	if colorReset == "" {
		t.Error("colorReset should not be empty")
	}
}

// =============================================================================
// Worker 500 Error Tests
// =============================================================================

func TestTestRunner_WorkerRejectsUnsigned_500Error(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		shouldPass bool
	}{
		{
			name:       "500 with misconfigured message",
			status:     500,
			body:       `{"error": "Worker misconfigured"}`,
			shouldPass: false,
		},
		{
			name:       "500 without misconfigured message",
			status:     500,
			body:       `{"error": "Internal error"}`,
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			runner := &TestRunner{
				workerURL: server.URL,
				client:    server.Client(),
				results:   make([]TestResult, 0),
				verbose:   false,
			}

			runner.testWorkerRejectsUnsigned()

			if len(runner.results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(runner.results))
			}

			if runner.results[0].Passed != tt.shouldPass {
				t.Errorf("expected passed=%v, got passed=%v: %s",
					tt.shouldPass, runner.results[0].Passed, runner.results[0].Message)
			}
		})
	}
}
