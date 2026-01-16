package repoman

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSource_ModsIndexURL(t *testing.T) {
	tests := []struct {
		name      string
		source    Source
		want      string
	}{
		{
			name: "CrossWire source",
			source: Source{
				URL:       "https://www.crosswire.org/ftpmirror",
				Directory: "/pub/sword/raw",
			},
			want: "https://www.crosswire.org/ftpmirror/pub/sword/raw/mods.d.tar.gz",
		},
		{
			name: "Directory with trailing slash",
			source: Source{
				URL:       "https://example.com",
				Directory: "/test/",
			},
			want: "https://example.com/test/mods.d.tar.gz",
		},
		{
			name: "Directory without trailing slash",
			source: Source{
				URL:       "https://example.com",
				Directory: "/test",
			},
			want: "https://example.com/test/mods.d.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.source.ModsIndexURL()
			if got != tt.want {
				t.Errorf("ModsIndexURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSource_ModulePackageURLs(t *testing.T) {
	tests := []struct {
		name      string
		source    Source
		moduleID  string
		wantCount int
		wantFirst string
	}{
		{
			name: "CrossWire raw directory",
			source: Source{
				URL:       "https://www.crosswire.org/ftpmirror",
				Directory: "/pub/sword/raw",
			},
			moduleID:  "KJV",
			wantCount: 3,
			wantFirst: "https://www.crosswire.org/ftpmirror/pub/sword/packages/rawzip/KJV.zip",
		},
		{
			name: "eBible directory",
			source: Source{
				URL:       "https://ebible.org",
				Directory: "/sword",
			},
			moduleID:  "WEB",
			wantCount: 2,
			wantFirst: "https://ebible.org/sword/zip/WEB.zip",
		},
		{
			name: "IBT raw directory",
			source: Source{
				URL:       "https://ibt.org",
				Directory: "/raw",
			},
			moduleID:  "TEST",
			wantCount: 3,
			wantFirst: "https://ibt.org/packages/rawzip/TEST.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.source.ModulePackageURLs(tt.moduleID)
			if len(got) != tt.wantCount {
				t.Errorf("ModulePackageURLs() returned %d URLs, want %d", len(got), tt.wantCount)
			}
			if len(got) > 0 && got[0] != tt.wantFirst {
				t.Errorf("ModulePackageURLs() first URL = %v, want %v", got[0], tt.wantFirst)
			}
		})
	}
}

func TestHTTPError_Error(t *testing.T) {
	err := &HTTPError{
		StatusCode: 404,
		Status:     "404 Not Found",
	}
	want := "HTTP error: 404 Not Found"
	if got := err.Error(); got != want {
		t.Errorf("HTTPError.Error() = %v, want %v", got, want)
	}
}

func TestHTTPError_IsNotFound(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"404 is not found", 404, true},
		{"403 is not not found", 403, false},
		{"500 is not not found", 500, false},
		{"200 is not not found", 200, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &HTTPError{StatusCode: tt.statusCode}
			if got := err.IsNotFound(); got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.httpClient == nil {
		t.Error("NewClient() httpClient is nil")
	}
	if client.userAgent != "capsule-repoman/1.0" {
		t.Errorf("NewClient() userAgent = %v, want capsule-repoman/1.0", client.userAgent)
	}
}

func TestClient_Download(t *testing.T) {
	tests := []struct {
		name       string
		setupURL   func() string
		wantErr    bool
		wantErrMsg string
		checkData  func([]byte) error
	}{
		{
			name: "successful download",
			setupURL: func() string {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("test data"))
				}))
				t.Cleanup(server.Close)
				return server.URL
			},
			wantErr: false,
			checkData: func(data []byte) error {
				if string(data) != "test data" {
					return fmt.Errorf("got %q, want %q", string(data), "test data")
				}
				return nil
			},
		},
		{
			name: "404 error",
			setupURL: func() string {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
				t.Cleanup(server.Close)
				return server.URL
			},
			wantErr:    true,
			wantErrMsg: "HTTP error",
		},
		{
			name: "empty URL",
			setupURL: func() string {
				return ""
			},
			wantErr:    true,
			wantErrMsg: "empty URL",
		},
		{
			name: "unsupported scheme",
			setupURL: func() string {
				return "ftp://example.com/file"
			},
			wantErr:    true,
			wantErrMsg: "unsupported URL scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient()
			ctx := context.Background()
			url := tt.setupURL()

			data, err := client.Download(ctx, url)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Download() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("Download() error = %v, want error containing %v", err, tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("Download() unexpected error = %v", err)
				return
			}

			if tt.checkData != nil {
				if err := tt.checkData(data); err != nil {
					t.Errorf("Download() data check failed: %v", err)
				}
			}
		})
	}
}

func TestDefaultSources(t *testing.T) {
	sources := DefaultSources()
	if len(sources) != 2 {
		t.Errorf("DefaultSources() returned %d sources, want 2", len(sources))
	}

	// Check CrossWire source
	found := false
	for _, s := range sources {
		if s.Name == "CrossWire" {
			found = true
			if s.URL != "https://www.crosswire.org/ftpmirror" {
				t.Errorf("CrossWire URL = %v, want https://www.crosswire.org/ftpmirror", s.URL)
			}
			if s.Directory != "/pub/sword/raw" {
				t.Errorf("CrossWire Directory = %v, want /pub/sword/raw", s.Directory)
			}
		}
	}
	if !found {
		t.Error("CrossWire source not found in DefaultSources()")
	}
}

func TestGetSource(t *testing.T) {
	tests := []struct {
		name       string
		sourceName string
		wantFound  bool
		wantURL    string
	}{
		{
			name:       "CrossWire exists",
			sourceName: "CrossWire",
			wantFound:  true,
			wantURL:    "https://www.crosswire.org/ftpmirror",
		},
		{
			name:       "eBible exists",
			sourceName: "eBible",
			wantFound:  true,
			wantURL:    "https://ebible.org",
		},
		{
			name:       "Unknown source",
			sourceName: "Unknown",
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, found := GetSource(tt.sourceName)
			if found != tt.wantFound {
				t.Errorf("GetSource() found = %v, want %v", found, tt.wantFound)
			}
			if found && source.URL != tt.wantURL {
				t.Errorf("GetSource() URL = %v, want %v", source.URL, tt.wantURL)
			}
		})
	}
}

func TestListSources(t *testing.T) {
	sources := ListSources()
	if len(sources) == 0 {
		t.Error("ListSources() returned empty list")
	}
}

func TestRefreshSource(t *testing.T) {
	tests := []struct {
		name       string
		sourceName string
		setupMock  func() *httptest.Server
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "unknown source",
			sourceName: "Unknown",
			setupMock:  func() *httptest.Server { return nil },
			wantErr:    true,
			wantErrMsg: "unknown source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupMock != nil {
				server := tt.setupMock()
				if server != nil {
					t.Cleanup(server.Close)
				}
			}

			err := RefreshSource(tt.sourceName)

			if tt.wantErr {
				if err == nil {
					t.Errorf("RefreshSource() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("RefreshSource() error = %v, want error containing %v", err, tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("RefreshSource() unexpected error = %v", err)
			}
		})
	}
}

func TestParseModuleConf(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    ModuleInfo
		wantErr bool
	}{
		{
			name: "valid conf",
			data: []byte(`[KJV]
Description=King James Version
Lang=en
Version=1.0
ModDrv=zText
DataPath=./modules/texts/ztext/kjv/`),
			want: ModuleInfo{
				Name:        "KJV",
				Description: "King James Version",
				Language:    "en",
				Version:     "1.0",
				Type:        "Bible",
				DataPath:    "./modules/texts/ztext/kjv/",
			},
			wantErr: false,
		},
		{
			name:    "empty conf",
			data:    []byte(""),
			wantErr: true,
		},
		{
			name:    "no section header",
			data:    []byte("Description=Test\nLang=en"),
			wantErr: true,
		},
		{
			name: "commentary module",
			data: []byte(`[MHC]
Description=Matthew Henry Commentary
ModDrv=RawCom
Lang=en`),
			want: ModuleInfo{
				Name:        "MHC",
				Description: "Matthew Henry Commentary",
				Language:    "en",
				Type:        "Commentary",
			},
			wantErr: false,
		},
		{
			name: "dictionary module",
			data: []byte(`[StrongsGreek]
Description=Strong's Greek Dictionary
ModDrv=zLD
Lang=grc`),
			want: ModuleInfo{
				Name:        "StrongsGreek",
				Description: "Strong's Greek Dictionary",
				Language:    "grc",
				Type:        "Dictionary",
			},
			wantErr: false,
		},
		{
			name: "genbook module",
			data: []byte(`[Book]
Description=Generic Book
ModDrv=RawGenBook
Lang=en`),
			want: ModuleInfo{
				Name:        "Book",
				Description: "Generic Book",
				Language:    "en",
				Type:        "GenBook",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseModuleConf(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseModuleConf() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Name != tt.want.Name {
					t.Errorf("ParseModuleConf() Name = %v, want %v", got.Name, tt.want.Name)
				}
				if got.Description != tt.want.Description {
					t.Errorf("ParseModuleConf() Description = %v, want %v", got.Description, tt.want.Description)
				}
				if got.Language != tt.want.Language {
					t.Errorf("ParseModuleConf() Language = %v, want %v", got.Language, tt.want.Language)
				}
				if got.Type != tt.want.Type {
					t.Errorf("ParseModuleConf() Type = %v, want %v", got.Type, tt.want.Type)
				}
			}
		})
	}
}

func TestModuleTypeFromDriver(t *testing.T) {
	tests := []struct {
		driver string
		want   string
	}{
		{"zText", "Bible"},
		{"RawText", "Bible"},
		{"zText4", "Bible"},
		{"RawText4", "Bible"},
		{"zCom", "Commentary"},
		{"RawCom", "Commentary"},
		{"zCom4", "Commentary"},
		{"RawCom4", "Commentary"},
		{"zLD", "Dictionary"},
		{"RawLD", "Dictionary"},
		{"RawLD4", "Dictionary"},
		{"RawGenBook", "GenBook"},
		{"Unknown", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			got := moduleTypeFromDriver(tt.driver)
			if got != tt.want {
				t.Errorf("moduleTypeFromDriver(%v) = %v, want %v", tt.driver, got, tt.want)
			}
		})
	}
}

func TestParseModsArchive(t *testing.T) {
	// Create a valid tar.gz archive with conf files
	createTestArchive := func() []byte {
		var buf bytes.Buffer
		gzw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gzw)

		// Add a valid conf file
		conf1 := []byte(`[KJV]
Description=King James Version
Lang=en
ModDrv=zText`)

		tw.WriteHeader(&tar.Header{
			Name: "kjv.conf",
			Mode: 0644,
			Size: int64(len(conf1)),
		})
		tw.Write(conf1)

		// Add another conf file
		conf2 := []byte(`[ASV]
Description=American Standard Version
Lang=en
ModDrv=RawText`)

		tw.WriteHeader(&tar.Header{
			Name: "asv.conf",
			Mode: 0644,
			Size: int64(len(conf2)),
		})
		tw.Write(conf2)

		// Add a directory (should be skipped)
		tw.WriteHeader(&tar.Header{
			Name:     "mods.d/",
			Mode:     0755,
			Typeflag: tar.TypeDir,
		})

		// Add a non-.conf file (should be skipped)
		tw.WriteHeader(&tar.Header{
			Name: "readme.txt",
			Mode: 0644,
			Size: 5,
		})
		tw.Write([]byte("hello"))

		tw.Close()
		gzw.Close()
		return buf.Bytes()
	}

	t.Run("valid archive", func(t *testing.T) {
		data := createTestArchive()
		modules, err := ParseModsArchive(data)
		if err != nil {
			t.Fatalf("ParseModsArchive() error = %v", err)
		}
		if len(modules) != 2 {
			t.Errorf("ParseModsArchive() returned %d modules, want 2", len(modules))
		}
	})

	t.Run("invalid gzip", func(t *testing.T) {
		data := []byte("not a gzip file")
		_, err := ParseModsArchive(data)
		if err == nil {
			t.Error("ParseModsArchive() expected error for invalid gzip")
		}
	})
}

func TestExtractZipArchive(t *testing.T) {
	t.Run("valid zip archive", func(t *testing.T) {
		// Create a test zip archive
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)

		// Add a file
		fw, err := zw.Create("mods.d/test.conf")
		if err != nil {
			t.Fatalf("Failed to create zip entry: %v", err)
		}
		fw.Write([]byte("[TEST]\nDescription=Test"))

		// Add a directory
		_, err = zw.Create("modules/")
		if err != nil {
			t.Fatalf("Failed to create directory entry: %v", err)
		}

		zw.Close()

		// Extract to temp directory
		tempDir := t.TempDir()
		err = ExtractZipArchive(buf.Bytes(), tempDir)
		if err != nil {
			t.Fatalf("ExtractZipArchive() error = %v", err)
		}

		// Verify extracted file
		confPath := filepath.Join(tempDir, "mods.d", "test.conf")
		data, err := os.ReadFile(confPath)
		if err != nil {
			t.Errorf("Failed to read extracted file: %v", err)
		}
		if !strings.Contains(string(data), "[TEST]") {
			t.Errorf("Extracted file content = %q, want to contain [TEST]", string(data))
		}
	})

	t.Run("invalid zip", func(t *testing.T) {
		tempDir := t.TempDir()
		err := ExtractZipArchive([]byte("not a zip"), tempDir)
		if err == nil {
			t.Error("ExtractZipArchive() expected error for invalid zip")
		}
	})

	t.Run("directory traversal protection", func(t *testing.T) {
		// Create a malicious zip with path traversal
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		fw, _ := zw.Create("../../../etc/passwd")
		fw.Write([]byte("malicious"))
		zw.Close()

		tempDir := t.TempDir()
		err := ExtractZipArchive(buf.Bytes(), tempDir)
		if err == nil {
			t.Error("ExtractZipArchive() should reject path traversal")
		}
		if err != nil && !strings.Contains(err.Error(), "invalid file path") {
			t.Errorf("ExtractZipArchive() error = %v, want 'invalid file path'", err)
		}
	})
}

func TestListInstalled(t *testing.T) {
	t.Run("no mods.d directory", func(t *testing.T) {
		tempDir := t.TempDir()
		modules, err := ListInstalled(tempDir)
		if err != nil {
			t.Fatalf("ListInstalled() error = %v", err)
		}
		if len(modules) != 0 {
			t.Errorf("ListInstalled() returned %d modules, want 0", len(modules))
		}
	})

	t.Run("with installed modules", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		confData := []byte(`[KJV]
Description=King James Version
Lang=en
ModDrv=zText`)
		os.WriteFile(filepath.Join(modsDir, "kjv.conf"), confData, 0644)

		modules, err := ListInstalled(tempDir)
		if err != nil {
			t.Fatalf("ListInstalled() error = %v", err)
		}
		if len(modules) != 1 {
			t.Errorf("ListInstalled() returned %d modules, want 1", len(modules))
		}
		if len(modules) > 0 && modules[0].Name != "KJV" {
			t.Errorf("ListInstalled() module name = %v, want KJV", modules[0].Name)
		}
	})

	t.Run("empty path defaults to current", func(t *testing.T) {
		// Should not error, just return empty list if mods.d doesn't exist
		modules, err := ListInstalled("")
		if err != nil {
			t.Fatalf("ListInstalled(\"\") error = %v", err)
		}
		// Don't check length as it depends on current directory
		_ = modules
	})
}

func TestUninstall(t *testing.T) {
	t.Run("module not installed", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		err := Uninstall("KJV", tempDir)
		if err == nil {
			t.Error("Uninstall() expected error for non-existent module")
		}
		if err != nil && !strings.Contains(err.Error(), "not installed") {
			t.Errorf("Uninstall() error = %v, want 'not installed'", err)
		}
	})

	t.Run("successful uninstall", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		dataDir := filepath.Join(tempDir, "modules", "texts", "ztext", "kjv")
		os.MkdirAll(dataDir, 0755)
		os.WriteFile(filepath.Join(dataDir, "ot.bzs"), []byte("data"), 0644)

		confData := []byte(`[KJV]
Description=King James Version
Lang=en
ModDrv=zText
DataPath=./modules/texts/ztext/kjv/`)
		confPath := filepath.Join(modsDir, "kjv.conf")
		os.WriteFile(confPath, confData, 0644)

		err := Uninstall("KJV", tempDir)
		if err != nil {
			t.Fatalf("Uninstall() error = %v", err)
		}

		// Verify conf file is removed
		if _, err := os.Stat(confPath); !errors.Is(err, os.ErrNotExist) {
			t.Error("Uninstall() did not remove conf file")
		}

		// Verify data directory is removed
		if _, err := os.Stat(dataDir); !errors.Is(err, os.ErrNotExist) {
			t.Error("Uninstall() did not remove data directory")
		}
	})
}

func TestVerify(t *testing.T) {
	t.Run("module not installed", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		result, err := Verify("KJV", tempDir)
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if result.Valid {
			t.Error("Verify() result.Valid = true, want false")
		}
		if len(result.Errors) == 0 {
			t.Error("Verify() expected errors for non-existent module")
		}
	})

	t.Run("valid module", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		dataDir := filepath.Join(tempDir, "modules", "texts", "ztext", "kjv")
		os.MkdirAll(dataDir, 0755)
		os.WriteFile(filepath.Join(dataDir, "ot.bzs"), []byte("data"), 0644)

		confData := []byte(`[KJV]
Description=King James Version
Lang=en
ModDrv=zText
DataPath=./modules/texts/ztext/kjv/`)
		os.WriteFile(filepath.Join(modsDir, "kjv.conf"), confData, 0644)

		result, err := Verify("KJV", tempDir)
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if !result.Valid {
			t.Errorf("Verify() result.Valid = false, want true. Errors: %v", result.Errors)
		}
	})

	t.Run("missing data directory", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		confData := []byte(`[KJV]
Description=King James Version
Lang=en
ModDrv=zText
DataPath=./modules/texts/ztext/kjv/`)
		os.WriteFile(filepath.Join(modsDir, "kjv.conf"), confData, 0644)

		result, err := Verify("KJV", tempDir)
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if result.Valid {
			t.Error("Verify() result.Valid = true, want false for missing data")
		}
		if !strings.Contains(strings.Join(result.Errors, " "), "missing") {
			t.Errorf("Verify() errors = %v, want 'missing'", result.Errors)
		}
	})

	t.Run("empty data directory", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		dataDir := filepath.Join(tempDir, "modules", "texts", "ztext", "kjv")
		os.MkdirAll(dataDir, 0755)

		confData := []byte(`[KJV]
Description=King James Version
Lang=en
ModDrv=zText
DataPath=./modules/texts/ztext/kjv/`)
		os.WriteFile(filepath.Join(modsDir, "kjv.conf"), confData, 0644)

		result, err := Verify("KJV", tempDir)
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if !result.Valid {
			t.Errorf("Verify() result.Valid = false, want true (empty data is warning, not error)")
		}
		if len(result.Warnings) == 0 {
			t.Error("Verify() expected warning for empty data directory")
		}
	})

	t.Run("data path is file not directory", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		dataFile := filepath.Join(tempDir, "modules", "texts", "ztext", "kjv")
		os.MkdirAll(filepath.Dir(dataFile), 0755)
		os.WriteFile(dataFile, []byte("not a directory"), 0644)

		confData := []byte(`[KJV]
Description=King James Version
Lang=en
ModDrv=zText
DataPath=./modules/texts/ztext/kjv`)
		os.WriteFile(filepath.Join(modsDir, "kjv.conf"), confData, 0644)

		result, err := Verify("KJV", tempDir)
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if result.Valid {
			t.Error("Verify() result.Valid = true, want false when data path is file")
		}
	})
}

func TestListAvailable(t *testing.T) {
	t.Run("unknown source", func(t *testing.T) {
		_, err := ListAvailable("UnknownSource")
		if err == nil {
			t.Error("ListAvailable() expected error for unknown source")
		}
	})
}

func TestInstall(t *testing.T) {
	t.Run("unknown source", func(t *testing.T) {
		err := Install("UnknownSource", "KJV", t.TempDir())
		if err == nil {
			t.Error("Install() expected error for unknown source")
		}
		if err != nil && !strings.Contains(err.Error(), "unknown source") {
			t.Errorf("Install() error = %v, want 'unknown source'", err)
		}
	})

	// Note: Testing successful install would require mocking GetSource
	// which isn't easily done without refactoring. The individual components
	// (Download and ExtractZipArchive) are tested separately.
}

func TestClient_Download_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep to allow context cancellation
		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Download(ctx, server.URL)
	if err == nil {
		t.Error("Download() expected error for canceled context")
	}
}

func TestExtractZipArchive_DirectoryCreation(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Add nested file to test directory creation
	fw, _ := zw.Create("a/b/c/file.txt")
	fw.Write([]byte("content"))

	zw.Close()

	tempDir := t.TempDir()
	err := ExtractZipArchive(buf.Bytes(), tempDir)
	if err != nil {
		t.Fatalf("ExtractZipArchive() error = %v", err)
	}

	// Verify nested file exists
	filePath := filepath.Join(tempDir, "a", "b", "c", "file.txt")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Errorf("Failed to read nested file: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("File content = %q, want 'content'", string(data))
	}
}

func TestParseModuleConf_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "only newlines",
			data:    []byte("\n\n\n"),
			wantErr: true,
		},
		{
			name: "with comments",
			data: []byte(`[TEST]
# This is a comment
Description=Test
Lang=en`),
			wantErr: false,
		},
		{
			name: "with empty lines",
			data: []byte(`[TEST]

Description=Test

Lang=en`),
			wantErr: false,
		},
		{
			name: "unknown driver type",
			data: []byte(`[TEST]
Description=Test
ModDrv=UnknownDriver`),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseModuleConf(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseModuleConf() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestListInstalled_InvalidConf(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Write an invalid conf file
	os.WriteFile(filepath.Join(modsDir, "invalid.conf"), []byte("invalid"), 0644)

	// Should skip invalid files
	modules, err := ListInstalled(tempDir)
	if err != nil {
		t.Fatalf("ListInstalled() error = %v", err)
	}
	if len(modules) != 0 {
		t.Errorf("ListInstalled() returned %d modules, want 0 (invalid conf should be skipped)", len(modules))
	}
}

func TestListInstalled_NonConfFiles(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Create non-.conf files and directories
	os.WriteFile(filepath.Join(modsDir, "readme.txt"), []byte("hello"), 0644)
	os.Mkdir(filepath.Join(modsDir, "subdir"), 0755)

	modules, err := ListInstalled(tempDir)
	if err != nil {
		t.Fatalf("ListInstalled() error = %v", err)
	}
	if len(modules) != 0 {
		t.Errorf("ListInstalled() returned %d modules, want 0", len(modules))
	}
}

func TestUninstall_ReadConfError(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Create an unreadable conf file (directory instead of file)
	confPath := filepath.Join(modsDir, "kjv.conf")
	os.Mkdir(confPath, 0755)

	err := Uninstall("KJV", tempDir)
	if err == nil {
		t.Error("Uninstall() expected error for unreadable conf")
	}
}

func TestUninstall_InvalidConf(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Write invalid conf
	confPath := filepath.Join(modsDir, "kjv.conf")
	os.WriteFile(confPath, []byte("invalid"), 0644)

	err := Uninstall("KJV", tempDir)
	if err == nil {
		t.Error("Uninstall() expected error for invalid conf")
	}
}

func TestVerify_InvalidConf(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Write invalid conf
	os.WriteFile(filepath.Join(modsDir, "kjv.conf"), []byte("invalid"), 0644)

	result, err := Verify("KJV", tempDir)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if result.Valid {
		t.Error("Verify() result.Valid = true, want false for invalid conf")
	}
}

func TestVerify_ReadConfError(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Create conf as directory
	confPath := filepath.Join(modsDir, "kjv.conf")
	os.Mkdir(confPath, 0755)

	result, err := Verify("KJV", tempDir)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if result.Valid {
		t.Error("Verify() result.Valid = true, want false")
	}
}

func TestParseModsArchive_CorruptedEntries(t *testing.T) {
	// Create tar.gz with some corrupted entries
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	// Add a valid conf
	conf1 := []byte(`[KJV]
Description=King James Version
Lang=en
ModDrv=zText`)

	tw.WriteHeader(&tar.Header{
		Name: "kjv.conf",
		Mode: 0644,
		Size: int64(len(conf1)),
	})
	tw.Write(conf1)

	// Add a conf with wrong size header (corrupted)
	tw.WriteHeader(&tar.Header{
		Name: "asv.conf",
		Mode: 0644,
		Size: 1, // Wrong size - will cause read error
	})
	// Don't write enough data - this will cause read error that ParseModsArchive handles

	tw.Close()
	gzw.Close()

	modules, err := ParseModsArchive(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseModsArchive() error = %v", err)
	}
	// Should have parsed at least the valid one
	if len(modules) == 0 {
		t.Error("ParseModsArchive() returned 0 modules, expected at least 1")
	}
}

func TestRefreshSource_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return minimal valid tar.gz
		var buf bytes.Buffer
		gzw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gzw)
		tw.Close()
		gzw.Close()

		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
	}))
	defer server.Close()

	// Can't easily test with real sources without mocking
	// Skip for now - the error case is already covered
}

func TestListAvailable_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return valid tar.gz with conf files
		var buf bytes.Buffer
		gzw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gzw)

		conf := []byte(`[KJV]
Description=King James Version
Lang=en
ModDrv=zText`)

		tw.WriteHeader(&tar.Header{
			Name: "kjv.conf",
			Mode: 0644,
			Size: int64(len(conf)),
		})
		tw.Write(conf)

		tw.Close()
		gzw.Close()

		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
	}))
	defer server.Close()

	// Can't easily test without mocking GetSource
	// The error case is already tested
}

// Test edge case where ExtractZipArchive creates explicit directories
func TestExtractZipArchive_ExplicitDirectory(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Add an explicit directory entry
	dirHeader := &zip.FileHeader{
		Name: "testdir/",
	}
	dirHeader.SetMode(0755 | os.ModeDir)
	_, err := zw.CreateHeader(dirHeader)
	if err != nil {
		t.Fatalf("Failed to create directory header: %v", err)
	}

	// Add a file in that directory
	fw, _ := zw.Create("testdir/file.txt")
	fw.Write([]byte("content"))

	zw.Close()

	tempDir := t.TempDir()
	err = ExtractZipArchive(buf.Bytes(), tempDir)
	if err != nil {
		t.Fatalf("ExtractZipArchive() error = %v", err)
	}

	// Verify directory was created
	dirPath := filepath.Join(tempDir, "testdir")
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Errorf("Directory not created: %v", err)
	}
	if info != nil && !info.IsDir() {
		t.Error("Expected directory, got file")
	}
}

// Test error handling for ParseModsArchive with tar errors
func TestParseModsArchive_TarError(t *testing.T) {
	// Create invalid tar (valid gzip but invalid tar)
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	gzw.Write([]byte("not a valid tar"))
	gzw.Close()

	modules, err := ParseModsArchive(buf.Bytes())
	// Should not return error, just empty modules list
	if err != nil {
		t.Errorf("ParseModsArchive() error = %v, expected no error", err)
	}
	if len(modules) != 0 {
		t.Errorf("ParseModsArchive() returned %d modules, want 0", len(modules))
	}
}

// Test Download with different HTTP status codes
func TestClient_Download_ServerErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{"500 Internal Server Error", 500, true},
		{"403 Forbidden", 403, true},
		{"400 Bad Request", 400, true},
		{"200 OK", 200, false},
		{"201 Created", 201, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte("response"))
			}))
			defer server.Close()

			client := NewClient()
			ctx := context.Background()
			_, err := client.Download(ctx, server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("Download() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test ListInstalled with read error on conf file
func TestListInstalled_ReadError(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Create a conf file as a directory (will cause read error)
	confPath := filepath.Join(modsDir, "test.conf")
	os.Mkdir(confPath, 0755)

	// Should skip unreadable files
	modules, err := ListInstalled(tempDir)
	if err != nil {
		t.Fatalf("ListInstalled() error = %v", err)
	}
	if len(modules) != 0 {
		t.Errorf("ListInstalled() returned %d modules, want 0", len(modules))
	}
}

// Test Verify with missing DataPath field
func TestVerify_NoDataPath(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Conf without DataPath
	confData := []byte(`[TEST]
Description=Test
Lang=en
ModDrv=zText`)
	os.WriteFile(filepath.Join(modsDir, "test.conf"), confData, 0644)

	result, err := Verify("TEST", tempDir)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	// Should be valid as DataPath is optional
	if !result.Valid {
		t.Errorf("Verify() result.Valid = false, errors: %v", result.Errors)
	}
}

// Test Uninstall with missing data directory (shouldn't fail)
func TestUninstall_MissingDataDir(t *testing.T) {
	tempDir := t.TempDir()
	modsDir := filepath.Join(tempDir, "mods.d")
	os.MkdirAll(modsDir, 0755)

	// Conf with DataPath that doesn't exist
	confData := []byte(`[TEST]
Description=Test
Lang=en
ModDrv=zText
DataPath=./nonexistent/path/`)
	confPath := filepath.Join(modsDir, "test.conf")
	os.WriteFile(confPath, confData, 0644)

	err := Uninstall("TEST", tempDir)
	if err != nil {
		t.Fatalf("Uninstall() error = %v, should succeed even if data missing", err)
	}

	// Verify conf was removed
	if _, err := os.Stat(confPath); !errors.Is(err, os.ErrNotExist) {
		t.Error("Uninstall() did not remove conf file")
	}
}
