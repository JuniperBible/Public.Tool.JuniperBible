package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Base test Bibles - 11 well-known modules for quick testing
var baseBibles = []struct {
	name        string
	lang        string
	description string
	modDrv      string
}{
	{"KJV", "en", "King James Version", "zText"},
	{"ESV", "en", "English Standard Version", "zText"},
	{"NIV", "en", "New International Version", "zText"},
	{"NASB", "en", "New American Standard Bible", "zText"},
	{"RSV", "en", "Revised Standard Version", "zText"},
	{"ASV", "en", "American Standard Version", "zText"},
	{"WEB", "en", "World English Bible", "zText"},
	{"YLT", "en", "Young's Literal Translation", "zText"},
	{"Darby", "en", "Darby Translation", "zText"},
	{"Geneva", "en", "Geneva Bible 1599", "zText"},
	{"Vulgate", "la", "Latin Vulgate", "zText"},
}

// TestBaseIngestion tests ingestion with 11 mock Bible modules.
// This is the quick test that doesn't require real SWORD modules.
func TestBaseIngestion(t *testing.T) {
	// Create mock SWORD structure
	swordDir, err := os.MkdirTemp("", "sword-base-test-*")
	if err != nil {
		t.Fatalf("failed to create temp sword dir: %v", err)
	}
	defer os.RemoveAll(swordDir)

	// Create output directory
	outputDir, err := os.MkdirTemp("", "capsules-base-test-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Create mock modules
	createMockSwordModules(t, swordDir, baseBibles)

	// Test listing modules
	t.Run("ListBaseModules", func(t *testing.T) {
		modules, err := ListModules(swordDir)
		if err != nil {
			t.Fatalf("ListModules failed: %v", err)
		}

		if len(modules) != len(baseBibles) {
			t.Errorf("expected %d modules, got %d", len(baseBibles), len(modules))
		}

		// Verify each module
		moduleMap := make(map[string]ModuleInfo)
		for _, m := range modules {
			moduleMap[m.Name] = m
		}

		for _, expected := range baseBibles {
			m, ok := moduleMap[expected.name]
			if !ok {
				t.Errorf("module %s not found", expected.name)
				continue
			}
			if m.Language != expected.lang {
				t.Errorf("module %s: expected lang %s, got %s", expected.name, expected.lang, m.Language)
			}
			if m.Type != "Bible" {
				t.Errorf("module %s: expected type Bible, got %s", expected.name, m.Type)
			}
		}
	})

	// Test ingestion of each module
	t.Run("IngestBaseModules", func(t *testing.T) {
		modules, _ := ListModules(swordDir)

		for _, m := range modules {
			if m.Type != "Bible" {
				continue
			}

			capsulePath := filepath.Join(outputDir, m.Name+".capsule.tar.gz")
			err := createModuleCapsule(swordDir, m, capsulePath)
			if err != nil {
				t.Errorf("failed to create capsule for %s: %v", m.Name, err)
				continue
			}

			// Verify capsule was created
			if _, err := os.Stat(capsulePath); os.IsNotExist(err) {
				t.Errorf("capsule not created for %s", m.Name)
				continue
			}

			// Verify capsule contents
			verifyBaseCapsule(t, capsulePath, m.Name)
		}
	})

	// Verify all 11 capsules created
	t.Run("VerifyAllBaseCapsules", func(t *testing.T) {
		entries, err := os.ReadDir(outputDir)
		if err != nil {
			t.Fatalf("failed to read output dir: %v", err)
		}

		capsuleCount := 0
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".capsule.tar.gz") {
				capsuleCount++
			}
		}

		if capsuleCount != len(baseBibles) {
			t.Errorf("expected %d capsules, got %d", len(baseBibles), capsuleCount)
		}
	})
}

// TestComprehensiveIngestion tests ingestion of all Bibles from ~/.sword.
// This test requires real SWORD modules to be installed.
// Run with: go test -v -run TestComprehensiveIngestion
func TestComprehensiveIngestion(t *testing.T) {
	// Skip IR extraction to speed up the test - IR extraction is tested separately
	skipIRExtraction = true
	defer func() { skipIRExtraction = false }()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	swordPath := filepath.Join(home, ".sword")
	if _, err := os.Stat(swordPath); os.IsNotExist(err) {
		t.Skip("~/.sword not found - skipping comprehensive test")
	}

	modsDir := filepath.Join(swordPath, "mods.d")
	if _, err := os.Stat(modsDir); os.IsNotExist(err) {
		t.Skip("~/.sword/mods.d not found - skipping comprehensive test")
	}

	// Create output directory
	outputDir, err := os.MkdirTemp("", "capsules-comprehensive-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// List all modules
	modules, err := ListModules(swordPath)
	if err != nil {
		t.Fatalf("ListModules failed: %v", err)
	}

	// Filter to Bible modules only
	var bibles []ModuleInfo
	for _, m := range modules {
		if m.Type == "Bible" {
			bibles = append(bibles, m)
		}
	}

	if len(bibles) == 0 {
		t.Skip("no Bible modules found in ~/.sword")
	}

	t.Logf("Found %d Bible modules in ~/.sword", len(bibles))

	// Track statistics
	var successful, failed, skipped int
	var errors []string

	for _, m := range bibles {
		t.Run("Ingest_"+m.Name, func(t *testing.T) {
			if m.Encrypted {
				t.Logf("Skipping %s (encrypted)", m.Name)
				skipped++
				return
			}

			capsulePath := filepath.Join(outputDir, m.Name+".capsule.tar.gz")
			err := createModuleCapsule(swordPath, m, capsulePath)
			if err != nil {
				failed++
				errors = append(errors, m.Name+": "+err.Error())
				t.Logf("Failed to create capsule for %s: %v", m.Name, err)
				return
			}

			// Verify capsule exists and has content
			info, err := os.Stat(capsulePath)
			if err != nil {
				failed++
				errors = append(errors, m.Name+": capsule not created")
				t.Errorf("capsule not created for %s", m.Name)
				return
			}

			if info.Size() < 100 {
				failed++
				errors = append(errors, m.Name+": capsule too small")
				t.Errorf("capsule for %s too small (%d bytes)", m.Name, info.Size())
				return
			}

			// Verify capsule structure
			if err := verifyCapsuleStructure(capsulePath); err != nil {
				failed++
				errors = append(errors, m.Name+": "+err.Error())
				t.Errorf("capsule %s structure invalid: %v", m.Name, err)
				return
			}

			successful++
			t.Logf("Successfully created capsule for %s (%d bytes)", m.Name, info.Size())
		})
	}

	// Summary
	t.Logf("\n=== Comprehensive Ingestion Summary ===")
	t.Logf("Total Bibles: %d", len(bibles))
	t.Logf("Successful:   %d", successful)
	t.Logf("Failed:       %d", failed)
	t.Logf("Skipped:      %d (encrypted)", skipped)

	if len(errors) > 0 {
		t.Logf("\nErrors:")
		for _, e := range errors {
			t.Logf("  - %s", e)
		}
	}

	// Require at least 50% success rate
	successRate := float64(successful) / float64(len(bibles)-skipped)
	if successRate < 0.5 && len(bibles)-skipped > 0 {
		t.Errorf("Success rate too low: %.1f%% (expected >= 50%%)", successRate*100)
	}
}

// createMockSwordModules creates mock SWORD module structure for testing.
func createMockSwordModules(t *testing.T, swordDir string, bibles []struct {
	name        string
	lang        string
	description string
	modDrv      string
}) {
	t.Helper()

	modsDir := filepath.Join(swordDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	for _, bible := range bibles {
		// Create conf file
		confContent := generateConfFile(bible.name, bible.lang, bible.description, bible.modDrv)
		confPath := filepath.Join(modsDir, strings.ToLower(bible.name)+".conf")
		if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
			t.Fatalf("failed to write conf for %s: %v", bible.name, err)
		}

		// Create mock data directory
		dataPath := filepath.Join(swordDir, "modules", "texts", "ztext", strings.ToLower(bible.name))
		if err := os.MkdirAll(dataPath, 0755); err != nil {
			t.Fatalf("failed to create data dir for %s: %v", bible.name, err)
		}

		// Create minimal mock data files
		createMockZTextData(t, dataPath, bible.name)
	}
}

// generateConfFile generates a SWORD conf file content.
func generateConfFile(name, lang, desc, modDrv string) string {
	return "[" + name + "]\n" +
		"DataPath=./modules/texts/ztext/" + strings.ToLower(name) + "/\n" +
		"ModDrv=" + modDrv + "\n" +
		"Lang=" + lang + "\n" +
		"Description=" + desc + "\n" +
		"Version=1.0\n" +
		"Encoding=UTF-8\n" +
		"CompressType=ZIP\n" +
		"BlockType=BOOK\n" +
		"SourceType=OSIS\n" +
		"Versification=KJV\n"
}

// createMockZTextData creates minimal mock zText data files.
func createMockZTextData(t *testing.T, dataPath, name string) {
	t.Helper()

	// Create OT and NT index and data files
	for _, testament := range []string{"ot", "nt"} {
		// Block index (compressed block locations)
		bzs := filepath.Join(dataPath, testament+".bzs")
		if err := os.WriteFile(bzs, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, 0600); err != nil {
			t.Fatalf("failed to write %s: %v", bzs, err)
		}

		// Block data (compressed blocks)
		bzz := filepath.Join(dataPath, testament+".bzz")
		if err := os.WriteFile(bzz, createMockCompressedBlock(name), 0600); err != nil {
			t.Fatalf("failed to write %s: %v", bzz, err)
		}

		// Verse index (verse locations within blocks)
		bzv := filepath.Join(dataPath, testament+".bzv")
		if err := os.WriteFile(bzv, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, 0600); err != nil {
			t.Fatalf("failed to write %s: %v", bzv, err)
		}
	}
}

// createMockCompressedBlock creates a mock zlib-compressed block.
func createMockCompressedBlock(name string) []byte {
	// Simple mock data - in real zText this would be zlib compressed verse data
	return []byte("Mock data for " + name)
}

// verifyBaseCapsule verifies the structure of a created capsule.
func verifyBaseCapsule(t *testing.T, capsulePath, moduleName string) {
	t.Helper()

	f, err := os.Open(capsulePath)
	if err != nil {
		t.Errorf("failed to open capsule %s: %v", capsulePath, err)
		return
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		t.Errorf("failed to create gzip reader for %s: %v", capsulePath, err)
		return
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	var hasManifest, hasModsD, hasData bool

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Errorf("error reading tar for %s: %v", capsulePath, err)
			return
		}

		if strings.Contains(header.Name, "manifest.json") {
			hasManifest = true
			// Verify manifest content
			var manifest map[string]interface{}
			data, _ := io.ReadAll(tr)
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Errorf("invalid manifest.json in %s: %v", capsulePath, err)
			} else {
				if manifest["id"] != moduleName {
					t.Errorf("manifest id mismatch: expected %s, got %v", moduleName, manifest["id"])
				}
			}
		}
		if strings.Contains(header.Name, "mods.d/") {
			hasModsD = true
		}
		if strings.Contains(header.Name, "modules/") {
			hasData = true
		}
	}

	if !hasManifest {
		t.Errorf("capsule %s missing manifest.json", capsulePath)
	}
	if !hasModsD {
		t.Errorf("capsule %s missing mods.d/", capsulePath)
	}
	if !hasData {
		t.Errorf("capsule %s missing module data", capsulePath)
	}
}

// verifyCapsuleStructure verifies a capsule has valid structure.
func verifyCapsuleStructure(capsulePath string) error {
	f, err := os.Open(capsulePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	var hasManifest bool
	var fileCount int

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		fileCount++
		if strings.Contains(header.Name, "manifest.json") {
			hasManifest = true

			// Verify manifest is valid JSON
			data, _ := io.ReadAll(tr)
			var manifest map[string]interface{}
			if err := json.Unmarshal(data, &manifest); err != nil {
				return err
			}
		}
	}

	if !hasManifest {
		return os.ErrNotExist
	}

	if fileCount < 3 {
		return os.ErrInvalid
	}

	return nil
}

// TestIngestSpecificModules tests ingesting specific modules by name.
func TestIngestSpecificModules(t *testing.T) {
	// Create mock SWORD structure with extra modules
	swordDir, err := os.MkdirTemp("", "sword-specific-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(swordDir)

	outputDir, err := os.MkdirTemp("", "capsules-specific-test-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Create all base modules
	createMockSwordModules(t, swordDir, baseBibles)

	// Get modules and ingest only specific ones
	modules, err := ListModules(swordDir)
	if err != nil {
		t.Fatalf("ListModules failed: %v", err)
	}

	// Only ingest KJV, ESV, and NIV
	targetModules := map[string]bool{"KJV": true, "ESV": true, "NIV": true}

	for _, m := range modules {
		if !targetModules[m.Name] {
			continue
		}

		capsulePath := filepath.Join(outputDir, m.Name+".capsule.tar.gz")
		if err := createModuleCapsule(swordDir, m, capsulePath); err != nil {
			t.Errorf("failed to ingest %s: %v", m.Name, err)
		}
	}

	// Verify only 3 capsules created
	entries, _ := os.ReadDir(outputDir)
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".capsule.tar.gz") {
			count++
		}
	}

	if count != 3 {
		t.Errorf("expected 3 capsules, got %d", count)
	}
}

// TestListCommand tests the CLI list command.
func TestListCommand(t *testing.T) {
	// Create mock SWORD structure
	swordDir, err := os.MkdirTemp("", "sword-list-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(swordDir)

	// Create 5 modules for this test
	createMockSwordModules(t, swordDir, baseBibles[:5])

	// Use ListModules directly (cmdList uses os.Args)
	modules, err := ListModules(swordDir)
	if err != nil {
		t.Fatalf("ListModules failed: %v", err)
	}

	// Filter to Bible type
	var bibles []ModuleInfo
	for _, m := range modules {
		if m.Type == "Bible" {
			bibles = append(bibles, m)
		}
	}

	if len(bibles) != 5 {
		t.Errorf("expected 5 Bible modules, got %d", len(bibles))
	}
}

// TestIngestOutputFormats tests that capsule output format is correct.
func TestIngestOutputFormats(t *testing.T) {
	swordDir, err := os.MkdirTemp("", "sword-format-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(swordDir)

	outputDir, err := os.MkdirTemp("", "capsules-format-test-*")
	if err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Create single module
	createMockSwordModules(t, swordDir, baseBibles[:1])

	modules, _ := ListModules(swordDir)
	if len(modules) == 0 {
		t.Fatal("no modules created")
	}

	m := modules[0]
	capsulePath := filepath.Join(outputDir, m.Name+".capsule.tar.gz")

	if err := createModuleCapsule(swordDir, m, capsulePath); err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Read and verify tar.gz structure
	f, err := os.Open(capsulePath)
	if err != nil {
		t.Fatalf("failed to open capsule: %v", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("failed to read gzip: %v", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	var files []string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read error: %v", err)
		}
		files = append(files, header.Name)
	}

	// Verify expected structure
	expectedPaths := []string{"manifest.json", "mods.d/", "modules/"}
	for _, expected := range expectedPaths {
		found := false
		for _, f := range files {
			if strings.Contains(f, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected path containing %s not found in capsule", expected)
		}
	}

	t.Logf("Capsule contains %d files/directories", len(files))
}

// BenchmarkBaseIngestion benchmarks ingestion of base modules.
func BenchmarkBaseIngestion(b *testing.B) {
	swordDir, _ := os.MkdirTemp("", "sword-bench-*")
	defer os.RemoveAll(swordDir)

	outputDir, _ := os.MkdirTemp("", "capsules-bench-*")
	defer os.RemoveAll(outputDir)

	// Setup: create mock modules
	modsDir := filepath.Join(swordDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	for _, bible := range baseBibles {
		confContent := generateConfFile(bible.name, bible.lang, bible.description, bible.modDrv)
		confPath := filepath.Join(modsDir, strings.ToLower(bible.name)+".conf")
		os.WriteFile(confPath, []byte(confContent), 0600)

		dataPath := filepath.Join(swordDir, "modules", "texts", "ztext", strings.ToLower(bible.name))
		os.MkdirAll(dataPath, 0755)

		// Create minimal mock data
		for _, testament := range []string{"ot", "nt"} {
			os.WriteFile(filepath.Join(dataPath, testament+".bzs"), []byte{0, 0, 0, 0, 0, 0, 0, 0}, 0600)
			os.WriteFile(filepath.Join(dataPath, testament+".bzz"), []byte("mock"), 0600)
			os.WriteFile(filepath.Join(dataPath, testament+".bzv"), []byte{0, 0, 0, 0, 0, 0}, 0600)
		}
	}

	modules, _ := ListModules(swordDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, m := range modules {
			if m.Type != "Bible" {
				continue
			}
			capsulePath := filepath.Join(outputDir, m.Name+".capsule.tar.gz")
			createModuleCapsule(swordDir, m, capsulePath)
			os.Remove(capsulePath)
		}
	}
}

// TestEncryptedModuleSkip verifies encrypted modules are properly detected.
func TestEncryptedModuleSkip(t *testing.T) {
	swordDir, err := os.MkdirTemp("", "sword-encrypted-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(swordDir)

	modsDir := filepath.Join(swordDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Create an encrypted module conf (CipherKey with a value means encrypted)
	confEncrypted := `[EncryptedBible]
DataPath=./modules/texts/ztext/encrypted/
ModDrv=zText
Lang=en
Description=Encrypted Bible
CipherKey=secret123
`
	os.WriteFile(filepath.Join(modsDir, "encrypted.conf"), []byte(confEncrypted), 0600)

	// Create a non-encrypted module conf (no CipherKey or empty means not encrypted)
	confPlain := `[PlainBible]
DataPath=./modules/texts/ztext/plain/
ModDrv=zText
Lang=en
Description=Plain Bible
`
	os.WriteFile(filepath.Join(modsDir, "plain.conf"), []byte(confPlain), 0600)

	// Create data directories
	os.MkdirAll(filepath.Join(swordDir, "modules", "texts", "ztext", "encrypted"), 0755)
	os.MkdirAll(filepath.Join(swordDir, "modules", "texts", "ztext", "plain"), 0755)

	modules, err := ListModules(swordDir)
	if err != nil {
		t.Fatalf("ListModules failed: %v", err)
	}

	if len(modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(modules))
	}

	// Check encryption status
	encryptedCount := 0
	plainCount := 0
	for _, m := range modules {
		if m.Encrypted {
			encryptedCount++
			if m.Name != "EncryptedBible" {
				t.Errorf("wrong module marked as encrypted: %s", m.Name)
			}
		} else {
			plainCount++
			if m.Name != "PlainBible" {
				t.Errorf("wrong module marked as plain: %s", m.Name)
			}
		}
	}

	if encryptedCount != 1 {
		t.Errorf("expected 1 encrypted module, got %d", encryptedCount)
	}
	if plainCount != 1 {
		t.Errorf("expected 1 plain module, got %d", plainCount)
	}
}

// TestCapsuleManifestContent verifies manifest.json content is correct.
func TestCapsuleManifestContent(t *testing.T) {
	swordDir, _ := os.MkdirTemp("", "sword-manifest-test-*")
	defer os.RemoveAll(swordDir)

	outputDir, _ := os.MkdirTemp("", "capsules-manifest-test-*")
	defer os.RemoveAll(outputDir)

	// Create single module with specific metadata
	modsDir := filepath.Join(swordDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	confContent := `[TestBible]
DataPath=./modules/texts/ztext/testbible/
ModDrv=zText
Lang=grc
Description=Test Greek Bible
Version=2.5
Versification=NRSV
`
	os.WriteFile(filepath.Join(modsDir, "testbible.conf"), []byte(confContent), 0600)

	dataPath := filepath.Join(swordDir, "modules", "texts", "ztext", "testbible")
	os.MkdirAll(dataPath, 0755)
	os.WriteFile(filepath.Join(dataPath, "nt.bzs"), []byte{0, 0, 0, 0}, 0600)
	os.WriteFile(filepath.Join(dataPath, "nt.bzz"), []byte("data"), 0600)
	os.WriteFile(filepath.Join(dataPath, "nt.bzv"), []byte{0, 0, 0, 0}, 0600)

	modules, _ := ListModules(swordDir)
	if len(modules) == 0 {
		t.Fatal("no modules found")
	}

	capsulePath := filepath.Join(outputDir, "TestBible.capsule.tar.gz")
	if err := createModuleCapsule(swordDir, modules[0], capsulePath); err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Extract and verify manifest
	f, _ := os.Open(capsulePath)
	defer f.Close()
	gzr, _ := gzip.NewReader(f)
	defer gzr.Close()
	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar error: %v", err)
		}

		if strings.Contains(header.Name, "manifest.json") {
			var buf bytes.Buffer
			io.Copy(&buf, tr)

			var manifest map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &manifest); err != nil {
				t.Fatalf("failed to parse manifest: %v", err)
			}

			// Verify manifest fields
			if manifest["id"] != "TestBible" {
				t.Errorf("expected id 'TestBible', got %v", manifest["id"])
			}
			if manifest["language"] != "grc" {
				t.Errorf("expected language 'grc', got %v", manifest["language"])
			}
			if manifest["module_type"] != "bible" {
				t.Errorf("expected module_type 'bible', got %v", manifest["module_type"])
			}
			if manifest["source_format"] != "sword" {
				t.Errorf("expected source_format 'sword', got %v", manifest["source_format"])
			}
			if manifest["versification"] != "NRSV" {
				t.Errorf("expected versification 'NRSV', got %v", manifest["versification"])
			}

			t.Logf("Manifest content verified: %s", buf.String())
			return
		}
	}

	t.Error("manifest.json not found in capsule")
}
