package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/validation"
)

func TestValidatePath(t *testing.T) {
	// Create temp directory for tests
	tmpDir, err := os.MkdirTemp("", "security-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		baseDir  string
		userPath string
		wantErr  bool
		errType  error
	}{
		{
			name:     "valid relative path",
			baseDir:  tmpDir,
			userPath: "capsule.tar.gz",
			wantErr:  false,
		},
		{
			name:     "valid nested path",
			baseDir:  tmpDir,
			userPath: "subdir/capsule.tar.gz",
			wantErr:  false,
		},
		{
			name:     "path traversal with ..",
			baseDir:  tmpDir,
			userPath: "../etc/passwd",
			wantErr:  true,
			errType:  validation.ErrPathTraversal,
		},
		{
			name:     "path traversal with ../ prefix",
			baseDir:  tmpDir,
			userPath: "../../secret.txt",
			wantErr:  true,
			errType:  validation.ErrPathTraversal,
		},
		{
			name:     "path traversal in middle",
			baseDir:  tmpDir,
			userPath: "safe/../../../etc/passwd",
			wantErr:  true,
			errType:  validation.ErrPathTraversal,
		},
		{
			name:     "absolute path",
			baseDir:  tmpDir,
			userPath: "/etc/passwd",
			wantErr:  true,
			errType:  validation.ErrPathTraversal,
		},
		{
			name:     "empty path",
			baseDir:  tmpDir,
			userPath: "",
			wantErr:  true,
			errType:  validation.ErrEmptyPath,
		},
		{
			name:     "path with null byte",
			baseDir:  tmpDir,
			userPath: "file\x00.txt",
			wantErr:  false, // SanitizePath doesn't check null bytes; OS will reject them
		},
		{
			name:     "path with encoded null byte",
			baseDir:  tmpDir,
			userPath: "file%00.txt",
			wantErr:  false, // URL decoding should happen before this
		},
		{
			name:     "windows-style path traversal",
			baseDir:  tmpDir,
			userPath: "..\\..\\windows\\system32",
			wantErr:  true,
			errType:  validation.ErrPathTraversal,
		},
		{
			name:     "unicode normalization attack",
			baseDir:  tmpDir,
			userPath: "file\u202E.txt", // Right-to-left override
			wantErr:  false,            // This is technically valid, but might be suspicious
		},
		{
			name:     "very long path",
			baseDir:  tmpDir,
			userPath: string(make([]byte, 5000)),
			wantErr:  true,
			errType:  validation.ErrPathTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanPath, err := ValidatePath(tt.baseDir, tt.userPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.errType != nil && err != tt.errType {
				if err == nil || err.Error() != tt.errType.Error() {
					// Allow wrapped errors
					if !containsError(err, tt.errType) {
						t.Errorf("ValidatePath() error = %v, want error type %v", err, tt.errType)
					}
				}
			}

			if !tt.wantErr {
				// Verify the clean path doesn't escape the base directory
				fullPath := filepath.Join(tt.baseDir, cleanPath)
				absBase, _ := filepath.Abs(tt.baseDir)
				absPath, _ := filepath.Abs(fullPath)
				relPath, err := filepath.Rel(absBase, absPath)
				if err != nil || filepath.IsAbs(relPath) || len(relPath) > 0 && relPath[0] == '.' && len(relPath) > 1 && relPath[1] == '.' {
					t.Errorf("ValidatePath() returned path that escapes base directory: %s", cleanPath)
				}
			}
		})
	}
}

// containsError checks if an error contains or wraps another error
func containsError(err, target error) bool {
	if err == nil || target == nil {
		return false
	}
	return err == target || (err.Error() != "" && target.Error() != "" &&
		(err.Error() == target.Error() || len(err.Error()) > len(target.Error()) &&
			err.Error()[:len(target.Error())] == target.Error()))
}

func TestValidateCapsulePath(t *testing.T) {
	// Set up a test capsules directory
	tmpDir, err := os.MkdirTemp("", "capsule-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Save original config and restore after test
	origCapsulesDir := ServerConfig.CapsulesDir
	defer func() { ServerConfig.CapsulesDir = origCapsulesDir }()
	ServerConfig.CapsulesDir = tmpDir

	tests := []struct {
		name        string
		capsulePath string
		wantErr     bool
	}{
		{
			name:        "valid capsule",
			capsulePath: "KJV.tar.gz",
			wantErr:     false,
		},
		{
			name:        "path traversal",
			capsulePath: "../../../etc/passwd",
			wantErr:     true,
		},
		{
			name:        "absolute path",
			capsulePath: "/etc/passwd",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateCapsulePath(tt.capsulePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCapsulePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsPathSafe(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "safe-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		baseDir  string
		userPath string
		want     bool
	}{
		{
			name:     "safe path",
			baseDir:  tmpDir,
			userPath: "safe.txt",
			want:     true,
		},
		{
			name:     "unsafe path with ..",
			baseDir:  tmpDir,
			userPath: "../unsafe.txt",
			want:     false,
		},
		{
			name:     "absolute path",
			baseDir:  tmpDir,
			userPath: "/etc/passwd",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPathSafe(tt.baseDir, tt.userPath); got != tt.want {
				t.Errorf("IsPathSafe() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizePathForDisplay(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "normal path",
			path: "capsules/KJV.tar.gz",
			want: "capsules/KJV.tar.gz",
		},
		{
			name: "path with null byte",
			path: "file\x00.txt",
			want: "file.txt",
		},
		{
			name: "path with ..",
			path: "../etc/passwd",
			want: "[INVALID_PATH]",
		},
		{
			name: "path with multiple ..",
			path: "../../secret",
			want: "[INVALID_PATH]",
		},
		{
			name: "clean path with redundant separators",
			path: "capsules//subdir///file.txt",
			want: "capsules/subdir/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizePathForDisplay(tt.path); got != tt.want {
				t.Errorf("SanitizePathForDisplay() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateArtifactID(t *testing.T) {
	tests := []struct {
		name       string
		artifactID string
		wantErr    bool
	}{
		{
			name:       "valid artifact ID",
			artifactID: "genesis.osis",
			wantErr:    false,
		},
		{
			name:       "artifact with path separator",
			artifactID: "subdir/file.osis",
			wantErr:    true,
		},
		{
			name:       "artifact with ..",
			artifactID: "../file.osis",
			wantErr:    true,
		},
		{
			name:       "empty artifact ID",
			artifactID: "",
			wantErr:    true,
		},
		{
			name:       "artifact with null byte",
			artifactID: "file\x00.osis",
			wantErr:    true,
		},
		{
			name:       "artifact starting with hyphen",
			artifactID: "-file.osis",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArtifactID(tt.artifactID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArtifactID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCapsuleID(t *testing.T) {
	tests := []struct {
		name      string
		capsuleID string
		wantErr   bool
	}{
		{
			name:      "valid capsule ID",
			capsuleID: "KJV",
			wantErr:   false,
		},
		{
			name:      "capsule ID with extension",
			capsuleID: "KJV.tar.gz",
			wantErr:   false,
		},
		{
			name:      "capsule ID with slash",
			capsuleID: "dir/KJV",
			wantErr:   true,
		},
		{
			name:      "capsule ID with backslash",
			capsuleID: "dir\\KJV",
			wantErr:   true,
		},
		{
			name:      "capsule ID with ..",
			capsuleID: "../KJV",
			wantErr:   true,
		},
		{
			name:      "empty capsule ID",
			capsuleID: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCapsuleID(tt.capsuleID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCapsuleID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Benchmark tests to ensure validation doesn't add significant overhead
func BenchmarkValidatePath(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench-*")
	defer os.RemoveAll(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidatePath(tmpDir, "capsules/test.tar.gz")
	}
}

func BenchmarkValidatePathWithTraversal(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench-*")
	defer os.RemoveAll(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidatePath(tmpDir, "../../../etc/passwd")
	}
}

func BenchmarkIsPathSafe(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench-*")
	defer os.RemoveAll(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsPathSafe(tmpDir, "capsules/test.tar.gz")
	}
}
