// Package plugins provides plugin loading and management for Juniper Bible.
package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrInvalidPluginPath is returned when a plugin path fails security validation.
var ErrInvalidPluginPath = errors.New("invalid plugin path")

// SecurityConfig holds plugin security settings.
type SecurityConfig struct {
	// AllowedPluginDirs is a list of directories where plugins may be loaded from.
	// Empty list means allow plugins from any directory (not recommended for production).
	AllowedPluginDirs []string

	// RequireManifest enforces that all plugins must have a valid plugin.json manifest.
	// Default: true
	RequireManifest bool

	// RestrictToKnownKinds enforces that plugin kinds must be in PluginKinds list.
	// Default: true
	RestrictToKnownKinds bool
}

var (
	// globalSecurityConfig is the active security configuration.
	// By default, it's permissive to maintain backward compatibility.
	globalSecurityConfig = SecurityConfig{
		AllowedPluginDirs:    nil,
		RequireManifest:      true,
		RestrictToKnownKinds: true,
	}
)

// SetSecurityConfig updates the global plugin security configuration.
// This should be called during server initialization before loading any plugins.
func SetSecurityConfig(cfg SecurityConfig) {
	globalSecurityConfig = cfg
}

// GetSecurityConfig returns the current plugin security configuration.
func GetSecurityConfig() SecurityConfig {
	return globalSecurityConfig
}

func resolvePluginAbsPath(pluginPath string) (string, error) {
	if pluginPath == "" {
		return "", fmt.Errorf("%w: empty path", ErrInvalidPluginPath)
	}
	if strings.Contains(pluginPath, "..") {
		return "", fmt.Errorf("%w: path traversal detected", ErrInvalidPluginPath)
	}
	absPath, err := filepath.Abs(pluginPath)
	if err != nil {
		return "", fmt.Errorf("%w: failed to resolve absolute path: %v", ErrInvalidPluginPath, err)
	}
	return absPath, nil
}

func lstatPluginFile(absPath string) (os.FileInfo, error) {
	info, err := os.Lstat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: plugin file not found", ErrInvalidPluginPath)
		}
		return nil, fmt.Errorf("%w: failed to stat plugin file: %v", ErrInvalidPluginPath, err)
	}
	return info, nil
}

func validateSymlink(absPath string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return fmt.Errorf("%w: failed to resolve symlink: %v", ErrInvalidPluginPath, err)
	}
	if err := validatePluginDirectory(realPath); err != nil {
		return fmt.Errorf("%w: symlink target failed validation: %v", ErrInvalidPluginPath, err)
	}
	return nil
}

func ValidatePluginPath(pluginPath string) error {
	absPath, err := resolvePluginAbsPath(pluginPath)
	if err != nil {
		return err
	}
	info, err := lstatPluginFile(absPath)
	if err != nil {
		return err
	}
	if err := validateSymlink(absPath, info); err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: not a regular file", ErrInvalidPluginPath)
	}
	return validatePluginDirectory(absPath)
}

// validatePluginDirectory checks if a path is within allowed plugin directories.
func validatePluginDirectory(absPath string) error {
	cfg := globalSecurityConfig

	// If no restrictions configured, allow any path
	if len(cfg.AllowedPluginDirs) == 0 {
		return nil
	}

	// Check if path is within any allowed directory
	for _, allowedDir := range cfg.AllowedPluginDirs {
		// Convert allowed dir to absolute path
		absAllowedDir, err := filepath.Abs(allowedDir)
		if err != nil {
			continue
		}

		// Check if plugin path is within this allowed directory
		relPath, err := filepath.Rel(absAllowedDir, absPath)
		if err != nil {
			continue
		}

		// If relPath doesn't start with "..", it's within the allowed directory
		if !strings.HasPrefix(relPath, "..") {
			return nil
		}
	}

	return fmt.Errorf("%w: path not in allowed plugin directories", ErrInvalidPluginPath)
}

// ValidatePluginManifestSecurity validates plugin manifest for security concerns.
func ValidatePluginManifestSecurity(manifest *PluginManifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}
	if err := validateManifestRequired(manifest); err != nil {
		return err
	}
	if err := validateManifestKind(manifest); err != nil {
		return err
	}
	return validateManifestEntrypoint(manifest)
}

// validateManifestRequired checks if required fields are present.
func validateManifestRequired(manifest *PluginManifest) error {
	if globalSecurityConfig.RequireManifest && manifest.PluginID == "" {
		return fmt.Errorf("plugin manifest required but missing plugin_id")
	}
	return nil
}

// validateManifestKind checks if the plugin kind is known.
func validateManifestKind(manifest *PluginManifest) error {
	if !globalSecurityConfig.RestrictToKnownKinds {
		return nil
	}
	for _, kind := range PluginKinds {
		if manifest.Kind == kind {
			return nil
		}
	}
	return fmt.Errorf("unknown plugin kind: %s (allowed: %v)", manifest.Kind, PluginKinds)
}

// validateManifestEntrypoint checks for path traversal in entrypoint.
func validateManifestEntrypoint(manifest *PluginManifest) error {
	if strings.Contains(manifest.Entrypoint, "..") {
		return fmt.Errorf("entrypoint contains path traversal")
	}
	return nil
}

// SecureEntrypointPath returns the validated full path to a plugin's entrypoint.
// This should be used instead of Plugin.EntrypointPath() when security is a concern.
func (p *Plugin) SecureEntrypointPath() (string, error) {
	if p.Manifest == nil {
		return "", fmt.Errorf("plugin has no manifest")
	}

	// Validate manifest security
	if err := ValidatePluginManifestSecurity(p.Manifest); err != nil {
		return "", fmt.Errorf("manifest validation failed: %w", err)
	}

	// Construct entrypoint path
	entrypoint := filepath.Join(p.Path, p.Manifest.Entrypoint)

	// Validate the entrypoint path
	if err := ValidatePluginPath(entrypoint); err != nil {
		return "", fmt.Errorf("entrypoint validation failed: %w", err)
	}

	return entrypoint, nil
}
