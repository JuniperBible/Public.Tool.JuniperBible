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

// runIngestCmd is the core logic for the ingest command, testable with custom I/O.
func runIngestCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	swordPath := getDefaultSwordPath()
	outputDir := "capsules"
	var selectedModules []string
	ingestAll := false

	// Parse arguments
	for i := 2; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--all" || arg == "-a":
			ingestAll = true
		case arg == "--output" || arg == "-o":
			if i+1 < len(args) {
				i++
				outputDir = args[i]
			}
		case arg == "--path" || arg == "-p":
			if i+1 < len(args) {
				i++
				swordPath = args[i]
			}
		default:
			selectedModules = append(selectedModules, arg)
		}
	}

	// Get available modules
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
		return fmt.Errorf("no Bible modules found in %s", swordPath)
	}

	// Determine which modules to ingest
	var toIngest []ModuleInfo
	if ingestAll {
		toIngest = bibles
	} else if len(selectedModules) > 0 {
		// Find specified modules
		moduleMap := make(map[string]ModuleInfo)
		for _, m := range bibles {
			moduleMap[m.Name] = m
		}
		for _, name := range selectedModules {
			if m, ok := moduleMap[name]; ok {
				toIngest = append(toIngest, m)
			} else {
				fmt.Fprintf(stderr, "Warning: module '%s' not found\n", name)
			}
		}
	} else {
		// Interactive selection
		fmt.Fprintln(stdout, "Available Bible modules:")
		for i, m := range bibles {
			encrypted := ""
			if m.Encrypted {
				encrypted = " [encrypted]"
			}
			fmt.Fprintf(stdout, "  %2d. %-15s %s%s\n", i+1, m.Name, m.Description, encrypted)
		}
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Enter module numbers to ingest (comma-separated), 'all', or 'q' to quit:")
		fmt.Fprint(stdout, "> ")

		scanner := bufio.NewScanner(stdin)
		if scanner.Scan() {
			input := scanner.Text()
			if input == "q" || input == "quit" {
				return nil
			}
			if input == "all" {
				toIngest = bibles
			} else {
				// Parse comma-separated numbers
				for _, s := range splitAndTrim(input, ",") {
					var idx int
					if _, err := fmt.Sscanf(s, "%d", &idx); err == nil {
						if idx >= 1 && idx <= len(bibles) {
							toIngest = append(toIngest, bibles[idx-1])
						}
					} else {
						// Try as module name
						for _, m := range bibles {
							if m.Name == s {
								toIngest = append(toIngest, m)
								break
							}
						}
					}
				}
			}
		}
	}

	if len(toIngest) == 0 {
		fmt.Fprintln(stdout, "No modules selected for ingestion.")
		return nil
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("error creating output directory: %w", err)
	}

	// Ingest each module
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
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		return fmt.Errorf("failed to create mods.d: %w", err)
	}

	// Copy conf file
	confData, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("failed to read conf: %w", err)
	}
	destConfPath := filepath.Join(modsDir, filepath.Base(confPath))
	if err := os.WriteFile(destConfPath, confData, 0644); err != nil {
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
	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), manifestData, 0644); err != nil {
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
