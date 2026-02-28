# SQLite Package Documentation

The `core/sqlite` package provides a unified SQLite interface using Public.Lib.Anthony, a pure Go SQLite implementation.

## Usage

### Opening Databases

```go
import "github.com/JuniperBible/Public.Tool.JuniperBible/core/sqlite"

// Open for read-write
db, err := sqlite.Open("path/to/database.db")

// Open read-only
db, err := sqlite.OpenReadOnly("path/to/database.db")
```

### Transaction Support

For bulk operations, use transactions for 10-100x better performance:

```go
err := sqlite.WithTransaction(db, func(tx *sql.Tx) error {
    for _, item := range items {
        _, err := tx.Exec("INSERT INTO table (...) VALUES (...)", ...)
        if err != nil {
            return err
        }
    }
    return nil
})
```

### Bulk Write Configuration

For large write operations:

```go
db, _ := sqlite.Open(path)
sqlite.ConfigureForBulkWrite(db) // Enables WAL, optimizes cache
sqlite.EnableForeignKeys(db)     // Enable FK constraints
```

## API Reference

- `Open(path string) (*sql.DB, error)` - Open database for read-write
- `OpenReadOnly(path string) (*sql.DB, error)` - Open database read-only
- `WithTransaction(db *sql.DB, fn func(*sql.Tx) error) error` - Execute in transaction
- `ConfigureForBulkWrite(db *sql.DB) error` - Optimize for bulk writes
- `EnableForeignKeys(db *sql.DB) error` - Enable FK constraints
- `DriverName() string` - Returns driver name ("sqlite_internal")
- `DriverType() string` - Returns "purego"
- `IsCGO() bool` - Always returns false
- `GetInfo() Info` - Returns driver information

## Implementation

Uses Public.Lib.Anthony, a comprehensive pure Go SQLite implementation with:
- Full SQL support (CTEs, subqueries, window functions)
- ACID transactions with WAL support
- No CGO dependencies
