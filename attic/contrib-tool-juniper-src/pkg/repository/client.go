package repository

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ClientOptions configures the repository client behavior
type ClientOptions struct {
	Timeout    time.Duration // HTTP request timeout
	MaxRetries int           // Maximum number of retry attempts
	RetryDelay time.Duration // Delay between retries
	UserAgent  string        // Custom User-Agent header
}

// DefaultClientOptions returns sensible default options
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		Timeout:    60 * time.Second,
		MaxRetries: 3,
		RetryDelay: time.Second,
		UserAgent:  "juniper/1.0",
	}
}

// ProgressFunc is called during downloads to report progress
type ProgressFunc func(downloaded, total int64)

// Client provides HTTP/FTP download functionality for SWORD repositories
type Client struct {
	httpClient *http.Client
	ftpClient  *FTPClient
	opts       ClientOptions
}

// NewClient creates a new repository client
func NewClient(opts ClientOptions) (*Client, error) {
	// Apply defaults for zero values
	if opts.Timeout == 0 {
		opts.Timeout = DefaultClientOptions().Timeout
	}
	if opts.UserAgent == "" {
		opts.UserAgent = DefaultClientOptions().UserAgent
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: opts.Timeout,
		},
		ftpClient: NewFTPClient(opts),
		opts:      opts,
	}, nil
}

// Download fetches a URL and returns its content as bytes
func (c *Client) Download(ctx context.Context, url string) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("empty URL")
	}

	// Validate URL scheme
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "ftp://") {
		return nil, fmt.Errorf("unsupported URL scheme: %s", url)
	}

	// Handle FTP URLs
	if strings.HasPrefix(url, "ftp://") {
		return c.downloadFTP(ctx, url)
	}

	var lastErr error
	maxAttempts := c.opts.MaxRetries + 1

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.opts.RetryDelay):
			}
		}

		data, err := c.downloadOnce(ctx, url)
		if err == nil {
			return data, nil
		}

		lastErr = err

		// Don't retry on 4xx errors (client errors)
		if isClientError(err) {
			return nil, err
		}
	}

	return nil, lastErr
}

// downloadFTP handles FTP URL downloads
func (c *Client) downloadFTP(ctx context.Context, url string) ([]byte, error) {
	// Parse the URL: ftp://host/path
	url = strings.TrimPrefix(url, "ftp://")
	parts := strings.SplitN(url, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid FTP URL format")
	}

	host := parts[0]
	path := "/" + parts[1]

	if err := c.ftpClient.Connect(ctx, host); err != nil {
		return nil, fmt.Errorf("connecting to FTP: %w", err)
	}
	defer c.ftpClient.Close()

	return c.ftpClient.Download(ctx, path)
}

// downloadFTPWithProgress handles FTP URL downloads with progress reporting
func (c *Client) downloadFTPWithProgress(ctx context.Context, url string, progress ProgressFunc) ([]byte, error) {
	// Parse the URL: ftp://host/path
	url = strings.TrimPrefix(url, "ftp://")
	parts := strings.SplitN(url, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid FTP URL format")
	}

	host := parts[0]
	path := "/" + parts[1]

	if err := c.ftpClient.Connect(ctx, host); err != nil {
		return nil, fmt.Errorf("connecting to FTP: %w", err)
	}
	defer c.ftpClient.Close()

	return c.ftpClient.DownloadWithProgress(ctx, path, progress)
}

func (c *Client) downloadOnce(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", c.opts.UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Status: resp.Status}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return data, nil
}

// DownloadToFile downloads a URL and saves it to a file
func (c *Client) DownloadToFile(ctx context.Context, url, destPath string) error {
	data, err := c.Download(ctx, url)
	if err != nil {
		return err
	}

	// Create parent directories if needed
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	if err := os.WriteFile(destPath, data, 0600); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

// DownloadWithProgress downloads a URL with progress reporting
func (c *Client) DownloadWithProgress(ctx context.Context, url string, progress ProgressFunc) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("empty URL")
	}

	// Handle FTP URLs
	if strings.HasPrefix(url, "ftp://") {
		return c.downloadFTPWithProgress(ctx, url, progress)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", c.opts.UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Status: resp.Status}
	}

	total := resp.ContentLength

	// Read with progress tracking
	var downloaded int64
	var data []byte
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
			downloaded += int64(n)
			if progress != nil {
				progress(downloaded, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}
	}

	return data, nil
}

// ListDirectory fetches an HTTP directory listing and extracts file names
func (c *Client) ListDirectory(ctx context.Context, url string) ([]string, error) {
	data, err := c.Download(ctx, url)
	if err != nil {
		return nil, err
	}

	// Parse HTML directory listing
	// Look for href attributes in anchor tags
	re := regexp.MustCompile(`href="([^"]+)"`)
	matches := re.FindAllSubmatch(data, -1)

	var files []string
	for _, match := range matches {
		if len(match) > 1 {
			filename := string(match[1])
			// Skip parent directory and absolute links
			if filename != "../" && !strings.HasPrefix(filename, "/") && !strings.HasPrefix(filename, "http") {
				files = append(files, filename)
			}
		}
	}

	return files, nil
}

// HTTPError represents an HTTP error response
type HTTPError struct {
	StatusCode int
	Status     string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP error: %s", e.Status)
}

// IsNotFound returns true if this is a 404 error
func (e *HTTPError) IsNotFound() bool {
	return e.StatusCode == 404
}

// FTPError represents an FTP error response
type FTPError struct {
	Code    int
	Message string
}

func (e *FTPError) Error() string {
	return fmt.Sprintf("FTP error %d: %s", e.Code, e.Message)
}

// IsNotFound returns true if this is a file not found error (550)
func (e *FTPError) IsNotFound() bool {
	return e.Code == 550
}

// IsNotFoundError checks if an error indicates a file/package was not found
func IsNotFoundError(err error) bool {
	if httpErr, ok := err.(*HTTPError); ok {
		return httpErr.IsNotFound()
	}
	if ftpErr, ok := err.(*FTPError); ok {
		return ftpErr.IsNotFound()
	}
	// Check for wrapped errors containing "550" or "404"
	errStr := err.Error()
	return strings.Contains(errStr, "550") || strings.Contains(errStr, "404") || strings.Contains(errStr, "not found")
}

func isClientError(err error) bool {
	if httpErr, ok := err.(*HTTPError); ok {
		return httpErr.StatusCode >= 400 && httpErr.StatusCode < 500
	}
	return false
}
