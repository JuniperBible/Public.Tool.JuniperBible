package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	home, _ := os.UserHomeDir()

	t.Run("SwordDir", func(t *testing.T) {
		expected := filepath.Join(home, ".sword")
		if cfg.SwordDir != expected {
			t.Errorf("SwordDir = %q, want %q", cfg.SwordDir, expected)
		}
	})

	t.Run("ESwordDir", func(t *testing.T) {
		expected := filepath.Join(home, "e-sword")
		if cfg.ESwordDir != expected {
			t.Errorf("ESwordDir = %q, want %q", cfg.ESwordDir, expected)
		}
	})

	t.Run("OutputDir", func(t *testing.T) {
		if cfg.OutputDir != "sword_data" {
			t.Errorf("OutputDir = %q, want %q", cfg.OutputDir, "sword_data")
		}
	})

	t.Run("ContentDir", func(t *testing.T) {
		if cfg.ContentDir != "content/religion" {
			t.Errorf("ContentDir = %q, want %q", cfg.ContentDir, "content/religion")
		}
	})

	t.Run("Granularity", func(t *testing.T) {
		if cfg.Granularity != GranularityChapter {
			t.Errorf("Granularity = %q, want %q", cfg.Granularity, GranularityChapter)
		}
	})

	t.Run("Modules", func(t *testing.T) {
		if cfg.Modules == nil {
			t.Error("Modules is nil, want empty slice")
		}
		if len(cfg.Modules) != 0 {
			t.Errorf("len(Modules) = %d, want 0", len(cfg.Modules))
		}
	})

	t.Run("FiltersTypes", func(t *testing.T) {
		expected := []string{"Bible", "Commentary", "Dictionary"}
		if len(cfg.Filters.Types) != len(expected) {
			t.Errorf("len(Filters.Types) = %d, want %d", len(cfg.Filters.Types), len(expected))
			return
		}
		for i, v := range expected {
			if cfg.Filters.Types[i] != v {
				t.Errorf("Filters.Types[%d] = %q, want %q", i, cfg.Filters.Types[i], v)
			}
		}
	})

	t.Run("OutputPreserveStrongs", func(t *testing.T) {
		if !cfg.Output.PreserveStrongs {
			t.Error("Output.PreserveStrongs = false, want true")
		}
	})

	t.Run("OutputPreserveMorphology", func(t *testing.T) {
		if !cfg.Output.PreserveMorphology {
			t.Error("Output.PreserveMorphology = false, want true")
		}
	})

	t.Run("OutputGenerateSearchIndex", func(t *testing.T) {
		if cfg.Output.GenerateSearchIndex {
			t.Error("Output.GenerateSearchIndex = true, want false")
		}
	})
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Cannot get home directory: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tilde only",
			input:    "~",
			expected: home,
		},
		{
			name:     "tilde with path",
			input:    "~/.sword",
			expected: filepath.Join(home, ".sword"),
		},
		{
			name:     "tilde with nested path",
			input:    "~/Documents/e-sword/modules",
			expected: filepath.Join(home, "Documents/e-sword/modules"),
		},
		{
			name:     "absolute path unchanged",
			input:    "/usr/share/sword",
			expected: "/usr/share/sword",
		},
		{
			name:     "relative path unchanged",
			input:    "./data/sword",
			expected: "./data/sword",
		},
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "tilde in middle unchanged",
			input:    "/home/~user/data",
			expected: "/home/~user/data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if result != tt.expected {
				t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoad_NonExistentFile_ReturnsDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load() returned error for non-existent file: %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Should return defaults
	defaults := DefaultConfig()
	if cfg.Granularity != defaults.Granularity {
		t.Errorf("Granularity = %q, want %q", cfg.Granularity, defaults.Granularity)
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
swordDir: /custom/sword
eswordDir: /custom/esword
outputDir: custom/output
contentDir: custom/content
granularity: verse
modules:
  - KJV
  - ESV
filters:
  languages:
    - en
    - de
  types:
    - Bible
output:
  preserveStrongs: false
  preserveMorphology: true
  generateSearchIndex: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.SwordDir != "/custom/sword" {
		t.Errorf("SwordDir = %q, want %q", cfg.SwordDir, "/custom/sword")
	}
	if cfg.ESwordDir != "/custom/esword" {
		t.Errorf("ESwordDir = %q, want %q", cfg.ESwordDir, "/custom/esword")
	}
	if cfg.OutputDir != "custom/output" {
		t.Errorf("OutputDir = %q, want %q", cfg.OutputDir, "custom/output")
	}
	if cfg.Granularity != GranularityVerse {
		t.Errorf("Granularity = %q, want %q", cfg.Granularity, GranularityVerse)
	}
	if len(cfg.Modules) != 2 {
		t.Errorf("len(Modules) = %d, want 2", len(cfg.Modules))
	}
	if !cfg.Output.GenerateSearchIndex {
		t.Error("Output.GenerateSearchIndex = false, want true")
	}
	if cfg.Output.PreserveStrongs {
		t.Error("Output.PreserveStrongs = true, want false")
	}
}

func TestLoad_MalformedYAML_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.yaml")

	// Invalid YAML with bad indentation
	badYAML := `
swordDir: /path
  this is not: valid yaml
    broken: indentation
`
	if err := os.WriteFile(configPath, []byte(badYAML), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() should return error for malformed YAML")
	}
}

func TestLoad_PartialConfig_MergesWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "partial.yaml")

	// Only specify some fields
	partialYAML := `
granularity: book
modules:
  - KJV
`
	if err := os.WriteFile(configPath, []byte(partialYAML), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Custom values should be set
	if cfg.Granularity != GranularityBook {
		t.Errorf("Granularity = %q, want %q", cfg.Granularity, GranularityBook)
	}
	if len(cfg.Modules) != 1 || cfg.Modules[0] != "KJV" {
		t.Errorf("Modules = %v, want [KJV]", cfg.Modules)
	}

	// Defaults should still be present
	home, _ := os.UserHomeDir()
	expectedSword := filepath.Join(home, ".sword")
	if cfg.SwordDir != expectedSword {
		t.Errorf("SwordDir = %q, want %q (default)", cfg.SwordDir, expectedSword)
	}
	if !cfg.Output.PreserveStrongs {
		t.Error("Output.PreserveStrongs should default to true")
	}
}

func TestLoad_TildeExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tilde.yaml")

	yamlContent := `
swordDir: ~/.sword
eswordDir: ~/Documents/e-sword
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	home, _ := os.UserHomeDir()

	expectedSword := filepath.Join(home, ".sword")
	if cfg.SwordDir != expectedSword {
		t.Errorf("SwordDir = %q, want %q (expanded)", cfg.SwordDir, expectedSword)
	}

	expectedESword := filepath.Join(home, "Documents/e-sword")
	if cfg.ESwordDir != expectedESword {
		t.Errorf("ESwordDir = %q, want %q (expanded)", cfg.ESwordDir, expectedESword)
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty.yaml")

	if err := os.WriteFile(configPath, []byte(""), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error for empty file: %v", err)
	}

	// Should return defaults
	if cfg.Granularity != GranularityChapter {
		t.Errorf("Granularity = %q, want %q (default)", cfg.Granularity, GranularityChapter)
	}
}

func TestLoad_DirectoryAsPath_ReturnsError(t *testing.T) {
	// Trying to read a directory as a file should return an error
	// This tests the non-IsNotExist error path
	tmpDir := t.TempDir()

	_, err := Load(tmpDir) // Pass directory, not file
	if err == nil {
		t.Error("Load() should return error when path is a directory")
	}
}

func TestGranularityConstants(t *testing.T) {
	tests := []struct {
		granularity Granularity
		expected    string
	}{
		{GranularityBook, "book"},
		{GranularityChapter, "chapter"},
		{GranularityVerse, "verse"},
	}

	for _, tt := range tests {
		t.Run(string(tt.granularity), func(t *testing.T) {
			if string(tt.granularity) != tt.expected {
				t.Errorf("Granularity constant %v = %q, want %q", tt.granularity, string(tt.granularity), tt.expected)
			}
		})
	}
}
