// conf.go implements SWORD .conf file parsing.
// SWORD conf files are INI-like configuration files that describe module metadata.
package swordpure

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
)

// ConfFile represents a parsed SWORD .conf file.
type ConfFile struct {
	ModuleName    string            `json:"module_name"`
	Description   string            `json:"description"`
	DataPath      string            `json:"data_path"`
	ModDrv        string            `json:"mod_drv"`
	Encoding      string            `json:"encoding"`
	Lang          string            `json:"lang"`
	Version       string            `json:"version"`
	About         string            `json:"about,omitempty"`
	Copyright     string            `json:"copyright,omitempty"`
	License       string            `json:"license,omitempty"`
	Category      string            `json:"category,omitempty"`
	LCSH          string            `json:"lcsh,omitempty"`
	SourceType    string            `json:"source_type,omitempty"`
	BlockType     string            `json:"block_type,omitempty"`
	CompressType  string            `json:"compress_type,omitempty"`
	CipherKey     string            `json:"cipher_key,omitempty"`
	Versification string            `json:"versification,omitempty"`
	Properties    map[string]string `json:"properties"`
	FilePath      string            `json:"file_path,omitempty"`
}

// ParseConfFile parses a SWORD .conf file.
func ParseConfFile(path string) (*ConfFile, error) {
	f, err := safefile.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open conf file: %w", err)
	}
	defer f.Close()

	conf := &ConfFile{
		Properties: make(map[string]string),
		FilePath:   path,
	}

	scanner := bufio.NewScanner(f)
	var currentSection string
	var multilineKey string
	var multilineValue strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// Handle section headers [ModuleName]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			// Flush any pending multiline value
			if multilineKey != "" {
				conf.setProperty(multilineKey, strings.TrimSpace(multilineValue.String()))
				multilineKey = ""
				multilineValue.Reset()
			}
			currentSection = strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[")
			if conf.ModuleName == "" {
				conf.ModuleName = currentSection
			}
			continue
		}

		// Skip empty lines and comments
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Handle line continuation (lines starting with whitespace)
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') && multilineKey != "" {
			multilineValue.WriteString(" ")
			multilineValue.WriteString(strings.TrimSpace(line))
			continue
		}

		// Flush any pending multiline value
		if multilineKey != "" {
			conf.setProperty(multilineKey, strings.TrimSpace(multilineValue.String()))
			multilineKey = ""
			multilineValue.Reset()
		}

		// Parse key=value
		idx := strings.Index(line, "=")
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// Check if this might be a multiline value
		if strings.HasSuffix(value, "\\") {
			multilineKey = key
			multilineValue.WriteString(strings.TrimSuffix(value, "\\"))
			continue
		}

		conf.setProperty(key, value)
	}

	// Flush any remaining multiline value
	if multilineKey != "" {
		conf.setProperty(multilineKey, strings.TrimSpace(multilineValue.String()))
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading conf file: %w", err)
	}

	return conf, nil
}

// setProperty sets a property on the ConfFile, mapping known keys to struct fields.
func (c *ConfFile) setProperty(key, value string) {
	// Store in Properties map for all keys
	c.Properties[key] = value

	// Map known keys to struct fields
	switch strings.ToLower(key) {
	case "description":
		c.Description = value
	case "datapath":
		c.DataPath = value
	case "moddrv":
		c.ModDrv = value
	case "encoding":
		c.Encoding = value
	case "lang":
		c.Lang = value
	case "version":
		c.Version = value
	case "about":
		c.About = value
	case "copyright":
		c.Copyright = value
	case "distributionlicense":
		c.License = value
	case "category":
		c.Category = value
	case "lcsh":
		c.LCSH = value
	case "sourcetype":
		c.SourceType = value
	case "blocktype":
		c.BlockType = value
	case "compresstype":
		c.CompressType = value
	case "cipherkey":
		c.CipherKey = value
	case "versification":
		c.Versification = value
	}
}

// ModuleType returns the type of module based on ModDrv.
func (c *ConfFile) ModuleType() string {
	switch strings.ToLower(c.ModDrv) {
	case "ztext", "ztext4", "rawtext", "rawtext4":
		return "Bible"
	case "zcom", "zcom4", "rawcom", "rawcom4":
		return "Commentary"
	case "zld", "rawld", "rawld4":
		return "Dictionary"
	case "rawgenbook":
		return "GenBook"
	default:
		return "Unknown"
	}
}

// IsCompressed returns true if the module uses compression.
func (c *ConfFile) IsCompressed() bool {
	switch strings.ToLower(c.ModDrv) {
	case "ztext", "ztext4", "zcom", "zcom4", "zld":
		return true
	default:
		return false
	}
}

// IsEncrypted returns true if the module is encrypted.
func (c *ConfFile) IsEncrypted() bool {
	return c.CipherKey != ""
}

// FindConfFiles finds all .conf files in a mods.d directory.
func FindConfFiles(modsDir string) ([]string, error) {
	var confFiles []string

	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read mods.d directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".conf") {
			confFiles = append(confFiles, filepath.Join(modsDir, name))
		}
	}

	return confFiles, nil
}

// LoadModulesFromPath loads all modules from a SWORD installation path.
func LoadModulesFromPath(swordPath string) ([]*ConfFile, error) {
	modsDir := filepath.Join(swordPath, "mods.d")

	confFiles, err := FindConfFiles(modsDir)
	if err != nil {
		return nil, err
	}

	var modules []*ConfFile
	for _, confPath := range confFiles {
		conf, err := ParseConfFile(confPath)
		if err != nil {
			// Log warning to stderr (plugins use stdout for JSON IPC)
			fmt.Fprintf(os.Stderr, "warning: failed to parse conf file %s: %v\n", confPath, err)
			continue
		}
		modules = append(modules, conf)
	}

	return modules, nil
}
