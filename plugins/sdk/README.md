# JuniperBible Plugin SDK

The Plugin SDK simplifies plugin development by providing high-level helpers that eliminate boilerplate code. It wraps the low-level IPC protocol with intuitive APIs, handling command dispatch, argument extraction, error handling, and blob storage automatically.

## Overview

Instead of manually implementing IPC message handling, plugins built with the SDK can focus on core functionality:

- **Format plugins**: Implement `Parse()` and `Emit()` functions to convert between native formats and the IR (Intermediate Representation)
- **Tool plugins**: Define metadata and implement `Check()` and execution handlers
- **Automatic handling**: The SDK handles all IPC communication, blob storage, error marshaling, and lifecycle management

## Package Structure

The SDK is organized into focused packages:

- **`plugins/sdk/format`** - Format plugin helpers (detect, ingest, extract-ir, emit-native, enumerate)
- **`plugins/sdk/tool`** - Tool plugin helpers (info, check, run)
- **`plugins/sdk/ir`** - IR corpus read/write utilities
- **`plugins/sdk/blob`** - Content-addressed blob storage
- **`plugins/sdk/errors`** - Standard error types with codes
- **`plugins/sdk/runtime`** - Low-level IPC dispatch and lifecycle
- **`plugins/sdk/types`** - Type re-exports from IPC package

## Quick Start

### Format Plugin Example

```go
//go:build sdk

package main

import (
	"github.com/JuniperBible/juniper/plugins/sdk/format"
	"github.com/JuniperBible/juniper/plugins/sdk/ir"
)

func main() {
	format.Run(&format.Config{
		Name:       "MyFormat",
		Extensions: []string{".myf"},

		Parse: func(path string) (*ir.Corpus, error) {
			// Parse file and return IR
			corpus := ir.NewCorpus("my-bible", "bible", "en")
			// ... populate corpus ...
			return corpus, nil
		},

		Emit: func(corpus *ir.Corpus, outputDir string) (string, error) {
			// Convert IR to native format
			// ... write output file ...
			return outputPath, nil
		},
	})
}
```

### Tool Plugin

```go
package main

import (
	"github.com/JuniperBible/juniper/plugins/sdk/tool"
)

func main() {
	tool.Run(&tool.Config{
		Name:        "my-tool",
		Version:     "1.0.0",
		Description: "Does something useful",
		Profiles: []tool.Profile{
			{ID: "default", Description: "Default profile"},
		},
		Requires: []string{"external-tool"},

		Check: func() (bool, error) {
			return tool.ExecCheck("external-tool"), nil
		},
	})
}
```

## Function Signatures

When implementing SDK-based plugins, your handlers should match these signatures:

### Format Plugin Functions

```go
// Detect performs custom format detection
// Returns: detection result with format name and confidence
Detect: func(path string) (*ipc.DetectResult, error)

// Parse reads a native format file and converts to IR
// Returns: populated IR corpus
Parse: func(path string) (*ir.Corpus, error)

// Emit converts IR corpus to native format
// Returns: path to output file
Emit: func(corpus *ir.Corpus, outputDir string) (string, error)

// Enumerate lists contents of archive formats
// Returns: list of entries (files/directories)
Enumerate: func(path string) (*ipc.EnumerateResult, error)
```

### Tool Plugin Functions

```go
// Check verifies tool availability
// Returns: true if tool is available and functional
Check: func() (bool, error)

// Run executes the tool with given parameters
// Returns: execution result or error
Run: func(req *ipc.ToolRunRequest) (interface{}, error)
```

## Build Tags

SDK-based plugins use Go build tags to enable conditional compilation:

```go
//go:build sdk
```

This allows maintaining both SDK and direct IPC implementations:

- `main.go` - Direct IPC implementation (build tag: `//go:build !sdk`)
- `main_sdk.go` - SDK implementation (build tag: `//go:build sdk`)

Build with SDK:
```bash
go build -tags sdk -o format-myformat main_sdk.go
```

Build without SDK (direct IPC):
```bash
go build -o format-myformat main.go
```

## Package Overview

### `plugins/sdk/format`

Helpers for building format plugins (detect, ingest, extract-ir, emit-native, enumerate).

- `Config` - Define plugin capabilities and handlers
- `Run(cfg)` - Start the plugin

### `plugins/sdk/tool`

Helpers for building tool plugins (info, check, run).

- `Config` - Define tool metadata and handlers
- `Run(cfg)` - Start the plugin
- `ExecCheck(name)` - Check if executable exists in PATH

### `plugins/sdk/runtime`

Low-level IPC dispatch and lifecycle management.

- `Dispatcher` - Maps commands to handlers
- `Run(handler)` - Start IPC loop
- `RunWithIO(handler, in, out)` - Run with custom I/O (for testing)

### `plugins/sdk/ir`

IR (Intermediate Representation) read/write helpers.

- `Read(path)` - Read Corpus from JSON file
- `Write(corpus, outputDir)` - Write Corpus to JSON file
- `NewCorpus(id, moduleType, language)` - Create new Corpus
- `Hash(corpus)` - Compute SHA-256 hash
- `Validate(corpus)` - Basic validation

### `plugins/sdk/blob`

Content-addressed storage helpers.

- `Store(outputDir, data)` - Store blob, returns hash
- `Retrieve(outputDir, hash)` - Retrieve blob by hash
- `Hash(data)` - Compute SHA-256 hash
- `ArtifactIDFromPath(path)` - Derive artifact ID from path

### `plugins/sdk/errors`

Standardized error types.

- `PluginError` - Structured error with code
- `MissingArg(name)` - Missing argument error
- `FileNotFound(path)` - File not found error
- `ParseError(format, cause)` - Parse error
- `IsRetryable(err)` - Check if error is retryable

### `plugins/sdk/types`

Re-exports of IPC types for convenience.

```go
import "github.com/JuniperBible/juniper/plugins/sdk/types"

// Use IPC types directly
var req types.Request
var corpus types.Corpus
```

## Examples

See `plugins/format-txt/` for a complete example of a format plugin using the SDK.

## Migration Guide

To migrate an existing IPC plugin to the SDK:

1. Replace command dispatch with `format.Run()` or `tool.Run()`
2. Move detection logic to `Config.Detect` or use extension-based detection
3. Move parsing logic to `Config.Parse`
4. Move emission logic to `Config.Emit`
5. Remove boilerplate (argument extraction, error envelope, blob storage)

### Before (IPC)

```go
func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req ipc.Request
		json.Unmarshal(scanner.Bytes(), &req)

		var result interface{}
		var err error

		switch req.Command {
		case "detect":
			result, err = handleDetect(req.Args)
		case "ingest":
			result, err = handleIngest(req.Args)
		// ... more commands ...
		}

		resp := ipc.Response{Status: "ok", Result: result}
		if err != nil {
			resp = ipc.Response{Status: "error", Error: err.Error()}
		}
		json.NewEncoder(os.Stdout).Encode(resp)
	}
}
```

### After (SDK)

```go
func main() {
	format.Run(&format.Config{
		Name:       "TXT",
		Extensions: []string{".txt"},
		Parse:      parseTXT,
		Emit:       emitTXT,
	})
}
```
