//go:build !cgo_sqlite

package main

import (
	"github.com/JuniperBible/juniper/core/sqlite"
)

var sqliteDriver = sqlite.DriverName()
