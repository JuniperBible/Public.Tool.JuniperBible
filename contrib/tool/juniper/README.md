# Juniper (Legacy Reference)

This is the **legacy reference implementation** of the Juniper Bible toolkit.
It contains known bugs and is preserved for comparison testing against the
new pure Go implementation in `plugins/juniper/`.

## Purpose

This legacy implementation provides:

- **SWORD Module Parsing** - Read Bible, Commentary, and Dictionary modules
- **e-Sword Parsing** - Read e-Sword SQLite databases (.bblx, .cmtx, .dctx)
- **Hugo JSON Generation** - Generate Hugo-compatible JSON from Bible sources
- **Repository Management** - Download modules from SWORD repositories

## Known Issues

This implementation has bugs that the new `plugins/juniper/` fixes:

- Uses CGO-based SQLite (`mattn/go-sqlite3`) instead of pure Go
- Some versification edge cases not handled correctly
- Placeholder text detection may miss some patterns

## Building

From the repository root:

```bash
# Build legacy juniper CLI
make juniper-legacy

# Build legacy extract tool
make juniper-legacy-extract

# Build both
make juniper-legacy-all
```

Or manually:

```bash
cd src/
CGO_ENABLED=1 go build -o ../bin/juniper ./cmd/juniper
CGO_ENABLED=1 go build -o ../bin/extract ./cmd/extract
```

## Usage

```bash
# Diatheke-compatible verse lookup
./capsule-juniper-legacy diatheke -m KJV "Gen 1:1"

# Extract Bible to Hugo JSON
./capsule-juniper-legacy-extract --modules KJV --output ./data/bibles

# List available modules from a repository
./capsule-juniper-legacy list --source CrossWire
```

## Comparison Testing

Use this legacy implementation to verify the new plugins produce correct output:

```bash
# Build both implementations
make juniper-legacy
cd plugins/juniper/hugo && go build -o hugo .

# Compare output
./capsule-juniper-legacy-extract --modules KJV --output /tmp/legacy
# (use new hugo plugin to generate same output)
# diff the results
```

## New Implementation

The new pure Go implementation is in `plugins/juniper/`:

- `plugins/juniper/sword` - SWORD module parsing (pure Go, no CGO)
- `plugins/juniper/esword` - e-Sword database parsing (modernc.org/sqlite)
- `plugins/juniper/repoman` - Repository management
- `plugins/juniper/hugo` - Hugo JSON generation

The new implementation:

- Has no CGO dependencies
- Has 113 tests with full coverage
- Fixes versification bugs
- Is designed for git submodule extraction
