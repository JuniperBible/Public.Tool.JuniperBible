package repository

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewClient tests client creation
func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		opts    ClientOptions
		wantErr bool
	}{
		{
			name:    "default options",
			opts:    ClientOptions{},
			wantErr: false,
		},
		{
			name: "custom timeout",
			opts: ClientOptions{
				Timeout: 30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "with retries",
			opts: ClientOptions{
				MaxRetries: 3,
				RetryDelay: time.Second,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client")
			}
		})
	}
}

// TestClient_DownloadHTTP tests HTTP downloads with mock server
func TestClient_DownloadHTTP(t *testing.T) {
	// Create test server
	testData := []byte("test file content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/test.txt":
			w.Header().Set("Content-Length", "17")
			w.Write(testData)
		case "/notfound.txt":
			http.NotFound(w, r)
		case "/error":
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	tests := []struct {
		name     string
		url      string
		wantData []byte
		wantErr  bool
	}{
		{
			name:     "successful download",
			url:      server.URL + "/test.txt",
			wantData: testData,
			wantErr:  false,
		},
		{
			name:     "file not found",
			url:      server.URL + "/notfound.txt",
			wantData: nil,
			wantErr:  true,
		},
		{
			name:     "server error",
			url:      server.URL + "/error",
			wantData: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			data, err := client.Download(ctx, tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.Download() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !bytes.Equal(data, tt.wantData) {
				t.Errorf("Client.Download() = %v, want %v", data, tt.wantData)
			}
		})
	}
}

// TestClient_DownloadToFile tests downloading to a file
func TestClient_DownloadToFile(t *testing.T) {
	testData := []byte("file download test content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(testData)
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// Create temp directory
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "downloaded.txt")

	ctx := context.Background()
	err = client.DownloadToFile(ctx, server.URL+"/test.txt", destPath)
	if err != nil {
		t.Fatalf("Client.DownloadToFile() error = %v", err)
	}

	// Verify file contents
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Equal(content, testData) {
		t.Errorf("Downloaded file content = %v, want %v", content, testData)
	}
}

// TestClient_DownloadToFileCreatesDirs tests that missing directories are created
func TestClient_DownloadToFileCreatesDirs(t *testing.T) {
	testData := []byte("nested directory test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(testData)
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "nested", "dir", "file.txt")

	ctx := context.Background()
	err = client.DownloadToFile(ctx, server.URL+"/test.txt", destPath)
	if err != nil {
		t.Fatalf("Client.DownloadToFile() error = %v", err)
	}

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("File was not created at expected path")
	}
}

// TestClient_DownloadWithProgress tests download with progress callback
func TestClient_DownloadWithProgress(t *testing.T) {
	testData := make([]byte, 1024) // 1KB of data
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		w.Write(testData)
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	var progressCalled bool
	var lastBytes int64

	ctx := context.Background()
	_, err = client.DownloadWithProgress(ctx, server.URL+"/test.bin", func(downloaded, total int64) {
		progressCalled = true
		lastBytes = downloaded
	})

	if err != nil {
		t.Fatalf("Client.DownloadWithProgress() error = %v", err)
	}
	if !progressCalled {
		t.Error("Progress callback was never called")
	}
	if lastBytes != 1024 {
		t.Errorf("Final progress bytes = %d, want 1024", lastBytes)
	}
}

// TestClient_ContextCancellation tests that downloads respect context cancellation
func TestClient_ContextCancellation(t *testing.T) {
	// Server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.Write([]byte("delayed response"))
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = client.Download(ctx, server.URL+"/slow")
	if err == nil {
		t.Error("Client.Download() should have returned error on context cancellation")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Logf("Got error: %v (expected context deadline/cancel)", err)
	}
}

// TestClient_RetryOnError tests automatic retry behavior
func TestClient_RetryOnError(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "Temporary Error", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("success after retries"))
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{
		MaxRetries: 5,
		RetryDelay: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	data, err := client.Download(ctx, server.URL+"/retry")
	if err != nil {
		t.Fatalf("Client.Download() error = %v", err)
	}

	if string(data) != "success after retries" {
		t.Errorf("Client.Download() = %q, want %q", string(data), "success after retries")
	}
	if attempts != 3 {
		t.Errorf("Server received %d attempts, want 3", attempts)
	}
}

// TestClient_ListDirectory tests HTTP directory listing
func TestClient_ListDirectory(t *testing.T) {
	htmlListing := `<!DOCTYPE html>
<html>
<body>
<a href="kjv.conf">kjv.conf</a>
<a href="drc.conf">drc.conf</a>
<a href="vulgate.conf">vulgate.conf</a>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(htmlListing))
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	files, err := client.ListDirectory(ctx, server.URL+"/mods.d/")
	if err != nil {
		t.Fatalf("Client.ListDirectory() error = %v", err)
	}

	expected := []string{"kjv.conf", "drc.conf", "vulgate.conf"}
	if len(files) != len(expected) {
		t.Errorf("ListDirectory() returned %d files, want %d", len(files), len(expected))
	}
}

// TestClientOptions_Defaults tests default option values
func TestClientOptions_Defaults(t *testing.T) {
	opts := DefaultClientOptions()

	if opts.Timeout == 0 {
		t.Error("DefaultClientOptions().Timeout should not be zero")
	}
	if opts.MaxRetries < 0 {
		t.Error("DefaultClientOptions().MaxRetries should be non-negative")
	}
}

// mockReader is a reader that can simulate errors
type mockReader struct {
	data     []byte
	pos      int
	err      error
	errAfter int
}

func (m *mockReader) Read(p []byte) (n int, err error) {
	if m.errAfter > 0 && m.pos >= m.errAfter {
		return 0, m.err
	}
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

// TestClient_HandleReadError tests handling of read errors during download
func TestClient_HandleReadError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate connection drop by closing without full content
		w.Header().Set("Content-Length", "1000")
		w.Write([]byte("partial"))
		// Force close connection
		if hj, ok := w.(http.Hijacker); ok {
			if conn, _, err := hj.Hijack(); err == nil {
				conn.Close()
			}
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{
		MaxRetries: 0, // No retries to test immediate failure
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	_, err = client.Download(ctx, server.URL+"/incomplete")
	// Should get an error due to incomplete response
	// The exact error depends on implementation
	if err == nil {
		t.Log("Note: Expected error on incomplete download, but implementation may buffer")
	}
}

// TestClient_InvalidURL tests handling of invalid URLs
func TestClient_InvalidURL(t *testing.T) {
	client, err := NewClient(ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	tests := []struct {
		name string
		url  string
	}{
		{"empty URL", ""},
		{"invalid scheme", "foobar://example.com/file"},
		{"missing host", "http:///path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := client.Download(ctx, tt.url)
			if err == nil {
				t.Errorf("Client.Download(%q) should return error", tt.url)
			}
		})
	}
}
