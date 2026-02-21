# Cross-Reference System

This document describes Juniper Bible's cross-reference extraction, storage, and emission system.

## Overview

Cross-references link related passages across the Bible. Juniper Bible's IR system preserves and processes cross-references during format conversion, enabling:

- Extraction from source formats (OSIS, USFM, etc.)
- Storage in the IR with full metadata
- Emission to target formats
- Cross-reference indexing and lookup

## Cross-Reference Types

| Type | Description | Example |
|------|-------------|---------|
| `quotation` | Direct quote | Matt 4:4 quotes Deut 8:3 |
| `allusion` | Indirect reference | Heb 11:17-19 alludes to Gen 22 |
| `parallel` | Synoptic parallel | Matt 3:1-12 // Mark 1:1-8 // Luke 3:1-18 |
| `prophecy` | OT→NT fulfillment | Isa 7:14 fulfilled in Matt 1:23 |
| `typology` | Type/antitype | Jonah 1:17 typifies Matt 12:40 |
| `general` | General relation | Thematic or topical link |

## IR Data Structures

### CrossReference Type

```go
type CrossReference struct {
    ID         string       `json:"id"`
    SourceRef  *Ref         `json:"source_ref"`
    TargetRef  *Ref         `json:"target_ref"`
    Type       CrossRefType `json:"type,omitempty"`
    Label      string       `json:"label,omitempty"`
    Notes      string       `json:"notes,omitempty"`
    Confidence float64      `json:"confidence,omitempty"`
    Source     string       `json:"source,omitempty"`
}
```

### CrossRefIndex Type

```go
type CrossRefIndex struct {
    BySource map[string][]*CrossReference  // Key: OSIS ref
    ByTarget map[string][]*CrossReference  // Key: OSIS ref
    All      []*CrossReference
}
```

## Format Support

### OSIS

OSIS uses `<reference>` elements with `osisRef` attributes:

```xml
<verse osisID="Matt.4.4">
  But he answered and said, It is written,
  <reference osisRef="Deut.8.3">Man shall not live by bread alone</reference>
</verse>
```

Extraction:
```go
func ExtractCrossRefsFromOSIS(xml string) ([]*CrossReference, error)
```

### USFM

USFM uses `\x` markers for cross-references:

```
\v 4 But he answered and said, It is written, \x + \xo 4.4: \xt Deut 8.3\x* Man shall not live by bread alone
```

Markers:
| Marker | Purpose |
|--------|---------|
| `\x` | Cross-reference start |
| `\xo` | Origin reference |
| `\xt` | Target reference text |
| `\xk` | Keyword |
| `\xq` | Quotation |
| `\x*` | Cross-reference end |

### TheWord

TheWord uses inline tags:

```
<RX Q0.0.0>Deut 8:3<Rx>
```

### e-Sword

e-Sword stores cross-references in a separate SQLite table:

```sql
CREATE TABLE cross_references (
    book INTEGER,
    chapter INTEGER,
    verse INTEGER,
    target_book INTEGER,
    target_chapter INTEGER,
    target_verse_start INTEGER,
    target_verse_end INTEGER
);
```

## Standard Cross-Reference Sets

### Treasury of Scripture Knowledge (TSK)

The classic cross-reference resource with ~700,000 references:

| Statistic | Value |
|-----------|-------|
| Total references | ~700,000 |
| Verses covered | 31,102 |
| Avg refs per verse | ~22 |
| Types | General, parallel |

### NASB Cross-References

Cross-references from the NASB translation:

| Statistic | Value |
|-----------|-------|
| Total references | ~50,000 |
| Types | Quotation, allusion, parallel |
| Confidence | High (editorial review) |

### ESV Cross-References

Cross-references from the ESV Study Bible:

| Statistic | Value |
|-----------|-------|
| Total references | ~65,000 |
| Types | All types |
| Additional data | Notes, study content |

## API Usage

### Extracting Cross-References

```go
import "github.com/JuniperBible/juniper/core/ir"

// From OSIS
corpus, err := extractIR(osisContent)
xrefs := corpus.CrossReferences

// Build index for lookup
corpus.BuildCrossRefIndex()

// Find references FROM a verse
ref := &ir.Ref{Book: "Matt", Chapter: 4, Verse: 4}
outgoing := corpus.CrossRefIndex.GetBySource(ref)

// Find references TO a verse
ref := &ir.Ref{Book: "Deut", Chapter: 8, Verse: 3}
incoming := corpus.CrossRefIndex.GetByTarget(ref)
```

### Adding Cross-References

```go
xref := &ir.CrossReference{
    ID:         "xref-001",
    SourceRef:  &ir.Ref{Book: "Matt", Chapter: 4, Verse: 4},
    TargetRef:  &ir.Ref{Book: "Deut", Chapter: 8, Verse: 3},
    Type:       ir.CrossRefQuotation,
    Confidence: 1.0,
    Source:     "TSK",
}
corpus.AddCrossReference(xref)
```

### Parsing Reference Strings

```go
// Parse various formats
xrefs, err := ir.ParseCrossRefString("Gen 1:1")
xrefs, err := ir.ParseCrossRefString("Gen 1:1-3")
xrefs, err := ir.ParseCrossRefString("Gen 1:1; Exod 2:3")
xrefs, err := ir.ParseCrossRefString("cf. Matt 5:3-12")
```

## CLI Usage

```bash
# Extract IR with cross-references
./capsule extract-ir bible.osis --out bible.ir.json

# View cross-reference statistics
./capsule ir-info bible.ir.json --crossrefs

# Convert preserving cross-references
./capsule convert bible.osis --to usfm --out bible.usfm

# Export cross-references only
./capsule ir-info bible.ir.json --crossrefs --format json > xrefs.json
```

## Cross-Reference in Conversions

### L0 Formats (Lossless)

OSIS, USFM, USX preserve cross-references fully:

```
OSIS → IR → USFM  (L0: all cross-refs preserved)
USFM → IR → OSIS  (L0: all cross-refs preserved)
```

### L1 Formats (Semantic)

Markdown, HTML, EPUB include cross-references as links:

```markdown
<!-- Markdown output -->
But he answered and said, It is written, [Man shall not live by bread alone](deut-8.html#v3)
```

### L2/L3 Formats

Some formats lose cross-reference metadata:

| Format | Cross-ref Handling |
|--------|-------------------|
| SWORD | Stored in separate module |
| RTF | Footnotes only |
| TXT | Not preserved |

## Confidence Scoring

Cross-references include confidence scores:

| Score | Meaning | Example |
|-------|---------|---------|
| 1.0 | Certain | Direct quotation with attribution |
| 0.8-0.9 | High | Clear allusion |
| 0.5-0.7 | Medium | Thematic parallel |
| 0.3-0.4 | Low | Possible connection |
| < 0.3 | Speculative | Suggested by commentators |

## Bidirectionality

Cross-references can be unidirectional or bidirectional:

```go
type CrossReference struct {
    // ...
    Bidirectional bool `json:"bidirectional,omitempty"`
}
```

For parallel passages (synoptic gospels), bidirectionality is typically true:

```
Matt 3:1-12 ↔ Mark 1:1-8 ↔ Luke 3:1-18
```

For quotations, unidirectional:

```
Deut 8:3 → Matt 4:4  (OT quoted in NT)
```

## Integration with Versification

Cross-references automatically adjust for versification:

```go
// Original reference in KJV system
xref.SourceRef = &ir.Ref{Book: "Ps", Chapter: 51, Verse: 1, Versification: ir.VersificationKJV}

// Convert to MT versification
mtXref := mapper.MapCrossReference(xref, ir.VersificationMT)
// Result: Ps.51.3!MT
```

## Schema Definition

Cross-references are defined in `schemas/ir.schema.json`:

```json
{
  "CrossReference": {
    "type": "object",
    "properties": {
      "id": {"type": "string"},
      "source_ref": {"$ref": "#/definitions/Ref"},
      "target_ref": {"$ref": "#/definitions/Ref"},
      "type": {
        "type": "string",
        "enum": ["quotation", "allusion", "parallel", "prophecy", "typology", "general"]
      },
      "label": {"type": "string"},
      "notes": {"type": "string"},
      "confidence": {"type": "number", "minimum": 0, "maximum": 1},
      "source": {"type": "string"},
      "bidirectional": {"type": "boolean"}
    },
    "required": ["id", "source_ref", "target_ref"]
  }
}
```

## Related Documentation

- [IR_IMPLEMENTATION.md](IR_IMPLEMENTATION.md) - Core IR system
- [VERSIFICATION.md](VERSIFICATION.md) - Versification for reference mapping
- [PARALLEL.md](PARALLEL.md) - Parallel corpus alignment
