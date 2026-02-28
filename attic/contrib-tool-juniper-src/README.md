# Juniper - Bible Swiss-Army-Knife

CLI toolkit for Bible module management.

## Features

- **Conversions**: SWORD → JSON, e-Sword → JSON
- **Downloads**: Fetch modules from SWORD repositories
- **Verifications**: Validate module integrity
- **Installations**: Install modules for use with Michael

## Supported Formats

### SWORD Modules

- zText (compressed Bible text)
- zCom (compressed commentary)
- zLD (compressed lexicon/dictionary)
- RawGenBook (general books)

### e-Sword Modules

- .bblx (Bible modules)
- .cmtx (Commentary modules)
- .dctx (Dictionary modules)

### Versification Systems

- KJV (Protestant standard)
- KJVA (KJV with Apocrypha)
- Vulgate (Catholic Latin)
- LXX (Septuagint)
- MT (Masoretic Text)
- Synodal (Orthodox)

## Installation

```bash
go install github.com/FocuswithJustin/juniper/cmd/juniper@latest
```

Or build from source:

```bash
git clone git@github.com:FocuswithJustin/juniper.git
cd juniper
go build ./cmd/juniper
```

## Usage

```bash
# List available modules
juniper list

# Download a module
juniper download KJV

# Convert SWORD to JSON
juniper convert --module KJV --output kjv.json

# Verify module integrity
juniper verify --module KJV

# Install for Michael
juniper install --module KJV --target ../michael/data/
```

## Companion Modules

| Module | Purpose |
|--------|---------|
| **Michael** | Hugo Bible extension (consumes Juniper output) |
| **AirFold** | Visual theme (CSS, base layouts) |
| **Gabriel** | Contact form functionality |

## Reference

See `attic` branch for pre-modularization working implementation.

## Status

Fresh build in progress. See [TODO.txt](TODO.txt) for current tasks.

## License

Copyright (c) 2024 - Present Justin. All rights reserved.
