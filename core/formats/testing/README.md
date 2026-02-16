# Format Testing Framework

This package provides a comprehensive table-driven test framework for format plugins, eliminating test code duplication across the codebase.

## Overview

The framework provides a single function `RunFormatTests()` that executes a complete test suite for any format plugin:

- **Detect**: Verify format detection works correctly
- **DetectNegative**: Verify non-matching files are rejected
- **Ingest**: Test blob storage functionality
- **Enumerate**: Test archive enumeration (for container formats)
- **ExtractIR**: Test parsing to IR representation
- **EmitNative**: Test conversion from IR back to native format
- **RoundTrip**: Test L0 lossless round-trip conversion

## Usage

### Basic Example

```go
package json_test

import (
    "testing"
    ftesting "github.com/FocuswithJustin/JuniperBible/core/formats/testing"
    "github.com/FocuswithJustin/JuniperBible/core/formats/json"
)

func TestJSONFormat(t *testing.T) {
    ftesting.RunFormatTests(t, ftesting.FormatTestCase{
        Config: json.Config,
        SampleContent: `{
          "meta": {"id": "test", "title": "Test Bible"},
          "books": []
        }`,
        ExpectedIR: &ftesting.IRExpectations{
            ID:               "test",
            Title:            "Test Bible",
            MinDocuments:     0,
            MinContentBlocks: 0,
        },
        ExpectedLossClass: "L0",
        RoundTrip:         true,
    })
}
```

### Using Sample Files

```go
func TestOSISFormat(t *testing.T) {
    ftesting.RunFormatTests(t, ftesting.FormatTestCase{
        Config:            osis.Config,
        SampleFile:        "testdata/sample.osis.xml",
        ExpectedIR: &ftesting.IRExpectations{
            MinDocuments:     1,
            MinContentBlocks: 2,
        },
        ExpectedLossClass: "L1",
    })
}
```

### Custom Validation

```go
func TestCustomValidation(t *testing.T) {
    ftesting.RunFormatTests(t, ftesting.FormatTestCase{
        Config:        myformat.Config,
        SampleContent: "...",
        ExpectedIR: &ftesting.IRExpectations{
            CustomValidation: func(t *testing.T, corpus *ipc.Corpus) {
                // Custom assertions
                if corpus.Versification != "KJV" {
                    t.Errorf("expected KJV versification")
                }
            },
        },
    })
}
```

### Skipping Tests

```go
func TestPartialSupport(t *testing.T) {
    ftesting.RunFormatTests(t, ftesting.FormatTestCase{
        Config:        readonly.Config,
        SampleContent: "...",
        SkipTests:     []string{"EmitNative", "RoundTrip"},
    })
}
```

## FormatTestCase Fields

| Field | Type | Description |
|-------|------|-------------|
| `Config` | `*format.Config` | **Required**. Format plugin configuration |
| `SampleFile` | `string` | Path to test fixture file (mutually exclusive with SampleContent) |
| `SampleContent` | `string` | Inline test content (mutually exclusive with SampleFile) |
| `ExpectedIR` | `*IRExpectations` | Assertions about parsed IR corpus |
| `RoundTrip` | `bool` | Enable L0 lossless round-trip testing |
| `NegativeDetection` | `string` | Content that should NOT be detected as this format |
| `ExpectedLossClass` | `string` | Expected loss class (L0, L1, L2, L3, L4) |
| `SkipTests` | `[]string` | List of subtests to skip |

## IRExpectations Fields

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `string` | Expected corpus ID |
| `Title` | `string` | Expected corpus title |
| `MinDocuments` | `int` | Minimum number of documents expected |
| `MinContentBlocks` | `int` | Minimum content blocks in first document |
| `CustomValidation` | `func(t *testing.T, corpus *ipc.Corpus)` | Custom validation function |

## Test Patterns Supported

The framework consolidates these common test patterns found across the codebase:

### 1. Extension-Based Detection
```go
// Before: 30-50 lines per format
func TestXMLDetect(t *testing.T) {
    tmpDir, _ := os.MkdirTemp(...)
    defer os.RemoveAll(tmpDir)
    xmlPath := filepath.Join(tmpDir, "test.xml")
    os.WriteFile(xmlPath, []byte(content), 0600)
    req := IPCRequest{Command: "detect", Args: map[string]interface{}{"path": xmlPath}}
    resp := executePlugin(t, &req)
    // ... assertions ...
}

// After: 3-5 lines total
ftesting.RunFormatTests(t, ftesting.FormatTestCase{
    Config: xml.Config,
    SampleContent: content,
})
```

### 2. Content-Based Detection
```go
// Automatically handles custom Detect functions in Config
Config: &format.Config{
    Name: "OSIS",
    Extensions: []string{".xml"},
    Detect: func(path string) (*ipc.DetectResult, error) {
        // Custom logic...
    },
}
```

### 3. IR Extraction Testing
```go
// Before: 40-60 lines per format
func TestXMLExtractIR(t *testing.T) {
    tmpDir, _ := os.MkdirTemp(...)
    // ... setup ...
    req := IPCRequest{Command: "extract-ir", Args: ...}
    resp := executePlugin(t, &req)
    irData, _ := os.ReadFile(result["ir_path"].(string))
    var corpus Corpus
    json.Unmarshal(irData, &corpus)
    // ... assertions ...
}

// After: Built-in
ExpectedIR: &ftesting.IRExpectations{
    ID: "test",
    MinDocuments: 1,
}
```

### 4. Round-Trip Testing
```go
// Before: 50-80 lines per format
func TestXMLRoundTrip(t *testing.T) {
    // ... extract IR ...
    // ... emit native ...
    // ... compare hashes ...
}

// After: One flag
RoundTrip: true
```

## Integration with Canonical Format Packages

This framework is designed to work with the Phase 2 canonical format structure:

```
core/formats/
├── json/
│   ├── format.go       # Config + Parse + Emit
│   ├── format_test.go  # Uses this framework
│   └── testdata/
│       └── sample.json
├── txt/
│   ├── format.go
│   ├── format_test.go
│   └── testdata/
│       └── sample.txt
└── testing/            # This package
    ├── suite.go
    ├── example_test.go
    └── README.md
```

## Benefits

1. **Eliminates 89% of test code** - Consolidates ~45,000 lines to ~5,000 lines
2. **Consistent coverage** - All formats tested the same way
3. **Easy to maintain** - Bug fixes apply to all formats
4. **Fast to write** - New formats need only 10-20 lines of test code
5. **Type-safe** - Compile-time checking of Config compatibility

## Migration Guide

### Step 1: Identify Test Patterns

Look at existing tests in:
- `plugins/format-*/main_test.go`
- `plugins/format/*/main_test.go`
- `internal/formats/*/handler_test.go`

### Step 2: Create Format Test

```go
// Before: 200-400 lines
package main
import ( /* many imports */ )
func TestDetect(t *testing.T) { /* 40 lines */ }
func TestDetectNegative(t *testing.T) { /* 30 lines */ }
func TestExtractIR(t *testing.T) { /* 50 lines */ }
func TestEmitNative(t *testing.T) { /* 40 lines */ }
func TestRoundTrip(t *testing.T) { /* 60 lines */ }
func executePlugin(...) { /* 40 lines */ }

// After: 10-20 lines
package format_test
import "github.com/FocuswithJustin/JuniperBible/core/formats/testing"

func TestFormat(t *testing.T) {
    testing.RunFormatTests(t, testing.FormatTestCase{
        Config:        myformat.Config,
        SampleContent: "...",
        ExpectedIR:    &testing.IRExpectations{...},
        RoundTrip:     true,
    })
}
```

### Step 3: Add Format-Specific Edge Cases

```go
// Keep format-specific edge case tests separate
func TestJSONUnicodeHandling(t *testing.T) { /* ... */ }
func TestJSONMalformedInput(t *testing.T) { /* ... */ }
```

## See Also

- [Deduplication Plan](../../../docs/DEDUPLICATION_PLAN.md) - Overall project roadmap
- [plugins/sdk/format](../../plugins/sdk/format/format.go) - Format Config definition
- [plugins/ipc](../../plugins/ipc/) - IPC types and protocol
