# External Dependencies (Clean Room)

This directory contains all external dependencies needed for comprehensive testing
and byte-for-byte reproducibility. All dependencies are isolated from the main
codebase for a clean room approach.

## Directory Structure

```
vendor_external/
├── sword_modules/      # SWORD Bible modules (binary)
├── esword_modules/     # e-Sword modules (SQLite databases)
├── libsword/           # libsword C++ library headers and binaries
├── diatheke/           # diatheke binary for reference testing
├── spdx/               # SPDX license definitions
├── versifications/     # Versification system definitions
└── testdata/           # Golden files and test fixtures
```

## Fetching Dependencies

Run the fetch script to download all external dependencies:

```bash
./fetch_all.sh
```

Or fetch individual categories:

```bash
./fetch_sword_modules.sh    # SWORD Bible modules
./fetch_esword_modules.sh   # e-Sword modules
./fetch_libsword.sh         # libsword library
./fetch_spdx.sh             # SPDX license data
```

## SWORD Modules

Required modules for comprehensive testing:

### Quick Set (5 modules)
- KJV - King James Version
- Tyndale - Tyndale's Bible
- Geneva1599 - Geneva Bible 1599
- DRC - Douay-Rheims Catholic
- Vulgate - Latin Vulgate

### Comprehensive Set (100+ modules)
See `sword_modules/module_list.txt` for full list.

## e-Sword Modules

Sample e-Sword databases for SQLite parsing tests.

## libsword (Optional)

For CGo reference validation:
- Headers: `swmgr.h`, `swmodule.h`, `versekey.h`, etc.
- Library: `libsword.so` / `libsword.dylib` / `sword.dll`

## SPDX Licenses

SPDX license list for license validation:
- Source: https://github.com/spdx/license-list-data
- Format: JSON

## Versification Systems

Canonical versification definitions:
- Protestant (66 books)
- Catholic (73 books)
- Ethiopian Orthodox (81 books)
- Tanakh (Hebrew Bible)
- LXX/Septuagint

## Integrity Verification

All downloaded files are verified against checksums in `checksums.sha256`.

```bash
sha256sum -c checksums.sha256
```

## Clean Room Principles

1. **Isolation**: No external dependencies in main codebase
2. **Reproducibility**: All versions pinned and checksummed
3. **Documentation**: Every dependency documented with source
4. **Verification**: Integrity checks for all downloads
5. **Separation**: Test data never mixed with production code
