package swordpure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfFile(t *testing.T) {
	// Create a temp conf file
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	confContent := `[TestModule]
DataPath=./modules/texts/ztext/testmod/
ModDrv=zText
Encoding=UTF-8
Lang=en
Version=1.0
Description=Test Module
About=This is a test module \
that spans multiple lines
SourceType=OSIS
BlockType=BOOK
CompressType=ZIP
Versification=KJV

# This is a comment
Category=Biblical Texts
Copyright=Public Domain
DistributionLicense=Public Domain
`

	confPath := filepath.Join(tmpDir, "testmod.conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		t.Fatalf("failed to write conf file: %v", err)
	}

	conf, err := ParseConfFile(confPath)
	if err != nil {
		t.Fatalf("ParseConfFile failed: %v", err)
	}

	// Verify parsed values
	if conf.ModuleName != "TestModule" {
		t.Errorf("ModuleName = %q, want %q", conf.ModuleName, "TestModule")
	}
	if conf.ModDrv != "zText" {
		t.Errorf("ModDrv = %q, want %q", conf.ModDrv, "zText")
	}
	if conf.Encoding != "UTF-8" {
		t.Errorf("Encoding = %q, want %q", conf.Encoding, "UTF-8")
	}
	if conf.Lang != "en" {
		t.Errorf("Lang = %q, want %q", conf.Lang, "en")
	}
	if conf.Description != "Test Module" {
		t.Errorf("Description = %q, want %q", conf.Description, "Test Module")
	}
	if conf.SourceType != "OSIS" {
		t.Errorf("SourceType = %q, want %q", conf.SourceType, "OSIS")
	}
	if conf.CompressType != "ZIP" {
		t.Errorf("CompressType = %q, want %q", conf.CompressType, "ZIP")
	}
	if conf.Versification != "KJV" {
		t.Errorf("Versification = %q, want %q", conf.Versification, "KJV")
	}
	if conf.License != "Public Domain" {
		t.Errorf("License = %q, want %q", conf.License, "Public Domain")
	}

	// Verify multiline value was parsed correctly (backslash continuation)
	// The parser removes the backslash and joins with space
	if conf.About == "" {
		t.Error("About should not be empty")
	}
}

func TestParseConfFileNonExistent(t *testing.T) {
	_, err := ParseConfFile("/nonexistent/path/test.conf")
	if err == nil {
		t.Error("ParseConfFile should fail for non-existent file")
	}
}

func TestConfFileModuleType(t *testing.T) {
	tests := []struct {
		modDrv   string
		wantType string
	}{
		{"ztext", "Bible"},
		{"zText", "Bible"},
		{"ztext4", "Bible"},
		{"rawtext", "Bible"},
		{"rawtext4", "Bible"},
		{"zcom", "Commentary"},
		{"zcom4", "Commentary"},
		{"rawcom", "Commentary"},
		{"rawcom4", "Commentary"},
		{"zld", "Dictionary"},
		{"rawld", "Dictionary"},
		{"rawld4", "Dictionary"},
		{"rawgenbook", "GenBook"},
		{"unknown", "Unknown"},
		{"", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.modDrv, func(t *testing.T) {
			conf := &ConfFile{ModDrv: tt.modDrv}
			if got := conf.ModuleType(); got != tt.wantType {
				t.Errorf("ModuleType() = %q, want %q", got, tt.wantType)
			}
		})
	}
}

func TestConfFileIsCompressed(t *testing.T) {
	tests := []struct {
		modDrv string
		want   bool
	}{
		{"ztext", true},
		{"ztext4", true},
		{"zcom", true},
		{"zcom4", true},
		{"zld", true},
		{"rawtext", false},
		{"rawcom", false},
		{"rawld", false},
		{"rawgenbook", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.modDrv, func(t *testing.T) {
			conf := &ConfFile{ModDrv: tt.modDrv}
			if got := conf.IsCompressed(); got != tt.want {
				t.Errorf("IsCompressed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfFileIsEncrypted(t *testing.T) {
	tests := []struct {
		name      string
		cipherKey string
		want      bool
	}{
		{"no cipher key", "", false},
		{"has cipher key", "secret123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := &ConfFile{CipherKey: tt.cipherKey}
			if got := conf.IsEncrypted(); got != tt.want {
				t.Errorf("IsEncrypted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindConfFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some .conf files
	confFiles := []string{"kjv.conf", "ESV.CONF", "niv.conf", "readme.txt"}
	for _, name := range confFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("[Test]\n"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	// Create a subdirectory (should be ignored)
	subDir := filepath.Join(tmpDir, "subdir.conf")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	found, err := FindConfFiles(tmpDir)
	if err != nil {
		t.Fatalf("FindConfFiles failed: %v", err)
	}

	// Should find 3 .conf files (not .txt or directory)
	if len(found) != 3 {
		t.Errorf("FindConfFiles found %d files, want 3", len(found))
	}
}

func TestFindConfFilesNonExistent(t *testing.T) {
	_, err := FindConfFiles("/nonexistent/path")
	if err == nil {
		t.Error("FindConfFiles should fail for non-existent directory")
	}
}

func TestLoadModulesFromPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swordpure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mods.d directory
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.Mkdir(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	// Create valid conf file
	validConf := `[ValidModule]
ModDrv=zText
Lang=en
`
	if err := os.WriteFile(filepath.Join(modsDir, "valid.conf"), []byte(validConf), 0644); err != nil {
		t.Fatalf("failed to write conf file: %v", err)
	}

	// Create invalid conf file (empty - should be skipped with warning)
	if err := os.WriteFile(filepath.Join(modsDir, "invalid.conf"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write invalid conf file: %v", err)
	}

	modules, err := LoadModulesFromPath(tmpDir)
	if err != nil {
		t.Fatalf("LoadModulesFromPath failed: %v", err)
	}

	// Should have at least the valid module
	if len(modules) < 1 {
		t.Errorf("LoadModulesFromPath returned %d modules, want at least 1", len(modules))
	}
}

func TestSetPropertyAllFields(t *testing.T) {
	conf := &ConfFile{Properties: make(map[string]string)}

	// Test all known property mappings
	properties := map[string]string{
		"Description":         "Test Description",
		"DataPath":            "./modules/test/",
		"ModDrv":              "zText",
		"Encoding":            "UTF-8",
		"Lang":                "en",
		"Version":             "1.0",
		"About":               "About text",
		"Copyright":           "(C) 2024",
		"DistributionLicense": "Public Domain",
		"Category":            "Biblical Texts",
		"LCSH":                "Bible",
		"SourceType":          "OSIS",
		"BlockType":           "BOOK",
		"CompressType":        "ZIP",
		"CipherKey":           "secret",
		"Versification":       "KJV",
	}

	for key, value := range properties {
		conf.setProperty(key, value)
	}

	// Verify struct fields
	if conf.Description != "Test Description" {
		t.Errorf("Description not set correctly")
	}
	if conf.DataPath != "./modules/test/" {
		t.Errorf("DataPath not set correctly")
	}
	if conf.ModDrv != "zText" {
		t.Errorf("ModDrv not set correctly")
	}
	if conf.Encoding != "UTF-8" {
		t.Errorf("Encoding not set correctly")
	}
	if conf.Lang != "en" {
		t.Errorf("Lang not set correctly")
	}
	if conf.Version != "1.0" {
		t.Errorf("Version not set correctly")
	}
	if conf.About != "About text" {
		t.Errorf("About not set correctly")
	}
	if conf.Copyright != "(C) 2024" {
		t.Errorf("Copyright not set correctly")
	}
	if conf.License != "Public Domain" {
		t.Errorf("License not set correctly")
	}
	if conf.Category != "Biblical Texts" {
		t.Errorf("Category not set correctly")
	}
	if conf.LCSH != "Bible" {
		t.Errorf("LCSH not set correctly")
	}
	if conf.SourceType != "OSIS" {
		t.Errorf("SourceType not set correctly")
	}
	if conf.BlockType != "BOOK" {
		t.Errorf("BlockType not set correctly")
	}
	if conf.CompressType != "ZIP" {
		t.Errorf("CompressType not set correctly")
	}
	if conf.CipherKey != "secret" {
		t.Errorf("CipherKey not set correctly")
	}
	if conf.Versification != "KJV" {
		t.Errorf("Versification not set correctly")
	}
}
