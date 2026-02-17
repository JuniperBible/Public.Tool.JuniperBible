package repository

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createTestModsArchive creates a mods.d.tar.gz for testing
func createTestModsArchive(t *testing.T, confFiles map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for name, content := range confFiles {
		hdr := &tar.Header{
			Name: "mods.d/" + name,
			Mode: 0600,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("Failed to write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write tar content: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("Failed to close tar: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("Failed to close gzip: %v", err)
	}

	return buf.Bytes()
}

// Sample conf file content
const sampleKJVConf = `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
BlockType=BOOK
CompressType=ZIP
SourceType=OSIS
Encoding=UTF-8
Lang=en
Description=King James Version (1769) with Strongs Numbers
About=The King James Version of the Holy Bible
Version=3.1
`

const sampleDRCConf = `[DRC]
DataPath=./modules/texts/ztext/drc/
ModDrv=zText
BlockType=BOOK
CompressType=ZIP
SourceType=ThML
Encoding=UTF-8
Lang=en
Description=Douay-Rheims Catholic Bible
Version=1.0
`

const sampleStrongsConf = `[StrongsGreek]
DataPath=./modules/lexdict/zld/strongsgreek/
ModDrv=zLD
CompressType=ZIP
SourceType=TEI
Encoding=UTF-8
Lang=grc
Feature=StrongsNumbers
Description=Strong's Greek Dictionary
Version=1.3
`

// TestParseModsArchive tests parsing a mods.d.tar.gz archive
func TestParseModsArchive(t *testing.T) {
	confFiles := map[string]string{
		"kjv.conf": sampleKJVConf,
		"drc.conf": sampleDRCConf,
	}
	archive := createTestModsArchive(t, confFiles)

	modules, err := ParseModsArchive(archive)
	if err != nil {
		t.Fatalf("ParseModsArchive() error = %v", err)
	}

	if len(modules) != 2 {
		t.Errorf("ParseModsArchive() returned %d modules, want 2", len(modules))
	}

	// Verify KJV module
	var kjv *ModuleInfo
	for i := range modules {
		if modules[i].ID == "KJV" {
			kjv = &modules[i]
			break
		}
	}

	if kjv == nil {
		t.Fatal("KJV module not found")
	}
	if kjv.Description != "King James Version (1769) with Strongs Numbers" {
		t.Errorf("KJV.Description = %q, want %q", kjv.Description, "King James Version (1769) with Strongs Numbers")
	}
	if kjv.Language != "en" {
		t.Errorf("KJV.Language = %q, want %q", kjv.Language, "en")
	}
	if kjv.DataPath != "./modules/texts/ztext/kjv/" {
		t.Errorf("KJV.DataPath = %q, want %q", kjv.DataPath, "./modules/texts/ztext/kjv/")
	}
}

// TestParseModsArchive_Empty tests parsing an empty archive
func TestParseModsArchive_Empty(t *testing.T) {
	archive := createTestModsArchive(t, map[string]string{})

	modules, err := ParseModsArchive(archive)
	if err != nil {
		t.Fatalf("ParseModsArchive() error = %v", err)
	}
	if len(modules) != 0 {
		t.Errorf("ParseModsArchive() returned %d modules, want 0", len(modules))
	}
}

// TestParseModsArchive_InvalidGzip tests handling of invalid gzip data
func TestParseModsArchive_InvalidGzip(t *testing.T) {
	_, err := ParseModsArchive([]byte("not gzip data"))
	if err == nil {
		t.Error("ParseModsArchive() should return error for invalid gzip")
	}
}

// TestParseModsArchive_InvalidTar tests handling of invalid tar data
func TestParseModsArchive_InvalidTar(t *testing.T) {
	// Create valid gzip with invalid tar content
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	gzw.Write([]byte("not tar data"))
	gzw.Close()

	// Should handle gracefully (no conf files found)
	modules, err := ParseModsArchive(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseModsArchive() error = %v", err)
	}
	if len(modules) != 0 {
		t.Errorf("ParseModsArchive() returned %d modules, want 0", len(modules))
	}
}

// TestParseModsArchive_SkipsNonConfFiles tests that non-.conf files are skipped
func TestParseModsArchive_SkipsNonConfFiles(t *testing.T) {
	confFiles := map[string]string{
		"kjv.conf":   sampleKJVConf,
		"readme.txt": "This is a readme file",
		".gitignore": "*.bak",
	}
	archive := createTestModsArchive(t, confFiles)

	modules, err := ParseModsArchive(archive)
	if err != nil {
		t.Fatalf("ParseModsArchive() error = %v", err)
	}
	if len(modules) != 1 {
		t.Errorf("ParseModsArchive() returned %d modules, want 1", len(modules))
	}
}

// TestParseModuleConf tests parsing a single conf file
func TestParseModuleConf(t *testing.T) {
	module, err := ParseModuleConf([]byte(sampleKJVConf), "kjv.conf")
	if err != nil {
		t.Fatalf("ParseModuleConf() error = %v", err)
	}

	if module.ID != "KJV" {
		t.Errorf("module.ID = %q, want %q", module.ID, "KJV")
	}
	if module.Driver != "zText" {
		t.Errorf("module.Driver = %q, want %q", module.Driver, "zText")
	}
	if module.SourceType != "OSIS" {
		t.Errorf("module.SourceType = %q, want %q", module.SourceType, "OSIS")
	}
	if module.Version != "3.1" {
		t.Errorf("module.Version = %q, want %q", module.Version, "3.1")
	}
}

// TestParseModuleConf_Dictionary tests parsing a dictionary/lexicon conf
func TestParseModuleConf_Dictionary(t *testing.T) {
	module, err := ParseModuleConf([]byte(sampleStrongsConf), "strongsgreek.conf")
	if err != nil {
		t.Fatalf("ParseModuleConf() error = %v", err)
	}

	if module.ID != "StrongsGreek" {
		t.Errorf("module.ID = %q, want %q", module.ID, "StrongsGreek")
	}
	if module.Driver != "zLD" {
		t.Errorf("module.Driver = %q, want %q", module.Driver, "zLD")
	}
	if !containsFeature(module.Features, "StrongsNumbers") {
		t.Errorf("module.Features should contain StrongsNumbers")
	}
}

// TestParseModuleConf_Empty tests parsing empty conf
func TestParseModuleConf_Empty(t *testing.T) {
	_, err := ParseModuleConf([]byte(""), "empty.conf")
	if err == nil {
		t.Error("ParseModuleConf() should return error for empty conf")
	}
}

// TestParseModuleConf_NoHeader tests parsing conf without section header
func TestParseModuleConf_NoHeader(t *testing.T) {
	content := `DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
`
	_, err := ParseModuleConf([]byte(content), "noheader.conf")
	if err == nil {
		t.Error("ParseModuleConf() should return error for conf without header")
	}
}

// TestParseModuleConf_Features tests parsing multiple features
func TestParseModuleConf_Features(t *testing.T) {
	content := `[TestModule]
DataPath=./modules/test/
ModDrv=zText
Feature=StrongsNumbers
Feature=Morphology
Feature=Footnotes
`
	module, err := ParseModuleConf([]byte(content), "test.conf")
	if err != nil {
		t.Fatalf("ParseModuleConf() error = %v", err)
	}

	expectedFeatures := []string{"StrongsNumbers", "Morphology", "Footnotes"}
	for _, f := range expectedFeatures {
		if !containsFeature(module.Features, f) {
			t.Errorf("module.Features should contain %q", f)
		}
	}
}

// TestModuleInfo_Type tests module type detection
func TestModuleInfo_Type(t *testing.T) {
	tests := []struct {
		driver   string
		wantType ModuleType
	}{
		{"zText", ModuleTypeBible},
		{"zText4", ModuleTypeBible},
		{"rawText", ModuleTypeBible},
		{"rawText4", ModuleTypeBible},
		{"zCom", ModuleTypeCommentary},
		{"zCom4", ModuleTypeCommentary},
		{"rawCom", ModuleTypeCommentary},
		{"rawCom4", ModuleTypeCommentary},
		{"zLD", ModuleTypeDictionary},
		{"rawLD", ModuleTypeDictionary},
		{"rawLD4", ModuleTypeDictionary},
		{"RawGenBook", ModuleTypeGenBook},
		{"rawGenBook", ModuleTypeGenBook},
		{"UnknownDriver", ModuleTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			module := ModuleInfo{Driver: tt.driver}
			if got := module.Type(); got != tt.wantType {
				t.Errorf("ModuleInfo{Driver: %q}.Type() = %v, want %v", tt.driver, got, tt.wantType)
			}
		})
	}
}

// TestModuleInfo_IsBible tests Bible detection
func TestModuleInfo_IsBible(t *testing.T) {
	tests := []struct {
		driver string
		want   bool
	}{
		{"zText", true},
		{"zText4", true},
		{"rawText", true},
		{"zCom", false},
		{"zLD", false},
		{"RawGenBook", false},
	}

	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			module := ModuleInfo{Driver: tt.driver}
			if got := module.IsBible(); got != tt.want {
				t.Errorf("ModuleInfo{Driver: %q}.IsBible() = %v, want %v", tt.driver, got, tt.want)
			}
		})
	}
}

// TestExtractModsArchive tests extracting archive to directory
func TestExtractModsArchive(t *testing.T) {
	confFiles := map[string]string{
		"kjv.conf": sampleKJVConf,
		"drc.conf": sampleDRCConf,
	}
	archive := createTestModsArchive(t, confFiles)

	tmpDir := t.TempDir()

	err := ExtractModsArchive(archive, tmpDir)
	if err != nil {
		t.Fatalf("ExtractModsArchive() error = %v", err)
	}

	// Verify files were extracted
	kjvPath := filepath.Join(tmpDir, "mods.d", "kjv.conf")
	if _, err := os.Stat(kjvPath); os.IsNotExist(err) {
		t.Error("kjv.conf was not extracted")
	}

	drcPath := filepath.Join(tmpDir, "mods.d", "drc.conf")
	if _, err := os.Stat(drcPath); os.IsNotExist(err) {
		t.Error("drc.conf was not extracted")
	}
}

// TestExtractModsArchive_OverwriteExisting tests overwriting existing files
func TestExtractModsArchive_OverwriteExisting(t *testing.T) {
	confFiles := map[string]string{
		"kjv.conf": sampleKJVConf,
	}
	archive := createTestModsArchive(t, confFiles)

	tmpDir := t.TempDir()

	// Create existing file
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.MkdirAll(modsDir, 0700)
	existingPath := filepath.Join(modsDir, "kjv.conf")
	os.WriteFile(existingPath, []byte("old content"), 0600)

	err := ExtractModsArchive(archive, tmpDir)
	if err != nil {
		t.Fatalf("ExtractModsArchive() error = %v", err)
	}

	// Verify file was overwritten
	content, _ := os.ReadFile(existingPath)
	if string(content) == "old content" {
		t.Error("Existing file was not overwritten")
	}
}

// TestListAvailableModules tests listing modules from parsed archive
func TestListAvailableModules(t *testing.T) {
	confFiles := map[string]string{
		"kjv.conf":          sampleKJVConf,
		"drc.conf":          sampleDRCConf,
		"strongsgreek.conf": sampleStrongsConf,
	}
	archive := createTestModsArchive(t, confFiles)

	modules, err := ParseModsArchive(archive)
	if err != nil {
		t.Fatalf("ParseModsArchive() error = %v", err)
	}

	// Filter by type
	bibles := FilterByType(modules, ModuleTypeBible)
	if len(bibles) != 2 {
		t.Errorf("FilterByType(Bible) returned %d, want 2", len(bibles))
	}

	dictionaries := FilterByType(modules, ModuleTypeDictionary)
	if len(dictionaries) != 1 {
		t.Errorf("FilterByType(Dictionary) returned %d, want 1", len(dictionaries))
	}
}

// TestFilterByLanguage tests filtering modules by language
func TestFilterByLanguage(t *testing.T) {
	modules := []ModuleInfo{
		{ID: "KJV", Language: "en"},
		{ID: "Luther", Language: "de"},
		{ID: "Vulgate", Language: "la"},
		{ID: "DRC", Language: "en"},
	}

	english := FilterByLanguage(modules, "en")
	if len(english) != 2 {
		t.Errorf("FilterByLanguage(en) returned %d, want 2", len(english))
	}
}

// TestSearchModules tests searching modules by keyword
func TestSearchModules(t *testing.T) {
	modules := []ModuleInfo{
		{ID: "KJV", Description: "King James Version"},
		{ID: "NKJV", Description: "New King James Version"},
		{ID: "ESV", Description: "English Standard Version"},
		{ID: "StrongsGreek", Description: "Strong's Greek Dictionary"},
	}

	results := SearchModules(modules, "king")
	if len(results) != 2 {
		t.Errorf("SearchModules(king) returned %d, want 2", len(results))
	}

	results = SearchModules(modules, "dictionary")
	if len(results) != 1 {
		t.Errorf("SearchModules(dictionary) returned %d, want 1", len(results))
	}
}

// Helper function to check if a slice contains a feature
func containsFeature(features []string, feature string) bool {
	for _, f := range features {
		if f == feature {
			return true
		}
	}
	return false
}

// TestParseModsArchiveFromReader tests parsing from an io.Reader
func TestParseModsArchiveFromReader(t *testing.T) {
	confFiles := map[string]string{
		"kjv.conf": sampleKJVConf,
	}
	archive := createTestModsArchive(t, confFiles)

	modules, err := ParseModsArchiveFromReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("ParseModsArchiveFromReader() error = %v", err)
	}
	if len(modules) != 1 {
		t.Errorf("ParseModsArchiveFromReader() returned %d modules, want 1", len(modules))
	}
}

// TestModuleInfo_HasFeature tests feature checking
func TestModuleInfo_HasFeature(t *testing.T) {
	module := ModuleInfo{
		Features: []string{"StrongsNumbers", "Morphology"},
	}

	if !module.HasFeature("StrongsNumbers") {
		t.Error("HasFeature(StrongsNumbers) should return true")
	}
	if !module.HasFeature("Morphology") {
		t.Error("HasFeature(Morphology) should return true")
	}
	if module.HasFeature("Footnotes") {
		t.Error("HasFeature(Footnotes) should return false")
	}
}

// TestModuleInfo_String tests string representation
func TestModuleInfo_String(t *testing.T) {
	module := ModuleInfo{
		ID:          "KJV",
		Description: "King James Version",
		Version:     "3.1",
	}

	str := module.String()
	if str == "" {
		t.Error("ModuleInfo.String() should not return empty string")
	}
	// Should contain the module ID at minimum
	if !bytes.Contains([]byte(str), []byte("KJV")) {
		t.Errorf("ModuleInfo.String() should contain module ID, got %q", str)
	}
}

// TestParseModuleConf_InstallSize tests parsing InstallSize field
func TestParseModuleConf_InstallSize(t *testing.T) {
	content := `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
InstallSize=4232647
`
	module, err := ParseModuleConf([]byte(content), "kjv.conf")
	if err != nil {
		t.Fatalf("ParseModuleConf() error = %v", err)
	}

	if module.InstallSize != 4232647 {
		t.Errorf("module.InstallSize = %d, want 4232647", module.InstallSize)
	}
}

// TestParseModuleConf_InstallSize_Invalid tests parsing invalid InstallSize
func TestParseModuleConf_InstallSize_Invalid(t *testing.T) {
	content := `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
InstallSize=invalid
`
	module, err := ParseModuleConf([]byte(content), "kjv.conf")
	if err != nil {
		t.Fatalf("ParseModuleConf() error = %v", err)
	}

	if module.InstallSize != 0 {
		t.Errorf("module.InstallSize = %d, want 0 for invalid value", module.InstallSize)
	}
}

// TestParseModuleConf_NoInstallSize tests parsing without InstallSize
func TestParseModuleConf_NoInstallSize(t *testing.T) {
	content := `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
`
	module, err := ParseModuleConf([]byte(content), "kjv.conf")
	if err != nil {
		t.Fatalf("ParseModuleConf() error = %v", err)
	}

	if module.InstallSize != 0 {
		t.Errorf("module.InstallSize = %d, want 0 when not specified", module.InstallSize)
	}
}

// parseModsArchiveFromReader is a wrapper for testing
func ParseModsArchiveFromReader(r io.Reader) ([]ModuleInfo, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ParseModsArchive(data)
}

// =============================================================================
// ToSPDXLicense Tests
// =============================================================================

func TestToSPDXLicense(t *testing.T) {
	tests := []struct {
		license  string
		expected string
	}{
		// Empty and dash
		{"", ""},
		{"-", ""},
		{"  ", ""},

		// Public domain variants
		{"Public Domain", "CC-PDDC"},
		{"public domain", "CC-PDDC"},
		{"This is public domain text", "CC-PDDC"},

		// GPL
		{"GPL", "GPL-3.0-or-later"},
		{"gpl", "GPL-3.0-or-later"},

		// Unrestricted
		{"Unrestricted", "Unlicense"},
		{"unrestricted", "Unlicense"},

		// CC0
		{"CC0", "CC0-1.0"},
		{"cc0-1.0", "CC0-1.0"},

		// CC BY-NC-ND
		{"CC BY-NC-ND 4.0", "CC-BY-NC-ND-4.0"},
		{"by-nc-nd 4.0", "CC-BY-NC-ND-4.0"},
		{"CC BY-NC-ND", "CC-BY-NC-ND-3.0"},
		{"by-nc-nd", "CC-BY-NC-ND-3.0"},

		// CC BY-NC-SA
		{"CC BY-NC-SA 4.0", "CC-BY-NC-SA-4.0"},
		{"by-nc-sa 4.0", "CC-BY-NC-SA-4.0"},
		{"CC BY-NC-SA", "CC-BY-NC-SA-3.0"},
		{"by-nc-sa", "CC-BY-NC-SA-3.0"},

		// CC BY-SA
		{"CC BY-SA 4.0", "CC-BY-SA-4.0"},
		{"by-sa 4.0", "CC-BY-SA-4.0"},
		{"CC BY-SA", "CC-BY-SA-3.0"},
		{"by-sa", "CC-BY-SA-3.0"},

		// CC BY-ND
		{"CC BY-ND 4.0", "CC-BY-ND-4.0"},
		{"by-nd 4.0", "CC-BY-ND-4.0"},
		{"CC BY-ND", "CC-BY-ND-3.0"},
		{"by-nd", "CC-BY-ND-3.0"},

		// CC BY
		{"CC BY 4.0", "CC-BY-4.0"},
		{"by 4.0", "CC-BY-4.0"},
		{"Attribution 4.0", "CC-BY-4.0"},
		{"Creative Commons: BY", "CC-BY-3.0"},

		// Copyrighted variants
		{"Copyrighted; Permission to distribute granted to CrossWire", "LicenseRef-Copyrighted"},
		{"Copyrighted; free non-commercial distribution", "LicenseRef-Copyrighted-Free"},
		{"copyrighted free", "LicenseRef-Copyrighted-Free"},

		// Unknown - returns original
		{"Some Custom License", "Some Custom License"},
		{"All Rights Reserved", "All Rights Reserved"},
	}

	for _, tt := range tests {
		t.Run(tt.license, func(t *testing.T) {
			result := ToSPDXLicense(tt.license)
			if result != tt.expected {
				t.Errorf("ToSPDXLicense(%q) = %q, want %q", tt.license, result, tt.expected)
			}
		})
	}
}

// TestModuleInfo_LicenseSPDX tests the LicenseSPDX method
func TestModuleInfo_LicenseSPDX(t *testing.T) {
	tests := []struct {
		license  string
		expected string
	}{
		{"Public Domain", "CC-PDDC"},
		{"GPL", "GPL-3.0-or-later"},
		{"CC BY-SA 4.0", "CC-BY-SA-4.0"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.license, func(t *testing.T) {
			module := ModuleInfo{License: tt.license}
			result := module.LicenseSPDX()
			if result != tt.expected {
				t.Errorf("ModuleInfo{License: %q}.LicenseSPDX() = %q, want %q",
					tt.license, result, tt.expected)
			}
		})
	}
}

// TestParseModuleConf_License tests parsing license field
func TestParseModuleConf_License(t *testing.T) {
	content := `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
DistributionLicense=Public Domain
`
	module, err := ParseModuleConf([]byte(content), "kjv.conf")
	if err != nil {
		t.Fatalf("ParseModuleConf() error = %v", err)
	}

	if module.License != "Public Domain" {
		t.Errorf("module.License = %q, want 'Public Domain'", module.License)
	}
	if module.LicenseSPDX() != "CC-PDDC" {
		t.Errorf("module.LicenseSPDX() = %q, want 'CC-PDDC'", module.LicenseSPDX())
	}
}

// TestParseModuleConf_Copyright tests parsing copyright field
func TestParseModuleConf_Copyright(t *testing.T) {
	content := `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
Copyright=Copyright 2024 Example
`
	module, err := ParseModuleConf([]byte(content), "kjv.conf")
	if err != nil {
		t.Fatalf("ParseModuleConf() error = %v", err)
	}

	if module.Copyright != "Copyright 2024 Example" {
		t.Errorf("module.Copyright = %q, want 'Copyright 2024 Example'", module.Copyright)
	}
}

// TestParseModuleConf_About tests parsing about field
func TestParseModuleConf_About(t *testing.T) {
	content := `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
About=This is the King James Version of the Holy Bible
`
	module, err := ParseModuleConf([]byte(content), "kjv.conf")
	if err != nil {
		t.Fatalf("ParseModuleConf() error = %v", err)
	}

	if module.About == "" {
		t.Error("module.About should not be empty")
	}
}

// =============================================================================
// Error Type Tests
// =============================================================================

func TestHTTPError_Error(t *testing.T) {
	err := &HTTPError{StatusCode: 404, Status: "404 Not Found"}
	expected := "HTTP error: 404 Not Found"
	if err.Error() != expected {
		t.Errorf("HTTPError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestHTTPError_IsNotFound(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{404, true},
		{200, false},
		{500, false},
		{403, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
			err := &HTTPError{StatusCode: tt.code}
			if err.IsNotFound() != tt.expected {
				t.Errorf("HTTPError{%d}.IsNotFound() = %v, want %v", tt.code, err.IsNotFound(), tt.expected)
			}
		})
	}
}

func TestFTPError_Error(t *testing.T) {
	err := &FTPError{Code: 550, Message: "File not found"}
	expected := "FTP error 550: File not found"
	if err.Error() != expected {
		t.Errorf("FTPError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestFTPError_IsNotFound(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{550, true},
		{226, false},
		{530, false},
		{421, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
			err := &FTPError{Code: tt.code}
			if err.IsNotFound() != tt.expected {
				t.Errorf("FTPError{%d}.IsNotFound() = %v, want %v", tt.code, err.IsNotFound(), tt.expected)
			}
		})
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"HTTPError 404", &HTTPError{StatusCode: 404}, true},
		{"HTTPError 500", &HTTPError{StatusCode: 500}, false},
		{"FTPError 550", &FTPError{Code: 550}, true},
		{"FTPError 226", &FTPError{Code: 226}, false},
		{"string with 404", fmt.Errorf("got 404 error"), true},
		{"string with 550", fmt.Errorf("got 550 error"), true},
		{"string with not found", fmt.Errorf("file not found"), true},
		{"generic error", fmt.Errorf("generic error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNotFoundError(tt.err)
			if result != tt.expected {
				t.Errorf("IsNotFoundError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Source URL Tests
// =============================================================================

func TestSource_ModulePackageURL(t *testing.T) {
	source := &Source{
		Name:      "test",
		Type:      SourceTypeHTTPS,
		Host:      "example.com",
		Directory: "/sword",
	}

	url := source.ModulePackageURL("KJV")
	if url == "" {
		t.Error("ModulePackageURL should not return empty string")
	}
	if !strings.Contains(url, "KJV") && !strings.Contains(url, "kjv") {
		t.Errorf("ModulePackageURL should contain module name, got %s", url)
	}
}
