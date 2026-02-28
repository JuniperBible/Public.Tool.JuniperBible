# Versification Mapping in Juniper

## Overview

Different Bible traditions use different verse numbering systems (versifications). Juniper supports multiple versification systems and can map between them.

## Supported Versification Systems

| System | Books | Description |
|--------|-------|-------------|
| KJV | 66 | Protestant canon (default) |
| KJVA | 81 | KJV with Apocrypha |
| Vulg | 76 | Latin Vulgate (Catholic) |
| LXX | 86+ | Septuagint (Greek OT) |

## Key Differences

### Psalm Numbering

The most common difference is in Psalm numbering:

```
Hebrew/KJV Psalms    LXX/Vulgate Psalms
----------------    ------------------
Psalm 9             Psalm 9 (first half)
Psalm 10            Psalm 9 (second half)
Psalms 11-113       Psalms 10-112
Psalm 114-115       Psalm 113
Psalm 116           Psalm 114-115
Psalms 117-146      Psalms 116-145
Psalm 147           Psalm 146-147
Psalms 148-150      Psalms 148-150
```

### Deuterocanonical Books

Books present in Catholic/Orthodox but not Protestant canons:

| Book | Vulgate | LXX | Notes |
|------|---------|-----|-------|
| Tobit | ✓ | ✓ | |
| Judith | ✓ | ✓ | |
| Wisdom | ✓ | ✓ | Wisdom of Solomon |
| Sirach | ✓ | ✓ | Ecclesiasticus |
| Baruch | ✓ | ✓ | Includes Letter of Jeremiah |
| 1 Maccabees | ✓ | ✓ | |
| 2 Maccabees | ✓ | ✓ | |
| 3 Maccabees | | ✓ | Orthodox only |
| 4 Maccabees | | ✓ | Orthodox only |
| 1 Esdras | | ✓ | Also called 3 Ezra |
| Prayer of Manasseh | | ✓ | |
| Psalm 151 | | ✓ | |

### Additional Esther Content

- KJV Esther: 10 chapters
- Vulgate Esther: 16 chapters (includes Additions to Esther)
- LXX Esther: Different arrangement of additions

## Using Versification in Juniper

### Detecting Versification

The versification system is specified in the module's `.conf` file:

```ini
[ModuleName]
Versification=KJV
```

If not specified, KJV is assumed.

### API Usage

```go
// Create parser with specific versification
parser, err := sword.NewZTextParserWithVersification(
    modulePath,
    "Vulg",
)

// Get versification from parser
versif := parser.Versification()

// Map a verse between systems
kjvRef := versif.MapToKJV("Psalm", 9, 21)  // Vulg Ps 9:21 -> KJV Ps 10:1
```

### JSON Output

When converting modules, versification info is included:

```json
{
  "id": "vulgate",
  "versification": "Vulg",
  "books": [
    {
      "id": "Ps",
      "originalVersification": "Vulg",
      "chapters": [...]
    }
  ]
}
```

## Adding New Versification Systems

1. Create a new file: `pkg/sword/versification_<system>.go`

2. Define the verse counts:

```go
var SystemVerseCount = map[string][]int{
    "Gen": {31, 25, 24, ...},  // Verses per chapter
    // ...
}
```

3. Register the system:

```go
func init() {
    RegisterVersification("System", SystemVerseCount)
}
```

4. Add mapping functions in `verse_mapper.go` if needed

5. Add tests in `versification_systems_test.go`

## Implementation Files

```
pkg/sword/
├── versification_systems.go      # Core types, registry
├── versification_kjv.go          # KJV 66-book system
├── versification_kjva.go         # KJVA 81-book system
├── versification_vulg.go         # Vulgate 76-book system
├── versification_lxx.go          # Septuagint system
├── verse_mapper.go               # Cross-versification mapping
└── verse_mapper_test.go          # Mapping tests
```

## References

- [CrossWire Alternate Versification](https://wiki.crosswire.org/Alternate_Versification)
- SWORD source: `include/canon_*.h`
- [JSword versification](https://github.com/crosswire/jsword)
