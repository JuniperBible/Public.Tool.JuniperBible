package juniper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/internal/formats/swordpure"
)

func TestStripMarkup(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple text",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "text with tags",
			input: "<div>Hello</div> <span>world</span>",
			want:  "Hello world",
		},
		{
			name:  "nested tags",
			input: "<div><span>Hello</span></div>",
			want:  "Hello",
		},
		{
			name:  "empty tags",
			input: "<div></div>",
			want:  "",
		},
		{
			name:  "text before and after tags",
			input: "Before <tag>middle</tag> after",
			want:  "Before middle after",
		},
		{
			name:  "only tags",
			input: "<tag1><tag2></tag2></tag1>",
			want:  "",
		},
		{
			name:  "OSIS markup",
			input: `<verse osisID="Gen.1.1">In the beginning</verse>`,
			want:  "In the beginning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkup(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkup() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsPlaceholder(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid verse",
			input: "In the beginning God created the heavens and the earth",
			want:  false,
		},
		{
			name:  "placeholder Gen 1:1",
			input: "Genesis 1:1",
			want:  true,
		},
		{
			name:  "placeholder with number",
			input: "1 John 3:16",
			want:  true,
		},
		{
			name:  "placeholder with roman numerals",
			input: "II Corinthians 5:17",
			want:  true,
		},
		{
			name:  "short text",
			input: "Hi",
			want:  true,
		},
		{
			name:  "empty",
			input: "",
			want:  true,
		},
		{
			name:  "placeholder with trailing colon",
			input: "Psalms 23:1:",
			want:  true,
		},
		{
			name:  "placeholder Song of Solomon",
			input: "Song of Solomon 1:1",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPlaceholder(tt.input)
			if got != tt.want {
				t.Errorf("isPlaceholder(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetLicense(t *testing.T) {
	tests := []struct {
		name string
		conf *swordpure.ConfFile
		want string
	}{
		{
			name: "has license field",
			conf: &swordpure.ConfFile{
				License:   "Public Domain",
				Copyright: "Copyright 2020",
			},
			want: "Public Domain",
		},
		{
			name: "no license but has copyright",
			conf: &swordpure.ConfFile{
				Copyright: "Copyright 2020",
			},
			want: "Copyright 2020",
		},
		{
			name: "neither license nor copyright",
			conf: &swordpure.ConfFile{},
			want: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getLicense(tt.conf)
			if got != tt.want {
				t.Errorf("getLicense() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetLicenseText(t *testing.T) {
	tests := []struct {
		name      string
		conf      *swordpure.ConfFile
		module    *Module
		swordPath string
		want      string
	}{
		{
			name: "has DistributionLicenseNotes",
			conf: &swordpure.ConfFile{
				Properties: map[string]string{
					"DistributionLicenseNotes": "This is the license",
				},
			},
			module: &Module{},
			want:   "This is the license",
		},
		{
			name: "has ShortCopyright",
			conf: &swordpure.ConfFile{
				Properties: map[string]string{
					"ShortCopyright": "Copyright notice",
				},
			},
			module: &Module{},
			want:   "Copyright notice",
		},
		{
			name: "has Copyright and TextSource",
			conf: &swordpure.ConfFile{
				Copyright: "Copyright 2020",
				Properties: map[string]string{
					"TextSource": "Original Text",
				},
			},
			module: &Module{},
			want:   "Copyright 2020\n\nSource: Original Text",
		},
		{
			name: "has About only",
			conf: &swordpure.ConfFile{
				About: "About this module",
			},
			module: &Module{},
			want:   "About this module",
		},
		{
			name: "has Copyright and About",
			conf: &swordpure.ConfFile{
				Copyright: "Copyright 2020",
				About:     "About this module",
			},
			module: &Module{},
			want:   "Copyright 2020\n\nAbout this module",
		},
		{
			name:   "nothing available",
			conf:   &swordpure.ConfFile{},
			module: &Module{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getLicenseText(tt.conf, tt.swordPath, tt.module)
			if got != tt.want {
				t.Errorf("getLicenseText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetLicenseText_FromFile(t *testing.T) {
	tempDir := t.TempDir()
	dataPath := filepath.Join(tempDir, "modules", "texts", "ztext", "kjv")
	os.MkdirAll(dataPath, 0755)

	// Create LICENSE file
	licenseContent := "This is the license from file"
	os.WriteFile(filepath.Join(dataPath, "LICENSE"), []byte(licenseContent), 0644)

	conf := &swordpure.ConfFile{}
	module := &Module{
		DataPath: "./modules/texts/ztext/kjv",
	}

	got := getLicenseText(conf, tempDir, module)
	if got != licenseContent {
		t.Errorf("getLicenseText() = %q, want %q", got, licenseContent)
	}
}

func TestGenerateBibleTags(t *testing.T) {
	tests := []struct {
		name       string
		module     *Module
		conf       *swordpure.ConfFile
		wantTags   []string
		checkTags  func([]string) bool
	}{
		{
			name: "English Bible",
			module: &Module{
				Name: "KJV",
				Lang: "en",
				Description: "King James Version",
			},
			conf: &swordpure.ConfFile{
				Versification: "KJV",
			},
			checkTags: func(tags []string) bool {
				hasEnglish := false
				hasProtestant := false
				for _, tag := range tags {
					if tag == "English" {
						hasEnglish = true
					}
					if tag == "Protestant Canon" {
						hasProtestant = true
					}
				}
				return hasEnglish && hasProtestant
			},
		},
		{
			name: "Latin Vulgate",
			module: &Module{
				Name: "Vulgate",
				Lang: "la",
			},
			conf: &swordpure.ConfFile{
				Versification: "Vulg",
			},
			checkTags: func(tags []string) bool {
				hasCatholic := false
				for _, tag := range tags {
					if tag == "Catholic Canon" {
						hasCatholic = true
					}
				}
				return hasCatholic
			},
		},
		{
			name: "Greek SBLGNT",
			module: &Module{
				Name: "SBLGNT",
				Lang: "grc",
			},
			conf: &swordpure.ConfFile{},
			checkTags: func(tags []string) bool {
				hasNT := false
				hasCritical := false
				for _, tag := range tags {
					if tag == "New Testament" {
						hasNT = true
					}
					if tag == "Critical Text" {
						hasCritical = true
					}
				}
				return hasNT && hasCritical
			},
		},
		{
			name: "Hebrew OSMHB",
			module: &Module{
				Name: "OSMHB",
				Lang: "he",
			},
			conf: &swordpure.ConfFile{},
			checkTags: func(tags []string) bool {
				hasOT := false
				for _, tag := range tags {
					if tag == "Old Testament" {
						hasOT = true
					}
				}
				return hasOT
			},
		},
		{
			name: "Public Domain",
			module: &Module{
				Name: "WEB",
				Lang: "en",
			},
			conf: &swordpure.ConfFile{
				License: "Public Domain",
			},
			checkTags: func(tags []string) bool {
				hasPD := false
				for _, tag := range tags {
					if tag == "Public Domain" {
						hasPD = true
					}
				}
				return hasPD
			},
		},
		{
			name: "LXX Septuagint",
			module: &Module{
				Name: "LXX",
				Lang: "grc",
			},
			conf: &swordpure.ConfFile{
				Versification: "LXX",
			},
			checkTags: func(tags []string) bool {
				hasOrthodox := false
				hasSeptuagint := false
				for _, tag := range tags {
					if tag == "Orthodox Canon" {
						hasOrthodox = true
					}
					if tag == "Septuagint" {
						hasSeptuagint = true
					}
				}
				return hasOrthodox && hasSeptuagint
			},
		},
		{
			name: "Masoretic Text",
			module: &Module{
				Name: "test",
				Lang: "he",
			},
			conf: &swordpure.ConfFile{
				Versification: "Leningrad",
			},
			checkTags: func(tags []string) bool {
				for _, tag := range tags {
					if tag == "Masoretic Text" {
						return true
					}
				}
				return false
			},
		},
		{
			name: "Modern Translation",
			module: &Module{
				Name: "WEB",
				Lang: "en",
				Description: "World English Bible",
			},
			conf: &swordpure.ConfFile{},
			checkTags: func(tags []string) bool {
				for _, tag := range tags {
					if tag == "Modern Translation" {
						return true
					}
				}
				return false
			},
		},
		{
			name: "With Strong's Numbers",
			module: &Module{
				Name: "KJVStrongs",
				Lang: "en",
				Description: "KJV with Strong's Numbers",
			},
			conf: &swordpure.ConfFile{},
			checkTags: func(tags []string) bool {
				for _, tag := range tags {
					if tag == "Strong's Numbers" {
						return true
					}
				}
				return false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateBibleTags(tt.module, tt.conf)
			if tt.checkTags != nil && !tt.checkTags(got) {
				t.Errorf("generateBibleTags() = %v, check failed", got)
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	t.Run("successful write", func(t *testing.T) {
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "test.json")

		data := map[string]interface{}{
			"key":   "value",
			"number": 42,
		}

		err := writeJSON(testFile, data)
		if err != nil {
			t.Fatalf("writeJSON() error = %v", err)
		}

		// Verify file exists and contains JSON
		content, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read written file: %v", err)
		}

		if !strings.Contains(string(content), "\"key\"") {
			t.Error("writeJSON() didn't write valid JSON")
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		err := writeJSON("/nonexistent/dir/test.json", map[string]string{"key": "value"})
		if err == nil {
			t.Error("writeJSON() expected error for invalid path")
		}
	})
}

func TestParseVerseRef(t *testing.T) {
	tests := []struct {
		name         string
		ref          string
		wantBook     string
		wantChapter  int
		wantVerse    int
	}{
		{
			name:        "simple reference",
			ref:         "Genesis 1:1",
			wantBook:    "Genesis",
			wantChapter: 1,
			wantVerse:   1,
		},
		{
			name:        "numbered book",
			ref:         "1 John 3:16",
			wantBook:    "1 John",
			wantChapter: 3,
			wantVerse:   16,
		},
		{
			name:        "book with multiple words",
			ref:         "Song of Solomon 2:1",
			wantBook:    "Song of Solomon",
			wantChapter: 2,
			wantVerse:   1,
		},
		{
			name:        "no chapter:verse",
			ref:         "Genesis",
			wantBook:    "Genesis",
			wantChapter: 0,
			wantVerse:   0,
		},
		{
			name:        "chapter only",
			ref:         "Psalms 23",
			wantBook:    "Psalms",
			wantChapter: 23,
			wantVerse:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			book, chapter, verse := parseVerseRef(tt.ref)
			if book != tt.wantBook {
				t.Errorf("parseVerseRef() book = %v, want %v", book, tt.wantBook)
			}
			if chapter != tt.wantChapter {
				t.Errorf("parseVerseRef() chapter = %v, want %v", chapter, tt.wantChapter)
			}
			if verse != tt.wantVerse {
				t.Errorf("parseVerseRef() verse = %v, want %v", verse, tt.wantVerse)
			}
		})
	}
}

func TestHugo(t *testing.T) {
	t.Run("no modules found", func(t *testing.T) {
		tempDir := t.TempDir()
		modsDir := filepath.Join(tempDir, "mods.d")
		os.MkdirAll(modsDir, 0755)

		cfg := HugoConfig{
			Path:   tempDir,
			Output: filepath.Join(tempDir, "output"),
			All:    true,
		}
		err := Hugo(cfg)
		if err == nil {
			t.Error("Hugo() expected error for no modules")
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

		cfg := HugoConfig{
			Path:   tempDir,
			Output: filepath.Join(tempDir, "output"),
		}
		err := Hugo(cfg)
		if err == nil {
			t.Error("Hugo() expected error when no modules specified")
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

		cfg := HugoConfig{
			Path:    tempDir,
			Output:  filepath.Join(tempDir, "output"),
			Modules: []string{"NONEXISTENT"},
		}
		err := Hugo(cfg)
		if err == nil {
			t.Error("Hugo() expected error when requested module not found")
		}
	})
}
