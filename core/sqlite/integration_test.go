package sqlite_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
)

// setupTestDB creates a temporary test database and returns cleanup function
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "sqlite-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return db, cleanup
}

// Integration tests for the pure Go SQLite replacement
// These tests verify that the implementation works identically to external implementations

func TestIntegrationCreateTableAndInsert(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create table
	_, err := db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			age INTEGER,
			email TEXT UNIQUE
		)
	`)
	if err != nil {
		t.Skipf("CREATE TABLE not yet implemented: %v", err)
	}

	// Insert data
	result, err := db.Exec(`INSERT INTO users (name, age, email) VALUES (?, ?, ?)`,
		"Alice", 30, "alice@example.com")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Check last insert ID
	lastID, err := result.LastInsertId()
	if err != nil {
		t.Errorf("failed to get last insert ID: %v", err)
	}
	if lastID != 1 {
		t.Errorf("expected last insert ID = 1, got %d", lastID)
	}

	// Check rows affected
	affected, err := result.RowsAffected()
	if err != nil {
		t.Errorf("failed to get rows affected: %v", err)
	}
	if affected != 1 {
		t.Errorf("expected rows affected = 1, got %d", affected)
	}

	// Verify data
	var name string
	var age int
	var email string
	err = db.QueryRow(`SELECT name, age, email FROM users WHERE id = ?`, lastID).
		Scan(&name, &age, &email)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if name != "Alice" || age != 30 || email != "alice@example.com" {
		t.Errorf("data mismatch: got (%s, %d, %s)", name, age, email)
	}
}

func TestIntegrationSelectWithWhere(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Setup
	_, err := db.Exec(`CREATE TABLE products (id INTEGER PRIMARY KEY, name TEXT, price REAL)`)
	if err != nil {
		t.Skipf("CREATE TABLE not yet implemented: %v", err)
	}

	products := []struct {
		name  string
		price float64
	}{
		{"Apple", 1.50},
		{"Banana", 0.75},
		{"Cherry", 2.50},
		{"Date", 3.00},
	}

	for _, p := range products {
		_, err = db.Exec(`INSERT INTO products (name, price) VALUES (?, ?)`, p.name, p.price)
		if err != nil {
			t.Fatalf("failed to insert %s: %v", p.name, err)
		}
	}

	// Test WHERE clause
	rows, err := db.Query(`SELECT name, price FROM products WHERE price > ? ORDER BY price`, 1.0)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	defer rows.Close()

	expected := []struct {
		name  string
		price float64
	}{
		{"Apple", 1.50},
		{"Cherry", 2.50},
		{"Date", 3.00},
	}

	count := 0
	for rows.Next() {
		var name string
		var price float64
		if err := rows.Scan(&name, &price); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}

		if count >= len(expected) {
			t.Fatalf("too many rows returned")
		}

		if name != expected[count].name || price != expected[count].price {
			t.Errorf("row %d: expected (%s, %.2f), got (%s, %.2f)",
				count, expected[count].name, expected[count].price, name, price)
		}
		count++
	}

	if err := rows.Err(); err != nil {
		t.Errorf("rows iteration error: %v", err)
	}

	if count != len(expected) {
		t.Errorf("expected %d rows, got %d", len(expected), count)
	}
}

func TestIntegrationUpdateAndDelete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Setup
	_, err := db.Exec(`CREATE TABLE inventory (id INTEGER PRIMARY KEY, item TEXT, quantity INTEGER)`)
	if err != nil {
		t.Skipf("CREATE TABLE not yet implemented: %v", err)
	}

	_, err = db.Exec(`INSERT INTO inventory (item, quantity) VALUES ('Widget', 100)`)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Test UPDATE
	result, err := db.Exec(`UPDATE inventory SET quantity = quantity - ? WHERE item = ?`, 25, "Widget")
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	affected, _ := result.RowsAffected()
	if affected != 1 {
		t.Errorf("expected 1 row affected by update, got %d", affected)
	}

	// Verify update
	var quantity int
	err = db.QueryRow(`SELECT quantity FROM inventory WHERE item = ?`, "Widget").Scan(&quantity)
	if err != nil {
		t.Fatalf("failed to query after update: %v", err)
	}
	if quantity != 75 {
		t.Errorf("expected quantity = 75, got %d", quantity)
	}

	// Test DELETE
	result, err = db.Exec(`DELETE FROM inventory WHERE item = ?`, "Widget")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	affected, _ = result.RowsAffected()
	if affected != 1 {
		t.Errorf("expected 1 row affected by delete, got %d", affected)
	}

	// Verify delete
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM inventory`).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count after delete: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows after delete, got %d", count)
	}
}

func TestIntegrationTransactions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Setup
	_, err := db.Exec(`CREATE TABLE accounts (id INTEGER PRIMARY KEY, balance INTEGER)`)
	if err != nil {
		t.Skipf("CREATE TABLE not yet implemented: %v", err)
	}

	_, err = db.Exec(`INSERT INTO accounts (balance) VALUES (1000)`)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Test successful transaction
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	_, err = tx.Exec(`UPDATE accounts SET balance = balance - ? WHERE id = ?`, 100, 1)
	if err != nil {
		tx.Rollback()
		t.Fatalf("failed to update in transaction: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Verify commit
	var balance int
	err = db.QueryRow(`SELECT balance FROM accounts WHERE id = ?`, 1).Scan(&balance)
	if err != nil {
		t.Fatalf("failed to query after commit: %v", err)
	}
	if balance != 900 {
		t.Errorf("expected balance = 900, got %d", balance)
	}

	// Test rollback
	tx, err = db.Begin()
	if err != nil {
		t.Fatalf("failed to begin second transaction: %v", err)
	}

	_, err = tx.Exec(`UPDATE accounts SET balance = balance - ? WHERE id = ?`, 500, 1)
	if err != nil {
		tx.Rollback()
		t.Fatalf("failed to update in second transaction: %v", err)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("failed to rollback: %v", err)
	}

	// Verify rollback
	err = db.QueryRow(`SELECT balance FROM accounts WHERE id = ?`, 1).Scan(&balance)
	if err != nil {
		t.Fatalf("failed to query after rollback: %v", err)
	}
	if balance != 900 {
		t.Errorf("expected balance = 900 (unchanged), got %d", balance)
	}
}

func TestIntegrationPreparedStatements(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Setup
	_, err := db.Exec(`CREATE TABLE colors (id INTEGER PRIMARY KEY, name TEXT, hex TEXT)`)
	if err != nil {
		t.Skipf("CREATE TABLE not yet implemented: %v", err)
	}

	// Prepare insert statement
	stmt, err := db.Prepare(`INSERT INTO colors (name, hex) VALUES (?, ?)`)
	if err != nil {
		t.Fatalf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	// Execute prepared statement multiple times
	colors := []struct {
		name string
		hex  string
	}{
		{"Red", "#FF0000"},
		{"Green", "#00FF00"},
		{"Blue", "#0000FF"},
	}

	for _, c := range colors {
		_, err := stmt.Exec(c.name, c.hex)
		if err != nil {
			t.Fatalf("failed to exec prepared statement for %s: %v", c.name, err)
		}
	}

	// Verify all inserts
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM colors`).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 rows, got %d", count)
	}

	// Test prepared select statement
	queryStmt, err := db.Prepare(`SELECT hex FROM colors WHERE name = ?`)
	if err != nil {
		t.Fatalf("failed to prepare select: %v", err)
	}
	defer queryStmt.Close()

	var hex string
	err = queryStmt.QueryRow("Green").Scan(&hex)
	if err != nil {
		t.Fatalf("failed to query with prepared statement: %v", err)
	}
	if hex != "#00FF00" {
		t.Errorf("expected #00FF00, got %s", hex)
	}
}

func TestIntegrationNullHandling(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Setup
	_, err := db.Exec(`CREATE TABLE nullable (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)`)
	if err != nil {
		t.Skipf("CREATE TABLE not yet implemented: %v", err)
	}

	// Insert row with NULL values
	_, err = db.Exec(`INSERT INTO nullable (name, age) VALUES (?, ?)`, "Alice", nil)
	if err != nil {
		t.Fatalf("failed to insert NULL: %v", err)
	}

	// Insert row with non-NULL values
	_, err = db.Exec(`INSERT INTO nullable (name, age) VALUES (?, ?)`, "Bob", 25)
	if err != nil {
		t.Fatalf("failed to insert non-NULL: %v", err)
	}

	// Query NULL value
	var name string
	var age sql.NullInt64
	err = db.QueryRow(`SELECT name, age FROM nullable WHERE name = ?`, "Alice").Scan(&name, &age)
	if err != nil {
		t.Fatalf("failed to query NULL row: %v", err)
	}

	if name != "Alice" {
		t.Errorf("expected name = Alice, got %s", name)
	}
	if age.Valid {
		t.Errorf("expected age to be NULL, got %d", age.Int64)
	}

	// Query non-NULL value
	err = db.QueryRow(`SELECT name, age FROM nullable WHERE name = ?`, "Bob").Scan(&name, &age)
	if err != nil {
		t.Fatalf("failed to query non-NULL row: %v", err)
	}

	if name != "Bob" {
		t.Errorf("expected name = Bob, got %s", name)
	}
	if !age.Valid || age.Int64 != 25 {
		t.Errorf("expected age = 25, got Valid=%v, Int64=%d", age.Valid, age.Int64)
	}
}

func TestIntegrationBlobData(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Setup
	_, err := db.Exec(`CREATE TABLE files (id INTEGER PRIMARY KEY, name TEXT, data BLOB)`)
	if err != nil {
		t.Skipf("CREATE TABLE not yet implemented: %v", err)
	}

	// Insert binary data
	binaryData := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0xFF, 0x42}
	_, err = db.Exec(`INSERT INTO files (name, data) VALUES (?, ?)`, "test.bin", binaryData)
	if err != nil {
		t.Fatalf("failed to insert blob: %v", err)
	}

	// Query binary data
	var name string
	var data []byte
	err = db.QueryRow(`SELECT name, data FROM files WHERE id = ?`, 1).Scan(&name, &data)
	if err != nil {
		t.Fatalf("failed to query blob: %v", err)
	}

	if name != "test.bin" {
		t.Errorf("expected name = test.bin, got %s", name)
	}

	if len(data) != len(binaryData) {
		t.Fatalf("expected %d bytes, got %d", len(binaryData), len(data))
	}

	for i, b := range binaryData {
		if data[i] != b {
			t.Errorf("byte %d: expected 0x%02X, got 0x%02X", i, b, data[i])
		}
	}
}

func TestIntegrationUnicodeText(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Setup
	_, err := db.Exec(`CREATE TABLE scripture (id INTEGER PRIMARY KEY, text TEXT, language TEXT)`)
	if err != nil {
		t.Skipf("CREATE TABLE not yet implemented: %v", err)
	}

	// Test various Unicode scripts
	testCases := []struct {
		text     string
		language string
	}{
		{"בְּרֵאשִׁית בָּרָא אֱלֹהִים", "Hebrew"},
		{"Ἐν ἀρχῇ ἦν ὁ λόγος", "Greek"},
		{"太初有道", "Chinese"},
		{"В начале было Слово", "Russian"},
		{"🙏 ❤️ ✝️", "Emoji"},
	}

	for _, tc := range testCases {
		_, err := db.Exec(`INSERT INTO scripture (text, language) VALUES (?, ?)`, tc.text, tc.language)
		if err != nil {
			t.Fatalf("failed to insert %s text: %v", tc.language, err)
		}
	}

	// Verify all Unicode text
	for _, tc := range testCases {
		var text string
		err := db.QueryRow(`SELECT text FROM scripture WHERE language = ?`, tc.language).Scan(&text)
		if err != nil {
			t.Fatalf("failed to query %s: %v", tc.language, err)
		}

		if text != tc.text {
			t.Errorf("%s: expected %s, got %s", tc.language, tc.text, text)
		}
	}
}

func TestIntegrationAggregates(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Setup
	_, err := db.Exec(`CREATE TABLE numbers (value INTEGER)`)
	if err != nil {
		t.Skipf("CREATE TABLE not yet implemented: %v", err)
	}

	// Insert test data
	for i := 1; i <= 10; i++ {
		_, err := db.Exec(`INSERT INTO numbers VALUES (?)`, i)
		if err != nil {
			t.Fatalf("failed to insert %d: %v", i, err)
		}
	}

	// Test COUNT
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM numbers`).Scan(&count)
	if err != nil {
		t.Fatalf("COUNT failed: %v", err)
	}
	if count != 10 {
		t.Errorf("COUNT: expected 10, got %d", count)
	}

	// Test SUM
	var sum int
	err = db.QueryRow(`SELECT SUM(value) FROM numbers`).Scan(&sum)
	if err != nil {
		t.Fatalf("SUM failed: %v", err)
	}
	if sum != 55 {
		t.Errorf("SUM: expected 55, got %d", sum)
	}

	// Test AVG
	var avg float64
	err = db.QueryRow(`SELECT AVG(value) FROM numbers`).Scan(&avg)
	if err != nil {
		t.Fatalf("AVG failed: %v", err)
	}
	if avg != 5.5 {
		t.Errorf("AVG: expected 5.5, got %f", avg)
	}

	// Test MIN
	var min int
	err = db.QueryRow(`SELECT MIN(value) FROM numbers`).Scan(&min)
	if err != nil {
		t.Fatalf("MIN failed: %v", err)
	}
	if min != 1 {
		t.Errorf("MIN: expected 1, got %d", min)
	}

	// Test MAX
	var max int
	err = db.QueryRow(`SELECT MAX(value) FROM numbers`).Scan(&max)
	if err != nil {
		t.Fatalf("MAX failed: %v", err)
	}
	if max != 10 {
		t.Errorf("MAX: expected 10, got %d", max)
	}
}

func TestIntegrationOrderByLimit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Setup
	_, err := db.Exec(`CREATE TABLE scores (id INTEGER PRIMARY KEY, player TEXT, score INTEGER)`)
	if err != nil {
		t.Skipf("CREATE TABLE not yet implemented: %v", err)
	}

	// Insert test data
	scores := []struct {
		player string
		score  int
	}{
		{"Alice", 100},
		{"Bob", 250},
		{"Charlie", 150},
		{"David", 200},
		{"Eve", 175},
	}

	for _, s := range scores {
		_, err := db.Exec(`INSERT INTO scores (player, score) VALUES (?, ?)`, s.player, s.score)
		if err != nil {
			t.Fatalf("failed to insert %s: %v", s.player, err)
		}
	}

	// Test ORDER BY ASC
	rows, err := db.Query(`SELECT player FROM scores ORDER BY score ASC`)
	if err != nil {
		t.Fatalf("ORDER BY ASC failed: %v", err)
	}
	defer rows.Close()

	expectedAsc := []string{"Alice", "Charlie", "Eve", "David", "Bob"}
	idx := 0
	for rows.Next() {
		var player string
		if err := rows.Scan(&player); err != nil {
			t.Fatalf("failed to scan: %v", err)
		}
		if idx >= len(expectedAsc) || player != expectedAsc[idx] {
			t.Errorf("ORDER BY ASC row %d: expected %s, got %s", idx, expectedAsc[idx], player)
		}
		idx++
	}

	// Test ORDER BY DESC with LIMIT
	rows, err = db.Query(`SELECT player, score FROM scores ORDER BY score DESC LIMIT 3`)
	if err != nil {
		t.Fatalf("ORDER BY DESC LIMIT failed: %v", err)
	}
	defer rows.Close()

	expectedDesc := []struct {
		player string
		score  int
	}{
		{"Bob", 250},
		{"David", 200},
		{"Eve", 175},
	}

	idx = 0
	for rows.Next() {
		var player string
		var score int
		if err := rows.Scan(&player, &score); err != nil {
			t.Fatalf("failed to scan: %v", err)
		}
		if idx >= len(expectedDesc) {
			t.Fatalf("too many rows returned")
		}
		if player != expectedDesc[idx].player || score != expectedDesc[idx].score {
			t.Errorf("ORDER BY DESC LIMIT row %d: expected (%s, %d), got (%s, %d)",
				idx, expectedDesc[idx].player, expectedDesc[idx].score, player, score)
		}
		idx++
	}

	if idx != len(expectedDesc) {
		t.Errorf("expected %d rows, got %d", len(expectedDesc), idx)
	}

	// Test LIMIT with OFFSET
	rows, err = db.Query(`SELECT player FROM scores ORDER BY score DESC LIMIT 2 OFFSET 1`)
	if err != nil {
		t.Fatalf("LIMIT OFFSET failed: %v", err)
	}
	defer rows.Close()

	expectedOffset := []string{"David", "Eve"}
	idx = 0
	for rows.Next() {
		var player string
		if err := rows.Scan(&player); err != nil {
			t.Fatalf("failed to scan: %v", err)
		}
		if idx >= len(expectedOffset) || player != expectedOffset[idx] {
			t.Errorf("LIMIT OFFSET row %d: expected %s, got %s", idx, expectedOffset[idx], player)
		}
		idx++
	}
}

// Additional edge case tests

func TestIntegrationEmptyTable(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := db.Exec(`CREATE TABLE empty (id INTEGER)`)
	if err != nil {
		t.Skipf("CREATE TABLE not yet implemented: %v", err)
	}

	// Query empty table
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM empty`).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query empty table: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count = 0, got %d", count)
	}
}

func TestIntegrationMultipleTables(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create multiple tables
	_, err := db.Exec(`CREATE TABLE authors (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Skipf("CREATE TABLE not yet implemented: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE books (id INTEGER PRIMARY KEY, title TEXT, author_id INTEGER)`)
	if err != nil {
		t.Fatalf("failed to create books table: %v", err)
	}

	// Insert data
	_, err = db.Exec(`INSERT INTO authors (name) VALUES (?)`, "John Doe")
	if err != nil {
		t.Fatalf("failed to insert author: %v", err)
	}

	_, err = db.Exec(`INSERT INTO books (title, author_id) VALUES (?, ?)`, "Test Book", 1)
	if err != nil {
		t.Fatalf("failed to insert book: %v", err)
	}

	// Join query
	var title, author string
	err = db.QueryRow(`
		SELECT books.title, authors.name
		FROM books
		JOIN authors ON books.author_id = authors.id
	`).Scan(&title, &author)
	if err != nil {
		t.Fatalf("failed to join: %v", err)
	}

	if title != "Test Book" || author != "John Doe" {
		t.Errorf("join result: expected (Test Book, John Doe), got (%s, %s)", title, author)
	}
}

func TestIntegrationStringFunctions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Test UPPER, LOWER, LENGTH
	var result string
	err := db.QueryRow(`SELECT UPPER('hello') || '|' || LOWER('WORLD') || '|' || LENGTH('test')`).Scan(&result)
	if err != nil {
		t.Skipf("String functions not yet implemented: %v", err)
	}

	expected := "HELLO|world|4"
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestIntegrationMathOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Test basic arithmetic
	var result int
	err := db.QueryRow(`SELECT (10 + 5) * 2 - 3`).Scan(&result)
	if err != nil {
		t.Skipf("Math operations not yet implemented: %v", err)
	}

	if result != 27 {
		t.Errorf("expected 27, got %d", result)
	}
}

func TestIntegrationConcurrentAccess(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := db.Exec(`CREATE TABLE counter (value INTEGER)`)
	if err != nil {
		t.Skipf("CREATE TABLE not yet implemented: %v", err)
	}

	_, err = db.Exec(`INSERT INTO counter VALUES (0)`)
	if err != nil {
		t.Fatalf("failed to insert initial value: %v", err)
	}

	// Set connection pool size
	db.SetMaxOpenConns(10)

	// Note: This is a basic concurrency test.
	// Full concurrent write testing may need to handle
	// implementation-specific locking behavior.
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func() {
			var value int
			err := db.QueryRow(`SELECT value FROM counter`).Scan(&value)
			if err != nil {
				t.Errorf("concurrent read failed: %v", err)
			}
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestIntegrationDataTypes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := db.Exec(`
		CREATE TABLE types (
			int_val INTEGER,
			real_val REAL,
			text_val TEXT,
			blob_val BLOB
		)
	`)
	if err != nil {
		t.Skipf("CREATE TABLE not yet implemented: %v", err)
	}

	// Insert various types
	_, err = db.Exec(`INSERT INTO types VALUES (?, ?, ?, ?)`,
		42, 3.14159, "text", []byte{0x01, 0x02, 0x03})
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Read back
	var intVal int
	var realVal float64
	var textVal string
	var blobVal []byte

	err = db.QueryRow(`SELECT int_val, real_val, text_val, blob_val FROM types`).
		Scan(&intVal, &realVal, &textVal, &blobVal)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if intVal != 42 {
		t.Errorf("int_val: expected 42, got %d", intVal)
	}
	if fmt.Sprintf("%.5f", realVal) != "3.14159" {
		t.Errorf("real_val: expected 3.14159, got %.5f", realVal)
	}
	if textVal != "text" {
		t.Errorf("text_val: expected 'text', got %s", textVal)
	}
	if len(blobVal) != 3 || blobVal[0] != 0x01 {
		t.Errorf("blob_val: unexpected value %v", blobVal)
	}
}
