# Religion Section Guide

The religion section at `/religion/` provides a comprehensive multi-tradition Bible comparison tool with 8 translations across multiple versification systems and 14 canonical traditions.

## Features

### Bible Translations

| Translation | ID | Language | Versification | Features |
|-------------|-----|----------|---------------|----------|
| King James Version | `kjv` | English | Protestant | Strong's Numbers |
| Douay-Rheims | `drc` | English | Catholic | Deuterocanon |
| Geneva Bible | `geneva1599` | English | Protestant | Historic |
| Latin Vulgate | `vulgate` | Latin | Catholic | Source Text |
| Tyndale Bible | `tyndale` | English | Protestant | Historic |
| Septuagint (LXX) | `lxx` | Greek | Orthodox | Strong's Numbers |
| Open Scriptures Hebrew | `osmhb` | Hebrew | Leningrad | Source Text |
| SBL Greek NT | `sblgnt` | Greek | Protestant | Source Text |

### Canonical Comparison

Compare which books are in each tradition's canon across 14 traditions:

- Roman Catholic, Eastern Orthodox, Protestant, Evangelical
- Ethiopian Orthodox (114 books), Latter-day Saints (89 books)
- Anglican, Jehovah's Witnesses, Coptic Orthodox
- Armenian Apostolic, Syriac Orthodox, Assyrian, Samaritan, Orthodox Jewish

### STEPBible-Inspired Features

#### Parallel Translation View
Compare up to 4 translations side-by-side at `/religion/bibles/compare/`:
```
URL: /religion/bibles/compare/?bibles=kjv,drc,vulgate&ref=Gen.1
```

**Features:**

- Auto-load on chapter or translation selection
- Chapter grid for quick navigation
- State persistence via URL and localStorage
- Verse-by-verse alignment across translations
- Responsive layout (stacks on mobile)

#### Bible Search
Full-text search across translations at `/religion/bibles/search/`:

- Case-sensitive and whole-word options
- Results grouped by book with highlighted matches
- On-demand chapter loading for performance
- **Strong's number search**: `H430` (Hebrew) or `G2316` (Greek)
- **Phrase search**: `"exact phrase"` with quotation marks

#### Verse Sharing
Share verses via URL, clipboard, or social media:

- Format: `/religion/bibles/kjv/gen/1/?v=1`
- Copy to clipboard with visual feedback
- Twitter/X and Facebook share buttons
- Auto-scroll to verse when URL has `?v=` parameter

## Data Architecture

### Data Files

| File | Purpose | Size |
|------|---------|------|
| `data/bibles.json` | Translation metadata | ~2KB |
| `data/bibles_auxiliary/` | Per-Bible content | ~5MB each |
| `data/book_mappings.json` | Canonical status data | ~15KB |

### Directory Structure

```
data/bibles_auxiliary/
├── kjv.json          # King James Version content
├── drc.json          # Douay-Rheims content
├── geneva1599.json   # Geneva Bible content
├── vulgate.json      # Latin Vulgate content
└── tyndale.json      # Tyndale Bible content
```

### JSON Structure

**bibles.json (metadata):**
```json
{
  "bibles": [
    {
      "id": "kjv",
      "title": "King James Version",
      "abbrev": "KJV",
      "description": "1769 edition with Strong's Numbers",
      "language": "English",
      "features": ["Strong's Numbers"],
      "tags": ["protestant", "historic"],
      "weight": 1
    }
  ]
}
```

**Per-Bible auxiliary file (e.g., kjv.json):**
```json
{
  "books": [
    {
      "id": "Gen",
      "name": "Genesis",
      "abbrev": "Gen",
      "testament": "OT",
      "chapters": [
        {
          "number": 1,
          "verses": [
            {
              "number": 1,
              "text": "In the beginning God created..."
            }
          ]
        }
      ]
    }
  ]
}
```

## SWORD Converter Tool

### Overview

The `tools/juniper/` Go CLI extracts Bible text from SWORD modules (used by CrossWire Bible Society software). It includes a native Go replacement for the `installmgr` tool.

### Quick Start

```bash
cd tools/juniper
go build ./cmd/juniper
./juniper convert --output ../../data/
```

### SWORD Module Repository

The native Go repository manager (`juniper repo`) replaces CrossWire's `installmgr` tool:

```bash
# List remote sources (11 sources: CrossWire, eBible.org, IBT, etc.)
./juniper repo list-sources

# List available modules from a source
./juniper repo list CrossWire

# Install a module
./juniper repo install CrossWire KJV

# List installed modules
./juniper repo installed

# Verify module integrity (size check without redownload)
./juniper repo verify KJV

# Uninstall a module
./juniper repo uninstall KJV
```

**Repository Package Structure:**

| Package | Purpose |
|---------|---------|
| `source.go` | Remote source definitions (11 FTP sources) |
| `client.go` | HTTP/FTP client with retry/timeout support |
| `ftp.go` | FTP protocol implementation |
| `index.go` | Module index parsing (mods.d.tar.gz) |
| `localconfig.go` | Local configuration (~/.sword) |
| `installer.go` | Module installation/uninstallation/verification |

**Supported Sources:**

| Source | Type | Description |
|--------|------|-------------|
| CrossWire | FTP | Main SWORD repository |
| CrossWire Beta | FTP | Beta/testing modules |
| CrossWire Attic | FTP | Archived modules |
| eBible.org | FTP | Public domain translations |
| IBT | FTP | Institute for Bible Translation |
| STEP Bible | FTP | Tyndale House modules |
| Xiphos | FTP | Xiphos Bible software |
| And 4 more... | | |

**Module Verification:**

Verify installed modules without redownloading:
```bash
./juniper repo verify        # Verify all
./juniper repo verify KJV    # Verify single module
```

Checks performed:

- Conf file exists
- Data directory has files
- Size matches `InstallSize` metadata (if available)

Modules are stored in `~/.sword/`:
```
~/.sword/
├── mods.d/          # Module configuration (.conf files)
└── modules/         # Binary data files
    └── texts/ztext/ # Compressed Bible text
```

### SWORD Binary Format

The zText format uses three files per testament:

| File | Purpose |
|------|---------|
| `ot.bzs` / `nt.bzs` | Block index (12 bytes/entry) |
| `ot.bzv` / `nt.bzv` | Verse index (10 bytes/entry) |
| `ot.bzz` / `nt.bzz` | Compressed text (zlib) |

See [data_structures.md](data_structures.md) for detailed binary format documentation.

### RawGenBook Format (General Books)

The RawGenBook format stores hierarchical content like creeds, commentaries, and the Quran:

| File | Purpose |
|------|---------|
| `*.bdt` | Binary content data |
| `*.idx` | 4-byte offsets into .bdt file |
| `*.dat` | TreeKey structure with entry keys |

**TreeKey Structure:**

- 8-byte marker (`0xFFFFFFFF 0xFFFFFFFF`)
- Metadata bytes (offset, size, flags)
- Null-terminated UTF-8 key string

Parser: `pkg/sword/rawgenbook.go`

### Verse Index Calculation

The SWORD verse index includes intro entries, not just verses:

```
Entry 0: Placeholder (empty)
Entry 1: Testament header
Entry 2: Book introduction
Entry 3: Chapter 1 introduction
Entry 4+: Actual verses (Genesis 1:1 starts here)
```

Formula (derived from pysword):
```go
index = 2 + bookOffset + chapterOffset + (verse - 1)

bookOffset = sum of all previous books' sizes
bookSize = totalVerses + numChapters + 1

chapterOffset = sum of previous chapters' verses + chapter count + 1
```

## Versification System

SWORD modules use different versification systems that affect verse indexing. The converter auto-detects the versification from the module's `.conf` file.

### Implemented Systems

| System | Books | Description | Modules |
|--------|-------|-------------|---------|
| KJV | 66 | Protestant standard | KJV, Geneva, Tyndale, SBLGNT |
| KJVA | 81 | KJV + 15 Apocrypha | KJV with Apocrypha |
| Vulg | 76 | Catholic with deuterocanon | Vulgate, DRC |
| LXX | 86 | Septuagint Greek OT | LXX |
| Leningrad | 39 | Masoretic Hebrew OT | OSMHB |

### Key Differences

1. **Psalm numbering**: LXX/Vulgate Psalm N = KJV Psalm N+1 (for Psalms 10-146)
2. **Deuterocanonical books**: Tobit, Judith, Wisdom, Sirach, Baruch, 1-2 Maccabees
3. **Additional LXX books**: 1 Esdras, Prayer of Manasseh, Psalm 151, 3-4 Maccabees
4. **Book order**: Different canonical ordering across traditions

### Implementation Files

```
tools/juniper/pkg/sword/
├── versification_systems.go      # Core types, registry
├── versification_kjv.go          # KJV 66-book system
├── versification_kjva.go         # KJVA 81-book system
├── versification_vulg.go         # Vulgate 76-book system
├── versification_lxx.go          # Septuagint system
├── verse_mapper.go               # Cross-versification mapping
└── verse_mapper_test.go          # Mapping tests
```

### Verse Mapping

The `VerseMapper` handles cross-versification references:

```go
mapper := NewVerseMapper()
// Map Vulgate Psalm 22:1 to KJV Psalm 23:1
kjvRef := mapper.MapToKJV("Ps", 22, 1, "Vulg")
// Returns: {Book: "Ps", Chapter: 23, Verse: 1}
```

## Hugo Integration

### Extension Location

```
themes/airfold/extensions/religion/
├── content/bibles/_content.gotmpl  # Page generator
├── layouts/bibles/
│   ├── list.html                   # Bible list page
│   └── single.html                 # Chapter view
└── README.md
```

### Module Mounts

In `hugo.toml`:
```toml
[[module.mounts]]
  source = 'themes/airfold/extensions/religion/content/bibles'
  target = 'content/religion/bibles'
```

### Templates

**Page generation (_content.gotmpl):**

- Reads `data/bibles.json` and `data/bibles_auxiliary/`
- Generates: Bible overview, book index, chapter pages
- Uses `$.AddPage` for dynamic page creation

**Layouts:**

- `list.html` - Bible index with canonical comparison table
- `single.html` - Chapter view with verse display and navigation

## Adding a New Translation

### Quick Start (just commands)

Use `just` commands for Bible management:

| Command | Description |
|---------|-------------|
| `just bible-sources` | List remote SWORD sources |
| `just bible-list` | List installed modules with metadata |
| `just bible-available [source]` | List modules from a source (default: CrossWire) |
| `just bible-download <module> [source]` | Download a module |
| `just bible-remove <module>` | Uninstall a module |
| `just bible-verify [module]` | Verify module integrity (size check) |
| `just bible-convert` | Convert installed modules to Hugo JSON |
| `just bible-add <module> [source]` | Complete workflow: download + convert |

**Bulk Download Commands:**

| Command | Description |
|---------|-------------|
| `just bible-download-all [source]` | Download all from a source, skip installed |
| `just bible-download-mega` | Download from ALL sources, skip installed |
| `just bible-download-all-verify [source]` | Verify all from source are installed |
| `just bible-download-mega-verify` | Verify all from ALL sources are installed |

Example: `just bible-add DRC` to add the Douay-Rheims Catholic Bible.

### Manual Steps

#### 1. List Available Modules

```bash
cd tools/juniper

# List remote sources
./juniper repo list-sources

# List available modules from CrossWire
./juniper repo list CrossWire

# List installed modules
./juniper repo installed
```

#### 2. Download SWORD Module

```bash
./juniper repo install CrossWire <ModuleName>
```

Popular modules:

- `DRC` - Douay-Rheims Catholic Bible
- `Vulgate` - Latin Vulgate
- `Geneva` - Geneva Bible 1599
- `AKJV` - American King James Version
- `WEB` - World English Bible

#### 3. Verify Installation

```bash
./juniper repo verify <ModuleName>
```

#### 4. Convert to Hugo JSON

```bash
cd tools/juniper
nix-shell -p python3Packages.pyyaml sword --run "python3 extract_scriptures.py data/"
```

Or using the Go converter:
```bash
./juniper convert --modules <ModuleName> --output ../../data/
```

#### 5. Register in bibles.json

Add entry to `data/bibles/bibles.json`:
```json
{
  "id": "modulename",
  "name": "Full Bible Name",
  "abbrev": "ABBR",
  "language": "en",
  "year": 1600
}
```

#### 6. Build and Verify

```bash
npm run build
npm run dev
# Check /religion/bibles/<id>/
```

## JavaScript Components

### parallel.js
Handles parallel translation view:

- Dynamic translation switching (auto-updates on change)
- URL parameter handling (`?bibles=kjv,drc&ref=Gen.1`)
- On-demand chapter fetching with caching
- Chapter grid for quick navigation
- Max 11 translations at once
- SSS (Side by Side Scripture) mode with two-pane layout

### bible-search.js
Full-text search functionality:

- On-demand chapter loading
- Search result caching
- Keyword highlighting
- Search types:
  - Text search: `word` or `multiple words`
  - Phrase search: `"exact phrase"` (quoted)
  - Strong's search: `H430` or `G2316` (Hebrew/Greek)

### share.js
Verse sharing features:

- Generate shareable URLs
- Copy to clipboard
- Social media integration

### strongs.js
Strong's number integration:

- Detect H#### and G#### patterns
- Link to Blue Letter Bible
- Keyboard accessible

## Related Documentation

- [Architecture Guide](architecture.md)
- [SWORD Binary Format](data_structures.md)
- [STEPBible Charter](stepbible-interface-charter.md)
- [SWORD Converter README](../tools/juniper/README.md)
