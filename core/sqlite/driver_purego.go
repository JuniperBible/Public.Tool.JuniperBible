//go:build !cgo_sqlite

// Pure Go SQLite driver using Anthony (Public.Lib.Anthony).
// This is the default build configuration for MichaelCore.
package sqlite

import (
	_ "github.com/JuniperBible/Public.Lib.Anthony" // Anthony SQLite driver
)
