# Testing Guide

This guide covers the test strategy, available tests, and coverage goals for the project.

## Test Layers

The project uses multiple testing layers:

| Layer | Scope | Tools |
|-------|-------|-------|
| Unit Tests | Individual functions | Go `testing`, Jest |
| Integration Tests | Full pipelines | Go tests with real data |
| Golden File Tests | Regression detection | Fixture comparison |
| Security Tests | Contact form, CAPTCHA | Shell scripts |
| Build Tests | Hugo compilation | npm scripts |

## Running Tests

### Quick Commands

```bash
# Hugo build test
npm run build

# SWORD converter tests
npm run test:sword

# Security tests (production)
npm run test:security

# Security tests (local)
npm run test:security:local
```

### SWORD Converter Tests

```bash
cd tools/juniper

# All tests
go test ./...

# Verbose output
go test -v ./...

# Specific package
go test -v ./pkg/sword/...

# Run only integration tests
go test -run Integration ./...

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Update golden files
UPDATE_GOLDEN=1 go test ./...

# Fuzz testing
go test -fuzz=FuzzOSISConverter -fuzztime=30s ./pkg/markup/
```

## Juniper Test Coverage

### Current Coverage (2026-01-01)

| Package | Coverage | Status |
|---------|----------|--------|
| pkg/cgo | 100.0% | ✓ Complete |
| pkg/config | 100.0% | ✓ Complete |
| pkg/markup | 99.1% | ✓ Complete |
| pkg/migrate | 92.4% | ✓ Good |
| pkg/testing | 89.5% | ✓ Good |
| pkg/esword | 88.6% | ✓ Good |
| pkg/sword | 86.0% | ✓ Good |
| pkg/output | 83.9% | ✓ Good |
| cmd/sectest | 64.3% | Needs improvement |
| pkg/repository | 64.4% | Needs improvement |
| cmd/extract | 31.3% | Needs improvement |
| cmd/juniper | 13.5% | CLI code (hard to test) |

**Total:** 350+ tests across 28+ test files

### Test Requirements

- All verse comparisons must be done **verse-by-verse**
- Each verse must be compared individually against reference output
- Use Go's table-driven tests for comprehensive verse testing
- Mock diatheke output for deterministic testing

### Test Categories

#### Core Infrastructure (Phase 1)
- `pkg/config/config_test.go` - YAML config parsing, path expansion
- `pkg/sword/conf_test.go` - SWORD .conf parsing, RTF escapes
- `pkg/sword/versification_test.go` - Book lookups, indices, aliases

#### Markup Converters (Phase 2)
- `pkg/markup/osis_test.go` - OSIS XML to Markdown
- `pkg/markup/thml_test.go` - ThML markup conversion
- `pkg/markup/gbf_test.go` - GBF format handling
- `pkg/markup/tei_test.go` - TEI dictionary entries

#### e-Sword Parsers (Phase 3)
- `pkg/esword/bible_test.go` - SQLite Bible parsing
- `pkg/esword/commentary_test.go` - Commentary entries
- `pkg/esword/dictionary_test.go` - Dictionary/lexicon

#### Binary Format Parsers (Phase 4)
- `pkg/sword/ztext_test.go` - Compressed Bible text
- `pkg/sword/zcom_test.go` - Compressed commentaries
- `pkg/sword/zld_test.go` - Compressed dictionaries

#### Output Generation (Phase 5)
- `pkg/output/json_test.go` - JSON file generation

#### Migration (Phase 6)

- `pkg/migrate/migrator_test.go` - Module file operations

#### Robustness (Phase 7)
- `pkg/markup/fuzz_test.go` - Fuzz testing for markup
- `pkg/sword/fuzz_test.go` - Fuzz testing for parsing

## Integration Tests

### Real SWORD Module Tests

Integration tests run against actual SWORD modules when available:

```bash
# Requires modules in ~/.sword
SWORD_PATH=~/.sword go test -run Integration ./...
```

**Tests included:**
- `TestIntegration_LoadKJV` - Load KJV module
- `TestIntegration_KJV_Genesis1` - Parse Genesis 1:1
- `TestIntegration_KJV_John316` - Verify verse content
- `TestIntegration_KJV_Psalm23` - Hebrew poetry
- `TestIntegration_KJV_AllBooks` - All 66 books
- `TestIntegration_MultipleModules` - Load all available modules

### Tool Comparison Tests

Validate Go extractor against Python/diatheke:

```bash
go test -v -run TestToolComparison ./pkg/testing/
```

**Validates:**

- JSON schema compliance
- Verse counts match
- Book ordering consistency
- Unicode handling
- Chapter/verse numbering

**Note:** The slow extractor comparison test (`TestToolComparison_OutputStructureMatch`)
requires an explicit opt-in:

```bash
RUN_EXTRACTOR_TESTS=1 go test -v -run TestToolComparison_OutputStructureMatch ./pkg/testing/
```

## Security Tests

The security test suite validates the contact form:

```bash
# Test production
npm run test:security

# Test local dev
npm run test:security:local
```

### What It Tests

1. CAPTCHA widget renders on page
2. Submit button disabled until CAPTCHA complete
3. API rejects requests without token
4. API rejects fake tokens
5. API requires provider field
6. Worker rejects unsigned requests
7. Worker rejects bad signatures
8. Worker rejects expired timestamps

## Golden File Tests

Golden files store expected output for regression detection.

### Creating Golden Files

```bash
UPDATE_GOLDEN=1 go test -run TestGolden ./...
```

### Golden File Location

```
tools/juniper/
├── pkg/testing/testdata/
│   ├── golden/          # Expected outputs
│   └── fixtures.go      # Test input data
```

## Fuzz Testing

Fuzz tests find edge cases in parsers:

```bash
# Run for 30 seconds
go test -fuzz=FuzzOSISConverter -fuzztime=30s ./pkg/markup/

# Run indefinitely
go test -fuzz=FuzzOSISConverter ./pkg/markup/
```

### Fuzz Targets

| Target | Package | Tests |
|--------|---------|-------|
| `FuzzOSISConverter` | markup | Random OSIS XML |
| `FuzzThMLConverter` | markup | Random ThML |
| `FuzzGBFConverter` | markup | Random GBF |
| `FuzzTEIConverter` | markup | Random TEI |
| `FuzzParseAboutText` | sword | Random RTF |
| `FuzzNormalizeBookID` | sword | Random book refs |

## Test Helpers

### e-Sword Database Helpers

Create test databases for SQLite testing:

```go
// pkg/esword/testhelpers_test.go
db := CreateTestBibleDB(t, map[string]string{
    "Gen 1:1": "In the beginning...",
})

db := CreateTestCommentaryDB(t, []TestEntry{...})
db := CreateTestDictionaryDB(t, []TestDictEntry{...})
```

### SWORD Binary Helpers

Create synthetic SWORD modules for testing:

```go
// pkg/sword/ztext_test.go
path := createTestZTextModule(t, map[string]string{
    "Gen.1.1": "In the beginning...",
})
```

## Edge Cases Covered

### Unicode
- Hebrew RTL with combining marks: `בְּרֵאשִׁית`
- Greek polytonic: `Ἐν ἀρχῇ ἦν ὁ λόγος`
- NFC normalization roundtrip

### Strong's Numbers

- H/G prefixes, leading zeros (H0430)
- Malformed: empty, non-numeric
- Duplicates in same verse
- Nested in red-letter markup

### Binary Formats
- Empty verse entries (Size = 0)
- Testament boundaries (OT→NT)
- Corrupted zlib data
- Truncated index files

### SQLite
- NULL values, missing tables
- Schema variations
- Encoding (Latin-1 vs UTF-8)

### Markup

- Unclosed tags
- Nested elements
- Empty tags
- Very long content (10K+ chars)

## Tiered Testing

Test infrastructure with three levels (implemented via justfile):

| Tier | Time | Coverage | Scope |
|------|------|----------|-------|
| Quick | < 30s | N/A | 5 bibles, integration only |
| Comprehensive | < 5m | 80% | 100 bibles, full suite |
| Extensive | < 30m | 90% | All modules, fuzz tests |

```bash
# Tiered test commands
just test-quick          # Pre-deploy validation
just test-comprehensive  # Release validation
just test-extensive      # Full coverage
just benchmark           # Speed comparison

# Development helpers
just coverage            # Generate coverage report
just coverage-html       # Open HTML coverage report
just fmt                 # Format Go code
just vet                 # Run go vet
just deps                # Download dependencies
```

### Bible Sets Configuration

Test sets are configured in `tools/juniper/testdata/bible_sets.yaml`:

- **Quick (5 Bibles):** KJV, Tyndale, Geneva1599, DRC, Vulgate
- **Comprehensive (100 modules):** Major translations, original languages, commentaries
- **Extensive (All):** All available SWORD modules in `~/.sword`

## CI Integration

### npm Scripts

| Script | Purpose |
|--------|---------|
| `npm run build` | Hugo build test |
| `npm run test:sword` | SWORD converter tests |
| `npm run test:security` | Contact form security |

### GitHub Actions (if used)

```yaml
# Quick tests on every push
- run: npm run build
- run: npm run test:sword

# Full tests on main
- run: go test -coverprofile=coverage.out ./...
```

## Debugging Failed Tests

### Verbose Output

```bash
go test -v -run TestSpecificTest ./pkg/sword/
```

### Print Debug Info

```bash
DEBUG=1 go test ./...
```

### Compare with Reference

```bash
# Use libsword reference (if available)
go test -tags libsword -run TestCGo ./...
```

## Related Documentation

- [Development Guide](development.md)
- [Religion Section](religion-section.md)
- [SWORD Converter README](../tools/juniper/README.md)
