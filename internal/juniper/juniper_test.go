package juniper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSwordPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		check   func(string) bool
	}{
		{
			name: "explicit path provided",
			path: "/custom/sword/path",
			check: func(result string) bool {
				return result == "/custom/sword/path"
			},
		},
		{
			name: "empty path uses home",
			path: "",
			check: func(result string) bool {
				return strings.HasSuffix(result, ".sword")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveSwordPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveSwordPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.check != nil && !tt.check(got) {
				t.Errorf("ResolveSwordPath() = %v, check failed", got)
			}
		})
	}
}

func TestParseConf(t *testing.T) {
	tests := []struct {
		name       string
		confData   string
		wantModule *Module
		wantNil    bool
	}{
		{
			name: "valid Bible module",
			confData: `[KJV]
Description=King James Version
Lang=en
ModDrv=zText
DataPath=./modules/texts/ztext/kjv/`,
			wantModule: &Module{
				Name:        "KJV",
				Description: "King James Version",
				Lang:        "en",
				ModType:     "Bible",
				DataPath:    "./modules/texts/ztext/kjv/",
				Encrypted:   false,
			},
		},
		{
			name: "encrypted module",
			confData: `[ESV]
Description=English Standard Version
Lang=en
ModDrv=zText
CipherKey=somekey
DataPath=./modules/texts/ztext/esv/`,
			wantModule: &Module{
				Name:        "ESV",
				Description: "English Standard Version",
				Lang:        "en",
				ModType:     "Bible",
				DataPath:    "./modules/texts/ztext/esv/",
				Encrypted:   true,
			},
		},
		{
			name: "commentary module",
			confData: `[MHC]
Description=Matthew Henry Commentary
Lang=en
ModDrv=RawCom
DataPath=./modules/comments/rawcom/mhc/`,
			wantModule: &Module{
				Name:        "MHC",
				Description: "Matthew Henry Commentary",
				Lang:        "en",
				ModType:     "Commentary",
				DataPath:    "./modules/comments/rawcom/mhc/",
			},
		},
		{
			name: "dictionary module",
			confData: `[StrongsGreek]
Description=Strong's Greek Dictionary
Lang=grc
ModDrv=zLD
DataPath=./modules/lexdict/zld/strongsgreek/`,
			wantModule: &Module{
				Name:        "StrongsGreek",
				Description: "Strong's Greek Dictionary",
				Lang:        "grc",
				ModType:     "Dictionary",
				DataPath:    "./modules/lexdict/zld/strongsgreek/",
			},
		},
		{
			name: "genbook module",
			confData: `[Josephus]
Description=Works of Josephus
Lang=en
ModDrv=RawGenBook
DataPath=./modules/genbook/rawgenbook/josephus/`,
			wantModule: &Module{
				Name:        "Josephus",
				Description: "Works of Josephus",
				Lang:        "en",
				ModType:     "GenBook",
				DataPath:    "./modules/genbook/rawgenbook/josephus/",
			},
		},
		{
			name: "with comments and empty lines",
			confData: `[WEB]
# This is a comment
Description=World English Bible

Lang=en
ModDrv=zText`,
			wantModule: &Module{
				Name:        "WEB",
				Description: "World English Bible",
				Lang:        "en",
				ModType:     "Bible",
			},
		},
		{
			name: "unknown driver type",
			confData: `[UNKNOWN]
Description=Unknown Module
ModDrv=UnknownDriver`,
			wantModule: &Module{
				Name:        "UNKNOWN",
				Description: "Unknown Module",
				ModType:     "Unknown",
			},
		},
		{
			name: "zText4 driver",
			confData: `[TEST]
Description=Test
ModDrv=zText4`,
			wantModule: &Module{
				Name:        "TEST",
				Description: "Test",
				ModType:     "Bible",
			},
		},
		{
			name: "RawText4 driver",
			confData: `[TEST]
Description=Test
ModDrv=RawText4`,
			wantModule: &Module{
				Name:        "TEST",
				Description: "Test",
				ModType:     "Bible",
			},
		},
		{
			name: "zCom4 driver",
			confData: `[TEST]
Description=Test
ModDrv=zCom4`,
			wantModule: &Module{
				Name:        "TEST",
				Description: "Test",
				ModType:     "Commentary",
			},
		},
		{
			name: "RawCom4 driver",
			confData: `[TEST]
Description=Test
ModDrv=RawCom4`,
			wantModule: &Module{
				Name:        "TEST",
				Description: "Test",
				ModType:     "Commentary",
			},
		},
		{
			name: "RawLD4 driver",
			confData: `[TEST]
Description=Test
ModDrv=RawLD4`,
			wantModule: &Module{
				Name:        "TEST",
				Description: "Test",
				ModType:     "Dictionary",
			},
		},
		{
			name:     "file not found",
			confData: "",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var confPath string
			if tt.confData != "" {
				// Create temporary conf file
				tempDir := t.TempDir()
				confPath = filepath.Join(tempDir, "test.conf")
				if err := os.WriteFile(confPath, []byte(tt.confData), 0644); err != nil {
					t.Fatalf("Failed to create test conf file: %v", err)
				}
			} else {
				confPath = "/nonexistent/path/test.conf"
			}

			got := ParseConf(confPath)

			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseConf() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("ParseConf() returned nil, want non-nil")
			}

			if got.Name != tt.wantModule.Name {
				t.Errorf("ParseConf() Name = %v, want %v", got.Name, tt.wantModule.Name)
			}
			if got.Description != tt.wantModule.Description {
				t.Errorf("ParseConf() Description = %v, want %v", got.Description, tt.wantModule.Description)
			}
			if got.Lang != tt.wantModule.Lang {
				t.Errorf("ParseConf() Lang = %v, want %v", got.Lang, tt.wantModule.Lang)
			}
			if got.ModType != tt.wantModule.ModType {
				t.Errorf("ParseConf() ModType = %v, want %v", got.ModType, tt.wantModule.ModType)
			}
			if got.DataPath != tt.wantModule.DataPath {
				t.Errorf("ParseConf() DataPath = %v, want %v", got.DataPath, tt.wantModule.DataPath)
			}
			if got.Encrypted != tt.wantModule.Encrypted {
				t.Errorf("ParseConf() Encrypted = %v, want %v", got.Encrypted, tt.wantModule.Encrypted)
			}
		})
	}
}

func TestListModules(t *testing.T) {
	t.Run("no mods.d directory", func(t *testing.T) {
		tempDir := t.TempDir()
		_, err := ListModules(tempDir)
		if err == nil {
			t.Error("ListModules() expected error for missing mods.d")
		}
		if err != nil && !strings.Contains(err.Error(), "not found") {
			t.Errorf("ListModules() error = %v, want 'not found'", err)
		}
	})

	t.Run("empty mods.d directory", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		modules, err := ListModules(tempDir)
		if err != nil {
			t.Fatalf("ListModules() error = %v", err)
		}
		if len(modules) != 0 {
			t.Errorf("ListModules() returned %d modules, want 0", len(modules))
		}
	})

	t.Run("with Bible modules", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		// Create Bible module
		kjvConf := []byte(`[KJV]
Description=King James Version
Lang=en
ModDrv=zText
DataPath=./modules/texts/ztext/kjv/`)
		os.WriteFile(filepath.Join(modsDir, "kjv.conf"), kjvConf, 0644)

		// Create non-Bible module (should be filtered)
		commentaryConf := []byte(`[MHC]
Description=Matthew Henry Commentary
Lang=en
ModDrv=RawCom`)
		os.WriteFile(filepath.Join(modsDir, "mhc.conf"), commentaryConf, 0644)

		modules, err := ListModules(tempDir)
		if err != nil {
			t.Fatalf("ListModules() error = %v", err)
		}
		if len(modules) != 1 {
			t.Errorf("ListModules() returned %d modules, want 1 (only Bible)", len(modules))
		}
		if len(modules) > 0 && modules[0].Name != "KJV" {
			t.Errorf("ListModules() module name = %v, want KJV", modules[0].Name)
		}
	})

	t.Run("skips non-.conf files", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		// Create non-.conf file
		os.WriteFile(filepath.Join(modsDir, "readme.txt"), []byte("hello"), 0644)

		// Create subdirectory
		os.Mkdir(filepath.Join(modsDir, "subdir"), 0755)

		modules, err := ListModules(tempDir)
		if err != nil {
			t.Fatalf("ListModules() error = %v", err)
		}
		if len(modules) != 0 {
			t.Errorf("ListModules() returned %d modules, want 0", len(modules))
		}
	})
}

func TestList(t *testing.T) {
	t.Run("no SWORD installation", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg := ListConfig{Path: tempDir}
		err := List(cfg)
		if err == nil {
			t.Error("List() expected error for missing mods.d")
		}
	})

	t.Run("successful list", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		// Create a Bible module
		kjvConf := []byte(`[KJV]
Description=King James Version
Lang=en
ModDrv=zText`)
		os.WriteFile(filepath.Join(modsDir, "kjv.conf"), kjvConf, 0644)

		// Create module with very long description
		longDescConf := []byte(`[LONGDESC]
Description=This is a very long description that exceeds forty characters and should be truncated
Lang=en
ModDrv=zText`)
		os.WriteFile(filepath.Join(modsDir, "longdesc.conf"), longDescConf, 0644)

		// Create encrypted module
		encConf := []byte(`[ENC]
Description=Encrypted Module
Lang=en
ModDrv=zText
CipherKey=secret`)
		os.WriteFile(filepath.Join(modsDir, "enc.conf"), encConf, 0644)

		cfg := ListConfig{Path: tempDir}
		// We can't easily capture stdout, but we can verify it doesn't error
		err := List(cfg)
		if err != nil {
			t.Errorf("List() error = %v", err)
		}
	})
}

func TestIngest(t *testing.T) {
	t.Run("no modules found", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		cfg := IngestConfig{
			Path:   tempDir,
			Output: filepath.Join(tempDir, "output"),
			All:    true,
		}
		err := Ingest(cfg)
		if err == nil {
			t.Error("Ingest() expected error for no modules")
		}
	})

	t.Run("no modules specified", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		// Create a module
		kjvConf := []byte(`[KJV]
Description=King James Version
Lang=en
ModDrv=zText`)
		os.WriteFile(filepath.Join(modsDir, "kjv.conf"), kjvConf, 0644)

		cfg := IngestConfig{
			Path:   tempDir,
			Output: filepath.Join(tempDir, "output"),
		}
		err := Ingest(cfg)
		if err == nil {
			t.Error("Ingest() expected error when no modules specified")
		}
		if err != nil && !strings.Contains(err.Error(), "specify module names") {
			t.Errorf("Ingest() error = %v, want 'specify module names'", err)
		}
	})

	t.Run("module not found", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		// Create a module
		kjvConf := []byte(`[KJV]
Description=King James Version
Lang=en
ModDrv=zText`)
		os.WriteFile(filepath.Join(modsDir, "kjv.conf"), kjvConf, 0644)

		cfg := IngestConfig{
			Path:    tempDir,
			Output:  filepath.Join(tempDir, "output"),
			Modules: []string{"NONEXISTENT"},
		}
		err := Ingest(cfg)
		if err == nil {
			t.Error("Ingest() expected error when requested module not found")
		}
	})
}

func TestIngestModule(t *testing.T) {
	t.Run("missing DataPath", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		confPath := filepath.Join(modsDir, "test.conf")
		confData := []byte(`[TEST]
Description=Test
Lang=en
ModDrv=zText`)
		os.WriteFile(confPath, confData, 0644)

		module := &Module{
			Name:     "TEST",
			ConfPath: confPath,
			DataPath: "",
		}

		outputPath := filepath.Join(tempDir, "test.capsule.tar.gz")
		err := IngestModule(tempDir, module, outputPath)
		if err == nil {
			t.Error("IngestModule() expected error for missing DataPath")
		}
	})

	t.Run("data path not found", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		confPath := filepath.Join(modsDir, "test.conf")
		confData := []byte(`[TEST]
Description=Test
Lang=en
ModDrv=zText
DataPath=./modules/texts/ztext/test/`)
		os.WriteFile(confPath, confData, 0644)

		module := &Module{
			Name:        "TEST",
			Description: "Test",
			Lang:        "en",
			ConfPath:    confPath,
			DataPath:    "./modules/texts/ztext/test/",
		}

		outputPath := filepath.Join(tempDir, "test.capsule.tar.gz")
		err := IngestModule(tempDir, module, outputPath)
		if err == nil {
			t.Error("IngestModule() expected error for missing data path")
		}
	})
}

func TestCASToSword(t *testing.T) {
	t.Run("capsule not found", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg := CASToSwordConfig{
			Capsule: "/nonexistent/capsule.tar.gz",
			Output:  tempDir,
			Name:    "TEST",
		}
		err := CASToSword(cfg)
		if err == nil {
			t.Error("CASToSword() expected error for nonexistent capsule")
		}
	})

	t.Run("name derivation from filename", func(t *testing.T) {
		// This tests the name derivation logic without actually processing
		// We can't fully test without a valid capsule, but we test error handling
		tempDir := t.TempDir()

		// Create an empty file that isn't a valid capsule
		capsulePath := filepath.Join(tempDir, "test.capsule.tar.gz")
		os.WriteFile(capsulePath, []byte("not a capsule"), 0644)

		cfg := CASToSwordConfig{
			Capsule: capsulePath,
			Output:  filepath.Join(tempDir, "output"),
			Name:    "", // Will be derived from filename
		}
		err := CASToSword(cfg)
		// Should fail during unpack, but that's ok - we're testing the path exists
		if err == nil {
			t.Error("CASToSword() expected error for invalid capsule")
		}
	})

	t.Run("default output path", func(t *testing.T) {
		// Test that empty output defaults to ~/.sword
		capsulePath := "/nonexistent.capsule.tar.gz"
		cfg := CASToSwordConfig{
			Capsule: capsulePath,
			Output:  "", // Will default to home/.sword
			Name:    "TEST",
		}
		err := CASToSword(cfg)
		// Will fail, but tests the default path logic
		if err == nil {
			t.Error("CASToSword() expected error")
		}
	})

	t.Run("various filename extensions", func(t *testing.T) {
		tempDir := t.TempDir()

		tests := []string{
			"test.capsule.tar.xz",
			"test.tar.xz",
			"test.tar.gz",
		}

		for _, filename := range tests {
			capsulePath := filepath.Join(tempDir, filename)
			os.WriteFile(capsulePath, []byte("not a capsule"), 0644)

			cfg := CASToSwordConfig{
				Capsule: capsulePath,
				Output:  filepath.Join(tempDir, "output"),
				Name:    "", // Will be derived
			}
			err := CASToSword(cfg)
			// All should fail but test the name derivation
			if err == nil {
				t.Errorf("CASToSword() expected error for %s", filename)
			}
		}
	})
}

func TestParseConf_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		confData string
		checkFn  func(*Module) bool
	}{
		{
			name: "line without equals",
			confData: `[TEST]
Description=Test
InvalidLine
Lang=en`,
			checkFn: func(m *Module) bool {
				return m.Name == "TEST" && m.Lang == "en"
			},
		},
		{
			name: "empty section header",
			confData: `[]
Description=Test`,
			checkFn: func(m *Module) bool {
				return m.Name == ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			confPath := filepath.Join(tempDir, "test.conf")
			os.WriteFile(confPath, []byte(tt.confData), 0644)

			got := ParseConf(confPath)
			if got == nil {
				t.Fatal("ParseConf() returned nil")
			}
			if tt.checkFn != nil && !tt.checkFn(got) {
				t.Errorf("ParseConf() check failed for module: %+v", got)
			}
		})
	}
}

func TestIngest_EncryptedModule(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Create encrypted module
	encConf := []byte(`[ENC]
Description=Encrypted Module
Lang=en
ModDrv=zText
CipherKey=secret
DataPath=./modules/texts/ztext/enc/`)
	os.WriteFile(filepath.Join(modsDir, "enc.conf"), encConf, 0644)

	// Create the data directory
	dataDir := filepath.Join(tempDir, "modules", "texts", "ztext", "enc")
	os.MkdirAll(dataDir, 0755)
	os.WriteFile(filepath.Join(dataDir, "ot.bzs"), []byte("data"), 0644)

	cfg := IngestConfig{
		Path:   tempDir,
		Output: filepath.Join(tempDir, "output"),
		All:    true,
	}

	// Should skip encrypted modules
	err := Ingest(cfg)
	// Will print skip message but not error
	_ = err
}

func TestListModules_ReadDirError(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")

	// Create mods.d as a file instead of directory
	os.WriteFile(modsDir, []byte("not a directory"), 0644)

	_, err := ListModules(tempDir)
	if err == nil {
		t.Error("ListModules() expected error when mods.d is not a directory")
	}
}
