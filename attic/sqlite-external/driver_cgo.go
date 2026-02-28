//go:build cgo_sqlite

// Package sqliteexternal provides a CGO-based SQLite driver using mattn/go-sqlite3.
// This is an optional external dependency for performance-critical applications.
//
// Build with: go build -tags cgo_sqlite
// Requires: CGO_ENABLED=1
package sqliteexternal

import (
	_ "github.com/mattn/go-sqlite3" // CGO SQLite driver
)

const (
	// DriverName is the SQL driver name to use with database/sql.
	DriverName = "sqlite3"

	// DriverType identifies this as the CGO implementation.
	DriverType = "cgo"

	// DriverPackage is the import path of the underlying driver.
	DriverPackage = "github.com/mattn/go-sqlite3"
)
