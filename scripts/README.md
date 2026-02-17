# Scripts Directory

This directory contains automation scripts for the JuniperBible project.

## Phase 3: Thin Wrapper Migration Scripts

### convert-to-thin-wrapper.sh

Converts a standalone format plugin from a full implementation (~600-800 lines) to a thin wrapper (~11 lines) that delegates to a canonical implementation in `core/formats/`.

**Usage**:
```bash
./scripts/convert-to-thin-wrapper.sh <format-name>
```

**Example**:
```bash
./scripts/convert-to-thin-wrapper.sh json
```

**Prerequisites**:
- Canonical implementation exists: `core/formats/<name>/format.go`
- Config variable exported: `var Config = &format.Config{...}`

**Features**:
- Validates prerequisites
- Creates timestamped backup automatically
- Generates thin wrapper from template
- Verifies compilation with `standalone` build tag
- Tests basic IPC functionality
- Reports line count reduction
- Automatic rollback on failure

**Output**:
- Backup: `plugins/format-<name>/main.go.backup.YYYYMMDD_HHMMSS`
- New wrapper: `plugins/format-<name>/main.go` (11 lines)

---

### validate-thin-wrapper.sh

Validates that a format plugin has been properly converted to a thin wrapper.

**Usage**:
```bash
./scripts/validate-thin-wrapper.sh <format-name>
```

**Example**:
```bash
./scripts/validate-thin-wrapper.sh json
```

**Checks** (14 total):

2. main.go exists
3. Has `//go:build standalone` tag
4. Line count ≤20 lines
5. Imports canonical format package
6. Imports SDK format package
7. Calls `format.Run()`
8. Passes Config variable
9. Canonical directory exists
10. Canonical format.go exists
11. Config variable exported
12. Compiles with standalone tag
13. No duplicated IPC types
14. No handler functions in wrapper

**Exit Codes**:
- 0: All checks passed
- 1: One or more checks failed

**Output**:
- Pass/fail/warn for each check
- Summary with counts
- Next steps if failures

---

### phase3-status.sh

Shows the current status of Phase 3 thin wrapper migration across all 32 format plugins.

**Usage**:
```bash
./scripts/phase3-status.sh
```

**Output**:
- Format-by-format status table
- Line count for each plugin
- Migration status: Migrated / Partial / Not Started
- Overall progress percentage
- Visual progress bar
- Summary statistics
- Next steps

**Example Output**:
```
Format               Status          Lines      Notes
--------------------------------------------------------------------------------
json                 ✓ Migrated      11 lines   Thin wrapper complete
xml                  ✗ Not Started   574 lines  Missing canonical (Phase 2 needed)
...

Summary:
  Total formats:         32
  Migrated (thin):       1
  Not migrated:          31
  Missing canonical:     31 (Phase 2 needed first)

Progress: 3% complete
[==------------------------------------------------] 3%
```

---

## Migration Workflow

### Standard workflow for converting a format:

1. **Ensure Phase 2 is complete** for the format:
   ```bash
   ls core/formats/<name>/format.go
   go test core/formats/<name>/...
   ```

2. **Convert to thin wrapper**:
   ```bash
   ./scripts/convert-to-thin-wrapper.sh <format-name>
   ```

3. **Validate conversion**:
   ```bash
   ./scripts/validate-thin-wrapper.sh <format-name>
   ```

4. **Run tests**:
   ```bash
   go test core/formats/<format-name>/...
   ```

5. **Check overall progress**:
   ```bash
   ./scripts/phase3-status.sh
   ```

### Rollback if needed:

```bash
# Find the backup
ls -lt plugins/format-<name>/main.go.backup.*

# Restore from backup
cp plugins/format-<name>/main.go.backup.TIMESTAMP plugins/format-<name>/main.go
```

---

## Script Maintenance

### Testing Scripts

All scripts can be tested with no arguments to see help:

```bash
./scripts/convert-to-thin-wrapper.sh
./scripts/validate-thin-wrapper.sh
./scripts/phase3-status.sh  # runs full scan
```

### Compatibility

All scripts use `#!/usr/bin/env bash` for NixOS compatibility.

### Error Handling

- All scripts use `set -euo pipefail` for safety
- Automatic backups before destructive operations
- Validation before accepting changes
- Color-coded output (info/success/warning/error)

---

## Documentation

For complete documentation, see:
- **Quick Start**: `docs/PHASE3_QUICKSTART.md`
- **Migration Guide**: `docs/THIN_WRAPPER_MIGRATION.md`
- **Infrastructure Summary**: `docs/PHASE3_SUMMARY.md`
- **Completion Report**: `PHASE3_COMPLETION_REPORT.md`
- **Overall Plan**: `docs/DEDUPLICATION_PLAN.md`

---

## Troubleshooting

### Script fails with "permission denied"

```bash
chmod +x scripts/*.sh
```

### Script fails with "canonical package not found"

Complete Phase 2 first - create `core/formats/<name>/format.go` with exported `Config`.

### Validation fails after conversion

Check error messages, ensure:
- Canonical package exists
- Config variable exported
- All tests pass in canonical package

### Want to undo a conversion

Use the automatic backup:
```bash
cp plugins/format-<name>/main.go.backup.TIMESTAMP plugins/format-<name>/main.go
```

---

## Expected Results

### Before Conversion
- Plugin: ~600-800 lines
- Contains: Full IPC types, handlers, format logic
- Tests: In plugin directory

### After Conversion
- Plugin: ~11 lines
- Contains: Just `format.Run(canonical.Config)`
- Tests: In canonical package only

### Reduction
- Per format: 98% reduction
- 32 formats: ~18,600 lines eliminated
- Net (with canonical): 73% reduction overall
