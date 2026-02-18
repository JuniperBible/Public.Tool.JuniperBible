// Package main provides a standalone CLI for format operations.
// This binary wraps the core/plugins package for standalone format detection and conversion.
//
// Usage:
//
//	capsule-format detect <file>
//	capsule-format convert <file> --to <format> --out <output>
//	capsule-format ir-extract <file> --format <format> --out <output>
//	capsule-format ir-emit <ir-file> --format <format> --out <output>
//	capsule-format ir-generate <capsule>
//	capsule-format ir-info <ir-file> [--json]
//
// Prefer using `capsule format` instead of this standalone binary.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"

	// Import embedded plugins registry to register all embedded plugins
	_ "github.com/FocuswithJustin/JuniperBible/internal/embedded"
)

var commands = map[string]func([]string){
	"detect":      runDetect,
	"convert":     runConvert,
	"ir-extract":  runIRExtract,
	"ir-emit":     runIREmit,
	"ir-generate": runIRGenerate,
	"ir-info":     runIRInfo,
	"help":        func(_ []string) { printUsage() },
	"-h":          func(_ []string) { printUsage() },
	"--help":      func(_ []string) { printUsage() },
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	if handler, ok := commands[os.Args[1]]; ok {
		handler(os.Args[2:])
		return
	}
	fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
	printUsage()
	os.Exit(1)
}

func runDetect(args []string) {
	fs := flag.NewFlagSet("detect", flag.ExitOnError)
	pluginDir := fs.String("plugin-dir", "", "Path to plugin directory (default: embedded plugins)")
	fs.Parse(args)

	path := validateDetectArgs(fs)
	formatPlugins := loadFormatPlugins(*pluginDir)

	fmt.Printf("Detecting format of: %s\n\n", path)
	for _, p := range formatPlugins {
		printDetectResult(p, path)
	}
}

func validateDetectArgs(fs *flag.FlagSet) string {
	if len(fs.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "Error: file path required\n")
		fs.Usage()
		os.Exit(1)
	}
	path, err := filepath.Abs(fs.Args()[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid path: %v\n", err)
		os.Exit(1)
	}
	return path
}

func loadFormatPlugins(pluginDir string) []*plugins.Plugin {
	loader := plugins.NewLoader()
	if pluginDir != "" {
		if err := loader.LoadFromDir(pluginDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to load plugins: %v\n", err)
			os.Exit(1)
		}
	}
	formatPlugins := loader.GetPluginsByKind("format")
	if len(formatPlugins) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no format plugins found\n")
		os.Exit(1)
	}
	return formatPlugins
}

func printDetectResult(p *plugins.Plugin, path string) {
	req := plugins.NewDetectRequest(path)
	resp, err := plugins.ExecutePlugin(p, req)
	if err != nil {
		fmt.Printf("  %s: error (%v)\n", p.Manifest.PluginID, err)
		return
	}
	result, err := plugins.ParseDetectResult(resp)
	if err != nil {
		fmt.Printf("  %s: parse error (%v)\n", p.Manifest.PluginID, err)
		return
	}
	if result.Detected {
		fmt.Printf("  [MATCH] %s: %s\n", p.Manifest.PluginID, result.Reason)
	} else {
		fmt.Printf("  [no]    %s: %s\n", p.Manifest.PluginID, result.Reason)
	}
}

func runConvert(args []string) {
	fs := flag.NewFlagSet("convert", flag.ExitOnError)
	to := fs.String("to", "", "Target format (required)")
	out := fs.String("out", "", "Output path (required)")
	pluginDir := fs.String("plugin-dir", "", "Path to plugin directory (default: embedded plugins)")
	fs.Parse(args)

	validateConvertArgs(fs, *to, *out)

	inputPath, _ := filepath.Abs(fs.Args()[0])
	outputPath, _ := filepath.Abs(*out)

	loader := plugins.NewLoader()
	if *pluginDir != "" {
		if err := loader.LoadFromDir(*pluginDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to load plugins: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Converting: %s\n", inputPath)
	fmt.Printf("  To format: %s\n", *to)
	fmt.Printf("  Output: %s\n", outputPath)
	fmt.Println()

	sourcePlugin := detectSourcePlugin(loader, inputPath)
	fmt.Printf("Detected source format: %s\n", sourcePlugin.Manifest.PluginID)

	extractAndEmit(loader, sourcePlugin, inputPath, outputPath, *to)
}

// validateConvertArgs checks that all required convert arguments are present
// and exits with an error message if any are missing.
func validateConvertArgs(fs *flag.FlagSet, to, out string) {
	if len(fs.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "Error: input file path required\n")
		fs.Usage()
		os.Exit(1)
	}
	if to == "" {
		fmt.Fprintf(os.Stderr, "Error: --to flag required\n")
		fs.Usage()
		os.Exit(1)
	}
	if out == "" {
		fmt.Fprintf(os.Stderr, "Error: --out flag required\n")
		fs.Usage()
		os.Exit(1)
	}
}

// detectSourcePlugin iterates all format plugins and returns the first one
// that successfully detects the given input file. Exits if none match.
func detectSourcePlugin(loader *plugins.Loader, inputPath string) *plugins.Plugin {
	for _, p := range loader.GetPluginsByKind("format") {
		req := plugins.NewDetectRequest(inputPath)
		resp, err := plugins.ExecutePlugin(p, req)
		if err != nil {
			continue
		}
		result, err := plugins.ParseDetectResult(resp)
		if err != nil || !result.Detected {
			continue
		}
		return p
	}
	fmt.Fprintf(os.Stderr, "Error: could not detect source format\n")
	os.Exit(1)
	return nil // unreachable; satisfies compiler
}

// extractAndEmit performs the IR extraction from the source plugin and emits
// the native format via the target plugin, moving the result to outputPath.
func extractAndEmit(loader *plugins.Loader, sourcePlugin *plugins.Plugin, inputPath, outputPath, toFormat string) {
	tempDir, err := os.MkdirTemp("", "capsule-convert-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	irPath := extractIRPhase(sourcePlugin, inputPath, tempDir)
	emitNativePhase(loader, toFormat, irPath, outputPath)
}

func extractIRPhase(sourcePlugin *plugins.Plugin, inputPath, tempDir string) string {
	extractResp, err := plugins.ExecutePlugin(sourcePlugin, plugins.NewExtractIRRequest(inputPath, tempDir))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to extract IR: %v\n", err)
		os.Exit(1)
	}
	extractResult, err := plugins.ParseExtractIRResult(extractResp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse extract-ir result: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Extracted IR to: %s\n", extractResult.IRPath)
	return extractResult.IRPath
}

func emitNativePhase(loader *plugins.Loader, toFormat, irPath, outputPath string) {
	targetPlugin, err := loader.GetPlugin("format." + toFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: target format plugin not found: %v\n", err)
		os.Exit(1)
	}

	emitResp, err := plugins.ExecutePlugin(targetPlugin, plugins.NewEmitNativeRequest(irPath, filepath.Dir(outputPath)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to emit native format: %v\n", err)
		os.Exit(1)
	}
	emitResult, err := plugins.ParseEmitNativeResult(emitResp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse emit-native result: %v\n", err)
		os.Exit(1)
	}

	finalizeOutput(emitResult, outputPath)
}

func finalizeOutput(emitResult *plugins.EmitNativeResult, outputPath string) {
	if emitResult.OutputPath != outputPath {
		if err := os.Rename(emitResult.OutputPath, outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to move output: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("Conversion complete: %s\n", outputPath)
	if emitResult.LossClass != "" {
		fmt.Printf("Loss class: %s\n", emitResult.LossClass)
	}
}

func validateIRExtractArgs(fs *flag.FlagSet, format, out string) {
	if len(fs.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "Error: input file path required\n")
		fs.Usage()
		os.Exit(1)
	}
	if format == "" {
		fmt.Fprintf(os.Stderr, "Error: --format flag required\n")
		fs.Usage()
		os.Exit(1)
	}
	if out == "" {
		fmt.Fprintf(os.Stderr, "Error: --out flag required\n")
		fs.Usage()
		os.Exit(1)
	}
}

func extractIRToPath(plugin *plugins.Plugin, inputPath, outputPath string) {
	resp, err := plugins.ExecutePlugin(plugin, plugins.NewExtractIRRequest(inputPath, filepath.Dir(outputPath)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to extract IR: %v\n", err)
		os.Exit(1)
	}
	result, err := plugins.ParseExtractIRResult(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse result: %v\n", err)
		os.Exit(1)
	}
	if result.IRPath != outputPath {
		if err := os.Rename(result.IRPath, outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to move output: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("IR extracted successfully: %s\n", outputPath)
	if result.LossClass != "" {
		fmt.Printf("Loss class: %s\n", result.LossClass)
	}
}

func runIRExtract(args []string) {
	fs := flag.NewFlagSet("ir-extract", flag.ExitOnError)
	format := fs.String("format", "", "Source format (required)")
	out := fs.String("out", "", "Output IR JSON path (required)")
	pluginDir := fs.String("plugin-dir", "", "Path to plugin directory (default: embedded plugins)")
	fs.Parse(args)

	validateIRExtractArgs(fs, *format, *out)

	inputPath, _ := filepath.Abs(fs.Args()[0])
	outputPath, _ := filepath.Abs(*out)

	fmt.Printf("Extracting IR from: %s\n", inputPath)
	fmt.Printf("  Format: %s\n", *format)
	fmt.Printf("  Output: %s\n", outputPath)
	fmt.Println()

	extractIRToPath(loadPlugin(plugins.NewLoader(), *pluginDir, *format), inputPath, outputPath)
}

func validateIREmitArgs(fs *flag.FlagSet, format, out string) {
	if len(fs.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "Error: IR JSON file path required\n")
		fs.Usage()
		os.Exit(1)
	}
	if format == "" {
		fmt.Fprintf(os.Stderr, "Error: --format flag required\n")
		fs.Usage()
		os.Exit(1)
	}
	if out == "" {
		fmt.Fprintf(os.Stderr, "Error: --out flag required\n")
		fs.Usage()
		os.Exit(1)
	}
}

func loadPlugin(loader *plugins.Loader, pluginDir, format string) *plugins.Plugin {
	if pluginDir != "" {
		if err := loader.LoadFromDir(pluginDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to load plugins: %v\n", err)
			os.Exit(1)
		}
	}
	plugin, err := loader.GetPlugin("format." + format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: plugin not found: %v\n", err)
		os.Exit(1)
	}
	return plugin
}

func emitNativeFromIR(plugin *plugins.Plugin, irPath, outputPath string) {
	resp, err := plugins.ExecutePlugin(plugin, plugins.NewEmitNativeRequest(irPath, filepath.Dir(outputPath)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to emit native format: %v\n", err)
		os.Exit(1)
	}
	result, err := plugins.ParseEmitNativeResult(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse result: %v\n", err)
		os.Exit(1)
	}
	if result.OutputPath != outputPath {
		if err := os.Rename(result.OutputPath, outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to move output: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("Native format emitted successfully: %s\n", outputPath)
	if result.LossClass != "" {
		fmt.Printf("Loss class: %s\n", result.LossClass)
	}
}

func runIREmit(args []string) {
	fs := flag.NewFlagSet("ir-emit", flag.ExitOnError)
	format := fs.String("format", "", "Target format (required)")
	out := fs.String("out", "", "Output path (required)")
	pluginDir := fs.String("plugin-dir", "", "Path to plugin directory (default: embedded plugins)")
	fs.Parse(args)

	validateIREmitArgs(fs, *format, *out)

	irPath, _ := filepath.Abs(fs.Args()[0])
	outputPath, _ := filepath.Abs(*out)

	plugin := loadPlugin(plugins.NewLoader(), *pluginDir, *format)

	fmt.Printf("Emitting native format from IR: %s\n", irPath)
	fmt.Printf("  Format: %s\n", *format)
	fmt.Printf("  Output: %s\n", outputPath)
	fmt.Println()

	emitNativeFromIR(plugin, irPath, outputPath)
}

func runIRGenerate(args []string) {
	fs := flag.NewFlagSet("ir-generate", flag.ExitOnError)
	fs.Parse(args)

	if len(fs.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "Error: capsule path required\n")
		fs.Usage()
		os.Exit(1)
	}

	capsulePath, _ := filepath.Abs(fs.Args()[0])
	fmt.Printf("IR generation not yet implemented for: %s\n", capsulePath)
	fmt.Println("This command would:")
	fmt.Println("  1. Unpack the capsule")
	fmt.Println("  2. Detect the format")
	fmt.Println("  3. Extract IR from the artifact")
	fmt.Println("  4. Repack the capsule with IR included")
	os.Exit(1)
}

func runIRInfo(args []string) {
	fs := flag.NewFlagSet("ir-info", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	irPath := validateIRInfoArgs(fs)
	ir := loadIRFile(irPath)

	if *jsonOutput {
		outputIRAsJSON(ir)
		return
	}
	printIRSummary(irPath, ir)
}

func validateIRInfoArgs(fs *flag.FlagSet) string {
	if len(fs.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "Error: IR JSON file path required\n")
		fs.Usage()
		os.Exit(1)
	}
	return fs.Args()[0]
}

func loadIRFile(irPath string) map[string]interface{} {
	data, err := os.ReadFile(irPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read IR file: %v\n", err)
		os.Exit(1)
	}
	var ir map[string]interface{}
	if err := json.Unmarshal(data, &ir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse IR file: %v\n", err)
		os.Exit(1)
	}
	return ir
}

func outputIRAsJSON(ir map[string]interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(ir)
}

func printIRSummary(irPath string, ir map[string]interface{}) {
	fmt.Printf("IR Information: %s\n\nTop-level keys:\n", irPath)
	for k := range ir {
		fmt.Printf("  - %s\n", k)
	}
	if meta, ok := ir["metadata"].(map[string]interface{}); ok {
		fmt.Printf("\nMetadata:\n")
		for k, v := range meta {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}
	if books, ok := ir["books"].([]interface{}); ok {
		fmt.Printf("\nBooks: %d\n", len(books))
	}
}

func printUsage() {
	fmt.Println(`capsule-format - Format detection and conversion tools

Usage:
  capsule-format <command> [options]

Commands:
  detect        Detect file format using plugins
  convert       Convert file to different format via IR
  ir-extract    Extract IR from a file
  ir-emit       Emit native format from IR
  ir-generate   Generate IR for capsule without one
  ir-info       Display IR structure summary

Options for 'detect':
  --plugin-dir  Path to plugin directory (default: embedded plugins)

Options for 'convert':
  --to          Target format (required)
  --out         Output path (required)
  --plugin-dir  Path to plugin directory (default: embedded plugins)

Options for 'ir-extract':
  --format      Source format (required, e.g., usfm, osis)
  --out         Output IR JSON path (required)
  --plugin-dir  Path to plugin directory (default: embedded plugins)

Options for 'ir-emit':
  --format      Target format (required, e.g., osis, html)
  --out         Output path (required)
  --plugin-dir  Path to plugin directory (default: embedded plugins)

Options for 'ir-info':
  --json        Output as JSON

Prefer using 'capsule format' instead of this standalone binary.`)
}
