package markup

import (
	"strings"
	"testing"
)

func TestNewTEIConverter(t *testing.T) {
	conv := NewTEIConverter()
	if conv == nil {
		t.Fatal("NewTEIConverter() returned nil")
	}
	if !conv.IncludeEtymology {
		t.Error("IncludeEtymology should default to true")
	}
	if !conv.IncludeGrammar {
		t.Error("IncludeGrammar should default to true")
	}
}

func TestTEIConverter_ExtractHeadword(t *testing.T) {
	conv := NewTEIConverter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple headword",
			input:    "<entry><orth>λόγος</orth></entry>",
			expected: "λόγος",
		},
		{
			name:     "headword with attributes",
			input:    `<entry><orth type="main" xml:lang="grc">θεός</orth></entry>`,
			expected: "θεός",
		},
		{
			name:     "no headword",
			input:    "<entry><def>A definition</def></entry>",
			expected: "",
		},
		{
			name:     "empty headword",
			input:    "<entry><orth></orth></entry>",
			expected: "",
		},
		{
			name:     "headword with whitespace",
			input:    "<entry><orth>  word  </orth></entry>",
			expected: "word",
		},
		{
			name:     "Hebrew headword",
			input:    "<entry><orth>אֱלֹהִים</orth></entry>",
			expected: "אֱלֹהִים",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conv.Convert(tt.input)
			if result.Headword != tt.expected {
				t.Errorf("Headword = %q, want %q", result.Headword, tt.expected)
			}
		})
	}
}

func TestTEIConverter_ExtractPartOfSpeech(t *testing.T) {
	conv := NewTEIConverter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "noun",
			input:    "<entry><gramGrp><pos>noun</pos></gramGrp></entry>",
			expected: "noun",
		},
		{
			name:     "verb",
			input:    "<entry><gramGrp><pos>verb</pos></gramGrp></entry>",
			expected: "verb",
		},
		{
			name:     "no part of speech",
			input:    "<entry><orth>word</orth></entry>",
			expected: "",
		},
		{
			name:     "POS with attributes",
			input:    `<entry><gramGrp><pos type="main">adjective</pos></gramGrp></entry>`,
			expected: "adjective",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conv.Convert(tt.input)
			if result.PartOfSpeech != tt.expected {
				t.Errorf("PartOfSpeech = %q, want %q", result.PartOfSpeech, tt.expected)
			}
		})
	}
}

func TestTEIConverter_ExtractEtymology(t *testing.T) {
	conv := NewTEIConverter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple etymology",
			input:    "<entry><etym>from Greek logos</etym></entry>",
			expected: "from Greek logos",
		},
		{
			name:     "no etymology",
			input:    "<entry><orth>word</orth></entry>",
			expected: "",
		},
		{
			name:     "etymology with attributes",
			input:    `<entry><etym type="compound">from aleph + bet</etym></entry>`,
			expected: "from aleph + bet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conv.Convert(tt.input)
			if result.Etymology != tt.expected {
				t.Errorf("Etymology = %q, want %q", result.Etymology, tt.expected)
			}
		})
	}
}

func TestTEIConverter_ExtractEtymology_Disabled(t *testing.T) {
	conv := &TEIConverter{IncludeEtymology: false, IncludeGrammar: true}

	input := "<entry><etym>from Latin</etym></entry>"
	result := conv.Convert(input)

	if result.Etymology != "" {
		t.Errorf("Etymology should be empty when disabled, got %q", result.Etymology)
	}
}

func TestTEIConverter_ExtractSenses(t *testing.T) {
	conv := NewTEIConverter()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single definition",
			input:    "<entry><def>a spoken word</def></entry>",
			expected: []string{"a spoken word"},
		},
		{
			name:     "multiple definitions",
			input:    "<entry><def>meaning one</def><def>meaning two</def></entry>",
			expected: []string{"meaning one", "meaning two"},
		},
		{
			name:     "sense elements",
			input:    "<entry><sense>first sense</sense><sense>second sense</sense></entry>",
			expected: []string{"first sense", "second sense"},
		},
		{
			name:     "no senses",
			input:    "<entry><orth>word</orth></entry>",
			expected: []string{},
		},
		{
			name:     "empty definitions ignored",
			input:    "<entry><def>valid</def><def></def><def>  </def></entry>",
			expected: []string{"valid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conv.Convert(tt.input)
			if len(result.Senses) != len(tt.expected) {
				t.Errorf("len(Senses) = %d, want %d", len(result.Senses), len(tt.expected))
				return
			}
			for i, sense := range result.Senses {
				if sense != tt.expected[i] {
					t.Errorf("Senses[%d] = %q, want %q", i, sense, tt.expected[i])
				}
			}
		})
	}
}

func TestTEIConverter_ExtractReferences(t *testing.T) {
	conv := NewTEIConverter()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single reference",
			input:    "<entry><ref>John 1:1</ref></entry>",
			expected: []string{"John 1:1"},
		},
		{
			name:     "multiple references",
			input:    "<entry><ref>Gen 1:1</ref><ref>John 3:16</ref></entry>",
			expected: []string{"Gen 1:1", "John 3:16"},
		},
		{
			name:     "no references",
			input:    "<entry><orth>word</orth></entry>",
			expected: []string{},
		},
		{
			name:     "reference with target",
			input:    `<entry><ref target="bible:John.1.1">John 1:1</ref></entry>`,
			expected: []string{"John 1:1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conv.Convert(tt.input)
			if len(result.References) != len(tt.expected) {
				t.Errorf("len(References) = %d, want %d", len(result.References), len(tt.expected))
				return
			}
			for i, ref := range result.References {
				if ref != tt.expected[i] {
					t.Errorf("References[%d] = %q, want %q", i, ref, tt.expected[i])
				}
			}
		})
	}
}

func TestTEIConverter_Convert_Basic(t *testing.T) {
	conv := NewTEIConverter()

	input := `<entry>
<orth>λόγος</orth>
<gramGrp><pos>noun</pos></gramGrp>
<def>word, speech</def>
</entry>`

	result := conv.Convert(input)

	if result.Headword != "λόγος" {
		t.Errorf("Headword = %q, want %q", result.Headword, "λόγος")
	}
	if result.PartOfSpeech != "noun" {
		t.Errorf("PartOfSpeech = %q, want %q", result.PartOfSpeech, "noun")
	}
	if len(result.Senses) != 1 || result.Senses[0] != "word, speech" {
		t.Errorf("Senses = %v, want [word, speech]", result.Senses)
	}
}

func TestTEIConverter_Convert_Complex(t *testing.T) {
	conv := NewTEIConverter()

	input := `<entry>
<orth>θεός</orth>
<pron>theos</pron>
<gramGrp><pos>noun</pos><gen>masculine</gen></gramGrp>
<etym>from Proto-Greek *thesos</etym>
<sense n="1"><def>God, deity</def></sense>
<sense n="2"><def>divine being</def></sense>
<ref target="bible:John.1.1">John 1:1</ref>
</entry>`

	result := conv.Convert(input)

	if result.Headword != "θεός" {
		t.Errorf("Headword = %q, want %q", result.Headword, "θεός")
	}
	if result.PartOfSpeech != "noun" {
		t.Errorf("PartOfSpeech = %q, want %q", result.PartOfSpeech, "noun")
	}
	if result.Etymology != "from Proto-Greek *thesos" {
		t.Errorf("Etymology = %q, want %q", result.Etymology, "from Proto-Greek *thesos")
	}
	if len(result.Senses) != 2 {
		t.Errorf("len(Senses) = %d, want 2", len(result.Senses))
	}
	if len(result.References) != 1 {
		t.Errorf("len(References) = %d, want 1", len(result.References))
	}
}

func TestTEIConverter_Convert_Hebrew(t *testing.T) {
	conv := NewTEIConverter()

	input := `<entry>
<orth>אֱלֹהִים</orth>
<pron>elohim</pron>
<gramGrp><pos>noun</pos><num>plural</num></gramGrp>
<def>God, gods</def>
<ref>Genesis 1:1</ref>
</entry>`

	result := conv.Convert(input)

	if result.Headword != "אֱלֹהִים" {
		t.Errorf("Headword = %q, want Hebrew text", result.Headword)
	}
	if !strings.Contains(result.Text, "## אֱלֹהִים") {
		t.Errorf("Text should contain Hebrew heading, got: %q", result.Text)
	}
}

func TestTEIConverter_ConvertToMarkdown_Headword(t *testing.T) {
	conv := NewTEIConverter()

	input := "<entry><orth>logos</orth></entry>"
	result := conv.Convert(input)

	if !strings.Contains(result.Text, "## logos") {
		t.Errorf("Text should contain heading '## logos', got: %q", result.Text)
	}
}

func TestTEIConverter_ConvertToMarkdown_Pronunciation(t *testing.T) {
	conv := NewTEIConverter()

	input := "<entry><pron>theos</pron></entry>"
	result := conv.Convert(input)

	if !strings.Contains(result.Text, "/*theos*/") {
		t.Errorf("Text should contain pronunciation '/*theos*/', got: %q", result.Text)
	}
}

func TestTEIConverter_ConvertToMarkdown_PartOfSpeech(t *testing.T) {
	conv := NewTEIConverter()

	input := "<entry><gramGrp><pos>noun</pos></gramGrp></entry>"
	result := conv.Convert(input)

	if !strings.Contains(result.Text, "**noun**") {
		t.Errorf("Text should contain bold POS '**noun**', got: %q", result.Text)
	}
}

func TestTEIConverter_ConvertToMarkdown_Gender(t *testing.T) {
	conv := NewTEIConverter()

	input := "<entry><gramGrp><gen>masculine</gen></gramGrp></entry>"
	result := conv.Convert(input)

	if !strings.Contains(result.Text, "(masculine)") {
		t.Errorf("Text should contain gender '(masculine)', got: %q", result.Text)
	}
}

func TestTEIConverter_ConvertToMarkdown_Etymology(t *testing.T) {
	conv := NewTEIConverter()

	input := "<entry><etym>from Greek</etym></entry>"
	result := conv.Convert(input)

	if !strings.Contains(result.Text, "**Etymology:** from Greek") {
		t.Errorf("Text should contain etymology, got: %q", result.Text)
	}
}

func TestTEIConverter_ConvertToMarkdown_Quote(t *testing.T) {
	conv := NewTEIConverter()

	input := "<entry><quote>example text</quote></entry>"
	result := conv.Convert(input)

	if !strings.Contains(result.Text, `"example text"`) {
		t.Errorf("Text should contain quoted text, got: %q", result.Text)
	}
}

func TestTEIConverter_ConvertToMarkdown_RefWithTarget(t *testing.T) {
	conv := NewTEIConverter()

	input := `<entry><ref target="bible:John.1.1">John 1:1</ref></entry>`
	result := conv.Convert(input)

	if !strings.Contains(result.Text, "[John 1:1](bible:John.1.1)") {
		t.Errorf("Text should contain markdown link, got: %q", result.Text)
	}
}

func TestTEIConverter_ConvertToMarkdown_RefWithoutTarget(t *testing.T) {
	conv := NewTEIConverter()

	input := "<entry><ref>John 1:1</ref></entry>"
	result := conv.Convert(input)

	if !strings.Contains(result.Text, "*John 1:1*") {
		t.Errorf("Text should contain italic reference, got: %q", result.Text)
	}
}

func TestTEIConverter_ConvertToMarkdown_Bibl(t *testing.T) {
	conv := NewTEIConverter()

	input := "<entry><bibl>Strong's 2316</bibl></entry>"
	result := conv.Convert(input)

	if !strings.Contains(result.Text, "(Strong's 2316)") {
		t.Errorf("Text should contain bibliography in parens, got: %q", result.Text)
	}
}

func TestTEIConverter_EdgeCase_Empty(t *testing.T) {
	conv := NewTEIConverter()

	result := conv.Convert("")

	if result.Text != "" {
		t.Errorf("Empty input should produce empty text, got: %q", result.Text)
	}
	if result.Headword != "" {
		t.Errorf("Headword should be empty, got: %q", result.Headword)
	}
}

func TestTEIConverter_EdgeCase_PlainText(t *testing.T) {
	conv := NewTEIConverter()

	result := conv.Convert("Just plain text without tags")

	if result.Text != "Just plain text without tags" {
		t.Errorf("Plain text should pass through, got: %q", result.Text)
	}
}

func TestTEIConverter_EdgeCase_Nested(t *testing.T) {
	conv := NewTEIConverter()

	input := `<entry><gramGrp><pos>noun</pos><gen>neuter</gen></gramGrp></entry>`
	result := conv.Convert(input)

	// Should handle nested tags
	if !strings.Contains(result.Text, "**noun**") {
		t.Errorf("Should extract nested pos, got: %q", result.Text)
	}
	if !strings.Contains(result.Text, "(neuter)") {
		t.Errorf("Should extract nested gen, got: %q", result.Text)
	}
}

func TestTEIConverter_EdgeCase_UnclosedTags(t *testing.T) {
	conv := NewTEIConverter()

	// Unclosed tags - should not panic
	input := "<entry><orth>word"
	result := conv.Convert(input)

	// Should not contain unprocessed tags
	if strings.Contains(result.Text, "<orth>") {
		t.Errorf("Unclosed tag should be handled, got: %q", result.Text)
	}
}

func TestTEIConverter_GrammarDisabled(t *testing.T) {
	conv := &TEIConverter{IncludeEtymology: true, IncludeGrammar: false}

	input := `<entry><gramGrp><pos>noun</pos><gen>masculine</gen></gramGrp></entry>`
	result := conv.Convert(input)

	if strings.Contains(result.Text, "**noun**") {
		t.Errorf("POS should be removed when grammar disabled, got: %q", result.Text)
	}
	if strings.Contains(result.Text, "(masculine)") {
		t.Errorf("Gender should be removed when grammar disabled, got: %q", result.Text)
	}
}

func TestTEIConverter_EtymologyDisabled(t *testing.T) {
	conv := &TEIConverter{IncludeEtymology: false, IncludeGrammar: true}

	input := "<entry><etym>from Latin</etym></entry>"
	result := conv.Convert(input)

	if strings.Contains(result.Text, "Etymology") {
		t.Errorf("Etymology should be removed when disabled, got: %q", result.Text)
	}
}

func TestTEIConverter_ConvertText(t *testing.T) {
	conv := NewTEIConverter()

	input := "<entry><orth>test</orth><def>a test</def></entry>"
	text := conv.ConvertText(input)

	if !strings.Contains(text, "## test") {
		t.Errorf("ConvertText should return markdown, got: %q", text)
	}
	if !strings.Contains(text, "a test") {
		t.Errorf("ConvertText should contain definition, got: %q", text)
	}
}

func TestTEIConverter_WhitespaceNormalization(t *testing.T) {
	conv := NewTEIConverter()

	input := "<entry>\n\n\n<orth>word</orth>\n\n\n\n<def>definition</def>\n\n\n</entry>"
	result := conv.Convert(input)

	// Should not have more than 2 consecutive newlines
	if strings.Contains(result.Text, "\n\n\n") {
		t.Errorf("Should normalize multiple newlines, got: %q", result.Text)
	}
}

func TestTEIConverter_Fixtures(t *testing.T) {
	conv := NewTEIConverter()

	// Table-driven tests from fixtures
	fixtures := []struct {
		name     string
		input    string
		wantHead string
		wantPOS  string
	}{
		{
			name:     "Strong's Greek entry",
			input:    `<entry><orth>G2316</orth><gramGrp><pos>noun</pos></gramGrp><def>theos; God</def></entry>`,
			wantHead: "G2316",
			wantPOS:  "noun",
		},
		{
			name:     "Strong's Hebrew entry",
			input:    `<entry><orth>H430</orth><gramGrp><pos>noun</pos></gramGrp><def>elohim; God</def></entry>`,
			wantHead: "H430",
			wantPOS:  "noun",
		},
		{
			name:     "Dictionary entry",
			input:    `<entry><orth>baptizo</orth><gramGrp><pos>verb</pos></gramGrp><def>to immerse</def></entry>`,
			wantHead: "baptizo",
			wantPOS:  "verb",
		},
	}

	for _, tt := range fixtures {
		t.Run(tt.name, func(t *testing.T) {
			result := conv.Convert(tt.input)
			if result.Headword != tt.wantHead {
				t.Errorf("Headword = %q, want %q", result.Headword, tt.wantHead)
			}
			if result.PartOfSpeech != tt.wantPOS {
				t.Errorf("PartOfSpeech = %q, want %q", result.PartOfSpeech, tt.wantPOS)
			}
		})
	}
}

func TestTEIResult_Structure(t *testing.T) {
	result := &TEIResult{
		Text:         "Some text",
		Headword:     "word",
		PartOfSpeech: "noun",
		Etymology:    "from Latin",
		Senses:       []string{"meaning 1", "meaning 2"},
		References:   []string{"John 1:1"},
	}

	if result.Text != "Some text" {
		t.Errorf("Text = %q, want 'Some text'", result.Text)
	}
	if len(result.Senses) != 2 {
		t.Errorf("len(Senses) = %d, want 2", len(result.Senses))
	}
	if len(result.References) != 1 {
		t.Errorf("len(References) = %d, want 1", len(result.References))
	}
}

func BenchmarkTEIConverter_Convert(b *testing.B) {
	conv := NewTEIConverter()
	input := `<entry>
<orth>θεός</orth>
<pron>theos</pron>
<gramGrp><pos>noun</pos><gen>masculine</gen></gramGrp>
<etym>from Proto-Greek *thesos</etym>
<sense n="1"><def>God, deity</def></sense>
<sense n="2"><def>divine being</def></sense>
<ref target="bible:John.1.1">John 1:1</ref>
</entry>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conv.Convert(input)
	}
}
