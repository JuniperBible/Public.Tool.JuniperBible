package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// createTestMyBible creates a minimal MyBible .SQLite3 database for testing.
func createTestMyBible(t *testing.T, path string) {
	t.Helper()

	db, err := sql.Open(sqlite.DriverName(), path)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	defer db.Close()

	// Create verses table (MyBible.zone schema)
	_, err = db.Exec(`CREATE TABLE verses (
		book_number INTEGER NOT NULL,
		chapter INTEGER NOT NULL,
		verse INTEGER NOT NULL,
		text TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("failed to create verses table: %v", err)
	}

	// Create info table
	_, err = db.Exec(`CREATE TABLE info (name TEXT, value TEXT)`)
	if err != nil {
		t.Fatalf("failed to create info table: %v", err)
	}

	// Insert test data - Genesis 1:1-2
	_, err = db.Exec(`INSERT INTO verses (book_number, chapter, verse, text) VALUES
		(1, 1, 1, 'In the beginning God created the heavens and the earth.'),
		(1, 1, 2, 'Now the earth was formless and empty.')`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Insert metadata
	_, err = db.Exec(`INSERT INTO info (name, value) VALUES
		('description', 'Test Bible'),
		('language', 'en'),
		('version', '1.0')`)
	if err != nil {
		t.Fatalf("failed to insert metadata: %v", err)
	}
}

// TestMyBibleDetect tests the detect command.
func TestMyBibleDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mybible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.SQLite3")
	createTestMyBible(t, dbPath)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": dbPath},
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
		t.Errorf("expected detected to be true, got false: %v", result["reason"])
	}
	if result["format"] != "MyBible" {
		t.Errorf("expected format MyBible, got %v", result["format"])
	}
}

// TestMyBibleDetectInvalidExtension tests detect with wrong extension.
func TestMyBibleDetectInvalidExtension(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mybible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open(sqlite.DriverName(), dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	db.Close()

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": dbPath},
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
		t.Error("expected detected to be false for wrong extension")
	}
}

// TestMyBibleDetectMissingVersesTable tests detect without verses table.
func TestMyBibleDetectMissingVersesTable(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mybible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.SQLite3")
	db, err := sql.Open(sqlite.DriverName(), dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create database with only info table
	_, err = db.Exec("CREATE TABLE info (name TEXT, value TEXT)")
	if err != nil {
		t.Fatalf("failed to create info table: %v", err)
	}
	db.Close()

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": dbPath},
	}

	resp := executePlugin(t, &req)
	result, _ := resp.Result.(map[string]interface{})

	if result["detected"] == true {
		t.Error("expected detected to be false for missing verses table")
	}
}

// TestMyBibleDetectDirectory tests detect on a directory.
func TestMyBibleDetectDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mybible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": tmpDir},
	}

	resp := executePlugin(t, &req)
	result, _ := resp.Result.(map[string]interface{})

	if result["detected"] == true {
		t.Error("expected detected to be false for directory")
	}
}

// TestMyBibleIngest tests the ingest command.
func TestMyBibleIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mybible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.SQLite3")
	createTestMyBible(t, dbPath)

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       dbPath,
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

	artifactID, _ := result["artifact_id"].(string)
	if artifactID != "test" {
		t.Errorf("expected artifact_id=test, got %s", artifactID)
	}

	blobSHA256, _ := result["blob_sha256"].(string)
	if blobSHA256 == "" {
		t.Error("expected non-empty blob_sha256")
	}

	// Verify blob was created
	blobPath := filepath.Join(outputDir, blobSHA256[:2], blobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Errorf("blob file not created at %s", blobPath)
	}
}

// TestMyBibleEnumerate tests the enumerate command.
func TestMyBibleEnumerate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mybible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.SQLite3")
	createTestMyBible(t, dbPath)

	req := ipc.Request{
		Command: "enumerate",
		Args:    map[string]interface{}{"path": dbPath},
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
		t.Errorf("expected at least 2 tables (verses, info), got %d", len(entries))
	}

	// Check for verses and info tables
	hasVerses, hasInfo := false, false
	for _, e := range entries {
		entry, _ := e.(map[string]interface{})
		path, _ := entry["path"].(string)
		if path == "verses" {
			hasVerses = true
		}
		if path == "info" {
			hasInfo = true
		}
	}

	if !hasVerses {
		t.Error("expected verses table in enumeration")
	}
	if !hasInfo {
		t.Error("expected info table in enumeration")
	}
}

// TestMyBibleExtractIR tests the extract-ir command.
func TestMyBibleExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mybible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.SQLite3")
	createTestMyBible(t, dbPath)

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       dbPath,
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

	irPath, _ := result["ir_path"].(string)
	if !strings.HasSuffix(irPath, ".ir.json") {
		t.Errorf("expected IR path to end with .ir.json, got %s", irPath)
	}

	// Verify IR file exists
	if _, err := os.Stat(irPath); os.IsNotExist(err) {
		t.Errorf("IR file not created at %s", irPath)
	}

	// Read and verify IR content
	irData, err := os.ReadFile(irPath)
	if err != nil {
		t.Fatalf("failed to read IR file: %v", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	if corpus.Title != "Test Bible" {
		t.Errorf("expected title='Test Bible', got '%s'", corpus.Title)
	}

	if corpus.Language != "en" {
		t.Errorf("expected language='en', got '%s'", corpus.Language)
	}

	if corpus.ModuleType != "BIBLE" {
		t.Errorf("expected module_type=BIBLE, got %s", corpus.ModuleType)
	}

	if len(corpus.Documents) == 0 {
		t.Fatal("expected at least one document")
	}

	if corpus.Documents[0].ID != "Gen" {
		t.Errorf("expected first document ID=Gen, got %s", corpus.Documents[0].ID)
	}

	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("expected 2 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

// TestMyBibleEmitNative tests the emit-native command.
func TestMyBibleEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mybible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	irPath := filepath.Join(tmpDir, "test.ir.json")
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	// Create IR file
	corpus := &ipc.Corpus{
		ID:           "test",
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "MyBible",
		Title:        "Test Bible",
		Language:     "en",
		Description:  "A test Bible module",
		LossClass:    "L1",
		Attributes:   map[string]string{"version": "1.0"},
		Documents: []*ipc.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				Attributes: map[string]string{
					"book_num": "1",
				},
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "In the beginning God created the heavens and the earth.",
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

	irData, _ := json.MarshalIndent(corpus, "", "  ")
	os.WriteFile(irPath, irData, 0644)

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

	outputPath, _ := result["output_path"].(string)
	if !strings.HasSuffix(outputPath, ".SQLite3") {
		t.Errorf("expected output path to end with .SQLite3, got %s", outputPath)
	}

	// Verify database was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Errorf("output database not created at %s", outputPath)
	}

	// Verify database content
	db, err := sql.Open(sqlite.DriverName(), outputPath)
	if err != nil {
		t.Fatalf("failed to open output database: %v", err)
	}
	defer db.Close()

	// Check verses table
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM verses").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query verses: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 verse, got %d", count)
	}

	// Check verse content
	var bookNum, chapter, verse int
	var text string
	err = db.QueryRow("SELECT book_number, chapter, verse, text FROM verses LIMIT 1").Scan(&bookNum, &chapter, &verse, &text)
	if err != nil {
		t.Fatalf("failed to query verse: %v", err)
	}

	if bookNum != 1 || chapter != 1 || verse != 1 {
		t.Errorf("expected Gen 1:1, got book %d, chapter %d, verse %d", bookNum, chapter, verse)
	}

	if text != "In the beginning God created the heavens and the earth." {
		t.Errorf("unexpected verse text: %s", text)
	}

	// Check info table
	var desc string
	err = db.QueryRow("SELECT value FROM info WHERE name = 'description'").Scan(&desc)
	if err != nil {
		t.Fatalf("failed to query info: %v", err)
	}

	if desc != "Test Bible" {
		t.Errorf("expected description='Test Bible', got '%s'", desc)
	}
}

// TestStripHTML tests the stripHTML function.
func TestStripHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<b>Hello</b>", "Hello"},
		{"Hello &amp; Goodbye", "Hello & Goodbye"},
		{"<p>Test &lt;html&gt;</p>", "Test <html>"},
		{"Plain text", "Plain text"},
		{"&quot;Quoted&quot;", "\"Quoted\""},
		{"<i>Italic</i> and <b>bold</b>", "Italic and bold"},
		{"Multiple&nbsp;spaces", "Multiple spaces"},
	}

	for _, tt := range tests {
		result := stripHTML(tt.input)
		if result != tt.expected {
			t.Errorf("stripHTML(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestBookNumToOSIS tests the book number mapping.
func TestBookNumToOSIS(t *testing.T) {
	tests := []struct {
		bookNum  int
		expected string
	}{
		{1, "Gen"},
		{2, "Exod"},
		{19, "Ps"},
		{40, "Matt"},
		{66, "Rev"},
	}

	for _, tt := range tests {
		result := bookNumToOSIS[tt.bookNum]
		if result != tt.expected {
			t.Errorf("bookNumToOSIS[%d] = %q, want %q", tt.bookNum, result, tt.expected)
		}
	}
}

// TestOSISToBookNum tests the reverse OSIS mapping.
func TestOSISToBookNum(t *testing.T) {
	tests := []struct {
		osis     string
		expected int
	}{
		{"Gen", 1},
		{"Exod", 2},
		{"Ps", 19},
		{"Matt", 40},
		{"Rev", 66},
	}

	for _, tt := range tests {
		result := osisToBookNum[tt.osis]
		if result != tt.expected {
			t.Errorf("osisToBookNum[%q] = %d, want %d", tt.osis, result, tt.expected)
		}
	}
}

// TestRoundTrip tests extract-ir followed by emit-native.
func TestRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mybible-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalPath := filepath.Join(tmpDir, "original.SQLite3")
	createTestMyBible(t, originalPath)

	irDir := filepath.Join(tmpDir, "ir")
	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outputDir, 0755)

	// Extract IR
	extractReq := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       originalPath,
			"output_dir": irDir,
		},
	}

	extractResp := executePlugin(t, &extractReq)
	if extractResp.Status != "ok" {
		t.Fatalf("extract-ir failed: %s", extractResp.Error)
	}

	extractResult, _ := extractResp.Result.(map[string]interface{})
	irPath, _ := extractResult["ir_path"].(string)

	// Emit native
	emitReq := ipc.Request{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irPath,
			"output_dir": outputDir,
		},
	}

	emitResp := executePlugin(t, &emitReq)
	if emitResp.Status != "ok" {
		t.Fatalf("emit-native failed: %s", emitResp.Error)
	}

	emitResult, _ := emitResp.Result.(map[string]interface{})
	outputPath, _ := emitResult["output_path"].(string)

	// Compare original and output
	originalDB, _ := sql.Open(sqlite.DriverName(), originalPath)
	defer originalDB.Close()

	outputDB, _ := sql.Open(sqlite.DriverName(), outputPath)
	defer outputDB.Close()

	// Check verse count
	var origCount, outCount int
	originalDB.QueryRow("SELECT COUNT(*) FROM verses").Scan(&origCount)
	outputDB.QueryRow("SELECT COUNT(*) FROM verses").Scan(&outCount)

	if origCount != outCount {
		t.Errorf("verse count mismatch: original=%d, output=%d", origCount, outCount)
	}

	// Check verse content
	origRows, _ := originalDB.Query("SELECT book_number, chapter, verse, text FROM verses ORDER BY book_number, chapter, verse")
	defer origRows.Close()

	outRows, _ := outputDB.Query("SELECT book_number, chapter, verse, text FROM verses ORDER BY book_number, chapter, verse")
	defer outRows.Close()

	for origRows.Next() && outRows.Next() {
		var origBook, origCh, origV int
		var origText string
		var outBook, outCh, outV int
		var outText string

		origRows.Scan(&origBook, &origCh, &origV, &origText)
		outRows.Scan(&outBook, &outCh, &outV, &outText)

		if origBook != outBook || origCh != outCh || origV != outV {
			t.Errorf("verse reference mismatch: orig=%d.%d.%d, out=%d.%d.%d",
				origBook, origCh, origV, outBook, outCh, outV)
		}

		if origText != outText {
			t.Errorf("verse text mismatch at %d.%d.%d: orig=%q, out=%q",
				origBook, origCh, origV, origText, outText)
		}
	}
}

// executePlugin runs the plugin with a request and returns the response.
func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()

	pluginPath := "./format-mybible"
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
		if err := buildCmd.Run(); err != nil {
			t.Fatalf("failed to build plugin: %v", err)
		}
	}

	var input bytes.Buffer
	if err := json.NewEncoder(&input).Encode(req); err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	cmd := exec.Command(pluginPath)
	cmd.Stdin = &input

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin execution failed: %v\nOutput: %s", err, string(output))
	}

	var resp ipc.Response
	if err := json.Unmarshal(output, &resp); err != nil {
		t.Fatalf("failed to decode response: %v\nOutput: %s", err, string(output))
	}

	return &resp
}
