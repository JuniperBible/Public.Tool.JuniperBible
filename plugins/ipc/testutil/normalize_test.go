// Package testutil provides test helpers for IPC protocol testing.
package testutil

import (
	"testing"
)

func TestNormalizeJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "sorts object keys",
			input:    `{"z": 1, "a": 2, "m": 3}`,
			expected: `{"a":2,"m":3,"z":1}`,
		},
		{
			name:     "replaces ISO timestamp",
			input:    `{"time": "2024-01-15T10:30:00Z"}`,
			expected: `{"time":"<TIMESTAMP>"}`,
		},
		{
			name:     "replaces timestamp with milliseconds",
			input:    `{"time": "2024-01-15T10:30:00.123Z"}`,
			expected: `{"time":"<TIMESTAMP>"}`,
		},
		{
			name:     "replaces timestamp with timezone offset",
			input:    `{"time": "2024-01-15T10:30:00+05:30"}`,
			expected: `{"time":"<TIMESTAMP>"}`,
		},
		{
			name:     "replaces UUID v4",
			input:    `{"id": "550e8400-e29b-41d4-a716-446655440000"}`,
			expected: `{"id":"<UUID>"}`,
		},
		{
			name:     "replaces temp path /tmp",
			input:    `{"path": "/tmp/test-123/file.txt"}`,
			expected: `{"path":"<TEMP_PATH>"}`,
		},
		{
			name:     "replaces macOS temp path",
			input:    `{"path": "/var/folders/xx/yy/T/test/file.txt"}`,
			expected: `{"path":"<TEMP_PATH>"}`,
		},
		{
			name:     "replaces Windows temp path",
			input:    `{"path": "C:\\Users\\test\\AppData\\Local\\Temp\\file.txt"}`,
			expected: `{"path":"<TEMP_PATH>"}`,
		},
		{
			name:     "handles nested objects",
			input:    `{"outer": {"inner": "2024-01-15T10:30:00Z"}}`,
			expected: `{"outer":{"inner":"<TIMESTAMP>"}}`,
		},
		{
			name:     "handles arrays",
			input:    `{"items": ["2024-01-15T10:30:00Z", "550e8400-e29b-41d4-a716-446655440000"]}`,
			expected: `{"items":["<TIMESTAMP>","<UUID>"]}`,
		},
		{
			name:     "preserves non-matching strings",
			input:    `{"name": "test", "value": 123}`,
			expected: `{"name":"test","value":123}`,
		},
		{
			name:     "handles empty object",
			input:    `{}`,
			expected: `{}`,
		},
		{
			name:     "handles empty array",
			input:    `{"items": []}`,
			expected: `{"items":[]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeJSONString(tt.input)
			if err != nil {
				t.Fatalf("NormalizeJSONString() error = %v", err)
			}

			// Compare without whitespace for simplicity
			resultCompact, _ := NormalizeJSONString(result)
			expectedCompact, _ := NormalizeJSONString(tt.expected)

			if resultCompact != expectedCompact {
				t.Errorf("NormalizeJSONString() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNormalizeJSON_InvalidJSON(t *testing.T) {
	_, err := NormalizeJSON([]byte("not valid json"))
	if err == nil {
		t.Error("NormalizeJSON() expected error for invalid JSON")
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "unix path unchanged",
			input:    "/home/user/file.txt",
			expected: "/home/user/file.txt",
		},
		{
			name:     "replaces temp path",
			input:    "/tmp/test-123/file.txt",
			expected: "<TEMP_PATH>",
		},
		{
			name:     "windows temp path",
			input:    "C:\\Users\\test\\AppData\\Local\\Temp\\file.txt",
			expected: "<TEMP_PATH>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizePath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCompareNormalizedJSON(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{
			name:     "identical objects",
			a:        `{"a": 1, "b": 2}`,
			b:        `{"b": 2, "a": 1}`,
			expected: true,
		},
		{
			name:     "timestamps normalized",
			a:        `{"time": "2024-01-15T10:30:00Z"}`,
			b:        `{"time": "2025-02-20T15:45:30Z"}`,
			expected: true,
		},
		{
			name:     "different values",
			a:        `{"a": 1}`,
			b:        `{"a": 2}`,
			expected: false,
		},
		{
			name:     "different keys",
			a:        `{"a": 1}`,
			b:        `{"b": 1}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CompareNormalizedJSON([]byte(tt.a), []byte(tt.b))
			if err != nil {
				t.Fatalf("CompareNormalizedJSON() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("CompareNormalizedJSON() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNormalizeFilePerms(t *testing.T) {
	tests := []struct {
		name     string
		mode     uint32
		expected string
	}{
		{
			name:     "standard file",
			mode:     0644,
			expected: "644",
		},
		{
			name:     "executable",
			mode:     0755,
			expected: "755",
		},
		{
			name:     "directory",
			mode:     0777,
			expected: "777",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeFilePerms(tt.mode)
			if result != tt.expected {
				t.Errorf("NormalizeFilePerms() = %v, want %v", result, tt.expected)
			}
		})
	}
}
