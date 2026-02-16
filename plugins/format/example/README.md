# Example Format Plugin

This is a comprehensive example plugin that demonstrates all plugin features. **This plugin is in NOOP mode** - it won't process real files and exists purely for documentation purposes.

## Purpose

Use this plugin as a reference when creating your own format plugins. It shows:

- How to implement all required commands (detect, ingest, enumerate, extract-ir, emit-native)
- How to use the `plugins/ipc` package for common operations
- IPC protocol structure and response formatting
- Error handling patterns
- IR (Intermediate Representation) integration
- Content-addressed storage usage

## Quick Start

### Building the Plugin

```bash
cd /tmp/port-example
go build -o plugins/format/example/format-example ./plugins/format/example
```

### Testing the Plugin

```bash
# Test detect command
echo '{"command":"detect","args":{"path":"test.example"}}' | \
  ./plugins/format/example/format-example

# Expected output (noop mode):
# {"status":"ok","result":{"detected":false,"reason":"noop plugin - for documentation only"}}
```

## SDK Version

This plugin provides two implementations:

1. **main.go** - Legacy IPC version (default)
2. **main_sdk.go** - SDK version using the format SDK

### Building with SDK

```bash
# Default: Legacy IPC version
go build .

# SDK version: Uses format SDK
go build -tags=sdk .
```

### SDK Implementation

The SDK version uses `format.Run()` with a `Config` struct that simplifies plugin development:

```go
func main() {
    format.Run(&format.Config{
        Name:       "format-example",
        Extensions: []string{".example"},
        Detect:     detectWrapper,
        Parse:      parseWrapper,
        Emit:       emitWrapper,
        Enumerate:  enumerateWrapper,
    })
}
```

### Wrapper Functions

The SDK requires specific function signatures. Wrapper functions convert between the internal function signatures and the SDK signatures:

- **detectWrapper**: Converts `detect(path) (bool, string, error)` to SDK signature `func(path string) (*ipc.DetectResult, error)`
- **parseWrapper**: Converts `parse(path, outputDir) (artifactID, blobHash, corpus, error)` to SDK signature `func(path string) (*ir.Corpus, error)`
- **emitWrapper**: Converts `emit(corpus, outputDir, formatVariant) (outputPath, error)` to SDK signature `func(corpus *ir.Corpus, outputDir string) (string, error)`
- **enumerateWrapper**: Converts `enumerate(path) ([]*ipc.EnumerateEntry, error)` to SDK signature `func(path string) (*ipc.EnumerateResult, error)`

The SDK handles:
- IPC protocol communication (JSON stdin/stdout)
- Command routing and argument parsing
- Response formatting and error handling
- Output directory management

This allows you to focus on format-specific logic rather than protocol details.

## IPC Protocol

All plugins communicate via JSON over stdin/stdout.

### Request Format

```json
{
  "command": "detect",
  "args": {
    "path": "/path/to/file"
  }
}
```

### Response Format

Success:
```json
{
  "status": "ok",
  "result": {
    "detected": true,
    "format": "example",
    "reason": "example format detected"
  }
}
```

Error:
```json
{
  "status": "error",
  "error": "path argument required"
}
```

## Commands

### detect

Determines if this plugin can handle the given file.

**Arguments:**
- `path` (string, required): Path to file or directory

**Returns:**
- `detected` (bool): Whether this plugin can handle the input
- `format` (string): Format name if detected
- `reason` (string): Human-readable explanation

**Example:**
```bash
echo '{"command":"detect","args":{"path":"bible.example"}}' | ./format-example
```

### ingest

Stores file bytes verbatim in content-addressed storage.

**Arguments:**
- `path` (string, required): Path to file to ingest
- `output_dir` (string, required): Directory for content-addressed storage

**Returns:**
- `artifact_id` (string): Identifier derived from filename
- `blob_sha256` (string): SHA-256 hash of file contents
- `size_bytes` (int64): File size in bytes
- `metadata` (map): Additional metadata

**Storage:**
Files are stored as: `output_dir/<hash[:2]>/<hash>`

**Example:**
```bash
echo '{"command":"ingest","args":{"path":"bible.example","output_dir":"/tmp/blobs"}}' | ./format-example
```

### enumerate

Lists components within an archive or container.

**Arguments:**
- `path` (string, required): Path to file/directory to enumerate

**Returns:**
- `entries` (array): List of files/components

Each entry contains:
- `path` (string): Relative path within archive
- `size_bytes` (int64): Size in bytes
- `is_dir` (bool): Whether this is a directory
- `mod_time` (string, optional): Modification time
- `metadata` (map, optional): Additional metadata

**Example:**
```bash
echo '{"command":"enumerate","args":{"path":"archive.example"}}' | ./format-example
```

### extract-ir

Converts native format to Intermediate Representation (IR).

**Arguments:**
- `path` (string, required): Path to source file
- `output_dir` (string, required): Directory for output files

**Returns:**
- `ir_path` (string): Path to IR JSON file (for large corpuses)
- `ir` (object): Inline IR data (for small corpuses)
- `loss_class` (string): L0-L4 classification
- `loss_report` (object, optional): Detailed loss information

**Loss Classes:**
- **L0**: Byte-for-byte round-trip (lossless)
- **L1**: Semantically lossless (formatting may differ)
- **L2**: Minor loss (some metadata/structure)
- **L3**: Significant loss (text preserved, markup lost)
- **L4**: Text-only (minimal preservation)

**Example:**
```bash
echo '{"command":"extract-ir","args":{"path":"bible.example","output_dir":"/tmp/ir"}}' | ./format-example
```

### emit-native

Converts Intermediate Representation (IR) to native format.

**Arguments:**
- `ir_path` (string, required): Path to IR JSON file
- `output_dir` (string, required): Directory for output files
- `format` (string, optional): Specific format variant to emit

**Returns:**
- `output_path` (string): Path to generated native file
- `format` (string): Output format name
- `loss_class` (string): L0-L4 classification
- `loss_report` (object, optional): Detailed loss information

**Example:**
```bash
echo '{"command":"emit-native","args":{"ir_path":"/tmp/ir/corpus.json","output_dir":"/tmp/out"}}' | ./format-example
```

## Using the IPC Package

The `plugins/ipc` package provides helpers to reduce boilerplate:

### Reading Requests

```go
import "github.com/FocuswithJustin/mimicry/plugins/ipc"

req, err := ipc.ReadRequest()
if err != nil {
    ipc.RespondErrorf("failed to read request: %v", err)
    return
}
```

### Extracting Arguments

```go
// Required string argument
path, err := ipc.StringArg(args, "path")
if err != nil {
    ipc.RespondError(err.Error())
    return
}

// Optional string with default
format := ipc.StringArgOr(args, "format", "default")

// Optional bool with default
verbose := ipc.BoolArg(args, "verbose", false)

// Common path + output_dir
path, outputDir, err := ipc.PathAndOutputDir(args)
```

### Sending Responses

```go
// Success response
ipc.MustRespond(&ipc.DetectResult{
    Detected: true,
    Format:   "example",
    Reason:   "example format detected",
})

// Error response (exits with status 1)
ipc.RespondError("path argument required")
ipc.RespondErrorf("failed to read: %v", err)
```

### Content-Addressed Storage

```go
// Store blob and get hash
hashHex, err := ipc.StoreBlob(outputDir, data)
if err != nil {
    ipc.RespondErrorf("failed to store blob: %v", err)
    return
}

// Extract artifact ID from path
artifactID := ipc.ArtifactIDFromPath(path)
```

### Common Handlers

For simple formats, use built-in handlers:

```go
// Detect with extension and content checks
ipc.HandleDetect(args, []string{".example"}, []string{"EXAMPLE"}, "example")

// Ingest with automatic blob storage
ipc.HandleIngest(args, "example")

// Enumerate single file
ipc.HandleEnumerateSingleFile(args, "example")
```

## IR Structure

The Intermediate Representation (IR) uses stand-off markup for overlapping annotations:

```go
corpus := &ipc.Corpus{
    ID:            "example-bible",
    ModuleType:    "bible",
    Versification: "KJV",
    Language:      "en",
    Title:         "Example Bible",
    Documents: []*ipc.Document{
        {
            ID:    "Gen",
            Title: "Genesis",
            Order: 1,
            ContentBlocks: []*ipc.ContentBlock{
                {
                    ID:       "Gen.1.1",
                    Sequence: 1,
                    Text:     "In the beginning...",
                    Anchors: []*ipc.Anchor{
                        {
                            ID:       "Gen.1.1.a0",
                            Position: 0,
                            Spans: []*ipc.Span{
                                {
                                    ID:   "Gen.1.1.verse",
                                    Type: "verse",
                                    Ref: &ipc.Ref{
                                        Book:    "Gen",
                                        Chapter: 1,
                                        Verse:   1,
                                    },
                                },
                            },
                        },
                    },
                },
            },
        },
    },
}
```

Key IR concepts:

- **Corpus**: Top-level container (Bible, commentary, dictionary, etc.)
- **Document**: Individual book or article
- **ContentBlock**: Unit of content (verse, paragraph, entry)
- **Anchor**: Position in text where spans can attach
- **Span**: Markup that spans from one anchor to another (verse markers, formatting, etc.)
- **Token**: Tokenized word or morpheme for linguistic analysis

## Creating Your Own Plugin

1. **Copy this example:**
   ```bash
   cp -r plugins/format/example plugins/format/myformat
   cd plugins/format/myformat
   ```

2. **Update plugin.json:**
   - Change `plugin_id` to `format.myformat`
   - Change `entrypoint` to `format-myformat`
   - Update `description` and capabilities

3. **Update main.go:**
   - Change `PluginName` constant
   - Set `NoopMode = false`
   - Implement real format parsing in each handler
   - Remove example comments

4. **Build and test:**
   ```bash
   go build -o format-myformat .
   echo '{"command":"detect","args":{"path":"test.myformat"}}' | ./format-myformat
   ```

5. **Register with loader:**
   The plugin will be automatically discovered when placed in `plugins/format/myformat/`

## Additional Resources

- **Plugin Development Guide**: `/tmp/port-example/docs/PLUGIN_DEVELOPMENT.md`
- **IR Documentation**: `/tmp/port-example/docs/IR_IMPLEMENTATION.md`
- **IPC Package**: `/tmp/port-example/plugins/ipc/`
- **Real Examples**: `/tmp/port-example/plugins/format/{json,osis,usfm}/`

## License

This example is part of the Juniper Bible project and follows the same license.
