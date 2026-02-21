// Package integration provides integration tests for juniper plugins.
// These tests verify that all 4 juniper-related plugins are discovered and functional.
// Note: Plugins have been reorganized from plugins/juniper/ to standard plugin directories:
//   - format.sword-pure (was juniper.sword)
//   - format.esword (merged with juniper.esword)
//   - tool.repoman (was juniper.repoman)
//   - tool.hugo (was juniper.hugo)
package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/JuniperBible/juniper/core/sqlite"
)

// juniperPlugin describes a plugin to test (formerly juniper plugins).
type juniperPlugin struct {
	name        string
	kind        string
	description string
	commands    []string
	dir         string // relative path from plugins/ directory
	binary      string // binary name
}

// juniperPlugins lists all formerly-juniper plugins and their expected properties.
// Note: format.esword is the original format plugin (with merged juniper functionality)
// and uses IPC-only mode (no info CLI command).
// Standalone plugins are now in plugins/format-<name>/ (with hyphen) directories.
// Note: format.sword-pure is now embedded only (canonical package in core/formats/sword-pure/)
// and doesn't have a standalone plugin wrapper.
var juniperPlugins = []juniperPlugin{
	{
		name:        "format.esword",
		kind:        "format",
		description: "e-Sword SQLite parser",
		commands:    []string{"detect", "ingest", "enumerate"},
		dir:         "format-esword",
		binary:      "format-esword",
	},
	{
		name:        "tool.repoman",
		kind:        "tool",
		description: "SWORD repository manager",
		commands:    []string{"list-sources", "refresh", "list", "install", "installed", "uninstall", "verify"},
		dir:         "tool/repoman",
		binary:      "repoman",
	},
	{
		name:        "tool.hugo",
		kind:        "tool",
		description: "Hugo JSON output generator",
		commands:    []string{"generate"},
		dir:         "tool/hugo",
		binary:      "hugo",
	},
}

// pluginsWithInfoCommand lists plugins that support the "info" CLI command.
// format.esword uses IPC-only mode (original format plugin pattern).
var pluginsWithInfoCommand = map[string]bool{
	"tool.repoman": true,
	"tool.hugo":    true,
}

// pluginInfoResponse represents the response from a plugin info command.
type pluginInfoResponse struct {
	PluginID    string `json:"plugin_id"`
	Version     string `json:"version"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

// ipcRequest represents an IPC request sent to a plugin.
type ipcRequest struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args"`
}

// ipcResponse represents an IPC response from a plugin.
type ipcResponse struct {
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// getPluginDir returns the full path to a plugin directory.
func getPluginDir(plugin juniperPlugin) string {
	return filepath.Join("..", "..", "plugins", plugin.dir)
}

// TestJuniperPluginBuilds verifies all formerly-juniper plugins can be built.
func TestJuniperPluginBuilds(t *testing.T) {
	for _, plugin := range juniperPlugins {
		plugin := plugin // capture range variable

		t.Run(plugin.name, func(t *testing.T) {
			pluginDir := getPluginDir(plugin)

			// Verify plugin directory exists
			if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
				t.Fatalf("plugin directory not found: %s", pluginDir)
			}

			// Verify go.mod exists (now all plugins should have their own or use the main module)
			goModPath := filepath.Join(pluginDir, "go.mod")
			mainModPath := filepath.Join("..", "..", "go.mod")
			if _, err := os.Stat(goModPath); os.IsNotExist(err) {
				if _, err := os.Stat(mainModPath); os.IsNotExist(err) {
					t.Fatalf("go.mod not found: %s or %s", goModPath, mainModPath)
				}
			}

			// Build the plugin
			cmd := exec.Command("go", "build", "-o", plugin.binary, ".")
			cmd.Dir = pluginDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("failed to build plugin: %v\nOutput: %s", err, output)
			}

			// Verify binary was created
			binaryPath := filepath.Join(pluginDir, plugin.binary)
			if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
				t.Fatalf("binary not found after build: %s", binaryPath)
			}

			// Clean up binary
			defer os.Remove(binaryPath)
		})
	}
}

// TestJuniperPluginInfo verifies plugin info command returns correct metadata.
// Note: Only tests plugins that support the "info" CLI command.
func TestJuniperPluginInfo(t *testing.T) {
	for _, plugin := range juniperPlugins {
		plugin := plugin // capture range variable

		// Skip plugins that don't support the info command
		if !pluginsWithInfoCommand[plugin.name] {
			continue
		}

		t.Run(plugin.name, func(t *testing.T) {
			pluginDir := getPluginDir(plugin)

			// Build the plugin first
			buildCmd := exec.Command("go", "build", "-o", plugin.binary, ".")
			buildCmd.Dir = pluginDir
			if output, err := buildCmd.CombinedOutput(); err != nil {
				t.Skipf("failed to build plugin: %v\nOutput: %s", err, output)
			}
			defer os.Remove(filepath.Join(pluginDir, plugin.binary))

			// Run info command
			binaryPath := filepath.Join(pluginDir, plugin.binary)
			cmd := exec.Command(binaryPath, "info")
			output, err := cmd.Output()
			if err != nil {
				t.Fatalf("info command failed: %v", err)
			}

			// Parse response
			var info pluginInfoResponse
			if err := json.Unmarshal(output, &info); err != nil {
				t.Fatalf("failed to parse info response: %v\nOutput: %s", err, output)
			}

			// Verify fields
			if info.PluginID != plugin.name {
				t.Errorf("expected plugin_id %q, got %q", plugin.name, info.PluginID)
			}
			if info.Kind != plugin.kind {
				t.Errorf("expected kind %q, got %q", plugin.kind, info.Kind)
			}
			if info.Version == "" {
				t.Error("version is empty")
			}
			if info.Description == "" {
				t.Error("description is empty")
			}
		})
	}
}

// TestJuniperPluginIPC verifies IPC communication works for each plugin.
func TestJuniperPluginIPC(t *testing.T) {
	for _, plugin := range juniperPlugins {
		plugin := plugin // capture range variable

		t.Run(plugin.name, func(t *testing.T) {
			pluginDir := getPluginDir(plugin)

			// Build the plugin first
			buildCmd := exec.Command("go", "build", "-o", plugin.binary, ".")
			buildCmd.Dir = pluginDir
			if output, err := buildCmd.CombinedOutput(); err != nil {
				t.Skipf("failed to build plugin: %v\nOutput: %s", err, output)
			}
			defer os.Remove(filepath.Join(pluginDir, plugin.binary))

			// Test invalid command
			binaryPath := filepath.Join(pluginDir, plugin.binary)
			cmd := exec.Command(binaryPath, "ipc")
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatalf("failed to get stdin pipe: %v", err)
			}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatalf("failed to get stdout pipe: %v", err)
			}

			if err := cmd.Start(); err != nil {
				t.Fatalf("failed to start plugin: %v", err)
			}

			// Send invalid command
			req := ipcRequest{
				Command: "nonexistent-command",
				Args:    map[string]interface{}{},
			}
			reqBytes, _ := json.Marshal(req)
			reqBytes = append(reqBytes, '\n')
			stdin.Write(reqBytes)
			stdin.Close()

			// Read response
			scanner := bufio.NewScanner(stdout)
			if !scanner.Scan() {
				t.Fatal("no response from plugin")
			}

			var resp ipcResponse
			if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			// Should be an error response
			if resp.Status != "error" {
				t.Errorf("expected error status for invalid command, got %q", resp.Status)
			}

			cmd.Wait()
		})
	}
}

// TestJuniperSwordDetect tests SWORD detection command using format-sword plugin.
// Note: The format-sword-pure functionality is now embedded only (no standalone wrapper).
func TestJuniperSwordDetect(t *testing.T) {
	pluginDir := filepath.Join("..", "..", "plugins", "format-sword")

	// Build the plugin
	buildCmd := exec.Command("go", "build", "-o", "format-sword", ".")
	buildCmd.Dir = pluginDir
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Skipf("failed to build plugin: %v\nOutput: %s", err, output)
	}
	defer os.Remove(filepath.Join(pluginDir, "format-sword"))

	// Create a temp directory with SWORD structure
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	modulesDir := filepath.Join(tmpDir, "modules")
	os.MkdirAll(modsDir, 0700)
	os.MkdirAll(modulesDir, 0700)

	// Create a minimal conf file
	confContent := `[TestModule]
DataPath=./modules/texts/ztext/testmod/
ModDrv=zText
Encoding=UTF-8
`
	os.WriteFile(filepath.Join(modsDir, "testmod.conf"), []byte(confContent), 0600)

	// Run detect command
	binaryPath := filepath.Join(pluginDir, "format-sword")
	cmd := exec.Command(binaryPath, "ipc")
	stdin, _ := cmd.StdinPipe()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	cmd.Start()

	req := ipcRequest{
		Command: "detect",
		Args: map[string]interface{}{
			"path": tmpDir,
		},
	}
	reqBytes, _ := json.Marshal(req)
	stdin.Write(append(reqBytes, '\n'))
	stdin.Close()
	cmd.Wait()

	// Parse response
	scanner := bufio.NewScanner(&stdout)
	if scanner.Scan() {
		var resp ipcResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.Status != "ok" {
			t.Errorf("expected ok status, got %q: %s", resp.Status, resp.Error)
		}
	}
}

// TestJuniperRepoManListSources tests repoman list-sources command.
func TestJuniperRepoManListSources(t *testing.T) {
	pluginDir := filepath.Join("..", "..", "plugins", "tool", "repoman")

	// Build the plugin
	buildCmd := exec.Command("go", "build", "-o", "repoman", ".")
	buildCmd.Dir = pluginDir
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Skipf("failed to build plugin: %v\nOutput: %s", err, output)
	}
	defer os.Remove(filepath.Join(pluginDir, "repoman"))

	// Run list-sources command
	binaryPath := filepath.Join(pluginDir, "repoman")
	cmd := exec.Command(binaryPath, "ipc")
	stdin, _ := cmd.StdinPipe()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	cmd.Start()

	req := ipcRequest{
		Command: "list-sources",
		Args:    map[string]interface{}{},
	}
	reqBytes, _ := json.Marshal(req)
	stdin.Write(append(reqBytes, '\n'))
	stdin.Close()
	cmd.Wait()

	// Parse response
	scanner := bufio.NewScanner(&stdout)
	if scanner.Scan() {
		var resp ipcResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.Status != "ok" {
			t.Errorf("expected ok status, got %q: %s", resp.Status, resp.Error)
		}
		// Verify sources exist in result
		if result, ok := resp.Result.(map[string]interface{}); ok {
			if sources, ok := result["sources"].([]interface{}); ok {
				if len(sources) == 0 {
					t.Error("expected at least one source")
				}
			}
		}
	}
}

// TestJuniperESwordDetect tests e-Sword detection command.
func TestJuniperESwordDetect(t *testing.T) {
	pluginDir := filepath.Join("..", "..", "plugins", "format-esword")

	// Build the plugin
	buildCmd := exec.Command("go", "build", "-o", "format-esword", ".")
	buildCmd.Dir = pluginDir
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Skipf("failed to build plugin: %v\nOutput: %s", err, output)
	}
	defer os.Remove(filepath.Join(pluginDir, "format-esword"))

	// Create a temp e-Sword Bible database
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bblx")

	// Create a valid SQLite database with e-Sword Bible structure
	db, err := sqlite.Open(testFile)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create required Bible table
	_, err = db.Exec("CREATE TABLE Bible (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")
	if err != nil {
		db.Close()
		t.Fatalf("failed to create Bible table: %v", err)
	}

	// Insert a test verse
	_, err = db.Exec("INSERT INTO Bible (Book, Chapter, Verse, Scripture) VALUES (1, 1, 1, 'In the beginning')")
	if err != nil {
		db.Close()
		t.Fatalf("failed to insert test verse: %v", err)
	}

	db.Close()

	// Run detect command
	binaryPath := filepath.Join(pluginDir, "format-esword")
	cmd := exec.Command(binaryPath, "ipc")
	stdin, _ := cmd.StdinPipe()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	cmd.Start()

	req := ipcRequest{
		Command: "detect",
		Args: map[string]interface{}{
			"path": testFile,
		},
	}
	reqBytes, _ := json.Marshal(req)
	stdin.Write(append(reqBytes, '\n'))
	stdin.Close()
	cmd.Wait()

	// Parse response
	scanner := bufio.NewScanner(&stdout)
	if scanner.Scan() {
		var resp ipcResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.Status != "ok" {
			t.Errorf("expected ok status, got %q: %s", resp.Status, resp.Error)
		}
		// Verify detection result
		if result, ok := resp.Result.(map[string]interface{}); ok {
			if detected, ok := result["detected"].(bool); ok && !detected {
				t.Error("expected e-Sword file to be detected")
			}
		}
	}
}

// TestJuniperHugoGenerate tests hugo generate command with mock data.
func TestJuniperHugoGenerate(t *testing.T) {
	pluginDir := filepath.Join("..", "..", "plugins", "tool", "hugo")

	// Build the plugin
	buildCmd := exec.Command("go", "build", "-o", "hugo", ".")
	buildCmd.Dir = pluginDir
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Skipf("failed to build plugin: %v\nOutput: %s", err, output)
	}
	defer os.Remove(filepath.Join(pluginDir, "hugo"))

	// Create test input file
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.json")
	outputDir := filepath.Join(tmpDir, "output")

	inputData := `{
		"id": "TST",
		"title": "Test Bible",
		"language": "en",
		"books": [{
			"id": "Gen",
			"name": "Genesis",
			"testament": "OT",
			"chapters": [{
				"number": 1,
				"verses": [{"number": 1, "text": "In the beginning."}]
			}]
		}]
	}`
	os.WriteFile(inputPath, []byte(inputData), 0600)

	// Run generate command
	binaryPath := filepath.Join(pluginDir, "hugo")
	cmd := exec.Command(binaryPath, "ipc")
	stdin, _ := cmd.StdinPipe()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	cmd.Start()

	req := ipcRequest{
		Command: "generate",
		Args: map[string]interface{}{
			"input":  inputPath,
			"output": outputDir,
		},
	}
	reqBytes, _ := json.Marshal(req)
	stdin.Write(append(reqBytes, '\n'))
	stdin.Close()
	cmd.Wait()

	// Parse response
	scanner := bufio.NewScanner(&stdout)
	if scanner.Scan() {
		var resp ipcResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.Status != "ok" {
			t.Errorf("expected ok status, got %q: %s", resp.Status, resp.Error)
		}
	}

	// Verify output was created
	if _, err := os.Stat(filepath.Join(outputDir, "bibles.json")); os.IsNotExist(err) {
		t.Error("bibles.json was not created")
	}
}
