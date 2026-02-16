// Package crosswire provides the embedded handler for CrossWire native format plugin.
// This handles SWORD module distributions in their native .zip format
// as distributed by CrossWire repositories (containing mods.d/ and modules/).
package crosswire

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// Handler implements the EmbeddedFormatHandler interface for CrossWire format.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "format.crosswire",
		Version:    "1.0.0",
		Kind:       "format",
		Entrypoint: "format-crosswire",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{"file", "directory"},
			Outputs: []string{"artifact.kind:crosswire"},
		},
	}
}

// Register registers this plugin with the embedded registry.
func Register() {
	plugins.RegisterEmbeddedPlugin(&plugins.EmbeddedPlugin{
		Manifest: Manifest(),
		Format:   &Handler{},
	})
}

func init() {
	Register()
}

// Detect implements EmbeddedFormatHandler.Detect.
func (h *Handler) Detect(path string) (*plugins.DetectResult, error) {
	// Check if it's a zip file
	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		return detectZipArchive(path)
	}

	// Check if it's a directory with mods.d/
	return detectDirectory(path)
}

func detectZipArchive(path string) (*plugins.DetectResult, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("failed to open zip: %v", err),
		}, nil
	}
	defer zr.Close()

	hasModsD := false
	hasModules := false
	hasConf := false

	for _, f := range zr.File {
		name := f.Name

		// Check both direct paths and paths with a single parent directory
		if strings.Contains(name, "mods.d/") {
			hasModsD = true
			if strings.HasSuffix(name, ".conf") {
				hasConf = true
			}
		}
		if strings.Contains(name, "modules/") {
			hasModules = true
		}
	}

	if hasModsD && hasConf && hasModules {
		return &plugins.DetectResult{
			Detected: true,
			Format:   "crosswire-zip",
		}, nil
	}

	return &plugins.DetectResult{
		Detected: false,
		Reason:   "zip does not contain CrossWire SWORD module structure (missing mods.d/*.conf or modules/)",
	}, nil
}

func detectDirectory(path string) (*plugins.DetectResult, error) {
	modsDir := filepath.Join(path, "mods.d")
	modulesDir := filepath.Join(path, "modules")

	modsStat, modsErr := os.Stat(modsDir)
	modulesStat, modulesErr := os.Stat(modulesDir)

	if modsErr != nil || modulesErr != nil {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   "directory does not contain mods.d/ and modules/",
		}, nil
	}

	if !modsStat.IsDir() || !modulesStat.IsDir() {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   "mods.d or modules is not a directory",
		}, nil
	}

	// Check for at least one .conf file
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("failed to read mods.d: %v", err),
		}, nil
	}

	hasConf := false
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".conf") {
			hasConf = true
			break
		}
	}

	if !hasConf {
		return &plugins.DetectResult{
			Detected: false,
			Reason:   "no .conf files in mods.d/",
		}, nil
	}

	return &plugins.DetectResult{
		Detected: true,
		Format:   "crosswire-directory",
	}, nil
}

// Ingest implements EmbeddedFormatHandler.Ingest.
func (h *Handler) Ingest(path, outputDir string) (*plugins.IngestResult, error) {
	// Extract to temp directory if it's a zip
	workDir := path
	var cleanup func()

	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		tmpDir, err := os.MkdirTemp("", "crosswire-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
		cleanup = func() { os.RemoveAll(tmpDir) }
		defer cleanup()

		if err := extractZip(path, tmpDir); err != nil {
			return nil, fmt.Errorf("failed to extract zip: %w", err)
		}

		workDir = tmpDir
	}

	// Copy the entire SWORD module structure to output
	if err := copyDir(workDir, outputDir); err != nil {
		return nil, fmt.Errorf("failed to copy module: %w", err)
	}

	return &plugins.IngestResult{
		ArtifactID: filepath.Base(path),
		BlobSHA256: "",
		SizeBytes:  0,
		Metadata: map[string]string{
			"format": "crosswire",
			"status": "ingested",
		},
	}, nil
}

// Enumerate implements EmbeddedFormatHandler.Enumerate.
func (h *Handler) Enumerate(path string) (*plugins.EnumerateResult, error) {
	// Extract to temp directory if it's a zip
	workDir := path
	var cleanup func()

	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		tmpDir, err := os.MkdirTemp("", "crosswire-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
		cleanup = func() { os.RemoveAll(tmpDir) }
		defer cleanup()

		if err := extractZip(path, tmpDir); err != nil {
			return nil, fmt.Errorf("failed to extract zip: %w", err)
		}

		workDir = tmpDir
	}

	// Find all .conf files in mods.d
	modsDir := filepath.Join(workDir, "mods.d")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read mods.d: %w", err)
	}

	var modules []plugins.EnumerateEntry
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".conf") {
			info, _ := entry.Info()
			modules = append(modules, plugins.EnumerateEntry{
				Path:      entry.Name(),
				SizeBytes: info.Size(),
				IsDir:     false,
			})
		}
	}

	return &plugins.EnumerateResult{
		Entries: modules,
	}, nil
}

// ExtractIR implements EmbeddedFormatHandler.ExtractIR.
func (h *Handler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
	// Create a minimal IR corpus
	// NOTE: For full IR extraction, this should delegate to format-sword-pure plugin
	corpus := &ir.Corpus{
		ID:         "crosswire-module",
		Version:    "1.0.0",
		ModuleType: ir.ModuleBible,
		Language:   "en",
		Title:      "CrossWire Module",
		Documents:  []*ir.Document{},
	}

	// Serialize IR to JSON
	irPath := filepath.Join(outputDir, "corpus.json")
	data, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal IR: %w", err)
	}

	if err := os.WriteFile(irPath, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write IR: %w", err)
	}

	return &plugins.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L2",
		LossReport: &plugins.LossReportIPC{
			Warnings: []string{"Basic IR extraction - for full parsing use format-sword-pure plugin"},
		},
	}, nil
}

// EmitNative implements EmbeddedFormatHandler.EmitNative.
func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
	// Read IR
	data, err := os.ReadFile(irPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read IR: %w", err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		return nil, fmt.Errorf("failed to parse IR: %w", err)
	}

	// Create basic SWORD structure
	modsDir := filepath.Join(outputDir, "mods.d")
	modulesDir := filepath.Join(outputDir, "modules")

	if err := os.MkdirAll(modsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create mods.d: %w", err)
	}

	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create modules: %w", err)
	}

	// Create a minimal conf file
	confPath := filepath.Join(modsDir, corpus.ID+".conf")
	confContent := fmt.Sprintf("[%s]\nDescription=Exported from IR\nModDrv=RawText\n", corpus.ID)
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		return nil, fmt.Errorf("failed to write conf: %w", err)
	}

	// Create a zip archive
	zipPath := filepath.Join(filepath.Dir(outputDir), corpus.ID+".zip")
	if err := createZipArchive(outputDir, zipPath); err != nil {
		return nil, fmt.Errorf("failed to create zip: %w", err)
	}

	return &plugins.EmitNativeResult{
		OutputPath: zipPath,
		Format:     "crosswire-zip",
		LossClass:  "L2",
		LossReport: &plugins.LossReportIPC{
			Warnings: []string{"Basic SWORD structure - for full emission use format-sword-pure plugin"},
		},
	}, nil
}

// extractZip extracts a zip archive to the destination directory.
func extractZip(zipPath, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if err := extractZipFile(f, destDir); err != nil {
			return err
		}
	}

	return nil
}

func extractZipFile(f *zip.File, destDir string) error {
	// Clean the path to prevent zip slip
	cleanPath := filepath.Clean(f.Name)
	if strings.HasPrefix(cleanPath, "..") {
		return fmt.Errorf("invalid file path: %s", f.Name)
	}

	destPath := filepath.Join(destDir, cleanPath)

	if f.FileInfo().IsDir() {
		return os.MkdirAll(destPath, 0755)
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// Extract file
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}

// createZipArchive creates a zip archive from a directory.
func createZipArchive(sourceDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == zipPath {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		w, err := zw.Create(relPath)
		if err != nil {
			return err
		}

		_, err = io.Copy(w, file)
		return err
	})
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath)
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
