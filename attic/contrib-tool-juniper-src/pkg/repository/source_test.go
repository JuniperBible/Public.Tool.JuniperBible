package repository

import (
	"testing"
)

// TestSource_Validate tests validation of Source configuration
func TestSource_Validate(t *testing.T) {
	tests := []struct {
		name    string
		source  Source
		wantErr bool
	}{
		{
			name: "valid FTP source",
			source: Source{
				Name:      "CrossWire",
				Type:      SourceTypeFTP,
				Host:      "ftp.crosswire.org",
				Directory: "/pub/sword/raw",
			},
			wantErr: false,
		},
		{
			name: "valid HTTP source",
			source: Source{
				Name:      "TestHTTP",
				Type:      SourceTypeHTTP,
				Host:      "example.com",
				Directory: "/sword",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			source: Source{
				Name:      "",
				Type:      SourceTypeFTP,
				Host:      "ftp.example.com",
				Directory: "/sword",
			},
			wantErr: true,
		},
		{
			name: "empty host",
			source: Source{
				Name:      "Test",
				Type:      SourceTypeFTP,
				Host:      "",
				Directory: "/sword",
			},
			wantErr: true,
		},
		{
			name: "empty directory",
			source: Source{
				Name:      "Test",
				Type:      SourceTypeFTP,
				Host:      "ftp.example.com",
				Directory: "",
			},
			wantErr: true,
		},
		{
			name: "invalid source type",
			source: Source{
				Name:      "Test",
				Type:      SourceType("invalid"),
				Host:      "example.com",
				Directory: "/sword",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.source.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Source.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSource_URL tests URL generation for sources
func TestSource_URL(t *testing.T) {
	tests := []struct {
		name     string
		source   Source
		wantURL  string
		wantPath string
	}{
		{
			name: "FTP URL",
			source: Source{
				Name:      "CrossWire",
				Type:      SourceTypeFTP,
				Host:      "ftp.crosswire.org",
				Directory: "/pub/sword/raw",
			},
			wantURL:  "ftp://ftp.crosswire.org",
			wantPath: "/pub/sword/raw",
		},
		{
			name: "HTTP URL",
			source: Source{
				Name:      "TestHTTP",
				Type:      SourceTypeHTTP,
				Host:      "example.com",
				Directory: "/sword",
			},
			wantURL:  "http://example.com",
			wantPath: "/sword",
		},
		{
			name: "HTTPS URL",
			source: Source{
				Name:      "TestHTTPS",
				Type:      SourceTypeHTTPS,
				Host:      "secure.example.com",
				Directory: "/modules",
			},
			wantURL:  "https://secure.example.com",
			wantPath: "/modules",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL := tt.source.BaseURL()
			if gotURL != tt.wantURL {
				t.Errorf("Source.BaseURL() = %v, want %v", gotURL, tt.wantURL)
			}
			if tt.source.Directory != tt.wantPath {
				t.Errorf("Source.Directory = %v, want %v", tt.source.Directory, tt.wantPath)
			}
		})
	}
}

// TestSource_ModsIndexURL tests generation of mods.d.tar.gz URL
func TestSource_ModsIndexURL(t *testing.T) {
	tests := []struct {
		name    string
		source  Source
		wantURL string
	}{
		{
			name: "CrossWire mods index",
			source: Source{
				Name:      "CrossWire",
				Type:      SourceTypeFTP,
				Host:      "ftp.crosswire.org",
				Directory: "/pub/sword/raw",
			},
			wantURL: "ftp://ftp.crosswire.org/pub/sword/raw/mods.d.tar.gz",
		},
		{
			name: "HTTP source mods index",
			source: Source{
				Name:      "TestHTTP",
				Type:      SourceTypeHTTP,
				Host:      "example.com",
				Directory: "/sword",
			},
			wantURL: "http://example.com/sword/mods.d.tar.gz",
		},
		{
			name: "directory with trailing slash",
			source: Source{
				Name:      "Test",
				Type:      SourceTypeFTP,
				Host:      "ftp.example.com",
				Directory: "/sword/",
			},
			wantURL: "ftp://ftp.example.com/sword/mods.d.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL := tt.source.ModsIndexURL()
			if gotURL != tt.wantURL {
				t.Errorf("Source.ModsIndexURL() = %v, want %v", gotURL, tt.wantURL)
			}
		})
	}
}

// TestDefaultSources tests that default sources are properly configured
func TestDefaultSources(t *testing.T) {
	sources := DefaultSources()

	// Check we have all the sources from the official installmgr
	expectedNames := []string{
		"Bible.org",
		"CrossWire",
		"CrossWire Attic",
		"CrossWire Beta",
		"CrossWire Wycliffe",
		"Deutsche Bibelgesellschaft",
		"IBT",
		"Lockman Foundation",
		"STEP Bible",
		"Xiphos",
		"eBible.org",
	}
	if len(sources) != len(expectedNames) {
		t.Errorf("DefaultSources() returned %d sources, want %d", len(sources), len(expectedNames))
	}

	// Create a map for easier lookup
	sourceMap := make(map[string]Source)
	for _, s := range sources {
		sourceMap[s.Name] = s
	}

	// Verify each expected source exists
	for _, name := range expectedNames {
		if _, ok := sourceMap[name]; !ok {
			t.Errorf("DefaultSources() missing expected source: %s", name)
		}
	}

	// Verify all sources are valid
	for _, s := range sources {
		if err := s.Validate(); err != nil {
			t.Errorf("DefaultSources() source %q is invalid: %v", s.Name, err)
		}
	}
}

// TestDefaultSources_CrossWire tests CrossWire source configuration
func TestDefaultSources_CrossWire(t *testing.T) {
	sources := DefaultSources()

	var crosswire *Source
	for i := range sources {
		if sources[i].Name == "CrossWire" {
			crosswire = &sources[i]
			break
		}
	}

	if crosswire == nil {
		t.Fatal("CrossWire source not found in DefaultSources()")
	}

	if crosswire.Type != SourceTypeFTP {
		t.Errorf("CrossWire.Type = %v, want %v", crosswire.Type, SourceTypeFTP)
	}
	if crosswire.Host != "ftp.crosswire.org" {
		t.Errorf("CrossWire.Host = %v, want ftp.crosswire.org", crosswire.Host)
	}
	if crosswire.Directory != "/pub/sword/raw" {
		t.Errorf("CrossWire.Directory = %v, want /pub/sword/raw", crosswire.Directory)
	}
}

// TestDefaultSources_CrossWireBeta tests CrossWire Beta source configuration
func TestDefaultSources_CrossWireBeta(t *testing.T) {
	sources := DefaultSources()

	var beta *Source
	for i := range sources {
		if sources[i].Name == "CrossWire Beta" {
			beta = &sources[i]
			break
		}
	}

	if beta == nil {
		t.Fatal("CrossWire Beta source not found in DefaultSources()")
	}

	if beta.Type != SourceTypeFTP {
		t.Errorf("CrossWire Beta.Type = %v, want %v", beta.Type, SourceTypeFTP)
	}
	if beta.Host != "ftp.crosswire.org" {
		t.Errorf("CrossWire Beta.Host = %v, want ftp.crosswire.org", beta.Host)
	}
	if beta.Directory != "/pub/sword/betaraw" {
		t.Errorf("CrossWire Beta.Directory = %v, want /pub/sword/betaraw", beta.Directory)
	}
}

// TestGetSource tests retrieving a source by name
func TestGetSource(t *testing.T) {
	tests := []struct {
		name       string
		sourceName string
		wantFound  bool
	}{
		{
			name:       "existing source",
			sourceName: "CrossWire",
			wantFound:  true,
		},
		{
			name:       "existing source case sensitive",
			sourceName: "crosswire",
			wantFound:  false,
		},
		{
			name:       "non-existent source",
			sourceName: "NonExistent",
			wantFound:  false,
		},
		{
			name:       "empty name",
			sourceName: "",
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, found := GetSource(tt.sourceName)
			if found != tt.wantFound {
				t.Errorf("GetSource(%q) found = %v, want %v", tt.sourceName, found, tt.wantFound)
			}
			if found && source.Name != tt.sourceName {
				t.Errorf("GetSource(%q) returned source with name %q", tt.sourceName, source.Name)
			}
		})
	}
}

// TestSourceType_String tests string representation of source types
func TestSourceType_String(t *testing.T) {
	tests := []struct {
		st   SourceType
		want string
	}{
		{SourceTypeFTP, "FTP"},
		{SourceTypeHTTP, "HTTP"},
		{SourceTypeHTTPS, "HTTPS"},
		{SourceType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.st), func(t *testing.T) {
			if got := tt.st.String(); got != tt.want {
				t.Errorf("SourceType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSource_ModuleDataURL tests generation of module data URL
func TestSource_ModuleDataURL(t *testing.T) {
	source := Source{
		Name:      "CrossWire",
		Type:      SourceTypeFTP,
		Host:      "ftp.crosswire.org",
		Directory: "/pub/sword/raw",
	}

	tests := []struct {
		name     string
		dataPath string
		wantURL  string
	}{
		{
			name:     "standard path",
			dataPath: "./modules/texts/ztext/kjv/",
			wantURL:  "ftp://ftp.crosswire.org/pub/sword/raw/modules/texts/ztext/kjv/",
		},
		{
			name:     "path without leading dot",
			dataPath: "modules/texts/ztext/kjv/",
			wantURL:  "ftp://ftp.crosswire.org/pub/sword/raw/modules/texts/ztext/kjv/",
		},
		{
			name:     "path without trailing slash",
			dataPath: "./modules/texts/ztext/kjv",
			wantURL:  "ftp://ftp.crosswire.org/pub/sword/raw/modules/texts/ztext/kjv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL := source.ModuleDataURL(tt.dataPath)
			if gotURL != tt.wantURL {
				t.Errorf("Source.ModuleDataURL(%q) = %v, want %v", tt.dataPath, gotURL, tt.wantURL)
			}
		})
	}
}

// TestParseSourcesConf tests parsing install.conf format
func TestParseSourcesConf(t *testing.T) {
	confContent := `[General]

[CrossWire]
FTPSource=ftp.crosswire.org|/pub/sword/raw|CrossWire

[CrossWire Beta]
FTPSource=ftp.crosswire.org|/pub/sword/betaraw|CrossWire Beta
`

	sources, err := ParseSourcesConf([]byte(confContent))
	if err != nil {
		t.Fatalf("ParseSourcesConf() error = %v", err)
	}

	if len(sources) != 2 {
		t.Errorf("ParseSourcesConf() returned %d sources, want 2", len(sources))
	}

	// Verify CrossWire
	if len(sources) > 0 {
		cw := sources[0]
		if cw.Name != "CrossWire" {
			t.Errorf("sources[0].Name = %q, want CrossWire", cw.Name)
		}
		if cw.Type != SourceTypeFTP {
			t.Errorf("sources[0].Type = %v, want FTP", cw.Type)
		}
		if cw.Host != "ftp.crosswire.org" {
			t.Errorf("sources[0].Host = %q, want ftp.crosswire.org", cw.Host)
		}
	}
}

// TestParseSourcesConf_HTTPSource tests parsing HTTP source entries
func TestParseSourcesConf_HTTPSource(t *testing.T) {
	confContent := `[General]

[TestHTTP]
HTTPSource=example.com|/sword|TestHTTP
`

	sources, err := ParseSourcesConf([]byte(confContent))
	if err != nil {
		t.Fatalf("ParseSourcesConf() error = %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("ParseSourcesConf() returned %d sources, want 1", len(sources))
	}

	if sources[0].Type != SourceTypeHTTP {
		t.Errorf("sources[0].Type = %v, want HTTP", sources[0].Type)
	}
}

// TestParseSourcesConf_Empty tests parsing empty conf
func TestParseSourcesConf_Empty(t *testing.T) {
	sources, err := ParseSourcesConf([]byte(""))
	if err != nil {
		t.Fatalf("ParseSourcesConf() error = %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("ParseSourcesConf() returned %d sources, want 0", len(sources))
	}
}

// TestParseSourcesConf_InvalidFormat tests parsing invalid format
func TestParseSourcesConf_InvalidFormat(t *testing.T) {
	confContent := `[General]

[BadSource]
FTPSource=invalid-format
`

	sources, err := ParseSourcesConf([]byte(confContent))
	// Should not error, just skip invalid entries
	if err != nil {
		t.Fatalf("ParseSourcesConf() unexpected error = %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("ParseSourcesConf() returned %d sources for invalid format, want 0", len(sources))
	}
}
