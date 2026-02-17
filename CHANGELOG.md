# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Pure Go SQLite Database Engine** - Complete replacement for external SQLite dependencies
  - Pager: Page cache, file I/O, journaling for ACID transactions
  - B-tree: Storage engine with read/write operations
  - Parser: Full SQL tokenizer and recursive descent parser
  - VDBE: Virtual Database Engine with 146 opcodes
  - Functions: 75+ built-in SQL functions (string, math, date/time, aggregates)
  - Query Planner: Cost-based optimization
  - Schema: sqlite_master table management
  - Driver: database/sql compatible interface
- Complete code deduplication infrastructure achieving 93% reduction (183,000 → 13,400 lines)
- 42 canonical format packages in `core/formats/<name>/` (single source of truth)
- SDK test infrastructure with PluginTest harness and shared fixtures
- Table-driven test framework in `core/formats/testing/`
- Thin wrapper template and migration scripts
- CI enforcement scripts for wrapper size limits
- New documentation: ARCHITECTURE.md, DEDUPLICATION_SUMMARY.md

### Changed

- Replaced `modernc.org/sqlite` with internal pure Go implementation
- Moved optional CGO SQLite driver to `contrib/sqlite-external/`
- Converted 32 standalone plugins to thin wrappers (~12 lines each, down from 600-800)
- Fixed embedded registration in all 41 `core/formats/*/register.go` files
- Enhanced `plugins/sdk/format/format.go` with RegisterEmbedded() support
- Updated all documentation to reflect new canonical package structure

### Removed

- External dependency on `modernc.org/sqlite` and its transitive dependencies
- External dependency on `github.com/mattn/go-sqlite3` (moved to contrib/)
- Deleted 41 redundant embedded plugins from `plugins/format/*/` (~71,000 lines)
- Deleted 40 redundant internal handlers from `internal/formats/*/` (~48,000 lines)
- Eliminated duplicated IPC type definitions from standalone plugins

## [0.2.0] - 2026-02-16

### Added

- General versification system with SWORD canon data
- Disk-based metadata cache for improved performance
- Archive TOC (Table of Contents) cache for faster module access
- Combined capsule scanning functionality
- Plugin SDK versions for all 87 plugins (42 format + 10 tool + 35 external)
- Atomic checks for efficient startup validation
- Filter capsules by HasIR capability
- SDK migration infrastructure:
  - `plugins/ipc/PROTOCOL.md` - IPC message envelope documentation
  - `plugins/ipc/testdata/` - Golden fixtures for all commands
  - `plugins/sdk/runtime/` - Dispatch and lifecycle
  - `plugins/sdk/format/` - FormatConfig and Run
  - `plugins/sdk/tool/` - ToolConfig and Run
  - `plugins/sdk/ir/` - Corpus read/write with hashing
  - `plugins/sdk/blob/` - Store and artifact IDs
  - `plugins/sdk/errors/` - Standardized error codes
  - `scripts/parity-test.sh` - SDK vs IPC parity testing

### Changed

- Refactored 33+ functions with approximately 280 helper functions extracted
- Optimized hot paths with atomic checks and efficient I/O operations
- Improved startup speed with targeted performance optimizations
- Enhanced pure Go SQLite implementation
- Comprehensive cyclomatic complexity remediation (CC ≤ 8 for all production code)

### Security

- Tightened file permissions from 0644 to 0600 across 190+ files
- Fixed XSS (Cross-Site Scripting) vulnerabilities
- Added path traversal protection
- Implemented ReDoS (Regular Expression Denial of Service) prevention

### Fixed

- Code complexity issues across the codebase
- Performance bottlenecks in module loading and initialization

## [0.1.0] - 2026-01-10

### Added

#### Core Infrastructure

- Content-addressed storage with SHA-256 (primary) and BLAKE3 (secondary) hashing
- Capsule pack/unpack in tar.xz format with gzip alternative
- Manifest generation with JSON schema validation
- NixOS VM execution harness for deterministic tool execution
- Plugin loader with format and tool plugin support
- Self-check engine with RoundTrip plans and SelfCheck reports
- Transcript capture in JSONL format

#### Format Plugins (40 bidirectional converters)

- **L0 Lossless**: osis, usfm, usx, zefania, theword, json
- **L1 Semantic**: esword, mysword, mybible, dbl, sqlite, markdown, html, epub, xml, odf, tei, morphgnt, oshb, sblgnt, sfm
- **L2 Structural**: sword, rtf, logos, accordance, onlinebible, flex, bibletime, crosswire, olive, ecm, na28app, tischendorf
- **L3 Text-primary**: txt, gobible, pdb
- **Archive**: file, zip, tar, dir

#### Tool Plugins (10)

- tool-libsword: SWORD module operations (list, render, mod2osis, osis2mod)
- tool-pandoc: Document format conversions (docx, odt, latex, html)
- tool-calibre: E-book format conversions (epub, mobi, azw3, pdf)
- tool-usfm2osis: USFM to OSIS XML conversion
- tool-sqlite: SQLite database operations
- tool-libxml2: XML validation and transformation (XSD, XSLT, XPath)
- tool-unrtf: RTF conversions
- tool-gobible-creator: GoBible J2ME packaging
- tool-repoman: SWORD repository management
- tool-hugo: Hugo JSON output generator

#### IR System (Intermediate Representation)

- Corpus, Document, ContentBlock types with stand-off markup
- Anchor and Span for overlapping annotations
- Ref type for verse references with OSIS ID support
- 17 versification systems (KJV, NRSV, LXX, Vulgate, Catholic, MT, Luther, Synodal, German, Armenian, Georgian, Slavonic, Syriac, Arabic, DSS, Samaritan, BHS, NA28)
- Cross-reference extraction and indexing
- Parallel corpus alignment (verse, sentence, word levels)
- Loss tracking with L0-L4 classification

#### CLI Commands (25+)

- Capsule operations: ingest, export, verify, selfcheck
- Plugin management: plugins, detect, enumerate
- IR operations: extract-ir, emit-native, convert, ir-info
- Run management: run, runs, compare, golden
- Tool operations: tool-run, tool-archive, tool-list
- Server commands: web, api
- Utilities: docgen

#### Web UI

- Capsule browser with artifact listing
- Bible reader with chapter navigation and search
- IR visualization with JSON tree view
- Format conversion interface
- Plugin listing and management
- Light/dark mode toggle

#### REST API

- OpenAPI 3.0 specification
- CORS support
- Standard response wrapper
- Endpoints for capsules, plugins, formats, conversion

#### Security

- Path traversal protection
- Input validation framework
- WebSocket security (origin validation, rate limiting)
- CSRF token protection
- Secure temp file handling

#### Code Quality

- 2,500+ tests passing
- 80%+ coverage for core packages
- Type-safe error handling
- Structured slog-based logging
- Plugin versioning with semantic constraints

#### Sample Data

- 11 complete Bible modules for testing (ASV, DRC, Geneva1599, KJV, LXX, OEB, OSMHB, SBLGNT, Tyndale, Vulgate, WEB)

#### Documentation

- PROJECT_CHARTER.md - System specification
- PLUGIN_DEVELOPMENT.md - Plugin authoring guide
- IR_IMPLEMENTATION.md - IR system documentation
- TDD_WORKFLOW.md - Test-driven development guide
- BUILD_MODES.md - SQLite driver selection
- VERSIFICATION.md - Versification systems
- OpenAPI specification for REST API

[Unreleased]: https://github.com/FocuswithJustin/JuniperBible/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/FocuswithJustin/JuniperBible/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/FocuswithJustin/JuniperBible/releases/tag/v0.1.0
