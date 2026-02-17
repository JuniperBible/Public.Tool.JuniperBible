//go:build sdk

// Plugin tool-libsword provides operations for SWORD Bible modules using libsword.
// This is the subdirectory version used alongside the top-level tool-libsword.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// TranscriptEvent extends the base event with libsword-specific fields.
type TranscriptEvent struct {
	ipc.TranscriptEvent
	Module   string   `json:"module,omitempty"`
	Ref      string   `json:"ref,omitempty"`
	Text     string   `json:"text,omitempty"`
	Modules  []string `json:"modules,omitempty"`
	KeyCount int      `json:"key_count,omitempty"`
}

// LibswordRequest extends ToolRunRequest with SWORD-specific fields.
type LibswordRequest struct {
	ipc.ToolRunRequest
	SwordPath string `json:"sword_path"`
	Module    string `json:"module,omitempty"`
	Ref       string `json:"ref,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tool-libsword <command> [args]")
		os.Exit(1)
	}

	config := &ipc.ToolConfig{
		PluginName: "tool-libsword",
		Info: ipc.ToolInfo{
			Name:        "tool-libsword",
			Version:     "1.1.0",
			Type:        "tool",
			Description: "SWORD Bible module operations using libsword",
			Profiles: []ipc.ProfileInfo{
				{ID: "list-modules", Description: "List available SWORD modules"},
				{ID: "render-verse", Description: "Render a specific verse"},
				{ID: "render-all", Description: "Render all verses in a module"},
				{ID: "enumerate-keys", Description: "Enumerate all keys in a module"},
				{ID: "mod2osis", Description: "Convert module to OSIS XML"},
				{ID: "osis2mod", Description: "Create SWORD module from OSIS XML"},
			},
			Requires: []string{"diatheke", "mod2osis", "mod2imp", "osis2mod"},
		},
		Profiles: map[string]ipc.ProfileHandler{
			"list-modules":   profileListModules,
			"render-verse":   profileRenderVerse,
			"render-all":     profileRenderAll,
			"enumerate-keys": profileEnumerateKeys,
			"mod2osis":       profileMod2OSIS,
			"osis2mod":       profileOsis2Mod,
		},
	}

	switch os.Args[1] {
	case "info":
		ipc.PrintToolInfo(config.Info)
	case "run":
		reqPath, outDir := ipc.ParseToolFlags()
		req := loadLibswordRequest(reqPath, outDir)
		executeWithTranscript(req, config)
	case "ipc":
		ipc.RunStandardToolIPC(config)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func loadLibswordRequest(reqPath, outDir string) *LibswordRequest {
	reqData, err := os.ReadFile(reqPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read request: %v\n", err)
		os.Exit(1)
	}

	var req LibswordRequest
	if err := json.Unmarshal(reqData, &req); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse request: %v\n", err)
		os.Exit(1)
	}

	req.OutDir = outDir
	if err := os.MkdirAll(outDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output dir: %v\n", err)
		os.Exit(1)
	}

	return &req
}

func executeWithTranscript(req *LibswordRequest, config *ipc.ToolConfig) {
	transcript := ipc.NewTranscript(req.OutDir)
	defer transcript.Close()

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "start", Plugin: config.PluginName, Profile: req.Profile},
	})

	handler, ok := config.Profiles[req.Profile]
	if !ok {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{Event: "error", Error: "unknown profile: " + req.Profile},
		})
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{Event: "end", ExitCode: 1},
		})
		os.Exit(1)
	}

	baseReq := &ipc.ToolRunRequest{
		Profile: req.Profile,
		Args:    req.Args,
		OutDir:  req.OutDir,
	}
	if baseReq.Args == nil {
		baseReq.Args = make(map[string]string)
	}
	baseReq.Args["sword_path"] = req.SwordPath
	baseReq.Args["module"] = req.Module
	baseReq.Args["ref"] = req.Ref

	err := handler(baseReq, transcript)

	exitCode := 0
	if err != nil {
		exitCode = 1
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{Event: "error", Error: err.Error()},
		})
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "end", ExitCode: exitCode},
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Profile execution failed: %v\n", err)
		os.Exit(1)
	}
}

func profileListModules(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	swordPath := req.Args["sword_path"]
	modsDir := filepath.Join(swordPath, "mods.d")
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
		TranscriptEvent: ipc.TranscriptEvent{Event: "list_modules"},
		Modules:         modules,
	})

	return nil
}

func profileRenderVerse(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	swordPath := req.Args["sword_path"]
	module := req.Args["module"]
	ref := req.Args["ref"]

	if module == "" {
		return fmt.Errorf("module required for render-verse")
	}
	if ref == "" {
		return fmt.Errorf("ref required for render-verse")
	}

	env := append(os.Environ(), "SWORD_PATH="+swordPath)

	cmd := exec.Command("diatheke", "-b", module, "-k", ref)
	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{Event: "verse_error", Error: err.Error()},
			Module:          module,
			Ref:             ref,
		})
		return err
	}

	text := strings.TrimSpace(string(output))

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "verse"},
		Module:          module,
		Ref:             ref,
		Text:            text,
	})

	return nil
}

func profileRenderAll(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	swordPath := req.Args["sword_path"]
	module := req.Args["module"]

	if module == "" {
		return fmt.Errorf("module required for render-all")
	}

	env := append(os.Environ(), "SWORD_PATH="+swordPath)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "module_start"},
		Module:          module,
	})

	cmd := exec.Command("mod2imp", module)
	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{Event: "module_error", Error: err.Error()},
			Module:          module,
		})
		return err
	}

	outPath := filepath.Join(req.OutDir, module+".imp")
	if err := os.WriteFile(outPath, output, 0600); err != nil {
		return err
	}

	lines := strings.Split(string(output), "\n")
	keyCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "$$$") {
			keyCount++
		}
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "module_end"},
		Module:          module,
		KeyCount:        keyCount,
	})

	return nil
}

func profileEnumerateKeys(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	swordPath := req.Args["sword_path"]
	module := req.Args["module"]

	if module == "" {
		return fmt.Errorf("module required for enumerate-keys")
	}

	env := append(os.Environ(), "SWORD_PATH="+swordPath)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "enumerate_start"},
		Module:          module,
	})

	cmd := exec.Command("mod2imp", module)
	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{Event: "enumerate_error", Error: err.Error()},
			Module:          module,
		})
		return err
	}

	var keys []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "$$$") {
			key := strings.TrimPrefix(line, "$$$")
			keys = append(keys, key)
		}
	}

	keysPath := filepath.Join(req.OutDir, module+".keys")
	keysData := strings.Join(keys, "\n")
	if err := os.WriteFile(keysPath, []byte(keysData), 0600); err != nil {
		return err
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "enumerate_end"},
		Module:          module,
		KeyCount:        len(keys),
	})

	return nil
}

func profileMod2OSIS(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	swordPath := req.Args["sword_path"]
	module := req.Args["module"]

	if module == "" {
		return fmt.Errorf("module required for mod2osis")
	}

	env := append(os.Environ(), "SWORD_PATH="+swordPath)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "convert_start"},
		Module:          module,
	})

	cmd := exec.Command("mod2osis", module)
	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{Event: "convert_error", Error: err.Error()},
			Module:          module,
		})
		return err
	}

	osisPath := filepath.Join(req.OutDir, module+".osis.xml")
	if err := os.WriteFile(osisPath, output, 0600); err != nil {
		return err
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "convert_end",
			Data: map[string]interface{}{
				"output_file": module + ".osis.xml",
				"size_bytes":  len(output),
			},
		},
		Module: module,
	})

	return nil
}

func profileOsis2Mod(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	osisFile := req.Args["osis_file"]
	if osisFile == "" {
		return fmt.Errorf("osis_file required for osis2mod")
	}

	moduleName := req.Args["module_name"]
	if moduleName == "" {
		moduleName = req.Args["module"]
	}
	if moduleName == "" {
		return fmt.Errorf("module_name required for osis2mod")
	}

	versification := req.Args["versification"]
	compress := req.Args["compress"]

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "create_start",
			Data: map[string]interface{}{
				"osis_file":     osisFile,
				"versification": versification,
				"compress":      compress,
			},
		},
		Module: moduleName,
	})

	modOutDir := filepath.Join(req.OutDir, "modules", "texts", "ztext", strings.ToLower(moduleName))
	if err := os.MkdirAll(modOutDir, 0700); err != nil {
		return fmt.Errorf("failed to create module directory: %w", err)
	}

	args := []string{modOutDir, osisFile}

	if versification != "" {
		args = append(args, "-v", versification)
	}

	if compress != "" {
		args = append(args, "-z", compress)
	}

	cmd := exec.Command("osis2mod", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "create_error",
				Error: fmt.Sprintf("%v: %s", err, string(output)),
			},
			Module: moduleName,
		})
		return fmt.Errorf("osis2mod failed: %w: %s", err, string(output))
	}

	modsDir := filepath.Join(req.OutDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0700); err != nil {
		return fmt.Errorf("failed to create mods.d directory: %w", err)
	}

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
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		return fmt.Errorf("failed to write conf file: %w", err)
	}

	entries, _ := os.ReadDir(modOutDir)
	fileCount := len(entries)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "create_end",
			Data: map[string]interface{}{
				"output_dir":      modOutDir,
				"conf_file":       confPath,
				"files_count":     fileCount,
				"osis2mod_output": string(output),
			},
		},
		Module: moduleName,
	})

	return nil
}
