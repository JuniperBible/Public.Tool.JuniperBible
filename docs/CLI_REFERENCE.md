# CLI Reference

This document describes the capsule CLI command structure.

## Synopsis

```
capsule <group> <command> [arguments]
```

The CLI uses a noun-first hierarchy for discoverability. Commands are organized into groups based on the primary noun they operate on.

## Command Groups

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

---

## capsule - Capsule Lifecycle Commands

### capsule ingest

Ingest a file into a new capsule

**Usage:**
```
capsule capsule ingest <path> --out <capsule.tar.xz>
```

**Example:**
```bash
capsule capsule ingest myfile.zip --out myfile.capsule.tar.xz
```

### capsule export

Export an artifact from a capsule

**Usage:**
```
capsule capsule export <capsule> --artifact <id> --out <path>
```

**Example:**
```bash
capsule capsule export my.capsule.tar.xz --artifact main --out restored.zip
```

### capsule verify

Verify capsule integrity (all hashes match)

**Usage:**
```
capsule capsule verify <capsule>
```

**Example:**
```bash
capsule capsule verify my.capsule.tar.xz
```

### capsule selfcheck

Run self-check verification plan

**Usage:**
```
capsule capsule selfcheck <capsule> [--plan <plan-id>] [--json]
```

**Example:**
```bash
capsule capsule selfcheck my.capsule.tar.xz --plan identity-bytes
```

### capsule enumerate

Enumerate contents of archive

**Usage:**
```
capsule capsule enumerate <path> [--plugin-dir <path>]
```

**Example:**
```bash
capsule capsule enumerate myfile.zip
```

### capsule convert

Convert capsule content to a different format (creates new capsule, backs up original)

**Usage:**
```
capsule capsule convert <capsule> -f <format>
```

**Example:**
```bash
capsule capsule convert my.capsule.tar.gz -f osis
```

---

## format - Format Detection and IR Commands

### format detect

Detect file format using plugins

**Usage:**
```
capsule format detect <path> [--plugin-dir <path>]
```

**Example:**
```bash
capsule format detect myfile.xml
```

### format convert

Convert file to different format via IR

**Usage:**
```
capsule format convert <path> --to <format> --out <path>
```

**Example:**
```bash
capsule format convert bible.usfm --to osis --out bible.osis
```

### format ir extract

Extract Intermediate Representation from a file

**Usage:**
```
capsule format ir extract <path> --format <format> --out <ir.json>
```

**Example:**
```bash
capsule format ir extract bible.usfm --format usfm --out bible.ir.json
```

### format ir emit

Emit native format from IR

**Usage:**
```
capsule format ir emit <ir.json> --format <format> --out <path>
```

**Example:**
```bash
capsule format ir emit bible.ir.json --format osis --out bible.osis
```

### format ir generate

Generate IR for a capsule that doesn't have one

**Usage:**
```
capsule format ir generate <capsule>
```

**Example:**
```bash
capsule format ir generate my.capsule.tar.gz
```

### format ir info

Display IR structure summary

**Usage:**
```
capsule format ir info <ir.json> [--json]
```

**Example:**
```bash
capsule format ir info bible.ir.json
```

---

## plugins - Plugin Management Commands

### plugins list

List available plugins

**Usage:**
```
capsule plugins list [--dir <path>]
```

**Example:**
```bash
capsule plugins list
```

---

## tools - Tool Execution Commands

### tools list

List available tools in contrib/tool

**Usage:**
```
capsule tools list [<contrib-dir>]
```

**Example:**
```bash
capsule tools list
```

### tools archive

Create tool archive capsule from binaries

**Usage:**
```
capsule tools archive <tool-id> --version <ver> --bin <name>=<path> --out <capsule>
```

**Example:**
```bash
capsule tools archive diatheke --version 1.8.1 --bin diatheke=/usr/bin/diatheke --out diatheke.capsule.tar.xz
```

### tools run

Run a tool plugin with Nix executor

**Usage:**
```
capsule tools run <tool> <profile> [--input <path>] [--out <dir>]
```

**Example:**
```bash
capsule tools run libsword list-modules --input ./kjv
```

### tools execute

Run tool on artifact and store transcript

**Usage:**
```
capsule tools execute <capsule> <artifact> <tool> <profile>
```

**Example:**
```bash
capsule tools execute kjv.capsule.tar.xz main libsword render-all
```

---

## runs - Run Transcript Commands

### runs list

List all runs in a capsule

**Usage:**
```
capsule runs list <capsule>
```

**Example:**
```bash
capsule runs list kjv.capsule.tar.xz
```

### runs compare

Compare transcripts between two runs

**Usage:**
```
capsule runs compare <capsule> <run-id-1> <run-id-2>
```

**Example:**
```bash
capsule runs compare kjv.capsule.tar.xz run-libsword-1 run-libsword-2
```

### runs golden save

Save golden transcript hash to file

**Usage:**
```
capsule runs golden save <capsule> <run-id> <output-file>
```

**Example:**
```bash
capsule runs golden save kjv.capsule.tar.xz run-libsword-1 goldens/kjv-list.sha256
```

### runs golden check

Check transcript against golden hash

**Usage:**
```
capsule runs golden check <capsule> <run-id> <golden-file>
```

**Example:**
```bash
capsule runs golden check kjv.capsule.tar.xz run-libsword-2 goldens/kjv-list.sha256
```

---

## juniper - Bible/SWORD Tools

### juniper list

List Bible modules in SWORD installation

**Usage:**
```
capsule juniper list [<sword-path>]
```

**Example:**
```bash
capsule juniper list
capsule juniper list ~/.sword
```

### juniper ingest

Ingest SWORD modules into capsules

**Usage:**
```
capsule juniper ingest [<modules>...] [--all] [-o <output-dir>]
```

**Example:**
```bash
capsule juniper ingest KJV ESV --all
capsule juniper ingest --all -o capsules/
```

### juniper cas-to-sword

Convert CAS capsule to SWORD module

**Usage:**
```
capsule juniper cas-to-sword <capsule> [-o <output-dir>] [-n <module-name>]
```

**Example:**
```bash
capsule juniper cas-to-sword my.capsule.tar.xz -o ~/.sword -n MYBIBLE
```

---

## dev - Development Commands

### dev test

Run tests against golden hashes

**Usage:**
```
capsule dev test <fixtures-dir> [--golden <goldens-dir>]
```

**Example:**
```bash
capsule dev test testdata/fixtures --golden testdata/goldens
```

### dev docgen

Generate documentation

**Usage:**
```
capsule dev docgen <type> [--output <dir>]
```

**Example:**
```bash
capsule dev docgen all --output docs/generated
capsule dev docgen plugins --output docs/generated
```

---

## web - Web UI Server

Start the web UI server for browsing capsules, Bibles, and performing conversions.

**Usage:**
```
capsule web [--port <port>] [--capsules <dir>] [--plugins <dir>] [--sword <dir>] [--plugins-external]
```

**Flags:**

- `--port` - HTTP server port (default: 8080)
- `--capsules` - Directory containing capsules (default: ./capsules)
- `--plugins` - Directory containing plugins (default: ./bin/plugins)
- `--sword` - Directory containing SWORD modules (default: ~/.sword)
- `--plugins-external` - Enable loading external plugins from plugins directory

**Example:**
```bash
# Start with defaults
capsule web

# Custom port and directories
capsule web --port 3000 --capsules ./my-capsules --sword ~/.sword

# Enable external plugins
capsule web --plugins-external
```

---

## api - REST API Server

Start the REST API server for programmatic access to capsule operations.

**Usage:**
```
capsule api [--port <port>] [--capsules <dir>] [--plugins <dir>] [--plugins-external]
```

**Flags:**

- `--port` - HTTP server port (default: 8081)
- `--capsules` - Directory containing capsules (default: ./capsules)
- `--plugins` - Directory containing plugins (default: ./plugins)
- `--plugins-external` - Enable loading external plugins from plugins directory

**API Endpoints:**

- `GET /health` - Health check
- `GET /capsules` - List all capsules
- `POST /capsules` - Create new capsule
- `GET /capsules/:id` - Get capsule details
- `DELETE /capsules/:id` - Delete capsule
- `POST /convert` - Convert between formats
- `GET /plugins` - List available plugins
- `GET /formats` - List supported formats

**Example:**
```bash
# Start with defaults
capsule api

# Custom port
capsule api --port 9000 --capsules ./my-capsules
```

---

## version

Print version information

**Usage:**
```
capsule version
```

**Example:**
```bash
capsule version
```
