# JuniperBible Plugin SDK

The Plugin SDK provides a high-level API for building JuniperBible plugins. It eliminates boilerplate code by providing standard implementations for common plugin operations.

## Quick Start

### Format Plugin

```go
package main

import (
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
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
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/tool"
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
import "github.com/FocuswithJustin/JuniperBible/plugins/sdk/types"

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
