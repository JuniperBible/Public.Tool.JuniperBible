# IPC Protocol Specification

This document defines the Inter-Process Communication (IPC) protocol between the JuniperBible host and plugins.

## Version

**Protocol Version: 1.0**

- Host supports versions N and N-1
- Version is implicit (no envelope header yet) - to be added in future version

## Overview

Plugins communicate with the host via JSON messages over stdin/stdout:

```
Host -> Plugin: JSON request on stdin
Plugin -> Host: JSON response on stdout
```

All messages are single-line JSON (no embedded newlines).

## Message Envelope

### Request Format

```json
{
  "command": "<command_name>",
  "args": {
    "<key>": "<value>",
    ...
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | Yes | Command to execute (detect, ingest, enumerate, extract-ir, emit-native, info) |
| `args` | object | No | Command-specific arguments |

### Response Format

```json
{
  "status": "ok|error",
  "result": { ... },
  "error": "<error_message>"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `status` | string | Yes | Either `"ok"` or `"error"` |
| `result` | object | If status=ok | Command-specific result object |
| `error` | string | If status=error | Human-readable error message |

## Format Plugin Commands

### detect

Determines if a file matches the plugin's format.

**Request args:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | Yes | Absolute path to file to detect |

**Result:**
```json
{
  "detected": true,
  "format": "JSON",
  "reason": "JSON format detected"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `detected` | boolean | Whether the format was detected |
| `format` | string | Format name (only if detected=true) |
| `reason` | string | Human-readable explanation |

### ingest

Ingests a file into content-addressed storage.

**Request args:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | Yes | Absolute path to file to ingest |
| `output_dir` | string | Yes | Directory to store blob |

**Result:**
```json
{
  "artifact_id": "example-bible",
  "blob_sha256": "abc123...",
  "size_bytes": 12345,
  "metadata": {
    "format": "JSON",
    "custom_key": "custom_value"
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `artifact_id` | string | Identifier derived from filename (without extension) |
| `blob_sha256` | string | SHA-256 hash of stored blob (hex) |
| `size_bytes` | integer | Size of blob in bytes |
| `metadata` | object | Key-value metadata (must include `format`) |

**Blob Storage Layout:**
```
output_dir/
  ab/
    abc123...  (blob stored at hash[:2]/hash)
```

### enumerate

Lists contents of a file or archive.

**Request args:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | Yes | Absolute path to file/directory |

**Result:**
```json
{
  "entries": [
    {
      "path": "Genesis.xml",
      "size_bytes": 12345,
      "is_dir": false,
      "mod_time": "2024-01-15T10:30:00Z",
      "metadata": {
        "format": "XML"
      }
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `entries` | array | List of EnumerateEntry objects |

**EnumerateEntry:**
| Field | Type | Description |
|-------|------|-------------|
| `path` | string | Relative path within archive/directory |
| `size_bytes` | integer | File size in bytes |
| `is_dir` | boolean | Whether entry is a directory |
| `mod_time` | string | ISO 8601 timestamp (optional) |
| `metadata` | object | Optional key-value metadata |

### extract-ir

Extracts Intermediate Representation from source format.

**Request args:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | Yes | Path to source file |
| `output_dir` | string | Yes | Directory to write IR |

**Result:**
```json
{
  "ir_path": "/path/to/corpus.json",
  "loss_class": "L1",
  "loss_report": {
    "source_format": "JSON",
    "target_format": "IR",
    "loss_class": "L1",
    "lost_elements": [],
    "warnings": []
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `ir_path` | string | Path to IR file (for large corpora) |
| `ir` | object | Inline IR data (for small corpora) - mutually exclusive with ir_path |
| `loss_class` | string | L0-L4 classification |
| `loss_report` | object | Detailed loss information (optional) |

**Loss Classes:**
- **L0**: Byte-for-byte round-trip (lossless)
- **L1**: Semantically lossless (formatting may differ)
- **L2**: Minor loss (some metadata/structure)
- **L3**: Significant loss (text preserved, markup lost)
- **L4**: Text-only (minimal preservation)

### emit-native

Converts IR back to native format.

**Request args:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ir_path` | string | Yes | Path to IR file |
| `output_dir` | string | Yes | Directory to write output |

**Result:**
```json
{
  "output_path": "/path/to/output.json",
  "format": "JSON",
  "loss_class": "L1",
  "loss_report": { ... }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `output_path` | string | Path to generated file |
| `format` | string | Output format name |
| `loss_class` | string | L0-L4 classification |
| `loss_report` | object | Detailed loss information (optional) |

## IR Types

### Corpus

Root container for a Bible, commentary, or other text collection.

```json
{
  "id": "kjv",
  "version": "1.0",
  "module_type": "bible",
  "versification": "KJV",
  "language": "en",
  "title": "King James Version",
  "description": "...",
  "publisher": "...",
  "rights": "Public Domain",
  "source_format": "OSIS",
  "documents": [ ... ],
  "source_hash": "abc123...",
  "loss_class": "L1",
  "attributes": {}
}
```

### Document

A single book or document within a corpus.

```json
{
  "id": "Gen",
  "title": "Genesis",
  "order": 1,
  "content_blocks": [ ... ],
  "attributes": {}
}
```

### ContentBlock

A unit of content (verse, paragraph) with stand-off markup.

```json
{
  "id": "Gen.1.1",
  "sequence": 1,
  "text": "In the beginning God created the heaven and the earth.",
  "tokens": [ ... ],
  "anchors": [ ... ],
  "hash": "abc123...",
  "attributes": {}
}
```

### Token, Anchor, Span, Ref

See `plugins/ipc/ir.go` for complete type definitions.

## Tool Plugin Commands

Tool plugins use a slightly different protocol with line-based JSON exchange.

### info

Returns plugin metadata.

**Response:**
```json
{
  "name": "tool-pandoc",
  "version": "1.0.0",
  "type": "tool",
  "description": "Document format converter",
  "profiles": [
    {"id": "docx-to-html", "description": "Convert DOCX to HTML"}
  ],
  "requires": ["pandoc"]
}
```

### check

Checks if required external tools are available.

**Response:**
```json
{
  "success": true,
  "data": {"tools_available": true}
}
```

### run

Tool execution is handled via request files and transcripts, not IPC.
See `plugins/ipc/tool_base.go` for the `ToolRunRequest` format.

## Error Codes

Errors are indicated by `status: "error"` with a human-readable message.
Future versions may add structured error codes.

**Standard Error Categories:**
- `"<arg> argument required"` - Missing required argument
- `"failed to read file: <error>"` - File I/O error
- `"failed to store blob: <error>"` - Storage error
- `"unknown command: <cmd>"` - Unrecognized command

## Compatibility Guarantees

1. **Request compatibility**: New optional fields may be added to requests; plugins should ignore unknown fields
2. **Response compatibility**: New optional fields may be added to responses; hosts should ignore unknown fields
3. **Command compatibility**: New commands may be added; plugins should return error for unknown commands
4. **Breaking changes**: Require protocol version bump; host supports N and N-1

## Implementation Files

- `plugins/ipc/protocol.go` - Request/Response types, encoding/decoding
- `plugins/ipc/handlers.go` - HandleDetect, HandleIngest, StandardIngest
- `plugins/ipc/detect_helpers.go` - StandardDetect, CheckExtension, etc.
- `plugins/ipc/args.go` - Argument extraction helpers
- `plugins/ipc/ir.go` - IR types (Corpus, Document, ContentBlock, etc.)
- `plugins/ipc/results.go` - ExtractIRResult, EmitNativeResult, LossReport
- `plugins/ipc/tool_base.go` - Tool plugin helpers
