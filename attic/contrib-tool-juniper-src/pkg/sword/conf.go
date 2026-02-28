package sword

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ParseConf parses a SWORD .conf file and returns module metadata.
// Uses Participle parser for structured parsing.
func ParseConf(confPath string) (*Module, error) {
	data, err := os.ReadFile(confPath) // #nosec G304 -- path is validated
	if err != nil {
		return nil, fmt.Errorf("failed to open conf file: %w", err)
	}

	confFile, err := parseConfBytes(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse conf file: %w", err)
	}

	return confFile.ToModule(confPath), nil
}

// parseAboutText converts SWORD About field RTF-like encoding to plain text.
func parseAboutText(text string) string {
	// Replace \par with newlines
	text = strings.ReplaceAll(text, "\\par\\par", "\n\n")
	text = strings.ReplaceAll(text, "\\par ", "\n")
	text = strings.ReplaceAll(text, "\\par", "\n")

	// Remove other RTF-like escapes
	text = strings.ReplaceAll(text, "\\qc", "")
	text = strings.ReplaceAll(text, "\\pard", "")

	return strings.TrimSpace(text)
}

// truncateDescription truncates text to maxLen, ending at a word boundary.
func truncateDescription(text string, maxLen int) string {
	// Take first paragraph
	if idx := strings.Index(text, "\n"); idx > 0 && idx < maxLen {
		text = text[:idx]
	}

	if len(text) <= maxLen {
		return text
	}

	// Find last space before maxLen
	truncated := text[:maxLen]
	if idx := strings.LastIndex(truncated, " "); idx > 0 {
		truncated = truncated[:idx]
	}

	return truncated + "..."
}

// driverToModuleType maps a module driver to its module type.
func driverToModuleType(driver ModuleDriver) ModuleType {
	switch driver {
	case DriverZText, DriverZText4, DriverRawText, DriverRawText4:
		return ModuleTypeBible
	case DriverZCom, DriverZCom4, DriverRawCom, DriverRawCom4:
		return ModuleTypeCommentary
	case DriverZLD, DriverRawLD, DriverRawLD4:
		return ModuleTypeDictionary
	case DriverRawGenBook:
		return ModuleTypeGenBook
	default:
		return ModuleTypeBible
	}
}

// DiscoverModules finds all .conf files in a SWORD mods.d directory.
func DiscoverModules(swordDir string) ([]string, error) {
	modsDir := filepath.Join(swordDir, "mods.d")

	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read mods.d directory: %w", err)
	}

	var confFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".conf") {
			confFiles = append(confFiles, filepath.Join(modsDir, entry.Name()))
		}
	}

	return confFiles, nil
}

// LoadAllModules loads metadata for all modules in a SWORD directory.
func LoadAllModules(swordDir string) ([]*Module, error) {
	confFiles, err := DiscoverModules(swordDir)
	if err != nil {
		return nil, err
	}

	var modules []*Module
	for _, confPath := range confFiles {
		module, err := ParseConf(confPath)
		if err != nil {
			// Log warning but continue with other modules
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", confPath, err)
			continue
		}
		modules = append(modules, module)
	}

	return modules, nil
}

// HasFeature checks if a module has a specific feature.
func (m *Module) HasFeature(feature string) bool {
	for _, f := range m.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// HasStrongsNumbers returns true if the module has Strong's numbers.
func (m *Module) HasStrongsNumbers() bool {
	return m.HasFeature("StrongsNumbers")
}

// HasMorphology returns true if the module has morphology data.
func (m *Module) HasMorphology() bool {
	for _, filter := range m.GlobalOptionFilters {
		if strings.Contains(filter, "Morph") {
			return true
		}
	}
	return false
}

// ResolveDataPath returns the absolute path to the module's data directory.
func (m *Module) ResolveDataPath(swordDir string) string {
	dataPath := m.DataPath

	// Remove leading ./ if present
	dataPath = strings.TrimPrefix(dataPath, "./")

	// SWORD data paths are relative to the SWORD root
	return filepath.Join(swordDir, dataPath)
}
