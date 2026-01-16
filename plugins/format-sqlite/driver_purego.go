//go:build !cgo_sqlite

package main

import (
	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
)

var sqliteDriver = sqlite.DriverName()
