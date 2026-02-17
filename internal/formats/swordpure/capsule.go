// Package swordpure contains capsule creation and CLI command functions.
package swordpure

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FocuswithJustin/JuniperBible/internal/fileutil"
	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
)

// skipIRExtraction can be set to true in tests to speed up capsule creation.
// This skips the expensive IR extraction step.
var skipIRExtraction = false

// runListCmd is the core logic for the list command, testable with custom I/O.
func runListCmd(args []string, stdout, stderr io.Writer) error {
	// Determine SWORD path
	swordPath := getDefaultSwordPath()
	if len(args) > 2 {
		swordPath = args[2]
	}

	modules, err := ListModules(swordPath)
	if err != nil {
		return fmt.Errorf("failed to list modules: %w", err)
	}

	// Filter to only Bible modules
	var bibles []ModuleInfo
	for _, m := range modules {
		if m.Type == "Bible" {
			bibles = append(bibles, m)
		}
	}

	if len(bibles) == 0 {
		fmt.Fprintf(stdout, "No Bible modules found in %s\n", swordPath)
		return nil
	}

	fmt.Fprintf(stdout, "Bible modules in %s:\n\n", swordPath)
	fmt.Fprintf(stdout, "%-15s %-8s %-40s\n", "MODULE", "LANG", "DESCRIPTION")
	fmt.Fprintf(stdout, "%-15s %-8s %-40s\n", "------", "----", "-----------")
	for _, m := range bibles {
		desc := m.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		encrypted := ""
		if m.Encrypted {
			encrypted = " [encrypted]"
		}
		fmt.Fprintf(stdout, "%-15s %-8s %-40s%s\n", m.Name, m.Language, desc, encrypted)
	}
	fmt.Fprintf(stdout, "\nTotal: %d Bible modules\n", len(bibles))
	return nil
}

// cmdList implements the "list" command to list Bible modules.
func cmdList() {
	if err := runListCmd(os.Args, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// ingestConfig holds parsed command-line configuration for ingestion.
type ingestConfig struct {
	swordPath       string
	outputDir       string
	selectedModules []string
	ingestAll       bool
}

// parseIngestArgs parses command-line arguments for the ingest command.
func parseIngestArgs(args []string) ingestConfig {
	cfg := ingestConfig{
		swordPath: getDefaultSwordPath(),
		outputDir: "capsules",
	}

	for i := 2; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--all" || arg == "-a":
			cfg.ingestAll = true
		case arg == "--output" || arg == "-o":
			if i+1 < len(args) {
				i++
				cfg.outputDir = args[i]
			}
		case arg == "--path" || arg == "-p":
			if i+1 < len(args) {
				i++
				cfg.swordPath = args[i]
			}
		default:
			cfg.selectedModules = append(cfg.selectedModules, arg)
		}
	}
	return cfg
}

// selectModulesToIngest determines which modules to ingest based on configuration.
func selectModulesToIngest(cfg ingestConfig, bibles []ModuleInfo, stdin io.Reader, stdout, stderr io.Writer) ([]ModuleInfo, error) {
	if cfg.ingestAll {
		return bibles, nil
	}

	if len(cfg.selectedModules) > 0 {
		return selectModulesByName(cfg.selectedModules, bibles, stderr), nil
	}

	return selectModulesInteractive(bibles, stdin, stdout)
}

// selectModulesByName finds modules matching the given names.
func selectModulesByName(names []string, bibles []ModuleInfo, stderr io.Writer) []ModuleInfo {
	moduleMap := make(map[string]ModuleInfo)
	for _, m := range bibles {
		moduleMap[m.Name] = m
	}

	var selected []ModuleInfo
	for _, name := range names {
		if m, ok := moduleMap[name]; ok {
			selected = append(selected, m)
		} else {
			fmt.Fprintf(stderr, "Warning: module '%s' not found\n", name)
		}
	}
	return selected
}

// selectModulesInteractive prompts the user to select modules interactively.
func selectModulesInteractive(bibles []ModuleInfo, stdin io.Reader, stdout io.Writer) ([]ModuleInfo, error) {
	displayModuleList(bibles, stdout)

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Enter module numbers to ingest (comma-separated), 'all', or 'q' to quit:")
	fmt.Fprint(stdout, "> ")

	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		return nil, nil
	}

	input := scanner.Text()
	if input == "q" || input == "quit" {
		return nil, nil
	}
	if input == "all" {
		return bibles, nil
	}

	return parseModuleSelection(input, bibles), nil
}

// displayModuleList prints the available modules to stdout.
func displayModuleList(bibles []ModuleInfo, stdout io.Writer) {
	fmt.Fprintln(stdout, "Available Bible modules:")
	for i, m := range bibles {
		encrypted := ""
		if m.Encrypted {
			encrypted = " [encrypted]"
		}
		fmt.Fprintf(stdout, "  %2d. %-15s %s%s\n", i+1, m.Name, m.Description, encrypted)
	}
}

// parseModuleSelection parses user input for module selection.
func parseModuleSelection(input string, bibles []ModuleInfo) []ModuleInfo {
	var selected []ModuleInfo
	for _, s := range splitAndTrim(input, ",") {
		var idx int
		if _, err := fmt.Sscanf(s, "%d", &idx); err == nil {
			if idx >= 1 && idx <= len(bibles) {
				selected = append(selected, bibles[idx-1])
			}
		} else {
			// Try as module name
			for _, m := range bibles {
				if m.Name == s {
					selected = append(selected, m)
					break
				}
			}
		}
	}
	return selected
}

// filterBibleModules returns only Bible modules from the given list.
func filterBibleModules(modules []ModuleInfo) []ModuleInfo {
	var bibles []ModuleInfo
	for _, m := range modules {
		if m.Type == "Bible" {
			bibles = append(bibles, m)
		}
	}
	return bibles
}

// ingestModules processes each module and creates capsules.
func ingestModules(toIngest []ModuleInfo, swordPath, outputDir string, stdout, stderr io.Writer) {
	fmt.Fprintf(stdout, "\nIngesting %d module(s) to %s/\n\n", len(toIngest), outputDir)
	for _, m := range toIngest {
		if m.Encrypted {
			fmt.Fprintf(stdout, "Skipping %s (encrypted modules not supported)\n", m.Name)
			continue
		}
		capsulePath := filepath.Join(outputDir, m.Name+".capsule.tar.xz")
		fmt.Fprintf(stdout, "Creating %s...\n", capsulePath)
		if err := createModuleCapsule(swordPath, m, capsulePath); err != nil {
			fmt.Fprintf(stderr, "  Error: %v\n", err)
			continue
		}
		info, _ := os.Stat(capsulePath)
		if info != nil {
			fmt.Fprintf(stdout, "  Created: %s (%d bytes)\n", capsulePath, info.Size())
		}
	}
	fmt.Fprintln(stdout, "\nDone!")
}

// runIngestCmd is the core logic for the ingest command, testable with custom I/O.
func runIngestCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cfg := parseIngestArgs(args)

	modules, err := ListModules(cfg.swordPath)
	if err != nil {
		return fmt.Errorf("failed to list modules: %w", err)
	}

	bibles := filterBibleModules(modules)
	if len(bibles) == 0 {
		return fmt.Errorf("no Bible modules found in %s", cfg.swordPath)
	}

	toIngest, err := selectModulesToIngest(cfg, bibles, stdin, stdout, stderr)
	if err != nil {
		return err
	}

	if len(toIngest) == 0 {
		fmt.Fprintln(stdout, "No modules selected for ingestion.")
		return nil
	}

	if err := os.MkdirAll(cfg.outputDir, 0700); err != nil {
		return fmt.Errorf("error creating output directory: %w", err)
	}

	ingestModules(toIngest, cfg.swordPath, cfg.outputDir, stdout, stderr)
	return nil
}

// cmdIngest implements the "ingest" command to create capsules from modules.
func cmdIngest() {
	if err := runIngestCmd(os.Args, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// getDefaultSwordPath returns the default SWORD installation path.
func getDefaultSwordPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".sword"
	}
	return filepath.Join(home, ".sword")
}

// splitAndTrim splits a string and trims whitespace from each part.
func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, p := range filepath.SplitList(s) {
		p = trimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	// Also handle comma separation
	result := make([]string, 0)
	for _, p := range parts {
		for _, q := range splitByComma(p) {
			q = trimSpace(q)
			if q != "" {
				result = append(result, q)
			}
		}
	}
	if len(result) == 0 {
		// Fall back to simple comma split
		for _, p := range splitByComma(s) {
			p = trimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
	}
	return result
}

func splitByComma(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// createModuleCapsule creates a capsule from a SWORD module.
// The capsule includes the SWORD module data and extracted IR.
func createModuleCapsule(swordPath string, module ModuleInfo, outputPath string) error {
	confPath, conf, err := findAndParseConf(swordPath, module.Name)
	if err != nil {
		return err
	}

	dataPath, fullDataPath, err := resolveDataPath(swordPath, conf)
	if err != nil {
		return err
	}

	tempDir, err := setupTempDirectory(confPath, dataPath, fullDataPath)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	irDir := filepath.Join(tempDir, "ir")
	extractModuleIR(conf, swordPath, irDir)

	if err := writeManifest(tempDir, module, conf, irDir); err != nil {
		return err
	}

	return createTarGZ(tempDir, outputPath)
}

// findAndParseConf locates the configuration file and parses it.
func findAndParseConf(swordPath, moduleName string) (string, *ConfFile, error) {
	confPath := filepath.Join(swordPath, "mods.d", moduleName+".conf")

	if _, err := os.Stat(confPath); errors.Is(err, os.ErrNotExist) {
		confPath = findConfCaseInsensitive(swordPath, moduleName)
	}

	conf, err := ParseConfFile(confPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse conf: %w", err)
	}

	return confPath, conf, nil
}

// findConfCaseInsensitive searches for a configuration file case-insensitively.
func findConfCaseInsensitive(swordPath, moduleName string) string {
	modsDir := filepath.Join(swordPath, "mods.d")
	entries, _ := os.ReadDir(modsDir)
	for _, e := range entries {
		name := e.Name()
		if len(name) > 5 && name[len(name)-5:] == ".conf" {
			baseName := name[:len(name)-5]
			if strings.EqualFold(baseName, moduleName) {
				return filepath.Join(modsDir, name)
			}
		}
	}
	return filepath.Join(modsDir, moduleName+".conf")
}

// resolveDataPath determines and validates the module data path.
func resolveDataPath(swordPath string, conf *ConfFile) (string, string, error) {
	dataPath := conf.DataPath
	if dataPath == "" {
		return "", "", fmt.Errorf("no DataPath in conf file")
	}

	dataPath = cleanDataPath(dataPath)
	fullDataPath := filepath.Join(swordPath, dataPath)

	if _, err := os.Stat(fullDataPath); errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("data path not found: %s", fullDataPath)
	}

	return dataPath, fullDataPath, nil
}

// cleanDataPath removes "./" prefix from relative paths.
func cleanDataPath(dataPath string) string {
	dataPath = filepath.Clean(dataPath)
	if len(dataPath) > 0 && dataPath[0] == '.' {
		if len(dataPath) > 2 {
			return dataPath[2:]
		}
		return ""
	}
	return dataPath
}

// setupTempDirectory creates and populates the temporary directory structure.
func setupTempDirectory(confPath, dataPath, fullDataPath string) (string, error) {
	tempDir, err := os.MkdirTemp("", "sword-capsule-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	if err := copyConfFile(tempDir, confPath); err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}

	if err := copyModuleData(tempDir, dataPath, fullDataPath); err != nil {
		os.RemoveAll(tempDir)
		return "", err
	}

	return tempDir, nil
}

// copyConfFile copies the configuration file to the temporary directory.
func copyConfFile(tempDir, confPath string) error {
	modsDir := filepath.Join(tempDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		return fmt.Errorf("failed to create mods.d: %w", err)
	}

	confData, err := safefile.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read conf: %w", err)
	}

	destConfPath := filepath.Join(modsDir, filepath.Base(confPath))
	if err := os.WriteFile(destConfPath, confData, 0600); err != nil {
		return fmt.Errorf("failed to write conf: %w", err)
	}

	return nil
}

// copyModuleData copies the module data to the temporary directory.
func copyModuleData(tempDir, dataPath, fullDataPath string) error {
	destDataPath := filepath.Join(tempDir, dataPath)
	if err := os.MkdirAll(filepath.Dir(destDataPath), 0700); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	if err := fileutil.CopyDir(fullDataPath, destDataPath); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	return nil
}

// extractModuleIR extracts IR for supported Bible modules.
func extractModuleIR(conf *ConfFile, swordPath, irDir string) {
	if err := os.MkdirAll(irDir, 0700); err != nil {
		return
	}

	if skipIRExtraction {
		return
	}

	if conf.ModuleType() == "Bible" && conf.IsCompressed() && !conf.IsEncrypted() {
		if err := extractIRToCapsule(conf, swordPath, irDir); err != nil {
			fmt.Printf("  Warning: Could not extract IR: %v\n", err)
		}
	}
}

// writeManifest creates and writes the manifest.json file.
func writeManifest(tempDir string, module ModuleInfo, conf *ConfFile, irDir string) error {
	manifest := map[string]interface{}{
		"capsule_version": "1.0",
		"module_type":     "bible",
		"id":              module.Name,
		"title":           conf.Description,
		"language":        conf.Lang,
		"source_format":   "sword",
		"versification":   conf.Versification,
		"has_ir":          hasFilesInDir(irDir),
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	manifestPath := filepath.Join(tempDir, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0600); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// extractIRToCapsule extracts IR from a SWORD module and writes it to the IR directory.
func extractIRToCapsule(conf *ConfFile, swordPath, irDir string) error {
	// Open the zText module
	zt, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		return fmt.Errorf("failed to open module: %w", err)
	}

	// Extract corpus with full verse text
	corpus, _, err := extractCorpus(zt, conf)
	if err != nil {
		return fmt.Errorf("failed to extract corpus: %w", err)
	}

	// Write IR JSON
	irPath := filepath.Join(irDir, conf.ModuleName+".ir.json")
	if err := writeCorpusJSON(corpus, irPath); err != nil {
		return fmt.Errorf("failed to write IR: %w", err)
	}

	return nil
}

// hasFilesInDir returns true if the directory contains any files.
func hasFilesInDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// resolveTarGZPath normalises dstPath so it always ends in ".tar.gz".
func resolveTarGZPath(dstPath string) string {
	if strings.HasSuffix(dstPath, ".tar.xz") {
		return strings.TrimSuffix(dstPath, ".tar.xz") + ".tar.gz"
	}
	if strings.HasSuffix(dstPath, ".tar.gz") {
		return dstPath
	}
	return dstPath + ".tar.gz"
}

// writeFileContent copies the content of a regular file into the tar writer.
func writeFileContent(tw *tar.Writer, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(tw, file)
	return err
}

// writeTarEntry writes a single filesystem entry (file or directory) into tw.
func writeTarEntry(tw *tar.Writer, srcDir, path string, info os.FileInfo) error {
	relPath, err := filepath.Rel(srcDir, path)
	if err != nil {
		return err
	}
	if relPath == "." {
		return nil
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = relPath
	if info.IsDir() {
		header.Name += "/"
	}
	header.ModTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	return writeFileContent(tw, path)
}

// closeTarGZWriters flushes and closes both writers, surfacing any write errors.
func closeTarGZWriters(tw *tar.Writer, gw *gzip.Writer) error {
	if err := tw.Close(); err != nil {
		return err
	}
	return gw.Close()
}

// createTarGZ creates a tar.gz archive from a directory.
// Files are stored at the root level (no directory prefix).
func createTarGZ(srcDir, dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0700); err != nil {
		return err
	}

	outFile, err := os.Create(resolveTarGZPath(dstPath))
	if err != nil {
		return fmt.Errorf("failed to create archive file: %w", err)
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	if err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return writeTarEntry(tw, srcDir, path, info)
	}); err != nil {
		return err
	}

	return closeTarGZWriters(tw, gw)
}
