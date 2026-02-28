package repository

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ModuleType represents the type of SWORD module
type ModuleType string

const (
	ModuleTypeBible      ModuleType = "Bible"
	ModuleTypeCommentary ModuleType = "Commentary"
	ModuleTypeDictionary ModuleType = "Dictionary"
	ModuleTypeGenBook    ModuleType = "GenBook"
	ModuleTypeUnknown    ModuleType = "Unknown"
)

// ModuleInfo contains metadata about a SWORD module
type ModuleInfo struct {
	ID          string   // Module identifier (e.g., "KJV")
	Description string   // Human-readable description
	Language    string   // Language code (e.g., "en")
	Version     string   // Module version
	DataPath    string   // Path to module data
	Driver      string   // Module driver (e.g., "zText")
	SourceType  string   // Source markup type (e.g., "OSIS")
	Encoding    string   // Text encoding (e.g., "UTF-8")
	Features    []string // Module features (e.g., "StrongsNumbers")
	About       string   // Extended description
	Copyright   string   // Copyright information
	License     string   // Distribution license (e.g., "Public Domain", "GPL")
	ConfPath    string   // Path to the .conf file (when installed)
	InstallSize int64    // Expected size in bytes (from InstallSize field)
}

// LicenseSPDX returns the SPDX identifier for the license
func (m *ModuleInfo) LicenseSPDX() string {
	return ToSPDXLicense(m.License)
}

// ToSPDXLicense converts a SWORD DistributionLicense string to an SPDX identifier
func ToSPDXLicense(license string) string {
	lower := strings.ToLower(strings.TrimSpace(license))

	switch {
	case lower == "" || lower == "-":
		return ""
	case lower == "public domain", strings.Contains(lower, "public domain"):
		return "CC-PDDC"
	case lower == "gpl":
		return "GPL-3.0-or-later"
	case lower == "unrestricted":
		return "Unlicense"

	// Creative Commons variants
	case strings.Contains(lower, "cc0"):
		return "CC0-1.0"
	case strings.Contains(lower, "by-nc-nd 4.0") || (strings.Contains(lower, "by-nc-nd") && strings.Contains(lower, "4.0")):
		return "CC-BY-NC-ND-4.0"
	case strings.Contains(lower, "by-nc-nd"):
		return "CC-BY-NC-ND-3.0"
	case strings.Contains(lower, "by-nc-sa 4.0") || (strings.Contains(lower, "by-nc-sa") && strings.Contains(lower, "4.0")):
		return "CC-BY-NC-SA-4.0"
	case strings.Contains(lower, "by-nc-sa"):
		return "CC-BY-NC-SA-3.0"
	case strings.Contains(lower, "by-sa 4.0") || (strings.Contains(lower, "by-sa") && strings.Contains(lower, "4.0")):
		return "CC-BY-SA-4.0"
	case strings.Contains(lower, "by-sa"):
		return "CC-BY-SA-3.0"
	case strings.Contains(lower, "by-nd 4.0") || (strings.Contains(lower, "by-nd") && strings.Contains(lower, "4.0")):
		return "CC-BY-ND-4.0"
	case strings.Contains(lower, "by-nd"):
		return "CC-BY-ND-3.0"
	case strings.Contains(lower, "by 4.0") || (strings.Contains(lower, "attribution") && strings.Contains(lower, "4.0")):
		return "CC-BY-4.0"
	case strings.Contains(lower, "creative commons: by"):
		return "CC-BY-3.0"

	// Copyrighted variants - use LicenseRef for non-standard
	case strings.Contains(lower, "copyrighted") && strings.Contains(lower, "free"):
		return "LicenseRef-Copyrighted-Free"
	case strings.Contains(lower, "copyrighted"):
		return "LicenseRef-Copyrighted"

	default:
		// Return original if no match
		return license
	}
}

// Type returns the module type based on the driver
func (m *ModuleInfo) Type() ModuleType {
	driver := strings.ToLower(m.Driver)
	switch {
	case strings.HasPrefix(driver, "ztext"), strings.HasPrefix(driver, "rawtext"):
		return ModuleTypeBible
	case strings.HasPrefix(driver, "zcom"), strings.HasPrefix(driver, "rawcom"):
		return ModuleTypeCommentary
	case strings.HasPrefix(driver, "zld"), strings.HasPrefix(driver, "rawld"):
		return ModuleTypeDictionary
	case strings.Contains(driver, "genbook"):
		return ModuleTypeGenBook
	default:
		return ModuleTypeUnknown
	}
}

// IsBible returns true if this is a Bible module
func (m *ModuleInfo) IsBible() bool {
	return m.Type() == ModuleTypeBible
}

// HasFeature checks if the module has a specific feature
func (m *ModuleInfo) HasFeature(feature string) bool {
	for _, f := range m.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// String returns a string representation of the module
func (m *ModuleInfo) String() string {
	return fmt.Sprintf("%s: %s (v%s)", m.ID, m.Description, m.Version)
}

// ParseModsArchive parses a mods.d.tar.gz archive and returns module info
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
			// Log but continue - may be corrupted entries
			break
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

		module, err := ParseModuleConf(content, filepath.Base(hdr.Name))
		if err != nil {
			continue // Skip invalid conf files
		}

		modules = append(modules, module)
	}

	return modules, nil
}

// ParseModuleConf parses a SWORD module .conf file
func ParseModuleConf(data []byte, filename string) (ModuleInfo, error) {
	if len(data) == 0 {
		return ModuleInfo{}, errors.New("empty conf file")
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return ModuleInfo{}, errors.New("empty conf file")
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
		return ModuleInfo{}, errors.New("no section header found")
	}

	module := ModuleInfo{
		ID:       moduleID,
		ConfPath: filename,
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
		case "DataPath":
			module.DataPath = value
		case "ModDrv":
			module.Driver = value
		case "SourceType":
			module.SourceType = value
		case "Encoding":
			module.Encoding = value
		case "About":
			module.About = value
		case "Copyright":
			module.Copyright = value
		case "DistributionLicense":
			module.License = value
		case "Feature":
			module.Features = append(module.Features, value)
		case "InstallSize":
			if size, err := strconv.ParseInt(value, 10, 64); err == nil {
				module.InstallSize = size
			}
		}
	}

	return module, nil
}

// ExtractModsArchive extracts a mods.d.tar.gz to a directory
func ExtractModsArchive(data []byte, destDir string) error {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to decompress: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		destPath := filepath.Join(destDir, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0700); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
		case tar.TypeReg:
			// Create parent directory
			if err := os.MkdirAll(filepath.Dir(destPath), 0700); err != nil {
				return fmt.Errorf("creating parent directory: %w", err)
			}

			// Create file
			f, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("creating file: %w", err)
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("writing file: %w", err)
			}
			f.Close()
		}
	}

	return nil
}

// ExtractZipArchive extracts a .zip archive to a directory
func ExtractZipArchive(data []byte, destDir string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}

	for _, f := range r.File {
		destPath := filepath.Join(destDir, f.Name)

		// Check for directory traversal attack
		if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(destDir)+string(os.PathSeparator)) && destPath != filepath.Clean(destDir) {
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

// FilterByType returns modules matching the specified type
func FilterByType(modules []ModuleInfo, moduleType ModuleType) []ModuleInfo {
	var result []ModuleInfo
	for _, m := range modules {
		if m.Type() == moduleType {
			result = append(result, m)
		}
	}
	return result
}

// FilterByLanguage returns modules matching the specified language
func FilterByLanguage(modules []ModuleInfo, lang string) []ModuleInfo {
	var result []ModuleInfo
	for _, m := range modules {
		if m.Language == lang {
			result = append(result, m)
		}
	}
	return result
}

// SearchModules searches modules by keyword in ID and Description
func SearchModules(modules []ModuleInfo, keyword string) []ModuleInfo {
	keyword = strings.ToLower(keyword)
	var result []ModuleInfo
	for _, m := range modules {
		if strings.Contains(strings.ToLower(m.ID), keyword) ||
			strings.Contains(strings.ToLower(m.Description), keyword) {
			result = append(result, m)
		}
	}
	return result
}
