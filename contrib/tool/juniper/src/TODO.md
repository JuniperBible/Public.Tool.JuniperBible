# Juniper TODO

This file tracks detailed tasks for the Juniper submodule.
For the project charter, see [docs/PROJECT-CHARTER.md](docs/PROJECT-CHARTER.md).

---

## Test Coverage - IN PROGRESS

Goal: Achieve high test coverage across all juniper packages.

### Current Coverage (as of 2026-01-01)
| Package | Coverage | Status |
|---------|----------|--------|
| pkg/cgo | 100.0% | ✓ Complete |
| pkg/config | 100.0% | ✓ Complete |
| pkg/markup | 99.1% | ✓ Complete |
| pkg/migrate | 92.4% | ✓ Good |
| pkg/testing | 89.5% | ✓ Good |
| pkg/esword | 88.6% | ✓ Good |
| pkg/sword | 85.9% | ✓ Good |
| pkg/output | 83.9% | ✓ Good |
| cmd/sectest | 64.3% | Needs improvement |
| pkg/repository | 64.4% | Needs improvement |
| cmd/extract | 31.3% | Needs improvement |
| cmd/juniper | 0.0% | Needs improvement (CLI code) |

### Test Requirements

- All verse comparisons must be done verse-by-verse
- Each verse must be compared individually against reference output
- Use Go's table-driven tests for comprehensive verse testing
- Mock diatheke output for deterministic testing

### Remaining Work

- [ ] Add more tests for pkg/sword to reach 90%+
- [ ] Add more tests for pkg/output to reach 90%+
- [ ] Add more tests for pkg/repository (FTP functions require mocking)
- [ ] Add end-to-end verse comparison tests

---

## InstallMgr Replacement - IN PROGRESS

Replace the SWORD `installmgr` tool with a native Go implementation in juniper.

### Phase 1: Core Infrastructure (Tests First) - COMPLETE

- [x] Write tests for source parsing (source_test.go) - 15 tests
- [x] Write tests for default sources
- [x] Implement `Source` struct and parser
- [x] Implement default source list
- [x] Write tests for HTTP client (client_test.go) - 12 tests
- [x] Write tests for directory listing
- [x] Write tests for file download
- [x] Implement HTTP client with timeout/retry
- [x] Write tests for tar.gz extraction (index_test.go) - 25 tests
- [x] Write tests for .conf file parsing
- [x] Write tests for module metadata extraction
- [x] Implement index parser
- [x] Write tests for local config (config_test.go) - 18 tests
- [x] Write tests for installer (installer_test.go) - 14 tests
- [x] Archive baseline tool for 1:1 testing (attic/baseline/installmgr/)

Total: 84 tests passing in pkg/repository/

### Phase 2: CLI Commands - IN PROGRESS

- [x] Create `cmd/juniper/repo.go` with repo subcommand
- [ ] Add `juniper repo list-sources` command
  - [ ] Write test for list-sources output format
  - [ ] Implement command to match installmgr -s format
  - [ ] Compare 1:1 with `installmgr -s` output
- [ ] Add `juniper repo refresh <source>` command
  - [ ] Write test for refresh downloading mods.d.tar.gz
  - [ ] Implement FTP client for CrossWire FTP sources
  - [ ] Compare 1:1 with `installmgr -r` behavior
- [ ] Add `juniper repo list <source>` command
  - [ ] Write test for module listing output format
  - [ ] Implement command to match installmgr -rl format
  - [ ] Compare 1:1 with `installmgr -rl` output
- [ ] Add `juniper repo install <source> <module>` command
  - [ ] Write test for module download and extraction
  - [ ] Write test for conf file installation
  - [ ] Compare installed files 1:1 with `installmgr -ri`
- [ ] Add `juniper repo installed` command
  - [ ] Write test for listing installed modules
  - [ ] Implement command to match installmgr -l format
  - [ ] Compare 1:1 with `installmgr -l` output
- [ ] Add `juniper repo uninstall <module>` command
  - [ ] Write test for module removal
  - [ ] Compare cleanup 1:1 with `installmgr -u`

### Phase 3: FTP Client Implementation

- [ ] Add FTP client using github.com/jlaffaye/ftp
- [ ] Write tests for FTP connection (mock)
- [ ] Write tests for FTP directory listing
- [ ] Write tests for FTP file download
- [ ] Implement timeout and retry logic
- [ ] Test against real CrossWire FTP server (integration test)

### Phase 4: Enhanced Features

- [ ] Add checksum verification for downloaded modules
- [ ] Add module integrity check (verify data files exist)
- [ ] Add version comparison for updates
- [ ] Cache module index locally (~/.sword/cache/)
- [ ] Support offline listing from cache
- [ ] Incremental index updates (delta downloads)

### Phase 5: 1:1 Validation

- [ ] Run compare_sources.sh and verify identical output
- [ ] Run compare_modules.sh and verify identical output
- [ ] Run test_install.sh and verify identical file installation
- [ ] Document any intentional differences

---

## Versification System - PHASES 1-3, 5 COMPLETE

**Goal:** Proper versification mapping for all Bible translations (KJV, Vulgate, LXX, etc.)

### Background
SWORD modules use different versification systems that affect verse indexing:

- **KJV** - Protestant 66-book canon (default)
- **KJVA** - KJV with Apocrypha
- **Vulg** - Latin Vulgate (Catholic, different Psalm numbering, deuterocanon)
- **LXX** - Septuagint (Greek OT, additional books, different chapter/verse counts)
- **Catholic/Catholic2** - Varying Esther chapter counts (10 or 16)
- **Leningrad/MT** - Masoretic Text Hebrew
- **NRSV/NRSVA** - New Revised Standard Version with/without Apocrypha
- **Synodal/SynodalProt** - Russian Orthodox

### Key Differences to Handle

1. **Psalm numbering**: LXX/Vulgate Psalm N = Hebrew/KJV Psalm N+1 (for Psalms 10-146)
2. **Deuterocanonical books**: Tobit, Judith, Wisdom, Sirach, Baruch, 1-2 Maccabees
3. **Additional LXX books**: 1 Esdras, Prayer of Manasseh, Psalm 151, 3-4 Maccabees
4. **Split/merged verses**: Some verses combined or split across versifications
5. **Book order**: Different canonical ordering across traditions

### Phase 1: Data Structures ✅ COMPLETE

- [x] Create `pkg/sword/versification_systems.go`
- [x] Define KJV verse counts (versification_kjv.go) - 66 books
- [x] Define KJVA verse counts (versification_kjva.go) - 81 books
- [x] Define Vulg verse counts (versification_vulg.go) - 76 books
- [x] Define LXX verse counts (versification_lxx.go)
- [x] Add versification_systems_test.go

**Remaining Phase 1 tasks:**

- [ ] Define Catholic/Catholic2 verse counts
- [ ] Define NRSV/NRSVA verse counts
- [ ] Define Synodal/SynodalProt verse counts
- [ ] Define Leningrad/MT verse counts

### Phase 2: Verse Mapping Functions ✅ COMPLETE

- [x] Create `pkg/sword/verse_mapper.go`
- [x] Create `pkg/sword/verse_mapper_test.go`
- [x] Implement Vulg ↔ KJV mappings
- [x] Implement LXX ↔ KJV mappings

### Phase 3: Parser Integration ✅ COMPLETE

- [x] Update `pkg/sword/ztext.go` with versification support
- [x] Add NewZTextParserWithVersification()
- [x] Add Versification() accessor method

### Phase 4: Output Normalization - PENDING

- [ ] Update `pkg/output/json.go`
  - Option to normalize all output to KJV versification
  - Include original versification references in JSON
  - Handle deuterocanonical books with proper DC flag
- [ ] Add versification info to bibles.json metadata
- [ ] Create verse mapping data for frontend cross-reference

### Phase 5: Testing & Validation ✅ COMPLETE

- [x] Extend versification_systems_test.go
- [x] Integration tests with real modules
- [x] Verify all 8 current Bibles parse correctly

### Phase 6: Documentation - PENDING

- [x] Update README.md with versification support
- [ ] Create docs/versification-mapping.md

### Current Files
```
pkg/sword/
├── versification_systems.go      # Core types, registry
├── versification_systems_test.go # Tests
├── versification_kjv.go          # KJV 66-book system
├── versification_kjva.go         # KJVA 81-book system
├── versification_vulg.go         # Vulgate 76-book system
├── versification_lxx.go          # Septuagint system
├── verse_mapper.go               # Cross-versification mapping
├── verse_mapper_test.go          # Mapping tests
├── versification.go              # Legacy KJV-only
└── integration_test.go           # Real module tests
```

### Reference Materials

- [CrossWire Alternate Versification](https://wiki.crosswire.org/Alternate_Versification)
- SWORD source: `include/canon_vulg.h`, `include/canon_lxx.h`
- [JSword versification classes](https://github.com/crosswire/jsword)
- [pysword books.py](https://gitlab.com/tgc-dk/pysword/-/blob/master/pysword/books.py)

---

## SWORD/e-Sword Converter - COMPLETE

### Phase 1: Foundation ✅

- [x] Go project structure
- [x] SWORD .conf INI parser
- [x] Versification mappings (KJV)
- [x] Migration package
- [x] CLI with cobra

### Phase 2: SWORD Parsing ✅

- [x] zText parser (ZIP-compressed Bible text)
- [x] OSIS → Markdown converter
- [x] Strong's numbers preservation

### Phase 3: Markup Converters ✅

- [x] ThML converter
- [x] GBF converter
- [x] TEI converter

### Phase 4: Additional Formats ✅

- [x] RawGenBook parser (for Quran, Ethiopian texts) - 2025-12-30
      - pkg/sword/rawgenbook.go with TreeKey support
      - Tested with KORAN (115 surahs), Enoch (112 entries), Jubilees (52 entries)
- [ ] e-Sword .bblx parser

### Phase 5: Hugo Integration ✅

- [x] JSON output for Hugo data files
- [x] Book/chapter/verse structure
- [x] Metadata extraction

### Phase 6: Testing ✅

- [x] Unit tests for all parsers
- [x] Integration tests with real modules (22 tests)
      - zText: KJV, DRC, Vulgate, Geneva1599, Tyndale (5 Bibles)
      - zCom: Barnes NT Commentary (24 entries Matthew 1)
      - zLD: StrongsGreek (5742 entries), AbbottSmith (5886 entries)
      - RawGenBook: KORAN, Enoch, Jubilees
- [x] Fuzz tests for markup converters
- [x] Fixed zLD parser for binary .idx format (8-byte entries: offset+size into .dat)
- [x] Fixed zCom parser for NT-only modules (CalculateNTVerseIndex)

---

## Book Filtering (2026-01-01) - COMPLETE

Fixed issue where book selectors showed books with no content.

- [x] Skip books with no verse content in json.go
- [x] Added ExcludedBooks field to track excluded books
- [x] Added tests for ExcludedBook struct

Results:

- OSMHB: 39 OT books (was 66), 27 excluded
- SBLGNT: 27 NT books, 39 excluded
- pkg/output coverage: 70.1% → 83.9%

---

## Diatheke Clone Mode - SCHEDULED

**Goal:** Add a `diatheke` compatibility mode to juniper that replicates the official
SWORD diatheke CLI tool's functionality using our pure Go implementation.

**Use Case:** Replace the system diatheke binary with our Go tool for faster extraction,
better cross-platform support, and integration with the juniper pipeline.

**Reference:** Official diatheke at https://crosswire.org/sword/

### Phase 1: Core CLI Implementation

- [ ] Add `juniper diatheke` subcommand
  - Mirror official diatheke CLI flags
  - Use existing zText/zCom/zLD parsers
- [ ] Implement key flags
  - `-b <module>` - Bible/module name (required)
  - `-k <key>` - Verse reference or search key (required)
  - `-f <format>` - Output format (plain, RTF, HTML, OSIS, ThML, GBF)
  - `-o <option>` - Additional options (n=verse numbers, f=footnotes, s=Strong's)
  - `-l <locale>` - Locale for book names
  - `-m <maxverses>` - Maximum verses to return
- [ ] Implement reference parser
  - Parse "Genesis 1:1", "Gen 1:1", "Gen.1.1" formats
  - Support ranges: "Genesis 1:1-5", "Genesis 1-2"
  - Support book-only: "Genesis" (full book)
  - Support chapter-only: "Genesis 1" (full chapter)
- [ ] Implement module auto-detection
  - Read module type from .conf file
  - Route to appropriate parser (zText, zCom, zLD)

### Phase 2: Output Format Compatibility

- [ ] Implement plain text output (-f plain)
- [ ] Implement OSIS output (-f OSIS)
- [ ] Implement HTML output (-f HTML)
- [ ] Implement RTF output (-f RTF)
- [ ] Add formatting options (-o flags)

### Phase 3: Search Functionality

- [ ] Implement text search (-s <type>)
  - regex: Regular expression search
  - phrase: Exact phrase search
  - multiword: Multiple words (AND)
- [ ] Add search range limiting

### Phase 4: Additional Module Support

- [ ] Dictionary/Lexicon support
- [ ] Commentary support
- [ ] GenBook support

### Phase 5: 1:1 Validation

- [ ] Create comparison test script
- [ ] Test with all 8 website Bibles
- [ ] Test edge cases
- [ ] Document intentional differences

### Phase 6: Testing

- [ ] Unit tests for reference parser
- [ ] Unit tests for formatters
- [ ] Unit tests for module detection
- [ ] Integration tests with real modules
- [ ] 1:1 comparison tests with official diatheke
- [ ] Performance benchmarks

### Success Criteria
| Metric | Target |
|--------|--------|
| Single verse lookup | < 5ms |
| Full chapter lookup | < 50ms |
| Full book lookup | < 500ms |
| Output parity with diatheke | 99%+ |
| Reference parsing accuracy | 100% |

---

## Completed Items

### Tool Migration to Go (2026-01-01) ✅

- [x] Renamed sword-converter to `juniper`
- [x] Restructured CLI
- [x] Moved Python extractors to attic
- [x] Converted security-test.sh to Go
- [x] Added all 27 SCRIPTURE modules
