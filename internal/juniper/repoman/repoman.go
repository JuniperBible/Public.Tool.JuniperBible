// Package repoman provides SWORD repository management functionality.
// This package is used by capsule-juniper, capsule-repoman, and the capsule CLI.
package repoman

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
)

// Source represents a SWORD repository source.
type Source struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Directory string `json:"directory"`
}

// ModsIndexURL returns the full URL to the mods.d.tar.gz index file.
func (s *Source) ModsIndexURL() string {
	dir := strings.TrimSuffix(s.Directory, "/")
	return fmt.Sprintf("%s%s/mods.d.tar.gz", s.URL, dir)
}

// ModulePackageURLs returns possible URLs to try for a module's zip package.
func (s *Source) ModulePackageURLs(moduleID string) []string {
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

// ModuleInfo contains metadata about a SWORD module.
type ModuleInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Language    string `json:"language"`
	Version     string `json:"version"`
	Size        int64  `json:"size"`
	DataPath    string `json:"data_path,omitempty"`
}

// VerifyResult contains the result of module verification.
type VerifyResult struct {
	Module   string   `json:"module"`
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// Client provides HTTP download functionality for SWORD repositories.
type Client struct {
	httpClient *http.Client
	userAgent  string
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

// NewClient creates a new repository client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		userAgent: "capsule-repoman/1.0",
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

// DefaultSources returns the default SWORD repository sources.
func DefaultSources() []Source {
	return []Source{
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
func GetSource(name string) (Source, bool) {
	for _, s := range DefaultSources() {
		if s.Name == name {
			return s, true
		}
	}
	return Source{}, false
}

// ListSources returns the list of configured sources.
func ListSources() []Source {
	return DefaultSources()
}

// RefreshSource refreshes the module index for a source.
func RefreshSource(sourceName string) error {
	source, ok := GetSource(sourceName)
	if !ok {
		return fmt.Errorf("unknown source: %s", sourceName)
	}

	client := NewClient()
	ctx := context.Background()

	indexURL := source.ModsIndexURL()
	_, err := client.Download(ctx, indexURL)
	if err != nil {
		return fmt.Errorf("downloading index: %w", err)
	}

	return nil
}

// ListAvailable lists available modules from a source.
func ListAvailable(sourceName string) ([]ModuleInfo, error) {
	source, ok := GetSource(sourceName)
	if !ok {
		return nil, fmt.Errorf("unknown source: %s", sourceName)
	}

	client := NewClient()
	ctx := context.Background()

	indexURL := source.ModsIndexURL()
	data, err := client.Download(ctx, indexURL)
	if err != nil {
		return nil, fmt.Errorf("downloading index: %w", err)
	}

	return ParseModsArchive(data)
}

// Install installs a module from a source.
func Install(sourceName, moduleID, destPath string) error {
	source, ok := GetSource(sourceName)
	if !ok {
		return fmt.Errorf("unknown source: %s", sourceName)
	}

	client := NewClient()
	ctx := context.Background()

	// Try multiple package URLs
	packageURLs := source.ModulePackageURLs(moduleID)
	var data []byte
	var lastErr error

	for _, url := range packageURLs {
		data, lastErr = client.Download(ctx, url)
		if lastErr == nil {
			break
		}
	}

	if data == nil {
		return fmt.Errorf("downloading module: %w", lastErr)
	}

	// Extract to destination
	if destPath == "" {
		destPath = "."
	}

	if err := ExtractZipArchive(data, destPath); err != nil {
		return fmt.Errorf("extracting module: %w", err)
	}

	return nil
}

// ListInstalled lists installed modules.
func ListInstalled(swordPath string) ([]ModuleInfo, error) {
	if swordPath == "" {
		swordPath = "."
	}

	modsDir := filepath.Join(swordPath, "mods.d")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []ModuleInfo{}, nil
		}
		return nil, fmt.Errorf("reading mods.d: %w", err)
	}

	var modules []ModuleInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") {
			continue
		}

		confPath := filepath.Join(modsDir, entry.Name())
		data, err := safefile.ReadFile(confPath)
		if err != nil {
			continue
		}

		module, err := ParseModuleConf(data)
		if err != nil {
			continue
		}

		modules = append(modules, module)
	}

	return modules, nil
}

// Uninstall removes an installed module.
func Uninstall(moduleID, swordPath string) error {
	if swordPath == "" {
		swordPath = "."
	}

	// Find and remove conf file
	confPath := filepath.Join(swordPath, "mods.d", strings.ToLower(moduleID)+".conf")
	if _, err := os.Stat(confPath); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("module %s not installed", moduleID)
	}

	// Read conf to find data path
	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("reading conf: %w", err)
	}

	module, err := ParseModuleConf(data)
	if err != nil {
		return fmt.Errorf("parsing conf: %w", err)
	}

	// Remove data directory
	if module.DataPath != "" {
		dataPath := filepath.Join(swordPath, module.DataPath)
		dataPath = strings.TrimPrefix(dataPath, "./")
		if err := os.RemoveAll(dataPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing data: %w", err)
		}
	}

	// Remove conf file
	if err := os.Remove(confPath); err != nil {
		return fmt.Errorf("removing conf: %w", err)
	}

	return nil
}

// Verify verifies a module's integrity.
func Verify(moduleID, swordPath string) (*VerifyResult, error) {
	if swordPath == "" {
		swordPath = "."
	}

	result := &VerifyResult{
		Module: moduleID,
		Valid:  false,
	}

	// Check conf file exists
	confPath := filepath.Join(swordPath, "mods.d", strings.ToLower(moduleID)+".conf")
	if _, err := os.Stat(confPath); errors.Is(err, os.ErrNotExist) {
		result.Errors = append(result.Errors, "module not installed")
		return result, nil
	}

	// Read and parse conf
	data, err := os.ReadFile(confPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("cannot read conf: %v", err))
		return result, nil
	}

	module, err := ParseModuleConf(data)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("invalid conf: %v", err))
		return result, nil
	}

	// Check data path exists
	if module.DataPath != "" {
		dataPath := filepath.Join(swordPath, module.DataPath)
		dataPath = strings.TrimPrefix(dataPath, "./")
		info, err := os.Stat(dataPath)
		if errors.Is(err, os.ErrNotExist) {
			result.Errors = append(result.Errors, "data directory missing")
			return result, nil
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("cannot stat data: %v", err))
			return result, nil
		}
		if !info.IsDir() {
			result.Errors = append(result.Errors, "data path is not a directory")
			return result, nil
		}

		// Check for data files
		entries, err := os.ReadDir(dataPath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("cannot read data dir: %v", err))
			return result, nil
		}
		if len(entries) == 0 {
			result.Warnings = append(result.Warnings, "data directory is empty")
		}
	}

	result.Valid = len(result.Errors) == 0
	return result, nil
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
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
			continue
		}

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
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
