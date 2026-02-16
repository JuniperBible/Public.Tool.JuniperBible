package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseConfFile tests parsing a SWORD .conf file.
func TestParseConfFile(t *testing.T) {
	// Create a temporary conf file
	tmpDir, err := os.MkdirTemp("", "sword-conf-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	confContent := `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
Encoding=UTF-8
Lang=en
Description=King James Version (1769)
Version=1.0
About=The King James Version of the Holy Bible
Copyright=Public Domain
DistributionLicense=Public Domain
Category=Biblical Texts
Versification=KJV
CompressType=ZIP
BlockType=BOOK
SourceType=OSIS
`

	confPath := filepath.Join(tmpDir, "kjv.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		t.Fatalf("failed to write conf file: %v", err)
	}

	conf, err := ParseConfFile(confPath)
	if err != nil {
		t.Fatalf("ParseConfFile failed: %v", err)
	}

	// Verify parsed values
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"ModuleName", conf.ModuleName, "KJV"},
		{"Description", conf.Description, "King James Version (1769)"},
		{"ModDrv", conf.ModDrv, "zText"},
		{"Encoding", conf.Encoding, "UTF-8"},
		{"Lang", conf.Lang, "en"},
		{"Version", conf.Version, "1.0"},
		{"About", conf.About, "The King James Version of the Holy Bible"},
		{"Copyright", conf.Copyright, "Public Domain"},
		{"License", conf.License, "Public Domain"},
		{"Category", conf.Category, "Biblical Texts"},
		{"Versification", conf.Versification, "KJV"},
		{"CompressType", conf.CompressType, "ZIP"},
		{"BlockType", conf.BlockType, "BOOK"},
		{"SourceType", conf.SourceType, "OSIS"},
	}

	for _, tt := range tests {
		if tt.got != tt.expected {
			t.Errorf("%s: expected %q, got %q", tt.name, tt.expected, tt.got)
		}
	}
}

// TestParseConfFileMultiline tests parsing multiline values.
func TestParseConfFileMultiline(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sword-conf-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Multiline with continuation
	confContent := `[TestMod]
Description=A test module
About=This is a long description \
 that spans multiple lines \
 for testing purposes.
`

	confPath := filepath.Join(tmpDir, "test.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		t.Fatalf("failed to write conf file: %v", err)
	}

	conf, err := ParseConfFile(confPath)
	if err != nil {
		t.Fatalf("ParseConfFile failed: %v", err)
	}

	if conf.ModuleName != "TestMod" {
		t.Errorf("ModuleName: expected 'TestMod', got %q", conf.ModuleName)
	}
}

// TestModuleType tests module type detection.
func TestModuleType(t *testing.T) {
	tests := []struct {
		modDrv   string
		expected string
	}{
		{"zText", "Bible"},
		{"zText4", "Bible"},
		{"RawText", "Bible"},
		{"zCom", "Commentary"},
		{"zCom4", "Commentary"},
		{"RawCom", "Commentary"},
		{"zLD", "Dictionary"},
		{"RawLD", "Dictionary"},
		{"RawGenBook", "GenBook"},
		{"Unknown", "Unknown"},
	}

	for _, tt := range tests {
		conf := &ConfFile{ModDrv: tt.modDrv}
		got := conf.ModuleType()
		if got != tt.expected {
			t.Errorf("ModuleType(%q): expected %q, got %q", tt.modDrv, tt.expected, got)
		}
	}
}

// TestIsCompressed tests compression detection.
func TestIsCompressed(t *testing.T) {
	tests := []struct {
		modDrv   string
		expected bool
	}{
		{"zText", true},
		{"zText4", true},
		{"zCom", true},
		{"zLD", true},
		{"RawText", false},
		{"RawCom", false},
		{"RawLD", false},
		{"RawGenBook", false},
	}

	for _, tt := range tests {
		conf := &ConfFile{ModDrv: tt.modDrv}
		got := conf.IsCompressed()
		if got != tt.expected {
			t.Errorf("IsCompressed(%q): expected %v, got %v", tt.modDrv, tt.expected, got)
		}
	}
}

// TestIsEncrypted tests encryption detection.
func TestIsEncrypted(t *testing.T) {
	conf := &ConfFile{}
	if conf.IsEncrypted() {
		t.Error("expected IsEncrypted() to return false for empty CipherKey")
	}

	conf.CipherKey = "secret"
	if !conf.IsEncrypted() {
		t.Error("expected IsEncrypted() to return true for non-empty CipherKey")
	}
}

// TestFindConfFiles tests finding .conf files in a directory.
func TestFindConfFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sword-conf-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "kjv.conf"), []byte("[KJV]"), 0600)
	os.WriteFile(filepath.Join(tmpDir, "esv.conf"), []byte("[ESV]"), 0600)
	os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("readme"), 0600)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	confFiles, err := FindConfFiles(tmpDir)
	if err != nil {
		t.Fatalf("FindConfFiles failed: %v", err)
	}

	if len(confFiles) != 2 {
		t.Errorf("expected 2 conf files, got %d", len(confFiles))
	}
}

// TestLoadModulesFromPath tests loading modules from a SWORD path.
func TestLoadModulesFromPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mods.d directory with conf files
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.Mkdir(modsDir, 0755)

	confContent1 := `[KJV]
Description=King James Version
ModDrv=zText
Lang=en
`
	confContent2 := `[ESV]
Description=English Standard Version
ModDrv=zText
Lang=en
`

	os.WriteFile(filepath.Join(modsDir, "kjv.conf"), []byte(confContent1), 0600)
	os.WriteFile(filepath.Join(modsDir, "esv.conf"), []byte(confContent2), 0600)

	modules, err := LoadModulesFromPath(tmpDir)
	if err != nil {
		t.Fatalf("LoadModulesFromPath failed: %v", err)
	}

	if len(modules) != 2 {
		t.Errorf("expected 2 modules, got %d", len(modules))
	}
}
