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

	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/safefile"
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

	return parseConfEntries(entries, modsDir), nil
}

func parseConfEntries(entries []os.DirEntry, modsDir string) []ModuleInfo {
	var modules []ModuleInfo
	for _, entry := range entries {
		if module, ok := parseConfEntry(entry, modsDir); ok {
			modules = append(modules, module)
		}
	}
	return modules
}

func parseConfEntry(entry os.DirEntry, modsDir string) (ModuleInfo, bool) {
	if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") {
		return ModuleInfo{}, false
	}
	confPath := filepath.Join(modsDir, entry.Name())
	data, err := safefile.ReadFile(confPath)
	if err != nil {
		return ModuleInfo{}, false
	}
	module, err := ParseModuleConf(data)
	if err != nil {
		return ModuleInfo{}, false
	}
	return module, true
}

// Uninstall removes an installed module.
func Uninstall(moduleID, swordPath string) error {
	if swordPath == "" {
		swordPath = "."
	}

	confPath := filepath.Join(swordPath, "mods.d", strings.ToLower(moduleID)+".conf")
	module, err := readUninstallModule(moduleID, confPath)
	if err != nil {
		return err
	}

	if err := removeModuleData(swordPath, module.DataPath); err != nil {
		return err
	}

	if err := os.Remove(confPath); err != nil {
		return fmt.Errorf("removing conf: %w", err)
	}
	return nil
}

func readUninstallModule(moduleID, confPath string) (ModuleInfo, error) {
	if _, err := os.Stat(confPath); errors.Is(err, os.ErrNotExist) {
		return ModuleInfo{}, fmt.Errorf("module %s not installed", moduleID)
	}
	data, err := os.ReadFile(confPath)
	if err != nil {
		return ModuleInfo{}, fmt.Errorf("reading conf: %w", err)
	}
	module, err := ParseModuleConf(data)
	if err != nil {
		return ModuleInfo{}, fmt.Errorf("parsing conf: %w", err)
	}
	return module, nil
}

func removeModuleData(swordPath, dataPath string) error {
	if dataPath == "" {
		return nil
	}
	fullPath := filepath.Join(swordPath, dataPath)
	fullPath = strings.TrimPrefix(fullPath, "./")
	if err := os.RemoveAll(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing data: %w", err)
	}
	return nil
}

func loadModuleConf(moduleID, swordPath string, result *VerifyResult) (*ModuleInfo, bool) {
	confPath := filepath.Join(swordPath, "mods.d", strings.ToLower(moduleID)+".conf")
	if _, err := os.Stat(confPath); errors.Is(err, os.ErrNotExist) {
		result.Errors = append(result.Errors, "module not installed")
		return nil, false
	}
	data, err := os.ReadFile(confPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("cannot read conf: %v", err))
		return nil, false
	}
	module, err := ParseModuleConf(data)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("invalid conf: %v", err))
		return nil, false
	}
	return &module, true
}

func verifyDataPath(dataPath string, result *VerifyResult) {
	info, err := os.Stat(dataPath)
	if errors.Is(err, os.ErrNotExist) {
		result.Errors = append(result.Errors, "data directory missing")
		return
	}
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("cannot stat data: %v", err))
		return
	}
	if !info.IsDir() {
		result.Errors = append(result.Errors, "data path is not a directory")
		return
	}
	entries, err := os.ReadDir(dataPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("cannot read data dir: %v", err))
		return
	}
	if len(entries) == 0 {
		result.Warnings = append(result.Warnings, "data directory is empty")
	}
}

// Verify verifies a module's integrity.
func Verify(moduleID, swordPath string) (*VerifyResult, error) {
	if swordPath == "" {
		swordPath = "."
	}
	result := &VerifyResult{Module: moduleID}
	module, ok := loadModuleConf(moduleID, swordPath, result)
	if !ok {
		return result, nil
	}
	if module.DataPath != "" {
		dataPath := strings.TrimPrefix(filepath.Join(swordPath, module.DataPath), "./")
		verifyDataPath(dataPath, result)
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

	return parseModsFromTar(tar.NewReader(gzr))
}

// parseModsFromTar reads module configs from a tar reader
func parseModsFromTar(tr *tar.Reader) ([]ModuleInfo, error) {
	var modules []ModuleInfo
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		if module, ok := parseModuleFromTarEntry(tr, hdr); ok {
			modules = append(modules, module)
		}
	}
	return modules, nil
}

// parseModuleFromTarEntry parses a single tar entry as a module conf
func parseModuleFromTarEntry(tr *tar.Reader, hdr *tar.Header) (ModuleInfo, bool) {
	if hdr.Typeflag == tar.TypeDir || !strings.HasSuffix(hdr.Name, ".conf") {
		return ModuleInfo{}, false
	}
	content := make([]byte, hdr.Size)
	if _, err := io.ReadFull(tr, content); err != nil {
		return ModuleInfo{}, false
	}
	module, err := ParseModuleConf(content)
	return module, err == nil
}

// confFieldSetters maps known .conf keys to the corresponding ModuleInfo field setter.
var confFieldSetters = map[string]func(*ModuleInfo, string){
	"Description": func(m *ModuleInfo, v string) { m.Description = v },
	"Lang":        func(m *ModuleInfo, v string) { m.Language = v },
	"Version":     func(m *ModuleInfo, v string) { m.Version = v },
	"ModDrv":      func(m *ModuleInfo, v string) { m.Type = moduleTypeFromDriver(v) },
	"DataPath":    func(m *ModuleInfo, v string) { m.DataPath = v },
}

// parseSectionHeader scans lines for the first "[ModuleID]" header and returns
// the module ID, or an error when none is found.
func parseSectionHeader(lines []string) (string, error) {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			return strings.Trim(line, "[]"), nil
		}
	}
	return "", fmt.Errorf("no section header found")
}

// isConfMetaLine reports whether a trimmed line should be skipped during
// key-value parsing (blank lines, section headers, and comments).
func isConfMetaLine(line string) bool {
	return line == "" || line[0] == '[' || line[0] == '#'
}

// applyConfLine parses a single trimmed conf line and, when it contains a
// recognised key, calls the corresponding field setter on module.
func applyConfLine(line string, module *ModuleInfo) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if setter, known := confFieldSetters[key]; known {
		setter(module, value)
	}
}

// ParseModuleConf parses a SWORD module .conf file.
func ParseModuleConf(data []byte) (ModuleInfo, error) {
	if len(data) == 0 {
		return ModuleInfo{}, fmt.Errorf("empty conf file")
	}

	lines := strings.Split(string(data), "\n")

	moduleID, err := parseSectionHeader(lines)
	if err != nil {
		return ModuleInfo{}, err
	}

	module := ModuleInfo{Name: moduleID}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if isConfMetaLine(line) {
			continue
		}
		applyConfLine(line, &module)
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

func zipPathIsSafe(destDir, destPath string) bool {
	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
	cleanPath := filepath.Clean(destPath)
	return strings.HasPrefix(cleanPath, cleanDest) || cleanPath == filepath.Clean(destDir)
}

func writeZipFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("opening file in zip: %w", err)
	}
	defer rc.Close()

	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, rc); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	return nil
}

func extractZipEntry(f *zip.File, destDir string) error {
	destPath := filepath.Join(destDir, f.Name)
	if !zipPathIsSafe(destDir, destPath) {
		return fmt.Errorf("invalid file path in zip: %s", f.Name)
	}
	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(destPath, 0700); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0700); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}
	return writeZipFile(f, destPath)
}

func ExtractZipArchive(data []byte, destDir string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	for _, f := range r.File {
		if err := extractZipEntry(f, destDir); err != nil {
			return err
		}
	}
	return nil
}
