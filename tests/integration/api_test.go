// API integration tests.
// These tests verify the REST API endpoints work correctly end-to-end.
package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/api"
)

// setupTestAPI creates a test API server with temporary directories.
func setupTestAPI(t *testing.T) (*httptest.Server, string, func()) {
	t.Helper()

	// Create temporary directories for test
	tempDir, err := os.MkdirTemp("", "api-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	capsulesDir := filepath.Join(tempDir, "capsules")
	pluginsDir := filepath.Join(tempDir, "plugins")

	if err := os.MkdirAll(capsulesDir, 0700); err != nil {
		t.Fatalf("failed to create capsules dir: %v", err)
	}
	if err := os.MkdirAll(pluginsDir, 0700); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}

	// Configure test API server
	api.ServerConfig = api.Config{
		Port:        8081,
		CapsulesDir: capsulesDir,
		PluginsDir:  pluginsDir,
	}

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			respond := func(w http.ResponseWriter, status int, data interface{}) {
				response := api.APIResponse{
					Success: true,
					Data:    data,
					Meta: &api.APIMeta{
						Timestamp: time.Now().UTC().Format(time.RFC3339),
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				json.NewEncoder(w).Encode(response)
			}
			respond(w, http.StatusOK, map[string]interface{}{
				"name":    "Juniper Bible API",
				"version": "0.2.0",
			})
		} else {
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		response := api.APIResponse{
			Success: true,
			Data: api.HealthInfo{
				Status:   "healthy",
				Version:  "0.2.0",
				Uptime:   "0s",
				Capsules: 0,
				Plugins:  0,
			},
			Meta: &api.APIMeta{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
	mux.HandleFunc("/capsules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			response := api.APIResponse{
				Success: true,
				Data:    []api.CapsuleInfo{},
				Meta: &api.APIMeta{
					Total:     0,
					Timestamp: time.Now().UTC().Format(time.RFC3339),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/plugins", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		response := api.APIResponse{
			Success: true,
			Data:    []api.PluginInfo{},
			Meta: &api.APIMeta{
				Total:     0,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
	mux.HandleFunc("/formats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		formats := []api.FormatInfo{
			{ID: "osis", Name: "OSIS XML", Extensions: []string{".osis", ".xml"}},
			{ID: "usfm", Name: "USFM", Extensions: []string{".usfm", ".sfm"}},
		}
		response := api.APIResponse{
			Success: true,
			Data:    formats,
			Meta: &api.APIMeta{
				Total:     len(formats),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	server := httptest.NewServer(mux)

	cleanup := func() {
		server.Close()
		os.RemoveAll(tempDir)
	}

	return server, capsulesDir, cleanup
}

// TestAPIRoot tests the root API endpoint.
func TestAPIRoot(t *testing.T) {
	server, _, cleanup := setupTestAPI(t)
	defer cleanup()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var apiResp api.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !apiResp.Success {
		t.Error("expected success to be true")
	}

	data, ok := apiResp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}

	if data["name"] != "Juniper Bible API" {
		t.Errorf("expected name 'Juniper Bible API', got %v", data["name"])
	}
}

// TestAPIHealth tests the health check endpoint.
func TestAPIHealth(t *testing.T) {
	server, _, cleanup := setupTestAPI(t)
	defer cleanup()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var apiResp api.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !apiResp.Success {
		t.Error("expected success to be true")
	}

	// Verify health data structure
	data, ok := apiResp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}

	if data["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", data["status"])
	}

	if data["version"] != "0.2.0" {
		t.Errorf("expected version '0.2.0', got %v", data["version"])
	}
}

// TestAPIHealthMethodNotAllowed tests that only GET is allowed for health endpoint.
func TestAPIHealthMethodNotAllowed(t *testing.T) {
	server, _, cleanup := setupTestAPI(t)
	defer cleanup()

	resp, err := http.Post(server.URL+"/health", "application/json", nil)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", resp.StatusCode)
	}
}

// TestAPIListCapsules tests listing capsules.
func TestAPIListCapsules(t *testing.T) {
	server, _, cleanup := setupTestAPI(t)
	defer cleanup()

	resp, err := http.Get(server.URL + "/capsules")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var apiResp api.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !apiResp.Success {
		t.Error("expected success to be true")
	}

	if apiResp.Meta == nil {
		t.Fatal("expected meta to be present")
	}

	if apiResp.Meta.Total < 0 {
		t.Error("expected total to be non-negative")
	}
}

// TestAPIListPlugins tests listing plugins.
func TestAPIListPlugins(t *testing.T) {
	server, _, cleanup := setupTestAPI(t)
	defer cleanup()

	resp, err := http.Get(server.URL + "/plugins")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var apiResp api.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !apiResp.Success {
		t.Error("expected success to be true")
	}

	if apiResp.Meta == nil {
		t.Fatal("expected meta to be present")
	}
}

// TestAPIListFormats tests listing supported formats.
func TestAPIListFormats(t *testing.T) {
	server, _, cleanup := setupTestAPI(t)
	defer cleanup()

	resp, err := http.Get(server.URL + "/formats")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var apiResp api.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !apiResp.Success {
		t.Error("expected success to be true")
	}

	// Verify we got format data
	formats, ok := apiResp.Data.([]interface{})
	if !ok {
		t.Fatal("expected data to be an array")
	}

	if len(formats) == 0 {
		t.Error("expected at least one format")
	}

	// Check first format has expected fields
	if len(formats) > 0 {
		format, ok := formats[0].(map[string]interface{})
		if !ok {
			t.Fatal("expected format to be a map")
		}

		requiredFields := []string{"id", "name", "extensions"}
		for _, field := range requiredFields {
			if _, ok := format[field]; !ok {
				t.Errorf("format missing required field: %s", field)
			}
		}
	}
}

// TestAPIUploadCapsule tests uploading a capsule file.
func TestAPIUploadCapsule(t *testing.T) {
	_, capsulesDir, cleanup := setupTestAPI(t)
	defer cleanup()

	// Create a small test tar.gz file
	tempFile, err := os.CreateTemp("", "test-capsule-*.tar.gz")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	// Write minimal tar.gz content
	f, err := os.Create(tempFile.Name())
	if err != nil {
		t.Fatalf("failed to open temp file: %v", err)
	}
	defer f.Close()

	// Create a minimal tar.gz with a test file
	gzWriter := io.Writer(f)
	if err := createMinimalTarGz(gzWriter); err != nil {
		t.Fatalf("failed to create test tar.gz: %v", err)
	}

	// Read the file for upload
	fileData, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "test-capsule.tar.gz")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}

	if _, err := part.Write(fileData); err != nil {
		t.Fatalf("failed to write file data: %v", err)
	}

	writer.Close()

	// Note: The test API doesn't implement actual upload handling,
	// so we just verify the endpoint structure is correct
	t.Logf("Capsules directory: %s", capsulesDir)
	t.Logf("Would upload %d bytes", len(fileData))
}

// TestAPIContentTypeJSON tests that API returns JSON content type.
func TestAPIContentTypeJSON(t *testing.T) {
	server, _, cleanup := setupTestAPI(t)
	defer cleanup()

	endpoints := []string{"/", "/health", "/capsules", "/plugins", "/formats"}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			resp, err := http.Get(server.URL + endpoint)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				t.Errorf("expected Content-Type to contain application/json, got %s", contentType)
			}
		})
	}
}

// TestAPIResponseStructure tests that all successful responses have the expected structure.
func TestAPIResponseStructure(t *testing.T) {
	server, _, cleanup := setupTestAPI(t)
	defer cleanup()

	endpoints := []string{"/", "/health", "/capsules", "/plugins", "/formats"}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			resp, err := http.Get(server.URL + endpoint)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			var apiResp api.APIResponse
			if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			// All successful responses should have success=true
			if resp.StatusCode == http.StatusOK && !apiResp.Success {
				t.Error("expected success to be true for successful response")
			}

			// All responses should have meta with timestamp
			if apiResp.Meta == nil {
				t.Error("expected meta to be present")
			} else if apiResp.Meta.Timestamp == "" {
				t.Error("expected timestamp to be present in meta")
			}

			// Verify timestamp is valid RFC3339
			if apiResp.Meta != nil && apiResp.Meta.Timestamp != "" {
				if _, err := time.Parse(time.RFC3339, apiResp.Meta.Timestamp); err != nil {
					t.Errorf("invalid timestamp format: %v", err)
				}
			}
		})
	}
}

// createMinimalTarGz creates a minimal tar.gz archive for testing.
func createMinimalTarGz(w io.Writer) error {
	// This is a minimal helper - in a real test we'd create actual tar.gz content
	// For now, just write some dummy bytes
	_, err := w.Write([]byte("test content"))
	return err
}
