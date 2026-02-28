# Scripture Conversion Architecture

## Goal

Support a universal scripture comparison system that can:

1. Import from multiple source formats (SWORD, plain text, structured JSON)
2. Auto-discover content structure (books, chapters, verses/sections)
3. Generate metadata for cross-tradition comparison
4. Output Hugo-compatible JSON for the website

## Core Concepts

### 1. Scripture Registry (`scriptures.yaml`)

Central registry defining all known scriptures with canonical metadata:

```yaml
scriptures:
  # Unique ID used in URLs and cross-references
  kjv:
    title: "King James Version (1769)"
    abbrev: "KJV"
    tradition: christian
    sub_tradition: protestant
    language: en
    source:
      type: sword
      module: KJV
    versification: kjv  # Reference to versification system
    tags: [English, Protestant, Historic]

  drc:
    title: "Douay-Rheims Bible"
    source:
      type: sword
      module: DRC
    versification: vulgate  # Uses Catholic versification

  quran:
    title: "Al-Quran"
    tradition: islamic
    language: ar
    source:
      type: sword
      module: Quran  # or custom format
    versification: quran  # 114 surahs

  tanakh:
    title: "Tanakh"
    tradition: jewish
    source:
      type: sword
      module: WLC  # Westminster Leningrad Codex
    versification: mt  # Masoretic

  talmud_bavli:
    title: "Talmud Bavli"
    tradition: jewish
    sub_tradition: rabbinic
    source:
      type: sefaria  # Different source API
      collection: Bavli
    structure: tractate  # Not chapter/verse
```

### 2. Versification Systems (`versifications/`)

Define the book structure for each tradition:

```yaml
# versifications/kjv.yaml
name: King James Version
tradition: protestant
books:

  - id: Gen
    name: Genesis
    testament: OT
    chapters: 50

  - id: Exod
    name: Exodus
    testament: OT
    chapters: 40
  # ... 66 books total

# versifications/vulgate.yaml
name: Vulgate
tradition: catholic
extends: kjv  # Includes all KJV books, plus:
additional_books:

  - id: Tob
    name: Tobit
    testament: DC  # Deuterocanonical
    chapters: 14
    insert_after: Neh

  - id: Jdt
    name: Judith
    testament: DC
    chapters: 16
    insert_after: Tob
  # ... 7 deuterocanonical books

# versifications/ethiopian.yaml
name: Ethiopian Orthodox
extends: vulgate
additional_books:

  - id: 1En
    name: 1 Enoch
    testament: OT  # Canonical in Ethiopian
    chapters: 108

  - id: Jub
    name: Jubilees
    testament: OT
    chapters: 50
  # ... unique Ethiopian books

# versifications/quran.yaml
name: Quranic
tradition: islamic
structure: surah  # Not "book"
books:

  - id: 1
    name: Al-Fatihah
    verses: 7

  - id: 2
    name: Al-Baqarah
    verses: 286
  # ... 114 surahs
```

### 3. Book Mappings (`mappings/books.yaml`)

Map equivalent books across traditions for comparison:

```yaml
mappings:
  genesis:
    canonical_id: Gen
    names:
      hebrew: בראשית
      greek: Γένεσις
      latin: Genesis
      arabic: سفر التكوين
    traditions:
      protestant: { id: Gen, canonical: true }
      catholic: { id: Gen, canonical: true }
      orthodox: { id: Gen, canonical: true }
      ethiopian: { id: Gen, canonical: true }
      jewish: { id: Bereshit, canonical: true }

  tobit:
    canonical_id: Tob
    traditions:
      protestant: { present: false }
      catholic: { id: Tob, canonical: true }
      orthodox: { id: Tob, canonical: true }
      ethiopian: { id: Tob, canonical: true }
      jewish: { present: false }

  1_enoch:
    canonical_id: 1En
    traditions:
      protestant: { present: false }
      catholic: { present: false }
      orthodox: { present: false }  # Quoted in Jude but not canonical
      ethiopian: { id: 1En, canonical: true }
      jewish: { id: 1En, canonical: false, status: pseudepigraphal }
```

### 4. Auto-Discovery System

When extracting from SWORD or other sources:

```python
def discover_books(source):
    """Auto-discover which books a source contains."""
    # Read source metadata (e.g., SWORD .conf file)
    versification = get_versification(source)

    discovered_books = []
    for book in versification.all_possible_books:
        # Try to fetch first verse
        content = fetch(source, f"{book.id} 1:1")
        if content:
            # Discover actual chapter count
            chapters = discover_chapters(source, book)
            discovered_books.append({
                "id": book.id,
                "name": book.name,
                "chapters": chapters,
                "testament": book.testament
            })

    return discovered_books
```

### 5. Category System

From your study guide, texts fall into categories:

```yaml
categories:
  canonical:
    symbol: "🟦"
    description: "Canonical scripture"

  deuterocanonical:
    symbol: "🟪"
    description: "Deuterocanonical or Ethiopian canonical"

  pseudepigraphal:
    symbol: "🟫"
    description: "Pseudepigraphal, apocryphal, or extra-biblical"

  mystical:
    symbol: "🟨"
    description: "Mystical, rabbinic, or Kabbalistic"

  hermetic:
    symbol: "🟩"
    description: "Hermetic or Hellenistic esoteric"

  patristic:
    symbol: "🟧"
    description: "Patristic or early Church writings"
```

## Output Structure

```
data/
  scriptures.json           # All scripture metadata
  scriptures_auxiliary.json # Full content by scripture ID

  # Or per-scripture for large corpora:
  scriptures/
    kjv.json
    drc.json
    quran.json
    talmud_bavli/
      berakhot.json
      shabbat.json
      ...
```

## Extraction Pipeline

```
┌─────────────────┐
│  scriptures.yaml │  (Registry)
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌──────────────┐
│  Versification  │◄────│ Auto-discover│
│     System      │     │   (SWORD)    │
└────────┬────────┘     └──────────────┘
         │
         ▼
┌─────────────────┐
│   Extractors    │
│  ├─ SWORD       │
│  ├─ Sefaria     │
│  ├─ Plain text  │
│  └─ Custom      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Book Mappings   │  (Cross-tradition)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Hugo JSON      │
│  Output         │
└─────────────────┘
```

## Implementation Phases

### Phase 1: Versification Support ✅

- [x] Define versification YAML files for major traditions
- [x] Update extract script to use versification systems
- [x] Auto-discover books from SWORD modules

### Phase 2: Full Canon Extraction (Current)

- [ ] Run `extract_scriptures.py` to get all deuterocanonical books (Tobit, Judith, Wisdom, Sirach, Baruch, 1-2 Maccabees)
- [ ] Verify Catholic/Orthodox canon completeness in DRC and Vulgate
- [ ] Update compressed data files with full canon

### Phase 3: Multiple Source Types

- [ ] Abstract extraction into pluggable extractors
- [ ] Add Sefaria API support for Jewish texts (Talmud, Mishnah, Midrash)
- [ ] Add plain text/Markdown import for custom sources
- [ ] Support e-Sword format conversion

### Phase 4: Cross-Tradition Mapping

- [ ] Create `mappings/books.yaml` with cross-references
- [ ] Map equivalent books across traditions (e.g., Daniel additions in Catholic vs Protestant)
- [ ] Generate comparison metadata showing which traditions include each book
- [ ] Link to study guide categories (canonical, deuterocanonical, pseudepigraphal, etc.)

### Phase 5: Extended Corpora

- [ ] Add Quran support (114 surahs from SWORD modules)
- [ ] Add Talmud structure support (tractate-based organization)
- [ ] Add patristic/mystical text structure (Didache, Church Fathers, etc.)
- [ ] Support Ethiopian unique texts (1 Enoch, Jubilees, Meqabyan)

### Phase 6: UI for Comparison

- [ ] Create Hugo layout showing canonical status across traditions
- [ ] Add visual indicators (🟦 canonical, 🟪 deuterocanonical, etc.)
- [ ] Enable parallel text viewing for comparison
- [ ] Link scripture pages to study guide

## Next Steps

1. **Run full extraction** - Execute `extract_scriptures.py` with Catholic versification to extract deuterocanonical books
2. **Add SWORD sources** - Use `installmgr` to add additional module repositories and install more translations
3. **Implement book mappings** - Create cross-reference file mapping equivalent books
4. **Build comparison UI** - Show which traditions include each book with visual indicators
