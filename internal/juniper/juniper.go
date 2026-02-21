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

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/capsule"
	"github.com/JuniperBible/Public.Tool.JuniperBible/core/ir"
	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/archive"
	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/fileutil"
	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/safefile"
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

	modsDir := filepath.Join(swordPath, "mods.d")
	entries, err := readModsDir(modsDir, swordPath)
	if err != nil {
		return err
	}

	printModuleHeader(swordPath)
	count := printBibleModules(entries, modsDir)
	fmt.Printf("\nTotal: %d Bible modules\n", count)
	return nil
}

func readModsDir(modsDir, swordPath string) ([]os.DirEntry, error) {
	if _, err := os.Stat(modsDir); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("SWORD installation not found at %s (missing mods.d)", swordPath)
	}
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read mods.d: %w", err)
	}
	return entries, nil
}

func printModuleHeader(swordPath string) {
	fmt.Printf("Bible modules in %s:\n\n", swordPath)
	fmt.Printf("%-15s %-8s %-40s\n", "MODULE", "LANG", "DESCRIPTION")
	fmt.Printf("%-15s %-8s %-40s\n", "------", "----", "-----------")
}

func printBibleModules(entries []os.DirEntry, modsDir string) int {
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}
		if printModuleIfBible(filepath.Join(modsDir, e.Name())) {
			count++
		}
	}
	return count
}

func printModuleIfBible(confPath string) bool {
	module := ParseConf(confPath)
	if module == nil || module.ModType != "Bible" {
		return false
	}
	desc := truncateDesc(module.Description, 40)
	encrypted := ""
	if module.Encrypted {
		encrypted = " [encrypted]"
	}
	fmt.Printf("%-15s %-8s %-40s%s\n", module.Name, module.Lang, desc, encrypted)
	return true
}

func truncateDesc(desc string, maxLen int) string {
	if len(desc) > maxLen {
		return desc[:maxLen-3] + "..."
	}
	return desc
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

// selectModules resolves which modules to process given an all-flag, an
// explicit name list, and the full set of available modules.  It prints a
// warning for any requested name that is not found.
func selectModules(all bool, names []string, available []*Module, noneMsg string) ([]*Module, error) {
	if all {
		return available, nil
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("specify module names or use --all")
	}
	index := make(map[string]*Module, len(available))
	for _, m := range available {
		index[m.Name] = m
	}
	var selected []*Module
	for _, name := range names {
		if m, ok := index[name]; ok {
			selected = append(selected, m)
		} else {
			fmt.Printf("Warning: module '%s' not found\n", name)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("%s", noneMsg)
	}
	return selected, nil
}

// ingestSingleModule ingests one non-encrypted module and reports the result.
func ingestSingleModule(swordPath, outputDir string, m *Module) {
	if m.Encrypted {
		fmt.Printf("Skipping %s (encrypted)\n", m.Name)
		return
	}
	capsulePath := filepath.Join(outputDir, m.Name+".capsule.tar.gz")
	fmt.Printf("Creating %s...\n", capsulePath)
	if err := IngestModule(swordPath, m, capsulePath); err != nil {
		fmt.Printf("  Error: %v\n", err)
		return
	}
	if info, _ := os.Stat(capsulePath); info != nil {
		fmt.Printf("  Created: %s (%d bytes)\n", capsulePath, info.Size())
	}
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

	toIngest, err := selectModules(cfg.All, cfg.Modules, modules, "no modules to ingest")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cfg.Output, 0700); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("Ingesting %d module(s) to %s/\n\n", len(toIngest), cfg.Output)
	for _, m := range toIngest {
		ingestSingleModule(swordPath, cfg.Output, m)
	}

	fmt.Println("\nDone!")
	return nil
}

// IngestModule creates a capsule from a single SWORD module.
func IngestModule(swordPath string, module *Module, outputPath string) error {
	confData, err := safefile.ReadFile(module.ConfPath)
	if err != nil {
		return fmt.Errorf("failed to read conf: %w", err)
	}

	dataPath, fullDataPath, err := resolveDataPath(swordPath, module.DataPath)
	if err != nil {
		return err
	}

	tempDir, err := os.MkdirTemp("", "sword-capsule-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	if err := setupCapsuleStructure(capsuleDir, module, confData, dataPath, fullDataPath); err != nil {
		return err
	}

	return archive.CreateCapsuleTarGz(capsuleDir, outputPath)
}

func resolveDataPath(swordPath, dataPath string) (string, string, error) {
	if dataPath == "" {
		return "", "", fmt.Errorf("no DataPath in conf file")
	}
	dataPath = strings.TrimPrefix(dataPath, "./")
	fullDataPath := filepath.Join(swordPath, dataPath)
	if _, err := os.Stat(fullDataPath); errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("data path not found: %s", fullDataPath)
	}
	return dataPath, fullDataPath, nil
}

func setupCapsuleStructure(capsuleDir string, module *Module, confData []byte, dataPath, fullDataPath string) error {
	modsDir := filepath.Join(capsuleDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		return fmt.Errorf("failed to create mods.d: %w", err)
	}

	confName := strings.ToLower(module.Name) + ".conf"
	if err := os.WriteFile(filepath.Join(modsDir, confName), confData, 0600); err != nil {
		return fmt.Errorf("failed to write conf: %w", err)
	}

	destDataPath := filepath.Join(capsuleDir, dataPath)
	if err := os.MkdirAll(filepath.Dir(destDataPath), 0700); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}
	if err := fileutil.CopyDir(fullDataPath, destDataPath); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	return writeModuleManifest(capsuleDir, module)
}

func writeModuleManifest(capsuleDir string, module *Module) error {
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
	return os.WriteFile(filepath.Join(capsuleDir, "manifest.json"), manifestData, 0600)
}

// InstallConfig holds configuration for installing SWORD modules as capsules with IR.
type InstallConfig struct {
	Path       string   // SWORD installation path
	Output     string   // Output directory for capsules
	Modules    []string // Specific modules to install
	All        bool     // Install all modules
	PluginsDir string   // Directory containing format plugins
}

// installSingleModule ingests one module and generates its IR.
// It returns true when both steps succeed.
func installSingleModule(swordPath, outputDir, pluginsDir string, m *Module) bool {
	if m.Encrypted {
		fmt.Printf("Skipping %s (encrypted)\n", m.Name)
		return false
	}
	capsulePath := filepath.Join(outputDir, m.Name+".capsule.tar.gz")
	fmt.Printf("Installing %s...\n", m.Name)

	fmt.Printf("  Ingesting SWORD module...\n")
	if err := IngestModule(swordPath, m, capsulePath); err != nil {
		fmt.Printf("  Error during ingest: %v\n", err)
		return false
	}

	fmt.Printf("  Generating IR...\n")
	if err := GenerateIRForCapsule(capsulePath, pluginsDir); err != nil {
		fmt.Printf("  Error during IR generation: %v\n", err)
		fmt.Printf("  (Capsule created but without IR)\n")
		return false
	}

	if info, _ := os.Stat(capsulePath); info != nil {
		fmt.Printf("  Done: %s (%d bytes)\n", capsulePath, info.Size())
	}
	return true
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

	toInstall, err := selectModules(cfg.All, cfg.Modules, modules, "no modules to install")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cfg.Output, 0700); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("Installing %d module(s) to %s/ (with IR generation)\n\n", len(toInstall), cfg.Output)

	successful := 0
	for _, m := range toInstall {
		if installSingleModule(swordPath, cfg.Output, cfg.PluginsDir, m) {
			successful++
		}
	}

	fmt.Printf("\nInstalled %d/%d modules successfully\n", successful, len(toInstall))
	return nil
}

// GenerateIRForCapsule generates IR for an existing capsule.
func GenerateIRForCapsule(capsulePath string, pluginsDir string) error {
	if archive.HasIR(capsulePath) {
		return nil
	}

	tempDir, err := os.MkdirTemp("", "capsule-ir-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	extractDir := filepath.Join(tempDir, "extract")
	if _, err := capsule.Unpack(capsulePath, extractDir); err != nil {
		return fmt.Errorf("failed to extract capsule: %w", err)
	}

	extractResult, sourceFormat, err := extractIRFromCapsuleContent(extractDir, filepath.Join(tempDir, "ir"), pluginsDir)
	if err != nil {
		return err
	}

	return buildAndReplaceCapsule(capsulePath, extractDir, filepath.Join(tempDir, "new-capsule"), extractResult, sourceFormat)
}

func extractIRFromCapsuleContent(extractDir, irDir, pluginsDir string) (*extractIRResult, string, error) {
	contentPath, sourceFormat := findConvertibleContent(extractDir)
	if contentPath == "" {
		return nil, "", fmt.Errorf("no convertible content found (supported: OSIS, USFM, USX, SWORD)")
	}

	os.MkdirAll(irDir, 0700)

	sourcePlugin, err := getPluginLoader(pluginsDir).GetPlugin("format." + sourceFormat)
	if err != nil {
		return nil, "", fmt.Errorf("no plugin for format '%s': %w", sourceFormat, err)
	}

	extractResp, err := executePlugin(sourcePlugin, newExtractIRRequest(contentPath, irDir))
	if err != nil {
		return nil, "", fmt.Errorf("IR extraction failed: %w", err)
	}

	extractResult, err := parseExtractIRResult(extractResp)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse result: %w", err)
	}

	return extractResult, sourceFormat, nil
}

func buildAndReplaceCapsule(capsulePath, extractDir, newCapsuleDir string, result *extractIRResult, sourceFormat string) error {
	if err := fileutil.CopyDir(extractDir, newCapsuleDir); err != nil {
		return fmt.Errorf("failed to copy contents: %w", err)
	}

	if err := writeIRToCapsuleDir(newCapsuleDir, capsulePath, result, sourceFormat); err != nil {
		return err
	}

	return replaceCapsuleArchive(capsulePath, newCapsuleDir)
}

func writeIRToCapsuleDir(newCapsuleDir, capsulePath string, result *extractIRResult, sourceFormat string) error {
	irData, err := safefile.ReadFile(result.IRPath)
	if err != nil {
		return fmt.Errorf("failed to read IR: %w", err)
	}

	baseName := deriveCapsuleBaseName(capsulePath)
	os.WriteFile(filepath.Join(newCapsuleDir, baseName+".ir.json"), irData, 0600)

	manifestPath := filepath.Join(newCapsuleDir, "manifest.json")
	manifest := make(map[string]interface{})
	if data, err := safefile.ReadFile(manifestPath); err == nil {
		json.Unmarshal(data, &manifest)
	}
	manifest["has_ir"] = true
	manifest["source_format"] = sourceFormat
	manifest["ir_loss_class"] = result.LossClass
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(manifestPath, manifestData, 0600)
	return nil
}

func replaceCapsuleArchive(capsulePath, newCapsuleDir string) error {
	oldPath := capsulePath + ".old"
	if err := os.Rename(capsulePath, oldPath); err != nil {
		return fmt.Errorf("failed to rename original: %w", err)
	}

	if err := archive.CreateCapsuleTarGz(newCapsuleDir, capsulePath); err != nil {
		os.Rename(oldPath, capsulePath)
		return fmt.Errorf("failed to create capsule: %w", err)
	}

	os.Remove(oldPath)
	return nil
}

func deriveCapsuleBaseName(capsulePath string) string {
	baseName := filepath.Base(capsulePath)
	for _, suffix := range []string{".capsule.tar.gz", ".capsule.tar.xz", ".tar.gz", ".tar.xz"} {
		if strings.HasSuffix(baseName, suffix) {
			return strings.TrimSuffix(baseName, suffix)
		}
	}
	return baseName
}

// CASToSword converts a CAS capsule to a SWORD module.
func CASToSword(cfg CASToSwordConfig) error {
	capsulePath, outputDir, moduleName, err := resolveCASToSwordPaths(cfg)
	if err != nil {
		return err
	}

	printConversionHeader(capsulePath, outputDir, moduleName)

	corpus, err := loadIRFromCapsule(capsulePath)
	if err != nil {
		return err
	}

	meta := extractCorpusMetadata(corpus, moduleName)
	printCorpusInfo(corpus, meta)

	confPath, dataPath, err := createSwordModuleStructure(outputDir, moduleName, meta)
	if err != nil {
		return err
	}

	printConversionComplete(confPath, dataPath)
	return nil
}

// resolveCASToSwordPaths resolves and validates paths for CAS-to-SWORD conversion.
func resolveCASToSwordPaths(cfg CASToSwordConfig) (capsulePath, outputDir, moduleName string, err error) {
	capsulePath, _ = filepath.Abs(cfg.Capsule)
	outputDir = cfg.Output
	moduleName = cfg.Name

	if outputDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		outputDir = filepath.Join(home, ".sword")
	}

	if moduleName == "" {
		moduleName = deriveModuleName(capsulePath)
	}
	return capsulePath, outputDir, moduleName, nil
}

// deriveModuleName extracts a module name from the capsule filename.
func deriveModuleName(capsulePath string) string {
	base := filepath.Base(capsulePath)
	name := strings.TrimSuffix(base, ".capsule.tar.xz")
	name = strings.TrimSuffix(name, ".capsule.tar.gz")
	name = strings.TrimSuffix(name, ".tar.xz")
	name = strings.TrimSuffix(name, ".tar.gz")
	return strings.ToUpper(name)
}

// loadIRFromCapsule unpacks a capsule and loads the IR corpus.
func loadIRFromCapsule(capsulePath string) (*ir.Corpus, error) {
	tempDir, err := os.MkdirTemp("", "cas-to-sword-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	cap, err := capsule.Unpack(capsulePath, tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack capsule: %w", err)
	}

	if len(cap.Manifest.IRExtractions) == 0 {
		return nil, fmt.Errorf("capsule has no IR - run 'capsule format ir generate' first")
	}

	var irRecord *capsule.IRRecord
	for _, rec := range cap.Manifest.IRExtractions {
		irRecord = rec
		break
	}

	irBlobData, err := cap.GetStore().Retrieve(irRecord.IRBlobSHA256)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve IR blob: %w", err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(irBlobData, &corpus); err != nil {
		return nil, fmt.Errorf("failed to parse IR corpus: %w", err)
	}
	return &corpus, nil
}

type swordModuleMeta struct {
	lang, description, versification string
}

// extractCorpusMetadata extracts SWORD metadata from an IR corpus.
func extractCorpusMetadata(corpus *ir.Corpus, moduleName string) swordModuleMeta {
	meta := swordModuleMeta{lang: "en", description: moduleName + " Bible Module", versification: "KJV"}
	if corpus.Language != "" {
		meta.lang = corpus.Language
	}
	if corpus.Title != "" {
		meta.description = corpus.Title
	}
	if corpus.Versification != "" {
		meta.versification = corpus.Versification
	}
	return meta
}

// createSwordModuleStructure creates the SWORD module directories and conf file.
func createSwordModuleStructure(outputDir, moduleName string, meta swordModuleMeta) (confPath, dataPath string, err error) {
	modsDir := filepath.Join(outputDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		return "", "", fmt.Errorf("failed to create mods.d: %w", err)
	}

	dataPath = filepath.Join("modules", "texts", "ztext", strings.ToLower(moduleName))
	fullDataPath := filepath.Join(outputDir, dataPath)
	if err := os.MkdirAll(fullDataPath, 0700); err != nil {
		return "", "", fmt.Errorf("failed to create data directory: %w", err)
	}

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
`, moduleName, dataPath, meta.lang, meta.description, meta.versification)

	confPath = filepath.Join(modsDir, strings.ToLower(moduleName)+".conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		return "", "", fmt.Errorf("failed to write conf file: %w", err)
	}
	return confPath, fullDataPath, nil
}

func printConversionHeader(capsulePath, outputDir, moduleName string) {
	fmt.Printf("Converting CAS capsule to SWORD module:\n")
	fmt.Printf("  Input:  %s\n", capsulePath)
	fmt.Printf("  Output: %s\n", outputDir)
	fmt.Printf("  Module: %s\n", moduleName)
	fmt.Println()
}

func printCorpusInfo(corpus *ir.Corpus, meta swordModuleMeta) {
	fmt.Printf("  IR ID:       %s\n", corpus.ID)
	fmt.Printf("  Language:    %s\n", meta.lang)
	fmt.Printf("  Title:       %s\n", meta.description)
	fmt.Printf("  Versification: %s\n", meta.versification)
	fmt.Printf("  Documents:   %d\n", len(corpus.Documents))
	fmt.Println()
}

func printConversionComplete(confPath, dataPath string) {
	fmt.Printf("Created SWORD module structure:\n")
	fmt.Printf("  Config: %s\n", confPath)
	fmt.Printf("  Data:   %s\n", dataPath)
	fmt.Println()
	fmt.Println("Note: To complete the conversion, use osis2mod or sword-utils to populate the module data.")
	fmt.Println("      This command creates the structure; use tool plugins for data conversion.")
}

// modTypeFromDriver maps a SWORD ModDrv value to a human-readable module type.
var modTypeFromDriver = map[string]string{
	"zText":      "Bible",
	"RawText":    "Bible",
	"zText4":     "Bible",
	"RawText4":   "Bible",
	"zCom":       "Commentary",
	"RawCom":     "Commentary",
	"zCom4":      "Commentary",
	"RawCom4":    "Commentary",
	"zLD":        "Dictionary",
	"RawLD":      "Dictionary",
	"RawLD4":     "Dictionary",
	"RawGenBook": "GenBook",
}

// applyConfKeyValue applies a single key=value pair from a SWORD conf file to
// the module being built.
func applyConfKeyValue(m *Module, key, value string) {
	switch key {
	case "Description":
		m.Description = value
	case "Lang":
		m.Lang = value
	case "ModDrv":
		if t, ok := modTypeFromDriver[value]; ok {
			m.ModType = t
		} else {
			m.ModType = "Unknown"
		}
	case "DataPath":
		m.DataPath = value
	case "CipherKey":
		m.Encrypted = value != ""
	}
}

// ParseConf parses a SWORD conf file.
func ParseConf(path string) *Module {
	data, err := safefile.ReadFile(path)
	if err != nil {
		return nil
	}

	module := &Module{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		if line[0] == '[' && line[len(line)-1] == ']' {
			module.Name = line[1 : len(line)-1]
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		applyConfKeyValue(module, strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]))
	}

	return module
}

// Helper functions for IR generation

var globFormatPatterns = []struct {
	pattern string
	format  string
}{
	{"*.osis", "osis"},
	{"*.osis.xml", "osis"},
	{"*.usfm", "usfm"},
	{"*.sfm", "usfm"},
	{"*.usx", "usx"},
}

func findSWORDConf(extractDir string) string {
	modsDir := filepath.Join(extractDir, "mods.d")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".conf") {
			return filepath.Join(modsDir, e.Name())
		}
	}
	return ""
}

func findConvertibleContent(extractDir string) (contentPath, format string) {
	if path := findSWORDConf(extractDir); path != "" {
		return path, "sword-pure"
	}
	for _, p := range globFormatPatterns {
		files, _ := filepath.Glob(filepath.Join(extractDir, p.pattern))
		if len(files) > 0 {
			return files[0], p.format
		}
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
