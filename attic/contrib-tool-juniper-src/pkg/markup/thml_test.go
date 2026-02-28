package markup

import (
	"testing"
)

func TestThMLConverter_ConvertBasicText(t *testing.T) {
	conv := NewThMLConverter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "This is a commentary note.",
			expected: "This is a commentary note.",
		},
		{
			name:     "emphasis",
			input:    "This is <em>emphasized</em> text.",
			expected: "This is *emphasized* text.",
		},
		{
			name:     "bold",
			input:    "This is <b>bold</b> text.",
			expected: "This is **bold** text.",
		},
		{
			name:     "scripture reference",
			input:    `See <scripRef passage="Gen.1.1">Genesis 1:1</scripRef>.`,
			expected: "See [Genesis 1:1](Gen.1.1).",
		},
		{
			name:     "heading",
			input:    "<h2>Section Title</h2>",
			expected: "## Section Title",
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

func TestThMLConverter_ExtractScripRefs(t *testing.T) {
	conv := NewThMLConverter()

	input := `See <scripRef passage="Gen.1.1">Genesis 1:1</scripRef> and <scripRef passage="John.3.16">John 3:16</scripRef>.`

	result := conv.Convert(input)

	if len(result.ScripRefs) != 2 {
		t.Errorf("expected 2 scripture refs, got %d", len(result.ScripRefs))
	}

	expected := []string{"Gen.1.1", "John.3.16"}
	for i, ref := range expected {
		if i >= len(result.ScripRefs) || result.ScripRefs[i] != ref {
			t.Errorf("ScripRef[%d]: got %q, want %q", i, result.ScripRefs[i], ref)
		}
	}
}

func TestThMLConverter_ExtractNotes(t *testing.T) {
	conv := NewThMLConverter()

	input := `The word <note>Greek: logos</note> means "word".`

	result := conv.Convert(input)

	if len(result.Notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(result.Notes))
	}

	if len(result.Notes) > 0 && result.Notes[0] != "Greek: logos" {
		t.Errorf("note: got %q, want %q", result.Notes[0], "Greek: logos")
	}
}

func TestThMLConverter_AllHeadings(t *testing.T) {
	conv := NewThMLConverter()

	tests := []struct {
		input    string
		expected string
	}{
		{"<h1>Title</h1>", "# Title"},
		{"<h2>Subtitle</h2>", "## Subtitle"},
		{"<h3>Section</h3>", "### Section"},
		{"<h4>Subsection</h4>", "#### Subsection"},
		{"<h5>Minor</h5>", "##### Minor"},
		{"<h6>Smallest</h6>", "###### Smallest"},
	}

	for _, tt := range tests {
		result := conv.ConvertText(tt.input)
		if result != tt.expected {
			t.Errorf("got %q, want %q", result, tt.expected)
		}
	}
}

func TestThMLConverter_Italic(t *testing.T) {
	conv := NewThMLConverter()

	input := `<i>italic</i> text`
	result := conv.ConvertText(input)

	if result != "*italic* text" {
		t.Errorf("got %q, want %q", result, "*italic* text")
	}
}

func TestThMLConverter_Strong(t *testing.T) {
	conv := NewThMLConverter()

	input := `<strong>strong</strong> text`
	result := conv.ConvertText(input)

	if result != "**strong** text" {
		t.Errorf("got %q, want %q", result, "**strong** text")
	}
}

func TestThMLConverter_Paragraph(t *testing.T) {
	conv := NewThMLConverter()

	input := `<p>First paragraph.</p><p>Second paragraph.</p>`
	result := conv.ConvertText(input)

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestThMLConverter_LineBreak(t *testing.T) {
	conv := NewThMLConverter()

	input := `Line one.<br>Line two.<br/>Line three.`
	result := conv.ConvertText(input)

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestThMLConverter_Blockquote(t *testing.T) {
	conv := NewThMLConverter()

	input := `<blockquote>A quoted text</blockquote>`
	result := conv.ConvertText(input)

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestThMLConverter_Foreign(t *testing.T) {
	conv := NewThMLConverter()

	input := `The word <foreign>logos</foreign> means word.`
	result := conv.ConvertText(input)

	if result != "The word *logos* means word." {
		t.Errorf("got %q, want %q", result, "The word *logos* means word.")
	}
}

func TestThMLConverter_SyncMarker(t *testing.T) {
	conv := NewThMLConverter()

	input := `Text <sync type="Strongs" value="H430"/> more text <sync type="x">`
	result := conv.ConvertText(input)

	if result != "Text more text" {
		t.Errorf("got %q, want %q", result, "Text more text")
	}
}

func TestThMLConverter_Div(t *testing.T) {
	conv := NewThMLConverter()

	input := `<div type="section">Content inside div</div>`
	result := conv.ConvertText(input)

	if result != "Content inside div" {
		t.Errorf("got %q, want %q", result, "Content inside div")
	}
}

func TestThMLConverter_EmptyNote(t *testing.T) {
	conv := NewThMLConverter()

	input := `Text with <note></note> empty note.`

	result := conv.Convert(input)

	if len(result.Notes) != 0 {
		t.Errorf("expected 0 notes for empty content, got %d", len(result.Notes))
	}
}

func TestThMLConverter_MultipleBlankLines(t *testing.T) {
	conv := NewThMLConverter()

	input := "<p>Para 1</p>\n\n\n\n<p>Para 2</p>"
	result := conv.ConvertText(input)

	// Should clean up multiple blank lines
	if result == "" {
		t.Error("expected non-empty result")
	}
}
