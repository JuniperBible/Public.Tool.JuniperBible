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

	// Should not panic for valid path
	db := MustOpen(dbPath)
	db.Close()
}

func TestDriverTypeConsistency(t *testing.T) {
	driverType := DriverType()

	switch driverType {
	case "purego":
		if IsCGO() {
			t.Error("IsCGO() should be false for purego driver")
		}
		if DriverName() != "sqlite" {
			t.Errorf("purego driver should use 'sqlite' name, got '%s'", DriverName())
		}
	case "cgo":
		if !IsCGO() {
			t.Error("IsCGO() should be true for cgo driver")
		}
		if DriverName() != "sqlite3" {
			t.Errorf("cgo driver should use 'sqlite3' name, got '%s'", DriverName())
		}
	default:
		t.Errorf("unknown driver type: %s", driverType)
	}
}

// Note: MustOpen panic path (lines 52-53) cannot be reliably tested.
// SQLite's sql.Open uses lazy initialization - it almost never returns an error
// from sql.Open() itself. Errors typically occur when actually using the connection.
// The panic path exists for safety but is unreachable with a registered SQLite driver.
// Coverage: 90.9% is the practical maximum for this package.
