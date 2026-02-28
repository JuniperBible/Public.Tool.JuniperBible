package markup

import (
	"testing"
)

func TestOSISConverter_ConvertBasicText(t *testing.T) {
	conv := NewOSISConverter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "In the beginning God created the heaven and the earth.",
			expected: "In the beginning God created the heaven and the earth.",
		},
		{
			name:     "word with strongs",
			input:    `<w lemma="strong:H430">God</w> created`,
			expected: "God^H430^ created",
		},
		{
			name:     "divine name",
			input:    `<divineName>LORD</divineName>`,
			expected: `<span class="divine-name">LORD</span>`,
		},
		{
			name:     "translator addition",
			input:    `the earth was <transChange type="added">without form</transChange>`,
			expected: "the earth was *without form*",
		},
		{
			name:     "quote",
			input:    `<q>Let there be light</q>`,
			expected: `"Let there be light"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conv.ConvertVerse(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestOSISConverter_ExtractStrongs(t *testing.T) {
	conv := NewOSISConverter()
	conv.PreserveStrongs = true

	input := `<w lemma="strong:H7225">In the beginning</w> <w lemma="strong:H430">God</w> <w lemma="strong:H1254">created</w>`

	result := conv.Convert(input)

	if len(result.Strongs) != 3 {
		t.Errorf("expected 3 Strong's numbers, got %d", len(result.Strongs))
	}

	expected := []string{"H7225", "H430", "H1254"}
	for i, s := range expected {
		if i >= len(result.Strongs) || result.Strongs[i] != s {
			t.Errorf("Strong's[%d]: got %q, want %q", i, result.Strongs[i], s)
		}
	}
}

func TestOSISConverter_ExtractNotes(t *testing.T) {
	conv := NewOSISConverter()

	input := `The <note type="x">Or, wind</note> Spirit of God moved`

	result := conv.Convert(input)

	if len(result.Notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(result.Notes))
	}

	if len(result.Notes) > 0 && result.Notes[0] != "Or, wind" {
		t.Errorf("note: got %q, want %q", result.Notes[0], "Or, wind")
	}

	// Note content should be removed from text
	if result.Text != "The Spirit of God moved" {
		t.Errorf("text: got %q, want %q", result.Text, "The Spirit of God moved")
	}
}

func TestOSISConverter_RedLetterText(t *testing.T) {
	conv := NewOSISConverter()

	input := `<q who="Jesus">I am the way, the truth, and the life</q>`

	result := conv.Convert(input)

	if !result.HasRedText {
		t.Error("expected HasRedText to be true")
	}

	expected := `<span class="red-letter">I am the way, the truth, and the life</span>`
	if result.Text != expected {
		t.Errorf("text: got %q, want %q", result.Text, expected)
	}
}

func TestOSISConverter_Poetry(t *testing.T) {
	conv := NewOSISConverter()

	input := `<lg><l level="1">The heavens declare the glory of God;</l><l level="2">and the firmament sheweth his handywork.</l></lg>`

	result := conv.ConvertVerse(input)

	if result == "" {
		t.Error("expected non-empty result for poetry")
	}
}

func TestOSISConverter_CrossReference(t *testing.T) {
	conv := NewOSISConverter()

	input := `See also <reference osisRef="John.3.16">John 3:16</reference>`

	result := conv.ConvertVerse(input)

	expected := "See also [John 3:16](John.3.16)"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestOSISConverter_ExtractMorphology(t *testing.T) {
	conv := NewOSISConverter()
	conv.PreserveMorphology = true

	input := `<w lemma="strong:H430" morph="robinson:N-ASM">God</w>`

	result := conv.Convert(input)

	if len(result.Morphology) != 1 {
		t.Errorf("expected 1 morphology code, got %d: %v", len(result.Morphology), result.Morphology)
	}
	if len(result.Morphology) > 0 && result.Morphology[0] != "N-ASM" {
		t.Errorf("morphology: got %q, want %q", result.Morphology[0], "N-ASM")
	}
}

func TestOSISConverter_ExtractMorphology_StrongMorph(t *testing.T) {
	conv := NewOSISConverter()
	conv.PreserveMorphology = true

	input := `<w morph="strongMorph:TH8799">created</w>`

	result := conv.Convert(input)

	if len(result.Morphology) != 1 {
		t.Errorf("expected 1 morphology code, got %d: %v", len(result.Morphology), result.Morphology)
	}
	if len(result.Morphology) > 0 && result.Morphology[0] != "TH8799" {
		t.Errorf("morphology: got %q, want %q", result.Morphology[0], "TH8799")
	}
}

func TestOSISConverter_ExtractMorphology_Dedup(t *testing.T) {
	conv := NewOSISConverter()
	conv.PreserveMorphology = true

	// Same morphology code twice
	input := `<w morph="robinson:N-ASM">word1</w> <w morph="robinson:N-ASM">word2</w>`

	result := conv.Convert(input)

	if len(result.Morphology) != 1 {
		t.Errorf("expected 1 unique morphology code, got %d: %v", len(result.Morphology), result.Morphology)
	}
}

func TestOSISConverter_ConvertWithAnnotations(t *testing.T) {
	conv := NewOSISConverter()
	conv.PreserveStrongs = true

	input := `<w lemma="strong:H430">God</w> <note>footnote</note>`

	result := conv.ConvertWithAnnotations(input)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Strongs) != 1 {
		t.Errorf("expected 1 Strong's number, got %d", len(result.Strongs))
	}
	if len(result.Notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(result.Notes))
	}
}

func TestOSISConverter_Title(t *testing.T) {
	conv := NewOSISConverter()

	input := `<title type="chapter">Chapter 1</title>In the beginning`

	result := conv.ConvertVerse(input)

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestOSISConverter_Milestone(t *testing.T) {
	conv := NewOSISConverter()

	input := `Text before<milestone type="x-p"/>text after`

	result := conv.ConvertVerse(input)

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestOSISConverter_EmptyNote(t *testing.T) {
	conv := NewOSISConverter()

	input := `Text with <note></note> empty note.`

	result := conv.Convert(input)

	if len(result.Notes) != 0 {
		t.Errorf("expected 0 notes for empty content, got %d", len(result.Notes))
	}
}

func TestOSISConverter_VerseElement(t *testing.T) {
	conv := NewOSISConverter()

	input := `<verse osisID="Gen.1.1">In the beginning</verse>`

	result := conv.ConvertVerse(input)

	// Verse tags should be stripped
	if result != "In the beginning" {
		t.Errorf("got %q, want %q", result, "In the beginning")
	}
}

func TestOSISConverter_ForeignLanguage(t *testing.T) {
	conv := NewOSISConverter()

	input := `<foreign xml:lang="he">בְּרֵאשִׁית</foreign>`

	result := conv.ConvertVerse(input)

	if result == "" {
		t.Error("expected non-empty result for foreign language")
	}
}

func TestOSISConverter_PreserveStrongsFalse(t *testing.T) {
	conv := NewOSISConverter()
	conv.PreserveStrongs = false

	input := `<w lemma="strong:H430">God</w>`

	result := conv.Convert(input)

	// With PreserveStrongs=false, Strong's numbers should be stripped
	if len(result.Strongs) != 0 {
		t.Errorf("expected 0 Strong's numbers with PreserveStrongs=false, got %d", len(result.Strongs))
	}
}

func TestOSISConverter_PoetryLevels(t *testing.T) {
	conv := NewOSISConverter()

	// Test level 2 and level 3 poetry indentation
	input := `<lg><l level="2">Level 2 line</l><l level="3">Level 3 line</l></lg>`

	result := conv.ConvertVerse(input)

	if result == "" {
		t.Error("expected non-empty result for poetry levels")
	}
}

func TestOSISConverter_PoetryNoLevel(t *testing.T) {
	conv := NewOSISConverter()

	input := `<lg><l>Simple line of poetry</l></lg>`

	result := conv.ConvertVerse(input)

	if result == "" {
		t.Error("expected non-empty result")
	}
}
