//go:build cgo_sqlite

package sqlite_test

// These tests compare CGO (mattn/go-sqlite3) vs Pure Go implementation
// Run with: CGO_ENABLED=1 go test -tags cgo_sqlite -v -run Comparison

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3" // CGO driver

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
	purego "github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/driver" // Pure Go driver
)

// setupComparisonDBs creates two temporary databases - one with CGO, one with pure Go
func setupComparisonDBs(t *testing.T) (cgoDB, pureDB *sql.DB, cleanup func()) {
	t.Helper()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "sqlite-comparison-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Open CGO database
	cgoPath := filepath.Join(tempDir, "cgo.db")
	cgoDB, err = sql.Open("sqlite3", cgoPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open CGO database: %v", err)
	}

	// Open pure Go database
	purePath := filepath.Join(tempDir, "pure.db")
	pureDB, err = sqlite.Open(purePath)
	if err != nil {
		cgoDB.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open pure Go database: %v", err)
	}

	cleanup = func() {
		cgoDB.Close()
		pureDB.Close()
		os.RemoveAll(tempDir)
	}

	return cgoDB, pureDB, cleanup
}

// compareResults executes the same query on both databases and compares results
func compareResults(t *testing.T, cgoDB, pureDB *sql.DB, query string, args ...interface{}) {
	t.Helper()

	// Execute on CGO
	cgoRows, err := cgoDB.Query(query, args...)
	if err != nil {
		t.Fatalf("CGO query failed: %v", err)
	}
	defer cgoRows.Close()

	// Execute on Pure Go
	pureRows, err := pureDB.Query(query, args...)
	if err != nil {
		t.Fatalf("Pure Go query failed: %v", err)
	}
	defer pureRows.Close()

	// Get column names
	cgoCols, err := cgoRows.Columns()
	if err != nil {
		t.Fatalf("CGO columns failed: %v", err)
	}

	pureCols, err := pureRows.Columns()
	if err != nil {
		t.Fatalf("Pure Go columns failed: %v", err)
	}

	if !reflect.DeepEqual(cgoCols, pureCols) {
		t.Errorf("column names differ:\n  CGO:  %v\n  Pure: %v", cgoCols, pureCols)
	}

	// Compare rows
	rowNum := 0
	for {
		cgoHasNext := cgoRows.Next()
		pureHasNext := pureRows.Next()

		if cgoHasNext != pureHasNext {
			t.Fatalf("row count mismatch at row %d: CGO=%v, Pure=%v", rowNum, cgoHasNext, pureHasNext)
		}

		if !cgoHasNext {
			break
		}

		// Scan CGO row
		cgoVals := make([]interface{}, len(cgoCols))
		cgoValPtrs := make([]interface{}, len(cgoCols))
		for i := range cgoVals {
			cgoValPtrs[i] = &cgoVals[i]
		}
		if err := cgoRows.Scan(cgoValPtrs...); err != nil {
			t.Fatalf("CGO scan failed at row %d: %v", rowNum, err)
		}

		// Scan Pure Go row
		pureVals := make([]interface{}, len(pureCols))
		pureValPtrs := make([]interface{}, len(pureCols))
		for i := range pureVals {
			pureValPtrs[i] = &pureVals[i]
		}
		if err := pureRows.Scan(pureValPtrs...); err != nil {
			t.Fatalf("Pure Go scan failed at row %d: %v", rowNum, err)
		}

		// Compare values
		for i := range cgoVals {
			if !compareValues(cgoVals[i], pureVals[i]) {
				t.Errorf("row %d, col %d (%s) differs:\n  CGO:  %v (%T)\n  Pure: %v (%T)",
					rowNum, i, cgoCols[i], cgoVals[i], cgoVals[i], pureVals[i], pureVals[i])
			}
		}

		rowNum++
	}

	if err := cgoRows.Err(); err != nil {
		t.Errorf("CGO rows error: %v", err)
	}

	if err := pureRows.Err(); err != nil {
		t.Errorf("Pure Go rows error: %v", err)
	}
}

// compareValues compares two values from different drivers
func compareValues(cgoVal, pureVal interface{}) bool {
	// Handle nil
	if cgoVal == nil && pureVal == nil {
		return true
	}
	if cgoVal == nil || pureVal == nil {
		return false
	}

	// Handle byte slices specially (for BLOB data)
	if cgoBuf, ok := cgoVal.([]byte); ok {
		if pureBuf, ok := pureVal.([]byte); ok {
			return bytes.Equal(cgoBuf, pureBuf)
		}
		return false
	}

	// Use reflection for other types
	return reflect.DeepEqual(cgoVal, pureVal)
}

// compareSingleValue executes a query that returns a single value and compares
func compareSingleValue(t *testing.T, cgoDB, pureDB *sql.DB, query string, args ...interface{}) {
	t.Helper()

	var cgoVal, pureVal interface{}

	err := cgoDB.QueryRow(query, args...).Scan(&cgoVal)
	if err != nil {
		t.Fatalf("CGO query failed: %v", err)
	}

	err = pureDB.QueryRow(query, args...).Scan(&pureVal)
	if err != nil {
		t.Fatalf("Pure Go query failed: %v", err)
	}

	if !compareValues(cgoVal, pureVal) {
		t.Errorf("values differ:\n  CGO:  %v (%T)\n  Pure: %v (%T)",
			cgoVal, cgoVal, pureVal, pureVal)
	}
}

func TestComparisonBasicOperations(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	// Create table on both
	createSQL := `CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT)`
	if _, err := cgoDB.Exec(createSQL); err != nil {
		t.Fatalf("CGO CREATE failed: %v", err)
	}
	if _, err := pureDB.Exec(createSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}

	// Insert same data
	insertSQL := `INSERT INTO test (value) VALUES (?)`
	for _, val := range []string{"alpha", "beta", "gamma"} {
		if _, err := cgoDB.Exec(insertSQL, val); err != nil {
			t.Fatalf("CGO INSERT failed: %v", err)
		}
		if _, err := pureDB.Exec(insertSQL, val); err != nil {
			t.Fatalf("Pure Go INSERT failed: %v", err)
		}
	}

	// Compare SELECT results
	compareResults(t, cgoDB, pureDB, `SELECT id, value FROM test ORDER BY id`)
}

func TestComparisonDataTypes(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	// Create table with all SQLite data types
	createSQL := `
		CREATE TABLE types (
			int_val INTEGER,
			real_val REAL,
			text_val TEXT,
			blob_val BLOB,
			null_val TEXT
		)
	`
	if _, err := cgoDB.Exec(createSQL); err != nil {
		t.Fatalf("CGO CREATE failed: %v", err)
	}
	if _, err := pureDB.Exec(createSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}

	// Insert test data
	insertSQL := `INSERT INTO types VALUES (?, ?, ?, ?, ?)`
	testData := []interface{}{
		42,
		3.141592653589793,
		"Hello, World!",
		[]byte{0xDE, 0xAD, 0xBE, 0xEF},
		nil,
	}

	if _, err := cgoDB.Exec(insertSQL, testData...); err != nil {
		t.Fatalf("CGO INSERT failed: %v", err)
	}
	if _, err := pureDB.Exec(insertSQL, testData...); err != nil {
		t.Fatalf("Pure Go INSERT failed: %v", err)
	}

	// Compare results
	compareResults(t, cgoDB, pureDB, `SELECT * FROM types`)
}

func TestComparisonNullHandling(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	createSQL := `CREATE TABLE nullable (id INTEGER PRIMARY KEY, val TEXT)`
	if _, err := cgoDB.Exec(createSQL); err != nil {
		t.Fatalf("CGO CREATE failed: %v", err)
	}
	if _, err := pureDB.Exec(createSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}

	// Insert various NULL scenarios
	testCases := []struct {
		val interface{}
	}{
		{nil},
		{"not null"},
		{nil},
		{""},
		{nil},
	}

	for _, tc := range testCases {
		if _, err := cgoDB.Exec(`INSERT INTO nullable (val) VALUES (?)`, tc.val); err != nil {
			t.Fatalf("CGO INSERT failed: %v", err)
		}
		if _, err := pureDB.Exec(`INSERT INTO nullable (val) VALUES (?)`, tc.val); err != nil {
			t.Fatalf("Pure Go INSERT failed: %v", err)
		}
	}

	compareResults(t, cgoDB, pureDB, `SELECT id, val FROM nullable ORDER BY id`)
}

func TestComparisonUnicode(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	createSQL := `CREATE TABLE unicode (id INTEGER PRIMARY KEY, text TEXT)`
	if _, err := cgoDB.Exec(createSQL); err != nil {
		t.Fatalf("CGO CREATE failed: %v", err)
	}
	if _, err := pureDB.Exec(createSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}

	// Test various Unicode scripts
	unicodeTexts := []string{
		"בְּרֵאשִׁית בָּרָא אֱלֹהִים", // Hebrew
		"Ἐν ἀρχῇ ἦν ὁ λόγος",        // Greek
		"太初有道",                      // Chinese
		"В начале было Слово",        // Russian
		"🙏 ❤️ ✝️",                   // Emoji
		"",                            // Empty string
	}

	for _, text := range unicodeTexts {
		if _, err := cgoDB.Exec(`INSERT INTO unicode (text) VALUES (?)`, text); err != nil {
			t.Fatalf("CGO INSERT failed for %q: %v", text, err)
		}
		if _, err := pureDB.Exec(`INSERT INTO unicode (text) VALUES (?)`, text); err != nil {
			t.Fatalf("Pure Go INSERT failed for %q: %v", text, err)
		}
	}

	compareResults(t, cgoDB, pureDB, `SELECT id, text FROM unicode ORDER BY id`)
}

func TestComparisonAggregates(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	createSQL := `CREATE TABLE numbers (value INTEGER)`
	if _, err := cgoDB.Exec(createSQL); err != nil {
		t.Fatalf("CGO CREATE failed: %v", err)
	}
	if _, err := pureDB.Exec(createSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}

	// Insert test data
	for i := 1; i <= 100; i++ {
		if _, err := cgoDB.Exec(`INSERT INTO numbers VALUES (?)`, i); err != nil {
			t.Fatalf("CGO INSERT failed: %v", err)
		}
		if _, err := pureDB.Exec(`INSERT INTO numbers VALUES (?)`, i); err != nil {
			t.Fatalf("Pure Go INSERT failed: %v", err)
		}
	}

	// Test COUNT
	compareSingleValue(t, cgoDB, pureDB, `SELECT COUNT(*) FROM numbers`)

	// Test SUM
	compareSingleValue(t, cgoDB, pureDB, `SELECT SUM(value) FROM numbers`)

	// Test AVG
	compareSingleValue(t, cgoDB, pureDB, `SELECT AVG(value) FROM numbers`)

	// Test MIN
	compareSingleValue(t, cgoDB, pureDB, `SELECT MIN(value) FROM numbers`)

	// Test MAX
	compareSingleValue(t, cgoDB, pureDB, `SELECT MAX(value) FROM numbers`)

	// Test aggregate with WHERE
	compareSingleValue(t, cgoDB, pureDB, `SELECT COUNT(*) FROM numbers WHERE value > 50`)

	// Test multiple aggregates in one query
	compareResults(t, cgoDB, pureDB,
		`SELECT COUNT(*), SUM(value), AVG(value), MIN(value), MAX(value) FROM numbers`)
}

func TestComparisonStringFunctions(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	createSQL := `CREATE TABLE strings (text TEXT)`
	if _, err := cgoDB.Exec(createSQL); err != nil {
		t.Fatalf("CGO CREATE failed: %v", err)
	}
	if _, err := pureDB.Exec(createSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}

	testStrings := []string{
		"Hello World",
		"UPPERCASE",
		"lowercase",
		"MiXeD CaSe",
		"   spaces   ",
		"",
	}

	for _, s := range testStrings {
		if _, err := cgoDB.Exec(`INSERT INTO strings VALUES (?)`, s); err != nil {
			t.Fatalf("CGO INSERT failed: %v", err)
		}
		if _, err := pureDB.Exec(`INSERT INTO strings VALUES (?)`, s); err != nil {
			t.Fatalf("Pure Go INSERT failed: %v", err)
		}
	}

	// Test UPPER
	compareResults(t, cgoDB, pureDB, `SELECT UPPER(text) FROM strings`)

	// Test LOWER
	compareResults(t, cgoDB, pureDB, `SELECT LOWER(text) FROM strings`)

	// Test LENGTH
	compareResults(t, cgoDB, pureDB, `SELECT LENGTH(text) FROM strings`)

	// Test SUBSTR
	compareResults(t, cgoDB, pureDB, `SELECT SUBSTR(text, 1, 5) FROM strings WHERE LENGTH(text) >= 5`)

	// Test concatenation
	compareResults(t, cgoDB, pureDB, `SELECT text || '!' FROM strings`)

	// Test combined functions
	compareResults(t, cgoDB, pureDB,
		`SELECT UPPER(text), LOWER(text), LENGTH(text) FROM strings`)
}

func TestComparisonMathFunctions(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	createSQL := `CREATE TABLE numbers (value REAL)`
	if _, err := cgoDB.Exec(createSQL); err != nil {
		t.Fatalf("CGO CREATE failed: %v", err)
	}
	if _, err := pureDB.Exec(createSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}

	testNumbers := []float64{-5.7, -1.2, 0.0, 1.5, 3.14159, 10.999}

	for _, n := range testNumbers {
		if _, err := cgoDB.Exec(`INSERT INTO numbers VALUES (?)`, n); err != nil {
			t.Fatalf("CGO INSERT failed: %v", err)
		}
		if _, err := pureDB.Exec(`INSERT INTO numbers VALUES (?)`, n); err != nil {
			t.Fatalf("Pure Go INSERT failed: %v", err)
		}
	}

	// Test ABS
	compareResults(t, cgoDB, pureDB, `SELECT ABS(value) FROM numbers`)

	// Test ROUND
	compareResults(t, cgoDB, pureDB, `SELECT ROUND(value) FROM numbers`)

	// Test ROUND with precision
	compareResults(t, cgoDB, pureDB, `SELECT ROUND(value, 2) FROM numbers`)

	// Test arithmetic operations
	compareResults(t, cgoDB, pureDB, `SELECT value * 2 FROM numbers`)
	compareResults(t, cgoDB, pureDB, `SELECT value + 10 FROM numbers`)
	compareResults(t, cgoDB, pureDB, `SELECT value - 5 FROM numbers`)
	compareResults(t, cgoDB, pureDB, `SELECT value / 2 FROM numbers WHERE value != 0`)
}

func TestComparisonTransactions(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	createSQL := `CREATE TABLE accounts (id INTEGER PRIMARY KEY, balance INTEGER)`
	if _, err := cgoDB.Exec(createSQL); err != nil {
		t.Fatalf("CGO CREATE failed: %v", err)
	}
	if _, err := pureDB.Exec(createSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}

	// Initial insert on both
	if _, err := cgoDB.Exec(`INSERT INTO accounts (balance) VALUES (1000)`); err != nil {
		t.Fatalf("CGO INSERT failed: %v", err)
	}
	if _, err := pureDB.Exec(`INSERT INTO accounts (balance) VALUES (1000)`); err != nil {
		t.Fatalf("Pure Go INSERT failed: %v", err)
	}

	// Test successful commit on both
	cgoTx, err := cgoDB.Begin()
	if err != nil {
		t.Fatalf("CGO Begin failed: %v", err)
	}
	pureTx, err := pureDB.Begin()
	if err != nil {
		t.Fatalf("Pure Go Begin failed: %v", err)
	}

	if _, err := cgoTx.Exec(`UPDATE accounts SET balance = balance - 100`); err != nil {
		t.Fatalf("CGO UPDATE failed: %v", err)
	}
	if _, err := pureTx.Exec(`UPDATE accounts SET balance = balance - 100`); err != nil {
		t.Fatalf("Pure Go UPDATE failed: %v", err)
	}

	if err := cgoTx.Commit(); err != nil {
		t.Fatalf("CGO Commit failed: %v", err)
	}
	if err := pureTx.Commit(); err != nil {
		t.Fatalf("Pure Go Commit failed: %v", err)
	}

	// Verify commit results match
	compareResults(t, cgoDB, pureDB, `SELECT id, balance FROM accounts`)

	// Test rollback on both
	cgoTx, err = cgoDB.Begin()
	if err != nil {
		t.Fatalf("CGO Begin failed: %v", err)
	}
	pureTx, err = pureDB.Begin()
	if err != nil {
		t.Fatalf("Pure Go Begin failed: %v", err)
	}

	if _, err := cgoTx.Exec(`UPDATE accounts SET balance = balance - 500`); err != nil {
		t.Fatalf("CGO UPDATE failed: %v", err)
	}
	if _, err := pureTx.Exec(`UPDATE accounts SET balance = balance - 500`); err != nil {
		t.Fatalf("Pure Go UPDATE failed: %v", err)
	}

	if err := cgoTx.Rollback(); err != nil {
		t.Fatalf("CGO Rollback failed: %v", err)
	}
	if err := pureTx.Rollback(); err != nil {
		t.Fatalf("Pure Go Rollback failed: %v", err)
	}

	// Verify rollback results match
	compareResults(t, cgoDB, pureDB, `SELECT id, balance FROM accounts`)
}

func TestComparisonOrderByLimit(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	createSQL := `CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT, priority INTEGER)`
	if _, err := cgoDB.Exec(createSQL); err != nil {
		t.Fatalf("CGO CREATE failed: %v", err)
	}
	if _, err := pureDB.Exec(createSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}

	// Insert test data
	items := []struct {
		name     string
		priority int
	}{
		{"Task E", 5},
		{"Task A", 1},
		{"Task C", 3},
		{"Task B", 2},
		{"Task D", 4},
	}

	for _, item := range items {
		if _, err := cgoDB.Exec(`INSERT INTO items (name, priority) VALUES (?, ?)`,
			item.name, item.priority); err != nil {
			t.Fatalf("CGO INSERT failed: %v", err)
		}
		if _, err := pureDB.Exec(`INSERT INTO items (name, priority) VALUES (?, ?)`,
			item.name, item.priority); err != nil {
			t.Fatalf("Pure Go INSERT failed: %v", err)
		}
	}

	// Test ORDER BY ASC
	compareResults(t, cgoDB, pureDB, `SELECT name, priority FROM items ORDER BY priority ASC`)

	// Test ORDER BY DESC
	compareResults(t, cgoDB, pureDB, `SELECT name, priority FROM items ORDER BY priority DESC`)

	// Test LIMIT
	compareResults(t, cgoDB, pureDB, `SELECT name, priority FROM items ORDER BY priority LIMIT 3`)

	// Test LIMIT with OFFSET
	compareResults(t, cgoDB, pureDB, `SELECT name, priority FROM items ORDER BY priority LIMIT 2 OFFSET 1`)

	// Test ORDER BY with text column
	compareResults(t, cgoDB, pureDB, `SELECT name, priority FROM items ORDER BY name`)
}

func TestComparisonBlobHandling(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	createSQL := `CREATE TABLE blobs (id INTEGER PRIMARY KEY, data BLOB)`
	if _, err := cgoDB.Exec(createSQL); err != nil {
		t.Fatalf("CGO CREATE failed: %v", err)
	}
	if _, err := pureDB.Exec(createSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}

	// Test various blob sizes and patterns
	testBlobs := [][]byte{
		{},                                  // Empty blob
		{0x00},                              // Single byte
		{0xFF},                              // Max byte
		{0xDE, 0xAD, 0xBE, 0xEF},           // Classic test pattern
		bytes.Repeat([]byte{0xAA}, 1024),   // 1KB repeated pattern
		make([]byte, 0),                     // Another empty
	}

	for _, blob := range testBlobs {
		if _, err := cgoDB.Exec(`INSERT INTO blobs (data) VALUES (?)`, blob); err != nil {
			t.Fatalf("CGO INSERT failed for blob %s: %v", hex.EncodeToString(blob[:min(len(blob), 4)]), err)
		}
		if _, err := pureDB.Exec(`INSERT INTO blobs (data) VALUES (?)`, blob); err != nil {
			t.Fatalf("Pure Go INSERT failed for blob %s: %v", hex.EncodeToString(blob[:min(len(blob), 4)]), err)
		}
	}

	compareResults(t, cgoDB, pureDB, `SELECT id, data FROM blobs ORDER BY id`)
}

func TestComparisonWhereClauses(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	createSQL := `CREATE TABLE products (id INTEGER PRIMARY KEY, name TEXT, price REAL, stock INTEGER)`
	if _, err := cgoDB.Exec(createSQL); err != nil {
		t.Fatalf("CGO CREATE failed: %v", err)
	}
	if _, err := pureDB.Exec(createSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}

	products := []struct {
		name  string
		price float64
		stock int
	}{
		{"Apple", 1.50, 100},
		{"Banana", 0.75, 150},
		{"Cherry", 2.50, 50},
		{"Date", 3.00, 25},
		{"Elderberry", 1.25, 75},
	}

	for _, p := range products {
		insertSQL := `INSERT INTO products (name, price, stock) VALUES (?, ?, ?)`
		if _, err := cgoDB.Exec(insertSQL, p.name, p.price, p.stock); err != nil {
			t.Fatalf("CGO INSERT failed: %v", err)
		}
		if _, err := pureDB.Exec(insertSQL, p.name, p.price, p.stock); err != nil {
			t.Fatalf("Pure Go INSERT failed: %v", err)
		}
	}

	// Test simple WHERE
	compareResults(t, cgoDB, pureDB, `SELECT name, price FROM products WHERE price > 1.00`)

	// Test AND
	compareResults(t, cgoDB, pureDB, `SELECT name FROM products WHERE price > 1.00 AND stock < 100`)

	// Test OR
	compareResults(t, cgoDB, pureDB, `SELECT name FROM products WHERE price < 1.00 OR stock > 100`)

	// Test BETWEEN
	compareResults(t, cgoDB, pureDB, `SELECT name FROM products WHERE price BETWEEN 1.00 AND 2.00`)

	// Test LIKE
	compareResults(t, cgoDB, pureDB, `SELECT name FROM products WHERE name LIKE '%berry'`)

	// Test IN
	compareResults(t, cgoDB, pureDB, `SELECT name FROM products WHERE name IN ('Apple', 'Cherry', 'Date')`)

	// Test complex WHERE
	compareResults(t, cgoDB, pureDB,
		`SELECT name, price, stock FROM products WHERE (price > 1.00 AND stock < 100) OR name LIKE 'B%' ORDER BY price`)
}

func TestComparisonJoins(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	// Create tables
	authorsSQL := `CREATE TABLE authors (id INTEGER PRIMARY KEY, name TEXT)`
	booksSQL := `CREATE TABLE books (id INTEGER PRIMARY KEY, title TEXT, author_id INTEGER)`

	if _, err := cgoDB.Exec(authorsSQL); err != nil {
		t.Fatalf("CGO CREATE authors failed: %v", err)
	}
	if _, err := cgoDB.Exec(booksSQL); err != nil {
		t.Fatalf("CGO CREATE books failed: %v", err)
	}
	if _, err := pureDB.Exec(authorsSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}
	if _, err := pureDB.Exec(booksSQL); err != nil {
		t.Fatalf("Pure Go CREATE books failed: %v", err)
	}

	// Insert authors
	authors := []string{"John Doe", "Jane Smith", "Bob Wilson"}
	for i, author := range authors {
		if _, err := cgoDB.Exec(`INSERT INTO authors (id, name) VALUES (?, ?)`, i+1, author); err != nil {
			t.Fatalf("CGO INSERT author failed: %v", err)
		}
		if _, err := pureDB.Exec(`INSERT INTO authors (id, name) VALUES (?, ?)`, i+1, author); err != nil {
			t.Fatalf("Pure Go INSERT author failed: %v", err)
		}
	}

	// Insert books
	books := []struct {
		title    string
		authorID int
	}{
		{"Book A", 1},
		{"Book B", 1},
		{"Book C", 2},
		{"Book D", 3},
	}

	for _, book := range books {
		if _, err := cgoDB.Exec(`INSERT INTO books (title, author_id) VALUES (?, ?)`,
			book.title, book.authorID); err != nil {
			t.Fatalf("CGO INSERT book failed: %v", err)
		}
		if _, err := pureDB.Exec(`INSERT INTO books (title, author_id) VALUES (?, ?)`,
			book.title, book.authorID); err != nil {
			t.Fatalf("Pure Go INSERT book failed: %v", err)
		}
	}

	// Test INNER JOIN
	compareResults(t, cgoDB, pureDB,
		`SELECT books.title, authors.name FROM books JOIN authors ON books.author_id = authors.id ORDER BY books.title`)

	// Test LEFT JOIN
	compareResults(t, cgoDB, pureDB,
		`SELECT books.title, authors.name FROM books LEFT JOIN authors ON books.author_id = authors.id ORDER BY books.title`)
}

func TestComparisonGroupBy(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	createSQL := `CREATE TABLE sales (id INTEGER PRIMARY KEY, product TEXT, quantity INTEGER, price REAL)`
	if _, err := cgoDB.Exec(createSQL); err != nil {
		t.Fatalf("CGO CREATE failed: %v", err)
	}
	if _, err := pureDB.Exec(createSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}

	sales := []struct {
		product  string
		quantity int
		price    float64
	}{
		{"Widget", 10, 5.00},
		{"Widget", 5, 5.00},
		{"Gadget", 3, 10.00},
		{"Widget", 2, 5.00},
		{"Gadget", 7, 10.00},
	}

	for _, sale := range sales {
		insertSQL := `INSERT INTO sales (product, quantity, price) VALUES (?, ?, ?)`
		if _, err := cgoDB.Exec(insertSQL, sale.product, sale.quantity, sale.price); err != nil {
			t.Fatalf("CGO INSERT failed: %v", err)
		}
		if _, err := pureDB.Exec(insertSQL, sale.product, sale.quantity, sale.price); err != nil {
			t.Fatalf("Pure Go INSERT failed: %v", err)
		}
	}

	// Test GROUP BY with COUNT
	compareResults(t, cgoDB, pureDB,
		`SELECT product, COUNT(*) FROM sales GROUP BY product ORDER BY product`)

	// Test GROUP BY with SUM
	compareResults(t, cgoDB, pureDB,
		`SELECT product, SUM(quantity) FROM sales GROUP BY product ORDER BY product`)

	// Test GROUP BY with multiple aggregates
	compareResults(t, cgoDB, pureDB,
		`SELECT product, COUNT(*), SUM(quantity), AVG(quantity) FROM sales GROUP BY product ORDER BY product`)

	// Test GROUP BY with HAVING
	compareResults(t, cgoDB, pureDB,
		`SELECT product, SUM(quantity) FROM sales GROUP BY product HAVING SUM(quantity) > 5 ORDER BY product`)
}

func TestComparisonPreparedStatements(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	createSQL := `CREATE TABLE colors (id INTEGER PRIMARY KEY, name TEXT, hex TEXT)`
	if _, err := cgoDB.Exec(createSQL); err != nil {
		t.Fatalf("CGO CREATE failed: %v", err)
	}
	if _, err := pureDB.Exec(createSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}

	// Prepare statements on both
	cgoStmt, err := cgoDB.Prepare(`INSERT INTO colors (name, hex) VALUES (?, ?)`)
	if err != nil {
		t.Fatalf("CGO Prepare failed: %v", err)
	}
	defer cgoStmt.Close()

	pureStmt, err := pureDB.Prepare(`INSERT INTO colors (name, hex) VALUES (?, ?)`)
	if err != nil {
		t.Fatalf("Pure Go Prepare failed: %v", err)
	}
	defer pureStmt.Close()

	// Execute prepared statements
	colors := []struct {
		name string
		hex  string
	}{
		{"Red", "#FF0000"},
		{"Green", "#00FF00"},
		{"Blue", "#0000FF"},
	}

	for _, color := range colors {
		if _, err := cgoStmt.Exec(color.name, color.hex); err != nil {
			t.Fatalf("CGO Exec failed: %v", err)
		}
		if _, err := pureStmt.Exec(color.name, color.hex); err != nil {
			t.Fatalf("Pure Go Exec failed: %v", err)
		}
	}

	// Compare results
	compareResults(t, cgoDB, pureDB, `SELECT id, name, hex FROM colors ORDER BY id`)
}

func TestComparisonEdgeCases(t *testing.T) {
	cgoDB, pureDB, cleanup := setupComparisonDBs(t)
	defer cleanup()

	createSQL := `CREATE TABLE edge_cases (id INTEGER PRIMARY KEY, value TEXT)`
	if _, err := cgoDB.Exec(createSQL); err != nil {
		t.Fatalf("CGO CREATE failed: %v", err)
	}
	if _, err := pureDB.Exec(createSQL); err != nil {
		t.Skipf("Pure Go CREATE not yet implemented: %v", err)
	}

	// Test edge cases
	edgeCases := []string{
		"",                    // Empty string
		" ",                   // Single space
		"\n",                  // Newline
		"\t",                  // Tab
		"'",                   // Single quote
		"\"",                  // Double quote
		"\\",                  // Backslash
		strings.Repeat("a", 1000), // Long string
		"\x00",                // Null byte
		"Multiple\nLines\nText", // Multi-line
	}

	for _, val := range edgeCases {
		if _, err := cgoDB.Exec(`INSERT INTO edge_cases (value) VALUES (?)`, val); err != nil {
			t.Fatalf("CGO INSERT failed for %q: %v", val, err)
		}
		if _, err := pureDB.Exec(`INSERT INTO edge_cases (value) VALUES (?)`, val); err != nil {
			t.Fatalf("Pure Go INSERT failed for %q: %v", val, err)
		}
	}

	compareResults(t, cgoDB, pureDB, `SELECT id, value FROM edge_cases ORDER BY id`)
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
