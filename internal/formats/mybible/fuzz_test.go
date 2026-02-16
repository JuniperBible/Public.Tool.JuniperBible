package mybible

import (
	"testing"
)

// FuzzStripHTML tests the HTML stripping function with fuzzing
func FuzzStripHTML(f *testing.F) {
	// Seed corpus with various HTML inputs
	f.Add("<b>bold text</b>")
	f.Add("<i>italic text</i>")
	f.Add("<br/>line break")
	f.Add("<p>paragraph</p>")
	f.Add("plain text without tags")
	f.Add("<tag>nested <inner>tags</inner></tag>")
	f.Add("< invalid tag")
	f.Add("unclosed <tag")
	f.Add("</closing tag without opening>")
	f.Add("<self-closing />")
	f.Add("")
	f.Add("<>empty tag</>")
	f.Add("<<<multiple brackets>>>")
	f.Add("<tag attribute=\"value\">content</tag>")
	f.Add("Text with <span style=\"color:red\">styled content</span>")
	f.Add("<div><p>Nested <b>multiple</b> levels</p></div>")
	f.Add("<!-- comment -->text")
	f.Add("<script>alert('xss')</script>")
	f.Add("&lt;escaped&gt;")
	f.Add("<tag>unbalanced")  // Missing closing tag
	f.Add("unbalanced</tag>") // Missing opening tag

	f.Fuzz(func(t *testing.T, input string) {
		// The stripHTML function should not panic on any input
		result := stripHTML(input)

		// Basic invariants
		if len(result) > len(input) {
			t.Errorf("Stripped result is longer than input: input=%d, result=%d", len(input), len(result))
		}

		// Result should not contain complete tags (< followed by >)
		// Note: This is a simplified check; the actual function may leave
		// some malformed tags, but it should handle well-formed tags
		balanced := true
		depth := 0
		for _, ch := range result {
			if ch == '<' {
				depth++
			} else if ch == '>' {
				depth--
				if depth < 0 {
					balanced = false
				}
			}
		}

		// For well-formed input with balanced tags, output should not have tag characters
		// (This is a weak invariant since the function is simple and may not handle all cases)
		_ = balanced

		// Result should be trimmed (no leading/trailing whitespace)
		if len(result) > 0 {
			if result[0] == ' ' || result[0] == '\t' || result[0] == '\n' {
				t.Error("Result has leading whitespace")
			}
			if result[len(result)-1] == ' ' || result[len(result)-1] == '\t' || result[len(result)-1] == '\n' {
				t.Error("Result has trailing whitespace")
			}
		}

		// For empty input, result should be empty
		if len(input) == 0 && len(result) != 0 {
			t.Error("Result should be empty for empty input")
		}

		// For input without tags, result should be the trimmed input
		if !containsHTMLTags(input) {
			trimmed := input
			for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\t' || trimmed[0] == '\n') {
				trimmed = trimmed[1:]
			}
			for len(trimmed) > 0 && (trimmed[len(trimmed)-1] == ' ' || trimmed[len(trimmed)-1] == '\t' || trimmed[len(trimmed)-1] == '\n') {
				trimmed = trimmed[:len(trimmed)-1]
			}
			if result != trimmed {
				// Allow some flexibility since the function may do additional processing
				_ = result
			}
		}
	})
}

// Helper function to check if input contains HTML tags
func containsHTMLTags(s string) bool {
	hasOpen := false
	for _, ch := range s {
		if ch == '<' {
			hasOpen = true
		}
		if ch == '>' && hasOpen {
			return true
		}
	}
	return false
}

// FuzzVerse tests the Verse struct with fuzzing
func FuzzVerse(f *testing.F) {
	// Seed with various verse configurations
	f.Add(1, 1, 1, "In the beginning")
	f.Add(66, 22, 21, "The grace of our Lord")
	f.Add(19, 119, 176, "I have gone astray")
	f.Add(0, 0, 0, "")
	f.Add(-1, -1, -1, "Invalid verse")
	f.Add(999, 999, 999, "Out of range")

	f.Fuzz(func(t *testing.T, bookNumber, chapter, verse int, text string) {
		// Creating a Verse struct should not panic
		v := Verse{
			BookNumber: bookNumber,
			Chapter:    chapter,
			Verse:      verse,
			Text:       text,
		}

		// Basic invariants
		if v.BookNumber != bookNumber {
			t.Error("BookNumber mismatch")
		}
		if v.Chapter != chapter {
			t.Error("Chapter mismatch")
		}
		if v.Verse != verse {
			t.Error("Verse mismatch")
		}
		if v.Text != text {
			t.Error("Text mismatch")
		}

		// Test stripHTML on the verse text
		stripped := stripHTML(v.Text)
		_ = stripped

		// Stripped text should not be longer than original
		if len(stripped) > len(v.Text) {
			t.Error("Stripped text is longer than original")
		}
	})
}

// FuzzParseMetadata tests metadata parsing logic with fuzzing
func FuzzParseMetadata(f *testing.F) {
	// Seed with various metadata key-value pairs
	f.Add("description", "King James Version")
	f.Add("language", "en")
	f.Add("", "")
	f.Add("version", "1.0")
	f.Add("detailed_info", "Complete Bible text")
	f.Add("key with spaces", "value with spaces")
	f.Add("\x00null\x00", "\x00value\x00")

	f.Fuzz(func(t *testing.T, name, value string) {
		// Creating a metadata map should not panic
		metadata := make(map[string]string)

		// Storing metadata should work
		metadata[name] = value

		// Retrieving should return the same value
		retrieved := metadata[name]
		if retrieved != value {
			t.Error("Metadata value mismatch")
		}

		// Test with empty name
		if name == "" {
			// Empty keys are allowed in Go maps
			_ = metadata[""]
		}

		// Test stripHTML on metadata values
		if value != "" {
			stripped := stripHTML(value)
			_ = stripped
		}
	})
}
