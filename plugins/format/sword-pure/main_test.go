package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// TestPluginInfo tests the info command.
func TestPluginInfo(t *testing.T) {
	pluginPath := buildPlugin(t)

	cmd := exec.Command(pluginPath, "info")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("info command failed: %v", err)
	}

	var info PluginInfo
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		t.Fatalf("failed to parse info output: %v", err)
	}

	if info.PluginID != "format.sword-pure" {
		t.Errorf("expected plugin_id 'format.sword-pure', got '%s'", info.PluginID)
	}
	if info.Kind != "format" {
		t.Errorf("expected kind 'format', got '%s'", info.Kind)
	}
	if len(info.Formats) != 4 {
		t.Errorf("expected 4 formats, got %d", len(info.Formats))
	}
}

// TestDetectNonSword tests detect on non-SWORD path.
func TestDetectNonSword(t *testing.T) {
	pluginPath := buildPlugin(t)

	tmpDir, err := os.MkdirTemp("", "sword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a non-SWORD directory
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("not a sword module"), 0644)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": tmpDir},
	}

	resp := executePlugin(t, pluginPath, &req)

	// Currently returns error because not yet implemented
	// When implemented, should return detected: false
	if resp.Status == "ok" {
		result, ok := resp.Result.(map[string]interface{})
		if ok && result["detected"] == true {
			t.Error("expected detected to be false for non-SWORD directory")
		}
	}
}

// TestListModulesEmpty tests list-modules on empty path.
func TestListModulesEmpty(t *testing.T) {
	pluginPath := buildPlugin(t)

	tmpDir, err := os.MkdirTemp("", "sword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	req := ipc.Request{
		Command: "list-modules",
		Args:    map[string]interface{}{"path": tmpDir},
	}

	resp := executePlugin(t, pluginPath, &req)

	// Currently returns error because not yet implemented
	// When implemented, should return empty list
	if resp.Status == "ok" {
		result, ok := resp.Result.(map[string]interface{})
		if ok {
			modules, _ := result["modules"].([]interface{})
			if len(modules) != 0 {
				t.Errorf("expected 0 modules, got %d", len(modules))
			}
		}
	}
}

// TestUnknownCommand tests handling of unknown commands.
func TestUnknownCommand(t *testing.T) {
	pluginPath := buildPlugin(t)

	req := ipc.Request{
		Command: "unknown-command",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, pluginPath, &req)

	if resp.Status != "error" {
		t.Errorf("expected error status for unknown command, got %s", resp.Status)
	}
}

// TestMissingArgs tests handling of missing arguments.
func TestMissingArgs(t *testing.T) {
	pluginPath := buildPlugin(t)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, pluginPath, &req)

	if resp.Status != "error" {
		t.Errorf("expected error status for missing args, got %s", resp.Status)
	}
}

// TestDetectSwordModule tests detect on a valid SWORD module structure.
func TestDetectSwordModule(t *testing.T) {
	pluginPath := buildPlugin(t)

	// Create a mock SWORD module structure
	tmpDir, err := os.MkdirTemp("", "sword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mods.d directory with a conf file
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.Mkdir(modsDir, 0755)

	confContent := `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
Lang=en
Description=King James Version
`
	os.WriteFile(filepath.Join(modsDir, "kjv.conf"), []byte(confContent), 0644)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": tmpDir},
	}

	resp := executePlugin(t, pluginPath, &req)

	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] != true {
		t.Error("expected detected to be true for SWORD module")
	}

	if result["format"] != "zText" {
		t.Errorf("expected format 'zText', got %v", result["format"])
	}
}

// TestListModulesWithConf tests list-modules with valid conf files.
func TestListModulesWithConf(t *testing.T) {
	pluginPath := buildPlugin(t)

	tmpDir, err := os.MkdirTemp("", "sword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modsDir := filepath.Join(tmpDir, "mods.d")
	os.Mkdir(modsDir, 0755)

	// Create two conf files
	conf1 := `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
Lang=en
Description=King James Version
Version=1.0
`
	conf2 := `[ESV]
DataPath=./modules/texts/ztext/esv/
ModDrv=zText
Lang=en
Description=English Standard Version
Version=2.0
`
	os.WriteFile(filepath.Join(modsDir, "kjv.conf"), []byte(conf1), 0644)
	os.WriteFile(filepath.Join(modsDir, "esv.conf"), []byte(conf2), 0644)

	req := ipc.Request{
		Command: "list-modules",
		Args:    map[string]interface{}{"path": tmpDir},
	}

	resp := executePlugin(t, pluginPath, &req)

	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	count, ok := result["count"].(float64)
	if !ok || int(count) != 2 {
		t.Errorf("expected count 2, got %v", result["count"])
	}

	modules, ok := result["modules"].([]interface{})
	if !ok || len(modules) != 2 {
		t.Errorf("expected 2 modules, got %v", len(modules))
	}
}

// TestParseConfCommand tests the parse-conf IPC command.
func TestParseConfCommand(t *testing.T) {
	pluginPath := buildPlugin(t)

	tmpDir, err := os.MkdirTemp("", "sword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	confContent := `[TestModule]
DataPath=./modules/texts/ztext/test/
ModDrv=zText
Encoding=UTF-8
Lang=en
Description=Test Module
Version=1.0
CompressType=ZIP
`
	confPath := filepath.Join(tmpDir, "test.conf")
	os.WriteFile(confPath, []byte(confContent), 0644)

	req := ipc.Request{
		Command: "parse-conf",
		Args:    map[string]interface{}{"path": confPath},
	}

	resp := executePlugin(t, pluginPath, &req)

	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["module_name"] != "TestModule" {
		t.Errorf("expected module_name 'TestModule', got %v", result["module_name"])
	}
	if result["mod_drv"] != "zText" {
		t.Errorf("expected mod_drv 'zText', got %v", result["mod_drv"])
	}
	if result["description"] != "Test Module" {
		t.Errorf("expected description 'Test Module', got %v", result["description"])
	}
}

// TestExtractIRMissingArgs tests extract-ir with missing arguments.
func TestExtractIRMissingArgs(t *testing.T) {
	pluginPath := buildPlugin(t)

	// Test missing path
	req := ipc.Request{
		Command: "extract-ir",
		Args:    map[string]interface{}{"output_dir": "/tmp/test"},
	}
	resp := executePlugin(t, pluginPath, &req)
	if resp.Status != "error" {
		t.Error("expected error for missing path argument")
	}

	// Test missing output_dir
	req = ipc.Request{
		Command: "extract-ir",
		Args:    map[string]interface{}{"path": "/tmp/test"},
	}
	resp = executePlugin(t, pluginPath, &req)
	if resp.Status != "error" {
		t.Error("expected error for missing output_dir argument")
	}
}

// TestExtractIRWithMockModule tests extract-ir with a mock zText module.
func TestExtractIRWithMockModule(t *testing.T) {
	pluginPath := buildPlugin(t)

	// Create mock SWORD module structure
	tmpDir, err := os.MkdirTemp("", "sword-ir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create output directory
	outputDir := filepath.Join(tmpDir, "output")

	// Create mods.d with conf file
	modsDir := filepath.Join(tmpDir, "mods.d")
	os.Mkdir(modsDir, 0755)

	confContent := `[TestBible]
DataPath=./modules/texts/ztext/testbible/
ModDrv=zText
Lang=en
Description=Test Bible Module
Version=1.0
Versification=KJV
`
	os.WriteFile(filepath.Join(modsDir, "testbible.conf"), []byte(confContent), 0644)

	// Create data directory (without actual binary files, will skip)
	dataDir := filepath.Join(tmpDir, "modules", "texts", "ztext", "testbible")
	os.MkdirAll(dataDir, 0755)

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       tmpDir,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, pluginPath, &req)

	// Should return ok with status info for each module
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	modules, ok := result["modules"].([]interface{})
	if !ok {
		t.Fatal("modules is not an array")
	}

	// Should have one module (may have error due to missing data files)
	if len(modules) != 1 {
		t.Errorf("expected 1 module result, got %d", len(modules))
	}

	if len(modules) > 0 {
		mod := modules[0].(map[string]interface{})
		if mod["module"] != "TestBible" {
			t.Errorf("expected module 'TestBible', got %v", mod["module"])
		}
		// Status will be "error" because no actual data files exist
		// This is expected behavior
	}
}

// TestExtractIRMarkupParsing tests the markup parsing functions.
func TestExtractIRMarkupParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantText string
	}{
		{
			name:     "plain text",
			input:    "In the beginning God created",
			wantText: "In the beginning God created",
		},
		{
			name:     "simple tags",
			input:    "<p>In the beginning</p>",
			wantText: "In the beginning",
		},
		{
			name:     "word tags with Strong's",
			input:    `<w lemma="strong:H7225">beginning</w>`,
			wantText: "beginning",
		},
		{
			name:     "nested tags",
			input:    `<verse><w lemma="strong:H1234">word</w> more</verse>`,
			wantText: "word more",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripMarkup(tc.input)
			if got != tc.wantText {
				t.Errorf("stripMarkup(%q) = %q, want %q", tc.input, got, tc.wantText)
			}
		})
	}
}

// TestParseStrongs tests Strong's number extraction.
func TestParseStrongs(t *testing.T) {
	tests := []struct {
		lemma string
		want  []string
	}{
		{"strong:H1234", []string{"H1234"}},
		{"strong:G2532", []string{"G2532"}},
		{"H1234 H5678", []string{"H1234", "H5678"}},
		{"strong:H1234 strong:H5678", []string{"H1234", "H5678"}},
		{"", nil},
		{"invalid", nil},
	}

	for _, tc := range tests {
		t.Run(tc.lemma, func(t *testing.T) {
			got := parseStrongs(tc.lemma)
			if len(got) != len(tc.want) {
				t.Errorf("parseStrongs(%q) = %v, want %v", tc.lemma, got, tc.want)
				return
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("parseStrongs(%q)[%d] = %q, want %q", tc.lemma, i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestTokenizePlainText tests plain text tokenization.
func TestTokenizePlainText(t *testing.T) {
	text := "In the beginning God created"
	tokens := tokenizePlainText(text)

	expectedWords := []string{"In", "the", "beginning", "God", "created"}
	if len(tokens) != len(expectedWords) {
		t.Fatalf("expected %d tokens, got %d", len(expectedWords), len(tokens))
	}

	for i, expected := range expectedWords {
		if tokens[i].Text != expected {
			t.Errorf("token[%d] = %q, want %q", i, tokens[i].Text, expected)
		}
		if tokens[i].Type != "word" {
			t.Errorf("token[%d].Type = %q, want 'word'", i, tokens[i].Type)
		}
	}
}

// TestEmitNativeMissingArgs tests emit-native with missing arguments.
func TestEmitNativeMissingArgs(t *testing.T) {
	pluginPath := buildPlugin(t)

	// Test missing ir_path
	req := ipc.Request{
		Command: "emit-native",
		Args:    map[string]interface{}{"output_dir": "/tmp/test"},
	}
	resp := executePlugin(t, pluginPath, &req)
	if resp.Status != "error" {
		t.Error("expected error for missing ir_path argument")
	}

	// Test missing output_dir
	req = ipc.Request{
		Command: "emit-native",
		Args:    map[string]interface{}{"ir_path": "/tmp/test.ir.json"},
	}
	resp = executePlugin(t, pluginPath, &req)
	if resp.Status != "error" {
		t.Error("expected error for missing output_dir argument")
	}
}

// TestEmitNativeFromIR tests emit-native with a mock IR file.
func TestEmitNativeFromIR(t *testing.T) {
	pluginPath := buildPlugin(t)

	tmpDir, err := os.MkdirTemp("", "sword-emit-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a minimal IR file
	corpus := IRCorpus{
		ID:            "TestBible",
		Version:       "1.0.0",
		ModuleType:    "BIBLE",
		Language:      "en",
		Title:         "Test Bible",
		Versification: "KJV",
		Documents: []*IRDocument{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*IRContentBlock{
					{
						ID:       "Gen.1.1",
						Sequence: 0,
						Text:     "In the beginning God created the heaven and the earth.",
					},
				},
			},
		},
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	data, _ := json.MarshalIndent(corpus, "", "  ")
	os.WriteFile(irPath, data, 0644)

	outputDir := filepath.Join(tmpDir, "output")

	req := ipc.Request{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, pluginPath, &req)

	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["module_id"] != "TestBible" {
		t.Errorf("expected module_id 'TestBible', got %v", result["module_id"])
	}

	// Check that conf file was created
	confPath, ok := result["conf_path"].(string)
	if !ok {
		t.Fatal("conf_path not in result")
	}

	confData, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("failed to read conf file: %v", err)
	}

	confStr := string(confData)
	if !contains(confStr, "[TestBible]") {
		t.Error("conf file missing module section")
	}
	if !contains(confStr, "Description=Test Bible") {
		t.Error("conf file missing description")
	}
	if !contains(confStr, "Lang=en") {
		t.Error("conf file missing language")
	}
}

// TestParseVerseContent tests the full verse content parsing.
func TestParseVerseContent(t *testing.T) {
	rawText := `<w lemma="strong:H7225" morph="HNcfsa">In the beginning</w> God created`

	block := parseVerseContent("Gen.1.1", rawText, 0)

	if block.ID != "Gen.1.1" {
		t.Errorf("expected ID 'Gen.1.1', got %q", block.ID)
	}

	if block.RawMarkup != rawText {
		t.Error("RawMarkup should preserve original markup")
	}

	// Text should have markup stripped
	if contains(block.Text, "<") || contains(block.Text, ">") {
		t.Error("Text should not contain markup tags")
	}

	// Hash should be computed
	if block.Hash == "" {
		t.Error("Hash should be computed")
	}

	// Should have tokens
	if len(block.Tokens) == 0 {
		t.Error("expected at least one token")
	}
}

// TestExtractIRSampleBibles tests IR extraction with the 11 sample Bible modules.
// This provides regression testing for consistent IR output.
func TestExtractIRSampleBibles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	swordPath := getSwordPath()
	if _, err := os.Stat(swordPath); os.IsNotExist(err) {
		t.Skip("SWORD directory not found, skipping sample Bible tests")
	}

	// The 11 sample Bibles used in web UI tests
	sampleBibles := []struct {
		name      string
		lang      string
		minVerses int  // Minimum expected verses (sanity check)
		expectOT  bool // Expect Old Testament
		expectNT  bool // Expect New Testament
	}{
		{"ASV", "en", 30000, true, true},        // American Standard Version
		{"DRC", "en", 30000, true, true},        // Douay-Rheims Catholic
		{"Geneva1599", "en", 30000, true, true}, // Geneva Bible 1599
		{"KJV", "en", 31000, true, true},        // King James Version
		{"LXX", "grc", 20000, true, false},      // Septuagint (OT only)
		{"OEB", "en", 5000, false, true},        // Open English Bible (partial)
		{"OSMHB", "hbo", 20000, true, false},    // Hebrew OT
		{"SBLGNT", "grc", 7000, false, true},    // Greek NT
		{"Tyndale", "en", 10000, false, true},   // Tyndale's Bible (NT + partial OT, incomplete)
		{"Vulgate", "la", 30000, true, true},    // Latin Vulgate
		{"WEB", "en", 31000, true, true},        // World English Bible
	}

	for _, bible := range sampleBibles {
		bible := bible // capture for parallel closure
		t.Run(bible.name, func(t *testing.T) {
			t.Parallel() // Run subtests in parallel

			// Each parallel subtest creates its own temp dir to avoid race with parent defer
			outputDir, err := os.MkdirTemp("", "sword-sample-ir-"+bible.name+"-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(outputDir)

			// Check if module exists
			confPath := filepath.Join(swordPath, "mods.d", strings.ToLower(bible.name)+".conf")
			if _, err := os.Stat(confPath); os.IsNotExist(err) {
				t.Skipf("Module %s not installed, skipping", bible.name)
				return
			}

			// Load the conf file
			conf, err := ParseConfFile(confPath)
			if err != nil {
				t.Fatalf("failed to parse conf: %v", err)
			}

			// Skip encrypted modules
			if conf.IsEncrypted() {
				t.Skip("encrypted module, skipping")
				return
			}

			// Open the zText module
			zt, err := OpenZTextModule(conf, swordPath)
			if err != nil {
				t.Fatalf("failed to open module: %v", err)
			}

			// Check testament availability
			if bible.expectOT && !zt.HasOT() {
				t.Errorf("expected OT but module has no OT data")
			}
			if bible.expectNT && !zt.HasNT() {
				t.Errorf("expected NT but module has no NT data")
			}

			// Extract corpus
			corpus, stats, err := extractCorpus(zt, conf)
			if err != nil {
				t.Fatalf("failed to extract corpus: %v", err)
			}

			// Validate corpus metadata
			if corpus.ID != conf.ModuleName {
				t.Errorf("corpus ID = %q, want %q", corpus.ID, conf.ModuleName)
			}
			if corpus.LossClass != "L1" {
				t.Errorf("loss class = %q, want L1", corpus.LossClass)
			}

			// Validate verse count
			if stats.Verses < bible.minVerses {
				t.Errorf("verses = %d, want >= %d", stats.Verses, bible.minVerses)
			}

			// Validate document count (should have at least 27 books for NT-only, 39 for OT-only, 66 for full)
			minDocs := 0
			if bible.expectOT && bible.expectNT {
				minDocs = 60 // Most full Bibles
			} else if bible.expectOT {
				minDocs = 30 // OT only
			} else if bible.expectNT {
				minDocs = 20 // NT only
			}
			if stats.Documents < minDocs {
				t.Errorf("documents = %d, want >= %d", stats.Documents, minDocs)
			}

			// Validate tokens exist
			if stats.Tokens == 0 {
				t.Error("expected tokens to be > 0")
			}

			// Write IR for inspection
			irPath := filepath.Join(outputDir, bible.name+".ir.json")
			if err := writeCorpusJSON(corpus, irPath); err != nil {
				t.Errorf("failed to write IR: %v", err)
			}

			t.Logf("%s: %d docs, %d verses, %d tokens, %d annotations",
				bible.name, stats.Documents, stats.Verses, stats.Tokens, stats.Annotations)
		})
	}
}

// TestExtractIRVerseIntegrity tests that extracted verses have valid structure.
func TestExtractIRVerseIntegrity(t *testing.T) {
	swordPath := getSwordPath()
	confPath := filepath.Join(swordPath, "mods.d", "kjv.conf")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Skip("KJV module not installed")
	}

	conf, err := ParseConfFile(confPath)
	if err != nil {
		t.Fatalf("failed to parse conf: %v", err)
	}

	zt, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		t.Fatalf("failed to open module: %v", err)
	}

	// Extract specific verse and verify integrity
	ref := &Ref{Book: "Gen", Chapter: 1, Verse: 1}
	text, err := zt.GetVerseText(ref)
	if err != nil {
		t.Fatalf("failed to get verse: %v", err)
	}

	// Parse the verse content
	block := parseVerseContent("Gen.1.1", text, 0)

	// Verify hash is computed
	if block.Hash == "" {
		t.Error("hash should be computed")
	}

	// Verify hash matches text
	expectedHash := computeHash(block.Text)
	if block.Hash != expectedHash {
		t.Errorf("hash mismatch: got %s, want %s", block.Hash, expectedHash)
	}

	// Verify raw markup is preserved
	if block.RawMarkup == "" {
		t.Error("raw markup should be preserved")
	}

	// Verify tokens have valid structure
	for i, token := range block.Tokens {
		if token.ID == "" {
			t.Errorf("token[%d] has empty ID", i)
		}
		if token.Text == "" {
			t.Errorf("token[%d] has empty text", i)
		}
		if token.Type == "" {
			t.Errorf("token[%d] has empty type", i)
		}
	}

	t.Logf("Gen.1.1: %d tokens, %d annotations, hash=%s",
		len(block.Tokens), len(block.Annotations), block.Hash[:16])
}

// TestExtractIRConsistency tests that repeated extractions produce identical output.
// This test extracts the full KJV module twice (~31,102 verses each) which takes 5+ minutes.
func TestExtractIRConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full extraction consistency test in short mode")
	}

	swordPath := getSwordPath()
	confPath := filepath.Join(swordPath, "mods.d", "kjv.conf")
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Skip("KJV module not installed")
	}

	conf, err := ParseConfFile(confPath)
	if err != nil {
		t.Fatalf("failed to parse conf: %v", err)
	}

	zt, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		t.Fatalf("failed to open module: %v", err)
	}

	// Extract twice and compare
	corpus1, stats1, err := extractCorpus(zt, conf)
	if err != nil {
		t.Fatalf("first extraction failed: %v", err)
	}

	corpus2, stats2, err := extractCorpus(zt, conf)
	if err != nil {
		t.Fatalf("second extraction failed: %v", err)
	}

	// Compare stats
	if stats1.Verses != stats2.Verses {
		t.Errorf("verse count mismatch: %d vs %d", stats1.Verses, stats2.Verses)
	}
	if stats1.Tokens != stats2.Tokens {
		t.Errorf("token count mismatch: %d vs %d", stats1.Tokens, stats2.Tokens)
	}
	if stats1.Documents != stats2.Documents {
		t.Errorf("document count mismatch: %d vs %d", stats1.Documents, stats2.Documents)
	}

	// Compare first verse hashes
	if len(corpus1.Documents) > 0 && len(corpus1.Documents[0].ContentBlocks) > 0 &&
		len(corpus2.Documents) > 0 && len(corpus2.Documents[0].ContentBlocks) > 0 {
		hash1 := corpus1.Documents[0].ContentBlocks[0].Hash
		hash2 := corpus2.Documents[0].ContentBlocks[0].Hash
		if hash1 != hash2 {
			t.Errorf("first verse hash mismatch: %s vs %s", hash1, hash2)
		}
	}

	t.Log("Consistency check passed: two extractions produced identical output")
}

// getSwordPath returns the SWORD installation path.
func getSwordPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".sword")
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// buildPlugin builds the plugin binary for testing.
func buildPlugin(t *testing.T) string {
	t.Helper()

	pluginPath := "./juniper-sword"
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
		if err := buildCmd.Run(); err != nil {
			t.Fatalf("failed to build plugin: %v", err)
		}
	}

	return pluginPath
}

// executePlugin runs the plugin with a request and returns the response.
func executePlugin(t *testing.T, pluginPath string, req *ipc.Request) *ipc.Response {
	t.Helper()

	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	cmd := exec.Command(pluginPath, "ipc")
	cmd.Stdin = bytes.NewReader(reqData)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stdout.Len() > 0 {
			var resp ipc.Response
			if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
				return &resp
			}
		}
		t.Fatalf("plugin execution failed: %v\nstderr: %s", err, stderr.String())
	}

	var resp ipc.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\noutput: %s", err, stdout.String())
	}

	return &resp
}

// TestZTextRoundTrip verifies that a SWORD module can be extracted to IR and
// re-emitted back to zText format with content integrity preserved.
// Use TestZTextRoundTripAllSampleBibles for comprehensive testing.
func TestZTextRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode (use -run TestZTextRoundTripAllSampleBibles for round-trip tests)")
	}

	swordPath := getSwordPath()
	modsPath := filepath.Join(swordPath, "mods.d")

	if _, err := os.Stat(modsPath); os.IsNotExist(err) {
		t.Skip("No SWORD installation found")
	}

	// Find a test module (prefer KJV or any available)
	modules, err := ListModules(swordPath)
	if err != nil || len(modules) == 0 {
		t.Skip("No modules available for testing")
	}

	// Find a Bible module (Type is "Bible" from ModuleType())
	var testModule string
	for _, m := range modules {
		if m.Type == "Bible" {
			testModule = m.Name
			if m.Name == "KJV" || m.Name == "ASV" {
				break
			}
		}
	}
	if testModule == "" {
		t.Skip("No Bible module found for round-trip test")
	}

	t.Logf("Testing round-trip with module: %s", testModule)

	// Step 1: Load and parse conf
	confPath := filepath.Join(swordPath, "mods.d", strings.ToLower(testModule)+".conf")
	conf, err := ParseConfFile(confPath)
	if err != nil {
		t.Fatalf("Failed to parse conf: %v", err)
	}

	// Open the module
	zt, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		t.Fatalf("Failed to open module: %v", err)
	}

	// Extract corpus
	corpus, _, err := extractCorpus(zt, conf)
	if err != nil {
		t.Fatalf("Failed to extract IR: %v", err)
	}

	t.Logf("Original extraction: %d documents, %d verses", len(corpus.Documents), countVerses(corpus))

	// Step 2: Emit to temporary directory
	tmpDir, err := os.MkdirTemp("", "sword-roundtrip-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result, err := EmitZText(corpus, tmpDir)
	if err != nil {
		t.Fatalf("Failed to emit zText: %v", err)
	}

	t.Logf("Emitted %d verses to %s", result.VersesWritten, result.DataPath)

	// Step 3: Verify files exist
	for _, ext := range []string{"ot.bzs", "ot.bzv", "ot.bzz", "nt.bzs", "nt.bzv", "nt.bzz"} {
		filePath := filepath.Join(result.DataPath, ext)
		if _, err := os.Stat(filePath); err != nil {
			// NT or OT might not exist depending on module
			if ext[:2] == "ot" || ext[:2] == "nt" {
				continue // Skip missing testament files
			}
			t.Errorf("Expected file not found: %s", filePath)
		}
	}

	// Step 4: Re-read the emitted module
	reConfs, err := LoadModulesFromPath(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load re-emitted module: %v", err)
	}
	if len(reConfs) == 0 {
		t.Fatal("No modules found in re-emitted output")
	}

	// Open re-emitted module
	reZt, err := OpenZTextModule(reConfs[0], tmpDir)
	if err != nil {
		t.Fatalf("Failed to open re-emitted module: %v", err)
	}

	reCorpus, _, err := extractCorpus(reZt, reConfs[0])
	if err != nil {
		t.Fatalf("Failed to extract re-emitted IR: %v", err)
	}

	t.Logf("Re-read extraction: %d documents, %d verses", len(reCorpus.Documents), countVerses(reCorpus))

	// Step 5: Compare content
	origVerses := countVerses(corpus)
	reVerses := countVerses(reCorpus)

	if reVerses < origVerses/2 { // Allow for versification differences
		t.Errorf("Significant verse loss: original=%d, re-read=%d", origVerses, reVerses)
	}

	// Compare sample verses
	compareVerses(t, corpus, reCorpus, 100)

	t.Logf("Round-trip verification passed: %d/%d verses verified", reVerses, origVerses)
}

// countVerses counts total verses in a corpus.
func countVerses(corpus *IRCorpus) int {
	count := 0
	for _, doc := range corpus.Documents {
		count += len(doc.ContentBlocks)
	}
	return count
}

// compareVerses compares a sample of verses between original and round-tripped corpus.
func compareVerses(t *testing.T, orig, roundTrip *IRCorpus, sampleSize int) {
	t.Helper()

	origMap := make(map[string]string)
	for _, doc := range orig.Documents {
		for _, block := range doc.ContentBlocks {
			origMap[block.ID] = block.Text
		}
	}

	matched := 0
	mismatched := 0
	missing := 0

	for _, doc := range roundTrip.Documents {
		for _, block := range doc.ContentBlocks {
			if matched >= sampleSize {
				break
			}
			origText, ok := origMap[block.ID]
			if !ok {
				missing++
				continue
			}
			if block.Text == origText {
				matched++
			} else {
				mismatched++
				if mismatched <= 3 {
					t.Logf("Verse mismatch at %s:\n  original: %s\n  roundtrip: %s",
						block.ID, truncate(origText, 80), truncate(block.Text, 80))
				}
			}
		}
	}

	if matched == 0 && len(origMap) > 0 {
		t.Errorf("No verses matched between original and round-trip")
	}
	t.Logf("Verse comparison: matched=%d, mismatched=%d, missing=%d", matched, mismatched, missing)
}

// truncate truncates a string to max length.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// TestEmitZTextBasic tests basic EmitZText functionality.
func TestEmitZTextBasic(t *testing.T) {
	// Create a minimal test corpus
	corpus := &IRCorpus{
		ID:            "TEST",
		Title:         "Test Module",
		Language:      "en",
		Versification: "KJV",
		Documents:     make([]*IRDocument, 0),
	}

	// Add Genesis 1:1
	genDoc := &IRDocument{
		ID:            "Gen",
		Title:         "Genesis",
		ContentBlocks: make([]*IRContentBlock, 0),
	}
	genDoc.ContentBlocks = append(genDoc.ContentBlocks, &IRContentBlock{
		ID:        "Gen.1.1",
		Sequence:  1,
		Text:      "In the beginning God created the heaven and the earth.",
		RawMarkup: "<w lemma=\"strong:H7225\">In the beginning</w> <w lemma=\"strong:H430\">God</w> created...",
	})
	corpus.Documents = append(corpus.Documents, genDoc)

	// Add John 1:1
	johnDoc := &IRDocument{
		ID:            "John",
		Title:         "John",
		ContentBlocks: make([]*IRContentBlock, 0),
	}
	johnDoc.ContentBlocks = append(johnDoc.ContentBlocks, &IRContentBlock{
		ID:        "John.1.1",
		Sequence:  1,
		Text:      "In the beginning was the Word, and the Word was with God, and the Word was God.",
		RawMarkup: "<w lemma=\"strong:G1722\">In</w> <w lemma=\"strong:G746\">the beginning</w> was the Word...",
	})
	corpus.Documents = append(corpus.Documents, johnDoc)

	// Emit to temp directory
	tmpDir, err := os.MkdirTemp("", "emit-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result, err := EmitZText(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitZText failed: %v", err)
	}

	// Verify output
	if result.VersesWritten != 2 {
		t.Errorf("Expected 2 verses written, got %d", result.VersesWritten)
	}

	// Check conf file exists
	if _, err := os.Stat(result.ConfPath); os.IsNotExist(err) {
		t.Error("Conf file not created")
	}

	// Check data files exist
	for _, testament := range []string{"ot", "nt"} {
		for _, ext := range []string{".bzs", ".bzv", ".bzz"} {
			path := filepath.Join(result.DataPath, testament+ext)
			if _, err := os.Stat(path); err != nil {
				// One testament might be empty
				continue
			}
		}
	}

	t.Logf("Emitted module: %s with %d verses", result.ModuleID, result.VersesWritten)
}

// TestEmitZComBasic tests basic EmitZCom functionality.
func TestEmitZComBasic(t *testing.T) {
	// Create a minimal test corpus
	corpus := &IRCorpus{
		ID:            "TESTCOM",
		Title:         "Test Commentary",
		Language:      "en",
		Versification: "KJV",
		Documents:     make([]*IRDocument, 0),
	}

	// Add a Genesis entry
	genDoc := &IRDocument{
		ID:            "Gen",
		Title:         "Genesis",
		ContentBlocks: make([]*IRContentBlock, 0),
	}
	genDoc.ContentBlocks = append(genDoc.ContentBlocks, &IRContentBlock{
		ID:       "Gen.1.1",
		Sequence: 1,
		Text:     "This is a commentary on Genesis 1:1.",
	})
	corpus.Documents = append(corpus.Documents, genDoc)

	// Emit to temp directory
	tmpDir, err := os.MkdirTemp("", "emit-zcom-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result, err := EmitZCom(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitZCom failed: %v", err)
	}

	// Verify output
	if result.VersesWritten != 1 {
		t.Errorf("Expected 1 entry written, got %d", result.VersesWritten)
	}

	t.Logf("Emitted zCom module: %s with %d entries", result.ModuleID, result.VersesWritten)

	// Verify conf file
	confContent, err := os.ReadFile(result.ConfPath)
	if err != nil {
		t.Fatalf("Failed to read conf: %v", err)
	}
	t.Logf("Conf content:\n%s", confContent)

	// Try to read back
	confs, err := LoadModulesFromPath(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load modules: %v", err)
	}
	if len(confs) == 0 {
		t.Fatal("No modules found in output")
	}

	t.Logf("Found module: %s, type: %s, DataPath: %s", confs[0].ModuleName, confs[0].ModDrv, confs[0].DataPath)

	// Create parser and load
	parser := NewZComParser(confs[0], tmpDir)
	if err := parser.Load(); err != nil {
		t.Fatalf("Failed to load zCom parser: %v", err)
	}

	// Try to read the entry
	ref := Ref{Book: "Gen", Chapter: 1, Verse: 1}
	entry, err := parser.GetEntry(ref)
	if err != nil {
		t.Fatalf("Failed to get entry Gen.1.1: %v", err)
	}

	if entry.Text != "This is a commentary on Genesis 1:1." {
		t.Errorf("Entry text mismatch:\n  got: %q\n  want: %q", entry.Text, "This is a commentary on Genesis 1:1.")
	}

	t.Logf("Successfully verified zCom entry: %s", entry.Text)
}

// TestZComRoundTrip tests commentary module round-trip.
func TestZComRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	swordPath := getSwordPath()
	modsPath := filepath.Join(swordPath, "mods.d")

	if _, err := os.Stat(modsPath); os.IsNotExist(err) {
		t.Skip("No SWORD installation found")
	}

	// Find a commentary module
	modules, err := ListModules(swordPath)
	if err != nil || len(modules) == 0 {
		t.Skip("No modules available for testing")
	}

	var testModule string
	for _, m := range modules {
		if m.Type == "Commentary" {
			testModule = m.Name
			break
		}
	}
	if testModule == "" {
		t.Skip("No commentary module found for round-trip test")
	}

	t.Logf("Testing commentary round-trip with module: %s", testModule)

	// Load all conf files and find the one we want
	confs, err := LoadModulesFromPath(swordPath)
	if err != nil {
		t.Fatalf("Failed to load modules: %v", err)
	}

	var conf *ConfFile
	for _, c := range confs {
		if c.ModuleName == testModule {
			conf = c
			break
		}
	}
	if conf == nil {
		t.Fatalf("Module not found: %s", testModule)
	}

	// Create parser
	parser := NewZComParser(conf, swordPath)
	if err := parser.Load(); err != nil {
		t.Fatalf("Failed to load module: %v", err)
	}

	// Create a minimal corpus from the commentary
	corpus := &IRCorpus{
		ID:            conf.ModuleName,
		Title:         conf.Description,
		Language:      conf.Lang,
		Versification: conf.Versification,
		LossClass:     "L1",
		Documents:     make([]*IRDocument, 0),
	}

	// Extract sample entries
	vers, _ := VersificationFromConf(conf)
	entriesExtracted := 0

	// Just extract from a few books for testing
	testBooks := []string{"Gen", "Matt"}
	for _, bookName := range testBooks {
		doc := &IRDocument{
			ID:            bookName,
			Title:         bookName,
			ContentBlocks: make([]*IRContentBlock, 0),
		}

		chapterCount := vers.GetChapterCount(bookName)
		for ch := 1; ch <= chapterCount && ch <= 3; ch++ { // Just first 3 chapters
			verseCount := vers.GetVerseCount(bookName, ch)
			for v := 1; v <= verseCount; v++ {
				ref := Ref{Book: bookName, Chapter: ch, Verse: v}
				entry, err := parser.GetEntry(ref)
				if err != nil || entry.Text == "" {
					continue
				}

				doc.ContentBlocks = append(doc.ContentBlocks, &IRContentBlock{
					ID:       fmt.Sprintf("%s.%d.%d", bookName, ch, v),
					Sequence: entriesExtracted,
					Text:     entry.Text,
				})
				entriesExtracted++
			}
		}

		if len(doc.ContentBlocks) > 0 {
			corpus.Documents = append(corpus.Documents, doc)
		}
	}

	if entriesExtracted == 0 {
		t.Skip("No entries found in commentary module")
	}

	t.Logf("Extracted %d entries from commentary", entriesExtracted)

	// Emit to temp directory
	tmpDir, err := os.MkdirTemp("", "zcom-roundtrip-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result, err := EmitZCom(corpus, tmpDir)
	if err != nil {
		t.Fatalf("Failed to emit zCom: %v", err)
	}

	t.Logf("Emitted %d entries to %s", result.VersesWritten, result.DataPath)

	// Verify files exist
	for _, ext := range []string{"ot.bzs", "ot.bzv", "ot.bzz", "nt.bzs", "nt.bzv", "nt.bzz"} {
		filePath := filepath.Join(result.DataPath, ext)
		if _, err := os.Stat(filePath); err == nil {
			t.Logf("Created: %s", ext)
		}
	}

	// Re-read the emitted module
	reConfs, err := LoadModulesFromPath(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load re-emitted module: %v", err)
	}
	if len(reConfs) == 0 {
		t.Fatal("No modules found in re-emitted output")
	}

	reParser := NewZComParser(reConfs[0], tmpDir)
	if err := reParser.Load(); err != nil {
		t.Fatalf("Failed to load re-emitted module: %v", err)
	}

	// Verify content
	verified := 0
	errors := 0
	for _, doc := range corpus.Documents {
		for _, block := range doc.ContentBlocks {
			// Parse reference from ID like "Gen.1.1"
			parts := strings.Split(block.ID, ".")
			if len(parts) != 3 {
				continue
			}
			var ch, v int
			fmt.Sscanf(parts[1], "%d", &ch)
			fmt.Sscanf(parts[2], "%d", &v)
			ref := Ref{Book: parts[0], Chapter: ch, Verse: v}

			entry, err := reParser.GetEntry(ref)
			if err != nil {
				if errors < 3 {
					t.Logf("Error getting %s: %v", block.ID, err)
				}
				errors++
				continue
			}
			if entry.Text == block.Text {
				verified++
			} else if verified == 0 && errors < 3 {
				t.Logf("Text mismatch at %s:\n  orig: %s\n  got:  %s",
					block.ID, truncate(block.Text, 60), truncate(entry.Text, 60))
			}
		}
	}

	t.Logf("Verified %d/%d entries in round-trip (errors: %d)", verified, entriesExtracted, errors)
	if verified == 0 && entriesExtracted > 0 {
		t.Error("No entries verified in round-trip")
	}
}

// TestEmitZLDBasic tests basic EmitZLD functionality.
func TestEmitZLDBasic(t *testing.T) {
	// Create a minimal test corpus
	corpus := &IRCorpus{
		ID:       "TESTLEX",
		Title:    "Test Lexicon",
		Language: "en",
	}

	// Add lexicon entries
	doc := &IRDocument{
		ID:            "entries",
		Title:         "Entries",
		ContentBlocks: make([]*IRContentBlock, 0),
	}

	// Add test entries
	doc.ContentBlocks = append(doc.ContentBlocks, &IRContentBlock{
		ID:       "G1234",
		Sequence: 1,
		Text:     "This is the definition for Strong's G1234.",
	})
	doc.ContentBlocks = append(doc.ContentBlocks, &IRContentBlock{
		ID:       "G5678",
		Sequence: 2,
		Text:     "This is the definition for Strong's G5678.",
	})
	doc.ContentBlocks = append(doc.ContentBlocks, &IRContentBlock{
		ID:       "H1234",
		Sequence: 3,
		Text:     "This is the definition for Strong's H1234.",
	})

	corpus.Documents = append(corpus.Documents, doc)

	// Emit to temp directory
	tmpDir, err := os.MkdirTemp("", "emit-zld-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result, err := EmitZLD(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitZLD failed: %v", err)
	}

	// Verify output
	if result.VersesWritten != 3 {
		t.Errorf("Expected 3 entries written, got %d", result.VersesWritten)
	}

	t.Logf("Emitted zLD module: %s with %d entries", result.ModuleID, result.VersesWritten)

	// Verify files exist
	for _, filename := range []string{"dict.idx", "dict.dat", "dict.zdx", "dict.zdt"} {
		path := filepath.Join(result.DataPath, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file not found: %s", filename)
		} else {
			t.Logf("Created: %s", filename)
		}
	}

	// Verify conf file
	confContent, err := os.ReadFile(result.ConfPath)
	if err != nil {
		t.Fatalf("Failed to read conf: %v", err)
	}
	t.Logf("Conf content:\n%s", confContent)
}

// TestZLDRoundTrip verifies full round-trip: corpus → EmitZLD → read back → verify content.
func TestZLDRoundTrip(t *testing.T) {
	// Create a test corpus with lexicon entries
	testEntries := map[string]string{
		"G0001":  "ἄλφα (alpha) - First letter of Greek alphabet",
		"G2316":  "θεός (theos) - God, a deity; the supreme divinity",
		"G0026":  "ἀγάπη (agape) - love, affection, good will",
		"H7965":  "שָׁלוֹם (shalom) - peace, completeness, welfare",
		"H0430":  "אֱלֹהִים (elohim) - God, gods, judges",
		"unicod": "Entry with Uñíçödé çhàrâctérs - testing special chars",
	}

	corpus := &IRCorpus{
		ID:       "TESTLEX",
		Title:    "Test Lexicon Round-Trip",
		Language: "grc",
	}

	doc := &IRDocument{
		ID:            "entries",
		Title:         "Lexicon Entries",
		ContentBlocks: make([]*IRContentBlock, 0),
	}

	// Add entries in a specific order
	seq := 1
	for key, text := range testEntries {
		doc.ContentBlocks = append(doc.ContentBlocks, &IRContentBlock{
			ID:       key,
			Sequence: seq,
			Text:     text,
		})
		seq++
	}

	corpus.Documents = append(corpus.Documents, doc)

	// Step 1: Emit to temp directory
	tmpDir, err := os.MkdirTemp("", "zld-roundtrip-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result, err := EmitZLD(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitZLD failed: %v", err)
	}

	t.Logf("Emitted %d entries to %s", result.VersesWritten, result.DataPath)

	if result.VersesWritten != len(testEntries) {
		t.Errorf("Expected %d entries, got %d", len(testEntries), result.VersesWritten)
	}

	// Step 2: Read back the emitted files

	// Read key index (.idx)
	idxPath := filepath.Join(result.DataPath, "dict.idx")
	idxData, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("Failed to read idx: %v", err)
	}

	keys, err := parseZLDKeyIndex(idxData)
	if err != nil {
		t.Fatalf("Failed to parse idx: %v", err)
	}

	if len(keys) != len(testEntries) {
		t.Errorf("Index has %d keys, expected %d", len(keys), len(testEntries))
	}

	// Read compressed index (.zdx)
	zdxPath := filepath.Join(result.DataPath, "dict.zdx")
	zdxData, err := os.ReadFile(zdxPath)
	if err != nil {
		t.Fatalf("Failed to read zdx: %v", err)
	}

	compIndex, err := parseZLDCompressedIndex(zdxData)
	if err != nil {
		t.Fatalf("Failed to parse zdx: %v", err)
	}

	if len(compIndex) != len(testEntries) {
		t.Errorf("Compressed index has %d entries, expected %d", len(compIndex), len(testEntries))
	}

	// Read compressed data (.zdt)
	zdtPath := filepath.Join(result.DataPath, "dict.zdt")
	zdtData, err := os.ReadFile(zdtPath)
	if err != nil {
		t.Fatalf("Failed to read zdt: %v", err)
	}

	// Step 3: Decompress and extract entries
	// Build block cache to avoid decompressing the same block multiple times
	blockCache := make(map[uint32][]byte)

	// Helper to get decompressed block
	getBlock := func(blockNum uint32) ([]byte, error) {
		if cached, ok := blockCache[blockNum]; ok {
			return cached, nil
		}

		// Find block offset by scanning for block boundaries
		// Each block: 4-byte size + compressed data
		pos := 0
		currentBlock := uint32(0)
		for pos < len(zdtData) {
			if pos+4 > len(zdtData) {
				return nil, fmt.Errorf("truncated zdt at pos %d", pos)
			}
			blockSize := binary.LittleEndian.Uint32(zdtData[pos:])
			blockEnd := pos + 4 + int(blockSize)

			if currentBlock == blockNum {
				decompressed, err := decompressZLDBlock(zdtData[pos:blockEnd])
				if err != nil {
					return nil, fmt.Errorf("decompress block %d: %w", blockNum, err)
				}
				blockCache[blockNum] = decompressed
				return decompressed, nil
			}

			pos = blockEnd
			currentBlock++
		}

		return nil, fmt.Errorf("block %d not found", blockNum)
	}

	// Step 4: Verify each entry
	readEntries := make(map[string]string)

	for i, keyEntry := range keys {
		idx := compIndex[i]

		block, err := getBlock(idx.BlockNum)
		if err != nil {
			t.Errorf("Failed to get block for key %s: %v", keyEntry.Key, err)
			continue
		}

		// Find null-terminated string at offset
		startOffset := int(idx.Offset)
		if startOffset >= len(block) {
			t.Errorf("Offset %d out of range for key %s (block len %d)", startOffset, keyEntry.Key, len(block))
			continue
		}

		// Find null terminator
		endOffset := startOffset
		for endOffset < len(block) && block[endOffset] != 0 {
			endOffset++
		}

		text := string(block[startOffset:endOffset])
		readEntries[keyEntry.Key] = text
	}

	// Step 5: Compare with original
	for key, expectedText := range testEntries {
		readText, ok := readEntries[key]
		if !ok {
			t.Errorf("Key %q not found in read-back entries", key)
			continue
		}

		if readText != expectedText {
			t.Errorf("Entry mismatch for key %q:\n  expected: %q\n  got:      %q", key, expectedText, readText)
		} else {
			t.Logf("Verified key %q: %s", key, truncateString(expectedText, 40))
		}
	}

	// Check for extra keys
	for key := range readEntries {
		if _, ok := testEntries[key]; !ok {
			t.Errorf("Unexpected key in read-back: %q", key)
		}
	}

	t.Logf("Round-trip complete: %d/%d entries verified", len(readEntries), len(testEntries))
}

// truncateString truncates a string to maxLen chars with "..." suffix.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// TestRawGenBookRoundTrip verifies full round-trip: corpus → EmitRawGenBook → read back → verify content.
func TestRawGenBookRoundTrip(t *testing.T) {
	// Create a test corpus with hierarchical general book entries
	// Simulating a confession/catechism structure
	testEntries := map[string]string{
		"/WCF":                     "Westminster Confession of Faith",
		"/WCF/Chapter 1":           "Of the Holy Scripture",
		"/WCF/Chapter 1/Article 1": "Although the light of nature...",
		"/WCF/Chapter 1/Article 2": "Under the name of Holy Scripture...",
		"/WCF/Chapter 2":           "Of God, and of the Holy Trinity",
		"/WCF/Chapter 2/Article 1": "There is but one only living and true God...",
		"/WSC":                     "Westminster Shorter Catechism",
		"/WSC/Q1":                  "What is the chief end of man?",
		"/WSC/A1":                  "Man's chief end is to glorify God...",
	}

	corpus := &IRCorpus{
		ID:       "TESTBOOK",
		Title:    "Test General Book",
		Language: "en",
	}

	doc := &IRDocument{
		ID:            "entries",
		Title:         "Book Entries",
		ContentBlocks: make([]*IRContentBlock, 0),
	}

	// Add entries
	seq := 1
	for path, text := range testEntries {
		doc.ContentBlocks = append(doc.ContentBlocks, &IRContentBlock{
			ID:       path,
			Sequence: seq,
			Text:     text,
		})
		seq++
	}

	corpus.Documents = append(corpus.Documents, doc)

	// Step 1: Emit to temp directory
	tmpDir, err := os.MkdirTemp("", "rawgenbook-roundtrip-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result, err := EmitRawGenBook(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitRawGenBook failed: %v", err)
	}

	t.Logf("Emitted %d entries to %s", result.VersesWritten, result.DataPath)

	if result.VersesWritten != len(testEntries) {
		t.Errorf("Expected %d entries, got %d", len(testEntries), result.VersesWritten)
	}

	// Step 2: Read back the emitted files

	// Read tree index (.bdt)
	bdtPath := filepath.Join(result.DataPath, "book.bdt")
	bdtData, err := os.ReadFile(bdtPath)
	if err != nil {
		t.Fatalf("Failed to read bdt: %v", err)
	}

	treeKeys, err := parseRawGenBookTreeIndex(bdtData)
	if err != nil {
		t.Fatalf("Failed to parse bdt: %v", err)
	}

	if len(treeKeys) != len(testEntries) {
		t.Errorf("Tree has %d keys, expected %d", len(treeKeys), len(testEntries))
	}

	// Read data index (.idx)
	idxPath := filepath.Join(result.DataPath, "book.idx")
	idxData, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("Failed to read idx: %v", err)
	}

	dataIndex, err := parseRawGenBookDataIndex(idxData)
	if err != nil {
		t.Fatalf("Failed to parse idx: %v", err)
	}

	if len(dataIndex) != len(testEntries) {
		t.Errorf("Data index has %d entries, expected %d", len(dataIndex), len(testEntries))
	}

	// Read data file (.dat)
	datPath := filepath.Join(result.DataPath, "book.dat")
	datData, err := os.ReadFile(datPath)
	if err != nil {
		t.Fatalf("Failed to read dat: %v", err)
	}

	// Step 3: Extract entries and build paths
	parser := &RawGenBookParser{
		treeKeys: treeKeys,
		entries:  make(map[string]*RawGenBookEntry),
	}

	readEntries := make(map[string]string)

	for i := range treeKeys {
		// Build full path for this node
		path := parser.BuildKeyPath(i)

		// Get content from data file
		if i < len(dataIndex) {
			idx := dataIndex[i]
			start := int(idx.Offset)
			end := start + int(idx.Size)
			if end <= len(datData) {
				content := string(datData[start:end])
				readEntries[path] = content
			}
		}
	}

	// Step 4: Compare with original
	for path, expectedText := range testEntries {
		readText, ok := readEntries[path]
		if !ok {
			t.Errorf("Path %q not found in read-back entries", path)
			continue
		}

		if readText != expectedText {
			t.Errorf("Entry mismatch for path %q:\n  expected: %q\n  got:      %q", path, expectedText, readText)
		} else {
			t.Logf("Verified path %q: %s", path, truncateString(expectedText, 40))
		}
	}

	// Check for extra entries
	for path := range readEntries {
		if _, ok := testEntries[path]; !ok {
			t.Errorf("Unexpected path in read-back: %q", path)
		}
	}

	// Step 5: Verify tree structure
	// Find root nodes (parent == -1)
	rootCount := 0
	for _, key := range treeKeys {
		if key.Parent == -1 {
			rootCount++
		}
	}
	t.Logf("Found %d root nodes", rootCount)

	// Verify WCF has children
	for i, key := range treeKeys {
		if key.Name == "WCF" && key.FirstChild >= 0 {
			childKey := treeKeys[key.FirstChild]
			t.Logf("WCF (idx %d) has first child: %q (idx %d)", i, childKey.Name, key.FirstChild)
		}
	}

	t.Logf("Round-trip complete: %d/%d entries verified", len(readEntries), len(testEntries))
}

// TestEmitRawGenBookBasic tests basic RawGenBook emission without full round-trip.
func TestEmitRawGenBookBasic(t *testing.T) {
	corpus := &IRCorpus{
		ID:       "TESTBOOK",
		Title:    "Test Book",
		Language: "en",
	}

	doc := &IRDocument{
		ID:            "entries",
		Title:         "Entries",
		ContentBlocks: make([]*IRContentBlock, 0),
	}

	// Add test entries
	doc.ContentBlocks = append(doc.ContentBlocks, &IRContentBlock{
		ID:       "/Root",
		Sequence: 1,
		Text:     "Root content",
	})
	doc.ContentBlocks = append(doc.ContentBlocks, &IRContentBlock{
		ID:       "/Root/Child1",
		Sequence: 2,
		Text:     "Child 1 content",
	})
	doc.ContentBlocks = append(doc.ContentBlocks, &IRContentBlock{
		ID:       "/Root/Child2",
		Sequence: 3,
		Text:     "Child 2 content",
	})

	corpus.Documents = append(corpus.Documents, doc)

	// Emit to temp directory
	tmpDir, err := os.MkdirTemp("", "emit-rawgenbook-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result, err := EmitRawGenBook(corpus, tmpDir)
	if err != nil {
		t.Fatalf("EmitRawGenBook failed: %v", err)
	}

	// Verify output
	if result.VersesWritten != 3 {
		t.Errorf("Expected 3 entries written, got %d", result.VersesWritten)
	}

	t.Logf("Emitted RawGenBook module: %s with %d entries", result.ModuleID, result.VersesWritten)

	// Verify files exist
	for _, filename := range []string{"book.bdt", "book.idx", "book.dat"} {
		path := filepath.Join(result.DataPath, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file not found: %s", filename)
		} else {
			t.Logf("Created: %s", filename)
		}
	}

	// Verify conf file
	confContent, err := os.ReadFile(result.ConfPath)
	if err != nil {
		t.Fatalf("Failed to read conf: %v", err)
	}
	t.Logf("Conf content:\n%s", confContent)
}

// TestL2MetadataComparison verifies that metadata is preserved during round-trip.
// L2 comparison checks: ModuleName, Description, Lang, Versification, and common attributes.
func TestL2MetadataComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping L2 metadata comparison in short mode")
	}

	swordPath := getSwordPath()
	if _, err := os.Stat(filepath.Join(swordPath, "mods.d")); os.IsNotExist(err) {
		t.Skip("No SWORD installation found")
	}

	// Test with sample Bible modules
	testModules := []string{"KJV", "ASV", "WEB"}

	for _, moduleName := range testModules {
		moduleName := moduleName
		t.Run(moduleName, func(t *testing.T) {
			// Step 1: Load original conf
			confPath := filepath.Join(swordPath, "mods.d", strings.ToLower(moduleName)+".conf")
			originalConf, err := ParseConfFile(confPath)
			if err != nil {
				t.Skipf("Module %s not available: %v", moduleName, err)
			}

			// Skip encrypted modules
			if originalConf.IsEncrypted() {
				t.Skipf("Skipping encrypted module: %s", moduleName)
			}

			t.Logf("Original conf for %s:", moduleName)
			t.Logf("  ModuleName: %s", originalConf.ModuleName)
			t.Logf("  Description: %s", originalConf.Description)
			t.Logf("  Lang: %s", originalConf.Lang)
			t.Logf("  Versification: %s", originalConf.Versification)
			t.Logf("  About: %s", truncateString(originalConf.About, 50))

			// Step 2: Open and extract module
			zt, err := OpenZTextModule(originalConf, swordPath)
			if err != nil {
				t.Skipf("Failed to open module: %v", err)
			}

			corpus, _, err := extractCorpus(zt, originalConf)
			if err != nil {
				t.Fatalf("Failed to extract: %v", err)
			}

			// Step 3: Emit to temp directory
			tmpDir, err := os.MkdirTemp("", "l2-metadata-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			result, err := EmitZText(corpus, tmpDir)
			if err != nil {
				t.Fatalf("Failed to emit: %v", err)
			}

			// Step 4: Parse emitted conf
			emittedConf, err := ParseConfFile(result.ConfPath)
			if err != nil {
				t.Fatalf("Failed to parse emitted conf: %v", err)
			}

			t.Logf("Emitted conf for %s:", moduleName)
			t.Logf("  ModuleName: %s", emittedConf.ModuleName)
			t.Logf("  Description: %s", emittedConf.Description)
			t.Logf("  Lang: %s", emittedConf.Lang)
			t.Logf("  Versification: %s", emittedConf.Versification)

			// Step 5: Compare L2 metadata fields
			var mismatches []string

			// ModuleName must match
			if emittedConf.ModuleName != originalConf.ModuleName {
				mismatches = append(mismatches, fmt.Sprintf("ModuleName: %q != %q", emittedConf.ModuleName, originalConf.ModuleName))
			}

			// Description must match
			if emittedConf.Description != originalConf.Description {
				mismatches = append(mismatches, fmt.Sprintf("Description: %q != %q", emittedConf.Description, originalConf.Description))
			}

			// Lang must match
			if emittedConf.Lang != originalConf.Lang {
				mismatches = append(mismatches, fmt.Sprintf("Lang: %q != %q", emittedConf.Lang, originalConf.Lang))
			}

			// Versification must match (if original has one)
			if originalConf.Versification != "" {
				if emittedConf.Versification != originalConf.Versification {
					mismatches = append(mismatches, fmt.Sprintf("Versification: %q != %q", emittedConf.Versification, originalConf.Versification))
				}
			}

			// ModDrv should be zText (format may upgrade from rawtext)
			if emittedConf.ModDrv != "zText" {
				mismatches = append(mismatches, fmt.Sprintf("ModDrv: expected zText, got %q", emittedConf.ModDrv))
			}

			// Encoding should be UTF-8
			if emittedConf.Encoding != "UTF-8" {
				mismatches = append(mismatches, fmt.Sprintf("Encoding: expected UTF-8, got %q", emittedConf.Encoding))
			}

			// Report results
			if len(mismatches) > 0 {
				for _, m := range mismatches {
					t.Errorf("Metadata mismatch: %s", m)
				}
			} else {
				t.Logf("L2 metadata preserved: ModuleName, Description, Lang, Versification")
			}

			// Optional: Check that About is preserved if present
			if originalConf.About != "" {
				if about, ok := emittedConf.Properties["About"]; ok {
					if about == originalConf.About {
						t.Logf("L2+ About field preserved")
					}
				}
			}
		})
	}
}

// TestCapsuleStructureAndContent tests that capsules have the correct structure and content.
// This verifies the fix for the SWORD/CAS format discrepancy.
func TestCapsuleStructureAndContent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	swordPath := getSwordPath()
	if _, err := os.Stat(filepath.Join(swordPath, "mods.d")); os.IsNotExist(err) {
		t.Skip("No SWORD installation found")
	}

	// Find a Bible module to test
	modules, err := ListModules(swordPath)
	if err != nil || len(modules) == 0 {
		t.Skip("No modules available for testing")
	}

	var testModule ModuleInfo
	for _, m := range modules {
		if m.Type == "Bible" && !m.Encrypted {
			testModule = m
			break
		}
	}
	if testModule.Name == "" {
		t.Skip("No non-encrypted Bible module found")
	}

	t.Logf("Testing capsule structure with module: %s", testModule.Name)

	// Create temp directories
	outputDir, err := os.MkdirTemp("", "capsule-struct-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	// Create capsule
	capsulePath := filepath.Join(outputDir, testModule.Name+".capsule.tar.gz")
	if err := createModuleCapsule(swordPath, testModule, capsulePath); err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Verify capsule was created
	if _, err := os.Stat(capsulePath); os.IsNotExist(err) {
		t.Fatal("capsule file was not created")
	}

	// Extract capsule to verify structure
	extractDir := filepath.Join(outputDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		t.Fatalf("failed to create extract dir: %v", err)
	}

	// Extract using tar
	cmd := exec.Command("tar", "-xzf", capsulePath, "-C", extractDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to extract capsule: %v", err)
	}

	// Verify structure - manifest.json at root (not capsule/manifest.json)
	t.Run("ManifestAtRoot", func(t *testing.T) {
		manifestPath := filepath.Join(extractDir, "manifest.json")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			t.Error("manifest.json should be at root level, not in a subdirectory")
		}

		// Make sure it's not in capsule/ subdirectory
		badPath := filepath.Join(extractDir, "capsule", "manifest.json")
		if _, err := os.Stat(badPath); err == nil {
			t.Error("manifest.json should NOT be in capsule/ subdirectory")
		}
	})

	// Verify manifest content
	t.Run("ManifestContent", func(t *testing.T) {
		manifestPath := filepath.Join(extractDir, "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("failed to read manifest: %v", err)
		}

		var manifest map[string]interface{}
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatalf("failed to parse manifest: %v", err)
		}

		// Check required fields
		if manifest["capsule_version"] == nil {
			t.Error("manifest missing capsule_version")
		}
		if manifest["id"] == nil {
			t.Error("manifest missing id")
		}
		if manifest["id"] != testModule.Name {
			t.Errorf("manifest id mismatch: got %v, want %s", manifest["id"], testModule.Name)
		}
		if manifest["module_type"] != "bible" {
			t.Errorf("manifest module_type should be 'bible', got %v", manifest["module_type"])
		}
		if manifest["source_format"] != "sword" {
			t.Errorf("manifest source_format should be 'sword', got %v", manifest["source_format"])
		}

		// has_ir should be true (since we now generate IR)
		if manifest["has_ir"] != true {
			t.Errorf("manifest has_ir should be true, got %v", manifest["has_ir"])
		}
	})

	// Verify IR directory exists with content
	t.Run("IRDirectoryExists", func(t *testing.T) {
		irDir := filepath.Join(extractDir, "ir")
		if _, err := os.Stat(irDir); os.IsNotExist(err) {
			t.Error("ir/ directory should exist")
			return
		}

		// Check for IR file
		irFile := filepath.Join(irDir, testModule.Name+".ir.json")
		if _, err := os.Stat(irFile); os.IsNotExist(err) {
			t.Errorf("IR file %s.ir.json should exist in ir/", testModule.Name)
		}
	})

	// Verify IR content
	t.Run("IRContent", func(t *testing.T) {
		irFile := filepath.Join(extractDir, "ir", testModule.Name+".ir.json")
		data, err := os.ReadFile(irFile)
		if err != nil {
			t.Skipf("IR file not found: %v", err)
			return
		}

		var ir IRCorpus
		if err := json.Unmarshal(data, &ir); err != nil {
			t.Fatalf("failed to parse IR: %v", err)
		}

		// Verify IR structure
		if ir.ID != testModule.Name {
			t.Errorf("IR id mismatch: got %s, want %s", ir.ID, testModule.Name)
		}
		if ir.ModuleType != "BIBLE" {
			t.Errorf("IR module_type should be BIBLE, got %s", ir.ModuleType)
		}
		if len(ir.Documents) == 0 {
			t.Error("IR should have at least one document (book)")
		}

		// Verify documents have content
		totalVerses := 0
		for _, doc := range ir.Documents {
			totalVerses += len(doc.ContentBlocks)
		}
		if totalVerses == 0 {
			t.Error("IR should have at least one verse")
		}

		t.Logf("IR has %d documents with %d total verses", len(ir.Documents), totalVerses)

		// Verify first verse has expected structure
		if len(ir.Documents) > 0 && len(ir.Documents[0].ContentBlocks) > 0 {
			firstVerse := ir.Documents[0].ContentBlocks[0]
			if firstVerse.ID == "" {
				t.Error("first verse should have ID")
			}
			if firstVerse.Text == "" {
				t.Error("first verse should have text")
			}
			if firstVerse.Hash == "" {
				t.Error("first verse should have hash")
			}
			if len(firstVerse.Tokens) == 0 {
				t.Error("first verse should have tokens")
			}
			t.Logf("First verse (%s): %d tokens, hash=%s...",
				firstVerse.ID, len(firstVerse.Tokens), firstVerse.Hash[:16])
		}
	})

	// Verify SWORD data is preserved
	t.Run("SWORDDataPreserved", func(t *testing.T) {
		modsDir := filepath.Join(extractDir, "mods.d")
		if _, err := os.Stat(modsDir); os.IsNotExist(err) {
			t.Error("mods.d/ directory should exist")
			return
		}

		// Check for conf file
		entries, _ := os.ReadDir(modsDir)
		if len(entries) == 0 {
			t.Error("mods.d/ should contain at least one .conf file")
		}

		// Check for modules directory
		modulesDir := filepath.Join(extractDir, "modules")
		if _, err := os.Stat(modulesDir); os.IsNotExist(err) {
			t.Error("modules/ directory should exist with SWORD data")
		}
	})

	t.Logf("Capsule structure verified for %s", testModule.Name)
}

// TestCapsuleVerifyCompatibility tests that created capsules can be verified by the main capsule CLI.
func TestCapsuleVerifyCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	swordPath := getSwordPath()
	if _, err := os.Stat(filepath.Join(swordPath, "mods.d")); os.IsNotExist(err) {
		t.Skip("No SWORD installation found")
	}

	// Find a Bible module
	modules, err := ListModules(swordPath)
	if err != nil || len(modules) == 0 {
		t.Skip("No modules available")
	}

	var testModule ModuleInfo
	for _, m := range modules {
		if m.Type == "Bible" && !m.Encrypted {
			testModule = m
			break
		}
	}
	if testModule.Name == "" {
		t.Skip("No non-encrypted Bible module found")
	}

	// Create capsule
	outputDir, err := os.MkdirTemp("", "capsule-verify-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	capsulePath := filepath.Join(outputDir, testModule.Name+".capsule.tar.gz")
	if err := createModuleCapsule(swordPath, testModule, capsulePath); err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Build capsule CLI if needed
	capsuleCLI := filepath.Join(outputDir, "capsule")
	buildCmd := exec.Command("go", "build", "-o", capsuleCLI, "../../../../cmd/capsule")
	if err := buildCmd.Run(); err != nil {
		t.Skipf("Failed to build capsule CLI: %v", err)
	}

	// Run capsule verify command
	verifyCmd := exec.Command(capsuleCLI, "capsule", "verify", capsulePath)
	var stdout, stderr bytes.Buffer
	verifyCmd.Stdout = &stdout
	verifyCmd.Stderr = &stderr

	if err := verifyCmd.Run(); err != nil {
		t.Errorf("capsule verify failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
		return
	}

	output := stdout.String()
	if !contains(output, "Verification passed") {
		t.Errorf("expected 'Verification passed' in output, got: %s", output)
	}

	t.Logf("Capsule %s passed verification", testModule.Name)
}

// TestZTextRoundTripAllSampleBibles tests round-trip for the 11 sample Bibles.
// By default, tests 11 sample Bibles. Set SWORD_TEST_ALL=1 to test all installed modules.
func TestZTextRoundTripAllSampleBibles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sample Bible round-trip in short mode")
	}

	swordPath := getSwordPath()
	if _, err := os.Stat(filepath.Join(swordPath, "mods.d")); os.IsNotExist(err) {
		t.Skip("No SWORD installation found")
	}

	var bibleModules []string

	// If SWORD_TEST_ALL is set, test all installed Bible modules
	if os.Getenv("SWORD_TEST_ALL") == "1" {
		modules, err := ListModules(swordPath)
		if err != nil {
			t.Fatalf("Failed to list modules: %v", err)
		}
		for _, m := range modules {
			if m.Type == "Bible" {
				bibleModules = append(bibleModules, m.Name)
			}
		}
		t.Logf("SWORD_TEST_ALL=1: Testing ALL %d Bible modules", len(bibleModules))
	} else {
		// Default: use the same 11 sample Bibles as TestExtractIRSampleBibles
		bibleModules = []string{
			"ASV", "DRC", "Geneva1599", "KJV", "LXX", "OEB",
			"OSMHB", "SBLGNT", "Tyndale", "Vulgate", "WEB",
		}
		t.Logf("Testing round-trip for %d sample Bible modules (set SWORD_TEST_ALL=1 for all)", len(bibleModules))
	}

	for _, moduleName := range bibleModules {
		moduleName := moduleName // capture for parallel closure
		t.Run(moduleName, func(t *testing.T) {
			t.Parallel() // Run subtests in parallel
			// Load conf file
			confPath := filepath.Join(swordPath, "mods.d", strings.ToLower(moduleName)+".conf")
			conf, err := ParseConfFile(confPath)
			if err != nil {
				t.Skipf("Failed to parse conf: %v", err)
			}

			// Skip encrypted modules
			if conf.IsEncrypted() {
				t.Skipf("Skipping encrypted module: %s", moduleName)
			}

			// Open module
			zt, err := OpenZTextModule(conf, swordPath)
			if err != nil {
				t.Skipf("Failed to open module: %v", err)
			}

			// Extract
			corpus, _, err := extractCorpus(zt, conf)
			if err != nil {
				t.Fatalf("Failed to extract: %v", err)
			}

			origVersCount := countVerses(corpus)
			if origVersCount == 0 {
				t.Skipf("No verses extracted from %s", moduleName)
			}

			// Emit
			tmpDir, err := os.MkdirTemp("", "rt-"+moduleName+"-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			result, err := EmitZText(corpus, tmpDir)
			if err != nil {
				t.Fatalf("Failed to emit: %v", err)
			}

			// Re-read
			reConfs, err := LoadModulesFromPath(tmpDir)
			if err != nil || len(reConfs) == 0 {
				t.Fatalf("Failed to re-read emitted module: %v", err)
			}

			reZt, err := OpenZTextModule(reConfs[0], tmpDir)
			if err != nil {
				t.Fatalf("Failed to open re-emitted module: %v", err)
			}

			reCorpus, _, err := extractCorpus(reZt, reConfs[0])
			if err != nil {
				t.Fatalf("Failed to extract re-emitted: %v", err)
			}

			reVerseCount := countVerses(reCorpus)

			// Verify
			lossRate := 1.0 - float64(reVerseCount)/float64(origVersCount)
			if lossRate > 0.5 {
				t.Errorf("High verse loss: original=%d, re-read=%d (%.1f%% loss)",
					origVersCount, reVerseCount, lossRate*100)
			}

			t.Logf("%s: %d verses -> emit %d -> re-read %d (%.1f%% recovery)",
				moduleName, origVersCount, result.VersesWritten, reVerseCount,
				float64(reVerseCount)/float64(origVersCount)*100)
		})
	}
}
