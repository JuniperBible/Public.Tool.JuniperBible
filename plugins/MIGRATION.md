# Plugin SDK Migration Guide

## Overview

This document describes the migration of 87 plugins from standalone executables to dual-build plugins that support both standalone mode and SDK mode. The migration enables plugins to be compiled either as separate binaries or as shared libraries that can be dynamically loaded by the main application.

## Migration Summary

- **Total plugins migrated**: 87
- **Build modes**: 2 (standalone and SDK)
- **Shared type files created**: 2
- **Build tag strategy**: Go build tags for conditional compilation

## Files Created Per Plugin

For each plugin, the following file structure was implemented:

### 1. `main_sdk.go` (SDK Build)

Created with `//go:build sdk` tag to enable SDK mode compilation.

**Purpose**: Provides the shared library entry points for dynamic loading.

**Template**:
```go
//go:build sdk

package main

import (
    "github.com/JuniperBible/sdk"
)

func GetSDKInfo() sdk.SDKInfo {
    return sdk.SDKInfo{
        APIVersion: "v1",
        PluginType: "<type>",
    }
}

// Plugin-specific SDK functions (e.g., FormatSDK, SearchSDK, etc.)
```

**Location**: `plugins/<type>/<plugin-name>/main_sdk.go`

### 2. `main.go` (Standalone Build)

Updated with `//go:build !sdk` tag to preserve standalone functionality.

**Purpose**: Maintains the original standalone executable behavior.

**Changes**:
- Added `//go:build !sdk` at the top of the file
- No other modifications to existing code

**Location**: `plugins/<type>/<plugin-name>/main.go`

### 3. Test Files

All test files were updated with `//go:build !sdk` tag.

**Purpose**: Ensures tests only run in standalone mode, avoiding SDK build conflicts.

**Changes**:
- Added `//go:build !sdk` at the top of each `*_test.go` file
- No other modifications to test logic

## Shared Type Files

To enable type compatibility across both build modes, shared type files were created for plugins that needed to expose internal types:

### 1. `plugins/format/sword-pure/types.go`

**Purpose**: Defines shared types for the SWORD format plugin.

**Contents**:
```go
package main

type SWORDCapsule struct {
    // Shared structure definition
}
```

**Why needed**: Allows SDK build to reference capsule types without duplicating code.

### 2. `plugins/tool/repoman/repoman.go`

**Purpose**: Defines the repository manager types and interfaces.

**Contents**:
```go
package main

type RepoManager struct {
    // Shared structure definition
}
```

**Why needed**: Enables both standalone and SDK builds to use the same repository logic.

## Building Plugins

### Standalone Mode (Default)

Build as a standalone executable:

```bash
# Single plugin
go build -o bin/<plugin-name> plugins/<type>/<plugin-name>/*.go

# All plugins
make build-plugins
```

### SDK Mode

Build as a shared library:

```bash
# Single plugin
go build -buildmode=plugin -tags=sdk -o bin/<plugin-name>.so plugins/<type>/<plugin-name>/*.go

# All plugins
make build-plugins-sdk
```

### Build Tags Explanation

- `//go:build sdk`: File is only included when `-tags=sdk` is used
- `//go:build !sdk`: File is only included when `-tags=sdk` is NOT used
- No tag: File is always included

## Migration Checklist

Use this checklist when migrating a new plugin to support SDK mode:

### Pre-Migration

- [ ] Identify plugin type (format, search, tool, transform)
- [ ] Review plugin's main.go structure
- [ ] Check for shared types that need to be extracted
- [ ] Verify plugin builds successfully in standalone mode

### Migration Steps

- [ ] Create `main_sdk.go` with `//go:build sdk` tag
- [ ] Add SDK info function: `GetSDKInfo()`
- [ ] Add plugin-specific SDK function (e.g., `FormatSDK()`, `SearchSDK()`)
- [ ] Add `//go:build !sdk` tag to `main.go`
- [ ] Add `//go:build !sdk` tag to all `*_test.go` files
- [ ] Extract shared types to separate file if needed (without build tags)
- [ ] Update imports if type file was created

### Testing

- [ ] Build in standalone mode: `go build`
- [ ] Run tests in standalone mode: `go test`
- [ ] Build in SDK mode: `go build -buildmode=plugin -tags=sdk`
- [ ] Verify SDK build produces `.so` file
- [ ] Test both builds work correctly

### Documentation

- [ ] Update plugin README if applicable
- [ ] Add notes about SDK support
- [ ] Document any special build requirements

## Plugin Types and SDK Functions

Each plugin type has a specific SDK function signature:

### Format Plugins

```go
func FormatSDK(capsulePath string) ([]byte, error)
```

### Search Plugins

```go
func SearchSDK(query string) ([]byte, error)
```

### Tool Plugins

```go
func ToolSDK(args []string) ([]byte, error)
```

### Transform Plugins

```go
func TransformSDK(input []byte) ([]byte, error)
```

## Common Issues and Solutions

### Issue: Duplicate symbols during SDK build

**Solution**: Ensure `main()` function is only in `main.go` with `//go:build !sdk` tag.

### Issue: Tests fail in SDK mode

**Solution**: Add `//go:build !sdk` to all test files. Tests are only run in standalone mode.

### Issue: Type not found in SDK build

**Solution**: Extract shared types to a separate file without build tags (e.g., `types.go`).

### Issue: Import cycle

**Solution**: Ensure shared type files don't import plugin-specific code that's build-tag restricted.

## Build System Integration

The build system supports both modes through Make targets:

```makefile
# Standalone builds
build-plugins:
    go build -o bin/<plugin> plugins/<type>/<plugin>/*.go

# SDK builds
build-plugins-sdk:
    go build -buildmode=plugin -tags=sdk -o bin/<plugin>.so plugins/<type>/<plugin>/*.go
```

## Best Practices

1. **Minimize SDK-specific code**: Keep `main_sdk.go` files small and focused on SDK interface only.

2. **Share common logic**: Put business logic in shared files without build tags.

3. **Test standalone mode**: Primary testing should occur in standalone mode.

4. **Document SDK changes**: Note any SDK-specific behavior in code comments.

5. **Consistent naming**: Use `main_sdk.go` for SDK builds, `main.go` for standalone.

6. **Type extraction**: Only extract types when necessary for cross-build compatibility.

## Future Considerations

- Consider automating migration with code generation tools
- Explore unified testing approach for both build modes
- Document performance differences between standalone and SDK modes
- Create SDK validation suite to ensure all plugins conform to SDK interface

## References

- Go build tags: https://pkg.go.dev/cmd/go#hdr-Build_constraints
- Plugin package: https://pkg.go.dev/plugin
- JuniperBible SDK: `github.com/JuniperBible/sdk`
