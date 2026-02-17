# Phase 3: Thin Wrapper Migration - Quick Start

## One-Liner Summary
Convert a 600-line standalone plugin to an 11-line thin wrapper that delegates to a canonical implementation.

## Prerequisites Checklist

Before converting a format:
- [ ] Phase 2 complete: `core/formats/<name>/` exists
- [ ] Canonical `format.go` exists with exported `Config`
- [ ] All canonical tests pass: `go test core/formats/<name>/...`

## 3-Step Conversion

### 1. Convert
```bash
./scripts/convert-to-thin-wrapper.sh <format-name>
```

### 2. Validate
```bash
./scripts/validate-thin-wrapper.sh <format-name>
```

### 3. Test
```bash
go test core/formats/<format-name>/...
go build -tags standalone plugins/format-<format-name>/main.go
```

## Check Progress
```bash
./scripts/phase3-status.sh
```

## Expected Result

**Before** (663 lines):
```go
package main

import (
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "os"
    // ... many more imports
)

// IPCRequest is the incoming JSON request.
type IPCRequest struct { ... }

// IPCResponse is the outgoing JSON response.
type IPCResponse struct { ... }

// ... 50+ more lines of type definitions

func main() { ... }

func handleDetect(args map[string]interface{}) { ... }
func handleIngest(args map[string]interface{}) { ... }
func handleEnumerate(args map[string]interface{}) { ... }
func handleExtractIR(args map[string]interface{}) { ... }
func handleEmitNative(args map[string]interface{}) { ... }

// ... 500+ more lines
```

**After** (11 lines):
```go
//go:build standalone

package main

import (
    "github.com/FocuswithJustin/JuniperBible/core/formats/json"
    "github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
)

func main() {
    format.Run(json.Config)
}
```

**Reduction**: 652 lines (98%)

## Rollback
```bash
# Backups are automatic with timestamps
cp plugins/format-<name>/main.go.backup.TIMESTAMP plugins/format-<name>/main.go
```

## Common Issues

### "Canonical package not found"
**Solution**: Complete Phase 2 first - create `core/formats/<name>/format.go`

### "Config variable not found"
**Solution**: Ensure canonical package exports `var Config = &format.Config{...}`

### "Compilation failed"
**Solution**: Run `go mod tidy` and verify imports

### "Tests fail after conversion"
**Solution**: Tests should be in canonical package, not plugin. Delete plugin tests.

## Migration Order

Start simple, progress to complex:

2. **Tier 2**: osis, usfm, usx
3. **Tier 3**: zip, tar, epub
4. **Tier 4**: esword, mybible, sqlite
5. **Tier 5**: sword, logos, morphgnt (hardest)

## Success Indicators

- ✓ Wrapper compiles with `-tags standalone`
- ✓ Wrapper is ≤20 lines
- ✓ No duplicated types in wrapper
- ✓ No handler functions in wrapper
- ✓ All tests pass in canonical package
- ✓ Validation script passes all checks

## Full Documentation

- **Complete guide**: `docs/THIN_WRAPPER_MIGRATION.md`
- **Infrastructure summary**: `docs/PHASE3_SUMMARY.md`
- **Overall plan**: `docs/DEDUPLICATION_PLAN.md`

## Scripts Reference

| Script | Purpose | Usage |
|--------|---------|-------|
| `convert-to-thin-wrapper.sh` | Convert plugin to wrapper | `./scripts/convert-to-thin-wrapper.sh json` |
| `validate-thin-wrapper.sh` | Verify conversion | `./scripts/validate-thin-wrapper.sh json` |
| `phase3-status.sh` | Show progress | `./scripts/phase3-status.sh` |

## Getting Help

1. Check error message
2. Read `docs/THIN_WRAPPER_MIGRATION.md` common issues section
3. Verify Phase 2 is complete
4. Review validation output
5. Check that canonical tests pass

## Example Workflow

```bash
# 1. Verify Phase 2 is complete
ls core/formats/json/
# Should show: format.go, format_test.go

# 2. Run canonical tests
go test core/formats/json/...
# Should pass

# 3. Convert to thin wrapper
./scripts/convert-to-thin-wrapper.sh json
# Creates backup, generates wrapper, validates

# 4. Validate result
./scripts/validate-thin-wrapper.sh json
# Should show all checks passing

# 5. Check progress
./scripts/phase3-status.sh
# Should show json as "✓ Migrated"

# 6. Test plugin
go build -tags standalone -o /tmp/format-json plugins/format-json/main.go
echo '{"command":"detect","args":{"path":"test.json"}}' | /tmp/format-json
# Should return valid JSON response
```

Done! The format is now a thin wrapper.
