// Package sword provides verse mapping between versification systems.
package sword

import (
	"fmt"
)

// VerseMapper handles mapping references between versification systems.
type VerseMapper struct {
	// mappings stores VersificationMapping indexed by "from->to" key
	mappings map[string]*VersificationMapping
}

// NewVerseMapper creates a new VerseMapper with standard mappings.
func NewVerseMapper() *VerseMapper {
	vm := &VerseMapper{
		mappings: make(map[string]*VersificationMapping),
	}
	vm.registerStandardMappings()
	return vm
}

// MapReference maps a reference from one versification system to another.
// If no direct mapping exists, it returns the reference unchanged with MapDirect type.
func (vm *VerseMapper) MapReference(ref Reference, fromSystem, toSystem string) (Reference, MappingType) {
	if fromSystem == toSystem {
		return ref, MapDirect
	}

	key := fmt.Sprintf("%s->%s", fromSystem, toSystem)
	mapping, ok := vm.mappings[key]
	if !ok {
		// No mapping defined, return unchanged
		return ref, MapDirect
	}

	// Look for a specific mapping rule
	for _, rule := range mapping.Rules {
		if rule.SourceBook == ref.Book &&
			(rule.SourceChapter == 0 || rule.SourceChapter == ref.Chapter) &&
			(rule.SourceVerse == 0 || rule.SourceVerse == ref.Verse) {

			mappedRef := Reference{
				Book:    rule.TargetBook,
				Chapter: rule.TargetChapter,
				Verse:   rule.TargetVerse,
			}

			// If target chapter/verse are 0, use source values
			if mappedRef.Chapter == 0 {
				mappedRef.Chapter = ref.Chapter
			}
			if mappedRef.Verse == 0 {
				mappedRef.Verse = ref.Verse
			}

			return mappedRef, rule.Type
		}
	}

	// Check for Psalm renumbering (most common mapping)
	if ref.Book == "Ps" {
		return vm.mapPsalm(ref, fromSystem, toSystem)
	}

	return ref, MapDirect
}

// MapToKJV maps a reference from any versification system to KJV.
func (vm *VerseMapper) MapToKJV(ref Reference, fromSystem string) (Reference, MappingType) {
	return vm.MapReference(ref, fromSystem, "KJV")
}

// MapFromKJV maps a reference from KJV to any versification system.
func (vm *VerseMapper) MapFromKJV(ref Reference, toSystem string) (Reference, MappingType) {
	return vm.MapReference(ref, "KJV", toSystem)
}

// mapPsalm handles the complex Psalm renumbering between systems.
// LXX/Vulgate Psalms have different numbering than Hebrew/KJV.
//
// Psalm mapping:
//   - KJV Ps 1-8 = LXX/Vulg Ps 1-8 (same)
//   - KJV Ps 9-10 = LXX/Vulg Ps 9 (merged in LXX)
//   - KJV Ps 11-113 = LXX/Vulg Ps 10-112 (offset by 1)
//   - KJV Ps 114-115 = LXX/Vulg Ps 113 (merged in LXX)
//   - KJV Ps 116:1-9 = LXX/Vulg Ps 114 (split in KJV)
//   - KJV Ps 116:10-19 = LXX/Vulg Ps 115 (split in KJV)
//   - KJV Ps 117-146 = LXX/Vulg Ps 116-145 (offset by 1)
//   - KJV Ps 147:1-11 = LXX/Vulg Ps 146 (split in KJV)
//   - KJV Ps 147:12-20 = LXX/Vulg Ps 147 (split in KJV)
//   - KJV Ps 148-150 = LXX/Vulg Ps 148-150 (same)
func (vm *VerseMapper) mapPsalm(ref Reference, fromSystem, toSystem string) (Reference, MappingType) {
	psNum := ref.Chapter

	// Map from KJV to LXX/Vulg
	if (fromSystem == "KJV" || fromSystem == "KJVA") && (toSystem == "LXX" || toSystem == "Vulg") {
		switch {
		case psNum <= 8:
			return ref, MapDirect
		case psNum == 9:
			// KJV 9 -> LXX 9:1-21 (first part)
			return Reference{Book: "Ps", Chapter: 9, Verse: ref.Verse}, MapDirect
		case psNum == 10:
			// KJV 10 -> LXX 9:22-39 (second part)
			return Reference{Book: "Ps", Chapter: 9, Verse: ref.Verse + 21}, MapMerge
		case psNum >= 11 && psNum <= 113:
			return Reference{Book: "Ps", Chapter: psNum - 1, Verse: ref.Verse}, MapRenumber
		case psNum == 114:
			// KJV 114 -> LXX 113:1-8
			return Reference{Book: "Ps", Chapter: 113, Verse: ref.Verse}, MapMerge
		case psNum == 115:
			// KJV 115 -> LXX 113:9+ (offset)
			return Reference{Book: "Ps", Chapter: 113, Verse: ref.Verse + 8}, MapMerge
		case psNum == 116:
			// KJV 116:1-9 -> LXX 114, KJV 116:10-19 -> LXX 115
			if ref.Verse <= 9 {
				return Reference{Book: "Ps", Chapter: 114, Verse: ref.Verse}, MapSplit
			}
			return Reference{Book: "Ps", Chapter: 115, Verse: ref.Verse - 9}, MapSplit
		case psNum >= 117 && psNum <= 146:
			return Reference{Book: "Ps", Chapter: psNum - 1, Verse: ref.Verse}, MapRenumber
		case psNum == 147:
			// KJV 147:1-11 -> LXX 146, KJV 147:12-20 -> LXX 147
			if ref.Verse <= 11 {
				return Reference{Book: "Ps", Chapter: 146, Verse: ref.Verse}, MapSplit
			}
			return Reference{Book: "Ps", Chapter: 147, Verse: ref.Verse - 11}, MapSplit
		case psNum >= 148:
			return ref, MapDirect
		}
	}

	// Map from LXX/Vulg to KJV
	if (fromSystem == "LXX" || fromSystem == "Vulg") && (toSystem == "KJV" || toSystem == "KJVA") {
		switch {
		case psNum <= 8:
			return ref, MapDirect
		case psNum == 9:
			// LXX 9:1-21 -> KJV 9, LXX 9:22+ -> KJV 10
			if ref.Verse <= 21 {
				return Reference{Book: "Ps", Chapter: 9, Verse: ref.Verse}, MapDirect
			}
			return Reference{Book: "Ps", Chapter: 10, Verse: ref.Verse - 21}, MapSplit
		case psNum >= 10 && psNum <= 112:
			return Reference{Book: "Ps", Chapter: psNum + 1, Verse: ref.Verse}, MapRenumber
		case psNum == 113:
			// LXX 113:1-8 -> KJV 114, LXX 113:9+ -> KJV 115
			if ref.Verse <= 8 {
				return Reference{Book: "Ps", Chapter: 114, Verse: ref.Verse}, MapSplit
			}
			return Reference{Book: "Ps", Chapter: 115, Verse: ref.Verse - 8}, MapSplit
		case psNum == 114:
			// LXX 114 -> KJV 116:1-9
			return Reference{Book: "Ps", Chapter: 116, Verse: ref.Verse}, MapMerge
		case psNum == 115:
			// LXX 115 -> KJV 116:10+
			return Reference{Book: "Ps", Chapter: 116, Verse: ref.Verse + 9}, MapMerge
		case psNum >= 116 && psNum <= 145:
			return Reference{Book: "Ps", Chapter: psNum + 1, Verse: ref.Verse}, MapRenumber
		case psNum == 146:
			// LXX 146 -> KJV 147:1-11
			return Reference{Book: "Ps", Chapter: 147, Verse: ref.Verse}, MapMerge
		case psNum == 147:
			// LXX 147 -> KJV 147:12+
			return Reference{Book: "Ps", Chapter: 147, Verse: ref.Verse + 11}, MapMerge
		case psNum >= 148:
			return ref, MapDirect
		}
	}

	return ref, MapDirect
}

// registerStandardMappings sets up the standard mappings between versification systems.
func (vm *VerseMapper) registerStandardMappings() {
	// Vulgate to KJV mapping (beyond Psalm renumbering)
	vm.registerMapping(&VersificationMapping{
		From: "Vulg",
		To:   "KJV",
		Rules: []MappingRule{
			// 3 Esdras (Greek Ezra) -> not in KJV
			{SourceBook: "3Esd", SourceChapter: 0, TargetBook: "", Type: MapMissing},
			// 4 Esdras -> not in KJV
			{SourceBook: "4Esd", SourceChapter: 0, TargetBook: "", Type: MapMissing},
			// Prayer of Manasseh -> not in KJV
			{SourceBook: "PrMan", SourceChapter: 0, TargetBook: "", Type: MapMissing},
		},
	})

	// KJV to Vulgate mapping (inverse)
	vm.registerMapping(&VersificationMapping{
		From:  "KJV",
		To:    "Vulg",
		Rules: []MappingRule{
			// No additional rules needed - Vulg has all KJV books
		},
	})

	// LXX to KJV mapping (beyond Psalm renumbering)
	vm.registerMapping(&VersificationMapping{
		From: "LXX",
		To:   "KJV",
		Rules: []MappingRule{
			// 3 Maccabees -> not in KJV
			{SourceBook: "3Macc", SourceChapter: 0, TargetBook: "", Type: MapMissing},
			// 4 Maccabees -> not in KJV
			{SourceBook: "4Macc", SourceChapter: 0, TargetBook: "", Type: MapMissing},
			// Odes -> not in KJV
			{SourceBook: "Odes", SourceChapter: 0, TargetBook: "", Type: MapMissing},
			// Psalms of Solomon -> not in KJV
			{SourceBook: "PssSol", SourceChapter: 0, TargetBook: "", Type: MapMissing},
		},
	})

	// KJV to LXX mapping (inverse)
	vm.registerMapping(&VersificationMapping{
		From:  "KJV",
		To:    "LXX",
		Rules: []MappingRule{
			// All KJV books exist in LXX (or have equivalents)
		},
	})
}

// registerMapping adds a mapping to the registry.
func (vm *VerseMapper) registerMapping(mapping *VersificationMapping) {
	key := fmt.Sprintf("%s->%s", mapping.From, mapping.To)
	vm.mappings[key] = mapping
}

// GetSystemForModule determines the versification system for a module.
func GetSystemForModule(module *Module) *VersificationSystem {
	versName := module.Versification
	if versName == "" {
		versName = "KJV" // Default
	}
	versName = NormalizeVersificationName(versName)

	system := GetVersification(versName)
	if system == nil {
		// Fallback to KJV if unknown
		system = GetVersification("KJV")
	}
	return system
}

// defaultMapper is a global mapper instance for convenience.
var defaultMapper = NewVerseMapper()

// MapReferenceToKJV maps any reference to KJV using the default mapper.
func MapReferenceToKJV(ref Reference, fromSystem string) (Reference, MappingType) {
	return defaultMapper.MapToKJV(ref, fromSystem)
}

// MapReferenceFromKJV maps from KJV to any system using the default mapper.
func MapReferenceFromKJV(ref Reference, toSystem string) (Reference, MappingType) {
	return defaultMapper.MapFromKJV(ref, toSystem)
}
