# CGO SQLite Driver (Optional)

This directory contains an optional CGO-based SQLite driver implementation using `github.com/mattn/go-sqlite3`.

## Important Notice

**This driver is NOT part of the default JuniperBible build.** The default build uses the internal pure Go SQLite implementation located at `core/sqlite/internal/driver`.

## When to Use This Driver

Use this CGO driver if you:
- Need maximum SQLite performance and are willing to deal with CGO complexity
- Have specific requirements that necessitate the C-based SQLite implementation
- Are comfortable managing CGO dependencies and build requirements

## Installation

To use this driver, you need to:

1. Copy `driver_cgo.go` to your local project or to `core/sqlite/`
2. Install the required dependency:
   ```bash
   go get github.com/mattn/go-sqlite3
   ```
3. Build with CGO enabled and the appropriate build tag:
   ```bash
   CGO_ENABLED=1 go build -tags cgo_sqlite
   ```

## Requirements

- CGO must be enabled (`CGO_ENABLED=1`)
- GCC or compatible C compiler must be installed
- SQLite C library headers may be required on some systems

## Performance

The CGO driver typically offers:
- Faster query execution for complex queries
- Better performance on large datasets
- Lower memory overhead in some scenarios

However, it comes with trade-offs:
- Requires C compiler toolchain
- Harder to cross-compile
- Platform-specific build requirements
- More complex deployment

## License

This driver uses `github.com/mattn/go-sqlite3` which is licensed under the MIT License.
See the main project's `THIRD_PARTY_LICENSES.md` for details.
