# Integration Tests

This directory contains integration tests that verify capsule functionality with external tools.

## Prerequisites

Integration tests require external tools to be installed. Tests automatically skip if required tools are not available.

### Required Tools by Test File

| Test File | Required Tools |
|-----------|----------------|
| `sword_test.go` | diatheke, mod2imp, osis2mod (sword-utils) |
| `calibre_test.go` | ebook-convert, ebook-meta (calibre) |
| `pandoc_test.go` | pandoc |
| `libxml2_test.go` | xmllint, xsltproc (libxml2-utils) |
| `sqlite_test.go` | sqlite3 |
| `sample_data_test.go` | None (uses capsule library) |
| `juniper_plugin_test.go` | None (builds plugins) |

## Installation

### Ubuntu/Debian

```bash
sudo apt install sword-utils calibre pandoc libxml2-utils sqlite3
```

### macOS (Homebrew)

```bash
brew install sword calibre pandoc libxml2 sqlite
```

### NixOS

```bash
nix-shell -p sword calibre pandoc libxml2 sqlite
```

Or use the provided shell.nix:

```bash
nix-shell tests/integration/shell.nix
```

## Running Tests

### Run all integration tests

```bash
go test ./tests/integration/... -v
```

### Run specific test file

```bash
go test ./tests/integration -run TestSWORD -v
go test ./tests/integration -run TestPandoc -v
go test ./tests/integration -run TestCalibre -v
go test ./tests/integration -run TestXML -v
go test ./tests/integration -run TestSQLite -v
```

### Check which tools are available

```bash
go test ./tests/integration -run TestToolsAvailable -v
```

## Test Categories

### SWORD Tests (`sword_test.go`)

Tests SWORD Bible software tools:

- Module listing via diatheke
- Verse rendering
- Module creation with osis2mod

### Calibre Tests (`calibre_test.go`)

Tests Calibre e-book tools:

- HTML to EPUB conversion
- EPUB to MOBI conversion
- Metadata extraction

### Pandoc Tests (`pandoc_test.go`)

Tests Pandoc document converter:

- Markdown to HTML conversion
- Markdown to EPUB conversion
- Metadata extraction with templates

### libxml2 Tests (`libxml2_test.go`)

Tests XML processing tools:

- XML validation with xmllint
- XPath queries
- XSLT transformations with xsltproc

### SQLite Tests (`sqlite_test.go`)

Tests SQLite CLI:

- Database creation and queries
- CSV export
- Schema inspection
- Full-text search (FTS5)

## Writing New Tests

Use the `RequireTool` helper to skip tests when tools are missing:

```go
func TestMyTool(t *testing.T) {
    RequireTool(t, ToolPandoc)  // Skips if pandoc not installed

    // Your test code here
}
```

Define new tools in `tools.go`:

```go
var ToolMyTool = Tool{
    Name:        "mytool",
    Command:     "mytool",
    Args:        []string{"--version"},
    Description: "What mytool does",
}
```
