// client.go implements HTTP/FTP client for downloading SWORD modules.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
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

// Client provides HTTP download functionality for SWORD repositories.
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// NewClient creates a new repository client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		userAgent: "juniper-repoman/1.0",
	}
}

// Download fetches a URL and returns its content as bytes.
func (c *Client) Download(ctx context.Context, url string) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("empty URL")
	}

	// Validate URL scheme - only support HTTP/HTTPS
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("unsupported URL scheme: %s", url)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)

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

// DownloadToFile downloads a URL and saves it to a file.
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

// ListDirectory fetches an HTTP directory listing and extracts file names.
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

// HTTPError represents an HTTP error response.
type HTTPError struct {
	StatusCode int
	Status     string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP error: %s", e.Status)
}

// IsNotFound returns true if this is a 404 error.
func (e *HTTPError) IsNotFound() bool {
	return e.StatusCode == 404
}

// SourceConfig represents a SWORD repository source.
type SourceConfig struct {
	Name      string `json:"name"`
	URL       string `json:"url"`       // Base URL
	Directory string `json:"directory"` // Base directory path
}

// ModsIndexURL returns the full URL to the mods.d.tar.gz index file.
func (s *SourceConfig) ModsIndexURL() string {
	dir := strings.TrimSuffix(s.Directory, "/")
	return fmt.Sprintf("%s%s/mods.d.tar.gz", s.URL, dir)
}

// ModulePackageURLs returns possible URLs to try for a module's zip package.
func (s *SourceConfig) ModulePackageURLs(moduleID string) []string {
	dir := strings.TrimSuffix(s.Directory, "/")

	var urls []string

	if strings.HasSuffix(dir, "raw") {
		parent := strings.TrimSuffix(dir, "raw")
		// CrossWire-style: raw -> packages/rawzip
		urls = append(urls, fmt.Sprintf("%s%spackages/rawzip/%s.zip", s.URL, parent, moduleID))
		// CrossWire variant
		urls = append(urls, fmt.Sprintf("%s%spackages/%s.zip", s.URL, parent, moduleID))
		// IBT-style: raw -> rawzip
		urls = append(urls, fmt.Sprintf("%s%srawzip/%s.zip", s.URL, parent, moduleID))
	} else {
		// eBible-style: /zip subdirectory
		urls = append(urls, fmt.Sprintf("%s%s/zip/%s.zip", s.URL, dir, moduleID))
		// Also try packages/rawzip
		urls = append(urls, fmt.Sprintf("%s%s/packages/rawzip/%s.zip", s.URL, dir, moduleID))
	}

	return urls
}

// DefaultSources returns the default SWORD repository sources.
func DefaultSources() []SourceConfig {
	return []SourceConfig{
		{
			Name:      "CrossWire",
			URL:       "https://www.crosswire.org/ftpmirror",
			Directory: "/pub/sword/raw",
		},
		{
			Name:      "eBible",
			URL:       "https://ebible.org",
			Directory: "/sword",
		},
	}
}

// GetSource finds a source by name.
func GetSource(name string) (SourceConfig, bool) {
	for _, s := range DefaultSources() {
		if s.Name == name {
			return s, true
		}
	}
	return SourceConfig{}, false
}

// ParseModsArchive parses a mods.d.tar.gz archive and returns module info.
func ParseModsArchive(data []byte) ([]ModuleInfo, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decompress: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var modules []ModuleInfo

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break // May be corrupted entries
		}

		// Skip directories and non-.conf files
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		if !strings.HasSuffix(hdr.Name, ".conf") {
			continue
		}

		// Read conf file content
		content := make([]byte, hdr.Size)
		if _, err := io.ReadFull(tr, content); err != nil {
			continue
		}

		module, err := ParseModuleConf(content)
		if err != nil {
			continue // Skip invalid conf files
		}

		modules = append(modules, module)
	}

	return modules, nil
}

// ParseModuleConf parses a SWORD module .conf file.
func ParseModuleConf(data []byte) (ModuleInfo, error) {
	if len(data) == 0 {
		return ModuleInfo{}, fmt.Errorf("empty conf file")
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return ModuleInfo{}, fmt.Errorf("empty conf file")
	}

	// Find section header
	var moduleID string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			moduleID = strings.Trim(line, "[]")
			break
		}
	}

	if moduleID == "" {
		return ModuleInfo{}, fmt.Errorf("no section header found")
	}

	module := ModuleInfo{
		Name: moduleID,
	}

	// Parse key-value pairs
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[") || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Description":
			module.Description = value
		case "Lang":
			module.Language = value
		case "Version":
			module.Version = value
		case "ModDrv":
			module.Type = moduleTypeFromDriver(value)
		case "DataPath":
			module.DataPath = value
		}
	}

	return module, nil
}

// moduleTypeFromDriver determines the module type from the driver.
func moduleTypeFromDriver(driver string) string {
	driver = strings.ToLower(driver)
	switch {
	case strings.HasPrefix(driver, "ztext"), strings.HasPrefix(driver, "rawtext"):
		return "Bible"
	case strings.HasPrefix(driver, "zcom"), strings.HasPrefix(driver, "rawcom"):
		return "Commentary"
	case strings.HasPrefix(driver, "zld"), strings.HasPrefix(driver, "rawld"):
		return "Dictionary"
	case strings.Contains(driver, "genbook"):
		return "GenBook"
	default:
		return "Unknown"
	}
}

// ExtractZipArchive extracts a .zip archive to a directory.
func ExtractZipArchive(data []byte, destDir string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}

	for _, f := range r.File {
		destPath := filepath.Join(destDir, f.Name)

		// Check for directory traversal attack
		cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
		cleanPath := filepath.Clean(destPath)
		if !strings.HasPrefix(cleanPath, cleanDest) && cleanPath != filepath.Clean(destDir) {
			return fmt.Errorf("invalid file path in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0700); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
			continue
		}

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(destPath), 0700); err != nil {
			return fmt.Errorf("creating parent directory: %w", err)
		}

		// Extract file
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("opening file in zip: %w", err)
		}

		outFile, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return fmt.Errorf("creating file: %w", err)
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			outFile.Close()
			rc.Close()
			return fmt.Errorf("writing file: %w", err)
		}

		outFile.Close()
		rc.Close()
	}

	return nil
}
