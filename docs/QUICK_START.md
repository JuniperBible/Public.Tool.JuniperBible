# Juniper Bible - Quick Start Guide

This guide will help you get started with Juniper Bible for Bible text format conversion.

## Installation

### Prerequisites

- Go 1.21 or later
- Nix package manager (recommended for complete development environment)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/JuniperBible/juniper.git
cd mimicry

# Enter the development environment (recommended)
nix-shell
# This provides: Go, SQLite, SWORD tools, pandoc, calibre, and all dependencies

# Using Make (recommended)
make build      # Build the CLI
make plugins    # Build all plugins
make test       # Run tests

# Or build manually
go build -o capsule ./cmd/capsule

# Verify installation
./capsule version
./capsule plugins list
```

### Test Data and Third-Party Modules

Test data and third-party dependencies are distributed as separate Go modules:

```bash
# Import test data (sample Bibles, fixtures)
go get github.com/JuniperBible/juniper/test-data@test-data

# Import third-party tool references
go get github.com/JuniperBible/juniper/test-contrib@test-contrib
```

```go
// In your test files
import (
    "github.com/JuniperBible/juniper/test-data/data"
    "github.com/JuniperBible/juniper/test-contrib/tools"
)

func TestWithSampleData(t *testing.T) {
    // Get sample SWORD module path
    kjvPath := data.SampleModule("kjv")

    // Get test fixture
    fixturePath := data.Fixture("sample.txt")

    // Get tool reference
    juniperSrc := tools.JuniperSrc()
}
```

### Development Environment (nix-shell)

The project includes a `shell.nix` that provides all development dependencies:

```bash
# Enter development shell
nix-shell

# What's included:
# - Go toolchain (go, gopls, gotools, delve)
# - CGO dependencies (sqlite, gcc, pkg-config)
# - Reference tools (sword, diatheke)
# - Document tools (pandoc, calibre)
# - XML tools (libxml2, libxslt)
# - Build utilities (make, git, jq)

# Build legacy juniper for comparison testing
make juniper-legacy
```

## Basic Conversions

### Convert Between Formats

The simplest way to convert files is using the `format convert` command:

```bash
# USFM to OSIS (L0 lossless)
./capsule format convert input.usfm --to osis --out output.osis

# OSIS to EPUB
./capsule format convert bible.osis --to epub --out bible.epub

# SWORD module to multiple formats
./capsule format convert ./kjv-module/ --to markdown --out kjv-md/
```

### Step-by-Step Conversion (for debugging)

For debugging or understanding the conversion pipeline:

```bash
# Step 1: Extract to Intermediate Representation
./capsule format ir extract input.usfm --format usfm --out intermediate.ir.json

# Step 2: Inspect the IR (optional)
./capsule format ir info intermediate.ir.json

# Step 3: Emit native format from IR
./capsule format ir emit intermediate.ir.json --format osis --out output.osis
```

### Batch Conversion

```bash
# Convert all USFM files in a directory to OSIS
for f in *.usfm; do
  ./capsule format convert "$f" --to osis --out "${f%.usfm}.osis"
done

# Convert with explicit format specification
./capsule format convert input-dir/ --from usfm --to osis --out output-dir/
```

## Working with Capsules

Capsules are the core storage format - immutable archives that preserve files byte-for-byte.

### Ingesting Files

```bash
# Ingest a single file
./capsule capsule ingest bible.osis --out bible.capsule.tar.xz

# Ingest a directory (e.g., SWORD module)
./capsule capsule ingest ./kjv-sword-module/ --out kjv.capsule.tar.xz
```

### Exporting from Capsules

```bash
# Export in original format (identity mode)
./capsule capsule export kjv.capsule.tar.xz --artifact main --out exported/

# Export with format conversion (derived mode)
./capsule capsule export kjv.capsule.tar.xz --artifact main --to epub --out kjv.epub
```

### Verifying Capsules

```bash
# Verify all hashes match
./capsule capsule verify my.capsule.tar.xz

# Run self-check plans
./capsule capsule selfcheck my.capsule.tar.xz --plan identity-bytes
```

## Format Detection

```bash
# Auto-detect file format
./capsule format detect myfile.xml
# Output: format.osis (confidence: high)

# List archive contents
./capsule capsule enumerate archive.zip
```

## Supported Formats

### L0 - Lossless (byte-identical round-trip)

- OSIS XML (`.osis`, `.xml`)
- USFM (`.usfm`, `.sfm`)
- USX (`.usx`)
- Zefania XML
- TheWord (`.ont`, `.nt`, `.twm`)
- JSON

### L1 - Semantically Lossless

- EPUB (`.epub`)
- HTML (`.html`)
- Markdown (`.md`)
- SQLite (`.db`, `.sqlite`)
- e-Sword (`.bblx`, `.cmtx`)
- Digital Bible Library bundles
- TEI XML

### L2 - Minor Metadata Loss

- SWORD modules
- RTF (`.rtf`)
- Logos/Libronix
- Accordance

### L3 - Text Only

- Plain text (`.txt`)
- GoBible (`.jar`)
- Palm Bible (`.pdb`)

## CLI Commands Reference

### Command Groups

| Group | Description |
|-------|-------------|
| `capsule` | Capsule lifecycle (ingest, export, verify, selfcheck, enumerate, convert) |
| `format` | Format detection and IR operations (detect, convert, ir) |
| `plugins` | Plugin management (list) |
| `tools` | Tool execution (list, archive, run, execute) |
| `runs` | Run transcripts (list, compare, golden save/check) |
| `juniper` | Bible/SWORD tools (list, ingest, cas-to-sword) |
| `dev` | Development tools (test, docgen) |
| `web` | Start web UI server |
| `api` | Start REST API server |
| `version` | Print version information |

### Capsule Commands
| Command | Description |
|---------|-------------|
| `capsule ingest` | Store files into a capsule |
| `capsule export` | Extract artifacts from a capsule |
| `capsule verify` | Verify capsule integrity |
| `capsule selfcheck` | Run verification plans |
| `capsule enumerate` | List archive contents |
| `capsule convert` | Convert capsule content to different format |

### Format Commands
| Command | Description |
|---------|-------------|
| `format detect` | Auto-detect file format |
| `format convert` | Convert between formats via IR |
| `format ir extract` | Extract IR from file |
| `format ir emit` | Generate native format from IR |
| `format ir generate` | Generate IR for capsule |
| `format ir info` | Display IR structure info |

### Plugin Commands
| Command | Description |
|---------|-------------|
| `plugins list` | List available plugins |

### Tool Commands
| Command | Description |
|---------|-------------|
| `tools run` | Execute tool plugin |
| `tools archive` | Archive tool binaries |
| `tools list` | List available tools |
| `tools execute` | Run tool on artifact and store transcript |

### Documentation Commands
| Command | Description |
|---------|-------------|
| `dev docgen plugins` | Generate plugin catalog |
| `dev docgen formats` | Generate format matrix |
| `dev docgen cli` | Generate CLI reference |
| `dev docgen all` | Generate all documentation |

### Juniper SWORD Commands
| Command | Description |
|---------|-------------|
| `juniper list` | List Bible modules in ~/.sword |
| `juniper ingest` | Ingest SWORD modules to capsules |

### Server Commands
| Command | Description |
|---------|-------------|
| `web` | Start web UI server (default: port 8080) |
| `api` | Start REST API server (default: port 8081) |

### Additional Capsule Operations
| Command | Description |
|---------|-------------|
| `capsule convert` | Convert capsule content to different format |
| `format ir generate` | Generate IR for capsule without one |

## Examples

### Starting the Web UI

```bash
# Start web UI with default settings
./capsule web

# Start on custom port with capsules directory
./capsule web --port 3000 --capsules ./my-capsules

# Open http://localhost:8080 (or :3000) in your browser
```

### Starting the REST API

```bash
# Start REST API with default settings
./capsule api

# Start on custom port
./capsule api --port 9000 --capsules ./my-capsules

# Test the API
curl http://localhost:8081/health
curl http://localhost:8081/plugins
```

### Importing SWORD Modules

```bash
# List all Bible modules in ~/.sword
./capsule juniper list

# Ingest specific modules to capsules
./capsule juniper ingest KJV DRC ESV

# Ingest all Bible modules
./capsule juniper ingest --all -o capsules/
```

### Converting Capsule Content

```bash
# Generate IR for a SWORD capsule (required before conversion)
./capsule format ir generate kjv.capsule.tar.gz

# Convert capsule content to OSIS
./capsule capsule convert kjv.capsule.tar.gz --to osis

# Note: Original is backed up as kjv-old.capsule.tar.gz
```

### Converting a SWORD Module to EPUB

```bash
# Ingest the SWORD module
./capsule capsule ingest ~/.sword/modules/texts/ztext/kjv/ --out kjv.capsule.tar.xz

# Convert to EPUB
./capsule capsule convert kjv.capsule.tar.xz --to epub --out kjv.epub
```

### Creating a SQLite Bible Database

```bash
# Convert OSIS to SQLite
./capsule format convert bible.osis --to sqlite --out bible.db

# Query the database
sqlite3 bible.db "SELECT * FROM verses WHERE book='Gen' AND chapter=1 LIMIT 10"
```

### Creating Markdown for Static Site

```bash
# Convert to Markdown (Hugo-compatible)
./capsule format convert bible.osis --to markdown --out site/content/bible/

# Each chapter becomes a separate file with frontmatter
```

## Troubleshooting

### Plugin Not Found
```bash
# Rebuild all plugins using Make
make plugins

# Or build manually
for plugin in plugins/format/*/; do
  name=$(basename "$plugin")
  go build -o "plugins/format/$name/format-$name" "./plugins/format/$name"
done
```

### Format Detection Fails
```bash
# Specify format explicitly
./capsule format convert input --from usfm --to osis --out output.osis
```

### Hash Verification Fails
```bash
# Check capsule integrity
./capsule capsule verify capsule.tar.xz

# If corrupt, re-ingest from original source
```

## Next Steps

- Read [API.md](generated/API.md) for complete API reference
- Read [INTEGRATION.md](INTEGRATION.md) to wrap the CLI in your application
- See [PLUGIN_DEVELOPMENT.md](PLUGIN_DEVELOPMENT.md) to create custom plugins
- Check [IR_IMPLEMENTATION.md](IR_IMPLEMENTATION.md) for IR system details
- See [VERSIFICATION.md](VERSIFICATION.md) for versification systems
- See [generated/CLI_REFERENCE.md](generated/CLI_REFERENCE.md) for complete CLI docs
