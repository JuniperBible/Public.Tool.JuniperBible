package validation

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizePath(t *testing.T) {
	baseDir := "/tmp/test"

	tests := []struct {
		name      string
		baseDir   string
		userPath  string
		want      string
		wantError error
	}{
		{
			name:      "simple valid path",
			baseDir:   baseDir,
			userPath:  "file.txt",
			want:      "file.txt",
			wantError: nil,
		},
		{
			name:      "nested valid path",
			baseDir:   baseDir,
			userPath:  "subdir/file.txt",
			want:      filepath.Join("subdir", "file.txt"),
			wantError: nil,
		},
		{
			name:      "path with redundant separators",
			baseDir:   baseDir,
			userPath:  "subdir//file.txt",
			want:      filepath.Join("subdir", "file.txt"),
			wantError: nil,
		},
		{
			name:      "path with dot component",
			baseDir:   baseDir,
			userPath:  "./file.txt",
			want:      "file.txt",
			wantError: nil,
		},
		{
			name:      "path traversal with dotdot",
			baseDir:   baseDir,
			userPath:  "../etc/passwd",
			want:      "",
			wantError: ErrPathTraversal,
		},
		{
			name:      "path traversal in middle",
			baseDir:   baseDir,
			userPath:  "subdir/../../etc/passwd",
			want:      "",
			wantError: ErrPathTraversal,
		},
		{
			name:      "absolute path",
			baseDir:   baseDir,
			userPath:  "/etc/passwd",
			want:      "",
			wantError: ErrPathTraversal,
		},
		{
			name:      "empty path",
			baseDir:   baseDir,
			userPath:  "",
			want:      "",
			wantError: ErrEmptyPath,
		},
		{
			name:      "very long path",
			baseDir:   baseDir,
			userPath:  strings.Repeat("a/", 2048) + "file.txt",
			want:      "",
			wantError: ErrPathTooLong,
		},
		{
			name:      "path that would escape after resolution",
			baseDir:   "/tmp/base/subdir",
			userPath:  "a/b/../../../etc/passwd",
			want:      "",
			wantError: ErrPathTraversal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizePath(tt.baseDir, tt.userPath)

			if tt.wantError != nil {
				if err == nil {
					t.Errorf("SanitizePath() expected error %v, got nil", tt.wantError)
					return
				}
				if !errors.Is(err, tt.wantError) && !strings.Contains(err.Error(), tt.wantError.Error()) {
					t.Errorf("SanitizePath() error = %v, want %v", err, tt.wantError)
				}
				return
			}

			if err != nil {
				t.Errorf("SanitizePath() unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("SanitizePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateFilename(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		wantError error
	}{
		{
			name:      "valid simple filename",
			filename:  "file.txt",
			wantError: nil,
		},
		{
			name:      "valid filename with spaces",
			filename:  "my file.txt",
			wantError: nil,
		},
		{
			name:      "valid filename with special chars",
			filename:  "file_name-2024.tar.gz",
			wantError: nil,
		},
		{
			name:      "empty filename",
			filename:  "",
			wantError: ErrInvalidFilename,
		},
		{
			name:      "dot filename",
			filename:  ".",
			wantError: ErrInvalidFilename,
		},
		{
			name:      "dotdot filename",
			filename:  "..",
			wantError: ErrInvalidFilename,
		},
		{
			name:      "filename with slash",
			filename:  "dir/file.txt",
			wantError: ErrInvalidFilename,
		},
		{
			name:      "filename with backslash",
			filename:  "dir\\file.txt",
			wantError: ErrInvalidFilename,
		},
		{
			name:      "filename with null byte",
			filename:  "file\x00.txt",
			wantError: ErrInvalidFilename,
		},
		{
			name:      "filename with control character",
			filename:  "file\n.txt",
			wantError: ErrInvalidFilename,
		},
		{
			name:      "filename starting with hyphen",
			filename:  "-file.txt",
			wantError: ErrInvalidFilename,
		},
		{
			name:      "too long filename",
			filename:  strings.Repeat("a", 256),
			wantError: ErrFilenameTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilename(tt.filename)

			if tt.wantError != nil {
				if err == nil {
					t.Errorf("ValidateFilename() expected error %v, got nil", tt.wantError)
					return
				}
				if !errors.Is(err, tt.wantError) && !strings.Contains(err.Error(), tt.wantError.Error()) {
					t.Errorf("ValidateFilename() error = %v, want %v", err, tt.wantError)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateFilename() unexpected error: %v", err)
			}
		})
	}
}

func TestIsPathSafe(t *testing.T) {
	baseDir := "/tmp/test"

	tests := []struct {
		name     string
		baseDir  string
		userPath string
		want     bool
	}{
		{
			name:     "safe path",
			baseDir:  baseDir,
			userPath: "file.txt",
			want:     true,
		},
		{
			name:     "safe nested path",
			baseDir:  baseDir,
			userPath: "subdir/file.txt",
			want:     true,
		},
		{
			name:     "unsafe path traversal",
			baseDir:  baseDir,
			userPath: "../etc/passwd",
			want:     false,
		},
		{
			name:     "unsafe absolute path",
			baseDir:  baseDir,
			userPath: "/etc/passwd",
			want:     false,
		},
		{
			name:     "empty path",
			baseDir:  baseDir,
			userPath: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPathSafe(tt.baseDir, tt.userPath)
			if got != tt.want {
				t.Errorf("IsPathSafe() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError error
	}{
		{
			name:      "valid relative path",
			path:      "file.txt",
			wantError: nil,
		},
		{
			name:      "valid absolute path",
			path:      "/tmp/file.txt",
			wantError: nil,
		},
		{
			name:      "valid nested path",
			path:      "dir/subdir/file.txt",
			wantError: nil,
		},
		{
			name:      "empty path",
			path:      "",
			wantError: ErrEmptyPath,
		},
		{
			name:      "path with null byte",
			path:      "file\x00.txt",
			wantError: ErrInvalidCharacter,
		},
		{
			name:      "path with control character",
			path:      "dir/file\n.txt",
			wantError: ErrInvalidCharacter,
		},
		{
			name:      "very long path",
			path:      strings.Repeat("a/", 2048) + "file.txt",
			wantError: ErrPathTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)

			if tt.wantError != nil {
				if err == nil {
					t.Errorf("ValidatePath() expected error %v, got nil", tt.wantError)
					return
				}
				if !errors.Is(err, tt.wantError) && !strings.Contains(err.Error(), tt.wantError.Error()) {
					t.Errorf("ValidatePath() error = %v, want %v", err, tt.wantError)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidatePath() unexpected error: %v", err)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		want      string
		wantError error
	}{
		{
			name:      "valid filename unchanged",
			filename:  "file.txt",
			want:      "file.txt",
			wantError: nil,
		},
		{
			name:      "filename with leading/trailing spaces",
			filename:  "  file.txt  ",
			want:      "file.txt",
			wantError: nil,
		},
		{
			name:      "filename with slashes replaced",
			filename:  "dir/file.txt",
			want:      "dir_file.txt",
			wantError: nil,
		},
		{
			name:      "filename with backslashes replaced",
			filename:  "dir\\file.txt",
			want:      "dir_file.txt",
			wantError: nil,
		},
		{
			name:      "filename with null byte removed",
			filename:  "file\x00name.txt",
			want:      "filename.txt",
			wantError: nil,
		},
		{
			name:      "filename with control characters removed",
			filename:  "file\nname\r.txt",
			want:      "filename.txt",
			wantError: nil,
		},
		{
			name:      "filename with leading hyphen removed",
			filename:  "-file.txt",
			want:      "file.txt",
			wantError: nil,
		},
		{
			name:      "empty filename",
			filename:  "",
			want:      "",
			wantError: ErrInvalidFilename,
		},
		{
			name:      "filename that becomes empty after sanitization",
			filename:  "---",
			want:      "",
			wantError: ErrInvalidFilename,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizeFilename(tt.filename)

			if tt.wantError != nil {
				if err == nil {
					t.Errorf("SanitizeFilename() expected error %v, got nil", tt.wantError)
					return
				}
				if !errors.Is(err, tt.wantError) && !strings.Contains(err.Error(), tt.wantError.Error()) {
					t.Errorf("SanitizeFilename() error = %v, want %v", err, tt.wantError)
				}
				return
			}

			if err != nil {
				t.Errorf("SanitizeFilename() unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("SanitizeFilename() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateFileType(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		content      []byte
		wantFileType FileType
		wantError    bool
	}{
		// Archive formats - exact matches
		{
			name:         "tar file with ustar magic",
			filename:     "archive.tar",
			content:      makeTarHeader(),
			wantFileType: FileTypeTar,
			wantError:    false,
		},
		{
			name:         "gzip file",
			filename:     "file.gz",
			content:      []byte{0x1f, 0x8b, 0x08, 0x00},
			wantFileType: FileTypeGzip,
			wantError:    false,
		},
		{
			name:         "xz file",
			filename:     "file.xz",
			content:      []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00},
			wantFileType: FileTypeXZ,
			wantError:    false,
		},
		{
			name:         "zip file",
			filename:     "archive.zip",
			content:      []byte{0x50, 0x4b, 0x03, 0x04},
			wantFileType: FileTypeZip,
			wantError:    false,
		},
		{
			name:         "sqlite file",
			filename:     "database.sqlite",
			content:      []byte("SQLite format 3\x00"),
			wantFileType: FileTypeSQLite,
			wantError:    false,
		},
		// Compressed tar archives
		{
			name:         "tar.xz file with xz magic",
			filename:     "archive.tar.xz",
			content:      []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00},
			wantFileType: FileTypeTarXZ,
			wantError:    false,
		},
		{
			name:         "tar.gz file with gzip magic",
			filename:     "archive.tar.gz",
			content:      []byte{0x1f, 0x8b, 0x08, 0x00},
			wantFileType: FileTypeTarGZ,
			wantError:    false,
		},
		{
			name:         "tgz file with gzip magic",
			filename:     "archive.tgz",
			content:      []byte{0x1f, 0x8b, 0x08, 0x00},
			wantFileType: FileTypeTarGZ,
			wantError:    false,
		},
		// Text formats
		{
			name:         "xml file",
			filename:     "document.xml",
			content:      []byte("<?xml version=\"1.0\"?>\n<root></root>"),
			wantFileType: FileTypeXML,
			wantError:    false,
		},
		{
			name:         "json file",
			filename:     "data.json",
			content:      []byte(`{"key": "value"}`),
			wantFileType: FileTypeJSON,
			wantError:    false,
		},
		{
			name:         "text file",
			filename:     "document.txt",
			content:      []byte("This is plain text content\nWith multiple lines"),
			wantFileType: FileTypeText,
			wantError:    false,
		},
		{
			name:         "osis file",
			filename:     "bible.osis",
			content:      []byte("<?xml version=\"1.0\"?>\n<osis></osis>"),
			wantFileType: FileTypeXML,
			wantError:    false,
		},
		{
			name:         "usfm file",
			filename:     "bible.usfm",
			content:      []byte("\\id GEN\n\\c 1\n\\v 1 In the beginning..."),
			wantFileType: FileTypeText,
			wantError:    false,
		},
		// Edge cases
		{
			name:         "unknown extension with no magic",
			filename:     "file.unknown",
			content:      []byte("random content"),
			wantFileType: FileTypeUnknown,
			wantError:    false,
		},
		{
			name:         "type mismatch - claims zip but is tar",
			filename:     "fake.zip",
			content:      makeTarHeader(),
			wantFileType: FileTypeUnknown,
			wantError:    true,
		},
		{
			name:         "type mismatch - claims sqlite but is zip",
			filename:     "fake.sqlite",
			content:      []byte{0x50, 0x4b, 0x03, 0x04},
			wantFileType: FileTypeUnknown,
			wantError:    true,
		},
		{
			name:         "empty file",
			filename:     "empty.txt",
			content:      []byte{},
			wantFileType: FileTypeText,
			wantError:    false,
		},
		{
			name:         "small file less than 512 bytes",
			filename:     "small.txt",
			content:      []byte("small"),
			wantFileType: FileTypeText,
			wantError:    false,
		},
		{
			name:         "binary content with text extension - falls back to expected",
			filename:     "fake.txt",
			content:      append([]byte("text"), bytes.Repeat([]byte{0x01, 0x02, 0x03}, 50)...),
			wantFileType: FileTypeText,
			wantError:    false,
		},
		{
			name:         "db extension for sqlite",
			filename:     "database.db",
			content:      []byte("SQLite format 3\x00"),
			wantFileType: FileTypeSQLite,
			wantError:    false,
		},
		{
			name:         "sqlite3 extension",
			filename:     "database.sqlite3",
			content:      []byte("SQLite format 3\x00"),
			wantFileType: FileTypeSQLite,
			wantError:    false,
		},
		{
			name:         "markdown file",
			filename:     "readme.md",
			content:      []byte("# Heading\n\nThis is markdown."),
			wantFileType: FileTypeText,
			wantError:    false,
		},
		{
			name:         "zefania xml file",
			filename:     "bible.zefania",
			content:      []byte("<?xml version=\"1.0\"?>\n<XMLBIBLE></XMLBIBLE>"),
			wantFileType: FileTypeXML,
			wantError:    false,
		},
		{
			name:         "usx xml file",
			filename:     "bible.usx",
			content:      []byte("<?xml version=\"1.0\"?>\n<usx></usx>"),
			wantFileType: FileTypeXML,
			wantError:    false,
		},
		{
			name:         "sfm text file",
			filename:     "bible.sfm",
			content:      []byte("\\id GEN\n\\c 1"),
			wantFileType: FileTypeText,
			wantError:    false,
		},
		{
			name:         "detected type is not unknown, expected is unknown",
			filename:     "file.bin",
			content:      []byte{0x1f, 0x8b, 0x08, 0x00},
			wantFileType: FileTypeGzip,
			wantError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(string(tt.content))
			gotFileType, err := ValidateFileType(reader, tt.filename)

			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateFileType() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateFileType() unexpected error: %v", err)
				return
			}

			if gotFileType != tt.wantFileType {
				t.Errorf("ValidateFileType() = %v, want %v", gotFileType, tt.wantFileType)
			}
		})
	}
}

// makeTarHeader creates a minimal tar header with ustar magic at offset 257
func makeTarHeader() []byte {
	buf := make([]byte, 512)
	copy(buf[257:], []byte("ustar"))
	return buf
}

// errorReader is a reader that always returns an error
type errorReader struct{}

func (e errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

func TestValidateFileType_ReadError(t *testing.T) {
	reader := errorReader{}
	_, err := ValidateFileType(reader, "test.txt")
	if err == nil {
		t.Error("ValidateFileType() expected error from reader, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read file header") {
		t.Errorf("ValidateFileType() error = %v, want error about reading file header", err)
	}
}

func TestDetectFileTypeFromMagic(t *testing.T) {
	tests := []struct {
		name         string
		content      []byte
		wantFileType FileType
	}{
		{
			name:         "tar magic at offset 257",
			content:      makeTarHeader(),
			wantFileType: FileTypeTar,
		},
		{
			name:         "gzip magic",
			content:      []byte{0x1f, 0x8b},
			wantFileType: FileTypeGzip,
		},
		{
			name:         "xz magic",
			content:      []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00},
			wantFileType: FileTypeXZ,
		},
		{
			name:         "zip magic",
			content:      []byte{0x50, 0x4b, 0x03, 0x04},
			wantFileType: FileTypeZip,
		},
		{
			name:         "sqlite magic",
			content:      []byte("SQLite format 3"),
			wantFileType: FileTypeSQLite,
		},
		{
			name:         "unknown magic",
			content:      []byte("random content"),
			wantFileType: FileTypeUnknown,
		},
		{
			name:         "empty buffer",
			content:      []byte{},
			wantFileType: FileTypeUnknown,
		},
		{
			name:         "partial magic bytes",
			content:      []byte{0x1f},
			wantFileType: FileTypeUnknown,
		},
		{
			name:         "buffer too small for tar",
			content:      make([]byte, 256),
			wantFileType: FileTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectFileTypeFromMagic(tt.content)
			if got != tt.wantFileType {
				t.Errorf("detectFileTypeFromMagic() = %v, want %v", got, tt.wantFileType)
			}
		})
	}
}

func TestDetectFileTypeFromExtension(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		wantFileType FileType
	}{
		// Multi-extension formats
		{
			name:         "tar.xz extension",
			filename:     "archive.tar.xz",
			wantFileType: FileTypeTarXZ,
		},
		{
			name:         "tar.gz extension",
			filename:     "archive.tar.gz",
			wantFileType: FileTypeTarGZ,
		},
		{
			name:         "tgz extension",
			filename:     "archive.tgz",
			wantFileType: FileTypeTarGZ,
		},
		// Single extension formats
		{
			name:         "tar extension",
			filename:     "archive.tar",
			wantFileType: FileTypeTar,
		},
		{
			name:         "xz extension",
			filename:     "file.xz",
			wantFileType: FileTypeXZ,
		},
		{
			name:         "gz extension",
			filename:     "file.gz",
			wantFileType: FileTypeGzip,
		},
		{
			name:         "zip extension",
			filename:     "archive.zip",
			wantFileType: FileTypeZip,
		},
		{
			name:         "sqlite extension",
			filename:     "database.sqlite",
			wantFileType: FileTypeSQLite,
		},
		{
			name:         "db extension",
			filename:     "database.db",
			wantFileType: FileTypeSQLite,
		},
		{
			name:         "sqlite3 extension",
			filename:     "database.sqlite3",
			wantFileType: FileTypeSQLite,
		},
		{
			name:         "xml extension",
			filename:     "document.xml",
			wantFileType: FileTypeXML,
		},
		{
			name:         "osis extension",
			filename:     "bible.osis",
			wantFileType: FileTypeXML,
		},
		{
			name:         "usx extension",
			filename:     "bible.usx",
			wantFileType: FileTypeXML,
		},
		{
			name:         "zefania extension",
			filename:     "bible.zefania",
			wantFileType: FileTypeXML,
		},
		{
			name:         "json extension",
			filename:     "data.json",
			wantFileType: FileTypeJSON,
		},
		{
			name:         "txt extension",
			filename:     "file.txt",
			wantFileType: FileTypeText,
		},
		{
			name:         "usfm extension",
			filename:     "bible.usfm",
			wantFileType: FileTypeText,
		},
		{
			name:         "sfm extension",
			filename:     "bible.sfm",
			wantFileType: FileTypeText,
		},
		{
			name:         "md extension",
			filename:     "readme.md",
			wantFileType: FileTypeText,
		},
		{
			name:         "unknown extension",
			filename:     "file.unknown",
			wantFileType: FileTypeUnknown,
		},
		{
			name:         "no extension",
			filename:     "file",
			wantFileType: FileTypeUnknown,
		},
		{
			name:         "uppercase extension",
			filename:     "ARCHIVE.TAR.GZ",
			wantFileType: FileTypeTarGZ,
		},
		{
			name:         "mixed case extension",
			filename:     "Archive.Tar.Xz",
			wantFileType: FileTypeTarXZ,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectFileTypeFromExtension(tt.filename)
			if got != tt.wantFileType {
				t.Errorf("detectFileTypeFromExtension() = %v, want %v", got, tt.wantFileType)
			}
		})
	}
}

func TestIsLikelyText(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    bool
	}{
		{
			name:    "plain ascii text",
			content: []byte("This is plain ASCII text."),
			want:    true,
		},
		{
			name:    "text with newlines",
			content: []byte("Line 1\nLine 2\nLine 3"),
			want:    true,
		},
		{
			name:    "text with tabs",
			content: []byte("Column1\tColumn2\tColumn3"),
			want:    true,
		},
		{
			name:    "text with carriage returns",
			content: []byte("Windows\r\nLine\r\nEndings"),
			want:    true,
		},
		{
			name:    "text with mixed whitespace",
			content: []byte("Text\t\twith\n\r\nspaces"),
			want:    true,
		},
		{
			name:    "xml content",
			content: []byte("<?xml version=\"1.0\"?>\n<root></root>"),
			want:    true,
		},
		{
			name:    "json content",
			content: []byte(`{"key": "value", "number": 123}`),
			want:    true,
		},
		{
			name:    "utf-8 text",
			content: []byte("Hello 世界 🌍"),
			want:    true,
		},
		{
			name:    "binary with null bytes",
			content: []byte{0x00, 0x01, 0x02, 0x03},
			want:    false,
		},
		{
			name:    "binary with control characters",
			content: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			want:    false,
		},
		{
			name:    "mixed binary and text",
			content: append([]byte("Text"), 0x00, 0x01, 0x02),
			want:    false,
		},
		{
			name:    "empty buffer",
			content: []byte{},
			want:    false,
		},
		{
			name:    "mostly printable with few control chars - above threshold",
			content: append([]byte(strings.Repeat("a", 96)), []byte{0x01, 0x02, 0x03, 0x04}...),
			want:    true,
		},
		{
			name:    "mostly printable but below 95% threshold",
			content: append([]byte(strings.Repeat("a", 94)), []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}...),
			want:    false,
		},
		{
			name:    "utf-8 continuation bytes",
			content: []byte("Test UTF-8: \xc3\xa9\xc3\xa8\xc3\xa0"),
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLikelyText(tt.content)
			if got != tt.want {
				t.Errorf("isLikelyText() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Benchmark tests
func BenchmarkSanitizePath(b *testing.B) {
	baseDir := "/tmp/test"
	userPath := "subdir/file.txt"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SanitizePath(baseDir, userPath)
	}
}

func BenchmarkValidateFilename(b *testing.B) {
	filename := "valid_filename.txt"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateFilename(filename)
	}
}

func BenchmarkSanitizeFilename(b *testing.B) {
	filename := "file-with-special_chars.txt"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SanitizeFilename(filename)
	}
}
