// Package swordpure contains the public API functions for the sword-pure plugin.
package swordpure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ModuleInfo contains metadata about a SWORD module.
type ModuleInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Language    string `json:"language"`
	Version     string `json:"version"`
	Encoding    string `json:"encoding"`
	DataPath    string `json:"data_path,omitempty"`
	Compressed  bool   `json:"compressed"`
	Encrypted   bool   `json:"encrypted"`
}

// Verse represents a single verse.
type Verse struct {
	Ref  string `json:"ref"`
	Text string `json:"text"`
}

// ListModules lists all SWORD modules at the given path.
func ListModules(path string) ([]ModuleInfo, error) {
	confs, err := LoadModulesFromPath(path)
	if err != nil {
		return nil, err
	}

	var modules []ModuleInfo
	for _, conf := range confs {
		modules = append(modules, ModuleInfo{
			Name:        conf.ModuleName,
			Description: conf.Description,
			Type:        conf.ModuleType(),
			Language:    conf.Lang,
			Version:     conf.Version,
			Encoding:    conf.Encoding,
			DataPath:    conf.DataPath,
			Compressed:  conf.IsCompressed(),
			Encrypted:   conf.IsEncrypted(),
		})
	}

	return modules, nil
}

// RenderVerse renders a specific verse from a module.
func RenderVerse(path, module, refStr string) (string, error) {
	// Parse the reference
	ref, err := ParseRef(refStr)
	if err != nil {
		return "", fmt.Errorf("invalid reference: %w", err)
	}

	// Find the module
	conf, err := findModuleByName(path, module)
	if err != nil {
		return "", fmt.Errorf("module not found: %w", err)
	}

	// Only zText modules are currently supported
	if !strings.HasPrefix(strings.ToLower(conf.ModDrv), "ztext") {
		return "", fmt.Errorf("unsupported module type: %s (only zText supported)", conf.ModDrv)
	}

	// Open the module
	mod, err := OpenZTextModule(conf, path)
	if err != nil {
		return "", fmt.Errorf("failed to open module: %w", err)
	}

	// Get the verse text
	text, err := mod.GetVerseText(ref)
	if err != nil {
		return "", fmt.Errorf("failed to get verse: %w", err)
	}

	return text, nil
}

// RenderAll renders all verses in a module.
func RenderAll(path, module string) ([]Verse, error) {
	// Find the module
	conf, err := findModuleByName(path, module)
	if err != nil {
		return nil, fmt.Errorf("module not found: %w", err)
	}

	// Only zText modules are currently supported
	if !strings.HasPrefix(strings.ToLower(conf.ModDrv), "ztext") {
		return nil, fmt.Errorf("unsupported module type: %s (only zText supported)", conf.ModDrv)
	}

	// Open the module
	mod, err := OpenZTextModule(conf, path)
	if err != nil {
		return nil, fmt.Errorf("failed to open module: %w", err)
	}

	// Get versification
	vers, err := NewVersification(VersificationID(conf.Versification))
	if err != nil {
		// Default to KJV if versification not found
		vers, _ = NewVersification(VersKJV)
	}

	var verses []Verse

	// Iterate through all books in the versification
	for _, book := range vers.Books {
		if book.Name == "" {
			continue
		}

		// Iterate through all chapters
		for chapterIdx, verseCount := range book.Chapters {
			chapter := chapterIdx + 1
			// Iterate through all verses in the chapter
			for verse := 1; verse <= verseCount; verse++ {
				ref := &Ref{Book: book.Name, Chapter: chapter, Verse: verse}
				text, err := mod.GetVerseText(ref)
				if err != nil {
					continue // Skip verses that fail
				}
				if text == "" {
					continue // Skip empty verses
				}
				verses = append(verses, Verse{
					Ref:  fmt.Sprintf("%s %d:%d", book.Name, chapter, verse),
					Text: text,
				})
			}
		}
	}

	return verses, nil
}

// findModuleByName finds a module by its name in a SWORD installation.
func findModuleByName(swordPath, moduleName string) (*ConfFile, error) {
	confs, err := LoadModulesFromPath(swordPath)
	if err != nil {
		return nil, err
	}

	for _, conf := range confs {
		if strings.EqualFold(conf.ModuleName, moduleName) {
			return conf, nil
		}
	}

	return nil, fmt.Errorf("module %q not found", moduleName)
}

// Detect checks if the path contains a SWORD module installation.
func Detect(path string) (bool, string, error) {
	// Check for mods.d directory
	modsDir := filepath.Join(path, "mods.d")
	if _, err := os.Stat(modsDir); errors.Is(err, os.ErrNotExist) {
		return false, "", nil
	}

	// Check for at least one .conf file
	confFiles, err := FindConfFiles(modsDir)
	if err != nil {
		return false, "", err
	}

	if len(confFiles) == 0 {
		return false, "", nil
	}

	// Parse the first conf to determine the format
	conf, err := ParseConfFile(confFiles[0])
	if err != nil {
		return true, "sword", nil // It's SWORD but we couldn't parse the conf
	}

	return true, conf.ModDrv, nil
}
