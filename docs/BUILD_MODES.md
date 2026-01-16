# Build Modes

Juniper Bible supports multiple build modes to balance between portability and performance.

## SQLite Driver Selection

The project supports two SQLite implementations:

| Driver | Package | Build Tag | CGO Required | Default |
|--------|---------|-----------|--------------|---------|
| Pure Go | modernc.org/sqlite | (none) | No | Yes |
| CGO | github.com/mattn/go-sqlite3 | `cgo_sqlite` | Yes | No |

### Default Build (Pure Go)

```bash
# Standard build - no CGO required
go build ./...

# Explicitly disable CGO
CGO_ENABLED=0 go build ./...

# Run tests
go test ./...
```

### CGO Build (Optional)

```bash
# Build with CGO SQLite
CGO_ENABLED=1 go build -tags cgo_sqlite ./...

# Run tests with CGO
CGO_ENABLED=1 go test -tags cgo_sqlite ./...
```

## Why Two Drivers?

### Pure Go (Default)
- **Portability**: Works on any platform without C compiler
- **Cross-compilation**: Easy cross-compilation to any GOOS/GOARCH
- **Reproducibility**: Deterministic builds
- **CI/CD**: Simpler CI pipelines without CGO toolchains

### CGO (Optional - in contrib/sqlite-external)
- **Performance**: Native SQLite may be faster for large databases (2-5x)
- **Compatibility**: Exact same behavior as system SQLite
- **Features**: Access to SQLite extensions (if needed)
- **Location**: Separated in `contrib/sqlite-external/` to clearly mark as optional external dependency

See [contrib/sqlite-external/README.md](../contrib/sqlite-external/README.md) for detailed CGO driver documentation.

## Divergence Prevention

Both drivers must produce identical results. The `core/sqlite` package includes:

1. **Divergence Tests**: Tests that run identical operations and compare results
2. **Hash Verification**: A hash of all test outputs that must match between drivers
3. **Golden Results**: Expected values that both drivers must produce

### Running Divergence Tests

```bash
# Test with pure Go driver
go test ./core/sqlite/... -v -run Divergence

# Test with CGO driver
CGO_ENABLED=1 go test -tags cgo_sqlite ./core/sqlite/... -v -run Divergence
```

### Current Divergence Hash

Both drivers must produce this hash:
```
e2fbdfdc9e33fac6b4e2812c044689135c749e4d70f5d2850e1a4ac4205849f5
```

If a driver produces a different hash, the `TestDivergenceHash` test will fail.

## Plugin Build Modes

### Main Module Plugins

Plugins in the main module (`plugins/format/sqlite`, `plugins/format/esword`) use the `core/sqlite` package and automatically support both build modes.

### Standalone Plugins

Plugins with separate `go.mod` files (`plugins/format/logos`, `plugins/format/accordance`) have their own driver selection files:

- `driver_purego.go` - Pure Go driver (default)
- `driver_cgo.go` - CGO driver (with `cgo_sqlite` tag)

Build with:
```bash
# Pure Go
cd plugins/format/logos && go build

# CGO
cd plugins/format/logos && CGO_ENABLED=1 go build -tags cgo_sqlite
```

## Makefile Targets

```bash
# Build everything (pure Go)
make build

# Build with CGO SQLite
make build-cgo

# Test with both drivers
make test-sqlite-divergence
```

## CI Integration

The CI pipeline should test both build modes:

```yaml
jobs:
  test-purego:
    runs-on: ubuntu-latest
    steps:
      - run: CGO_ENABLED=0 go test ./...

  test-cgo:
    runs-on: ubuntu-latest
    steps:
      - run: CGO_ENABLED=1 go test -tags cgo_sqlite ./...

  verify-divergence:
    runs-on: ubuntu-latest
    steps:
      - name: Test Pure Go
        run: go test ./core/sqlite/... -v -run DivergenceHash | tee purego.txt
      - name: Test CGO
        run: CGO_ENABLED=1 go test -tags cgo_sqlite ./core/sqlite/... -v -run DivergenceHash | tee cgo.txt
      - name: Compare hashes
        run: |
          grep "Divergence hash:" purego.txt > hash1.txt
          grep "Divergence hash:" cgo.txt > hash2.txt
          diff hash1.txt hash2.txt
```

## Embedded vs External Plugins

Juniper Bible supports two plugin modes that affect how plugins are loaded and executed.

### Embedded Plugins (Default)

By default, all plugins are embedded directly into the main binaries:

| Binary | Includes |
|--------|----------|
| `capsule` | All format and tool plugins |
| `capsule-web` | All format and tool plugins |
| `capsule-api` | All format and tool plugins |

**Benefits:**
- Single-binary deployment
- No external dependencies
- Faster plugin execution (no subprocess overhead)
- More secure (no external code execution)

**Build:**
```bash
# Standard build includes embedded plugins
make build
make web
make api
```

### External Plugins (Optional)

External plugins can be enabled for custom or premium plugins:

```bash
# Enable via command-line flag
./capsule-web --plugins-external

# Enable via environment variable
CAPSULE_PLUGINS_EXTERNAL=1 ./capsule-web
```

When external plugins are enabled:
1. Embedded plugins are still tried first
2. If no embedded handler found, external plugins in `./plugins/` are used
3. External plugins communicate via JSON IPC (stdin/stdout)

### Distribution Builds

Distribution packages come in two variants:

| Variant | SQLite Driver | Dependencies | Best For |
|---------|--------------|--------------|----------|
| **purego** | modernc.org/sqlite | ~35 Go packages | Portability, cross-compilation |
| **cgo** | mattn/go-sqlite3 | 0 Go packages | Performance, native SQLite |

```bash
# Build all distributions (pure-Go for all platforms, CGO for linux/darwin)
make dist

# Build pure-Go only (all 6 platforms)
make dist-purego

# Build CGO only (current platform)
make dist-cgo

# Build both variants for current platform
make dist-local

# Output structure:
dist/
├── juniper-bible-0.5.1-linux-amd64-purego.tar.gz
├── juniper-bible-0.5.1-linux-amd64-cgo.tar.gz
├── juniper-bible-0.5.1-linux-arm64-purego.tar.gz
├── juniper-bible-0.5.1-darwin-amd64-purego.tar.gz
├── juniper-bible-0.5.1-darwin-amd64-cgo.tar.gz
├── juniper-bible-0.5.1-darwin-arm64-purego.tar.gz
├── juniper-bible-0.5.1-windows-amd64-purego.tar.gz
└── juniper-bible-0.5.1-windows-arm64-purego.tar.gz  # Windows: pure-Go only
```

Each distribution contains:
- **bin/**: Main binaries with all plugins embedded
- **plugins/**: Noop placeholder for custom/premium plugins
- **capsules/**: Sample Bible capsules (if available)

### Plugin Development

For plugin development with external plugins:

```bash
# Build plugins to bin/plugins/
make plugins

# Run with external plugins enabled
CAPSULE_PLUGINS_EXTERNAL=1 ./bin/capsule-web -port 8080 -plugins ./bin/plugins
```

See [PLUGIN_DEVELOPMENT.md](PLUGIN_DEVELOPMENT.md) for details on creating embedded or external plugins.
