package repository

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultSwordDir tests getting the default SWORD directory
func TestDefaultSwordDir(t *testing.T) {
	dir := DefaultSwordDir()

	if dir == "" {
		t.Error("DefaultSwordDir() returned empty string")
	}

	// Should end with .sword
	if filepath.Base(dir) != ".sword" {
		t.Errorf("DefaultSwordDir() = %q, expected to end with .sword", dir)
	}
}

// TestLocalConfig_Load tests loading local configuration
func TestLocalConfig_Load(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock .sword directory structure
	modsDir := filepath.Join(tmpDir, "mods.d")
	modulesDir := filepath.Join(tmpDir, "modules")
	os.MkdirAll(modsDir, 0755)
	os.MkdirAll(modulesDir, 0755)

	// Create a sample conf file
	confContent := `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
Lang=en
Description=King James Version
`
	os.WriteFile(filepath.Join(modsDir, "kjv.conf"), []byte(confContent), 0600)

	cfg, err := LoadLocalConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	if cfg.SwordDir != tmpDir {
		t.Errorf("cfg.SwordDir = %q, want %q", cfg.SwordDir, tmpDir)
	}
}

// TestLocalConfig_Load_NonExistent tests loading from non-existent directory
func TestLocalConfig_Load_NonExistent(t *testing.T) {
	_, err := LoadLocalConfig("/nonexistent/path/.sword")
	if err == nil {
		t.Error("LoadLocalConfig() should return error for non-existent directory")
	}
}

// TestLocalConfig_ListInstalledModules tests listing installed modules
func TestLocalConfig_ListInstalledModules(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Create multiple conf files
	confs := map[string]string{
		"kjv.conf": `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
Lang=en
Description=King James Version
`,
		"drc.conf": `[DRC]
DataPath=./modules/texts/ztext/drc/
ModDrv=zText
Lang=en
Description=Douay-Rheims
`,
	}

	for name, content := range confs {
		os.WriteFile(filepath.Join(modsDir, name), []byte(content), 0600)
	}

	cfg, err := LoadLocalConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	modules, err := cfg.ListInstalledModules()
	if err != nil {
		t.Fatalf("ListInstalledModules() error = %v", err)
	}

	if len(modules) != 2 {
		t.Errorf("ListInstalledModules() returned %d modules, want 2", len(modules))
	}
}

// TestLocalConfig_GetInstalledModule tests getting a specific installed module
func TestLocalConfig_GetInstalledModule(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	confContent := `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
Lang=en
Description=King James Version
Version=2.5
`
	os.WriteFile(filepath.Join(modsDir, "kjv.conf"), []byte(confContent), 0600)

	cfg, err := LoadLocalConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	module, found := cfg.GetInstalledModule("KJV")
	if !found {
		t.Fatal("GetInstalledModule(KJV) should find module")
	}

	if module.Version != "2.5" {
		t.Errorf("module.Version = %q, want %q", module.Version, "2.5")
	}
}

// TestLocalConfig_GetInstalledModule_NotFound tests getting non-existent module
func TestLocalConfig_GetInstalledModule_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	cfg, err := LoadLocalConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	_, found := cfg.GetInstalledModule("NonExistent")
	if found {
		t.Error("GetInstalledModule(NonExistent) should not find module")
	}
}

// TestLocalConfig_IsModuleInstalled tests checking if a module is installed
func TestLocalConfig_IsModuleInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	confContent := `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
`
	os.WriteFile(filepath.Join(modsDir, "kjv.conf"), []byte(confContent), 0600)

	cfg, err := LoadLocalConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	if !cfg.IsModuleInstalled("KJV") {
		t.Error("IsModuleInstalled(KJV) should return true")
	}

	if cfg.IsModuleInstalled("NonExistent") {
		t.Error("IsModuleInstalled(NonExistent) should return false")
	}
}

// TestLocalConfig_ModsDir tests getting the mods.d directory path
func TestLocalConfig_ModsDir(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	cfg, err := LoadLocalConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "mods.d")
	if cfg.ModsDir() != expected {
		t.Errorf("ModsDir() = %q, want %q", cfg.ModsDir(), expected)
	}
}

// TestLocalConfig_ModulesDir tests getting the modules directory path
func TestLocalConfig_ModulesDir(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	cfg, err := LoadLocalConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "modules")
	if cfg.ModulesDir() != expected {
		t.Errorf("ModulesDir() = %q, want %q", cfg.ModulesDir(), expected)
	}
}

// TestLocalConfig_EnsureDirectories tests creating required directories
func TestLocalConfig_EnsureDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	swordDir := filepath.Join(tmpDir, ".sword")

	cfg := &LocalConfig{SwordDir: swordDir}

	err := cfg.EnsureDirectories()
	if err != nil {
		t.Fatalf("EnsureDirectories() error = %v", err)
	}

	// Check directories were created
	modsDir := filepath.Join(swordDir, "mods.d")
	if _, err := os.Stat(modsDir); os.IsNotExist(err) {
		t.Error("mods.d directory was not created")
	}

	modulesDir := filepath.Join(swordDir, "modules")
	if _, err := os.Stat(modulesDir); os.IsNotExist(err) {
		t.Error("modules directory was not created")
	}
}

// TestLocalConfig_SaveInstallConf tests saving install.conf
func TestLocalConfig_SaveInstallConf(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	cfg, err := LoadLocalConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	sources := []Source{
		{Name: "CrossWire", Type: SourceTypeFTP, Host: "ftp.crosswire.org", Directory: "/pub/sword/raw"},
	}

	err = cfg.SaveInstallConf(sources)
	if err != nil {
		t.Fatalf("SaveInstallConf() error = %v", err)
	}

	// Verify file was created
	confPath := filepath.Join(tmpDir, "mods.d", "install.conf")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Error("install.conf was not created")
	}
}

// TestLocalConfig_LoadInstallConf tests loading install.conf
func TestLocalConfig_LoadInstallConf(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Create install.conf
	confContent := `[General]

[CrossWire]
FTPSource=ftp.crosswire.org|/pub/sword/raw|CrossWire

[eBible]
FTPSource=ftp.ebible.org|/sword|eBible.org
`
	os.WriteFile(filepath.Join(modsDir, "install.conf"), []byte(confContent), 0600)

	cfg, err := LoadLocalConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	sources, err := cfg.LoadInstallConf()
	if err != nil {
		t.Fatalf("LoadInstallConf() error = %v", err)
	}

	if len(sources) != 2 {
		t.Errorf("LoadInstallConf() returned %d sources, want 2", len(sources))
	}
}

// TestLocalConfig_LoadInstallConf_NoFile tests loading when install.conf doesn't exist
func TestLocalConfig_LoadInstallConf_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	cfg, err := LoadLocalConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	sources, err := cfg.LoadInstallConf()
	if err != nil {
		t.Fatalf("LoadInstallConf() should not error for missing file, got %v", err)
	}

	// Should return default sources when no install.conf
	if len(sources) == 0 {
		t.Log("LoadInstallConf() returned no sources (expected if using defaults)")
	}
}

// TestLocalConfig_GetModuleDataPath tests getting the full data path for a module
func TestLocalConfig_GetModuleDataPath(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	cfg, err := LoadLocalConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	// Test with relative path starting with ./
	dataPath := cfg.GetModuleDataPath("./modules/texts/ztext/kjv/")
	expected := filepath.Join(tmpDir, "modules", "texts", "ztext", "kjv")
	if dataPath != expected {
		t.Errorf("GetModuleDataPath() = %q, want %q", dataPath, expected)
	}
}

// TestNewLocalConfig tests creating a new LocalConfig
func TestNewLocalConfig(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := NewLocalConfig(tmpDir)

	if cfg.SwordDir != tmpDir {
		t.Errorf("cfg.SwordDir = %q, want %q", cfg.SwordDir, tmpDir)
	}
}

// TestLocalConfig_ListInstalledModules_Empty tests listing with no modules
func TestLocalConfig_ListInstalledModules_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	cfg, err := LoadLocalConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	modules, err := cfg.ListInstalledModules()
	if err != nil {
		t.Fatalf("ListInstalledModules() error = %v", err)
	}

	if len(modules) != 0 {
		t.Errorf("ListInstalledModules() returned %d modules, want 0", len(modules))
	}
}

// TestLocalConfig_ListInstalledModules_SkipsInvalid tests skipping invalid conf files
func TestLocalConfig_ListInstalledModules_SkipsInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Create valid and invalid conf files
	os.WriteFile(filepath.Join(modsDir, "kjv.conf"), []byte(`[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
`), 0644)
	os.WriteFile(filepath.Join(modsDir, "invalid.conf"), []byte("invalid content"), 0600)
	os.WriteFile(filepath.Join(modsDir, "empty.conf"), []byte(""), 0600)

	cfg, err := LoadLocalConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	modules, err := cfg.ListInstalledModules()
	if err != nil {
		t.Fatalf("ListInstalledModules() error = %v", err)
	}

	// Should only find the valid KJV module
	if len(modules) != 1 {
		t.Errorf("ListInstalledModules() returned %d modules, want 1", len(modules))
	}
}
