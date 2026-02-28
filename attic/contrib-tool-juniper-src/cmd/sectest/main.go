// Package main provides a security test suite for the contact form.
//
// This tool tests CAPTCHA validation, HMAC authentication, and replay protection.
//
// Usage:
//
//	sectest [flags]
//	sectest -base-url https://example.com -worker-url https://worker.example.com
//
// Flags:
//
//	-base-url    Base URL of the website (default: https://focuswithjustin.com)
//	-worker-url  Email worker URL (default: from site config)
//	-local       Test against local development server
//	-verbose     Verbose output
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// TestResult holds the result of a single test
type TestResult struct {
	Name    string
	Passed  bool
	Message string
	Details string
}

// TestRunner manages test execution and reporting
type TestRunner struct {
	baseURL     string
	workerURL   string
	contactAPI  string
	contactPage string
	verbose     bool
	client      *http.Client
	results     []TestResult
}

// ANSI color codes
const (
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorReset  = "\033[0m"
)

func main() {
	baseURL := flag.String("base-url", "https://focuswithjustin.com", "Base URL of the website")
	workerURL := flag.String("worker-url", "https://focuswithjustin-email-sender.domains-ca9.workers.dev", "Email worker URL")
	local := flag.Bool("local", false, "Test against local development server")
	verbose := flag.Bool("verbose", false, "Verbose output")
	flag.Parse()

	if *local {
		*baseURL = "http://localhost:1313"
		// Local testing might not have worker
	}

	runner := &TestRunner{
		baseURL:     *baseURL,
		workerURL:   *workerURL,
		contactAPI:  *baseURL + "/api/contact",
		contactPage: *baseURL + "/contact/",
		verbose:     *verbose,
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects
			},
		},
		results: make([]TestResult, 0),
	}

	runner.printHeader()
	runner.runAllTests()
	runner.printSummary()

	// Exit with non-zero if any tests failed
	for _, r := range runner.results {
		if !r.Passed {
			os.Exit(1)
		}
	}
}

func (r *TestRunner) printHeader() {
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           CONTACT FORM SECURITY TEST SUITE                        ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Base URL:   %-52s║\n", r.baseURL)
	fmt.Printf("║  Worker URL: %-52s║\n", r.workerURL)
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
}

func (r *TestRunner) runAllTests() {
	r.testCaptchaWidgetRenders()
	r.testSubmitButtonDisabled()
	r.testAPIRequiresCaptchaToken()
	r.testAPIRejectsFakeToken()
	r.testAPIRequiresProviderField()
	r.testWorkerRejectsUnsigned()
	r.testWorkerRejectsBadSignature()
	r.testWorkerRejectsExpired()
}

func (r *TestRunner) addResult(name string, passed bool, message, details string) {
	result := TestResult{
		Name:    name,
		Passed:  passed,
		Message: message,
		Details: details,
	}
	r.results = append(r.results, result)

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("%s[INFO]%s Test %d: %s\n", colorYellow, colorReset, len(r.results), name)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if passed {
		fmt.Printf("%s[PASS]%s %s\n", colorGreen, colorReset, message)
	} else {
		fmt.Printf("%s[FAIL]%s %s\n", colorRed, colorReset, message)
	}

	if r.verbose && details != "" {
		fmt.Printf("       Details: %s\n", details)
	}
}

func (r *TestRunner) printSummary() {
	passed := 0
	failed := 0
	var failedTests []string

	for _, result := range r.results {
		if result.Passed {
			passed++
		} else {
			failed++
			failedTests = append(failedTests, result.Name)
		}
	}

	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                         TEST SUMMARY                              ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Tests Run:    %-52d║\n", len(r.results))
	fmt.Printf("║  %sPassed:%s        %-52d║\n", colorGreen, colorReset, passed)
	fmt.Printf("║  %sFailed:%s        %-52d║\n", colorRed, colorReset, failed)
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")

	if failed > 0 {
		fmt.Println()
		fmt.Printf("%sFAILED TESTS:%s\n", colorRed, colorReset)
		for _, name := range failedTests {
			fmt.Printf("  %s✗%s %s\n", colorRed, colorReset, name)
		}
		fmt.Println()
	} else {
		fmt.Println()
		fmt.Printf("%sAll security tests passed!%s\n", colorGreen, colorReset)
		fmt.Println()
	}
}

// Test 1: CAPTCHA widget renders on contact page
func (r *TestRunner) testCaptchaWidgetRenders() {
	testName := "CAPTCHA widget renders on contact page"

	resp, err := r.client.Get(r.contactPage)
	if err != nil {
		r.addResult(testName, false, "Failed to fetch contact page", err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		r.addResult(testName, false, "Failed to read response body", err.Error())
		return
	}

	bodyStr := string(body)

	// Check for CAPTCHA widget
	captchaPattern := regexp.MustCompile(`cf-turnstile|g-recaptcha|h-captcha|frc-captcha`)
	if !captchaPattern.MatchString(bodyStr) {
		r.addResult(testName, false, "CAPTCHA widget not found - check HUGO_*_SITE_KEY env var", "")
		return
	}

	// Check for captcha_provider field
	if !strings.Contains(bodyStr, "captcha_provider") {
		r.addResult(testName, false, "captcha_provider hidden field missing", "")
		return
	}

	r.addResult(testName, true, "CAPTCHA widget and provider field found", "")
}

// Test 2: Submit button is disabled by default
func (r *TestRunner) testSubmitButtonDisabled() {
	testName := "Submit button is disabled until CAPTCHA completed"

	resp, err := r.client.Get(r.contactPage)
	if err != nil {
		r.addResult(testName, false, "Failed to fetch contact page", err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		r.addResult(testName, false, "Failed to read response body", err.Error())
		return
	}

	bodyStr := string(body)

	// Check for disabled submit button
	disabledPattern := regexp.MustCompile(`id=["']?submit-btn["']?[^>]*disabled|submit-btn.*disabled`)
	if !disabledPattern.MatchString(bodyStr) {
		r.addResult(testName, false, "Submit button is NOT disabled - users can submit without CAPTCHA", "")
		return
	}

	r.addResult(testName, true, "Submit button is disabled by default", "")
}

// Test 3: API rejects requests without CAPTCHA token
func (r *TestRunner) testAPIRequiresCaptchaToken() {
	testName := "API rejects requests without CAPTCHA token"

	data := url.Values{}
	data.Set("name", "SecurityTest")
	data.Set("email", "test@example.com")
	data.Set("subject", "Test")
	data.Set("message", "No token")
	data.Set("captcha_provider", "turnstile")

	resp, err := r.client.PostForm(r.contactAPI, data)
	if err != nil {
		r.addResult(testName, false, "Request failed", err.Error())
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 400 && strings.Contains(string(body), "CAPTCHA verification required") {
		r.addResult(testName, true, fmt.Sprintf("API correctly rejects missing CAPTCHA token (HTTP %d)", resp.StatusCode), "")
	} else {
		r.addResult(testName, false, fmt.Sprintf("API should reject missing token with 400, got HTTP %d", resp.StatusCode), string(body))
	}
}

// Test 4: API rejects requests with fake CAPTCHA token
func (r *TestRunner) testAPIRejectsFakeToken() {
	testName := "API rejects fake/invalid CAPTCHA token"

	data := url.Values{}
	data.Set("name", "SecurityTest")
	data.Set("email", "test@example.com")
	data.Set("subject", "Test")
	data.Set("message", "Fake token")
	data.Set("captcha_provider", "turnstile")
	data.Set("cf-turnstile-response", "fake-invalid-token-12345")

	resp, err := r.client.PostForm(r.contactAPI, data)
	if err != nil {
		r.addResult(testName, false, "Request failed", err.Error())
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 400 && strings.Contains(string(body), "CAPTCHA verification failed") {
		r.addResult(testName, true, fmt.Sprintf("API correctly rejects fake CAPTCHA token (HTTP %d)", resp.StatusCode), "")
	} else {
		r.addResult(testName, false, fmt.Sprintf("API should reject fake token with 400, got HTTP %d", resp.StatusCode), string(body))
	}
}

// Test 5: API rejects requests without captcha_provider field
func (r *TestRunner) testAPIRequiresProviderField() {
	testName := "API rejects requests without captcha_provider field"

	data := url.Values{}
	data.Set("name", "SecurityTest")
	data.Set("email", "test@example.com")
	data.Set("subject", "Test")
	data.Set("message", "No provider field")

	resp, err := r.client.PostForm(r.contactAPI, data)
	if err != nil {
		r.addResult(testName, false, "Request failed", err.Error())
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Should NOT succeed (303 is success redirect)
	if resp.StatusCode == 303 {
		r.addResult(testName, false, "API accepted request without captcha_provider - CAPTCHA bypass possible!", string(body))
	} else {
		r.addResult(testName, true, fmt.Sprintf("API rejected request without captcha_provider (HTTP %d)", resp.StatusCode), "")
	}
}

// Test 6: Email worker rejects unsigned requests
func (r *TestRunner) testWorkerRejectsUnsigned() {
	testName := "Email worker rejects unsigned requests (HMAC)"

	reqBody := map[string]string{
		"name":    "Attacker",
		"email":   "attacker@example.com",
		"subject": "Direct Attack",
		"body":    "Unsigned request",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", r.workerURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		r.addResult(testName, false, "Failed to create request", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		r.addResult(testName, false, "Request failed", err.Error())
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case 401:
		r.addResult(testName, true, "Worker correctly rejects unsigned request (HTTP 401)", "")
	case 500:
		if strings.Contains(string(body), "misconfigured") {
			r.addResult(testName, false, "Worker secret not configured - run: npx wrangler secret put TURNSTILE_SECRET_KEY", string(body))
		} else {
			r.addResult(testName, false, fmt.Sprintf("Unexpected server error: HTTP %d", resp.StatusCode), string(body))
		}
	case 200:
		r.addResult(testName, false, "Worker accepted unsigned request - HMAC auth not working!", string(body))
	default:
		r.addResult(testName, false, fmt.Sprintf("Unexpected response from worker: HTTP %d", resp.StatusCode), string(body))
	}
}

// Test 7: Email worker rejects requests with invalid signature
func (r *TestRunner) testWorkerRejectsBadSignature() {
	testName := "Email worker rejects requests with invalid signature"

	reqBody := map[string]string{
		"name":    "Attacker",
		"email":   "attacker@example.com",
		"subject": "Bad Sig",
		"body":    "Invalid signature",
	}
	jsonBody, _ := json.Marshal(reqBody)

	timestamp := fmt.Sprintf("%d000", time.Now().Unix())

	req, err := http.NewRequest("POST", r.workerURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		r.addResult(testName, false, "Failed to create request", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", "aW52YWxpZC1zaWduYXR1cmU=") // "invalid-signature" in base64

	resp, err := r.client.Do(req)
	if err != nil {
		r.addResult(testName, false, "Request failed", err.Error())
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 && strings.Contains(string(body), "Invalid signature") {
		r.addResult(testName, true, "Worker correctly rejects invalid signature (HTTP 401)", "")
	} else {
		r.addResult(testName, false, fmt.Sprintf("Worker should reject invalid signature, got HTTP %d", resp.StatusCode), string(body))
	}
}

// Test 8: Email worker rejects expired requests (replay protection)
func (r *TestRunner) testWorkerRejectsExpired() {
	testName := "Email worker rejects expired requests (replay protection)"

	reqBody := map[string]string{
		"name":    "Replayer",
		"email":   "replay@example.com",
		"subject": "Replay",
		"body":    "Old request",
	}
	jsonBody, _ := json.Marshal(reqBody)

	// Timestamp from 10 minutes ago
	oldTimestamp := fmt.Sprintf("%d000", time.Now().Unix()-600)

	req, err := http.NewRequest("POST", r.workerURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		r.addResult(testName, false, "Failed to create request", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Timestamp", oldTimestamp)
	req.Header.Set("X-Signature", "c29tZS1zaWduYXR1cmU=") // "some-signature" in base64

	resp, err := r.client.Do(req)
	if err != nil {
		r.addResult(testName, false, "Request failed", err.Error())
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 && strings.Contains(string(body), "expired") {
		r.addResult(testName, true, "Worker correctly rejects expired request (HTTP 401)", "")
	} else {
		r.addResult(testName, false, fmt.Sprintf("Worker should reject expired request, got HTTP %d", resp.StatusCode), string(body))
	}
}
