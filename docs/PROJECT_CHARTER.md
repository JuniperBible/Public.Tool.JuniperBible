# Juniper Bible - Project Charter

## 1. Vision & Purpose

### Mission Statement
A content-addressed capsule system that stores original bytes verbatim and produces deterministic behavior transcripts by running reference tools (libSWORD and others) inside a pinned NixOS VM, with self-check plans that power TDD/CI.

### Problem Being Solved
Bible formats and conversion tools are:

- Poorly specified
- Historically quirky
- Full of edge cases people rely on

Re-implementing them by "reading docs" fails. Juniper Bible solves this by measuring what tools actually do, freezing that behavior, and making our code match.

### Target Users/Audience

- Bible tool developers
- Religious text researchers
- Archivists requiring byte-perfect preservation
- Anyone needing deterministic conversion verification

## 2. Core Guarantees (Non-Negotiables)

### Byte Sovereignty (Absolute)

- All inputs are stored **verbatim**
- SHA-256 and BLAKE3 hashes are recorded
- Exporting "back to native" always re-emits the exact original bytes
- No normalization. No cleanup. No guessing.

### Behavioral Authority

- "Correct behavior" is defined by **reference tools**
- Tools run in a pinned NixOS VM
- Output is captured as deterministic **transcripts**
- Transcripts are compared by hash in tests
- Documentation never overrides observed behavior

### Determinism

- Same input + same engine = same hashes
- If it's flaky, it's a bug
- Fix the harness, don't loosen the tests

## 3. Scope

### In-Scope Features

- Content-addressed blob storage (SHA-256 primary, BLAKE3 secondary)
- Capsule pack/unpack (tar.xz and zip formats)
- Manifest generation and JSON schema validation
- Deterministic NixOS VM execution environment
- Format plugins: file, dir, zip, tar, SWORD
- Tool plugins: libSWORD (first reference implementation)
- RoundTrip Plan execution
- SelfCheck Report generation
- CLI commands: ingest, enumerate, run, export, verify, selfcheck, test

### Out-of-Scope (Explicitly)

- Guaranteed byte-identical regeneration from semantic IR for every format
- Naive file conversion
- Canonicalization tools
- Parsers that "fix" data
- Best-effort systems

### Dependencies

- NixOS for deterministic VM
- SWORD library (libsword) for reference behavior
- Go standard library for implementation

## 4. Core Concepts

### Artifact
Immutable record pointing to a content-addressed blob and metadata:

- kind: zip, tar, sword-module, sword-conf, sword-data, osis, usfm, pdf, unknown
- hashes: sha256 + optional blake3
- blob path in capsule

### Capsule
Portable container (tar.xz or zip) holding:

- manifest.json
- blobs/sha256/<2>/<sha256>
- optional blobs/blake3/<2>/<blake3>.json pointer files
- transcripts and derived artifacts stored as blobs

### Engine
Reproducible execution environment:

- NixOS VM, pinned flake.lock hash
- Pinned locale/timezone (TZ=UTC, LC_ALL=C.UTF-8)
- No network
- Fixed mount points /work/in and /work/out

### Transcript
Deterministic JSONL event stream emitted by a tool plugin:

- ENGINE_INFO, MODULE_DISCOVERED, KEY_ENUM
- ENTRY_RENDERED, WARN, ERROR
- Each event references derived payloads by hash

### RoundTrip Plan
Declarative pipeline definition:

- Run tool(s)
- Export identity
- Run tool(s) again
- Compare hashes

### SelfCheck Report
Machine-readable deterministic pass/fail artifact:

- Stored as blobs
- Compared by hash
- Forms the basis of TDD/CI

## 5. Current Status

### Phase
**All Phases Complete** - Fully implemented and tested (1988+ tests passing)

### Key Metrics

- Architecture: Implemented
- CAS + manifest schemas: Implemented (SHA-256 + BLAKE3)
- Deterministic VM engine: Implemented (NixOS with SWORD support)
- Format plugins: 43 implemented (all bidirectional with IR support)
- Tool plugins: libsword with transcript generation
- IR System: Complete with L0-L4 loss class tracking
- Sample Data: 11 complete Bible modules for integration testing

### Recent Accomplishments

- All phases 1-19.5 complete
- All IR success metrics complete
- NixOS VM integration working with SWORD modules
- Plugin IPC calls in selfcheck executor
- 11 sample Bible modules (KJV, Vulgate, LXX, SBLGNT, etc.)
- Comprehensive integration tests with hash regression
- Phase 18: Juniper plugins (205 tests) - Pure Go SWORD/e-Sword parsing
- Phase 19.0: Self-contained architecture with pure Go SQLite
- Phase 19.5: Pure Go replacements for external tools (xml, rtf, epub, gobible)

## 6. Roadmap (All Complete)

### Phase 1: Foundation ✓

- Content-addressed blob store (SHA-256 primary, BLAKE3 secondary)
- Capsule pack/unpack
- Manifest generation + schema validation
- Identity export (prove byte-for-byte round-trip)

### Phase 2: Deterministic Execution Harness ✓

- Nix flake that builds the engine VM
- Host-side VM runner (mounts /work/in and /work/out)
- Enforce TZ/LC_ALL/LANG, no network

### Phase 3: Plugin System ✓

- Plugin loader + contract enforcement
- Format plugins: file, dir, zip, tar, SWORD enumerator
- Tool plugin: tools.libsword (emits transcript JSONL and content blobs)

### Phase 4: Self-Check Engine ✓

- RoundTripPlan execution
- SelfCheckReport generation
- Default plans: identity-bytes, libsword-behavior-identity

### Phases 5-14: Extended Implementation ✓

- 43 bidirectional format plugins (L0-L3 loss classes)
- IR (Intermediate Representation) system with lossless round-trips
- Cross-format conversion via IR pipeline
- NixOS VM integration for SWORD transcript generation
- 11 sample Bible modules for integration testing
- Plugin IPC calls in selfcheck executor

### Phases 15-19: Advanced Features ✓

- Phases 15-17: Additional format plugins and tools
- Phase 18: Juniper plugins - Pure Go SWORD/e-Sword parsing (205 tests)
  - juniper-sword: zText, zCom, zLD, RawGenBook, 9 versification systems
  - juniper-esword: .bblx, .cmtx, .dctx SQLite support (no CGO)
  - juniper-repoman: Repository management
  - juniper-hugo: Hugo JSON output
- Phase 19.0: Self-contained architecture (pure Go SQLite)
- Phase 19.5: Pure Go external tool replacements (88 tests)
  - core/xml: XPath-enabled XML parsing (replaces libxml2)
  - core/rtf: RTF to HTML/text/LaTeX (replaces unrtf)
  - core/epub: EPUB3 creation/parsing (replaces calibre)
  - core/gobible: GoBible JAR creation (replaces GoBible Creator)

See TODO.txt for detailed phase breakdown.

## 7. Success Criteria

### Per-Phase Goals
| Phase | Criterion |
|-------|-----------|
| 1 | ingest -> export(ID) produces identical hashes |
| 2 | Same tool run twice produces identical transcript hashes |
| 3 | libSWORD output fully captured and replayable |
| 4 | capsule selfcheck produces stable SelfCheckReport |

### Quality Metrics

- Tests compare **hashes**, not text
- Goldens are transcript hashes and selfcheck hashes
- If a hash changes, it must be explained
- No snapshot tests. No "looks right."

### Feature Completeness
A feature is done when:

- Its outputs are blobs with hashes
- Its behavior is captured in a transcript
- There is a self-check plan that verifies it
- CI compares hashes, not text diffs

## 8. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| SWORD format changes | Medium | Pin libsword version in Nix flake |
| VM non-determinism | High | Strict environment controls, no timestamps |
| Tool output timestamps | Medium | Normalization plugin step if needed |
| Large corpus performance | Medium | Chunked JSONL, content-addressed bundles |
| Plugin security | Low | Plugins from trusted paths, never auto-execute from capsules |

## 9. Stakeholders

### Owner/Maintainer

- Project lead responsible for architectural decisions

### Contributors

- Developers following TDD workflow
- Must match transcripts, not expectations

### Users

- Bible software developers
- Religious text researchers
- Anyone needing forensic-grade format conversion

## 10. Development Methodology

### Test-Driven Development (TDD)
This project follows strict TDD:

1. Write tests FIRST for each feature before implementation
2. Tests compare **hashes**, not text
3. Goldens are transcript hashes and selfcheck hashes
4. Each task follows: [T] Test -> [I] Implement -> [V] Verify pattern

### Implementation Language
**Go (Golang)** - Plugin interfaces for format/tool extensions

### The Rule to Remember
> **If the transcript says it behaves that way, that's how it behaves.**

Everything else is an implementation detail.

## 11. Related Documentation

- [README.md](README.md) - Project overview
- [docs/DESIGN_NOTES.md](docs/DESIGN_NOTES.md) - Full system plan with schemas
- [docs/DEVELOPER_NOTES.md](docs/DEVELOPER_NOTES.md) - Day-to-day development guidelines
- [docs/DEVELOPER_LEAD_NOTES.md](docs/DEVELOPER_LEAD_NOTES.md) - Phase breakdown and success criteria

## 12. Repository Structure

```
.
├── cmd/                    # CLI entrypoints
├── core/
│   ├── cas/                # content-addressed storage (SHA-256, BLAKE3)
│   ├── capsule/            # manifest handling, schema validation
│   ├── runner/             # VM execution harness
│   ├── selfcheck/          # round-trip plan execution
│   ├── xml/                # pure Go XML with XPath (replaces libxml2)
│   ├── rtf/                # pure Go RTF parser (replaces unrtf)
│   ├── epub/               # pure Go EPUB3 creation (replaces calibre)
│   └── gobible/            # pure Go GoBible JAR (replaces GoBible Creator)
├── plugins/
│   ├── format/             # format plugins (osis, usfm, sword, etc.)
│   │   ├── osis/           # OSIS XML format plugin
│   │   ├── usfm/           # USFM format plugin
│   │   ├── sword/          # SWORD binary modules plugin
│   │   └── .../            # 43 total format plugins
│   ├── tool/               # tool plugins (10 total)
│   │   └── libsword/       # SWORD library operations
│   └── meta/               # meta plugins (1 total)
│       └── juniper/        # Unified Bible module CLI
├── nix/
│   └── flake.nix           # deterministic NixOS VM engine
├── schemas/                # JSON Schemas (manifest, transcript, selfcheck)
├── testdata/
│   ├── fixtures/           # raw input files
│   └── goldens/            # expected transcript/selfcheck hashes
├── PROJECT_CHARTER.md      # This document
├── TODO.txt                # Implementation task list
└── README.md
```

---

*Last Updated: 2026-01-09*
