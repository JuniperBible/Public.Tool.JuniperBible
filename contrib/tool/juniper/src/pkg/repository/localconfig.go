package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LocalConfig represents the local SWORD configuration (~/.sword)
type LocalConfig struct {
	SwordDir string // Path to the .sword directory
}

// DefaultSwordDir returns the default SWORD directory path
func DefaultSwordDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".sword"
	}
	return filepath.Join(home, ".sword")
}

// NewLocalConfig creates a new LocalConfig for the given directory
func NewLocalConfig(swordDir string) *LocalConfig {
	return &LocalConfig{SwordDir: swordDir}
}

// LoadLocalConfig loads the local SWORD configuration from a directory
func LoadLocalConfig(swordDir string) (*LocalConfig, error) {
	info, err := os.Stat(swordDir)
	if err != nil {
		return nil, fmt.Errorf("sword directory not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", swordDir)
	}

	return &LocalConfig{SwordDir: swordDir}, nil
}

// ModsDir returns the path to the mods.d directory
func (c *LocalConfig) ModsDir() string {
	return filepath.Join(c.SwordDir, "mods.d")
}

// ModulesDir returns the path to the modules directory
func (c *LocalConfig) ModulesDir() string {
	return filepath.Join(c.SwordDir, "modules")
}

// EnsureDirectories creates the required directory structure
func (c *LocalConfig) EnsureDirectories() error {
	dirs := []string{
		c.SwordDir,
		c.ModsDir(),
		c.ModulesDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	return nil
}

// ListInstalledModules returns all installed modules
func (c *LocalConfig) ListInstalledModules() ([]ModuleInfo, error) {
	modsDir := c.ModsDir()

	entries, err := os.ReadDir(modsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading mods.d: %w", err)
	}

	var modules []ModuleInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") {
			continue
		}
		if entry.Name() == "install.conf" {
			continue // Skip install.conf
		}

		confPath := filepath.Join(modsDir, entry.Name())
		data, err := os.ReadFile(confPath) // #nosec G304 -- path is validated
		if err != nil {
			continue
		}

		module, err := ParseModuleConf(data, entry.Name())
		if err != nil {
			continue
		}

		module.ConfPath = confPath
		modules = append(modules, module)
	}

	return modules, nil
}

// GetInstalledModule returns a specific installed module by ID
func (c *LocalConfig) GetInstalledModule(moduleID string) (ModuleInfo, bool) {
	modules, err := c.ListInstalledModules()
	if err != nil {
		return ModuleInfo{}, false
	}

	for _, m := range modules {
		if m.ID == moduleID {
			return m, true
		}
	}

	return ModuleInfo{}, false
}

// IsModuleInstalled checks if a module is installed
func (c *LocalConfig) IsModuleInstalled(moduleID string) bool {
	_, found := c.GetInstalledModule(moduleID)
	return found
}

// GetModuleDataPath returns the full path to a module's data directory
func (c *LocalConfig) GetModuleDataPath(dataPath string) string {
	// Remove leading "./"
	dataPath = strings.TrimPrefix(dataPath, "./")
	// Remove trailing "/"
	dataPath = strings.TrimSuffix(dataPath, "/")
	return filepath.Join(c.SwordDir, dataPath)
}

// SaveInstallConf saves the sources configuration to install.conf
func (c *LocalConfig) SaveInstallConf(sources []Source) error {
	var sb strings.Builder
	sb.WriteString("[General]\n\n")

	for _, source := range sources {
		sb.WriteString(fmt.Sprintf("[%s]\n", source.Name))
		switch source.Type {
		case SourceTypeFTP:
			sb.WriteString(fmt.Sprintf("FTPSource=%s|%s|%s\n", source.Host, source.Directory, source.Name))
		case SourceTypeHTTP:
			sb.WriteString(fmt.Sprintf("HTTPSource=%s|%s|%s\n", source.Host, source.Directory, source.Name))
		case SourceTypeHTTPS:
			sb.WriteString(fmt.Sprintf("HTTPSSource=%s|%s|%s\n", source.Host, source.Directory, source.Name))
		}
		sb.WriteString("\n")
	}

	confPath := filepath.Join(c.ModsDir(), "install.conf")
	return os.WriteFile(confPath, []byte(sb.String()), 0644)
}

// LoadInstallConf loads the sources configuration from install.conf
func (c *LocalConfig) LoadInstallConf() ([]Source, error) {
	confPath := filepath.Join(c.ModsDir(), "install.conf")

	data, err := os.ReadFile(confPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty list if file doesn't exist
			return nil, nil
		}
		return nil, fmt.Errorf("reading install.conf: %w", err)
	}

	return ParseSourcesConf(data)
}

// GetModuleActualSize calculates the actual size of installed module data on disk
func (c *LocalConfig) GetModuleActualSize(dataPath string) (int64, error) {
	fullPath := c.GetModuleDataPath(dataPath)

	var totalSize int64
	err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return totalSize, nil
}
