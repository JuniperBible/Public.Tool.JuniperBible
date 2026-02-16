//go:build !sdk

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPluginInfo(t *testing.T) {
	buildPlugin(t)

	cmd := exec.Command("./tool-gobible-creator", "info")
	cmd.Dir = pluginDir(t)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("info command failed: %v", err)
	}

	var info map[string]interface{}
	json.Unmarshal(output, &info)

	if info["name"] != "tool-gobible-creator" {
		t.Errorf("expected name 'tool-gobible-creator', got %v", info["name"])
	}

	profiles := info["profiles"].([]interface{})
	if len(profiles) != 3 {
		t.Errorf("expected 3 profiles, got %d", len(profiles))
	}
}

func TestValidateProfile(t *testing.T) {
	buildPlugin(t)
	tempDir := t.TempDir()

	// Create minimal OSIS file
	osisFile := filepath.Join(tempDir, "test.osis.xml")
	os.WriteFile(osisFile, []byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="Test" xml:lang="en">
    <header><work osisWork="Test"/></header>
    <div type="book" osisID="Gen">
      <chapter osisID="Gen.1">
        <verse osisID="Gen.1.1">In the beginning.</verse>
      </chapter>
    </div>
  </osisText>
</osis>`), 0644)

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "validate",
		"args":    map[string]string{"input": osisFile},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0600)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-gobible-creator", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	cmd.Run()

	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
		t.Error("transcript.jsonl not created")
	}
}

func TestInfoProfile(t *testing.T) {
	buildPlugin(t)
	tempDir := t.TempDir()

	// Create minimal OSIS file
	osisFile := filepath.Join(tempDir, "test.osis.xml")
	os.WriteFile(osisFile, []byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="TestBible" xml:lang="en">
    <header><work osisWork="TestBible"/></header>
  </osisText>
</osis>`), 0644)

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "info",
		"args":    map[string]string{"input": osisFile},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0600)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-gobible-creator", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	cmd.Run()

	// Check info file was created
	infoPath := filepath.Join(outDir, "info.json")
	if _, err := os.Stat(infoPath); os.IsNotExist(err) {
		t.Error("info.json not created")
	}
}

func TestCreateProfile(t *testing.T) {
	if !hasGoBibleCreator() {
		t.Skip("GoBibleCreator not installed")
	}

	buildPlugin(t)
	tempDir := t.TempDir()

	// Create OSIS file
	osisFile := filepath.Join(tempDir, "test.osis.xml")
	os.WriteFile(osisFile, []byte(`<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="Test" xml:lang="en">
    <header><work osisWork="Test"/></header>
    <div type="book" osisID="Gen">
      <chapter osisID="Gen.1">
        <verse osisID="Gen.1.1">In the beginning God created the heaven and the earth.</verse>
      </chapter>
    </div>
  </osisText>
</osis>`), 0644)

	// Create collection file
	collectionFile := filepath.Join(tempDir, "collection.txt")
	os.WriteFile(collectionFile, []byte(`Info: Test Bible
Source-Text: `+osisFile+`
Phone-Image-Width: 128
Phone-Image-Height: 128
`), 0644)

	reqFile := filepath.Join(tempDir, "request.json")
	req := map[string]interface{}{
		"profile": "create",
		"args": map[string]string{
			"collection": collectionFile,
			"output":     "test.jar",
		},
		"out_dir": tempDir,
	}
	reqData, _ := json.Marshal(req)
	os.WriteFile(reqFile, reqData, 0600)

	outDir := filepath.Join(tempDir, "output")
	os.MkdirAll(outDir, 0755)

	cmd := exec.Command("./tool-gobible-creator", "run", "--request", reqFile, "--out", outDir)
	cmd.Dir = pluginDir(t)
	cmd.Run()

	transcriptPath := filepath.Join(outDir, "transcript.jsonl")
	if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
		t.Error("transcript.jsonl not created")
	}
}

func buildPlugin(t *testing.T) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", "tool-gobible-creator", ".")
	cmd.Dir = pluginDir(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build: %v\n%s", err, output)
	}
}

func pluginDir(t *testing.T) string { return "." }

func hasGoBibleCreator() bool {
	// Check for gobiblecreator.jar in common locations
	paths := []string{
		"/usr/share/java/gobiblecreator.jar",
		"/opt/gobiblecreator/GoBibleCreator.jar",
		"gobiblecreator.jar",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}
