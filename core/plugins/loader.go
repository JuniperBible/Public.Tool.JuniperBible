// Package plugins provides plugin loading and management for Juniper Bible.
// Plugins are external executables that handle format detection/ingestion
// or run reference tools in the deterministic VM.
package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	apperrors "github.com/JuniperBible/Public.Tool.JuniperBible/core/errors"
	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/logging"
	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/safefile"
)

// ErrIncompatibleVersion is returned when a plugin's version requirements are not met.
var ErrIncompatibleVersion = errors.New("incompatible plugin version")

// HostVersion is the version of the plugin host system.
// This should match the application version that loads plugins.
const HostVersion = "0.5.0"

// PluginManifest represents the plugin.json manifest file.
type PluginManifest struct {
	PluginID   string `json:"plugin_id"`
	Version    string `json:"version"`
	Kind       string `json:"kind"` // Plugin kind: "format", "tool", "juniper", "example" (see PluginKinds)
	Entrypoint string `json:"entrypoint"`
	// MinHostVersion specifies the minimum host version required to run this plugin.
	// This allows plugins to declare compatibility requirements.
	// Format: semantic version (e.g., "0.5.0", "1.0.0")
	MinHostVersion string `json:"min_host_version,omitempty"`
	// License is a short license identifier (e.g., "MIT", "Apache-2.0", "GPL-3.0").
	License string `json:"license,omitempty"`
	// Capabilities describes what the plugin can do.
	// Used primarily by format plugins for input/output types.
	Capabilities Capabilities `json:"capabilities,omitempty"`
	// IRSupport describes the plugin's IR extraction/emission capabilities.
	// Only applicable to format plugins that support the IR pipeline.
	IRSupport *IRCapabilities `json:"ir_support,omitempty"`
}

// Capabilities describes what a plugin can do.
type Capabilities struct {
	Inputs   []string `json:"inputs,omitempty"`
	Outputs  []string `json:"outputs,omitempty"`
	Profiles []string `json:"profiles,omitempty"`
}

// IRCapabilities describes a plugin's IR support.
type IRCapabilities struct {
	CanExtract bool     `json:"can_extract"` // Can extract IR from native format
	CanEmit    bool     `json:"can_emit"`    // Can emit native format from IR
	LossClass  string   `json:"loss_class"`  // Expected loss class (L0-L4)
	Formats    []string `json:"formats"`     // Native formats supported
}

// Plugin represents a loaded plugin.
type Plugin struct {
	Manifest *PluginManifest
	Path     string // Directory containing the plugin
}

// Loader manages plugin discovery and loading.
type Loader struct {
	plugins map[string]*Plugin
}

// NewLoader creates a new plugin loader.
// It automatically includes all registered embedded plugins.
func NewLoader() *Loader {
	l := &Loader{
		plugins: make(map[string]*Plugin),
	}

	// Auto-register all embedded plugins
	for _, ep := range ListEmbeddedPlugins() {
		if ep.Manifest != nil {
			l.plugins[ep.Manifest.PluginID] = &Plugin{
				Manifest: ep.Manifest,
				Path:     "(embedded)",
			}
			logging.PluginLoading(ep.Manifest.PluginID, ep.Manifest.Version, ep.Manifest.Kind,
				"source", "embedded")
		}
	}

	return l
}

// LoadFromDir discovers and loads all plugins from a directory.
// Only loads external plugins if external plugins are enabled.
func (l *Loader) LoadFromDir(dir string) error {
	if !ExternalPluginsEnabled() {
		// External plugins disabled, skip loading from directory
		return nil
	}

	plugins, err := DiscoverPlugins(dir)
	if err != nil {
		return err
	}

	for _, p := range plugins {
		// Check version compatibility before loading
		if err := p.CheckCompatibility(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping incompatible plugin %s: %v\n", p.Manifest.PluginID, err)
			continue
		}
		// External plugins can override embedded ones
		l.plugins[p.Manifest.PluginID] = p
		logging.PluginLoading(p.Manifest.PluginID, p.Manifest.Version, p.Manifest.Kind,
			"source", "external",
			"path", p.Path)
	}

	return nil
}

// GetPlugin returns a plugin by its ID.
func (l *Loader) GetPlugin(id string) (*Plugin, error) {
	plugin, ok := l.plugins[id]
	if !ok {
		return nil, apperrors.NewNotFound("plugin", id)
	}
	return plugin, nil
}

// GetPluginsByKind returns all plugins of a specific kind.
func (l *Loader) GetPluginsByKind(kind string) []*Plugin {
	var result []*Plugin
	for _, p := range l.plugins {
		if p.Manifest.Kind == kind {
			result = append(result, p)
		}
	}
	return result
}

// ListPlugins returns all loaded plugins.
func (l *Loader) ListPlugins() []*Plugin {
	result := make([]*Plugin, 0, len(l.plugins))
	for _, p := range l.plugins {
		result = append(result, p)
	}
	return result
}

// PluginKinds defines the supported plugin kind directories.
//
// ADDING A NEW PLUGIN KIND:
// To add a new plugin kind (e.g., "mykind"), you must:
//
//  1. Add the kind name to this slice:
//     var PluginKinds = []string{"format", "tool", "juniper", "example", "mykind"}
//
//  2. Add a helper method on Plugin (see IsExample below):
//     func (p *Plugin) IsMyKind() bool {
//     return p.Manifest.Kind == "mykind"
//     }
//
//  3. Create the plugin directory:
//     mkdir -p plugins/mykind/
//
//  4. Add a test in loader_test.go (see TestExamplePluginKind)
//
//  5. Update the Kind field comment in PluginManifest struct above
//
//  6. (Optional) Update documentation:
//     - docs/PLUGIN_DEVELOPMENT.md
//     - README.md repository structure
//
// Plugin directories follow the nested structure:
//
//	plugins/
//	├── format/     # Format plugins (osis, usfm, sword, etc.)
//	├── tool/       # Tool plugins (libsword, etc.)
//	├── juniper/    # Juniper plugins
//	├── example/    # Example plugins (for documentation/templates)
//	└── mykind/     # Your new plugin kind
//
// Each plugin within a kind directory has its own subdirectory:
//
//	plugins/mykind/
//	└── my-plugin/
//	    ├── plugin.json    # Required: plugin manifest
//	    └── mykind-my-plugin  # Required: executable matching entrypoint
var PluginKinds = []string{"format", "tool", "juniper", "example"}

// DiscoverPlugins discovers plugins in a directory.
// It supports both flat structure (plugins/format-osis/) and
// nested structure (plugins/format/osis/).
func DiscoverPlugins(dir string) ([]*Plugin, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve plugin directory: %w", err)
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, apperrors.NewIO("read", absDir, err)
	}

	return discoverPluginsInEntries(absDir, entries), nil
}

func discoverPluginsInEntries(dir string, entries []os.DirEntry) []*Plugin {
	var plugins []*Plugin
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		entryPath := filepath.Join(dir, entry.Name())
		found := discoverPluginsFromEntry(entryPath, entry.Name())
		plugins = append(plugins, found...)
	}
	return plugins
}

func discoverPluginsFromEntry(entryPath, name string) []*Plugin {
	manifestPath := filepath.Join(entryPath, "plugin.json")
	if _, err := os.Stat(manifestPath); err == nil {
		if plugin, err := loadPluginFromDir(entryPath); err == nil {
			return []*Plugin{plugin}
		}
		logging.Warn("failed to load plugin", "path", entryPath)
		return nil
	}

	if isKindDirectory(name) {
		plugins, err := discoverPluginsInKindDir(entryPath)
		if err != nil {
			logging.Warn("failed to scan kind directory", "path", entryPath, "error", err)
			return nil
		}
		return plugins
	}
	return nil
}

// isKindDirectory checks if a directory name is a plugin kind directory.
func isKindDirectory(name string) bool {
	for _, kind := range PluginKinds {
		if name == kind {
			return true
		}
	}
	return false
}

// discoverPluginsInKindDir discovers plugins in a kind directory (e.g., plugins/format/).
func discoverPluginsInKindDir(kindDir string) ([]*Plugin, error) {
	var plugins []*Plugin

	entries, err := os.ReadDir(kindDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(kindDir, entry.Name())
		plugin, err := loadPluginFromDir(pluginDir)
		if err != nil {
			// Not a valid plugin directory, skip
			continue
		}
		plugins = append(plugins, plugin)
	}

	return plugins, nil
}

// loadPluginFromDir loads a plugin from a directory containing plugin.json.
func loadPluginFromDir(pluginDir string) (*Plugin, error) {
	manifestPath := filepath.Join(pluginDir, "plugin.json")

	if _, err := os.Stat(manifestPath); errors.Is(err, os.ErrNotExist) {
		return nil, apperrors.NewNotFound("plugin.json", pluginDir)
	}

	manifest, err := ParsePluginManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	return &Plugin{
		Manifest: manifest,
		Path:     pluginDir,
	}, nil
}

// ParsePluginManifest parses a plugin.json file.
func ParsePluginManifest(path string) (*PluginManifest, error) {
	data, err := safefile.ReadFile(path)
	if err != nil {
		return nil, apperrors.NewIO("read", path, err)
	}

	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, apperrors.NewParse("JSON", path, err.Error())
	}

	// Validate required fields
	if manifest.PluginID == "" {
		return nil, apperrors.NewValidation("plugin_id", "is required")
	}
	if manifest.Version == "" {
		return nil, apperrors.NewValidation("version", "is required")
	}
	if manifest.Kind == "" {
		return nil, apperrors.NewValidation("kind", "is required")
	}
	if manifest.Entrypoint == "" {
		return nil, apperrors.NewValidation("entrypoint", "is required")
	}

	return &manifest, nil
}

// EntrypointPath returns the full path to the plugin's entrypoint executable.
func (p *Plugin) EntrypointPath() string {
	return filepath.Join(p.Path, p.Manifest.Entrypoint)
}

// IsFormat returns true if this is a format plugin.
func (p *Plugin) IsFormat() bool {
	return p.Manifest.Kind == "format"
}

// IsTool returns true if this is a tool plugin.
func (p *Plugin) IsTool() bool {
	return p.Manifest.Kind == "tool"
}

// IsJuniper returns true if this is a juniper plugin.
func (p *Plugin) IsJuniper() bool {
	return p.Manifest.Kind == "juniper"
}

// IsExample returns true if this is an example plugin.
//
// ADDING A NEW KIND HELPER:
// When adding a new plugin kind, add a corresponding Is<Kind>() method:
//
//	// IsMyKind returns true if this is a mykind plugin.
//	func (p *Plugin) IsMyKind() bool {
//	    return p.Manifest.Kind == "mykind"
//	}
//
// This helper is used by code that needs to filter or handle specific
// plugin kinds differently. The method name should be Is<Kind> where
// <Kind> is the capitalized version of the kind string.
func (p *Plugin) IsExample() bool {
	return p.Manifest.Kind == "example"
}

// CheckCompatibility checks if this plugin is compatible with the host version.
// Returns nil if compatible, or ErrIncompatibleVersion with details if not.
func (p *Plugin) CheckCompatibility() error {
	return CheckPluginCompatibility(p.Manifest, HostVersion)
}

// CheckPluginCompatibility checks if a plugin manifest is compatible with a host version.
// This is separated from the Plugin method to allow testing with different host versions.
func CheckPluginCompatibility(manifest *PluginManifest, hostVersion string) error {
	// If no minimum host version is specified, assume compatible
	if manifest.MinHostVersion == "" {
		return nil
	}

	// Parse host version
	host, err := ParseVersion(hostVersion)
	if err != nil {
		return fmt.Errorf("invalid host version %q: %w", hostVersion, err)
	}

	// Parse minimum required version
	minRequired, err := ParseVersion(manifest.MinHostVersion)
	if err != nil {
		return fmt.Errorf("invalid min_host_version %q in plugin %s: %w",
			manifest.MinHostVersion, manifest.PluginID, err)
	}

	// Check if host version is compatible with minimum requirement
	if !host.IsCompatibleWith(minRequired) {
		return fmt.Errorf("%w: plugin %s requires host version %s, but current version is %s",
			ErrIncompatibleVersion, manifest.PluginID, minRequired.String(), host.String())
	}

	return nil
}

// SupportsIR returns true if this plugin has any IR capabilities.
func (p *Plugin) SupportsIR() bool {
	return p.Manifest.IRSupport != nil
}

// CanExtractIR returns true if this plugin can extract IR from native format.
func (p *Plugin) CanExtractIR() bool {
	return p.Manifest.IRSupport != nil && p.Manifest.IRSupport.CanExtract
}

// CanEmitIR returns true if this plugin can emit native format from IR.
func (p *Plugin) CanEmitIR() bool {
	return p.Manifest.IRSupport != nil && p.Manifest.IRSupport.CanEmit
}

// GetIRCapablePlugins returns all plugins that support IR operations.
func (l *Loader) GetIRCapablePlugins() []*Plugin {
	var result []*Plugin
	for _, p := range l.plugins {
		if p.SupportsIR() {
			result = append(result, p)
		}
	}
	return result
}

// AddPlugin adds a plugin to the loader directly.
// This is primarily used for testing and for registering discovered plugins
// without going through LoadFromDir's ExternalPluginsEnabled check.
func (l *Loader) AddPlugin(p *Plugin) {
	l.plugins[p.Manifest.PluginID] = p
}

// LoadFromDirAlways loads plugins from a directory regardless of ExternalPluginsEnabled setting.
// This is useful for testing the plugin discovery mechanism.
func (l *Loader) LoadFromDirAlways(dir string) error {
	plugins, err := DiscoverPlugins(dir)
	if err != nil {
		return err
	}

	for _, p := range plugins {
		// Check version compatibility before loading
		if err := p.CheckCompatibility(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping incompatible plugin %s: %v\n", p.Manifest.PluginID, err)
			continue
		}
		l.plugins[p.Manifest.PluginID] = p
	}

	return nil
}
