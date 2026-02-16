# Plugin IPC Package

This package provides shared types and utilities for plugin development, eliminating code duplication across 33+ format plugins.

## Contents

### Protocol Types (`protocol.go`)

- `Request/Response`: Core IPC message structure
- `DetectResult`, `IngestResult`, `EnumerateResult`, `EnumerateEntry`: Standard command results
- `ReadRequest()`, `Respond()`, `RespondError()`, `MustRespond()`: IPC helpers

### IR Types (`ir.go`)
Shared Intermediate Representation types used across all plugins:

- `Corpus`, `Document`, `ContentBlock`: Core IR structure
- `Token`, `Anchor`, `Span`, `Ref`: Stand-off markup types
- `ParallelCorpus`, `Alignment`, `InterlinearLine`: Advanced IR types

### Result Types (`results.go`)
IR conversion result types:

- `ExtractIRResult`: Result of extract-ir command
- `EmitNativeResult`: Result of emit-native command
- `LossReport`, `LostElement`: Loss classification (L0-L4)

### Argument Helpers (`args.go`)

- `StringArg()`, `StringArgOr()`: Extract string arguments
- `BoolArg()`, `IntArg()`: Extract typed arguments
- `PathAndOutputDir()`: Extract common path/output_dir pair
- `StoreBlob()`: Content-addressed storage helper
- `ArtifactIDFromPath()`: Generate artifact IDs from paths

### Detect Helpers (`detect_helpers.go`)
Standardized detection utilities that reduce detect handler code from 40+ lines to 5-10 lines:

**Check Functions** (return bool):

- `CheckExtension(path, extensions...)`: Check file extension (case-insensitive)
- `CheckMagicBytes(path, magic)`: Check file header bytes
- `CheckContentContains(path, substrings...)`: Check for all substrings in file
- `CheckContentContainsAny(path, substrings...)`: Check for any substring in file

**Detect Functions** (return `*DetectResult`):

- `DetectByExtension(path, format, extensions...)`: Extension-only detection
- `DetectByMagicBytes(path, format, magic)`: Magic byte detection
- `DetectByContent(path, format, substrings...)`: Content detection (all patterns)
- `DetectByContentAny(path, format, substrings...)`: Content detection (any pattern)
- `StandardDetect(path, format, extensions, contentPatterns)`: Two-stage detection (most common pattern)

**Result Constructors**:

- `DetectSuccess(format, reason)`: Create successful detection
- `DetectFailure(reason)`: Create failed detection

Example - before (40+ lines):
```go
func handleDetect(args map[string]interface{}) {
    path, ok := args["path"].(string)
    if !ok {
        ipc.RespondError("path argument required")
        return
    }
    ext := strings.ToLower(filepath.Ext(path))
    if ext != ".xml" {
        ipc.MustRespond(&ipc.DetectResult{Detected: false, Reason: "not an .xml file"})
        return
    }
    data, err := os.ReadFile(path)
    if err != nil {
        ipc.MustRespond(&ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot read: %v", err)})
        return
    }
    content := string(data)
    if strings.Contains(content, "<bible") {
        ipc.MustRespond(&ipc.DetectResult{Detected: true, Format: "XML", Reason: "XML Bible detected"})
        return
    }
    ipc.MustRespond(&ipc.DetectResult{Detected: false, Reason: "no Bible XML found"})
}
```

Example - after (7 lines):
```go
func handleDetect(args map[string]interface{}) {
    path, err := ipc.StringArg(args, "path")
    if err != nil {
        ipc.RespondError(err.Error())
        return
    }
    result := ipc.StandardDetect(path, "XML", []string{".xml"}, []string{"<bible"})
    ipc.MustRespond(result)
}
```

### Common Handlers (`handlers.go`)
Reusable handler patterns:

- `HandleDetect()`: Generic file detection
- `HandleIngest()`: Generic file ingestion
- `HandleEnumerateSingleFile()`: Generic enumeration
- `ComputeHash()`, `ComputeSourceHash()`: Hash utilities

### Escaping (`escape.go`)
XML/HTML entity encoding:

- `EscapeHTML()`, `UnescapeHTML()`
- `EscapeXML()`, `UnescapeXML()`

## SDK Integration

The IPC package provides the foundational protocol types used by all plugins, whether they use the SDK or implement the protocol directly.

### Architecture Overview

**IPC Package (`plugins/ipc`)**: Core protocol layer

- Defines protocol types: `Request`, `Response`, `DetectResult`, `IngestResult`, `EnumerateResult`, `EnumerateEntry`
- Provides IR types: `Corpus`, `Document`, `ContentBlock`, `Token`, etc.
- Offers helper functions: `ReadRequest()`, `Respond()`, argument extraction, detection helpers
- Used by both SDK-based and non-SDK plugins

**SDK Package (`plugins/sdk/format`)**: High-level wrapper

- Wraps IPC types for easier plugin development
- Provides `FormatHandler` interface with simplified method signatures
- Handles IPC communication automatically
- Offers test helpers for plugin development
- Built on top of the IPC package

### Key Types

**Protocol Types** (defined in `protocol.go`):

- `Request`: IPC request with command and arguments
- `Response`: IPC response with success/error status and data
- `DetectResult`: Detection result with format name and confidence
- `IngestResult`: Ingestion result with content hash and artifact ID
- `EnumerateResult`: Enumeration result with entry list
- `EnumerateEntry`: Single enumerable entry (path, type, size)

**IR Types** (defined in `ir.go`):

- See "IR Types" section above for complete list

### Plugin Development Approaches

**Option 1: Direct IPC Usage** (non-SDK plugins)

- Implement protocol directly using IPC package types
- Use `main.go` with build tag `//go:build !sdk`
- Full control over protocol handling
- More boilerplate code but maximum flexibility

Example structure:
```go
//go:build !sdk

package main

import "github.com/FocuswithJustin/mimicry/plugins/ipc"

func main() {
    req, err := ipc.ReadRequest()
    if err != nil {
        ipc.RespondError(err.Error())
        return
    }

    switch req.Command {
    case "detect":
        handleDetect(req.Args)
    // ...
    }
}
```

**Option 2: SDK-Based Plugins** (recommended for new plugins)

- Implement `FormatHandler` interface from SDK package
- Use `main_sdk.go` with build tag `//go:build sdk`
- SDK handles IPC communication automatically
- Less boilerplate, focus on format-specific logic

Example structure:
```go
//go:build sdk

package main

import "github.com/FocuswithJustin/mimicry/plugins/sdk/format"

type MyFormatHandler struct{}

func (h *MyFormatHandler) Detect(path string) (bool, string, error) {
    // Detection logic using IPC helpers
    return true, "MyFormat", nil
}

func main() {
    format.Main(&MyFormatHandler{})
}
```

### Build Tag Separation

Build tags ensure proper separation between SDK and non-SDK implementations:

- `main.go` with `//go:build !sdk`: Direct IPC implementation
- `main_sdk.go` with `//go:build sdk`: SDK wrapper implementation

This allows the same plugin to support both modes, with the build system selecting the appropriate implementation based on build flags.

## Usage Example

```go
package main

import (
    "encoding/json"
    "github.com/FocuswithJustin/mimicry/plugins/ipc"
    "os"
)

func main() {
    var req ipc.Request
    if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
        ipc.RespondErrorf("failed to decode request: %v", err)
        return
    }

    switch req.Command {
    case "detect":
        handleDetect(req.Args)
    case "ingest":
        handleIngest(req.Args)
    default:
        ipc.RespondErrorf("unknown command: %s", req.Command)
    }
}

func handleDetect(args map[string]interface{}) {
    ipc.HandleDetect(args, []string{".json"}, []string{"\"meta\""}, "JSON")
}

func handleIngest(args map[string]interface{}) {
    ipc.HandleIngest(args, "JSON")
}
```

## Testing

All utilities have comprehensive test coverage:
```bash
go test ./plugins/ipc/...
```

## Migration Guide

Old pattern (duplicated in each plugin):
```go
// IR Types
type Corpus struct {
    ID string `json:"id"`
    // ... 15 fields
}
// ... 10 more IR types

// Result types
type ExtractIRResult struct {
    // ...
}

// Escape functions
func escapeHTML(s string) string {
    s = strings.ReplaceAll(s, "&", "&amp;")
    // ...
}
```

New pattern (use shared package):
```go
import "github.com/FocuswithJustin/mimicry/plugins/ipc"

// Use ipc.Corpus, ipc.ExtractIRResult, ipc.EscapeHTML, etc.
```

## Benefits

1. **Eliminates ~2000 lines of duplicate code** across format plugins
2. **Consistent types** - IR types match exactly across all plugins
3. **Tested once, used everywhere** - shared test coverage
4. **Easier maintenance** - update once, apply everywhere
5. **Better documentation** - single source of truth for IPC protocol
