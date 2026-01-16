package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// createTestBible creates a minimal e-Sword .bblx database for testing.
func createTestBible(t *testing.T, path string) {
	t.Helper()

	db, err := sql.Open(sqlite.DriverName(), path)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	defer db.Close()

	// Create Bible table
	_, err = db.Exec(`CREATE TABLE Bible (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)`)
	if err != nil {
		t.Fatalf("failed to create Bible table: %v", err)
	}

	// Create Details table
	_, err = db.Exec(`CREATE TABLE Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version TEXT, Font TEXT, RightToLeft INTEGER)`)
	if err != nil {
		t.Fatalf("failed to create Details table: %v", err)
	}

	// Insert test data - Genesis 1:1-2
	_, err = db.Exec(`INSERT INTO Bible (Book, Chapter, Verse, Scripture) VALUES
		(1, 1, 1, 'In the beginning God created the heaven and the earth.'),
		(1, 1, 2, 'And the earth was without form, and void.')`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Insert metadata
	_, err = db.Exec(`INSERT INTO Details (Title, Abbreviation, Information, Version, RightToLeft) VALUES
		('Test Bible', 'TB', 'A test Bible for unit testing', '1.0', 0)`)
	if err != nil {
		t.Fatalf("failed to insert metadata: %v", err)
	}
}

// TestESwordDetect tests the detect command.
func TestESwordDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "esword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	bblxPath := filepath.Join(tmpDir, "test.bblx")
	createTestBible(t, bblxPath)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": bblxPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] != true {
		t.Error("expected detected to be true")
	}
	if result["format"] != "e-Sword" {
		t.Errorf("expected format e-Sword, got %v", result["format"])
	}
}

// TestESwordDetectNonESword tests detect command on non-e-Sword file.
func TestESwordDetectNonESword(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "esword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	txtPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtPath, []byte("Hello world"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": txtPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] == true {
		t.Error("expected detected to be false for non-e-Sword file")
	}
}

// TestESwordExtractIR tests the extract-ir command.
func TestESwordExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "esword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	bblxPath := filepath.Join(tmpDir, "test.bblx")
	createTestBible(t, bblxPath)

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       bblxPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["loss_class"] != "L1" {
		t.Errorf("expected loss_class L1, got %v", result["loss_class"])
	}

	irPath, ok := result["ir_path"].(string)
	if !ok {
		t.Fatal("ir_path is not a string")
	}

	irData, err := os.ReadFile(irPath)
	if err != nil {
		t.Fatalf("failed to read IR file: %v", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	if corpus.ID != "test" {
		t.Errorf("expected ID test, got %s", corpus.ID)
	}
	if corpus.Title != "Test Bible" {
		t.Errorf("expected title Test Bible, got %s", corpus.Title)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("expected 2 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

// TestESwordEmitNative tests the emit-native command.
func TestESwordEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "esword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create IR corpus
	corpus := ipc.Corpus{
		ID:         "test",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Test Bible",
		Documents: []*ipc.Document{
			{
				ID:         "Gen",
				Title:      "Genesis",
				Order:      1,
				Attributes: map[string]string{"book_num": "1"},
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "In the beginning.",
						Anchors: []*ipc.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*ipc.Span{
									{
										ID:            "s-Gen.1.1",
										Type:          "VERSE",
										StartAnchorID: "a-1-0",
										Ref: &ipc.Ref{
											Book:    "Gen",
											Chapter: 1,
											Verse:   1,
											OSISID:  "Gen.1.1",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	irData, err := json.MarshalIndent(&corpus, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal IR: %v", err)
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["format"] != "e-Sword" {
		t.Errorf("expected format e-Sword, got %v", result["format"])
	}

	outputPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	// Verify the output is a valid SQLite database with expected data
	db, err := sql.Open(sqlite.DriverName(), outputPath+"?mode=ro")
	if err != nil {
		t.Fatalf("failed to open output database: %v", err)
	}
	defer db.Close()

	var count int
	row := db.QueryRow("SELECT COUNT(*) FROM Bible")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("failed to count verses: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 verse, got %d", count)
	}
}

// TestESwordRoundTrip tests the round-trip (L1 - semantic preservation).
func TestESwordRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "esword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	bblxPath := filepath.Join(tmpDir, "original.bblx")
	createTestBible(t, bblxPath)

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)

	// Extract IR
	extractReq := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       bblxPath,
			"output_dir": irDir,
		},
	}

	extractResp := executePlugin(t, &extractReq)
	if extractResp.Status != "ok" {
		t.Fatalf("extract-ir failed: %s", extractResp.Error)
	}

	extractResult := extractResp.Result.(map[string]interface{})
	irPath := extractResult["ir_path"].(string)

	// Emit native
	emitReq := ipc.Request{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irPath,
			"output_dir": outDir,
		},
	}

	emitResp := executePlugin(t, &emitReq)
	if emitResp.Status != "ok" {
		t.Fatalf("emit-native failed: %s", emitResp.Error)
	}

	emitResult := emitResp.Result.(map[string]interface{})
	outputPath := emitResult["output_path"].(string)

	// Compare original and output databases (L1 - semantic comparison)
	origDB, err := sql.Open(sqlite.DriverName(), bblxPath+"?mode=ro")
	if err != nil {
		t.Fatalf("failed to open original: %v", err)
	}
	defer origDB.Close()

	outDB, err := sql.Open(sqlite.DriverName(), outputPath+"?mode=ro")
	if err != nil {
		t.Fatalf("failed to open output: %v", err)
	}
	defer outDB.Close()

	// Compare verse count
	var origCount, outCount int
	origDB.QueryRow("SELECT COUNT(*) FROM Bible").Scan(&origCount)
	outDB.QueryRow("SELECT COUNT(*) FROM Bible").Scan(&outCount)

	if origCount != outCount {
		t.Errorf("verse count mismatch: original %d, output %d", origCount, outCount)
	}

	// Compare verse text
	rows, _ := origDB.Query("SELECT Book, Chapter, Verse, Scripture FROM Bible ORDER BY Book, Chapter, Verse")
	defer rows.Close()

	for rows.Next() {
		var book, chapter, verse int
		var scripture string
		rows.Scan(&book, &chapter, &verse, &scripture)

		var outScripture string
		outDB.QueryRow("SELECT Scripture FROM Bible WHERE Book=? AND Chapter=? AND Verse=?", book, chapter, verse).Scan(&outScripture)

		if scripture != outScripture {
			t.Errorf("text mismatch at %d.%d.%d: %q vs %q", book, chapter, verse, scripture, outScripture)
		}
	}
}

// TestESwordEnumerate tests the enumerate command.
func TestESwordEnumerate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "esword-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	bblxPath := filepath.Join(tmpDir, "test.bblx")
	createTestBible(t, bblxPath)

	req := ipc.Request{
		Command: "enumerate",
		Args:    map[string]interface{}{"path": bblxPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	entries, ok := result["entries"].([]interface{})
	if !ok {
		t.Fatal("entries is not an array")
	}

	if len(entries) < 2 {
		t.Errorf("expected at least 2 tables (Bible, Details), got %d", len(entries))
	}
}

// executePlugin runs the plugin with a request and returns the response.
func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()

	pluginPath := "./format-esword"
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
		if err := buildCmd.Run(); err != nil {
			t.Fatalf("failed to build plugin: %v", err)
		}
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	cmd := exec.Command(pluginPath)
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
