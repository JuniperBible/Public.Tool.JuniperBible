# Reference Conversion Tools

This directory contains documentation, installation instructions, and reference copies of the defacto standard Bible format conversion tools.

## Directory Structure

```
contrib/tool/
├── tool_name/
│   ├── README.md       # Tool documentation and usage
│   ├── LICENSE.txt     # Tool license
│   ├── nixos/          # NixOS package definitions for offline use
│   │   └── default.nix # Nix derivation
│   ├── capsule/        # Capsule archives of tool binaries
│   ├── bin/            # Official binaries (when available)
│   └── src/            # Source code (when available)
```

## Available Tools

### Bible-Specific Tools

| Tool | Purpose | Format Support | License |
|------|---------|----------------|---------|
| sword-utils | SWORD module manipulation | SWORD, OSIS, IMP | GPL-2.0+ |
| usfm2osis | USFM to OSIS conversion | USFM → OSIS | GPL-3.0 |
| osis2mod | OSIS to SWORD module | OSIS → SWORD | GPL-2.0+ |
| gobible-creator | GoBible J2ME apps | OSIS → GoBible | GPL-2.0 |

### General-Purpose Tools

| Tool | Purpose | Format Support | License |
|------|---------|----------------|---------|
| pandoc | Universal document converter | Markdown, HTML, EPUB, RTF, ODT | GPL-2.0+ |
| calibre | E-book toolkit | EPUB creation/validation | GPL-3.0 |
| sqlite | SQL database engine | SQLite, e-Sword | Public Domain |
| libxml2 | XML toolkit (xmllint, xsltproc) | OSIS, USX, Zefania, TEI, XML | MIT |
| unrtf | RTF converter | RTF parsing | GPL-3.0 |

## Format to Tool Mapping

| Format Plugin | Reference Tools |
|---------------|-----------------|
| format-osis | sword-utils, libxml2 |
| format-usfm | usfm2osis |
| format-usx | libxml2 |
| format-sword | sword-utils |
| format-zefania | libxml2 |
| format-tei | libxml2 |
| format-xml | libxml2 |
| format-dbl | libxml2 |
| format-epub | pandoc, calibre |
| format-html | pandoc |
| format-markdown | pandoc |
| format-odf | pandoc |
| format-rtf | pandoc, unrtf |
| format-txt | pandoc |
| format-sqlite | sqlite |
| format-esword | sqlite |
| format-gobible | gobible-creator |
| format-json | (built-in) |

## Behavioral Authority

**The output of these reference tools defines correct behavior.**

When our implementation disagrees with a reference tool, our code is wrong. We capture tool behavior in transcripts and use those as test oracles.

## Capsule Archives

Each tool has archived versions stored in `tool_name/capsule/`. These are created using the Juniper Bible `ingest` command:

```bash
# Archive a tool binary
./capsule ingest /path/to/tool-binary --out contrib/tool/tool_name/capsule/tool.capsule.tar.xz

# Verify integrity
./capsule verify contrib/tool/tool_name/capsule/tool.capsule.tar.xz
```

## Offline Use

To prepare for offline use:

1. Download source tarballs:
```bash
cd tool_name/nixos/
nix-prefetch-url <source-url>
```

2. Build the package:
```bash
nix-build default.nix
```

3. Copy binaries to `bin/`:
```bash
cp result/bin/* ../bin/
```

4. Create capsule archive:
```bash
./capsule ingest bin/tool-binary --out capsule/tool.capsule.tar.xz
```

## Adding New Tools

1. Create directory: `mkdir -p tool_name/{nixos,bin,src,capsule}`
2. Add README.md with documentation
3. Add LICENSE.txt with license information
4. Add nixos/default.nix for reproducible builds
5. Download source to src/ when available
6. Add prebuilt binaries to bin/ when available
7. Create capsule archive in capsule/

## Tool Requirements

For Juniper Bible, tools must be:

- Deterministic (same input → same output)
- Available on NixOS for reproducible builds
- Well-documented with known behavior
