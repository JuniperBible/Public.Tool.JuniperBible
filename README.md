# Juniper Bible

**Byte-for-byte preservation. Bug-for-bug behavior. Deterministic forever.**

**Juniper Bible** (a.k.a. **JuniperBible**) is a **Bible Reader, Retrieval, Archival, Transmittal, and Conversion** system for Bible and religious text formats (SWORD, OSIS, USFM, GenBook, archives, and more).

It is **not a normal converter**.

It is intended to be a **forensic harness** that captures how reference tools *actually behave* (including quirks) and used that behavior for **automated, hash-based tests**.

> **Rule:** If our code disagrees with the reference tool, **our code is wrong**.

Juniper Bible exists to make these workflows portable, testable, and reproducible—without rewriting history or "fixing" the data.

Juniper Bible was originally created and is currently maintained by Justin Michael Weeks to power content and format workflows for focuswithjustin.com. While building that pipeline, it became clear that although many excellent tools and libraries exist, the ecosystem often falls short in one or more practical areas: portability, clear documentation, long-term maintainability, modern reproducible builds, or truly open availability. Some projects depend on aging stacks, some are hard to deploy broadly, and others are proprietary or no longer actively maintained. In a few cases, this project exists for the most honorable engineering reason of all: I wanted to scratch an itch in Go and offer another solid option.

And in that work, we are reminded of something deeper. The talents we have are not self-made; we can create nothing except from the materials the Creator has given us. They are gifts. In applying our craft, we are humbled by the drive to explore and create that seems embedded in who we are, as though it were part of what it means to be made in the image of our Creator. Humanity keeps trying to close the fracture we caused, wandering back toward love and goodness through sin, repair, and creation. Somewhere in the process, we realize how small we are and how immense the mercy is. Existence itself is a gift beyond measuring, and it is awe-inspiring that the Author of existence would still provide us the means to understand, to seek, and to approach salvation and achieve everlasting life, which we so greatly do not deserve. As part of the glorious story of deliverance and salvation, we as makers and heirs of creation get to participate in the glorious textual transmission of the word of our Lord and Saviour.  We do this through the witness of a sinful creature, saved by an unalterable and eternal word.  We do so, because it is the will of the father, that all humanity who ask shall be saved by and through his Word.

---

## Why this exists

Bible formats and conversion tools are:

- poorly specified
- historically quirky
- full of edge cases people rely on

Re-implementing them by "reading docs" fails.

**Juniper Bible** solves this by:

- Preserving original files **byte-for-byte**
- Running real reference tools (e.g., **libSWORD**) in a locked, deterministic VM
- Recording exactly what those tools do (**transcripts**)
- Using transcripts as the **test oracle**

That makes reliable conversion, auditing, archival, and reverse-engineering possible.

---

## Core guarantees

### 1) Byte sovereignty (absolute)

- All inputs are stored **verbatim**
- **SHA-256** and **BLAKE3** hashes are recorded
- Exporting "back to native" can re-emit the **exact original bytes** (where the format is lossless / identity-preserving)

No normalization. No cleanup. No guessing.

### 2) Behavioral authority

- "Correct behavior" is defined by **reference tools**
- Tools run in a pinned **NixOS VM**
- Output is captured as deterministic **transcripts**
- Transcripts are compared by **hash** in tests

Documentation never overrides observed behavior.

### 3) Determinism

Same input + same engine = same hashes.

If it's flaky, it's a bug.

---

## Two core artifacts: Capsule vs. Juniper's Sword

### What is a Capsule?

A **Capsule** is the portable, immutable archive bundle used by Juniper Bible.

It contains:

- original input bytes (content-addressed)
- derived artifacts (also content-addressed)
- a manifest describing relationships
- behavior transcripts from reference tools
- self-check reports used by CI

A capsule is portable, auditable, and immutable.

### What is *Juniper's Sword*?

**Juniper's Sword** is Juniper Bible's **archival-quality Intermediate Representation (IR)**.

It's an OSIS-complete abstract text-and-annotation graph that enables round-trip conversions:

```
Input Format → extract-ir → Juniper's Sword (IR) → emit-native → Output Format
```

- For **L0 lossless formats**, round-tripping to the same format can be **byte-identical**
- For lossy formats, the IR preserves as much structure/metadata as the loss class allows

---

## Repository structure

```
.
├── api/                    # REST API (OpenAPI spec)
├── cmd/                    # CLI entrypoints
│   ├── capsule/            # Main CLI (includes web and api subcommands)
│   └── capsule-api/        # REST API server (standalone)
├── core/
│   ├── cas/                # content-addressed storage (SHA-256, BLAKE3)
│   ├── capsule/            # manifest handling, schema validation
│   ├── runner/             # VM execution harness
│   ├── selfcheck/          # round-trip plan execution
│   ├── ir/                 # Juniper's Sword IR implementation
│   ├── sqlite/             # unified SQLite driver (pure Go + CGO option)
│   ├── xml/                # pure Go XML with XPath (replaces libxml2)
│   ├── rtf/                # pure Go RTF parser (replaces unrtf)
│   ├── epub/               # pure Go EPUB3 creation (replaces calibre)
│   ├── gobible/            # pure Go GoBible JAR (replaces GoBible Creator)
│   └── docgen/             # documentation generator
├── internal/
│   ├── fileutil/           # shared file utilities
│   ├── archive/            # shared archive utilities
│   ├── embedded/           # embedded plugin registry
│   └── formats/            # embedded format handlers
├── plugins/
│   ├── format/             # format plugins (40 total)
│   ├── tool/               # tool plugins (10 total)
│   ├── meta/               # meta plugins (aggregators)
│   └── ipc/                # shared IPC protocol package
├── tests/
│   └── integration/        # integration tests
├── nix/
│   └── flake.nix           # deterministic NixOS VM engine
├── schemas/                # JSON Schemas (manifest, transcript, selfcheck, IR)
├── testdata/
│   ├── fixtures/           # raw input files
│   └── goldens/            # expected transcript/selfcheck hashes
└── README.md
```

> **Note on naming:** the primary binary is currently `capsule` for historical reasons. The project identity is **Juniper Bible**.

---

## Key concepts

### Content-addressed storage (CAS)

- Every file is stored once, addressed by hash
- Nothing is ever overwritten
- Hashes are the truth

### Format plugins

Handle ingest only:

- detect format
- store bytes verbatim
- optionally enumerate components
- extract-ir / emit-native (when supported)

They do not "fix" content.

### Tool plugins

Run reference tools inside the VM:

- libSWORD (Bible module operations)
- Pandoc (document conversions)
- Calibre (e-book conversions)
- usfm2osis (USFM to OSIS)
- SQLite (database operations)
- libxml2 (XML validation/transformation)
- unrtf (RTF parsing)
- GoBible Creator (J2ME packaging)

### Meta plugins

Aggregate multiple plugins into unified CLIs:

- `meta.juniper`: Unified Bible module CLI delegating to format and tool plugins

### Transcripts

**JSONL event streams** describing tool behavior:

- discovered modules
- key enumeration
- rendered verse outputs
- warnings/errors

Transcripts are the behavioral spec.

### RoundTrip plans and SelfCheck reports

Declarative pipelines:

- run tool(s)
- export identity
- run tool(s) again
- compare hashes

Used for automated self-checks and CI.

---

## IR pipeline and loss classes

All bidirectional formats can convert to any other format through Juniper's Sword:

```
Source → Juniper's Sword (IR) → Target
```

Conversion quality:

- Same loss class: best fidelity (L0→L0, L1→L1)
- Higher to lower: minimal loss (L0→L3)
- Lower to higher: cannot recover lost data (L3→L0)

Loss classes:

| Class | Description |
|------:|-------------|
| L0 | Byte-identical round-trip (lossless) |
| L1 | Semantically lossless (formatting may differ) |
| L2 | Minor loss (some metadata or structure) |
| L3 | Significant loss (text preserved, markup reduced) |
| L4 | Text-only (minimal preservation) |

---

## Implemented format plugins (40 total, all bidirectional)

| Plugin | extract-ir | emit-native | Loss Class | Notes |
|---|:---:|:---:|:---:|---|
| **L0 Lossless** |||||
| format-osis | ✓ | ✓ | L0 | Gold standard XML |
| format-usfm | ✓ | ✓ | L0 | Translation standard |
| format-usx | ✓ | ✓ | L0 | Unified Scripture XML |
| format-zefania | ✓ | ✓ | L0 | German Bible format |
| format-theword | ✓ | ✓ | L0 | .ont/.nt/.twm files |
| format-json | ✓ | ✓ | L0/L1 | Clean JSON structure |
| **L1 Semantic** |||||
| format-esword | ✓ | ✓ | L1 | SQLite-based .bblx/.cmtx/.dctx |
| format-sqlite | ✓ | ✓ | L1 | Queryable database |
| format-markdown | ✓ | ✓ | L1 | Hugo-compatible |
| format-html | ✓ | ✓ | L1 | Static site |
| format-epub | ✓ | ✓ | L1 | EPUB3 with NCX |
| format-xml | ✓ | ✓ | L1 | Generic XML Bible |
| format-odf | ✓ | ✓ | L1 | Open Document Format |
| format-dbl | ✓ | ✓ | L1 | Digital Bible Library |
| format-tei | ✓ | ✓ | L1 | TEI XML scholarly |
| format-morphgnt | ✓ | ✓ | L1 | MorphGNT Greek NT |
| format-oshb | ✓ | ✓ | L1 | OpenScriptures Hebrew |
| format-sblgnt | ✓ | ✓ | L1 | SBL Greek NT |
| format-sfm | ✓ | ✓ | L1 | Paratext/SIL SFM |
| **L1 Semantic (SWORD)** |||||
| format-sword-pure | ✓ | ✓ | L1 | Pure Go with full binary round-trip (zText, zCom, zLD) |
| format-mysword | ✓ | ✓ | L1 | MySword SQLite Bible format |
| format-mybible | ✓ | ✓ | L1 | MyBible SQLite Bible format |
| format-bibletime | ✓ | ✓ | L1 | BibleTime SWORD variant |
| **L2 Structural** |||||
| format-sword | ✓ | ✓ | L2 | CGO libsword wrapper |
| format-crosswire | ✓ | ✓ | L2 | Crosswire modules |
| format-olive | ✓ | ✓ | L2 | Olive Tree format |
| format-ecm | ✓ | ✓ | L2 | ECM |
| format-na28app | ✓ | ✓ | L2 | NA28 apparatus |
| format-tischendorf | ✓ | ✓ | L2 | Tischendorf Greek NT |
| format-rtf | ✓ | ✓ | L2 | Rich Text Format |
| format-logos | ✓ | ✓ | L2 | Logos/Libronix .lbxlls |
| format-accordance | ✓ | ✓ | L2 | Accordance Mac format |
| format-onlinebible | ✓ | ✓ | L2 | OnlineBible legacy |
| format-flex | ✓ | ✓ | L2 | FLEx/Fieldworks |
| **L3 Text-primary** |||||
| format-txt | ✓ | ✓ | L3 | Plain text verse-per-line |
| format-gobible | ✓ | ✓ | L3 | GoBible J2ME .gbk |
| format-pdb | ✓ | ✓ | L3 | Palm Bible+ .pdb |
| **Archive / Container** |||||
| format-file | ✓ | ✓ | - | Single file |
| format-zip | ✓ | ✓ | - | ZIP archives |
| format-tar | ✓ | ✓ | - | TAR archives |
| format-dir | ✓ | ✓ | - | Directories |

---

## CLI structure

The CLI uses a noun-first hierarchy for discoverability:

```
capsule <group> <action> [options]
```

Command groups:

| Group | Description |
|---|---|
| capsule | Capsule lifecycle (ingest, export, verify, selfcheck, enumerate, convert) |
| format | Format detection and IR operations (detect, convert, ir extract/emit/generate/info) |
| plugins | Plugin management (list) |
| tools | Tool execution (list, run, execute) |
| runs | Run transcripts (list, compare, golden save/check) |
| juniper | Bible/SWORD workflows (list, ingest, cas-to-sword) |
| dev | Development tools (test, docgen) |
| web | Start web UI server |
| api | Start REST API server |
| version | Print version information |

---

## Typical workflows

### Ingest a file into a capsule

```bash
capsule capsule ingest KJV.zip --out kjv.capsule.tar.xz
```

### Verify capsule integrity

```bash
capsule capsule verify kjv.capsule.tar.xz
```

### Export with byte-for-byte identity

```bash
capsule capsule export kjv.capsule.tar.xz --artifact KJV --out KJV-restored.zip
diff KJV.zip KJV-restored.zip
```

### Convert between formats via IR pipeline

```bash
# Extract IR (Juniper's Sword) from USFM
capsule format ir extract bible.usfm --format usfm --out bible.ir.json

# Emit OSIS from IR
capsule format ir emit bible.ir.json --format osis --out bible.osis

# Or do it in one step
capsule format convert bible.usfm --to osis --out bible.osis
```

### Behavioral regression testing (the "forensics loop")

```bash
# 1) Ingest
capsule capsule ingest KJV.zip --out kjv.capsule.tar.xz

# 2) Run reference tool in the VM and store transcript
capsule tools execute kjv.capsule.tar.xz KJV libsword list-modules

# 3) Save transcript hash as a golden
capsule runs golden save kjv.capsule.tar.xz run-libsword-list-modules-1 goldens/kjv-list.sha256

# 4) Later: re-run and check against golden
capsule tools execute kjv.capsule.tar.xz KJV libsword list-modules
capsule runs golden check kjv.capsule.tar.xz run-libsword-list-modules-2 goldens/kjv-list.sha256
```

---

## Testing philosophy

* Tests compare **hashes**, not text
* Goldens are **transcript hashes** and **selfcheck hashes**
* If a hash changes, it must be explained

No snapshot tests. No "looks right."

---

## What Juniper Bible enables

* Reverse-engineering undocumented converters
* Safe refactors with behavior locked down
* Auditable provenance for religious texts
* Deterministic builds and reproducible research
* Confidence across thousands of modules

---

## What this is not

* ❌ A naive file converter
* ❌ A canonicalization tool
* ❌ A parser that "fixes" data
* ❌ A best-effort system

Juniper Bible is strict by design.

> **If the transcript says it behaves that way, that's how it behaves.**
> Everything else is an implementation detail.

---

## Development guide

### Build commands

```bash
# Development environment (recommended)
nix-shell

# Build (pure Go, no CGO)
make build
make plugins
make api
make all

# Build with CGO SQLite
make build-cgo
```

### Testing

Juniper Bible follows the **testing pyramid** approach:

```
        /\           E2E / Runner (few, slow, high confidence)
       /  \
      /----\         Integration (some, medium speed)
     /      \
    /--------\       Unit tests (many, fast, coverage metrics)
```

| Layer | Purpose | Command |
|-------|---------|---------|
| **Unit** | Coverage, fast feedback | `go test ./...` |
| **Integration** | Real tools, CLI commands | `go test ./tests/integration/...` |
| **E2E** | Full workflows | `make integration` |

```bash
# Unit tests
make test
go test ./...

# With coverage
make test-coverage
go test -cover ./...

# Integration tests
go test ./tests/integration/... -v

# SQLite driver consistency
make test-sqlite-divergence

# CGO variant
make test-cgo
```

See [docs/TESTING.md](docs/TESTING.md) for comprehensive testing documentation.

### Documentation

```bash
make docs
```

### SQLite driver selection

Default is pure Go (**modernc.org/sqlite**). CGO available with `-tags cgo_sqlite`.

Both drivers must produce identical results (enforced by divergence tests):

```bash
make test-sqlite-divergence
```

Use `core/sqlite.Open()` or `core/sqlite.OpenReadOnly()` — never import drivers directly.

---

## Distribution build

### Single-binary architecture

The main `capsule` binary embeds all 40 format plugins and 10 tool plugins. No external plugin directory required for standard operation.

### External plugin loading (optional)

For testing custom or premium plugins:

```bash
capsule --plugins-external <command>
# or
CAPSULE_PLUGINS_EXTERNAL=1 capsule <command>
```

### Distribution variants

Two distribution variants are available:

| Variant | SQLite Driver | Cross-compile | Performance |
|---------|---------------|---------------|-------------|
| `*-purego` | modernc.org/sqlite | All platforms | Baseline |
| `*-cgo` | mattn/go-sqlite3 | Linux/macOS only | ~10% faster |

Build distributions:

```bash
make dist-purego   # Pure Go for all platforms
make dist-cgo      # CGO for supported platforms
make dist-local    # Both variants for current platform
```

---

## Plugin IPC protocol

Plugins communicate via JSON over stdin/stdout:

```json
{"command": "detect", "args": {"path": "/path/to/file"}}
{"status": "ok", "result": {...}}
```

(or)

```json
{"status": "error", "error": "message"}
```

---

## Quick start

```bash
nix-shell
make all

# Start web UI
./capsule web --port 8080 --capsules ./capsules

# Start REST API
./capsule api --port 8081 --capsules ./capsules

# Detect a file
./capsule format detect myfile.zip

# Ingest + verify
./capsule capsule ingest myfile.zip --out myfile.capsule.tar.xz
./capsule capsule verify myfile.capsule.tar.xz
```

---

## Cloudflare Pages deployment (Hugo sites with Bible data)

JuniperBible can generate Bible data for Hugo sites deployed to Cloudflare Pages.

### Makefile setup

```makefile
# JuniperBible binary location
JUNIPER := /path/to/capsule-juniper

# Bible modules to export (must be installed in ~/.sword)
BIBLES := KJV ASV WEB DRC Vulgate

DATA_DIR := data

bibles: $(DATA_DIR)/bibles.json

$(DATA_DIR)/bibles.json:
	@mkdir -p $(DATA_DIR)
	$(JUNIPER) hugo --output $(DATA_DIR) $(BIBLES)
```

### Cloudflare Pages build command

```bash
make bibles && hugo --minify
```

Or if Bible data is pre-committed:

```bash
hugo --minify
```

### Cloudflare Pages deploy command

```bash
wrangler pages deploy public --project-name=your-project
```

### Environment variables (Cloudflare dashboard)

| Variable | Value | Description |
|----------|-------|-------------|
| `HUGO_VERSION` | `0.145.0` | Hugo version to use |
| `GO_VERSION` | `1.23` | Go version (if building JuniperBible) |

### Pre-built Bible data workflow

For faster builds, pre-generate Bible data locally and commit to the repo:

```bash
# Generate Bible data locally
make bibles

# Commit the data files
git add data/bibles.json data/bibles_auxiliary/
git commit -m "Update Bible data"
git push

# Cloudflare will now only run: hugo --minify
```

### Build output directory

Set the **Build output directory** in Cloudflare Pages settings to: `public`

---

## Sample data

The repository includes 11 complete Bible modules for testing:

| Module | Description |
|--------|-------------|
| ASV | American Standard Version |
| DRC | Douay-Rheims Catholic Bible |
| Geneva1599 | Geneva Bible 1599 |
| KJV | King James Version 1769 |
| LXX | Septuagint (Rahlfs) |
| OEB | Open English Bible |
| OSMHB | Open Scriptures Morphological Hebrew Bible |
| SBLGNT | SBL Greek New Testament |
| Tyndale | William Tyndale Bible |
| Vulgate | Latin Vulgate |
| WEB | World English Bible |

Located in `contrib/sample-data/`.

---

## Status

* 2,500+ tests passing
* Pure Go build: `CGO_ENABLED=0 go build ./...` succeeds
* CAS + schema validation complete (SHA-256 + BLAKE3)
* Capsules: XZ default + gzip optional
* Juniper's Sword IR complete (supports L0 lossless round-trips where applicable)
* 40 format plugins implemented (all bidirectional)
* 10 tool plugins implemented
* Web UI: browse, compare, search
* CI/CD: GitHub Actions + reproducible Nix VM runner
* Test coverage: 80%+ for core packages

---

## License

TBD (depends on deployment context and tool licensing constraints)

---

## FAQ

### How do I import from `~/.sword` and export to another format?

```bash
# 1) List modules
./capsule juniper list ~/.sword

# 2) Ingest module (e.g., KJV)
./capsule juniper ingest --path ~/.sword -o capsules KJV

# 3) Convert to OSIS
./capsule format convert capsules/KJV/mods.d/kjv.conf --to osis --out kjv.osis.xml
```

### How do I export SWORD modules to Hugo JSON data files?

Use `capsule-juniper hugo` to export SWORD Bible modules directly to Hugo-compatible JSON:

```bash
# Build the standalone CLI
go build -o capsule-juniper ./cmd/capsule-juniper

# Export specific modules
./capsule-juniper hugo KJV ASV WEB

# Export all Bible modules
./capsule-juniper hugo --all

# Specify output directory
./capsule-juniper hugo --output data/ KJV ASV

# Use a custom SWORD path
./capsule-juniper hugo --path /custom/sword KJV
```

Output structure:
```
data/
├── bibles.json              # Metadata index for all exported Bibles
└── bibles_auxiliary/
    ├── kjv.json             # Full KJV content (books, chapters, verses)
    ├── asv.json             # Full ASV content
    └── web.json             # Full WEB content
```

The `bibles.json` file contains metadata accessible via `.Site.Data.bibles` in Hugo templates.
Individual Bible content is in `bibles_auxiliary/[id].json` for lazy loading.

### How do I browse Bible texts in a web browser?

```bash
./capsule web --capsules contrib/sample-data/capsules --port 8080
# open http://localhost:8080
```

### How do I detect the format of an unknown Bible file?

```bash
./capsule format detect myfile.zip
```

### How do I see what plugins are available?

```bash
./capsule plugins list
```

### What's the difference between L0, L1, L2, L3, and L4 loss classes?

| Class | Description | Example Formats |
|-------|-------------|-----------------|
| **L0** | Byte-identical round-trip | OSIS, USFM, USX |
| **L1** | Semantically lossless (formatting may differ) | e-Sword, SWORD-pure, JSON |
| **L2** | Minor loss (some metadata/structure) | SWORD, RTF, Logos |
| **L3** | Significant loss (text preserved) | Plain text, GoBible |
| **L4** | Text-only (minimal preservation) | Extracted text |

### How do I contribute a new format plugin?

See `docs/PLUGIN_DEVELOPMENT.md` for the complete guide. Key steps:

1. Create `plugins/format/<name>/` directory
2. Implement IPC protocol (detect, ingest, enumerate, extract-ir, emit-native)
3. Add `plugin.json` manifest
4. Write tests with hash verification
5. Submit PR

---

## The rule to remember (again, because it's the whole point)

**Measure first. Guess never.**
**If the transcript says it behaves that way, that's how it behaves.**
