// Plugin tool-libsword provides operations for SWORD Bible modules using libsword.
//
// This is a TOOL plugin (not a format plugin). It runs reference implementations
// to produce deterministic transcripts that serve as behavioral specifications.
//
// Profiles:
//   - list-modules: List all available modules
//   - render-verse: Render a specific verse
//   - render-all: Render all verses in a module
//   - enumerate-keys: List all keys in a module
//   - mod2osis: Convert module to OSIS XML
//   - osis2mod: Create SWORD module from OSIS XML
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

type IPCRequest struct {
	Command string            `json:"command"`
	Path    string            `json:"path,omitempty"`
	Args    map[string]string `json:"args,omitempty"`
}

type IPCResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ToolRunRequest is the request for running a tool operation.
type ToolRunRequest struct {
	Profile   string            `json:"profile"`
	SwordPath string            `json:"sword_path"`
	Module    string            `json:"module,omitempty"`
	Ref       string            `json:"ref,omitempty"`
	Args      map[string]string `json:"args,omitempty"`
	OutDir    string            `json:"out_dir"`
}

// TranscriptEvent represents a single event in the transcript.
type TranscriptEvent struct {
	Event     string      `json:"event"`
	Timestamp string      `json:"timestamp,omitempty"`
	Plugin    string      `json:"plugin,omitempty"`
	Profile   string      `json:"profile,omitempty"`
	Module    string      `json:"module,omitempty"`
	Ref       string      `json:"ref,omitempty"`
	Text      string      `json:"text,omitempty"`
	Error     string      `json:"error,omitempty"`
	ExitCode  int         `json:"exit_code,omitempty"`
	Modules   []string    `json:"modules,omitempty"`
	KeyCount  int         `json:"key_count,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tool-libsword <command> [args]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "info":
		printInfo()
	case "run":
		runTool()
	case "ipc":
		runIPC()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func printInfo() {
	info := map[string]interface{}{
		"name":        "tool-libsword",
		"version":     "1.1.0",
		"type":        "tool",
		"description": "SWORD Bible module operations using libsword",
		"profiles": []map[string]string{
			{"id": "list-modules", "description": "List available SWORD modules"},
			{"id": "render-verse", "description": "Render a specific verse"},
			{"id": "render-all", "description": "Render all verses in a module"},
			{"id": "enumerate-keys", "description": "Enumerate all keys in a module"},
			{"id": "mod2osis", "description": "Convert module to OSIS XML"},
			{"id": "osis2mod", "description": "Create SWORD module from OSIS XML"},
		},
		"requires": []string{"diatheke", "mod2osis", "mod2imp", "osis2mod"},
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(info)
}

func runIPC() {
	reader := bufio.NewReader(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		var req IPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			encoder.Encode(IPCResponse{Success: false, Error: err.Error()})
			continue
		}

		resp := handleIPCRequest(&req)
		encoder.Encode(resp)
	}
}

func handleIPCRequest(req *IPCRequest) IPCResponse {
	switch req.Command {
	case "info":
		return IPCResponse{
			Success: true,
			Data: map[string]interface{}{
				"name":    "tool-libsword",
				"version": "1.0.0",
				"type":    "tool",
			},
		}
	case "check":
		// Check if libsword tools are available
		available := checkTools()
		return IPCResponse{
			Success: available,
			Data:    map[string]bool{"tools_available": available},
		}
	default:
		return IPCResponse{Success: false, Error: "unknown command: " + req.Command}
	}
}

func checkTools() bool {
	tools := []string{"diatheke", "mod2osis", "mod2imp", "osis2mod"}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			return false
		}
	}
	return true
}

func runTool() {
	// Parse flags
	var reqPath, outDir string
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--request":
			if i+1 < len(os.Args) {
				reqPath = os.Args[i+1]
				i++
			}
		case "--out":
			if i+1 < len(os.Args) {
				outDir = os.Args[i+1]
				i++
			}
		}
	}

	if reqPath == "" || outDir == "" {
		fmt.Fprintln(os.Stderr, "Usage: tool-libsword run --request <path> --out <dir>")
		os.Exit(1)
	}

	// Read request
	reqData, err := safefile.ReadFile(reqPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read request: %v\n", err)
		os.Exit(1)
	}

	var req ToolRunRequest
	if err := json.Unmarshal(reqData, &req); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse request: %v\n", err)
		os.Exit(1)
	}

	req.OutDir = outDir
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output dir: %v\n", err)
		os.Exit(1)
	}

	// Run the profile
	if err := executeProfile(&req); err != nil {
		fmt.Fprintf(os.Stderr, "Profile execution failed: %v\n", err)
		os.Exit(1)
	}
}

func executeProfile(req *ToolRunRequest) error {
	transcript := ipc.NewTranscript(req.OutDir)
	defer transcript.Close()

	transcript.WriteEvent(TranscriptEvent{
		Event:     "start",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Plugin:    "tool-libsword",
		Profile:   req.Profile,
	})

	var err error
	switch req.Profile {
	case "list-modules":
		err = profileListModules(req, transcript)
	case "render-verse":
		err = profileRenderVerse(req, transcript)
	case "render-all":
		err = profileRenderAll(req, transcript)
	case "enumerate-keys":
		err = profileEnumerateKeys(req, transcript)
	case "mod2osis":
		err = profileMod2OSIS(req, transcript)
	case "osis2mod":
		err = profileOsis2Mod(req, transcript)
	default:
		err = fmt.Errorf("unknown profile: %s", req.Profile)
	}

	exitCode := 0
	if err != nil {
		exitCode = 1
		transcript.WriteEvent(TranscriptEvent{
			Event: "error",
			Error: err.Error(),
		})
	}

	transcript.WriteEvent(TranscriptEvent{
		Event:     "end",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		ExitCode:  exitCode,
	})

	return err
}

func profileListModules(req *ToolRunRequest, transcript *ipc.Transcript) error {
	// Find all .conf files in mods.d
	modsDir := filepath.Join(req.SwordPath, "mods.d")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return fmt.Errorf("failed to read mods.d: %w", err)
	}

	var modules []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".conf") {
			modName := strings.TrimSuffix(name, ".conf")
			modules = append(modules, modName)
		}
	}

	sort.Strings(modules)

	transcript.WriteEvent(TranscriptEvent{
		Event:   "list_modules",
		Modules: modules,
	})

	return nil
}

func profileRenderVerse(req *ToolRunRequest, transcript *ipc.Transcript) error {
	if req.Module == "" {
		return fmt.Errorf("module required for render-verse")
	}
	if req.Ref == "" {
		return fmt.Errorf("ref required for render-verse")
	}

	// Set SWORD_PATH environment
	env := append(os.Environ(), "SWORD_PATH="+req.SwordPath)

	// Run diatheke
	cmd := exec.Command("diatheke", "-b", req.Module, "-k", req.Ref)
	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			Event:  "verse_error",
			Module: req.Module,
			Ref:    req.Ref,
			Error:  err.Error(),
		})
		return err
	}

	text := strings.TrimSpace(string(output))

	transcript.WriteEvent(TranscriptEvent{
		Event:  "verse",
		Module: req.Module,
		Ref:    req.Ref,
		Text:   text,
	})

	return nil
}

func profileRenderAll(req *ToolRunRequest, transcript *ipc.Transcript) error {
	if req.Module == "" {
		return fmt.Errorf("module required for render-all")
	}

	env := append(os.Environ(), "SWORD_PATH="+req.SwordPath)

	transcript.WriteEvent(TranscriptEvent{
		Event:  "module_start",
		Module: req.Module,
	})

	// Use mod2imp to get all entries
	cmd := exec.Command("mod2imp", req.Module)
	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			Event:  "module_error",
			Module: req.Module,
			Error:  err.Error(),
		})
		return err
	}

	// Write output to file
	outPath := filepath.Join(req.OutDir, req.Module+".imp")
	if err := os.WriteFile(outPath, output, 0644); err != nil {
		return err
	}

	// Count entries
	lines := strings.Split(string(output), "\n")
	keyCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "$$$") {
			keyCount++
		}
	}

	transcript.WriteEvent(TranscriptEvent{
		Event:    "module_end",
		Module:   req.Module,
		KeyCount: keyCount,
	})

	return nil
}

func profileEnumerateKeys(req *ToolRunRequest, transcript *ipc.Transcript) error {
	if req.Module == "" {
		return fmt.Errorf("module required for enumerate-keys")
	}

	env := append(os.Environ(), "SWORD_PATH="+req.SwordPath)

	transcript.WriteEvent(TranscriptEvent{
		Event:  "enumerate_start",
		Module: req.Module,
	})

	// Use mod2imp and extract just the keys
	cmd := exec.Command("mod2imp", req.Module)
	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			Event:  "enumerate_error",
			Module: req.Module,
			Error:  err.Error(),
		})
		return err
	}

	// Extract keys (lines starting with $$$)
	var keys []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "$$$") {
			key := strings.TrimPrefix(line, "$$$")
			keys = append(keys, key)
		}
	}

	// Write keys to file
	keysPath := filepath.Join(req.OutDir, req.Module+".keys")
	keysData := strings.Join(keys, "\n")
	if err := os.WriteFile(keysPath, []byte(keysData), 0644); err != nil {
		return err
	}

	transcript.WriteEvent(TranscriptEvent{
		Event:    "enumerate_end",
		Module:   req.Module,
		KeyCount: len(keys),
	})

	return nil
}

func profileMod2OSIS(req *ToolRunRequest, transcript *ipc.Transcript) error {
	if req.Module == "" {
		return fmt.Errorf("module required for mod2osis")
	}

	env := append(os.Environ(), "SWORD_PATH="+req.SwordPath)

	transcript.WriteEvent(TranscriptEvent{
		Event:  "convert_start",
		Module: req.Module,
	})

	// Run mod2osis
	cmd := exec.Command("mod2osis", req.Module)
	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			Event:  "convert_error",
			Module: req.Module,
			Error:  err.Error(),
		})
		return err
	}

	// Write OSIS to file
	osisPath := filepath.Join(req.OutDir, req.Module+".osis.xml")
	if err := os.WriteFile(osisPath, output, 0644); err != nil {
		return err
	}

	transcript.WriteEvent(TranscriptEvent{
		Event:  "convert_end",
		Module: req.Module,
		Data: map[string]interface{}{
			"output_file": req.Module + ".osis.xml",
			"size_bytes":  len(output),
		},
	})

	return nil
}

func profileOsis2Mod(req *ToolRunRequest, transcript *ipc.Transcript) error {
	// Get OSIS file from Args
	osisFile := req.Args["osis_file"]
	if osisFile == "" {
		return fmt.Errorf("osis_file required for osis2mod")
	}

	// Get module name (required)
	moduleName := req.Args["module_name"]
	if moduleName == "" {
		moduleName = req.Module
	}
	if moduleName == "" {
		return fmt.Errorf("module_name required for osis2mod")
	}

	// Optional parameters
	versification := req.Args["versification"]
	compress := req.Args["compress"]

	transcript.WriteEvent(TranscriptEvent{
		Event:  "create_start",
		Module: moduleName,
		Data: map[string]interface{}{
			"osis_file":     osisFile,
			"versification": versification,
			"compress":      compress,
		},
	})

	// Build osis2mod command
	// osis2mod <output_path> <osis_file> [options]
	modOutDir := filepath.Join(req.OutDir, "modules", "texts", "ztext", strings.ToLower(moduleName))
	if err := os.MkdirAll(modOutDir, 0755); err != nil {
		return fmt.Errorf("failed to create module directory: %w", err)
	}

	args := []string{modOutDir, osisFile}

	// Add optional versification
	if versification != "" {
		args = append(args, "-v", versification)
	}

	// Add optional compression
	if compress != "" {
		args = append(args, "-z", compress)
	}

	cmd := exec.Command("osis2mod", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			Event:  "create_error",
			Module: moduleName,
			Error:  fmt.Sprintf("%v: %s", err, string(output)),
		})
		return fmt.Errorf("osis2mod failed: %w: %s", err, string(output))
	}

	// Create mods.d directory and conf file
	modsDir := filepath.Join(req.OutDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		return fmt.Errorf("failed to create mods.d directory: %w", err)
	}

	// Generate basic .conf file
	confContent := fmt.Sprintf(`[%s]
DataPath=./modules/texts/ztext/%s/
ModDrv=zText
Encoding=UTF-8
SourceType=OSIS
Lang=en
`, moduleName, strings.ToLower(moduleName))

	if versification != "" {
		confContent += fmt.Sprintf("Versification=%s\n", versification)
	}

	confPath := filepath.Join(modsDir, strings.ToLower(moduleName)+".conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0644); err != nil {
		return fmt.Errorf("failed to write conf file: %w", err)
	}

	// Count files created
	entries, _ := os.ReadDir(modOutDir)
	fileCount := len(entries)

	transcript.WriteEvent(TranscriptEvent{
		Event:  "create_end",
		Module: moduleName,
		Data: map[string]interface{}{
			"output_dir":      modOutDir,
			"conf_file":       confPath,
			"files_count":     fileCount,
			"osis2mod_output": string(output),
		},
	})

	return nil
}
