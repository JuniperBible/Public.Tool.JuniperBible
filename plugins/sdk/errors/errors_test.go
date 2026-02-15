package errors

import (
	"fmt"
	"testing"
)

func TestPluginError(t *testing.T) {
	err := New(CodeMissingArg, "path argument required")

	if err.Code != CodeMissingArg {
		t.Errorf("Code = %v, want %v", err.Code, CodeMissingArg)
	}
	if err.Message != "path argument required" {
		t.Errorf("Message = %v, want %v", err.Message, "path argument required")
	}

	expected := "MISSING_ARG: path argument required"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestPluginErrorWithCause(t *testing.T) {
	cause := fmt.Errorf("underlying error")
	err := Wrap(CodeFileReadErr, "failed to read file", cause)

	if err.Cause != cause {
		t.Errorf("Cause = %v, want %v", err.Cause, cause)
	}
	if err.Unwrap() != cause {
		t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), cause)
	}

	// Error message should include cause
	if err.Error() != "FILE_READ_ERR: failed to read file: underlying error" {
		t.Errorf("Error() = %v", err.Error())
	}
}

func TestConvenienceConstructors(t *testing.T) {
	tests := []struct {
		name     string
		err      *PluginError
		wantCode ErrorCode
		wantMsg  string
	}{
		{
			name:     "MissingArg",
			err:      MissingArg("path"),
			wantCode: CodeMissingArg,
			wantMsg:  "path argument required",
		},
		{
			name:     "InvalidArg",
			err:      InvalidArg("path", "must be absolute"),
			wantCode: CodeInvalidArg,
			wantMsg:  "invalid path: must be absolute",
		},
		{
			name:     "UnknownCommand",
			err:      UnknownCommand("foo"),
			wantCode: CodeUnknownCmd,
			wantMsg:  "unknown command: foo",
		},
		{
			name:     "FileNotFound",
			err:      FileNotFound("/path/to/file"),
			wantCode: CodeFileNotFound,
			wantMsg:  "file not found: /path/to/file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Errorf("Code = %v, want %v", tt.err.Code, tt.wantCode)
			}
			if tt.err.Message != tt.wantMsg {
				t.Errorf("Message = %v, want %v", tt.err.Message, tt.wantMsg)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "storage error is retryable",
			err:  StorageError(fmt.Errorf("disk full")),
			want: true,
		},
		{
			name: "file write error is retryable",
			err:  FileWriteError("/path", fmt.Errorf("disk full")),
			want: true,
		},
		{
			name: "internal error is retryable",
			err:  Internal("unexpected", nil),
			want: true,
		},
		{
			name: "missing arg is not retryable",
			err:  MissingArg("path"),
			want: false,
		},
		{
			name: "parse error is not retryable",
			err:  ParseError("JSON", nil),
			want: false,
		},
		{
			name: "regular error is not retryable",
			err:  fmt.Errorf("regular error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "plugin error returns message",
			err:  MissingArg("path"),
			want: "path argument required",
		},
		{
			name: "regular error returns full error",
			err:  fmt.Errorf("some error"),
			want: "some error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToMessage(tt.err); got != tt.want {
				t.Errorf("ToMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
