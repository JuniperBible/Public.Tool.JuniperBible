// Package main contains capsule creation and CLI command functions.
package main

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

// cmdList implements the "list" command to list Bible modules.
func cmdList() {
	// Determine SWORD path
	swordPath := getDefaultSwordPath()
	if len(os.Args) > 2 {
		swordPath = os.Args[2]
	}

	modules, err := ListModules(swordPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Filter to only Bible modules
	var bibles []ModuleInfo
	for _, m := range modules {
		if m.Type == "Bible" {
			bibles = append(bibles, m)
		}
	}

	if len(bibles) == 0 {
		fmt.Printf("No Bible modules found in %s\n", swordPath)
		return
	}

	fmt.Printf("Bible modules in %s:\n\n", swordPath)
	fmt.Printf("%-15s %-8s %-40s\n", "MODULE", "LANG", "DESCRIPTION")
	fmt.Printf("%-15s %-8s %-40s\n", "------", "----", "-----------")
	for _, m := range bibles {
		desc := m.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		encrypted := ""
		if m.Encrypted {
			encrypted = " [encrypted]"
		}
		fmt.Printf("%-15s %-8s %-40s%s\n", m.Name, m.Language, desc, encrypted)
	}
	fmt.Printf("\nTotal: %d Bible modules\n", len(bibles))
}

// ingestConfig holds configuration for the ingest command.
type ingestConfig struct {
	swordPath       string
	outputDir       string
	selectedModules []string
	ingestAll       bool
}

// cmdIngest implements the "ingest" command to create capsules from modules.
func cmdIngest() {
	config := parseIngestArgs()
	bibles := getAvailableBibles(config.swordPath)
	toIngest := selectModulesToIngest(config, bibles)
	if len(toIngest) == 0 {
		fmt.Println("No modules selected for ingestion.")
		return
	}
	processIngestion(config, toIngest)
}

// parseIngestArgs parses command-line arguments for the ingest command.
func parseIngestArgs() ingestConfig {
	config := ingestConfig{
		swordPath: getDefaultSwordPath(),
		outputDir: "capsules",
	}

	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "--all" || arg == "-a":
			config.ingestAll = true
		case arg == "--output" || arg == "-o":
			if i+1 < len(os.Args) {
				i++
				config.outputDir = os.Args[i]
			}
		case arg == "--path" || arg == "-p":
			if i+1 < len(os.Args) {
				i++
				config.swordPath = os.Args[i]
			}
		default:
			config.selectedModules = append(config.selectedModules, arg)
		}
	}
	return config
}

// getAvailableBibles returns available Bible modules from the SWORD path.
func getAvailableBibles(swordPath string) []ModuleInfo {
	modules, err := ListModules(swordPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var bibles []ModuleInfo
	for _, m := range modules {
		if m.Type == "Bible" {
			bibles = append(bibles, m)
		}
	}

	if len(bibles) == 0 {
		fmt.Fprintf(os.Stderr, "No Bible modules found in %s\n", swordPath)
		os.Exit(1)
	}
	return bibles
}

// selectModulesToIngest determines which modules to ingest based on config.
func selectModulesToIngest(config ingestConfig, bibles []ModuleInfo) []ModuleInfo {
	if config.ingestAll {
		return bibles
	}
	if len(config.selectedModules) > 0 {
		return findSelectedModules(config.selectedModules, bibles)
	}
	return interactiveModuleSelection(bibles)
}

// findSelectedModules finds modules by name from the selected list.
func findSelectedModules(selectedNames []string, bibles []ModuleInfo) []ModuleInfo {
	moduleMap := make(map[string]ModuleInfo)
	for _, m := range bibles {
		moduleMap[m.Name] = m
	}

	var toIngest []ModuleInfo
	for _, name := range selectedNames {
		if m, ok := moduleMap[name]; ok {
			toIngest = append(toIngest, m)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: module '%s' not found\n", name)
		}
	}
	return toIngest
}

// interactiveModuleSelection prompts the user to select modules interactively.
func interactiveModuleSelection(bibles []ModuleInfo) []ModuleInfo {
	fmt.Println("Available Bible modules:")
	for i, m := range bibles {
		encrypted := ""
		if m.Encrypted {
			encrypted = " [encrypted]"
		}
		fmt.Printf("  %2d. %-15s %s%s\n", i+1, m.Name, m.Description, encrypted)
	}
	fmt.Println()
	fmt.Println("Enter module numbers to ingest (comma-separated), 'all', or 'q' to quit:")
	fmt.Print("> ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return nil
	}

	input := scanner.Text()
	if input == "q" || input == "quit" {
		return nil
	}
	if input == "all" {
		return bibles
	}
	return parseUserSelection(input, bibles)
}

// parseUserSelection parses user input to select modules.
func parseUserSelection(input string, bibles []ModuleInfo) []ModuleInfo {
	var toIngest []ModuleInfo
	for _, s := range splitAndTrim(input, ",") {
		var idx int
		if _, err := fmt.Sscanf(s, "%d", &idx); err == nil {
			if idx >= 1 && idx <= len(bibles) {
				toIngest = append(toIngest, bibles[idx-1])
			}
		} else {
			if m := findModuleInfoByName(s, bibles); m != nil {
				toIngest = append(toIngest, *m)
			}
		}
	}
	return toIngest
}

// findModuleInfoByName finds a module by name.
func findModuleInfoByName(name string, modules []ModuleInfo) *ModuleInfo {
	for _, m := range modules {
		if m.Name == name {
			return &m
		}
	}
	return nil
}

// processIngestion creates capsules for the selected modules.
func processIngestion(config ingestConfig, toIngest []ModuleInfo) {
	if err := os.MkdirAll(config.outputDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nIngesting %d module(s) to %s/\n\n", len(toIngest), config.outputDir)
	for _, m := range toIngest {
		if m.Encrypted {
			fmt.Printf("Skipping %s (encrypted modules not supported)\n", m.Name)
			continue
		}
		ingestModule(config.swordPath, config.outputDir, m)
	}
	fmt.Println("\nDone!")
}

// ingestModule creates a capsule for a single module.
func ingestModule(swordPath, outputDir string, m ModuleInfo) {
	capsulePath := filepath.Join(outputDir, m.Name+".capsule.tar.xz")
	fmt.Printf("Creating %s...\n", capsulePath)
	if err := createModuleCapsule(swordPath, m, capsulePath); err != nil {
		fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		return
	}
	info, _ := os.Stat(capsulePath)
	if info != nil {
		fmt.Printf("  Created: %s (%d bytes)\n", capsulePath, info.Size())
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
	// Find the conf file
	confPath := filepath.Join(swordPath, "mods.d", module.Name+".conf")

	// Try lowercase if not found
	if _, err := os.Stat(confPath); errors.Is(err, os.ErrNotExist) {
		entries, _ := os.ReadDir(filepath.Join(swordPath, "mods.d"))
		for _, e := range entries {
			name := e.Name()
			if len(name) > 5 && name[len(name)-5:] == ".conf" {
				baseName := name[:len(name)-5]
				if strings.EqualFold(baseName, module.Name) {
					confPath = filepath.Join(swordPath, "mods.d", name)
					break
				}
			}
		}
	}

	conf, err := ParseConfFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to parse conf: %w", err)
	}

	// Determine the data path
	dataPath := conf.DataPath
	if dataPath == "" {
		return fmt.Errorf("no DataPath in conf file")
	}

	// DataPath is relative to SWORD root, clean it
	dataPath = filepath.Clean(dataPath)
	if len(dataPath) > 0 && dataPath[0] == '.' {
		if len(dataPath) > 2 {
			dataPath = dataPath[2:] // Remove "./" prefix
		} else {
			dataPath = ""
		}
	}

	fullDataPath := filepath.Join(swordPath, dataPath)
	if _, err := os.Stat(fullDataPath); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("data path not found: %s", fullDataPath)
	}

	// Create a temp directory for capsule contents (files at root level)
	tempDir, err := os.MkdirTemp("", "sword-capsule-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Create SWORD structure at root level
	modsDir := filepath.Join(tempDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		return fmt.Errorf("failed to create mods.d: %w", err)
	}

	// Copy conf file
	confData, err := safefile.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read conf: %w", err)
	}
	destConfPath := filepath.Join(modsDir, filepath.Base(confPath))
	if err := os.WriteFile(destConfPath, confData, 0600); err != nil {
		return fmt.Errorf("failed to write conf: %w", err)
	}

	// Copy module data
	destDataPath := filepath.Join(tempDir, dataPath)
	if err := os.MkdirAll(filepath.Dir(destDataPath), 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}
	if err := fileutil.CopyDir(fullDataPath, destDataPath); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	// Extract IR from the module (if it's a supported type)
	irDir := filepath.Join(tempDir, "ir")
	if err := os.MkdirAll(irDir, 0755); err != nil {
		return fmt.Errorf("failed to create ir dir: %w", err)
	}

	// Try to extract IR for Bible modules (skip if testing to speed up)
	if !skipIRExtraction && conf.ModuleType() == "Bible" && conf.IsCompressed() && !conf.IsEncrypted() {
		if err := extractIRToCapsule(conf, swordPath, irDir); err != nil {
			// Log but don't fail - IR is optional
			fmt.Printf("  Warning: Could not extract IR: %v\n", err)
		}
	}

	// Create manifest.json at root level
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
	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), manifestData, 0600); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Create tar.gz archive (files at root level)
	if err := createTarGZ(tempDir, outputPath); err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
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

// createTarGZ creates a tar.gz archive from a directory.
// Files are stored at the root level (no directory prefix).
func createTarGZ(srcDir, dstPath string) error {
	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	// Ensure .tar.gz extension
	gzPath := dstPath
	if strings.HasSuffix(dstPath, ".tar.xz") {
		gzPath = strings.TrimSuffix(dstPath, ".tar.xz") + ".tar.gz"
	} else if !strings.HasSuffix(dstPath, ".tar.gz") {
		gzPath = dstPath + ".tar.gz"
	}

	// Create the output file
	outFile, err := os.Create(gzPath)
	if err != nil {
		return fmt.Errorf("failed to create archive file: %w", err)
	}
	defer outFile.Close()

	// Create gzip writer
	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	// Create tar writer
	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path (files at root level, no prefix)
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Create header from file info
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Use relative path directly (no prefix)
		header.Name = relPath
		if info.IsDir() {
			header.Name += "/"
		}

		// Normalize timestamps for reproducibility
		header.ModTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// Write file content
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Close writers explicitly to check for errors
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gw.Close(); err != nil {
		return err
	}

	return nil
}
