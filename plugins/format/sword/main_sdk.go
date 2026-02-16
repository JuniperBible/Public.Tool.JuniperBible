//go:build sdk

// Plugin format-sword handles SWORD Bible module ingestion using the SDK pattern.
// It detects SWORD module directories by looking for mods.d/*.conf files
// and modules/* data directories.
//
// IR Support:
// - extract-ir: Extracts IR from SWORD module (L2 - requires libsword for full text)
// - emit-native: Converts IR back to SWORD module format (L2)
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// SwordModule represents parsed SWORD module metadata.
type SwordModule struct {
	Name        string
	Description string
	Version     string
	DataPath    string
	ConfPath    string
	ModDrv      string // Module driver type (zText, RawText, etc.)
	Lang        string
	Encoding    string
}

func mainSDK() error {
	return format.Run(&format.Config{
		Name:       "sword",
		Extensions: []string{}, // SWORD uses directories, not file extensions
		Detect:     detectSwordModule,
		Parse:      parseSwordModule,
		Emit:       emitSwordModule,
		Enumerate:  enumerateSwordModule,
	})
}

// detectSwordModule detects SWORD module directories.
func detectSwordModule(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		}, nil
	}

	if !info.IsDir() {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "path is not a directory",
		}, nil
	}

	// Check for SWORD module structure
	// Look for mods.d/ directory with .conf files
	modsD := filepath.Join(path, "mods.d")
	if _, err := os.Stat(modsD); errors.Is(err, os.ErrNotExist) {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "no mods.d directory found",
		}, nil
	}

	// Check for at least one .conf file
	confFiles, err := filepath.Glob(filepath.Join(modsD, "*.conf"))
	if err != nil || len(confFiles) == 0 {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "no .conf files in mods.d/",
		}, nil
	}

	// Check for modules/ directory
	modulesDir := filepath.Join(path, "modules")
	if _, err := os.Stat(modulesDir); errors.Is(err, os.ErrNotExist) {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "no modules directory found",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: true,
		Format:   "sword",
		Reason:   fmt.Sprintf("SWORD module detected: %d .conf file(s)", len(confFiles)),
	}, nil
}

// parseSwordModule parses a SWORD module directory and returns an IR Corpus.
func parseSwordModule(path string) (*ir.Corpus, error) {
	// Parse all SWORD modules
	modules, err := parseSwordModules(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse modules: %w", err)
	}

	if len(modules) == 0 {
		return nil, fmt.Errorf("no SWORD modules found")
	}

	// Create IR corpus from module metadata
	// Note: Full text extraction requires libsword via tool-libsword plugin
	module := modules[0]
	corpus := &ir.Corpus{
		ID:           module.Name,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		Language:     module.Lang,
		Title:        module.Description,
		SourceFormat: "SWORD",
		LossClass:    "L2", // L2 because we can't access full text without libsword
		Attributes:   make(map[string]string),
	}

	// Store module metadata for potential reconstruction
	corpus.Attributes["_sword_module_name"] = module.Name
	corpus.Attributes["_sword_data_path"] = module.DataPath
	corpus.Attributes["_sword_mod_drv"] = module.ModDrv
	corpus.Attributes["_sword_version"] = module.Version
	if module.Encoding != "" {
		corpus.Attributes["_sword_encoding"] = module.Encoding
	}

	// Read conf file content for L0 reconstruction
	if confData, err := safefile.ReadFile(module.ConfPath); err == nil {
		corpus.Attributes["_sword_conf"] = string(confData)
	}

	// Create a placeholder document
	corpus.Documents = []*ir.Document{
		{
			ID:    module.Name,
			Title: module.Description,
			Order: 1,
			Attributes: map[string]string{
				"note": "Full text extraction requires tool-libsword plugin",
			},
		},
	}

	// Compute source hash from conf file
	if confData, err := safefile.ReadFile(module.ConfPath); err == nil {
		h := sha256.Sum256(confData)
		corpus.SourceHash = hex.EncodeToString(h[:])
	}

	return corpus, nil
}

// emitSwordModule converts an IR Corpus to SWORD module format.
func emitSwordModule(corpus *ir.Corpus, outputDir string) (string, error) {
	// Create SWORD module directory structure
	moduleDir := filepath.Join(outputDir, corpus.ID)
	modsD := filepath.Join(moduleDir, "mods.d")
	modulesDir := filepath.Join(moduleDir, "modules")

	if err := os.MkdirAll(modsD, 0755); err != nil {
		return "", fmt.Errorf("failed to create mods.d: %w", err)
	}
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create modules: %w", err)
	}

	// Check if we have the original conf file
	if confContent, ok := corpus.Attributes["_sword_conf"]; ok && confContent != "" {
		// Write original conf file (L0 for conf)
		confPath := filepath.Join(modsD, strings.ToLower(corpus.ID)+".conf")
		if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
			return "", fmt.Errorf("failed to write conf: %w", err)
		}
	} else {
		// Generate minimal conf file
		var confBuf strings.Builder
		confBuf.WriteString(fmt.Sprintf("[%s]\n", corpus.ID))
		if dataPath, ok := corpus.Attributes["_sword_data_path"]; ok {
			confBuf.WriteString(fmt.Sprintf("DataPath=%s\n", dataPath))
		} else {
			confBuf.WriteString(fmt.Sprintf("DataPath=./modules/texts/ztext/%s/\n", strings.ToLower(corpus.ID)))
		}
		if modDrv, ok := corpus.Attributes["_sword_mod_drv"]; ok {
			confBuf.WriteString(fmt.Sprintf("ModDrv=%s\n", modDrv))
		} else {
			confBuf.WriteString("ModDrv=zText\n")
		}
		if corpus.Title != "" {
			confBuf.WriteString(fmt.Sprintf("Description=%s\n", corpus.Title))
		}
		if corpus.Language != "" {
			confBuf.WriteString(fmt.Sprintf("Lang=%s\n", corpus.Language))
		}
		if version, ok := corpus.Attributes["_sword_version"]; ok {
			confBuf.WriteString(fmt.Sprintf("Version=%s\n", version))
		}
		if encoding, ok := corpus.Attributes["_sword_encoding"]; ok {
			confBuf.WriteString(fmt.Sprintf("Encoding=%s\n", encoding))
		}

		confPath := filepath.Join(modsD, strings.ToLower(corpus.ID)+".conf")
		if err := os.WriteFile(confPath, []byte(confBuf.String()), 0644); err != nil {
			return "", fmt.Errorf("failed to write conf: %w", err)
		}
	}

	return moduleDir, nil
}

// enumerateSwordModule lists contents of a SWORD module directory.
func enumerateSwordModule(path string) (*ipc.EnumerateResult, error) {
	var entries []ipc.EnumerateEntry

	// Parse modules first
	modules, _ := parseSwordModules(path)
	moduleMap := make(map[string]*SwordModule)
	for _, m := range modules {
		moduleMap[m.ConfPath] = m
	}

	// Walk the directory
	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(path, p)
		if rel == "." {
			return nil
		}

		entry := ipc.EnumerateEntry{
			Path:      rel,
			SizeBytes: info.Size(),
			IsDir:     info.IsDir(),
		}

		// Add metadata for .conf files
		if strings.HasSuffix(rel, ".conf") && strings.HasPrefix(rel, "mods.d/") {
			if m, ok := moduleMap[p]; ok {
				entry.Metadata = map[string]string{
					"module_name":    m.Name,
					"description":    m.Description,
					"module_version": m.Version,
				}
			}
		}

		entries = append(entries, entry)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to enumerate: %w", err)
	}

	return &ipc.EnumerateResult{
		Entries: entries,
	}, nil
}

// parseSwordModules parses all SWORD modules in a directory.
func parseSwordModules(path string) ([]*SwordModule, error) {
	modsD := filepath.Join(path, "mods.d")

	confFiles, err := filepath.Glob(filepath.Join(modsD, "*.conf"))
	if err != nil {
		return nil, err
	}

	var modules []*SwordModule
	for _, confPath := range confFiles {
		m, err := parseConfFile(confPath)
		if err != nil {
			continue // Skip invalid conf files
		}
		modules = append(modules, m)
	}

	return modules, nil
}

// parseConfFile parses a SWORD .conf file.
func parseConfFile(path string) (*SwordModule, error) {
	f, err := safefile.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	module := &SwordModule{
		ConfPath: path,
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse section header [ModuleName]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			module.Name = line[1 : len(line)-1]
			continue
		}

		// Parse key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Description":
			module.Description = value
		case "Version":
			module.Version = value
		case "DataPath":
			module.DataPath = value
		case "ModDrv":
			module.ModDrv = value
		case "Lang":
			module.Lang = value
		case "Encoding":
			module.Encoding = value
		}
	}

	if module.Name == "" {
		return nil, fmt.Errorf("no module name found")
	}

	return module, nil
}

// IngestTransform is handled by the SDK's default behavior,
// which stores the manifest as JSON blob.
// We'll implement custom ingest by storing module manifest.
func ingestSwordModule(path string) ([]byte, map[string]string, error) {
	// Parse all SWORD modules in the directory
	modules, err := parseSwordModules(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse modules: %w", err)
	}

	if len(modules) == 0 {
		return nil, nil, fmt.Errorf("no SWORD modules found")
	}

	// Create a manifest of all modules
	manifest := struct {
		RootPath string         `json:"root_path"`
		Modules  []*SwordModule `json:"modules"`
	}{
		RootPath: filepath.Base(path),
		Modules:  modules,
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	metadata := map[string]string{
		"format":       "sword",
		"module_count": fmt.Sprintf("%d", len(modules)),
	}

	return manifestData, metadata, nil
}
