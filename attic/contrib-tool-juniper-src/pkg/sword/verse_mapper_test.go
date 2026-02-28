package sword

import (
	"testing"
)

func TestVerseMapper_SameSystem(t *testing.T) {
	vm := NewVerseMapper()

	ref := Reference{Book: "Ps", Chapter: 23, Verse: 1}
	mapped, typ := vm.MapReference(ref, "KJV", "KJV")

	if mapped != ref {
		t.Errorf("Same system mapping changed reference: got %v, want %v", mapped, ref)
	}
	if typ != MapDirect {
		t.Errorf("Same system mapping type: got %v, want MapDirect", typ)
	}
}

func TestVerseMapper_PsalmMapping_KJVtoLXX(t *testing.T) {
	vm := NewVerseMapper()

	tests := []struct {
		name     string
		kjvRef   Reference
		wantRef  Reference
		wantType MappingType
	}{
		// Psalms 1-8 unchanged
		{
			name:     "Ps 1:1 unchanged",
			kjvRef:   Reference{Book: "Ps", Chapter: 1, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 1, Verse: 1},
			wantType: MapDirect,
		},
		{
			name:     "Ps 8:9 unchanged",
			kjvRef:   Reference{Book: "Ps", Chapter: 8, Verse: 9},
			wantRef:  Reference{Book: "Ps", Chapter: 8, Verse: 9},
			wantType: MapDirect,
		},
		// KJV Ps 9-10 -> LXX Ps 9
		{
			name:     "Ps 9:1 -> Ps 9:1",
			kjvRef:   Reference{Book: "Ps", Chapter: 9, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 9, Verse: 1},
			wantType: MapDirect,
		},
		{
			name:     "Ps 10:1 -> Ps 9:22",
			kjvRef:   Reference{Book: "Ps", Chapter: 10, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 9, Verse: 22},
			wantType: MapMerge,
		},
		// KJV Ps 11-113 -> LXX Ps 10-112 (offset by 1)
		{
			name:     "Ps 23:1 -> Ps 22:1 (famous shepherd psalm)",
			kjvRef:   Reference{Book: "Ps", Chapter: 23, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 22, Verse: 1},
			wantType: MapRenumber,
		},
		{
			name:     "Ps 51:1 -> Ps 50:1 (Miserere)",
			kjvRef:   Reference{Book: "Ps", Chapter: 51, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 50, Verse: 1},
			wantType: MapRenumber,
		},
		{
			name:     "Ps 100:1 -> Ps 99:1",
			kjvRef:   Reference{Book: "Ps", Chapter: 100, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 99, Verse: 1},
			wantType: MapRenumber,
		},
		// KJV Ps 116 -> LXX Ps 114-115 (split)
		{
			name:     "Ps 116:1 -> Ps 114:1",
			kjvRef:   Reference{Book: "Ps", Chapter: 116, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 114, Verse: 1},
			wantType: MapSplit,
		},
		{
			name:     "Ps 116:9 -> Ps 114:9",
			kjvRef:   Reference{Book: "Ps", Chapter: 116, Verse: 9},
			wantRef:  Reference{Book: "Ps", Chapter: 114, Verse: 9},
			wantType: MapSplit,
		},
		{
			name:     "Ps 116:10 -> Ps 115:1",
			kjvRef:   Reference{Book: "Ps", Chapter: 116, Verse: 10},
			wantRef:  Reference{Book: "Ps", Chapter: 115, Verse: 1},
			wantType: MapSplit,
		},
		// KJV Ps 147 -> LXX Ps 146-147 (split)
		{
			name:     "Ps 147:1 -> Ps 146:1",
			kjvRef:   Reference{Book: "Ps", Chapter: 147, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 146, Verse: 1},
			wantType: MapSplit,
		},
		{
			name:     "Ps 147:12 -> Ps 147:1",
			kjvRef:   Reference{Book: "Ps", Chapter: 147, Verse: 12},
			wantRef:  Reference{Book: "Ps", Chapter: 147, Verse: 1},
			wantType: MapSplit,
		},
		// Psalms 148-150 unchanged
		{
			name:     "Ps 150:6 unchanged",
			kjvRef:   Reference{Book: "Ps", Chapter: 150, Verse: 6},
			wantRef:  Reference{Book: "Ps", Chapter: 150, Verse: 6},
			wantType: MapDirect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped, typ := vm.MapReference(tt.kjvRef, "KJV", "LXX")

			if mapped != tt.wantRef {
				t.Errorf("MapReference(%v, KJV, LXX) = %v, want %v", tt.kjvRef, mapped, tt.wantRef)
			}
			if typ != tt.wantType {
				t.Errorf("MapReference(%v, KJV, LXX) type = %v, want %v", tt.kjvRef, typ, tt.wantType)
			}
		})
	}
}

func TestVerseMapper_PsalmMapping_LXXtoKJV(t *testing.T) {
	vm := NewVerseMapper()

	tests := []struct {
		name     string
		lxxRef   Reference
		wantRef  Reference
		wantType MappingType
	}{
		// LXX Ps 1-8 unchanged
		{
			name:     "Ps 1:1 unchanged",
			lxxRef:   Reference{Book: "Ps", Chapter: 1, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 1, Verse: 1},
			wantType: MapDirect,
		},
		// LXX Ps 9 -> KJV Ps 9-10 (split)
		{
			name:     "Ps 9:1 -> Ps 9:1",
			lxxRef:   Reference{Book: "Ps", Chapter: 9, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 9, Verse: 1},
			wantType: MapDirect,
		},
		{
			name:     "Ps 9:22 -> Ps 10:1",
			lxxRef:   Reference{Book: "Ps", Chapter: 9, Verse: 22},
			wantRef:  Reference{Book: "Ps", Chapter: 10, Verse: 1},
			wantType: MapSplit,
		},
		// LXX Ps 10-112 -> KJV Ps 11-113 (offset by 1)
		{
			name:     "Ps 22:1 -> Ps 23:1 (shepherd psalm)",
			lxxRef:   Reference{Book: "Ps", Chapter: 22, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 23, Verse: 1},
			wantType: MapRenumber,
		},
		{
			name:     "Ps 50:1 -> Ps 51:1 (Miserere)",
			lxxRef:   Reference{Book: "Ps", Chapter: 50, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 51, Verse: 1},
			wantType: MapRenumber,
		},
		// LXX Ps 113 -> KJV Ps 114-115 (split)
		{
			name:     "Ps 113:1 -> Ps 114:1",
			lxxRef:   Reference{Book: "Ps", Chapter: 113, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 114, Verse: 1},
			wantType: MapSplit,
		},
		{
			name:     "Ps 113:9 -> Ps 115:1",
			lxxRef:   Reference{Book: "Ps", Chapter: 113, Verse: 9},
			wantRef:  Reference{Book: "Ps", Chapter: 115, Verse: 1},
			wantType: MapSplit,
		},
		// LXX Ps 114-115 -> KJV Ps 116 (merge)
		{
			name:     "Ps 114:1 -> Ps 116:1",
			lxxRef:   Reference{Book: "Ps", Chapter: 114, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 116, Verse: 1},
			wantType: MapMerge,
		},
		{
			name:     "Ps 115:1 -> Ps 116:10",
			lxxRef:   Reference{Book: "Ps", Chapter: 115, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 116, Verse: 10},
			wantType: MapMerge,
		},
		// LXX Ps 146-147 -> KJV Ps 147 (merge)
		{
			name:     "Ps 146:1 -> Ps 147:1",
			lxxRef:   Reference{Book: "Ps", Chapter: 146, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 147, Verse: 1},
			wantType: MapMerge,
		},
		{
			name:     "Ps 147:1 -> Ps 147:12",
			lxxRef:   Reference{Book: "Ps", Chapter: 147, Verse: 1},
			wantRef:  Reference{Book: "Ps", Chapter: 147, Verse: 12},
			wantType: MapMerge,
		},
		// LXX Ps 148-150 unchanged
		{
			name:     "Ps 150:6 unchanged",
			lxxRef:   Reference{Book: "Ps", Chapter: 150, Verse: 6},
			wantRef:  Reference{Book: "Ps", Chapter: 150, Verse: 6},
			wantType: MapDirect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped, typ := vm.MapReference(tt.lxxRef, "LXX", "KJV")

			if mapped != tt.wantRef {
				t.Errorf("MapReference(%v, LXX, KJV) = %v, want %v", tt.lxxRef, mapped, tt.wantRef)
			}
			if typ != tt.wantType {
				t.Errorf("MapReference(%v, LXX, KJV) type = %v, want %v", tt.lxxRef, typ, tt.wantType)
			}
		})
	}
}

func TestVerseMapper_VulgateMapping(t *testing.T) {
	vm := NewVerseMapper()

	// Vulgate uses same Psalm numbering as LXX
	ref := Reference{Book: "Ps", Chapter: 23, Verse: 1}
	mapped, typ := vm.MapReference(ref, "KJV", "Vulg")

	if mapped.Chapter != 22 {
		t.Errorf("Vulgate Psalm mapping: got chapter %d, want 22", mapped.Chapter)
	}
	if typ != MapRenumber {
		t.Errorf("Vulgate Psalm mapping type: got %v, want MapRenumber", typ)
	}
}

func TestVerseMapper_NonPsalmBooks(t *testing.T) {
	vm := NewVerseMapper()

	// Non-Psalm books should pass through unchanged between KJV and LXX
	tests := []Reference{
		{Book: "Gen", Chapter: 1, Verse: 1},
		{Book: "Exod", Chapter: 20, Verse: 1},
		{Book: "Matt", Chapter: 5, Verse: 1},
		{Book: "John", Chapter: 3, Verse: 16},
		{Book: "Rev", Chapter: 22, Verse: 21},
	}

	for _, ref := range tests {
		t.Run(ref.Book, func(t *testing.T) {
			mapped, typ := vm.MapReference(ref, "KJV", "LXX")

			if mapped != ref {
				t.Errorf("Non-Psalm book changed: got %v, want %v", mapped, ref)
			}
			if typ != MapDirect {
				t.Errorf("Non-Psalm book mapping type: got %v, want MapDirect", typ)
			}
		})
	}
}

func TestVerseMapper_MissingBooks(t *testing.T) {
	vm := NewVerseMapper()

	// Books that don't exist in KJV
	tests := []struct {
		book       string
		fromSystem string
	}{
		{"3Macc", "LXX"},
		{"4Macc", "LXX"},
		{"Odes", "LXX"},
		{"PssSol", "LXX"},
		{"3Esd", "Vulg"},
		{"4Esd", "Vulg"},
		{"PrMan", "Vulg"},
	}

	for _, tt := range tests {
		t.Run(tt.book, func(t *testing.T) {
			ref := Reference{Book: tt.book, Chapter: 1, Verse: 1}
			_, typ := vm.MapReference(ref, tt.fromSystem, "KJV")

			if typ != MapMissing {
				t.Errorf("MapReference(%v, %s, KJV) type = %v, want MapMissing", ref, tt.fromSystem, typ)
			}
		})
	}
}

func TestMapReferenceToKJV(t *testing.T) {
	// Test the convenience function
	ref := Reference{Book: "Ps", Chapter: 22, Verse: 1}
	mapped, typ := MapReferenceToKJV(ref, "LXX")

	if mapped.Chapter != 23 {
		t.Errorf("MapReferenceToKJV: got chapter %d, want 23", mapped.Chapter)
	}
	if typ != MapRenumber {
		t.Errorf("MapReferenceToKJV type: got %v, want MapRenumber", typ)
	}
}

func TestMapReferenceFromKJV(t *testing.T) {
	// Test the convenience function
	ref := Reference{Book: "Ps", Chapter: 23, Verse: 1}
	mapped, typ := MapReferenceFromKJV(ref, "Vulg")

	if mapped.Chapter != 22 {
		t.Errorf("MapReferenceFromKJV: got chapter %d, want 22", mapped.Chapter)
	}
	if typ != MapRenumber {
		t.Errorf("MapReferenceFromKJV type: got %v, want MapRenumber", typ)
	}
}

func TestGetSystemForModule(t *testing.T) {
	tests := []struct {
		name           string
		versification  string
		expectedSystem string
	}{
		{"Empty defaults to KJV", "", "KJV"},
		{"Explicit KJV", "KJV", "KJV"},
		{"Vulgate normalized", "Vulgate", "Vulg"},
		{"LXX lowercase", "lxx", "LXX"},
		{"Unknown falls back to KJV", "Unknown", "KJV"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &Module{Versification: tt.versification}
			system := GetSystemForModule(module)

			if system == nil {
				t.Fatal("GetSystemForModule returned nil")
			}
			if system.Name != tt.expectedSystem {
				t.Errorf("GetSystemForModule() = %s, want %s", system.Name, tt.expectedSystem)
			}
		})
	}
}

func TestVerseMapper_RoundTrip(t *testing.T) {
	vm := NewVerseMapper()

	// Test that mapping to KJV and back preserves the reference
	// (for cases where mapping is reversible)
	tests := []struct {
		name   string
		ref    Reference
		system string
	}{
		// Simple renumber cases
		{"Ps 22:1 LXX roundtrip", Reference{Book: "Ps", Chapter: 22, Verse: 1}, "LXX"},
		{"Ps 50:3 Vulg roundtrip", Reference{Book: "Ps", Chapter: 50, Verse: 3}, "Vulg"},
		// Edge cases that should be preserved
		{"Ps 1:1 unchanged", Reference{Book: "Ps", Chapter: 1, Verse: 1}, "LXX"},
		{"Ps 150:6 unchanged", Reference{Book: "Ps", Chapter: 150, Verse: 6}, "LXX"},
		// Non-Psalm books
		{"Gen 1:1 roundtrip", Reference{Book: "Gen", Chapter: 1, Verse: 1}, "LXX"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Map to KJV
			kjvRef, _ := vm.MapReference(tt.ref, tt.system, "KJV")
			// Map back
			backRef, _ := vm.MapReference(kjvRef, "KJV", tt.system)

			if backRef != tt.ref {
				t.Errorf("Roundtrip failed: %v -> %v -> %v, want %v", tt.ref, kjvRef, backRef, tt.ref)
			}
		})
	}
}
