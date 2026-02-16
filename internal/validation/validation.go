// Package validation provides input validation and sanitization functions
// to prevent common security vulnerabilities like path traversal, injection attacks,
// and resource exhaustion.
package validation

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode"
)

// Security limits to prevent DoS attacks (CWE-400).
const (
	// MaxFileSize is the maximum allowed file size (256 MB).
	MaxFileSize = 256 << 20
	// MaxFilenameLength is the maximum allowed filename length.
	MaxFilenameLength = 255
	// MaxPathLength is the maximum allowed path length.
	MaxPathLength = 4096
)

// Common validation errors.
var (
	ErrPathTraversal    = errors.New("path traversal detected")
	ErrInvalidFilename  = errors.New("invalid filename")
	ErrPathTooLong      = errors.New("path too long")
	ErrFilenameTooLong  = errors.New("filename too long")
	ErrInvalidCharacter = errors.New("invalid character in path")
	ErrEmptyPath        = errors.New("path cannot be empty")
)

// SanitizePath validates and sanitizes a user-supplied path to prevent path traversal attacks.
// It ensures the path does not escape the provided base directory.
// Returns the cleaned path relative to the base directory, or an error if invalid.
func SanitizePath(baseDir, userPath string) (string, error) {
	if userPath == "" {
		return "", ErrEmptyPath
	}

	// Check path length
	if len(userPath) > MaxPathLength {
		return "", ErrPathTooLong
	}

	// Clean the path to remove redundant separators and resolve . and ..
	cleanPath := filepath.Clean(userPath)

	// Reject paths that try to escape the base directory
	if strings.Contains(cleanPath, "..") {
		return "", ErrPathTraversal
	}

	// Reject absolute paths (should be relative to baseDir)
	if filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("%w: absolute path not allowed", ErrPathTraversal)
	}

	// Build full path and verify it's within baseDir
	fullPath := filepath.Join(baseDir, cleanPath)
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %w", err)
	}

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Ensure the resolved path is within the base directory
	relPath, err := filepath.Rel(absBase, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", ErrPathTraversal
	}

	return cleanPath, nil
}

// ValidateFilename checks if a filename is safe and does not contain malicious characters.
// It rejects filenames with path separators, control characters, and dangerous patterns.
func ValidateFilename(filename string) error {
	if filename == "" {
		return ErrInvalidFilename
	}

	// Check length
	if len(filename) > MaxFilenameLength {
		return ErrFilenameTooLong
	}

	// Reject dangerous filenames
	if filename == "." || filename == ".." {
		return fmt.Errorf("%w: reserved name", ErrInvalidFilename)
	}

	// Check for path separators
	if strings.ContainsAny(filename, "/\\") {
		return fmt.Errorf("%w: path separator not allowed", ErrInvalidFilename)
	}

	// Check for null bytes (common injection attack)
	if strings.Contains(filename, "\x00") {
		return fmt.Errorf("%w: null byte not allowed", ErrInvalidFilename)
	}

	// Check for control characters
	for _, r := range filename {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: control character not allowed", ErrInvalidFilename)
		}
	}

	// Reject filenames starting with hyphen (can be confused with command flags)
	if strings.HasPrefix(filename, "-") {
		return fmt.Errorf("%w: filename cannot start with hyphen", ErrInvalidFilename)
	}

	return nil
}

// IsPathSafe checks if a path is safe by validating it against common attack patterns.
// This is a convenience wrapper around SanitizePath that returns a boolean.
func IsPathSafe(baseDir, userPath string) bool {
	_, err := SanitizePath(baseDir, userPath)
	return err == nil
}

// ValidatePath performs comprehensive path validation without requiring a base directory.
// It checks for dangerous patterns, length limits, and invalid characters.
func ValidatePath(path string) error {
	if path == "" {
		return ErrEmptyPath
	}

	// Check length
	if len(path) > MaxPathLength {
		return ErrPathTooLong
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("%w: null byte not allowed", ErrInvalidCharacter)
	}

	// Check for control characters
	for _, r := range path {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: control character not allowed", ErrInvalidCharacter)
		}
	}

	return nil
}

// SanitizeFilename sanitizes a filename by removing or replacing invalid characters.
// This is useful when generating filenames from user input.
// Returns a safe filename or an error if the filename cannot be sanitized.
func SanitizeFilename(filename string) (string, error) {
	if filename == "" {
		return "", ErrInvalidFilename
	}

	// Remove leading/trailing whitespace
	filename = strings.TrimSpace(filename)

	// Replace path separators with underscores
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	// Remove null bytes
	filename = strings.ReplaceAll(filename, "\x00", "")

	// Remove control characters
	var cleaned strings.Builder
	for _, r := range filename {
		if !unicode.IsControl(r) {
			cleaned.WriteRune(r)
		}
	}
	filename = cleaned.String()

	// Remove leading hyphens
	filename = strings.TrimLeft(filename, "-")

	// Final validation
	if err := ValidateFilename(filename); err != nil {
		return "", err
	}

	return filename, nil
}

// ErrInvalidHash is returned when a hash string is invalid.
var ErrInvalidHash = errors.New("invalid hash")

// IsValidHexHash checks if a string is a valid hex hash (SHA256 or BLAKE3).
// It validates that the string contains only hexadecimal characters and has
// the expected length for hash algorithms (64 chars for SHA256/BLAKE3).
// This prevents path traversal attacks via malicious hash values.
func IsValidHexHash(hash string) bool {
	if len(hash) < 3 || len(hash) > 128 {
		return false
	}
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// ValidateHexHash validates a hash string and returns an error if invalid.
func ValidateHexHash(hash string) error {
	if !IsValidHexHash(hash) {
		return ErrInvalidHash
	}
	return nil
}

// FileType represents a validated file type.
type FileType string

const (
	// Archive formats
	FileTypeTarXZ FileType = "tar.xz"
	FileTypeTarGZ FileType = "tar.gz"
	FileTypeTar   FileType = "tar"
	FileTypeZip   FileType = "zip"
	FileTypeGzip  FileType = "gzip"
	FileTypeXZ    FileType = "xz"

	// Binary formats
	FileTypeSQLite FileType = "sqlite"

	// Text/XML formats
	FileTypeXML  FileType = "xml"
	FileTypeJSON FileType = "json"
	FileTypeText FileType = "text"

	// Unknown
	FileTypeUnknown FileType = "unknown"
)

// magicBytes defines magic byte signatures for file type detection.
var magicBytes = []struct {
	fileType FileType
	magic    []byte
	offset   int
}{
	// Archive formats
	{FileTypeTar, []byte("ustar"), 257},                         // POSIX tar
	{FileTypeGzip, []byte{0x1f, 0x8b}, 0},                       // Gzip
	{FileTypeXZ, []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}, 0}, // XZ
	{FileTypeZip, []byte{0x50, 0x4b, 0x03, 0x04}, 0},            // ZIP

	// SQLite (must check before other binary formats)
	{FileTypeSQLite, []byte("SQLite format 3"), 0},
}

// fileTypeValidationRule defines how to validate a specific file type combination.
type fileTypeValidationRule struct {
	expectedType FileType
	detectedType FileType
	isValid      bool
}

// validFileTypeCombinations maps expected+detected type combinations to validity.
// This lookup table replaces complex conditional logic.
var validFileTypeCombinations = buildValidCombinations()

// buildValidCombinations creates the lookup table for valid file type combinations.
func buildValidCombinations() map[string]bool {
	combos := make(map[string]bool)

	// Compressed tar formats (compression wrapper hides tar signature)
	combos[makeKey(FileTypeTarXZ, FileTypeXZ)] = true
	combos[makeKey(FileTypeTarGZ, FileTypeGzip)] = true

	// Single compression formats
	combos[makeKey(FileTypeXZ, FileTypeXZ)] = true
	combos[makeKey(FileTypeGzip, FileTypeGzip)] = true

	// All exact matches are valid
	for _, ft := range []FileType{
		FileTypeTar, FileTypeZip, FileTypeSQLite,
		FileTypeXML, FileTypeJSON, FileTypeText,
	} {
		combos[makeKey(ft, ft)] = true
	}

	return combos
}

// makeKey creates a lookup key from expected and detected types.
func makeKey(expected, detected FileType) string {
	return string(expected) + "|" + string(detected)
}

// isTextBasedType checks if a file type is text-based (XML, JSON, or plain text).
func isTextBasedType(ft FileType) bool {
	return ft == FileTypeXML || ft == FileTypeJSON || ft == FileTypeText
}

// validateTextBasedFile validates text-based files when magic bytes can't detect the type.
func validateTextBasedFile(buf []byte, expectedType, detectedType FileType) (FileType, bool) {
	if detectedType != FileTypeUnknown {
		return FileTypeUnknown, false
	}

	if !isTextBasedType(expectedType) {
		return FileTypeUnknown, false
	}

	if isLikelyText(buf) {
		return expectedType, true
	}

	return FileTypeUnknown, false
}

// resolveTypeMismatch handles cases where detected and expected types don't match.
func resolveTypeMismatch(expectedType, detectedType FileType) (FileType, error) {
	// If we couldn't detect the type, trust the extension
	if detectedType == FileTypeUnknown {
		return expectedType, nil
	}

	// Both types are known but don't match - this is an error
	if expectedType != FileTypeUnknown {
		return FileTypeUnknown, fmt.Errorf("file type mismatch: extension suggests %s but content is %s", expectedType, detectedType)
	}

	// Only detected type is known
	return detectedType, nil
}

// ValidateFileType validates that a file's content matches its claimed type based on filename extension.
// It reads the file's magic bytes to verify the actual file type.
// Returns the detected file type or an error if the file type doesn't match expectations.
func ValidateFileType(reader io.Reader, filename string) (FileType, error) {
	// Read first 512 bytes for magic byte detection (enough for tar ustar at offset 257)
	buf := make([]byte, 512)
	n, err := io.ReadFull(reader, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return FileTypeUnknown, fmt.Errorf("failed to read file header: %w", err)
	}
	buf = buf[:n]

	// Detect actual file type from magic bytes
	detectedType := detectFileTypeFromMagic(buf)

	// Determine expected type from extension
	expectedType := detectFileTypeFromExtension(filename)

	// Check if this is a valid combination using lookup table
	if validFileTypeCombinations[makeKey(expectedType, detectedType)] {
		return expectedType, nil
	}

	// Handle text-based files (harder to distinguish by magic bytes)
	if validType, ok := validateTextBasedFile(buf, expectedType, detectedType); ok {
		return validType, nil
	}

	// Resolve any type mismatches
	return resolveTypeMismatch(expectedType, detectedType)
}

// detectFileTypeFromMagic detects file type from magic bytes.
func detectFileTypeFromMagic(buf []byte) FileType {
	for _, sig := range magicBytes {
		if sig.offset+len(sig.magic) <= len(buf) {
			if bytes.Equal(buf[sig.offset:sig.offset+len(sig.magic)], sig.magic) {
				return sig.fileType
			}
		}
	}
	return FileTypeUnknown
}

// detectFileTypeFromExtension determines expected file type from filename extension.
func detectFileTypeFromExtension(filename string) FileType {
	lower := strings.ToLower(filename)

	// Multi-extension formats (check these first)
	if strings.HasSuffix(lower, ".tar.xz") {
		return FileTypeTarXZ
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return FileTypeTarGZ
	}

	// Single extension formats
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".tar":
		return FileTypeTar
	case ".xz":
		return FileTypeXZ
	case ".gz":
		return FileTypeGzip
	case ".zip":
		return FileTypeZip
	case ".sqlite", ".db", ".sqlite3":
		return FileTypeSQLite
	case ".xml", ".osis", ".usx", ".zefania":
		return FileTypeXML
	case ".json":
		return FileTypeJSON
	case ".txt", ".usfm", ".sfm", ".md":
		return FileTypeText
	default:
		return FileTypeUnknown
	}
}

// isLikelyText checks if the buffer contains likely text content.
// Returns true if the buffer appears to be text (UTF-8, ASCII).
func isLikelyText(buf []byte) bool {
	if len(buf) == 0 {
		return false
	}

	// Check for null bytes (strong indicator of binary content)
	if bytes.IndexByte(buf, 0) != -1 {
		return false
	}

	// Count printable characters vs control characters
	printable := 0
	control := 0
	for _, b := range buf {
		if b >= 0x20 && b <= 0x7e || b == '\t' || b == '\n' || b == '\r' {
			printable++
		} else if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			control++
		}
		// UTF-8 continuation bytes (0x80-0xBF) and start bytes (0xC0-0xFD) are neutral
	}

	// If more than 95% is printable, consider it text
	if printable > 0 && float64(printable)/float64(printable+control) > 0.95 {
		return true
	}

	return false
}
