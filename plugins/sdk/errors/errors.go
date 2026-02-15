// Package errors provides standardized error types for SDK plugins.
package errors

import (
	"fmt"
)

// ErrorCode represents a standardized error code.
type ErrorCode string

// Standard error codes
const (
	// Request errors
	CodeMissingArg    ErrorCode = "MISSING_ARG"
	CodeInvalidArg    ErrorCode = "INVALID_ARG"
	CodeUnknownCmd    ErrorCode = "UNKNOWN_CMD"
	CodeInvalidJSON   ErrorCode = "INVALID_JSON"

	// File errors
	CodeFileNotFound  ErrorCode = "FILE_NOT_FOUND"
	CodeFileReadErr   ErrorCode = "FILE_READ_ERR"
	CodeFileWriteErr  ErrorCode = "FILE_WRITE_ERR"
	CodePermissionErr ErrorCode = "PERMISSION_ERR"

	// Format errors
	CodeNotDetected   ErrorCode = "NOT_DETECTED"
	CodeParseErr      ErrorCode = "PARSE_ERR"
	CodeInvalidFormat ErrorCode = "INVALID_FORMAT"

	// Storage errors
	CodeStorageErr    ErrorCode = "STORAGE_ERR"
	CodeHashMismatch  ErrorCode = "HASH_MISMATCH"

	// IR errors
	CodeIRReadErr     ErrorCode = "IR_READ_ERR"
	CodeIRWriteErr    ErrorCode = "IR_WRITE_ERR"
	CodeIRInvalid     ErrorCode = "IR_INVALID"

	// Internal errors
	CodeInternal      ErrorCode = "INTERNAL"
)

// PluginError represents a structured plugin error.
type PluginError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Cause   error     `json:"-"`
}

// Error implements the error interface.
func (e *PluginError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause.
func (e *PluginError) Unwrap() error {
	return e.Cause
}

// New creates a new PluginError.
func New(code ErrorCode, message string) *PluginError {
	return &PluginError{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an existing error with a PluginError.
func Wrap(code ErrorCode, message string, cause error) *PluginError {
	return &PluginError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// Convenience constructors

// MissingArg creates an error for a missing required argument.
func MissingArg(name string) *PluginError {
	return New(CodeMissingArg, fmt.Sprintf("%s argument required", name))
}

// InvalidArg creates an error for an invalid argument value.
func InvalidArg(name, reason string) *PluginError {
	return New(CodeInvalidArg, fmt.Sprintf("invalid %s: %s", name, reason))
}

// UnknownCommand creates an error for an unknown command.
func UnknownCommand(cmd string) *PluginError {
	return New(CodeUnknownCmd, fmt.Sprintf("unknown command: %s", cmd))
}

// FileNotFound creates an error for a missing file.
func FileNotFound(path string) *PluginError {
	return New(CodeFileNotFound, fmt.Sprintf("file not found: %s", path))
}

// FileReadError creates an error for file read failures.
func FileReadError(path string, cause error) *PluginError {
	return Wrap(CodeFileReadErr, fmt.Sprintf("failed to read file: %s", path), cause)
}

// FileWriteError creates an error for file write failures.
func FileWriteError(path string, cause error) *PluginError {
	return Wrap(CodeFileWriteErr, fmt.Sprintf("failed to write file: %s", path), cause)
}

// ParseError creates an error for parsing failures.
func ParseError(format string, cause error) *PluginError {
	return Wrap(CodeParseErr, fmt.Sprintf("failed to parse %s", format), cause)
}

// StorageError creates an error for blob storage failures.
func StorageError(cause error) *PluginError {
	return Wrap(CodeStorageErr, "failed to store blob", cause)
}

// IRReadError creates an error for IR read failures.
func IRReadError(path string, cause error) *PluginError {
	return Wrap(CodeIRReadErr, fmt.Sprintf("failed to read IR: %s", path), cause)
}

// IRWriteError creates an error for IR write failures.
func IRWriteError(path string, cause error) *PluginError {
	return Wrap(CodeIRWriteErr, fmt.Sprintf("failed to write IR: %s", path), cause)
}

// Internal creates an internal error.
func Internal(message string, cause error) *PluginError {
	return Wrap(CodeInternal, message, cause)
}

// IsRetryable returns true if the error is potentially retryable.
func IsRetryable(err error) bool {
	if pe, ok := err.(*PluginError); ok {
		switch pe.Code {
		case CodeStorageErr, CodeFileWriteErr, CodeInternal:
			return true
		}
	}
	return false
}

// ToMessage converts a PluginError to a human-readable message for IPC.
// This strips the error code for backward compatibility with existing error format.
func ToMessage(err error) string {
	if pe, ok := err.(*PluginError); ok {
		return pe.Message
	}
	return err.Error()
}
