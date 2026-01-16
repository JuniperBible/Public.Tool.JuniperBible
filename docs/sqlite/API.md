# SQLite API Reference

This document describes the public API of the `core/sqlite` package.

## Package: `github.com/FocuswithJustin/JuniperBible/core/sqlite`

The `sqlite` package provides a unified interface for SQLite database access, supporting both pure Go and CGO implementations.

## Functions

### Open

```go
func Open(dataSourceName string) (*sql.DB, error)
```

Opens a SQLite database using the appropriate driver. This is the preferred way to open SQLite databases.

**Parameters:**
- `dataSourceName`: Path to the database file, or `:memory:` for in-memory database

**Returns:**
- `*sql.DB`: Standard database handle from `database/sql`
- `error`: Error if the database cannot be opened

**Example:**
```go
db, err := sqlite.Open("mydata.db")
if err != nil {
    log.Fatal(err)
}
defer db.Close()
```

**Data Source Name (DSN) Options:**

```go
// File database
sqlite.Open("mydata.db")

// In-memory database
sqlite.Open(":memory:")

// Read-only mode
sqlite.Open("mydata.db?mode=ro")

// Read-write mode (default)
sqlite.Open("mydata.db?mode=rw")

// Read-write-create mode
sqlite.Open("mydata.db?mode=rwc")

// With cache mode
sqlite.Open("mydata.db?cache=shared")
sqlite.Open("mydata.db?cache=private")
```

---

### OpenReadOnly

```go
func OpenReadOnly(path string) (*sql.DB, error)
```

Opens a SQLite database in read-only mode. The database file must already exist.

**Parameters:**
- `path`: Path to the database file

**Returns:**
- `*sql.DB`: Standard database handle in read-only mode
- `error`: Error if the database cannot be opened

**Example:**
```go
db, err := sqlite.OpenReadOnly("readonly.db")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// This will fail - database is read-only
_, err = db.Exec("INSERT INTO users VALUES (1, 'Alice')")
// err != nil
```

---

### MustOpen

```go
func MustOpen(dataSourceName string) *sql.DB
```

Opens a SQLite database and panics on error. Use `Open` instead if you need to handle errors gracefully.

This is intended for use in tests or initialization code where database access failure is unrecoverable.

**Parameters:**
- `dataSourceName`: Path to the database file

**Returns:**
- `*sql.DB`: Standard database handle

**Panics:**
- If the database cannot be opened

**Example:**
```go
// In test code or initialization
db := sqlite.MustOpen(":memory:")
defer db.Close()
```

---

### DriverName

```go
func DriverName() string
```

Returns the SQL driver name to use. This is always "sqlite" for the pure Go driver and "sqlite3" for the CGO driver.

**Returns:**
- `string`: Driver name ("sqlite" or "sqlite3")

**Example:**
```go
name := sqlite.DriverName()
fmt.Println(name) // "sqlite" (pure Go) or "sqlite3" (CGO)
```

---

### DriverType

```go
func DriverType() string
```

Returns a string identifying the underlying implementation.

**Returns:**
- `"purego"`: Pure Go implementation (default)
- `"cgo"`: CGO implementation with mattn/go-sqlite3

**Example:**
```go
driverType := sqlite.DriverType()
fmt.Println(driverType) // "purego" or "cgo"
```

---

### IsCGO

```go
func IsCGO() bool
```

Returns true if the CGO implementation is being used.

**Returns:**
- `bool`: true if using CGO driver, false if using pure Go driver

**Example:**
```go
if sqlite.IsCGO() {
    fmt.Println("Using CGO SQLite driver")
} else {
    fmt.Println("Using pure Go SQLite driver")
}
```

---

### GetInfo

```go
func GetInfo() Info
```

Returns information about the current SQLite configuration.

**Returns:**
- `Info`: Structure containing driver information

**Example:**
```go
info := sqlite.GetInfo()
fmt.Printf("Driver: %s (%s)\n", info.DriverName, info.DriverType)
fmt.Printf("Package: %s\n", info.Package)
fmt.Printf("Is CGO: %v\n", info.IsCGO)
```

---

## Types

### Info

```go
type Info struct {
    DriverName string `json:"driver_name"` // SQL driver name
    DriverType string `json:"driver_type"` // "purego" or "cgo"
    IsCGO      bool   `json:"is_cgo"`      // true if CGO driver
    Package    string `json:"package"`     // Import path of driver package
}
```

Contains information about the SQLite driver configuration.

**Fields:**

- `DriverName`: The name used with `sql.Open()` ("sqlite" or "sqlite3")
- `DriverType`: Implementation type ("purego" or "cgo")
- `IsCGO`: Whether the CGO implementation is in use
- `Package`: Full import path of the driver package

---

## Usage Patterns

### Basic Database Operations

```go
package main

import (
    "log"

    "github.com/FocuswithJustin/JuniperBible/core/sqlite"
)

func main() {
    // Open database
    db, err := sqlite.Open("example.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create table
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY,
            name TEXT NOT NULL,
            email TEXT
        )
    `)
    if err != nil {
        log.Fatal(err)
    }

    // Insert data
    result, err := db.Exec(
        "INSERT INTO users (name, email) VALUES (?, ?)",
        "Alice",
        "alice@example.com",
    )
    if err != nil {
        log.Fatal(err)
    }

    id, _ := result.LastInsertId()
    log.Printf("Inserted user with ID: %d", id)

    // Query data
    rows, err := db.Query("SELECT id, name, email FROM users")
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()

    for rows.Next() {
        var id int64
        var name, email string
        if err := rows.Scan(&id, &name, &email); err != nil {
            log.Fatal(err)
        }
        log.Printf("User: %d, %s, %s", id, name, email)
    }
}
```

### Transactions

```go
// Begin transaction
tx, err := db.Begin()
if err != nil {
    log.Fatal(err)
}

// Execute operations
_, err = tx.Exec("INSERT INTO users (name) VALUES (?)", "Bob")
if err != nil {
    tx.Rollback()
    log.Fatal(err)
}

_, err = tx.Exec("UPDATE users SET email = ? WHERE name = ?", "bob@example.com", "Bob")
if err != nil {
    tx.Rollback()
    log.Fatal(err)
}

// Commit transaction
if err := tx.Commit(); err != nil {
    log.Fatal(err)
}
```

### Prepared Statements

```go
// Prepare statement
stmt, err := db.Prepare("INSERT INTO users (name, email) VALUES (?, ?)")
if err != nil {
    log.Fatal(err)
}
defer stmt.Close()

// Execute multiple times
for _, user := range users {
    _, err := stmt.Exec(user.Name, user.Email)
    if err != nil {
        log.Fatal(err)
    }
}
```

### In-Memory Database

```go
// Create in-memory database
db, err := sqlite.Open(":memory:")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Use like a regular database
_, err = db.Exec("CREATE TABLE test (id INTEGER, value TEXT)")
// ...
```

### Read-Only Access

```go
// Open read-only database
db, err := sqlite.OpenReadOnly("readonly.db")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Can read data
rows, err := db.Query("SELECT * FROM users")
// ...

// Cannot write data
_, err = db.Exec("INSERT INTO users VALUES (1, 'Alice')")
// err != nil (database is read-only)
```

### Connection Pool Configuration

```go
db, err := sqlite.Open("mydata.db")
if err != nil {
    log.Fatal(err)
}

// Set maximum open connections
db.SetMaxOpenConns(25)

// Set maximum idle connections
db.SetMaxIdleConns(5)

// Set connection lifetime
db.SetConnMaxLifetime(5 * time.Minute)
```

### Error Handling

```go
db, err := sqlite.Open("mydata.db")
if err != nil {
    log.Fatal("Failed to open database:", err)
}
defer db.Close()

// Check if table exists
_, err = db.Exec("SELECT 1 FROM users LIMIT 1")
if err != nil {
    // Table doesn't exist, create it
    _, err = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
    if err != nil {
        log.Fatal("Failed to create table:", err)
    }
}
```

## Standard database/sql Features

Since the package returns a standard `*sql.DB`, all `database/sql` features are available:

- **Query/Exec**: Execute SQL statements
- **Prepare**: Prepare statements for reuse
- **Begin/Commit/Rollback**: Transaction support
- **QueryRow**: Query single row
- **Scan**: Type conversion
- **NULL handling**: sql.NullString, sql.NullInt64, etc.
- **Connection pooling**: Automatic connection management

See the [database/sql documentation](https://pkg.go.dev/database/sql) for complete details.

## Type Mapping

SQLite types map to Go types as follows:

| SQLite Type | Go Type |
|-------------|---------|
| INTEGER | int64 |
| REAL | float64 |
| TEXT | string |
| BLOB | []byte |
| NULL | nil |

Use sql.Null* types for nullable columns:

```go
var name sql.NullString
err := db.QueryRow("SELECT name FROM users WHERE id = ?", 1).Scan(&name)
if name.Valid {
    fmt.Println(name.String)
} else {
    fmt.Println("NULL")
}
```

## Build Tags

The driver selection is controlled by build tags:

```bash
# Pure Go driver (default)
go build

# CGO driver
CGO_ENABLED=1 go build -tags cgo_sqlite
```

See [Build Modes](../BUILD_MODES.md) for details.

## Thread Safety

All functions in this package are thread-safe and can be called concurrently from multiple goroutines. The underlying `database/sql` package handles connection pooling and synchronization.

## See Also

- [ARCHITECTURE.md](./ARCHITECTURE.md) - System architecture
- [IMPLEMENTATION.md](./IMPLEMENTATION.md) - Implementation details
- [MIGRATION.md](./MIGRATION.md) - Migration guide
- [database/sql documentation](https://pkg.go.dev/database/sql)
