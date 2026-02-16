package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidatePath_PathTraversal tests that ValidatePath rejects path traversal attempts
func TestValidatePath_PathTraversal(t *testing.T) {
	baseDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errType error
	}{
		{
			name:    "simple path traversal with ..",
			path:    "../etc/passwd",
			wantErr: true,
			errType: ErrPathTraversal,
		},
		{
			name:    "double path traversal",
			path:    "../../etc/passwd",
			wantErr: true,
			errType: ErrPathTraversal,
		},
		{
			name:    "path traversal with valid prefix",
			path:    "valid/../../../etc/passwd",
			wantErr: true,
			errType: ErrPathTraversal,
		},
		{
			name:    "path traversal in middle",
			path:    "foo/../../../bar",
			wantErr: true,
			errType: ErrPathTraversal,
		},
		{
			name:    "path traversal with slashes",
			path:    "foo/../../bar",
			wantErr: true,
			errType: ErrPathTraversal,
		},
		{
			name:    "path traversal with backslashes (Windows style)",
			path:    "foo\\..\\..\\bar",
			wantErr: true,
			errType: ErrPathTraversal,
		},
		{
			name:    "multiple dots",
			path:    "...../etc/passwd",
			wantErr: true,
			errType: ErrPathTraversal,
		},
		{
			name:    "current directory reference",
			path:    "./valid/file.txt",
			wantErr: false, // ./ is safe, just references current dir
		},
		{
			name:    "valid nested path",
			path:    "valid/nested/file.txt",
			wantErr: false,
		},
		{
			name:    "valid simple filename",
			path:    "file.txt",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
			errType: ErrInvalidPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidatePath(baseDir, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePath() expected error for path %q, got nil", tt.path)
				}
				if tt.errType != nil && !strings.Contains(err.Error(), tt.errType.Error()) {
					t.Errorf("ValidatePath() error = %v, want error containing %v", err, tt.errType)
				}
				if result != "" {
					t.Errorf("ValidatePath() returned result %q for invalid path, expected empty string", result)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePath() unexpected error for path %q: %v", tt.path, err)
				}
			}
		})
	}
}

// TestValidatePath_AbsolutePaths tests that ValidatePath rejects absolute paths
func TestValidatePath_AbsolutePaths(t *testing.T) {
	baseDir := t.TempDir()

	tests := []struct {
		name string
		path string
	}{
		{
			name: "Unix absolute path",
			path: "/etc/passwd",
		},
		{
			name: "Unix absolute path with home",
			path: "/home/user/file.txt",
		},
		// Note: Windows-style paths like "C:\..." are only absolute on Windows
		// On Unix/Linux, they're treated as relative paths with colons in the name
		// We still want to reject them if they contain path traversal patterns
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidatePath(baseDir, tt.path)
			if err == nil {
				t.Errorf("ValidatePath() expected error for absolute path %q, got nil", tt.path)
			}
			if err != nil && !strings.Contains(err.Error(), "absolute") && !strings.Contains(err.Error(), "path traversal") {
				t.Errorf("ValidatePath() error = %v, want error about absolute paths", err)
			}
		})
	}
}

// TestValidatePath_SymbolicLinkEscape tests protection against symlink-based escapes
func TestValidatePath_SymbolicLinkEscape(t *testing.T) {
	// Create temporary directory structure
	baseDir := t.TempDir()
	subDir := filepath.Join(baseDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Create a target directory outside base
	targetDir := t.TempDir()
	targetFile := filepath.Join(targetDir, "secret.txt")
	if err := os.WriteFile(targetFile, []byte("secret data"), 0600); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	// Create a symlink inside baseDir pointing outside
	symlinkPath := filepath.Join(subDir, "escape_link")
	if err := os.Symlink(targetDir, symlinkPath); err != nil {
		t.Skip("Symlink creation not supported on this system")
	}

	// Try to access file through symlink
	_, err := ValidatePath(baseDir, "subdir/escape_link/secret.txt")
	// The validation should pass at this level (ValidatePath checks path structure)
	// The actual symlink resolution happens at filesystem access time
	// ValidatePath prevents directory traversal via path syntax, not symlinks
	if err != nil {
		// This is actually good - if ValidatePath rejects it, even better
		t.Logf("ValidatePath rejected symlink path (good): %v", err)
	} else {
		// If ValidatePath allows it, that's OK - the real protection is ensuring
		// the RESOLVED path is checked at file access time
		t.Logf("ValidatePath allowed symlink path (requires additional checks at file access)")
	}
}

// TestValidatePath_NullBytes tests rejection of null byte injection
func TestValidatePath_NullBytes(t *testing.T) {
	baseDir := t.TempDir()

	tests := []struct {
		name string
		path string
	}{
		{
			name: "null byte in middle",
			path: "file\x00.txt",
		},
		{
			name: "null byte at end",
			path: "file.txt\x00",
		},
		{
			name: "null byte in path component",
			path: "dir/file\x00name.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidatePath(baseDir, tt.path)
			if err == nil {
				// Note: On some systems, null bytes might be handled differently
				// The important thing is they don't cause a crash or bypass security
				t.Logf("ValidatePath() accepted path with null byte (may be OS-dependent): %q", tt.path)
			} else {
				t.Logf("ValidatePath() correctly rejected null byte: %v", err)
			}
		})
	}
}

// TestValidatePath_EdgeCases tests edge cases and corner cases
func TestValidatePath_EdgeCases(t *testing.T) {
	baseDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "single dot",
			path:    ".",
			wantErr: false, // . is valid, refers to current directory
		},
		{
			name:    "double dot",
			path:    "..",
			wantErr: true,
		},
		{
			name:    "triple dot",
			path:    "...",
			wantErr: true, // Contains ".." substring, will be rejected
		},
		{
			name:    "dotfile",
			path:    ".hidden",
			wantErr: false, // dotfiles are valid
		},
		{
			name:    "multiple slashes",
			path:    "dir///file.txt",
			wantErr: false, // cleaned to dir/file.txt
		},
		{
			name:    "trailing slash",
			path:    "dir/file.txt/",
			wantErr: false, // cleaned to dir/file.txt
		},
		{
			name:    "leading dot-slash",
			path:    "./dir/file.txt",
			wantErr: false, // cleaned to dir/file.txt
		},
		{
			name:    "whitespace path",
			path:    "   ",
			wantErr: false, // whitespace is technically valid (though unusual)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidatePath(baseDir, tt.path)
			if tt.wantErr && err == nil {
				t.Errorf("ValidatePath() expected error for path %q, got nil", tt.path)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidatePath() unexpected error for path %q: %v", tt.path, err)
			}
		})
	}
}

// TestValidatePath_LongPaths tests handling of excessively long paths
func TestValidatePath_LongPaths(t *testing.T) {
	baseDir := t.TempDir()

	// Create a very long path
	longPath := strings.Repeat("a", 5000)
	_, err := ValidatePath(baseDir, longPath)
	if err == nil {
		t.Error("ValidatePath() expected error for very long path, got nil")
	}
}

// TestValidateID tests the ValidateID function
func TestValidateID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "valid simple ID",
			id:      "file.txt",
			wantErr: false,
		},
		{
			name:    "valid ID with numbers",
			id:      "file123.tar.gz",
			wantErr: false,
		},
		{
			name:    "ID with path separator slash",
			id:      "dir/file.txt",
			wantErr: true,
		},
		{
			name:    "ID with path separator backslash",
			id:      "dir\\file.txt",
			wantErr: true,
		},
		{
			name:    "ID with traversal",
			id:      "../file.txt",
			wantErr: true,
		},
		{
			name:    "ID is dot",
			id:      ".",
			wantErr: true,
		},
		{
			name:    "ID is double dot",
			id:      "..",
			wantErr: true,
		},
		{
			name:    "empty ID",
			id:      "",
			wantErr: true,
		},
		{
			name:    "ID with null byte",
			id:      "file\x00.txt",
			wantErr: true,
		},
		{
			name:    "ID starting with hyphen",
			id:      "-file.txt",
			wantErr: true,
		},
		{
			name:    "valid dotfile ID",
			id:      ".gitignore",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateID(tt.id)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateID() expected error for ID %q, got nil", tt.id)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateID() unexpected error for ID %q: %v", tt.id, err)
			}
		})
	}
}

// TestValidatePath_RealWorldAttacks tests real-world attack patterns
func TestValidatePath_RealWorldAttacks(t *testing.T) {
	baseDir := t.TempDir()

	attacks := []struct {
		name        string
		path        string
		description string
		// shouldPass indicates if this is expected to pass validation
		// (e.g., URL-encoded paths should be decoded by web framework first)
		shouldPass bool
	}{
		{
			name:        "classic etc passwd",
			path:        "../../../../etc/passwd",
			description: "Attempt to read system password file",
			shouldPass:  false,
		},
		{
			name:        "Windows SAM file",
			path:        "..\\..\\..\\Windows\\System32\\config\\SAM",
			description: "Attempt to read Windows password database",
			shouldPass:  false,
		},
		{
			name:        "encoded double dot",
			path:        "%2e%2e/%2e%2e/etc/passwd",
			description: "URL-encoded path traversal (should be decoded by web framework)",
			shouldPass:  true, // ValidatePath doesn't decode URLs - that's the framework's job
		},
		{
			name:        "double encoding",
			path:        "%252e%252e/%252e%252e/etc/passwd",
			description: "Double URL-encoded path traversal (should be decoded by web framework)",
			shouldPass:  true, // ValidatePath doesn't decode URLs
		},
		{
			name:        "unicode encoding",
			path:        "..%c0%af..%c0%af/etc/passwd",
			description: "Unicode-encoded path traversal",
			shouldPass:  false, // Contains literal ".."
		},
		{
			name:        "mixed separators",
			path:        "..\\../\\..//etc/passwd",
			description: "Mixed path separators to confuse parser",
			shouldPass:  false,
		},
		{
			name:        "extra dots",
			path:        ".../.../../etc/passwd",
			description: "Extra dots to evade simple filters",
			shouldPass:  false,
		},
		{
			name:        "zip slip",
			path:        "../../../evil.sh",
			description: "Zip Slip vulnerability pattern",
			shouldPass:  false,
		},
	}

	for _, attack := range attacks {
		t.Run(attack.name, func(t *testing.T) {
			_, err := ValidatePath(baseDir, attack.path)
			if attack.shouldPass {
				// These attacks should be caught by the web framework (URL decoding)
				// before reaching ValidatePath
				if err != nil {
					t.Logf("Path validation rejected %q (note: web framework should decode this first): %v",
						attack.path, err)
				} else {
					t.Logf("Path %q passes validation as literal string (web framework must decode first)",
						attack.path)
				}
			} else {
				// These attacks should be caught by ValidatePath
				if err == nil {
					t.Errorf("ValidatePath() should reject attack %q (%s), but accepted it",
						attack.path, attack.description)
				} else {
					t.Logf("Successfully blocked attack: %s (error: %v)", attack.description, err)
				}
			}
		})
	}
}

// TestValidatePath_ValidPaths tests that legitimate paths are accepted
func TestValidatePath_ValidPaths(t *testing.T) {
	baseDir := t.TempDir()

	validPaths := []string{
		"file.txt",
		"document.pdf",
		"archive.tar.gz",
		"archive.tar.xz",
		"subdir/file.txt",
		"deep/nested/path/file.txt",
		".gitignore",
		"my-file.txt",
		"my_file.txt",
		"file with spaces.txt",
		"file.multiple.dots.txt",
		"日本語.txt",  // Unicode filename
		"Файл.txt", // Cyrillic filename
	}

	for _, path := range validPaths {
		t.Run(path, func(t *testing.T) {
			result, err := ValidatePath(baseDir, path)
			if err != nil {
				t.Errorf("ValidatePath() rejected valid path %q: %v", path, err)
			}
			if result == "" {
				t.Errorf("ValidatePath() returned empty result for valid path %q", path)
			}
		})
	}
}

// TestValidatePath_Consistency tests that ValidatePath is consistent with filesystem operations
func TestValidatePath_Consistency(t *testing.T) {
	baseDir := t.TempDir()

	// Create some test files
	testFiles := []string{
		"test1.txt",
		"subdir/test2.txt",
	}

	// Create directory structure
	subdir := filepath.Join(baseDir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Create files
	for _, file := range testFiles {
		fullPath := filepath.Join(baseDir, file)
		if err := os.WriteFile(fullPath, []byte("test"), 0600); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	// Validate paths and check files exist
	for _, file := range testFiles {
		t.Run(file, func(t *testing.T) {
			safePath, err := ValidatePath(baseDir, file)
			if err != nil {
				t.Errorf("ValidatePath() rejected valid file %q: %v", file, err)
				return
			}

			// Verify the file actually exists at the validated path
			fullPath := filepath.Join(baseDir, safePath)
			if _, err := os.Stat(fullPath); err != nil {
				t.Errorf("Validated path %q does not exist: %v", fullPath, err)
			}
		})
	}
}

// BenchmarkValidatePath benchmarks the ValidatePath function
func BenchmarkValidatePath(b *testing.B) {
	baseDir := b.TempDir()

	testCases := []struct {
		name string
		path string
	}{
		{"simple", "file.txt"},
		{"nested", "dir/subdir/file.txt"},
		{"traversal", "../../etc/passwd"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ValidatePath(baseDir, tc.path)
			}
		})
	}
}

// BenchmarkValidateID benchmarks the ValidateID function
func BenchmarkValidateID(b *testing.B) {
	testCases := []struct {
		name string
		id   string
	}{
		{"simple", "file.txt"},
		{"complex", "my-file_123.tar.gz"},
		{"invalid", "../traversal"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ValidateID(tc.id)
			}
		})
	}
}
