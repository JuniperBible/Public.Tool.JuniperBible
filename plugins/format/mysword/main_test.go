//go:build !sdk

package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
)

// TestDetectMySword tests detection of MySword files
func TestDetectMySword(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create a test .mybible file
	dbPath := filepath.Join(tmpDir, "test.mybible")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create Books table (MySword style)
	_, err = db.Exec(`CREATE TABLE Books (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create Books table: %v", err)
	}

	// Insert test data
	_, err = db.Exec(`INSERT INTO Books (Book, Chapter, Verse, Scripture) VALUES (1, 1, 1, 'In the beginning God created the heaven and the earth.')`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to insert test data: %v", err)
	}
	db.Close()

	// Test detection
	args := map[string]interface{}{"path": dbPath}
	result := testDetect(args)

	if !result.Detected {
		t.Errorf("expected detected=true, got false: %s", result.Reason)
	}
	if result.Format != "MySword" {
		t.Errorf("expected format=MySword, got %s", result.Format)
	}
}

// TestDetectMySwordCommentary tests detection of commentary files
func TestDetectMySwordCommentary(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.commentaries.mybible")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE commentaries (book_number INTEGER, chapter_number_from INTEGER, chapter_number_to INTEGER, verse_number_from INTEGER, verse_number_to INTEGER, text TEXT)`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create table: %v", err)
	}
	db.Close()

	args := map[string]interface{}{"path": dbPath}
	result := testDetect(args)

	if !result.Detected {
		t.Errorf("expected detected=true for commentary, got false: %s", result.Reason)
	}
}

// TestMySwordRoundTrip tests full round-trip: create -> extract-ir -> emit-native -> verify
func TestMySwordRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	// Step 1: Create a MySword database with test content
	srcPath := filepath.Join(tmpDir, "source.mybible")
	db, err := sqlite.Open(srcPath)
	if err != nil {
		t.Fatalf("failed to create source database: %v", err)
	}

	// Create tables
	_, err = db.Exec(`CREATE TABLE Books (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create Books table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE info (name TEXT, value TEXT)`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create info table: %v", err)
	}

	// Insert test verses (Genesis 1:1-3)
	verses := []struct {
		book, chapter, verse int
		text                 string
	}{
		{1, 1, 1, "In the beginning God created the heaven and the earth."},
		{1, 1, 2, "And the earth was without form, and void; and darkness was upon the face of the deep."},
		{1, 1, 3, "And God said, Let there be light: and there was light."},
		{40, 1, 1, "The book of the generation of Jesus Christ, the son of David, the son of Abraham."},
	}

	for _, v := range verses {
		_, err = db.Exec(`INSERT INTO Books (Book, Chapter, Verse, Scripture) VALUES (?, ?, ?, ?)`,
			v.book, v.chapter, v.verse, v.text)
		if err != nil {
			db.Close()
			t.Fatalf("failed to insert verse: %v", err)
		}
	}

	// Insert metadata
	_, err = db.Exec(`INSERT INTO info (name, value) VALUES ('description', 'Test Bible')`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to insert metadata: %v", err)
	}
	db.Close()

	// Step 2: Extract IR
	irDir := filepath.Join(tmpDir, "ir")
	if err := os.MkdirAll(irDir, 0755); err != nil {
		t.Fatalf("failed to create IR dir: %v", err)
	}

	extractArgs := map[string]interface{}{
		"path":       srcPath,
		"output_dir": irDir,
	}
	corpus := testExtractIR(t, extractArgs)

	// Verify extraction
	if corpus.ModuleType != "BIBLE" {
		t.Errorf("expected module type BIBLE, got %s", corpus.ModuleType)
	}
	if len(corpus.Documents) != 2 {
		t.Errorf("expected 2 documents (Gen, Matt), got %d", len(corpus.Documents))
	}

	// Count verses
	totalVerses := 0
	for _, doc := range corpus.Documents {
		totalVerses += len(doc.ContentBlocks)
	}
	if totalVerses != 4 {
		t.Errorf("expected 4 verses, got %d", totalVerses)
	}

	// Step 3: Emit native
	emitDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(emitDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	irPath := filepath.Join(irDir, "source.ir.json")
	emitArgs := map[string]interface{}{
		"ir_path":    irPath,
		"output_dir": emitDir,
	}
	emitResult := testEmitNative(t, emitArgs)

	if emitResult.Format != "MySword" {
		t.Errorf("expected format MySword, got %s", emitResult.Format)
	}

	// Step 4: Verify output database
	outPath := emitResult.OutputPath
	outDB, err := sqlite.OpenReadOnly(outPath)
	if err != nil {
		t.Fatalf("failed to open output database: %v", err)
	}
	defer outDB.Close()

	// Count verses in output
	var count int
	err = outDB.QueryRow("SELECT COUNT(*) FROM Books").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count verses: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 verses in output, got %d", count)
	}

	// Verify specific verse content
	var scripture string
	err = outDB.QueryRow("SELECT Scripture FROM Books WHERE Book=1 AND Chapter=1 AND Verse=1").Scan(&scripture)
	if err != nil {
		t.Fatalf("failed to query verse: %v", err)
	}
	if scripture != verses[0].text {
		t.Errorf("verse text mismatch:\ngot:  %s\nwant: %s", scripture, verses[0].text)
	}
}

// TestMySwordHTMLStripping tests that HTML is properly stripped
func TestMySwordHTMLStripping(t *testing.T) {
	tmpDir := t.TempDir()

	// Create database with HTML content
	srcPath := filepath.Join(tmpDir, "html-test.mybible")
	db, err := sqlite.Open(srcPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE Books (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert verse with HTML formatting
	htmlText := `<p>In the <b>beginning</b> God created the <i>heaven</i> and the earth.</p>`
	_, err = db.Exec(`INSERT INTO Books (Book, Chapter, Verse, Scripture) VALUES (1, 1, 1, ?)`, htmlText)
	if err != nil {
		db.Close()
		t.Fatalf("failed to insert verse: %v", err)
	}
	db.Close()

	// Extract IR
	irDir := filepath.Join(tmpDir, "ir")
	os.MkdirAll(irDir, 0755)

	extractArgs := map[string]interface{}{
		"path":       srcPath,
		"output_dir": irDir,
	}
	corpus := testExtractIR(t, extractArgs)

	// Verify HTML was stripped
	if len(corpus.Documents) == 0 || len(corpus.Documents[0].ContentBlocks) == 0 {
		t.Fatal("no content blocks extracted")
	}

	text := corpus.Documents[0].ContentBlocks[0].Text
	expectedText := "In the beginning God created the heaven and the earth."
	if text != expectedText {
		t.Errorf("HTML stripping failed:\ngot:  %s\nwant: %s", text, expectedText)
	}
}

// TestMySwordDictionary tests dictionary round-trip
func TestMySwordDictionary(t *testing.T) {
	tmpDir := t.TempDir()

	// Create dictionary database
	srcPath := filepath.Join(tmpDir, "test.dictionary.mybible")
	db, err := sqlite.Open(srcPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE dictionary (topic TEXT, definition TEXT)`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create table: %v", err)
	}

	entries := []struct {
		topic, definition string
	}{
		{"Abraham", "The father of the Hebrew nation, called by God to leave Ur."},
		{"Babel", "A city in ancient Mesopotamia known for its tower."},
		{"Covenant", "A solemn agreement between God and His people."},
	}

	for _, e := range entries {
		_, err = db.Exec(`INSERT INTO dictionary (topic, definition) VALUES (?, ?)`, e.topic, e.definition)
		if err != nil {
			db.Close()
			t.Fatalf("failed to insert entry: %v", err)
		}
	}
	db.Close()

	// Extract IR
	irDir := filepath.Join(tmpDir, "ir")
	os.MkdirAll(irDir, 0755)

	extractArgs := map[string]interface{}{
		"path":       srcPath,
		"output_dir": irDir,
	}
	corpus := testExtractIR(t, extractArgs)

	if corpus.ModuleType != "DICTIONARY" {
		t.Errorf("expected module type DICTIONARY, got %s", corpus.ModuleType)
	}

	// Count entries
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) != 3 {
		t.Errorf("expected 3 entries, got %d", len(corpus.Documents[0].ContentBlocks))
	}

	// Emit and verify
	emitDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(emitDir, 0755)

	irPath := filepath.Join(irDir, "test.ir.json")
	emitArgs := map[string]interface{}{
		"ir_path":    irPath,
		"output_dir": emitDir,
	}
	emitResult := testEmitNative(t, emitArgs)

	// Verify output
	outDB, err := sqlite.OpenReadOnly(emitResult.OutputPath)
	if err != nil {
		t.Fatalf("failed to open output: %v", err)
	}
	defer outDB.Close()

	var count int
	err = outDB.QueryRow("SELECT COUNT(*) FROM dictionary").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count entries: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 dictionary entries, got %d", count)
	}
}

// TestMySwordCommentary tests commentary round-trip
func TestMySwordCommentary(t *testing.T) {
	tmpDir := t.TempDir()

	// Create commentary database
	srcPath := filepath.Join(tmpDir, "test.commentaries.mybible")
	db, err := sqlite.Open(srcPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE commentaries (book_number INTEGER, chapter_number_from INTEGER, chapter_number_to INTEGER, verse_number_from INTEGER, verse_number_to INTEGER, text TEXT)`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert test commentary entries
	_, err = db.Exec(`INSERT INTO commentaries VALUES (1, 1, 1, 1, 1, 'Commentary on Genesis 1:1')`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to insert entry: %v", err)
	}
	_, err = db.Exec(`INSERT INTO commentaries VALUES (1, 1, 1, 2, 3, 'Commentary on Genesis 1:2-3')`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to insert entry: %v", err)
	}
	db.Close()

	// Extract IR
	irDir := filepath.Join(tmpDir, "ir")
	os.MkdirAll(irDir, 0755)

	extractArgs := map[string]interface{}{
		"path":       srcPath,
		"output_dir": irDir,
	}
	corpus := testExtractIR(t, extractArgs)

	if corpus.ModuleType != "COMMENTARY" {
		t.Errorf("expected module type COMMENTARY, got %s", corpus.ModuleType)
	}

	// Emit and verify round-trip
	emitDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(emitDir, 0755)

	irPath := filepath.Join(irDir, "test.ir.json")
	emitArgs := map[string]interface{}{
		"ir_path":    irPath,
		"output_dir": emitDir,
	}
	emitResult := testEmitNative(t, emitArgs)

	outDB, err := sqlite.OpenReadOnly(emitResult.OutputPath)
	if err != nil {
		t.Fatalf("failed to open output: %v", err)
	}
	defer outDB.Close()

	var count int
	err = outDB.QueryRow("SELECT COUNT(*) FROM commentaries").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count entries: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 commentary entries, got %d", count)
	}
}

// Helper functions for testing

func testDetect(args map[string]interface{}) *ipc.DetectResult {
	path := args["path"].(string)

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return &ipc.DetectResult{Detected: false, Reason: "invalid path"}
	}

	base := filepath.Base(path)
	if !contains(base, ".mybible") {
		return &ipc.DetectResult{Detected: false, Reason: "not a .mybible file"}
	}

	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: "not a SQLite database"}
	}
	defer db.Close()

	return &ipc.DetectResult{
		Detected: true,
		Format:   "MySword",
		Reason:   "MySword database detected",
	}
}

func testExtractIR(t *testing.T, args map[string]interface{}) *ipc.Corpus {
	path := args["path"].(string)
	outputDir := args["output_dir"].(string)

	base := filepath.Base(path)
	artifactID := base
	for contains(artifactID, ".") {
		artifactID = artifactID[:len(artifactID)-len(filepath.Ext(artifactID))]
	}

	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	corpus := &ipc.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		SourceFormat: "MySword",
		LossClass:    "L1",
		Attributes:   make(map[string]string),
	}

	var lostElements []ipc.LostElement

	if contains(base, ".commentaries.mybible") {
		corpus.ModuleType = "COMMENTARY"
		extractCommentaryIR(db, corpus, &lostElements)
	} else if contains(base, ".dictionary.mybible") {
		corpus.ModuleType = "DICTIONARY"
		extractDictionaryIR(db, corpus, &lostElements)
	} else {
		corpus.ModuleType = "BIBLE"
		extractBibleIR(db, corpus, &lostElements)
	}

	// Write IR file
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatalf("failed to serialize IR: %v", err)
	}

	irPath := filepath.Join(outputDir, artifactID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("failed to write IR: %v", err)
	}

	return corpus
}

func testEmitNative(t *testing.T, args map[string]interface{}) *ipc.EmitNativeResult {
	irPath := args["ir_path"].(string)
	outputDir := args["output_dir"].(string)

	data, err := os.ReadFile(irPath)
	if err != nil {
		t.Fatalf("failed to read IR: %v", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	ext := ".mybible"
	switch corpus.ModuleType {
	case "COMMENTARY":
		ext = ".commentaries.mybible"
	case "DICTIONARY":
		ext = ".dictionary.mybible"
	}

	outputPath := filepath.Join(outputDir, corpus.ID+ext)

	db, err := sqlite.Open(outputPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	switch corpus.ModuleType {
	case "BIBLE":
		if err := emitBibleNative(db, &corpus); err != nil {
			t.Fatalf("failed to emit Bible: %v", err)
		}
	case "COMMENTARY":
		if err := emitCommentaryNative(db, &corpus); err != nil {
			t.Fatalf("failed to emit commentary: %v", err)
		}
	case "DICTIONARY":
		if err := emitDictionaryNative(db, &corpus); err != nil {
			t.Fatalf("failed to emit dictionary: %v", err)
		}
	}

	return &ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "MySword",
		LossClass:  "L1",
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
