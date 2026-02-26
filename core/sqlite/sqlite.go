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
	return sql.Open(driverName, dataSourceName)
}

// OpenReadOnly opens a SQLite database in read-only mode.
func OpenReadOnly(path string) (*sql.DB, error) {
	dsn := path + "?mode=ro"
	return Open(dsn)
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
