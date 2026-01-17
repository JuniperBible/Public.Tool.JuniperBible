// Package juniper provides shared logic for Bible/SWORD module tools.
// This package is used by both the standalone capsule-juniper binary
// and the capsule CLI's juniper subcommand.
package juniper

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/capsule"
	"github.com/FocuswithJustin/JuniperBible/core/ir"
	"github.com/FocuswithJustin/JuniperBible/internal/archive"
	"github.com/FocuswithJustin/JuniperBible/internal/fileutil"
)

// Module holds parsed SWORD module metadata.
type Module struct {
	Name        string
	Description string
	Lang        string
	ModType     string
	DataPath    string
	Encrypted   bool
	ConfPath    string
}

// ListConfig holds configuration for listing SWORD modules.
type ListConfig struct {
	Path string // SWORD installation path (default: ~/.sword)
}

// IngestConfig holds configuration for ingesting SWORD modules.
type IngestConfig struct {
	Path    string   // SWORD installation path
	Output  string   // Output directory for capsules
	Modules []string // Specific modules to ingest
	All     bool     // Ingest all modules
}

// CASToSwordConfig holds configuration for CAS-to-SWORD conversion.
type CASToSwordConfig struct {
	Capsule string // Path to CAS capsule
	Output  string // Output directory for SWORD module
	Name    string // Module name
}

// ResolveSwordPath resolves the SWORD installation path.
func ResolveSwordPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".sword"), nil
}

// List lists Bible modules in a SWORD installation.
func List(cfg ListConfig) error {
	swordPath, err := ResolveSwordPath(cfg.Path)
	if err != nil {
		return err
	}

	// Check if mods.d exists
	modsDir := filepath.Join(swordPath, "mods.d")
	if _, err := os.Stat(modsDir); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("SWORD installation not found at %s (missing mods.d)", swordPath)
	}

	// Find all .conf files
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return fmt.Errorf("failed to read mods.d: %w", err)
	}

	fmt.Printf("Bible modules in %s:\n\n", swordPath)
	fmt.Printf("%-15s %-8s %-40s\n", "MODULE", "LANG", "DESCRIPTION")
	fmt.Printf("%-15s %-8s %-40s\n", "------", "----", "-----------")

	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}

		confPath := filepath.Join(modsDir, e.Name())
		module := ParseConf(confPath)
		if module == nil || module.ModType != "Bible" {
			continue
		}

		desc := module.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		encrypted := ""
		if module.Encrypted {
			encrypted = " [encrypted]"
		}
		fmt.Printf("%-15s %-8s %-40s%s\n", module.Name, module.Lang, desc, encrypted)
		count++
	}

	fmt.Printf("\nTotal: %d Bible modules\n", count)
	return nil
}

// ListModules returns all Bible modules in a SWORD installation.
func ListModules(swordPath string) ([]*Module, error) {
	modsDir := filepath.Join(swordPath, "mods.d")
	if _, err := os.Stat(modsDir); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("SWORD installation not found at %s", swordPath)
	}

	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read mods.d: %w", err)
	}

	var modules []*Module
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}

		confPath := filepath.Join(modsDir, e.Name())
		module := ParseConf(confPath)
		if module == nil || module.ModType != "Bible" {
			continue
		}
		module.ConfPath = confPath
		modules = append(modules, module)
	}

	return modules, nil
}

// Ingest ingests SWORD modules into capsules.
func Ingest(cfg IngestConfig) error {
	swordPath, err := ResolveSwordPath(cfg.Path)
	if err != nil {
		return err
	}

	modules, err := ListModules(swordPath)
	if err != nil {
		return err
	}

	if len(modules) == 0 {
		return fmt.Errorf("no Bible modules found in %s", swordPath)
	}

	// Determine which modules to ingest
	var toIngest []*Module
	if cfg.All {
		toIngest = modules
	} else if len(cfg.Modules) > 0 {
		moduleMap := make(map[string]*Module)
		for _, m := range modules {
			moduleMap[m.Name] = m
		}
		for _, name := range cfg.Modules {
			if m, ok := moduleMap[name]; ok {
				toIngest = append(toIngest, m)
			} else {
				fmt.Printf("Warning: module '%s' not found\n", name)
			}
		}
	} else {
		return fmt.Errorf("specify module names or use --all")
	}

	if len(toIngest) == 0 {
		return fmt.Errorf("no modules to ingest")
	}

	// Create output directory
	if err := os.MkdirAll(cfg.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("Ingesting %d module(s) to %s/\n\n", len(toIngest), cfg.Output)

	for _, m := range toIngest {
		if m.Encrypted {
			fmt.Printf("Skipping %s (encrypted)\n", m.Name)
			continue
		}

		capsulePath := filepath.Join(cfg.Output, m.Name+".capsule.tar.gz")
		fmt.Printf("Creating %s...\n", capsulePath)

		if err := IngestModule(swordPath, m, capsulePath); err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		info, _ := os.Stat(capsulePath)
		if info != nil {
			fmt.Printf("  Created: %s (%d bytes)\n", capsulePath, info.Size())
		}
	}

	fmt.Println("\nDone!")
	return nil
}

// IngestModule creates a capsule from a single SWORD module.
func IngestModule(swordPath string, module *Module, outputPath string) error {
	// Read conf file
	confData, err := os.ReadFile(module.ConfPath)
	if err != nil {
		return fmt.Errorf("failed to read conf: %w", err)
	}

	// Determine data path
	dataPath := module.DataPath
	if dataPath == "" {
		return fmt.Errorf("no DataPath in conf file")
	}

	// Clean up data path
	dataPath = strings.TrimPrefix(dataPath, "./")
	fullDataPath := filepath.Join(swordPath, dataPath)

	if _, err := os.Stat(fullDataPath); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("data path not found: %s", fullDataPath)
	}

	// Create temp directory for capsule contents
	tempDir, err := os.MkdirTemp("", "sword-capsule-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Create capsule structure
	capsuleDir := filepath.Join(tempDir, "capsule")
	modsDir := filepath.Join(capsuleDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		return fmt.Errorf("failed to create mods.d: %w", err)
	}

	// Write conf file
	confName := strings.ToLower(module.Name) + ".conf"
	if err := os.WriteFile(filepath.Join(modsDir, confName), confData, 0644); err != nil {
		return fmt.Errorf("failed to write conf: %w", err)
	}

	// Copy module data
	destDataPath := filepath.Join(capsuleDir, dataPath)
	if err := os.MkdirAll(filepath.Dir(destDataPath), 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}
	if err := fileutil.CopyDir(fullDataPath, destDataPath); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	// Create manifest.json
	manifest := map[string]interface{}{
		"capsule_version": "1.0",
		"module_type":     "bible",
		"id":              module.Name,
		"title":           module.Description,
		"language":        module.Lang,
		"source_format":   "sword",
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(capsuleDir, "manifest.json"), manifestData, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Create tar.gz archive
	return archive.CreateCapsuleTarGz(capsuleDir, outputPath)
}

// InstallConfig holds configuration for installing SWORD modules as capsules with IR.
type InstallConfig struct {
	Path       string   // SWORD installation path
	Output     string   // Output directory for capsules
	Modules    []string // Specific modules to install
	All        bool     // Install all modules
	PluginsDir string   // Directory containing format plugins
}

// Install installs SWORD modules as capsules with IR generated.
// This combines ingest + IR generation in one step.
func Install(cfg InstallConfig) error {
	swordPath, err := ResolveSwordPath(cfg.Path)
	if err != nil {
		return err
	}

	modules, err := ListModules(swordPath)
	if err != nil {
		return err
	}

	if len(modules) == 0 {
		return fmt.Errorf("no Bible modules found in %s", swordPath)
	}

	// Determine which modules to install
	var toInstall []*Module
	if cfg.All {
		toInstall = modules
	} else if len(cfg.Modules) > 0 {
		moduleMap := make(map[string]*Module)
		for _, m := range modules {
			moduleMap[m.Name] = m
		}
		for _, name := range cfg.Modules {
			if m, ok := moduleMap[name]; ok {
				toInstall = append(toInstall, m)
			} else {
				fmt.Printf("Warning: module '%s' not found\n", name)
			}
		}
	} else {
		return fmt.Errorf("specify module names or use --all")
	}

	if len(toInstall) == 0 {
		return fmt.Errorf("no modules to install")
	}

	// Create output directory
	if err := os.MkdirAll(cfg.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("Installing %d module(s) to %s/ (with IR generation)\n\n", len(toInstall), cfg.Output)

	successful := 0
	for _, m := range toInstall {
		if m.Encrypted {
			fmt.Printf("Skipping %s (encrypted)\n", m.Name)
			continue
		}

		capsulePath := filepath.Join(cfg.Output, m.Name+".capsule.tar.gz")
		fmt.Printf("Installing %s...\n", m.Name)

		// Step 1: Ingest
		fmt.Printf("  Ingesting SWORD module...\n")
		if err := IngestModule(swordPath, m, capsulePath); err != nil {
			fmt.Printf("  Error during ingest: %v\n", err)
			continue
		}

		// Step 2: Generate IR
		fmt.Printf("  Generating IR...\n")
		if err := GenerateIRForCapsule(capsulePath, cfg.PluginsDir); err != nil {
			fmt.Printf("  Error during IR generation: %v\n", err)
			fmt.Printf("  (Capsule created but without IR)\n")
			continue
		}

		info, _ := os.Stat(capsulePath)
		if info != nil {
			fmt.Printf("  Done: %s (%d bytes)\n", capsulePath, info.Size())
		}
		successful++
	}

	fmt.Printf("\nInstalled %d/%d modules successfully\n", successful, len(toInstall))
	return nil
}

// GenerateIRForCapsule generates IR for an existing capsule.
func GenerateIRForCapsule(capsulePath string, pluginsDir string) error {
	// Check if already has IR
	if archive.HasIR(capsulePath) {
		return nil // Already has IR
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "capsule-ir-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract capsule
	extractDir := filepath.Join(tempDir, "extract")
	if _, err := capsule.Unpack(capsulePath, extractDir); err != nil {
		return fmt.Errorf("failed to extract capsule: %w", err)
	}

	// Find content and detect format
	contentPath, sourceFormat := findConvertibleContent(extractDir)
	if contentPath == "" {
		return fmt.Errorf("no convertible content found (supported: OSIS, USFM, USX, SWORD)")
	}

	// Load plugins using embedded registry
	loader := getPluginLoader(pluginsDir)

	// Extract IR
	irDir := filepath.Join(tempDir, "ir")
	os.MkdirAll(irDir, 0755)

	sourcePlugin, err := loader.GetPlugin("format." + sourceFormat)
	if err != nil {
		return fmt.Errorf("no plugin for format '%s': %w", sourceFormat, err)
	}

	extractReq := newExtractIRRequest(contentPath, irDir)
	extractResp, err := executePlugin(sourcePlugin, extractReq)
	if err != nil {
		return fmt.Errorf("IR extraction failed: %w", err)
	}

	extractResult, err := parseExtractIRResult(extractResp)
	if err != nil {
		return fmt.Errorf("failed to parse result: %w", err)
	}

	// Create new capsule with IR
	newCapsuleDir := filepath.Join(tempDir, "new-capsule")

	// Copy all original files
	if err := fileutil.CopyDir(extractDir, newCapsuleDir); err != nil {
		return fmt.Errorf("failed to copy contents: %w", err)
	}

	// Add IR file
	irData, err := os.ReadFile(extractResult.IRPath)
	if err != nil {
		return fmt.Errorf("failed to read IR: %w", err)
	}

	baseName := filepath.Base(capsulePath)
	baseName = strings.TrimSuffix(baseName, ".capsule.tar.gz")
	baseName = strings.TrimSuffix(baseName, ".capsule.tar.xz")
	baseName = strings.TrimSuffix(baseName, ".tar.gz")
	baseName = strings.TrimSuffix(baseName, ".tar.xz")
	os.WriteFile(filepath.Join(newCapsuleDir, baseName+".ir.json"), irData, 0644)

	// Update manifest
	manifestPath := filepath.Join(newCapsuleDir, "manifest.json")
	manifest := make(map[string]interface{})
	if data, err := os.ReadFile(manifestPath); err == nil {
		json.Unmarshal(data, &manifest)
	}
	manifest["has_ir"] = true
	manifest["source_format"] = sourceFormat
	manifest["ir_loss_class"] = extractResult.LossClass
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(manifestPath, manifestData, 0644)

	// Rename original and create new
	oldPath := capsulePath + ".old"
	if err := os.Rename(capsulePath, oldPath); err != nil {
		return fmt.Errorf("failed to rename original: %w", err)
	}

	if err := archive.CreateCapsuleTarGz(newCapsuleDir, capsulePath); err != nil {
		os.Rename(oldPath, capsulePath)
		return fmt.Errorf("failed to create capsule: %w", err)
	}

	// Remove old file
	os.Remove(oldPath)

	return nil
}

// CASToSword converts a CAS capsule to a SWORD module.
func CASToSword(cfg CASToSwordConfig) error {
	capsulePath, _ := filepath.Abs(cfg.Capsule)
	outputDir := cfg.Output
	moduleName := cfg.Name

	// Default output to ~/.sword
	if outputDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		outputDir = filepath.Join(home, ".sword")
	}

	// Derive module name from capsule filename if not provided
	if moduleName == "" {
		base := filepath.Base(capsulePath)
		moduleName = strings.TrimSuffix(base, ".capsule.tar.xz")
		moduleName = strings.TrimSuffix(moduleName, ".capsule.tar.gz")
		moduleName = strings.TrimSuffix(moduleName, ".tar.xz")
		moduleName = strings.TrimSuffix(moduleName, ".tar.gz")
		moduleName = strings.ToUpper(moduleName)
	}

	fmt.Printf("Converting CAS capsule to SWORD module:\n")
	fmt.Printf("  Input:  %s\n", capsulePath)
	fmt.Printf("  Output: %s\n", outputDir)
	fmt.Printf("  Module: %s\n", moduleName)
	fmt.Println()

	// Create temporary directory for unpacking
	tempDir, err := os.MkdirTemp("", "cas-to-sword-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Unpack the capsule
	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		return fmt.Errorf("failed to unpack capsule: %w", err)
	}

	// Check if capsule has IR extractions
	if len(cap.Manifest.IRExtractions) == 0 {
		return fmt.Errorf("capsule has no IR - run 'capsule format ir generate' first")
	}

	// Get the first IR extraction and directly read the IR blob
	var irRecord *capsule.IRRecord
	for _, rec := range cap.Manifest.IRExtractions {
		irRecord = rec
		break
	}

	// Directly retrieve the IR blob from CAS
	irBlobData, err := cap.GetStore().Retrieve(irRecord.IRBlobSHA256)
	if err != nil {
		return fmt.Errorf("failed to retrieve IR blob: %w", err)
	}

	// Parse the IR corpus directly
	var corpus ir.Corpus
	if err := json.Unmarshal(irBlobData, &corpus); err != nil {
		return fmt.Errorf("failed to parse IR corpus: %w", err)
	}

	// Get metadata directly from IR Corpus
	lang := corpus.Language
	if lang == "" {
		lang = "en"
	}
	description := corpus.Title
	if description == "" {
		description = moduleName + " Bible Module"
	}
	versification := corpus.Versification
	if versification == "" {
		versification = "KJV"
	}

	fmt.Printf("  IR ID:       %s\n", corpus.ID)
	fmt.Printf("  Language:    %s\n", lang)
	fmt.Printf("  Title:       %s\n", description)
	fmt.Printf("  Versification: %s\n", versification)
	fmt.Printf("  Documents:   %d\n", len(corpus.Documents))
	fmt.Println()

	// Ensure output directories exist
	modsDir := filepath.Join(outputDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		return fmt.Errorf("failed to create mods.d: %w", err)
	}

	// Create a zText format module structure
	dataPath := filepath.Join("modules", "texts", "ztext", strings.ToLower(moduleName))
	fullDataPath := filepath.Join(outputDir, dataPath)
	if err := os.MkdirAll(fullDataPath, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create a basic .conf file with versification from IR
	confContent := fmt.Sprintf(`[%s]
DataPath=./%s/
ModDrv=zText
SourceType=OSIS
Encoding=UTF-8
Lang=%s
Description=%s
About=Converted from CAS capsule using Juniper Bible
Category=Biblical Texts
TextSource=Juniper Bible
Versification=%s
Version=1.0
LCSH=Bible.
DistributionLicense=Copyrighted; Free non-commercial distribution
`, moduleName, dataPath, lang, description, versification)

	confPath := filepath.Join(modsDir, strings.ToLower(moduleName)+".conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		return fmt.Errorf("failed to write conf file: %w", err)
	}

	fmt.Printf("Created SWORD module structure:\n")
	fmt.Printf("  Config: %s\n", confPath)
	fmt.Printf("  Data:   %s\n", fullDataPath)
	fmt.Println()
	fmt.Println("Note: To complete the conversion, use osis2mod or sword-utils to populate the module data.")
	fmt.Println("      This command creates the structure; use tool plugins for data conversion.")

	return nil
}

// ParseConf parses a SWORD conf file.
func ParseConf(path string) *Module {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	module := &Module{}
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}

		// Parse section header
		if line[0] == '[' && line[len(line)-1] == ']' {
			module.Name = line[1 : len(line)-1]
			continue
		}

		// Parse key=value
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		switch key {
		case "Description":
			module.Description = value
		case "Lang":
			module.Lang = value
		case "ModDrv":
			switch value {
			case "zText", "RawText", "zText4", "RawText4":
				module.ModType = "Bible"
			case "zCom", "RawCom", "zCom4", "RawCom4":
				module.ModType = "Commentary"
			case "zLD", "RawLD", "RawLD4":
				module.ModType = "Dictionary"
			case "RawGenBook":
				module.ModType = "GenBook"
			default:
				module.ModType = "Unknown"
			}
		case "DataPath":
			module.DataPath = value
		case "CipherKey":
			module.Encrypted = value != ""
		}
	}

	return module
}

// Helper functions for IR generation

// findConvertibleContent finds content in a capsule that can be converted to IR.
func findConvertibleContent(extractDir string) (contentPath, format string) {
	// Check for SWORD module (mods.d/*.conf)
	modsDir := filepath.Join(extractDir, "mods.d")
	if entries, err := os.ReadDir(modsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".conf") {
				return filepath.Join(modsDir, e.Name()), "sword-pure"
			}
		}
	}

	// Check for OSIS
	if files, _ := filepath.Glob(filepath.Join(extractDir, "*.osis")); len(files) > 0 {
		return files[0], "osis"
	}
	if files, _ := filepath.Glob(filepath.Join(extractDir, "*.osis.xml")); len(files) > 0 {
		return files[0], "osis"
	}

	// Check for USFM
	if files, _ := filepath.Glob(filepath.Join(extractDir, "*.usfm")); len(files) > 0 {
		return files[0], "usfm"
	}
	if files, _ := filepath.Glob(filepath.Join(extractDir, "*.sfm")); len(files) > 0 {
		return files[0], "usfm"
	}

	// Check for USX
	if files, _ := filepath.Glob(filepath.Join(extractDir, "*.usx")); len(files) > 0 {
		return files[0], "usx"
	}

	return "", ""
}

// extractIRResult holds the result of IR extraction.
type extractIRResult struct {
	IRPath    string
	LossClass string
}

// getPluginLoader returns a plugin loader with embedded plugins.
func getPluginLoader(pluginsDir string) pluginLoader {
	return &embeddedPluginLoader{}
}

// pluginLoader interface for loading plugins.
type pluginLoader interface {
	GetPlugin(name string) (plugin, error)
}

// plugin interface for executing plugins.
type plugin interface {
	Execute(request map[string]interface{}) (map[string]interface{}, error)
}

// embeddedPluginLoader uses the embedded plugin registry.
type embeddedPluginLoader struct{}

func (l *embeddedPluginLoader) GetPlugin(name string) (plugin, error) {
	// Use the embedded plugin registry
	p := getEmbeddedPlugin(name)
	if p == nil {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}
	return p, nil
}

// getEmbeddedPlugin returns an embedded plugin by name.
func getEmbeddedPlugin(name string) plugin {
	// Import the embedded registry and get the plugin
	registry := getEmbeddedRegistry()
	if registry == nil {
		return nil
	}
	return registry.Get(name)
}

// embeddedRegistry interface for getting embedded plugins.
type embeddedRegistry interface {
	Get(name string) plugin
}

// Global registry - set by init() in embedded package
var globalRegistry embeddedRegistry

func getEmbeddedRegistry() embeddedRegistry {
	return globalRegistry
}

// SetEmbeddedRegistry sets the global embedded plugin registry.
// Called by the embedded package's init().
func SetEmbeddedRegistry(r embeddedRegistry) {
	globalRegistry = r
}

// newExtractIRRequest creates a request for IR extraction.
func newExtractIRRequest(contentPath, outputDir string) map[string]interface{} {
	return map[string]interface{}{
		"action":     "extract_ir",
		"input_path": contentPath,
		"output_dir": outputDir,
	}
}

// executePlugin executes a plugin with a request.
func executePlugin(p plugin, request map[string]interface{}) (map[string]interface{}, error) {
	return p.Execute(request)
}

// parseExtractIRResult parses the result of IR extraction.
func parseExtractIRResult(resp map[string]interface{}) (*extractIRResult, error) {
	result := &extractIRResult{}

	if irPath, ok := resp["ir_path"].(string); ok {
		result.IRPath = irPath
	} else if outputPath, ok := resp["output_path"].(string); ok {
		result.IRPath = outputPath
	} else {
		return nil, fmt.Errorf("no ir_path in response")
	}

	if lossClass, ok := resp["loss_class"].(string); ok {
		result.LossClass = lossClass
	}

	return result, nil
}
