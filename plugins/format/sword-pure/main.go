//go:build !sdk

// Package main implements a pure Go SWORD module parser plugin.
// This plugin provides SWORD module parsing without CGO dependencies.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "info":
			printInfo()
			return
		case "ipc":
			runIPC()
			return
		case "list":
			cmdList()
			return
		case "ingest":
			cmdIngest()
			return
		case "help":
			printUsage()
			return
		}
	}

	// Default: run IPC mode (read from stdin)
	runIPC()
}

func printUsage() {
	fmt.Print(`format-sword-pure - Pure Go SWORD module parser

Usage:
  format-sword-pure list [path]           List Bible modules (default: ~/.sword)
  format-sword-pure ingest [modules...]   Ingest modules into capsules
  format-sword-pure info                  Print plugin info as JSON
  format-sword-pure ipc                   Run in IPC mode (for plugin system)
  format-sword-pure help                  Print this help

Examples:
  format-sword-pure list                  List all Bibles in ~/.sword
  format-sword-pure list /path/to/sword   List Bibles in custom path
  format-sword-pure ingest                Interactive selection
  format-sword-pure ingest KJV DRC        Ingest specific modules
  format-sword-pure ingest --all          Ingest all Bible modules
`)
}

func printInfo() {
	info := PluginInfo{
		PluginID:    "format.sword-pure",
		Version:     "0.1.0",
		Kind:        "format",
		Description: "Pure Go SWORD module parsing (no CGO dependencies)",
		Formats:     []string{"ztext", "zcom", "zld", "rawgenbook"},
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(info)
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
			ipc.RespondError(fmt.Sprintf("invalid JSON: %v", err))
			continue
		}

		handleRequest(&req)
	}

	if err := scanner.Err(); err != nil {
		ipc.RespondError(fmt.Sprintf("stdin read error: %v", err))
	}
}

func handleRequest(req *ipc.Request) {
	switch req.Command {
	case "list-modules":
		handleListModules(req)
	case "render-verse":
		handleRenderVerse(req)
	case "render-all":
		handleRenderAll(req)
	case "detect":
		handleDetect(req)
	case "parse-conf":
		handleParseConf(req)
	case "extract-ir":
		handleExtractIR(req)
	case "emit-native":
		handleEmitNative(req)
	default:
		ipc.RespondError(fmt.Sprintf("unknown command: %s", req.Command))
	}
}

func handleListModules(req *ipc.Request) {
	path, ok := req.Args["path"].(string)
	if !ok || path == "" {
		ipc.RespondError("missing required argument: path")
		return
	}

	modules, err := ListModules(path)
	if err != nil {
		ipc.RespondError(fmt.Sprintf("failed to list modules: %v", err))
		return
	}

	ipc.MustRespond(map[string]interface{}{
		"modules": modules,
		"count":   len(modules),
	})
}

func handleRenderVerse(req *ipc.Request) {
	path, pathOk := req.Args["path"].(string)
	module, moduleOk := req.Args["module"].(string)
	refStr, refOk := req.Args["ref"].(string)

	// Check for missing arguments
	if path == "" || module == "" || refStr == "" {
		ipc.RespondError("missing required arguments: path, module, ref")
		return
	}

	// Check for wrong type (argument present but not a string)
	if !pathOk || !moduleOk || !refOk {
		ipc.RespondError("invalid argument type: path, module, and ref must be strings")
		return
	}

	text, err := RenderVerse(path, module, refStr)
	if err != nil {
		ipc.RespondError(fmt.Sprintf("failed to render verse: %v", err))
		return
	}

	ipc.MustRespond(map[string]interface{}{
		"ref":  refStr,
		"text": text,
	})
}

func handleRenderAll(req *ipc.Request) {
	path, pathOk := req.Args["path"].(string)
	module, moduleOk := req.Args["module"].(string)

	// Check for missing arguments
	if path == "" || module == "" {
		ipc.RespondError("missing required arguments: path, module")
		return
	}

	// Check for wrong type (argument present but not a string)
	if !pathOk || !moduleOk {
		ipc.RespondError("invalid argument type: path and module must be strings")
		return
	}

	verses, err := RenderAll(path, module)
	if err != nil {
		ipc.RespondError(fmt.Sprintf("failed to render all: %v", err))
		return
	}

	ipc.MustRespond(map[string]interface{}{
		"verses": verses,
		"count":  len(verses),
	})
}

func handleDetect(req *ipc.Request) {
	path, ok := req.Args["path"].(string)
	if !ok || path == "" {
		ipc.RespondError("missing required argument: path")
		return
	}

	detected, format, err := Detect(path)
	if err != nil {
		ipc.RespondError(fmt.Sprintf("detection failed: %v", err))
		return
	}

	ipc.MustRespond(map[string]interface{}{
		"detected": detected,
		"format":   format,
	})
}

func handleParseConf(req *ipc.Request) {
	path, ok := req.Args["path"].(string)
	if !ok || path == "" {
		ipc.RespondError("missing required argument: path")
		return
	}

	conf, err := ParseConfFile(path)
	if err != nil {
		ipc.RespondError(fmt.Sprintf("failed to parse conf: %v", err))
		return
	}

	ipc.MustRespond(conf)
}

// handleExtractIR extracts Intermediate Representation from SWORD modules.
// This provides FULL verse text extraction (L1 loss class), unlike CGO which only extracts metadata (L2).
func handleExtractIR(req *ipc.Request) {
	path, pathOk := req.Args["path"].(string)
	outputDir, outputDirOk := req.Args["output_dir"].(string)
	moduleFilter, _ := req.Args["module"].(string) // optional, ok to be missing

	// Check for missing required arguments
	if path == "" {
		ipc.RespondError("missing required argument: path")
		return
	}
	if outputDir == "" {
		ipc.RespondError("missing required argument: output_dir")
		return
	}

	// Check for wrong type (argument present but not a string)
	if !pathOk || !outputDirOk {
		ipc.RespondError("invalid argument type: path and output_dir must be strings")
		return
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		ipc.RespondError(fmt.Sprintf("failed to create output dir: %v", err))
		return
	}

	// Load modules from path
	confs, err := LoadModulesFromPath(path)
	if err != nil {
		ipc.RespondError(fmt.Sprintf("failed to load modules: %v", err))
		return
	}

	var results []map[string]interface{}
	for _, conf := range confs {
		// Filter to specific module if requested
		if moduleFilter != "" && conf.ModuleName != moduleFilter {
			continue
		}

		// Skip encrypted modules
		if conf.IsEncrypted() {
			results = append(results, map[string]interface{}{
				"module": conf.ModuleName,
				"status": "skipped",
				"reason": "encrypted",
			})
			continue
		}

		// Only handle zText Bible modules for now
		if conf.ModuleType() != "Bible" || !conf.IsCompressed() {
			results = append(results, map[string]interface{}{
				"module": conf.ModuleName,
				"status": "skipped",
				"reason": fmt.Sprintf("unsupported type: %s/%s", conf.ModuleType(), conf.ModDrv),
			})
			continue
		}

		// Open the zText module
		zt, err := OpenZTextModule(conf, path)
		if err != nil {
			results = append(results, map[string]interface{}{
				"module": conf.ModuleName,
				"status": "error",
				"error":  err.Error(),
			})
			continue
		}

		// Extract corpus with full verse text
		corpus, stats, err := extractCorpus(zt, conf)
		if err != nil {
			results = append(results, map[string]interface{}{
				"module": conf.ModuleName,
				"status": "error",
				"error":  err.Error(),
			})
			continue
		}

		// Write IR JSON
		irPath := filepath.Join(outputDir, conf.ModuleName+".ir.json")
		if err := writeCorpusJSON(corpus, irPath); err != nil {
			results = append(results, map[string]interface{}{
				"module": conf.ModuleName,
				"status": "error",
				"error":  fmt.Sprintf("failed to write IR: %v", err),
			})
			continue
		}

		results = append(results, map[string]interface{}{
			"module":      conf.ModuleName,
			"status":      "ok",
			"ir_path":     irPath,
			"documents":   stats.Documents,
			"verses":      stats.Verses,
			"tokens":      stats.Tokens,
			"annotations": stats.Annotations,
			"loss_class":  string(corpus.LossClass),
		})
	}

	ipc.MustRespond(map[string]interface{}{
		"modules": results,
		"count":   len(results),
	})
}

// handleEmitNative converts IR back to native SWORD format.
func handleEmitNative(req *ipc.Request) {
	irPath, irPathOk := req.Args["ir_path"].(string)
	outputDir, outputDirOk := req.Args["output_dir"].(string)

	// Check for missing required arguments
	if irPath == "" {
		ipc.RespondError("missing required argument: ir_path")
		return
	}
	if outputDir == "" {
		ipc.RespondError("missing required argument: output_dir")
		return
	}

	// Check for wrong type (argument present but not a string)
	if !irPathOk || !outputDirOk {
		ipc.RespondError("invalid argument type: ir_path and output_dir must be strings")
		return
	}

	// Load IR corpus
	data, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondError(fmt.Sprintf("failed to read IR file: %v", err))
		return
	}

	var corpus IRCorpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		ipc.RespondError(fmt.Sprintf("failed to parse IR: %v", err))
		return
	}

	// Use EmitZText for full binary generation
	result, err := EmitZText(&corpus, outputDir)
	if err != nil {
		ipc.RespondError(fmt.Sprintf("failed to emit zText: %v", err))
		return
	}

	ipc.MustRespond(map[string]interface{}{
		"status":         "ok",
		"message":        "Generated complete zText module with binary data.",
		"conf_path":      result.ConfPath,
		"data_path":      result.DataPath,
		"module_id":      result.ModuleID,
		"verses_written": result.VersesWritten,
		"output_dir":     outputDir,
	})
}
