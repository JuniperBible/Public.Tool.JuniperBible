package markup

import (
	"testing"
)

func TestGBFConverter_ConvertBasicText(t *testing.T) {
	conv := NewGBFConverter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "In the beginning God created.",
			expected: "In the beginning God created.",
		},
		{
			name:     "italic",
			input:    "This is <FI>italic<Fi> text.",
			expected: "This is *italic* text.",
		},
		{
			name:     "bold",
			input:    "This is <FB>bold<Fb> text.",
			expected: "This is **bold** text.",
		},
		{
			name:     "line break",
			input:    "Line one.<CL>Line two.",
			expected: "Line one.\nLine two.",
		},
		{
			name:     "paragraph",
			input:    "Para one.<CM>Para two.",
			expected: "Para one.\n\nPara two.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conv.ConvertText(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGBFConverter_ExtractStrongs(t *testing.T) {
	conv := NewGBFConverter()
	conv.PreserveStrongs = true

	input := `<WH430>God<Wh> <WH1254>created<Wh>`

	result := conv.Convert(input)

	if len(result.Strongs) != 2 {
		t.Errorf("expected 2 Strong's numbers, got %d", len(result.Strongs))
	}

	expected := []string{"H430", "H1254"}
	for i, s := range expected {
		if i >= len(result.Strongs) || result.Strongs[i] != s {
			t.Errorf("Strong's[%d]: got %q, want %q", i, result.Strongs[i], s)
		}
	}
}

func TestGBFConverter_RedLetterText(t *testing.T) {
	conv := NewGBFConverter()

	input := `Jesus said, <FR>I am the way<Fr>.`

	result := conv.Convert(input)

	if !result.HasRedText {
		t.Error("expected HasRedText to be true")
	}

	if !contains(result.Text, "red-letter") {
		t.Errorf("expected red-letter span in text: %q", result.Text)
	}
}

func TestGBFConverter_ExtractFootnotes(t *testing.T) {
	conv := NewGBFConverter()

	input := `The word <RF>Or, wind<Rf> Spirit.`

	result := conv.Convert(input)

	if len(result.Footnotes) != 1 {
		t.Errorf("expected 1 footnote, got %d", len(result.Footnotes))
	}

	if len(result.Footnotes) > 0 && result.Footnotes[0] != "Or, wind" {
		t.Errorf("footnote: got %q, want %q", result.Footnotes[0], "Or, wind")
	}
}

func TestGBFConverter_ExtractMorphology(t *testing.T) {
	conv := NewGBFConverter()
	conv.PreserveMorphology = true

	input := `<WTV-AAI-3S>created<Wt> <WTN-ASM>heaven<Wt>`

	result := conv.Convert(input)

	if len(result.Morphology) != 2 {
		t.Errorf("expected 2 morphology codes, got %d: %v", len(result.Morphology), result.Morphology)
	}
}

func TestGBFConverter_ExtractMorphology_Dedup(t *testing.T) {
	conv := NewGBFConverter()
	conv.PreserveMorphology = true

	// Same morphology code twice
	input := `<WTV-AAI-3S>word1<Wt> <WTV-AAI-3S>word2<Wt>`

	result := conv.Convert(input)

	if len(result.Morphology) != 1 {
		t.Errorf("expected 1 unique morphology code, got %d: %v", len(result.Morphology), result.Morphology)
	}
}

func TestGBFConverter_ExtractCrossRefs(t *testing.T) {
	conv := NewGBFConverter()

	input := `See also <RX>Genesis 1:1<Rx> and <RX>John 1:1<Rx>.`

	result := conv.Convert(input)

	if len(result.CrossRefs) != 2 {
		t.Errorf("expected 2 cross-refs, got %d", len(result.CrossRefs))
	}
}

func TestGBFConverter_Underline(t *testing.T) {
	conv := NewGBFConverter()

	input := `This is <FU>underlined<Fu> text.`
	result := conv.ConvertText(input)

	// Underline should be converted (check it doesn't crash)
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestGBFConverter_Superscript(t *testing.T) {
	conv := NewGBFConverter()

	input := `Text with <FS>superscript<Fs> content.`
	result := conv.ConvertText(input)

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestGBFConverter_GreekStrongs(t *testing.T) {
	conv := NewGBFConverter()
	conv.PreserveStrongs = true

	input := `<WG2316>God<Wg>`

	result := conv.Convert(input)

	if len(result.Strongs) != 1 {
		t.Errorf("expected 1 Strong's number, got %d", len(result.Strongs))
	}
	if len(result.Strongs) > 0 && result.Strongs[0] != "G2316" {
		t.Errorf("Strong's: got %q, want %q", result.Strongs[0], "G2316")
	}
}

func TestGBFConverter_EmptyFootnote(t *testing.T) {
	conv := NewGBFConverter()

	input := `Text with <RF><Rf> empty footnote.`

	result := conv.Convert(input)

	if len(result.Footnotes) != 0 {
		t.Errorf("expected 0 footnotes for empty content, got %d", len(result.Footnotes))
	}
}

func TestGBFConverter_EmptyCrossRef(t *testing.T) {
	conv := NewGBFConverter()

	input := `Text with <RX><Rx> empty cross-ref.`

	result := conv.Convert(input)

	if len(result.CrossRefs) != 0 {
		t.Errorf("expected 0 cross-refs for empty content, got %d", len(result.CrossRefs))
	}
}

func TestGBFConverter_PreserveStrongsFalse(t *testing.T) {
	conv := NewGBFConverter()
	conv.PreserveStrongs = false

	input := `<WH430>God<Wh> <WG2316>theos<Wg>`

	result := conv.Convert(input)

	// With PreserveStrongs=false, Strong's numbers should be stripped
	if len(result.Strongs) != 0 {
		t.Errorf("expected 0 Strong's numbers with PreserveStrongs=false, got %d", len(result.Strongs))
	}
	// Text should still contain the words
	if result.Text == "" {
		t.Error("expected non-empty text")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
