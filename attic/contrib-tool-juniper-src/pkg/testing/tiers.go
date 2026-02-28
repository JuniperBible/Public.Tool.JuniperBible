// Package testing provides test infrastructure for juniper.
// This file implements tiered testing support with configurable Bible sets.

package testing

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// TestTier represents the level of testing thoroughness
type TestTier int

const (
	// TierQuick runs minimal integration tests (< 30 seconds)
	TierQuick TestTier = iota

	// TierComprehensive runs full unit + integration tests (< 5 minutes)
	TierComprehensive

	// TierExtensive runs all tests including fuzz tests (< 30 minutes)
	TierExtensive
)

// String returns the human-readable name of the tier
func (t TestTier) String() string {
	switch t {
	case TierQuick:
		return "quick"
	case TierComprehensive:
		return "comprehensive"
	case TierExtensive:
		return "extensive"
	default:
		return "unknown"
	}
}

// Timeout returns the maximum duration for tests in this tier
func (t TestTier) Timeout() time.Duration {
	switch t {
	case TierQuick:
		return 60 * time.Second
	case TierComprehensive:
		return 5 * time.Minute
	case TierExtensive:
		return 30 * time.Minute
	default:
		return time.Minute
	}
}

// BibleSetsConfig represents the structure of bible_sets.yaml
type BibleSetsConfig struct {
	Quick         TierConfig          `yaml:"quick"`
	Comprehensive ComprehensiveConfig `yaml:"comprehensive"`
	Extensive     ExtensiveConfig     `yaml:"extensive"`
	ModuleTypes   map[string][]string `yaml:"module_types"`
	Validation    ValidationConfig    `yaml:"validation"`
}

// TierConfig represents a basic tier configuration
type TierConfig struct {
	Description string   `yaml:"description"`
	Timeout     string   `yaml:"timeout"`
	Bibles      []string `yaml:"bibles"`
}

// ComprehensiveConfig represents the comprehensive tier with categories
type ComprehensiveConfig struct {
	Description     string   `yaml:"description"`
	Timeout         string   `yaml:"timeout"`
	EnglishMajor    []string `yaml:"english_major"`
	EnglishHistoric []string `yaml:"english_historic"`
	NonEnglish      []string `yaml:"non_english"`
	OriginalLangs   []string `yaml:"original_languages"`
	StrongsBibles   []string `yaml:"strongs_bibles"`
	Commentaries    []string `yaml:"commentaries"`
	Dictionaries    []string `yaml:"dictionaries"`
}

// ExtensiveConfig represents the extensive tier configuration
type ExtensiveConfig struct {
	Description string            `yaml:"description"`
	Timeout     string            `yaml:"timeout"`
	DiscoverAll bool              `yaml:"discover_all"`
	Additional  AdditionalModules `yaml:"additional"`
}

// AdditionalModules represents additional modules for extensive testing
type AdditionalModules struct {
	GeneralBooks    []string `yaml:"general_books"`
	AncientVersions []string `yaml:"ancient_versions"`
}

// ValidationConfig represents validation rules
type ValidationConfig struct {
	MinVerses          map[string]int `yaml:"min_verses"`
	ExpectedBooks      map[string]int `yaml:"expected_books"`
	RequiredReferences []string       `yaml:"required_references"`
}

// LoadBibleSets loads the bible_sets.yaml configuration
func LoadBibleSets() (*BibleSetsConfig, error) {
	// Try multiple locations for the config file
	paths := []string{
		"testdata/bible_sets.yaml",
		"../testdata/bible_sets.yaml",
		"../../testdata/bible_sets.yaml",
		filepath.Join("..", "..", "testdata", "bible_sets.yaml"),
		filepath.Join("..", "..", "..", "..", "tools", "juniper", "testdata", "bible_sets.yaml"),
	}

	var data []byte
	var err error
	for _, path := range paths {
		data, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}

	if data == nil {
		return nil, err
	}

	var config BibleSetsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// GetBiblesForTier returns the list of Bible IDs for a given tier
func GetBiblesForTier(tier TestTier) []string {
	config, err := LoadBibleSets()
	if err != nil {
		// Return default quick set if config not found
		return []string{"KJV", "Tyndale", "Geneva1599", "DRC", "VulgClementwordsof"}
	}

	switch tier {
	case TierQuick:
		return config.Quick.Bibles

	case TierComprehensive:
		var bibles []string
		bibles = append(bibles, config.Comprehensive.EnglishMajor...)
		bibles = append(bibles, config.Comprehensive.EnglishHistoric...)
		bibles = append(bibles, config.Comprehensive.NonEnglish...)
		bibles = append(bibles, config.Comprehensive.OriginalLangs...)
		bibles = append(bibles, config.Comprehensive.StrongsBibles...)
		return bibles

	case TierExtensive:
		// For extensive, we'll return comprehensive + additional
		// The actual discovery happens at test runtime
		bibles := GetBiblesForTier(TierComprehensive)
		bibles = append(bibles, config.Extensive.Additional.GeneralBooks...)
		bibles = append(bibles, config.Extensive.Additional.AncientVersions...)
		return bibles

	default:
		return config.Quick.Bibles
	}
}

// GetCommentariesForTier returns the list of commentary IDs for a given tier
func GetCommentariesForTier(tier TestTier) []string {
	if tier == TierQuick {
		return nil // No commentaries in quick tier
	}

	config, err := LoadBibleSets()
	if err != nil {
		return nil
	}

	if tier >= TierComprehensive {
		return config.Comprehensive.Commentaries
	}

	return nil
}

// GetDictionariesForTier returns the list of dictionary IDs for a given tier
func GetDictionariesForTier(tier TestTier) []string {
	if tier == TierQuick {
		return nil // No dictionaries in quick tier
	}

	config, err := LoadBibleSets()
	if err != nil {
		return nil
	}

	if tier >= TierComprehensive {
		return config.Comprehensive.Dictionaries
	}

	return nil
}

// GetValidationRules returns the validation rules for testing
func GetValidationRules() *ValidationConfig {
	config, err := LoadBibleSets()
	if err != nil {
		// Return defaults
		return &ValidationConfig{
			MinVerses: map[string]int{
				"full_bible":    23000,
				"new_testament": 7000,
				"old_testament": 20000,
			},
			ExpectedBooks: map[string]int{
				"protestant": 66,
				"catholic":   73,
				"orthodox":   76,
				"ethiopian":  81,
			},
			RequiredReferences: []string{
				"Gen.1.1",
				"Ps.23.1",
				"John.3.16",
				"Rev.22.21",
			},
		}
	}

	return &config.Validation
}

// QuickBibles returns the quick tier Bible list (convenience function)
var QuickBibles = []string{
	"KJV",
	"Tyndale",
	"Geneva1599",
	"DRC",
	"VulgClementwordsof",
}

// IsQuickBible checks if a Bible ID is in the quick tier
func IsQuickBible(id string) bool {
	for _, b := range QuickBibles {
		if b == id {
			return true
		}
	}
	return false
}
