// Package sqliteexternal provides optional external SQLite drivers.
//
// This package is part of the main github.com/FocuswithJustin/JuniperBible module
// and provides CGO-based SQLite drivers for performance-critical applications.
//
// # CGO SQLite Driver
//
// To use the CGO driver (github.com/mattn/go-sqlite3):
//
//	import _ "github.com/FocuswithJustin/JuniperBible/contrib/sqlite-external"
//
// Build with:
//
//	CGO_ENABLED=1 go build -tags cgo_sqlite
//
// # Default Pure Go Driver
//
// By default, Juniper Bible uses a pure Go SQLite implementation that requires
// no CGO. See github.com/FocuswithJustin/JuniperBible/core/sqlite for details.
//
// # When to Use
//
// Use this package when:
//   - Performance is critical (2-5x faster for large databases)
//   - You need specific SQLite extensions
//   - You already have CGO in your build pipeline
//
// Use the default pure Go driver when:
//   - Portability is important
//   - Cross-compilation is required
//   - You want simpler deployment (single binary)
//
// See README.md for detailed usage instructions and examples.
package sqliteexternal
