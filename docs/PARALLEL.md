# Parallel Corpus System

This document describes Juniper Bible's parallel corpus support for aligning multiple Bible translations and creating interlinear texts.

## Overview

A parallel corpus aligns multiple translations of the same text, enabling:

- Side-by-side comparison of translations
- Interlinear texts (Hebrew/Greek with glosses)
- Translation analysis and study
- Alignment at verse, sentence, or word level

## Use Cases

| Use Case | Alignment Level | Example |
|----------|-----------------|---------|
| Parallel Bible | Verse | KJV // NIV // ESV side-by-side |
| Gospel Synopsis | Verse | Matt // Mark // Luke // John |
| Interlinear | Word | Hebrew + transliteration + gloss |
| Translation Study | Sentence | Source + multiple target languages |
| Textual Criticism | Word | Manuscript variants |

## IR Data Structures

### ParallelCorpus Type

```go
type ParallelCorpus struct {
    ID         string            `json:"id"`
    Version    string            `json:"version"`
    BaseCorpus string            `json:"base_corpus"`
    Corpora    []string          `json:"corpora"`
    Alignments []*Alignment      `json:"alignments,omitempty"`
    Metadata   map[string]string `json:"metadata,omitempty"`
}
```

### Alignment Type

```go
type AlignmentLevel string

const (
    AlignBook    AlignmentLevel = "book"
    AlignChapter AlignmentLevel = "chapter"
    AlignVerse   AlignmentLevel = "verse"
    AlignToken   AlignmentLevel = "token"
)

type Alignment struct {
    ID    string          `json:"id"`
    Level AlignmentLevel  `json:"level"`
    Units []*AlignedUnit  `json:"units"`
}

type AlignedUnit struct {
    CorpusID string   `json:"corpus_id"`
    Ref      *Ref     `json:"ref,omitempty"`
    TokenIDs []string `json:"token_ids,omitempty"`
    Text     string   `json:"text,omitempty"`
}
```

### TokenAlignment Type

```go
type TokenAlignment struct {
    ID           string   `json:"id"`
    SourceTokens []string `json:"source_tokens"`
    TargetTokens []string `json:"target_tokens"`
    Confidence   float64  `json:"confidence,omitempty"`
    AlignType    string   `json:"align_type,omitempty"`
}
```

## Alignment Levels

### Verse-Level Alignment

The simplest alignment matches corresponding verses:

```json
{
  "level": "verse",
  "units": [
    {"corpus_id": "kjv", "ref": {"book": "Gen", "chapter": 1, "verse": 1}, "text": "In the beginning..."},
    {"corpus_id": "niv", "ref": {"book": "Gen", "chapter": 1, "verse": 1}, "text": "In the beginning..."},
    {"corpus_id": "esv", "ref": {"book": "Gen", "chapter": 1, "verse": 1}, "text": "In the beginning..."}
  ]
}
```

### Word-Level Alignment (Interlinear)

Token alignment for interlinear texts:

```json
{
  "level": "token",
  "units": [
    {"corpus_id": "hebrew", "token_ids": ["t1"], "text": "בְּרֵאשִׁית"},
    {"corpus_id": "translit", "token_ids": ["t1"], "text": "bərēʾšîṯ"},
    {"corpus_id": "gloss", "token_ids": ["t1"], "text": "in-beginning"}
  ]
}
```

### Interlinear Line Type

```go
type InterlinearLine struct {
    Ref    *Ref                          `json:"ref"`
    Layers map[string]*InterlinearLayer  `json:"layers"`
}

type InterlinearLayer struct {
    CorpusID string   `json:"corpus_id,omitempty"`
    Tokens   []*Token `json:"tokens"`
    Label    string   `json:"label,omitempty"`
}
```

## Creating Parallel Corpora

### From Multiple IR Files

```go
import "github.com/JuniperBible/juniper/core/ir"

// Load corpora
kjv, _ := ir.LoadCorpus("kjv.ir.json")
niv, _ := ir.LoadCorpus("niv.ir.json")
esv, _ := ir.LoadCorpus("esv.ir.json")

// Create verse-aligned parallel corpus
parallel, err := ir.AlignByVerse([]*ir.Corpus{kjv, niv, esv})
```

### With Token Alignment

```go
// Align tokens using Strong's numbers
opts := &ir.AlignOptions{
    UseStrongs:     true,
    MinConfidence:  0.8,
    AllowUnaligned: true,
}

hebrew, _ := ir.LoadCorpus("osmhb.ir.json")
english, _ := ir.LoadCorpus("kjv.ir.json")

alignments, err := ir.AlignTokens(hebrew, english, opts)
```

## CLI Usage

```bash
# Create parallel corpus from multiple translations
./capsule parallel create kjv.ir.json niv.ir.json esv.ir.json --out parallel.json

# Align at verse level (default)
./capsule parallel align kjv.ir.json niv.ir.json --level verse --out aligned.json

# Create interlinear from Hebrew + English
./capsule parallel interlinear osmhb.ir.json kjv.ir.json --out interlinear.json

# Export side-by-side HTML
./capsule parallel export parallel.json --format html --out parallel.html

# Export TSV for spreadsheet analysis
./capsule parallel export parallel.json --format tsv --out parallel.tsv
```

## Export Formats

### Side-by-Side HTML

```html
<div class="parallel-verse" data-ref="Gen.1.1">
  <div class="translation" data-corpus="kjv">
    <span class="label">KJV</span>
    <span class="text">In the beginning God created the heaven and the earth.</span>
  </div>
  <div class="translation" data-corpus="niv">
    <span class="label">NIV</span>
    <span class="text">In the beginning God created the heavens and the earth.</span>
  </div>
</div>
```

### Side-by-Side Markdown

```markdown
## Genesis 1:1

| KJV | NIV | ESV |
|-----|-----|-----|
| In the beginning God created the heaven and the earth. | In the beginning God created the heavens and the earth. | In the beginning, God created the heavens and the earth. |
```

### Interlinear HTML

```html
<div class="interlinear-line" data-ref="Gen.1.1">
  <div class="word-stack">
    <span class="hebrew">בְּרֵאשִׁית</span>
    <span class="translit">bərēʾšîṯ</span>
    <span class="strongs">H7225</span>
    <span class="gloss">In-beginning</span>
  </div>
  <div class="word-stack">
    <span class="hebrew">בָּרָא</span>
    <span class="translit">bārāʾ</span>
    <span class="strongs">H1254</span>
    <span class="gloss">created</span>
  </div>
  <!-- ... -->
</div>
```

### TSV Export

```tsv
Reference	KJV	NIV	ESV
Gen.1.1	In the beginning God created the heaven and the earth.	In the beginning God created the heavens and the earth.	In the beginning, God created the heavens and the earth.
Gen.1.2	And the earth was without form, and void...	Now the earth was formless and empty...	The earth was without form and void...
```

## Standard Alignments

### Pre-built Alignment Resources

| Resource | Description | Alignment Level |
|----------|-------------|-----------------|
| Interlinear Hebrew-English | OSMHB + KJV | Token (Strong's) |
| Interlinear Greek-English | SBLGNT + NASB | Token (Strong's) |
| Gospel Synopsis | Matt/Mark/Luke/John | Verse + pericope |
| LXX-MT Alignment | Septuagint to Masoretic | Verse |

### Using Pre-built Alignments

```go
// Load standard interlinear alignment
interlinear, err := ir.LoadStandardAlignment("hebrew-english")

// Apply to corpora
parallel, err := interlinear.Apply(hebrew, english)
```

## Versification Handling

When aligning corpora with different versification:

```go
// Corpora use different versifications
kjv.Versification = ir.VersificationKJV
lxx.Versification = ir.VersificationLXX

// Alignment handles mapping automatically
parallel, err := ir.AlignByVerse([]*ir.Corpus{kjv, lxx}, &ir.AlignOptions{
    HandleVersification: true,
    BaseVersification:   ir.VersificationKJV,
})
```

Alignment records versification mappings:

```json
{
  "alignment": {
    "level": "verse",
    "units": [
      {"corpus_id": "kjv", "ref": {"book": "Ps", "chapter": 10, "verse": 1}},
      {"corpus_id": "lxx", "ref": {"book": "Ps", "chapter": 9, "verse": 22}}
    ],
    "versification_note": "LXX combines Ps 9-10"
  }
}
```

## Confidence and Provenance

Alignments include confidence scores and provenance:

```go
type Alignment struct {
    // ...
    Confidence float64 `json:"confidence,omitempty"`
    Provenance string  `json:"provenance,omitempty"`
    Method     string  `json:"method,omitempty"`
}
```

| Method | Description | Typical Confidence |
|--------|-------------|-------------------|
| `manual` | Human-curated | 1.0 |
| `strongs` | Strong's number match | 0.9 |
| `statistical` | Statistical alignment | 0.7-0.8 |
| `heuristic` | Rule-based | 0.6-0.7 |

## Loss Tracking

Parallel corpus operations track information loss:

| Operation | Loss Level | Notes |
|-----------|------------|-------|
| Verse alignment | L0 | No content loss |
| Token alignment | L1 | Alignment may be imperfect |
| Export to TSV | L2 | Loses markup |
| Versification mapping | L1 | Some verses may not map |

## Schema Definition

Parallel corpus schema in `schemas/parallel.schema.json`:

```json
{
  "ParallelCorpus": {
    "type": "object",
    "properties": {
      "id": {"type": "string"},
      "version": {"type": "string"},
      "base_corpus": {"type": "string"},
      "corpora": {
        "type": "array",
        "items": {"type": "string"}
      },
      "alignments": {
        "type": "array",
        "items": {"$ref": "#/definitions/Alignment"}
      }
    },
    "required": ["id", "corpora"]
  },
  "Alignment": {
    "type": "object",
    "properties": {
      "id": {"type": "string"},
      "level": {
        "type": "string",
        "enum": ["book", "chapter", "verse", "token"]
      },
      "units": {
        "type": "array",
        "items": {"$ref": "#/definitions/AlignedUnit"}
      },
      "confidence": {"type": "number"},
      "provenance": {"type": "string"}
    }
  }
}
```

## Plugin Support

### Format Plugins with Parallel Support

| Plugin | Parallel Export | Notes |
|--------|-----------------|-------|
| format-json | Full | Native parallel structure |
| format-html | Full | Side-by-side and interlinear |
| format-markdown | Partial | Table format only |
| format-epub | Partial | Chapter-level parallel |

### Example: HTML Parallel Export

```go
parallel, _ := ir.LoadParallelCorpus("parallel.json")

opts := &ir.ExportOptions{
    Format:      "html",
    Interlinear: false,
    SideBySide:  true,
    Columns:     []string{"kjv", "niv", "esv"},
}

err := parallel.ExportSideBySideHTML(w, opts)
```

## Related Documentation

- [IR_IMPLEMENTATION.md](IR_IMPLEMENTATION.md) - Core IR system
- [VERSIFICATION.md](VERSIFICATION.md) - Versification for alignment
- [CROSSREF.md](CROSSREF.md) - Cross-references across parallel texts
