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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
)

// IPCRequest is the incoming JSON request.
type IPCRequest struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args,omitempty"`
}

// IPCResponse is the outgoing JSON response.
type IPCResponse struct {
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// DetectResult is the result of a detect command.
type DetectResult struct {
	Detected bool   `json:"detected"`
	Format   string `json:"format,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// IngestResult is the result of an ingest command.
type IngestResult struct {
	ArtifactID string            `json:"artifact_id"`
	BlobSHA256 string            `json:"blob_sha256"`
	SizeBytes  int64             `json:"size_bytes"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// EnumerateResult is the result of an enumerate command.
type EnumerateResult struct {
	Entries []EnumerateEntry `json:"entries"`
}

// EnumerateEntry represents a file entry.
type EnumerateEntry struct {
	Path      string            `json:"path"`
	SizeBytes int64             `json:"size_bytes"`
	IsDir     bool              `json:"is_dir"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ExtractIRResult is the result of an extract-ir command.
type ExtractIRResult struct {
	IRPath     string      `json:"ir_path"`
	LossClass  string      `json:"loss_class"`
	LossReport *LossReport `json:"loss_report,omitempty"`
}

// EmitNativeResult is the result of an emit-native command.
type EmitNativeResult struct {
	OutputPath string      `json:"output_path"`
	Format     string      `json:"format"`
	LossClass  string      `json:"loss_class"`
	LossReport *LossReport `json:"loss_report,omitempty"`
}

// LossReport describes any data loss during conversion.
type LossReport struct {
	SourceFormat string        `json:"source_format"`
	TargetFormat string        `json:"target_format"`
	LossClass    string        `json:"loss_class"`
	LostElements []LostElement `json:"lost_elements,omitempty"`
	Warnings     []string      `json:"warnings,omitempty"`
}

// LostElement describes a specific element that was lost.
type LostElement struct {
	Path          string      `json:"path"`
	ElementType   string      `json:"element_type"`
	Reason        string      `json:"reason"`
	OriginalValue interface{} `json:"original_value,omitempty"`
}

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

// IR Types (matching core/ir package)
type Corpus struct {
	ID            string            `json:"id"`
	Version       string            `json:"version"`
	ModuleType    string            `json:"module_type"`
	Versification string            `json:"versification,omitempty"`
	Language      string            `json:"language,omitempty"`
	Title         string            `json:"title,omitempty"`
	Description   string            `json:"description,omitempty"`
	Publisher     string            `json:"publisher,omitempty"`
	Rights        string            `json:"rights,omitempty"`
	SourceFormat  string            `json:"source_format,omitempty"`
	Documents     []*Document       `json:"documents,omitempty"`
	SourceHash    string            `json:"source_hash,omitempty"`
	LossClass     string            `json:"loss_class,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type Document struct {
	ID            string            `json:"id"`
	Title         string            `json:"title,omitempty"`
	Order         int               `json:"order"`
	ContentBlocks []*ContentBlock   `json:"content_blocks,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type ContentBlock struct {
	ID         string                 `json:"id"`
	Sequence   int                    `json:"sequence"`
	Text       string                 `json:"text"`
	Tokens     []*Token               `json:"tokens,omitempty"`
	Anchors    []*Anchor              `json:"anchors,omitempty"`
	Hash       string                 `json:"hash,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type Token struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Text     string `json:"text"`
	StartPos int    `json:"start_pos"`
	EndPos   int    `json:"end_pos"`
}

type Anchor struct {
	ID       string  `json:"id"`
	Position int     `json:"position"`
	Spans    []*Span `json:"spans,omitempty"`
}

type Span struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	StartAnchorID string                 `json:"start_anchor_id"`
	EndAnchorID   string                 `json:"end_anchor_id,omitempty"`
	Ref           *Ref                   `json:"ref,omitempty"`
	Attributes    map[string]interface{} `json:"attributes,omitempty"`
}

type Ref struct {
	Book     string `json:"book"`
	Chapter  int    `json:"chapter,omitempty"`
	Verse    int    `json:"verse,omitempty"`
	VerseEnd int    `json:"verse_end,omitempty"`
	SubVerse string `json:"sub_verse,omitempty"`
	OSISID   string `json:"osis_id,omitempty"`
}

func main() {
	var req IPCRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		respondError(fmt.Sprintf("failed to decode request: %v", err))
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
		respondError(fmt.Sprintf("unknown command: %s", req.Command))
	}
}

func handleDetect(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		respond(&DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if !info.IsDir() {
		respond(&DetectResult{
			Detected: false,
			Reason:   "path is not a directory",
		})
		return
	}

	// Check for SWORD module structure
	// Look for mods.d/ directory with .conf files
	modsD := filepath.Join(path, "mods.d")
	if _, err := os.Stat(modsD); os.IsNotExist(err) {
		respond(&DetectResult{
			Detected: false,
			Reason:   "no mods.d directory found",
		})
		return
	}

	// Check for at least one .conf file
	confFiles, err := filepath.Glob(filepath.Join(modsD, "*.conf"))
	if err != nil || len(confFiles) == 0 {
		respond(&DetectResult{
			Detected: false,
			Reason:   "no .conf files in mods.d/",
		})
		return
	}

	// Check for modules/ directory
	modulesDir := filepath.Join(path, "modules")
	if _, err := os.Stat(modulesDir); os.IsNotExist(err) {
		respond(&DetectResult{
			Detected: false,
			Reason:   "no modules directory found",
		})
		return
	}

	respond(&DetectResult{
		Detected: true,
		Format:   "sword",
		Reason:   fmt.Sprintf("SWORD module detected: %d .conf file(s)", len(confFiles)),
	})
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		respondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	// Parse all SWORD modules in the directory
	modules, err := parseSwordModules(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to parse modules: %v", err))
		return
	}

	if len(modules) == 0 {
		respondError("no SWORD modules found")
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
		respondError(fmt.Sprintf("failed to create blob dir: %v", err))
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, manifestData, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write blob: %v", err))
		return
	}

	// Use first module name as artifact ID
	artifactID := "sword-modules"
	if len(modules) > 0 {
		artifactID = modules[0].Name
	}

	respond(&IngestResult{
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
		respondError("path argument required")
		return
	}

	var entries []EnumerateEntry

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

		entry := EnumerateEntry{
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
		respondError(fmt.Sprintf("failed to enumerate: %v", err))
		return
	}

	respond(&EnumerateResult{
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
		respondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	// Parse all SWORD modules
	modules, err := parseSwordModules(path)
	if err != nil {
		respondError(fmt.Sprintf("failed to parse modules: %v", err))
		return
	}

	if len(modules) == 0 {
		respondError("no SWORD modules found")
		return
	}

	// Create IR corpus from module metadata
	// Note: Full text extraction requires libsword via tool-libsword plugin
	module := modules[0]
	corpus := &Corpus{
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
	corpus.Documents = []*Document{
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
		respondError(fmt.Sprintf("failed to serialize IR: %v", err))
		return
	}

	// Write IR to output directory
	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		respondError(fmt.Sprintf("failed to write IR: %v", err))
		return
	}

	respond(&ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L2",
		LossReport: &LossReport{
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
		respondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		respondError("output_dir argument required")
		return
	}

	// Read IR file
	data, err := safefile.ReadFile(irPath)
	if err != nil {
		respondError(fmt.Sprintf("failed to read IR file: %v", err))
		return
	}

	// Parse IR
	var corpus Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		respondError(fmt.Sprintf("failed to parse IR: %v", err))
		return
	}

	// Create SWORD module directory structure
	moduleDir := filepath.Join(outputDir, corpus.ID)
	modsD := filepath.Join(moduleDir, "mods.d")
	modulesDir := filepath.Join(moduleDir, "modules")

	if err := os.MkdirAll(modsD, 0755); err != nil {
		respondError(fmt.Sprintf("failed to create mods.d: %v", err))
		return
	}
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		respondError(fmt.Sprintf("failed to create modules: %v", err))
		return
	}

	// Check if we have the original conf file
	if confContent, ok := corpus.Attributes["_sword_conf"]; ok && confContent != "" {
		// Write original conf file (L0 for conf)
		confPath := filepath.Join(modsD, strings.ToLower(corpus.ID)+".conf")
		if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
			respondError(fmt.Sprintf("failed to write conf: %v", err))
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
		if err := os.WriteFile(confPath, []byte(confBuf.String()), 0644); err != nil {
			respondError(fmt.Sprintf("failed to write conf: %v", err))
			return
		}
	}

	respond(&EmitNativeResult{
		OutputPath: moduleDir,
		Format:     "SWORD",
		LossClass:  "L2",
		LossReport: &LossReport{
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

func respond(result interface{}) {
	resp := IPCResponse{
		Status: "ok",
		Result: result,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
}

func respondError(msg string) {
	resp := IPCResponse{
		Status: "error",
		Error:  msg,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
	os.Exit(1)
}

// Compile check
var _ = io.Copy
