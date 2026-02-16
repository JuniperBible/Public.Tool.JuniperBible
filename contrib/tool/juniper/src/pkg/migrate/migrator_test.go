package migrate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewMigrator(t *testing.T) {
	m := NewMigrator("/source", "/dest")

	if m == nil {
		t.Fatal("NewMigrator() returned nil")
	}
	if m.SourceDir != "/source" {
		t.Errorf("SourceDir = %q, want %q", m.SourceDir, "/source")
	}
	if m.DestDir != "/dest" {
		t.Errorf("DestDir = %q, want %q", m.DestDir, "/dest")
	}
	if m.Verbose {
		t.Error("Verbose should default to false")
	}
}

func TestMigrator_Verbose(t *testing.T) {
	m := NewMigrator("/source", "/dest")
	m.Verbose = true

	if !m.Verbose {
		t.Error("Verbose should be settable to true")
	}
}

func TestMigrateResult_Structure(t *testing.T) {
	result := &MigrateResult{
		ModulesFound:   10,
		ModulesCopied:  8,
		ModulesSkipped: 2,
		Errors:         []string{"error1", "error2"},
	}

	if result.ModulesFound != 10 {
		t.Errorf("ModulesFound = %d, want 10", result.ModulesFound)
	}
	if result.ModulesCopied != 8 {
		t.Errorf("ModulesCopied = %d, want 8", result.ModulesCopied)
	}
	if result.ModulesSkipped != 2 {
		t.Errorf("ModulesSkipped = %d, want 2", result.ModulesSkipped)
	}
	if len(result.Errors) != 2 {
		t.Errorf("len(Errors) = %d, want 2", len(result.Errors))
	}
}

func TestCopyFile(t *testing.T) {
	// Create source file
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "source.txt")
	content := []byte("test content")
	if err := os.WriteFile(srcFile, content, 0600); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Copy to destination
	dstDir := t.TempDir()
	dstFile := filepath.Join(dstDir, "nested", "dest.txt")

	err := copyFile(srcFile, dstFile)
	if err != nil {
		t.Fatalf("copyFile() returned error: %v", err)
	}

	// Verify destination exists
	if _, err := os.Stat(dstFile); os.IsNotExist(err) {
		t.Error("Destination file was not created")
	}

	// Verify content matches
	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination: %v", err)
	}

	if string(dstContent) != string(content) {
		t.Errorf("Content mismatch: got %q, want %q", dstContent, content)
	}
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	dstDir := t.TempDir()
	err := copyFile("/nonexistent/source.txt", filepath.Join(dstDir, "dest.txt"))
	if err == nil {
		t.Error("copyFile() should return error for non-existent source")
	}
}

func TestCopyDir(t *testing.T) {
	// Create source directory structure
	srcDir := t.TempDir()
	srcSubDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(srcSubDir, 0755); err != nil {
		t.Fatalf("Failed to create source subdir: %v", err)
	}

	// Create files
	files := map[string]string{
		"file1.txt":        "content1",
		"subdir/file2.txt": "content2",
	}
	for name, content := range files {
		path := filepath.Join(srcDir, name)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatalf("Failed to create %s: %v", name, err)
		}
	}

	// Copy directory
	dstDir := filepath.Join(t.TempDir(), "dest")
	err := copyDir(srcDir, dstDir)
	if err != nil {
		t.Fatalf("copyDir() returned error: %v", err)
	}

	// Verify all files copied
	for name, expectedContent := range files {
		path := filepath.Join(dstDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Failed to read %s: %v", name, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("Content mismatch for %s: got %q, want %q", name, content, expectedContent)
		}
	}
}

func TestCopyDir_SourceNotFound(t *testing.T) {
	dstDir := t.TempDir()
	err := copyDir("/nonexistent/source", filepath.Join(dstDir, "dest"))
	if err == nil {
		t.Error("copyDir() should return error for non-existent source")
	}
}

func TestCopyDir_EmptyDirectory(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "dest")

	err := copyDir(srcDir, dstDir)
	if err != nil {
		t.Fatalf("copyDir() returned error: %v", err)
	}

	// Verify destination exists
	info, err := os.Stat(dstDir)
	if err != nil {
		t.Errorf("Destination directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Destination should be a directory")
	}
}

func TestCopyDir_PreservesStructure(t *testing.T) {
	srcDir := t.TempDir()

	// Create nested structure
	dirs := []string{"a", "a/b", "a/b/c", "x/y/z"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(srcDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create %s: %v", dir, err)
		}
		// Create a file in each
		filePath := filepath.Join(srcDir, dir, "test.txt")
		if err := os.WriteFile(filePath, []byte(dir), 0600); err != nil {
			t.Fatalf("Failed to create file in %s: %v", dir, err)
		}
	}

	dstDir := filepath.Join(t.TempDir(), "dest")
	err := copyDir(srcDir, dstDir)
	if err != nil {
		t.Fatalf("copyDir() returned error: %v", err)
	}

	// Verify structure
	for _, dir := range dirs {
		dirPath := filepath.Join(dstDir, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Errorf("Directory %s not copied", dir)
		}

		filePath := filepath.Join(dstDir, dir, "test.txt")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("File in %s not copied", dir)
		}
	}
}

func TestMigrator_Migrate_EmptySource(t *testing.T) {
	// Create empty SWORD-like directory structure
	srcDir := t.TempDir()
	modsDir := filepath.Join(srcDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("Failed to create mods.d: %v", err)
	}

	dstDir := t.TempDir()
	m := NewMigrator(srcDir, dstDir)

	result, err := m.Migrate(nil)
	if err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	if result.ModulesFound != 0 {
		t.Errorf("ModulesFound = %d, want 0", result.ModulesFound)
	}
	if result.ModulesCopied != 0 {
		t.Errorf("ModulesCopied = %d, want 0", result.ModulesCopied)
	}
}

func TestMigrator_Migrate_NonexistentSource(t *testing.T) {
	m := NewMigrator("/nonexistent/sword/path", t.TempDir())

	_, err := m.Migrate(nil)
	if err == nil {
		t.Error("Migrate() should return error for non-existent source")
	}
}

func TestMigrator_ListModules_EmptySource(t *testing.T) {
	// Create empty SWORD-like directory structure
	srcDir := t.TempDir()
	modsDir := filepath.Join(srcDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("Failed to create mods.d: %v", err)
	}

	m := NewMigrator(srcDir, t.TempDir())

	modules, err := m.ListModules()
	if err != nil {
		t.Fatalf("ListModules() returned error: %v", err)
	}

	if len(modules) != 0 {
		t.Errorf("len(modules) = %d, want 0", len(modules))
	}
}

func TestMigrator_ListModules_NonexistentSource(t *testing.T) {
	m := NewMigrator("/nonexistent/sword/path", t.TempDir())

	_, err := m.ListModules()
	if err == nil {
		t.Error("ListModules() should return error for non-existent source")
	}
}

// Helper to create a minimal SWORD module structure for testing
func createTestModule(t *testing.T, srcDir, modID string) {
	t.Helper()

	// Create mods.d directory and conf file
	modsDir := filepath.Join(srcDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("Failed to create mods.d: %v", err)
	}

	confContent := "[" + modID + "]\n" +
		"Description=Test Module\n" +
		"ModDrv=zText\n" +
		"DataPath=./modules/texts/ztext/" + modID + "/\n"

	confPath := filepath.Join(modsDir, modID+".conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		t.Fatalf("Failed to create conf file: %v", err)
	}

	// Create data directory with a test file
	dataDir := filepath.Join(srcDir, "modules", "texts", "ztext", modID)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	testFile := filepath.Join(dataDir, "test.bzz")
	if err := os.WriteFile(testFile, []byte("test data"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
}

func TestMigrator_Migrate_SingleModule(t *testing.T) {
	srcDir := t.TempDir()
	createTestModule(t, srcDir, "TestMod")

	dstDir := t.TempDir()
	m := NewMigrator(srcDir, dstDir)

	result, err := m.Migrate(nil)
	if err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	if result.ModulesFound != 1 {
		t.Errorf("ModulesFound = %d, want 1", result.ModulesFound)
	}
	if result.ModulesCopied != 1 {
		t.Errorf("ModulesCopied = %d, want 1", result.ModulesCopied)
	}

	// Verify files were copied
	confPath := filepath.Join(dstDir, "incoming", "mods.d", "TestMod.conf")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Error("Conf file was not copied")
	}

	dataFile := filepath.Join(dstDir, "incoming", "modules", "texts", "ztext", "TestMod", "test.bzz")
	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		t.Error("Data file was not copied")
	}
}

func TestMigrator_Migrate_MultipleModules(t *testing.T) {
	srcDir := t.TempDir()
	createTestModule(t, srcDir, "Mod1")
	createTestModule(t, srcDir, "Mod2")
	createTestModule(t, srcDir, "Mod3")

	dstDir := t.TempDir()
	m := NewMigrator(srcDir, dstDir)

	result, err := m.Migrate(nil)
	if err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	if result.ModulesFound != 3 {
		t.Errorf("ModulesFound = %d, want 3", result.ModulesFound)
	}
	if result.ModulesCopied != 3 {
		t.Errorf("ModulesCopied = %d, want 3", result.ModulesCopied)
	}
}

func TestMigrator_Migrate_Filter(t *testing.T) {
	srcDir := t.TempDir()
	createTestModule(t, srcDir, "KJV")
	createTestModule(t, srcDir, "ESV")
	createTestModule(t, srcDir, "NIV")

	dstDir := t.TempDir()
	m := NewMigrator(srcDir, dstDir)

	// Only migrate KJV
	result, err := m.Migrate([]string{"KJV"})
	if err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	if result.ModulesFound != 3 {
		t.Errorf("ModulesFound = %d, want 3", result.ModulesFound)
	}
	if result.ModulesCopied != 1 {
		t.Errorf("ModulesCopied = %d, want 1", result.ModulesCopied)
	}
	if result.ModulesSkipped != 2 {
		t.Errorf("ModulesSkipped = %d, want 2", result.ModulesSkipped)
	}
}

func TestMigrator_Migrate_FilterCaseInsensitive(t *testing.T) {
	srcDir := t.TempDir()
	createTestModule(t, srcDir, "KJV")

	dstDir := t.TempDir()
	m := NewMigrator(srcDir, dstDir)

	// Filter with different case
	result, err := m.Migrate([]string{"kjv"})
	if err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	if result.ModulesCopied != 1 {
		t.Errorf("ModulesCopied = %d, want 1 (case-insensitive filter)", result.ModulesCopied)
	}
}

func TestMigrator_ListModules(t *testing.T) {
	srcDir := t.TempDir()
	createTestModule(t, srcDir, "Mod1")
	createTestModule(t, srcDir, "Mod2")

	m := NewMigrator(srcDir, t.TempDir())

	modules, err := m.ListModules()
	if err != nil {
		t.Fatalf("ListModules() returned error: %v", err)
	}

	if len(modules) != 2 {
		t.Errorf("len(modules) = %d, want 2", len(modules))
	}
}

func TestMigrator_Migrate_FilterMultiple(t *testing.T) {
	srcDir := t.TempDir()
	createTestModule(t, srcDir, "KJV")
	createTestModule(t, srcDir, "ESV")
	createTestModule(t, srcDir, "NIV")
	createTestModule(t, srcDir, "NASB")

	dstDir := t.TempDir()
	m := NewMigrator(srcDir, dstDir)

	// Migrate KJV and NIV only
	result, err := m.Migrate([]string{"KJV", "NIV"})
	if err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	if result.ModulesFound != 4 {
		t.Errorf("ModulesFound = %d, want 4", result.ModulesFound)
	}
	if result.ModulesCopied != 2 {
		t.Errorf("ModulesCopied = %d, want 2", result.ModulesCopied)
	}
	if result.ModulesSkipped != 2 {
		t.Errorf("ModulesSkipped = %d, want 2", result.ModulesSkipped)
	}
}

func TestMigrator_Migrate_VerboseOutput(t *testing.T) {
	srcDir := t.TempDir()
	createTestModule(t, srcDir, "TestMod")

	dstDir := t.TempDir()
	m := NewMigrator(srcDir, dstDir)
	m.Verbose = true

	result, err := m.Migrate(nil)
	if err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	if result.ModulesCopied != 1 {
		t.Errorf("ModulesCopied = %d, want 1", result.ModulesCopied)
	}
}

func TestCopyFile_CreatesNestedDirectories(t *testing.T) {
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "source.txt")
	if err := os.WriteFile(srcFile, []byte("content"), 0600); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	dstDir := t.TempDir()
	dstFile := filepath.Join(dstDir, "a", "b", "c", "d", "e", "dest.txt")

	err := copyFile(srcFile, dstFile)
	if err != nil {
		t.Fatalf("copyFile() returned error: %v", err)
	}

	if _, err := os.Stat(dstFile); os.IsNotExist(err) {
		t.Error("Destination file was not created with nested directories")
	}
}

func TestCopyDir_NestedEmpty(t *testing.T) {
	srcDir := t.TempDir()

	// Create nested empty directories
	nestedPath := filepath.Join(srcDir, "a", "b", "c")
	if err := os.MkdirAll(nestedPath, 0755); err != nil {
		t.Fatalf("Failed to create nested dirs: %v", err)
	}

	dstDir := filepath.Join(t.TempDir(), "dest")
	err := copyDir(srcDir, dstDir)
	if err != nil {
		t.Fatalf("copyDir() returned error: %v", err)
	}

	// Verify nested structure was copied
	expectedPath := filepath.Join(dstDir, "a", "b", "c")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("Nested empty directories were not copied")
	}
}

func TestMigrateResult_NoErrors(t *testing.T) {
	result := &MigrateResult{
		ModulesFound:  5,
		ModulesCopied: 5,
		Errors:        nil,
	}

	if len(result.Errors) != 0 {
		t.Errorf("Errors should be empty, got %v", result.Errors)
	}
}

func TestMigrateResult_WithErrors(t *testing.T) {
	result := &MigrateResult{
		ModulesFound:   3,
		ModulesCopied:  1,
		ModulesSkipped: 0,
		Errors: []string{
			"mod1: file not found",
			"mod2: permission denied",
		},
	}

	if len(result.Errors) != 2 {
		t.Errorf("len(Errors) = %d, want 2", len(result.Errors))
	}
}

// Helper to create a module with missing data directory
func createTestModuleMissingData(t *testing.T, srcDir, modID string) {
	t.Helper()

	// Create mods.d directory and conf file
	modsDir := filepath.Join(srcDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("Failed to create mods.d: %v", err)
	}

	// Points to a data directory that doesn't exist
	confContent := "[" + modID + "]\n" +
		"Description=Test Module\n" +
		"ModDrv=zText\n" +
		"DataPath=./modules/texts/ztext/" + modID + "/\n"

	confPath := filepath.Join(modsDir, modID+".conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		t.Fatalf("Failed to create conf file: %v", err)
	}
	// Note: We intentionally don't create the data directory
}

func TestMigrator_Migrate_MissingDataDirectory(t *testing.T) {
	srcDir := t.TempDir()
	createTestModuleMissingData(t, srcDir, "MissingData")

	dstDir := t.TempDir()
	m := NewMigrator(srcDir, dstDir)

	result, err := m.Migrate(nil)
	if err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	// Module should have been found but had an error during migration
	if result.ModulesFound != 1 {
		t.Errorf("ModulesFound = %d, want 1", result.ModulesFound)
	}
	if result.ModulesCopied != 0 {
		t.Errorf("ModulesCopied = %d, want 0 (data dir missing)", result.ModulesCopied)
	}
	if len(result.Errors) != 1 {
		t.Errorf("len(Errors) = %d, want 1", len(result.Errors))
	}
}

func TestMigrator_Migrate_MixedSuccess(t *testing.T) {
	srcDir := t.TempDir()
	// Create one good module and one with missing data
	createTestModule(t, srcDir, "GoodMod")
	createTestModuleMissingData(t, srcDir, "BadMod")

	dstDir := t.TempDir()
	m := NewMigrator(srcDir, dstDir)

	result, err := m.Migrate(nil)
	if err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	if result.ModulesFound != 2 {
		t.Errorf("ModulesFound = %d, want 2", result.ModulesFound)
	}
	if result.ModulesCopied != 1 {
		t.Errorf("ModulesCopied = %d, want 1", result.ModulesCopied)
	}
	if len(result.Errors) != 1 {
		t.Errorf("len(Errors) = %d, want 1", len(result.Errors))
	}
}

func TestCopyFile_DestinationDirNotWritable(t *testing.T) {
	// Skip on CI or if not running as non-root
	if os.Getuid() == 0 {
		t.Skip("Cannot test permission errors as root")
	}

	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "source.txt")
	if err := os.WriteFile(srcFile, []byte("content"), 0600); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create a read-only directory
	readOnlyDir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(readOnlyDir, 0555); err != nil {
		t.Fatalf("Failed to create read-only dir: %v", err)
	}
	defer os.Chmod(readOnlyDir, 0755) // Restore permissions for cleanup

	// Try to copy to a path inside the read-only directory
	dstFile := filepath.Join(readOnlyDir, "subdir", "dest.txt")
	err := copyFile(srcFile, dstFile)
	if err == nil {
		t.Error("copyFile() should return error when destination directory is not writable")
	}
}

func TestCopyDir_SourceNotReadable(t *testing.T) {
	// Skip on CI or if not running as non-root
	if os.Getuid() == 0 {
		t.Skip("Cannot test permission errors as root")
	}

	srcDir := t.TempDir()
	// Create a directory with a non-readable subdirectory
	unreadableDir := filepath.Join(srcDir, "unreadable")
	if err := os.MkdirAll(unreadableDir, 0000); err != nil {
		t.Fatalf("Failed to create unreadable dir: %v", err)
	}
	defer os.Chmod(unreadableDir, 0755) // Restore permissions for cleanup

	dstDir := filepath.Join(t.TempDir(), "dest")
	err := copyDir(srcDir, dstDir)
	// Should fail because it can't read the unreadable directory
	if err == nil {
		t.Error("copyDir() should return error when source has unreadable directory")
	}
}

func TestCopyDir_CopyFileError(t *testing.T) {
	// Skip on CI or if not running as non-root
	if os.Getuid() == 0 {
		t.Skip("Cannot test permission errors as root")
	}

	srcDir := t.TempDir()
	// Create a file that can't be read
	unreadableFile := filepath.Join(srcDir, "unreadable.txt")
	if err := os.WriteFile(unreadableFile, []byte("content"), 0000); err != nil {
		t.Fatalf("Failed to create unreadable file: %v", err)
	}
	defer os.Chmod(unreadableFile, 0644) // Restore permissions for cleanup

	dstDir := filepath.Join(t.TempDir(), "dest")
	err := copyDir(srcDir, dstDir)
	// Should fail because it can't read the unreadable file
	if err == nil {
		t.Error("copyDir() should return error when source has unreadable file")
	}
}

// Helper to create a module with unreadable conf file
func createTestModuleUnreadableConf(t *testing.T, srcDir, modID string) {
	t.Helper()

	// Create mods.d directory
	modsDir := filepath.Join(srcDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("Failed to create mods.d: %v", err)
	}

	confContent := "[" + modID + "]\n" +
		"Description=Test Module\n" +
		"ModDrv=zText\n" +
		"DataPath=./modules/texts/ztext/" + modID + "/\n"

	confPath := filepath.Join(modsDir, modID+".conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		t.Fatalf("Failed to create conf file: %v", err)
	}

	// Create data directory with a test file
	dataDir := filepath.Join(srcDir, "modules", "texts", "ztext", modID)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}
	testFile := filepath.Join(dataDir, "test.bzz")
	if err := os.WriteFile(testFile, []byte("test data"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Make conf file unreadable AFTER successful parsing
	// (sword.LoadAllModules will still parse it, but copying will fail)
	// Actually, we need a different approach - make it unreadable after loading
}

func TestMigrator_Migrate_ConfCopyError(t *testing.T) {
	// Skip on CI or if not running as non-root
	if os.Getuid() == 0 {
		t.Skip("Cannot test permission errors as root")
	}

	srcDir := t.TempDir()
	createTestModule(t, srcDir, "TestMod")

	confPath := filepath.Join(srcDir, "mods.d", "TestMod.conf")

	dstDir := t.TempDir()
	m := NewMigrator(srcDir, dstDir)

	// First discover modules (this reads the conf)
	modules, err := m.ListModules()
	if err != nil {
		t.Fatalf("ListModules() failed: %v", err)
	}
	if len(modules) != 1 {
		t.Fatalf("Expected 1 module, got %d", len(modules))
	}

	// Now make it unreadable
	if err := os.Chmod(confPath, 0000); err != nil {
		t.Fatalf("Failed to make conf unreadable: %v", err)
	}
	defer os.Chmod(confPath, 0644) // Restore for cleanup

	// Migration will fail because LoadAllModules can't parse the conf anymore
	// This tests that the migrate code handles LoadAllModules errors gracefully
	// The module is effectively not found due to the unreadable conf
	result, err := m.Migrate(nil)
	if err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	// Module won't be found because conf is unreadable during discovery phase
	// This is expected behavior - unreadable conf files are skipped during LoadAllModules
	if result.ModulesFound != 0 {
		t.Errorf("ModulesFound = %d, want 0 (conf unreadable during discovery)", result.ModulesFound)
	}
}

func TestMigrator_Migrate_DataCopyError(t *testing.T) {
	// Skip on CI or if not running as non-root
	if os.Getuid() == 0 {
		t.Skip("Cannot test permission errors as root")
	}

	srcDir := t.TempDir()
	createTestModule(t, srcDir, "TestMod")

	// Make data file unreadable
	dataFile := filepath.Join(srcDir, "modules", "texts", "ztext", "TestMod", "test.bzz")
	if err := os.Chmod(dataFile, 0000); err != nil {
		t.Fatalf("Failed to make data unreadable: %v", err)
	}
	defer os.Chmod(dataFile, 0644) // Restore for cleanup

	dstDir := t.TempDir()
	m := NewMigrator(srcDir, dstDir)

	result, err := m.Migrate(nil)
	if err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	if result.ModulesCopied != 0 {
		t.Errorf("ModulesCopied = %d, want 0 (data file unreadable)", result.ModulesCopied)
	}
	if len(result.Errors) == 0 {
		t.Error("Expected error due to unreadable data file")
	}
}

func TestCopyFile_EmptyFile(t *testing.T) {
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "empty.txt")
	if err := os.WriteFile(srcFile, []byte{}, 0600); err != nil {
		t.Fatalf("Failed to create empty source file: %v", err)
	}

	dstDir := t.TempDir()
	dstFile := filepath.Join(dstDir, "dest.txt")

	err := copyFile(srcFile, dstFile)
	if err != nil {
		t.Fatalf("copyFile() returned error for empty file: %v", err)
	}

	// Verify destination exists and is empty
	info, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("Failed to stat destination: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("Destination size = %d, want 0", info.Size())
	}
}

func TestCopyFile_LargeFile(t *testing.T) {
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "large.bin")

	// Create a 1MB file
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if err := os.WriteFile(srcFile, data, 0600); err != nil {
		t.Fatalf("Failed to create large source file: %v", err)
	}

	dstDir := t.TempDir()
	dstFile := filepath.Join(dstDir, "dest.bin")

	err := copyFile(srcFile, dstFile)
	if err != nil {
		t.Fatalf("copyFile() returned error for large file: %v", err)
	}

	// Verify content matches
	dstData, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination: %v", err)
	}
	if len(dstData) != len(data) {
		t.Errorf("Destination size = %d, want %d", len(dstData), len(data))
	}
}

func TestMigrator_Migrate_FilterNonMatching(t *testing.T) {
	srcDir := t.TempDir()
	createTestModule(t, srcDir, "KJV")
	createTestModule(t, srcDir, "ESV")

	dstDir := t.TempDir()
	m := NewMigrator(srcDir, dstDir)

	// Filter for modules that don't exist
	result, err := m.Migrate([]string{"NIV", "NASB"})
	if err != nil {
		t.Fatalf("Migrate() returned error: %v", err)
	}

	if result.ModulesFound != 2 {
		t.Errorf("ModulesFound = %d, want 2", result.ModulesFound)
	}
	if result.ModulesCopied != 0 {
		t.Errorf("ModulesCopied = %d, want 0 (no filter match)", result.ModulesCopied)
	}
	if result.ModulesSkipped != 2 {
		t.Errorf("ModulesSkipped = %d, want 2", result.ModulesSkipped)
	}
}
