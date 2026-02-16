// SQLite tool integration tests.
// These tests require sqlite3 CLI to be installed.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSQLite3Available checks if sqlite3 is installed.
func TestSQLite3Available(t *testing.T) {
	if !HasTool(ToolSQLite3) {
		t.Skip("sqlite3 not installed")
	}

	output, err := RunTool(t, ToolSQLite3, "--version")
	if err != nil {
		t.Fatalf("sqlite3 --version failed: %v", err)
	}

	if !strings.Contains(output, "3.") {
		t.Errorf("unexpected sqlite3 output: %s", output)
	}

	t.Logf("sqlite3 version: %s", strings.TrimSpace(output))
}

// TestSQLite3CreateAndQuery tests creating a database and querying it.
func TestSQLite3CreateAndQuery(t *testing.T) {
	RequireTool(t, ToolSQLite3)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "sqlite-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Create table and insert data
	sql := `
CREATE TABLE books (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    author TEXT NOT NULL
);
INSERT INTO books (title, author) VALUES ('The Go Programming Language', 'Alan Donovan');
INSERT INTO books (title, author) VALUES ('Programming in Go', 'Mark Summerfield');
INSERT INTO books (title, author) VALUES ('Go in Action', 'William Kennedy');
`

	cmd := exec.Command("sqlite3", dbPath, sql)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to create database: %v\nOutput: %s", err, output)
	}

	// Query the data
	cmd = exec.Command("sqlite3", dbPath, "SELECT title FROM books ORDER BY id;")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("query failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "The Go Programming Language") {
		t.Errorf("expected title not found: %s", outputStr)
	}

	t.Log("successfully created and queried database")
}

// TestSQLite3ExportCSV tests exporting data to CSV.
func TestSQLite3ExportCSV(t *testing.T) {
	RequireTool(t, ToolSQLite3)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "sqlite-csv-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Create and populate database
	sql := `
CREATE TABLE verses (
    book TEXT,
    chapter INTEGER,
    verse INTEGER,
    text TEXT
);
INSERT INTO verses VALUES ('Genesis', 1, 1, 'In the beginning God created the heaven and the earth.');
INSERT INTO verses VALUES ('Genesis', 1, 2, 'And the earth was without form, and void.');
INSERT INTO verses VALUES ('Genesis', 1, 3, 'And God said, Let there be light: and there was light.');
`

	cmd := exec.Command("sqlite3", dbPath, sql)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create database: %v\nOutput: %s", err, output)
	}

	// Export to CSV
	csvPath := filepath.Join(tempDir, "export.csv")
	cmd = exec.Command("sqlite3", "-header", "-csv", dbPath, "SELECT * FROM verses;")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CSV export failed: %v\nOutput: %s", err, output)
	}

	if err := os.WriteFile(csvPath, output, 0600); err != nil {
		t.Fatalf("failed to write CSV: %v", err)
	}

	csv := string(output)
	if !strings.Contains(csv, "book,chapter,verse,text") {
		t.Error("CSV header not found")
	}
	if !strings.Contains(csv, "Genesis") {
		t.Error("Genesis not found in CSV")
	}

	t.Logf("successfully exported to CSV (%d bytes)", len(output))
}

// TestSQLite3Schema tests dumping database schema.
func TestSQLite3Schema(t *testing.T) {
	RequireTool(t, ToolSQLite3)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "sqlite-schema-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Create tables
	sql := `
CREATE TABLE books (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    isbn TEXT UNIQUE
);
CREATE TABLE authors (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);
CREATE TABLE book_authors (
    book_id INTEGER REFERENCES books(id),
    author_id INTEGER REFERENCES authors(id),
    PRIMARY KEY (book_id, author_id)
);
CREATE INDEX idx_books_title ON books(title);
`

	cmd := exec.Command("sqlite3", dbPath, sql)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create database: %v\nOutput: %s", err, output)
	}

	// Dump schema
	cmd = exec.Command("sqlite3", dbPath, ".schema")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("schema dump failed: %v\nOutput: %s", err, output)
	}

	schema := string(output)
	if !strings.Contains(schema, "CREATE TABLE books") {
		t.Error("books table not found in schema")
	}
	if !strings.Contains(schema, "CREATE TABLE authors") {
		t.Error("authors table not found in schema")
	}
	if !strings.Contains(schema, "CREATE INDEX") {
		t.Error("index not found in schema")
	}

	t.Log("successfully dumped schema")
}

// TestSQLite3Tables tests listing tables.
func TestSQLite3Tables(t *testing.T) {
	RequireTool(t, ToolSQLite3)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "sqlite-tables-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Create multiple tables
	sql := `
CREATE TABLE users (id INTEGER PRIMARY KEY);
CREATE TABLE posts (id INTEGER PRIMARY KEY);
CREATE TABLE comments (id INTEGER PRIMARY KEY);
`

	cmd := exec.Command("sqlite3", dbPath, sql)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create database: %v\nOutput: %s", err, output)
	}

	// List tables
	cmd = exec.Command("sqlite3", dbPath, ".tables")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("table list failed: %v\nOutput: %s", err, output)
	}

	tables := string(output)
	if !strings.Contains(tables, "users") {
		t.Error("users table not listed")
	}
	if !strings.Contains(tables, "posts") {
		t.Error("posts table not listed")
	}
	if !strings.Contains(tables, "comments") {
		t.Error("comments table not listed")
	}

	t.Log("successfully listed tables")
}

// TestSQLite3FTS tests Full-Text Search if available.
func TestSQLite3FTS(t *testing.T) {
	RequireTool(t, ToolSQLite3)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "sqlite-fts-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Create FTS table
	sql := `
CREATE VIRTUAL TABLE verses_fts USING fts5(book, chapter, verse, text);
INSERT INTO verses_fts VALUES ('Genesis', '1', '1', 'In the beginning God created the heaven and the earth.');
INSERT INTO verses_fts VALUES ('Genesis', '1', '3', 'And God said, Let there be light: and there was light.');
INSERT INTO verses_fts VALUES ('John', '1', '1', 'In the beginning was the Word, and the Word was with God.');
`

	cmd := exec.Command("sqlite3", dbPath, sql)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// FTS5 might not be available
		if strings.Contains(string(output), "no such module") {
			t.Skip("FTS5 not available in this SQLite build")
		}
		t.Fatalf("failed to create FTS table: %v\nOutput: %s", err, output)
	}

	// Search for "beginning"
	cmd = exec.Command("sqlite3", dbPath, "SELECT book, verse FROM verses_fts WHERE verses_fts MATCH 'beginning';")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("FTS query failed: %v\nOutput: %s", err, output)
	}

	results := string(output)
	if !strings.Contains(results, "Genesis") || !strings.Contains(results, "John") {
		t.Errorf("expected both Genesis and John in results: %s", results)
	}

	t.Log("successfully used FTS5")
}
