# TODO

This document tracks ongoing and planned work for Public.Tool.JuniperBible.

## Completed

### SQLite Migration (2026-02-28)
- [x] Migrate SQLite implementation to Public.Lib.Anthony
- [x] Remove CGO-based SQLite drivers (mattn/go-sqlite3, modernc.org/sqlite)
- [x] Remove contrib/sqlite-external directory
- [x] Update core/sqlite package to use Public.Lib.Anthony
- [x] Remove driver_cgo.go files
- [x] Archive legacy contrib/tool/juniper/src code to attic/
- [x] Add transaction support for bulk operations
- [x] Update documentation to reflect pure Go SQLite implementation
- [x] Remove SQLite driver divergence tests
- [x] Remove CGO build variants

### Code Deduplication (2026-02-16)
- [x] Create canonical format packages in core/formats/
- [x] Convert 32 standalone plugins to thin wrappers
- [x] Remove redundant embedded plugins
- [x] Remove redundant internal handlers
- [x] Achieve 93% code reduction (183,000 → 13,400 lines)

### Plugin SDK (2026-02-16)
- [x] Implement Plugin SDK for all 87 plugins
- [x] Create SDK runtime and format/tool infrastructure
- [x] Add parity testing between SDK and IPC

## In Progress

### Testing & Quality
- [ ] Expand integration test coverage
- [ ] Add performance benchmarks for SQLite operations
- [ ] Document transaction batching best practices

### Documentation
- [ ] Create migration guide for users updating from older versions
- [ ] Document Public.Lib.Anthony integration points
- [ ] Update BUILD_MODES.md to reflect pure Go only builds

## Planned

### Performance Optimization
- [ ] Profile and optimize hot paths in format conversions
- [ ] Add caching for frequently accessed metadata
- [ ] Optimize bulk insert operations with larger transaction batches

### Features
- [ ] Add support for SQLite virtual tables
- [ ] Implement full-text search using Public.Lib.Anthony FTS5
- [ ] Add support for additional versification systems

### Infrastructure
- [ ] Simplify Makefile now that CGO variants are removed
- [ ] Update CI/CD to remove CGO build matrix
- [ ] Add automated performance regression tests

## Future Considerations

### Cross-Project Integration
- [ ] Share SQLite utilities between Public.Tool.JuniperBible and Public.Website.MichaelCore
- [ ] Consider extracting common database patterns to shared library
- [ ] Evaluate Public.Lib.Anthony performance for production workloads

### Platform Support
- [ ] Test Public.Lib.Anthony on ARM platforms
- [ ] Verify Windows file locking behavior
- [ ] Test on BSD variants

## Notes

- CGO support has been completely removed as of 2026-02-28
- All SQLite operations now use Public.Lib.Anthony
- Transaction batching can improve bulk write performance by 10-100x
- See CHANGELOG.md for detailed version history
