// Package migrate handles copying SWORD modules from system directories.
package migrate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/juniper/pkg/sword"
)

// Migrator handles copying SWORD modules to a destination directory.
type Migrator struct {
	SourceDir string
	DestDir   string
	Verbose   bool
}

// MigrateResult contains the results of a migration operation.
type MigrateResult struct {
	ModulesFound   int
	ModulesCopied  int
	ModulesSkipped int
	Errors         []string
}

// NewMigrator creates a new Migrator instance.
func NewMigrator(sourceDir, destDir string) *Migrator {
	return &Migrator{
		SourceDir: sourceDir,
		DestDir:   destDir,
	}
}

// Migrate copies SWORD modules from source to destination.
func (m *Migrator) Migrate(moduleFilter []string) (*MigrateResult, error) {
	result := &MigrateResult{}

	// Ensure destination directories exist
	incomingDir := filepath.Join(m.DestDir, "incoming")
	if err := os.MkdirAll(filepath.Join(incomingDir, "mods.d"), 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directories: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(incomingDir, "modules"), 0755); err != nil {
		return nil, fmt.Errorf("failed to create modules directory: %w", err)
	}

	// Discover source modules
	modules, err := sword.LoadAllModules(m.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load source modules: %w", err)
	}

	result.ModulesFound = len(modules)

	// Filter modules if specified
	filterSet := make(map[string]bool)
	for _, f := range moduleFilter {
		filterSet[strings.ToLower(f)] = true
	}

	for _, mod := range modules {
		// Apply filter
		if len(filterSet) > 0 && !filterSet[strings.ToLower(mod.ID)] {
			result.ModulesSkipped++
			continue
		}

		if err := m.migrateModule(mod, incomingDir); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", mod.ID, err))
			continue
		}

		result.ModulesCopied++
		if m.Verbose {
			fmt.Printf("Migrated: %s (%s)\n", mod.ID, mod.Title)
		}
	}

	return result, nil
}

// migrateModule copies a single module's files.
func (m *Migrator) migrateModule(mod *sword.Module, destDir string) error {
	// Copy .conf file
	confDest := filepath.Join(destDir, "mods.d", filepath.Base(mod.ConfPath))
	if err := copyFile(mod.ConfPath, confDest); err != nil {
		return fmt.Errorf("failed to copy conf file: %w", err)
	}

	// Copy module data directory
	dataPath := mod.ResolveDataPath(m.SourceDir)
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		return fmt.Errorf("data directory not found: %s", dataPath)
	}

	// Determine destination data path
	// DataPath is typically something like "./modules/texts/ztext/kjv/"
	relPath := strings.TrimPrefix(mod.DataPath, "./")
	destDataPath := filepath.Join(destDir, relPath)

	if err := copyDir(dataPath, destDataPath); err != nil {
		return fmt.Errorf("failed to copy data directory: %w", err)
	}

	return nil
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	srcFile, err := os.Open(src) // #nosec G304 -- path is validated
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// ListModules returns a list of available modules in the source directory.
func (m *Migrator) ListModules() ([]*sword.Module, error) {
	return sword.LoadAllModules(m.SourceDir)
}
