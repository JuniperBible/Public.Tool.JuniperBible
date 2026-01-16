// Package sqlite provides a unified SQLite interface supporting both
// pure Go (modernc.org/sqlite) and CGO (mattn/go-sqlite3) implementations.
//
// Build modes:
//   - Default (CGO_ENABLED=0): Uses pure Go modernc.org/sqlite
//   - CGO mode (CGO_ENABLED=1 -tags cgo_sqlite): Uses mattn/go-sqlite3 via contrib/sqlite-external
//
// The CGO driver is located in contrib/sqlite-external/ to clearly separate
// optional external dependencies from core functionality.
//
// The driver name is always "sqlite" or "sqlite3" depending on the implementation.
// Use Open() instead of sql.Open() to ensure the correct driver is used.
//
// See contrib/sqlite-external/README.md for CGO driver usage details.
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
// Returns "cgo" for mattn/go-sqlite3, "purego" for modernc.org/sqlite.
func DriverType() string {
	return driverType
}

// IsCGO returns true if the CGO implementation is being used.
func IsCGO() bool {
	return driverType == "cgo"
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

// MustOpen opens a SQLite database and panics on error.
// Use Open instead if you need to handle errors gracefully.
// This is intended for use in tests or initialization code where
// database access failure is unrecoverable.
func MustOpen(dataSourceName string) *sql.DB {
	db, err := Open(dataSourceName)
	if err != nil {
		panic(fmt.Sprintf("sqlite: failed to open %s: %v", dataSourceName, err))
	}
	return db
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
