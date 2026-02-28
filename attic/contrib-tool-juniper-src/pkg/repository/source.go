// Package repository provides SWORD module repository management
// functionality, replacing the installmgr tool with a native Go implementation.
package repository

import (
	"errors"
	"fmt"
	"strings"
)

// SourceType represents the protocol type for a remote source
type SourceType string

const (
	SourceTypeFTP   SourceType = "FTP"
	SourceTypeHTTP  SourceType = "HTTP"
	SourceTypeHTTPS SourceType = "HTTPS"
)

// String returns the string representation of a SourceType
func (st SourceType) String() string {
	return string(st)
}

// Source represents a remote SWORD module repository
type Source struct {
	Name      string     // Human-readable name (e.g., "CrossWire")
	Type      SourceType // FTP, HTTP, or HTTPS
	Host      string     // Hostname (e.g., "ftp.crosswire.org")
	Directory string     // Base directory path (e.g., "/pub/sword/raw")
}

// Validate checks if the source configuration is valid
func (s *Source) Validate() error {
	if s.Name == "" {
		return errors.New("source name cannot be empty")
	}
	if s.Host == "" {
		return errors.New("source host cannot be empty")
	}
	if s.Directory == "" {
		return errors.New("source directory cannot be empty")
	}
	switch s.Type {
	case SourceTypeFTP, SourceTypeHTTP, SourceTypeHTTPS:
		// Valid types
	default:
		return fmt.Errorf("invalid source type: %s", s.Type)
	}
	return nil
}

// BaseURL returns the base URL for the source (without directory path)
func (s *Source) BaseURL() string {
	var scheme string
	switch s.Type {
	case SourceTypeFTP:
		scheme = "ftp"
	case SourceTypeHTTP:
		scheme = "http"
	case SourceTypeHTTPS:
		scheme = "https"
	default:
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s", scheme, s.Host)
}

// ModsIndexURL returns the full URL to the mods.d.tar.gz index file
func (s *Source) ModsIndexURL() string {
	dir := strings.TrimSuffix(s.Directory, "/")
	return fmt.Sprintf("%s%s/mods.d.tar.gz", s.BaseURL(), dir)
}

// ModuleDataURL returns the full URL to a module's data directory
func (s *Source) ModuleDataURL(dataPath string) string {
	// Remove leading "./" if present
	dataPath = strings.TrimPrefix(dataPath, "./")
	dir := strings.TrimSuffix(s.Directory, "/")
	return fmt.Sprintf("%s%s/%s", s.BaseURL(), dir, dataPath)
}

// ModulePackageURL returns the full URL to a module's zip package
// SWORD modules are distributed as .zip files in various locations depending on the source
func (s *Source) ModulePackageURL(moduleID string) string {
	dir := strings.TrimSuffix(s.Directory, "/")

	// Different sources have different package directory structures:
	// CrossWire: /pub/sword/raw -> /pub/sword/packages/rawzip/
	// IBT: /pub/modsword/raw -> /pub/modsword/rawzip/
	// eBible: /sword -> /sword/zip/
	if strings.HasSuffix(dir, "raw") {
		// Try CrossWire-style first: raw -> packages/rawzip
		dir = strings.TrimSuffix(dir, "raw") + "packages/rawzip"
	} else {
		// For non-raw directories like eBible's /sword, try /zip subdirectory
		dir = dir + "/zip"
	}

	return fmt.Sprintf("%s%s/%s.zip", s.BaseURL(), dir, moduleID)
}

// ModulePackageURLs returns possible URLs to try for a module's zip package
// Different sources have different directory structures
func (s *Source) ModulePackageURLs(moduleID string) []string {
	dir := strings.TrimSuffix(s.Directory, "/")
	base := s.BaseURL()

	var urls []string

	if strings.HasSuffix(dir, "raw") {
		parent := strings.TrimSuffix(dir, "raw")
		// CrossWire-style: raw -> packages/rawzip (e.g., /pub/sword/raw -> /pub/sword/packages/rawzip)
		urls = append(urls, fmt.Sprintf("%s%spackages/rawzip/%s.zip", base, parent, moduleID))
		// CrossWire variant sources: {name}raw -> {name}packages (e.g., lockmanraw -> lockmanpackages)
		urls = append(urls, fmt.Sprintf("%s%spackages/%s.zip", base, parent, moduleID))
		// IBT-style: raw -> rawzip (sibling to raw)
		urls = append(urls, fmt.Sprintf("%s%srawzip/%s.zip", base, parent, moduleID))
	} else {
		// eBible-style: /zip subdirectory
		urls = append(urls, fmt.Sprintf("%s%s/zip/%s.zip", base, dir, moduleID))
		// Also try packages/rawzip
		urls = append(urls, fmt.Sprintf("%s%s/packages/rawzip/%s.zip", base, dir, moduleID))
	}

	return urls
}

// DefaultSources returns the list of default SWORD module sources
// These match the sources from the official installmgr tool
func DefaultSources() []Source {
	return []Source{
		{
			Name:      "Bible.org",
			Type:      SourceTypeFTP,
			Host:      "ftp.crosswire.org",
			Directory: "/pub/bible.org/sword",
		},
		{
			Name:      "CrossWire",
			Type:      SourceTypeFTP,
			Host:      "ftp.crosswire.org",
			Directory: "/pub/sword/raw",
		},
		{
			Name:      "CrossWire Attic",
			Type:      SourceTypeFTP,
			Host:      "ftp.crosswire.org",
			Directory: "/pub/sword/atticraw",
		},
		{
			Name:      "CrossWire Beta",
			Type:      SourceTypeFTP,
			Host:      "ftp.crosswire.org",
			Directory: "/pub/sword/betaraw",
		},
		{
			Name:      "CrossWire Wycliffe",
			Type:      SourceTypeFTP,
			Host:      "ftp.crosswire.org",
			Directory: "/pub/sword/wyclifferaw",
		},
		{
			Name:      "Deutsche Bibelgesellschaft",
			Type:      SourceTypeFTP,
			Host:      "ftp.crosswire.org",
			Directory: "/pub/sword/dbgraw",
		},
		{
			Name:      "IBT",
			Type:      SourceTypeFTP,
			Host:      "ftp.ibt.org.ru",
			Directory: "/pub/modsword/raw",
		},
		{
			Name:      "Lockman Foundation",
			Type:      SourceTypeFTP,
			Host:      "ftp.crosswire.org",
			Directory: "/pub/sword/lockmanraw",
		},
		{
			Name:      "STEP Bible",
			Type:      SourceTypeFTP,
			Host:      "ftp.stepbible.org",
			Directory: "/pub/sword",
		},
		{
			Name:      "Xiphos",
			Type:      SourceTypeFTP,
			Host:      "ftp.xiphos.org",
			Directory: "/pub/xiphos",
		},
		{
			Name:      "eBible.org",
			Type:      SourceTypeFTP,
			Host:      "ftp.ebible.org",
			Directory: "/sword",
		},
	}
}

// GetSource retrieves a source by name from the default sources
func GetSource(name string) (Source, bool) {
	for _, s := range DefaultSources() {
		if s.Name == name {
			return s, true
		}
	}
	return Source{}, false
}

// ParseSourcesConf parses an install.conf file format used by SWORD
// Format: [SectionName]\nFTPSource=host|directory|name
func ParseSourcesConf(data []byte) ([]Source, error) {
	var sources []Source
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Parse FTPSource entries
		if strings.HasPrefix(line, "FTPSource=") {
			parts := strings.Split(strings.TrimPrefix(line, "FTPSource="), "|")
			if len(parts) == 3 {
				sources = append(sources, Source{
					Name:      strings.TrimSpace(parts[2]),
					Type:      SourceTypeFTP,
					Host:      strings.TrimSpace(parts[0]),
					Directory: strings.TrimSpace(parts[1]),
				})
			}
		}

		// Parse HTTPSource entries
		if strings.HasPrefix(line, "HTTPSource=") {
			parts := strings.Split(strings.TrimPrefix(line, "HTTPSource="), "|")
			if len(parts) == 3 {
				sources = append(sources, Source{
					Name:      strings.TrimSpace(parts[2]),
					Type:      SourceTypeHTTP,
					Host:      strings.TrimSpace(parts[0]),
					Directory: strings.TrimSpace(parts[1]),
				})
			}
		}
	}

	return sources, nil
}
