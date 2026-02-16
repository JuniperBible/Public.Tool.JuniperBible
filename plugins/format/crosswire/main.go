//go:build !sdk

// Package main implements a CrossWire native format plugin.
// This handles SWORD module distributions in their native .zip format
// as distributed by CrossWire repositories (containing mods.d/ and modules/).
package main

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// PluginInfo contains plugin metadata.
type PluginInfo struct {
	PluginID    string   `json:"plugin_id"`
	Version     string   `json:"version"`
	Kind        string   `json:"kind"`
	Description string   `json:"description"`
	Formats     []string `json:"formats"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "info" {
		printInfo()
		return
	}

	runIPC()
}

func printInfo() {
	info := PluginInfo{
		PluginID:    "format.crosswire",
		Version:     "0.1.0",
		Kind:        "format",
		Description: "CrossWire native SWORD module distribution format (.zip archives with mods.d/ and modules/)",
		Formats:     []string{"crosswire-zip", "sword-distribution"},
	}

	ipc.Respond(info)
}

func runIPC() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req ipc.Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			ipc.RespondErrorf("invalid JSON: %v", err)
			continue
		}

		handleRequest(&req)
	}

	if err := scanner.Err(); err != nil {
		ipc.RespondErrorf("stdin read error: %v", err)
	}
}

func handleRequest(req *ipc.Request) {
	switch req.Command {
	case "detect":
		handleDetect(req)
	case "enumerate":
		handleEnumerate(req)
	case "extract-ir":
		handleExtractIR(req)
	case "emit-native":
		handleEmitNative(req)
	case "ingest":
		handleIngest(req)
	default:
		ipc.RespondErrorf("unknown command: %s", req.Command)
	}
}

func handleDetect(req *ipc.Request) {
	path, err := ipc.StringArg(req.Args, "path")
	if err != nil {
		ipc.RespondErrorf("missing path argument: %v", err)
		return
	}

	result := detectCrossWire(path)
	ipc.Respond(result)
}

// detectCrossWire checks if the path is a CrossWire SWORD module distribution.
func detectCrossWire(path string) ipc.DetectResult {
	// Check if it's a zip file
	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		return detectZipArchive(path)
	}

	// Check if it's a directory with mods.d/
	return detectDirectory(path)
}

func detectZipArchive(path string) ipc.DetectResult {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("failed to open zip: %v", err),
		}
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
		return ipc.DetectResult{
			Detected: true,
			Format:   "crosswire-zip",
		}
	}

	return ipc.DetectResult{
		Detected: false,
		Reason:   "zip does not contain CrossWire SWORD module structure (missing mods.d/*.conf or modules/)",
	}
}

func detectDirectory(path string) ipc.DetectResult {
	modsDir := filepath.Join(path, "mods.d")
	modulesDir := filepath.Join(path, "modules")

	modsStat, modsErr := os.Stat(modsDir)
	modulesStat, modulesErr := os.Stat(modulesDir)

	if modsErr != nil || modulesErr != nil {
		return ipc.DetectResult{
			Detected: false,
			Reason:   "directory does not contain mods.d/ and modules/",
		}
	}

	if !modsStat.IsDir() || !modulesStat.IsDir() {
		return ipc.DetectResult{
			Detected: false,
			Reason:   "mods.d or modules is not a directory",
		}
	}

	// Check for at least one .conf file
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("failed to read mods.d: %v", err),
		}
	}

	hasConf := false
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".conf") {
			hasConf = true
			break
		}
	}

	if !hasConf {
		return ipc.DetectResult{
			Detected: false,
			Reason:   "no .conf files in mods.d/",
		}
	}

	return ipc.DetectResult{
		Detected: true,
		Format:   "crosswire-directory",
	}
}

func handleEnumerate(req *ipc.Request) {
	path, err := ipc.StringArg(req.Args, "path")
	if err != nil {
		ipc.RespondErrorf("missing path argument: %v", err)
		return
	}

	// Extract to temp directory if it's a zip
	workDir := path
	var cleanup func()

	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		tmpDir, err := os.MkdirTemp("", "crosswire-*")
		if err != nil {
			ipc.RespondErrorf("failed to create temp dir: %v", err)
			return
		}
		cleanup = func() { os.RemoveAll(tmpDir) }
		defer cleanup()

		if err := extractZip(path, tmpDir); err != nil {
			ipc.RespondErrorf("failed to extract zip: %v", err)
			return
		}

		workDir = tmpDir
	}

	// Find all .conf files in mods.d
	modsDir := filepath.Join(workDir, "mods.d")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		ipc.RespondErrorf("failed to read mods.d: %v", err)
		return
	}

	var modules []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".conf") {
			// Module name is the conf filename without .conf
			moduleName := strings.TrimSuffix(entry.Name(), ".conf")
			modules = append(modules, moduleName)
		}
	}

	result := map[string]interface{}{
		"modules": modules,
		"count":   len(modules),
	}

	ipc.Respond(result)
}

func handleExtractIR(req *ipc.Request) {
	_, err := ipc.StringArg(req.Args, "path")
	if err != nil {
		ipc.RespondErrorf("missing path argument: %v", err)
		return
	}

	module := ipc.StringArgOr(req.Args, "module", "")

	// Create a minimal IR corpus
	// NOTE: For full IR extraction, this should delegate to format-sword-pure plugin
	// or implement full SWORD parsing. This is a basic stub.
	corpus := &ir.Corpus{
		ID:         module,
		Version:    "1.0.0",
		ModuleType: ir.ModuleBible,
		Language:   "en",
		Title:      module,
		Documents:  []*ir.Document{},
	}

	// Serialize IR to JSON
	data, err := json.Marshal(corpus)
	if err != nil {
		ipc.RespondErrorf("failed to marshal IR: %v", err)
		return
	}

	result := map[string]interface{}{
		"ir":     json.RawMessage(data),
		"format": "ir-json",
		"note":   "Basic IR extraction - for full parsing use format-sword-pure plugin",
	}

	ipc.Respond(result)
}

func handleEmitNative(req *ipc.Request) {
	irData := req.Args["ir"]
	if irData == nil {
		ipc.RespondErrorf("missing ir argument")
		return
	}

	outputDir, err := ipc.StringArg(req.Args, "output_dir")
	if err != nil {
		ipc.RespondErrorf("missing output_dir argument: %v", err)
		return
	}

	// Parse IR
	var corpus ir.Corpus
	irBytes, err := json.Marshal(irData)
	if err != nil {
		ipc.RespondErrorf("failed to marshal IR: %v", err)
		return
	}

	if err := json.Unmarshal(irBytes, &corpus); err != nil {
		ipc.RespondErrorf("failed to parse IR: %v", err)
		return
	}

	// Create basic SWORD structure
	// NOTE: For full SWORD emission, this should delegate to format-sword-pure plugin
	// This is a minimal stub implementation
	modsDir := filepath.Join(outputDir, "mods.d")
	modulesDir := filepath.Join(outputDir, "modules")

	if err := os.MkdirAll(modsDir, 0755); err != nil {
		ipc.RespondErrorf("failed to create mods.d: %v", err)
		return
	}

	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		ipc.RespondErrorf("failed to create modules: %v", err)
		return
	}

	// Create a minimal conf file
	confPath := filepath.Join(modsDir, corpus.ID+".conf")
	confContent := fmt.Sprintf("[%s]\nDescription=Exported from IR\nModDrv=RawText\n", corpus.ID)
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		ipc.RespondErrorf("failed to write conf: %v", err)
		return
	}

	// Create a zip archive
	zipPath := filepath.Join(filepath.Dir(outputDir), corpus.ID+".zip")
	if err := createZipArchive(outputDir, zipPath); err != nil {
		ipc.RespondErrorf("failed to create zip: %v", err)
		return
	}

	result := map[string]interface{}{
		"output_path": zipPath,
		"format":      "crosswire-zip",
		"note":        "Basic SWORD structure - for full emission use format-sword-pure plugin",
	}

	ipc.Respond(result)
}

func handleIngest(req *ipc.Request) {
	path, outputDir, err := ipc.PathAndOutputDir(req.Args)
	if err != nil {
		ipc.RespondErrorf("%v", err)
		return
	}

	// Extract to temp directory if it's a zip
	workDir := path
	var cleanup func()

	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		tmpDir, err := os.MkdirTemp("", "crosswire-*")
		if err != nil {
			ipc.RespondErrorf("failed to create temp dir: %v", err)
			return
		}
		cleanup = func() { os.RemoveAll(tmpDir) }
		defer cleanup()

		if err := extractZip(path, tmpDir); err != nil {
			ipc.RespondErrorf("failed to extract zip: %v", err)
			return
		}

		workDir = tmpDir
	}

	// Copy the entire SWORD module structure to output
	if err := copyDir(workDir, outputDir); err != nil {
		ipc.RespondErrorf("failed to copy module: %v", err)
		return
	}

	result := map[string]interface{}{
		"output_path": outputDir,
		"status":      "ingested",
	}

	ipc.Respond(result)
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
