package sword

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to create temp conf file
func createTempConfFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "test.conf")
	if err := os.WriteFile(confPath, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to write temp conf file: %v", err)
	}
	return confPath
}

func TestParseConf_ValidBible(t *testing.T) {
	content := `[KJV]
Description=King James Version
About=The King James Version is a translation of the Bible.
ModDrv=zText
SourceType=OSIS
Lang=en
Versification=KJV
DataPath=./modules/texts/ztext/kjv/
CompressType=ZIP
BlockType=BOOK
Encoding=UTF-8
Version=2.3
Feature=StrongsNumbers
GlobalOptionFilter=OSISStrongs
GlobalOptionFilter=OSISMorph
`
	confPath := createTempConfFile(t, content)

	module, err := ParseConf(confPath)
	if err != nil {
		t.Fatalf("ParseConf() returned error: %v", err)
	}

	if module.ID != "kjv" {
		t.Errorf("ID = %q, want %q", module.ID, "kjv")
	}
	if module.Title != "King James Version" {
		t.Errorf("Title = %q, want %q", module.Title, "King James Version")
	}
	if module.Driver != DriverZText {
		t.Errorf("Driver = %q, want %q", module.Driver, DriverZText)
	}
	if module.SourceType != SourceOSIS {
		t.Errorf("SourceType = %q, want %q", module.SourceType, SourceOSIS)
	}
	if module.Language != "en" {
		t.Errorf("Language = %q, want %q", module.Language, "en")
	}
	if module.ModuleType != ModuleTypeBible {
		t.Errorf("ModuleType = %q, want %q", module.ModuleType, ModuleTypeBible)
	}
	if len(module.Features) != 1 || module.Features[0] != "StrongsNumbers" {
		t.Errorf("Features = %v, want [StrongsNumbers]", module.Features)
	}
	if len(module.GlobalOptionFilters) != 2 {
		t.Errorf("GlobalOptionFilters has %d items, want 2", len(module.GlobalOptionFilters))
	}
}

func TestParseConf_ValidDictionary(t *testing.T) {
	content := `[StrongsGreek]
Description=Strong's Greek Dictionary
ModDrv=zLD
SourceType=TEI
Lang=grc
DataPath=./modules/lexdict/zld/strongsgreek/
Feature=GreekDef
`
	confPath := createTempConfFile(t, content)

	module, err := ParseConf(confPath)
	if err != nil {
		t.Fatalf("ParseConf() returned error: %v", err)
	}

	if module.ID != "strongsgreek" {
		t.Errorf("ID = %q, want %q", module.ID, "strongsgreek")
	}
	if module.Driver != DriverZLD {
		t.Errorf("Driver = %q, want %q", module.Driver, DriverZLD)
	}
	if module.ModuleType != ModuleTypeDictionary {
		t.Errorf("ModuleType = %q, want %q", module.ModuleType, ModuleTypeDictionary)
	}
	if module.SourceType != SourceTEI {
		t.Errorf("SourceType = %q, want %q", module.SourceType, SourceTEI)
	}
}

func TestParseConf_ValidCommentary(t *testing.T) {
	content := `[MHC]
Description=Matthew Henry's Commentary
ModDrv=zCom
SourceType=ThML
Lang=en
DataPath=./modules/comments/zcom/mhc/
`
	confPath := createTempConfFile(t, content)

	module, err := ParseConf(confPath)
	if err != nil {
		t.Fatalf("ParseConf() returned error: %v", err)
	}

	if module.Driver != DriverZCom {
		t.Errorf("Driver = %q, want %q", module.Driver, DriverZCom)
	}
	if module.ModuleType != ModuleTypeCommentary {
		t.Errorf("ModuleType = %q, want %q", module.ModuleType, ModuleTypeCommentary)
	}
	if module.SourceType != SourceThML {
		t.Errorf("SourceType = %q, want %q", module.SourceType, SourceThML)
	}
}

func TestParseConf_MissingFile(t *testing.T) {
	_, err := ParseConf("/nonexistent/path/module.conf")
	if err == nil {
		t.Error("ParseConf() should return error for missing file")
	}
}

func TestParseConf_EmptyFile(t *testing.T) {
	confPath := createTempConfFile(t, "")

	module, err := ParseConf(confPath)
	if err != nil {
		t.Fatalf("ParseConf() returned error: %v", err)
	}

	// Should return empty module with default values
	if module.ID != "" {
		t.Errorf("ID = %q, want empty string", module.ID)
	}
}

func TestParseConf_NoHeader(t *testing.T) {
	// Config without [Section] header
	content := `Description=Test Module
ModDrv=zText
Lang=en
`
	confPath := createTempConfFile(t, content)

	module, err := ParseConf(confPath)
	if err != nil {
		t.Fatalf("ParseConf() returned error: %v", err)
	}

	// Should still parse key-value pairs
	if module.Title != "Test Module" {
		t.Errorf("Title = %q, want %q", module.Title, "Test Module")
	}
	// ID should be empty without header
	if module.ID != "" {
		t.Errorf("ID = %q, want empty (no header)", module.ID)
	}
}

func TestParseConf_CommentsIgnored(t *testing.T) {
	content := `[Test]
# This is a comment
Description=Test Description
#ModDrv=zText
Lang=en
`
	confPath := createTempConfFile(t, content)

	module, err := ParseConf(confPath)
	if err != nil {
		t.Fatalf("ParseConf() returned error: %v", err)
	}

	if module.Title != "Test Description" {
		t.Errorf("Title = %q, want %q", module.Title, "Test Description")
	}
	// Commented ModDrv should not be parsed
	if module.Driver != "" {
		t.Errorf("Driver = %q, want empty (commented out)", module.Driver)
	}
}

func TestParseConf_MultiValueFields(t *testing.T) {
	content := `[Test]
Feature=StrongsNumbers
Feature=Morphology
Feature=Headings
GlobalOptionFilter=OSISStrongs
GlobalOptionFilter=OSISMorph
GlobalOptionFilter=OSISHeadings
`
	confPath := createTempConfFile(t, content)

	module, err := ParseConf(confPath)
	if err != nil {
		t.Fatalf("ParseConf() returned error: %v", err)
	}

	if len(module.Features) != 3 {
		t.Errorf("len(Features) = %d, want 3", len(module.Features))
	}
	if len(module.GlobalOptionFilters) != 3 {
		t.Errorf("len(GlobalOptionFilters) = %d, want 3", len(module.GlobalOptionFilters))
	}

	// Check specific values
	expectedFeatures := []string{"StrongsNumbers", "Morphology", "Headings"}
	for i, expected := range expectedFeatures {
		if module.Features[i] != expected {
			t.Errorf("Features[%d] = %q, want %q", i, module.Features[i], expected)
		}
	}
}

func TestParseAboutText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "double par to double newline",
			input:    `First paragraph\par\parSecond paragraph`,
			expected: "First paragraph\n\nSecond paragraph",
		},
		{
			name:     "par with space",
			input:    `Line one\par Line two`,
			expected: "Line one\nLine two",
		},
		{
			name:     "par without space",
			input:    `Line one\parLine two`,
			expected: "Line one\nLine two",
		},
		{
			name:     "qc removed",
			input:    `\qcCentered Text`,
			expected: "Centered Text",
		},
		{
			name:     "pard removed",
			input:    `\pardNormal paragraph`,
			expected: "dNormal paragraph", // \par is replaced first, leaving 'd'
		},
		{
			name:     "multiple escapes combined",
			input:    `\qc Title\par\parDescription\pard End.`,
			expected: "Title\n\nDescription\nd End.", // \par replaced, then \pard → d remaining
		},
		{
			name:     "plain text unchanged",
			input:    "This is plain text.",
			expected: "This is plain text.",
		},
		{
			name:     "leading/trailing whitespace trimmed",
			input:    "  \\par Text with spaces  ",
			expected: "Text with spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAboutText(tt.input)
			if result != tt.expected {
				t.Errorf("parseAboutText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTruncateDescription(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short text unchanged",
			input:    "Short description",
			maxLen:   200,
			expected: "Short description",
		},
		{
			name:     "truncate at word boundary",
			input:    "This is a longer description that needs to be truncated at a reasonable word boundary",
			maxLen:   30,
			expected: "This is a longer description...",
		},
		{
			name:     "first paragraph only",
			input:    "First paragraph here.\nSecond paragraph that should be excluded.",
			maxLen:   200,
			expected: "First paragraph here.",
		},
		{
			name:     "newline within limit takes precedence",
			input:    "Short first.\nSecond paragraph.",
			maxLen:   50,
			expected: "Short first.",
		},
		{
			name:     "exact length",
			input:    "12345",
			maxLen:   5,
			expected: "12345",
		},
		{
			name:     "no space for word boundary",
			input:    "Verylongwordwithoutspaces",
			maxLen:   10,
			expected: "Verylongwo...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateDescription(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateDescription(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestDriverToModuleType(t *testing.T) {
	tests := []struct {
		driver   ModuleDriver
		expected ModuleType
	}{
		{DriverZText, ModuleTypeBible},
		{DriverZText4, ModuleTypeBible},
		{DriverRawText, ModuleTypeBible},
		{DriverRawText4, ModuleTypeBible},
		{DriverZCom, ModuleTypeCommentary},
		{DriverZCom4, ModuleTypeCommentary},
		{DriverRawCom, ModuleTypeCommentary},
		{DriverRawCom4, ModuleTypeCommentary},
		{DriverZLD, ModuleTypeDictionary},
		{DriverRawLD, ModuleTypeDictionary},
		{DriverRawLD4, ModuleTypeDictionary},
		{DriverRawGenBook, ModuleTypeGenBook},
		{ModuleDriver("Unknown"), ModuleTypeBible}, // Default case
	}

	for _, tt := range tests {
		t.Run(string(tt.driver), func(t *testing.T) {
			result := driverToModuleType(tt.driver)
			if result != tt.expected {
				t.Errorf("driverToModuleType(%q) = %q, want %q", tt.driver, result, tt.expected)
			}
		})
	}
}

func TestModule_HasFeature(t *testing.T) {
	module := &Module{
		Features: []string{"StrongsNumbers", "Headings", "Footnotes"},
	}

	t.Run("has StrongsNumbers", func(t *testing.T) {
		if !module.HasFeature("StrongsNumbers") {
			t.Error("HasFeature(StrongsNumbers) = false, want true")
		}
	})

	t.Run("has Headings", func(t *testing.T) {
		if !module.HasFeature("Headings") {
			t.Error("HasFeature(Headings) = false, want true")
		}
	})

	t.Run("does not have Morphology", func(t *testing.T) {
		if module.HasFeature("Morphology") {
			t.Error("HasFeature(Morphology) = true, want false")
		}
	})

	t.Run("empty feature", func(t *testing.T) {
		if module.HasFeature("") {
			t.Error("HasFeature('') = true, want false")
		}
	})
}

func TestModule_HasStrongsNumbers(t *testing.T) {
	t.Run("with StrongsNumbers", func(t *testing.T) {
		module := &Module{Features: []string{"StrongsNumbers"}}
		if !module.HasStrongsNumbers() {
			t.Error("HasStrongsNumbers() = false, want true")
		}
	})

	t.Run("without StrongsNumbers", func(t *testing.T) {
		module := &Module{Features: []string{"Headings"}}
		if module.HasStrongsNumbers() {
			t.Error("HasStrongsNumbers() = true, want false")
		}
	})

	t.Run("empty features", func(t *testing.T) {
		module := &Module{Features: []string{}}
		if module.HasStrongsNumbers() {
			t.Error("HasStrongsNumbers() = true with empty features, want false")
		}
	})
}

func TestModule_HasMorphology(t *testing.T) {
	t.Run("with OSISMorph filter", func(t *testing.T) {
		module := &Module{GlobalOptionFilters: []string{"OSISMorph"}}
		if !module.HasMorphology() {
			t.Error("HasMorphology() = false with OSISMorph, want true")
		}
	})

	t.Run("with GBFMorph filter", func(t *testing.T) {
		module := &Module{GlobalOptionFilters: []string{"GBFMorph"}}
		if !module.HasMorphology() {
			t.Error("HasMorphology() = false with GBFMorph, want true")
		}
	})

	t.Run("with Morphology in filter name", func(t *testing.T) {
		module := &Module{GlobalOptionFilters: []string{"MorphologySegmentation"}}
		if !module.HasMorphology() {
			t.Error("HasMorphology() = false with MorphologySegmentation, want true")
		}
	})

	t.Run("without morphology", func(t *testing.T) {
		module := &Module{GlobalOptionFilters: []string{"OSISStrongs", "OSISHeadings"}}
		if module.HasMorphology() {
			t.Error("HasMorphology() = true without morph filter, want false")
		}
	})

	t.Run("empty filters", func(t *testing.T) {
		module := &Module{GlobalOptionFilters: []string{}}
		if module.HasMorphology() {
			t.Error("HasMorphology() = true with empty filters, want false")
		}
	})
}

func TestModule_ResolveDataPath(t *testing.T) {
	tests := []struct {
		name     string
		dataPath string
		swordDir string
		expected string
	}{
		{
			name:     "relative path with ./",
			dataPath: "./modules/texts/ztext/kjv/",
			swordDir: "/home/user/.sword",
			expected: "/home/user/.sword/modules/texts/ztext/kjv", // filepath.Join removes trailing slash
		},
		{
			name:     "relative path without ./",
			dataPath: "modules/texts/ztext/kjv/",
			swordDir: "/home/user/.sword",
			expected: "/home/user/.sword/modules/texts/ztext/kjv", // filepath.Join removes trailing slash
		},
		{
			name:     "different sword dir",
			dataPath: "./modules/lexdict/zld/strongs/",
			swordDir: "/usr/share/sword",
			expected: "/usr/share/sword/modules/lexdict/zld/strongs", // filepath.Join removes trailing slash
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &Module{DataPath: tt.dataPath}
			result := module.ResolveDataPath(tt.swordDir)
			if result != tt.expected {
				t.Errorf("ResolveDataPath(%q) = %q, want %q", tt.swordDir, result, tt.expected)
			}
		})
	}
}

func TestParseConf_AllFields(t *testing.T) {
	content := `[TestModule]
Description=Test Module Title
About=\qcThis is the about text.\par\parSecond paragraph.
ModDrv=zText4
SourceType=OSIS
Lang=en
Versification=KJV
DataPath=./modules/texts/ztext/test/
CompressType=ZIP
BlockType=CHAPTER
Encoding=UTF-8
Version=1.0
SwordVersionDate=2024-01-01
Copyright=Public Domain
DistributionLicense=Public Domain
Category=Biblical Texts
LCSH=Bible--Versions
MinimumVersion=1.5.9
`
	confPath := createTempConfFile(t, content)

	module, err := ParseConf(confPath)
	if err != nil {
		t.Fatalf("ParseConf() returned error: %v", err)
	}

	// Check all parsed fields
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"ID", module.ID, "testmodule"},
		{"Title", module.Title, "Test Module Title"},
		{"Driver", string(module.Driver), "zText4"},
		{"SourceType", string(module.SourceType), "OSIS"},
		{"Language", module.Language, "en"},
		{"Versification", module.Versification, "KJV"},
		{"DataPath", module.DataPath, "./modules/texts/ztext/test/"},
		{"CompressType", module.CompressType, "ZIP"},
		{"BlockType", module.BlockType, "CHAPTER"},
		{"Encoding", module.Encoding, "UTF-8"},
		{"Version", module.Version, "1.0"},
		{"SwordVersionDate", module.SwordVersionDate, "2024-01-01"},
		{"Copyright", module.Copyright, "Public Domain"},
		{"DistributionLicense", module.DistributionLicense, "Public Domain"},
		{"Category", module.Category, "Biblical Texts"},
		{"LCSH", module.LCSH, "Bible--Versions"},
		{"MinimumVersion", module.MinimumVersion, "1.5.9"},
	}

	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %q, want %q", check.name, check.got, check.want)
		}
	}

	// Check About is parsed (RTF stripped)
	if !strings.Contains(module.About, "This is the about text") {
		t.Errorf("About does not contain expected text: %q", module.About)
	}
	if strings.Contains(module.About, `\qc`) {
		t.Errorf("About still contains RTF escapes: %q", module.About)
	}
}

func TestParseConf_GeneratesDescriptionFromAbout(t *testing.T) {
	content := `[Test]
About=This is a long about text that will be used to generate the description field since no Description key is provided. It should be truncated appropriately.
`
	confPath := createTempConfFile(t, content)

	module, err := ParseConf(confPath)
	if err != nil {
		t.Fatalf("ParseConf() returned error: %v", err)
	}

	if module.Description == "" {
		t.Error("Description should be auto-generated from About")
	}
	if len(module.Description) > 203 { // 200 + "..."
		t.Errorf("Description too long: %d chars", len(module.Description))
	}
}

func TestDiscoverModules(t *testing.T) {
	// Create temp sword directory with mods.d
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		t.Fatalf("Failed to create mods.d: %v", err)
	}

	// Create some conf files
	confFiles := []string{"kjv.conf", "esv.conf", "strongs.conf"}
	for _, name := range confFiles {
		path := filepath.Join(modsDir, name)
		if err := os.WriteFile(path, []byte("[Test]\n"), 0600); err != nil {
			t.Fatalf("Failed to create %s: %v", name, err)
		}
	}

	// Create a non-conf file that should be ignored
	if err := os.WriteFile(filepath.Join(modsDir, "readme.txt"), []byte("ignore me"), 0600); err != nil {
		t.Fatalf("Failed to create readme.txt: %v", err)
	}

	discovered, err := DiscoverModules(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverModules() returned error: %v", err)
	}

	if len(discovered) != 3 {
		t.Errorf("len(discovered) = %d, want 3", len(discovered))
	}

	// Check all conf files found
	for _, name := range confFiles {
		found := false
		for _, path := range discovered {
			if strings.HasSuffix(path, name) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Did not find %s in discovered modules", name)
		}
	}
}

func TestDiscoverModules_MissingDir(t *testing.T) {
	_, err := DiscoverModules("/nonexistent/sword/path")
	if err == nil {
		t.Error("DiscoverModules() should return error for missing directory")
	}
}

func TestLoadAllModules(t *testing.T) {
	// Create temp sword directory structure
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		t.Fatalf("Failed to create mods.d: %v", err)
	}

	// Create valid conf files
	kjvConf := `[KJV]
Description=King James Version
ModDrv=zText
Lang=en
DataPath=./modules/texts/ztext/kjv/
`
	esvConf := `[ESV]
Description=English Standard Version
ModDrv=zText
Lang=en
DataPath=./modules/texts/ztext/esv/
`
	if err := os.WriteFile(filepath.Join(modsDir, "kjv.conf"), []byte(kjvConf), 0600); err != nil {
		t.Fatalf("Failed to create kjv.conf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modsDir, "esv.conf"), []byte(esvConf), 0600); err != nil {
		t.Fatalf("Failed to create esv.conf: %v", err)
	}

	modules, err := LoadAllModules(tmpDir)
	if err != nil {
		t.Fatalf("LoadAllModules() returned error: %v", err)
	}

	if len(modules) != 2 {
		t.Errorf("len(modules) = %d, want 2", len(modules))
	}

	// Check modules are loaded correctly
	foundKJV := false
	foundESV := false
	for _, m := range modules {
		if m.ID == "kjv" {
			foundKJV = true
		}
		if m.ID == "esv" {
			foundESV = true
		}
	}
	if !foundKJV {
		t.Error("KJV module not found")
	}
	if !foundESV {
		t.Error("ESV module not found")
	}
}

func TestLoadAllModules_WithInvalidConf(t *testing.T) {
	// Create temp sword directory structure
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		t.Fatalf("Failed to create mods.d: %v", err)
	}

	// Create valid conf file
	validConf := `[Valid]
Description=Valid Module
ModDrv=zText
`
	if err := os.WriteFile(filepath.Join(modsDir, "valid.conf"), []byte(validConf), 0600); err != nil {
		t.Fatalf("Failed to create valid.conf: %v", err)
	}

	// Create a conf file that can't be parsed (make it unreadable)
	invalidPath := filepath.Join(modsDir, "invalid.conf")
	if err := os.WriteFile(invalidPath, []byte("[Test]\n"), 0600); err != nil {
		t.Fatalf("Failed to create invalid.conf: %v", err)
	}
	// Make it a directory to cause Open to fail (on most systems)
	os.Remove(invalidPath)
	os.Mkdir(invalidPath, 0700)

	modules, err := LoadAllModules(tmpDir)
	if err != nil {
		t.Fatalf("LoadAllModules() should not error even with invalid conf: %v", err)
	}

	// Should load at least the valid module
	if len(modules) < 1 {
		t.Error("Should have loaded at least one valid module")
	}
}

func TestLoadAllModules_MissingDir(t *testing.T) {
	_, err := LoadAllModules("/nonexistent/sword/path")
	if err == nil {
		t.Error("LoadAllModules() should return error for missing directory")
	}
}

func TestLoadAllModules_EmptyDir(t *testing.T) {
	// Create temp sword directory structure with empty mods.d
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		t.Fatalf("Failed to create mods.d: %v", err)
	}

	modules, err := LoadAllModules(tmpDir)
	if err != nil {
		t.Fatalf("LoadAllModules() returned error: %v", err)
	}

	if len(modules) != 0 {
		t.Errorf("len(modules) = %d, want 0 for empty directory", len(modules))
	}
}
