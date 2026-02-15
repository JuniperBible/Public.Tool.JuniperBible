# Plugin SDK Migration Checklist

This document tracks the migration of IPC plugins to the SDK.

## Migration Steps

For each plugin:

1. **Analyze existing plugin**
   - [ ] Count lines of boilerplate vs format-specific logic
   - [ ] Identify Parse/Emit functions if IR-enabled
   - [ ] Note any custom detection logic

2. **Create SDK version**
   - [ ] Create `main_sdk.go` alongside existing `main.go`
   - [ ] Define `format.Config` or `tool.Config`
   - [ ] Move format-specific logic to Config callbacks

3. **Verify parity**
   - [ ] Run `scripts/parity-test.sh test <plugin>`
   - [ ] Compare golden outputs
   - [ ] Run existing tests

4. **Replace and cleanup**
   - [ ] Rename `main.go` to `main_legacy.go`
   - [ ] Rename `main_sdk.go` to `main.go`
   - [ ] Remove legacy file after verification

## Format Plugins (42 total)

### Batch 1: Simple Formats (8 plugins)
| Plugin | Status | Lines Before | Lines After | Reduction |
|--------|--------|--------------|-------------|-----------|
| txt | ⏳ Pending | 536 | ~80 | ~85% |
| zip | ⏳ Pending | - | - | - |
| tar | ⏳ Pending | - | - | - |
| xml | ⏳ Pending | - | - | - |
| file | ⏳ Pending | - | - | - |
| dir | ⏳ Pending | - | - | - |
| rtf | ⏳ Pending | - | - | - |
| html | ⏳ Pending | - | - | - |

### Batch 2: IR-Enabled Formats (10 plugins)
| Plugin | Status | Lines Before | Lines After | Reduction |
|--------|--------|--------------|-------------|-----------|
| json | ⏳ Pending | - | - | - |
| zefania | ⏳ Pending | - | - | - |
| tei | ⏳ Pending | - | - | - |
| sfm | ⏳ Pending | - | - | - |
| pdb | ⏳ Pending | - | - | - |
| odf | ⏳ Pending | - | - | - |
| tischendorf | ⏳ Pending | - | - | - |
| morphgnt | ⏳ Pending | - | - | - |
| sblgnt | ⏳ Pending | - | - | - |
| ecm | ⏳ Pending | - | - | - |

### Batch 3: Complex Formats (12 plugins)
| Plugin | Status | Lines Before | Lines After | Reduction |
|--------|--------|--------------|-------------|-----------|
| osis | ⏳ Pending | - | - | - |
| usfm | ⏳ Pending | - | - | - |
| usx | ⏳ Pending | - | - | - |
| esword | ⏳ Pending | - | - | - |
| mysword | ⏳ Pending | - | - | - |
| mybible | ⏳ Pending | - | - | - |
| theword | ⏳ Pending | - | - | - |
| gobible | ⏳ Pending | - | - | - |
| onlinebible | ⏳ Pending | - | - | - |
| sqlite | ⏳ Pending | - | - | - |
| olive | ⏳ Pending | - | - | - |
| flex | ⏳ Pending | - | - | - |

### Batch 4: Proprietary/CGO Formats (8 plugins)
| Plugin | Status | Lines Before | Lines After | Reduction |
|--------|--------|--------------|-------------|-----------|
| sword | ⏳ Pending | - | - | - |
| sword-pure | ⏳ Pending | - | - | - |
| crosswire | ⏳ Pending | - | - | - |
| logos | ⏳ Pending | - | - | - |
| accordance | ⏳ Pending | - | - | - |
| dbl | ⏳ Pending | - | - | - |
| epub | ⏳ Pending | - | - | - |
| oshb | ⏳ Pending | - | - | - |

## Tool Plugins (10 total)

| Plugin | Status | Lines Before | Lines After | Reduction |
|--------|--------|--------------|-------------|-----------|
| pandoc | ⏳ Pending | - | - | - |
| calibre | ⏳ Pending | - | - | - |
| unrtf | ⏳ Pending | - | - | - |
| hugo | ⏳ Pending | - | - | - |
| libsword | ⏳ Pending | - | - | - |
| libxml2 | ⏳ Pending | - | - | - |
| sqlite | ⏳ Pending | - | - | - |
| usfm2osis | ⏳ Pending | - | - | - |
| gobible-creator | ⏳ Pending | - | - | - |
| repoman | ⏳ Pending | - | - | - |

## Status Legend

- ⏳ Pending - Not started
- 🔄 In Progress - Migration underway
- ✅ Complete - Migrated and verified
- ⚠️ Blocked - Issues found

## Summary

| Category | Total | Complete | In Progress | Pending |
|----------|-------|----------|-------------|---------|
| Format Plugins | 42 | 0 | 0 | 42 |
| Tool Plugins | 10 | 0 | 0 | 10 |
| **Total** | **52** | **0** | **0** | **52** |

**Expected reduction**: ~21,650 lines (68% overall)
