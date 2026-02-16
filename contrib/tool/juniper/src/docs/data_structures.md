# Data Structures Documentation

This document describes the binary file formats and data structures used by the scripture converter tools.

## SWORD Module Format

SWORD is the standard Bible software format developed by the CrossWire Bible Society. The format supports multiple compression and organization schemes.

### References

- [PySword Module Format Documentation](https://tgc-dk.gitlab.io/pysword/module-format.html)
- [PySword GitHub Repository](https://github.com/tgc-dk/pysword)
- [SWORD Project](https://www.crosswire.org/sword/)

### Module Configuration (.conf)

Each SWORD module has a `.conf` file in `mods.d/` with metadata:

```ini
[ModuleID]
Description=Human readable title
DataPath=./modules/texts/ztext/modid/
ModDrv=zText          # Module driver: zText, zText4, RawText, zCom, zLD, etc.
SourceType=OSIS       # Markup: OSIS, ThML, GBF, TEI, Plain
Encoding=UTF-8
CompressType=ZIP      # ZIP (zlib), LZSS, or none
BlockType=BOOK        # BOOK, CHAPTER, or VERSE (for zText)
Versification=KJV     # Versification system: KJV, KJVA, Synodal, etc.
Lang=en               # ISO language code
```

### zText Format

The zText format stores compressed Bible text in three files per testament (OT/NT):

#### Files

| File | Description |
|------|-------------|
| `ot.bzs` / `nt.bzs` | Block/Buffer index - maps buffer numbers to compressed data locations |
| `ot.bzv` / `nt.bzv` | Verse index - maps verse indices to buffer positions |
| `ot.bzz` / `nt.bzz` | Compressed data - zlib-compressed verse text |

#### BZS File Structure (Block Index)

12-byte records, little-endian:

| Offset | Size | Type | Description |
|--------|------|------|-------------|
| 0 | 4 | uint32 | Offset in `.bzz` file where compressed block starts |
| 4 | 4 | uint32 | Compressed size of block in bytes |
| 8 | 4 | uint32 | Uncompressed size (informational) |

Record location: `byte_offset = 12 * block_number`

#### BZV File Structure (Verse Index)

10-byte records (zText) or 12-byte records (zText4), little-endian:

**zText (10 bytes per record):**

| Offset | Size | Type | Description |
|--------|------|------|-------------|
| 0 | 4 | uint32 | Block number (index into `.bzs`) |
| 4 | 4 | uint32 | Offset within uncompressed block where verse text starts |
| 8 | 2 | uint16 | Verse length in bytes |

**zText4 (12 bytes per record):**

| Offset | Size | Type | Description |
|--------|------|------|-------------|
| 0 | 4 | uint32 | Block number |
| 4 | 4 | uint32 | Offset within uncompressed block |
| 8 | 4 | uint32 | Verse length (expanded from uint16) |

Record location: `byte_offset = record_size * verse_index`

#### Verse Index Calculation

**IMPORTANT DISCOVERY (2025-12-30):** The SWORD verse index (.bzv) contains more entries
than just verses. Based on analysis of the KJV module, each testament includes:

| Entry Index | Content Type | Description |
|-------------|--------------|-------------|
| 0 | Placeholder | Empty entry (Len=0) |
| 1 | Module Header | osis2mod milestone marker (83 bytes) |
| 2 | Book Intro | Book title and intro divs (e.g., "THE FIRST BOOK OF MOSES CALLED GENESIS") |
| 3 | Chapter Intro | Chapter title (e.g., "CHAPTER 1.") |
| 4+ | Verses | Actual verse content (Genesis 1:1 starts here) |

**Observed Structure (KJV OT):**
```
Entry 0: Block=0, Start=0, Len=0     # Empty placeholder
Entry 1: Block=0, Start=0, Len=83    # Module milestone
Entry 2: Block=1, Start=0, Len=195   # Genesis book intro
Entry 3: Block=1, Start=195, Len=105 # Genesis chapter 1 intro
Entry 4: Block=1, Start=300, Len=265 # Genesis 1:1 ("In the beginning...")
Entry 5: Block=1, Start=565, Len=609 # Genesis 1:2
...
```

**Implication:** The simple cumulative verse count calculation does NOT work for
real SWORD modules. The index must account for:

- 1 placeholder entry per testament
- 1 module header entry per testament
- 1 intro entry per book
- 1 intro entry per chapter

**Current Workaround:** Use diatheke CLI for extraction (production-proven approach).
The Go parser requires additional work to handle these intro entries correctly.

**Theoretical Algorithm (not yet validated):**

```go
func calculateVerseIndexWithIntros(book, chapter, verse int) int {
    index := 2  // Skip placeholder and module header

    // Add previous books with their intros
    for i := 0; i < book; i++ {
        index += 1  // Book intro entry
        for ch := 0; ch < versification[i].ChapterCount; ch++ {
            index += 1  // Chapter intro entry
            index += versification[i].ChapterVerseCounts[ch]
        }
    }

    // Add current book's intro
    index += 1

    // Add previous chapters in current book with their intros
    for ch := 0; ch < chapter - 1; ch++ {
        index += 1  // Chapter intro entry
        index += versification[book].ChapterVerseCounts[ch]
    }

    // Add current chapter's intro
    index += 1

    // Add verse position
    index += verse

    return index
}
```

**Naive Algorithm (INCORRECT for real SWORD modules):**

This simpler formula assumes no intro entries and is only valid for synthetic test data:

```
verse_index = sum(verses_in_books_before)

            + sum(verses_in_chapters_before)
            + verse_number
```

```go
func calculateVerseIndexSimple(book, chapter, verse int, versification []BookInfo) int {
    index := 0

    // Add verses from all previous books
    for i := 0; i < book; i++ {
        for _, chapterVerses := range versification[i].ChapterVerseCounts {
            index += chapterVerses
        }
    }

    // Add verses from previous chapters in current book
    for ch := 0; ch < chapter - 1; ch++ {
        index += versification[book].ChapterVerseCounts[ch]
    }

    // Add verse position (0-indexed internally, 1-indexed user-facing)
    index += verse

    return index
}
```

**Example (with intro entries): Genesis 1:1 in KJV**

- Placeholder entry: 1
- Module header: 1
- Genesis book intro: 1
- Genesis chapter 1 intro: 1
- Verse: 1
- Index = 1 + 1 + 1 + 1 + 1 = 5... but observed index is 4!

**Note:** The exact formula needs validation against multiple SWORD modules.
Some modules may have different intro structures.

#### Verse Lookup Process

1. **Calculate verse index** using book, chapter, verse with versification data
2. **Read BZV entry** at `verse_index * 10` (or 12 for zText4)
3. **Read BZS entry** at `block_number * 12` to get block location
4. **Read compressed data** from BZZ at the specified offset and size
5. **Decompress** using zlib
6. **Extract verse text** from decompressed block using verse offset and length

```go
func getVerse(book, chapter, verse int) (string, error) {
    // Step 1: Calculate verse index
    idx := calculateVerseIndex(book, chapter, verse, versification)

    // Step 2: Read BZV entry
    bzvOffset := idx * 10
    blockNum := readUint32(bzv, bzvOffset)
    verseStart := readUint32(bzv, bzvOffset + 4)
    verseLen := readUint16(bzv, bzvOffset + 8)

    // Step 3: Read BZS entry
    bzsOffset := blockNum * 12
    compOffset := readUint32(bzs, bzsOffset)
    compSize := readUint32(bzs, bzsOffset + 4)

    // Step 4: Read compressed block
    compressed := bzz[compOffset : compOffset + compSize]

    // Step 5: Decompress
    decompressed := zlibDecompress(compressed)

    // Step 6: Extract verse
    return string(decompressed[verseStart : verseStart + verseLen])
}
```

### BlockType Variants

The `BlockType` configuration affects how verses are grouped into compressed blocks:

| BlockType | Description |
|-----------|-------------|
| BOOK | Entire book compressed together (most common) |
| CHAPTER | Each chapter compressed separately |
| VERSE | Each verse compressed separately (rare) |

### Versification Systems

SWORD supports multiple Bible versification systems:

| System | Description | Books |
|--------|-------------|-------|
| KJV | King James Version Protestant canon | 66 |
| KJVA | KJV with Apocrypha | 80+ |
| Synodal | Russian Orthodox | 77 |
| Leningrad | Hebrew Bible (Tanakh) | 39 |
| MT | Masoretic Text | 39 |
| NRSV | NRSV ordering | 66 |
| Catholic | Roman Catholic | 73 |
| Catholic2 | Alternative Catholic | 73 |
| LXX | Septuagint | 76+ |
| Orthodox | Orthodox canon | 76+ |
| Luther | Luther German ordering | 66 |
| Vulg | Vulgate | 73 |

Each versification defines:

- Book order and IDs
- Number of chapters per book
- Number of verses per chapter

### KJV Versification Verse Counts

For reference, the KJV versification chapter verse counts:

```
Genesis: 31 25 24 26 32 22 24 22 29 32 32 20 18 24 21 16 27 33 38 18 34 24 20 67 34 35 46 22 35 43 55 32 20 31 29 43 36 30 23 23 57 38 34 34 28 34 31 22 33 26
Exodus: 22 25 22 31 23 30 25 32 35 29 10 51 22 31 27 36 16 27 25 26 36 31 33 18 40 37 21 43 46 38 18 35 23 35 35 38 29 31 43 38
...
Revelation: 20 29 22 11 14 17 17 13 21 11 19 17 18 20 8 21 18 24 21 15 27 21
```

Total verses in KJV: 31,102 (23,145 OT + 7,957 NT)

---

## e-Sword Format

e-Sword uses SQLite databases with standardized schemas.

### Bible Format (.bblx)

SQLite database with tables:

```sql
-- Book information
CREATE TABLE Books (
    Book INTEGER PRIMARY KEY,
    Short TEXT,
    Long TEXT
);

-- Verse content
CREATE TABLE Bible (
    Book INTEGER,
    Chapter INTEGER,
    Verse INTEGER,
    Scripture TEXT,
    PRIMARY KEY (Book, Chapter, Verse)
);

-- Module metadata
CREATE TABLE Details (
    Description TEXT,
    Abbreviation TEXT,
    Comments TEXT,
    Version TEXT,
    Font TEXT,
    RightToLeft INTEGER,
    OT INTEGER,
    NT INTEGER,
    Strong INTEGER
);
```

### Commentary Format (.cmtx)

```sql
CREATE TABLE Commentary (
    Book INTEGER,
    ChapterBegin INTEGER,
    VerseBegin INTEGER,
    ChapterEnd INTEGER,
    VerseEnd INTEGER,
    Comments TEXT,
    PRIMARY KEY (Book, ChapterBegin, VerseBegin)
);
```

### Dictionary Format (.dctx)

```sql
CREATE TABLE Dictionary (
    Topic TEXT PRIMARY KEY,
    Definition TEXT
);
```

---

## Hugo JSON Output Format

The converter generates JSON files for Hugo's data-driven templates.

### bibles.json (Metadata)

```json
{
  "bibles": [
    {
      "id": "kjv",
      "title": "King James Version",
      "abbrev": "KJV",
      "description": "...",
      "language": "en",
      "features": ["StrongsNumbers", "Morphology"],
      "tags": ["protestant", "historic"],
      "weight": 1
    }
  ],
  "meta": {
    "granularity": "chapter",
    "generated": "2025-12-30T00:00:00Z"
  }
}
```

### bibles_auxiliary.json (Content)

```json
{
  "kjv": {
    "books": [
      {
        "id": "Gen",
        "name": "Genesis",
        "abbrev": "Gen",
        "testament": "OT",
        "chapters": [
          {
            "number": 1,
            "verses": [
              {
                "number": 1,
                "text": "In the beginning God created..."
              }
            ]
          }
        ]
      }
    ]
  }
}
```

---

## Changelog

| Date | Change |
|------|--------|
| 2025-12-30 | Initial documentation created from reverse engineering and pysword reference |
