# Pure Go SQLite Implementation

This directory documents JuniperBible's pure Go SQLite database engine, a complete from-scratch implementation of SQLite in pure Go.

## Overview

JuniperBible includes a custom-built SQLite database engine implemented entirely in Go. This allows the application to:

- Run without CGO dependencies
- Cross-compile to any platform
- Maintain reproducible builds
- Provide transparent, educational code

The implementation is based on the SQLite 3.51.2 reference specification and aims for full compatibility with the SQLite file format and SQL dialect.

## Documentation

### [Architecture](./ARCHITECTURE.md)
System architecture, component diagram, data flow, and package dependencies.

### [API Reference](./API.md)
Public API documentation for the `core/sqlite` package.

### [Implementation Details](./IMPLEMENTATION.md)
File format compatibility, SQL support matrix, limitations, and performance characteristics.

### [Migration Guide](./MIGRATION.md)
Guide for migrating from modernc.org/sqlite or mattn/go-sqlite3, with API compatibility notes.

## Quick Start

```go
package main

import (
    "log"

    "github.com/FocuswithJustin/JuniperBible/core/sqlite"
)

func main() {
    // Open a database
    db, err := sqlite.Open("mydata.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Use standard database/sql operations
    _, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`)
    if err != nil {
        log.Fatal(err)
    }

    _, err = db.Exec(`INSERT INTO users (name) VALUES (?)`, "John Doe")
    if err != nil {
        log.Fatal(err)
    }
}
```

## Build Modes

The project supports both pure Go and CGO-based SQLite implementations:

| Mode | Default | CGO Required | Build Command |
|------|---------|--------------|---------------|
| Pure Go (internal) | Yes | No | `go build` |
| CGO (mattn/go-sqlite3) | No | Yes | `CGO_ENABLED=1 go build -tags cgo_sqlite` |

See [Build Modes](../BUILD_MODES.md) for details.

## Key Features

- **Pure Go**: No CGO dependencies required
- **SQLite Compatible**: Reads and writes standard SQLite 3.x database files
- **Standard API**: Uses Go's `database/sql` interface
- **Transparent**: Educational, readable codebase
- **Modular**: Clean separation of concerns (pager, btree, VDBE, parser, etc.)
- **Tested**: Comprehensive test coverage with divergence testing

## Components

The implementation consists of several internal packages:

- **driver**: `database/sql` driver implementation
- **pager**: Page cache and file I/O management
- **btree**: B-tree storage engine
- **parser**: SQL parser (lexer, AST)
- **vdbe**: Virtual Database Engine (bytecode interpreter)
- **planner**: Query planner and optimizer
- **expr**: Expression evaluation
- **functions**: Built-in SQL functions (75+ functions)
- **utf**: UTF-8/UTF-16 encoding and collation
- **format**: File format constants and structures
- **schema**: Schema table management

## Status

This is a working implementation with support for:

- [x] Core SQL operations (SELECT, INSERT, UPDATE, DELETE)
- [x] Table creation and DDL
- [x] Transactions (ACID compliant with rollback journal)
- [x] Indexes
- [x] String, math, aggregate, and date/time functions
- [x] UTF-8 text encoding
- [x] B-tree storage with proper page management
- [x] Query optimization and planning

Limitations (compared to full SQLite):

- No WAL (Write-Ahead Logging) support
- Simplified file locking
- No memory-mapped I/O
- Limited virtual table support
- No FTS (Full-Text Search)

See [IMPLEMENTATION.md](./IMPLEMENTATION.md) for a complete feature matrix.

## Performance

The pure Go implementation provides reasonable performance for most use cases:

- Suitable for embedded databases in applications
- Good performance for typical workloads
- Optimized for simplicity and correctness over maximum speed
- Page caching reduces I/O overhead

For performance-critical applications, the CGO build mode using mattn/go-sqlite3 is available.

## Testing

Comprehensive testing ensures compatibility:

```bash
# Run all tests
go test ./core/sqlite/...

# Run divergence tests (verify identical behavior between drivers)
go test ./core/sqlite/... -run Divergence

# Run with coverage
go test -cover ./core/sqlite/...
```

## Contributing

Contributions are welcome! When adding features:

1. Consult the SQLite specification for behavior
2. Update the parser for new syntax
3. Add VDBE opcodes as needed
4. Implement in the appropriate component
5. Add comprehensive tests
6. Update documentation

## References

- [SQLite File Format](https://www.sqlite.org/fileformat.html)
- [SQLite Architecture](https://www.sqlite.org/arch.html)
- [SQLite SQL Syntax](https://www.sqlite.org/lang.html)
- [SQLite VDBE Opcodes](https://www.sqlite.org/opcode.html)

## License

This implementation is part of JuniperBible and follows the project's licensing.
