package archive

import (
	"testing"
)

func TestExtractCapsuleID(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"KJV.capsule.tar.xz", "KJV"},
		{"KJV.capsule.tar.gz", "KJV"},
		{"KJV.tar.xz", "KJV"},
		{"KJV.tar.gz", "KJV"},
		{"KJV.tar", "KJV"},
		{"DRC.Bible.capsule.tar.xz", "DRC.Bible"},
		{"my-bible-v2.tar.gz", "my-bible-v2"},
		{"no-extension", "no-extension"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := ExtractCapsuleID(tt.filename)
			if got != tt.want {
				t.Errorf("ExtractCapsuleID(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestExtractIRName(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"KJV.capsule.tar.xz", "KJV.ir.json"},
		{"KJV.tar.gz", "KJV.ir.json"},
		{"DRC.Bible.capsule.tar.xz", "DRC.Bible.ir.json"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := ExtractIRName(tt.filename)
			if got != tt.want {
				t.Errorf("ExtractIRName(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"file.tar.xz", "tar.xz"},
		{"file.tar.gz", "tar.gz"},
		{"file.tar", "tar"},
		{"file.zip", "unknown"},
		{"file", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := DetectFormat(tt.path)
			if got != tt.want {
				t.Errorf("DetectFormat(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsSupportedFormat(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"file.tar.xz", true},
		{"file.tar.gz", true},
		{"file.tar", true},
		{"file.zip", false},
		{"file", false},
		{"file.capsule.tar.xz", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsSupportedFormat(tt.path)
			if got != tt.want {
				t.Errorf("IsSupportedFormat(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsCASCapsule(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  bool
	}{
		{
			name: "CAS capsule with blobs",
			setup: func(t *testing.T) string {
				return createTestTarXz(t, dir)
			},
			want: true,
		},
		{
			name: "non-CAS capsule",
			setup: func(t *testing.T) string {
				return createTestTarGz(t, dir)
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			got := IsCASCapsule(path)
			if got != tt.want {
				t.Errorf("IsCASCapsule() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasIR(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  bool
	}{
		{
			name: "capsule with IR",
			setup: func(t *testing.T) string {
				return createTestTarGz(t, dir)
			},
			want: true,
		},
		{
			name: "capsule without IR",
			setup: func(t *testing.T) string {
				return createTestTarXz(t, dir) // CAS capsule has no IR
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			got := HasIR(path)
			if got != tt.want {
				t.Errorf("HasIR() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadIR(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir)

	ir, err := ReadIR(path)
	if err != nil {
		t.Fatalf("ReadIR() error = %v", err)
	}

	if ir == nil {
		t.Error("ReadIR() returned nil")
	}

	if _, ok := ir["test"]; !ok {
		t.Error("IR missing expected 'test' field")
	}
}

func TestReadIR_NoIRFile(t *testing.T) {
	dir := t.TempDir()
	// Use CAS capsule which has no IR file
	path := createTestTarXz(t, dir)

	_, err := ReadIR(path)
	if err == nil {
		t.Error("ReadIR() expected error for capsule without IR file")
	}
}

func TestReadIR_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGzInvalidIR(t, dir)

	_, err := ReadIR(path)
	if err == nil {
		t.Error("ReadIR() expected error for invalid JSON")
	}
}
