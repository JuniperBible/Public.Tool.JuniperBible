# Project File Formats

This document describes the standard formats used for project tracking files.

## TODO List (todo.txt)

The project task list follows the [todo.txt format](https://github.com/todotxt/todo.txt).

### Location

- `todo.txt` - Root project task list

### Format Specification

```
(A) 2026-01-15 Task description +Project @Context due:YYYY-MM-DD key:value
x 2026-01-15 2026-01-10 Completed task +Project @Context
```

### Syntax Elements

| Element | Syntax | Description |
|---------|--------|-------------|
| Priority | `(A)` to `(Z)` | Uppercase letter in parentheses, must be first |
| Creation date | `YYYY-MM-DD` | ISO 8601 date after priority |
| Completion | `x ` | Lowercase x followed by space at line start |
| Completion date | `YYYY-MM-DD` | Date task was completed (after `x`) |
| Project | `+ProjectName` | Plus sign prefix, no spaces |
| Context | `@ContextName` | At sign prefix, no spaces |
| Key:Value | `key:value` | Custom metadata, no spaces in key or value |

### Priority Levels

- `(A)` - Critical/Blocking
- `(B)` - High priority
- `(C)` - Medium priority
- `(D)` - Low priority/Future

### Projects Used

| Project | Description |
|---------|-------------|
| `+coverage` | Test coverage improvements |
| `+sdk` | Plugin SDK development |
| `+sqlite` | Pure Go SQLite implementation |
| `+dedup` | Code deduplication |
| `+juniper` | Juniper submodule tasks |
| `+diatheke` | Diatheke clone implementation |
| `+future` | Future enhancements |

### Contexts Used

| Context | Description |
|---------|-------------|
| `@core` | Core package work |
| `@internal` | Internal package work |
| `@plugin` | Plugin development |
| `@docs` | Documentation |
| `@test` | Testing |
| `@cli` | CLI commands |

### Examples

```
# High priority task with project and context
(A) Increase core/plugins coverage from 77.5% to 100% +coverage @core

# Completed task with dates
x 2026-02-15 2026-02-10 Create plugins/sdk/README.md +sdk @docs

# Task with due date
(B) Add GraphQL endpoint +future @core due:2026-03-01
```

### Tools

- [todo.txt CLI](https://github.com/todotxt/todo.txt-cli) - Command-line interface
- [todotxt.net](http://benrhughes.github.io/todotxt.net/) - Windows GUI
- [Simpletask](https://github.com/mpcjanssen/simpletask-android) - Android app
- VS Code extensions: "Todo+" or "todotxt-mode"

---

## Changelog (CHANGELOG.md)

The project changelog follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

### Location

- `CHANGELOG.md` - Root project changelog

### Format Specification

```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- New features

### Changed
- Changes to existing functionality

### Deprecated
- Features to be removed in future versions

### Removed
- Features removed in this version

### Fixed
- Bug fixes

### Security
- Security-related changes

## [1.0.0] - 2026-01-15

### Added
- Initial release features

[Unreleased]: https://github.com/user/repo/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/user/repo/releases/tag/v1.0.0
```

### Structure

1. **Header** - Title and format reference
2. **Unreleased** - Changes not yet released (always first)
3. **Version sections** - Reverse chronological order (newest first)
4. **Footer links** - GitHub comparison URLs

### Change Types

| Type | Description |
|------|-------------|
| `Added` | New features |
| `Changed` | Changes in existing functionality |
| `Deprecated` | Soon-to-be removed features |
| `Removed` | Now removed features |
| `Fixed` | Bug fixes |
| `Security` | Vulnerability fixes |

### Version Headers

Format: `## [VERSION] - YYYY-MM-DD`

- Use [Semantic Versioning](https://semver.org/): MAJOR.MINOR.PATCH
- Date in ISO 8601 format (YYYY-MM-DD)
- Link version to GitHub release/comparison

### Guidelines

1. **Write for humans** - Not a commit dump
2. **Group by type** - Use the standard change types
3. **Reverse chronological** - Newest versions first
4. **Link versions** - Add comparison URLs at bottom
5. **Keep Unreleased** - Gather changes before release
6. **One entry per change** - Don't combine unrelated changes

### Examples

```markdown
## [0.2.0] - 2026-02-16

### Added
- Plugin SDK with FormatConfig and Run functions
- 42 canonical format packages in `core/formats/`

### Changed
- Refactored 33+ functions reducing cyclomatic complexity

### Security
- Fixed XSS vulnerabilities in web UI
- Added path traversal protection

### Fixed
- Performance bottlenecks in module loading
```

---

## References

- [todo.txt format](https://github.com/todotxt/todo.txt)
- [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
- [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
