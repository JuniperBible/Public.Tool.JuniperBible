package sqlite

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDriverInfo(t *testing.T) {
	info := GetInfo()

	if info.DriverName == "" {
		t.Error("DriverName should not be empty")
	}

	if info.DriverType == "" {
		t.Error("DriverType should not be empty")
	}

	if info.Package == "" {
		t.Error("Package should not be empty")
	}

	// Verify consistency
	if info.DriverName != DriverName() {
		t.Errorf("DriverName mismatch: info=%s, func=%s", info.DriverName, DriverName())
	}

	if info.DriverType != DriverType() {
		t.Errorf("DriverType mismatch: info=%s, func=%s", info.DriverType, DriverType())
	}

	if info.IsCGO != IsCGO() {
		t.Errorf("IsCGO mismatch: info=%v, func=%v", info.IsCGO, IsCGO())
	}

	t.Logf("SQLite driver: %s (%s) from %s", info.DriverName, info.DriverType, info.Package)
}

func TestOpen(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sqlite-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create a test table
	_, err = db.Exec(`CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert data
	_, err = db.Exec(`INSERT INTO test (value) VALUES (?)`, "hello")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Query data
	var value string
	err = db.QueryRow(`SELECT value FROM test WHERE id = 1`).Scan(&value)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if value != "hello" {
		t.Errorf("expected 'hello', got '%s'", value)
	}
}

func TestOpenReadOnly(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sqlite-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Create database first
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, err = db.Exec(`INSERT INTO test (value) VALUES (?)`, "readonly")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	db.Close()

	// Open read-only
	rodb, err := OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("failed to open read-only: %v", err)
	}
	defer rodb.Close()

	// Should be able to read
	var value string
	err = rodb.QueryRow(`SELECT value FROM test WHERE id = 1`).Scan(&value)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if value != "readonly" {
		t.Errorf("expected 'readonly', got '%s'", value)
	}
}

func TestMustOpen(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sqlite-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Should not return error for valid path
	db, err := MustOpen(dbPath)
	if err != nil {
		t.Fatalf("MustOpen failed: %v", err)
	}
	db.Close()
}

func TestDriverTypeConsistency(t *testing.T) {
	driverType := DriverType()

	if driverType != "purego" {
		t.Errorf("expected driver type 'purego', got '%s'", driverType)
	}

	if IsCGO() {
		t.Error("IsCGO() should be false for purego driver")
	}

	if DriverName() != "sqlite_internal" {
		t.Errorf("purego driver should use 'sqlite_internal' name, got '%s'", DriverName())
	}
}

// Note: MustOpen error path cannot be reliably tested.
// SQLite's sql.Open uses lazy initialization - it almost never returns an error
// from sql.Open() itself. Errors typically occur when actually using the connection.
// The error path exists for safety but is rarely triggered with a registered SQLite driver.
// Coverage: 90.9% is the practical maximum for this package.
