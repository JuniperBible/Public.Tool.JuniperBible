// Package web provides HTTP handlers with security hardening.
// This file contains web-specific security utilities that complement
// the validation package for defense in depth.
package web

import (
	"path/filepath"
	"strings"

	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/validation"
)

// ValidatePath performs web-specific path validation for capsule operations.
// It wraps validation.SanitizePath with additional web-specific checks.
//
// This function provides defense-in-depth by:
// 1. Rejecting paths with ".." components (path traversal)
// 2. Ensuring paths are within the base directory
// 3. Cleaning paths using filepath.Clean
// 4. Validating against null bytes and control characters
//
// Returns the cleaned path relative to baseDir, or an error if validation fails.
func ValidatePath(baseDir, userPath string) (string, error) {
	// Use the validation package for core path sanitization
	return validation.SanitizePath(baseDir, userPath)
}

// ValidateCapsulePath validates a capsule path from user input.
// This is a convenience wrapper specific to capsule operations.
func ValidateCapsulePath(capsulePath string) (string, error) {
	return validation.SanitizePath(ServerConfig.CapsulesDir, capsulePath)
}

// IsPathSafe checks if a path is safe for use in web handlers.
// Returns true if the path passes all validation checks.
func IsPathSafe(baseDir, userPath string) bool {
	_, err := validation.SanitizePath(baseDir, userPath)
	return err == nil
}

// SanitizePathForDisplay sanitizes a path for safe display in HTML.
// This prevents injection attacks when showing paths to users.
func SanitizePathForDisplay(path string) string {
	// Remove any null bytes
	path = strings.ReplaceAll(path, "\x00", "")

	// Clean the path
	path = filepath.Clean(path)

	// Ensure it's not trying to escape
	if strings.Contains(path, "..") {
		return "[INVALID_PATH]"
	}

	return path
}

// ValidateArtifactID validates an artifact ID from query parameters.
// Artifact IDs should be safe filenames without path separators.
func ValidateArtifactID(artifactID string) error {
	return validation.ValidateFilename(artifactID)
}

// ValidateCapsuleID validates a capsule ID used in URL routing.
// Capsule IDs should not contain path separators or special characters.
func ValidateCapsuleID(capsuleID string) error {
	if capsuleID == "" {
		return validation.ErrEmptyPath
	}

	// Capsule IDs should be simple filenames
	if strings.Contains(capsuleID, "/") || strings.Contains(capsuleID, "\\") {
		return validation.ErrInvalidFilename
	}

	// Check for path traversal attempts
	if strings.Contains(capsuleID, "..") {
		return validation.ErrPathTraversal
	}

	return nil
}
