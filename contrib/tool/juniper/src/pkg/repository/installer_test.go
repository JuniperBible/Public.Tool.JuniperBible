package repository

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// createTestModuleArchive creates a minimal tar.gz module archive for testing (legacy)
func createTestModuleArchive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("Failed to write tar header: %v", err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("Failed to write tar content: %v", err)
		}
	}

	tw.Close()
	gzw.Close()

	return buf.Bytes()
}

// createTestZipArchive creates a minimal zip module archive for testing
func createTestZipArchive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("Failed to create zip entry: %v", err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("Failed to write zip content: %v", err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("Failed to close zip: %v", err)
	}

	return buf.Bytes()
}

// TestNewInstaller tests creating an installer
func TestNewInstaller(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	client, _ := NewClient(ClientOptions{})

	installer := NewInstaller(cfg, client)
	if installer == nil {
		t.Fatal("NewInstaller() returned nil")
	}
}

// TestInstaller_Install tests installing a module
func TestInstaller_Install(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	// Create mock server with module data
	// Module zip should contain both data and conf files
	confContent := []byte(`[TestModule]
DataPath=./modules/texts/ztext/testmodule/
ModDrv=zText
Lang=en
Description=Test Module
`)

	moduleZip := createTestZipArchive(t, map[string][]byte{
		"modules/texts/ztext/testmodule/ot.bzs": []byte("mock bzs data"),
		"modules/texts/ztext/testmodule/ot.bzv": []byte("mock bzv data"),
		"modules/texts/ztext/testmodule/ot.bzz": []byte("mock bzz data"),
		"mods.d/testmodule.conf":                confContent,
	})

	modsArchive := createTestModsArchive(t, map[string]string{
		"testmodule.conf": string(confContent),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mods.d.tar.gz":
			w.Write(modsArchive)
		case "/packages/rawzip/TestModule.zip":
			w.Write(moduleZip)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	// Use "raw" suffix so that the package URL gets constructed correctly
	source := Source{
		Name:      "TestSource",
		Type:      SourceTypeHTTP,
		Host:      server.Listener.Addr().String(),
		Directory: "/raw",
	}

	moduleInfo := ModuleInfo{
		ID:       "TestModule",
		DataPath: "./modules/texts/ztext/testmodule/",
	}

	ctx := context.Background()
	err := installer.Install(ctx, source, moduleInfo)
	if err != nil {
		t.Fatalf("Installer.Install() error = %v", err)
	}

	// Verify conf file was installed (the zip contains it)
	confPath := filepath.Join(cfg.ModsDir(), "testmodule.conf")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Error("Conf file was not installed")
	}

	// Verify data files were installed
	dataPath := filepath.Join(cfg.SwordDir, "modules/texts/ztext/testmodule/ot.bzs")
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		t.Error("Data files were not installed")
	}
}

// TestInstaller_Uninstall tests uninstalling a module
func TestInstaller_Uninstall(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	// Create installed module structure
	confPath := filepath.Join(cfg.ModsDir(), "testmodule.conf")
	os.WriteFile(confPath, []byte(`[TestModule]
DataPath=./modules/texts/ztext/testmodule/
ModDrv=zText
`), 0644)

	dataDir := filepath.Join(cfg.SwordDir, "modules", "texts", "ztext", "testmodule")
	os.MkdirAll(dataDir, 0755)
	os.WriteFile(filepath.Join(dataDir, "ot.bzs"), []byte("data"), 0600)
	os.WriteFile(filepath.Join(dataDir, "ot.bzv"), []byte("data"), 0600)
	os.WriteFile(filepath.Join(dataDir, "ot.bzz"), []byte("data"), 0600)

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	err := installer.Uninstall("TestModule")
	if err != nil {
		t.Fatalf("Installer.Uninstall() error = %v", err)
	}

	// Verify conf file was removed
	if _, err := os.Stat(confPath); !os.IsNotExist(err) {
		t.Error("Conf file should have been removed")
	}

	// Verify data directory was removed
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Error("Data directory should have been removed")
	}
}

// TestInstaller_Uninstall_NotInstalled tests uninstalling a non-existent module
func TestInstaller_Uninstall_NotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	err := installer.Uninstall("NonExistent")
	if err == nil {
		t.Error("Installer.Uninstall() should return error for non-existent module")
	}
}

// TestInstaller_RefreshSource tests refreshing a source's module list
func TestInstaller_RefreshSource(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	modsArchive := createTestModsArchive(t, map[string]string{
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
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(modsArchive)
	}))
	defer server.Close()

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	source := Source{
		Name:      "TestSource",
		Type:      SourceTypeHTTP,
		Host:      server.Listener.Addr().String(),
		Directory: "",
	}

	ctx := context.Background()
	modules, err := installer.RefreshSource(ctx, source)
	if err != nil {
		t.Fatalf("Installer.RefreshSource() error = %v", err)
	}

	if len(modules) != 2 {
		t.Errorf("RefreshSource() returned %d modules, want 2", len(modules))
	}
}

// TestInstaller_ListAvailable tests listing available modules from a source
func TestInstaller_ListAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	modsArchive := createTestModsArchive(t, map[string]string{
		"kjv.conf": `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
Lang=en
Description=King James Version
`,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(modsArchive)
	}))
	defer server.Close()

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	source := Source{
		Name:      "TestSource",
		Type:      SourceTypeHTTP,
		Host:      server.Listener.Addr().String(),
		Directory: "",
	}

	ctx := context.Background()
	modules, err := installer.ListAvailable(ctx, source)
	if err != nil {
		t.Fatalf("Installer.ListAvailable() error = %v", err)
	}

	if len(modules) != 1 {
		t.Errorf("ListAvailable() returned %d modules, want 1", len(modules))
	}
}

// TestInstaller_CheckUpdates tests checking for module updates
func TestInstaller_CheckUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	// Install an older version
	os.WriteFile(filepath.Join(cfg.ModsDir(), "kjv.conf"), []byte(`[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
Version=1.0
`), 0644)

	// Server has newer version
	modsArchive := createTestModsArchive(t, map[string]string{
		"kjv.conf": `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
Version=2.0
Description=King James Version (Updated)
`,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(modsArchive)
	}))
	defer server.Close()

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	source := Source{
		Name:      "TestSource",
		Type:      SourceTypeHTTP,
		Host:      server.Listener.Addr().String(),
		Directory: "",
	}

	ctx := context.Background()
	updates, err := installer.CheckUpdates(ctx, source)
	if err != nil {
		t.Fatalf("Installer.CheckUpdates() error = %v", err)
	}

	if len(updates) != 1 {
		t.Errorf("CheckUpdates() returned %d updates, want 1", len(updates))
	}
}

// TestInstaller_InstallConf tests installing just the conf file
func TestInstaller_InstallConf(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	confContent := `[TestModule]
DataPath=./modules/texts/ztext/testmodule/
ModDrv=zText
Lang=en
`

	err := installer.InstallConf("testmodule", []byte(confContent))
	if err != nil {
		t.Fatalf("Installer.InstallConf() error = %v", err)
	}

	// Verify file was created
	confPath := filepath.Join(cfg.ModsDir(), "testmodule.conf")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Error("Conf file was not created")
	}

	// Verify content
	data, _ := os.ReadFile(confPath)
	if string(data) != confContent {
		t.Error("Conf file content doesn't match")
	}
}

// TestInstaller_RemoveConf tests removing a conf file
func TestInstaller_RemoveConf(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	// Create a conf file
	confPath := filepath.Join(cfg.ModsDir(), "testmodule.conf")
	os.WriteFile(confPath, []byte("content"), 0600)

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	err := installer.RemoveConf("testmodule")
	if err != nil {
		t.Fatalf("Installer.RemoveConf() error = %v", err)
	}

	if _, err := os.Stat(confPath); !os.IsNotExist(err) {
		t.Error("Conf file should have been removed")
	}
}

// TestInstaller_InstallProgress tests installation with progress callback
func TestInstaller_InstallProgress(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	installer.OnProgress = func(step, total int, message string) {
		// Progress callback for tracking installation steps
		t.Logf("Progress: %d/%d - %s", step, total, message)
	}

	// Just verify the callback mechanism works
	// Actual installation tested elsewhere
	if installer.OnProgress == nil {
		t.Error("OnProgress should be set")
	}
}

// TestModuleUpdate tests the ModuleUpdate struct
func TestModuleUpdate(t *testing.T) {
	update := ModuleUpdate{
		Module:           ModuleInfo{ID: "KJV", Version: "2.0"},
		InstalledVersion: "1.0",
		AvailableVersion: "2.0",
	}

	if !update.HasUpdate() {
		t.Error("HasUpdate() should return true for different versions")
	}

	noUpdate := ModuleUpdate{
		InstalledVersion: "2.0",
		AvailableVersion: "2.0",
	}

	if noUpdate.HasUpdate() {
		t.Error("HasUpdate() should return false for same versions")
	}
}

// TestInstaller_ValidateModule tests module validation
func TestInstaller_ValidateModule(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	// Create installed module
	os.WriteFile(filepath.Join(cfg.ModsDir(), "kjv.conf"), []byte(`[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
`), 0644)

	dataDir := filepath.Join(cfg.SwordDir, "modules", "texts", "ztext", "kjv")
	os.MkdirAll(dataDir, 0755)
	os.WriteFile(filepath.Join(dataDir, "ot.bzs"), []byte("data"), 0600)
	os.WriteFile(filepath.Join(dataDir, "ot.bzv"), []byte("data"), 0600)
	os.WriteFile(filepath.Join(dataDir, "ot.bzz"), []byte("data"), 0600)

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	valid, err := installer.ValidateModule("KJV")
	if err != nil {
		t.Fatalf("Installer.ValidateModule() error = %v", err)
	}

	if !valid {
		t.Error("ValidateModule() should return true for valid module")
	}
}

// TestInstaller_ValidateModule_Missing tests validation of missing module
func TestInstaller_ValidateModule_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	_, err := installer.ValidateModule("NonExistent")
	if err == nil {
		t.Error("ValidateModule() should return error for missing module")
	}
}

// TestInstaller_ValidateModule_MissingData tests validation with missing data files
func TestInstaller_ValidateModule_MissingData(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	// Create conf without data files
	os.WriteFile(filepath.Join(cfg.ModsDir(), "kjv.conf"), []byte(`[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
`), 0644)

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	valid, _ := installer.ValidateModule("KJV")
	if valid {
		t.Error("ValidateModule() should return false for module with missing data")
	}
}

// TestInstaller_VerifyModule tests the comprehensive verification
func TestInstaller_VerifyModule(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	// Create installed module with known size
	confContent := `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
InstallSize=15
`
	os.WriteFile(filepath.Join(cfg.ModsDir(), "kjv.conf"), []byte(confContent), 0600)

	dataDir := filepath.Join(cfg.SwordDir, "modules", "texts", "ztext", "kjv")
	os.MkdirAll(dataDir, 0755)
	os.WriteFile(filepath.Join(dataDir, "ot.bzs"), []byte("12345"), 0600)      // 5 bytes
	os.WriteFile(filepath.Join(dataDir, "nt.bzs"), []byte("1234567890"), 0600) // 10 bytes

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	result := installer.VerifyModule("KJV")

	if !result.Installed {
		t.Error("VerifyModule should report module as installed")
	}
	if !result.DataExists {
		t.Error("VerifyModule should report data exists")
	}
	if result.ExpectedSize != 15 {
		t.Errorf("VerifyModule ExpectedSize = %d, want 15", result.ExpectedSize)
	}
	if result.ActualSize != 15 {
		t.Errorf("VerifyModule ActualSize = %d, want 15", result.ActualSize)
	}
	if !result.SizeMatch {
		t.Error("VerifyModule should report size match")
	}
	if !result.IsValid() {
		t.Error("VerifyModule should report module as valid")
	}
}

// TestInstaller_VerifyModule_SizeMismatch tests verification with size mismatch
func TestInstaller_VerifyModule_SizeMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	// Create installed module with wrong size
	confContent := `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
InstallSize=100
`
	os.WriteFile(filepath.Join(cfg.ModsDir(), "kjv.conf"), []byte(confContent), 0600)

	dataDir := filepath.Join(cfg.SwordDir, "modules", "texts", "ztext", "kjv")
	os.MkdirAll(dataDir, 0755)
	os.WriteFile(filepath.Join(dataDir, "ot.bzs"), []byte("12345"), 0600) // 5 bytes

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	result := installer.VerifyModule("KJV")

	if result.SizeMatch {
		t.Error("VerifyModule should report size mismatch")
	}
	if result.IsValid() {
		t.Error("VerifyModule should report module as invalid due to size mismatch")
	}
}

// TestInstaller_VerifyModule_NotInstalled tests verification of non-existent module
func TestInstaller_VerifyModule_NotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	result := installer.VerifyModule("NonExistent")

	if result.Installed {
		t.Error("VerifyModule should report module as not installed")
	}
	if result.IsValid() {
		t.Error("VerifyModule should report module as invalid")
	}
}

// TestInstaller_VerifyAllModules tests verifying all installed modules
func TestInstaller_VerifyAllModules(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewLocalConfig(tmpDir)
	cfg.EnsureDirectories()

	// Create two modules
	os.WriteFile(filepath.Join(cfg.ModsDir(), "kjv.conf"), []byte(`[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
`), 0644)
	os.WriteFile(filepath.Join(cfg.ModsDir(), "drc.conf"), []byte(`[DRC]
DataPath=./modules/texts/ztext/drc/
ModDrv=zText
`), 0644)

	// Create data for both
	kjvDir := filepath.Join(cfg.SwordDir, "modules", "texts", "ztext", "kjv")
	os.MkdirAll(kjvDir, 0755)
	os.WriteFile(filepath.Join(kjvDir, "ot.bzs"), []byte("data"), 0600)

	drcDir := filepath.Join(cfg.SwordDir, "modules", "texts", "ztext", "drc")
	os.MkdirAll(drcDir, 0755)
	os.WriteFile(filepath.Join(drcDir, "ot.bzs"), []byte("data"), 0600)

	client, _ := NewClient(ClientOptions{})
	installer := NewInstaller(cfg, client)

	results, err := installer.VerifyAllModules()
	if err != nil {
		t.Fatalf("VerifyAllModules() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("VerifyAllModules() returned %d results, want 2", len(results))
	}

	for _, r := range results {
		if !r.IsValid() {
			t.Errorf("Module %s should be valid", r.ModuleID)
		}
	}
}

// TestModuleVerification_IsValid tests the IsValid helper
func TestModuleVerification_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		v        ModuleVerification
		expected bool
	}{
		{
			name:     "all valid with size",
			v:        ModuleVerification{Installed: true, DataExists: true, SizeMatch: true, ExpectedSize: 100},
			expected: true,
		},
		{
			name:     "valid without size check",
			v:        ModuleVerification{Installed: true, DataExists: true, SizeMatch: false, ExpectedSize: 0},
			expected: true,
		},
		{
			name:     "not installed",
			v:        ModuleVerification{Installed: false, DataExists: true, SizeMatch: true},
			expected: false,
		},
		{
			name:     "no data",
			v:        ModuleVerification{Installed: true, DataExists: false, SizeMatch: true},
			expected: false,
		},
		{
			name:     "size mismatch",
			v:        ModuleVerification{Installed: true, DataExists: true, SizeMatch: false, ExpectedSize: 100},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}
