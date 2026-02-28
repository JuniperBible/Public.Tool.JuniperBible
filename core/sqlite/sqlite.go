// Package sqlite provides a unified SQLite interface that wraps the Anthony
// pure Go SQLite driver with MichaelCore-specific functionality.
//
// This package delegates to the Anthony library (Public.Lib.Anthony) for all
// SQLite operations, while providing additional transaction helpers and
// database configuration utilities specific to MichaelCore's needs.
//
// Use Open() instead of sql.Open() to ensure the correct driver is used.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/JuniperBible/Public.Lib.Anthony"
)

// DriverName returns the SQL driver name to use.
// This delegates to Anthony's driver name.
func DriverName() string {
	return anthony.DriverName
}

// DriverType returns a string identifying the underlying implementation.
// Always returns "anthony" since this package now exclusively uses the Anthony driver.
func DriverType() string {
	return "anthony"
}

// IsCGO returns true if the CGO implementation is being used.
// Always returns false since Anthony is a pure Go implementation.
func IsCGO() bool {
	return false
}

// Open opens a SQLite database using the Anthony driver.
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
		DriverName: DriverName(),
		DriverType: DriverType(),
		IsCGO:      IsCGO(),
		Package:    "github.com/JuniperBible/Public.Lib.Anthony",
	}
}

// WithTransaction executes a function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// Otherwise, the transaction is committed.
//
// Example:
//
//	err := sqlite.WithTransaction(ctx, db, func(tx *sql.Tx) error {
//	    _, err := tx.Exec("INSERT INTO users (name) VALUES (?)", "Alice")
//	    return err
//	})
func WithTransaction(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction error: %w (rollback error: %v)", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// ConfigureForBulkWrite optimizes a SQLite database connection for bulk write operations.
// This should be called before performing large batch inserts or updates.
//
// Settings applied:
//   - WAL mode for better concurrent write performance
//   - NORMAL synchronous mode (faster, still safe with WAL)
//   - Increased cache size (10MB)
//   - 64MB temp store in memory
func ConfigureForBulkWrite(ctx context.Context, db *sql.DB) error {
	settings := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-10000", // 10MB
		"PRAGMA temp_store=MEMORY",
		"PRAGMA temp_store_size=67108864", // 64MB
	}

	for _, pragma := range settings {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("failed to execute %s: %w", pragma, err)
		}
	}

	return nil
}

// EnableForeignKeys enables foreign key constraint enforcement.
// SQLite disables foreign keys by default for backwards compatibility.
// This should be called after opening a database if you need foreign key support.
func EnableForeignKeys(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON")
	if err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	return nil
}
