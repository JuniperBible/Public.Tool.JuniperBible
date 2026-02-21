// conf.go implements SWORD .conf file parsing.
// SWORD conf files are INI-like configuration files that describe module metadata.
package swordpure

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/safefile"
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

// confScanState holds mutable state threaded through the conf file scan loop.
type confScanState struct {
	currentSection string
	multilineKey   string
	multilineValue strings.Builder
}

// flushMultiline commits any pending multiline key/value to the conf and resets state.
func (s *confScanState) flushMultiline(conf *ConfFile) {
	if s.multilineKey == "" {
		return
	}
	conf.setProperty(s.multilineKey, strings.TrimSpace(s.multilineValue.String()))
	s.multilineKey = ""
	s.multilineValue.Reset()
}

// handleSectionHeader processes a "[SectionName]" line, flushing any pending
// multiline value and recording the module name on first encounter.
func (s *confScanState) handleSectionHeader(line string, conf *ConfFile) {
	s.flushMultiline(conf)
	s.currentSection = strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[")
	if conf.ModuleName == "" {
		conf.ModuleName = s.currentSection
	}
}

// handleLineContinuation appends a continuation line to the active multiline value.
// Returns true if the line was consumed as a continuation.
func (s *confScanState) handleLineContinuation(line string) bool {
	if s.multilineKey == "" || len(line) == 0 {
		return false
	}
	if line[0] != ' ' && line[0] != '\t' {
		return false
	}
	s.multilineValue.WriteString(" ")
	s.multilineValue.WriteString(strings.TrimSpace(line))
	return true
}

// handleKeyValue parses a "key=value" line. If the value ends with "\", it
// begins a multiline sequence; otherwise it sets the property immediately.
// Returns false if the line contains no "=" and should be skipped.
func (s *confScanState) handleKeyValue(line string, conf *ConfFile) bool {
	idx := strings.Index(line, "=")
	if idx == -1 {
		return false
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	if strings.HasSuffix(value, "\\") {
		s.multilineKey = key
		s.multilineValue.WriteString(strings.TrimSuffix(value, "\\"))
		return true
	}
	conf.setProperty(key, value)
	return true
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

	if err := scanConfFile(f, conf); err != nil {
		return nil, err
	}
	return conf, nil
}

func scanConfFile(f *os.File, conf *ConfFile) error {
	scanner := bufio.NewScanner(f)
	var state confScanState

	for scanner.Scan() {
		processConfLine(scanner.Text(), &state, conf)
	}
	state.flushMultiline(conf)

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading conf file: %w", err)
	}
	return nil
}

func processConfLine(line string, state *confScanState, conf *ConfFile) {
	if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
		state.handleSectionHeader(line, conf)
		return
	}

	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return
	}

	if state.handleLineContinuation(line) {
		return
	}

	state.flushMultiline(conf)
	state.handleKeyValue(line, conf)
}

// confFieldMap returns a map from lowercase conf-file key to the corresponding
// *string field on c. Only the keys that have dedicated struct fields are listed;
// everything else is stored exclusively in Properties.
func (c *ConfFile) confFieldMap() map[string]*string {
	return map[string]*string{
		"description":         &c.Description,
		"datapath":            &c.DataPath,
		"moddrv":              &c.ModDrv,
		"encoding":            &c.Encoding,
		"lang":                &c.Lang,
		"version":             &c.Version,
		"about":               &c.About,
		"copyright":           &c.Copyright,
		"distributionlicense": &c.License,
		"category":            &c.Category,
		"lcsh":                &c.LCSH,
		"sourcetype":          &c.SourceType,
		"blocktype":           &c.BlockType,
		"compresstype":        &c.CompressType,
		"cipherkey":           &c.CipherKey,
		"versification":       &c.Versification,
	}
}

// setProperty sets a property on the ConfFile, mapping known keys to struct fields.
func (c *ConfFile) setProperty(key, value string) {
	c.Properties[key] = value
	if field, ok := c.confFieldMap()[strings.ToLower(key)]; ok {
		*field = value
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
