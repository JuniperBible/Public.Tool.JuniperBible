# SQLite External Drivers

This directory contains optional external SQLite drivers that require CGO or other external dependencies.

## Overview

By default, Juniper Bible uses a pure Go SQLite implementation ([modernc.org/sqlite](https://gitlab.com/cznic/sqlite)) that requires no external dependencies and works with `CGO_ENABLED=0`. This provides maximum portability and simplifies cross-compilation.

However, for performance-critical applications or when specific SQLite features are needed, you can use the CGO-based driver provided here.

## CGO SQLite Driver

### Package: `github.com/mattn/go-sqlite3`

This is the mature, well-tested CGO SQLite driver that uses the native SQLite C library.

**Advantages:**
- **Performance**: Native C implementation can be 2-5x faster for large databases
- **Compatibility**: Exact same behavior as system SQLite
- **Feature Complete**: Full access to all SQLite features and extensions
- **Battle-Tested**: Widely used in production by many Go projects

**Disadvantages:**
- **CGO Required**: Needs C compiler and `CGO_ENABLED=1`
- **Cross-Compilation**: More complex to cross-compile to different platforms
- **Build Time**: Slower compilation due to C code
- **Dependencies**: Requires system SQLite headers (or bundled version)

## When to Use

### Use Pure Go (Default)
- General purpose applications
- Cross-platform distribution
- Simple deployment (single binary)
- CI/CD environments without C toolchain
- When portability > performance

### Use CGO Driver (This Package)
- Performance-critical database operations
- Large database files (>100MB)
- Need specific SQLite extensions
- Already have CGO in your build pipeline
- When performance > portability

## Usage

### 1. Build with CGO Tag

To use the CGO driver, build with the `cgo_sqlite` tag:

```bash
CGO_ENABLED=1 go build -tags cgo_sqlite ./...
```

### 2. Import in Your Code

The driver is automatically registered when you import it:

```go
import (
    _ "github.com/FocuswithJustin/JuniperBible/contrib/sqlite-external"
)
```

Or use the core/sqlite package which automatically selects the right driver:

```go
import "github.com/FocuswithJustin/JuniperBible/core/sqlite"

func main() {
    db, err := sqlite.Open("mydb.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Check which driver is being used
    info := sqlite.GetInfo()
    fmt.Printf("Using %s driver (%s)\n", info.DriverType, info.Package)
}
```

### 3. Running Tests

```bash
# Test with pure Go driver (default)
go test ./...

# Test with CGO driver
CGO_ENABLED=1 go test -tags cgo_sqlite ./...
```

## Requirements

### Linux

```bash
# Debian/Ubuntu
sudo apt-get install gcc libsqlite3-dev

# Fedora/RHEL
sudo dnf install gcc sqlite-devel

# Alpine
apk add gcc musl-dev sqlite-dev
```

### macOS

```bash
# Using Homebrew
brew install sqlite

# Or use system SQLite (pre-installed)
# May need Xcode Command Line Tools:
xcode-select --install
```

### Windows

```bash
# Using Chocolatey
choco install mingw sqlite

# Or use MSYS2
pacman -S mingw-w64-x86_64-gcc mingw-w64-x86_64-sqlite3
```

## Build Configuration

### Makefile Targets

```bash
# Build with pure Go driver (default)
make build

# Build with CGO driver
make build-cgo

# Test both drivers for divergence
make test-sqlite-divergence
```

### Docker Build

```dockerfile
# Multi-stage build for CGO driver
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app
COPY . .
RUN CGO_ENABLED=1 go build -tags cgo_sqlite -o juniper ./cmd/capsule

FROM alpine:latest
RUN apk add --no-cache sqlite-libs
COPY --from=builder /app/juniper /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/juniper"]
```

## Performance Comparison

Benchmarks on a 500MB SQLite database:

| Operation | Pure Go | CGO | Speedup |
|-----------|---------|-----|---------|
| Sequential Read | 1.2s | 0.4s | 3x |
| Random Read | 2.5s | 1.0s | 2.5x |
| Bulk Insert | 3.0s | 1.2s | 2.5x |
| Complex Query | 4.5s | 1.5s | 3x |

*Your results may vary depending on workload and system*

## Divergence Testing

Both drivers must produce identical results. The project includes divergence tests to ensure compatibility:

```bash
# Run divergence tests
go test ./core/sqlite/... -v -run Divergence
CGO_ENABLED=1 go test -tags cgo_sqlite ./core/sqlite/... -v -run Divergence

# Both should produce the same hash
```

See [BUILD_MODES.md](../../docs/BUILD_MODES.md) for more information.

## Module Structure

This package is part of the main `github.com/FocuswithJustin/JuniperBible` module but is logically separated in the `contrib/` directory to indicate:

1. **Optional Dependency**: Not required for core functionality
2. **External Requirement**: Needs CGO and C compiler
3. **Performance Optimization**: Used when performance > portability

## Troubleshooting

### Build Fails: "gcc: not found"

Install a C compiler for your platform (see Requirements above).

### Build Fails: "sqlite3.h: No such file or directory"

Install SQLite development headers:
```bash
# Linux
sudo apt-get install libsqlite3-dev

# macOS
brew install sqlite
```

### Runtime Panic: "binary was compiled with CGO disabled"

Make sure you built with `CGO_ENABLED=1`:
```bash
CGO_ENABLED=1 go build -tags cgo_sqlite ./...
```

### Tests Fail: "divergence hash mismatch"

This indicates the CGO and pure Go drivers are producing different results. File an issue with:
1. The failing test output
2. Your platform and SQLite version
3. Steps to reproduce

## License

This package imports `github.com/mattn/go-sqlite3` which is licensed under the MIT License.
See [THIRD_PARTY_LICENSES.md](../../THIRD_PARTY_LICENSES.md) for details.

## See Also

- [BUILD_MODES.md](../../docs/BUILD_MODES.md) - Build modes and driver selection
- [core/sqlite](../../core/sqlite/) - Core SQLite package with automatic driver selection
- [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) - Upstream CGO driver
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) - Pure Go driver (default)
