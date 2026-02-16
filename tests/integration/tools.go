// Package integration provides integration tests for capsule functionality.
// These tests may require external tools to be installed.
package integration

import (
	"os/exec"
	"sync"
	"testing"
)

// Tool represents an external tool that may be required for tests.
type Tool struct {
	Name        string   // Display name
	Command     string   // Command to check (e.g., "diatheke")
	Args        []string // Args to verify tool works (e.g., ["--version"])
	Description string   // What the tool is used for
}

// Common tools used in integration tests.
var (
	ToolDiatheke = Tool{
		Name:        "diatheke",
		Command:     "diatheke",
		Args:        []string{"-b", "system", "-k", "modulelist"},
		Description: "SWORD module query tool",
	}
	ToolMod2Imp = Tool{
		Name:        "mod2imp",
		Command:     "mod2imp",
		Args:        []string{"--help"},
		Description: "SWORD module to IMP converter",
	}
	ToolOsis2Mod = Tool{
		Name:        "osis2mod",
		Command:     "osis2mod",
		Args:        []string{"--help"},
		Description: "OSIS to SWORD module converter",
	}
	ToolCalibre = Tool{
		Name:        "calibre",
		Command:     "ebook-convert",
		Args:        []string{"--version"},
		Description: "E-book conversion tool",
	}
	ToolPandoc = Tool{
		Name:        "pandoc",
		Command:     "pandoc",
		Args:        []string{"--version"},
		Description: "Universal document converter",
	}
	ToolXMLLint = Tool{
		Name:        "xmllint",
		Command:     "xmllint",
		Args:        []string{"--version"},
		Description: "XML validation and XPath tool",
	}
	ToolXSLTProc = Tool{
		Name:        "xsltproc",
		Command:     "xsltproc",
		Args:        []string{"--version"},
		Description: "XSLT processor",
	}
	ToolSQLite3 = Tool{
		Name:        "sqlite3",
		Command:     "sqlite3",
		Args:        []string{"--version"},
		Description: "SQLite command-line interface",
	}
	ToolUnRTF = Tool{
		Name:        "unrtf",
		Command:     "unrtf",
		Args:        []string{"--version"},
		Description: "RTF to other format converter",
	}
)

// toolCache caches tool availability checks.
var (
	toolCache   = make(map[string]bool)
	toolCacheMu sync.RWMutex
)

// HasTool checks if a tool is available on the system.
// Results are cached for performance.
func HasTool(tool Tool) bool {
	toolCacheMu.RLock()
	if available, ok := toolCache[tool.Command]; ok {
		toolCacheMu.RUnlock()
		return available
	}
	toolCacheMu.RUnlock()

	// Check if command exists
	_, err := exec.LookPath(tool.Command)
	available := err == nil

	toolCacheMu.Lock()
	toolCache[tool.Command] = available
	toolCacheMu.Unlock()

	return available
}

// RequireTool skips the test if the specified tool is not available.
func RequireTool(t *testing.T, tool Tool) {
	t.Helper()
	if !HasTool(tool) {
		t.Skipf("skipping: %s (%s) not installed", tool.Name, tool.Command)
	}
}

// RunTool executes a tool and returns its output.
// It skips the test if the tool is not available.
func RunTool(t *testing.T, tool Tool, args ...string) (string, error) {
	t.Helper()
	RequireTool(t, tool)

	cmd := exec.Command(tool.Command, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
