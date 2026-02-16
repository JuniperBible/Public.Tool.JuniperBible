# Versification Systems

This document describes the versification systems supported by Juniper Bible and how to map between them.

## Overview

Different Bible translations use different verse numbering systems. The same content may have different verse references depending on the tradition:

| Tradition | Example | Notes |
|-----------|---------|-------|
| KJV/Protestant | Psalm 51:1 | Superscription counted as verse 1 |
| Hebrew (MT) | Psalm 51:3 | Same content, different number |
| LXX/Orthodox | Psalm 50:1 | Different psalm numbering |

Juniper Bible's IR system tracks versification explicitly to enable accurate cross-system conversion.

## Supported Versification Systems

### Primary Systems (8)

| ID | Description | Canon |
|----|-------------|-------|
| `KJV` | King James Version | Protestant 66 books |
| `LXX` | Septuagint | Orthodox canon with deuterocanon |
| `Vulgate` | Latin Vulgate | Catholic canon |
| `MT` | Masoretic Text | Hebrew Bible 39 books |
| `NRSV` | New Revised Standard | Modern ecumenical |
| `Ethiopian` | Ethiopian Orthodox | Full Ethiopian canon |
| `Catholic` | Roman Catholic | Catholic 73 books |
| `Synodal` | Russian Synodal | Russian Orthodox |

### Extended Systems (9)

| ID | Description | Notes |
|----|-------------|-------|
| `Armenian` | Armenian Orthodox | Unique ordering |
| `Georgian` | Georgian Orthodox | Additional books |
| `Slavonic` | Church Slavonic | Eastern tradition |
| `Syriac` | Peshitta | Syriac tradition |
| `Arabic` | Arabic traditions | Various systems |
| `DSS` | Dead Sea Scrolls | Qumran manuscripts |
| `Samaritan` | Samaritan Pentateuch | Unique textual tradition |
| `BHS` | Biblia Hebraica | Critical Hebrew text |
| `NA28` | Nestle-Aland 28 | Critical Greek NT |

All 17 versification systems are fully implemented with mapping support.

## Reference Format

References use the OSIS format with explicit versification:

```
Book.Chapter.Verse!Versification
```

Examples:

- `Gen.1.1!KJV` - Genesis 1:1 in KJV system
- `Ps.51.1!MT` - Psalm 51:1 in Masoretic numbering
- `Sir.1.1!Vulgate` - Sirach 1:1 in Vulgate system

## Common Mapping Differences

### Psalms

The most significant differences occur in Psalms:

| LXX | MT/KJV | Notes |
|-----|--------|-------|
| Ps 1-8 | Ps 1-8 | Identical |
| Ps 9 | Ps 9-10 | LXX combines |
| Ps 10-112 | Ps 11-113 | Off by one |
| Ps 113 | Ps 114-115 | LXX combines |
| Ps 114-115 | Ps 116 | LXX splits |
| Ps 116-145 | Ps 117-146 | Off by one |
| Ps 146-147 | Ps 147 | LXX splits |
| Ps 148-150 | Ps 148-150 | Identical |
| Ps 151 | - | LXX only |

### Verse Numbering Within Chapters

Some chapters have different verse divisions:

| Reference | KJV | MT | Notes |
|-----------|-----|-----|-------|
| Gen 31:55 | 31:55 | 32:1 | Chapter break differs |
| Exod 8:1-4 | 7:26-29 | 8:1-4 | Verse shift |
| Mal 4:1-6 | 3:19-24 | 4:1-6 | Chapter division |

### Deuterocanonical/Apocryphal Books

| Book | Protestant | Catholic | Orthodox |
|------|------------|----------|----------|
| Tobit | Apocrypha | Canon | Canon |
| Judith | Apocrypha | Canon | Canon |
| Wisdom | Apocrypha | Canon | Canon |
| Sirach | Apocrypha | Canon | Canon |
| Baruch | Apocrypha | Canon | Canon |
| 1 Maccabees | Apocrypha | Canon | Canon |
| 2 Maccabees | Apocrypha | Canon | Canon |
| 3 Maccabees | - | - | Canon |
| 4 Maccabees | - | - | Appendix |
| Prayer of Manasseh | Apocrypha | Appendix | Canon |
| 1 Esdras | Apocrypha | - | Canon |
| 2 Esdras | Apocrypha | - | - |

## Mapping API

### Basic Mapping

```go
import "github.com/FocuswithJustin/mimicry/core/ir"

// Create a reference
ref := &ir.Ref{
    Book:          "Ps",
    Chapter:       51,
    Verse:         1,
    Versification: ir.VersificationKJV,
}

// Map to MT system
mapper := ir.NewVersificationMapper()
mtRef, err := mapper.Map(ref, ir.VersificationMT)
// Result: Ps.51.3!MT
```

### Mapping Registry

```go
// Create registry with built-in mappings
registry := ir.NewMappingRegistry()
registry.LoadBuiltinMappings()

// Direct mapping
lxxRef, err := registry.MapRefBetweenSystems(ref, ir.VersificationKJV, ir.VersificationLXX)

// Chained mapping (KJV -> MT -> LXX)
lxxRef, err := registry.GetChainedMapping(ir.VersificationKJV, ir.VersificationLXX)
```

### Split and Merge Operations

Some mappings require splitting or merging verses:

```go
// MT Gen.31.55 -> KJV splits into Gen.31.55 + Gen.32.1
refs := ir.SplitRef(mtRef, mapping)

// LXX Ps.9.22 + Ps.9.23 -> KJV merges into Ps.10.1
mergedRef := ir.MergeRefs(lxxRefs, mapping)
```

## Mapping File Format

Mappings are stored as JSON files in `core/ir/data/`:

```json
{
  "id": "kjv-lxx",
  "from": "KJV",
  "to": "LXX",
  "version": "1.0.0",
  "mappings": [
    {
      "from": "Ps.9.1-Ps.9.21",
      "to": "Ps.9.1-Ps.9.21",
      "type": "identical"
    },
    {
      "from": "Ps.10.1-Ps.10.18",
      "to": "Ps.9.22-Ps.9.39",
      "type": "renumber"
    },
    {
      "from": "Ps.11.1-Ps.113.9",
      "to": "Ps.10.1-Ps.112.9",
      "type": "shift",
      "offset": -1
    }
  ]
}
```

### Mapping Types

| Type | Description | Example |
|------|-------------|---------|
| `identical` | Same numbering | Ps.1.1 -> Ps.1.1 |
| `renumber` | Different numbers, same content | Ps.10.1 -> Ps.9.22 |
| `shift` | Consistent offset | Ps.11-113 shifted by -1 |
| `split` | One verse becomes multiple | Gen.31.55 -> Gen.31.55 + Gen.32.1 |
| `merge` | Multiple verses become one | Ps.114+115 -> Ps.116 |
| `absent` | Verse not present in target | Ps.151 (LXX only) |

## Loss Tracking

Versification changes are tracked in the IR's loss budget:

| Operation | Loss Level | Notes |
|-----------|------------|-------|
| Identical mapping | L0 | No loss |
| Renumbering | L0 | Reference changes, content preserved |
| Split/Merge | L1 | Verse boundaries change |
| Absent verse | L2 | Content not mappable |

## CLI Usage

```bash
# Convert OSIS with versification mapping
./capsule convert kjv.osis --to osis --versification LXX --out kjv-as-lxx.osis

# Extract IR with explicit versification
./capsule extract-ir bible.usfm --versification MT --out bible.ir.json

# View versification info
./capsule ir-info bible.ir.json --versification
```

## Best Practices

1. **Always specify versification** - Don't assume KJV as default
2. **Preserve original** - Store original versification in metadata
3. **Track mappings** - Record which mappings were applied
4. **Handle missing verses** - Decide policy for unmappable content
5. **Validate ranges** - Ensure chapter/verse exists in target system

## Related Documentation

- [IR_IMPLEMENTATION.md](IR_IMPLEMENTATION.md) - Core IR system
- [CROSSREF.md](CROSSREF.md) - Cross-references (uses versification)
- [PARALLEL.md](PARALLEL.md) - Parallel corpora (aligns across versifications)
