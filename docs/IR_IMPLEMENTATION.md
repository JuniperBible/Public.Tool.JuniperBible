# IR-Based Bible Format Converter Implementation

## Overview

This document describes the Intermediate Representation (IR) system for lossless Bible format conversion built on top of Juniper Bible's content-addressed storage foundation.

**Core Guarantee**: All conversions go to and from IR and back to native formats with no loss of data. This is the basis for all unit tests and integration tests (TDD).

## Design Principles

### Stand-off Markup

The IR uses **stand-off markup** to handle overlapping structures that are common in Bible texts:

- Verses can span across poetry lines
- Quotations can cross chapter boundaries
- Red-letter text overlaps with verse boundaries
- Footnotes attach to arbitrary text ranges

Instead of inline markup (which forces a tree structure), we use:

- **Anchors**: Position markers within content blocks
- **Spans**: Regions defined by start/end anchors (can overlap freely)
- **Annotations**: Metadata attached to spans

### Loss Classification

Every format conversion is classified by fidelity:

| Class | Name | Description |
|-------|------|-------------|
| L0 | Lossless | Byte-for-byte round-trip possible |
| L1 | Semantically Lossless | All content preserved, formatting may differ |
| L2 | Minor Loss | Some formatting lost (e.g., custom fonts) |
| L3 | Significant Loss | Annotations lost (e.g., Strong's numbers) |
| L4 | Plain Text | Only raw text preserved |

### Content Addressing

All IR content is hashed using SHA-256 for:

- Deduplication in content-addressed storage
- Change detection across conversions
- Verification of round-trip fidelity

## Core Types

### Corpus

Top-level container for a complete Bible module:

```go
type Corpus struct {
    ID              string           // Unique identifier
    Version         string           // IR schema version (e.g., "1.0.0")
    ModuleType      ModuleType       // BIBLE, COMMENTARY, DICTIONARY, etc.
    Versification   string           // One of 17 systems: "KJV", "Catholic", "LXX", "Ethiopian", "MT", etc.
    Language        string           // BCP-47 tag (e.g., "en", "he", "grc")
    Title           string           // Human-readable title
    Documents       []*Document      // Books, articles, or entries
    MappingTables   []*MappingTable  // Versification mappings
    SourceHash      string           // SHA-256 of source artifact
    LossClass       LossClass        // Fidelity of extraction
}
```

### Document

A single book, article, or dictionary entry:

```go
type Document struct {
    ID              string           // e.g., "Gen", "Matt"
    CanonicalRef    *Ref             // Primary reference
    Title           string           // e.g., "Genesis", "Matthew"
    Order           int              // Position in corpus
    ContentBlocks   []*ContentBlock  // Text content
    Annotations     []*Annotation    // Stand-off annotations
}
```

### ContentBlock

Contiguous text unit (typically a paragraph or section):

```go
type ContentBlock struct {
    ID       string     // Unique within document
    Sequence int        // Order within document
    Text     string     // Raw UTF-8 text
    Tokens   []*Token   // Word-level breakdown
    Anchors  []*Anchor  // Position markers
    Hash     string     // SHA-256 of Text
}
```

### Token

Word or whitespace unit for linguistic annotation:

```go
type Token struct {
    ID         string   // Unique within content block
    Index      int      // Position in token sequence
    CharStart  int      // UTF-8 byte offset start
    CharEnd    int      // UTF-8 byte offset end
    Text       string   // Token text
    Type       string   // "word", "whitespace", "punctuation"
    Lemma      string   // Dictionary form (optional)
    Strongs    []string // Strong's numbers (e.g., ["H1234"])
    Morphology string   // Morphological code (optional)
}
```

### Anchor

Position marker for stand-off markup:

```go
type Anchor struct {
    ID             string  // Unique within content block
    ContentBlockID string  // Parent content block
    CharOffset     int     // UTF-8 byte offset
    TokenIndex     int     // Token position (optional)
    Hash           string  // Hash of ContentBlock at this anchor
}
```

### Span

Region between two anchors (overlapping allowed):

```go
type Span struct {
    ID            string                 // Unique within document
    Type          SpanType               // VERSE, CHAPTER, POETRY_LINE, etc.
    StartAnchorID string                 // Opening anchor
    EndAnchorID   string                 // Closing anchor
    Ref           *Ref                   // Scripture reference (optional)
    Attributes    map[string]interface{} // Type-specific metadata
}
```

### SpanType Values

```go
const (
    SpanVerse      SpanType = "VERSE"
    SpanChapter    SpanType = "CHAPTER"
    SpanParagraph  SpanType = "PARAGRAPH"
    SpanPoetryLine SpanType = "POETRY_LINE"
    SpanQuotation  SpanType = "QUOTATION"
    SpanRedLetter  SpanType = "RED_LETTER"
    SpanNote       SpanType = "NOTE"
    SpanCrossRef   SpanType = "CROSS_REF"
    SpanSection    SpanType = "SECTION"
    SpanTitle      SpanType = "TITLE"
    SpanDivine     SpanType = "DIVINE_NAME"
    SpanEmphasis   SpanType = "EMPHASIS"
    SpanForeign    SpanType = "FOREIGN"
    SpanSelah      SpanType = "SELAH"
)
```

### Annotation

Data attached to a span:

```go
type Annotation struct {
    ID         string         // Unique within document
    SpanID     string         // Parent span
    Type       AnnotationType // STRONGS, MORPHOLOGY, FOOTNOTE, etc.
    Value      interface{}    // Type-specific data
    Confidence float64        // 0.0-1.0 (optional)
    Source     string         // Attribution (optional)
}
```

### AnnotationType Values

```go
const (
    AnnotationStrongs    AnnotationType = "STRONGS"
    AnnotationMorphology AnnotationType = "MORPHOLOGY"
    AnnotationFootnote   AnnotationType = "FOOTNOTE"
    AnnotationCrossRef   AnnotationType = "CROSS_REF"
    AnnotationGloss      AnnotationType = "GLOSS"
    AnnotationSource     AnnotationType = "SOURCE"
    AnnotationAlternate  AnnotationType = "ALTERNATE"
    AnnotationVariant    AnnotationType = "VARIANT"
)
```

### Ref (Scripture Reference)

Canonical scripture reference:

```go
type Ref struct {
    Book     string  // OSIS book ID: "Gen", "Matt", "1John"
    Chapter  int     // 1-indexed
    Verse    int     // 1-indexed
    VerseEnd int     // For ranges (optional)
    SubVerse string  // "a", "b" for verse subdivisions
    OSISID   string  // Full OSIS ID: "Gen.1.1", "Matt.5.3-12"
}
```

### MappingTable

Versification mappings between systems (supports all 17 versification systems):

```go
type MappingTable struct {
    ID         string           // Unique identifier
    FromSystem VersificationID  // Source: one of 17 systems
    ToSystem   VersificationID  // Target: one of 17 systems
    Mappings   []*RefMapping    // Individual mappings
    Hash       string           // For change detection
}

type RefMapping struct {
    From *Ref
    To   *Ref
    Type MappingType // "exact", "split", "merge", "missing"
}
```

**Supported versification systems** (17 total): KJV, LXX, Vulgate, MT, NRSV, Ethiopian, Catholic, Synodal, Armenian, Georgian, Slavonic, Syriac, Arabic, DSS, Samaritan, BHS, NA28. See [VERSIFICATION.md](VERSIFICATION.md) for complete details.

### LossReport

Documents conversion fidelity:

```go
type LossReport struct {
    SourceFormat string        // e.g., "SWORD"
    TargetFormat string        // e.g., "IR"
    LossClass    LossClass     // L0-L4
    LostElements []LostElement // What was lost
    Warnings     []string      // Non-fatal issues
}

type LostElement struct {
    Path        string // Location in source
    ElementType string // What was lost
    Reason      string // Why it was lost
}
```

## Format Support

The project includes **43 format plugins** supporting various Bible formats. Key formats include:

### Priority 1 (Core)

| Format | Extract IR | Emit Native | Loss Level |
|--------|------------|-------------|------------|
| OSIS | Yes | Yes | L0 (gold standard) |
| USFM | Yes | Yes | L0/L1 |
| SWORD (pure-go) | Yes | **Yes** | L1 (full binary round-trip) |
| SWORD (CGO) | Yes | Yes | L2 |

### Priority 2

| Format | Extract IR | Emit Native | Loss Level |
|--------|------------|-------------|------------|
| USX | Yes | Yes | L0/L1 |
| Zefania | Yes | Yes | L1/L2 |
| e-Sword | Yes | Yes | L2 |

### Priority 3+

| Format | Extract IR | Emit Native | Loss Level |
|--------|------------|-------------|------------|
| JSON | Yes | Yes | L1/L2 |
| SQLite | Yes | Yes | L2 |
| Markdown | Yes | Yes | L3/L4 |
| EPUB | Yes | Yes | L2/L3 |

Additional supported formats include: MySword, MyBible, OnlineBible, TheWord, GoBible, Accordance, Logos, MorphGNT, SBLGNT, NA28App, OSHB, Tischendorf, ECM, DBL, TEI, SFM, RTF, ODF, and more. See `plugins/format/` directory for complete list.

## Plugin Commands

### extract-ir

Converts native format to IR:

```json
// Request
{
  "command": "extract-ir",
  "args": {
    "path": "/path/to/source",
    "output_dir": "/path/to/output",
    "options": {
      "versification": "KJV"
    }
  }
}

// Response
{
  "status": "ok",
  "result": {
    "ir_blob_sha256": "abc123...",
    "loss_report": {
      "source_format": "SWORD",
      "target_format": "IR",
      "loss_class": "L1",
      "lost_elements": [],
      "warnings": []
    }
  }
}
```

### emit-native

Converts IR to native format:

```json
// Request
{
  "command": "emit-native",
  "args": {
    "ir_path": "/path/to/ir.json",
    "output_dir": "/path/to/output",
    "target_format": "osis"
  }
}

// Response
{
  "status": "ok",
  "result": {
    "output_blob_sha256": "def456...",
    "loss_report": {
      "source_format": "IR",
      "target_format": "OSIS",
      "loss_class": "L0",
      "lost_elements": [],
      "warnings": []
    }
  }
}
```

## Self-Check Integration

### New Check Types

- **IR_STRUCTURE_EQUAL**: Semantic comparison of two IR artifacts
- **IR_ROUNDTRIP**: Verify native → IR → native preserves content
- **IR_FIDELITY**: Verify loss class is within budget

### New Step Types

- **EXTRACT_IR**: Run extract-ir plugin command
- **EMIT_NATIVE**: Run emit-native plugin command
- **COMPARE_IR**: Compare two IR structures semantically

## CLI Commands

```bash
# Extract IR from artifact
capsule extract-ir <capsule> --artifact <id> --out <ir.json>

# Emit native from IR
capsule emit-native <capsule> --ir <id> --format <format> --out <file>

# Convert between formats via IR
capsule convert <input> --to <format> --out <output>

# Show IR structure
capsule ir-info <capsule> --artifact <id>
```

## Implementation Phases

1. **Phase 7**: IR Schema (core/ir/) - Foundation types
2. **Phase 8**: Capsule IR Integration - Store/Load IR
3. **Phase 9**: Format Plugins - OSIS (L0), USFM, SWORD
4. **Phase 10**: DERIVED Export - IR-based conversion pipeline
5. **Phase 11**: Self-Check Extensions - IR verification
6. **Phase 12**: CLI Commands - User interface
7. **Phase 13**: Test Infrastructure - Fixtures and goldens

## Success Metrics

- All OSIS round-trips are L0 (byte-identical)
- All USFM/USX round-trips are L0 or L1
- SWORD → IR → SWORD produces identical libsword transcripts
- `capsule selfcheck --plan ir-roundtrip-*` passes for all formats
- CI golden hash tests catch any IR regression

## References

- OSIS XML Schema: https://crosswire.org/osis/
- USFM Documentation: https://ubsicap.github.io/usfm/
- SWORD Library: https://crosswire.org/sword/
- BCP-47 Language Tags: https://www.rfc-editor.org/rfc/rfc5646
