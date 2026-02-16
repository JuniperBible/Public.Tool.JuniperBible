# Juniper Bible - Project Analysis

**Last Updated:** 2026-01-09
**Status:** All 34 phases complete, 2,500+ tests passing, 40 bidirectional format plugins

## Executive Summary

**Juniper Bible** is a production-ready, forensic-grade framework for Bible and religious text format conversion with byte-perfect preservation. It's a **behavioral recording system** that captures exactly how reference tools behave and uses those recordings as test oracles.

**Core Philosophy:** "Byte-for-byte preservation. Bug-for-bug behavior. Deterministic forever."

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         CAPSULE LAB                                  │
├─────────────────────────────────────────────────────────────────────┤
│  CLI (cmd/capsule/)                                                  │
│  ├── ingest, export, verify, selfcheck                              │
│  ├── plugins, detect, enumerate                                      │
│  ├── extract-ir, emit-native, convert, ir-info                      │
│  └── run, tool-run, tool-archive, tool-list                         │
├─────────────────────────────────────────────────────────────────────┤
│  Core Packages                                                       │
│  ├── core/cas/       Content-Addressed Storage (SHA-256 + BLAKE3)   │
│  ├── core/capsule/   Container format with manifest, blobs          │
│  ├── core/ir/        Intermediate Representation                    │
│  ├── core/plugins/   Plugin loader and IPC                          │
│  ├── core/runner/    Deterministic NixOS VM execution               │
│  └── core/selfcheck/ RoundTrip plans and verification               │
├─────────────────────────────────────────────────────────────────────┤
│  Plugins                                                             │
│  ├── plugins/format/ 40 bidirectional format plugins                │
│  ├── plugins/tool/   10 tool plugins                                 │
│  └── plugins/example Template plugin                                 │
├─────────────────────────────────────────────────────────────────────┤
│  Reference Tools (contrib/tool/)                                     │
│  sword-utils, pandoc, calibre, usfm2osis, sqlite, libxml2, unrtf,  │
│  gobible-creator                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Core Components

| Component | Location | Purpose |
|-----------|----------|---------|
| **CAS** | `core/cas/` | Content-Addressed Storage (SHA-256 + BLAKE3) |
| **Capsule** | `core/capsule/` | Container format with manifest, blobs, artifacts |
| **IR System** | `core/ir/` | Intermediate Representation for format-agnostic conversion |
| **Plugin System** | `core/plugins/` | IPC-based format and tool handlers |
| **Runner** | `core/runner/` | Deterministic NixOS VM execution |
| **SelfCheck** | `core/selfcheck/` | RoundTrip plans and verification |
| **CLI** | `cmd/capsule/` | 18+ commands for all operations |

## Loss Classification

| Class | Description | Round-trip | Formats |
|-------|-------------|------------|---------|
| **L0** | Lossless | Byte-identical | osis, usfm, usx, json, zefania, theword |
| **L1** | Semantically lossless | Content preserved | epub, html, markdown, sqlite, esword, dbl, tei, morphgnt, oshb, sblgnt, sfm, xml, odf |
| **L2** | Minor loss | Some metadata lost | sword, rtf, logos, accordance, onlinebible, flex |
| **L3** | Significant loss | Text only | txt, gobible, pdb |

## Key Features

1. **Byte Sovereignty** - All inputs stored verbatim with cryptographic hashes
2. **Behavioral Authority** - Reference tools run in pinned NixOS VM
3. **IR System** - Corpus/Document/ContentBlock/Token hierarchy with stand-off markup
4. **40 Bidirectional Plugins** - All support extract-ir and emit-native
5. **SelfCheck Engine** - Declarative RoundTrip plans with pass/fail reports
6. **11 Sample Modules** - KJV, Vulgate, LXX, SBLGNT, ASV, WEB, etc.
7. **Modular Distribution** - Test data and third-party packages in separate branches

## Modular Distribution

Test data and third-party dependencies are distributed as separate Go modules:

| Branch | Contents | Size | License |
|--------|----------|------|---------|
| `main` | Production code | ~5 MB | Proprietary |
| `test-data` | Sample Bible modules, test fixtures | ~67 MB | Public Domain (modules) |
| `test-contrib` | Third-party tool references | ~16 MB | Various (see LICENSE-CONTRIBUTION.txt) |

```go
// Import in tests
import "github.com/FocuswithJustin/mimicry/test-data/data"
import "github.com/FocuswithJustin/mimicry/test-contrib/tools"
```

## Data Flow

```
Source File → Ingest (hash + store) → Capsule Archive
                                            ↓
                              Export (identity/derived)
                                            ↓
                              Plugin extract-ir → IR Corpus
                                            ↓
                              Plugin emit-native → Target Format
```

## Plugin Architecture

- **40 Format Plugins** in `plugins/format/`
- **10 Tool Plugins** in `plugins/tool/`
- IPC over JSON stdin/stdout
- Commands: detect, ingest, enumerate, extract-ir, emit-native

## Code Quality Metrics

- **2,500+ tests** across 18 packages
- **53 test files** with hash-based verification
- **Golden hash tests** prevent regressions
- **TDD methodology** enforced

## Documentation

| File | Purpose |
|------|---------|
| [README.md](../README.md) | Project overview and quick start |
| [PROJECT_CHARTER.md](PROJECT_CHARTER.md) | Vision, scope, guarantees |
| [DESIGN_NOTES.md](DESIGN_NOTES.md) | Full system architecture |
| [PLUGIN_DEVELOPMENT.md](PLUGIN_DEVELOPMENT.md) | Plugin authoring guide |
| [TDD_WORKFLOW.md](TDD_WORKFLOW.md) | Test methodology |
| [IR_IMPLEMENTATION.md](IR_IMPLEMENTATION.md) | IR system documentation |
| [QUICK_START.md](QUICK_START.md) | User quick start guide |
| [INTEGRATION.md](INTEGRATION.md) | Developer CLI integration guide |

## Development Roadmap

See [TODO.txt](../TODO.txt) for complete implementation roadmap including:

- **Phase 15**: Tool plugins for all reference tools
- **Phase 16**: IR system enhancements (versification, cross-refs, parallel corpora)
- **Phase 17**: Infrastructure and documentation generation

## Related Documents

- [QUICK_START.md](QUICK_START.md) - Installation and basic usage
- [INTEGRATION.md](INTEGRATION.md) - Wrapping the CLI in other languages
- [CLI_REFERENCE.md](generated/CLI_REFERENCE.md) - Complete CLI command reference (auto-generated)
- [PLUGINS.md](PLUGINS.md) - Plugin catalog (auto-generated)
- [FORMATS.md](FORMATS.md) - Format support matrix (auto-generated)
