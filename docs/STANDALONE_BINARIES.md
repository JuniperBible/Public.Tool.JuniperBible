# Standalone Binaries and Internal Packages

This document describes the architecture of Juniper Bible's internal packages and the relationship between the main `capsule` CLI and its standalone API binary.

## Overview

Juniper Bible uses a unified CLI approach where all functionality is accessed through subcommands of the main `capsule` binary. The internal packages provide shared implementations used by the CLI subcommands.

## Architecture

### Main CLI: `capsule`

The primary entry point is `cmd/capsule/main.go`, which provides a unified interface to all Juniper Bible functionality through Kong-based command groups:

```bash
capsule web       # Start web UI server
capsule api       # Start REST API server
capsule juniper   # Bible/SWORD module tools
capsule format    # Format detection and conversion
capsule tools     # Tool execution
capsule plugins   # Plugin management
```

**Note:** Standalone binaries (`capsule-web`, `capsule-api`) have been removed. Use `capsule web` and `capsule api` subcommands instead.

### Internal Package Structure

All shared functionality lives in `internal/`, organized by purpose:

#### Server Packages

- **`internal/web/`** - Web UI server implementation
  - Embedded templates and static assets
  - Capsule browsing and Bible module viewing
  - Used by `capsule web` command

- **`internal/api/`** - REST API server implementation
  - JSON endpoints for capsule operations
  - Plugin discovery and execution
  - Used by `capsule api` command

- **`internal/server/`** - Shared server utilities
  - CORS middleware
  - Plugin configuration
  - Common HTTP helpers

#### Tool Packages

- **`internal/tools/repoman/`** - SWORD repository management
  - Embedded handler for repository operations
  - Registers as `tool.repoman` plugin
  - Used by `capsule juniper` commands

- **`internal/tools/hugo/`** - Hugo JSON output generation
- **`internal/tools/libsword/`** - libsword wrapper (legacy)
- **`internal/tools/pandoc/`** - Pandoc wrapper (legacy)
- **`internal/tools/calibre/`** - Calibre wrapper (legacy)
- **`internal/tools/usfm2osis/`** - USFM to OSIS converter (legacy)
- **`internal/tools/sqlite/`** - SQLite tool wrapper
- **`internal/tools/libxml2/`** - libxml2 wrapper (legacy)
- **`internal/tools/unrtf/`** - RTF converter wrapper (legacy)
- **`internal/tools/gobiblecreator/`** - GoBible JAR creator (legacy)

#### Format Packages

- **`internal/formats/`** - Embedded format handlers (33 total)
  - Each format has its own package: `swordpure/`, `esword/`, `osis/`, `usfm/`, etc.
  - All register themselves via `init()` when `internal/embedded` is imported
  - Used by format detection, ingest, and conversion operations

#### Embedded Plugin Registry

- **`internal/embedded/`** - Central registry for all embedded plugins
  - Imports all format and tool packages to trigger registration
  - Main CLI and standalone binaries import this to enable all plugins
  - Provides `Init()` function (currently a no-op, registration happens via `init()`)

#### Utility Packages

- **`internal/fileutil/`** - File operations (CopyDir, CopyFile)
- **`internal/archive/`** - Tar.gz archive creation for capsules

## Usage Examples

### Web UI Server

```bash
capsule web --port 8080 --capsules ./capsules --sword ~/.sword
```

This command:

- Starts an HTTP server on the specified port
- Serves the embedded web UI from `internal/web/templates/` and `internal/web/static/`
- Browses capsules in the specified directory
- Displays SWORD modules from the specified directory

### REST API Server

**Unified CLI:**
```bash
capsule api --port 8081 --capsules ./capsules
```

This command:

- Starts a REST API server on the specified port
- Provides JSON endpoints for capsule operations
- Supports plugin discovery and execution

### Bible/SWORD Module Tools (Juniper)

The `juniper` command group provides Bible-specific operations:

```bash
# List Bible modules in ~/.sword
capsule juniper list

# Ingest specific SWORD modules into capsules
capsule juniper ingest KJV ESV

# Ingest all Bible modules
capsule juniper ingest --all

# Convert CAS capsule back to SWORD format
capsule juniper cas-to-sword capsule.tar.gz --output ~/.sword --name KJV
```

**Note:** There is no standalone `capsule-juniper` binary. Juniper functionality is only available through the main `capsule` CLI.

### Format Detection and Conversion

```bash
# Detect file format
capsule format detect file.xml

# Convert between formats via IR
capsule format convert input.xml --output output.osis --target osis

# Extract IR from file
capsule format ir extract input.usfm --output ir/

# Emit native format from IR
capsule format ir emit ir/ --format osis --output output.osis
```

### Tool Execution

```bash
# List available tools
capsule tools list

# Run a tool plugin
capsule tools run tool-repoman list-sources

# Execute tool on artifact and store transcript
capsule tools execute capsule.tar.gz --tool repoman --command install
```

## Migration Guide

If you were using the standalone `capsule-web` binary, migrate as follows:

### From `capsule-web` (removed)

**Before:**
```bash
capsule-web --port 8080 --capsules ./capsules
```

**After:**
```bash
capsule web --port 8080 --capsules ./capsules
```

The `capsule-web` standalone binary has been removed. Use `capsule web` instead.

### Makefile Changes

The Makefile has been simplified:

```makefile
# Current targets
make build        # Builds main capsule CLI
make all          # Builds capsule + plugins + runs tests
```

## Plugin System Integration

### Embedded vs External Plugins

Juniper Bible supports two plugin loading strategies:

1. **Embedded Plugins** (default)
   - Compiled into the main binary
   - Registered via `internal/embedded` package
   - No external dependencies
   - Used by default in all binaries

2. **External Plugins** (opt-in)
   - Loaded from filesystem at runtime
   - Enabled with `--plugins-external` flag or `CAPSULE_PLUGINS_EXTERNAL=1` env var
   - Useful for development and testing
   - Can coexist with embedded plugins

Example:
```bash
# Use embedded plugins only (default)
capsule web --port 8080

# Enable loading external plugins from bin/plugins/
capsule web --port 8080 --plugins-external

# Or via environment variable
CAPSULE_PLUGINS_EXTERNAL=1 capsule web --port 8080
```

### Plugin Discovery Order

When external plugins are enabled:

1. Check embedded plugins first
2. Fall back to external plugins in `--plugins` directory
3. Log which plugins are loaded from where

## Internal Package API

### Web Server (`internal/web`)

```go
import "github.com/FocuswithJustin/mimicry/internal/web"

cfg := web.Config{
    Port:            8080,
    CapsulesDir:     "./capsules",
    PluginsDir:      "./bin/plugins",
    SwordDir:        "~/.sword",
    PluginsExternal: false,
}

if err := web.Start(cfg); err != nil {
    log.Fatal(err)
}
```

### REST API (`internal/api`)

```go
import "github.com/FocuswithJustin/mimicry/internal/api"

cfg := api.Config{
    Port:            8081,
    CapsulesDir:     "./capsules",
    PluginsDir:      "./plugins",
    PluginsExternal: false,
}

if err := api.Start(cfg); err != nil {
    log.Fatal(err)
}
```

### Embedded Plugin Registration

All embedded plugins register themselves automatically when `internal/embedded` is imported:

```go
import (
    // Import embedded plugins registry to register all embedded plugins
    _ "github.com/FocuswithJustin/mimicry/internal/embedded"
)
```

This triggers `init()` functions in all format and tool handlers, which call their respective `Register()` functions.

## Build System

### Main CLI

```bash
# Pure Go build (no CGO, embedded plugins only)
make build
# Output: bin/capsule

# With CGO SQLite support
make build-cgo
# Output: bin/capsule
```

### Build Everything

```bash
# Build main CLI + plugins + run tests
make all

# Build only plugins
make plugins
# Output: bin/plugins/format/* and bin/plugins/tool/*
```

## Current Architecture

All functionality is accessed through the unified `capsule` CLI:

- `capsule web` for web UI
- `capsule api` for REST API
- `capsule juniper` for Bible/SWORD operations
- `capsule format` for format detection and conversion
- `capsule tools` for tool execution

The internal packages provide the underlying implementation for these commands.

## Related Documentation

- [`docs/PLUGIN_DEVELOPMENT.md`](PLUGIN_DEVELOPMENT.md) - Plugin authoring guide
- [`docs/generated/CLI_REFERENCE.md`](generated/CLI_REFERENCE.md) - Complete CLI command reference
- [`docs/generated/API.md`](generated/API.md) - REST API endpoints
- [`docs/QUICK_START.md`](QUICK_START.md) - Getting started guide
- [`docs/BUILD_MODES.md`](BUILD_MODES.md) - SQLite driver selection and build options
