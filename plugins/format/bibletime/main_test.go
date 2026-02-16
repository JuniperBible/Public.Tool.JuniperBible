package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// Test helpers
func runCommand(t *testing.T, cmd string, args map[string]interface{}) *ipc.Response {
	t.Helper()

	req := &ipc.Request{
		Command: cmd,
		Args:    args,
	}

	// Create pipes for stdin/stdout
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	// Create request JSON
	reqJSON, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	// Write request to temp file and redirect stdin
	tmpIn, err := os.CreateTemp("", "test-stdin-")
	if err != nil {
		t.Fatalf("failed to create temp stdin: %v", err)
	}
	defer os.Remove(tmpIn.Name())

	if _, err := tmpIn.Write(reqJSON); err != nil {
		t.Fatalf("failed to write request: %v", err)
	}
	if _, err := tmpIn.Seek(0, 0); err != nil {
		t.Fatalf("failed to seek: %v", err)
	}
	os.Stdin = tmpIn

	// Capture stdout
	tmpOut, err := os.CreateTemp("", "test-stdout-")
	if err != nil {
		t.Fatalf("failed to create temp stdout: %v", err)
	}
	defer os.Remove(tmpOut.Name())
	os.Stdout = tmpOut

	// Run main
	main()

	// Read response
	if _, err := tmpOut.Seek(0, 0); err != nil {
		t.Fatalf("failed to seek output: %v", err)
	}

	var resp ipc.Response
	dec := json.NewDecoder(tmpOut)
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	return &resp
}

func TestDetect_XBELBookmarkFile(t *testing.T) {
	// Create temp .xbel file
	tmpDir := t.TempDir()
	xbelPath := filepath.Join(tmpDir, "bookmarks.xbel")

	xbelContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE xbel>
<xbel version="1.0">
  <info>
    <metadata owner="bibletime">
      <bookmarks>
        <bookmark key="Genesis 1:1">
          <title>Creation</title>
        </bookmark>
      </bookmarks>
    </metadata>
  </info>
</xbel>`

	if err := os.WriteFile(xbelPath, []byte(xbelContent), 0600); err != nil {
		t.Fatalf("failed to write xbel: %v", err)
	}

	resp := runCommand(t, "detect", map[string]interface{}{
		"path": xbelPath,
	})

	if resp.Status != "ok" {
		t.Fatalf("expected ok status, got: %s (%s)", resp.Status, resp.Error)
	}

	var result ipc.DetectResult
	resultJSON, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if !result.Detected {
		t.Errorf("expected detected=true, got false: %s", result.Reason)
	}

	if result.Format != "BIBLETIME" {
		t.Errorf("expected format=BIBLETIME, got: %s", result.Format)
	}
}

func TestDetect_SWORDModuleDirectory(t *testing.T) {
	// Create temp SWORD module structure
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	confPath := filepath.Join(modsDir, "kjv.conf")
	confContent := `[KJV]
Description=King James Version
ModulePath=./modules/texts/ztext/kjv/
`
	if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
		t.Fatalf("failed to write conf: %v", err)
	}

	resp := runCommand(t, "detect", map[string]interface{}{
		"path": tmpDir,
	})

	if resp.Status != "ok" {
		t.Fatalf("expected ok status, got: %s (%s)", resp.Status, resp.Error)
	}

	var result ipc.DetectResult
	resultJSON, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if !result.Detected {
		t.Errorf("expected detected=true, got false: %s", result.Reason)
	}

	if result.Format != "BIBLETIME" {
		t.Errorf("expected format=BIBLETIME, got: %s", result.Format)
	}
}

func TestDetect_NonBibleTimeFile(t *testing.T) {
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(txtPath, []byte("random content"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	resp := runCommand(t, "detect", map[string]interface{}{
		"path": txtPath,
	})

	if resp.Status != "ok" {
		t.Fatalf("expected ok status, got: %s (%s)", resp.Status, resp.Error)
	}

	var result ipc.DetectResult
	resultJSON, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result.Detected {
		t.Errorf("expected detected=false, got true")
	}
}

func TestIngest_XBELFile(t *testing.T) {
	tmpDir := t.TempDir()
	xbelPath := filepath.Join(tmpDir, "bookmarks.xbel")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	xbelContent := `<?xml version="1.0" encoding="UTF-8"?>
<xbel><info><metadata owner="bibletime"></metadata></info></xbel>`

	if err := os.WriteFile(xbelPath, []byte(xbelContent), 0600); err != nil {
		t.Fatalf("failed to write xbel: %v", err)
	}

	resp := runCommand(t, "ingest", map[string]interface{}{
		"path":       xbelPath,
		"output_dir": outputDir,
	})

	if resp.Status != "ok" {
		t.Fatalf("expected ok status, got: %s (%s)", resp.Status, resp.Error)
	}

	var result ipc.IngestResult
	resultJSON, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result.BlobSHA256 == "" {
		t.Errorf("expected non-empty blob hash")
	}

	if result.SizeBytes != int64(len(xbelContent)) {
		t.Errorf("expected size=%d, got: %d", len(xbelContent), result.SizeBytes)
	}

	if result.Metadata["format"] != "BIBLETIME" {
		t.Errorf("expected format=BIBLETIME, got: %s", result.Metadata["format"])
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); err != nil {
		t.Errorf("blob not found at %s: %v", blobPath, err)
	}
}

func TestEnumerate_ModuleDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	modsDir := filepath.Join(tmpDir, "mods.d")
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		t.Fatalf("failed to create mods.d: %v", err)
	}

	// Create multiple .conf files
	confFiles := []string{"kjv.conf", "asv.conf", "web.conf"}
	for _, name := range confFiles {
		confPath := filepath.Join(modsDir, name)
		if err := os.WriteFile(confPath, []byte("[Module]\n"), 0600); err != nil {
			t.Fatalf("failed to write conf: %v", err)
		}
	}

	resp := runCommand(t, "enumerate", map[string]interface{}{
		"path": tmpDir,
	})

	if resp.Status != "ok" {
		t.Fatalf("expected ok status, got: %s (%s)", resp.Status, resp.Error)
	}

	var result ipc.EnumerateResult
	resultJSON, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(result.Entries) != 3 {
		t.Errorf("expected 3 entries, got: %d", len(result.Entries))
	}

	// Verify all are .conf files
	for _, entry := range result.Entries {
		if !strings.HasSuffix(entry.Path, ".conf") {
			t.Errorf("expected .conf file, got: %s", entry.Path)
		}
	}
}

func TestExtractIR_CreatesCorpus(t *testing.T) {
	tmpDir := t.TempDir()
	xbelPath := filepath.Join(tmpDir, "bookmarks.xbel")
	outputDir := filepath.Join(tmpDir, "ir")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	xbelContent := `<?xml version="1.0" encoding="UTF-8"?><xbel></xbel>`
	if err := os.WriteFile(xbelPath, []byte(xbelContent), 0600); err != nil {
		t.Fatalf("failed to write xbel: %v", err)
	}

	resp := runCommand(t, "extract-ir", map[string]interface{}{
		"path":       xbelPath,
		"output_dir": outputDir,
	})

	if resp.Status != "ok" {
		t.Fatalf("expected ok status, got: %s (%s)", resp.Status, resp.Error)
	}

	var result ipc.ExtractIRResult
	resultJSON, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result.LossClass != "L1" {
		t.Errorf("expected loss_class=L1, got: %s", result.LossClass)
	}

	// Verify IR file was created
	if _, err := os.Stat(result.IRPath); err != nil {
		t.Errorf("IR file not found at %s: %v", result.IRPath, err)
	}

	// Read and verify IR content
	data, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatalf("failed to read IR: %v", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("failed to unmarshal corpus: %v", err)
	}

	if corpus.SourceFormat != "BIBLETIME" {
		t.Errorf("expected source_format=BIBLETIME, got: %s", corpus.SourceFormat)
	}
}

func TestEmitNative_CreatesModuleStructure(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "corpus.json")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	// Create minimal corpus
	corpus := &ipc.Corpus{
		ID:           "test-module",
		Version:      "1.0",
		ModuleType:   "Bible",
		Title:        "Test Bible",
		SourceFormat: "BIBLETIME",
	}

	data, err := json.Marshal(corpus)
	if err != nil {
		t.Fatalf("failed to marshal corpus: %v", err)
	}

	if err := os.WriteFile(irPath, data, 0600); err != nil {
		t.Fatalf("failed to write IR: %v", err)
	}

	resp := runCommand(t, "emit-native", map[string]interface{}{
		"ir_path":    irPath,
		"output_dir": outputDir,
	})

	if resp.Status != "ok" {
		t.Fatalf("expected ok status, got: %s (%s)", resp.Status, resp.Error)
	}

	var result ipc.EmitNativeResult
	resultJSON, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result.Format != "BIBLETIME" {
		t.Errorf("expected format=BIBLETIME, got: %s", result.Format)
	}

	if result.LossClass != "L1" {
		t.Errorf("expected loss_class=L1, got: %s", result.LossClass)
	}

	// Verify module structure
	modsDir := filepath.Join(result.OutputPath, "mods.d")
	if _, err := os.Stat(modsDir); err != nil {
		t.Errorf("mods.d directory not found: %v", err)
	}

	// Verify .conf file
	confPath := filepath.Join(modsDir, "module.conf")
	if _, err := os.Stat(confPath); err != nil {
		t.Errorf("module.conf not found: %v", err)
	}

	// Read and verify conf content
	confData, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("failed to read conf: %v", err)
	}

	confStr := string(confData)
	if !strings.Contains(confStr, corpus.ID) {
		t.Errorf("conf should contain module ID %s", corpus.ID)
	}
	if !strings.Contains(confStr, corpus.Title) {
		t.Errorf("conf should contain module title %s", corpus.Title)
	}
}

func TestDetect_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	resp := runCommand(t, "detect", map[string]interface{}{
		"path": tmpDir,
	})

	if resp.Status != "ok" {
		t.Fatalf("expected ok status, got: %s (%s)", resp.Status, resp.Error)
	}

	var result ipc.DetectResult
	resultJSON, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result.Detected {
		t.Errorf("expected detected=false for empty directory")
	}
}

// Note: Error path tests removed as they cause process exit via ipc.RespondError
// which is expected behavior for plugin error handling
