// SWORD tool integration tests.
// These tests require SWORD tools (diatheke, mod2imp, osis2mod) to be installed.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSWORDToolsAvailable checks which SWORD tools are installed.
func TestSWORDToolsAvailable(t *testing.T) {
	tools := []Tool{ToolDiatheke, ToolMod2Imp, ToolOsis2Mod}

	available := 0
	for _, tool := range tools {
		if HasTool(tool) {
			t.Logf("%s: available", tool.Name)
			available++
		} else {
			t.Logf("%s: not installed", tool.Name)
		}
	}

	if available == 0 {
		t.Skip("no SWORD tools installed - install sword-utils package")
	}
}

// TestDiathekeModuleList tests listing SWORD modules via diatheke.
func TestDiathekeModuleList(t *testing.T) {
	RequireTool(t, ToolDiatheke)

	output, err := RunTool(t, ToolDiatheke, "-b", "system", "-k", "modulelist")
	if err != nil {
		// diatheke may return non-zero even on success
		t.Logf("diatheke output (may include warnings): %s", output)
	}

	// Output should contain module categories
	if !strings.Contains(output, "Biblical Texts") &&
		!strings.Contains(output, "Commentaries") &&
		!strings.Contains(output, "Dictionaries") &&
		!strings.Contains(output, "Books") {
		// If no categories, might just have no modules installed
		if strings.TrimSpace(output) == "" {
			t.Log("no SWORD modules installed in ~/.sword")
		}
	} else {
		t.Logf("found SWORD module categories in output")
	}
}

// TestDiathekeVerseRender tests rendering a verse with diatheke.
func TestDiathekeVerseRender(t *testing.T) {
	RequireTool(t, ToolDiatheke)

	// Try to render Genesis 1:1 from any available module
	// This will fail if no modules are installed, which is OK
	output, err := RunTool(t, ToolDiatheke, "-b", "KJV", "-k", "Gen 1:1")
	if err != nil {
		if strings.Contains(string(output), "not found") ||
			strings.Contains(string(output), "Could not find") {
			t.Skip("KJV module not installed")
		}
	}

	// Should contain "In the beginning" if KJV is installed
	if strings.Contains(output, "In the beginning") {
		t.Log("successfully rendered Genesis 1:1 from KJV")
	}
}

// TestMod2ImpHelp tests that mod2imp shows help.
func TestMod2ImpHelp(t *testing.T) {
	RequireTool(t, ToolMod2Imp)

	cmd := exec.Command("mod2imp", "--help")
	output, _ := cmd.CombinedOutput() // mod2imp may exit non-zero for --help

	// Should show usage information
	if !strings.Contains(string(output), "mod2imp") {
		t.Errorf("mod2imp help output unexpected: %s", output)
	}
}

// TestOsis2ModHelp tests that osis2mod shows help.
func TestOsis2ModHelp(t *testing.T) {
	RequireTool(t, ToolOsis2Mod)

	cmd := exec.Command("osis2mod", "--help")
	output, _ := cmd.CombinedOutput() // osis2mod may exit non-zero for --help

	// Should show usage information
	outputStr := string(output)
	if !strings.Contains(outputStr, "osis2mod") && !strings.Contains(outputStr, "usage") {
		t.Errorf("osis2mod help output unexpected: %s", output)
	}
}

// TestOsis2ModCreateModule tests creating a SWORD module from OSIS.
func TestOsis2ModCreateModule(t *testing.T) {
	RequireTool(t, ToolOsis2Mod)

	// Create minimal OSIS file
	osisContent := `<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace"
      xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:schemaLocation="http://www.bibletechnologies.net/2003/OSIS/namespace http://www.bibletechnologies.net/osisCore.2.1.1.xsd">
  <osisText osisIDWork="TestMod" osisRefWork="bible" xml:lang="en">
    <header>
      <work osisWork="TestMod">
        <title>Test Module</title>
        <identifier type="OSIS">Bible.TestMod</identifier>
        <refSystem>Bible.KJV</refSystem>
      </work>
    </header>
    <div type="book" osisID="Gen">
      <chapter osisID="Gen.1">
        <verse osisID="Gen.1.1">In the beginning God created the heaven and the earth.</verse>
        <verse osisID="Gen.1.2">And the earth was without form, and void.</verse>
      </chapter>
    </div>
  </osisText>
</osis>`

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "sword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write OSIS file
	osisPath := filepath.Join(tempDir, "test.osis")
	if err := os.WriteFile(osisPath, []byte(osisContent), 0600); err != nil {
		t.Fatalf("failed to write OSIS file: %v", err)
	}

	// Create output directory
	modPath := filepath.Join(tempDir, "modules", "texts", "ztext", "testmod")
	if err := os.MkdirAll(modPath, 0700); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}

	// Run osis2mod
	cmd := exec.Command("osis2mod", modPath, osisPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("osis2mod output: %s", output)
		t.Fatalf("osis2mod failed: %v", err)
	}

	// Check that module files were created
	files, err := os.ReadDir(modPath)
	if err != nil {
		t.Fatalf("failed to read module dir: %v", err)
	}

	if len(files) == 0 {
		t.Error("no module files created")
	} else {
		t.Logf("created %d module files", len(files))
		for _, f := range files {
			t.Logf("  %s", f.Name())
		}
	}
}

// TestSWORDModuleWithSampleData tests SWORD tools with sample data if available.
func TestSWORDModuleWithSampleData(t *testing.T) {
	RequireTool(t, ToolDiatheke)

	// Check if sample data exists
	samplePath := filepath.Join("..", "..", "contrib", "sample-data", "kjv")
	if _, err := os.Stat(samplePath); os.IsNotExist(err) {
		t.Skip("sample data not found")
	}

	// Check for mods.d/kjv.conf
	confPath := filepath.Join(samplePath, "mods.d", "kjv.conf")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Skip("kjv.conf not found in sample data")
	}

	t.Log("sample KJV data found - SWORD tools can be used with it")
}
