# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Code deduplication plan to reduce duplication from 50%+ to under 10%
- Documentation: docs/DEDUPLICATION_PLAN.md with full architecture details

### Changed
- TODO.txt updated with deduplication project phases and acceptance criteria

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
