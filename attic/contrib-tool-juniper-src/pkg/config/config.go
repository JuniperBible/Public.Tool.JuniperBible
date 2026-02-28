// Package config handles loading and validation of converter configuration.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Granularity defines the page generation level.
type Granularity string

const (
	GranularityBook    Granularity = "book"
	GranularityChapter Granularity = "chapter"
	GranularityVerse   Granularity = "verse"
)

// Config holds the converter configuration.
type Config struct {
	SwordDir    string      `yaml:"swordDir"`
	ESwordDir   string      `yaml:"eswordDir"`
	OutputDir   string      `yaml:"outputDir"`
	ContentDir  string      `yaml:"contentDir"`
	Granularity Granularity `yaml:"granularity"`
	Modules     []string    `yaml:"modules"`
	Filters     Filters     `yaml:"filters"`
	Output      Output      `yaml:"output"`
}

// Filters specifies which modules to process.
type Filters struct {
	Languages []string `yaml:"languages"`
	Types     []string `yaml:"types"`
}

// Output specifies output options.
type Output struct {
	PreserveStrongs     bool `yaml:"preserveStrongs"`
	PreserveMorphology  bool `yaml:"preserveMorphology"`
	GenerateSearchIndex bool `yaml:"generateSearchIndex"`
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		SwordDir:    filepath.Join(home, ".sword"),
		ESwordDir:   filepath.Join(home, "e-sword"),
		OutputDir:   "sword_data",
		ContentDir:  "content/religion",
		Granularity: GranularityChapter,
		Modules:     []string{},
		Filters: Filters{
			Languages: []string{},
			Types:     []string{"Bible", "Commentary", "Dictionary"},
		},
		Output: Output{
			PreserveStrongs:     true,
			PreserveMorphology:  true,
			GenerateSearchIndex: false,
		},
	}
}

// Load reads configuration from a YAML file.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Use defaults if config doesn't exist
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Expand home directory in paths
	cfg.SwordDir = expandPath(cfg.SwordDir)
	cfg.ESwordDir = expandPath(cfg.ESwordDir)

	return cfg, nil
}

// expandPath expands ~ to the user's home directory.
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
