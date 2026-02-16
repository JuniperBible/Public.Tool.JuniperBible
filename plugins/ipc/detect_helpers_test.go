package ipc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckExtension(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		extensions []string
		want       bool
	}{
		{
			name:       "single extension match",
			path:       "/path/to/file.xml",
			extensions: []string{".xml"},
			want:       true,
		},
		{
			name:       "multiple extensions, first match",
			path:       "/path/to/file.xml",
			extensions: []string{".xml", ".html"},
			want:       true,
		},
		{
			name:       "multiple extensions, second match",
			path:       "/path/to/file.html",
			extensions: []string{".xml", ".html"},
			want:       true,
		},
		{
			name:       "case insensitive match",
			path:       "/path/to/file.XML",
			extensions: []string{".xml"},
			want:       true,
		},
		{
			name:       "no match",
			path:       "/path/to/file.txt",
			extensions: []string{".xml", ".html"},
			want:       false,
		},
		{
			name:       "no extension",
			path:       "/path/to/file",
			extensions: []string{".xml"},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckExtension(tt.path, tt.extensions...)
			if got != tt.want {
				t.Errorf("CheckExtension() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckMagicBytes(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create test file with known content
	testFile := filepath.Join(tmpDir, "test.bin")
	testData := []byte{0x50, 0x4b, 0x03, 0x04, 0x00, 0x00}
	if err := os.WriteFile(testFile, testData, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name  string
		path  string
		magic []byte
		want  bool
	}{
		{
			name:  "exact match",
			path:  testFile,
			magic: []byte{0x50, 0x4b, 0x03, 0x04},
			want:  true,
		},
		{
			name:  "partial match at start",
			path:  testFile,
			magic: []byte{0x50, 0x4b},
			want:  true,
		},
		{
			name:  "no match",
			path:  testFile,
			magic: []byte{0xff, 0xd8, 0xff, 0xe0},
			want:  false,
		},
		{
			name:  "file too short",
			path:  testFile,
			magic: []byte{0x50, 0x4b, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00},
			want:  false,
		},
		{
			name:  "file not found",
			path:  "/nonexistent/file",
			magic: []byte{0x50, 0x4b},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckMagicBytes(tt.path, tt.magic)
			if got != tt.want {
				t.Errorf("CheckMagicBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckContentContains(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	xmlFile := filepath.Join(tmpDir, "test.xml")
	xmlContent := `<?xml version="1.0"?><bible><book><verse>text</verse></book></bible>`
	if err := os.WriteFile(xmlFile, []byte(xmlContent), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name       string
		path       string
		substrings []string
		want       bool
	}{
		{
			name:       "single substring found",
			path:       xmlFile,
			substrings: []string{"<bible>"},
			want:       true,
		},
		{
			name:       "multiple substrings all found",
			path:       xmlFile,
			substrings: []string{"<bible>", "<book>", "<verse>"},
			want:       true,
		},
		{
			name:       "one substring not found",
			path:       xmlFile,
			substrings: []string{"<bible>", "<chapter>"},
			want:       false,
		},
		{
			name:       "substring not found",
			path:       xmlFile,
			substrings: []string{"<html>"},
			want:       false,
		},
		{
			name:       "file not found",
			path:       "/nonexistent/file",
			substrings: []string{"<bible>"},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckContentContains(tt.path, tt.substrings...)
			if got != tt.want {
				t.Errorf("CheckContentContains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckContentContainsAny(t *testing.T) {
	tmpDir := t.TempDir()

	xmlFile := filepath.Join(tmpDir, "test.xml")
	xmlContent := `<?xml version="1.0"?><bible><book><verse>text</verse></book></bible>`
	if err := os.WriteFile(xmlFile, []byte(xmlContent), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name       string
		path       string
		substrings []string
		want       bool
	}{
		{
			name:       "first substring found",
			path:       xmlFile,
			substrings: []string{"<bible>", "<html>"},
			want:       true,
		},
		{
			name:       "second substring found",
			path:       xmlFile,
			substrings: []string{"<html>", "<bible>"},
			want:       true,
		},
		{
			name:       "none found",
			path:       xmlFile,
			substrings: []string{"<html>", "<body>"},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckContentContainsAny(tt.path, tt.substrings...)
			if got != tt.want {
				t.Errorf("CheckContentContainsAny() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectByExtension(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		formatName string
		extensions []string
		wantDetect bool
		wantFormat string
	}{
		{
			name:       "match",
			path:       "/path/to/file.xml",
			formatName: "XML",
			extensions: []string{".xml"},
			wantDetect: true,
			wantFormat: "XML",
		},
		{
			name:       "no match",
			path:       "/path/to/file.txt",
			formatName: "XML",
			extensions: []string{".xml"},
			wantDetect: false,
			wantFormat: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectByExtension(tt.path, tt.formatName, tt.extensions...)
			if result.Detected != tt.wantDetect {
				t.Errorf("DetectByExtension() Detected = %v, want %v", result.Detected, tt.wantDetect)
			}
			if result.Format != tt.wantFormat {
				t.Errorf("DetectByExtension() Format = %v, want %v", result.Format, tt.wantFormat)
			}
			if result.Reason == "" {
				t.Error("DetectByExtension() Reason should not be empty")
			}
		})
	}
}

func TestDetectByMagicBytes(t *testing.T) {
	tmpDir := t.TempDir()

	zipFile := filepath.Join(tmpDir, "test.zip")
	zipData := []byte{0x50, 0x4b, 0x03, 0x04, 0x00, 0x00}
	if err := os.WriteFile(zipFile, zipData, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name       string
		path       string
		formatName string
		magic      []byte
		wantDetect bool
	}{
		{
			name:       "match",
			path:       zipFile,
			formatName: "ZIP",
			magic:      []byte{0x50, 0x4b, 0x03, 0x04},
			wantDetect: true,
		},
		{
			name:       "no match",
			path:       zipFile,
			formatName: "GZIP",
			magic:      []byte{0x1f, 0x8b},
			wantDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectByMagicBytes(tt.path, tt.formatName, tt.magic)
			if result.Detected != tt.wantDetect {
				t.Errorf("DetectByMagicBytes() Detected = %v, want %v", result.Detected, tt.wantDetect)
			}
		})
	}
}

func TestStandardDetect(t *testing.T) {
	tmpDir := t.TempDir()

	// Create XML file with Bible content
	xmlFile := filepath.Join(tmpDir, "test.xml")
	xmlContent := `<?xml version="1.0"?><bible><book><verse>text</verse></book></bible>`
	if err := os.WriteFile(xmlFile, []byte(xmlContent), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create XML file without Bible content
	nonBibleXML := filepath.Join(tmpDir, "other.xml")
	otherContent := `<?xml version="1.0"?><root><item>data</item></root>`
	if err := os.WriteFile(nonBibleXML, []byte(otherContent), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create non-XML file
	txtFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtFile, []byte("plain text"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name            string
		path            string
		formatName      string
		extensions      []string
		contentPatterns []string
		wantDetect      bool
	}{
		{
			name:            "extension and content match",
			path:            xmlFile,
			formatName:      "XML",
			extensions:      []string{".xml"},
			contentPatterns: []string{"<bible>", "<verse>"},
			wantDetect:      true,
		},
		{
			name:            "extension match, no content patterns",
			path:            xmlFile,
			formatName:      "XML",
			extensions:      []string{".xml"},
			contentPatterns: []string{},
			wantDetect:      true,
		},
		{
			name:            "extension match, content mismatch",
			path:            nonBibleXML,
			formatName:      "XML",
			extensions:      []string{".xml"},
			contentPatterns: []string{"<bible>"},
			wantDetect:      false,
		},
		{
			name:            "extension mismatch",
			path:            txtFile,
			formatName:      "XML",
			extensions:      []string{".xml"},
			contentPatterns: []string{"<bible>"},
			wantDetect:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StandardDetect(tt.path, tt.formatName, tt.extensions, tt.contentPatterns)
			if result.Detected != tt.wantDetect {
				t.Errorf("StandardDetect() Detected = %v, want %v", result.Detected, tt.wantDetect)
			}
			if result.Reason == "" {
				t.Error("StandardDetect() Reason should not be empty")
			}
		})
	}
}

func TestDetectSuccessFailure(t *testing.T) {
	success := DetectSuccess("TestFormat", "test reason")
	if !success.Detected {
		t.Error("DetectSuccess() should set Detected to true")
	}
	if success.Format != "TestFormat" {
		t.Errorf("DetectSuccess() Format = %v, want TestFormat", success.Format)
	}
	if success.Reason != "test reason" {
		t.Errorf("DetectSuccess() Reason = %v, want test reason", success.Reason)
	}

	failure := DetectFailure("failure reason")
	if failure.Detected {
		t.Error("DetectFailure() should set Detected to false")
	}
	if failure.Reason != "failure reason" {
		t.Errorf("DetectFailure() Reason = %v, want failure reason", failure.Reason)
	}
}
