# Scripture Format Plugin Specifications (Schemas + Implementation Notes)

**Purpose:** This document is a developer-facing spec for implementing *format plugins* that can:

- **extract-ir**: parse a native format into the project’s **Intermediate Representation (IR)**
- **emit-native**: generate a native format from IR

It focuses on **schemas**, **validation artifacts**, and **minimum implementation requirements** for each format listed in the plugin matrix.

---

## 0) Terms and conventions

### Loss classes (contractual meaning)

- **L0 – Lossless:** A round-trip `native → IR → native` can be byte-for-byte identical *or* provably equivalent under the format’s own canonicalization rules (e.g., XML attribute order differences that are explicitly non-semantic).
- **L1 – Semantic:** Meaning and structure are preserved, but presentation details may not round-trip (e.g., CSS, typographic choices, “pretty printing”).
- **L2 – Structural:** Major structure is preserved (books/chapters/verses, headings), but fine-grained markup, annotations, or metadata may be dropped or approximated.
- **L3 – Text-primary:** Primarily plain text; structure inferred heuristically or from conventions.

> Recommendation: treat **L0/L1** plugins as “validation-first”; treat **L2/L3** plugins as “best-effort with explicit loss reporting.”

### Required plugin capabilities (minimum bar)

Every plugin MUST implement the following surfaces:

- `detect(input) -> Confidence + FormatVersion?`
- `extract_ir(input, opts) -> IR`
- `emit_native(ir, opts) -> bytes/dir/archive`
- `validate_native(input) -> list[Issue]` (MUST support **“fail-fast”** and **“collect-all”** modes)
- `self_check(ir, input?) -> Evidence` (recommended; enables conversion test harness)

### Security requirements (non-negotiable)

All plugins MUST:

- **Disable XML External Entities (XXE)** and external DTD fetching by default.
- Enforce **size limits** on:
  - total bytes read
  - maximum nesting depth (XML/HTML)
  - maximum decompressed size (ZIP/TAR)
  - maximum entity expansion (XML)
- For archives: prevent **ZipSlip / path traversal**; never write outside a designated root.

---

## 1) Plugin registry (required names)

The following **format plugin names** are reserved and MUST exist in the codebase (even if some are “detect-only” for now).

| Name | Type | Description |
|---|---|---|
| accordance | format | Format plugin for accordance |
| dbl | format | Format plugin for dbl |
| dir | format | Format plugin for dir |
| epub | format | Format plugin for epub |
| esword | format | Format plugin for esword |
| file | format | Format plugin for file |
| flex | format | Format plugin for flex |
| gobible | format | Format plugin for gobible |
| html | format | Format plugin for html |
| json | format | Format plugin for json |
| logos | format | Format plugin for logos |
| markdown | format | Format plugin for markdown |
| morphgnt | format | Format plugin for morphgnt |
| odf | format | Format plugin for odf |
| onlinebible | format | Format plugin for onlinebible |
| oshb | format | Format plugin for oshb |
| osis | format | Format plugin for osis |
| pdb | format | Format plugin for pdb |
| rtf | format | Format plugin for rtf |
| sblgnt | format | Format plugin for sblgnt |
| sfm | format | Format plugin for sfm |
| sqlite | format | Format plugin for sqlite |
| sword | format | Format plugin for sword |
| sword-pure | format | Format plugin for sword-pure |
| tar | format | Format plugin for tar |
| tei | format | Format plugin for tei |
| theword | format | Format plugin for theword |
| txt | format | Format plugin for txt |
| usfm | format | Format plugin for usfm |
| usx | format | Format plugin for usx |
| xml | format | Format plugin for xml |
| zefania | format | Format plugin for zefania |
| zip | format | Format plugin for zip |

> Note: the matrix you provided previously used `format-<name>` labels; this document treats the canonical plugin IDs as the **bare names** above.

## 2) Baseline IR expectations (format-agnostic)

The IR is project-defined, but to make cross-format conversion sane, every plugin SHOULD support these common semantic fields:

### Text + reference model

- `work`: overall metadata (title, language, identifiers, copyright/licensing)
- `book`: canonical book id, human name, order/index
- `chapter`: number (and optional “published chapter label”)
- `verse`: number (supports non-integer patterns where formats allow, e.g., `3a`, `3-4`)
- `blocks`: paragraphs/poetry/section headings
- `inline`: character styling spans (bold/italic/smallcaps), divine-name styling, additions, etc.
- `notes`: footnotes + cross references with callers and targets
- `milestones`: verse boundaries, page breaks, speaker changes, etc.

### Canonical identifiers

- Prefer **OSIS identifiers** for book codes where possible.
- Maintain a `versification` field when the source format supports or implies it.

### Required normalization rules

- Text MUST be Unicode **NFC** (unless a source format mandates otherwise).
- Newlines in IR MUST be `\n`.
- Preserve significant whitespace where the source spec requires it (notably USFM).

---

## 3) Format specifications

Each section includes:

- **What it is**
- **Primary schema/spec references**
- **Validation artifacts**
- **Minimum implementation profile**
- **IR mapping notes**

---

# L0 Lossless formats

## format-osis (OSIS XML)

**What it is:** OSIS is an XML schema for scripture and related texts. It is commonly used as an interchange format and is strongly schema-driven.

**Primary references**

- OSIS official page + schema links: https://crosswire.org/osis/
- Official schema: `osisCore.2.1.1.xsd`: http://www.crosswire.org/osis/osisCore.2.1.1.xsd
- OSIS 2.1.1 user manual (PDF): https://www.crosswire.org/osis/OSIS%202.1.1%20User%20Manual%2006March2006.pdf

**Validation artifacts**

- XSD validation against `osisCore.2.1.1.xsd`.
- Many ecosystems use a “cw-latest” variant for practical validation: https://www.crosswire.org/~dmsmith/osis/osisCore.2.1.1-cw-latest.xsd (useful in CI but not the “official” schema).

**Minimum implementation profile (MUST)**

- Parse `<osis>` root with namespace and validate against XSD.
- Support `osisText` with `div` structure, plus `chapter`, `verse`, `title`, `p`, `lg/l`, `note`, and `reference`.
- Preserve OSIS identifiers:
  - `osisID` (anchors)
  - `osisRef` (cross references)
- Support `seg`, `hi`, and `w` for inline semantics when present.

**IR mapping notes**

- `book` → `div[@type="book"]` and/or `osisIDWork` mappings.
- Verse boundaries are explicit (`<verse osisID="Gen.1.1">`), but OSIS also allows milestone patterns; implement both if you want true L0 behavior.
- Footnotes are usually `<note type="footnote">…</note>`, crossrefs often `type="crossReference"`.

---

## format-usfm (USFM)

**What it is:** USFM is a marker-based plain-text markup standard for scripture. It’s structurally rich and widely used in translation workflows.

**Primary references**

- USFM documentation portal: https://ubsicap.github.io/usfm/
- USFM 3.x Syntax Notes (whitespace, parsing rules): https://ubsicap.github.io/usfm/usfm3.0/about/syntax.html

**Validation artifacts**

- There is no single XSD; validation is rule-based (marker grammar + structural constraints).
- Implement a validator that checks:
  - marker legality per declared `\usfm` version
  - marker pairing and nesting rules
  - required identification markers (`\id`, optionally `\usfm`)
  - chapter/verse progression (`\c`, `\v`)

**Minimum implementation profile (MUST)**

- Correctly tokenize markers beginning with `\`.
- Preserve **significant whitespace** per USFM syntax notes.
- Support (at minimum):
  - Book header: `\id`, `\h`, `\toc1-\toc3`, `\mt`
  - Structure: `\c`, `\v`, `\p`, `\m`, `\q1-\q4`, `\s1-\s3`, `\r`
  - Notes: `\f … \f*` footnotes, `\x … \x*` cross references
  - Inline: `\add … \add*`, `\bd … \bd*`, `\it … \it*`, `\w … \w*`
- Handle milestone-style markers (delimited by `\*`) correctly.

**IR mapping notes**

- `\c` and `\v` define the canonical ref spine.
- Paragraph markers are non-nesting; treat them as block boundaries.
- Notes become IR `notes[]` with caller and payload; parse subfields (e.g., `\ft`, `\fq`, `\fr`) when present.

---

## format-usx (USX)

**What it is:** USX is the XML representation of USFM (unified scripture XML). It is schema-validated (Relax NG).

**Primary references**

- USX schemas page (links to RNG/RNC): https://ubsicap.github.io/usx/schema.html
- Canonical schemas (GitHub):
  - RNG: https://github.com/ubsicap/usx/blob/master/schema/usx.rng
  - RNC: https://github.com/ubsicap/usx/blob/master/schema/usx.rnc

**Validation artifacts**

- Validate using Relax NG against `usx.rng` matching the document's USX version series.

**Minimum implementation profile (MUST)**

- Support `usx` root and primary structural nodes: `book`, `chapter`, `verse`, `para`, `char`, `note`, `ref`, `figure`.
- Preserve `style` attributes (they map to USFM markers and drive semantics).
- Support `loc` / ref addressing when present.

**IR mapping notes**

- USX is often a cleaner pipeline target than USFM because structure is explicit and XML-safe.
- `para[@style]` → paragraph/poetry/section blocks in IR.
- `char[@style]` and `note[@style]` map to inline spans and notes.

---

## format-zefania (Zefania XML)

**What it is:** Zefania is an XML schema for encoding biblical texts and related content, with schema documentation and community distribution of modules.

**Primary references**

- Zefania XML schema documentation: https://www.bgfdb.de/zefaniaxml/bml/
- SourceForge project (points at docs + validator tooling): https://sourceforge.net/projects/zefania-sharp/

**Validation artifacts**

- Validate using the official Zefania schema files referenced by the documentation.
- Expect real-world corpora to contain minor schema drift; provide `--strict` vs `--tolerant` validation modes.

**Minimum implementation profile (MUST)**

- Parse book/chapter/verse structures as defined in the schema docs.
- Preserve metadata blocks (title, language, identifiers) when present.
- Enforce UTF-8 and well-formed XML.

**IR mapping notes**

- Treat Zefania as “XML-first” but not always “schema-perfect”; robust parsing beats fragile strictness.
- Verse text often arrives as mixed content; preserve inline tags you understand, pass-through unknown tags with loss reporting.

---

## format-theword (theWord Bible modules)

**What it is:** theWord modules are a practical, widely used Bible software format. Specification is semi-public and community-oriented.

**Primary references**

- theWord tools & technical docs page (links to “Bible module file specification”): https://www.thewordbooks.com/index.php/download-tools-and-utilities/
- Community forum thread pointing at the spec doc: https://forum.theword.net/viewtopic.php?t=3991

**Validation artifacts**

- Specification is not consistently accessible via automated tooling; validation is typically “loadable by theWord” plus internal consistency checks.
- Implement schema validation only if you vend/specify a known spec version in-repo.

**Minimum implementation profile (MUST)**

- Implement a *best-effort* extractor with robust detection (magic bytes / container sniffing).
- Support module metadata extraction (name, language, version, encoding).
- Emitters SHOULD be gated behind a conformance test corpus (known-good modules).

**IR mapping notes**

- Treat “encrypted variants” as **unsupported** unless you have legal/technical clearance (detect and return a clear error).
- Many toolchains translate to/from intermediate text or SQLite; design for pluggable backends.

---

## format-json (project-defined JSON schema)

**What it is:** JSON is a syntax standard; your schema defines semantics.

**Primary references**

- JSON spec: RFC 8259: https://datatracker.ietf.org/doc/html/rfc8259

**Validation artifacts**

- Define and version your JSON schema:
  - Recommend **JSON Schema 2020-12** (or later) with strict `additionalProperties: false` on core objects.
- Provide canonicalization rules for L0 behavior:
  - stable key ordering in emission
  - stable float/int handling (ideally avoid floats)

**Minimum implementation profile (MUST)**

- Require UTF-8 and reject invalid Unicode.
- Emit stable, deterministic JSON (canonical sorting + stable whitespace policy).
- Provide a published schema file in the repo, e.g., `schemas/ir.schema.json`.

**IR mapping notes**

- JSON is your best “developer-friendly IR transport.” Keep it boring and rigid.

---

# L1 Semantic formats

## format-esword (e-Sword .bblx family)

**What it is:** e-Sword “.bblx” is commonly implemented as a SQLite database container, but the schema varies by module type/version.

**Primary references**

- SQLite database file format (official): https://www.sqlite.org/fileformat.html

**Validation artifacts**

- Validate the SQLite container at the file-format level (header, page size, integrity checks).
- Validate *content schema* by introspection:
  - read `sqlite_schema`
  - detect presence of expected tables/columns by heuristic signatures
  - verify verse key uniqueness and coverage

**Minimum implementation profile (MUST)**

- Open read-only and run `PRAGMA integrity_check;` (or equivalent) in validation mode.
- Implement schema detection profiles (examples):
  - “Bible text profile” (verses table with book/chapter/verse keys + text)
  - “Commentary profile” (verse keyed notes)
  - “Dictionary profile” (headword keyed entries)
- Emitters SHOULD write a documented schema version (pick one profile and commit to it).

**IR mapping notes**

- Because schema varies, treat `.bblx` as **L1**: preserve semantics; do not promise byte-level identity.
- Record the detected schema profile in IR provenance for reproducible re-emission.

---

## format-sqlite (SQLite interchange container)

**What it is:** SQLite as a container; semantics are schema-defined by your project.

**Primary references**

- SQLite file format: https://www.sqlite.org/fileformat.html

**Validation artifacts**

- File format validation + `PRAGMA integrity_check`.
- Schema validation: your project MUST publish a canonical schema (DDL) for interchange.

**Minimum implementation profile (MUST)**

- Use a single canonical schema version at a time, e.g. `schema_version` table.
- Require UTF-8 text, and specify collation behavior for reference keys.
- Provide deterministic dump/restore paths for tests.

**IR mapping notes**

- Think “relational IR transport” for very large corpora where JSON is too heavy.

---

## format-markdown (CommonMark)

**What it is:** Portable text with light structure. Great for web publishing, but semantics beyond headings/lists are mostly conventional.

**Primary references**

- CommonMark spec: https://spec.commonmark.org/

**Validation artifacts**

- Validate via CommonMark conformance tests if you claim CommonMark compliance.
- For scripture content, define and validate *conventions*:
  - headings for book/chapter
  - verse anchors (e.g., `{#Gen.1.1}`) if your system uses them

**Minimum implementation profile (MUST)**

- Parse CommonMark to an AST using a compliant parser.
- Preserve source line breaks where meaningful to your rendering pipeline.
- Emit in a normalized style (stable heading levels, stable wrapping).

**IR mapping notes**

- Markdown is usually L1 because typography and minor structure can change while meaning remains.

---

## format-html (HTML Living Standard)

**What it is:** Web-native presentation. It’s a rendering target more than a canonical interchange.

**Primary references**

- HTML Living Standard: https://html.spec.whatwg.org/

**Validation artifacts**

- Use an HTML5 parser (tag soup tolerant) and validate with an HTML validator if desired.
- For strict interchange, constrain to a known subset (e.g., “EPUB XHTML profile”).

**Minimum implementation profile (MUST)**

- Extract meaningful text while preserving structural cues (headings, paragraphs, verse anchors).
- Sanitize scripts and unsafe attributes by default.

**IR mapping notes**

- HTML is commonly `IR -> HTML` only; reverse extraction is possible but needs strong conventions.

---

## format-epub (EPUB 3.3)

**What it is:** A ZIP-packaged publication format containing (X)HTML, CSS, images, and metadata.

**Primary references**

- EPUB 3 overview: https://www.w3.org/TR/epub-overview-33/
- EPUB 3 Reading Systems 3.3: https://w3c.github.io/epub-specs/epub33/rs/

**Validation artifacts**

- Validate ZIP container + EPUB structure:
  - `mimetype` file first and stored (no compression)
  - `META-INF/container.xml`
  - OPF package document
  - NAV document
- Recommend integrating **epubcheck** in CI (external tool), but keep plugin validator self-contained where possible.

**Minimum implementation profile (MUST)**

- Read/write EPUB 3 with stable packaging rules.
- Extract spine order and map XHTML sections to IR divisions.
- Preserve metadata: title, language, identifiers, creator, rights.

**IR mapping notes**

- EPUB is L1 because layout/styling is not reliably round-trippable.

---

## format-xml (generic XML)

**What it is:** XML syntax without a fixed vocabulary.

**Primary references**

- XML 1.0 (Fifth Edition): https://www.w3.org/TR/xml/

**Validation artifacts**

- Well-formedness checks only unless a schema is supplied.
- If schema supplied: validate via XSD / RNG / Schematron as configured.

**Minimum implementation profile (MUST)**

- Secure XML parsing (no XXE).
- Optional schema-driven mode that accepts `--schema path`.

**IR mapping notes**

- Generic XML is only meaningful with a known vocabulary. Treat unknown vocabularies as L2 unless you ship explicit mappings.

---

## format-odf (OpenDocument Format, ODT/ODS/ODP)

**What it is:** OASIS OpenDocument packages (ZIP with XML parts).

**Primary references**

- ODF 1.3 standard landing page (includes links to schema parts): https://www.oasis-open.org/standard/open-document-format-for-office-applications-opendocument-version-1-3/
- ODF 1.3 schema (Relax NG): https://oasis-tcs.github.io/odf-tc/odf1.3/csd03/OpenDocument-v1.3-csd03-schema-rng.html

**Validation artifacts**

- Validate ZIP + required files (`content.xml`, `styles.xml`, `meta.xml`).
- Validate against ODF RNG if implementing strict mode.

**Minimum implementation profile (MUST)**

- Extract text flow from `content.xml` with headings/paragraphs.
- Emit ODT with stable, minimal styles (avoid excessive style explosion).

**IR mapping notes**

- ODF is mostly a presentation container; scripture structure must be encoded via conventions (styles, bookmarks, headings).

---

## format-dbl (Scripture Burrito)

**What it is:** Scripture Burrito is a “bundle” interchange for scripture content + metadata, commonly used in DBL-adjacent workflows.

**Primary references**

- Scripture Burrito docs: https://docs.burrito.bible/
- SB metadata schema repository: https://github.com/bible-technology/scripture-burrito

**Validation artifacts**

- Validate metadata files against the published SB schemas (YAML/JSON) for the target version.
- Validate that referenced content files exist and match declared checksums/hashes if provided.

**Minimum implementation profile (MUST)**

- Parse bundle manifest/metadata and locate content files.
- Preserve licensing/rights metadata as first-class IR fields.
- Emit bundles with stable file naming and deterministic hashing if used.

**IR mapping notes**

- Burrito is a strong choice for “interop packaging” when you need both scripture text and robust metadata.

---

## format-tei (TEI P5)

**What it is:** TEI is a scholarly XML framework. TEI P5 is extensive and customizable (ODD → custom schemas).

**Primary references**

- TEI P5 Relax NG schema (default “all” schema): https://www.tei-c.org/release/xml/tei/custom/schema/relaxng/tei_all.rng

**Validation artifacts**

- Validate against `tei_all.rng` or a project-specific TEI customization schema.
- TEI is large; you likely want a constrained subset and a custom ODD.

**Minimum implementation profile (MUST)**

- Support `teiHeader` extraction (bibliographic metadata, language, publication info).
- Extract `text/body` with structural cues (divs, heads, paragraphs, milestones).
- Preserve critical annotations (notes, refs) where possible.

**IR mapping notes**

- TEI’s depth can exceed your IR; define explicit down-mapping rules and record loss.

---

## format-morphgnt (MorphGNT datasets)

**What it is:** MorphGNT is a community corpus for Greek NT morphology and related projects.

**Primary references**

- MorphGNT site: https://morphgnt.org/
- MorphGNT + SBLGNT integration repo (example corpus structure + notes): https://github.com/morphgnt/sblgnt

**Validation artifacts**

- Dataset validation is typically “schema-by-convention”:
  - file naming conventions
  - token and morphology code formats
  - verse alignment consistency

**Minimum implementation profile (MUST)**

- Support reading the corpus’ canonical file layout as documented in the repo.
- Parse tokens with lemma + morphology codes where present.
- Emit a normalized IR “word layer” aligned to verse refs.

**IR mapping notes**

- Treat as a *linguistic layer* on top of base scripture text.
- Preserve provenance carefully; licensing may differ between base text and morphology annotations.

---

## format-oshb (Open Scriptures Hebrew Bible)

**What it is:** OSHB is a morphology/lemma project for the Hebrew Bible; sources are commonly distributed as OSIS XML plus additional resources.

**Primary references**

- OSHB repo (morphhb): https://github.com/openscriptures/morphhb
- OSHB site: https://hb.openscriptures.org/

**Validation artifacts**

- Validate OSIS XML (see OSIS section) plus verify morphology/lemma attributes on tokens where present.

**Minimum implementation profile (MUST)**

- Parse OSIS sources and extract word-level attributes (lemma/morph) when present.
- Emit IR with token-level annotations linked to verse refs.

**IR mapping notes**

- OSHB uses OSIS, so your OSIS plugin can be reused; this plugin is mainly a specialized “OSIS + morphology” profile.

---

## format-sblgnt (SBL Greek New Testament)

**What it is:** A critically edited Greek NT text with freely available electronic form, with licensing terms that must be respected.

**Primary references**

- SBLGNT main site: https://sblgnt.com/
- SBLGNT license/EULA page: https://www.sblgnt.com/license/

**Validation artifacts**

- Validate against the distribution format you ingest (often XML/OSIS/plain text variants).
- Validate verse coverage and canonical ref mapping.

**Minimum implementation profile (MUST)**

- Preserve verse refs and the exact Greek text (Unicode normalization must be controlled; do not “helpfully” change tonos/breathings).
- Preserve licensing metadata in IR.

**IR mapping notes**

- Often consumed as a base text with optional morphological layers (see MorphGNT).

---

## format-sfm (Standard Format Markers family)

**What it is:** A broader family of marker-based formats; USFM is the scripture-specific specialization.

**Primary references**

- USFM “about” pages describe SFM evolution: https://ubsicap.github.io/usfm/about/index.html

**Validation artifacts**

- Rule-based parser/validator similar to USFM but with configurable marker sets.

**Minimum implementation profile (MUST)**

- Implement a configurable marker grammar (marker definitions + nesting rules).
- Provide an SFM profile for “USFM-like scripture” and for generic SFM documents.

**IR mapping notes**

- Treat as “USFM generalized.” If you already implement USFM robustly, most work is making marker sets data-driven.

---

# L2 Structural formats

## format-sword (CrossWire SWORD modules)

**What it is:** The CrossWire SWORD module system (compressed data + index + metadata) used by SWORD/JSword frontends.

**Primary references**

- SWORD module development docs: https://www.crosswire.org/sword/develop/swordmodule/

**Validation artifacts**

- Validate module directory structure and `*.conf` metadata.
- Validate index/data consistency (e.g., verse offsets) using SWORD tooling or independent checks.

**Minimum implementation profile (MUST)**

- Read `*.conf` metadata and module type.
- Support at least one Bible module type (e.g., zText) in extraction.
- Emitters SHOULD target known-build tooling unless you fully reimplement module building.

**IR mapping notes**

- SWORD is L2 in many pipelines because the format is optimized for lookup/indexing and may not preserve higher-level markup consistently across module types.

---

## format-sword-pure (SWORD modules, pure implementation)

**What it is:** A **pure-language** (e.g., pure Go) implementation of SWORD module parsing/emission that does **not** depend on `libsword` / CGO bindings.

**Primary references**

- SWORD module development docs: https://www.crosswire.org/sword/develop/swordmodule/

**Validation artifacts**

- Provide a **cross-check mode** that compares results against a known-good implementation (when available):
  - Optional: `libsword`-backed plugin in CI to verify the pure implementation produces identical IR (or identical native outputs where possible).
- Required: internal index/data consistency checks (offset bounds, monotonicity, record counts, verse coverage).

**Minimum implementation profile (MUST)**

- Everything in `format-sword` extraction that you claim to support, implemented without external native deps.
- Deterministic behavior across platforms (important for reproducible builds).
- Clear feature flags for module types:
  - start with one Bible module family (e.g., zText) and grow from there.

**IR mapping notes**

- Treat `sword-pure` as the “reproducible core” parser/emitter and use external toolchains only as optional validators.


## format-rtf (Rich Text Format)

**What it is:** A keyword-based text format for formatted text interchange.

**Primary references**

- Microsoft RTF spec reference pointer (informative references): https://learn.microsoft.com/en-us/openspecs/exchange_server_protocols/ms-oxrtfcp/85c0b884-a960-4d1a-874e-53eeee527ca6

**Validation artifacts**

- Parse-level validation (balanced groups `{…}`, legal control words, encoding rules).
- Prefer robust third-party parsers; RTF is deceptively gnarly.

**Minimum implementation profile (MUST)**

- Extract plain text reliably.
- Preserve basic inline styles if feasible (bold/italic/superscript).
- Sanitize embedded objects by default (no OLE execution paths).

**IR mapping notes**

- Consider RTF a “presentation-ish” source. Without strong conventions, mapping to verses is heuristic.

---

## format-logos (Logos/Libronix .lbxlls and related)

**What it is:** Proprietary container(s) used by Logos/Libronix; public specs are sparse.

**Primary references**

- File type description (high-level): https://fileinfo.com/extension/lbxlls

**Validation artifacts**

- None officially public.
- Implement only if you have lawful access to specs or a sanctioned API/export path.

**Minimum implementation profile (MUST)**

- Detection only + a clear “unsupported/proprietary” error unless you have an approved workflow.

**IR mapping notes**

- Prefer user export formats (OSIS/USX/HTML/EPUB) over reverse engineering.

---

## format-accordance (Accordance internal modules)

**What it is:** Proprietary internal formats; public conversion workflows exist, but not a formal open schema.

**Primary references**

- Community discussion on conversion workflows: https://forums.accordancebible.com/topic/27157-how-to-convert-bibles-for-accordance/

**Validation artifacts**

- None formally public.
- Treat as “workflow-driven” conversions using Accordance-approved import/export.

**Minimum implementation profile (MUST)**

- Detection only + clear error unless operating on a documented export format.

---

## format-onlinebible (Online Bible)

**What it is:** Online Bible user module formats + import/compile tooling.

**Primary references**

- Online Bible import help: https://onlinebible.net/help/helpeng/source/html/helpimportmodule.htm

**Validation artifacts**

- Tool-driven; validate by running the import/compile tool in a sandboxed process where permitted.

**Minimum implementation profile (MUST)**

- Support extraction from the toolchain’s export outputs (often text-like with conventions).
- For strict fidelity, integrate tool-based conversion rather than guessing internal formats.

---

## format-flex (FLEx / FieldWorks interchange)

**What it is:** FLEx interchange commonly uses **LIFT** (Lexicon Interchange FormaT) and related FieldWorks XML.

**Primary references**

- SIL LIFT technical note/spec: https://software.sil.org/fieldworks/resources/technical-notes/lift/

**Validation artifacts**

- Validate LIFT XML as well-formed and against published schema/constraints when available.
- Enforce consistent IDs and cross-references between lexical entries and senses.

**Minimum implementation profile (MUST)**

- Extract lexicon entries (headword, senses, glosses, examples).
- Preserve language tags and writing system identifiers.

**IR mapping notes**

- This is usually for lexicons/dictionaries, not Bible text; map into IR “lexicon” objects, not verse streams.

---

# L3 Text-primary formats

## format-txt (verse-per-line and similar)

**What it is:** Plain text conventions, often verse-per-line or reference-prefixed lines.

**Primary references**

- CrossWire dev tools page (module conventions and tooling context): https://wiki.crosswire.org/DevTools:Modules

**Validation artifacts**

- Conventions-based validation:
  - detect line format patterns
  - verify monotonic reference progression
  - check book/chapter/verse coverage

**Minimum implementation profile (MUST)**

- Support at least:
  - `Book Chapter:Verse<TAB or space>Text`
  - `OSISRef<TAB>Text`
- Provide a configurable ref parser to accommodate variants.

**IR mapping notes**

- L3: structure is inferred; preserve original lines in provenance so users can audit losses.

---

## format-gobible (Go Bible / J2ME ecosystem)

**What it is:** Tool-driven packaging in the Go Bible ecosystem; formal open specs vary by toolchain and era.

**Primary references**

- SourceForge tool project (historic tool context): https://sourceforge.net/projects/eswordtogobible/

**Validation artifacts**

- Usually validate by attempting to load in a target reader/emulator or via toolchain verification.
- Prefer converting through a documented intermediate (OSIS/USFM/USX) when possible.

**Minimum implementation profile (MUST)**

- Treat as best-effort extraction; emit only if you have a validated toolchain pipeline.

---

## format-pdb (Palm Database container)

**What it is:** Palm Database (PDB) is a container format; payload conventions vary by application.

**Primary references**

- Background description: https://en.wikipedia.org/wiki/Palm_database

**Validation artifacts**

- Validate container header + record directory integrity.
- Payload decoding is application-specific; define profiles if you support any.

**Minimum implementation profile (MUST)**

- Container parser with safe bounds checking.
- Clear “unknown payload type” reporting.

---

# Archive/container helpers (not scripture semantics)

These are “wrappers” used for distribution. They do not define scripture semantics by themselves, but plugins should support them to ingest/export multi-file bundles.

## format-file (single file wrapper)

**Primary references**

- POSIX concepts (files): https://pubs.opengroup.org/onlinepubs/9699919799/

**Minimum implementation profile**

- Treat as raw bytes; content type is discovered by detector chain.

---

## format-zip (ZIP archives)

**Primary references**

- PKWARE APPNOTE (ZIP spec): https://support.pkware.com/pkzip/appnote

**Minimum implementation profile**

- ZipSlip-safe extraction.
- Enforce max decompressed size.
- Preserve timestamps optionally, but never require them for semantics.

---

## format-tar (tar/pax archives)

**Primary references**

- POSIX `pax` utility (tar/pax behavior): https://pubs.opengroup.org/onlinepubs/9699919799/utilities/pax.html

**Minimum implementation profile**

- Path traversal protection.
- Handle PAX headers for long paths/metadata.
- Enforce max unpacked size.

---

## format-dir (directory trees)

**Primary references**

- POSIX filesystem concepts: https://pubs.opengroup.org/onlinepubs/9699919799/

**Minimum implementation profile**

- Treat as a structured input root.
- Apply a detector chain to each file or a manifest if present.

---

## 4) Test requirements (recommended to codify in CI)

To satisfy “100% conversion path coverage” goals, implement a universal test harness that:

- Loads the canonical sample corpus for each format.
- Runs `extract_ir` and validates:
  - reference spine integrity (book/chapter/verse)
  - expected counts and coverage
  - schema or rule validation passes (in strict mode where feasible)
- Performs round-trip tests based on loss class:
  - L0: strict round-trip equality or defined canonical equivalence
  - L1: semantic equivalence (IR equality) + allowed formatting diffs
  - L2/L3: reference/text equivalence + explicit loss reports

---

## 5) Deliverables expected from each plugin repo

Each plugin SHOULD include:

- `SPEC.md` summarizing supported versions and known limitations
- `schemas/` (vendored or pinned schema links + checksums) where applicable
- `samples/` minimal fixtures + one “real world” fixture
- `tests/` conformance tests and round-trip tests
- `CHANGELOG.md` (SemVer + Keep a Changelog style)
- `LICENSE` and any third-party notice files

---