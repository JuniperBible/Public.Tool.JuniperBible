# Thin Wrapper Migration Guide

This document describes the process for converting standalone format plugins from full implementations (~600-800 lines) to thin wrappers (~5-15 lines) that delegate to canonical implementations in `core/formats/`.

## Overview

**Goal**: Eliminate code duplication by having one canonical implementation per format in `core/formats/<name>/`, with thin standalone plugin wrappers in `plugins/format-<name>/main.go`.

**Dependencies**: This migration requires Phase 2 (Create Canonical Format Packages) to be completed first for each format.

## Architecture

### Before (Current State)
```
plugins/format-json/main.go (663 lines)
  - Full IPC protocol implementation
  - Duplicated type definitions
  - Format-specific parsing logic
  - Format-specific emission logic
  - All command handlers
```

### After (Target State)
```
plugins/format-json/main.go (11 lines)
  //go:build standalone
  package main
  import (
      "github.com/JuniperBible/juniper/core/formats/json"
      "github.com/JuniperBible/juniper/plugins/sdk/format"
  )
  func main() {
      format.Run(json.Config)
  }

core/formats/json/format.go (~150 lines)
  - Config definition
  - Format-specific parseJSON() function
  - Format-specific emitJSON() function
```

## Prerequisites

Before migrating a format, ensure:

1. **Phase 2 Complete**: The canonical package exists at `core/formats/<name>/`
   - Contains `format.go` with `Config` variable
   - Contains `format_test.go` with comprehensive tests
   - All tests pass: `go test core/formats/<name>/...`

2. **Backup Created**: Original plugin is backed up (script does this automatically)

3. **Dependencies Available**:
   - `plugins/sdk/format` package exists
   - `plugins/ipc` package exists
   - All imports are accessible

## Migration Process

### Automated Conversion (Recommended)

Use the provided script for automatic conversion:

```bash
# Convert a single format
./scripts/convert-to-thin-wrapper.sh json

# The script will:
# 1. Validate prerequisites
# 2. Create timestamped backup
# 3. Generate thin wrapper
# 4. Verify compilation
# 5. Test basic IPC functionality
```

### Manual Conversion

If you need to convert manually:

1. **Backup the original**:
   ```bash
   cp plugins/format-<name>/main.go plugins/format-<name>/main.go.backup.$(date +%Y%m%d_%H%M%S)
   ```

2. **Create the thin wrapper**:
   ```bash
   cat > plugins/format-<name>/main.go <<'EOF'
   //go:build standalone

   package main

   import (
       "github.com/JuniperBible/juniper/core/formats/<name>"
       "github.com/JuniperBible/juniper/plugins/sdk/format"
   )

   func main() {
       format.Run(<name>.Config)
   }
   EOF
   ```

3. **Verify compilation**:
   ```bash
   go build -tags standalone -o /tmp/test-plugin plugins/format-<name>/main.go
   ```

4. **Test IPC protocol**:
   ```bash
   echo '{"command":"detect","args":{"path":"test.json"}}' | /tmp/test-plugin
   ```

## Validation Checklist

After conversion, verify each item:

### Build Validation
- [ ] Plugin compiles with `standalone` build tag
- [ ] No compilation errors or warnings
- [ ] Binary size is reasonable (should be smaller)

### Functional Validation
- [ ] `detect` command works
- [ ] `ingest` command works
- [ ] `enumerate` command works (if applicable)
- [ ] `extract-ir` command works
- [ ] `emit-native` command works

### Test Validation
- [ ] All unit tests pass in canonical package: `go test core/formats/<name>/...`
- [ ] Integration tests pass: `go test tests/integration/...`
- [ ] No regressions in existing functionality

### Code Quality
- [ ] Wrapper is 5-15 lines (excluding comments)
- [ ] Uses `//go:build standalone` tag
- [ ] Imports are correct
- [ ] No dead code remains

## Testing Strategy

### Unit Tests
Canonical package tests are the source of truth:
```bash
# Test canonical implementation
go test -v core/formats/json/...

# Test with coverage
go test -cover core/formats/json/...
```

### Integration Tests
Test the standalone plugin as it would be used in production:
```bash
# Build the plugin
go build -tags standalone -o /tmp/format-json plugins/format-json/main.go

# Test detect command
echo '{"command":"detect","args":{"path":"testdata/sample.json"}}' | /tmp/format-json

# Test ingest command
echo '{"command":"ingest","args":{"path":"testdata/sample.json","output_dir":"/tmp/test"}}' | /tmp/format-json

# Test extract-ir command
echo '{"command":"extract-ir","args":{"path":"testdata/sample.json","output_dir":"/tmp/test"}}' | /tmp/format-json
```

### Round-Trip Tests
Verify L0 formats maintain perfect fidelity:
```bash
# For L0 formats, test round-trip conversion
cd core/formats/json
go test -v -run TestRoundTrip
```

## Rollback Procedure

If issues are discovered after conversion:

1. **Immediate Rollback**:
   ```bash
   # Restore from backup (use latest timestamp)
   cp plugins/format-<name>/main.go.backup.TIMESTAMP plugins/format-<name>/main.go
   ```

2. **Verify Restoration**:
   ```bash
   go build -tags standalone plugins/format-<name>/main.go
   go test plugins/format-<name>/...
   ```

3. **Document Issues**:
   - Create GitHub issue with details
   - Include error messages
   - Note which tests failed
   - Attach backup file reference

## Migration Order

Follow the order defined in `DEDUPLICATION_PLAN.md`:

### Tier 1 - Simple Formats (Start Here)
Extensions-only detection, straightforward structure:
- [ ] txt
- [ ] json
- [ ] xml
- [ ] markdown
- [ ] html
- [ ] rtf

### Tier 2 - XML-Based Formats
Content markers, standard structure:
- [ ] osis
- [ ] usfm
- [ ] usx
- [ ] zefania
- [ ] tei
- [ ] sfm

### Tier 3 - Archive/Container Formats
Multiple files, enumeration required:
- [ ] zip
- [ ] tar
- [ ] dir
- [ ] file
- [ ] epub
- [ ] odf
- [ ] dbl

### Tier 4 - Database Formats
SQLite-based formats:
- [ ] esword
- [ ] mysword
- [ ] mybible
- [ ] theword
- [ ] olive
- [ ] sqlite

### Tier 5 - Complex/Binary Formats
Specialized parsing, complex structures:
- [ ] sword
- [ ] sword-pure
- [ ] bibletime
- [ ] crosswire
- [ ] logos
- [ ] accordance
- [ ] gobible
- [ ] pdb
- [ ] morphgnt
- [ ] sblgnt
- [ ] oshb
- [ ] tischendorf
- [ ] na28app
- [ ] flex
- [ ] onlinebible
- [ ] ecm

## Common Issues and Solutions

### Issue: Compilation Error - Package Not Found

**Error**:
```
cannot find package "github.com/JuniperBible/juniper/core/formats/json"
```

**Solution**:
- Ensure Phase 2 is complete for this format
- Verify `core/formats/json/format.go` exists
- Run `go mod tidy` to update dependencies

### Issue: Config Variable Not Found

**Error**:
```
undefined: json.Config
```

**Solution**:
- Check that canonical `format.go` exports `var Config = &format.Config{...}`
- Ensure Config is capitalized (exported)
- Verify import path matches package structure

### Issue: Build Tag Not Recognized

**Error**:
Plugin builds even without `-tags standalone`

**Solution**:
- Ensure first line is exactly `//go:build standalone`
- No space before `//`
- Blank line after build tag before `package main`

### Issue: Tests Fail After Conversion

**Error**:
Integration tests fail after wrapper conversion

**Solution**:

2. Verify wrapper compiles: `go build -tags standalone ...`
3. Test IPC protocol manually
4. If issues persist, rollback and investigate canonical implementation

### Issue: Wrapper Too Large

**Error**:
Wrapper exceeds 20-line limit

**Solution**:
- Wrapper should only call `format.Run()`
- Move any logic to canonical package
- Remove comments if necessary (keep build tag comment only)

## Success Metrics

Track progress for each format:

| Metric | Before | After | Target |
|--------|--------|-------|--------|
| Lines of code | 600-800 | 5-15 | <20 |
| Duplication | 100% | 0% | 0% |
| Test coverage | Varies | Same | >80% |
| Build time | Baseline | Faster | <baseline |

## Post-Migration Cleanup

After successful migration and validation:

1. **Delete Test Files in Plugin Directory**:
   ```bash
   # Tests now live in canonical package
   rm plugins/format-<name>/*_test.go
   ```

2. **Update Documentation**:
   - Update README.md with new architecture
   - Document canonical package location
   - Update developer guide

3. **CI/CD Updates**:
   - Ensure CI builds standalone plugins
   - Add wrapper size checks
   - Verify integration tests still pass

4. **Delete Old Backups** (after confidence period):
   ```bash
   # After 30 days of successful operation
   find plugins/format-*/main.go.backup.* -mtime +30 -delete
   ```

## Support and Help

- **Questions**: Check `DEDUPLICATION_PLAN.md` for architecture details
- **Issues**: Create GitHub issue with "Phase 3 Migration" label
- **Blockers**: Ensure Phase 2 is complete before attempting Phase 3

## References

- Main plan: `docs/DEDUPLICATION_PLAN.md`
- SDK format package: `plugins/sdk/format/format.go`
- Example canonical: `core/formats/json/format.go` (after Phase 2)
- Template wrapper: `plugins/format-template/main.go.example`
