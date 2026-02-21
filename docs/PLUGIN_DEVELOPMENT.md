# Plugin Development Guide

This guide covers how to develop format plugins and tool plugins for Juniper Bible.

---

## Overview

Juniper Bible uses a plugin architecture to extend its capabilities:

- **Format plugins** handle detection, ingestion, and enumeration of file formats
- **Tool plugins** run reference tools and generate deterministic transcripts

All plugins communicate with the core system via JSON over stdin/stdout.

---

## Plugin SDK (Recommended)

The Plugin SDK provides a higher-level abstraction for building format plugins, significantly reducing boilerplate code and providing automatic command routing with standard error handling.

### Benefits

- **Reduced boilerplate** - No manual command routing or IPC handling
- **Automatic command routing** - Commands are automatically mapped to your handler functions
- **Standard error handling** - Consistent error responses across all commands
- **Type-safe interfaces** - Compile-time verification of handler signatures
- **Easier testing** - Test individual handler functions without IPC setup

### Build Tags

The SDK pattern uses Go build tags to allow a single codebase to support both embedded and standalone plugin modes:

- `//go:build sdk` - Standalone plugin with `main()` function using SDK
- `//go:build !sdk` - Embedded plugin handler (default)

This allows the same plugin logic to be compiled either as a standalone executable or embedded directly into the core binaries.

### Usage Example - Canonical Package (New Architecture)

Here's how to create a new format using the canonical package pattern:

**Step 1**: Create canonical package in `core/formats/myformat/format.go`:

```go
package myformat

import (
    "github.com/JuniperBible/juniper/plugins/ipc"
    "github.com/JuniperBible/juniper/plugins/sdk/format"
    "github.com/JuniperBible/juniper/plugins/sdk/ir"
)

// Config defines the myformat plugin configuration
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
    // Parse the file and convert to IR
    corpus := &ir.Corpus{
        Meta: ir.Metadata{
            Title: "My Bible",
        },
        Books: []ir.Book{
            // Your book data
        },
    }
    return corpus, nil
}

func emit(corpus *ir.Corpus, outputDir string) (string, error) {
    // Convert IR back to native format
    outputPath := filepath.Join(outputDir, "output.mf")
    // Write your format here
    return outputPath, nil
}

func enumerate(path string) (*ipc.EnumerateResult, error) {
    return &ipc.EnumerateResult{
        Entries: []ipc.Entry{
            {Path: "myformat", SizeBytes: 1024, IsDir: false},
        },
    }, nil
}
```

**Step 2**: Create embedded registration in `core/formats/myformat/register.go`:

```go
//go:build !standalone

package myformat

func init() {
    Config.RegisterEmbedded()
}
```

**Step 3**: Create thin wrapper in `plugins/format-myformat/main.go`:

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

This gives you both embedded and standalone plugin support with zero code duplication.

### Migration Status

**87 plugins** have been successfully migrated to the SDK pattern, demonstrating its stability and effectiveness across a wide variety of format implementations.

### When to Use

- **New plugins** - Always use the SDK for new format plugins
- **Existing plugins** - Gradually migrate to SDK as plugins are updated
- **Complex formats** - SDK handles command routing, letting you focus on format-specific logic

---

## Plugin Structure

### New Architecture (Post-Deduplication)

As of 2025, Juniper Bible uses a canonical package architecture to eliminate code duplication:

```
core/formats/             # Canonical format implementations (42 packages)
├── json/
│   ├── format.go         # Format-specific logic
│   └── register.go       # Embedded registration
├── osis/
│   ├── format.go
│   └── register.go
└── .../

plugins/format-*/         # Thin standalone wrappers (5-15 lines each)
├── format-json/
│   └── main.go           # Imports core/formats/json
├── format-osis/
│   └── main.go           # Imports core/formats/osis
└── .../

plugins/tool/             # Tool plugins (unchanged)
└── libsword/
    ├── plugin.json
    └── tool-libsword
```

Key points:
- **Canonical implementations** live in `core/formats/<name>/`
- **Standalone plugins** are thin wrappers (~10 lines) that import canonical packages
- **No duplication** - each format has exactly one implementation
- **Embedded mode** - canonical packages register via `init()` for embedded use
- **Standalone mode** - wrappers call `format.Run(canonicalPackage.Config)`

---

## Format Plugin Contract

Format plugins handle file format detection and byte preservation.

### Required Commands

| Command | Description |
|---------|-------------|
| `detect` | Check if the plugin can handle a given path |
| `ingest` | Store file bytes verbatim in CAS |
| `enumerate` | List components within an archive/container |

### plugin.json Schema

```json
{
  "plugin_id": "format.file",
  "version": "1.0.0",
  "kind": "format",
  "entrypoint": "format-file",
  "capabilities": {
    "inputs": ["file"],
    "outputs": ["artifact.kind:file"]
  }
}
```

### IPC Protocol

**Request format (stdin):**

```json
{
  "command": "detect",
  "args": {
    "path": "/path/to/file"
  }
}
```

**Response format (stdout):**

```json
{
  "status": "ok",
  "result": {
    "detected": true,
    "format": "file",
    "reason": "single file detected"
  }
}
```

**Error response:**

```json
{
  "status": "error",
  "error": "path argument required"
}
```

### Command Details

#### detect

Check if this plugin can handle the given path.

**Args:**

- `path` (string): Path to the file or directory

**Result:**
```json
{
  "detected": true,
  "format": "file",
  "reason": "single file detected"
}
```

#### ingest

Store the file bytes verbatim in CAS.

**Args:**

- `path` (string): Path to the file
- `output_dir` (string): Directory to write blob files

**Result:**
```json
{
  "artifact_id": "sample",
  "blob_sha256": "abc123...",
  "size_bytes": 1024,
  "metadata": {
    "original_name": "sample.txt"
  }
}
```

#### enumerate

List components within the file (for archives/containers).

**Args:**

- `path` (string): Path to the file

**Result:**
```json
{
  "entries": [
    {
      "path": "file.txt",
      "size_bytes": 100,
      "is_dir": false
    }
  ]
}
```

---

## Tool Plugin Contract

Tool plugins run reference tools in a deterministic environment and generate transcripts.

### Required Commands

| Command | Description |
|---------|-------------|
| `info` | Return plugin metadata and available profiles |
| `run` | Execute a profile and generate transcript |

### plugin.json Schema

```json
{
  "name": "tool-libsword",
  "version": "1.0.0",
  "type": "tool",
  "description": "SWORD Bible module operations using libsword",
  "profiles": [
    {
      "id": "list-modules",
      "description": "List available SWORD modules"
    },
    {
      "id": "render-verse",
      "description": "Render a specific verse"
    }
  ],
  "requires": ["diatheke", "mod2osis"]
}
```

### Profile Execution

Tool plugins are invoked with command-line arguments:

```bash
./tool-libsword run --profile list-modules --sword-path /path/to/modules --out /path/to/output
```

### Transcript Output

Tool plugins generate `transcript.jsonl` in the output directory:

```jsonl
{"event":"start","timestamp":"2026-01-02T04:45:53Z","plugin":"tool-libsword","profile":"list-modules"}
{"event":"list_modules","modules":["kjv"]}
{"event":"end","timestamp":"2026-01-02T04:45:53Z"}
```

### Determinism Requirements

Tool plugins must ensure:

- Same inputs produce same outputs (byte-for-byte)
- Timestamps are normalized to UTC
- No random or non-deterministic behavior
- Environment variables are controlled (TZ=UTC, LC_ALL=C.UTF-8)

---

## Comprehensive Example Plugin

For a complete, well-documented example showing all plugin features, see:

**`plugins/format/example/`**

This example demonstrates:

- All required commands (detect, ingest, enumerate, extract-ir, emit-native)
- Using the `plugins/ipc` package helpers
- IR structure and conversion
- Content-addressed storage
- Error handling patterns
- Extensive inline documentation

The example plugin is in NOOP mode (disabled) for safety, but serves as a complete reference implementation.

Quick start:
```bash
cd plugins/format/example
cat README.md        # Comprehensive documentation
cat main.go          # Annotated example code
go build -o format-example .
```

---

## Example: Format Plugin (Go)

### Basic Structure

```go
package main

import (
    "encoding/json"
    "os"
    "github.com/JuniperBible/juniper/plugins/ipc"
)

func main() {
    // Read request from stdin
    req, err := ipc.ReadRequest()
    if err != nil {
        ipc.RespondErrorf("failed to read request: %v", err)
        return
    }

    // Route to command handlers
    switch req.Command {
    case "detect":
        handleDetect(req.Args)
    case "ingest":
        handleIngest(req.Args)
    case "enumerate":
        handleEnumerate(req.Args)
    case "extract-ir":
        handleExtractIR(req.Args)
    case "emit-native":
        handleEmitNative(req.Args)
    default:
        ipc.RespondErrorf("unknown command: %s", req.Command)
    }
}

func handleDetect(args map[string]interface{}) {
    // Extract arguments
    path, err := ipc.StringArg(args, "path")
    if err != nil {
        ipc.RespondError(err.Error())
        return
    }

    // Check format
    // ... detection logic ...

    // Respond
    ipc.MustRespond(&ipc.DetectResult{
        Detected: true,
        Format:   "myformat",
        Reason:   "format detected",
    })
}

// Implement other handlers...
```

### Using IPC Helpers

The `plugins/ipc` package provides common functionality:

```go
import "github.com/JuniperBible/juniper/plugins/ipc"

// Recommended: Use StandardDetect for two-stage detection (extension + content)
func handleDetect(args map[string]interface{}) {
    path, err := ipc.StringArg(args, "path")
    if err != nil {
        ipc.RespondError(err.Error())
        return
    }

    result := ipc.StandardDetect(path, "myformat",
        []string{".txt", ".text"},           // Extensions to check
        []string{"marker1", "marker2"},      // Content patterns (any match)
    )
    ipc.MustRespond(result)
}

// Recommended: Use StandardIngest with optional metadata callback
func handleIngest(args map[string]interface{}) {
    ipc.StandardIngest(args, "myformat", func(path string, data []byte) map[string]string {
        // Optional: Add format-specific metadata
        return map[string]string{
            "line_count": fmt.Sprintf("%d", bytes.Count(data, []byte("\n"))),
            "original_name": filepath.Base(path),
        }
    })
}

// Use StandardIngest without custom metadata (simpler)
func handleIngestSimple(args map[string]interface{}) {
    ipc.StandardIngest(args, "myformat", nil)
}

func handleEnumerate(args map[string]interface{}) {
    ipc.HandleEnumerateSingleFile(args, "myformat")
}

// Additional argument extraction helpers
func handleWithOptions(args map[string]interface{}) {
    // Optional string with default
    format := ipc.StringArgOr(args, "format", "default")

    // Optional bool with default
    verbose := ipc.BoolArg(args, "verbose", false)

    // Optional int with default (handles JSON float64)
    limit := ipc.IntArg(args, "limit", 100)
}

// Hash computation helpers
func computeHashes(data []byte) {
    // Just the hex string
    hexHash := ipc.ComputeHash(data)

    // Both raw bytes and hex string
    rawHash, hexHash := ipc.ComputeSourceHash(data)
}
```

Additional detect helpers available:

```go
// Extension-only detection
result := ipc.DetectByExtension(path, "myformat", ".txt", ".text")

// Content-only detection (all patterns must match)
result := ipc.DetectByContent(path, "myformat", "pattern1", "pattern2")

// Content-only detection (any pattern matches)
result := ipc.DetectByContentAny(path, "myformat", "pattern1", "pattern2")

// Magic bytes detection
result := ipc.DetectByMagicBytes(path, "myformat", []byte{0x50, 0x4B, 0x03, 0x04})

// Low-level check functions (return bool)
hasExt := ipc.CheckExtension(path, ".txt", ".text")
hasContent := ipc.CheckContentContains(path, "required1", "required2")
hasAny := ipc.CheckContentContainsAny(path, "option1", "option2")
hasMagic := ipc.CheckMagicBytes(path, []byte{0x50, 0x4B})

// Convenience constructors for results
result := ipc.DetectSuccess("myformat", "format detected successfully")
result := ipc.DetectFailure("not a valid myformat file")

// Error response helpers (do not exit)
err := ipc.RespondError("error message")
err := ipc.RespondErrorf("error: %v", someError)

// Error response helpers (exit with status 1)
ipc.RespondErrorAndExit("fatal error message")
ipc.RespondErrorfAndExit("fatal error: %v", someError)
```

---

## Example: Tool Plugin (Go)

```go
package main

import (
    "encoding/json"
    "flag"
    "os"
    "path/filepath"
    "time"
)

type TranscriptEvent struct {
    Event     string   `json:"event"`
    Timestamp string   `json:"timestamp,omitempty"`
    Plugin    string   `json:"plugin,omitempty"`
    Profile   string   `json:"profile,omitempty"`
    Data      interface{} `json:"data,omitempty"`
}

func main() {
    if len(os.Args) < 2 {
        printUsage()
        return
    }

    switch os.Args[1] {
    case "info":
        printInfo()
    case "run":
        runProfile()
    default:
        printUsage()
    }
}

func runProfile() {
    fs := flag.NewFlagSet("run", flag.ExitOnError)
    profile := fs.String("profile", "", "Profile to run")
    outDir := fs.String("out", "", "Output directory")
    fs.Parse(os.Args[2:])

    // Create transcript file
    f, _ := os.Create(filepath.Join(*outDir, "transcript.jsonl"))
    defer f.Close()
    enc := json.NewEncoder(f)

    // Write start event
    enc.Encode(TranscriptEvent{
        Event:     "start",
        Timestamp: time.Now().UTC().Format(time.RFC3339),
        Plugin:    "my-tool",
        Profile:   *profile,
    })

    // Execute profile logic here...

    // Write end event
    enc.Encode(TranscriptEvent{
        Event:     "end",
        Timestamp: time.Now().UTC().Format(time.RFC3339),
    })
}
```

---

## Building Plugins

### Go Plugins

```bash
# Build format plugin
go build -o plugins/format/myformat/format-myformat ./plugins/format/myformat

# Build tool plugin
go build -o plugins/tool/mytool/tool-mytool ./plugins/tool/mytool

# Build all core format plugins
for p in file zip dir tar sword osis usfm; do
  go build -o plugins/format/$p/format-$p ./plugins/format/$p
done

# Build all tool plugins
go build -o plugins/tool/libsword/tool-libsword ./plugins/tool/libsword
```

### Testing Plugins

```bash
# Test format plugin detection
echo '{"command":"detect","args":{"path":"testdata/sample.txt"}}' | ./plugins/format/file/format-file

# Test tool plugin info
./plugins/tool/libsword/tool-libsword info

# Test tool plugin execution
./plugins/tool/libsword/tool-libsword run --profile list-modules --sword-path /path --out /tmp/out
```

---

## Plugin Discovery

Juniper Bible discovers plugins from nested directories:

1. `./plugins/format/*/plugin.json` (format plugins)
2. `./plugins/tool/*/plugin.json` (tool plugins)
3. `$CAPSULE_PLUGIN_PATH/format/*/plugin.json` (custom format plugins)
4. `$CAPSULE_PLUGIN_PATH/tool/*/plugin.json` (custom tool plugins)

The loader also supports flat structure for backwards compatibility:

- `./plugins/format-*/plugin.json`
- `./plugins/tool-*/plugin.json`

List discovered plugins:

```bash
./capsule plugins
```

---

## Best Practices

1. **Never mutate input bytes** - Format plugins must preserve bytes exactly
2. **Generate deterministic output** - Same inputs must produce same hashes
3. **Use UTC timestamps** - All times should be in UTC
4. **Handle errors gracefully** - Return proper error responses, not panics
5. **Document profiles** - Each profile should have a clear description
6. **Test with hashes** - Verify output hashes match expected values

---

## Round-Trip Implementation

For formats that support `emit-native`, you need to implement a writer that generates the native binary format from IR.

### Example: SWORD zText Writer

The sword-pure plugin demonstrates a complete round-trip implementation:

1. **IR Extraction** (`extract-ir`): Reads SWORD binary files and extracts to IR
2. **Binary Generation** (`emit-native`): Generates SWORD binary files from IR

#### Writer Structure

```go
// ZTextWriter writes zText format SWORD modules.
type ZTextWriter struct {
    dataPath        string
    vers            *Versification
    currentBlock    bytes.Buffer
    blockEntries    []BlockEntry
    verseEntries    []VerseEntry
    compressedBuf   bytes.Buffer
    currentBlockNum uint32
}

func (w *ZTextWriter) WriteModule(corpus *IRCorpus) (int, error) {
    // 1. Create data directory
    // 2. Build verse map from corpus
    // 3. Write OT and NT separately
    // 4. Generate .bzs, .bzv, .bzz files
}
```

#### Binary File Generation

For SWORD zText format:

- `.bzs` - Block section index (12 bytes per entry: offset[4], size[4], ucsize[4])
- `.bzv` - Verse index (10 bytes per entry: block[4], offset[4], size[2])
- `.bzz` - Compressed text data (zlib compressed blocks)

#### Round-Trip Testing

```go
func TestZTextRoundTrip(t *testing.T) {
    // 1. Extract IR from original module
    corpus, _, err := extractCorpus(zt, conf)
    
    // 2. Emit to temporary directory
    result, err := EmitZText(corpus, tmpDir)
    
    // 3. Re-read the emitted module
    reCorpus, _, err := extractCorpus(reZt, reConfs[0])
    
    // 4. Compare content
    compareVerses(t, corpus, reCorpus, 100)
}
```

### Implemented Writers

| Format | Writer | Status | Files Generated |
|--------|--------|--------|-----------------|
| SWORD zText | ztext_writer.go | Complete | .bzs, .bzv, .bzz |
| SWORD zCom | zcom_writer.go | Complete | .bzs, .bzv, .bzz |
| SWORD zLD | zld_writer.go | Complete | .idx, .dat, .zdx, .zdt |

### Adding a New Writer

1. Create `{format}_writer.go` in the plugin directory
2. Implement the writer struct with `WriteModule()` method
3. Add `Emit{Format}()` function for public API
4. Add tests: `TestEmit{Format}Basic` and `Test{Format}RoundTrip`
5. Update plugin.json `ir_support.can_emit` to `true`

---

## Embedded vs External Plugins

Juniper Bible supports two plugin execution modes:

### Embedded Plugins (Default)

Plugins can be embedded directly into the main binaries (`capsule`, `capsule-web`, `capsule-api`). This is the default and recommended mode:

- **No external plugin directory required**
- **Faster execution** - no subprocess overhead
- **Simpler deployment** - single binary distribution
- **More secure** - no external code execution

#### Embedded Plugin Interface

Embedded plugins implement the `EmbeddedFormatHandler` interface:

```go
type EmbeddedFormatHandler interface {
    Detect(path string) (*DetectResult, error)
    Ingest(path, outputDir string) (*IngestResult, error)
    Enumerate(path string) (*EnumerateResult, error)
    ExtractIR(path, outputDir string) (*ExtractIRResult, error)
    EmitNative(irPath, outputDir string) (*EmitNativeResult, error)
}
```

#### Creating an Embedded Plugin (New Architecture)

**Important**: The architecture has changed. Embedded plugins now use canonical packages in `core/formats/<name>/`.

1. Create a canonical package in `core/formats/<name>/format.go`:

```go
package myformat

import (
    "github.com/JuniperBible/juniper/plugins/sdk/format"
    "github.com/JuniperBible/juniper/plugins/ipc"
)

var Config = &format.Config{
    PluginID:      "format.myformat",
    Name:          "MyFormat",
    Version:       "1.0.0",
    Extensions:    []string{".mf"},
    Detect:        detect,
    Parse:         parse,
    Emit:          emit,
}

func detect(path string) (*ipc.DetectResult, error) {
    // Detection logic
    return ipc.StandardDetect(path, "myformat", []string{".mf"}, nil), nil
}

// Implement parse, emit, etc.
```

2. Create registration file in `core/formats/<name>/register.go`:

```go
//go:build !standalone

package myformat

func init() {
    Config.RegisterEmbedded()
}
```

3. The package is automatically available in embedded mode (no manual registry import needed)

### External Plugins (Optional)

External plugins are standalone executables using JSON IPC. They are disabled by default for security.

#### Enabling External Plugins

```bash
# Via command-line flag
./capsule-web --plugins-external

# Via environment variable
CAPSULE_PLUGINS_EXTERNAL=1 ./capsule-web
```

#### When to Use External Plugins

- **Custom plugins** - Third-party or user-developed plugins
- **Premium plugins** - Separately licensed functionality
- **Testing** - Developing and testing new plugins
- **Isolation** - Plugins requiring specific dependencies

### Plugin Execution Flow

1. Plugin request received (detect, ingest, etc.)
2. Check embedded plugin registry for handler
3. If found, execute embedded handler directly
4. If not found AND external plugins enabled, execute external plugin via IPC
5. If not found AND external plugins disabled, return error

This ensures embedded plugins are always preferred when available.
