//go:build !sdk

// Plugin format-sword handles SWORD Bible module ingestion.
// It detects SWORD module directories by looking for mods.d/*.conf files
// and modules/* data directories.
//
// IR Support:
// - extract-ir: Extracts IR from SWORD module (L1/L2 - requires libsword)
// - emit-native: Converts IR back to SWORD module format (L1/L2)
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
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

func main() {
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorf("failed to decode request: %v", err)
		return
	}

	switch req.Command {
	case "detect":
		handleDetect(req.Args)
	case "ingest":
		handleIngest(req.Args)
	case "enumerate":
		handleEnumerate(req.Args)
	case "extract-ir":
		handleExtractIR(req.Args)
	case "emit-native":
		handleEmitNative(req.Args)
	default:
		ipc.RespondErrorf("unknown command: %s", req.Command)
	}
}

func handleDetect(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if !info.IsDir() {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "path is not a directory",
		})
		return
	}

	// Check for SWORD module structure
	// Look for mods.d/ directory with .conf files
	modsD := filepath.Join(path, "mods.d")
	if _, err := os.Stat(modsD); errors.Is(err, os.ErrNotExist) {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "no mods.d directory found",
		})
		return
	}

	// Check for at least one .conf file
	confFiles, err := filepath.Glob(filepath.Join(modsD, "*.conf"))
	if err != nil || len(confFiles) == 0 {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "no .conf files in mods.d/",
		})
		return
	}

	// Check for modules/ directory
	modulesDir := filepath.Join(path, "modules")
	if _, err := os.Stat(modulesDir); errors.Is(err, os.ErrNotExist) {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "no modules directory found",
		})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   "sword",
		Reason:   fmt.Sprintf("SWORD module detected: %d .conf file(s)", len(confFiles)),
	})
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	// Parse all SWORD modules in the directory
	modules, err := parseSwordModules(path)
	if err != nil {
		ipc.RespondErrorf("failed to parse modules: %v", err)
		return
	}

	if len(modules) == 0 {
		ipc.RespondError("no SWORD modules found")
		return
	}

	// For now, create a manifest of all modules
	manifest := struct {
		RootPath string         `json:"root_path"`
		Modules  []*SwordModule `json:"modules"`
	}{
		RootPath: filepath.Base(path),
		Modules:  modules,
	}

	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	hash := sha256.Sum256(manifestData)
	hashHex := hex.EncodeToString(hash[:])

	// Write manifest blob
	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		ipc.RespondErrorf("failed to create blob dir: %v", err)
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, manifestData, 0600); err != nil {
		ipc.RespondErrorf("failed to write blob: %v", err)
		return
	}

	// Use first module name as artifact ID
	artifactID := "sword-modules"
	if len(modules) > 0 {
		artifactID = modules[0].Name
	}
	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(manifestData)),
		Metadata: map[string]string{
			"format":       "sword",
			"module_count": fmt.Sprintf("%d", len(modules)),
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

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
		ipc.RespondErrorf("failed to enumerate: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: entries,
	})
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
		}
	}

	if module.Name == "" {
		return nil, fmt.Errorf("no module name found")
	}

	return module, nil
}

func handleExtractIR(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	// Parse all SWORD modules
	modules, err := parseSwordModules(path)
	if err != nil {
		ipc.RespondErrorf("failed to parse modules: %v", err)
		return
	}

	if len(modules) == 0 {
		ipc.RespondError("no SWORD modules found")
		return
	}

	// Create IR corpus from module metadata
	// Note: Full text extraction requires libsword via tool-libsword plugin
	module := modules[0]
	corpus := &ipc.Corpus{
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
	corpus.Documents = []*ipc.Document{
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

	// Serialize IR to JSON
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to serialize IR: %v", err)
		return
	}

	// Write IR to output directory
	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0600); err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}
	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L2",
		LossReport: &ipc.LossReport{
			SourceFormat: "SWORD",
			TargetFormat: "IR",
			LossClass:    "L2",
			Warnings: []string{
				"Full text extraction requires tool-libsword plugin",
				"Only module metadata was extracted",
			},
		},
	})
}

func handleEmitNative(args map[string]interface{}) {
	irPath, ok := args["ir_path"].(string)
	if !ok {
		ipc.RespondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	// Read IR file
	data, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR file: %v", err)
		return
	}

	// Parse IR
	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		ipc.RespondErrorf("failed to parse IR: %v", err)
		return
	}

	// Create SWORD module directory structure
	moduleDir := filepath.Join(outputDir, corpus.ID)
	modsD := filepath.Join(moduleDir, "mods.d")
	modulesDir := filepath.Join(moduleDir, "modules")

	if err := os.MkdirAll(modsD, 0755); err != nil {
		ipc.RespondErrorf("failed to create mods.d: %v", err)
		return
	}
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		ipc.RespondErrorf("failed to create modules: %v", err)
		return
	}

	// Check if we have the original conf file
	if confContent, ok := corpus.Attributes["_sword_conf"]; ok && confContent != "" {
		// Write original conf file (L0 for conf)
		confPath := filepath.Join(modsD, strings.ToLower(corpus.ID)+".conf")
		if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
			ipc.RespondErrorf("failed to write conf: %v", err)
			return
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
		if err := os.WriteFile(confPath, []byte(confBuf.String()), 0600); err != nil {
			ipc.RespondErrorf("failed to write conf: %v", err)
			return
		}
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: moduleDir,
		Format:     "SWORD",
		LossClass:  "L2",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "SWORD",
			LossClass:    "L2",
			Warnings: []string{
				"SWORD module data files not generated",
				"Only conf file structure was created",
				"Full module creation requires tool-libsword",
			},
		},
	})
}

// Compile check
var _ = io.Copy
