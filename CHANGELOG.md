# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Complete code deduplication infrastructure achieving 93% reduction (183,000 → 13,400 lines)
- 42 canonical format packages in `core/formats/<name>/` (single source of truth)
- SDK test infrastructure with PluginTest harness and shared fixtures
- Table-driven test framework in `core/formats/testing/`
- Thin wrapper template and migration scripts
- CI enforcement scripts for wrapper size limits
- New documentation: ARCHITECTURE.md, DEDUPLICATION_SUMMARY.md

### Changed
- Converted 32 standalone plugins to thin wrappers (~12 lines each, down from 600-800)
- Fixed embedded registration in all 41 `core/formats/*/register.go` files
- Enhanced `plugins/sdk/format/format.go` with RegisterEmbedded() support
- Updated all documentation to reflect new canonical package structure

### Removed
- Deleted 41 redundant embedded plugins from `plugins/format/*/` (~71,000 lines)
- Deleted 40 redundant internal handlers from `internal/formats/*/` (~48,000 lines)
- Eliminated duplicated IPC type definitions from standalone plugins

## [0.2.0] - 2026-02-16

### Added
- General versification system with SWORD canon data
- Disk-based metadata cache for improved performance
- Archive TOC (Table of Contents) cache for faster module access
- Combined capsule scanning functionality
- Plugin SDK versions for all 87 plugins
- Atomic checks for efficient startup validation
- Filter capsules by HasIR capability

### Changed
- Refactored 33+ functions with approximately 280 helper functions extracted
- Optimized hot paths with atomic checks and efficient I/O operations
- Improved startup speed with targeted performance optimizations
- Enhanced pure Go SQLite implementation
- Comprehensive cyclomatic complexity remediation (CC <= 8 for all production code)

### Security
- Tightened file permissions from 0644 to 0600 across 190+ files
- Fixed XSS (Cross-Site Scripting) vulnerabilities
- Added path traversal protection
- Implemented ReDoS (Regular Expression Denial of Service) prevention

### Fixed
- Code complexity issues across the codebase
- Performance bottlenecks in module loading and initialization

## [0.1.0] - Initial Release

### Added
- Initial implementation of JuniperBible
- Core Bible module functionality
- Basic plugin system
- SQLite-based data storage

[Unreleased]: https://github.com/yourusername/JuniperBible/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/yourusername/JuniperBible/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/yourusername/JuniperBible/releases/tag/v0.1.0
