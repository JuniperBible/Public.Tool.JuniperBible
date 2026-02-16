//go:build sdk

// Package main implements a CrossWire native format plugin using the SDK pattern.
// This handles SWORD module distributions in their native .zip format
// as distributed by CrossWire repositories (containing mods.d/ and modules/).
package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

func main() {
	if err := format.Run(&format.Config{
		Name:       "crosswire",
		Extensions: []string{".zip"},
		Detect:     Detect,
		Parse:      Parse,
		Emit:       Emit,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Detect checks if the path is a CrossWire SWORD module distribution.
func Detect(path string) (*ipc.DetectResult, error) {
	// Check if it's a zip file
	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		return detectZipArchive(path)
	}

	// Check if it's a directory with mods.d/
	return detectDirectory(path)
}

func detectZipArchive(path string) (*ipc.DetectResult, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return &ipc.DetectResult{
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
		// (handles both /mods.d/test.conf and /somemodule/mods.d/test.conf)
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
		return &ipc.DetectResult{
			Detected: true,
			Format:   "crosswire-zip",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "zip does not contain CrossWire SWORD module structure (missing mods.d/*.conf or modules/)",
	}, nil
}

func detectDirectory(path string) (*ipc.DetectResult, error) {
	modsDir := filepath.Join(path, "mods.d")
	modulesDir := filepath.Join(path, "modules")

	modsStat, modsErr := os.Stat(modsDir)
	modulesStat, modulesErr := os.Stat(modulesDir)

	if modsErr != nil || modulesErr != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "directory does not contain mods.d/ and modules/",
		}, nil
	}

	if !modsStat.IsDir() || !modulesStat.IsDir() {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "mods.d or modules is not a directory",
		}, nil
	}

	// Check for at least one .conf file
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return &ipc.DetectResult{
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
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "no .conf files in mods.d/",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: true,
		Format:   "crosswire-directory",
	}, nil
}

// Parse extracts a CrossWire SWORD module distribution into IR format.
func Parse(path string) (*ir.Corpus, error) {
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

	// Find all .conf files in mods.d to enumerate modules
	modsDir := filepath.Join(workDir, "mods.d")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read mods.d: %w", err)
	}

	var modules []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".conf") {
			// Module name is the conf filename without .conf
			moduleName := strings.TrimSuffix(entry.Name(), ".conf")
			modules = append(modules, moduleName)
		}
	}

	// For now, create a minimal IR corpus
	// NOTE: For full IR extraction, this should parse SWORD module data
	// or delegate to format-sword-pure plugin
	var moduleID string
	if len(modules) > 0 {
		moduleID = modules[0]
	} else {
		moduleID = "unknown"
	}

	corpus := &ir.Corpus{
		ID:         moduleID,
		Version:    "1.0.0",
		ModuleType: "bible",
		Language:   "en",
		Title:      moduleID,
		Documents:  []*ir.Document{},
	}

	return corpus, nil
}

// Emit converts an IR corpus to CrossWire SWORD module format.
func Emit(corpus *ir.Corpus, outputDir string) (string, error) {
	// Create basic SWORD structure
	// NOTE: For full SWORD emission, this should delegate to format-sword-pure plugin
	// This is a minimal stub implementation
	modsDir := filepath.Join(outputDir, "mods.d")
	modulesDir := filepath.Join(outputDir, "modules")

	if err := os.MkdirAll(modsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create mods.d: %w", err)
	}

	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create modules: %w", err)
	}

	// Create a minimal conf file
	confPath := filepath.Join(modsDir, corpus.ID+".conf")
	confContent := fmt.Sprintf("[%s]\nDescription=Exported from IR\nModDrv=RawText\n", corpus.ID)
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write conf: %w", err)
	}

	// Create a zip archive
	zipPath := filepath.Join(filepath.Dir(outputDir), corpus.ID+".zip")
	if err := createZipArchive(outputDir, zipPath); err != nil {
		return "", fmt.Errorf("failed to create zip: %w", err)
	}

	return zipPath, nil
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
			return nil // Skip the zip file itself
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
