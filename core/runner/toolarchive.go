// Package runner provides execution harnesses for running tool plugins.
package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/core/capsule"
	"github.com/FocuswithJustin/JuniperBible/core/cas"
)

// Injectable functions for testing.
var (
	toolOsMkdirTemp = os.MkdirTemp
	capsuleUnpack   = capsule.Unpack
	capsuleNew      = capsule.New
)

// ToolArchive represents an archived tool binary stored in a capsule.
type ToolArchive struct {
	// ToolID is the unique identifier for the tool (e.g., "sword-utils", "pandoc").
	ToolID string `json:"tool_id"`

	// Version is the tool version string.
	Version string `json:"version"`

	// Platform is the target platform (e.g., "x86_64-linux").
	Platform string `json:"platform"`

	// Executables maps executable names to their artifact IDs.
	Executables map[string]string `json:"executables"`

	// Libraries maps library names to their artifact IDs.
	Libraries map[string]string `json:"libraries,omitempty"`

	// NixDerivation is the Nix derivation hash if built with Nix.
	NixDerivation string `json:"nix_derivation,omitempty"`

	// SourceHash is the hash of the source code used to build.
	SourceHash string `json:"source_hash,omitempty"`

	// capsule is the backing capsule containing the binaries.
	capsule *capsule.Capsule
}

// ToolArchiveManifest is stored in each tool capsule archive.
type ToolArchiveManifest struct {
	ToolID      string            `json:"tool_id"`
	Version     string            `json:"version"`
	Platform    string            `json:"platform"`
	Executables map[string]string `json:"executables"`
	Libraries   map[string]string `json:"libraries,omitempty"`
	NixDrv      string            `json:"nix_drv,omitempty"`
	SourceHash  string            `json:"source_hash,omitempty"`
	CreatedAt   string            `json:"created_at"`
}

// LoadToolArchive loads a tool archive from a capsule file.
func LoadToolArchive(capsulePath string) (*ToolArchive, error) {
	// Create temp directory for unpacking
	tempDir, err := toolOsMkdirTemp("", "tool-load-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	// Note: tempDir is not cleaned up here; caller should use Close()

	cap, err := capsuleUnpack(capsulePath, tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to unpack tool archive: %w", err)
	}

	// Look for tool manifest
	manifestArtifact, ok := cap.Manifest.Artifacts["tool-manifest"]
	if !ok {
		return nil, fmt.Errorf("tool archive missing tool-manifest artifact")
	}

	manifestData, err := cap.GetStore().Retrieve(manifestArtifact.PrimaryBlobSHA256)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve tool manifest: %w", err)
	}

	var manifest ToolArchiveManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse tool manifest: %w", err)
	}

	return &ToolArchive{
		ToolID:        manifest.ToolID,
		Version:       manifest.Version,
		Platform:      manifest.Platform,
		Executables:   manifest.Executables,
		Libraries:     manifest.Libraries,
		NixDerivation: manifest.NixDrv,
		SourceHash:    manifest.SourceHash,
		capsule:       cap,
	}, nil
}

// ExtractTo extracts all tool binaries to the specified directory.
func (t *ToolArchive) ExtractTo(destDir string) error {
	// Create bin directory
	binDir := filepath.Join(destDir, "bin")
	if err := os.MkdirAll(binDir, 0700); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Create lib directory if needed
	if len(t.Libraries) > 0 {
		libDir := filepath.Join(destDir, "lib")
		if err := os.MkdirAll(libDir, 0700); err != nil {
			return fmt.Errorf("failed to create lib directory: %w", err)
		}
	}

	// Extract executables
	for name, artifactID := range t.Executables {
		if err := t.extractArtifact(artifactID, filepath.Join(binDir, name), 0700); err != nil {
			return fmt.Errorf("failed to extract executable %s: %w", name, err)
		}
	}

	// Extract libraries
	for name, artifactID := range t.Libraries {
		libDir := filepath.Join(destDir, "lib")
		if err := t.extractArtifact(artifactID, filepath.Join(libDir, name), 0600); err != nil {
			return fmt.Errorf("failed to extract library %s: %w", name, err)
		}
	}

	return nil
}

// extractArtifact extracts a single artifact to a file.
func (t *ToolArchive) extractArtifact(artifactID, destPath string, mode os.FileMode) error {
	artifact, ok := t.capsule.Manifest.Artifacts[artifactID]
	if !ok {
		return fmt.Errorf("artifact not found: %s", artifactID)
	}

	data, err := t.capsule.GetStore().Retrieve(artifact.PrimaryBlobSHA256)
	if err != nil {
		return fmt.Errorf("failed to retrieve artifact: %w", err)
	}

	if err := os.WriteFile(destPath, data, mode); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// GetExecutablePath returns the path to an executable after extraction.
func (t *ToolArchive) GetExecutablePath(destDir, name string) string {
	return filepath.Join(destDir, "bin", name)
}

// Close closes the underlying capsule.
func (t *ToolArchive) Close() error {
	// Capsule doesn't have a close method, so this is a no-op
	return nil
}

// ToolRegistry manages a collection of tool archives.
type ToolRegistry struct {
	// ArchiveDir is the base directory containing tool capsule archives.
	ArchiveDir string

	// tools maps tool IDs to their loaded archives.
	tools map[string]*ToolArchive
}

// NewToolRegistry creates a new tool registry.
func NewToolRegistry(archiveDir string) *ToolRegistry {
	return &ToolRegistry{
		ArchiveDir: archiveDir,
		tools:      make(map[string]*ToolArchive),
	}
}

// LoadTool loads a tool archive by ID.
func (r *ToolRegistry) LoadTool(toolID string) (*ToolArchive, error) {
	// Check cache
	if tool, ok := r.tools[toolID]; ok {
		return tool, nil
	}

	// Look for archive file
	// Try common archive names
	archiveNames := []string{
		toolID + ".capsule.tar.xz",
		toolID + ".tar.xz",
		toolID + ".capsule",
	}

	var archivePath string
	for _, name := range archiveNames {
		path := filepath.Join(r.ArchiveDir, toolID, "capsule", name)
		if _, err := os.Stat(path); err == nil {
			archivePath = path
			break
		}
		// Also try directly in archive dir
		path = filepath.Join(r.ArchiveDir, name)
		if _, err := os.Stat(path); err == nil {
			archivePath = path
			break
		}
	}

	if archivePath == "" {
		return nil, fmt.Errorf("tool archive not found: %s", toolID)
	}

	tool, err := LoadToolArchive(archivePath)
	if err != nil {
		return nil, err
	}

	r.tools[toolID] = tool
	return tool, nil
}

// ListTools lists all available tool IDs.
func (r *ToolRegistry) ListTools() ([]string, error) {
	var tools []string

	entries, err := os.ReadDir(r.ArchiveDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read archive directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if this directory has a capsule subdirectory
		capsuleDir := filepath.Join(r.ArchiveDir, entry.Name(), "capsule")
		if _, err := os.Stat(capsuleDir); err == nil {
			tools = append(tools, entry.Name())
		}
	}

	return tools, nil
}

// CreateToolArchive creates a new tool archive capsule from binaries.
func CreateToolArchive(toolID, version, platform string, binaries map[string]string, destPath string) error {
	// Create a temporary capsule
	tempDir, err := toolOsMkdirTemp("", "tool-archive-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize capsule
	cap, err := capsuleNew(tempDir)
	if err != nil {
		return fmt.Errorf("failed to create capsule: %w", err)
	}

	executables := make(map[string]string)

	// Add binaries as artifacts
	for name, path := range binaries {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read binary %s: %w", name, err)
		}

		hash := cas.Hash(data)
		artifactID := "exe-" + name
		executables[name] = artifactID

		// Store the binary
		if _, err := cap.GetStore().Store(data); err != nil {
			return fmt.Errorf("failed to store binary: %w", err)
		}

		// Add artifact to manifest
		cap.Manifest.Artifacts[artifactID] = &capsule.Artifact{
			ID:                artifactID,
			Kind:              "executable",
			PrimaryBlobSHA256: hash,
			OriginalName:      name,
			SizeBytes:         int64(len(data)),
		}
	}

	// Create tool manifest
	manifest := ToolArchiveManifest{
		ToolID:      toolID,
		Version:     version,
		Platform:    platform,
		Executables: executables,
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}

	manifestHash := cas.Hash(manifestData)
	if _, err := cap.GetStore().Store(manifestData); err != nil {
		return fmt.Errorf("failed to store manifest: %w", err)
	}

	cap.Manifest.Artifacts["tool-manifest"] = &capsule.Artifact{
		ID:                "tool-manifest",
		Kind:              "metadata",
		PrimaryBlobSHA256: manifestHash,
		OriginalName:      "tool-manifest.json",
		SizeBytes:         int64(len(manifestData)),
	}

	// Pack the capsule
	if err := cap.Pack(destPath); err != nil {
		return fmt.Errorf("failed to pack capsule: %w", err)
	}

	return nil
}
