//go:build !cgo_sqlite

// Pure Go SQLite driver using modernc.org/sqlite.
// This is the default when CGO is disabled or cgo_sqlite tag is not set.
package sqlite

import (
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

const (
	driverName    = "sqlite"
	driverType    = "purego"
	driverPackage = "modernc.org/sqlite"
)
