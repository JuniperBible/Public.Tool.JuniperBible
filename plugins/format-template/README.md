# Format Plugin Template

This directory contains templates for creating thin wrapper format plugins as part of the code deduplication effort (Phase 3).

## Files

### main.go.example

Template for a thin wrapper standalone plugin that delegates to a canonical format implementation in `core/formats/`.

**Usage**:

2. Ensure the canonical implementation exists at `core/formats/FORMATNAME/`
3. The canonical package must export a `Config` variable of type `*format.Config`

**Example** (for JSON format):
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

## Automated Conversion

Instead of using this template manually, use the conversion script:

```bash
./scripts/convert-to-thin-wrapper.sh <format-name>
```

The script will:
- Validate prerequisites
- Create automatic backup
- Generate the thin wrapper
- Verify compilation
- Test basic functionality

## Architecture

### Thin Wrapper Pattern

The thin wrapper pattern eliminates duplication by:

2. **Shared SDK**: Common IPC handling in `plugins/sdk/format/format.go`
3. **Minimal wrapper**: Standalone plugin is just a `main()` that calls `format.Run()`

### Benefits

- **Reduced duplication**: From ~600-800 lines to ~5-15 lines per plugin
- **Single source of truth**: Format logic only exists in one place
- **Easier maintenance**: Bug fixes and features only need one change
- **Better testing**: One comprehensive test suite per format
- **Faster builds**: Less code to compile

## Requirements

Before using this template:

1. **Complete Phase 2**: Create the canonical format package
   - Location: `core/formats/<name>/format.go`
   - Must export: `var Config = &format.Config{...}`
   - Must include: `Parse()` and/or `Emit()` functions
   - Must have: Comprehensive test suite

2. **Verify SDK**: Ensure SDK packages are available
   - `plugins/sdk/format` - Main SDK
   - `plugins/sdk/ir` - IR helpers
   - `plugins/sdk/blob` - Storage helpers
   - `plugins/ipc` - IPC protocol types

## See Also

- **Migration Guide**: `docs/THIN_WRAPPER_MIGRATION.md`
- **Main Plan**: `docs/DEDUPLICATION_PLAN.md`
- **Conversion Script**: `scripts/convert-to-thin-wrapper.sh`
- **SDK Format**: `plugins/sdk/format/format.go`
