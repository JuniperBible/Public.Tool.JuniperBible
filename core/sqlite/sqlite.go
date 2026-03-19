// Package sqlite provides a pure Go SQLite interface using Public.Lib.Anthony.
//
// The driver registers as "sqlite_internal" and provides full SQLite functionality
// without any CGO dependencies.
//
// Use Open() instead of sql.Open() to ensure the correct driver is used.
package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/cyanitol/Public.Lib.Anthony"
)

// DriverName returns the SQL driver name to use.
// This is always "sqlite" for compatibility.
func DriverName() string {
	return driverName
}

// DriverType returns a string identifying the underlying implementation.
// Always returns "purego" as CGO support has been removed.
func DriverType() string {
	return driverType
}

// IsCGO returns true if the CGO implementation is being used.
// Always returns false as CGO support has been removed.
func IsCGO() bool {
	return false
}

// Open opens a SQLite database using the appropriate driver.
// This is the preferred way to open SQLite databases.
func Open(dataSourceName string) (*sql.DB, error) {
	return anthony.Open(dataSourceName)
}

// OpenReadOnly opens a SQLite database in read-only mode.
func OpenReadOnly(path string) (*sql.DB, error) {
	return anthony.OpenReadOnly(path)
}

// MustOpen opens a SQLite database and returns an error if it fails.
// Deprecated: Use Open instead for clearer error handling semantics.
func MustOpen(dataSourceName string) (*sql.DB, error) {
	db, err := Open(dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("sqlite: failed to open %s: %w", dataSourceName, err)
	}
	return db, nil
}

// Info contains information about the SQLite driver configuration.
type Info struct {
	DriverName string `json:"driver_name"`
	DriverType string `json:"driver_type"`
	IsCGO      bool   `json:"is_cgo"`
	Package    string `json:"package"`
}

// GetInfo returns information about the current SQLite configuration.
func GetInfo() Info {
	return Info{
		DriverName: driverName,
		DriverType: driverType,
		IsCGO:      IsCGO(),
		Package:    driverPackage,
	}
}

// WithTransaction wraps a function in a transaction for bulk operations.
// If fn returns an error, the transaction is rolled back.
// If fn succeeds, the transaction is committed.
func WithTransaction(db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ConfigureForBulkWrite sets pragmas optimized for bulk writing operations.
// Enables WAL mode, sets synchronous to NORMAL, and increases cache size.
func ConfigureForBulkWrite(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-64000",
		"PRAGMA temp_store=MEMORY",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("exec %s: %w", pragma, err)
		}
	}
	return nil
}

// Optimize runs PRAGMA optimize to update query planner statistics.
// Call after bulk writes to ensure the query planner has up-to-date info.
// Returns nil if the driver does not support this pragma.
func Optimize(db *sql.DB) error {
	_, err := db.Exec("PRAGMA optimize")
	if err != nil {
		// Some drivers block PRAGMA optimize for security reasons.
		// This is a best-effort optimization, not a correctness requirement.
		return nil
	}
	return nil
}

// ValidateIntegrity runs PRAGMA integrity_check and returns an error
// if the database is corrupted.
func ValidateIntegrity(db *sql.DB) error {
	var result string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("integrity check failed: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity check failed: %s", result)
	}
	return nil
}

// EnableForeignKeys enables foreign key constraint checking.
func EnableForeignKeys(db *sql.DB) error {
	_, err := db.Exec("PRAGMA foreign_keys=ON")
	return err
}
