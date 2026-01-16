//go:build !cgo_sqlite

package sqlite

import (
	_ "github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/driver"
)

const (
	driverName    = "sqlite"
	driverType    = "purego"
	driverPackage = "github.com/FocuswithJustin/JuniperBible/core/sqlite/internal"
)
