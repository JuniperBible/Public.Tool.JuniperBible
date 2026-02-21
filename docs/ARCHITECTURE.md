# Juniper Bible Architecture

## Overview

Juniper Bible uses a **canonical package architecture** to eliminate code duplication while supporting both embedded and standalone plugin modes. As of 2025, the deduplication project has reduced code duplication from 50%+ to under 10%.

## Core Principles

1. **Single Source of Truth**: Each format has exactly ONE canonical implementation
2. **Zero Duplication**: Format logic lives in canonical packages, not copied across plugins
3. **Dual Mode Support**: Same code works embedded (fast) and standalone (portable)
4. **Build Tags**: Go build tags control embedded vs standalone compilation

## Directory Structure

```
core/formats/<name>/         # Canonical format implementations (42 packages)
├── format.go                # Format-specific logic (parse, emit, detect)
└── register.go              # Embedded registration (with build tag)

plugins/format-<name>/       # Thin standalone wrappers (~10 lines each)
└── main.go                  # Imports canonical package, calls format.Run()

plugins/ipc/                 # Canonical IPC protocol (PRESERVED)
├── protocol.go              # Request, Response types
├── results.go               # DetectResult, IngestResult, etc.
├── ir.go                    # Corpus, Document, ContentBlock
├── detect_helpers.go        # StandardDetect, CheckExtension
├── args.go                  # StringArg, IntArg, BoolArg
└── handlers.go              # HandleDetect, StandardIngest

plugins/sdk/                 # SDK built on top of plugins/ipc
├── format/                  # Config + Run() for format plugins
├── ir/                      # IR read/write helpers
├── blob/                    # Content-addressed storage helpers
├── runtime/                 # Dispatcher for command routing
└── errors/                  # Typed error handling

internal/formats/base/       # Base utilities (PRESERVED)
```

## Canonical Package Pattern

### File: `core/formats/<name>/format.go`

Contains all format-specific logic:

```go
package myformat

import (
    "github.com/JuniperBible/juniper/plugins/ipc"
    "github.com/JuniperBible/juniper/plugins/sdk/format"
    "github.com/JuniperBible/juniper/plugins/sdk/ir"
)

// Config is the canonical configuration for this format
var Config = &format.Config{
    PluginID:      "format.myformat",
    Name:          "MyFormat",
    Version:       "1.0.0",
    Extensions:    []string{".mf"},
    LossClass:     "L1",
    CanExtractIR:  true,
    CanEmitNative: true,
    Detect:        detect,
    Parse:         parse,
    Emit:          emit,
    Enumerate:     enumerate,
}

func detect(path string) (*ipc.DetectResult, error) {
    return ipc.StandardDetect(path, "myformat",
        []string{".mf"},
        []string{"MYFORMAT_MARKER"},
    ), nil
}

func parse(path string) (*ir.Corpus, error) {
    // Format-specific parsing logic (~80 lines)
    // Converts native format to IR
}

func emit(corpus *ir.Corpus, outputDir string) (string, error) {
    // Format-specific emission logic (~60 lines)
    // Converts IR to native format
}

func enumerate(path string) (*ipc.EnumerateResult, error) {
    // List components within the file
}
```

**Key points**:
- All format logic lives here
- No IPC protocol handling (SDK handles that)
- Focused on format-specific operations only
- Typical size: 150-300 lines (down from 600-800 duplicated lines)

### File: `core/formats/<name>/register.go`

Handles embedded registration:

```go
//go:build !standalone

package myformat

func init() {
    Config.RegisterEmbedded()
}
```

**Key points**:
- Build tag `!standalone` means this only compiles in embedded mode
- Automatically registers format when package is imported
- Single line of actual code

## Standalone Wrapper Pattern

### File: `plugins/format-<name>/main.go`

Thin wrapper for standalone execution:

```go
//go:build standalone

package main

import (
    "github.com/JuniperBible/juniper/core/formats/myformat"
    "github.com/JuniperBible/juniper/plugins/sdk/format"
)

func main() {
    format.Run(myformat.Config)
}
```

**Key points**:
- Build tag `standalone` means this only compiles with `-tags standalone`
- Imports canonical package
- Delegates to SDK's `format.Run()`
- Typical size: 10-12 lines
- 98% reduction from original 600-800 line implementations

## Build Tag System

### Embedded Mode (Default)

```bash
go build ./cmd/capsule
```

- Compiles: `core/formats/<name>/register.go` (has `!standalone` tag)
- Skips: `plugins/format-<name>/main.go` (has `standalone` tag)
- Result: All formats embedded in single binary
- Speed: Fast (no subprocess overhead)
- Size: Larger binary (all formats included)

### Standalone Mode

```bash
go build -tags standalone ./plugins/format-json/
```

- Compiles: `plugins/format-json/main.go` (has `standalone` tag)
- Skips: `core/formats/json/register.go` (has `!standalone` tag)
- Uses: `core/formats/json/format.go` (no tag, always compiled)
- Result: Single plugin executable
- Speed: Slower (subprocess + IPC)
- Size: Smaller per-plugin (only one format)

## Dependency Chain

```
┌─────────────────────────────────────────────────────────────┐
│ Thin Wrapper (main.go, ~10 lines, build tag: standalone)    │
│   plugins/format-<name>/main.go                             │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ Canonical Format Package (format.go, 150-300 lines)         │
│   core/formats/<name>/format.go                             │
│   - Config variable                                          │
│   - detect(), parse(), emit(), enumerate()                  │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ Plugin SDK (plugins/sdk/format)                              │
│   - Run() function (command routing, IPC handling)          │
│   - Config struct                                            │
│   - RegisterEmbedded()                                       │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ IPC Protocol Package (plugins/ipc) - CANONICAL              │
│   - Request, Response types                                  │
│   - DetectResult, IngestResult, etc.                         │
│   - Corpus, Document, ContentBlock (IR types)                │
│   - StandardDetect(), StandardIngest() helpers               │
└─────────────────────────────────────────────────────────────┘
```

## IPC Architecture (Preserved)

**IMPORTANT**: `plugins/ipc/` is the canonical IPC package and was preserved during deduplication.

### Why IPC was Preserved

- Foundation for all plugin communication
- Defines Request, Response, DetectResult, IngestResult
- Contains IR types (Corpus, Document, ContentBlock)
- Provides helper functions (StandardDetect, StandardIngest)
- Used by both SDK and canonical packages

### What was Removed

- Duplicated IPC types in standalone plugins (was in 32 plugins)
- Duplicated IPC types in embedded plugins (was in 42 plugins)
- Duplicated IPC types in internal handlers (was in 42 handlers)

All now import from the single canonical `plugins/ipc/` package.

## Format Plugin Lifecycle

### 1. Embedded Execution (Fast Path)

```
User runs: capsule format detect sample.json
                    │
                    ▼
         Core binary checks embedded registry
                    │
                    ▼
         Finds core/formats/json (registered via init())
                    │
                    ▼
         Calls json.Config.Detect(path) directly
                    │
                    ▼
         Returns DetectResult (no IPC, no subprocess)
```

### 2. Standalone Execution (Compatibility Path)

```
User runs: format-json (standalone plugin)
                    │
                    ▼
         Reads JSON request from stdin
                    │
                    ▼
         format.Run() parses request
                    │
                    ▼
         Routes to json.Config.Detect(path)
                    │
                    ▼
         Writes JSON response to stdout
```

## Testing Strategy

### Canonical Package Tests

Tests live in canonical packages:

```
core/formats/<name>/format_test.go
```

**Benefits**:
- Single test suite per format
- No duplicated test code
- Tests the actual implementation
- Works for both embedded and standalone modes

### Integration Tests

Test actual plugin execution:

```bash
# Build standalone plugin
go build -tags standalone -o /tmp/format-json plugins/format-json/

# Test via IPC
echo '{"command":"detect","args":{"path":"test.json"}}' | /tmp/format-json
```

## Migration Status

### Completed

- [x] Phase 1: SDK Infrastructure (RegisterEmbedded, test utilities)
- [x] Phase 2: Canonical Format Packages (42 packages created)
- [x] Phase 4: Test Consolidation (tests moved to canonical packages)

### In Progress

- [ ] Phase 3: Convert Standalone Plugins to Thin Wrappers (ongoing)

### Pending

- [ ] Phase 5: CI Enforcement (after Phase 3 completes)
- [ ] Delete `plugins/format/*/` (42 embedded duplicates)
- [ ] Delete `internal/formats/*/` (42 internal duplicates, except base/)

## Impact Summary

| Category | Before | After | Reduction |
|----------|--------|-------|-----------|
| Standalone plugins (33) | ~20,000 lines | ~400 lines | 98% |
| Embedded plugins (42) | ~70,000 lines | 0 (deleted) | 100% |
| Internal handlers (42) | ~48,000 lines | 0 (deleted) | 100% |
| Canonical formats | 0 | ~8,000 lines | N/A |
| Test files | ~45,000 lines | ~5,000 lines | 89% |
| **Total** | ~183,000 lines | ~13,400 lines | **93%** |

**Final duplication**: <5% (only format-specific logic remains unique)

## Key Locations

### Canonical Implementations

```
core/formats/accordance/
core/formats/bibletime/
core/formats/crosswire/
core/formats/dbl/
core/formats/dir/
core/formats/ecm/
core/formats/epub/
core/formats/esword/
core/formats/file/
core/formats/flex/
core/formats/gobible/
core/formats/html/
core/formats/json/
core/formats/logos/
core/formats/markdown/
core/formats/morphgnt/
core/formats/mybible/
core/formats/mysword/
core/formats/na28app/
core/formats/odf/
core/formats/olive/
core/formats/onlinebible/
core/formats/oshb/
core/formats/osis/
core/formats/pdb/
core/formats/rtf/
core/formats/sblgnt/
core/formats/sfm/
core/formats/sqlite/
core/formats/sword/
core/formats/swordpure/
core/formats/tar/
core/formats/tei/
core/formats/theword/
core/formats/tischendorf/
core/formats/txt/
core/formats/usfm/
core/formats/usx/
core/formats/xml/
core/formats/zefania/
core/formats/zip/
```

### Foundation Packages (Preserved)

```
plugins/ipc/              # Canonical IPC protocol
plugins/sdk/format/       # SDK for format plugins
internal/formats/base/    # Base utilities
```

## Developer Guidelines

### Adding a New Format

1. Create canonical package: `core/formats/<name>/format.go`
2. Define Config with detect, parse, emit functions
3. Create register file: `core/formats/<name>/register.go`
4. Write tests: `core/formats/<name>/format_test.go`
5. Create thin wrapper: `plugins/format-<name>/main.go` (~10 lines)

### Modifying an Existing Format

1. Edit canonical package: `core/formats/<name>/format.go`
2. Update tests: `core/formats/<name>/format_test.go`
3. No changes needed to wrapper (it just delegates)

### Testing

```bash
# Test canonical package
go test core/formats/json/...

# Test standalone plugin
go build -tags standalone -o /tmp/format-json plugins/format-json/
echo '{"command":"detect","args":{"path":"test.json"}}' | /tmp/format-json

# Test embedded mode
go test ./cmd/capsule/...
```

## References

- **Deduplication Plan**: `docs/DEDUPLICATION_PLAN.md`
- **Plugin Development**: `docs/PLUGIN_DEVELOPMENT.md`
- **Thin Wrapper Migration**: `docs/THIN_WRAPPER_MIGRATION.md`
- **SDK Documentation**: `plugins/sdk/README.md`
- **IPC Protocol**: `plugins/ipc/PROTOCOL.md`
