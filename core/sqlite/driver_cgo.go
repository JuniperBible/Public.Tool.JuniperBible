//go:build cgo_sqlite

// CGO SQLite driver using mattn/go-sqlite3.
// This is used when the cgo_sqlite build tag is set.
//
// Build with: go build -tags cgo_sqlite
// Requires: CGO_ENABLED=1
//
// The actual driver implementation is in contrib/sqlite-external
// to clearly separate optional external dependencies from core functionality.
package sqlite

import (
	_ "github.com/FocuswithJustin/JuniperBible/contrib/sqlite-external" // CGO SQLite driver
)

const (
	driverName    = "sqlite3"
	driverType    = "cgo"
	driverPackage = "github.com/mattn/go-sqlite3 (via contrib/sqlite-external)"
)
