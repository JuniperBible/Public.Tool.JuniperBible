# Juniper - Project Charter

## 1. Vision & Purpose

### Mission Statement
A pure Go scripture conversion toolkit that replaces external dependencies (diatheke, installmgr) with a unified, well-tested tool for converting SWORD and e-Sword modules to Hugo-compatible JSON.

### Problem Being Solved

- SWORD's CGo bindings are complex and platform-specific
- The `diatheke` and `installmgr` tools lack modern features and programmatic access
- No unified tool handles SWORD, e-Sword, and other formats in one pipeline
- Cross-versification mapping is poorly documented and implemented

### Target Users/Audience

- **Focus with Justin website**: Primary consumer of scripture JSON output
- **Hugo developers**: Anyone needing Bible data in static sites
- **Scripture scholars**: Those needing reliable format conversion

## 2. Scope

### In-Scope Features

| Feature | Status | Description |
|---------|--------|-------------|
| **SWORD Parsing** | Complete | zText, zCom, zLD, RawGenBook formats |
| **e-Sword Parsing** | Complete | .bblx, .cmtx, .dctx SQLite databases |
| **Markup Conversion** | Complete | OSIS, ThML, GBF, TEI to Markdown |
| **Versification** | Complete | KJV, KJVA, Vulgate, LXX systems |
| **Module Migration** | Complete | Copy modules from system paths |
| **Hugo JSON Output** | Complete | bibles.json + auxiliary content |
| **InstallMgr Replacement** | In Progress | Native Go repository management |
| **Diatheke Clone** | Planned | CLI-compatible verse lookup |

### Out-of-Scope Items

- GUI application
- Web server/API mode
- Non-scripture content (maps, images)
- Runtime module loading (static generation only)

### Dependencies

| Dependency | Purpose | License |
|------------|---------|---------|
| Go 1.22+ | Language runtime | BSD-3-Clause |
| libsword-dev | CGo testing only | GPL-2.0 |
| SQLite | e-Sword database access | Public Domain |

## 3. Current Status

### Phase/Milestone
**Mature** - Core conversion pipeline complete, repository management in progress

### Key Metrics

| Package | Test Coverage | Status |
|---------|---------------|--------|
| pkg/cgo | 100% | Complete |
| pkg/config | 100% | Complete |
| pkg/markup | 99.1% | Complete |
| pkg/migrate | 92.4% | Good |
| pkg/testing | 89.5% | Good |
| pkg/esword | 88.6% | Good |
| pkg/sword | 79.7% | Needs work |
| pkg/output | 83.9% | Good |
| pkg/repository | 62.4% | In Progress |

### Recent Accomplishments

- Renamed tool from sword-converter to Juniper
- Implemented versification system (KJV, KJVA, Vulgate, LXX)
- Added RawGenBook parser for Quran and Ethiopian texts
- Fixed zLD parser for binary .idx format
- Fixed zCom parser for NT-only modules
- Added book filtering to exclude empty books
- Moved to separate git repository as submodule

## 4. Roadmap

### Phase: InstallMgr Replacement (In Progress)
Replace SWORD `installmgr` with native Go implementation:

- [x] Core infrastructure (source, client, index, installer) - 84 tests
- [ ] CLI commands (list-sources, refresh, list, install, uninstall)
- [ ] FTP client for CrossWire sources
- [ ] Checksum verification
- [ ] 1:1 validation against official installmgr

### Phase: Versification Expansion (Pending)

- [ ] Add Catholic/Catholic2 verse counts
- [ ] Add NRSV/NRSVA verse counts
- [ ] Add Synodal/SynodalProt verse counts
- [ ] Output normalization to KJV versification
- [ ] Create docs/versification-mapping.md

### Phase: Diatheke Clone (Scheduled)

- [ ] Add `juniper diatheke` subcommand
- [ ] Reference parser (Genesis 1:1, Gen 1:1, ranges)
- [ ] Output format compatibility (plain, OSIS, HTML)
- [ ] Search functionality (regex, phrase, multiword)
- [ ] 1:1 validation against official diatheke

### Phase: Test Coverage Goals

- [ ] pkg/sword to 90%+
- [ ] pkg/output to 90%+
- [ ] pkg/repository to 80%+
- [ ] End-to-end verse comparison tests

## 5. Success Criteria

### Performance Targets

| Metric | Target |
|--------|--------|
| Single verse lookup | < 5ms |
| Full chapter conversion | < 50ms |
| Full book conversion | < 500ms |
| 66-book Bible conversion | < 30s |
| Output parity with diatheke | 99%+ |

### Quality Metrics

| Metric | Target |
|--------|--------|
| Overall test coverage | > 85% |
| Critical path coverage | > 95% |
| Integration test pass rate | 100% |
| CGo parity | Byte-exact for supported formats |

### Output Requirements

- JSON validates against schema
- All Strong's numbers preserved
- All morphology codes preserved
- Unicode text correctly normalized

## 6. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| SWORD format changes | High | Version locking, format docs in data_structures.md |
| Binary format edge cases | Medium | Extensive integration tests with real modules |
| CrossWire FTP instability | Low | HTTP fallback, cached index |
| CGo test dependency | Low | Mock CGo in CI, real tests on dev machines |
| Versification complexity | Medium | Comprehensive mapping tests, reference materials |

## 7. Stakeholders

### Owner/Maintainer

- **Justin Williams** - Primary developer

### Consumers

- Focus with Justin website (Hugo JSON consumer)
- Potential external Hugo sites

### External Dependencies

- CrossWire Bible Society (SWORD modules and format)
- e-Sword developer (database format)

## 8. Related Documentation

### Internal Documentation
| Document | Purpose |
|----------|---------|
| [README.md](../README.md) | Quick start and overview |
| [ARCHITECTURE.md](ARCHITECTURE.md) | System design and data flow |
| [data_structures.md](data_structures.md) | SWORD binary format reference |
| [TODO.md](../TODO.md) | Detailed task tracking |

### External References
| Resource | URL |
|----------|-----|
| CrossWire SWORD | https://crosswire.org/sword/ |
| CrossWire Wiki | https://wiki.crosswire.org/ |
| Alternate Versification | https://wiki.crosswire.org/Alternate_Versification |
| pysword books.py | https://gitlab.com/tgc-dk/pysword/-/blob/master/pysword/books.py |

### Parent Project

- [Focus with Justin Project Charter](../../../docs/project-charter.md)

## 9. Architecture

### Package Structure
```
pkg/
├── config/      # Configuration handling (100% coverage)
├── sword/       # SWORD format parsers (zText, zCom, zLD, RawGenBook)
├── esword/      # e-Sword SQLite parsers (.bblx, .cmtx, .dctx)
├── markup/      # Markup converters (OSIS, ThML, GBF, TEI)
├── output/      # Hugo JSON generators
├── migrate/     # File migration utilities
├── repository/  # Module repository management (installmgr replacement)
├── cgo/         # CGo libsword bindings (test reference)
└── testing/     # Test framework and fixtures
```

### Data Flow
```
SWORD/e-Sword Modules
        ↓
    [Migration]
        ↓
  sword_data/incoming/
        ↓
    [Conversion]
   ┌────┴────┐
   ↓         ↓
Parser    Markup
(zText)   (OSIS→MD)
   ↓         ↓
   └────┬────┘
        ↓
    [Output]
        ↓
  Hugo JSON files
        ↓
  Hugo Templates
```

### Versification Systems
| System | Books | Primary Use |
|--------|-------|-------------|
| KJV | 66 | Protestant Bibles |
| KJVA | 81 | Protestant with Apocrypha |
| Vulg | 76 | Catholic Bibles |
| LXX | Variable | Orthodox/Septuagint |
| MT | 39 | Hebrew/Masoretic |

---

**Charter Created:** 2026-01-01
**Last Updated:** 2026-01-01
**Next Review:** After InstallMgr replacement completion
