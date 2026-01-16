//go:build cgo_sqlite

// CGO SQLite driver using mattn/go-sqlite3.
// This is used when the cgo_sqlite build tag is set.
//
// Build with: go build -tags cgo_sqlite
// Requires: CGO_ENABLED=1
package sqlite

import (
	_ "github.com/mattn/go-sqlite3" // CGO SQLite driver
)

const (
	driverName    = "sqlite3"
	driverType    = "cgo"
	driverPackage = "github.com/mattn/go-sqlite3"
)
