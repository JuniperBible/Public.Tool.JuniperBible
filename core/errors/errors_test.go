package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      *NotFoundError
		wantMsg  string
		wantBase error
	}{
		{
			name:     "with ID",
			err:      &NotFoundError{Resource: "plugin", ID: "test-plugin"},
			wantMsg:  "plugin not found: test-plugin",
			wantBase: ErrNotFound,
		},
		{
			name:     "without ID",
			err:      &NotFoundError{Resource: "artifact"},
			wantMsg:  "artifact not found",
			wantBase: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
			if got := tt.err.Unwrap(); !errors.Is(got, tt.wantBase) {
				t.Errorf("Unwrap() = %v, want %v", got, tt.wantBase)
			}
		})
	}

	// Test with underlying error separately
	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := fmt.Errorf("disk error")
		err := &NotFoundError{Resource: "file", ID: "test.txt", Err: underlyingErr}
		if got := err.Error(); got != "file not found: test.txt" {
			t.Errorf("Error() = %q, want %q", got, "file not found: test.txt")
		}
		if got := err.Unwrap(); got != underlyingErr {
			t.Errorf("Unwrap() = %v, want %v", got, underlyingErr)
		}
	})
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name     string
		err      *ValidationError
		wantMsg  string
		wantBase error
	}{
		{
			name:     "with field",
			err:      &ValidationError{Field: "username", Message: "must not be empty"},
			wantMsg:  "validation failed for username: must not be empty",
			wantBase: ErrInvalidInput,
		},
		{
			name:     "without field",
			err:      &ValidationError{Message: "invalid format"},
			wantMsg:  "validation failed: invalid format",
			wantBase: ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
			if got := tt.err.Unwrap(); !errors.Is(got, tt.wantBase) {
				t.Errorf("Unwrap() = %v, want %v", got, tt.wantBase)
			}
		})
	}

	// Test with underlying error separately
	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := fmt.Errorf("regex parse error")
		err := &ValidationError{Field: "pattern", Message: "invalid regex", Err: underlyingErr}
		if got := err.Unwrap(); got != underlyingErr {
			t.Errorf("Unwrap() = %v, want %v", got, underlyingErr)
		}
	})
}

func TestPermissionError(t *testing.T) {
	tests := []struct {
		name     string
		err      *PermissionError
		wantMsg  string
		wantBase error
	}{
		{
			name:     "full context",
			err:      &PermissionError{Operation: "delete", Resource: "plugin", Reason: "external plugins disabled"},
			wantMsg:  "permission denied: cannot delete plugin: external plugins disabled",
			wantBase: ErrUnauthorized,
		},
		{
			name:     "reason only",
			err:      &PermissionError{Reason: "read-only mode"},
			wantMsg:  "permission denied: read-only mode",
			wantBase: ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
			if got := tt.err.Unwrap(); !errors.Is(got, tt.wantBase) {
				t.Errorf("Unwrap() = %v, want %v", got, tt.wantBase)
			}
		})
	}

	// Test with underlying error separately
	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := fmt.Errorf("access denied by OS")
		err := &PermissionError{Operation: "write", Resource: "file", Reason: "no access", Err: underlyingErr}
		if got := err.Unwrap(); got != underlyingErr {
			t.Errorf("Unwrap() = %v, want %v", got, underlyingErr)
		}
	})
}

func TestIOError(t *testing.T) {
	baseErr := fmt.Errorf("permission denied")
	tests := []struct {
		name    string
		err     *IOError
		wantMsg string
	}{
		{
			name:    "with path",
			err:     &IOError{Operation: "read", Path: "/test/file.txt", Err: baseErr},
			wantMsg: "failed to read /test/file.txt: permission denied",
		},
		{
			name:    "without path",
			err:     &IOError{Operation: "write", Err: baseErr},
			wantMsg: "failed to write: permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
			if got := tt.err.Unwrap(); !errors.Is(got, baseErr) {
				t.Errorf("Unwrap() = %v, want %v", got, baseErr)
			}
		})
	}
}

func TestParseError(t *testing.T) {
	tests := []struct {
		name     string
		err      *ParseError
		wantMsg  string
		wantBase error
	}{
		{
			name:     "with path",
			err:      &ParseError{Format: "JSON", Path: "manifest.json", Message: "unexpected EOF"},
			wantMsg:  "failed to parse JSON at manifest.json: unexpected EOF",
			wantBase: ErrInvalidInput,
		},
		{
			name:     "without path",
			err:      &ParseError{Format: "XML", Message: "malformed tag"},
			wantMsg:  "failed to parse XML: malformed tag",
			wantBase: ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
			if got := tt.err.Unwrap(); !errors.Is(got, tt.wantBase) {
				t.Errorf("Unwrap() = %v, want %v", got, tt.wantBase)
			}
		})
	}

	// Test with underlying error separately
	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := fmt.Errorf("json: unexpected token")
		err := &ParseError{Format: "JSON", Path: "config.json", Message: "invalid syntax", Err: underlyingErr}
		if got := err.Unwrap(); got != underlyingErr {
			t.Errorf("Unwrap() = %v, want %v", got, underlyingErr)
		}
	})
}

func TestUnsupportedError(t *testing.T) {
	tests := []struct {
		name     string
		err      *UnsupportedError
		wantMsg  string
		wantBase error
	}{
		{
			name:     "with reason",
			err:      &UnsupportedError{Feature: "compression format", Reason: "lz4 not available"},
			wantMsg:  "unsupported compression format: lz4 not available",
			wantBase: ErrUnsupported,
		},
		{
			name:     "without reason",
			err:      &UnsupportedError{Feature: "format"},
			wantMsg:  "unsupported format",
			wantBase: ErrUnsupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
			if got := tt.err.Unwrap(); !errors.Is(got, tt.wantBase) {
				t.Errorf("Unwrap() = %v, want %v", got, tt.wantBase)
			}
		})
	}

	// Test with underlying error separately
	t.Run("with underlying error", func(t *testing.T) {
		underlyingErr := fmt.Errorf("codec not compiled")
		err := &UnsupportedError{Feature: "video codec", Reason: "h265 missing", Err: underlyingErr}
		if got := err.Unwrap(); got != underlyingErr {
			t.Errorf("Unwrap() = %v, want %v", got, underlyingErr)
		}
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("NewNotFound", func(t *testing.T) {
		err := NewNotFound("capsule", "test-id")
		if err.Resource != "capsule" || err.ID != "test-id" {
			t.Errorf("NewNotFound() = %+v, want Resource=capsule, ID=test-id", err)
		}
	})

	t.Run("NewValidation", func(t *testing.T) {
		err := NewValidation("email", "invalid format")
		if err.Field != "email" || err.Message != "invalid format" {
			t.Errorf("NewValidation() = %+v, want Field=email, Message=invalid format", err)
		}
	})

	t.Run("NewPermission", func(t *testing.T) {
		err := NewPermission("write", "file", "read-only mode")
		if err.Operation != "write" || err.Resource != "file" || err.Reason != "read-only mode" {
			t.Errorf("NewPermission() = %+v, unexpected values", err)
		}
	})

	t.Run("NewIO", func(t *testing.T) {
		baseErr := fmt.Errorf("disk full")
		err := NewIO("write", "/tmp/test", baseErr)
		if err.Operation != "write" || err.Path != "/tmp/test" || err.Err != baseErr {
			t.Errorf("NewIO() = %+v, unexpected values", err)
		}
	})

	t.Run("NewParse", func(t *testing.T) {
		err := NewParse("YAML", "config.yaml", "invalid syntax")
		if err.Format != "YAML" || err.Path != "config.yaml" || err.Message != "invalid syntax" {
			t.Errorf("NewParse() = %+v, unexpected values", err)
		}
	})

	t.Run("NewUnsupported", func(t *testing.T) {
		err := NewUnsupported("codec", "not compiled in")
		if err.Feature != "codec" || err.Reason != "not compiled in" {
			t.Errorf("NewUnsupported() = %+v, unexpected values", err)
		}
	})
}

func TestWrap(t *testing.T) {
	t.Run("wraps error", func(t *testing.T) {
		baseErr := fmt.Errorf("base error")
		wrapped := Wrap(baseErr, "context message")
		if wrapped == nil {
			t.Fatal("Wrap() returned nil")
		}
		if !errors.Is(wrapped, baseErr) {
			t.Errorf("Wrap() error does not unwrap to base error")
		}
		wantMsg := "context message: base error"
		if wrapped.Error() != wantMsg {
			t.Errorf("Wrap() = %q, want %q", wrapped.Error(), wantMsg)
		}
	})

	t.Run("nil error returns nil", func(t *testing.T) {
		if got := Wrap(nil, "context"); got != nil {
			t.Errorf("Wrap(nil) = %v, want nil", got)
		}
	})
}

func TestWrapf(t *testing.T) {
	t.Run("wraps error with formatting", func(t *testing.T) {
		baseErr := fmt.Errorf("base error")
		wrapped := Wrapf(baseErr, "failed to process %s", "file.txt")
		if wrapped == nil {
			t.Fatal("Wrapf() returned nil")
		}
		if !errors.Is(wrapped, baseErr) {
			t.Errorf("Wrapf() error does not unwrap to base error")
		}
		wantMsg := "failed to process file.txt: base error"
		if wrapped.Error() != wantMsg {
			t.Errorf("Wrapf() = %q, want %q", wrapped.Error(), wantMsg)
		}
	})

	t.Run("nil error returns nil", func(t *testing.T) {
		if got := Wrapf(nil, "context %s", "test"); got != nil {
			t.Errorf("Wrapf(nil) = %v, want nil", got)
		}
	})
}

func TestIs(t *testing.T) {
	err := &NotFoundError{Resource: "test"}
	if !Is(err, ErrNotFound) {
		t.Error("Is() failed to match NotFoundError to ErrNotFound")
	}
}

func TestAs(t *testing.T) {
	err := &NotFoundError{Resource: "test", ID: "123"}
	var nfErr *NotFoundError
	if !As(err, &nfErr) {
		t.Error("As() failed to match NotFoundError")
	}
	if nfErr.ID != "123" {
		t.Errorf("As() nfErr.ID = %q, want %q", nfErr.ID, "123")
	}
}
